import React from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import { projectRiskOptions, useProjectAccessStore } from "@multica/core/projects";
import type { Project } from "@multica/core/types";
import { p1Overview, p1Project } from "@multica/core/projects/test-fixtures/p1";
import { renderWithI18n } from "../../test/i18n";
import { ProjectRiskIssues } from "./project-risk-issues";

vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ issueDetail: (id: string) => `/source/issues/${id}` }) }));
vi.mock("../../navigation", () => ({ AppLink: ({ href, children, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));
const project: Project = { ...p1Project, status: "in_progress", priority: "medium", lead_type: "member" };
const warning = "Data changed while you continued after the previous position. Refresh from the start to include new or re-entered matches before it.";
let qc: QueryClient;
function page(title: string | null, version: string, refreshed: boolean, next: string | null, total = 3) {
  return { workspace_id: project.workspace_id, project_id: project.id, signal: "blocked", total,
    items: title ? [{ id: title, identifier: title, title, status: "blocked", due_date: null }] : [],
    snapshot_version: version, refreshed, overview: { ...p1Overview, statistics: { ...p1Overview.statistics, snapshot_version: version } }, next_cursor: next };
}
function render(fetcher: ReturnType<typeof vi.fn>) {
  setApiInstance({ getProjectRiskIssues: fetcher } as unknown as ApiClient);
  return renderWithI18n(<QueryClientProvider client={qc}><ProjectRiskIssues project={project} signal="blocked" version="A" onBack={vi.fn()} onProtectedError={vi.fn()} /></QueryClientProvider>);
}
beforeEach(() => { qc = new QueryClient({ defaultOptions: { queries: { retry: false, retryDelay: 0, staleTime: Infinity } } }); useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} }); });
afterEach(() => qc.clear());

it("keeps changed-continuation disclosure across later unchanged pages and replaces rows", async () => {
  const fetcher = vi.fn().mockResolvedValueOnce(page("row-050", "A", false, "after050"))
    .mockResolvedValueOnce(page("row-100", "B", true, "after100"))
    .mockResolvedValueOnce(page("row-150", "B", false, "after150"))
    .mockResolvedValueOnce(page("row-200", "B", false, null));
  render(fetcher); await screen.findByRole("link", { name: /row-050/ });
  fireEvent.click(screen.getByRole("button", { name: "Next page" })); await screen.findByRole("link", { name: /row-100/ });
  expect(screen.getByText(warning)).toBeInTheDocument(); expect(screen.queryByRole("link", { name: /row-050/ })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Next page" })); await screen.findByRole("link", { name: /row-150/ });
  expect(screen.getByText(warning)).toBeInTheDocument(); expect(screen.queryByRole("link", { name: /row-100/ })).not.toBeInTheDocument();
  expect(fetcher.mock.calls[2]?.[2]).toEqual({ signal: "blocked", cursor: "after100", version: "B" });
  fireEvent.click(screen.getByRole("button", { name: "Next page" })); await screen.findByRole("link", { name: /row-200/ });
  expect(screen.getByText(warning)).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /row-150/ })).not.toBeInTheDocument();
});
it("a successful restart fetches despite cached first-page data and includes a low-ID re-entry", async () => {
  qc.setQueryData([...projectRiskOptions(project.workspace_id, project.id, "blocked").queryKey], page("stale-cached-first", "old", false, null));
  let finish!: (value: unknown) => void;
  const fetcher = vi.fn().mockResolvedValueOnce(page("row-050", "A", false, "after050"))
    .mockResolvedValueOnce(page("row-100", "B", true, null))
    .mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  render(fetcher); await screen.findByRole("link", { name: /row-050/ });
  fireEvent.click(screen.getByRole("button", { name: "Next page" })); await screen.findByRole("link", { name: /row-100/ });
  fireEvent.click(screen.getByRole("button", { name: "Refresh from start" }));
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(3));
  expect(screen.getByText(warning)).toBeInTheDocument(); expect(screen.queryByRole("link", { name: /stale-cached-first/ })).not.toBeInTheDocument();
  await act(async () => finish(page("row-010-reentered", "C", false, "after010")));
  await screen.findByRole("link", { name: /row-010-reentered/ }); expect(screen.queryByText(warning)).not.toBeInTheDocument();
  expect(fetcher.mock.calls[2]?.[2]).toEqual({ signal: "blocked", cursor: undefined, version: undefined });
});
it("failed restart keeps the current page and sticky disclosure", async () => {
  const fetcher = vi.fn().mockResolvedValueOnce(page("row-050", "A", false, "after050"))
    .mockResolvedValueOnce(page("row-100", "B", true, null)).mockRejectedValue(new Error("Restart unavailable"));
  render(fetcher); await screen.findByRole("link", { name: /row-050/ });
  fireEvent.click(screen.getByRole("button", { name: "Next page" })); await screen.findByRole("link", { name: /row-100/ });
  fireEvent.click(screen.getByRole("button", { name: "Refresh from start" }));
  await screen.findByRole("alert"); expect(screen.getByText(warning)).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /row-100/ })).toBeInTheDocument();
});
it("distinguishes an empty suffix from a project with no matching risk", async () => {
  const fetcher = vi.fn().mockResolvedValueOnce(page("row-050", "A", false, "after050"))
    .mockResolvedValueOnce(page(null, "B", true, null, 2));
  render(fetcher); await screen.findByRole("link", { name: /row-050/ });
  fireEvent.click(screen.getByRole("button", { name: "Next page" }));
  await screen.findByText("No matches after this position. Earlier matches may still exist; refresh from the start to see them.");
  expect(screen.queryByText("No matching issues")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Refresh from start" })).toBeEnabled();
});
it("discloses a changed initial card snapshot without claiming continued or frozen traversal", async () => {
  render(vi.fn().mockResolvedValue(page("new-first-page", "B", true, null)));
  await screen.findByRole("link", { name: /new-first-page/ });
  expect(screen.getByText("The project changed. Counts and results have been refreshed together.")).toBeInTheDocument();
  expect(screen.queryByText(warning)).not.toBeInTheDocument();
});
