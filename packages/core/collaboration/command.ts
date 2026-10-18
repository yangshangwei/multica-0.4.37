import { z } from "zod";
import type { QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import { collaborationIssueAccessDenied, collaborationKeys, CollaborationUnavailableError, isCollaborationDenied, protectCollaborationRead, type CollaborationScope } from "./access";
import { clearCollaborationDrafts, removeCollaborationDraft, saveCollaborationDraft, type CollaborationDraft } from "./draft";
import type { CollaborationIssue } from "./schema";
import { CollaborationCommandCancelInputSchema } from "./command-schema";

const uuid = z.uuid().transform((value) => value.toLowerCase());
const payload = z.strictObject({ domain: z.literal("cancel_command"), commandId: uuid, input: CollaborationCommandCancelInputSchema });
export function parseCollaborationCommandCancelDraft(draft: CollaborationDraft) {
  const parsed = payload.safeParse(draft.payload);
  if (draft.kind !== "operation" || !parsed.success) throw new CollaborationUnavailableError();
  return parsed.data;
}
export function prepareCollaborationCommandCancel(scope: CollaborationScope, issue: CollaborationIssue, requestId: string, commandId: string): CollaborationDraft {
  const command = issue.execution?.commands.find((item) => item.id === commandId && item.state === "waiting");
  if (!command || !issue.write_context || ![command.author_user_id, issue.creator.user_id, issue.handler.user_id].includes(scope.userId)) throw new CollaborationUnavailableError();
  const input = CollaborationCommandCancelInputSchema.parse({ expected_lifecycle_epoch: issue.lifecycle_epoch, expected_participant_epoch: issue.write_context.participant_epoch,
    expected_account_epoch: issue.write_context.account_epoch, expected_collaboration_revision: issue.collaboration_revision, target_id: command.target_id });
  return { scope: { server: scope.server, userId: scope.userId, issueId: issue.id }, requestId, kind: "operation", payload: { domain: "cancel_command", commandId, input }, createdAt: Date.now() };
}
export async function executeCollaborationCommandCancel(client: QueryClient, scope: CollaborationScope, source: CollaborationDraft, recovery = false) {
  const data = parseCollaborationCommandCancelDraft(source);
  const draft = { ...source, scope: { ...source.scope }, payload: data };
  if (draft.scope.server !== scope.server || draft.scope.userId !== scope.userId) throw new CollaborationUnavailableError();
  const denied = collaborationIssueAccessDenied(client, scope, draft.scope.issueId);
  try {
    const receipt = await protectCollaborationRead(client, scope, recovery && denied ? null : draft.scope.issueId, async () => {
      if (!denied) saveCollaborationDraft(draft);
      const api = getApi();
      const result = recovery ? await api.getCollaborationCommandCancelOperation(draft.scope.issueId, draft.requestId, data.commandId, data.input)
        : await api.cancelCollaborationWaitingCommand(draft.scope.issueId, data.commandId, draft.requestId, data.input);
      if (denied && result?.redacted !== true) throw new CollaborationUnavailableError();
      return result;
    });
    removeCollaborationDraft(draft);
    if (!denied) void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, draft.scope.issueId) });
    return receipt;
  } catch (error) { if (isCollaborationDenied(error)) clearCollaborationDrafts(undefined, draft.scope); throw error; }
}
