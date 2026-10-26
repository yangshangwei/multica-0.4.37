// @vitest-environment node
import { beforeEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiError } from "../api";
import { parseAdminCollaborationMemoryResult, parseAdminCollaborationMemoryVersions } from "./collaboration-schema";
import { adminApiScope, type AdminScope } from "./queries";
import { configureAdminControlStorage, readAdminCollaborationMemoryDraft } from "./operation-draft";
import { AdminControlUncertainError } from "./operation-queries";
import { adminCollaborationMemoryVersionsOptions, captureAdminCollaborationMemoryPurge,
  dismissAdminCollaborationMemoryReceipt, findAdminCollaborationMemoryPurge, submitAdminCollaborationMemoryPurge } from "./collaboration-memory-queries";

const state = vi.hoisted(() => ({ userId: "00000000-0000-4000-8000-000000000001", server: "https://admin.example.test", api: {
  getBaseUrl: (): string => state.server, getSessionScope: () => "session",
  getAdminCollaborationMemoryVersions: vi.fn(), getAdminCollaborationMemoryOperation: vi.fn(), purgeAdminCollaborationMemoryVersion: vi.fn(),
} }));
vi.mock("../api", async original => ({ ...await original<typeof import("../api")>(), getApi: () => state.api }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: state.userId }, status: "authenticated" }) } }));
const id = (n: number) => `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`;
const meta = { version: "1", bytes: 7, author: null, created_at: "2026-10-26T00:00:00Z", restored_from_version: null,
  purged: false, purged_at: null, purged_by: null, purged_by_admin: false };
const rawPage = { scope: id(2), issue_id: id(3), current_version: "2", items: [meta], next_cursor: null };
const rawReceipt = { operation: { id: id(8), organization_id: id(2), target_id: id(3), actor_id: id(1), target_kind: "collaboration_issue",
  kind: "collaboration.purge_memory", state: "applied", result_code: "memory_purged", confirmation: "not_required", reconciliation_state: "complete", version: 1 },
  result: { issue_id: id(3), request_id: id(4), action: "purge_memory", code: "memory_purged", version: "1", current_version: "2", bytes: 7,
    creator_user_id: id(5), purged_at: "2026-10-26T00:00:00Z" } };
let scope: AdminScope;
let values: Map<string, string>;
beforeEach(() => {
  vi.resetAllMocks(); state.userId = id(1);
  scope = { apiScope: adminApiScope(), userId: id(1), organizationId: id(2) };
  values = new Map();
  configureAdminControlStorage({ getItem: key => values.get(key) ?? null, setItem: (key, value) => { values.set(key, value); },
    removeItem: key => { values.delete(key); }, keys: () => [...values.keys()] });
});

it("parses the server's metadata-only page without requiring document limits or invented capabilities", () => {
  expect(parseAdminCollaborationMemoryVersions(rawPage)).toEqual({ scope: id(2), issueId: id(3), currentVersion: "2", items: [meta], nextCursor: null });
  const parsed = parseAdminCollaborationMemoryVersions({ ...rawPage, content: "PRIVATE_CONTEXT", items: [{ ...meta, content: "PRIVATE_VERSION" }] });
  expect(JSON.stringify(parsed)).not.toContain("PRIVATE_");
  expect(parseAdminCollaborationMemoryVersions({ ...rawPage, next_cursor: "2" })).toBeNull();
  expect(parseAdminCollaborationMemoryVersions({ ...rawPage, items: [meta, meta] })).toBeNull();
});

it("validates both the operation and memory receipt before confirming a purge", () => {
  expect(parseAdminCollaborationMemoryResult(rawReceipt)?.result).toMatchObject({ issueId: id(3), creatorUserId: id(5), version: "1" });
  expect(parseAdminCollaborationMemoryResult({ ...rawReceipt, result: { ...rawReceipt.result, version: "2" } })).toBeNull();
  expect(parseAdminCollaborationMemoryResult({ ...rawReceipt, operation: { ...rawReceipt.operation, target_id: id(9) } })).toBeNull();
});

it("isolates metadata by organization before caching it", async () => {
  const client = new QueryClient();
  try {
    state.api.getAdminCollaborationMemoryVersions.mockResolvedValueOnce({ ...parseAdminCollaborationMemoryVersions(rawPage), scope: id(9) });
    await expect(client.fetchInfiniteQuery(adminCollaborationMemoryVersionsOptions(scope, id(3)))).rejects.toThrow();
  } finally { client.clear(); }
});

it("keeps a lost purge response under its exact key until original-result recovery is confirmed", async () => {
  const input = captureAdminCollaborationMemoryPurge(id(3), id(5), id(4), "1", "Reported credential");
  const storageScope = { server: state.server, userId: id(1), organizationId: id(2) };
  state.api.purgeAdminCollaborationMemoryVersion.mockRejectedValueOnce(new TypeError("Connection lost"));
  state.api.getAdminCollaborationMemoryOperation.mockRejectedValueOnce(new ApiError("Not found", 404, "Not found", { code: "admin_not_found" }));
  await expect(submitAdminCollaborationMemoryPurge(scope, input)).rejects.toThrow(AdminControlUncertainError);
  expect(readAdminCollaborationMemoryDraft(storageScope, input.id)?.input).toEqual(input);
  const receipt = parseAdminCollaborationMemoryResult(rawReceipt)!;
  state.api.getAdminCollaborationMemoryOperation.mockResolvedValueOnce(receipt);
  await expect(findAdminCollaborationMemoryPurge(scope, input)).resolves.toEqual(receipt);
  expect(state.api.purgeAdminCollaborationMemoryVersion).toHaveBeenCalledExactlyOnceWith(input.id, input.body, input.key);
  expect(state.api.getAdminCollaborationMemoryOperation).toHaveBeenLastCalledWith(input.id, input.key, { signal: undefined });
  dismissAdminCollaborationMemoryReceipt(scope, input, receipt);
  expect(readAdminCollaborationMemoryDraft(storageScope, input.id)).toBeNull();
});
