package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) collaborationMemoryService() *service.CollaborationService {
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	svc.MemoryMaxBytes = h.cfg.CollaborationMemoryMaxBytes
	return svc
}

// collaborationNonnegativeVersion parses an exact decimal string. "0" names
// the empty shared-context document; leading zeros are refused.
func collaborationNonnegativeVersion(value string) (int64, bool) {
	if value == "0" {
		return 0, true
	}
	return collaborationPositiveVersion(value)
}

func collaborationMemoryVersionsQuery(w http.ResponseWriter, r *http.Request) (service.CollaborationMemoryVersionsQuery, bool) {
	out := service.CollaborationMemoryVersionsQuery{Limit: 50}
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_limit", "error": "Invalid page limit"})
			return out, false
		}
		out.Limit = limit
	}
	if value := r.URL.Query().Get("cursor"); value != "" {
		version, ok := collaborationPositiveVersion(value)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_cursor", "error": "Invalid page cursor"})
			return out, false
		}
		out.BeforeVersion = pgtype.Int8{Int64: version, Valid: true}
	}
	return out, true
}

func collaborationMemoryPathVersion(w http.ResponseWriter, r *http.Request) (int64, bool) {
	version, ok := collaborationPositiveVersion(chi.URLParam(r, "version"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid shared-context version"})
	}
	return version, ok
}

func writeCollaborationMemoryReceipt(w http.ResponseWriter, receipt service.CollaborationMemoryReceipt, err error) {
	if err != nil && receipt.Code == "" {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, receipt.Status, receipt)
}

func (h *Handler) GetCollaborationMemory(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	view, err := h.collaborationMemoryService().GetMemory(r.Context(), issue)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *Handler) ListCollaborationMemoryVersions(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	query, ok := collaborationMemoryVersionsQuery(w, r)
	if !ok {
		return
	}
	page, err := h.collaborationMemoryService().ListMemoryVersions(r.Context(), issue, query)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) GetCollaborationMemoryVersion(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	version, ok := collaborationMemoryPathVersion(w, r)
	if !ok {
		return
	}
	detail, err := h.collaborationMemoryService().GetMemoryVersion(r.Context(), issue, version)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) UpdateCollaborationMemory(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	svc := h.collaborationMemoryService()
	var body struct {
		ExpectedVersion *string `json:"expected_version"`
		Content         *string `json:"content"`
	}
	// The read limit covers worst-case JSON escaping; the service enforces the
	// decoded UTF-8 byte limit and reports it with the original request key.
	if !decodeCollaborationWorkflowBodyLimit(w, r, &body, service.CollaborationMemoryReadLimit(svc.MemoryLimit())) {
		return
	}
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	if body.ExpectedVersion == nil || body.Content == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_memory_request", "error": "Invalid shared-context request"})
		return
	}
	expected, ok := collaborationNonnegativeVersion(*body.ExpectedVersion)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid shared-context version"})
		return
	}
	receipt, err := svc.UpdateMemory(r.Context(), service.CollaborationMemoryUpdateParams{IssueID: issue,
		RequestID: r.Header.Get("Idempotency-Key"), ExpectedVersion: expected, Content: *body.Content})
	writeCollaborationMemoryReceipt(w, receipt, err)
}

func (h *Handler) RestoreCollaborationMemoryVersion(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		ExpectedVersion *string `json:"expected_version"`
	}
	if !decodeCollaborationWorkflowBodyLimit(w, r, &body, 4096) {
		return
	}
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	version, ok := collaborationMemoryPathVersion(w, r)
	if !ok {
		return
	}
	if body.ExpectedVersion == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_memory_request", "error": "Invalid shared-context request"})
		return
	}
	expected, ok := collaborationNonnegativeVersion(*body.ExpectedVersion)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid shared-context version"})
		return
	}
	receipt, err := h.collaborationMemoryService().RestoreMemoryVersion(r.Context(), service.CollaborationMemoryRestoreParams{IssueID: issue,
		RequestID: r.Header.Get("Idempotency-Key"), Version: version, ExpectedVersion: expected})
	writeCollaborationMemoryReceipt(w, receipt, err)
}

func (h *Handler) PurgeCollaborationMemoryVersion(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body struct {
		Reason *string `json:"reason"`
	}
	if !decodeCollaborationWorkflowBodyLimit(w, r, &body, 8192) {
		return
	}
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	version, ok := collaborationMemoryPathVersion(w, r)
	if !ok {
		return
	}
	if body.Reason == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_reason", "error": "A purge reason is required"})
		return
	}
	receipt, err := h.collaborationMemoryService().PurgeMemoryVersion(r.Context(), service.CollaborationMemoryPurgeParams{IssueID: issue,
		RequestID: r.Header.Get("Idempotency-Key"), Version: version, Reason: *body.Reason})
	writeCollaborationMemoryReceipt(w, receipt, err)
}

func (h *Handler) GetCollaborationMemoryOperation(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	receipt, err := h.collaborationMemoryService().GetMemoryOperation(r.Context(), issue, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}
