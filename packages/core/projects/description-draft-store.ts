import { createDraftStore } from "../drafts/create-draft-store";
interface DescriptionDraft { body: string; baseBody: string; revision: number }
export const useProjectDescriptionDraftStore = createDraftStore<{ entries: Record<string, DescriptionDraft> }>({
  storageKey: "multica_project_description_drafts", workspaceScoped: false,
  emptyData: { entries: {} }, hasMeaningful: (value) => Object.keys(value.entries).length > 0,
});
export function writeProjectDescriptionDraft(key: string, value: DescriptionDraft) {
  const store = useProjectDescriptionDraftStore.getState();
  store.setDraft({ entries: { ...store.draft.entries, [key]: value } });
}
export function acknowledgeProjectDescriptionDraft(key: string, body: string, revision: number) {
  const store = useProjectDescriptionDraftStore.getState(); const current = store.draft.entries[key];
  if (!current) return;
  const entries = { ...store.draft.entries };
  if (current.body === body) delete entries[key];
  else entries[key] = { ...current, baseBody: body, revision };
  store.setDraft({ entries });
}
export function clearProjectDescriptionDrafts(wsId: string, projectId?: string) {
  const store = useProjectDescriptionDraftStore.getState();
  const entries = Object.fromEntries(Object.entries(store.draft.entries).filter(([key]) => {
    try { const parts: unknown = JSON.parse(key); return !(Array.isArray(parts) && parts[1] === wsId && (!projectId || parts[2] === projectId)); } catch { return false; }
  }));
  store.setDraft({ entries });
}

export function clearProjectDescriptionDraft(key: string) {
  const store = useProjectDescriptionDraftStore.getState(); const entries = { ...store.draft.entries };
  delete entries[key]; store.setDraft({ entries });
}
