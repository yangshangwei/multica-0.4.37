import { queryOptions, useQuery, type QueryClient } from "@tanstack/react-query";
import { getApi } from "../api";
import type { CollaborationScope } from "./access";
import { knowledgeScopeKey, readKnowledgeProjection } from "./knowledge-cache";
import { knowledgeQueryFreshness, useKnowledgeLifecycle } from "./knowledge-queries";

export const knowledgeAutomationKeys = {
  policy: (scope: CollaborationScope, workspaceId: string, projectId: string) => [...knowledgeScopeKey(scope), "policy", workspaceId, projectId] as const,
  jobs: (scope: CollaborationScope, issueId: string) => [...knowledgeScopeKey(scope), "jobs", issueId] as const,
};

export function projectKnowledgePolicyOptions(client: QueryClient, scope: CollaborationScope, workspaceId: string, projectId: string) {
  const key = knowledgeAutomationKeys.policy(scope, workspaceId, projectId);
  return queryOptions({ queryKey: key, ...knowledgeQueryFreshness,
    queryFn: ({ signal }) => readKnowledgeProjection(client, scope, key, key,
      () => getApi().getProjectKnowledgePolicy(workspaceId, projectId, { signal })),
  });
}

export function knowledgeAutomationJobsOptions(client: QueryClient, scope: CollaborationScope, issueId: string) {
  const key = knowledgeAutomationKeys.jobs(scope, issueId);
  return queryOptions({ queryKey: key, ...knowledgeQueryFreshness,
    queryFn: ({ signal }) => readKnowledgeProjection(client, scope, key, key,
      () => getApi().getCollaborationKnowledgeJobs(issueId, { signal }), undefined, issueId),
    refetchInterval: (query) => {
      const interval = knowledgeQueryFreshness.refetchInterval(query);
      if (interval === false) return false;
      return query.state.data?.value.items.some((job) => job.source_current && ["queued", "generating", "checking", "settling"].includes(job.phase)) ? 3000 : interval;
    },
  });
}

export function useProjectKnowledgePolicy(scope: CollaborationScope, workspaceId: string, projectId: string) {
  const client = useKnowledgeLifecycle(scope);
  return useQuery({ ...projectKnowledgePolicyOptions(client, scope, workspaceId, projectId), enabled: scope.userId !== "" });
}

export function useKnowledgeAutomationJobs(scope: CollaborationScope, issueId: string, enabled = true) {
  const client = useKnowledgeLifecycle(scope);
  return useQuery({ ...knowledgeAutomationJobsOptions(client, scope, issueId), enabled: enabled && scope.userId !== "" });
}
