package handler

import (
	"context"
	"net/http"
	"reflect"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) RegisterCollaborationRuntimeCapabilities(w http.ResponseWriter, r *http.Request) {
	if !requireCollaborationManagedSource(w, r) {
		return
	}
	var body protocol.CollaborationRuntimeCapabilitiesRequest
	if !decodeCollaborationRuntimeBody(w, r, &body) {
		return
	}
	if _, ok := parseUUIDOrBadRequest(w, body.RuntimeID, "runtime_id"); !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.RegisterRuntimeCapabilities(r.Context(), body)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetCollaborationRuntimeCapabilities(w http.ResponseWriter, r *http.Request) {
	if !requireCollaborationManagedSource(w, r) {
		return
	}
	runtimeID, ok := parseUUIDOrBadRequest(w, r.URL.Query().Get("runtime_id"), "runtime_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.RuntimeCapabilities(r.Context(), runtimeID)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetCollaborationRuntimePreparation(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	runID, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	svc := h.collaborationAttachmentService()
	result, err := svc.RuntimePreparation(r.Context(), runID, token)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetCollaborationRuntimeAttachmentContent(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	runID, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	attachmentID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "attachmentId"), "attachment_id")
	if !ok {
		return
	}
	query := r.URL.Query()
	version, err := strconv.ParseInt(query.Get("version"), 10, 64)
	if err != nil || version < 1 || strconv.FormatInt(version, 10) != query.Get("version") || len(query["version"]) != 1 || len(query) != 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_attachment_version", "error": "Invalid attachment version"})
		return
	}
	svc := h.collaborationAttachmentService()
	row, err := svc.RuntimeAttachment(r.Context(), runID, attachmentID, version, token)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	// Machine downloads use the full authorized file, including large text. The
	// shared byte path bounds buffering and reauthorizes after storage I/O.
	h.serveCollaborationAttachmentBytes(w, r, svc, row, true, func(ctx context.Context) error {
		current, err := svc.RuntimeAttachment(ctx, runID, attachmentID, version, token)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, row) {
			return &service.CollaborationError{Status: http.StatusConflict, Code: "attachment_changed"}
		}
		w.Header().Set("X-Multica-Run-ID", chi.URLParam(r, "runId"))
		w.Header().Set("X-Multica-Attachment-ID", chi.URLParam(r, "attachmentId"))
		w.Header().Set("X-Multica-Attachment-Version", strconv.FormatInt(version, 10))
		return nil
	})
}

func (h *Handler) RecordCollaborationRuntimePreparation(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	var body protocol.CollaborationPreparationObservation
	if !decodeCollaborationRuntimeBody(w, r, &body) {
		return
	}
	runID, ok := collaborationRuntimeRunID(w, r, &body.Identity.RunID)
	if !ok {
		return
	}
	svc := h.collaborationAttachmentService()
	result, err := svc.RecordRuntimePreparation(r.Context(), runID, token, body)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
