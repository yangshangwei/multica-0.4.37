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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationWorkflowRequest struct {
	CollaborationQueueRequest
	ExpectedCollaborationRevision int64
}

type CollaborationParticipantParams struct {
	CollaborationWorkflowRequest
	TargetUserID pgtype.UUID
}

type CollaborationTransferParams struct {
	CollaborationParticipantParams
	Note string
}

// Receipts describe this operation only. Clients must reload current authority
// and state; replaying a receipt never reconstructs an old private projection.
type CollaborationWorkflowReceipt struct {
	Status                int                         `json:"status"`
	Code                  string                      `json:"code"`
	CollaborationRevision int64                       `json:"collaboration_revision,string"`
	QueueState            string                      `json:"queue_state"`
	ControlVersion        int64                       `json:"control_version,string"`
	RunID                 pgtype.UUID                 `json:"run_id"`
	RunSettled            bool                        `json:"run_settled"`
	Execution             *CollaborationQueueReceipt  `json:"execution,omitempty"`
	StatusOptions         *CollaborationStatusOptions `json:"status_options,omitempty"`
}

// This is the only recovery projection allowed after object access is lost.
type CollaborationWorkflowOperationReceipt struct {
	Status                int    `json:"status"`
	Code                  string `json:"code"`
	CollaborationRevision int64  `json:"collaboration_revision,string"`
}

type collaborationWorkflowTx struct {
	*collaborationQueueTx
	tx        pgx.Tx
	iteration db.Iteration
	catalog   []db.IssueStatus
	users     map[pgtype.UUID]db.User
	operation db.CollaborationOperation
}

func (t *collaborationWorkflowTx) result(code string, status int) CollaborationWorkflowReceipt {
	return CollaborationWorkflowReceipt{Status: status, Code: code, CollaborationRevision: t.access.Revision,
		QueueState: t.session.QueueState, ControlVersion: t.session.ControlVersion}
}

func (s *CollaborationService) InviteParticipant(ctx context.Context, p CollaborationParticipantParams) (CollaborationWorkflowReceipt, error) {
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, p.TargetUserID, "invite_participant", p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		eligible, err := t.eligibleRecipient(ctx, p.TargetUserID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if !eligible {
			return t.result("target_unavailable", http.StatusConflict), nil
		}
		joined, err := t.enroll(ctx, p.TargetUserID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if !joined {
			return t.result("already_participant", http.StatusOK), nil
		}
		return t.finish(ctx, "participant_added", p.TargetUserID, nil)
	})
}

func (s *CollaborationService) Transfer(ctx context.Context, p CollaborationTransferParams) (CollaborationWorkflowReceipt, error) {
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, p.TargetUserID, "transfer", p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if collaboration.TerminalCategory(t.access.StatusCategory) || t.session.QueueState == "closed" {
			return t.result("issue_closed", http.StatusConflict), nil
		}
		if !collaboration.NonterminalCategory(t.access.StatusCategory) {
			return t.result("status_unavailable", http.StatusConflict), nil
		}
		eligible, err := t.eligibleRecipient(ctx, p.TargetUserID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if !eligible {
			return t.result("target_unavailable", http.StatusConflict), nil
		}
		if t.access.Issue.AssigneeID == p.TargetUserID {
			return t.result("already_handler", http.StatusOK), nil
		}
		if _, err := t.enroll(ctx, p.TargetUserID); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if err := t.changeHandler(ctx, p.TargetUserID, "transfer", p.Note); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		return t.finish(ctx, "transferred", p.TargetUserID, nil)
	})
}

func (s *CollaborationService) RemoveParticipant(ctx context.Context, p CollaborationParticipantParams) (CollaborationWorkflowReceipt, error) {
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, p.TargetUserID, "remove_participant", p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if p.TargetUserID == t.access.Issue.CreatorID {
			return t.result("creator_cannot_be_removed", http.StatusConflict), nil
		}
		return t.depart(ctx, p.TargetUserID, "participant_removed")
	})
}

func (s *CollaborationService) Leave(ctx context.Context, p CollaborationWorkflowRequest) (CollaborationWorkflowReceipt, error) {
	return s.mutateCollaborationWorkflow(ctx, p, pgtype.UUID{}, "leave", p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if t.access.Issue.CreatorID == t.actor {
			return t.result("creator_cannot_leave", http.StatusConflict), nil
		}
		if t.access.Issue.AssigneeID == t.actor {
			return t.result("handler_must_transfer", http.StatusConflict), nil
		}
		return t.depart(ctx, t.actor, "participant_left")
	})
}

func (t *collaborationWorkflowTx) validTarget(id pgtype.UUID) bool {
	u, ok := t.users[id]
	return ok && !u.DisabledAt.Valid && !auth.IsTemporarilyDisabledUser(util.UUIDToString(u.ID), u.Email.String)
}

func (t *collaborationWorkflowTx) eligibleRecipient(ctx context.Context, id pgtype.UUID) (bool, error) {
	if !t.validTarget(id) {
		return false, nil
	}
	return collaborationRecipientEligible(ctx, t.q, id, t.access.Issue.WorkspaceID)
}

func (t *collaborationWorkflowTx) enroll(ctx context.Context, target pgtype.UUID) (bool, error) {
	participant, err := t.q.GetCollaborationParticipant(ctx, db.GetCollaborationParticipantParams{IssueID: t.access.Issue.ID, UserID: target})
	if err == nil && !participant.RevokedAt.Valid {
		return false, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if _, err = t.q.EnsureCollaborationAccountFence(ctx, target); err != nil {
		return false, err
	}
	_, err = t.q.EnrollCollaborationParticipant(ctx, db.EnrollCollaborationParticipantParams{IssueID: t.access.Issue.ID, UserID: target})
	return err == nil, err
}

// changeHandler is an ordinary responsibility change. It ends the information
// request chain, which no longer matches the new responsibility.
func (t *collaborationWorkflowTx) changeHandler(ctx context.Context, target pgtype.UUID, purpose, note string) error {
	if _, err := t.handoff(ctx, collaborationHandoff{target: target, purpose: purpose, note: note}); err != nil {
		return err
	}
	return t.supersedeInformation(ctx)
}

// collaborationHandoff is one responsibility change. An empty status keeps the
// current source key; requester and parent are set only by the request chain.
type collaborationHandoff struct {
	target            pgtype.UUID
	purpose, note     string
	status            string
	requester, parent pgtype.UUID
}

// handoff writes handler, optional status, one iteration fact and the handoff
// record together. It leaves the information request chain to the caller.
func (t *collaborationWorkflowTx) handoff(ctx context.Context, h collaborationHandoff) (db.IssueHandoff, error) {
	before := t.access.Issue
	record, err := iteration.PrepareIssueRecord(ctx, t.tx, before, t.iteration)
	if err != nil {
		return db.IssueHandoff{}, err
	}
	after := before
	if h.status != "" && h.status != before.Status {
		if after, err = t.q.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{ID: before.ID, WorkspaceID: before.WorkspaceID, Status: h.status}); err != nil {
			return db.IssueHandoff{}, err
		}
	}
	if after, err = t.q.UpdateCollaborationHandler(ctx, db.UpdateCollaborationHandlerParams{IssueID: before.ID, HandlerID: h.target}); err != nil {
		return db.IssueHandoff{}, err
	}
	actor, err := json.Marshal(map[string]any{"type": "member", "id": t.actor, "user_id": t.actor})
	if err != nil {
		return db.IssueHandoff{}, err
	}
	if err = iteration.RecordIssueChange(ctx, t.tx, record, after, actor, t.operation.ID); err != nil {
		return db.IssueHandoff{}, err
	}
	created, err := t.q.CreateCollaborationHandoff(ctx, db.CreateCollaborationHandoffParams{IssueID: before.ID, ActorUserID: t.actor,
		FromUserID: before.AssigneeID, ToUserID: h.target, Purpose: h.purpose, Note: h.note,
		RequesterUserID: h.requester, ParentRequestID: h.parent, PreviousStatus: before.Status})
	if err != nil {
		return db.IssueHandoff{}, err
	}
	t.access.Issue = after
	return created, nil
}

func (t *collaborationWorkflowTx) supersedeInformation(ctx context.Context) error {
	if err := t.q.SupersedeCollaborationInformationRequests(ctx, t.access.Issue.ID); err != nil {
		return err
	}
	if err := t.q.ClearCollaborationInformationRequest(ctx, t.access.Issue.ID); err != nil {
		return err
	}
	t.access.InformationRequestID = pgtype.UUID{}
	return nil
}

func (t *collaborationWorkflowTx) depart(ctx context.Context, target pgtype.UUID, reason string) (CollaborationWorkflowReceipt, error) {
	participant, err := t.q.LockCollaborationParticipant(ctx, db.LockCollaborationParticipantParams{IssueID: t.access.Issue.ID, UserID: target})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && participant.RevokedAt.Valid) {
		return t.result("participant_not_found", http.StatusConflict), nil
	}
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	grants, err := t.q.LockCollaborationOwnerGrants(ctx, db.LockCollaborationOwnerGrantsParams{IssueID: t.access.Issue.ID, OwnerUserID: target})
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if target == t.access.Issue.AssigneeID {
		if err = t.changeHandler(ctx, t.access.Issue.CreatorID, "removal_return", ""); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
	}
	if _, err = t.q.RevokeCollaborationParticipant(ctx, db.RevokeCollaborationParticipantParams{IssueID: t.access.Issue.ID, UserID: target, Reason: reason}); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if err = t.q.RevokeCollaborationOwnerGrants(ctx, db.RevokeCollaborationOwnerGrantsParams{IssueID: t.access.Issue.ID, OwnerUserID: target, Reason: reason}); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if err = t.q.CancelCollaborationAuthorWaitingCommands(ctx, db.CancelCollaborationAuthorWaitingCommandsParams{SessionID: t.session.ID, AuthorUserID: target, Reason: reason}); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	stopped, err := t.stopInvalidatedWork(ctx, target, 0, grants, reason)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	return t.finish(ctx, reason, target, stopped)
}

func (t *collaborationWorkflowTx) finish(ctx context.Context, code string, target pgtype.UUID, stopped *CollaborationQueueReceipt) (CollaborationWorkflowReceipt, error) {
	return t.finishWith(ctx, code, target, stopped, nil)
}

// finishWith adds event-only facts. The receipt itself stays minimal.
func (t *collaborationWorkflowTx) finishWith(ctx context.Context, code string, target pgtype.UUID, stopped *CollaborationQueueReceipt, facts map[string]any) (CollaborationWorkflowReceipt, error) {
	var err error
	t.access.Revision, err = t.q.AdvanceCollaborationWorkflowRevision(ctx, t.access.Issue.ID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	receipt := t.result(code, http.StatusOK)
	if stopped != nil {
		receipt.RunID, receipt.RunSettled, receipt.Execution = stopped.RunID, stopped.RunSettled, stopped
	}
	seq, err := t.q.AdvanceCollaborationEventSequence(ctx, t.access.Issue.ID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	event := map[string]any{"target_user_id": target, "receipt": receipt, "request_id": t.operation.RequestID}
	for key, value := range facts {
		event[key] = value
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if _, err = t.q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: t.access.Issue.ID, EventSeq: seq,
		ActorUserID: t.actor, Kind: code, Payload: payload}); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	recipients, err := t.q.ListCollaborationNotificationRecipients(ctx, t.access.Issue.ID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	for _, recipient := range recipients {
		if recipient.ID == t.actor || !t.validTarget(recipient.ID) {
			continue
		}
		if _, err = t.q.CreateCollaborationNotification(ctx, db.CreateCollaborationNotificationParams{RecipientUserID: recipient.ID,
			IssueID: t.access.Issue.ID, EventSeq: seq, Kind: code}); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
	}
	// Notifications store references only. A removed recipient gets a distinct
	// content-free kind; future delivery must never hydrate it with issue data.
	if code == "participant_removed" && target != t.actor {
		if _, err = t.q.CreateCollaborationNotification(ctx, db.CreateCollaborationNotificationParams{RecipientUserID: target,
			IssueID: t.access.Issue.ID, EventSeq: seq, Kind: "removed"}); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
	}
	return receipt, nil
}

func workflowRoleAllowed(t *collaborationWorkflowTx, kind string) bool {
	switch kind {
	case "invite_participant":
		return t.access.Capabilities.Invite
	case "transfer":
		return t.access.Capabilities.Transfer
	case "remove_participant":
		return t.access.Capabilities.RemoveParticipants
	case "leave":
		// The action produces explicit creator/handler refusals below.
		return t.access.Capabilities.ControlAI
	case "complete", "cancel", "reopen":
		// Check identity independently of current business state: a successful
		// close/reopen key must still replay after its state transition.
		return t.access.Capabilities.ControlAI && t.access.Issue.CreatorType == "member" && t.access.Issue.CreatorID == t.actor
	case "change_status", "edit_issue":
		return t.access.Capabilities.Edit
	case "request_information", "return_information", "submit_acceptance":
		// Directed handoffs check current responsibility inside the action, so a
		// successful key still replays after responsibility moves away.
		return t.access.Capabilities.ControlAI
	default:
		return false
	}
}

func (s *CollaborationService) mutateCollaborationWorkflow(ctx context.Context, p CollaborationWorkflowRequest, target pgtype.UUID, kind string, payload any,
	mutate func(*collaborationWorkflowTx) (CollaborationWorkflowReceipt, error)) (CollaborationWorkflowReceipt, error) {
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return CollaborationWorkflowReceipt{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil || !p.IssueID.Valid || p.RequestID == "" || len(p.RequestID) > 128 || p.ExpectedCollaborationRevision < 1 ||
		p.ExpectedLifecycleEpoch < 1 || p.ExpectedParticipantEpoch < 1 || p.ExpectedAccountEpoch < 1 ||
		((kind == "invite_participant" || kind == "transfer" || kind == "remove_participant" || kind == "request_information") && !target.Valid) {
		return CollaborationWorkflowReceipt{}, collaborationError(http.StatusBadRequest, "invalid_workflow_request")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	digest := sha256.Sum256(encoded)
	hash := hex.EncodeToString(digest[:])
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := s.TxStarter.Begin(ctx)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		receipt, err := s.workflowInTx(ctx, tx, actor, target, p, kind, hash, mutate)
		if err == nil {
			if err = tx.Commit(ctx); err != nil {
				rollbackCollaborationQueue(ctx, tx)
				return CollaborationWorkflowReceipt{}, fmt.Errorf("commit collaboration workflow: %w", err)
			}
			if receipt.Status >= 400 {
				return receipt, collaborationError(receipt.Status, receipt.Code)
			}
			return receipt, nil
		}
		rollbackCollaborationQueue(ctx, tx)
		var conflict *pgconn.PgError
		if !errors.As(err, &conflict) || (conflict.Code != "40001" && conflict.Code != "40P01" && conflict.Code != "55P03") {
			return CollaborationWorkflowReceipt{}, err
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(25+attempt*50+rand.IntN(26)) * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return CollaborationWorkflowReceipt{}, ctx.Err()
			}
		}
	}
	return CollaborationWorkflowReceipt{}, collaborationError(http.StatusServiceUnavailable, "collaboration_busy")
}

func (s *CollaborationService) workflowInTx(ctx context.Context, tx pgx.Tx, actor, target pgtype.UUID, p CollaborationWorkflowRequest,
	kind, hash string, mutate func(*collaborationWorkflowTx) (CollaborationWorkflowReceipt, error)) (CollaborationWorkflowReceipt, error) {
	t := &collaborationWorkflowTx{collaborationQueueTx: &collaborationQueueTx{q: s.Queries.WithTx(tx), actor: actor}, tx: tx, users: make(map[pgtype.UUID]db.User)}
	if _, err := tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL READ COMMITTED"); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	hint, err := s.ResolveIssueAccess(ctx, t.q, p.IssueID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	idsParam := db.GetCollaborationWorkflowUserIDsParams{IssueID: p.IssueID, ActorID: actor, TargetID: target}
	ids, err := t.q.GetCollaborationWorkflowUserIDs(ctx, idsParam)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	users, err := t.q.LockCollaborationUsers(ctx, ids)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	for _, user := range users {
		t.users[user.ID] = user
	}
	if err = authorizeWorkflowAccount(ctx, t.q, actor, t.users[actor]); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if _, err = t.q.LockWorkspaceForChatSessionCreate(ctx, hint.Issue.WorkspaceID); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if err = t.q.LockIssueStatusCatalogShared(ctx, hint.Issue.WorkspaceID); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if collaborationStatusAction(kind) {
		t.catalog, err = t.q.LockCollaborationStatusEntries(ctx, hint.Issue.WorkspaceID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
	}
	if err = iteration.LockWorkspace(ctx, tx, hint.Issue.WorkspaceID); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	t.iteration, err = iteration.LockIssueIteration(ctx, tx, hint.Issue.WorkspaceID, p.IssueID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	issue, err := t.q.LockCollaborationIssue(ctx, p.IssueID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if issue.WorkspaceID != hint.Issue.WorkspaceID {
		return CollaborationWorkflowReceipt{}, collaborationError(http.StatusConflict, "stale_source")
	}
	currentIDs, err := t.q.GetCollaborationWorkflowUserIDs(ctx, idsParam)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	lockedIDs := make(map[pgtype.UUID]bool, len(ids))
	for _, id := range ids {
		lockedIDs[id] = true
	}
	for _, id := range currentIDs {
		if !lockedIDs[id] {
			return CollaborationWorkflowReceipt{}, &pgconn.PgError{Code: "40001", Message: "collaboration account lock set changed"}
		}
	}
	if _, err = t.q.LockIssueCollaboration(ctx, p.IssueID); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	t.session, err = t.q.LockCollaborationSession(ctx, p.IssueID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	t.channels, err = t.q.LockCollaborationChannels(ctx, t.session.ID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	t.targets, err = t.q.LockCollaborationTargets(ctx, t.session.ID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	t.access, err = s.ResolveIssueAccess(ctx, t.q, p.IssueID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	allowed := workflowRoleAllowed(t, kind)
	key := db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: p.IssueID, ActorUserID: actor, RequestID: p.RequestID}
	previous, err := t.q.GetCollaborationOperation(ctx, key)
	if err == nil {
		if !allowed {
			return CollaborationWorkflowReceipt{}, collaborationError(http.StatusForbidden, "forbidden")
		}
		if previous.Kind != kind || previous.PayloadHash != hash {
			return CollaborationWorkflowReceipt{}, collaborationError(http.StatusConflict, "idempotency_conflict")
		}
		receipt, err := decodeWorkflowReceipt(previous)
		if err != nil || receipt.StatusOptions == nil {
			return receipt, err
		}
		return t.projectStatusOptions(ctx, kind, receipt)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CollaborationWorkflowReceipt{}, err
	}
	t.operation, err = t.q.CreateCollaborationOperation(ctx, db.CreateCollaborationOperationParams{ScopeKind: "issue", ScopeID: p.IssueID,
		ActorUserID: actor, RequestID: p.RequestID, Kind: kind, PayloadHash: hash})
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	receipt := t.result("forbidden", http.StatusForbidden)
	if allowed {
		t.participant, err = t.q.LockCollaborationParticipant(ctx, db.LockCollaborationParticipantParams{IssueID: p.IssueID, UserID: actor})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.account, err = t.q.GetCollaborationAccountFence(ctx, actor)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if t.access.LifecycleEpoch != p.ExpectedLifecycleEpoch || t.participant.Epoch != p.ExpectedParticipantEpoch || t.account.Epoch != p.ExpectedAccountEpoch {
			receipt = t.result("stale_authority", http.StatusConflict)
		} else if t.access.Revision != p.ExpectedCollaborationRevision {
			receipt = t.result("stale_revision", http.StatusConflict)
		} else {
			receipt, err = mutate(t)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
		}
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	_, err = t.q.CompleteCollaborationOperation(ctx, db.CompleteCollaborationOperationParams{ID: t.operation.ID, ResultCode: receipt.Code,
		Result: encoded, ResolvedIssueID: p.IssueID})
	return receipt, err
}

func authorizeWorkflowAccount(ctx context.Context, q *db.Queries, actor pgtype.UUID, user db.User) error {
	if !user.ID.Valid || user.ID != actor || user.DisabledAt.Valid || auth.IsTemporarilyDisabledUser(util.UUIDToString(actor), user.Email.String) {
		return collaborationError(http.StatusForbidden, "forbidden")
	}
	if auth.PasswordMode() {
		ps, ok := auth.PasswordSessionFromContext(ctx)
		if !ok || ps.UserID != util.UUIDToString(actor) {
			return collaborationError(http.StatusForbidden, "forbidden")
		}
		if _, err := auth.CheckPasswordVersion(ctx, q, ps.UserID, ps.Version); err != nil {
			if errors.Is(err, auth.ErrPasswordSession) {
				return collaborationError(http.StatusForbidden, "forbidden")
			}
			return err
		}
	}
	return nil
}

func decodeWorkflowReceipt(op db.CollaborationOperation) (CollaborationWorkflowReceipt, error) {
	var receipt CollaborationWorkflowReceipt
	if !op.CompletedAt.Valid {
		return receipt, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	if err := json.Unmarshal(op.Result, &receipt); err != nil {
		return receipt, errors.New("invalid collaboration workflow receipt")
	}
	if receipt.Code == "" || receipt.Code != op.ResultCode.String || receipt.Status < 200 || receipt.Status > 599 || receipt.CollaborationRevision < 1 {
		return CollaborationWorkflowReceipt{}, errors.New("invalid collaboration workflow receipt")
	}
	return receipt, nil
}

// GetWorkflowOperation checks the real current account but needs no source
// membership or participant row. It can only return the actor's own minimal
// historical result, even after a successful voluntary leave.
func (s *CollaborationService) GetWorkflowOperation(ctx context.Context, issueID pgtype.UUID, requestID string) (CollaborationWorkflowOperationReceipt, error) {
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return CollaborationWorkflowOperationReceipt{}, collaborationError(http.StatusNotFound, "not_found")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil || !issueID.Valid || requestID == "" || len(requestID) > 128 {
		return CollaborationWorkflowOperationReceipt{}, collaborationError(http.StatusBadRequest, "invalid_workflow_request")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return CollaborationWorkflowOperationReceipt{}, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	q := s.Queries.WithTx(tx)
	users, err := q.LockCollaborationUsers(ctx, []pgtype.UUID{actor})
	if err != nil {
		return CollaborationWorkflowOperationReceipt{}, err
	}
	if len(users) != 1 {
		return CollaborationWorkflowOperationReceipt{}, collaborationError(http.StatusNotFound, "not_found")
	}
	if err = authorizeWorkflowAccount(ctx, q, actor, users[0]); err != nil {
		return CollaborationWorkflowOperationReceipt{}, err
	}
	op, err := q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: issueID, ActorUserID: actor, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return CollaborationWorkflowOperationReceipt{}, collaborationError(http.StatusNotFound, "not_found")
	}
	if err != nil {
		return CollaborationWorkflowOperationReceipt{}, err
	}
	switch op.Kind {
	case "invite_participant", "transfer", "remove_participant", "leave", "complete", "cancel", "reopen", "change_status", "edit_issue",
		"request_information", "return_information", "submit_acceptance":
	default:
		return CollaborationWorkflowOperationReceipt{}, collaborationError(http.StatusNotFound, "not_found")
	}
	r, err := decodeWorkflowReceipt(op)
	if err != nil {
		return CollaborationWorkflowOperationReceipt{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CollaborationWorkflowOperationReceipt{}, err
	}
	return CollaborationWorkflowOperationReceipt{Status: r.Status, Code: r.Code, CollaborationRevision: r.CollaborationRevision}, nil
}
