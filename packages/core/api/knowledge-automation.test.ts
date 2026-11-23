// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setCurrentWorkspace } from "../platform";
import { knowledgeIds, knowledgePolicyFixture, knowledgeJobsFixture, knowledgeJobFixture } from "../collaboration/knowledge-test-fixtures";

afterEach(() => { vi.unstubAllGlobals(); setCurrentWorkspace(null, null); });
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
const policyReceipt = { protocol_version: 2, request_id: knowledgeIds.request, project_id: knowledgeIds.project, code: "knowledge_policy_updated", revision: "1" };
const retryReceipt = { protocol_version: 2, request_id: knowledgeIds.request, issue_id: knowledgeIds.issue, job_id: knowledgeJobFixture.id, code: "knowledge_job_retried", revision: "2" };

it("uses v2 and explicit project workspace scope while original recovery remains account-scoped", async () => {
  setCurrentWorkspace("ambient", knowledgeIds.actor);
  const fetch = vi.fn().mockResolvedValueOnce(response(knowledgePolicyFixture)).mockResolvedValueOnce(response(policyReceipt))
    .mockResolvedValueOnce(response(policyReceipt)).mockResolvedValueOnce(response(knowledgeJobsFixture))
    .mockResolvedValueOnce(response(retryReceipt)).mockResolvedValueOnce(response(retryReceipt));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://server.example");
  expect(await api.getProjectKnowledgePolicy(knowledgeIds.workspace, knowledgeIds.project)).toEqual(knowledgePolicyFixture);
  const input = { request_id: knowledgeIds.request, expected_revision: "0", is_enabled: false, max_candidates: 1 };
  await api.updateProjectKnowledgePolicy(knowledgeIds.workspace, knowledgeIds.project, input);
  await api.getProjectKnowledgePolicyOperation(knowledgeIds.project, knowledgeIds.request);
  expect(await api.getCollaborationKnowledgeJobs(knowledgeIds.issue)).toEqual(knowledgeJobsFixture);
  await api.retryCollaborationKnowledgeJob(knowledgeIds.issue, knowledgeJobFixture.id, { request_id: knowledgeIds.request, expected_revision: "1" });
  await api.getCollaborationKnowledgeJobOperation(knowledgeIds.issue, knowledgeJobFixture.id, knowledgeIds.request);
  for (const [index, [url, init]] of fetch.mock.calls.entries()) {
    const headers = new Headers(init.headers);
    expect(headers.get("X-Multica-Knowledge-Version")).toBe("2");
    expect(headers.get("X-Workspace-ID")).toBe(index < 2 ? knowledgeIds.workspace : null);
    expect(init.cache).toBe("no-store");
    expect(url).not.toContain("?");
  }
  expect(fetch.mock.calls[1]?.[1].method).toBe("PUT");
  expect(JSON.parse(fetch.mock.calls[1]?.[1].body)).toEqual(input);
  expect(JSON.parse(fetch.mock.calls[4]?.[1].body)).toEqual({ request_id: knowledgeIds.request, expected_revision: "1" });
});

it("retains sealed refusals but rejects mismatched identity, revision and HTTP outcome", async () => {
  const conflict = { ...policyReceipt, code: "knowledge_policy_revision_conflict", revision: "3" };
  const fetch = vi.fn().mockResolvedValueOnce(response(conflict, 409)).mockResolvedValueOnce(response(conflict))
    .mockResolvedValueOnce(response({ ...policyReceipt, request_id: knowledgeIds.actor }))
    .mockResolvedValueOnce(response({ ...policyReceipt, revision: "4" }))
    .mockResolvedValueOnce(response({ code: "operation_not_found" }, 404));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://server.example");
  const input = { request_id: knowledgeIds.request, expected_revision: "0", is_enabled: false, max_candidates: 1 };
  expect(await api.updateProjectKnowledgePolicy(knowledgeIds.workspace, knowledgeIds.project, input)).toEqual(conflict);
  await expect(api.updateProjectKnowledgePolicy(knowledgeIds.workspace, knowledgeIds.project, input)).rejects.toThrow();
  await expect(api.updateProjectKnowledgePolicy(knowledgeIds.workspace, knowledgeIds.project, input)).rejects.toThrow();
  await expect(api.updateProjectKnowledgePolicy(knowledgeIds.workspace, knowledgeIds.project, input)).rejects.toThrow();
  expect(await api.getProjectKnowledgePolicyOperation(knowledgeIds.project, knowledgeIds.request)).toBeNull();
});

it("keeps policy request and provider errors out of generic logs", async () => {
  const logger = { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() };
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ code: "knowledge_permission_denied", error: "PRIVATE_PROVIDER_SENTINEL" }, 403)));
  const api = new ApiClient("https://server.example", { logger });
  await expect(api.getProjectKnowledgePolicy(knowledgeIds.workspace, knowledgeIds.project)).rejects.toMatchObject({ status: 403 });
  expect(JSON.stringify([...logger.debug.mock.calls, ...logger.info.mock.calls, ...logger.warn.mock.calls, ...logger.error.mock.calls])).not.toContain("PRIVATE_PROVIDER_SENTINEL");
});

it.each(["knowledge_job_revision_conflict", "knowledge_source_changed", "knowledge_automation_disabled"])("returns the confirmed %s retry outcome without another request", async (code) => {
  const refusal = { ...retryReceipt, code, revision: "1" };
  const fetch = vi.fn().mockResolvedValue(response(refusal, 409));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://server.example");
  expect(await api.retryCollaborationKnowledgeJob(knowledgeIds.issue, knowledgeJobFixture.id,
    { request_id: knowledgeIds.request, expected_revision: "1" })).toEqual(refusal);
  expect(fetch).toHaveBeenCalledTimes(1);
});
