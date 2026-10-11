// @vitest-environment node
import { describe, expect, it } from "vitest";
import { ApiError } from "@multica/core/api";
import {
  isValidTriageDate,
  isValidTriageSourceUrl,
  triageCreateServerFieldErrors,
  validateTriageCreateInput,
} from "./triage-create-dialog";

describe("triage create field validation", () => {
  it.each([
    ["2026-02-28", true],
    ["2028-02-29", true],
    ["2026-02-29", false],
    ["2026-04-31", false],
    ["0000-02-29", true],
    ["0004-02-29", true],
    ["0096-02-29", true],
    ["0001-02-29", false],
    ["0100-02-29", false],
    ["0400-02-29", true],
    ["2026-13-01", false],
    ["2026-01-00", false],
    ["2026-2-01", false],
    ["2026/04/01", false],
    ["", false],
  ])("validates date %s", (value, valid) => {
    expect(isValidTriageDate(value)).toBe(valid);
  });

  it.each([
    ["https://example.com/request", true],
    ["http://intranet.test/item/1", true],
    [" HTTPS://localhost:8080/request ", true],
    ["https://example.com/request?search=100%", true],
    ["http:example.com", false],
    ["http:/example.com", false],
    ["https:///example.com", false],
    ["https://example.com\\other/path", false],
    ["https://example.com/path\nmore", false],
    ["https://example.com/%ZZ", false],
    ["https://example.com/%#20", false],
    ["https://example.com/#%ZZ", false],
    ["ftp://example.com/request", false],
    ["javascript:alert(1)", false],
    ["https://", false],
    ["", true],
  ])("validates source URL %s", (value, valid) => {
    expect(isValidTriageSourceUrl(value)).toBe(valid);
  });

  it("matches the title limit in Unicode codepoints after payload trimming", () => {
    expect(validateTriageCreateInput({ title: "   " })).toEqual({ title: "invalid_title" });
    expect(validateTriageCreateInput({ title: ` ${"🙂".repeat(500)} ` })).toEqual({});
    expect(validateTriageCreateInput({ title: "🙂".repeat(501) })).toEqual({ title: "invalid_title_length" });
  });

  it("measures the description limit in UTF-8 bytes", () => {
    const description = `${"汉".repeat(Math.floor((1 << 20) / 3))}a`;
    expect(new TextEncoder().encode(description)).toHaveLength(1 << 20);
    expect(validateTriageCreateInput({ title: "Valid", description })).toEqual({});
    expect(validateTriageCreateInput({ title: "Valid", description: `${description}a` })).toEqual({ description: "invalid_description" });
  });

  it("rejects NUL in the field that contains it", () => {
    expect(validateTriageCreateInput({ title: "Invalid\0title", description: "Invalid\0description" })).toEqual({ title: "invalid_content", description: "invalid_content" });
  });

  it("keeps date-only validation strict and reports date order on both controls", () => {
    expect(validateTriageCreateInput({ title: "Valid", start_date: "2026-02-29", due_date: "2026-04-31" })).toEqual({ start_date: "invalid_start_date", due_date: "invalid_due_date" });
    expect(validateTriageCreateInput({ title: "Valid", start_date: "2026-10-12", due_date: "2026-10-11" })).toEqual({ start_date: "start_date_after_due_date", due_date: "due_date_before_start_date" });
    expect(validateTriageCreateInput({ title: "Valid", start_date: "2026-10-11", due_date: "2026-10-11" })).toEqual({});
    expect(validateTriageCreateInput({ title: "Valid", start_date: " 2026-10-11 " })).toEqual({ start_date: "invalid_start_date" });
  });
});

describe("triage create server validation feedback", () => {
  const combined = new ApiError("title is required (maximum 500 characters); description maximum is 1 MiB", 400, "Bad Request");

  it("does not infer invalid fields from the server's combined intake message", () => {
    expect(triageCreateServerFieldErrors(combined, { title: "Valid", description: "Valid" })).toEqual({});
    expect(triageCreateServerFieldErrors(combined, { title: "Valid", description: "a".repeat((1 << 20) + 1) })).toEqual({ description: "invalid_description" });
    expect(triageCreateServerFieldErrors(combined, { title: "a".repeat(501), description: "Valid" })).toEqual({ title: "invalid_title_length" });
  });

  it.each([
    new ApiError("source_url must be an HTTP or HTTPS URL", 500, "Server Error", { code: "invalid_source_url" }),
    new Error("source_url must be an HTTP or HTTPS URL"),
    new ApiError("source_url access denied", 403, "Forbidden"),
  ])("keeps nonvalidation and non-4xx errors global", (error) => {
    expect(triageCreateServerFieldErrors(error, { title: "Valid" })).toEqual({});
  });

  it("maps an explicit 4xx validation code even without a message", () => {
    expect(triageCreateServerFieldErrors(new ApiError("", 422, "Unprocessable Entity", { code: "invalid_source_url" }), { title: "Valid" })).toEqual({ source_url: "invalid_source_url" });
  });

  it("maps the date order response to both related fields", () => {
    expect(triageCreateServerFieldErrors(new ApiError("start_date must not be after due_date", 400, "Bad Request"), { title: "Valid" })).toEqual({ start_date: "start_date_after_due_date", due_date: "due_date_before_start_date" });
  });
});
