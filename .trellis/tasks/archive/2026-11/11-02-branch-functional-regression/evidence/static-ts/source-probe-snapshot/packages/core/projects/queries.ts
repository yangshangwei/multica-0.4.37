import { infiniteQueryOptions, queryOptions, type QueryKey } from "@tanstack/react-query";
import { protectProjectRequest } from "./access";
import { api, ApiError } from "../api";
import { isIssueUuid } from "../issues/canonical-id";
import { isFormalAdmission } from "../triage";
import type { Issue } from "../types";

export const projectKeys = {
  all: (wsId: string) => ["projects", wsId] as const,
  list: (wsId: string) => [...projectKeys.all(wsId), "list"] as const,
  issueCandidates: (wsId: string) => [...projectKeys.all(wsId), "issue-candidates"] as const,
  detail: (wsId: string, id: string) =>
    [...projectKeys.all(wsId), "detail", id] as const,
};

export function projectListOptions(wsId: string) {
  return queryOptions({
    queryKey: projectKeys.list(wsId),
    queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, undefined, () => api.listProjects(undefined, { workspaceId: wsId, signal })),
    select: (data) => data.projects,
  });
}

export function projectDetailOptions(wsId: string, id: string) {
  return queryOptions({
    queryKey: projectKeys.detail(wsId, id),
    queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, id, () => api.getProject(id, { workspaceId: wsId, signal })),
  });
}

export function projectIssueCandidatesOptions(wsId: string, search: string) {
  const q = search.trim();
  const isReference = /^[a-z\d]{1,10}-[1-9]\d*$/i.test(q) || isIssueUuid(q);
  const canLink = (issue: Issue) => issue.workspace_id === wsId && issue.project_id === null &&
    isFormalAdmission(issue.admission_status);
  return infiniteQueryOptions({
    queryKey: [...projectKeys.issueCandidates(wsId), q] as QueryKey,
    initialPageParam: 0,
    queryFn: async ({ pageParam, signal }): Promise<{ issues: Issue[]; nextOffset: number | undefined }> => {
      if (isReference) {
        try {
          const issue = await api.getIssue(q, { workspaceId: wsId, signal });
          return { issues: canLink(issue) ? [issue] : [], nextOffset: undefined };
        } catch (error) {
          if (error instanceof ApiError && error.status === 404)
            return { issues: [], nextOffset: undefined };
          throw error;
        }
      }
      const page = await api.listIssues({
        workspace_id: wsId,
        q,
        include_no_project: true,
        limit: 30,
        offset: pageParam,
        sort_by: "created_at",
        sort_direction: "desc",
      });
      // Advance by the server window, even if an older backend ignored a facet.
      const nextOffset = pageParam + page.issues.length;
      return {
        issues: page.issues.filter(canLink),
        nextOffset: page.issues.length > 0 && nextOffset < page.total ? nextOffset : undefined,
      };
    },
    getNextPageParam: (page) => page.nextOffset,
    enabled: wsId.length > 0,
    staleTime: 0,
    retry: false,
  });
}
