import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationAccountSummarySchema } from "./schema";
import { CollaborationTargetHoldReasonSchema } from "./execution-schema";
import { positiveVersion } from "./version";
import { collaborationAttachmentIds } from "./attachment-schema";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const timestamp = z.iso.datetime({ offset: true });
const targetResult = z.object({
  target_id: uuid.nullable(), kind: z.enum(["agent", "squad"]), resource_id: uuid,
  target_epoch: positiveVersion.optional(), config_version: positiveVersion.optional(),
  hold_reason: CollaborationTargetHoldReasonSchema.nullable(), hold_version: positiveVersion.nullable(),
  state: z.enum(["waiting", "blocked", "suppressed"]), code: z.string().min(1),
  waiting_reason: z.string().optional(), command_id: uuid.nullable(), command_sequence: positiveVersion.optional(),
  // Multiple immutable comment inputs may belong to one pending batch.
  channel_id: uuid.nullish().transform((value) => value ?? null),
  batch_id: uuid.nullish().transform((value) => value ?? null),
  // Additive v2 data: old ordinary queues omit it; joined inputs require proof
  // of their actual group below and never receive this null fallback.
  group_id: uuid.nullish().transform((value) => value ?? null),
}).refine((value) => value.target_id === null
  ? value.hold_version === null && value.hold_reason === null
  : value.hold_version !== null && value.target_epoch !== undefined && value.config_version !== undefined)
  .refine((value) => value.code === "command_joined_squad"
    ? value.state === "waiting" && value.waiting_reason === "joined_squad" && value.group_id !== null && value.command_id !== null && value.command_sequence !== undefined && value.channel_id !== null && value.batch_id !== null && value.target_id !== null
    : value.group_id === null);
const uniqueTargets = <T extends { kind: string; resource_id: string }>(items: T[]) => new Set(items.map((item) => `${item.kind}:${item.resource_id}`)).size === items.length;

export const CollaborationCommentSchema = z.object({
  id: uuid, issue_id: uuid, author_type: z.string(), author: CollaborationAccountSummarySchema.nullable(),
  content: z.string(), type: z.string(), parent_id: uuid.nullable(), comment_revision: positiveVersion,
  attachment_ids: collaborationAttachmentIds.nullish().transform((value) => value ?? []),
  created_at: timestamp, updated_at: timestamp, deleted_at: timestamp.nullable(), resolved_at: timestamp.nullable(),
  resolved_by: CollaborationAccountSummarySchema.nullable(), can_edit: z.boolean(), can_resolve: z.boolean(),
  reactions: z.array(z.object({ emoji: z.string(), count: z.number().int().nonnegative().safe(), viewer_reacted: z.boolean() })),
});
export const CollaborationCommentsSchema = z.object({
  issue_id: uuid, items: z.array(CollaborationCommentSchema), next_cursor: z.string().max(512).nullable(),
}).refine((value) => value.items.every((item) => item.issue_id === value.issue_id) && new Set(value.items.map((item) => item.id)).size === value.items.length);

export const CollaborationCommentHistorySchema = z.object({
  issue_id: uuid, comment_id: uuid,
  items: z.array(z.object({
    event_id: uuid, event_seq: positiveVersion, kind: z.enum(["comment_created", "comment_edited", "comment_deleted"]),
    actor: CollaborationAccountSummarySchema.nullable(), created_at: timestamp, comment_revision: positiveVersion,
    before: z.string().nullable(), after: z.string(),
    before_attachment_ids: collaborationAttachmentIds.nullish(), after_attachment_ids: collaborationAttachmentIds.nullish(),
  })),
}).refine((value) => new Set(value.items.map((item) => item.event_id)).size === value.items.length &&
  new Set(value.items.map((item) => item.event_seq)).size === value.items.length);

export const CollaborationMentionTargetsSchema = z.object({
  protocol_version: z.literal(2), issue_id: uuid, participants: z.array(CollaborationAccountSummarySchema), can_submit_commands: z.boolean(),
  targets: z.array(z.object({
    target_id: uuid, kind: z.enum(["agent", "squad"]), resource_id: uuid, name: z.string(),
    target_epoch: positiveVersion, config_version: positiveVersion, queue_state: z.enum(["active", "paused"]),
    hold_reason: CollaborationTargetHoldReasonSchema.nullable(), hold_version: positiveVersion,
    waiting_reason: z.string(), execution_available: z.literal(false),
  })).refine(uniqueTargets),
}).refine((value) => new Set(value.participants.map((item) => item.user_id.toLowerCase())).size === value.participants.length && new Set(value.targets.map((item) => item.target_id)).size === value.targets.length);

export const CollaborationTriggerPreviewSchema = z.object({
  protocol_version: z.literal(2), issue_id: uuid, lifecycle_epoch: positiveVersion,
  write_context: z.object({ participant_epoch: positiveVersion, account_epoch: positiveVersion }).nullish(),
  targets: z.array(targetResult).refine(uniqueTargets),
  recipients: z.array(z.object({ recipient: CollaborationAccountSummarySchema, is_mention: z.boolean() })),
}).refine((value) => value.targets.every((item) => item.command_id === null && item.command_sequence === undefined && item.group_id === null) && new Set(value.recipients.map((item) => item.recipient.user_id.toLowerCase())).size === value.recipients.length);

export const CollaborationCommentReceiptSchema = z.object({
  status: z.number().int(), code: z.string().min(1), protocol_version: z.literal(2),
  request_id: uuid.optional(), action: z.string().optional(),
  issue_id: uuid, comment_id: uuid, comment_revision: positiveVersion, event_id: uuid.nullish(), event_seq: positiveVersion.nullish(),
  targets: z.array(targetResult).refine(uniqueTargets),
  attachment_ids: collaborationAttachmentIds.optional(),
}).refine((value) => {
  if (value.status >= 400) {
    const expectedStatus: Record<string, number> = { stale_comment_revision: 409, comment_deleted: 409, forbidden: 403, not_found: 404, invalid_comment_request: 400 };
    return value.status === expectedStatus[value.code] &&
      value.request_id !== undefined && (value.action === "edit_comment" || value.action === "delete_comment") &&
      (value.code !== "invalid_comment_request" || value.action === "edit_comment") &&
      value.event_id == null && value.event_seq == null && value.targets.length === 0;
  }
  if (value.code === "unchanged") {
    return value.status === 200 && value.event_id == null && value.event_seq == null && value.targets.length === 0;
  }
  return value.event_id != null && value.event_seq != null && value.targets.every((item) => item.state === "waiting" ? item.target_id !== null && item.command_id !== null && item.command_sequence !== undefined : item.command_id === null);
});

export const CollaborationCommentInputSchema = z.object({
  content: z.string().trim().min(1), parent_id: uuid.nullable(), suppressed_target_ids: z.array(uuid),
  draft_id: uuid.optional(), attachment_ids: collaborationAttachmentIds.optional(),
  expected_lifecycle_epoch: positiveVersion,
  expected_participant_epoch: positiveVersion.nullable(), expected_account_epoch: positiveVersion.nullable(),
  target_versions: z.array(z.object({ target_id: uuid, expected_target_epoch: positiveVersion, expected_config_version: positiveVersion })),
});
export type CollaborationCommentInput = z.infer<typeof CollaborationCommentInputSchema>;
export interface CollaborationPreviewInput { content: string; parent_id: string | null; suppressed_target_ids: string[]; draft_id?: string; attachment_ids?: string[] }

export const CollaborationNotificationSchema = z.object({
  id: uuid, issue_id: uuid, event_seq: positiveVersion, kind: z.string(), is_mention: z.boolean(),
  source_kind: z.enum(["issue", "comment", "body"]), source_id: uuid.nullable(), issue_title: z.string().nullable(),
  // Additive so installed clients can keep opening the issue without run support.
  run_id: uuid.optional(),
  can_open: z.boolean(), created_at: timestamp, read_at: timestamp.nullable(), archived_at: timestamp.nullable(),
}).refine((value) => value.can_open ? value.issue_title !== null && (value.source_kind !== "comment" || value.source_id !== null) : value.issue_title === null && value.source_kind === "issue" && value.source_id === null)
  .refine((value) => value.run_id === undefined || (value.can_open && !value.is_mention &&
    value.source_kind === "issue" && value.source_id === null && ["ai_completed", "ai_failed", "ai_stopped"].includes(value.kind)));
export const CollaborationNotificationsSchema = z.object({
  items: z.array(CollaborationNotificationSchema), unread_count: z.number().int().nonnegative().safe(), next_cursor: z.string().max(512).nullable(),
});
export const CollaborationNotificationReceiptSchema = z.object({ id: uuid, read_at: timestamp.nullable(), archived_at: timestamp.nullable() });

function parse<T>(data: unknown, schema: z.ZodType<T>, endpoint: string): T | null {
  return parseWithFallback<T | null>(data, schema, null, { endpoint, redact: true });
}
export const parseCollaborationComments = (data: unknown) => parse(data, CollaborationCommentsSchema, "collaboration comments");
export const parseCollaborationComment = (data: unknown) => parse(data, CollaborationCommentSchema, "collaboration comment");
export const parseCollaborationCommentHistory = (data: unknown) => parse(data, CollaborationCommentHistorySchema, "collaboration comment history");
export const parseCollaborationMentionTargets = (data: unknown) => parse(data, CollaborationMentionTargetsSchema, "collaboration mention targets");
export const parseCollaborationTriggerPreview = (data: unknown) => parse(data, CollaborationTriggerPreviewSchema, "collaboration trigger preview");
export const parseCollaborationCommentReceipt = (data: unknown) => parse(data, CollaborationCommentReceiptSchema, "collaboration comment receipt");
export const parseCollaborationNotifications = (data: unknown) => parse(data, CollaborationNotificationsSchema, "collaboration notifications");
export const parseCollaborationNotificationReceipt = (data: unknown) => parse(data, CollaborationNotificationReceiptSchema, "collaboration notification receipt");
export type CollaborationComment = z.infer<typeof CollaborationCommentSchema>;
export type CollaborationComments = z.infer<typeof CollaborationCommentsSchema>;
export type CollaborationMentionTargets = z.infer<typeof CollaborationMentionTargetsSchema>;
export type CollaborationTriggerPreview = z.infer<typeof CollaborationTriggerPreviewSchema>;
export type CollaborationCommentReceipt = z.infer<typeof CollaborationCommentReceiptSchema>;
export function matchesCollaborationCommentAttachments(receipt: CollaborationCommentReceipt, expected: string[] | undefined): boolean {
  if (expected === undefined) return true;
  const actual = receipt.attachment_ids ?? [];
  return actual.length === expected.length && actual.every((id) => expected.includes(id));
}
export type CollaborationNotification = z.infer<typeof CollaborationNotificationSchema>;
