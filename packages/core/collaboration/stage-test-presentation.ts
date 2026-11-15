import type { CollaborationIssue } from "./schema";
import { deriveCollaborationStageCodeState } from "./stage-code-presentation";
import type { CollaborationStageCodeCandidate, CollaborationStageView } from "./stage-schema";
import type { CollaborationTestReport } from "./stage-test-report-schema";
import { sameCollaborationTestSubject, type CollaborationTestSubject } from "./stage-test-subject-schema";

export const collaborationCodeTestSubject = (record: CollaborationStageCodeCandidate): CollaborationTestSubject => ({
  candidate_id: record.candidate.candidate_id, candidate_version: record.candidate.version, candidate_digest: record.candidate.content_digest,
});

/** Profession is not authority; the existing human comment/command rights still govern admission. */
export function canPrepareCollaborationTest(issue: CollaborationIssue, view: CollaborationStageView | null, record: CollaborationStageCodeCandidate | null,
  viewerId: string, canSubmitCommands: boolean, subject?: CollaborationTestSubject, now = Date.now()): boolean {
  // The mention projection governs typed-comment admission. The issue's
  // direct-command affordance is intentionally disabled until runtime acceptance.
  if (!view || !record || issue.capabilities.read !== true || issue.capabilities.comment !== true || canSubmitCommands !== true || issue.execution?.protocol_version !== 6 ||
    issue.ai_policy !== "assisted" || view.stage?.current !== "qa" || subject && !sameCollaborationTestSubject(subject, collaborationCodeTestSubject(record))) return false;
  const state = deriveCollaborationStageCodeState(issue, view, record, viewerId, now);
  return state.writable && state.readable && record.observed.pull_request.state === "current" &&
    record.observed.pull_request.head_sha === record.candidate.baseline?.pull_request.head_sha && state.validDecisions.some((decision) => decision.step === "code_review" && decision.decision === "approved" &&
    decision.evidence.ci_digest === record.observed.ci.ci_digest);
}

/** T4 detail must describe the selected R4 report and the exact source-bound candidate. */
export function collaborationTestReportMatchesCandidate(record: CollaborationStageCodeCandidate, report: CollaborationTestReport): boolean {
  const baseline = record.candidate.baseline;
  const summary = record.test_reports.find((item) => item.report_id === report.report_id);
  return baseline !== null && record.blocked_evidence === 0 && record.issue_id === report.issue_id && summary !== undefined &&
    summary.run_id === report.run_id && summary.attempt === report.attempt && summary.reported_result === report.reported_result &&
    summary.applicability === report.applicability && summary.case_count === report.case_count && summary.failed_case_count === report.failed_case_count &&
    sameCollaborationTestSubject(collaborationCodeTestSubject(record), report.subject) && report.subject.source_revision === baseline.pull_request.head_sha &&
    report.subject.artifact_digest === (baseline.artifact?.artifact_digest ?? null);
}
