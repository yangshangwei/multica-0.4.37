import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { protectProjectRequest, canAccessProject } from "./access";
import { projectKeys } from "./queries";
import type { ProjectUpdateDraft, ProjectUpdateWriteInput } from "../types/project-p1";
export function usePreviewProjectUpdate(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({ mutationKey: [...projectKeys.detail(wsId, projectId), "preview"],
    mutationFn: (draft: ProjectUpdateDraft) => protectProjectRequest(qc, wsId, projectId, () => api.previewProjectUpdate(wsId, projectId, draft)) });
}
export function usePublishProjectUpdate(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({ mutationKey: [...projectKeys.detail(wsId, projectId), "publish"],
    mutationFn: (input: ProjectUpdateWriteInput) => protectProjectRequest(qc, wsId, projectId, () => input.draft.operation === "correct" && input.draft.update_id
      ? api.correctProjectUpdate(wsId, projectId, input.draft.update_id, input)
      : api.createProjectUpdate(wsId, projectId, input)),
    onSuccess: () => canAccessProject(wsId, projectId) ? qc.invalidateQueries({ queryKey: projectKeys.all(wsId) }) : undefined,
  });
}
export function useProjectPlanningTimezone(wsId: string) {
  const qc = useQueryClient();
  return useMutation({ mutationKey: [...projectKeys.all(wsId), "timezone-write"],
    mutationFn: (timezone: string | null) => protectProjectRequest(qc, wsId, undefined, () => api.updateProjectPlanningTimezone(wsId, timezone)),
    onSuccess: () => canAccessProject(wsId) ? qc.invalidateQueries({ queryKey: projectKeys.all(wsId) }) : undefined });
}
