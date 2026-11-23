package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
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

func stageMetricDigest(metric StageRollbackMetric) (string, error) {
	metric.NativeID = ""
	return stageExecutionDigest("collaboration-stage-rollback-metric-v1", metric)
}
func decodeStageMetric(row db.CollaborationStageRollbackObservation) (StageRollbackMetric, error) {
	var metric StageRollbackMetric
	if json.Unmarshal(row.Metric, &metric) != nil || metric.DeploymentID != util.UUIDToString(row.DeploymentID) || metric.NativeVersion != stageVersion(row.NativeVersion) || stageMetricStream(metric) != row.StreamKey {
		return metric, collaborationError(503, "stage_rollback_observation_unavailable")
	}
	digest, err := stageMetricDigest(metric)
	if err != nil || digest != row.ObservationDigest {
		return metric, collaborationError(503, "stage_rollback_observation_unavailable")
	}
	return metric, nil
}
func stageMetricValid(metric StageRollbackMetric, deployment StageExecutionDeployment, now time.Time) bool {
	return stageBoundedText(metric.NativeID, 256) && stagePositiveVersion(metric.NativeVersion) && stageBoundedText(metric.StreamIdentity, 256) && stageIntegrationValid(metric.Source) && metric.Source != deployment.Invocation.Target.Source && metric.DeploymentID == deployment.FactID && metric.TargetDigest == deployment.Invocation.TargetDigest && metric.ArtifactDigest == deployment.Invocation.ArtifactDigest && stageBoundedText(metric.QueryID, 256) && stageDigest(metric.QueryDigest) && stageBoundedText(metric.RuleID, 256) && stageDigest(metric.RuleDigest) && stageDigest(metric.EvidenceDigest) && !metric.WindowStart.IsZero() && metric.WindowStart.Before(metric.WindowEnd) && !metric.WindowEnd.After(metric.ObservedAt) && !metric.ObservedAt.After(now) && metric.ObservedAt.Before(metric.ValidUntil) && len(metric.Summary) <= 2000 && utf8.ValidString(metric.Summary) && !strings.ContainsRune(metric.Summary, 0) && (metric.Condition == "anomaly" || metric.Condition == "healthy" || metric.Condition == "unknown")
}
func (s *CollaborationService) RecordStageRollbackObservation(ctx context.Context, deploymentID pgtype.UUID, proof StageExecutionProof) (StageRollbackMetricReceipt, error) {
	if s.StageExecutionSources == nil {
		return StageRollbackMetricReceipt{}, collaborationError(409, "stage_execution_unavailable")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	q := db.New(tx)
	if err = stageExecutionReady(ctx, q); err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	fact, err := q.GetStageExecutionFact(ctx, deploymentID)
	if err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	intent, err := q.GetStageExecutionIntent(ctx, fact.IntentID)
	if err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	deployment, _, err := stageExecutionDeploymentFor(ctx, q, deploymentID, intent.IssueID, intent.CandidateID)
	if err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	metric, err := s.StageExecutionSources.MetricObservation(ctx, tx, deployment, proof)
	if err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	metric.Summary = redact.Text(metric.Summary)
	if !stageMetricValid(metric, deployment, now.Time) {
		return StageRollbackMetricReceipt{}, collaborationError(409, "stage_rollback_observation_changed")
	}
	if err = s.StageExecutionSources.CheckMetricCurrent(ctx, tx, deployment.Invocation.Target, metric); err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	stream := stageMetricStream(metric)
	locked, err := q.TryLockCollaborationAuthorityNamespace(ctx, "collaboration-stage-metric:"+stream)
	if err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	if !locked {
		return StageRollbackMetricReceipt{}, collaborationError(409, "stage_rollback_observation_unavailable")
	}
	digest, err := stageMetricDigest(metric)
	if err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	native, _ := strconv.ParseInt(metric.NativeVersion, 10, 64)
	existing, err := q.GetStageRollbackObservationByContent(ctx, db.GetStageRollbackObservationByContentParams{StreamKey: stream, NativeVersion: native, ObservationDigest: digest})
	receipt := func(row db.CollaborationStageRollbackObservation) StageRollbackMetricReceipt {
		return StageRollbackMetricReceipt{Version: 1, IssueID: util.UUIDToString(row.IssueID), ObservationID: util.UUIDToString(row.ObservationID), ObservationVersion: stageVersion(row.Version), ObservationDigest: row.ObservationDigest}
	}
	if err == nil {
		return receipt(existing), tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return StageRollbackMetricReceipt{}, err
	}
	version, err := q.StageRollbackNextVersion(ctx, stream)
	if err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	raw, err := json.Marshal(metric)
	if err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	row, err := q.CreateStageRollbackObservation(ctx, db.CreateStageRollbackObservationParams{ObservationID: dbid.NewV7(), WorkspaceID: intent.WorkspaceID, IssueID: intent.IssueID, CandidateID: intent.CandidateID, DeploymentID: deploymentID, StreamKey: stream, Version: version, NativeVersion: native, ObservationDigest: digest, Metric: raw})
	if err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	if err = recordCollaborationExecutionChange(ctx, q, intent.IssueID, pgtype.UUID{}, pgtype.UUID{}, "stage_rollback_observed"); err != nil {
		return StageRollbackMetricReceipt{}, err
	}
	return receipt(row), tx.Commit(ctx)
}
func stageMetricState(ctx context.Context, tx pgx.Tx, sources StageExecutionSources, row db.CollaborationStageRollbackObservation, target StageExecutionTarget, now time.Time) (string, error) {
	q := db.New(tx)
	metric, err := decodeStageMetric(row)
	if err != nil {
		return "", err
	}
	head, err := q.StageRollbackNativeHead(ctx, row.StreamKey)
	if err != nil {
		return "", err
	}
	if head != row.NativeVersion {
		return "superseded", nil
	}
	ambiguous, err := q.StageRollbackNativeAmbiguous(ctx, db.StageRollbackNativeAmbiguousParams{StreamKey: row.StreamKey, NativeVersion: head})
	if err != nil {
		return "", err
	}
	if ambiguous {
		return "ambiguous", nil
	}
	if !now.Before(metric.ValidUntil) {
		return "expired", nil
	}
	if sources == nil {
		return "unavailable", nil
	}
	if err = sources.CheckMetricCurrent(ctx, tx, target, metric); err != nil {
		if stageExecutionCode(err) != "" {
			return "unavailable", nil
		}
		return "", err
	}
	return "current", nil
}
func stageCurrentRollbackMetric(ctx context.Context, tx pgx.Tx, sources StageExecutionSources, issue, candidate pgtype.UUID, deployment StageExecutionDeployment, ref CollaborationStageRollbackRef, now time.Time) (StageRollbackMetric, error) {
	q := db.New(tx)
	row, err := q.GetStageRollbackObservation(ctx, util.MustParseUUID(ref.ObservationID))
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (row.IssueID != issue || row.CandidateID != candidate || row.DeploymentID != util.MustParseUUID(deployment.FactID) || row.ObservationDigest != ref.ObservationDigest) {
		return StageRollbackMetric{}, collaborationError(409, "stage_rollback_observation_changed")
	}
	if err != nil {
		return StageRollbackMetric{}, err
	}
	locked, err := q.TryLockCollaborationAuthorityNamespace(ctx, "collaboration-stage-metric:"+row.StreamKey)
	if err != nil {
		return StageRollbackMetric{}, err
	}
	if !locked {
		return StageRollbackMetric{}, collaborationError(409, "stage_rollback_observation_unavailable")
	}
	metric, err := decodeStageMetric(row)
	if err != nil {
		return metric, err
	}
	state, err := stageMetricState(ctx, tx, sources, row, deployment.Invocation.Target, now)
	if err != nil {
		return metric, err
	}
	if state != "current" || metric.Condition != "anomaly" || metric.TargetDigest != deployment.Invocation.TargetDigest || metric.ArtifactDigest != deployment.Invocation.ArtifactDigest {
		return metric, collaborationError(409, "stage_rollback_observation_unavailable")
	}
	return metric, nil
}
