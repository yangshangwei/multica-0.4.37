package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestProjectOperationDenialDoesNotClaimWorkspaceRevocation(t *testing.T) {
	project := dbfx.Project(t, "permission error contract")
	member := dbfx.User(t, "Project member", "project-permission-member@example.test")
	dbfx.Member(t, testWorkspaceID, member, "member")
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		path    string
		id      string
		body    map[string]any
	}{
		{"delete", testHandler.DeleteProject, "/api/projects/" + project, project, nil},
		{"timezone", testHandler.UpdateProjectPlanningTimezone, "/api/workspaces/" + testWorkspaceID + "/planning-timezone", testWorkspaceID, map[string]any{"planning_timezone": "UTC"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			method := http.MethodPut
			if tc.name == "delete" {
				method = http.MethodDelete
			}
			req := withURLParam(newRequest(method, tc.path, tc.body), "id", tc.id)
			req.Header.Set("X-User-ID", member)
			var out map[string]any
			testutil.Call(t, tc.handler, req).Want(403).JSON(&out)
			if out["code"] != "project_permission_denied" {
				t.Fatalf("operation denial must preserve readable workspace state: %v", out)
			}
			read := withURLParam(newRequest("GET", "/api/projects/"+project, nil), "id", project)
			read.Header.Set("X-User-ID", member)
			testutil.Call(t, testHandler.GetProject, read).Want(200)
		})
	}
}

func TestProjectMachineActionDenialIsOperationScoped(t *testing.T) {
	request := withURLParam(newRequest("PUT", "/api/workspaces/"+testWorkspaceID+"/planning-timezone", map[string]any{"planning_timezone": "UTC"}), "id", testWorkspaceID)
	request.Header.Set("X-Actor-Source", "task_token")
	var out map[string]any
	testutil.Call(t, testHandler.UpdateProjectPlanningTimezone, request).Want(403).JSON(&out)
	if out["code"] != "project_permission_denied" {
		t.Fatalf("machine action denial: %v", out)
	}
}

func TestProjectMissingMembershipStillSignalsRevocation(t *testing.T) {
	user := dbfx.User(t, "Former project member", "project-permission-outsider@example.test")
	request := withURLParam(newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/project-capabilities", nil), "id", testWorkspaceID)
	request.Header.Set("X-User-ID", user)
	var out map[string]any
	testutil.Call(t, testHandler.GetProjectCapabilities, request).Want(403).JSON(&out)
	if out["code"] != "forbidden" {
		t.Fatalf("lost membership must still clear protected workspace state: %v", out)
	}
}
