package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/admission"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// CollaborationCreationResult is a content-free result of the caller's original
// creation. It grants no source membership, issue access or attachment authority.
type CollaborationCreationResult struct {
	RequestID         string      `json:"request_id"`
	SourceWorkspaceID pgtype.UUID `json:"source_workspace_id"`
	ActorUserID       pgtype.UUID `json:"actor_user_id"`
	Outcome           string      `json:"outcome"`
	Code              string      `json:"code"`
}

// This private error marks a receipt staged by CreateInTx. Only the transaction
// owner can make it durable; ordinary authorization/dependency errors never do.
type collaborationCreationRejection struct{ cause *CollaborationError }

func (e *collaborationCreationRejection) Error() string { return e.cause.Error() }
func (e *collaborationCreationRejection) Unwrap() error { return e.cause }

func newCollaborationCreationRejection(code string) *collaborationCreationRejection {
	return &collaborationCreationRejection{cause: &CollaborationError{Status: collaborationCreationRejectionStatus(code), Code: code}}
}

func collaborationCreationRejectionStatus(code string) int {
	switch code {
	case "account_unavailable":
		return http.StatusForbidden
	case "collaboration_unavailable":
		return http.StatusServiceUnavailable
	case "invalid_attachment_selection", "parent_issue_not_found", "project_not_found":
		return http.StatusBadRequest
	case "attachment_not_found":
		return http.StatusNotFound
	case "attachment_scope_mismatch", "attachment_draft_consumed", "triage_review_required":
		return http.StatusConflict
	case "issue_limit_reached":
		return http.StatusPaymentRequired
	default:
		return 0
	}
}

// Only known selection/admission errors are settled. In particular, a service
// unavailability error is not evidence that the creation gate was closed.
func collaborationCreationBusinessRejection(err error) string {
	var domain *CollaborationError
	if errors.As(err, &domain) {
		switch domain.Code {
		case "invalid_attachment_selection", "attachment_not_found", "attachment_scope_mismatch", "attachment_draft_consumed":
			if domain.Status == collaborationCreationRejectionStatus(domain.Code) {
				return domain.Code
			}
		}
	}
	if errors.Is(err, ErrParentIssueNotFound) {
		return "parent_issue_not_found"
	}
	if errors.Is(err, ErrProjectNotFound) {
		return "project_not_found"
	}
	var blocked *admission.Blocked
	if errors.As(err, &blocked) {
		return "triage_review_required"
	}
	var limit *IssueLimitReachedError
	if errors.As(err, &limit) {
		return "issue_limit_reached"
	}
	return ""
}

func rejectCollaborationCreation(ctx context.Context, q *db.Queries, op db.CollaborationOperation, code string) error {
	if collaborationCreationRejectionStatus(code) == 0 {
		return errors.New("invalid collaboration creation rejection")
	}
	result := CollaborationCreationResult{RequestID: op.RequestID, SourceWorkspaceID: op.ScopeID, ActorUserID: op.ActorUserID, Outcome: "rejected", Code: code}
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if _, err = q.CompleteCollaborationOperation(ctx, db.CompleteCollaborationOperationParams{ID: op.ID, ResultCode: code, Result: encoded}); err != nil {
		return err
	}
	return newCollaborationCreationRejection(code)
}

func decodeCollaborationCreationResult(op db.CollaborationOperation) (CollaborationCreationResult, error) {
	var empty CollaborationCreationResult
	if op.ScopeKind != "create" || op.Kind != "create" || !op.ScopeID.Valid || !op.ActorUserID.Valid || len(op.RequestID) == 0 || len(op.RequestID) > 128 {
		return empty, errors.New("invalid collaboration creation operation")
	}
	if !op.CompletedAt.Valid {
		return empty, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	out := CollaborationCreationResult{RequestID: op.RequestID, SourceWorkspaceID: op.ScopeID, ActorUserID: op.ActorUserID, Code: op.ResultCode.String}
	if op.ResultCode.Valid && out.Code == "created" && op.ResolvedIssueID.Valid {
		out.Outcome = "created"
		return out, nil
	}
	if !op.ResultCode.Valid || op.ResolvedIssueID.Valid || collaborationCreationRejectionStatus(out.Code) == 0 {
		return empty, errors.New("invalid collaboration creation result")
	}
	out.Outcome = "rejected"
	var saved CollaborationCreationResult
	if json.Unmarshal(op.Result, &saved) != nil || saved != out {
		return empty, errors.New("invalid collaboration creation rejection result")
	}
	return out, nil
}

// GetCreationResult reads only the authenticated actor's immutable result. It
// deliberately does not expose the issue or require continued source membership.
func (s *CollaborationService) GetCreationResult(ctx context.Context, workspaceID pgtype.UUID, requestID string) (CollaborationCreationResult, error) {
	var empty CollaborationCreationResult
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return empty, collaborationError(http.StatusNotFound, "not_found")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil || !workspaceID.Valid || len(requestID) == 0 || len(requestID) > 128 {
		return empty, collaborationError(http.StatusBadRequest, "invalid_collaboration_creation")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return empty, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	q := s.Queries.WithTx(tx)
	users, err := q.LockCollaborationUsers(ctx, []pgtype.UUID{actor})
	if err != nil {
		return empty, err
	}
	if len(users) != 1 {
		return empty, collaborationError(http.StatusNotFound, "not_found")
	}
	if err = authorizeWorkflowAccount(ctx, q, actor, users[0]); err != nil {
		return empty, err
	}
	op, err := q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "create", ScopeID: workspaceID, ActorUserID: actor, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && op.Kind != "create" {
		return empty, collaborationError(http.StatusNotFound, "operation_not_found")
	}
	if err != nil {
		return empty, err
	}
	out, err := decodeCollaborationCreationResult(op)
	if err != nil {
		return empty, err
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, fmt.Errorf("read collaboration creation result: %w", err)
	}
	return out, nil
}
