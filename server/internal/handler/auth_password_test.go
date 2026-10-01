package handler

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func passwordTestSetup(t *testing.T) {
	t.Helper()
	t.Setenv("MULTICA_AUTH_MODE", "password")
	t.Setenv("MULTICA_PASSWORD_LIMITER_MODE", "single")
	old := testHandler.PasswordLimiter
	testHandler.PasswordLimiter = auth.NewPasswordLimiter(nil)
	t.Cleanup(func() { testHandler.PasswordLimiter = old })
	cfg := testHandler.cfg
	testHandler.cfg.AllowSignup = true
	t.Cleanup(func() { testHandler.cfg = cfg })
}
func passwordCall(t *testing.T, method, path string, body any, token string, handler http.HandlerFunc) *testutil.Response {
	t.Helper()
	req := testutil.JSONRequest(method, path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		handler = middleware.Auth(testHandler.Queries, nil, nil)(handler).ServeHTTP
	}
	return testutil.Call(t, handler, req)
}
func passwordRegister(t *testing.T, username string) LoginResponse {
	t.Helper()
	dbfx.Cleanup(t, `DELETE FROM "user" WHERE id IN (SELECT user_id FROM user_password_credential WHERE username = $1)`, username)
	dbfx.Cleanup(t, `DELETE FROM user_password_credential WHERE username = $1`, username)
	// Register the user cleanup last because credential cleanup runs first otherwise.
	var result LoginResponse
	passwordCall(t, "POST", "/auth/register", map[string]string{"username": username, "password": "correct horse battery staple", "name": "Same Name"}, "", testHandler.PasswordRegister).Want(201).JSON(&result)
	dbfx.Cleanup(t, `DELETE FROM "user" WHERE id = $1`, result.User.ID)
	return result
}
func TestPasswordRegistrationLoginAndRevocation(t *testing.T) {
	passwordTestSetup(t)
	username := fmt.Sprintf("00%d", time.Now().UnixNano())
	registered := passwordRegister(t, username)
	if registered.User.Username != username || registered.User.Email != "" || registered.User.RequiresAccountSetup || registered.User.OnboardedAt != nil {
		t.Fatalf("invalid new user: %+v", registered.User)
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM member WHERE user_id=$1`, registered.User.ID).Scan(&count)
	if count != 0 {
		t.Fatal("registration created a workspace membership")
	}
	passwordCall(t, "POST", "/auth/register", map[string]string{"username": username, "password": "correct horse battery staple", "name": "Other Name"}, "", testHandler.PasswordRegister).Want(409)
	var login LoginResponse
	passwordCall(t, "POST", "/auth/login", map[string]string{"username": username, "password": "correct horse battery staple"}, "", testHandler.PasswordLogin).Want(200).JSON(&login)
	if login.User.ID != registered.User.ID {
		t.Fatal("login changed identity")
	}
	passwordCall(t, "POST", "/auth/login", map[string]string{"username": username, "password": "wrong horse battery staple"}, "", testHandler.PasswordLogin).Want(401)
	var changed LoginResponse
	passwordCall(t, "POST", "/api/me/password/change", map[string]string{"current_password": "correct horse battery staple", "new_password": "new correct horse battery staple"}, login.Token, testHandler.PasswordChange).Want(200).JSON(&changed)
	passwordCall(t, "GET", "/api/me", nil, login.Token, testHandler.GetMe).Want(401)
	passwordCall(t, "GET", "/api/me", nil, changed.Token, testHandler.GetMe).Want(200)
	passwordCall(t, "POST", "/auth/device", map[string]string{"device_id": "old-device"}, "", testHandler.DeviceLogin).Want(403)
}

func TestPasswordAccountValidationErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		name, username, displayName, password, code string
	}{
		{"username", "a-b", "Test User", "correct horse battery staple", "invalid_username"},
		{"name", "alice", " ", "correct horse battery staple", "invalid_name"},
		{"password", "alice", "Test User", "short", "invalid_password"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := passwordAccountRequest{Username: tc.username, Name: tc.displayName, Password: tc.password}
			var result map[string]string
			testutil.Call(t, func(w http.ResponseWriter, r *http.Request) {
				if validatePasswordAccount(w, &req) {
					t.Fatal("invalid account accepted")
				}
			}, testutil.JSONRequest("POST", "/auth/register", req)).Want(400).JSON(&result)
			if result["code"] != tc.code || result["error"] == "" {
				t.Fatalf("validation response = %v; want code %q and a message", result, tc.code)
			}
		})
	}
}

func TestPasswordSixCharacterAccountFlow(t *testing.T) {
	passwordTestSetup(t)
	username := fmt.Sprintf("00%d", time.Now().UnixNano())
	dbfx.Cleanup(t, `DELETE FROM user_password_credential WHERE username=$1`, username)
	passwordCall(t, "POST", "/auth/register", map[string]string{"username": username, "password": "abc12", "name": "Short Password Test"}, "", testHandler.PasswordRegister).Want(400)
	var registered LoginResponse
	passwordCall(t, "POST", "/auth/register", map[string]string{"username": username, "password": "abc123", "name": "Short Password Test"}, "", testHandler.PasswordRegister).Want(201).JSON(&registered)
	dbfx.Cleanup(t, `DELETE FROM "user" WHERE id=$1`, registered.User.ID)
	var login LoginResponse
	passwordCall(t, "POST", "/auth/login", map[string]string{"username": username, "password": "abc123"}, "", testHandler.PasswordLogin).Want(200).JSON(&login)
	if login.User.ID != registered.User.ID {
		t.Fatal("login changed identity")
	}
	passwordCall(t, "POST", "/api/me/password/change", map[string]string{"current_password": "abc123", "new_password": "def45"}, login.Token, testHandler.PasswordChange).Want(400)
	passwordCall(t, "POST", "/api/me/password/change", map[string]string{"current_password": "abc123", "new_password": "def456"}, login.Token, testHandler.PasswordChange).Want(200)
	passwordCall(t, "POST", "/auth/login", map[string]string{"username": username, "password": "abc123"}, "", testHandler.PasswordLogin).Want(401)
	passwordCall(t, "POST", "/auth/login", map[string]string{"username": username, "password": "def456"}, "", testHandler.PasswordLogin).Want(200)
}

func TestPasswordLegacyBindingPreservesIdentity(t *testing.T) {
	passwordTestSetup(t)
	now := time.Now()
	t.Setenv("MULTICA_PASSWORD_MIGRATION_CUTOFF", now.Add(-time.Minute).UTC().Format(time.RFC3339))
	t.Setenv("MULTICA_PASSWORD_MIGRATION_DEADLINE", now.Add(time.Hour).UTC().Format(time.RFC3339))
	id := dbfx.User(t, "Legacy User", fmt.Sprintf("legacy-%d@example.com", now.UnixNano()))
	dbfx.Cleanup(t, `DELETE FROM user_password_credential WHERE user_id = $1`, id)
	fx := testutil.New(testPool, testWorkspaceID, id)
	memberID := fx.Member(t, testWorkspaceID, id, "member")
	issueID := fx.Issue(t, "Preserved legacy issue")
	commentID := fx.Comment(t, issueID, "Preserved legacy comment")
	runtimeID := fx.Runtime(t, "Legacy personal runtime", testutil.Cols{"runtime_mode": "local", "provider": "local"})
	agentID := fx.Agent(t, "Preserved legacy agent", runtimeID, testutil.Cols{"runtime_mode": "local"})
	taskID := fx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID})
	otherRuntimeID := dbfx.Runtime(t, "Other owner runtime")

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": id, "iat": now.Add(-time.Hour).Unix(), "exp": now.Add(time.Hour).Unix()}).SignedString(auth.JWTSecret())
	if err != nil {
		t.Fatal(err)
	}
	var me UserResponse
	passwordCall(t, "GET", "/api/me", nil, token, testHandler.GetMe).Want(200).JSON(&me)
	if !me.RequiresAccountSetup {
		t.Fatal("legacy user was not restricted")
	}
	passwordCall(t, "GET", "/api/workspaces", nil, token, func(w http.ResponseWriter, r *http.Request) { t.Fatal("restricted session reached business handler") }).Want(403)
	var bound LoginResponse
	passwordCall(t, "POST", "/api/me/password/setup", map[string]string{"username": fmt.Sprintf("legacy%d", now.UnixNano()), "password": "abc123", "name": "New Name"}, token, testHandler.PasswordSetup).Want(200).JSON(&bound)
	if bound.User.ID != id || bound.User.RequiresAccountSetup {
		t.Fatal("binding changed identity")
	}
	passwordCall(t, "GET", "/api/me", nil, token, testHandler.GetMe).Want(401)
	passwordCall(t, "GET", "/api/me", nil, bound.Token, testHandler.GetMe).Want(200)

	var preserved bool
	fx.QueryRow(t, `SELECT EXISTS(SELECT 1 FROM member WHERE id=$1 AND user_id=$2) AND EXISTS(SELECT 1 FROM issue WHERE id=$3 AND creator_id=$2) AND EXISTS(SELECT 1 FROM comment WHERE id=$4 AND author_id=$2) AND EXISTS(SELECT 1 FROM agent WHERE id=$5 AND archived_at IS NULL)`, memberID, id, issueID, commentID, agentID).Scan(&preserved)
	if !preserved {
		t.Fatal("binding removed business identity or relationships")
	}
	var taskStatus, runtimeStatus, otherRuntimeStatus string
	fx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, taskID).Scan(&taskStatus)
	fx.QueryRow(t, `SELECT status FROM agent_runtime WHERE id=$1`, runtimeID).Scan(&runtimeStatus)
	fx.QueryRow(t, `SELECT status FROM agent_runtime WHERE id=$1`, otherRuntimeID).Scan(&otherRuntimeStatus)
	if taskStatus != "cancelled" || runtimeStatus != "offline" || otherRuntimeStatus != "online" {
		t.Fatalf("revocation scope: task=%s runtime=%s other=%s", taskStatus, runtimeStatus, otherRuntimeStatus)
	}
	credential, err := testHandler.Queries.GetPasswordCredential(context.Background(), parseUUID(id))
	if err != nil || credential.SessionVersion != 1 {
		t.Fatal("invalid initial credential", err)
	}
}

func TestPasswordPATMintAndRevoke(t *testing.T) {
	passwordTestSetup(t)
	username := fmt.Sprintf("pat%d", time.Now().UnixNano())
	registered := passwordRegister(t, username)
	dbfx.Cleanup(t, `DELETE FROM personal_access_token WHERE user_id=$1`, registered.User.ID)
	var created CreatePATResponse
	passwordCall(t, "POST", "/api/tokens", map[string]string{"name": "test token"}, registered.Token, testHandler.CreatePersonalAccessToken).Want(201).JSON(&created)
	pat, err := testHandler.Queries.GetPersonalAccessTokenByHash(context.Background(), auth.HashToken(created.Token))
	if err != nil || pat.AuthVersion != 1 {
		t.Fatal("PAT not bound to password version", err)
	}
	passwordCall(t, "GET", "/api/me", nil, created.Token, testHandler.GetMe).Want(200)
	passwordCall(t, "POST", "/api/me/password/setup", map[string]string{"username": username, "password": "correct horse battery staple", "name": "Test"}, created.Token, testHandler.PasswordSetup).Want(403)
	var cli map[string]string
	passwordCall(t, "POST", "/api/cli-token", nil, registered.Token, testHandler.IssueCliToken).Want(200).JSON(&cli)
	passwordCall(t, "GET", "/api/me", nil, cli["token"], testHandler.GetMe).Want(200)
	passwordCall(t, "POST", "/api/me/password/change", map[string]string{"current_password": "correct horse battery staple", "new_password": "new correct horse battery staple"}, registered.Token, testHandler.PasswordChange).Want(200)
	passwordCall(t, "GET", "/api/me", nil, created.Token, testHandler.GetMe).Want(401)
	passwordCall(t, "GET", "/api/me", nil, cli["token"], testHandler.GetMe).Want(401)
	// A request authenticated before reset must not mint credentials afterward.
	ctx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: registered.User.ID, Version: 1, Kind: "jwt"})
	req := testutil.WithHeaders(testutil.JSONRequest("POST", "/api/tokens", map[string]string{"name": "late"}), "X-User-ID", registered.User.ID).WithContext(ctx)
	testutil.Call(t, testHandler.CreatePersonalAccessToken, req).Want(401)
}

func TestPasswordCLITokenRequiresJWTAndPreservesPATRenewal(t *testing.T) {
	passwordTestSetup(t)
	registered := passwordRegister(t, fmt.Sprintf("boundary%d", time.Now().UnixNano()))
	dbfx.Cleanup(t, `DELETE FROM personal_access_token WHERE user_id=$1`, registered.User.ID)
	var created CreatePATResponse
	passwordCall(t, "POST", "/api/tokens", map[string]string{"name": "CLI login"}, registered.Token, testHandler.CreatePersonalAccessToken).Want(201).JSON(&created)
	var denied map[string]string
	passwordCall(t, "POST", "/api/cli-token", nil, created.Token, testHandler.IssueCliToken).Want(403).JSON(&denied)
	if denied["code"] != "session_required" {
		t.Fatalf("PAT conversion error = %v; want session_required", denied)
	}
	dbfx.Exec(t, `UPDATE personal_access_token SET expires_at=now()+interval '1 day' WHERE user_id=$1`, registered.User.ID)
	var renewed RenewPATResponse
	passwordCall(t, "POST", "/api/tokens/current/renew", nil, created.Token, testHandler.RenewCurrentPersonalAccessToken).Want(200).JSON(&renewed)
	if !renewed.Renewed {
		t.Fatal("JWT mint restriction prevented the CLI/daemon PAT from renewing")
	}
	for _, cookie := range []bool{false, true} {
		t.Run(fmt.Sprintf("cookie=%t", cookie), func(t *testing.T) {
			req := testutil.JSONRequest("POST", "/api/cli-token", nil)
			if cookie {
				cookies := testutil.Call(t, func(w http.ResponseWriter, _ *http.Request) {
					if err := auth.SetAuthCookies(w, registered.Token); err != nil {
						t.Fatal(err)
					}
				}, testutil.JSONRequest("POST", "/auth/login", nil)).Result().Cookies()
				for _, c := range cookies {
					req.AddCookie(c)
					if c.Name == auth.CSRFCookieName {
						req.Header.Set("X-CSRF-Token", c.Value)
					}
				}
			} else {
				req.Header.Set("Authorization", "Bearer "+registered.Token)
			}
			var cli map[string]string
			testutil.Call(t, middleware.Auth(testHandler.Queries, nil, nil)(http.HandlerFunc(testHandler.IssueCliToken)).ServeHTTP, req).Want(200).JSON(&cli)
			passwordCall(t, "GET", "/api/me", nil, cli["token"], testHandler.GetMe).Want(200)
		})
	}
}

func TestPasswordDisabledAccountRejectsLoginAndCredentials(t *testing.T) {
	passwordTestSetup(t)
	registered := passwordRegister(t, fmt.Sprintf("disabled%d", time.Now().UnixNano()))
	dbfx.Cleanup(t, `DELETE FROM personal_access_token WHERE user_id=$1`, registered.User.ID)
	var created CreatePATResponse
	passwordCall(t, "POST", "/api/tokens", map[string]string{"name": "existing PAT"}, registered.Token, testHandler.CreatePersonalAccessToken).Want(201).JSON(&created)
	// Keep the version unchanged to prove the persisted disabled state is itself
	// an authentication boundary, including after restoration of old DB rows.
	dbfx.Exec(t, `UPDATE "user" SET disabled_at=now(), disabled_reason='test suspension' WHERE id=$1`, registered.User.ID)
	passwordCall(t, "POST", "/auth/login", map[string]string{"username": registered.User.Username, "password": "correct horse battery staple"}, "", testHandler.PasswordLogin).Want(403)
	for _, raw := range []string{registered.Token, created.Token} {
		passwordCall(t, "GET", "/api/me", nil, raw, testHandler.GetMe).Want(401)
	}
	if _, err := auth.CheckPasswordVersion(t.Context(), testHandler.Queries, registered.User.ID, 1); !errors.Is(err, auth.ErrPasswordSession) {
		t.Fatalf("disabled account passed version check: %v", err)
	}
	for _, daemon := range []bool{false, true} {
		if _, err := auth.CheckPasswordToken(t.Context(), testHandler.Queries, created.Token, daemon); !errors.Is(err, auth.ErrPasswordSession) {
			t.Fatalf("disabled account passed realtime/daemon token check: %v", err)
		}
	}
	ctx := auth.WithPasswordSession(t.Context(), auth.PasswordSession{UserID: registered.User.ID, Version: 1, Kind: "jwt"})
	for _, change := range []bool{false, true} {
		req := testutil.JSONRequest("POST", "/api/cli-token", nil)
		handler := testHandler.IssueCliToken
		if change {
			req = testutil.JSONRequest("POST", "/api/me/password/change", map[string]string{"current_password": "correct horse battery staple", "new_password": "should not be committed"})
			handler = testHandler.PasswordChange
		}
		req.Header.Set("X-User-ID", registered.User.ID)
		testutil.Call(t, handler, req.WithContext(ctx)).Want(401)
	}
	credential, err := testHandler.Queries.GetPasswordCredential(t.Context(), parseUUID(registered.User.ID))
	if err != nil || credential.SessionVersion != 1 {
		t.Fatalf("disabled password change mutated credential: version=%d err=%v", credential.SessionVersion, err)
	}
}

func TestPasswordDisabledLegacyAccountCannotSetUp(t *testing.T) {
	passwordTestSetup(t)
	now := time.Now()
	t.Setenv("MULTICA_PASSWORD_MIGRATION_CUTOFF", now.Add(-time.Minute).UTC().Format(time.RFC3339))
	t.Setenv("MULTICA_PASSWORD_MIGRATION_DEADLINE", now.Add(time.Hour).UTC().Format(time.RFC3339))
	id := dbfx.User(t, "Disabled legacy account", fmt.Sprintf("disabled-legacy-%d@example.com", now.UnixNano()), testutil.Cols{"disabled_at": now, "disabled_reason": "test suspension"})
	dbfx.Cleanup(t, `DELETE FROM user_password_credential WHERE user_id=$1`, id)
	claims := jwt.MapClaims{"sub": id, "iat": float64(now.Add(-time.Hour).Unix()), "exp": float64(now.Add(time.Hour).Unix())}
	if _, err := auth.CheckPasswordJWT(t.Context(), testHandler.Queries, claims); !errors.Is(err, auth.ErrPasswordSession) {
		t.Fatalf("disabled legacy account passed JWT check: %v", err)
	}
	// Exercise a request that already passed middleware before suspension.
	ctx := auth.WithPasswordSession(t.Context(), auth.PasswordSession{UserID: id, Kind: "jwt", Setup: true})
	req := testutil.JSONRequest("POST", "/api/me/password/setup", map[string]string{"username": fmt.Sprintf("dlegacy%d", now.UnixNano()), "password": "abc123", "name": "Disabled legacy account"})
	req.Header.Set("X-User-ID", id)
	testutil.Call(t, testHandler.PasswordSetup, req.WithContext(ctx)).Want(401)
	if n := dbfx.Count(t, `SELECT count(*) FROM user_password_credential WHERE user_id=$1`, id); n != 0 {
		t.Fatal("disabled account created password credentials")
	}
}

func TestPasswordTemporaryCredentialRestriction(t *testing.T) {
	passwordTestSetup(t)
	registered := passwordRegister(t, fmt.Sprintf("temp%d", time.Now().UnixNano()))
	dbfx.Exec(t, `UPDATE user_password_credential SET must_change_password=true, session_version=session_version+1 WHERE user_id=$1`, registered.User.ID)
	var temporary LoginResponse
	passwordCall(t, "POST", "/auth/login", map[string]string{"username": registered.User.Username, "password": "correct horse battery staple"}, "", testHandler.PasswordLogin).Want(200).JSON(&temporary)
	if !temporary.User.RequiresPasswordChange {
		t.Fatal("temporary session not restricted")
	}
	passwordCall(t, "POST", "/api/cli-token", nil, temporary.Token, testHandler.IssueCliToken).Want(403)
	passwordCall(t, "GET", "/api/workspaces", nil, temporary.Token, func(w http.ResponseWriter, r *http.Request) { t.Fatal("temporary session reached business handler") }).Want(403)
	var changed LoginResponse
	passwordCall(t, "POST", "/api/me/password/change", map[string]string{"current_password": "correct horse battery staple", "new_password": "new correct horse battery staple"}, temporary.Token, testHandler.PasswordChange).Want(200).JSON(&changed)
	if changed.User.RequiresPasswordChange {
		t.Fatal("password change did not clear restriction")
	}
	passwordCall(t, "GET", "/api/me", nil, temporary.Token, testHandler.GetMe).Want(401)
	passwordCall(t, "GET", "/api/me", nil, changed.Token, testHandler.GetMe).Want(200)
}

func TestPasswordRegistrationRaceAndClosedSignup(t *testing.T) {
	passwordTestSetup(t)
	username := fmt.Sprintf("race%d", time.Now().UnixNano())
	dbfx.Cleanup(t, `DELETE FROM user_password_credential WHERE username=$1`, username)
	// Each request gets its own recorder; a unique constraint decides the winner.
	responses := make(chan *testutil.Response, 2)
	for range 2 {
		go func() {
			req := testutil.JSONRequest("POST", "/auth/register", map[string]string{"username": username, "password": "correct horse battery staple", "name": "Same Name"})
			responses <- testutil.Call(t, testHandler.PasswordRegister, req)
		}()
	}
	first, second := <-responses, <-responses
	if !((first.Code == 201 && second.Code == 409) || (first.Code == 409 && second.Code == 201)) {
		t.Fatalf("concurrent signup: %d %d", first.Code, second.Code)
	}
	for _, response := range []*testutil.Response{first, second} {
		if response.Code == 201 {
			var result LoginResponse
			response.JSON(&result)
			dbfx.Cleanup(t, `DELETE FROM "user" WHERE id=$1`, result.User.ID)
		}
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM user_password_credential WHERE username=$1`, username).Scan(&count)
	if count != 1 {
		t.Fatal("duplicate credential")
	}
	testHandler.cfg.AllowSignup = false
	passwordCall(t, "POST", "/auth/register", map[string]string{"username": username + "x", "password": "correct horse battery staple", "name": "Same Name"}, "", testHandler.PasswordRegister).Want(403)
	passwordCall(t, "POST", "/auth/login", map[string]string{"username": username, "password": "correct horse battery staple"}, "", testHandler.PasswordLogin).Want(200)
}

func TestPasswordSessionsClearWildcardCDNCookies(t *testing.T) {
	passwordTestSetup(t)
	previous := testHandler.CFSigner
	testHandler.CFSigner = &auth.CloudFrontSigner{}
	t.Cleanup(func() { testHandler.CFSigner = previous })
	t.Setenv("COOKIE_DOMAIN", "example.test")
	registered := passwordRegister(t, fmt.Sprintf("cdn%d", time.Now().UnixNano()))
	dbfx.Exec(t, `UPDATE user_password_credential SET must_change_password=true WHERE user_id=$1`, registered.User.ID)
	response := passwordCall(t, "POST", "/auth/login", map[string]string{"username": registered.User.Username, "password": "correct horse battery staple"}, "", testHandler.PasswordLogin).Want(200)
	deleted := 0
	for _, cookie := range response.Result().Cookies() {
		if strings.HasPrefix(cookie.Name, "CloudFront-") {
			if cookie.Value != "" || cookie.MaxAge >= 0 {
				t.Fatal("restricted password login granted wildcard CDN access")
			}
			deleted++
		}
	}
	if deleted != 6 {
		t.Fatalf("expected host and domain CDN cookie deletion, got %d", deleted)
	}
}

func TestPasswordSessionDatabaseErrorsRemainRetryable(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{auth.ErrPasswordSession, 401}, {pgx.ErrNoRows, 401}, {errors.New("database unavailable"), 503}} {
		response := testutil.Call(t, func(w http.ResponseWriter, r *http.Request) { passwordSessionError(w, tc.err) }, testutil.JSONRequest("POST", "/api/cli-token", nil)).Want(tc.status)
		if response.Map()["code"] == "session_revoked" && tc.status == 503 {
			t.Fatal("database failure expired valid session")
		}
	}
}

func TestPasswordRejectsCrossOriginSimpleRequests(t *testing.T) {
	passwordTestSetup(t)
	username := fmt.Sprintf("csrf%d", time.Now().UnixNano())
	dbfx.Cleanup(t, `DELETE FROM user_password_credential WHERE username=$1`, username)
	dbfx.Cleanup(t, `DELETE FROM "user" WHERE id IN (SELECT user_id FROM user_password_credential WHERE username=$1)`, username)
	for _, contentType := range []string{"", "text/plain", "application/x-www-form-urlencoded", "multipart/form-data; boundary=attack"} {
		for _, route := range []struct {
			path    string
			handler http.HandlerFunc
		}{{"/auth/register", testHandler.PasswordRegister}, {"/auth/login", testHandler.PasswordLogin}} {
			body := map[string]string{"username": username, "password": "=known-password"}
			if route.path == "/auth/register" {
				body["name"] = "Attacker"
			}
			req := testutil.JSONRequest("POST", route.path, body)
			req.Header.Set("Origin", "https://untrusted.example")
			req.Header.Set("Content-Type", contentType)
			response := testutil.Call(t, route.handler, req).Want(http.StatusUnsupportedMediaType)
			if len(response.Header().Values("Set-Cookie")) != 0 {
				t.Fatal("rejected cross-origin request set authentication cookies")
			}
		}
	}
	var count int
	dbfx.QueryRow(t, `SELECT count(*) FROM user_password_credential WHERE username=$1`, username).Scan(&count)
	if count != 0 {
		t.Fatal("cross-origin form request created an account")
	}
}

func TestPasswordMigrationClaimBoundaries(t *testing.T) {
	passwordTestSetup(t)
	now := time.Now().UTC().Truncate(time.Second)
	cutoff := now.Add(-time.Hour)
	t.Setenv("MULTICA_PASSWORD_MIGRATION_CUTOFF", cutoff.Format(time.RFC3339))
	t.Setenv("MULTICA_PASSWORD_MIGRATION_DEADLINE", now.Add(time.Hour).Format(time.RFC3339))
	id := dbfx.User(t, "Migration boundaries", fmt.Sprintf("boundaries-%d@example.com", time.Now().UnixNano()))
	for _, tc := range []struct {
		name           string
		issued, expiry any
		deadline       string
		status         int
	}{
		{"eligible", cutoff.Add(-time.Second).Unix(), now.Add(time.Hour).Unix(), "", 200},
		{"missing issued", nil, now.Add(time.Hour).Unix(), "", 401},
		{"missing expiry", cutoff.Add(-time.Second).Unix(), nil, "", 401},
		{"string issued", "yesterday", now.Add(time.Hour).Unix(), "", 401},
		{"string expiry", cutoff.Add(-time.Second).Unix(), "tomorrow", "", 401},
		{"future issued", now.Add(time.Hour).Unix(), now.Add(2 * time.Hour).Unix(), "", 401},
		{"cutoff equality", cutoff.Unix(), now.Add(time.Hour).Unix(), "", 401},
		{"after cutoff", cutoff.Add(time.Second).Unix(), now.Add(time.Hour).Unix(), "", 401},
		{"expiry equality", cutoff.Add(-time.Second).Unix(), now.Unix(), "", 401},
		{"deadline reached", cutoff.Add(-time.Second).Unix(), now.Add(time.Hour).Unix(), now.Format(time.RFC3339), 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.deadline != "" {
				t.Setenv("MULTICA_PASSWORD_MIGRATION_DEADLINE", tc.deadline)
			}
			claims := jwt.MapClaims{"sub": id}
			if tc.issued != nil {
				claims["iat"] = tc.issued
			}
			if tc.expiry != nil {
				claims["exp"] = tc.expiry
			}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(auth.JWTSecret())
			if err != nil {
				t.Fatal(err)
			}
			response := passwordCall(t, "GET", "/api/me", nil, token, testHandler.GetMe).Want(tc.status)
			if tc.status == 200 {
				var me UserResponse
				response.JSON(&me)
				if !me.RequiresAccountSetup {
					t.Fatal("eligible legacy session escaped setup restriction")
				}
			}
		})
	}
}

func TestPasswordConcurrentBindingAndReplay(t *testing.T) {
	passwordTestSetup(t)
	now := time.Now().UTC()
	t.Setenv("MULTICA_PASSWORD_MIGRATION_CUTOFF", now.Add(-time.Minute).Format(time.RFC3339))
	t.Setenv("MULTICA_PASSWORD_MIGRATION_DEADLINE", now.Add(time.Hour).Format(time.RFC3339))
	id := dbfx.User(t, "Concurrent binding", fmt.Sprintf("binding-%d@example.com", now.UnixNano()))
	dbfx.Cleanup(t, `DELETE FROM user_password_credential WHERE user_id=$1`, id)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": id, "iat": now.Add(-time.Hour).Unix(), "exp": now.Add(time.Hour).Unix()}).SignedString(auth.JWTSecret())
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]string{"username": fmt.Sprintf("binding%d", now.UnixNano()), "password": "correct horse battery staple", "name": "Bound Name"}
	responses := make(chan *testutil.Response, 2)
	// Both requests have already passed middleware on the same old JWT. This
	// forces the handler's transaction fence, even if one finishes KDF first.
	for range 2 {
		req := testutil.WithHeaders(testutil.JSONRequest("POST", "/api/me/password/setup", body), "X-User-ID", id)
		req = req.WithContext(auth.WithPasswordSession(req.Context(), auth.PasswordSession{UserID: id, Kind: "jwt", Setup: true}))
		go func() { responses <- testutil.Call(t, testHandler.PasswordSetup, req) }()
	}
	a, b := <-responses, <-responses
	if a.Code != 200 {
		a, b = b, a
	}
	a.Want(200)
	b.Want(409)
	var bound LoginResponse
	a.JSON(&bound)
	if bound.User.ID != id {
		t.Fatal("binding replaced user UUID")
	}
	passwordCall(t, "POST", "/api/me/password/setup", body, token, testHandler.PasswordSetup).Want(401)
	c, err := testHandler.Queries.GetPasswordCredential(t.Context(), parseUUID(id))
	if err != nil || c.SessionVersion != 1 || c.Username != body["username"] {
		t.Fatalf("binding overwrite: version=%d username=%s err=%v", c.SessionVersion, c.Username, err)
	}
}

func TestPasswordBlockedAuthenticationCannotSurviveReset(t *testing.T) {
	passwordTestSetup(t)
	for _, kind := range []string{"login", "pat mint", "pat renew", "password change"} {
		t.Run(kind, func(t *testing.T) {
			registered := passwordRegister(t, fmt.Sprintf("fence%d", time.Now().UnixNano()))
			dbfx.Cleanup(t, `DELETE FROM personal_access_token WHERE user_id=$1`, registered.User.ID)
			var renewing CreatePATResponse
			if kind == "pat renew" {
				passwordCall(t, "POST", "/api/tokens", map[string]string{"name": "renewing"}, registered.Token, testHandler.CreatePersonalAccessToken).Want(201).JSON(&renewing)
			}
			hash, err := auth.HashPassword(t.Context(), "replacement correct horse password")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			tx, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			q := db.New(tx)
			uid := parseUUID(registered.User.ID)
			if _, err = q.LockPasswordUser(ctx, uid); err != nil {
				t.Fatal(err)
			}
			var holder int32
			if err = tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&holder); err != nil {
				t.Fatal(err)
			}
			var req *http.Request
			var handler http.HandlerFunc
			if kind == "login" {
				req = testutil.JSONRequest("POST", "/auth/login", map[string]string{"username": registered.User.Username, "password": "correct horse battery staple"})
				handler = testHandler.PasswordLogin
				req = req.WithContext(ctx)
			} else {
				req = testutil.WithHeaders(testutil.JSONRequest("POST", "/api/tokens", map[string]string{"name": "must not mint"}), "X-User-ID", registered.User.ID)
				req = req.WithContext(auth.WithPasswordSession(ctx, auth.PasswordSession{UserID: registered.User.ID, Kind: "jwt", Version: 1}))
				handler = testHandler.CreatePersonalAccessToken
				if kind == "pat renew" {
					req = testutil.WithHeaders(testutil.JSONRequest("POST", "/api/tokens/current/renew", nil), "X-User-ID", registered.User.ID, "Authorization", "Bearer "+renewing.Token)
					req = req.WithContext(auth.WithPasswordSession(ctx, auth.PasswordSession{UserID: registered.User.ID, Kind: "pat", Version: 1}))
					handler = testHandler.RenewCurrentPersonalAccessToken
				} else if kind == "password change" {
					req = testutil.WithHeaders(testutil.JSONRequest("POST", "/api/me/password/change", map[string]string{"current_password": "correct horse battery staple", "new_password": "losing concurrent password"}), "X-User-ID", registered.User.ID)
					req = req.WithContext(auth.WithPasswordSession(ctx, auth.PasswordSession{UserID: registered.User.ID, Kind: "jwt", Version: 1}))
					handler = testHandler.PasswordChange
				}
			}
			responses := make(chan *testutil.Response, 1)
			go func() { responses <- testutil.Call(t, handler, req) }()
			// Observe PostgreSQL's actual lock wait, not a sleep or goroutine start.
			for {
				var blocked bool
				if err = testPool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, holder).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				select {
				case response := <-responses:
					t.Fatalf("request returned %d before row lock", response.Code)
				case <-ctx.Done():
					t.Fatal("request never waited for user lock")
				case <-time.After(5 * time.Millisecond):
				}
			}
			if _, err = q.ChangePasswordCredential(ctx, db.ChangePasswordCredentialParams{UserID: uid, PasswordHash: hash, SessionVersion: 1}); err != nil {
				t.Fatal(err)
			}
			if _, err = auth.RevokePasswordCredentials(ctx, tx, uid); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case response := <-responses:
				response.Want(401)
				if len(response.Result().Cookies()) != 0 {
					t.Fatal("stale request set session cookies")
				}
			case <-ctx.Done():
				t.Fatal("request remained blocked after reset")
			}
			var count int
			dbfx.QueryRow(t, `SELECT count(*) FROM personal_access_token WHERE user_id=$1 AND NOT revoked`, registered.User.ID).Scan(&count)
			if count != 0 {
				t.Fatal("stale request persisted PAT")
			}
			passwordCall(t, "POST", "/auth/login", map[string]string{"username": registered.User.Username, "password": "replacement correct horse password"}, "", testHandler.PasswordLogin).Want(200)
		})
	}
}
