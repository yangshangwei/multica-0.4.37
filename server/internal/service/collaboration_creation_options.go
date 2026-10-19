package service

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
)

type CollaborationCreationSource struct {
	ID   pgtype.UUID `json:"id"`
	Name string      `json:"name"`
	Slug string      `json:"slug"`
}

type CollaborationCreationOptions struct {
	CreationEnabled    bool                          `json:"creation_enabled"`
	Sources            []CollaborationCreationSource `json:"sources"`
	Source             *CollaborationCreationSource  `json:"source"`
	MaxAttachmentBytes int64                         `json:"max_attachment_bytes"`
	Fields             *CollaborationCreationFields  `json:"fields,omitempty"`
}

// CreationOptions returns only the caller's source memberships. Closing the
// gate remains observable without granting any cross-workspace discovery.
func (s *CollaborationService) CreationOptions(ctx context.Context, source pgtype.UUID, parentSearch ...string) (CollaborationCreationOptions, error) {
	out := CollaborationCreationOptions{Sources: make([]CollaborationCreationSource, 0), MaxAttachmentBytes: CollaborationAttachmentMaxBytes}
	if p, ok := collaboration.PrincipalFromContext(ctx); !ok || !p.Human() {
		return out, collaborationError(http.StatusForbidden, "forbidden")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"); err != nil {
		return out, err
	}
	q := s.Queries.WithTx(tx)
	actor, err := collaborationNotificationViewer(ctx, q)
	if err != nil {
		return out, err
	}
	rows, err := q.ListCollaborationCreationSources(ctx, actor)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		item := CollaborationCreationSource{ID: row.ID, Name: row.Name, Slug: row.Slug}
		out.Sources = append(out.Sources, item)
		if row.ID == source {
			out.Source = &item
		}
	}
	if source.Valid && out.Source == nil {
		return out, collaborationError(http.StatusForbidden, "forbidden")
	}
	gate, err := q.GetCollaborationGate(ctx)
	if err != nil {
		return out, err
	}
	out.CreationEnabled = gate.CreationEnabled
	if source.Valid {
		search, err := collaborationParentSearch(parentSearch)
		if err != nil {
			return out, err
		}
		out.Fields, err = collaborationCreationFields(ctx, q, source, search)
		if err != nil {
			return out, err
		}
	}
	return out, tx.Commit(ctx)
}
