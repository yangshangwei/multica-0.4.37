// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { InfiniteQueryObserver, QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationAccessEpoch, collaborationIssueAccessDenied, collaborationKeys, collaborationScope,
  clearCollaborationAccess, protectCollaborationRead } from "./access";
import * as collaboration from "./index";
import { collaborationIssueFixture } from "./test-fixtures";
import { collaborationRunRecordsFixture as fixture } from "./run-records-test-fixtures";

const actorId = fixture.issue_id;
const issueId = fixture.issue_id;
const runId = fixture.run.id;
const otherId = "00000000-0000-4000-8000-0000000000ab";
const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));
const cleanups: (() => void)[] = [];
afterEach(() => { cleanups.splice(0).reverse().forEach((dispose) => dispose()); auth.user.id = actorId; vi.unstubAllGlobals(); vi.restoreAllMocks(); });
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });

function setup() {
  const api = new ApiClient("https://server.example");
  setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  cleanups.push(() => client.clear());
  const scope = collaborationScope(actorId);
  const issueKey = collaborationKeys.issue(scope, issueId);
  client.setQueryData(issueKey, collaborationIssueFixture);
  const options = collaboration.collaborationRunRecordsOptions(client, scope, issueId, runId);
  return { api, client, scope, issueKey, options };
}

describe("protected recorded-run paging", () => {
  it("keeps independent run caches, normalized identities and complete cursor pages", async () => {
    const f = setup();
    expect(f.options.queryKey).toEqual(collaboration.collaborationRunRecordsOptions(f.client, f.scope, issueId.toUpperCase(), runId.toUpperCase()).queryKey);
    expect(f.options.queryKey).not.toEqual(collaboration.collaborationRunRecordsOptions(f.client, f.scope, issueId, otherId).queryKey);
    const first = { ...fixture, items: fixture.items.slice(0, 2), next_cursor: "next-page" };
    const second = { ...fixture, items: fixture.items.slice(2) };
    const fetch = vi.fn().mockResolvedValueOnce(json(first)).mockResolvedValueOnce(json(second));
    vi.stubGlobal("fetch", fetch);
    const result = await f.client.fetchInfiniteQuery({ ...f.options, pages: 2 });
    expect(result.pages.flatMap((page) => page.items)).toEqual(fixture.items);
    expect(result.pageParams).toEqual([undefined, "next-page"]);
    expect(fetch.mock.calls[1]?.[0]).toContain("cursor=next-page");
    expect(result.pages.at(-1)?.next_cursor).toBeNull();
  });

  it.each(["missing-route", "run-not-found", "malformed-records"])("preserves an authorized issue for %s and revalidates its current authority", async (condition) => {
    const f = setup();
    const fetch = vi.fn().mockResolvedValueOnce(condition === "missing-route" ? new Response("404 page not found", { status: 404 })
      : condition === "run-not-found" ? json({ code: "run_not_found" }, 404) : json({ protocol_version: 99 }))
      .mockResolvedValueOnce(json(collaborationIssueFixture));
    vi.stubGlobal("fetch", fetch);
    await expect(f.client.fetchInfiniteQuery(f.options)).rejects.toMatchObject({ name: "CollaborationUnavailableError" });
    expect(fetch.mock.calls.map((call) => call[0])).toEqual([
      `https://server.example/api/collaboration/issues/${issueId}/runs/${runId}/records?limit=50`,
      `https://server.example/api/collaboration/issues/${issueId}`,
    ]);
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(false);
    expect(collaborationAccessEpoch(f.client, f.scope, issueId)).toBe(0);
    expect(f.client.getQueryData(f.issueKey)).toEqual(collaborationIssueFixture);
  });

  it("clears protected data when the current parent recheck denies an ambiguous missing records route", async () => {
    const f = setup();
    f.client.setQueryData(f.options.queryKey, { pages: [collaboration.parseCollaborationRunRecords(fixture)!], pageParams: [undefined] });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response("404 page not found", { status: 404 }))
      .mockResolvedValueOnce(json({ code: "not_found" }, 404)));
    await expect(f.client.fetchInfiniteQuery(f.options)).rejects.toThrow();
    expect(f.client.getQueryData(f.issueKey)).toBeUndefined();
    expect(f.client.getQueryData(f.options.queryKey)).toBeUndefined();
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(true);
  });

  it("keeps previous lawful records when an optional-route authority recheck has a transient failure", async () => {
    const f = setup();
    f.client.setQueryData(f.options.queryKey, { pages: [collaboration.parseCollaborationRunRecords(fixture)!], pageParams: [undefined] });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response("404 page not found", { status: 404 }))
      .mockResolvedValueOnce(json({ code: "unavailable" }, 503)));
    await expect(f.client.fetchInfiniteQuery(f.options)).rejects.toThrow();
    expect(f.client.getQueryData(f.issueKey)).toEqual(collaborationIssueFixture);
    expect(f.client.getQueryData(f.options.queryKey)).toEqual({ pages: [fixture], pageParams: [undefined] });
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(false);
  });

  it("does not append another run's next page or fall back to ordinary messages", async () => {
    const f = setup();
    const first = { ...fixture, items: fixture.items.slice(0, 2), next_cursor: "second-page" };
    const fetch = vi.fn().mockResolvedValueOnce(json(first))
      .mockResolvedValueOnce(json({ ...fixture, run: { ...fixture.run, id: otherId } }))
      .mockResolvedValueOnce(json(collaborationIssueFixture));
    vi.stubGlobal("fetch", fetch);
    await f.client.fetchInfiniteQuery(f.options);
    const observer = new InfiniteQueryObserver(f.client, { ...f.options, enabled: false });
    cleanups.push(observer.subscribe(() => {}));
    await observer.fetchNextPage();
    expect(observer.getCurrentResult().isFetchNextPageError).toBe(true);
    expect(observer.getCurrentResult().data?.pages).toEqual([first]);
    expect(f.client.getQueryData(f.issueKey)).toEqual(collaborationIssueFixture);
    expect(fetch.mock.calls.every((call) => String(call[0]).includes("/api/collaboration/issues/"))).toBe(true);
  });

  it("stops a multi-page cursor cycle and reports unavailable without revoking issue access", async () => {
    const f = setup();
    const first = { ...fixture, items: fixture.items.slice(0, 2), next_cursor: "page-two" };
    const second = { ...fixture, items: fixture.items.slice(2, 4), next_cursor: "page-three" };
    const third = { ...fixture, items: fixture.items.slice(4), next_cursor: "page-two" };
    const fetch = vi.fn().mockResolvedValueOnce(json(first)).mockResolvedValueOnce(json(second))
      .mockResolvedValueOnce(json(third));
    vi.stubGlobal("fetch", fetch);
    await expect(f.client.fetchInfiniteQuery({ ...f.options, pages: 10 }))
      .rejects.toMatchObject({ name: "CollaborationUnavailableError" });
    expect(fetch).toHaveBeenCalledTimes(3);
    expect(f.client.getQueryData(f.options.queryKey)).toBeUndefined();
    expect(f.client.getQueryData(f.issueKey)).toEqual(collaborationIssueFixture);
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(false);
  });

  it("does not replace an immutable recorded entry with a conflicting copy from a later page", async () => {
    const f = setup();
    const logged = vi.fn();
    vi.spyOn(console, "error").mockImplementation(logged);
    vi.spyOn(console, "warn").mockImplementation(logged);
    const first = { ...fixture, items: fixture.items.slice(0, 2), next_cursor: "page-two" };
    const changed = { ...fixture.items[1], body: "CONFLICTING_RECORD_BODY" };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json(first))
      .mockResolvedValueOnce(json({ ...fixture, items: [changed] })));
    await f.client.fetchInfiniteQuery(f.options);
    const observer = new InfiniteQueryObserver(f.client, { ...f.options, enabled: false });
    cleanups.push(observer.subscribe(() => {}));
    await observer.fetchNextPage();
    expect(observer.getCurrentResult().error).toMatchObject({ name: "CollaborationUnavailableError" });
    expect(JSON.stringify(observer.getCurrentResult().data)).not.toContain("CONFLICTING_RECORD_BODY");
    expect(JSON.stringify(logged.mock.calls)).not.toContain("CONFLICTING_RECORD_BODY");
    expect(f.client.getQueryData(f.options.queryKey)).toEqual({ pages: [first], pageParams: [undefined] });
    expect(f.client.getQueryData(f.issueKey)).toEqual(collaborationIssueFixture);
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(false);
    clearCollaborationAccess(f.client, f.scope, issueId);
    expect(observer.getCurrentResult().data).toBeUndefined();
    expect(f.client.getQueryData(f.options.queryKey)).toBeUndefined();
  });

  it("accepts identical paging overlap and replaces a snapshot during a full refresh", async () => {
    const f = setup();
    const first = { ...fixture, items: fixture.items.slice(0, 2), next_cursor: "old-page-two" };
    const overlapping = { ...fixture, items: fixture.items.slice(1) };
    const refreshed = { ...fixture, items: fixture.items.slice(0, 3), next_cursor: "new-page-two" };
    const final = { ...fixture, items: fixture.items.slice(3) };
    const fetch = vi.fn().mockResolvedValueOnce(json(first)).mockResolvedValueOnce(json(overlapping))
      .mockResolvedValueOnce(json(refreshed)).mockResolvedValueOnce(json(final));
    vi.stubGlobal("fetch", fetch);
    await f.client.fetchInfiniteQuery(f.options);
    const observer = new InfiniteQueryObserver(f.client, { ...f.options, enabled: false });
    cleanups.push(observer.subscribe(() => {}));
    await observer.fetchNextPage();
    expect(observer.getCurrentResult().error).toBeNull();
    expect(observer.getCurrentResult().data?.pages).toEqual([first, overlapping]);
    await observer.refetch();
    expect(observer.getCurrentResult().error).toBeNull();
    expect(observer.getCurrentResult().data?.pages).toEqual([refreshed, final]);
    expect(observer.getCurrentResult().data?.pageParams).toEqual([undefined, "new-page-two"]);
  });

  it("revokes every loaded record page when access is lost during fetch-next-page", async () => {
    const f = setup();
    const first = { ...fixture, next_cursor: "second-page" };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(json(first)).mockResolvedValueOnce(json({ code: "not_found" }, 404)));
    await f.client.fetchInfiniteQuery(f.options);
    const observer = new InfiniteQueryObserver(f.client, { ...f.options, enabled: false });
    cleanups.push(observer.subscribe(() => {}));
    await observer.fetchNextPage();
    expect(f.client.getQueryData(f.issueKey)).toBeUndefined();
    expect(f.client.getQueryData(f.options.queryKey)).toBeUndefined();
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(true);
  });

  it.each(["success", "denial"])("fences a delayed %s after access was revoked and recovered in the same session", async (outcome) => {
    const f = setup();
    let finish!: (value: Response) => void;
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((resolve) => { finish = resolve; })));
    const queryFn = f.options.queryFn;
    if (typeof queryFn !== "function") throw new Error("Missing record query");
    const pending = Promise.resolve(queryFn({ client: f.client, queryKey: f.options.queryKey, signal: new AbortController().signal,
      pageParam: undefined, direction: "forward", meta: undefined }));
    const rejected = expect(pending).rejects.toMatchObject({ name: "AbortError" });
    clearCollaborationAccess(f.client, f.scope, issueId);
    await protectCollaborationRead(f.client, f.scope, issueId, async () => collaborationIssueFixture, true);
    f.client.setQueryData(f.issueKey, { title: "Current recovered projection" });
    finish(outcome === "success" ? json(fixture) : json({ code: "not_found" }, 404));
    await rejected;
    expect(f.client.getQueryData(f.issueKey)).toEqual({ title: "Current recovered projection" });
    expect(collaborationIssueAccessDenied(f.client, f.scope, issueId)).toBe(false);
  });

  it.each(["server", "viewer"])("ignores old-record denials after the %s identity changes", async (identity) => {
    const f = setup();
    let finish!: (value: Response) => void;
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((resolve) => { finish = resolve; })));
    const pending = f.client.fetchInfiniteQuery(f.options);
    const rejected = expect(pending).rejects.toMatchObject({ name: "AbortError" });
    if (identity === "server") setApiInstance(new ApiClient("https://other-server.example"));
    else auth.user.id = otherId;
    const current = collaborationScope(auth.user.id);
    const key = collaborationKeys.issue(current, issueId);
    f.client.setQueryData(key, { title: "Current viewer projection" });
    finish(json({ code: "not_found" }, 404));
    await rejected;
    expect(f.client.getQueryData(key)).toEqual({ title: "Current viewer projection" });
    expect(collaborationIssueAccessDenied(f.client, current, issueId)).toBe(false);
  });
});
