import { useSyncExternalStore } from "react";
import { z } from "zod";
import { parseCollaborationActionDraft } from "./action";
import { parseCollaborationCommentDraft } from "./comment";
import { parseCollaborationCommentChangeDraft } from "./comment-actions";
import { parseCollaborationDiscussionDraft } from "./discussion";
import { parseCollaborationCommandCancelDraft } from "./command";
import { parseCollaborationControlDraft } from "./control";
import { parseCollaborationSwitchDraft } from "./switch";
import { parseCollaborationResourceDraft } from "./resource";
import { parseCollaborationMemoryDraft } from "./memory-actions";
import { collaborationDraftRevision, CollaborationDraftError, readCollaborationDrafts, subscribeCollaborationDrafts, type CollaborationDraft, type CollaborationDraftScope } from "./draft";
import type { StorageAdapter } from "../types/storage";

export function readCollaborationPendingDrafts(scope: CollaborationDraftScope, storage?: StorageAdapter) {
  const result = { actions: [] as CollaborationDraft[], comments: [] as CollaborationDraft[], commentChanges: [] as CollaborationDraft[], discussion: [] as CollaborationDraft[], execution: [] as CollaborationDraft[], memory: [] as CollaborationDraft[], error: false };
  try {
    for (const draft of readCollaborationDrafts(scope, storage)) {
      if (draft.kind === "comment") {
        parseCollaborationCommentDraft(draft);
        result.comments.push(draft);
        continue;
      }
      const domain = z.object({ domain: z.string().optional() }).safeParse(draft.payload);
      if (!domain.success) throw new CollaborationDraftError();
      switch (domain.data.domain) {
        case "comment_change": parseCollaborationCommentChangeDraft(draft); result.commentChanges.push(draft); break;
        case "discussion": parseCollaborationDiscussionDraft(draft); result.discussion.push(draft); break;
        case "execution_control": parseCollaborationControlDraft(draft); result.execution.push(draft); break;
        case "target_switch": parseCollaborationSwitchDraft(draft); result.execution.push(draft); break;
        case "target_resource": parseCollaborationResourceDraft(draft); result.execution.push(draft); break;
        case "cancel_command": parseCollaborationCommandCancelDraft(draft); result.execution.push(draft); break;
        case "memory": parseCollaborationMemoryDraft(draft); result.memory.push(draft); break;
        case undefined: parseCollaborationActionDraft(draft); result.actions.push(draft); break;
        default: throw new CollaborationDraftError();
      }
    }
  } catch {
    // A corrupt/unknown saved write must not disappear and unlock a replacement.
    result.error = true;
  }
  return result;
}

export function useCollaborationPendingDrafts(scope: CollaborationDraftScope) {
  useSyncExternalStore(subscribeCollaborationDrafts, collaborationDraftRevision, () => 0);
  return readCollaborationPendingDrafts(scope);
}
