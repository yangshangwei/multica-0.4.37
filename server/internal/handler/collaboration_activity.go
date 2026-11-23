package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/service"
)

type collaborationActivityCursor struct {
	IssueID   pgtype.UUID `json:"issue_id"`
	BeforeSeq string      `json:"before_seq"`
}

func (h *Handler) ListCollaborationActivity(w http.ResponseWriter, r *http.Request) {
	prepareCollaborationContentRequest(w, r)
	issue, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue_id")
	if !ok {
		return
	}
	p := service.CollaborationActivityQuery{Limit: 50}
	for key, values := range r.URL.Query() {
		if len(values) != 1 || key != "limit" && key != "cursor" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_activity_query"})
			return
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_limit"})
			return
		}
		p.Limit = limit
	}
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		data, err := base64.RawURLEncoding.DecodeString(raw)
		var cursor collaborationActivityCursor
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if len(raw) > 512 || err != nil || iteration.ValidateObjectJSON(data) != nil || decoder.Decode(&cursor) != nil || cursor.IssueID != issue {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_cursor"})
			return
		}
		seq, valid := collaborationPositiveVersion(cursor.BeforeSeq)
		if !valid {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": "invalid_cursor"})
			return
		}
		p.BeforeSeq = pgtype.Int8{Int64: seq, Valid: true}
	}
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	page, err := svc.ListActivity(r.Context(), issue, p)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	var next *string
	if page.HasMore && len(page.Items) > 0 {
		data, err := json.Marshal(collaborationActivityCursor{IssueID: issue, BeforeSeq: strconv.FormatInt(page.Items[len(page.Items)-1].EventSeq, 10)})
		if err != nil {
			writeCollaborationError(w, err)
			return
		}
		encoded := base64.RawURLEncoding.EncodeToString(data)
		next = &encoded
	}
	writeJSON(w, http.StatusOK, map[string]any{"issue_id": issue, "items": page.Items, "next_cursor": next})
}
