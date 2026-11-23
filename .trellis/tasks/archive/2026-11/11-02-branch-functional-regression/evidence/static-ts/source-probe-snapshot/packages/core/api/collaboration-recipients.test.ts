// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient, ApiError } from "./client";
import type { CollaborationRecipientInput } from "../collaboration/schema";
import { setSchemaLogger } from "./schema";
import { noopLogger } from "../logger";
import { setCurrentWorkspace } from "../platform/workspace-storage";

const issueId = "00000000-0000-4000-8000-000000000001";
const sourceWorkspaceId = "00000000-0000-4000-8000-000000000002";
const userId = "00000000-0000-4000-8000-000000000003";
const response = {
  issue_id: issueId,
  source_workspace_id: sourceWorkspaceId,
  recipient: { user_id: userId, name: "Recipient", avatar_url: "", external: true },
};
const issueInput: CollaborationRecipientInput = { kind: "email", value: " PRIVATE_QUERY@example.test ", issueId };

function jsonResponse(data: unknown, status = 200, headers?: HeadersInit) {
  return new Response(JSON.stringify(data), { status, headers });
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

it.each([
  { input: issueInput, data: response, context: { issue_id: issueId } },
  { input: { kind: "username", value: "PRIVATE_USERNAME", sourceWorkspaceId } satisfies CollaborationRecipientInput, data: { ...response, issue_id: null }, context: { source_workspace_id: sourceWorkspaceId } },
  { input: { kind: "user_id", value: userId, issueId } satisfies CollaborationRecipientInput, data: response, context: { issue_id: issueId } },
])("resolves the exact target with only the supplied account context", async ({ input, data, context }) => {
  setCurrentWorkspace("unrelated-workspace", userId);
  const info = vi.fn();
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse(data));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, info } });
  client.setToken("test-only-token");
  const controller = new AbortController();
  await expect(client.resolveCollaborationRecipient(input, { signal: controller.signal })).resolves.toEqual(data);
  expect(fetchMock).toHaveBeenCalledOnce();
  const [url, init] = fetchMock.mock.calls[0]!;
  expect(url).toBe("https://api.example.test/api/collaboration/recipients/resolve");
  expect(init.method).toBe("POST");
  expect(JSON.parse(init.body)).toEqual({ kind: input.kind, value: input.value, ...context });
  const headers = new Headers(init.headers);
  expect(headers.get("Authorization")).toBe("Bearer test-only-token");
  expect(headers.get("Content-Type")).toBe("application/json");
  expect(headers.has("X-Workspace-ID")).toBe(false);
  expect(headers.has("X-Workspace-Slug")).toBe(false);
  expect(headers.has("Idempotency-Key")).toBe(false);
  controller.abort();
  expect(init.signal.aborted).toBe(true);
  expect(JSON.stringify(info.mock.calls)).not.toContain("PRIVATE_");
});

it.each([
  { input: issueInput, data: { ...response, issue_id: userId } },
  { input: issueInput, data: { ...response, issue_id: null } },
  { input: { kind: "email", value: "PRIVATE_EMAIL", sourceWorkspaceId } satisfies CollaborationRecipientInput, data: response },
  { input: { kind: "email", value: "PRIVATE_EMAIL", sourceWorkspaceId } satisfies CollaborationRecipientInput, data: { ...response, issue_id: null, source_workspace_id: userId } },
])("rejects a response that belongs to a different lookup context", async ({ input, data }) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(data)));
  await expect(new ApiClient("https://api.example.test").resolveCollaborationRecipient(input)).resolves.toBeNull();
});

it("refuses missing, mixed or null lookup contexts before sending a request", async () => {
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  // @ts-expect-error Exercise callers that bypass the exclusive context type.
  await expect(client.resolveCollaborationRecipient({ ...issueInput, sourceWorkspaceId })).rejects.toThrow("exactly one context");
  // @ts-expect-error Exercise an older or untyped caller without a context.
  await expect(client.resolveCollaborationRecipient({ kind: "email", value: "PRIVATE_QUERY" })).rejects.toThrow("exactly one context");
  // @ts-expect-error Explicit null is not a source- or issue-scoped lookup.
  await expect(client.resolveCollaborationRecipient({ ...issueInput, issueId: null })).rejects.toThrow("exactly one context");
  expect(fetchMock).not.toHaveBeenCalled();
});

it("binds a user-ID confirmation to the originally requested UUID", async () => {
  const matchedId = "00000000-0000-4000-8000-00000000abcd";
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(response))
    .mockResolvedValueOnce(jsonResponse({ ...response, recipient: { ...response.recipient, user_id: matchedId } }));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  const input: CollaborationRecipientInput = { kind: "user_id", value: ` ${matchedId.toUpperCase()} `, issueId };
  await expect(client.resolveCollaborationRecipient(input)).resolves.toBeNull();
  await expect(client.resolveCollaborationRecipient(input)).resolves.toMatchObject({ recipient: { user_id: matchedId } });
  expect(JSON.parse(fetchMock.mock.calls[1]![1].body).value).toBe(input.value);
});

it("does not reinterpret an in-flight lookup when its caller changes context", async () => {
  const pending = deferred<Response>();
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(pending.promise));
  const input = { ...issueInput };
  const result = new ApiClient("https://api.example.test").resolveCollaborationRecipient(input);
  input.issueId = userId;
  pending.resolve(jsonResponse({ ...response, issue_id: userId }));
  await expect(result).resolves.toBeNull();
});

it.each(["invalid json", "invalid summary"])("returns unavailable for %s without logging private values", async (format) => {
  const warn = vi.fn();
  const info = vi.fn();
  setSchemaLogger({ ...noopLogger, warn });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(format === "invalid json"
    ? new Response("PRIVATE_JSON")
    : jsonResponse({ ...response, recipient: { ...response.recipient, user_id: "PRIVATE_USER" } })));
  const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, info } });
  await expect(client.resolveCollaborationRecipient(issueInput)).resolves.toBeNull();
  expect(warn).toHaveBeenCalledOnce();
  expect(JSON.stringify([...warn.mock.calls, ...info.mock.calls])).not.toContain("PRIVATE_");
});

it.each([
  { status: 404, code: "recipient_not_found" },
  { status: 409, code: "recipient_ambiguous" },
  { status: 429, code: "rate_limited" },
  { status: 503, code: "collaboration_unavailable" },
])("preserves the $code outcome without a broad lookup or retry", async ({ status, code }) => {
  const error = vi.fn();
  const warn = vi.fn();
  const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ error: "PRIVATE_ERROR", code }, status, { "Retry-After": "12" }));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, error, warn } });
  const result = client.resolveCollaborationRecipient(issueInput);
  await expect(result).rejects.toBeInstanceOf(ApiError);
  await expect(result).rejects.toMatchObject({ status, body: { code }, retryAfterSeconds: 12 });
  expect(fetchMock).toHaveBeenCalledOnce();
  expect(JSON.stringify([...warn.mock.calls, ...error.mock.calls])).not.toContain("PRIVATE_");
});

it.each(["account", "server", "caller"])("discards recipient confirmation after %s change during body parsing", async (reason) => {
  const body = deferred<unknown>();
  const parsing = deferred<void>();
  const pendingResponse = new Response();
  vi.spyOn(pendingResponse, "json").mockImplementation(() => { parsing.resolve(); return body.promise; });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(pendingResponse));
  const client = new ApiClient("https://api.example.test");
  client.setToken("first-account");
  const controller = new AbortController();
  const result = expect(client.resolveCollaborationRecipient(issueInput, { signal: controller.signal })).rejects.toMatchObject({ name: "AbortError" });
  await parsing.promise;
  if (reason === "account") client.setToken("second-account");
  else if (reason === "server") client.invalidateSession();
  else controller.abort();
  body.resolve(response);
  await result;
});

it.each([401, 403])("does not let an old recipient %s reject a newer login", async (status) => {
  const pending = deferred<Response>();
  const onUnauthorized = vi.fn();
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(pending.promise));
  const client = new ApiClient("https://api.example.test", { onUnauthorized });
  client.setToken("first-account");
  const result = expect(client.resolveCollaborationRecipient(issueInput)).rejects.toMatchObject({ name: "AbortError" });
  client.setToken("second-account");
  pending.resolve(jsonResponse({ code: "forbidden" }, status));
  await result;
  expect(onUnauthorized).not.toHaveBeenCalled();
});
