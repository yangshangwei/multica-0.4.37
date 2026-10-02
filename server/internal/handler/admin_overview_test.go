package handler

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestAdminOverviewUsageCountsTasksNotReports(t *testing.T) {
	login := adminHandlerSetup(t)
	ws := dbfx.Workspace(t, "Overview usage", "overview-usage-"+uuid.NewString())
	org, err := testHandler.Queries.AssignWorkspaceOrganization(t.Context(), parseUUID(ws))
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, "DELETE FROM organization_workspace WHERE workspace_id=$1", ws)
	fx := testutil.New(testPool, ws, login.User.ID)
	runtime := fx.Runtime(t, "Overview usage")
	agent := fx.Agent(t, "Overview usage", runtime)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	asOf := from.Add(12 * time.Hour)
	to := from.Add(24 * time.Hour)
	finished := func(at time.Time) string {
		return fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "status": "completed", "created_at": from.Add(-48 * time.Hour), "queued_at": from.Add(-time.Minute), "queued_at_source": "transition", "dispatched_at": from, "started_at": from, "completed_at": at})
	}
	report := func(task, model string, cost any, tokens int) {
		fx.Insert(t, "task_usage", testutil.Cols{"task_id": task, "provider": "fixture", "model": model, "cost_usd_ticks": cost, "input_tokens": tokens, "output_tokens": tokens * 2, "cache_read_tokens": tokens * 3, "cache_write_tokens": tokens * 4})
	}
	mixed := finished(from)
	report(mixed, "priced", 1, 10)
	report(mixed, "unpriced-a", nil, 20)
	report(mixed, "unpriced-b", nil, 30)
	priced := finished(asOf)
	report(priced, "priced", 0, 40)
	finished(from.Add(time.Hour)) // Missing usage stays distinct from reported zero tokens.
	zero := finished(from.Add(2 * time.Hour))
	report(zero, "zero", 0, 0)
	for _, at := range []time.Time{from.Add(-time.Microsecond), asOf.Add(time.Microsecond), to} {
		report(finished(at), "outside", nil, 1000)
	}
	for _, status := range []string{"queued", "dispatched", "running", "waiting_local_directory", "deferred"} {
		fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "status": status, "created_at": from.Add(-90 * 24 * time.Hour)})
	}
	fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "status": "queued", "created_at": asOf.Add(time.Microsecond)})
	params := db.GetAdminOverviewExecutionsParams{OrganizationID: org, TimeFrom: adminTimestamp(from), TimeTo: adminTimestamp(to), AsOf: adminTimestamp(asOf), WorkspaceID: parseUUID(ws)}
	row, err := testHandler.Queries.GetAdminOverviewExecutions(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}
	if row.FinishedCount != 4 || row.Completed != 4 || row.ReportedTasks != 3 || row.MissingTasks != 1 || row.UnpricedTasks != 1 {
		t.Fatalf("report multiplicity or finished-window boundaries changed: %+v", row)
	}
	if row.InputTokens != 100 || row.OutputTokens != 200 || row.CacheReadTokens != 300 || row.CacheWriteTokens != 400 {
		t.Fatalf("all report tokens must contribute exactly once: %+v", row)
	}
	if row.QueueSamples != 4 || row.QueueP50 != 60 || row.QueueP95 != 60 || row.Unfinished != 5 || row.Queued != 1 || row.Dispatched != 1 || row.Running != 1 || row.WaitingLocalDirectory != 1 || row.Deferred != 1 {
		t.Fatalf("usage reports changed outcome clocks or all-age current states: %+v", row)
	}
	for _, mismatch := range []string{"organization", "workspace"} {
		scoped := params
		if mismatch == "organization" {
			scoped.OrganizationID = parseUUID(uuid.NewString())
		} else {
			scoped.WorkspaceID = parseUUID(uuid.NewString())
		}
		empty, err := testHandler.Queries.GetAdminOverviewExecutions(t.Context(), scoped)
		if err != nil {
			t.Fatal(err)
		}
		if empty.FinishedCount != 0 || empty.ReportedTasks != 0 || empty.MissingTasks != 0 || empty.UnpricedTasks != 0 || empty.InputTokens != 0 || empty.Unfinished != 0 || empty.QueueP50 != -1 || empty.RunP50 != -1 {
			t.Fatalf("%s scope leaked aggregate data: %+v", mismatch, empty)
		}
	}
}
