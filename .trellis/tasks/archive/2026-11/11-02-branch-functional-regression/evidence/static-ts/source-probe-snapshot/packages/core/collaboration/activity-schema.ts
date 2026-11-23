import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationAccountSummarySchema } from "./schema";
import { positiveVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const kind = z.enum(["created", "participant_added", "participant_removed", "participant_left", "transferred", "information_requested",
  "information_returned", "acceptance_submitted", "status_changed", "completed", "cancelled", "reopened", "issue_edited",
  "iteration_changed", "comment_created", "comment_edited", "comment_deleted", "other"]);
const change = z.object({
  field: z.enum(["title", "description", "start_date", "due_date", "priority", "labels", "properties", "status", "project_id", "parent_issue_id", "stage", "current_iteration_id"]),
  before: z.string().nullable(), after: z.string().nullable(),
});
const handoff = z.object({
  id: uuid, purpose: z.enum(["transfer", "information", "information_return", "acceptance", "removal_return"]),
  from: CollaborationAccountSummarySchema.nullable(), to: CollaborationAccountSummarySchema.nullable(), note: z.string(),
});
export const CollaborationActivityEntrySchema = z.object({
  id: uuid, issue_id: uuid, event_seq: positiveVersion, kind,
  actor: CollaborationAccountSummarySchema.nullable(), created_at: z.iso.datetime({ offset: true }),
  details_available: z.boolean(), target: CollaborationAccountSummarySchema.nullable(),
  note: z.string().max(2000).optional(),
  handoff: handoff.nullable(), comment_id: uuid.nullable(), changes: z.array(change),
}).refine((entry) => {
  if (!entry.details_available) return entry.target === null && entry.handoff === null && entry.comment_id === null && entry.changes.length === 0 && !entry.note;
  if (entry.note !== undefined && entry.kind !== "iteration_changed") return false;
  if (new Set(entry.changes.map((item) => item.field)).size !== entry.changes.length) return false;
  switch (entry.kind) {
    case "other": return false;
    case "comment_created": case "comment_edited": case "comment_deleted":
      return entry.comment_id !== null && entry.target === null && entry.handoff === null && entry.changes.length === 0;
    case "transferred": case "information_requested": case "information_returned": case "acceptance_submitted":
      return entry.handoff !== null && entry.comment_id === null && entry.changes.length === 0;
    case "status_changed": case "completed": case "cancelled": case "reopened":
      return entry.handoff === null && entry.comment_id === null && entry.changes.length === 1 && entry.changes[0]?.field === "status";
    case "issue_edited": return entry.handoff === null && entry.comment_id === null && entry.changes.every((item) => item.field !== "status");
    case "iteration_changed": return entry.handoff === null && entry.comment_id === null && entry.changes.length === 1 && entry.changes[0]?.field === "current_iteration_id";
    default: return entry.comment_id === null && entry.changes.length === 0;
  }
});
export const CollaborationActivitySchema = z.object({
  issue_id: uuid, items: z.array(CollaborationActivityEntrySchema), next_cursor: z.string().max(512).nullable(),
}).refine((page) => new Set(page.items.map((item) => item.id)).size === page.items.length && page.items.every((item, index) =>
  item.issue_id === page.issue_id && positiveVersion.safeParse(item.event_seq).success &&
  (index === 0 || positiveVersion.safeParse(page.items[index - 1]!.event_seq).success && BigInt(page.items[index - 1]!.event_seq) > BigInt(item.event_seq))));

export type CollaborationActivityEntry = z.infer<typeof CollaborationActivityEntrySchema>;
export type CollaborationActivity = z.infer<typeof CollaborationActivitySchema>;
export const parseCollaborationActivity = (data: unknown) => parseWithFallback<CollaborationActivity | null>(data, CollaborationActivitySchema, null, { endpoint: "collaboration activity", redact: true });
