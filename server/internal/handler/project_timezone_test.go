package handler

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/featureflags"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/featureflag"
)

func TestProjectPlanningTimezoneContractAndHumanPermissions(t *testing.T) {
	dbfx.Cleanup(t, "UPDATE workspace SET planning_timezone=NULL WHERE id=$1", testWorkspaceID)
	call := func(body map[string]any, status int) ProjectPlanningTimezone {
		t.Helper()
		var out ProjectPlanningTimezone
		c := testutil.Call(t, testHandler.UpdateProjectPlanningTimezone, withURLParam(newRequest("PUT", "/api/workspaces/"+testWorkspaceID+"/planning-timezone", body), "id", testWorkspaceID)).Want(status)
		if status == 200 {
			c.JSON(&out)
		}
		return out
	}
	var initial ProjectPlanningTimezone
	testutil.Call(t, testHandler.GetProjectPlanningTimezone, withURLParam(newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/planning-timezone", nil), "id", testWorkspaceID)).Want(200).JSON(&initial)
	if initial.Configured || initial.EffectiveTimezone != "Asia/Shanghai" || initial.PlanningTimezone != nil {
		t.Fatalf("default: %+v", initial)
	}
	assertIterationSettings := func(zone string, configured bool) {
		t.Helper()
		var settings iteration.Settings
		testutil.Call(t, testHandler.GetIterationSettings, iterationSettingsRequest("GET", "iteration-settings", nil)).Want(200).JSON(&settings)
		if settings.EffectiveTimezone != zone || settings.TimezoneConfigured != configured || (settings.PlanningTimezone != nil) != configured {
			t.Fatalf("iteration planning timezone: %+v", settings)
		}
		if configured && *settings.PlanningTimezone != zone {
			t.Fatalf("iteration configured timezone: %+v", settings)
		}
	}
	assertIterationSettings("Asia/Shanghai", false)
	for _, zone := range []string{"UTC", "America/New_York"} {
		out := call(map[string]any{"planning_timezone": zone}, 200)
		if !out.Configured || out.EffectiveTimezone != zone || out.PlanningTimezone == nil || *out.PlanningTimezone != zone || out.WorkspaceID != testWorkspaceID {
			t.Fatalf("configured: %+v", out)
		}
		assertIterationSettings(zone, true)
	}
	// Ordinary settings replacement cannot overwrite the dedicated planning field.
	testutil.Call(t, testHandler.UpdateWorkspace, withURLParam(newRequest("PUT", "/api/workspaces/"+testWorkspaceID, map[string]any{"settings": map[string]any{}}), "id", testWorkspaceID)).Want(200)
	var persisted string
	dbfx.QueryRow(t, "SELECT planning_timezone FROM workspace WHERE id=$1", testWorkspaceID).Scan(&persisted)
	if persisted != "America/New_York" {
		t.Fatalf("settings lost timezone: %s", persisted)
	}
	for _, zone := range []string{"Local", "UTC+08:00", "", "Invalid/Place"} {
		call(map[string]any{"planning_timezone": zone}, 422)
	}
	call(map[string]any{}, 400)
	for _, source := range []string{"task_token", "cloud_pat"} {
		r := withURLParam(newRequest("PUT", "/api/workspaces/"+testWorkspaceID+"/planning-timezone", map[string]any{"planning_timezone": "UTC"}), "id", testWorkspaceID)
		r.Header.Set("X-Actor-Source", source)
		testutil.Call(t, testHandler.UpdateProjectPlanningTimezone, r).Want(403)
	}
	member := dbfx.User(t, "P1 member", "p1-timezone@example.test")
	dbfx.Member(t, testWorkspaceID, member, "member")
	r := withURLParam(newRequest("PUT", "/api/workspaces/"+testWorkspaceID+"/planning-timezone", map[string]any{"planning_timezone": "UTC"}), "id", testWorkspaceID)
	r.Header.Set("X-User-ID", member)
	testutil.Call(t, testHandler.UpdateProjectPlanningTimezone, r).Want(403)
	out := call(map[string]any{"planning_timezone": nil}, 200)
	if out.Configured || out.EffectiveTimezone != "Asia/Shanghai" || out.PlanningTimezone != nil {
		t.Fatalf("clear: %+v", out)
	}
	assertIterationSettings("Asia/Shanghai", false)
	var cleared pgtype.Text
	dbfx.QueryRow(t, "SELECT planning_timezone FROM workspace WHERE id=$1", testWorkspaceID).Scan(&cleared)
	if cleared.Valid {
		t.Fatalf("clearing persisted a configured timezone: %+v", cleared)
	}
}

func TestProjectCapabilitiesRequiresCurrentMembership(t *testing.T) {
	var out map[string]any
	req := withURLParam(newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/project-capabilities", nil), "id", testWorkspaceID)
	testutil.Call(t, testHandler.GetProjectCapabilities, req).Want(200).JSON(&out)
	if out["workspace_id"] != testWorkspaceID || out["description_cas"] != true || out["planning_timezone"] != true {
		t.Fatalf("capabilities: %v", out)
	}
	outsider := dbfx.User(t, "P1 outsider", "p1-capability@example.test")
	req = withURLParam(newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/project-capabilities", nil), "id", testWorkspaceID)
	req.Header.Set("X-User-ID", outsider)
	testutil.Call(t, testHandler.GetProjectCapabilities, req).Want(403)
}

func TestProjectCapabilitiesReadOnlyRollbackKeepsCASAndTimezone(t *testing.T) {
	h := *testHandler
	provider := featureflag.NewStaticProvider()
	provider.Set(featureflags.ProjectsP1, featureflag.Rule{Default: false})
	h.FeatureFlags = featureflag.NewService(provider)
	var out map[string]any
	req := withURLParam(newRequest("GET", "/api/workspaces/"+testWorkspaceID+"/project-capabilities", nil), "id", testWorkspaceID)
	testutil.Call(t, h.GetProjectCapabilities, req).Want(200).JSON(&out)
	if out["overview"] != false || out["updates"] != false || out["description_cas"] != true || out["planning_timezone"] != true {
		t.Fatalf("rollback capabilities: %v", out)
	}
	id := dbfx.Project(t, "rollback description", testutil.Cols{"description": "original"})
	testutil.Call(t, h.UpdateProject, withURLParam(newRequest("PUT", "/api/projects/"+id, map[string]any{"description": "unversioned"}), "id", id)).Want(428)
	testutil.Call(t, h.GetProject, withURLParam(newRequest("GET", "/api/projects/"+id, nil), "id", id)).Want(200)
}
