import { invalidateCollaborationSourceViews } from "./invalidation";
import { z } from "zod";
import { queryOptions, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import type { StorageAdapter } from "../types/storage";
import type { CollaborationIssue } from "./schema";
import { collaborationAccessEpoch, collaborationIssueAccessDenied, collaborationKeys, CollaborationUnavailableError, isCollaborationDenied,
  isCollaborationScopeCurrent, protectCollaborationRead, type CollaborationScope } from "./access";
import { clearCollaborationDrafts, removeCollaborationDraft, saveCollaborationDraft, type CollaborationDraft } from "./draft";
import { CollaborationResourceInputSchema, type CollaborationResourceInput, type CollaborationResources, type CollaborationResourceOption,
  type CollaborationResourceIdentityView, type CollaborationTargetResource } from "./resource-schema";

const payloadSchema = z.strictObject({ domain: z.literal("target_resource"), requestId: z.uuid(), input: CollaborationResourceInputSchema });
export type CollaborationResourceSelection =
  | { action: "register_target"; candidate: CollaborationResourceOption }
  | { action: "consent_resource"; targetId: string; agentId: string; identity: CollaborationResourceIdentityView }
  | { action: "revoke_grant"; targetId: string; agentId: string };

export function canOrganizeCollaborationResources(scope: CollaborationScope, issue: CollaborationIssue): boolean {
  return isCollaborationScopeCurrent(scope) && issue.write_context != null && issue.execution !== null && issue.capabilities.control_ai === true &&
    issue.capabilities.invite === true && [issue.creator.user_id, issue.handler.user_id].includes(scope.userId) &&
    ["backlog", "todo", "in_progress", "in_review", "blocked"].includes(issue.status_category) && issue.session.queue_state !== "closed";
}

export function collaborationResourceRoster(candidate: { resources: CollaborationTargetResource[] }): CollaborationTargetResource[] {
  return candidate.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) => ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }));
}

export function prepareCollaborationResource(scope: CollaborationScope, issue: CollaborationIssue, resources: CollaborationResources, requestId: string, selection: CollaborationResourceSelection): CollaborationDraft {
  if (!isCollaborationScopeCurrent(scope) || !issue.write_context || issue.capabilities.control_ai !== true || !issue.execution ||
    resources.issue_id !== issue.id.toLowerCase() || resources.lifecycle_epoch !== issue.lifecycle_epoch || resources.target_set_version !== issue.execution.target_set_version) throw new CollaborationUnavailableError();
  const base = { expected_lifecycle_epoch: issue.lifecycle_epoch, expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
  let input: unknown;
  if (selection.action === "register_target") {
    if (!canOrganizeCollaborationResources(scope, issue) || resources.can_organize !== true || selection.candidate.available !== true) throw new CollaborationUnavailableError();
    input = { ...base, action: selection.action, kind: selection.candidate.kind, resource_id: selection.candidate.resource_id,
      expected_target_set_version: resources.target_set_version, roster: collaborationResourceRoster(selection.candidate) };
  } else {
    const target = resources.targets.find((item) => item.target_id === selection.targetId);
    const current = issue.execution.targets.find((item) => item.id === selection.targetId);
    const resource = target?.resources.find((item) => item.agent_id === selection.agentId);
    if (!target || !current || !resource || resource.owner_user_id !== scope.userId || current.target_epoch !== target.target_epoch || current.config_version !== target.config_version) throw new CollaborationUnavailableError();
    if (selection.action === "revoke_grant") {
      if (resource.can_revoke !== true || !resource.grant_id || !resource.grant_epoch) throw new CollaborationUnavailableError();
      input = { ...base, action: selection.action, grant_id: resource.grant_id, expected_grant_epoch: resource.grant_epoch };
    } else {
      const identity = selection.identity;
      if (resource.can_consent !== true || issue.session.queue_state === "closed" || identity.issue_id !== issue.id.toLowerCase() || identity.target_id !== target.target_id ||
        identity.agent_id !== resource.agent_id || identity.proposal_id !== null || !identity.identity) throw new CollaborationUnavailableError();
      input = { ...base, action: selection.action, target_id: target.target_id, agent_id: resource.agent_id, expected_target_epoch: target.target_epoch,
        expected_config_version: target.config_version, identity: identity.identity };
    }
  }
  const parsed = CollaborationResourceInputSchema.safeParse(input);
  if (!parsed.success || !z.uuid().safeParse(requestId).success) throw new CollaborationUnavailableError();
  return { scope: { server: scope.server, userId: scope.userId, issueId: issue.id }, requestId, kind: "operation", createdAt: Date.now(),
    payload: { domain: "target_resource", requestId, input: parsed.data } };
}

export function parseCollaborationResourceDraft(draft: CollaborationDraft): CollaborationResourceInput {
  const parsed = payloadSchema.safeParse(draft.payload);
  if (draft.kind !== "operation" || !parsed.success || parsed.data.requestId !== draft.requestId) throw new CollaborationUnavailableError();
  return parsed.data.input;
}

export async function executeCollaborationResource(client: QueryClient, scope: CollaborationScope, source: CollaborationDraft, recovery = false, options?: { storage?: StorageAdapter; signal?: AbortSignal }) {
  const input = parseCollaborationResourceDraft(source);
  const draft: CollaborationDraft = { ...source, scope: { ...source.scope }, payload: { domain: "target_resource", requestId: source.requestId, input } };
  if (draft.scope.server !== scope.server || draft.scope.userId !== scope.userId) throw new CollaborationUnavailableError();
  const epoch = collaborationAccessEpoch(client, scope, draft.scope.issueId);
  const denied = collaborationIssueAccessDenied(client, scope, draft.scope.issueId);
  try {
    const result = await protectCollaborationRead(client, scope, recovery && denied ? null : draft.scope.issueId, async () => {
      options?.signal?.throwIfAborted();
      if (!denied) saveCollaborationDraft(draft, options?.storage);
      const receipt = await getApi().writeCollaborationResource(draft.scope.issueId, draft.requestId, input, recovery, options);
      if (denied && receipt?.redacted !== true) throw new CollaborationUnavailableError();
      return receipt;
    });
    if (!isCollaborationScopeCurrent(scope) || collaborationAccessEpoch(client, scope, draft.scope.issueId) !== epoch) throw new DOMException("Collaboration access changed", "AbortError");
    removeCollaborationDraft(draft, options?.storage);
    if (!denied) {
      invalidateCollaborationSourceViews(client, scope, draft.scope.issueId);
      void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, draft.scope.issueId) });
    }
    return result;
  } catch (error) {
    if (isCollaborationDenied(error)) clearCollaborationDrafts(options?.storage, draft.scope);
    throw error;
  }
}

export function collaborationResourcesOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  return queryOptions({ queryKey: [...collaborationKeys.issue(scope, issueId), "resources"], staleTime: 0, retry: false,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationResources(issueId, { signal })) });
}
export function collaborationResourceWorkspacesOptions(client: QueryClient, scope: CollaborationScope) {
  return queryOptions({ queryKey: [...collaborationKeys.scope(scope), "resource-workspaces"], staleTime: 0, retry: false,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, null, () => getApi().listWorkspaces({ signal })) });
}
export function collaborationResourceOptionsOptions(client: QueryClient, scope: CollaborationScope, issueId: string, workspaceId: string) {
  return queryOptions({ queryKey: [...collaborationKeys.issue(scope, issueId), "resource-options", workspaceId.toLowerCase()], staleTime: 0, retry: false, enabled: z.uuid().safeParse(workspaceId).success,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationResourceOptions(issueId, workspaceId, { signal })) });
}
export function collaborationResourceIdentityOptions(client: QueryClient, scope: CollaborationScope, issueId: string, targetId: string, agentId: string, proposalId: string | null = null) {
  return queryOptions({ queryKey: [...collaborationKeys.issue(scope, issueId), "resource-identity", targetId, agentId, proposalId], staleTime: 0, retry: false,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, () => getApi().getCollaborationResourceIdentity(issueId, targetId, agentId, proposalId, { signal })) });
}
