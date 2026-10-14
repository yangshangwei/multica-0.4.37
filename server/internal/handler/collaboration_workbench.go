package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
)

type CollaborationWorkbenchItem struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Status          string    `json:"status"`
	StatusCategory  string    `json:"status_category"`
	Priority        string    `json:"priority"`
	Revision        string    `json:"collaboration_revision"`
	UpdatedAt       time.Time `json:"updated_at"`
	SourceWorkspace struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Identifier string `json:"identifier"`
	} `json:"source_workspace"`
	Handler CollaborationAccountSummary `json:"handler"`
}

type CollaborationWorkbenchResponse struct {
	Items      []CollaborationWorkbenchItem `json:"items"`
	Total      int64                        `json:"total"`
	NextCursor *string                      `json:"next_cursor"`
}

type collaborationWorkbenchCursor struct {
	UpdatedAt time.Time `json:"updated_at"`
	ID        string    `json:"id"`
}

func (h *Handler) GetCollaborationWorkbench(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	input := service.CollaborationWorkbenchQuery{View: r.URL.Query().Get("view"), Limit: 50}
	if input.View == "" {
		input.View = "current"
	}
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		input.Limit = limit
	}
	if value := r.URL.Query().Get("cursor"); value != "" {
		if len(value) > 512 {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		data, err := base64.RawURLEncoding.DecodeString(value)
		var cursor collaborationWorkbenchCursor
		if err != nil || json.Unmarshal(data, &cursor) != nil || cursor.UpdatedAt.IsZero() {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		id, err := util.ParseUUID(cursor.ID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		input.CursorID = id
		input.CursorUpdatedAt = pgtype.Timestamptz{Time: cursor.UpdatedAt, Valid: true}
	}
	r.Header.Del("X-Workspace-ID")
	r.Header.Del("X-Workspace-Slug")
	svc := service.NewCollaborationService(h.Queries, h.TxStarter, h.IssueService)
	page, err := svc.ListWorkbench(r.Context(), input)
	if err != nil {
		writeCollaborationError(w, err)
		return
	}
	out := CollaborationWorkbenchResponse{Items: make([]CollaborationWorkbenchItem, 0, len(page.Items)), Total: page.Total}
	for _, row := range page.Items {
		item := CollaborationWorkbenchItem{ID: uuidToString(row.ID), Title: row.Title, Status: row.Status, StatusCategory: row.StatusCategory,
			Priority: row.Priority, Revision: strconv.FormatInt(row.Revision, 10), UpdatedAt: row.UpdatedAt.Time,
			Handler: CollaborationAccountSummary{UserID: uuidToString(row.HandlerUserID), Name: row.HandlerName, AvatarURL: row.HandlerAvatarUrl, External: row.HandlerExternal}}
		item.SourceWorkspace.ID, item.SourceWorkspace.Name = uuidToString(row.SourceWorkspaceID), row.SourceName
		item.SourceWorkspace.Identifier = fmt.Sprintf("%s-%d", util.ResolveIssuePrefix(row.SourcePrefix, row.SourceName), row.Number)
		out.Items = append(out.Items, item)
	}
	if page.HasMore {
		last := out.Items[len(out.Items)-1]
		data, err := json.Marshal(collaborationWorkbenchCursor{UpdatedAt: last.UpdatedAt, ID: last.ID})
		if err != nil {
			writeCollaborationError(w, err)
			return
		}
		cursor := base64.RawURLEncoding.EncodeToString(data)
		out.NextCursor = &cursor
	}
	writeJSON(w, http.StatusOK, out)
}
