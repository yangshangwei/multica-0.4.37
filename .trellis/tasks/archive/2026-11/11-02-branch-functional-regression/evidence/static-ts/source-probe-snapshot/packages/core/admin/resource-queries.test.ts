// @vitest-environment node
import { beforeEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "../api";
import { createAuthStore, registerAuthStore } from "../auth";
import { EMPTY_USER } from "../api/schemas";
import { adminApiScope } from "./queries";
import { adminResourcesOptions, AdminResourceUncertainError, findAdminResourceOperation, submitAdminResource } from "./resource-queries";
import type { AdminResourceMutationInput } from "./resource-queries";
import type { AdminResourceResult } from "./resource-schema";

const userId = "11111111-1111-4111-8111-111111111111", organizationId = "22222222-2222-4222-8222-222222222222", operationId = "33333333-3333-4333-8333-333333333333";
let api: ApiClient;
const scope = () => ({ apiScope: adminApiScope(), userId, organizationId });
const input: AdminResourceMutationInput = { action: "withdraw", kind: "skill", key: "sample", operationId, expectedVersion: "r1", reason: "obsolete" };
const receipt: AdminResourceResult = { operationId, replayed: true, resource: { kind: "skill", key: "sample", name: "Sample", description: "", source: "managed", state: "withdrawn", version: "r2", contentDigest: "d1", fileCount: 1, byteCount: 10, updatedAt: null, updatedBy: null } };
beforeEach(() => {
  api = new ApiClient("https://admin.example"); setApiInstance(api);
  const store = createAuthStore({ api, storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } });
  registerAuthStore(store); store.setState({ user: { ...EMPTY_USER, id: userId }, status: "authenticated" });
});
it("separates catalogs by kind and fails closed on unsupported responses", async () => {
  expect(adminResourcesOptions(scope(), "skill").queryKey).not.toEqual(adminResourcesOptions(scope(), "mcp").queryKey);
  vi.spyOn(api, "getAdminResources").mockResolvedValue(null);
  await expect(new QueryClient().fetchQuery(adminResourcesOptions(scope(), "skill"))).rejects.toThrow("Resource publishing is unsupported");
});
it("checks the same operation receipt after an unknown write without resending", async () => {
  const write = vi.spyOn(api, "withdrawAdminResource").mockRejectedValue(new ApiError("Unknown", 503, "", { code: "resource_outcome_unknown" }));
  const lookup = vi.spyOn(api, "getAdminResourceOperation").mockResolvedValue(receipt);
  await expect(submitAdminResource(scope(), input)).resolves.toEqual(receipt);
  expect(lookup).toHaveBeenCalledWith(operationId);
  expect(write).toHaveBeenCalledTimes(1);
});
it("retains an unknown operation until its own matching receipt is available", async () => {
  vi.spyOn(api, "withdrawAdminResource").mockRejectedValue(new Error("network"));
  vi.spyOn(api, "getAdminResourceOperation").mockRejectedValue(new ApiError("Missing", 404, ""));
  await expect(submitAdminResource(scope(), input)).rejects.toBeInstanceOf(AdminResourceUncertainError);
  vi.mocked(api.getAdminResourceOperation).mockResolvedValue({ ...receipt, operationId: userId });
  await expect(findAdminResourceOperation(scope(), input)).rejects.toThrow();
});
it("leaves determinate validation and permission errors with their caller", async () => {
  const lookup = vi.spyOn(api, "getAdminResourceOperation");
  const denied = new ApiError("Forbidden", 403, "", { code: "admin_forbidden" });
  vi.spyOn(api, "withdrawAdminResource").mockRejectedValue(denied);
  await expect(submitAdminResource(scope(), input)).rejects.toBe(denied);
  expect(lookup).not.toHaveBeenCalled();
});
it("rejects old session responses without querying a receipt as the new user", async () => {
  const captured = scope();
  const lookup = vi.spyOn(api, "getAdminResourceOperation");
  vi.spyOn(api, "withdrawAdminResource").mockImplementation(async () => { api.setToken("new-session"); return receipt; });
  await expect(submitAdminResource(captured, input)).rejects.toThrow("Admin session changed");
  expect(lookup).not.toHaveBeenCalled();
});
