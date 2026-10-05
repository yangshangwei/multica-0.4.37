package handler

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestProjectDescriptionRequiresRevisionAndPreservesNewerText(t *testing.T) {
	id := dbfx.Insert(t, "project", testutil.Cols{"workspace_id": testWorkspaceID, "title": "P1 revision", "description": "original"})
	request := func(body map[string]any, status int) map[string]any {
		t.Helper()
		var out map[string]any
		testutil.Call(t, testHandler.UpdateProject, withURLParam(newRequest("PUT", "/api/projects/"+id, body), "id", id)).Want(status).JSON(&out)
		return out
	}
	out := request(map[string]any{"description": "new text"}, http.StatusPreconditionRequired)
	if out["code"] != "project_description_revision_required" {
		t.Fatalf("missing token code: %v", out)
	}
	out = request(map[string]any{"description": "new text", "expected_description_revision": 1}, 200)
	if out["revision"] != float64(2) || out["description_revision"] != float64(2) {
		t.Fatalf("versions: %v", out)
	}
	out = request(map[string]any{"description": "stale text", "expected_description_revision": 1}, 409)
	if out["code"] != "project_description_conflict" {
		t.Fatalf("conflict: %v", out)
	}
	out = request(map[string]any{"priority": "high"}, 200)
	if out["description"] != "new text" || out["description_revision"] != float64(2) {
		t.Fatalf("attribute overwrote description: %v", out)
	}
	out = request(map[string]any{"description": "new text"}, 200)
	if out["revision"] != float64(3) {
		t.Fatalf("no-op incremented revision: %v", out)
	}
	request(map[string]any{"priority": "low", "expected_revision": 2}, 409)
}

func TestProjectFinalDateOrderValidatedOnlyOnDateEdits(t *testing.T) {
	id := dbfx.Insert(t, "project", testutil.Cols{"workspace_id": testWorkspaceID, "title": "P1 dates", "start_date": "2026-04-10", "due_date": "2026-04-01"})
	testutil.Call(t, testHandler.UpdateProject, withURLParam(newRequest("PUT", "/api/projects/"+id, map[string]any{"priority": "high"}), "id", id)).Want(200)
	testutil.Call(t, testHandler.UpdateProject, withURLParam(newRequest("PUT", "/api/projects/"+id, map[string]any{"start_date": "2026-04-11"}), "id", id)).Want(422)
	testutil.Call(t, testHandler.UpdateProject, withURLParam(newRequest("PUT", "/api/projects/"+id, map[string]any{"start_date": nil}), "id", id)).Want(200)
}

func TestProjectStateAuditDoesNotChangeIssuesOrExecution(t *testing.T) {
	id := dbfx.Insert(t, "project", testutil.Cols{"workspace_id": testWorkspaceID, "title": "P1 state"})
	dbfx.Cleanup(t, "DELETE FROM project_state_change WHERE project_id=$1", id)
	issue := dbfx.Issue(t, "untouched task", testutil.Cols{"project_id": id, "status": "in_progress"})
	for _, status := range []string{"in_progress", "paused", "cancelled", "completed", "in_progress"} {
		var out map[string]any
		testutil.Call(t, testHandler.UpdateProject, withURLParam(newRequest("PUT", "/api/projects/"+id, map[string]any{"status": status}), "id", id)).Want(200).JSON(&out)
		if out["description_revision"] != float64(1) {
			t.Fatalf("status invalidated acceptance: %v", out)
		}
		if status == "in_progress" && (out["in_progress_since"] == nil || out["in_progress_since_source"] != "transition") {
			t.Fatalf("transition clock: %v", out)
		}
		if status != "in_progress" && out["in_progress_since"] != nil {
			t.Fatalf("ended current clock: %v", out)
		}
	}
	testutil.Call(t, testHandler.UpdateProject, withURLParam(newRequest("PUT", "/api/projects/"+id, map[string]any{"status": "in_progress"}), "id", id)).Want(200)
	if n := dbfx.Count(t, "SELECT count(*) FROM project_state_change WHERE project_id=$1 AND reason IS NULL", id); n != 5 {
		t.Fatalf("audit/no-op count=%d", n)
	}
	var status, projectID string
	dbfx.QueryRow(t, "SELECT status,project_id::text FROM issue WHERE id=$1", issue).Scan(&status, &projectID)
	if status != "in_progress" || projectID != id {
		t.Fatalf("issue changed: %s %s", status, projectID)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE issue_id=$1", issue); n != 0 {
		t.Fatalf("state enqueued %d tasks", n)
	}
}

func TestProjectTransactionRetriesWholeRepeatableReadUnit(t *testing.T) {
	attempts := 0
	ctx := context.Background()
	err := testHandler.runProjectTransaction(ctx, parseUUID(testWorkspaceID), parseUUID(testUserID), func(tx pgx.Tx, q *db.Queries) error {
		attempts++
		var isolation, readonly string
		if err := tx.QueryRow(ctx, "SHOW transaction_isolation").Scan(&isolation); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, "SHOW transaction_read_only").Scan(&readonly); err != nil {
			return err
		}
		if isolation != "repeatable read" || readonly != "off" {
			t.Fatalf("transaction = %s, read_only=%s", isolation, readonly)
		}
		if _, err := tx.Exec(ctx, "UPDATE workspace SET planning_timezone='Asia/Shanghai' WHERE id=$1", testWorkspaceID); err != nil {
			return err
		}
		if attempts < 3 {
			return &pgconn.PgError{Code: "40001", Message: "injected serialization failure"}
		}
		return nil
	})
	dbfx.Cleanup(t, "UPDATE workspace SET planning_timezone=NULL WHERE id=$1", testWorkspaceID)
	if err != nil || attempts != 3 {
		t.Fatalf("retry: %d %v", attempts, err)
	}
	var zone string
	dbfx.QueryRow(t, "SELECT planning_timezone FROM workspace WHERE id=$1", testWorkspaceID).Scan(&zone)
	if zone != "Asia/Shanghai" {
		t.Fatalf("committed final callback: %s", zone)
	}
}

func TestProjectUpdateConstraintErrorPreserved(t *testing.T) {
	id := dbfx.Project(t, "P1 validation")
	testutil.Call(t, testHandler.UpdateProject, withURLParam(newRequest("PUT", "/api/projects/"+id, map[string]any{"lead_type": "unsupported"}), "id", id)).Want(400)
}

func TestProjectConcurrentDescriptionWritesOneWinner(t *testing.T) {
	id := dbfx.Project(t, "P1 concurrent", testutil.Cols{"description": "original"})
	outcomes := make(chan int, 2)
	start := make(chan struct{})
	for _, body := range []string{"version A", "version B"} {
		go func(text string) {
			<-start
			response := testutil.Call(t, testHandler.UpdateProject, withURLParam(newRequest("PUT", "/api/projects/"+id, map[string]any{"description": text, "expected_description_revision": 1}), "id", id)).WantOneOf(200, 409)
			outcomes <- response.Code
		}(body)
	}
	close(start)
	first, second := <-outcomes, <-outcomes
	if first+second != 609 {
		t.Fatalf("two edits must have one winner: %d %d", first, second)
	}
	var description string
	var revision int64
	dbfx.QueryRow(t, "SELECT description,description_revision FROM project WHERE id=$1", id).Scan(&description, &revision)
	if revision != 2 || (description != "version A" && description != "version B") {
		t.Fatalf("committed result %q @ %d", description, revision)
	}
}

type projectMemberFenceStarter struct {
	txStarter
	entered chan struct{}
	resume  chan struct{}
	once    *sync.Once
}
type projectMemberFenceTx struct {
	pgx.Tx
	gate projectMemberFenceStarter
}

func (s projectMemberFenceStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.txStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return projectMemberFenceTx{Tx: tx, gate: s}, nil
}
func (tx projectMemberFenceTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "-- name: LockSubscriberWrites ") {
		tx.gate.once.Do(func() {
			close(tx.gate.entered)
			select {
			case <-tx.gate.resume:
			case <-ctx.Done():
			}
		})
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func TestProjectWriteRechecksMemberAfterRepeatableReadFenceWait(t *testing.T) {
	id := dbfx.Project(t, "P1 membership", testutil.Cols{"description": "original"})
	user := dbfx.User(t, "P1 removed member", "p1-fence-member@example.test")
	member := dbfx.Member(t, testWorkspaceID, user, "member")
	entered, resume := make(chan struct{}), make(chan struct{})
	h := *testHandler
	h.TxStarter = projectMemberFenceStarter{txStarter: h.TxStarter, entered: entered, resume: resume, once: &sync.Once{}}
	req := withURLParam(newRequest("PUT", "/api/projects/"+id, map[string]any{"description": "revoked", "expected_description_revision": 1}), "id", id)
	req.Header.Set("X-User-ID", user)
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	done := make(chan int, 1)
	go func() { done <- testutil.Call(t, h.UpdateProject, req).Code }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("writer never reached the revocation fence")
	}
	// The workspace locking read has already established the RR snapshot. A
	// current membership locking read must reject this committed revocation.
	dbfx.Exec(t, "DELETE FROM member WHERE id=$1", member)
	close(resume)
	if code := <-done; code != 403 {
		t.Fatalf("revoked actor received HTTP %d", code)
	}
	var description string
	dbfx.QueryRow(t, "SELECT description FROM project WHERE id=$1", id).Scan(&description)
	if description != "original" {
		t.Fatalf("revoked write committed %q", description)
	}
}
