// @vitest-environment node
import { beforeEach, expect, it, vi } from "vitest";
import { ApiError } from "../api";
import { adminOverviewOptions, adminAlertOptions, adminAlertsOptions, adminCollaborationGateOptions } from "./observability-queries";
const state = vi.hoisted(() => ({ user: "actor", session: "scope", api: { getAdminOverview: vi.fn(), getAdminAlerts: vi.fn(), getAdminAlert: vi.fn(), getAdminCollaborationGate: vi.fn() } }));
vi.mock("../api", async original => ({ ...await original<typeof import("../api")>(), getApi: () => state.api }));
vi.mock("../auth", () => ({ useAuthStore: { getState: () => ({ user: { id: state.user } }) } }));
vi.mock("./queries", async original => ({ ...await original<typeof import("./queries")>(), adminApiScope: () => state.session }));
vi.mock("./use-admin-access", () => ({ useAdminAccess: vi.fn() }));
const scope = { apiScope: "scope", userId: "actor", organizationId: "org" };
beforeEach(() => { vi.resetAllMocks(); state.session = "scope"; });
it("includes the complete normalized window in its scoped query key and avoids background polling", () => {
  const params = new URLSearchParams("timezone=Asia%2FShanghai&time_to=end&time_from=start");
  const options = adminOverviewOptions(scope, params);
  expect(options.queryKey).toEqual(["admin", "scope", "actor", "org", "overview", { query: "time_from=start&time_to=end&timezone=Asia%2FShanghai" }]);
  expect(options.refetchInterval).toBe(15000);
  expect(options.refetchIntervalInBackground).toBe(false);
});
it("rejects late data after a session change and another organization's detail", async () => {
  state.api.getAdminOverview.mockImplementationOnce(async () => { state.session = "other"; return { scope: "org" }; });
  const overview = adminOverviewOptions(scope, new URLSearchParams());
  await expect(overview.queryFn!({ signal: new AbortController().signal } as never)).rejects.toThrow("session");
  state.session = "scope";
  state.api.getAdminAlert.mockResolvedValueOnce({ id: "alert", organizationId: "other" });
  const alert = adminAlertOptions(scope, "alert");
  await expect(alert.queryFn!({ signal: new AbortController().signal } as never)).rejects.toThrow();
});
it("does not silently bound active alerts to a new time window", () => {
  const options = adminAlertsOptions(scope, new URLSearchParams());
  expect(options.queryKey.at(-1)).toEqual({ query: "" });
});
it("omits collaboration switches on a server without the route but still rejects another organization's gate", async () => {
  const gate = adminCollaborationGateOptions(scope);
  const context = { signal: new AbortController().signal } as never;
  state.api.getAdminCollaborationGate.mockRejectedValueOnce(new ApiError("Not found", 404, "Not Found"));
  await expect(gate.queryFn!(context)).resolves.toBeNull();
  state.api.getAdminCollaborationGate.mockRejectedValueOnce(new ApiError("Unavailable", 503, "Service Unavailable"));
  await expect(gate.queryFn!(context)).rejects.toThrow("Unavailable");
  state.api.getAdminCollaborationGate.mockResolvedValueOnce({ scope: "other" });
  await expect(gate.queryFn!(context)).rejects.toThrow();
});
