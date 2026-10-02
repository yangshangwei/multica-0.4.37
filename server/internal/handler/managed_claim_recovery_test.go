package handler

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// A successful claim whose response is lost remains dispatched until the
// recovery window expires. Retrying over HTTP must recover that exact managed
// execution, without immediately duplicating it or reclaiming a started task.
func TestManagedBatchClaimLostResponseRecovery(t *testing.T) {
	f := managedDaemonSetup(t)
	fx := testutil.New(testPool, f.workspace, f.owner)
	agent := fx.Agent(t, "Lost managed claim", f.runtime)
	issue := fx.Issue(t, "Recover lost managed claim")
	taskID := fx.Task(t, agent, testutil.Cols{"runtime_id": f.runtime, "issue_id": issue})
	fx.Cleanup(t, "DELETE FROM task_token WHERE task_id=$1", taskID)
	body := map[string]any{"daemon_id": f.daemon, "runtime_ids": []string{f.runtime}, "max_tasks": 1}
	claim := func() batchClaimResponse {
		t.Helper()
		var result batchClaimResponse
		f.call(t, f.token, "/api/daemon/tasks/claim", body, f.h.ClaimTasksByRuntime).Want(200).JSON(&result)
		return result
	}
	first := claim()
	if len(first.Tasks) != 1 || first.Tasks[0].ID != taskID {
		t.Fatal("initial managed claim did not return the queued task")
	}
	before, err := f.h.Queries.GetAgentTask(t.Context(), parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	if !before.ExecutionBindingID.Valid || uuidToString(before.ExecutionBindingID) != f.binding {
		t.Fatal("initial claim did not capture the managed binding")
	}
	if len(claim().Tasks) != 0 {
		t.Fatal("an immediate retry duplicated an unacknowledged claim")
	}
	fx.Exec(t, "UPDATE agent_task_queue SET prepare_lease_expires_at=now()-interval '1 second' WHERE id=$1", taskID)
	if len(claim().Tasks) != 0 {
		t.Fatal("an expired prepare lease bypassed the dispatch recovery window")
	}
	// Age only fixture timestamps; no sleep or production timeout changes are
	// needed to exercise the real SQL eligibility and finalization paths.
	fx.Exec(t, "UPDATE agent_task_queue SET dispatched_at=now()-interval '2 minutes',prepare_lease_expires_at=now()+interval '1 minute' WHERE id=$1", taskID)
	if len(claim().Tasks) != 0 {
		t.Fatal("a stale dispatch bypassed the active prepare lease")
	}
	fx.Exec(t, "UPDATE agent_task_queue SET prepare_lease_expires_at=now()-interval '1 second' WHERE id=$1", taskID)
	recovered := claim()
	if len(recovered.Tasks) != 1 || recovered.Tasks[0].ID != taskID || recovered.Tasks[0].AuthToken == "" {
		t.Fatal("HTTP retry failed to recover the stale managed claim with credentials")
	}
	after, err := f.h.Queries.GetAgentTask(t.Context(), parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	if after.ClaimGeneration != before.ClaimGeneration+1 || after.ExecutionBindingID != before.ExecutionBindingID || after.ExecutionBindingEpoch != before.ExecutionBindingEpoch || after.ExecutionAdmissionVersion != before.ExecutionAdmissionVersion {
		t.Fatal("recovery did not advance the generation while preserving execution authority")
	}
	if len(claim().Tasks) != 0 {
		t.Fatal("recovery lease allowed an immediate duplicate delivery")
	}
	if _, err = f.h.TaskService.StartTask(t.Context(), parseUUID(taskID)); err != nil {
		t.Fatal(err)
	}
	// Make time gates eligible again so only the started-task fence prevents reclaim.
	fx.Exec(t, "UPDATE agent_task_queue SET dispatched_at=now()-interval '2 minutes',prepare_lease_expires_at=CASE WHEN prepare_lease_expires_at IS NULL THEN NULL ELSE now()-interval '1 second' END WHERE id=$1", taskID)
	if len(claim().Tasks) != 0 {
		t.Fatal("a started managed task was reclaimed")
	}
	if fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='running' AND started_at IS NOT NULL", taskID) != 1 {
		t.Fatal("recovered task did not remain running")
	}
}
