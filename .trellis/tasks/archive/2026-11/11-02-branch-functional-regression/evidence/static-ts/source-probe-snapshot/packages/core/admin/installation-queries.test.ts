// @vitest-environment node
import { beforeEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { createAuthStore, registerAuthStore } from "../auth";
import { EMPTY_USER } from "../api/schemas";
import { adminApiScope } from "./queries";
import { adminInstallationsOptions, adminInstallationOptions, adminUnassociatedRuntimesOptions } from "./installation-queries";
const userId = "11111111-1111-4111-8111-111111111111", organizationId = "22222222-2222-4222-8222-222222222222";
let api: ApiClient;
beforeEach(() => {
  api = new ApiClient("https://example.test");
  setApiInstance(api);
  const store = createAuthStore({
    api, storage: {
      getItem: () => null, setItem: vi.fn(), removeItem: vi.fn()
    }
  });
  registerAuthStore(store);
  store.setState({ user: { ...EMPTY_USER, id: userId }, status: "authenticated" });
});
it("isolates installation, legacy and filtered pages while polling only foreground", () => {
  const scope = {
    apiScope: adminApiScope(), userId, organizationId
  };
  const list = adminInstallationsOptions(scope, new URLSearchParams("lifecycle=active"));
  expect(list.queryKey).not.toEqual(adminUnassociatedRuntimesOptions(scope, new URLSearchParams()).queryKey);
  expect(list.refetchIntervalInBackground).toBe(false);
  expect(adminInstallationOptions(scope, userId).refetchInterval).toBe(5000);
});
it("rejects malformed or different-organization installation responses", async () => {
  const scope = {
    apiScope: adminApiScope(), userId, organizationId
  };
  const request = vi.spyOn(api, "getAdminInstallations").mockResolvedValue(null);
  await expect(new QueryClient().fetchQuery(adminInstallationsOptions(scope, new URLSearchParams()))).rejects.toThrow();
  request.mockResolvedValue({
    items: [], asOf: "2026-10-01T00:00:00Z", scope: userId, nextCursor: null, dataQuality: "complete"
  });
  await expect(new QueryClient().fetchQuery(adminInstallationsOptions(scope, new URLSearchParams()))).rejects.toThrow();
});
it("does not reuse an old installation response after the credential changes", async () => {
  const scope = {
    apiScope: adminApiScope(), userId, organizationId
  };
  let release!: (value: {
    items: [
    ];
    asOf: string;
    scope: string;
    nextCursor: null;
    dataQuality: "complete";
  }) => void;
  vi.spyOn(api, "getAdminInstallations").mockImplementation(() => new Promise(resolve => {
    release = resolve;
  }));
  const request = new QueryClient().fetchQuery(adminInstallationsOptions(scope, new URLSearchParams()));
  api.setToken("replacement-session");
  release({
    items: [], asOf: "2026-10-01T00:00:00Z", scope: organizationId, nextCursor: null, dataQuality: "complete"
  });
  await expect(request).rejects.toThrow("Admin session changed");
});
