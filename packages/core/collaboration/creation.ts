import { z } from "zod";
import { queryOptions, type QueryClient } from "@tanstack/react-query";
import { ApiError, getApi } from "../api";
import type { StorageAdapter } from "../types/storage";
import { collaborationKeys, CollaborationUnavailableError, isCollaborationDenied, isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope } from "./access";
import { CollaborationRecipientNotFoundError } from "./queries";
import { CollaborationCreationDraftSchema, finishCollaborationDeliveryDraft, removeCollaborationDeliveryRecord, saveCollaborationDeliveryRecord,
  type CollaborationCreationDraft, type CollaborationDeliveryForm } from "./delivery-draft";
import { collaborationDeliveryKey, protectCollaborationDelivery, reconcileCollaborationCreationSources } from "./delivery-access";
import { CollaborationCreationRejectionCodeSchema, type CollaborationCreationRejectionCode } from "./creation-schema";
import type { CollaborationIssue } from "./schema";

export class CollaborationCreationRejectedError extends Error {
  constructor(readonly code: CollaborationCreationRejectionCode) { super("The original creation was rejected"); this.name = "CollaborationCreationRejectedError"; }
}

export function collaborationCreationOptions(client: QueryClient, scope: CollaborationScope, sourceId?: string) {
  const context = sourceId ? { kind: "creation" as const, sourceWorkspaceId: sourceId } : null;
  return queryOptions({
    queryKey: context ? [...collaborationDeliveryKey(scope, context), "options"] : [...collaborationKeys.scope(scope), "creation-options"],
    enabled: scope.userId !== "", staleTime: 0, retry: false, refetchOnMount: "always", refetchOnWindowFocus: "always", refetchInterval: 15_000,
    queryFn: async ({ signal }) => {
      const load = () => getApi().getCollaborationCreationOptions(sourceId, { signal });
      const result = context ? await protectCollaborationDelivery(client, scope, context, load, true)
        : await protectCollaborationRead(client, scope, null, load);
      reconcileCollaborationCreationSources(client, scope, result.sources.map((source) => source.id));
      return result;
    },
  });
}
export function collaborationCreationRecipientOptions(client: QueryClient, scope: CollaborationScope, sourceId: string, value: string) {
  const exact = value.trim();
  const kind = z.uuid().safeParse(exact).success ? "user_id" : exact.includes("@") ? "email" : "username";
  const context = { kind: "creation" as const, sourceWorkspaceId: sourceId };
  return queryOptions({
    queryKey: [...collaborationDeliveryKey(scope, context), "recipient", kind, exact], enabled: exact !== "", staleTime: 0, retry: false,
    queryFn: ({ signal }) => protectCollaborationDelivery(client, scope, context, async () => {
      try { return await getApi().resolveCollaborationRecipient({ sourceWorkspaceId: sourceId, kind, value: exact }, { signal }); }
      catch (error) {
        if (error instanceof ApiError && error.status === 404 && z.object({ code: z.literal("recipient_not_found") }).safeParse(error.body).success) throw new CollaborationRecipientNotFoundError();
        throw error;
      }
    }),
  });
}
export function prepareCollaborationCreation(form: CollaborationDeliveryForm, requestId: string): CollaborationCreationDraft {
  if (form.scope.context.kind !== "creation" || form.purpose !== "creation") throw new CollaborationUnavailableError();
  return CollaborationCreationDraftSchema.parse({ kind: "creation", scope: form.scope, draftId: form.draftId, requestId, createdAt: Date.now(),
    input: { source_workspace_id: form.scope.context.sourceWorkspaceId, title: form.title, description: form.content,
      handler_user_id: form.recipient?.user_id ?? form.scope.userId,
      ...(form.selectedAttachmentIds.length ? { draft_id: form.draftId, attachment_ids: form.selectedAttachmentIds } : {}) } });
}

export async function executeCollaborationCreation(client: QueryClient, scope: CollaborationScope, value: CollaborationCreationDraft, recovery = false, storage?: StorageAdapter) {
  const draft = CollaborationCreationDraftSchema.parse(value);
  if (draft.scope.server !== scope.server || draft.scope.userId !== scope.userId.toLowerCase()) throw new CollaborationUnavailableError();
  if (!recovery && draft.input === null) throw new CollaborationUnavailableError();
  type Outcome = { kind: "created"; issue: CollaborationIssue } | { kind: "rejected"; code: CollaborationCreationRejectionCode };
  const sourceId = draft.scope.context.sourceWorkspaceId;
  const request = async (assertCurrent: () => void): Promise<Outcome | null> => {
    saveCollaborationDeliveryRecord(draft, storage);
    const rejection = async (): Promise<Outcome | null> => {
      assertCurrent();
      const result = await getApi().getCollaborationCreationResult(sourceId, draft.requestId);
      assertCurrent();
      return result?.outcome === "rejected" && result.actor_user_id === scope.userId.toLowerCase() ? { kind: "rejected", code: result.code } : null;
    };
    let issue: CollaborationIssue | null;
    if (recovery) {
      try { issue = await getApi().getCollaborationCreation(sourceId, draft.requestId); }
      catch (error) {
        assertCurrent();
        if (error instanceof ApiError && error.status === 401 || error instanceof Error && error.name === "AbortError") throw error;
        return rejection();
      }
      if (issue === null) return rejection();
    } else {
      try { issue = await getApi().createCollaborationIssue(draft.requestId, draft.input!); }
      catch (error) {
        assertCurrent();
        if (error instanceof ApiError && z.object({ code: CollaborationCreationRejectionCodeSchema }).safeParse(error.body).success) {
          const result = await rejection();
          if (result) return result;
          // Recipient/file/business refusals do not prove source access loss.
          // Without a durable result, keep the original request unchanged.
          await getApi().getCollaborationCreationOptions(sourceId);
          throw new CollaborationUnavailableError();
        }
        throw error;
      }
    }
    return issue && issue.creator.user_id.toLowerCase() === scope.userId.toLowerCase() ? { kind: "created", issue } : null;
  };
  // The original creator may retain issue access after leaving the source.
  const result = recovery ? await protectCollaborationRead(client, scope, null, () => request(() => {
    if (!isCollaborationScopeCurrent(scope)) throw new DOMException("Collaboration account changed", "AbortError");
  }))
    : await protectCollaborationDelivery(client, scope, draft.scope.context, request, false, storage);
  if (result.kind === "rejected") {
    try {
      await protectCollaborationDelivery(client, scope, draft.scope.context, () => getApi().getCollaborationCreationOptions(sourceId), true, storage);
    } catch (error) {
      if (isCollaborationDenied(error)) removeCollaborationDeliveryRecord(draft, storage);
      throw error;
    }
    removeCollaborationDeliveryRecord(draft, storage);
    void client.invalidateQueries({ queryKey: collaborationDeliveryKey(scope, draft.scope.context) });
    throw new CollaborationCreationRejectedError(result.code);
  }
  finishCollaborationDeliveryDraft(draft.scope, draft.draftId, storage);
  void client.invalidateQueries({ queryKey: collaborationKeys.scope(scope) });
  return result.issue;
}
