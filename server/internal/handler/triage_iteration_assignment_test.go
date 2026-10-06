package handler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func triageIterationAcceptBody(item TriageItem, target, action, request string) map[string]any {
	return map[string]any{"request_id": request, "expected_revision": item.Issue.Revision, "action": action, "fields": map[string]any{"current_iteration_id": target}}
}
func TestTriageIterationAcceptanceSharesActionAndTransaction(t *testing.T) {
	for _, action := range []string{"accept", "accept_and_execute"} {
		t.Run(action, func(t *testing.T) {
			h := lifecycleHTTPHandler(t)
			target := lifecycleHTTPCreate(t, h, "Reviewed work")
			triageEnableForTest(t)
			input := triageInputForTest()
			project := dbfx.Project(t, "Triage iteration project")
			input["candidate_project_id"] = project
			if action == "accept_and_execute" {
				agent := dbfx.Agent(t, "Triage iteration executor", testRuntimeID)
				input["candidate_assignee_type"] = "agent"
				input["candidate_assignee_id"] = agent
			}
			var item TriageItem
			testutil.Call(t, h.CreateTriageItem, newRequest("POST", "/api/triage/items", input)).Want(201).JSON(&item)
			body := triageIterationAcceptBody(item, target, action, uuid.NewString())
			var first, again TriageActionResult
			request := func() *testutil.Response {
				return testutil.Call(t, h.ActOnTriageItem, withURLParam(newRequest("POST", "/api/triage/items/"+item.Issue.ID+"/actions", body), "id", item.Issue.ID))
			}
			request().Want(200).JSON(&first)
			request().Want(200).JSON(&again)
			if first.Action.ID != again.Action.ID {
				t.Fatal("triage replay changed action identity")
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND admission_status='accepted' AND current_iteration_id=$2 AND project_id=$3`, item.Issue.ID, target, project); n != 1 {
				t.Fatal("accept/project/iteration not committed together")
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE issue_id=$1 AND iteration_id=$2 AND operation_id=$3`, item.Issue.ID, target, first.Action.ID); n != 1 {
				t.Fatal("membership event did not reuse triage action identity")
			}
			want := 0
			if action == "accept_and_execute" {
				want = 1
			}
			if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, item.Issue.ID); n != want {
				t.Fatalf("execution rows=%d want%d", n, want)
			}
		})
	}
}

type triageIterationFailStarter struct{ inner txStarter }

func (s triageIterationFailStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &triageIterationFailTx{tx}, nil
}

type triageIterationFailTx struct{ pgx.Tx }

func (tx *triageIterationFailTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "AppendIterationLifecycleEvent") {
		return pgconn.CommandTag{}, errors.New("injected triage membership event failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}
func TestTriageIterationRecorderFailureKeepsPending(t *testing.T) {
	h := lifecycleHTTPHandler(t)
	target := lifecycleHTTPCreate(t, h, "Atomic acceptance")
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	id := uuid.NewString()
	h.TxStarter = triageIterationFailStarter{h.TxStarter}
	testutil.Call(t, h.ActOnTriageItem, withURLParam(newRequest("POST", "/api/triage/items/actions", triageIterationAcceptBody(item, target, "accept", id)), "id", item.Issue.ID)).Want(500)
	if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND admission_status='pending' AND current_iteration_id IS NULL AND revision=$2`, item.Issue.ID, item.Issue.Revision); n != 1 {
		t.Fatal("failed membership left accepted issue")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM triage_action WHERE request_id=$1`, id); n != 0 {
		t.Fatal("failed join retained action")
	}
}
func TestTriageIterationTargetCancelledWhileWaiting(t *testing.T) {
	h := lifecycleHTTPHandler(t)
	target := lifecycleHTTPCreate(t, h, "Cancelled before acceptance")
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	holder, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if err = iteration.LockWorkspace(ctx, holder, parseUUID(testWorkspaceID)); err != nil {
		t.Fatal(err)
	}
	reached := make(chan struct{}, 1)
	h.TxStarter = iterationFenceBarrierStarter{h.TxStarter, reached}
	request := withURLParam(newRequest("POST", "/api/triage/items/actions", triageIterationAcceptBody(item, target, "accept", uuid.NewString())), "id", item.Issue.ID)
	result := make(chan *testutil.Response, 1)
	go func() { result <- testutil.Call(t, h.ActOnTriageItem, request) }()
	waitIterationBarrier(t, reached)
	if _, err = holder.Exec(ctx, `UPDATE iteration SET status='cancelled' WHERE id=$1`, target); err != nil {
		t.Fatal(err)
	}
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-result:
		response.Want(409)
	case <-ctx.Done():
		t.Fatal("acceptance failed to resume")
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND admission_status='pending' AND current_iteration_id IS NULL`, item.Issue.ID); n != 1 {
		t.Fatal("stale target changed admission")
	}
}

type triageIterationOrderStarter struct {
	inner             txStarter
	sampled, prepared bool
}

func (s *triageIterationOrderStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &triageIterationOrderTx{Tx: tx, state: s}, nil
}

type triageIterationOrderTx struct {
	pgx.Tx
	state *triageIterationOrderStarter
}

func (tx *triageIterationOrderTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "LockIssueIterationParticipation") {
		tx.state.prepared = true
	}
	if sql == "SELECT clock_timestamp()" {
		tx.state.sampled = true
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}
func (tx *triageIterationOrderTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if (strings.Contains(sql, "UPDATE issue SET admission_status='accepted'") || strings.Contains(sql, "DELETE FROM issue_to_label")) && (!tx.state.prepared || !tx.state.sampled) {
		return pgconn.CommandTag{}, errors.New("acceptance preceded participation locks and clock sample")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}
func TestTriageIterationPreparesBeforeAcceptanceWrite(t *testing.T) {
	h := lifecycleHTTPHandler(t)
	target := lifecycleHTTPCreate(t, h, "Prepared acceptance")
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	state := &triageIterationOrderStarter{inner: h.TxStarter}
	h.TxStarter = state
	testutil.Call(t, h.ActOnTriageItem, withURLParam(newRequest("POST", "/api/triage/items/actions", triageIterationAcceptBody(item, target, "accept", uuid.NewString())), "id", item.Issue.ID)).Want(200)
	if !state.prepared || !state.sampled {
		t.Fatal("membership preparation not observed")
	}
}

func TestTriageIterationAcceptanceIntoActiveRetainsEmptyOriginal(t *testing.T) {
	h := lifecycleHTTPHandler(t)
	target := lifecycleHTTPCreate(t, h, "Active reviewed work")
	revision, scope := int64(1), int64(1)
	lifecycleHTTPApply(t, h, iteration.Draft{Operation: "start", IterationID: &target, ExpectedIterationRevision: &revision, ExpectedScopeRevision: &scope, ExpectedSettingsRevision: 2, Start: &iteration.StartDraft{TargetID: target, Mode: "scheduled", TerminalChoices: []iteration.TerminalChoice{}}})
	triageEnableForTest(t)
	item := triageCreateForTest(t)
	testutil.Call(t, h.ActOnTriageItem, withURLParam(newRequest("POST", "/api/triage/items/actions", triageIterationAcceptBody(item, target, "accept", uuid.NewString())), "id", item.Issue.ID)).Want(200)
	var detail struct {
		Statistics iteration.Statistics `json:"statistics"`
	}
	testutil.Call(t, h.GetIteration, lifecycleHTTPRequest("GET", "iterations/"+target, target, nil)).Want(200).JSON(&detail)
	if detail.Statistics.Original != 0 || detail.Statistics.Current != 1 || detail.Statistics.AddedUnique != 1 || detail.Statistics.OriginalRatio != nil {
		t.Fatalf("accepted scope changed fixed empty baseline: %+v", detail.Statistics)
	}
}
