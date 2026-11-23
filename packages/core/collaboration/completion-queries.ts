import { queryOptions, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import { authorizeCompletionRequest, type CompletionActionOptions } from "../issues/completion-queries";
import { assertCompletionScope, type CompletionScope } from "../issues/completion";
import { CompletionAuthorizationRequestSchema, completionUUID } from "../issues/completion-schema";
import { collaborationScope } from "./access";
import { collaborationStageKeys } from "./stage-queries";
import { eraseCollaborationStageSource, protectCollaborationStageSource } from "./stage-source-cache";
import { collaborationIssueOptions } from "./queries";
import { CollaborationCompletionPreviewSchema, type CollaborationCompletionPreview, type CollaborationCompletionPreviewInput, type CollaborationCompletionRead } from "./completion-schema";

export type CollaborationCompletionScope = CompletionScope & { adminOrganizationId: string | null };
export const collaborationCompletionKeys = {
  root: (scope: CollaborationCompletionScope) => [...collaborationStageKeys.root(collaborationScope(scope.userId), scope.issueId), "completion", scope.workspaceId, scope.adminOrganizationId] as const,
  candidates: (scope: CollaborationCompletionScope, sourceId: string, cursor?: string) => [...collaborationCompletionKeys.root(scope), "candidates", sourceId, cursor ?? null] as const,
  preview: (scope: CollaborationCompletionScope, input: CollaborationCompletionPreviewInput | null) => [...collaborationCompletionKeys.root(scope), "preview", input] as const,
};
async function read<T>(client: QueryClient, scope: CollaborationCompletionScope, key: readonly unknown[], request: () => Promise<CollaborationCompletionRead<T>>) {
  assertCompletionScope(scope);
  const accountScope = collaborationScope(scope.userId);
  const result = await protectCollaborationStageSource(client, accountScope, scope.adminOrganizationId ? null : scope.issueId, scope.workspaceId, request);
  assertCompletionScope(scope);
  if (result.state === "denied") {
    eraseCollaborationStageSource(client, accountScope, scope.workspaceId, key);
    if (!scope.adminOrganizationId) await client.fetchQuery(collaborationIssueOptions(client, accountScope, scope.issueId));
  }
  return result;
}
const reads = { retry: false, staleTime: 0, gcTime: 0, refetchOnWindowFocus: true, refetchOnReconnect: true } as const;
export function collaborationCompletionCandidatesOptions(client: QueryClient, scope: CollaborationCompletionScope, sourceId: string, cursor?: string) {
  const key = collaborationCompletionKeys.candidates(scope, sourceId, cursor);
  return queryOptions({ queryKey: key, ...reads, enabled: completionUUID.safeParse(sourceId).success,
    queryFn: ({ signal }) => read(client, scope, key, () => getApi().listCollaborationCompletionCandidates(scope.issueId, sourceId, { cursor }, { admin: scope.adminOrganizationId !== null, signal })) });
}
export function collaborationCompletionPreviewOptions(client: QueryClient, scope: CollaborationCompletionScope, input: CollaborationCompletionPreviewInput | null) {
  const key = collaborationCompletionKeys.preview(scope, input);
  return queryOptions({ queryKey: key, ...reads, enabled: input !== null,
    queryFn: ({ signal }) => input ? read(client, scope, key, () => getApi().previewCollaborationCompletion(scope.issueId, input, { admin: scope.adminOrganizationId !== null, signal })) : Promise.reject(new Error("Completion target unavailable")) });
}
export function authorizeCollaborationCompletion(scope: CollaborationCompletionScope, source: CollaborationCompletionPreview,
  input: { reason: string; expiresAt: string; operationId: string }, options?: CompletionActionOptions) {
  assertCompletionScope(scope, options?.signal);
  const preview = CollaborationCompletionPreviewSchema.parse(source), binding = preview.authorization_binding;
  if (!binding || binding.target.kind === "ordinary" || binding.source_workspace_id !== scope.workspaceId || binding.target.issue_id !== scope.issueId || binding.consumer.user_id !== scope.userId ||
    (scope.adminOrganizationId === null ? binding.consumer.kind !== "human" : binding.consumer.kind !== "admin" || binding.consumer.organization_id !== scope.adminOrganizationId) ||
    !preview.max_expires_at || Date.parse(input.expiresAt) <= Date.now() || Date.parse(input.expiresAt) > Date.parse(preview.max_expires_at)) throw new Error("Completion evidence is not available for this authorization");
  return authorizeCompletionRequest(scope, CompletionAuthorizationRequestSchema.parse({ ...binding, operation_id: input.operationId, reason: input.reason, expires_at: input.expiresAt }), options);
}
