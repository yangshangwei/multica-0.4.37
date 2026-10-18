import type { QueryClient } from "@tanstack/react-query";
import type { StorageAdapter } from "../types/storage";
import { getApi } from "../api";
import { CollaborationActionSchema, collaborationWriteContext, parseCollaborationActionDraft, recoverCollaborationAction, sendCollaborationAction, type CollaborationAction } from "./action";
import { collaborationIssueAccessDenied, collaborationKeys, CollaborationUnavailableError, isCollaborationDenied, isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope } from "./access";
import { clearCollaborationDrafts, removeCollaborationDraft, saveCollaborationDraft, type CollaborationDraft } from "./draft";
import type { CollaborationIssue } from "./schema";

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
      if (recovery && denied) return getApi().getCollaborationOperation(draft.scope.issueId, draft.requestId);
      return recovery ? recoverCollaborationAction(scope, draft) : sendCollaborationAction(scope, draft.scope.issueId, action);
    });
    removeCollaborationDraft(draft, storage);
    if (!denied) void client.invalidateQueries({ queryKey: collaborationKeys.scope(scope) });
    return result;
  } catch (error) {
    if (isCollaborationDenied(error)) clearCollaborationDrafts(storage, draft.scope);
    throw error;
  }
}
