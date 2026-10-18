package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) ListCollaborationNotifications(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	limit, before, beforeID, ok := collaborationContentPagination(w, r, 50)
	if !ok {
		return
	}
	includeArchived := false
	if value := r.URL.Query().Get("include_archived"); value != "" {
		var err error
		includeArchived, err = strconv.ParseBool(value)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_notification_query", "error": "Invalid archive filter"})
			return
		}
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	page, err := svc.ListNotifications(r.Context(), service.CollaborationNotificationsQuery{
		Limit: limit, IncludeArchived: includeArchived, BeforeAt: before, BeforeID: beforeID})
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	var next *string
	if page.HasMore && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		next, err = collaborationNextContentCursor(last.CreatedAt, last.ID)
		if err != nil {
			writeCollaborationError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "unread_count": page.UnreadCount, "next_cursor": next})
}

func (h *Handler) ReadCollaborationNotification(w http.ResponseWriter, r *http.Request) {
	h.setCollaborationNotificationState(w, r, "read")
}

func (h *Handler) ArchiveCollaborationNotification(w http.ResponseWriter, r *http.Request) {
	h.setCollaborationNotificationState(w, r, "archive")
}

func (h *Handler) setCollaborationNotificationState(w http.ResponseWriter, r *http.Request, action string) {
	prepareCollaborationContentRequest(w, r)
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "notificationId"), "notification_id")
	if !ok {
		return
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	receipt, err := svc.SetNotificationState(r.Context(), id, action)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}
