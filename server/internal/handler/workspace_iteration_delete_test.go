package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/testutil"
)

var iterationWorkspaceTables = []string{
	"workspace_iteration_settings", "iteration", "iteration_participation",
	"iteration_event", "iteration_snapshot", "iteration_operation", "iteration_notification", "inbox_item",
}

func workspaceIterationHistoryFixture(t *testing.T) string {
	t.Helper()
	if testHandler == nil {
		t.Skip("database not available")
	}
	ws := dbfx.Workspace(t, "Iteration deletion", "i1-delete-"+uuid.NewString())
	f := testutil.New(testPool, ws, testUserID)
	f.Member(t, ws, testUserID, "owner")
	f.InsertNoID(t, "workspace_iteration_settings", testutil.Cols{"workspace_id": ws, "enabled": true}, "workspace_id=$1", ws)
	iterationID := f.Insert(t, "iteration", testutil.Cols{"workspace_id": ws, "name": "Retained history", "timezone": "UTC", "start_date": "2026-10-01", "end_date": "2026-10-14", "status": "completed", "created_by": testUserID})
	issueID := f.Issue(t, "Private historical title")
	operationID := f.Insert(t, "iteration_operation", testutil.Cols{"workspace_id": ws, "actor_user_id": testUserID, "request_id": uuid.NewString(), "operation": "end", "payload_hash": "test-hash", "result": `{"retained":"private result"}`})
	f.InsertNoID(t, "iteration_participation", testutil.Cols{"workspace_id": ws, "iteration_id": iterationID, "issue_id": issueID, "first_joined_at": testutil.Raw("clock_timestamp()"), "in_original": true, "original_facts": `{"title":"Private historical title"}`}, "workspace_id=$1", ws)
	f.Insert(t, "iteration_event", testutil.Cols{"workspace_id": ws, "iteration_id": iterationID, "issue_id": issueID, "sequence": 1, "operation_id": operationID, "kind": "end", "actor": `{"type":"member"}`, "occurred_at": testutil.Raw("clock_timestamp()"), "sampled_at": testutil.Raw("clock_timestamp()")})
	f.InsertNoID(t, "iteration_snapshot", testutil.Cols{"workspace_id": ws, "iteration_id": iterationID, "operation_id": operationID, "body": `{"title":"Private historical title"}`, "created_at": testutil.Raw("clock_timestamp()")}, "workspace_id=$1", ws)
	noticeID := f.Insert(t, "iteration_notification", testutil.Cols{"workspace_id": ws, "iteration_id": iterationID, "operation_id": operationID, "recipient_user_id": testUserID, "kind": "ended"})
	f.Insert(t, "inbox_item", testutil.Cols{"id": noticeID, "workspace_id": ws, "recipient_type": "member", "recipient_id": testUserID, "type": "iteration_ended", "title": "Private historical title", "details": `{"iteration_id":"` + iterationID + `"}`})
	return ws
}

func assertWorkspaceIterationRows(t *testing.T, ws string, want int) {
	t.Helper()
	for _, table := range iterationWorkspaceTables {
		if count := dbfx.Count(t, "SELECT count(*) FROM "+table+" WHERE workspace_id=$1", ws); count != want {
			t.Errorf("%s rows for %s = %d, want %d", table, ws, count, want)
		}
	}
}

func iterationWorkspaceDeleteRequest(ws string) *http.Request {
	r := withURLParam(newRequest(http.MethodDelete, "/api/workspaces/"+ws, nil), "id", ws)
	r.Header.Set("X-Workspace-ID", ws)
	return r
}

func TestDeleteWorkspaceRemovesIterationHistoryAndKeepsNeighbour(t *testing.T) {
	victim := workspaceIterationHistoryFixture(t)
	neighbour := workspaceIterationHistoryFixture(t)
	testutil.Call(t, testHandler.DeleteWorkspace, iterationWorkspaceDeleteRequest(victim)).Want(http.StatusNoContent)
	assertWorkspaceIterationRows(t, victim, 0)
	assertWorkspaceIterationRows(t, neighbour, 1)
}

type iterationDeleteFailureStarter struct{ inner txStarter }

func (s iterationDeleteFailureStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &iterationDeleteFailureTx{Tx: tx}, nil
}

type iterationDeleteFailureTx struct{ pgx.Tx }

func (tx *iterationDeleteFailureTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "DeleteWorkspaceIssueRoots") {
		return pgconn.CommandTag{}, errors.New("injected failure after iteration history cleanup")
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func TestDeleteWorkspaceIterationCleanupRollsBackWithIssueDeletion(t *testing.T) {
	ws := workspaceIterationHistoryFixture(t)
	h := *testHandler
	h.TxStarter = iterationDeleteFailureStarter{inner: h.TxStarter}
	testutil.Call(t, h.DeleteWorkspace, iterationWorkspaceDeleteRequest(ws)).Want(http.StatusInternalServerError)
	assertWorkspaceIterationRows(t, ws, 1)
	if dbfx.Count(t, "SELECT count(*) FROM workspace WHERE id=$1", ws) != 1 || dbfx.Count(t, "SELECT count(*) FROM issue WHERE workspace_id=$1", ws) != 1 {
		t.Fatal("failed deletion removed the workspace or issue")
	}
}

type iterationDeleteFenceStarter struct {
	inner    txStarter
	reached  chan<- uint32
	observed chan<- int
}

func (s iterationDeleteFenceStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &iterationDeleteFenceTx{Tx: tx, reached: s.reached, observed: s.observed}, nil
}

type iterationDeleteFenceTx struct {
	pgx.Tx
	reached  chan<- uint32
	observed chan<- int
}

func (tx *iterationDeleteFenceTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "DeleteWorkspaceIterationData") {
		var events int
		if err := tx.Tx.QueryRow(ctx, "SELECT count(*) FROM iteration_event WHERE workspace_id=$1", args[0]).Scan(&events); err != nil {
			return pgconn.CommandTag{}, err
		}
		tx.observed <- events
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func (tx *iterationDeleteFenceTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "LockWorkspaceForDelete") {
		tx.reached <- tx.Conn().PgConn().PID()
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func TestDeleteWorkspaceWaitsForIterationWriterAndSweepsCommittedFacts(t *testing.T) {
	ws := workspaceIterationHistoryFixture(t)
	var issueID, iterationID string
	dbfx.QueryRow(t, "SELECT id FROM issue WHERE workspace_id=$1", ws).Scan(&issueID)
	dbfx.QueryRow(t, "SELECT id FROM iteration WHERE workspace_id=$1", ws).Scan(&iterationID)
	dbfx.Exec(t, "UPDATE iteration SET status='active' WHERE id=$1", iterationID)
	dbfx.Exec(t, "UPDATE issue SET current_iteration_id=$1 WHERE id=$2", iterationID, issueID)
	dbfx.Exec(t, "UPDATE iteration_participation SET current_joined_at=clock_timestamp() WHERE workspace_id=$1", ws)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	writerReached, writerRelease := make(chan struct{}, 1), make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(writerRelease) })
	writer := *testHandler
	writer.TxStarter = projectAssociationCommitStarter{base: writer.TxStarter, reached: writerReached, release: writerRelease, once: &sync.Once{}}
	writerResult := make(chan *testutil.Response, 1)
	r := withURLParam(newRequest(http.MethodPut, "/api/issues/"+issueID, map[string]any{"title": "Committed immediately before deletion"}).WithContext(ctx), "id", issueID)
	r.Header.Set("X-Workspace-ID", ws)
	go func() { writerResult <- testutil.Call(t, writer.UpdateIssue, r) }()
	waitIterationBarrier(t, writerReached)
	deleterReached := make(chan uint32, 1)
	observed := make(chan int, 1)
	deleter := *testHandler
	deleter.TxStarter = iterationDeleteFenceStarter{inner: deleter.TxStarter, reached: deleterReached, observed: observed}
	deleteResult := make(chan *testutil.Response, 1)
	go func() {
		deleteResult <- testutil.Call(t, deleter.DeleteWorkspace, withURLParam(iterationWorkspaceDeleteRequest(ws).WithContext(ctx), "id", ws))
	}()
	var pid uint32
	select {
	case pid = <-deleterReached:
	case <-ctx.Done():
		t.Fatal("deletion did not attempt its workspace lock")
	}
	// Prove actual server-side waiting, not merely arrival at a test callback.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := testPool.QueryRow(ctx, "SELECT cardinality(pg_blocking_pids($1)) > 0", pid).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("workspace deletion did not wait for the uncommitted iteration writer")
		}
	}
	releaseOnce.Do(func() { close(writerRelease) })
	(<-writerResult).Want(http.StatusOK)
	(<-deleteResult).Want(http.StatusNoContent)
	select {
	case count := <-observed:
		if count != 2 {
			t.Fatalf("delete snapshot saw %d events, want original plus newly committed writer fact", count)
		}
	case <-ctx.Done():
		t.Fatal("workspace deletion never swept iteration history")
	}
	assertWorkspaceIterationRows(t, ws, 0)
	if dbfx.Count(t, "SELECT count(*) FROM workspace WHERE id=$1", ws) != 0 {
		t.Fatal("workspace survived deletion")
	}
}
