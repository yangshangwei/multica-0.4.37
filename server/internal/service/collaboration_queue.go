package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// CollaborationQueueRequest carries the identity observed when the human made
// the intent. Rejoining, reopening or restoring an account cannot revive it.
// These services are internal until runtime admission and controlled content
// are complete; no ordinary task enqueue or runtime fallback is permitted.
type CollaborationQueueRequest struct {
	IssueID                  pgtype.UUID
	RequestID                string
	ExpectedLifecycleEpoch   int64
	ExpectedParticipantEpoch int64
	ExpectedAccountEpoch     int64
}

type CollaborationSubmitCommandParams struct {
	CollaborationQueueRequest
	Content string
}

type CollaborationCancelCommandParams struct {
	CollaborationQueueRequest
	CommandID                     pgtype.UUID
	ExpectedCollaborationRevision int64
}

type CollaborationStopQueueParams struct {
	CollaborationQueueRequest
	ExpectedControlVersion  int64
	ObservedRunID           pgtype.UUID
	ObservedFence           int64
	ObservedClaimGeneration int64
}

type CollaborationResumeQueueParams struct {
	CollaborationQueueRequest
	ExpectedControlVersion int64
	ExpectedExecutionEpoch int64
}

// CollaborationQueueReceipt is a content-free durable result, not a refreshed
// queue snapshot. In particular a rejected Resume always replays its rejection.
type CollaborationQueueReceipt struct {
	Status          int         `json:"status"`
	Code            string      `json:"code"`
	CommandID       pgtype.UUID `json:"command_id"`
	CommandSequence int64       `json:"command_sequence,string,omitempty"`
	QueueState      string      `json:"queue_state"`
	ControlVersion  int64       `json:"control_version,string"`
	RunID           pgtype.UUID `json:"run_id"`
	RunSettled      bool        `json:"run_settled"`
}

type collaborationQueueTx struct {
	q           *db.Queries
	actor       pgtype.UUID
	access      CollaborationIssueAccess
	session     db.CollaborationSession
	participant db.IssueParticipant
	account     db.CollaborationAccountFence
	gate        db.CollaborationExecutionGate
}

func (t *collaborationQueueTx) receipt(code string, status int) CollaborationQueueReceipt {
	return CollaborationQueueReceipt{Code: code, Status: status, QueueState: t.session.QueueState, ControlVersion: t.session.ControlVersion}
}

// SubmitCommand saves an immutable human input even during maintenance or with
// no runtime bound. Only a future admitted promoter may turn it into execution.
func (s *CollaborationService) SubmitCommand(ctx context.Context, p CollaborationSubmitCommandParams) (CollaborationQueueReceipt, error) {
	if strings.TrimSpace(p.Content) == "" {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "empty_command")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "submit_command", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		if !t.access.Capabilities.SubmitCommand || (t.access.Issue.AdmissionStatus != "not_required" && t.access.Issue.AdmissionStatus != "accepted") {
			return t.receipt("command_unavailable", http.StatusConflict), nil
		}
		seq, err := t.q.AllocateCollaborationCommandSequence(ctx, t.session.ID)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		command, err := t.q.CreateCollaborationCommand(ctx, db.CreateCollaborationCommandParams{
			SessionID: t.session.ID, Seq: seq, AuthorUserID: t.actor,
			ParticipantEpoch: t.participant.Epoch, AuthorAuthEpoch: t.account.Epoch,
			LifecycleEpoch: t.access.LifecycleEpoch, Content: p.Content,
		})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		receipt := t.receipt("command_queued", http.StatusCreated)
		receipt.CommandID, receipt.CommandSequence = command.ID, command.Seq
		return receipt, t.event(ctx, "command_queued", receipt)
	})
}

func (s *CollaborationService) CancelWaitingCommand(ctx context.Context, p CollaborationCancelCommandParams) (CollaborationQueueReceipt, error) {
	if !p.CommandID.Valid || p.ExpectedCollaborationRevision < 1 {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_command_cancellation")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "cancel_command", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		command, err := t.q.LockCollaborationCommand(ctx, db.LockCollaborationCommandParams{SessionID: t.session.ID, CommandID: p.CommandID})
		if errors.Is(err, pgx.ErrNoRows) {
			return t.receipt("not_found", http.StatusNotFound), nil
		}
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		creator := t.access.Issue.CreatorType == "member" && t.access.Issue.CreatorID == t.actor
		handler := t.access.Issue.AssigneeType.String == "member" && t.access.Issue.AssigneeID == t.actor
		if command.AuthorUserID != t.actor && !creator && !handler {
			return t.receipt("forbidden", http.StatusForbidden), nil
		}
		if command.State != "waiting" {
			return t.receipt("command_not_waiting", http.StatusConflict), nil
		}
		if t.access.Revision != p.ExpectedCollaborationRevision {
			return t.receipt("stale_revision", http.StatusConflict), nil
		}
		command, err = t.q.CancelWaitingCollaborationCommand(ctx, db.CancelWaitingCollaborationCommandParams{SessionID: t.session.ID, CommandID: p.CommandID, Reason: "cancelled_by_participant"})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		receipt := t.receipt("command_cancelled", http.StatusOK)
		receipt.CommandID, receipt.CommandSequence = command.ID, command.Seq
		return receipt, t.event(ctx, "command_cancelled", receipt)
	})
}

// StopQueue records control intent. Only the server's never-issued-permit
// fact can settle a run here. Neither a business outcome nor a task terminal
// state, timeout, missing PID or daemon claim of exit is accepted as proof.
func (s *CollaborationService) StopQueue(ctx context.Context, p CollaborationStopQueueParams) (CollaborationQueueReceipt, error) {
	if p.ExpectedControlVersion < 1 || (p.ObservedRunID.Valid && (p.ObservedFence < 1 || p.ObservedClaimGeneration < 1)) ||
		(!p.ObservedRunID.Valid && (p.ObservedFence != 0 || p.ObservedClaimGeneration != 0)) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_stop")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "stop_queue", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		run, err := t.unsettledRun(ctx)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if run.ID.Valid && (p.ObservedRunID != run.ID || p.ObservedFence != run.Fence || p.ObservedClaimGeneration != run.ClaimGeneration) {
			return t.receipt("stale_execution", http.StatusConflict), nil
		}
		if !run.ID.Valid && p.ObservedRunID.Valid {
			previous, err := t.q.GetCollaborationRun(ctx, db.GetCollaborationRunParams{SessionID: t.session.ID, RunID: p.ObservedRunID})
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!previous.SettledAt.Valid || previous.Fence != p.ObservedFence)) {
				return t.receipt("stale_execution", http.StatusConflict), nil
			}
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
		}
		if t.session.ControlVersion != p.ExpectedControlVersion {
			return t.receipt("stale_control_version", http.StatusConflict), nil
		}
		t.session, err = t.q.PauseCollaborationSession(ctx, db.PauseCollaborationSessionParams{SessionID: t.session.ID, PauseReason: "participant_stop"})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		receipt := t.receipt("already_ended", http.StatusOK)
		if run.ID.Valid {
			receipt, err = t.stopRun(ctx, run, "stopped", "participant_stop")
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
		}
		return receipt, t.event(ctx, "queue_stop_requested", receipt)
	})
}

// stopRun borrows the source/session/run locks. The original pre-stop facts
// are the only server evidence accepted for settlement; a forced disposition
// never supplies proof. Later controls retain the first forced-disposition root.
func (t *collaborationQueueTx) stopRun(ctx context.Context, run db.CollaborationRun, disposition, reason string) (CollaborationQueueReceipt, error) {
	neverLaunched := false
	switch run.ProgressState {
	case "prepared", "dispatched", "preparing":
		neverLaunched = run.AuthorizedAttemptSeq == 0 && !run.StartedAt.Valid && run.ProofState == "unknown" &&
			(!run.BusinessOutcome.Valid || run.BusinessOutcome.String == "failure")
	}
	// Cancellation applies only to work proven never to have launched. A
	// departing author with any potentially live execution requests a stop.
	// The SQL writers still preserve the first recorded termination root.
	if disposition == "cancelled" && !neverLaunched {
		disposition = "stopped"
	}
	var err error
	switch disposition {
	case "stopped":
		run, err = t.q.RequestCollaborationRunStop(ctx, db.RequestCollaborationRunStopParams{RunID: run.ID, DispositionReason: reason})
	case "cancelled":
		run, err = t.q.RequestCollaborationRunCancellation(ctx, db.RequestCollaborationRunCancellationParams{RunID: run.ID, Reason: reason})
	default:
		return CollaborationQueueReceipt{}, errors.New("invalid collaboration run disposition")
	}
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if err = t.q.RequestCollaborationTerminationStop(ctx, run.ID); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	receipt := t.receipt("stop_requested", http.StatusOK)
	receipt.RunID = run.ID
	if neverLaunched {
		run, err = t.q.SettleNeverLaunchedCollaborationRun(ctx, run.ID)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		command, err := t.q.FinishCollaborationCommand(ctx, db.FinishCollaborationCommandParams{SessionID: t.session.ID, CommandID: run.CommandID,
			State: run.ForcedDisposition.String, Reason: run.DispositionReason.String})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if _, err = t.q.ClearCollaborationActiveRun(ctx, db.ClearCollaborationActiveRunParams{SessionID: t.session.ID, RunID: run.ID}); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		receipt.Code, receipt.RunSettled = "stopped_before_launch", true
		receipt.CommandID, receipt.CommandSequence = command.ID, command.Seq
	}
	return receipt, nil
}

// ResumeQueue currently implements durable refusal only. It deliberately has
// no active-state writer until full resource/installation/provider admission
// exists. A valid future request must still be a fresh human intent.
func (s *CollaborationService) ResumeQueue(ctx context.Context, p CollaborationResumeQueueParams) (CollaborationQueueReceipt, error) {
	if p.ExpectedControlVersion < 1 || p.ExpectedExecutionEpoch < 1 {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_resume")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "resume_queue", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		run, err := t.q.LockUnsettledCollaborationRun(ctx, t.session.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return CollaborationQueueReceipt{}, err
		}
		// Either barrier is enough to refuse. Persist the refusal even when
		// they disagree, so a later repair cannot revive this request key.
		if run.ID.Valid || t.session.ActiveRunID.Valid {
			return t.receipt("stop_unconfirmed", http.StatusConflict), nil
		}
		if !t.access.Capabilities.SubmitCommand {
			return t.receipt("command_unavailable", http.StatusConflict), nil
		}
		if p.ExpectedControlVersion != t.session.ControlVersion {
			return t.receipt("stale_control_version", http.StatusConflict), nil
		}
		if p.ExpectedExecutionEpoch != t.gate.Epoch {
			return t.receipt("stale_admission", http.StatusConflict), nil
		}
		return t.receipt("collaboration_execution_unavailable", http.StatusServiceUnavailable), nil
	})
}

func (t *collaborationQueueTx) unsettledRun(ctx context.Context) (db.CollaborationRun, error) {
	run, err := t.q.LockUnsettledCollaborationRun(ctx, t.session.ID)
	if errors.Is(err, pgx.ErrNoRows) && !t.session.ActiveRunID.Valid {
		return db.CollaborationRun{}, nil
	}
	if err != nil {
		return db.CollaborationRun{}, err
	}
	// Never rely on the convenience pointer alone to decide that a session is
	// idle. A disagreement with the durable unsettled-run row is an error.
	if run.ID != t.session.ActiveRunID {
		return db.CollaborationRun{}, errors.New("collaboration active run does not match unsettled run")
	}
	return run, nil
}

func (t *collaborationQueueTx) event(ctx context.Context, kind string, receipt CollaborationQueueReceipt) error {
	seq, err := t.q.AdvanceCollaborationEventSequence(ctx, t.access.Issue.ID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	_, err = t.q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: t.access.Issue.ID, EventSeq: seq, ActorUserID: t.actor, Kind: kind, Payload: payload})
	return err
}

func (s *CollaborationService) mutateCollaborationQueue(ctx context.Context, p CollaborationQueueRequest, kind string, payload any,
	mutate func(*collaborationQueueTx) (CollaborationQueueReceipt, error)) (CollaborationQueueReceipt, error) {
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil || !p.IssueID.Valid || len(p.RequestID) == 0 || len(p.RequestID) > 128 ||
		p.ExpectedLifecycleEpoch < 1 || p.ExpectedParticipantEpoch < 1 || p.ExpectedAccountEpoch < 1 {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_queue_request")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	digest := sha256.Sum256(encoded)
	payloadHash := hex.EncodeToString(digest[:])
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := s.TxStarter.Begin(ctx)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		receipt, err := s.collaborationQueueMutationInTx(ctx, tx, actor, p, kind, payloadHash, mutate)
		if err == nil {
			if err = tx.Commit(ctx); err != nil {
				rollbackCollaborationQueue(ctx, tx)
				// Even a retryable-looking SQLSTATE on an uncertain commit must
				// not trigger another mutation. Recover by the original key.
				return CollaborationQueueReceipt{}, fmt.Errorf("commit collaboration queue operation: %w", err)
			}
			if receipt.Status >= http.StatusBadRequest {
				return receipt, collaborationError(receipt.Status, receipt.Code)
			}
			return receipt, nil
		}
		rollbackCollaborationQueue(ctx, tx)
		var conflict *pgconn.PgError
		if !errors.As(err, &conflict) || (conflict.Code != "40001" && conflict.Code != "40P01" && conflict.Code != "55P03") {
			return CollaborationQueueReceipt{}, err
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(25+attempt*50+rand.IntN(26)) * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return CollaborationQueueReceipt{}, ctx.Err()
			}
		}
	}
	return CollaborationQueueReceipt{}, collaborationError(http.StatusServiceUnavailable, "collaboration_busy")
}

func rollbackCollaborationQueue(ctx context.Context, tx pgx.Tx) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(cleanup)
}

func (s *CollaborationService) collaborationQueueMutationInTx(ctx context.Context, tx pgx.Tx, actor pgtype.UUID,
	p CollaborationQueueRequest, kind, payloadHash string, mutate func(*collaborationQueueTx) (CollaborationQueueReceipt, error)) (CollaborationQueueReceipt, error) {
	t := &collaborationQueueTx{q: s.Queries.WithTx(tx), actor: actor}
	// Pre-read only discovers the source workspace; all authority is re-read
	// after the actor, source issue/session and participant locks are held.
	hint, err := s.ResolveIssueAccess(ctx, t.q, p.IssueID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if _, err = t.q.LockCollaborationUsers(ctx, []pgtype.UUID{actor}); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if kind == "resume_queue" {
		// The gate lock precedes source locks, even while Resume only refuses.
		// Do not move it into the callback when positive admission is added.
		t.gate, err = t.q.LockCollaborationGateShared(ctx)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
	}
	if _, err = t.q.LockWorkspaceForChatSessionCreate(ctx, hint.Issue.WorkspaceID); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	issue, err := t.q.LockCollaborationIssue(ctx, p.IssueID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if issue.WorkspaceID != hint.Issue.WorkspaceID {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusConflict, "stale_source")
	}
	if _, err = t.q.LockIssueCollaboration(ctx, p.IssueID); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	t.session, err = t.q.LockCollaborationSession(ctx, p.IssueID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	t.participant, err = t.q.LockCollaborationParticipant(ctx, db.LockCollaborationParticipantParams{IssueID: p.IssueID, UserID: actor})
	if errors.Is(err, pgx.ErrNoRows) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	t.access, err = s.ResolveIssueAccess(ctx, t.q, p.IssueID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if !t.access.Capabilities.ControlAI || t.participant.RevokedAt.Valid {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	t.account, err = t.q.GetCollaborationAccountFence(ctx, actor)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	opKey := db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: p.IssueID, ActorUserID: actor, RequestID: p.RequestID}
	previous, err := t.q.GetCollaborationOperation(ctx, opKey)
	if err == nil {
		if previous.Kind != kind || previous.PayloadHash != payloadHash {
			return CollaborationQueueReceipt{}, collaborationError(http.StatusConflict, "idempotency_conflict")
		}
		if !previous.CompletedAt.Valid {
			return CollaborationQueueReceipt{}, collaborationError(http.StatusConflict, "operation_incomplete")
		}
		var receipt CollaborationQueueReceipt
		if err = json.Unmarshal(previous.Result, &receipt); err != nil {
			return receipt, fmt.Errorf("decode collaboration receipt: %w", err)
		}
		if receipt.Code == "" || receipt.Code != previous.ResultCode.String || receipt.Status < 200 || receipt.Status > 599 {
			return CollaborationQueueReceipt{}, errors.New("invalid collaboration operation receipt")
		}
		return receipt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CollaborationQueueReceipt{}, err
	}
	op, err := t.q.CreateCollaborationOperation(ctx, db.CreateCollaborationOperationParams{ScopeKind: "issue", ScopeID: p.IssueID, ActorUserID: actor, RequestID: p.RequestID, Kind: kind, PayloadHash: payloadHash})
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	var receipt CollaborationQueueReceipt
	if t.access.LifecycleEpoch != p.ExpectedLifecycleEpoch || t.participant.Epoch != p.ExpectedParticipantEpoch || t.account.Epoch != p.ExpectedAccountEpoch {
		receipt = t.receipt("stale_authority", http.StatusConflict)
	} else {
		receipt, err = mutate(t)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	_, err = t.q.CompleteCollaborationOperation(ctx, db.CompleteCollaborationOperationParams{ID: op.ID, ResultCode: receipt.Code, Result: encoded, ResolvedIssueID: p.IssueID})
	return receipt, err
}
