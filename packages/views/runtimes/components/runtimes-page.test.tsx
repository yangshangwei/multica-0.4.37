import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AgentRuntime, RuntimeProfile } from "@multica/core/types";
import { runtimeKeys } from "@multica/core/runtimes/queries";
import { runtimeProfileKeys } from "@multica/core/runtimes";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { RuntimesPage, type RuntimesPageProps } from "./runtimes-page";

const mocks = vi.hoisted(() => ({
  listRuntimes: vi.fn(), listRuntimeProfiles: vi.fn(),
  events: new Map<string, () => void>(),
  auth: { isLoading: false, user: { id: "user-1" } },
}));

vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: {
    listRuntimes: mocks.listRuntimes,
    listRuntimeProfiles: mocks.listRuntimeProfiles,
    listAgents: async () => [],
    listChatSessions: async () => [],
    getAgentTaskSnapshot: async () => [],
  },
}));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector: (state: typeof mocks.auth) => unknown) => selector(mocks.auth),
    { getState: () => mocks.auth },
  ),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return {
    ...actual,
    useWorkspaceSlug: () => "welcome-workspace",
    useRequiredWorkspaceSlug: () => "welcome-workspace",
    useWorkspacePaths: () => actual.paths.workspace("welcome-workspace"),
  };
});
vi.mock("@multica/core/realtime", () => ({
  useWSEvent: (event: string, handler: () => void) => { mocks.events.set(event, handler); },
}));
vi.mock("@multica/core/onboarding", () => ({
  MIKA_AVATAR_PATH: "/mika.png", memberNeedsMikaSetup: () => false,
  useBootstrapMika: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("./connect-remote-dialog", () => ({
  ConnectRemoteDialog: ({ onClose }: { onClose: () => void }) => <div role="dialog" aria-label="Add a computer"><button onClick={onClose}>Close</button></div>,
}));
vi.mock("./cloud-runtime-dialog", () => ({ CloudRuntimeDialog: () => null }));
vi.mock("./runtime-list", () => ({
  buildWorkloadIndex: () => new Map(),
  RuntimeList: ({ runtimes }: { runtimes: AgentRuntime[] }) => <ul>{runtimes.map((runtime) => <li key={runtime.id}>{runtime.custom_name ?? runtime.name}</li>)}</ul>,
}));

const heading = "Give your agents a computer to work on";
const runtime: AgentRuntime = {
  id: "runtime-1", workspace_id: "ws-1", daemon_id: "daemon-1",
  name: "Codex (studio-mbp)", runtime_mode: "local", provider: "codex",
  launch_header: "codex", status: "offline", device_info: "studio-mbp",
  metadata: {}, owner_id: "user-1", visibility: "private", profile_id: null,
  last_seen_at: "2026-01-01T00:00:00Z", created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z",
};
const profile: RuntimeProfile = {
  id: "profile-1", workspace_id: "ws-1", display_name: "Private custom runtime",
  protocol_family: "codex", command_name: "custom-codex", description: null,
  fixed_args: [], visibility: "private", created_by: "user-1", enabled: true,
  created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z",
};

function renderPage(props: RuntimesPageProps = {}, cachedRuntimes?: AgentRuntime[]) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  if (cachedRuntimes) client.setQueryData(runtimeKeys.list("ws-1"), cachedRuntimes);
  const adapter: NavigationAdapter = {
    push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/welcome-workspace/runtimes",
    searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path,
  };
  const view = renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={adapter}><RuntimesPage {...props} /></NavigationProvider></QueryClientProvider>);
  return { ...view, client, adapter };
}

beforeEach(() => {
  mocks.listRuntimes.mockReset().mockResolvedValue([]);
  mocks.listRuntimeProfiles.mockReset().mockResolvedValue([]);
  mocks.events.clear();
  mocks.auth.isLoading = false;
});

describe("RuntimesPage welcome gate", () => {
  it("shows the welcome after both inventories confirm empty and wires the connection action", async () => {
    renderPage();
    expect(await screen.findByRole("heading", { name: heading })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Add runtime" }));
    expect(screen.getByRole("dialog", { name: "Add a computer" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("does not interpret a loading inventory as a new workspace", async () => {
    let resolveRuntimes!: (value: AgentRuntime[]) => void;
    mocks.listRuntimes.mockReturnValue(new Promise<AgentRuntime[]>((resolve) => { resolveRuntimes = resolve; }));
    renderPage();
    expect(screen.queryByRole("heading", { name: heading })).not.toBeInTheDocument();
    await act(async () => resolveRuntimes([]));
    expect(await screen.findByRole("heading", { name: heading })).toBeInTheDocument();
  });

  it.each(["runtimes", "profiles"])("shows recovery for failed %s rather than welcome, then retries both inventories", async (failing) => {
    const fail = failing === "runtimes" ? mocks.listRuntimes : mocks.listRuntimeProfiles;
    fail.mockRejectedValueOnce(new Error("Inventory unavailable"));
    renderPage();
    expect(await screen.findByRole("alert")).toHaveTextContent("Couldn't load runtimes");
    expect(screen.queryByRole("heading", { name: heading })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("heading", { name: heading })).toBeInTheDocument();
    expect(mocks.listRuntimes.mock.calls.length).toBeGreaterThanOrEqual(2);
    expect(mocks.listRuntimeProfiles.mock.calls.length).toBeGreaterThanOrEqual(2);
  });

  it("replaces welcome with the existing offline machine after registration invalidation", async () => {
    const { client } = renderPage();
    await screen.findByRole("heading", { name: heading });
    mocks.listRuntimes.mockResolvedValue([runtime]);
    await act(async () => { mocks.events.get("daemon:register")?.(); });
    expect(await screen.findByRole("link", { name: /studio-mbp/ })).toHaveAttribute("href", "/welcome-workspace/runtimes/local%3Adaemon-1");
    expect(screen.queryByRole("heading", { name: heading })).not.toBeInTheDocument();
    expect(client.getQueryData(runtimeKeys.list("ws-1"))).toEqual([runtime]);
  });

  it("keeps cached nonempty inventory usable after a refresh fails", async () => {
    mocks.listRuntimes.mockRejectedValue(new Error("Background failure"));
    renderPage({}, [runtime]);
    expect(await screen.findByRole("link", { name: /studio-mbp/ })).toBeInTheDocument();
    await waitFor(() => expect(mocks.listRuntimes).toHaveBeenCalled());
    expect(screen.queryByRole("heading", { name: heading })).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("does not retain welcome after an empty inventory refresh fails", async () => {
    const { client } = renderPage();
    await screen.findByRole("heading", { name: heading });
    mocks.listRuntimeProfiles.mockRejectedValue(new Error("Profile refresh unavailable"));
    await act(async () => { await client.invalidateQueries({ queryKey: runtimeProfileKeys.list("ws-1") }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Couldn't load runtimes");
    expect(screen.queryByRole("heading", { name: heading })).not.toBeInTheDocument();
  });

  it("keeps the desktop local placeholder instead of onboarding", async () => {
    renderPage({ hasLocalMachine: true, localMachineName: "This desktop", localDaemonId: "local-daemon" });
    expect(await screen.findByRole("link", { name: /This desktop/ })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: heading })).not.toBeInTheDocument();
  });

  it("retains the daemon starting state", async () => {
    renderPage({ bootstrapping: true });
    expect(await screen.findByText("Starting local runtime…")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: heading })).not.toBeInTheDocument();
  });

  it("shows pending custom profiles instead of onboarding", async () => {
    mocks.listRuntimeProfiles.mockResolvedValue([profile]);
    renderPage();
    expect(await screen.findByText(/Private custom runtime/)).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: heading })).not.toBeInTheDocument();
  });
});
