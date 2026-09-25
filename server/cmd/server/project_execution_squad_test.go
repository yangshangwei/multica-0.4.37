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

func TestProjectExecutionSquadsRoute(t *testing.T) {
	f := testutil.New(testPool, testWorkspaceID, testUserID)
	request := func(method, path string, body any) *http.Request {
		return testutil.WithHeaders(testutil.JSONRequest(method, path, body),
			"Authorization", "Bearer "+testToken, "X-Workspace-ID", testWorkspaceID)
	}
	var created handler.ProjectResponse
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("POST", "/api/projects", map[string]any{
		"title": "Multiple execution squads route", "execution_squads": []map[string]any{{"template_key": "feature-delivery"}, {"template_key": "docs"}},
	})).Want(http.StatusCreated).JSON(&created)
	f.Cleanup(t, `DELETE FROM project WHERE id = $1`, created.ID)
	if len(created.ExecutionSquads) != 2 || created.ExecutionSquad != created.ExecutionSquads[0] {
		t.Fatalf("POST lost ordered choices: %+v", created)
	}
	var updated handler.ProjectResponse
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("PUT", "/api/projects/"+created.ID+"/execution-squads", map[string]any{
		"squads": []map[string]any{{"template_key": "docs"}, {"template_key": "bug-fix"}},
	})).Want(http.StatusOK).JSON(&updated)
	if updated.Title != created.Title || len(updated.ExecutionSquads) != 2 || updated.ExecutionSquad.TemplateKey != "docs" {
		t.Fatalf("plural PUT did not replace choices: %+v", updated)
	}
	var stored handler.ProjectResponse
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("GET", "/api/projects/"+created.ID, nil)).Want(http.StatusOK).JSON(&stored)
	if len(stored.ExecutionSquads) != 2 || stored.ExecutionSquads[1].TemplateKey != "bug-fix" {
		t.Fatalf("GET lost plural choices: %+v", stored)
	}
	var list, search struct {
		Projects []handler.ProjectResponse `json:"projects"`
	}
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("GET", "/api/projects", nil)).Want(http.StatusOK).JSON(&list)
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("GET", "/api/projects/search?q=Multiple", nil)).Want(http.StatusOK).JSON(&search)
	for _, collection := range [][]handler.ProjectResponse{list.Projects, search.Projects} {
		found := false
		for _, project := range collection {
			if project.ID == created.ID {
				found = true
				if len(project.ExecutionSquads) != 2 || project.ExecutionSquad.TemplateKey != "docs" {
					t.Fatalf("list/search lost choices: %+v", project)
				}
			}
		}
		if !found {
			t.Fatal("list/search omitted created project")
		}
	}
	var cleared handler.ProjectResponse
	testutil.Call(t, testServer.Config.Handler.ServeHTTP, request("PUT", "/api/projects/"+created.ID+"/execution-squads", map[string]any{"squads": []any{}})).Want(http.StatusOK).JSON(&cleared)
	if cleared.ExecutionSquads == nil || len(cleared.ExecutionSquads) != 0 || cleared.ExecutionSquad.State != "none" {
		t.Fatalf("plural clear failed: %+v", cleared)
	}
}
