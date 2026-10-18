import { z } from "zod";
import type { QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import type { StorageAdapter } from "../types/storage";
import type { CollaborationIssue } from "./schema";
import { CollaborationControlInputSchema, type CollaborationControlInput } from "./control-schema";
import { collaborationAccessEpoch, collaborationKeys, CollaborationUnavailableError, isCollaborationDenied, isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope } from "./access";
import { clearCollaborationDrafts, removeCollaborationDraft, saveCollaborationDraft, type CollaborationDraft } from "./draft";

const payloadSchema = z.strictObject({ domain: z.literal("execution_control"), requestId: z.uuid(), input: CollaborationControlInputSchema });
type Selection = { scope: "issue" } | { scope: "target"; targetId: string };

/** Keep authority, hold and gate observations from the same authorized read. */
export function prepareCollaborationControl(
  scope: CollaborationScope, issue: CollaborationIssue, requestId: string,
  action: "stop" | "resume", selection: Selection,
): CollaborationDraft {
  if (!issue.execution || !issue.write_context || !isCollaborationScopeCurrent(scope)) throw new CollaborationUnavailableError();
  const execution = issue.execution;
  const selected = selection.scope === "issue" ? execution.targets : execution.targets.filter((target) => target.id === selection.targetId.toLowerCase());
  const parsed = CollaborationControlInputSchema.safeParse({
    action, scope: selection.scope,
    expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch,
    expected_account_epoch: issue.write_context.account_epoch,
    ...(selection.scope === "issue" ? { expected_control_version: execution.control.control_version, expected_target_set_version: execution.target_set_version } : {}),
    ...(action === "resume" ? { expected_execution_epoch: execution.execution_gate.epoch } : {}),
    targets: selected.map((target) => ({
      target_id: target.id, expected_target_epoch: target.target_epoch, expected_config_version: target.config_version,
      expected_control_version: target.control.control_version, observed_group_id: target.active_group_id, observed_group_generation: target.group_generation,
      ...(action === "resume" ? { expected_hold_version: target.hold_version } : {}),
    })),
  });
  if (!parsed.success || !z.uuid().safeParse(requestId).success) throw new CollaborationUnavailableError();
  return { scope: { server: scope.server, userId: scope.userId, issueId: issue.id }, requestId, kind: "operation",
    payload: { domain: "execution_control", requestId, input: parsed.data }, createdAt: Date.now() };
}

export function parseCollaborationControlDraft(draft: CollaborationDraft): CollaborationControlInput {
  const parsed = payloadSchema.safeParse(draft.payload);
  if (draft.kind !== "operation" || !parsed.success || parsed.data.requestId !== draft.requestId) throw new CollaborationUnavailableError();
  return parsed.data.input;
}

interface ControlOptions { storage?: StorageAdapter; signal?: AbortSignal }

async function controlRequest(client: QueryClient, scope: CollaborationScope, sourceDraft: CollaborationDraft, recovery: boolean, options?: ControlOptions) {
  const input = parseCollaborationControlDraft(sourceDraft);
  const draft: CollaborationDraft = { ...sourceDraft, scope: { ...sourceDraft.scope }, payload: { domain: "execution_control", requestId: sourceDraft.requestId, input } };
  if (draft.scope.server !== scope.server || draft.scope.userId.toLowerCase() !== scope.userId.toLowerCase()) throw new CollaborationUnavailableError();
  const epoch = collaborationAccessEpoch(client, scope, draft.scope.issueId);
  try {
    const result = await protectCollaborationRead(client, scope, draft.scope.issueId, async () => {
      options?.signal?.throwIfAborted();
      saveCollaborationDraft(draft, options?.storage);
      const api = getApi();
      return recovery
        ? api.getCollaborationControlOperation(draft.scope.issueId, draft.requestId, input, { signal: options?.signal })
        : api.controlCollaborationIssue(draft.scope.issueId, draft.requestId, input, { signal: options?.signal });
    });
    if (!isCollaborationScopeCurrent(scope) || collaborationAccessEpoch(client, scope, draft.scope.issueId) !== epoch) {
      throw new DOMException("Collaboration access changed", "AbortError");
    }
    removeCollaborationDraft(draft, options?.storage);
    // The receipt is historical; re-read state rather than projecting it into
    // the live channel/hold cache, especially after operation recovery.
    void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, draft.scope.issueId) });
    return result;
  } catch (error) {
    if (isCollaborationDenied(error)) clearCollaborationDrafts(options?.storage, draft.scope);
    throw error;
  }
}

export const sendCollaborationControl = (client: QueryClient, scope: CollaborationScope, draft: CollaborationDraft, options?: ControlOptions) => controlRequest(client, scope, draft, false, options);

/** A missing/unknown outcome keeps its original draft; recovery never sends POST. */
export const recoverCollaborationControl = (client: QueryClient, scope: CollaborationScope, draft: CollaborationDraft, options?: ControlOptions) => controlRequest(client, scope, draft, true, options);
