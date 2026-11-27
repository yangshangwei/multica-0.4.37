import { z } from "zod";
import { createDraftStore } from "../drafts/create-draft-store";

export const MaterialEditorDraftSchema = z.object({
  version: z.number().int().nonnegative(), baselineId: z.string().optional(), content: z.string(), planContent: z.string(),
  acceptance: z.string(), blockers: z.string(), ready: z.boolean(),
  sources: z.array(z.object({ resource_id: z.string(), commit: z.string(), path: z.string() })),
});
export type MaterialEditorDraft = z.infer<typeof MaterialEditorDraftSchema>;
const StoreSchema = z.object({
  editors: z.record(z.string(), MaterialEditorDraftSchema),
  requests: z.record(z.string(), z.unknown()),
  retiredRequests: z.record(z.string(), z.array(z.unknown())).default({}),
});
export const useMaterialDraftStore = createDraftStore<z.infer<typeof StoreSchema>>({
  storageKey: "multica_material_drafts", workspaceScoped: false, emptyData: { editors: {}, requests: {}, retiredRequests: {} },
  hasMeaningful: (value) => Object.keys(value.editors).length > 0 || Object.keys(value.requests).length > 0 || Object.keys(value.retiredRequests).length > 0,
  migrateData: (value) => StoreSchema.safeParse(value).data ?? {},
});
export const materialDraftKey = (userId: string | undefined, wsId: string, projectId: string, subject: string) => JSON.stringify([userId ?? "", wsId, projectId, subject]);
export function setMaterialPendingRequest(key: string, value: unknown | null) {
  const store = useMaterialDraftStore.getState();
  const requests = { ...store.draft.requests };
  if (value === null) delete requests[key]; else requests[key] = value;
  store.setDraft({ requests });
}
// Reconciliation is deliberate: preserve the exact old body as evidence,
// then allow a new request identity against freshly read configuration.
export function retireMaterialPendingRequest(key: string) {
  const store = useMaterialDraftStore.getState();
  if (!Object.hasOwn(store.draft.requests, key)) return;
  const requests = { ...store.draft.requests };
  const original = requests[key];
  delete requests[key];
  store.setDraft({ requests, retiredRequests: { ...store.draft.retiredRequests,
    [key]: [...(store.draft.retiredRequests[key] ?? []), original] } });
}
export function setMaterialEditorDraft(key: string, value: MaterialEditorDraft | null) {
  const store = useMaterialDraftStore.getState();
  const editors = { ...store.draft.editors };
  if (value === null) delete editors[key]; else editors[key] = value;
  store.setDraft({ editors });
}
export function clearMaterialDrafts(wsId: string, projectId?: string) {
  const belongs = (key: string) => {
    try { const value: unknown = JSON.parse(key); return Array.isArray(value) && value[1] === wsId && (!projectId || value[2] === projectId); }
    catch { return true; }
  };
  const store = useMaterialDraftStore.getState();
  store.setDraft({
    editors: Object.fromEntries(Object.entries(store.draft.editors).filter(([key]) => !belongs(key))),
    requests: Object.fromEntries(Object.entries(store.draft.requests).filter(([key]) => !belongs(key))),
    retiredRequests: Object.fromEntries(Object.entries(store.draft.retiredRequests).filter(([key]) => !belongs(key))),
  });
}
