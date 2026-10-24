// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { InfiniteQueryObserver, QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { setCurrentWorkspace } from "../platform/workspace-storage";
import { collaborationAccessEpoch, collaborationIssueAccessDenied, collaborationKeys, collaborationScope,
  clearCollaborationAccess, protectCollaborationRead } from "./access";
import { collaborationCommandHistoryOptions } from "./command-history-queries";
import { parseCollaborationCommandHistory } from "./command-history-schema";
import { collaborationCommandHistoryFixture as fixture } from "./command-history-test-fixtures";
import { collaborationIssueFixture } from "./test-fixtures";

const issueId = fixture.issue_id;
const actorId = issueId;
const other = "00000000-0000-4000-8000-000000000099";
const entry = fixture.items[0]!;
const older = { ...entry, id: other, input_event_id: other, input_event_seq: "1", comment_id: other,
  seq: "1", state: "waiting", reason: null, cancellation: null, content: "Older original input", created_at: "2026-10-20T08:00:00Z" };
const first = { ...fixture, next_cursor: "history-page-two" };
const second = { ...fixture, items: [older] };
const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
const cleanups: (() => void)[] = [];
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
afterEach(() => { cleanups.splice(0).reverse().forEach((dispose) => dispose()); auth.user.id = actorId; setCurrentWorkspace(null, null); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

function setup() {
  const api = new ApiClient("https://server.example");
  setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  cleanups.push(() => client.clear());
  const scope = collaborationScope(actorId);
  const issueKey = collaborationKeys.issue(scope, issueId);
  client.setQueryData(issueKey, collaborationIssueFixture);
  const options = collaborationCommandHistoryOptions(client, scope, issueId);
  return { api, client, scope, issueKey, options };
}

function observe(f: ReturnType<typeof setup>) {
  const observer = new InfiniteQueryObserver(f.client, { ...f.options, enabled: false });
  cleanups.push(observer.subscribe(() => {}));
  return observer;
}

describe("command history API request boundary", () => {
  it("reads with no ambient workspace headers, preserves no-store and binds issue/limit/cursor responses", async () => {
    const f = setup();
    setCurrentWorkspace("unrelated", other);
    const fetch = vi.fn().mockResolvedValueOnce(json(fixture)).mockResolvedValueOnce(json({ ...fixture, issue_id: other }))
      .mockResolvedValueOnce(json({ ...fixture, items: [entry, older] })).mockResolvedValueOnce(json(first));
    vi.stubGlobal("fetch", fetch);
    const signal = new AbortController().signal;
    await expect(f.api.getCollaborationCommandHistory(issueId, { limit: 1, cursor: "encoded+/ cursor" }, { signal })).resolves.toEqual(fixture);
    const [url, request] = fetch.mock.calls[0]!;
    expect(new URL(url).searchParams.get("cursor")).toBe("encoded+/ cursor");
    expect(request.cache).toBe("no-store");
    expect(request.signal).toBeInstanceOf(AbortSignal);
    const headers = new Headers(request.headers);
    expect(headers.has("X-Workspace-ID")).toBe(false);
    expect(headers.has("X-Workspace-Slug")).toBe(false);
    await expect(f.api.getCollaborationCommandHistory(issueId)).resolves.toBeNull();
    await expect(f.api.getCollaborationCommandHistory(issueId, { limit: 1 })).resolves.toBeNull();
    await expect(f.api.getCollaborationCommandHistory(issueId, { cursor: first.next_cursor })).resolves.toBeNull();
  });

  it("rejects invalid outbound identifiers, limits and cursors before transport", async () => {
    const f = setup();
    const fetch = vi.fn();
    vi.stubGlobal("fetch", fetch);
    await expect(f.api.getCollaborationCommandHistory("not-an-issue")).rejects.toThrow(TypeError);
    for (const limit of [0, 101, 1.5, Number.NaN, Number.POSITIVE_INFINITY]) {
      await expect(f.api.getCollaborationCommandHistory(issueId, { limit })).rejects.toThrow(TypeError);
    }
    for (const cursor of ["", "x".repeat(1025)]) {
      await expect(f.api.getCollaborationCommandHistory(issueId, { cursor })).rejects.toThrow(TypeError);
    }
    expect(fetch).not.toHaveBeenCalled();
  });

  it("degrades malformed JSON and incomplete evidence without exposing private response text", async () => {
    const f = setup();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response("PRIVATE_MALFORMED_COMMAND{"))
      .mockResolvedValueOnce(json({ ...fixture, items: [{ ...entry, cancellation: { kind: "manual" } }] })));
    await expect(f.api.getCollaborationCommandHistory(issueId)).resolves.toBeNull();
    await expect(f.api.getCollaborationCommandHistory(issueId)).resolves.toBeNull();
  });

  it("forwards cancellation to the composed in-flight transport signal", async () => {
    const f = setup();
    const controller = new AbortController();
    const aborted = vi.fn();
    vi.stubGlobal("fetch", vi.fn((_url: string, request: RequestInit) => new Promise<Response>((_resolve, reject) => {
      request.signal?.addEventListener("abort", () => { aborted(); reject(request.signal?.reason); }, { once: true });
    })));
    const request = f.api.getCollaborationCommandHistory(issueId, {}, { signal: controller.signal });
    const rejected = expect(request).rejects.toMatchObject({ name: "AbortError" });
    controller.abort();
    await rejected;
    expect(aborted).toHaveBeenCalledOnce();
  });

  it.each(["success", "denial"])("rejects a late API %s after the credential epoch changes", async (outcome) => {
    const f = setup();
    let finish!: (response: Response) => void;
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((resolve) => { finish = resolve; })));
    const request = f.api.getCollaborationCommandHistory(issueId);
    const rejected = expect(request).rejects.toMatchObject({ name: "AbortError" });
    f.api.setToken("replacement-credential");
    finish(outcome === "success" ? json(fixture) : json({ code: "not_found" }, 404));
    await rejected;
  });
});

describe("protected command history paging", () => {
  it("uses normalized issue, Server and viewer scope and requests complete cursor pages", async () => {
    const f = setup();
    expect(f.options.queryKey).toEqual(collaborationCommandHistoryOptions(f.client, f.scope, issueId.toUpperCase()).queryKey);
    expect(collaborationCommandHistoryOptions(f.client, f.scope, "00000000-0000-4000-8000-0000000000ab").queryKey)
      .toEqual(collaborationCommandHistoryOptions(f.client, f.scope, "00000000-0000-4000-8000-0000000000AB").queryKey);
    expect(f.options.queryKey).not.toEqual(collaborationCommandHistoryOptions(f.client, f.scope, other).queryKey);
    expect(f.options.queryKey).not.toEqual(collaborationCommandHistoryOptions(f.client, { ...f.scope, userId: other }, issueId).queryKey);
    expect(f.options.queryKey).not.toEqual(collaborationCommandHistoryOptions(f.client, { ...f.scope, apiScope: "other-session" }, issueId).queryKey);
    const fetch = vi.fn().mockResolvedValueOnce(json(first)).mockResolvedValueOnce(json(second));
    vi.stubGlobal("fetch", fetch);
    const result = await f.client.fetchInfiniteQuery({ ...f.options, pages: 2 });
    expect(result.pages.flatMap((page) => page.items)).toEqual([entry, older]);
    expect(result.pageParams).toEqual([undefined, first.next_cursor]);
    const requested = new URL(fetch.mock.calls[1]?.[0]);
    expect(requested.pathname).toBe(`/api/collaboration/issues/${issueId}/commands`);
    expect(requested.searchParams.get("limit")).toBe("50");
    expect(requested.searchParams.get("cursor")).toBe(first.next_cursor);
  });

  it.each(["missing-route", "malformed-history", "wrong-issue"])("rechecks current issue authority for %s without granting or revoking control", async (condition) => {
    const f = setup();
    const fetch = vi.fn().mockResolvedValueOnce(condition === "missing-route" ? new Response("404 page not found", { status: 404 })
      : condition === "wrong-issue" ? json({ ...fixture, issue_id: other }) : json({ protocol_version: 99 }))
      .mockResolvedValueOnce(json(collaborationIssueFixture));
    vi.stubGlobal("fetch", fetch);
    await expect(f.client.fetchInfiniteQuery(f.options)).rejects.toMatchObject({ name: "CollaborationUnavailableError" });
    expect(fetch.mock.calls[1]?.[0]).toBe(`https://server.example/api/collaboration/issues/${issueId}`);
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(false);
    expect(collaborationAccessEpoch(f.client, f.scope, issueId)).toBe(0);
    expect(f.client.getQueryData(f.issueKey)).toEqual(collaborationIssueFixture);
  });

  it("clears every protected page when the current issue recheck confirms access loss", async () => {
    const f = setup();
    f.client.setQueryData(f.options.queryKey, { pages: [parseCollaborationCommandHistory(fixture)!], pageParams: [undefined] });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response("404 page not found", { status: 404 }))
      .mockResolvedValueOnce(json({ code: "not_found" }, 404)));
    await expect(f.client.fetchInfiniteQuery(f.options)).rejects.toThrow();
    expect(f.client.getQueryData(f.issueKey)).toBeUndefined();
    expect(f.client.getQueryData(f.options.queryKey)).toBeUndefined();
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(true);
  });

  it("preserves lawful history when an ambiguous route's authority recheck has a transient failure", async () => {
    const f = setup();
    const cached = { pages: [parseCollaborationCommandHistory(fixture)!], pageParams: [undefined] };
    f.client.setQueryData(f.options.queryKey, cached);
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response("404 page not found", { status: 404 }))
      .mockResolvedValueOnce(json({ code: "unavailable" }, 503)));
    await expect(f.client.fetchInfiniteQuery(f.options)).rejects.toThrow();
    expect(f.client.getQueryData(f.issueKey)).toEqual(collaborationIssueFixture);
    expect(f.client.getQueryData(f.options.queryKey)).toEqual(cached);
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(false);
  });

  it("stops a returned cursor cycle before more transport and reports unavailable without denying the issue", async () => {
    const f = setup();
    const thirdId = "00000000-0000-4000-8000-000000000098";
    const third = { ...fixture, items: [{ ...older, id: thirdId, input_event_id: thirdId, comment_id: thirdId,
      input_event_seq: "2", seq: "2", created_at: "2026-10-19T08:00:00Z" }], next_cursor: first.next_cursor };
    const fetch = vi.fn().mockResolvedValueOnce(json(first)).mockResolvedValueOnce(json({ ...second, next_cursor: "history-page-three" }))
      .mockResolvedValueOnce(json(third));
    vi.stubGlobal("fetch", fetch);
    await expect(f.client.fetchInfiniteQuery({ ...f.options, pages: 10 })).rejects.toMatchObject({ name: "CollaborationUnavailableError" });
    expect(fetch).toHaveBeenCalledTimes(3);
    expect(f.client.getQueryData(f.options.queryKey)).toBeUndefined();
    expect(f.client.getQueryData(f.issueKey)).toEqual(collaborationIssueFixture);
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(false);
  });

  it.each(["duplicate", "conflicting-copy", "wrong-session", "reverse-order", "reused-cancellation"])("rejects %s across otherwise valid pages before caching it", async (condition) => {
    const f = setup();
    const logged = vi.fn();
    vi.spyOn(console, "error").mockImplementation(logged);
    vi.spyOn(console, "warn").mockImplementation(logged);
    const invalid = condition === "duplicate" ? entry
      : condition === "conflicting-copy" ? { ...entry, content: "CONFLICTING_PRIVATE_INPUT" }
      : condition === "wrong-session" ? { ...older, session_id: other }
      : condition === "reverse-order" ? { ...older, created_at: "2026-10-22T08:00:00Z" }
      : { ...older, state: "cancelled", reason: "cancelled_by_participant", cancellation: entry.cancellation };
    expect(parseCollaborationCommandHistory({ ...fixture, items: [invalid] })).not.toBeNull();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json(first)).mockResolvedValueOnce(json({ ...fixture, items: [invalid] })));
    await f.client.fetchInfiniteQuery(f.options);
    const observer = observe(f);
    await observer.fetchNextPage();
    expect(observer.getCurrentResult().isFetchNextPageError).toBe(true);
    expect(observer.getCurrentResult().error).toMatchObject({ name: "CollaborationUnavailableError" });
    expect(observer.getCurrentResult().data?.pages).toEqual([first]);
    expect(JSON.stringify(logged.mock.calls)).not.toContain("CONFLICTING_PRIVATE_INPUT");
    expect(f.client.getQueryData(f.issueKey)).toEqual(collaborationIssueFixture);
    clearCollaborationAccess(f.client, f.scope, issueId);
    expect(observer.getCurrentResult().data).toBeUndefined();
  });

  it("accepts a full refreshed snapshot after cancellation and Switch without comparing to obsolete placement", async () => {
    const f = setup();
    const waiting = { ...first, items: [{ ...entry, seq: "1", channel_id: other, batch_id: other, state: "waiting", reason: null, cancellation: null }] };
    const fetch = vi.fn().mockResolvedValueOnce(json(waiting)).mockResolvedValueOnce(json(second))
      .mockResolvedValueOnce(json({ ...first, next_cursor: "fresh-page-two" })).mockResolvedValueOnce(json(second));
    vi.stubGlobal("fetch", fetch);
    await f.client.fetchInfiniteQuery({ ...f.options, pages: 2 });
    const observer = observe(f);
    await observer.refetch();
    expect(observer.getCurrentResult().error).toBeNull();
    expect(observer.getCurrentResult().data?.pages.flatMap((page) => page.items)).toEqual([entry, older]);
    expect(observer.getCurrentResult().data?.pageParams).toEqual([undefined, "fresh-page-two"]);
    expect(fetch.mock.calls[3]?.[0]).toContain("cursor=fresh-page-two");
  });

  it("revokes all loaded pages and sibling private content after a denied continuation", async () => {
    const f = setup();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json(first)).mockResolvedValueOnce(json({ code: "not_found" }, 404)));
    await f.client.fetchInfiniteQuery(f.options);
    const observer = observe(f);
    await observer.fetchNextPage();
    expect(f.client.getQueryData(f.issueKey)).toBeUndefined();
    expect(f.client.getQueryData(f.options.queryKey)).toBeUndefined();
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(true);
  });

  it.each(["success", "denial"])("fences a delayed %s after same-session object access was revoked and recovered", async (outcome) => {
    const f = setup();
    let finish!: (response: Response) => void;
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((resolve) => { finish = resolve; })));
    const queryFn = f.options.queryFn;
    if (typeof queryFn !== "function") throw new Error("Missing history query");
    const pending = Promise.resolve(queryFn({ client: f.client, queryKey: f.options.queryKey, signal: new AbortController().signal,
      pageParam: undefined, direction: "forward", meta: undefined }));
    const rejected = expect(pending).rejects.toMatchObject({ name: "AbortError" });
    clearCollaborationAccess(f.client, f.scope, issueId);
    await protectCollaborationRead(f.client, f.scope, issueId, async () => collaborationIssueFixture, true);
    f.client.setQueryData(f.issueKey, { title: "Current recovered issue" });
    finish(outcome === "success" ? json(fixture) : json({ code: "not_found" }, 404));
    await rejected;
    expect(f.client.getQueryData(f.issueKey)).toEqual({ title: "Current recovered issue" });
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(false);
  });

  it.each(["server", "viewer", "session"])("ignores a delayed history response after the %s identity changes", async (identity) => {
    const f = setup();
    let finish!: (response: Response) => void;
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((resolve) => { finish = resolve; })));
    const pending = f.client.fetchInfiniteQuery(f.options);
    const rejected = expect(pending).rejects.toMatchObject({ name: "AbortError" });
    if (identity === "server") setApiInstance(new ApiClient("https://another-server.example"));
    else if (identity === "viewer") auth.user.id = other;
    else f.api.setToken("new-session");
    const current = collaborationScope(auth.user.id);
    const currentKey = collaborationKeys.issue(current, issueId);
    f.client.setQueryData(currentKey, { title: "Current identity" });
    finish(json({ code: "not_found" }, 404));
    await rejected;
    expect(f.client.getQueryData(currentKey)).toEqual({ title: "Current identity" });
    expect(collaborationIssueAccessDenied(f.client, current, issueId)).toBe(false);
  });
});
