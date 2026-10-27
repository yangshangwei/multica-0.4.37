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
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func collaborationMemoryKind(kind string) bool {
	return kind == "memory_update" || kind == "memory_restore" || kind == "memory_purge"
}

// mutateCollaborationMemory owns one human shared-context write. It follows
// the content transaction (own counter, original-key ledger, access resolved
// again under locks) and never advances the issue's collaboration revision.
func (s *CollaborationService) mutateCollaborationMemory(ctx context.Context, issueID pgtype.UUID, requestID, kind string, payload any,
	mutate func(*collaborationMemoryTx) (CollaborationMemoryReceipt, error)) (CollaborationMemoryReceipt, error) {
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil || !issueID.Valid || !collaborationMemoryKind(kind) {
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusBadRequest, "invalid_memory_request")
	}
	if requestID == "" || len(requestID) > 128 {
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusBadRequest, "invalid_request_id")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	digest := sha256.Sum256(encoded)
	hash := hex.EncodeToString(digest[:])
	for attempt := 0; attempt < 3; attempt++ {
		tx, err := s.TxStarter.Begin(ctx)
		if err != nil {
			return CollaborationMemoryReceipt{}, err
		}
		receipt, err := s.collaborationMemoryInTx(ctx, tx, actor, issueID, requestID, kind, hash, mutate)
		if err == nil {
			if err = tx.Commit(ctx); err != nil {
				rollbackCollaborationQueue(ctx, tx)
				return CollaborationMemoryReceipt{}, fmt.Errorf("commit collaboration memory: %w", err)
			}
			if receipt.Status >= 400 {
				return receipt, collaborationError(receipt.Status, receipt.Code)
			}
			return receipt, nil
		}
		rollbackCollaborationQueue(ctx, tx)
		var conflict *pgconn.PgError
		if !errors.As(err, &conflict) || (conflict.Code != "40001" && conflict.Code != "40P01" && conflict.Code != "55P03") {
			return CollaborationMemoryReceipt{}, err
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(25+attempt*50+rand.IntN(26)) * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return CollaborationMemoryReceipt{}, ctx.Err()
			}
		}
	}
	return CollaborationMemoryReceipt{}, collaborationError(http.StatusServiceUnavailable, "collaboration_busy")
}

func (s *CollaborationService) collaborationMemoryInTx(ctx context.Context, tx pgx.Tx, actor, issueID pgtype.UUID, requestID, kind, hash string,
	mutate func(*collaborationMemoryTx) (CollaborationMemoryReceipt, error)) (CollaborationMemoryReceipt, error) {
	t := &collaborationMemoryTx{q: s.Queries.WithTx(tx), actor: actor, maxBytes: s.MemoryLimit()}
	hint, err := s.ResolveIssueAccess(ctx, t.q, issueID)
	if err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	// The lock order is a prefix of the content and workflow writers: user,
	// source workspace, membership, status catalog, issue, collaboration, session.
	users, err := t.q.LockCollaborationUsers(ctx, []pgtype.UUID{actor})
	if err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	var actorRow db.User
	if len(users) == 1 {
		actorRow = users[0]
	}
	if err = authorizeWorkflowAccount(ctx, t.q, actor, actorRow); err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	if _, err = t.q.LockWorkspaceForChatSessionCreate(ctx, hint.Issue.WorkspaceID); err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	if _, err = t.q.LockCollaborationSourceMember(ctx, db.LockCollaborationSourceMemberParams{WorkspaceID: hint.Issue.WorkspaceID, UserID: actor}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return CollaborationMemoryReceipt{}, err
	}
	if err = t.q.LockIssueStatusCatalogShared(ctx, hint.Issue.WorkspaceID); err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	issue, err := t.q.LockCollaborationIssue(ctx, issueID)
	if err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	if issue.WorkspaceID != hint.Issue.WorkspaceID {
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusConflict, "stale_source")
	}
	if _, err = t.q.LockIssueCollaboration(ctx, issueID); err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	if t.session, err = t.q.LockCollaborationSession(ctx, issueID); err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	if t.access, err = s.ResolveIssueAccess(ctx, t.q, issueID); err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	// Role refusals are not sealed: restoring authority must not replay a
	// refusal, and a former writer cannot use replay to probe current state.
	allowed := t.access.Capabilities.ControlAI
	if kind == "memory_purge" {
		allowed = collaborationMemoryInitiator(t.access, actor)
	}
	if !allowed {
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusForbidden, "forbidden")
	}
	previous, err := t.q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{
		ScopeKind: "issue", ScopeID: issueID, ActorUserID: actor, RequestID: requestID})
	if err == nil {
		if previous.Kind != kind || previous.PayloadHash != hash {
			return CollaborationMemoryReceipt{}, collaborationError(http.StatusConflict, "idempotency_conflict")
		}
		return decodeCollaborationMemoryReceipt(previous, issueID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return CollaborationMemoryReceipt{}, err
	}
	t.operation, err = t.q.CreateCollaborationOperation(ctx, db.CreateCollaborationOperationParams{
		ScopeKind: "issue", ScopeID: issueID, ActorUserID: actor, RequestID: requestID, Kind: kind, PayloadHash: hash})
	if err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	receipt, err := mutate(t)
	if err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	_, err = t.q.CompleteCollaborationOperation(ctx, db.CompleteCollaborationOperationParams{
		ID: t.operation.ID, ResultCode: receipt.Code, Result: encoded, ResolvedIssueID: issueID})
	return receipt, err
}

func decodeCollaborationMemoryReceipt(op db.CollaborationOperation, issueID pgtype.UUID) (CollaborationMemoryReceipt, error) {
	if !op.CompletedAt.Valid {
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	var receipt CollaborationMemoryReceipt
	if err := json.Unmarshal(op.Result, &receipt); err != nil || receipt.Code == "" || receipt.Code != op.ResultCode.String ||
		receipt.IssueID != issueID || receipt.Action != op.Kind || receipt.RequestID != op.RequestID ||
		receipt.Status < 200 || receipt.Status > 599 || receipt.ProtocolVersion != CollaborationMemoryProtocolVersion {
		return CollaborationMemoryReceipt{}, errors.New("invalid collaboration memory receipt")
	}
	return receipt, nil
}

// GetMemoryOperation recovers the actor's own original receipt after an
// uncertain response. The receipt is content-free, so it survives access loss.
// A missing key is operation_not_found, never the issue-denial not_found.
func (s *CollaborationService) GetMemoryOperation(ctx context.Context, issueID pgtype.UUID, requestID string) (CollaborationMemoryReceipt, error) {
	if !issueID.Valid || requestID == "" || len(requestID) > 128 {
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusBadRequest, "invalid_request_id")
	}
	viewer, err := collaborationNotificationViewer(ctx, s.Queries)
	if err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	op, err := s.Queries.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{
		ScopeKind: "issue", ScopeID: issueID, ActorUserID: viewer, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !collaborationMemoryKind(op.Kind) {
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusNotFound, "operation_not_found")
	}
	if err != nil {
		return CollaborationMemoryReceipt{}, err
	}
	receipt, err := decodeCollaborationMemoryReceipt(op, issueID)
	if err != nil {
		var domain *CollaborationError
		if errors.As(err, &domain) {
			return CollaborationMemoryReceipt{}, err
		}
		return CollaborationMemoryReceipt{}, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	return receipt, nil
}
