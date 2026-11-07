// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../api";
import { adminApiScope } from "./queries";
import { adminUsersOptions, submitAdminUserChange, AdminUserOperationUncertainError } from "./user-queries";
import type { AdminScope } from "./queries";

const state = vi.hoisted(() => ({
  userId: "actor", session: "session-one",
  api: {
    getBaseUrl: () => "https://test.invalid",
    getSessionScope: (): string => state.session,
    getAdminUsers: vi.fn(), getAdminUser: vi.fn(), getAdminOperations: vi.fn(),
    changeAdminAccount: vi.fn(), changeAdminRole: vi.fn(),
  },
}));
vi.mock("../api", async (original) => ({ ...await original<typeof import("../api")>(), getApi: () => state.api }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: state.userId } }) } }));
const operation = { id: "operation", organizationId: "organization", targetId: "target", kind: "user.disable", state: "applied", resultCode: "account_disabled", version: 1, confirmation: "not_required" };
const input = { id: "target", key: "original-key", action: "disable" as const, body: { expectedAuthVersion: 1, password: "current-password", reason: "Access review" } };
let scope: AdminScope;
beforeEach(() => {
  vi.clearAllMocks(); state.session = "session-one";
  scope = { apiScope: adminApiScope(), userId: state.userId, organizationId: "organization" };
});

describe("administrative account requests", () => {
  it("keys directory queries by the complete platform scope and filters", () => {
    const filters = { q: "name", status: "disabled", cursor: "next", timeFrom: "2026-01-01T00:00:00Z" };
    expect(adminUsersOptions(scope, filters).queryKey).toEqual(["admin", scope.apiScope, "actor", "organization", "users", filters]);
  });
  it("recovers a lost response without issuing the write a second time", async () => {
    state.api.changeAdminAccount.mockRejectedValueOnce(new TypeError("Network disconnected"));
    state.api.getAdminOperations.mockResolvedValueOnce({ items: [operation], scope: "organization" });
    await expect(submitAdminUserChange(scope, input)).resolves.toEqual(operation);
    expect(state.api.changeAdminAccount).toHaveBeenCalledTimes(1);
    expect(state.api.getAdminOperations).toHaveBeenCalledWith("original-key");
  });
  it("retains the original key when no durable result is found", async () => {
    state.api.changeAdminAccount.mockRejectedValueOnce(new TypeError("Network disconnected"));
    state.api.getAdminOperations.mockResolvedValueOnce({ items: [], scope: "organization" });
    await expect(submitAdminUserChange(scope, input)).rejects.toEqual(new AdminUserOperationUncertainError("original-key"));
    state.api.changeAdminAccount.mockResolvedValueOnce(operation);
    await expect(submitAdminUserChange(scope, input)).resolves.toEqual(operation);
    expect(state.api.changeAdminAccount.mock.calls.map((call) => call[3])).toEqual(["original-key", "original-key"]);
  });
  it("keeps password verification errors as definite form errors", async () => {
    const error = new ApiError("Incorrect password", 403, "Forbidden", { code: "password_verification_failed" });
    state.api.changeAdminAccount.mockRejectedValueOnce(error);
    await expect(submitAdminUserChange(scope, input)).rejects.toBe(error);
    expect(state.api.getAdminOperations).not.toHaveBeenCalled();
  });
  it("does not recover an old request against a newly selected server/session", async () => {
    state.api.changeAdminAccount.mockImplementationOnce(async () => { state.session = "new-session"; throw new TypeError("Disconnected"); });
    await expect(submitAdminUserChange(scope, input)).rejects.toThrow("session changed");
    expect(state.api.getAdminOperations).not.toHaveBeenCalled();
  });
});
