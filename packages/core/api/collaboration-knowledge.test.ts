// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { knowledgeCandidatesFixture, knowledgeDetailFixture, knowledgeIds, knowledgeProposalFixture, knowledgeReceiptFixture, knowledgeTargetFixture } from "../collaboration/knowledge-test-fixtures";

afterEach(() => { vi.unstubAllGlobals(); setCurrentWorkspace(null, null); });
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
const api = () => new ApiClient("https://server.example");
const approve = { protocol_version: 1 as const, request_id: knowledgeIds.request, expected_state_revision: "1", expected_version: "9007199254740993", expected_content_sha256: "b".repeat(64) };

it("routes every human read with the source-independent scope and exact pagination", async () => {
  setCurrentWorkspace("ambient", knowledgeIds.actor);
  const project = { kind: "project" as const, workspace_id: knowledgeIds.workspace, project_id: knowledgeIds.project };
  const fetchMock = vi.fn().mockResolvedValueOnce(json(knowledgeCandidatesFixture)).mockResolvedValueOnce(json(knowledgeDetailFixture))
    .mockResolvedValueOnce(json(knowledgeTargetFixture)).mockResolvedValueOnce(json({ ...knowledgeTargetFixture, target: project, items: [] }))
    .mockResolvedValueOnce(json(knowledgeReceiptFixture)); vi.stubGlobal("fetch", fetchMock);
  const client = api();
  expect(await client.getCollaborationKnowledgeCandidates(knowledgeIds.issue)).toEqual(knowledgeCandidatesFixture);
  expect(await client.getCollaborationKnowledgeCandidate(knowledgeIds.candidate)).toEqual(knowledgeDetailFixture);
  expect(await client.getCollaborationKnowledgeTarget(knowledgeTargetFixture.target)).toEqual(knowledgeTargetFixture);
  await client.getCollaborationKnowledgeTarget(project, { limit: 50, cursor: "opaque+cursor" });
  expect(await client.getCollaborationKnowledgeOperation(knowledgeIds.candidate, knowledgeIds.request)).toEqual(knowledgeReceiptFixture);
  expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
    `https://server.example/api/collaboration/issues/${knowledgeIds.issue}/knowledge-candidates?limit=20`,
    `https://server.example/api/collaboration/knowledge-candidates/${knowledgeIds.candidate}`,
    `https://server.example/api/workspaces/${knowledgeIds.workspace}/knowledge?limit=20`,
    `https://server.example/api/projects/${knowledgeIds.project}/knowledge?limit=50&cursor=opaque%2Bcursor`,
    `https://server.example/api/collaboration/knowledge-candidates/${knowledgeIds.candidate}/operations/${knowledgeIds.request}`,
  ]);
  fetchMock.mock.calls.forEach(([_url, init], index) => {
    expect(new Headers(init.headers).get("X-Workspace-ID")).toBe(index === 2 || index === 3 ? knowledgeIds.workspace : null);
    expect(init.cache).toBe("no-store");
  });
});

it("serializes all writes with the preselected candidate/request and exact revision unchanged", async () => {
  const receipts = [ { ...knowledgeReceiptFixture, action: "propose", code: "knowledge_proposed", state: "pending" }, knowledgeReceiptFixture,
    { ...knowledgeReceiptFixture, action: "reject", code: "knowledge_rejected", state: "rejected", state_revision: "3" },
    { ...knowledgeReceiptFixture, action: "withdraw", code: "knowledge_withdrawn", state: "withdrawn", state_revision: "3" }, knowledgeReceiptFixture ];
  const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(json(receipts.shift()))); vi.stubGlobal("fetch", fetchMock);
  const client = api(), decision = { protocol_version: 1 as const, request_id: knowledgeIds.request, expected_state_revision: "2", reason: "Current reason" };
  await client.proposeCollaborationKnowledge(knowledgeIds.issue, knowledgeProposalFixture);
  await client.approveCollaborationKnowledge(knowledgeIds.candidate, approve);
  await client.rejectCollaborationKnowledge(knowledgeIds.candidate, decision);
  await client.withdrawCollaborationKnowledge(knowledgeIds.candidate, decision);
  await client.approveCollaborationKnowledge(knowledgeIds.candidate, approve);
  expect(fetchMock.mock.calls.map(([, init]) => JSON.parse(init.body))).toEqual([knowledgeProposalFixture, approve, decision, decision, approve]);
  for (const [, init] of fetchMock.mock.calls) expect(new Headers(init.headers).get("Idempotency-Key")).toBe(knowledgeIds.request);
  expect(fetchMock.mock.calls[1]?.[1].body).toBe(fetchMock.mock.calls[4]?.[1].body);
});

it("does not misclassify structured access loss or malformed claimed support as an older route", async () => {
  const fetchMock = vi.fn().mockResolvedValueOnce(json({}, 404)).mockResolvedValueOnce(json({ code: "knowledge_candidate_not_found" }, 404))
    .mockResolvedValueOnce(json({ protocol_version: 1, candidate: { content: "PRIVATE" } })).mockResolvedValueOnce(json({ code: "operation_not_found" }, 404));
  vi.stubGlobal("fetch", fetchMock); const client = api();
  expect(await client.getCollaborationKnowledgeCandidate(knowledgeIds.candidate)).toBeNull();
  await expect(client.getCollaborationKnowledgeCandidate(knowledgeIds.candidate)).rejects.toMatchObject({ status: 404 });
  await expect(client.getCollaborationKnowledgeCandidate(knowledgeIds.candidate)).rejects.toMatchObject({ name: "CollaborationKnowledgeProtocolError" });
  expect(await client.getCollaborationKnowledgeOperation(knowledgeIds.candidate, knowledgeIds.request)).toBeNull();
});

it("rejects mismatched identities, cyclic pages, over-limit pages and foreign operation results", async () => {
  const fetchMock = vi.fn().mockResolvedValueOnce(json({ ...knowledgeCandidatesFixture, issue_id: knowledgeIds.actor, items: [] }))
    .mockResolvedValueOnce(json({ ...knowledgeCandidatesFixture, next_cursor: "same" })).mockResolvedValueOnce(json({ ...knowledgeReceiptFixture, candidate_id: knowledgeIds.actor }))
    .mockResolvedValueOnce(json({ ...knowledgeTargetFixture, items: Array.from({ length: 21 }, () => knowledgeTargetFixture.items[0]) }));
  vi.stubGlobal("fetch", fetchMock); const client = api();
  await expect(client.getCollaborationKnowledgeCandidates(knowledgeIds.issue)).rejects.toThrow();
  await expect(client.getCollaborationKnowledgeCandidates(knowledgeIds.issue, { cursor: "same" })).rejects.toThrow();
  await expect(client.getCollaborationKnowledgeOperation(knowledgeIds.candidate, knowledgeIds.request)).rejects.toThrow();
  await expect(client.getCollaborationKnowledgeTarget(knowledgeTargetFixture.target)).rejects.toThrow();
});

it("fails malformed write intent before transport and preserves HTTP conflicts for explicit reread", async () => {
  const fetchMock = vi.fn().mockResolvedValue(json({ code: "knowledge_revision_conflict" }, 409)); vi.stubGlobal("fetch", fetchMock); const client = api();
  await expect(client.approveCollaborationKnowledge(knowledgeIds.candidate, { ...approve, expected_content_sha256: "stale" })).rejects.toThrow();
  expect(fetchMock).not.toHaveBeenCalled();
  await expect(client.approveCollaborationKnowledge(knowledgeIds.candidate, approve)).rejects.toMatchObject({ status: 409 });
});

it("rejects in-progress recovery, malformed JSON and an approval of a different observed version", async () => {
  const fetchMock = vi.fn().mockResolvedValueOnce(json(knowledgeReceiptFixture, 202)).mockResolvedValueOnce(new Response("invalid PRIVATE json"))
    .mockResolvedValueOnce(json({ ...knowledgeReceiptFixture, version: "9007199254740994" }));
  vi.stubGlobal("fetch", fetchMock); const client = api();
  await expect(client.getCollaborationKnowledgeOperation(knowledgeIds.candidate, knowledgeIds.request)).rejects.toMatchObject({ name: "CollaborationKnowledgeProtocolError" });
  await expect(client.getCollaborationKnowledgeCandidate(knowledgeIds.candidate)).rejects.toMatchObject({ name: "CollaborationKnowledgeProtocolError" });
  await expect(client.approveCollaborationKnowledge(knowledgeIds.candidate, approve)).rejects.toMatchObject({ name: "CollaborationKnowledgeProtocolError" });
});

it("does not confirm contradictory HTTP status and sealed receipt outcomes", async () => {
  const conflict = { ...knowledgeReceiptFixture, code: "knowledge_revision_conflict", state: "pending" };
  const fetchMock = vi.fn().mockResolvedValueOnce(json(knowledgeReceiptFixture, 409)).mockResolvedValueOnce(json(conflict, 200))
    .mockResolvedValueOnce(json(conflict, 409)); vi.stubGlobal("fetch", fetchMock); const client = api();
  await expect(client.approveCollaborationKnowledge(knowledgeIds.candidate, approve)).rejects.toMatchObject({ name: "CollaborationKnowledgeProtocolError" });
  await expect(client.approveCollaborationKnowledge(knowledgeIds.candidate, approve)).rejects.toMatchObject({ name: "CollaborationKnowledgeProtocolError" });
  await expect(client.approveCollaborationKnowledge(knowledgeIds.candidate, approve)).resolves.toEqual(conflict);
});

it.each([
  ["project", 404], ["project", 500], ["workspace", 404], ["workspace", 500],
] as const)("redacts %s knowledge request/error logs at HTTP %s without losing workspace headers", async (kind, status) => {
  setCurrentWorkspace("ambient-private-slug", knowledgeIds.actor);
  const logger = { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() };
  const fetchMock = vi.fn().mockResolvedValue(json({ code: "knowledge_target_not_found", error: "PRIVATE_ERROR_SENTINEL" }, status));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://server.example", { logger });
  const target = kind === "project" ? { kind, workspace_id: knowledgeIds.workspace, project_id: knowledgeIds.project }
    : { kind, workspace_id: knowledgeIds.workspace };
  await expect(client.getCollaborationKnowledgeTarget(target, { cursor: "PRIVATE_CURSOR_SENTINEL" })).rejects.toMatchObject({ status });
  const entries = [...logger.debug.mock.calls, ...logger.info.mock.calls, ...logger.warn.mock.calls, ...logger.error.mock.calls];
  expect(JSON.stringify(entries)).not.toContain("PRIVATE_CURSOR_SENTINEL");
  expect(JSON.stringify(entries)).not.toContain("PRIVATE_ERROR_SENTINEL");
  expect(logger.info.mock.calls[0]?.[0]).toContain(`/knowledge`);
  expect(logger.info.mock.calls[0]?.[0]).not.toContain("?");
  const headers = new Headers(fetchMock.mock.calls[0]?.[1].headers);
  expect(headers.get("X-Workspace-ID")).toBe(knowledgeIds.workspace);
  expect(headers.get("X-Workspace-Slug")).toBe("");
});
