// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "./client";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { collaborationExecutionIssueFixture, collaborationIssueFixture } from "../collaboration/test-fixtures";
import { noopLogger } from "../logger";

const reads = [
  { name: "issue", read: (client: ApiClient) => client.getCollaborationIssue(collaborationIssueFixture.id), data: collaborationExecutionIssueFixture },
  { name: "creation", read: (client: ApiClient) => client.getCollaborationCreation(collaborationIssueFixture.source_workspace.id, "original-request"), data: collaborationExecutionIssueFixture },
  { name: "workbench", read: (client: ApiClient) => client.getCollaborationWorkbench(), data: { items: [], total: 0, next_cursor: null } },
];

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

afterEach(() => {
  vi.unstubAllGlobals();
  setCurrentWorkspace(null, null);
});

// The complete malformed/lineage matrix belongs to execution-schema.test.ts.
// Both private detail transports must actually use that canonical boundary.
it.each(reads.slice(0, 2))("validates the v3 execution projection for $name", async ({ read }) => {
  const valid = collaborationExecutionIssueFixture;
  vi.stubGlobal("fetch", vi.fn()
    .mockResolvedValueOnce(new Response(JSON.stringify(valid)))
    .mockResolvedValueOnce(new Response(JSON.stringify({ ...valid, execution: { ...valid.execution, protocol_version: 4 } }))));
  const client = new ApiClient("https://api.example.test");
  await expect(read(client)).resolves.toMatchObject({
    execution: valid.execution,
    capabilities: { read: true, edit: true, control_ai: false, submit_command: false },
  });
  await expect(read(client)).resolves.toBeNull();
});

it("keeps collaboration reads account-scoped while retaining authentication", async () => {
  setCurrentWorkspace("unrelated-space", "00000000-0000-4000-8000-000000000099");
  const fetchMock = vi.fn().mockImplementation((url: string, _init: RequestInit) => Promise.resolve(new Response(JSON.stringify(
    url.includes("/me/workbench") ? { items: [], total: 0, next_cursor: null } : collaborationIssueFixture,
  ), { status: 200, headers: { "Content-Type": "application/json" } })));
  vi.stubGlobal("fetch", fetchMock);
  const client = new ApiClient("https://api.example.test");
  client.setToken("test-only-token");
  const controller = new AbortController();
  expect((await client.getCollaborationIssue(collaborationIssueFixture.id, { signal: controller.signal }))?.id).toBe(collaborationIssueFixture.id);
  await client.getCollaborationCreation(collaborationIssueFixture.source_workspace.id, "original-request");
  expect(await client.getCollaborationWorkbench({ view: "participating", limit: 10 })).toEqual({ items: [], total: 0, next_cursor: null });
  for (const [, init] of fetchMock.mock.calls) {
    const headers = new Headers(init.headers);
    expect(headers.get("Authorization")).toBe("Bearer test-only-token");
    expect(headers.has("X-Workspace-ID")).toBe(false);
    expect(headers.has("X-Workspace-Slug")).toBe(false);
    expect(init.signal).toBeInstanceOf(AbortSignal);
  }
  expect(fetchMock.mock.calls[2]?.[0]).toContain("view=participating&limit=10");
});

it("keeps unsupported-server errors distinct from an empty workbench", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: "not supported" }), { status: 404 })));
  const client = new ApiClient("https://api.example.test");
  await expect(client.getCollaborationWorkbench()).rejects.toMatchObject({ status: 404 });
});

it.each(reads)("redacts private HTTP errors from $name logs", async ({ read }) => {
  const error = vi.fn();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({
    error: "PRIVATE_ERROR_SENTINEL", code: "forbidden", detail: "PRIVATE_BODY_SENTINEL",
  }), { status: 403 })));
  const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, error } });
  await expect(read(client)).rejects.toMatchObject({ status: 403, body: { code: "forbidden" } });
  expect(error).toHaveBeenCalledOnce();
  expect(JSON.stringify(error.mock.calls)).not.toContain("PRIVATE_");
});

it("omits opaque cursor values from workbench request logs", async () => {
  const info = vi.fn();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [], total: 0, next_cursor: null }))));
  const client = new ApiClient("https://api.example.test", { logger: { ...noopLogger, info } });
  await client.getCollaborationWorkbench({ cursor: "PRIVATE_CURSOR_SENTINEL" });
  expect(info).toHaveBeenCalledTimes(2);
  expect(JSON.stringify(info.mock.calls)).not.toContain("PRIVATE_CURSOR_SENTINEL");
});

it.each(reads)("discards $name data after a credential change during body parsing", async ({ read, data }) => {
  const body = deferred<unknown>();
  const parsing = deferred<void>();
  const response = new Response();
  vi.spyOn(response, "json").mockImplementation(() => { parsing.resolve(); return body.promise; });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));
  const client = new ApiClient("https://api.example.test");
  client.setToken("first-account");
  const result = expect(read(client)).rejects.toMatchObject({ name: "AbortError" });
  await parsing.promise;
  client.setToken("second-account");
  body.resolve(data);
  await result;
});

it.each([401, 403])("does not let a stale collaboration %s reject the replacement session", async (status) => {
  const response = deferred<Response>();
  const onUnauthorized = vi.fn();
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(response.promise));
  const client = new ApiClient("https://api.example.test", { onUnauthorized });
  client.setToken("first-account");
  const result = expect(client.getCollaborationIssue(collaborationIssueFixture.id)).rejects.toMatchObject({ name: "AbortError" });
  client.setToken("second-account");
  response.resolve(new Response(JSON.stringify({ error: "forbidden", code: "forbidden" }), { status }));
  await result;
  expect(onUnauthorized).not.toHaveBeenCalled();
});

it("keeps a current collaboration 401 authoritative", async () => {
  const onUnauthorized = vi.fn();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 })));
  const client = new ApiClient("https://api.example.test", { onUnauthorized });
  client.setToken("current-account");
  await expect(client.getCollaborationWorkbench()).rejects.toMatchObject({ status: 401, isStaleAuthResponse: false });
  expect(onUnauthorized).toHaveBeenCalledOnce();
});

it("discards a collaboration result after a cookie login replaces the session", async () => {
  const response = deferred<Response>();
  vi.stubGlobal("fetch", vi.fn().mockImplementation((url: string) => url.includes("/auth/verify-code")
    ? Promise.resolve(new Response(JSON.stringify({ token: "", user: { id: "new-user" } })))
    : response.promise));
  const client = new ApiClient("https://api.example.test");
  const result = expect(client.getCollaborationIssue(collaborationIssueFixture.id)).rejects.toMatchObject({ name: "AbortError" });
  await client.verifyCode("fixture@example.test", "test-code");
  response.resolve(new Response(JSON.stringify(collaborationIssueFixture)));
  await result;
});

it.each(["valid", "malformed"])("forwards cancellation through %s collaboration body parsing", async (format) => {
  const body = deferred<unknown>();
  const parsing = deferred<void>();
  const response = new Response();
  vi.spyOn(response, "json").mockImplementation(() => {
    parsing.resolve();
    return body.promise.then((data) => {
      if (format === "malformed") throw new SyntaxError("PRIVATE_JSON_SENTINEL");
      return data;
    });
  });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));
  const client = new ApiClient("https://api.example.test");
  const controller = new AbortController();
  const result = expect(client.getCollaborationWorkbench({}, { signal: controller.signal })).rejects.toMatchObject({ name: "AbortError" });
  await parsing.promise;
  controller.abort();
  body.resolve({ items: [], total: 0, next_cursor: null });
  await result;
});

it("discards collaboration data after switching servers", async () => {
  const response = deferred<Response>();
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(response.promise));
  const client = new ApiClient("https://first-server.example.test");
  const result = expect(client.getCollaborationIssue(collaborationIssueFixture.id)).rejects.toMatchObject({ name: "AbortError" });
  client.invalidateSession();
  response.resolve(new Response(JSON.stringify(collaborationIssueFixture)));
  await result;
});

it.each(reads)("marks malformed $name JSON unavailable without exposing parser values", async ({ read }) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("PRIVATE_JSON_SENTINEL invalid json")));
  const client = new ApiClient("https://api.example.test");
  await expect(read(client)).resolves.toBeNull();
});

it("does not accept a different issue from a valid detail response", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({
    ...collaborationIssueFixture, id: "00000000-0000-4000-8000-000000000002",
  }))));
  const client = new ApiClient("https://api.example.test");
  await expect(client.getCollaborationIssue(collaborationIssueFixture.id)).resolves.toBeNull();
});

it("does not accept a creation result from a different source workspace", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({
    ...collaborationIssueFixture,
    source_workspace: { ...collaborationIssueFixture.source_workspace, id: "00000000-0000-4000-8000-000000000002" },
  }))));
  const client = new ApiClient("https://api.example.test");
  await expect(client.getCollaborationCreation(collaborationIssueFixture.source_workspace.id, "original-request")).resolves.toBeNull();
});
