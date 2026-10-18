import { z } from "zod";
import type { QueryClient } from "@tanstack/react-query";
import type { StorageAdapter } from "../types/storage";
import { getApi } from "../api";
import { CollaborationCommentInputSchema, type CollaborationMentionTargets, type CollaborationTriggerPreview } from "./content-schema";
import { collaborationKeys, CollaborationUnavailableError, isCollaborationDenied, protectCollaborationRead, type CollaborationScope } from "./access";
import { clearCollaborationDrafts, removeCollaborationDraft, saveCollaborationDraft, type CollaborationDraft } from "./draft";

const payload = z.strictObject({ domain: z.literal("create_comment"), input: CollaborationCommentInputSchema });
export function parseCollaborationCommentDraft(draft: CollaborationDraft) {
  const parsed = payload.safeParse(draft.payload);
  if (draft.kind !== "comment" || !parsed.success) throw new CollaborationUnavailableError();
  return parsed.data.input;
}
export function prepareCollaborationComment(scope: CollaborationScope, issueId: string, requestId: string, content: string, suppressed: string[], preview: CollaborationTriggerPreview): CollaborationDraft {
  if (preview.issue_id !== issueId.toLowerCase() || !z.uuid().safeParse(requestId).success) throw new CollaborationUnavailableError();
  const input = CollaborationCommentInputSchema.parse({ content, parent_id: null, suppressed_target_ids: suppressed,
    expected_lifecycle_epoch: preview.lifecycle_epoch, expected_participant_epoch: preview.write_context?.participant_epoch ?? null,
    expected_account_epoch: preview.write_context?.account_epoch ?? null,
    target_versions: preview.targets.filter((target) => target.target_id !== null && target.target_epoch !== undefined && target.config_version !== undefined).map((target) => ({ target_id: target.target_id, expected_target_epoch: target.target_epoch, expected_config_version: target.config_version })),
  });
  return { scope: { server: scope.server, userId: scope.userId, issueId }, requestId, kind: "comment", payload: { domain: "create_comment", input }, createdAt: Date.now() };
}
export async function executeCollaborationComment(client: QueryClient, scope: CollaborationScope, source: CollaborationDraft, recovery = false, storage?: StorageAdapter) {
  const input = parseCollaborationCommentDraft(source);
  const draft = { ...source, scope: { ...source.scope }, payload: { domain: "create_comment", input } };
  if (draft.scope.server !== scope.server || draft.scope.userId !== scope.userId) throw new CollaborationUnavailableError();
  try {
    const receipt = await protectCollaborationRead(client, scope, draft.scope.issueId, async () => {
      saveCollaborationDraft(draft, storage);
      return recovery ? getApi().getCollaborationCommentOperation(draft.scope.issueId, draft.requestId) : getApi().createCollaborationComment(draft.scope.issueId, draft.requestId, input);
    });
    removeCollaborationDraft(draft, storage);
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
