package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) ResolveCollaborationComment(w http.ResponseWriter, r *http.Request) {
	h.changeCollaborationDiscussion(w, r, "resolve", true)
}
func (h *Handler) UnresolveCollaborationComment(w http.ResponseWriter, r *http.Request) {
	h.changeCollaborationDiscussion(w, r, "unresolve", true)
}
func (h *Handler) AddCollaborationCommentReaction(w http.ResponseWriter, r *http.Request) {
	h.changeCollaborationDiscussion(w, r, "add_reaction", true)
}
func (h *Handler) RemoveCollaborationCommentReaction(w http.ResponseWriter, r *http.Request) {
	h.changeCollaborationDiscussion(w, r, "remove_reaction", true)
}
func (h *Handler) AddCollaborationIssueReaction(w http.ResponseWriter, r *http.Request) {
	h.changeCollaborationDiscussion(w, r, "add_reaction", false)
}
func (h *Handler) RemoveCollaborationIssueReaction(w http.ResponseWriter, r *http.Request) {
	h.changeCollaborationDiscussion(w, r, "remove_reaction", false)
}

func (h *Handler) changeCollaborationDiscussion(w http.ResponseWriter, r *http.Request, action string, comment bool) {
	prepareCollaborationContentRequest(w, r)
	issueID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	p := service.CollaborationDiscussionParams{IssueID: issueID, RequestID: r.Header.Get("Idempotency-Key"), Action: action, Emoji: chi.URLParam(r, "emoji")}
	if comment {
		p.CommentID, ok = parseUUIDOrBadRequest(w, chi.URLParam(r, "commentId"), "comment_id")
		if !ok {
			return
		}
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.ChangeDiscussion(r.Context(), p)
	if err != nil && receipt.Code == "" {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, receipt.Status, receipt)
}

func (h *Handler) ListCollaborationIssueReactions(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issueID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	items, err := svc.ListIssueReactions(r.Context(), issueID)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"issue_id": issueID, "items": items})
}

func (h *Handler) GetCollaborationDiscussionOperation(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issueID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.GetDiscussionOperation(r.Context(), issueID, chi.URLParam(r, "requestId"))
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}
