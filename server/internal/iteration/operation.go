package iteration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type OperationKey struct {
	WorkspaceID string
	ActorUserID string
	RequestID   string
	Operation   string
}

func (key OperationKey) queryKey() (db.GetIterationOperationParams, error) {
	var result db.GetIterationOperationParams
	for _, field := range []struct {
		value  string
		target *pgtype.UUID
	}{
		{key.WorkspaceID, &result.WorkspaceID},
		{key.ActorUserID, &result.ActorUserID},
		{key.RequestID, &result.RequestID},
	} {
		parsed, err := uuid.Parse(field.value)
		if err != nil || parsed == uuid.Nil {
			return result, fmt.Errorf("operation identity requires nonzero UUIDs")
		}
		*field.target = pgtype.UUID{Bytes: parsed, Valid: true}
	}
	switch key.Operation {
	case "create", "edit", "enable", "start", "move", "end", "cancel", "disable", "delete", "handoff":
	default:
		return result, fmt.Errorf("unsupported iteration operation")
	}
	return result, nil
}

// PayloadHash receives normalized input. Draft callers must first use
// CanonicalDraftJSON and embed its result as json.RawMessage, not []byte.
func (key OperationKey) PayloadHash(payload any) (string, error) {
	identity, err := key.queryKey()
	if err != nil {
		return "", err
	}
	return CanonicalHash(map[string]any{
		"contract_version": SchemaVersion,
		"workspace_id":     uuid.UUID(identity.WorkspaceID.Bytes).String(),
		"actor_user_id":    uuid.UUID(identity.ActorUserID.Bytes).String(),
		"operation":        key.Operation,
		"input":            payload,
	})
}

// LoadOperation must be called only after current workspace/actor authorization
// and the iteration fence. It deliberately does not inspect the current entity:
// a successful deleted result remains replayable to its authorized actor.
func LoadOperation(ctx context.Context, tx pgx.Tx, key OperationKey, payloadHash string) (WriteResult, bool, error) {
	params, err := key.queryKey()
	if err != nil {
		return WriteResult{}, false, err
	}
	row, err := db.New(tx).GetIterationOperation(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return WriteResult{}, false, nil
	}
	if err != nil {
		return WriteResult{}, false, err
	}
	result, err := decodeOperationResult(row, key, payloadHash, true)
	return result, err == nil, err
}

func decodeOperationResult(row db.IterationOperation, key OperationKey, hash string, replayed bool) (WriteResult, error) {
	params, err := key.queryKey()
	if err != nil {
		return WriteResult{}, err
	}
	if row.WorkspaceID != params.WorkspaceID || row.ActorUserID != params.ActorUserID || row.RequestID != params.RequestID {
		return WriteResult{}, fmt.Errorf("stored operation identity mismatch")
	}
	if row.PayloadHash != hash || row.Operation != key.Operation {
		return WriteResult{}, &OperationError{Status: 409, Code: "idempotency_conflict", Message: "Request ID is already used for another intention"}
	}
	var result WriteResult
	if err := json.Unmarshal(row.Result, &result); err != nil {
		return result, fmt.Errorf("decode stored iteration operation: %w", err)
	}
	if result.WorkspaceID != uuid.UUID(row.WorkspaceID.Bytes).String() || result.RequestID != uuid.UUID(row.RequestID.Bytes).String() || result.OperationID != uuid.UUID(row.ID.Bytes).String() || result.Operation != row.Operation || result.CommittedAt.IsZero() || result.IterationIDs == nil || result.Result.SettingsRevision < 1 || result.Result.IssueCount < 0 {
		return WriteResult{}, fmt.Errorf("stored iteration operation result is incomplete")
	}
	if result.Result.SettingsRevision > 9007199254740991 || int64(result.Result.IssueCount) > 9007199254740991 {
		return WriteResult{}, fmt.Errorf("stored iteration operation counters exceed safe integer range")
	}
	validReference := func(value string) bool {
		id, err := uuid.Parse(value)
		return err == nil && id != uuid.Nil && id.String() == value
	}
	seen := make(map[string]bool, len(result.IterationIDs))
	for _, id := range result.IterationIDs {
		if !validReference(id) || seen[id] {
			return WriteResult{}, fmt.Errorf("stored iteration operation has invalid or duplicate iteration identities")
		}
		seen[id] = true
	}
	if result.Result.SnapshotID != nil && !validReference(*result.Result.SnapshotID) {
		return WriteResult{}, fmt.Errorf("stored iteration operation has an invalid snapshot identity")
	}
	result.Replayed = replayed
	return result, nil
}

// SaveOperation stores the result in the same transaction as all business
// writes. Do not swallow a unique conflict: roll back and reauthorize before
// looking up the winning request, using the original payload hash.
func SaveOperation(ctx context.Context, tx pgx.Tx, key OperationKey, payloadHash string, result WriteResult) error {
	params, err := key.queryKey()
	if err != nil {
		return err
	}
	id, err := uuid.Parse(result.OperationID)
	if err != nil || id == uuid.Nil {
		return fmt.Errorf("operation result requires a stable UUID")
	}
	result.Replayed = false
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	row := db.IterationOperation{ID: pgtype.UUID{Bytes: id, Valid: true}, WorkspaceID: params.WorkspaceID, ActorUserID: params.ActorUserID, RequestID: params.RequestID, Operation: key.Operation, PayloadHash: payloadHash, Result: raw}
	if _, err := decodeOperationResult(row, key, payloadHash, false); err != nil {
		return err
	}
	_, err = db.New(tx).InsertIterationOperation(ctx, db.InsertIterationOperationParams{
		ID: row.ID, WorkspaceID: row.WorkspaceID, ActorUserID: row.ActorUserID, RequestID: row.RequestID, Operation: row.Operation, PayloadHash: row.PayloadHash, Result: row.Result,
		CreatedAt: pgtype.Timestamptz{Time: result.CommittedAt, Valid: true},
	})
	return err
}

// ReadOperation loads the actor's durable result after current authorization.
// GET callers do not possess the original payload hash or operation name. Those
// stored fields validate the result's integrity; they never grant authorization.
func ReadOperation(ctx context.Context, tx pgx.Tx, workspaceID, actorID, requestID pgtype.UUID) (WriteResult, error) {
	for _, id := range []pgtype.UUID{workspaceID, actorID, requestID} {
		if !id.Valid || id.Bytes == [16]byte{} {
			return WriteResult{}, &OperationError{Status: 400, Code: "invalid_request", Message: "Operation lookup requires nonzero UUIDs"}
		}
	}
	row, err := db.New(tx).GetIterationOperation(ctx, db.GetIterationOperationParams{WorkspaceID: workspaceID, ActorUserID: actorID, RequestID: requestID})
	if errors.Is(err, pgx.ErrNoRows) {
		return WriteResult{}, &OperationError{Status: 404, Code: "operation_not_found", Message: "Operation not found"}
	}
	if err != nil {
		return WriteResult{}, err
	}
	key := OperationKey{WorkspaceID: uuid.UUID(workspaceID.Bytes).String(), ActorUserID: uuid.UUID(actorID.Bytes).String(), RequestID: uuid.UUID(requestID.Bytes).String(), Operation: row.Operation}
	return decodeOperationResult(row, key, row.PayloadHash, true)
}
