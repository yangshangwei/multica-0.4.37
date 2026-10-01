"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { queryOptions, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, errorCode, getApi } from "../api";
import { useAuthStore } from "../auth";
import { adminApiScope, adminKeys, AdminUnsupportedError, type AdminScope } from "./queries";
import { operationNeedsPolling, type AdminOperation } from "./operation-schema";
import type { AdminAdmissionChange, AdminExecutionCancellation } from "./control-schema";
import type { AdminAlertChange } from "./alert-schema";
import { readAdminControlDraft, saveAdminControlDraft, removeAdminControlDraft, type AdminControlDraftScope, type AdminControlDraft } from "./operation-draft";

export type AdminControlInput = { id: string; key: string } & (
  { action: "admission"; body: AdminAdmissionChange } |
  { action: "cancel"; body: AdminExecutionCancellation } |
  { action: "alert"; body: AdminAlertChange }
);
export class AdminControlUncertainError extends Error {
  constructor(readonly idempotencyKey: string) {
    super("The operation outcome is uncertain; query or retry the original request");
    this.name = "AdminControlUncertainError";
  }
}
function assertScope(scope: AdminScope) {
  if (scope.apiScope !== adminApiScope() || scope.userId !== useAuthStore.getState().user?.id) throw new Error("Admin session changed");
}
function draftScope(scope: AdminScope): AdminControlDraftScope {
  assertScope(scope);
  if (!scope.organizationId) throw new AdminUnsupportedError();
  return { server: getApi().getBaseUrl(), userId: scope.userId, organizationId: scope.organizationId };
}
function clearDeterminedDraft(scope: AdminScope, input: AdminControlInput) {
  // A storage cleanup failure cannot make a durable receipt uncertain again.
  // On the next mount the retained original key will resolve to this receipt.
  try { removeAdminControlDraft(draftScope(scope), input.id, input.key); return true; }
  catch { return false; }
}
function matches(scope: AdminScope, input: AdminControlInput, operation: AdminOperation): boolean {
  return operation.organizationId === scope.organizationId && operation.actorId === scope.userId && operation.targetId === input.id &&
    operation.kind === (input.action === "cancel" ? "task.cancel" : input.action === "alert" ? `alert.${input.body.action}` : "installation.admission");
}
export async function findAdminControlOperation(scope: AdminScope, input: AdminControlInput): Promise<AdminOperation | null> {
  assertScope(scope);
  const result = await getApi().getAdminOperations(input.key);
  assertScope(scope);
  if (!result || result.scope !== scope.organizationId || result.items.some(operation => !matches(scope, input, operation))) throw new AdminUnsupportedError();
  const operation = result.items[0] ?? null;
  if (operation) clearDeterminedDraft(scope, input);
  return operation;
}
export async function submitAdminControl(scope: AdminScope, input: AdminControlInput): Promise<AdminOperation> {
  assertScope(scope);
  const persistentScope = draftScope(scope);
  const existing = readAdminControlDraft(persistentScope, input.id);
  saveAdminControlDraft(persistentScope, input);
  try {
    const result = input.action === "alert" ? await getApi().changeAdminAlert(input.id, input.body, input.key) : input.action === "cancel"
      ? await getApi().cancelAdminExecution(input.id, input.body, input.key)
      : await getApi().changeAdminAdmission(input.id, input.body, input.key);
    assertScope(scope);
    if (!result || result.target.id !== input.id || !matches(scope, input, result.operation)) throw new AdminUnsupportedError();
    clearDeterminedDraft(scope, input);
    return result.operation;
  } catch (error) {
    assertScope(scope);
    if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
      const hasFailedReceipt = error.status === 409 && (input.action === "cancel" && errorCode(error) === "execution_fence_conflict" || input.action === "alert" && ["alert_version_conflict", "alert_state_conflict", "alert_assignee_unavailable", "alert_resolution_invalid"].includes(errorCode(error) ?? ""));
      if (!existing && !hasFailedReceipt) { clearDeterminedDraft(scope, input); throw error; }
      if ([401, 403].includes(error.status)) throw error;
    }
    let lookupEmpty = false;
    try {
      const operation = await findAdminControlOperation(scope, input);
      if (operation) return operation;
      lookupEmpty = true;
    } catch (lookupError) {
      assertScope(scope);
      if (lookupError instanceof ApiError && [401, 403].includes(lookupError.status)) throw lookupError;
    }
    // The admission handler serializes same-actor key lookup before checking
    // its monotonic version. This exact rejection makes the old policy write
    // impossible, even if its original HTTP request arrives later. Execution
    // fence or generic conflicts do not provide that guarantee.
    if (lookupEmpty && input.action === "admission" && error instanceof ApiError && error.status === 409 &&
        errorCode(error) === "admission_version_conflict" && clearDeterminedDraft(scope, input)) throw error;
    throw new AdminControlUncertainError(input.key);
  }
}
export function useAdminControlDraft(scope: AdminScope, id: string) {
  const { apiScope, userId, organizationId } = scope;
  const identity = JSON.stringify([apiScope, userId, organizationId, id]);
  const [state, setState] = useState<{ identity: string; draft: AdminControlDraft | null; error: boolean } | null>(null);
  const reload = useCallback(() => {
    try { setState({ identity, draft: readAdminControlDraft(draftScope({ apiScope, userId, organizationId }), id), error: false }); }
    catch { setState({ identity, draft: null, error: true }); }
  }, [identity, apiScope, userId, organizationId, id]);
  useEffect(reload, [reload]);
  return { loading: state?.identity !== identity, draft: state?.identity === identity ? state.draft : null, error: state?.identity === identity && state.error, reload };
}
export function adminOperationOptions(scope: AdminScope, id: string) {
  return queryOptions({
    queryKey: adminKeys.resource(scope, "operation", { id }),
    queryFn: async ({ signal }) => {
      assertScope(scope);
      const result = await getApi().getAdminOperation(id, { signal });
      assertScope(scope);
      if (!result || result.id !== id || result.organizationId !== scope.organizationId) throw new AdminUnsupportedError();
      return result;
    },
    retry: false, staleTime: 0, gcTime: 0,
    refetchIntervalInBackground: false, refetchOnWindowFocus: false,
  });
}
export function useAdminOperation(scope: AdminScope, id: string, initialData?: AdminOperation) {
  const startedAt = useRef(Date.now());
  return useQuery({
    ...adminOperationOptions(scope, id), initialData,
    enabled: !!scope.organizationId && !!scope.userId && !!id,
    refetchInterval: query => query.state.error || !query.state.data || !operationNeedsPolling(query.state.data, Date.now() - startedAt.current) ? false : 2_000,
  });
}
export function useAdminControlMutation(scope: AdminScope) {
  const client = useQueryClient();
  return useMutation({
    mutationKey: adminKeys.resource(scope, "control-action"),
    mutationFn: (input: AdminControlInput) => submitAdminControl(scope, input),
    retry: false, gcTime: 0,
    onSuccess: operation => {
      assertScope(scope);
      client.setQueryData(adminOperationOptions(scope, operation.id).queryKey, operation);
      void client.invalidateQueries({ queryKey: adminKeys.all });
    },
  });
}
export function useAdminControlLookup(scope: AdminScope) {
  return useMutation({
    mutationKey: adminKeys.resource(scope, "control-lookup"),
    mutationFn: (input: AdminControlInput) => findAdminControlOperation(scope, input),
    retry: false, gcTime: 0,
  });
}
