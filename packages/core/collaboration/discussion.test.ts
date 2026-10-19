// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationIssueAccessDenied, collaborationScope, clearCollaborationAccess } from "./access";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";
import { readCollaborationDrafts } from "./draft";
import { prepareCollaborationDiscussion, executeCollaborationDiscussion } from "./discussion";
import { parseCollaborationDiscussionReceipt, parseCollaborationIssueReactions } from "./discussion-schema";

const actor = "00000000-0000-4000-8000-000000000001";
const target = "00000000-0000-4000-8000-000000000002";
const requestId = "00000000-0000-4000-8000-000000000003";
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const values = new Map<string, string>();
const storage = { keys: () => [...values.keys()], getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => { values.set(key, value); }, removeItem: (key: string) => { values.delete(key); } };
const client = new QueryClient();
const issue = parseCollaborationIssue(collaborationIssueFixture)!;
const receipt = { protocol_version: 2, issue_id: actor, comment_id: target, request_id: requestId, action: "add_reaction", emoji: "👍", status: 200, code: "reaction_added" };
afterEach(() => { client.clear(); values.clear(); vi.unstubAllGlobals(); });

it("rejects invalid reaction summaries and inconsistent or non-private operation receipts", () => {
  expect(parseCollaborationIssueReactions({ issue_id: actor, items: [{ emoji: "👍", count: 2, viewer_reacted: true }] })).not.toBeNull();
  expect(parseCollaborationIssueReactions({ issue_id: actor, items: [{ emoji: "👍", count: "2", viewer_reacted: true }] })).toBeNull();
  expect(parseCollaborationDiscussionReceipt(receipt)).not.toBeNull();
  for (const change of [{ action: "resolve" }, { code: "command_queued" }, { description: "Private text" }, { request_id: undefined }]) {
    expect(parseCollaborationDiscussionReceipt({ ...receipt, ...change })).toBeNull();
  }
});

it("retains original target, action and emoji and recovers by GET without repeating a reaction", async () => {
  setApiInstance(new ApiClient("https://discussion.example"));
  const scope = collaborationScope(actor);
  const draft = prepareCollaborationDiscussion(scope, issue, requestId, { action: "add_reaction", commentId: target, emoji: "👍" });
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("offline")).mockResolvedValueOnce(new Response(JSON.stringify(receipt)));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationDiscussion(client, scope, draft, false, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  await expect(executeCollaborationDiscussion(client, scope, draft, true, storage)).resolves.toEqual(receipt);
  expect(fetch.mock.calls[0]?.[0]).toContain(`/comments/${target}/reactions/${encodeURIComponent("👍")}`);
  expect(fetch.mock.calls[0]?.[1].headers["Idempotency-Key"]).toBe(requestId);
  expect(fetch.mock.calls[1]?.[0]).toContain(`/discussion/operations/${requestId}`);
  expect(fetch.mock.calls[1]?.[1].method ?? "GET").toBe("GET");
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
});

it("keeps a missing or mismatched result pending without revoking the issue", async () => {
  setApiInstance(new ApiClient("https://discussion.example"));
  const scope = collaborationScope(actor);
  const draft = prepareCollaborationDiscussion(scope, issue, requestId, { action: "resolve", commentId: target });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ code: "operation_not_found" }), { status: 404 }))
    .mockResolvedValueOnce(new Response(JSON.stringify(receipt))));
  await expect(executeCollaborationDiscussion(client, scope, draft, true, storage)).rejects.toThrow();
  await expect(executeCollaborationDiscussion(client, scope, draft, true, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  expect(collaborationIssueAccessDenied(client, scope, actor)).toBe(false);
});

it("can settle only a content-free own receipt after known access loss", async () => {
  setApiInstance(new ApiClient("https://discussion.example"));
  const scope = collaborationScope(actor);
  const draft = prepareCollaborationDiscussion(scope, issue, requestId, { action: "add_reaction", commentId: target, emoji: "👍" });
  clearCollaborationAccess(client, scope, actor);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify(receipt))));
  await expect(executeCollaborationDiscussion(client, scope, draft, true, storage)).resolves.toEqual(receipt);
  expect(collaborationIssueAccessDenied(client, scope, actor)).toBe(true);
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
});

it.each([undefined, null, "🎉", 1])("retains an original reaction when immediate and recovered emoji do not match (%j)", async (emoji) => {
  setApiInstance(new ApiClient("https://discussion.example"));
  const scope = collaborationScope(actor);
  const draft = prepareCollaborationDiscussion(scope, issue, requestId, { action: "add_reaction", commentId: target, emoji: "👍" });
  const fetch = vi.fn().mockImplementation(async () => new Response(JSON.stringify({ ...receipt, emoji })));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationDiscussion(client, scope, draft, false, storage)).rejects.toThrow();
  await expect(executeCollaborationDiscussion(client, scope, draft, true, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  expect(fetch.mock.calls).toHaveLength(2);
});

it("requires null emoji for thread actions and rejects impossible tombstone refusals", () => {
  const resolved = { ...receipt, action: "resolve", code: "comment_resolved", emoji: null };
  expect(parseCollaborationDiscussionReceipt(resolved)).not.toBeNull();
  expect(parseCollaborationDiscussionReceipt({ ...resolved, emoji: undefined })).toBeNull();
  expect(parseCollaborationDiscussionReceipt({ ...resolved, emoji: "👍" })).toBeNull();
  expect(parseCollaborationDiscussionReceipt({ ...resolved, status: 409, code: "comment_deleted" })).not.toBeNull();
  expect(parseCollaborationDiscussionReceipt({ ...resolved, action: "unresolve", status: 409, code: "comment_deleted" })).toBeNull();
  expect(parseCollaborationDiscussionReceipt({ ...receipt, action: "remove_reaction", status: 409, code: "comment_deleted" })).toBeNull();
});

it("matches the 64-byte server reaction limit before reserving a draft", async () => {
  setApiInstance(new ApiClient("https://discussion.example"));
  const scope = collaborationScope(actor);
  expect(() => prepareCollaborationDiscussion(scope, issue, requestId, { action: "add_reaction", commentId: target, emoji: "👍".repeat(16) })).not.toThrow();
  expect(() => prepareCollaborationDiscussion(scope, issue, requestId, { action: "add_reaction", commentId: target, emoji: "字".repeat(22) })).toThrow();
  expect(readCollaborationDrafts({ ...scope, issueId: issue.id }, storage)).toEqual([]);
});
