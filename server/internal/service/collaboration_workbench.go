package service

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationWorkbenchQuery struct {
	View            string
	Limit           int
	CursorUpdatedAt pgtype.Timestamptz
	CursorID        pgtype.UUID
}

type CollaborationWorkbenchPage struct {
	Items   []db.ListCollaborationWorkbenchRow
	Total   int64
	HasMore bool
}

func (s *CollaborationService) ListWorkbench(ctx context.Context, input CollaborationWorkbenchQuery) (CollaborationWorkbenchPage, error) {
	var out CollaborationWorkbenchPage
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return out, collaborationError(http.StatusForbidden, "forbidden")
	}
	if (input.View != "current" && input.View != "participating") || input.Limit < 1 || input.Limit > 100 {
		return out, collaborationError(http.StatusBadRequest, "invalid_workbench_query")
	}
	if input.CursorID.Valid != input.CursorUpdatedAt.Valid {
		return out, collaborationError(http.StatusBadRequest, "invalid_cursor")
	}
	viewer, err := util.ParseUUID(principal.UserID)
	if err != nil {
		return out, collaborationError(http.StatusForbidden, "forbidden")
	}
	user, err := s.Queries.GetUser(ctx, viewer)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, collaborationError(http.StatusForbidden, "forbidden")
	}
	if err != nil {
		return out, err
	}
	if user.DisabledAt.Valid || auth.IsTemporarilyDisabledUser(principal.UserID, user.Email.String) {
		return out, collaborationError(http.StatusForbidden, "forbidden")
	}
	var expected pgtype.Int8
	if auth.PasswordMode() {
		ps, ok := auth.PasswordSessionFromContext(ctx)
		if !ok || ps.UserID != principal.UserID {
			return out, collaborationError(http.StatusForbidden, "forbidden")
		}
		if _, err := auth.CheckPasswordVersion(ctx, s.Queries, principal.UserID, ps.Version); err != nil {
			return out, err
		}
		expected = pgtype.Int8{Int64: ps.Version, Valid: true}
	}
	rows, err := s.Queries.ListCollaborationWorkbench(ctx, db.ListCollaborationWorkbenchParams{
		ViewerID: viewer, ExpectedAuthVersion: expected, ViewKind: input.View,
		CursorUpdatedAt: input.CursorUpdatedAt, CursorID: input.CursorID, PageLimit: int32(input.Limit + 1),
	})
	if err != nil {
		return out, err
	}
	out.Items = make([]db.ListCollaborationWorkbenchRow, 0, input.Limit)
	for _, row := range rows {
		if !row.ViewerActive {
			return CollaborationWorkbenchPage{}, collaborationError(http.StatusForbidden, "forbidden")
		}
		out.Total = row.TotalCount
		if !row.ID.Valid {
			continue
		} // The count row is retained even for an empty page.
		if !row.SourceWorkspaceID.Valid || !row.HandlerUserID.Valid || !row.UpdatedAt.Valid || row.Revision < 1 {
			return CollaborationWorkbenchPage{}, collaborationError(http.StatusServiceUnavailable, "collaboration_unavailable")
		}
		if len(out.Items) == input.Limit {
			out.HasMore = true
			break
		}
		out.Items = append(out.Items, row)
	}
	return out, nil
}
