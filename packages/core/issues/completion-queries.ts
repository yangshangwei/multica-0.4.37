import { queryOptions, type QueryClient } from "@tanstack/react-query";
import { ApiError, getApi, errorCode, type ApiRequestOptions } from "../api";
import type { CompletionRequestOptions } from "../api/client";
import { defaultStorage } from "../platform/storage";
import type { StorageAdapter } from "../types/storage";
import { applyIssueChange, invalidateIssueDerivatives, invalidateStaleListKeys } from "./cache-coordinator";
import { issueKeys } from "./queries";
import { invalidateIssueOwnerProjections, reconcileIssueFullSnapshotRevision } from "./ws-updaters";
import { assertCompletionScope, completionDraftScope, completionOutcome, isCompletionScopeCurrent, validateCompletionPreview, type CompletionResult, type CompletionScope } from "./completion";
import { CompletionDraftError, readCompletionDrafts, removeCompletionDraft, saveCompletionDraft, type CompletionDraft } from "./completion-draft";
import { CompletionAuthorizationRequestSchema, completionReason, matchesCompletionAuthorization, type CompletionAuthorization, type CompletionPreview, type CompletionPreviewInput } from "./completion-schema";

export type CompletionActionOptions = ApiRequestOptions & { storage?: StorageAdapter };
export const completionKeys = {
  scope: (scope: CompletionScope) => ["delivery-completion", scope.server, scope.apiScope, scope.userId, scope.workspaceId, scope.issueId] as const,
  candidates: (scope: CompletionScope, params: { limit?: number; cursor?: string }) => [...completionKeys.scope(scope), "candidates", params] as const,
  preview: (scope: CompletionScope, input: CompletionPreviewInput) => [...completionKeys.scope(scope), "preview", input] as const,
};
const requestOptions = (scope: CompletionScope, options?: ApiRequestOptions): CompletionRequestOptions => ({ workspaceId: scope.workspaceId, signal: options?.signal, timeoutMs: options?.timeoutMs });
const denied = (error: unknown) => error instanceof ApiError && [401, 403, 404, 410].includes(error.status);

/** A denied refetch must not leave the previously readable evidence in cache. */
export function clearCompletionProjection(client: QueryClient, scope: CompletionScope): void {
  const queryKey = completionKeys.scope(scope);
  void client.cancelQueries({ queryKey }, { revert: false });
  client.removeQueries({ queryKey });
}
async function protectedRead<T>(scope: CompletionScope, signal: AbortSignal, read: () => Promise<T>, client?: QueryClient): Promise<T> {
  assertCompletionScope(scope, signal);
  try {
    const result = await read();
    assertCompletionScope(scope, signal);
    return result;
  } catch (error) {
    if (client && denied(error)) clearCompletionProjection(client, scope);
    throw error;
  }
}
export function completionCandidatesOptions(scope: CompletionScope, params: { limit?: number; cursor?: string } = {}, client?: QueryClient) {
  return queryOptions({
    queryKey: completionKeys.candidates(scope, params),
    queryFn: ({ signal }) => protectedRead(scope, signal, () => getApi().listDeliveryCompletionCandidates(scope.issueId, params, requestOptions(scope, { signal })), client),
    retry: false, staleTime: 0, gcTime: 0, refetchOnWindowFocus: false, refetchOnReconnect: false,
  });
}
export function completionPreviewOptions(scope: CompletionScope, input: CompletionPreviewInput, client?: QueryClient) {
  return queryOptions({
    queryKey: completionKeys.preview(scope, input),
    queryFn: ({ signal }) => protectedRead(scope, signal, () => getApi().previewDeliveryCompletion(scope.issueId, input, requestOptions(scope, { signal })), client),
    retry: false, staleTime: 0, gcTime: 0, refetchOnWindowFocus: false, refetchOnReconnect: false,
  });
}

const flights = new Map<string, { intent: string; result: Promise<CompletionResult> }>();
function singleFlight(scope: CompletionScope, intent: string, run: () => Promise<CompletionResult>): Promise<CompletionResult> {
  const key = JSON.stringify(completionKeys.scope(scope)), pending = flights.get(key);
  if (pending) {
    if (pending.intent !== intent) return Promise.reject(new Error("A different completion operation is pending"));
    return pending.result;
  }
  // Queue the work after registering the flight; synchronous storage is also protected.
  const result = Promise.resolve().then(run).finally(() => { if (flights.get(key)?.result === result) flights.delete(key); });
  flights.set(key, { intent, result });
  return result;
}
function loadDraft(scope: CompletionScope, draft: CompletionDraft, options?: CompletionActionOptions): CompletionDraft {
  if (JSON.stringify(completionDraftScope(scope)) !== JSON.stringify(draft.scope)) throw new CompletionDraftError();
  const stored = readCompletionDrafts(scope, options?.storage).find(item => item.requestKey === draft.requestKey);
  if (!stored || JSON.stringify(stored.request) !== JSON.stringify(draft.request)) throw new CompletionDraftError();
  return stored;
}
function baseResult(draft: CompletionDraft): CompletionResult {
  return { outcome: "unknown", draft, authorization: null, issue: null, currentIssueRead: "not_requested", reason: "completion_outcome_unknown", submittedNow: false };
}
function failureResult(draft: CompletionDraft, error: unknown, recovering: boolean): CompletionResult {
  const knownRefusal = !recovering && error instanceof ApiError && error.status >= 400 && error.status < 500 && error.status !== 408 && error.status !== 429;
  return { ...baseResult(draft), outcome: knownRefusal ? "refused" : "unknown", reason: errorCode(error) ?? (knownRefusal ? "completion_request_refused" : "completion_outcome_unknown") };
}
function acceptAuthorization(scope: CompletionScope, draft: CompletionDraft, authorization: CompletionAuthorization, options?: CompletionActionOptions): CompletionResult {
  if (!matchesCompletionAuthorization(authorization, draft.request) || (draft.authorizationId !== null && draft.authorizationId !== authorization.authorization_id)) {
    return { ...baseResult(draft), reason: "invalid_delivery_completion_response" };
  }
  assertCompletionScope(scope, options?.signal);
  const updated = { ...draft, authorizationId: authorization.authorization_id };
  saveCompletionDraft(updated, options?.storage);
  return { ...baseResult(updated), ...completionOutcome(updated, authorization), authorization };
}
async function withCurrentIssue(scope: CompletionScope, result: CompletionResult, options?: CompletionActionOptions): Promise<CompletionResult> {
  try {
    const issue = await getApi().getDeliveryCompletionIssue(scope.issueId, requestOptions(scope, options));
    assertCompletionScope(scope, options?.signal);
    return { ...result, issue, currentIssueRead: "ok" };
  } catch (error) {
    assertCompletionScope(scope, options?.signal);
    return { ...result, issue: null, currentIssueRead: denied(error) ? "denied" : "unknown" };
  }
}
async function recover(scope: CompletionScope, input: CompletionDraft, options?: CompletionActionOptions): Promise<CompletionResult> {
  assertCompletionScope(scope, options?.signal);
  const draft = loadDraft(scope, input, options);
  try {
    const api = getApi();
    const authorization = draft.authorizationId
      ? await api.getDeliveryCompletionAuthorization(draft.authorizationId, requestOptions(scope, options))
      : await api.getDeliveryCompletionAuthorizationByKey(draft.requestKey, requestOptions(scope, options));
    assertCompletionScope(scope, options?.signal);
    return withCurrentIssue(scope, acceptAuthorization(scope, draft, authorization, options), options);
  } catch (error) {
    assertCompletionScope(scope, options?.signal);
    return failureResult(draft, error, true);
  }
}

export function authorizeIssueCompletion(scope: CompletionScope, preview: CompletionPreview, input: { reason: string; expiresAt: string }, options?: CompletionActionOptions): Promise<CompletionResult> {
  return singleFlight(scope, JSON.stringify(["authorize", preview, input]), async () => {
    assertCompletionScope(scope, options?.signal);
    const reviewed = validateCompletionPreview(scope, preview, input.expiresAt);
    if (readCompletionDrafts(scope, options?.storage).length) throw new Error("Recover the original completion operation first");
    const request = CompletionAuthorizationRequestSchema.parse({ ...reviewed.authorization_binding, operation_id: crypto.randomUUID(), reason: input.reason, expires_at: input.expiresAt });
    const draft: CompletionDraft = { version: 1, scope: completionDraftScope(scope), requestKey: crypto.randomUUID(), request, createdAt: Date.now(), creationDispatched: true, authorizationId: null, submissionDispatched: false, revocation: null };
    saveCompletionDraft(draft, options?.storage);
    assertCompletionScope(scope, options?.signal);
    try {
      const authorization = await getApi().createDeliveryCompletionAuthorization(draft.requestKey, draft.request, requestOptions(scope, options));
      assertCompletionScope(scope, options?.signal);
      const result = acceptAuthorization(scope, draft, authorization, options);
      return result.outcome === "consumed" ? withCurrentIssue(scope, result, options) : result;
    } catch (error) {
      assertCompletionScope(scope, options?.signal);
      return failureResult(draft, error, false);
    }
  });
}

export function recoverIssueCompletion(scope: CompletionScope, draft: CompletionDraft, options?: CompletionActionOptions): Promise<CompletionResult> {
  return singleFlight(scope, `recover:${draft.requestKey}`, () => recover(scope, draft, options));
}

export function submitIssueCompletion(scope: CompletionScope, input: CompletionDraft, options?: CompletionActionOptions): Promise<CompletionResult> {
  return singleFlight(scope, `submit:${input.requestKey}`, async () => {
    assertCompletionScope(scope, options?.signal);
    let draft = loadDraft(scope, input, options);
    if (draft.submissionDispatched || draft.revocation?.dispatched) return recover(scope, draft, options);
    const current = await recover(scope, draft, options);
    if (current.outcome !== "authorized" || current.currentIssueRead !== "ok" || !current.draft.authorizationId) return current;
    assertCompletionScope(scope, options?.signal);
    draft = { ...current.draft, submissionDispatched: true };
    saveCompletionDraft(draft, options?.storage);
    assertCompletionScope(scope, options?.signal);
    try {
      const completed = await getApi().submitDeliveryCompletion(scope.issueId, {
        status: draft.request.target.target_status,
        completion: { authorization_id: draft.authorizationId!, operation_id: draft.request.operation_id, expected_issue_revision: draft.request.target.expected_issue_revision },
      }, requestOptions(scope, options));
      assertCompletionScope(scope, options?.signal);
      return { ...baseResult(draft), outcome: "consumed", authorization: { ...current.authorization!, state: "consumed", applicability: "consumed", completion_receipt: completed.receipt }, issue: completed.issue, currentIssueRead: "ok", reason: null, submittedNow: true };
    } catch (error) {
      assertCompletionScope(scope, options?.signal);
      return failureResult(draft, error, false);
    }
  });
}

export function revokeIssueCompletion(scope: CompletionScope, input: CompletionDraft, reason: string, options?: CompletionActionOptions): Promise<CompletionResult> {
  return singleFlight(scope, JSON.stringify(["revoke", input.requestKey, reason]), async () => {
    assertCompletionScope(scope, options?.signal);
    let draft = loadDraft(scope, input, options);
    if (draft.revocation?.dispatched) return recover(scope, draft, options);
    if (!draft.authorizationId) {
      const current = await recover(scope, draft, options);
      if (!current.draft.authorizationId || ["consumed", "revoked"].includes(current.outcome)) return current;
      draft = current.draft;
    }
    draft = { ...draft, revocation: { requestKey: crypto.randomUUID(), reason: completionReason.parse(reason), dispatched: true } };
    saveCompletionDraft(draft, options?.storage);
    assertCompletionScope(scope, options?.signal);
    try {
      const authorization = await getApi().revokeDeliveryCompletionAuthorization(draft.authorizationId!, draft.revocation!.requestKey, draft.revocation!.reason, requestOptions(scope, options));
      assertCompletionScope(scope, options?.signal);
      const result = acceptAuthorization(scope, draft, authorization, options);
      return result.outcome === "consumed" ? withCurrentIssue(scope, result, options) : result;
    } catch (error) {
      assertCompletionScope(scope, options?.signal);
      return failureResult(draft, error, false);
    }
  });
}

/** This is an explicit UI action after a known terminal or original-write refusal. */
export function retireCompletionDraft(scope: CompletionScope, result: CompletionResult, options?: CompletionActionOptions): void {
  assertCompletionScope(scope, options?.signal);
  if (result.outcome === "unknown" || result.outcome === "authorized") throw new CompletionDraftError();
  removeCompletionDraft(loadDraft(scope, result.draft, options), options?.storage ?? defaultStorage);
}

/** Reconcile the separately verified current issue, never the receipt's historical target. */
export function reconcileCompletionResult(client: QueryClient, scope: CompletionScope, result: CompletionResult): void {
  if (!isCompletionScopeCurrent(scope)) return;
  const issue = result.issue;
  if (issue && issue.id === scope.issueId && issue.workspace_id === scope.workspaceId) {
    const { properties: _properties, ...snapshot } = issue;
    const change = applyIssueChange(client, scope.workspaceId, scope.issueId, snapshot, {
      changed: { assignee: false, project: false, status: true }, baseIssue: issue,
      acceptCurrent: current => current.revision === undefined || (issue.revision !== undefined && issue.revision > current.revision),
    });
    reconcileIssueFullSnapshotRevision(client, scope.workspaceId, scope.issueId, issue.revision);
    invalidateStaleListKeys(client, change.staleKeys);
  }
  if (result.currentIssueRead === "denied") clearCompletionProjection(client, scope);
  invalidateIssueOwnerProjections(client, scope.workspaceId, scope.issueId);
  void client.invalidateQueries({ queryKey: issueKeys.detail(scope.workspaceId, scope.issueId) });
  void client.invalidateQueries({ queryKey: issueKeys.childProgress(scope.workspaceId) });
  invalidateIssueDerivatives(client, scope.workspaceId, { statusOrProjectChanged: true });
}
