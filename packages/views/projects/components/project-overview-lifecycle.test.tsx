import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import type { Project } from "@multica/core/types";
import { useProjectAccessStore } from "@multica/core/projects";
import { p1Project, p1Overview } from "@multica/core/projects/test-fixtures/p1";
import { renderWithI18n } from "../../test/i18n";
import { ProjectOverviewPanel } from "./project-overview";

const project: Project = { ...p1Project, status: "in_progress", priority: "medium", lead_type: "member" };
let qc: QueryClient;
beforeEach(() => { qc = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } }); useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} }); });
afterEach(() => { qc.clear(); vi.useRealTimers(); vi.restoreAllMocks(); });
function render() { return renderWithI18n(<QueryClientProvider client={qc}><ProjectOverviewPanel project={project} canEditTimezone={false} updatesSupported={false} onRisk={vi.fn()} onProtectedError={vi.fn()} /></QueryClientProvider>); }
function overviewOn(day: string) { return { ...p1Overview, statistics: { ...p1Overview.statistics, reference_date: day, calculated_at: `${day}T00:00:30Z` } }; }
it("G2 the mounted minute timer and focus recovery refresh real Query data across planning days", async () => {
  vi.useFakeTimers({ toFake: ["Date", "setInterval", "clearInterval"] });
  vi.setSystemTime(new Date("2026-10-05T23:59:30Z"));
  const getProjectOverview = vi.fn().mockResolvedValueOnce(overviewOn("2026-10-05"))
    .mockResolvedValueOnce(overviewOn("2026-10-06")).mockResolvedValueOnce(overviewOn("2026-10-07"));
  setApiInstance({ getProjectOverview } as unknown as ApiClient);
  render(); await screen.findByText(/Reference date 2026-10-05/);
  await act(async () => { await vi.advanceTimersByTimeAsync(60_000); });
  await screen.findByText(/Reference date 2026-10-06/); expect(getProjectOverview).toHaveBeenCalledTimes(2);
  vi.setSystemTime(new Date("2026-10-07T04:00:00Z"));
  act(() => window.dispatchEvent(new Event("focus")));
  await screen.findByText(/Reference date 2026-10-07/); expect(getProjectOverview).toHaveBeenCalledTimes(3);
});
it.each([{ completed: 10, cancelled: 0 }, { completed: 0, cancelled: 10 }])("G3 closed scope never synthesizes acceptance: %j", async (counts) => {
  setApiInstance({ getProjectOverview: vi.fn().mockResolvedValue({ ...p1Overview, latest_acceptance: null, current_description_acceptance: null,
    statistics: { ...p1Overview.statistics, counts: { ...p1Overview.statistics.counts, total: 10, open: 0, ...counts }, closure_ratio: 1 } }) } as unknown as ApiClient);
  render(); await screen.findByText("100%");
  await waitFor(() => expect(screen.getAllByText("No acceptance recorded")).toHaveLength(2));
  expect(screen.queryByText("Passed", { exact: true })).not.toBeInTheDocument();
});
