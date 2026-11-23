package service

import (
	"context"
	"encoding/json"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStageExecutionTargetAndPhysicalIdentity(t *testing.T) {
	target := StageExecutionTarget{WorkspaceID: uuid.NewString(), Environment: "synthetic", Source: StageIntegrationIdentity{Adapter: "controlled", AdapterVersion: "1", IntegrationID: uuid.NewString(), TrustRevision: "1", ConfigRevision: "1"}, ConcurrencyDomain: "controlled-provider", RemoteInstance: "instance", RemoteTenant: "tenant", RemoteTarget: "target", SupportsConditional: true, SupportsOriginalQuery: true, ConditionalVersion: "state-1"}
	original, err := StageExecutionTargetDigest(target)
	if err != nil {
		t.Fatal(err)
	}
	physical := stageExecutionPhysicalKey(target)
	target.WorkspaceID = uuid.NewString()
	target.Source.IntegrationID = uuid.NewString()
	target.Source.TrustRevision = "2"
	target.Source.ConfigRevision = "2"
	target.Environment = "alias"
	changed, err := StageExecutionTargetDigest(target)
	if err != nil || original == changed || stageExecutionPhysicalKey(target) != physical {
		t.Fatalf("local binding bypassed physical domain: %s %v", changed, err)
	}
	before := changed
	target.ConditionalVersion = "state-2"
	artifact := strings.Repeat("a", 64)
	target.CurrentArtifactDigest = &artifact
	changed, err = StageExecutionTargetDigest(target)
	if err != nil || changed != before {
		t.Fatal("execution state changed reviewed configuration identity")
	}
	target.RemoteTarget = "other"
	if stageExecutionPhysicalKey(target) == physical {
		t.Fatal("unrelated physical targets collided")
	}
	target.ConcurrencyDomain = ""
	if _, err = StageExecutionTargetDigest(target); err == nil {
		t.Fatal("unsupported physical domain was accepted")
	}
}

func TestStageExecutionReleaseRollbackAndOriginalReceipt(t *testing.T) {
	f := newStageExecutionFixture(t)
	p, release := f.release(t)
	bad := f.source.consumer(p)
	bad.Authentication[0] ^= 0xff
	if _, err := f.svc.PrepareStageExecution(context.Background(), p, bad); !collaborationErrorCode(err, "forbidden") {
		t.Fatalf("unauthenticated consumer entered: %v", err)
	}
	receipt, err := f.svc.PrepareStageExecution(context.Background(), p, f.source.consumer(p))
	if err != nil || receipt.State != "succeeded" || receipt.FactID == nil {
		t.Fatalf("deployment: %+v %v", receipt, err)
	}
	if f.source.calls != 1 || f.stage.view(t).Stage.Current != "release" {
		t.Fatal("deployment repeated or advanced the human stage")
	}
	replay, err := f.svc.PrepareStageExecution(context.Background(), p, f.source.consumer(p))
	if err != nil || replay.IntentID != receipt.IntentID || replay.FactID == nil || *replay.FactID != *receipt.FactID || f.source.calls != 1 {
		t.Fatalf("historical replay dispatched: %+v %v", replay, err)
	}
	if _, err = f.svc.RevokeStageDecision(f.stage.releaseCtx, CollaborationStageRevocationParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.releaseCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, DecisionID: p.ApprovalDecisionID, Reason: "Cannot cancel an admitted external operation"}); !collaborationErrorCode(err, "stage_decision_not_revocable") {
		t.Fatalf("consumed approval revoked: %v", err)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, f.candidate, "verification", release)); err != nil {
		t.Fatalf("own authenticated after-state did not unlock verification: %v", err)
	}
	metric := f.metric(t, *receipt.FactID, 1, "anomaly")
	rollback := f.rollback(t, release, *receipt.FactID, metric)
	rolled, err := f.svc.PrepareStageExecution(context.Background(), rollback, f.source.consumer(rollback))
	if err != nil || rolled.State != "succeeded" || rolled.Kind != "rollback" || rolled.FactID == nil || f.source.calls != 2 {
		t.Fatalf("rollback: %+v %v", rolled, err)
	}
	view := f.stage.evidence(t, f.ctx, f.candidate)
	if len(view.ExecutionFacts) != 2 || len(view.ExecutionOperations) != 2 || len(view.RollbackObservations) != 1 {
		t.Fatalf("execution projections: %+v", view)
	}
	if view.ExecutionFacts[0].OperationID == "" || stageUUID(view.ExecutionFacts[0].OperationID) {
		t.Fatal("native operation identity was coerced to UUID")
	}
}

func TestStageExecutionUnknownBlocksRebindingAndCrossWorkspace(t *testing.T) {
	first := newStageExecutionFixture(t)
	first.source.behavior = "unknown"
	p, _ := first.release(t)
	initial, err := first.svc.PrepareStageExecution(context.Background(), p, first.source.consumer(p))
	if err != nil || initial.State != "unknown" || initial.FactID != nil {
		t.Fatalf("unknown observation: %+v %v", initial, err)
	}
	first.source.target.Source.ConfigRevision = "2"
	newRequest, _ := first.release(t)
	if _, err = first.svc.PrepareStageExecution(context.Background(), newRequest, first.source.consumer(newRequest)); !collaborationErrorCode(err, "stage_execution_pending") {
		t.Fatalf("config binding bypassed unresolved physical target: %v", err)
	}
	second := newStageExecutionFixture(t)
	second.source.target.RemoteTarget = first.source.target.RemoteTarget
	secondRequest, _ := second.release(t)
	if _, err = second.svc.PrepareStageExecution(context.Background(), secondRequest, second.source.consumer(secondRequest)); !collaborationErrorCode(err, "stage_execution_pending") {
		t.Fatalf("another workspace bypassed physical target: %v", err)
	}
	if first.source.calls != 1 || second.source.calls != 0 {
		t.Fatal("unresolved target was invoked again")
	}
	second.source.target.RemoteTarget = uuid.NewString()
	unrelated, _ := second.release(t)
	if _, err = second.svc.PrepareStageExecution(context.Background(), unrelated, second.source.consumer(unrelated)); err != nil {
		t.Fatalf("same display name blocked unrelated physical target: %v", err)
	}
}

func TestStageExecutionCommitUncertaintyAndPreEntryCancellation(t *testing.T) {
	for _, kind := range []string{"unknown_commit", "cancel_after_commit", "panic", "transport"} {
		t.Run(kind, func(t *testing.T) {
			f := newStageExecutionFixture(t)
			p, _ := f.release(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if kind == "unknown_commit" {
				f.svc.TxStarter = &queueFaultStarter{TxStarter: f.pool, lostCommit: true}
			}
			if kind == "cancel_after_commit" {
				f.svc.TxStarter = &afterCommitTxStarter{pool: f.pool, afterCommit: cancel}
			}
			if kind == "panic" || kind == "transport" {
				f.source.behavior = kind
			}
			receipt, err := f.svc.PrepareStageExecution(ctx, p, f.source.consumer(p))
			f.svc.TxStarter = f.pool
			if kind == "unknown_commit" {
				if err == nil || f.source.calls != 0 {
					t.Fatalf("ambiguous commit invoked or returned success: %+v %v", receipt, err)
				}
			} else if kind == "cancel_after_commit" {
				if err != nil || receipt.State != "not_started" || receipt.FactID != nil || f.source.calls != 0 {
					t.Fatalf("pre-entry cancellation: %+v %v", receipt, err)
				}
			} else if err != nil || receipt.State != "unknown" || receipt.FactID != nil || f.source.calls != 1 {
				t.Fatalf("post-entry uncertainty: %+v %v", receipt, err)
			}
			count := f.source.calls
			history, err := f.svc.StageExecutionReceipt(context.Background(), p, f.source.consumer(p))
			if err != nil || f.source.calls != count || history.IntentID == "" {
				t.Fatalf("original lookup changed invocation authority: %+v %v", history, err)
			}
		})
	}
}

func TestStageExecutionSigningStateDriftPreventsDispatch(t *testing.T) {
	f := newStageExecutionFixture(t)
	p, _ := f.release(t)
	f.source.target.ConditionalVersion = "outside-deployment"
	digest := strings.Repeat("f", 64)
	f.source.target.CurrentArtifactDigest = &digest
	if _, err := f.svc.PrepareStageExecution(context.Background(), p, f.source.consumer(p)); !collaborationErrorCode(err, "stage_execution_target_changed") || f.source.calls != 0 {
		t.Fatalf("unchanged config digest hid external state drift: %v", err)
	}
	if f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_execution_intent WHERE issue_id=$1`, f.issue.Issue.ID) != 0 {
		t.Fatal("rejected state drift left an intent")
	}
}

func TestStageExecutionMetricAmbiguityInvalidatesExistingRollback(t *testing.T) {
	for _, secondCondition := range []string{"healthy", "unknown", "anomaly"} {
		t.Run(secondCondition, func(t *testing.T) {
			f := newStageExecutionFixture(t)
			p, release := f.release(t)
			deployed, err := f.svc.PrepareStageExecution(context.Background(), p, f.source.consumer(p))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, f.candidate, "verification", release)); err != nil {
				t.Fatal(err)
			}
			first := f.metric(t, *deployed.FactID, 1, "anomaly")
			rollback := f.rollback(t, release, *deployed.FactID, first)
			f.metric(t, *deployed.FactID, 1, secondCondition)
			if _, err = f.svc.PrepareStageExecution(context.Background(), rollback, f.source.consumer(rollback)); !collaborationErrorCode(err, "stage_rollback_observation_unavailable") {
				t.Fatalf("contradictory native version consumed old anomaly: %v", err)
			}
			view := f.stage.evidence(t, f.ctx, f.candidate)
			for _, row := range view.RollbackObservations {
				if row.State != "ambiguous" {
					t.Fatalf("contradiction was not retained as ambiguous: %+v", row)
				}
			}
			newer := f.metric(t, *deployed.FactID, 2, "anomaly")
			if _, err = f.svc.PrepareStageExecution(context.Background(), rollback, f.source.consumer(rollback)); !collaborationErrorCode(err, "stage_rollback_observation_unavailable") {
				t.Fatalf("new observation revived old approval: %v", err)
			}
			updated := f.rollback(t, release, *deployed.FactID, newer)
			if _, err = f.svc.PrepareStageExecution(context.Background(), updated, f.source.consumer(updated)); err != nil {
				t.Fatalf("strictly newer explicitly approved anomaly failed: %v", err)
			}
		})
	}
}

func TestStageExecutionConcurrentPrepareHasOneEntry(t *testing.T) {
	f := newStageExecutionFixture(t)
	p, _ := f.release(t)
	proof := f.source.consumer(p)
	var wg sync.WaitGroup
	receipts := make(chan StageExecutionReceipt, 4)
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := f.svc.PrepareStageExecution(context.Background(), p, proof)
			receipts <- r
			errs <- err
		}()
	}
	wg.Wait()
	close(receipts)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	intent := ""
	for r := range receipts {
		if intent == "" {
			intent = r.IntentID
		}
		if intent != r.IntentID {
			t.Fatal("concurrent approval admitted multiple intents")
		}
	}
	if f.source.calls != 1 || f.fx.Count(t, `SELECT count(*) FROM collaboration_stage_execution_intent WHERE issue_id=$1`, f.issue.Issue.ID) != 1 {
		t.Fatal("concurrent prepare entered more than once")
	}
	changed := p
	changed.RequestID = uuid.NewString()
	if _, err := f.svc.PrepareStageExecution(context.Background(), changed, f.source.consumer(changed)); !collaborationErrorCode(err, "stage_execution_conflict") {
		t.Fatalf("replacement key restored consumed opportunity: %v", err)
	}
}

func TestStageExecutionCommitDelayRetiresExpiredEntry(t *testing.T) {
	f := newStageExecutionFixture(t)
	p, _ := f.release(t, 2*time.Second)
	decision, err := f.svc.Queries.GetCollaborationStageDecision(f.ctx, db.GetCollaborationStageDecisionParams{IssueID: p.IssueID, DecisionID: p.ApprovalDecisionID})
	if err != nil {
		t.Fatal(err)
	}
	f.svc.TxStarter = &afterCommitTxStarter{pool: f.pool, afterCommit: func() { time.Sleep(time.Until(decision.ExpiresAt.Time) + 20*time.Millisecond) }}
	receipt, err := f.svc.PrepareStageExecution(context.Background(), p, f.source.consumer(p))
	f.svc.TxStarter = f.pool
	if err != nil || receipt.State != "not_started" || receipt.FactID != nil || f.source.calls != 0 {
		t.Fatalf("expired committed entry was dispatched: %+v %v", receipt, err)
	}
}

func TestStageExecutionLateAuthenticatedFactDoesNotRenewApproval(t *testing.T) {
	f := newStageExecutionFixture(t)
	p, release := f.release(t, 2*time.Second)
	f.source.behavior = "unknown"
	initial, err := f.svc.PrepareStageExecution(context.Background(), p, f.source.consumer(p))
	if err != nil {
		t.Fatal(err)
	}
	row, err := f.svc.Queries.GetStageExecutionIntent(f.ctx, util.MustParseUUID(initial.IntentID))
	if err != nil {
		t.Fatal(err)
	}
	inv, err := stageExecutionInvocation(row)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Until(row.DispatchExpiresAt.Time) + 20*time.Millisecond)
	operation := "late/native-operation"
	artifact := inv.ArtifactDigest
	observation := StageExecutionObservation{NativeID: "late-result", NativeVersion: "2", Source: inv.Target.Source, ExternalRequestKey: inv.ExternalRequestKey, ExternalOperationID: &operation, Result: "succeeded", TargetDigest: inv.TargetDigest, ConditionalVersion: inv.Target.ConditionalVersion, AfterConditionalVersion: "late-after", BeforeArtifactDigest: inv.Target.CurrentArtifactDigest, AfterArtifactDigest: &artifact, ObservedAt: time.Now().UTC(), EvidenceDigest: strings.Repeat("d", 64), Summary: "Authenticated late result"}
	f.source.target.ConditionalVersion = observation.AfterConditionalVersion
	f.source.target.CurrentArtifactDigest = &artifact
	proof := f.source.proof(stageSourceProofFixture{Kind: "execution", Observation: observation})
	fact, err := f.svc.RecordStageExecutionObservation(context.Background(), row.IntentID, proof)
	if err != nil || fact.FactID == nil {
		t.Fatalf("late authenticated history lost: %+v %v", fact, err)
	}
	replay, err := f.svc.RecordStageExecutionObservation(context.Background(), row.IntentID, proof)
	if err != nil || replay.FactID == nil || *replay.FactID != *fact.FactID {
		t.Fatalf("equivalent observation duplicated fact: %+v %v", replay, err)
	}
	observation.Result = "failed"
	if _, err = f.svc.RecordStageExecutionObservation(context.Background(), row.IntentID, f.source.proof(stageSourceProofFixture{Kind: "execution", Observation: observation})); !collaborationErrorCode(err, "stage_execution_conflict") {
		t.Fatalf("contradictory terminal overwrote history: %v", err)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, f.candidate, "verification", release)); !collaborationErrorCode(err, "stage_decision_expired") {
		t.Fatalf("late fact renewed expired signature: %v", err)
	}
	if f.source.calls != 1 || f.stage.view(t).Stage.Current != "release" {
		t.Fatal("late result reran or advanced")
	}
}

func TestStageExecutionProductionUnavailableAndSourceDeniedProjection(t *testing.T) {
	f := newStageExecutionFixture(t)
	p, _ := f.release(t)
	f.svc.StageExecutionSources = nil
	if _, err := f.svc.PrepareStageExecution(context.Background(), p, StageExecutionProof{}); !collaborationErrorCode(err, "stage_execution_unavailable") {
		t.Fatalf("production missing source admitted execution: %v", err)
	}
	view := f.stage.evidence(t, f.stage.releaseCtx, f.candidate)
	raw, _ := json.Marshal(view)
	for _, field := range []string{"execution_targets", "execution_operations", "rollback_observations"} {
		if !strings.Contains(string(raw), `"`+field+`":[]`) {
			t.Fatalf("2c omitted known empty array %s: %s", field, raw)
		}
	}
	f.svc.StageExecutionSources = f.source
	f.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.stage.release)
	denied := f.stage.evidence(t, f.stage.releaseCtx, f.candidate)
	if denied.Candidate.Baseline != nil || denied.BlockedEvidence != 1 || len(denied.ExecutionTargets) != 0 || len(denied.ExecutionOperations) != 0 || len(denied.RollbackObservations) != 0 || len(denied.ExecutionFacts) != 0 {
		t.Fatalf("source denial exposed execution evidence: %+v", denied)
	}
}

func TestStageExecutionOriginalArtifactReaderRevocation(t *testing.T) {
	f := newStageExecutionFixture(t)
	p, _ := f.release(t)
	f.fx.Exec(t, `UPDATE member SET role='member' WHERE workspace_id=$1 AND user_id=$2`, f.workspace, f.stage.reviewer)
	hidden := f.stage.evidence(t, f.stage.reviewerCtx, f.candidate)
	if hidden.Candidate.Baseline != nil {
		t.Fatal("original reviewer still has artifact read authority")
	}
	if _, err := f.svc.PrepareStageExecution(context.Background(), p, f.source.consumer(p)); !collaborationErrorCode(err, "stage_signer_changed") || f.source.calls != 0 {
		t.Fatalf("another manager consumer substituted for revoked original artifact reader: %v calls=%d", err, f.source.calls)
	}
}

func TestStageExecutionReturnKeepsReadableReportWithoutAdapter(t *testing.T) {
	f := newStageExecutionFixture(t)
	qa, err := f.svc.Queries.GetCollaborationStageDecision(f.ctx, db.GetCollaborationStageDecisionParams{IssueID: f.issue.Issue.ID, DecisionID: util.MustParseUUID(f.qaDecision)})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := stageExecutionPayload(qa)
	if err != nil {
		t.Fatal(err)
	}
	f.svc.StageExecutionSources = nil
	p := CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.releaseCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`), Step: "release", Decision: "return", Candidate: f.candidate, Evidence: CollaborationStageReviewEvidence{TestReportID: payload.Evidence.TestReportID}, DependsOn: []string{}, Reason: "Return with readable historical report"}
	if _, err = f.svc.DecideStage(f.stage.releaseCtx, p); err != nil {
		t.Fatalf("optional readable report incorrectly needs execution adapter/passing release: %v", err)
	}
}

func TestStageExecutionMetricReverseOrderAndOlderArrivalCannotResolveAmbiguity(t *testing.T) {
	f := newStageExecutionFixture(t)
	p, release := f.release(t)
	deployed, err := f.svc.PrepareStageExecution(context.Background(), p, f.source.consumer(p))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.AdvanceStage(f.ctx, f.stage.advanceParams(t, f.candidate, "verification", release)); err != nil {
		t.Fatal(err)
	}
	f.metric(t, *deployed.FactID, 2, "healthy")
	anomaly := f.metric(t, *deployed.FactID, 2, "anomaly")
	f.metric(t, *deployed.FactID, 1, "anomaly")
	previous := strings.Repeat("e", 64)
	expiry := time.Now().UTC().Add(10 * time.Second).Truncate(time.Microsecond)
	decision := CollaborationStageReviewParams{CollaborationWorkflowRequest: f.stage.req(t, f.stage.releaseCtx), ExpectedPolicyRevision: 1, ExpectedRoleRevision: 1, ExpectedStageRevision: f.stage.stageInt(t, `SELECT stage_revision FROM collaboration_stage_task WHERE issue_id=$1`), Step: "rollback", Decision: "approve", Candidate: f.candidate, Evidence: CollaborationStageReviewEvidence{DeploymentID: deployed.FactID, RollbackTargetDigest: &previous, RollbackObservation: &CollaborationStageRollbackRef{ObservationID: anomaly.ObservationID, ObservationDigest: anomaly.ObservationDigest}}, DependsOn: []string{release}, Reason: "Cannot consume ambiguous original observation", ExpiresAt: &expiry}
	if _, err = f.svc.DecideStage(f.stage.releaseCtx, decision); !collaborationErrorCode(err, "stage_rollback_observation_unavailable") {
		t.Fatalf("reverse/older delivery selected an ambiguous winner: %v", err)
	}
	if f.source.calls != 1 {
		t.Fatal("metric recommendation executed rollback")
	}
}
