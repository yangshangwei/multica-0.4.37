import { z } from "zod";
import { IssueSchema } from "./schemas";
import { parseWithFallback } from "./schema";

const identity = z.string().min(1);
const count = z.number().int().nonnegative();
const instant = z.string().datetime({ offset: true });
const snapshot = z.record(z.string(), z.unknown());
export const TriageSettingsSchema = z.object({
  iteration_assignment: z.boolean().optional(),
  supported: z.literal(true), enabled: z.boolean(), acceptance_status: identity,
  require_priority: z.boolean(), responsibility_mode: z.string(), responsibility_member_id: identity.nullable(), revision: count,
}).loose();
export const TriageItemSchema = z.object({
  issue: IssueSchema.extend({ id: identity, workspace_id: identity, admission_status: identity, revision: z.number().int().positive() }),
  candidate_project_id: identity.nullable(), candidate_assignee_type: z.string().nullable(), candidate_assignee_id: identity.nullable(),
  reviewer_id: identity.nullable(), reviewer_valid: z.boolean(), round: z.number().int().positive(),
  first_entered_at: instant, entered_at: instant, snoozed_until: instant.nullable(),
  duplicate_issue_id: identity.nullable(), duplicate_identifier: z.string().nullable(), source: identity,
  source_url: z.string().nullable(), external_id: z.string().nullable(), batch_id: identity.nullable(), filename: z.string().nullable(), row_number: z.number().int().positive().nullable(),
}).loose();
export const TriageActionSchema = z.object({
  id: identity, issue_id: identity, actor_id: identity, action: identity, round: z.number().int().positive(),
  reason: z.string().nullable(), before: snapshot, after: snapshot, created_at: instant,
  execution_status: identity, task_id: identity.nullable(), execution_error: z.string().nullable(),
}).loose().refine(action => action.execution_status !== "queued" || action.task_id !== null, "A queued execution must retain its task identity");
export const TriageActionResultSchema = z.object({ item: TriageItemSchema, action: TriageActionSchema }).loose().refine(result => result.item.issue.id === result.action.issue_id, "Action and item identities must agree");
export const TriageCountsSchema = z.object({ pending: count, ready: count, snoozed: count }).loose();
export const TriageListResponseSchema = z.object({ items: z.array(TriageItemSchema), total: count, counts: TriageCountsSchema, limit: count, offset: count }).loose();
export const TriageItemHistorySchema = z.object({ events: z.array(TriageActionSchema) }).loose();
const importCounts = z.object({ created: count, skipped: count, failed: count }).loose();
export const TriageHistoryEntrySchema = z.object({
  id: identity, kind: identity, issue_id: identity.nullable(), identifier: z.string().nullable(), title: z.string(), action: identity, actor_id: identity, created_at: instant,
  reason: z.string().nullable(), before: snapshot, after: snapshot, batch_id: identity.nullable(), filename: z.string().nullable(), counts: importCounts.nullable(),
}).loose();
export const TriageHistoryResponseSchema = z.object({ entries: z.array(TriageHistoryEntrySchema), total: count, limit: count, offset: count }).loose();
export const TriageBatchPreviewSchema = z.object({ items: z.array(z.object({ issue_id: identity, expected_revision: count, valid: z.boolean(), error: z.string().nullable() }).loose()), valid_count: count }).loose().refine(result => result.valid_count === result.items.filter(row => row.valid === true).length, "Preview count must agree with valid rows");
export const TriageBatchResultSchema = z.object({
  results: z.array(z.object({ issue_id: identity, status: identity, result: TriageActionResultSchema.optional(), error: z.string().optional() }).loose().refine(row => row.status !== "success" || row.result !== undefined, "A successful decision must include its receipt").refine(row => !row.result || row.issue_id === row.result.item.issue.id, "Batch row and receipt identities must agree")),
  success_count: count,
}).loose().refine(result => result.success_count === result.results.filter(row => row.status === "success").length, "Success count must agree with receipts");
export const TriageImportRowSchema = z.object({
  row_number: z.number().int().positive(), values: z.record(z.string(), z.string()), warnings: z.array(z.string()), errors: z.array(z.string()),
  duplicate: z.boolean(), duplicate_issue_id: identity.nullable(), similar_issue_ids: z.array(identity), status: identity, issue_id: identity.nullable(), error: z.string().nullable(),
}).loose().refine(row => row.status !== "created" || row.issue_id !== null, "A created row must include its issue identity");
export const TriageImportPreviewSchema = z.object({
  batch_id: identity, filename: z.string(), headers: z.array(z.string()), mapping: z.record(z.string(), z.string()), rows: z.array(TriageImportRowSchema),
  counts: z.object({ valid: count, warning: count, error: count, duplicate: count }).loose(),
  limits: z.object({ max_rows: z.number().int().positive(), max_bytes: z.number().int().positive() }).loose(),
}).loose();
export const TriageImportResultSchema = importCounts.extend({ batch_id: identity, results: z.array(TriageImportRowSchema) }).refine(result =>
  result.created === result.results.filter(row => row.status === "created").length &&
  result.skipped === result.results.filter(row => row.status === "skipped").length &&
  result.failed === result.results.filter(row => row.status === "failed").length,
  "Import counts must agree with selected row outcomes",
);
export const TriageUpdatedPayloadSchema = z.object({ workspace_id: identity, issue_id: identity.optional(), batch_id: identity.optional(), settings_changed: z.boolean().optional() }).loose();

/** Authorization receipts and their projections have no safe fabricated success. */
export function parseTriageResponse<T>(raw: unknown, schema: z.ZodType, endpoint: string, matchesRequest?: (value: T) => boolean): T {
  const result = parseWithFallback<T | null>(raw, schema, null, { endpoint, redact: true });
  if (result === null || (matchesRequest && !matchesRequest(result))) throw new Error(`${endpoint} returned a malformed triage response`);
  return result;
}
