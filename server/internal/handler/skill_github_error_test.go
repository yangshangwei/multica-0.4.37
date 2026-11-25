package handler

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestFetchGitHubTreeErrorDiagnostics(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		headers map[string]string
		want    string
	}{
		{"primary quota", 403, map[string]string{"X-RateLimit-Remaining": "0"}, "primary rate limit exceeded"},
		{"secondary throttle", 403, map[string]string{"Retry-After": "60"}, "request throttled"},
		{"dated throttle", 403, map[string]string{"Retry-After": "Wed, 25 Nov 2026 10:00:00 GMT"}, "request throttled"},
		{"unknown forbidden", 403, nil, "access forbidden; reason unconfirmed"},
		{"remaining quota", 403, map[string]string{"X-RateLimit-Remaining": "35"}, "access forbidden; reason unconfirmed"},
		{"invalid retry delay", 403, map[string]string{"Retry-After": "private-header-marker"}, "access forbidden; reason unconfirmed"},
		{"negative retry delay", 403, map[string]string{"Retry-After": "-1"}, "access forbidden; reason unconfirmed"},
		{"authentication", 401, nil, "authentication rejected"},
		{"throttled", 429, nil, "rate limit exceeded"},
		{"missing", 404, nil, "request failed"},
		{"unavailable", 503, nil, "request failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, requests := newGitHubFixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
				for name, value := range tt.headers {
					w.Header().Set(name, value)
				}
				w.Header().Set("Set-Cookie", "private-cookie-marker")
				w.Header().Set("X-GitHub-Request-Id", "private-header-marker")
				http.Error(w, "private-body-marker", tt.status)
			})
			entries, truncated, err := fetchGitHubTree(t.Context(), client, "acme", "skills", "main")
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), http.StatusText(tt.status)) {
				t.Fatalf("error = %v, want status %d and %q", err, tt.status, tt.want)
			}
			if entries != nil || truncated || len(*requests) != 1 {
				t.Fatalf("failed tree response must not yield entries or retry: entries=%v truncated=%v requests=%v", entries, truncated, *requests)
			}
			if strings.Contains(err.Error(), "private-") {
				t.Fatalf("upstream private data leaked: %v", err)
			}
		})
	}
}

func TestFetchFromSkillsShForbiddenDoesNotAssumeRateLimit(t *testing.T) {
	client, requests := newGitHubFixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/acme/skills" {
			writeJSON(w, http.StatusOK, map[string]string{"default_branch": "main"})
			return
		}
		http.Error(w, "private-body-marker", http.StatusForbidden)
	})
	result, err := fetchFromSkillsSh(t.Context(), client, "https://skills.sh/acme/skills/example")
	if result != nil || !errors.Is(err, errImportSourceUnavailable) {
		t.Fatalf("result = %v, error = %v; want unavailable import", result, err)
	}
	if !strings.Contains(err.Error(), "access forbidden; reason unconfirmed") || strings.Contains(err.Error(), "GITHUB_TOKEN") || strings.Contains(err.Error(), "private-body-marker") {
		t.Fatalf("error must retain evidence without assuming credentials solve it: %v", err)
	}
	if len(*requests) != 2 {
		t.Fatalf("unexpected fallback or retry: %v", *requests)
	}
}
