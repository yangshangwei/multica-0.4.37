// @vitest-environment node
import { beforeEach, describe, expect, it } from "vitest";
import type { StorageAdapter } from "../types/storage";
import { clearWorkspaceStorage } from "../platform/storage-cleanup";
import { AdminControlDraftError, configureAdminControlStorage, readAdminControlDraft, saveAdminControlDraft } from "./operation-draft";

const id = (n: number) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const scope = { server: "https://admin.example.test", userId: id(1), organizationId: id(2) };
const input = {
  id: id(3), key: id(4), action: "collaboration" as const, creatorUserId: id(5), handlerUserId: id(6),
  body: { action: "reopen" as const, expectedCollaborationRevision: "7", expectedStatus: "done", targetStatus: "todo",
    statusOptionsVersion: "a".repeat(64), reason: "Restore responsibility", successorUserId: id(7) },
};
let values: Map<string, string>;
let storage: StorageAdapter;
beforeEach(() => {
  values = new Map();
  storage = { getItem: key => values.get(key) ?? null, setItem: (key, value) => { values.set(key, value); },
    removeItem: key => { values.delete(key); }, keys: () => [...values.keys()] };
  configureAdminControlStorage(storage);
});

describe("collaboration administration intent persistence", () => {
  it("preserves the exact issue, baseline, original creator, handler and successor before transport", () => {
    saveAdminControlDraft(scope, input);
    expect(values.size).toBe(1);
    expect(JSON.parse([...values.values()][0]!)).toMatchObject({ scope, input });
  });

  it("retains the original key and successor when a new key or edited retry would replace them", () => {
    saveAdminControlDraft(scope, input);
    for (const replacement of [{ ...input, key: id(8) }, { ...input, body: { ...input.body, successorUserId: id(8) } },
      { ...input, body: { ...input.body, expectedCollaborationRevision: "8" } },
      { ...input, creatorUserId: id(8) }, { ...input, handlerUserId: id(8) }]) {
      expect(() => saveAdminControlDraft(scope, replacement)).toThrow(AdminControlDraftError);
    }
    expect(JSON.parse([...values.values()][0]!).input).toEqual(input);
  });

  it("keeps legacy controls from consuming a collaboration intent for the same UUID", () => {
    saveAdminControlDraft(scope, input);
    expect(() => readAdminControlDraft(scope, input.id)).toThrow(AdminControlDraftError);
  });

  it("keeps pending administration intent when workspace-local state is cleared", () => {
    saveAdminControlDraft(scope, input);
    clearWorkspaceStorage(storage, id(9));
    expect(values.size).toBe(1);
    expect(JSON.parse([...values.values()][0]!).input).toEqual(input);
  });
});
