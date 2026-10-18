// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationIssueAccessDenied, collaborationKeys, collaborationScope } from "./access";
import { parseCollaborationMentionTargets, parseCollaborationTriggerPreview } from "./content-schema";
import { readCollaborationDrafts } from "./draft";
import { collaborationMentionMarkdown, executeCollaborationComment, prepareCollaborationComment } from "./comment";
import { newCollaborationDeliveryForm, readCollaborationDeliveryRecords, saveCollaborationDeliveryRecord } from "./delivery-draft";

const actor = "00000000-0000-4000-8000-000000000001";
const other = "00000000-0000-4000-8000-000000000002";
const key = "00000000-0000-4000-8000-000000000003";
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const values = new Map<string, string>();
const storage = { keys: () => [...values.keys()], getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => { values.set(key, value); }, removeItem: (key: string) => { values.delete(key); } };
const client = new QueryClient();
afterEach(() => { client.clear(); values.clear(); vi.unstubAllGlobals(); });
const json = (data: unknown) => new Response(JSON.stringify(data));
const receipt = { status: 201, code: "comment_created", protocol_version: 2, issue_id: actor, comment_id: other, comment_revision: "1", event_id: key, event_seq: "1", targets: [] };

it("restricts mention insertion to the issue projection and escapes authored labels", () => {
  const source = parseCollaborationMentionTargets({ protocol_version: 2, issue_id: actor, participants: [{ user_id: actor, name: "Name", avatar_url: "", external: true }], can_submit_commands: false, targets: [] })!;
  expect(collaborationMentionMarkdown("member", actor, "Name](bad)", source)).toBe(`[@Name (bad)](mention://member/${actor})`);
  expect(() => collaborationMentionMarkdown("member", other, "Outside", source)).toThrow();
  expect(() => collaborationMentionMarkdown("agent", actor, "Outside", source)).toThrow();
});

it("saves immutable preview authority before comment send and recovers by GET without reposting", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(actor);
  const preview = parseCollaborationTriggerPreview({ protocol_version: 2, issue_id: actor, lifecycle_epoch: "9007199254740993", write_context: { participant_epoch: "2", account_epoch: "3" }, targets: [], recipients: [] })!;
  const draft = prepareCollaborationComment(scope, actor, key, "Original comment", [], preview);
  const fetch = vi.fn().mockImplementationOnce(async () => { expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]); throw new TypeError("offline"); }).mockResolvedValueOnce(json(receipt));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationComment(client, scope, draft, false, storage)).rejects.toThrow();
  preview.lifecycle_epoch = "1";
  await expect(executeCollaborationComment(client, scope, draft, true, storage)).resolves.toEqual(receipt);
  expect(fetch.mock.calls[0]?.[1].method).toBe("POST");
  expect(fetch.mock.calls[1]?.[0]).toContain(`/comments/operations/${key}`);
  expect(fetch.mock.calls[1]?.[1].method ?? "GET").toBe("GET");
  expect(JSON.parse(fetch.mock.calls[0]?.[1].body).expected_lifecycle_epoch).toBe("9007199254740993");
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
});

it("makes malformed or wrong-issue comment recovery unavailable", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json({ ...receipt, issue_id: other })).mockResolvedValueOnce(json({ ...receipt, comment_revision: 1 })));
  const api = new ApiClient("https://server.example");
  await expect(api.getCollaborationCommentOperation(actor, key)).resolves.toBeNull();
  await expect(api.getCollaborationCommentOperation(actor, key)).resolves.toBeNull();
});

it("keeps an attachment comment pending until its receipt confirms the original file selection", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(actor);
  const preview = parseCollaborationTriggerPreview({ protocol_version: 2, issue_id: actor, lifecycle_epoch: "1", targets: [], recipients: [] })!;
  const draft = prepareCollaborationComment(scope, actor, key, "Deliver this file", [], preview, { draftId: other, attachmentIds: [other] });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json(receipt)).mockResolvedValueOnce(json({ ...receipt, attachment_ids: [key] }))
    .mockResolvedValueOnce(json({ ...receipt, attachment_ids: [other] })));
  await expect(executeCollaborationComment(client, scope, draft, false, storage)).rejects.toThrow();
  await expect(executeCollaborationComment(client, scope, draft, true, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  await expect(executeCollaborationComment(client, scope, draft, true, storage)).resolves.toMatchObject({ attachment_ids: [other] });
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
});

it("keeps the original comment after a missing operation without revoking issue access", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(actor);
  const preview = parseCollaborationTriggerPreview({ protocol_version: 2, issue_id: actor, lifecycle_epoch: "1", targets: [], recipients: [] })!;
  const draft = prepareCollaborationComment(scope, actor, key, "Not yet confirmed", [], preview);
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("offline"))
    .mockResolvedValueOnce(new Response(JSON.stringify({ code: "not_found" }), { status: 404 }));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationComment(client, scope, draft, false, storage)).rejects.toThrow();
  await expect(executeCollaborationComment(client, scope, draft, true, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  expect(collaborationIssueAccessDenied(client, scope, actor)).toBe(false);
  expect(fetch.mock.calls).toHaveLength(2);
});

it.each([{ attachmentIds: [] }, { attachmentIds: [other] }])("retires only the submitted comment bundle after confirmed delivery ($attachmentIds)", async ({ attachmentIds }) => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(actor);
  const deliveryScope = { ...scope, context: { kind: "issue" as const, issueId: actor, sourceWorkspaceId: other } };
  const form = { ...newCollaborationDeliveryForm(deliveryScope, "comment"), content: "Saved comment", selectedAttachmentIds: attachmentIds };
  const otherForm = newCollaborationDeliveryForm(deliveryScope, "attachment");
  saveCollaborationDeliveryRecord(form, storage);
  saveCollaborationDeliveryRecord(otherForm, storage);
  const preview = parseCollaborationTriggerPreview({ protocol_version: 2, issue_id: actor, lifecycle_epoch: "1", targets: [], recipients: [] })!;
  const draft = prepareCollaborationComment(scope, actor, key, form.content, [], preview, { draftId: form.draftId, attachmentIds });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ ...receipt, attachment_ids: attachmentIds })));

  await executeCollaborationComment(client, scope, draft, false, storage);

  expect(readCollaborationDeliveryRecords(scope, storage)).toEqual([otherForm]);
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
});

it("clears the protected issue when comment recovery receives a fresh denial", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(actor);
  const preview = parseCollaborationTriggerPreview({ protocol_version: 2, issue_id: actor, lifecycle_epoch: "1", targets: [], recipients: [] })!;
  const draft = prepareCollaborationComment(scope, actor, key, "Private pending comment", [], preview);
  const queryKey = collaborationKeys.issue(scope, actor);
  client.setQueryData(queryKey, { description: "Private cached content" });
  const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ code: "forbidden" }), { status: 403 }));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationComment(client, scope, draft, true, storage)).rejects.toMatchObject({ status: 403 });
  expect(client.getQueryData(queryKey)).toBeUndefined();
  expect(collaborationIssueAccessDenied(client, scope, actor)).toBe(true);
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(fetch.mock.calls.map(([, init]) => init.method ?? "GET")).toEqual(["GET"]);
});
