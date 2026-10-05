// @vitest-environment node
import { describe, expect, it } from "vitest";
import { adminDetailHref, adminListParams, adminReturnHref, adminInstantInput, normalizeAdminFilters, validateAdminFilters } from "./view-params";

const now = new Date("2026-10-04T11:30:00Z");
const fields = (values: Record<string, string>) => {
  const form = new FormData();
  for (const [key, value] of Object.entries(values)) form.set(key, value);
  return form;
};

describe("administration filter boundaries", () => {
  it("normalizes native date/time input in UTC without altering explicit offsets", () => {
    const params = normalizeAdminFilters(fields({ time_from: "2026-10-02T09:02", time_to: "2026-10-03T10:00:00+08:00", q: "  audit  " }));
    expect(params.get("time_from")).toBe("2026-10-02T09:02:00Z");
    expect(params.get("time_to")).toBe("2026-10-03T10:00:00+08:00");
    expect(params.get("q")).toBe("audit");
    expect(adminInstantInput("2026-10-02T09:02:00+08:00")).toBe("2026-10-02T01:02:00");
  });
  it("includes an entire selected historical final date and caps today's end at now", () => {
    const past = normalizeAdminFilters(fields({ time_from: "2026-10-02", time_to: "2026-10-02" }), { dateOnly: true, now });
    expect(past.get("time_from")).toBe("2026-10-02T00:00:00.000Z");
    expect(past.get("time_to")).toBe("2026-10-03T00:00:00.000Z");
    expect(validateAdminFilters(past, { now })).toEqual({});
    const today = normalizeAdminFilters(fields({ time_from: "2026-10-04", time_to: "2026-10-04" }), { dateOnly: true, now });
    expect(today.get("time_to")).toBe(now.toISOString());
    expect(validateAdminFilters(today, { now })).toEqual({});
  });
  it.each([
    ["time_from", "2026-02-30T00:00:00Z", "invalidTime"],
    ["time_from", "2026-10-01", "invalidTime"],
    ["timezone", "Mars/Phobos", "invalidTimezone"],
    ["timezone", "Local", "invalidTimezone"],
    ["workspace_id", "abc", "invalidId"],
    ["q", "a".repeat(129), "tooLong"],
    ["time_to", "2026-10-06T00:00:00Z", "futureTime"],
  ])("identifies invalid %s at its field", (name, value, code) => {
    expect(validateAdminFilters(new URLSearchParams({ [name]: value }), { now })[name]).toBe(code);
  });
  it("does not coerce an impossible calendar day into another date", () => {
    const params = normalizeAdminFilters(fields({ time_from: "2026-02-30" }), { dateOnly: true, now });
    expect(validateAdminFilters(params, { now }).time_from).toBe("invalidTime");
  });
  it("enforces the server's UTF-8 byte limit for Chinese and supplementary characters", () => {
    expect(validateAdminFilters(new URLSearchParams({ q: "中".repeat(42) }), { now })).toEqual({});
    expect(validateAdminFilters(new URLSearchParams({ q: "中".repeat(43) }), { now }).q).toBe("tooLong");
    expect(validateAdminFilters(new URLSearchParams({ q: "𠀀".repeat(32) }), { now })).toEqual({});
    expect(validateAdminFilters(new URLSearchParams({ q: "𠀀".repeat(33) }), { now }).q).toBe("tooLong");
  });
  it.each([["PST", "America/Los_Angeles"], ["asia/shanghai", "Asia/Shanghai"], ["utc", "UTC"]])("normalizes the accepted timezone %s to a server-readable name", (input, canonical) => {
    const params = normalizeAdminFilters(fields({ timezone: input }));
    expect(params.get("timezone")).toBe(canonical);
    expect(validateAdminFilters(params, { now })).toEqual({});
  });
  it("checks order and the exact 31-day ceiling, with no ceiling for account directories", () => {
    const params = new URLSearchParams({ time_from: "2026-10-02T00:00:00Z", time_to: "2026-10-02T00:00:00Z" });
    expect(validateAdminFilters(params, { now }).time_to).toBe("invalidRange");
    params.set("time_from", "2026-09-01T00:00:00Z");
    expect(validateAdminFilters(params, { now })).toEqual({});
    params.set("time_from", "2026-08-31T23:59:59Z");
    expect(validateAdminFilters(params, { now }).time_from).toBe("rangeTooLong");
    expect(validateAdminFilters(params, { now, maxWindowDays: null })).toEqual({});
  });
  it("allows empty/current-state filters, UTC, and canonical UUIDs", () => {
    expect(validateAdminFilters(new URLSearchParams("state_scope=current&timezone=UTC&workspace_id=a0000000-0000-4000-8000-000000000001"), { now })).toEqual({});
  });
});

describe("administration list/detail context", () => {
  it("retains the full list query and selected zone", () => {
    const params = new URLSearchParams("status=running&timezone=Asia%2FShanghai&cursor=pinned");
    const detail = new URL(adminDetailHref("/admin/tasks/id", "/admin/tasks", params), "https://example.test");
    expect(detail.searchParams.get("timezone")).toBe("Asia/Shanghai");
    expect(adminReturnHref(detail.searchParams, "/admin/tasks")).toBe(`/admin/tasks?${params}`);
  });
  it("forwards a detail page's list filters with its own selected zone", () => {
    const params = new URLSearchParams({ timezone: "Asia/Shanghai", return_to: "/admin/tasks?status=running&timezone=UTC" });
    expect(adminListParams(params, "/admin/tasks").toString()).toBe("status=running&timezone=Asia%2FShanghai");
    expect(adminListParams(new URLSearchParams({ return_to: "https://evil.test/admin/tasks?status=running" }), "/admin/tasks").toString()).toBe("");
    expect(adminListParams(new URLSearchParams({ return_to: "/admin/audit?phase=applied" }), "/admin/tasks").toString()).toBe("");
  });
  it("preserves the administrator directory for user details", () => {
    expect(adminReturnHref(new URLSearchParams({ return_to: "/admin/administrators?role=super_admin" }), "/admin/users")).toBe("/admin/administrators?role=super_admin");
  });
  it.each(["https://evil.test/admin/tasks", "//evil.test/admin/tasks", "/admin/users", "/admin/tasks/other", "/admin/tasks/../users", "/admin/tasks\\evil"])("rejects an unrelated or external return target %s", (target) => {
    expect(adminReturnHref(new URLSearchParams({ return_to: target }), "/admin/tasks")).toBe("/admin/tasks");
  });
});
