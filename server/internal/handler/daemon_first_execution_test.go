package handler

import (
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// CompleteTask answers a late /complete for an already-finalized task with the
// existing row. Only a task that actually completed may count as the issue's
// first execution; a run that was stopped or failed never executed the issue.
func TestFirstExecutionAnalyticsIgnoresTasksThatDidNotComplete(t *testing.T) {
	runtimeID := handlerTestRuntimeID(t)
	agentID := dbfx.Agent(t, "First execution guard", runtimeID)
	for _, status := range []string{"cancelled", "failed", "completed"} {
		t.Run(status, func(t *testing.T) {
			issueID := dbfx.Issue(t, "First execution after "+status)
			taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID, "status": status})
			r := newRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/complete", nil)
			task, err := testHandler.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
			if err != nil {
				t.Fatal(err)
			}
			testHandler.emitIssueExecutedOnFirstCompletion(r, &task)
			var first pgtype.Timestamptz
			dbfx.QueryRow(t, `SELECT first_executed_at FROM issue WHERE id=$1`, issueID).Scan(&first)
			if first.Valid != (status == "completed") {
				t.Fatalf("first_executed_at set = %v for a %s task, want %v", first.Valid, status, status == "completed")
			}
		})
	}
}
