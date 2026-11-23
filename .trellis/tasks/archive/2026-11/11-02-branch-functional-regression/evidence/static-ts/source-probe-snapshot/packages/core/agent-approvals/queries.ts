import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { ApprovalStatus } from "../types/agent-template";

/**
 * Server state for the human approval queue.
 *
 * The backend decides what a caller may see: a person gets the whole workspace's
 * queue, an agent only its own. That is one endpoint either way, so there is one
 * query here and no client-side filtering of somebody else's rows.
 *
 * Keys carry `wsId` like every other workspace-scoped key, even though the
 * endpoint reads the workspace from the request header — switching workspaces has
 * to miss the cache rather than show the previous workspace's queue.
 */
export const agentApprovalKeys = {
  all: (wsId: string) => ["agent-approvals", wsId] as const,
  list: (wsId: string, status?: ApprovalStatus) =>
    [...agentApprovalKeys.all(wsId), "list", status ?? "any"] as const,
  detail: (wsId: string, id: string) =>
    [...agentApprovalKeys.all(wsId), "detail", id] as const,
};

/**
 * One page of the queue, newest first.
 *
 * Refetched on focus because the interesting case is a person coming back to a
 * tab they left open while an agent filed something. Approvals publish no
 * realtime event, so focus and the mutations below are the only things that move
 * this cache — deliberately not a poll, since a stale queue is visible rather
 * than harmful.
 */
export function agentApprovalListOptions(
  wsId: string,
  status?: ApprovalStatus,
  limit?: number,
) {
  return queryOptions({
    queryKey: agentApprovalKeys.list(wsId, status),
    queryFn: () => api.listAgentApprovals({ status, limit }),
    enabled: wsId.length > 0,
    staleTime: 15_000,
    refetchOnWindowFocus: true,
  });
}

export function agentApprovalDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: agentApprovalKeys.detail(wsId, id),
    queryFn: () => api.getAgentApproval(id),
    enabled: wsId.length > 0 && id.length > 0,
  });
}
