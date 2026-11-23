package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func stageReviewState(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, state *collaborationStageState, candidate db.CollaborationStageCandidate, content stageCodeContent, d db.CollaborationStageDecision, observed CollaborationStageObserved, candidateRefusal string, now time.Time) (string, *CollaborationStageRevocationView, error) {
	q := db.New(tx)
	revocation, err := q.GetCollaborationStageRevocation(ctx, db.GetCollaborationStageRevocationParams{IssueID: d.IssueID, DecisionID: d.DecisionID})
	if err == nil {
		return "revoked", &CollaborationStageRevocationView{RevocationID: util.UUIDToString(revocation.RevocationID), ActorUserID: util.UUIDToString(revocation.ActorUserID), CreatedAt: revocation.CreatedAt.Time.UTC(), Reason: revocation.Reason}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", nil, err
	}
	if d.ExpiresAt.Valid && !now.Before(d.ExpiresAt.Time) {
		return "expired", nil, nil
	}
	if d.RoleRevision != state.task.RoleRevision {
		return "roles_changed", nil, nil
	}
	if d.PolicyRevision != state.policy.Revision || !state.policy.Enabled {
		return "policy_changed", nil, nil
	}
	if d.LifecycleEpoch != access.LifecycleEpoch || d.CandidateID != candidate.CandidateID || d.CandidateVersion.Int64 != candidate.Version || d.ContentDigest.String != candidate.ContentDigest {
		return "evidence_changed", nil, nil
	}
	current, _, err := stageSignerCurrent(ctx, q, state.role, d, now)
	if err != nil {
		return "", nil, err
	}
	if !current {
		return "signer_changed", nil, nil
	}
	head, err := q.GetCollaborationStageReviewHead(ctx, db.GetCollaborationStageReviewHeadParams{IssueID: d.IssueID, CandidateID: d.CandidateID, Step: d.Step})
	if err != nil {
		return "", nil, err
	}
	if head.DecisionID != d.DecisionID {
		return "superseded", nil, nil
	}
	if d.Decision == "approved" {
		if _, code, err := lockStageFinalSigners(ctx, tx, access, state, []db.CollaborationStageDecision{d}); err != nil {
			return "", nil, err
		} else if code != "" {
			status := map[string]string{"stage_decision_expired": "expired", "stage_signer_changed": "signer_changed", "stage_roles_changed": "roles_changed", "stage_policy_changed": "policy_changed"}[code]
			if status == "" {
				status = "evidence_changed"
			}
			return status, nil, nil
		}
		var payload stageReviewPayload
		if json.Unmarshal(d.Payload, &payload) != nil {
			return "", nil, errors.New("invalid retained stage decision")
		}
		if candidateRefusal != "" {
			return "evidence_changed", nil, nil
		}
		switch d.Step {
		case "code_review":
			if stageCIRefusal(observed.CI) != "" || payload.Evidence.CIDigest == nil || observed.CI.CIDigest == nil || *payload.Evidence.CIDigest != *observed.CI.CIDigest {
				return "evidence_changed", nil, nil
			}
		case "qa":
			valid, err := stageQAStandingState(ctx, tx, access, state, candidate, content, payload, observed, candidateRefusal, now)
			if err != nil {
				return "", nil, err
			}
			if !valid {
				return "evidence_changed", nil, nil
			}
		default:
			return "evidence_changed", nil, nil
		}
	}
	return "valid", nil, nil
}

func stageCodeCandidateState(candidate, head db.CollaborationStageCandidate, content stageCodeContent, access CollaborationIssueAccess, state *collaborationStageState, refusal string) string {
	if candidate.CandidateID != head.CandidateID {
		return "superseded"
	}
	if candidate.LifecycleEpoch != access.LifecycleEpoch || content.RoleRevision != state.task.RoleRevision || refusal != "" {
		return "stale"
	}
	return "current"
}

func stageDiscoverCodeCandidates(ctx context.Context, q *db.Queries, access CollaborationIssueAccess, state *collaborationStageState) (CollaborationStageCodeCandidates, error) {
	out := CollaborationStageCodeCandidates{Items: []CollaborationStageCodeCandidateSummary{}}
	rows, err := q.ListCollaborationStageCodeCandidates(ctx, db.ListCollaborationStageCodeCandidatesParams{IssueID: access.Issue.ID, Limit: 51})
	if err != nil {
		return out, err
	}
	if len(rows) > 50 {
		rows, out.Truncated = rows[:50], true
	}
	if len(rows) == 0 {
		return out, nil
	}
	now, err := q.ArtifactExecutionDatabaseTime(ctx)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		content, err := decodeStageCode(row)
		if err != nil {
			return out, err
		}
		refusal := ""
		if access.SourceMember {
			_, refusal, err = stageCodeCurrent(ctx, q, access, state, row, content, now.Time)
			if err != nil {
				return out, err
			}
		}
		out.Items = append(out.Items, CollaborationStageCodeCandidateSummary{CandidateID: util.UUIDToString(row.CandidateID), Version: stageVersion(row.Version), ContentDigest: row.ContentDigest, State: stageCodeCandidateState(row, rows[0], content, access, state, refusal)})
	}
	return out, nil
}

func (s *CollaborationService) StageCodeEvidence(ctx context.Context, issueID, candidateID pgtype.UUID) (CollaborationStageCodeEvidenceView, error) {
	out := CollaborationStageCodeEvidenceView{ProtocolVersion: 1, IssueID: util.UUIDToString(issueID), TestReports: []CollaborationStageTestReportSummary{}, Decisions: []CollaborationStageReviewView{}, ExecutionFacts: []CollaborationStageExecutionFactView{}, Signable: []CollaborationStageSignable{}, ExecutionTargets: []CollaborationStageExecutionTargetView{}, ExecutionOperations: []CollaborationStageExecutionOperationView{}, RollbackObservations: []CollaborationStageRollbackObservationView{}}
	err := s.readCollaborationStageEvidence(ctx, issueID, func(tx pgx.Tx, q *db.Queries, access CollaborationIssueAccess) error {
		if err := stageReady(ctx, q); err != nil {
			return err
		}
		state, _, err := stageReadState(ctx, q, access)
		if err != nil {
			return err
		}
		candidate, err := q.GetCollaborationStageCodeCandidate(ctx, db.GetCollaborationStageCodeCandidateParams{IssueID: issueID, CandidateID: candidateID})
		if errors.Is(err, pgx.ErrNoRows) {
			return collaborationError(404, "stage_candidate_not_found")
		}
		if err != nil {
			return err
		}
		head, err := q.GetCollaborationStageCodeHead(ctx, issueID)
		if err != nil {
			return err
		}
		content, err := decodeStageCode(candidate)
		if err != nil {
			return err
		}
		sourceReadable := access.SourceMember
		artifactRefusal := ""
		if sourceReadable && content.Baseline.Artifact != nil {
			artifactRefusal, err = checkStageArtifactCurrent(ctx, tx, content)
			if err != nil {
				return err
			}
			if artifactRefusal == "stage_evidence_unavailable" {
				sourceReadable = false
			}
		}
		now, err := q.ArtifactExecutionDatabaseTime(ctx)
		if err != nil {
			return err
		}
		out.CheckedAt = now.Time.UTC()
		out.Candidate = CollaborationStageCodeCandidateView{CandidateID: util.UUIDToString(candidateID), Version: stageVersion(candidate.Version), ContentDigest: candidate.ContentDigest, ParentCandidateID: stageText(candidate.ParentCandidateID), LifecycleEpoch: stageVersion(candidate.LifecycleEpoch), SubmittedBy: util.UUIDToString(candidate.SubmittedBy), ImplementerUserID: content.ImplementerUserID, RoleRevision: stageVersion(content.RoleRevision), CreatedAt: candidate.CreatedAt.Time.UTC()}
		out.Observed = CollaborationStageObserved{PullRequest: CollaborationStageObservedPR{State: "unavailable"}, CI: CollaborationStageObservedCI{State: "unavailable"}}
		refusal := ""
		if sourceReadable {
			out.Candidate.Baseline = &content.Baseline
			out.Observed, refusal, err = stageCodeCurrent(ctx, q, access, state, candidate, content, now.Time)
			if err != nil {
				return err
			}
			if refusal == "" && artifactRefusal != "" {
				refusal = artifactRefusal
			}
		} else {
			out.BlockedEvidence = 1
		}
		out.Candidate.State = stageCodeCandidateState(candidate, head, content, access, state, refusal)
		if sourceReadable {
			reports, err := q.ListCollaborationCandidateTestReportEvents(ctx, db.ListCollaborationCandidateTestReportEventsParams{IssueID: issueID, CandidateID: util.UUIDToString(candidateID), RowLimit: 50})
			if err != nil {
				return err
			}
			for _, event := range reports {
				stored, err := decodeStoredTestReport(event)
				if err != nil {
					return err
				}
				run, err := testReportReadRun(ctx, tx, access, event.RunID)
				if err != nil {
					return err
				}
				if err = testReportSourceRead(ctx, tx, access, event, stored); err != nil {
					return err
				}
				applicability, err := testReportApplicability(ctx, q, access, run, event, stored)
				if err != nil {
					return err
				}
				item := testReportItem(event, stored, applicability)
				out.TestReports = append(out.TestReports, CollaborationStageTestReportSummary{ReportID: item.ReportID, RunID: util.UUIDToString(event.RunID), Attempt: item.Attempt, SourceKind: item.SourceKind, ReportedResult: item.ReportedResult, Applicability: item.Applicability, CaseCount: item.CaseCount, FailedCaseCount: item.FailedCaseCount, ReceivedAt: item.ReceivedAt})
			}
			decisions, err := q.ListCollaborationStageReviewDecisions(ctx, db.ListCollaborationStageReviewDecisionsParams{IssueID: issueID, CandidateID: candidateID, Limit: 50})
			if err != nil {
				return err
			}
			for _, d := range decisions {
				var payload stageReviewPayload
				if json.Unmarshal(d.Payload, &payload) != nil {
					return errors.New("invalid retained stage decision")
				}
				status, revoked, err := s.stageExecutionReviewState(ctx, tx, access, state, candidate, content, d, out.Observed, refusal, now.Time)
				if err != nil {
					return err
				}
				var expires *time.Time
				if d.ExpiresAt.Valid {
					v := d.ExpiresAt.Time.UTC()
					expires = &v
				}
				out.Decisions = append(out.Decisions, CollaborationStageReviewView{DecisionID: util.UUIDToString(d.DecisionID), Step: d.Step, Decision: d.Decision, State: status, ActorUserID: util.UUIDToString(d.ActorUserID), Role: d.Role, RoleRevision: stageVersion(d.RoleRevision), PolicyRevision: stageVersion(d.PolicyRevision), CreatedAt: d.CreatedAt.Time.UTC(), ExpiresAt: expires, Reason: d.Reason, Evidence: payload.Evidence, DependsOn: payload.DependsOn, Revocation: revoked})
			}
		}
		if sourceReadable {
			if err = s.stageExecutionEvidence(ctx, tx, access, candidate, &out, now.Time); err != nil {
				return err
			}
		}
		principal, _ := auth.AuthenticatedPrincipalFromContext(ctx)
		for _, step := range []string{"code_review", "qa", "release", "rollback"} {
			signable := CollaborationStageSignable{Step: step, Blockers: []CollaborationStageBlocker{}}
			block := func(code string) {
				stage := step
				if step == "rollback" {
					stage = "verification"
				}
				if code == "stage_not_reached" {
					stage = state.task.Stage
				}
				signable.Blockers = append(signable.Blockers, CollaborationStageBlocker{Code: code, Stage: stage})
			}
			if !sourceReadable {
				block("stage_evidence_unavailable")
				out.Signable = append(out.Signable, signable)
				continue
			}
			switch step {
			case "release", "rollback":
				signable, err = s.stageExecutionSignable(ctx, tx, access, state, candidate, content, out, step, now.Time)
				if err != nil {
					return err
				}
			case "code_review", "qa":
				holder, _ := stageRoleHolder(state.role, step)
				if !access.Capabilities.ControlAI || holder != util.MustParseUUID(principal.UserID) || step == "code_review" && principal.UserID == content.ImplementerUserID {
					block("stage_evidence_unavailable")
				}
				if state.task.Stage != step {
					block("stage_not_reached")
				}
				if state.role == nil {
					block("stage_roles_unconfigured")
				}
				if len(collaboration.StageMissing(state.facts)) != 0 {
					block("stage_scope_unconfirmed")
				}
				if !state.policy.Enabled {
					block("stage_policy_changed")
				}
				if out.Candidate.State != "current" {
					block("stage_candidate_stale")
				}
				if refusal != "" {
					block(refusal)
				} else if code := stageCIRefusal(out.Observed.CI); code != "" {
					block(code)
				}
				var expiry *time.Time
				if principal.Kind == auth.CredentialJWT {
					if principal.ExpiresAt.IsZero() || !now.Time.Before(principal.ExpiresAt) {
						block("stage_evidence_unavailable")
					} else {
						v := principal.ExpiresAt
						expiry = &v
					}
				} else if principal.Kind == auth.CredentialPAT {
					token, err := q.GetPersonalAccessTokenByHash(ctx, principal.PATTokenHash)
					if errors.Is(err, pgx.ErrNoRows) || err == nil && util.UUIDToString(token.UserID) != principal.UserID {
						block("stage_evidence_unavailable")
					} else if err != nil {
						return err
					} else if token.ExpiresAt.Valid {
						v := token.ExpiresAt.Time
						expiry = &v
					}
				} else {
					block("stage_evidence_unavailable")
				}
				var dependencyExpiry *time.Time
				if step == "qa" {
					for _, decision := range out.Decisions {
						if decision.Step != "code_review" {
							continue
						}
						if decision.Decision == "approved" && decision.State == "valid" {
							dependencyExpiry = decision.ExpiresAt
						}
						break
					}
					if dependencyExpiry == nil {
						block("stage_code_review_missing")
					}
					usable := false
					for _, report := range out.TestReports {
						if report.Applicability == "usable" && report.ReportedResult == "passed" {
							usable = true
						}
					}
					if !usable {
						block("stage_test_report_unusable")
					}
				}
				if len(signable.Blockers) == 0 {
					maximum := stageMaxExpiry(now.Time, out.Observed.CI, expiry, content)
					if dependencyExpiry != nil && dependencyExpiry.Before(maximum) {
						maximum = *dependencyExpiry
					}
					if maximum.After(now.Time) {
						signable.Allowed = true
						signable.MaxExpiresAt = &maximum
					} else {
						block("stage_decision_expired")
					}
				}
			}
			out.Signable = append(out.Signable, signable)
		}
		return nil
	})
	if err != nil {
		return CollaborationStageCodeEvidenceView{}, err
	}
	return out, nil
}
