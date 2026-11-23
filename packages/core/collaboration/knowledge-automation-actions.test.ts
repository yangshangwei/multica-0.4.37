// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationScope } from "./access";
import { executeKnowledgeAutomationIntent, prepareProjectKnowledgePolicy, prepareKnowledgeAutomationRetry,
  readPendingKnowledgeAutomationOperations, recoverKnowledgeAutomationOperation } from "./knowledge-automation-actions";
import { knowledgeIds, knowledgePolicyFixture, knowledgeJobFixture } from "./knowledge-test-fixtures";
import { KnowledgeUnusedIntentError } from "./knowledge-actions";
import type { StorageAdapter } from "../types/storage";

vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); vi.restoreAllMocks(); });
function setup() {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const client = new QueryClient(); clients.push(client);
  const entries = new Map<string, string>();
  const storage: StorageAdapter = { keys: () => [...entries.keys()], getItem: (key) => entries.get(key) ?? null,
    setItem: (key, value) => { entries.set(key, value); }, removeItem: (key) => { entries.delete(key); } };
  return { api, client, scope: collaborationScope(knowledgeIds.actor), storage, entries };
}

it("retries the frozen policy request and recovers only routing identity after reload", async () => {
  const f = setup();
  const intent = prepareProjectKnowledgePolicy(f.scope, knowledgePolicyFixture, { isEnabled: false, maxCandidates: 1 });
  const receipt = { protocol_version: 2 as const, request_id: intent.requestId, project_id: knowledgeIds.project,
    revision: "1", code: "knowledge_policy_updated" as const };
  const send = vi.spyOn(f.api, "updateProjectKnowledgePolicy").mockRejectedValueOnce(new TypeError("Lost response")).mockResolvedValueOnce(receipt);
  await expect(executeKnowledgeAutomationIntent(f.client, f.scope, intent, f.storage)).rejects.toThrow("Lost response");
  const stored = readPendingKnowledgeAutomationOperations(f.scope, f.storage);
  expect(stored).toHaveLength(1);
  expect(JSON.stringify([...f.entries.values()])).not.toMatch(/requestJSON|is_enabled|max_candidates|expected_revision/);
  await expect(executeKnowledgeAutomationIntent(f.client, f.scope, intent, f.storage)).resolves.toEqual(receipt);
  expect(send.mock.calls[0]).toEqual(send.mock.calls[1]);
  expect(readPendingKnowledgeAutomationOperations(f.scope, f.storage)).toEqual([]);
  const read = vi.spyOn(f.api, "getProjectKnowledgePolicyOperation").mockResolvedValue(receipt);
  await expect(recoverKnowledgeAutomationOperation(f.client, f.scope, stored[0]!, f.storage)).resolves.toEqual(receipt);
  expect(read).toHaveBeenCalledExactlyOnceWith(knowledgeIds.project, intent.requestId);
  expect(send).toHaveBeenCalledTimes(2);
});

it("does not erase an uncertain request or declare it unused after a later storage failure", async () => {
  const f = setup();
  const intent = prepareProjectKnowledgePolicy(f.scope, knowledgePolicyFixture, { isEnabled: false, maxCandidates: 1 });
  vi.spyOn(f.api, "updateProjectKnowledgePolicy").mockRejectedValue(new TypeError("Lost response"));
  await expect(executeKnowledgeAutomationIntent(f.client, f.scope, intent, f.storage)).rejects.toThrow("Lost response");
  vi.spyOn(f.storage, "setItem").mockImplementation(() => { throw new Error("Storage blocked"); });
  const retry = executeKnowledgeAutomationIntent(f.client, f.scope, intent, f.storage);
  await expect(retry).rejects.not.toBeInstanceOf(KnowledgeUnusedIntentError);
  expect(readPendingKnowledgeAutomationOperations(f.scope, f.storage)).toHaveLength(1);
});

it("requires current retry permission and keeps the original job revision on retry", async () => {
  const f = setup();
  expect(() => prepareKnowledgeAutomationRetry(f.scope, knowledgeJobFixture)).toThrow();
  const job = { ...knowledgeJobFixture, phase: "review_pending" as const, can_retry: true };
  const intent = prepareKnowledgeAutomationRetry(f.scope, job);
  const send = vi.spyOn(f.api, "retryCollaborationKnowledgeJob").mockResolvedValue({ protocol_version: 2,
    request_id: intent.requestId, issue_id: job.issue_id, job_id: job.id, revision: "2", code: "knowledge_job_retried" });
  await expect(executeKnowledgeAutomationIntent(f.client, f.scope, intent, f.storage)).resolves.toMatchObject({ code: "knowledge_job_retried" });
  expect(send).toHaveBeenCalledExactlyOnceWith(job.issue_id, job.id, { request_id: intent.requestId, expected_revision: "1" });
});
