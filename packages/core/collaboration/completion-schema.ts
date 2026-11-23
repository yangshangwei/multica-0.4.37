import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationCompletionTargetSchema, CompletionAuthorizationBindingSchema, CompletionCandidatesSchema, completionPreviewFields,
  completionRevision, completionUUID, type CollaborationCompletionTarget, type CompletionAuthorization, type CompletionBindingSchema } from "../issues/completion-schema";
import { positiveVersion } from "./version";
import { CollaborationStageBlockerSchema } from "./stage-schema";
import type { CollaborationIssue } from "./schema";
import type { CollaborationActionIntent } from "./operation";

const sourceFields = { source_issue_id: completionUUID, source_issue_revision: completionRevision };
export const CollaborationCompletionCandidatesSchema = CompletionCandidatesSchema.safeExtend(sourceFields)
  .refine((v) => v.issue_id !== v.source_issue_id);
export const AdminCompletionTargetContextSchema = z.strictObject({ expected_lifecycle_epoch: positiveVersion, submission_id: completionUUID.nullable() });
export const AdminCollaborationCompletionCandidatesSchema = CollaborationCompletionCandidatesSchema.safeExtend({ target_context: AdminCompletionTargetContextSchema });
export type CollaborationCompletionCandidates = z.infer<typeof CollaborationCompletionCandidatesSchema> & { target_context?: z.infer<typeof AdminCompletionTargetContextSchema> };
export const CollaborationCompletionPreviewInputSchema = z.strictObject({ source_issue_id: completionUUID, approval_id: completionUUID, target: CollaborationCompletionTargetSchema })
  .refine((v) => v.source_issue_id !== v.target.issue_id);
export type CollaborationCompletionPreviewInput = z.infer<typeof CollaborationCompletionPreviewInputSchema>;
export const CollaborationCompletionPreviewSchema = z.object({ ...completionPreviewFields, ...sourceFields,
  authorization_binding: CompletionAuthorizationBindingSchema.nullable(), stage_blockers: z.array(CollaborationStageBlockerSchema).max(32), stage_blockers_truncated: z.boolean(),
}).refine((v) => {
  if (v.issue_id === v.source_issue_id) return false;
  if (v.observation && (v.observation.issue_id !== v.source_issue_id || v.observation.issue_revision !== v.source_issue_revision)) return false;
  if (v.reason === "stage_incomplete") return v.stage_blockers.length > 0 && v.authorization_binding === null && v.max_expires_at === null;
  if (v.stage_blockers.length !== 0 || v.stage_blockers_truncated) return false;
  const b = v.authorization_binding, o = v.observation;
  if (!b) return v.max_expires_at === null;
  return b.target.kind !== "ordinary" && v.policy_mode === "enforce" && v.target_category === "done" && v.current_category !== "done" &&
    v.reason === "authorization_available" && v.max_expires_at !== null && o?.verdict === "pass" && o.valid_until !== null &&
    b.target.issue_id === v.issue_id && b.target.expected_issue_revision === v.issue_revision && b.target.expected_status === v.current_status && b.target.target_status === v.target_status &&
    b.project.id === v.project_id && b.project.expected_revision === v.project_revision && b.project.expected_policy_revision === v.policy_revision &&
    b.evidence.issue_id === v.source_issue_id && b.evidence.expected_issue_revision === v.source_issue_revision && o.project_id === v.project_id && o.project_revision === v.project_revision &&
    o.approval_id === b.evidence.approval_id && o.source_revision === b.evidence.expected_source_revision && o.spec_digest === b.evidence.expected_spec_digest && o.artifact_digest === b.evidence.expected_artifact_digest &&
    Date.parse(v.max_expires_at) > Date.parse(v.checked_at) && Date.parse(v.max_expires_at) <= Date.parse(o.valid_until);
});
export type CollaborationCompletionPreview = z.infer<typeof CollaborationCompletionPreviewSchema>;
export type CollaborationCompletionRead<T> = { state: "available"; value: T } | { state: "denied" } | { state: "unavailable"; code: string | null };
export const parseCollaborationCompletionPreview = (value: unknown) => parseWithFallback<CollaborationCompletionPreview | null>(value, CollaborationCompletionPreviewSchema, null, { endpoint: "collaboration completion preview", redact: true });

export function collaborationCompletionTarget(issue: CollaborationIssue, intent: Extract<CollaborationActionIntent, { action: "complete" }>, revision: number): CollaborationCompletionTarget {
  return CollaborationCompletionTargetSchema.parse({ issue_id: issue.id, expected_issue_revision: revision, expected_status: issue.status, target_status: intent.status,
    expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context?.participant_epoch, expected_account_epoch: issue.write_context?.account_epoch, status_options_version: intent.statusOptionsVersion,
    ...(intent.submissionId ? { kind: "collaboration_submission", submission_id: intent.submissionId, acceptance_note: intent.note ?? "" }
      : { kind: "collaboration_self", result: intent.note, basis: intent.basis, draft_id: intent.draftId ?? null, attachment_ids: intent.attachmentIds ?? [], source_comment_id: intent.sourceCommentId ?? null }) });
}

/** The ordinary writer receipt is not substituted for the original collaboration/admin operation. */
export function matchesCollaborationCompletionConsumption(grant: CompletionAuthorization, binding: z.infer<typeof CompletionBindingSchema>, issueId: string,
  targetStatus: string, before: string, submission: string | "self" | "admin"): boolean {
  const r = grant.completion_receipt;
  return grant.state === "consumed" && grant.authorization_id === binding.authorization_id && grant.operation_id === binding.operation_id &&
    grant.issue_id === issueId && grant.issue_revision === binding.expected_issue_revision && grant.target_status === targetStatus && !!r &&
    r.authorization_id === binding.authorization_id && r.operation_id === binding.operation_id && r.issue_id === issueId && r.target_status === targetStatus &&
    r.completed_revision > binding.expected_issue_revision && r.collaboration_revision !== undefined && BigInt(r.collaboration_revision) === BigInt(before) + 1n &&
    (submission === "admin" ? r.submission_id === undefined : submission === "self" ? r.submission_id !== undefined : r.submission_id === submission);
}
