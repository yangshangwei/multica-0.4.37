// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { Issue } from "../types";
import {
  classifyCompletionHintIntent,
  isLegacyCompletionHintError,
} from "./completion-hint";
import { ApiError } from "../api/client";

const issue = (overrides: Partial<Issue> = {}): Pick<Issue, "status" | "status_category" | "revision"> => ({
  status: "in_progress",
  status_category: "in_progress",
  revision: 4,
  ...overrides,
});

describe("classifyCompletionHintIntent", () => {
  it("creates a strict null-key request for a done transition", () => {
    expect(classifyCompletionHintIntent(issue(), "done", "done")).toEqual({
      kind: "requires_hint",
      request: {
        expected_issue_revision: 4,
        target_status: "done",
        evaluation_key: null,
      },
    });
  });

  it("recognizes custom done by category while preserving its key", () => {
    expect(classifyCompletionHintIntent(issue(), "shipped", "done")).toEqual({
      kind: "requires_hint",
      request: {
        expected_issue_revision: 4,
        target_status: "shipped",
        evaluation_key: null,
      },
    });
  });

  it.each([
    [issue({ status: "done", status_category: "done" }), "done", "done", "already_done"],
    [issue(), "in_review", "in_review", "target_not_done"],
  ] as const)("does not hint when the transition is %s", (snapshot, target, category, reason) => {
    expect(classifyCompletionHintIntent(snapshot, target, category)).toEqual({
      kind: "not_applicable",
      reason,
    });
  });

  it.each([
    [issue({ revision: undefined }), "missing_revision"],
    [issue({ status: "awaiting_response", status_category: undefined }), "missing_source_category"],
  ] as const)("fails closed when issue metadata is %s", (snapshot, reason) => {
    expect(classifyCompletionHintIntent(snapshot, "done", "done")).toEqual({
      kind: "unknown",
      reason,
    });
  });

  it("fails closed when a custom target category is unresolved", () => {
    expect(classifyCompletionHintIntent(issue(), "shipped")).toEqual({
      kind: "unknown",
      reason: "missing_target_category",
    });
  });
});

describe("isLegacyCompletionHintError", () => {
  it.each([404, 405])("accepts an unstructured legacy %i", (status) => {
    expect(isLegacyCompletionHintError(new ApiError("Not found", status, "Not Found", { error: "Not found" }))).toBe(true);
  });

  it.each([
    new ApiError("Not found", 404, "Not Found", { error: "Not found", code: "not_found" }),
    new ApiError("Forbidden", 403, "Forbidden", { error: "Forbidden", code: "permission_denied" }),
    new ApiError("Conflict", 409, "Conflict", { error: "Conflict", code: "delivery_issue_revision_conflict" }),
    new ApiError("Not found", 404, "Not Found", { error: "Not found", resource_type: "issue", resource_id: "issue-1" }),
    new ApiError("Not found", 404, "Not Found", { error_code: "permission_denied" }),
    new ApiError("Not found", 404, "Not Found", { resource: "issue" }),
    new ApiError("Not found", 404, "Not Found", { permission_code: "denied" }),
    new ApiError("Not found", 404, "Not Found", { delivery_code: "missing_evidence" }),
  ])("does not downgrade structured errors", (error) => {
    expect(isLegacyCompletionHintError(error)).toBe(false);
  });
});
