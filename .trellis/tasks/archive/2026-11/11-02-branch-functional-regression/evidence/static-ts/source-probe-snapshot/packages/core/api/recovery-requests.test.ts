// @vitest-environment node
import { afterEach, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

const requests = [
  { name: "identity", send: (api: ApiClient, signal?: AbortSignal) => api.getMe({ signal, timeoutMs: 10_000 }) },
  { name: "config", send: (api: ApiClient, signal?: AbortSignal) => api.getConfig({ signal, timeoutMs: 10_000 }) },
  { name: "device login", send: (api: ApiClient, signal?: AbortSignal) => api.deviceLogin("device-id", "Test device", { signal, timeoutMs: 10_000 }) },
  { name: "workspace list", send: (api: ApiClient, signal?: AbortSignal) => api.listWorkspaces({ signal, timeoutMs: 10_000 }) },
];

afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals(); });

it.each(requests)("bounds a hanging $name request and aborts its network signal", async ({ send }) => {
  vi.useFakeTimers();
  const fetchRequest = vi.fn<typeof fetch>().mockImplementation(() => new Promise(() => {}));
  vi.stubGlobal("fetch", fetchRequest);
  const api = new ApiClient("https://server-a.test");
  const outcome = send(api).catch((error: unknown) => error);
  await vi.advanceTimersByTimeAsync(10_000);
  expect(await outcome).toMatchObject({ name: "TimeoutError" });
  expect(fetchRequest.mock.calls[0]?.[1]?.signal?.aborted).toBe(true);
  expect(vi.getTimerCount()).toBe(0);
});

it.each(requests)("keeps the $name deadline active through delayed body parsing", async ({ send }) => {
  vi.useFakeTimers();
  let finish!: (data: unknown) => void;
  const response = Response.json({});
  const parsing = vi.spyOn(response, "json").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  vi.stubGlobal("fetch", vi.fn<typeof fetch>().mockResolvedValue(response));
  const api = new ApiClient("https://server-a.test");
  const scope = api.getSessionScope();
  const outcome = send(api).catch((error: unknown) => error);
  await vi.advanceTimersByTimeAsync(0);
  expect(parsing).toHaveBeenCalledOnce();
  await vi.advanceTimersByTimeAsync(10_000);
  expect(await outcome).toMatchObject({ name: "TimeoutError" });
  finish({ token: "late-token", user: { id: "late-user" } });
  await vi.advanceTimersByTimeAsync(0);
  expect(api.getSessionScope()).toBe(scope);
});

it.each(requests)("cancels an obsolete $name request before its late 401 can end a session", async ({ send }) => {
  let finish!: (response: Response) => void;
  const fetchRequest = vi.fn<typeof fetch>().mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  vi.stubGlobal("fetch", fetchRequest);
  const onUnauthorized = vi.fn();
  const api = new ApiClient("https://server-a.test", { onUnauthorized });
  api.setToken("saved-token");
  const controller = new AbortController();
  const outcome = send(api, controller.signal).catch((error: unknown) => error);
  controller.abort();
  finish(Response.json({ error: "fixture-secret" }, { status: 401 }));
  expect(await outcome).toMatchObject({ name: "AbortError" });
  await Promise.resolve();
  expect(onUnauthorized).not.toHaveBeenCalled();
  expect(fetchRequest.mock.calls[0]?.[1]?.signal?.aborted).toBe(true);
});

it.each(requests)("cancels $name body parsing when the endpoint is invalidated", async ({ send }) => {
  vi.useFakeTimers();
  let finish!: (body: unknown) => void;
  const response = Response.json({});
  const parsing = vi.spyOn(response, "json").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const fetchRequest = vi.fn<typeof fetch>().mockResolvedValue(response);
  vi.stubGlobal("fetch", fetchRequest);
  const onUnauthorized = vi.fn();
  const api = new ApiClient("https://server-a.test", { onUnauthorized });
  api.setToken("saved-token");
  const outcome = send(api).catch((error: unknown) => error);
  await vi.advanceTimersByTimeAsync(0);
  expect(parsing).toHaveBeenCalledOnce();

  api.invalidateSession();
  const switchedScope = api.getSessionScope();
  expect(await outcome).toMatchObject({ name: "AbortError" });
  expect(fetchRequest.mock.calls[0]?.[1]?.signal?.aborted).toBe(true);
  expect(vi.getTimerCount()).toBe(0);
  finish({ token: "late-token", user: { id: "late-user" } });
  await vi.advanceTimersByTimeAsync(0);
  expect(api.getSessionScope()).toBe(switchedScope);
  expect(onUnauthorized).not.toHaveBeenCalled();
});

it("does not normalize a canceled legacy identity body into a session rejection", async () => {
  vi.useFakeTimers();
  let finish!: (body: unknown) => void;
  const response = Response.json({}, { status: 404 });
  const parsing = vi.spyOn(response, "json").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  vi.stubGlobal("fetch", vi.fn<typeof fetch>().mockResolvedValue(response));
  const onUnauthorized = vi.fn();
  const api = new ApiClient("https://server-a.test", { onUnauthorized });
  api.setToken("saved-token");
  const scope = api.getSessionScope();
  const controller = new AbortController();
  const outcome = api.getMe({ signal: controller.signal, timeoutMs: 10_000 }).catch((error: unknown) => error);
  await vi.advanceTimersByTimeAsync(0);
  expect(parsing).toHaveBeenCalledOnce();

  controller.abort();
  expect(await outcome).toMatchObject({ name: "AbortError" });
  finish({ error: "user not found" });
  await vi.advanceTimersByTimeAsync(0);
  expect(api.getSessionScope()).toBe(scope);
  expect(onUnauthorized).not.toHaveBeenCalled();
  expect(vi.getTimerCount()).toBe(0);
});

it("does not rotate a device credential whose body arrives after a newer login", async () => {
  let finish!: (body: unknown) => void;
  const response = Response.json({});
  const parsing = vi.spyOn(response, "json").mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  vi.stubGlobal("fetch", vi.fn<typeof fetch>().mockResolvedValue(response));
  const api = new ApiClient("https://server-a.test");
  const outcome = api.deviceLogin("device-id").catch((error: unknown) => error);
  await vi.waitFor(() => expect(parsing).toHaveBeenCalledOnce());
  api.setToken("new-login");
  const currentScope = api.getSessionScope();
  finish({ token: "old-device-token", user: { id: "old-device-user" } });
  expect(await outcome).toBeInstanceOf(Error);
  expect(api.getSessionScope()).toBe(currentScope);
});
