package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationCreationFields struct {
	Labels        []CollaborationFieldLabel         `json:"labels"`
	Properties    []CollaborationPropertyDefinition `json:"properties"`
	Statuses      []CollaborationStatusOption       `json:"statuses"`
	DefaultStatus string                            `json:"default_status"`
	SourceOptions *CollaborationSourceOptions       `json:"source_options"`
}

func collaborationCreationFields(ctx context.Context, q *db.Queries, workspace pgtype.UUID, search string) (*CollaborationCreationFields, error) {
	out := &CollaborationCreationFields{Labels: []CollaborationFieldLabel{}, Properties: []CollaborationPropertyDefinition{}, Statuses: []CollaborationStatusOption{}, DefaultStatus: "todo"}
	labels, err := q.ListCollaborationFieldLabels(ctx, workspace)
	if err != nil {
		return nil, err
	}
	for _, label := range labels {
		out.Labels = append(out.Labels, CollaborationFieldLabel{ID: util.UUIDToString(label.ID), Name: label.Name, Color: label.Color})
	}
	definitions, err := q.ListCollaborationFieldProperties(ctx, db.ListCollaborationFieldPropertiesParams{WorkspaceID: workspace})
	if err != nil {
		return nil, err
	}
	for _, definition := range definitions {
		value, err := collaborationPropertyDefinition(definition)
		if err != nil {
			return nil, err
		}
		out.Properties = append(out.Properties, value)
	}
	statuses, err := q.ListIssueStatusEntries(ctx, db.ListIssueStatusEntriesParams{WorkspaceID: workspace, IncludeArchived: true})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, status := range statuses {
		seen[status.Key] = true
		if !status.ArchivedAt.Valid && collaboration.NonterminalCategory(status.Category) {
			out.Statuses = append(out.Statuses, CollaborationStatusOption{Key: status.Key, Name: status.Name, Category: status.Category})
		}
	}
	for _, key := range issuestatus.Canonical() {
		if !seen[key] && collaboration.NonterminalCategory(key) {
			status, err := issuestatus.Resolve(ctx, q, workspace, key)
			if err != nil {
				return nil, err
			}
			out.Statuses = append(out.Statuses, CollaborationStatusOption{Key: status.Key, Name: status.Name, Category: status.Category})
		}
	}
	out.SourceOptions, err = collaborationSourceOptions(ctx, q, workspace, pgtype.UUID{}, search)
	return out, err
}

// All initial values are validated before creation, under the same catalog/I1
// fences. Initial membership locks precede the new issue and private files.
func (s *CollaborationService) prepareCollaborationCreationFields(ctx context.Context, tx pgx.Tx, p CollaborationCreateParams) (db.Issue, db.Iteration, error) {
	q := s.Queries.WithTx(tx)
	initial := db.Issue{WorkspaceID: p.SourceWorkspaceID, Status: "todo", Priority: "none", Properties: []byte("{}")}
	var target db.Iteration
	if err := q.LockIssueStatusCatalogShared(ctx, p.SourceWorkspaceID); err != nil {
		return initial, target, err
	}
	if err := iteration.LockWorkspace(ctx, tx, p.SourceWorkspaceID); err != nil {
		return initial, target, err
	}
	if p.Status != nil {
		initial.Status = *p.Status
	}
	status, err := issuestatus.Resolve(ctx, q, p.SourceWorkspaceID, initial.Status)
	if errors.Is(err, issuestatus.ErrUnknownStatus) || err == nil && !collaboration.NonterminalCategory(status.Category) {
		return initial, target, collaborationError(409, "status_unavailable")
	}
	if err != nil {
		return initial, target, err
	}
	initial.Status = status.Key
	for _, date := range []struct {
		input  *string
		target *pgtype.Date
		code   string
	}{{p.StartDate, &initial.StartDate, "invalid_start_date"}, {p.DueDate, &initial.DueDate, "invalid_due_date"}} {
		if date.input == nil || *date.input == "" {
			continue
		}
		*date.target, err = util.ParseCalendarDate(*date.input)
		if err != nil {
			return initial, target, collaborationError(400, date.code)
		}
	}
	if p.Stage != nil {
		if p.Stage.Valid && p.Stage.Int32 < 1 {
			return initial, target, collaborationError(400, "invalid_stage")
		}
		initial.Stage = *p.Stage
	}
	if p.CurrentIterationID != nil && p.CurrentIterationID.Valid {
		if err = q.EnsureIterationSettings(ctx, p.SourceWorkspaceID); err != nil {
			return initial, target, err
		}
		settings, err := q.LockIterationSettings(ctx, p.SourceWorkspaceID)
		if err != nil {
			return initial, target, err
		}
		if !settings.Enabled {
			return initial, target, collaborationError(409, "iteration_disabled")
		}
		rows, err := q.LockIterations(ctx, db.LockIterationsParams{WorkspaceID: p.SourceWorkspaceID, Column2: []pgtype.UUID{*p.CurrentIterationID}})
		if err != nil {
			return initial, target, err
		}
		if len(rows) != 1 {
			return initial, target, collaborationError(409, "iteration_not_found")
		}
		target = rows[0]
		if target.Revision != p.ExpectedIterationRevision {
			return initial, target, collaborationError(409, "iteration_revision_conflict")
		}
		if target.Status != "active" && target.Status != "planned" {
			return initial, target, collaborationError(409, "iteration_history_move_unsupported")
		}
	} else if p.ExpectedIterationRevision != 0 {
		return initial, target, collaborationError(400, "invalid_request")
	}
	w := &collaborationWorkflowTx{collaborationQueueTx: &collaborationQueueTx{q: q, access: CollaborationIssueAccess{Issue: initial}}, tx: tx}
	fieldParams := CollaborationEditParams{Priority: p.Priority, Properties: p.Properties}
	if p.LabelIDs != nil {
		fieldParams.LabelIDs = &p.LabelIDs
	}
	_, refusal, err := w.prepareFieldEdit(ctx, fieldParams, &initial)
	if err != nil {
		return initial, target, err
	}
	if refusal != "" {
		return initial, target, collaborationError(400, refusal)
	}
	return initial, target, nil
}

func (s *CollaborationService) finishCollaborationCreationFields(ctx context.Context, tx pgx.Tx, created db.Issue, properties []byte, target db.Iteration, actorID pgtype.UUID) (db.Issue, error) {
	q := s.Queries.WithTx(tx)
	if string(properties) != "{}" {
		var err error
		created, err = q.UpdateCollaborationIssueFields(ctx, db.UpdateCollaborationIssueFieldsParams{IssueID: created.ID, WorkspaceID: created.WorkspaceID, Title: created.Title,
			Description: created.Description, StartDate: created.StartDate, DueDate: created.DueDate, Priority: created.Priority, Properties: properties})
		if err != nil {
			return db.Issue{}, err
		}
	}
	if target.ID.Valid {
		change, err := iteration.PrepareMembershipChange(ctx, tx, created, db.Iteration{}, target, false)
		if err != nil {
			return db.Issue{}, err
		}
		actor, err := json.Marshal(map[string]any{"type": "member", "id": actorID, "user_id": actorID})
		if err != nil {
			return db.Issue{}, err
		}
		sample, err := iteration.SampleBusinessTime(ctx, tx, nil)
		if err != nil {
			return db.Issue{}, err
		}
		created, err = iteration.CommitMembershipChange(ctx, tx, change, actor, pgtype.UUID{Bytes: uuid.New(), Valid: true}, sample)
		if err != nil {
			return db.Issue{}, err
		}
	}
	return created, nil
}
