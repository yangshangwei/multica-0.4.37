import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { IterationHistory } from "./iteration-history";
import { statistics, source, ws } from "./test-fixtures";
import projects from "../locales/en/projects.json";
vi.mock("../i18n", () => ({
  useLocale: () => "en",
  useT: () => ({ t: (fn: (x: typeof projects) => string) => fn(projects) }),
}));
vi.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
  LineChart: () => null,
  Line: () => null,
  XAxis: () => null,
  YAxis: () => null,
  Tooltip: () => null,
}));
describe("iteration chart alternative", () => {
  it("exposes every plotted value through a focusable data disclosure", async () => {
    const user = userEvent.setup();
    const statistics = {
      original: 5,
      current: 4,
      cancelled: 1,
      effective: 3,
      completed: 2,
      original_completed: 2,
      remaining: 1,
      added_unique: 0,
      removed_events: 1,
      reentry_events: 0,
      cancel_events: 1,
      reopen_events: 0,
      started: 3,
      initial_effective: 5,
      net_effective_change: -2,
      net_effective_change_ratio: -0.4,
      effective_ratio: 2 / 3,
      original_ratio: 0.4,
      chart: [
        { date: "2026-10-01", effective: 5, completed: 0, original: 5 },
        { date: "2026-10-02", effective: 3, completed: 2, original: 5 },
      ],
      calculated_at: "2026-10-02T00:00:00Z",
    };
    render(
      <IterationHistory
        statistics={statistics}
        snapshot={null}
        timezone="UTC"
      />,
    );
    expect(
      screen.getByRole("img", { name: "Scope and completion over time" }),
    ).toBeInTheDocument();
    const table = screen.getByRole("table", { name: "Chart data" });
    expect(table).not.toBeVisible();
    const disclosure = screen.getByText("View chart data", { selector: "summary" });
    disclosure.focus();
    expect(disclosure).toHaveFocus();
    await user.click(disclosure);
    expect(table).toBeVisible();
    expect(within(table).getAllByRole("row")).toHaveLength(3);
    expect(within(table).getByText("2026-10-02")).toBeInTheDocument();
    expect(within(table).getByText("2")).toBeInTheDocument();
  });
  it("keeps task totals and the started count available in count details", async () => {
    const user = userEvent.setup();
    render(<IterationHistory statistics={{ ...statistics, current: 9, cancelled: 2, started: 6 }} snapshot={null} timezone="UTC" />);
    const disclosure = screen.getByText("View count details", { selector: "summary" });
    const details = disclosure.closest("details")!;
    expect(within(details).getByText("Started tasks")).not.toBeVisible();
    await user.click(disclosure);
    for (const [label, value] of [["Current scope", "9"], ["Cancelled tasks", "2"], ["Started tasks", "6"]]) {
      expect(within(details).getByText(label!).parentElement).toHaveTextContent(value!);
    }
    expect(within(details).getByText("Started tasks")).toBeVisible();
    // Scope event counters are covered by the Scope Changes panel suite.
    expect(screen.queryByText("Removal events")).not.toBeInTheDocument();
  });
  it.each(["empty denominator", "missing historical rate"])("shows not-applicable rates for %s", (kind) => {
    const values = kind === "empty denominator"
      ? { ...statistics, original: 0, current: 0, effective: 0, remaining: 0 }
      : { ...statistics, original_ratio: null, effective_ratio: null };
    render(<IterationHistory statistics={values} snapshot={null} timezone="UTC" />);
    const summary = screen.getByRole("region", { name: "Delivery summary" });
    expect(within(summary).getAllByText("Not applicable")).toHaveLength(2);
    expect(within(summary).queryByText("0%")).not.toBeInTheDocument();
  });
  it("formats closure time on the saved iteration clock and retains full closeout details", async () => {
    const user = userEvent.setup();
    const closedAt = "2026-10-06T18:05:30.123456Z";
    render(
      <IterationHistory
        statistics={{ ...statistics, current: 99 }}
        timezone="Asia/Shanghai"
        snapshot={{
          schema_version: 1,
          workspace_id: ws,
          iteration_id: source.id,
          operation_id: ws,
          end_type: "completed",
          reason: "Finished",
          logical_ended_at: closedAt,
          processed_at: closedAt,
          original: [],
          scope: [],
          events: [],
          statistics,
          destinations: [{ issue_id: ws, target_iteration_id: source.id, rollover_count_before: 2, rollover_count_after: 3 }],
        }}
      />,
    );
    const time = document.querySelector("time");
    expect(time).toHaveAttribute("datetime", closedAt);
    expect(time).toHaveTextContent("2026");
    expect(time).toHaveTextContent("Oct 7");
    expect(time).toHaveTextContent("02:05");
    expect(time).not.toHaveTextContent("123456");
    expect(time).toHaveAttribute("title", "Asia/Shanghai");
    expect(screen.queryByText(/Asia\/Shanghai/)).not.toBeInTheDocument();
    await user.click(screen.getByText("View count details", { selector: "summary" }));
    expect(screen.getByText("Scope at closure", { selector: "dt" }).parentElement).toHaveTextContent(String(statistics.current));
    expect(screen.queryByText("99")).not.toBeInTheDocument();
    await user.click(screen.getByText("View closure details", { selector: "summary" }));
    expect(screen.getByText("Finished")).toBeInTheDocument();
    expect(screen.getByText("Processed at").parentElement).toHaveTextContent("Oct 7");
    expect(screen.getByText(/2 → 3/)).toBeInTheDocument();
    expect(screen.getByText(/Rolled over at least three times/)).toBeInTheDocument();
    expect(screen.getByText("Some unfinished task destinations were not recorded.")).toBeVisible();
    expect(screen.queryByText("Tasks removed at closure")).not.toBeInTheDocument();
  });
});

vi.mock("./iteration-catalogue", () => ({ useIterationCatalogue: () => ({ data: [] }), IterationReference: ({ id }: { id: string | null }) => <span>{id ?? "Unassigned"}</span> }));
vi.mock("./iteration-events-view", () => ({ IterationEventsView: () => null }));
