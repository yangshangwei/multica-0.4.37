import { useSyncExternalStore } from "react";
import { z } from "zod";
import { defaultStorage } from "../platform/storage";
import type { StorageAdapter } from "../types/storage";
import { collaborationTextByteLength } from "./text-limits";
import {
  COLLABORATION_STAGE_ISSUE_ACTIONS, CollaborationStageRequestSchemas, collaborationStageBodyLimit, CollaborationStageConfirmationIdentitySchema, collaborationStageConfirmationIdentity,
  type CollaborationStageFrozenRequest, type CollaborationStageRequestAction, type CollaborationStageConfirmationIdentity,
} from "./stage-schema";

/**
 * A pending stage intent is client intent only: the frozen Idempotency-Key, the
 * route identity and the exact body. No server projection is ever stored, and
 * the record is plaintext like every other collaboration draft.
 *
 * This module stays a leaf below `draft.ts`. Issue denial strips the write
 * payload but retains content-free GET-only confirmation identity; account
 * teardown erases both kinds of record under this same storage prefix.
 */
const prefix = "multica_collaboration_stage:";
const uuid = z.uuid().transform((value) => value.toLowerCase());
const scopeSchema = z.strictObject({ server: z.string().min(1), userId: uuid });
const targetSchema = z.discriminatedUnion("kind", [
  z.strictObject({ kind: z.literal("issue"), issueId: uuid }),
  z.strictObject({ kind: z.literal("project"), workspaceId: uuid, projectId: uuid }),
]);
const actions = [...COLLABORATION_STAGE_ISSUE_ACTIONS, "revise_policy"] as const satisfies readonly CollaborationStageRequestAction[];
const intentSchema = z.strictObject({
  version: z.literal(1), scope: scopeSchema, target: targetSchema, action: z.enum(actions),
  // The Idempotency-Key. A UUID keeps it inside the 1-128 character contract.
  requestId: uuid, proposalId: uuid.optional(), decisionId: uuid.optional(), bodyJSON: z.string().min(2), createdAt: z.number().int().nonnegative(),
});

export type CollaborationStageIntentScope = z.infer<typeof scopeSchema>;
export type CollaborationStageIntentTarget = z.infer<typeof targetSchema>;
export type CollaborationStageIntent = Readonly<z.infer<typeof intentSchema>>;

const recoverySchema = z.strictObject({ version: z.literal(1), recoveryOnly: z.literal(true), scope: scopeSchema,
  target: z.strictObject({ kind: z.literal("issue"), issueId: uuid }), action: z.enum(COLLABORATION_STAGE_ISSUE_ACTIONS), requestId: uuid,
  createdAt: z.number().int().nonnegative(), confirmation: CollaborationStageConfirmationIdentitySchema,
}).refine((v) => v.confirmation.action === v.action && "issueId" in v.confirmation && v.confirmation.issueId === v.target.issueId, "Recovery identity matches its target and action");
export type CollaborationStageRecoveryIntent = Readonly<z.infer<typeof recoverySchema>>;
export type CollaborationStageStoredIntent = CollaborationStageIntent | CollaborationStageRecoveryIntent;
export function isCollaborationStageSendable(intent: CollaborationStageStoredIntent): intent is CollaborationStageIntent {
  return !("recoveryOnly" in intent);
}
export function collaborationStageIntentIdentity(intent: CollaborationStageStoredIntent): CollaborationStageConfirmationIdentity | null {
  if (!isCollaborationStageSendable(intent)) {
    const parsed = recoverySchema.safeParse(intent);
    return parsed.success ? parsed.data.confirmation : null;
  }
  const request = collaborationStageFrozenRequest(intent);
  return request ? collaborationStageConfirmationIdentity(request) : null;
}

export class CollaborationStageIntentStorageError extends Error {
  /** `false`: the request was never stored, so it was provably never sent from here. */
  constructor(readonly previouslyStored?: boolean) {
    super("The original stage request could not be preserved");
    this.name = "CollaborationStageIntentStorageError";
  }
}

const listeners = new Set<() => void>();
let revision = 0;
export const collaborationStageIntentRevision = () => revision;
export function subscribeCollaborationStageIntents(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}
function notify() { revision += 1; for (const listener of listeners) listener(); }

const identityPrefix = (scope: CollaborationStageIntentScope) => JSON.stringify([scope.server, scope.userId.toLowerCase()]).slice(0, -1);
function targetTuple(scope: CollaborationStageIntentScope, target: CollaborationStageIntentTarget): string[] {
  return target.kind === "issue" ? [scope.server, scope.userId.toLowerCase(), "issue", target.issueId.toLowerCase()]
    : [scope.server, scope.userId.toLowerCase(), "project", target.workspaceId.toLowerCase(), target.projectId.toLowerCase()];
}
const targetPrefix = (scope: CollaborationStageIntentScope, target: CollaborationStageIntentTarget) => `${prefix}${encodeURIComponent(JSON.stringify(targetTuple(scope, target)))}:`;
const keyFor = (intent: Pick<CollaborationStageIntent, "scope" | "target" | "requestId">) => `${targetPrefix(intent.scope, intent.target)}${intent.requestId.toLowerCase()}`;

/** The typed request an intent stands for, or null when the record is not a valid frozen request. */
export function collaborationStageFrozenRequest(intent: CollaborationStageIntent): CollaborationStageFrozenRequest | null {
  const record = intentSchema.safeParse(intent);
  if (!record.success) return null;
  const { action, target, proposalId, decisionId, bodyJSON } = record.data;
  const needsProposal = action === "revise_proposal" || action === "decide_proposal";
  if ((action === "revise_policy") !== (target.kind === "project") || needsProposal !== (proposalId !== undefined) || (action === "revoke_decision") !== (decisionId !== undefined)) return null;
  if (collaborationTextByteLength(bodyJSON) > collaborationStageBodyLimit(action)) return null;
  let raw: unknown;
  try { raw = JSON.parse(bodyJSON); } catch { return null; }
  const body = CollaborationStageRequestSchemas[action].safeParse(raw);
  // The stored text is the exact body that was (or will be) sent: it must be
  // the canonical serialization of a valid request, not merely parseable.
  if (!body.success || JSON.stringify(body.data) !== bodyJSON) return null;
  if (target.kind === "project") return { action: "revise_policy", workspaceId: target.workspaceId, projectId: target.projectId, body: body.data } as CollaborationStageFrozenRequest;
  return { action, issueId: target.issueId, ...(proposalId !== undefined ? { proposalId } : {}), ...(decisionId !== undefined ? { decisionId } : {}), body: body.data } as CollaborationStageFrozenRequest;
}

/** Validate and freeze a new intent. Nothing is stored or sent here. */
export function freezeCollaborationStageIntent(input: {
  scope: CollaborationStageIntentScope; target: CollaborationStageIntentTarget; action: CollaborationStageRequestAction;
  requestId: string; proposalId?: string; decisionId?: string; body: unknown; createdAt: number;
}): CollaborationStageIntent {
  const body = CollaborationStageRequestSchemas[input.action].parse(input.body);
  const record = intentSchema.parse({ version: 1, scope: input.scope, target: input.target, action: input.action, requestId: input.requestId,
    ...(input.proposalId !== undefined ? { proposalId: input.proposalId } : {}), ...(input.decisionId !== undefined ? { decisionId: input.decisionId } : {}), bodyJSON: JSON.stringify(body), createdAt: input.createdAt });
  if (collaborationStageFrozenRequest(record) === null) throw new TypeError("Invalid stage intent");
  return Object.freeze({ ...record, scope: Object.freeze(record.scope), target: Object.freeze(record.target) });
}

function decode(key: string, raw: string | null): CollaborationStageStoredIntent {
  if (raw === null) throw new CollaborationStageIntentStorageError();
  const intent = z.union([intentSchema, recoverySchema]).parse(JSON.parse(raw));
  if (keyFor(intent) !== key || collaborationStageIntentIdentity(intent) === null) throw new CollaborationStageIntentStorageError();
  return intent;
}

/** Returns whether this exact request was already stored (an earlier attempt may have been sent). */
export function saveCollaborationStageIntent(intent: CollaborationStageIntent, storage: StorageAdapter = defaultStorage): boolean {
  let previouslyStored: boolean | undefined;
  try {
    const record = intentSchema.parse(intent);
    if (collaborationStageFrozenRequest(record) === null) throw new CollaborationStageIntentStorageError();
    const encoded = JSON.stringify(record), key = keyFor(record), previous = storage.getItem(key);
    previouslyStored = previous !== null;
    // A key is bound to one body for ever; a different record under it is never overwritten.
    if (!storage.keys || previous !== null && previous !== encoded) throw new CollaborationStageIntentStorageError();
    storage.setItem(key, encoded);
    if (storage.getItem(key) !== encoded) throw new CollaborationStageIntentStorageError();
  } catch { throw new CollaborationStageIntentStorageError(previouslyStored); }
  notify();
  return previouslyStored;
}

/** A corrupt or relocated record is an error: it must not disappear and unlock a replacement. */
export function readCollaborationStageIntents(scope: CollaborationStageIntentScope, target: CollaborationStageIntentTarget, storage: StorageAdapter = defaultStorage): CollaborationStageStoredIntent[] {
  try {
    if (!storage.keys) throw new CollaborationStageIntentStorageError();
    const match = targetPrefix(scope, target);
    return storage.keys().filter((key) => key.startsWith(match)).map((key) => decode(key, storage.getItem(key)))
      .sort((a, b) => a.createdAt - b.createdAt || a.requestId.localeCompare(b.requestId));
  } catch { throw new CollaborationStageIntentStorageError(); }
}

export function retireCollaborationStageIntent(intent: Pick<CollaborationStageIntent, "scope" | "target" | "requestId">, storage: StorageAdapter = defaultStorage): void {
  try {
    const key = keyFor(intent);
    storage.removeItem(key);
    if (storage.getItem(key) !== null) throw new CollaborationStageIntentStorageError();
  } catch { throw new CollaborationStageIntentStorageError(); }
  finally { notify(); }
}

function storedTuple(key: string): unknown {
  try { return JSON.parse(decodeURIComponent(key.slice(prefix.length).split(":", 1)[0] ?? "")); } catch { return null; }
}

/** Object denial erases private payloads but keeps GET-only receipt identity. Session teardown removes both kinds. */
export function clearCollaborationStageIntents(storage: StorageAdapter = defaultStorage, scope?: CollaborationStageIntentScope & { issueId: string }): void {
  try {
    const match = scope ? targetPrefix(scope, { kind: "issue", issueId: scope.issueId }) : prefix;
    for (const key of storage.keys?.() ?? []) {
      if (!key.startsWith(match)) continue;
      if (!scope) { storage.removeItem(key); continue; }
      try {
        const intent = decode(key, storage.getItem(key));
        if (!isCollaborationStageSendable(intent)) continue;
        const confirmation = collaborationStageIntentIdentity(intent);
        const recovery = recoverySchema.parse({ version: 1, recoveryOnly: true, scope: intent.scope, target: intent.target,
          action: intent.action, requestId: intent.requestId, createdAt: intent.createdAt, confirmation });
        const encoded = JSON.stringify(recovery);
        storage.setItem(key, encoded);
        if (storage.getItem(key) !== encoded) storage.removeItem(key);
      } catch { storage.removeItem(key); }
    }
  } catch { /* Unavailable storage stays isolated by server, actor and target. */ }
  finally { notify(); }
}

/** Endpoint/account changes discard pending input that belongs to the previous identity. */
export function retainCollaborationStageIntentScope(scope: CollaborationStageIntentScope, storage: StorageAdapter = defaultStorage): void {
  try {
    const owned = identityPrefix(scope);
    for (const key of storage.keys?.() ?? []) {
      if (!key.startsWith(prefix)) continue;
      const tuple = storedTuple(key);
      // An unreadable identity cannot retain another account's input.
      if (!Array.isArray(tuple) || !JSON.stringify(tuple).startsWith(`${owned},`)) storage.removeItem(key);
    }
  } catch { /* Scope checks still prevent another identity from reading it. */ }
  finally { notify(); }
}

/** Pending intents of one issue or project; `error` means storage could not be read and nothing may be assumed absent. */
export function useCollaborationStageIntents(scope: CollaborationStageIntentScope, target: CollaborationStageIntentTarget | null): { intents: CollaborationStageStoredIntent[]; error: boolean } {
  useSyncExternalStore(subscribeCollaborationStageIntents, collaborationStageIntentRevision, () => 0);
  if (target === null) return { intents: [], error: false };
  try { return { intents: readCollaborationStageIntents({ server: scope.server, userId: scope.userId }, target), error: false }; }
  catch { return { intents: [], error: true }; }
}
