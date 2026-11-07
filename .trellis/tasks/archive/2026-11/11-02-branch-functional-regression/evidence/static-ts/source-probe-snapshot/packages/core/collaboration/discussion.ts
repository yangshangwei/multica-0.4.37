import { invalidateCollaborationSourceViews } from "./invalidation";
import { z } from "zod";
import type { QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import type { StorageAdapter } from "../types/storage";
import { collaborationIssueAccessDenied, collaborationKeys, CollaborationUnavailableError, isCollaborationDenied, protectCollaborationRead, type CollaborationScope } from "./access";
import { CollaborationDiscussionSelectionSchema, type CollaborationDiscussionSelection } from "./discussion-schema";
import { clearCollaborationDrafts, removeCollaborationDraft, saveCollaborationDraft, type CollaborationDraft } from "./draft";
import type { CollaborationIssue } from "./schema";

const payload = z.strictObject({ domain: z.literal("discussion"), selection: CollaborationDiscussionSelectionSchema });
export function parseCollaborationDiscussionDraft(draft: CollaborationDraft) {
  const parsed = payload.safeParse(draft.payload);
  if (draft.kind !== "operation" || !parsed.success) throw new CollaborationUnavailableError();
  return parsed.data.selection;
}
export function prepareCollaborationDiscussion(scope: CollaborationScope, issue: CollaborationIssue, requestId: string, selection: CollaborationDiscussionSelection): CollaborationDraft {
  const resolving = selection.action === "resolve" || selection.action === "unresolve";
  if (!z.uuid().safeParse(requestId).success || issue.capabilities.comment !== true || (resolving ? issue.capabilities.resolve_thread : issue.capabilities.react) !== true) throw new CollaborationUnavailableError();
  return { scope: { server: scope.server, userId: scope.userId, issueId: issue.id }, requestId, kind: "operation", payload: payload.parse({ domain: "discussion", selection }), createdAt: Date.now() };
}
export async function executeCollaborationDiscussion(client: QueryClient, scope: CollaborationScope, source: CollaborationDraft, recovery = false, storage?: StorageAdapter) {
  const selection = parseCollaborationDiscussionDraft(source);
  const draft = { ...source, scope: { ...source.scope }, payload: payload.parse(source.payload) };
  if (draft.scope.server !== scope.server || draft.scope.userId !== scope.userId) throw new CollaborationUnavailableError();
  const denied = collaborationIssueAccessDenied(client, scope, draft.scope.issueId);
  try {
    const receipt = await protectCollaborationRead(client, scope, recovery && denied ? null : draft.scope.issueId, async () => {
      if (!denied) saveCollaborationDraft(draft, storage);
      const api = getApi();
      const result = recovery ? await api.getCollaborationDiscussionOperation(draft.scope.issueId, draft.requestId)
        : await api.changeCollaborationDiscussion(draft.scope.issueId, draft.requestId, selection);
      return result && result.issue_id === draft.scope.issueId && result.request_id === draft.requestId && result.comment_id === selection.commentId && result.action === selection.action && result.emoji === ("emoji" in selection ? selection.emoji : null) ? result : null;
    });
    removeCollaborationDraft(draft, storage);
    if (!denied) {
      invalidateCollaborationSourceViews(client, scope, draft.scope.issueId);
      void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, draft.scope.issueId) });
    }
    return receipt;
  } catch (error) { if (isCollaborationDenied(error)) clearCollaborationDrafts(storage, draft.scope); throw error; }
}
