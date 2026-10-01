// @vitest-environment jsdom
import { createElement, type ReactNode } from "react";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClient, ApiError, setApiInstance } from "../api";
import { createAuthStore, registerAuthStore } from "../auth";
import { EMPTY_USER } from "../api/schemas";
import { adminApiScope, adminKeys } from "./queries";
import { useAdminAccess } from "./use-admin-access";

const user = { ...EMPTY_USER, id: "11111111-1111-4111-8111-111111111111", name: "Admin", email: "", avatar_url: null, created_at: "" };
const identity = { userId: user.id, organizationId: "22222222-2222-4222-8222-222222222222", role: "super_admin" as const, supported: true as const, allowedActions: [] };
let api: ApiClient;
let store: ReturnType<typeof createAuthStore>;
let qc: QueryClient;

beforeEach(() => {
  api = new ApiClient("https://example.test");
  setApiInstance(api);
  store = createAuthStore({ api, storage: { getItem: () => null, setItem: vi.fn(), removeItem: vi.fn() } });
  registerAuthStore(store);
  store.setState({ user, status: "authenticated", isLoading: false });
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});
afterEach(() => { cleanup(); qc.clear(); });

function mount() {
  return renderHook(() => useAdminAccess(), {
    wrapper: ({ children }: { children: ReactNode }) => createElement(QueryClientProvider, { client: qc }, children),
  });
}

describe("admin access lifecycle", () => {
  it("does not probe administration before full authentication", () => {
    store.setState({ status: "password_change_required" });
    const me = vi.spyOn(api, "getAdminMe");
    const { result } = mount();
    expect(result.current.status).toBe("loading");
    expect(me).not.toHaveBeenCalled();
  });

  it("hides details and clears admin caches after any current admin request is forbidden", async () => {
    vi.spyOn(api, "getAdminMe").mockResolvedValue(identity);
    const { result } = mount();
    await waitFor(() => expect(result.current.status).toBe("ready"));
    const key = adminKeys.resource({ apiScope: adminApiScope(), userId: user.id, organizationId: identity.organizationId }, "tasks");
    qc.setQueryData(key, ["sensitive"]);
    qc.setQueryData(["workspace"], ["ordinary"]);
    await act(async () => {
      await qc.fetchQuery({ queryKey: key, staleTime: 0, queryFn: () => Promise.reject(new ApiError("forbidden", 403, "Forbidden")) }).catch(() => undefined);
    });
    await waitFor(() => expect(result.current.status).toBe("denied"));
    expect(result.current.identity).toBeUndefined();
    expect(qc.getQueryData(key)).toBeUndefined();
    expect(qc.getQueryData(["workspace"])).toEqual(["ordinary"]);
    expect(store.getState().status).toBe("authenticated");
  });

  it("preserves the administration session and form error after password verification fails", async () => {
    vi.spyOn(api, "getAdminMe").mockResolvedValue(identity);
    const { result } = mount();
    await waitFor(() => expect(result.current.status).toBe("ready"));
    const key = adminKeys.resource({ apiScope: adminApiScope(), userId: user.id, organizationId: identity.organizationId }, "role-change");
    const error = new ApiError("Verify password again", 403, "Forbidden", { code: "password_verification_failed" });
    const mutation = qc.getMutationCache().build(qc, { mutationKey: key, mutationFn: () => Promise.reject(error) });
    await act(async () => { await mutation.execute(undefined).catch(() => undefined); });
    expect(result.current.status).toBe("ready");
    expect(result.current.identity).toEqual(identity);
    expect(qc.getMutationCache().getAll()).toContain(mutation);
    expect(mutation.state.error).toBe(error);
    expect(store.getState().status).toBe("authenticated");
  });

  it("can recheck access after a denial without preserving protected data", async () => {
    const me = vi.spyOn(api, "getAdminMe").mockRejectedValue(new ApiError("forbidden", 403, "Forbidden"));
    const { result } = mount();
    await waitFor(() => expect(result.current.status).toBe("denied"));
    me.mockResolvedValue(identity);
    act(() => result.current.retry());
    await waitFor(() => expect(result.current.status).toBe("ready"));
    expect(me).toHaveBeenCalledTimes(2);
  });

  it("identifies a disabled server authentication mode separately from missing permission", async () => {
    vi.spyOn(api, "getAdminMe").mockRejectedValue(new ApiError("mode disabled", 403, "Forbidden", { code: "admin_mode_disabled" }));
    const { result } = mount();
    await waitFor(() => expect(result.current.status).toBe("unsupported"));
    expect(result.current.identity).toBeUndefined();
    expect(store.getState().status).toBe("authenticated");
  });

  it("renders unsupported and retains no identity after a malformed response", async () => {
    vi.spyOn(api, "getAdminMe").mockResolvedValue(null);
    const { result } = mount();
    await waitFor(() => expect(result.current.status).toBe("unsupported"));
    expect(result.current.identity).toBeUndefined();
  });

  it("rechecks the same user object when its credential changes", async () => {
    const me = vi.spyOn(api, "getAdminMe").mockResolvedValue(identity);
    const { result } = mount();
    await waitFor(() => expect(result.current.status).toBe("ready"));
    me.mockRejectedValue(new ApiError("forbidden", 403, "Forbidden"));
    act(() => {
      api.setToken("new-session");
      store.setState({ user });
    });
    await waitFor(() => expect(result.current.status).toBe("denied"));
    expect(me).toHaveBeenCalledTimes(2);
  });

  it.each(["account", "credential", "server"])("ignores an old in-flight 403 after a %s switch", async (change) => {
    vi.spyOn(api, "getAdminMe").mockResolvedValue(identity);
    const { result, rerender } = mount();
    await waitFor(() => expect(result.current.status).toBe("ready"));
    const previousKey = adminKeys.resource({ apiScope: adminApiScope(), userId: user.id, organizationId: identity.organizationId }, "old-detail");
    let reject!: (error: Error) => void;
    const pending = qc.fetchQuery({ queryKey: previousKey, queryFn: () => new Promise((_resolve, fail) => { reject = fail; }) }).catch(() => undefined);
    const nextIdentity = change === "account" ? { ...identity, userId: "another-user" } : identity;
    act(() => {
      if (change === "account") {
        vi.spyOn(api, "getAdminMe").mockResolvedValue(nextIdentity);
        store.setState({ user: { ...user, id: nextIdentity.userId } });
      } else if (change === "credential") {
        api.setToken("replacement");
        store.setState({ user });
      } else {
        const replacement = new ApiClient("https://example.test");
        vi.spyOn(replacement, "getAdminMe").mockResolvedValue(identity);
        setApiInstance(replacement);
        rerender();
      }
    });
    await waitFor(() => expect(result.current.identity).toEqual(nextIdentity));
    const nextKey = adminKeys.resource({ apiScope: adminApiScope(), userId: nextIdentity.userId, organizationId: identity.organizationId }, "new-detail");
    qc.setQueryData(nextKey, ["new authorization"]);
    await act(async () => { reject(new ApiError("old forbidden", 403, "Forbidden")); await pending; });
    expect(result.current.status).toBe("ready");
    expect(result.current.identity).toEqual(nextIdentity);
    expect(qc.getQueryData(nextKey)).toEqual(["new authorization"]);
  });

  it("drops the previous organization's resource cache after scope refresh", async () => {
    const me = vi.spyOn(api, "getAdminMe").mockResolvedValue(identity);
    const { result } = mount();
    await waitFor(() => expect(result.current.status).toBe("ready"));
    const key = adminKeys.resource({ apiScope: adminApiScope(), userId: user.id, organizationId: identity.organizationId }, "tasks");
    qc.setQueryData(key, ["old organization"]);
    me.mockResolvedValue({ ...identity, organizationId: "33333333-3333-4333-8333-333333333333" });
    act(() => result.current.retry());
    await waitFor(() => expect(qc.getQueryData(key)).toBeUndefined());
    expect(result.current.identity?.organizationId).not.toBe(identity.organizationId);
  });
});
