import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { clearProjectDescriptionDrafts } from "./description-draft-store";
import { clearProjectProgressDrafts } from "./progress-draft-store";
import { canAccessProject, protectProjectRequest, beginProjectDelete } from "./access";
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
      protectProjectRequest(qc, wsId, id, () => api.configureProjectSquad(id, data, { workspaceId: wsId })),
    onSuccess: (project) => {
      if (!canAccessProject(wsId, project.id)) return;
      qc.setQueryData(projectKeys.detail(wsId, project.id), project);
      qc.setQueryData<ListProjectsResponse>(projectKeys.list(wsId), (old) =>
        old ? { ...old, projects: old.projects.map((item) => item.id === project.id ? project : item) } : old,
      );
    },
    onSettled: (_data, _err, { id }) => canAccessProject(wsId, id) ? Promise.all([
      qc.invalidateQueries({ queryKey: projectKeys.detail(wsId, id) }),
      qc.invalidateQueries({ queryKey: projectKeys.list(wsId) }),
      qc.invalidateQueries({ queryKey: workspaceKeys.squads(wsId) }),
      qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) }),
      qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) }),
    ]) : undefined,
  });
}

export function useConfigureProjectSquads(wsId: string) {
  const qc = useQueryClient();
  return useMutation({
    // A scope change must detach the observer instead of replacing the
    // callbacks of an in-flight mutation with another workspace's closures.
    mutationKey: [...projectKeys.all(wsId), "configure-squads"],
    mutationFn: ({ id, squads }: { id: string; squads: ConfigureProjectSquadRequest[] }) =>
      protectProjectRequest(qc, wsId, id, () => api.configureProjectSquads(id, squads, { workspaceId: wsId })),
    onSuccess: (project) => {
      if (!canAccessProject(wsId, project.id)) return;
      qc.setQueryData(projectKeys.detail(wsId, project.id), project);
      qc.setQueryData<ListProjectsResponse>(projectKeys.list(wsId), (old) =>
        old ? { ...old, projects: old.projects.map((item) => item.id === project.id ? project : item) } : old,
      );
    },
    onSettled: (_data, _err, { id }) => canAccessProject(wsId, id) ? Promise.all([
      qc.invalidateQueries({ queryKey: projectKeys.detail(wsId, id) }),
      qc.invalidateQueries({ queryKey: projectKeys.list(wsId) }),
      qc.invalidateQueries({ queryKey: workspaceKeys.squads(wsId) }),
      qc.invalidateQueries({ queryKey: workspaceKeys.agents(wsId) }),
      qc.invalidateQueries({ queryKey: workspaceKeys.skills(wsId) }),
    ]) : undefined,
  });
}

export function useCreateProject() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationKey: [...projectKeys.all(wsId), "create"],
    mutationFn: (data: CreateProjectRequest) => protectProjectRequest(qc, wsId, undefined, () => api.createProject(data, { workspaceId: wsId })),
    onSuccess: (newProject) => {
      if (!canAccessProject(wsId, newProject.id)) return;
      qc.setQueryData(projectKeys.detail(wsId, newProject.id), newProject);
      qc.setQueryData<ListProjectsResponse>(projectKeys.list(wsId), (old) =>
        old && !old.projects.some((p) => p.id === newProject.id)
          ? { ...old, projects: [...old.projects, newProject], total: old.total + 1 }
          : old,
      );
    },
    onSettled: (_data, _err, variables) => {
      if (!canAccessProject(wsId)) return;
      qc.invalidateQueries({ queryKey: projectKeys.list(wsId) });
      if (variables.execution_squad || variables.execution_squads?.length) {
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
      protectProjectRequest(qc, wsId, id, () => api.updateProject(id, data, { workspaceId: wsId })),
    onMutate: ({ id, ...data }) => {
      if (!canAccessProject(wsId, id)) return undefined;
      if ("description" in data || "status" in data || "expected_revision" in data) return undefined;
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
    onError: (_error, vars, ctx) => {
      if (!canAccessProject(wsId, vars.id)) return;
      if (ctx?.prevList) qc.setQueryData(projectKeys.list(wsId), ctx.prevList);
      if (ctx?.prevDetail) qc.setQueryData(projectKeys.detail(wsId, ctx.id), ctx.prevDetail);
    },
    onSuccess: (project) => {
      if (!canAccessProject(wsId, project.id)) return;
      qc.setQueryData(projectKeys.detail(wsId, project.id), project);
    },
    onSettled: (_data, _err, vars) => {
      if (!canAccessProject(wsId, vars.id)) return;
      qc.invalidateQueries({ queryKey: projectKeys.detail(wsId, vars.id) });
      qc.invalidateQueries({ queryKey: projectKeys.list(wsId) });
    },
  });
}

export function useDeleteProject() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationKey: [...projectKeys.all(wsId), "delete"],
    mutationFn: async (id: string) => {
      const finish = beginProjectDelete(wsId, id);
      try { return await protectProjectRequest(qc, wsId, id, () => api.deleteProject(id, { workspaceId: wsId })); }
      finally { finish(); }
    },
    onSuccess: async (_data, id) => {
      if (!canAccessProject(wsId, id)) return;
      await qc.cancelQueries({ queryKey: projectKeys.all(wsId) });
      qc.setQueryData<ListProjectsResponse>(projectKeys.list(wsId), (old) => old ? {
        ...old, projects: old.projects.filter((p) => p.id !== id),
        total: Math.max(0, old.total - (old.projects.some((p) => p.id === id) ? 1 : 0)),
      } : old);
      qc.removeQueries({ queryKey: projectKeys.detail(wsId, id) });
      clearProjectProgressDrafts(wsId, id);
      clearProjectDescriptionDrafts(wsId, id);
      qc.invalidateQueries({ queryKey: ["issues", wsId] });
      useRecentContextStore.getState().forgetContext(wsId, { type: "project", id });
      clearIssueSurfaceViewState(issueScopeKey({ type: "project", projectId: id }));
    },
    onSettled: () => {
      if (!canAccessProject(wsId)) return;
      qc.invalidateQueries({ queryKey: projectKeys.list(wsId) });
    },
  });
}
