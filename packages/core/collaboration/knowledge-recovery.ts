import { z } from "zod";
import { defaultStorage } from "../platform/storage";
import type { StorageAdapter } from "../types/storage";
import type { KnowledgeOperationIdentity } from "./knowledge-actions";

const prefix = "multica_knowledge_request:";
const identitySchema = z.strictObject({
  scope: z.strictObject({ server: z.string().min(1), userId: z.uuid() }),
  candidateId: z.uuid(), requestId: z.uuid(), action: z.enum(["propose", "approve", "reject", "withdraw"]),
});
const scopePrefix = (scope: KnowledgeOperationIdentity["scope"]) => `${prefix}${encodeURIComponent(JSON.stringify([scope.server, scope.userId]))}:`;
const keyFor = (identity: KnowledgeOperationIdentity) => `${scopePrefix(identity.scope)}${identity.requestId}`;
export class KnowledgeRecoveryStorageError extends Error {
  constructor() { super("The original knowledge request could not be preserved"); this.name = "KnowledgeRecoveryStorageError"; }
}

/** Persist only client-generated recovery identity. No source ID, body, summary, refs or server receipt enters storage. */
export function saveKnowledgeOperationIdentity(identity: KnowledgeOperationIdentity, storage: StorageAdapter = defaultStorage): void {
  try {
    const record = identitySchema.parse({ scope: identity.scope, candidateId: identity.candidateId, requestId: identity.requestId, action: identity.action });
    const encoded = JSON.stringify(record), key = keyFor(record), previous = storage.getItem(key);
    if (!storage.keys || previous !== null && previous !== encoded) throw new KnowledgeRecoveryStorageError();
    storage.setItem(key, encoded);
    if (storage.getItem(key) !== encoded) throw new KnowledgeRecoveryStorageError();
  } catch { throw new KnowledgeRecoveryStorageError(); }
}

/** Recovery reads survive source/target role loss and a fresh login by the same original actor. */
export function readPendingKnowledgeOperations(scope: KnowledgeOperationIdentity["scope"], storage: StorageAdapter = defaultStorage): KnowledgeOperationIdentity[] {
  try {
    if (!storage.keys) throw new KnowledgeRecoveryStorageError();
    return storage.keys().filter((key) => key.startsWith(scopePrefix(scope))).map((key) => {
      const record = identitySchema.parse(JSON.parse(storage.getItem(key) ?? "null"));
      if (keyFor(record) !== key) throw new KnowledgeRecoveryStorageError();
      return record;
    });
  } catch { throw new KnowledgeRecoveryStorageError(); }
}

export function retireKnowledgeOperationIdentity(identity: KnowledgeOperationIdentity, storage: StorageAdapter = defaultStorage): void {
  try {
    const key = keyFor(identity); storage.removeItem(key);
    if (storage.getItem(key) !== null) throw new KnowledgeRecoveryStorageError();
  } catch { throw new KnowledgeRecoveryStorageError(); }
}
