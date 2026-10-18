package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const collaborationAccountReconciliationBatchSize = 32

// CollaborationAccountReconciler consumes durable credential invalidations.
// It never borrows the credential transaction or locks users/resources after
// source locks. Current account epochs already deny obsolete execution authority;
// this worker converges its retained commands, grants and stop intent.
type CollaborationAccountReconciler struct {
	Queries   *db.Queries
	TxStarter TxStarter
}

func NewCollaborationAccountReconciler(q *db.Queries, tx TxStarter) *CollaborationAccountReconciler {
	return &CollaborationAccountReconciler{Queries: q, TxStarter: tx}
}

// Reconcile attempts at most limit steps (each with at most three transactions):
// one source issue or intent completion per step. A failed account is excluded
// for this pass and its attempt time persists a fair position for later passes.
// Failure scheduling adds at most one nonblocking, intent-only statement.
// Work remains durable across shutdown, lock conflicts and uncertain commits.
// Concurrent workers cannot overtake an earlier pending epoch for an account.
func (s *CollaborationAccountReconciler) Reconcile(ctx context.Context, limit int32) (int, error) {
	if limit < 1 || limit > 128 {
		return 0, errors.New("collaboration reconciliation limit must be between 1 and 128")
	}
	processed := 0
	excluded := make([]pgtype.UUID, 0)
	var failures []error
	for range limit {
		if err := ctx.Err(); err != nil {
			return processed, errors.Join(append(failures, err)...)
		}
		worked, account, err := s.reconcileNext(ctx, excluded)
		if err != nil {
			failures = append(failures, err)
			if !account.Valid || ctx.Err() != nil {
				return processed, errors.Join(failures...)
			}
			excluded = append(excluded, account)
			continue
		}
		if !worked {
			return processed, errors.Join(failures...)
		}
		processed++
	}
	return processed, errors.Join(failures...)
}

// Run catches up on startup and remains active when creation/execution is
// disabled. Cancelling ctx stops polling and rolls back an unfinished issue.
func (s *CollaborationAccountReconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		processed, err := s.Reconcile(ctx, collaborationAccountReconciliationBatchSize)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			code := "reconciliation_unavailable"
			var failure *CollaborationError
			var database *pgconn.PgError
			if errors.As(err, &failure) {
				code = failure.Code
			} else if errors.As(err, &database) {
				code = database.Code
			}
			// Constraint failures can contain private row values. Log only the
			// bounded outcome code, never the raw database error or its detail.
			slog.WarnContext(ctx, "collaboration account reconciliation deferred", "code", code)
		} else if processed == collaborationAccountReconciliationBatchSize {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *CollaborationAccountReconciler) reconcileNext(ctx context.Context, excluded []pgtype.UUID) (bool, pgtype.UUID, error) {
	var intent db.CollaborationAccountInvalidation
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := s.TxStarter.Begin(ctx)
		if err != nil {
			return false, pgtype.UUID{}, err
		}
		q := s.Queries.WithTx(tx)
		intent, err = q.LockNextCollaborationAccountInvalidation(ctx, excluded)
		if errors.Is(err, pgx.ErrNoRows) {
			rollbackCollaborationQueue(ctx, tx)
			return false, pgtype.UUID{}, nil
		}
		if err == nil {
			err = s.reconcileInTx(ctx, q, intent)
		}
		if err == nil {
			if err = tx.Commit(ctx); err != nil {
				rollbackCollaborationQueue(ctx, tx)
				// Do not retry an uncertain commit here, even for SQLSTATE 40001.
				// The next scan reads the original durable cursor, not a new key.
				return false, intent.UserID, s.recordFailedAttempt(ctx, intent, fmt.Errorf("commit collaboration account reconciliation: %w", err))
			}
			return true, intent.UserID, nil
		}
		rollbackCollaborationQueue(ctx, tx)
		var conflict *pgconn.PgError
		if !errors.As(err, &conflict) || (conflict.Code != "40001" && conflict.Code != "40P01" && conflict.Code != "55P03") {
			return false, intent.UserID, s.recordFailedAttempt(ctx, intent, err)
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(25+attempt*50+rand.IntN(26)) * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return false, intent.UserID, ctx.Err()
			}
		}
	}
	return false, intent.UserID, s.recordFailedAttempt(ctx, intent, collaborationError(http.StatusServiceUnavailable, "collaboration_busy"))
}

// Only scheduler metadata may be written after a failed/unknown source commit.
// The original intent and cursor remain authoritative; this cannot replay any
// cancellation, stop or grant effect. Persisting the attempt moves failed work
// behind unattempted accounts even after a process restart or a full failed batch.
func (s *CollaborationAccountReconciler) recordFailedAttempt(ctx context.Context, intent db.CollaborationAccountInvalidation, cause error) error {
	if !intent.UserID.Valid || ctx.Err() != nil {
		return cause
	}
	err := s.Queries.RecordCollaborationInvalidationAttempt(ctx, db.RecordCollaborationInvalidationAttemptParams{
		UserID: intent.UserID, AccountEpoch: intent.AccountEpoch,
	})
	return errors.Join(cause, err)
}

func (s *CollaborationAccountReconciler) reconcileInTx(ctx context.Context, q *db.Queries, intent db.CollaborationAccountInvalidation) error {
	next, err := q.NextCollaborationAccountInvalidationIssue(ctx, db.NextCollaborationAccountInvalidationIssueParams{
		UserID: intent.UserID, AccountEpoch: intent.AccountEpoch, LastIssueID: intent.LastIssueID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		count, err := q.CompleteCollaborationAccountInvalidation(ctx, db.CompleteCollaborationAccountInvalidationParams{
			UserID: intent.UserID, AccountEpoch: intent.AccountEpoch,
		})
		if err == nil && count != 1 {
			err = errors.New("collaboration account invalidation completion lost its intent")
		}
		return err
	}
	if err != nil {
		return err
	}
	if !next.WorkspaceID.Valid {
		return errors.New("collaboration account invalidation source is missing")
	}
	if _, err = q.LockCollaborationReconciliationWorkspace(ctx, next.WorkspaceID); err != nil {
		return err
	}
	issue, err := q.LockCollaborationReconciliationIssue(ctx, next.IssueID)
	if err != nil {
		return err
	}
	if issue.WorkspaceID != next.WorkspaceID {
		return errors.New("collaboration account invalidation source changed")
	}
	state, err := q.LockCollaborationReconciliationState(ctx, issue.ID)
	if err != nil {
		return err
	}
	session, err := q.LockCollaborationReconciliationSession(ctx, issue.ID)
	if err != nil {
		return err
	}
	// This is a system invalidation, not an action attributed to the disabled
	// user. No current human read authority is needed to stop obsolete work.
	t := &collaborationQueueTx{q: q, session: session, access: CollaborationIssueAccess{Issue: issue, LifecycleEpoch: state.LifecycleEpoch}}
	t.channels, err = q.LockCollaborationChannels(ctx, session.ID)
	if err != nil {
		return err
	}
	t.targets, err = q.LockCollaborationTargets(ctx, session.ID)
	if err != nil {
		return err
	}
	if err = t.invalidateAccount(ctx, intent); err != nil {
		return err
	}
	count, err := q.AdvanceCollaborationAccountInvalidation(ctx, db.AdvanceCollaborationAccountInvalidationParams{
		UserID: intent.UserID, AccountEpoch: intent.AccountEpoch, IssueID: issue.ID,
	})
	if err == nil && count != 1 {
		err = errors.New("collaboration account invalidation cursor did not advance")
	}
	return err
}

func (t *collaborationQueueTx) invalidateAccount(ctx context.Context, intent db.CollaborationAccountInvalidation) error {
	grants, err := t.q.LockCollaborationOwnerGrants(ctx, db.LockCollaborationOwnerGrantsParams{
		IssueID: t.access.Issue.ID, OwnerUserID: intent.UserID,
	})
	if err != nil {
		return err
	}
	cancelled, err := t.q.CancelObsoleteCollaborationAuthorCommands(ctx, db.CancelObsoleteCollaborationAuthorCommandsParams{
		SessionID: t.session.ID, UserID: intent.UserID, AccountEpoch: intent.AccountEpoch, Reason: intent.Reason,
	})
	if err != nil {
		return err
	}
	revoked, err := t.q.RevokeObsoleteCollaborationOwnerGrants(ctx, db.RevokeObsoleteCollaborationOwnerGrantsParams{
		IssueID: t.access.Issue.ID, UserID: intent.UserID, AccountEpoch: intent.AccountEpoch, Reason: intent.Reason,
	})
	if err != nil {
		return err
	}
	stopped, err := t.stopInvalidatedWork(ctx, intent.UserID, intent.AccountEpoch, grants, intent.Reason)
	if err != nil {
		return err
	}
	if cancelled == 0 && revoked == 0 && stopped == nil {
		return nil
	}
	seq, err := t.q.AdvanceCollaborationEventSequence(ctx, t.access.Issue.ID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		AccountUserID     string                     `json:"account_user_id"`
		AccountEpoch      int64                      `json:"account_epoch,string"`
		Reason            string                     `json:"reason"`
		CancelledCommands int64                      `json:"cancelled_commands"`
		RevokedGrants     int64                      `json:"revoked_grants"`
		Stop              *CollaborationQueueReceipt `json:"stop,omitempty"`
	}{util.UUIDToString(intent.UserID), intent.AccountEpoch, intent.Reason, cancelled, revoked, stopped})
	if err != nil {
		return err
	}
	_, err = t.q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{
		IssueID: t.access.Issue.ID, EventSeq: seq, Kind: "account_credentials_revoked", Payload: payload,
	})
	return err
}
