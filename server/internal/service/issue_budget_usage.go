package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/budget"
	"github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type IssueBudgetUsageService struct {
	Queries   *db.Queries
	TxStarter TxStarter
	Evidence  IssueBudgetUsageEvidenceTable
	Prices    IssueBudgetUsagePriceSource
}

func NewIssueBudgetUsageService(q *db.Queries, tx TxStarter) *IssueBudgetUsageService {
	return &IssueBudgetUsageService{Queries: q, TxStarter: tx, Evidence: productionIssueBudgetUsageEvidence(), Prices: metrics.PriceForModelAlias}
}

// Public receipts deliberately contain no amounts, root identity or revision.
type IssueBudgetUsageReceipt struct {
	EntryID     string               `json:"entry_id"`
	Kind        budget.EntryKind     `json:"kind"`
	Disposition budget.Disposition   `json:"disposition"`
	Reason      budget.UnknownReason `json:"reason"`
}

type IssueBudgetUsageResponse struct {
	Status    string                     `json:"status"`
	Replayed  *bool                      `json:"replayed,omitempty"`
	Receipts  *[]IssueBudgetUsageReceipt `json:"receipts,omitempty"`
	Reason    string                     `json:"reason,omitempty"`
	QueryPath string                     `json:"query_path,omitempty"`
}

type issueBudgetUsageOperationResult struct {
	OperationID    string                             `json:"operation_id"`
	OperationKind  string                             `json:"operation_kind"`
	BudgetRootID   string                             `json:"budget_root_id"`
	AttemptID      string                             `json:"attempt_id"`
	BatchKey       string                             `json:"batch_key"`
	PayloadDigest  string                             `json:"payload_digest"`
	LedgerRevision string                             `json:"ledger_revision"`
	Receipts       []budget.Receipt                   `json:"receipts"`
	Invocations    []issueBudgetUsageInvocationResult `json:"invocations"`
}

func (r issueBudgetUsageOperationResult) response(replayed bool) IssueBudgetUsageResponse {
	receipts := make([]IssueBudgetUsageReceipt, 0, len(r.Receipts))
	for _, receipt := range r.Receipts {
		receipts = append(receipts, IssueBudgetUsageReceipt{EntryID: receipt.EntryID, Kind: receipt.Kind, Disposition: receipt.Disposition, Reason: receipt.Reason})
	}
	return IssueBudgetUsageResponse{Status: "recorded", Replayed: &replayed, Receipts: &receipts}
}

type IssueBudgetUsageInvalidError struct{ Cause error }

func (e *IssueBudgetUsageInvalidError) Error() string {
	return "invalid budget usage: " + e.Cause.Error()
}
func (e *IssueBudgetUsageInvalidError) Unwrap() error { return e.Cause }

func issueBudgetUsageIntent(attempt db.IssueBudgetAttempt, payloadDigest string) string {
	intent := struct {
		Kind          string `json:"kind"`
		WorkspaceID   string `json:"workspace_id"`
		ScopeKind     string `json:"scope_kind"`
		ScopeID       string `json:"scope_id"`
		PayloadDigest string `json:"payload_digest"`
	}{"usage_batch", util.UUIDToString(attempt.WorkspaceID), "attempt", util.UUIDToString(attempt.ID), payloadDigest}
	raw, _ := json.Marshal(intent)
	sum := sha256.Sum256(append([]byte("budget-operation-intent-v1\n"), raw...))
	return hex.EncodeToString(sum[:])
}

// Report receives an already-authorized, retained attempt. It never needs its
// task/issue, so deletion, supersession and lifecycle holds cannot lose usage.
func (s *IssueBudgetUsageService) Report(ctx context.Context, attempt db.IssueBudgetAttempt, raw []byte, receivedAt time.Time) (IssueBudgetUsageResponse, error) {
	batch, err := DecodeIssueBudgetUsage(raw)
	if err != nil {
		return IssueBudgetUsageResponse{}, &IssueBudgetUsageInvalidError{Cause: err}
	}
	if !attempt.ProtocolVersion.Valid || attempt.ProtocolVersion.Int32 != batch.ProtocolVersion {
		return IssueBudgetUsageResponse{}, issueBudgetError("protocol_mismatch")
	}
	if s.TxStarter == nil {
		return IssueBudgetUsageResponse{}, errors.New("budget usage requires a transaction")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return IssueBudgetUsageResponse{}, fmt.Errorf("begin budget usage: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, issueBudgetIsolationSQL); err != nil {
		return IssueBudgetUsageResponse{}, fmt.Errorf("declare budget usage isolation: %w", err)
	}
	q := s.Queries.WithTx(tx)
	root, err := q.LockIssueBudgetRoot(ctx, db.LockIssueBudgetRootParams{WorkspaceID: attempt.WorkspaceID, ID: attempt.BudgetRootID})
	if err != nil {
		return IssueBudgetUsageResponse{}, fmt.Errorf("lock budget usage root: %w", err)
	}
	digest := issueBudgetUsageIntent(attempt, batch.PayloadDigest)
	operation, err := q.GetIssueBudgetOperationByKey(ctx, db.GetIssueBudgetOperationByKeyParams{WorkspaceID: attempt.WorkspaceID, ScopeKind: "attempt", ScopeID: attempt.ID, OperationKey: batch.BatchKey})
	if err == nil {
		if operation.IntentDigest != digest || operation.OperationKind != "usage_batch" {
			return IssueBudgetUsageResponse{}, issueBudgetError("operation_conflict")
		}
		return issueBudgetUsageStoredResponse(operation)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return IssueBudgetUsageResponse{}, fmt.Errorf("read budget usage operation: %w", err)
	}
	count, err := q.CountIssueBudgetUsageBatches(ctx, db.CountIssueBudgetUsageBatchesParams{WorkspaceID: attempt.WorkspaceID, ScopeID: attempt.ID})
	if err != nil {
		return IssueBudgetUsageResponse{}, fmt.Errorf("count budget usage batches: %w", err)
	}
	if count != 0 {
		return IssueBudgetUsageResponse{}, issueBudgetError("batch_conflict")
	}
	if root.Mode == "off" {
		return IssueBudgetUsageResponse{Status: "not_recorded", Reason: "observation_off"}, nil
	}
	ledger, err := issueBudgetLedgers.loadUnderRootLock(ctx, q, root)
	if err != nil {
		return IssueBudgetUsageResponse{}, err
	}
	normalized := normalizeIssueBudgetUsage(batch, attempt.ID, ledger.HasReservation(util.UUIDToString(attempt.ID)), s.Evidence, s.Prices)
	operationID := dbid.NewV7()
	for i := range normalized.Entries {
		normalized.Entries[i].OperationID = operationID
		normalized.Entries[i].ReceivedAt = receivedAt
	}
	appended, err := issueBudgetLedgers.appendEntries(ctx, q, root, ledger, normalized.Entries)
	if err != nil {
		if isIssueBudgetUsageMeteringError(err) {
			return IssueBudgetUsageResponse{}, &IssueBudgetUsageInvalidError{Cause: err}
		}
		return IssueBudgetUsageResponse{}, err
	}
	for _, receipt := range appended.Receipts {
		if receipt.Replayed {
			return IssueBudgetUsageResponse{}, errors.New("budget usage first delivery unexpectedly replayed an entry")
		}
	}
	revision, err := budget.FormatDecimal(appended.Root.LedgerRevision)
	if err != nil {
		return IssueBudgetUsageResponse{}, err
	}
	result := issueBudgetUsageOperationResult{OperationID: util.UUIDToString(operationID), OperationKind: "usage_batch", BudgetRootID: util.UUIDToString(root.ID), AttemptID: util.UUIDToString(attempt.ID), BatchKey: batch.BatchKey, PayloadDigest: batch.PayloadDigest, LedgerRevision: revision, Receipts: appended.Receipts, Invocations: normalized.Invocations}
	encoded, err := json.Marshal(result)
	if err != nil {
		return IssueBudgetUsageResponse{}, fmt.Errorf("encode budget usage result: %w", err)
	}
	if _, err := q.InsertIssueBudgetOperation(ctx, db.InsertIssueBudgetOperationParams{ID: operationID, WorkspaceID: attempt.WorkspaceID, BudgetRootID: root.ID, ScopeKind: "attempt", ScopeID: attempt.ID, OperationKey: batch.BatchKey, OperationKind: "usage_batch", IntentDigest: digest, ActorType: "runtime", ActorID: attempt.RuntimeID, Result: encoded}); err != nil {
		return IssueBudgetUsageResponse{}, fmt.Errorf("insert budget usage operation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return IssueBudgetUsageResponse{}, fmt.Errorf("commit budget usage: %w", err)
	}
	appended.Committed()
	return result.response(false), nil
}

func isIssueBudgetUsageMeteringError(err error) bool {
	for _, target := range []error{budget.ErrInvalidEntry, budget.ErrOverflow, budget.ErrNegativeAmount, budget.ErrConflict, budget.ErrModeMismatch, budget.ErrUnknownReservation, budget.ErrReleaseProofIncomplete} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func issueBudgetUsageStoredResponse(operation db.IssueBudgetOperation) (IssueBudgetUsageResponse, error) {
	var stored issueBudgetUsageOperationResult
	if err := json.Unmarshal(operation.Result, &stored); err != nil {
		return IssueBudgetUsageResponse{}, fmt.Errorf("decode budget usage result: %w", err)
	}
	if stored.OperationKind != "usage_batch" || stored.OperationID != util.UUIDToString(operation.ID) || stored.AttemptID != util.UUIDToString(operation.ScopeID) || stored.BatchKey != operation.OperationKey {
		return IssueBudgetUsageResponse{}, errors.New("invalid stored budget usage result")
	}
	return stored.response(true), nil
}

// Query is an unlocked read. Absence means unknown, including a transaction
// still committing. A caller must preserve the original bytes and key.
func (s *IssueBudgetUsageService) Query(ctx context.Context, attempt db.IssueBudgetAttempt, batchKey string) (IssueBudgetUsageResponse, error) {
	if !ValidIssueBudgetUsageBatchKey(batchKey) {
		return IssueBudgetUsageResponse{}, &IssueBudgetUsageInvalidError{Cause: errors.New("batch_key: invalid format")}
	}
	operation, err := s.Queries.GetIssueBudgetOperationByKey(ctx, db.GetIssueBudgetOperationByKeyParams{WorkspaceID: attempt.WorkspaceID, ScopeKind: "attempt", ScopeID: attempt.ID, OperationKey: batchKey})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && operation.OperationKind != "usage_batch") {
		return IssueBudgetUsageResponse{Status: "unknown", QueryPath: "/api/daemon/budget/attempts/" + util.UUIDToString(attempt.ID) + "/usage/" + batchKey}, nil
	}
	if err != nil {
		return IssueBudgetUsageResponse{}, fmt.Errorf("query budget usage: %w", err)
	}
	return issueBudgetUsageStoredResponse(operation)
}
