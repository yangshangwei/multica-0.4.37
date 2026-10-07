package service

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
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
	if input.Draft.Operation == "disable" && authorize != nil {
		initialAuthorize := authorize
		authorize = func(ctx context.Context, tx pgx.Tx) error {
			if err := initialAuthorize(ctx, tx); err != nil {
				return err
			}
			return requireIterationAdministrator(ctx, tx, ws, actor)
		}
	}
	return s.lifecycleOperation(ctx, ws, actor, input.RequestID, input.Draft.Operation, input, authorize, func(ctx context.Context, tx pgx.Tx, operationID pgtype.UUID) (iteration.WriteResult, error) {
		p, err := s.prepareLifecycle(ctx, tx, ws, actor, input.Draft, true, input.PreviewHash)
		if err != nil {
			return iteration.WriteResult{}, err
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
		var snapshotID *string
		if input.Draft.Operation == "end" || input.Draft.Operation == "cancel" || input.Draft.Operation == "handoff" || input.Draft.Operation == "disable" {
			snapshotID, err = s.freezeClosure(ctx, tx, ws, actor, operationID, input.Draft, &p)
			if err != nil {
				return iteration.WriteResult{}, err
			}
		}
		if err = iteration.CommitMembershipChanges(ctx, tx, p.changes, iterationActor(actor), operationID, p.sampled); err != nil {
			return iteration.WriteResult{}, err
		}

		var iid pgtype.UUID
		if input.Draft.IterationID != nil {
			iid, _ = iterationUUID(input.Draft.IterationID)
		}
		deleted := false
		switch input.Draft.Operation {
		case "start", "handoff":
			iid, _ = iterationUUID(&input.Draft.Start.TargetID)
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
			if err = captureStartedOriginals(ctx, tx, ws, iid, actor, operationID, p.sampled); err != nil {
				return iteration.WriteResult{}, err
			}
			if err = EnqueueIterationNotifications(ctx, q, ws, iid, operationID, "start", p.preview.Recipients); err != nil {
				return iteration.WriteResult{}, err
			}

		case "cancel", "disable":
			for _, item := range p.preview.Iterations {
				row := p.rows[util.MustParseUUID(item.ID)]
				if row.Status != "planned" || (input.Draft.Operation == "cancel" && row.ID != iid) {
					continue
				}
				iid = row.ID
				if _, err = q.CancelPlannedIteration(ctx, db.CancelPlannedIterationParams{WorkspaceID: ws, ID: iid, BusinessAt: sample, Reason: iterationNullable(iterationClosureReason(input.Draft))}); err != nil {
					return iteration.WriteResult{}, err
				}
				if err = appendLifecycleEvent(ctx, tx, ws, iid, actor, operationID, "cancel_planned", nil, nil, iterationClosureReason(input.Draft), p.sampled); err != nil {
					return iteration.WriteResult{}, err
				}
				if input.Draft.Operation != "disable" {
					if err = EnqueueIterationNotifications(ctx, q, ws, iid, operationID, "cancel", p.preview.Recipients); err != nil {
						return iteration.WriteResult{}, err
					}
				}
			}
			if input.Draft.Operation == "disable" {
				if err = EnqueueIterationNotifications(ctx, q, ws, pgtype.UUID{}, operationID, "disable", p.preview.Recipients); err != nil {
					return iteration.WriteResult{}, err
				}
				if _, err = q.DisableIterationSettings(ctx, ws); err != nil {
					return iteration.WriteResult{}, err
				}
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
		if err = s.persistClosureSnapshots(ctx, tx, ws, &p); err != nil {
			return iteration.WriteResult{}, err
		}
		iterationIDs := make([]string, 0, len(p.preview.Iterations))
		for _, row := range p.preview.Iterations {
			iterationIDs = append(iterationIDs, row.ID)
		}
		settingsRevision := input.Draft.ExpectedSettingsRevision
		if input.Draft.Operation == "disable" {
			settingsRevision++
		}
		return iteration.WriteResult{IterationIDs: iterationIDs, Result: iteration.WriteSummary{SnapshotID: snapshotID, Deleted: deleted, SettingsRevision: settingsRevision, IssueCount: len(p.issues)}, CommittedAt: p.sampled}, nil
	})
}
