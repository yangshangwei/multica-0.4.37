/**
 * Project mutations. Mirrors the optimistic-patch + event-always-wins pattern
 * of `useUpdateIssue` (data/mutations/issues.ts:276): apply the patch to
 * both list and detail caches up-front, server response or WS event later
 * overwrites with authoritative state.
 *
 * Cache shapes touched:
 *   - projectKeys.list(wsId)      → `Project[]`     (patch in place)
 *   - projectKeys.detail(wsId,id) → `Project`       (replace fully)
 *   - projectKeys.resources(...)  → `ProjectResource[]` (append / filter)
 *
 * No realtime-driven `project:*` updaters exist on web yet (see
 * apps/mobile/CLAUDE.md realtime section) so mobile mirrors the design
 * — mobile-owned ws-updaters live in `data/realtime/project-ws-updaters.ts`
 * and are invoked by `use-projects-realtime.ts` + `use-project-realtime.ts`.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type {
  CreateProjectRequest,
  CreateProjectResourceRequest,
  Project,
  ProjectResource,
  UpdateProjectRequest,
} from "@multica/core/types";
import { api } from "@/data/api";
import { projectKeys } from "@/data/queries/projects";
import { useWorkspaceStore } from "@/data/workspace-store";
import { patchProjectDetail, patchProjectsList } from "../realtime/project-ws-updaters";
import { isProjectAccessDenied, isProjectAccessError, revokeProjectAccess } from "../realtime/project-access";

export function useCreateProject() {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);

  return useMutation({
    mutationFn: (body: CreateProjectRequest) => api.createProject(body, { workspaceId: wsId }),
    onSuccess: (project) => {
      if (isProjectAccessDenied(project.workspace_id)) return;
      // Seed the detail cache so the post-create navigation lands on a
      // populated page (no spinner flash). The list cache gets a prepend
      // — list ordering is server-driven, so a brief out-of-order render
      // is acceptable and corrected by the WS `project:created` event
      // (or the next refetch).
      qc.setQueryData<Project>(projectKeys.detail(project.workspace_id, project.id), project);
      qc.setQueryData<Project[]>(projectKeys.list(project.workspace_id), (old) =>
        old ? [project, ...old.filter((p) => p.id !== project.id)] : [project],
      );
    },
  });
}

export function useUpdateProject(projectId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);

  return useMutation({
    mutationKey: ["updateProject", projectId] as const,
    mutationFn: (patch: UpdateProjectRequest) =>
      api.updateProject(projectId, patch, { workspaceId: wsId }),
    onMutate: async (patch) => {
      const detailKey = projectKeys.detail(wsId, projectId);
      const listKey = projectKeys.list(wsId);
      // Cancel both — a concurrent list refetch can race-overwrite the
      // optimistic patch otherwise (brief stale flash on screen).
      await Promise.all([
        qc.cancelQueries({ queryKey: detailKey }),
        qc.cancelQueries({ queryKey: listKey }),
      ]);

      const prevDetail = qc.getQueryData<Project>(detailKey);
      const prevList = qc.getQueryData<Project[]>(listKey);

      // Description CAS and completion are server-first. In particular, do
      // not stamp a newest-cache revision onto an older editor's draft.
      const optimistic = patch.description === undefined && patch.status === undefined &&
        patch.expected_revision === undefined && patch.expected_description_revision === undefined && patch.status_reason === undefined;
      if (!optimistic) return { prevDetail, prevList, detailKey, listKey, wsId, optimistic };

      if (prevDetail) {
        qc.setQueryData<Project>(detailKey, { ...prevDetail, ...patch });
      }
      qc.setQueryData<Project[]>(listKey, (old) =>
        old
          ? old.map((p) => (p.id === projectId ? { ...p, ...patch } : p))
          : old,
      );

      return { prevDetail, prevList, detailKey, listKey, wsId, optimistic };
    },
    onError: (_err, _vars, ctx) => {
      if (!ctx) return;
      if (ctx.wsId && (isProjectAccessError(_err) || isProjectAccessDenied(ctx.wsId))) {
        revokeProjectAccess(qc, ctx.wsId);
        return;
      }
      if (!ctx.optimistic) return;
      if (ctx.prevDetail !== undefined) {
        qc.setQueryData(ctx.detailKey, ctx.prevDetail);
      }
      if (ctx.prevList !== undefined) {
        qc.setQueryData(ctx.listKey, ctx.prevList);
      }
    },
    onSuccess: (server) => {
      patchProjectDetail(qc, server.workspace_id, server);
      patchProjectsList(qc, server.workspace_id, server);
    },
  });
}

export function useDeleteProject(projectId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);

  return useMutation({
    mutationKey: ["deleteProject", projectId] as const,
    mutationFn: async () => {
      if (!wsId) throw new Error("A workspace is required");
      await api.deleteProject(projectId, { workspaceId: wsId });
      return wsId;
    },
    onSuccess: (workspaceId) => {
      qc.setQueryData<Project[]>(projectKeys.list(workspaceId), (old) =>
        old ? old.filter((p) => p.id !== projectId) : old,
      );
      qc.removeQueries({ queryKey: projectKeys.detail(workspaceId, projectId) });
    },
  });
}

export function useCreateProjectResource(projectId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);

  return useMutation({
    mutationKey: ["createProjectResource", projectId] as const,
    mutationFn: (body: CreateProjectResourceRequest) =>
      api.createProjectResource(projectId, body, { workspaceId: wsId }),
    onSuccess: (resource) => {
      if (isProjectAccessDenied(resource.workspace_id)) return;
      qc.setQueryData<ProjectResource[]>(
        projectKeys.resources(resource.workspace_id, projectId),
        (old) =>
          old
            ? [...old.filter((r) => r.id !== resource.id), resource]
            : [resource],
      );
      // Bump the parent's resource_count so the chip on detail/list
      // increments without a refetch.
      const bumpCount = (p: Project): Project => ({
        ...p,
        resource_count: p.resource_count + 1,
      });
      qc.setQueryData<Project>(
        projectKeys.detail(resource.workspace_id, projectId),
        (old) => (old ? bumpCount(old) : old),
      );
      qc.setQueryData<Project[]>(projectKeys.list(resource.workspace_id), (old) =>
        old
          ? old.map((p) => (p.id === projectId ? bumpCount(p) : p))
          : old,
      );
    },
  });
}

export function useDeleteProjectResource(projectId: string) {
  const qc = useQueryClient();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);

  return useMutation({
    mutationKey: ["deleteProjectResource", projectId] as const,
    mutationFn: async (resourceId: string) => {
      if (!wsId) throw new Error("A workspace is required");
      await api.deleteProjectResource(projectId, resourceId, { workspaceId: wsId });
      return { resourceId, workspaceId: wsId };
    },
    onSuccess: ({ resourceId, workspaceId }) => {
      if (isProjectAccessDenied(workspaceId)) return;
      const key = projectKeys.resources(workspaceId, projectId);
      qc.setQueryData<ProjectResource[]>(key, (old) =>
        old ? old.filter((r) => r.id !== resourceId) : old,
      );
      const dropCount = (p: Project): Project => ({
        ...p,
        resource_count: Math.max(0, p.resource_count - 1),
      });
      qc.setQueryData<Project>(
        projectKeys.detail(workspaceId, projectId),
        (old) => (old ? dropCount(old) : old),
      );
      qc.setQueryData<Project[]>(projectKeys.list(workspaceId), (old) =>
        old
          ? old.map((p) => (p.id === projectId ? dropCount(p) : p))
          : old,
      );
    },
  });
}
