import { createDraftStore } from "../drafts/create-draft-store";
import type { ProjectUpdateDraft, ProjectUpdatePreview, ProjectUpdateWriteInput } from "../types/project-p1";

interface ProgressDraftEntry { draft: ProjectUpdateDraft; intent: ProjectUpdateWriteInput | null; updatedAt: number }
interface ProgressDrafts { entries: Record<string, ProgressDraftEntry> }
export const useProjectProgressDraftStore = createDraftStore<ProgressDrafts>({
  storageKey: "multica_project_progress_drafts", workspaceScoped: false,
  emptyData: { entries: {} }, hasMeaningful: (data) => Object.keys(data.entries).length > 0,
});
export const projectProgressDraftKey = (connection: string, wsId: string, projectId: string, updateId?: string) =>
  JSON.stringify([connection, wsId, projectId, updateId ?? "create"]);
export function emptyProjectUpdateDraft(): ProjectUpdateDraft {
  return { operation: "create", update_id: null, expected_revision: null, kind: "progress", body: "",
    health_judgment: null, evidence: [], acceptance: null, expected_description_revision: null,
    include_statistics: false, correction_reason: null };
}
export function writeProjectProgressDraft(key: string, draft: ProjectUpdateDraft) {
  const store = useProjectProgressDraftStore.getState();
  const previous = store.draft.entries[key];
  if (previous && JSON.stringify(previous.draft) === JSON.stringify(draft)) return;
  store.setDraft({ entries: { ...store.draft.entries, [key]: { draft, intent: null, updatedAt: Date.now() } } });
}
export function prepareProjectUpdateIntent(key: string, preview: ProjectUpdatePreview): ProjectUpdateWriteInput {
  const store = useProjectProgressDraftStore.getState();
  const previous = store.draft.entries[key];
  if (previous?.intent?.preview_hash === preview.preview_hash) return previous.intent;
  const intent: ProjectUpdateWriteInput = { request_id: crypto.randomUUID(), draft: preview.draft,
    preview_hash: preview.preview_hash, evidence_versions: preview.evidence_versions };
  store.setDraft({ entries: { ...store.draft.entries, [key]: { draft: preview.draft, intent, updatedAt: Date.now() } } });
  return intent;
}
export function clearProjectProgressDraft(key: string, requestId?: string) {
  const store = useProjectProgressDraftStore.getState();
  if (requestId && store.draft.entries[key]?.intent?.request_id !== requestId) return;
  const entries = { ...store.draft.entries }; delete entries[key]; store.setDraft({ entries });
}
export function clearProjectProgressDrafts(wsId: string, projectId?: string) {
  const store = useProjectProgressDraftStore.getState();
  const entries = Object.fromEntries(Object.entries(store.draft.entries).filter(([key]) => {
    try { const parts: unknown = JSON.parse(key); return !(Array.isArray(parts) && parts[1] === wsId && (!projectId || parts[2] === projectId)); } catch { return false; }
  }));
  store.setDraft({ entries });
}
