package service

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
)

func TestTaskCapabilityFailureCannotWakeCoordinator(t *testing.T) {
	f, svc := seedDelegatedFailureFixture(t)
	fx := testutil.New(f.pool, f.workspaceID, f.userID)
	fx.Exec(t, `UPDATE agent SET template_key='code-reviewer',template_version=2 WHERE id=$1`, f.worker)
	taskID := f.insertWorkerTask(t, "running", "comment", 1, 2)
	failed, err := svc.FailTask(context.Background(), taskID, "synthetic restricted failure", "", "", "", "agent_error.process_failure", false, "", "")
	if err != nil || failed == nil {
		t.Fatalf("failure settlement: %v", err)
	}
	if count := fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1 AND id<>$2`, f.coordinator, f.sourceTask); count != 0 {
		t.Fatal("restricted failure woke coordinator")
	}
	if count := fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id=$1 AND source_task_id=$2 AND author_type='system' AND type='progress_update'`, f.issueID, util.UUIDToString(taskID)); count != 0 {
		t.Fatal("restricted failure left a replayable recovery event")
	}
}
