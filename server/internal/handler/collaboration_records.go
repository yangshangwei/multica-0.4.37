package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) GetCollaborationRunRecords(w http.ResponseWriter, r *http.Request) {
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
	p := service.CollaborationRunRecordsQuery{Limit: 50}
	for key, values := range r.URL.Query() {
		if len(values) != 1 || values[0] == "" || key != "limit" && key != "cursor" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_records_query"})
			return
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 || strconv.Itoa(limit) != raw {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_records_query"})
			return
		}
		p.Limit = limit
	}
	p.Cursor = r.URL.Query().Get("cursor")
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	page, err := svc.RunRecords(r.Context(), issue, run, p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
