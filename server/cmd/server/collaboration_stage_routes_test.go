package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The service and handler suites own the stage matrices. This test proves the
// production router: real JWT authentication, the account route group, the
// project route group with its human-actor guard, and the temporary gating that
// keeps a stage policy from being enabled over HTTP.
func TestCollaborationStageRoutesAreRegisteredAndEnablingStaysUnavailable(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router fixture required")
	}
	base := testutil.New(testPool, testWorkspaceID, testUserID)
	source := base.Workspace(t, "Stage source", "collaboration-stage-"+uuid.NewString())
	base.Member(t, source, testUserID, "owner")
	fx := testutil.New(testPool, source, testUserID)
	project := fx.Project(t, "Stage governed project")
	issue := fx.Issue(t, "Stage governed root", testutil.Cols{"assignee_type": "member", "assignee_id": testUserID, "project_id": project})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
	fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
	fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": testUserID}, "issue_id=$1 AND user_id=$2", issue, testUserID)
	ensureCollaborationRouteAccountFence(t, fx, testUserID)
	for _, table := range []string{"issue_participant", "collaboration_event", "collaboration_notification"} {
		fx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", issue)
	}
	fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
	for _, table := range []string{"collaboration_stage_child", "collaboration_stage_decision", "collaboration_stage_candidate", "collaboration_stage_role", "collaboration_stage_task", "project_stage_policy"} {
		fx.Cleanup(t, `DELETE FROM `+table+` WHERE workspace_id=$1`, source)
	}
	call := func(token, method, path, key string, body any, headers ...string) *testutil.Response {
		t.Helper()
		req := testutil.JSONRequest(method, path, body)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("X-Workspace-ID", source)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		testutil.WithHeaders(req, headers...)
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
	}
	stage, policy := "/api/collaboration/issues/"+issue+"/stage", "/api/projects/"+project+"/stage-policy"
	rows := func() int {
		return fx.Count(t, `SELECT count(*) FROM project_stage_policy WHERE project_id=$1`, project)
	}
	observations := func() map[string]any {
		t.Helper()
		current := call(testToken, http.MethodGet, "/api/collaboration/issues/"+issue, "", nil).Want(http.StatusOK).Map()
		write := current["write_context"].(map[string]any)
		return map[string]any{"expected_collaboration_revision": current["collaboration_revision"], "expected_lifecycle_epoch": current["lifecycle_epoch"],
			"expected_participant_epoch": write["participant_epoch"], "expected_account_epoch": write["account_epoch"]}
	}
	with := func(fields map[string]any) map[string]any {
		body := observations()
		for key, value := range fields {
			body[key] = value
		}
		return body
	}

	call("", http.MethodGet, stage, "", nil).Want(http.StatusUnauthorized)
	view := call(testToken, http.MethodGet, stage, "", nil).Want(http.StatusOK).Map()
	if view["managed"] != false || view["issue_id"] != issue || view["policy"].(map[string]any)["state"] != "none" {
		t.Fatalf("unmanaged stage view through the router: %v", view)
	}
	// An agent-actor hint on a human token is not a human reader or writer.
	call(testToken, http.MethodGet, stage, "", nil, "X-Agent-ID", uuid.NewString()).WantOneOf(http.StatusForbidden, http.StatusNotFound)

	enable := map[string]any{"expected_policy_revision": "0", "enabled": true, "reason": "switch the stage gates on"}
	if out := call(testToken, http.MethodPut, policy, uuid.NewString(), enable).Want(http.StatusServiceUnavailable).Map(); out["code"] != "stage_policy_unavailable" || rows() != 0 {
		t.Fatalf("the production router enabled a stage policy: %v rows=%d", out, rows())
	}
	call("", http.MethodPut, policy, uuid.NewString(), enable).Want(http.StatusUnauthorized)
	call(testToken, http.MethodPut, policy, uuid.NewString(), enable, "X-Agent-ID", uuid.NewString(), "X-Task-ID", uuid.NewString()).Want(http.StatusForbidden)
	if read := call(testToken, http.MethodGet, policy, "", nil).Want(http.StatusOK).Map(); read["policy_revision"] != "0" || read["activation_available"] != false {
		t.Fatalf("policy read through the router: %v", read)
	}
	if rows() != 0 {
		t.Fatal("a refused or read request wrote a policy revision")
	}

	// Issue-level stage routes exist and refuse while the project is unmanaged.
	key := uuid.NewString()
	roles := map[string]any{"business_owner_user_id": testUserID, "implementer_user_id": testUserID, "product_approver_user_id": testUserID,
		"code_reviewer_user_id": uuid.NewString(), "qa_approver_user_id": testUserID, "release_approver_user_id": uuid.NewString()}
	refused := call(testToken, http.MethodPost, stage+"/roles", key, with(map[string]any{"expected_policy_revision": "1", "expected_role_revision": "0", "roles": roles})).Want(http.StatusConflict).Map()
	if refused["code"] != "stage_policy_disabled" || refused["status"] != float64(http.StatusConflict) {
		t.Fatalf("stored refusal through the router: %v", refused)
	}
	if recovered := call(testToken, http.MethodGet, "/api/collaboration/issues/"+issue+"/operations/"+key, "", nil).Want(http.StatusOK).Map(); recovered["code"] != "stage_policy_disabled" || recovered["status"] != float64(http.StatusConflict) {
		t.Fatalf("original operation recovery through the router: %v", recovered)
	}
	proposal := uuid.NewString()
	for _, path := range []string{"/proposals", "/proposals/" + proposal + "/versions", "/proposals/" + proposal + "/decisions", "/scope-confirmations"} {
		if out := call(testToken, http.MethodPost, stage+path, uuid.NewString(), `{"unknown":true}`).Want(http.StatusBadRequest).Map(); out["code"] != "invalid_request" {
			t.Fatalf("stage route %s is not registered with the strict decoder: %v", path, out)
		}
	}
	call(testToken, http.MethodGet, stage+"/proposals", "", nil).Want(http.StatusOK)
	call(testToken, http.MethodGet, stage+"/proposals/"+proposal, "", nil).Want(http.StatusNotFound)

	// Only the service can enable in this increment; the routes then act on it.
	svc := service.NewCollaborationService(db.New(testPool), testPool, nil)
	if _, err := svc.ReviseStagePolicy(t.Context(), service.StagePolicyRevisionParams{WorkspaceID: util.MustParseUUID(source), ProjectID: util.MustParseUUID(project),
		RequestID: uuid.NewString(), ExpectedRevision: 0, Enabled: true, Reason: "router fixture",
		Credential: service.ArtifactCredentialSnapshot{Principal: auth.AuthenticatedPrincipal{UserID: testUserID, Kind: auth.CredentialJWT, ExpiresAt: time.Now().Add(time.Hour)}}}); err != nil {
		t.Fatal(err)
	}
	view = call(testToken, http.MethodGet, stage, "", nil).Want(http.StatusOK).Map()
	missing, _ := view["missing"].([]any)
	if view["managed"] != true || view["needs_confirmation"] != true || len(missing) != 2 || view["stage"].(map[string]any)["current"] != "development" {
		t.Fatalf("managed stage view through the router: %v", view)
	}
	created := call(testToken, http.MethodPost, stage+"/proposals", uuid.NewString(), with(map[string]any{"expected_policy_revision": "1",
		"content": map[string]any{"summary": "", "children": []any{map[string]any{"title": "Router child", "description": "", "priority": "none"}}},
		"source":  map[string]any{"kind": "manual"}})).Want(http.StatusOK).Map()
	if confirmation, _ := created["stage_confirmation"].(map[string]any); created["code"] != "stage_proposal_created" || confirmation["action"] != "create_proposal" ||
		fx.Count(t, `SELECT count(*) FROM issue WHERE parent_issue_id=$1`, issue) != 0 {
		t.Fatalf("proposal creation through the router: %v", created)
	}
	// The explicit rollback path stays available, with its own original-key read.
	disableKey := uuid.NewString()
	disabled := call(testToken, http.MethodPut, policy, disableKey, map[string]any{"expected_policy_revision": "1", "enabled": false, "reason": "explicit rollback"}).Want(http.StatusOK).Map()
	if confirmation, _ := disabled["stage_confirmation"].(map[string]any); confirmation["policy_revision"] != "2" || confirmation["enabled"] != false {
		t.Fatalf("disable through the router: %v", disabled)
	}
	if recovered := call(testToken, http.MethodGet, policy+"/operations/"+disableKey, "", nil).Want(http.StatusOK).Map(); recovered["code"] != "stage_policy_revised" {
		t.Fatalf("policy operation recovery through the router: %v", recovered)
	}
	if out := call(testToken, http.MethodPut, policy, uuid.NewString(), map[string]any{"expected_policy_revision": "2", "enabled": true, "reason": "again"}).Want(http.StatusServiceUnavailable).Map(); out["code"] != "stage_policy_unavailable" || rows() != 2 {
		t.Fatalf("re-enable through the router: %v rows=%d", out, rows())
	}
}
