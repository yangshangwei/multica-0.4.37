import { z } from "zod";
import { defaultStorage } from "../platform/storage";
import type { StorageAdapter } from "../types/storage";
import { CompletionAuthorizationRequestSchema, completionUUID, completionReason } from "./completion-schema";

const prefix = "multica_completion_draft:";
export const CompletionDraftScopeSchema = z.strictObject({ server: z.string(), userId: completionUUID, workspaceId: completionUUID, issueId: completionUUID });
export const CompletionDraftSchema = z.strictObject({
  version: z.literal(1), scope: CompletionDraftScopeSchema, requestKey: completionUUID, request: CompletionAuthorizationRequestSchema,
  createdAt: z.number().int().nonnegative().safe(), creationDispatched: z.boolean(), authorizationId: completionUUID.nullable(), submissionDispatched: z.boolean(),
  revocation: z.strictObject({ requestKey: completionUUID, reason: completionReason, dispatched: z.boolean() }).nullable(),
}).refine((v) => v.request.source_workspace_id === v.scope.workspaceId && v.request.target.issue_id === v.scope.issueId && v.request.consumer.user_id === v.scope.userId &&
  (!v.submissionDispatched || (v.creationDispatched && v.authorizationId !== null)) && (!v.revocation || v.authorizationId !== null));
export type CompletionDraftScope = z.infer<typeof CompletionDraftScopeSchema>;
export type CompletionDraft = z.infer<typeof CompletionDraftSchema>;
export class CompletionDraftError extends Error {
  constructor() { super("The original completion request could not be preserved"); this.name = "CompletionDraftError"; }
}
const listeners = new Set<() => void>();
let revision = 0;
export const completionDraftRevision = () => revision;
export function subscribeCompletionDrafts(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; }
function notify() { revision += 1; for (const listener of listeners) listener(); }
function scopePrefix(scope: CompletionDraftScope): string {
  const s = CompletionDraftScopeSchema.parse({ server: scope.server, userId: scope.userId, workspaceId: scope.workspaceId, issueId: scope.issueId });
  return `${prefix}${encodeURIComponent(JSON.stringify([s.server, s.userId, s.workspaceId, s.issueId]))}:`;
}
function draftKey(draft: CompletionDraft) { return `${scopePrefix(draft.scope)}${draft.requestKey}`; }

export function readCompletionDrafts(scope: CompletionDraftScope, storage: StorageAdapter = defaultStorage): CompletionDraft[] {
  try {
    if (!storage.keys) throw new CompletionDraftError();
    const result: CompletionDraft[] = [];
    for (const key of storage.keys().filter(key => key.startsWith(scopePrefix(scope)))) {
      const raw = storage.getItem(key);
      if (raw === null) continue;
      const draft = CompletionDraftSchema.parse(JSON.parse(raw));
      if (draftKey(draft) !== key) throw new CompletionDraftError();
      result.push(draft);
    }
    return result.sort((a, b) => a.createdAt - b.createdAt || a.requestKey.localeCompare(b.requestKey));
  } catch { throw new CompletionDraftError(); }
}

/** Stage markers only advance. Stored server outcomes are never a permission cache. */
export function saveCompletionDraft(input: CompletionDraft, storage: StorageAdapter = defaultStorage): void {
  try {
    const draft = CompletionDraftSchema.parse(input);
    const drafts = readCompletionDrafts(draft.scope, storage);
    const existing = drafts.find(item => item.requestKey === draft.requestKey);
    if (!existing && drafts.length) throw new CompletionDraftError();
    if (existing && (JSON.stringify(existing.request) !== JSON.stringify(draft.request) || existing.createdAt !== draft.createdAt ||
      (existing.creationDispatched && !draft.creationDispatched) || (existing.submissionDispatched && !draft.submissionDispatched) ||
      (existing.authorizationId !== null && existing.authorizationId !== draft.authorizationId) ||
      (existing.revocation !== null && (draft.revocation === null || existing.revocation.requestKey !== draft.revocation.requestKey || existing.revocation.reason !== draft.revocation.reason || (existing.revocation.dispatched && !draft.revocation.dispatched))))) throw new CompletionDraftError();
    const encoded = JSON.stringify(draft), key = draftKey(draft);
    storage.setItem(key, encoded);
    if (storage.getItem(key) !== encoded) throw new CompletionDraftError();
    notify();
  } catch { throw new CompletionDraftError(); }
}

export function removeCompletionDraft(draft: CompletionDraft, storage: StorageAdapter = defaultStorage): void {
  const original = readCompletionDrafts(draft.scope, storage).find(item => item.requestKey === draft.requestKey);
  if (!original || JSON.stringify(original) !== JSON.stringify(draft)) throw new CompletionDraftError();
  const key = draftKey(draft);
  storage.removeItem(key);
  if (storage.getItem(key) !== null) throw new CompletionDraftError();
  notify();
}

/** Logout erases all actors' original input, including when no workspace list was loaded. */
export function clearCompletionDrafts(storage: StorageAdapter = defaultStorage): void {
  try { for (const key of storage.keys?.() ?? []) if (key.startsWith(prefix)) storage.removeItem(key); }
  finally { notify(); }
}
