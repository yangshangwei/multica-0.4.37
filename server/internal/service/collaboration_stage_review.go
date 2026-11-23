package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func (t *collaborationWorkflowTx) stageRoleRevision(state *collaborationStageState, expected int64) *CollaborationWorkflowReceipt {
	if state.role == nil {
		r := t.result("stage_roles_unconfigured", 409)
		return &r
	}
	if state.task.RoleRevision != expected {
		r := t.result("stage_roles_changed", 409)
		return &r
	}
	return nil
}

func (t *collaborationWorkflowTx) codeCandidate(ctx context.Context, ref CollaborationStageCandidateRef) (db.CollaborationStageCandidate, stageCodeContent, *CollaborationWorkflowReceipt, error) {
	c, err := t.q.GetCollaborationStageCodeHead(ctx, t.access.Issue.ID)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !ref.matches(c) {
		r := t.result("stage_candidate_changed", 409)
		return c, stageCodeContent{}, &r, nil
	}
	if err != nil {
		return c, stageCodeContent{}, nil, err
	}
	content, err := decodeStageCode(c)
	return c, content, nil, err
}

func (t *collaborationWorkflowTx) lockCodeChild(ctx context.Context, content stageCodeContent) error {
	rows, err := t.q.LockCollaborationStageScopeChildren(ctx, []pgtype.UUID{util.MustParseUUID(content.Baseline.ChildIssueID)})
	if err != nil {
		return err
	}
	if len(rows) != 1 || rows[0].WorkspaceID != t.access.Issue.WorkspaceID || rows[0].Collaboration {
		return collaborationError(409, "stage_evidence_unavailable")
	}
	return nil
}

func stageArtifactUnavailable(err error) bool {
	var artifact ArtifactError
	return errors.Is(err, pgx.ErrNoRows) || errors.As(err, &artifact)
}

func stageReviewStep(step string) bool {
	return step == "code_review" || step == "qa" || step == "release" || step == "rollback"
}
func stageTarget(stage string) bool {
	return stage == "development" || stage == "code_review" || stage == "qa" || stage == "release" || stage == "verification"
}

func (s *CollaborationService) CreateStageCodeCandidate(ctx context.Context, p CollaborationStageCodeParams) (CollaborationWorkflowReceipt, error) {
	if err := stageHuman(ctx); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if p.ExpectedPolicyRevision < 1 || p.ExpectedRoleRevision < 1 || !p.ChildIssueID.Valid || p.ChildIssueID.Bytes == [16]byte{} || !p.PullRequest.valid() ||
		(p.Supersedes != nil && !p.Supersedes.valid()) || (p.Artifact != nil && (!p.Artifact.ApprovalID.Valid || p.Artifact.ApprovalID.Bytes == [16]byte{})) {
		return stageInvalid()
	}
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, stageKindCreateCode, p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if t.stageClosed() {
			return t.result("issue_closed", 409), nil
		}
		state, refusal, err := t.loadStage(ctx, p.ExpectedPolicyRevision)
		if refusal != nil || err != nil {
			return stageRefusal(refusal), err
		}
		if refusal = t.stageRoleRevision(state, p.ExpectedRoleRevision); refusal != nil {
			return *refusal, nil
		}
		if len(collaboration.StageMissing(state.facts)) != 0 {
			return t.result("stage_needs_confirmation", 409), nil
		}
		if !t.access.SourceMember {
			return t.result("stage_pull_request_unavailable", 409), nil
		}
		if err = CheckArtifactCredentialInTx(ctx, t.tx, completionRequestCredential(ctx)); err != nil {
			return CollaborationWorkflowReceipt{}, stageCredentialFailure(err)
		}
		prior, err := t.q.GetCollaborationStageCodeHead(ctx, p.IssueID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return CollaborationWorkflowReceipt{}, err
		}
		found := err == nil
		if (found && (p.Supersedes == nil || !p.Supersedes.matches(prior))) || (!found && p.Supersedes != nil) {
			return t.result("stage_candidate_changed", 409), nil
		}
		rows, err := t.q.LockCollaborationStageScopeChildren(ctx, []pgtype.UUID{p.ChildIssueID})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if len(rows) != 1 || rows[0].WorkspaceID != t.access.Issue.WorkspaceID || rows[0].Collaboration {
			return t.result("stage_child_unconfirmed", 409), nil
		}
		mapping, err := t.q.GetCollaborationStageChild(ctx, p.ChildIssueID)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && (mapping.RootIssueID != p.IssueID || mapping.LifecycleEpoch != t.access.LifecycleEpoch) {
			return t.result("stage_child_unconfirmed", 409), nil
		}
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if err = lockStageCodeEvidence(ctx, t.tx); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		now, err := t.q.ArtifactExecutionDatabaseTime(ctx)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		pr, source, _, err := loadStageCodeSource(ctx, t.q, t.access.Issue.WorkspaceID, p.ChildIssueID, p.PullRequest, now.Time)
		if errors.Is(err, pgx.ErrNoRows) {
			return t.result("stage_pull_request_unavailable", 409), nil
		}
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if pr.HeadSHA != p.PullRequest.HeadSHA {
			return t.result("stage_head_changed", 409), nil
		}
		content := stageCodeContent{ImplementerUserID: util.UUIDToString(state.role.ImplementerUserID), RoleRevision: state.task.RoleRevision, Source: source,
			Baseline: CollaborationStageCodeBaseline{RequirementDigest: collaboration.StageChildContentDigest(t.access.Issue.Title, t.access.Issue.Description.String), ChildIssueID: util.UUIDToString(p.ChildIssueID), ChildSemanticVersion: stageVersion(mapping.SemanticVersion), PullRequest: pr}}
		if p.Artifact != nil {
			child, err := t.q.GetIssue(ctx, p.ChildIssueID)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			observation, err := stageArtifactObservation(ctx, t.tx, child, p.Artifact.ApprovalID, pr.HeadSHA)
			if stageArtifactUnavailable(err) {
				return t.result("stage_artifact_unavailable", 409), nil
			}
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			if observation.Verdict != "pass" {
				return t.result("stage_artifact_unavailable", 409), nil
			}
			content.Baseline.Artifact = &CollaborationStageArtifactBaseline{ApprovalID: util.UUIDToString(p.Artifact.ApprovalID), ArtifactDigest: observation.ArtifactDigest}
			content.Source.ArtifactObservation = &observation
		}
		raw, digest, err := stageJSONDigest(content)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		version, parent := int64(1), pgtype.UUID{}
		if found {
			version, parent = prior.Version+1, prior.CandidateID
		}
		candidate, err := t.q.CreateCollaborationStageCodeCandidate(ctx, db.CreateCollaborationStageCodeCandidateParams{CandidateID: dbid.NewV7(), IssueID: p.IssueID, WorkspaceID: t.access.Issue.WorkspaceID,
			Version: version, ParentCandidateID: parent, Content: raw, ContentDigest: digest, Source: []byte(`{}`), LifecycleEpoch: t.access.LifecycleEpoch, SubmittedBy: t.actor, OperationID: t.operation.ID})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.stageConfirmation = &CollaborationStageConfirmation{Action: "create_code_candidate", IssueID: util.UUIDToString(p.IssueID), CandidateID: util.UUIDToString(candidate.CandidateID), Version: stageVersion(version), ContentDigest: digest}
		if parent.Valid {
			t.stageConfirmation.ParentCandidateID = util.UUIDToString(parent)
		}
		return t.finishWith(ctx, "stage_code_candidate_created", pgtype.UUID{}, nil, map[string]any{"stage": map[string]any{"candidate_id": util.UUIDToString(candidate.CandidateID), "version": stageVersion(version), "content_digest": digest, "parent_candidate_id": stageText(candidate.ParentCandidateID)}})
	})
}

type stageReviewPayload struct {
	Evidence  CollaborationStageReviewEvidence `json:"evidence"`
	DependsOn []string                         `json:"depends_on"`
	Execution *stageExecutionBinding           `json:"execution,omitempty"`
}

func stageReviewEvidenceValid(e CollaborationStageReviewEvidence) bool {
	return (e.CIDigest == nil || stageDigest(*e.CIDigest)) && (e.TestReportID == nil || stageUUID(*e.TestReportID)) &&
		(e.DeploymentID == nil || stageUUID(*e.DeploymentID)) && (e.RollbackTargetDigest == nil || stageDigest(*e.RollbackTargetDigest)) &&
		(e.RollbackObservation == nil || stageUUID(e.RollbackObservation.ObservationID) && stageDigest(e.RollbackObservation.ObservationDigest)) &&
		(e.Build == nil || (e.Build.TargetDigest == "" || stageDigest(e.Build.TargetDigest)) && (stageDigest(e.Build.ArtifactDigest) && len(e.Build.Environment) > 0 && len(e.Build.Environment) <= 128 && !strings.ContainsRune(e.Build.Environment, 0)))
}

func stageDistinctIDs(ids []string) bool {
	if len(ids) > 32 {
		return false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !stageUUID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func (s *CollaborationService) DecideStage(ctx context.Context, p CollaborationStageReviewParams) (CollaborationWorkflowReceipt, error) {
	if err := stageHuman(ctx); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if p.ExpectedPolicyRevision < 1 || p.ExpectedRoleRevision < 1 || p.ExpectedStageRevision < 1 || !stageReviewStep(p.Step) || !p.Candidate.valid() || !stageReason(p.Reason) || !stageDistinctIDs(p.DependsOn) || !stageReviewEvidenceValid(p.Evidence) ||
		(p.Decision != "approve" && p.Decision != "return") || (p.Decision == "approve" && p.ExpiresAt == nil) || (p.Decision == "return" && (p.ExpiresAt != nil || strings.TrimSpace(p.Reason) == "")) {
		return stageInvalid()
	}
	if (p.Step == "code_review" || p.Step == "qa") && (p.Evidence.RollbackObservation != nil || p.Evidence.Build != nil && p.Evidence.Build.TargetDigest != "") || p.Step != "rollback" && p.Evidence.RollbackObservation != nil {
		return stageInvalid()
	}
	if p.Decision == "approve" && (p.Step == "release" && (p.Evidence.Build == nil || !stageDigest(p.Evidence.Build.TargetDigest)) || p.Step == "rollback" && (p.Evidence.DeploymentID == nil || p.Evidence.RollbackTargetDigest == nil || p.Evidence.RollbackObservation == nil)) {
		return stageInvalid()
	}
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, stageKindDecide, p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if t.stageClosed() {
			return t.result("issue_closed", 409), nil
		}
		state, refusal, err := t.loadStage(ctx, p.ExpectedPolicyRevision)
		if refusal != nil || err != nil {
			return stageRefusal(refusal), err
		}
		if refusal = t.stageRoleRevision(state, p.ExpectedRoleRevision); refusal != nil {
			return *refusal, nil
		}
		holder, _ := stageRoleHolder(state.role, p.Step)
		if holder != t.actor {
			return t.result("stage_role_required", 403), nil
		}
		expectedStage := p.Step
		if p.Step == "rollback" {
			expectedStage = "verification"
		}
		if state.task.StageRevision != p.ExpectedStageRevision || state.task.Stage != expectedStage {
			return t.result("stage_stage_changed", 409), nil
		}
		if p.Step == "qa" {
			return t.decideStageQA(ctx, p, state)
		}
		if p.Step == "release" || p.Step == "rollback" {
			return s.decideStageExecution(ctx, t, p, state)
		}
		candidate, content, refusal, err := t.codeCandidate(ctx, p.Candidate)
		if refusal != nil || err != nil {
			return stageRefusal(refusal), err
		}
		if content.ImplementerUserID == util.UUIDToString(t.actor) {
			return t.result("stage_self_review_forbidden", 409), nil
		}
		if !t.access.SourceMember && p.Decision == "approve" {
			return t.result("stage_evidence_unavailable", 409), nil
		}
		if t.access.SourceMember {
			if err = t.lockCodeChild(ctx, content); err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
		}
		if err = lockStageCodeEvidence(ctx, t.tx); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		signer, credential, err := freezeStageSigner(ctx, t.tx, t.access.Issue.WorkspaceID)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		now, err := t.q.ArtifactExecutionDatabaseTime(ctx)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		observed, code := CollaborationStageObserved{}, "stage_evidence_unavailable"
		if t.access.SourceMember {
			observed, code, err = stageCodeCurrent(ctx, t.q, t.access, state, candidate, content, now.Time)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
		}
		if p.Decision == "approve" {
			if signer.SourceMemberID == "" {
				return t.result("stage_evidence_unavailable", 409), nil
			}
			if len(collaboration.StageMissing(state.facts)) != 0 {
				return t.result("stage_needs_confirmation", 409), nil
			}
			if code != "" {
				return t.result(code, 409), nil
			}
			if code = stageCIRefusal(observed.CI); code != "" {
				return t.result(code, 409), nil
			}
			if p.Evidence.CIDigest == nil || observed.CI.CIDigest == nil || *p.Evidence.CIDigest != *observed.CI.CIDigest {
				return t.result("stage_evidence_changed", 409), nil
			}
			maximum := stageMaxExpiry(now.Time, observed.CI, signer.ExpiresAt, content)
			if !p.ExpiresAt.After(now.Time) || p.ExpiresAt.After(maximum) {
				return CollaborationWorkflowReceipt{}, collaborationError(http.StatusBadRequest, "stage_expiry_exceeds_limit")
			}
			if a := content.Baseline.Artifact; a != nil {
				child, err := t.q.GetIssue(ctx, util.MustParseUUID(content.Baseline.ChildIssueID))
				if err != nil {
					return CollaborationWorkflowReceipt{}, err
				}
				observation, err := stageArtifactObservation(ctx, t.tx, child, util.MustParseUUID(a.ApprovalID), content.Baseline.PullRequest.HeadSHA)
				if err != nil {
					return CollaborationWorkflowReceipt{}, err
				}
				if observation.Verdict != "pass" || observation.ArtifactDigest != a.ArtifactDigest {
					return t.result("stage_evidence_changed", 409), nil
				}
			}
		} else if p.Evidence.CIDigest != nil {
			if !t.access.SourceMember {
				return t.result("stage_evidence_unavailable", 409), nil
			}
			belongs, err := stageCIEvidenceBelongs(ctx, t.q, p.IssueID, candidate.CandidateID, *p.Evidence.CIDigest, observed.CI)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			if !belongs {
				return t.result("stage_evidence_changed", 409), nil
			}
		}
		// Code review has no test/build/deployment dependency. Never accept a
		// foreign future-step identifier merely because this is a return.
		if p.Evidence.TestReportID != nil || p.Evidence.Build != nil || p.Evidence.DeploymentID != nil || p.Evidence.RollbackTargetDigest != nil || (p.Decision == "approve" && len(p.DependsOn) != 0) {
			return t.result("stage_dependency_invalid", 409), nil
		}
		for _, id := range p.DependsOn {
			if !t.access.SourceMember {
				return t.result("stage_evidence_unavailable", 409), nil
			}
			d, err := t.q.GetCollaborationStageDecision(ctx, db.GetCollaborationStageDecisionParams{IssueID: p.IssueID, DecisionID: util.MustParseUUID(id)})
			if errors.Is(err, pgx.ErrNoRows) || err == nil && d.CandidateID != candidate.CandidateID {
				return t.result("stage_dependency_invalid", 409), nil
			}
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
		}
		if err = CheckArtifactCredentialInTx(ctx, t.tx, completionRequestCredential(ctx)); err != nil {
			return CollaborationWorkflowReceipt{}, stageCredentialFailure(err)
		}
		return t.recordStageReview(ctx, p, state, candidate, signer, credential)
	})
}

func (s *CollaborationService) RevokeStageDecision(ctx context.Context, p CollaborationStageRevocationParams) (CollaborationWorkflowReceipt, error) {
	if err := stageHuman(ctx); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if p.ExpectedPolicyRevision < 1 || p.ExpectedRoleRevision < 1 || !p.DecisionID.Valid || p.DecisionID.Bytes == [16]byte{} || !stageReason(p.Reason) || strings.TrimSpace(p.Reason) == "" {
		return stageInvalid()
	}
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, stageKindRevoke, p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		state, refusal, err := t.loadStage(ctx, p.ExpectedPolicyRevision)
		if refusal != nil || err != nil {
			return stageRefusal(refusal), err
		}
		if refusal = t.stageRoleRevision(state, p.ExpectedRoleRevision); refusal != nil {
			return *refusal, nil
		}
		d, err := t.q.GetCollaborationStageDecision(ctx, db.GetCollaborationStageDecisionParams{DecisionID: p.DecisionID, IssueID: p.IssueID})
		if errors.Is(err, pgx.ErrNoRows) {
			return t.result("stage_decision_not_found", 404), nil
		}
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if d.Step == "scope" || d.Step == "proposal" {
			return t.result("stage_decision_not_revocable", 409), nil
		}
		holder, _ := stageRoleHolder(state.role, d.Step)
		admin := t.sourceMemberLocked && (t.sourceMember.Role == "owner" || t.sourceMember.Role == "admin")
		if !(d.ActorUserID == t.actor && holder == t.actor) && !admin {
			return t.result("stage_role_required", 403), nil
		}
		if err = CheckArtifactCredentialInTx(ctx, t.tx, completionRequestCredential(ctx)); err != nil {
			return CollaborationWorkflowReceipt{}, stageCredentialFailure(err)
		}
		_, err = t.q.GetCollaborationStageRevocation(ctx, db.GetCollaborationStageRevocationParams{IssueID: p.IssueID, DecisionID: p.DecisionID})
		if err == nil {
			return t.result("stage_decision_not_revocable", 409), nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return CollaborationWorkflowReceipt{}, err
		}
		if err = stageExecutionReady(ctx, t.q); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if _, e := t.q.GetStageExecutionIntentByApproval(ctx, db.GetStageExecutionIntentByApprovalParams{IssueID: p.IssueID, ApprovalDecisionID: p.DecisionID}); e == nil {
			return t.result("stage_decision_not_revocable", 409), nil
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return CollaborationWorkflowReceipt{}, e
		}
		r, err := t.q.CreateCollaborationStageRevocation(ctx, db.CreateCollaborationStageRevocationParams{RevocationID: dbid.NewV7(), DecisionID: p.DecisionID, IssueID: p.IssueID,
			WorkspaceID: t.access.Issue.WorkspaceID, ActorUserID: t.actor, OperationID: t.operation.ID, Reason: strings.TrimSpace(p.Reason)})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.stageConfirmation = &CollaborationStageConfirmation{Action: "revoke_decision", IssueID: util.UUIDToString(p.IssueID), DecisionID: util.UUIDToString(p.DecisionID), RevocationID: util.UUIDToString(r.RevocationID)}
		return t.finishWith(ctx, "stage_decision_revoked", pgtype.UUID{}, nil, map[string]any{"stage": map[string]any{"decision_id": util.UUIDToString(p.DecisionID), "revocation_id": util.UUIDToString(r.RevocationID)}})
	})
}

// A9 consumes standing decisions under the same workflow, source and signer
// locks. The database clock is re-read immediately before changing the stage.
func (s *CollaborationService) AdvanceStage(ctx context.Context, p CollaborationStageAdvanceParams) (CollaborationWorkflowReceipt, error) {
	if err := stageHuman(ctx); err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if p.ExpectedPolicyRevision < 1 || p.ExpectedRoleRevision < 1 || p.ExpectedStageRevision < 1 || !stageTarget(p.TargetStage) || !p.Candidate.valid() || !stageDistinctIDs(p.DecisionIDs) {
		return stageInvalid()
	}
	return s.mutateCollaborationWorkflow(ctx, p.CollaborationWorkflowRequest, pgtype.UUID{}, stageKindAdvance, p, func(t *collaborationWorkflowTx) (CollaborationWorkflowReceipt, error) {
		if t.stageClosed() {
			return t.result("issue_closed", 409), nil
		}
		state, refusal, err := t.loadStage(ctx, p.ExpectedPolicyRevision)
		if refusal != nil || err != nil {
			return stageRefusal(refusal), err
		}
		if refusal = t.stageRoleRevision(state, p.ExpectedRoleRevision); refusal != nil {
			return *refusal, nil
		}
		if state.task.StageRevision != p.ExpectedStageRevision {
			return t.result("stage_stage_changed", 409), nil
		}
		candidate, content, refusal, err := t.codeCandidate(ctx, p.Candidate)
		if refusal != nil || err != nil {
			return stageRefusal(refusal), err
		}
		if !t.access.SourceMember && p.TargetStage != "development" {
			return t.result("stage_evidence_unavailable", 409), nil
		}
		if t.access.SourceMember {
			if err = t.lockCodeChild(ctx, content); err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
		}
		if err = lockStageCodeEvidence(ctx, t.tx); err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if err = CheckArtifactCredentialInTx(ctx, t.tx, completionRequestCredential(ctx)); err != nil {
			return CollaborationWorkflowReceipt{}, stageCredentialFailure(err)
		}
		now, err := t.q.ArtifactExecutionDatabaseTime(ctx)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		observed, code, err := stageCodeCurrent(ctx, t.q, t.access, state, candidate, content, now.Time)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		owner := t.actor == state.role.BusinessOwnerUserID
		var dependencies []db.CollaborationStageDecision
		switch {
		case p.TargetStage == "verification" && state.task.Stage == "release":
			var executionErr error
			dependencies, executionErr = s.stageAdvanceExecution(ctx, t, p, state, candidate, content, observed, code, now.Time)
			if executionErr != nil {
				if refusal := stageExecutionCode(executionErr); refusal != "" {
					return t.result(refusal, 409), nil
				}
				return CollaborationWorkflowReceipt{}, executionErr
			}
		case p.TargetStage == "code_review" && state.task.Stage == "development":
			if !owner {
				return t.result("stage_role_required", 403), nil
			}
			if len(p.DecisionIDs) != 0 {
				return t.result("stage_dependency_invalid", 409), nil
			}
			if len(collaboration.StageMissing(state.facts)) != 0 {
				return t.result("stage_needs_confirmation", 409), nil
			}
			if code != "" {
				return t.result(code, 409), nil
			}
			if code = stageCIRefusal(observed.CI); code != "" {
				return t.result(code, 409), nil
			}
		case p.TargetStage == "qa" && state.task.Stage == "code_review", p.TargetStage == "release" && state.task.Stage == "qa", p.TargetStage == "development" && (state.task.Stage == "code_review" || state.task.Stage == "qa" || state.task.Stage == "release" || state.task.Stage == "verification"):
			if len(p.DecisionIDs) != 1 {
				return t.result("stage_dependency_invalid", 409), nil
			}
			d, err := t.q.GetCollaborationStageDecision(ctx, db.GetCollaborationStageDecisionParams{IssueID: p.IssueID, DecisionID: util.MustParseUUID(p.DecisionIDs[0])})
			if errors.Is(err, pgx.ErrNoRows) {
				return t.result("stage_dependency_invalid", 409), nil
			}
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			want := "approved"
			if p.TargetStage == "development" {
				want = "returned"
			}
			expectedStep := state.task.Stage
			if expectedStep == "verification" {
				expectedStep = "rollback"
			}
			if d.CandidateID != candidate.CandidateID || d.Decision != want || d.Step != expectedStep {
				return t.result("stage_dependency_invalid", 409), nil
			}
			holder, _ := stageRoleHolder(state.role, d.Step)
			if !owner && t.actor != holder {
				return t.result("stage_role_required", 403), nil
			}
			valid, _, err := stageReviewState(ctx, t.tx, t.access, state, candidate, content, d, observed, code, now.Time)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			if valid != "valid" {
				return t.result(stageDecisionRefusal(valid), 409), nil
			}
			current, signer, err := stageSignerCurrent(ctx, t.q, state.role, d, now.Time)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			if !current {
				return t.result("stage_signer_changed", 409), nil
			}
			if err = CheckArtifactCredentialInTx(ctx, t.tx, signer.credential(util.UUIDToString(d.ActorUserID))); err != nil {
				var denied *CollaborationError
				if errors.As(stageCredentialFailure(err), &denied) && denied.Code == "forbidden" {
					return t.result("stage_signer_changed", 409), nil
				}
				return CollaborationWorkflowReceipt{}, err
			}
			if want == "approved" {
				members, err := t.q.LockArtifactMembers(ctx, db.LockArtifactMembersParams{WorkspaceID: t.access.Issue.WorkspaceID, UserIds: []pgtype.UUID{d.ActorUserID}})
				if err != nil {
					return CollaborationWorkflowReceipt{}, err
				}
				if len(members) != 1 || util.UUIDToString(members[0].ID) != signer.SourceMemberID {
					return t.result("stage_signer_changed", 409), nil
				}
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
			dependencies = append(dependencies, d)
			if d.Step == "qa" && want == "approved" {
				var payload stageReviewPayload
				if json.Unmarshal(d.Payload, &payload) != nil || payload.Evidence.TestReportID == nil {
					return t.result("stage_dependency_invalid", 409), nil
				}
				if _, code, err := stageQAReport(ctx, t.tx, t.access, candidate, *payload.Evidence.TestReportID, true); err != nil {
					return CollaborationWorkflowReceipt{}, err
				} else if code != "" {
					return t.result(code, 409), nil
				}
				prior, err := stageQAFinalDependencies(ctx, t.q, d)
				if err != nil {
					return CollaborationWorkflowReceipt{}, err
				}
				dependencies = append(dependencies, prior...)
			}
		default:
			return t.result("stage_transition_invalid", 409), nil
		}
		if p.TargetStage != "development" {
			artifactCode, err := checkStageArtifactCurrent(ctx, t.tx, content)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
			if artifactCode != "" {
				return t.result(artifactCode, 409), nil
			}
		}
		finalNow, finalCode, err := checkStageFinalAuthority(ctx, t.tx, t.access, state, dependencies)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if finalCode == "forbidden" {
			return CollaborationWorkflowReceipt{}, collaborationError(403, finalCode)
		}
		if finalCode != "" {
			return t.result(finalCode, 409), nil
		}
		if p.TargetStage != "development" && (observed.CI.ObservedAt == nil || !finalNow.Before(observed.CI.ObservedAt.Add(stageCIHorizon))) {
			return t.result("stage_ci_stale", 409), nil
		}
		next, err := t.q.AdvanceCollaborationStage(ctx, db.AdvanceCollaborationStageParams{IssueID: p.IssueID, Stage: p.TargetStage, StageRevision: p.ExpectedStageRevision})
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		t.stageConfirmation = &CollaborationStageConfirmation{Action: "advance_stage", IssueID: util.UUIDToString(p.IssueID), Stage: p.TargetStage, StageRevision: stageVersion(next.StageRevision)}
		return t.finishWith(ctx, "stage_advanced", pgtype.UUID{}, nil, map[string]any{"stage": map[string]any{"current": p.TargetStage, "stage_revision": stageVersion(next.StageRevision), "candidate_id": util.UUIDToString(candidate.CandidateID)}})
	})
}

func stageDecisionRefusal(state string) string {
	switch state {
	case "expired":
		return "stage_decision_expired"
	case "revoked":
		return "stage_decision_revoked"
	case "signer_changed":
		return "stage_signer_changed"
	case "roles_changed":
		return "stage_roles_changed"
	case "policy_changed":
		return "stage_policy_changed"
	default:
		return "stage_evidence_changed"
	}
}

func (t *collaborationWorkflowTx) recordStageReview(ctx context.Context, p CollaborationStageReviewParams, state *collaborationStageState, candidate db.CollaborationStageCandidate, signer stageSignerCredential, credential []byte) (CollaborationWorkflowReceipt, error) {
	return t.recordStageReviewWithExecution(ctx, p, state, candidate, signer, credential, nil)
}

func (t *collaborationWorkflowTx) recordStageReviewWithExecution(ctx context.Context, p CollaborationStageReviewParams, state *collaborationStageState, candidate db.CollaborationStageCandidate, signer stageSignerCredential, credential []byte, execution *stageExecutionBinding) (CollaborationWorkflowReceipt, error) {
	_, role := stageRoleHolder(state.role, p.Step)
	decision := "approved"
	if p.Decision == "return" {
		decision = "returned"
	}
	if p.ExpiresAt != nil {
		finalNow, err := t.q.ArtifactExecutionDatabaseTime(ctx)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if !finalNow.Time.Before(*p.ExpiresAt) {
			return CollaborationWorkflowReceipt{}, collaborationError(400, "stage_expiry_exceeds_limit")
		}
	}
	dependencies := append([]string{}, p.DependsOn...)
	payload, err := json.Marshal(stageReviewPayload{Evidence: p.Evidence, DependsOn: dependencies, Execution: execution})
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	expires := pgtype.Timestamptz{}
	if p.ExpiresAt != nil {
		expires = pgtype.Timestamptz{Time: p.ExpiresAt.UTC(), Valid: true}
	}
	d, err := t.q.CreateCollaborationStageReviewDecision(ctx, db.CreateCollaborationStageReviewDecisionParams{DecisionID: dbid.NewV7(), IssueID: p.IssueID, WorkspaceID: t.access.Issue.WorkspaceID,
		Step: p.Step, Decision: decision, CandidateID: candidate.CandidateID, CandidateVersion: pgtype.Int8{Int64: candidate.Version, Valid: true}, ContentDigest: pgtype.Text{String: candidate.ContentDigest, Valid: true},
		Role: role, RoleRevision: state.task.RoleRevision, PolicyRevision: state.policy.Revision, LifecycleEpoch: t.access.LifecycleEpoch, ActorUserID: t.actor, ParticipantEpoch: t.participant.Epoch,
		AccountEpoch: t.account.Epoch, CredentialKind: string(signer.Kind), SignerCredential: credential, ExpiresAt: expires, Reason: strings.TrimSpace(p.Reason), Payload: payload, OperationID: t.operation.ID})
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	t.stageConfirmation = &CollaborationStageConfirmation{Action: "decide_stage", IssueID: util.UUIDToString(p.IssueID), Step: p.Step, Decision: decision, CandidateID: util.UUIDToString(candidate.CandidateID), Version: stageVersion(candidate.Version),
		ContentDigest: candidate.ContentDigest, DecisionID: util.UUIDToString(d.DecisionID), RoleRevision: stageVersion(state.task.RoleRevision), PolicyRevision: stageVersion(state.policy.Revision), ExpiresAt: p.ExpiresAt}
	return t.finishWith(ctx, "stage_decision_recorded", pgtype.UUID{}, nil, map[string]any{"stage": map[string]any{"candidate_id": util.UUIDToString(candidate.CandidateID), "decision_id": util.UUIDToString(d.DecisionID), "step": p.Step}})
}

func stageCIEvidenceBelongs(ctx context.Context, q *db.Queries, issue, candidate pgtype.UUID, digest string, observed CollaborationStageObservedCI) (bool, error) {
	if observed.CIDigest != nil && *observed.CIDigest == digest {
		return true, nil
	}
	decisions, err := q.ListCollaborationStageReviewDecisions(ctx, db.ListCollaborationStageReviewDecisionsParams{IssueID: issue, CandidateID: candidate, Limit: 50})
	if err != nil {
		return false, err
	}
	for _, decision := range decisions {
		var payload stageReviewPayload
		if json.Unmarshal(decision.Payload, &payload) != nil {
			return false, errors.New("invalid retained stage decision")
		}
		if payload.Evidence.CIDigest != nil && *payload.Evidence.CIDigest == digest {
			return true, nil
		}
	}
	return false, nil
}
