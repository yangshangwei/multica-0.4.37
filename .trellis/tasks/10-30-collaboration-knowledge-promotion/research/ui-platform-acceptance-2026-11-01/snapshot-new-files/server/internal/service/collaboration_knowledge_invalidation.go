package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// EnqueueKnowledgeSourceInvalidation borrows the authoritative source writer's
// transaction. This INSERT is its only new edge: no target or candidate lookup.
func EnqueueKnowledgeSourceInvalidation(ctx context.Context, q *db.Queries, cause pgtype.UUID, scope string, scopeID pgtype.UUID) error {
	if !cause.Valid || (scope != "all_sources" && scope != "source_issue" && scope != "source_workspace") || (scope == "all_sources") == scopeID.Valid {
		return errors.New("invalid knowledge scan identity")
	}
	_, err := q.EnqueueKnowledgeScan(ctx, db.EnqueueKnowledgeScanParams{
		ID: knowledgeInvalidationID(cause, "scan"), CauseID: cause, ScopeKind: pgtype.Text{String: scope, Valid: true}, ScopeID: scopeID,
	})
	return err
}

func knowledgeInvalidationID(cause pgtype.UUID, suffix string) pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.NewSHA1(uuid.UUID(cause.Bytes), []byte("collaboration-knowledge:"+suffix)), Valid: true}
}

// DeleteKnowledgeTargetsInTx retains final hints before the caller deletes its
// parent. An absent project selects the whole workspace. The parent lock must
// precede this call; knowledge writers/scanners use that same parent fence.
func DeleteKnowledgeTargetsInTx(ctx context.Context, q *db.Queries, cause, workspace, project pgtype.UUID) error {
	if !cause.Valid || !workspace.Valid {
		return errors.New("invalid knowledge target deletion identity")
	}
	targets, err := q.LockKnowledgeTargetsForDeletion(ctx, db.LockKnowledgeTargetsForDeletionParams{WorkspaceID: workspace, ProjectID: project})
	if err != nil {
		return err
	}
	for _, target := range targets {
		// Initialized but unpublished targets must not signal pending proposals.
		if target.KnowledgeRevision > 0 {
			if err = advanceKnowledgeTargetHint(ctx, q, cause, target); err != nil {
				return err
			}
		}
		if _, err = q.DeleteKnowledgeTarget(ctx, db.DeleteKnowledgeTargetParams{TargetKind: target.TargetKind, TargetID: target.TargetID, WorkspaceID: workspace}); err != nil {
			return err
		}
	}
	return nil
}

// The counter is already locked. A cause affects each target once even when
// many immutable versions route there or an unknown commit is replayed.
func advanceKnowledgeTargetHint(ctx context.Context, q *db.Queries, cause pgtype.UUID, target db.CollaborationKnowledgeTarget) error {
	_, err := q.GetKnowledgeTargetHintForCause(ctx, db.GetKnowledgeTargetHintForCauseParams{
		CauseID: cause, TargetKind: pgtype.Text{String: target.TargetKind, Valid: true}, TargetID: target.TargetID,
	})
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	advanced, err := q.AdvanceKnowledgeTarget(ctx, db.AdvanceKnowledgeTargetParams{TargetKind: target.TargetKind, TargetID: target.TargetID, KnowledgeRevision: target.KnowledgeRevision})
	if err != nil {
		return err
	}
	n, err := q.EnqueueKnowledgeTargetHint(ctx, db.EnqueueKnowledgeTargetHintParams{
		ID: knowledgeInvalidationID(cause, "target:"+target.TargetKind+":"+util.UUIDToString(target.TargetID)), CauseID: cause,
		TargetKind: pgtype.Text{String: target.TargetKind, Valid: true}, TargetID: target.TargetID, TargetWorkspaceID: target.WorkspaceID,
		KnowledgeRevision: pgtype.Int8{Int64: advanced.KnowledgeRevision, Valid: true},
	})
	if err == nil && n != 1 {
		return errors.New("knowledge target hint changed under its counter lock")
	}
	return err
}

// CollaborationKnowledgeInvalidation drains the knowledge outbox independently
// of source events. Neither scanner nor delivery locks source/candidate rows.
type CollaborationKnowledgeInvalidation struct {
	Queries   *db.Queries
	TxStarter TxStarter
	Publisher realtime.RelayPublisher
}

func (s *CollaborationKnowledgeInvalidation) Run(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lastDiagnostic := time.Now()
	for {
		if _, err := s.ScanPending(ctx, 100); err != nil && ctx.Err() == nil {
			slog.Warn("knowledge invalidation scan deferred")
		}
		if _, err := s.DeliverPending(ctx, 100); err != nil && ctx.Err() == nil {
			slog.Warn("knowledge invalidation delivery deferred")
		}
		if s.Queries != nil && time.Since(lastDiagnostic) >= time.Minute {
			bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
			age, err := s.Queries.GetKnowledgeInvalidationPendingAge(bounded)
			cancel()
			if err == nil && age.OldestSeconds >= 30 {
				slog.Warn("knowledge invalidation backlog", "pending_jobs", age.PendingJobs, "oldest_seconds", age.OldestSeconds)
			}
			lastDiagnostic = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ScanPending performs at most limit short transactions and at most one routed
// candidate per transaction. Busy jobs retain their cursor and yield to other
// scans for this turn; failures never masquerade as a successful empty scan.
func (s *CollaborationKnowledgeInvalidation) ScanPending(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 || s.Queries == nil || s.TxStarter == nil {
		return 0, errors.New("invalid knowledge invalidation configuration")
	}
	excluded := make([]pgtype.UUID, 0)
	completed := 0
	var deferred error
	for i := 0; i < limit; i++ {
		id, found, err := s.scanNext(ctx, excluded)
		if err != nil {
			deferred = errors.Join(deferred, err)
			if !id.Valid || ctx.Err() != nil {
				return completed, deferred
			}
			excluded = append(excluded, id)
			if _, attemptErr := s.Queries.RecordKnowledgeJobAttempt(ctx, id); attemptErr != nil {
				return completed, errors.Join(deferred, attemptErr)
			}
			continue
		}
		if !found {
			break
		}
		completed++
	}
	return completed, deferred
}

func (s *CollaborationKnowledgeInvalidation) scanNext(ctx context.Context, excluded []pgtype.UUID) (id pgtype.UUID, found bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return id, false, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	q := s.Queries.WithTx(tx)
	job, err := q.LockNextKnowledgeScan(ctx, excluded)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, false, nil
	}
	if err != nil {
		return id, false, err
	}
	id = job.ID
	if !job.ScanStartedAt.Valid {
		// Persist the snapshot boundary separately. A later busy target cannot
		// repeatedly move the upper bound or extend this job indefinitely.
		if _, err = q.StartKnowledgeScan(ctx, id); err != nil {
			return id, true, err
		}
		return id, true, tx.Commit(ctx)
	}
	var candidate db.NextKnowledgeScanCandidateRow
	if job.ScanUpperBound.Valid {
		candidate, err = q.NextKnowledgeScanCandidate(ctx, db.NextKnowledgeScanCandidateParams{
			UpperBound: job.ScanUpperBound, AfterID: job.ScanCursor, ScopeKind: job.ScopeKind.String, ScopeID: job.ScopeID,
		})
	} else {
		err = pgx.ErrNoRows
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err = q.CompleteKnowledgeScan(ctx, id); err != nil {
			return id, true, err
		}
		return id, true, tx.Commit(ctx)
	}
	if err != nil {
		return id, true, err
	}
	if err = invalidateKnowledgeScanTarget(ctx, q, job.CauseID, candidate); err != nil {
		return id, true, err
	}
	n, err := q.AdvanceKnowledgeScan(ctx, db.AdvanceKnowledgeScanParams{ID: id, Cursor: candidate.ID})
	if err != nil {
		return id, true, err
	}
	if n != 1 {
		return id, true, errors.New("knowledge scan cursor did not advance")
	}
	return id, true, tx.Commit(ctx)
}

func invalidateKnowledgeScanTarget(ctx context.Context, q *db.Queries, cause pgtype.UUID, candidate db.NextKnowledgeScanCandidateRow) error {
	if _, err := q.LockKnowledgeWorkspace(ctx, candidate.TargetWorkspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // Retained routing for a deleted parent is processed, never recreated.
		}
		return err
	}
	targetID := candidate.TargetWorkspaceID
	if candidate.TargetKind == "project" {
		targetID = candidate.TargetProjectID
		if _, err := q.LockKnowledgeProject(ctx, db.LockKnowledgeProjectParams{ID: targetID, WorkspaceID: candidate.TargetWorkspaceID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
	}
	target, err := q.LockKnowledgeTargetForWrite(ctx, db.LockKnowledgeTargetForWriteParams{TargetKind: candidate.TargetKind, TargetID: targetID})
	if err != nil {
		// A live parent with published history but no counter is corruption,
		// not an empty target. Do not advance past it or initialize revision 0.
		return err
	}
	if target.WorkspaceID != candidate.TargetWorkspaceID {
		return errors.New("knowledge target routing mismatch")
	}
	return advanceKnowledgeTargetHint(ctx, q, cause, target)
}

func (s *CollaborationKnowledgeInvalidation) DeliverPending(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 || s.Queries == nil || s.TxStarter == nil || s.Publisher == nil {
		return 0, errors.New("invalid knowledge invalidation configuration")
	}
	for delivered := 0; delivered < limit; delivered++ {
		found, err := s.deliverNext(ctx)
		if err != nil || !found {
			return delivered, err
		}
	}
	return limit, nil
}

func (s *CollaborationKnowledgeInvalidation) deliverNext(ctx context.Context) (found bool, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return false, err
	}
	var hintID pgtype.UUID
	defer func() {
		rollbackCollaborationQueue(ctx, tx)
		if err != nil && hintID.Valid && ctx.Err() == nil {
			_, attemptErr := s.Queries.RecordKnowledgeJobAttempt(ctx, hintID)
			err = errors.Join(err, attemptErr)
		}
	}()
	q := s.Queries.WithTx(tx)
	hint, err := q.LockNextKnowledgeTargetHint(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	hintID = hint.ID
	frame, err := json.Marshal(struct {
		Type    string                                    `json:"type"`
		Payload protocol.CollaborationKnowledgeTargetHint `json:"payload"`
	}{protocol.EventCollaborationKnowledgeChanged, protocol.CollaborationKnowledgeTargetHint{
		TargetKind: hint.TargetKind.String, TargetID: util.UUIDToString(hint.TargetID), KnowledgeRevision: strconv.FormatInt(hint.KnowledgeRevision.Int64, 10),
	}})
	if err != nil {
		return true, err
	}
	if err = s.Publisher.PublishWithID(realtime.ScopeWorkspace, util.UUIDToString(hint.TargetWorkspaceID), "", frame, util.UUIDToString(hint.ID)+":target"); err != nil {
		return true, err
	}
	n, err := q.MarkKnowledgeTargetHintDelivered(ctx, hint.ID)
	if err != nil {
		return true, err
	}
	if n != 1 {
		return true, errors.New("knowledge target delivery marker unavailable")
	}
	return true, tx.Commit(ctx)
}

// AuthorizeKnowledgeWorkspace checks the room's current human authority without
// consulting the source or a possibly deleted project. Projects have the same
// workspace-wide visibility as their parent; the outbox retains that routing.
func (s *CollaborationService) AuthorizeKnowledgeWorkspace(ctx context.Context, userID, workspaceID string) (bool, error) {
	viewer, err := collaborationNotificationViewer(ctx, s.Queries)
	if err != nil {
		var denied *CollaborationError
		if errors.As(err, &denied) && (denied.Status == 403 || denied.Status == 404) {
			return false, nil
		}
		return false, err
	}
	if util.UUIDToString(viewer) != userID {
		return false, nil
	}
	workspace, err := util.ParseUUID(workspaceID)
	if err != nil {
		return false, nil
	}
	if _, err = s.Queries.GetWorkspace(ctx, workspace); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	_, err = s.Queries.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: viewer, WorkspaceID: workspace})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
