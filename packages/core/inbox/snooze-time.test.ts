// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  inboxSnoozePresets,
  parseInboxSnoozeDateTime,
  toInboxLocalDateTime,
} from "./snooze-time";

afterEach(() => vi.unstubAllEnvs());

describe("inboxSnoozePresets", () => {
  it("keeps hour durations and calendar-day 09:00 presets distinct", () => {
    const now = new Date(2026, 5, 15, 17, 45, 20, 123);
    const presets = inboxSnoozePresets(now);

    expect(presets.map((entry) => entry.key)).toEqual([
      "one_hour", "three_hours", "tomorrow", "three_days", "one_week",
    ]);
    expect(presets[0]?.date.getTime()).toBe(now.getTime() + 60 * 60 * 1000);
    expect(presets[1]?.date.getTime()).toBe(now.getTime() + 3 * 60 * 60 * 1000);
    expect(presets.slice(2).map((entry) => toInboxLocalDateTime(entry.date))).toEqual([
      "2026-06-16T09:00", "2026-06-18T09:00", "2026-06-22T09:00",
    ]);
  });

  it("uses local calendar dates over the year boundary", () => {
    const presets = inboxSnoozePresets(new Date(2026, 11, 31, 23, 30));
    expect(presets.slice(2).map((entry) => toInboxLocalDateTime(entry.date))).toEqual([
      "2027-01-01T09:00", "2027-01-03T09:00", "2027-01-07T09:00",
    ]);
  });

  it("keeps tomorrow at local 09:00 through a DST transition", () => {
    vi.stubEnv("TZ", "America/New_York");
    const now = new Date(2026, 2, 7, 9, 0);
    const tomorrow = inboxSnoozePresets(now).find((entry) => entry.key === "tomorrow");

    expect(tomorrow?.date.getHours()).toBe(9);
    expect(tomorrow?.date.getTime()).toBe(now.getTime() + 23 * 60 * 60 * 1000);
  });
});

describe("parseInboxSnoozeDateTime", () => {
  const now = new Date(2026, 5, 15, 8, 0);

  it("parses a future wall-clock value into its local instant", () => {
    expect(parseInboxSnoozeDateTime("2026-06-16T09:30", now)).toEqual(
      new Date(2026, 5, 16, 9, 30),
    );
  });

  it.each([
    "", "2026-06-15T07:59", "2026-06-15T08:00", "2026-02-30T09:00",
    "2026-13-16T09:00", "2026-06-16T24:00", "2026-06-16T09:60",
    "2026-06-16T09:00Z", "2026-06-16", "not-a-date",
  ])("rejects malformed, normalized or non-future time %s", (value) => {
    expect(parseInboxSnoozeDateTime(value, now)).toBeNull();
  });

  it("rejects a nonexistent local time instead of silently normalizing it", () => {
    vi.stubEnv("TZ", "America/New_York");
    expect(
      parseInboxSnoozeDateTime("2026-03-08T02:30", new Date(2026, 2, 7, 0, 0)),
    ).toBeNull();
  });

  it("resolves a repeated local time to the first occurrence with its actual offset", () => {
    vi.stubEnv("TZ", "America/New_York");
    const date = parseInboxSnoozeDateTime("2026-11-01T01:30", new Date(2026, 9, 31, 0, 0));
    expect(date?.toISOString()).toBe("2026-11-01T05:30:00.000Z");
    expect(date?.getTimezoneOffset()).toBe(240);
  });

  it("has no separate 90-day triage horizon", () => {
    expect(parseInboxSnoozeDateTime("2027-06-16T09:00", now)).toEqual(
      new Date(2027, 5, 16, 9, 0),
    );
  });
});

describe("toInboxLocalDateTime", () => {
  it("pads the actual local fields for a datetime-local input", () => {
    expect(toInboxLocalDateTime(new Date(2027, 0, 2, 3, 4))).toBe("2027-01-02T03:04");
  });
});
