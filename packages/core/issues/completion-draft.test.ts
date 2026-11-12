// @vitest-environment node
import { describe, expect, it } from "vitest";
import type { StorageAdapter } from "../types/storage";
import { clearCompletionDrafts, readCompletionDrafts, saveCompletionDraft } from "./completion-draft";
import { completionIds as ids, completionPreviewFixture } from "./completion-test-fixtures";

function memoryStorage(): StorageAdapter {
  const data = new Map<string, string>();
  return { getItem: key => data.get(key) ?? null, setItem: (key, value) => { data.set(key, value); }, removeItem: key => { data.delete(key); }, keys: () => [...data.keys()] };
}
const scope = { server: "https://api.test", userId: ids.user, workspaceId: ids.workspace, issueId: ids.issue };
function draft() {
  return { version: 1 as const, scope, requestKey: ids.key, request: { ...completionPreviewFixture().authorization_binding, operation_id: ids.operation, reason: "Accepted", expires_at: "2099-01-01T00:15:00Z" }, createdAt: 1, creationDispatched: true, authorizationId: null, submissionDispatched: false, revocation: null };
}

describe("completion original intent storage", () => {
  it("preserves the immutable request while allowing only monotonic stage updates", () => {
    const storage = memoryStorage();
    const original = draft();
    saveCompletionDraft(original, storage);
    saveCompletionDraft({ ...original, authorizationId: ids.authorization, submissionDispatched: true }, storage);
    expect(readCompletionDrafts(scope, storage)[0]?.submissionDispatched).toBe(true);
    expect(() => saveCompletionDraft(original, storage)).toThrow();
    expect(() => saveCompletionDraft({ ...original, request: { ...original.request, reason: "Changed" } }, storage)).toThrow();
  });

  it("refuses silent storage failure and adapters without enumeration", () => {
    const storage = memoryStorage();
    expect(() => saveCompletionDraft(draft(), { ...storage, setItem: () => {} })).toThrow();
    expect(() => saveCompletionDraft(draft(), { ...storage, keys: undefined })).toThrow();
  });

  it("isolates scope and removes all recovery input when the session ends", () => {
    const storage = memoryStorage();
    saveCompletionDraft(draft(), storage);
    expect(readCompletionDrafts({ ...scope, userId: ids.operation }, storage)).toEqual([]);
    expect(readCompletionDrafts({ ...scope, workspaceId: ids.operation }, storage)).toEqual([]);
    clearCompletionDrafts(storage);
    expect(readCompletionDrafts(scope, storage)).toEqual([]);
  });
});
