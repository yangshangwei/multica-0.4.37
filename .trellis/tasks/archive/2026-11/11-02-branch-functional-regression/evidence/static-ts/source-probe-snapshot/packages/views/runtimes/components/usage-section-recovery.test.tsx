import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { AgentRuntime, RuntimeUsage } from "@multica/core/types";
import { runtimeKeys } from "@multica/core/runtimes/queries";
import { renderWithI18n } from "../../test/i18n";
import { UsageSection } from "./usage-section";

const api = vi.hoisted(() => ({
  getRuntimeUsage: vi.fn(),
  getRuntimeUsageByAgent: vi.fn(),
  listAgents: vi.fn(),
}));
vi.mock("@multica/core/api", () => ({ api }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("../../common/use-viewing-timezone", () => ({ useViewingTimezone: () => "UTC" }));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => null }));
vi.mock("./charts", () => ({
  DailyCostChart: () => <div data-testid="daily-cost" />,
  DailyTokensChart: () => null,
  WeeklyCostChart: () => <div data-testid="weekly-cost" />,
  WeeklyTokensChart: () => null,
  ActivityHeatmap: () => null,
}));
vi.mock("./custom-pricing-dialog", () => ({ CustomPricingDialog: () => null }));
vi.mock("@multica/core/runtimes/custom-pricing-store", () => {
  const state = { pricings: {} };
  return {
    useCustomPricingStore: Object.assign((select: (value: typeof state) => unknown) => select(state), { getState: () => state }),
    getCustomPricing: () => undefined,
  };
});

const runtime: AgentRuntime = {
  id: "r-1", workspace_id: "ws-1", daemon_id: null, name: "test-runtime",
  runtime_mode: "cloud", provider: "claude", launch_header: "", status: "online",
  device_info: "", metadata: {}, owner_id: null, visibility: "private", last_seen_at: null,
  created_at: "2026-05-01T00:00:00Z", updated_at: "2026-05-01T00:00:00Z",
};
const rows: RuntimeUsage[] = [{
  runtime_id: "r-1", date: new Date().toISOString().slice(0, 10), provider: "anthropic",
  model: "claude-sonnet-4-6", input_tokens: 1_000_000, output_tokens: 0,
  cache_read_tokens: 0, cache_write_tokens: 0,
}];
const key = runtimeKeys.usage(runtime.id, 180, "UTC");
let client: QueryClient;

function mount() {
  return renderWithI18n(<QueryClientProvider client={client}><UsageSection runtime={runtime} /></QueryClientProvider>);
}
beforeEach(() => {
  vi.clearAllMocks();
  client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  api.getRuntimeUsageByAgent.mockResolvedValue([]);
  api.listAgents.mockResolvedValue([]);
});
afterEach(() => { cleanup(); client.clear(); });

describe("Runtime usage recovery", () => {
  it("shows loading rather than no usage before the initial read", () => {
    api.getRuntimeUsage.mockReturnValue(new Promise(() => {}));
    mount();
    expect(screen.getByRole("status")).toHaveTextContent("Loading usage");
    expect(screen.queryByText("No usage data yet")).toBeNull();
  });

  it("retains cold Retry during recovery and shows empty only after a successful read", async () => {
    api.getRuntimeUsage.mockRejectedValueOnce(new Error("offline"));
    let resolve!: (data: RuntimeUsage[]) => void;
    api.getRuntimeUsage.mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
    mount();
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not load usage");
    expect(screen.queryByText("No usage data yet")).toBeNull();
    const retry = screen.getByRole("button", { name: "Retry" });
    fireEvent.click(retry);
    await waitFor(() => expect(retry).toBeDisabled());
    expect(screen.getByRole("button", { name: "Retrying..." })).toBe(retry);
    fireEvent.click(retry);
    expect(api.getRuntimeUsage).toHaveBeenCalledTimes(2);
    await act(async () => resolve([]));
    expect(await screen.findByText("No usage data yet")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("keeps cached charts and the selected period when refresh fails", async () => {
    client.setQueryData(key, rows);
    api.getRuntimeUsage.mockRejectedValue(new Error("offline"));
    mount();
    fireEvent.click(screen.getByRole("button", { name: "Weekly" }));
    fireEvent.click(screen.getByRole("button", { name: "90d" }));
    const chart = screen.getByTestId("weekly-cost");
    const period = screen.getByRole("button", { name: "90d" });
    await act(async () => { await client.invalidateQueries({ queryKey: key }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh usage");
    expect(screen.getByTestId("weekly-cost")).toBe(chart);
    expect(screen.getByRole("button", { name: "90d" })).toBe(period);
    expect(screen.getByText("Cost · 90D")).toBeInTheDocument();
    expect(screen.queryByText("No usage data yet")).toBeNull();
  });

  it("keeps known empty usage with a refresh warning", async () => {
    client.setQueryData(key, []);
    api.getRuntimeUsage.mockRejectedValue(new Error("offline"));
    mount();
    await act(async () => { await client.invalidateQueries({ queryKey: key }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh usage");
    expect(screen.getByText("No usage data yet")).toBeInTheDocument();
  });

  it("distinguishes a pending or failed agent breakdown from zero agents and retries locally", async () => {
    client.setQueryData(key, rows);
    let reject!: (error: Error) => void;
    api.getRuntimeUsageByAgent.mockImplementationOnce(() => new Promise((_resolve, fail) => { reject = fail; }));
    mount();
    expect(screen.getByRole("status")).toHaveTextContent("Loading usage by agent");
    expect(screen.queryByText("No usage in this period.")).toBeNull();
    await act(async () => reject(new Error("offline")));
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not load usage by agent");
    expect(screen.queryByText("0 agents on this runtime")).toBeNull();
    expect(screen.getByTestId("daily-cost")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
    expect(api.getRuntimeUsageByAgent).toHaveBeenCalledTimes(2);
    expect(api.getRuntimeUsage).not.toHaveBeenCalled();
  });

  it("retains cached agent attribution through a failed refresh", async () => {
    client.setQueryData(key, rows);
    const agentKey = runtimeKeys.usageByAgent(runtime.id, 30, "UTC");
    client.setQueryData(agentKey, [{ ...rows[0], agent_id: "agent-one", task_count: 1 }]);
    api.getRuntimeUsageByAgent.mockRejectedValue(new Error("offline"));
    mount();
    const agent = screen.getByText("agent-one");
    await act(async () => { await client.invalidateQueries({ queryKey: agentKey }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh usage by agent");
    expect(screen.getByText("agent-one")).toBe(agent);
    expect(screen.getByTestId("daily-cost")).toBeInTheDocument();
  });
});
