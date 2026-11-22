// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { ApiClient, setApiInstance } from "../api";
import { collaborationScope } from "./access";
import { collaborationNotificationsOptions, useUpdateCollaborationNotification } from "./content-queries";
import { COLLABORATION_NOTIFICATION_VIEWS } from "./notification-view";

const id = "00000000-0000-4000-8000-000000000001";
const timestamp = "2026-11-22T00:00:00Z";
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
const clients: QueryClient[] = [];
afterEach(() => { cleanup(); clients.splice(0).forEach((client) => client.clear()); vi.unstubAllGlobals(); });
function setup() {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  clients.push(client);
  const options = COLLABORATION_NOTIFICATION_VIEWS.map((view) => collaborationNotificationsOptions(client, scope, view));
  for (const option of options) {
    client.setQueryData(option.queryKey, { pages: [{ items: [], unread_count: 7, next_cursor: null }], pageParams: [undefined] });
  }
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  const hook = renderHook(() => useUpdateCollaborationNotification(scope), { wrapper });
  return { client, options, hook };
}

it("invalidates every notification view only after the write is confirmed", async () => {
  const { client, options, hook } = setup();
  let finish!: (response: Response) => void;
  const fetch = vi.fn(() => new Promise<Response>((resolve) => { finish = resolve; }));
  vi.stubGlobal("fetch", fetch);
  let pending!: Promise<unknown>;
  act(() => { pending = hook.result.current.mutateAsync({ id, action: "archive" }); });
  await waitFor(() => {
    expect(fetch).toHaveBeenCalledOnce();
    expect(hook.result.current.isPending).toBe(true);
  });
  expect(options.every((option) => client.getQueryState(option.queryKey)?.isInvalidated === false)).toBe(true);
  await act(async () => {
    finish(response({ id, read_at: null, archived_at: timestamp }));
    await pending;
  });
  expect(options.every((option) => client.getQueryState(option.queryKey)?.isInvalidated === true)).toBe(true);
  await waitFor(() => expect(hook.result.current.data).toEqual({ id, read_at: null, archived_at: timestamp }));
});

it("preserves a failed write while a list refresh retries only GET, and permits an explicit write retry", async () => {
  const { client, options, hook } = setup();
  const fetch = vi.fn().mockResolvedValueOnce(response({ code: "unavailable" }, 503))
    .mockResolvedValueOnce(response({ items: [], unread_count: 7, next_cursor: null, view: "unread" }))
    .mockResolvedValueOnce(response({ id, read_at: timestamp, archived_at: null }));
  vi.stubGlobal("fetch", fetch);
  await act(async () => {
    await expect(hook.result.current.mutateAsync({ id, action: "read" })).rejects.toMatchObject({ status: 503 });
  });
  await waitFor(() => expect(hook.result.current.isError).toBe(true));
  expect(options.every((option) => client.getQueryState(option.queryKey)?.isInvalidated === false)).toBe(true);
  await client.fetchInfiniteQuery(options[0]!);
  expect(fetch.mock.calls.map(([, init]) => init.method ?? "GET")).toEqual(["POST", "GET"]);
  expect(hook.result.current.isError).toBe(true);
  await act(async () => { await hook.result.current.mutateAsync({ id, action: "read" }); });
  expect(fetch.mock.calls.map(([, init]) => init.method ?? "GET")).toEqual(["POST", "GET", "POST"]);
  expect(options.every((option) => client.getQueryState(option.queryKey)?.isInvalidated === true)).toBe(true);
});
