// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { collaborationScope, CollaborationUnavailableError } from "./access";
import {
  CollaborationMemoryTooLargeError,
  executeCollaborationMemoryChange,
  parseCollaborationMemoryDraft,
  prepareCollaborationMemoryChange,
} from "./memory-actions";
import { readCollaborationDrafts } from "./draft";
import { readCollaborationPendingDrafts } from "./pending";
import { parseCollaborationMemoryReceipt, type CollaborationMemory } from "./memory-schema";

const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => auth } }));

const issueId = "00000000-0000-4000-8000-000000000002";
const requestId = "00000000-0000-4000-8000-000000000003";
const memory = (overrides: Partial<CollaborationMemory> = {}): CollaborationMemory => ({
  issue_id: issueId, version: "2", content: "current", bytes: 7, max_bytes: 128,
  author: { kind: "member", member: { user_id: auth.user.id, name: "Writer", avatar_url: "", external: false }, agent: null, resource_owner: null },
  created_at: "2026-10-26T00:00:00Z", restored_from_version: null, can_edit: true, can_purge: true, ...overrides,
});

const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach(client => client.clear()); vi.unstubAllGlobals(); vi.restoreAllMocks(); });

describe("shared-context draft actions", () => {
  function setup() {
    setApiInstance(new ApiClient("https://server.example"));
    return collaborationScope(auth.user.id);
  }

  it("prepares exact update, restore and purge payloads", () => {
    const scope = setup();
    const update = prepareCollaborationMemoryChange(scope, memory(), requestId, { action: "update", baseVersion: "2", content: "new state" });
    expect(update).toMatchObject({ kind: "operation", requestId, scope: { issueId } });
    expect(parseCollaborationMemoryDraft(update)).toEqual({ domain: "memory", action: "memory_update", input: { expected_version: "2", content: "new state" } });
    const restore = prepareCollaborationMemoryChange(scope, memory(), requestId, { action: "restore", version: "1" });
    expect(parseCollaborationMemoryDraft(restore)).toEqual({ domain: "memory", action: "memory_restore", version: "1", input: { expected_version: "2" } });
    const purge = prepareCollaborationMemoryChange(scope, memory(), requestId, { action: "purge", version: "1", reason: " remove old secret " });
    expect(parseCollaborationMemoryDraft(purge)).toEqual({ domain: "memory", action: "memory_purge", version: "1", input: { reason: "remove old secret" } });
  });

  it("rejects client-side unsafe changes before a request can be sent", () => {
    const scope = setup();
    expect(() => prepareCollaborationMemoryChange(scope, memory(), requestId, { action: "update", baseVersion: "2", content: "界".repeat(43) })).toThrow(CollaborationMemoryTooLargeError);
    expect(() => prepareCollaborationMemoryChange(scope, memory(), requestId, { action: "update", baseVersion: "3", content: "future" })).toThrow(CollaborationUnavailableError);
    expect(() => prepareCollaborationMemoryChange(scope, memory(), requestId, { action: "update", baseVersion: "2", content: "bad\0content" })).toThrow(CollaborationUnavailableError);
    expect(() => prepareCollaborationMemoryChange(scope, memory(), requestId, { action: "restore", version: "2" })).toThrow(CollaborationUnavailableError);
    expect(() => prepareCollaborationMemoryChange(scope, memory(), requestId, { action: "purge", version: "2", reason: "current" })).toThrow(CollaborationUnavailableError);
  });

  it("rejects a corrupt operation payload or wrong kind", () => {
    const scope = setup();
    const draft = prepareCollaborationMemoryChange(scope, memory(), requestId, { action: "update", baseVersion: "2", content: "new" });
    expect(() => parseCollaborationMemoryDraft({ ...draft, kind: "comment" })).toThrow(CollaborationUnavailableError);
    expect(() => parseCollaborationMemoryDraft({ ...draft, payload: { domain: "memory", action: "memory_update", input: { expected_version: "2", content: "bad\0content" } } })).toThrow(CollaborationUnavailableError);
  });

  it("retains an uncertain write and recovers its exact original key without resending", async () => {
    const api = new ApiClient("https://server.example");
    setApiInstance(api);
    const scope = collaborationScope(auth.user.id);
    const client = new QueryClient(); clients.push(client);
    const values = new Map<string, string>();
    const storage = { keys: () => [...values.keys()], getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => { values.set(key, value); }, removeItem: (key: string) => { values.delete(key); } };
    const draft = prepareCollaborationMemoryChange(scope, memory(), requestId, { action: "update", baseVersion: "2", content: "original input" });
    const update = vi.spyOn(api, "updateCollaborationMemory").mockRejectedValueOnce(new TypeError("Connection lost"));
    await expect(executeCollaborationMemoryChange(client, scope, draft, false, storage)).rejects.toThrow("Connection lost");
    expect(readCollaborationPendingDrafts(draft.scope, storage).memory).toEqual([draft]);
    const receipt = parseCollaborationMemoryReceipt({ request_id: requestId, action: "memory_update", status: 200, code: "memory_updated",
      protocol_version: 1, issue_id: issueId, version: "3", current_version: "3", bytes: 14, max_bytes: 128, event_seq: "4" })!;
    const recovery = vi.spyOn(api, "getCollaborationMemoryOperation").mockResolvedValueOnce(receipt);
    const retained = readCollaborationDrafts(draft.scope, storage)[0]!;
    await expect(executeCollaborationMemoryChange(client, scope, retained, true, storage)).resolves.toEqual(receipt);
    expect(update).toHaveBeenCalledExactlyOnceWith(issueId, requestId, { expected_version: "2", content: "original input" });
    expect(recovery).toHaveBeenCalledExactlyOnceWith(issueId, requestId);
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
  });

  it("retires only a matching sealed receipt and keeps malformed outcomes pending", async () => {
    const api = new ApiClient("https://server.example"); setApiInstance(api);
    const scope = collaborationScope(auth.user.id);
    const client = new QueryClient(); clients.push(client);
    const values = new Map<string, string>();
    const storage = { keys: () => [...values.keys()], getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => { values.set(key, value); }, removeItem: (key: string) => { values.delete(key); } };
    const draft = prepareCollaborationMemoryChange(scope, memory(), requestId, { action: "update", baseVersion: "1", content: "stale input" });
    const receipt = parseCollaborationMemoryReceipt({ request_id: requestId, action: "memory_update", status: 409,
      code: "stale_memory_version", protocol_version: 1, issue_id: issueId, version: null, current_version: "2", max_bytes: 128 })!;
    const update = vi.spyOn(api, "updateCollaborationMemory").mockResolvedValueOnce({ ...receipt, request_id: issueId });
    await expect(executeCollaborationMemoryChange(client, scope, draft, false, storage)).rejects.toThrow(CollaborationUnavailableError);
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([draft]);
    update.mockResolvedValueOnce(receipt);
    await expect(executeCollaborationMemoryChange(client, scope, draft, false, storage)).resolves.toEqual(receipt);
    expect(readCollaborationDrafts(draft.scope, storage)).toEqual([]);
    expect(update.mock.calls[1]).toEqual(update.mock.calls[0]);
  });
});
