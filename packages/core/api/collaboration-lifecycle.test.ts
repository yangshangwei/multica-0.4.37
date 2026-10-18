// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "./client";
import { setSchemaLogger } from "./schema";
import { parseCollaborationStatusOptions, type CollaborationStatusAction } from "../collaboration/schema";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { noopLogger } from "../logger";

const issueId = "00000000-0000-4000-8000-000000000001";
const input = {
  requestId: "original-lifecycle-request",
  expectedCollaborationRevision: "9007199254740993",
  expectedLifecycleEpoch: "9007199254740994",
  expectedParticipantEpoch: "9007199254740995",
  expectedAccountEpoch: "9223372036854775807",
  status: "source_custom",
  statusOptionsVersion: "a".repeat(64),
};
const expectedBody = {
  expected_collaboration_revision: input.expectedCollaborationRevision,
  expected_lifecycle_epoch: input.expectedLifecycleEpoch,
  expected_participant_epoch: input.expectedParticipantEpoch,
  expected_account_epoch: input.expectedAccountEpoch,
  status: input.status,
  status_options_version: input.statusOptionsVersion,
};
const receipt = { code: "status_changed", collaboration_revision: "9007199254740996" };
const statusOptions = {
  action: "change_status", status_options_version: input.statusOptionsVersion,
  default_status: "source_custom", options: [{ key: "source_custom", name: "Source status", category: "todo" }],
};
const writes = [
  {
    name: "status change", method: "PATCH", route: "", body: expectedBody,
    invoke: (client: ApiClient, signal?: AbortSignal) => client.changeCollaborationStatus(issueId, input, { signal }),
  },
  ...(["complete", "cancel", "reopen"] as const).map((action) => ({
    name: action, method: "POST", route: "/lifecycle", body: { ...expectedBody, action },
    invoke: (client: ApiClient, signal?: AbortSignal) => client.changeCollaborationLifecycle(issueId, { ...input, action }, { signal }),
  })),
];
const operations = [
  ...writes.map((write) => ({ ...write, response: receipt })),
  {
    name: "status options", response: statusOptions,
    invoke: (client: ApiClient, signal?: AbortSignal) => client.getCollaborationStatusOptions(issueId, "change_status", { signal }),
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

it.each(writes)("sends $name using the captured versions, status, digest and key", async ({ invoke, method, route, body }) => {
  setCurrentWorkspace("unrelated-workspace", issueId);
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse(receipt));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  client.setToken("test-only-token");
  await expect(invoke(client)).resolves.toEqual(receipt);
  expect(fetchMock).toHaveBeenCalledOnce();
  const [url, init] = fetchMock.mock.calls[0]!;
  expect(url).toBe(`https://api.example.test/api/collaboration/issues/${issueId}${route}`);
  expect(init.method).toBe(method);
  expect(JSON.parse(init.body)).toEqual(body);
  const headers = new Headers(init.headers);
  expect(headers.get("Idempotency-Key")).toBe(input.requestId);
  expect(headers.get("Authorization")).toBe("Bearer test-only-token");
  expect(headers.get("Content-Type")).toBe("application/json");
  expect(headers.has("X-Workspace-ID")).toBe(false);
  expect(headers.has("X-Workspace-Slug")).toBe(false);
});

it("keeps the lifecycle POST method when caller options contain unrelated transport fields", async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse(receipt));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  const options = { signal: new AbortController().signal, method: "PATCH" as const };
  await client.changeCollaborationLifecycle(issueId, { ...input, action: "complete" }, options);
  expect(fetchMock.mock.calls[0]?.[1].method).toBe("POST");
});

it.each([
  { action: "change_status", category: "todo" },
  { action: "complete", category: "done" },
  { action: "cancel", category: "cancelled" },
  { action: "reopen", category: "todo" },
  { action: "request_information", category: "blocked" },
  { action: "return_information", category: "in_progress" },
  { action: "submit_acceptance", category: "in_review" },
] satisfies { action: CollaborationStatusAction; category: string }[])("reads account-scoped options for $action", async ({ action, category }) => {
  setCurrentWorkspace("unrelated-workspace", issueId);
  const options = { ...statusOptions, action, options: [{ ...statusOptions.options[0], category }] };
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse(options));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  await expect(client.getCollaborationStatusOptions(issueId, action)).resolves.toEqual(options);
  expect(fetchMock).toHaveBeenCalledOnce();
  const [url, init] = fetchMock.mock.calls[0]!;
  expect(url).toBe(`https://api.example.test/api/collaboration/issues/${issueId}/status-options?action=${action}`);
  expect(init.method ?? "GET").toBe("GET");
  expect(init.body).toBeUndefined();
  const headers = new Headers(init.headers);
  expect(headers.has("X-Workspace-ID")).toBe(false);
  expect(headers.has("X-Workspace-Slug")).toBe(false);
});

it("does not accept options for a different action", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ ...statusOptions, action: "reopen" })));
  const client = new ApiClient("https://api.example.test");
  await expect(client.getCollaborationStatusOptions(issueId, "change_status")).resolves.toBeNull();
});

it("keeps status options a GET when caller options contain unrelated transport fields", async () => {
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse(statusOptions));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  const options = { signal: new AbortController().signal, method: "POST", body: "PRIVATE_BODY" };
  await client.getCollaborationStatusOptions(issueId, "change_status", options);
  const init = fetchMock.mock.calls[0]?.[1];
  expect(init.method ?? "GET").toBe("GET");
  expect(init.body).toBeUndefined();
});

it.each(writes)("preserves the literal stale $name intent after a fresh-options rejection", async ({ invoke, body, name }) => {
  const action = name === "status change" ? "change_status" : name;
  const category = action === "complete" ? "done" : action === "cancel" ? "cancelled" : "todo";
  const freshOptions = { ...statusOptions, action, status_options_version: "b".repeat(64), default_status: "new_default", options: [{ key: "new_default", name: "New default", category }] };
  const errorBody = { error: "PRIVATE_ERROR", code: "status_options_changed", collaboration_revision: "9007199254740999", status_options: freshOptions };
  const error = vi.fn();
  const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(jsonResponse(errorBody, 409)));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, error } });
  const result = invoke(client);
  await expect(result).rejects.toBeInstanceOf(ApiError);
  const { error: _privateError, ...validatedReceipt } = errorBody;
  await expect(result).rejects.toMatchObject({ status: 409, body: validatedReceipt });
  await expect(result).rejects.not.toHaveProperty("body.error");
  expect(fetchMock).toHaveBeenCalledTimes(1);
  await expect(invoke(client)).rejects.toMatchObject({ status: 409, body: validatedReceipt });
  expect(fetchMock).toHaveBeenCalledTimes(2);
  for (const [, init] of fetchMock.mock.calls) {
    expect(new Headers(init.headers).get("Idempotency-Key")).toBe(input.requestId);
    expect(JSON.parse(init.body)).toEqual(body);
  }
  expect(parseCollaborationStatusOptions(errorBody.status_options)).toEqual(freshOptions);
  expect(JSON.stringify(error.mock.calls)).not.toContain("PRIVATE_");
});

it("retains current options supplied with an original operation recovery receipt", async () => {
  const original = { code: "status_options_changed", collaboration_revision: "9007199254740993", status_options: statusOptions };
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(original)));
  const client = new ApiClient("https://api.example.test");
  await expect(client.getCollaborationOperation(issueId, input.requestId)).resolves.toEqual(original);
});

it.each(operations)("marks malformed $name responses unavailable without logging content", async ({ invoke }) => {
  const warn = vi.fn();
  const info = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(new Response("PRIVATE_INVALID_JSON"))
    .mockResolvedValueOnce(jsonResponse({ code: "PRIVATE_CODE", default_status: "PRIVATE_STATUS" }));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, info } });
  await expect(invoke(client)).resolves.toBeNull();
  await expect(invoke(client)).resolves.toBeNull();
  expect(warn).toHaveBeenCalledTimes(2);
  expect(JSON.stringify([...warn.mock.calls, ...info.mock.calls])).not.toContain("PRIVATE_");
});

it.each(operations)("discards $name data after an account change during body parsing", async ({ invoke, response: data }) => {
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
  body.resolve(data);
  await result;
});

it.each(operations)("forwards caller cancellation through $name body parsing", async ({ invoke, response: data }) => {
  const body = deferred<unknown>();
  const parsing = deferred<void>();
  const response = new Response();
  vi.spyOn(response, "json").mockImplementation(() => { parsing.resolve(); return body.promise; });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));
  const client = new ApiClient("https://api.example.test");
  const controller = new AbortController();
  const result = expect(invoke(client, controller.signal)).rejects.toMatchObject({ name: "AbortError" });
  await parsing.promise;
  controller.abort();
  body.resolve(data);
  await result;
});
