import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { canAccessProject, protectProjectRequest } from "../projects/access";
import { materialKeys } from "./queries";
import type { MaterialInitialization, MaterialInitializationCommand } from "./initialization-schema";

export const materialInitializationOptions = (wsId: string, projectId: string) => queryOptions({
  queryKey: materialKeys.initialization(wsId, projectId),
  queryFn: ({ signal, client }) => protectProjectRequest(client, wsId, projectId, () => api.getMaterialInitialization(projectId, { workspaceId: wsId, signal })),
  enabled: !!wsId && !!projectId && canAccessProject(wsId, projectId), retry: false, refetchInterval: 5000,
});
export function useMaterialInitializationCommand(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({
    retry: false, networkMode: "always", mutationKey: [...materialKeys.initialization(wsId, projectId), "write"],
    mutationFn: (command: MaterialInitializationCommand) => protectProjectRequest(qc, wsId, projectId, () => api.sendMaterialInitializationCommand(projectId, command, { workspaceId: wsId })),
    onSuccess: (result) => {
      if (canAccessProject(wsId, projectId)) qc.setQueryData<MaterialInitialization>(materialKeys.initialization(wsId, projectId),
        old => old && old.version > result.version ? old : result);
    },
    onSettled: () => { if (canAccessProject(wsId, projectId)) void qc.invalidateQueries({ queryKey: materialKeys.all(wsId, projectId) }); },
  });
}
