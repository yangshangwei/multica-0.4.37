import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { ApiError, getApi } from "../api";
import { collaborationKeys, CollaborationUnavailableError, isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope } from "./access";
import { expireKnowledgeProjections } from "./knowledge-cache";
import { KnowledgeRecoveryStorageError, retireKnowledgeOperationIdentity, saveKnowledgeOperationIdentity } from "./knowledge-recovery";
import type { StorageAdapter } from "../types/storage";
import {
  CollaborationKnowledgeApproveRequestSchema, CollaborationKnowledgeDecisionRequestSchema, CollaborationKnowledgeProposalSchema,
  knowledgeReceiptConfirmed, knowledgeReceiptRefusalStatus,
  type CollaborationKnowledgeCandidate, type CollaborationKnowledgeCandidates, type CollaborationKnowledgeProposal, type CollaborationKnowledgeReceipt,
} from "./knowledge-schema";

export interface KnowledgeOperationIdentity {
  readonly scope: Pick<CollaborationScope, "server" | "userId">; readonly candidateId: string; readonly requestId: string;
  readonly action: CollaborationKnowledgeReceipt["action"];
}
/** Frozen serialized client intent: retries cannot silently adopt a newer reviewed revision. Never persist server projections. */
export interface KnowledgeIntent extends KnowledgeOperationIdentity { readonly requestJSON: string; readonly issueId?: string }

/** The original identity is proven unused; the human may edit and create a new intent. */
export class KnowledgeUnusedIntentError extends Error {
  constructor(readonly reason: "storage" | "invalid" | "rejected", readonly cleanupFailed: boolean, readonly code?: string) {
    super(reason === "storage" ? "The original knowledge request could not be preserved before sending" : "The knowledge intent was not applied");
    this.name = "KnowledgeUnusedIntentError";
  }
}
/** A known server result survives failure to remove its local recovery identity. */
export class KnowledgeReceiptCleanupError extends Error {
  constructor(readonly receipt: CollaborationKnowledgeReceipt) {
    super("The knowledge result is confirmed; its local recovery identity could not be removed");
    this.name = "KnowledgeReceiptCleanupError";
  }
}

// Client dispatch facts only. Stored recovery identities cover reload; this
// counter also protects overlapping calls and replacement storage adapters.
const attempts = new WeakMap<QueryClient, Map<string, { count: number }>>();
function attemptState(client: QueryClient, identity: KnowledgeOperationIdentity) {
  let entries = attempts.get(client);
  if (!entries) { entries = new Map(); attempts.set(client, entries); }
  const key = JSON.stringify([identity.scope.server, identity.scope.userId, identity.requestId, identity.candidateId, identity.action]);
  let state = entries.get(key);
  if (!state) { state = { count: 0 }; entries.set(key, state); }
  return state;
}

function unusedIntent(identity: KnowledgeOperationIdentity, reason: KnowledgeUnusedIntentError["reason"], storage?: StorageAdapter, code?: string): KnowledgeUnusedIntentError {
  let cleanupFailed = false;
  try { retireKnowledgeOperationIdentity(identity, storage); } catch { cleanupFailed = true; }
  return new KnowledgeUnusedIntentError(reason, cleanupFailed, code);
}

function definitiveUnsealedRejection(error: unknown): string | null {
  if (!(error instanceof ApiError) || error.isStaleAuthResponse) return null;
  const parsed = z.strictObject({ code: z.enum(["invalid_knowledge_request", "knowledge_candidate_too_large", "knowledge_candidate_not_found",
    "knowledge_target_not_found", "knowledge_permission_denied", "knowledge_revision_required", "knowledge_state_conflict", "knowledge_source_unavailable", "knowledge_expired"]),
  error: z.string().optional() }).safeParse(error.body);
  return parsed.success && knowledgeReceiptRefusalStatus(parsed.data.code) === error.status ? parsed.data.code : null;
}

function finishKnowledgeReceipt(client: QueryClient, scope: CollaborationScope, identity: KnowledgeOperationIdentity, receipt: CollaborationKnowledgeReceipt, storage?: StorageAdapter) {
  expireKnowledgeProjections(client, scope);
  try { retireKnowledgeOperationIdentity(identity, storage); }
  catch { throw new KnowledgeReceiptCleanupError(receipt); }
  return receipt;
}

export function prepareKnowledgeProposal(scope: CollaborationScope, view: CollaborationKnowledgeCandidates,
  proposal: Omit<CollaborationKnowledgeProposal, "protocol_version" | "candidate_id" | "request_id">): KnowledgeIntent {
  if (!isCollaborationScopeCurrent(scope) || view.capabilities.can_propose !== true) throw new CollaborationUnavailableError();
  const input = CollaborationKnowledgeProposalSchema.parse({ ...proposal, protocol_version: 1, candidate_id: crypto.randomUUID(), request_id: crypto.randomUUID() });
  if (!input.sources.every((source) => source.issue_id === view.issue_id)) throw new TypeError("Knowledge sources must belong to the source issue");
  return Object.freeze({ scope: Object.freeze({ server: scope.server, userId: scope.userId }), issueId: view.issue_id, candidateId: input.candidate_id,
    requestId: input.request_id, action: "propose", requestJSON: JSON.stringify(input) });
}

export function prepareKnowledgeDecision(scope: CollaborationScope, candidate: CollaborationKnowledgeCandidate,
  action: "approve" | "reject" | "withdraw", reason?: string): KnowledgeIntent {
  if (!isCollaborationScopeCurrent(scope)) throw new CollaborationUnavailableError();
  const requestId = crypto.randomUUID();
  const base = { protocol_version: 1, request_id: requestId, expected_state_revision: candidate.state_revision };
  let input;
  switch (action) {
    case "approve":
      if (candidate.availability !== "available" || candidate.capabilities.can_approve !== true || candidate.state !== "pending") throw new CollaborationUnavailableError();
      input = CollaborationKnowledgeApproveRequestSchema.parse({ ...base, expected_version: candidate.version, expected_content_sha256: candidate.content_sha256 });
      break;
    case "reject":
      if (candidate.availability === "restricted" || candidate.capabilities.can_reject !== true || candidate.state !== "pending") throw new CollaborationUnavailableError();
      input = CollaborationKnowledgeDecisionRequestSchema.parse({ ...base, reason });
      break;
    case "withdraw":
      if (candidate.capabilities.can_withdraw !== true || !["pending", "approved"].includes(candidate.state)) throw new CollaborationUnavailableError();
      input = CollaborationKnowledgeDecisionRequestSchema.parse({ ...base, reason });
      break;
    default: throw new CollaborationUnavailableError();
  }
  return Object.freeze({ scope: Object.freeze({ server: scope.server, userId: scope.userId }), candidateId: candidate.candidate_id, requestId, action, requestJSON: JSON.stringify(input) });
}

export function matchesKnowledgeReceipt(identity: KnowledgeOperationIdentity, receipt: CollaborationKnowledgeReceipt | null): receipt is CollaborationKnowledgeReceipt {
  if (!receipt || receipt.candidate_id !== identity.candidateId || receipt.request_id !== identity.requestId || receipt.action !== identity.action) return false;
  return knowledgeReceiptConfirmed(receipt);
}

function checkScope(scope: CollaborationScope, identity: KnowledgeOperationIdentity): void {
  if (!isCollaborationScopeCurrent(scope) || identity.scope.server !== scope.server || identity.scope.userId !== scope.userId ||
    !["propose", "approve", "reject", "withdraw"].includes(identity.action) ||
    !z.uuid().safeParse(identity.candidateId).success || !z.uuid().safeParse(identity.requestId).success) throw new CollaborationUnavailableError();
}

/** GET-only, account-scoped recovery works after both source access and target role are lost. */
export async function recoverKnowledgeOperation(client: QueryClient, scope: CollaborationScope, identity: KnowledgeOperationIdentity, storage?: StorageAdapter): Promise<CollaborationKnowledgeReceipt> {
  checkScope(scope, identity);
  const result = await protectCollaborationRead(client, scope, null, async () => {
    const receipt = await getApi().getCollaborationKnowledgeOperation(identity.candidateId, identity.requestId);
    return matchesKnowledgeReceipt(identity, receipt) ? receipt : null;
  });
  return finishKnowledgeReceipt(client, scope, identity, result, storage);
}

/** Await an exact receipt. An ambiguous response leaves the immutable intent available for the same-key retry/recovery. */
export async function executeKnowledgeIntent(client: QueryClient, scope: CollaborationScope, intent: KnowledgeIntent, storage?: StorageAdapter): Promise<CollaborationKnowledgeReceipt> {
  checkScope(scope, intent);
  const attempt = attemptState(client, intent);
  const hadLocalAttempt = attempt.count > 0;
  let previouslyStored: boolean;
  try { previouslyStored = saveKnowledgeOperationIdentity(intent, storage); }
  catch (error) {
    if (!hadLocalAttempt && attempt.count === 0 && error instanceof KnowledgeRecoveryStorageError && error.previouslyStored === false) throw unusedIntent(intent, "storage", storage);
    throw error;
  }
  let dispatched = false;
  const dispatch = (request: () => Promise<CollaborationKnowledgeReceipt | null>) => {
    dispatched = true; attempt.count++;
    return request();
  };
  try {
    const result = await protectCollaborationRead(client, scope, null, async () => {
      let raw: unknown;
      try { raw = JSON.parse(intent.requestJSON); } catch { throw new CollaborationUnavailableError(); }
      const api = getApi();
      let receipt: CollaborationKnowledgeReceipt | null;
      if (intent.action === "propose") {
        const input = CollaborationKnowledgeProposalSchema.parse(raw);
        if (!intent.issueId || input.candidate_id !== intent.candidateId || input.request_id !== intent.requestId) throw new CollaborationUnavailableError();
        receipt = await dispatch(() => api.proposeCollaborationKnowledge(intent.issueId!, input));
      } else if (intent.action === "approve") {
        const input = CollaborationKnowledgeApproveRequestSchema.parse(raw);
        if (input.request_id !== intent.requestId) throw new CollaborationUnavailableError();
        receipt = await dispatch(() => api.approveCollaborationKnowledge(intent.candidateId, input));
        if (receipt?.code === "knowledge_approved" && receipt.version !== input.expected_version) throw new CollaborationUnavailableError();
      } else {
        const input = CollaborationKnowledgeDecisionRequestSchema.parse(raw);
        if (input.request_id !== intent.requestId) throw new CollaborationUnavailableError();
        receipt = await dispatch(() => intent.action === "reject" ? api.rejectCollaborationKnowledge(intent.candidateId, input) : api.withdrawCollaborationKnowledge(intent.candidateId, input));
      }
      return matchesKnowledgeReceipt(intent, receipt) ? receipt : null;
    });
    return finishKnowledgeReceipt(client, scope, intent, result, storage);
  } catch (error) {
    if (isCollaborationScopeCurrent(scope) && error instanceof ApiError && [401, 403, 404, 409, 410, 428].includes(error.status)) expireKnowledgeProjections(client, scope);
    if (!hadLocalAttempt && !previouslyStored && isCollaborationScopeCurrent(scope)) {
      if (!dispatched && attempt.count === 0) throw unusedIntent(intent, "invalid", storage);
      const code = attempt.count === 1 ? definitiveUnsealedRejection(error) : null;
      if (code !== null) {
        const unused = unusedIntent(intent, "rejected", storage, code);
        attempt.count = 0;
        throw unused;
      }
    }
    throw error;
  }
}

export function useCollaborationKnowledgeAction(scope: CollaborationScope) {
  const client = useQueryClient();
  return useMutation({ mutationKey: [...collaborationKeys.scope(scope), "knowledge-action"], retry: false, gcTime: 0,
    mutationFn: (intent: KnowledgeIntent) => executeKnowledgeIntent(client, scope, intent) });
}
