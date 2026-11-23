import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { collaborationAttachmentIds } from "./attachment-schema";
import { CollaborationAccountSummarySchema } from "./schema";
import { positiveVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const timestamp = z.iso.datetime({ offset: true }).max(35).refine((value) => (value.match(/\.(\d+)/)?.[1]?.length ?? 0) <= 9);
const account = CollaborationAccountSummarySchema.extend({ user_id: uuid });

export const CollaborationCommandHistoryTargetSchema = z.object({ kind: z.enum(["agent", "squad"]), resource_id: uuid });
export const CollaborationCommandCancellationSchema = z.object({
  event_id: uuid.nullable(), event_seq: positiveVersion.nullable(),
  actor_user_id: uuid.nullable(), actor: account.nullable(),
  created_at: timestamp.nullable(), kind: z.enum(["manual", "automatic"]).nullable(),
}).refine((value) => {
  if (value.event_id === null) return Object.values(value).every((field) => field === null);
  return value.event_seq !== null && value.created_at !== null && value.kind !== null &&
    (value.kind !== "manual" || value.actor_user_id !== null) &&
    (value.actor === null || value.actor.user_id === value.actor_user_id);
}, "Cancellation requires either exact evidence or explicitly unknown legacy facts");

export const CollaborationCommandHistoryEntrySchema = z.object({
  id: uuid, session_id: uuid, target_id: uuid, target: CollaborationCommandHistoryTargetSchema,
  input_event_id: uuid, input_event_seq: positiveVersion.nullable(), comment_id: uuid.nullable(),
  channel_id: uuid, batch_id: uuid, group_id: uuid.nullable(), seq: positiveVersion,
  author_user_id: uuid, author: account.nullable(), content: z.string(), attachment_ids: collaborationAttachmentIds,
  state: z.enum(["waiting", "in_flight", "succeeded", "failed", "stopped", "cancelled"]),
  reason: z.string().min(1).nullable(), created_at: timestamp, cancellation: CollaborationCommandCancellationSchema.nullable(),
}).refine((entry) => {
  if ((entry.author !== null && entry.author.user_id !== entry.author_user_id) ||
    (entry.input_event_seq === null) !== (entry.comment_id === null) ||
    (entry.state === "cancelled") !== (entry.cancellation !== null)) return false;
  const cancellation = entry.cancellation;
  if (cancellation?.event_id == null) return true;
  if (entry.reason === null || cancellation.event_id === entry.input_event_id ||
    (cancellation.kind === "manual" && entry.reason !== "cancelled_by_participant")) return false;
  // Event sequence is the ordering fact; transaction-start timestamps may be
  // older even when the cancellation committed after the original input.
  const before = entry.input_event_seq;
  const after = cancellation.event_seq;
  return before === null || (after !== null && (after.length > before.length || (after.length === before.length && after > before)));
}, "Command author, source and cancellation must name the same recorded facts");

export type CollaborationCommandHistoryEntry = z.infer<typeof CollaborationCommandHistoryEntrySchema>;

function timestampOrder(value: string): bigint | null {
  const fraction = value.match(/\.(\d+)/)?.[1] ?? "";
  const milliseconds = Date.parse(value.replace(/\.\d+/, ""));
  if (fraction.length > 9 || !Number.isFinite(milliseconds)) return null;
  return BigInt(milliseconds) * 1_000_000n + BigInt(fraction.padEnd(9, "0"));
}

/** Shared by the page boundary and the assembled keyset chain before caching. */
export function hasValidCollaborationCommandHistoryItems(items: CollaborationCommandHistoryEntry[]): boolean {
  const ids = new Set<string>();
  const positions = new Set<string>();
  const sources = new Map<string, string>();
  const targets = new Map<string, string>();
  const cancellations = new Set<string>();
  const events = new Map<string, string>();
  let previous: CollaborationCommandHistoryEntry | undefined;
  for (const entry of items) {
    if (entry.session_id !== items[0]?.session_id || ids.has(entry.id)) return false;
    ids.add(entry.id);
    const position = `${entry.channel_id}:${entry.seq}`;
    if (positions.has(position)) return false;
    positions.add(position);
    if (previous) {
      const before = timestampOrder(previous.created_at);
      const after = timestampOrder(entry.created_at);
      if (before === null || after === null || before < after || (before === after && previous.id <= entry.id)) return false;
    }
    previous = entry;
    const source = JSON.stringify([entry.input_event_seq, entry.comment_id, entry.author_user_id, entry.content, [...entry.attachment_ids].sort()]);
    const oldSource = sources.get(entry.input_event_id);
    if (cancellations.has(entry.input_event_id) || (oldSource !== undefined && oldSource !== source)) return false;
    sources.set(entry.input_event_id, source);
    const target = `${entry.target.kind}:${entry.target.resource_id}`;
    const oldTarget = targets.get(entry.target_id);
    if (oldTarget !== undefined && oldTarget !== target) return false;
    targets.set(entry.target_id, target);
    const cancellation = entry.cancellation;
    if (cancellation?.event_id != null) {
      if (sources.has(cancellation.event_id) || cancellations.has(cancellation.event_id)) return false;
      cancellations.add(cancellation.event_id);
    }
    for (const [seq, id] of [[entry.input_event_seq, entry.input_event_id], [cancellation?.event_seq, cancellation?.event_id]]) {
      if (seq == null || id == null) continue;
      const oldEvent = events.get(seq);
      if (oldEvent !== undefined && oldEvent !== id) return false;
      events.set(seq, id);
    }
  }
  return true;
}

export const CollaborationCommandHistorySchema = z.object({
  protocol_version: z.literal(1), issue_id: uuid, items: z.array(CollaborationCommandHistoryEntrySchema).max(100),
  next_cursor: z.string().min(1).max(1024).nullable(),
}).refine((page) => (page.next_cursor === null || page.items.length > 0) && hasValidCollaborationCommandHistoryItems(page.items),
  "History pages require consistent identities and descending created-at/ID order");
export type CollaborationCommandHistory = z.infer<typeof CollaborationCommandHistorySchema>;

export function parseCollaborationCommandHistory(data: unknown): CollaborationCommandHistory | null {
  return parseWithFallback<CollaborationCommandHistory | null>(data, CollaborationCommandHistorySchema, null, {
    endpoint: "collaboration command history", redact: true,
  });
}
