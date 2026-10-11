import { useState } from "react";
import { onlineManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import { configStore, type CliInstallCommands } from "@multica/core/config";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { runtimeKeys } from "@multica/core/runtimes/queries";
import type { AgentRuntime } from "@multica/core/types";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import enCommon from "../../locales/en/common.json";
import enRuntimes from "../../locales/en/runtimes.json";
import { ConnectRemoteDialog } from "./connect-remote-dialog";

// Shell/URL and identity matrices live in connect-remote-{commands,machine}.test.ts.
const TEST_RESOURCES = { en: { common: enCommon, runtimes: enRuntimes } };
const apiState = vi.hoisted(() => ({ listRuntimes: vi.fn<(...args: unknown[]) => Promise<AgentRuntime[]>>() }));
const authState = vi.hoisted(() => ({ user: { id: "user-test" } }));
const wsState = vi.hoisted(() => ({
  handlers: new Map<string, (payload: unknown) => void>(),
  reconnect: null as (() => void) | null,
}));
const clipboardState = vi.hoisted(() => ({ copyText: vi.fn<(text: string) => Promise<boolean>>() }));

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-test" }));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector?: (state: typeof authState) => unknown) => selector ? selector(authState) : authState,
    { getState: () => authState },
  ),
}));
vi.mock("@multica/core/api", () => ({ api: { listRuntimes: apiState.listRuntimes } }));
vi.mock("@multica/core/realtime", () => ({
  useWSEvent: (event: string, handler: (payload: unknown) => void) => { wsState.handlers.set(event, handler); },
  useWSReconnect: (handler: () => void) => { wsState.reconnect = handler; },
}));
vi.mock("@multica/ui/lib/clipboard", () => ({ copyText: clipboardState.copyText }));

function runtime(overrides: Partial<AgentRuntime> = {}): AgentRuntime {
  return {
    id: "rt-new", workspace_id: "ws-test", daemon_id: "daemon-new", owner_id: "user-test",
    name: "Codex (Office-PC)", provider: "codex", runtime_mode: "local", status: "online",
    launch_header: "", device_info: "Office-PC · windows-amd64", metadata: {}, visibility: "private",
    last_seen_at: "2026-10-11T12:00:00Z", created_at: "2026-10-11T12:00:00Z", updated_at: "2026-10-11T12:00:00Z",
    ...overrides,
  };
}

const clients = new Set<QueryClient>();
function renderDialog({ config, client, withLauncher = false }: {
  config?: { daemonServerUrl?: string; daemonAppUrl?: string; cliInstallCommand?: string; cliInstallCommands?: CliInstallCommands };
  client?: QueryClient;
  withLauncher?: boolean;
} = {}) {
  configStore.getState().setDaemonConfig(config ?? {});
  const qc = client ?? new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.add(qc);
  const onClose = vi.fn();
  const navigation: NavigationAdapter = {
    push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/workspace-test/runtimes",
    searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path,
  };
  function Launcher() {
    const [open, setOpen] = useState(false);
    return <>
      <button onClick={() => setOpen(true)}>Add another computer</button>
      {open && <ConnectRemoteDialog onClose={() => { onClose(); setOpen(false); }} />}
    </>;
  }
  const view = render(
    <QueryClientProvider client={qc}>
      <I18nProvider locale="en" resources={TEST_RESOURCES}>
        <WorkspaceSlugProvider slug="workspace-test">
          <NavigationProvider value={navigation}>
            {withLauncher ? <Launcher /> : <ConnectRemoteDialog onClose={onClose} />}
          </NavigationProvider>
        </WorkspaceSlugProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
  return { ...view, qc, navigation, onClose };
}
async function choosePlatform(name: "Windows" | "macOS" | "Linux" = "macOS") {
  await userEvent.click(screen.getByRole("tab", { name }));
}
async function waitForBaseline() {
  await screen.findByText("We'll update this view when a new computer connects.");
}
function emitRegistration(payload: unknown = { runtime_id: "rt-new" }) {
  act(() => { wsState.handlers.get("daemon:register")?.(payload); });
}

describe("ConnectRemoteDialog", () => {
  beforeEach(() => {
    wsState.handlers.clear();
    wsState.reconnect = null;
    apiState.listRuntimes.mockReset().mockResolvedValue([]);
    clipboardState.copyText.mockReset().mockResolvedValue(true);
    onlineManager.setOnline(true);
  });
  afterEach(() => {
    cleanup();
    clients.forEach((client) => client.clear());
    clients.clear();
    onlineManager.setOnline(true);
    vi.useRealTimers();
  });

  it("starts with no selected system or commands and supports keyboard selection", async () => {
    const { baseElement } = renderDialog();
    expect(screen.getAllByRole("tab")).toHaveLength(3);
    screen.getAllByRole("tab").forEach((tab) => expect(tab).toHaveAttribute("aria-selected", "false"));
    expect(baseElement).not.toHaveTextContent("multica setup");
    expect(baseElement).not.toHaveTextContent("install.sh");
    await choosePlatform("Windows");
    await userEvent.keyboard("{ArrowRight}{Enter}");
    expect(screen.getByRole("tab", { name: "macOS" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "macOS" })).toHaveFocus();
    expect(screen.getByRole("tabpanel", { name: "macOS" })).toBeInTheDocument();
  });

  it("wires installation overrides and shell hints for the selected system", async () => {
    const { baseElement } = renderDialog({ config: {
      cliInstallCommands: { windows: "internal-powershell-installer" }, cliInstallCommand: "internal-unix-installer",
    } });
    await choosePlatform("Windows");
    expect(baseElement).toHaveTextContent("internal-powershell-installer");
    expect(baseElement).toHaveTextContent("PowerShell");
    expect(baseElement).toHaveTextContent("WSL");
    await choosePlatform("Linux");
    expect(baseElement).toHaveTextContent("internal-unix-installer");
    expect(baseElement).toHaveTextContent("SSH");
    expect(baseElement).not.toHaveTextContent("internal-powershell-installer");
  });

  it("offers a local guide rather than a public installer when the selected internal source is missing", async () => {
    const { baseElement } = renderDialog({ config: { cliInstallCommand: "internal-unix-installer" } });
    await choosePlatform("Windows");
    expect(screen.getByText("Installation for this system hasn't been configured")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View CLI installation guide" })).toHaveAttribute("href", "/workspace-test/docs/cli");
    expect(screen.queryByRole("button", { name: "Copy Windows installation command" })).not.toBeInTheDocument();
    expect(baseElement).not.toHaveTextContent("raw.githubusercontent.com");
  });

  it("keeps installation but blocks setup and token commands for an inaccessible address", async () => {
    const { baseElement } = renderDialog({ config: { daemonServerUrl: "http://localhost:18572", daemonAppUrl: "http://localhost:13492" } });
    await choosePlatform("Windows");
    await userEvent.click(screen.getByText("No browser, or still not connected?"));
    expect(baseElement).toHaveTextContent("install.ps1");
    expect(baseElement).toHaveTextContent("http://localhost:18572");
    expect(baseElement).toHaveTextContent("http://localhost:13492");
    expect(screen.getByRole("link", { name: "View address configuration guide" })).toHaveAttribute("href", "/workspace-test/docs/self-host-quickstart");
    expect(baseElement).not.toHaveTextContent("multica setup");
    expect(baseElement).not.toHaveTextContent("multica config set");
    expect(baseElement).not.toHaveTextContent("multica login --token");
    expect(screen.getByText("Configure reachable service addresses to get the connection command.")).toBeInTheDocument();
  });

  it("copies a full single-line self-host command, preserves code selection and keeps focus", async () => {
    const { baseElement } = renderDialog({ config: { daemonServerUrl: "https://api.example.com/", daemonAppUrl: "https://app.example.com/a'b?x=1&y=2" } });
    await choosePlatform("Windows");
    const expected = "multica setup self-host --server-url 'https://api.example.com' --app-url 'https://app.example.com/a''b?x=1&y=2'";
    const code = Array.from(baseElement.querySelectorAll("code")).find((node) => node.textContent === expected);
    expect(code).toHaveClass("[font-variant-ligatures:none]", "[font-feature-settings:'liga'_0]", "select-text");
    const button = screen.getByRole("button", { name: "Copy connection command" });
    await userEvent.click(button);
    expect(clipboardState.copyText).toHaveBeenCalledWith(expected);
    expect(button).toHaveTextContent("Copied");
    expect(button).toHaveFocus();
  });

  it("reports copy failure and clears feedback on system changes while keeping help expanded", async () => {
    clipboardState.copyText.mockResolvedValueOnce(false);
    const { baseElement } = renderDialog();
    await choosePlatform("Windows");
    await userEvent.click(screen.getByText("No browser, or still not connected?"));
    await userEvent.click(screen.getByRole("button", { name: "Copy Windows installation command" }));
    expect(screen.getByText("Couldn't copy. Select the command and copy it manually.")).toBeVisible();
    await choosePlatform("Linux");
    expect(screen.queryByText("Couldn't copy. Select the command and copy it manually.")).not.toBeInTheDocument();
    expect(baseElement.querySelector("details")).toHaveAttribute("open");
    await userEvent.click(screen.getByRole("button", { name: "Copy Linux installation command" }));
    expect(screen.getByRole("button", { name: "Copy Linux installation command" })).toHaveTextContent("Copied");
    await choosePlatform("macOS");
    expect(screen.getByRole("button", { name: "Copy macOS installation command" })).not.toHaveTextContent("Copied");
  });

  it("clears successful copy feedback after 1.5 seconds", async () => {
    renderDialog();
    await choosePlatform();
    await waitForBaseline();
    vi.useFakeTimers();
    const button = screen.getByRole("button", { name: "Copy connection command" });
    await act(async () => { fireEvent.click(button); });
    expect(button).toHaveTextContent("Copied");
    await act(async () => { await vi.advanceTimersByTimeAsync(1500); });
    expect(button).not.toHaveTextContent("Copied");
  });

  it("copies configuration, interactive login and daemon start separately", async () => {
    renderDialog();
    await choosePlatform();
    await userEvent.click(screen.getByText("No browser, or still not connected?"));
    expect(screen.getByRole("link", { name: "Settings → API tokens" })).toHaveAttribute("href", "/workspace-test/settings?tab=tokens");
    for (const name of ["Copy service configuration commands", "Copy token login command", "Copy daemon start command"]) {
      await userEvent.click(screen.getByRole("button", { name }));
    }
    expect(clipboardState.copyText.mock.calls.map(([text]) => text)).toEqual([
      "multica config set server_url 'https://api.multica.ai'\nmultica config set app_url 'https://multica.ai'",
      "multica login --token", "multica daemon start",
    ]);
    expect(screen.getByText("multica login --token")).toHaveClass("[font-variant-ligatures:none]");
    expect(screen.queryByText(/YOUR_TOKEN/)).not.toBeInTheDocument();
  });

  it("confirms query-derived computer details and opens the computer route", async () => {
    const { navigation, onClose } = renderDialog();
    await choosePlatform("macOS");
    await waitForBaseline();
    apiState.listRuntimes.mockResolvedValue([
      runtime(), runtime({ id: "rt-claude", provider: "claude", name: "Claude Code (Office-PC)" }),
    ]);
    emitRegistration({ runtime_id: "untrusted-id" });
    await screen.findByText("Computer connected");
    expect(screen.getByText("Office-PC")).toBeInTheDocument();
    expect(screen.getByText(/Windows/)).toBeInTheDocument();
    expect(screen.getByText("Claude Code")).toBeInTheDocument();
    expect(screen.getByText("Codex")).toBeInTheDocument();
    expect(apiState.listRuntimes).toHaveBeenCalledWith({ workspace_id: "ws-test", owner: "me" }, undefined);
    await userEvent.click(screen.getByRole("button", { name: "View computer" }));
    expect(onClose).toHaveBeenCalledOnce();
    expect(navigation.push).toHaveBeenCalledWith("/workspace-test/runtimes/local%3Adaemon-new");
  });

  it("takes the success action to agent creation", async () => {
    const { navigation } = renderDialog();
    await choosePlatform();
    await waitForBaseline();
    apiState.listRuntimes.mockResolvedValue([runtime()]);
    emitRegistration();
    await userEvent.click(await screen.findByRole("button", { name: "Create an agent" }));
    expect(navigation.push).toHaveBeenCalledWith("/workspace-test/agents/new");
  });

  it("never treats a registration payload as connection proof", async () => {
    renderDialog();
    await choosePlatform();
    await waitForBaseline();
    for (const payload of [null, { runtime_id: "rt-new" }, { status: "online", name: "Invented computer" }]) {
      emitRegistration(payload);
      expect(screen.queryByText("Computer connected")).not.toBeInTheDocument();
    }
    await waitFor(() => expect(apiState.listRuntimes.mock.calls.length).toBeGreaterThan(1));
    expect(screen.getByText("Waiting for your computer")).toBeInTheDocument();
  });

  it("locks a fresh baseline even when a stale empty list was cached", async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
    qc.setQueryData(runtimeKeys.listMine("ws-test"), []);
    apiState.listRuntimes.mockResolvedValue([runtime()]);
    renderDialog({ client: qc });
    await choosePlatform();
    await waitForBaseline();
    expect(apiState.listRuntimes).toHaveBeenCalledOnce();
    expect(screen.queryByText("Computer connected")).not.toBeInTheDocument();
    emitRegistration();
    await waitFor(() => expect(apiState.listRuntimes).toHaveBeenCalledTimes(2));
    expect(screen.queryByText("Computer connected")).not.toBeInTheDocument();
  });

  it("keeps its baseline across a failed refresh and confirms on retry", async () => {
    renderDialog();
    await choosePlatform();
    await waitForBaseline();
    apiState.listRuntimes.mockRejectedValueOnce(new Error("Invalid runtime inventory response"));
    emitRegistration();
    await screen.findByText("Connection status can't update right now");
    expect(screen.queryByText("Computer connected")).not.toBeInTheDocument();
    apiState.listRuntimes.mockResolvedValue([runtime()]);
    await userEvent.click(screen.getByRole("button", { name: "Retry connection check" }));
    await screen.findByText("Computer connected");
  });

  it("waits for its first reliable inventory after an initial failure", async () => {
    apiState.listRuntimes.mockRejectedValueOnce(new Error("Invalid runtime inventory response"));
    renderDialog();
    await choosePlatform();
    await screen.findByText("Connection status can't update right now");
    apiState.listRuntimes.mockResolvedValue([runtime()]);
    await userEvent.click(screen.getByRole("button", { name: "Retry connection check" }));
    await waitForBaseline();
    expect(screen.queryByText("Computer connected")).not.toBeInTheDocument();
  });

  it("reports page disconnection and refreshes on reconnect without claiming the computer is offline", async () => {
    renderDialog();
    await choosePlatform();
    await waitForBaseline();
    act(() => { onlineManager.setOnline(false); });
    expect(screen.getByText("Connection status can't update right now")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry connection check" })).toBeDisabled();
    act(() => { onlineManager.setOnline(true); wsState.reconnect?.(); });
    await waitFor(() => expect(apiState.listRuntimes).toHaveBeenCalledTimes(2));
    await waitForBaseline();
  });

  it("offers troubleshooting after a long wait without inferring installation failure", async () => {
    renderDialog();
    await waitFor(() => expect(apiState.listRuntimes).toHaveBeenCalled());
    vi.useFakeTimers();
    await act(async () => { fireEvent.click(screen.getByRole("tab", { name: "Linux" })); });
    expect(screen.getByText("Waiting for your computer")).toBeInTheDocument();
    await act(async () => { await vi.advanceTimersByTimeAsync(120000); });
    expect(screen.getByText("No new computer detected yet")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Check connection steps" }));
    expect(screen.getByText("No browser, or still not connected?").closest("details")).toHaveAttribute("open");
  });

  it("returns focus on Escape and resets selection on reopen", async () => {
    renderDialog({ withLauncher: true });
    const trigger = screen.getByRole("button", { name: "Add another computer" });
    await userEvent.click(trigger);
    await choosePlatform("Windows");
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(trigger).toHaveFocus();
    await userEvent.click(trigger);
    screen.getAllByRole("tab").forEach((tab) => expect(tab).toHaveAttribute("aria-selected", "false"));
    expect(screen.queryByRole("button", { name: "Copy connection command" })).not.toBeInTheDocument();
  });
});
