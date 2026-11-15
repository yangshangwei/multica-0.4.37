// @vitest-environment node
import { expect, it } from "vitest";
import { parseCollaborationTestReport, parseCollaborationTestReports, verifyCollaborationTestReportLogs } from "./stage-test-report-schema";
import { stageReportDetailFixture as detail, stageReportPageFixture as page } from "./stage-test-report-fixtures";
it("retains original failure, attempt, null source-only artifact and resolved log identity", async () => {
  const report = parseCollaborationTestReport(detail)!;
  expect(report).toMatchObject({ source_kind: "runtime_reported", subject: { artifact_digest: null }, reported_result: "failed", applicability: "execution_failed", failed_case_count: 1 });
  expect(report.logs[0]).toMatchObject({ kind: "record", event_id: detail.logs[0]!.event_id, message_index: 2 });
  expect(await verifyCollaborationTestReportLogs(report)).toBe(true);
  expect(parseCollaborationTestReports(page)?.items[0]?.subject).toEqual(detail.subject);
});
it.each([
  { version: 2 }, { subject: { ...detail.subject, artifact_digest: undefined } }, { source_kind: "verified_ci" },
  { attempt: 1 }, { case_count: 3 }, { failed_case_count: 0 }, { reported_result: "passed" },
  { cases: [detail.cases[0], detail.cases[0]] }, { cases: [{ ...detail.cases[0], log_index: 9 }, detail.cases[1]] },
  { logs: [{ kind: "inline", body: "ok", body_sha256: detail.logs[1]!.body_sha256, event_id: detail.logs[0]!.event_id }] },
  { logs: [] }, { cases: [] }, { logs: [{ ...detail.logs[0], body: "a".repeat(65537) }, detail.logs[1]] },
])("rejects malformed or contradictory report data %j", (change) => { expect(parseCollaborationTestReport({ ...detail, ...change })).toBeNull(); });
it("does not equate producer pass with usable execution", () => {
  expect(parseCollaborationTestReport({ ...detail, cases: detail.cases.map((c) => ({ ...c, outcome: "passed" })), failed_case_count: 0,
    reported_result: "passed", applicability: "execution_failed" })).toMatchObject({ reported_result: "passed", applicability: "execution_failed" });
});
it("rejects changed resolved bytes without trusting the stated digest", async () => {
  const report = parseCollaborationTestReport({ ...detail, logs: [{ ...detail.logs[0], body: "changed" }, detail.logs[1]] })!;
  expect(await verifyCollaborationTestReportLogs(report)).toBe(false);
});
it("rejects repeated page items and accepts explicit empty authorized history", () => {
  expect(parseCollaborationTestReports({ ...page, items: [page.items[0], page.items[0]] })).toBeNull();
  expect(parseCollaborationTestReports({ ...page, items: [], next_cursor: null })).not.toBeNull();
});
