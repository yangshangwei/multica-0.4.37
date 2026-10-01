package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type managedDaemonFixture struct {
	h                                                      *Handler
	owner, workspace, daemon, binding, runtime, token, pat string
}

func managedDaemonSetup(t *testing.T) managedDaemonFixture {
	t.Helper()
	passwordTestSetup(t)
	deployment := "00000000-0000-4000-8000-000000000009"
	t.Setenv("MULTICA_DEPLOYMENT_ID", deployment)
	user := passwordRegister(t, fmt.Sprintf("managed%d", time.Now().UnixNano()))
	org, err := testHandler.Queries.EnsureInternalOrganization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ws := dbfx.Workspace(t, "Managed daemon workspace", "managed-"+uuid.NewString())
	dbfx.Member(t, ws, user.User.ID, "owner")
	dbfx.InsertNoID(t, "organization_workspace", testutil.Cols{"organization_id": org.ID, "workspace_id": ws}, "workspace_id=$1", ws)
	inst := dbfx.Insert(t, "managed_installation", testutil.Cols{"deployment_id": deployment, "organization_id": org.ID, "public_key": make([]byte, 32), "key_fingerprint": auth.HashToken(uuid.NewString()), "responsible_user_id": user.User.ID})
	daemon := uuid.NewString()
	binding := dbfx.Insert(t, "installation_daemon_binding", testutil.Cols{"installation_id": inst, "workspace_id": ws, "daemon_id": daemon, "principal_user_id": user.User.ID, "auth_version": int64(1), "binding_epoch": int64(1)})
	token, err := auth.GenerateDaemonToken()
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Insert(t, "daemon_token", testutil.Cols{"token_hash": auth.HashToken(token), "workspace_id": ws, "daemon_id": daemon, "user_id": user.User.ID, "auth_version": int64(1), "installation_binding_id": binding, "installation_binding_epoch": int64(1), "expires_at": time.Now().Add(time.Hour)})
	pat, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Insert(t, "personal_access_token", testutil.Cols{"user_id": user.User.ID, "name": "managed fallback test", "token_hash": auth.HashToken(pat), "token_prefix": pat[:12], "auth_version": int64(1), "expires_at": time.Now().Add(time.Hour)})
	runtime := dbfx.Runtime(t, "Managed runtime", testutil.Cols{"workspace_id": ws, "owner_id": user.User.ID, "daemon_id": daemon, "runtime_mode": "local"})
	h := *testHandler
	h.DaemonHub = daemonws.NewHub()
	h.DaemonHub.SetPasswordQueries(h.Queries)
	h.DaemonHub.SetRuntimeAuthorizer(h.AuthorizeDaemonConnection)
	h.DaemonHub.SetHeartbeatHandler(h.HandleDaemonWSHeartbeat)
	h.DaemonHub.SetRPCHandler(h.DaemonRPCHandler)
	return managedDaemonFixture{&h, user.User.ID, ws, daemon, binding, runtime, token, pat}
}
func (f managedDaemonFixture) call(t *testing.T, token, path string, body any, handler http.HandlerFunc) *testutil.Response {
	t.Helper()
	req := testutil.JSONRequest(http.MethodPost, path, body)
	req.Header.Set("Authorization", "Bearer "+token)
	return testutil.Call(t, middleware.DaemonAuth(f.h.Queries, nil, nil, nil)(handler).ServeHTTP, req)
}
func TestManagedRuntimeHTTPRejectsPATAndOwnerMismatch(t *testing.T) {
	f := managedDaemonSetup(t)
	for _, token := range []string{f.pat, f.token} {
		status := 403
		if token == f.token {
			status = 200
		}
		f.call(t, token, "/api/daemon/heartbeat", map[string]any{"runtime_id": f.runtime}, f.h.DaemonHeartbeat).Want(status)
	}
	other := passwordRegister(t, fmt.Sprintf("managedother%d", time.Now().UnixNano()))
	dbfx.Exec(t, "UPDATE agent_runtime SET owner_id=$2 WHERE id=$1", f.runtime, other.User.ID)
	f.call(t, f.token, "/api/daemon/heartbeat", map[string]any{"runtime_id": f.runtime}, f.h.DaemonHeartbeat).Want(403)
}
func TestManagedRuntimeRegistrationPinsPrincipalAndNamespace(t *testing.T) {
	f := managedDaemonSetup(t)
	body := map[string]any{"workspace_id": f.workspace, "daemon_id": f.daemon, "runtimes": []map[string]any{{"type": "codex", "name": "Managed codex", "version": "fixture"}}}
	f.call(t, f.pat, "/api/daemon/register", body, f.h.DaemonRegister).Want(403)
	f.call(t, f.token, "/api/daemon/register", body, f.h.DaemonRegister).Want(200)
	dbfx.Cleanup(t, "DELETE FROM agent_runtime WHERE workspace_id=$1", f.workspace)
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_runtime WHERE workspace_id=$1 AND owner_id IS DISTINCT FROM $2", f.workspace, f.owner); n != 0 {
		t.Fatal("managed registration lost principal ownership")
	}
	body["daemon_id"] = uuid.NewString()
	f.call(t, f.token, "/api/daemon/register", body, f.h.DaemonRegister).Want(403)
}

func TestManagedRuntimeRevokedHistoryHTTPDeniesLegacyFallback(t *testing.T) {
	f := managedDaemonSetup(t)
	dbfx.Exec(t, "UPDATE installation_daemon_binding SET state='revoked' WHERE id=$1", f.binding)
	body := map[string]any{"workspace_id": f.workspace, "daemon_id": f.daemon, "runtimes": []map[string]any{{"type": "codex", "name": "Revoked namespace"}}}
	f.call(t, f.pat, "/api/daemon/register", body, f.h.DaemonRegister).Want(403)
	f.call(t, f.pat, "/api/daemon/heartbeat", map[string]any{"runtime_id": f.runtime}, f.h.DaemonHeartbeat).Want(403)
	router := chi.NewRouter()
	router.Post("/api/daemon/runtimes/{runtimeId}/tasks/claim", f.h.ClaimTaskByRuntime)
	f.call(t, f.pat, "/api/daemon/runtimes/"+f.runtime+"/tasks/claim", nil, router.ServeHTTP).Want(403)
}
func TestManagedRuntimeWorkspaceListingStaysTokenScoped(t *testing.T) {
	f := managedDaemonSetup(t)
	other := dbfx.Workspace(t, "Other membership", "managed-other-"+uuid.NewString())
	dbfx.Member(t, other, f.owner, "member")
	req := testutil.JSONRequest(http.MethodGet, "/api/daemon/workspaces", nil)
	req.Header.Set("Authorization", "Bearer "+f.token)
	var rows []DaemonWorkspaceResponse
	testutil.Call(t, middleware.DaemonAuth(f.h.Queries, nil, nil, nil)(http.HandlerFunc(f.h.ListDaemonWorkspaces)).ServeHTTP, req).Want(200).JSON(&rows)
	if len(rows) != 1 || rows[0].ID != f.workspace {
		t.Fatalf("bound token listed other workspaces: %+v", rows)
	}
}
func TestManagedRuntimeWSRejectsCrossScopeAndRechecksOutgoingOwner(t *testing.T) {
	f := managedDaemonSetup(t)
	otherWS := dbfx.Workspace(t, "Other workspace", "managed-ws-"+uuid.NewString())
	dbfx.Member(t, otherWS, f.owner, "owner")
	foreign := dbfx.Runtime(t, "Other scope runtime", testutil.Cols{"workspace_id": otherWS, "owner_id": f.owner, "daemon_id": uuid.NewString()})
	server := httptest.NewServer(middleware.DaemonAuth(f.h.Queries, nil, nil, nil)(http.HandlerFunc(f.h.DaemonWebSocket)))
	defer server.Close()
	headers := http.Header{"Authorization": {"Bearer " + f.token}}
	// Preserve the daemon API's existing invisible-workspace 404 boundary.
	_, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?runtime_id="+foreign, headers)
	if err == nil || response == nil || response.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-workspace upgrade=%v, %v", response, err)
	}
	if response != nil {
		response.Body.Close()
	}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?runtime_id="+f.runtime, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err = conn.WriteJSON(map[string]any{"type": "daemon:heartbeat", "payload": map[string]any{"runtime_id": f.runtime}}); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err = conn.ReadMessage(); err != nil {
		t.Fatal("authorized heartbeat did not establish the connection", err)
	}
	// The preceding acknowledgement proves the connection was registered and authorized.
	other := passwordRegister(t, fmt.Sprintf("newowner%d", time.Now().UnixNano()))
	dbfx.Exec(t, "UPDATE agent_runtime SET owner_id=$2 WHERE id=$1", f.runtime, other.User.ID)
	f.h.DaemonHub.NotifyTaskAvailable(f.runtime, uuid.NewString())
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("outgoing event crossed runtime owner change")
	}
	if timeout, ok := err.(interface{ Timeout() bool }); ok && timeout.Timeout() {
		t.Fatal("unauthorized websocket remained open")
	}
}
func TestManagedRuntimeRPCPreservesBoundSourceAndPinnedRuntimeSet(t *testing.T) {
	f := managedDaemonSetup(t)
	source, err := auth.CheckPasswordToken(t.Context(), f.h.Queries, f.token, true)
	if err != nil {
		t.Fatal(err)
	}
	identity := daemonws.ClientIdentity{UserID: f.owner, DaemonID: f.daemon, WorkspaceID: f.workspace, WorkspaceIDs: []string{f.workspace}, RuntimeIDs: []string{f.runtime}}
	ctx := auth.WithPasswordSession(context.Background(), source.Session)
	status, _, _ := f.h.DaemonRPCHandler(ctx, identity, "tasks.claim", []byte(fmt.Sprintf(`{"daemon_id":%q,"runtime_ids":[%q],"max_tasks":1}`, f.daemon, uuid.NewString())))
	if status != 403 {
		t.Fatalf("RPC escaped pinned runtimes: %d", status)
	}
	dbfx.Exec(t, "UPDATE installation_daemon_binding SET state='revoked' WHERE id=$1", f.binding)
	status, _, _ = f.h.DaemonRPCHandler(ctx, identity, "tasks.claim", []byte(fmt.Sprintf(`{"daemon_id":%q,"runtime_ids":[%q],"max_tasks":1}`, f.daemon, f.runtime)))
	if status != 401 {
		t.Fatalf("revoked RPC source=%d", status)
	}
}

func TestLegacyRegistrationCannotLaunderForeignOrUnknownRuntimeOwnership(t *testing.T) {
	f := managedDaemonSetup(t)
	// A legacy namespace has never been managed; revoking a binding does not
	// restore legacy authorization.
	f.daemon = uuid.NewString()
	dbfx.Exec(t, "UPDATE agent_runtime SET daemon_id=$2 WHERE id=$1", f.runtime, f.daemon)
	other := passwordRegister(t, fmt.Sprintf("legacyowner%d", time.Now().UnixNano()))
	dbfx.Member(t, f.workspace, other.User.ID, "member")
	otherPAT, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Insert(t, "personal_access_token", testutil.Cols{"user_id": other.User.ID, "name": "legacy ownership attempt", "token_hash": auth.HashToken(otherPAT), "token_prefix": otherPAT[:12], "auth_version": int64(1), "expires_at": time.Now().Add(time.Hour)})
	body := map[string]any{"workspace_id": f.workspace, "daemon_id": f.daemon, "runtimes": []map[string]any{{"type": "handler_test_runtime", "name": "Ownership attempt"}}}
	f.call(t, otherPAT, "/api/daemon/register", body, f.h.DaemonRegister).Want(403)
	var owner string
	dbfx.QueryRow(t, "SELECT owner_id::text FROM agent_runtime WHERE id=$1", f.runtime).Scan(&owner)
	if owner != f.owner {
		t.Fatal("PAT changed a known owner before binding")
	}
	// Choosing a fresh namespace and claiming the victim as a legacy hostname
	// must not bypass the owner check by merging away the original runtime.
	newDaemon := uuid.NewString()
	body["daemon_id"] = newDaemon
	body["legacy_daemon_ids"] = []string{f.daemon}
	f.call(t, otherPAT, "/api/daemon/register", body, f.h.DaemonRegister).Want(200)
	dbfx.Cleanup(t, "DELETE FROM agent_runtime WHERE workspace_id=$1 AND daemon_id=$2", f.workspace, newDaemon)
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_runtime WHERE id=$1 AND owner_id=$2", f.runtime, f.owner); n != 1 {
		t.Fatal("legacy merge laundered another owner's runtime")
	}
	// No registration payload may turn unknown historical ownership into proof.
	dbfx.Exec(t, "UPDATE agent_runtime SET owner_id=NULL WHERE id=$1", f.runtime)
	body["daemon_id"] = f.daemon
	delete(body, "legacy_daemon_ids")
	f.call(t, f.pat, "/api/daemon/register", body, f.h.DaemonRegister).Want(200)
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_runtime WHERE id=$1 AND owner_id IS NULL", f.runtime); n != 1 {
		t.Fatal("legacy NULL owner became an asserted caller's ownership")
	}
}

func TestLegacyCustomProfileRegistrationPreservesOwnership(t *testing.T) {
	f := managedDaemonSetup(t)
	f.daemon = uuid.NewString()
	dbfx.Exec(t, "UPDATE agent_runtime SET daemon_id=$2 WHERE id=$1", f.runtime, f.daemon)
	other := passwordRegister(t, fmt.Sprintf("profileowner%d", time.Now().UnixNano()))
	dbfx.Member(t, f.workspace, other.User.ID, "member")
	pat, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Insert(t, "personal_access_token", testutil.Cols{"user_id": other.User.ID, "name": "profile ownership test", "token_hash": auth.HashToken(pat), "token_prefix": pat[:12], "auth_version": int64(1), "expires_at": time.Now().Add(time.Hour)})
	profile := dbfx.Insert(t, "runtime_profile", testutil.Cols{"workspace_id": f.workspace, "display_name": "Owned profile", "protocol_family": "codex", "command_name": "fixture-runtime", "created_by": f.owner})
	runtime := dbfx.Runtime(t, "Owned profile runtime", testutil.Cols{"workspace_id": f.workspace, "owner_id": f.owner, "daemon_id": f.daemon, "profile_id": profile, "provider": "codex"})
	body := map[string]any{"workspace_id": f.workspace, "daemon_id": f.daemon, "runtimes": []map[string]any{{"profile_id": profile, "type": "codex"}}}
	f.call(t, pat, "/api/daemon/register", body, f.h.DaemonRegister).Want(403)
	failed := map[string]any{"workspace_id": f.workspace, "daemon_id": f.daemon, "failed_profiles": []map[string]any{{"profile_id": profile, "reason": "Fixture runtime unavailable"}}}
	f.call(t, pat, "/api/daemon/register", failed, f.h.DaemonRegister).Want(403)
	dbfx.Exec(t, "UPDATE agent_runtime SET owner_id=NULL WHERE id=$1", runtime)
	f.call(t, f.pat, "/api/daemon/register", body, f.h.DaemonRegister).Want(200)
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_runtime WHERE id=$1 AND owner_id IS NULL", runtime); n != 1 {
		t.Fatal("custom profile upsert assigned an unknown owner")
	}
}
