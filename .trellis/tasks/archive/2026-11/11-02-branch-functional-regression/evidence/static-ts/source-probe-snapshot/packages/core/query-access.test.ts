// @vitest-environment node
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./api";
import { withQueryAccessBoundary } from "./query-access";

let client: QueryClient;
const key = ["settings", "workspace-1"] as const;

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
});
afterEach(() => { client.clear(); vi.useRealTimers(); });

describe("withQueryAccessBoundary", () => {
  it.each([401, 403])("evicts a %i snapshot through a later failure and remount", async (status) => {
    const denied = new ApiError("denied", status, "Denied");
    const offline = new ApiError("offline", 500, "Unavailable");
    const read = vi.fn<() => Promise<{ name: string }>>()
      .mockRejectedValueOnce(denied)
      .mockRejectedValueOnce(offline)
      .mockImplementationOnce(() => new Promise(() => {}))
      .mockResolvedValue({ name: "Fresh settings" });
    const options = { queryKey: key, queryFn: withQueryAccessBoundary(read) };
    client.setQueryData(key, { name: "Denied settings" });
    client.setQueryData(["settings", "workspace-2"], { name: "Other workspace" });

    await expect(client.fetchQuery(options)).rejects.toBe(denied);
    expect(client.getQueryState(key)).toMatchObject({ data: undefined, dataUpdatedAt: 0 });
    await expect(client.fetchQuery(options)).rejects.toBe(offline);
    expect(client.getQueryData(key)).toBeUndefined();

    const observer = new QueryObserver(client, options);
    const unsubscribe = observer.subscribe(() => {});
    expect(observer.getCurrentResult().data).toBeUndefined();
    expect(read).toHaveBeenCalledTimes(3);
    unsubscribe();

    expect(client.getQueryData(["settings", "workspace-2"])).toEqual({ name: "Other workspace" });
    await expect(client.fetchQuery(options)).resolves.toEqual({ name: "Fresh settings" });
  });

  it.each([404, 409, 500])("retains cached data after a non-access %i failure", async (status) => {
    const error = new ApiError("failed", status, "Failed");
    client.setQueryData(key, { name: "Saved settings" });
    await expect(client.fetchQuery({
      queryKey: key,
      queryFn: withQueryAccessBoundary(() => Promise.reject(error)),
    })).rejects.toBe(error);
    expect(client.getQueryData(key)).toEqual({ name: "Saved settings" });
  });

  it("does not restore a denied snapshot when unmount cancels an automatic retry", async () => {
    vi.useFakeTimers();
    const read = vi.fn<() => Promise<{ name: string }>>()
      .mockRejectedValueOnce(new ApiError("denied", 403, "Forbidden"))
      .mockImplementationOnce(() => new Promise(() => {}))
      .mockResolvedValue({ name: "Fresh settings" });
    const options = {
      queryKey: key,
      queryFn: withQueryAccessBoundary(read),
      retry: 1,
      retryDelay: 1_000,
    };
    client.setQueryData(key, { name: "Denied settings" });
    const observer = new QueryObserver(client, options);
    const unsubscribe = observer.subscribe(() => {});
    await vi.advanceTimersByTimeAsync(0);
    expect(client.getQueryState(key)?.fetchFailureCount).toBe(1);
    expect(client.getQueryData(key)).toBeUndefined();

    unsubscribe();
    expect(client.getQueryData(key)).toBeUndefined();
    const remounted = new QueryObserver(client, options);
    const stop = remounted.subscribe(() => {});
    expect(remounted.getCurrentResult().data).toBeUndefined();
    stop();

    await expect(client.fetchQuery(options)).resolves.toEqual({ name: "Fresh settings" });
  });

  it("accepts a successful automatic retry after clearing a denied snapshot", async () => {
    vi.useFakeTimers();
    const read = vi.fn<() => Promise<{ name: string }>>()
      .mockRejectedValueOnce(new ApiError("denied", 403, "Forbidden"))
      .mockResolvedValue({ name: "Fresh settings" });
    client.setQueryData(key, { name: "Denied settings" });
    const fetching = client.fetchQuery({
      queryKey: key,
      queryFn: withQueryAccessBoundary(read),
      retry: 1,
      retryDelay: 1_000,
    });
    await vi.advanceTimersByTimeAsync(0);
    expect(client.getQueryData(key)).toBeUndefined();
    await vi.advanceTimersByTimeAsync(1_000);
    await expect(fetching).resolves.toEqual({ name: "Fresh settings" });
    expect(client.getQueryData(key)).toEqual({ name: "Fresh settings" });
    expect(read).toHaveBeenCalledTimes(2);
  });

  it("does not let a cancelled older denial erase a newer snapshot on the same Query", async () => {
    let reject!: (error: Error) => void;
    const pending = new Promise<{ name: string }>((_resolve, fail) => { reject = fail; });
    const read = vi.fn(() => pending);
    client.setQueryData(key, { name: "Previous settings" });
    const query = client.getQueryCache().find({ queryKey: key, exact: true });
    const fetching = client.fetchQuery({ queryKey: key, queryFn: withQueryAccessBoundary(read) });
    const settlement = fetching.catch(() => undefined);
    await client.cancelQueries({ queryKey: key });
    client.setQueryData(key, { name: "Newer settings" });
    reject(new ApiError("old denial", 403, "Forbidden"));
    await settlement;
    await Promise.resolve();
    expect(client.getQueryCache().find({ queryKey: key, exact: true })).toBe(query);
    expect(client.getQueryData(key)).toEqual({ name: "Newer settings" });
  });

  it("does not revoke a newer session's snapshot for a stale credential response", async () => {
    const error = new ApiError("previous credential", 401, "Unauthorized", undefined, () => false);
    client.setQueryData(key, { name: "Current session settings" });
    await expect(client.fetchQuery({
      queryKey: key,
      queryFn: withQueryAccessBoundary(() => Promise.reject(error)),
    })).rejects.toBe(error);
    expect(client.getQueryData(key)).toEqual({ name: "Current session settings" });
  });
});
