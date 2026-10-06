package iteration

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// OperationMutation borrows the owning transaction. It must validate/lock the
// complete affected set before writes and never publish or invoke execution.
// It supplies the locked database-time sample in CommittedAt; the runner stamps
// stable identity fields and validates the entire result before persistence.
type OperationMutation func(context.Context, pgx.Tx, pgtype.UUID) (WriteResult, error)
type operationRequestCollision struct{ error }

// RunOperation owns the bounded RC transaction and durable result. Authorize
// checks the current transport/role under the member fence, without taking
// business row locks. The mutation handles entity permissions after the I1
// fence. A lost commit response is returned, never automatically re-executed.
func RunOperation(ctx context.Context, beginner TransactionBeginner, key OperationKey, payload any, authorize func(context.Context, pgx.Tx) error, mutate OperationMutation) (WriteResult, error) {
	if authorize == nil || mutate == nil {
		return WriteResult{}, errors.New("iteration operation requires current authorization and mutation")
	}
	params, err := key.queryKey()
	if err != nil {
		return WriteResult{}, &OperationError{Status: 400, Code: "invalid_request", Message: "Invalid operation identity"}
	}
	hash, err := key.PayloadHash(payload)
	if err != nil {
		return WriteResult{}, &OperationError{Status: 400, Code: "invalid_request", Message: "Invalid operation payload"}
	}
	operationID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	var result WriteResult
	err = RunTransaction(ctx, beginner, func(ctx context.Context, tx pgx.Tx) error {
		q := db.New(tx)
		if _, e := q.LockWorkspaceForChatSessionCreate(ctx, params.WorkspaceID); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return &OperationError{Status: 404, Code: "workspace_not_found", Message: "Workspace not found"}
			}
			return e
		}
		if e := q.LockSubscriberWrites(ctx, db.LockSubscriberWritesParams{WorkspaceID: params.WorkspaceID, UserID: params.ActorUserID}); e != nil {
			return e
		}
		if _, e := q.LockActiveMember(ctx, db.LockActiveMemberParams{WorkspaceID: params.WorkspaceID, UserID: params.ActorUserID}); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				return &OperationError{Status: 403, Code: "forbidden", Message: "Workspace membership required"}
			}
			return e
		}
		if e := authorize(ctx, tx); e != nil {
			return e
		}
		if e := q.LockIssueStatusCatalogShared(ctx, params.WorkspaceID); e != nil {
			return e
		}
		if e := LockWorkspace(ctx, tx, params.WorkspaceID); e != nil {
			return e
		}
		previous, found, e := LoadOperation(ctx, tx, key, hash)
		if e != nil {
			return e
		}
		if found {
			result = previous
			return nil
		}
		outcome, e := mutate(ctx, tx, operationID)
		if e != nil {
			return e
		}
		result = outcome
		result.WorkspaceID = uuid.UUID(params.WorkspaceID.Bytes).String()
		result.RequestID = uuid.UUID(params.RequestID.Bytes).String()
		result.OperationID = uuid.UUID(operationID.Bytes).String()
		result.Operation = key.Operation
		result.Replayed = false

		if e = SaveOperation(ctx, tx, key, hash, result); e != nil {
			var conflict *pgconn.PgError
			if errors.As(e, &conflict) && conflict.Code == "23505" && conflict.ConstraintName == "iteration_operation_request" {
				return &operationRequestCollision{e}
			}
			return e
		}
		return nil
	})
	if err != nil {
		return WriteResult{}, fmt.Errorf("iteration operation: %w", err)
	}
	return result, nil
}
