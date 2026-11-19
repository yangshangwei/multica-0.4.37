package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationCreationSource struct {
	ID   pgtype.UUID `json:"id"`
	Name string      `json:"name"`
	Slug string      `json:"slug"`
}

type CollaborationCreationOptions struct {
	CreationEnabled        bool                                         `json:"creation_enabled"`
	Sources                []CollaborationCreationSource                `json:"sources"`
	Source                 *CollaborationCreationSource                 `json:"source"`
	MaxAttachmentBytes     int64                                        `json:"max_attachment_bytes"`
	Fields                 *CollaborationCreationFields                 `json:"fields,omitempty"`
	StagePolicyObservation *CollaborationCreationStagePolicyObservation `json:"stage_policy_observation,omitempty"`
}

type CollaborationCreationStagePolicyObservation struct {
	ProtocolVersion   int         `json:"protocol_version"`
	State             string      `json:"state"`
	SourceWorkspaceID pgtype.UUID `json:"source_workspace_id"`
	ProjectID         pgtype.UUID `json:"project_id"`
	PolicyRevision    *string     `json:"policy_revision"`
	Managed           *bool       `json:"managed"`
}

// CreationOptions returns only the caller's source memberships. Closing the
// gate remains observable without granting any cross-workspace discovery.
func (s *CollaborationService) CreationOptions(ctx context.Context, source pgtype.UUID, parentSearch ...string) (CollaborationCreationOptions, error) {
	return s.creationOptions(ctx, source, pgtype.UUID{}, parentSearch...)
}

func (s *CollaborationService) CreationOptionsForProject(ctx context.Context, source, project pgtype.UUID, parentSearch string) (CollaborationCreationOptions, error) {
	return s.creationOptions(ctx, source, project, parentSearch)
}

func (s *CollaborationService) creationOptions(ctx context.Context, source, project pgtype.UUID, parentSearch ...string) (CollaborationCreationOptions, error) {
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
	if project.Valid && !source.Valid {
		return out, collaborationError(http.StatusBadRequest, "invalid_request")
	}
	out.StagePolicyObservation = &CollaborationCreationStagePolicyObservation{ProtocolVersion: 1, State: "not_selected", SourceWorkspaceID: source}
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
	if project.Valid {
		_, err := q.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: project, WorkspaceID: source})
		if errors.Is(err, pgx.ErrNoRows) {
			return out, collaborationError(http.StatusNotFound, "not_found")
		}
		if err != nil {
			return out, err
		}
		policy, err := informationPolicyFor(ctx, q, db.Issue{WorkspaceID: source, ProjectID: project}, false)
		if err != nil {
			return out, err
		}
		revision := strconv.FormatInt(policy.Revision, 10)
		out.StagePolicyObservation = &CollaborationCreationStagePolicyObservation{ProtocolVersion: 1, State: "known", SourceWorkspaceID: source,
			ProjectID: project, PolicyRevision: &revision, Managed: &policy.Managed}
		if policy.Managed && out.Fields != nil {
			statuses := make([]CollaborationStatusOption, 0, len(out.Fields.Statuses))
			for _, status := range out.Fields.Statuses {
				if status.Category != "blocked" {
					statuses = append(statuses, status)
				}
			}
			out.Fields.Statuses = statuses
			foundDefault := false
			for _, status := range statuses {
				foundDefault = foundDefault || status.Key == out.Fields.DefaultStatus
			}
			if !foundDefault {
				if len(statuses) == 0 {
					return out, collaborationError(http.StatusServiceUnavailable, "information_context_unavailable")
				}
				out.Fields.DefaultStatus = statuses[0].Key
			}
		}
	}
	return out, tx.Commit(ctx)
}
