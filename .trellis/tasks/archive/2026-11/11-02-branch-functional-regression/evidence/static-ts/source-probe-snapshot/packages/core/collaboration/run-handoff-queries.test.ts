// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { focusManager, onlineManager, QueryClient, QueryObserver } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { bindCollaborationRealtime } from "../realtime/collaboration";
import type { WSEventType } from "../types/events";
import { collaborationAccessEpoch, collaborationIssueAccessDenied, collaborationKeys, collaborationScope, clearCollaborationAccess, protectCollaborationRead } from "./access";
import { collaborationRunHandoffOptions } from "./run-handoff-queries";
import type { CollaborationRunHandoff } from "./run-handoff-schema";
import { collaborationIssueFixture as issue } from "./test-fixtures";

const actor = issue.handler.user_id;
const issueId = issue.id;
const runId = "00000000-0000-4000-8000-0000000000ab";
const otherId = "00000000-0000-4000-8000-0000000000ac";
const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
const page = { total: 0, returned: 0, truncated: false, items: [] };
const handoff: CollaborationRunHandoff = { protocol_version: 1, issue_id: issueId, run_id: runId, mode: "live", event_seq: "1",
  authority: { status: "in_progress", handler_id: actor }, assignment: { role: "agent", step_key: "agent" }, handoff: null, information_request: null,
  memory: { version: "0", availability: "absent" }, inputs: page, results: page, wakeup_causes: { ...page, completeness: "unknown" }, sources: [], missing: [] };
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
const cleanups: (() => void)[] = [];
afterEach(() => {
  cleanups.splice(0).reverse().forEach((cleanup) => cleanup());
  auth.user.id = actor;
  focusManager.setFocused(undefined);
  onlineManager.setOnline(true);
  vi.unstubAllGlobals();
});

function setup() {
  setApiInstance(new ApiClient("https://handoff.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  client.mount();
  cleanups.push(() => { client.unmount(); client.clear(); });
  const scope = collaborationScope(actor);
  const issueKey = collaborationKeys.issue(scope, issueId);
  client.setQueryData(issueKey, issue);
  const options = collaborationRunHandoffOptions(client, scope, issueId, runId);
  return { client, scope, issueKey, options };
}

it("normalizes run identity and keeps handoff caches separate from other runs", () => {
  const f = setup();
  expect(f.options.queryKey).toEqual(collaborationRunHandoffOptions(f.client, f.scope, issueId.toUpperCase(), runId.toUpperCase()).queryKey);
  expect(f.options.queryKey).not.toEqual(collaborationRunHandoffOptions(f.client, f.scope, issueId, otherId).queryKey);
});

it.each([403, 404, 410])("clears cached handoff content on a direct current denial (%s)", async (status) => {
  const f = setup();
  f.client.setQueryData(f.options.queryKey, handoff);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(json({ code: "not_found" }, status)));
  await expect(f.client.fetchQuery(f.options)).rejects.toThrow();
  expect(f.client.getQueryData(f.issueKey)).toBeUndefined();
  expect(f.client.getQueryData(f.options.queryKey)).toBeUndefined();
  expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(true);
});

it.each(["missing-route", "missing-run", "legacy-response", "malformed-response"])("revalidates parent authority without revoking it for %s", async (condition) => {
  const f = setup();
  const response = condition === "missing-route" ? new Response("404 page not found", { status: 404 })
    : condition === "missing-run" ? json({ code: "run_not_found" }, 404)
      : json(condition === "legacy-response" ? {} : { ...handoff, protocol_version: 99 });
  const fetch = vi.fn().mockResolvedValueOnce(response).mockResolvedValueOnce(json(issue));
  vi.stubGlobal("fetch", fetch);
  await expect(f.client.fetchQuery(f.options)).rejects.toMatchObject({ name: "CollaborationUnavailableError" });
  expect(fetch.mock.calls.map(([url]) => url)).toEqual([
    `https://handoff.example/api/collaboration/issues/${issueId}/runs/${runId}/handoff?execution_view_version=6`,
    `https://handoff.example/api/collaboration/issues/${issueId}?execution_view_version=6`,
  ]);
  expect(f.client.getQueryData(f.issueKey)).toEqual(issue);
  expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(false);
});

it.each([404, 503])("handles parent authority recheck failure (%s) without inventing access", async (status) => {
  const f = setup();
  f.client.setQueryData(f.options.queryKey, handoff);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response("404 page not found", { status: 404 }))
    .mockResolvedValueOnce(json({ code: status === 404 ? "not_found" : "unavailable" }, status)));
  await expect(f.client.fetchQuery(f.options)).rejects.toThrow();
  expect(f.client.getQueryData(f.options.queryKey)).toEqual(status === 404 ? undefined : handoff);
  expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(status === 404);
});

it.each(["server", "viewer", "epoch"])("fences both late success and late denial after a changed %s", async (identity) => {
  for (const denied of [false, true]) {
    const f = setup();
    let finish!: (response: Response) => void;
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((resolve) => { finish = resolve; })));
    const queryFn = f.options.queryFn;
    if (typeof queryFn !== "function") throw new Error("Missing handoff query");
    const pending = Promise.resolve(queryFn({ client: f.client, queryKey: f.options.queryKey, signal: new AbortController().signal, meta: undefined }));
    const rejected = expect(pending).rejects.toMatchObject({ name: "AbortError" });
    if (identity === "server") setApiInstance(new ApiClient("https://replacement.example"));
    else if (identity === "viewer") auth.user.id = otherId;
    else {
      clearCollaborationAccess(f.client, f.scope, issueId);
      await protectCollaborationRead(f.client, f.scope, issueId, async () => issue, true);
    }
    const currentScope = collaborationScope(auth.user.id);
    const currentKey = collaborationKeys.issue(currentScope, issueId);
    f.client.setQueryData(currentKey, { title: "Current projection" });
    finish(denied ? json({ code: "not_found" }, 404) : json(handoff));
    await rejected;
    expect(f.client.getQueryData(currentKey)).toEqual({ title: "Current projection" });
    expect(collaborationIssueAccessDenied(f.client, currentScope, issueId)).toBe(false);
    auth.user.id = actor;
  }
});

it.each(["issue-event", "socket-reconnect", "online-reconnect", "refocus"])("refreshes mounted handoff through the actual %s lifecycle using GET only", async (event) => {
  const f = setup();
  const refreshed = { ...handoff, event_seq: "2", memory: { version: "1", availability: "purged" } };
  const fetch = vi.fn().mockResolvedValueOnce(json(handoff)).mockResolvedValueOnce(json(refreshed));
  vi.stubGlobal("fetch", fetch);
  const observer = new QueryObserver(f.client, f.options);
  cleanups.push(observer.subscribe(() => {}));
  await vi.waitFor(() => expect(observer.getCurrentResult().data).toEqual(handoff));
  const handlers = new Map<WSEventType, (value: unknown) => void>();
  let authenticated = () => {};
  cleanups.push(bindCollaborationRealtime(f.client, f.scope, issueId, {
    subscribe: (kind, callback) => { handlers.set(kind, callback); return () => handlers.delete(kind); },
    onAuthenticated: (callback) => { authenticated = callback; return () => {}; },
    subscribeScope: () => () => {}, revalidateScope: () => {},
  }));
  if (event === "issue-event") handlers.get("collaboration:changed")?.({ issue_id: issueId, event_seq: "2", collaboration_revision: "2", authority_epoch: "1" });
  else if (event === "socket-reconnect") authenticated();
  else if (event === "online-reconnect") { onlineManager.setOnline(false); onlineManager.setOnline(true); }
  else { focusManager.setFocused(false); focusManager.setFocused(true); }
  await vi.waitFor(() => expect(observer.getCurrentResult().data).toEqual(refreshed));
  expect(fetch).toHaveBeenCalledTimes(2);
  expect(fetch.mock.calls.every(([url, init]) => String(url).includes("/handoff?") && (init.method ?? "GET") === "GET" && init.cache === "no-store")).toBe(true);
  expect(collaborationAccessEpoch(f.client, f.scope, issueId)).toBe(0);
});
