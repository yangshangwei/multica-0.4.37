// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  decodeTriageCsv,
  duplicateReference,
  snoozePresets,
  nextTriageSelection,
} from "./triage-ui";

describe("triage input boundaries", () => {
  it("rejects invalid UTF-8 instead of silently replacing bytes", () => {
    expect(() =>
      decodeTriageCsv(new Uint8Array([0xff, 0xfe]).buffer),
    ).toThrow();
    expect(
      decodeTriageCsv(new TextEncoder().encode("title\n中文").buffer),
    ).toBe("title\n中文");
  });
  it("keeps quoted CSV content intact for server parsing", () => {
    const csv = 'title,description\n"A,B","line 1\nline 2"';
    expect(decodeTriageCsv(new TextEncoder().encode(csv).buffer)).toBe(csv);
  });
  it("extracts a pasted scoped task link and rejects a different workspace", () => {
    expect(
      duplicateReference("https://example.test/acme/issues/ACM-12?x=1", "acme"),
    ).toBe("ACM-12");
    expect(duplicateReference("ACM-12", "acme")).toBe("ACM-12");
    expect(() =>
      duplicateReference("https://example.test/other/issues/OTHER-1", "acme"),
    ).toThrow();
  });
  it("offers future local-calendar instants rather than date-only values", () => {
    const now = new Date(2026, 9, 4, 22, 15);
    const presets = snoozePresets(now);
    expect(presets.hour.getTime() - now.getTime()).toBe(3600000);
    expect(presets.tomorrow.getHours()).toBe(9);
    expect(presets.tomorrow.getDate()).toBe(5);
    expect(presets.week.getDay()).toBe(1);
    expect(presets.week.getTime()).toBeGreaterThan(now.getTime());
  });
  it("advances through the visible queue and focuses the empty queue at the end", () => {
    expect(nextTriageSelection(["a", "b", "c"], "b")).toBe("c");
    expect(nextTriageSelection(["a", "b"], "b")).toBe("a");
    expect(nextTriageSelection(["a"], "a")).toBe("");
  });
});

import { triageSnapshotChanges } from "./triage-ui";
it("shows only changed review fields from immutable snapshots, excluding storage metadata", () => {
  const changes = triageSnapshotChanges(
    {
      issue: {
        title: "Original",
        revision: 1,
        admission_status: "pending",
        priority: "low",
      },
      reviewer_id: null,
    },
    {
      issue: {
        title: "Original",
        revision: 2,
        admission_status: "accepted",
        priority: "high",
      },
      reviewer_id: "user-1",
    },
  );
  expect(changes.map((change) => change.field)).toEqual([
    "priority",
    "admission_status",
    "reviewer_id",
  ]);
  expect(changes[0]).toEqual({
    field: "priority",
    before: "low",
    after: "high",
  });
});
