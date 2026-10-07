import { render, screen, within } from "@testing-library/react";
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
  it("exposes every plotted value in a labelled data table", () => {
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
    expect(within(table).getAllByRole("row")).toHaveLength(3);
    expect(within(table).getByText("2026-10-02")).toBeInTheDocument();
    expect(within(table).getByText("2")).toBeInTheDocument();
  });
  it("shows current scope and every scope-change counter", () => {
    render(<IterationHistory statistics={{ ...statistics, current: 9, added_unique: 2, removed_events: 1, reentry_events: 3, cancel_events: 4, reopen_events: 5, started: 6, net_effective_change: -2 }} snapshot={null} timezone="UTC" />);
    for (const [label, value] of [["Current scope", "9"], ["Added tasks", "2"], ["Removal events", "1"], ["Re-entry events", "3"], ["Cancellation events", "4"], ["Reopen events", "5"], ["Started tasks", "6"], ["Net effective scope change", "-2"]]) {
      expect(screen.getAllByText(label!).find((node) => node.tagName === "DT")!.parentElement).toHaveTextContent(value!);
    }
  });
  it("formats closure time on the saved iteration clock rather than raw UTC microseconds", () => {
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
    expect(screen.getByText(/Asia\/Shanghai/)).toBeInTheDocument();
    expect(screen.getAllByText("Current scope").find((node) => node.tagName === "DT")!.parentElement).toHaveTextContent(String(statistics.current));
    expect(screen.queryByText("99")).not.toBeInTheDocument();
    expect(screen.getByText("Finished")).toBeInTheDocument();
    expect(screen.getByText("Processed at").parentElement).toHaveTextContent("Oct 7");
    expect(screen.getByText(/2 → 3/)).toBeInTheDocument();
    expect(screen.getByText(/Rolled over at least three times/)).toBeInTheDocument();
  });
});

vi.mock("./iteration-catalogue", () => ({ useIterationCatalogue: () => ({ data: [] }), IterationReference: ({ id }: { id: string | null }) => <span>{id ?? "Unassigned"}</span> }));
vi.mock("./iteration-events-view", () => ({ IterationEventsView: () => null }));
