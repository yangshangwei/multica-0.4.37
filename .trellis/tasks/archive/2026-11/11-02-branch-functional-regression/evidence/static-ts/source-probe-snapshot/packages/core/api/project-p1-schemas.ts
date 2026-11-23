import { z } from "zod";
import { IssueSchema, AgentTaskSchema } from "./schemas";
import { parseWithFallback } from "./schema";

const uuid = z.string().uuid();
const revision = z.number().int().positive().safe();
const count = z.number().int().nonnegative().safe();
const safeHref = z.string().refine((value) => {
  if (value.startsWith("/") && !value.startsWith("//")) return true;
  try { const url = new URL(value); return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password; } catch { return false; }
});
const timestamp = z.iso.datetime({ offset: true });
const identity = { workspace_id: uuid, project_id: uuid };
export const ProjectCapabilitiesSchema = z.object({
  workspace_id: uuid, schema_version: z.literal(1), overview: z.boolean(), updates: z.boolean(),
  description_cas: z.boolean(), planning_timezone: z.boolean(),
}).strict();
export const ProjectPlanningTimezoneSchema = z.object({
  workspace_id: uuid, planning_timezone: z.string().nullable(), effective_timezone: z.string().min(1), configured: z.boolean(),
}).strict();
export const HistoricalMemberSchema = z.object({
  id: uuid, name: z.string().nullable(), avatar_url: z.string().nullable(),
  availability: z.enum(["active", "departed", "deleted"]),
}).strict();
const conclusion = z.enum(["passed", "partial", "failed"]);
const kind = z.enum(["progress", "risk", "acceptance"]);
const judgment = z.enum(["on_track", "attention", "risk"]).nullable();
export const ProjectStatisticsSnapshotSchema = z.object({
  ...identity, project_revision: revision, snapshot_version: z.string().min(1), calculated_at: timestamp,
  reference_date: z.iso.date(), timezone: z.string().min(1), timezone_configured: z.boolean(),
  complete: z.boolean(), incomplete_reasons: z.array(z.string()),
  counts: z.object({ total: count.nullable(), completed: count.nullable(), cancelled: count.nullable(),
    open: count.nullable(), blocked: count.nullable(), overdue: count.nullable(), unassigned: count.nullable(),
    in_review: count.nullable(), risk_union: count.nullable(), unknown_status: count.nullable(),
    execution_environment_unavailable: count.nullable() }).strict(),
  closure_ratio: z.number().min(0).max(1).nullable(), project_overdue: z.boolean().nullable(),
  lead_valid: z.boolean().nullable(), latest_update_at: timestamp.nullable(), progress_age_days: count.nullable(),
  health: z.enum(["unavailable", "empty", "risk", "attention", "clear"]), reasons: z.array(z.string()),
}).strict().superRefine((value, ctx) => {
  if (value.complete && Object.values(value.counts).some((n) => n === null)) {
    ctx.addIssue({ code: "custom", message: "Complete statistics require every count" });
  }
  if (!value.complete && value.health !== "unavailable") {
    ctx.addIssue({ code: "custom", message: "Incomplete statistics cannot assert health" });
  }
  if (value.counts.total === 0 && value.closure_ratio !== null) {
    ctx.addIssue({ code: "custom", message: "An empty scope has no closure ratio" });
  }
});
export const ProjectAcceptanceSummarySchema = z.object({
  update_id: uuid, revision, published_at: timestamp, conclusion, description_revision: revision,
  applicable_to_current_description: z.boolean(), author: HistoricalMemberSchema,
}).strict();
export const ProjectOverviewSchema = z.object({
  ...identity, description_revision: revision, statistics: ProjectStatisticsSnapshotSchema,
  latest_acceptance: ProjectAcceptanceSummarySchema.nullable(),
  current_description_acceptance: ProjectAcceptanceSummarySchema.nullable(),
}).strict();
export const ProjectRiskSignalSchema = z.enum(["blocked", "overdue", "unassigned", "in_review"]);
export const ProjectRiskPageSchema = z.object({
  ...identity, signal: ProjectRiskSignalSchema, items: z.array(IssueSchema), total: count,
  snapshot_version: z.string().min(1), refreshed: z.boolean(), overview: ProjectOverviewSchema,
  next_cursor: z.string().nullable(),
}).strict();
export const ProjectEvidenceInputSchema = z.object({
  kind: z.enum(["issue", "execution", "url"]), id: uuid.nullable(), url: z.string().nullable(),
}).strict().superRefine((value, ctx) => {
  if (value.kind === "url") {
    try {
      const url = new URL(value.url ?? "");
      if (!["http:", "https:"].includes(url.protocol) || url.username || url.password || value.id !== null) throw new Error();
    } catch { ctx.addIssue({ code: "custom", message: "Evidence URL must use HTTP or HTTPS without credentials" }); }
  } else if (value.id === null || value.url !== null) ctx.addIssue({ code: "custom", message: "Evidence must identify its source" });
});
export const ProjectEvidenceVersionSchema = z.discriminatedUnion("kind", [
  z.object({ kind: z.literal("issue"), id: uuid, revision }).strict(),
  z.object({ kind: z.literal("execution"), id: uuid, state_version: count, result_digest: z.string().min(1) }).strict(),
  z.object({ kind: z.literal("url"), url: z.string(), verification: z.literal("unverified") }).strict(),
]);
export const ProjectEvidenceViewSchema = z.object({
  input: ProjectEvidenceInputSchema, observed_version: ProjectEvidenceVersionSchema, collected_at: timestamp,
  availability: z.enum(["available", "changed", "deleted", "inaccessible", "unverified"]),
  current_version: ProjectEvidenceVersionSchema.nullable(), label: z.string().nullable(), href: safeHref.nullable(),
}).strict();
const acceptanceInput = z.object({ conclusion, scope: z.string(), explanation: z.string().nullable() }).strict();
export const ProjectAcceptanceSchema = acceptanceInput.extend({
  description_revision: revision, description_snapshot: z.string(),
}).strict();
export const ProjectUpdateDraftSchema = z.object({
  operation: z.enum(["create", "correct"]), update_id: uuid.nullable(), expected_revision: revision.nullable(),
  kind, body: z.string(), health_judgment: judgment, evidence: z.array(ProjectEvidenceInputSchema),
  acceptance: acceptanceInput.nullable(), expected_description_revision: revision.nullable(),
  include_statistics: z.boolean(), correction_reason: z.string().nullable(),
}).strict();
export const ProjectUpdatePreviewSchema = z.object({
  ...identity, draft: ProjectUpdateDraftSchema, evidence_versions: z.array(ProjectEvidenceVersionSchema),
  description_revision: revision.nullable(), recipients: z.array(HistoricalMemberSchema),
  statistics_snapshot: ProjectStatisticsSnapshotSchema.nullable(), preview_hash: z.string().min(1), previewed_at: timestamp,
}).strict();
export const ProjectUpdateRevisionSchema = z.object({
  ...identity, update_id: uuid, revision, editor: HistoricalMemberSchema, created_at: timestamp,
  kind, body: z.string(), health_judgment: judgment, correction_reason: z.string().nullable(),
  evidence: z.array(ProjectEvidenceViewSchema), statistics_snapshot: ProjectStatisticsSnapshotSchema.nullable(),
  acceptance: ProjectAcceptanceSchema.nullable(),
}).strict().superRefine((value, ctx) => {
  if ((value.kind === "acceptance") !== (value.acceptance !== null)) ctx.addIssue({ code: "custom", message: "Acceptance payload must match update kind" });
});
export const ProjectUpdateSchema = z.object({
  ...identity, id: uuid, author: HistoricalMemberSchema, published_at: timestamp,
  current_revision: revision, current: ProjectUpdateRevisionSchema,
}).strict();
export const ProjectUpdatesPageSchema = z.object({
  ...identity, items: z.array(ProjectUpdateSchema), next_cursor: z.string().nullable(),
}).strict();
export const ProjectUpdateRevisionsPageSchema = z.object({
  ...identity, update_id: uuid, items: z.array(ProjectUpdateRevisionSchema), next_cursor: z.string().nullable(),
}).strict();
export const ProjectUpdateWriteInputSchema = z.object({
  request_id: uuid, draft: ProjectUpdateDraftSchema, preview_hash: z.string().min(1),
  evidence_versions: z.array(ProjectEvidenceVersionSchema),
}).strict();
export const ProjectUpdateWriteResultSchema = z.object({
  ...identity, request_id: uuid, update_id: uuid, result_revision: revision, replayed: z.boolean(),
  result: ProjectUpdateRevisionSchema, author: HistoricalMemberSchema, published_at: timestamp,
}).strict();
export const ProjectDeleteImpactSchema = z.object({
  ...identity, project_revision: revision, issue_count: count, formal_issue_count: count,
  resource_count: count, update_count: count, autopilot_count: count,
  preserves_issues: z.literal(true), preserves_executions: z.literal(true),
}).strict();

/** New P1 endpoints never turn malformed protected content into empty success. */
export function parseProjectP1<T>(raw: unknown, schema: z.ZodType<T>, workspaceId: string, projectId?: string): T {
  const parsed = parseWithFallback<T | null>(raw, schema, null, { endpoint: "project-p1", redact: true });
  if (parsed === null) throw new Error("Invalid project response");
  function inspect(value: unknown): void {
    if (!value || typeof value !== "object") return;
    if (Array.isArray(value)) { value.forEach(inspect); return; }
    const record = value as Record<string, unknown>;
    if (("workspace_id" in record && record.workspace_id !== workspaceId) ||
      (projectId !== undefined && "project_id" in record && record.project_id !== projectId)) throw new Error("Project response identity mismatch");
    if (record.result && typeof record.result === "object") {
      const result = record.result as Record<string, unknown>;
      if (record.result_revision !== result.revision || record.update_id !== result.update_id) throw new Error("Project write result revision mismatch");
    }
    if (record.current && typeof record.current === "object") {
      const current = record.current as Record<string, unknown>;
      if (record.current_revision !== current.revision || record.id !== current.update_id) throw new Error("Project update revision mismatch");
    }
    Object.values(record).forEach(inspect);
  }
  inspect(parsed);
  return parsed;
}

export const ProjectExecutionEvidenceSchema = z.object({
  ...identity, update_id: uuid, revision,
  task: AgentTaskSchema.extend({ id: uuid, kind: z.enum(["comment", "autopilot", "chat", "quick_create", "direct"]).optional(), status: z.enum(["queued", "dispatched", "waiting_local_directory", "running", "completed", "failed", "cancelled"]) }),
  messages: z.array(z.object({ task_id: uuid, issue_id: z.string(), chat_session_id: z.string().optional(), seq: count,
    type: z.enum(["text", "thinking", "tool_use", "tool_result", "error"]), tool: z.string().optional(), content: z.string().optional(),
    input: z.record(z.string(), z.unknown()).optional(), output: z.string().optional(), created_at: z.string().optional() }).loose()),
}).strict().superRefine((value, ctx) => {
  if (value.messages.some((message) => message.task_id !== value.task.id)) ctx.addIssue({ code: "custom", message: "Execution message identity mismatch" });
});
