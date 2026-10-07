package main

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/featureflags"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/featureflag"
	"github.com/multica-ai/multica/server/pkg/plugincontract"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	publicapiv1 "github.com/multica-ai/multica/server/pkg/publicapi/v1"
)

func TestPluginActionRouteTrustBoundaries(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		authorization string
		wantStatus    int
		wantHandler   bool
		wantProblem   bool
	}{
		{
			name:       "public API rejects a missing token before the handler",
			path:       "/v1/context",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:          "public API rejects a browser session token",
			path:          "/v1/context",
			authorization: "Bearer " + testToken,
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "public API passes plugin tokens to the Action handler",
			path:          "/v1/context",
			authorization: "Bearer mpi_invalid",
			wantHandler:   true,
		},
		{
			name:          "surface bridge accepts a browser session",
			path:          "/api/plugin-bridge/v1/context",
			authorization: "Bearer " + testToken,
			wantHandler:   true,
		},
		{
			name:       "removed legacy prefix is not routed",
			path:       "/api/v1/plugin/context",
			wantStatus: http.StatusNotFound,
		},
		{
			name:          "public API does not expose person-triggered hooks",
			path:          "/v1/hooks/summarize",
			authorization: "Bearer mpi_invalid",
			wantStatus:    http.StatusNotFound,
			wantProblem:   true,
		},
		{
			name:          "public API empty resource path uses the problem contract",
			path:          "/v1/issues/",
			authorization: "Bearer mpi_invalid",
			wantStatus:    http.StatusNotFound,
			wantProblem:   true,
		},
		{
			name:          "public API unsupported method uses the problem contract",
			method:        http.MethodPut,
			path:          "/v1/context",
			authorization: "Bearer mpi_invalid",
			wantStatus:    http.StatusMethodNotAllowed,
			wantProblem:   true,
		},
		{
			name:          "surface bridge retains person-triggered hooks",
			path:          "/api/plugin-bridge/v1/hooks/summarize",
			authorization: "Bearer " + testToken,
			wantStatus:    http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := tt.method
			if method == "" {
				method = http.MethodGet
			}
			req, err := http.NewRequest(method, testServer.URL+tt.path, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer response.Body.Close()
			body, _ := io.ReadAll(response.Body)

			if tt.wantHandler {
				if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusNotFound {
					t.Fatalf("status = %d, request did not reach the Action handler", response.StatusCode)
				}
				return
			}
			if response.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.wantStatus)
			}
			if tt.wantProblem {
				if got := response.Header.Get("Content-Type"); got != publicapiv1.ProblemContentType {
					t.Fatalf("Content-Type = %q, body=%s", got, body)
				}
				var problem publicapiv1.Problem
				if err := json.Unmarshal(body, &problem); err != nil {
					t.Fatalf("decode problem: %v; body=%s", err, body)
				}
				if problem.Status != tt.wantStatus || problem.RequestID == "" {
					t.Fatalf("unexpected problem: %+v", problem)
				}
			}
		})
	}
}

func TestPluginActionBaseURLPrefersDedicatedVersionedBase(t *testing.T) {
	t.Setenv("MULTICA_PLUGIN_API_URL", " https://plugin-api.example.com/v1/ ")

	if got := pluginActionBaseURL("https://api.example.com/"); got != "https://plugin-api.example.com/v1" {
		t.Fatalf("pluginActionBaseURL() = %q", got)
	}
}

func TestPluginActionBaseURLFallsBackToPublicURL(t *testing.T) {
	t.Setenv("MULTICA_PLUGIN_API_URL", "")

	if got := pluginActionBaseURL(" https://api.example.com/ "); got != "https://api.example.com/v1" {
		t.Fatalf("pluginActionBaseURL() = %q", got)
	}
}

func TestPluginActionBaseURLOmittedWithoutPublicOrigin(t *testing.T) {
	t.Setenv("MULTICA_PLUGIN_API_URL", "")

	if got := pluginActionBaseURL(""); got != "" {
		t.Fatalf("pluginActionBaseURL() = %q, want empty", got)
	}
}

// Exercise the real Auth middleware with genuine fixture credentials. A task's
// owner belongs to both workspaces, but that does not grant the task a human
// principal in either installation. Detailed callback revocation is canonical
// in handler/plugin_password_test.go.
func TestPluginBridgePrincipalBoundary(t *testing.T) {
	for _, mode := range []string{"legacy", "password"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("MULTICA_AUTH_MODE", mode)
			t.Setenv("MULTICA_MESSAGING_INTEGRATIONS_ENABLED", "false")
			base := testutil.New(testPool, testWorkspaceID, testUserID)
			userID := base.User(t, "Bridge member", "bridge-"+uuid.NewString()+"@example.invalid")
			base.InsertNoID(t, "user_password_credential", testutil.Cols{"user_id": userID, "username": "bridge" + strings.ReplaceAll(uuid.NewString(), "-", ""), "password_hash": "unused-fixture-hash", "session_version": 1}, "user_id=$1", userID)
			jwtToken := platformRouterJWT(t, userID, nil)
			pat, err := auth.GeneratePATToken()
			if err != nil {
				t.Fatal(err)
			}
			base.Insert(t, "personal_access_token", testutil.Cols{"user_id": userID, "name": "Bridge PAT", "token_hash": auth.HashToken(pat), "token_prefix": pat[:12], "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
			var fleetCalls atomic.Int32
			fleet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/pat/verify" {
					t.Errorf("unexpected Fleet request %s", r.URL.Path)
				}
				fleetCalls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"valid": true, "owner_id": userID, "instance_id": "fixture", "instance_record_id": uuid.NewString()})
			}))
			defer fleet.Close()
			t.Setenv("MULTICA_CLOUD_URL", fleet.URL)
			var hookCalls atomic.Int32
			endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hookCalls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer endpoint.Close()
			router, h := NewRouterWithOptions(testPool, nil, events.New(), analytics.NoopClient{}, nil, RouterOptions{})
			provider := featureflag.NewStaticProvider()
			provider.Set(featureflags.PluginsV1, featureflag.Rule{Default: true})
			h.FeatureFlags = featureflag.NewService(provider)
			h.PluginService.FeatureFlags = h.FeatureFlags
			h.PluginService.DeploymentKey = []byte("0123456789abcdef0123456789abcdef")
			h.PluginService.DevOrigins = []string{endpoint.URL}
			h.PluginService.HookClient = endpoint.Client()

			parent := t
			var taskToken string
			for index, scope := range []string{"same_workspace", "cross_workspace"} {
				t.Run(scope, func(t *testing.T) {
					ws := base.Workspace(parent, "Bridge workspace", "bridge-"+uuid.NewString())
					base.Member(parent, ws, userID, "member")
					fx := testutil.New(testPool, ws, userID)
					if index == 0 {
						runtime := fx.Runtime(parent, "Bridge runtime")
						agent := fx.Agent(parent, "Bridge agent", runtime)
						task := fx.Task(parent, agent, testutil.Cols{"status": "running", "runtime_id": runtime})
						taskToken, err = auth.GenerateAgentTaskToken()
						if err != nil {
							t.Fatal(err)
						}
						// These rows outlive the same-workspace subtest: the second workspace
						// must authenticate this same task, not a new token bound to its target.
						base.Insert(parent, "task_token", testutil.Cols{"token_hash": auth.HashToken(taskToken), "task_id": task, "agent_id": agent, "workspace_id": ws, "user_id": userID, "auth_version": 1, "expires_at": time.Now().Add(time.Hour)})
					}
					issue := fx.Issue(t, "Bridge protected issue")
					scopes := []string{"issues:read", "issues:write", "comments:read", "comments:write", "storage:workspace", "storage:user", "net:127.0.0.1"}
					manifestRaw, err := json.Marshal(map[string]any{"manifest_version": 1, "key": "com.example.bridge", "name": "Bridge", "description": "Fixture", "version": "1.0.0", "author": map[string]any{"name": "Fixture"}, "scopes": scopes, "contributes": map[string]any{"hooks": []any{map[string]any{"key": "check", "name": "Check", "description": "Fixture hook", "triggers": []string{"ui", "manual"}, "transport": map[string]any{"type": "http", "url": endpoint.URL}, "timeout_ms": 1000}}}})
					if err != nil {
						t.Fatal(err)
					}
					_, manifest, err := plugincontract.ParseManifest(manifestRaw)
					if err != nil {
						t.Fatal(err)
					}
					granted, err := json.Marshal(scopes)
					if err != nil {
						t.Fatal(err)
					}
					installID := fx.Insert(t, "plugin_installation", testutil.Cols{"workspace_id": ws, "plugin_key": "com.example.bridge", "package_version_id": uuid.NewString(), "version": "1.0.0", "manifest": manifest, "granted_scopes": granted, "installed_by": userID})
					fx.Cleanup(t, "DELETE FROM comment WHERE issue_id=$1", issue)
					fx.Cleanup(t, "DELETE FROM plugin_storage WHERE installation_id=$1", installID)
					fx.Cleanup(t, "DELETE FROM plugin_invocation WHERE installation_id=$1", installID)
					fx.Cleanup(t, "DELETE FROM plugin_install_token WHERE installation_id=$1", installID)
					fx.Insert(t, "plugin_storage", testutil.Cols{"installation_id": installID, "scope_type": "workspace", "scope_id": ws, "key": "existing", "value": "original"})
					call := func(t *testing.T, token string, cookie bool, method, path string, body any, etag string) *testutil.Response {
						t.Helper()
						req := testutil.JSONRequest(method, path, body)
						if cookie {
							for _, c := range platformRouterCookies(t, token) {
								req.AddCookie(c)
								if c.Name == auth.CSRFCookieName {
									req.Header.Set("X-CSRF-Token", c.Value)
								}
							}
						} else {
							req.Header.Set("Authorization", "Bearer "+token)
						}
						req.Header.Set("X-Multica-Plugin-Installation", installID)
						req.Header.Set("X-Actor-Source", "member")
						if token == jwtToken || token == pat {
							req.Header.Set("X-Actor-Source", "task_token")
						}
						req.Header.Set("X-Workspace-ID", ws)
						req.Header.Set("X-Agent-ID", uuid.NewString())
						if etag != "" {
							req.Header.Set("If-Match", etag)
						}
						return testutil.Call(t, router.ServeHTTP, req)
					}
					issuePath := pluginBridgePrefix + "/issues/" + issue
					get := call(t, jwtToken, false, http.MethodGet, issuePath, nil, "").Want(200)
					etag := get.Header().Get("ETag")
					var initialRevision int64
					fx.QueryRow(t, "SELECT revision FROM issue WHERE id=$1", issue).Scan(&initialRevision)
					beforeHooks := hookCalls.Load()
					operations := []struct {
						method, path string
						body         any
					}{
						{"GET", "/context", nil}, {"GET", "/issues/" + issue, nil}, {"PATCH", "/issues/" + issue, map[string]any{"title": "Machine overwrite"}},
						{"GET", "/issues/" + issue + "/comments", nil}, {"POST", "/issues/" + issue + "/comments", map[string]any{"content": "Machine comment"}},
						{"GET", "/storage/workspace", nil}, {"GET", "/storage/workspace/existing", nil}, {"PUT", "/storage/workspace/new", map[string]any{"value": "machine"}},
						{"DELETE", "/storage/workspace/existing", nil}, {"POST", "/hooks/check", map[string]any{"trigger": "ui", "issue_id": issue}},
					}
					cloudStatus := http.StatusForbidden
					if mode == "password" {
						cloudStatus = http.StatusUnauthorized
					}
					for _, credential := range []struct {
						name, token string
						status      int
					}{{"task", taskToken, 403}, {"cloud", "mcn_fixture_" + uuid.NewString(), cloudStatus}} {
						for _, op := range operations {
							t.Run(credential.name+"_"+op.method+op.path, func(t *testing.T) {
								call(t, credential.token, false, op.method, pluginBridgePrefix+op.path, op.body, etag).Want(credential.status)
							})
						}
					}
					var title string
					var revision int64
					fx.QueryRow(t, "SELECT title,revision FROM issue WHERE id=$1", issue).Scan(&title, &revision)
					if title != "Bridge protected issue" || revision != initialRevision {
						t.Errorf("machine changed issue: %q revision=%d", title, revision)
					}
					if n := fx.Count(t, "SELECT count(*) FROM comment WHERE issue_id=$1", issue); n != 0 {
						t.Errorf("machine wrote %d comments", n)
					}
					if n := fx.Count(t, "SELECT count(*) FROM plugin_storage WHERE installation_id=$1 AND key='existing' AND value='original'", installID); n != 1 {
						t.Error("machine removed storage")
					}
					if n := fx.Count(t, "SELECT count(*) FROM plugin_storage WHERE installation_id=$1", installID); n != 1 {
						t.Errorf("machine changed storage count to %d", n)
					}
					if n := fx.Count(t, "SELECT count(*) FROM plugin_invocation WHERE installation_id=$1", installID); n != 0 {
						t.Errorf("machine invoked hook %d times", n)
					}
					if hookCalls.Load() != beforeHooks {
						t.Error("machine reached hook transport")
					}
					for _, human := range []struct {
						name, token string
						cookie      bool
					}{{"jwt", jwtToken, false}, {"cookie", jwtToken, true}, {"pat", pat, false}} {
						t.Run("human_"+human.name, func(t *testing.T) {
							call(t, human.token, human.cookie, "GET", pluginBridgePrefix+"/context", nil, "").Want(200)
							call(t, human.token, human.cookie, "POST", issuePath+"/comments", map[string]any{"content": human.name}, "").Want(201)
							call(t, human.token, human.cookie, "POST", pluginBridgePrefix+"/hooks/check", map[string]any{"trigger": "manual", "issue_id": issue}, "").Want(200)
							if n := fx.Count(t, "SELECT count(*) FROM comment WHERE issue_id=$1 AND content=$2 AND author_type='member' AND author_id=$3 AND via_plugin_id=$4", issue, human.name, userID, installID); n != 1 {
								t.Error("human comment attribution changed")
							}
						})
					}
					install, err := h.Queries.GetPluginInstallation(context.Background(), parseUUID(installID))
					if err != nil {
						t.Fatal(err)
					}
					installToken, err := h.PluginService.IssueInstallToken(context.Background(), install.ID)
					if err != nil {
						t.Fatal(err)
					}
					for _, actorType := range []string{"plugin", "member"} {
						actorID := install.ID
						if actorType == "member" {
							actorID = parseUUID(userID)
						}
						ctx := auth.WithPasswordSession(context.Background(), auth.PasswordSession{UserID: userID, Version: 1, Kind: "jwt"})
						callback, err := h.PluginService.Callbacks.Issue(ctx, service.HookInvocation{Installation: install, Actor: service.HookActor{Type: actorType, ID: actorID}})
						if err != nil {
							t.Fatal(err)
						}
						token := callback
						for _, kind := range []string{"callback", "install"} {
							if kind == "install" {
								if actorType == "member" {
									continue
								}
								token = installToken
							}
							call(t, token, false, "GET", "/v1/issues/"+issue, nil, "").Want(200)
							content := actorType + "_" + kind
							call(t, token, false, "POST", "/v1/issues/"+issue+"/comments", map[string]any{"content": content}, "").Want(201)
							if n := fx.Count(t, "SELECT count(*) FROM comment WHERE issue_id=$1 AND content=$2 AND author_type=$3 AND author_id=$4 AND via_plugin_id=$5", issue, content, actorType, actorID, installID); n != 1 {
								t.Errorf("%s attribution changed", content)
							}
						}
					}
					for _, token := range []string{jwtToken, pat, taskToken, "mcn_fixture"} {
						call(t, token, false, "GET", "/v1/context", nil, "").Want(401)
					}
					call(t, jwtToken, true, "GET", "/v1/context", nil, "").Want(401)
				})
			}
			if mode == "legacy" && fleetCalls.Load() == 0 {
				t.Error("cloud authentication never reached Fleet verifier")
			}
		})
	}
}
