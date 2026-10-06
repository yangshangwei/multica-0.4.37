package handler

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type squadIterationFixture struct {
	workspaceID string
	leaderID    string
	squadID     string
	issueIDs    []string
	iterations  []string
	autopilotID string
	taskID      string
}

func newSquadIterationFixture(t *testing.T) squadIterationFixture {
	t.Helper()
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	x := squadIterationFixture{workspaceID: dbfx.Workspace(t, "Squad iteration transfer", "squad-i1-"+uuid.NewString())}
	f := testutil.New(testPool, x.workspaceID, testUserID)
	f.Member(t, x.workspaceID, testUserID, "owner")
	runtimeID := f.Runtime(t, "Squad iteration runtime")
	x.leaderID = f.Agent(t, "Squad iteration leader", runtimeID)
	x.squadID = f.Squad(t, "Squad iteration transfer", x.leaderID)
	for _, status := range []string{"active", "planned"} {
		id := f.Insert(t, "iteration", testutil.Cols{"workspace_id": x.workspaceID, "name": status, "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-14", "status": status, "created_by": testUserID})
		x.iterations = append(x.iterations, id)
		f.Cleanup(t, "DELETE FROM iteration_event WHERE iteration_id=$1", id)
	}
	for i := 0; i < 4; i++ {
		cols := testutil.Cols{"assignee_type": "squad", "assignee_id": x.squadID, "iteration_rollover_count": 2}
		if i < 3 {
			cols["current_iteration_id"] = x.iterations[i/2]
		} else {
			cols["admission_status"] = "pending"
		}
		id := f.Issue(t, "Squad iteration commitment", cols)
		x.issueIDs = append(x.issueIDs, id)
		if i < 3 {
			f.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": x.workspaceID, "iteration_id": x.iterations[i/2], "issue_id": id, "first_joined_at": testutil.Raw("clock_timestamp()"), "current_joined_at": testutil.Raw("clock_timestamp()"), "in_original": i < 2, "original_facts": `{"title":"frozen commitment"}`}, "iteration_id=$1 AND issue_id=$2", x.iterations[i/2], id)
		}
	}
	x.autopilotID = f.Insert(t, "autopilot", testutil.Cols{"workspace_id": x.workspaceID, "title": "Transfer with squad", "assignee_type": "squad", "assignee_id": x.squadID, "created_by_type": "member", "created_by_id": testUserID})
	x.taskID = f.Task(t, x.leaderID, testutil.Cols{"runtime_id": runtimeID, "issue_id": x.issueIDs[0], "status": "running", "started_at": testutil.Raw("clock_timestamp()"), "context": `{"execution_identity":"must-stay"}`})
	return x
}

func (x squadIterationFixture) request() *http.Request {
	r := testutil.WithURLParams(newRequest(http.MethodDelete, "/api/workspaces/"+x.workspaceID+"/squads/"+x.squadID, nil), "workspaceId", x.workspaceID, "id", x.squadID)
	r.Header.Set("X-Workspace-ID", x.workspaceID)
	return r
}

func squadIterationState(t *testing.T, x squadIterationFixture) string {
	t.Helper()
	var state string
	dbfx.QueryRow(t, `SELECT jsonb_build_object(
 'issues',(SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM issue i WHERE workspace_id=$1),
 'iterations',(SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM iteration i WHERE workspace_id=$1),
 'participations',(SELECT jsonb_agg(to_jsonb(p) ORDER BY issue_id) FROM iteration_participation p WHERE workspace_id=$1),
 'events',(SELECT jsonb_agg(to_jsonb(e) ORDER BY iteration_id,sequence) FROM iteration_event e WHERE workspace_id=$1),
 'autopilots',(SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM autopilot a WHERE workspace_id=$1),
 'squad',(SELECT to_jsonb(s) FROM squad s WHERE id=$2),
 'task',(SELECT to_jsonb(t) FROM agent_task_queue t WHERE id=$3))::text`, x.workspaceID, x.squadID, x.taskID).Scan(&state)
	return state
}

func TestDeleteSquadIterationAtomicTransfer(t *testing.T) {
	x := newSquadIterationFixture(t)
	f := testutil.New(testPool, x.workspaceID, testUserID)
	otherSquad := f.Squad(t, "Unrelated squad", x.leaderID)
	otherIssue := f.Issue(t, "Unrelated assignment", testutil.Cols{"assignee_type": "squad", "assignee_id": otherSquad})
	foreignWS := f.Workspace(t, "Foreign transfer guard", "squad-i1-other-"+uuid.NewString())
	foreign := testutil.New(testPool, foreignWS, testUserID)
	foreignIssue := foreign.Issue(t, "Foreign same UUID reference", testutil.Cols{"assignee_type": "squad", "assignee_id": x.squadID})
	foreignAutopilot := foreign.Insert(t, "autopilot", testutil.Cols{"workspace_id": foreignWS, "title": "Foreign same UUID reference", "assignee_type": "squad", "assignee_id": x.squadID, "created_by_type": "member", "created_by_id": testUserID})
	var taskBefore, taskAfter string
	dbfx.QueryRow(t, `SELECT to_jsonb(t)::text FROM agent_task_queue t WHERE id=$1`, x.taskID).Scan(&taskBefore)
	testutil.Call(t, testHandler.DeleteSquad, x.request()).Want(http.StatusNoContent)
	if n := f.Count(t, `SELECT count(*) FROM iteration_event WHERE workspace_id=$1`, x.workspaceID); n != 3 {
		t.Errorf("transfer must record all three participating issues, got %d events", n)
	}
	if n := f.Count(t, `SELECT count(DISTINCT sampled_at) FROM iteration_event WHERE workspace_id=$1`, x.workspaceID); n != 1 {
		t.Errorf("one transfer must share one lock-time sample, got %d", n)
	}
	if n := f.Count(t, `SELECT count(DISTINCT operation_id) FROM iteration_event WHERE workspace_id=$1`, x.workspaceID); n != 1 {
		t.Errorf("one transfer must share one operation identity, got %d", n)
	}
	for i, id := range x.issueIDs {
		var assignee, typ, pointer string
		var revision, rollover int
		f.QueryRow(t, `SELECT assignee_type,assignee_id,revision,iteration_rollover_count,COALESCE(current_iteration_id::text,'') FROM issue WHERE id=$1`, id).Scan(&typ, &assignee, &revision, &rollover, &pointer)
		if typ != "agent" || assignee != x.leaderID || revision != 2 || rollover != 2 || (i < 3 && pointer != x.iterations[i/2]) || (i == 3 && pointer != "") {
			t.Errorf("issue transfer lost atomic fields: %s %s revision=%d rollover=%d pointer=%s", typ, assignee, revision, rollover, pointer)
		}
	}
	for i, id := range x.iterations {
		var scope int
		f.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, id).Scan(&scope)
		if scope != 3-i {
			t.Errorf("iteration %d scope=%d, want %d", i, scope, 3-i)
		}
	}
	if n := f.Count(t, `SELECT count(*) FROM iteration_event WHERE workspace_id=$1 AND actor->>'type'='member' AND actor->>'id'=$2 AND actor->>'user_id'=$2 AND before_facts->>'assignee_type'='squad' AND before_facts->>'assignee_id'=$3 AND after_facts->>'assignee_type'='agent' AND after_facts->>'assignee_id'=$4`, x.workspaceID, testUserID, x.squadID, x.leaderID); n != 3 {
		t.Errorf("fact and actor payloads differ from the real transfer: %d matching", n)
	}
	if n := f.Count(t, `SELECT count(*) FROM iteration_participation WHERE workspace_id=$1 AND original_facts='{"title":"frozen commitment"}'::jsonb AND current_joined_at IS NOT NULL AND NOT has_started_current_participation`, x.workspaceID); n != 3 {
		t.Errorf("transfer changed original/participation facts: %d intact", n)
	}
	for _, target := range []struct{ table, id, assignee string }{{"issue", otherIssue, otherSquad}, {"issue", foreignIssue, x.squadID}, {"autopilot", foreignAutopilot, x.squadID}} {
		var typ, assignee string
		f.QueryRow(t, "SELECT assignee_type,assignee_id FROM "+target.table+" WHERE id=$1", target.id).Scan(&typ, &assignee)
		if typ != "squad" || assignee != target.assignee {
			t.Errorf("unrelated %s row changed: type=%s assignee=%s", target.table, typ, assignee)
		}
	}
	if n := f.Count(t, `SELECT count(*) FROM autopilot WHERE id=$1 AND assignee_type='agent' AND assignee_id=$2 AND status='active'`, x.autopilotID, x.leaderID); n != 1 {
		t.Error("owned autopilot was not transferred with its status intact")
	}
	if n := f.Count(t, `SELECT count(*) FROM squad WHERE id=$1 AND archived_at IS NOT NULL AND archived_by=$2`, x.squadID, testUserID); n != 1 {
		t.Error("squad was not archived by the actual user")
	}
	f.QueryRow(t, `SELECT to_jsonb(t)::text FROM agent_task_queue t WHERE id=$1`, x.taskID).Scan(&taskAfter)
	if taskBefore != taskAfter || f.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, x.leaderID) != 1 {
		t.Fatal("management transfer changed execution identity or task count")
	}
}

type squadIterationFailureStarter struct {
	inner  txStarter
	failAt string
}

func (s squadIterationFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &squadIterationFailureTx{Tx: tx, failAt: s.failAt}, nil
}

type squadIterationFailureTx struct {
	pgx.Tx
	appends int
	failAt  string
}

func (tx *squadIterationFailureTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "AppendIterationIssueEvent") {
		tx.appends++
		if tx.appends == 2 && tx.failAt == "event" {
			return pgconn.CommandTag{}, errors.New("injected second squad iteration event failure")
		}
	}
	if tx.failAt == "autopilot" && strings.Contains(sql, "TransferSquadAutopilotsToLeader") {
		return pgconn.CommandTag{}, errors.New("injected squad autopilot transfer failure")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func (tx *squadIterationFailureTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx.failAt == "archive" && strings.Contains(sql, "ArchiveSquad") {
		return errRow{err: errors.New("injected squad archive failure")}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func TestDeleteSquadIterationFailureRollsBackEntireTransfer(t *testing.T) {
	for _, failure := range []string{"event", "autopilot", "archive"} {
		t.Run(failure, func(t *testing.T) {
			x := newSquadIterationFixture(t)
			before := squadIterationState(t, x)
			h := *testHandler
			h.TxStarter = squadIterationFailureStarter{inner: h.TxStarter, failAt: failure}
			testutil.Call(t, h.DeleteSquad, x.request()).Want(http.StatusInternalServerError)
			if after := squadIterationState(t, x); before != after {
				t.Error("failed transfer changed issue, autopilot, archive, iteration or execution state")
			}
		})
	}
}

func TestDeleteSquadIterationRevalidatesCurrentAuthority(t *testing.T) {
	for _, change := range []string{"revoked", "demoted", "archived", "leader_archived", "agent_autonomy", "agent_archived"} {
		t.Run(change, func(t *testing.T) {
			x := newSquadIterationFixture(t)
			f := testutil.New(testPool, x.workspaceID, testUserID)
			r := x.request()
			var actorID string
			if strings.HasPrefix(change, "agent_") {
				runtimeID := f.Runtime(t, "Current coordinator runtime")
				actorID = f.Agent(t, "Current coordinator", runtimeID, testutil.Cols{"autonomy_level": "coordinator"})
				taskID := f.Task(t, actorID, testutil.Cols{"runtime_id": runtimeID, "status": "running"})
				r = asAgent(r, actorID, taskID)
			}
			if change == "demoted" {
				creator := f.User(t, "Other squad creator", "squad-creator-"+uuid.NewString()+"@multica.test")
				f.Exec(t, `UPDATE squad SET creator_id=$1 WHERE id=$2`, creator, x.squadID)
			}
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			reached, release := make(chan struct{}, 1), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			h := *testHandler
			h.TxStarter = iterationBeginBarrier{h.TxStarter, reached, release}
			result := make(chan *testutil.Response, 1)
			go func() { result <- testutil.Call(t, h.DeleteSquad, r.WithContext(ctx)) }()
			select {
			case <-reached:
			case response := <-result:
				t.Fatalf("delete bypassed transactional reauthorization: %d %s", response.Code, response.Body.String())
			case <-ctx.Done():
				t.Fatal("delete did not reach its transaction")
			}
			want := http.StatusForbidden
			switch change {
			case "revoked":
				f.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, x.workspaceID, testUserID)
			case "demoted":
				f.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, x.workspaceID, testUserID)
			case "archived":
				f.Exec(t, `UPDATE squad SET archived_at=clock_timestamp() WHERE id=$1`, x.squadID)
				want = http.StatusBadRequest
			case "leader_archived":
				f.Exec(t, `UPDATE agent SET archived_at=clock_timestamp() WHERE id=$1`, x.leaderID)
				want = http.StatusBadRequest
			case "agent_autonomy":
				f.Exec(t, `UPDATE agent SET autonomy_level='contributor' WHERE id=$1`, actorID)
			case "agent_archived":
				f.Exec(t, `UPDATE agent SET archived_at=clock_timestamp() WHERE id=$1`, actorID)
			}
			before := squadIterationState(t, x)
			once.Do(func() { close(release) })
			select {
			case response := <-result:
				response.Want(want)
			case <-ctx.Done():
				t.Fatal("delete did not complete after reauthorization")
			}
			if after := squadIterationState(t, x); before != after {
				t.Fatal("revoked authority or archived squad still changed transfer facts")
			}
		})
	}
}

func TestDeleteSquadIterationUsesCurrentLeader(t *testing.T) {
	x := newSquadIterationFixture(t)
	f := testutil.New(testPool, x.workspaceID, testUserID)
	leaderID := f.Agent(t, "Rotated squad leader", "")
	r := x.request()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	reached, release := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	h := *testHandler
	h.TxStarter = iterationBeginBarrier{h.TxStarter, reached, release}
	result := make(chan *testutil.Response, 1)
	go func() { result <- testutil.Call(t, h.DeleteSquad, r.WithContext(ctx)) }()
	select {
	case <-reached:
	case response := <-result:
		t.Fatalf("delete used a preflight leader without an owning transaction: %d", response.Code)
	case <-ctx.Done():
		t.Fatal("delete did not reach transaction")
	}
	f.Exec(t, `UPDATE squad SET leader_id=$1 WHERE id=$2`, leaderID, x.squadID)
	once.Do(func() { close(release) })
	select {
	case response := <-result:
		response.Want(http.StatusNoContent)
	case <-ctx.Done():
		t.Fatal("delete did not finish")
	}
	if n := f.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND assignee_type='agent' AND assignee_id=$2`, x.workspaceID, leaderID); n != 4 {
		t.Errorf("transfer used stale leader for %d issues", 4-n)
	}
	if n := f.Count(t, `SELECT count(*) FROM autopilot WHERE id=$1 AND assignee_type='agent' AND assignee_id=$2`, x.autopilotID, leaderID); n != 1 {
		t.Error("autopilot transfer used stale leader")
	}
}

type squadIterationObservedStarter struct {
	inner     txStarter
	statement string
	reached   chan<- uint32
}

func (s squadIterationObservedStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &squadIterationObservedTx{Tx: tx, statement: s.statement, reached: s.reached}, nil
}

type squadIterationObservedTx struct {
	pgx.Tx
	statement string
	reached   chan<- uint32
}

func (tx *squadIterationObservedTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, tx.statement) {
		tx.reached <- tx.Conn().PgConn().PID()
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func (tx *squadIterationObservedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, tx.statement) {
		tx.reached <- tx.Conn().PgConn().PID()
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func squadIterationWaitBlocked(t *testing.T, ctx context.Context, reached <-chan uint32, result <-chan *testutil.Response) {
	t.Helper()
	var pid uint32
	select {
	case pid = <-reached:
	case response := <-result:
		t.Fatalf("archive finished before lock acquisition: %d %s", response.Code, response.Body.String())
	case <-ctx.Done():
		t.Fatal("archive did not reach expected lock")
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := testPool.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return
		}
		select {
		case response := <-result:
			t.Fatalf("archive bypassed a concurrent owner lock: %d %s", response.Code, response.Body.String())
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("archive did not wait for the concurrent transaction")
		}
	}
}

func TestDeleteSquadIterationIncludesPriorAssociation(t *testing.T) {
	x := newSquadIterationFixture(t)
	f := testutil.New(testPool, x.workspaceID, testUserID)
	incoming := f.Issue(t, "Concurrent squad association", testutil.Cols{"status": "backlog"})
	deleterUser := f.User(t, "Concurrent archive admin", "squad-i1-admin-"+uuid.NewString()+"@multica.test")
	f.Member(t, x.workspaceID, deleterUser, "admin")
	r := withURLParam(newRequest(http.MethodPut, "/api/issues/"+incoming, map[string]any{"assignee_type": "squad", "assignee_id": x.squadID}), "id", incoming)
	r.Header.Set("X-Workspace-ID", x.workspaceID)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	reached, release := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	writer := *testHandler
	writer.TxStarter = projectAssociationCommitStarter{base: writer.TxStarter, reached: reached, release: release, once: &sync.Once{}}
	writeResult := make(chan *testutil.Response, 1)
	go func() { writeResult <- testutil.Call(t, writer.UpdateIssue, r.WithContext(ctx)) }()
	select {
	case <-reached:
	case response := <-writeResult:
		t.Fatalf("association stopped before commit: %d %s", response.Code, response.Body.String())
	case <-ctx.Done():
		t.Fatal("association did not reach commit")
	}
	deleteReached := make(chan uint32, 1)
	deleter := *testHandler
	deleter.TxStarter = squadIterationObservedStarter{inner: deleter.TxStarter, statement: iteration.WorkspaceFenceSQL, reached: deleteReached}
	deleteResult := make(chan *testutil.Response, 1)
	deleteReq := x.request()
	deleteReq.Header.Set("X-User-ID", deleterUser)
	deleteCtx, deleteCancel := context.WithTimeout(deleteReq.Context(), 10*time.Second)
	defer deleteCancel()
	go func() { deleteResult <- testutil.Call(t, deleter.DeleteSquad, deleteReq.WithContext(deleteCtx)) }()
	squadIterationWaitBlocked(t, ctx, deleteReached, deleteResult)
	probe, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Rollback(context.Background())
	if _, err = probe.Exec(ctx, `SELECT id FROM issue WHERE id=$1 FOR UPDATE NOWAIT`, x.issueIDs[0]); err != nil {
		t.Fatalf("archive locked issues before the I1 fence: %v", err)
	}
	if _, err = probe.Exec(ctx, `SELECT id FROM autopilot WHERE id=$1 FOR UPDATE NOWAIT`, x.autopilotID); err != nil {
		t.Fatalf("archive locked autopilot before the I1 fence: %v", err)
	}
	if err = probe.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	once.Do(func() { close(release) })
	(<-writeResult).Want(http.StatusOK)
	(<-deleteResult).Want(http.StatusNoContent)
	if n := f.Count(t, `SELECT count(*) FROM issue WHERE id=$1 AND assignee_type='agent' AND assignee_id=$2 AND revision=3`, incoming, x.leaderID); n != 1 {
		t.Fatal("archive missed the issue association that committed before its fence")
	}
}

func TestDeleteSquadIterationKeepsCreatorAndAgentIdentity(t *testing.T) {
	for _, kind := range []string{"creator", "agent"} {
		t.Run(kind, func(t *testing.T) {
			x := newSquadIterationFixture(t)
			r := x.request()
			actorType, actorID := "member", testUserID
			if kind == "creator" {
				dbfx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, x.workspaceID, testUserID)
			} else {
				r = asAgent(r, x.leaderID, x.taskID)
				actorType, actorID = "agent", x.leaderID
			}
			testutil.Call(t, testHandler.DeleteSquad, r).Want(http.StatusNoContent)
			if n := dbfx.Count(t, `SELECT count(*) FROM iteration_event WHERE workspace_id=$1 AND actor->>'type'=$2 AND actor->>'id'=$3 AND actor->>'user_id'=$4`, x.workspaceID, actorType, actorID, testUserID); n != 3 {
				t.Fatalf("archive did not retain authorized %s identity: %d events", kind, n)
			}
		})
	}
}

func TestDeleteSquadIterationSerializesAutopilotAssociation(t *testing.T) {
	for _, first := range []string{"association", "archive"} {
		t.Run(first, func(t *testing.T) {
			x := newSquadIterationFixture(t)
			createReq := newRequest(http.MethodPost, "/api/autopilots", map[string]any{"title": "Concurrent squad autopilot", "assignee_type": "squad", "assignee_id": x.squadID, "execution_mode": "create_issue"})
			createReq.Header.Set("X-Workspace-ID", x.workspaceID)
			ctx, cancel := context.WithTimeout(createReq.Context(), 10*time.Second)
			defer cancel()
			createReq = createReq.WithContext(ctx)
			deleteReq := x.request()
			deleteCtx, deleteCancel := context.WithTimeout(deleteReq.Context(), 10*time.Second)
			defer deleteCancel()
			deleteReq = deleteReq.WithContext(deleteCtx)
			firstHandler, secondHandler := *testHandler, *testHandler
			reached, release := make(chan struct{}, 1), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			firstHandler.TxStarter = projectAssociationCommitStarter{base: firstHandler.TxStarter, reached: reached, release: release, once: &sync.Once{}}
			firstResult, secondResult := make(chan *testutil.Response, 1), make(chan *testutil.Response, 1)
			lockReached := make(chan uint32, 1)
			if first == "association" {
				secondHandler.TxStarter = squadIterationObservedStarter{inner: secondHandler.TxStarter, statement: "LockSquadForUpdate", reached: lockReached}
				go func() { firstResult <- testutil.Call(t, firstHandler.CreateAutopilot, createReq) }()
			} else {
				secondHandler.TxStarter = squadIterationObservedStarter{inner: secondHandler.TxStarter, statement: "LockSquadForAutopilotAssignment", reached: lockReached}
				go func() { firstResult <- testutil.Call(t, firstHandler.DeleteSquad, deleteReq) }()
			}
			select {
			case <-reached:
			case response := <-firstResult:
				t.Fatalf("first operation stopped before commit: %d %s", response.Code, response.Body.String())
			case <-ctx.Done():
				t.Fatal("first operation did not reach commit")
			}
			if first == "association" {
				go func() { secondResult <- testutil.Call(t, secondHandler.DeleteSquad, deleteReq) }()
			} else {
				go func() { secondResult <- testutil.Call(t, secondHandler.CreateAutopilot, createReq) }()
				probe, err := testPool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				_, err = probe.Exec(ctx, `SELECT id FROM agent WHERE id=$1 FOR NO KEY UPDATE NOWAIT`, x.leaderID)
				_ = probe.Rollback(context.Background())
				var pgerr *pgconn.PgError
				if !errors.As(err, &pgerr) || pgerr.Code != "55P03" {
					t.Fatalf("archive did not retain current leader stability through commit: %v", err)
				}
			}
			squadIterationWaitBlocked(t, ctx, lockReached, secondResult)
			once.Do(func() { close(release) })
			if first == "association" {
				var created AutopilotResponse
				(<-firstResult).Want(http.StatusCreated).JSON(&created)
				cleanupTemplateAutopilot(t, created.ID)
				(<-secondResult).Want(http.StatusNoContent)
				if n := dbfx.Count(t, `SELECT count(*) FROM autopilot WHERE id=$1 AND assignee_type='agent' AND assignee_id=$2`, created.ID, x.leaderID); n != 1 {
					t.Fatal("archive missed the Autopilot committed before its squad lock")
				}
			} else {
				(<-firstResult).Want(http.StatusNoContent)
				(<-secondResult).Want(http.StatusUnprocessableEntity)
				if n := dbfx.Count(t, `SELECT count(*) FROM autopilot WHERE workspace_id=$1`, x.workspaceID); n != 1 {
					t.Fatal("an Autopilot binding entered after the squad archive")
				}
			}
		})
	}
}

func TestDeleteSquadIterationSamplesAfterLastParticipationLock(t *testing.T) {
	x := newSquadIterationFixture(t)
	participating := append([]string(nil), x.issueIDs[:3]...)
	sort.Strings(participating)
	r := x.request()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	blocker, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(context.Background())
	if _, err = blocker.Exec(ctx, `SELECT issue_id FROM iteration_participation WHERE issue_id=$1 FOR UPDATE`, participating[2]); err != nil {
		t.Fatal(err)
	}
	reached := make(chan uint32, 3)
	h := *testHandler
	h.TxStarter = squadIterationObservedStarter{inner: h.TxStarter, statement: "LockIssueIterationParticipation", reached: reached}
	result := make(chan *testutil.Response, 1)
	go func() { result <- testutil.Call(t, h.DeleteSquad, r.WithContext(ctx)) }()
	squadIterationWaitBlocked(t, ctx, reached, result)
	var cutoff time.Time
	if err = testPool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&cutoff); err != nil {
		t.Fatal(err)
	}
	if err = blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	(<-result).Want(http.StatusNoContent)
	var earliest, latest time.Time
	dbfx.QueryRow(t, `SELECT min(sampled_at),max(sampled_at) FROM iteration_event WHERE workspace_id=$1`, x.workspaceID).Scan(&earliest, &latest)
	if earliest.Before(cutoff) || !earliest.Equal(latest) {
		t.Fatalf("batch sample was not one post-lock instant: earliest=%s latest=%s final-lock-cutoff=%s", earliest, latest, cutoff)
	}
}
