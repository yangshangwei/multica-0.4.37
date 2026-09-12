package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/changelog"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Reader validation and concurrency matrices live in internal/changelog.
// These tests cover only the HTTP contract; the real auth/config wiring is
// exercised by cmd/server/changelog_test.go.
func changelogTestHandler(filePath, version string) *Handler {
	return &Handler{changelogReader: changelog.New(filePath, version)}
}

func assertChangelogHeaders(t *testing.T, header http.Header) {
	t.Helper()
	for key, want := range map[string]string{
		"Content-Type":           "application/json",
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "private, no-cache",
	} {
		if got := header.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if etag := header.Get("ETag"); len(etag) != 66 || etag[0] != '"' || etag[len(etag)-1] != '"' {
		t.Errorf("ETag is not a strong SHA-256 validator: %q", etag)
	}
}

func TestGetChangelogReturnsCompleteEnvelope(t *testing.T) {
	for _, version := range []string{"", "v1.2.3"} {
		t.Run("server_version="+version, func(t *testing.T) {
			h := changelogTestHandler("", version)
			res := testutil.Call(t, h.GetChangelog, testutil.JSONRequest(http.MethodGet, "/api/changelog", nil)).Want(http.StatusOK)
			assertChangelogHeaders(t, res.Header())
			if got := res.Header().Get("Content-Length"); got != strconv.Itoa(res.Body.Len()) {
				t.Errorf("Content-Length = %q, want %d", got, res.Body.Len())
			}
			var out changelog.Response
			res.JSON(&out)
			if out.SchemaVersion != 1 || out.GeneratedAt == "" || out.Releases == nil || out.FeedSource != "embedded" || out.IsStale || out.Warning != nil {
				t.Fatalf("incomplete changelog envelope: %+v", out)
			}
			if version == "" {
				if value, ok := res.Map()["server_version"]; !ok || value != nil {
					t.Fatalf("missing build version must be explicit null, got %v", value)
				}
			} else if out.ServerVersion == nil || *out.ServerVersion != version {
				t.Fatalf("server version = %v, want %q", out.ServerVersion, version)
			}
		})
	}
}

func TestGetChangelogConditionalRequests(t *testing.T) {
	h := changelogTestHandler("", "v1.2.3")
	initial := testutil.Call(t, h.GetChangelog, testutil.JSONRequest(http.MethodGet, "/api/changelog", nil)).Want(http.StatusOK)
	etag := initial.Header().Get("ETag")
	for name, value := range map[string]string{
		"strong":   etag,
		"weak":     "W/" + etag,
		"list":     `"unrelated", W/` + etag,
		"wildcard": "*",
	} {
		t.Run(name, func(t *testing.T) {
			req := testutil.WithHeaders(testutil.JSONRequest(http.MethodGet, "/api/changelog", nil), "If-None-Match", value)
			res := testutil.Call(t, h.GetChangelog, req).Want(http.StatusNotModified)
			assertChangelogHeaders(t, res.Header())
			if res.Body.Len() != 0 {
				t.Fatal("304 contains a JSON body")
			}
		})
	}
	req := testutil.WithHeaders(testutil.JSONRequest(http.MethodGet, "/api/changelog", nil), "If-None-Match", `"different"`)
	testutil.Call(t, h.GetChangelog, req).Want(http.StatusOK)
}

func writeChangelogHTTPFixture(t *testing.T, path, generatedAt string) {
	t.Helper()
	data, err := json.Marshal(changelog.Feed{SchemaVersion: 1, GeneratedAt: generatedAt, Releases: []changelog.Release{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".next", data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".next", path); err != nil {
		t.Fatal(err)
	}
}

func TestGetChangelogETagChangesOnReplacementAndRefreshFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private-feed.json")
	writeChangelogHTTPFixture(t, path, "2026-09-13T00:00:00Z")
	h := changelogTestHandler(path, "v1.2.3")
	request := func(etag string) *http.Request {
		return testutil.WithHeaders(testutil.JSONRequest(http.MethodGet, "/api/changelog", nil), "If-None-Match", etag)
	}
	first := testutil.Call(t, h.GetChangelog, request("")).Want(http.StatusOK)
	firstETag := first.Header().Get("ETag")
	testutil.Call(t, h.GetChangelog, request(firstETag)).Want(http.StatusNotModified)

	writeChangelogHTTPFixture(t, path, "2026-09-14T00:00:00Z")
	updated := testutil.Call(t, h.GetChangelog, request(firstETag)).Want(http.StatusOK)
	if updated.Map()["generated_at"] != "2026-09-14T00:00:00Z" || updated.Header().Get("ETag") == firstETag {
		t.Fatal("same handler did not expose replaced file")
	}
	if err := os.WriteFile(path, []byte("broken JSON"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := testutil.Call(t, h.GetChangelog, request(updated.Header().Get("ETag"))).Want(http.StatusOK)
	var out changelog.Response
	stale.JSON(&out)
	if !out.IsStale || out.Warning == nil || *out.Warning != changelog.WarningFileInvalid || out.FeedSource != "file" || out.GeneratedAt != "2026-09-14T00:00:00Z" {
		t.Fatalf("invalid file must preserve history with public stale metadata: %+v", out)
	}
	if strings.Contains(stale.Text(), path) || strings.Contains(stale.Text(), "broken JSON") {
		t.Fatal("response contains internal source/error details")
	}
	testutil.Call(t, h.GetChangelog, request(stale.Header().Get("ETag"))).Want(http.StatusNotModified)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	missing := testutil.Call(t, h.GetChangelog, request(stale.Header().Get("ETag"))).Want(http.StatusOK)
	if missing.Map()["warning"] != changelog.WarningFileUnavailable {
		t.Fatal("changed source failure must invalidate the old warning")
	}
	writeChangelogHTTPFixture(t, path, "2026-09-14T00:00:00Z")
	recovered := testutil.Call(t, h.GetChangelog, request(missing.Header().Get("ETag"))).Want(http.StatusOK)
	if recovered.Map()["is_stale"] != false || recovered.Header().Get("ETag") != updated.Header().Get("ETag") {
		t.Fatal("restored source did not recover original healthy response")
	}
}

func TestGetChangelogIgnoresClientSourceSelection(t *testing.T) {
	h := changelogTestHandler("", "v1.2.3")
	first := testutil.Call(t, h.GetChangelog, testutil.JSONRequest(http.MethodGet, "/api/changelog", nil)).Want(http.StatusOK)
	malicious := testutil.Call(t, h.GetChangelog, testutil.JSONRequest(http.MethodGet, "/api/changelog?file=/etc/passwd&url=https://example.invalid", nil)).Want(http.StatusOK)
	if first.Text() != malicious.Text() {
		t.Fatal("client parameters changed deployment-owned source")
	}
}
