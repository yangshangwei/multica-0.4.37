import { queryOptions, type QueryClient, type QueryKey } from "@tanstack/react-query";
import { getApi, ApiError, errorCode } from "../api";
import { useAuthStore } from "../auth";
import type { AdminIdentity } from "./schema";

export interface AdminScope {
  apiScope: string;
  userId: string;
  organizationId: string | null;
}

export const adminKeys = {
  all: ["admin"] as const,
  resource: (scope: AdminScope, resource: string, filters: Readonly<Record<string, unknown>> = {}) =>
    ["admin", scope.apiScope, scope.userId, scope.organizationId, resource, filters] as const,
  me: (scope: AdminScope) => adminKeys.resource(scope, "me"),
};

const apiInstances = new WeakMap<object, number>();
let nextApiInstance = 0;

export function adminApiScope(): string {
  const api = getApi();
  let instance = apiInstances.get(api);
  if (instance === undefined) {
    instance = ++nextApiInstance;
    apiInstances.set(api, instance);
  }
  return JSON.stringify([api.getBaseUrl(), instance, api.getSessionScope()]);
}

export function isAdminKey(key: QueryKey | undefined): boolean {
  return key?.[0] === "admin";
}

export function clearAdminCache(client: QueryClient): void {
  void client.cancelQueries({ queryKey: adminKeys.all });
  client.removeQueries({ queryKey: adminKeys.all });
  for (const mutation of client.getMutationCache().getAll()) {
    if (isAdminKey(mutation.options.mutationKey)) client.getMutationCache().remove(mutation);
  }
}

/** Remove stale scope data, including pending requests, without touching workspace queries. */
export function retainAdminScope(client: QueryClient, scope: Pick<AdminScope, "apiScope" | "userId"> | null): void {
  const stale = (key: QueryKey | undefined) => isAdminKey(key) &&
    (!scope || key?.[1] !== scope.apiScope || key?.[2] !== scope.userId);
  const predicate = (query: { queryKey: QueryKey }) => stale(query.queryKey);
  void client.cancelQueries({ predicate });
  client.removeQueries({ predicate });
  for (const mutation of client.getMutationCache().getAll()) {
    if (stale(mutation.options.mutationKey)) client.getMutationCache().remove(mutation);
  }
}

export function isAdminPermissionDenied(error: unknown): boolean {
  if (!(error instanceof ApiError) || error.status !== 403) return false;
  // Sensitive actions can reject password confirmation while the platform
  // role remains valid. Keep these errors with the form that submitted it.
  const code = errorCode(error);
  return code !== "password_verification_failed" && code !== "password_verification_stale";
}

export class AdminUnsupportedError extends Error {
  constructor() {
    super("Platform administration is not supported by this response");
    this.name = "AdminUnsupportedError";
  }
}

export function adminMeOptions(scope: AdminScope) {
  return queryOptions({
    queryKey: adminKeys.me(scope),
    queryFn: async ({ signal }): Promise<AdminIdentity> => {
      const identity = await getApi().getAdminMe({ signal });
      // A response sent before a login or server switch must never authorize
      // the new session, even if it arrives before cancellation is observed.
      if (scope.apiScope !== adminApiScope() || useAuthStore.getState().user?.id !== scope.userId) {
        throw new Error("Admin session changed");
      }
      if (!identity || identity.userId !== scope.userId) throw new AdminUnsupportedError();
      return identity;
    },
    staleTime: 0,
    gcTime: 0,
    retry: false,
    refetchOnMount: "always",
    refetchOnWindowFocus: true,
    refetchInterval: 15_000,
    refetchIntervalInBackground: false,
  });
}
