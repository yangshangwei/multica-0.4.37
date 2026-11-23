package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type stageControlConfiguration struct {
	target                                   service.StageExecutionTarget
	metricSource                             service.StageIntegrationIdentity
	queryID, queryDigest, ruleID, ruleDigest string
	freshness                                time.Duration
}
type stageControlProof struct {
	Kind        string
	Source      service.StageIntegrationIdentity
	Request     service.StageExecutionRequest
	Consumer    string
	ValidUntil  time.Time
	Observation service.StageExecutionObservation
	Metric      service.StageRollbackMetric
}
type stageControlCalls struct {
	Count int                     `json:"count"`
	Last  *stageControlInvocation `json:"last"`
}
type stageControlInvocation struct {
	IntentID           string `json:"intent_id"`
	Kind               string `json:"kind"`
	TargetDigest       string `json:"target_digest"`
	ArtifactDigest     string `json:"artifact_digest"`
	ExternalRequestKey string `json:"external_request_key"`
	ConditionalVersion string `json:"conditional_version"`
}

type stageControlCapture struct {
	InputDigest string
	Observation service.StageExecutionObservation
}

type stageControlledSource struct {
	mu          sync.Mutex
	pool        *pgxpool.Pool
	key         []byte
	configured  bool
	current     stageControlConfiguration
	retained    map[string]stageControlConfiguration
	captures    map[string]stageControlCapture
	behavior    string
	delay       time.Duration
	operationID *string
	calls       stageControlCalls
}

func newStageControlledSource(pool *pgxpool.Pool) (*stageControlledSource, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return &stageControlledSource{pool: pool, key: key, retained: map[string]stageControlConfiguration{}, behavior: "succeeded", captures: map[string]stageControlCapture{}}, nil
}
func stageControlError(code string) error {
	return &service.CollaborationError{Status: 409, Code: code}
}
func (s *stageControlledSource) seal(value stageControlProof) service.StageExecutionProof {
	raw, _ := json.Marshal(value)
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(raw)
	return service.StageExecutionProof{Payload: raw, Authentication: mac.Sum(nil)}
}
func (s *stageControlledSource) open(proof service.StageExecutionProof, kind string) (stageControlProof, error) {
	var out stageControlProof
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(proof.Payload)
	if !hmac.Equal(mac.Sum(nil), proof.Authentication) || json.Unmarshal(proof.Payload, &out) != nil || out.Kind != kind {
		return out, stageControlError("stage_execution_unavailable")
	}
	return out, nil
}
func stageControlTrust(ctx context.Context, tx pgx.Tx, workspace string, source service.StageIntegrationIdentity) error {
	var revision int64
	if err := tx.QueryRow(ctx, `SELECT trust_revision FROM github_installation WHERE id=$1 AND workspace_id=$2 FOR SHARE NOWAIT`, source.IntegrationID, workspace).Scan(&revision); err != nil {
		return err
	}
	if strconv.FormatInt(revision, 10) != source.TrustRevision {
		return stageControlError("stage_execution_target_changed")
	}
	return nil
}
func (s *stageControlledSource) configuration(id string) (stageControlConfiguration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, ok := s.retained[id]
	if !ok {
		return out, stageControlError("stage_execution_unavailable")
	}
	if s.current.target.Source.IntegrationID == id {
		out = s.current
	}
	return out, nil
}
func (s *stageControlledSource) Targets(ctx context.Context, tx pgx.Tx, workspace pgtype.UUID) ([]service.StageExecutionTarget, error) {
	s.mu.Lock()
	ready, c := s.configured, s.current
	s.mu.Unlock()
	if !ready || c.target.WorkspaceID != util.UUIDToString(workspace) {
		return []service.StageExecutionTarget{}, nil
	}
	if err := stageControlTrust(ctx, tx, c.target.WorkspaceID, c.target.Source); err != nil {
		return nil, err
	}
	return []service.StageExecutionTarget{c.target}, nil
}
func (s *stageControlledSource) AuthorizeExecution(ctx context.Context, tx pgx.Tx, p service.StageExecutionRequest, target service.StageExecutionTarget, proof service.StageExecutionProof) (service.StageExecutionConsumer, error) {
	v, err := s.open(proof, "consumer")
	if err != nil {
		return service.StageExecutionConsumer{}, err
	}
	c, err := s.configuration(target.Source.IntegrationID)
	if err != nil {
		return service.StageExecutionConsumer{}, err
	}
	if !reflect.DeepEqual(p, v.Request) || v.Source != c.target.Source || c.target.WorkspaceID != util.UUIDToString(p.WorkspaceID) {
		return service.StageExecutionConsumer{}, stageControlError("stage_execution_unavailable")
	}
	if err = stageControlTrust(ctx, tx, c.target.WorkspaceID, c.target.Source); err != nil {
		return service.StageExecutionConsumer{}, err
	}
	return service.StageExecutionConsumer{Subject: v.Consumer, ValidUntil: v.ValidUntil}, nil
}
func (s *stageControlledSource) consumer(ctx context.Context, p service.StageExecutionRequest, target service.StageExecutionTarget) (service.StageExecutionProof, error) {
	c, err := s.configuration(target.Source.IntegrationID)
	if err != nil {
		return service.StageExecutionProof{}, err
	}
	now, err := db.New(s.pool).ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return service.StageExecutionProof{}, err
	}
	return s.seal(stageControlProof{Kind: "consumer", Source: c.target.Source, Request: p, Consumer: "controlled-stage-consumer", ValidUntil: now.Time.Add(time.Minute)}), nil
}
func (s *stageControlledSource) Execute(ctx context.Context, inv service.StageExecutionInvocation) (service.StageExecutionProof, error) {
	s.mu.Lock()
	s.calls.Count++
	s.calls.Last = &stageControlInvocation{inv.IntentID, inv.Kind, inv.TargetDigest, inv.ArtifactDigest, inv.ExternalRequestKey, inv.Target.ConditionalVersion}
	behavior, delay, operation := s.behavior, s.delay, s.operationID
	s.behavior, s.delay, s.operationID = "succeeded", 0, nil
	s.mu.Unlock()
	if behavior == "panic" {
		panic("controlled stage callback panic")
	}
	if behavior == "delayed" {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return service.StageExecutionProof{}, ctx.Err()
		case <-timer.C:
		}
		behavior = "succeeded"
	}
	return s.captureExecution(ctx, inv, "callback:"+inv.IntentID, "1", behavior, operation, true)
}
func (s *stageControlledSource) captureExecution(ctx context.Context, inv service.StageExecutionInvocation, nativeID, nativeVersion, result string, operation *string, conditional bool) (service.StageExecutionProof, error) {
	now, err := db.New(s.pool).ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return service.StageExecutionProof{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.retained[inv.Target.Source.IntegrationID]
	if s.current.target.Source.IntegrationID == inv.Target.Source.IntegrationID {
		c = s.current
	}
	if !ok {
		return service.StageExecutionProof{}, stageControlError("stage_execution_unavailable")
	}
	if operation == nil && (result == "succeeded" || result == "failed") {
		v := "controlled/" + inv.IntentID
		operation = &v
	}
	input, _ := json.Marshal(struct {
		Result      string
		Operation   *string
		Conditional bool
	}{result, operation, conditional})
	captureKey := inv.IntentID + ":" + nativeID + ":" + nativeVersion
	inputDigest := stageControlHash(string(input))
	if prior, exists := s.captures[captureKey]; exists {
		if prior.InputDigest != inputDigest {
			return service.StageExecutionProof{}, stageControlError("stage_execution_conflict")
		}
		return s.seal(stageControlProof{Kind: "execution", Source: c.target.Source, Observation: prior.Observation}), nil
	}
	if conditional && (c.target.ConditionalVersion != inv.Target.ConditionalVersion || !reflect.DeepEqual(c.target.CurrentArtifactDigest, inv.Target.CurrentArtifactDigest)) {
		result = "not_started"
	}
	o := service.StageExecutionObservation{NativeID: nativeID, NativeVersion: nativeVersion, Source: inv.Target.Source, ExternalRequestKey: inv.ExternalRequestKey, Result: result, TargetDigest: inv.TargetDigest, ConditionalVersion: inv.Target.ConditionalVersion, BeforeArtifactDigest: inv.Target.CurrentArtifactDigest, ObservedAt: now.Time.UTC(), EvidenceDigest: stageControlHash("controlled-execution:" + inv.IntentID + ":" + result), Summary: "Controlled test integration result"}
	if result == "succeeded" || result == "failed" {
		if operation == nil {
			v := "controlled/" + inv.IntentID
			operation = &v
		}
		o.ExternalOperationID = operation
	}
	if result == "succeeded" {
		artifact := inv.ArtifactDigest
		c.target.CurrentArtifactDigest = &artifact
		c.target.ConditionalVersion = inv.Target.ConditionalVersion + "/after/" + inv.IntentID
		o.AfterArtifactDigest = &artifact
		o.AfterConditionalVersion = c.target.ConditionalVersion
		s.retained[inv.Target.Source.IntegrationID] = c
		if s.current.target.Source.IntegrationID == inv.Target.Source.IntegrationID {
			s.current = c
		}
	}
	s.captures[captureKey] = stageControlCapture{InputDigest: inputDigest, Observation: o}
	return s.seal(stageControlProof{Kind: "execution", Source: c.target.Source, Observation: o}), nil
}
func (s *stageControlledSource) ExecutionObservation(ctx context.Context, tx pgx.Tx, inv service.StageExecutionInvocation, proof service.StageExecutionProof) (service.StageExecutionObservation, error) {
	v, err := s.open(proof, "execution")
	if err != nil {
		return v.Observation, err
	}
	c, err := s.configuration(inv.Target.Source.IntegrationID)
	if err != nil {
		return v.Observation, err
	}
	if v.Source != c.target.Source {
		return v.Observation, stageControlError("stage_execution_target_changed")
	}
	if err = stageControlTrust(ctx, tx, c.target.WorkspaceID, c.target.Source); err != nil {
		return v.Observation, err
	}
	return v.Observation, nil
}
func (s *stageControlledSource) MetricObservation(ctx context.Context, tx pgx.Tx, deployment service.StageExecutionDeployment, proof service.StageExecutionProof) (service.StageRollbackMetric, error) {
	v, err := s.open(proof, "metric")
	if err != nil {
		return v.Metric, err
	}
	if v.Source != v.Metric.Source {
		return v.Metric, stageControlError("stage_rollback_observation_changed")
	}
	return v.Metric, s.CheckMetricCurrent(ctx, tx, deployment.Invocation.Target, v.Metric)
}
func (s *stageControlledSource) CheckMetricCurrent(ctx context.Context, tx pgx.Tx, target service.StageExecutionTarget, metric service.StageRollbackMetric) error {
	c, err := s.configuration(target.Source.IntegrationID)
	if err != nil {
		return err
	}
	if metric.Source != c.metricSource || metric.QueryID != c.queryID || metric.QueryDigest != c.queryDigest || metric.RuleID != c.ruleID || metric.RuleDigest != c.ruleDigest || !metric.ValidUntil.Equal(metric.ObservedAt.Add(c.freshness)) {
		return stageControlError("stage_rollback_observation_changed")
	}
	return stageControlTrust(ctx, tx, c.target.WorkspaceID, c.metricSource)
}
func (s *stageControlledSource) callSnapshot() stageControlCalls {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.calls
	if out.Last != nil {
		last := *out.Last
		out.Last = &last
	}
	return out
}
func stageControlHash(value string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(value))) }

var _ service.StageExecutionSources = (*stageControlledSource)(nil)
