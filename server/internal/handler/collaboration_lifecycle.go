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
	collaborationResultBody
	Action       string  `json:"action"`
	SubmissionID *string `json:"submission_id"`
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
	if !decodeCollaborationWorkflowBodyLimit(w, r, &body, 1<<20) {
		return
	}
	request, ok := collaborationWorkflowRequest(w, r, body.collaborationWorkflowBody)
	if !ok {
		return
	}
	draft, attachments, comment, ok := collaborationResultSelection(w, body.collaborationResultBody)
	if !ok {
		return
	}
	p := service.CollaborationLifecycleParams{CollaborationWorkflowRequest: request,
		Action: body.Action, TargetStatus: body.Status, StatusOptionsVersion: body.StatusOptionsVersion,
		Note: body.Note, Basis: body.Basis, DraftID: draft, AttachmentIDs: attachments, SourceCommentID: comment}
	if body.SubmissionID != nil {
		id, valid := parseUUIDOrBadRequest(w, *body.SubmissionID, "submission_id")
		if !valid {
			return
		}
		p.SubmissionID = &id
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.ChangeLifecycle(r.Context(), p)
	if err == nil && receipt.ParentWakeup != nil {
		h.notifyParentOfChildDone(r.Context(), receipt.ParentWakeup.Before, receipt.ParentWakeup.After)
	}
	writeCollaborationWorkflowResponse(w, receipt, err)
}
