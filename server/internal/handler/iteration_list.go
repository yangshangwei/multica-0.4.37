package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type iterationListCursor struct {
	WorkspaceID string `json:"workspace_id"`
	Version     string `json:"version"`
	Filter      string `json:"filter"`
	AfterDate   string `json:"after_date"`
	AfterID     string `json:"after_id"`
}

func (h *Handler) ListIterations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ws, actor, err := iterationWorkspaceScope(r)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	query := r.URL.Query()
	limit, err := historyPageLimit(query)
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	status := query.Get("status")
	switch status {
	case "", "planned", "active", "completed", "cancelled":
	default:
		writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid iteration status"))
		return
	}
	dates := make([]pgtype.Date, 2)
	for i, key := range []string{"from", "to"} {
		if raw := query.Get(key); raw != "" {
			value, e := iteration.ParseDate(raw)
			if e != nil {
				writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid calendar date"))
				return
			}
			dates[i] = pgtype.Date{Time: value, Valid: true}
		}
	}
	if dates[0].Valid && dates[1].Valid && dates[0].Time.After(dates[1].Time) {
		writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Date range is reversed"))
		return
	}
	filter := historyFilterKey(query, "iterations")
	var cursor iterationListCursor
	var afterDate pgtype.Date
	var afterID pgtype.UUID
	if raw := query.Get("cursor"); raw != "" {
		data, e := base64.RawURLEncoding.DecodeString(raw)
		if e != nil || len(data) > 2048 || json.Unmarshal(data, &cursor) != nil {
			writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid cursor"))
			return
		}
		day, e := iteration.ParseDate(cursor.AfterDate)
		if e != nil {
			writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid cursor date"))
			return
		}
		afterDate = pgtype.Date{Time: day, Valid: true}
		afterID, e = util.ParseUUID(cursor.AfterID)
		if e != nil || afterID.Bytes == [16]byte{} {
			writeIterationAPIError(w, iterationAPIError(400, "invalid_request", "Invalid cursor identity"))
			return
		}
	}
	items := []iteration.Iteration{}
	var next *string
	err = h.withIterationHistoryRead(r.Context(), ws, actor, func(tx pgx.Tx, q *db.Queries) error {
		version, e := q.GetWorkspaceIterationListVersion(r.Context(), ws)
		if e != nil {
			return e
		}
		if query.Get("cursor") != "" && (cursor.WorkspaceID != uuidToString(ws) || cursor.Filter != filter || cursor.Version != version) {
			return iterationAPIError(409, "cursor_stale", "Iteration list changed; start from the first page")
		}
		rows, e := q.ListWorkspaceIterations(r.Context(), db.ListWorkspaceIterationsParams{WorkspaceID: ws, Status: status, Search: strings.TrimSpace(query.Get("search")), FromDate: dates[0], ToDate: dates[1], HasAfter: afterID.Valid, AfterDate: afterDate, AfterID: afterID, PageLimit: int32(limit + 1)})
		if e != nil {
			return e
		}
		items = []iteration.Iteration{}
		next = nil
		count := len(rows)
		if count > limit {
			count = limit
		}
		for _, row := range rows[:count] {
			items = append(items, iteration.IterationFromRow(row))
		}
		if len(rows) > limit {
			last := rows[count-1]
			raw, e := json.Marshal(iterationListCursor{WorkspaceID: uuidToString(ws), Version: version, Filter: filter, AfterDate: last.StartDate.Time.Format(time.DateOnly), AfterID: uuidToString(last.ID)})
			if e != nil {
				return e
			}
			value := base64.RawURLEncoding.EncodeToString(raw)
			next = &value
		}
		return nil
	})
	if err != nil {
		writeIterationAPIError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"workspace_id": uuidToString(ws), "items": items, "next_cursor": next})
}
