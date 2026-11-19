package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func stageExecutionPayload(d db.CollaborationStageDecision) (stageReviewPayload, error) {
	var p stageReviewPayload
	if json.Unmarshal(d.Payload, &p) != nil || !stageDistinctIDs(p.DependsOn) {
		return p, collaborationError(409, "stage_dependency_invalid")
	}
	return p, nil
}

// This validates retained human issuers explicitly. It never installs an
// approver as the integration consumer or manufactures a request context.
func stageExecutionApprovalChain(ctx context.Context, tx pgx.Tx, sources StageExecutionSources, access CollaborationIssueAccess, state *collaborationStageState, candidate db.CollaborationStageCandidate, content stageCodeContent, d db.CollaborationStageDecision, observed CollaborationStageObserved, now time.Time) ([]db.CollaborationStageDecision, error) {
	q := db.New(tx)
	if d.CandidateID != candidate.CandidateID || d.CandidateVersion.Int64 != candidate.Version || d.ContentDigest.String != candidate.ContentDigest || d.Decision != "approved" || d.LifecycleEpoch != access.LifecycleEpoch {
		return nil, collaborationError(409, "stage_evidence_changed")
	}
	if !d.ExpiresAt.Valid || !now.Before(d.ExpiresAt.Time) {
		return nil, collaborationError(409, "stage_decision_expired")
	}
	if d.RoleRevision != state.task.RoleRevision {
		return nil, collaborationError(409, "stage_roles_changed")
	}
	if d.PolicyRevision != state.policy.Revision || !state.policy.Enabled {
		return nil, collaborationError(409, "stage_policy_changed")
	}
	_, err := q.GetCollaborationStageRevocation(ctx, db.GetCollaborationStageRevocationParams{IssueID: d.IssueID, DecisionID: d.DecisionID})
	if err == nil {
		return nil, collaborationError(409, "stage_decision_revoked")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	head, err := q.GetCollaborationStageReviewHead(ctx, db.GetCollaborationStageReviewHeadParams{IssueID: d.IssueID, CandidateID: d.CandidateID, Step: d.Step})
	if err != nil {
		return nil, err
	}
	if head.DecisionID != d.DecisionID {
		return nil, collaborationError(409, "stage_evidence_changed")
	}
	valid, signer, err := stageSignerCurrent(ctx, q, state.role, d, now)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, collaborationError(409, "stage_signer_changed")
	}
	payload, err := stageExecutionPayload(d)
	if err != nil {
		return nil, err
	}
	var chain []db.CollaborationStageDecision
	if d.Step == "code_review" {
		if len(payload.DependsOn) != 0 || payload.Evidence.CIDigest == nil || observed.CI.CIDigest == nil || *payload.Evidence.CIDigest != *observed.CI.CIDigest || stageCIRefusal(observed.CI) != "" {
			return nil, collaborationError(409, "stage_evidence_changed")
		}
	} else {
		if len(payload.DependsOn) != 1 {
			return nil, collaborationError(409, "stage_dependency_invalid")
		}
		previous, err := q.GetCollaborationStageDecision(ctx, db.GetCollaborationStageDecisionParams{IssueID: access.Issue.ID, DecisionID: util.MustParseUUID(payload.DependsOn[0])})
		if err != nil {
			return nil, err
		}
		expected := map[string]string{"qa": "code_review", "release": "qa", "rollback": "release"}[d.Step]
		if expected == "" || previous.Step != expected {
			return nil, collaborationError(409, "stage_dependency_invalid")
		}
		chain, err = stageExecutionApprovalChain(ctx, tx, sources, access, state, candidate, content, previous, observed, now)
		if err != nil {
			return nil, err
		}
	}
	switch d.Step {
	case "qa":
		if payload.Evidence.TestReportID == nil {
			return nil, collaborationError(409, "stage_test_report_unusable")
		}
		projectIssuerAccess := completionCollaborationAccess
		if _, readProjection := tx.(collaborationStageEvidenceReadTx); readProjection {
			projectIssuerAccess = readCompletionCollaborationAccess
		}
		issuerAccess, err := projectIssuerAccess(ctx, tx, access.Issue, d.ActorUserID)
		if err != nil {
			return nil, err
		}
		sourceAccess := access
		sourceAccess.SourceMember = issuerAccess.SourceMember
		credential := signer.credential(util.UUIDToString(d.ActorUserID))
		var code string
		if _, readProjection := tx.(collaborationStageEvidenceReadTx); readProjection {
			_, code, err = stageQAReportReadAs(ctx, tx, sourceAccess, candidate, *payload.Evidence.TestReportID, credential)
		} else {
			_, code, err = stageQAReportAs(ctx, tx, sourceAccess, candidate, *payload.Evidence.TestReportID, true, credential)
		}
		if err != nil {
			return nil, err
		}
		if code != "" {
			return nil, collaborationError(409, code)
		}
	case "release", "rollback":
		binding := payload.Execution
		if binding == nil || !stageDigest(binding.TargetDigest) || !stageDigest(binding.ArtifactDigest) {
			return nil, collaborationError(409, "stage_execution_unavailable")
		}
		digest, err := StageExecutionTargetDigest(binding.Target)
		if err != nil || digest != binding.TargetDigest {
			return nil, collaborationError(409, "stage_execution_target_changed")
		}
		currentTarget, err := stageResolveExecutionTarget(ctx, tx, sources, access.Issue.WorkspaceID, binding.TargetDigest)
		if err != nil {
			return nil, err
		}
		if _, e := q.GetStageExecutionIntentByApproval(ctx, db.GetStageExecutionIntentByApprovalParams{IssueID: d.IssueID, ApprovalDecisionID: d.DecisionID}); errors.Is(e, pgx.ErrNoRows) {
			if !stageTargetStateEqual(currentTarget, binding.Target) {
				return nil, collaborationError(409, "stage_execution_target_changed")
			}
		} else if e != nil {
			return nil, e
		}
		if d.Step == "release" {
			if payload.Evidence.Build == nil || payload.Evidence.Build.TargetDigest != binding.TargetDigest || payload.Evidence.Build.ArtifactDigest != binding.ArtifactDigest || payload.Evidence.Build.Environment != binding.Target.Environment || content.Baseline.Artifact == nil || content.Baseline.Artifact.ArtifactDigest != binding.ArtifactDigest {
				return nil, collaborationError(409, "stage_evidence_changed")
			}
			qa, _ := stageExecutionPayload(chain[len(chain)-1])
			if qa.Evidence.TestReportID == nil {
				return nil, collaborationError(409, "stage_test_report_unusable")
			}
			event, err := q.GetCollaborationStageTestReportEvent(ctx, db.GetCollaborationStageTestReportEventParams{IssueID: access.Issue.ID, ID: util.MustParseUUID(*qa.Evidence.TestReportID)})
			if err != nil {
				return nil, err
			}
			report, err := decodeStoredTestReport(event)
			if err != nil {
				return nil, err
			}
			if report.Report.Subject.ArtifactDigest == nil || *report.Report.Subject.ArtifactDigest != binding.ArtifactDigest {
				return nil, collaborationError(409, "stage_evidence_changed")
			}
		} else {
			if binding.DeploymentID == nil || binding.Metric == nil || payload.Evidence.DeploymentID == nil || *payload.Evidence.DeploymentID != *binding.DeploymentID || payload.Evidence.RollbackTargetDigest == nil || *payload.Evidence.RollbackTargetDigest != binding.ArtifactDigest || !reflect.DeepEqual(payload.Evidence.RollbackObservation, binding.Metric) {
				return nil, collaborationError(409, "stage_dependency_invalid")
			}
			deployment, intent, err := stageExecutionDeploymentFor(ctx, q, util.MustParseUUID(*binding.DeploymentID), access.Issue.ID, candidate.CandidateID)
			if err != nil {
				return nil, err
			}
			if intent.ApprovalDecisionID != chain[len(chain)-1].DecisionID || deployment.Observation.BeforeArtifactDigest == nil || *deployment.Observation.BeforeArtifactDigest != binding.ArtifactDigest {
				return nil, collaborationError(409, "stage_evidence_changed")
			}
			if _, err = stageCurrentRollbackMetric(ctx, tx, sources, access.Issue.ID, candidate.CandidateID, deployment, *binding.Metric, now); err != nil {
				return nil, err
			}
		}
	}
	return append(chain, d), nil
}

func stageExecutionDependency(ctx context.Context, tx pgx.Tx, issue pgtype.UUID, ids []string, step string) (db.CollaborationStageDecision, error) {
	if len(ids) != 1 {
		return db.CollaborationStageDecision{}, collaborationError(409, "stage_dependency_invalid")
	}
	d, err := db.New(tx).GetCollaborationStageDecision(ctx, db.GetCollaborationStageDecisionParams{IssueID: issue, DecisionID: util.MustParseUUID(ids[0])})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && d.Step != step {
		return d, collaborationError(409, "stage_dependency_invalid")
	}
	return d, err
}
func stageEarlier(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func (s *CollaborationService) decideStageExecution(ctx context.Context, t *collaborationWorkflowTx, p CollaborationStageReviewParams, state *collaborationStageState) (CollaborationWorkflowReceipt, error) {
	candidate, content, refusal, err := t.codeCandidate(ctx, p.Candidate)
	if refusal != nil || err != nil {
		return stageRefusal(refusal), err
	}
	refuse := func(err error) (CollaborationWorkflowReceipt, error) {
		if code := stageExecutionCode(err); code != "" {
			return t.result(code, 409), nil
		}
		return CollaborationWorkflowReceipt{}, err
	}
	hasEvidence := p.Evidence.CIDigest != nil || p.Evidence.TestReportID != nil || p.Evidence.Build != nil || p.Evidence.DeploymentID != nil || p.Evidence.RollbackTargetDigest != nil || p.Evidence.RollbackObservation != nil || len(p.DependsOn) > 0
	if !t.access.SourceMember && (p.Decision == "approve" || hasEvidence) {
		return t.result("stage_evidence_unavailable", 409), nil
	}
	signer, credential, err := freezeStageSigner(ctx, t.tx, t.access.Issue.WorkspaceID)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if p.Decision == "return" && !hasEvidence {
		return t.recordStageReview(ctx, p, state, candidate, signer, credential)
	}
	if p.Decision == "return" {
		if err = s.validateStageExecutionReturn(ctx, t, p, candidate, content); err != nil {
			return refuse(err)
		}
		if _, code, err := checkStageFinalAuthority(ctx, t.tx, t.access, state, nil); err != nil {
			return CollaborationWorkflowReceipt{}, err
		} else if code != "" {
			return t.result(code, 409), nil
		}
		return t.recordStageReview(ctx, p, state, candidate, signer, credential)
	}
	if err = t.lockCodeChild(ctx, content); err != nil {
		return refuse(err)
	}
	if err = lockStageCodeEvidence(ctx, t.tx); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	now, err := t.q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	observed, code, err := stageCodeCurrent(ctx, t.q, t.access, state, candidate, content, now.Time)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if p.Decision == "approve" {
		if len(collaboration.StageMissing(state.facts)) != 0 {
			return t.result("stage_needs_confirmation", 409), nil
		}
		if code != "" {
			return t.result(code, 409), nil
		}
		if code = stageCIRefusal(observed.CI); code != "" {
			return t.result(code, 409), nil
		}
	}
	if content.Baseline.Artifact == nil {
		return t.result("stage_evidence_unavailable", 409), nil
	}
	child, err := t.q.GetIssue(ctx, util.MustParseUUID(content.Baseline.ChildIssueID))
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	artifact, err := stageArtifactObservation(ctx, t.tx, child, util.MustParseUUID(content.Baseline.Artifact.ApprovalID), content.Baseline.PullRequest.HeadSHA)
	if stageArtifactUnavailable(err) {
		return t.result("stage_evidence_unavailable", 409), nil
	}
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if p.Decision == "approve" && (artifact.Verdict != "pass" || artifact.ArtifactDigest != content.Baseline.Artifact.ArtifactDigest) {
		return t.result("stage_evidence_changed", 409), nil
	}
	if p.Evidence.CIDigest != nil && (observed.CI.CIDigest == nil || *p.Evidence.CIDigest != *observed.CI.CIDigest) {
		return t.result("stage_evidence_changed", 409), nil
	}
	if p.Evidence.TestReportID != nil {
		_, code, err := stageQAReport(ctx, t.tx, t.access, candidate, *p.Evidence.TestReportID, true)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if code != "" && (code != "stage_test_report_unusable" || p.Decision == "approve") {
			return t.result(code, 409), nil
		}
	}
	if err = stageExecutionReady(ctx, t.q); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	var binding *stageExecutionBinding
	var dependencies []db.CollaborationStageDecision
	maximum := stageMaxExpiry(now.Time, observed.CI, signer.ExpiresAt, content)
	if p.Step == "release" {
		if p.Evidence.DeploymentID != nil || p.Evidence.RollbackTargetDigest != nil || p.Evidence.RollbackObservation != nil {
			return t.result("stage_dependency_invalid", 409), nil
		}
		if p.Evidence.Build == nil {
			return t.result("stage_evidence_unavailable", 409), nil
		}
		target, err := stageResolveExecutionTarget(ctx, t.tx, s.StageExecutionSources, t.access.Issue.WorkspaceID, p.Evidence.Build.TargetDigest)
		if err != nil {
			return refuse(err)
		}
		if target.Environment != p.Evidence.Build.Environment || p.Evidence.Build.ArtifactDigest != content.Baseline.Artifact.ArtifactDigest {
			return t.result("stage_evidence_changed", 409), nil
		}
		binding = &stageExecutionBinding{Target: target, TargetDigest: p.Evidence.Build.TargetDigest, ArtifactDigest: p.Evidence.Build.ArtifactDigest}
		if p.Decision == "approve" {
			d, err := stageExecutionDependency(ctx, t.tx, p.IssueID, p.DependsOn, "qa")
			if err != nil {
				return refuse(err)
			}
			dependencies, err = stageExecutionApprovalChain(ctx, t.tx, s.StageExecutionSources, t.access, state, candidate, content, d, observed, now.Time)
			if err != nil {
				return refuse(err)
			}
			qa, _ := stageExecutionPayload(d)
			if qa.Evidence.TestReportID == nil {
				return t.result("stage_test_report_unusable", 409), nil
			}
			report, code, err := stageQAReport(ctx, t.tx, t.access, candidate, *qa.Evidence.TestReportID, true)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			if code != "" {
				return t.result(code, 409), nil
			}
			if report.Report.Subject.ArtifactDigest == nil || *report.Report.Subject.ArtifactDigest != binding.ArtifactDigest || p.Evidence.TestReportID != nil && *p.Evidence.TestReportID != *qa.Evidence.TestReportID {
				return t.result("stage_evidence_changed", 409), nil
			}
		}
	} else {
		if p.Evidence.Build != nil || p.Evidence.DeploymentID == nil || p.Evidence.RollbackTargetDigest == nil || p.Evidence.RollbackObservation == nil {
			return t.result("stage_dependency_invalid", 409), nil
		}
		deployment, intent, err := stageExecutionDeploymentFor(ctx, t.q, util.MustParseUUID(*p.Evidence.DeploymentID), p.IssueID, candidate.CandidateID)
		if err != nil {
			return refuse(err)
		}
		target, err := stageResolveExecutionTarget(ctx, t.tx, s.StageExecutionSources, t.access.Issue.WorkspaceID, intent.TargetDigest)
		if err != nil {
			return refuse(err)
		}
		if deployment.Observation.BeforeArtifactDigest == nil || *deployment.Observation.BeforeArtifactDigest != *p.Evidence.RollbackTargetDigest {
			return t.result("stage_evidence_changed", 409), nil
		}
		if target.ConditionalVersion != deployment.Observation.AfterConditionalVersion || !reflect.DeepEqual(target.CurrentArtifactDigest, deployment.Observation.AfterArtifactDigest) {
			return t.result("stage_execution_target_changed", 409), nil
		}
		binding = &stageExecutionBinding{Target: target, TargetDigest: intent.TargetDigest, ArtifactDigest: *p.Evidence.RollbackTargetDigest, DeploymentID: p.Evidence.DeploymentID, Metric: p.Evidence.RollbackObservation}
		metric, err := stageCurrentRollbackMetric(ctx, t.tx, s.StageExecutionSources, p.IssueID, candidate.CandidateID, deployment, *p.Evidence.RollbackObservation, now.Time)
		if err != nil {
			return refuse(err)
		}
		maximum = stageEarlier(maximum, metric.ValidUntil)
		if p.Decision == "approve" {
			d, err := stageExecutionDependency(ctx, t.tx, p.IssueID, p.DependsOn, "release")
			if err != nil {
				return refuse(err)
			}
			if d.DecisionID != intent.ApprovalDecisionID {
				return t.result("stage_dependency_invalid", 409), nil
			}
			dependencies, err = stageExecutionApprovalChain(ctx, t.tx, s.StageExecutionSources, t.access, state, candidate, content, d, observed, now.Time)
			if err != nil {
				return refuse(err)
			}
		}
	}
	if p.Decision == "return" {
		for _, id := range p.DependsOn {
			d, err := t.q.GetCollaborationStageDecision(ctx, db.GetCollaborationStageDecisionParams{IssueID: p.IssueID, DecisionID: util.MustParseUUID(id)})
			if err != nil || d.CandidateID != candidate.CandidateID {
				return t.result("stage_dependency_invalid", 409), nil
			}
		}
	} else {
		for _, d := range dependencies {
			maximum = stageEarlier(maximum, d.ExpiresAt.Time)
		}
		if p.ExpiresAt == nil || !p.ExpiresAt.After(now.Time) || p.ExpiresAt.After(maximum) {
			return CollaborationWorkflowReceipt{}, collaborationError(400, "stage_expiry_exceeds_limit")
		}
	}
	final, code, err := checkStageFinalAuthority(ctx, t.tx, t.access, state, dependencies)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if code != "" {
		return t.result(code, 409), nil
	}
	if p.Decision == "approve" && (!final.Before(maximum) || observed.CI.ObservedAt == nil || !final.Before(observed.CI.ObservedAt.Add(stageCIHorizon))) {
		return t.result("stage_evidence_changed", 409), nil
	}
	if binding != nil {
		current, err := stageResolveExecutionTarget(ctx, t.tx, s.StageExecutionSources, t.access.Issue.WorkspaceID, binding.TargetDigest)
		if err != nil {
			return refuse(err)
		}
		if !stageTargetStateEqual(current, binding.Target) {
			return t.result("stage_execution_target_changed", 409), nil
		}
	}
	return t.recordStageReviewWithExecution(ctx, p, state, candidate, signer, credential, binding)
}

func (s *CollaborationService) stageAdvanceExecution(ctx context.Context, t *collaborationWorkflowTx, p CollaborationStageAdvanceParams, state *collaborationStageState, candidate db.CollaborationStageCandidate, content stageCodeContent, observed CollaborationStageObserved, candidateCode string, now time.Time) ([]db.CollaborationStageDecision, error) {
	if err := stageExecutionReady(ctx, t.q); err != nil {
		return nil, err
	}
	if t.actor != state.role.BusinessOwnerUserID && t.actor != state.role.ReleaseApproverUserID {
		return nil, collaborationError(403, "stage_role_required")
	}
	if len(collaboration.StageMissing(state.facts)) != 0 {
		return nil, collaborationError(409, "stage_needs_confirmation")
	}
	if candidateCode != "" {
		return nil, collaborationError(409, candidateCode)
	}
	decision, err := stageExecutionDependency(ctx, t.tx, p.IssueID, p.DecisionIDs, "release")
	if err != nil {
		return nil, err
	}
	chain, err := stageExecutionApprovalChain(ctx, t.tx, s.StageExecutionSources, t.access, state, candidate, content, decision, observed, now)
	if err != nil {
		return nil, err
	}
	intent, err := t.q.GetStageExecutionIntentByApproval(ctx, db.GetStageExecutionIntentByApprovalParams{IssueID: p.IssueID, ApprovalDecisionID: decision.DecisionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, collaborationError(409, "stage_execution_fact_missing")
	}
	if err != nil {
		return nil, err
	}
	fact, err := t.q.GetStageExecutionTerminal(ctx, intent.IntentID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && fact.Result != "succeeded" {
		return nil, collaborationError(409, "stage_execution_fact_missing")
	}
	if err != nil {
		return nil, err
	}
	deployment, _, err := stageExecutionDeploymentFor(ctx, t.q, fact.ObservationID, p.IssueID, candidate.CandidateID)
	if err != nil {
		return nil, err
	}
	target, err := stageResolveExecutionTarget(ctx, t.tx, s.StageExecutionSources, t.access.Issue.WorkspaceID, intent.TargetDigest)
	if err != nil {
		return nil, err
	}
	if target.ConditionalVersion != deployment.Observation.AfterConditionalVersion || !reflect.DeepEqual(target.CurrentArtifactDigest, deployment.Observation.AfterArtifactDigest) {
		return nil, collaborationError(409, "stage_execution_target_changed")
	}
	return chain, nil
}

// Optional return references need current read authority and exact association,
// not current passing evidence or a still-configured execution adapter.
func (s *CollaborationService) validateStageExecutionReturn(ctx context.Context, t *collaborationWorkflowTx, p CollaborationStageReviewParams, candidate db.CollaborationStageCandidate, content stageCodeContent) error {
	if err := stageArtifactSourceReadAs(ctx, t.tx, t.access, content, completionRequestCredential(ctx)); err != nil {
		if collaborationErrorCodeForReport(err, "not_found") {
			return collaborationError(409, "stage_evidence_unavailable")
		}
		return err
	}
	if p.Evidence.CIDigest != nil {
		now, err := t.q.ArtifactExecutionDatabaseTime(ctx)
		if err != nil {
			return err
		}
		root, _, err := stageReadState(ctx, t.q, t.access)
		if err != nil {
			return err
		}
		observed, _, err := stageCodeCurrent(ctx, t.q, t.access, root, candidate, content, now.Time)
		if err != nil {
			return err
		}
		belongs, err := stageCIEvidenceBelongs(ctx, t.q, p.IssueID, candidate.CandidateID, *p.Evidence.CIDigest, observed.CI)
		if err != nil {
			return err
		}
		if !belongs {
			return collaborationError(409, "stage_evidence_changed")
		}
	}
	if p.Evidence.TestReportID != nil {
		_, code, err := stageQAReport(ctx, t.tx, t.access, candidate, *p.Evidence.TestReportID, true)
		if err != nil {
			return err
		}
		if code != "" && code != "stage_test_report_unusable" {
			return collaborationError(409, code)
		}
	}
	if p.Evidence.Build != nil {
		build := p.Evidence.Build
		if content.Baseline.Artifact == nil || content.Baseline.Artifact.ArtifactDigest != build.ArtifactDigest {
			return collaborationError(409, "stage_evidence_changed")
		}
		target, err := stageResolveExecutionTarget(ctx, t.tx, s.StageExecutionSources, t.access.Issue.WorkspaceID, build.TargetDigest)
		known := err == nil && target.Environment == build.Environment
		if !known {
			decisions, err := t.q.ListCollaborationStageReviewDecisions(ctx, db.ListCollaborationStageReviewDecisionsParams{IssueID: p.IssueID, CandidateID: candidate.CandidateID, Limit: 50})
			if err != nil {
				return err
			}
			for _, d := range decisions {
				payload, err := stageExecutionPayload(d)
				if err != nil {
					return err
				}
				if payload.Evidence.Build != nil && reflect.DeepEqual(payload.Evidence.Build, build) {
					known = true
					break
				}
			}
		}
		if !known {
			return collaborationError(409, "stage_evidence_changed")
		}
	}
	if p.Evidence.DeploymentID != nil {
		fact, err := t.q.GetStageExecutionFact(ctx, util.MustParseUUID(*p.Evidence.DeploymentID))
		if err != nil {
			return collaborationError(409, "stage_evidence_changed")
		}
		intent, err := t.q.GetStageExecutionIntent(ctx, fact.IntentID)
		if err != nil {
			return err
		}
		if intent.IssueID != p.IssueID || intent.CandidateID != candidate.CandidateID || intent.Kind != "deployment" {
			return collaborationError(409, "stage_evidence_changed")
		}
		if p.Evidence.RollbackTargetDigest != nil {
			var observed StageExecutionObservation
			if json.Unmarshal(fact.Observation, &observed) != nil || observed.BeforeArtifactDigest == nil || *observed.BeforeArtifactDigest != *p.Evidence.RollbackTargetDigest {
				return collaborationError(409, "stage_evidence_changed")
			}
		}
	} else if p.Evidence.RollbackTargetDigest != nil {
		return collaborationError(409, "stage_dependency_invalid")
	}
	if p.Evidence.RollbackObservation != nil {
		row, err := t.q.GetStageRollbackObservation(ctx, util.MustParseUUID(p.Evidence.RollbackObservation.ObservationID))
		if err != nil {
			return collaborationError(409, "stage_rollback_observation_unavailable")
		}
		if row.IssueID != p.IssueID || row.CandidateID != candidate.CandidateID || row.ObservationDigest != p.Evidence.RollbackObservation.ObservationDigest || p.Evidence.DeploymentID != nil && util.UUIDToString(row.DeploymentID) != *p.Evidence.DeploymentID {
			return collaborationError(409, "stage_rollback_observation_changed")
		}
	}
	for _, id := range p.DependsOn {
		d, err := t.q.GetCollaborationStageDecision(ctx, db.GetCollaborationStageDecisionParams{IssueID: p.IssueID, DecisionID: util.MustParseUUID(id)})
		if err != nil || d.CandidateID != candidate.CandidateID {
			return collaborationError(409, "stage_dependency_invalid")
		}
	}
	return nil
}
