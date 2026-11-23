import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { api } from "@multica/core/api";
import { IterationHistory } from "./iteration-history";
import { source, statistics, ws } from "./test-fixtures";
import projects from "../locales/en/projects.json";

const chart = vi.hoisted(() => vi.fn(() => null));

vi.mock("../i18n", () => ({
  useLocale: () => "en",
  useT: () => ({ t: (select: (labels: typeof projects) => string) => select(projects) }),
}));
vi.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  LineChart: chart,
  Line: () => null,
  XAxis: () => null,
  YAxis: () => null,
  Tooltip: () => null,
}));
vi.mock("./iteration-catalogue", () => ({
  useIterationCatalogue: () => ({ data: [] }),
  IterationReference: () => null,
}));
vi.mock("./iteration-events-view", () => ({ IterationEventsView: () => null }));

describe("iteration chart data closeout", () => {
  it("keeps every chart and table value frozen when current statistics change", async () => {
    const user = userEvent.setup();
    const frozen = {
      ...statistics,
      chart: [
        { date: "2026-10-01", effective: 12, completed: 3, original: 11 },
        { date: "2026-10-02", effective: 14, completed: 9, original: 11 },
      ],
    };
    const snapshot: NonNullable<Awaited<ReturnType<typeof api.getIteration>>["snapshot"]> = {
      schema_version: 1,
      workspace_id: ws,
      iteration_id: source.id,
      operation_id: ws,
      end_type: "completed",
      reason: "Finished",
      logical_ended_at: "2026-10-02T12:00:00Z",
      processed_at: "2026-10-02T12:00:00Z",
      original: [],
      scope: [],
      events: [],
      statistics: frozen,
      destinations: [],
    };
    const { rerender } = render(
      <IterationHistory statistics={statistics} snapshot={snapshot} timezone="UTC" />,
    );
    const disclosure = screen.getByText("View chart data", { selector: "summary" });
    disclosure.focus();
    expect(disclosure).toHaveFocus();
    await user.click(disclosure);
    const table = screen.getByRole("table", { name: "Chart data" });
    expect(table).toBeVisible();
    expect(within(table).getAllByRole("columnheader").map((cell) => cell.textContent)).toEqual([
      "Date", "Effective scope", "Completed", "Original commitment",
    ]);
    const rows = () => within(table).getAllByRole("row").slice(1).map((row) => [
      within(row).getByRole("rowheader").textContent,
      ...within(row).getAllByRole("cell").map((cell) => cell.textContent),
    ]);
    expect(rows()).toEqual([
      ["2026-10-01", "12", "3", "11"],
      ["2026-10-02", "14", "9", "11"],
    ]);
    expect(chart).toHaveBeenLastCalledWith(expect.objectContaining({ data: frozen.chart }), undefined);

    rerender(<IterationHistory
      statistics={{ ...statistics, chart: [{ date: "2026-10-03", effective: 99, completed: 98, original: 97 }] }}
      snapshot={snapshot}
      timezone="UTC"
    />);

    expect(rows()).toEqual([
      ["2026-10-01", "12", "3", "11"],
      ["2026-10-02", "14", "9", "11"],
    ]);
    expect(chart).toHaveBeenLastCalledWith(expect.objectContaining({ data: frozen.chart }), undefined);
    expect(table).toBeVisible();
    expect(within(table).queryByRole("rowheader", { name: "2026-10-03" })).not.toBeInTheDocument();
  });
});
