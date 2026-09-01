/**
 * @vitest-environment jsdom
 */
import { act, render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { setApiInstance } from "../api";
import { ApiError, type ApiClient } from "../api/client";
import {
  createAuthStore,
  registerAuthStore,
  useAuthStore,
} from "../auth";
import type { StorageAdapter, User, Workspace } from "../types";
import { workspaceKeys } from "../workspace/queries";
import { AuthInitializer } from "./auth-initializer";
import { configStore } from "../config";

const logger = vi.hoisted(() => ({
  debug: vi.fn(),
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn(),
}));

vi.mock("../logger", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../logger")>();
  return { ...actual, createLogger: () => logger };
});

vi.mock("../analytics", () => ({
  captureSignupSource: vi.fn(),
  identify: vi.fn(),
  initAnalytics: vi.fn(),
  resetAnalytics: vi.fn(),
}));

const fakeUser = {
  id: "user-1",
  name: "Alice",
  email: "alice@example.com",
  avatar_url: null,
} as User;

const fakeWorkspaces = [{ id: "ws-1", slug: "acme" }] as Workspace[];

function makeStorage(initial: Record<string, string> = {}): StorageAdapter & {
  snapshot: () => Record<string, string>;
} {
  const values = { ...initial };
  return {
    getItem: (key) => values[key] ?? null,
    setItem: (key, value) => {
      values[key] = value;
    },
    removeItem: (key) => {
      delete values[key];
    },
    snapshot: () => ({ ...values }),
  };
}

function makeApi(overrides: Partial<ApiClient> = {}): ApiClient {
  return {
    getConfig: vi.fn().mockResolvedValue({}),
    getMe: vi.fn().mockResolvedValue(fakeUser),
    listWorkspaces: vi.fn().mockResolvedValue(fakeWorkspaces),
    setToken: vi.fn(),
    ...overrides,
  } as unknown as ApiClient;
}

function renderInitializer({
  api,
  storage = makeStorage({ multica_token: "token-1" }),
  cookieAuth = false,
  platform = "desktop",
  deviceAuth,
}: {
  api: ApiClient;
  storage?: StorageAdapter;
  cookieAuth?: boolean;
  platform?: "desktop" | "web";
  deviceAuth?: { deviceId: string; deviceName?: string };
}) {
  const onLogin = vi.fn();
  const onLogout = vi.fn();
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  setApiInstance(api);
  registerAuthStore(
    createAuthStore({ api, storage, cookieAuth, onLogin, onLogout }),
  );

  const result = render(
    <QueryClientProvider client={queryClient}>
      <AuthInitializer
        cookieAuth={cookieAuth}
        identity={{ platform }}
        onLogin={onLogin}
        onLogout={onLogout}
        storage={storage}
        deviceAuth={deviceAuth}
      >
        <div>child</div>
      </AuthInitializer>
    </QueryClientProvider>,
  );

  return { ...result, onLogin, onLogout, queryClient };
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("AuthInitializer recovery", () => {
  it("keeps the token and recovers on the online event after a network failure", async () => {
    const storage = makeStorage({ multica_token: "token-1" });
    const getMe = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("fetch failed"))
      .mockResolvedValue(fakeUser);
    const api = makeApi({ getMe });
    const { onLogout } = renderInitializer({ api, storage });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("recovering");
    });
    expect(storage.snapshot().multica_token).toBe("token-1");
    expect(onLogout).not.toHaveBeenCalled();

    act(() => window.dispatchEvent(new Event("online")));

    await waitFor(() => {
      expect(useAuthStore.getState().user).toEqual(fakeUser);
    });
    expect(getMe).toHaveBeenCalledTimes(2);
    expect(onLogout).not.toHaveBeenCalled();
  });

  it("lets a manual retry restart a recoverable auth attempt", async () => {
    const getMe = vi
      .fn()
      .mockRejectedValueOnce(new ApiError("unavailable", 503, "Unavailable"))
      .mockResolvedValue(fakeUser);
    const api = makeApi({ getMe });
    const { onLogout } = renderInitializer({ api });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("recovering");
    });
    act(() => useAuthStore.getState().retryAuthentication());

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("authenticated");
    });
    expect(getMe).toHaveBeenCalledTimes(2);
    expect(onLogout).not.toHaveBeenCalled();
  });

  it("continues automatic retries at the capped backoff interval", async () => {
    vi.useFakeTimers();
    const getMe = vi.fn().mockRejectedValue(new TypeError("still offline"));
    const api = makeApi({ getMe });
    const { onLogout } = renderInitializer({ api });

    await act(async () => {
      await Promise.resolve();
    });
    expect(useAuthStore.getState().status).toBe("recovering");

    await act(async () => {
      await vi.advanceTimersByTimeAsync(31_000);
    });
    expect(getMe).toHaveBeenCalledTimes(6);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(30_000);
    });
    expect(getMe).toHaveBeenCalledTimes(7);
    expect(logger.error).toHaveBeenCalledTimes(1);
    expect(onLogout).not.toHaveBeenCalled();
  });

  it("does not invalidate a verified desktop session when workspace loading fails", async () => {
    const listWorkspaces = vi
      .fn()
      .mockRejectedValue(new TypeError("network unavailable"));
    const api = makeApi({ listWorkspaces });
    const { onLogout, queryClient } = renderInitializer({ api });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("authenticated");
      expect(
        queryClient.getQueryState(workspaceKeys.list())?.status,
      ).toBe("error");
    });
    expect(useAuthStore.getState().user).toEqual(fakeUser);
    expect(onLogout).not.toHaveBeenCalled();
  });

  it("publishes web auth independently when workspace loading fails", async () => {
    const listWorkspaces = vi
      .fn()
      .mockRejectedValue(new TypeError("backend restarting"));
    const api = makeApi({ listWorkspaces });
    const { onLogout, queryClient } = renderInitializer({
      api,
      cookieAuth: true,
      platform: "web",
    });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("authenticated");
      expect(queryClient.getQueryState(workspaceKeys.list())?.status).toBe(
        "error",
      );
    });
    expect(useAuthStore.getState().user).toEqual(fakeUser);
    expect(onLogout).not.toHaveBeenCalled();
  });

  it("reloads app config after auth recovers from a transient failure", async () => {
    const getConfig = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("network unavailable"))
      .mockResolvedValue({ feature_flags: { recovered: true } });
    const getMe = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("network unavailable"))
      .mockResolvedValue(fakeUser);
    const api = makeApi({ getConfig, getMe });
    renderInitializer({ api });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("recovering");
    });
    act(() => useAuthStore.getState().retryAuthentication());

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("authenticated");
      expect(getConfig).toHaveBeenCalledTimes(2);
    });
  });

  it("keeps retrying app config until it loads", async () => {
    vi.useFakeTimers();
    const getConfig = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("network unavailable"))
      .mockRejectedValueOnce(new TypeError("still unavailable"))
      .mockResolvedValue({ feature_flags: { recovered: true } });
    const api = makeApi({ getConfig });
    renderInitializer({ api });

    await act(async () => {
      await Promise.resolve();
    });
    expect(getConfig).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3_000);
    });
    expect(getConfig).toHaveBeenCalledTimes(3);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(getConfig).toHaveBeenCalledTimes(3);
  });

  it("publishes a definitive logout for a genuine 401", async () => {
    const storage = makeStorage({ multica_token: "token-1" });
    const getMe = vi.fn().mockImplementation(() => {
      storage.removeItem("multica_token");
      return Promise.reject(new ApiError("unauthorized", 401, "Unauthorized"));
    });
    const api = makeApi({ getMe });
    const { onLogout } = renderInitializer({ api, storage });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("unauthenticated");
    });
    expect(storage.snapshot().multica_token).toBeUndefined();
    expect(onLogout).toHaveBeenCalledOnce();
  });
});

describe("AuthInitializer device auth", () => {
  const deviceAuth = { deviceId: "a".repeat(32), deviceName: "artisan@mac-mini" };

  beforeEach(() => {
    // configStore is a module singleton, and a test whose config load fails
    // would otherwise inherit the previous test's capability.
    configStore.setState({ deviceAuthAvailable: false });
  });

  it("signs in from the device identity when the server declares it", async () => {
    const storage = makeStorage();
    const deviceLogin = vi
      .fn()
      .mockResolvedValue({ token: "device-jwt", user: fakeUser });
    const api = makeApi({
      getConfig: vi.fn().mockResolvedValue({ device_auth_available: true }),
      deviceLogin,
    } as Partial<ApiClient>);
    const { onLogin } = renderInitializer({ api, storage, deviceAuth });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("authenticated");
    });
    expect(deviceLogin).toHaveBeenCalledWith(deviceAuth.deviceId, deviceAuth.deviceName);
    expect(storage.snapshot().multica_token).toBe("device-jwt");
    expect(useAuthStore.getState().user).toEqual(fakeUser);
    expect(onLogin).toHaveBeenCalled();
    // The workspace list is warmed the same way the token path warms it, so the
    // shell has a definitive answer on its first render.
    await waitFor(() => {
      expect(api.listWorkspaces).toHaveBeenCalled();
    });
  });

  it("shows the login page when the server does not declare device auth", async () => {
    const storage = makeStorage();
    const deviceLogin = vi.fn();
    const api = makeApi({
      getConfig: vi.fn().mockResolvedValue({}),
      deviceLogin,
    } as Partial<ApiClient>);
    const { onLogout } = renderInitializer({ api, storage, deviceAuth });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("unauthenticated");
    });
    // Absent means 403 — probing it would only trade a usable login page for a
    // failed request.
    expect(deviceLogin).not.toHaveBeenCalled();
    expect(onLogout).toHaveBeenCalled();
  });

  it("keeps retrying while the server is unreachable instead of dropping to the login page", async () => {
    const storage = makeStorage();
    const deviceLogin = vi
      .fn()
      .mockResolvedValue({ token: "device-jwt", user: fakeUser });
    const getConfig = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("fetch failed"))
      .mockResolvedValue({ device_auth_available: true });
    const api = makeApi({ getConfig, deviceLogin } as Partial<ApiClient>);
    renderInitializer({ api, storage, deviceAuth });

    // A config request that failed says nothing about the capability. On an
    // intranet it usually means the server is not up yet, and settling for the
    // login page there would strand a client that cannot log in any other way.
    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("recovering");
    });
    expect(deviceLogin).not.toHaveBeenCalled();

    act(() => window.dispatchEvent(new Event("online")));

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("authenticated");
    });
    expect(deviceLogin).toHaveBeenCalledTimes(1);
  });

  it("stops at the login page when the server rejects the device id", async () => {
    const storage = makeStorage();
    const deviceLogin = vi
      .fn()
      .mockRejectedValue(new ApiError("device auth is not enabled", 403, "Forbidden"));
    const api = makeApi({
      getConfig: vi.fn().mockResolvedValue({ device_auth_available: true }),
      deviceLogin,
    } as Partial<ApiClient>);
    const { onLogout } = renderInitializer({ api, storage, deviceAuth });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("unauthenticated");
    });
    // A 4xx is an answer, not a blip: retrying cannot change it.
    expect(deviceLogin).toHaveBeenCalledTimes(1);
    expect(onLogout).toHaveBeenCalled();
  });

  it("leaves a stored session alone instead of re-identifying the client as its host", async () => {
    const storage = makeStorage({ multica_token: "token-1" });
    const deviceLogin = vi.fn();
    const api = makeApi({
      getConfig: vi.fn().mockResolvedValue({ device_auth_available: true }),
      deviceLogin,
    } as Partial<ApiClient>);
    renderInitializer({ api, storage, deviceAuth });

    await waitFor(() => {
      expect(useAuthStore.getState().user).toEqual(fakeUser);
    });
    expect(api.getMe).toHaveBeenCalled();
    expect(deviceLogin).not.toHaveBeenCalled();
    expect(storage.snapshot().multica_token).toBe("token-1");
  });
});

// The web app's session lives in an HttpOnly cookie it cannot read, so it has
// no "signed out" branch to take before asking the server. Its only signal is a
// 401 from getMe, which is where the device path has to be reachable from.
describe("AuthInitializer device auth in cookie mode", () => {
  const deviceAuth = { deviceId: "b".repeat(32), deviceName: "web-macos-bbbbbbbb" };
  const unauthorized = () =>
    new ApiError("unauthorized", 401, "Unauthorized");

  beforeEach(() => {
    configStore.setState({ deviceAuthAvailable: false });
  });

  it("signs in from the device identity after the cookie session comes back 401", async () => {
    const storage = makeStorage();
    const deviceLogin = vi
      .fn()
      .mockResolvedValue({ token: "device-jwt", user: fakeUser });
    const api = makeApi({
      getMe: vi.fn().mockRejectedValue(unauthorized()),
      getConfig: vi.fn().mockResolvedValue({ device_auth_available: true }),
      deviceLogin,
    } as Partial<ApiClient>);
    const { onLogin } = renderInitializer({
      api,
      storage,
      cookieAuth: true,
      platform: "web",
      deviceAuth,
    });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("authenticated");
    });
    expect(deviceLogin).toHaveBeenCalledWith(
      deviceAuth.deviceId,
      deviceAuth.deviceName,
    );
    expect(useAuthStore.getState().user).toEqual(fakeUser);
    expect(onLogin).toHaveBeenCalled();
    // Cookie mode: /auth/device set the HttpOnly cookies. Mirroring the bearer
    // token into localStorage is the exact exposure the cookie migration
    // exists to remove.
    expect(storage.snapshot().multica_token).toBeUndefined();
  });

  it("settles logged out on a 401 when the client has no device identity", async () => {
    const storage = makeStorage();
    const deviceLogin = vi.fn();
    const api = makeApi({
      getMe: vi.fn().mockRejectedValue(unauthorized()),
      getConfig: vi.fn().mockResolvedValue({ device_auth_available: true }),
      deviceLogin,
    } as Partial<ApiClient>);
    const { onLogout } = renderInitializer({
      api,
      storage,
      cookieAuth: true,
      platform: "web",
    });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("unauthenticated");
    });
    expect(onLogout).toHaveBeenCalled();
    // The deployment offers device auth; this client has nothing to present, so
    // the login page is still the right answer. Regression guard for every
    // platform that passes no identity.
    expect(deviceLogin).not.toHaveBeenCalled();
  });

  it("settles logged out on a 401 when the deployment does not offer device auth", async () => {
    const storage = makeStorage();
    const deviceLogin = vi.fn();
    const api = makeApi({
      getMe: vi.fn().mockRejectedValue(unauthorized()),
      getConfig: vi.fn().mockResolvedValue({}),
      deviceLogin,
    } as Partial<ApiClient>);
    const { onLogout } = renderInitializer({
      api,
      storage,
      cookieAuth: true,
      platform: "web",
      deviceAuth,
    });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("unauthenticated");
    });
    expect(deviceLogin).not.toHaveBeenCalled();
    expect(onLogout).toHaveBeenCalled();
  });

  it("leaves a live cookie session alone", async () => {
    const deviceLogin = vi.fn();
    const api = makeApi({
      getConfig: vi.fn().mockResolvedValue({ device_auth_available: true }),
      deviceLogin,
    } as Partial<ApiClient>);
    renderInitializer({
      api,
      storage: makeStorage(),
      cookieAuth: true,
      platform: "web",
      deviceAuth,
    });

    await waitFor(() => {
      expect(useAuthStore.getState().user).toEqual(fakeUser);
    });
    expect(deviceLogin).not.toHaveBeenCalled();
  });

  it("does not mint a second identity when the warmed workspace list 401s", async () => {
    const deviceLogin = vi
      .fn()
      .mockResolvedValue({ token: "device-jwt", user: fakeUser });
    const api = makeApi({
      getMe: vi.fn().mockRejectedValue(unauthorized()),
      getConfig: vi.fn().mockResolvedValue({ device_auth_available: true }),
      deviceLogin,
      listWorkspaces: vi.fn().mockRejectedValue(unauthorized()),
    } as Partial<ApiClient>);
    renderInitializer({
      api,
      storage: makeStorage(),
      cookieAuth: true,
      platform: "web",
      deviceAuth,
    });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("unauthenticated");
    });
    // A session that cannot read its own workspace list is broken in a way
    // another identity would not fix — and each attempt would leave one more
    // member row behind.
    expect(deviceLogin).toHaveBeenCalledTimes(1);
  });

  it("re-mints a desktop session whose stored token has expired", async () => {
    const storage = makeStorage({ multica_token: "stale-token" });
    const deviceLogin = vi
      .fn()
      .mockResolvedValue({ token: "fresh-jwt", user: fakeUser });
    const api = makeApi({
      getMe: vi.fn().mockRejectedValue(unauthorized()),
      getConfig: vi.fn().mockResolvedValue({ device_auth_available: true }),
      deviceLogin,
    } as Partial<ApiClient>);
    renderInitializer({ api, storage, deviceAuth });

    await waitFor(() => {
      expect(useAuthStore.getState().status).toBe("authenticated");
    });
    // Token mode keeps persisting: the desktop app has no cookie jar to fall
    // back on, and the expired token must not outlive the session it named.
    expect(storage.snapshot().multica_token).toBe("fresh-jwt");
    expect(deviceLogin).toHaveBeenCalledTimes(1);
  });
});
