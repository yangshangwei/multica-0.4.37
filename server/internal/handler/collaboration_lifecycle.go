package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationStatusBody struct {
	collaborationWorkflowBody
	Status               string `json:"status"`
	StatusOptionsVersion string `json:"status_options_version"`
}

type collaborationLifecycleBody struct {
	collaborationStatusBody
	Action string `json:"action"`
}

func (h *Handler) GetCollaborationStatusOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	options, err := svc.StatusOptions(r.Context(), id, r.URL.Query().Get("action"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, options)
}

func (h *Handler) ChangeCollaborationStatus(w http.ResponseWriter, r *http.Request) {
	h.UpdateCollaborationIssue(w, r)
}

func (h *Handler) ChangeCollaborationLifecycle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	var body collaborationLifecycleBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	request, ok := collaborationWorkflowRequest(w, r, body.collaborationWorkflowBody)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.ChangeLifecycle(r.Context(), service.CollaborationLifecycleParams{CollaborationWorkflowRequest: request,
		Action: body.Action, TargetStatus: body.Status, StatusOptionsVersion: body.StatusOptionsVersion})
	if err == nil && receipt.ParentWakeup != nil {
		h.notifyParentOfChildDone(r.Context(), receipt.ParentWakeup.Before, receipt.ParentWakeup.After)
	}
	writeCollaborationWorkflowResponse(w, receipt, err)
}
