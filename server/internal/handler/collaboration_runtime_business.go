package handler

import (
	"net/http"

	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) GetCollaborationRuntimeContext(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.RuntimeContext(r.Context(), run, token)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) CreateCollaborationRuntimeComment(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	var body struct {
		CommentID string  `json:"comment_id"`
		ParentID  *string `json:"parent_id"`
		Content   string  `json:"content"`
	}
	if !decodeCollaborationRuntimeBody(w, r, &body) {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	p := service.CollaborationRuntimeCommentParams{Content: body.Content}
	p.CommentID, ok = parseUUIDOrBadRequest(w, body.CommentID, "comment_id")
	if !ok {
		return
	}
	if body.ParentID != nil {
		p.ParentID, ok = parseUUIDOrBadRequest(w, *body.ParentID, "parent_id")
		if !ok {
			return
		}
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	result, err := svc.CreateRuntimeComment(r.Context(), run, token, p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) RecordCollaborationRuntimeSquadActivity(w http.ResponseWriter, r *http.Request) {
	token, ok := collaborationNarrowBearer(w, r, service.CollaborationBusinessTokenPrefix)
	if !ok {
		return
	}
	var body struct {
		EventID string `json:"event_id"`
		Outcome string `json:"outcome"`
		Reason  string `json:"reason"`
	}
	if !decodeCollaborationRuntimeBody(w, r, &body) {
		return
	}
	run, ok := collaborationRuntimeRunID(w, r, nil)
	if !ok {
		return
	}
	if _, ok = parseUUIDOrBadRequest(w, body.EventID, "event_id"); !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	if err := svc.RuntimeSquadActivity(r.Context(), run, token, body.EventID, body.Outcome, body.Reason); err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"code": "squad_activity_recorded"})
}
