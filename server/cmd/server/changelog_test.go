package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func changelogTestRouter(t *testing.T, filePath string) http.Handler {
	t.Helper()
	t.Setenv("CHANGELOG_FILE", filePath)
	// Keep router construction independent of ambient provider credentials and
	// of unrelated deployment-policy features in a concurrent working tree.
	for _, key := range []string{
		"MULTICA_LARK_SECRET_KEY", "MULTICA_SLACK_SECRET_KEY",
		"MULTICA_DINGTALK_SECRET_KEY", "MULTICA_WECOM_SECRET_KEY",
		"MULTICA_TELEGRAM_SECRET_KEY",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("CHANNEL_WS_LEASE_BACKEND", "postgres")
	router, h := NewRouterWithOptions(testPool, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil, RouterOptions{})
	t.Cleanup(func() {
		if !h.ChannelRouter.Drain(context.Background()) {
			t.Error("channel router did not drain")
		}
	})
	return router
}

func changelogAuthenticatedRequest() *http.Request {
	return testutil.WithHeaders(testutil.JSONRequest(http.MethodGet, "/api/changelog", nil), "Authorization", "Bearer "+testToken)
}

func TestChangelogRouteRequiresAuthenticationWithoutWorkspace(t *testing.T) {
	router := changelogTestRouter(t, "")
	testutil.Call(t, router.ServeHTTP, testutil.JSONRequest(http.MethodGet, "/api/changelog", nil)).Want(http.StatusUnauthorized)
	// No workspace header or membership lookup is needed for deployment-wide
	// release history; authentication uses the real production middleware.
	res := testutil.Call(t, router.ServeHTTP, changelogAuthenticatedRequest()).Want(http.StatusOK)
	if res.Map()["feed_source"] != "embedded" || res.Map()["schema_version"] != float64(1) {
		t.Fatalf("changelog route returned a different API payload: %s", res.Text())
	}
}

func TestChangelogRouteAllowsCrossOriginRevalidation(t *testing.T) {
	const origin = "https://changelog-fixture.invalid"
	t.Setenv("CORS_ALLOWED_ORIGINS", origin)
	router := changelogTestRouter(t, "")
	request := testutil.WithHeaders(testutil.JSONRequest(http.MethodOptions, "/api/changelog", nil),
		"Origin", origin,
		"Access-Control-Request-Method", http.MethodGet,
		"Access-Control-Request-Headers", "Authorization, If-None-Match")
	preflight := testutil.Call(t, router.ServeHTTP, request).Want(http.StatusOK)
	if preflight.Header().Get("Access-Control-Allow-Origin") != origin || !strings.Contains(strings.ToLower(preflight.Header().Get("Access-Control-Allow-Headers")), "if-none-match") {
		t.Fatalf("browser cannot revalidate changelog across origins: %v", preflight.Header())
	}
	request = testutil.WithHeaders(changelogAuthenticatedRequest(), "Origin", origin)
	response := testutil.Call(t, router.ServeHTTP, request).Want(http.StatusOK)
	if !strings.Contains(strings.ToLower(response.Header().Get("Access-Control-Expose-Headers")), "etag") {
		t.Fatal("browser cannot read changelog ETag")
	}
}

func TestChangelogRouterCapturesConfiguredSourceAndRefreshesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "changelog.json")
	initial := []byte(`{"schema_version":1,"generated_at":"2026-09-13T00:00:00Z","releases":[]}`)
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	router := changelogTestRouter(t, path)
	// Source configuration belongs to the router instance, not to process-global
	// cache state or an environment lookup performed for each request.
	t.Setenv("CHANGELOG_FILE", filepath.Join(t.TempDir(), "absent.json"))
	first := testutil.Call(t, router.ServeHTTP, changelogAuthenticatedRequest()).Want(http.StatusOK)
	if first.Map()["feed_source"] != "file" || first.Map()["is_stale"] != false || first.Map()["generated_at"] != "2026-09-13T00:00:00Z" {
		t.Fatalf("CHANGELOG_FILE not captured by handler construction: %s", first.Text())
	}
	updated := []byte(`{"schema_version":1,"generated_at":"2026-09-14T00:00:00Z","releases":[]}`)
	if err := os.WriteFile(path+".next", updated, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".next", path); err != nil {
		t.Fatal(err)
	}
	second := testutil.Call(t, router.ServeHTTP, changelogAuthenticatedRequest()).Want(http.StatusOK)
	if second.Map()["generated_at"] != "2026-09-14T00:00:00Z" || second.Header().Get("ETag") == first.Header().Get("ETag") {
		t.Fatalf("same production router did not expose a hot-published feed: %s", second.Text())
	}
}
