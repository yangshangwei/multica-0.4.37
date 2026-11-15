import { stageCodeCandidateRefFixture, stageIds, stagePullRequestFixture } from "./stage-test-fixtures";
export const stageTestSubjectFixture = { candidate_id: stageCodeCandidateRefFixture.candidate_id, candidate_version: stageCodeCandidateRefFixture.version, candidate_digest: stageCodeCandidateRefFixture.content_digest };
export const stageReportSummaryFixture = {
  report_id: "00000000-0000-4000-8000-000000000901", report_digest: "d".repeat(64), agent_id: stageIds.agent,
  attempt: "1", source_kind: "runtime_reported", subject: { ...stageTestSubjectFixture, binding_digest: "e".repeat(64), source_revision: stagePullRequestFixture.head_sha, artifact_digest: null },
  reported_result: "failed", applicability: "execution_failed", case_count: 2, failed_case_count: 1, received_at: "2026-11-14T03:02:00Z",
};
export const stageReportPageFixture = { version: 1, issue_id: stageIds.issue, run_id: stageIds.run, items: [stageReportSummaryFixture], next_cursor: null };
export const stageReportDetailFixture = {
  version: 1, issue_id: stageIds.issue, run_id: stageIds.run, ...stageReportSummaryFixture, suite_id: "synthetic-payment-regression",
  cases: [{ id: "callback-delay", outcome: "failed", log_index: 0 }, { id: "callback-duplicate", outcome: "passed", log_index: 1 }],
  logs: [{ kind: "record", event_id: stageIds.comment, message_index: 2, body_sha256: "621ac065ce4642b3a5646742a423ecf9ddf5d60b04c49346e58dfb0eb5dcdf0c", body: "Expected paid, observed pending" },
    { kind: "inline", body_sha256: "2689367b205c16ce32ed4200942b8b8b1e262dfc70d9bc9fbc77c49699a4f1df", body: "ok" }],
};
