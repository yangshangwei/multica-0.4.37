// @vitest-environment node
import { beforeEach, expect, it } from "vitest";
import { clearWorkspaceStorage } from "../platform/storage-cleanup";
import type { StorageAdapter } from "../types/storage";
import { configureAdminControlStorage, readAdminControlDraft, saveAdminControlDraft, removeAdminControlDraft, AdminControlDraftError } from "./operation-draft";
const id = "11111111-1111-4111-8111-111111111111";
const key = "22222222-2222-4222-8222-222222222222";
const scope = { server: "https://server.test", userId: id, organizationId: id };
const input = { id, key, action: "cancel" as const, body: { expectedExecutionFence: { runtimeId: null, dispatchedAt: null, targetVersion: "5" }, reason: "Operator request" } };
let values: Map<string, string>;
let storage: StorageAdapter;
beforeEach(() => { values = new Map(); storage = { getItem: key => values.get(key) ?? null, setItem: (key, value) => { values.set(key, value); }, removeItem: key => { values.delete(key); }, keys: () => [...values.keys()] }; configureAdminControlStorage(storage); });
it("retains an unresolved administrative request when an unrelated workspace is removed", () => {
  saveAdminControlDraft(scope, input);
  clearWorkspaceStorage(storage, "unrelated");
  expect(readAdminControlDraft(scope, id)?.input).toEqual(input);
});
it("does not lose interleaved tab saves for different or identical targets", () => {
  for (const otherId of [key, id]) {
    values.clear();
    const other = { ...input, id: otherId, key: id };
    let interleaved = false;
    storage.setItem = (storageKey, value) => {
      if (!interleaved) { interleaved = true; saveAdminControlDraft(scope, other); }
      values.set(storageKey, value);
    };
    saveAdminControlDraft(scope, input);
    if (otherId !== id) {
      expect(readAdminControlDraft(scope, otherId)?.input).toEqual(other);
      expect(readAdminControlDraft(scope, id)?.input).toEqual(input);
    } else {
      const first = readAdminControlDraft(scope, id)!;
      removeAdminControlDraft(scope, id, first.input.key);
      const second = readAdminControlDraft(scope, id)!;
      expect(new Set([first.input.key, second.input.key])).toEqual(new Set([input.key, other.key]));
    }
  }
});
it("recovers the original non-secret request across remount and isolates server, actor and organization", () => {
  saveAdminControlDraft(scope, input);
  expect(readAdminControlDraft(scope, id)?.input).toEqual(input);
  for (const other of [{ ...scope, server: "other" }, { ...scope, userId: key }, { ...scope, organizationId: key }]) expect(readAdminControlDraft(other, id)).toBeNull();
  expect([...values.values()].join("")).not.toContain("password");
});
it("prevents replacing an unresolved original intent and removes only the matching key", () => {
  saveAdminControlDraft(scope, input);
  expect(() => saveAdminControlDraft(scope, { ...input, key: id })).toThrow(AdminControlDraftError);
  removeAdminControlDraft(scope, id, id);
  expect(readAdminControlDraft(scope, id)).not.toBeNull();
  removeAdminControlDraft(scope, id, key);
  expect(readAdminControlDraft(scope, id)).toBeNull();
});
it("fails closed when original requests cannot be preserved or storage is malformed", () => {
  configureAdminControlStorage({ getItem: () => null, setItem: () => { throw new Error("Full"); }, removeItem: () => {}, keys: () => [] });
  expect(() => saveAdminControlDraft(scope, input)).toThrow(AdminControlDraftError);
  configureAdminControlStorage(storage);
  saveAdminControlDraft(scope, input);
  for (const key of values.keys()) values.set(key, "not-json");
  expect(() => readAdminControlDraft(scope, id)).toThrow(AdminControlDraftError);
});
it("does not write if the adapter cannot enumerate independently persisted requests", () => {
  configureAdminControlStorage({ getItem: storage.getItem, setItem: storage.setItem, removeItem: storage.removeItem });
  expect(() => saveAdminControlDraft(scope, input)).toThrow(AdminControlDraftError);
  expect(values.size).toBe(0);
});
