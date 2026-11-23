// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "../api";
import { collaborationIssueFixture } from "./test-fixtures";

const actor = "00000000-0000-4000-8000-000000000001";
const source = "00000000-0000-4000-8000-000000000002";
const draft = "00000000-0000-4000-8000-000000000003";
const fileId = "00000000-0000-4000-8000-000000000004";
const requestId = "00000000-0000-4000-8000-000000000005";
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
const metadata = { id: fileId, draft_id: draft, issue_id: null, intended_issue_id: null, source_workspace_id: source, uploader_user_id: actor,
  name: "notes.txt", content_type: "text/plain; charset=utf-8", size_bytes: 5, version: "1", created_at: "2026-10-18T00:00:00Z", delivered_at: null };
afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks(); });

it("uses an account-scoped source picker and rejects malformed or wrong-source options", async () => {
  const options = { creation_enabled: false, sources: [{ id: source, name: "Source", slug: "source" }], source: null, max_attachment_bytes: 104857600 };
  const fetch = vi.fn().mockResolvedValueOnce(json(options)).mockResolvedValueOnce(json({ ...options, creation_enabled: "true" }))
    .mockResolvedValueOnce(json({ ...options, source: options.sources[0] }));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://server.example");
  await expect(api.getCollaborationCreationOptions()).resolves.toEqual(options);
  await expect(api.getCollaborationCreationOptions()).resolves.toBeNull();
  await expect(api.getCollaborationCreationOptions(actor)).resolves.toBeNull();
  expect(fetch.mock.calls[0]?.[0]).toBe("https://server.example/api/collaboration/creation-options");
});

it("uploads a private multipart draft and keeps the original key without a JSON content type", async () => {
  const fetch = vi.fn().mockResolvedValue(json(metadata));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://server.example");
  await expect(api.uploadCollaborationAttachment({ draft_id: draft, source_workspace_id: source }, requestId,
    new File(["hello"], "notes.txt", { type: "text/plain" }))).resolves.toEqual(metadata);
  const [url, init] = fetch.mock.calls[0]!;
  expect(url).toBe("https://server.example/api/collaboration/attachments/drafts");
  expect(init.headers["Idempotency-Key"]).toBe(requestId);
  expect(init.headers["Content-Type"]).toBeUndefined();
  expect(init.body).toBeInstanceOf(FormData);
  expect(init.body.get("source_workspace_id")).toBe(source);
  expect(init.body.has("issue_id")).toBe(false);
});

it("requires explicit pre-issue intention and binds existing-issue upload responses to that exact issue", async () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://server.example");
  const file = new File(["hello"], "notes.txt", { type: "text/plain" });
  for (const intended_issue_id of [undefined, "invalid-uuid", actor]) {
    fetch.mockResolvedValueOnce(json({ ...metadata, intended_issue_id }));
    await expect(api.uploadCollaborationAttachment({ draft_id: draft, source_workspace_id: source }, requestId, file)).resolves.toBeNull();
  }
  for (const intended_issue_id of [undefined, null, source]) {
    fetch.mockResolvedValueOnce(json({ ...metadata, intended_issue_id }));
    await expect(api.uploadCollaborationAttachment({ draft_id: draft, issue_id: actor }, requestId, file)).resolves.toBeNull();
  }
  fetch.mockResolvedValueOnce(json({ ...metadata, intended_issue_id: actor }));
  await expect(api.uploadCollaborationAttachment({ draft_id: draft, issue_id: actor }, requestId, file)).resolves.toMatchObject({ intended_issue_id: actor, issue_id: null });
  fetch.mockResolvedValueOnce(json({ ...metadata, issue_id: actor, delivered_at: metadata.created_at }));
  await expect(api.getCollaborationAttachment(actor, fileId)).resolves.toMatchObject({ intended_issue_id: null, issue_id: actor });
});

it("creates the original source issue and refuses a wrong-source response", async () => {
  const issue = { ...collaborationIssueFixture, source_workspace: { ...collaborationIssueFixture.source_workspace, id: source } };
  const fetch = vi.fn().mockResolvedValueOnce(json(issue, 201)).mockResolvedValueOnce(json(collaborationIssueFixture, 201));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://server.example");
  const input = { source_workspace_id: source, title: "Original title", description: "Original body", handler_user_id: actor,
    draft_id: draft, attachment_ids: [fileId] };
  expect(await api.createCollaborationIssue(requestId, input)).toMatchObject({ id: actor, source_workspace: { id: source } });
  await expect(api.createCollaborationIssue(requestId, input)).resolves.toBeNull();
  expect(fetch.mock.calls[0]?.[1].headers["Idempotency-Key"]).toBe(requestId);
  expect(JSON.parse(fetch.mock.calls[0]?.[1].body)).toEqual(input);
});

it("accepts only a complete creation result correlated to the requested source and key", async () => {
  const result = { request_id: requestId, source_workspace_id: source, actor_user_id: actor, outcome: "rejected", code: "account_unavailable" };
  const fetch = vi.fn().mockResolvedValueOnce(json(result));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://server.example");
  await expect(api.getCollaborationCreationResult(source, requestId)).resolves.toEqual(result);
  expect(fetch.mock.calls[0]?.[0]).toBe(`https://server.example/api/collaboration/creations/${requestId}/result?source_workspace_id=${source}`);
  for (const invalid of [
    { ...result, request_id: draft }, { ...result, source_workspace_id: actor }, { ...result, actor_user_id: undefined },
    { ...result, outcome: "pending" }, { ...result, outcome: "created" }, { ...result, code: "created" }, { ...result, code: "future_rejection" },
    { ...result, recipient: { name: "Unexpected private data" } },
  ]) {
    fetch.mockResolvedValueOnce(json(invalid));
    await expect(api.getCollaborationCreationResult(source, requestId)).resolves.toBeNull();
  }
  fetch.mockResolvedValueOnce(json({ ...result, outcome: "created", code: "created" }));
  await expect(api.getCollaborationCreationResult(source, requestId)).resolves.toMatchObject({ outcome: "created", code: "created" });
  fetch.mockResolvedValueOnce(json(result, 202));
  await expect(api.getCollaborationCreationResult(source, requestId)).resolves.toBeNull();
  fetch.mockResolvedValueOnce(json({ code: "operation_not_found" }, 404));
  await expect(api.getCollaborationCreationResult(source, requestId)).resolves.toBeNull();
  fetch.mockResolvedValueOnce(json({ code: "operation_incomplete" }, 409));
  await expect(api.getCollaborationCreationResult(source, requestId)).rejects.toMatchObject({ status: 409 });
});

it("accepts only same-origin exact-file native capabilities and never an ordinary/public URL", async () => {
  const path = `/api/collaboration/issues/${actor}/attachments/${fileId}/capability?cap=opaque-token`;
  const capability = { issue_id: actor, attachment_id: fileId, version: "1", url: path, expires_at: new Date(Date.now() + 30_000).toISOString() };
  const fetch = vi.fn().mockResolvedValueOnce(json(capability));
  vi.stubGlobal("fetch", fetch);
  const api = new ApiClient("https://server.example");
  expect(await api.getCollaborationAttachmentCapability(actor, fileId, "1", "download")).toMatchObject({ url: `https://server.example${path}` });
  for (const url of [`https://outside.example${path}`, `/uploads/${fileId}`, `/api/attachments/${fileId}/signed-download?cap=x`,
    path.replace(fileId, source), `${path}&cap=second`, `${path}#fragment`, `https://user:password@server.example${path}`]) {
    fetch.mockResolvedValueOnce(json({ ...capability, url }));
    await expect(api.getCollaborationAttachmentCapability(actor, fileId, "1", "download")).resolves.toBeNull();
  }
});
