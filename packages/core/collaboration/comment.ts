import { invalidateCollaborationSourceViews } from "./invalidation";
import { z } from "zod";
import type { QueryClient } from "@tanstack/react-query";
import type { StorageAdapter } from "../types/storage";
import { getApi } from "../api";
import { CollaborationCommentInputSchema, matchesCollaborationCommentAttachments, type CollaborationMentionTargets, type CollaborationTriggerPreview } from "./content-schema";
import { collaborationKeys, CollaborationUnavailableError, isCollaborationDenied, protectCollaborationRead, type CollaborationScope } from "./access";
import { clearCollaborationDrafts, removeCollaborationDraft, saveCollaborationDraft, type CollaborationDraft } from "./draft";
import { finishCollaborationDeliveryDraft, readCollaborationDeliveryRecords } from "./delivery-draft";

const payload = z.strictObject({ domain: z.literal("create_comment"), input: CollaborationCommentInputSchema, deliveryDraftId: z.uuid().optional() });
export function parseCollaborationCommentDraft(draft: CollaborationDraft) {
  const parsed = payload.safeParse(draft.payload);
  if (draft.kind !== "comment" || !parsed.success) throw new CollaborationUnavailableError();
  return parsed.data.input;
}
export function prepareCollaborationComment(scope: CollaborationScope, issueId: string, requestId: string, content: string, suppressed: string[], preview: CollaborationTriggerPreview, attachments?: { draftId?: string; attachmentIds: string[] }, parentId: string | null = null): CollaborationDraft {
  if (preview.issue_id !== issueId.toLowerCase() || !z.uuid().safeParse(requestId).success) throw new CollaborationUnavailableError();
  const input = CollaborationCommentInputSchema.parse({ content, parent_id: parentId, suppressed_target_ids: suppressed,
    ...(attachments?.attachmentIds.length ? { ...(attachments.draftId ? { draft_id: attachments.draftId } : {}), attachment_ids: attachments.attachmentIds } : {}),
    expected_lifecycle_epoch: preview.lifecycle_epoch, expected_participant_epoch: preview.write_context?.participant_epoch ?? null,
    expected_account_epoch: preview.write_context?.account_epoch ?? null,
    target_versions: preview.targets.filter((target) => target.target_id !== null && target.target_epoch !== undefined && target.config_version !== undefined).map((target) => ({ target_id: target.target_id, expected_target_epoch: target.target_epoch, expected_config_version: target.config_version })),
  });
  return { scope: { server: scope.server, userId: scope.userId, issueId }, requestId, kind: "comment",
    payload: payload.parse({ domain: "create_comment", input, ...(attachments?.draftId ? { deliveryDraftId: attachments.draftId } : {}) }), createdAt: Date.now() };
}
export async function executeCollaborationComment(client: QueryClient, scope: CollaborationScope, source: CollaborationDraft, recovery = false, storage?: StorageAdapter) {
  const input = parseCollaborationCommentDraft(source);
  const draft = { ...source, scope: { ...source.scope }, payload: payload.parse(source.payload) };
  if (draft.scope.server !== scope.server || draft.scope.userId !== scope.userId) throw new CollaborationUnavailableError();
  try {
    const receipt = await protectCollaborationRead(client, scope, draft.scope.issueId, async () => {
      saveCollaborationDraft(draft, storage);
      const result = recovery ? await getApi().getCollaborationCommentOperation(draft.scope.issueId, draft.requestId) : await getApi().createCollaborationComment(draft.scope.issueId, draft.requestId, input);
      return result && matchesCollaborationCommentAttachments(result, input.attachment_ids) ? result : null;
    });
    const deliveryDraftId = draft.payload.deliveryDraftId ?? input.draft_id;
    if (receipt.status >= 200 && receipt.status < 300 && deliveryDraftId) {
      const delivery = readCollaborationDeliveryRecords(draft.scope, storage).find((record) => record.draftId === deliveryDraftId &&
        record.scope.context.kind === "issue" && record.scope.context.issueId === draft.scope.issueId.toLowerCase());
      if (delivery) finishCollaborationDeliveryDraft(delivery.scope, deliveryDraftId, storage);
    }
    removeCollaborationDraft(draft, storage);
    invalidateCollaborationSourceViews(client, scope, draft.scope.issueId);
    void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, draft.scope.issueId) });
    return receipt;
  } catch (error) { if (isCollaborationDenied(error)) clearCollaborationDrafts(storage, draft.scope); throw error; }
}

/** Suggestions come only from this issue's authorized projection. */
export function collaborationMentionMarkdown(kind: "member" | "agent" | "squad", id: string, name: string, source: CollaborationMentionTargets): string {
  const allowed = kind === "member" ? source.participants.some((account) => account.user_id === id) : source.targets.some((target) => target.kind === kind && target.resource_id === id);
  if (!allowed || !z.uuid().safeParse(id).success) throw new CollaborationUnavailableError();
  const label = name.replace(/[\\[\]\r\n]/g, " ");
  return `[@${label}](mention://${kind}/${id})`;
}
