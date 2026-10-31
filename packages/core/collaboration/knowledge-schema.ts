import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { collaborationTextByteLength } from "./text-limits";
import { nonnegativeVersion, positiveVersion } from "./version";

export const KNOWLEDGE_MAX_VISIBLE_MS = 30_000;
export const KNOWLEDGE_REFRESH_MS = 15_000;
const uuid = z.uuid().transform((value) => value.toLowerCase());
const timestamp = z.iso.datetime({ offset: true });
const utcTimestamp = timestamp.refine((value) => /(?:Z|[+-]00:00)$/.test(value));
const hash = z.string().regex(/^[a-f0-9]{64}$/);
const text = (max: number) => z.string().min(1).refine((value) =>
  value.trim() !== "" && !value.includes("\0") && value.isWellFormed() && collaborationTextByteLength(value) <= max);
const cursor = z.string().min(1).max(2048);

export const CollaborationKnowledgeTargetSchema = z.discriminatedUnion("kind", [
  z.strictObject({ kind: z.literal("workspace"), workspace_id: uuid }),
  z.strictObject({ kind: z.literal("project"), workspace_id: uuid, project_id: uuid }),
]);
export const CollaborationKnowledgeSourceSchema = z.discriminatedUnion("kind", [
  z.strictObject({ kind: z.literal("memory"), issue_id: uuid, version: positiveVersion, sha256: hash }),
  z.strictObject({ kind: z.literal("comment"), issue_id: uuid, comment_id: uuid, version: positiveVersion, sha256: hash }),
  z.strictObject({ kind: z.literal("run_record"), issue_id: uuid, run_id: uuid, record_id: text(256), sha256: hash }),
]);
const verification = z.enum(["unverified", "observed", "verified"]);
export const CollaborationKnowledgeEvidenceSchema = z.discriminatedUnion("kind", [
  z.strictObject({ kind: z.literal("run_record"), run_id: uuid, record_id: text(256), verification }),
  z.strictObject({ kind: z.literal("commit"), source: CollaborationKnowledgeSourceSchema, reference: text(2048), verification }),
  z.strictObject({ kind: z.literal("test"), source: CollaborationKnowledgeSourceSchema, reference: text(2048), verification }),
]);
export const CollaborationKnowledgeApplicabilitySchema = z.strictObject({
  surfaces: z.array(z.enum(["issue", "chat", "quick_create", "autopilot", "collaboration"])).min(1).max(5)
    .refine((values) => new Set(values).size === values.length),
});
const material = {
  summary: text(2048), sources: z.array(CollaborationKnowledgeSourceSchema).min(1).max(32),
  evidence: z.array(CollaborationKnowledgeEvidenceSchema).max(32), applicability: CollaborationKnowledgeApplicabilitySchema,
};
export const CollaborationKnowledgeProposalSchema = z.strictObject({
  protocol_version: z.literal(1), candidate_id: uuid, request_id: uuid,
  target: CollaborationKnowledgeTargetSchema, ...material, content: text(65_536),
  expires_at: utcTimestamp.nullable(), supersedes_candidate_id: uuid.nullable(), derived_from_candidate_id: uuid.nullable(),
}).refine((value) => {
  const metadata = { ...value, content: undefined, summary: undefined };
  return value.sources.length + value.evidence.length <= 32 && knowledgeMaterialBytes(metadata) <= 16_384;
}, "Knowledge proposal metadata exceeds its budget").refine((value) => value.evidence.every((item) =>
  value.sources.some((source) => item.kind === "run_record"
    ? source.kind === "run_record" && item.run_id === source.run_id && item.record_id === source.record_id
    : JSON.stringify(item.source) === JSON.stringify(source))), "Evidence anchors must also be required sources")
  .refine((value) => value.sources.every((source) => source.issue_id === value.sources[0]?.issue_id) &&
    !(value.supersedes_candidate_id && value.derived_from_candidate_id) && value.supersedes_candidate_id !== value.candidate_id && value.derived_from_candidate_id !== value.candidate_id);
export const CollaborationKnowledgeApproveRequestSchema = z.strictObject({
  protocol_version: z.literal(1), request_id: uuid, expected_state_revision: positiveVersion,
  expected_version: positiveVersion, expected_content_sha256: hash,
});
export const CollaborationKnowledgeDecisionRequestSchema = z.strictObject({
  protocol_version: z.literal(1), request_id: uuid, expected_state_revision: positiveVersion, reason: text(1000),
});
export const CollaborationKnowledgeStateSchema = z.enum(["pending", "approved", "rejected", "withdrawn", "superseded"]);
const action = z.enum(["propose", "approve", "reject", "withdraw"]);
export const CollaborationKnowledgeReceiptSchema = z.strictObject({
  protocol_version: z.literal(1), request_id: uuid, action, code: z.string().min(1).max(128),
  candidate_id: uuid, version: positiveVersion, state_revision: positiveVersion, state: CollaborationKnowledgeStateSchema,
  target_knowledge_revision: nonnegativeVersion.optional(),
});

// An absent affordance is unknown, never a grant. Malformed claimed booleans fail parsing.
const permission = z.boolean().optional().transform((value) => value === true);
const capabilities = z.object({ can_approve: permission, can_reject: permission, can_withdraw: permission })
  .prefault({});
const control = { candidate_id: uuid, state: CollaborationKnowledgeStateSchema, state_revision: positiveVersion };
const metadata = {
  ...control, lineage_id: uuid, version: positiveVersion, target: CollaborationKnowledgeTargetSchema,
  source_workspace_id: uuid, source_issue_id: uuid, proposed_by_user_id: uuid, proposed_at: timestamp,
  expires_at: timestamp.nullable(), supersedes_candidate_id: uuid.nullable(), derived_from_candidate_id: uuid.nullable(),
  approved_by_user_id: uuid.nullable(), approved_at: timestamp.nullable(), published_target_revision: nonnegativeVersion.nullable(),
  decision: z.strictObject({ actor_user_id: uuid, at: timestamp, reason: text(1000).nullable() }).nullable(),
};
const unavailable = z.strictObject({ ...metadata, availability: z.literal("source_unavailable"), capabilities,
  decision: z.strictObject({ actor_user_id: uuid, at: timestamp }).nullable(),
})
  .refine((value) => !value.capabilities.can_approve);
const restricted = z.strictObject({ ...control, availability: z.literal("restricted"),
  capabilities: z.strictObject({ can_withdraw: permission }).prefault({}),
});
const available = z.strictObject({ ...metadata, availability: z.literal("available"), ...material, content_sha256: hash, capabilities })
  .refine((value) => value.sources.every((source) => source.issue_id === value.source_issue_id));
export const CollaborationKnowledgeCandidateSummarySchema = z.union([available, unavailable, restricted]);
export const CollaborationKnowledgeCandidateSchema = z.union([available.safeExtend({ content: text(65_536) }), unavailable, restricted]);
export const CollaborationKnowledgeProjectionSchema = z.object({
  protocol_version: z.literal(1), projection_checked_at: timestamp, valid_until: timestamp, next_expiry_at: timestamp.nullable(),
}).refine((value) => Date.parse(value.valid_until) > Date.parse(value.projection_checked_at));
export const CollaborationKnowledgeCandidatesSchema = CollaborationKnowledgeProjectionSchema.safeExtend({
  issue_id: uuid, items: z.array(CollaborationKnowledgeCandidateSummarySchema).max(50), next_cursor: cursor.nullable(),
  capabilities: z.object({ can_propose: permission }).prefault({}),
}).refine((value) => new Set(value.items.map((item) => item.candidate_id)).size === value.items.length &&
  value.items.every((item) => item.availability !== "restricted" && item.source_issue_id === value.issue_id));
export const CollaborationKnowledgeDetailSchema = CollaborationKnowledgeProjectionSchema.safeExtend({
  candidate: CollaborationKnowledgeCandidateSchema, history: z.array(CollaborationKnowledgeCandidateSummarySchema).max(50),
  history_truncated: z.boolean(),
}).refine((value) => value.candidate.availability === "restricted" ? value.history.length === 0 :
  new Set(value.history.map((item) => item.candidate_id)).size === value.history.length &&
  value.history.every((item) => item.availability !== "restricted" && value.candidate.availability !== "restricted" &&
    item.lineage_id === value.candidate.lineage_id && item.source_issue_id === value.candidate.source_issue_id && knowledgeTargetsEqual(item.target, value.candidate.target)));
export const CollaborationKnowledgeTargetListSchema = CollaborationKnowledgeProjectionSchema.safeExtend({
  target: CollaborationKnowledgeTargetSchema, knowledge_revision: nonnegativeVersion,
  items: z.array(CollaborationKnowledgeCandidateSummarySchema).max(50), next_cursor: cursor.nullable(),
}).refine((value) => new Set(value.items.map((item) => item.candidate_id)).size === value.items.length &&
  value.items.every((item) => item.availability === "restricted" || knowledgeTargetsEqual(item.target, value.target)));
export const CollaborationKnowledgeHintSchema = z.strictObject({
  target_kind: z.enum(["workspace", "project"]), target_id: uuid, knowledge_revision: nonnegativeVersion,
});

export type CollaborationKnowledgeTarget = z.infer<typeof CollaborationKnowledgeTargetSchema>;
export type CollaborationKnowledgeSource = z.infer<typeof CollaborationKnowledgeSourceSchema>;
export type CollaborationKnowledgeEvidence = z.infer<typeof CollaborationKnowledgeEvidenceSchema>;
export type CollaborationKnowledgeProposal = z.infer<typeof CollaborationKnowledgeProposalSchema>;
export type CollaborationKnowledgeApproveRequest = z.infer<typeof CollaborationKnowledgeApproveRequestSchema>;
export type CollaborationKnowledgeDecisionRequest = z.infer<typeof CollaborationKnowledgeDecisionRequestSchema>;
export type CollaborationKnowledgeReceipt = z.infer<typeof CollaborationKnowledgeReceiptSchema>;
export type CollaborationKnowledgeCandidate = z.infer<typeof CollaborationKnowledgeCandidateSchema>;
export type CollaborationKnowledgeCandidateSummary = z.infer<typeof CollaborationKnowledgeCandidateSummarySchema>;
export type CollaborationKnowledgeProjection = z.infer<typeof CollaborationKnowledgeProjectionSchema>;
export type CollaborationKnowledgeCandidates = z.infer<typeof CollaborationKnowledgeCandidatesSchema>;
export type CollaborationKnowledgeDetail = z.infer<typeof CollaborationKnowledgeDetailSchema>;
export type CollaborationKnowledgeTargetList = z.infer<typeof CollaborationKnowledgeTargetListSchema>;
export type CollaborationKnowledgeHint = z.infer<typeof CollaborationKnowledgeHintSchema>;
export interface CollaborationKnowledgePageParams { cursor?: string; limit?: number }

const successOutcomes = { propose: ["knowledge_proposed", "pending"], approve: ["knowledge_approved", "approved"],
  reject: ["knowledge_rejected", "rejected"], withdraw: ["knowledge_withdrawn", "withdrawn"] } as const;
const refusalStatuses: Record<string, number> = {
  invalid_knowledge_request: 400, knowledge_candidate_too_large: 413, knowledge_candidate_not_found: 404,
  knowledge_target_not_found: 404, knowledge_permission_denied: 403, knowledge_revision_required: 428,
  knowledge_revision_conflict: 409, knowledge_content_changed: 409, knowledge_state_conflict: 409,
  knowledge_source_unavailable: 409, knowledge_expired: 409, knowledge_target_budget_exceeded: 409, idempotency_conflict: 409,
};
export function knowledgeReceiptRefusalStatus(code: string): number | undefined {
  return Object.hasOwn(refusalStatuses, code) ? refusalStatuses[code] : undefined;
}
export function knowledgeReceiptConfirmed(receipt: CollaborationKnowledgeReceipt): boolean {
  const [code, state] = successOutcomes[receipt.action];
  return receipt.code === code ? receipt.state === state : knowledgeReceiptRefusalStatus(receipt.code) !== undefined;
}

/** Match the Go material budget: decoded UTF-8 strings, JSON framing and a final newline. */
function knowledgeMaterialBytes(value: unknown): number {
  let size = collaborationTextByteLength(JSON.stringify(value)) + 1;
  const visit = (item: unknown): void => {
    if (typeof item === "string") size -= collaborationTextByteLength(JSON.stringify(item)) - 2 - collaborationTextByteLength(item);
    else if (Array.isArray(item)) item.forEach(visit);
    else if (item !== null && typeof item === "object") for (const [key, child] of Object.entries(item)) {
      if (child !== undefined) { visit(key); visit(child); }
    }
  };
  visit(value);
  return size;
}

export function knowledgeTargetsEqual(a: CollaborationKnowledgeTarget, b: CollaborationKnowledgeTarget): boolean {
  return a.kind === b.kind && a.workspace_id === b.workspace_id &&
    (a.kind !== "project" || b.kind === "project" && a.project_id === b.project_id);
}
export class CollaborationKnowledgeProtocolError extends Error {
  constructor() { super("Knowledge response does not match its declared protocol"); this.name = "CollaborationKnowledgeProtocolError"; }
}
/** null means an older response with no knowledge protocol, never a malformed v1. */
function parse<T>(data: unknown, schema: z.ZodType<T>): T | null {
  if (data === undefined || data === null || typeof data === "object" && !Object.hasOwn(data, "protocol_version")) return null;
  const result = parseWithFallback<T | null>(data, schema, null, { endpoint: "collaboration knowledge", redact: true });
  if (result === null) throw new CollaborationKnowledgeProtocolError();
  return result;
}
export const parseCollaborationKnowledgeCandidates = (data: unknown) => parse(data, CollaborationKnowledgeCandidatesSchema);
export const parseCollaborationKnowledgeDetail = (data: unknown) => parse(data, CollaborationKnowledgeDetailSchema);
export const parseCollaborationKnowledgeTargetList = (data: unknown) => parse(data, CollaborationKnowledgeTargetListSchema);
export const parseCollaborationKnowledgeReceipt = (data: unknown) => parse(data, CollaborationKnowledgeReceiptSchema);

export function knowledgePageQuery(params: CollaborationKnowledgePageParams): string {
  const parsed = z.strictObject({ limit: z.number().int().min(1).max(50).default(20), cursor: cursor.optional() }).parse(params);
  const query = new URLSearchParams({ limit: String(parsed.limit) });
  if (parsed.cursor !== undefined) query.set("cursor", parsed.cursor);
  return query.toString();
}
