// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "../api";
import { collaborationIssueAccessDenied, collaborationKeys, collaborationScope } from "./access";
import { collaborationStageCodeCandidateOptions } from "./stage-queries";
import { collaborationTestReportOptions, collaborationTestReportsOptions } from "./stage-test-report-queries";
import { eraseCollaborationStageSource, protectCollaborationStageSource } from "./stage-source-cache";
import { parseCollaborationTestReport, parseCollaborationTestReports } from "./stage-test-report-schema";
import { parseCollaborationStageCodeCandidate } from "./stage-schema";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";
import { stageCodeViewFixture, stageIds } from "./stage-test-fixtures";
import { stageReportDetailFixture, stageReportPageFixture } from "./stage-test-report-fixtures";
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000203" } }) } }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); vi.restoreAllMocks(); });
function setup() {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); clients.push(client);
  const scope = collaborationScope(stageIds.product);
  return { api, client, scope };
}
const detail = parseCollaborationTestReport(stageReportDetailFixture)!;
const page = parseCollaborationTestReports(stageReportPageFixture)!;
it("clears workspace T4 and R4 on ambiguous report denial while preserving readable issue identity", async () => {
  const { api, client, scope } = setup();
  const current = collaborationTestReportOptions(client, scope, stageIds.issue, stageIds.workspace, stageIds.run, detail.report_id);
  const sibling = collaborationTestReportsOptions(client, scope, stageIds.child1, stageIds.workspace, stageIds.run);
  const other = collaborationTestReportsOptions(client, scope, stageIds.child2, stageIds.project, stageIds.run);
  const code = collaborationStageCodeCandidateOptions(client, scope, stageIds.issue, stageIds.candidate1, stageIds.workspace);
  client.setQueryData(sibling.queryKey, { pages: [{ state: "available", value: page }], pageParams: [undefined] });
  client.setQueryData(other.queryKey, { pages: [{ state: "available", value: page }], pageParams: [undefined] });
  client.setQueryData(code.queryKey, { state: "available", value: parseCollaborationStageCodeCandidate(stageCodeViewFixture)! });
  vi.spyOn(api, "getCollaborationTestReport").mockResolvedValue({ state: "not_found" });
  const readIssue = vi.spyOn(api, "getCollaborationIssue").mockResolvedValue(parseCollaborationIssue(collaborationIssueFixture));
  await expect(client.fetchQuery(current)).resolves.toEqual({ state: "not_found" });
  expect(readIssue).toHaveBeenCalledOnce();
  expect(client.getQueryData(sibling.queryKey)).toBeUndefined();
  expect(client.getQueryData(code.queryKey)).toBeUndefined();
  expect(client.getQueryData(other.queryKey)).toBeDefined();
  expect(client.getQueryData(collaborationKeys.issue(scope, stageIds.issue))).toHaveProperty("id", stageIds.issue);
  expect(collaborationIssueAccessDenied(client, scope, stageIds.issue)).toBe(false);
});
it("lets an authoritative issue denial perform normal issue cleanup", async () => {
  const { api, client, scope } = setup();
  vi.spyOn(api, "getCollaborationTestReport").mockResolvedValue({ state: "not_found" });
  vi.spyOn(api, "getCollaborationIssue").mockRejectedValue(new ApiError("denied", 404, "Not Found"));
  await expect(client.fetchQuery(collaborationTestReportOptions(client, scope, stageIds.issue, stageIds.workspace, stageIds.run, detail.report_id))).rejects.toThrow();
  expect(collaborationIssueAccessDenied(client, scope, stageIds.issue)).toBe(true);
});
it.each(["success", "error"])("fences late T4 %s after workspace source loss", async (mode) => {
  const { client, scope } = setup();
  let finish!: () => void;
  const result = protectCollaborationStageSource(client, scope, stageIds.issue, stageIds.workspace,
    () => new Promise((resolve, reject) => { finish = () => mode === "success" ? resolve(detail) : reject(new ApiError("old denial", 404, "Not Found")); })).catch((error: unknown) => error);
  eraseCollaborationStageSource(client, scope, stageIds.workspace);
  finish();
  await expect(result).resolves.toMatchObject({ name: "AbortError" });
  expect(collaborationIssueAccessDenied(client, scope, stageIds.issue)).toBe(false);
});
it.each(["duplicate", "loop", "identity", "unavailable"])("refuses an assembled T4 %s page chain", (mode) => {
  const { client, scope } = setup();
  const options = collaborationTestReportsOptions(client, scope, stageIds.issue, stageIds.workspace, stageIds.run);
  const sharing = options.structuralSharing;
  if (typeof sharing !== "function") throw new Error("Page validation missing");
  const next = mode === "unavailable" ? { state: "unavailable", reason: "storage" } : { state: "available", value: {
    ...page, issue_id: mode === "identity" ? stageIds.child1 : stageIds.issue,
    items: mode === "duplicate" ? page.items : [{ ...page.items[0], report_id: stageIds.child2 }], next_cursor: mode === "loop" ? "cursor" : null,
  } };
  expect(() => sharing(undefined, { pages: [{ state: "available", value: { ...page, next_cursor: "cursor" } }, next], pageParams: [undefined, "cursor"] })).toThrow();
});
