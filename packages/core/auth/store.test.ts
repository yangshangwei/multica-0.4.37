import { describe, expect, it, vi } from "vitest";
import type { ApiClient } from "../api/client";
import type { StorageAdapter, User } from "../types";
import { createAuthStore } from "./store";

const fakeUser: User = {
  id: "u1",
  name: "Alice",
  email: "alice@example.com",
  avatar_url: null,
} as User;

function makeStorage(initial: Record<string, string> = {}): StorageAdapter & {
  snapshot: () => Record<string, string>;
} {
  const data = { ...initial };
  return {
    getItem: (k) => data[k] ?? null,
    setItem: (k, v) => {
      data[k] = v;
    },
    removeItem: (k) => {
      delete data[k];
    },
    snapshot: () => ({ ...data }),
  };
}

function makeApi(): ApiClient {
  return {
    setToken: vi.fn(),
  } as unknown as ApiClient;
}

describe("authStore", () => {
  it("publishes a retry request instead of silently ignoring it", () => {
    const storage = makeStorage({ multica_token: "t" });
    const api = makeApi();
    const store = createAuthStore({ api, storage });

    store.setState({ isLoading: true, status: "recovering" });
    store.getState().retryAuthentication();

    expect(store.getState().status).toBe("authenticating");
    expect(store.getState().retryGeneration).toBe(1);
  });

  it("explicit logout still clears credentials and publishes unauthenticated state", () => {
    const storage = makeStorage({ multica_token: "t" });
    const api = makeApi();
    const onLogout = vi.fn();
    const store = createAuthStore({ api, storage, onLogout });

    store.setState({ user: fakeUser, status: "authenticated", isLoading: false });
    store.getState().logout();

    expect(storage.snapshot().multica_token).toBeUndefined();
    expect(api.setToken).toHaveBeenCalledWith(null);
    expect(onLogout).toHaveBeenCalledOnce();
    expect(store.getState().user).toBeNull();
    expect(store.getState().status).toBe("unauthenticated");
  });

  it("stores the session a device login returns", async () => {
    const storage = makeStorage();
    const deviceLogin = vi
      .fn()
      .mockResolvedValue({ token: "device-jwt", user: fakeUser });
    const api = { setToken: vi.fn(), deviceLogin } as unknown as ApiClient;
    const onLogin = vi.fn();
    const store = createAuthStore({ api, storage, onLogin });

    const user = await store
      .getState()
      .loginWithDevice("a".repeat(32), "artisan@mac-mini");

    expect(deviceLogin).toHaveBeenCalledWith("a".repeat(32), "artisan@mac-mini");
    expect(storage.snapshot().multica_token).toBe("device-jwt");
    expect(api.setToken).toHaveBeenCalledWith("device-jwt");
    expect(onLogin).toHaveBeenCalledOnce();
    expect(user).toEqual(fakeUser);
    expect(store.getState().status).toBe("authenticated");
  });

  // The api client answers an unreadable body with an empty session instead of
  // throwing, so the store is the last place that can stop a "signed in" state
  // whose every request 401s.
  it("rejects an empty device session instead of persisting it", async () => {
    const storage = makeStorage();
    const api = {
      setToken: vi.fn(),
      deviceLogin: vi
        .fn()
        .mockResolvedValue({ token: "", user: { ...fakeUser, id: "" } }),
    } as unknown as ApiClient;
    const store = createAuthStore({ api, storage });

    await expect(
      store.getState().loginWithDevice("b".repeat(32)),
    ).rejects.toThrow();
    expect(storage.snapshot().multica_token).toBeUndefined();
    expect(api.setToken).not.toHaveBeenCalled();
    expect(store.getState().status).not.toBe("authenticated");
  });
});
