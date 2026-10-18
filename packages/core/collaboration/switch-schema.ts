import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { nonnegativeVersion, positiveVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const authority = { expected_lifecycle_epoch: positiveVersion, expected_participant_epoch: positiveVersion, expected_account_epoch: positiveVersion };
const resource = z.strictObject({ role: z.enum(["agent", "leader", "member"]), agent_id: uuid, runtime_id: uuid, execution_workspace_id: uuid, owner_user_id: uuid });
const roster = z.array(resource).min(1).max(256).refine((items) => new Set(items.map((item) => item.agent_id)).size === items.length);
const control = z.strictObject({ target_id: uuid, expected_target_epoch: positiveVersion, expected_config_version: positiveVersion,
  expected_hold_version: positiveVersion, expected_control_version: positiveVersion, observed_group_id: uuid.nullable(), observed_group_generation: nonnegativeVersion,
}).refine((value) => value.observed_group_id === null || value.observed_group_generation !== "0");
const base = { ...authority, target_id: uuid };

/** The action and path identities belong to the saved intent, not the body. */
export const CollaborationSwitchInputSchema = z.discriminatedUnion("action", [
  z.strictObject({ ...base, action: z.literal("propose_target_switch"), expected_target_epoch: positiveVersion, expected_config_version: positiveVersion, roster }),
  z.strictObject({ ...base, action: z.literal("consent_target_switch"), proposal_id: uuid, agent_id: uuid,
    identity: z.strictObject({ kind: z.enum(["managed", "enrolled"]), installation_id: uuid, binding_id: uuid, binding_epoch: positiveVersion }) }),
  z.strictObject({ ...base, action: z.literal("revoke_target_switch_consent"), proposal_id: uuid, agent_id: uuid }),
  z.strictObject({ ...base, action: z.literal("commit_target_switch"), proposal_id: uuid, control }),
]).refine((value) => value.action !== "commit_target_switch" || value.target_id === value.control.target_id);
export type CollaborationSwitchInput = z.infer<typeof CollaborationSwitchInputSchema>;

const resourceView = resource.extend({ consented: z.boolean(), revoked: z.boolean(), grant_id: uuid.nullable(), grant_epoch: positiveVersion.nullable() })
  .refine((value) => !(value.consented && value.revoked) && (value.grant_id === null) === (value.grant_epoch === null));

function nextVersion(proposed: string, base: string): boolean {
  // Aggregate refinements can run even after a child string failed validation.
  return positiveVersion.safeParse(proposed).success && positiveVersion.safeParse(base).success && BigInt(proposed) === BigInt(base) + 1n;
}
export const CollaborationSwitchProposalSchema = z.object({
  proposal_id: uuid, target_id: uuid, proposed_by: uuid, lifecycle_epoch: positiveVersion,
  base_target_epoch: positiveVersion, base_config_version: positiveVersion, proposed_target_epoch: positiveVersion, proposed_config_version: positiveVersion,
  status: z.enum(["pending", "consent_ready", "stale", "committed"]), reason: z.string(), resources: z.array(resourceView).min(1).max(256),
}).refine((value) => nextVersion(value.proposed_target_epoch, value.base_target_epoch) &&
  nextVersion(value.proposed_config_version, value.base_config_version) &&
  new Set(value.resources.map((item) => item.agent_id)).size === value.resources.length &&
  (value.status !== "consent_ready" || value.resources.every((item) => item.consented && !item.revoked)) &&
  (value.status === "committed" || value.resources.every((item) => item.grant_id === null)));
export type CollaborationSwitchProposal = z.infer<typeof CollaborationSwitchProposalSchema>;

const outcomes = {
  propose_target_switch: "switch_proposed", consent_target_switch: "switch_resource_consented",
  revoke_target_switch_consent: "switch_consent_revoked", commit_target_switch: "target_switched",
} as const;
const refusal = z.enum(["stale_authority", "stale_scope", "stale_control_version", "stale_execution", "stale_target", "stale_hold", "stale_admission",
  "issue_closed", "forbidden", "target_not_found", "target_pause_required", "invalid_target_configuration", "config_invalid", "grant_required",
  "stop_unconfirmed", "runtime_registration_required", "runtime_upgrade_required", "resource_unavailable", "owner_not_participant",
  "switch_proposal_not_found", "switch_consent_not_found", "switch_already_committed", "stale_switch_proposal", "stale_switch_consent", "switch_consent_required"]);

const fullSwitchReceipt = z.object({
  redacted: z.literal(false).optional(),
  protocol_version: z.literal(2), issue_id: uuid, request_id: z.string().min(1).max(128), action: z.enum(Object.keys(outcomes) as [keyof typeof outcomes, ...Array<keyof typeof outcomes>]),
  status: z.number().int().min(200).max(599), code: z.union([z.enum(Object.values(outcomes)), refusal]), scope: z.literal("target"),
  target_id: uuid, proposal_id: uuid.nullable(), channel_id: uuid.nullable(), queue_state: z.enum(["active", "paused", "closed"]),
  control_version: positiveVersion, target_set_version: positiveVersion, group_id: uuid.nullable(), group_settled: z.literal(false),
  grant_id: z.null(), batch_id: z.null(), command_id: z.null(), run_id: z.null(), run_settled: z.literal(false),
  channels: z.array(z.never()).length(0), target_holds: z.array(z.never()).length(0),
}).refine((value) => {
  const succeeded = value.code === outcomes[value.action];
  if (!succeeded) return value.status >= 400 && refusal.safeParse(value.code).success;
  return value.proposal_id !== null && (value.status === 200 || value.status === 201) &&
    (value.action !== "commit_target_switch" || value.group_id === null);
});
const redactedSwitchReceipt = z.strictObject({
  redacted: z.literal(true), protocol_version: z.literal(2), issue_id: uuid,
  request_id: z.string().min(1).max(128), action: z.enum(Object.keys(outcomes) as [keyof typeof outcomes, ...Array<keyof typeof outcomes>]),
  status: z.number().int().min(200).max(599), code: z.union([z.enum(Object.values(outcomes)), refusal]),
}).refine((value) => value.code === outcomes[value.action] ? value.status === 200 || value.status === 201 : value.status >= 400 && refusal.safeParse(value.code).success);
export const CollaborationSwitchReceiptSchema = z.union([redactedSwitchReceipt, fullSwitchReceipt]);
export type CollaborationSwitchReceipt = z.infer<typeof CollaborationSwitchReceiptSchema>;

export function parseCollaborationSwitchReceipt(value: unknown): CollaborationSwitchReceipt | null {
  return parseWithFallback<CollaborationSwitchReceipt | null>(value, CollaborationSwitchReceiptSchema, null, { endpoint: "collaboration Switch receipt", redact: true });
}
export function parseCollaborationSwitchProposal(value: unknown): CollaborationSwitchProposal | null {
  return parseWithFallback<CollaborationSwitchProposal | null>(value, CollaborationSwitchProposalSchema, null, { endpoint: "collaboration Switch proposal", redact: true });
}
export function parseCollaborationSwitchProposals(value: unknown): CollaborationSwitchProposal[] | null {
  return parseWithFallback<CollaborationSwitchProposal[] | null>(value, z.array(CollaborationSwitchProposalSchema)
    .refine((items) => new Set(items.map((item) => item.proposal_id)).size === items.length), null, { endpoint: "collaboration Switch proposals", redact: true });
}
export function matchesCollaborationSwitchReceipt(value: CollaborationSwitchReceipt, issueId: string, requestId: string, input: CollaborationSwitchInput): boolean {
  return value.issue_id === issueId.toLowerCase() && value.request_id === requestId && value.action === input.action && (value.redacted === true || (value.target_id === input.target_id.toLowerCase() &&
    (input.action === "propose_target_switch" || value.proposal_id === input.proposal_id.toLowerCase())));
}

export function collaborationSwitchRequest(issueId: string, input: CollaborationSwitchInput): { path: string; body: Record<string, unknown> } {
  const { action, target_id, ...data } = input;
  const path = `/api/collaboration/issues/${encodeURIComponent(issueId)}/targets/${encodeURIComponent(target_id)}/switch-proposals`;
  switch (action) {
    case "propose_target_switch": return { path, body: data };
    case "consent_target_switch": {
      const { proposal_id, ...body } = input;
      return { path: `${path}/${encodeURIComponent(proposal_id)}/consents`, body: { ...authorityBody(input), agent_id: body.agent_id, identity: body.identity } };
    }
    case "revoke_target_switch_consent": return { path: `${path}/${encodeURIComponent(input.proposal_id)}/consents/${encodeURIComponent(input.agent_id)}/revoke`, body: authorityBody(input) };
    case "commit_target_switch": return { path: `${path}/${encodeURIComponent(input.proposal_id)}/commit`, body: { ...authorityBody(input), control: input.control } };
    default: throw new TypeError("Invalid collaboration Switch intent");
  }
}
function authorityBody(input: CollaborationSwitchInput) {
  return { expected_lifecycle_epoch: input.expected_lifecycle_epoch, expected_participant_epoch: input.expected_participant_epoch, expected_account_epoch: input.expected_account_epoch };
}
