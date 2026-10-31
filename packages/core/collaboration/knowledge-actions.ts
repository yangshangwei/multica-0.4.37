import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { ApiError, getApi } from "../api";
import { collaborationKeys, CollaborationUnavailableError, isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope } from "./access";
import { expireKnowledgeProjections } from "./knowledge-cache";
import { retireKnowledgeOperationIdentity, saveKnowledgeOperationIdentity } from "./knowledge-recovery";
import type { StorageAdapter } from "../types/storage";
import {
  CollaborationKnowledgeApproveRequestSchema, CollaborationKnowledgeDecisionRequestSchema, CollaborationKnowledgeProposalSchema,
  knowledgeReceiptConfirmed,
  type CollaborationKnowledgeCandidate, type CollaborationKnowledgeCandidates, type CollaborationKnowledgeProposal, type CollaborationKnowledgeReceipt,
} from "./knowledge-schema";

export interface KnowledgeOperationIdentity {
  readonly scope: Pick<CollaborationScope, "server" | "userId">; readonly candidateId: string; readonly requestId: string;
  readonly action: CollaborationKnowledgeReceipt["action"];
}
/** Frozen serialized client intent: retries cannot silently adopt a newer reviewed revision. Never persist server projections. */
export interface KnowledgeIntent extends KnowledgeOperationIdentity { readonly requestJSON: string; readonly issueId?: string }

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
  retireKnowledgeOperationIdentity(identity, storage);
  expireKnowledgeProjections(client, scope);
  return result;
}

/** Await an exact receipt. An ambiguous response leaves the immutable intent available for the same-key retry/recovery. */
export async function executeKnowledgeIntent(client: QueryClient, scope: CollaborationScope, intent: KnowledgeIntent, storage?: StorageAdapter): Promise<CollaborationKnowledgeReceipt> {
  checkScope(scope, intent);
  saveKnowledgeOperationIdentity(intent, storage);
  try {
    const result = await protectCollaborationRead(client, scope, null, async () => {
      let raw: unknown;
      try { raw = JSON.parse(intent.requestJSON); } catch { throw new CollaborationUnavailableError(); }
      const api = getApi();
      let receipt: CollaborationKnowledgeReceipt | null;
      if (intent.action === "propose") {
        const input = CollaborationKnowledgeProposalSchema.parse(raw);
        if (!intent.issueId || input.candidate_id !== intent.candidateId || input.request_id !== intent.requestId) throw new CollaborationUnavailableError();
        receipt = await api.proposeCollaborationKnowledge(intent.issueId, input);
      } else if (intent.action === "approve") {
        const input = CollaborationKnowledgeApproveRequestSchema.parse(raw);
        if (input.request_id !== intent.requestId) throw new CollaborationUnavailableError();
        receipt = await api.approveCollaborationKnowledge(intent.candidateId, input);
        if (receipt?.code === "knowledge_approved" && receipt.version !== input.expected_version) throw new CollaborationUnavailableError();
      } else {
        const input = CollaborationKnowledgeDecisionRequestSchema.parse(raw);
        if (input.request_id !== intent.requestId) throw new CollaborationUnavailableError();
        receipt = intent.action === "reject" ? await api.rejectCollaborationKnowledge(intent.candidateId, input) : await api.withdrawCollaborationKnowledge(intent.candidateId, input);
      }
      return matchesKnowledgeReceipt(intent, receipt) ? receipt : null;
    });
    retireKnowledgeOperationIdentity(intent, storage);
    expireKnowledgeProjections(client, scope);
    return result;
  } catch (error) {
    if (isCollaborationScopeCurrent(scope) && error instanceof ApiError && [401, 403, 404, 409, 410, 428].includes(error.status)) expireKnowledgeProjections(client, scope);
    throw error;
  }
}

export function useCollaborationKnowledgeAction(scope: CollaborationScope) {
  const client = useQueryClient();
  return useMutation({ mutationKey: [...collaborationKeys.scope(scope), "knowledge-action"], retry: false, gcTime: 0,
    mutationFn: (intent: KnowledgeIntent) => executeKnowledgeIntent(client, scope, intent) });
}
