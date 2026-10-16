// @vitest-environment node

import { QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import { ApiError } from "../api/client";
import {
  dingtalkAgentGroupsOptions,
  dingtalkAgentInactiveGroupsOptions,
  dingtalkGroupsOptions,
  dingtalkInactiveGroupsOptions,
  dingtalkKeys,
} from "./queries";

vi.mock("../api", () => ({
  api: { listDingTalkGroups: vi.fn(), listAgentDingTalkGroups: vi.fn() },
}));
afterEach(() => { vi.resetAllMocks(); });

describe("dingtalkGroupsOptions", () => {
  it("is workspace-scoped and polls only while discovery is healthy", () => {
    const options = dingtalkGroupsOptions("ws-1");
    expect(options.queryKey).toEqual(dingtalkKeys.groups("ws-1"));
    expect(options.enabled).toBe(true);
    const interval = options.refetchInterval;
    expect(typeof interval).toBe("function");
    if (typeof interval !== "function") return;
    expect(
      interval({
        state: {
          status: "success",
          data: { groups: [], group_discovery_supported: true },
        },
      } as never),
    ).toBe(5_000);
    expect(
      interval({
        state: {
          status: "success",
          data: { groups: [], group_discovery_supported: false },
        },
      } as never),
    ).toBe(false);
    expect(
      interval({
        state: {
          status: "success",
          data: { groups: [] },
        },
      } as never),
    ).toBe(false);
    expect(
      interval({
        state: {
          status: "success",
          data: { groups: [], group_discovery_supported: null },
        },
      } as never),
    ).toBe(false);
    expect(interval({ state: { status: "error" } } as never)).toBe(false);
  });

  it("stays disabled before workspace context is available", () => {
    expect(dingtalkGroupsOptions("").enabled).toBe(false);
  });

  it("separates each Agent detail cache from the admin workspace inventory", () => {
    const options = dingtalkAgentGroupsOptions("ws-1", "agent-1");
    expect(options.queryKey).toEqual(dingtalkKeys.agentGroups("ws-1", "agent-1"));
    expect(options.queryKey).not.toEqual(dingtalkKeys.groups("ws-1"));
    expect(options.enabled).toBe(true);
    expect(dingtalkAgentGroupsOptions("ws-1", "").enabled).toBe(false);
  });

  it("keeps inactive pages scoped to one installation", () => {
    const workspace = dingtalkInactiveGroupsOptions("ws-1", "inst-1");
    expect(workspace.queryKey).toEqual(
      dingtalkKeys.inactiveGroups("ws-1", "inst-1"),
    );
    expect(workspace.initialPageParam).toBe(0);
    expect(workspace.getNextPageParam?.({ next_offset: 20 } as never, [], 0, [])).toBe(20);

    const agent = dingtalkAgentInactiveGroupsOptions("ws-1", "agent-1", "inst-1");
    expect(agent.queryKey).toEqual(
      dingtalkKeys.agentInactiveGroups("ws-1", "agent-1", "inst-1"),
    );
    expect(dingtalkInactiveGroupsOptions("ws-1", "").enabled).toBe(false);
  });

  it.each(["workspace", "agent"] as const)("preserves %s inactive offsets and request scope through the access boundary", async (scope) => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const options = scope === "workspace"
      ? dingtalkInactiveGroupsOptions("ws-1", "inst-1")
      : dingtalkAgentInactiveGroupsOptions("ws-1", "agent-1", "inst-1");
    const read = scope === "workspace" ? api.listDingTalkGroups : api.listAgentDingTalkGroups;
    vi.mocked(read)
      .mockResolvedValueOnce({ group_discovery_supported: true, groups: [], next_offset: 20 })
      .mockResolvedValueOnce({ group_discovery_supported: true, groups: [] });
    try {
      const result = await client.fetchInfiniteQuery({ ...options, pages: 2 });
      expect(result.pageParams).toEqual([0, 20]);
      expect(read).toHaveBeenNthCalledWith(1, scope === "workspace" ? "ws-1" : "agent-1", {
        activity: "inactive", installationId: "inst-1", offset: 0,
      });
      expect(read).toHaveBeenNthCalledWith(2, scope === "workspace" ? "ws-1" : "agent-1", {
        activity: "inactive", installationId: "inst-1", offset: 20,
      });
    } finally {
      client.clear();
    }
  });

  it("evicts the agent-scoped group snapshot when its own read is denied", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const options = dingtalkAgentGroupsOptions("ws-1", "agent-1");
    client.setQueryData(options.queryKey, { group_discovery_supported: true, groups: [] });
    const denial = new ApiError("denied", 403, "Forbidden");
    vi.mocked(api.listAgentDingTalkGroups).mockRejectedValueOnce(denial);
    try {
      await expect(client.fetchQuery(options)).rejects.toBe(denial);
      expect(client.getQueryData(options.queryKey)).toBeUndefined();
      expect(api.listAgentDingTalkGroups).toHaveBeenCalledExactlyOnceWith("agent-1");
    } finally {
      client.clear();
    }
  });
});
