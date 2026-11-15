import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationAccountSummarySchema } from "./schema";
import { positiveVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const stageId = uuid.refine((value) => value !== "00000000-0000-0000-0000-000000000000");
const stageMetadata = {
  stage_roles_configured: z.strictObject({ role_revision: positiveVersion }),
  stage_roles_unchanged: z.strictObject({ role_revision: positiveVersion }),
  stage_proposal_created: z.strictObject({ proposal_id: stageId, candidate_id: stageId, version: positiveVersion }),
  stage_proposal_revised: z.strictObject({ proposal_id: stageId, candidate_id: stageId, version: positiveVersion, parent_candidate_id: stageId }),
  stage_proposal_accepted: z.strictObject({ proposal_id: stageId, candidate_id: stageId, version: positiveVersion, decision_id: stageId }),
  stage_proposal_rejected: z.strictObject({ proposal_id: stageId, candidate_id: stageId, version: positiveVersion, decision_id: stageId }),
  stage_scope_confirmed: z.strictObject({ decision_id: stageId, scope_revision: positiveVersion, lifecycle_epoch: positiveVersion }),
  stage_code_candidate_created: z.strictObject({ candidate_id: stageId, version: positiveVersion, parent_candidate_id: stageId.nullable() }),
  stage_decision_recorded: z.strictObject({ candidate_id: stageId, decision_id: stageId, step: z.enum(["code_review", "qa", "release", "rollback"]) }),
  stage_decision_revoked: z.strictObject({ decision_id: stageId, revocation_id: stageId }),
  stage_advanced: z.strictObject({ candidate_id: stageId, current: z.enum(["development", "code_review", "qa", "release", "verification"]), stage_revision: positiveVersion }),
};
function stageKind(value: string): value is keyof typeof stageMetadata { return Object.hasOwn(stageMetadata, value); }
const kind = z.enum(["created", "participant_added", "participant_removed", "participant_left", "transferred", "information_requested",
  "information_returned", "acceptance_submitted", "status_changed", "completed", "cancelled", "reopened", "issue_edited",
  "iteration_changed", "comment_created", "comment_edited", "comment_deleted", "other",
  "stage_roles_configured", "stage_roles_unchanged", "stage_proposal_created", "stage_proposal_revised", "stage_proposal_accepted", "stage_proposal_rejected",
  "stage_scope_confirmed", "stage_code_candidate_created", "stage_decision_recorded", "stage_decision_revoked", "stage_advanced", "stage_execution_admitted", "stage_execution_observed", "stage_rollback_observed"]);
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
  stage: z.unknown().optional(),
}).transform((entry) => {
  if (stageKind(entry.kind) || entry.kind === "stage_execution_admitted" || entry.kind === "stage_execution_observed" || entry.kind === "stage_rollback_observed") {
    const parsed = stageKind(entry.kind) ? stageMetadata[entry.kind].safeParse(entry.stage) : null;
    if (!parsed?.success || !entry.details_available || entry.target !== null || entry.handoff !== null || entry.comment_id !== null || entry.changes.length > 0 || entry.note !== undefined) {
      return { ...entry, stage: null, details_available: false, target: null, handoff: null, comment_id: null, changes: [], note: undefined };
    }
    return { ...entry, stage: parsed.data };
  }
  return { ...entry, stage: undefined };
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
