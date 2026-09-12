import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { projectKeys } from "./queries";
import { useWorkspaceId } from "../hooks";
import { useRecentContextStore } from "../chat/recent-context-store";
import { clearIssueSurfaceViewState } from "../issues/stores/surface-view-store";
import { issueScopeKey } from "../issues/surface/scope";
import { workspaceKeys } from "../workspace/queries";
import type { Project, CreateProjectRequest, UpdateProjectRequest, ListProjectsResponse, ConfigureProjectSquadRequest } from "../types";

export function useConfigureProjectSquad(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    // A scope change must detach the observer instead of replacing the
    // callbacks of an in-flight mutation with another workspace's closures.
    mutationKey: [...projectKeys.all(wsId), "configure-squad"],
    mutationFn: ({ id, ...data }: { id: string } & ConfigureProjectSquadRequest) =>
      api.configureProjectSquad(id, data, { workspaceId: wsId }),
    onSuccess: (project) => {
      qc.setQueryData(projectKeys.detail(wsId, project.id), project);
      qc.setQueryData<ListProjectsResponse>(projectKeys.list(wsId), (old) =>
        old ? { ...old, projects: old.projects.map((item) => item.id === project.id ? project : item) } : old,
      );
    },
    onSettled: (_data, _err, { id }) => Promise.all([
      qc.invalidateQueries({ queryKey: projectKeys.detail(wsId, id) }),
      qc.invalidateQueries({ queryKey: projectKeys.list(wsId) }),
      qc.invalidateQueries({ queryKey: workspaceKeys.squads(wsId) }),
      qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) }),
      qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) }),
    ]),
  });
}

export function useCreateProject() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationKey: [...projectKeys.all(wsId), "create"],
    mutationFn: (data: CreateProjectRequest) => api.createProject(data, { workspaceId: wsId }),
    onSuccess: (newProject) => {
      qc.setQueryData(projectKeys.detail(wsId, newProject.id), newProject);
      qc.setQueryData<ListProjectsResponse>(projectKeys.list(wsId), (old) =>
        old && !old.projects.some((p) => p.id === newProject.id)
          ? { ...old, projects: [...old.projects, newProject], total: old.total + 1 }
          : old,
      );
    },
    onSettled: (_data, _err, variables) => {
      qc.invalidateQueries({ queryKey: projectKeys.list(wsId) });
      if (variables.execution_squad) {
        qc.invalidateQueries({ queryKey: workspaceKeys.squads(wsId) });
        qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) });
        qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) });
      }
    },
  });
}

export function useUpdateProject() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationKey: [...projectKeys.all(wsId), "update"],
    mutationFn: ({ id, ...data }: { id: string } & UpdateProjectRequest) =>
      api.updateProject(id, data, { workspaceId: wsId }),
    onMutate: ({ id, ...data }) => {
      qc.cancelQueries({ queryKey: projectKeys.list(wsId) });
      const prevList = qc.getQueryData<ListProjectsResponse>(projectKeys.list(wsId));
      const prevDetail = qc.getQueryData<Project>(projectKeys.detail(wsId, id));
      qc.setQueryData<ListProjectsResponse>(projectKeys.list(wsId), (old) =>
        old ? { ...old, projects: old.projects.map((p) => (p.id === id ? { ...p, ...data } : p)) } : old,
      );
      qc.setQueryData<Project>(projectKeys.detail(wsId, id), (old) =>
        old ? { ...old, ...data } : old,
      );
      return { prevList, prevDetail, id };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prevList) qc.setQueryData(projectKeys.list(wsId), ctx.prevList);
      if (ctx?.prevDetail) qc.setQueryData(projectKeys.detail(wsId, ctx.id), ctx.prevDetail);
    },
    onSettled: (_data, _err, vars) => {
      qc.invalidateQueries({ queryKey: projectKeys.detail(wsId, vars.id) });
      qc.invalidateQueries({ queryKey: projectKeys.list(wsId) });
    },
  });
}

export function useDeleteProject() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.deleteProject(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: projectKeys.list(wsId) });
      const prevList = qc.getQueryData<ListProjectsResponse>(projectKeys.list(wsId));
      qc.setQueryData<ListProjectsResponse>(projectKeys.list(wsId), (old) =>
        old ? { ...old, projects: old.projects.filter((p) => p.id !== id), total: old.total - 1 } : old,
      );
      qc.removeQueries({ queryKey: projectKeys.detail(wsId, id) });
      return { prevList };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prevList) qc.setQueryData(projectKeys.list(wsId), ctx.prevList);
    },
    onSuccess: (_data, id) => {
      useRecentContextStore.getState().forgetContext(wsId, { type: "project", id });
      clearIssueSurfaceViewState(issueScopeKey({ type: "project", projectId: id }));
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: projectKeys.list(wsId) });
    },
  });
}
