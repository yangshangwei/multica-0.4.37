import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { SupportedLocale } from "@multica/core/i18n";
import { RESOURCES } from "@multica/views/locales";
import type { DaemonStatus } from "../../../shared/daemon-types";

const { authState, reauthenticateDaemon } = vi.hoisted(() => ({
  authState: { user: { id: "user-1" } as { id: string } | null },
  reauthenticateDaemon: vi.fn<() => Promise<void>>(),
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector: (state: typeof authState) => unknown) => selector(authState),
    { getState: () => authState },
  ),
}));
vi.mock("../platform/daemon-reauth", () => ({ reauthenticateDaemon }));

import { DaemonSettingsTab } from "./daemon-settings-tab";

function stubDaemonAPI(status: DaemonStatus) {
  vi.stubGlobal("daemonAPI", {
    getPrefs: vi.fn(async () => ({ autoStart: true, autoStop: false })),
    isCliInstalled: vi.fn(async () => true),
    getStatus: vi.fn(async () => status),
    onStatusChange: vi.fn(() => () => {}),
  });
}

function settings(locale: SupportedLocale = "en") {
  return (
    <I18nProvider locale={locale} resources={RESOURCES}>
      <DaemonSettingsTab />
    </I18nProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  authState.user = { id: "user-1" };
  localStorage.setItem("multica_token", "session-token");
  reauthenticateDaemon.mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
  localStorage.clear();
  vi.unstubAllGlobals();
});

describe("DaemonSettingsTab sign-in recovery", () => {
  it("names both daemon preference switches from their visible row labels", async () => {
    stubDaemonAPI({ state: "stopped" });
    render(settings());
    await screen.findByText("multica CLI is installed and available in PATH.");
    expect(screen.getByRole("switch", { name: "Auto-start on launch" })).toBeChecked();
    expect(screen.getByRole("switch", { name: "Auto-stop on quit" })).not.toBeChecked();
  });

  // Session invalidation and transient failures are covered in daemon-reauth.test.ts.
  it.each([
    ["en", "Reconnect this device", "Reconnecting..."],
    ["zh-Hans", "重新连接此设备", "正在重新连接..."],
  ] as const)("offers recovery before auth_expired is detected in %s", async (locale, label, pendingLabel) => {
    stubDaemonAPI({ state: "stopped" });
    let finishReauth!: () => void;
    reauthenticateDaemon.mockImplementation(() => new Promise((resolve) => { finishReauth = resolve; }));
    render(settings(locale));

    const button = await screen.findByRole("button", { name: label });
    await waitFor(() => expect(button).toBeEnabled());
    fireEvent.click(button);

    expect(reauthenticateDaemon).toHaveBeenCalledOnce();
    expect(screen.getByRole("button", { name: pendingLabel })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: pendingLabel }));
    expect(reauthenticateDaemon).toHaveBeenCalledOnce();

    await act(async () => { finishReauth(); });
    expect(screen.getByRole("button", { name: label })).toBeEnabled();
  });

  it("keeps the expired-daemon warning and one recovery action", async () => {
    stubDaemonAPI({ state: "auth_expired" });
    render(settings());

    expect(await screen.findByText("Daemon sign-in expired")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Reconnect this device" })).toHaveLength(1);
    expect(screen.getByRole("button", { name: "Reconnect this device" })).toBeEnabled();
  });

  it("shows a disabled action for a daemon managed outside the app", async () => {
    stubDaemonAPI({ state: "running", externallyManaged: true });
    render(settings());

    await screen.findByText("Running");
    const button = screen.getByRole("button", { name: "Reconnect this device" });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(reauthenticateDaemon).not.toHaveBeenCalled();
  });

  it("waits for the daemon's management status before allowing sign-in", async () => {
    stubDaemonAPI({ state: "stopped" });
    vi.mocked(window.daemonAPI.getStatus).mockReturnValue(new Promise(() => {}));
    render(settings());

    await screen.findByText("multica CLI is installed and available in PATH.");
    expect(screen.getByRole("button", { name: "Reconnect this device" })).toBeDisabled();
  });

  it("allows recovery while a daemon is stuck starting after an inconclusive auth probe", async () => {
    stubDaemonAPI({ state: "starting" });
    render(settings());

    await screen.findByText("Starting...");
    const button = screen.getByRole("button", { name: "Reconnect this device" });
    expect(button).toBeEnabled();
    await act(async () => { fireEvent.click(button); });
    expect(reauthenticateDaemon).toHaveBeenCalledOnce();
  });

  it.each([
    ["stopping", "Stopping..."],
    ["installing_cli", "Setting up..."],
  ] as const)("disables sign-in while the daemon is %s", async (state, label) => {
    stubDaemonAPI({ state });
    render(settings());

    await screen.findByText(label);
    expect(screen.getByRole("button", { name: "Reconnect this device" })).toBeDisabled();
  });

  it.each(["user", "token"])("shows a disabled action without a session %s", async (missing) => {
    stubDaemonAPI({ state: "stopped" });
    if (missing === "user") authState.user = null;
    else localStorage.removeItem("multica_token");
    render(settings());

    await screen.findByText("multica CLI is installed and available in PATH.");
    const button = screen.getByRole("button", { name: "Reconnect this device" });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(reauthenticateDaemon).not.toHaveBeenCalled();
  });
});
