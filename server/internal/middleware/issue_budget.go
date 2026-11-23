package middleware

import (
	"context"
	"net/http"
	"strings"
)

type issueBudgetActorHintsKey struct{}

// IssueBudgetActorHints records deny-only client hints before Auth sanitizes
// server-owned headers. It can never confer an identity or authority.
func IssueBudgetActorHints(ctx context.Context) bool {
	marked, _ := ctx.Value(issueBudgetActorHintsKey{}).(bool)
	return marked
}

// IssueBudgetHasActorHeaders distinguishes the absent agent/task headers from
// an empty actor-source value, as the management credential contract requires.
func IssueBudgetHasActorHeaders(headers http.Header) bool {
	for name, values := range headers {
		if strings.EqualFold(name, "X-Agent-ID") || strings.EqualFold(name, "X-Task-ID") {
			return true
		}
		if strings.EqualFold(name, "X-Actor-Source") {
			for _, value := range values {
				if value != "" {
					return true
				}
			}
		}
	}
	return false
}

func IsIssueBudgetManagementPath(path string) bool {
	return path == "/api/issue-budgets" || strings.HasPrefix(path, "/api/issue-budgets/") ||
		(strings.HasPrefix(path, "/api/issues/") && (strings.HasSuffix(path, "/budget") || strings.Contains(path, "/budget/")))
}

// IssueBudgetNoStore must precede both human and daemon authentication so
// rejected credentials receive the same cache policy as successful reads.
func IssueBudgetNoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if IsIssueBudgetManagementPath(path) || strings.HasPrefix(path, "/api/daemon/budget/") {
			w.Header().Set("Cache-Control", "no-store")
			if IssueBudgetHasActorHeaders(r.Header) {
				r = r.WithContext(context.WithValue(r.Context(), issueBudgetActorHintsKey{}, true))
			}
		}
		next.ServeHTTP(w, r)
	})
}
