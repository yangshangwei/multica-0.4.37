import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { positiveVersion } from "./version";
import { CollaborationExecutionViewVersionSchema } from "./execution-schema";

const uuid = z.uuid().transform((value) => value.toLowerCase());
export const CollaborationCommandCancelInputSchema = z.strictObject({
  execution_view_version: CollaborationExecutionViewVersionSchema.optional(),
  expected_lifecycle_epoch: positiveVersion, expected_participant_epoch: positiveVersion, expected_account_epoch: positiveVersion,
  target_id: uuid, expected_collaboration_revision: positiveVersion,
});
export type CollaborationCommandCancelInput = z.infer<typeof CollaborationCommandCancelInputSchema>;
const result = { protocol_version: z.literal(2), issue_id: uuid, request_id: z.string().min(1).max(128), action: z.literal("cancel_command"), status: z.number().int().min(200).max(599),
  code: z.enum(["command_cancelled", "command_not_waiting", "stale_revision", "stale_authority", "stale_scope", "forbidden", "not_found", "target_not_found", "issue_closed", "collaboration_client_upgrade_required"]) };
const redacted = z.strictObject({ ...result, redacted: z.literal(true) });
const full = z.object({ ...result, redacted: z.literal(false).optional(), scope: z.enum(["target", "issue"]), target_id: uuid.nullable(),
  command_id: uuid.nullable(), command_sequence: positiveVersion.optional(), channel_id: uuid.nullable(), batch_id: uuid.nullable(), group_id: uuid.nullable(),
  queue_state: z.enum(["active", "paused", "closed"]), control_version: positiveVersion, group_settled: z.boolean(),
  run_id: z.null(), run_settled: z.literal(false), grant_id: z.null(), channels: z.array(z.never()).length(0), target_holds: z.array(z.never()).length(0),
}).refine((value) => value.code !== "command_cancelled" || value.target_id !== null && value.command_id !== null && value.command_sequence !== undefined && value.channel_id !== null && value.batch_id !== null && value.scope === "target");
export const CollaborationCommandCancelReceiptSchema = z.union([redacted, full]).refine((value) => value.code === "command_cancelled" ? value.status === 200 : value.status >= 400);
export type CollaborationCommandCancelReceipt = z.infer<typeof CollaborationCommandCancelReceiptSchema>;
export function parseCollaborationCommandCancelReceipt(data: unknown) {
  return parseWithFallback<CollaborationCommandCancelReceipt | null>(data, CollaborationCommandCancelReceiptSchema, null, { endpoint: "collaboration command cancellation", redact: true });
}
export function matchesCollaborationCommandCancelReceipt(receipt: CollaborationCommandCancelReceipt, issueId: string, requestId: string, commandId: string, input: CollaborationCommandCancelInput) {
  return receipt.issue_id === issueId.toLowerCase() && receipt.request_id === requestId && (receipt.redacted === true ||
    (receipt.command_id === null || receipt.command_id === commandId.toLowerCase()) && (receipt.target_id === null || receipt.target_id === input.target_id.toLowerCase()));
}
