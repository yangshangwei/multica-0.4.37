import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import { withQueryAccessBoundary } from "../query-access";

export const vcsKeys = {
  all: (wsId: string) => ["vcs", wsId] as const,
  connections: (wsId: string) => [...vcsKeys.all(wsId), "connections"] as const,
};

export const vcsConnectionsOptions = (wsId: string) =>
  queryOptions({
    queryKey: vcsKeys.connections(wsId),
    queryFn: withQueryAccessBoundary(() => api.listVCSConnections(wsId)),
    enabled: !!wsId,
  });
