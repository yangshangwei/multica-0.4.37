import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationKnowledgeProjectionSchema, CollaborationKnowledgeProtocolError } from "./knowledge-schema";
import { positiveVersion, nonnegativeVersion } from "./version";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const timestamp = z.iso.datetime({ offset: true });
const count = z.number().int().min(0).max(Number.MAX_SAFE_INTEGER);
const text = (max: number) => z.string().min(1).max(max).refine((value) => value.trim() !== "" && value.isWellFormed() && !value.includes("\0"));
const reasonCode = z.string().regex(/^[a-z][a-z0-9_]{0,95}$/);

export const ProjectKnowledgePolicySchema = CollaborationKnowledgeProjectionSchema.safeExtend({
  protocol_version: z.literal(2), project_id: uuid, workspace_id: uuid, revision: nonnegativeVersion,
  is_enabled: z.boolean(), max_candidates: z.number().int().min(1).max(3), rule_version: text(128),
  origin: z.enum(["system", "human"]), changed_by_user_id: uuid.nullable(), updated_at: timestamp.nullable(), can_manage: z.boolean(),
}).strict().refine((value) => value.origin === "system"
  ? value.revision === "0" && value.changed_by_user_id === null && value.updated_at === null
  : value.revision !== "0" && value.changed_by_user_id !== null && value.updated_at !== null);

export const ProjectKnowledgePolicyRequestSchema = z.strictObject({
  request_id: uuid, expected_revision: nonnegativeVersion, is_enabled: z.boolean(), max_candidates: z.number().int().min(1).max(3),
});
export const ProjectKnowledgePolicyReceiptSchema = z.strictObject({
  protocol_version: z.literal(2), request_id: uuid, project_id: uuid, revision: nonnegativeVersion,
  code: z.enum(["knowledge_policy_updated", "knowledge_policy_revision_conflict"]),
});
export const KnowledgeAutomationRetryRequestSchema = z.strictObject({ request_id: uuid, expected_revision: positiveVersion });
export const KnowledgeAutomationRetryReceiptSchema = z.strictObject({
  protocol_version: z.literal(2), request_id: uuid, issue_id: uuid, job_id: uuid, revision: positiveVersion,
  code: z.enum(["knowledge_job_retried", "knowledge_job_revision_conflict", "knowledge_source_changed", "knowledge_automation_disabled"]),
});
export const KnowledgeAutomationUsageSchema = z.strictObject({
  status: z.enum(["known", "partial", "unknown"]), input_tokens: count.nullable().optional(), output_tokens: count.nullable().optional(),
  total_tokens: count.nullable().optional(), cached_input_tokens: count.nullable().optional(), reasoning_tokens: count.nullable().optional(),
}).refine((value) => {
  const primary = [value.input_tokens, value.output_tokens, value.total_tokens];
  const all = [...primary, value.cached_input_tokens, value.reasoning_tokens];
  const present = (item: number | null | undefined) => item !== null && item !== undefined;
  return value.status === "known" ? primary.every(present)
    : value.status === "partial" ? !primary.every(present) && all.some(present) : !all.some(present);
});
export const KnowledgeAutomationJobSchema = z.strictObject({
  id: uuid, issue_id: uuid, project_id: uuid, submission_id: uuid, accepted_event_seq: positiveVersion, revision: positiveVersion,
  phase: z.enum(["queued", "generating", "checking", "settling", "completed", "review_pending", "waiting_configuration", "skipped", "cancelled"]),
  reason_code: reasonCode.nullable(), source_current: z.boolean(), can_retry: z.boolean(),
  counts: z.strictObject({ saved: count, restored: count, revised: count, pending: count, skipped: count }),
  attempts: z.array(z.strictObject({ stage: z.enum(["generating", "checking"]), state: z.enum(["running", "succeeded", "uncertain", "failed"]),
    model: text(512).nullable(), usage: KnowledgeAutomationUsageSchema })).max(42),
}).refine((value) => !value.can_retry || value.source_current && ["review_pending", "waiting_configuration"].includes(value.phase));
export const KnowledgeAutomationJobsSchema = CollaborationKnowledgeProjectionSchema.safeExtend({
  protocol_version: z.literal(2), issue_id: uuid, items: z.array(KnowledgeAutomationJobSchema).max(20), has_more: z.boolean(),
}).strict().refine((value) => new Set(value.items.map((item) => item.id)).size === value.items.length &&
  value.items.every((item) => item.issue_id === value.issue_id) && (!value.has_more || value.items.length === 20));

export type ProjectKnowledgePolicy = z.infer<typeof ProjectKnowledgePolicySchema>;
export type ProjectKnowledgePolicyRequest = z.infer<typeof ProjectKnowledgePolicyRequestSchema>;
export type ProjectKnowledgePolicyReceipt = z.infer<typeof ProjectKnowledgePolicyReceiptSchema>;
export type KnowledgeAutomationJob = z.infer<typeof KnowledgeAutomationJobSchema>;
export type KnowledgeAutomationJobs = z.infer<typeof KnowledgeAutomationJobsSchema>;
export type KnowledgeAutomationRetryRequest = z.infer<typeof KnowledgeAutomationRetryRequestSchema>;
export type KnowledgeAutomationRetryReceipt = z.infer<typeof KnowledgeAutomationRetryReceiptSchema>;
export type KnowledgeAutomationReceipt = ProjectKnowledgePolicyReceipt | KnowledgeAutomationRetryReceipt;

function parse<T>(data: unknown, schema: z.ZodType<T>): T | null {
  if (data === undefined || data === null || typeof data === "object" && !Object.hasOwn(data, "protocol_version")) return null;
  const parsed = parseWithFallback<T | null>(data, schema, null, { endpoint: "knowledge automation", redact: true });
  if (parsed === null) throw new CollaborationKnowledgeProtocolError();
  return parsed;
}
export const parseProjectKnowledgePolicy = (data: unknown) => parse(data, ProjectKnowledgePolicySchema);
export const parseProjectKnowledgePolicyReceipt = (data: unknown) => parse(data, ProjectKnowledgePolicyReceiptSchema);
export const parseKnowledgeAutomationJobs = (data: unknown) => parse(data, KnowledgeAutomationJobsSchema);
export const parseKnowledgeAutomationRetryReceipt = (data: unknown) => parse(data, KnowledgeAutomationRetryReceiptSchema);

/** A previous acceptance is historical, never evidence that the latest one is processing. */
export function selectKnowledgeAutomationJob(jobs: KnowledgeAutomationJobs | undefined, selection: {
  issueId: string; projectId: string | null; submissionId: string | null; acceptedEventSeq: string | null; aiPolicy: string | undefined;
}): KnowledgeAutomationJob | null {
  if (!jobs || selection.aiPolicy === "manual" || !selection.projectId || !selection.submissionId || !selection.acceptedEventSeq || jobs.issue_id !== selection.issueId) return null;
  return jobs.items.find((job) => job.project_id === selection.projectId && job.submission_id === selection.submissionId && job.accepted_event_seq === selection.acceptedEventSeq) ?? null;
}
