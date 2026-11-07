// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { CollaborationControlInputSchema, matchesCollaborationControlReceipt, parseCollaborationControlReceipt } from "./control-schema";
import { collaborationControlReceiptFixture as receipt, collaborationControlRequestId as requestId, collaborationControlResumeInput as input } from "./test-fixtures";

const target = input.targets[0]!;
const targetInput = { action: "resume" as const, scope: "target" as const,
  expected_lifecycle_epoch: input.expected_lifecycle_epoch, expected_participant_epoch: input.expected_participant_epoch,
  expected_account_epoch: input.expected_account_epoch, expected_execution_epoch: input.expected_execution_epoch, targets: [target] };
const unknownId = "00000000-0000-4000-8000-000000000099";
afterEach(() => setSchemaLogger(noopLogger));

describe("captured collaboration control input", () => {
  it("retains exact versions and discriminates all four action/scope shapes", () => {
    expect(CollaborationControlInputSchema.parse(input)).toEqual(input);
    expect(CollaborationControlInputSchema.parse(targetInput)).toEqual(targetInput);
    const { expected_execution_epoch: _gate, ...stop } = input;
    const targets = stop.targets.map(({ expected_hold_version: _hold, ...observed }) => observed);
    expect(CollaborationControlInputSchema.safeParse({ ...stop, action: "stop", targets }).success).toBe(true);
    expect(CollaborationControlInputSchema.safeParse({ ...targetInput, action: "stop", expected_execution_epoch: undefined, targets }).success).toBe(false);
    const { expected_execution_epoch: _targetGate, ...targetStop } = targetInput;
    expect(CollaborationControlInputSchema.safeParse({ ...targetStop, action: "stop", targets: [targets[0]] }).success).toBe(true);
  });

  it("rejects missing or stale-format hold/gate versions, duplicates and cross-scope fields", () => {
    for (const value of [undefined, null, "0", "01", "1.2", "1e3", 2, "9223372036854775808"]) {
      expect(CollaborationControlInputSchema.safeParse({ ...input, expected_execution_epoch: value }).success).toBe(false);
      expect(CollaborationControlInputSchema.safeParse({ ...input, targets: [{ ...target, expected_hold_version: value }] }).success).toBe(false);
    }
    for (const value of [
      { ...targetInput, targets: [] }, { ...targetInput, targets: [target, target] },
      { ...input, targets: [target, { ...target, target_id: target.target_id.toUpperCase() }] },
      { ...targetInput, expected_control_version: "1" }, { ...input, extra: "PRIVATE_INPUT" },
      { ...input, targets: [{ ...target, observed_group_generation: "0" }] },
      { ...input, targets: [{ ...target, observed_group_id: undefined }] },
    ]) expect(CollaborationControlInputSchema.safeParse(value).success).toBe(false);
    expect(CollaborationControlInputSchema.safeParse({ ...input, targets: [] }).success).toBe(true);
  });
});

describe("original collaboration control receipt", () => {
  it("preserves partial Resume All without converting an invalid target into a lane pause", () => {
    expect(parseCollaborationControlReceipt(receipt)).toEqual(receipt);
    const alias = { ...receipt.targets[1]!, ...receipt.channels[0]!, target_id: receipt.targets[1]!.target_id };
    const shared = { ...receipt, channels: [receipt.channels[0]], targets: [receipt.targets[0], alias], target_holds: [receipt.target_holds[0], { ...receipt.target_holds[1], channel_id: alias.channel_id }] };
    expect(parseCollaborationControlReceipt(shared)?.channels[0]?.queue_state).toBe("active");
    expect(parseCollaborationControlReceipt(shared)?.target_holds[1]?.code).toBe("grant_required");
    const retained = { ...receipt, targets: [receipt.targets[0]], target_holds: [receipt.target_holds[0]] };
    expect(parseCollaborationControlReceipt(retained)?.channels).toHaveLength(2);
    expect(parseCollaborationControlReceipt({ ...shared, code: "resumed" })).toBeNull();
  });

  it("keeps target refusal atomic and accepts an already-active channel releasing only a hold", () => {
    const refused = { ...receipt, status: 409, code: "stop_unconfirmed", scope: "target", target_id: target.target_id,
      channel_id: receipt.channels[0]!.channel_id, queue_state: "paused", targets: [], channels: [], target_holds: [] };
    expect(parseCollaborationControlReceipt(refused)?.status).toBe(409);
    expect(parseCollaborationControlReceipt({ ...refused, target_holds: [receipt.target_holds[0]] })).toBeNull();
    expect(parseCollaborationControlReceipt({ ...refused, channels: [receipt.channels[0]] })).toBeNull();
    const channel = { ...receipt.channels[0]!, code: "already_active" };
    const resumed = { ...refused, status: 200, code: "resumed", queue_state: "active", group_settled: channel.group_settled, channels: [channel],
      targets: [{ target_id: target.target_id, ...channel }], target_holds: [receipt.target_holds[0]] };
    expect(parseCollaborationControlReceipt(resumed)?.target_holds[0]?.code).toBe("hold_released");
    expect(parseCollaborationControlReceipt({ ...resumed, target_holds: [] })).toBeNull();
    expect(parseCollaborationControlReceipt({ ...resumed, target_holds: [{ ...receipt.target_holds[0], hold_reason: "initial" }] })).toBeNull();
    expect(parseCollaborationControlReceipt({ ...resumed, code: "already_active" })).toBeNull();
  });

  it("represents empty-scope Resume All explicitly without fabricating target decisions", () => {
    const empty = { ...receipt, code: "resumed", targets: [], channels: [], target_holds: [] };
    expect(parseCollaborationControlReceipt(empty)).toEqual(empty);
    for (const field of ["channels", "target_holds"]) {
      for (const value of [undefined, null, {}]) expect(parseCollaborationControlReceipt({ ...empty, [field]: value })).toBeNull();
    }
  });

  it("rejects duplicate, mismatched, unknown or contradictory decisions", () => {
    for (const field of ["channels", "targets", "target_holds"] as const) {
      expect(parseCollaborationControlReceipt({ ...receipt, [field]: [...receipt[field], receipt[field][0]] })).toBeNull();
    }
    for (const value of [
      { ...receipt, action: "unknown" }, { ...receipt, protocol_version: 3 }, { ...receipt, scope: "channel" },
      { ...receipt, code: "future_success" }, { ...receipt, status: 409 },
      { ...receipt, channels: [{ ...receipt.channels[0], code: "grant_required" }, receipt.channels[1]] },
      { ...receipt, channels: [{ ...receipt.channels[0], group_settled: true, unsettled_runs: 1 }, receipt.channels[1]] },
      { ...receipt, targets: [{ ...receipt.targets[0], channel_id: unknownId }, receipt.targets[1]] },
      { ...receipt, target_holds: [{ ...receipt.target_holds[0], channel_id: unknownId }, receipt.target_holds[1]] },
      { ...receipt, target_holds: [{ ...receipt.target_holds[0], hold_reason: "paused" }, receipt.target_holds[1]] },
      { ...receipt, target_holds: [{ ...receipt.target_holds[0], hold_version: "0" }, receipt.target_holds[1]] },
      { ...receipt, control_version: "1e3" }, { ...receipt, target_set_version: 1 },
    ]) expect(parseCollaborationControlReceipt(value)).toBeNull();
  });

  it("binds a recovered response to its original issue, key, action, scope and targets", () => {
    const parsed = parseCollaborationControlReceipt(receipt)!;
    expect(matchesCollaborationControlReceipt(parsed, receipt.issue_id, requestId, input)).toBe(true);
    expect(matchesCollaborationControlReceipt(parsed, unknownId, requestId, input)).toBe(false);
    expect(matchesCollaborationControlReceipt(parsed, receipt.issue_id, "new-key", input)).toBe(false);
    expect(matchesCollaborationControlReceipt(parsed, receipt.issue_id, requestId, targetInput)).toBe(false);
    expect(matchesCollaborationControlReceipt(parsed, receipt.issue_id, requestId, { ...input, targets: [target] })).toBe(false);
    const incomplete = parseCollaborationControlReceipt({ ...receipt, code: "resumed", targets: [], channels: [], target_holds: [] })!;
    expect(matchesCollaborationControlReceipt(incomplete, receipt.issue_id, requestId, input)).toBe(false);
    expect(matchesCollaborationControlReceipt(incomplete, receipt.issue_id, requestId, { ...input, targets: [] })).toBe(true);
  });

  it("redacts private fields from parsed output and all malformed-receipt logging", () => {
    const warn = vi.fn();
    setSchemaLogger({ ...noopLogger, warn });
    expect(JSON.stringify(parseCollaborationControlReceipt({ ...receipt, token: "PRIVATE_TOKEN", grant: "PRIVATE_GRANT" }))).not.toContain("PRIVATE_");
    expect(parseCollaborationControlReceipt({ ...receipt, code: "PRIVATE_BODY" })).toBeNull();
    expect(warn).toHaveBeenCalledOnce();
    expect(JSON.stringify(warn.mock.calls)).not.toContain("PRIVATE_");
    expect(JSON.stringify(warn.mock.calls)).not.toContain("received");
  });
});
