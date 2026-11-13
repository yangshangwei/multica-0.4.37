package middleware

import (
	"context"
	"net/http"
	"path"
	"strings"
)

type reportSnapshotHeadersKey struct{}

var reportSnapshotIdentityHeaders = []string{"X-Actor-Source", "X-User-ID", "X-Agent-ID", "X-Task-ID", "X-Workspace-ID", "X-Workspace-Slug"}

// PreserveReportSnapshotHeaders runs before Auth, which normally overwrites
// identity hints. Only the new snapshot namespace uses the stricter contract.
func PreserveReportSnapshotHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Normalization only selects cache protection, never route authority.
		protectedPath := path.Clean(r.URL.Path)
		if strings.HasPrefix(protectedPath, "/api/issues/") && strings.Contains(protectedPath, "/report-snapshots") {
			w.Header().Set("Cache-Control", "no-store")
			original := make(http.Header)
			for _, key := range reportSnapshotIdentityHeaders {
				if values, exists := r.Header[http.CanonicalHeaderKey(key)]; exists {
					original[key] = append([]string(nil), values...)
				}
			}
			r = r.WithContext(context.WithValue(r.Context(), reportSnapshotHeadersKey{}, original))
		}
		next.ServeHTTP(w, r)
	})
}

// ReportSnapshotOriginalHeaders returns a copy of the pre-auth hints. The
// private context key cannot be supplied by an HTTP caller.
func ReportSnapshotOriginalHeaders(ctx context.Context) (http.Header, bool) {
	h, ok := ctx.Value(reportSnapshotHeadersKey{}).(http.Header)
	return h.Clone(), ok
}
