import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { positiveVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
export const COLLABORATION_ATTACHMENT_MAX_BYTES = 104857600;
export const collaborationAttachmentIds = z.array(uuid).max(32).refine((ids) => new Set(ids).size === ids.length);
export const CollaborationAttachmentSchema = z.object({
  id: uuid, draft_id: uuid, issue_id: uuid.nullable(), intended_issue_id: uuid.nullable(), source_workspace_id: uuid, uploader_user_id: uuid,
  name: z.string().min(1).max(1024), content_type: z.string().min(1).max(256),
  size_bytes: z.number().int().nonnegative().max(COLLABORATION_ATTACHMENT_MAX_BYTES), version: positiveVersion,
  created_at: z.iso.datetime({ offset: true }), delivered_at: z.iso.datetime({ offset: true }).nullable(),
}).refine((value) => (value.issue_id === null) === (value.delivered_at === null) &&
  (value.issue_id === null || value.intended_issue_id === null || value.issue_id === value.intended_issue_id));
export const CollaborationAttachmentsSchema = z.object({ issue_id: uuid, attachments: z.array(CollaborationAttachmentSchema) })
  .refine((value) => new Set(value.attachments.map((file) => file.id)).size === value.attachments.length &&
    value.attachments.every((file) => file.issue_id === value.issue_id));
export const CollaborationDraftAttachmentsSchema = z.object({ draft_id: uuid, attachments: z.array(CollaborationAttachmentSchema).max(32) })
  .refine((value) => new Set(value.attachments.map((file) => file.id)).size === value.attachments.length &&
    value.attachments.every((file) => file.draft_id === value.draft_id));
export const CollaborationAttachmentUploadInputSchema = z.union([
  z.strictObject({ draft_id: uuid, source_workspace_id: uuid }),
  z.strictObject({ draft_id: uuid, issue_id: uuid }),
]);
export const CollaborationAttachmentSelectionSchema = z.strictObject({ draft_id: uuid, attachment_ids: collaborationAttachmentIds.min(1) });
export const CollaborationAttachmentDeliveryReceiptSchema = z.object({
  status: z.literal(200), code: z.literal("attachments_delivered"), protocol_version: z.literal(2),
  issue_id: uuid, attachment_ids: collaborationAttachmentIds.min(1), event_id: uuid, event_seq: positiveVersion,
});
export const CollaborationAttachmentCapabilitySchema = z.object({
  issue_id: uuid, attachment_id: uuid, version: positiveVersion, url: z.string().min(1).max(8192), expires_at: z.iso.datetime({ offset: true }),
});

function parse<T>(data: unknown, schema: z.ZodType<T>, endpoint: string): T | null {
  return parseWithFallback<T | null>(data, schema, null, { endpoint, redact: true });
}
export const parseCollaborationAttachment = (data: unknown) => parse(data, CollaborationAttachmentSchema, "collaboration attachment");
export const parseCollaborationAttachments = (data: unknown) => parse(data, CollaborationAttachmentsSchema, "collaboration attachments");
export const parseCollaborationDraftAttachments = (data: unknown) => parse(data, CollaborationDraftAttachmentsSchema, "collaboration draft attachments");
export const parseCollaborationAttachmentDeliveryReceipt = (data: unknown) => parse(data, CollaborationAttachmentDeliveryReceiptSchema, "collaboration attachment delivery");

/** Native loaders receive only a current API capability for this exact file. */
export function parseCollaborationAttachmentCapability(data: unknown, apiOrigin: string, issueId: string, attachmentId: string, version: string, now = Date.now()) {
  const value = parse(data, CollaborationAttachmentCapabilitySchema, "collaboration attachment capability");
  if (!value || value.issue_id !== issueId.toLowerCase() || value.attachment_id !== attachmentId.toLowerCase() || value.version !== version) return null;
  try {
    const base = new URL(apiOrigin);
    const url = new URL(value.url, base);
    const expires = Date.parse(value.expires_at);
    if (!["http:", "https:"].includes(url.protocol) || url.origin !== base.origin || url.username || url.password || url.hash ||
      url.pathname !== `/api/collaboration/issues/${value.issue_id}/attachments/${value.attachment_id}/capability` ||
      [...url.searchParams.keys()].length !== 1 || url.searchParams.getAll("cap").length !== 1 || !url.searchParams.get("cap") ||
      expires <= now || expires > now + 65_000) return null;
    return { ...value, url: url.toString() };
  } catch { return null; }
}
export type CollaborationAttachment = z.infer<typeof CollaborationAttachmentSchema>;
export type CollaborationAttachmentUploadInput = z.infer<typeof CollaborationAttachmentUploadInputSchema>;
export type CollaborationAttachmentSelection = z.infer<typeof CollaborationAttachmentSelectionSchema>;
