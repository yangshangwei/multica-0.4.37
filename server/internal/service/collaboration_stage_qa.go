package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/collaboration"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func stageQAReport(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, candidate db.CollaborationStageCandidate, id string, lock bool) (collaborationStoredTestReport, string, error) {
	return stageQAReportAs(ctx, tx, access, candidate, id, lock, completionRequestCredential(ctx))
}

func stageQAReportAs(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, candidate db.CollaborationStageCandidate, id string, lock bool, credential ArtifactCredentialSnapshot) (collaborationStoredTestReport, string, error) {
	q := db.New(tx)
	var p collaborationStoredTestReport
	if !access.SourceMember {
		return p, "stage_evidence_unavailable", nil
	}
	e, err := q.GetCollaborationStageTestReportEvent(ctx, db.GetCollaborationStageTestReportEventParams{IssueID: access.Issue.ID, ID: util.MustParseUUID(id)})
	if errors.Is(err, pgx.ErrNoRows) {
		return p, "stage_evidence_changed", nil
	}
	if err != nil {
		return p, "", err
	}
	p, err = decodeStoredTestReport(e)
	if err != nil {
		return p, "", err
	}
	if err = testReportSourceReadAs(ctx, tx, access, e, p, credential); err != nil {
		if collaborationErrorCodeForReport(err, "not_found") {
			return p, "stage_evidence_unavailable", nil
		}
		return p, "", err
	}
	if p.Report.Subject.CandidateID != util.UUIDToString(candidate.CandidateID) || p.Report.Subject.CandidateVersion != stageVersion(candidate.Version) || p.Report.Subject.CandidateDigest != candidate.ContentDigest {
		return p, "stage_evidence_changed", nil
	}
	if _, err = testReportLogs(ctx, q, e, p); err != nil {
		if collaborationErrorCodeForReport(err, "test_report_log_not_retained") {
			return p, "stage_evidence_unavailable", nil
		}
		return p, "", err
	}
	run, err := q.GetCollaborationRecordedRun(ctx, db.GetCollaborationRecordedRunParams{IssueID: access.Issue.ID, RunID: e.RunID})
	if err != nil {
		return p, "", err
	}
	if lock {
		run, err = q.LockCollaborationStageTestRun(ctx, run.ID)
		if err != nil {
			return p, "", err
		}
	}
	app, err := testReportApplicability(ctx, q, access, run, e, p)
	if err != nil {
		return p, "", err
	}
	if app != "usable" || p.Report.ReportedResult() != "passed" {
		return p, "stage_test_report_unusable", nil
	}
	return p, "", nil
}

func stageQADependency(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, state *collaborationStageState, candidate db.CollaborationStageCandidate, content stageCodeContent, id string, observed CollaborationStageObserved, candidateCode string, now time.Time) (db.CollaborationStageDecision, string, error) {
	q := db.New(tx)
	d, err := q.GetCollaborationStageDecision(ctx, db.GetCollaborationStageDecisionParams{IssueID: access.Issue.ID, DecisionID: util.MustParseUUID(id)})
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (d.CandidateID != candidate.CandidateID || d.Step != "code_review" || d.Decision != "approved") {
		return d, "stage_dependency_invalid", nil
	}
	if err != nil {
		return d, "", err
	}
	valid, _, err := stageReviewState(ctx, tx, access, state, candidate, content, d, observed, candidateCode, now)
	if err != nil {
		return d, "", err
	}
	if valid != "valid" {
		return d, stageDecisionRefusal(valid), nil
	}
	return d, "", nil
}

func (t *collaborationWorkflowTx) decideStageQA(ctx context.Context, p CollaborationStageReviewParams, state *collaborationStageState) (CollaborationWorkflowReceipt, error) {
	candidate, content, refusal, err := t.codeCandidate(ctx, p.Candidate)
	if refusal != nil || err != nil {
		return stageRefusal(refusal), err
	}
	if p.Evidence.DeploymentID != nil || p.Evidence.RollbackTargetDigest != nil {
		return t.result("stage_dependency_invalid", 409), nil
	}
	if !t.access.SourceMember && (p.Decision == "approve" || p.Evidence.TestReportID != nil || p.Evidence.CIDigest != nil || p.Evidence.Build != nil || len(p.DependsOn) > 0) {
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
	observed, candidateCode := CollaborationStageObserved{}, "stage_evidence_unavailable"
	if t.access.SourceMember {
		observed, candidateCode, err = stageCodeCurrent(ctx, t.q, t.access, state, candidate, content, now.Time)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
	}
	if p.Evidence.CIDigest != nil && (observed.CI.CIDigest == nil || *p.Evidence.CIDigest != *observed.CI.CIDigest) {
		belongs := false
		if p.Decision == "return" {
			belongs, err = stageCIEvidenceBelongs(ctx, t.q, p.IssueID, candidate.CandidateID, *p.Evidence.CIDigest, observed.CI)
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
		}
		if !belongs {
			return t.result("stage_evidence_changed", 409), nil
		}
	}
	var dependencies []db.CollaborationStageDecision
	var report collaborationStoredTestReport
	if p.Evidence.TestReportID != nil {
		var code string
		report, code, err = stageQAReport(ctx, t.tx, t.access, candidate, *p.Evidence.TestReportID, true)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if code != "" && (code != "stage_test_report_unusable" || p.Decision == "approve") {
			return t.result(code, 409), nil
		}
	}
	if p.Evidence.Build != nil {
		a := content.Baseline.Artifact
		if a == nil || p.Evidence.Build.ArtifactDigest != a.ArtifactDigest || p.Evidence.TestReportID != nil && (report.Report.Subject.ArtifactDigest == nil || *report.Report.Subject.ArtifactDigest != a.ArtifactDigest) {
			return t.result("stage_evidence_changed", 409), nil
		}
	}
	if p.Decision == "approve" {
		if len(collaboration.StageMissing(state.facts)) != 0 {
			return t.result("stage_needs_confirmation", 409), nil
		}
		if candidateCode != "" {
			return t.result(candidateCode, 409), nil
		}
		if p.Evidence.TestReportID == nil {
			return t.result("stage_test_report_unusable", 409), nil
		}
		if len(p.DependsOn) != 1 {
			return t.result("stage_dependency_invalid", 409), nil
		}
		d, code, err := stageQADependency(ctx, t.tx, t.access, state, candidate, content, p.DependsOn[0], observed, candidateCode, now.Time)
		if err != nil {
			return CollaborationWorkflowReceipt{}, err
		}
		if code != "" {
			return t.result(code, 409), nil
		}
		dependencies = append(dependencies, d)
		maximum := stageMaxExpiry(now.Time, observed.CI, signer.ExpiresAt, content)
		if d.ExpiresAt.Time.Before(maximum) {
			maximum = d.ExpiresAt.Time
		}
		if p.ExpiresAt == nil || !p.ExpiresAt.After(now.Time) || p.ExpiresAt.After(maximum) {
			return CollaborationWorkflowReceipt{}, collaborationError(400, "stage_expiry_exceeds_limit")
		}
		if code, err = checkStageArtifactCurrent(ctx, t.tx, content); err != nil {
			return CollaborationWorkflowReceipt{}, err
		} else if code != "" {
			return t.result(code, 409), nil
		}
	} else {
		// Return may cite an expired/superseded decision; it does not consume
		// that decision as authority. Foreign references still refuse.
		for _, id := range p.DependsOn {
			d, err := t.q.GetCollaborationStageDecision(ctx, db.GetCollaborationStageDecisionParams{IssueID: p.IssueID, DecisionID: util.MustParseUUID(id)})
			if errors.Is(err, pgx.ErrNoRows) || err == nil && d.CandidateID != candidate.CandidateID {
				return t.result("stage_dependency_invalid", 409), nil
			}
			if err != nil {
				return CollaborationWorkflowReceipt{}, err
			}
		}
	}
	final, code, err := checkStageFinalAuthority(ctx, t.tx, t.access, state, dependencies)
	if err != nil {
		return CollaborationWorkflowReceipt{}, err
	}
	if code == "forbidden" {
		return CollaborationWorkflowReceipt{}, collaborationError(403, code)
	}
	if code != "" {
		return t.result(code, 409), nil
	}
	if p.Decision == "approve" && (observed.CI.ObservedAt == nil || !final.Before(observed.CI.ObservedAt.Add(stageCIHorizon))) {
		return t.result("stage_ci_stale", 409), nil
	}
	return t.recordStageReview(ctx, p, state, candidate, signer, credential)
}

func stageQAStandingState(ctx context.Context, tx pgx.Tx, access CollaborationIssueAccess, state *collaborationStageState, candidate db.CollaborationStageCandidate, content stageCodeContent, payload stageReviewPayload, observed CollaborationStageObserved, candidateCode string, now time.Time) (bool, error) {
	if payload.Evidence.TestReportID == nil || len(payload.DependsOn) != 1 || payload.Evidence.DeploymentID != nil || payload.Evidence.RollbackTargetDigest != nil {
		return false, nil
	}
	_, code, err := stageQADependency(ctx, tx, access, state, candidate, content, payload.DependsOn[0], observed, candidateCode, now)
	if err != nil || code != "" {
		return false, err
	}
	report, code, err := stageQAReport(ctx, tx, access, candidate, *payload.Evidence.TestReportID, false)
	if err != nil || code != "" {
		return false, err
	}
	if payload.Evidence.Build != nil && (report.Report.Subject.ArtifactDigest == nil || *report.Report.Subject.ArtifactDigest != payload.Evidence.Build.ArtifactDigest) {
		return false, nil
	}
	return true, nil
}

func stageQAFinalDependencies(ctx context.Context, q *db.Queries, d db.CollaborationStageDecision) ([]db.CollaborationStageDecision, error) {
	if d.Step != "qa" || d.Decision != "approved" {
		return nil, nil
	}
	var p stageReviewPayload
	if json.Unmarshal(d.Payload, &p) != nil || len(p.DependsOn) != 1 || !stageUUID(p.DependsOn[0]) {
		return nil, collaborationError(409, "stage_dependency_invalid")
	}
	dependency, err := q.GetCollaborationStageDecision(ctx, db.GetCollaborationStageDecisionParams{IssueID: d.IssueID, DecisionID: util.MustParseUUID(p.DependsOn[0])})
	if err != nil {
		return nil, err
	}
	return []db.CollaborationStageDecision{dependency}, nil
}
