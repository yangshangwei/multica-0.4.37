import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminOverviewPage } from "./overview-page";
const state = vi.hoisted(() => ({ error: false }));
vi.mock("@multica/core/admin", async original => ({ ...await original<typeof import("@multica/core/admin")>(), useAdminOverview: () => ({ isPending: false, isError: state.error, refetch: vi.fn(), data: {
  asOf: "2026-10-02T00:00:00Z", dataQuality: "partial", ruleVersion: "1", window: { timeFrom: "2026-10-01T00:00:00Z", timeTo: "2026-10-02T00:00:00Z", timezone: "Asia/Shanghai" },
  installations: { total: 2, retired: 0, clientActive: null, daemonReachable: null, ready: null, unassociated: 1 },
  executions: { completed: 0, failed: 0, cancelled: 1, unfinished: 2, queued: 1, running: 1, successRate: null, queueSeconds: { p50: null, p95: null, samples: 0, lowerBoundSamples: 1, unknownSamples: 1 }, runSeconds: { p50: null, p95: null, samples: 0 } },
  usage: { totalTokens: null, inputTokens: null, outputTokens: null, cacheReadTokens: null, cacheWriteTokens: null, missingTasks: 1, unpricedTasks: 1, quality: "unknown" }, alerts: { open: 1, acknowledged: 0, resolved: 2, closed: 3 },
} }) }));
function mount() { return render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><NavigationProvider value={{ pathname: "/admin", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}><AdminOverviewPage /></NavigationProvider></I18nProvider>); }
it("shows no-sample outcomes and unknown usage with a finished-window drilldown", () => {
  state.error = false; mount();
  expect(screen.getByText("No samples")).toBeInTheDocument();
  expect(screen.getByText(/not a bill/)).toBeInTheDocument();
  const href = screen.getByRole("link", { name: /Failed/ }).getAttribute("href")!;
  expect(new URL(href, "https://test.invalid").searchParams.get("time_basis")).toBe("finished");
  expect(new URL(href, "https://test.invalid").searchParams.get("timezone")).toBe("Asia/Shanghai");
  const liveParams = new URL(screen.getByRole("link", { name: "Queued" }).getAttribute("href")!, "https://test.invalid").searchParams;
  expect(liveParams.get("state_scope")).toBe("current");
  expect(liveParams.get("timezone")).toBe("Asia/Shanghai");
  expect(liveParams.has("time_from")).toBe(false);
});
it("does not present stale cached numbers as a successful source read", () => {
  state.error = true; mount();
  expect(screen.getByRole("alert")).toHaveTextContent("Data could not be loaded");
  expect(screen.queryByText("Execution success rate")).not.toBeInTheDocument();
});
it("pins inactive alert history to first-seen window while active alerts retain all ages", () => {
  state.error = false; mount();
  for (const [label, status] of [["Recovered alerts", "resolved"], ["Closed alerts", "closed"]]) {
    const href = screen.getByRole("link", { name: label }).getAttribute("href")!;
    const params = new URL(href, "https://test.invalid").searchParams;
    expect(params.get("status")).toBe(status);
    expect(params.get("time_from")).toBe("2026-10-01T00:00:00Z");
    expect(params.get("time_to")).toBe("2026-10-02T00:00:00Z");
    expect(params.get("timezone")).toBe("Asia/Shanghai");
  }
  for (const label of ["Open alerts", "Acknowledged alerts"]) {
    const href = screen.getByRole("link", { name: label }).getAttribute("href")!;
    expect(new URL(href, "https://test.invalid").searchParams.has("time_from")).toBe(false);
    expect(new URL(href, "https://test.invalid").searchParams.get("timezone")).toBe("Asia/Shanghai");
  }
  expect(screen.getByRole("heading", { name: "Alert history first observed in this window" })).toBeInTheDocument();
});
