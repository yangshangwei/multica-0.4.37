import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { canAccessProject, protectProjectRequest } from "../projects/access";
import { projectKeys } from "../projects/queries";
import type { MaterialCommand, MaterialDetail, MaterialSettingsEnvelope, MaterialSettingsInput } from "./schema";

export const materialKeys = {
  all: (wsId: string, projectId: string) => [...projectKeys.detail(wsId, projectId), "materials"] as const,
  settings: (wsId: string, projectId: string) => [...materialKeys.all(wsId, projectId), "settings"] as const,
  list: (wsId: string, projectId: string) => [...materialKeys.all(wsId, projectId), "list"] as const,
  detail: (wsId: string, projectId: string, materialId: string) => [...materialKeys.all(wsId, projectId), "detail", materialId] as const,
};
export const materialSettingsOptions = (wsId: string, projectId: string) => queryOptions({
  queryKey: materialKeys.settings(wsId, projectId),
  queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, projectId, () => api.getMaterialSettings(projectId, { workspaceId: wsId, signal })),
  enabled: !!wsId && !!projectId && canAccessProject(wsId, projectId), retry: false,
});
export const materialListOptions = (wsId: string, projectId: string) => queryOptions({
  queryKey: materialKeys.list(wsId, projectId),
  queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, projectId, () => api.listMaterials(projectId, { workspaceId: wsId, signal })),
  enabled: !!wsId && !!projectId && canAccessProject(wsId, projectId), retry: false, refetchInterval: 15000,
});
export const materialDetailOptions = (wsId: string, projectId: string, materialId: string) => queryOptions({
  queryKey: materialKeys.detail(wsId, projectId, materialId),
  queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, projectId, () => api.getMaterial(projectId, materialId, { workspaceId: wsId, signal })),
  enabled: !!wsId && !!projectId && !!materialId && canAccessProject(wsId, projectId), retry: false, refetchInterval: 5000,
});
export function useSaveMaterialSettings(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    retry: false, networkMode: "always",
    mutationKey: [...materialKeys.all(wsId, projectId), "save-settings"],
    mutationFn: (data: MaterialSettingsInput) => protectProjectRequest(qc, wsId, projectId, () => api.saveMaterialSettings(projectId, data, { workspaceId: wsId })),
    onSuccess: (result) => {
      if (canAccessProject(wsId, projectId)) qc.setQueryData<MaterialSettingsEnvelope>(materialKeys.settings(wsId, projectId),
        (old) => (old?.settings?.version ?? 0) > (result.settings?.version ?? 0) ? old : result);
    },
    onSettled: () => { if (canAccessProject(wsId, projectId)) void qc.invalidateQueries({ queryKey: materialKeys.all(wsId, projectId) }); },
  });
}
export function useCreateMaterial(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    retry: false, networkMode: "always",
    mutationKey: [...materialKeys.all(wsId, projectId), "create"],
    mutationFn: (data: { request_id: string; title: string }) => protectProjectRequest(qc, wsId, projectId, () => api.createMaterial(projectId, data, { workspaceId: wsId })),
    onSuccess: (result) => { if (canAccessProject(wsId, projectId)) qc.setQueryData(materialKeys.detail(wsId, projectId, result.material.id), result); },
    onSettled: () => { if (canAccessProject(wsId, projectId)) void qc.invalidateQueries({ queryKey: materialKeys.list(wsId, projectId) }); },
  });
}
export function useMaterialCommand(wsId: string, projectId: string, materialId: string) {
  const qc = useQueryClient();
  return useMutation({
    retry: false, networkMode: "always",
    mutationKey: [...materialKeys.detail(wsId, projectId, materialId), "write"],
    mutationFn: (command: MaterialCommand) => protectProjectRequest(qc, wsId, projectId, () => api.sendMaterialCommand(projectId, materialId, command, { workspaceId: wsId })),
    onSuccess: (result) => {
      if (!canAccessProject(wsId, projectId)) return;
      qc.setQueryData<MaterialDetail>(materialKeys.detail(wsId, projectId, materialId), (old) => old && old.material.version > result.material.version ? old : result);
    },
    onSettled: () => { if (canAccessProject(wsId, projectId)) void qc.invalidateQueries({ queryKey: materialKeys.all(wsId, projectId) }); },
  });
}
