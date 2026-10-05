import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { protectProjectRequest, canAccessProject } from "./access";
import { projectKeys } from "./queries";
import type {
  CreateProjectResourceRequest,
  ListProjectResourcesResponse,
  ProjectResource,
  UpdateProjectResourceRequest,
} from "../types";

export const projectResourceKeys = {
  list: (wsId: string, projectId: string) =>
    [...projectKeys.detail(wsId, projectId), "resources"] as const,
};

export function projectResourcesOptions(wsId: string, projectId: string) {
  return queryOptions({
    queryKey: projectResourceKeys.list(wsId, projectId),
    queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, projectId, () => api.listProjectResources(projectId, { workspaceId: wsId, signal })),
    select: (data) => data.resources,
  });
}

export function useCreateProjectResource(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationKey: [...projectKeys.detail(wsId, projectId), "resource-write"],
    mutationFn: (data: CreateProjectResourceRequest) =>
      protectProjectRequest(qc, wsId, projectId, () => api.createProjectResource(projectId, data)),
    onSuccess: (created) => {
      if (!canAccessProject(wsId, projectId)) return;
      qc.setQueryData<ListProjectResourcesResponse>(
        projectResourceKeys.list(wsId, projectId),
        (old) =>
          old && !old.resources.some((r) => r.id === created.id)
            ? {
                ...old,
                resources: [...old.resources, created],
                total: old.total + 1,
              }
            : old,
      );
    },
    onSettled: () => {
      if (!canAccessProject(wsId, projectId)) return;
      qc.invalidateQueries({
        queryKey: projectResourceKeys.list(wsId, projectId),
      });
    },
  });
}

export function useUpdateProjectResource(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationKey: [...projectKeys.detail(wsId, projectId), "resource-write"],
    mutationFn: ({
      resourceId,
      data,
    }: {
      resourceId: string;
      data: UpdateProjectResourceRequest;
    }) => protectProjectRequest(qc, wsId, projectId, () => api.updateProjectResource(projectId, resourceId, data)),
    onSuccess: (updated) => {
      if (!canAccessProject(wsId, projectId)) return;
      qc.setQueryData<ListProjectResourcesResponse>(
        projectResourceKeys.list(wsId, projectId),
        (old) =>
          old
            ? {
                ...old,
                resources: old.resources.map((r) =>
                  r.id === updated.id ? updated : r,
                ),
              }
            : old,
      );
    },
    onSettled: () => {
      if (!canAccessProject(wsId, projectId)) return;
      qc.invalidateQueries({
        queryKey: projectResourceKeys.list(wsId, projectId),
      });
    },
  });
}

export function useDeleteProjectResource(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationKey: [...projectKeys.detail(wsId, projectId), "resource-write"],
    mutationFn: (resourceId: string) =>
      protectProjectRequest(qc, wsId, projectId, () => api.deleteProjectResource(projectId, resourceId)),
    onMutate: async (resourceId) => {
      if (!canAccessProject(wsId, projectId)) return;
      await qc.cancelQueries({
        queryKey: projectResourceKeys.list(wsId, projectId),
      });
      const prev = qc.getQueryData<ListProjectResourcesResponse>(
        projectResourceKeys.list(wsId, projectId),
      );
      qc.setQueryData<ListProjectResourcesResponse>(
        projectResourceKeys.list(wsId, projectId),
        (old) =>
          old
            ? {
                ...old,
                resources: old.resources.filter(
                  (r: ProjectResource) => r.id !== resourceId,
                ),
                total: old.total - 1,
              }
            : old,
      );
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (!canAccessProject(wsId, projectId)) return;
      if (ctx?.prev) {
        qc.setQueryData(projectResourceKeys.list(wsId, projectId), ctx.prev);
      }
    },
    onSettled: () => {
      if (!canAccessProject(wsId, projectId)) return;
      qc.invalidateQueries({
        queryKey: projectResourceKeys.list(wsId, projectId),
      });
    },
  });
}
