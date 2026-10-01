package main

import (
	"github.com/gorilla/websocket"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestPlatformAccountRouterDisableRestoreRevokesOnlyTargetAndPreservesMembership(t *testing.T) {
	f := newPlatformAdminRouterFixture(t)
	actor, target := f.user(t, "super_admin"), f.user(t, "")
	actorJWT, targetJWT := platformRouterJWT(t, actor, nil), platformRouterJWT(t, target, nil)
	workspace := f.fx.Workspace(t, "Preserved workspace", "s05-"+uuid.NewString())
	f.fx.Member(t, workspace, target, "member")
	token, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": target, "name": "Target PAT", "token_hash": auth.HashToken(token), "token_prefix": token[:12], "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
	runtime := f.fx.Runtime(t, "Target runtime", testutil.Cols{"workspace_id": workspace, "owner_id": target})
	agent := f.fx.Agent(t, "Target agent", runtime, testutil.Cols{"workspace_id": workspace, "owner_id": target})
	task := f.fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "status": "running"})
	taskToken, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Insert(t, "task_token", testutil.Cols{"user_id": target, "workspace_id": workspace, "agent_id": agent, "task_id": task, "token_hash": auth.HashToken(taskToken), "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
	daemonToken, err := auth.GenerateDaemonToken()
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Insert(t, "daemon_token", testutil.Cols{"user_id": target, "workspace_id": workspace, "daemon_id": uuid.NewString(), "token_hash": auth.HashToken(daemonToken), "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
	if _, err := auth.CheckPasswordToken(t.Context(), db.New(f.pool), daemonToken, true); err != nil {
		t.Fatal("daemon control invalid", err)
	}
	body := map[string]any{"expected_auth_version": 1, "reason": "Review target account access", "password": platformRouterPassword}
	result := f.request(t, actorJWT, false, http.MethodPost, "/api/admin/users/"+target+"/disable", body, nil).Want(200)
	platformAssertNoStore(t, result)
	f.request(t, actorJWT, false, http.MethodGet, "/api/admin/me", nil, nil).Want(200)
	for _, credential := range []string{targetJWT, token, taskToken} {
		f.request(t, credential, false, http.MethodGet, "/api/me", nil, nil).Want(401)
	}
	if _, err := auth.CheckPasswordToken(t.Context(), db.New(f.pool), daemonToken, true); err == nil {
		t.Fatal("daemon credential still valid")
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='cancelled'", task); n != 1 {
		t.Fatal("target execution was deleted or not cancelled")
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM member WHERE user_id=$1", target); n != 1 {
		t.Fatal("disable removed target membership")
	}
	body["expected_auth_version"] = 2
	f.request(t, actorJWT, false, http.MethodPost, "/api/admin/users/"+target+"/restore", body, nil).Want(200)
	for _, credential := range []string{targetJWT, token, taskToken} {
		f.request(t, credential, false, http.MethodGet, "/api/me", nil, nil).Want(401)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_audit_event WHERE actor_user_id=$1 AND target_id=$2", actor, target); n != 2 {
		t.Fatalf("actor-target audit count=%d", n)
	}
}

func TestPlatformAccountRouterDirectoryProjectionAndStableCursor(t *testing.T) {
	f := newPlatformAdminRouterFixture(t)
	actor := f.user(t, "super_admin")
	token := platformRouterJWT(t, actor, nil)
	f.user(t, "")
	f.user(t, "platform_observer")
	first := f.request(t, token, false, http.MethodGet, "/api/admin/users?limit=1", nil, nil).Want(200).Map()
	items, ok := first["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("first page=%v", first)
	}
	row := items[0].(map[string]any)
	for _, secret := range []string{"password_hash", "email", "disabled_reason"} {
		if _, exists := row[secret]; exists {
			t.Fatalf("directory leaked %s", secret)
		}
	}
	cursor, ok := first["next_cursor"].(string)
	if !ok || cursor == "" {
		t.Fatal("first page missing continuation")
	}
	second := f.request(t, token, false, http.MethodGet, "/api/admin/users?limit=1&cursor="+cursor, nil, nil).Want(200).Map()
	secondItems := second["items"].([]any)
	if len(secondItems) != 1 || secondItems[0].(map[string]any)["id"] == row["id"] {
		t.Fatal("cursor duplicated first row")
	}
	f.request(t, token, false, http.MethodGet, "/api/admin/users?limit=1&status=disabled&cursor="+cursor, nil, nil).Want(400)
	id := row["id"].(string)
	detail := f.request(t, token, false, http.MethodGet, "/api/admin/users/"+id, nil, nil).Want(200).Map()
	if detail["memberships"] == nil {
		t.Fatal("empty membership list must encode []")
	}
}

func TestPlatformAccountRouterRecoveryReturnsNoCredentialsAndRequiresPasswordChange(t *testing.T) {
	f := newPlatformAdminRouterFixture(t)
	actor, target := f.user(t, "super_admin"), f.user(t, "")
	var username string
	f.fx.QueryRow(t, "SELECT username FROM user_password_credential WHERE user_id=$1", target).Scan(&username)
	body := map[string]any{"expected_auth_version": 1, "reason": "Lost target password", "password": platformRouterPassword, "temporary_password": "temporary-secret-42"}
	result := f.request(t, platformRouterJWT(t, actor, nil), false, http.MethodPost, "/api/admin/users/"+target+"/recover-password", body, nil).Want(200).Map()
	if _, ok := result["token"]; ok {
		t.Fatal("recovery returned target session")
	}
	response := testutil.Call(t, f.router.ServeHTTP, testutil.JSONRequest(http.MethodPost, "/auth/login", map[string]any{"username": username, "password": "temporary-secret-42"})).Want(200).Map()
	session, ok := response["token"].(string)
	if !ok || session == "" {
		t.Fatalf("temporary password login failed: %v", response)
	}
	f.request(t, session, false, http.MethodGet, "/api/admin/me", nil, nil).Want(403)
	f.request(t, session, false, http.MethodGet, "/api/workspaces", nil, nil).Want(403)
	changed := f.request(t, session, false, http.MethodPost, "/api/me/password/change", map[string]string{"current_password": "temporary-secret-42", "new_password": "personal-password-42"}, nil).Want(200).Map()
	normal, ok := changed["token"].(string)
	if !ok || normal == "" {
		t.Fatal("password change did not return normal login")
	}
	f.request(t, session, false, http.MethodGet, "/api/me", nil, nil).Want(401)
	f.request(t, normal, false, http.MethodGet, "/api/me", nil, nil).Want(200)
	var audit string
	f.fx.QueryRow(t, "SELECT string_agg(to_jsonb(a)::text,'') FROM admin_audit_event a WHERE target_id=$1", target).Scan(&audit)
	if strings.Contains(audit, "temporary-secret") || strings.Contains(audit, platformRouterPassword) || strings.Contains(audit, "pbkdf2") {
		t.Fatal("recovery audit contains a secret")
	}
}

func TestPlatformAccountRouterDisconnectsTargetWebSocketOnly(t *testing.T) {
	f := newPlatformAdminRouterFixture(t)
	actor, target := f.user(t, "super_admin"), f.user(t, "")
	workspace := f.fx.Workspace(t, "Session scope", "s05-ws-"+uuid.NewString())
	f.fx.Member(t, workspace, actor, "owner")
	f.fx.Member(t, workspace, target, "member")
	hub := realtime.NewHub()
	go hub.Run()
	f.router = NewRouter(f.pool, hub, events.New(), analytics.NoopClient{}, nil)
	server := httptest.NewServer(f.router)
	defer server.Close()
	connect := func(user string) *websocket.Conn {
		headers := http.Header{"Cookie": {auth.AuthCookieName + "=" + platformRouterJWT(t, user, nil)}}
		connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws?workspace_id="+workspace, headers)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { connection.Close() })
		deadline := time.Now().Add(3 * time.Second)
		for !hub.HasLocalSubscribers(realtime.ScopeUser, user) {
			if time.Now().After(deadline) {
				t.Fatal("websocket registration timed out")
			}
			time.Sleep(time.Millisecond)
		}
		return connection
	}
	a, b := connect(actor), connect(target)
	f.request(t, platformRouterJWT(t, actor, nil), false, http.MethodPost, "/api/admin/users/"+target+"/disable", map[string]any{"expected_auth_version": 1, "reason": "Revoke named target sessions", "password": platformRouterPassword}, nil).Want(200)
	_ = b.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := b.ReadMessage()
	if err == nil {
		t.Fatal("target websocket stayed usable")
	}
	if timeout, ok := err.(interface{ Timeout() bool }); ok && timeout.Timeout() {
		t.Fatal("target websocket was not actively disconnected")
	}
	hub.SendToUser(actor, []byte(`{"type":"actor-still-connected"}`))
	_ = a.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, message, err := a.ReadMessage()
	if err != nil || !strings.Contains(string(message), "actor-still-connected") {
		t.Fatalf("actor websocket was affected: %s, %v", message, err)
	}
}
