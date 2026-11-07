// @vitest-environment node
import { beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "../api";
import { createAuthStore, registerAuthStore } from "../auth";
import { EMPTY_USER } from "../api/schemas";
import { adminApiScope, adminKeys, adminMeOptions, AdminUnsupportedError, clearAdminCache, retainAdminScope, isAdminPermissionDenied } from "./queries";

const userId = "11111111-1111-4111-8111-111111111111";
const organizationId = "22222222-2222-4222-8222-222222222222";
const identity = { userId, organizationId, role: "super_admin" as const, allowedActions: [], supported: true as const };
let api: ApiClient;
let store: ReturnType<typeof createAuthStore>;

beforeEach(() => {
  api = new ApiClient("https://one.example");
  setApiInstance(api);
  store = createAuthStore({ api, storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } });
  registerAuthStore(store);
  store.setState({ user: { ...EMPTY_USER, id: userId, name: "Admin" }, status: "authenticated", isLoading: false });
});

it("keys isolate deployment, user, organization, resource and all filters", () => {
  const scope = { apiScope: adminApiScope(), userId, organizationId };
  expect(adminKeys.resource(scope, "tasks", { cursor: "next", sort: "created_at", time_from: "today" }))
    .toEqual(["admin", scope.apiScope, userId, organizationId, "tasks", { cursor: "next", sort: "created_at", time_from: "today" }]);
  const before = adminApiScope();
  api.setToken("new-session");
  expect(adminApiScope()).not.toBe(before);
  setApiInstance(new ApiClient("https://one.example"));
  expect(adminApiScope()).not.toBe(before);
  setApiInstance(new ApiClient("https://two.example"));
  expect(adminApiScope()).not.toBe(before);
});

describe("cache isolation", () => {
  it("removes admin queries and mutations but keeps workspace state", () => {
    const qc = new QueryClient();
    const key = adminKeys.me({ apiScope: adminApiScope(), userId, organizationId });
    qc.setQueryData(key, identity);
    qc.setQueryData(["workspace", "one"], { name: "My workspace" });
    const mutation = qc.getMutationCache().build(qc, { mutationKey: key });
    expect(qc.getMutationCache().getAll()).toContain(mutation);
    clearAdminCache(qc);
    expect(qc.getQueryData(key)).toBeUndefined();
    expect(qc.getMutationCache().getAll()).toEqual([]);
    expect(qc.getQueryData(["workspace", "one"])).toEqual({ name: "My workspace" });
  });

  it("keeps only the current account and server scope", () => {
    const qc = new QueryClient();
    const current = { apiScope: adminApiScope(), userId, organizationId };
    const stale = [
      { ...current, userId: "another-user" },
      { ...current, apiScope: "old-server" },
    ];
    qc.setQueryData(adminKeys.me(current), identity);
    stale.forEach((scope) => qc.setQueryData(adminKeys.me(scope), identity));
    retainAdminScope(qc, current);
    expect(qc.getQueryData(adminKeys.me(current))).toEqual(identity);
    stale.forEach((scope) => expect(qc.getQueryData(adminKeys.me(scope))).toBeUndefined());
    retainAdminScope(qc, null);
    expect(qc.getQueryCache().getAll()).toEqual([]);
  });

  it("cancels in-flight requests before they can repopulate cleared data", async () => {
    const qc = new QueryClient();
    let resolve!: (value: string) => void;
    let signal: AbortSignal | undefined;
    const key = adminKeys.me({ apiScope: adminApiScope(), userId, organizationId });
    const pending = qc.fetchQuery({ queryKey: key, queryFn: (context) => {
      signal = context.signal;
      return new Promise<string>((done) => { resolve = done; });
    } }).catch(() => undefined);
    clearAdminCache(qc);
    expect(signal?.aborted).toBe(true);
    resolve("private");
    await pending;
    expect(qc.getQueryData(key)).toBeUndefined();
  });
});

describe("identity query", () => {
  it("rejects a valid response for a different user", async () => {
    vi.spyOn(api, "getAdminMe").mockResolvedValue({ ...identity, userId: "other" });
    const qc = new QueryClient();
    await expect(qc.fetchQuery(adminMeOptions({ apiScope: adminApiScope(), userId, organizationId: null })))
      .rejects.toBeInstanceOf(AdminUnsupportedError);
  });

  it.each(["account", "server"])("rejects a stale successful identity after a %s switch", async (change) => {
    let resolve!: (value: typeof identity) => void;
    vi.spyOn(api, "getAdminMe").mockImplementation(() => new Promise((done) => { resolve = done; }));
    const qc = new QueryClient();
    const options = adminMeOptions({ apiScope: adminApiScope(), userId, organizationId: null });
    const pending = qc.fetchQuery(options);
    if (change === "account") store.setState({ user: { ...EMPTY_USER, id: "new-user" } });
    else setApiInstance(new ApiClient("https://one.example"));
    resolve(identity);
    await expect(pending).rejects.toThrow("Admin session changed");
    expect(qc.getQueryData(options.queryKey)).toBeUndefined();
  });

  it("rejects a response that arrives after a same-user re-login", async () => {
    let resolve!: (value: typeof identity) => void;
    vi.spyOn(api, "getAdminMe").mockImplementation(() => new Promise((done) => { resolve = done; }));
    const pending = new QueryClient().fetchQuery(adminMeOptions({ apiScope: adminApiScope(), userId, organizationId: null }));
    api.setToken("replacement");
    resolve(identity);
    await expect(pending).rejects.toThrow("Admin session changed");
  });
});

describe("administrative authorization errors", () => {
  it.each(["password_verification_failed", "password_verification_stale"])("keeps %s local to the sensitive form", (code) => {
    expect(isAdminPermissionDenied(new ApiError("Verify password again", 403, "Forbidden", { code }))).toBe(false);
  });

  it.each(["admin_forbidden", "admin_session_required", "admin_scope_forbidden", undefined])("continues to fail closed for authorization denial %s", (code) => {
    expect(isAdminPermissionDenied(new ApiError("Forbidden", 403, "Forbidden", { code }))).toBe(true);
  });
});
