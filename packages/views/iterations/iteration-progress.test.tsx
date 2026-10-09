import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { IterationProgressChart } from "./iteration-progress";
import { statistics } from "./test-fixtures";
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

beforeEach(() => vi.clearAllMocks());

describe("iteration progress data density", () => {
  it("distinguishes no recorded data from a zero-valued trend", () => {
    render(<IterationProgressChart statistics={statistics} />);
    expect(screen.getByText("No daily progress data was recorded.")).toBeVisible();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    expect(chart).not.toHaveBeenCalled();
    expect(screen.getByText("Effective scope completion").parentElement).toHaveTextContent("0%");
  });

  it("shows a single saved date and its actual values without inventing a trend", () => {
    render(<IterationProgressChart statistics={{
      ...statistics,
      effective: 99,
      completed: 98,
      original: 97,
      chart: [{ date: "2026-10-06", effective: 4, completed: 2, original: 5 }],
    }} />);
    const summary = screen.getByRole("region", { name: "Scope and completion over time" });
    expect(within(summary).getByText("Only one day was recorded.")).toBeVisible();
    expect(summary.querySelector("time")).toHaveAttribute("datetime", "2026-10-06");
    expect(summary.querySelector("time")).toHaveTextContent("Oct 6");
    for (const [label, value] of [["Effective scope", "4"], ["Completed", "2"], ["Original commitment", "5"]]) {
      expect(within(summary).getByText(label!).parentElement).toHaveTextContent(value!);
    }
    expect(within(summary).queryByText("99")).not.toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    expect(chart).not.toHaveBeenCalled();
  });

  // Multi-day chart values and the shared table stay covered by
  // iteration-chart-closeout.test.tsx, including conflicting live data.
});
