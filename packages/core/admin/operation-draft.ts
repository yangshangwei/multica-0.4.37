import { z } from "zod";
import { defaultStorage } from "../platform/storage";
import type { StorageAdapter } from "../types/storage";
import type { AdminControlInput } from "./operation-queries";
import { adminVersionSchema } from "./control-schema";

const storagePrefix = "multica_admin_control_draft:";
let storage: StorageAdapter = defaultStorage;
const scopeSchema = z.object({ server: z.string(), userId: z.uuid(), organizationId: z.uuid() });
const base = { id: z.uuid(), key: z.uuid() };
const reason = z.string().trim().min(1).max(1000);
const inputSchema = z.discriminatedUnion("action", [
  z.object({ ...base, action: z.literal("admission"), body: z.object({ admission: z.enum(["accepting", "stopped"]), expectedAdmissionVersion: adminVersionSchema, reason }) }),
  z.object({ ...base, action: z.literal("cancel"), body: z.object({ expectedExecutionFence: z.object({ runtimeId: z.uuid().nullable(), dispatchedAt: z.iso.datetime({ offset: true }).nullable(), targetVersion: adminVersionSchema }), reason }) }),
]);
const draftSchema = z.object({ scope: scopeSchema, input: inputSchema, createdAt: z.number().int().positive() });
export type AdminControlDraftScope = z.output<typeof scopeSchema>;
export type AdminControlDraft = z.output<typeof draftSchema>;
export class AdminControlDraftError extends Error {
  constructor() { super("The original administrative request could not be preserved"); this.name = "AdminControlDraftError"; }
}
export function configureAdminControlStorage(adapter: StorageAdapter) { storage = adapter; }
function targetPrefix(scope: AdminControlDraftScope, id: string): string {
  return `${storagePrefix}${encodeURIComponent(JSON.stringify([scope.server, scope.userId, scope.organizationId, id]))}:`;
}
function draftKey(scope: AdminControlDraftScope, input: Pick<AdminControlInput, "id" | "key">): string {
  return `${targetPrefix(scope, input.id)}${input.key}`;
}
function readDrafts(scope: AdminControlDraftScope, id: string): AdminControlDraft[] {
  try {
    // Enumeration is required so simultaneous tabs can retain independent
    // original keys without a shared read-modify-write index.
    if (!storage.keys) throw new AdminControlDraftError();
    const drafts: AdminControlDraft[] = [];
    for (const key of storage.keys().filter(key => key.startsWith(targetPrefix(scope, id)))) {
      const raw = storage.getItem(key);
      if (raw === null) continue;
      const draft = draftSchema.parse(JSON.parse(raw));
      if (draftKey(draft.scope, draft.input) !== key) throw new AdminControlDraftError();
      drafts.push(draft);
    }
    return drafts.sort((a, b) => a.createdAt - b.createdAt || a.input.key.localeCompare(b.input.key));
  } catch { throw new AdminControlDraftError(); }
}
export function readAdminControlDraft(scope: AdminControlDraftScope, id: string): AdminControlDraft | null {
  return readDrafts(scope, id)[0] ?? null;
}
export function saveAdminControlDraft(scope: AdminControlDraftScope, input: AdminControlInput): void {
  try {
    const normalized = inputSchema.parse(input);
    const drafts = readDrafts(scope, input.id);
    const existing = drafts.find(draft => draft.input.key === input.key);
    if (existing) {
      if (JSON.stringify(existing.input) !== JSON.stringify(normalized)) throw new AdminControlDraftError();
      return;
    }
    if (drafts.length) throw new AdminControlDraftError();
    const draft = draftSchema.parse({ scope, input: normalized, createdAt: Date.now() });
    const key = draftKey(scope, normalized);
    const encoded = JSON.stringify(draft);
    storage.setItem(key, encoded);
    if (storage.getItem(key) !== encoded) throw new AdminControlDraftError();
  } catch { throw new AdminControlDraftError(); }
}
export function removeAdminControlDraft(scope: AdminControlDraftScope, id: string, key: string): void {
  const storedKey = draftKey(scope, { id, key });
  storage.removeItem(storedKey);
  if (storage.getItem(storedKey) !== null) throw new AdminControlDraftError();
}
/** Only session termination clears all command intents; workspace cleanup must not. */
export function clearAdminControlDrafts(adapter: StorageAdapter = storage): void {
  try {
    for (const key of adapter.keys?.() ?? []) {
      if (key.startsWith(storagePrefix)) adapter.removeItem(key);
    }
  } catch { /* Unavailable storage remains isolated by actor and server scope. */ }
}
