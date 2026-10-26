package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// AdminCollaborationService owns only R38/R40 lifecycle administration. It
// never grants ordinary collaboration participation or resource authority.
type AdminCollaborationService struct {
	Queries   *db.Queries
	TxStarter TxStarter
	Enabled   bool
}

func NewAdminCollaborationService(q *db.Queries, tx TxStarter, enabled bool) *AdminCollaborationService {
	return &AdminCollaborationService{Queries: q, TxStarter: tx, Enabled: enabled}
}

type AdminCollaborationLifecycleParams struct {
	OrganizationID, IssueID, IdempotencyKey pgtype.UUID
	ExpectedCollaborationRevision           int64
	Action, ExpectedStatus, TargetStatus    string
	StatusOptionsVersion                    string
	Reason, RequestID                       string
	SuccessorUserID                         *pgtype.UUID
}

type AdminCollaborationReceipt struct {
	IssueID               pgtype.UUID `json:"issue_id"`
	RequestID             string      `json:"request_id"`
	Action                string      `json:"action"`
	Code                  string      `json:"code"`
	CollaborationRevision int64       `json:"collaboration_revision,string"`
	CreatorUserID         pgtype.UUID `json:"creator_user_id"`
	PreviousHandlerUserID pgtype.UUID `json:"previous_handler_user_id"`
	HandlerUserID         pgtype.UUID `json:"handler_user_id"`
	PreviousStatus        string      `json:"previous_status"`
	Status                string      `json:"status"`
	QueueState            string      `json:"queue_state"`
	ControlVersion        int64       `json:"control_version,string"`
	UnsettledRuns         int64       `json:"unsettled_runs"`
}

type AdminCollaborationResult struct {
	Operation db.AdminOperation         `json:"-"`
	Result    AdminCollaborationReceipt `json:"result"`
	// MemoryResult replaces Result for a shared-context purge operation.
	MemoryResult *AdminCollaborationMemoryReceipt `json:"-"`
	Replayed     bool                             `json:"-"`
	ParentWakeup *CollaborationParentWakeup       `json:"-"`
}

func (s *AdminCollaborationService) authorize(ctx context.Context, q *db.Queries, organization pgtype.UUID) (PlatformAdminIdentity, error) {
	if s == nil || !s.Enabled || !auth.PasswordMode() {
		return PlatformAdminIdentity{}, platformError(403, "admin_mode_disabled", "Collaboration administration is not enabled")
	}
	principal, ok := collaboration.PrincipalFromContext(ctx)
	session, hasSession := auth.PasswordSessionFromContext(ctx)
	if !ok || !principal.Human() || principal.Kind != auth.CredentialJWT || !hasSession || principal.UserID != session.UserID {
		return PlatformAdminIdentity{}, platformError(403, "admin_session_required", "A complete password login is required")
	}
	// Even the minimal read/candidate surface is part of the R38 management
	// exception. A platform observer does not acquire access to these objects.
	actor, err := authorizePlatformAdmin(ctx, q, true)
	if err != nil {
		return PlatformAdminIdentity{}, err
	}
	if !organization.Valid || actor.OrganizationID != organization {
		return PlatformAdminIdentity{}, platformError(403, "admin_scope_forbidden", "The organization is outside the administrative scope")
	}
	return actor, nil
}

func adminCollaborationCode(action string) string {
	switch action {
	case "complete":
		return "completed"
	case "cancel":
		return "cancelled"
	case "reopen":
		return "reopened"
	default:
		return ""
	}
}

func validateAdminCollaborationLifecycle(p AdminCollaborationLifecycleParams) error {
	if !p.IssueID.Valid || !p.IdempotencyKey.Valid || p.ExpectedCollaborationRevision < 1 || adminCollaborationCode(p.Action) == "" ||
		strings.TrimSpace(p.ExpectedStatus) == "" || len(p.ExpectedStatus) > 128 || !validateCollaborationStatusInput(p.TargetStatus, p.StatusOptionsVersion) ||
		(p.SuccessorUserID != nil && !p.SuccessorUserID.Valid) {
		return platformError(400, "invalid_lifecycle_request", "An exact lifecycle action, revision and status confirmation are required")
	}
	return validateAdminReason(p.Reason, p.RequestID)
}

func adminCollaborationHash(p AdminCollaborationLifecycleParams) (string, error) {
	// RequestID is the transport trace, not caller intent. Enumerate the
	// non-secret payload, including the optional successor's exact identity.
	payload, err := json.Marshal(struct {
		OrganizationID       pgtype.UUID  `json:"organization_id"`
		IssueID              pgtype.UUID  `json:"issue_id"`
		Action               string       `json:"action"`
		Revision             int64        `json:"expected_collaboration_revision"`
		ExpectedStatus       string       `json:"expected_status"`
		TargetStatus         string       `json:"target_status"`
		StatusOptionsVersion string       `json:"status_options_version"`
		Reason               string       `json:"reason"`
		SuccessorUserID      *pgtype.UUID `json:"successor_user_id,omitempty"`
	}{p.OrganizationID, p.IssueID, p.Action, p.ExpectedCollaborationRevision, p.ExpectedStatus, p.TargetStatus,
		p.StatusOptionsVersion, strings.TrimSpace(p.Reason), p.SuccessorUserID})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func adminCollaborationServiceError(err error) error {
	var collaborationErr *CollaborationError
	if errors.As(err, &collaborationErr) {
		return platformError(collaborationErr.Status, collaborationErr.Code, "The collaboration action is unavailable in its current state")
	}
	return err
}

// ChangeLifecycle owns one transaction, including both operation ledgers. It
// does not call the ordinary transaction-owning lifecycle or handoff wrappers,
// change the actor into the creator, or create an administrator participant.
func (s *AdminCollaborationService) ChangeLifecycle(ctx context.Context, p AdminCollaborationLifecycleParams) (out AdminCollaborationResult, resultErr error) {
	defer func() { resultErr = adminCollaborationServiceError(resultErr) }()
	actor, err := s.authorize(ctx, s.Queries, p.OrganizationID)
	if err != nil {
		return out, err
	}
	if err = validateAdminCollaborationLifecycle(p); err != nil {
		return out, err
	}
	hash, err := adminCollaborationHash(p)
	if err != nil {
		return out, err
	}
	if s.TxStarter == nil {
		return out, platformError(503, "admin_unavailable", "Collaboration administration is unavailable")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	t, err := s.lockLifecycle(ctx, tx, actor, p)
	if err != nil {
		return out, err
	}
	creator := t.users[t.access.Issue.CreatorID]
	if !adminCollaborationAccountDisabled(creator.ID, creator.Email, creator.DisabledAt) {
		return out, platformError(409, "creator_not_disabled", "Only a disabled creator permits lifecycle administration")
	}
	op, replayed, err := NewPlatformAdminService(s.Queries, s.TxStarter).CreateOperationInTx(ctx, tx, db.CreateAdminOperationParams{
		OrganizationID: p.OrganizationID, TargetKind: "collaboration_issue", TargetID: p.IssueID,
		Kind: "collaboration." + p.Action, IdempotencyKey: p.IdempotencyKey, PayloadHash: hash,
		Reason: strings.TrimSpace(p.Reason), State: "applied", ResultCode: adminCollaborationCode(p.Action),
		AppliedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return out, err
	}
	if replayed {
		receipt, err := loadAdminCollaborationReceipt(ctx, t.q, op)
		return AdminCollaborationResult{Operation: op, Result: receipt, Replayed: true}, err
	}
	if t.access.Revision != p.ExpectedCollaborationRevision {
		return out, platformError(409, "stale_revision", "The collaboration changed; refresh its state")
	}
	if t.access.Issue.Status != p.ExpectedStatus {
		return out, platformError(409, "stale_status", "The current status changed; refresh its state")
	}
	if p.Action == "reopen" {
		if !collaboration.TerminalCategory(t.access.StatusCategory) || t.session.QueueState != "closed" {
			return out, platformError(409, "issue_not_closed", "Only a closed collaboration can be reopened")
		}
	} else if collaboration.TerminalCategory(t.access.StatusCategory) || t.session.QueueState == "closed" {
		return out, platformError(409, "issue_closed", "The collaboration is already closed")
	} else if !collaboration.NonterminalCategory(t.access.StatusCategory) || (t.session.QueueState != "active" && t.session.QueueState != "paused") {
		return out, platformError(409, "status_unavailable", "The collaboration status is unavailable")
	}
	if refusal, err := t.validateStatus(ctx, p.Action, p.TargetStatus, p.StatusOptionsVersion); err != nil {
		return out, err
	} else if refusal != nil {
		return out, platformError(refusal.Status, refusal.Code, "The status choices changed; refresh before submitting")
	}
	if err = validateAdminCollaborationSuccessor(ctx, t, p); err != nil {
		return out, err
	}
	t.operation, err = t.q.CreateCollaborationOperation(ctx, db.CreateCollaborationOperationParams{
		ScopeKind: "issue", ScopeID: p.IssueID, ActorUserID: actor.UserID,
		RequestID: util.UUIDToString(p.IdempotencyKey), Kind: "admin_" + p.Action, PayloadHash: hash,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, platformError(409, "idempotency_conflict", "The request key already identifies another collaboration action")
	}
	if err != nil {
		return out, err
	}
	before := adminCollaborationAuditStateFor(t)
	if p.SuccessorUserID != nil {
		if err = t.changeHandler(ctx, *p.SuccessorUserID, "admin_reopen", strings.TrimSpace(p.Reason)); err != nil {
			return out, err
		}
	}
	receipt, err := t.changeLifecycle(ctx, p.Action, p.TargetStatus)
	if err != nil {
		return out, err
	}
	unsettled, err := t.q.CountAdminCollaborationUnsettledRuns(ctx, t.session.ID)
	if err != nil {
		return out, err
	}
	result := AdminCollaborationReceipt{IssueID: p.IssueID, RequestID: t.operation.RequestID, Action: p.Action,
		Code: receipt.Code, CollaborationRevision: receipt.CollaborationRevision, CreatorUserID: before.CreatorUserID,
		PreviousHandlerUserID: before.HandlerUserID, HandlerUserID: t.access.Issue.AssigneeID,
		PreviousStatus: before.Status, Status: t.access.Issue.Status, QueueState: receipt.QueueState,
		ControlVersion: receipt.ControlVersion, UnsettledRuns: unsettled}
	after := adminCollaborationAuditStateFor(t)
	if err = recordAdminCollaborationHistory(ctx, t, op, p.RequestID, before, after); err != nil {
		return out, err
	}
	encoded, err := json.Marshal(adminCollaborationStoredResult{OperationID: op.ID, Result: result})
	if err != nil {
		return out, err
	}
	if _, err = t.q.CompleteCollaborationOperation(ctx, db.CompleteCollaborationOperationParams{
		ID: t.operation.ID, ResultCode: result.Code, Result: encoded, ResolvedIssueID: p.IssueID,
	}); err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		// Do not retry even a serialization-looking commit failure. The only
		// recovery path is the original actor/org/key and its recorded result.
		return out, fmt.Errorf("commit collaboration administration %s: %w", util.UUIDToString(op.ID), err)
	}
	return AdminCollaborationResult{Operation: op, Result: result, ParentWakeup: receipt.ParentWakeup}, nil
}

func (s *AdminCollaborationService) lockLifecycle(ctx context.Context, tx pgx.Tx, actor PlatformAdminIdentity, p AdminCollaborationLifecycleParams) (*collaborationWorkflowTx, error) {
	q := s.Queries.WithTx(tx)
	hint, err := q.GetAdminCollaborationIssue(ctx, db.GetAdminCollaborationIssueParams{OrganizationID: p.OrganizationID, IssueID: p.IssueID})
	if err != nil {
		return nil, err
	}
	var successor pgtype.UUID
	if p.SuccessorUserID != nil {
		successor = *p.SuccessorUserID
	}
	idsParam := db.GetCollaborationWorkflowUserIDsParams{IssueID: p.IssueID, ActorID: actor.UserID, TargetID: successor}
	ids, err := q.GetCollaborationWorkflowUserIDs(ctx, idsParam)
	if err != nil {
		return nil, err
	}
	if err = LockPlatformAdminMutation(ctx, tx, ids...); err != nil {
		return nil, err
	}
	if _, err = s.authorize(ctx, q, p.OrganizationID); err != nil {
		return nil, err
	}
	users, err := q.LockCollaborationUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	t := &collaborationWorkflowTx{collaborationQueueTx: &collaborationQueueTx{q: q, actor: actor.UserID}, tx: tx,
		users: make(map[pgtype.UUID]db.User, len(users))}
	for _, user := range users {
		t.users[user.ID] = user
	}
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, hint.SourceWorkspaceID); err != nil {
		return nil, err
	}
	org, err := q.LockAdminCollaborationOrganization(ctx, p.OrganizationID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (!org.Internal || org.State != "active") {
		return nil, platformError(503, "admin_unavailable", "The administration organization is unavailable")
	}
	if err != nil {
		return nil, err
	}
	if _, err = q.LockAdminCollaborationSourceScope(ctx, db.LockAdminCollaborationSourceScopeParams{WorkspaceID: hint.SourceWorkspaceID, OrganizationID: p.OrganizationID}); err != nil {
		return nil, err
	}
	if err = q.LockIssueStatusCatalogShared(ctx, hint.SourceWorkspaceID); err != nil {
		return nil, err
	}
	t.catalog, err = q.LockCollaborationStatusEntries(ctx, hint.SourceWorkspaceID)
	if err != nil {
		return nil, err
	}
	if err = iteration.LockWorkspace(ctx, tx, hint.SourceWorkspaceID); err != nil {
		return nil, err
	}
	t.iteration, err = iteration.LockIssueIteration(ctx, tx, hint.SourceWorkspaceID, p.IssueID)
	if err != nil {
		return nil, err
	}
	issue, err := q.LockCollaborationIssue(ctx, p.IssueID)
	if err != nil {
		return nil, err
	}
	if issue.WorkspaceID != hint.SourceWorkspaceID {
		return nil, platformError(409, "stale_source", "The source changed; refresh its state")
	}
	currentIDs, err := q.GetCollaborationWorkflowUserIDs(ctx, idsParam)
	if err != nil {
		return nil, err
	}
	for _, id := range currentIDs {
		if _, locked := t.users[id]; !locked {
			// Never acquire a newly discovered account out of sorted order, or
			// authorize a changed recipient set using the earlier snapshot.
			return nil, platformError(409, "stale_revision", "The collaboration participants changed; refresh its state")
		}
	}
	state, err := q.LockIssueCollaboration(ctx, p.IssueID)
	if err != nil {
		return nil, err
	}
	t.session, err = q.LockCollaborationSession(ctx, p.IssueID)
	if err != nil {
		return nil, err
	}
	t.channels, err = q.LockCollaborationChannels(ctx, t.session.ID)
	if err != nil {
		return nil, err
	}
	t.targets, err = q.LockCollaborationTargets(ctx, t.session.ID)
	if err != nil {
		return nil, err
	}
	current, err := q.GetAdminCollaborationIssue(ctx, db.GetAdminCollaborationIssueParams{OrganizationID: p.OrganizationID, IssueID: p.IssueID})
	if err != nil {
		return nil, err
	}
	if current.SourceWorkspaceID != issue.WorkspaceID || current.CreatorUserID != issue.CreatorID || current.HandlerUserID != issue.AssigneeID || current.SessionID != t.session.ID {
		return nil, platformError(503, "admin_unavailable", "The collaboration identity is unavailable")
	}
	t.access = CollaborationIssueAccess{Issue: issue, Revision: state.Revision, LifecycleEpoch: state.LifecycleEpoch,
		AuthorityEpoch: state.AuthorityEpoch, SessionID: t.session.ID, QueueState: t.session.QueueState,
		ControlVersion: t.session.ControlVersion, StatusCategory: current.StatusCategory, InformationRequestID: state.ActiveInformationRequestID}
	return t, nil
}

func validateAdminCollaborationSuccessor(ctx context.Context, t *collaborationWorkflowTx, p AdminCollaborationLifecycleParams) error {
	handler := t.users[t.access.Issue.AssigneeID]
	required := p.Action == "reopen" && adminCollaborationAccountDisabled(handler.ID, handler.Email, handler.DisabledAt)
	if !required && p.SuccessorUserID != nil {
		return platformError(400, "successor_forbidden", "A successor is only allowed when reopening with both creator and handler disabled")
	}
	if !required {
		return nil
	}
	if p.SuccessorUserID == nil {
		return platformError(409, "successor_required", "An existing active participant must be selected")
	}
	participant, err := t.q.LockCollaborationParticipant(ctx, db.LockCollaborationParticipantParams{IssueID: p.IssueID, UserID: *p.SuccessorUserID})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && participant.RevokedAt.Valid {
		return platformError(409, "successor_unavailable", "The selected participant is no longer available")
	}
	if err != nil {
		return err
	}
	eligible, err := t.eligibleRecipient(ctx, *p.SuccessorUserID)
	if err != nil {
		return err
	}
	if !eligible {
		return platformError(409, "successor_unavailable", "The selected participant is no longer available")
	}
	return nil
}

type adminCollaborationAuditState struct {
	IssueID               pgtype.UUID `json:"issue_id"`
	SourceWorkspaceID     pgtype.UUID `json:"source_workspace_id"`
	CreatorUserID         pgtype.UUID `json:"creator_user_id"`
	HandlerUserID         pgtype.UUID `json:"handler_user_id"`
	Status                string      `json:"status"`
	CollaborationRevision int64       `json:"collaboration_revision,string"`
	LifecycleEpoch        int64       `json:"lifecycle_epoch,string"`
	QueueState            string      `json:"queue_state"`
	ControlVersion        int64       `json:"control_version,string"`
}

func adminCollaborationAuditStateFor(t *collaborationWorkflowTx) adminCollaborationAuditState {
	return adminCollaborationAuditState{IssueID: t.access.Issue.ID, SourceWorkspaceID: t.access.Issue.WorkspaceID,
		CreatorUserID: t.access.Issue.CreatorID, HandlerUserID: t.access.Issue.AssigneeID, Status: t.access.Issue.Status,
		CollaborationRevision: t.access.Revision, LifecycleEpoch: t.access.LifecycleEpoch,
		QueueState: t.session.QueueState, ControlVersion: t.session.ControlVersion}
}

func recordAdminCollaborationHistory(ctx context.Context, t *collaborationWorkflowTx, op db.AdminOperation, requestID string, before, after adminCollaborationAuditState) error {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	if err = recordRoleAudit(ctx, t.q, op, requestID, beforeJSON, afterJSON); err != nil {
		return err
	}
	seq, err := t.q.AdvanceCollaborationEventSequence(ctx, t.access.Issue.ID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		OperationID pgtype.UUID                  `json:"operation_id"`
		RequestID   string                       `json:"request_id"`
		Reason      string                       `json:"reason"`
		HandoffID   pgtype.UUID                  `json:"handoff_id"`
		Before      adminCollaborationAuditState `json:"before"`
		After       adminCollaborationAuditState `json:"after"`
	}{op.ID, t.operation.RequestID, op.Reason, t.handoffID, before, after})
	if err != nil {
		return err
	}
	_, err = t.q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: t.access.Issue.ID,
		EventSeq: seq, ActorUserID: op.ActorID, Kind: "admin_" + op.ResultCode, Payload: payload})
	return err
}
