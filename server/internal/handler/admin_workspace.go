package handler

import (
	"context"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"net/http"
	"strings"
	"time"
)

func (h *Handler) AdminWorkspaces(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requirePlatformAccess(w, r, false)
	if !ok {
		return
	}
	p, err := ParseAdminListQuery(r, AdminListScope{Resource: "workspaces", ActorID: uuidToString(identity.UserID), OrganizationID: uuidToString(identity.OrganizationID), Window: 31 * 24 * time.Hour}, []string{"q"}, time.Now().UTC())
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	if len(p.Filters["q"]) > 128 || strings.ContainsRune(p.Filters["q"], 0) {
		adminServiceError(w, r, adminQueryError())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := h.Queries.ListAdminWorkspaces(ctx, db.ListAdminWorkspacesParams{OrganizationID: identity.OrganizationID, TimeFrom: adminTimestamp(p.From), TimeTo: adminTimestamp(p.To), AsOf: adminTimestamp(p.AsOf), Search: p.Filters["q"], AfterTime: adminOptionalTimestamp(p.AfterTime), AfterID: adminFilterID(p.AfterID), PageLimit: p.Limit + 1})
	if err != nil {
		adminServiceError(w, r, err)
		return
	}
	var cursor *string
	if len(rows) > int(p.Limit) {
		rows = rows[:p.Limit]
		last := rows[len(rows)-1]
		value, err := p.Cursor(last.CreatedAt.Time, uuidToString(last.ID))
		if err != nil {
			adminServiceError(w, r, err)
			return
		}
		cursor = &value
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": uuidToString(row.ID), "name": row.Name, "slug": row.Slug, "member_count": row.MemberCount, "execution_count": row.ExecutionCount, "created_at": timestampToString(row.CreatedAt)})
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": cursor, "scope": uuidToString(identity.OrganizationID), "as_of": p.AsOf.Format(time.RFC3339Nano), "data_quality": "complete", "window": adminWindow(p)})
}
