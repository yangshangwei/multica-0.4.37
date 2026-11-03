import type { QueryClient } from "@tanstack/react-query";
import type { StorageAdapter } from "../types/storage";
import { ApiError, getApi } from "../api";
import { CollaborationActionSchema, collaborationWriteContext, parseCollaborationActionDraft, recoverCollaborationAction, sendCollaborationAction, type CollaborationAction } from "./action";
import { collaborationAccessEpoch, collaborationIssueAccessDenied, collaborationKeys, CollaborationUnavailableError, isCollaborationDenied, isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope } from "./access";
import { clearCollaborationDrafts, removeCollaborationDraft, saveCollaborationDraft, type CollaborationDraft } from "./draft";
import { parseCollaborationOperation, type CollaborationIssue } from "./schema";
import { invalidateCollaborationSourceViews, waitForCollaborationIssueRefresh } from "./invalidation";
import { collaborationDeliveryRefusalStatus, matchesCollaborationDeliveryConfirmation } from "./delivery-schema";
import { finishCollaborationDeliveryDraft, readCollaborationDeliveryRecords } from "./delivery-draft";

type Intent<T = CollaborationAction> = T extends CollaborationAction ? Omit<T, keyof ReturnType<typeof collaborationWriteContext>> : never;
export type CollaborationActionIntent = Intent;

export function prepareCollaborationAction(scope: CollaborationScope, issue: CollaborationIssue, requestId: string, intent: CollaborationActionIntent): CollaborationDraft {
  if (!isCollaborationScopeCurrent(scope)) throw new CollaborationUnavailableError();
  const submission = issue.delivery?.current;
  const observation = intent.action === "complete" && intent.submissionId !== undefined ? {
    submissionAttachmentIds: submission?.id === intent.submissionId.toLowerCase() ? submission.attachment_ids : undefined,
  } : {};
  const parsed = CollaborationActionSchema.safeParse({ ...intent, ...observation, ...collaborationWriteContext(issue, requestId) });
  if (!parsed.success) throw new CollaborationUnavailableError();
  return { scope: { server: scope.server, userId: scope.userId, issueId: issue.id }, requestId, kind: "operation", payload: parsed.data, createdAt: Date.now() };
}

/** Preserve before POST, retire only a validated receipt, and re-read live state. */
export async function executeCollaborationAction(client: QueryClient, scope: CollaborationScope, source: CollaborationDraft, recovery = false, storage?: StorageAdapter) {
  const action = parseCollaborationActionDraft(source);
  const draft: CollaborationDraft = { ...source, scope: { ...source.scope }, payload: action };
  if (draft.scope.server !== scope.server || draft.scope.userId.toLowerCase() !== scope.userId.toLowerCase()) throw new CollaborationUnavailableError();
  const denied = collaborationIssueAccessDenied(client, scope, draft.scope.issueId);
  const epoch = collaborationAccessEpoch(client, scope, draft.scope.issueId);
  try {
    const result = await protectCollaborationRead(client, scope, recovery && denied ? null : draft.scope.issueId, async () => {
      if (!denied) saveCollaborationDraft(draft, storage);
      try {
        const result = recovery && denied ? await getApi().getCollaborationOperation(draft.scope.issueId, draft.requestId)
          : await (recovery ? recoverCollaborationAction(scope, draft) : sendCollaborationAction(scope, draft.scope.issueId, action));
        const refusalStatus = result ? collaborationDeliveryRefusalStatus(result.code) : null;
        if (result && refusalStatus !== null) {
          if (result.delivery_confirmation !== undefined || result.status !== undefined && result.status !== refusalStatus) throw new CollaborationUnavailableError();
          return { ...result, status: refusalStatus };
        }
        const formal = action.action === "submit_acceptance" || action.action === "complete" || action.action === "return_acceptance" || action.action === "change_ai_policy";
        if (result && formal && result.status !== undefined && result.status >= 300) throw new CollaborationUnavailableError();
        if (result && action.action === "return_acceptance" && result.code !== "acceptance_returned") throw new CollaborationUnavailableError();
        if (result && action.action === "change_ai_policy" && result.code !== "ai_policy_changed" && result.code !== "ai_policy_unchanged") throw new CollaborationUnavailableError();
        if (result && (action.action === "submit_acceptance" || action.action === "complete") && (result.status === undefined || result.status < 300)) {
          const expectedCode = action.action === "complete" ? "completed" : "acceptance_submitted";
          const expectedFiles = action.action === "complete" && action.submissionId !== undefined ? action.submissionAttachmentIds : action.attachmentIds;
          if (result.code !== expectedCode || !matchesCollaborationDeliveryConfirmation(result.delivery_confirmation, expectedFiles, "submissionId" in action ? action.submissionId : undefined)) {
            throw new CollaborationUnavailableError();
          }
        }
        return result;
      } catch (error) {
        if (error instanceof ApiError && !error.isStaleAuthResponse) {
          const receipt = parseCollaborationOperation(error.body);
          if (receipt && receipt.delivery_confirmation === undefined && collaborationDeliveryRefusalStatus(receipt.code) === error.status && (receipt.status === undefined || receipt.status === error.status)) return { ...receipt, status: error.status };
        }
        throw error;
      }
    });
    if (result && (result.status === undefined || result.status < 300) && (action.action === "submit_acceptance" || action.action === "complete") && action.deliveryDraftId) {
      const deliveryDraftId = action.deliveryDraftId.toLowerCase();
      const form = readCollaborationDeliveryRecords(draft.scope, storage).find((record) => record.kind === "form" && record.purpose === "result" &&
        record.draftId === deliveryDraftId && record.scope.context.kind === "issue" && record.scope.context.issueId === draft.scope.issueId.toLowerCase());
      if (form) finishCollaborationDeliveryDraft(form.scope, form.draftId, storage);
    }
    removeCollaborationDraft(draft, storage);
    invalidateCollaborationSourceViews(client, scope, draft.scope.issueId);
    if (!denied) {
      // The scope invalidation starts all active account reads, including the
      // issue query. Wait only for that issue read before returning so the next
      // action captures the versions committed by this write. Calling a second
      // issue invalidation here would cancel/restart the same request twice.
      void client.invalidateQueries({ queryKey: collaborationKeys.scope(scope) });
      await waitForCollaborationIssueRefresh(client, scope, draft.scope.issueId);
      if (!isCollaborationScopeCurrent(scope) || collaborationAccessEpoch(client, scope, draft.scope.issueId) !== epoch) {
        throw new DOMException("Collaboration access changed", "AbortError");
      }
    }
    return result;
  } catch (error) {
    if (isCollaborationDenied(error)) clearCollaborationDrafts(storage, draft.scope);
    throw error;
  }
}
