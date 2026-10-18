// @vitest-environment node
import { expect, it } from "vitest";
import { readCollaborationPendingDrafts } from "./pending";
import { saveCollaborationDraft } from "./draft";

it("blocks replacement writes when a saved request has an unknown or corrupt payload", () => {
  const values = new Map<string, string>();
  const storage = { keys: () => [...values.keys()], getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => { values.set(key, value); }, removeItem: (key: string) => { values.delete(key); } };
  const scope = { server: "https://server.example", userId: "00000000-0000-4000-8000-000000000001", issueId: "00000000-0000-4000-8000-000000000002" };
  for (const payload of [{ domain: "future_action" }, { domain: "cancel_command", input: {} }, { action: "transfer", expectedCollaborationRevision: "broken" }]) {
    values.clear();
    saveCollaborationDraft({ scope, requestId: "00000000-0000-4000-8000-000000000003", kind: "operation", createdAt: 1, payload }, storage);
    expect(readCollaborationPendingDrafts(scope, storage).error).toBe(true);
    expect(values.size).toBe(1);
  }
  expect(readCollaborationPendingDrafts({ ...scope, userId: scope.issueId }, storage).error).toBe(false);
});
