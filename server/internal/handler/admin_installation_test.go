package handler

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestAdminInstallationStateAxesRemainIndependent(t *testing.T) {
	now := time.Now().UTC()
	fresh := pgtype.Timestamptz{Time: now.Add(-time.Second), Valid: true}
	stale := pgtype.Timestamptz{Time: now.Add(-10 * time.Minute), Valid: true}
	rt := adminInstallationRuntimeEvidence{RuntimeID: "runtime", BindingID: "binding", Authorized: true, Capability: "1", Status: "online", LastSeen: fresh, BindingSeen: fresh}
	client, daemon, ready := installationAxes(now, "active", "accepting", stale, []adminInstallationRuntimeEvidence{rt}, map[string]bool{}, false, false)
	if client.State != "inactive" || daemon.State != "reachable" || ready.State != "ready" {
		t.Fatalf("closed app/live daemon: %+v %+v %+v", client, daemon, ready)
	}
	rt.Status = "offline"
	rt.OfflineCode = "not_executable"
	client, daemon, ready = installationAxes(now, "active", "accepting", fresh, []adminInstallationRuntimeEvidence{rt}, nil, false, false)
	if client.State != "active" || daemon.State != "reachable" || ready.State != "environment_unavailable" {
		t.Fatalf("active app/broken engine: %+v %+v %+v", client, daemon, ready)
	}
	_, daemon, ready = installationAxes(now, "active", "accepting", fresh, []adminInstallationRuntimeEvidence{rt}, nil, true, false)
	if daemon.State != "unavailable" || ready.State != "unavailable" {
		t.Fatal("liveness error became offline or ready")
	}
	_, _, ready = installationAxes(now, "active", "stopped", fresh, []adminInstallationRuntimeEvidence{rt}, nil, false, false)
	if ready.State != "stopped" {
		t.Fatal("stopped admission not represented")
	}
	rt.Status = "online"
	rt.LastSeen = stale
	rt.BindingSeen = stale
	_, daemon, ready = installationAxes(now, "active", "accepting", fresh, []adminInstallationRuntimeEvidence{rt}, map[string]bool{"runtime": true}, true, true)
	if daemon.State != "reachable" || ready.State == "ready" {
		t.Fatal("cache liveness bypassed DB claim freshness")
	}
	rt.Authorized = false
	_, _, ready = installationAxes(now, "active", "accepting", fresh, []adminInstallationRuntimeEvidence{rt}, nil, false, false)
	if ready.State != "no_permission" {
		t.Fatal("invalid credential reported ready")
	}
	client, daemon, ready = installationAxes(now, "active", "accepting", pgtype.Timestamptz{}, nil, nil, false, false)
	if client.State != "unknown" || daemon.State != "unknown" || ready.State != "unknown" {
		t.Fatal("missing evidence invented a state")
	}
	rt.Authorized = true
	rt.Status = "offline"
	rt.BindingSeen = pgtype.Timestamptz{}
	rt.LastSeen = stale
	_, _, ready = installationAxes(now, "active", "accepting", fresh, []adminInstallationRuntimeEvidence{rt}, nil, false, false)
	if ready.Freshness != "stale" {
		t.Fatal("old runtime observation was marked fresh")
	}
	rt.Status = "online"
	rt.LastSeen = fresh
	_, daemon, _ = installationAxes(now, "active", "accepting", fresh, []adminInstallationRuntimeEvidence{rt}, nil, false, false)
	if daemon.Source != "runtime_heartbeat" {
		t.Fatal("runtime evidence mislabeled as a binding heartbeat")
	}
}

type failingAdminLiveness struct{}

func (failingAdminLiveness) Available() bool                                    { return true }
func (failingAdminLiveness) Touch(context.Context, string, time.Duration) error { return nil }
func (failingAdminLiveness) Forget(context.Context, string)                     {}
func (failingAdminLiveness) IsAliveBatch(context.Context, []string) (map[string]bool, bool) {
	return nil, false
}

func TestAdminInstallationReadModelDeduplicatesAndRedacts(t *testing.T) {
	login := adminHandlerSetup(t)
	deployment := uuid.NewString()
	t.Setenv("MULTICA_DEPLOYMENT_ID", deployment)
	org, err := testHandler.Queries.GetInternalOrganization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ws := dbfx.Workspace(t, "Installation workspace", "inst-"+uuid.NewString())
	if _, err := testHandler.Queries.AssignWorkspaceOrganization(t.Context(), parseUUID(ws)); err != nil {
		t.Fatal(err)
	}
	dbfx.Cleanup(t, "DELETE FROM organization_workspace WHERE workspace_id=$1", ws)
	fx := testutil.New(testPool, ws, login.User.ID)
	fx.Member(t, ws, login.User.ID, "member")
	inst := fx.Insert(t, "managed_installation", testutil.Cols{"deployment_id": deployment, "organization_id": uuidToString(org.ID), "public_key": []byte(strings.Repeat("k", 32)), "key_fingerprint": strings.Repeat("a", 64), "responsible_user_id": login.User.ID, "display_name": "Managed test", "client_seen_at": time.Now().Add(-10 * time.Minute)})
	daemonID := uuid.NewString()
	binding := fx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": inst, "workspace_id": ws, "daemon_id": daemonID, "principal_user_id": login.User.ID, "auth_version": 1, "binding_epoch": 1, "last_seen_at": time.Now()})
	fx.Insert(t, "daemon_token", testutil.Cols{"token_hash": "s02-read-" + uuid.NewString(), "workspace_id": ws, "daemon_id": daemonID, "user_id": login.User.ID, "auth_version": 1, "installation_binding_id": binding, "installation_binding_epoch": 1, "expires_at": time.Now().Add(time.Hour)})
	one := fx.Runtime(t, "PRIVATE RUNTIME /home/private", testutil.Cols{"daemon_id": daemonID, "metadata": testutil.Raw(`'{"secret":"PRIVATE METADATA"}'::jsonb`)})
	second := fx.Runtime(t, "PRIVATE SECOND", testutil.Cols{"daemon_id": daemonID, "provider": "claude"})
	waitingAgent := fx.Agent(t, "Waiting agent", second)
	fx.Task(t, waitingAgent, testutil.Cols{"runtime_id": second, "status": "waiting_local_directory"})
	legacy := fx.Runtime(t, "PRIVATE LEGACY", testutil.Cols{"daemon_id": uuid.NewString()})
	var result struct {
		Items []map[string]any `json:"items"`
	}
	response := passwordCall(t, "GET", "/api/admin/installations?q="+inst, nil, login.Token, testHandler.AdminInstallations).Want(200)
	response.JSON(&result)
	if len(result.Items) != 1 || result.Items[0]["runtime_count"] != float64(2) {
		t.Fatalf("installation duplicated or lost runtimes: %+v", result)
	}
	if strings.Contains(response.Body.String(), "PRIVATE") || strings.Contains(response.Body.String(), "public_key") || strings.Contains(response.Body.String(), "token_hash") {
		t.Fatal("private runtime/key material leaked")
	}
	states := result.Items[0]
	if states["client_activity"].(map[string]any)["state"] != "inactive" || states["daemon_reachability"].(map[string]any)["state"] != "reachable" || states["execution_readiness"].(map[string]any)["state"] != "ready" {
		t.Fatalf("incorrect axes: %+v", states)
	}
	passwordCall(t, "GET", "/api/admin/installations/unassociated?workspace_id="+ws, nil, login.Token, testHandler.AdminUnassociatedRuntimes).Want(200).JSON(&result)
	if len(result.Items) != 1 || result.Items[0]["id"] != legacy {
		t.Fatalf("legacy association inferred: %+v", result)
	}
	original := testHandler.LivenessStore
	testHandler.LivenessStore = failingAdminLiveness{}
	t.Cleanup(func() { testHandler.LivenessStore = original })
	passwordCall(t, "GET", "/api/admin/installations?q="+inst, nil, login.Token, testHandler.AdminInstallations).Want(200).JSON(&result)
	if result.Items[0]["daemon_reachability"].(map[string]any)["state"] != "unavailable" {
		t.Fatal("liveness outage was reported offline")
	}
	testHandler.LivenessStore = original
	fx.Exec(t, "UPDATE agent_runtime SET owner_id=NULL WHERE id=$1", one)
	passwordCall(t, "GET", "/api/admin/installations/unassociated?workspace_id="+ws, nil, login.Token, testHandler.AdminUnassociatedRuntimes).Want(200).JSON(&result)
	if len(result.Items) != 2 {
		t.Fatal("owner mismatch merged into verified installation")
	}
	router := chi.NewRouter()
	router.Use(middleware.Auth(testHandler.Queries, nil, nil))
	router.Get("/api/admin/installations/{id}", testHandler.AdminInstallation)
	callDetail := func(id string) *testutil.Response {
		req := testutil.JSONRequest("GET", "/api/admin/installations/"+id, nil)
		req.Header.Set("Authorization", "Bearer "+login.Token)
		return testutil.Call(t, router.ServeHTTP, req)
	}
	var detail map[string]any
	callDetail(inst).Want(200).JSON(&detail)
	if len(detail["bindings"].([]any)) != 1 || len(detail["runtimes"].([]any)) != 1 {
		t.Fatalf("invalid detail association: %v", detail)
	}
	if detail["runtimes"].([]any)[0].(map[string]any)["running_tasks"] != float64(1) {
		t.Fatal("capacity-bearing directory waiter missing from in-flight count")
	}
	fx.Exec(t, "DELETE FROM daemon_token WHERE installation_binding_id=$1", binding)
	callDetail(inst).Want(200).JSON(&detail)
	if detail["installation"].(map[string]any)["execution_readiness"].(map[string]any)["state"] != "no_permission" {
		t.Fatal("expired/revoked binding credential reported ready")
	}
	fx.Exec(t, "UPDATE user_password_credential SET session_version=session_version+1 WHERE user_id=$1", login.User.ID)
	passwordCall(t, "GET", "/api/admin/installations", nil, login.Token, testHandler.AdminInstallations).Want(http.StatusUnauthorized)
}

func TestAdminInstallationCursorAndDeploymentIsolation(t *testing.T) {
	login := adminHandlerSetup(t)
	deployment := uuid.NewString()
	t.Setenv("MULTICA_DEPLOYMENT_ID", deployment)
	org, err := testHandler.Queries.GetInternalOrganization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for n, scope := range []string{deployment, deployment, uuid.NewString()} {
		dbfx.Insert(t, "managed_installation", testutil.Cols{"deployment_id": scope, "organization_id": uuidToString(org.ID), "public_key": []byte(strings.Repeat("x", 32)), "key_fingerprint": strings.Repeat(string(rune('b'+n)), 64), "display_name": "Read cursor fixture", "created_at": time.Now().Add(-time.Duration(n+1) * time.Hour)})
	}
	var page struct {
		Items  []map[string]any `json:"items"`
		Cursor *string          `json:"next_cursor"`
	}
	passwordCall(t, "GET", "/api/admin/installations?limit=1", nil, login.Token, testHandler.AdminInstallations).Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Cursor == nil {
		t.Fatalf("missing page: %+v", page)
	}
	first := page.Items[0]["id"]
	cursor := *page.Cursor
	page.Items = nil
	path := "/api/admin/installations?limit=1&cursor=" + url.QueryEscape(cursor)
	passwordCall(t, "GET", path, nil, login.Token, testHandler.AdminInstallations).Want(200).JSON(&page)
	if len(page.Items) != 1 || page.Items[0]["id"] == first || page.Cursor != nil {
		t.Fatalf("deployment pagination: %+v", page)
	}
	t.Setenv("MULTICA_DEPLOYMENT_ID", uuid.NewString())
	passwordCall(t, "GET", path, nil, login.Token, testHandler.AdminInstallations).Want(400)
	t.Setenv("MULTICA_DEPLOYMENT_ID", "")
	passwordCall(t, "GET", "/api/admin/installations", nil, login.Token, testHandler.AdminInstallations).Want(503)
}
