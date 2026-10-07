// @vitest-environment node
import { expect, it } from "vitest";
import { iterationDefaultDates, iterationIsOverdue } from "./calendar";
it("uses fourteen inclusive local calendar days across DST", () => {
  expect(
    iterationDefaultDates("America/New_York", new Date("2026-03-08T04:30:00Z")),
  ).toEqual({ start: "2026-03-07", end: "2026-03-20" });
});
it("marks overdue only after the local end date and only while active", () => {
  const now = new Date("2026-10-06T18:00:00Z");
  expect(iterationIsOverdue("active", "2026-10-06", "Asia/Shanghai", now)).toBe(
    true,
  );
  expect(
    iterationIsOverdue("active", "2026-10-06", "America/New_York", now),
  ).toBe(false);
  expect(
    iterationIsOverdue("planned", "2026-10-06", "Asia/Shanghai", now),
  ).toBe(false);
});
