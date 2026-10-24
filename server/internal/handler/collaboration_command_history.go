package handler

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) GetCollaborationCommandHistory(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	p := service.CollaborationCommandHistoryQuery{Limit: 50}
	// ParseQuery reports malformed escapes/semicolons that URL.Query discards.
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_command_history_query"})
		return
	}
	for key, values := range query {
		if len(values) != 1 || values[0] == "" || key != "limit" && key != "cursor" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_command_history_query"})
			return
		}
	}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 || strconv.Itoa(limit) != raw {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_command_history_query"})
			return
		}
		p.Limit = limit
	}
	p.Cursor = query.Get("cursor")
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	page, err := svc.CommandHistory(r.Context(), issue, p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
