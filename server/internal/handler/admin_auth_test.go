package handler

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func adminHandlerSetup(t *testing.T) LoginResponse {
	t.Helper()
	passwordTestSetup(t)
	testHandler.cfg.PlatformAdminEnabled = true
	if _, err := testHandler.Queries.EnsureInternalOrganization(context.Background()); err != nil {
		t.Fatal(err)
	}
	login := passwordRegister(t, fmt.Sprintf("admin%d", time.Now().UnixNano()))
	dbfx.InsertNoID(t, "platform_role_binding", testutil.Cols{"user_id": login.User.ID, "role": "super_admin"}, "user_id=$1", login.User.ID)
	dbfx.Cleanup(t, "DELETE FROM admin_operation WHERE actor_id=$1", login.User.ID)
	dbfx.Cleanup(t, "DELETE FROM admin_audit_event WHERE actor_user_id=$1", login.User.ID)
	return login
}

func TestAdminMeUsesHumanSessionWithoutWorkspace(t *testing.T) {
	login := adminHandlerSetup(t)
	var me map[string]any
	passwordCall(t, "GET", "/api/admin/me", nil, login.Token, testHandler.AdminMe).Want(200).JSON(&me)
	if me["user_id"] != login.User.ID || me["role"] != "super_admin" || me["supported"] != true {
		t.Fatalf("identity: %+v", me)
	}
	if dbfx.Count(t, "SELECT count(*) FROM member WHERE user_id=$1", login.User.ID) != 0 {
		t.Fatal("platform access created membership")
	}
	ordinary := passwordRegister(t, fmt.Sprintf("ordinary%d", time.Now().UnixNano()))
	req := testutil.JSONRequest("GET", "/api/admin/me", nil)
	req.Header.Set("Authorization", "Bearer "+ordinary.Token)
	req.Header.Set("X-User-ID", login.User.ID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	req.Header.Set("X-Platform-Role", "super_admin")
	h := middleware.Auth(testHandler.Queries, nil, nil)(http.HandlerFunc(testHandler.AdminMe))
	testutil.Call(t, h.ServeHTTP, req).Want(403)
	dbfx.Exec(t, "DELETE FROM platform_role_binding WHERE user_id=$1", login.User.ID)
	passwordCall(t, "GET", "/api/admin/me", nil, login.Token, testHandler.AdminMe).Want(403)
	passwordCall(t, "GET", "/api/me", nil, login.Token, testHandler.GetMe).Want(200)
}

func TestAdminFeatureGateAndRoleOperationRecovery(t *testing.T) {
	login := adminHandlerSetup(t)
	testHandler.cfg.PlatformAdminEnabled = false
	passwordCall(t, "GET", "/api/admin/me", nil, login.Token, testHandler.AdminMe).Want(403)
	testHandler.cfg.PlatformAdminEnabled = true
	target := passwordRegister(t, fmt.Sprintf("observer%d", time.Now().UnixNano()))
	dbfx.Cleanup(t, "DELETE FROM platform_role_binding WHERE user_id=$1", target.User.ID)
	key := uuid.NewString()
	body := map[string]any{"role": "platform_observer", "expected_role": nil, "expected_auth_version": 1, "reason": "Allow operations monitoring", "password": "correct horse battery staple"}
	call := func() *testutil.Response {
		r := chi.NewRouter()
		r.Use(middleware.Auth(testHandler.Queries, nil, nil))
		r.Post("/api/admin/users/{id}/role", testHandler.AdminChangeRole)
		req := testutil.JSONRequest("POST", "/api/admin/users/"+target.User.ID+"/role", body)
		req.Header.Set("Authorization", "Bearer "+login.Token)
		req.Header.Set("Idempotency-Key", key)
		return testutil.Call(t, r.ServeHTTP, req)
	}
	var first, replay map[string]any
	call().Want(200).JSON(&first)
	call().Want(200).JSON(&replay)
	if first["id"] == nil || first["id"] != replay["id"] {
		t.Fatalf("operation not idempotent: %v %v", first, replay)
	}
	var found struct {
		Items []map[string]any `json:"items"`
	}
	passwordCall(t, "GET", "/api/admin/operations?idempotency_key="+key, nil, login.Token, testHandler.AdminOperations).Want(200).JSON(&found)
	if len(found.Items) != 1 || found.Items[0]["id"] != first["id"] {
		t.Fatalf("lost response lookup: %+v", found)
	}
	passwordCall(t, "GET", "/api/me", nil, target.Token, testHandler.GetMe).Want(401)
	passwordCall(t, "GET", "/api/me", nil, login.Token, testHandler.GetMe).Want(200)
	body["reason"] = "Different operation"
	call().Want(409)
}
