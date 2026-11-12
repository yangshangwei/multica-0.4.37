import { z } from "zod";
import type { Issue } from "../types";

export const completionUUID = z.uuid().refine((value) => value === value.toLowerCase() && value !== "00000000-0000-0000-0000-000000000000");
export const completionRevision = z.number().int().nonnegative().safe();
const digest = z.string().regex(/^[a-f0-9]{64}$/);
const sourceRevision = z.string().regex(/^(?:[a-f0-9]{40}|[a-f0-9]{64})$/);
const timestamp = z.iso.datetime().refine((value) => /(?:\.\d{1,6})?Z$/.test(value));
const status = z.string().min(1).max(128);
const category = z.enum(["backlog", "todo", "in_progress", "in_review", "done", "blocked", "cancelled"]);
export const completionReason = z.string().trim().min(1).refine((value) => [...value].length <= 1000 && !value.includes("\0"));

export const CompletionBindingSchema = z.strictObject({
  authorization_id: completionUUID, operation_id: completionUUID, expected_issue_revision: completionRevision,
});
const ordinaryTarget = z.strictObject({
  kind: z.literal("ordinary"), issue_id: completionUUID, expected_issue_revision: completionRevision,
  expected_status: status, target_status: status, target_position: z.null(),
});
const project = z.strictObject({ id: completionUUID, expected_revision: completionRevision, expected_policy_revision: completionRevision.min(1) });
const evidence = z.strictObject({
  issue_id: completionUUID, expected_issue_revision: completionRevision, approval_id: completionUUID,
  expected_source_revision: sourceRevision, expected_spec_digest: digest, expected_artifact_digest: digest,
});
const authorizationFields = {
  contract_version: z.literal("delivery-completion-enforcement-v1"), source_workspace_id: completionUUID,
  target: ordinaryTarget, project, evidence, consumer: z.strictObject({ kind: z.literal("human"), user_id: completionUUID }),
};
export const CompletionAuthorizationBindingSchema = z.strictObject(authorizationFields).refine((v) =>
  v.target.issue_id === v.evidence.issue_id && v.target.expected_issue_revision === v.evidence.expected_issue_revision);
export const CompletionAuthorizationRequestSchema = z.strictObject({
  ...authorizationFields, operation_id: completionUUID, expires_at: timestamp, reason: completionReason,
}).refine((v) => v.target.issue_id === v.evidence.issue_id && v.target.expected_issue_revision === v.evidence.expected_issue_revision);
export const CompletionPreviewInputSchema = z.strictObject({ expected_issue_revision: completionRevision, target_status: status, approval_id: completionUUID });

export const CompletionObservationSchema = z.object({
  mode: z.literal("observe"), policy_version: z.literal("delivery-artifact-observe-v1"), verdict: z.enum(["pass", "hold", "unknown"]), reason: z.string().min(1),
  issue_id: completionUUID, issue_revision: completionRevision, project_id: completionUUID.nullable(), project_revision: completionRevision.nullable(),
  approval_id: completionUUID, execution_id: completionUUID.optional(), source_revision: z.string(), observed_head_sha: z.string(),
  revision_provenance: z.literal("database_observed_pr_head_not_remote_verified"), spec_digest: digest.optional(), artifact_digest: digest.optional(), receipt_digest: digest.optional(),
  producer_agent_id: completionUUID.optional(), producer_task_id: completionUUID.optional(), producer_task_generation: completionRevision,
  observed_at: timestamp, valid_until: timestamp.nullable(),
});
export const CompletionCandidateSchema = z.object({
  approval_id: completionUUID, execution_id: completionUUID, source_revision: sourceRevision, spec_digest: digest,
  artifact_digest: digest.nullable(), receipt_digest: digest.nullable(), created_at: timestamp, expires_at: timestamp,
  summary: z.string(), status: z.string().min(1), result_code: z.string().nullable(), selectable: z.boolean(),
}).refine((v) => !v.selectable || (v.artifact_digest !== null && v.receipt_digest !== null));
export const CompletionCandidatesSchema = z.object({
  protocol_version: z.literal(1), issue_id: completionUUID, issue_revision: completionRevision, project_id: completionUUID.nullable(), checked_at: timestamp,
  items: z.array(CompletionCandidateSchema).max(50), next_cursor: z.string().min(1).nullable(), has_more: z.boolean(),
}).refine((v) => v.has_more === (v.next_cursor !== null) && new Set(v.items.map((item) => item.approval_id)).size === v.items.length);

export const CompletionPreviewSchema = z.object({
  protocol_version: z.literal(1), issue_id: completionUUID, issue_revision: completionRevision,
  current_status: status, current_category: category, target_status: status, target_category: category,
  project_id: completionUUID.nullable(), project_revision: completionRevision.nullable(), policy_mode: z.enum(["off", "observe", "enforce"]), policy_revision: completionRevision,
  checked_at: timestamp, reason: z.string().min(1), observation: CompletionObservationSchema.nullable(),
  authorization_binding: CompletionAuthorizationBindingSchema.nullable(), max_expires_at: timestamp.nullable(),
  enforced: z.literal(false), usable_for_completion: z.literal(false), skip_supported: z.literal(false),
}).refine((v) => {
  const b = v.authorization_binding;
  if (!b) return v.max_expires_at === null;
  const o = v.observation;
  return v.policy_mode === "enforce" && v.target_category === "done" && v.current_category !== "done" &&
    v.reason === "authorization_available" && v.max_expires_at !== null && o?.verdict === "pass" && o.valid_until !== null &&
    b.target.issue_id === v.issue_id && b.target.expected_issue_revision === v.issue_revision &&
    b.target.expected_status === v.current_status && b.target.target_status === v.target_status &&
    b.project.id === v.project_id && b.project.expected_revision === v.project_revision && b.project.expected_policy_revision === v.policy_revision &&
    o.issue_id === v.issue_id && o.issue_revision === v.issue_revision && o.project_id === v.project_id && o.project_revision === v.project_revision &&
    o.approval_id === b.evidence.approval_id && o.source_revision === b.evidence.expected_source_revision &&
    o.spec_digest === b.evidence.expected_spec_digest && o.artifact_digest === b.evidence.expected_artifact_digest &&
    Date.parse(v.max_expires_at) > Date.parse(v.checked_at) && Date.parse(v.max_expires_at) <= Date.parse(o.valid_until);
});

export const CompletionReceiptSchema = z.object({
  authorization_id: completionUUID, operation_id: completionUUID, issue_id: completionUUID, completed_revision: completionRevision,
  target_status: status, completed_at: timestamp, replayed: z.boolean(),
});
export const CompletionAuthorizationSchema = z.object({
  authorization_id: completionUUID, operation_id: completionUUID, issue_id: completionUUID, issue_revision: completionRevision, target_status: status,
  contract_version: z.literal("delivery-completion-enforcement-v1"), created_at: timestamp, expires_at: timestamp,
  state: z.enum(["issued", "consumed", "revoked"]), applicability: z.enum(["not_checked", "expired", "consumed", "revoked"]), replayed: z.boolean(),
  completion_receipt: CompletionReceiptSchema.optional(),
}).refine((v) => {
  if (v.state === "issued") return ["not_checked", "expired"].includes(v.applicability) && v.completion_receipt === undefined;
  if (v.state === "revoked") return v.applicability === "revoked" && v.completion_receipt === undefined;
  const r = v.completion_receipt;
  return v.applicability === "consumed" && !!r && r.authorization_id === v.authorization_id && r.operation_id === v.operation_id &&
    r.issue_id === v.issue_id && r.target_status === v.target_status && r.completed_revision > v.issue_revision;
});
export type CompletionPreviewInput = z.infer<typeof CompletionPreviewInputSchema>;
export type CompletionPreview = z.infer<typeof CompletionPreviewSchema>;
export type CompletionCandidates = z.infer<typeof CompletionCandidatesSchema>;
export type CompletionCandidate = z.infer<typeof CompletionCandidateSchema>;
export type CompletionAuthorizationRequest = z.infer<typeof CompletionAuthorizationRequestSchema>;
export type CompletionAuthorization = z.infer<typeof CompletionAuthorizationSchema>;
export type CompletionReceipt = z.infer<typeof CompletionReceiptSchema>;
export type CompletionWriteResult = { issue: Issue; receipt: CompletionReceipt };

export function matchesCompletionReceipt(receipt: CompletionReceipt, issueId: string, targetStatus: string, binding: z.infer<typeof CompletionBindingSchema>): boolean {
  return receipt.authorization_id === binding.authorization_id && receipt.operation_id === binding.operation_id && receipt.issue_id === issueId &&
    receipt.target_status === targetStatus && receipt.completed_revision > binding.expected_issue_revision;
}

export function matchesCompletionAuthorization(value: CompletionAuthorization, request: CompletionAuthorizationRequest): boolean {
  return value.operation_id === request.operation_id && value.issue_id === request.target.issue_id &&
    value.issue_revision === request.target.expected_issue_revision && value.target_status === request.target.target_status &&
    value.contract_version === request.contract_version && Date.parse(value.expires_at) === Date.parse(request.expires_at);
}
