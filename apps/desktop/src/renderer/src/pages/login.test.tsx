import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { RESOURCES } from "@multica/views/locales";
import { configStore } from "@multica/core/config";

const DEVICE_ID = "a".repeat(32);

const mocks = vi.hoisted(() => ({
  loginWithDevice: vi.fn(),
  listWorkspaces: vi.fn(),
  sendCode: vi.fn(),
  verifyCode: vi.fn(),
  setToken: vi.fn(),
  getMe: vi.fn(),
  issueCliToken: vi.fn(),
  openExternal: vi.fn(),
  testRuntimeConfig: vi.fn(),
  saveRuntimeConfig: vi.fn(),
  daemonStop: vi.fn(),
}));

vi.mock("@multica/core/auth", () => {
  const state = {
    loginWithDevice: mocks.loginWithDevice,
    sendCode: mocks.sendCode,
    verifyCode: mocks.verifyCode,
  };
  return {
    useAuthStore: Object.assign(
      (selector?: (s: typeof state) => unknown) =>
        selector ? selector(state) : state,
      { getState: () => state },
    ),
  };
});

vi.mock("@multica/core/api", () => {
  const api = {
    listWorkspaces: mocks.listWorkspaces,
    verifyCode: mocks.verifyCode,
    setToken: mocks.setToken,
    getMe: mocks.getMe,
    issueCliToken: mocks.issueCliToken,
  };
  return { api, getApi: () => api };
});

import { DesktopLoginPage } from "./login";

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nProvider locale="en" resources={{ en: RESOURCES.en }}>
        <DesktopLoginPage />
      </I18nProvider>
    </QueryClientProvider>,
  );
}

describe("DesktopLoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.testRuntimeConfig.mockResolvedValue({ ok: true, latencyMs: 12 });
    mocks.listWorkspaces.mockResolvedValue([]);
    mocks.getMe.mockRejectedValue(new Error("unauthorized"));

    // Server declared nothing beyond the defaults: no Google, no device auth.
    configStore.getState().setAuthConfig({ allowSignup: true });
    configStore.getState().setServerVersion("");

    Object.defineProperty(window, "desktopAPI", {
      configurable: true,
      value: {
        runtimeConfig: {
          ok: true,
          config: {
            schemaVersion: 1,
            apiUrl: "https://multica.example.com",
            appUrl: "https://app.multica.example.com",
            wsUrl: "wss://multica.example.com/ws",
          },
        },
        testRuntimeConfig: mocks.testRuntimeConfig,
        saveRuntimeConfig: mocks.saveRuntimeConfig,
        openExternal: mocks.openExternal,
        deviceIdentity: { deviceId: DEVICE_ID, deviceName: "artisan@mac-mini" },
        systemLocale: "en-US",
      },
    });
    Object.defineProperty(window, "daemonAPI", {
      configurable: true,
      value: { stop: mocks.daemonStop },
    });
  });

  it("names the connected deployment instead of the managed cloud", async () => {
    renderPage();

    expect(
      screen.getByText("Sign in to multica.example.com"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/^sign in to multica$/i)).not.toBeInTheDocument();
    expect(screen.getByText("https://multica.example.com")).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByText("Connected")).toBeInTheDocument(),
    );
  });

  it("keeps the Google button off a deployment that declared no client id", () => {
    renderPage();

    expect(
      screen.queryByRole("button", { name: /continue with google/i }),
    ).not.toBeInTheDocument();
  });

  it("opens Google in the browser once the server declares a client id", async () => {
    configStore
      .getState()
      .setAuthConfig({ allowSignup: true, googleClientId: "goog-123" });
    renderPage();

    fireEvent.click(
      screen.getByRole("button", { name: /continue with google/i }),
    );

    expect(mocks.openExternal).toHaveBeenCalledWith(
      "https://app.multica.example.com/login?platform=desktop",
    );
  });

  it("shows the server version next to the address once the probe answers", async () => {
    configStore.getState().setServerVersion("v0.4.37");
    renderPage();

    expect(
      await screen.findByText("Connected · server version v0.4.37"),
    ).toBeInTheDocument();
  });

  it("offers a retry when the server cannot be reached, without blocking sign-in", async () => {
    mocks.testRuntimeConfig.mockResolvedValue({
      ok: false,
      category: "network",
      message: "Connection refused",
    });
    renderPage();

    expect(
      await screen.findByText("Cannot reach this server"),
    ).toBeInTheDocument();
    expect(screen.getByText(/connection refused/i)).toBeInTheDocument();
    // The probe is advisory — a deployment behind a redirecting proxy fails it
    // and still signs in, so the action stays reachable.
    expect(
      screen.getByRole("button", { name: /^continue$/i }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    await waitFor(() =>
      expect(mocks.testRuntimeConfig).toHaveBeenCalledTimes(2),
    );
  });

  // -------------------------------------------------------------------------
  // Intranet device auth — the deployment where an email form is a dead end
  // -------------------------------------------------------------------------

  it("replaces the email form with a single device button", async () => {
    configStore
      .getState()
      .setAuthConfig({ allowSignup: true, deviceAuthAvailable: true });
    mocks.loginWithDevice.mockResolvedValue({ id: "user-1" });
    renderPage();

    expect(
      screen.getByText("Connect to multica.example.com"),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText(/email/i)).not.toBeInTheDocument();
    expect(screen.getByText("This device: artisan@mac-mini")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: /continue as this device/i }),
    );

    expect(mocks.loginWithDevice).toHaveBeenCalledWith(
      DEVICE_ID,
      "artisan@mac-mini",
    );
    await waitFor(() => expect(mocks.listWorkspaces).toHaveBeenCalled());
  });

  it("reports a rejected device login and leaves the button usable", async () => {
    configStore
      .getState()
      .setAuthConfig({ allowSignup: true, deviceAuthAvailable: true });
    mocks.loginWithDevice.mockRejectedValue(
      new Error("device auth is not enabled"),
    );
    renderPage();

    fireEvent.click(
      screen.getByRole("button", { name: /continue as this device/i }),
    );

    expect(
      await screen.findByText("device auth is not enabled"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /continue as this device/i }),
    ).toBeEnabled();
  });

  it("still offers the email form when this machine has no device identity", () => {
    configStore
      .getState()
      .setAuthConfig({ allowSignup: true, deviceAuthAvailable: true });
    Object.defineProperty(window, "desktopAPI", {
      configurable: true,
      value: { ...window.desktopAPI, deviceIdentity: null },
    });
    renderPage();

    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /continue as this device/i }),
    ).not.toBeInTheDocument();
  });

  // -------------------------------------------------------------------------
  // Change server — the only entry point that works while signed out
  // -------------------------------------------------------------------------

  it("prefills the endpoint editor with the current address and comes back", async () => {
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: /change server/i }));

    expect(screen.getByLabelText("Backend address")).toHaveValue(
      "https://multica.example.com",
    );

    fireEvent.click(screen.getByRole("button", { name: /back to sign-in/i }));

    expect(
      screen.getByText("Sign in to multica.example.com"),
    ).toBeInTheDocument();
  });
});
