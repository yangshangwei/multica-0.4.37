import { queryOptions, type QueryClient } from "@tanstack/react-query";
import { ApiError, getApi } from "../api";
import { CollaborationUnavailableError, collaborationKeys, protectCollaborationRead, type CollaborationScope } from "./access";
import type { CollaborationIssue } from "./schema";

/** Field definitions are an issue projection, never an ambient workspace read. */
export function collaborationFieldOptions(client: QueryClient, scope: CollaborationScope, issue: CollaborationIssue, parentSearch = "") {
  return queryOptions({
    queryKey: [...collaborationKeys.issue(scope, issue.id), "field-options", parentSearch],
    enabled: issue.capabilities.edit === true && issue.write_context != null,
    staleTime: 0, retry: false, refetchOnMount: "always", refetchOnWindowFocus: "always",
    queryFn: ({ signal }) => protectCollaborationRead(client, scope, issue.id, async () => {
      try {
        const options = await getApi().getCollaborationFieldOptions(issue.id, { signal,
          parentSearch: issue.capabilities.edit_source_structure === true ? parentSearch : undefined });
        return options?.source_workspace_id.toLowerCase() === issue.source_workspace.id.toLowerCase() ? options : null;
      } catch (error) {
        if (error instanceof ApiError && [403, 404].includes(error.status)) {
          // Losing edit authority or an optional route is not loss of the
          // independent read basis. Only a current issue denial clears data.
          const current = await getApi().getCollaborationIssue(issue.id, { signal });
          if (!current) throw new CollaborationUnavailableError();
          void client.invalidateQueries({ queryKey: collaborationKeys.issue(scope, issue.id), exact: true });
          throw new CollaborationUnavailableError();
        }
        throw error;
      }
    }),
  });
}
