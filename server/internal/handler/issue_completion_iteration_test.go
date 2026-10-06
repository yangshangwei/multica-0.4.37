package handler

import (
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// W17 records the first lifetime completion for analytics. A delayed completion
// from before this participation must not masquerade as a current execution start.
func TestFirstCompletionAnalyticsDoesNotCreateIterationFacts(t *testing.T) {
	issueID, iterationID := iterationIssueFixture(t)
	runtimeID := handlerTestRuntimeID(t)
	agentID := dbfx.Agent(t, "Prior participation execution", runtimeID)
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "completed",
		"started_at":   testutil.Raw("clock_timestamp() - interval '10 minutes'"),
		"completed_at": testutil.Raw("clock_timestamp() - interval '1 minute'"),
	})
	h := *testHandler
	h.TaskService = service.NewTaskService(h.Queries, testPool, nil, events.New())
	r := newRequest(http.MethodPost, "/api/tasks/"+taskID+"/complete", nil)
	task, err := h.Queries.GetAgentTask(r.Context(), parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	h.emitIssueExecutedOnFirstCompletion(r, &task)
	var first, repeated pgtype.Timestamptz
	dbfx.QueryRow(t, `SELECT first_executed_at FROM issue WHERE id=$1`, issueID).Scan(&first)
	if !first.Valid {
		t.Fatal("real completion path did not mark the analytics timestamp")
	}
	h.emitIssueExecutedOnFirstCompletion(r, &task)
	dbfx.QueryRow(t, `SELECT first_executed_at FROM issue WHERE id=$1`, issueID).Scan(&repeated)
	if !repeated.Time.Equal(first.Time) {
		t.Fatal("repeated completion changed the first lifetime timestamp")
	}
	var count, scope, revision, rollover int64
	var started, original bool
	var pointer, status string
	dbfx.QueryRow(t, `SELECT count(*) FROM iteration_event WHERE iteration_id=$1`, iterationID).Scan(&count)
	dbfx.QueryRow(t, `SELECT scope_revision FROM iteration WHERE id=$1`, iterationID).Scan(&scope)
	dbfx.QueryRow(t, `SELECT has_started_current_participation,in_original FROM iteration_participation WHERE iteration_id=$1 AND issue_id=$2`, iterationID, issueID).Scan(&started, &original)
	dbfx.QueryRow(t, `SELECT revision,current_iteration_id,iteration_rollover_count,status FROM issue WHERE id=$1`, issueID).Scan(&revision, &pointer, &rollover, &status)
	if count != 0 || scope != 1 || revision != 1 || started || !original || pointer != iterationID || rollover != 2 || status != "todo" {
		t.Fatalf("completion analytics changed participation: events=%d scope=%d issue=%d started=%v original=%v pointer=%s rollover=%d status=%s", count, scope, revision, started, original, pointer, rollover, status)
	}
}
