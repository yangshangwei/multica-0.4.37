package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestProjectIconCreateAndUpdate(t *testing.T) {
	f := projectExecutionFixture(t)
	var created ProjectResponse
	request := func(method, id string, body any) *http.Request {
		return testutil.WithURLParams(testutil.WithHeaders(testutil.JSONRequest(method, "/api/projects/"+id, body),
			"X-User-ID", f.UserID, "X-Workspace-ID", f.WorkspaceID), "id", id, "workspaceId", f.WorkspaceID)
	}
	testutil.Call(t, testHandler.CreateProject, request("POST", "", map[string]any{"title": "Icon test"})).Want(http.StatusCreated).JSON(&created)
	if created.Icon == nil || *created.Icon != "icon:package" {
		t.Fatalf("default icon = %v", created.Icon)
	}
	for _, icon := range []string{"icon:chart-no-axes-column", "icon:users", "🚀", ""} {
		var updated ProjectResponse
		testutil.Call(t, testHandler.UpdateProject, request("PATCH", created.ID, map[string]any{"icon": icon})).Want(http.StatusOK).JSON(&updated)
		if updated.Icon == nil || *updated.Icon != icon {
			t.Errorf("updated icon = %v, want %q", updated.Icon, icon)
		}
	}
	for _, icon := range []string{"icon:unknown", "icon:https://example.com/image.png", "icon:<svg>"} {
		testutil.Call(t, testHandler.CreateProject, request("POST", "", map[string]any{"title": "Invalid icon", "icon": icon})).Want(http.StatusBadRequest)
		testutil.Call(t, testHandler.UpdateProject, request("PATCH", created.ID, map[string]any{"icon": icon})).Want(http.StatusBadRequest)
	}
}
