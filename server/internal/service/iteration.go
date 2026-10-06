package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/iteration"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type IterationService struct {
	TxStarter TxStarter
	Available func(context.Context) bool
	// AuthorizeIssues checks current task-level rights after the complete row set
	// is locked. New management transports must provide it for nonempty sets.
	AuthorizeIssues func(context.Context, pgx.Tx, []db.Issue) error
	// Now supplies the lock-time database clock; nil uses SampleBusinessTime.
	Now func(context.Context, pgx.Tx) (time.Time, error)
}
type EnableIterationInput struct {
	RequestID         string `json:"request_id"`
	ExpectedRevision  int64  `json:"expected_revision"`
	ConfirmedTimezone string `json:"confirmed_timezone"`
}

type iterationTransactionStarter struct{ TxStarter }

func (s iterationTransactionStarter) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if options.IsoLevel != pgx.ReadCommitted {
		_ = tx.Rollback(ctx)
		return nil, errors.New("unsupported iteration write isolation")
	}
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL READ COMMITTED, READ WRITE"); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

func (s *IterationService) Enable(ctx context.Context, workspaceID, actorID pgtype.UUID, input EnableIterationInput, authorize func(context.Context, pgx.Tx) error) (iteration.WriteResult, error) {
	if s.TxStarter == nil {
		return iteration.WriteResult{}, errors.New("iteration operation requires transaction starter")
	}
	requestID, err := uuid.Parse(input.RequestID)
	if err != nil || requestID == uuid.Nil || input.ExpectedRevision < 1 || input.ExpectedRevision > 9007199254740991 {
		return iteration.WriteResult{}, &iteration.OperationError{Status: 400, Code: "invalid_request", Message: "A request ID and safe expected revision are required"}
	}
	input.RequestID = requestID.String()
	input.ConfirmedTimezone = strings.TrimSpace(strings.ReplaceAll(input.ConfirmedTimezone, "\r\n", "\n"))
	if _, err = iteration.ValidateTimezone(input.ConfirmedTimezone); err != nil {
		return iteration.WriteResult{}, &iteration.OperationError{Status: 422, Code: "iteration_validation_failed", Message: "Confirm a valid IANA timezone"}
	}
	key := iteration.OperationKey{WorkspaceID: uuid.UUID(workspaceID.Bytes).String(), ActorUserID: uuid.UUID(actorID.Bytes).String(), RequestID: input.RequestID, Operation: "enable"}
	return iteration.RunOperation(ctx, iterationTransactionStarter{s.TxStarter}, key, input, authorize, func(ctx context.Context, tx pgx.Tx, _ pgtype.UUID) (iteration.WriteResult, error) {
		fail := func(err error) (iteration.WriteResult, error) { return iteration.WriteResult{}, err }
		if s.Available == nil || !s.Available(ctx) {
			return fail(&iteration.OperationError{Status: 422, Code: "iteration_disabled", Message: "Iteration management is not available"})
		}
		q := db.New(tx)
		if err := q.EnsureIterationSettings(ctx, workspaceID); err != nil {
			return fail(err)
		}
		settings, err := q.LockIterationSettings(ctx, workspaceID)
		if err != nil {
			return fail(err)
		}
		if settings.Revision != input.ExpectedRevision {
			return fail(&iteration.OperationError{Status: 409, Code: "iteration_revision_conflict", Message: "Iteration settings changed"})
		}
		// P1 owns this shared setting. The NOWAIT reference lock prevents timezone
		// drift until commit without reversing the workspace/counter writer order.
		zone, err := q.LockIterationPlanningTimezone(ctx, workspaceID)
		if err != nil {
			return fail(err)
		}
		effective := "UTC"
		if zone.Valid {
			effective = zone.String
		}
		if effective != input.ConfirmedTimezone {
			return fail(&iteration.OperationError{Status: 409, Code: "iteration_preview_stale", Message: "Planning timezone changed; confirm the current timezone"})
		}
		sampled, err := iteration.SampleBusinessTime(ctx, tx, nil)
		if err != nil {
			return fail(err)
		}
		if !settings.Enabled {
			settings, err = q.EnableIterationSettings(ctx, db.EnableIterationSettingsParams{WorkspaceID: workspaceID, ExpectedRevision: input.ExpectedRevision})
			if errors.Is(err, pgx.ErrNoRows) {
				return fail(&iteration.OperationError{Status: 409, Code: "iteration_revision_conflict", Message: "Iteration settings changed"})
			}
			if err != nil {
				return fail(err)
			}
		}
		return iteration.WriteResult{IterationIDs: []string{}, Result: iteration.WriteSummary{SettingsRevision: settings.Revision}, CommittedAt: sampled}, nil
	})
}

// ReadIterationSettings requires the caller's current workspace/member read
// authorization. Missing settings is the disabled revision-one default.
func ReadIterationSettings(ctx context.Context, tx pgx.Tx, workspaceID pgtype.UUID) (iteration.Settings, error) {
	out := iteration.Settings{WorkspaceID: uuid.UUID(workspaceID.Bytes).String(), Revision: 1, EffectiveTimezone: "UTC"}
	q := db.New(tx)
	settings, err := q.GetIterationSettings(ctx, workspaceID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if err == nil {
		out.Enabled = settings.Enabled
		out.Revision = settings.Revision
	}
	zone, err := q.GetWorkspacePlanningTimezone(ctx, workspaceID)
	if err != nil {
		return out, err
	}
	if zone.Valid {
		out.PlanningTimezone = &zone.String
		out.EffectiveTimezone = zone.String
		out.TimezoneConfigured = true
	}
	return out, nil
}
