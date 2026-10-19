import type { QueryClient } from "@tanstack/react-query";
import type { StorageAdapter } from "../types/storage";
import { ApiError, getApi } from "../api";
import { CollaborationActionSchema, collaborationWriteContext, parseCollaborationActionDraft, recoverCollaborationAction, sendCollaborationAction, type CollaborationAction } from "./action";
import { collaborationIssueAccessDenied, collaborationKeys, CollaborationUnavailableError, isCollaborationDenied, isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope } from "./access";
import { clearCollaborationDrafts, removeCollaborationDraft, saveCollaborationDraft, type CollaborationDraft } from "./draft";
import { parseCollaborationOperation, type CollaborationIssue } from "./schema";
import { invalidateCollaborationSourceViews } from "./invalidation";

type Intent<T = CollaborationAction> = T extends CollaborationAction ? Omit<T, keyof ReturnType<typeof collaborationWriteContext>> : never;
export type CollaborationActionIntent = Intent;

export function prepareCollaborationAction(scope: CollaborationScope, issue: CollaborationIssue, requestId: string, intent: CollaborationActionIntent): CollaborationDraft {
  if (!isCollaborationScopeCurrent(scope)) throw new CollaborationUnavailableError();
  const parsed = CollaborationActionSchema.safeParse({ ...intent, ...collaborationWriteContext(issue, requestId) });
  if (!parsed.success) throw new CollaborationUnavailableError();
  return { scope: { server: scope.server, userId: scope.userId, issueId: issue.id }, requestId, kind: "operation", payload: parsed.data, createdAt: Date.now() };
}

/** Preserve before POST, retire only a validated receipt, and re-read live state. */
export async function executeCollaborationAction(client: QueryClient, scope: CollaborationScope, source: CollaborationDraft, recovery = false, storage?: StorageAdapter) {
  const action = parseCollaborationActionDraft(source);
  const draft: CollaborationDraft = { ...source, scope: { ...source.scope }, payload: action };
  if (draft.scope.server !== scope.server || draft.scope.userId.toLowerCase() !== scope.userId.toLowerCase()) throw new CollaborationUnavailableError();
  const denied = collaborationIssueAccessDenied(client, scope, draft.scope.issueId);
  try {
    const result = await protectCollaborationRead(client, scope, recovery && denied ? null : draft.scope.issueId, async () => {
      if (!denied) saveCollaborationDraft(draft, storage);
      try {
        if (recovery && denied) return await getApi().getCollaborationOperation(draft.scope.issueId, draft.requestId);
        return await (recovery ? recoverCollaborationAction(scope, draft) : sendCollaborationAction(scope, draft.scope.issueId, action));
      } catch (error) {
        if (error instanceof ApiError && !error.isStaleAuthResponse && error.status === 409) {
          const receipt = parseCollaborationOperation(error.body);
          if (receipt?.code === "stale_revision" && (receipt.status === undefined || receipt.status === 409)) return { ...receipt, status: 409 };
        }
        throw error;
      }
    });
    removeCollaborationDraft(draft, storage);
    invalidateCollaborationSourceViews(client, scope, draft.scope.issueId);
    if (!denied) void client.invalidateQueries({ queryKey: collaborationKeys.scope(scope) });
    return result;
  } catch (error) {
    if (isCollaborationDenied(error)) clearCollaborationDrafts(storage, draft.scope);
    throw error;
  }
}
