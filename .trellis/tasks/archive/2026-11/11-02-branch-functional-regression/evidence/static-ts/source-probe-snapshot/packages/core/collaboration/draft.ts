import { z } from "zod";
import { defaultStorage } from "../platform/storage";
import type { StorageAdapter } from "../types/storage";
import { clearCollaborationDeliveryDrafts, retainCollaborationDeliveryScope } from "./delivery-draft";

const prefix = "multica_collaboration_draft:";
const scopeSchema = z.object({ server: z.string(), userId: z.uuid().transform((value) => value.toLowerCase()), issueId: z.uuid().transform((value) => value.toLowerCase()) });
const draftSchema = z.object({
  scope: scopeSchema, requestId: z.uuid(), kind: z.enum(["operation", "comment"]),
  payload: z.unknown(), createdAt: z.number().int().nonnegative(),
});
export type CollaborationDraftScope = z.infer<typeof scopeSchema>;
export type CollaborationDraft = z.infer<typeof draftSchema>;
export class CollaborationDraftError extends Error {
  constructor() { super("The original collaboration request could not be preserved"); this.name = "CollaborationDraftError"; }
}
const listeners = new Set<() => void>();
let revision = 0;
export const collaborationDraftRevision = () => revision;
export function subscribeCollaborationDrafts(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}
function notify() { revision += 1; for (const listener of listeners) listener(); }
function scopePrefix(scope: CollaborationDraftScope): string {
  return `${prefix}${encodeURIComponent(JSON.stringify([scope.server, scope.userId.toLowerCase(), scope.issueId.toLowerCase()]))}:`;
}
function draftKey(draft: Pick<CollaborationDraft, "scope" | "requestId">): string { return `${scopePrefix(draft.scope)}${draft.requestId}`; }

/** Callers validate the kind-specific payload before sending; corrupt storage never authorizes a write. */
export function readCollaborationDrafts(scope: CollaborationDraftScope, storage: StorageAdapter = defaultStorage): CollaborationDraft[] {
  try {
    if (!storage.keys) throw new CollaborationDraftError();
    return storage.keys().filter((key) => key.startsWith(scopePrefix(scope))).flatMap((key) => {
      const raw = storage.getItem(key);
      if (raw === null) return [];
      const draft = draftSchema.parse(JSON.parse(raw));
      if (draftKey(draft) !== key) throw new CollaborationDraftError();
      return [draft];
    }).sort((a, b) => a.createdAt - b.createdAt || a.requestId.localeCompare(b.requestId));
  } catch { throw new CollaborationDraftError(); }
}

export function saveCollaborationDraft(draft: CollaborationDraft, storage: StorageAdapter = defaultStorage): void {
  try {
    const encoded = JSON.stringify(draftSchema.parse(draft));
    const key = draftKey(draft);
    const existing = storage.getItem(key);
    if (existing !== null && existing !== encoded) throw new CollaborationDraftError();
    if (!storage.keys) throw new CollaborationDraftError();
    storage.setItem(key, encoded);
    if (storage.getItem(key) !== encoded) throw new CollaborationDraftError();
    notify();
  } catch { throw new CollaborationDraftError(); }
}

export function removeCollaborationDraft(draft: CollaborationDraft, storage: StorageAdapter = defaultStorage): void {
  const key = draftKey(draft);
  storage.removeItem(key);
  if (storage.getItem(key) !== null) throw new CollaborationDraftError();
  notify();
}

/** Only object access loss or session teardown clears private pending inputs. */
export function clearCollaborationDrafts(storage: StorageAdapter = defaultStorage, scope?: CollaborationDraftScope): void {
  clearCollaborationDeliveryDrafts(storage, scope);
  try {
    const match = scope ? scopePrefix(scope) : prefix;
    for (const key of storage.keys?.() ?? []) if (key.startsWith(match)) storage.removeItem(key);
  } catch { /* Unavailable storage stays isolated by server, actor and issue. */ }
  finally { notify(); }
}

/** Endpoint/account changes discard private input from the previous identity. */
export function retainCollaborationDraftScope(scope: Pick<CollaborationDraftScope, "server" | "userId">, storage: StorageAdapter = defaultStorage): void {
  retainCollaborationDeliveryScope(scope, storage);
  try {
    for (const key of storage.keys?.() ?? []) {
      if (!key.startsWith(prefix)) continue;
      const encodedScope = key.slice(prefix.length).split(":", 1)[0] ?? "";
      let owned = false;
      try {
        const tuple: unknown = JSON.parse(decodeURIComponent(encodedScope));
        owned = Array.isArray(tuple) && tuple.length === 3 && tuple[0] === scope.server && tuple[1] === scope.userId.toLowerCase();
      } catch { /* An invalid identity cannot retain another account's input. */ }
      if (!owned) storage.removeItem(key);
    }
  } catch { /* Scope checks still prevent another identity from reading it. */ }
  finally { notify(); }
}
