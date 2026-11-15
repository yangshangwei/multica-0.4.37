package handler

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) GetCollaborationRuntimeTestSubject(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	s := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := s.RuntimeTestSubject(r.Context(), run, token)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *Handler) RecordCollaborationRuntimeTestReport(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeJSON(w, 415, map[string]string{"code": "json_required", "error": "JSON request required"})
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, protocol.CollaborationTestReportJSONLimit))
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			writeJSON(w, 413, map[string]string{"code": "request_too_large", "error": "Test report is too large"})
		} else {
			writeJSON(w, 400, map[string]string{"code": "invalid_test_report", "error": "Invalid test report"})
		}
		return
	}
	p, err := protocol.DecodeCollaborationTestReportRequest(raw)
	if err != nil {
		writeJSON(w, 400, map[string]string{"code": "invalid_test_report", "error": "Invalid test report"})
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	s := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := s.RecordRuntimeTestReport(r.Context(), run, token, p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *Handler) GetCollaborationRuntimeTestReportReceipt(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	s := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := s.RuntimeTestReportReceipt(r.Context(), run, token, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func (h *Handler) GetCollaborationTestReports(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	run, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "run_id")
	if !ok {
		return
	}
	p := service.CollaborationTestReportsQuery{Limit: 20}
	query, err := parseCollaborationTestReportQuery(r)
	if err {
		writeJSON(w, 400, map[string]string{"code": "invalid_test_report_query", "error": "Invalid test report query"})
		return
	}
	if value := query["limit"]; value != "" {
		p.Limit, _ = strconv.Atoi(value)
	}
	p.Cursor = query["cursor"]
	s := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, e := s.TestReports(r.Context(), issue, run, p)
	if e != nil {
		writeTestReportReadError(w, e)
		return
	}
	writeJSON(w, 200, out)
}

func parseCollaborationTestReportQuery(r *http.Request) (map[string]string, bool) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, true
	}
	out := map[string]string{}
	for key, values := range q {
		if (key != "limit" && key != "cursor") || len(values) != 1 || values[0] == "" {
			return nil, true
		}
		out[key] = values[0]
	}
	if v, ok := out["limit"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 || strconv.Itoa(n) != v {
			return nil, true
		}
	}
	return out, false
}

func (h *Handler) GetCollaborationTestReport(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	run, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "run_id")
	if !ok {
		return
	}
	report, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "reportId"), "report_id")
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		writeJSON(w, 400, map[string]string{"code": "invalid_test_report_query", "error": "Invalid test report query"})
		return
	}
	s := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := s.TestReport(r.Context(), issue, run, report)
	if err != nil {
		writeTestReportReadError(w, err)
		return
	}
	writeJSON(w, 200, out)
}

func writeTestReportReadError(w http.ResponseWriter, err error) {
	var domain *service.CollaborationError
	if errors.As(err, &domain) {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "test_report_unavailable", "error": "Test report is unavailable"})
}
