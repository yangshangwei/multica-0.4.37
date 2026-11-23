"use client";

import { queryOptions, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, getApi } from "../api";
import { useAuthStore } from "../auth";
import { adminApiScope, adminKeys, AdminUnsupportedError, type AdminScope } from "./queries";
import type { AdminUserFilters, AdminAccountAction, AdminAccountChange, AdminRoleChange, AdminUserOperation } from "./user-schema";

function currentScope(scope: AdminScope): boolean {
  return scope.apiScope === adminApiScope() && scope.userId === useAuthStore.getState().user?.id;
}
function assertScope(scope: AdminScope): void {
  if (!currentScope(scope)) throw new Error("The administrative session changed");
}
export function adminUsersOptions(scope: AdminScope, filters: AdminUserFilters) {
  return queryOptions({
    queryKey: adminKeys.resource(scope, "users", { ...filters }),
    queryFn: async ({ signal }) => {
      const result = await getApi().getAdminUsers(filters, { signal });
      assertScope(scope);
      if (!result || result.scope !== scope.organizationId) throw new AdminUnsupportedError();
      return result;
    },
    retry: false,
    staleTime: 0,
    gcTime: 0,
    refetchInterval: 15_000,
    refetchIntervalInBackground: false,
  });
}
export function adminUserOptions(scope: AdminScope, id: string) {
  return queryOptions({
    queryKey: adminKeys.resource(scope, "user", { id }),
    queryFn: async ({ signal }) => {
      const result = await getApi().getAdminUser(id, { signal });
      assertScope(scope);
      if (!result || result.scope !== scope.organizationId || result.user.id !== id) throw new AdminUnsupportedError();
      return result;
    },
    retry: false,
    staleTime: 0,
    gcTime: 0,
    refetchInterval: 5_000,
    refetchIntervalInBackground: false,
  });
}
export function useAdminUsers(scope: AdminScope, filters: AdminUserFilters) {
  return useQuery({ ...adminUsersOptions(scope, filters), enabled: !!scope.organizationId && !!scope.userId });
}
export function useAdminUser(scope: AdminScope, id: string) {
  return useQuery({ ...adminUserOptions(scope, id), enabled: !!scope.organizationId && !!id });
}
export class AdminUserOperationUncertainError extends Error {
  constructor(readonly idempotencyKey: string) {
    super("The operation result is unknown; check its status before retrying");
    this.name = "AdminUserOperationUncertainError";
  }
}
export type AdminUserMutationInput = { id: string; key: string } & (
  { action: AdminAccountAction; body: AdminAccountChange } |
  { action: "role"; body: AdminRoleChange }
);
export async function findAdminUserOperation(scope: AdminScope, key: string): Promise<AdminUserOperation | null> {
  assertScope(scope);
  const result = await getApi().getAdminOperations(key);
  assertScope(scope);
  if (!result || result.scope !== scope.organizationId || result.items.some((item) => item.organizationId !== scope.organizationId)) throw new AdminUnsupportedError();
  return result.items[0] ?? null;
}
export async function submitAdminUserChange(scope: AdminScope, input: AdminUserMutationInput): Promise<AdminUserOperation> {
  assertScope(scope);
  const kind = { role: "user.role", disable: "user.disable", restore: "user.restore", "recover-password": "user.password.recover" }[input.action];
  const matches = (result: AdminUserOperation | null): result is AdminUserOperation =>
    !!result && result.organizationId === scope.organizationId && result.targetId === input.id && result.kind === kind;
  try {
    const result = input.action === "role"
      ? await getApi().changeAdminRole(input.id, input.body, input.key)
      : await getApi().changeAdminAccount(input.id, input.action, input.body, input.key);
    assertScope(scope);
    if (!matches(result)) throw new AdminUnsupportedError();
    return result;
  } catch (error) {
    assertScope(scope);
    if (error instanceof ApiError && error.status >= 400 && error.status < 500) throw error;
    try {
      const existing = await findAdminUserOperation(scope, input.key);
      if (matches(existing)) return existing;
    } catch {
      assertScope(scope);
    }
    throw new AdminUserOperationUncertainError(input.key);
  }
}
export function useAdminUserMutation(scope: AdminScope) {
  const client = useQueryClient();
  return useMutation({
    mutationKey: adminKeys.resource(scope, "user-action"),
    mutationFn: (input: AdminUserMutationInput) => submitAdminUserChange(scope, input),
    gcTime: 0,
    retry: false,
    onSuccess: () => {
      if (currentScope(scope)) void client.invalidateQueries({ queryKey: adminKeys.all });
    },
  });
}
