// @vitest-environment node
import type { TriageHistoryEntry } from "@multica/core/triage";
import { describe, expect, it } from "vitest";
import { groupTriageHistoryEntries } from "./triage-history-dates";

function entry(id: string, created_at: string): TriageHistoryEntry {
  return {
    id,
    created_at,
    kind: "action",
    action: "accept",
    issue_id: id,
    identifier: id,
    title: id,
    actor_id: "actor-1",
    reason: null,
    before: {},
    after: {},
    batch_id: null,
    filename: null,
    counts: null,
  };
}

describe("history display dates", () => {
  it("groups at display-clock midnight and formats adjacent times on that same clock", () => {
    const groups = groupTriageHistoryEntries(
      [
        entry("a", "2026-10-09T16:05:01Z"),
        entry("b", "2026-10-09T16:00:00Z"),
        entry("c", "2026-10-09T15:59:59Z"),
      ],
      "en-US",
      "Asia/Shanghai",
    );

    expect(groups.map((group) => group.dateLabel)).toEqual([
      "October 10, 2026",
      "October 9, 2026",
    ]);
    expect(groups[0]?.events.map(({ entry }) => entry.id)).toEqual(["a", "b"]);
    expect(groups[0]?.events[0]?.time).toBe("12:05 AM");
    expect(groups[0]?.events[0]?.fullTime).toBe(
      "October 10, 2026 at 12:05:01 AM GMT+8",
    );
    expect(groups[1]?.events[0]?.time).toBe("11:59 PM");
  });

  it("keeps equal-time identities and noncontiguous days in their incoming order", () => {
    const entries = [
      entry("a", "2026-10-10T15:00:00Z"),
      entry("b", "2026-10-10T15:00:00Z"),
      entry("c", "2026-10-09T15:00:00Z"),
      entry("d", "2026-10-10T16:00:00Z"),
    ];
    const groups = groupTriageHistoryEntries(entries, "en-US", "UTC");

    expect(groups.map((group) => group.events.map(({ entry }) => entry.id))).toEqual([
      ["a", "b"],
      ["c"],
      ["d"],
    ]);
    expect(new Set(groups.map((group) => group.key)).size).toBe(3);
    const flattened = groups.flatMap((group) => group.events.map(({ entry }) => entry));
    flattened.forEach((value, index) => expect(value).toBe(entries[index]));
  });

  it("retains both repeated daylight-saving times with their full timezone evidence", () => {
    const groups = groupTriageHistoryEntries(
      [
        entry("later", "2026-11-01T06:30:00Z"),
        entry("earlier", "2026-11-01T05:30:00Z"),
      ],
      "en-US",
      "America/New_York",
    );
    expect(groups).toHaveLength(1);
    expect(groups[0]?.events.map((event) => event.time)).toEqual([
      "01:30 AM",
      "01:30 AM",
    ]);
    expect(groups[0]?.events[0]?.fullTime).toContain("EST");
    expect(groups[0]?.events[1]?.fullTime).toContain("EDT");
  });

  it("keeps malformed timestamps without passing an invalid Date to Intl", () => {
    const groups = groupTriageHistoryEntries(
      [
        entry("bad", "not-a-date"),
        entry("missing", ""),
        entry("valid", "2026-10-10T00:00:00Z"),
        entry("bad-again", "still-not-a-date"),
      ],
      "zh-Hans",
      "UTC",
    );
    expect(groups.map((group) => group.dateLabel)).toEqual([
      null,
      "2026年10月10日",
      null,
    ]);
    expect(groups[0]?.events[0]).toMatchObject({
      entry: { id: "bad", created_at: "not-a-date" },
      time: null,
      fullTime: null,
    });
    expect(groups.flatMap((group) => group.events)).toHaveLength(4);
  });

  it("uses the browser clock by default and accepts an empty page", () => {
    const entries = [entry("a", "2026-10-09T23:30:00Z")];
    expect(groupTriageHistoryEntries(entries, "zh-Hans")).toEqual(
      groupTriageHistoryEntries(
        entries,
        "zh-Hans",
        Intl.DateTimeFormat().resolvedOptions().timeZone,
      ),
    );
    expect(groupTriageHistoryEntries([], "en-US")).toEqual([]);
  });
});
