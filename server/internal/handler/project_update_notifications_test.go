package handler

import (
	"context"
	"testing"
	"time"
)

func TestProjectUpdateMentionsNeverExecuteAndNotifyOnce(t *testing.T) {
	p := progressProject(t)
	runtime := dbfx.Runtime(t, "Progress runtime")
	agent := dbfx.Agent(t, "Mention only", runtime)
	recipient := dbfx.User(t, "Mention recipient", "progress-mention@example.invalid")
	dbfx.Member(t, testWorkspaceID, recipient, "member")
	dbfx.Cleanup(t, "DELETE FROM inbox_item WHERE workspace_id=$1 AND recipient_id=$2", testWorkspaceID, recipient)
	d := progressDraft("[@Member](mention://member/" + recipient + ") [@Agent](mention://agent/" + agent + ")")
	preview := progressPreview(t, p, d)
	if len(preview["recipients"].([]any)) != 1 {
		t.Fatalf("recipients: %v", preview)
	}
	input := progressInput(preview)
	first := progressCall(t, "create", p, "", input, 201)
	progressCall(t, "create", p, "", input, 200)
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE agent_id=$1", agent); n != 0 {
		t.Fatalf("mention executed %d tasks", n)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM project_update_notification WHERE project_id=$1", p); n != 1 {
		t.Fatalf("outbox rows=%d", n)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { testHandler.RunProjectUpdateNotifications(ctx); close(done) }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE workspace_id=$1 AND recipient_id=$2 AND type='project_update'", testWorkspaceID, recipient) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker ignored context cancellation")
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM inbox_item WHERE workspace_id=$1 AND recipient_id=$2 AND type='project_update'", testWorkspaceID, recipient); n != 1 {
		t.Fatalf("inbox=%d update=%v", n, first)
	}
}
