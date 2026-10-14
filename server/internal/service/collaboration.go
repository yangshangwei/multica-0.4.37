package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type CollaborationError struct {
	Status int
	Code   string
}

func (e *CollaborationError) Error() string { return e.Code }

func collaborationError(status int, code string) error {
	return &CollaborationError{Status: status, Code: code}
}

type CollaborationService struct {
	Queries   *db.Queries
	TxStarter TxStarter
	Issues    *IssueService
}

func NewCollaborationService(q *db.Queries, tx TxStarter, issues *IssueService) *CollaborationService {
	return &CollaborationService{Queries: q, TxStarter: tx, Issues: issues}
}

// CollaborationIssueAccess is internal service data, not an HTTP response.
// Adapters must use a dedicated DTO; the generic Issue wire remains unchanged.
type CollaborationIssueAccess struct {
	Issue            db.Issue    `json:"-"`
	Revision         int64       `json:"-"`
	LifecycleEpoch   int64       `json:"-"`
	SessionID        pgtype.UUID `json:"-"`
	QueueState       string      `json:"-"`
	ControlVersion   int64       `json:"-"`
	ParticipantEpoch pgtype.Int8 `json:"-"`
	AccountEpoch     pgtype.Int8 `json:"-"`
	StatusCategory   string      `json:"-"`
	// InformationRequestID is the open top of the request chain, if any.
	InformationRequestID pgtype.UUID                `json:"-"`
	Capabilities         collaboration.Capabilities `json:"capabilities"`
}

func (s *CollaborationService) ResolveIssueAccess(ctx context.Context, q *db.Queries, issueID pgtype.UUID) (CollaborationIssueAccess, error) {
	var result CollaborationIssueAccess
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return result, collaborationError(http.StatusNotFound, "not_found")
	}
	viewerID, err := util.ParseUUID(principal.UserID)
	if err != nil {
		return result, collaborationError(http.StatusNotFound, "not_found")
	}
	if q == nil {
		q = s.Queries
	}
	row, err := q.GetCollaborationAccess(ctx, db.GetCollaborationAccessParams{ViewerID: viewerID, IssueID: issueID})
	if errors.Is(err, pgx.ErrNoRows) {
		return result, collaborationError(http.StatusNotFound, "not_found")
	}
	if err != nil {
		return result, fmt.Errorf("resolve collaboration access: %w", err)
	}
	active := row.AccountActive && !auth.IsTemporarilyDisabledUser(principal.UserID, row.ViewerEmail.String)
	if auth.PasswordMode() {
		session, hasSession := auth.PasswordSessionFromContext(ctx)
		if !hasSession || session.UserID != principal.UserID {
			active = false
		} else if _, err := auth.CheckPasswordVersion(ctx, q, principal.UserID, session.Version); err != nil {
			if !errors.Is(err, auth.ErrPasswordSession) {
				return result, err
			}
			active = false
		}
	}
	caps := collaboration.CapabilitiesFor(principal, collaboration.AccessState{
		AccountActive: active, SourceMember: row.SourceMember, Participant: row.Participant,
		Creator:              row.Issue.CreatorType == "member" && row.Issue.CreatorID == viewerID,
		Handler:              row.Issue.AssigneeType.Valid && row.Issue.AssigneeType.String == "member" && row.Issue.AssigneeID == viewerID,
		InformationRecipient: row.InformationRequestID.Valid && row.InformationRecipientID == viewerID,
		StatusCategory:       row.StatusCategory, QueueState: row.QueueState,
	})
	if !caps.Read {
		return result, collaborationError(http.StatusNotFound, "not_found")
	}
	return CollaborationIssueAccess{Issue: row.Issue, Revision: row.CollaborationRevision, LifecycleEpoch: row.LifecycleEpoch,
		SessionID: row.SessionID, QueueState: row.QueueState, ControlVersion: row.ControlVersion,
		ParticipantEpoch: row.ParticipantEpoch, AccountEpoch: row.AccountEpoch, StatusCategory: row.StatusCategory,
		InformationRequestID: row.InformationRequestID, Capabilities: caps}, nil
}

type CollaborationCreateParams struct {
	SourceWorkspaceID pgtype.UUID
	HandlerID         pgtype.UUID
	RequestID         string
	Title             string
	Description       string
	ProjectID         pgtype.UUID
	ParentIssueID     pgtype.UUID
}

func (s *CollaborationService) Create(ctx context.Context, p CollaborationCreateParams) (CollaborationIssueAccess, error) {
	if principal, ok := collaboration.PrincipalFromContext(ctx); !ok || !principal.Human() {
		return CollaborationIssueAccess{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	policy := ResolveIssueCountPolicy(ctx, s.Issues.Entitlements, p.SourceWorkspaceID)
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return CollaborationIssueAccess{}, err
	}
	defer tx.Rollback(ctx)
	result, err := s.CreateInTx(ctx, tx, p, policy)
	if err != nil {
		return CollaborationIssueAccess{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		// The caller must query the original key. Never retry an uncertain commit
		// with a new request identity or create an ordinary issue as fallback.
		return CollaborationIssueAccess{}, fmt.Errorf("commit collaboration creation: %w", err)
	}
	return result, nil
}

// CreateInTx borrows the transaction and a policy resolved before locks. It
// never commits, publishes ordinary issue events, or enqueues ordinary AI work.
func (s *CollaborationService) CreateInTx(ctx context.Context, tx pgx.Tx, p CollaborationCreateParams, policy IssueCountPolicy) (CollaborationIssueAccess, error) {
	var empty CollaborationIssueAccess
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return empty, collaborationError(http.StatusForbidden, "forbidden")
	}
	actorID, err := util.ParseUUID(principal.UserID)
	if err != nil || !p.SourceWorkspaceID.Valid || !p.HandlerID.Valid || strings.TrimSpace(p.Title) == "" || len(p.RequestID) == 0 || len(p.RequestID) > 128 {
		return empty, collaborationError(http.StatusBadRequest, "invalid_collaboration_creation")
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		return empty, err
	}
	digest := sha256.Sum256(encoded)
	payloadHash := hex.EncodeToString(digest[:])
	q := s.Queries.WithTx(tx)
	opKey := db.GetCollaborationOperationParams{ScopeKind: "create", ScopeID: p.SourceWorkspaceID, ActorUserID: actorID, RequestID: p.RequestID}
	// A completed request does not revalidate its old handler or require source
	// membership/creation to remain enabled. It only revalidates current access.
	previous, err := q.GetCollaborationOperation(ctx, opKey)
	if err == nil {
		return s.replayCollaborationCreation(ctx, q, previous, payloadHash)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return empty, err
	}
	ids := []pgtype.UUID{actorID}
	if actorID != p.HandlerID {
		ids = append(ids, p.HandlerID)
	}
	users, err := q.LockCollaborationUsers(ctx, ids)
	if err != nil {
		return empty, err
	}
	if len(users) != len(ids) {
		return empty, collaborationError(http.StatusForbidden, "forbidden")
	}
	for _, user := range users {
		if user.DisabledAt.Valid || auth.IsTemporarilyDisabledUser(util.UUIDToString(user.ID), user.Email.String) {
			return empty, collaborationError(http.StatusForbidden, "account_unavailable")
		}
	}
	if auth.PasswordMode() {
		ps, ok := auth.PasswordSessionFromContext(ctx)
		if !ok || ps.UserID != principal.UserID {
			return empty, collaborationError(http.StatusForbidden, "forbidden")
		}
		if _, err := auth.CheckPasswordVersion(ctx, q, principal.UserID, ps.Version); err != nil {
			return empty, err
		}
	}
	previous, err = q.GetCollaborationOperation(ctx, opKey)
	if err == nil {
		return s.replayCollaborationCreation(ctx, q, previous, payloadHash)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return empty, err
	}
	gate, err := q.LockCollaborationGateShared(ctx)
	if err != nil {
		return empty, err
	}
	if !gate.CreationEnabled {
		return empty, collaborationError(http.StatusServiceUnavailable, "collaboration_unavailable")
	}
	if _, err := q.LockWorkspaceForChatSessionCreate(ctx, p.SourceWorkspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return empty, collaborationError(http.StatusForbidden, "forbidden")
		}
		return empty, err
	}
	if _, err := q.LockCollaborationSourceMember(ctx, db.LockCollaborationSourceMemberParams{WorkspaceID: p.SourceWorkspaceID, UserID: actorID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return empty, collaborationError(http.StatusForbidden, "forbidden")
		}
		return empty, err
	}
	op, err := q.CreateCollaborationOperation(ctx, db.CreateCollaborationOperationParams{ScopeKind: "create", ScopeID: p.SourceWorkspaceID, ActorUserID: actorID, RequestID: p.RequestID, Kind: "create", PayloadHash: payloadHash})
	if err != nil {
		return empty, err
	}
	for _, user := range users {
		if _, err := q.EnsureCollaborationAccountFence(ctx, user.ID); err != nil {
			return empty, err
		}
	}
	created, err := s.Issues.CreateInTx(ctx, tx, IssueCreateParams{
		WorkspaceID: p.SourceWorkspaceID, Title: p.Title, Description: pgtype.Text{String: p.Description, Valid: true},
		Status: "todo", Priority: "none", AssigneeType: pgtype.Text{String: "member", Valid: true}, AssigneeID: p.HandlerID,
		CreatorType: "member", CreatorID: actorID, ProjectID: p.ProjectID, ParentIssueID: p.ParentIssueID, AllowDuplicate: true,
	}, policy)
	if err != nil {
		return empty, err
	}
	if _, err = q.CreateIssueCollaboration(ctx, created.Issue.ID); err != nil {
		return empty, err
	}
	for _, user := range users {
		if _, err = q.CreateCollaborationParticipant(ctx, db.CreateCollaborationParticipantParams{IssueID: created.Issue.ID, UserID: user.ID}); err != nil {
			return empty, err
		}
	}
	if _, err = q.CreateCollaborationSession(ctx, created.Issue.ID); err != nil {
		return empty, err
	}
	seq, err := q.AdvanceCollaborationEventSequence(ctx, created.Issue.ID)
	if err != nil {
		return empty, err
	}
	payload, err := json.Marshal(map[string]string{"handler_user_id": util.UUIDToString(p.HandlerID)})
	if err != nil {
		return empty, err
	}
	if _, err = q.CreateCollaborationEvent(ctx, db.CreateCollaborationEventParams{IssueID: created.Issue.ID, EventSeq: seq, ActorUserID: actorID, Kind: "created", Payload: payload}); err != nil {
		return empty, err
	}
	if actorID != p.HandlerID {
		if _, err = q.CreateCollaborationNotification(ctx, db.CreateCollaborationNotificationParams{RecipientUserID: p.HandlerID, IssueID: created.Issue.ID, EventSeq: seq, Kind: "assigned"}); err != nil {
			return empty, err
		}
	}
	if _, err = q.CompleteCollaborationCreation(ctx, db.CompleteCollaborationCreationParams{ID: op.ID, ResolvedIssueID: created.Issue.ID}); err != nil {
		return empty, err
	}
	return s.ResolveIssueAccess(ctx, q, created.Issue.ID)
}

func (s *CollaborationService) replayCollaborationCreation(ctx context.Context, q *db.Queries, op db.CollaborationOperation, payloadHash string) (CollaborationIssueAccess, error) {
	if !op.ResolvedIssueID.Valid || !op.CompletedAt.Valid {
		return CollaborationIssueAccess{}, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	access, err := s.ResolveIssueAccess(ctx, q, op.ResolvedIssueID)
	if err != nil {
		return CollaborationIssueAccess{}, err
	}
	if op.PayloadHash != payloadHash {
		return CollaborationIssueAccess{}, collaborationError(http.StatusConflict, "idempotency_conflict")
	}
	return access, nil
}

// GetCreation recovers a lost response without requiring continued membership
// in the source workspace. Current object authority is still required.
func (s *CollaborationService) GetCreation(ctx context.Context, workspaceID pgtype.UUID, requestID string) (CollaborationIssueAccess, error) {
	p, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !p.Human() {
		return CollaborationIssueAccess{}, collaborationError(http.StatusNotFound, "not_found")
	}
	actor, err := util.ParseUUID(p.UserID)
	if err != nil {
		return CollaborationIssueAccess{}, collaborationError(http.StatusNotFound, "not_found")
	}
	op, err := s.Queries.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "create", ScopeID: workspaceID, ActorUserID: actor, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return CollaborationIssueAccess{}, collaborationError(http.StatusNotFound, "not_found")
	}
	if err != nil {
		return CollaborationIssueAccess{}, err
	}
	if !op.ResolvedIssueID.Valid || !op.CompletedAt.Valid {
		return CollaborationIssueAccess{}, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	return s.ResolveIssueAccess(ctx, nil, op.ResolvedIssueID)
}

// Reorder changes source-workspace layout only. It does not take a session
// lock, advance the collaboration version, emit activity, or enqueue work.
func (s *CollaborationService) Reorder(ctx context.Context, issueID pgtype.UUID, position float64, expectedRevision *int64) (db.Issue, error) {
	p, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !p.Human() {
		return db.Issue{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	if math.IsNaN(position) || math.IsInf(position, 0) || (expectedRevision != nil && *expectedRevision < 1) {
		return db.Issue{}, collaborationError(http.StatusBadRequest, "invalid_position")
	}
	hint, err := s.ResolveIssueAccess(ctx, nil, issueID)
	if err != nil {
		return db.Issue{}, err
	}
	actor, err := util.ParseUUID(p.UserID)
	if err != nil {
		return db.Issue{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return db.Issue{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if _, err := q.LockCollaborationUsers(ctx, []pgtype.UUID{actor}); err != nil {
		return db.Issue{}, err
	}
	if _, err := q.LockWorkspaceForChatSessionCreate(ctx, hint.Issue.WorkspaceID); err != nil {
		return db.Issue{}, err
	}
	if _, err := q.LockCollaborationSourceMember(ctx, db.LockCollaborationSourceMemberParams{WorkspaceID: hint.Issue.WorkspaceID, UserID: actor}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Issue{}, collaborationError(http.StatusForbidden, "forbidden")
		}
		return db.Issue{}, err
	}
	current, err := s.ResolveIssueAccess(ctx, q, issueID)
	if err != nil {
		return db.Issue{}, err
	}
	if !current.Capabilities.Reorder {
		return db.Issue{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	var expected pgtype.Int8
	if expectedRevision != nil {
		expected = pgtype.Int8{Int64: *expectedRevision, Valid: true}
	}
	result, err := q.UpdateCollaborationPosition(ctx, db.UpdateCollaborationPositionParams{IssueID: issueID, WorkspaceID: current.Issue.WorkspaceID, Position: position, ExpectedRevision: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Issue{}, collaborationError(http.StatusConflict, "stale_revision")
	}
	if err != nil {
		return db.Issue{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Issue{}, err
	}
	return result, nil
}
