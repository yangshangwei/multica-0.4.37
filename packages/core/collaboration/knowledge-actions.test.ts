// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "../api";
import { clearCollaborationAccess, collaborationScope } from "./access";
import { executeKnowledgeIntent, matchesKnowledgeReceipt, prepareKnowledgeDecision, prepareKnowledgeProposal, recoverKnowledgeOperation } from "./knowledge-actions";
import { collaborationKnowledgeKeys } from "./knowledge-queries";
import { knowledgeCandidateFixture, knowledgeCandidatesFixture, knowledgeIds, knowledgeProposalFixture, knowledgeReceiptFixture, knowledgeUnavailableCandidateFixture } from "./knowledge-test-fixtures";
import { expireKnowledgeProjections } from "./knowledge-cache";
import { readPendingKnowledgeOperations } from "./knowledge-recovery";
import type { StorageAdapter } from "../types/storage";

vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const saved = vi.hoisted(() => new Map<string, string>());
vi.mock("../platform/storage", () => ({ defaultStorage: {
  keys: () => [...saved.keys()], getItem: (key: string) => saved.get(key) ?? null,
  setItem: (key: string, value: string) => saved.set(key, value), removeItem: (key: string) => saved.delete(key),
} }));
const cleanups: (() => void)[] = [];
afterEach(() => { cleanups.splice(0).forEach((dispose) => dispose()); saved.clear(); vi.restoreAllMocks(); });
function setup() {
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  const scope = collaborationScope(knowledgeIds.actor);
  cleanups.push(() => { expireKnowledgeProjections(client, scope, false); client.clear(); });
  return { api, client, scope };
}

it("allocates a new request UUID per intent and preselects the recoverable candidate before create", () => {
  const f = setup();
  const first = prepareKnowledgeProposal(f.scope, knowledgeCandidatesFixture, knowledgeProposalFixture);
  const second = prepareKnowledgeProposal(f.scope, knowledgeCandidatesFixture, knowledgeProposalFixture);
  expect(first.requestId).not.toBe(second.requestId); expect(first.candidateId).not.toBe(second.candidateId);
  expect(JSON.parse(first.requestJSON)).toMatchObject({ candidate_id: first.candidateId, request_id: first.requestId });
  expect(Object.isFrozen(first)).toBe(true);
});

it.each([null, "2020-01-01T00:00:00Z"])("can reject an unavailable pending candidate expiring at %s using only its exact state revision", async (expires_at) => {
  const f = setup(); const candidate = { ...knowledgeUnavailableCandidateFixture, expires_at };
  const intent = prepareKnowledgeDecision(f.scope, candidate, "reject", "Cannot establish this contribution");
  const send = vi.spyOn(f.api, "rejectCollaborationKnowledge").mockResolvedValue({ ...knowledgeReceiptFixture, request_id: intent.requestId,
    action: "reject", code: "knowledge_rejected", state: "rejected" });
  await expect(executeKnowledgeIntent(f.client, f.scope, intent)).resolves.toMatchObject({ code: "knowledge_rejected" });
  expect(send).toHaveBeenCalledExactlyOnceWith(candidate.candidate_id, { protocol_version: 1, request_id: intent.requestId,
    expected_state_revision: candidate.state_revision, reason: "Cannot establish this contribution" });
  expect(intent.requestJSON).not.toContain("PRIVATE");
  expect(() => prepareKnowledgeDecision(f.scope, candidate, "approve")).toThrow();
});

it("does not infer rejection rights from unavailable or restricted rows and keeps expired approval denied", () => {
  const f = setup();
  expect(() => prepareKnowledgeDecision(f.scope, { ...knowledgeUnavailableCandidateFixture, capabilities: { can_approve: false, can_reject: false, can_withdraw: true } }, "reject", "Reason")).toThrow();
  expect(() => prepareKnowledgeDecision(f.scope, { ...knowledgeUnavailableCandidateFixture, state: "rejected" }, "reject", "Reason")).toThrow();
  expect(() => prepareKnowledgeDecision(f.scope, { candidate_id: knowledgeIds.candidate, availability: "restricted", state: "pending", state_revision: "1", capabilities: { can_withdraw: true } }, "reject", "Reason")).toThrow();
  expect(() => prepareKnowledgeDecision(f.scope, { ...knowledgeCandidateFixture, expires_at: "2020-01-01T00:00:00Z", capabilities: { can_approve: false, can_reject: true, can_withdraw: true } }, "approve")).toThrow();
});

it("pins exact reviewed content/state revisions and refuses inferred or unavailable approval authority", () => {
  const f = setup(); const intent = prepareKnowledgeDecision(f.scope, knowledgeCandidateFixture, "approve");
  expect(JSON.parse(intent.requestJSON)).toMatchObject({ expected_state_revision: "1", expected_version: "9007199254740993", expected_content_sha256: "b".repeat(64) });
  expect(() => prepareKnowledgeDecision(f.scope, { ...knowledgeCandidateFixture, state: "withdrawn" }, "approve")).toThrow();
  expect(() => prepareKnowledgeDecision(f.scope, { ...knowledgeCandidateFixture, capabilities: { can_approve: false, can_reject: true, can_withdraw: true } }, "approve")).toThrow();
  const restricted = { candidate_id: knowledgeIds.candidate, state: "approved" as const, state_revision: "2", availability: "restricted" as const, capabilities: { can_withdraw: true } };
  expect(() => prepareKnowledgeDecision(f.scope, restricted, "approve")).toThrow();
  const withdraw = prepareKnowledgeDecision(f.scope, restricted, "withdraw", "No longer applicable");
  expect(JSON.parse(withdraw.requestJSON)).toEqual({ protocol_version: 1, request_id: withdraw.requestId, expected_state_revision: "2", reason: "No longer applicable" });
  expect(withdraw).not.toHaveProperty("issueId");
});

it("retains the same immutable request after uncertain transport and never optimistically approves", async () => {
  const f = setup(); const intent = prepareKnowledgeDecision(f.scope, knowledgeCandidateFixture, "approve");
  const send = vi.spyOn(f.api, "approveCollaborationKnowledge").mockRejectedValueOnce(new TypeError("network lost"))
    .mockResolvedValueOnce({ ...knowledgeReceiptFixture, request_id: intent.requestId });
  const key = collaborationKnowledgeKeys.candidate(f.scope, intent.candidateId); f.client.setQueryData(key, knowledgeCandidateFixture);
  await expect(executeKnowledgeIntent(f.client, f.scope, intent)).rejects.toThrow("network lost");
  expect(readPendingKnowledgeOperations(f.scope)).toEqual([{ scope: intent.scope, candidateId: intent.candidateId, requestId: intent.requestId, action: intent.action }]);
  expect(f.client.getQueryData(key)).toEqual(knowledgeCandidateFixture);
  await expect(executeKnowledgeIntent(f.client, f.scope, intent)).resolves.toMatchObject({ code: "knowledge_approved" });
  expect(send.mock.calls[0]).toEqual(send.mock.calls[1]); expect(f.client.getQueryData(key)).toBeUndefined();
  expect(readPendingKnowledgeOperations(f.scope)).toEqual([]);
});

it("recovers a content-free original receipt with candidate/request only after losing source access and target role", async () => {
  const f = setup(); const intent = prepareKnowledgeDecision(f.scope, knowledgeCandidateFixture, "withdraw", "Remove contribution");
  clearCollaborationAccess(f.client, f.scope, knowledgeIds.issue);
  const read = vi.spyOn(f.api, "getCollaborationKnowledgeOperation").mockResolvedValue({ ...knowledgeReceiptFixture, action: "withdraw", code: "knowledge_withdrawn", state: "withdrawn", request_id: intent.requestId });
  const write = vi.spyOn(f.api, "withdrawCollaborationKnowledge");
  const identity = { scope: intent.scope, candidateId: intent.candidateId, requestId: intent.requestId, action: intent.action };
  await expect(recoverKnowledgeOperation(f.client, f.scope, identity)).resolves.toMatchObject({ code: "knowledge_withdrawn" });
  expect(read).toHaveBeenCalledExactlyOnceWith(intent.candidateId, intent.requestId); expect(write).not.toHaveBeenCalled();
});

it("keeps unknown/mismatched receipts uncertain and clears private projections for explicit conflict reread", async () => {
  const f = setup(); const intent = prepareKnowledgeDecision(f.scope, knowledgeCandidateFixture, "approve");
  for (const fields of [{ action: "withdraw" as const }, { candidate_id: knowledgeIds.actor }, { request_id: knowledgeIds.actor }, { code: "new_backend_outcome" }, { state: "rejected" as const }]) {
    expect(matchesKnowledgeReceipt(intent, { ...knowledgeReceiptFixture, request_id: intent.requestId, ...fields })).toBe(false);
  }
  const send = vi.spyOn(f.api, "approveCollaborationKnowledge").mockResolvedValueOnce({ ...knowledgeReceiptFixture, request_id: intent.requestId, code: "new_backend_outcome" })
    .mockRejectedValueOnce(new ApiError("changed", 409, "Conflict"));
  await expect(executeKnowledgeIntent(f.client, f.scope, intent)).rejects.toThrow(/unavailable/);
  const key = collaborationKnowledgeKeys.candidate(f.scope, intent.candidateId); f.client.setQueryData(key, knowledgeCandidateFixture);
  await expect(executeKnowledgeIntent(f.client, f.scope, intent)).rejects.toMatchObject({ status: 409 });
  expect(f.client.getQueryData(key)).toBeUndefined(); expect(send).toHaveBeenCalledTimes(2);
  expect(JSON.parse(intent.requestJSON).expected_state_revision).toBe("1");
});

it("recovers the same original actor/key after a fresh login while fencing the old live request scope", async () => {
  const f = setup(); const intent = prepareKnowledgeDecision(f.scope, knowledgeCandidateFixture, "approve");
  const nextApi = new ApiClient("https://server.example"); setApiInstance(nextApi);
  const current = collaborationScope(knowledgeIds.actor);
  vi.spyOn(nextApi, "getCollaborationKnowledgeOperation").mockResolvedValue({ ...knowledgeReceiptFixture, request_id: intent.requestId });
  await expect(recoverKnowledgeOperation(f.client, f.scope, intent)).rejects.toThrow(/unavailable/);
  await expect(recoverKnowledgeOperation(f.client, current, intent)).resolves.toMatchObject({ code: "knowledge_approved" });
});

it("saves only recovery identity before a lost create response and refuses transport if preservation fails", async () => {
  const f = setup(); const intent = prepareKnowledgeProposal(f.scope, knowledgeCandidatesFixture, knowledgeProposalFixture);
  const send = vi.spyOn(f.api, "proposeCollaborationKnowledge").mockImplementation(async () => {
    expect(readPendingKnowledgeOperations(f.scope)[0]?.candidateId).toBe(intent.candidateId);
    throw new Error("lost response");
  });
  await expect(executeKnowledgeIntent(f.client, f.scope, intent)).rejects.toThrow("lost response");
  const stored = JSON.stringify([...saved.values()]);
  for (const text of ["PRIVATE", "source", "requestJSON", "sha256", "receipt"]) expect(stored).not.toContain(text);
  expect(readPendingKnowledgeOperations({ ...f.scope, userId: knowledgeIds.project })).toEqual([]);
  await expect(executeKnowledgeIntent(f.client, f.scope, intent, { getItem: () => null, setItem: () => {}, removeItem: () => {} })).rejects.toThrow(/preserved/);
  expect(send).toHaveBeenCalledTimes(1);
});

const unsealedRejection = () => new ApiError("invalid_knowledge_request", 400, "Bad Request", { code: "invalid_knowledge_request", error: "invalid_knowledge_request" });
function testStorage(): StorageAdapter {
  const entries = new Map<string, string>();
  return { keys: () => [...entries.keys()], getItem: (key) => entries.get(key) ?? null, setItem: (key, value) => { entries.set(key, value); }, removeItem: (key) => { entries.delete(key); } };
}

it("retires only a first definitively unused proposal identity so a new explicit intent can be created", async () => {
  const f = setup(); const intent = prepareKnowledgeProposal(f.scope, knowledgeCandidatesFixture, knowledgeProposalFixture);
  vi.spyOn(f.api, "proposeCollaborationKnowledge").mockRejectedValue(unsealedRejection());
  await expect(executeKnowledgeIntent(f.client, f.scope, intent)).rejects.toMatchObject({ name: "KnowledgeUnusedIntentError", reason: "rejected" });
  expect(readPendingKnowledgeOperations(f.scope)).toEqual([]);
  const next = prepareKnowledgeProposal(f.scope, knowledgeCandidatesFixture, { ...knowledgeProposalFixture, content: "Explicit corrected proposal" });
  expect(next.requestId).not.toBe(intent.requestId); expect(next.candidateId).not.toBe(intent.candidateId);
});

it("retains an earlier ambiguous request through later unsealed rejection, missing recovery and storage changes", async () => {
  const f = setup(); const intent = prepareKnowledgeProposal(f.scope, knowledgeCandidatesFixture, knowledgeProposalFixture);
  const send = vi.spyOn(f.api, "proposeCollaborationKnowledge").mockRejectedValueOnce(new TypeError("Lost response")).mockRejectedValue(unsealedRejection());
  vi.spyOn(f.api, "getCollaborationKnowledgeOperation").mockResolvedValue(null);
  await expect(executeKnowledgeIntent(f.client, f.scope, intent)).rejects.toThrow("Lost response");
  await expect(executeKnowledgeIntent(f.client, f.scope, intent)).rejects.toMatchObject({ status: 400 });
  await expect(recoverKnowledgeOperation(f.client, f.scope, intent)).rejects.toThrow(/unavailable/);
  expect(readPendingKnowledgeOperations(f.scope)[0]?.requestId).toBe(intent.requestId);
  const reloadedClient = new QueryClient();
  try { await expect(executeKnowledgeIntent(reloadedClient, f.scope, intent)).rejects.toMatchObject({ status: 400 }); }
  finally { reloadedClient.clear(); }
  // A fresh storage adapter does not erase dispatch evidence in the live client.
  await expect(executeKnowledgeIntent(f.client, f.scope, intent, testStorage())).rejects.toMatchObject({ status: 400 });
  expect(send.mock.calls[0]).toEqual(send.mock.calls[1]); expect(send.mock.calls[1]).toEqual(send.mock.calls[2]); expect(send.mock.calls[2]).toEqual(send.mock.calls[3]);
});

it("does not discard an overlapping attempt when the first call receives an unsealed rejection", async () => {
  const f = setup(); const intent = prepareKnowledgeProposal(f.scope, knowledgeCandidatesFixture, knowledgeProposalFixture);
  let rejectFirst!: (error: unknown) => void;
  let finishSecond!: (value: typeof knowledgeReceiptFixture | null) => void;
  vi.spyOn(f.api, "proposeCollaborationKnowledge").mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectFirst = reject; }))
    .mockImplementationOnce(() => new Promise((resolve) => { finishSecond = resolve; }));
  const first = executeKnowledgeIntent(f.client, f.scope, intent).catch((error: unknown) => error);
  const second = executeKnowledgeIntent(f.client, f.scope, intent);
  rejectFirst(unsealedRejection());
  expect(await first).toMatchObject({ status: 400 }); expect(readPendingKnowledgeOperations(f.scope)[0]?.requestId).toBe(intent.requestId);
  finishSecond({ ...knowledgeReceiptFixture, candidate_id: intent.candidateId, request_id: intent.requestId, action: "propose", code: "knowledge_proposed", state: "pending" });
  await expect(second).resolves.toMatchObject({ code: "knowledge_proposed" });
});

it("distinguishes failed preservation before POST from failed cleanup after a known receipt", async () => {
  const f = setup(); const intent = prepareKnowledgeProposal(f.scope, knowledgeCandidatesFixture, knowledgeProposalFixture);
  const send = vi.spyOn(f.api, "proposeCollaborationKnowledge");
  const storage = testStorage(); vi.spyOn(storage, "setItem").mockImplementation(() => { throw new Error("storage full"); });
  await expect(executeKnowledgeIntent(f.client, f.scope, intent, storage)).rejects.toMatchObject({ name: "KnowledgeUnusedIntentError", reason: "storage" });
  expect(send).not.toHaveBeenCalled(); expect(readPendingKnowledgeOperations(f.scope, storage)).toEqual([]);
  const after = testStorage(); vi.spyOn(after, "removeItem").mockImplementation(() => { throw new Error("cleanup failed"); });
  const receipt = { ...knowledgeReceiptFixture, candidate_id: intent.candidateId, request_id: intent.requestId, action: "propose" as const, code: "knowledge_proposed", state: "pending" as const };
  send.mockResolvedValue(receipt);
  await expect(executeKnowledgeIntent(f.client, f.scope, intent, after)).rejects.toMatchObject({ name: "KnowledgeReceiptCleanupError", receipt });
  expect(send).toHaveBeenCalledOnce(); expect(readPendingKnowledgeOperations(f.scope, after)[0]?.requestId).toBe(intent.requestId);
  vi.spyOn(f.api, "getCollaborationKnowledgeOperation").mockResolvedValue(receipt);
  await expect(recoverKnowledgeOperation(f.client, f.scope, intent, after)).rejects.toMatchObject({ name: "KnowledgeReceiptCleanupError", receipt });
  expect(send).toHaveBeenCalledOnce();
});
