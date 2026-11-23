"use client";

import { useCallback, useEffect, useState } from "react";
import { infiniteQueryOptions, queryOptions, useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, errorCode, getApi } from "../api";
import { useAuthStore } from "../auth";
import { compareCollaborationMemoryVersions } from "../collaboration/memory-schema";
import { adminApiScope, adminKeys, AdminUnsupportedError, type AdminScope } from "./queries";
import { AdminControlUncertainError } from "./operation-queries";
import { readAdminCollaborationMemoryDraft, removeAdminCollaborationMemoryDraft, saveAdminCollaborationMemoryDraft,
  type AdminCollaborationMemoryDraft, type AdminCollaborationMemoryInput, type AdminControlDraftScope } from "./operation-draft";
import { AdminCollaborationMemoryPurgeSchema, matchesAdminCollaborationMemoryResult, type AdminCollaborationMemoryResult } from "./collaboration-schema";

const reads = { retry: false as const, staleTime: 0, gcTime: 0, refetchOnWindowFocus: false };

export class AdminCollaborationMemoryRejectedError extends Error {
  constructor(readonly response: ApiError) { super(response.message); this.name = "AdminCollaborationMemoryRejectedError"; }
}
function assertScope(scope: AdminScope): void {
  const auth = useAuthStore.getState();
  if (scope.apiScope !== adminApiScope() || scope.userId !== auth.user?.id || auth.status !== "authenticated") throw new Error("Admin session changed");
  if (!scope.organizationId) throw new AdminUnsupportedError();
}
function persistentScope(scope: AdminScope): AdminControlDraftScope {
  assertScope(scope);
  return { server: getApi().getBaseUrl(), userId: scope.userId, organizationId: scope.organizationId! };
}

/** Version metadata only. A restored creator returns creator_not_disabled. */
export function adminCollaborationMemoryVersionsOptions(scope: AdminScope, issueId: string) {
  return infiniteQueryOptions({
    queryKey: adminKeys.resource(scope, "collaboration-memory-versions", { issueId }), initialPageParam: null as string | null,
    queryFn: async ({ signal, pageParam }) => {
      assertScope(scope);
      try {
        const result = await getApi().getAdminCollaborationMemoryVersions(issueId, { limit: 50, cursor: pageParam ?? undefined }, { signal });
        assertScope(scope);
        if (!result || result.scope !== scope.organizationId || result.issueId !== issueId ||
          (pageParam !== null && result.items.some(item => compareCollaborationMemoryVersions(item.version, pageParam) >= 0))) throw new AdminUnsupportedError();
        return result;
      } catch (error) { assertScope(scope); throw error; }
    },
    getNextPageParam: (page, _pages, _param, params) => page.nextCursor !== null && !params.includes(page.nextCursor) ? page.nextCursor : undefined, ...reads,
  });
}

export function captureAdminCollaborationMemoryPurge(issueId: string, creatorUserId: string, key: string, version: string, reason: string): AdminCollaborationMemoryInput {
  return { id: issueId, key, action: "collaboration_memory", creatorUserId, body: AdminCollaborationMemoryPurgeSchema.parse({ version, reason }) };
}
export function isAdminCollaborationMemoryReasonValid(reason: string): boolean {
  return AdminCollaborationMemoryPurgeSchema.shape.reason.safeParse(reason).success;
}
function matches(scope: AdminScope, input: AdminCollaborationMemoryInput, receipt: AdminCollaborationMemoryResult): boolean {
  return receipt.operation.organizationId === scope.organizationId && receipt.operation.actorId === scope.userId &&
    receipt.result.creatorUserId === input.creatorUserId.toLowerCase() && matchesAdminCollaborationMemoryResult(receipt, input.id, input.key, input.body);
}
export async function findAdminCollaborationMemoryPurge(scope: AdminScope, input: AdminCollaborationMemoryInput, signal?: AbortSignal): Promise<AdminCollaborationMemoryResult | null> {
  assertScope(scope);
  try {
    const result = await getApi().getAdminCollaborationMemoryOperation(input.id, input.key, { signal });
    assertScope(scope);
    if (!result || !matches(scope, input, result)) throw new AdminUnsupportedError();
    return result;
  } catch (error) {
    assertScope(scope);
    if (error instanceof ApiError && error.status === 404 && errorCode(error) === "admin_not_found") return null;
    throw error;
  }
}

// Refusals the server decides before commit; the reserved operation rolls back.
const rejectedCodes = new Set(["creator_not_disabled", "memory_version_not_found", "memory_version_purged", "memory_version_current",
  "invalid_memory_purge", "invalid_reason", "invalid_version"]);
export async function submitAdminCollaborationMemoryPurge(scope: AdminScope, input: AdminCollaborationMemoryInput): Promise<AdminCollaborationMemoryResult> {
  const storageScope = persistentScope(scope);
  const existing = readAdminCollaborationMemoryDraft(storageScope, input.id);
  saveAdminCollaborationMemoryDraft(storageScope, input);
  try {
    const result = await getApi().purgeAdminCollaborationMemoryVersion(input.id, input.body, input.key);
    assertScope(scope);
    if (!result || !matches(scope, input, result)) throw new AdminUnsupportedError();
    return result;
  } catch (error) {
    assertScope(scope);
    if (error instanceof ApiError && [401, 403].includes(error.status)) throw error;
    // A previous lost attempt stays uncertain even if a retry is refused.
    if (!existing && error instanceof ApiError && [400, 404, 409].includes(error.status) && rejectedCodes.has(errorCode(error) ?? "")) {
      try { removeAdminCollaborationMemoryDraft(storageScope, input.id, input.key); }
      catch { throw new AdminControlUncertainError(input.key); }
      throw new AdminCollaborationMemoryRejectedError(error);
    }
    try {
      const receipt = await findAdminCollaborationMemoryPurge(scope, input);
      if (receipt) return receipt;
    } catch (lookupError) {
      assertScope(scope);
      if (lookupError instanceof ApiError && [401, 403].includes(lookupError.status)) throw lookupError;
    }
    throw new AdminControlUncertainError(input.key);
  }
}
export function dismissAdminCollaborationMemoryReceipt(scope: AdminScope, input: AdminCollaborationMemoryInput, receipt: AdminCollaborationMemoryResult): void {
  if (!matches(scope, input, receipt)) throw new AdminUnsupportedError();
  removeAdminCollaborationMemoryDraft(persistentScope(scope), input.id, input.key);
}
export function adminCollaborationMemoryOperationOptions(scope: AdminScope, input: AdminCollaborationMemoryInput | null) {
  return queryOptions({
    queryKey: adminKeys.resource(scope, "collaboration-memory-operation", { issueId: input?.id, requestId: input?.key }),
    queryFn: ({ signal }) => input ? findAdminCollaborationMemoryPurge(scope, input, signal) : Promise.reject(new AdminUnsupportedError()), ...reads,
  });
}
export function useAdminCollaborationMemoryDraft(scope: AdminScope, issueId: string) {
  const { apiScope, userId, organizationId } = scope;
  const identity = JSON.stringify([apiScope, userId, organizationId, issueId]);
  const [state, setState] = useState<{ identity: string; draft: AdminCollaborationMemoryDraft | null; error: boolean } | null>(null);
  const reload = useCallback(() => {
    try { setState({ identity, draft: readAdminCollaborationMemoryDraft(persistentScope({ apiScope, userId, organizationId }), issueId), error: false }); }
    catch { setState({ identity, draft: null, error: true }); }
  }, [identity, apiScope, userId, organizationId, issueId]);
  useEffect(reload, [reload]);
  return { loading: state?.identity !== identity, draft: state?.identity === identity ? state.draft : null, error: state?.identity === identity && state.error, reload };
}
export function useAdminCollaborationMemoryVersions(scope: AdminScope, issueId: string, enabled: boolean) {
  return useInfiniteQuery({ ...adminCollaborationMemoryVersionsOptions(scope, issueId), enabled });
}
export function useAdminCollaborationMemoryOperation(scope: AdminScope, input: AdminCollaborationMemoryInput | null, enabled: boolean) {
  return useQuery({ ...adminCollaborationMemoryOperationOptions(scope, input), enabled: !!input && enabled });
}
export function useAdminCollaborationMemoryPurge(scope: AdminScope) {
  const client = useQueryClient();
  return useMutation({
    mutationKey: adminKeys.resource(scope, "collaboration-memory-purge"),
    mutationFn: (input: AdminCollaborationMemoryInput) => submitAdminCollaborationMemoryPurge(scope, input),
    retry: false, gcTime: 0,
    onSuccess: (receipt, input) => {
      assertScope(scope);
      client.setQueryData(adminCollaborationMemoryOperationOptions(scope, input).queryKey, receipt);
      void client.invalidateQueries({ queryKey: adminKeys.resource(scope, "collaboration-memory-versions", { issueId: input.id }) });
    },
    onError: (_error, input) => {
      void client.invalidateQueries({ queryKey: adminKeys.resource(scope, "collaboration-memory-versions", { issueId: input.id }) });
    },
  });
}
