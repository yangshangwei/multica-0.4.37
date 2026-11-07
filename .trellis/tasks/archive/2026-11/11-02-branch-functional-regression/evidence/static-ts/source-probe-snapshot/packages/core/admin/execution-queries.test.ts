// @vitest-environment node
import { beforeEach, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "../api";
import { createAuthStore, registerAuthStore } from "../auth";
import { EMPTY_USER } from "../api/schemas";
import { adminApiScope } from "./queries";
import { adminExecutionOptions, adminExecutionListOptions, adminIssueListOptions } from "./execution-queries";
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
it("isolates all execution filters and the organization in the query key", () => {
  const scope = {
    apiScope: adminApiScope(), userId, organizationId
  };
  const a = adminExecutionListOptions(scope, new URLSearchParams("status=failed&cursor=one"));
  const b = adminExecutionListOptions(scope, new URLSearchParams("status=failed&cursor=two"));
  expect(a.queryKey).not.toEqual(b.queryKey);
  expect(a.queryKey.slice(0, 4)).toEqual(["admin", scope.apiScope, userId, organizationId]);
  expect(adminIssueListOptions(scope, new URLSearchParams()).queryKey).not.toEqual(a.queryKey);
  expect(adminExecutionOptions(scope, userId).refetchInterval).toBe(5000);
});
it("rejects malformed lists and responses from a different organization", async () => {
  const scope = {
    apiScope: adminApiScope(), userId, organizationId
  };
  vi.spyOn(api, "getAdminTasks").mockResolvedValue(null);
  await expect(new QueryClient().fetchQuery(adminExecutionListOptions(scope, new URLSearchParams()))).rejects.toThrow();
  vi.spyOn(api, "getAdminTasks").mockResolvedValue({
    items: [], nextCursor: null, asOf: "2026-10-01T00:00:00Z", scope: userId, timeFrom: null, timeTo: null
  });
  await expect(new QueryClient().fetchQuery(adminExecutionListOptions(scope, new URLSearchParams()))).rejects.toThrow();
});
