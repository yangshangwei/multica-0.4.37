// @vitest-environment jsdom
import { focusManager, onlineManager, QueryObserver, type QueryClient } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import { createQueryClient } from "../query-client";
import type { McpServerTemplate } from "../types";
import { mcpServerTemplateListOptions, workspaceKeys } from "./queries";

vi.mock("../api", () => ({ api: { listMcpServerTemplates: vi.fn(), getBaseUrl: vi.fn(() => "https://first.test") } }));

function template(version: string): McpServerTemplate {
  return { key: "search", title: "Search", description: "", config: {}, source: "deployment", transport: "http", version };
}

describe("deployment MCP catalog queries", () => {
  let client: QueryClient;
  let observer: QueryObserver<McpServerTemplate[], Error, McpServerTemplate[], McpServerTemplate[], ReturnType<typeof workspaceKeys.mcpServerTemplates>>;
  let unsubscribe: () => void;

  beforeEach(() => {
    vi.useFakeTimers();
    focusManager.setFocused(true);
    onlineManager.setOnline(true);
    vi.mocked(api.getBaseUrl).mockReturnValue("https://first.test");
    vi.mocked(api.listMcpServerTemplates).mockResolvedValue([]);
    client = createQueryClient();
    client.mount();
    observer = new QueryObserver(client, mcpServerTemplateListOptions("ws-1", "zh", { poll: true }));
    unsubscribe = observer.subscribe(() => {});
  });

  afterEach(() => {
    unsubscribe?.();
    observer?.destroy();
    client?.unmount();
    client?.clear();
    focusManager.setFocused(undefined);
    onlineManager.setOnline(true);
    vi.useRealTimers();
    vi.resetAllMocks();
  });

  it("partitions data by API server, workspace and language", async () => {
    await vi.advanceTimersByTimeAsync(1);
    const first = mcpServerTemplateListOptions("ws-1", "zh").queryKey;
    expect(first).toContain("https://first.test");
    expect(first).not.toEqual(mcpServerTemplateListOptions("ws-2", "zh").queryKey);
    expect(first).not.toEqual(mcpServerTemplateListOptions("ws-1", "en").queryKey);
    vi.mocked(api.getBaseUrl).mockReturnValue("https://second.test");
    expect(first).not.toEqual(mcpServerTemplateListOptions("ws-1", "zh").queryKey);
    expect(api.listMcpServerTemplates).toHaveBeenCalledWith("ws-1", "zh", expect.any(AbortSignal));
  });

  it("keeps an old observer bound to the API instance named in its key", async () => {
    await vi.advanceTimersByTimeAsync(1);
    const options = mcpServerTemplateListOptions("ws-2", "en");
    const firstApi = api.listMcpServerTemplates;
    api.listMcpServerTemplates = vi.fn().mockResolvedValue([template("other-server")]);
    vi.mocked(api.getBaseUrl).mockReturnValue("https://second.test");
    try {
      await expect(client.fetchQuery(options)).resolves.toEqual([]);
      await expect(client.fetchQuery(mcpServerTemplateListOptions("ws-2", "en"))).resolves.toEqual([template("other-server")]);
    } finally {
      api.listMcpServerTemplates = firstApi;
    }
  });

  it("discovers additions, changes and removal every 30 seconds", async () => {
    await vi.advanceTimersByTimeAsync(1);
    for (const catalog of [[template("one")], [template("two")], []]) {
      vi.mocked(api.listMcpServerTemplates).mockResolvedValue(catalog);
      await vi.advanceTimersByTimeAsync(30_000);
      expect(observer.getCurrentResult().data).toEqual(catalog);
    }
    expect(api.listMcpServerTemplates).toHaveBeenCalledTimes(4);
  });

  it.each(["mount", "focus", "reconnect"])("refreshes on %s despite global infinite freshness", async (trigger) => {
    await vi.advanceTimersByTimeAsync(1);
    vi.mocked(api.listMcpServerTemplates).mockResolvedValue([template("new")]);
    if (trigger === "mount") {
      unsubscribe();
      unsubscribe = observer.subscribe(() => {});
    } else if (trigger === "focus") {
      focusManager.setFocused(false);
      focusManager.setFocused(true);
    } else {
      onlineManager.setOnline(false);
      onlineManager.setOnline(true);
    }
    await vi.advanceTimersByTimeAsync(1);
    expect(observer.getCurrentResult().data).toEqual([template("new")]);
    expect(api.listMcpServerTemplates).toHaveBeenCalledTimes(2);
  });

  it("stops polling while hidden, opted out or unmounted", async () => {
    await vi.advanceTimersByTimeAsync(1);
    focusManager.setFocused(false);
    await vi.advanceTimersByTimeAsync(90_000);
    expect(api.listMcpServerTemplates).toHaveBeenCalledTimes(1);
    focusManager.setFocused(true);
    await vi.advanceTimersByTimeAsync(1);
    expect(api.listMcpServerTemplates).toHaveBeenCalledTimes(2);
    observer.setOptions(mcpServerTemplateListOptions("ws-1", "zh"));
    await vi.advanceTimersByTimeAsync(90_000);
    expect(api.listMcpServerTemplates).toHaveBeenCalledTimes(2);
    observer.setOptions(mcpServerTemplateListOptions("ws-1", "zh", { poll: true }));
    unsubscribe();
    await vi.advanceTimersByTimeAsync(90_000);
    expect(api.listMcpServerTemplates).toHaveBeenCalledTimes(2);
  });

  it("retains cached templates during catalog failure and recovers on a later poll", async () => {
    await vi.advanceTimersByTimeAsync(1);
    vi.mocked(api.listMcpServerTemplates).mockResolvedValue([template("cached")]);
    await observer.refetch();
    vi.mocked(api.listMcpServerTemplates).mockRejectedValue(new Error("unavailable"));
    await vi.advanceTimersByTimeAsync(31_001);
    expect(observer.getCurrentResult().isError).toBe(true);
    expect(observer.getCurrentResult().data).toEqual([template("cached")]);
    vi.mocked(api.listMcpServerTemplates).mockResolvedValue([template("recovered")]);
    await vi.advanceTimersByTimeAsync(30_000);
    expect(observer.getCurrentResult().data).toEqual([template("recovered")]);
    expect(observer.getCurrentResult().isSuccess).toBe(true);
  });
});
