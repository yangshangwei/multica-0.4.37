"use client";
import { queryOptions, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, errorCode, getApi } from "../api";
import { useAuthStore } from "../auth";
import { adminApiScope, adminKeys, type AdminScope } from "./queries";
import type { AdminResourceKind, AdminResourcePublish, AdminResourceResult, AdminResourceUpload, AdminResourceWithdraw } from "./resource-schema";

export class AdminResourceUnsupportedError extends Error {
  constructor() { super("Resource publishing is unsupported by this response"); this.name = "AdminResourceUnsupportedError"; }
}
export class AdminResourceUncertainError extends Error {
  constructor(readonly operationId: string) { super("Resource operation outcome is unknown; check its receipt"); this.name = "AdminResourceUncertainError"; }
}
function assertScope(scope: AdminScope) {
  if (scope.apiScope !== adminApiScope() || scope.userId !== useAuthStore.getState().user?.id) throw new Error("Admin session changed");
}
export function adminResourcesOptions(scope: AdminScope, kind: AdminResourceKind) {
  return queryOptions({
    queryKey: adminKeys.resource(scope, "resources", { kind }),
    queryFn: async ({ signal }) => {
      assertScope(scope);
      const result = await getApi().getAdminResources(kind, { signal });
      assertScope(scope);
      if (!result || result.items.some(item => item.kind !== kind)) throw new AdminResourceUnsupportedError();
      return result;
    },
    retry: false, staleTime: 0, gcTime: 0, refetchInterval: 30_000, refetchIntervalInBackground: false,
  });
}
export function useAdminResources(scope: AdminScope, kind: AdminResourceKind) {
  return useQuery({ ...adminResourcesOptions(scope, kind), enabled: !!scope.organizationId && !!scope.userId });
}
export function useAdminResourcePreview(scope: AdminScope, kind: AdminResourceKind) {
  return useMutation({
    mutationKey: adminKeys.resource(scope, "resource-preview", { kind }),
    mutationFn: async (input: AdminResourceUpload & { signal?: AbortSignal }) => {
      assertScope(scope);
      const result = await getApi().previewAdminResource(kind, input, { signal: input.signal });
      assertScope(scope);
      if (!result || result.resource.kind !== kind || result.resource.key !== input.key || result.resource.source !== "managed") throw new AdminResourceUnsupportedError();
      return result;
    },
    retry: false, gcTime: 0,
  });
}
export type AdminResourceMutationInput = { kind: AdminResourceKind; key: string; operationId: string } & (
  { action: "publish" } & AdminResourcePublish | { action: "withdraw" } & AdminResourceWithdraw
);
function matches(input: AdminResourceMutationInput, result: AdminResourceResult | null): result is AdminResourceResult {
  return !!result && result.operationId === input.operationId && result.resource.kind === input.kind && result.resource.key === input.key && result.resource.source === "managed" && result.resource.state === (input.action === "publish" ? "published" : "withdrawn");
}
export async function findAdminResourceOperation(scope: AdminScope, input: AdminResourceMutationInput): Promise<AdminResourceResult | null> {
  assertScope(scope);
  try {
    const result = await getApi().getAdminResourceOperation(input.operationId);
    assertScope(scope);
    if (!matches(input, result)) throw new AdminResourceUnsupportedError();
    return result;
  } catch (error) {
    assertScope(scope);
    if (error instanceof ApiError && error.status === 404) return null;
    throw error;
  }
}
export async function submitAdminResource(scope: AdminScope, input: AdminResourceMutationInput): Promise<AdminResourceResult> {
  assertScope(scope);
  try {
    const result = input.action === "publish"
      ? await getApi().publishAdminResource(input.kind, input, input.operationId)
      : await getApi().withdrawAdminResource(input.kind, input.key, input, input.operationId);
    assertScope(scope);
    if (!matches(input, result)) throw new AdminResourceUnsupportedError();
    return result;
  } catch (error) {
    assertScope(scope);
    if (error instanceof ApiError && (error.status >= 400 && error.status < 500 || ["resource_publishing_disabled", "resource_store_unavailable", "resource_store_full"].includes(errorCode(error) ?? ""))) throw error;
    try {
      const receipt = await findAdminResourceOperation(scope, input);
      if (receipt) return receipt;
    } catch (lookupError) {
      assertScope(scope);
      if (lookupError instanceof ApiError && [401, 403].includes(lookupError.status)) throw lookupError;
    }
    // A missing receipt does not prove a delayed write cannot still commit.
    throw new AdminResourceUncertainError(input.operationId);
  }
}
export function useAdminResourceMutation(scope: AdminScope) {
  const client = useQueryClient();
  return useMutation({
    mutationKey: adminKeys.resource(scope, "resource-action"),
    mutationFn: (input: AdminResourceMutationInput) => submitAdminResource(scope, input),
    retry: false, gcTime: 0,
    onSuccess: () => { assertScope(scope); void client.invalidateQueries({ queryKey: adminKeys.all }); },
  });
}
export function useAdminResourceLookup(scope: AdminScope) {
  const client = useQueryClient();
  return useMutation({
    mutationKey: adminKeys.resource(scope, "resource-receipt"),
    mutationFn: (input: AdminResourceMutationInput) => findAdminResourceOperation(scope, input),
    retry: false, gcTime: 0,
    onSuccess: result => { assertScope(scope); if (result) void client.invalidateQueries({ queryKey: adminKeys.all }); },
  });
}
