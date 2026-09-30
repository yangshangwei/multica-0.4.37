import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { SupportedLocale } from "@multica/core/i18n";
import { RESOURCES } from "@multica/views/locales";
import { toast } from "sonner";
import { useQuery } from "@tanstack/react-query";

import type { DaemonStatus } from "../../../shared/daemon-types";

// The component only needs these to render; stub them so the test focuses on
// the externally-managed branching, not data fetching.
vi.mock("@tanstack/react-query", () => ({
  useQuery: vi.fn(() => ({ data: [] })),
}));
vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));
vi.mock("@multica/core/runtimes", () => ({
  runtimeListOptions: () => ({ queryKey: ["runtimes"] }),
}));
vi.mock("@multica/core/agents", () => ({
  agentTaskSnapshotOptions: () => ({ queryKey: ["snapshot"] }),
}));
vi.mock("./daemon-panel", () => ({ DaemonPanel: () => null }));
vi.mock("../platform/daemon-reauth", () => ({
  reauthenticateDaemon: vi.fn(),
}));
vi.mock("sonner", () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}));

import { DaemonRuntimeActions } from "./daemon-runtime-card";

function stubDaemonAPI(status: DaemonStatus) {
  Object.defineProperty(window, "daemonAPI", {
    configurable: true,
    value: {
      getStatus: vi.fn().mockResolvedValue(status),
      onStatusChange: vi.fn(() => () => {}),
      restart: vi.fn().mockResolvedValue({ success: true }),
      start: vi.fn().mockResolvedValue({ success: true }),
      stop: vi.fn().mockResolvedValue({ success: true }),
    },
  });
}

function actions(locale: SupportedLocale = "en") {
  return (
    <I18nProvider locale={locale} resources={RESOURCES}>
      <DaemonRuntimeActions />
    </I18nProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useQuery).mockReset().mockReturnValue({ data: [] } as ReturnType<typeof useQuery>);
});

describe("DaemonRuntimeActions — externally managed daemon (#3916)", () => {
  it("hides Stop/Restart and shows the managed-outside hint for a daemon the app can't control", async () => {
    stubDaemonAPI({ state: "running", daemonId: "d1", externallyManaged: true });
    render(actions());

    // View logs still renders, confirming the running branch mounted.
    expect(await screen.findByText("View logs")).toBeInTheDocument();
    expect(screen.getByText("Managed outside the app")).toBeInTheDocument();
    expect(screen.queryByText("Restart")).not.toBeInTheDocument();
    expect(screen.queryByText("Stop")).not.toBeInTheDocument();
  });

  it("shows Stop/Restart for a normally-managed running daemon (no 误伤)", async () => {
    stubDaemonAPI({
      state: "running",
      daemonId: "d1",
      externallyManaged: false,
    });
    render(actions());

    expect(await screen.findByText("Restart")).toBeInTheDocument();
    expect(screen.getByText("Stop")).toBeInTheDocument();
    expect(
      screen.queryByText("Managed outside the app"),
    ).not.toBeInTheDocument();
  });
});

describe("DaemonRuntimeActions — recovery budget", () => {
  it("offers a manual Start when automatic recovery is paused", async () => {
    stubDaemonAPI({ state: "recovery_paused" });
    render(actions());

    expect(await screen.findByText("Start")).toBeInTheDocument();
  });
});

describe("DaemonRuntimeActions — language selection", () => {
  it("switches running controls and restart feedback with the app language", async () => {
    stubDaemonAPI({ state: "running", daemonId: "d1" });
    const { rerender } = render(actions());
    expect(await screen.findByRole("button", { name: "View logs" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Stop" })).toBeInTheDocument();

    rerender(actions("zh-Hans"));
    expect(await screen.findByRole("button", { name: "查看日志" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "停止" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "重启" }));
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith("正在重启守护进程", {
      description: "运行时将在几秒后重新上线。",
    }));
    expect(window.daemonAPI.restart).toHaveBeenCalledOnce();

    rerender(actions());
    expect(await screen.findByRole("button", { name: "Restart" })).toBeInTheDocument();
  });

  it.each([
    ["stopped", "启动"],
    ["recovery_paused", "启动"],
    ["cli_not_found", "重试安装"],
    ["auth_expired", "重新登录"],
    ["starting", "启动中..."],
    ["stopping", "停止中..."],
    ["installing_cli", "正在安装 CLI..."],
  ] as const)("localizes the %s controls", async (state, label) => {
    stubDaemonAPI({ state });
    render(actions("zh-Hans"));
    expect(await screen.findByRole("button", { name: label })).toBeInTheDocument();
  });

  it("localizes the externally managed hint while keeping lifecycle controls hidden", async () => {
    stubDaemonAPI({ state: "running", externallyManaged: true });
    render(actions("zh-Hans"));
    expect(await screen.findByText("由应用外部管理")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重启" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "停止" })).not.toBeInTheDocument();
  });

  it.each([
    ["en", 1, "Stop", "Stop daemon with 1 active task?", "Stop daemon"],
    ["en", 2, "Stop", "Stop daemon with 2 active tasks?", "Stop daemon"],
    ["zh-Hans", 2, "停止", "仍有 2 个 task 运行，停止守护进程？", "停止守护进程"],
  ] as const)("localizes the stop confirmation in %s for %i active tasks", async (locale, count, stop, title, confirm) => {
    stubDaemonAPI({ state: "running", daemonId: "d1" });
    vi.mocked(useQuery).mockImplementation((options) => ({
      data: options.queryKey[0] === "runtimes"
        ? [{ id: "r1", daemon_id: "d1" }]
        : Array.from({ length: count }, (_, i) => ({ id: `task-${i}`, runtime_id: "r1", status: "running" })),
    }) as ReturnType<typeof useQuery>);
    render(actions(locale));
    fireEvent.click(await screen.findByRole("button", { name: stop }));
    expect(await screen.findByRole("heading", { name: title })).toBeInTheDocument();
    expect(window.daemonAPI.stop).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: confirm }));
    await waitFor(() => expect(window.daemonAPI.stop).toHaveBeenCalledOnce());
  });
});
