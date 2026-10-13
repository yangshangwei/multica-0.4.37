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
  it.each(["", "\uFEFF"])("prefers UTF-8 with BOM prefix %j", (prefix) => {
    const csv = "title\n中文 é 😀";
    expect(decodeTriageCsv(new TextEncoder().encode(prefix + csv).buffer)).toBe(csv);
  });
  it.each([
    {
      encoding: "GBK with English headers",
      bytes: [0x74, 0x69, 0x74, 0x6c, 0x65, 0x0a, 0xd6, 0xd0, 0xce, 0xc4],
      csv: "title\n中文",
    },
    {
      encoding: "GBK with Chinese headers",
      bytes: [0xb1, 0xea, 0xcc, 0xe2, 0x0a, 0xd6, 0xd0, 0xce, 0xc4],
      csv: "标题\n中文",
    },
    {
      encoding: "GB18030 four-byte characters",
      bytes: [0x74, 0x69, 0x74, 0x6c, 0x65, 0x0a, 0x94, 0x39, 0xfc, 0x36],
      csv: "title\n😀",
    },
    {
      encoding: "UTF-16 LE with BOM",
      bytes: [
        0xff, 0xfe, 0x07, 0x68, 0x98, 0x98, 0x0a, 0x00,
        0x2d, 0x4e, 0x87, 0x65, 0x3d, 0xd8, 0x00, 0xde,
      ],
      csv: "标题\n中文😀",
    },
    {
      encoding: "UTF-16 BE with BOM",
      bytes: [
        0xfe, 0xff, 0x68, 0x07, 0x98, 0x98, 0x00, 0x0a,
        0x4e, 0x2d, 0x65, 0x87, 0xd8, 0x3d, 0xde, 0x00,
      ],
      csv: "标题\n中文😀",
    },
  ])("decodes $encoding without changing CSV content", ({ bytes, csv }) => {
    expect(decodeTriageCsv(new Uint8Array(bytes).buffer)).toBe(csv);
  });
  it.each([
    { name: "empty file", bytes: [] },
    { name: "UTF-8 BOM", bytes: [0xef, 0xbb, 0xbf] },
    { name: "UTF-16 LE BOM", bytes: [0xff, 0xfe] },
    { name: "UTF-16 BE BOM", bytes: [0xfe, 0xff] },
  ])("decodes $name as empty content", ({ bytes }) => {
    expect(decodeTriageCsv(new Uint8Array(bytes).buffer)).toBe("");
  });
  it.each([
    { name: "invalid byte", bytes: [0xff] },
    { name: "truncated GBK character", bytes: [0xd6] },
    { name: "truncated GB18030 character", bytes: [0x81, 0x30, 0x81] },
    { name: "invalid GB18030 character", bytes: [0x81, 0x30, 0x81, 0x2f] },
    { name: "odd UTF-16 LE payload", bytes: [0xff, 0xfe, 0x74] },
    { name: "odd UTF-16 BE payload", bytes: [0xfe, 0xff, 0x00] },
    { name: "unpaired UTF-16 LE surrogate", bytes: [0xff, 0xfe, 0x00, 0xd8] },
    { name: "unpaired UTF-16 BE surrogate", bytes: [0xfe, 0xff, 0xdc, 0x00] },
    // These bytes are valid GB18030, but their BOM declares UTF-8.
    { name: "malformed BOM-declared UTF-8", bytes: [0xef, 0xbb, 0xbf, 0xc3, 0xa9, 0xd6] },
  ])("rejects $name instead of replacing or reinterpreting bytes", ({ bytes }) => {
    expect(() => decodeTriageCsv(new Uint8Array(bytes).buffer)).toThrow();
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
  it.each(["ACM-12", "1A-12", "123-12", "A123456789-12", "acm-12",
    "a56e8429-ff4e-4eec-8743-93907c5e7469"])("accepts the task identity %s", (reference) => {
    expect(duplicateReference(` ${reference} `, "acme")).toBe(reference);
    expect(duplicateReference(`/acme/issues/${reference}`, "acme")).toBe(reference);
  });
  it.each(["", "Fix login", "login", "ACM-0", "TOOLONGPREFIX-1",
    "https://example.test/acme/issues/a-title", "/acme/issues/ACM-1/extra",
    "/acme/issues/ACM-1%2Fextra", "javascript://example.test/acme/issues/ACM-1"])(
    "rejects a search title or invalid reference %s", (reference) => {
      expect(() => duplicateReference(reference, "acme")).toThrow();
    },
  );
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
