package handler

import (
	"net/http"

	"github.com/multica-ai/multica/server/internal/service"
)

// Runtime shared-context routes use the run's mcb_ business capability and the
// same current-authority gate as runtime comments.

func (h *Handler) GetCollaborationRuntimeMemoryBase(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	result, err := h.collaborationMemoryService().RuntimeMemoryBase(r.Context(), run, token)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetCollaborationRuntimeMemory(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	result, err := h.collaborationMemoryService().RuntimeMemory(r.Context(), run, token)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) UpdateCollaborationRuntimeMemory(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	svc := h.collaborationMemoryService()
	var body struct {
		RequestID   *string `json:"request_id"`
		BaseVersion *string `json:"base_version"`
		Content     *string `json:"content"`
	}
	// The generic 96 KiB runtime bound is smaller than one legal document. The
	// read bound covers worst-case JSON escaping; the service enforces the
	// redacted UTF-8 byte limit.
	if !decodeCollaborationRuntimeBodyLimit(w, r, &body, service.CollaborationMemoryReadLimit(svc.MemoryLimit())) {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	if body.RequestID == nil || body.BaseVersion == nil || body.Content == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_memory_request", "error": "Invalid shared-context request"})
		return
	}
	requestID, ok := parseUUIDOrBadRequest(w, *body.RequestID, "request_id")
	if !ok {
		return
	}
	base, ok := collaborationNonnegativeVersion(*body.BaseVersion)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_version", "error": "Invalid shared-context version"})
		return
	}
	receipt, status, err := svc.UpdateRuntimeMemory(r.Context(), run, token, service.CollaborationRuntimeMemoryParams{RequestID: requestID, BaseVersion: base, Content: *body.Content})
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, status, receipt)
}
