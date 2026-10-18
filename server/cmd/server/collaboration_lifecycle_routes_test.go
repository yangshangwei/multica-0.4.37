package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// These route tests retain an already accepted input. They exercise human
// workflow, not resource consent or execution admission, which remain disabled.
func seedCollaborationRouteCommand(t *testing.T, fx *testutil.Fixture, session, author, content string) {
	t.Helper()
	runtime := fx.Runtime(t, "Synthetic collaboration route runtime", testutil.Cols{"status": "offline"})
	agent := fx.Agent(t, "Synthetic collaboration route target "+uuid.NewString(), runtime)
	roster, err := json.Marshal([]collaboration.TargetResource{{Role: "agent", AgentID: agent, RuntimeID: runtime, ExecutionWorkspaceID: fx.WorkspaceID, OwnerUserID: fx.UserID}})
	if err != nil {
		t.Fatal(err)
	}
	channel := fx.Insert(t, "collaboration_channel", testutil.Cols{"session_id": session, "root_agent_id": agent, "next_batch_seq": 2})
	target := fx.Insert(t, "collaboration_target", testutil.Cols{"session_id": session, "kind": "agent", "resource_id": agent, "roster": roster, "next_command_seq": 2})
	// Registration binds the target to the channel's complete control snapshot.
	if _, err = db.New(fx.Pool).BindCollaborationTargetChannel(t.Context(), db.BindCollaborationTargetChannelParams{
		SessionID: util.MustParseUUID(session), TargetID: util.MustParseUUID(target), ChannelID: util.MustParseUUID(channel),
	}); err != nil {
		t.Fatal(err)
	}
	batch := fx.Insert(t, "collaboration_batch", testutil.Cols{"session_id": session, "channel_id": channel, "effective_target_id": target, "seq": 1})
	fx.Exec(t, `UPDATE collaboration_session SET target_set_version=target_set_version+1 WHERE id=$1`, session)
	fx.Insert(t, "collaboration_command", testutil.Cols{"session_id": session, "target_id": target, "channel_id": channel, "batch_id": batch, "input_event_id": uuid.NewString(),
		"seq": 1, "author_user_id": author, "participant_epoch": 1, "author_auth_epoch": 1, "lifecycle_epoch": 1, "content": content})
}

func TestCollaborationLifecycleRoutesKeepCreatorAuthorityAfterSourceDeparture(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router fixture required")
	}
	baseFX := testutil.New(testPool, testWorkspaceID, testUserID)
	source := baseFX.Workspace(t, "Lifecycle source", "lifecycle-router-"+uuid.NewString())
	baseFX.Member(t, source, testUserID, "owner")
	email := "lifecycle-router-" + uuid.NewString() + "@example.test"
	handler := baseFX.User(t, "External handler", email)
	handlerToken, err := generateTestJWT(handler, email, "External handler")
	if err != nil {
		t.Fatal(err)
	}
	fx := testutil.New(testPool, source, testUserID)
	if err = db.New(testPool).SeedIssueStatusEntries(t.Context(), util.MustParseUUID(source)); err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, source)
	issue := fx.Issue(t, "Lifecycle route fixture", testutil.Cols{"assignee_type": "member", "assignee_id": handler})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
	session := fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	for _, actor := range []string{testUserID, handler} {
		fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": actor}, "issue_id=$1 AND user_id=$2", issue, actor)
		fx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": actor}, "user_id=$1", actor)
	}
	seedCollaborationRouteCommand(t, fx, session, handler, "waiting work")
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	fx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	path := "/api/collaboration/issues/" + issue
	call := func(token, method, url, key string, body any) *testutil.Response {
		t.Helper()
		req := testutil.JSONRequest(method, url, body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Workspace-ID", "unrelated-ambient-space")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
	}
	options := func(token, action string) map[string]any {
		t.Helper()
		return call(token, http.MethodGet, path+"/status-options?action="+action, "", nil).Want(http.StatusOK).Map()
	}
	input := func(token, status string, options map[string]any) map[string]any {
		t.Helper()
		current := call(token, http.MethodGet, path, "", nil).Want(http.StatusOK).Map()
		write := current["write_context"].(map[string]any)
		return map[string]any{"expected_collaboration_revision": current["collaboration_revision"], "expected_lifecycle_epoch": current["lifecycle_epoch"], "expected_participant_epoch": write["participant_epoch"], "expected_account_epoch": write["account_epoch"], "status": status, "status_options_version": options["status_options_version"]}
	}
	oldOptions := options(handlerToken, "change_status")
	response := call(handlerToken, http.MethodPatch, path, uuid.NewString(), input(handlerToken, "in_progress", oldOptions)).Want(http.StatusOK).Map()
	if response["code"] != "status_changed" {
		t.Fatalf("manual status failed: %v", response)
	}
	staleKey := uuid.NewString()
	staleBody := input(handlerToken, "blocked", oldOptions)
	stale := call(handlerToken, http.MethodPatch, path, staleKey, staleBody).Want(http.StatusConflict).Map()
	if stale["code"] != "status_options_changed" || stale["status_options"] == nil {
		t.Fatalf("missing current status choices: %v", stale)
	}
	completeOptions := options(testToken, "complete")
	wrongOwner := input(handlerToken, "done", completeOptions)
	wrongOwner["action"] = "complete"
	call(handlerToken, http.MethodPost, path+"/lifecycle", uuid.NewString(), wrongOwner).Want(http.StatusForbidden)
	call(handlerToken, http.MethodGet, path+"/status-options?action=complete", "", nil).Want(http.StatusForbidden)
	// Source membership and creator collaboration authority are independent.
	fx.Exec(t, `DELETE FROM member WHERE workspace_id=$1 AND user_id=$2`, source, testUserID)
	complete := input(testToken, "done", options(testToken, "complete"))
	complete["action"] = "complete"
	completeKey := uuid.NewString()
	completed := call(testToken, http.MethodPost, path+"/lifecycle", completeKey, complete).Want(http.StatusOK).Map()
	if completed["code"] != "completed" || completed["collaboration_revision"] != "3" {
		t.Fatalf("departed creator completion: %v", completed)
	}
	call(testToken, http.MethodPost, path+"/lifecycle", completeKey, complete).Want(http.StatusOK)
	closed := call(handlerToken, http.MethodGet, path, "", nil).Want(http.StatusOK).Map()
	if closed["status"] != "done" || closed["session"].(map[string]any)["queue_state"] != "closed" {
		t.Fatalf("lifecycle did not close session: %v", closed)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND state='cancelled'`, session) != 1 {
		t.Fatal("completion retained waiting command")
	}
	replay := call(handlerToken, http.MethodPatch, path, staleKey, staleBody).Want(http.StatusConflict).Map()
	if replay["code"] != "status_options_changed" || replay["status_options"] != nil {
		t.Fatalf("replay disclosed ineligible status choices: %v", replay)
	}
	call(handlerToken, http.MethodPatch, path, uuid.NewString(), input(handlerToken, "todo", oldOptions)).Want(http.StatusConflict)
	reopen := input(testToken, "todo", options(testToken, "reopen"))
	reopen["action"] = "reopen"
	call(testToken, http.MethodPost, path+"/lifecycle", uuid.NewString(), reopen).Want(http.StatusOK)
	opened := call(handlerToken, http.MethodGet, path, "", nil).Want(http.StatusOK).Map()
	if opened["status"] != "todo" || opened["lifecycle_epoch"] != "3" || opened["session"].(map[string]any)["queue_state"] != "paused" {
		t.Fatalf("reopen did not preserve paused lifecycle: %v", opened)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND state='waiting'`, session) != 0 || fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue) != 0 {
		t.Fatal("reopen revived cancelled work or started ordinary AI")
	}
}
