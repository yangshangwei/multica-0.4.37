package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// An actor may recover only their original cancellation. Current content access
// determines whether the input identity is returned or the receipt is redacted.
func (s *CollaborationService) GetCommandOperation(ctx context.Context, issueID pgtype.UUID, key string) (CollaborationQueueReceipt, error) {
	var empty CollaborationQueueReceipt
	principal, ok := collaboration.PrincipalFromContext(ctx)
	if !ok || !principal.Human() {
		return empty, collaborationError(http.StatusNotFound, "not_found")
	}
	actor, err := util.ParseUUID(principal.UserID)
	if err != nil || !issueID.Valid || len(key) == 0 || len(key) > 128 {
		return empty, collaborationError(http.StatusBadRequest, "invalid_queue_request")
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
	op, err := q.GetCollaborationOperation(ctx, db.GetCollaborationOperationParams{ScopeKind: "issue", ScopeID: issueID, ActorUserID: actor, RequestID: key})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && op.Kind != "cancel_command" {
		return empty, collaborationError(http.StatusNotFound, "operation_not_found")
	}
	if err != nil {
		return empty, err
	}
	if !op.CompletedAt.Valid {
		return empty, collaborationError(http.StatusConflict, "operation_incomplete")
	}
	var out CollaborationQueueReceipt
	if json.Unmarshal(op.Result, &out) != nil || out.IssueID != issueID || out.Code == "" || out.Code != op.ResultCode.String || out.Status < 200 || out.Status > 599 || out.Action != "cancel_command" || out.RequestID != key || out.ProtocolVersion != collaboration.ExecutionProtocolVersion {
		return empty, errors.New("invalid collaboration command operation receipt")
	}
	access, err := s.ResolveIssueAccess(ctx, q, issueID)
	var denied *CollaborationError
	if err != nil && (!errors.As(err, &denied) || denied.Code != "not_found") {
		return empty, err
	}
	if err != nil || !access.Capabilities.ControlAI {
		out = CollaborationQueueReceipt{Status: out.Status, Code: out.Code, Redacted: true, IssueID: issueID, RequestID: key, Action: out.Action, ProtocolVersion: out.ProtocolVersion}
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	return out, nil
}
