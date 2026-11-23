import { z } from "zod";
import { defaultStorage } from "../platform/storage";
import type { StorageAdapter } from "../types/storage";
import type { AdminControlInput } from "./operation-queries";
import { adminVersionSchema } from "./control-schema";
import { adminAlertChangeSchema } from "./alert-schema";
import { AdminCollaborationChangeSchema, AdminCollaborationMemoryPurgeSchema } from "./collaboration-schema";

const storagePrefix = "multica_admin_control_draft:";
let storage: StorageAdapter = defaultStorage;
const scopeSchema = z.object({ server: z.string(), userId: z.uuid(), organizationId: z.uuid() });
const base = { id: z.uuid(), key: z.uuid() };
const reason = z.string().trim().min(1).max(1000);
const inputSchema = z.discriminatedUnion("action", [
  z.object({ ...base, action: z.literal("admission"), body: z.object({ admission: z.enum(["accepting", "stopped"]), expectedAdmissionVersion: adminVersionSchema, reason }) }),
  z.object({ ...base, action: z.literal("cancel"), body: z.object({ expectedExecutionFence: z.object({ runtimeId: z.uuid().nullable(), dispatchedAt: z.iso.datetime({ offset: true }).nullable(), targetVersion: adminVersionSchema }), reason }) }),
  z.object({ ...base, action: z.literal("alert"), body: adminAlertChangeSchema }),
  z.object({ ...base, action: z.literal("collaboration_gate"), body: z.union([
    z.strictObject({ action: z.enum(["open_creation", "close_creation"]), reason }),
    z.strictObject({ action: z.literal("open_execution"), expectedExecutionEpoch: adminVersionSchema, reason }),
  ]) }),
]);
const collaborationInputSchema = z.strictObject({
  ...base, action: z.literal("collaboration"), creatorUserId: z.uuid(), handlerUserId: z.uuid(), body: AdminCollaborationChangeSchema,
});
const storedInputSchema = z.discriminatedUnion("action", [...inputSchema.options, collaborationInputSchema]);
const draftSchema = z.object({ scope: scopeSchema, input: storedInputSchema, createdAt: z.number().int().positive() });
export type AdminControlDraftScope = z.output<typeof scopeSchema>;
type StoredDraft = z.output<typeof draftSchema>;
export type AdminControlDraft = StoredDraft & { input: AdminControlInput };
export type AdminCollaborationInput = z.output<typeof collaborationInputSchema>;
export type AdminCollaborationDraft = StoredDraft & { input: AdminCollaborationInput };
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
function readDrafts(scope: AdminControlDraftScope, id: string): StoredDraft[] {
  try {
    // Enumeration is required so simultaneous tabs can retain independent
    // original keys without a shared read-modify-write index.
    if (!storage.keys) throw new AdminControlDraftError();
    const drafts: StoredDraft[] = [];
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
  const draft = readDrafts(scope, id)[0];
  if (!draft) return null;
  if (draft.input.action === "collaboration") throw new AdminControlDraftError();
  return { ...draft, input: draft.input };
}
export function readAdminCollaborationDraft(scope: AdminControlDraftScope, id: string): AdminCollaborationDraft | null {
  const draft = readDrafts(scope, id)[0];
  if (!draft) return null;
  if (draft.input.action !== "collaboration") throw new AdminControlDraftError();
  return { ...draft, input: draft.input };
}
export function saveAdminControlDraft(scope: AdminControlDraftScope, input: AdminControlInput | AdminCollaborationInput): void {
  try {
    const normalized = storedInputSchema.parse(input);
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
// Shared-context purges keep their own namespace so a pending purge never
// blocks or is mistaken for the issue's lifecycle request.
const memoryStoragePrefix = `${storagePrefix}memory:`;
const memoryInputSchema = z.strictObject({ ...base, action: z.literal("collaboration_memory"), creatorUserId: z.uuid(), body: AdminCollaborationMemoryPurgeSchema });
const memoryDraftSchema = z.object({ scope: scopeSchema, input: memoryInputSchema, createdAt: z.number().int().positive() });
export type AdminCollaborationMemoryInput = z.output<typeof memoryInputSchema>;
export type AdminCollaborationMemoryDraft = z.output<typeof memoryDraftSchema>;
function memoryPrefix(scope: AdminControlDraftScope, id: string): string {
  return `${memoryStoragePrefix}${encodeURIComponent(JSON.stringify([scope.server, scope.userId, scope.organizationId, id]))}:`;
}
export function readAdminCollaborationMemoryDraft(scope: AdminControlDraftScope, id: string): AdminCollaborationMemoryDraft | null {
  try {
    if (!storage.keys) throw new AdminControlDraftError();
    const drafts: AdminCollaborationMemoryDraft[] = [];
    for (const key of storage.keys().filter(key => key.startsWith(memoryPrefix(scope, id)))) {
      const raw = storage.getItem(key);
      if (raw === null) continue;
      const draft = memoryDraftSchema.parse(JSON.parse(raw));
      if (`${memoryPrefix(draft.scope, draft.input.id)}${draft.input.key}` !== key) throw new AdminControlDraftError();
      drafts.push(draft);
    }
    return drafts.sort((a, b) => a.createdAt - b.createdAt || a.input.key.localeCompare(b.input.key))[0] ?? null;
  } catch { throw new AdminControlDraftError(); }
}
export function saveAdminCollaborationMemoryDraft(scope: AdminControlDraftScope, input: AdminCollaborationMemoryInput): void {
  try {
    const normalized = memoryInputSchema.parse(input);
    const existing = readAdminCollaborationMemoryDraft(scope, input.id);
    if (existing) {
      if (existing.input.key !== normalized.key || JSON.stringify(existing.input) !== JSON.stringify(normalized)) throw new AdminControlDraftError();
      return;
    }
    const key = `${memoryPrefix(scope, normalized.id)}${normalized.key}`;
    const encoded = JSON.stringify(memoryDraftSchema.parse({ scope, input: normalized, createdAt: Date.now() }));
    storage.setItem(key, encoded);
    if (storage.getItem(key) !== encoded) throw new AdminControlDraftError();
  } catch { throw new AdminControlDraftError(); }
}
export function removeAdminCollaborationMemoryDraft(scope: AdminControlDraftScope, id: string, key: string): void {
  const storedKey = `${memoryPrefix(scope, id)}${key}`;
  storage.removeItem(storedKey);
  if (storage.getItem(storedKey) !== null) throw new AdminControlDraftError();
}

/** Only session termination clears all command intents; workspace cleanup must not. */
export function clearAdminControlDrafts(adapter: StorageAdapter = storage): void {
  try {
    // The prefix also covers the shared-context purge namespace.
    for (const key of adapter.keys?.() ?? []) {
      if (key.startsWith(storagePrefix)) adapter.removeItem(key);
    }
  } catch { /* Unavailable storage remains isolated by actor and server scope. */ }
}
