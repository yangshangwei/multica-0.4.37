import { queryOptions, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import { collaborationKeys, protectCollaborationRead, type CollaborationScope } from "./access";

export function collaborationRunHandoffOptions(client: QueryClient, scope: CollaborationScope, issueId: string, runId: string) {
  return queryOptions({
    queryKey: [...collaborationKeys.issue(scope, issueId), "run", runId.toLowerCase(), "handoff"],
    staleTime: 0, retry: false, refetchOnMount: "always", refetchOnWindowFocus: "always", refetchOnReconnect: "always",
    refetchInterval: 15_000, refetchIntervalInBackground: false,
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issueId, async () => {
      const api = getApi();
      const handoff = await api.getCollaborationRunHandoff(issueId, runId, { signal });
      if (handoff === null) await api.getCollaborationIssue(issueId, { signal });
      return handoff;
    }),
  });
}
