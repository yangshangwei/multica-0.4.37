// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationKeys, collaborationScope } from "./access";
import { collaborationNotificationsOptions } from "./content-queries";

const id = "00000000-0000-4000-8000-000000000001";
const auth = vi.hoisted(() => ({ userId: "00000000-0000-4000-8000-000000000001" }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: auth.userId } }) } }));
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); auth.userId = id; vi.unstubAllGlobals(); });
function setup() {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  return { client, scope };
}

it("keeps all three filtered cursor chains in independent account caches", async () => {
  const { client, scope } = setup();
  const calls: string[] = [];
  vi.stubGlobal("fetch", vi.fn((url: string) => {
    calls.push(url);
    const query = new URL(url).searchParams;
    const view = query.get("view");
    return Promise.resolve(response({ items: [], unread_count: 7, view, next_cursor: query.has("cursor") ? null : `${view}-next` }));
  }));
  const options = (["unread", "all", "archived"] as const).map((view) => collaborationNotificationsOptions(client, scope, view));
  for (const option of options) {
    const result = await client.fetchInfiniteQuery({ ...option, pages: 2 });
    expect(result.pages).toHaveLength(2);
    expect(result.pages[0]?.view).toBe(new URL(calls[calls.length - 2]!).searchParams.get("view"));
    expect(result.pages.every((page) => page.unread_count === 7)).toBe(true);
    expect(result.pageParams).toEqual([undefined, `${result.pages[0]?.view}-next`]);
  }
  expect(client.getQueryCache().findAll({ queryKey: collaborationKeys.notifications(scope) })).toHaveLength(3);
  expect(options.map((option) => client.getQueryData(option.queryKey)?.pages[0]?.view)).toEqual(["unread", "all", "archived"]);
  expect(collaborationNotificationsOptions(client, scope).queryKey).toEqual(options[1]?.queryKey);
});

it("surfaces an unsupported filtered page as unavailable instead of cached empty success", async () => {
  const { client, scope } = setup();
  const options = collaborationNotificationsOptions(client, scope, "unread");
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response({ items: [], unread_count: 0, next_cursor: null })));
  await expect(client.fetchInfiniteQuery(options)).rejects.toMatchObject({ name: "CollaborationUnavailableError" });
  expect(client.getQueryData(options.queryKey)).toBeUndefined();
});

it("clears a denied notification snapshot and keeps query retry read-only", async () => {
  const { client, scope } = setup();
  const options = collaborationNotificationsOptions(client, scope, "archived");
  client.setQueryData(options.queryKey, { pages: [{ items: [{ id, issue_id: id, event_seq: "1", kind: "assigned", is_mention: false,
    source_kind: "issue", source_id: null, issue_title: "Old private title", can_open: true,
    created_at: "2026-11-22T00:00:00Z", read_at: null, archived_at: "2026-11-22T01:00:00Z" }], unread_count: 7, next_cursor: null }], pageParams: [undefined] });
  const fetch = vi.fn().mockResolvedValueOnce(response({ code: "forbidden" }, 403))
    .mockResolvedValueOnce(response({ items: [], unread_count: 7, next_cursor: null, view: "archived" }));
  vi.stubGlobal("fetch", fetch);
  await expect(client.fetchInfiniteQuery(options)).rejects.toThrow();
  expect(client.getQueryData(options.queryKey)).toBeUndefined();
  await client.fetchInfiniteQuery(options);
  expect(fetch).toHaveBeenCalledTimes(2);
  expect(fetch.mock.calls.every(([, init]) => (init.method ?? "GET") === "GET")).toBe(true);
});

it("fences a filtered page that arrives after the viewer changes", async () => {
  const { client, scope } = setup();
  const options = collaborationNotificationsOptions(client, scope, "unread");
  let finish!: (response: Response) => void;
  let began!: () => void;
  const started = new Promise<void>((resolve) => { began = resolve; });
  vi.stubGlobal("fetch", vi.fn(() => { began(); return new Promise<Response>((resolve) => { finish = resolve; }); }));
  const pending = client.fetchInfiniteQuery(options);
  await started;
  auth.userId = "00000000-0000-4000-8000-000000000002";
  finish(response({ items: [], unread_count: 7, next_cursor: null, view: "unread" }));
  await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  expect(client.getQueryData(options.queryKey)).toBeUndefined();
});
