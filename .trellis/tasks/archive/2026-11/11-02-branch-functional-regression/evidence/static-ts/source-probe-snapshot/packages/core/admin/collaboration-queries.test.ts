// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiError } from "../api";
import { adminApiScope, AdminUnsupportedError, type AdminScope } from "./queries";
import { AdminControlUncertainError } from "./operation-queries";
import { configureAdminControlStorage, readAdminCollaborationDraft, saveAdminControlDraft } from "./operation-draft";
import {
  AdminCollaborationRejectedError, AdminCollaborationStaleError, adminCollaborationCandidatesOptions, adminCollaborationIssueOptions,
  adminCollaborationIssuesOptions, adminCollaborationSnapshotMatches, captureAdminCollaborationInput,
  dismissAdminCollaborationReceipt, findAdminCollaborationOperation, submitAdminCollaboration,
} from "./collaboration-queries";
import { adminCollaborationCandidatesFixture as candidates, adminCollaborationDetailFixture as detail,
  adminCollaborationInputFixture as input, adminCollaborationResultFixture as receipt, adminCollaborationTestId as id } from "./test-fixtures";

const state = vi.hoisted(() => ({ userId: "", authStatus: "authenticated", session: "session", server: "https://admin.example.test", api: {
  getBaseUrl: (): string => state.server, getSessionScope: (): string => state.session,
  getAdminCollaborationIssues: vi.fn(), getAdminCollaborationIssue: vi.fn(), getAdminCollaborationCandidates: vi.fn(),
  getAdminCollaborationOperation: vi.fn(), changeAdminCollaborationLifecycle: vi.fn(),
} }));
vi.mock("../api", async original => ({ ...await original<typeof import("../api")>(), getApi: () => state.api }));
vi.mock("../auth", () => ({ useAuthStore: Object.assign((selector: (value: { user: { id: string }; status: string }) => unknown) => selector({ user: { id: state.userId }, status: state.authStatus }),
  { getState: () => ({ user: { id: state.userId }, status: state.authStatus }) }) }));
let scope: AdminScope;
let values: Map<string, string>;
let client: QueryClient;
const storageScope = () => ({ server: state.server, userId: scope.userId, organizationId: id(2) });
const pending = () => readAdminCollaborationDraft(storageScope(), input.id);
const notFound = () => new ApiError("Not recorded", 404, "Not found", { code: "admin_not_found" });
beforeEach(() => {
  vi.resetAllMocks();
  state.userId = id(1); state.authStatus = "authenticated"; state.session = "session"; state.server = "https://admin.example.test";
  scope = { apiScope: adminApiScope(), userId: id(1), organizationId: id(2) };
  values = new Map();
  configureAdminControlStorage({ getItem: key => values.get(key) ?? null, setItem: (key, value) => { values.set(key, value); },
    removeItem: key => { values.delete(key); }, keys: () => [...values.keys()] });
  client = new QueryClient();
});

describe("protected collaboration administration reads", () => {
  it("uses administration keys scoped by server/session, actor and organization", () => {
    const options = adminCollaborationIssueOptions(scope, input.id);
    expect(options.queryKey).toEqual(["admin", scope.apiScope, id(1), id(2), "collaboration-issue", { issueId: input.id }]);
    expect(options).toMatchObject({ retry: false, staleTime: 0, gcTime: 0, refetchOnWindowFocus: false });
  });
  it("reads the protected minimal detail without an ordinary collaboration request", async () => {
    state.api.getAdminCollaborationIssue.mockResolvedValueOnce(detail);
    expect(await client.fetchQuery(adminCollaborationIssueOptions(scope, input.id))).toEqual(detail);
    expect(state.api.getAdminCollaborationIssue).toHaveBeenCalledWith(input.id, { signal: expect.any(AbortSignal) });
  });
  it.each(["list", "detail", "candidates"])("rejects a different organization on the %s response", async resource => {
    if (resource === "list") {
      state.api.getAdminCollaborationIssues.mockResolvedValueOnce({ scope: id(20), items: [detail], nextCursor: null });
      await expect(client.fetchInfiniteQuery(adminCollaborationIssuesOptions(scope))).rejects.toThrow(AdminUnsupportedError);
    } else if (resource === "detail") {
      state.api.getAdminCollaborationIssue.mockResolvedValueOnce({ ...detail, scope: id(20) });
      await expect(client.fetchQuery(adminCollaborationIssueOptions(scope, input.id))).rejects.toThrow(AdminUnsupportedError);
    } else {
      state.api.getAdminCollaborationCandidates.mockResolvedValueOnce({ ...candidates, scope: id(20) });
      await expect(client.fetchInfiniteQuery(adminCollaborationCandidatesOptions(scope, input.id, "7"))).rejects.toThrow(AdminUnsupportedError);
    }
  });
  it("rejects another issue and a changed candidate revision", async () => {
    state.api.getAdminCollaborationIssue.mockResolvedValueOnce({ ...detail, issueId: id(20) });
    await expect(client.fetchQuery(adminCollaborationIssueOptions(scope, input.id))).rejects.toThrow(AdminUnsupportedError);
    state.api.getAdminCollaborationCandidates.mockResolvedValueOnce({ ...candidates, collaborationRevision: "8" });
    await expect(client.fetchInfiniteQuery(adminCollaborationCandidatesOptions(scope, input.id, "7"))).rejects.toThrow(AdminCollaborationStaleError);
  });
  it("lets paginated candidates advance only beyond the captured page boundary", async () => {
    state.api.getAdminCollaborationCandidates.mockResolvedValueOnce({ ...candidates, items: [{ ...candidates.items[0]!, userId: id(8) }] });
    const options = { ...adminCollaborationCandidatesOptions(scope, input.id, "7"), initialPageParam: id(7) };
    expect((await client.fetchInfiniteQuery(options)).pages[0]?.items[0]?.userId).toBe(id(8));
    expect(state.api.getAdminCollaborationCandidates).toHaveBeenCalledWith(input.id, { limit: 50, cursor: id(7) }, { signal: expect.any(AbortSignal) });
    client.clear();
    state.api.getAdminCollaborationCandidates.mockResolvedValueOnce(candidates);
    await expect(client.fetchInfiniteQuery(options)).rejects.toThrow(AdminUnsupportedError);
  });
  it("does not install a result or an old denial after the session changes", async () => {
    state.api.getAdminCollaborationIssue.mockImplementationOnce(async () => { state.session = "new"; return detail; });
    await expect(client.fetchQuery(adminCollaborationIssueOptions(scope, input.id))).rejects.toThrow("session changed");
    expect(client.getQueryData(adminCollaborationIssueOptions(scope, input.id).queryKey)).toBeUndefined();
    state.session = "session";
    state.api.getAdminCollaborationIssue.mockImplementationOnce(async () => { state.userId = id(20); throw new ApiError("Old denial", 403, "Forbidden"); });
    await expect(client.fetchQuery(adminCollaborationIssueOptions(scope, input.id))).rejects.toThrow("session changed");
  });
});

describe("frozen lifecycle confirmation", () => {
  it("captures a selected participant from a later page without replacing the original creator", () => {
    expect(captureAdminCollaborationInput(detail, input.key, input.body, [{ ...candidates, items: [], nextCursor: null }, candidates])).toEqual(input);
  });
  it.each(["organization", "issue", "revision", "selection"])("refuses a stale or missing candidate %s", field => {
    const page = { ...candidates, ...(field === "organization" ? { scope: id(20) } : field === "issue" ? { issueId: id(20) }
      : field === "revision" ? { collaborationRevision: "8" } : { items: [] }) };
    expect(() => captureAdminCollaborationInput(detail, input.key, input.body, [page])).toThrow(AdminCollaborationStaleError);
  });
  it("requires explicit successor selection only for a double-disabled reopen", () => {
    const { successorUserId: _unused, ...withoutSuccessor } = input.body;
    expect(() => captureAdminCollaborationInput(detail, input.key, withoutSuccessor, [candidates])).toThrow();
    const activeHandler = { ...detail, handler: { ...detail.handler, disabled: false }, successorRequired: false };
    expect(captureAdminCollaborationInput(activeHandler, input.key, withoutSuccessor).body).not.toHaveProperty("successorUserId");
    expect(() => captureAdminCollaborationInput(activeHandler, input.key, input.body, [candidates])).toThrow();
  });
  it.each(["revision", "status", "creator", "handler", "disabled", "options", "scope", "source"])("does not silently rebase the opened %s confirmation", field => {
    const current = { ...detail, ...(field === "revision" ? { collaborationRevision: "8" } : field === "status" ? { status: "cancelled" }
      : field === "creator" ? { creator: { ...detail.creator, disabled: false } } : field === "handler" ? { handler: { ...detail.handler, userId: id(20) } }
      : field === "disabled" ? { handler: { ...detail.handler, disabled: false } } : field === "scope" ? { scope: id(20) }
      : field === "source" ? { sourceWorkspaceId: id(20) } : { actions: [] }) };
    expect(adminCollaborationSnapshotMatches(detail, current)).toBe(false);
  });
});

describe("durable original-key submission and recovery", () => {
  it("does not send a request after the same account loses complete authentication", async () => {
    state.authStatus = "password_change_required";
    await expect(submitAdminCollaboration(scope, input)).rejects.toThrow("session changed");
    expect(state.api.changeAdminCollaborationLifecycle).not.toHaveBeenCalled();
    expect(values.size).toBe(0);
  });
  it("persists the entire intent before its only POST and keeps it recoverable after a known result", async () => {
    state.api.changeAdminCollaborationLifecycle.mockImplementationOnce(async () => { expect(pending()?.input).toEqual(input); return receipt; });
    await expect(submitAdminCollaboration(scope, input)).resolves.toEqual(receipt);
    expect(state.api.changeAdminCollaborationLifecycle).toHaveBeenCalledExactlyOnceWith(input.id, input.body, input.key);
    expect(pending()?.input).toEqual(input);
    state.api.getAdminCollaborationOperation.mockResolvedValueOnce(receipt);
    await expect(findAdminCollaborationOperation(scope, pending()!.input)).resolves.toEqual(receipt);
    expect(state.api.changeAdminCollaborationLifecycle).toHaveBeenCalledTimes(1);
  });
  it("does not post when persistent storage is unavailable", async () => {
    configureAdminControlStorage({ getItem: () => null, keys: () => [], setItem: () => {}, removeItem: () => {} });
    await expect(submitAdminCollaboration(scope, input)).rejects.toThrow();
    expect(state.api.changeAdminCollaborationLifecycle).not.toHaveBeenCalled();
  });
  it("recovers a lost response by original GET without a second write", async () => {
    state.api.changeAdminCollaborationLifecycle.mockRejectedValueOnce(new TypeError("Lost response"));
    state.api.getAdminCollaborationOperation.mockResolvedValueOnce(receipt);
    await expect(submitAdminCollaboration(scope, input)).resolves.toEqual(receipt);
    expect(state.api.changeAdminCollaborationLifecycle).toHaveBeenCalledTimes(1);
    expect(state.api.getAdminCollaborationOperation).toHaveBeenCalledExactlyOnceWith(input.id, input.key, { signal: undefined });
  });
  it("does not turn an empty lookup into a new key or silently changed successor", async () => {
    state.api.changeAdminCollaborationLifecycle.mockRejectedValueOnce(new TypeError("Offline"));
    state.api.getAdminCollaborationOperation.mockRejectedValueOnce(notFound());
    await expect(submitAdminCollaboration(scope, input)).rejects.toEqual(new AdminControlUncertainError(input.key));
    expect(pending()?.input).toEqual(input);
    for (const changed of [{ ...input, key: id(20) }, { ...input, body: { ...input.body, successorUserId: id(20) } }]) {
      await expect(submitAdminCollaboration(scope, changed)).rejects.toThrow();
    }
    expect(state.api.changeAdminCollaborationLifecycle).toHaveBeenCalledTimes(1);
  });
  it("keeps a previous unknown request frozen after a stale reply and another empty lookup", async () => {
    saveAdminControlDraft(storageScope(), input);
    state.api.changeAdminCollaborationLifecycle.mockRejectedValueOnce(new ApiError("Changed", 409, "Conflict", { code: "stale_revision" }));
    state.api.getAdminCollaborationOperation.mockRejectedValueOnce(notFound());
    await expect(submitAdminCollaboration(scope, input)).rejects.toEqual(new AdminControlUncertainError(input.key));
    expect(pending()?.input).toEqual(input);
  });
  it.each(["stale_revision", "successor_unavailable", "creator_not_disabled", "invalid_reason"])("settles only a first-attempt explicit %s refusal", async code => {
    const refused = new ApiError("Refresh required", code === "invalid_reason" ? 400 : 409, "Conflict", { code });
    state.api.changeAdminCollaborationLifecycle.mockRejectedValueOnce(refused);
    await expect(submitAdminCollaboration(scope, input)).rejects.toEqual(new AdminCollaborationRejectedError(refused));
    expect(pending()).toBeNull();
    expect(state.api.getAdminCollaborationOperation).not.toHaveBeenCalled();
  });
  it.each(["actor", "organization", "issue", "key", "creator", "handler", "successor", "status"])("never settles another %s result", async field => {
    const result = { operation: { ...receipt.operation, ...(field === "actor" ? { actorId: id(20) } : field === "organization" ? { organizationId: id(20) } : {}) },
      result: { ...receipt.result, ...(field === "issue" ? { issueId: id(20) } : field === "key" ? { requestId: id(20) }
        : field === "creator" ? { creatorUserId: id(20) } : field === "handler" ? { previousHandlerUserId: id(20) }
        : field === "successor" ? { handlerUserId: id(20) } : field === "status" ? { status: "in_progress" } : {}) } };
    saveAdminControlDraft(storageScope(), input);
    state.api.getAdminCollaborationOperation.mockResolvedValueOnce(result);
    await expect(findAdminCollaborationOperation(scope, input)).rejects.toThrow(AdminUnsupportedError);
    expect(pending()?.input).toEqual(input);
  });
  it("recovers while the active creator prevents fresh detail reads", async () => {
    state.api.getAdminCollaborationIssue.mockRejectedValue(new ApiError("Creator restored", 409, "Conflict", { code: "creator_not_disabled" }));
    state.api.getAdminCollaborationOperation.mockResolvedValueOnce(receipt);
    await expect(findAdminCollaborationOperation(scope, input)).resolves.toEqual(receipt);
    expect(state.api.getAdminCollaborationIssue).not.toHaveBeenCalled();
  });
  it.each([401, 403])("preserves current %s denials so the administration cache can be cleared", async status => {
    state.api.changeAdminCollaborationLifecycle.mockRejectedValueOnce(new TypeError("Lost response"));
    const denied = new ApiError("Denied", status, "Denied");
    state.api.getAdminCollaborationOperation.mockRejectedValueOnce(denied);
    await expect(submitAdminCollaboration(scope, input)).rejects.toBe(denied);
    expect(pending()?.input).toEqual(input);
  });
  it("rejects an explicit dismissal unless the known receipt matches the original request", () => {
    saveAdminControlDraft(storageScope(), input);
    expect(() => dismissAdminCollaborationReceipt(scope, input, { ...receipt, result: { ...receipt.result, requestId: id(20) } })).toThrow();
    expect(pending()).not.toBeNull();
    dismissAdminCollaborationReceipt(scope, input, receipt);
    expect(pending()).toBeNull();
  });
  it("does not repeat or acknowledge writes after a server/account switch", async () => {
    state.api.changeAdminCollaborationLifecycle.mockImplementationOnce(async () => { state.session = "new"; return receipt; });
    await expect(submitAdminCollaboration(scope, input)).rejects.toThrow("session changed");
    expect(state.api.getAdminCollaborationOperation).not.toHaveBeenCalled();
    expect(() => dismissAdminCollaborationReceipt(scope, input, receipt)).toThrow("session changed");
  });
});
