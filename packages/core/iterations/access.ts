import { notifyPendingIterations } from "./pending-signal";
import type { QueryClient } from "@tanstack/react-query";
import { api, isIterationAccessDenied } from "../api";
import { defaultStorage } from "../platform/storage";
import { ApiError } from "../api/client";

interface AccessState {
  epoch: number;
  denied: boolean;
}
const accessKey = (wsId: string) => ["iterations", wsId, "access"] as const;
const accessState = (client: QueryClient, wsId: string): AccessState =>
  client.getQueryData<AccessState>(accessKey(wsId)) ?? {
    epoch: 0,
    denied: false,
  };
const revokedError = () =>
  new ApiError("Iteration access revoked", 403, "Forbidden");

export function clearIterationReadAccess(client: QueryClient, wsId: string) {
  const prefix = `multica_iteration_command:${encodeURIComponent(api.getBaseUrl?.() ?? "")}:`;
  for (const key of defaultStorage.keys?.() ?? []) {
    if (key.startsWith(prefix) && key.includes(`:${wsId}:`))
      defaultStorage.removeItem(key);
  }
  notifyPendingIterations();
  // Do not let observer teardown restore a fetch's pre-revocation data.
  void client.cancelQueries(
    { queryKey: ["iterations", wsId] },
    { revert: false },
  );
  const current = accessState(client, wsId);
  client.setQueryData(accessKey(wsId), {
    epoch: current.epoch + 1,
    denied: true,
  });
  for (const mutation of client.getMutationCache().getAll()) {
    const key = mutation.options.mutationKey;
    if (key?.[0] !== "iterations" || key[1] !== wsId) continue;
    mutation.state = {
      ...mutation.state,
      data: undefined,
      variables: undefined,
      context: undefined,
    };
    client.getMutationCache().remove(mutation);
  }
  for (const query of client
    .getQueryCache()
    .findAll({ queryKey: ["iterations", wsId] })) {
    if (query.queryKey[2] === "access") continue;
    // Erase protected data even from the active failed query. Merely removing an
    // active query leaves its observer holding the previous successful result.
    query.setState({
      data: undefined,
      dataUpdatedAt: 0,
      error: revokedError(),
      status: "error",
    });
  }
}

export async function protectIterationRead<T>(
  client: QueryClient,
  wsId: string,
  request: () => Promise<T>,
  allowRecovery = false,
): Promise<T> {
  const session = api.getSessionScope?.();
  const before = accessState(client, wsId);
  if (before.denied && !allowRecovery) throw revokedError();
  try {
    const result = await request();
    if (api.getSessionScope?.() !== session)
      throw new Error("Iteration session changed");
    const after = accessState(client, wsId);
    if (after.epoch !== before.epoch || (after.denied && !allowRecovery))
      throw revokedError();
    if (allowRecovery && after.denied)
      client.setQueryData(accessKey(wsId), { ...after, denied: false });
    return result;
  } catch (error) {
    // Old-account reads must neither expose data nor revoke the new account.
    if (api.getSessionScope?.() !== session)
      throw new Error("Iteration session changed");
    if (isIterationAccessDenied(error) && !accessState(client, wsId).denied) {
      clearIterationReadAccess(client, wsId);
    }
    throw error;
  }
}

export function hasIterationReadAccess(
  client: QueryClient,
  wsId: string,
): boolean {
  return !accessState(client, wsId).denied;
}

export function iterationAccessEpoch(
  client: QueryClient,
  wsId: string,
): number {
  return accessState(client, wsId).epoch;
}
