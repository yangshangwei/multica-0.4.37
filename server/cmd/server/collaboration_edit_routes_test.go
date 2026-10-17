package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCollaborationEditRouteRetainsStatusWireAndExternalAuthority(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router fixture required")
	}
	base := testutil.New(testPool, testWorkspaceID, testUserID)
	source := base.Workspace(t, "Edit source", "collaboration-edit-"+uuid.NewString())
	base.Member(t, source, testUserID, "owner")
	email := "edit-router-" + uuid.NewString() + "@example.invalid"
	external := base.User(t, "External editor", email)
	token, err := generateTestJWT(external, email, "External editor")
	if err != nil {
		t.Fatal(err)
	}
	fx := testutil.New(testPool, source, testUserID)
	if err = db.New(testPool).SeedIssueStatusEntries(t.Context(), util.MustParseUUID(source)); err != nil {
		t.Fatal(err)
	}
	fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id=$1`, source)
	issue := fx.Issue(t, "Original title", testutil.Cols{"assignee_type": "member", "assignee_id": external})
	label := fx.Insert(t, "issue_label", testutil.Cols{"workspace_id": source, "resource_type": "issue", "name": "Delivery", "color": "#123456"})
	property := fx.Insert(t, "issue_property", testutil.Cols{"workspace_id": source, "name": "Delivery note", "type": "text"})
	fx.Cleanup(t, `DELETE FROM issue_to_label WHERE issue_id=$1`, issue)
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
	session := fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	for _, user := range []string{testUserID, external} {
		fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": user}, "issue_id=$1 AND user_id=$2", issue, user)
		fx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": user}, "user_id=$1", user)
	}
	seedCollaborationRouteCommand(t, fx, session, external, "Immutable command input")
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	fx.Cleanup(t, `DELETE FROM collaboration_event WHERE issue_id=$1`, issue)
	path := "/api/collaboration/issues/" + issue
	call := func(method, url, key string, body any) *testutil.Response {
		t.Helper()
		req := testutil.JSONRequest(method, url, body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Workspace-ID", "unrelated-client-workspace")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
	}
	input := func() map[string]any {
		t.Helper()
		current := call(http.MethodGet, path, "", nil).Want(http.StatusOK).Map()
		write := current["write_context"].(map[string]any)
		return map[string]any{"expected_collaboration_revision": current["collaboration_revision"],
			"expected_lifecycle_epoch": current["lifecycle_epoch"], "expected_participant_epoch": write["participant_epoch"],
			"expected_account_epoch": write["account_epoch"]}
	}
	// The service/handler suites own the validation matrices. Here an external
	// handler must reach both account-scoped reads through real authentication.
	fields := call(http.MethodGet, path+"/field-options", "", nil).Want(http.StatusOK).Map()
	if fields["issue_id"] != issue || fields["source_workspace_id"] != source || len(fields["labels"].([]any)) != 1 || len(fields["properties"].([]any)) != 1 {
		t.Fatalf("source field options lost their issue scope: %v", fields)
	}
	resolved := call(http.MethodPost, "/api/collaboration/recipients/resolve", "", map[string]any{
		"kind": "user_id", "value": testUserID, "issue_id": issue,
	}).Want(http.StatusOK).Map()
	if resolved["issue_id"] != issue || resolved["source_workspace_id"] != source || resolved["recipient"].(map[string]any)["user_id"] != testUserID {
		t.Fatalf("exact recipient resolution lost its account context: %v", resolved)
	}
	edit := input()
	edit["title"], edit["description"] = "Edited by external handler", "Revised body"
	edit["start_date"], edit["due_date"] = "2026-10-14T00:00:00Z", "2026-10-31"
	edit["priority"], edit["label_ids"] = "urgent", []string{label}
	edit["properties"] = map[string]any{property: "Ready for review"}
	key := uuid.NewString()
	receipt := call(http.MethodPatch, path, key, edit).Want(http.StatusOK).Map()
	if receipt["code"] != "issue_edited" {
		t.Fatalf("external metadata edit did not reach its domain service: %v", receipt)
	}
	call(http.MethodPatch, path, key, edit).Want(http.StatusOK)
	detail := call(http.MethodGet, path, "", nil).Want(http.StatusOK).Map()
	if detail["title"] != "Edited by external handler" || detail["start_date"] != "2026-10-14" || detail["due_date"] != "2026-10-31" {
		t.Fatalf("metadata edit did not survive the account read boundary: %v", detail)
	}
	if detail["priority"] != "urgent" || detail["properties"].(map[string]any)[property] != "Ready for review" ||
		len(detail["labels"].([]any)) != 1 || len(detail["property_definitions"].([]any)) != 1 {
		t.Fatalf("edited source values did not survive the account read boundary: %v", detail)
	}
	options := call(http.MethodGet, path+"/status-options?action=change_status", "", nil).Want(http.StatusOK).Map()
	status := input()
	status["status"], status["status_options_version"] = "blocked", options["status_options_version"]
	status["due_date"] = nil
	call(http.MethodPatch, path, uuid.NewString(), status).Want(http.StatusBadRequest)
	delete(status, "due_date")
	if got := call(http.MethodPatch, path, uuid.NewString(), status).Want(http.StatusOK).Map(); got["code"] != "status_changed" {
		t.Fatalf("existing status-only PATCH contract stopped working: %v", got)
	}
	detail = call(http.MethodGet, path, "", nil).Want(http.StatusOK).Map()
	if detail["status"] != "blocked" || detail["due_date"] != "2026-10-31" || detail["session"].(map[string]any)["queue_state"] != "paused" {
		t.Fatalf("mixed refusal or status-only change altered unrelated state: %v", detail)
	}
	if fx.Count(t, `SELECT count(*) FROM collaboration_command WHERE session_id=$1 AND state='waiting' AND content='Immutable command input'`, session) != 1 ||
		fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue) != 0 {
		t.Fatal("metadata/status changes rewrote explicit input or dispatched ordinary AI")
	}
}
