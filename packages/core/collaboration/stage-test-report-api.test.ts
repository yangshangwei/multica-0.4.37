// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api";
import { stageIds } from "./stage-test-fixtures";
import { stageReportDetailFixture as detail, stageReportPageFixture as page } from "./stage-test-report-fixtures";
afterEach(() => vi.unstubAllGlobals());
const setup = (...responses: Response[]) => {
  const fetch = vi.fn(); responses.forEach((response) => fetch.mockResolvedValueOnce(response)); vi.stubGlobal("fetch", fetch);
  return { api: new ApiClient("https://server.example"), fetch };
};
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
it("reads T4 list/detail under account scope with opaque cursor, exact identity and resolved hash verification", async () => {
  const { api, fetch } = setup(json(page), json(detail));
  await expect(api.getCollaborationTestReports(stageIds.issue, stageIds.run, { cursor: "opaque/+", limit: 20 })).resolves.toMatchObject({ state: "available" });
  await expect(api.getCollaborationTestReport(stageIds.issue, stageIds.run, detail.report_id)).resolves.toMatchObject({ state: "available", value: detail });
  expect(new URL(fetch.mock.calls[0]![0]).searchParams.get("cursor")).toBe("opaque/+");
  for (const [, init] of fetch.mock.calls) { expect(init.cache).toBe("no-store"); expect(init.headers).not.toHaveProperty("X-Workspace-ID"); }
});
it.each([
  { ...detail, issue_id: stageIds.child1 }, { ...detail, run_id: stageIds.child1 }, { ...detail, report_id: stageIds.child1 },
  { ...detail, logs: [{ ...detail.logs[0], body: "tampered" }, detail.logs[1]] },
])("refuses mismatched identity or resolved log bytes", async (value) => {
  const { api } = setup(json(value));
  await expect(api.getCollaborationTestReport(stageIds.issue, stageIds.run, detail.report_id)).resolves.toEqual({ state: "unavailable", reason: "malformed" });
});
it.each([
  [404, "not_found", { state: "not_found" }], [409, "test_report_log_not_retained", { state: "unavailable", reason: "log_not_retained" }],
  [503, "test_report_unavailable", { state: "unavailable", reason: "storage" }], [404, undefined, { state: "unsupported" }],
  [404, "unexpected_code", { state: "unavailable", reason: "malformed" }],
])("keeps T4 %s/%s separate from issue denial and drops private error text", async (status, code, expected) => {
  const { api } = setup(json({ code, error: "private log text" }, status));
  await expect(api.getCollaborationTestReport(stageIds.issue, stageIds.run, detail.report_id)).resolves.toEqual(expected);
});
it("rejects invalid pagination and a repeated cursor", async () => {
  const { api, fetch } = setup(json({ ...page, next_cursor: "same" }));
  await expect(api.getCollaborationTestReports(stageIds.issue, stageIds.run, { limit: 51 })).rejects.toThrow();
  expect(fetch).not.toHaveBeenCalled();
  await expect(api.getCollaborationTestReports(stageIds.issue, stageIds.run, { cursor: "same" })).resolves.toEqual({ state: "unavailable", reason: "malformed" });
});
