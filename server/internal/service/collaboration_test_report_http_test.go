package service_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// Keep the artifact/claim/report setup canonical in service tests while using
// the actual HTTP adapters to check that denied output is content-free.
func TestStageArtifactReportHTTPReadAuthorityEnvelope(t *testing.T) {
	svc, manager, member, report, demote := service.StageArtifactReportTransportFixture(t)
	h := &handler.Handler{Queries: svc.Queries, TxStarter: svc.TxStarter, IssueService: svc.Issues}
	for _, detail := range []bool{false, true} {
		fn, suffix := h.GetCollaborationTestReports, ""
		if detail {
			fn, suffix = h.GetCollaborationTestReport, "/"+report.ReportID
		}
		request := func(ctx context.Context) *http.Request {
			r := testutil.WithURLParams(testutil.JSONRequest(http.MethodGet, "/api/collaboration/issues/"+report.IssueID+"/runs/"+report.RunID+"/test-reports"+suffix, nil).WithContext(ctx),
				"id", report.IssueID, "runId", report.RunID, "reportId", report.ReportID)
			return r
		}
		allowed := testutil.Call(t, fn, request(manager)).Want(http.StatusOK)
		if allowed.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private report response was cacheable")
		}
		denied := testutil.Call(t, fn, request(member)).Want(http.StatusNotFound)
		body := denied.Map()
		if denied.Header().Get("Cache-Control") != "no-store" || len(body) != 2 || body["code"] != "not_found" || body["error"] != "not_found" {
			t.Fatalf("denied report contained a private projection: %s", denied.Text())
		}
	}
	demote()
	for _, fn := range []http.HandlerFunc{h.GetCollaborationTestReports, h.GetCollaborationTestReport} {
		r := testutil.WithURLParams(testutil.JSONRequest(http.MethodGet, "/api/collaboration/issues/"+report.IssueID+"/runs/"+report.RunID+"/test-reports", nil).WithContext(manager),
			"id", report.IssueID, "runId", report.RunID, "reportId", report.ReportID)
		denied := testutil.Call(t, fn, r).Want(http.StatusNotFound)
		if body := denied.Map(); len(body) != 2 || body["code"] != "not_found" || body["error"] != "not_found" {
			t.Fatalf("revoked manager recovered private report output: %s", denied.Text())
		}
	}
}
