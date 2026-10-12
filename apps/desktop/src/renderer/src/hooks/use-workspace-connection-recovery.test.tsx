import { StrictMode, type ReactNode } from "react";
import { act, cleanup, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "@multica/core/api";
import { useWorkspaceList } from "@multica/core/workspace";
import { workspaceKeys } from "@multica/core/workspace/queries";
import type { Workspace } from "@multica/core/types";
import { useWorkspaceConnectionRecovery } from "./use-workspace-connection-recovery";

vi.mock("@multica/core/api", () => ({ api: { listWorkspaces: vi.fn() } }));
let client: QueryClient;

function mount({ retry = false, strict = false }: { retry?: boolean; strict?: boolean } = {}) {
  client = new QueryClient({
    defaultOptions: { queries: { retry: retry ? 1 : false, retryDelay: 50, gcTime: Infinity } },
  });
  const Wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>
      {strict ? <StrictMode>{children}</StrictMode> : children}
    </QueryClientProvider>
  );
  return renderHook(({ enabled }) => {
    const list = useWorkspaceList({ enabled });
    return { list, ...useWorkspaceConnectionRecovery(list, enabled) };
  }, { initialProps: { enabled: true }, wrapper: Wrapper });
}

async function tick(ms: number) {
  await act(async () => { await vi.advanceTimersByTimeAsync(ms); });
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(1_792_000_000_000);
  vi.mocked(api.listWorkspaces).mockReset().mockRejectedValue(new TypeError("private-network-error"));
});
afterEach(() => { cleanup(); client?.clear(); vi.useRealTimers(); });

describe("useWorkspaceConnectionRecovery", () => {
  it("schedules an actual retry deadline and recovers without a WebSocket", async () => {
    vi.mocked(api.listWorkspaces).mockRejectedValueOnce(new TypeError("offline")).mockResolvedValue([]);
    const { result } = mount({ strict: true });
    await tick(5);
    expect(result.current.recovery?.failure?.kind).toBe("network");
    const nextRetryAt = result.current.recovery?.nextRetryAt;
    expect(nextRetryAt).toBeGreaterThan(Date.now());
    expect(api.listWorkspaces).toHaveBeenCalledOnce();
    await tick(nextRetryAt! - Date.now() + 5);
    expect(api.listWorkspaces).toHaveBeenCalledTimes(2);
    expect(result.current.list.ready).toBe(true);
    expect(result.current.recovery).toBeNull();
    await tick(60_000);
    expect(api.listWorkspaces).toHaveBeenCalledTimes(2);
  });

  it("lets Query finish its own retry before scheduling another attempt", async () => {
    const { result } = mount({ retry: true });
    await tick(5);
    expect(result.current.recovery).toBeNull();
    expect(result.current.list.isFetching).toBe(true);
    await tick(60);
    expect(api.listWorkspaces).toHaveBeenCalledTimes(2);
    expect(result.current.recovery?.nextRetryAt).toBeGreaterThan(Date.now());
    await tick(500);
    expect(api.listWorkspaces).toHaveBeenCalledTimes(2);
  });

  it("cancels a scheduled retry for a manual attempt and rejects overlapping clicks", async () => {
    const { result } = mount();
    await tick(5);
    let settle!: (value: Workspace[]) => void;
    vi.mocked(api.listWorkspaces).mockImplementation(() => new Promise((resolve) => { settle = resolve; }));
    act(() => { for (let index = 0; index < 8; index += 1) result.current.retry(); });
    expect(api.listWorkspaces).toHaveBeenCalledTimes(2);
    expect(result.current.recovery?.isRetrying).toBe(true);
    expect(result.current.recovery?.nextRetryAt).toBeNull();
    await tick(30_000);
    expect(api.listWorkspaces).toHaveBeenCalledTimes(2);
    await act(async () => settle([]));
    await tick(5);
    expect(result.current.recovery).toBeNull();
  });

  it("clears retry timers on logout and disposal", async () => {
    const { result, rerender, unmount } = mount();
    await tick(5);
    expect(result.current.recovery).not.toBeNull();
    rerender({ enabled: false });
    expect(result.current.recovery).toBeNull();
    await tick(60_000);
    expect(api.listWorkspaces).toHaveBeenCalledOnce();
    unmount();
    await tick(60_000);
    expect(api.listWorkspaces).toHaveBeenCalledOnce();
  });

  it("keeps a cached successful list usable after a background failure", async () => {
    const { result } = mount();
    act(() => client.setQueryData(workspaceKeys.list(), []));
    await tick(5);
    expect(result.current.list.ready).toBe(true);
    expect(result.current.list.workspaces).toEqual([]);
    expect(result.current.recovery).toBeNull();
    await tick(60_000);
    expect(api.listWorkspaces).toHaveBeenCalledOnce();
  });
});
