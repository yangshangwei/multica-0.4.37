import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { CollaborationAccountSummarySchema, CollaborationStatusOptionsSchema } from "../collaboration/schema";
import { nonnegativeVersion, positiveVersion } from "../collaboration/version";
import { collaborationTextByteLength } from "../collaboration/text-limits";
import { CollaborationMemoryVersionMetaSchema, compareCollaborationMemoryVersions } from "../collaboration/memory-schema";
import { adminOperationSchema } from "./operation-schema";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const lifecycleAction = z.enum(["complete", "cancel", "reopen"]);
const statusKey = z.string().min(1).refine((value) => !value.includes("\0") && collaborationTextByteLength(value) <= 128);
const category = z.enum(["backlog", "todo", "in_progress", "in_review", "blocked", "done", "cancelled"]);
const account = z.object({ user_id: uuid, name: z.string(), avatar_url: z.string(), disabled: z.boolean() })
  .transform((value) => ({ userId: value.user_id, name: value.name, avatarUrl: value.avatar_url, disabled: value.disabled }));
const issueFields = {
  issue_id: uuid, identifier: z.string().min(1), source_workspace_id: uuid, source_name: z.string(),
  status: z.string().min(1), status_category: category, collaboration_revision: positiveVersion,
  creator: account, handler: account, queue_state: z.enum(["active", "paused", "closed"]), unsettled_runs: z.number().int().nonnegative().safe(),
};
const issueWire = z.object(issueFields).refine((value) => value.creator.disabled === true &&
  (value.queue_state === "closed") === ["done", "cancelled"].includes(value.status_category));
function issueView(value: z.output<typeof issueWire>) { return {
  issueId: value.issue_id, identifier: value.identifier, sourceWorkspaceId: value.source_workspace_id, sourceName: value.source_name,
  status: value.status, statusCategory: value.status_category, collaborationRevision: value.collaboration_revision,
  creator: value.creator, handler: value.handler, queueState: value.queue_state, unsettledRuns: value.unsettled_runs,
}; }
const issue = issueWire.transform(issueView);
export type AdminCollaborationIssue = z.output<typeof issue>;

export const AdminCollaborationIssueListSchema = z.object({ scope: uuid, items: z.array(issue).max(100), next_cursor: uuid.nullable() })
  .refine((value) => new Set(value.items.map((item) => item.issueId)).size === value.items.length &&
    (value.next_cursor === null || value.next_cursor === value.items.at(-1)?.issueId))
  .transform((value) => ({ scope: value.scope, items: value.items, nextCursor: value.next_cursor }));
export type AdminCollaborationIssueList = z.output<typeof AdminCollaborationIssueListSchema>;

const statusOptions = CollaborationStatusOptionsSchema.safeExtend({ action: lifecycleAction });
export const AdminCollaborationDetailSchema = issueWire.safeExtend({
  scope: uuid, actions: z.array(statusOptions).max(2), successor_required: z.boolean(),
}).refine((value) => {
  const closed = value.queue_state === "closed";
  return value.successor_required === (closed && value.handler.disabled) &&
    new Set(value.actions.map((action) => action.action)).size === value.actions.length &&
    value.actions.every((action) => closed ? action.action === "reopen" : action.action !== "reopen");
}).transform((value) => ({ ...issueView(value), scope: value.scope, actions: value.actions, successorRequired: value.successor_required }));
export type AdminCollaborationDetail = z.output<typeof AdminCollaborationDetailSchema>;

const candidate = CollaborationAccountSummarySchema.extend({ user_id: uuid })
  .transform((value) => ({ userId: value.user_id, name: value.name, avatarUrl: value.avatar_url, external: value.external }));
export const AdminCollaborationCandidatesSchema = z.object({
  scope: uuid, issue_id: uuid, collaboration_revision: positiveVersion, items: z.array(candidate).max(100), next_cursor: uuid.nullable(),
}).refine((value) => new Set(value.items.map((item) => item.userId)).size === value.items.length &&
  (value.next_cursor === null || value.next_cursor === value.items.at(-1)?.userId))
  .transform((value) => ({ scope: value.scope, issueId: value.issue_id, collaborationRevision: value.collaboration_revision, items: value.items, nextCursor: value.next_cursor }));
export type AdminCollaborationCandidates = z.output<typeof AdminCollaborationCandidatesSchema>;

export const AdminCollaborationChangeSchema = z.strictObject({
  action: lifecycleAction, expectedCollaborationRevision: positiveVersion, expectedStatus: statusKey,
  targetStatus: statusKey, statusOptionsVersion: z.string().regex(/^[a-f0-9]{64}$/),
  reason: z.string().trim().min(1).refine((value) => !value.includes("\0") && collaborationTextByteLength(value) <= 1000), successorUserId: uuid.optional(),
}).refine((value) => value.action === "reopen" || value.successorUserId === undefined);
export type AdminCollaborationChange = z.output<typeof AdminCollaborationChangeSchema>;

const receipt = z.object({
  issue_id: uuid, request_id: uuid, action: lifecycleAction, code: z.enum(["completed", "cancelled", "reopened"]),
  collaboration_revision: positiveVersion, creator_user_id: uuid, previous_handler_user_id: uuid, handler_user_id: uuid,
  previous_status: z.string().min(1), status: z.string().min(1), queue_state: z.enum(["paused", "closed"]),
  control_version: positiveVersion, unsettled_runs: z.number().int().nonnegative().safe(),
}).refine((value) => value.code === ({ complete: "completed", cancel: "cancelled", reopen: "reopened" })[value.action] &&
  (value.action === "reopen" ? value.queue_state === "paused" : value.queue_state === "closed" && value.handler_user_id === value.previous_handler_user_id))
  .transform((value) => ({ issueId: value.issue_id, requestId: value.request_id, action: value.action, code: value.code,
    collaborationRevision: value.collaboration_revision, creatorUserId: value.creator_user_id,
    previousHandlerUserId: value.previous_handler_user_id, handlerUserId: value.handler_user_id,
    previousStatus: value.previous_status, status: value.status, queueState: value.queue_state,
    controlVersion: value.control_version, unsettledRuns: value.unsettled_runs }));
export const AdminCollaborationResultSchema = z.object({ operation: adminOperationSchema, result: receipt })
  .refine(({ operation, result }) => operation.targetKind === "collaboration_issue" && operation.targetId.toLowerCase() === result.issueId &&
    operation.actorId !== null && operation.kind === `collaboration.${result.action}` && operation.resultCode === result.code &&
    operation.state === "applied" && operation.confirmation === "not_required");
export type AdminCollaborationResult = z.output<typeof AdminCollaborationResultSchema>;

export function matchesAdminCollaborationResult(value: AdminCollaborationResult, issueId: string, key: string, input?: AdminCollaborationChange): boolean {
  const result = value.result;
  if (result.issueId !== issueId.toLowerCase() || result.requestId !== key.toLowerCase()) return false;
  return !input || result.action === input.action && result.previousStatus === input.expectedStatus && result.status === input.targetStatus &&
    BigInt(result.collaborationRevision) > BigInt(input.expectedCollaborationRevision) &&
    result.handlerUserId === (input.successorUserId ?? result.previousHandlerUserId);
}

// R38 shared-context disposal reads version metadata only; no text is exposed.
export const AdminCollaborationMemoryVersionsSchema = z.object({
  scope: uuid, issue_id: uuid, current_version: nonnegativeVersion,
  items: z.array(CollaborationMemoryVersionMetaSchema).max(100), next_cursor: positiveVersion.nullable(),
}).refine((page) => page.items.every((item, index) => compareCollaborationMemoryVersions(item.version, page.current_version) <= 0 &&
  (index === 0 || compareCollaborationMemoryVersions(item.version, page.items[index - 1]!.version) < 0)) &&
  (page.next_cursor === null || page.next_cursor === page.items.at(-1)?.version))
  .transform((value) => ({ scope: value.scope, issueId: value.issue_id, currentVersion: value.current_version, items: value.items, nextCursor: value.next_cursor }));
export type AdminCollaborationMemoryVersions = z.output<typeof AdminCollaborationMemoryVersionsSchema>;

export const AdminCollaborationMemoryPurgeSchema = z.strictObject({
  version: positiveVersion,
  reason: z.string().trim().min(1).refine((value) => !value.includes("\0") && collaborationTextByteLength(value) <= 1000),
});
export type AdminCollaborationMemoryPurge = z.output<typeof AdminCollaborationMemoryPurgeSchema>;

const memoryReceipt = z.object({
  issue_id: uuid, request_id: uuid, action: z.literal("purge_memory"), code: z.literal("memory_purged"),
  version: positiveVersion, current_version: positiveVersion, bytes: z.number().int().nonnegative().safe(),
  creator_user_id: uuid, purged_at: z.iso.datetime({ offset: true }),
}).refine((value) => compareCollaborationMemoryVersions(value.version, value.current_version) < 0)
  .transform((value) => ({ issueId: value.issue_id, requestId: value.request_id, action: value.action, code: value.code,
    version: value.version, currentVersion: value.current_version, bytes: value.bytes, creatorUserId: value.creator_user_id, purgedAt: value.purged_at }));
export const AdminCollaborationMemoryResultSchema = z.object({ operation: adminOperationSchema, result: memoryReceipt })
  .refine(({ operation, result }) => operation.targetKind === "collaboration_issue" && operation.targetId.toLowerCase() === result.issueId &&
    operation.actorId !== null && operation.kind === "collaboration.purge_memory" && operation.resultCode === result.code &&
    operation.state === "applied" && operation.confirmation === "not_required");
export type AdminCollaborationMemoryResult = z.output<typeof AdminCollaborationMemoryResultSchema>;

export function matchesAdminCollaborationMemoryResult(value: AdminCollaborationMemoryResult, issueId: string, key: string, input?: AdminCollaborationMemoryPurge): boolean {
  return value.result.issueId === issueId.toLowerCase() && value.result.requestId === key.toLowerCase() &&
    (!input || value.result.version === input.version);
}
export const parseAdminCollaborationMemoryVersions = (raw: unknown): AdminCollaborationMemoryVersions | null =>
  parseWithFallback(raw, AdminCollaborationMemoryVersionsSchema, null, { endpoint: "GET /api/admin/collaboration/issues/:id/memory/versions", redact: true });
export const parseAdminCollaborationMemoryResult = (raw: unknown): AdminCollaborationMemoryResult | null =>
  parseWithFallback(raw, AdminCollaborationMemoryResultSchema, null, { endpoint: "admin collaboration shared context purge result", redact: true });

export const parseAdminCollaborationIssueList = (raw: unknown): AdminCollaborationIssueList | null =>
  parseWithFallback(raw, AdminCollaborationIssueListSchema, null, { endpoint: "GET /api/admin/collaboration/issues", redact: true });
export const parseAdminCollaborationDetail = (raw: unknown): AdminCollaborationDetail | null =>
  parseWithFallback(raw, AdminCollaborationDetailSchema, null, { endpoint: "GET /api/admin/collaboration/issues/:id", redact: true });
export const parseAdminCollaborationCandidates = (raw: unknown): AdminCollaborationCandidates | null =>
  parseWithFallback(raw, AdminCollaborationCandidatesSchema, null, { endpoint: "GET /api/admin/collaboration/issues/:id/reopen-candidates", redact: true });
export const parseAdminCollaborationResult = (raw: unknown): AdminCollaborationResult | null =>
  parseWithFallback(raw, AdminCollaborationResultSchema, null, { endpoint: "admin collaboration lifecycle result", redact: true });
