// @vitest-environment node
import { describe, expect, it } from "vitest";
import csvTemplate from "./triage-csv-template.json";
import {
  decodeTriageCsv,
  duplicateReference,
  formatTriageCsv,
  snoozePresets,
  nextTriageSelection,
  TRIAGE_CSV_FIELDS,
  triageCsvTemplate,
  triageHistoryFilters,
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
  it("quotes only the cells a spreadsheet would otherwise split", () => {
    expect(
      formatTriageCsv([
        ["plain", "a,b", 'say "hi"', "line 1\nline 2", " padded"],
      ]),
    ).toBe('plain,"a,b","say ""hi""","line 1\nline 2"," padded"\r\n');
  });
  it("covers every mapping field with a BOM-prefixed template per locale", () => {
    expect(csvTemplate.fields).toEqual(TRIAGE_CSV_FIELDS);
    for (const [locale, header] of [
      ["en", "title"],
      ["zh-Hans", "标题"],
    ] as const) {
      const template = triageCsvTemplate(locale);
      expect(template.headers).toHaveLength(TRIAGE_CSV_FIELDS.length);
      expect(template.headers[0]).toBe(header);
      expect(template.csv.startsWith(`\uFEFF${header},`)).toBe(true);
      // The import decoder must accept the downloaded bytes unchanged.
      const bytes = new TextEncoder().encode(template.csv).buffer;
      expect(decodeTriageCsv(bytes)).toBe(template.csv.slice(1));
    }
    expect(triageCsvTemplate("fr").filename).toBe(
      csvTemplate.locales.en.filename,
    );
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

describe("applied history filters", () => {
  it("reads exactly the history fields without reinterpreting date-only bounds", () => {
    const params = new URLSearchParams({
      view: "history",
      q: "launch plan",
      source: "csv",
      result: "accept",
      processed_by: "member-1",
      processed_after: "2026-10-01",
      processed_before: "2026-10-09",
      offset: "50",
      priority: "high",
      reviewer_id: "queue-reviewer",
      from: "saved-link",
    });
    expect(triageHistoryFilters(params)).toEqual([
      { key: "q", value: "launch plan" },
      { key: "source", value: "csv" },
      { key: "result", value: "accept" },
      { key: "processed_by", value: "member-1" },
      { key: "processed_after", value: "2026-10-01" },
      { key: "processed_before", value: "2026-10-09" },
    ]);
  });

  it("ignores empty conditions and retains unfamiliar applied values", () => {
    expect(triageHistoryFilters(new URLSearchParams("q=&source=&offset=50"))).toEqual([]);
    expect(
      triageHistoryFilters(new URLSearchParams("source=future-source&result=future-action")),
    ).toEqual([
      { key: "source", value: "future-source" },
      { key: "result", value: "future-action" },
    ]);
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
