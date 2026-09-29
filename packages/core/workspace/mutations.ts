import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { Workspace } from "../types";
import { api } from "../api";
import { defaultStorage } from "../platform/storage";
import { clearWorkspaceStorage } from "../platform/storage-cleanup";
import { workspaceKeys } from "./queries";
import {
  markWorkspaceDeletePending,
  unmarkWorkspaceDeletePending,
} from "./pending-delete";

export function useCreateWorkspace() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: {
      name: string;
      slug: string;
      description?: string;
      /** Omit to let the server derive it from the slug. */
      issue_prefix?: string;
    }) => api.createWorkspace(data),
    // Seed the workspace list cache BEFORE callers navigate to /{newWs.slug}/issues.
    // The destination [workspaceSlug]/layout queries by slug from this cache;
    // without seeding, it would briefly show "loading" before the background
    // invalidation completes. TanStack Query guarantees this onSuccess runs
    // before mutateAsync's resolver / before any callback-style onSuccess
    // passed to mutate(), so any caller that navigates after the mutation
    // resolves will see the seeded data synchronously. Switching workspaces
    // is pure navigation now — no imperative store writes needed.
    onSuccess: (newWs) => {
      qc.setQueryData(workspaceKeys.list(), (old: Workspace[] = []) => [...old, newWs]);
    },
    // A failed create should not have changed server state, but invalidate the
    // list once to reconcile ambiguous transport failures where the server may
    // have committed before the client lost the response. Keep this off the
    // success path so the canonical response seeded above is not fetched again.
    onError: () => {
      qc.invalidateQueries({ queryKey: workspaceKeys.list() });
    },
  });
}

export function useLeaveWorkspace() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (workspaceId: string) => api.leaveWorkspace(workspaceId),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: workspaceKeys.list() });
    },
  });
}

export function useDeleteWorkspace() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (workspaceId: string) => api.deleteWorkspace(workspaceId),
    // No optimistic removal: the delete flow awaits this mutation with the
    // confirm dialog in a loading state and only navigates on success, so
    // the cache staying truthful (server still has the row until commit)
    // is correct, and a failed DELETE needs no rollback.
    onMutate: (workspaceId) => {
      // Mark the delete as self-initiated so the realtime `workspace:deleted`
      // handler no-ops instead of racing this flow's navigation with its own
      // full-page relocate. See pending-delete.ts for lifetime rules.
      markWorkspaceDeletePending(workspaceId);
      // Capture the slug for onSuccess's storage cleanup — cheap here, and
      // the row is guaranteed to still be in the list pre-mutation.
      const slug = qc
        .getQueryData<Workspace[]>(workspaceKeys.list())
        ?.find((w) => w.id === workspaceId)?.slug;
      return { slug };
    },
    // Success is the only path that clears the deleted workspace's persisted
    // `${key}:${slug}` namespace — a failed DELETE means the workspace still
    // exists and its drafts/view state must survive. The realtime handler
    // skips self-initiated deletes, so cleanup has to happen here.
    onSuccess: (_data, _workspaceId, ctx) => {
      if (ctx?.slug) clearWorkspaceStorage(defaultStorage, ctx.slug);
    },
    // The workspace still exists after a failed DELETE, so a later external
    // delete of the same ID must be handled by the realtime handler again.
    onError: (_err, workspaceId) => {
      unmarkWorkspaceDeletePending(workspaceId);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: workspaceKeys.list() });
    },
  });
}

/**
 * Adds a server to the workspace library. It is assigned to no agent: an
 * agent owner gives it to their agent separately.
 */
export function useCreateWorkspaceMcpServer(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, config }: { name: string; config: Record<string, unknown> }) =>
      api.createWorkspaceMcpServer(wsId, name, config),
    onSettled: () =>
      queryClient.invalidateQueries({ queryKey: workspaceKeys.mcpServers(wsId) }),
  });
}

/** Creates a trusted template copy; creation itself never assigns an agent. */
export function useCreateWorkspaceMcpServerFromTemplate(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, templateKey, templateVersion }: {
      name: string;
      templateKey: string;
      templateVersion: string;
    }) => api.createWorkspaceMcpServerFromTemplate(wsId, name, templateKey, templateVersion),
    onSettled: () =>
      queryClient.invalidateQueries({ queryKey: workspaceKeys.mcpServers(wsId) }),
  });
}

export interface McpAssignmentResult {
  succeeded: string[];
  failed: { agentId: string; message: string }[];
}

/** Additive assignment with per-agent confirmation and retry-safe existing bindings. */
export function useAssignWorkspaceMcpServer(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ serverId, agentIds }: { serverId: string; agentIds: string[] }): Promise<McpAssignmentResult> => {
      const ids = [...new Set(agentIds)];
      const outcomes = await Promise.allSettled(ids.map(async (agentId) => {
        const existing = await api.listAgentMcpServers(agentId, { workspaceId: wsId });
        const containsServer = (servers: typeof existing) => servers.some(
          (server) => server.id === serverId && server.workspace_id === wsId,
        );
        // An existing disabled binding is still assigned. Do not change its toggle.
        if (containsServer(existing)) return;
        const assigned = await api.addAgentMcpServer(agentId, serverId, { workspaceId: wsId });
        if (!containsServer(assigned)) throw new Error("MCP assignment could not be confirmed. Refresh and retry.");
      }));
      const result: McpAssignmentResult = { succeeded: [], failed: [] };
      outcomes.forEach((outcome, index) => {
        const agentId = ids[index]!;
        if (outcome.status === "fulfilled") result.succeeded.push(agentId);
        else result.failed.push({
          agentId,
          message: outcome.reason instanceof Error ? outcome.reason.message : "MCP assignment failed",
        });
      });
      return result;
    },
    onSettled: (_result, _error, { agentIds }) => Promise.all(
      [...new Set(agentIds)].map((agentId) => queryClient.invalidateQueries({
        queryKey: ["agents", agentId, "mcp-servers"],
      })),
    ),
  });
}

export function useUpdateWorkspaceMcpServer(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ serverId, ...update }: {
      serverId: string;
      name?: string;
      config?: Record<string, unknown>;
    }) => api.updateWorkspaceMcpServer(wsId, serverId, update),
    onSettled: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: workspaceKeys.mcpServers(wsId) }),
        // Assignments include the library entry's name and transport, so every
        // agent's cached projection may change when the entry is updated.
        queryClient.invalidateQueries({ queryKey: ["agents"] }),
      ]),
  });
}

export function useDeleteWorkspaceMcpServer(wsId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (serverId: string) => api.deleteWorkspaceMcpServer(wsId, serverId),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: workspaceKeys.mcpServers(wsId) });
      // Deleting a library entry drops it from every agent that had it, so
      // no agent's assignment list can be trusted afterwards.
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    },
  });
}

/**
 * Assignment writes all return the agent's resulting list, so the cache is
 * updated from the server's answer rather than a guess.
 */
function useAgentMcpMutation<TVariables>(
  agentId: string,
  mutationFn: (variables: TVariables) => Promise<unknown>,
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn,
    onSettled: () =>
      queryClient.invalidateQueries({ queryKey: ["agents", agentId, "mcp-servers"] }),
  });
}

export function useAddAgentMcpServer(agentId: string) {
  return useAgentMcpMutation(agentId, (serverId: string) =>
    api.addAgentMcpServer(agentId, serverId));
}

export function useSetAgentMcpServerEnabled(agentId: string) {
  return useAgentMcpMutation(agentId, ({ serverId, enabled }: { serverId: string; enabled: boolean }) =>
    api.setAgentMcpServerEnabled(agentId, serverId, enabled));
}

export function useRemoveAgentMcpServer(agentId: string) {
  return useAgentMcpMutation(agentId, (serverId: string) =>
    api.removeAgentMcpServer(agentId, serverId));
}
