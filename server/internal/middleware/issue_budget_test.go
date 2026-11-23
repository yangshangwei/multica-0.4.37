package middleware

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestIssueBudgetNoStore(t *testing.T) {
	for _, path := range []string{
		"/api/issue-budgets", "/api/issue-budgets/root", "/api/issue-budgets/root/operations/key",
		"/api/issues/MUL-1/budget", "/api/issues/id/budget/operations/key",
		"/api/daemon/budget/usage", "/api/daemon/budget/operations/key",
	} {
		for _, status := range []int{200, 401, 403, 404, 409} {
			h := IssueBudgetNoStore(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			r := testutil.Call(t, h.ServeHTTP, testutil.JSONRequest("GET", path, nil)).Want(status)
			if r.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("%s status %d lacks no-store", path, status)
			}
		}
	}
	for _, path := range []string{"/api/issues", "/api/issues/id", "/api/issue-budgets-other", "/api/daemon/tasks", "/api/other/id/budget"} {
		h := IssueBudgetNoStore(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
		r := testutil.Call(t, h.ServeHTTP, testutil.JSONRequest("GET", path, nil)).Want(200)
		if r.Header().Get("Cache-Control") != "" {
			t.Fatalf("unrelated %s received budget cache policy", path)
		}
	}
}

func TestIssueBudgetActorHeaderPresence(t *testing.T) {
	for _, tc := range []struct {
		header http.Header
		deny   bool
	}{
		{http.Header{}, false},
		{http.Header{"X-Agent-Id": {""}}, true},
		{http.Header{"X-Task-Id": nil}, true},
		{http.Header{"X-Actor-Source": {"", ""}}, false},
		{http.Header{"X-Actor-Source": {"", "agent"}}, true},
		{http.Header{"x-actor-source": {"agent", ""}}, true},
	} {
		if got := IssueBudgetHasActorHeaders(tc.header); got != tc.deny {
			t.Errorf("%v denied=%v want %v", tc.header, got, tc.deny)
		}
	}
}
