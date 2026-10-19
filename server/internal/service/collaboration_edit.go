package service

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// CollaborationEditParams preserves the ordinary text/date PATCH semantics.
// Date presence is separate because null explicitly clears a calendar date.
type CollaborationEditParams struct {
	CollaborationWorkflowRequest
	Title        *string
	Description  *string
	StartDate    *string
	DueDate      *string
	SetStartDate bool
	SetDueDate   bool
	// Omit absent new fields from the digest so pre-R42 metadata operations
	// retain their original payload identity. A present empty label set clears.
	Priority      *string                    `json:"Priority,omitempty"`
	LabelIDs      *[]pgtype.UUID             `json:"LabelIDs,omitempty"`
	Properties    map[string]json.RawMessage `json:"Properties,omitempty"`
	ProjectID     *pgtype.UUID               `json:"ProjectID,omitempty"`
	ParentIssueID *pgtype.UUID               `json:"ParentIssueID,omitempty"`
	Stage         *pgtype.Int4               `json:"Stage,omitempty"`
}

func (s *CollaborationService) EditIssue(ctx context.Context, p CollaborationEditParams) (CollaborationWorkflowReceipt, error) {
	kind := "edit_issue"
	if p.ProjectID != nil || p.ParentIssueID != nil || p.Stage != nil {
		kind = "edit_source_structure"
	}
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, kind, p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		before := t.access.Issue
		after := before
		refusal, err := t.prepareSourceStructure(ctx, p, &after)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if refusal != "" {
			return t.result(refusal, http.StatusBadRequest), nil
		}
		if p.Title != nil {
			after.Title = util.SanitizeTextForPostgres(*p.Title)
		}
		if p.Description != nil {
			after.Description = pgtype.Text{String: util.SanitizeTextForPostgres(*p.Description), Valid: true}
		}
		for _, field := range []struct {
			name    string
			present bool
			input   *string
			value   *pgtype.Date
		}{
			{"start_date", p.SetStartDate, p.StartDate, &after.StartDate},
			{"due_date", p.SetDueDate, p.DueDate, &after.DueDate},
		} {
			if !field.present {
				if field.input != nil {
					return t.result("invalid_edit_request", http.StatusBadRequest), nil
				}
				continue
			}
			*field.value = pgtype.Date{}
			if field.input != nil && *field.input != "" {
				value, err := util.ParseCalendarDate(*field.input)
				if err != nil {
					// The shared parser's error includes the input; keep private
					// values out of HTTP errors and durable operation receipts.
					return t.result("invalid_"+field.name, http.StatusBadRequest), nil
				}
				*field.value = value
			}
		}
		startUnchanged := before.StartDate.Valid == after.StartDate.Valid && (!after.StartDate.Valid || before.StartDate.Time.Equal(after.StartDate.Time))
		dueUnchanged := before.DueDate.Valid == after.DueDate.Valid && (!after.DueDate.Valid || before.DueDate.Time.Equal(after.DueDate.Time))
		fields, refusal, err := t.prepareFieldEdit(ctx, p, &after)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if refusal != "" {
			return t.result(refusal, http.StatusBadRequest), nil
		}
		structureChanged := before.ProjectID != after.ProjectID || before.ParentIssueID != after.ParentIssueID || before.Stage != after.Stage
		if before.Title == after.Title && before.Description == after.Description && startUnchanged && dueUnchanged && !structureChanged &&
			before.Priority == after.Priority && !fields.labelsChanged && !fields.propertiesChanged {
			return t.result("unchanged", http.StatusOK), nil
		}

		record, err := iteration.PrepareIssueRecord(ctx, t.tx, before, t.iteration)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		structure := after
		after, err = t.q.UpdateCollaborationIssueFields(ctx, db.UpdateCollaborationIssueFieldsParams{
			IssueID: before.ID, WorkspaceID: before.WorkspaceID, Title: after.Title, Description: after.Description,
			StartDate: after.StartDate, DueDate: after.DueDate,
			Priority: after.Priority, Properties: after.Properties,
		})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if structureChanged {
			after, err = t.q.UpdateCollaborationIssueStructure(ctx, db.UpdateCollaborationIssueStructureParams{
				IssueID: before.ID, WorkspaceID: before.WorkspaceID, ProjectID: structure.ProjectID, ParentIssueID: structure.ParentIssueID, Stage: structure.Stage,
			})
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
		}
		if fields.labelsChanged {
			if err = t.replaceFieldLabels(ctx, fields.afterLabels); err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
		}
		actor, err := json.Marshal(map[string]any{"type": "member", "id": t.actor, "user_id": t.actor})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if err = iteration.RecordIssueChange(ctx, t.tx, record, after, actor, t.operation.ID); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.access.Issue = after
		t.access.Revision, err = t.q.AdvanceCollaborationEditRevision(ctx, before.ID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		receipt := t.result("issue_edited", http.StatusOK)
		seq, err := t.q.AdvanceCollaborationEventSequence(ctx, before.ID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		payload, err := json.Marshal(map[string]any{
			"before": collaborationEditedValues(before, fields.beforeLabels), "after": collaborationEditedValues(after, fields.afterLabels),
			"edited_at": util.TimestampToNanoPtr(after.UpdatedAt), "receipt": receipt, "request_id": t.operation.RequestID,
		})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		// Ordinary metadata edits remain silent. Only newly added human body
		// mentions produce a located notification in this same transaction.
		event, err := t.q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{
			IssueID: before.ID, EventSeq: seq, ActorUserID: t.actor, Kind: "issue_edited", Payload: payload,
		})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		return receipt, notifyAddedCollaborationMentions(ctx, t.q, t.access, t.actor, event.ID, seq, "body", before.Description.String, after.Description.String)
	})
}

func collaborationEditedValues(issue db.Issue, labels []db.IssueLabel) map[string]any {
	return map[string]any{"title": issue.Title, "description": util.TextToPtr(issue.Description),
		"start_date": util.DateToPtr(issue.StartDate), "due_date": util.DateToPtr(issue.DueDate),
		"priority": issue.Priority, "labels": collaborationFieldLabels(labels), "properties": json.RawMessage(issue.Properties),
		"project_id": util.UUIDToPtr(issue.ProjectID), "parent_issue_id": util.UUIDToPtr(issue.ParentIssueID), "stage": issue.Stage}
}
