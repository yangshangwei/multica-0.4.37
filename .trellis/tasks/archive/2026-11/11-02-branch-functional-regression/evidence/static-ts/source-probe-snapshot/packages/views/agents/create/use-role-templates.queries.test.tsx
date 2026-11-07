// @vitest-environment jsdom

import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useRoleTemplates, useSquadTemplates } from "./use-role-templates";

const mocks = vi.hoisted(() => ({ roles: vi.fn(), squads: vi.fn(), locale: "en" }));
vi.mock("@multica/core/api", () => ({ api: { listAgentRoleTemplates: mocks.roles, listSquadTemplates: mocks.squads } }));
vi.mock("../../i18n", () => ({ useLocale: () => mocks.locale }));

function wrapperFor(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

describe("role metadata query enablement", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.locale = "en";
    mocks.roles.mockResolvedValue([]);
    mocks.squads.mockResolvedValue([]);
  });

  it("does not request disabled metadata and uses the same locale caches when enabled", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const view = renderHook(({ enabled }) => ({
      roles: useRoleTemplates({ enabled }), squads: useSquadTemplates({ enabled }),
    }), { initialProps: { enabled: false }, wrapper: wrapperFor(client) });
    expect(mocks.roles).not.toHaveBeenCalled();
    expect(mocks.squads).not.toHaveBeenCalled();
    view.rerender({ enabled: true });
    await waitFor(() => expect(view.result.current.roles.isSuccess && view.result.current.squads.isSuccess).toBe(true));
    expect(mocks.roles).toHaveBeenCalledExactlyOnceWith("en");
    expect(mocks.squads).toHaveBeenCalledExactlyOnceWith("en");
    expect(client.getQueryData(["agent-role-templates", "en"])).toEqual([]);
    expect(client.getQueryData(["squad-templates", "en"])).toEqual([]);
  });

  it("preserves default enablement and supported-language query keys for existing callers", async () => {
    mocks.locale = "zh-Hans";
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(() => ({ roles: useRoleTemplates(), squads: useSquadTemplates() }), { wrapper: wrapperFor(client) });
    await waitFor(() => expect(result.current.roles.isSuccess && result.current.squads.isSuccess).toBe(true));
    expect(mocks.roles).toHaveBeenCalledExactlyOnceWith("zh");
    expect(mocks.squads).toHaveBeenCalledExactlyOnceWith("zh");
    expect(client.getQueryData(["agent-role-templates", "zh"])).toEqual([]);
    expect(client.getQueryData(["squad-templates", "zh"])).toEqual([]);
  });

  it("retains cached catalogs through refresh failure and lets retry recover them", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(["agent-role-templates", "en"], []);
    client.setQueryData(["squad-templates", "en"], []);
    const { result } = renderHook(() => ({ roles: useRoleTemplates(), squads: useSquadTemplates() }), { wrapper: wrapperFor(client) });
    expect(mocks.roles).not.toHaveBeenCalled();
    expect(mocks.squads).not.toHaveBeenCalled();
    expect(result.current.roles.isError).toBe(false);
    expect(result.current.squads.isError).toBe(false);
    mocks.roles.mockRejectedValueOnce(new Error("Metadata unavailable"));
    mocks.squads.mockRejectedValueOnce(new Error("Metadata unavailable"));
    await act(async () => { await Promise.all([result.current.roles.refetch(), result.current.squads.refetch()]); });
    await waitFor(() => expect(result.current.roles.isError && result.current.squads.isError).toBe(true));
    expect(result.current.roles.data).toEqual([]);
    expect(result.current.squads.data).toEqual([]);
    await act(async () => { await Promise.all([result.current.roles.refetch(), result.current.squads.refetch()]); });
    await waitFor(() => expect(result.current.roles.isSuccess && result.current.squads.isSuccess).toBe(true));
  });
});
