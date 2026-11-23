package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (h *Handler) knowledgeService() *service.CollaborationService {
	return service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
}

func knowledgeRequestError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"code": code, "error": "Knowledge request unavailable"})
}
func decodeKnowledgeBody(w http.ResponseWriter, r *http.Request, value any) bool {
	if r.Body == nil {
		knowledgeRequestError(w, 400, "invalid_knowledge_request")
		return false
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, protocol.CollaborationKnowledgeMaxRequestBytes))
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		knowledgeRequestError(w, 413, "knowledge_candidate_too_large")
		return false
	}
	if err == nil {
		err = json.Unmarshal(raw, value)
	}
	if err == nil {
		return true
	}
	switch {
	case errors.Is(err, protocol.ErrCollaborationKnowledgeTooLarge):
		knowledgeRequestError(w, 413, "knowledge_candidate_too_large")
	case errors.Is(err, protocol.ErrCollaborationKnowledgeRevisionRequired):
		knowledgeRequestError(w, 428, "knowledge_revision_required")
	default:
		knowledgeRequestError(w, 400, "invalid_knowledge_request")
	}
	return false
}
func knowledgePageQuery(w http.ResponseWriter, r *http.Request) (service.KnowledgePageQuery, bool) {
	p := service.KnowledgePageQuery{Limit: 20}
	for key, values := range r.URL.Query() {
		if (key != "limit" && key != "cursor") || len(values) != 1 {
			knowledgeRequestError(w, 400, "invalid_knowledge_request")
			return p, false
		}
	}
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 50 {
			knowledgeRequestError(w, 400, "invalid_knowledge_request")
			return p, false
		}
		p.Limit = limit
	}
	p.Cursor = r.URL.Query().Get("cursor")
	if len(p.Cursor) > 2048 {
		knowledgeRequestError(w, 400, "invalid_knowledge_request")
		return p, false
	}
	return p, true
}
func writeKnowledgeReceipt(w http.ResponseWriter, r protocol.CollaborationKnowledgeReceipt, err error, created bool) {
	if err != nil && r.Code == "" {
		writeCollaborationError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	if err != nil {
		var e *service.CollaborationError
		if !errors.As(err, &e) {
			writeCollaborationError(w, err)
			return
		}
		status = e.Status
	}
	writeJSON(w, status, r)
}

func (h *Handler) ProposeCollaborationKnowledge(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	var p protocol.CollaborationKnowledgeProposal
	if !decodeKnowledgeBody(w, r, &p) {
		return
	}
	receipt, err := h.knowledgeService().ProposeKnowledge(r.Context(), id, p)
	writeKnowledgeReceipt(w, receipt, err, true)
}
func (h *Handler) ApproveCollaborationKnowledge(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "candidateId"), "candidate_id")
	if !ok {
		return
	}
	var p protocol.CollaborationKnowledgeApproveRequest
	if !decodeKnowledgeBody(w, r, &p) {
		return
	}
	receipt, err := h.knowledgeService().ApproveKnowledge(r.Context(), id, p)
	writeKnowledgeReceipt(w, receipt, err, false)
}
func (h *Handler) RejectCollaborationKnowledge(w http.ResponseWriter, r *http.Request) {
	h.decideCollaborationKnowledge(w, r, false)
}
func (h *Handler) WithdrawCollaborationKnowledge(w http.ResponseWriter, r *http.Request) {
	h.decideCollaborationKnowledge(w, r, true)
}
func (h *Handler) decideCollaborationKnowledge(w http.ResponseWriter, r *http.Request, withdraw bool) {
	prepareCollaborationContentRequest(w, r)
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "candidateId"), "candidate_id")
	if !ok {
		return
	}
	var p protocol.CollaborationKnowledgeDecisionRequest
	if !decodeKnowledgeBody(w, r, &p) {
		return
	}
	var receipt protocol.CollaborationKnowledgeReceipt
	var err error
	if withdraw {
		receipt, err = h.knowledgeService().WithdrawKnowledge(r.Context(), id, p)
	} else {
		receipt, err = h.knowledgeService().RejectKnowledge(r.Context(), id, p)
	}
	writeKnowledgeReceipt(w, receipt, err, false)
}
func (h *Handler) GetCollaborationKnowledgeOperation(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "candidateId"), "candidate_id")
	if !ok {
		return
	}
	receipt, err := h.knowledgeService().GetKnowledgeOperation(r.Context(), id, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, 200, receipt)
}
func (h *Handler) ListCollaborationKnowledgeCandidates(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	p, ok := knowledgePageQuery(w, r)
	if !ok {
		return
	}
	view, err := h.knowledgeService().ListKnowledgeCandidates(r.Context(), id, p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (h *Handler) GetCollaborationKnowledgeCandidate(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "candidateId"), "candidate_id")
	if !ok {
		return
	}
	view, err := h.knowledgeService().KnowledgeCandidate(r.Context(), id)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
func (h *Handler) GetProjectKnowledge(w http.ResponseWriter, r *http.Request) {
	h.getTargetKnowledge(w, r, "project")
}
func (h *Handler) GetWorkspaceKnowledge(w http.ResponseWriter, r *http.Request) {
	h.getTargetKnowledge(w, r, "workspace")
}
func (h *Handler) getTargetKnowledge(w http.ResponseWriter, r *http.Request, kind string) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "target_id")
	if !ok {
		return
	}
	workspace, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return
	}
	target := protocol.CollaborationKnowledgeTarget{Kind: kind, WorkspaceID: uuidToString(workspace)}
	if kind == "project" {
		target.ProjectID = uuidToString(id)
	} else if id != workspace {
		knowledgeRequestError(w, 404, "knowledge_target_not_found")
		return
	}
	p, ok := knowledgePageQuery(w, r)
	if !ok {
		return
	}
	view, err := h.knowledgeService().ListTargetKnowledge(r.Context(), target, p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, 200, view)
}
