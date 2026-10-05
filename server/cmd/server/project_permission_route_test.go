package main

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

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
