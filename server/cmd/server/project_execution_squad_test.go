package main

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestProjectExecutionSquadRoute(t *testing.T) {
	f := testutil.New(testPool, testWorkspaceID, testUserID)
	request := func(method, path string, body any) *http.Request {
		return testutil.WithHeaders(testutil.JSONRequest(method, path, body),
			"Authorization", "Bearer "+testToken, "X-Workspace-ID", testWorkspaceID)
	}
	var created handler.ProjectResponse
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("POST", "/api/projects", map[string]any{
		"title": "Project execution squad route", "execution_squad": map[string]any{"template_key": "feature-delivery"},
	})).Want(http.StatusCreated).JSON(&created)
	f.Cleanup(t, `DELETE FROM project WHERE id = $1`, created.ID)
	if created.ExecutionSquad.State != "needs_runtime" {
		t.Fatalf("POST did not retain selection: %+v", created.ExecutionSquad)
	}
	var configured handler.ProjectResponse
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("PUT", "/api/projects/"+created.ID+"/execution-squad", map[string]any{})).Want(http.StatusOK).JSON(&configured)
	if configured.ID != created.ID || configured.Title != created.Title || configured.ExecutionSquad.State != "none" {
		t.Fatalf("PUT did not return the complete project: %+v", configured)
	}
	var stored handler.ProjectResponse
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("GET", "/api/projects/"+created.ID, nil)).Want(http.StatusOK).JSON(&stored)
	if stored.ExecutionSquad.State != "none" {
		t.Fatalf("configuration did not survive GET: %+v", stored.ExecutionSquad)
	}
}
