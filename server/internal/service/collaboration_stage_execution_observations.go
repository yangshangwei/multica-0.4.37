package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/redact"
)

func stageObservationValid(inv StageExecutionInvocation, o StageExecutionObservation, now time.Time) bool {
	if !stageBoundedText(o.NativeID, 256) || !stageBoundedText(o.NativeVersion, 256) || o.Source != inv.Target.Source || o.ExternalRequestKey != inv.ExternalRequestKey || o.TargetDigest != inv.TargetDigest || o.ConditionalVersion != inv.Target.ConditionalVersion || !stageDigest(o.EvidenceDigest) || o.ObservedAt.IsZero() || o.ObservedAt.After(now) || len(o.Summary) > 2000 || !utf8.ValidString(o.Summary) || strings.ContainsRune(o.Summary, 0) {
		return false
	}
	if o.BeforeArtifactDigest != nil && !stageDigest(*o.BeforeArtifactDigest) || o.AfterArtifactDigest != nil && !stageDigest(*o.AfterArtifactDigest) {
		return false
	}
	if o.ExternalOperationID != nil && !stageBoundedText(*o.ExternalOperationID, 256) {
		return false
	}
	switch o.Result {
	case "succeeded":
		return o.ExternalOperationID != nil && o.AfterArtifactDigest != nil && *o.AfterArtifactDigest == inv.ArtifactDigest && stageBoundedText(o.AfterConditionalVersion, 256) && reflect.DeepEqual(o.BeforeArtifactDigest, inv.Target.CurrentArtifactDigest)
	case "failed":
		return o.ExternalOperationID != nil
	case "not_started":
		return o.ExternalOperationID == nil
	case "unknown":
		return true
	default:
		return false
	}
}
func stageTerminalEquivalent(a, b StageExecutionObservation) bool {
	a.NativeID, b.NativeID = "", ""
	a.NativeVersion, b.NativeVersion = "", ""
	a.ObservedAt, b.ObservedAt = time.Time{}, time.Time{}
	return reflect.DeepEqual(a, b)
}
func (s *CollaborationService) RecordStageExecutionObservation(ctx context.Context, intentID pgtype.UUID, proof StageExecutionProof) (StageExecutionReceipt, error) {
	if s.StageExecutionSources == nil {
		return StageExecutionReceipt{}, collaborationError(409, "stage_execution_unavailable")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	q := db.New(tx)
	if err = stageExecutionReady(ctx, q); err != nil {
		return StageExecutionReceipt{}, err
	}
	initial, err := q.GetStageExecutionIntent(ctx, intentID)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if err = lockStageExecutionPhysical(ctx, q, initial.PhysicalTargetKey); err != nil {
		return StageExecutionReceipt{}, err
	}
	row, err := q.LockStageExecutionIntent(ctx, intentID)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	inv, err := stageExecutionInvocation(row)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	observation, err := s.StageExecutionSources.ExecutionObservation(ctx, tx, inv, proof)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	observation.Summary = redact.Text(observation.Summary)
	if !stageObservationValid(inv, observation, now.Time) {
		return StageExecutionReceipt{}, collaborationError(409, "stage_execution_conflict")
	}
	receipt, err := appendStageExecutionObservation(ctx, tx, row, "integration", observation)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	return receipt, tx.Commit(ctx)
}
func (s *CollaborationService) recordStageInternalObservation(ctx context.Context, row db.CollaborationStageExecutionIntent, entry *stageExecutionEntry, result string) (StageExecutionReceipt, error) {
	// Only the service-owned, consumed entry can assert an internal dispatch
	// outcome. No exported recorder accepts this origin or a caller boolean.
	if entry == nil || !entry.consumed.Load() || entry.invocation.IntentID != util.UUIDToString(row.IntentID) || (result != "unknown" && result != "not_started") {
		return StageExecutionReceipt{}, collaborationError(409, "stage_execution_conflict")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	q := db.New(tx)
	if err = lockStageExecutionPhysical(ctx, q, row.PhysicalTargetKey); err != nil {
		return StageExecutionReceipt{}, err
	}
	locked, err := q.LockStageExecutionIntent(ctx, row.IntentID)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	inv, err := stageExecutionInvocation(locked)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	// JSON drops time.Location identity and monotonic clock readings. Compare
	// the exact deadline instant while retaining every other invocation field.
	expected := entry.invocation
	inv.DispatchExpiresAt = inv.DispatchExpiresAt.UTC()
	expected.DispatchExpiresAt = expected.DispatchExpiresAt.UTC()
	if !reflect.DeepEqual(inv, expected) {
		return StageExecutionReceipt{}, collaborationError(409, "stage_execution_conflict")
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	observation := StageExecutionObservation{NativeID: "service-entry", NativeVersion: "1", Source: inv.Target.Source, ExternalRequestKey: inv.ExternalRequestKey, Result: result, TargetDigest: inv.TargetDigest, ConditionalVersion: inv.Target.ConditionalVersion, ObservedAt: now.Time, EvidenceDigest: row.RequestDigest}
	receipt, err := appendStageExecutionObservation(ctx, tx, locked, "internal", observation)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	return receipt, tx.Commit(ctx)
}
func appendStageExecutionObservation(ctx context.Context, tx pgx.Tx, row db.CollaborationStageExecutionIntent, origin string, o StageExecutionObservation) (StageExecutionReceipt, error) {
	q := db.New(tx)
	digest, err := stageExecutionDigest("collaboration-stage-execution-observation-v1", o)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	previous, err := q.GetStageExecutionObservationByNative(ctx, db.GetStageExecutionObservationByNativeParams{IntentID: row.IntentID, Origin: origin, NativeID: o.NativeID, NativeVersion: o.NativeVersion})
	if err == nil {
		if previous.ObservationDigest != digest {
			return StageExecutionReceipt{}, collaborationError(409, "stage_execution_conflict")
		}
		return stageExecutionReceiptFor(ctx, q, row)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return StageExecutionReceipt{}, err
	}
	terminal, err := q.GetStageExecutionTerminal(ctx, row.IntentID)
	if err == nil {
		var existing StageExecutionObservation
		if json.Unmarshal(terminal.Observation, &existing) != nil {
			return StageExecutionReceipt{}, collaborationError(503, "stage_execution_unavailable")
		}
		if o.Result != "unknown" && !stageTerminalEquivalent(existing, o) {
			return StageExecutionReceipt{}, collaborationError(409, "stage_execution_conflict")
		}
		return stageExecutionReceiptFor(ctx, q, row)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return StageExecutionReceipt{}, err
	}
	if o.ExternalOperationID != nil && (o.Result == "succeeded" || o.Result == "failed") {
		var reused bool
		err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM collaboration_stage_execution_observation o JOIN collaboration_stage_execution_intent i ON i.intent_id=o.intent_id WHERE i.physical_target_key=$1 AND o.external_operation_id=$2 AND o.intent_id<>$3 AND o.result IN ('succeeded','failed'))`, row.PhysicalTargetKey, *o.ExternalOperationID, row.IntentID).Scan(&reused)
		if err != nil {
			return StageExecutionReceipt{}, err
		}
		if reused {
			return StageExecutionReceipt{}, collaborationError(409, "stage_execution_conflict")
		}
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	external := pgtype.Text{}
	if o.ExternalOperationID != nil {
		external = pgtype.Text{String: *o.ExternalOperationID, Valid: true}
	}
	_, err = q.CreateStageExecutionObservation(ctx, db.CreateStageExecutionObservationParams{ObservationID: dbid.NewV7(), IntentID: row.IntentID, WorkspaceID: row.WorkspaceID, IssueID: row.IssueID, Origin: origin, NativeID: o.NativeID, NativeVersion: o.NativeVersion, ObservationDigest: digest, Result: o.Result, ExternalOperationID: external, Observation: raw})
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if err = recordCollaborationExecutionChange(ctx, q, row.IssueID, pgtype.UUID{}, pgtype.UUID{}, "stage_execution_observed"); err != nil {
		return StageExecutionReceipt{}, err
	}
	return stageExecutionReceiptFor(ctx, q, row)
}
