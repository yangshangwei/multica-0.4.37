// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../api";
import { adminApiScope, type AdminScope } from "./queries";
import { submitAdminControl, findAdminControlOperation, AdminControlUncertainError, adminOperationOptions } from "./operation-queries";
import { readAdminControlDraft, saveAdminControlDraft, removeAdminControlDraft } from "./operation-draft";
const state = vi.hoisted(() => ({ userId: "actor", session: "session", api: {
  getBaseUrl: () => "https://test.invalid", getSessionScope: (): string => state.session,
  changeAdminAdmission: vi.fn(), cancelAdminExecution: vi.fn(), getAdminOperations: vi.fn(), getAdminOperation: vi.fn(),
} }));
vi.mock("../api", async original => ({ ...await original<typeof import("../api")>(), getApi: () => state.api }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: state.userId } }) } }));
vi.mock("./operation-draft", () => ({ readAdminControlDraft: vi.fn(() => null), saveAdminControlDraft: vi.fn(), removeAdminControlDraft: vi.fn() }));
const receipt = { id: "operation", organizationId: "organization", actorId: "actor", targetId: "target", kind: "task.cancel", state: "applied", confirmation: "pending", reconciliationState: "pending" };
const input = { id: "target", key: "original-key", action: "cancel" as const, body: { expectedExecutionFence: { runtimeId: null, dispatchedAt: null, targetVersion: "5" }, reason: "Operator request" } };
let scope: AdminScope;
beforeEach(() => { vi.resetAllMocks(); state.session = "session"; scope = { apiScope: adminApiScope(), userId: "actor", organizationId: "organization" }; });
describe("control submission and recovery", () => {
  it("does not send a command when its original intent cannot be saved", async () => {
    vi.mocked(saveAdminControlDraft).mockImplementationOnce(() => { throw new Error("Storage unavailable"); });
    await expect(submitAdminControl(scope, input)).rejects.toThrow("Storage unavailable");
    expect(state.api.cancelAdminExecution).not.toHaveBeenCalled();
  });
  it("keeps an earlier uncertain intent after a generic conflict and empty lookup", async () => {
    vi.mocked(readAdminControlDraft).mockReturnValueOnce({ scope: { server: "", userId: "actor", organizationId: "organization" }, input, createdAt: 1 });
    state.api.cancelAdminExecution.mockRejectedValueOnce(new ApiError("Conflict", 409, "Conflict"));
    state.api.getAdminOperations.mockResolvedValueOnce({ items: [], scope: "organization" });
    await expect(submitAdminControl(scope, input)).rejects.toEqual(new AdminControlUncertainError(input.key));
    expect(removeAdminControlDraft).not.toHaveBeenCalled();
  });
  it("permits rebasing only after a serialized obsolete admission version and an empty original-key lookup", async () => {
    const admission = { id: input.id, key: input.key, action: "admission" as const, body: { admission: "stopped" as const, expectedAdmissionVersion: "5", reason: "Original request" } };
    vi.mocked(readAdminControlDraft).mockReturnValueOnce({ scope: { server: "", userId: "actor", organizationId: "organization" }, input: admission, createdAt: 1 });
    const conflict = new ApiError("Stale admission", 409, "Conflict", { code: "admission_version_conflict" });
    state.api.changeAdminAdmission.mockRejectedValueOnce(conflict);
    state.api.getAdminOperations.mockResolvedValueOnce({ items: [], scope: "organization" });
    await expect(submitAdminControl(scope, admission)).rejects.toBe(conflict);
    expect(state.api.getAdminOperations).toHaveBeenCalledWith(admission.key);
    expect(removeAdminControlDraft).toHaveBeenCalledWith(expect.objectContaining({ userId: "actor", organizationId: "organization" }), admission.id, admission.key);
  });
  it("does not retire obsolete admission intent when reconciliation is unavailable", async () => {
    const admission = { id: input.id, key: input.key, action: "admission" as const, body: { admission: "stopped" as const, expectedAdmissionVersion: "5", reason: "Original request" } };
    vi.mocked(readAdminControlDraft).mockReturnValueOnce({ scope: { server: "", userId: "actor", organizationId: "organization" }, input: admission, createdAt: 1 });
    state.api.changeAdminAdmission.mockRejectedValueOnce(new ApiError("Stale admission", 409, "Conflict", { code: "admission_version_conflict" }));
    state.api.getAdminOperations.mockRejectedValueOnce(new TypeError("Offline"));
    await expect(submitAdminControl(scope, admission)).rejects.toEqual(new AdminControlUncertainError(admission.key));
    expect(removeAdminControlDraft).not.toHaveBeenCalled();
  });
  it("recovers a durable failed cancellation receipt after a conflicting retry", async () => {
    vi.mocked(readAdminControlDraft).mockReturnValueOnce({ scope: { server: "", userId: "actor", organizationId: "organization" }, input, createdAt: 1 });
    state.api.cancelAdminExecution.mockRejectedValueOnce(new ApiError("Stale execution", 409, "Conflict", { code: "execution_fence_conflict" }));
    const failed = { ...receipt, state: "failed", confirmation: "not_required", reconciliationState: "complete", resultCode: "execution_fence_conflict" };
    state.api.getAdminOperations.mockResolvedValueOnce({ items: [failed], scope: "organization" });
    await expect(submitAdminControl(scope, input)).resolves.toEqual(failed);
    expect(removeAdminControlDraft).toHaveBeenCalledWith(expect.objectContaining({ userId: "actor", organizationId: "organization" }), input.id, input.key);
    expect(state.api.cancelAdminExecution).toHaveBeenCalledTimes(1);
  });
  it("shows the durable failed receipt on an initial execution-fence rejection", async () => {
    state.api.cancelAdminExecution.mockRejectedValueOnce(new ApiError("Stale execution", 409, "Conflict", { code: "execution_fence_conflict" }));
    const failed = { ...receipt, state: "failed", confirmation: "not_required", reconciliationState: "complete", resultCode: "execution_fence_conflict" };
    state.api.getAdminOperations.mockResolvedValueOnce({ items: [failed], scope: "organization" });
    await expect(submitAdminControl(scope, input)).resolves.toEqual(failed);
    expect(state.api.getAdminOperations).toHaveBeenCalledWith(input.key);
  });
  it("queries the original key after a lost response and does not repeat the write", async () => {
    state.api.cancelAdminExecution.mockRejectedValueOnce(new TypeError("Disconnected"));
    state.api.getAdminOperations.mockResolvedValueOnce({ items: [receipt], scope: "organization" });
    await expect(submitAdminControl(scope, input)).resolves.toEqual(receipt);
    expect(state.api.cancelAdminExecution).toHaveBeenCalledTimes(1);
    expect(state.api.getAdminOperations).toHaveBeenCalledWith(input.key);
  });
  it("retains uncertainty after an empty lookup and retries exactly the original snapshot", async () => {
    state.api.cancelAdminExecution.mockResolvedValueOnce(null);
    state.api.getAdminOperations.mockResolvedValueOnce({ items: [], scope: "organization" });
    await expect(submitAdminControl(scope, input)).rejects.toEqual(new AdminControlUncertainError(input.key));
    state.api.cancelAdminExecution.mockResolvedValueOnce({ operation: receipt, target: { id: "target" } });
    await expect(submitAdminControl(scope, input)).resolves.toEqual(receipt);
    expect(state.api.cancelAdminExecution.mock.calls).toEqual([[input.id, input.body, input.key], [input.id, input.body, input.key]]);
  });
  it("rejects another actor, target, kind or organization during recovery", async () => {
    for (const patch of [{ actorId: "other" }, { targetId: "other" }, { kind: "user.disable" }, { organizationId: "other" }]) {
      state.api.getAdminOperations.mockResolvedValueOnce({ items: [{ ...receipt, ...patch }], scope: "organization" });
      await expect(findAdminControlOperation(scope, input)).rejects.toThrow();
    }
  });
  it("does not retry definite version conflicts or permission denials", async () => {
    for (const status of [403, 409]) {
      const error = new ApiError("Rejected", status, "Rejected");
      state.api.cancelAdminExecution.mockRejectedValueOnce(error);
      await expect(submitAdminControl(scope, input)).rejects.toBe(error);
    }
    expect(state.api.getAdminOperations).not.toHaveBeenCalled();
  });
  it("preserves a permission denial found during recovery for cache revocation", async () => {
    const denied = new ApiError("Denied", 403, "Forbidden");
    state.api.cancelAdminExecution.mockRejectedValueOnce(new TypeError("Disconnected"));
    state.api.getAdminOperations.mockRejectedValueOnce(denied);
    await expect(submitAdminControl(scope, input)).rejects.toBe(denied);
  });
  it("does not query or commit results into another session", async () => {
    state.api.cancelAdminExecution.mockImplementationOnce(async () => { state.session = "new"; return { operation: receipt, target: { id: "target" } }; });
    await expect(submitAdminControl(scope, input)).rejects.toThrow("session changed");
    expect(state.api.getAdminOperations).not.toHaveBeenCalled();
  });
  it("uses a scoped operation key without background polling", () => {
    const options = adminOperationOptions(scope, "operation");
    expect(options.queryKey).toEqual(["admin", scope.apiScope, "actor", "organization", "operation", { id: "operation" }]);
    expect(options.refetchIntervalInBackground).toBe(false);
    expect(options.retry).toBe(false);
  });
});
