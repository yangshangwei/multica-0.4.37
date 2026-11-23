import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { COLLABORATION_REACTION_MAX_BYTES, collaborationTextByteLength } from "./text-limits";

const uuid = z.uuid().transform((value) => value.toLowerCase());
export const CollaborationReactionSchema = z.object({ emoji: z.string().min(1), count: z.number().int().positive().safe(), viewer_reacted: z.boolean() });
const reactionEmoji = z.string().min(1).refine((value) => collaborationTextByteLength(value) <= COLLABORATION_REACTION_MAX_BYTES && value === value.trim() && !/[\r\n/\0]/.test(value));
export const CollaborationDiscussionSelectionSchema = z.discriminatedUnion("action", [
  z.strictObject({ action: z.enum(["resolve", "unresolve"]), commentId: uuid }),
  z.strictObject({ action: z.enum(["add_reaction", "remove_reaction"]), commentId: uuid.nullable(), emoji: reactionEmoji }),
]);
export type CollaborationDiscussionSelection = z.infer<typeof CollaborationDiscussionSelectionSchema>;
export const CollaborationIssueReactionsSchema = z.object({ issue_id: uuid, items: z.array(CollaborationReactionSchema) })
  .refine((value) => new Set(value.items.map((item) => item.emoji)).size === value.items.length);

export const CollaborationDiscussionReceiptSchema = z.strictObject({
  protocol_version: z.literal(2), issue_id: uuid, comment_id: uuid.nullable(), request_id: uuid,
  action: z.enum(["resolve", "unresolve", "add_reaction", "remove_reaction"]), status: z.number().int(), code: z.string(),
  emoji: reactionEmoji.nullable(),
}).refine((value) => {
  const resolving = value.action === "resolve" || value.action === "unresolve";
  if (resolving ? value.comment_id === null || value.emoji !== null : value.emoji === null) return false;
  if (value.status === 200) return value.code === "unchanged" || value.code === {
    resolve: "comment_resolved", unresolve: "comment_unresolved", add_reaction: "reaction_added", remove_reaction: "reaction_removed",
  }[value.action];
  return (value.status === 409 && value.comment_id !== null && value.code === "comment_deleted" && (value.action === "resolve" || value.action === "add_reaction")) || (value.status === 404 && value.code === "not_found");
});
export type CollaborationDiscussionReceipt = z.infer<typeof CollaborationDiscussionReceiptSchema>;
export const parseCollaborationDiscussionReceipt = (data: unknown) => parseWithFallback<CollaborationDiscussionReceipt | null>(data, CollaborationDiscussionReceiptSchema, null, { endpoint: "collaboration discussion receipt", redact: true });
export const parseCollaborationIssueReactions = (data: unknown) => parseWithFallback<z.infer<typeof CollaborationIssueReactionsSchema> | null>(data, CollaborationIssueReactionsSchema, null, { endpoint: "collaboration issue reactions", redact: true });
