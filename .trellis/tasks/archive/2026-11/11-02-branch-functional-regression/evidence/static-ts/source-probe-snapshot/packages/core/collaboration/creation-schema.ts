import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationFieldCatalogSchema, CollaborationPropertyPatchSchema, CollaborationSourceOptionsSchema } from "./schema";
import { positiveVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const source = z.object({ id: uuid, name: z.string(), slug: z.string().min(1) });
export const CollaborationCreationOptionsSchema = z.object({
  creation_enabled: z.boolean(), sources: z.array(source), source: source.nullable(),
  max_attachment_bytes: z.number().int().positive().max(104857600),
  fields: CollaborationFieldCatalogSchema.extend({
    statuses: z.array(z.object({ key: z.string().min(1), name: z.string(), category: z.enum(["backlog", "todo", "in_progress", "in_review", "blocked"]) })),
    default_status: z.string().min(1), source_options: CollaborationSourceOptionsSchema,
  }).refine((value) => new Set(value.statuses.map((item) => item.key)).size === value.statuses.length && value.statuses.some((item) => item.key === value.default_status)).optional(),
}).refine((value) => new Set(value.sources.map((item) => item.id)).size === value.sources.length &&
  (value.source === null ? value.fields === undefined : value.sources.some((item) => item.id === value.source?.id && item.slug === value.source.slug)));

export const CollaborationCreationFieldsSchema = z.strictObject({
  status: z.string().min(1).optional(), priority: z.enum(["none", "low", "medium", "high", "urgent"]).optional(),
  start_date: z.iso.date().nullable().optional(), due_date: z.iso.date().nullable().optional(),
  label_ids: z.array(uuid).optional(), properties: CollaborationPropertyPatchSchema.optional(),
  project_id: uuid.nullable().optional(), parent_issue_id: uuid.nullable().optional(), stage: z.number().int().min(1).max(2147483647).nullable().optional(),
  current_iteration_id: uuid.nullable().optional(), expected_iteration_revision: positiveVersion.optional(),
}).refine((value) => Boolean(value.current_iteration_id) === (value.expected_iteration_revision !== undefined));
export type CollaborationCreationFields = z.infer<typeof CollaborationCreationFieldsSchema>;

export const CollaborationCreationInputSchema = z.strictObject({
  ...CollaborationCreationFieldsSchema.shape,
  source_workspace_id: uuid, title: z.string().trim().min(1).max(1000), description: z.string().max(1_000_000).optional(),
  handler_user_id: uuid.optional(),
  draft_id: uuid.optional(), attachment_ids: z.array(uuid).max(32).optional(),
}).refine((value) => new Set(value.attachment_ids).size === (value.attachment_ids?.length ?? 0) &&
  (!value.attachment_ids?.length || value.draft_id !== undefined) &&
  Boolean(value.current_iteration_id) === (value.expected_iteration_revision !== undefined));

export type CollaborationCreationOptions = z.infer<typeof CollaborationCreationOptionsSchema>;
export type CollaborationCreationInput = z.infer<typeof CollaborationCreationInputSchema>;
export const parseCollaborationCreationOptions = (data: unknown) => parseWithFallback<CollaborationCreationOptions | null>(
  data, CollaborationCreationOptionsSchema, null, { endpoint: "collaboration creation options", redact: true },
);

export const CollaborationCreationRejectionCodeSchema = z.enum([
  "account_unavailable", "collaboration_unavailable", "invalid_attachment_selection", "attachment_not_found",
  "attachment_scope_mismatch", "attachment_draft_consumed", "parent_issue_not_found", "project_not_found",
  "triage_review_required", "issue_limit_reached",
  "status_unavailable", "invalid_priority", "invalid_start_date", "invalid_due_date", "invalid_labels", "invalid_properties",
  "property_archived", "invalid_property_value", "properties_too_large", "invalid_stage", "parent_cycle",
  "iteration_disabled", "iteration_not_found", "iteration_revision_conflict", "iteration_history_move_unsupported", "iteration_validation_failed",
]);
const resultIdentity = { request_id: z.string().min(1).max(128), source_workspace_id: uuid, actor_user_id: uuid };
export const CollaborationCreationResultSchema = z.discriminatedUnion("outcome", [
  z.strictObject({ ...resultIdentity, outcome: z.literal("created"), code: z.literal("created") }),
  z.strictObject({ ...resultIdentity, outcome: z.literal("rejected"), code: CollaborationCreationRejectionCodeSchema }),
]);
export type CollaborationCreationResult = z.infer<typeof CollaborationCreationResultSchema>;
export type CollaborationCreationRejectionCode = z.infer<typeof CollaborationCreationRejectionCodeSchema>;
export const parseCollaborationCreationResult = (data: unknown) => parseWithFallback<CollaborationCreationResult | null>(
  data, CollaborationCreationResultSchema, null, { endpoint: "collaboration creation result", redact: true },
);
