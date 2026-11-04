// @vitest-environment node
import { QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient, setApiInstance } from "../api";
import { collaborationIssueAccessDenied, collaborationKeys, collaborationScope, clearCollaborationAccess } from "./access";
import { executeCollaborationAction, prepareCollaborationAction } from "./operation";
import { parseCollaborationIssue } from "./schema";
import { readCollaborationDrafts } from "./draft";
import { newCollaborationDeliveryForm, readCollaborationDeliveryRecords, saveCollaborationDeliveryRecord } from "./delivery-draft";
import { collaborationIssueFixture } from "./test-fixtures";

const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
const id = (n: number) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const stored = new Map<string, string>();
const storage = { keys: () => [...stored.keys()], getItem: (key: string) => stored.get(key) ?? null,
  setItem: (key: string, value: string) => { stored.set(key, value); }, removeItem: (key: string) => { stored.delete(key); } };
const client = new QueryClient();
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
function setup(files: string[] = []) {
  setApiInstance(new ApiClient("https://example.test"));
  const scope = collaborationScope(auth.user.id);
  const issue = parseCollaborationIssue({ ...collaborationIssueFixture, write_context: { participant_epoch: "1", account_epoch: "1" } })!;
  const deliveryScope = { server: scope.server, userId: scope.userId, context: { kind: "issue" as const, issueId: issue.id, sourceWorkspaceId: issue.source_workspace.id } };
  const resultForm = { ...newCollaborationDeliveryForm(deliveryScope, "result"), createdAt: 1, content: "Original result", basis: "Original checks", selectedAttachmentIds: files };
  const commentForm = { ...newCollaborationDeliveryForm(deliveryScope, "comment"), createdAt: 2, content: "Concurrent discussion" };
  saveCollaborationDeliveryRecord(resultForm, storage);
  saveCollaborationDeliveryRecord(commentForm, storage);
  const draft = prepareCollaborationAction(scope, issue, id(4), { action: "submit_acceptance", note: resultForm.content, basis: resultForm.basis!,
    expectedStatus: "review", statusOptionsVersion: "a".repeat(64), attachmentIds: files, deliveryDraftId: resultForm.draftId });
  return { scope, issue, draft, resultForm, commentForm };
}
afterEach(() => { stored.clear(); client.clear(); vi.unstubAllGlobals(); });

describe("formal result original intent and bundle retirement", () => {
  it("confirms an exact pending submission's retained evidence without binding those files again", async () => {
    const { scope, issue, resultForm } = setup();
    const submission = { id: id(3), issue_id: issue.id, event_seq: "1", lifecycle_epoch: "1", author_user_id: id(9), author: null,
      created_at: "2026-11-03T00:00:00Z", result: "External delivery", basis: "External checks", attachment_ids: [id(8)],
      source_comment_id: null, evidence_state: "available", decision: null };
    const pending = parseCollaborationIssue({ ...issue, execution: undefined, status: "review", status_category: "in_review", delivery: { current: submission, latest: submission, has_history: true } })!;
    const draft = prepareCollaborationAction(scope, pending, id(4), { action: "complete", submissionId: submission.id, status: "done", statusOptionsVersion: "a".repeat(64) });
    const fetch = vi.fn().mockResolvedValue(json({ code: "completed", collaboration_revision: "2", delivery_confirmation: { submission_id: submission.id, attachment_ids: [id(8)] } }));
    vi.stubGlobal("fetch", fetch);
    await expect(executeCollaborationAction(client, scope, draft, false, storage)).resolves.toMatchObject({ code: "completed" });
    const body = JSON.parse(fetch.mock.calls[0]?.[1]?.body);
    expect(body.submission_id).toBe(submission.id);
    expect(body).not.toHaveProperty("attachment_ids");
    expect(readCollaborationDeliveryRecords(resultForm.scope, storage)).toContainEqual(resultForm);
  });
  it("retires exactly the result bundle after correlated text-only success, preserving discussion", async () => {
    const { scope, draft, resultForm, commentForm } = setup();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code: "acceptance_submitted", collaboration_revision: "2", delivery_confirmation: { submission_id: id(3), attachment_ids: [] } })));
    await executeCollaborationAction(client, scope, draft, false, storage);
    expect(readCollaborationDeliveryRecords(resultForm.scope, storage)).toEqual([commentForm]);
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  });

  it("retires the same result bundle when its valid UUID observation uses uppercase", async () => {
    const { scope, issue, resultForm, commentForm } = setup();
    const draftId = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
    const form = { ...resultForm, draftId };
    stored.clear();
    saveCollaborationDeliveryRecord(form, storage);
    saveCollaborationDeliveryRecord(commentForm, storage);
    const draft = prepareCollaborationAction(scope, issue, id(4), { action: "submit_acceptance", note: form.content, basis: form.basis!,
      expectedStatus: "review", statusOptionsVersion: "a".repeat(64), deliveryDraftId: draftId.toUpperCase() });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code: "acceptance_submitted", collaboration_revision: "2",
      delivery_confirmation: { submission_id: id(3), attachment_ids: [] } })));
    await executeCollaborationAction(client, scope, draft, false, storage);
    expect(readCollaborationDeliveryRecords(form.scope, storage)).toEqual([commentForm]);
  });

  it.each([
    undefined,
    { submission_id: id(3), attachment_ids: [] },
    { submission_id: id(3), attachment_ids: [id(9)] },
    { submission_id: id(3), attachment_ids: [id(8)], result: "Unexpected private prose" },
  ])("preserves an uncertain result on absent/malformed/mismatched file confirmation", async (confirmation) => {
    const { scope, draft, resultForm, commentForm } = setup([id(8)]);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code: "acceptance_submitted", collaboration_revision: "2", delivery_confirmation: confirmation })));
    await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
    expect(readCollaborationDeliveryRecords(resultForm.scope, storage)).toEqual([resultForm, commentForm]);
  });

  it("recovers by the original GET and files without a replacement write", async () => {
    const { scope, draft, resultForm } = setup([id(8)]);
    const fetch = vi.fn().mockRejectedValueOnce(new TypeError("Disconnected"))
      .mockResolvedValueOnce(json({ code: "acceptance_submitted", collaboration_revision: "2", delivery_confirmation: { submission_id: id(3), attachment_ids: [id(8)] } }));
    vi.stubGlobal("fetch", fetch);
    await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
    await executeCollaborationAction(client, scope, draft, true, storage);
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(fetch.mock.calls[1]?.[0]).toContain(`/operations/${draft.requestId}`);
    expect(fetch.mock.calls[1]?.[1]?.method ?? "GET").toBe("GET");
    expect(readCollaborationDeliveryRecords(resultForm.scope, storage).some((form) => form.draftId === resultForm.draftId)).toBe(false);
  });

  it.each([
    { code: "basis_required", status: 400 }, { code: "submission_changed", status: 409 },
    { code: "acceptance_pending", status: 409 }, { code: "task_manual", status: 409 },
  ])("settles only the durable $code refusal while retaining unsent result text/files", async ({ code, status }) => {
    const { scope, draft, resultForm, commentForm } = setup([id(8)]);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code, collaboration_revision: "2" }, status)));
    expect(await executeCollaborationAction(client, scope, draft, false, storage)).toEqual({ code, status, collaboration_revision: "2" });
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
    expect(readCollaborationDeliveryRecords(resultForm.scope, storage)).toEqual([resultForm, commentForm]);
  });

  it.each([
    { code: "forbidden", status: 403 }, { code: "attachment_not_found", status: 404 }, { code: "comment_not_found", status: 404 },
  ])("settles the committed $code POST receipt without treating it as issue-access loss", async ({ code, status }) => {
    const { scope, issue, draft, resultForm, commentForm } = setup([id(8)]);
    const issueKey = collaborationKeys.issue(scope, issue.id);
    client.setQueryData(issueKey, issue);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code, error: code, status, collaboration_revision: "2" }, status)));
    await expect(executeCollaborationAction(client, scope, draft, false, storage)).resolves.toEqual({ code, status, collaboration_revision: "2" });
    expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(false);
    expect(client.getQueryData(issueKey)).toEqual(issue);
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
    expect(readCollaborationDeliveryRecords(resultForm.scope, storage)).toEqual([resultForm, commentForm]);
  });

  it("settles a content-free statusless original refusal returned by GET 200", async () => {
    const { scope, draft, resultForm } = setup();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code: "basis_required", collaboration_revision: "2" })));
    expect(await executeCollaborationAction(client, scope, draft, true, storage)).toMatchObject({ code: "basis_required", status: 400 });
    expect(readCollaborationDeliveryRecords(resultForm.scope, storage)).toContainEqual(resultForm);
  });

  it.each([
    { code: "basis_required", http: 409, status: undefined },
    { code: "submission_changed", http: 500, status: undefined },
    { code: "basis_required", http: 400, status: 409 },
    { code: "made_up", http: 400, status: undefined },
    { code: "made_up", http: 500, status: 500 },
    { code: "acceptance_submitted", http: 200, status: 302 },
  ])("retains unknown intent for contradictory/unrecognized refusal tuples", async ({ code, http, status }) => {
    const { scope, draft } = setup();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code, status, collaboration_revision: "2" }, http)));
    await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  });

  it.each([
    { code: "forbidden", status: 403, http: 403, revision: "2", confirmation: { submission_id: id(3), attachment_ids: [] } },
    { code: "attachment_not_found", status: 403, http: 404, revision: "2", confirmation: undefined },
    { code: "comment_not_found", status: 404, http: 404, revision: 2, confirmation: undefined },
    { code: "made_up", status: 403, http: 403, revision: "2", confirmation: undefined },
  ])("preserves unknown declared POST receipt intent without inferring issue denial", async ({ code, status, http, revision, confirmation }) => {
    const { scope, issue, draft, resultForm, commentForm } = setup();
    const issueKey = collaborationKeys.issue(scope, issue.id);
    client.setQueryData(issueKey, issue);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code, status, collaboration_revision: revision, delivery_confirmation: confirmation }, http)));
    await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
    expect(collaborationIssueAccessDenied(client, scope, issue.id)).toBe(false);
    expect(client.getQueryData(issueKey)).toEqual(issue);
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
    expect(readCollaborationDeliveryRecords(resultForm.scope, storage)).toEqual([resultForm, commentForm]);
  });

  it("keeps a refusal that contradictorily carries delivery confirmation uncertain", async () => {
    const { scope, draft } = setup();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code: "basis_required", status: 400, collaboration_revision: "2",
      delivery_confirmation: { submission_id: id(3), attachment_ids: [] } }, 400)));
    await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  });

  it.each([100, 199])("keeps an informational body status %s from confirming a formal result", async (status) => {
    const { scope, draft, resultForm, commentForm } = setup();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code: "acceptance_submitted", status, collaboration_revision: "2",
      delivery_confirmation: { submission_id: id(3), attachment_ids: [] } })));
    await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
    expect(readCollaborationDeliveryRecords(resultForm.scope, storage)).toEqual([resultForm, commentForm]);
  });

  it("permits only content-free own confirmation recovery after known issue-access loss", async () => {
    const { scope, issue, draft } = setup([id(8)]);
    clearCollaborationAccess(client, scope, issue.id);
    const fetch = vi.fn().mockResolvedValue(json({ code: "acceptance_submitted", collaboration_revision: "2", delivery_confirmation: { submission_id: id(3), attachment_ids: [id(8)] } }));
    vi.stubGlobal("fetch", fetch);
    await expect(executeCollaborationAction(client, scope, draft, true, storage)).resolves.toMatchObject({ code: "acceptance_submitted" });
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch.mock.calls[0]?.[0]).toContain(`/operations/${draft.requestId}`);
    expect(fetch.mock.calls[0]?.[1]?.method ?? "GET").toBe("GET");
  });
});
