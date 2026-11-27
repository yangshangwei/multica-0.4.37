import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { MaterialOperationSchema, MaterialResponseError } from "./schema";

const id = z.string().min(1);
const version = z.number().int().nonnegative();
const sha = z.string().regex(/^[a-f0-9]{40}(?:[a-f0-9]{24})?$/i);
const digest = z.string().regex(/^[a-f0-9]{64}$/i);
const source = z.string().max(1 << 20);
const path = z.string().min(1).max(1024).refine((v) => !v.includes("\\") && !Array.from(v).some(char => char.charCodeAt(0) < 32) && v.split("/").every((part) => part !== "" && part !== "." && part !== ".." && part !== ".git"));
const reviewURL = z.string().url().refine((v) => { const u = new URL(v); return u.protocol === "https:" && !u.username && !u.password; });
export const InitializationFileSchema = z.object({
  path, action: z.string(), before_sha256: digest.nullable(), after_sha256: digest.nullable(),
  before_content: source.nullable(), after_content: source.nullable(), explanation: z.string().nullable(),
});
export const InitializationCandidateSchema = z.object({
  id, kind: z.string(), version, binding_generation: version, bundle_version: id, manifest_sha256: z.union([digest, z.literal("")]),
  base_commit: sha, head_sha: sha.nullable(), review_url: reviewURL.nullable(), publication_status: z.string(),
  files: z.array(InitializationFileSchema).max(1000),
}).superRefine((value, ctx) => {
  if (new Set(value.files.map(f => f.path)).size !== value.files.length || value.files.some(f =>
    (f.before_content !== null && f.before_sha256 === null) || (f.after_content !== null && f.after_sha256 === null) || (["add", "modify"].includes(f.action) && (f.after_content === null || f.after_sha256 === null)) || (f.action === "modify" && (f.before_content === null || f.before_sha256 === null)))) {
    ctx.addIssue({ code: "custom", message: "Incomplete initialization file manifest" });
  }
});
export const InitializationSchema = z.object({
  workspace_id: id, project_id: id, protocol_version: version, version, settings_version: version,
  binding_generation: version, status: z.string(), blocker: z.string().nullable(),
  readiness: z.object({ publish_allowed: z.boolean(), verified_commit: sha.nullable(), manifest_sha256: digest.nullable(),
    schema_version: version.nullable(), bundle_version: id.nullable(), verified_at: z.string().datetime({ offset: true }).nullable() }),
  history: z.array(z.object({ publish_allowed: z.boolean(), verified_commit: sha.nullable(), manifest_sha256: digest.nullable(), schema_version: version.nullable(), bundle_version: id.nullable(), verified_at: z.string().datetime({ offset: true }).nullable() })).nullish(),
  operation: MaterialOperationSchema.nullable(), candidate: InitializationCandidateSchema.nullable(),
  available_bundle_versions: z.array(id),
  decisions: z.array(z.object({ id, candidate_id: id, actor_id: id, decision: z.string(), reason: z.string(), head_sha: sha,
    manifest_sha256: digest, created_at: z.string().datetime({ offset: true }) })),
  capabilities: z.object({ can_check: z.boolean(), can_review: z.boolean(), can_retry: z.boolean(), can_maintain: z.boolean() }),
});
export type MaterialInitialization = z.infer<typeof InitializationSchema>;
export type InitializationCandidate = z.infer<typeof InitializationCandidateSchema>;
const command = { request_id: id, expected_version: version, expected_settings_version: version };
export const InitializationCommandSchema = z.discriminatedUnion("action", [
  z.object({ action: z.enum(["check", "retry"]), data: z.object(command).strict() }).strict(),
  z.object({ action: z.literal("maintenance"), data: z.object({ ...command, target_bundle_version: id }).strict() }).strict(),
  z.object({ action: z.literal("decisions"), data: z.object({ ...command, candidate_id: id, head_sha: sha, manifest_sha256: digest,
    decision: z.enum(["approve", "request_changes", "withdraw"]), reason: z.string() }).strict() }).strict(),
]);
export type MaterialInitializationCommand = z.infer<typeof InitializationCommandSchema>;
export const decodeMaterialInitializationCommand = (value: unknown): MaterialInitializationCommand | null => InitializationCommandSchema.safeParse(value).data ?? null;
export function parseMaterialInitialization(raw: unknown, workspaceId: string, projectId: string): MaterialInitialization {
  const result = parseWithFallback<MaterialInitialization | null>(raw, InitializationSchema, null, { endpoint: "material repository initialization", redact: true });
  if (!result || result.workspace_id.toLowerCase() !== workspaceId.toLowerCase() || result.project_id.toLowerCase() !== projectId.toLowerCase()) throw new MaterialResponseError();
  return result;
}
const states = new Set(["unconfigured", "queued", "checking", "needs_review", "publishing", "in_review", "changes_requested", "approved", "merging", "verifying", "ready", "blocked", "unknown"]);
export const supportsMaterialInitialization = (value: MaterialInitialization | undefined) => value?.protocol_version === 1 && states.has(value.status);
export function canPublishInitializedMaterial(value: MaterialInitialization | undefined, settingsVersion: number | undefined): boolean {
  if (!value || !supportsMaterialInitialization(value) || value.settings_version !== settingsVersion || value.binding_generation <= 0) return false;
  const proof = value.readiness;
  return proof.publish_allowed === true && proof.schema_version === 1 && !!proof.bundle_version && !!proof.verified_commit && !!proof.manifest_sha256 && !!proof.verified_at;
}
export function initializationReviewKey(value: MaterialInitialization): string | null {
  const c = value.candidate;
  return c?.head_sha ? JSON.stringify([c.id, c.version, c.binding_generation, c.head_sha, c.manifest_sha256, value.settings_version]) : null;
}
export function canReviewInitialization(value: MaterialInitialization): boolean {
  const c = value.candidate;
  return supportsMaterialInitialization(value) && value.capabilities.can_review === true && !!c &&
    ["initialize", "maintenance"].includes(c.kind) && c.binding_generation === value.binding_generation &&
    !!c.head_sha && /^[a-f0-9]{64}$/i.test(c.manifest_sha256) && !!c.review_url && ["in_review", "published", "approved", "changes_requested", "withdrawn"].includes(c.publication_status) && c.files.length > 0 &&
    c.files.every(f => ["add", "preserve", "modify"].includes(f.action) && (f.action === "preserve" || f.after_content !== null));
}
