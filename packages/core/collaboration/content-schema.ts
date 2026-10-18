import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationAccountSummarySchema } from "./schema";
import { positiveVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const timestamp = z.iso.datetime({ offset: true });
const targetResult = z.object({
  target_id: uuid.nullable(), kind: z.enum(["agent", "squad"]), resource_id: uuid,
  target_epoch: positiveVersion.optional(), config_version: positiveVersion.optional(),
  state: z.enum(["waiting", "blocked", "suppressed"]), code: z.string().min(1),
  waiting_reason: z.string().optional(), command_id: uuid.nullable(), command_sequence: positiveVersion.optional(),
  // Multiple immutable comment inputs may belong to one pending batch.
  channel_id: uuid.nullish().transform((value) => value ?? null),
  batch_id: uuid.nullish().transform((value) => value ?? null),
});
const uniqueTargets = <T extends { kind: string; resource_id: string }>(items: T[]) => new Set(items.map((item) => `${item.kind}:${item.resource_id}`)).size === items.length;

export const CollaborationCommentSchema = z.object({
  id: uuid, issue_id: uuid, author_type: z.string(), author: CollaborationAccountSummarySchema.nullable(),
  content: z.string(), type: z.string(), parent_id: uuid.nullable(), comment_revision: positiveVersion,
  created_at: timestamp, updated_at: timestamp, deleted_at: timestamp.nullable(), resolved_at: timestamp.nullable(),
  resolved_by: CollaborationAccountSummarySchema.nullable(), can_edit: z.boolean(), can_resolve: z.boolean(),
  reactions: z.array(z.object({ emoji: z.string(), count: z.number().int().nonnegative().safe(), viewer_reacted: z.boolean() })),
});
export const CollaborationCommentsSchema = z.object({
  issue_id: uuid, items: z.array(CollaborationCommentSchema), next_cursor: z.string().max(512).nullable(),
}).refine((value) => value.items.every((item) => item.issue_id === value.issue_id) && new Set(value.items.map((item) => item.id)).size === value.items.length);

export const CollaborationMentionTargetsSchema = z.object({
  protocol_version: z.literal(2), issue_id: uuid, participants: z.array(CollaborationAccountSummarySchema), can_submit_commands: z.boolean(),
  targets: z.array(z.object({
    target_id: uuid, kind: z.enum(["agent", "squad"]), resource_id: uuid, name: z.string(),
    target_epoch: positiveVersion, config_version: positiveVersion, queue_state: z.enum(["active", "paused"]),
    waiting_reason: z.string(), execution_available: z.literal(false),
  })).refine(uniqueTargets),
}).refine((value) => new Set(value.participants.map((item) => item.user_id.toLowerCase())).size === value.participants.length && new Set(value.targets.map((item) => item.target_id)).size === value.targets.length);

export const CollaborationTriggerPreviewSchema = z.object({
  protocol_version: z.literal(2), issue_id: uuid, lifecycle_epoch: positiveVersion,
  write_context: z.object({ participant_epoch: positiveVersion, account_epoch: positiveVersion }).nullish(),
  targets: z.array(targetResult).refine(uniqueTargets),
  recipients: z.array(z.object({ recipient: CollaborationAccountSummarySchema, is_mention: z.boolean() })),
}).refine((value) => value.targets.every((item) => item.command_id === null && item.command_sequence === undefined) && new Set(value.recipients.map((item) => item.recipient.user_id.toLowerCase())).size === value.recipients.length);

export const CollaborationCommentReceiptSchema = z.object({
  status: z.number().int(), code: z.string().min(1), protocol_version: z.literal(2),
  issue_id: uuid, comment_id: uuid, comment_revision: positiveVersion, event_id: uuid.nullish(), event_seq: positiveVersion.nullish(),
  targets: z.array(targetResult).refine(uniqueTargets),
}).refine((value) => {
  if (value.code === "unchanged") {
    return value.status === 200 && value.event_id == null && value.event_seq == null && value.targets.length === 0;
  }
  return value.event_id != null && value.event_seq != null && value.targets.every((item) => item.state === "waiting" ? item.target_id !== null && item.command_id !== null && item.command_sequence !== undefined : item.command_id === null);
});

export const CollaborationCommentInputSchema = z.object({
  content: z.string().trim().min(1), parent_id: uuid.nullable(), suppressed_target_ids: z.array(uuid),
  expected_lifecycle_epoch: positiveVersion,
  expected_participant_epoch: positiveVersion.nullable(), expected_account_epoch: positiveVersion.nullable(),
  target_versions: z.array(z.object({ target_id: uuid, expected_target_epoch: positiveVersion, expected_config_version: positiveVersion })),
});
export type CollaborationCommentInput = z.infer<typeof CollaborationCommentInputSchema>;
export interface CollaborationPreviewInput { content: string; parent_id: string | null; suppressed_target_ids: string[] }

export const CollaborationNotificationSchema = z.object({
  id: uuid, issue_id: uuid, event_seq: positiveVersion, kind: z.string(), is_mention: z.boolean(),
  source_kind: z.enum(["issue", "comment", "body"]), source_id: uuid.nullable(), issue_title: z.string().nullable(),
  can_open: z.boolean(), created_at: timestamp, read_at: timestamp.nullable(), archived_at: timestamp.nullable(),
}).refine((value) => value.can_open ? value.issue_title !== null && (value.source_kind !== "comment" || value.source_id !== null) : value.issue_title === null && value.source_kind === "issue" && value.source_id === null);
export const CollaborationNotificationsSchema = z.object({
  items: z.array(CollaborationNotificationSchema), unread_count: z.number().int().nonnegative().safe(), next_cursor: z.string().max(512).nullable(),
});
export const CollaborationNotificationReceiptSchema = z.object({ id: uuid, read_at: timestamp.nullable(), archived_at: timestamp.nullable() });

function parse<T>(data: unknown, schema: z.ZodType<T>, endpoint: string): T | null {
  return parseWithFallback<T | null>(data, schema, null, { endpoint, redact: true });
}
export const parseCollaborationComments = (data: unknown) => parse(data, CollaborationCommentsSchema, "collaboration comments");
export const parseCollaborationComment = (data: unknown) => parse(data, CollaborationCommentSchema, "collaboration comment");
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
export type CollaborationNotification = z.infer<typeof CollaborationNotificationSchema>;
