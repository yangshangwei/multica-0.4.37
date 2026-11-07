// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationIssueAccessDenied, collaborationScope } from "./access";
import { parseCollaborationComment, parseCollaborationTriggerPreview } from "./content-schema";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";
import { readCollaborationDrafts, saveCollaborationDraft } from "./draft";
import { executeCollaborationCommentChange, prepareCollaborationCommentChange } from "./comment-actions";
import { prepareCollaborationComment } from "./comment";

const actor = "00000000-0000-4000-8000-000000000001";
const commentId = "00000000-0000-4000-8000-000000000002";
const requestId = "00000000-0000-4000-8000-000000000003";
const timestamp = "2026-10-19T00:00:00Z";
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const values = new Map<string, string>();
const storage = { keys: () => [...values.keys()], getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => { values.set(key, value); }, removeItem: (key: string) => { values.delete(key); } };
const client = new QueryClient();
const issue = parseCollaborationIssue(collaborationIssueFixture)!;
const comment = parseCollaborationComment({ id: commentId, issue_id: actor, author_type: "member", author: issue.creator, content: "Original", type: "comment", parent_id: null,
  comment_revision: "9007199254740993", attachment_ids: [requestId], created_at: timestamp, updated_at: timestamp, deleted_at: null, resolved_at: null, resolved_by: null, can_edit: true, can_resolve: true, reactions: [] })!;
afterEach(() => { client.clear(); values.clear(); vi.unstubAllGlobals(); });

it("uses a stable reply parent without turning that parent into an AI target", () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const preview = parseCollaborationTriggerPreview({ protocol_version: 2, issue_id: actor, lifecycle_epoch: "1", recipients: [], targets: [] })!;
  const draft = prepareCollaborationComment(collaborationScope(actor), actor, requestId, "A reply", [], preview, undefined, commentId);
  expect(draft.payload).toMatchObject({ input: { parent_id: commentId, target_versions: [] } });
});

it("requires both author identity and current comment permission for editing", () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const change = { action: "edit" as const, content: "Changed", attachmentIds: [] };
  for (const invalid of [{ ...comment, author: { ...comment.author!, user_id: requestId } }, { ...comment, author_type: "agent" }, { ...comment, deleted_at: timestamp }, { ...comment, can_edit: false }]) {
    expect(() => prepareCollaborationCommentChange(scope, issue, invalid, requestId, change)).toThrow();
  }
  expect(() => prepareCollaborationCommentChange(scope, { ...issue, capabilities: { ...issue.capabilities, comment: false } }, comment, requestId, change)).toThrow();
});

it("preserves an edit and its exact version through ambiguous transport and a confirmed conflict", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const draft = prepareCollaborationCommentChange(scope, issue, comment, requestId, { action: "edit", content: "My unsent edit", attachmentIds: [] });
  const refusal = { status: 409, code: "stale_comment_revision", protocol_version: 2, request_id: requestId, action: "edit_comment", issue_id: actor,
    comment_id: commentId, comment_revision: "9007199254740994", event_id: null, targets: [] };
  const fetch = vi.fn().mockImplementationOnce(async () => { expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]); throw new TypeError("offline"); })
    .mockResolvedValueOnce(new Response(JSON.stringify(refusal)));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationCommentChange(client, scope, draft, false, storage)).rejects.toThrow();
  await expect(executeCollaborationCommentChange(client, scope, draft, true, storage)).resolves.toMatchObject(refusal);
  expect(JSON.parse(fetch.mock.calls[0]?.[1].body)).toEqual({ content: "My unsent edit", attachment_ids: [], expected_comment_revision: "9007199254740993" });
  expect(fetch.mock.calls[1]?.[0]).toContain(`/comments/operations/${requestId}`);
  expect(fetch.mock.calls[1]?.[1].method ?? "GET").toBe("GET");
  expect(draft.payload).toMatchObject({ input: { content: "My unsent edit", expected_comment_revision: "9007199254740993" } });
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(collaborationIssueAccessDenied(client, scope, actor)).toBe(false);
});

it("does not mistake a foreign comment receipt for the pending delete", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const draft = prepareCollaborationCommentChange(scope, issue, comment, requestId, { action: "delete" });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ status: 200, code: "comment_deleted", protocol_version: 2, issue_id: actor,
    comment_id: requestId, comment_revision: "2", event_id: requestId, event_seq: "1", targets: [] }))));
  await expect(executeCollaborationCommentChange(client, scope, draft, true, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
});

it.each([{ request_id: undefined }, { request_id: commentId }, { action: "edit_comment" }, { status: 302 }])("keeps incomplete or mismatched delete success pending (%j)", async (change) => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const draft = prepareCollaborationCommentChange(scope, issue, comment, requestId, { action: "delete" });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ status: 200, code: "comment_deleted", protocol_version: 2, issue_id: actor,
    comment_id: commentId, request_id: requestId, action: "delete_comment", comment_revision: "9007199254740994", event_id: requestId, event_seq: "1", targets: [], ...change }))));
  await expect(executeCollaborationCommentChange(client, scope, draft, true, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
});

it.each(["界".repeat(21845) + "a", "😀".repeat(16384)])("accepts exactly 65536 UTF-8 bytes and refuses one byte more", async (content) => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  expect(() => prepareCollaborationCommentChange(scope, issue, comment, requestId, { action: "edit", content })).not.toThrow();
  expect(() => prepareCollaborationCommentChange(scope, issue, comment, requestId, { action: "edit", content: content + "a" })).toThrow();
  expect(readCollaborationDrafts({ ...scope, issueId: actor }, storage)).toEqual([]);
});

function oversizedOriginal(scope: ReturnType<typeof collaborationScope>) {
  const initial = prepareCollaborationCommentChange(scope, issue, comment, requestId, { action: "edit", content: "Valid draft", attachmentIds: [] });
  return { ...initial, payload: { domain: "comment_change", commentId, action: "edit", input: { content: "界".repeat(30000), expected_comment_revision: comment.comment_revision, attachment_ids: [] } } };
}
const oversizedRefusal = { status: 400, code: "invalid_comment_request", protocol_version: 2, request_id: requestId, action: "edit_comment", issue_id: actor,
  comment_id: commentId, comment_revision: comment.comment_revision, event_id: null, targets: [] };

it.each([false, true])("never persists or transports a newly constructed oversized request (recovery=%s)", async (recovery) => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const draft = oversizedOriginal(scope);
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationCommentChange(client, scope, draft, recovery, storage)).rejects.toThrow();
  expect(fetch).not.toHaveBeenCalled();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
});

it("uses only the saved oversized request to obtain a durable refusal before unlocking editing", async () => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const draft = oversizedOriginal(scope);
  saveCollaborationDraft(draft, storage);
  const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(oversizedRefusal), { status: 400 }))
    .mockResolvedValueOnce(new Response(JSON.stringify(oversizedRefusal)));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationCommentChange(client, scope, draft, false, storage)).resolves.toMatchObject(oversizedRefusal);
  expect(JSON.parse(fetch.mock.calls[0]?.[1].body)).toMatchObject({ content: "界".repeat(30000), expected_comment_revision: comment.comment_revision });
  expect(fetch.mock.calls[0]?.[1].headers["Idempotency-Key"]).toBe(requestId);
  expect(fetch.mock.calls[1]?.[0]).toContain(`/comments/operations/${requestId}`);
  expect(fetch.mock.calls[1]?.[1].method ?? "GET").toBe("GET");
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
});

it.each([202, 404])("does not retire an oversized original when the result GET has HTTP %d", async (status) => {
  setApiInstance(new ApiClient("https://comments.example"));
  const scope = collaborationScope(actor);
  const draft = oversizedOriginal(scope);
  saveCollaborationDraft(draft, storage);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(oversizedRefusal), { status: 400 }))
    .mockResolvedValueOnce(new Response(JSON.stringify(status === 404 ? { code: "not_found" } : oversizedRefusal), { status })));
  await expect(executeCollaborationCommentChange(client, scope, draft, false, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  expect(collaborationIssueAccessDenied(client, scope, actor)).toBe(false);
});
