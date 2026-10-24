// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "../api";
import { collaborationKeys, collaborationScope, clearCollaborationAccess } from "./access";
import { parseCollaborationIssue } from "./schema";
import { collaborationIssueFixture } from "./test-fixtures";
import { readCollaborationDrafts } from "./draft";
import { prepareCollaborationAction, executeCollaborationAction } from "./operation";

const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
const stored = new Map<string, string>();
const storage = { keys: () => [...stored.keys()], getItem: (key: string) => stored.get(key) ?? null,
  setItem: (key: string, value: string) => { stored.set(key, value); }, removeItem: (key: string) => { stored.delete(key); } };
const client = new QueryClient();
const key = "00000000-0000-4000-8000-000000000003";
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
afterEach(() => { client.clear(); stored.clear(); vi.unstubAllGlobals(); });
function setup() {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(auth.user.id);
  const issue = parseCollaborationIssue({ ...collaborationIssueFixture, write_context: { participant_epoch: "2", account_epoch: "3" } })!;
  const draft = prepareCollaborationAction(scope, issue, key, { action: "transfer", targetUserId: auth.user.id, note: "Original note" });
  return { scope, issue, draft };
}

it("persists the captured human intent before transport and waits for a receipt before invalidating", async () => {
  const { scope, issue, draft } = setup();
  client.setQueryData(collaborationKeys.issue(scope, issue.id), issue);
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async () => {
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
    expect(client.getQueryData(collaborationKeys.issue(scope, issue.id))).toEqual(issue);
    return json({ code: "transferred", collaboration_revision: "9007199254740994" });
  }));
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).resolves.toMatchObject({ code: "transferred" });
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(client.getQueryState(collaborationKeys.issue(scope, issue.id))?.isInvalidated).toBe(true);
});

it("keeps an unknown operation across recovery with the original key and payload", async () => {
  const { scope, draft } = setup();
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("offline"))
    .mockResolvedValueOnce(json({ code: "operation_not_found" }, 404))
    .mockResolvedValueOnce(json({ code: "transferred", collaboration_revision: "9007199254740994" }));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  await executeCollaborationAction(client, scope, draft, true, storage);
  expect(fetch.mock.calls[2]?.[1]?.body).toBe(fetch.mock.calls[0]?.[1]?.body);
  expect(new Headers(fetch.mock.calls[2]?.[1]?.headers).get("Idempotency-Key")).toBe(key);
});

it("fences a late human receipt after access loss without restoring private data", async () => {
  const { scope, issue, draft } = setup();
  let finish!: (value: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise<Response>((resolve) => { finish = resolve; })));
  const pending = expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toMatchObject({ name: "AbortError" });
  clearCollaborationAccess(client, scope, issue.id);
  finish(json({ code: "transferred", collaboration_revision: "9007199254740994" }));
  await pending;
  expect(client.getQueryData(collaborationKeys.issue(scope, issue.id))).toBeUndefined();
});

it("retires a durable refusal and keeps it refused when recovered through HTTP 200", async () => {
  const { scope, draft } = setup();
  const receipt = { status: 409, code: "stale_revision", collaboration_revision: "9007199254740994" };
  const fetch = vi.fn().mockResolvedValueOnce(json(receipt, 409)).mockResolvedValueOnce(json(receipt));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).resolves.toEqual(receipt);
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).resolves.toEqual(receipt);
});

it("settles any committed workflow refusal that carries its HTTP status", async () => {
  const { scope, draft } = setup();
  const receipt = { status: 409, code: "issue_closed", collaboration_revision: "9007199254740994" };
  const fetch = vi.fn().mockResolvedValueOnce(json({ ...receipt, error: "issue_closed" }, 409));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).resolves.toEqual(receipt);
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(fetch).toHaveBeenCalledTimes(1);
});

it("retires an actual HTTP 409 stale-revision refusal without a body status", async () => {
  const { scope, issue, draft } = setup();
  const queryKey = collaborationKeys.issue(scope, issue.id);
  client.setQueryData(queryKey, issue);
  const fetch = vi.fn().mockResolvedValueOnce(json({ error: "stale_revision", code: "stale_revision", collaboration_revision: "9007199254740994" }, 409));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).resolves.toEqual({
    status: 409, code: "stale_revision", collaboration_revision: "9007199254740994",
  });
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(client.getQueryData(queryKey)).toEqual(issue);
  expect(client.getQueryState(queryKey)?.isInvalidated).toBe(true);
  expect(fetch).toHaveBeenCalledTimes(1);
});

it("settles a statusless stale-revision refusal from an original-key recovery replay", async () => {
  const { scope, draft } = setup();
  const fetch = vi.fn().mockRejectedValueOnce(new TypeError("offline"))
    .mockResolvedValueOnce(json({ code: "operation_not_found" }, 404))
    .mockResolvedValueOnce(json({ error: "stale_revision", code: "stale_revision", collaboration_revision: "9007199254740994", private_note: "Never return this" }, 409));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toThrow();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).resolves.toEqual({
    status: 409, code: "stale_revision", collaboration_revision: "9007199254740994",
  });
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(fetch.mock.calls[1]?.[0]).toContain(`/operations/${key}`);
  expect(fetch.mock.calls[1]?.[1]?.method ?? "GET").toBe("GET");
  expect(fetch.mock.calls[2]?.[1]?.body).toBe(fetch.mock.calls[0]?.[1]?.body);
  expect(new Headers(fetch.mock.calls[2]?.[1]?.headers).get("Idempotency-Key")).toBe(key);
});

it.each([
  { name: "another refusal code", status: 409, body: { code: "busy", collaboration_revision: "2" } },
  { name: "another HTTP status", status: 500, body: { code: "stale_revision", collaboration_revision: "2" } },
  { name: "a success body status", status: 409, body: { status: 200, code: "stale_revision", collaboration_revision: "2" } },
  { name: "a different error body status", status: 409, body: { status: 500, code: "stale_revision", collaboration_revision: "2" } },
  { name: "a null body status", status: 409, body: { status: null, code: "stale_revision", collaboration_revision: "2" } },
  { name: "a string body status", status: 409, body: { status: "409", code: "stale_revision", collaboration_revision: "2" } },
  { name: "a fractional body status", status: 409, body: { status: 409.5, code: "stale_revision", collaboration_revision: "2" } },
  { name: "an out-of-range body status", status: 409, body: { status: 600, code: "stale_revision", collaboration_revision: "2" } },
  { name: "a missing revision", status: 409, body: { code: "stale_revision" } },
  { name: "a numeric revision", status: 409, body: { code: "stale_revision", collaboration_revision: 2 } },
  { name: "a zero revision", status: 409, body: { code: "stale_revision", collaboration_revision: "0" } },
  { name: "an overflowing revision", status: 409, body: { code: "stale_revision", collaboration_revision: "9223372036854775808" } },
])("keeps $name uncertain without refreshing or replacing the original intent", async ({ status, body }) => {
  const { scope, issue, draft } = setup();
  const queryKey = collaborationKeys.issue(scope, issue.id);
  client.setQueryData(queryKey, issue);
  const fetch = vi.fn().mockResolvedValueOnce(json(body, status));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toMatchObject({ status });
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
  expect(client.getQueryState(queryKey)?.isInvalidated).toBe(false);
  expect(fetch).toHaveBeenCalledTimes(1);
});

it("does not interpret an untyped rejection as a confirmed refusal", async () => {
  const { scope, draft } = setup();
  const error = { status: 409, body: { code: "stale_revision", collaboration_revision: "2" } };
  vi.stubGlobal("fetch", vi.fn().mockRejectedValueOnce(error));
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toBe(error);
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
});

it("does not retire an obsolete-auth stale-revision response", async () => {
  const { scope, draft } = setup();
  vi.stubGlobal("fetch", vi.fn().mockRejectedValueOnce(new ApiError("Obsolete response", 409, "Conflict", { code: "stale_revision", collaboration_revision: "2" }, () => false)));
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toMatchObject({ status: 409, isStaleAuthResponse: true });
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
});

it.each([401, 403, 404, 410])("clears protected data on HTTP %s even with a stale-revision-shaped body", async (status) => {
  const { scope, issue, draft } = setup();
  const queryKey = collaborationKeys.issue(scope, issue.id);
  client.setQueryData(queryKey, issue);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json({ code: "stale_revision", collaboration_revision: "2" }, status)));
  await expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toMatchObject({ status });
  expect(client.getQueryData(queryKey)).toBeUndefined();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
});

it("fences a late statusless stale-revision refusal after access loss", async () => {
  const { scope, issue, draft } = setup();
  const queryKey = collaborationKeys.issue(scope, issue.id);
  client.setQueryData(queryKey, issue);
  let finish!: (value: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise<Response>((resolve) => { finish = resolve; })));
  const pending = expect(executeCollaborationAction(client, scope, draft, false, storage)).rejects.toMatchObject({ name: "AbortError" });
  clearCollaborationAccess(client, scope, issue.id);
  finish(json({ code: "stale_revision", collaboration_revision: "2" }, 409));
  await pending;
  expect(client.getQueryData(queryKey)).toBeUndefined();
  expect(client.getQueryState(queryKey)?.isInvalidated).toBe(false);
});

it("recovers content-free human outcomes after access loss without posting", async () => {
  const { scope, issue, draft } = setup();
  clearCollaborationAccess(client, scope, issue.id);
  const fetch = vi.fn().mockResolvedValueOnce(json({ status: 200, code: "participant_left", collaboration_revision: "9007199254740994" }));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).resolves.toMatchObject({ code: "participant_left" });
  expect(fetch.mock.calls[0]?.[1]?.method ?? "GET").toBe("GET");
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
});

it("clears the protected issue when human operation recovery receives a fresh denial", async () => {
  const { scope, issue, draft } = setup();
  const queryKey = collaborationKeys.issue(scope, issue.id);
  client.setQueryData(queryKey, issue);
  const fetch = vi.fn().mockResolvedValueOnce(json({ code: "forbidden" }, 403));
  vi.stubGlobal("fetch", fetch);
  await expect(executeCollaborationAction(client, scope, draft, true, storage)).rejects.toMatchObject({ status: 403 });
  expect(client.getQueryData(queryKey)).toBeUndefined();
  expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  expect(fetch.mock.calls.map(([, init]) => init.method ?? "GET")).toEqual(["GET"]);
});
