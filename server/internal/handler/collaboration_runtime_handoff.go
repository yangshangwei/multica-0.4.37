package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) GetCollaborationRuntimeHandoff(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := svc.RuntimeHandoff(r.Context(), run, token, service.CollaborationHandoffQuery{Cursor: r.URL.Query().Get("cursor")})
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (h *Handler) CreateCollaborationRuntimeHandoffSnapshot(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	var body protocol.CollaborationHandoffSnapshotRequest
	if !decodeCollaborationRuntimeBodyLimit(w, r, &body, 4096) {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := svc.RuntimeHandoffSnapshot(r.Context(), run, token, body)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (h *Handler) BindCollaborationRuntimeHandoff(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	var body protocol.CollaborationHandoffBinding
	if !decodeCollaborationRuntimeBodyLimit(w, r, &body, 4096) {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := svc.BindRuntimeHandoff(r.Context(), run, token, body)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (h *Handler) GetCollaborationRunHandoff(w http.ResponseWriter, r *http.Request) {
	if !collaborationExecutionRead(w, r) {
		return
	}
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	run, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "run_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := svc.RunHandoff(r.Context(), issue, run, service.CollaborationHandoffQuery{Cursor: r.URL.Query().Get("cursor")})
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (h *Handler) GetCollaborationRuntimeHandoffSource(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	source := protocol.CollaborationHandoffSource{ID: chi.URLParam(r, "sourceId"), Kind: r.URL.Query().Get("kind"), RunID: r.URL.Query().Get("source_run_id"), Version: r.URL.Query().Get("version"), SHA256: r.URL.Query().Get("sha256")}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || strconv.Itoa(n) != v {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_handoff_offset"})
			return
		}
		offset = n
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	out, err := svc.RuntimeHandoffSource(r.Context(), run, token, source, offset)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
