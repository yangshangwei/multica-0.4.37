import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationAccountSummarySchema } from "./account-schema";
import { collaborationAttachmentIds } from "./attachment-schema";
import { collaborationCommentWithinByteLimit } from "./text-limits";
import { positiveVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const timestamp = z.iso.datetime({ offset: true });
export const CollaborationAIPolicySchema = z.enum(["manual", "assisted"]);
export type CollaborationAIPolicy = z.infer<typeof CollaborationAIPolicySchema>;
export const collaborationResultText = z.string().refine((value) => value.trim().length > 0 && collaborationCommentWithinByteLimit(value));
export const collaborationBoundedResultText = z.string().refine(collaborationCommentWithinByteLimit);

export const CollaborationDeliveryDecisionSchema = z.object({
  kind: z.enum(["accepted", "returned", "cancelled"]), submission_id: uuid,
  event_seq: positiveVersion, actor_user_id: uuid, actor: CollaborationAccountSummarySchema.nullable(),
  created_at: timestamp, reason: collaborationBoundedResultText.nullable(), target_user_id: uuid.nullable(),
}).refine((value) => (value.actor === null || value.actor.user_id.toLowerCase() === value.actor_user_id) &&
  (value.kind === "returned" ? value.target_user_id !== null && value.reason !== null && value.reason.trim().length > 0 : value.target_user_id === null));

export const CollaborationSubmissionSchema = z.object({
  id: uuid, issue_id: uuid, event_seq: positiveVersion, lifecycle_epoch: positiveVersion.nullable(),
  author_user_id: uuid, author: CollaborationAccountSummarySchema.nullable(), created_at: timestamp,
  result: collaborationBoundedResultText.nullable(), basis: collaborationBoundedResultText.nullable(),
  attachment_ids: collaborationAttachmentIds, source_comment_id: uuid.nullable(),
  evidence_state: z.enum(["available", "legacy", "unavailable"]), decision: CollaborationDeliveryDecisionSchema.nullable(),
}).refine((value) => (value.author === null || value.author.user_id.toLowerCase() === value.author_user_id) &&
  (value.evidence_state !== "available" || value.lifecycle_epoch !== null && value.result !== null && value.result.trim().length > 0 && value.basis !== null && value.basis.trim().length > 0) &&
  (value.decision === null || value.decision.submission_id === value.id && BigInt(value.decision.event_seq) > BigInt(value.event_seq)));

export const CollaborationDeliverySummarySchema = z.object({
  current: CollaborationSubmissionSchema.nullable(), latest: CollaborationSubmissionSchema.nullable(), has_history: z.boolean(),
}).refine((value) => value.current?.decision == null &&
  (value.has_history ? value.latest !== null : value.current === null && value.latest === null) &&
  (value.current === null || value.latest !== null && value.latest.id === value.current.id &&
    JSON.stringify(value.latest) === JSON.stringify(value.current)));

export const CollaborationDeliveryHistorySchema = z.object({
  issue_id: uuid, items: z.array(CollaborationSubmissionSchema).max(100), next_cursor: z.string().min(1).max(512).nullable(),
}).refine((value) => (value.next_cursor === null || value.items.length > 0) &&
  new Set(value.items.map((item) => item.id)).size === value.items.length &&
  value.items.every((item, index) => item.issue_id === value.issue_id &&
    (index === 0 || BigInt(value.items[index - 1]!.event_seq) > BigInt(item.event_seq))));

/** Own recovery may expose only this original-write identity, never content. */
export const CollaborationDeliveryConfirmationSchema = z.strictObject({ submission_id: uuid, attachment_ids: collaborationAttachmentIds });
export type CollaborationSubmission = z.infer<typeof CollaborationSubmissionSchema>;
export type CollaborationDeliveryDecision = z.infer<typeof CollaborationDeliveryDecisionSchema>;
export type CollaborationDeliveryHistory = z.infer<typeof CollaborationDeliveryHistorySchema>;
export type CollaborationDeliveryConfirmation = z.infer<typeof CollaborationDeliveryConfirmationSchema>;

export const parseCollaborationDeliveryHistory = (data: unknown) => parseWithFallback<CollaborationDeliveryHistory | null>(
  data, CollaborationDeliveryHistorySchema, null, { endpoint: "collaboration delivery history", redact: true },
);

export function matchesCollaborationDeliveryConfirmation(confirmation: CollaborationDeliveryConfirmation | undefined, attachmentIds: readonly string[] = [], submissionId?: string) {
  return confirmation !== undefined && (submissionId === undefined || confirmation.submission_id === submissionId.toLowerCase()) &&
    confirmation.attachment_ids.length === attachmentIds.length &&
    attachmentIds.every((id) => confirmation.attachment_ids.includes(id.toLowerCase()));
}

/** Only explicitly recognized, validated final business refusals settle intent. */
export function collaborationDeliveryRefusalStatus(code: string): number | null {
  switch (code) {
    case "result_required": case "basis_required": case "result_too_large": case "basis_too_large":
    case "revision_reason_required": case "revision_reason_too_large": case "invalid_ai_policy":
    case "invalid_workflow_request": case "invalid_status_request": case "invalid_attachment_selection": case "invalid_comment_request": return 400;
    case "forbidden": return 403;
    case "attachment_not_found": case "comment_not_found": return 404;
    case "delivery_required": case "submission_changed": case "submission_not_pending": case "delivery_unavailable":
    case "acceptance_pending": case "task_manual": case "stale_revision": case "stale_authority":
    case "status_options_changed": case "status_unavailable": case "target_unavailable": case "return_target_unavailable":
    case "information_request_not_active": case "already_creator": case "issue_closed": case "attachment_scope_mismatch":
    case "attachment_draft_consumed": return 409;
    default: return null;
  }
}
