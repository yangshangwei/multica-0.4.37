package main

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCollaborationWorkflowRoutesPreserveResponsibilityAndMinimalRecovery(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router DB fixture is required")
	}
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	user := func(name string) (string, string) {
		t.Helper()
		email := "workflow-" + uuid.NewString() + "@example.test"
		id := fx.User(t, name, email)
		token, err := generateTestJWT(id, email, name)
		if err != nil {
			t.Fatal(err)
		}
		return id, token
	}
	b, bToken := user("Independent handler")
	c, cToken := user("Next handler without workspace")
	d, _ := user("Uninvited account")
	other := fx.Workspace(t, "Independent", "workflow-"+uuid.NewString())
	fx.Member(t, other, b, "owner")
	issue := fx.Issue(t, "Workflow HTTP fixture", testutil.Cols{"assignee_type": "member", "assignee_id": testUserID})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
	fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": testUserID}, "issue_id=$1 AND user_id=$2", issue, testUserID)
	ensureCollaborationRouteAccountFence(t, fx, testUserID)
	fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{b, c, d})
	for _, table := range []string{"issue_participant", "issue_handoff", "collaboration_event", "collaboration_notification"} {
		fx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", issue)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	base := "/api/collaboration/issues/" + issue
	call := func(token, method, path, key string, body any) *testutil.Response {
		t.Helper()
		r := testutil.JSONRequest(method, path, body)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Workspace-ID", "unrelated-ambient-value")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, r)
	}
	input := func(revision string, target string) map[string]string {
		out := map[string]string{"expected_collaboration_revision": revision, "expected_lifecycle_epoch": "1", "expected_participant_epoch": "1", "expected_account_epoch": "1"}
		if target != "" {
			out["target_user_id"] = target
		}
		return out
	}
	transfer := func(revision, target string) map[string]string {
		out := input(revision, target)
		out["purpose"] = "transfer"
		return out
	}
	verify := func(response *testutil.Response, code, revision string) {
		t.Helper()
		body := response.Want(http.StatusOK).Map()
		if len(body) != 2 || body["code"] != code || body["collaboration_revision"] != revision || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("nonminimal or wrong workflow receipt: %v", body)
		}
	}
	key := uuid.NewString()
	verify(call(testToken, http.MethodPost, base+"/participants", key, input("1", b)), "participant_added", "2")
	verify(call(testToken, http.MethodPost, base+"/participants", key, input("1", b)), "participant_added", "2")
	detail := call(bToken, http.MethodGet, base, "", nil).Want(http.StatusOK).Map()
	context, ok := detail["write_context"].(map[string]any)
	if !ok || context["participant_epoch"] != "1" || context["account_epoch"] != "1" {
		t.Fatalf("missing actor write context: %v", detail)
	}
	verify(call(testToken, http.MethodPost, base+"/handoffs", uuid.NewString(), transfer("2", b)), "transferred", "3")
	current := call(bToken, http.MethodGet, "/api/me/workbench?view=current", "", nil).Want(http.StatusOK).Map()
	if current["total"] != float64(1) {
		t.Fatalf("transfer lost durable responsibility: %v", current)
	}
	// Purpose is required: a body without it cannot be read as an ordinary transfer.
	call(bToken, http.MethodPost, base+"/handoffs", uuid.NewString(), input("3", c)).Want(http.StatusBadRequest)
	verify(call(bToken, http.MethodPost, base+"/participants", uuid.NewString(), input("3", c)), "participant_added", "4")
	verify(call(bToken, http.MethodPost, base+"/handoffs", uuid.NewString(), transfer("4", c)), "transferred", "5")
	call(bToken, http.MethodPost, base+"/participants", uuid.NewString(), input("5", d)).Want(http.StatusForbidden)
	leaveKey := uuid.NewString()
	refusal := call(cToken, http.MethodPost, base+"/leave", leaveKey, input("5", "")).Want(http.StatusConflict).Map()
	if refusal["code"] != "handler_must_transfer" {
		t.Fatalf("handler leave refusal: %v", refusal)
	}
	verify(call(testToken, http.MethodPost, base+"/participants/"+c+"/remove", uuid.NewString(), input("5", "")), "participant_removed", "6")
	var handler string
	fx.QueryRow(t, `SELECT assignee_id::text FROM issue WHERE id=$1`, issue).Scan(&handler)
	if handler != testUserID {
		t.Fatal("removal did not atomically return responsibility to creator")
	}
	call(cToken, http.MethodGet, base, "", nil).Want(http.StatusNotFound)
	verify(call(cToken, http.MethodGet, base+"/operations/"+leaveKey, "", nil), "handler_must_transfer", "5")
	key = uuid.NewString()
	verify(call(bToken, http.MethodPost, base+"/leave", key, input("6", "")), "participant_left", "7")
	call(bToken, http.MethodGet, base, "", nil).Want(http.StatusNotFound)
	verify(call(bToken, http.MethodGet, base+"/operations/"+key, "", nil), "participant_left", "7")
	call(testToken, http.MethodGet, base+"/operations/"+key, "", nil).Want(http.StatusNotFound)
	if fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=ANY($2::uuid[])`, testWorkspaceID, []string{b, c}) != 0 {
		t.Fatal("workflow fabricated source memberships")
	}
	if fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, issue) != 0 {
		t.Fatal("human workflow triggered ordinary AI")
	}
	for _, invalid := range []string{"0", "01", "1e1", "9223372036854775808"} {
		call(testToken, http.MethodPost, base+"/participants", uuid.NewString(), input(invalid, d)).Want(http.StatusBadRequest)
	}
	call(testToken, http.MethodPost, base+"/participants", "", input("7", d)).Want(http.StatusBadRequest)
	duplicate := fmt.Sprintf(`{"expected_collaboration_revision":"7","expected_collaboration_revision":"8","expected_lifecycle_epoch":"1","expected_participant_epoch":"1","expected_account_epoch":"1","target_user_id":%q}`, d)
	call(testToken, http.MethodPost, base+"/participants", uuid.NewString(), duplicate).Want(http.StatusBadRequest)
	if fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, issue, d) != 0 {
		t.Fatal("invalid input created a participant")
	}
}
