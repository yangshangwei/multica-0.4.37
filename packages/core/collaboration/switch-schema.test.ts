// @vitest-environment node
import { expect, it, vi } from "vitest";
import { setSchemaLogger } from "../api/schema";
import { noopLogger } from "../logger";
import { CollaborationSwitchInputSchema, CollaborationSwitchProposalSchema, parseCollaborationSwitchProposal, parseCollaborationSwitchProposals, parseCollaborationSwitchReceipt, matchesCollaborationSwitchReceipt } from "./switch-schema";

const id = "00000000-0000-4000-8000-000000000001";
const other = "00000000-0000-4000-8000-000000000002";
const authority = { expected_lifecycle_epoch: "1", expected_participant_epoch: "2", expected_account_epoch: "9007199254740993" };
const resource = { role: "leader", agent_id: id, runtime_id: id, execution_workspace_id: id, owner_user_id: id };
const input = { ...authority, action: "commit_target_switch" as const, target_id: id, proposal_id: other,
  control: { target_id: id, expected_target_epoch: "1", expected_config_version: "2", expected_hold_version: "3", expected_control_version: "4", observed_group_id: null, observed_group_generation: "0" } };
const proposal = { proposal_id: other, target_id: id, proposed_by: id, lifecycle_epoch: "1", base_target_epoch: "1", base_config_version: "2",
  proposed_target_epoch: "2", proposed_config_version: "3", status: "consent_ready", reason: "",
  resources: [{ ...resource, consented: true, revoked: false, grant_id: null, grant_epoch: null }] };
const receipt = { protocol_version: 2, issue_id: id, request_id: "original", action: input.action, status: 200, code: "target_switched", scope: "target",
  target_id: id, proposal_id: other, channel_id: null, queue_state: "active", control_version: "4", target_set_version: "5",
  grant_id: null, batch_id: null, command_id: null, group_id: null, group_settled: false, run_id: null, run_settled: false, channels: [], target_holds: [] };

it("captures exact authority and refuses mismatched or incomplete Switch observations", () => {
  expect(CollaborationSwitchInputSchema.parse(input)).toEqual(input);
  for (const change of [{ control: { ...input.control, target_id: other } }, { control: { ...input.control, observed_group_id: id } },
    { expected_account_epoch: Number.MAX_SAFE_INTEGER + 1 }, { expected_account_epoch: "9223372036854775808" }, { control: { ...input.control, expected_hold_version: undefined } }]) {
    expect(CollaborationSwitchInputSchema.safeParse({ ...input, ...change }).success).toBe(false);
  }
});

it("requires unique proposed resources and an exact known runtime identity", () => {
  const proposed = { ...authority, action: "propose_target_switch", target_id: id, expected_target_epoch: "1", expected_config_version: "2", roster: [resource] };
  expect(CollaborationSwitchInputSchema.safeParse(proposed).success).toBe(true);
  expect(CollaborationSwitchInputSchema.safeParse({ ...proposed, roster: [resource, resource] }).success).toBe(false);
  expect(CollaborationSwitchInputSchema.safeParse({ ...authority, action: "consent_target_switch", target_id: id, proposal_id: other, agent_id: id,
    identity: { kind: "unknown", installation_id: id, binding_id: other, binding_epoch: "1" } }).success).toBe(false);
});

it("rejects false readiness, version drift, incomplete grant identities and unknown proposal states", () => {
  expect(CollaborationSwitchProposalSchema.safeParse(proposal).success).toBe(true);
  for (const change of [{ status: "future" }, { proposed_config_version: "4" }, { resources: [] },
    { resources: [{ ...proposal.resources[0], consented: false }] }, { resources: [{ ...proposal.resources[0], revoked: true }] },
    { resources: [{ ...proposal.resources[0], grant_id: id }] }]) {
    expect(CollaborationSwitchProposalSchema.safeParse({ ...proposal, ...change }).success).toBe(false);
  }
  expect(CollaborationSwitchProposalSchema.safeParse({ ...proposal, status: "committed", resources: [{ ...proposal.resources[0], grant_id: id, grant_epoch: "1" }] }).success).toBe(true);
});

it("makes malformed private proposal versions unavailable without throwing or logging their values", () => {
  const warning = vi.fn();
  setSchemaLogger({ ...noopLogger, warn: warning });
  try {
    for (const field of ["base_target_epoch", "base_config_version", "proposed_target_epoch", "proposed_config_version"]) {
      const malformed = { ...proposal, [field]: "PRIVATE_MALFORMED_VERSION" };
      expect(() => CollaborationSwitchProposalSchema.safeParse(malformed)).not.toThrow();
      expect(parseCollaborationSwitchProposal(malformed)).toBeNull();
      expect(parseCollaborationSwitchProposals([malformed])).toBeNull();
    }
    expect(JSON.stringify(warning.mock.calls)).not.toContain("PRIVATE_MALFORMED_VERSION");
    expect(warning).toHaveBeenCalledTimes(8);
  } finally {
    setSchemaLogger(noopLogger);
  }
});

it("keeps original Switch refusals and rejects wrong proposal, action, key or execution side effects", () => {
  const parsed = parseCollaborationSwitchReceipt(receipt)!;
  expect(parsed).not.toBeNull();
  expect(matchesCollaborationSwitchReceipt(parsed, id, "original", input)).toBe(true);
  expect(matchesCollaborationSwitchReceipt(parsed, id, "other", input)).toBe(false);
  expect(matchesCollaborationSwitchReceipt(parsed, id, "original", { ...input, proposal_id: id })).toBe(false);
  const refused = { ...receipt, status: 409, code: "switch_consent_required" };
  expect(parseCollaborationSwitchReceipt(refused)?.status).toBe(409);
  for (const change of [{ status: 200, code: "switch_consent_required" }, { action: "stop" }, { run_id: id }, { group_settled: true }, { channels: [{}] }]) {
    expect(parseCollaborationSwitchReceipt({ ...receipt, ...change })).toBeNull();
  }
});

it("accepts only explicitly redacted original outcomes without current resource data", () => {
  const redacted = { redacted: true, protocol_version: 2, issue_id: id, request_id: "original", action: input.action, status: 200, code: "target_switched" };
  const parsed = parseCollaborationSwitchReceipt(redacted)!;
  expect(parsed).toEqual(redacted);
  expect(matchesCollaborationSwitchReceipt(parsed, id, "original", input)).toBe(true);
  expect(matchesCollaborationSwitchReceipt(parsed, id, "other", input)).toBe(false);
  for (const patch of [{ redacted: undefined }, { target_id: id }, { proposal_id: other }, { status: 409 }, { action: "stop" }]) {
    expect(parseCollaborationSwitchReceipt({ ...redacted, ...patch })).toBeNull();
  }
});
