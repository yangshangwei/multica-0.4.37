import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
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

function stubDaemonAPI(status: DaemonStatus, cliPath: string | null = "/fixture/bin/multica") {
  vi.stubGlobal("daemonAPI", {
    getPrefs: vi.fn(async () => ({ autoStart: true, autoStop: false })),
    getCliPath: vi.fn(async () => cliPath),
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
    await screen.findByText("A usable multica CLI is available to this app.");
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

    await screen.findByText("A usable multica CLI is available to this app.");
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

    await screen.findByText("A usable multica CLI is available to this app.");
    const button = screen.getByRole("button", { name: "Reconnect this device" });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(reauthenticateDaemon).not.toHaveBeenCalled();
  });
});

// The complete category matrix belongs to platform/daemon-diagnostics.test.ts.
it.each([
  ["en", "Runtime setup", "Device connection", "Synchronize", "Setup Failed"],
  ["zh-Hans", "运行时安装", "设备连接", "同步", "安装失败"],
] as const)("retains separate diagnostics across status updates and remount in %s", async (locale, cliTitle, connectionTitle, timeStep, stateLabel) => {
  const status: DaemonStatus = { state: "cli_not_found", diagnostics: {
    cli: { code: "cli_extract_failed", stage: "cli_setup", occurredAt: 1_800_000_000_000 },
    connection: { code: "challenge_expired", stage: "registration", occurredAt: 1_800_000_000_001 },
  } };
  stubDaemonAPI(status);
  const first = render(settings(locale));
  const connection = await screen.findByRole("region", { name: connectionTitle });
  expect(screen.getByRole("region", { name: cliTitle })).toBeInTheDocument();
  expect(within(connection).getAllByRole("listitem").some(item => item.textContent?.includes(timeStep))).toBe(true);
  expect(await screen.findByText(stateLabel)).toBeInTheDocument();
  const listener = vi.mocked(window.daemonAPI.onStatusChange).mock.calls[0]![0];
  act(() => listener(status));
  expect(screen.getByRole("region", { name: connectionTitle })).toBeInTheDocument();
  first.unmount();
  render(settings(locale));
  expect(await screen.findByRole("region", { name: connectionTitle })).toBeInTheDocument();
  expect(screen.getByRole("region", { name: cliTitle })).toBeInTheDocument();
});


it.each([["en", "Installation path"], ["zh-Hans", "安装路径"]] as const)("shows the exact selected Windows CLI path in %s", async (locale, label) => {
  const cliPath = String.raw`C:\Users\张 三\AppData\Local\Programs\Multica Desktop\resources\bin\multica.exe`;
  stubDaemonAPI({ state: "stopped" }, cliPath);
  render(settings(locale));
  const path = await screen.findByText(cliPath, { exact: true });
  expect(path.textContent).toBe(cliPath);
  expect(screen.getByText(label)).toBeInTheDocument();
  expect(window.daemonAPI.getCliPath).toHaveBeenCalledOnce();
});

it("keeps the installation guide when no usable CLI path is found", async () => {
  stubDaemonAPI({ state: "cli_not_found" }, null);
  render(settings());
  expect(await screen.findByText("No usable multica CLI was found. Check the runtime setup diagnostics below.")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Installation Guide" })).toBeInTheDocument();
  expect(screen.queryByText("Installation path")).not.toBeInTheDocument();
});

it("reports a failed CLI path lookup without claiming the CLI is missing or exposing its exception", async () => {
  stubDaemonAPI({ state: "stopped" });
  vi.mocked(window.daemonAPI.getCliPath).mockRejectedValue(new Error("private lookup detail"));
  render(settings());
  expect(await screen.findByText("Unable to read CLI installation details. Reopen Settings to try again.")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Installation Guide" })).not.toBeInTheDocument();
  expect(screen.queryByText("Installation path")).not.toBeInTheDocument();
  expect(screen.queryByText("Checking...")).not.toBeInTheDocument();
  expect(document.body.textContent).not.toContain("private lookup detail");
});
