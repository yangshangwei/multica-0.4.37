package main

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// The domain matrix lives in internal/handler and internal/service. This test
// exercises real authentication and route placement: an account route must not
// accidentally inherit workspace membership or trust actor headers.
func TestCollaborationRoutesUseAuthenticatedAccountWithoutSourceMembership(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router DB fixture is required")
	}
	fx := testutil.New(testPool, testWorkspaceID, testUserID)
	externalEmail := "collaboration-router-" + uuid.NewString() + "@example.test"
	external := fx.User(t, "External collaboration handler", externalEmail)
	otherWorkspace := fx.Workspace(t, "Independent workspace", "collaboration-router-"+uuid.NewString())
	fx.Member(t, otherWorkspace, external, "owner")
	outsiderEmail := "collaboration-outsider-" + uuid.NewString() + "@example.test"
	outsider := fx.User(t, "Unrelated account", outsiderEmail)
	const privateBody = "route-private-description-sentinel"
	issueID := fx.Issue(t, "Shared route fixture", testutil.Cols{
		"description": privateBody, "assignee_type": "member", "assignee_id": external,
	})
	fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issueID}, "issue_id=$1", issueID)
	fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issueID})
	for _, participant := range []string{testUserID, external} {
		fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issueID, "user_id": participant},
			"issue_id=$1 AND user_id=$2", issueID, participant)
	}
	externalJWT, err := generateTestJWT(external, externalEmail, "External collaboration handler")
	if err != nil {
		t.Fatal(err)
	}
	outsiderJWT, err := generateTestJWT(outsider, outsiderEmail, "Unrelated account")
	if err != nil {
		t.Fatal(err)
	}
	pat, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	fx.Insert(t, "personal_access_token", testutil.Cols{
		"user_id": external, "name": "Collaboration route human PAT", "token_hash": auth.HashToken(pat),
		"token_prefix": pat[:12], "expires_at": time.Now().Add(time.Hour),
	})
	call := func(token, method, path string) *testutil.Response {
		t.Helper()
		req := testutil.JSONRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Workspace-ID", "invalid-ambient-workspace")
		req.Header.Set("X-Workspace-Slug", "unrelated-space")
		req.Header.Set("X-Actor-Type", "member")
		req.Header.Set("X-Actor-ID", testUserID)
		return testutil.Call(t, testServer.Config.Handler.ServeHTTP, req)
	}
	path := "/api/collaboration/issues/" + issueID
	for _, token := range []string{externalJWT, pat} {
		response := call(token, http.MethodGet, path).Want(http.StatusOK)
		body := response.Map()
		if body["id"] != issueID || body["description"] != privateBody || body["collaboration_revision"] != "1" {
			t.Fatalf("wrong account-scoped projection: %v", body)
		}
		if response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Text(), externalEmail) {
			t.Fatal("private account response was cacheable or disclosed email")
		}
		workbench := call(token, http.MethodGet, "/api/me/workbench?view=current").Want(http.StatusOK).Map()
		if workbench["total"] != float64(1) {
			t.Fatalf("independent account lost current responsibility: %v", workbench)
		}
	}
	if fx.Count(t, `SELECT count(*) FROM member WHERE workspace_id=$1 AND user_id=$2`, testWorkspaceID, external) != 0 {
		t.Fatal("account read fabricated source membership")
	}
	call(outsiderJWT, http.MethodGet, path).Want(http.StatusNotFound)
	call(testToken, http.MethodPost, "/api/collaboration/issues").Want(http.StatusNotFound)

	// This machine owns a real running task and carries the source owner's ID.
	// Neither that attribution nor its forged human headers grants content.
	runtimeID := fx.Runtime(t, "Collaboration route runtime")
	agentID := fx.Agent(t, "Collaboration route agent", runtimeID)
	taskID := fx.Task(t, agentID, testutil.Cols{"status": "running", "runtime_id": runtimeID})
	taskToken, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	fx.Insert(t, "task_token", testutil.Cols{
		"token_hash": auth.HashToken(taskToken), "task_id": taskID, "agent_id": agentID,
		"workspace_id": testWorkspaceID, "user_id": testUserID, "expires_at": time.Now().Add(time.Hour),
	})
	call(taskToken, http.MethodGet, path).Want(http.StatusNotFound)
	call(taskToken, http.MethodGet, "/api/me/workbench").Want(http.StatusForbidden)

	fx.Exec(t, `UPDATE issue_participant SET revoked_at=now(), epoch=epoch+1 WHERE issue_id=$1 AND user_id=$2`, issueID, external)
	for _, token := range []string{externalJWT, pat} {
		call(token, http.MethodGet, path).Want(http.StatusNotFound)
		workbench := call(token, http.MethodGet, "/api/me/workbench?view=participating").Want(http.StatusOK).Map()
		if workbench["total"] != float64(0) {
			t.Fatal("revoked participant retained account workbench entry")
		}
	}
	call(testToken, http.MethodGet, path).Want(http.StatusOK)
}

func TestCollaborationRoutesDenyLegacyAgentContext(t *testing.T) {
	if testPool == nil || testServer == nil {
		t.Fatal("isolated real-router DB fixture is required")
	}
	for _, mode := range []string{"legacy", "password"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", mode)
			fx := testutil.New(testPool, "", "")
			actor := fx.User(t, "Legacy agent credential owner", "legacy-actor-"+uuid.NewString()+"@example.invalid")
			fx.UserID = actor
			fx.WorkspaceID = fx.Workspace(t, "Legacy actor boundary", "legacy-actor-"+uuid.NewString())
			fx.Member(t, fx.WorkspaceID, actor, "owner")
			if mode == "password" {
				fx.InsertNoID(t, "user_password_credential", testutil.Cols{
					"user_id": actor, "username": "legacy_actor_" + uuid.NewString(), "password_hash": "unused-by-versioned-token-authentication",
				}, "user_id=$1", actor)
			}
			pat, err := auth.GeneratePATToken()
			if err != nil {
				t.Fatal(err)
			}
			fx.Insert(t, "personal_access_token", testutil.Cols{
				"user_id": actor, "name": "Legacy actor boundary PAT", "token_hash": auth.HashToken(pat),
				"token_prefix": pat[:12], "auth_version": 1, "expires_at": time.Now().Add(time.Hour),
			})
			const privateBody = "legacyagentprivatebodysentinel"
			issue := fx.Issue(t, "Legacy actor reference", testutil.Cols{"description": privateBody, "assignee_type": "member", "assignee_id": actor})
			fx.InsertNoID(t, "issue_collaboration", testutil.Cols{"issue_id": issue}, "issue_id=$1", issue)
			fx.Insert(t, "collaboration_session", testutil.Cols{"issue_id": issue})
			fx.InsertNoID(t, "issue_participant", testutil.Cols{"issue_id": issue, "user_id": actor}, "issue_id=$1 AND user_id=$2", issue, actor)
			fx.InsertNoID(t, "collaboration_account_fence", testutil.Cols{"user_id": actor}, "user_id=$1", actor)
			invited := fx.User(t, "Human invitation control", uuid.NewString()+"@example.invalid")
			denied := fx.User(t, "Agent invitation target", uuid.NewString()+"@example.invalid")
			if mode == "password" {
				for _, target := range []string{invited, denied} {
					fx.InsertNoID(t, "user_password_credential", testutil.Cols{
						"user_id": target, "username": "recipient_" + uuid.NewString(), "password_hash": "unused-by-versioned-token-authentication",
					}, "user_id=$1", target)
				}
			}
			fx.Cleanup(t, `DELETE FROM collaboration_account_fence WHERE user_id=ANY($1::uuid[])`, []string{invited, denied})
			for _, table := range []string{"issue_participant", "collaboration_event", "collaboration_notification"} {
				fx.Cleanup(t, "DELETE FROM "+table+" WHERE issue_id=$1", issue)
			}
			fx.Cleanup(t, `DELETE FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1`, issue)
			runtime := fx.Runtime(t, "Legacy agent runtime")
			agent := fx.Agent(t, "Legacy agent", runtime)
			task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "status": "running"})
			base := "/api/collaboration/issues/" + issue
			key := uuid.NewString()
			body := func(revision, target string) map[string]string {
				return map[string]string{"expected_collaboration_revision": revision, "expected_lifecycle_epoch": "1",
					"expected_participant_epoch": "1", "expected_account_epoch": "1", "target_user_id": target}
			}
			call := func(t *testing.T, token, method, path, requestID string, body any, agentContext bool) *testutil.Response {
				t.Helper()
				r := testutil.JSONRequest(method, path, body)
				r.Header.Set("Authorization", "Bearer "+token)
				r.Header.Set("X-Workspace-ID", fx.WorkspaceID)
				if requestID != "" {
					r.Header.Set("Idempotency-Key", requestID)
				}
				if agentContext {
					r.Header.Set("X-Agent-ID", agent)
					r.Header.Set("X-Task-ID", task)
				}
				return testutil.Call(t, testServer.Config.Handler.ServeHTTP, r)
			}
			for _, credential := range []struct{ name, token string }{{"jwt", platformRouterJWT(t, actor, nil)}, {"pat", pat}} {
				t.Run(credential.name, func(t *testing.T) {
					// The same verified credential retains human access and can
					// create/replay its original workflow intent without agent hints.
					call(t, credential.token, http.MethodPost, base+"/participants", key, body("1", invited), false).Want(http.StatusOK)
					ordinary := call(t, credential.token, http.MethodGet, base, "", nil, false).Want(http.StatusOK).Map()
					if ordinary["description"] != privateBody {
						t.Fatal("ordinary human credential lost private read authority")
					}
					call(t, credential.token, http.MethodGet, base+"/operations/"+key, "", nil, false).Want(http.StatusOK)
					call(t, credential.token, http.MethodGet, base, "", nil, true).Want(http.StatusNotFound)
					call(t, credential.token, http.MethodGet, base+"/field-options", "", nil, true).Want(http.StatusNotFound)
					call(t, credential.token, http.MethodPost, "/api/collaboration/recipients/resolve", "", map[string]any{
						"kind": "user_id", "value": invited, "issue_id": issue,
					}, true).Want(http.StatusForbidden)
					call(t, credential.token, http.MethodGet, "/api/me/workbench", "", nil, true).Want(http.StatusForbidden)
					call(t, credential.token, http.MethodGet, base+"/operations/"+key, "", nil, true).Want(http.StatusNotFound)
					call(t, credential.token, http.MethodPost, base+"/participants", uuid.NewString(), body("2", denied), true).Want(http.StatusForbidden)
					search := call(t, credential.token, http.MethodGet, "/api/issues/search?q="+url.QueryEscape(privateBody), "", nil, true).Want(http.StatusOK).Map()
					if search["total"] != float64(0) {
						t.Fatalf("legacy agent searched private collaboration content: %v", search)
					}
					call(t, credential.token, http.MethodGet, "/api/issues/"+issue, "", nil, true).Want(http.StatusNotFound)
					references := call(t, credential.token, http.MethodGet, "/api/issues/search?q="+url.QueryEscape("Legacy actor reference"), "", nil, true).Want(http.StatusOK).Map()
					items, ok := references["issues"].([]any)
					if !ok || len(items) != 1 {
						t.Fatalf("legacy agent lost permitted title reference: %v", references)
					}
					reference, ok := items[0].(map[string]any)
					if !ok || len(reference) != 7 || reference["collaboration_reference"] != true || reference["can_open"] != false || reference["id"] != issue {
						t.Fatalf("legacy agent received more than a source reference: %v", reference)
					}
				})
			}
			if fx.Count(t, `SELECT count(*) FROM issue_participant WHERE issue_id=$1 AND user_id=$2`, issue, denied) != 0 ||
				fx.Count(t, `SELECT count(*) FROM collaboration_operation WHERE scope_id=$1`, issue) != 1 {
				t.Fatal("legacy agent workflow mutated participation or operation history")
			}
		})
	}
}
