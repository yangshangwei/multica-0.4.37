import { create } from "zustand";
import type { QueryClient } from "@tanstack/react-query";
import { ApiError } from "../api";
import { projectKeys } from "./queries";
import { clearProjectDescriptionDrafts } from "./description-draft-store";
import { clearProjectProgressDrafts } from "./progress-draft-store";
export const useProjectAccessStore = create<{ denied: Record<string, true> }>(() => ({ denied: {} }));

export function isProjectAccessLost(error: unknown): boolean {
  return error instanceof ApiError && [401, 403, 404].includes(error.status);
}
export function clearProtectedProjectContent(qc: QueryClient, wsId: string, projectId?: string) {
  useProjectAccessStore.setState((state) => ({ denied: { ...state.denied, [JSON.stringify([wsId, projectId ?? "*"])]: true } }));
  const key = projectId ? projectKeys.detail(wsId, projectId) : projectKeys.all(wsId);
  void qc.cancelQueries({ queryKey: key });
  qc.removeQueries({ queryKey: key });
  // Lists may retain the revoked description; clear rather than patch it.
  qc.removeQueries({ queryKey: projectKeys.list(wsId) });
  clearProjectProgressDrafts(wsId, projectId);
  clearProjectDescriptionDrafts(wsId, projectId);
}
