// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { StorageAdapter } from "../types/storage";
import { clearCollaborationDrafts, readCollaborationDrafts, removeCollaborationDraft, retainCollaborationDraftScope, saveCollaborationDraft, type CollaborationDraft } from "./draft";

const id = (tail: number) => `00000000-0000-4000-8000-${String(tail).padStart(12, "0")}`;
const scope = { server: "https://server.example", userId: id(1), issueId: id(2) };
const original: CollaborationDraft = { scope, requestId: id(3), kind: "comment", payload: { content: "Original input", expected_lifecycle_epoch: "9007199254740993" }, createdAt: 1 };
function memory() {
  const items = new Map<string, string>();
  const adapter: StorageAdapter = { getItem: (key) => items.get(key) ?? null, setItem: (key, value) => { items.set(key, value); }, removeItem: (key) => { items.delete(key); }, keys: () => [...items.keys()] };
  return { items, adapter };
}

describe("durable collaboration requests", () => {
  it("retains an exact original payload and independent keys from concurrent tabs", () => {
    const { adapter } = memory();
    saveCollaborationDraft(original, adapter);
    saveCollaborationDraft({ ...original, requestId: id(4), payload: { content: "Other input" } }, adapter);
    expect(readCollaborationDrafts(scope, adapter)).toHaveLength(2);
    expect(readCollaborationDrafts(scope, adapter)[0]).toEqual(original);
    expect(() => saveCollaborationDraft({ ...original, payload: { content: "Changed" } }, adapter)).toThrow();
    removeCollaborationDraft(original, adapter);
    expect(readCollaborationDrafts(scope, adapter).map((draft) => draft.requestId)).toEqual([id(4)]);
  });

  it("fails before a write if storage cannot be enumerated, persisted or safely decoded", () => {
    const { adapter, items } = memory();
    expect(() => saveCollaborationDraft(original, { ...adapter, keys: undefined })).toThrow();
    expect(() => saveCollaborationDraft(original, { ...adapter, setItem: () => {} })).toThrow();
    saveCollaborationDraft(original, adapter);
    const key = [...items.keys()][0];
    if (!key) throw new Error("The original request was not stored");
    items.set(key, "corrupt-private-sentinel");
    expect(() => readCollaborationDrafts(scope, adapter)).toThrow("The original collaboration request could not be preserved");
  });

  it("clears only the revoked issue; workspace failure has no draft cleanup path", () => {
    const { adapter } = memory();
    saveCollaborationDraft(original, adapter);
    const other = { ...original, scope: { ...scope, issueId: id(5) } };
    saveCollaborationDraft(other, adapter);
    clearCollaborationDrafts(adapter, scope);
    expect(readCollaborationDrafts(scope, adapter)).toEqual([]);
    expect(readCollaborationDrafts(other.scope, adapter)).toEqual([other]);
  });

  it("clears previous server/account input on a scope change and all input on teardown", () => {
    const { adapter, items } = memory();
    saveCollaborationDraft(original, adapter);
    saveCollaborationDraft({ ...original, scope: { ...scope, server: "https://other.example" } }, adapter);
    saveCollaborationDraft({ ...original, scope: { ...scope, userId: id(9) } }, adapter);
    retainCollaborationDraftScope(scope, adapter);
    expect(items.size).toBe(1);
    clearCollaborationDrafts(adapter);
    expect(items.size).toBe(0);
  });
});
