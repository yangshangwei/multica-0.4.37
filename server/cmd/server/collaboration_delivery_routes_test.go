package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationPublicCreationUsesOriginalAccountKey(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router DB fixture is required")
	}
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=true WHERE id=1`)
	fx.Cleanup(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
	key := uuid.NewString()
	title := "Public collaboration " + uuid.NewString()
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='create' AND scope_id=$1 AND request_id=$2`, testWorkspaceID, key)
	fx.Cleanup(t, `DELETE FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title)
	for _, table := range []string{"issue_collaboration", "issue_participant", "collaboration_session", "collaboration_event", "collaboration_notification"} {
		fx.Cleanup(t, `DELETE FROM `+table+` WHERE issue_id IN (SELECT id FROM issue WHERE workspace_id=$1 AND title=$2)`, testWorkspaceID, title)
	}
	call := func(method, path string, body any) *testutil.Response {
		t.Helper()
		r := testutil.JSONRequest(method, path, body)
		r.Header.Set("Authorization", "Bearer "+testToken)
		r.Header.Set("Idempotency-Key", key)
		r.Header.Set("X-Workspace-ID", "invalid-ambient-workspace")
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, r)
	}
	body := map[string]any{"source_workspace_id": testWorkspaceID, "title": title, "description": "Creation must not execute @AI"}
	first := call(http.MethodPost, "/api/collaboration/issues", body).Want(http.StatusCreated).Map()
	id, ok := first["id"].(string)
	if !ok || id == "" || first["description"] != body["description"] {
		t.Fatalf("unexpected creation response: %v", first)
	}
	second := call(http.MethodPost, "/api/collaboration/issues", body).Want(http.StatusCreated).Map()
	if second["id"] != id || fx.Count(t, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND title=$2`, testWorkspaceID, title) != 1 {
		t.Fatal("original creation key did not recover the same source issue")
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_session WHERE issue_id=$1 AND queue_state='paused'`, id) != 1 ||
		fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, id) != 0 {
		t.Fatal("public creation bypassed the paused collaboration transaction")
	}
	fx.Exec(t, `UPDATE collaboration_execution_gate SET creation_enabled=false WHERE id=1`)
	recovery := call(http.MethodGet, "/api/collaboration/creations/"+key+"?source_workspace_id="+testWorkspaceID, nil).Want(http.StatusOK)
	if recovery.Map()["id"] != id || recovery.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("creation gate closure broke private original-key recovery")
	}
	result := call(http.MethodGet, "/api/collaboration/creations/"+key+"/result?source_workspace_id="+testWorkspaceID, nil).Want(http.StatusOK).Map()
	if len(result) != 5 || result["outcome"] != "created" || result["code"] != "created" || result["actor_user_id"] != testUserID || result["source_workspace_id"] != testWorkspaceID || result["request_id"] != key {
		t.Fatal("registered creation result route lost its narrow own-operation contract")
	}
	call(http.MethodPost, "/api/collaboration/issues", body).Want(http.StatusCreated)
	body["title"] = "Changed original intent"
	call(http.MethodPost, "/api/collaboration/issues", body).Want(http.StatusConflict)
}
