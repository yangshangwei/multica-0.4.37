// @vitest-environment node
import { describe, expect, it } from "vitest";
import { adminVersionSchema, executionFenceSchema, canControlAdmission, canCancelExecution } from "./control-schema";

describe("control request snapshots", () => {
  it("normalizes safe legacy counters while preserving full bigint precision", () => {
    expect(adminVersionSchema.parse(2)).toBe("2");
    expect(adminVersionSchema.parse("9223372036854775807")).toBe("9223372036854775807");
    for (const value of [0, -1, 1.5, Number.MAX_SAFE_INTEGER + 1, "0", "01", "1e2", "9223372036854775808", null]) {
      expect(adminVersionSchema.safeParse(value).success).toBe(false);
    }
  });
  it("accepts queued snapshots without inventing runtime or dispatch identity", () => {
    expect(executionFenceSchema.parse({ runtime_id: null, dispatched_at: null, target_version: "7" })).toEqual({ runtimeId: null, dispatchedAt: null, targetVersion: "7" });
    expect(executionFenceSchema.safeParse({ target_version: "7" }).success).toBe(false);
    expect(executionFenceSchema.safeParse({ runtime_id: null, dispatched_at: null }).success).toBe(false);
  });
  it("requires both the current role and authoritative allowed action", () => {
    expect(canControlAdmission("super_admin", { lifecycle: "active", admission: "accepting", allowedActions: ["stop_admission"], admissionVersion: "1" })).toBe("stopped");
    expect(canControlAdmission("super_admin", { lifecycle: "active", admission: "stopped", allowedActions: ["resume_admission"], admissionVersion: "1" })).toBe("accepting");
    expect(canControlAdmission("platform_observer", { lifecycle: "active", admission: "accepting", allowedActions: ["stop_admission"], admissionVersion: "1" })).toBeNull();
    expect(canControlAdmission("super_admin", { lifecycle: "active", admission: "accepting", allowedActions: [], admissionVersion: "1" })).toBeNull();
    const target = { status: "queued", allowedActions: ["cancel"], executionFence: { runtimeId: null, dispatchedAt: null, targetVersion: "1" } };
    expect(canCancelExecution("super_admin", target)).toBe(true);
    expect(canCancelExecution("platform_observer", target)).toBe(false);
    expect(canCancelExecution("super_admin", { ...target, status: "completed" })).toBe(false);
    expect(canCancelExecution("super_admin", { ...target, executionFence: null })).toBe(false);
  });
});
