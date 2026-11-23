package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type stageSourceProofFixture struct {
	Kind        string
	Request     StageExecutionRequest
	Subject     string
	ValidUntil  time.Time
	Observation StageExecutionObservation
	Metric      StageRollbackMetric
}
type stageExecutionSourceFixture struct {
	mu               sync.Mutex
	key              []byte
	target           StageExecutionTarget
	metricSource     StageIntegrationIdentity
	calls            int
	behavior         string
	last             StageExecutionInvocation
	consumerUntil    time.Time
	observationClock func(context.Context) (time.Time, error)
}

func (s *stageExecutionSourceFixture) proof(value stageSourceProofFixture) StageExecutionProof {
	raw, _ := json.Marshal(value)
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(raw)
	return StageExecutionProof{Payload: raw, Authentication: mac.Sum(nil)}
}
func (s *stageExecutionSourceFixture) decode(proof StageExecutionProof, kind string) (stageSourceProofFixture, error) {
	var out stageSourceProofFixture
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write(proof.Payload)
	if !hmac.Equal(mac.Sum(nil), proof.Authentication) || json.Unmarshal(proof.Payload, &out) != nil || out.Kind != kind {
		return out, collaborationError(403, "forbidden")
	}
	return out, nil
}
func (s *stageExecutionSourceFixture) consumer(p StageExecutionRequest) StageExecutionProof {
	s.mu.Lock()
	defer s.mu.Unlock()
	until := s.consumerUntil
	if until.IsZero() {
		until = time.Now().Add(time.Minute)
	}
	return s.proof(stageSourceProofFixture{Kind: "consumer", Request: p, Subject: "controlled-integration-consumer", ValidUntil: until})
}
func stageFixtureTrust(ctx context.Context, tx pgx.Tx, workspace string, source StageIntegrationIdentity) error {
	var trust int64
	err := tx.QueryRow(ctx, `SELECT trust_revision FROM github_installation WHERE id=$1 AND workspace_id=$2 FOR SHARE NOWAIT`, source.IntegrationID, workspace).Scan(&trust)
	if err != nil {
		return err
	}
	if stageVersion(trust) != source.TrustRevision {
		return collaborationError(409, "stage_execution_target_changed")
	}
	return nil
}
func (s *stageExecutionSourceFixture) Targets(ctx context.Context, tx pgx.Tx, workspace pgtype.UUID) ([]StageExecutionTarget, error) {
	s.mu.Lock()
	target := s.target
	s.mu.Unlock()
	if target.WorkspaceID != util.UUIDToString(workspace) {
		return []StageExecutionTarget{}, nil
	}
	if err := stageFixtureTrust(ctx, tx, target.WorkspaceID, target.Source); err != nil {
		return nil, err
	}
	return []StageExecutionTarget{target}, nil
}
func (s *stageExecutionSourceFixture) AuthorizeExecution(ctx context.Context, tx pgx.Tx, p StageExecutionRequest, target StageExecutionTarget, proof StageExecutionProof) (StageExecutionConsumer, error) {
	value, err := s.decode(proof, "consumer")
	if err != nil {
		return StageExecutionConsumer{}, err
	}
	if !reflect.DeepEqual(value.Request, p) || target.WorkspaceID != util.UUIDToString(p.WorkspaceID) {
		return StageExecutionConsumer{}, collaborationError(403, "forbidden")
	}
	s.mu.Lock()
	current := s.target
	s.mu.Unlock()
	if current.Source.IntegrationID != target.Source.IntegrationID {
		return StageExecutionConsumer{}, collaborationError(403, "forbidden")
	}
	if err = stageFixtureTrust(ctx, tx, current.WorkspaceID, current.Source); err != nil {
		return StageExecutionConsumer{}, err
	}
	return StageExecutionConsumer{Subject: value.Subject, ValidUntil: value.ValidUntil}, nil
}
func (s *stageExecutionSourceFixture) Execute(ctx context.Context, inv StageExecutionInvocation) (StageExecutionProof, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.last = inv
	if s.behavior == "panic" {
		panic("controlled callback panic")
	}
	if s.behavior == "transport" {
		return StageExecutionProof{}, errors.New("controlled transport lost")
	}
	result := s.behavior
	if result == "" {
		result = "succeeded"
	}
	if !stageTargetStateEqual(s.target, inv.Target) {
		result = "not_started"
	}
	observedAt, err := s.observationClock(ctx)
	if err != nil {
		return StageExecutionProof{}, err
	}
	o := StageExecutionObservation{NativeID: "callback:" + inv.IntentID, NativeVersion: "1", Source: inv.Target.Source, ExternalRequestKey: inv.ExternalRequestKey, Result: result, TargetDigest: inv.TargetDigest, ConditionalVersion: inv.Target.ConditionalVersion, BeforeArtifactDigest: inv.Target.CurrentArtifactDigest, ObservedAt: observedAt.UTC(), EvidenceDigest: strings.Repeat("a", 64), Summary: "Controlled integration observation"}
	if result == "succeeded" || result == "failed" {
		operation := "native-operation/" + inv.IntentID
		o.ExternalOperationID = &operation
	}
	if result == "succeeded" {
		digest := inv.ArtifactDigest
		s.target.CurrentArtifactDigest = &digest
		s.target.ConditionalVersion += ".next"
		o.AfterArtifactDigest = &digest
		o.AfterConditionalVersion = s.target.ConditionalVersion
	}
	return s.proof(stageSourceProofFixture{Kind: "execution", Observation: o}), nil
}
func (s *stageExecutionSourceFixture) ExecutionObservation(ctx context.Context, tx pgx.Tx, inv StageExecutionInvocation, proof StageExecutionProof) (StageExecutionObservation, error) {
	v, err := s.decode(proof, "execution")
	if err != nil {
		return v.Observation, err
	}
	s.mu.Lock()
	current := s.target
	s.mu.Unlock()
	if err = stageFixtureTrust(ctx, tx, current.WorkspaceID, current.Source); err != nil {
		return v.Observation, err
	}
	if current.Source.IntegrationID != inv.Target.Source.IntegrationID {
		return v.Observation, collaborationError(403, "forbidden")
	}
	return v.Observation, nil
}
func (s *stageExecutionSourceFixture) MetricObservation(ctx context.Context, tx pgx.Tx, deployment StageExecutionDeployment, proof StageExecutionProof) (StageRollbackMetric, error) {
	v, err := s.decode(proof, "metric")
	if err != nil {
		return v.Metric, err
	}
	return v.Metric, s.CheckMetricCurrent(ctx, tx, deployment.Invocation.Target, v.Metric)
}
func (s *stageExecutionSourceFixture) CheckMetricCurrent(ctx context.Context, tx pgx.Tx, target StageExecutionTarget, metric StageRollbackMetric) error {
	s.mu.Lock()
	source := s.metricSource
	s.mu.Unlock()
	if source != metric.Source || metric.QueryID != "controlled-query" || metric.QueryDigest != strings.Repeat("b", 64) || metric.RuleID != "controlled-rule" || metric.RuleDigest != strings.Repeat("c", 64) || !metric.ValidUntil.Equal(metric.ObservedAt.Add(time.Minute)) {
		return collaborationError(409, "stage_rollback_observation_changed")
	}
	return stageFixtureTrust(ctx, tx, target.WorkspaceID, source)
}

type stageExecutionFixture struct {
	*stageRuntimeFixture
	source     *stageExecutionSourceFixture
	qaDecision string
}

func newStageExecutionFixture(t *testing.T, setup ...func(*testing.T, *stageReviewFixture)) *stageExecutionFixture {
	t.Helper()
	if len(setup) == 0 {
		setup = []func(*testing.T, *stageReviewFixture){stageArtifactReadFixture}
	}
	f := newStageRuntimeFixture(t, true, setup...)
	for _, table := range []string{"collaboration_stage_execution_intent", "collaboration_stage_execution_observation", "collaboration_stage_rollback_observation"} {
		f.fx.Cleanup(t, `DELETE FROM `+table+` WHERE workspace_id=$1`, f.workspace)
	}
	for _, user := range []string{f.stage.qa, f.stage.release} {
		f.fx.Exec(t, `UPDATE member SET role='admin' WHERE workspace_id=$1 AND user_id=$2`, f.workspace, user)
	}
	run, claim, permit := startStageReportRuntime(t, f)
	subject, err := f.svc.RuntimeTestSubject(context.Background(), run, permit.BusinessToken)
	if err != nil {
		t.Fatal(err)
	}
	report := stageRuntimeReport(subject.Subject, claim.Identity)
	report.Report.Cases[0].Outcome = "passed"
	ack, err := f.svc.RecordRuntimeTestReport(context.Background(), run, permit.BusinessToken, report)
	if err != nil {
		t.Fatal(err)
	}
	finishSquadRun(t, f.managedCollaborationFixture, run, claim, permit.BusinessToken, permit.AttemptNonce, "success")
	review, err := f.svc.Queries.GetCollaborationStageReviewHead(f.ctx, db.GetCollaborationStageReviewHeadParams{IssueID: f.issue.Issue.ID, CandidateID: f.candidate.CandidateID, Step: "code_review"})
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().UTC().Add(4 * time.Minute).Truncate(time.Microsecond)
	qa, err := f.svc.DecideStage(f.stage.qaCtx, CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.qaCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: f.stage.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`), ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`), Step: "qa", Decision: "approve", Candidate: f.candidate, Evidence: CollaborationStageReviewEvidence{TestReportID: &ack.ReportID}, DependsOn: []string{util.UUIDToString(review.DecisionID)}, Reason: "Synthetic human QA", ExpiresAt: &expiry})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, f.candidate, "release", qa.StageConfirmation.DecisionID)); err != nil {
		t.Fatal(err)
	}
	deploymentID := f.fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": f.workspace, "installation_id": int64(2), "account_login": "controlled-deployment", "account_type": "User"})
	metricID := f.fx.Insert(t, "github_installation", testutil.Cols{"workspace_id": f.workspace, "installation_id": int64(3), "account_login": "controlled-metrics", "account_type": "User"})
	previous := strings.Repeat("e", 64)
	source := &stageExecutionSourceFixture{key: make([]byte, 32), behavior: "succeeded", target: StageExecutionTarget{WorkspaceID: f.workspace, Environment: "synthetic", Source: StageIntegrationIdentity{Adapter: "controlled-deployment", AdapterVersion: "1", IntegrationID: deploymentID, TrustRevision: "1", ConfigRevision: "1"}, ConcurrencyDomain: "controlled-provider", RemoteInstance: "local", RemoteTenant: "tenant", RemoteTarget: uuid.NewString(), SupportsConditional: true, SupportsOriginalQuery: true, ConditionalVersion: "initial", CurrentArtifactDigest: &previous}, metricSource: StageIntegrationIdentity{Adapter: "controlled-metrics", AdapterVersion: "1", IntegrationID: metricID, TrustRevision: "1", ConfigRevision: "1"}}
	// The synthetic proof and its verifier must share the database clock. The
	// host clock can lead a container clock even when both are synchronized.
	source.observationClock = func(ctx context.Context) (time.Time, error) {
		observed, err := f.svc.Queries.ArtifactExecutionDatabaseTime(ctx)
		return observed.Time, err
	}
	if _, err = rand.Read(source.key); err != nil {
		t.Fatal(err)
	}
	f.svc.StageExecutionSources = source
	return &stageExecutionFixture{stageRuntimeFixture: f, source: source, qaDecision: qa.StageConfirmation.DecisionID}
}
func (f *stageExecutionFixture) release(t *testing.T, lifetime ...time.Duration) (StageExecutionRequest, string) {
	t.Helper()
	view := f.stage.evidence(t, f.stage.releaseCtx, f.candidate)
	if len(view.ExecutionTargets) != 1 {
		t.Fatalf("missing controlled target: %+v", view.ExecutionTargets)
	}
	duration := time.Minute
	if len(lifetime) > 0 {
		duration = lifetime[0]
	}
	expiry := time.Now().UTC().Add(duration).Truncate(time.Microsecond)
	p := CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.releaseCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: f.stage.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`), ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`), Step: "release", Decision: "approve", Candidate: f.candidate, Evidence: CollaborationStageReviewEvidence{Build: &CollaborationStageBuild{ArtifactDigest: view.Candidate.Baseline.Artifact.ArtifactDigest, Environment: view.ExecutionTargets[0].Environment, TargetDigest: view.ExecutionTargets[0].TargetDigest}}, DependsOn: []string{f.qaDecision}, Reason: "Synthetic release approval", ExpiresAt: &expiry}
	receipt, err := f.svc.DecideStage(f.stage.releaseCtx, p)
	if err != nil {
		t.Fatal(err)
	}
	return StageExecutionRequest{WorkspaceID: util.MustParseUUID(f.workspace), IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), ApprovalDecisionID: util.MustParseUUID(receipt.StageConfirmation.DecisionID), Candidate: f.candidate, TargetDigest: p.Evidence.Build.TargetDigest}, receipt.StageConfirmation.DecisionID
}
func (f *stageExecutionFixture) metric(t *testing.T, fact string, native int64, condition string) StageRollbackMetricReceipt {
	t.Helper()
	now := time.Now().UTC().Add(-time.Second).Truncate(time.Microsecond)
	f.source.mu.Lock()
	source := f.source.metricSource
	target := f.source.target
	f.source.mu.Unlock()
	digest, _ := StageExecutionTargetDigest(target)
	factRow, err := f.svc.Queries.GetStageExecutionFact(f.ctx, util.MustParseUUID(fact))
	if err != nil {
		t.Fatal(err)
	}
	intent, err := f.svc.Queries.GetStageExecutionIntent(f.ctx, factRow.IntentID)
	if err != nil {
		t.Fatal(err)
	}
	metric := StageRollbackMetric{NativeID: uuid.NewString(), NativeVersion: stageVersion(native), StreamIdentity: "controlled-metrics", Source: source, DeploymentID: fact, TargetDigest: digest, ArtifactDigest: intent.ArtifactDigest, QueryID: "controlled-query", QueryDigest: strings.Repeat("b", 64), RuleID: "controlled-rule", RuleDigest: strings.Repeat("c", 64), WindowStart: now.Add(-time.Minute), WindowEnd: now, ObservedAt: now, ValidUntil: now.Add(time.Minute), Condition: condition, EvidenceDigest: strings.Repeat("d", 64), Summary: "Synthetic metric observation"}
	receipt, err := f.svc.RecordStageRollbackObservation(context.Background(), util.MustParseUUID(fact), f.source.proof(stageSourceProofFixture{Kind: "metric", Metric: metric}))
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}
func (f *stageExecutionFixture) rollback(t *testing.T, releaseID, fact string, metric StageRollbackMetricReceipt) StageExecutionRequest {
	t.Helper()
	f.source.mu.Lock()
	target := f.source.target
	f.source.mu.Unlock()
	digest, _ := StageExecutionTargetDigest(target)
	previous := strings.Repeat("e", 64)
	expiry := time.Now().UTC().Add(20 * time.Second).Truncate(time.Microsecond)
	p := CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.releaseCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: f.stage.stageInt(t, `SELECT role_revision FROM collaboration_stage_task WHERE issue_id=$1`), ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`), Step: "rollback", Decision: "approve", Candidate: f.candidate, Evidence: CollaborationStageReviewEvidence{DeploymentID: &fact, RollbackTargetDigest: &previous, RollbackObservation: &CollaborationStageRollbackRef{ObservationID: metric.ObservationID, ObservationDigest: metric.ObservationDigest}}, DependsOn: []string{releaseID}, Reason: "Synthetic immediate predecessor rollback", ExpiresAt: &expiry}
	r, err := f.svc.DecideStage(f.stage.releaseCtx, p)
	if err != nil {
		t.Fatal(err)
	}
	return StageExecutionRequest{WorkspaceID: util.MustParseUUID(f.workspace), IssueID: f.issue.Issue.ID, RequestID: uuid.NewString(), ApprovalDecisionID: util.MustParseUUID(r.StageConfirmation.DecisionID), Candidate: f.candidate, TargetDigest: digest}
}
