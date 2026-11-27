import { z } from "zod";
import { parseWithFallback } from "../api/schema";

const id = z.string().min(1);
const version = z.number().int().nonnegative();
const sha = z.string().regex(/^[a-f0-9]{40}(?:[a-f0-9]{24})?$/i);
const digest = z.string().regex(/^[a-f0-9]{64}$/i);
const webURL = z.string().url().refine((value) => {
  const url = new URL(value);
  return ["https:", "http:"].includes(url.protocol) && !url.username && !url.password;
});

export const MaterialSourceSchema = z.object({ resource_id: id, commit: sha, path: z.string() });
export const MaterialSettingsSchema = z.object({
  version, enabled: z.boolean(), provider: z.string(), resource_id: id,
  execution_runtime_id: z.string().uuid().nullable().default(null), repository: id, base_branch: id, root_path: z.string(),
  reviewer_ids: z.array(id), auto_start_plan: z.boolean(),
});
export const MaterialSettingsEnvelopeSchema = z.object({
  workspace_id: id, project_id: id, can_configure: z.boolean(), settings: MaterialSettingsSchema.nullable(),
  execution_protocol: z.number().int().nonnegative().optional(),
});
export const MaterialSchema = z.object({
  id, workspace_id: id, project_id: id, title: z.string(), path: z.string(), version,
  status: z.string(), blocker: z.string().nullable(), created_at: id, updated_at: id,
});
export const MaterialRevisionSchema = z.object({
  id, kind: z.string(), content: z.string(), plan_content: z.string().nullable(),
  content_sha256: digest, sources: z.array(MaterialSourceSchema),
  acceptance_criteria: z.array(z.string()), blocking_questions: z.array(z.string()),
  source_baseline_id: id.nullable(), publication_status: z.string(), review_url: webURL.nullable(),
  head_sha: sha.nullable(), created_at: id,
});
export const MaterialDecisionSchema = z.object({
  id, revision_id: id, actor_id: id, decision: z.string(), reason: z.string(), head_sha: sha, created_at: id,
});
export const MaterialCommentSchema = z.object({ id, revision_id: id, actor_id: id, body: z.string(), created_at: id });
export const MaterialBaselineSchema = z.object({
  id, revision_id: id, commit: sha, path: id, content_sha256: digest, review_url: webURL, verified_at: id,
});
export const MaterialPlanSchema = z.object({
  issue_id: id.nullable(), status: z.string(), blocker: z.string().nullable(), baseline_id: id, revision_id: id.nullable(),
});
export const MaterialListSchema = z.object({ workspace_id: id, project_id: id, materials: z.array(MaterialSchema) });
export const MaterialOperationSchema = z.object({
  id, kind: z.string(), status: z.string(), task_id: id.nullable(),
  execution_runtime_id: z.string().uuid().nullable(), blocker: z.string().nullable(),
});
export const MaterialDetailSchema = z.object({
  operation: MaterialOperationSchema.nullable().optional(),
  material: MaterialSchema, revisions: z.array(MaterialRevisionSchema), decisions: z.array(MaterialDecisionSchema),
  comments: z.array(MaterialCommentSchema), baseline: MaterialBaselineSchema.nullable(), plan: MaterialPlanSchema.nullable(),
  capabilities: z.object({ can_edit: z.boolean(), can_review: z.boolean(), can_start_plan: z.boolean(), can_retry: z.boolean() }),
}).superRefine((value, ctx) => {
  const revisions = new Map(value.revisions.map((revision) => [revision.id, revision]));
  if (revisions.size !== value.revisions.length ||
    value.decisions.some((decision) => !revisions.has(decision.revision_id)) ||
    value.comments.some((comment) => !revisions.has(comment.revision_id)) ||
    value.baseline && (!revisions.has(value.baseline.revision_id) ||
      revisions.get(value.baseline.revision_id)?.content_sha256 !== value.baseline.content_sha256)) {
    ctx.addIssue({ code: "custom", message: "Inconsistent material revision references" });
  }
});

export type MaterialSource = z.infer<typeof MaterialSourceSchema>;
export type MaterialSettings = z.infer<typeof MaterialSettingsSchema>;
export type MaterialSettingsEnvelope = z.infer<typeof MaterialSettingsEnvelopeSchema>;
export type Material = z.infer<typeof MaterialSchema>;
export type MaterialRevision = z.infer<typeof MaterialRevisionSchema>;
export type MaterialDetail = z.infer<typeof MaterialDetailSchema>;
export type MaterialList = z.infer<typeof MaterialListSchema>;
export type MaterialKind = "requirement" | "plan";
export type MaterialSettingsInput = Omit<MaterialSettings, "version" | "provider"> & {
  request_id: string; expected_version: number; provider: "github" | "gitlab";
};
export interface MaterialRevisionInput {
  request_id: string; expected_version: number; kind: MaterialKind; content: string; plan_content?: string;
  sources: MaterialSource[]; acceptance_criteria: string[]; blocking_questions: string[]; ready: boolean; source_baseline_id?: string;
}
export interface MaterialDecisionInput {
  request_id: string; expected_version: number; revision_id: string; head_sha: string;
  decision: "approve" | "request_changes" | "withdraw"; reason: string;
}
export type MaterialCommand =
  | { action: "revisions"; data: MaterialRevisionInput }
  | { action: "decisions"; data: MaterialDecisionInput }
  | { action: "comments"; data: { request_id: string; revision_id: string; body: string } }
  | { action: "retry" | "start-plan"; data: { request_id: string; expected_version: number } };

const requestID = z.string().min(1);
const expectedVersion = { request_id: requestID, expected_version: version };
export const MaterialCreateInputSchema = z.object({ request_id: requestID, title: z.string().min(1) });
export const MaterialSettingsInputSchema = MaterialSettingsSchema.omit({ version: true }).extend({
  ...expectedVersion, provider: z.enum(["github", "gitlab"]),
  execution_runtime_id: z.string().uuid().nullable(),
}).strict().refine((value) => !value.enabled || value.execution_runtime_id !== null, {
  message: "Enabled materials require an execution runtime",
});
export const MaterialRevisionInputSchema = z.object({
  ...expectedVersion, kind: z.enum(["requirement", "plan"]), content: z.string(), plan_content: z.string().optional(),
  sources: z.array(MaterialSourceSchema), acceptance_criteria: z.array(z.string()), blocking_questions: z.array(z.string()),
  ready: z.boolean(), source_baseline_id: id.optional(),
});
export const MaterialCommandSchema = z.discriminatedUnion("action", [
  z.object({ action: z.literal("revisions"), data: MaterialRevisionInputSchema }),
  z.object({ action: z.literal("decisions"), data: z.object({ ...expectedVersion, revision_id: id, head_sha: sha, decision: z.enum(["approve", "request_changes", "withdraw"]), reason: z.string() }) }),
  z.object({ action: z.literal("comments"), data: z.object({ request_id: requestID, revision_id: id, body: z.string() }) }),
  z.object({ action: z.enum(["retry", "start-plan"]), data: z.object(expectedVersion) }),
]);
export const decodeMaterialCreate = (value: unknown) => MaterialCreateInputSchema.safeParse(value).data ?? null;
export const decodeMaterialSettingsInput = (value: unknown) => MaterialSettingsInputSchema.safeParse(value).data ?? null;
export const decodeMaterialCommand = (value: unknown): MaterialCommand | null => MaterialCommandSchema.safeParse(value).data ?? null;

export class MaterialResponseError extends Error {
  constructor() { super("Material response is unavailable"); this.name = "MaterialResponseError"; }
}

const sameID = (a: string, b: string) => a.toLowerCase() === b.toLowerCase();
function parse<T>(raw: unknown, schema: z.ZodType, endpoint: string): T {
  const result = parseWithFallback<T | null>(raw, schema, null, { endpoint, redact: true });
  if (!result) throw new MaterialResponseError();
  return result;
}
export function parseMaterialSettings(raw: unknown, workspaceId: string, projectId: string): MaterialSettingsEnvelope {
  const result = parse<MaterialSettingsEnvelope>(raw, MaterialSettingsEnvelopeSchema, "project material settings");
  if (!sameID(result.workspace_id, workspaceId) || !sameID(result.project_id, projectId)) throw new MaterialResponseError();
  return result;
}
export function parseMaterialList(raw: unknown, workspaceId: string, projectId: string): MaterialList {
  const result = parse<MaterialList>(raw, MaterialListSchema, "project material list");
  if (!sameID(result.workspace_id, workspaceId) || !sameID(result.project_id, projectId) || result.materials.some((material) =>
    !sameID(material.workspace_id, workspaceId) || !sameID(material.project_id, projectId))) throw new MaterialResponseError();
  return result;
}
export function parseMaterialDetail(raw: unknown, workspaceId: string, projectId: string, materialId?: string): MaterialDetail {
  const result = parse<MaterialDetail>(raw, MaterialDetailSchema, "project material detail");
  if (!sameID(result.material.workspace_id, workspaceId) || !sameID(result.material.project_id, projectId) ||
    materialId !== undefined && !sameID(result.material.id, materialId)) throw new MaterialResponseError();
  return result;
}

export const MATERIAL_STATES = ["draft", "publishing", "in_review", "changes_requested", "approved", "merging", "blocked", "baseline_ready", "planning", "plan_review", "plan_approved"] as const;
export function isKnownMaterialState(status: string): status is typeof MATERIAL_STATES[number] {
  return MATERIAL_STATES.some((state) => state === status);
}
export function newestMaterialRevision(detail: MaterialDetail, kind: MaterialKind): MaterialRevision | undefined {
  // ListMaterialRevisions returns SQL history order (created_at, id).
  return detail.revisions.findLast((revision) => revision.kind === kind);
}
export function canReviewMaterialRevision(detail: MaterialDetail, revision: MaterialRevision): boolean {
  const current = detail.revisions.at(-1);
  return detail.capabilities.can_review === true && isKnownMaterialState(detail.material.status) &&
    (revision.kind === "requirement" || revision.kind === "plan") &&
    current?.id === revision.id && detail.material.status !== "changes_requested" &&
    revision.head_sha !== null && revision.review_url !== null &&
    ["in_review", "published", "approved"].includes(revision.publication_status) &&
    (revision.kind !== "plan" || revision.source_baseline_id === detail.baseline?.id);
}
