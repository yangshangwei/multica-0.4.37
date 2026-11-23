import { getApi } from "../api";
import { useAuthStore } from "../auth";
import type { Issue } from "../types";
import type { CompletionDraft, CompletionDraftScope } from "./completion-draft";
import { CompletionDraftScopeSchema } from "./completion-draft";
import type { CompletionAuthorization, CompletionPreview } from "./completion-schema";
import { CompletionPreviewSchema } from "./completion-schema";

export type CompletionScope = CompletionDraftScope & { apiScope: string };
export type CompletionResult = {
  outcome: "authorized" | "consumed" | "revoked" | "refused" | "unknown";
  draft: CompletionDraft; authorization: CompletionAuthorization | null; issue: Issue | null;
  currentIssueRead: "ok" | "denied" | "unknown" | "not_requested"; reason: string | null; submittedNow: boolean;
};
const instances = new WeakMap<object, number>();
let nextInstance = 0;
export function deliveryCompletionScope(userId: string, workspaceId: string, issueId: string): CompletionScope {
  const api = getApi();
  let instance = instances.get(api);
  if (instance === undefined) { instance = ++nextInstance; instances.set(api, instance); }
  const scope = CompletionDraftScopeSchema.parse({ server: api.getBaseUrl(), userId, workspaceId, issueId });
  return { ...scope, apiScope: JSON.stringify([instance, api.getSessionScope()]) };
}
export function isCompletionScopeCurrent(scope: CompletionScope): boolean {
  return useAuthStore.getState()?.user?.id === scope.userId && deliveryCompletionScope(scope.userId, scope.workspaceId, scope.issueId).apiScope === scope.apiScope && getApi().getBaseUrl() === scope.server;
}
export function completionDraftScope(scope: CompletionScope): CompletionDraftScope {
  return { server: scope.server, userId: scope.userId, workspaceId: scope.workspaceId, issueId: scope.issueId };
}
export function assertCompletionScope(scope: CompletionScope, signal?: AbortSignal): void {
  signal?.throwIfAborted();
  if (!isCompletionScopeCurrent(scope)) throw new DOMException("Completion session changed", "AbortError");
}
export function validateCompletionPreview(scope: CompletionScope, value: CompletionPreview, expiresAt: string): CompletionPreview {
  const preview = CompletionPreviewSchema.parse(value);
  const b = preview.authorization_binding;
  if (!b || preview.issue_id !== scope.issueId || b.source_workspace_id !== scope.workspaceId || b.consumer.user_id !== scope.userId || !preview.max_expires_at ||
    !Number.isFinite(Date.parse(expiresAt)) || Date.parse(expiresAt) <= Date.now() || Date.parse(expiresAt) > Date.parse(preview.max_expires_at)) throw new Error("Completion evidence is not available for this authorization");
  return preview;
}
export function completionOutcome(draft: CompletionDraft, authorization: CompletionAuthorization): Pick<CompletionResult, "outcome" | "reason"> {
  if (authorization.state === "consumed") return { outcome: "consumed", reason: null };
  if (authorization.state === "revoked") return { outcome: "revoked", reason: null };
  if (draft.submissionDispatched || draft.revocation?.dispatched) return { outcome: "unknown", reason: "completion_outcome_unknown" };
  if (authorization.applicability === "expired" || Date.parse(authorization.expires_at) <= Date.now()) return { outcome: "refused", reason: "delivery_completion_expiry_invalid" };
  return { outcome: "authorized", reason: null };
}
