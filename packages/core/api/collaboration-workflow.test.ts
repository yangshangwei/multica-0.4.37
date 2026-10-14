// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "./client";
import { setSchemaLogger } from "./schema";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { noopLogger } from "../logger";

const issueId = "00000000-0000-4000-8000-000000000001";
const targetUserId = "00000000-0000-4000-8000-000000000002";
const input = {
  requestId: "original-request",
  expectedCollaborationRevision: "9007199254740993",
  expectedLifecycleEpoch: "9007199254740994",
  expectedParticipantEpoch: "9007199254740995",
  expectedAccountEpoch: "9223372036854775807",
};
const expectedVersions = {
  expected_collaboration_revision: input.expectedCollaborationRevision,
  expected_lifecycle_epoch: input.expectedLifecycleEpoch,
  expected_participant_epoch: input.expectedParticipantEpoch,
  expected_account_epoch: input.expectedAccountEpoch,
};
const receipt = { code: "transferred", collaboration_revision: "9007199254740996" };
const statusOptionsVersion = "a".repeat(64);
const mutations = [
  {
    name: "add participant", route: "participants",
    invoke: (client: ApiClient, signal?: AbortSignal) => client.addCollaborationParticipant(issueId, { ...input, targetUserId }, { signal }),
    body: { ...expectedVersions, target_user_id: targetUserId },
  },
  {
    name: "handoff", route: "handoffs",
    invoke: (client: ApiClient, signal?: AbortSignal) => client.createCollaborationHandoff(issueId, { ...input, targetUserId, purpose: "transfer", note: "PRIVATE_HANDOFF_NOTE" }, { signal }),
    body: { ...expectedVersions, target_user_id: targetUserId, purpose: "transfer", note: "PRIVATE_HANDOFF_NOTE" },
  },
  {
    name: "information request", route: "handoffs",
    invoke: (client: ApiClient, signal?: AbortSignal) => client.createCollaborationHandoff(issueId, { ...input, targetUserId, purpose: "information", expectedStatus: "blocked", statusOptionsVersion, note: "PRIVATE_QUESTION" }, { signal }),
    body: { ...expectedVersions, target_user_id: targetUserId, purpose: "information", expected_status: "blocked", status_options_version: statusOptionsVersion, note: "PRIVATE_QUESTION" },
  },
  {
    name: "information return", route: "information-returns",
    invoke: (client: ApiClient, signal?: AbortSignal) => client.returnCollaborationInformation(issueId, { ...input, informationRequestId: targetUserId, expectedStatus: "in_progress", statusOptionsVersion, note: "PRIVATE_ANSWER" }, { signal }),
    body: { ...expectedVersions, information_request_id: targetUserId, expected_status: "in_progress", status_options_version: statusOptionsVersion, note: "PRIVATE_ANSWER" },
  },
  {
    name: "acceptance submission", route: "acceptance-submissions",
    invoke: (client: ApiClient, signal?: AbortSignal) => client.submitCollaborationAcceptance(issueId, { ...input, expectedStatus: "in_review", statusOptionsVersion, note: "PRIVATE_RESULT" }, { signal }),
    body: { ...expectedVersions, expected_status: "in_review", status_options_version: statusOptionsVersion, note: "PRIVATE_RESULT" },
  },
  {
    name: "remove participant", route: `participants/${targetUserId}/remove`,
    invoke: (client: ApiClient, signal?: AbortSignal) => client.removeCollaborationParticipant(issueId, targetUserId, input, { signal }),
    body: expectedVersions,
  },
  {
    name: "leave", route: "leave",
    invoke: (client: ApiClient, signal?: AbortSignal) => client.leaveCollaborationIssue(issueId, input, { signal }),
    body: expectedVersions,
  },
  {
    name: "issue edit", route: "", method: "PATCH",
    invoke: (client: ApiClient, signal?: AbortSignal) => client.editCollaborationIssue(issueId, { ...input, title: "PRIVATE_TITLE", description: "PRIVATE_BODY", startDate: "2026-10-14", dueDate: null }, { signal }),
    body: { ...expectedVersions, title: "PRIVATE_TITLE", description: "PRIVATE_BODY", start_date: "2026-10-14", due_date: null },
  },
];
const operations = [
  ...mutations,
  {
    name: "operation receipt",
    invoke: (client: ApiClient, signal?: AbortSignal) => client.getCollaborationOperation(issueId, input.requestId, { signal }),
  },
];

function jsonResponse(data: unknown, status = 200) {
  return new Response(JSON.stringify(data), { status, headers: { "Content-Type": "application/json" } });
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

afterEach(() => {
  vi.unstubAllGlobals();
  setSchemaLogger(noopLogger);
  setCurrentWorkspace(null, null);
});

it.each(mutations)("sends $name with the original key and string versions in account scope", async ({ invoke, route, body, method }) => {
  setCurrentWorkspace("unrelated-workspace", targetUserId);
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse(receipt));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  client.setToken("test-only-token");
  const controller = new AbortController();
  await expect(invoke(client, controller.signal)).resolves.toEqual(receipt);
  expect(fetchMock).toHaveBeenCalledOnce();
  const [url, init] = fetchMock.mock.calls[0]!;
  expect(url).toBe(`https://api.example.test/api/collaboration/issues/${issueId}${route ? `/${route}` : ""}`);
  expect(init.method).toBe(method ?? "POST");
  expect(JSON.parse(init.body)).toEqual(body);
  const headers = new Headers(init.headers);
  expect(headers.get("Idempotency-Key")).toBe(input.requestId);
  expect(headers.get("Authorization")).toBe("Bearer test-only-token");
  expect(headers.get("Content-Type")).toBe("application/json");
  expect(headers.has("X-Workspace-ID")).toBe(false);
  expect(headers.has("X-Workspace-Slug")).toBe(false);
  controller.abort();
  expect(init.signal.aborted).toBe(true);
});

it.each(["participant_left", "forbidden", "stale_revision"])("reads the original %s receipt after leaving without a write context", async (code) => {
  setCurrentWorkspace("unrelated-workspace", targetUserId);
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ ...receipt, code }));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  client.setToken("test-only-token");
  await expect(client.getCollaborationOperation(issueId, "original/request?key")).resolves.toEqual({ ...receipt, code });
  expect(fetchMock).toHaveBeenCalledOnce();
  const [url, init] = fetchMock.mock.calls[0]!;
  expect(url).toBe(`https://api.example.test/api/collaboration/issues/${issueId}/operations/original%2Frequest%3Fkey`);
  expect(init.method ?? "GET").toBe("GET");
  expect(init.body).toBeUndefined();
  const headers = new Headers(init.headers);
  expect(headers.has("Idempotency-Key")).toBe(false);
  expect(headers.has("X-Workspace-ID")).toBe(false);
  expect(headers.has("X-Workspace-Slug")).toBe(false);
  expect(headers.get("Authorization")).toBe("Bearer test-only-token");
});

it("keeps the same key and payload when retrying an unknown then refused operation", async () => {
  const fetchMock = vi.fn()
    .mockRejectedValueOnce(new TypeError("Network unavailable"))
    .mockResolvedValueOnce(jsonResponse({ error: "Changed", code: "stale_revision", collaboration_revision: "9007199254740999" }, 409))
    .mockResolvedValueOnce(jsonResponse({ error: "Changed", code: "stale_revision", collaboration_revision: "9007199254740999" }, 409));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  const handoff = { ...input, targetUserId, purpose: "transfer" as const, note: "Original note" };
  await expect(client.createCollaborationHandoff(issueId, handoff)).rejects.toThrow("Network unavailable");
  expect(fetchMock).toHaveBeenCalledTimes(1);
  await expect(client.createCollaborationHandoff(issueId, handoff)).rejects.toMatchObject({
    status: 409, body: { code: "stale_revision", collaboration_revision: "9007199254740999" },
  });
  expect(fetchMock).toHaveBeenCalledTimes(2);
  await expect(client.createCollaborationHandoff(issueId, handoff)).rejects.toMatchObject({
    status: 409, body: { code: "stale_revision", collaboration_revision: "9007199254740999" },
  });
  expect(fetchMock).toHaveBeenCalledTimes(3);
  for (const [, init] of fetchMock.mock.calls) {
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe(input.requestId);
    expect(JSON.parse(init.body)).toEqual({ ...expectedVersions, target_user_id: targetUserId, purpose: "transfer", note: "Original note" });
  }
  expect(handoff).toEqual({ ...input, targetUserId, purpose: "transfer", note: "Original note" });
});

it.each(operations)("returns null for malformed $name JSON or receipts without logging values", async ({ invoke }) => {
  const warn = vi.fn();
  const info = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(new Response("PRIVATE_INVALID_JSON"))
    .mockResolvedValueOnce(jsonResponse({ code: "PRIVATE_CODE", collaboration_revision: "PRIVATE_REVISION" }));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, info } });
  await expect(invoke(client)).resolves.toBeNull();
  await expect(invoke(client)).resolves.toBeNull();
  expect(warn).toHaveBeenCalledTimes(2);
  expect(JSON.stringify([...warn.mock.calls, ...info.mock.calls])).not.toContain("PRIVATE_");
});

it.each(operations)("keeps $name HTTP rejection codes and original revision out of logs", async ({ invoke }) => {
  const error = vi.fn();
  const body = { error: "PRIVATE_ERROR", code: "target_unavailable", collaboration_revision: "9007199254740999", note: "PRIVATE_NOTE" };
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(body, 409)));
  const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, error } });
  const result = invoke(client);
  await expect(result).rejects.toBeInstanceOf(ApiError);
  await expect(result).rejects.toMatchObject({ status: 409, body });
  expect(error).toHaveBeenCalledOnce();
  expect(JSON.stringify(error.mock.calls)).not.toContain("PRIVATE_");
});

it.each(operations)("discards $name results after a credential change during parsing", async ({ invoke }) => {
  const body = deferred<unknown>();
  const parsing = deferred<void>();
  const response = new Response();
  vi.spyOn(response, "json").mockImplementation(() => { parsing.resolve(); return body.promise; });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));
  const client = new ApiClient("https://api.example.test");
  client.setToken("first-account");
  const result = expect(invoke(client)).rejects.toMatchObject({ name: "AbortError" });
  await parsing.promise;
  client.setToken("second-account");
  body.resolve(receipt);
  await result;
});

it.each(operations.flatMap((operation) => [401, 403].map((status) => ({ ...operation, status }))))("discards stale $name $status responses without rejecting the new account", async ({ status, invoke }) => {
  const response = deferred<Response>();
  const onUnauthorized = vi.fn();
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(response.promise));
  const client = new ApiClient("https://api.example.test", { onUnauthorized });
  client.setToken("first-account");
  const result = expect(invoke(client)).rejects.toMatchObject({ name: "AbortError" });
  client.setToken("second-account");
  response.resolve(jsonResponse({ code: "forbidden" }, status));
  await result;
  expect(onUnauthorized).not.toHaveBeenCalled();
});

it.each(operations.flatMap((operation) => ["caller cancellation", "server switch"].map((reason) => ({ ...operation, reason }))))("discards $name on $reason during body parsing", async ({ reason, invoke }) => {
  const body = deferred<unknown>();
  const parsing = deferred<void>();
  const response = new Response();
  vi.spyOn(response, "json").mockImplementation(() => { parsing.resolve(); return body.promise; });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));
  const client = new ApiClient("https://api.example.test");
  const controller = new AbortController();
  const result = expect(invoke(client, controller.signal)).rejects.toMatchObject({ name: "AbortError" });
  await parsing.promise;
  if (reason === "caller cancellation") controller.abort();
  else client.invalidateSession();
  body.resolve(receipt);
  await result;
});

it.each([
  { fields: { title: "Changed", startDate: undefined, dueDate: undefined }, wire: { title: "Changed" } },
  { fields: { title: "", description: "" }, wire: { title: "", description: "" } },
  { fields: { startDate: null, dueDate: null }, wire: { start_date: null, due_date: null } },
  { fields: { dueDate: "" }, wire: { due_date: "" } },
  { fields: { startDate: "2026-10-14", dueDate: "2026-10-15T00:00:00Z" }, wire: { start_date: "2026-10-14", due_date: "2026-10-15T00:00:00Z" } },
])("preserves edit field omission, explicit clears and ordinary date syntax", async ({ fields, wire }) => {
  const result = { code: "issue_edited", collaboration_revision: "9223372036854775807" };
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse(result));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  const options = { signal: new AbortController().signal, method: "POST", body: "PRIVATE_REPLACEMENT", headers: { "Idempotency-Key": "wrong-key" } };
  await expect(client.editCollaborationIssue(issueId, { ...input, ...fields }, options)).resolves.toEqual(result);
  const init = fetchMock.mock.calls[0]?.[1];
  expect(init.method).toBe("PATCH");
  expect(JSON.parse(init.body)).toEqual({ ...expectedVersions, ...wire });
  expect(new Headers(init.headers).get("Idempotency-Key")).toBe(input.requestId);
});

it("keeps the original edit after an unknown response or stale-revision refusal", async () => {
  const failure = { code: "stale_revision", collaboration_revision: "9223372036854775807" };
  const fetchMock = vi.fn()
    .mockRejectedValueOnce(new TypeError("Network unavailable"))
    .mockImplementation(() => Promise.resolve(jsonResponse(failure, 409)));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  const edit = { ...input, description: "PRIVATE_ORIGINAL_EDIT", dueDate: null };
  await expect(client.editCollaborationIssue(issueId, edit)).rejects.toThrow("Network unavailable");
  await expect(client.editCollaborationIssue(issueId, edit)).rejects.toMatchObject({ status: 409, body: failure });
  await expect(client.editCollaborationIssue(issueId, edit)).rejects.toMatchObject({ status: 409, body: failure });
  expect(fetchMock).toHaveBeenCalledTimes(3);
  for (const [, init] of fetchMock.mock.calls) {
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe(input.requestId);
    expect(JSON.parse(init.body)).toEqual({ ...expectedVersions, description: "PRIVATE_ORIGINAL_EDIT", due_date: null });
  }
});

it.each([
  { expectedStatus: "blocked" }, { expectedStatus: null },
  { statusOptionsVersion }, { statusOptionsVersion: undefined },
])("rejects status confirmation fields on an ordinary transfer before sending", async (fields) => {
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  const handoff = { ...input, targetUserId, purpose: "transfer" as const, note: "Ordinary transfer", ...fields };
  await expect(client.createCollaborationHandoff(issueId, handoff)).rejects.toThrow("Transfer does not accept status confirmation fields");
  expect(fetchMock).not.toHaveBeenCalled();
});

it.each(mutations.filter(({ name }) => ["information request", "information return", "acceptance submission"].includes(name)))("does not refresh or replace the original $name confirmation on conflict", async ({ invoke, body }) => {
  const failure = { code: "status_options_changed", collaboration_revision: "9223372036854775807", status_options: {
    action: "return_information", status_options_version: "b".repeat(64), default_status: "replacement", options: [{ key: "replacement", name: "PRIVATE_OPTION", category: "in_progress" }],
  } };
  const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(jsonResponse(failure, 409)));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  await expect(invoke(client)).rejects.toMatchObject({ status: 409, body: failure });
  expect(fetchMock).toHaveBeenCalledOnce();
  await expect(invoke(client)).rejects.toMatchObject({ status: 409, body: failure });
  expect(fetchMock).toHaveBeenCalledTimes(2);
  for (const [, init] of fetchMock.mock.calls) {
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe(input.requestId);
    expect(JSON.parse(init.body)).toEqual(body);
  }
});
