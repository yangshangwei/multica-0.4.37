package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestAdminOverviewFinishedWindowSamplesAndDrilldown(t *testing.T) {
	login := adminHandlerSetup(t)
	t.Setenv("MULTICA_DEPLOYMENT_ID", uuid.NewString())
	ws := dbfx.Workspace(t, "Overview fixture", "overview-"+uuid.NewString())
	org, err := testHandler.Queries.AssignWorkspaceOrganization(t.Context(), parseUUID(ws))
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, "DELETE FROM organization_workspace WHERE workspace_id=$1", ws)
	fx := testutil.New(testPool, ws, login.User.ID)
	runtimeID := fx.Runtime(t, "PRIVATE runtime")
	agentID := fx.Agent(t, "PRIVATE agent", runtimeID)
	from := time.Date(2026, 3, 8, 5, 0, 0, 0, time.UTC)
	to := from.Add(23 * time.Hour)
	base := from.Add(-60 * 24 * time.Hour)
	completed := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "status": "completed", "created_at": base, "queued_at": from.Add(-10 * time.Minute), "queued_at_source": "transition", "dispatched_at": from, "started_at": from, "completed_at": from.Add(10 * time.Minute)})
	failed := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "status": "failed", "created_at": base, "queued_at": from, "queued_at_source": "observation", "dispatched_at": from.Add(2 * time.Minute), "started_at": from.Add(2 * time.Minute), "completed_at": from.Add(20 * time.Minute), "error": "PRIVATE error"})
	cancelled := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "status": "cancelled", "created_at": base, "completed_at": from.Add(30 * time.Minute)})
	fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "status": "completed", "created_at": from, "completed_at": to})
	fx.Insert(t, "task_usage", testutil.Cols{"task_id": completed, "provider": "fixture", "model": "fixture", "input_tokens": 10, "output_tokens": 2, "cache_read_tokens": 0, "cache_write_tokens": 0})
	fx.Insert(t, "task_usage", testutil.Cols{"task_id": failed, "provider": "fixture", "model": "fixture", "input_tokens": 20, "output_tokens": 4, "cache_read_tokens": 0, "cache_write_tokens": 0, "cost_usd_ticks": 1})
	row, err := testHandler.Queries.GetAdminOverviewExecutions(t.Context(), db.GetAdminOverviewExecutionsParams{OrganizationID: org, TimeFrom: adminTimestamp(from), TimeTo: adminTimestamp(to), AsOf: adminTimestamp(time.Now()), WorkspaceID: parseUUID(ws)})
	if err != nil {
		t.Fatal(err)
	}
	if row.Completed != 1 || row.Failed != 1 || row.Cancelled != 1 || row.QueueSamples != 1 || row.QueueLowerBoundSamples != 1 || row.RunSamples != 2 || row.MissingTasks != 1 || row.UnpricedTasks != 1 {
		t.Fatalf("wrong population/samples: %+v", row)
	}
	executions, usage, quality := adminOverviewExecutionMetrics(row)
	if rate := executions["success_rate"].(*float64); rate == nil || *rate != 0.5 {
		t.Fatal("cancelled polluted success denominator")
	}
	if quality != "partial" || usage["quality"] != "partial" || usage["billing"] != "tokens_only" {
		t.Fatal("unknown samples or costs were misrepresented")
	}
	if row.QueueP50 != 600 || row.QueueP95 != 600 {
		t.Fatal("observation clock entered exact percentile")
	}
	overviewPath := "/api/admin/overview?time_from=" + url.QueryEscape(from.Format(time.RFC3339Nano)) + "&time_to=" + url.QueryEscape(to.Format(time.RFC3339Nano)) + "&timezone=America%2FNew_York"
	var overview struct {
		Executions map[string]any    `json:"executions"`
		Usage      map[string]any    `json:"usage"`
		Window     map[string]string `json:"window"`
	}
	passwordCall(t, "GET", overviewPath, nil, login.Token, testHandler.AdminOverview).Want(200).JSON(&overview)
	denominator := overview.Executions["completed"].(float64) + overview.Executions["failed"].(float64)
	if overview.Executions["success_rate"] != overview.Executions["completed"].(float64)/denominator || overview.Usage["billing"] != "tokens_only" || overview.Window["timezone"] != "America/New_York" || overview.Window["time_to"] != to.Format(time.RFC3339Nano) {
		t.Fatal("overview DTO changed denominator/window/cost semantics")
	}

	router := chi.NewRouter()
	router.Use(middleware.Auth(testHandler.Queries, nil, nil))
	router.Get("/api/admin/tasks", testHandler.AdminTasks)
	call := func(query string) *testutil.Response {
		request := testutil.JSONRequest("GET", "/api/admin/tasks?"+query, nil)
		request.Header.Set("Authorization", "Bearer "+login.Token)
		return testutil.Call(t, router.ServeHTTP, request)
	}
	values := url.Values{"workspace_id": {ws}, "time_from": {from.Format(time.RFC3339Nano)}, "time_to": {to.Format(time.RFC3339Nano)}, "timezone": {"America/New_York"}, "time_basis": {"finished"}, "limit": {"1"}}
	ids := map[string]bool{}
	for {
		var page struct {
			Items  []map[string]any `json:"items"`
			Cursor *string          `json:"next_cursor"`
			Basis  string           `json:"time_basis"`
		}
		call(values.Encode()).Want(200).JSON(&page)
		if page.Basis != "finished" {
			t.Fatal("lost window basis")
		}
		for _, item := range page.Items {
			id := item["id"].(string)
			if ids[id] {
				t.Fatal("duplicate finished cursor row")
			}
			ids[id] = true
		}
		if page.Cursor == nil {
			break
		}
		values.Set("cursor", *page.Cursor)
	}
	if len(ids) != 3 || !ids[completed] || !ids[failed] || !ids[cancelled] {
		t.Fatalf("finished drilldown differs: %v", ids)
	}
	values.Del("cursor")
	values.Set("time_basis", "created")
	values.Set("limit", "100")
	var created struct {
		Items []map[string]any `json:"items"`
	}
	call(values.Encode()).Want(200).JSON(&created)
	for _, item := range created.Items {
		if item["id"] == completed {
			t.Fatal("created default replaced by completion time")
		}
	}
}

func TestAdminOverviewMissingSamplesStayNull(t *testing.T) {
	executions, usage, _ := adminOverviewExecutionMetrics(db.GetAdminOverviewExecutionsRow{QueueP50: -1, QueueP95: -1, RunP50: -1, RunP95: -1})
	raw, _ := json.Marshal(map[string]any{"executions": executions, "usage": usage})
	var value map[string]map[string]any
	_ = json.Unmarshal(raw, &value)
	if value["executions"]["success_rate"] != nil || value["usage"]["total_tokens"] != nil || value["usage"]["quality"] != "unknown" {
		t.Fatal("no denominator/usage became zero")
	}
	if value["executions"]["queue_seconds"].(map[string]any)["p50"] != nil {
		t.Fatal("missing latency became zero")
	}
}

func TestAdminAuditProjectsOnlyHistoricalAllowedSnapshots(t *testing.T) {
	login := adminHandlerSetup(t)
	org, err := testHandler.Queries.GetInternalOrganization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	old := dbfx.Insert(t, "admin_audit_event", testutil.Cols{"organization_id": org.ID, "actor_kind": "user", "actor_user_id": login.User.ID, "target_kind": "task", "target_id": uuid.NewString(), "action": "test.snapshot", "phase": "applied", "request_id": uuid.NewString(), "reason": "fixture reason", "result_code": "applied", "before_state": testutil.Raw(`'{"role":"super_admin","auth_version":9007199254740993,"password":"PRIVATE","env":{"secret":"PRIVATE"},"work_dir":"PRIVATE"}'::jsonb`), "after_state": testutil.Raw(`'{}'::jsonb`)})
	captured := dbfx.Insert(t, "admin_audit_event", testutil.Cols{"organization_id": org.ID, "actor_kind": "user", "actor_user_id": login.User.ID, "actor_display_name": "Historic Person", "target_kind": "task", "target_id": uuid.NewString(), "action": "test.snapshot", "phase": "applied", "request_id": uuid.NewString(), "reason": "fixture reason", "result_code": "applied"})
	dbfx.Exec(t, `UPDATE "user" SET name='Current Person' WHERE id=$1`, login.User.ID)
	var page struct {
		Items []map[string]any `json:"items"`
	}
	response := passwordCall(t, "GET", "/api/admin/audit?action=test.snapshot&actor_user_id="+login.User.ID, nil, login.Token, testHandler.AdminAudit).Want(200)
	response.JSON(&page)
	raw, _ := json.Marshal(page)
	if strings.Contains(string(raw), "PRIVATE") || strings.Contains(string(raw), "Current Person") {
		t.Fatal("private/current identity leaked into history")
	}
	seen := map[string]map[string]any{}
	for _, item := range page.Items {
		seen[item["id"].(string)] = item
	}
	if seen[old]["actor_display_name"] != nil || seen[old]["actor_snapshot_quality"] != "unknown" || seen[captured]["actor_display_name"] != "Historic Person" {
		t.Fatal("audit snapshot provenance lost")
	}
	if seen[old]["before_state"].(map[string]any)["auth_version"] != "9007199254740993" {
		t.Fatal("historical version rounded through float")
	}
}

type adminFailingPinger struct{ txStarter }

func (adminFailingPinger) Ping(context.Context) error {
	return errors.New("postgres://SECRET connection failure")
}
func TestAdminHealthAndSettingsDoNotInventAvailabilityOrRetention(t *testing.T) {
	login := adminHandlerSetup(t)
	originalStarter, originalHealth, originalConfig := testHandler.TxStarter, testHandler.AdminHealthSnapshot, testHandler.AdminReadConfig
	t.Cleanup(func() {
		testHandler.TxStarter = originalStarter
		testHandler.AdminHealthSnapshot = originalHealth
		testHandler.AdminReadConfig = originalConfig
	})
	testHandler.TxStarter = adminFailingPinger{txStarter: originalStarter}
	testHandler.AdminHealthSnapshot = func(context.Context) AdminHealthSnapshot {
		now := time.Now().UTC()
		return AdminHealthSnapshot{Sources: []AdminHealthSource{{Name: "alert_detector", State: "healthy", Code: "postgres://SECRET"}, {Name: "liveness", State: "unknown", Code: "database_fallback", CheckedAt: &now}}}
	}
	var health struct {
		Sources  []AdminHealthSource `json:"sources"`
		Detector string              `json:"detector_state"`
	}
	passwordCall(t, "GET", "/api/admin/health", nil, login.Token, testHandler.AdminHealth).Want(200).JSON(&health)
	raw, _ := json.Marshal(health)
	if strings.Contains(string(raw), "SECRET") || health.Detector != "unknown" {
		t.Fatal("health invented freshness or leaked diagnostic")
	}
	databaseState := ""
	for _, source := range health.Sources {
		if source.Name == "database" {
			databaseState = source.State
		}
	}
	if databaseState != "unavailable" {
		t.Fatal("failed database was reported healthy")
	}
	for _, source := range health.Sources {
		if source.Name == "liveness" && source.State != "unavailable" {
			t.Fatal("database-backed liveness did not inherit failed DB probe")
		}
	}
	testHandler.AdminReadConfig = AdminReadConfig{Configured: true, AuthMode: "password", RegistrationEnabled: true, RegistrationPolicy: "self_service"}
	var settings map[string]any
	passwordCall(t, "GET", "/api/admin/settings", nil, login.Token, testHandler.AdminSettings).Want(200).JSON(&settings)
	retention := settings["retention"].(map[string]any)
	if retention["audit_days"] != nil || retention["automatic_deletion_enabled"] != false || retention["policy_source"] != "unknown" {
		t.Fatal("unset policy invented deletion/retention")
	}
	ordinary := passwordRegister(t, fmt.Sprintf("s06ordinary%d", time.Now().UnixNano()))
	passwordCall(t, "GET", "/api/admin/settings", nil, ordinary.Token, testHandler.AdminSettings).Want(http.StatusForbidden)
}

func TestAdminCurrentTasksIncludeOldActiveAndRejectHistoricalScope(t *testing.T) {
	login := adminHandlerSetup(t)
	ws := dbfx.Workspace(t, "Old queue fixture", "old-queue-"+uuid.NewString())
	if _, err := testHandler.Queries.AssignWorkspaceOrganization(t.Context(), parseUUID(ws)); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, "DELETE FROM organization_workspace WHERE workspace_id=$1", ws)
	fx := testutil.New(testPool, ws, login.User.ID)
	runtimeID := fx.Runtime(t, "fixture")
	agentID := fx.Agent(t, "fixture", runtimeID)
	old := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "status": "queued", "created_at": time.Now().Add(-90 * 24 * time.Hour)})
	path := "/api/admin/tasks?workspace_id=" + ws + "&status=queued&state_scope=current&time_basis=created"
	var page struct {
		Items []map[string]any `json:"items"`
	}
	passwordCall(t, "GET", path, nil, login.Token, testHandler.AdminTasks).Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0]["id"] != old {
		t.Fatal("current queued drilldown omitted old active work")
	}
	for _, change := range []string{"time_from", "time_to", "status", "time_basis"} {
		parsed, _ := url.Parse(path)
		values := parsed.Query()
		switch change {
		case "time_from":
			values.Set(change, "")
		case "time_to":
			values.Set(change, "2026-10-01T00:00:00Z")
		case "status":
			values.Set(change, "completed")
		case "time_basis":
			values.Set(change, "finished")
		}
		passwordCall(t, "GET", parsed.Path+"?"+values.Encode(), nil, login.Token, testHandler.AdminTasks).Want(400)
	}
}

func TestAdminWorkspaceCatalogExposesOnlyMetadataAndWindowedExecutionCounts(t *testing.T) {
	login := adminHandlerSetup(t)
	slug := "s06workspace-" + uuid.NewString()
	ws := dbfx.Workspace(t, "Visible workspace name", slug)
	if _, err := testHandler.Queries.AssignWorkspaceOrganization(t.Context(), parseUUID(ws)); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, "DELETE FROM organization_workspace WHERE workspace_id=$1", ws)
	fx := testutil.New(testPool, ws, login.User.ID)
	runtimeID := fx.Runtime(t, "PRIVATE runtime")
	agentID := fx.Agent(t, "PRIVATE agent", runtimeID)
	issue := fx.Issue(t, "PRIVATE issue", testutil.Cols{"description": "PRIVATE content"})
	fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issue, "status": "queued", "context": testutil.Raw(`'{"prompt":"PRIVATE prompt"}'::jsonb`)})
	fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "status": "completed", "created_at": time.Now().Add(-60 * 24 * time.Hour)})
	dbfx.Exec(t, "UPDATE platform_role_binding SET role='platform_observer' WHERE user_id=$1", login.User.ID)
	var page struct {
		Items []map[string]any `json:"items"`
	}
	passwordCall(t, "GET", "/api/admin/workspaces?q="+url.QueryEscape(slug), nil, login.Token, testHandler.AdminWorkspaces).Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0]["id"] != ws || page.Items[0]["execution_count"] != float64(1) {
		t.Fatal("workspace catalog/count window mismatch")
	}
	raw, _ := json.Marshal(page)
	if strings.Contains(string(raw), "PRIVATE") || strings.Contains(string(raw), "config") {
		t.Fatal("workspace metadata leaked private content")
	}
}

func TestAdminAuditRetainsAllowedAlertDisposition(t *testing.T) {
	value := adminAuditSnapshot([]byte(`{"resolution_code":"handled","related_task_id":"11111111-1111-4111-8111-111111111111","error":"PRIVATE diagnostic"}`))
	if value["resolution_code"] != "handled" || value["related_task_id"] != "11111111-1111-4111-8111-111111111111" || value["error"] != nil {
		t.Fatal("audit lost safe disposition or retained diagnostics")
	}
}

func TestAdminOverviewInactiveAlertsMatchFirstSeenWindow(t *testing.T) {
	login := adminHandlerSetup(t)
	t.Setenv("MULTICA_DEPLOYMENT_ID", uuid.NewString())
	org, err := testHandler.Queries.GetInternalOrganization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	query := "?time_from=" + url.QueryEscape(from.Format(time.RFC3339Nano)) + "&time_to=" + url.QueryEscape(now.Format(time.RFC3339Nano)) + "&timezone=UTC"
	read := func() map[string]float64 {
		var response struct {
			Alerts map[string]float64 `json:"alerts"`
		}
		passwordCall(t, "GET", "/api/admin/overview"+query, nil, login.Token, testHandler.AdminOverview).Want(200).JSON(&response)
		return response.Alerts
	}
	before := read()
	subject := uuid.NewString()
	add := func(status string, seen time.Time) string {
		return dbfx.Insert(t, "admin_alert", testutil.Cols{"organization_id": org.ID, "rule": "queue_timeout", "subject_kind": "task", "subject_id": subject, "fingerprint": uuid.NewString(), "severity": "warning", "status": status, "condition_active": false, "first_seen_at": seen, "last_seen_at": seen, "last_observed_at": seen})
	}
	old := add("resolved", now.Add(-60*24*time.Hour))
	current := add("resolved", from.Add(time.Hour))
	add("closed", now.Add(-60*24*time.Hour))
	add("open", now.Add(-60*24*time.Hour))
	add("acknowledged", now.Add(-60*24*time.Hour))
	after := read()
	if after["resolved"]-before["resolved"] != 1 || after["closed"] != before["closed"] {
		t.Fatal("inactive overview count included old first-observed alerts")
	}
	if after["open"]-before["open"] != 1 || after["acknowledged"]-before["acknowledged"] != 1 {
		t.Fatal("active alerts were incorrectly bounded by history window")
	}
	var page struct {
		Items []map[string]any `json:"items"`
	}
	passwordCall(t, "GET", "/api/admin/alerts"+query+"&status=resolved&q="+subject, nil, login.Token, testHandler.AdminAlerts).Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0]["id"] != current || page.Items[0]["id"] == old {
		t.Fatal("inactive overview/list populations differ")
	}
}

func TestAdminAuditUnknownDecisionValuesDoNotLeakSlugDiagnostics(t *testing.T) {
	snapshot := adminAuditSnapshot([]byte(`{"resolution_code":"private-token-shaped-text","state":"private-token-shaped-text","status":"private-token-shaped-text","admission":"private-token-shaped-text"}`))
	for _, key := range []string{"resolution_code", "state", "status", "admission"} {
		if snapshot[key] != "unknown" {
			t.Fatalf("uncontrolled %s escaped historical projection", key)
		}
	}
}
func TestAdminOverviewUnknownCurrentStateMarksPartial(t *testing.T) {
	_, _, quality := adminOverviewExecutionMetrics(db.GetAdminOverviewExecutionsRow{Unfinished: 1, QueueP50: -1, QueueP95: -1, RunP50: -1, RunP95: -1})
	if quality != "partial" {
		t.Fatal("unknown current state was labelled complete")
	}
}
