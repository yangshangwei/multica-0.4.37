// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationIssueAccessDenied, collaborationKeys, collaborationScope, CollaborationUnavailableError } from "./access";
import { collaborationMemoryOptions, collaborationMemoryVersionOptions } from "./memory-queries";

const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
const issueId = "00000000-0000-4000-8000-000000000002";
const clients: QueryClient[] = [];
const empty = { issue_id: issueId, version: "0", content: "", bytes: 0, max_bytes: 131072,
  author: null, created_at: null, restored_from_version: null, can_edit: true, can_purge: false };
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
afterEach(() => { clients.splice(0).forEach(client => client.clear()); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
function setup() {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); clients.push(client);
  return { client, scope: collaborationScope(auth.user.id) };
}

it("refreshes an active shared-context read through the existing issue invalidation prefix", async () => {
  const fetch = vi.fn().mockImplementation(async () => json(empty)); vi.stubGlobal("fetch", fetch);
  const { client, scope } = setup();
  const options = collaborationMemoryOptions(client, scope, issueId);
  const observer = new QueryObserver(client, options);
  const unsubscribe = observer.subscribe(() => {});
  try {
    await client.fetchQuery(options);
    expect(fetch).toHaveBeenCalledTimes(1);
    await client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, issueId) });
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(client.getQueryData(options.queryKey)).toMatchObject(empty);
  } finally { unsubscribe(); }
});

it("treats an absent route and absent historical version as unavailable without denying the issue", async () => {
  const fetch = vi.fn().mockResolvedValueOnce(new Response("Not found", { status: 404 }))
    .mockResolvedValueOnce(json({ code: "memory_version_not_found" }, 404)); vi.stubGlobal("fetch", fetch);
  const { client, scope } = setup();
  client.setQueryData(collaborationKeys.issue(scope, issueId), { title: "Authorized issue" });
  await expect(client.fetchQuery(collaborationMemoryOptions(client, scope, issueId))).rejects.toThrow(CollaborationUnavailableError);
  await expect(client.fetchQuery(collaborationMemoryVersionOptions(client, scope, issueId, "1"))).rejects.toThrow(CollaborationUnavailableError);
  expect(collaborationIssueAccessDenied(client, scope, issueId)).toBe(false);
  expect(client.getQueryData(collaborationKeys.issue(scope, issueId))).toEqual({ title: "Authorized issue" });
});

it("clears all issue-scoped content when the memory read establishes actual access loss", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code: "not_found" }, 404)));
  const { client, scope } = setup();
  const issueKey = collaborationKeys.issue(scope, issueId);
  client.setQueryData<{ title: string }>(issueKey, { title: "Private issue" });
  client.setQueryData<{ content: string }>(collaborationMemoryVersionOptions(client, scope, issueId, "1").queryKey, { content: "Private history" });
  await expect(client.fetchQuery(collaborationMemoryOptions(client, scope, issueId))).rejects.toThrow();
  expect(collaborationIssueAccessDenied(client, scope, issueId)).toBe(true);
  expect(client.getQueryData(issueKey)).toBeUndefined();
  expect(client.getQueryData(collaborationMemoryVersionOptions(client, scope, issueId, "1").queryKey)).toBeUndefined();
});
