import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationExecutionViewVersionSchema, CollaborationTargetHoldReasonSchema } from "./execution-schema";
import { nonnegativeVersion, positiveVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const requestId = z.string().min(1).max(128).regex(/^[^\r\n]+$/);
const authority = {
  execution_view_version: CollaborationExecutionViewVersionSchema.optional(),
  expected_lifecycle_epoch: positiveVersion,
  expected_participant_epoch: positiveVersion,
  expected_account_epoch: positiveVersion,
};
const observedTarget = {
  target_id: uuid,
  expected_target_epoch: positiveVersion,
  expected_config_version: positiveVersion,
  expected_control_version: positiveVersion,
  observed_group_id: uuid.nullable(),
  observed_group_generation: nonnegativeVersion,
};
const validObservation = (value: { observed_group_id: string | null; observed_group_generation: string }) => value.observed_group_id === null || value.observed_group_generation !== "0";
const stopTarget = z.strictObject(observedTarget).refine(validObservation);
const resumeTarget = z.strictObject({ ...observedTarget, expected_hold_version: positiveVersion }).refine(validObservation);
const issueVersions = { expected_control_version: positiveVersion, expected_target_set_version: positiveVersion };

/** Capture the complete intent once. Stop deliberately has no hold/gate fence. */
export const CollaborationControlInputSchema = z.discriminatedUnion("action", [
  z.discriminatedUnion("scope", [
    z.strictObject({ ...authority, action: z.literal("stop"), scope: z.literal("target"), targets: z.array(stopTarget).length(1) }),
    z.strictObject({ ...authority, ...issueVersions, action: z.literal("stop"), scope: z.literal("issue"), targets: z.array(stopTarget).max(256) }),
  ]),
  z.discriminatedUnion("scope", [
    z.strictObject({ ...authority, action: z.literal("resume"), scope: z.literal("target"), expected_execution_epoch: positiveVersion, targets: z.array(resumeTarget).length(1) }),
    z.strictObject({ ...authority, ...issueVersions, action: z.literal("resume"), scope: z.literal("issue"), expected_execution_epoch: positiveVersion, targets: z.array(resumeTarget).max(256) }),
  ]),
]).refine((value) => new Set(value.targets.map((item) => item.target_id)).size === value.targets.length);

const refusalCodes = [
  "stale_authority", "stale_scope", "stale_control_version", "stale_execution", "stale_target", "stale_hold", "stale_admission",
  "issue_closed", "issue_paused", "command_unavailable", "collaboration_execution_unavailable",
  "grant_required", "config_invalid", "stop_unconfirmed", "runtime_upgrade_required", "squad_runtime_unavailable",
  "collaboration_client_upgrade_required", "execution_mode_unsupported", "execution_consent_required",
] as const;
const channelFields = {
  code: z.enum(["resumed", "already_active", "stop_unconfirmed", "stop_requested", "already_ended"]),
  queue_state: z.enum(["active", "paused"]),
  control_version: positiveVersion,
  group_generation: nonnegativeVersion,
  group_id: uuid.nullable(),
  group_settled: z.boolean(),
  stopped_runs: z.number().int().nonnegative().safe(),
  unsettled_runs: z.number().int().nonnegative().safe(),
};
const channelResult = z.object({ channel_id: uuid, ...channelFields });
const targetResult = z.object({ target_id: uuid, channel_id: uuid.nullable(), ...channelFields });
const holdResult = z.object({
  target_id: uuid,
  channel_id: uuid.nullable(),
  code: z.enum(["hold_released", "already_active", "grant_required", "config_invalid", "stop_unconfirmed", "runtime_upgrade_required", "squad_runtime_unavailable"]),
  hold_reason: CollaborationTargetHoldReasonSchema.nullable(),
  hold_version: positiveVersion,
});
const receiptBase = {
  protocol_version: z.literal(2),
  issue_id: uuid,
  request_id: requestId,
  status: z.number().int().min(200).max(599),
  scope: z.enum(["target", "issue"]),
  target_id: uuid.nullable(),
  channel_id: uuid.nullable(),
  queue_state: z.enum(["active", "paused", "closed"]),
  control_version: positiveVersion,
  target_set_version: positiveVersion,
  group_id: uuid.nullable(),
  group_settled: z.boolean(),
  // These legacy queue fields never grant authority to a control response.
  grant_id: z.null(), batch_id: z.null(), command_id: z.null(), run_id: z.null(), run_settled: z.literal(false),
  targets: z.array(targetResult).optional().default([]),
  channels: z.array(channelResult),
  target_holds: z.array(holdResult),
};
const receipt = z.discriminatedUnion("action", [
  z.object({ ...receiptBase, action: z.literal("stop"), code: z.enum(["stop_requested", "already_ended", ...refusalCodes]) }),
  z.object({ ...receiptBase, action: z.literal("resume"), code: z.enum(["resumed", "already_active", "partially_resumed", ...refusalCodes]) }),
]);
const holdSucceeded = (code: string) => code === "hold_released" || code === "already_active";

function consistentReceipt(value: z.infer<typeof receipt>): boolean {
  const channels = new Map(value.channels.map((item) => [item.channel_id, item]));
  const targets = new Map(value.targets.map((item) => [item.target_id, item]));
  if (channels.size !== value.channels.length || targets.size !== value.targets.length ||
    new Set(value.target_holds.map((item) => item.target_id)).size !== value.target_holds.length) return false;
  if (value.scope === "issue" ? value.target_id !== null || value.channel_id !== null : value.target_id === null) return false;
  for (const item of [...value.channels, ...value.targets]) {
    if ((item.group_id !== null && item.group_generation === "0") || (item.group_settled && item.unsettled_runs !== 0)) return false;
    if (["resumed", "already_active"].includes(item.code) && item.queue_state !== "active") return false;
    if (["stop_requested", "already_ended"].includes(item.code) && item.queue_state !== "paused") return false;
  }
  for (const item of value.targets) {
    if (item.channel_id === null) {
      if (item.code !== "already_ended" || item.group_id !== null || item.group_generation !== "0") return false;
      continue;
    }
    const owner = channels.get(item.channel_id);
    if (!owner || item.code !== owner.code || item.queue_state !== owner.queue_state || item.control_version !== owner.control_version ||
      item.group_generation !== owner.group_generation || item.group_id !== owner.group_id || item.group_settled !== owner.group_settled ||
      item.stopped_runs !== owner.stopped_runs || item.unsettled_runs !== owner.unsettled_runs) return false;
  }
  for (const item of value.target_holds) {
    if (item.channel_id === null ? !["config_invalid", "stop_unconfirmed"].includes(item.code) : !channels.has(item.channel_id)) return false;
    if (holdSucceeded(item.code) && (item.hold_reason !== null || item.channel_id === null)) return false;
    const alias = targets.get(item.target_id);
    if (alias && alias.channel_id !== item.channel_id) return false;
  }
  const success = ["stop_requested", "already_ended", "resumed", "already_active", "partially_resumed"].includes(value.code);
  if (success !== (value.status === 200)) return false;
  if (!success) {
    if (value.status < 400 || value.group_settled || value.channels.some((item) => item.code !== "stop_unconfirmed") || value.target_holds.some((item) => holdSucceeded(item.code))) return false;
    return value.scope !== "target" || (value.channels.length <= 1 && value.target_holds.every((item) => item.target_id === value.target_id));
  }
  if (value.action === "stop") {
    if (value.target_holds.length !== 0 || value.queue_state === "active" || value.channels.some((item) => !["stop_requested", "already_ended"].includes(item.code))) return false;
    if (value.code === "already_ended" && value.channels.some((item) => item.code === "stop_requested")) return false;
  } else {
    if (value.queue_state !== "active" || value.channels.some((item) => !["resumed", "already_active", "stop_unconfirmed"].includes(item.code))) return false;
    const partial = value.channels.some((item) => item.code === "stop_unconfirmed") || value.target_holds.some((item) => !holdSucceeded(item.code));
    if (partial !== (value.code === "partially_resumed") || (partial && value.scope !== "issue")) return false;
    if (value.target_holds.some((item) => !holdSucceeded(item.code) && item.hold_reason === null)) return false;
    if (value.code === "already_active" && (value.channels.some((item) => item.code !== "already_active") || value.target_holds.some((item) => item.code !== "already_active"))) return false;
    if (value.scope === "target" && (value.channels.length !== 1 || value.target_holds.length !== 1 ||
      value.target_holds[0]?.target_id !== value.target_id || value.target_holds[0]?.channel_id !== value.channel_id)) return false;
  }
  if (value.scope === "target") {
    const selected = targets.get(value.target_id!);
    if (!selected || selected.channel_id !== value.channel_id || selected.queue_state !== value.queue_state || selected.control_version !== value.control_version ||
      selected.group_id !== value.group_id || selected.group_settled !== value.group_settled ||
      value.channels.some((item) => item.channel_id !== value.channel_id)) return false;
  }
  return true;
}

/** A structured refusal remains a refusal even when recovered through HTTP 200. */
export const CollaborationControlReceiptSchema = receipt.refine(consistentReceipt, "Control decisions must retain their scope and outcome");
export type CollaborationControlInput = z.infer<typeof CollaborationControlInputSchema>;
export type CollaborationControlReceipt = z.infer<typeof CollaborationControlReceiptSchema>;
export type CollaborationChannelControlResult = z.infer<typeof channelResult>;
export type CollaborationTargetHoldResult = z.infer<typeof holdResult>;

export function parseCollaborationControlReceipt(data: unknown): CollaborationControlReceipt | null {
  return parseWithFallback<CollaborationControlReceipt | null>(data, CollaborationControlReceiptSchema, null, { endpoint: "collaboration control receipt", redact: true });
}

/** Only the original intent may consume its response or recovered receipt. */
export function matchesCollaborationControlReceipt(value: CollaborationControlReceipt, issueId: string, key: string, input: CollaborationControlInput): boolean {
  if (value.issue_id !== issueId.toLowerCase() || value.request_id !== key || value.action !== input.action || value.scope !== input.scope) return false;
  const observed = new Set(input.targets.map((item) => item.target_id.toLowerCase()));
  if (value.target_holds.some((item) => !observed.has(item.target_id))) return false;
  if (input.scope === "target") return value.target_id === input.targets[0]?.target_id.toLowerCase();
  const represented = new Set([...value.targets, ...value.target_holds].map((item) => item.target_id));
  return value.targets.every((item) => observed.has(item.target_id)) && (value.status !== 200 || input.targets.every((item) => represented.has(item.target_id)));
}
