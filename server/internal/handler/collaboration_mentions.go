package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationCommentInputBody struct {
	Content             string   `json:"content"`
	ParentID            *string  `json:"parent_id"`
	SuppressedTargetIDs []string `json:"suppressed_target_ids"`
}

func collaborationCommentInput(w http.ResponseWriter, body collaborationCommentInputBody) (service.CollaborationCommentInput, bool) {
	input := service.CollaborationCommentInput{Content: body.Content, SuppressedTargetIDs: make([]pgtype.UUID, 0, len(body.SuppressedTargetIDs))}
	if body.ParentID != nil {
		id, ok := parseUUIDOrBadRequest(w, *body.ParentID, "parent_id")
		if !ok {
			return input, false
		}
		input.ParentID = id
	}
	for _, raw := range body.SuppressedTargetIDs {
		id, ok := parseUUIDOrBadRequest(w, raw, "suppressed_target_id")
		if !ok {
			return input, false
		}
		input.SuppressedTargetIDs = append(input.SuppressedTargetIDs, id)
	}
	return input, true
}

func (h *Handler) GetCollaborationMentionTargets(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.MentionTargets(r.Context(), issue)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) PreviewCollaborationComment(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	var body collaborationCommentInputBody
	if !decodeCollaborationWorkflowBody(w, r, &body) {
		return
	}
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	input, ok := collaborationCommentInput(w, body)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.PreviewComment(r.Context(), issue, input)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
