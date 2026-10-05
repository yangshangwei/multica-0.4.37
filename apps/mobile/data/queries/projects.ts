/**
 * Workspace project queries. Three query shapes:
 *
 *   - List       (projectKeys.list)       — `Project[]`
 *   - Detail     (projectKeys.detail)     — `Project`
 *   - Resources  (projectKeys.resources)  — `ProjectResource[]` (per project)
 *
 * Detail and Resources are workspace-scoped via the `wsId` segment so
 * switching workspaces flips the cache without manual invalidate, per the
 * root CLAUDE.md "Workspace-scoped queries must key on wsId" rule.
 *
 * Issues belonging to a project are NOT a project query — they live under
 * `issueKeys.list(wsId, { project_id })` and reuse the issues cache shape.
 * See `projectIssuesOptions` below for the binding helper.
 */
import { queryOptions } from "@tanstack/react-query";
import type { Project } from "@multica/core/types";
import { api } from "@/data/api";
import { issueKeys } from "@/data/queries/issue-keys";
import { protectProjectQuery } from "../realtime/project-access";

export const projectKeys = {
  all: (wsId: string | null) => ["projects", wsId] as const,
  list: (wsId: string | null) => [...projectKeys.all(wsId), "list"] as const,
  detail: (wsId: string | null, id: string) =>
    [...projectKeys.all(wsId), "detail", id] as const,
  resources: (wsId: string | null, id: string) =>
    [...projectKeys.all(wsId), "detail", id, "resources"] as const,
};

export const projectListOptions = (wsId: string | null) =>
  queryOptions({
    queryKey: projectKeys.list(wsId),
    queryFn: ({ signal, client }) => protectProjectQuery(client, wsId, async () => {
      const res = await api.listProjects({ signal, workspaceId: wsId });
      return res.projects;
    }),
    enabled: !!wsId,
  });

export const projectDetailOptions = (wsId: string | null, id: string) =>
  queryOptions({
    queryKey: projectKeys.detail(wsId, id),
    queryFn: ({ signal, client }) => protectProjectQuery(client, wsId, () => api.getProject(id, { signal, workspaceId: wsId })),
    enabled: !!wsId && !!id,
  });

export const projectResourcesOptions = (wsId: string | null, id: string) =>
  queryOptions({
    queryKey: projectKeys.resources(wsId, id),
    queryFn: ({ signal, client }) => protectProjectQuery(client, wsId, async () => {
      const res = await api.listProjectResources(id, { signal, workspaceId: wsId });
      return res.resources;
    }),
    enabled: !!wsId && !!id,
  });

/**
 * Issues filtered by `project_id`. Lives under the issues cache prefix
 * (not the projects one) so a WS `issue:*` event invalidating
 * `issueKeys.list(wsId)` also refreshes this list — single source of
 * truth for issue caches.
 */
export const projectIssuesOptions = (wsId: string | null, projectId: string) =>
  queryOptions({
    queryKey: [
      ...issueKeys.list(wsId),
      "byProject",
      projectId,
    ] as const,
    queryFn: ({ signal, client }) => protectProjectQuery(client, wsId, async () => {
      const res = await api.listIssues(
        { project_id: projectId },
        { signal, workspaceId: wsId },
      );
      if (res.issues.some((issue) => issue.workspace_id !== wsId || issue.project_id !== projectId)) throw new Error("Invalid project issue identity");
      return res.issues;
    }),
    enabled: !!wsId && !!projectId,
  });

/**
 * Helper for the read-only project chip — returns the project matching id,
 * or undefined. Caller selects from the list query and looks up by id.
 */
export function findProject(
  projects: Project[],
  id: string | null,
): Project | undefined {
  if (!id) return undefined;
  return projects.find((p) => p.id === id);
}
