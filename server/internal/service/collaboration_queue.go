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
	TargetID              pgtype.UUID
	InputEventID          pgtype.UUID
	ExpectedTargetEpoch   int64
	ExpectedConfigVersion int64
	Content               string
}

type CollaborationCancelCommandParams struct {
	CollaborationQueueRequest
	TargetID                      pgtype.UUID
	CommandID                     pgtype.UUID
	ExpectedCollaborationRevision int64
}

// Queue receipts describe the original operation, never refreshed authority.
// RunID/RunSettled are used only for unresolved pre-v2 execution history.
type CollaborationQueueReceipt struct {
	Status           int                                `json:"status"`
	Code             string                             `json:"code"`
	ProtocolVersion  int                                `json:"protocol_version"`
	Scope            collaboration.ControlScope         `json:"scope"`
	TargetID         pgtype.UUID                        `json:"target_id"`
	CommandID        pgtype.UUID                        `json:"command_id"`
	CommandSequence  int64                              `json:"command_sequence,string,omitempty"`
	QueueState       string                             `json:"queue_state"`
	ControlVersion   int64                              `json:"control_version,string"`
	TargetSetVersion int64                              `json:"target_set_version,string,omitempty"`
	GroupID          pgtype.UUID                        `json:"group_id"`
	GroupSettled     bool                               `json:"group_settled"`
	Targets          []CollaborationTargetControlResult `json:"targets,omitempty"`
	RunID            pgtype.UUID                        `json:"run_id"`
	RunSettled       bool                               `json:"run_settled"`
}

type collaborationQueueTx struct {
	q           *db.Queries
	actor       pgtype.UUID
	access      CollaborationIssueAccess
	session     db.CollaborationSession
	participant db.IssueParticipant
	account     db.CollaborationAccountFence
	gate        db.CollaborationExecutionGate
	targets     []db.CollaborationTarget
}

func (t *collaborationQueueTx) receipt(code string, status int) CollaborationQueueReceipt {
	return CollaborationQueueReceipt{Code: code, Status: status, ProtocolVersion: collaboration.ExecutionProtocolVersion, Scope: collaboration.ControlIssue, QueueState: t.session.QueueState, ControlVersion: t.session.ControlVersion, TargetSetVersion: t.session.TargetSetVersion}
}

// SubmitCommand saves an immutable input for one explicitly selected target.
// It does not promote, wake a daemon, or clear either level of pause.
func (s *CollaborationService) SubmitCommand(ctx context.Context, p CollaborationSubmitCommandParams) (CollaborationQueueReceipt, error) {
	if !p.TargetID.Valid || !p.InputEventID.Valid || p.ExpectedTargetEpoch < 1 || p.ExpectedConfigVersion < 1 {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_command_target")
	}
	if strings.TrimSpace(p.Content) == "" {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "empty_command")
	}
	return s.mutateCollaborationQueue(ctx, p.CollaborationQueueRequest, "submit_command", p, func(t *collaborationQueueTx) (CollaborationQueueReceipt, error) {
		if !t.access.Capabilities.SubmitCommand || (t.access.Issue.AdmissionStatus != "not_required" && t.access.Issue.AdmissionStatus != "accepted") {
			return t.receipt("command_unavailable", http.StatusConflict), nil
		}
		target, ok := t.target(p.TargetID)
		if !ok {
			return t.receipt("target_not_found", http.StatusNotFound), nil
		}
		// The input event identity deduplicates a target even when delivery uses
		// another transport request key. Never assign a different snapshot.
		prior, err := t.q.GetCollaborationCommandInput(ctx, db.GetCollaborationCommandInputParams{TargetID: target.ID, InputEventID: p.InputEventID})
		if err == nil {
			if prior.Content != p.Content || prior.AuthorUserID != t.actor || prior.ParticipantEpoch != t.participant.Epoch ||
				prior.AuthorAuthEpoch != t.account.Epoch || prior.LifecycleEpoch != t.access.LifecycleEpoch {
				return t.targetReceipt("input_event_conflict", http.StatusConflict, target), nil
			}
			r := t.targetReceipt("command_queued", http.StatusCreated, target)
			r.CommandID, r.CommandSequence = prior.ID, prior.Seq
			return r, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return CollaborationQueueReceipt{}, err
		}
		if target.TargetEpoch != p.ExpectedTargetEpoch || target.ConfigVersion != p.ExpectedConfigVersion {
			return t.targetReceipt("stale_target", http.StatusConflict, target), nil
		}
		if target.RevokedAt.Valid {
			return t.targetReceipt("grant_required", http.StatusConflict, target), nil
		}
		if code, err := t.authorizeTargetInput(ctx, target); err != nil || code != "" {
			if err != nil {
				return CollaborationQueueReceipt{}, err
			}
			return t.targetReceipt(code, http.StatusConflict, target), nil
		}
		seq, err := t.q.AllocateCollaborationCommandSequence(ctx, db.AllocateCollaborationCommandSequenceParams{SessionID: t.session.ID, TargetID: target.ID})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		command, err := t.q.CreateCollaborationCommand(ctx, db.CreateCollaborationCommandParams{
			SessionID: t.session.ID, TargetID: target.ID, InputEventID: p.InputEventID, Seq: seq, AuthorUserID: t.actor,
			ParticipantEpoch: t.participant.Epoch, AuthorAuthEpoch: t.account.Epoch,
			LifecycleEpoch: t.access.LifecycleEpoch, Content: p.Content,
		})
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		r := t.targetReceipt("command_queued", http.StatusCreated, target)
		r.CommandID, r.CommandSequence = command.ID, command.Seq
		return r, t.event(ctx, "command_queued", r)
	})
}

func (s *CollaborationService) CancelWaitingCommand(ctx context.Context, p CollaborationCancelCommandParams) (CollaborationQueueReceipt, error) {
	if !p.TargetID.Valid || !p.CommandID.Valid || p.ExpectedCollaborationRevision < 1 {
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
		target, ok := t.target(p.TargetID)
		if !ok || command.TargetID != p.TargetID || command.LegacyUnresolved {
			return t.receipt("target_not_found", http.StatusNotFound), nil
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
		receipt := t.targetReceipt("command_cancelled", http.StatusOK, target)
		receipt.CommandID, receipt.CommandSequence = command.ID, command.Seq
		return receipt, t.event(ctx, "command_cancelled", receipt)
	})
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
	idsParam := db.GetCollaborationWorkflowUserIDsParams{IssueID: p.IssueID, ActorID: actor}
	ids, err := t.q.GetCollaborationWorkflowUserIDs(ctx, idsParam)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if _, err = t.q.LockCollaborationUsers(ctx, ids); err != nil {
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
	currentIDs, err := t.q.GetCollaborationWorkflowUserIDs(ctx, idsParam)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if !sameCollaborationUserIDs(ids, currentIDs) {
		return CollaborationQueueReceipt{}, &pgconn.PgError{Code: "40001", Message: "collaboration account lock set changed"}
	}
	t.targets, err = t.q.LockCollaborationTargets(ctx, t.session.ID)
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

func sameCollaborationUserIDs(a, b []pgtype.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
