// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseCollaborationOperation } from "./schema";
import { collaborationDeliveryRefusalStatus } from "./delivery-schema";
import { parseAdminCollaborationStageRefusal, parseAdminCollaborationResult } from "../admin/collaboration-schema";
import { stageIds } from "./stage-test-fixtures";
const blockers = [{ code: "stage_ci_missing", stage: "code_review", candidate_id: stageIds.candidate1 }];
const receipt = { code: "stage_incomplete", status: 409, collaboration_revision: "12", stage_blockers: blockers, stage_blockers_truncated: false };

describe("stage refusal on generic collaboration operations", () => {
  it("preserves the stored status, stage blockers and truncation; unknown blocker codes still block", () => {
    expect(parseCollaborationOperation(receipt)).toEqual(receipt);
    expect(collaborationDeliveryRefusalStatus(receipt.code)).toBe(409);
    expect(collaborationDeliveryRefusalStatus("stage_policy_exit_required")).toBe(409);
    expect(parseCollaborationOperation({ ...receipt, stage_blockers: [{ code: "future_gate", stage: "qa" }] })?.stage_blockers).toEqual([{ code: "unknown", stage: "qa" }]);
  });
  it.each([
    { status: 200 }, { status: undefined }, { stage_blockers: [] }, { stage_blockers: undefined }, { stage_blockers_truncated: undefined },
    { stage_blockers: [{ code: "stage_evidence_unavailable", stage: "qa", candidate_id: stageIds.candidate1 }] },
    { stage_blockers: [{ code: "stage_ci_missing", stage: "qa", reason: "PRIVATE" }] },
    { delivery_confirmation: { submission_id: stageIds.candidate1, attachment_ids: [] } },
    { stage_confirmation: { action: "advance_stage", issue_id: stageIds.issue, stage: "qa", stage_revision: "2" } },
    { code: "completed" }, { code: "stage_policy_exit_required" },
  ])("keeps malformed/contradictory stage data unavailable: %j", (change) => { expect(parseCollaborationOperation({ ...receipt, ...change })).toBeNull(); });
  it("accepts policy-exit refusal only at its declared status and without invented blockers", () => {
    expect(parseCollaborationOperation({ code: "stage_policy_exit_required", status: 409, collaboration_revision: "12" })).not.toBeNull();
    expect(parseCollaborationOperation({ code: "stage_policy_exit_required", status: 200, collaboration_revision: "12" })).toBeNull();
  });
});

describe("R38 stage refusal uses the existing administration error envelope", () => {
  const error = { code: "stage_incomplete", error: "PRIVATE raw message", request_id: "transport-trace", stage_blockers: blockers, stage_blockers_truncated: false };
  it("projects only safe stage fields and actual HTTP status, with no successful lifecycle receipt", () => {
    const parsed = parseAdminCollaborationStageRefusal(error, 409);
    expect(parsed).toEqual({ code: "stage_incomplete", status: 409, stage_blockers: blockers, stage_blockers_truncated: false });
    expect(JSON.stringify(parsed)).not.toContain("PRIVATE");
    expect(parseAdminCollaborationResult(error)).toBeNull();
  });
  it.each([
    { stage_blockers: [] }, { request_id: undefined }, { stage_blockers_truncated: null }, { result: {} }, { operation: {} },
    { status: 200 }, { delivery_confirmation: {} }, { collaboration_revision: "12" },
  ])("does not infer an administration rejection from %j", (change) => { expect(parseAdminCollaborationStageRefusal({ ...error, ...change }, 409)).toBeNull(); });
  it("rejects a status mismatch", () => { expect(parseAdminCollaborationStageRefusal(error, 500)).toBeNull(); });
});
