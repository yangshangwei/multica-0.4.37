import { z } from "zod";
import { queryOptions, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import type { StorageAdapter } from "../types/storage";
import type { CollaborationIssue } from "./schema";
import { CollaborationSwitchInputSchema, type CollaborationSwitchInput } from "./switch-schema";
import { collaborationAccessEpoch, collaborationIssueAccessDenied, collaborationKeys, CollaborationUnavailableError, isCollaborationDenied,
  isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope } from "./access";
import { clearCollaborationDrafts, removeCollaborationDraft, saveCollaborationDraft, type CollaborationDraft } from "./draft";

const payloadSchema = z.strictObject({ domain: z.literal("target_switch"), requestId: z.uuid(), input: CollaborationSwitchInputSchema });
export type CollaborationSwitchSelection = { targetId: string } & (
  | { action: "propose_target_switch"; roster: Extract<CollaborationSwitchInput, { action: "propose_target_switch" }>["roster"] }
  | { action: "consent_target_switch"; proposalId: string; agentId: string; identity: Extract<CollaborationSwitchInput, { action: "consent_target_switch" }>["identity"] }
  | { action: "revoke_target_switch_consent"; proposalId: string; agentId: string }
  | { action: "commit_target_switch"; proposalId: string }
);

/** Capture authority and control observations together; consent never changes
 * live configuration and commit never implies Resume. */
export function prepareCollaborationSwitch(
  scope: CollaborationScope, issue: CollaborationIssue, requestId: string, selection: CollaborationSwitchSelection,
): CollaborationDraft {
  if (!issue.execution || !issue.write_context || !isCollaborationScopeCurrent(scope)) throw new CollaborationUnavailableError();
  const target = issue.execution.targets.find((candidate) => candidate.id === selection.targetId.toLowerCase());
  if (!target) throw new CollaborationUnavailableError();
  const base = { action: selection.action, target_id: target.id, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
  let input: unknown;
  switch (selection.action) {
    case "propose_target_switch":
      input = { ...base, expected_target_epoch: target.target_epoch, expected_config_version: target.config_version, roster: selection.roster };
      break;
    case "consent_target_switch":
      input = { ...base, proposal_id: selection.proposalId, agent_id: selection.agentId, identity: selection.identity };
      break;
    case "revoke_target_switch_consent":
      input = { ...base, proposal_id: selection.proposalId, agent_id: selection.agentId };
      break;
    case "commit_target_switch":
      input = { ...base, proposal_id: selection.proposalId, control: {
        target_id: target.id, expected_target_epoch: target.target_epoch, expected_config_version: target.config_version,
        expected_hold_version: target.hold_version, expected_control_version: target.control.control_version,
        observed_group_id: target.active_group_id, observed_group_generation: target.group_generation,
      } };
      break;
    default: throw new CollaborationUnavailableError();
  }
  const parsed = CollaborationSwitchInputSchema.safeParse(input);
  if (!parsed.success || !z.uuid().safeParse(requestId).success) throw new CollaborationUnavailableError();
  return { scope: { server: scope.server, userId: scope.userId, issueId: issue.id }, requestId, kind: "operation",
    payload: { domain: "target_switch", requestId, input: parsed.data }, createdAt: Date.now() };
}

export function parseCollaborationSwitchDraft(draft: CollaborationDraft): CollaborationSwitchInput {
  const parsed = payloadSchema.safeParse(draft.payload);
  if (draft.kind !== "operation" || !parsed.success || parsed.data.requestId !== draft.requestId) throw new CollaborationUnavailableError();
  return parsed.data.input;
}

interface SwitchOptions { storage?: StorageAdapter; signal?: AbortSignal }

async function switchRequest(client: QueryClient, scope: CollaborationScope, sourceDraft: CollaborationDraft, recovery: boolean, options?: SwitchOptions) {
  const input = parseCollaborationSwitchDraft(sourceDraft);
  const draft: CollaborationDraft = { ...sourceDraft, scope: { ...sourceDraft.scope }, payload: { domain: "target_switch", requestId: sourceDraft.requestId, input } };
  if (draft.scope.server !== scope.server || draft.scope.userId.toLowerCase() !== scope.userId.toLowerCase()) throw new CollaborationUnavailableError();
  const epoch = collaborationAccessEpoch(client, scope, draft.scope.issueId);
  const denied = collaborationIssueAccessDenied(client, scope, draft.scope.issueId);
  try {
    const result = await protectCollaborationRead(client, scope, recovery && denied ? null : draft.scope.issueId, async () => {
      options?.signal?.throwIfAborted();
      if (!denied) saveCollaborationDraft(draft, options?.storage);
      const api = getApi();
      if (!recovery) return api.switchCollaborationTarget(draft.scope.issueId, draft.requestId, input, { signal: options?.signal });
      const receipt = await api.getCollaborationSwitchOperation(draft.scope.issueId, draft.requestId, input, { signal: options?.signal });
      // Access loss permits only a content-free, explicitly tagged GET result.
      if (denied && receipt?.redacted !== true) throw new CollaborationUnavailableError();
      return receipt;
    });
    if (!isCollaborationScopeCurrent(scope) || collaborationAccessEpoch(client, scope, draft.scope.issueId) !== epoch) {
      throw new DOMException("Collaboration access changed", "AbortError");
    }
    removeCollaborationDraft(draft, options?.storage);
    // Receipts describe the original operation, including old refusals. Both
    // the live execution state and proposal consent state need a fresh read.
    if (!denied) void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, draft.scope.issueId) });
    return result;
  } catch (error) {
    if (isCollaborationDenied(error)) clearCollaborationDrafts(options?.storage, draft.scope);
    throw error;
  }
}

export const sendCollaborationSwitch = (client: QueryClient, scope: CollaborationScope, draft: CollaborationDraft, options?: SwitchOptions) => switchRequest(client, scope, draft, false, options);

/** A missing, incomplete or malformed receipt retains the original draft.
 * Recovery only looks up that key; it cannot post a replacement intent. */
export const recoverCollaborationSwitch = (client: QueryClient, scope: CollaborationScope, draft: CollaborationDraft, options?: SwitchOptions) => switchRequest(client, scope, draft, true, options);

export function collaborationSwitchProposalsOptions(client: QueryClient, scope: CollaborationScope, issueId: string, targetId: string) {
  return queryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "switch-proposals", targetId.toLowerCase()],
    staleTime: 0, retry: false,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().listCollaborationTargetSwitchProposals(issueId, targetId, { signal })),
  });
}

export function collaborationSwitchProposalOptions(client: QueryClient, scope: CollaborationScope, issueId: string, targetId: string, proposalId: string) {
  return queryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "switch-proposals", targetId.toLowerCase(), proposalId.toLowerCase()],
    staleTime: 0, retry: false,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationTargetSwitchProposal(issueId, targetId, proposalId, { signal })),
  });
}
