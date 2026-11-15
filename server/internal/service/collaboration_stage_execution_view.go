package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (s *CollaborationService) stageExecutionReviewState(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, state *collaborationStageState, candidate db.CollaborationStageCandidate, content stageCodeContent, d db.CollaborationStageDecision, observed CollaborationStageObserved, refusal string, now time.Time) (string, *CollaborationStageRevocationView, error) {
	status, revocation, err := stageReviewState(ctx, tx, access, state, candidate, content, d, observed, refusal, now)
	if err != nil || status != "evidence_changed" || refusal != "" || d.Decision != "approved" || (d.Step != "release" && d.Step != "rollback") {
		return status, revocation, err
	}
	chain, err := stageExecutionApprovalChain(ctx, tx, s.StageExecutionSources, access, state, candidate, content, d, observed, now)
	if err == nil {
		// Match consumption's current authority for every original issuer,
		// including artifact-read rights that the current viewer cannot replace.
		_, code, issuerErr := lockStageFinalSigners(ctx, tx, access, state, chain)
		if issuerErr != nil {
			return "", nil, issuerErr
		}
		if code != "" {
			return "evidence_changed", revocation, nil
		}
		return "valid", revocation, nil
	}
	if stageExecutionCode(err) != "" {
		return "evidence_changed", revocation, nil
	}
	return "", nil, err
}
func (s *CollaborationService) stageExecutionEvidence(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, candidate db.CollaborationStageCandidate, out *CollaborationStageCodeEvidenceView, now time.Time) error {
	q := db.New(tx)
	if err := stageExecutionReady(ctx, q); err != nil {
		return err
	}
	targets, err := stageExecutionTargets(ctx, tx, s.StageExecutionSources, access.Issue.WorkspaceID)
	if err != nil && stageExecutionCode(err) == "" {
		return err
	}
	for _, target := range targets {
		digest, _ := StageExecutionTargetDigest(target)
		out.ExecutionTargets = append(out.ExecutionTargets, CollaborationStageExecutionTargetView{Environment: target.Environment, TargetDigest: digest, SourceKind: "integration"})
	}
	intents, err := q.ListStageExecutionIntents(ctx, db.ListStageExecutionIntentsParams{IssueID: access.Issue.ID, CandidateID: candidate.CandidateID, Limit: 50})
	if err != nil {
		return err
	}
	for _, intent := range intents {
		inv, err := stageExecutionInvocation(intent)
		if err != nil {
			return err
		}
		receipt, err := stageExecutionReceiptFor(ctx, q, intent)
		if err != nil {
			return err
		}
		operation := CollaborationStageExecutionOperationView{IntentID: inv.IntentID, Kind: inv.Kind, ApprovalDecisionID: inv.ApprovalDecisionID, Environment: inv.Target.Environment, TargetDigest: inv.TargetDigest, ArtifactDigest: inv.ArtifactDigest, State: receipt.State, FactID: receipt.FactID, CreatedAt: intent.AdmittedAt.Time.UTC()}
		observation, err := q.GetStageExecutionTerminal(ctx, intent.IntentID)
		if errors.Is(err, pgx.ErrNoRows) {
			observation, err = q.GetStageExecutionLatestObservation(ctx, intent.IntentID)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && observation.ExternalOperationID.Valid {
			id := observation.ExternalOperationID.String
			operation.ExternalOperationID = &id
		}
		out.ExecutionOperations = append(out.ExecutionOperations, operation)
	}
	facts, err := q.ListStageExecutionFacts(ctx, db.ListStageExecutionFactsParams{IssueID: access.Issue.ID, CandidateID: candidate.CandidateID, Limit: 50})
	if err != nil {
		return err
	}
	for _, observation := range facts {
		intent, err := q.GetStageExecutionIntent(ctx, observation.IntentID)
		if err != nil {
			return err
		}
		inv, err := stageExecutionInvocation(intent)
		if err != nil {
			return err
		}
		var fact StageExecutionObservation
		if json.Unmarshal(observation.Observation, &fact) != nil || !observation.ExternalOperationID.Valid {
			return collaborationError(503, "stage_execution_unavailable")
		}
		out.ExecutionFacts = append(out.ExecutionFacts, CollaborationStageExecutionFactView{FactID: util.UUIDToString(observation.ObservationID), IntentID: inv.IntentID, TargetDigest: inv.TargetDigest, Kind: inv.Kind, SourceKind: "integration", Environment: inv.Target.Environment, OperationID: observation.ExternalOperationID.String, Result: observation.Result, ArtifactDigest: inv.ArtifactDigest, PreviousArtifactDigest: fact.BeforeArtifactDigest, ApprovalDecisionID: inv.ApprovalDecisionID, ObservedAt: fact.ObservedAt.UTC()})
	}

	metrics, err := q.ListStageRollbackObservations(ctx, db.ListStageRollbackObservationsParams{IssueID: access.Issue.ID, CandidateID: candidate.CandidateID, Limit: 50})
	if err != nil {
		return err
	}
	for _, row := range metrics {
		metric, err := decodeStageMetric(row)
		if err != nil {
			return err
		}
		deployment, _, err := stageExecutionDeploymentFor(ctx, q, row.DeploymentID, access.Issue.ID, candidate.CandidateID)
		if err != nil {
			return err
		}
		state, err := stageMetricState(ctx, tx, s.StageExecutionSources, row, deployment.Invocation.Target, now)
		if err != nil {
			return err
		}
		out.RollbackObservations = append(out.RollbackObservations, CollaborationStageRollbackObservationView{ObservationID: util.UUIDToString(row.ObservationID), Version: stageVersion(row.Version), ObservationDigest: row.ObservationDigest, DeploymentID: metric.DeploymentID, Environment: deployment.Invocation.Target.Environment, TargetDigest: metric.TargetDigest, ArtifactDigest: metric.ArtifactDigest, Condition: metric.Condition, State: state, WindowStart: metric.WindowStart.UTC(), WindowEnd: metric.WindowEnd.UTC(), ObservedAt: metric.ObservedAt.UTC(), ValidUntil: metric.ValidUntil.UTC(), Summary: metric.Summary})
	}
	return nil
}
func (s *CollaborationService) stageExecutionSignable(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, state *collaborationStageState, candidate db.CollaborationStageCandidate, content stageCodeContent, view CollaborationStageCodeEvidenceView, step string, now time.Time) (CollaborationStageSignable, error) {
	out := CollaborationStageSignable{Step: step, Blockers: []CollaborationStageBlocker{}}
	stage := step
	if stage == "rollback" {
		stage = "verification"
	}
	block := func(code string) {
		blockStage := stage
		if code == "stage_not_reached" {
			blockStage = state.task.Stage
		}
		out.Blockers = append(out.Blockers, CollaborationStageBlocker{Code: code, Stage: blockStage})
	}
	principal, _ := auth.AuthenticatedPrincipalFromContext(ctx)
	if state.role == nil {
		block("stage_roles_unconfigured")
		return out, nil
	}
	if principal.UserID != util.UUIDToString(state.role.ReleaseApproverUserID) || !access.Capabilities.ControlAI {
		block("stage_evidence_unavailable")
	}
	if state.task.Stage != stage {
		block("stage_not_reached")
	}
	if !state.policy.Enabled {
		block("stage_policy_changed")
	}
	if len(collaboration.StageMissing(state.facts)) != 0 {
		block("stage_scope_unconfirmed")
	}
	if view.Candidate.State != "current" {
		block("stage_candidate_stale")
	}
	if code := stageCIRefusal(view.Observed.CI); code != "" {
		block(code)
	}
	if content.Baseline.Artifact == nil {
		block("stage_evidence_unavailable")
	}
	if s.StageExecutionSources == nil || len(view.ExecutionTargets) == 0 {
		block("stage_execution_unavailable")
	}
	expiry := stageMaxExpiry(now, view.Observed.CI, nil, content)
	if principal.Kind == auth.CredentialJWT {
		if principal.ExpiresAt.IsZero() {
			block("stage_evidence_unavailable")
		} else {
			expiry = stageEarlier(expiry, principal.ExpiresAt)
		}
	} else if principal.Kind == auth.CredentialPAT {
		token, err := db.New(tx).GetPersonalAccessTokenByHash(ctx, principal.PATTokenHash)
		if err != nil {
			return out, err
		}
		if token.ExpiresAt.Valid {
			expiry = stageEarlier(expiry, token.ExpiresAt.Time)
		}
	} else {
		block("stage_evidence_unavailable")
	}
	dependencyStep := "qa"
	if step == "rollback" {
		dependencyStep = "release"
	}
	found := false
	for _, d := range view.Decisions {
		if d.Step == dependencyStep && d.Decision == "approved" && d.State == "valid" && d.ExpiresAt != nil {
			found = true
			expiry = stageEarlier(expiry, *d.ExpiresAt)
			break
		}
	}
	if !found {
		if step == "release" {
			block("stage_qa_missing")
		} else {
			block("stage_release_approval_missing")
		}
	}
	if step == "rollback" {
		matched := false
		for _, metric := range view.RollbackObservations {
			if metric.State != "current" || metric.Condition != "anomaly" {
				continue
			}
			for _, fact := range view.ExecutionFacts {
				if fact.FactID == metric.DeploymentID && fact.Kind == "deployment" && fact.Result == "succeeded" && fact.PreviousArtifactDigest != nil {
					for _, decision := range view.Decisions {
						if decision.DecisionID == fact.ApprovalDecisionID && decision.Step == "release" && decision.Decision == "approved" && decision.State == "valid" {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
					expiry = stageEarlier(expiry, metric.ValidUntil)
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			block("stage_rollback_observation_unavailable")
		}
	}
	if len(out.Blockers) == 0 {
		if now.Before(expiry) {
			out.Allowed = true
			out.MaxExpiresAt = &expiry
		} else {
			block("stage_decision_expired")
		}
	}
	return out, nil
}
