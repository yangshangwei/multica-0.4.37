package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const platformRouterPassword = "router-admin-password-42"

type platformAdminRouterFixture struct {
	pool   *pgxpool.Pool
	fx     *testutil.Fixture
	router http.Handler
	hash   string
}

func newPlatformAdminRouterFixture(t *testing.T) *platformAdminRouterFixture {
	t.Helper()
	if testPool == nil {
		t.Fatal("real-router database fixture is required")
	}
	t.Setenv("MULTICA_AUTH_MODE", "password")
	t.Setenv("MULTICA_PLATFORM_ADMIN_ENABLED", "true")
	t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "local")
	t.Setenv("MULTICA_MESSAGING_INTEGRATIONS_ENABLED", "false")
	schema := "admin_router_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	qualifiedSchema := pgx.Identifier{schema}.Sanitize()
	if _, err := testPool.Exec(context.Background(), "CREATE SCHEMA "+qualifiedSchema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := testPool.Exec(context.Background(), "DROP SCHEMA "+qualifiedSchema+" CASCADE"); err != nil {
			t.Errorf("remove router test schema: %v", err)
		}
	})
	// Auth and admin mutations must never see another test package's platform
	// roles, organizations, or credentials. The remaining router is production.
	for _, table := range []string{"user", "user_password_credential", "platform_role_binding", "organization", "organization_workspace", "admin_operation", "admin_audit_event", "personal_access_token", "task_token", "daemon_token", "workspace", "member", "agent", "agent_runtime", "agent_task_queue"} {
		dst, src := pgx.Identifier{schema, table}.Sanitize(), pgx.Identifier{"public", table}.Sanitize()
		if _, err := testPool.Exec(context.Background(), "CREATE TABLE "+dst+" (LIKE "+src+" INCLUDING ALL)"); err != nil {
			t.Fatalf("clone router fixture %s: %v", table, err)
		}
	}
	config := testPool.Config().Copy()
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	fx.Insert(t, "organization", testutil.Cols{"name": "Router test organization"})
	hash, err := auth.HashPassword(context.Background(), platformRouterPassword)
	if err != nil {
		t.Fatal(err)
	}
	return &platformAdminRouterFixture{pool: pool, fx: fx, router: NewRouter(pool, nil, events.New(), analytics.NoopClient{}, nil), hash: hash}
}

func (f *platformAdminRouterFixture) user(t *testing.T, role string) string {
	t.Helper()
	unique := strings.ReplaceAll(uuid.NewString(), "-", "")
	userID := f.fx.User(t, "Router admin user", "router-"+unique+"@example.invalid")
	f.fx.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": userID, "username": unique, "password_hash": f.hash}, "user_id = $1", userID)
	if role != "" {
		f.fx.InsertNoID(t, "platform_role_binding", testutil.Cols{"user_id": userID, "role": role}, "user_id = $1", userID)
	}
	return userID
}
func platformRouterJWT(t *testing.T, userID string, extra jwt.MapClaims) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": userID, "auth_version": 1, "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()}
	for key, value := range extra {
		claims[key] = value
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(auth.JWTSecret())
	if err != nil {
		t.Fatal(err)
	}
	return token
}
func platformRouterCookies(t *testing.T, token string) []*http.Cookie {
	t.Helper()
	recorder := httptest.NewRecorder()
	if err := auth.SetAuthCookies(recorder, token); err != nil {
		t.Fatal(err)
	}
	return recorder.Result().Cookies()
}
func platformRoleBody() map[string]any {
	return map[string]any{"role": "platform_observer", "expected_role": nil, "expected_auth_version": 1, "reason": "Reviewed router authorization grant", "password": platformRouterPassword}
}
func (f *platformAdminRouterFixture) request(t *testing.T, token string, cookie bool, method, path string, body any, headers map[string]string) *testutil.Response {
	t.Helper()
	req := testutil.JSONRequest(method, path, body)
	if cookie {
		for _, c := range platformRouterCookies(t, token) {
			req.AddCookie(c)
			if c.Name == auth.CSRFCookieName {
				req.Header.Set("X-CSRF-Token", c.Value)
			}
		}
	} else if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Idempotency-Key", uuid.NewString())
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return testutil.Call(t, f.router.ServeHTTP, req)
}
func platformAssertNoStore(t *testing.T, response *testutil.Response) {
	t.Helper()
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("admin response Cache-Control = %q, want no-store", got)
	}
}

// This is the transport/credential matrix. Transaction races, idempotent
// persistence and last-admin semantics are canonical in service tests.
func TestPlatformAdminRouterCredentialMatrix(t *testing.T) {
	f := newPlatformAdminRouterFixture(t)
	super, observer, ordinary := f.user(t, "super_admin"), f.user(t, "platform_observer"), f.user(t, "")
	workspace := f.fx.Workspace(t, "Ordinary administrator workspace", "admin-router-"+uuid.NewString())
	f.fx.Member(t, workspace, ordinary, "admin")
	if n := f.fx.Count(t, "SELECT count(*) FROM member WHERE user_id = $1", super); n != 0 {
		t.Fatal("super administrator fixture unexpectedly has a workspace")
	}
	pat, err := auth.GeneratePATToken()
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Insert(t, "personal_access_token", testutil.Cols{"user_id": super, "name": "Super administrator PAT", "token_hash": auth.HashToken(pat), "token_prefix": pat[:12], "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
	runtime := f.fx.Runtime(t, "Token runtime", testutil.Cols{"workspace_id": workspace, "owner_id": super})
	agent := f.fx.Agent(t, "Token agent", runtime, testutil.Cols{"workspace_id": workspace, "owner_id": super})
	task := f.fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "status": "running"})
	taskToken, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Insert(t, "task_token", testutil.Cols{"user_id": super, "workspace_id": workspace, "agent_id": agent, "task_id": task, "token_hash": auth.HashToken(taskToken), "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
	recovery := f.user(t, "super_admin")
	f.fx.Exec(t, "UPDATE user_password_credential SET must_change_password = true WHERE user_id = $1", recovery)
	daemonToken, err := auth.GenerateDaemonToken()
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Insert(t, "daemon_token", testutil.Cols{"user_id": super, "workspace_id": workspace, "daemon_id": uuid.NewString(), "token_hash": auth.HashToken(daemonToken), "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
	if _, err := auth.CheckPasswordToken(context.Background(), db.New(f.pool), daemonToken, true); err != nil {
		t.Fatalf("daemon credential control is not valid for its own protocol: %v", err)
	}
	superJWT := platformRouterJWT(t, super, nil)
	observerJWT := platformRouterJWT(t, observer, nil)
	ordinaryJWT := platformRouterJWT(t, ordinary, jwt.MapClaims{"role": "super_admin", "platform_role": "super_admin"})
	cases := []struct {
		name, token             string
		cookie                  bool
		readStatus, writeStatus int
		headers                 map[string]string
	}{
		{name: "super_jwt_no_workspace", token: superJWT, readStatus: 200, writeStatus: 200},
		{name: "super_cookie_no_workspace", token: superJWT, cookie: true, readStatus: 200, writeStatus: 200},
		{name: "super_unrelated_workspace_header", token: superJWT, readStatus: 200, writeStatus: 200, headers: map[string]string{"X-Workspace-ID": uuid.NewString()}},
		{name: "stale_super_jwt", token: platformRouterJWT(t, super, jwt.MapClaims{"auth_version": 2}), readStatus: 401, writeStatus: 401},
		{name: "expired_super_jwt", token: platformRouterJWT(t, super, jwt.MapClaims{"exp": time.Now().Add(-time.Minute).Unix()}), readStatus: 401, writeStatus: 401},
		{name: "recovery_super_jwt", token: platformRouterJWT(t, recovery, nil), readStatus: 403, writeStatus: 403},
		{name: "observer_jwt", token: observerJWT, readStatus: 200, writeStatus: 403},
		{name: "observer_cookie", token: observerJWT, cookie: true, readStatus: 200, writeStatus: 403},
		{name: "workspace_admin_jwt", token: ordinaryJWT, readStatus: 403, writeStatus: 403, headers: map[string]string{"X-Workspace-ID": workspace}},
		{name: "workspace_admin_cookie", token: ordinaryJWT, cookie: true, readStatus: 403, writeStatus: 403},
		{name: "forged_identity_and_role_headers", token: ordinaryJWT, readStatus: 403, writeStatus: 403, headers: map[string]string{"X-User-ID": super, "X-Role": "super_admin", "X-Platform-Role": "super_admin", "X-Actor-Source": "jwt"}},
		{name: "super_pat", token: pat, readStatus: 403, writeStatus: 403, headers: map[string]string{"X-Actor-Source": "jwt"}},
		{name: "super_task_token", token: taskToken, readStatus: 403, writeStatus: 403},
		{name: "super_daemon_token", token: daemonToken, readStatus: 401, writeStatus: 401},
		{name: "cloud_pat", token: "mcn_" + strings.Repeat("2", 40), readStatus: 401, writeStatus: 401},
		{name: "plugin_installation_token", token: "mpi_" + strings.Repeat("3", 40), readStatus: 401, writeStatus: 401},
		{name: "plugin_callback_token", token: "mpc_" + strings.Repeat("4", 40), readStatus: 401, writeStatus: 401},
		{name: "unknown_credential", token: "unknown-token", readStatus: 401, writeStatus: 401},
		{name: "missing_credential", readStatus: 401, writeStatus: 401},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, path := range []string{"/api/admin/me", "/api/admin/operations?idempotency_key=" + uuid.NewString()} {
				response := f.request(t, tc.token, tc.cookie, http.MethodGet, path, nil, tc.headers).Want(tc.readStatus)
				platformAssertNoStore(t, response)
				if path == "/api/admin/me" && tc.readStatus == 200 {
					me := response.Map()
					expected := super
					if strings.HasPrefix(tc.name, "observer") {
						expected = observer
					}
					if me["user_id"] != expected {
						t.Fatalf("admin identity came from an untrusted header: %v", me)
					}
				}
			}
			target := f.user(t, "")
			response := f.request(t, tc.token, tc.cookie, http.MethodPost, "/api/admin/users/"+target+"/role", platformRoleBody(), tc.headers).Want(tc.writeStatus)
			platformAssertNoStore(t, response)
			if tc.writeStatus == 200 {
				result := response.Map()
				id, ok := result["id"].(string)
				if !ok || id == "" {
					t.Fatalf("role response has no operation id: %v", result)
				}
				platformAssertNoStore(t, f.request(t, tc.token, tc.cookie, http.MethodGet, "/api/admin/operations/"+id, nil, nil).Want(200))
			} else if n := f.fx.Count(t, "SELECT count(*) FROM platform_role_binding WHERE user_id = $1", target); n != 0 {
				t.Fatal("rejected credential changed platform role")
			}
		})
	}
}

func TestPlatformAdminRouterCookieWritesRequireBoundCSRF(t *testing.T) {
	f := newPlatformAdminRouterFixture(t)
	actor, target := f.user(t, "super_admin"), f.user(t, "")
	token := platformRouterJWT(t, actor, nil)
	otherToken := platformRouterJWT(t, f.user(t, ""), nil)
	var otherCSRF string
	for _, cookie := range platformRouterCookies(t, otherToken) {
		if cookie.Name == auth.CSRFCookieName {
			otherCSRF = cookie.Value
		}
	}
	for _, csrf := range []string{"", "forged.csrf", otherCSRF} {
		response := f.request(t, token, true, http.MethodPost, "/api/admin/users/"+target+"/role", platformRoleBody(), map[string]string{"X-CSRF-Token": csrf}).Want(http.StatusForbidden)
		platformAssertNoStore(t, response)
	}
	if n := f.fx.Count(t, "SELECT count(*) FROM admin_operation"); n != 0 {
		t.Fatal("CSRF rejection wrote an operation")
	}
	platformAssertNoStore(t, f.request(t, token, true, http.MethodPost, "/api/admin/users/"+target+"/role", platformRoleBody(), nil).Want(http.StatusOK))
}

func TestPlatformAdminRouterRejectsDisabledFeatureAndOtherAuthMode(t *testing.T) {
	for _, tc := range []struct{ name, mode, enabled string }{{"feature_disabled", "password", "false"}, {"classic_auth", "classic", "true"}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPlatformAdminRouterFixture(t)
			actor, target := f.user(t, "super_admin"), f.user(t, "")
			t.Setenv("MULTICA_AUTH_MODE", tc.mode)
			t.Setenv("MULTICA_PLATFORM_ADMIN_ENABLED", tc.enabled)
			f.router = NewRouter(f.pool, nil, events.New(), analytics.NoopClient{}, nil)
			token := platformRouterJWT(t, actor, nil)
			for _, request := range []struct {
				method, path string
				body         any
			}{{http.MethodGet, "/api/admin/me", nil}, {http.MethodPost, "/api/admin/users/" + target + "/role", platformRoleBody()}} {
				response := f.request(t, token, false, request.method, request.path, request.body, nil).Want(http.StatusForbidden)
				platformAssertNoStore(t, response)
				if code := response.Map()["code"]; code != "admin_mode_disabled" {
					t.Fatalf("disabled-mode code = %v", code)
				}
			}
		})
	}
}

func TestPlatformAdminRouterDatabaseFailureIsUnavailable(t *testing.T) {
	f := newPlatformAdminRouterFixture(t)
	token := platformRouterJWT(t, f.user(t, "super_admin"), nil)
	f.pool.Close()
	response := f.request(t, token, false, http.MethodGet, "/api/admin/me", nil, nil).Want(http.StatusServiceUnavailable)
	platformAssertNoStore(t, response)
	if code := response.Map()["code"]; code != "auth_unavailable" {
		t.Fatalf("database outage reported as an authorization decision: %v", code)
	}
}

func TestPlatformAdminRouterMissingOrInactiveOrganizationIsUnavailable(t *testing.T) {
	for _, state := range []string{"missing", "inactive"} {
		t.Run(state, func(t *testing.T) {
			f := newPlatformAdminRouterFixture(t)
			token := platformRouterJWT(t, f.user(t, "super_admin"), nil)
			if state == "missing" {
				f.fx.Exec(t, "DELETE FROM organization")
			} else {
				f.fx.Exec(t, "UPDATE organization SET state = 'inactive'")
			}
			response := f.request(t, token, false, http.MethodGet, "/api/admin/me", nil, nil).Want(http.StatusServiceUnavailable)
			platformAssertNoStore(t, response)
			if code := response.Map()["code"]; code != "admin_unavailable" {
				t.Fatalf("organization fault code = %v, want admin_unavailable", code)
			}
		})
	}
}
