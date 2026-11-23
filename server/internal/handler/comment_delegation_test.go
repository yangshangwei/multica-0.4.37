package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func delegationRequest(method, issue, key string, body any) *http.Request {
	req := withURLParam(newRequest(method, "/api/issues/"+issue+"/comments", body), "id", issue)
	req = testutil.WithURLParams(req, "id", issue, "key", key)
	req.Header.Set("Idempotency-Key", key)
	return req.WithContext(auth.WithAuthenticatedPrincipal(req.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}))
}
func delegationPost(t *testing.T, h *Handler, issue, key, content string, status int) CommentResponse {
	t.Helper()
	var out CommentResponse
	testutil.Call(t, h.CreateComment, delegationRequest("POST", issue, key, map[string]any{"content": content})).Want(status).JSON(&out)
	return out
}
func delegationRead(t *testing.T, h *Handler, issue, key string) DelegationReceipt {
	t.Helper()
	var out DelegationReceipt
	testutil.Call(t, h.GetDelegationReceipt, delegationRequest("GET", issue, key, nil)).Want(200).JSON(&out)
	return out
}
func delegationCleanup(t *testing.T, issue string) {
	t.Helper()
	t.Cleanup(func() {
		_, err := testPool.Exec(context.Background(), "DELETE FROM comment_delegation_receipt WHERE issue_id=$1", issue)
		if err != nil {
			t.Error(err)
		}
	})
}

func TestDurableDelegationActualQueueMergeDeferredAndRestart(t *testing.T) {
	agent := createHandlerTestAgent(t, "durable target", nil)
	issue := dbfx.Issue(t, "durable dispatch")
	delegationCleanup(t, issue)
	content := fmt.Sprintf("[@target](mention://agent/%s) do work", agent)
	key := uuidToString(dbid.NewV7())
	first := delegationPost(t, testHandler, issue, key, content, 201)
	if first.DelegationReceipt == nil || first.DelegationReceipt.State != "recorded" {
		t.Fatalf("missing recorded receipt: %+v", first)
	}
	out := findCommentOutcome(t, first.TriggerOutcomes, agent)
	if out.Status != DispatchQueued || out.TaskID == "" || out.ExecutingAgentID != agent {
		t.Fatalf("not actual queued identity: %+v", out)
	}
	task, err := testHandler.Queries.GetAgentTask(context.Background(), parseUUID(out.TaskID))
	if err != nil {
		t.Fatal(err)
	}
	if uuidToString(task.TriggerCommentID) != first.ID || uuidToString(task.IssueID) != issue || uuidToString(task.AgentID) != agent {
		t.Fatal("receipt does not match persisted task")
	}
	restarted := *testHandler
	restarted.Queries = db.New(testPool)
	recovered := delegationRead(t, &restarted, issue, key)
	if recovered.CommentID != first.ID || recovered.Targets[0].TaskID != out.TaskID {
		t.Fatalf("restart changed receipt: %+v", recovered)
	}
	replay := delegationPost(t, &restarted, issue, key, content, 200)
	if replay.ID != first.ID || countQueuedCommentTriggerTasks(t, issue, agent) != 1 {
		t.Fatal("replay dispatched again")
	}
	testutil.Call(t, testHandler.CreateComment, delegationRequest("POST", issue, key, map[string]any{"content": content + " changed"})).Want(409)
	merged := delegationPost(t, testHandler, issue, uuidToString(dbid.NewV7()), content+" followup", 201)
	merge := findCommentOutcome(t, merged.TriggerOutcomes, agent)
	if merge.Status != DispatchCoalesced || merge.TaskID != out.TaskID {
		t.Fatalf("missing atomic merge identity: %+v", merge)
	}
	if _, err = testPool.Exec(context.Background(), "UPDATE agent_task_queue SET status='dispatched', claim_generation=1, dispatched_at=now() WHERE id=$1", out.TaskID); err != nil {
		t.Fatal(err)
	}
	deferred := delegationPost(t, testHandler, issue, uuidToString(dbid.NewV7()), content+" after claim", 201)
	deferOut := findCommentOutcome(t, deferred.TriggerOutcomes, agent)
	if deferOut.Status != DispatchDeferred || deferOut.TaskID != out.TaskID {
		t.Fatalf("missing durable deferred identity: %+v", deferOut)
	}
	task, err = testHandler.Queries.GetAgentTask(context.Background(), parseUUID(out.TaskID))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, id := range task.CoalescedCommentIds {
		found = found || uuidToString(id) == deferred.ID
	}
	if !found {
		t.Fatal("deferred comment not durably attached")
	}
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='running' WHERE id=$1", out.TaskID)
	if w := completeTaskViaHandler(t, out.TaskID, "done"); w.Code != 200 {
		t.Fatalf("completion %d %s", w.Code, w.Body.String())
	}
	if n := pendingTaskCountForAgentIssue(t, issue, agent); n != 1 {
		t.Fatalf("recorded admission lost normal followup: %d", n)
	}
}

func TestDurableDelegationRejectionsAndFrozenTargets(t *testing.T) {
	private, _, _ := privateAgentTestFixture(t)
	unavailable := createHandlerTestAgent(t, "unbound target", nil)
	if _, err := testPool.Exec(context.Background(), "UPDATE agent SET runtime_id=NULL WHERE id=$1", unavailable); err != nil {
		t.Fatal(err)
	}
	issue := dbfx.Issue(t, "durable refusals")
	delegationCleanup(t, issue)
	key := uuidToString(dbid.NewV7())
	content := fmt.Sprintf("[@private](mention://agent/%s) [@unbound](mention://agent/%s)", private, unavailable)
	first := delegationPost(t, testHandler, issue, key, content, 201)
	for _, target := range []string{private, unavailable} {
		o := findCommentOutcome(t, first.TriggerOutcomes, target)
		if o.Status != DispatchBlocked || o.TaskID != "" {
			t.Fatalf("refusal fabricated task: %+v", o)
		}
	}
	if _, err := testPool.Exec(context.Background(), "UPDATE agent SET name='replacement display', runtime_id=$2 WHERE id=$1", unavailable, testRuntimeID); err != nil {
		t.Fatal(err)
	}
	after := delegationRead(t, testHandler, issue, key)
	if after.Targets[1].TargetID != unavailable || after.Targets[1].Status != DispatchBlocked {
		t.Fatalf("historical refusal rewritten: %+v", after)
	}
}

type delegationFinalizeFault struct{ db.DBTX }

func (f delegationFinalizeFault) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "-- name: RecordCommentDelegationReceipt") {
		return pgconn.CommandTag{}, errors.New("injected receipt finalization failure")
	}
	return f.DBTX.Exec(ctx, sql, args...)
}
func TestDurableDelegationUnknownDoesNotRedispatch(t *testing.T) {
	for _, mode := range []string{"claim_commit_after", "finalization"} {
		t.Run(mode, func(t *testing.T) {
			agent := createHandlerTestAgent(t, "unknown target", nil)
			issue := dbfx.Issue(t, "unknown receipt")
			delegationCleanup(t, issue)
			key := uuidToString(dbid.NewV7())
			content := fmt.Sprintf("[@target](mention://agent/%s)", agent)
			h := *testHandler
			if mode == "claim_commit_after" {
				h.TxStarter = &artifactFaultStarter{txStarter: h.TxStarter, failAt: 1, after: true}
				testutil.Call(t, h.CreateComment, delegationRequest("POST", issue, key, map[string]any{"content": content})).Want(503)
			} else {
				h.Queries = db.New(delegationFinalizeFault{DBTX: testPool})
				initial := delegationPost(t, &h, issue, key, content, 201)
				if initial.DelegationReceipt.State != "unknown" || initial.TriggerOutcomes[0].Status != "unknown" {
					t.Fatalf("unpersisted success leaked: %+v", initial)
				}
			}
			before := countQueuedCommentTriggerTasks(t, issue, agent)
			replay := delegationPost(t, testHandler, issue, key, content, 200)
			read := delegationRead(t, testHandler, issue, key)
			if replay.DelegationReceipt.State != "unknown" || read.State != "unknown" || read.Targets[0].Status != "unknown" || read.Targets[0].TaskID != "" {
				t.Fatalf("unknown changed: %+v %+v", replay, read)
			}
			if countQueuedCommentTriggerTasks(t, issue, agent) != before {
				t.Fatal("unknown redispatched")
			}
			want := 0
			if mode == "finalization" {
				want = 1
			}
			if before != want {
				t.Fatalf("actual queue count %d want%d", before, want)
			}
		})
	}
}

func TestDurableDelegationRejectsInvalidKeyAndAttachmentsBeforeWrite(t *testing.T) {
	issue := dbfx.Issue(t, "invalid durable request")
	delegationCleanup(t, issue)
	for _, key := range []string{"", "bad", "00000000-0000-0000-0000-000000000000"} {
		testutil.Call(t, testHandler.CreateComment, delegationRequest("POST", issue, key, map[string]any{"content": "note"})).Want(400)
	}
	key := uuidToString(dbid.NewV7())
	duplicate := delegationRequest("POST", issue, "", map[string]any{"content": "note"})
	duplicate.Header.Add("Idempotency-Key", key)
	testutil.Call(t, testHandler.CreateComment, duplicate).Want(400)
	testutil.Call(t, testHandler.CreateComment, delegationRequest("POST", issue, key, map[string]any{"content": "note", "attachment_ids": []string{uuidToString(dbid.NewV7())}})).Want(400)
	var count int
	if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM comment WHERE issue_id=$1", issue).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("invalid request wrote comment")
	}
}

func TestDurableDelegationUnknownCannotReconcileOnCompletion(t *testing.T) {
	for _, mode := range []string{"claim_commit_after", "finalization"} {
		t.Run(mode, func(t *testing.T) {
			agent := createHandlerTestAgent(t, "uncertain completion target", nil)
			issue := dbfx.Issue(t, "unknown completion", testutil.Cols{"status": "in_progress"})
			delegationCleanup(t, issue)
			task := dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": testRuntimeID, "status": "dispatched", "created_at": testutil.Raw("now() - interval '2 minutes'"), "dispatched_at": testutil.Raw("now() - interval '1 minute'"), "claim_generation": 1})
			key := uuidToString(dbid.NewV7())
			content := fmt.Sprintf("[@target](mention://agent/%s)", agent)
			h := *testHandler
			if mode == "claim_commit_after" {
				h.TxStarter = &artifactFaultStarter{txStarter: h.TxStarter, failAt: 1, after: true}
				testutil.Call(t, h.CreateComment, delegationRequest("POST", issue, key, map[string]any{"content": content})).Want(503)
			} else {
				h.Queries = db.New(delegationFinalizeFault{DBTX: testPool})
				delegationPost(t, &h, issue, key, content, 201)
			}
			dbfx.Exec(t, "UPDATE agent_task_queue SET status='running' WHERE id=$1", task)
			if w := completeTaskViaHandler(t, task, "done"); w.Code != 200 {
				t.Fatalf("completion %d %s", w.Code, w.Body.String())
			}
			var completed string
			dbfx.QueryRow(t, "SELECT status FROM agent_task_queue WHERE id=$1", task).Scan(&completed)
			if completed != "completed" {
				t.Fatalf("completion was not committed: %s", completed)
			}
			if n := pendingTaskCountForAgentIssue(t, issue, agent); n != 0 {
				t.Fatalf("unknown receipt automatically replayed into %d tasks", n)
			}
			if got := delegationRead(t, testHandler, issue, key); got.State != "unknown" {
				t.Fatalf("completion rewrote unknown %+v", got)
			}
		})
	}
}

func TestDurableDelegationUnknownSurvivorIsNotRequeued(t *testing.T) {
	agent := createHandlerTestAgent(t, "unknown survivor target", nil)
	issue := dbfx.Issue(t, "unknown survivor")
	delegationCleanup(t, issue)
	content := fmt.Sprintf("[@target](mention://agent/%s)", agent)
	var initial CommentResponse
	testutil.Call(t, testHandler.CreateComment, withURLParam(newRequest("POST", "/api/issues/"+issue+"/comments", map[string]any{"content": content + " initial"}), "id", issue)).Want(201).JSON(&initial)
	h := *testHandler
	h.Queries = db.New(delegationFinalizeFault{DBTX: testPool})
	key := uuidToString(dbid.NewV7())
	uncertain := delegationPost(t, &h, issue, key, content+" uncertain followup", 201)
	if uncertain.DelegationReceipt.State != "unknown" {
		t.Fatal("fixture did not retain unknown")
	}
	if got := pendingTaskCountForAgentIssue(t, issue, agent); got != 1 {
		t.Fatalf("fixture queue %d", got)
	}
	req := withURLParam(newRequest("DELETE", "/api/comments/"+initial.ID, nil), "commentId", initial.ID)
	testutil.Call(t, testHandler.DeleteComment, req).Want(204)
	if got := pendingTaskCountForAgentIssue(t, issue, agent); got != 0 {
		t.Fatalf("delete automatically requeued unknown survivor: %d", got)
	}
	if receipt := delegationRead(t, testHandler, issue, key); receipt.State != "unknown" || receipt.CommentID != uncertain.ID {
		t.Fatalf("survivor changed receipt %+v", receipt)
	}
}

func TestDurableDelegationReplayCannotAdoptReplacementLeader(t *testing.T) {
	original := createHandlerTestAgent(t, "original leader", nil)
	replacement := createHandlerTestAgent(t, "replacement leader", nil)
	squad := createCommentTriggerPreviewSquad(t, "durable squad", original)
	issue := dbfx.Issue(t, "replacement cannot inherit delegation")
	delegationCleanup(t, issue)
	replacementTask := dbfx.Task(t, replacement, testutil.Cols{"issue_id": issue, "runtime_id": testRuntimeID, "status": "dispatched", "created_at": testutil.Raw("now() - interval '2 minutes'"), "dispatched_at": testutil.Raw("now() - interval '1 minute'"), "claim_generation": 1})
	key := uuidToString(dbid.NewV7())
	first := delegationPost(t, testHandler, issue, key, fmt.Sprintf("[@squad](mention://squad/%s)", squad), 201)
	originalOut := findCommentOutcome(t, first.TriggerOutcomes, squad)
	if originalOut.ExecutingAgentID != original || originalOut.Status != DispatchQueued {
		t.Fatalf("original receipt %+v", originalOut)
	}
	dbfx.Exec(t, "UPDATE squad SET leader_id=$2 WHERE id=$1", squad, replacement)
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='running' WHERE id=$1", replacementTask)
	if w := completeTaskViaHandler(t, replacementTask, "done"); w.Code != 200 {
		t.Fatalf("completion %d %s", w.Code, w.Body.String())
	}
	if n := pendingTaskCountForAgentIssue(t, issue, replacement); n != 0 {
		t.Fatalf("replacement acquired historical delegation: %d", n)
	}
	if n := pendingTaskCountForAgentIssue(t, issue, original); n != 1 {
		t.Fatalf("original task changed: %d", n)
	}
	current := delegationRead(t, testHandler, issue, key)
	if !reflect.DeepEqual(current.Targets[0], originalOut) {
		t.Fatalf("receipt rewritten %+v", current)
	}
}

func TestDurableDelegationReplayCannotAdoptPreviouslyRefusedSquad(t *testing.T) {
	accepted := createHandlerTestAgent(t, "accepted agent", nil)
	private, _, _ := privateAgentTestFixture(t)
	squad := createCommentTriggerPreviewSquad(t, "previously refused squad", private)
	issue := dbfx.Issue(t, "refused squad cannot inherit agent admission")
	delegationCleanup(t, issue)
	task := dbfx.Task(t, accepted, testutil.Cols{"issue_id": issue, "runtime_id": testRuntimeID, "status": "dispatched", "created_at": testutil.Raw("now() - interval '2 minutes'"), "dispatched_at": testutil.Raw("now() - interval '1 minute'"), "claim_generation": 1})
	key := uuidToString(dbid.NewV7())
	content := fmt.Sprintf("[@refused](mention://squad/%s) [@accepted](mention://agent/%s)", squad, accepted)
	first := delegationPost(t, testHandler, issue, key, content, 201)
	if findCommentOutcome(t, first.TriggerOutcomes, squad).Status != DispatchBlocked || findCommentOutcome(t, first.TriggerOutcomes, accepted).Status != DispatchDeferred {
		t.Fatalf("incorrect admission fixture %+v", first.TriggerOutcomes)
	}
	dbfx.Exec(t, "UPDATE squad SET leader_id=$2 WHERE id=$1", squad, accepted)
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='running' WHERE id=$1", task)
	if w := completeTaskViaHandler(t, task, "done"); w.Code != 200 {
		t.Fatalf("completion %d %s", w.Code, w.Body.String())
	}
	var completed string
	dbfx.QueryRow(t, "SELECT status FROM agent_task_queue WHERE id=$1", task).Scan(&completed)
	if completed != "completed" {
		t.Fatalf("not completed %s", completed)
	}
	if n := pendingTaskCountForAgentIssue(t, issue, accepted); n != 0 {
		t.Fatalf("original agent admission granted new squad context: %d", n)
	}
}

func TestDurableDelegationDirectAgentDeferredOnSquadTaskKeepsFollowup(t *testing.T) {
	agent := createHandlerTestAgent(t, "squad task direct recipient", nil)
	squad := createCommentTriggerPreviewSquad(t, "existing squad context", agent)
	issue := dbfx.Issue(t, "direct intent on squad receiver")
	delegationCleanup(t, issue)
	task := dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": testRuntimeID, "squad_id": squad, "is_leader_task": true, "status": "dispatched", "created_at": testutil.Raw("now() - interval '2 minutes'"), "dispatched_at": testutil.Raw("now() - interval '1 minute'"), "claim_generation": 1})
	key := uuidToString(dbid.NewV7())
	first := delegationPost(t, testHandler, issue, key, fmt.Sprintf("[@agent](mention://agent/%s)", agent), 201)
	outcome := findCommentOutcome(t, first.TriggerOutcomes, agent)
	if outcome.Status != DispatchDeferred || outcome.TaskID != task || outcome.ExecutingSquadID != squad {
		t.Fatalf("wrong actual covering task %+v", outcome)
	}
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='running' WHERE id=$1", task)
	if w := completeTaskViaHandler(t, task, "done"); w.Code != 200 {
		t.Fatalf("completion %d %s", w.Code, w.Body.String())
	}
	var completed string
	dbfx.QueryRow(t, "SELECT status FROM agent_task_queue WHERE id=$1", task).Scan(&completed)
	if completed != "completed" {
		t.Fatalf("not completed %s", completed)
	}
	if n := pendingTaskCountForAgentIssue(t, issue, agent); n != 1 {
		t.Fatalf("original direct-agent admission lost its followup: %d", n)
	}
	var squadCount int
	dbfx.QueryRow(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND status='queued' AND squad_id IS NOT NULL", issue).Scan(&squadCount)
	if squadCount != 0 {
		t.Fatal("direct followup inherited unrelated covering squad")
	}
}

type delegationScopeRaceStarter struct {
	txStarter
	target    string
	afterRead func()
}

func (s delegationScopeRaceStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.txStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &delegationScopeRaceTx{Tx: tx, target: s.target, afterRead: s.afterRead}, nil
}

type delegationScopeRaceTx struct {
	pgx.Tx
	target    string
	afterRead func()
}

func (s *delegationScopeRaceTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := s.Tx.QueryRow(ctx, sql, args...)
	if strings.HasPrefix(sql, "-- name: GetIssue :one") && len(args) > 0 {
		if id, ok := args[0].(pgtype.UUID); ok && uuidToString(id) == s.target {
			return delegationScopeRaceRow{Row: row, after: s.afterRead}
		}
	}
	return row
}

type delegationScopeRaceRow struct {
	pgx.Row
	after func()
}

func (r delegationScopeRaceRow) Scan(dest ...any) error {
	err := r.Row.Scan(dest...)
	if err == nil {
		r.after()
	}
	return err
}

func TestDurableDelegationReporterLocksTargetBeforeScopeRead(t *testing.T) {
	f := newTaskCapabilityFixture(t, "progress-reporter", 2, false)
	target := f.issue
	delegationCleanup(t, target)
	token, err := testHandler.Queries.GetTaskTokenByHash(t.Context(), auth.HashToken(f.token))
	if err != nil {
		t.Fatal(err)
	}
	key := uuidToString(dbid.NewV7())
	request := func(token db.TaskToken, method string, body any) *http.Request {
		r := delegationRequest(method, target, key, body)
		r.Header.Set("X-Actor-Source", "task_token")
		r.Header.Set("X-Agent-ID", f.agent)
		r.Header.Set("X-Task-ID", uuidToString(token.TaskID))
		return r.WithContext(auth.WithVerifiedTaskToken(auth.WithAuthenticatedPrincipal(r.Context(), auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialTask}), token))
	}
	var first CommentResponse
	testutil.Call(t, testHandler.CreateComment, request(token, "POST", map[string]any{"content": "retained report"})).Want(201).JSON(&first)
	source := dbfx.Issue(t, "new reporter source", testutil.Cols{"project_id": f.project})
	sourceTask := dbfx.Task(t, f.agent, testutil.Cols{"issue_id": source, "runtime_id": f.runtime, "status": "dispatched", "claim_generation": 1, "originator_user_id": testUserID, "accountable_user_id": testUserID})
	capability, err := json.Marshal(auth.TaskCapability{Version: 1, Profile: auth.TaskCapabilityReporter, IssueID: source, ProjectID: f.project, OriginatorID: testUserID, RuntimeID: f.runtime, ClaimGeneration: 1})
	if err != nil {
		t.Fatal(err)
	}
	current, err := testHandler.Queries.CreateTaskToken(t.Context(), db.CreateTaskTokenParams{TokenHash: auth.HashToken(uuidToString(dbid.NewV7())), TaskID: parseUUID(sourceTask), AgentID: parseUUID(f.agent), WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(testUserID), ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}, ApiCapability: capability})
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, "DELETE FROM task_token WHERE id=$1", current.ID)
	outside := dbfx.Project(t, "outside reporter scope")
	attempts := 0
	var moveErr error
	h := *testHandler
	h.TxStarter = delegationScopeRaceStarter{txStarter: h.TxStarter, target: target, afterRead: func() {
		attempts++
		tx, e := testPool.Begin(t.Context())
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(t.Context())
		if _, e = tx.Exec(t.Context(), "SET LOCAL lock_timeout='100ms'"); e != nil {
			t.Fatal(e)
		}
		_, moveErr = tx.Exec(t.Context(), "UPDATE issue SET project_id=$2 WHERE id=$1", target, outside)
		if moveErr == nil {
			if e = tx.Commit(t.Context()); e != nil {
				t.Fatal(e)
			}
		}
	}}
	var read DelegationReceipt
	testutil.Call(t, h.GetDelegationReceipt, request(current, "GET", nil)).Want(200).JSON(&read)
	var pgerr *pgconn.PgError
	if attempts != 1 || !errors.As(moveErr, &pgerr) || pgerr.Code != "55P03" {
		t.Fatalf("scope was read before target lock: attempts=%d move=%v", attempts, moveErr)
	}
	if read.CommentID != first.ID {
		t.Fatal("same-agent restarted reporter could not recover retained receipt")
	}
	dbfx.Exec(t, "UPDATE issue SET project_id=$2 WHERE id=$1", target, outside)
	testutil.Call(t, testHandler.GetDelegationReceipt, request(current, "GET", nil)).Want(403)
}
