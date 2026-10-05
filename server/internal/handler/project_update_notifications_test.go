package handler

import (
	"context"
	"encoding/json"
	"github.com/multica-ai/multica/server/internal/testutil"
	"net/http"
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

func TestProjectUpdateInboxDetailsRemainStringValues(t *testing.T) {
	_, recipient, row := progressNotification(t)
	old := dbfx.Insert(t, "inbox_item", testutil.Cols{"workspace_id": testWorkspaceID, "recipient_type": "member", "recipient_id": recipient, "type": "status_changed", "severity": "info", "title": "Existing notification", "details": `{"status":"done"}`})
	if err := testHandler.deliverProjectUpdateNotification(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	req := inboxRequest(http.MethodGet, "/api/inbox", testWorkspaceID)
	req.Header.Set("X-User-ID", recipient)
	response := testutil.Call(t, inboxWorkspaceHandler(testHandler.ListInbox), req).Want(200)
	t.Logf("PROJECT_UPDATE_INBOX_FIXTURE %s", response.Text())
	var items []struct {
		ID      string            `json:"id"`
		Details map[string]string `json:"details"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &items); err != nil {
		t.Fatalf("existing inbox string-details contract rejected actual response: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("mixed inbox lost items: %s", response.Text())
	}
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.ID] = true
		if item.ID == uuidToString(row.ID) && item.Details["revision"] != "1" {
			t.Fatalf("project revision must be wire string: %v", item.Details)
		}
	}
	if !seen[old] || !seen[uuidToString(row.ID)] {
		t.Fatalf("mixed inbox identities=%v", seen)
	}
}
