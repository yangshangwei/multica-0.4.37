import { invalidateCollaborationSourceViews } from "./invalidation";
import { z } from "zod";
import type { QueryClient } from "@tanstack/react-query";
import type { StorageAdapter } from "../types/storage";
import { getApi } from "../api";
import { collaborationIssueAccessDenied, collaborationKeys, CollaborationUnavailableError, isCollaborationDenied, protectCollaborationRead, type CollaborationScope } from "./access";
import { clearCollaborationDrafts, readCollaborationDrafts, removeCollaborationDraft, saveCollaborationDraft, type CollaborationDraft } from "./draft";
import { matchesCollaborationCommentAttachments, type CollaborationComment, type CollaborationCommentReceipt } from "./content-schema";
import { collaborationAttachmentIds } from "./attachment-schema";
import { positiveVersion } from "./version";
import type { CollaborationIssue } from "./schema";
import { collaborationCommentWithinByteLimit } from "./text-limits";

const base = { domain: z.literal("comment_change"), commentId: z.uuid() };
// Keep the saved shape readable for original-result recovery. New edits enforce
// the stricter UTF-8 byte limit before preparation or storage below.
const payload = z.discriminatedUnion("action", [
  z.strictObject({ ...base, action: z.literal("edit"), input: z.strictObject({ content: z.string().max(65536), expected_comment_revision: positiveVersion, attachment_ids: collaborationAttachmentIds.optional() }) }),
  z.strictObject({ ...base, action: z.literal("delete"), input: z.strictObject({ expected_comment_revision: positiveVersion }) }),
]);
export type CollaborationCommentChange = { action: "edit"; content: string; attachmentIds?: string[] } | { action: "delete" };

export function parseCollaborationCommentChangeDraft(draft: CollaborationDraft) {
  const parsed = payload.safeParse(draft.payload);
  if (draft.kind !== "operation" || !parsed.success) throw new CollaborationUnavailableError();
  return parsed.data;
}

export function prepareCollaborationCommentChange(scope: CollaborationScope, issue: CollaborationIssue, comment: CollaborationComment, requestId: string, change: CollaborationCommentChange): CollaborationDraft {
  if (!z.uuid().safeParse(requestId).success || comment.issue_id !== issue.id || issue.capabilities.comment !== true || comment.can_edit !== true ||
    comment.author_type !== "member" || comment.type !== "comment" || comment.author?.user_id !== scope.userId || comment.deleted_at !== null) throw new CollaborationUnavailableError();
  if (change.action === "edit" && !collaborationCommentWithinByteLimit(change.content)) throw new RangeError("Comment exceeds its UTF-8 byte limit");
  const data = payload.parse({ domain: "comment_change", commentId: comment.id, action: change.action,
    input: { expected_comment_revision: comment.comment_revision, ...(change.action === "edit" ? { content: change.content, ...(change.attachmentIds ? { attachment_ids: change.attachmentIds } : {}) } : {}) } });
  return { scope: { server: scope.server, userId: scope.userId, issueId: issue.id }, kind: "operation", requestId, payload: data, createdAt: Date.now() };
}

export async function executeCollaborationCommentChange(client: QueryClient, scope: CollaborationScope, source: CollaborationDraft, recovery = false, storage?: StorageAdapter) {
  const data = parseCollaborationCommentChangeDraft(source);
  const draft = { ...source, scope: { ...source.scope }, payload: data };
  if (draft.scope.server !== scope.server || draft.scope.userId !== scope.userId) throw new CollaborationUnavailableError();
  const oversized = data.action === "edit" && !collaborationCommentWithinByteLimit(data.input.content);
  const denied = collaborationIssueAccessDenied(client, scope, draft.scope.issueId);
  if (oversized && !(recovery && denied)) {
    const saved = readCollaborationDrafts(draft.scope, storage).find((item) => item.requestId === draft.requestId);
    if (!saved || JSON.stringify(parseCollaborationCommentChangeDraft(saved)) !== JSON.stringify(data)) throw new RangeError("Comment exceeds its UTF-8 byte limit");
  }
  function matches(result: CollaborationCommentReceipt | null): result is CollaborationCommentReceipt {
    return result !== null && result.issue_id === draft.scope.issueId && result.comment_id === data.commentId &&
      result.request_id === draft.requestId && result.action === `${data.action}_comment` && result.targets.length === 0;
  }
  try {
    const receipt = await protectCollaborationRead(client, scope, recovery && denied ? null : draft.scope.issueId, async () => {
      if (!denied) saveCollaborationDraft(draft, storage);
      const api = getApi();
      let result = recovery ? await api.getCollaborationCommentOperation(draft.scope.issueId, draft.requestId)
        : data.action === "edit" ? await api.editCollaborationComment(draft.scope.issueId, data.commentId, draft.requestId, data.input)
          : await api.deleteCollaborationComment(draft.scope.issueId, data.commentId, draft.requestId, data.input.expected_comment_revision);
      if (oversized && !recovery) {
        // Only an exact, already-saved invalid request may be replayed to seal
        // its refusal. Retirement still requires the HTTP 200 own-result GET.
        if (!matches(result) || result.status !== 400 || result.code !== "invalid_comment_request") return null;
        result = await api.getCollaborationCommentOperation(draft.scope.issueId, draft.requestId);
      }
      if (!matches(result)) return null;
      if (!denied && result.status < 300 && data.action === "edit" && !matchesCollaborationCommentAttachments(result, data.input.attachment_ids)) return null;
      if (result.status < 400 && (result.status !== 200 || ![data.action === "edit" ? "comment_edited" : "comment_deleted", "unchanged"].includes(result.code))) return null;
      return result;
    });
    removeCollaborationDraft(draft, storage);
    if (!denied) {
      invalidateCollaborationSourceViews(client, scope, draft.scope.issueId);
      void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, draft.scope.issueId) });
    }
    return receipt;
  } catch (error) { if (isCollaborationDenied(error)) clearCollaborationDrafts(storage, draft.scope); throw error; }
}
