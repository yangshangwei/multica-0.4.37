// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  AGENT_AUTONOMY_LEVELS,
  APPROVAL_RISK_CLASSES,
  APPROVAL_STATUSES,
  isApprovalActionable,
  isApprovalPending,
  isKnownApprovalRiskClass,
  isKnownApprovalStatus,
  isKnownAutonomyLevel,
} from "./agent-template";

// The canonical home for these boundary predicates (the component suites point
// here rather than re-running the matrix through a DOM mount).

describe("isKnownAutonomyLevel", () => {
  it("accepts every level the backend can declare", () => {
    for (const level of AGENT_AUTONOMY_LEVELS) {
      expect(isKnownAutonomyLevel(level)).toBe(true);
    }
  });

  it("rejects absent and unknown values", () => {
    // Absent is the important case: it means the backend declared NO policy for
    // that agent, so a UI that treated it as the lowest level would claim a
    // restriction the backend is not enforcing.
    expect(isKnownAutonomyLevel(undefined)).toBe(false);
    expect(isKnownAutonomyLevel(null)).toBe(false);
    expect(isKnownAutonomyLevel("")).toBe(false);
    // A level a newer backend might add must not be mistaken for a known one.
    expect(isKnownAutonomyLevel("supervisor")).toBe(false);
    expect(isKnownAutonomyLevel("Observer")).toBe(false);
  });

  it("orders the levels from least to most permissive", () => {
    expect(AGENT_AUTONOMY_LEVELS).toEqual([
      "observer",
      "contributor",
      "coordinator",
      "operator",
    ]);
  });
});

describe("approval status predicates", () => {
  it("treats only 'approved' as authorization", () => {
    expect(isApprovalActionable("approved")).toBe(true);
    for (const status of [
      "pending",
      "rejected",
      "executed",
      "cancelled",
      "",
      // A status this client has never seen must never read as authorized.
      "auto_approved",
    ]) {
      expect(isApprovalActionable(status)).toBe(false);
    }
  });

  it("identifies what still needs a person", () => {
    expect(isApprovalPending("pending")).toBe(true);
    expect(isApprovalPending("approved")).toBe(false);
    expect(isApprovalPending("unknown")).toBe(false);
  });
});

describe("APPROVAL_RISK_CLASSES", () => {
  it("covers the five classes the backend accepts", () => {
    expect(APPROVAL_RISK_CLASSES).toEqual([
      "production_release",
      "database_migration",
      "secret_access",
      "external_notification",
      "destructive_operation",
    ]);
  });
});

describe("isKnownApprovalStatus", () => {
  it("accepts every status the backend's CHECK constraint allows", () => {
    for (const status of APPROVAL_STATUSES) {
      expect(isKnownApprovalStatus(status)).toBe(true);
    }
  });

  it("rejects anything else, including absent values", () => {
    for (const value of ["", "auto_approved", "APPROVED", undefined, null]) {
      expect(isKnownApprovalStatus(value)).toBe(false);
    }
  });

  // The separation that matters: this predicate is about having copy for a value,
  // never about authorization. A newer backend status is renderable and unapproved
  // at the same time.
  it("is independent of isApprovalActionable", () => {
    expect(isKnownApprovalStatus("rejected")).toBe(true);
    expect(isApprovalActionable("rejected")).toBe(false);
    expect(isKnownApprovalStatus("preapproved")).toBe(false);
    expect(isApprovalActionable("preapproved")).toBe(false);
  });
});

describe("isKnownApprovalRiskClass", () => {
  it("accepts the five classes and nothing else", () => {
    for (const value of APPROVAL_RISK_CLASSES) {
      expect(isKnownApprovalRiskClass(value)).toBe(true);
    }
    for (const value of ["", "data_export", "Production_Release", undefined, null]) {
      expect(isKnownApprovalRiskClass(value)).toBe(false);
    }
  });
});
