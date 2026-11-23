package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReportSnapshotHeadersNoStoreBeforeAuthentication(t *testing.T) {
	for _, target := range []string{
		"/api/issues/00000000-0000-4000-8000-000000000001/report-snapshots/current",
		"/api/issues/00000000-0000-4000-8000-000000000001/report%2Dsnapshots/current",
		"/api//issues/00000000-0000-4000-8000-000000000001/report-snapshots/current",
		"/api/issues/00000000-0000-4000-8000-000000000001/report-snapshots/current?",
	} {
		t.Run(target, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, target, nil)
			req.Header.Add("X-Workspace-ID", "first")
			req.Header.Add("X-Workspace-ID", "second")
			out := httptest.NewRecorder()
			PreserveReportSnapshotHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Error("authentication would exit before cache protection")
				}
				original, ok := ReportSnapshotOriginalHeaders(r.Context())
				if !ok || len(original["X-Workspace-ID"]) != 2 {
					t.Fatal("original duplicate identity values were lost")
				}
				r.Header.Set("X-Workspace-ID", "authenticated")
				original["X-Workspace-ID"][0] = "changed copy"
				again, _ := ReportSnapshotOriginalHeaders(r.Context())
				if len(again["X-Workspace-ID"]) != 2 || again["X-Workspace-ID"][0] != "first" {
					t.Error("returned original headers shared mutable state")
				}
				w.WriteHeader(http.StatusUnauthorized)
			})).ServeHTTP(out, req)
			if out.Header().Get("Cache-Control") != "no-store" || out.Code != http.StatusUnauthorized {
				t.Fatal("early denial was cacheable")
			}
		})
	}
}

func TestReportSnapshotHeadersLeaveOtherNamespacesUnchanged(t *testing.T) {
	out := httptest.NewRecorder()
	PreserveReportSnapshotHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ReportSnapshotOriginalHeaders(r.Context()); ok {
			t.Error("unrelated request received snapshot identity state")
		}
		w.WriteHeader(http.StatusUnauthorized)
	})).ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/api/issues/example/comments", nil))
	if out.Header().Get("Cache-Control") != "" {
		t.Error("unrelated cache policy changed")
	}
}
