package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (s *IterationService) Apply(ctx context.Context, ws, actor pgtype.UUID, input ApplyIterationInput, authorize func(context.Context, pgx.Tx) error) (iteration.WriteResult, error) {
	var err error
	input.Draft, err = normalizeLifecycleDraft(input.Draft)
	if err != nil {
		return iteration.WriteResult{}, err
	}
	if len(input.PreviewHash) != 64 {
		return iteration.WriteResult{}, iterationFailure(428, "iteration_confirmation_required", "A complete preview confirmation is required")
	}
	return s.lifecycleOperation(ctx, ws, actor, input.RequestID, input.Draft.Operation, input, authorize, func(ctx context.Context, tx pgx.Tx, operationID pgtype.UUID) (iteration.WriteResult, error) {
		p, err := s.prepareLifecycle(ctx, tx, ws, actor, input.Draft, true)
		if err != nil {
			return iteration.WriteResult{}, err
		}
		if p.preview.PreviewHash != input.PreviewHash {
			return iteration.WriteResult{}, iterationFailure(409, "iteration_preview_stale", "Preview facts changed; refresh the complete preview")
		}
		if len(p.preview.InvalidItems) > 0 {
			code := p.preview.InvalidItems[0].Code
			status := 422
			switch code {
			case "iteration_active_conflict", "iteration_history_move_unsupported", "triage_review_required":
				status = 409
			case "iteration_revision_conflict":
				status = 409
				code = "iteration_preview_stale"
			default:
				code = "iteration_validation_failed"
			}
			return iteration.WriteResult{}, iterationFailure(status, code, "Preview contains invalid items")
		}
		q := db.New(tx)
		sample := pgtype.Timestamptz{Time: p.sampled, Valid: true}
		for _, change := range p.changes {
			if _, err = iteration.CommitMembershipChange(ctx, tx, change, iterationActor(actor), operationID, p.sampled); err != nil {
				return iteration.WriteResult{}, err
			}
		}
		var iid pgtype.UUID
		if input.Draft.IterationID != nil {
			iid, _ = iterationUUID(input.Draft.IterationID)
		}
		deleted := false
		switch input.Draft.Operation {
		case "start":
			dates := p.preview.StartPreview
			first, last, err := iterationDates(dates.EffectiveStartDate, dates.EffectiveEndDate)
			if err != nil {
				return iteration.WriteResult{}, err
			}
			if _, err = q.StartIteration(ctx, db.StartIterationParams{WorkspaceID: ws, ID: iid, StartDate: first, EndDate: last, StartedBy: actor, BusinessAt: sample}); err != nil {
				return iteration.WriteResult{}, err
			}
			if err = appendLifecycleEvent(ctx, tx, ws, iid, actor, operationID, "start", nil, dates, input.Draft.Reason, p.sampled); err != nil {
				return iteration.WriteResult{}, err
			}
			choices := map[string]bool{}
			for _, choice := range input.Draft.Start.TerminalChoices {
				choices[choice.IssueID] = choice.Retain
			}
			for i, issue := range p.issues {
				historical := p.historical[i]
				if retain, chosen := choices[historical.IssueID]; chosen && !retain {
					continue
				}
				part, err := q.LockIssueIterationParticipation(ctx, db.LockIssueIterationParticipationParams{WorkspaceID: ws, IterationID: iid, IssueID: issue.ID})
				if err != nil {
					return iteration.WriteResult{}, err
				}
				historical.WasCompletedAtStart = historical.StatusCategory == "done"
				original := iteration.OriginalFacts{HistoricalIssue: historical, HasStarted: part.HasStartedCurrentParticipation || historical.StatusCategory == "in_progress" || historical.StatusCategory == "in_review" || historical.StatusCategory == "done"}
				raw, err := json.Marshal(original)
				if err != nil {
					return iteration.WriteResult{}, err
				}
				changed, err := q.CaptureIterationOriginal(ctx, db.CaptureIterationOriginalParams{WorkspaceID: ws, IterationID: iid, IssueID: issue.ID, OriginalFacts: raw, HasStarted: original.HasStarted})
				if err != nil {
					return iteration.WriteResult{}, err
				}
				if changed != 1 {
					return iteration.WriteResult{}, errors.New("original commitment is already frozen or not current")
				}
				if err = q.AppendIterationIssueEvent(ctx, db.AppendIterationIssueEventParams{WorkspaceID: ws, IterationID: iid, IssueID: issue.ID, OperationID: operationID, Kind: "baseline", Actor: iterationActor(actor), SampledAt: sample, BeforeFacts: []byte("null"), AfterFacts: raw}); err != nil {
					return iteration.WriteResult{}, err
				}
			}
		case "cancel":
			if _, err = q.CancelPlannedIteration(ctx, db.CancelPlannedIterationParams{WorkspaceID: ws, ID: iid, BusinessAt: sample, Reason: iterationNullable(input.Draft.Reason)}); err != nil {
				return iteration.WriteResult{}, err
			}
			if err = appendLifecycleEvent(ctx, tx, ws, iid, actor, operationID, "cancel_planned", nil, nil, input.Draft.Reason, p.sampled); err != nil {
				return iteration.WriteResult{}, err
			}
		case "delete":
			count, err := q.DeleteUnusedPlannedIteration(ctx, db.DeleteUnusedPlannedIterationParams{WorkspaceID: ws, ID: iid})
			if err != nil {
				return iteration.WriteResult{}, err
			}
			if count != 1 {
				return iteration.WriteResult{}, iterationFailure(409, "iteration_preview_stale", "Plan can no longer be deleted")
			}
			deleted = true
		}
		iterationIDs := make([]string, 0, len(p.preview.Iterations))
		for _, row := range p.preview.Iterations {
			iterationIDs = append(iterationIDs, row.ID)
		}
		return iteration.WriteResult{IterationIDs: iterationIDs, Result: iteration.WriteSummary{Deleted: deleted, SettingsRevision: input.Draft.ExpectedSettingsRevision, IssueCount: len(p.issues)}, CommittedAt: p.sampled}, nil
	})
}
