package handler

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"net/http"
	"testing"

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
