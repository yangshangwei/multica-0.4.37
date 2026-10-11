import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import { useProjectAccessStore } from "@multica/core/projects";
import { p1Overview, p1Project } from "@multica/core/projects/test-fixtures/p1";
import type { Project, ProjectStatisticsSnapshot } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { ProjectOverviewPanel } from "./project-overview";

const project: Project = { ...p1Project, status: "in_progress", priority: "medium", lead_type: "member" };
const riskControls = [
  ["blocked", "Blocked"],
  ["overdue", "Overdue"],
  ["unassigned", "Unassigned"],
  ["in_review", "Awaiting review"],
] as const;
let qc: QueryClient;

beforeEach(() => {
  // Project query options retain their retry policy without the default one-second delay.
  qc = new QueryClient({ defaultOptions: { queries: { retry: false, retryDelay: 0, staleTime: Infinity } } });
  useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} });
});

afterEach(() => {
  qc.clear();
  vi.restoreAllMocks();
});

function installOverview(statistics: Partial<ProjectStatisticsSnapshot> = {}) {
  const getProjectOverview = vi.fn().mockResolvedValue({
    ...p1Overview,
    statistics: { ...p1Overview.statistics, reasons: [], ...statistics },
  });
  setApiInstance({ getProjectOverview } as unknown as ApiClient);
  return getProjectOverview;
}

function renderOverview(overrides: Partial<Project> = {}) {
  const onRisk = vi.fn();
  renderWithI18n(
    <QueryClientProvider client={qc}>
      <ProjectOverviewPanel
        project={{ ...project, ...overrides }}
        updatesSupported={false}
        onRisk={onRisk}
        onProtectedError={vi.fn()}
      />
    </QueryClientProvider>,
  );
  return onRisk;
}

async function healthSection() {
  const heading = await screen.findByRole("heading", { name: "System health" });
  const section = heading.closest("section");
  if (!section) throw new Error("System health section is missing");
  return section;
}

describe("project health presentation", () => {
  it("keeps each risk control connected to its signal and the displayed snapshot", async () => {
    installOverview({
      snapshot_version: "displayed-snapshot",
      counts: { ...p1Overview.statistics.counts, blocked: 2, overdue: 1, unassigned: 0, in_review: 3 },
    });
    const onRisk = renderOverview();
    const section = await healthSection();
    const user = userEvent.setup();

    for (const [signal, label] of riskControls) {
      const button = within(section).getByRole("button", { name: new RegExp(label) });
      expect(button.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
      await user.click(button);
      expect(onRisk).toHaveBeenLastCalledWith(signal, "displayed-snapshot");
    }
    expect(onRisk).toHaveBeenCalledTimes(4);
    expect(within(section).getByRole("button", { name: "Unassigned 0" })).toBeEnabled();
    expect(within(section).getByRole("heading").querySelector("svg")).toHaveClass("lucide-activity", "text-muted-foreground");
  });

  it("discloses task-risk explanations while keeping the four risk controls visible", async () => {
    installOverview({ reasons: ["blocked_issues", "overdue_issues", "unassigned_issues", "in_review_issues"] });
    renderOverview();
    const section = await healthSection();
    const details = within(section).getByText("View assessment details").closest("details");
    expect(details).not.toHaveAttribute("open");
    expect(within(section).getByText("Some issues are blocked.")).not.toBeVisible();
    for (const [, label] of riskControls) {
      expect(within(section).getByRole("button", { name: new RegExp(label) })).toBeVisible();
    }

    await userEvent.setup().click(within(section).getByText("View assessment details"));
    expect(details).toHaveAttribute("open");
    expect(within(section).getByText("Some issues are blocked.")).toBeVisible();
    expect(within(section).getByText("Some issues await review.")).toBeVisible();
  });

  it("keeps every project warning visible and replaces repeated generic reasons with actionable copy", async () => {
    installOverview({
      project_overdue: true,
      lead_valid: false,
      counts: { ...p1Overview.statistics.counts, execution_environment_unavailable: 1 },
      reasons: [
        "blocked_issues", "project_overdue", "invalid_project_lead", "stale_progress",
        "ended_project_open_issues", "execution_environment_unavailable", "new_server_reason",
        "invalid_project_lead", "new_server_reason",
      ],
    });
    renderOverview({ start_date: "2026-10-11", due_date: "2026-10-10", in_progress_since_source: "migration" });
    const section = await healthSection();
    const warnings = [
      "The project is past its due date.",
      "The project lead is unavailable. Choose a current member or agent in Properties.",
      "No progress has been published for seven planning days.",
      "This ended project still has open issues.",
      "Some assigned execution environments are unavailable. Review the project resources and squads.",
      "Start date is after due date. Correct the project dates.",
      "Historical start time is unknown; progress age is measured from the upgrade.",
      "Needs attention",
    ];
    for (const warning of warnings) {
      expect(within(section).getAllByText(warning)).toHaveLength(1);
      expect(within(section).getByText(warning)).toBeVisible();
    }
    expect(within(section).queryByText("The project is overdue.")).not.toBeInTheDocument();
    expect(within(section).queryByText("The project lead is unavailable.")).not.toBeInTheDocument();
    expect(within(section).queryByText("An assigned execution environment is unavailable.")).not.toBeInTheDocument();
  });

  it.each([
    ["clear", "No current risk signals", "lucide-circle-check", "text-foreground"],
    ["risk", "Risk", "lucide-triangle-alert", "text-destructive-foreground"],
    ["attention", "Needs attention", "lucide-circle-alert", "text-warning-foreground"],
    ["empty", "No formal issues yet", "lucide-circle-dashed", "text-muted-foreground"],
    ["unavailable", "Statistics are incomplete", "lucide-circle-question-mark", "text-muted-foreground"],
  ] as const)("presents %s with its own state icon and semantic tone", async (health, label, icon, tone) => {
    installOverview({ health, complete: health !== "unavailable" });
    renderOverview();
    const section = await healthSection();
    const status = within(section).getByText(label);
    expect(status).toHaveClass(tone);
    expect(status.querySelector("svg")).toHaveClass(icon);
    expect(within(section).getByRole("heading").querySelector("svg")).toHaveClass("lucide-activity", "text-muted-foreground");
  });

  it("keeps zero and unknown counts neutral and blocks incomplete-snapshot navigation", async () => {
    installOverview({
      health: "clear",
      complete: false,
      incomplete_reasons: ["unknown_status"],
      counts: { ...p1Overview.statistics.counts, blocked: 0, overdue: null },
    });
    const onRisk = renderOverview();
    const section = await healthSection();
    expect(within(section).queryByText("No current risk signals")).not.toBeInTheDocument();
    expect(within(section).getByText("Statistics are incomplete")).toHaveClass("text-muted-foreground");
    expect(within(section).getByText("Some issue statuses cannot be classified.")).toBeVisible();
    for (const [, label] of riskControls) {
      const button = within(section).getByRole("button", { name: new RegExp(label) });
      expect(button).toBeDisabled();
      expect(button.querySelector("svg")).not.toHaveClass("text-destructive", "text-warning-foreground");
      await userEvent.setup().click(button);
    }
    expect(within(section).getByRole("button", { name: "Blocked 0" })).toBeVisible();
    expect(within(section).getByRole("button", { name: "Overdue Not available" })).toBeVisible();
    expect(onRisk).not.toHaveBeenCalled();
  });

  it("removes the healthy check and pauses risk links after a cached overview fails to refresh", async () => {
    const getProjectOverview = installOverview({ health: "clear" });
    const onRisk = renderOverview();
    const section = await healthSection();
    expect(within(section).getByText("No current risk signals")).toBeVisible();
    getProjectOverview.mockRejectedValue(new Error("Network unavailable"));

    await userEvent.setup().click(screen.getByRole("button", { name: "Refresh" }));
    await screen.findByText("Refresh failed. These are the last available statistics.");
    expect(within(section).queryByText("No current risk signals")).not.toBeInTheDocument();
    expect(section.querySelector(".text-success")).not.toBeInTheDocument();
    expect(section.querySelector(".lucide-circle-check-big")).not.toBeInTheDocument();
    for (const [, label] of riskControls) {
      expect(within(section).getByRole("button", { name: new RegExp(label) })).toBeDisabled();
    }
    expect(onRisk).not.toHaveBeenCalled();
  });
});
