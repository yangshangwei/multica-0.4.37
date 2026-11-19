import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { collaborationTextByteLength } from "./text-limits";
import { nonnegativeVersion, positiveVersion } from "./version";

const uuid = z.uuid().refine((value) => value !== "00000000-0000-0000-0000-000000000000").transform((value) => value.toLowerCase());
const digest = z.string().regex(/^[a-f0-9]{64}$/);
const status = z.string().min(1);
const category = z.enum(["backlog", "todo", "in_progress", "in_review", "blocked", "done", "cancelled"]);

export function collaborationInformationTextValid(value: string): boolean {
  if (!value.trim() || value.includes("\0") || collaborationTextByteLength(value) > 4096) return false;
  for (let index = 0; index < value.length; index += 1) {
    const unit = value.charCodeAt(index);
    if (unit >= 0xd800 && unit <= 0xdbff) {
      const next = value.charCodeAt(index + 1);
      if (!(next >= 0xdc00 && next <= 0xdfff)) return false;
      index += 1;
    } else if (unit >= 0xdc00 && unit <= 0xdfff) return false;
  }
  return true;
}

export const CollaborationInformationTextSchema = z.string().refine(collaborationInformationTextValid);
/** Validate the retained input string without rounding or expiring an old request. */
export const CollaborationInformationEstimateSchema = z.iso.datetime({ offset: true }).refine((value) => {
  const fraction = /\.(\d+)/.exec(value)?.[1];
  return fraction === undefined || fraction.length <= 6;
});
export const CollaborationInformationTimestampSchema = z.iso.datetime({ precision: 6 });
export const CollaborationInformationMetadataSchema = z.strictObject({
  unblock_condition: CollaborationInformationTextSchema,
  estimated_resolution_at: CollaborationInformationEstimateSchema,
});

export const CollaborationInformationBlockSchema = z.discriminatedUnion("state", [
  z.strictObject({ protocol_version: z.literal(1), state: z.literal("complete"),
    unblock_condition: CollaborationInformationTextSchema, estimated_resolution_at: CollaborationInformationTimestampSchema }),
  z.strictObject({ protocol_version: z.literal(1), state: z.literal("legacy") }),
  z.strictObject({ protocol_version: z.literal(1), state: z.literal("unavailable"), reason_code: z.literal("information_metadata_invalid") }),
]);
export type CollaborationInformationBlock = z.infer<typeof CollaborationInformationBlockSchema>;

export const CollaborationInformationReasonSchema = z.enum([
  "information_repair_required", "information_metadata_required", "information_metadata_invalid",
  "information_context_changed", "information_context_unavailable", "information_repair_not_applicable",
  "information_repair_basis_unavailable", "information_request_not_active", "forbidden", "stale_revision", "stale_authority",
  "status_options_changed", "status_unavailable", "return_target_unavailable", "target_unavailable",
  "acceptance_pending", "delivery_unavailable", "idempotency_conflict", "issue_closed",
]);
const action = z.strictObject({ available: z.boolean(), code: CollaborationInformationReasonSchema.nullable() })
  .refine((value) => !value.available || value.code === null);
export const CollaborationInformationWorkflowSchema = z.strictObject({
  protocol_version: z.literal(1), managed: z.boolean(), context_version: digest,
  active_request_id: uuid.nullable(), record_state: z.enum(["none", "complete", "legacy", "unavailable"]),
  actions: z.strictObject({ request: action, return: action, repair: action }),
}).refine((value) => value.record_state !== "none" || value.active_request_id === null)
  .refine((value) => !["complete", "legacy"].includes(value.record_state) || value.active_request_id !== null);
export type CollaborationInformationWorkflow = z.infer<typeof CollaborationInformationWorkflowSchema>;

export const CollaborationInformationRepairBasisSchema = z.discriminatedUnion("kind", [
  z.strictObject({ kind: z.literal("human_selected") }),
  z.strictObject({ kind: z.literal("historical_information"), origin_request_id: uuid, origin_event_id: uuid }),
  z.strictObject({ kind: z.literal("pending_submission"), submission_id: uuid, submission_event_id: uuid }),
]);
export type CollaborationInformationRepairBasis = z.infer<typeof CollaborationInformationRepairBasisSchema>;
export const CollaborationInformationRepairSelectionSchema = z.strictObject({
  context_version: digest, active_request_id: uuid.nullable(), basis: CollaborationInformationRepairBasisSchema,
});
export type CollaborationInformationRepairSelection = z.infer<typeof CollaborationInformationRepairSelectionSchema>;
export const CollaborationInformationRepairContextSchema = z.strictObject({
  protocol_version: z.literal(1), context_version: digest, active_request_id: uuid.nullable(),
  historical_origin: z.strictObject({ origin_request_id: uuid, origin_event_id: uuid, status }).nullable(),
  pending_submission: z.strictObject({ submission_id: uuid, submission_event_id: uuid, creator_user_id: uuid }).nullable(),
  human_selection_allowed: z.boolean(),
}).refine((value) => value.pending_submission === null || value.human_selection_allowed === false);
export type CollaborationInformationRepairContext = z.infer<typeof CollaborationInformationRepairContextSchema>;

export const CollaborationInformationRepairStatusOptionsSchema = z.strictObject({
  action: z.literal("repair_information"), status_options_version: digest, default_status: z.literal(""),
  options: z.array(z.strictObject({ key: status, name: z.string(), category })).min(1),
  information_repair_context: CollaborationInformationRepairContextSchema,
}).refine((value) => new Set(value.options.map((item) => item.key)).size === value.options.length)
  .refine((value) => value.options.every((item) => value.information_repair_context.pending_submission !== null
    ? item.category === "in_review"
    : ["backlog", "todo", "in_progress"].includes(item.category) ||
      item.key === value.information_repair_context.historical_origin?.status && item.category === "in_review"));

export const CollaborationInformationRepairConfirmationSchema = z.strictObject({
  repair_handoff_id: uuid, prior_active_request_id: uuid.nullable(), status,
  handler_user_id: uuid, superseded_count: nonnegativeVersion,
});
export type CollaborationInformationRepairConfirmation = z.infer<typeof CollaborationInformationRepairConfirmationSchema>;

export const CollaborationInformationProvenanceSchema = z.discriminatedUnion("state", [
  z.strictObject({ state: z.literal("linked"), event_id: uuid, event_sequence: positiveVersion }),
  z.strictObject({ state: z.literal("legacy_unlinked"), event_id: z.null(), event_sequence: z.null() }),
  z.strictObject({ state: z.literal("unavailable"), event_id: z.null(), event_sequence: z.null(), reason_code: z.literal("information_context_unavailable") }),
]);
const readableProvenance = z.union([CollaborationInformationProvenanceSchema.options[0], CollaborationInformationProvenanceSchema.options[1]]);
const identity = { handoff_id: uuid, created_at: CollaborationInformationTimestampSchema };
const common = { ...identity, provenance: readableProvenance, actor_user_id: uuid, note: z.string(),
  from_user_id: uuid.nullable(), to_user_id: uuid.nullable(), previous_status: status,
  resolution: z.enum(["open", "returned", "superseded"]) };
const request = z.strictObject({ ...common, kind: z.literal("information_request"),
  requester_user_id: uuid.nullable(), parent_request_id: uuid.nullable(), block: CollaborationInformationBlockSchema,
  superseded_by_repair_handoff_id: uuid.nullable() }).refine((value) => value.block.state !== "complete" ||
    collaborationInformationTextValid(value.note) && value.to_user_id !== null);
const returned = z.strictObject({ ...common, kind: z.literal("information_return"), consumed_request_id: uuid.nullable(),
  restored_status: status.nullable(), return_status_fallback: z.boolean().nullable() });
const repaired = z.strictObject({ ...common, kind: z.literal("information_repair"),
  provenance: CollaborationInformationProvenanceSchema.options[0], prior_active_request_id: uuid.nullable(),
  restored_status: status, target_user_id: uuid, basis: CollaborationInformationRepairBasisSchema,
  superseded_request_count: nonnegativeVersion }).refine((value) => value.to_user_id === value.target_user_id);
const unavailable = z.strictObject({ ...identity, kind: z.enum(["information_request", "information_return", "information_repair"]),
  provenance: CollaborationInformationProvenanceSchema.options[2] });
const readableItem = z.union([request, returned, repaired]);
export const CollaborationInformationHistoryItemSchema = z.union([readableItem, unavailable]);
export type CollaborationInformationHistoryItem = z.infer<typeof CollaborationInformationHistoryItemSchema>;
export function collaborationInformationHistoryHasDetails(item: CollaborationInformationHistoryItem): item is z.infer<typeof readableItem> {
  return "note" in item;
}

export const CollaborationInformationTupleSchema = z.strictObject({ created_at: CollaborationInformationTimestampSchema, handoff_id: uuid });
export type CollaborationInformationTuple = z.infer<typeof CollaborationInformationTupleSchema>;
export function compareCollaborationInformationTuple(left: CollaborationInformationTuple, right: CollaborationInformationTuple): number {
  const micros = (value: string) => BigInt(Date.parse(`${value.slice(0, 19)}Z`)) * 1000n + BigInt(value.slice(20, 26));
  const a = micros(left.created_at), b = micros(right.created_at);
  return a < b ? -1 : a > b ? 1 : left.handoff_id < right.handoff_id ? -1 : left.handoff_id > right.handoff_id ? 1 : 0;
}
export const CollaborationInformationCursorSchema = z.string().min(1).refine((value) =>
  collaborationTextByteLength(value) <= 2048 && Array.from(value).every((character) => character.charCodeAt(0) >= 0x20 && character.charCodeAt(0) !== 0x7f));
const envelope = { protocol_version: z.literal(1), issue_id: uuid, collaboration_revision: positiveVersion,
  lifecycle_epoch: positiveVersion, upper_handoff: CollaborationInformationTupleSchema.nullable(),
  next_cursor: CollaborationInformationCursorSchema.nullable() };
export const CollaborationInformationHistorySchema = z.strictObject({ ...envelope,
  items: z.array(CollaborationInformationHistoryItemSchema).max(50) }).refine((value) => {
  if (value.upper_handoff === null) return value.items.length === 0 && value.next_cursor === null;
  return new Set(value.items.map((item) => item.handoff_id)).size === value.items.length &&
    (value.items.length !== 0 || value.next_cursor === null) && value.items.every((item, index) =>
      compareCollaborationInformationTuple(item, value.upper_handoff!) <= 0 &&
      (index === 0 || compareCollaborationInformationTuple(value.items[index - 1]!, item) > 0));
});
export type CollaborationInformationHistory = z.infer<typeof CollaborationInformationHistorySchema>;
export const CollaborationInformationDetailSchema = z.strictObject({ protocol_version: z.literal(1), issue_id: uuid,
  item: CollaborationInformationHistoryItemSchema });
export type CollaborationInformationDetail = z.infer<typeof CollaborationInformationDetailSchema>;
export const CollaborationInformationAffectedItemSchema = z.strictObject({ request_id: uuid, request_event_id: uuid.nullable(),
  provenance: CollaborationInformationProvenanceSchema, resolution: z.literal("superseded"), superseded_by_repair_handoff_id: uuid })
  .refine((value) => value.request_event_id === value.provenance.event_id);
export const CollaborationInformationAffectedSchema = z.strictObject({ ...envelope, repair_handoff_id: uuid,
  superseded_request_count: nonnegativeVersion, items: z.array(CollaborationInformationAffectedItemSchema).max(50) })
  .refine((value) => new Set(value.items.map((item) => item.request_id)).size === value.items.length &&
    value.items.every((item) => item.superseded_by_repair_handoff_id === value.repair_handoff_id) &&
    BigInt(value.items.length) <= BigInt(value.superseded_request_count))
  .refine((value) => value.superseded_request_count === "0"
    ? value.upper_handoff === null && value.items.length === 0 && value.next_cursor === null
    : value.upper_handoff !== null && (value.items.length !== 0 || value.next_cursor === null));
export type CollaborationInformationAffected = z.infer<typeof CollaborationInformationAffectedSchema>;

export const parseCollaborationInformationHistory = (data: unknown) => parseWithFallback<CollaborationInformationHistory | null>(
  data, CollaborationInformationHistorySchema, null, { endpoint: "collaboration information history", redact: true });
export const parseCollaborationInformationDetail = (data: unknown) => parseWithFallback<CollaborationInformationDetail | null>(
  data, CollaborationInformationDetailSchema, null, { endpoint: "collaboration information detail", redact: true });
export const parseCollaborationInformationAffected = (data: unknown) => parseWithFallback<CollaborationInformationAffected | null>(
  data, CollaborationInformationAffectedSchema, null, { endpoint: "collaboration information affected requests", redact: true });
