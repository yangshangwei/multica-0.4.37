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
// RunID/RunSettled describe one run inside a target execution group.
type CollaborationQueueReceipt struct {
	Status           int                                 `json:"status"`
	Code             string                              `json:"code"`
	IssueID          pgtype.UUID                         `json:"issue_id"`
	RequestID        string                              `json:"request_id,omitempty"`
	Action           string                              `json:"action,omitempty"`
	ProtocolVersion  int                                 `json:"protocol_version"`
	Scope            collaboration.ControlScope          `json:"scope"`
	TargetID         pgtype.UUID                         `json:"target_id"`
	GrantID          pgtype.UUID                         `json:"grant_id"`
	ChannelID        pgtype.UUID                         `json:"channel_id"`
	BatchID          pgtype.UUID                         `json:"batch_id"`
	CommandID        pgtype.UUID                         `json:"command_id"`
	CommandSequence  int64                               `json:"command_sequence,string,omitempty"`
	QueueState       string                              `json:"queue_state"`
	ControlVersion   int64                               `json:"control_version,string"`
	TargetSetVersion int64                               `json:"target_set_version,string,omitempty"`
	GroupID          pgtype.UUID                         `json:"group_id"`
	GroupSettled     bool                                `json:"group_settled"`
	Targets          []CollaborationTargetControlResult  `json:"targets,omitempty"`
	Channels         []CollaborationChannelControlResult `json:"channels"`
	TargetHolds      []CollaborationTargetHoldResult     `json:"target_holds"`
	RunID            pgtype.UUID                         `json:"run_id"`
	RunSettled       bool                                `json:"run_settled"`
}

type collaborationQueueTx struct {
	q             *db.Queries
	actor         pgtype.UUID
	access        CollaborationIssueAccess
	session       db.CollaborationSession
	participant   db.IssueParticipant
	account       db.CollaborationAccountFence
	gate          db.CollaborationExecutionGate
	targets       []db.CollaborationTarget
	channels      []db.CollaborationChannel
	requestID     string
	controlAction string
	controlScope  collaboration.ControlScope
	controlTarget pgtype.UUID
}

func (t *collaborationQueueTx) receipt(code string, status int) CollaborationQueueReceipt {
	r := CollaborationQueueReceipt{Code: code, Status: status, IssueID: t.access.Issue.ID, RequestID: t.requestID, Action: t.controlAction,
		ProtocolVersion: collaboration.ExecutionProtocolVersion, Scope: collaboration.ControlIssue, QueueState: t.session.QueueState,
		ControlVersion: t.session.ControlVersion, TargetSetVersion: t.session.TargetSetVersion,
		Channels: []CollaborationChannelControlResult{}, TargetHolds: []CollaborationTargetHoldResult{}}
	if t.controlScope == collaboration.ControlTarget {
		r.Scope, r.TargetID = t.controlScope, t.controlTarget
		if target, exists := t.target(t.controlTarget); exists {
			r.ChannelID, r.GroupID = target.ChannelID, target.ActiveGroupID
			r.QueueState, r.ControlVersion = target.QueueState, target.ControlVersion
		}
	}
	return r
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
		receipt, inserted, err := t.appendInput(ctx, p)
		if err != nil {
			return CollaborationQueueReceipt{}, err
		}
		if inserted {
			err = t.event(ctx, "command_queued", receipt)
		}
		return receipt, err
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
		if !ok || command.TargetID != p.TargetID {
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
		if err = t.q.CloseEmptyCollaborationBatches(ctx, t.session.ID); err != nil {
			return CollaborationQueueReceipt{}, err
		}
		receipt := t.targetReceipt("command_cancelled", http.StatusOK, target)
		receipt.CommandID, receipt.CommandSequence, receipt.ChannelID, receipt.BatchID = command.ID, command.Seq, command.ChannelID, command.BatchID
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
		receipt, err := s.collaborationQueueMutationInTx(ctx, tx, actor, p, kind, payloadHash, payload, mutate)
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
	p CollaborationQueueRequest, kind, payloadHash string, payload any, mutate func(*collaborationQueueTx) (CollaborationQueueReceipt, error)) (CollaborationQueueReceipt, error) {
	t := &collaborationQueueTx{q: s.Queries.WithTx(tx), actor: actor}
	if kind == "stop_queue" || kind == "resume_queue" {
		t.requestID, t.controlAction = p.RequestID, strings.TrimSuffix(kind, "_queue")
		var control CollaborationStopQueueParams
		switch request := payload.(type) {
		case CollaborationStopQueueParams:
			control = request
		case CollaborationResumeQueueParams:
			control = request.CollaborationStopQueueParams
		}
		t.controlScope = control.Scope
		if control.Scope == collaboration.ControlTarget && len(control.Targets) == 1 {
			t.controlTarget = control.Targets[0].TargetID
		}
	}
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
		// Every active-state write retains the shared gate fence until commit.
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
	t.channels, err = t.q.LockCollaborationChannels(ctx, t.session.ID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
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

// GetControlOperation returns the original full receipt after current object
// and human-account authorization. A missing key is distinct from lost access;
// reading an operation never retries it or substitutes a newer observation.
func (s *CollaborationService) GetControlOperation(ctx context.Context, issueID pgtype.UUID, requestID string) (CollaborationQueueReceipt, error) {
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusNotFound, "not_found")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil || !issueID.Valid || requestID == "" || len(requestID) > 128 {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusBadRequest, "invalid_queue_request")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	if _, err = tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	q := s.Queries.WithTx(tx)
	access, err := s.ResolveIssueAccess(ctx, q, issueID)
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if !access.Capabilities.ControlAI {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusNotFound, "not_found")
	}
	op, err := q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: issueID, ActorUserID: actor, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusNotFound, "operation_not_found")
	}
	if err != nil {
		return CollaborationQueueReceipt{}, err
	}
	if op.Kind != "stop_queue" && op.Kind != "resume_queue" {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusNotFound, "operation_not_found")
	}
	if !op.CompletedAt.Valid {
		return CollaborationQueueReceipt{}, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	var r CollaborationQueueReceipt
	if err = json.Unmarshal(op.Result, &r); err != nil || r.Code == "" || r.Code != op.ResultCode.String || r.Status < 200 || r.Status > 599 ||
		r.IssueID != issueID || r.RequestID != requestID || r.Action != strings.TrimSuffix(op.Kind, "_queue") ||
		r.ProtocolVersion != collaboration.ExecutionProtocolVersion || r.Channels == nil || r.TargetHolds == nil ||
		(r.Scope != collaboration.ControlIssue && r.Scope != collaboration.ControlTarget) {
		return CollaborationQueueReceipt{}, errors.New("invalid collaboration control operation receipt")
	}
	if err = tx.Commit(ctx); err != nil {
		return CollaborationQueueReceipt{}, err
	}
	return r, nil
}
