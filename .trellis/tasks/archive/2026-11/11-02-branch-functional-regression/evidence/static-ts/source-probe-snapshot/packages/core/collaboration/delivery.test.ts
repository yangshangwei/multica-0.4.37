// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationIssueFixture } from "./test-fixtures";
import { collaborationScope, collaborationKeys, clearCollaborationAccess, clearCollaborationProjection } from "./access";
import { executeCollaborationCreation, prepareCollaborationCreation } from "./creation";
import { executeCollaborationUpload, loadCollaborationAttachmentPreview, prepareCollaborationUpload, stageCollaborationUpload } from "./attachment";
import { clearCollaborationCreationDrafts, clearCollaborationDeliveryDrafts, newCollaborationDeliveryForm,
  readCollaborationDeliveryRecords, saveCollaborationDeliveryRecord, retainCollaborationDeliveryScope } from "./delivery-draft";
import { clearCollaborationSourceAccess } from "./delivery-access";

const actor = "00000000-0000-4000-8000-000000000001";
const source = "00000000-0000-4000-8000-000000000002";
const requestId = "00000000-0000-4000-8000-000000000003";
const fileId = "00000000-0000-4000-8000-000000000004";
const user = vi.hoisted(() => ({ id: "00000000-0000-4000-8000-000000000001" }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user }) } }));
const values = new Map<string, string>();
const storage = { keys: () => [...values.keys()], getItem: (key: string) => values.get(key) ?? null,
  setItem: (key: string, value: string) => { values.set(key, value); }, removeItem: (key: string) => { values.delete(key); } };
const client = new QueryClient();
const json = (data: unknown, status = 200) => new Response(JSON.stringify(data), { status });
afterEach(() => { client.clear(); values.clear(); vi.unstubAllGlobals(); user.id = actor; });
function setup() {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(actor);
  const form = { ...newCollaborationDeliveryForm({ ...scope, context: { kind: "creation", sourceWorkspaceId: source } }, "creation"), title: "Original title", content: "Private original body" };
  saveCollaborationDeliveryRecord(form, storage);
  return { scope, form };
}
const created = { ...collaborationIssueFixture, source_workspace: { ...collaborationIssueFixture.source_workspace, id: source } };

it("persists source-scoped intent before POST and recovers a lost creation with the original key", async () => {
  const { scope, form } = setup();
  const draft = prepareCollaborationCreation(form, requestId);
  const fetch = vi.fn().mockImplementationOnce(async () => {
    expect(readCollaborationDeliveryRecords(form.scope, storage)).toContainEqual(draft);
    throw new TypeError("connection lost after commit");
  }).mockResolvedValueOnce(json(created));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationCreation(client, scope, draft, false, storage)).rejects.toThrow();
  expect(draft.scope.context).toEqual({ kind: "creation", sourceWorkspaceId: source });
  expect(draft).not.toHaveProperty("scope.issueId");
  const persisted = readCollaborationDeliveryRecords(scope, storage).find((record) => record.kind === "creation")!;
  expect(persisted.kind).toBe("creation");
  await expect(executeCollaborationCreation(client, scope, draft, true, storage)).resolves.toMatchObject({ id: actor });
  expect(fetch.mock.calls[1]?.[0]).toContain(`/creations/${requestId}?source_workspace_id=${source}`);
  expect(fetch.mock.calls[1]?.[1].method ?? "GET").toBe("GET");
  expect(readCollaborationDeliveryRecords(scope, storage)).toEqual([]);
});

it("does not replace unknown creation after missing or malformed recovery", async () => {
  const { scope, form } = setup();
  const draft = prepareCollaborationCreation(form, requestId);
  vi.stubGlobal("fetch", vi.fn().mockRejectedValueOnce(new TypeError("offline"))
    .mockResolvedValueOnce(json({ code: "not_found" }, 404)).mockResolvedValueOnce(json({ ...created, id: null })));
  await expect(executeCollaborationCreation(client, scope, draft, false, storage)).rejects.toThrow();
  await expect(executeCollaborationCreation(client, scope, draft, true, storage)).rejects.toThrow();
  await expect(executeCollaborationCreation(client, scope, draft, true, storage)).rejects.toThrow();
  expect(() => saveCollaborationDeliveryRecord(prepareCollaborationCreation(form, crypto.randomUUID()), storage)).toThrow();
  expect(readCollaborationDeliveryRecords(scope, storage)).toContainEqual(draft);
});

it("clears private source drafts but retains content-free creation recovery after membership loss", async () => {
  const { scope, form } = setup();
  const draft = prepareCollaborationCreation(form, requestId);
  saveCollaborationDeliveryRecord(draft, storage);
  clearCollaborationCreationDrafts(scope, source, storage);
  const records = readCollaborationDeliveryRecords(scope, storage);
  expect(records).toEqual([{ ...draft, input: null }]);
  const redacted = { ...draft, input: null };
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json(created)));
  await expect(executeCollaborationCreation(client, scope, redacted, false, storage)).rejects.toThrow();
  await expect(executeCollaborationCreation(client, scope, redacted, true, storage)).resolves.toMatchObject({ id: actor });
});

it("retains the original creation and file selection when only the recipient becomes unavailable", async () => {
  const { scope, form } = setup();
  const selected = { ...form, selectedAttachmentIds: [fileId], recipient: { user_id: fileId, name: "Recipient", avatar_url: "", external: true } };
  saveCollaborationDeliveryRecord(selected, storage);
  const draft = prepareCollaborationCreation(selected, requestId);
  const sourceChoice = { id: source, name: "Source", slug: "source" };
  const fetch = vi.fn().mockResolvedValueOnce(json({ code: "account_unavailable" }, 403))
    .mockResolvedValueOnce(json({ code: "operation_not_found" }, 404))
    .mockResolvedValueOnce(json({ creation_enabled: true, sources: [sourceChoice], source: sourceChoice, max_attachment_bytes: 104857600 }));
  vi.stubGlobal("fetch", fetch);

  await expect(executeCollaborationCreation(client, scope, draft, false, storage)).rejects.toThrow();

  expect(readCollaborationDeliveryRecords(scope, storage)).toContainEqual(selected);
  expect(readCollaborationDeliveryRecords(scope, storage)).toContainEqual(draft);
  expect(fetch.mock.calls[2]?.[0]).toContain(`/creation-options?source_workspace_id=${source}`);
  expect(fetch.mock.calls[2]?.[1].method ?? "GET").toBe("GET");
});

it("clears private creation inputs when the recipient failure recheck also denies the source", async () => {
  const { scope, form } = setup();
  const draft = prepareCollaborationCreation(form, requestId);
  const fetch = vi.fn().mockResolvedValueOnce(json({ code: "account_unavailable" }, 403))
    .mockResolvedValueOnce(json({ code: "operation_not_found" }, 404))
    .mockResolvedValueOnce(json({ code: "forbidden" }, 403));
  vi.stubGlobal("fetch", fetch);

  await expect(executeCollaborationCreation(client, scope, draft, false, storage)).rejects.toThrow();

  expect(fetch).toHaveBeenCalledTimes(3);
  expect(fetch.mock.calls[2]?.[0]).toContain(`/creation-options?source_workspace_id=${source}`);
  expect(readCollaborationDeliveryRecords(scope, storage)).toEqual([{ ...draft, input: null }]);
});

it("fences a delayed creation response after source access or account changes", async () => {
  const { scope, form } = setup();
  let resolve!: (value: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockImplementation(() => new Promise<Response>((done) => { resolve = done; })));
  const pending = executeCollaborationCreation(client, scope, prepareCollaborationCreation(form, requestId), false, storage);
  clearCollaborationSourceAccess(client, scope, source, storage);
  resolve(json(created));
  await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  expect(readCollaborationDeliveryRecords(scope, storage)).toEqual([expect.objectContaining({ kind: "creation", input: null })]);
  user.id = source;
  await expect(executeCollaborationCreation(client, scope, prepareCollaborationCreation(form, requestId), true, storage)).rejects.toMatchObject({ name: "AbortError" });
});

it("recovers an uploaded file by its original key and retains the selected file on reload", async () => {
  const { scope, form } = setup();
  const file = new File(["hello"], "notes.txt", { type: "text/plain" });
  const upload = await prepareCollaborationUpload(form, requestId, file);
  const metadata = { id: fileId, draft_id: form.draftId, issue_id: null, intended_issue_id: null, source_workspace_id: source, uploader_user_id: actor,
    name: file.name, content_type: file.type, size_bytes: 5, version: "1", created_at: "2026-10-18T00:00:00Z", delivered_at: null };
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("response lost")).mockResolvedValueOnce(json(metadata));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationUpload(client, scope, upload, file, storage)).rejects.toThrow();
  expect(readCollaborationDeliveryRecords(scope, storage)).toContainEqual(upload);
  await expect(executeCollaborationUpload(client, scope, upload, new File(["other"], file.name, { type: file.type }), storage)).rejects.toMatchObject({ code: "attachment_file_changed" });
  expect(fetch).toHaveBeenCalledTimes(1);
  await expect(executeCollaborationUpload(client, scope, upload, undefined, storage)).resolves.toEqual(metadata);
  expect(fetch.mock.calls[1]?.[0]).toContain(`/drafts/${form.draftId}/operations/${requestId}`);
  expect(readCollaborationDeliveryRecords(scope, storage)).toEqual([{ ...form, selectedAttachmentIds: [fileId] }]);
});

it("rejects wrong uploader/source upload responses and cannot send without recoverable storage", async () => {
  const { scope, form } = setup();
  const file = new File(["hello"], "notes.txt");
  const upload = await prepareCollaborationUpload(form, requestId, file);
  const api = vi.fn().mockResolvedValue(json({ id: fileId, draft_id: form.draftId, issue_id: null, intended_issue_id: null, source_workspace_id: source, uploader_user_id: source,
    name: file.name, content_type: "text/plain", size_bytes: 5, version: "1", created_at: "2026-10-18T00:00:00Z", delivered_at: null }));
  vi.stubGlobal("fetch", api);
  await expect(executeCollaborationUpload(client, scope, upload, file, { ...storage, keys: undefined })).rejects.toThrow();
  expect(api).not.toHaveBeenCalled();
  await expect(executeCollaborationUpload(client, scope, upload, file, storage)).rejects.toThrow();
  expect(readCollaborationDeliveryRecords(scope, storage)).toContainEqual(upload);
});

it("isolates cleanup between existing issues, creation sources and account identities", () => {
  const { scope, form } = setup();
  const comment = newCollaborationDeliveryForm({ ...scope, context: { kind: "issue", issueId: actor, sourceWorkspaceId: source } }, "comment");
  saveCollaborationDeliveryRecord(comment, storage);
  clearCollaborationDeliveryDrafts(storage, { ...scope, issueId: actor });
  expect(readCollaborationDeliveryRecords(scope, storage)).toEqual([form]);
  retainCollaborationDeliveryScope({ ...scope, userId: source }, storage);
  expect(values.size).toBe(0);
  clearCollaborationAccess(client, scope, actor);
});

it("cancels the upload transport while preserving its original key for recovery", async () => {
  const { scope, form } = setup();
  const file = new File(["hello"], "notes.txt", { type: "text/plain" });
  const draft = await prepareCollaborationUpload(form, requestId, file);
  const metadata = { id: fileId, draft_id: form.draftId, issue_id: null, intended_issue_id: null, source_workspace_id: source, uploader_user_id: actor,
    name: file.name, content_type: file.type, size_bytes: 5, version: "1", created_at: "2026-10-18T00:00:00Z", delivered_at: null };
  let finish!: (response: Response) => void;
  const fetch = vi.fn().mockImplementationOnce(() => new Promise<Response>((resolve) => { finish = resolve; })).mockResolvedValueOnce(json(metadata));
  vi.stubGlobal("fetch", fetch);
  const controller = new AbortController();
  const pending = executeCollaborationUpload(client, scope, draft, file, storage, controller.signal);
  await vi.waitFor(() => expect(fetch).toHaveBeenCalledOnce());
  controller.abort();
  finish(json(metadata));

  await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  expect(fetch.mock.calls[0]?.[1].signal.aborted).toBe(true);
  expect(readCollaborationDeliveryRecords(scope, storage)).toContainEqual(draft);
  await expect(executeCollaborationUpload(client, scope, draft, undefined, storage)).resolves.toEqual(metadata);
  expect(fetch.mock.calls[1]?.[0]).toContain(`/operations/${requestId}`);
  expect(fetch.mock.calls[1]?.[1].method ?? "GET").toBe("GET");
});

it("fences file hashing when the issue projection changes before staging", async () => {
  const { scope } = setup();
  const form = newCollaborationDeliveryForm({ ...scope, context: { kind: "issue", issueId: actor, sourceWorkspaceId: source } }, "comment");
  const file = new File(["hello"], "notes.txt");
  let finish!: (bytes: ArrayBuffer) => void;
  Object.defineProperty(file, "arrayBuffer", { value: () => new Promise<ArrayBuffer>((resolve) => { finish = resolve; }) });
  const pending = stageCollaborationUpload(client, scope, form, requestId, file, storage);
  clearCollaborationProjection(client, scope, actor);
  finish(new TextEncoder().encode("hello").buffer);

  await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  expect(readCollaborationDeliveryRecords(form.scope, storage)).toEqual([]);
});

it("keeps upload intent when an older server has no private upload route", async () => {
  const { scope, form } = setup();
  const file = new File(["hello"], "notes.txt");
  const draft = await prepareCollaborationUpload(form, requestId, file);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("Not Found", { status: 404 })));

  await expect(executeCollaborationUpload(client, scope, draft, file, storage)).rejects.toThrow();

  expect(readCollaborationDeliveryRecords(scope, storage)).toContainEqual(form);
  expect(readCollaborationDeliveryRecords(scope, storage)).toContainEqual(draft);
});

it.each([false, true])("rechecks issue authority after an ambiguous file denial (access lost: %s)", async (denied) => {
  const { scope } = setup();
  const queryKey = collaborationKeys.issue(scope, actor);
  client.setQueryData(queryKey, { description: "Authorized issue content" });
  const file = { id: fileId, draft_id: requestId, issue_id: actor, intended_issue_id: actor, source_workspace_id: source, uploader_user_id: actor,
    name: "notes.txt", content_type: "text/plain", size_bytes: 5, version: "1", created_at: "2026-10-18T00:00:00Z", delivered_at: "2026-10-18T00:00:00Z" };
  const fetch = vi.fn().mockResolvedValueOnce(json({ code: "not_found" }, 404))
    .mockResolvedValueOnce(denied ? json({ code: "not_found" }, 404) : json(created));
  vi.stubGlobal("fetch", fetch);

  await expect(loadCollaborationAttachmentPreview(client, scope, file)).rejects.toThrow();

  expect(fetch.mock.calls[1]?.[0]).toBe(`https://server.example/api/collaboration/issues/${actor}?execution_view_version=6`);
  expect(client.getQueryData(queryKey)).toEqual(denied ? undefined : { description: "Authorized issue content" });
});

it("retires a proven creation rejection and allows explicit same-input resubmission with its files", async () => {
  const { scope, form } = setup();
  const selected = { ...form, selectedAttachmentIds: [fileId], recipient: { user_id: fileId, name: "Unavailable recipient", avatar_url: "", external: true } };
  saveCollaborationDeliveryRecord(selected, storage);
  const draft = prepareCollaborationCreation(selected, requestId);
  const sourceChoice = { id: source, name: "Source", slug: "source" };
  const rejection = { request_id: requestId, source_workspace_id: source, actor_user_id: actor, outcome: "rejected", code: "account_unavailable" };
  const fetch = vi.fn().mockResolvedValueOnce(json({ code: "account_unavailable" }, 403)).mockResolvedValueOnce(json(rejection))
    .mockResolvedValueOnce(json({ creation_enabled: true, sources: [sourceChoice], source: sourceChoice, max_attachment_bytes: 104857600 }))
    .mockResolvedValueOnce(json({ ...created, handler: selected.recipient }, 201));
  vi.stubGlobal("fetch", fetch);

  await expect(executeCollaborationCreation(client, scope, draft, false, storage)).rejects.toMatchObject({ name: "CollaborationCreationRejectedError", code: "account_unavailable" });

  const records = readCollaborationDeliveryRecords(scope, storage);
  const saved = records.find((record) => record.kind === "form")!;
  expect(records).toHaveLength(1);
  expect(saved).toEqual(selected);
  const nextKey = crypto.randomUUID();
  const resubmission = prepareCollaborationCreation(saved, nextKey);
  expect(resubmission.input).toEqual(draft.input);
  expect(fetch.mock.calls.filter(([, init]) => init.method === "POST")).toHaveLength(1);
  await expect(executeCollaborationCreation(client, scope, resubmission, false, storage)).resolves.toMatchObject({ id: actor });
  expect(fetch.mock.calls.filter(([, init]) => init.method === "POST").map(([, init]) => init.headers["Idempotency-Key"])).toEqual([requestId, nextKey]);
  expect(JSON.parse(fetch.mock.calls[3]?.[1].body)).toEqual(draft.input);
});

it("recovers a lost rejection through the original key without another POST", async () => {
  const { scope, form } = setup();
  const draft = prepareCollaborationCreation(form, requestId);
  const sourceChoice = { id: source, name: "Source", slug: "source" };
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("reply lost"))
    .mockResolvedValueOnce(json({ code: "not_found" }, 404))
    .mockResolvedValueOnce(json({ request_id: requestId, source_workspace_id: source, actor_user_id: actor, outcome: "rejected", code: "collaboration_unavailable" }))
    .mockResolvedValueOnce(json({ creation_enabled: false, sources: [sourceChoice], source: sourceChoice, max_attachment_bytes: 104857600 }));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationCreation(client, scope, draft, false, storage)).rejects.toThrow();

  await expect(executeCollaborationCreation(client, scope, draft, true, storage)).rejects.toMatchObject({ name: "CollaborationCreationRejectedError", code: "collaboration_unavailable" });

  expect(fetch.mock.calls.filter(([, init]) => init.method === "POST")).toHaveLength(1);
  expect(fetch.mock.calls[2]?.[0]).toContain(`/creations/${requestId}/result?`);
  expect(readCollaborationDeliveryRecords(scope, storage)).toEqual([form]);
});

it.each(["missing", "wrong actor", "malformed", "created", "incomplete", "unknown commit"])("does not retire an unproven creation rejection (%s)", async (failure) => {
  const { scope, form } = setup();
  const draft = prepareCollaborationCreation(form, requestId);
  const sourceChoice = { id: source, name: "Source", slug: "source" };
  const result = { request_id: requestId, source_workspace_id: source, actor_user_id: source, outcome: "rejected", code: "account_unavailable" };
  const unproven = failure === "missing" ? json({ code: "operation_not_found" }, 404)
    : failure === "incomplete" ? json({ code: "operation_incomplete" }, 409)
    : failure === "unknown commit" ? json({ code: "collaboration_unavailable" }, 503)
    : json(failure === "wrong actor" ? result : failure === "created" ? { ...result, actor_user_id: actor, outcome: "created", code: "created" } : { ...result, outcome: undefined });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json({ code: "account_unavailable" }, 403)).mockResolvedValueOnce(unproven)
    .mockResolvedValueOnce(json({ creation_enabled: true, sources: [sourceChoice], source: sourceChoice, max_attachment_bytes: 104857600 })));

  await expect(executeCollaborationCreation(client, scope, draft, false, storage)).rejects.not.toMatchObject({ name: "CollaborationCreationRejectedError" });

  expect(readCollaborationDeliveryRecords(scope, storage)).toContainEqual(form);
  expect(readCollaborationDeliveryRecords(scope, storage)).toContainEqual(draft);
});

it("clears private inputs after source loss even when a rejected creation is already proven", async () => {
  const { scope, form } = setup();
  const draft = prepareCollaborationCreation(form, requestId);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json({ code: "account_unavailable" }, 403))
    .mockResolvedValueOnce(json({ request_id: requestId, source_workspace_id: source, actor_user_id: actor, outcome: "rejected", code: "account_unavailable" }))
    .mockResolvedValueOnce(json({ code: "forbidden" }, 403)));

  await expect(executeCollaborationCreation(client, scope, draft, false, storage)).rejects.toMatchObject({ status: 403 });

  expect(readCollaborationDeliveryRecords(scope, storage)).toEqual([]);
});
