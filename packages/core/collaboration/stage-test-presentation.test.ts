// @vitest-environment node
import { expect, it } from "vitest";
import { parseCollaborationIssue } from "./schema";
import { parseCollaborationStageCodeCandidate, parseCollaborationStageView } from "./stage-schema";
import { parseCollaborationTestReport } from "./stage-test-report-schema";
import { canPrepareCollaborationTest, collaborationTestReportMatchesCandidate } from "./stage-test-presentation";
import { deriveCollaborationStageCodeState } from "./stage-code-presentation";
import { collaborationHostExecutionFixture, collaborationIssueFixture } from "./test-fixtures";
import { stageCodeViewFixture, stageIds, stageCodeCandidateRefFixture, stageCodeReviewDecisionFixture, stageIssueContextFixture, stageViewManagedFixture } from "./stage-test-fixtures";
import { stageReportDetailFixture, stageTestSubjectFixture } from "./stage-test-report-fixtures";
const issue = parseCollaborationIssue({ ...collaborationIssueFixture, ...stageIssueContextFixture, execution: collaborationHostExecutionFixture, ai_policy: "assisted" })!;
const view = parseCollaborationStageView({ ...stageViewManagedFixture, stage: { current: "qa", stage_revision: "3" }, code_candidates: { items: [{ ...stageCodeCandidateRefFixture, state: "current" }], truncated: false } })!;
const record = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture, decisions: [stageCodeReviewDecisionFixture] })!;
const now = Date.parse("2026-11-14T03:05:00Z");
it("uses current comment/command authority and a valid review, independent of the viewer's profession", () => {
  expect(canPrepareCollaborationTest(issue, view, record, stageIds.product, true, stageTestSubjectFixture, now)).toBe(true);
  expect(canPrepareCollaborationTest({ ...issue, capabilities: { ...issue.capabilities, submit_command: false } }, view, record, stageIds.qa, false, undefined, now)).toBe(false);
  expect(canPrepareCollaborationTest({ ...issue, ai_policy: "manual" }, view, record, stageIds.product, true, undefined, now)).toBe(false);
  expect(canPrepareCollaborationTest(issue, view, record, stageIds.product, true, { ...stageTestSubjectFixture, candidate_version: "2" }, now)).toBe(false);
  expect(canPrepareCollaborationTest(issue, view, record, stageIds.product, true, undefined, Date.parse("2026-11-14T03:25:00Z"))).toBe(false);
});
it("blocks test input after source, candidate, stage or approval changes", () => {
  expect(canPrepareCollaborationTest(issue, view, { ...record, candidate: { ...record.candidate, baseline: null } }, stageIds.product, true, undefined, now)).toBe(false);
  expect(canPrepareCollaborationTest(issue, view, { ...record, decisions: [] }, stageIds.product, true, undefined, now)).toBe(false);
  expect(canPrepareCollaborationTest(issue, view, { ...record, observed: { ...record.observed, pull_request: { state: "head_changed", head_sha: "b".repeat(40) } } }, stageIds.product, true, undefined, now)).toBe(false);
  expect(canPrepareCollaborationTest(issue, { ...view, stage: { current: "development", stage_revision: "4" } }, record, stageIds.product, true, undefined, now)).toBe(false);
  expect(canPrepareCollaborationTest(issue, { ...view, code_candidates: { items: [], truncated: false } }, record, stageIds.product, true, undefined, now)).toBe(false);
});
it("B01 respects current R1 source denial before a cached R4 refresh", () => {
  const denied = parseCollaborationStageView({ ...view, children: { ...view.children,
    items: view.children.items.map((child) => ({ ...child, can_open: false, content_digest: null })) } })!;
  expect(canPrepareCollaborationTest(issue, denied, record, stageIds.product, true, undefined, now)).toBe(false);
  const state = deriveCollaborationStageCodeState(issue, denied, record, stageIds.product, now);
  expect(state.readable).toBe(false);
  expect(state.writable).toBe(false);
  expect(state.approve).toEqual([]);
  expect(state.advances).toEqual([]);
});
it("requires exact selected report/source/artifact identity and never substitutes a source-only build", () => {
  const report = parseCollaborationTestReport(stageReportDetailFixture)!;
  const code = { ...record, test_reports: [{ ...report, run_id: report.run_id }] };
  expect(collaborationTestReportMatchesCandidate(code, report)).toBe(true);
  for (const subject of [{ ...report.subject, candidate_version: "2" }, { ...report.subject, source_revision: "b".repeat(40) }, { ...report.subject, artifact_digest: "a".repeat(64) }])
    expect(collaborationTestReportMatchesCandidate(code, { ...report, subject })).toBe(false);
  expect(collaborationTestReportMatchesCandidate(code, { ...report, applicability: "usable" })).toBe(false);
});
