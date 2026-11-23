"use client";

import { useCallback, useEffect, useState } from "react";
import { infiniteQueryOptions, queryOptions, useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, errorCode, getApi } from "../api";
import { useAuthStore } from "../auth";
import { adminApiScope, adminKeys, AdminUnsupportedError, type AdminScope } from "./queries";
import { AdminControlUncertainError } from "./operation-queries";
import {
  readAdminCollaborationDraft, removeAdminControlDraft, saveAdminControlDraft,
  type AdminCollaborationDraft, type AdminCollaborationInput, type AdminControlDraftScope,
} from "./operation-draft";
import {
  AdminCollaborationChangeSchema, matchesAdminCollaborationResult,
  type AdminCollaborationCandidates, type AdminCollaborationChange, type AdminCollaborationDetail, type AdminCollaborationResult,
} from "./collaboration-schema";

const reads = { retry: false as const, staleTime: 0, gcTime: 0, refetchOnWindowFocus: false };
const pageSize = 50;

export class AdminCollaborationStaleError extends Error {
  constructor() { super("The collaboration confirmation is no longer current"); this.name = "AdminCollaborationStaleError"; }
}
export class AdminCollaborationRejectedError extends Error {
  constructor(readonly response: ApiError) { super(response.message); this.name = "AdminCollaborationRejectedError"; }
}
function assertScope(scope: AdminScope): void {
  const auth = useAuthStore.getState();
  if (scope.apiScope !== adminApiScope() || scope.userId !== auth.user?.id || auth.status !== "authenticated") throw new Error("Admin session changed");
  if (!scope.organizationId) throw new AdminUnsupportedError();
}
function persistentScope(scope: AdminScope): AdminControlDraftScope {
  assertScope(scope);
  if (!scope.organizationId) throw new AdminUnsupportedError();
  return { server: getApi().getBaseUrl(), userId: scope.userId, organizationId: scope.organizationId };
}
async function readScoped<T extends { scope: string }>(scope: AdminScope, read: () => Promise<T | null>): Promise<T> {
  assertScope(scope);
  try {
    const result = await read();
    assertScope(scope);
    if (!result || result.scope !== scope.organizationId) throw new AdminUnsupportedError();
    return result;
  } catch (error) { assertScope(scope); throw error; }
}
function orderedPage(ids: string[], cursor: string | null): void {
  let previous = cursor;
  for (const id of ids) {
    if (previous !== null && id <= previous) throw new AdminUnsupportedError();
    previous = id;
  }
}

export function adminCollaborationIssuesOptions(scope: AdminScope) {
  return infiniteQueryOptions({
    queryKey: adminKeys.resource(scope, "collaboration-issues"), initialPageParam: null as string | null,
    queryFn: async ({ signal, pageParam }) => {
      const result = await readScoped(scope, () => getApi().getAdminCollaborationIssues({ limit: pageSize, cursor: pageParam ?? undefined }, { signal }));
      orderedPage(result.items.map(item => item.issueId), pageParam);
      return result;
    },
    getNextPageParam: page => page.nextCursor ?? undefined, ...reads,
  });
}
export function adminCollaborationIssueOptions(scope: AdminScope, issueId: string) {
  return queryOptions({
    queryKey: adminKeys.resource(scope, "collaboration-issue", { issueId }),
    queryFn: async ({ signal }) => {
      const result = await readScoped(scope, () => getApi().getAdminCollaborationIssue(issueId, { signal }));
      if (result.issueId !== issueId) throw new AdminUnsupportedError();
      return result;
    }, ...reads,
  });
}
export function adminCollaborationCandidatesOptions(scope: AdminScope, issueId: string, revision: string) {
  return infiniteQueryOptions({
    queryKey: adminKeys.resource(scope, "collaboration-candidates", { issueId, revision }), initialPageParam: null as string | null,
    queryFn: async ({ signal, pageParam }) => {
      const result = await readScoped(scope, () => getApi().getAdminCollaborationCandidates(issueId, { limit: pageSize, cursor: pageParam ?? undefined }, { signal }));
      if (result.issueId !== issueId) throw new AdminUnsupportedError();
      if (result.collaborationRevision !== revision) throw new AdminCollaborationStaleError();
      orderedPage(result.items.map(item => item.userId), pageParam);
      return result;
    },
    getNextPageParam: page => page.nextCursor ?? undefined, ...reads,
  });
}

export function adminCollaborationSnapshotMatches(snapshot: AdminCollaborationDetail, current: AdminCollaborationDetail | null | undefined): boolean {
  return !!current && snapshot.scope === current.scope && snapshot.issueId === current.issueId && snapshot.collaborationRevision === current.collaborationRevision &&
    snapshot.sourceWorkspaceId === current.sourceWorkspaceId &&
    snapshot.status === current.status && snapshot.creator.userId === current.creator.userId && current.creator.disabled === true &&
    snapshot.handler.userId === current.handler.userId && snapshot.handler.disabled === current.handler.disabled &&
    snapshot.queueState === current.queueState && snapshot.successorRequired === current.successorRequired &&
    JSON.stringify(snapshot.actions) === JSON.stringify(current.actions);
}
export function isAdminCollaborationReasonValid(reason: string): boolean {
  return AdminCollaborationChangeSchema.shape.reason.safeParse(reason).success;
}
export function captureAdminCollaborationInput(
  snapshot: AdminCollaborationDetail, key: string, body: AdminCollaborationChange, candidates: AdminCollaborationCandidates[] = [],
): AdminCollaborationInput {
  const parsed = AdminCollaborationChangeSchema.parse(body);
  const options = snapshot.actions.find(option => option.action === parsed.action);
  if (!snapshot.creator.disabled || parsed.expectedCollaborationRevision !== snapshot.collaborationRevision || parsed.expectedStatus !== snapshot.status ||
    !options || parsed.statusOptionsVersion !== options.status_options_version || !options.options.some(option => option.key === parsed.targetStatus)) {
    throw new AdminCollaborationStaleError();
  }
  if (snapshot.successorRequired && parsed.action === "reopen") {
    if (!parsed.successorUserId || !candidates.length || candidates.some(page => page.scope !== snapshot.scope || page.issueId !== snapshot.issueId || page.collaborationRevision !== snapshot.collaborationRevision) ||
      !candidates.some(page => page.items.some(candidate => candidate.userId === parsed.successorUserId))) throw new AdminCollaborationStaleError();
  } else if (parsed.successorUserId !== undefined) throw new AdminCollaborationStaleError();
  return { id: snapshot.issueId, key, action: "collaboration", creatorUserId: snapshot.creator.userId, handlerUserId: snapshot.handler.userId, body: parsed };
}
function matches(scope: AdminScope, input: AdminCollaborationInput, receipt: AdminCollaborationResult): boolean {
  return receipt.operation.organizationId === scope.organizationId && receipt.operation.actorId === scope.userId &&
    receipt.result.creatorUserId === input.creatorUserId && receipt.result.previousHandlerUserId === input.handlerUserId &&
    matchesAdminCollaborationResult(receipt, input.id, input.key, input.body);
}
export async function findAdminCollaborationOperation(scope: AdminScope, input: AdminCollaborationInput, signal?: AbortSignal): Promise<AdminCollaborationResult | null> {
  assertScope(scope);
  try {
    const result = await getApi().getAdminCollaborationOperation(input.id, input.key, { signal });
    assertScope(scope);
    if (!result || !matches(scope, input, result)) throw new AdminUnsupportedError();
    return result;
  } catch (error) {
    assertScope(scope);
    if (error instanceof ApiError && error.status === 404 && errorCode(error) === "admin_not_found") return null;
    throw error;
  }
}

// These explicit first-attempt refusals happen before commit. A previous lost
// request remains uncertain even if a later attempt receives the same refusal.
const rejectedCodes = new Set(["creator_not_disabled", "successor_required", "successor_forbidden", "successor_unavailable",
  "stale_revision", "stale_status", "stale_source", "status_options_changed", "invalid_lifecycle_request", "invalid_status_request", "invalid_reason",
  "issue_closed", "issue_not_closed", "status_unavailable"]);
export async function submitAdminCollaboration(scope: AdminScope, input: AdminCollaborationInput): Promise<AdminCollaborationResult> {
  assertScope(scope);
  const storageScope = persistentScope(scope);
  const existing = readAdminCollaborationDraft(storageScope, input.id);
  saveAdminControlDraft(storageScope, input);
  try {
    const result = await getApi().changeAdminCollaborationLifecycle(input.id, input.body, input.key);
    assertScope(scope);
    if (!result || !matches(scope, input, result)) throw new AdminUnsupportedError();
    return result;
  } catch (error) {
    assertScope(scope);
    if (error instanceof ApiError && [401, 403].includes(error.status)) throw error;
    if (!existing && error instanceof ApiError && [400, 409].includes(error.status) && rejectedCodes.has(errorCode(error) ?? "")) {
      try { removeAdminControlDraft(storageScope, input.id, input.key); }
      catch { throw new AdminControlUncertainError(input.key); }
      throw new AdminCollaborationRejectedError(error);
    }
    try {
      const receipt = await findAdminCollaborationOperation(scope, input);
      if (receipt) return receipt;
    } catch (lookupError) {
      assertScope(scope);
      if (lookupError instanceof ApiError && [401, 403].includes(lookupError.status)) throw lookupError;
    }
    throw new AdminControlUncertainError(input.key);
  }
}
export function dismissAdminCollaborationReceipt(scope: AdminScope, input: AdminCollaborationInput, receipt: AdminCollaborationResult): void {
  assertScope(scope);
  if (!matches(scope, input, receipt)) throw new AdminUnsupportedError();
  removeAdminControlDraft(persistentScope(scope), input.id, input.key);
}
export function adminCollaborationOperationOptions(scope: AdminScope, input: AdminCollaborationInput | null) {
  return queryOptions({
    queryKey: adminKeys.resource(scope, "collaboration-operation", { issueId: input?.id, requestId: input?.key }),
    queryFn: ({ signal }) => input ? findAdminCollaborationOperation(scope, input, signal) : Promise.reject(new AdminUnsupportedError()), ...reads,
  });
}
export function useAdminCollaborationDraft(scope: AdminScope, issueId: string) {
  const { apiScope, userId, organizationId } = scope;
  const identity = JSON.stringify([apiScope, userId, organizationId, issueId]);
  const [state, setState] = useState<{ identity: string; draft: AdminCollaborationDraft | null; error: boolean } | null>(null);
  const reload = useCallback(() => {
    try { setState({ identity, draft: readAdminCollaborationDraft(persistentScope({ apiScope, userId, organizationId }), issueId), error: false }); }
    catch { setState({ identity, draft: null, error: true }); }
  }, [identity, apiScope, userId, organizationId, issueId]);
  useEffect(reload, [reload]);
  return { loading: state?.identity !== identity, draft: state?.identity === identity ? state.draft : null, error: state?.identity === identity && state.error, reload };
}
export function useAdminCollaborationIssues(scope: AdminScope) { return useInfiniteQuery(adminCollaborationIssuesOptions(scope)); }
export function useAdminCollaborationIssue(scope: AdminScope, issueId: string) { return useQuery(adminCollaborationIssueOptions(scope, issueId)); }
export function useAdminCollaborationCandidates(scope: AdminScope, issueId: string, revision: string, enabled: boolean) {
  return useInfiniteQuery({ ...adminCollaborationCandidatesOptions(scope, issueId, revision), enabled });
}
export function useAdminCollaborationOperation(scope: AdminScope, input: AdminCollaborationInput | null, enabled: boolean) {
  return useQuery({ ...adminCollaborationOperationOptions(scope, input), enabled: !!input && enabled });
}
export function useAdminCollaborationMutation(scope: AdminScope) {
  const client = useQueryClient();
  return useMutation({
    mutationKey: adminKeys.resource(scope, "collaboration-lifecycle"), mutationFn: (input: AdminCollaborationInput) => submitAdminCollaboration(scope, input),
    retry: false, gcTime: 0,
    onSuccess: (receipt, input) => {
      assertScope(scope);
      client.setQueryData(adminCollaborationOperationOptions(scope, input).queryKey, receipt);
      void client.invalidateQueries({ queryKey: adminKeys.resource(scope, "collaboration-issue", { issueId: input.id }) });
      void client.invalidateQueries({ queryKey: adminKeys.resource(scope, "collaboration-issues") });
    },
  });
}
