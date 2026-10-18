import { useSyncExternalStore } from "react";
import { z } from "zod";
import { parseCollaborationActionDraft } from "./action";
import { parseCollaborationCommentDraft } from "./comment";
import { parseCollaborationCommandCancelDraft } from "./command";
import { parseCollaborationControlDraft } from "./control";
import { parseCollaborationSwitchDraft } from "./switch";
import { collaborationDraftRevision, CollaborationDraftError, readCollaborationDrafts, subscribeCollaborationDrafts, type CollaborationDraft, type CollaborationDraftScope } from "./draft";
import type { StorageAdapter } from "../types/storage";

export function readCollaborationPendingDrafts(scope: CollaborationDraftScope, storage?: StorageAdapter) {
  const result = { actions: [] as CollaborationDraft[], comments: [] as CollaborationDraft[], execution: [] as CollaborationDraft[], error: false };
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
        case "execution_control": parseCollaborationControlDraft(draft); result.execution.push(draft); break;
        case "target_switch": parseCollaborationSwitchDraft(draft); result.execution.push(draft); break;
        case "cancel_command": parseCollaborationCommandCancelDraft(draft); result.execution.push(draft); break;
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
