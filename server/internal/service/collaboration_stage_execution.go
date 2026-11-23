package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/iteration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func stageExecutionRequestValid(p StageExecutionRequest) bool {
	return p.WorkspaceID.Valid && p.WorkspaceID.Bytes != [16]byte{} && p.IssueID.Valid && p.IssueID.Bytes != [16]byte{} && p.ApprovalDecisionID.Valid && p.ApprovalDecisionID.Bytes != [16]byte{} && stageUUID(p.RequestID) && p.Candidate.valid() && stageDigest(p.TargetDigest)
}
func stageExecutionInvocation(row db.CollaborationStageExecutionIntent) (StageExecutionInvocation, error) {
	var inv StageExecutionInvocation
	if json.Unmarshal(row.Invocation, &inv) != nil || inv.IntentID != util.UUIDToString(row.IntentID) || inv.IssueID != util.UUIDToString(row.IssueID) || inv.ApprovalDecisionID != util.UUIDToString(row.ApprovalDecisionID) || inv.Kind != row.Kind || inv.TargetDigest != row.TargetDigest || inv.ArtifactDigest != row.ArtifactDigest || inv.Target.WorkspaceID != util.UUIDToString(row.WorkspaceID) || inv.ExternalRequestKey != util.UUIDToString(row.IntentID) || !inv.DispatchExpiresAt.Equal(row.DispatchExpiresAt.Time) {
		return inv, collaborationError(503, "stage_execution_unavailable")
	}
	digest, err := StageExecutionTargetDigest(inv.Target)
	if err != nil || digest != row.TargetDigest || stageExecutionPhysicalKey(inv.Target) != row.PhysicalTargetKey {
		return inv, collaborationError(503, "stage_execution_unavailable")
	}
	return inv, nil
}
func stageExecutionReceiptFor(ctx context.Context, q *db.Queries, row db.CollaborationStageExecutionIntent) (StageExecutionReceipt, error) {
	out := StageExecutionReceipt{Version: 1, IssueID: util.UUIDToString(row.IssueID), RequestID: util.UUIDToString(row.RequestID), IntentID: util.UUIDToString(row.IntentID), ApprovalDecisionID: util.UUIDToString(row.ApprovalDecisionID), Kind: row.Kind, State: "pending"}
	observation, err := q.GetStageExecutionTerminal(ctx, row.IntentID)
	if errors.Is(err, pgx.ErrNoRows) {
		observation, err = q.GetStageExecutionLatestObservation(ctx, row.IntentID)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.State = observation.Result
	id := util.UUIDToString(observation.ObservationID)
	out.ObservationID = &id
	if observation.Result == "succeeded" || observation.Result == "failed" {
		out.FactID = &id
	}
	return out, nil
}
func stageCheckConsumer(ctx context.Context, tx pgx.Tx, sources StageExecutionSources, p StageExecutionRequest, target StageExecutionTarget, proof StageExecutionProof) (StageExecutionConsumer, error) {
	if sources == nil {
		return StageExecutionConsumer{}, collaborationError(409, "stage_execution_unavailable")
	}
	consumer, err := sources.AuthorizeExecution(ctx, tx, p, target, proof)
	if err != nil {
		return consumer, err
	}
	now, err := db.New(tx).ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return consumer, err
	}
	if !stageBoundedText(consumer.Subject, 256) || consumer.ValidUntil.IsZero() || !now.Time.Before(consumer.ValidUntil) {
		return consumer, collaborationError(403, "forbidden")
	}
	return consumer, nil
}
func (s *CollaborationService) stageExecutionHistory(ctx context.Context, tx pgx.Tx, p StageExecutionRequest, proof StageExecutionProof) (StageExecutionReceipt, bool, error) {
	q := db.New(tx)
	row, err := q.GetStageExecutionIntentByApproval(ctx, db.GetStageExecutionIntentByApprovalParams{IssueID: p.IssueID, ApprovalDecisionID: p.ApprovalDecisionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return StageExecutionReceipt{}, false, nil
	}
	if err != nil {
		return StageExecutionReceipt{}, false, err
	}
	inv, err := stageExecutionInvocation(row)
	if err != nil {
		return StageExecutionReceipt{}, true, err
	}
	consumer, err := stageCheckConsumer(ctx, tx, s.StageExecutionSources, p, inv.Target, proof)
	if err != nil {
		return StageExecutionReceipt{}, true, err
	}
	digest, err := stageExecutionRequestDigest(p)
	if err != nil {
		return StageExecutionReceipt{}, true, err
	}
	if row.WorkspaceID != p.WorkspaceID || row.RequestDigest != digest || row.ConsumerSubject != consumer.Subject {
		return StageExecutionReceipt{}, true, collaborationError(409, "stage_execution_conflict")
	}
	receipt, err := stageExecutionReceiptFor(ctx, q, row)
	return receipt, true, err
}

func (s *CollaborationService) StageExecutionReceipt(ctx context.Context, p StageExecutionRequest, proof StageExecutionProof) (StageExecutionReceipt, error) {
	if !stageExecutionRequestValid(p) {
		return StageExecutionReceipt{}, collaborationError(400, "invalid_stage_request")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	defer rollbackCollaborationQueue(ctx, tx)
	if err = stageExecutionReady(ctx, db.New(tx)); err != nil {
		return StageExecutionReceipt{}, err
	}
	receipt, found, err := s.stageExecutionHistory(ctx, tx, p, proof)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if !found {
		return StageExecutionReceipt{}, collaborationError(404, "not_found")
	}
	return receipt, tx.Commit(ctx)
}

// The internal consumer borrows the existing workflow lock order. The access
// row describes the original signer, not a human caller impersonation; no
// human context or public workflow mutation is constructed for the integration.
func stageLockExecutionRoot(ctx context.Context, tx pgx.Tx, p StageExecutionRequest) (*collaborationWorkflowTx, db.CollaborationStageDecision, error) {
	q := db.New(tx)
	d, err := q.GetCollaborationStageDecision(ctx, db.GetCollaborationStageDecisionParams{IssueID: p.IssueID, DecisionID: p.ApprovalDecisionID})
	if err != nil {
		return nil, d, err
	}
	hint, err := q.GetIssue(ctx, p.IssueID)
	if err != nil {
		return nil, d, err
	}
	if hint.WorkspaceID != p.WorkspaceID {
		return nil, d, collaborationError(403, "forbidden")
	}
	idsParam := db.GetCollaborationWorkflowUserIDsParams{IssueID: p.IssueID, ActorID: d.ActorUserID}
	ids, err := q.GetCollaborationWorkflowUserIDs(ctx, idsParam)
	if err != nil {
		return nil, d, err
	}
	if _, err = q.LockCollaborationUsers(ctx, ids); err != nil {
		return nil, d, err
	}
	if _, err = q.LockWorkspaceForChatSessionCreate(ctx, p.WorkspaceID); err != nil {
		return nil, d, err
	}
	if err = q.LockIssueStatusCatalogShared(ctx, p.WorkspaceID); err != nil {
		return nil, d, err
	}
	if err = iteration.LockWorkspace(ctx, tx, p.WorkspaceID); err != nil {
		return nil, d, err
	}
	if _, err = iteration.LockIssueIteration(ctx, tx, p.WorkspaceID, p.IssueID); err != nil {
		return nil, d, err
	}
	issue, err := q.LockCollaborationIssue(ctx, p.IssueID)
	if err != nil {
		return nil, d, err
	}
	if issue.WorkspaceID != p.WorkspaceID {
		return nil, d, collaborationError(409, "stage_evidence_changed")
	}
	currentIDs, err := q.GetCollaborationWorkflowUserIDs(ctx, idsParam)
	if err != nil {
		return nil, d, err
	}
	locked := map[pgtype.UUID]bool{}
	for _, id := range ids {
		locked[id] = true
	}
	for _, id := range currentIDs {
		if !locked[id] {
			return nil, d, collaborationError(409, "stage_execution_conflict")
		}
	}
	if _, err = q.LockIssueCollaboration(ctx, p.IssueID); err != nil {
		return nil, d, err
	}
	session, err := q.LockCollaborationSession(ctx, p.IssueID)
	if err != nil {
		return nil, d, err
	}
	row, err := q.GetCollaborationAccess(ctx, db.GetCollaborationAccessParams{ViewerID: d.ActorUserID, IssueID: p.IssueID})
	if err != nil {
		return nil, d, err
	}
	access := CollaborationIssueAccess{Issue: row.Issue, Revision: row.CollaborationRevision, LifecycleEpoch: row.LifecycleEpoch, SessionID: row.SessionID, QueueState: row.QueueState, ControlVersion: row.ControlVersion, SourceMember: row.SourceMember, StatusCategory: row.StatusCategory}
	t := &collaborationWorkflowTx{tx: tx, collaborationQueueTx: &collaborationQueueTx{q: q, access: access, session: session}}
	if t.stageClosed() {
		return nil, d, collaborationError(409, "issue_closed")
	}
	return t, d, nil
}

type stageExecutionEntry struct {
	consumed       atomic.Bool
	invocation     StageExecutionInvocation
	databaseTime   time.Time
	monotonicStart time.Time
}

// Prepare owns the only call opportunity. A receipt never contains a permit;
// commit ambiguity, history lookup and a later process cannot recreate entry.
func (s *CollaborationService) PrepareStageExecution(ctx context.Context, p StageExecutionRequest, proof StageExecutionProof) (StageExecutionReceipt, error) {
	if !stageExecutionRequestValid(p) {
		return StageExecutionReceipt{}, collaborationError(400, "invalid_stage_request")
	}
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
	if receipt, found, err := s.stageExecutionHistory(ctx, tx, p, proof); err != nil || found {
		if err != nil {
			return StageExecutionReceipt{}, err
		}
		return receipt, tx.Commit(ctx)
	}
	t, decision, err := stageLockExecutionRoot(ctx, tx, p)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	// The root lock serializes another consumer and A8. Recheck history after it.
	if receipt, found, err := s.stageExecutionHistory(ctx, tx, p, proof); err != nil || found {
		if err != nil {
			return StageExecutionReceipt{}, err
		}
		return receipt, tx.Commit(ctx)
	}
	if decision.Step != "release" && decision.Step != "rollback" {
		return StageExecutionReceipt{}, collaborationError(409, "stage_dependency_invalid")
	}
	state, refusal, err := t.loadStage(ctx, decision.PolicyRevision)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if refusal != nil {
		return StageExecutionReceipt{}, collaborationError(refusal.Status, refusal.Code)
	}
	expectedStage := decision.Step
	if expectedStage == "rollback" {
		expectedStage = "verification"
	}
	if state.task.Stage != expectedStage {
		return StageExecutionReceipt{}, collaborationError(409, "stage_stage_changed")
	}
	candidate, content, refusal, err := t.codeCandidate(ctx, p.Candidate)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if refusal != nil {
		return StageExecutionReceipt{}, collaborationError(refusal.Status, refusal.Code)
	}
	if len(collaboration.StageMissing(state.facts)) != 0 {
		return StageExecutionReceipt{}, collaborationError(409, "stage_needs_confirmation")
	}
	if err = t.lockCodeChild(ctx, content); err != nil {
		return StageExecutionReceipt{}, err
	}
	if err = lockStageCodeEvidence(ctx, tx); err != nil {
		return StageExecutionReceipt{}, err
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	observed, code, err := stageCodeCurrent(ctx, q, t.access, state, candidate, content, now.Time)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if code != "" {
		return StageExecutionReceipt{}, collaborationError(409, code)
	}
	if code = stageCIRefusal(observed.CI); code != "" {
		return StageExecutionReceipt{}, collaborationError(409, code)
	}
	chain, err := stageExecutionApprovalChain(ctx, tx, s.StageExecutionSources, t.access, state, candidate, content, decision, observed, now.Time)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	payload, err := stageExecutionPayload(decision)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	binding := payload.Execution
	if binding == nil || binding.TargetDigest != p.TargetDigest {
		return StageExecutionReceipt{}, collaborationError(409, "stage_execution_target_changed")
	}
	target, err := stageResolveExecutionTarget(ctx, tx, s.StageExecutionSources, p.WorkspaceID, p.TargetDigest)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if !stageTargetStateEqual(target, binding.Target) {
		return StageExecutionReceipt{}, collaborationError(409, "stage_execution_target_changed")
	}
	consumer, err := stageCheckConsumer(ctx, tx, s.StageExecutionSources, p, target, proof)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	_, err = q.GetStageExecutionIntentByRequest(ctx, db.GetStageExecutionIntentByRequestParams{WorkspaceID: p.WorkspaceID, ConsumerSubject: consumer.Subject, RequestID: util.MustParseUUID(p.RequestID)})
	if err == nil {
		return StageExecutionReceipt{}, collaborationError(409, "stage_execution_conflict")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return StageExecutionReceipt{}, err
	}
	physical := stageExecutionPhysicalKey(target)
	if err = lockStageExecutionPhysical(ctx, q, physical); err != nil {
		return StageExecutionReceipt{}, err
	}
	pending, err := q.HasUnresolvedStageExecution(ctx, physical)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if pending {
		return StageExecutionReceipt{}, collaborationError(409, "stage_execution_pending")
	}
	// Re-observe the retained build using the actual original issuer's explicit
	// credential. The integration's independently authenticated proof is separate.
	var signer stageSignerCredential
	if json.Unmarshal(decision.SignerCredential, &signer) != nil {
		return StageExecutionReceipt{}, collaborationError(409, "stage_signer_changed")
	}
	if content.Baseline.Artifact == nil {
		return StageExecutionReceipt{}, collaborationError(409, "stage_evidence_unavailable")
	}
	child, err := q.GetIssue(ctx, util.MustParseUUID(content.Baseline.ChildIssueID))
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	artifact, err := stageArtifactObservationAs(ctx, tx, child, util.MustParseUUID(content.Baseline.Artifact.ApprovalID), content.Baseline.PullRequest.HeadSHA, signer.credential(util.UUIDToString(decision.ActorUserID)))
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if artifact.Verdict != "pass" || artifact.ArtifactDigest != content.Baseline.Artifact.ArtifactDigest {
		return StageExecutionReceipt{}, collaborationError(409, "stage_evidence_changed")
	}
	signers, code, err := lockStageFinalSigners(ctx, tx, t.access, state, chain)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if code != "" {
		return StageExecutionReceipt{}, collaborationError(409, code)
	}
	finalTarget, err := stageResolveExecutionTarget(ctx, tx, s.StageExecutionSources, p.WorkspaceID, p.TargetDigest)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if !stageTargetStateEqual(finalTarget, target) {
		return StageExecutionReceipt{}, collaborationError(409, "stage_execution_target_changed")
	}
	finalConsumer, err := stageCheckConsumer(ctx, tx, s.StageExecutionSources, p, target, proof)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if finalConsumer.Subject != consumer.Subject {
		return StageExecutionReceipt{}, collaborationError(403, "forbidden")
	}
	monotonic := time.Now()
	final, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	expiry := stageEarlier(final.Time.Add(30*time.Second), stageEarlier(consumer.ValidUntil, finalConsumer.ValidUntil))
	expiry = stageEarlier(expiry, stageMaxExpiry(final.Time, observed.CI, nil, content))
	for i, d := range chain {
		expiry = stageEarlier(expiry, d.ExpiresAt.Time)
		if signers[i].ExpiresAt != nil {
			expiry = stageEarlier(expiry, *signers[i].ExpiresAt)
		}
	}
	if binding.Metric != nil {
		row, err := q.GetStageRollbackObservation(ctx, util.MustParseUUID(binding.Metric.ObservationID))
		if err != nil {
			return StageExecutionReceipt{}, err
		}
		metric, err := decodeStageMetric(row)
		if err != nil {
			return StageExecutionReceipt{}, err
		}
		expiry = stageEarlier(expiry, metric.ValidUntil)
	}
	if !final.Time.Before(expiry) {
		return StageExecutionReceipt{}, collaborationError(409, "stage_decision_expired")
	}
	intentID := dbid.NewV7()
	kind := "deployment"
	if decision.Step == "rollback" {
		kind = "rollback"
	}
	inv := StageExecutionInvocation{IntentID: util.UUIDToString(intentID), IssueID: util.UUIDToString(p.IssueID), ApprovalDecisionID: util.UUIDToString(p.ApprovalDecisionID), Kind: kind, Target: target, TargetDigest: p.TargetDigest, ArtifactDigest: binding.ArtifactDigest, ExternalRequestKey: util.UUIDToString(intentID), DispatchExpiresAt: expiry, DeploymentID: binding.DeploymentID}
	raw, err := json.Marshal(inv)
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	ids := make([]string, 0, len(chain))
	for _, d := range chain {
		ids = append(ids, util.UUIDToString(d.DecisionID))
	}
	dependencies, _ := json.Marshal(ids)
	digest, _ := stageExecutionRequestDigest(p)
	row, err := q.CreateStageExecutionIntent(ctx, db.CreateStageExecutionIntentParams{IntentID: intentID, WorkspaceID: p.WorkspaceID, IssueID: p.IssueID, CandidateID: candidate.CandidateID, CandidateVersion: candidate.Version, CandidateDigest: candidate.ContentDigest, LifecycleEpoch: t.access.LifecycleEpoch, RoleRevision: state.task.RoleRevision, PolicyRevision: state.policy.Revision, ApprovalDecisionID: p.ApprovalDecisionID, RequestID: util.MustParseUUID(p.RequestID), RequestDigest: digest, ConsumerSubject: consumer.Subject, PhysicalTargetKey: physical, TargetDigest: p.TargetDigest, ArtifactDigest: binding.ArtifactDigest, Kind: kind, Invocation: raw, Dependencies: dependencies, AdmittedAt: final, DispatchExpiresAt: pgtype.Timestamptz{Time: expiry, Valid: true}})
	if err != nil {
		return StageExecutionReceipt{}, err
	}
	if err = recordCollaborationExecutionChange(ctx, q, p.IssueID, pgtype.UUID{}, pgtype.UUID{}, "stage_execution_admitted"); err != nil {
		return StageExecutionReceipt{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return StageExecutionReceipt{}, err
	}
	return s.enterStageExecution(ctx, row, &stageExecutionEntry{invocation: inv, databaseTime: final.Time, monotonicStart: monotonic})
}

func (s *CollaborationService) enterStageExecution(ctx context.Context, row db.CollaborationStageExecutionIntent, entry *stageExecutionEntry) (StageExecutionReceipt, error) {
	if !entry.consumed.CompareAndSwap(false, true) {
		return StageExecutionReceipt{}, collaborationError(409, "stage_execution_conflict")
	}
	if ctx.Err() != nil || time.Since(entry.monotonicStart) >= entry.invocation.DispatchExpiresAt.Sub(entry.databaseTime) {
		finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.recordStageInternalObservation(finalCtx, row, entry, "not_started")
	}
	var proof StageExecutionProof
	func() {
		defer func() {
			if recover() != nil {
				proof = StageExecutionProof{}
			}
		}()
		proof, _ = s.StageExecutionSources.Execute(ctx, entry.invocation)
	}()
	// Recording and its uncertainty fallback share one post-callback budget.
	// Exhausting it cannot authorize another finalization window or invocation.
	finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if len(proof.Payload) > 0 || len(proof.Authentication) > 0 {
		receipt, err := s.RecordStageExecutionObservation(finalCtx, row.IntentID, proof)
		if err == nil {
			return receipt, nil
		}
	}
	// A local error, cancellation or panic alone proves no outcome.
	return s.recordStageInternalObservation(finalCtx, row, entry, "unknown")
}

func stageExecutionDeploymentFor(ctx context.Context, q *db.Queries, factID, issue, candidate pgtype.UUID) (StageExecutionDeployment, db.CollaborationStageExecutionIntent, error) {
	var out StageExecutionDeployment
	fact, err := q.GetStageExecutionFact(ctx, factID)
	if err != nil {
		return out, db.CollaborationStageExecutionIntent{}, err
	}
	intent, err := q.GetStageExecutionIntent(ctx, fact.IntentID)
	if err != nil {
		return out, intent, err
	}
	if fact.Result != "succeeded" || intent.Kind != "deployment" || intent.IssueID != issue || intent.CandidateID != candidate {
		return out, intent, collaborationError(409, "stage_execution_fact_missing")
	}
	inv, err := stageExecutionInvocation(intent)
	if err != nil {
		return out, intent, err
	}
	var observation StageExecutionObservation
	if json.Unmarshal(fact.Observation, &observation) != nil || observation.Result != "succeeded" || observation.AfterArtifactDigest == nil || *observation.AfterArtifactDigest != intent.ArtifactDigest {
		return out, intent, collaborationError(503, "stage_execution_unavailable")
	}
	return StageExecutionDeployment{FactID: util.UUIDToString(factID), Invocation: inv, Observation: observation}, intent, nil
}
