/** @vitest-environment jsdom */
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import type { StorageAdapter } from "../types/storage";

// Keep authentication and its real API callback connected; only unrelated
// background services are disabled in this provider integration suite.
vi.mock("../realtime", () => ({
  WSProvider: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("../client-usage", () => ({ ClientUsageReporter: () => null }));
vi.mock("../diagnostics/freeze-watchdog", () => ({ installFreezeWatchdog: vi.fn() }));
vi.mock("../analytics", () => ({
  captureSignupSource: vi.fn(), identify: vi.fn(), initAnalytics: vi.fn(), resetAnalytics: vi.fn(),
}));

const user = { id: "device-user", name: "Device user", email: "device@example.test", avatar_url: null };
const deviceAuth = { deviceId: "a".repeat(32), deviceName: "Test device" };

function deferredResponse() {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((done) => { resolve = done; });
  return { promise, resolve };
}

async function mountCore({ cookieAuth = true, strictMode = false, storedToken }: {
  cookieAuth?: boolean;
  strictMode?: boolean;
  storedToken?: string;
} = {}) {
  const { CoreProvider } = await import("./core-provider");
  const { useAuthStore } = await import("../auth");
  const { getApi } = await import("../api");
  const values = new Map<string, string>(storedToken ? [["multica_token", storedToken]] : []);
  const storage: StorageAdapter = {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => { values.set(key, value); },
    removeItem: (key) => { values.delete(key); },
    keys: () => [...values.keys()],
  };
  const onLogin = vi.fn();
  const onSessionExpired = vi.fn();
  function SessionScreen() {
    const isLoading = useAuthStore((state) => state.isLoading);
    const currentUser = useAuthStore((state) => state.user);
    return <div>{isLoading ? "Restoring session" : currentUser ? "Signed in" : "Sign in"}</div>;
  }
  render(
    <CoreProvider apiBaseUrl="https://api.example.test" storage={storage}
      cookieAuth={cookieAuth} deviceAuth={deviceAuth} identity={{ platform: "web" }}
      onLogin={onLogin} onSessionExpired={onSessionExpired} locale="en" resources={{ en: {} }}>
      <SessionScreen />
    </CoreProvider>,
    { reactStrictMode: strictMode },
  );
  return { api: getApi(), useAuthStore, storage, onLogin, onSessionExpired };
}

beforeEach(() => { vi.resetModules(); });
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("CoreProvider authentication handoff", () => {
  it.each([false, true])("keeps the session pending until device authentication resolves (StrictMode: %s)", async (strictMode) => {
    const identities: ReturnType<typeof deferredResponse>[] = [];
    const device = deferredResponse();
    const deviceRequested = vi.fn();
    vi.stubGlobal("fetch", vi.fn((url: string) => {
      if (url.endsWith("/api/me")) {
        const identity = deferredResponse();
        identities.push(identity);
        return identity.promise;
      }
      if (url.endsWith("/api/config")) return Promise.resolve(Response.json({ device_auth_available: true }));
      if (url.endsWith("/auth/device")) { deviceRequested(); return device.promise; }
      if (url.endsWith("/api/workspaces")) return Promise.resolve(Response.json([]));
      throw new Error(`Unexpected request: ${url}`);
    }));
    const { useAuthStore, onLogin, onSessionExpired } = await mountCore({ strictMode });
    const signedOut = vi.fn();
    const unsubscribe = useAuthStore.subscribe((state) => {
      if (!state.isLoading && !state.user) signedOut();
    });
    try {
      await waitFor(() => expect(identities).toHaveLength(strictMode ? 2 : 1));
      await act(async () => {
        for (const identity of identities) identity.resolve(Response.json({ error: "unauthorized" }, { status: 401 }));
      });
      await waitFor(() => expect(deviceRequested).toHaveBeenCalledOnce());
      expect(screen.getByText("Restoring session")).toBeTruthy();
      expect(onSessionExpired).not.toHaveBeenCalled();
      expect(signedOut).not.toHaveBeenCalled();
      await act(async () => { device.resolve(Response.json({ token: "device-token", user })); });
      await waitFor(() => expect(screen.getByText("Signed in")).toBeTruthy());
      expect(onLogin).toHaveBeenCalledOnce();
      expect(useAuthStore.getState()).toMatchObject({ status: "authenticated", expired: false });
      expect(signedOut).not.toHaveBeenCalled();
    } finally { unsubscribe(); }
  });

  it("settles a rejected stored session when device authentication is disabled", async () => {
    vi.stubGlobal("fetch", vi.fn((url: string) => {
      if (url.endsWith("/api/config")) return Promise.resolve(Response.json({ device_auth_available: false }));
      if (url.endsWith("/api/me")) return Promise.resolve(Response.json({ error: "unauthorized" }, { status: 401 }));
      throw new Error(`Unexpected request: ${url}`);
    }));
    const { useAuthStore, storage, onSessionExpired } = await mountCore({ cookieAuth: false, storedToken: "rejected-token" });
    await waitFor(() => expect(screen.getByText("Sign in")).toBeTruthy());
    expect(useAuthStore.getState()).toMatchObject({ status: "unauthenticated", expired: true });
    expect(storage.getItem("multica_token")).toBeNull();
    expect(onSessionExpired).toHaveBeenCalledOnce();
  });

  it("settles signed out when the device login itself is rejected", async () => {
    vi.stubGlobal("fetch", vi.fn((url: string) => {
      if (url.endsWith("/api/config")) return Promise.resolve(Response.json({ device_auth_available: true }));
      if (url.endsWith("/api/me")) return Promise.resolve(Response.json({ error: "unauthorized" }, { status: 401 }));
      if (url.endsWith("/auth/device")) return Promise.resolve(Response.json({ error: "device rejected" }, { status: 403 }));
      throw new Error(`Unexpected request: ${url}`);
    }));
    const { useAuthStore, onLogin, onSessionExpired } = await mountCore();
    await waitFor(() => expect(screen.getByText("Sign in")).toBeTruthy());
    expect(useAuthStore.getState().status).toBe("unauthenticated");
    expect(onLogin).not.toHaveBeenCalled();
    expect(onSessionExpired).toHaveBeenCalledOnce();
  });

  it("still expires an authenticated session on a later unauthorized response", async () => {
    let rejectWorkspace = false;
    vi.stubGlobal("fetch", vi.fn((url: string) => {
      if (url.endsWith("/api/config")) return Promise.resolve(Response.json({}));
      if (url.endsWith("/api/me")) return Promise.resolve(Response.json(user));
      if (url.endsWith("/api/workspaces")) return Promise.resolve(rejectWorkspace
        ? Response.json({ error: "unauthorized" }, { status: 401 }) : Response.json([]));
      throw new Error(`Unexpected request: ${url}`);
    }));
    const { api, useAuthStore, onSessionExpired } = await mountCore();
    await waitFor(() => expect(screen.getByText("Signed in")).toBeTruthy());
    rejectWorkspace = true;
    await act(async () => { await expect(api.listWorkspaces()).rejects.toMatchObject({ status: 401 }); });
    expect(screen.getByText("Sign in")).toBeTruthy();
    expect(useAuthStore.getState()).toMatchObject({ status: "unauthenticated", expired: true });
    expect(onSessionExpired).toHaveBeenCalledOnce();
  });
});
