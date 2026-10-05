import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { projectKeys } from "./queries";
import type { ProjectUpdateDraft, ProjectUpdateWriteInput } from "../types/project-p1";
export function usePreviewProjectUpdate(wsId: string, projectId: string) {
  return useMutation({ mutationKey: [...projectKeys.detail(wsId, projectId), "preview"],
    mutationFn: (draft: ProjectUpdateDraft) => api.previewProjectUpdate(wsId, projectId, draft) });
}
export function usePublishProjectUpdate(wsId: string, projectId: string) {
  const qc = useQueryClient();
  return useMutation({ mutationKey: [...projectKeys.detail(wsId, projectId), "publish"],
    mutationFn: (input: ProjectUpdateWriteInput) => input.draft.operation === "correct" && input.draft.update_id
      ? api.correctProjectUpdate(wsId, projectId, input.draft.update_id, input)
      : api.createProjectUpdate(wsId, projectId, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: projectKeys.all(wsId) }),
  });
}
export function useProjectPlanningTimezone(wsId: string) {
  const qc = useQueryClient();
  return useMutation({ mutationKey: [...projectKeys.all(wsId), "timezone-write"],
    mutationFn: (timezone: string | null) => api.updateProjectPlanningTimezone(wsId, timezone),
    onSuccess: () => qc.invalidateQueries({ queryKey: projectKeys.all(wsId) }) });
}
