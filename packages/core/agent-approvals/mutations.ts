import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { useWorkspaceId } from "../hooks";
import { agentApprovalKeys } from "./queries";
import type { AgentApproval } from "../types/agent-template";

/**
 * Deciding and cancelling approval requests.
 *
 * Neither is optimistic. Both navigate nothing and both are cheap to await, but
 * more to the point a decision is the record the whole feature exists to produce:
 * showing "approved" before the server agreed would be showing an authorization
 * that may not exist. The server also refuses a second decision with 409, and an
 * optimistic patch would paint over exactly the conflict the person needs to see.
 */

/** Records a person's approve/reject on one request. */
export function useDecideAgentApproval() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({
      id,
      decision,
      note,
    }: {
      id: string;
      decision: "approve" | "reject";
      note?: string;
    }) => api.decideAgentApproval(id, { decision, note }),
    onSuccess: (decided: AgentApproval) => {
      qc.setQueryData(agentApprovalKeys.detail(wsId, decided.id), decided);
    },
    onSettled: () => {
      // Every list variant, because a decision moves the row between status
      // filters — the pending list it leaves is a different cache entry from the
      // approved list it joins.
      qc.invalidateQueries({ queryKey: agentApprovalKeys.all(wsId) });
    },
  });
}

/** Withdraws a request a person no longer wants decided. */
export function useCancelAgentApproval() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.cancelAgentApproval(id),
    onSuccess: (cancelled: AgentApproval) => {
      qc.setQueryData(agentApprovalKeys.detail(wsId, cancelled.id), cancelled);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: agentApprovalKeys.all(wsId) });
    },
  });
}
