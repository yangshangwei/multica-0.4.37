package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestKnowledgePolicyRecoveryRouteSurvivesWorkspaceDeparture(t *testing.T) {
	seed := testutil.New(testPool, testWorkspaceID, testUserID)
	email := "knowledge-route-" + uuid.NewString() + "@example.test"
	user := seed.User(t, "Knowledge project lead", email)
	workspace := seed.Workspace(t, "Knowledge policy route", "knowledge-route-"+uuid.NewString())
	f := testutil.New(testPool, workspace, user)
	f.Member(t, workspace, testUserID, "owner")
	f.Member(t, workspace, user, "member")
	project := f.Project(t, "Knowledge policy recovery", testutil.Cols{"lead_id": user, "lead_type": "member"})
	f.Cleanup(t, `DELETE FROM project_knowledge_policy WHERE project_id=$1`, project)
	token, err := generateTestJWT(user, email, "Knowledge project lead")
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, body any) *http.Request {
		return testutil.WithHeaders(testutil.JSONRequest(method, path, body), "Authorization", "Bearer "+token,
			"X-Workspace-ID", workspace, handler.HeaderKnowledgeVersion, "2")
	}
	path := "/api/projects/" + project + "/knowledge-policy"
	var policy service.ProjectKnowledgePolicyView
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("GET", path, nil)).Want(200).JSON(&policy)
	if policy.Revision != "0" || !policy.IsEnabled || !policy.CanManage {
		t.Fatal("current project lead cannot read the built-in policy")
	}
	id := uuid.NewString()
	var receipt service.ProjectKnowledgePolicyReceipt
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("PUT", path, service.ProjectKnowledgePolicyRequest{
		RequestID: id, ExpectedRevision: "0", IsEnabled: false, MaxCandidates: 1,
	})).Want(200).JSON(&receipt)
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("POST", "/api/workspaces/"+workspace+"/leave", nil)).Want(204)
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("GET", path, nil)).Want(404)
	for _, sendWorkspace := range []bool{true, false} {
		r := request("GET", path+"/operations/"+id, nil)
		if !sendWorkspace {
			r.Header.Del("X-Workspace-ID")
		}
		var recovered service.ProjectKnowledgePolicyReceipt
		testutil.Call(t, testServer.Config.Handler.ServeHTTP, r).Want(200).JSON(&recovered)
		if recovered != receipt {
			t.Fatal("actual project route lost the original receipt after workspace departure")
		}
	}
}

func TestProjectPermissionErrorsSurviveActualWorkspaceRoutes(t *testing.T) {
	seed := testutil.New(testPool, testWorkspaceID, testUserID)
	email := "p1-route-" + uuid.NewString() + "@example.test"
	user := seed.User(t, "P1 route member", email)
	ws := seed.Workspace(t, "P1 route permissions", "p1-route-"+uuid.NewString())
	f := testutil.New(testPool, ws, user)
	f.Member(t, ws, testUserID, "owner")
	f.Member(t, ws, user, "member")
	project := f.Project(t, "P1 route project")
	token, err := generateTestJWT(user, email, "P1 route member")
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, body any) *http.Request {
		return testutil.WithHeaders(testutil.JSONRequest(method, path, body), "Authorization", "Bearer "+token, "X-Workspace-ID", ws)
	}
	var denied map[string]any
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("PUT", "/api/workspaces/"+ws+"/planning-timezone", map[string]any{"planning_timezone": "UTC"})).Want(403).JSON(&denied)
	if denied["code"] != "project_permission_denied" {
		t.Errorf("an operation denial must not revoke a readable workspace: %v", denied)
	}
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("GET", "/api/projects/"+project, nil)).Want(200)
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("POST", "/api/workspaces/"+ws+"/leave", nil)).Want(204)
	for _, path := range []string{"/api/projects/" + project, "/api/projects/" + project + "/overview"} {
		var lost map[string]any
		testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("GET", path, nil)).Want(404).JSON(&lost)
		if lost["code"] != "workspace_access_denied" || lost["error"] != "workspace not found" {
			t.Errorf("membership loss must remain private and machine-readable: %v", lost)
		}
	}
}
