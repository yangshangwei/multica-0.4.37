import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationAccountSummarySchema } from "./schema";
import { positiveVersion } from "./version";

export const CollaborationParticipantsSchema = z.object({
  protocol_version: z.literal(1), issue_id: z.uuid().transform((value) => value.toLowerCase()), collaboration_revision: positiveVersion,
  participants: z.array(CollaborationAccountSummarySchema.extend({ account_active: z.boolean() })),
}).refine((value) => new Set(value.participants.map((item) => item.user_id.toLowerCase())).size === value.participants.length);
export type CollaborationParticipants = z.infer<typeof CollaborationParticipantsSchema>;
export function parseCollaborationParticipants(value: unknown): CollaborationParticipants | null {
  return parseWithFallback<CollaborationParticipants | null>(value, CollaborationParticipantsSchema, null, { endpoint: "collaboration participants", redact: true });
}
