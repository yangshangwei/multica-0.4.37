package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
)

func prepareKnowledgeAutomationRequest(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if !prepareKnowledgeProtocol(w, r) {
		return false
	}
	version, _ := knowledgeRequestProtocol(w, r)
	if version != 2 {
		knowledgeRequestError(w, 409, "knowledge_protocol_unsupported")
		return false
	}
	if r.URL.RawQuery != "" {
		knowledgeRequestError(w, 400, "invalid_knowledge_request")
		return false
	}
	return true
}

func (h *Handler) knowledgePolicyTarget(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, bool) {
	project, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project_id")
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	workspace, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	return project, workspace, ok
}

func (h *Handler) GetProjectKnowledgePolicy(w http.ResponseWriter, r *http.Request) {
	if !prepareKnowledgeAutomationRequest(w, r) {
		return
	}
	project, workspace, ok := h.knowledgePolicyTarget(w, r)
	if !ok {
		return
	}
	view, err := h.knowledgeService().GetProjectKnowledgePolicy(r.Context(), project, workspace)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, 200, view)
}

func (h *Handler) UpdateProjectKnowledgePolicy(w http.ResponseWriter, r *http.Request) {
	if !prepareKnowledgeAutomationRequest(w, r) {
		return
	}
	project, workspace, ok := h.knowledgePolicyTarget(w, r)
	if !ok {
		return
	}
	var body struct {
		RequestID        string `json:"request_id"`
		ExpectedRevision string `json:"expected_revision"`
		IsEnabled        *bool  `json:"is_enabled"`
		MaxCandidates    *int   `json:"max_candidates"`
	}
	if r.Body == nil {
		knowledgeRequestError(w, 400, "invalid_knowledge_request")
		return
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	if body.IsEnabled == nil || body.MaxCandidates == nil {
		knowledgeRequestError(w, 400, "invalid_knowledge_request")
		return
	}
	receipt, err := h.knowledgeService().UpdateProjectKnowledgePolicy(r.Context(), project, workspace, service.ProjectKnowledgePolicyRequest{
		RequestID: body.RequestID, ExpectedRevision: body.ExpectedRevision, IsEnabled: *body.IsEnabled, MaxCandidates: *body.MaxCandidates,
	})
	if err != nil {
		var domain *service.CollaborationError
		if receipt.Code != "" && errors.As(err, &domain) {
			writeJSON(w, domain.Status, receipt)
		} else {
			writeCollaborationError(w, err)
		}
		return
	}
	writeJSON(w, 200, receipt)
}

// Original receipts are account-scoped. Resolving the current project or
// workspace here would strand a committed request after role loss/deletion.
func (h *Handler) GetProjectKnowledgePolicyOperation(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	if !prepareKnowledgeAutomationRequest(w, r) {
		return
	}
	project, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project_id")
	if !ok {
		return
	}
	receipt, err := h.knowledgeService().GetProjectKnowledgePolicyOperation(r.Context(), project, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, 200, receipt)
}

func (h *Handler) ListCollaborationKnowledgeJobs(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	if !prepareKnowledgeAutomationRequest(w, r) {
		return
	}
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	view, err := h.knowledgeService().ListKnowledgeAutomationJobs(r.Context(), issue)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, 200, view)
}

func (h *Handler) RetryCollaborationKnowledgeJob(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	if !prepareKnowledgeAutomationRequest(w, r) {
		return
	}
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	job, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "jobId"), "job_id")
	if !ok {
		return
	}
	var body service.KnowledgeAutomationRetryRequest
	if r.Body == nil {
		knowledgeRequestError(w, 400, "invalid_knowledge_request")
		return
	}
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	receipt, err := h.knowledgeService().RetryKnowledgeAutomationJob(r.Context(), issue, job, body)
	if err != nil {
		var domain *service.CollaborationError
		if receipt.Code != "" && errors.As(err, &domain) {
			writeJSON(w, domain.Status, receipt)
		} else {
			writeCollaborationError(w, err)
		}
		return
	}
	writeJSON(w, 200, receipt)
}

func (h *Handler) GetCollaborationKnowledgeJobOperation(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	if !prepareKnowledgeAutomationRequest(w, r) {
		return
	}
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	job, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "jobId"), "job_id")
	if !ok {
		return
	}
	receipt, err := h.knowledgeService().GetKnowledgeAutomationRetryOperation(r.Context(), issue, job, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, 200, receipt)
}
