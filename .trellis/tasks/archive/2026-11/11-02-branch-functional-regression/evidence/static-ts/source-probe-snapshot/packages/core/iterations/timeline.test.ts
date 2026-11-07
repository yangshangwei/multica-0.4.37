// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { Iteration } from "../api/iteration-schemas";
import { iterationTimeline } from "./timeline";

const period = (id: string, start: string, end: string, fields: Partial<Iteration> = {}): Iteration => ({
  id, workspace_id: "workspace", name: id, description: null, coordinator_user_id: null,
  mode: "manual", status: "planned", start_date: start, end_date: end, timezone: "UTC",
  revision: 1, scope_revision: 1, started_at: null, logical_ended_at: null, processed_at: null,
  ...fields,
});
const early = period("early", "2026-10-01", "2026-10-07", { status: "active" });
const next = period("next", "2026-10-10", "2026-10-16");

describe("iterationTimeline", () => {
  it("orders each group by descending dates and IDs without mutating the catalogue", () => {
    const items = [next, early, period("z", next.start_date, next.end_date),
      period("old", "2026-08-01", "2026-08-07", { status: "completed" }),
      period("recent", "2026-09-01", "2026-09-07", { status: "cancelled" }),
      period("mode", "2026-11-01", "2026-11-07", { mode: "future-mode" }),
      period("state", "2026-12-01", "2026-12-07", { status: "future-state" })];
    const result = iterationTimeline(items);
    expect(result.planned.map(item => item.id)).toEqual(["z", "next"]);
    expect(result.active.map(item => item.id)).toEqual(["early"]);
    expect(result.history.map(item => item.id)).toEqual(["recent", "old"]);
    expect(result.unknown.map(item => item.id)).toEqual(["state", "mode"]);
    expect(result.upcomingId).toBe("next");
    expect(items[0]).toBe(next);
  });

  it("keeps the global upcoming identity and hides gaps under every filter", () => {
    const later = period("later", "2026-11-01", "2026-11-07");
    for (const filters of [{ search: "later" }, { from: "2026-11-01" }, { to: "2026-10-07" }, { status: "planned" }]) {
      const result = iterationTimeline([early, next, later], filters);
      expect(result.upcomingId).toBe(next.id);
      expect(result.gaps.size).toBe(0);
    }
    expect(iterationTimeline([early, next], { search: "  NEXT  " }).planned).toEqual([next]);
    expect(iterationTimeline([early, next], { from: "2026-10-07", to: "2026-10-10" }).active).toEqual([early]);
    expect(iterationTimeline([next], { search: "missing" }).planned).toEqual([]);
  });

  it("counts only complete unallocated calendar days", () => {
    expect(iterationTimeline([next, early]).gaps.get(next.id)).toBe(2);
    expect(iterationTimeline([next]).gaps.size).toBe(0);
    expect(iterationTimeline([early]).upcomingId).toBeNull();
    const old = period("old", "2026-08-01", "2026-08-20", { status: "completed" });
    expect(iterationTimeline([next, early, old, { ...old, id: "old-overlap" }]).gaps.get(next.id)).toBe(2);
  });

  it.each([
    ["2026-03-07", "2026-03-10", "America/New_York", 2],
    ["2026-10-31", "2026-11-03", "America/New_York", 2],
    ["2024-02-28", "2024-03-01", "UTC", 1],
    ["2026-12-30", "2027-01-02", "UTC", 2],
  ])("uses date arithmetic across %s → %s", (end, start, timezone, days) => {
    const before = period("before", end, end, { timezone });
    const after = period("after", start, start, { timezone });
    expect(iterationTimeline([after, before]).gaps.get(after.id)).toBe(days);
  });

  it.each([
    period("overlap", "2026-10-06", "2026-10-11"),
    period("nested", "2026-10-03", "2026-10-04"),
    period("cancelled", "2026-10-08", "2026-10-09", { status: "cancelled" }),
    period("history", "2026-10-08", "2026-10-09", { status: "completed" }),
    period("unknown-mode", "2026-10-08", "2026-10-09", { mode: "automatic" }),
    period("unknown-status", "2026-10-08", "2026-10-09", { status: "unrecognized" }),
  ])("does not infer through intervening or overlapping record $id", item => {
    expect(iterationTimeline([early, next, item]).gaps.size).toBe(0);
  });

  it("suppresses incomparable and invalid boundaries", () => {
    const variants = [
      { timezone: "Asia/Shanghai" }, { workspace_id: "other" },
      { start_date: "2026-02-30" }, { start_date: "2026-10-20" },
      { start_date: "2026-10-8" },
    ];
    for (const fields of variants) expect(iterationTimeline([early, { ...next, ...fields }]).gaps.size).toBe(0);
    for (const start of ["2026-10-07", "2026-10-08"]) {
      expect(iterationTimeline([early, { ...next, start_date: start }]).gaps.size).toBe(0);
    }
  });
});
