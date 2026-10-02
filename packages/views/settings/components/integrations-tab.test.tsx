import { useState, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiError } from "@multica/core/api";
import { configStore } from "@multica/core/config";
import { COMPOSIO_MCP_APPS_FLAG } from "@multica/core/feature-flags";
import { NavigationProvider } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";

const state = vi.hoisted(() => ({
  calls: [] as { queryKey: readonly unknown[]; enabled?: boolean }[],
  github: { data: { installations: [] as { account_login: string }[], configured: true }, isPending: false, isError: false },
  members: { data: [{ user_id: "user-1" }], isPending: false, isError: false },
  vcs: { data: { connections: [], configured: true }, isPending: false, isError: false },
  composioError: null as Error | null,
  workspace: { settings: {} as Record<string, unknown> },
}));
vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...await importOriginal<typeof import("@tanstack/react-query")>(),
  useQuery: (opts: { queryKey: readonly unknown[]; enabled?: boolean }) => {
    state.calls.push(opts);
    if (opts.queryKey.includes("github")) return state.github;
    if (opts.queryKey.includes("members")) return state.members;
    if (opts.queryKey.includes("vcs")) return state.vcs;
    return { error: opts.enabled === false ? null : state.composioError };
  },
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/paths", () => ({ useCurrentWorkspace: () => state.workspace }));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector: (s: { user: { id: string } }) => unknown) => selector({ user: { id: "user-1" } }),
    { getState: () => ({ user: { id: "user-1" } }) },
  ),
}));
vi.mock("./github-tab", () => ({ GitHubTab: () => <div data-testid="github-tab" /> }));
vi.mock("./lark-tab", () => ({ LarkTab: () => <div data-testid="lark-tab" /> }));
vi.mock("./composio-tab", () => ({ ComposioTab: () => <div data-testid="composio-tab" /> }));
vi.mock("./slack-tab", () => ({ SlackTab: () => <div data-testid="slack-tab" /> }));
vi.mock("./dingtalk-tab", () => ({ DingTalkTab: () => <div data-testid="dingtalk-tab" /> }));
vi.mock("./vcs-tab", () => ({ VCSTab: () => <div data-testid="vcs-tab" /> }));
vi.mock("./wecom-tab", () => ({ WecomTab: () => <div data-testid="wecom-tab" /> }));
vi.mock("./telegram-tab", () => ({ TelegramTab: () => <div data-testid="telegram-tab" /> }));

import { IntegrationsTab } from "./integrations-tab";

const push = vi.fn();
const replace = vi.fn();
function renderTab(search = "tab=integrations", locale: "en" | "zh-Hans" = "en") {
  function Harness({ children }: { children: ReactNode }) {
    const [path, setPath] = useState(`/acme/settings?${search}`);
    push.mockImplementation(setPath);
    replace.mockImplementation(setPath);
    const url = new URL(path, "https://app.example");
    return <NavigationProvider value={{
      pathname: url.pathname, searchParams: url.searchParams, hash: url.hash,
      push, replace, back: vi.fn(), getShareableUrl: (p) => `https://app.example${p}`,
    }}>{children}</NavigationProvider>;
  }
  return renderWithI18n(<Harness><IntegrationsTab /></Harness>, { locale });
}

beforeEach(() => {
  vi.clearAllMocks();
  state.calls = [];
  state.github = { data: { installations: [], configured: true }, isPending: false, isError: false };
  state.members = { data: [{ user_id: "user-1" }], isPending: false, isError: false };
  state.composioError = null;
  state.workspace.settings = {};
  configStore.getState().setFeatureFlags({ [COMPOSIO_MCP_APPS_FLAG]: false });
  configStore.getState().setAuthConfig({ allowSignup: true, vcsIntegrationAvailable: true, messagingIntegrationsEnabled: false });
});

describe("Settings integration catalog", () => {
  it("opens on the ordered catalog with inert planned items and no provider forms", () => {
    renderTab();
    expect(screen.getAllByRole("heading", { level: 3 }).map((h) => h.textContent))
      .toEqual(["Code hosting", "Task management", "Communication & collaboration"]);
    expect(screen.getByRole("link", { name: "GitHub" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Self-hosted Git" })).toBeInTheDocument();
    for (const name of ["ONES", "Plane", "Kaneo", "Fuxin"]) {
      expect(screen.getByText(name)).toBeInTheDocument();
      expect(screen.queryByRole("link", { name })).toBeNull();
      expect(screen.queryByRole("button", { name })).toBeNull();
    }
    expect(screen.getAllByText("Planned")).toHaveLength(4);
    expect(screen.queryByTestId("github-tab")).toBeNull();
    expect(screen.queryByTestId("vcs-tab")).toBeNull();
    expect(screen.queryByText("Apps & tools")).toBeNull();
  });

  it("opens a detail by keyboard and restores the card focus on returning", async () => {
    const user = userEvent.setup();
    renderTab("tab=integrations&keep=yes#section");
    const github = screen.getByRole("link", { name: "GitHub" });
    act(() => github.focus());
    await user.keyboard("{Enter}");
    expect(push).toHaveBeenLastCalledWith("/acme/settings?tab=integrations&keep=yes&integration=github#section");
    expect(screen.getByTestId("github-tab")).toBeInTheDocument();
    expect(screen.queryByText("ONES")).toBeNull();
    const back = screen.getByRole("link", { name: "All integrations" });
    expect(back).toHaveFocus();
    await user.click(back);
    expect(screen.queryByTestId("github-tab")).toBeNull();
    expect(screen.getByRole("link", { name: "GitHub" })).toHaveFocus();
  });

  // URL parsing/serialization matrices are canonical in settings-integration-navigation.test.ts.
  it("mounts GitHub detail for the backend install callback", () => {
    renderTab("tab=github");
    expect(screen.getByTestId("github-tab")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "GitHub" })).toBeInTheDocument();
  });

  it.each(["unknown", "ones", "plane", "kaneo", "fuxin", "vcs", "lark", "slack", "dingtalk", "wecom", "telegram"])(
    "rejects unavailable detail %s without mounting it", async (id) => {
      configStore.getState().setAuthConfig({ allowSignup: true, vcsIntegrationAvailable: false, messagingIntegrationsEnabled: false });
      renderTab(`tab=integrations&integration=${id}`);
      expect(replace).not.toHaveBeenCalled();
      expect(screen.queryByTestId(`${id}-tab`)).toBeNull();
      expect(screen.getByRole("status")).toHaveTextContent("This integration is unavailable");
      expect(screen.getByText("ONES")).toBeInTheDocument();
    },
  );

  it("keeps a gated deep link until slow deployment configuration enables it", () => {
    renderTab("tab=lark");
    expect(screen.queryByTestId("lark-tab")).toBeNull();
    expect(replace).not.toHaveBeenCalled();
    act(() => configStore.getState().setAuthConfig({ allowSignup: true, messagingIntegrationsEnabled: true }));
    expect(screen.getByTestId("lark-tab")).toBeInTheDocument();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("keeps deployment-enabled channels accessible with distinct brand marks", async () => {
    configStore.getState().setAuthConfig({ allowSignup: true, vcsIntegrationAvailable: false, messagingIntegrationsEnabled: true });
    renderTab();
    expect(screen.queryByRole("link", { name: "Self-hosted Git" })).toBeNull();
    const shapes = ["lark", "slack", "dingtalk", "wecom", "telegram"].map((channel) =>
      screen.getByTestId(`integration-channel-icon-${channel}`).innerHTML);
    expect(new Set(shapes).size).toBe(shapes.length);
    await userEvent.click(screen.getByRole("link", { name: "Slack" }));
    expect(screen.getByTestId("slack-tab")).toBeInTheDocument();
    expect(screen.queryByTestId("github-tab")).toBeNull();
  });

  it("preserves the deployment-gated Composio surface and its callback parameters", () => {
    configStore.getState().setFeatureFlags({ [COMPOSIO_MCP_APPS_FLAG]: true });
    renderTab("tab=integrations&connected=notion");
    expect(screen.getByTestId("composio-tab")).toBeInTheDocument();
    expect(replace).not.toHaveBeenCalled();
  });

  it("does not query disabled Composio or expose its form", () => {
    renderTab();
    expect(state.calls.find((opts) => opts.queryKey.includes("composio"))?.enabled).toBe(false);
    expect(screen.queryByTestId("composio-tab")).toBeNull();
  });

  it("hides unconfigured Composio without an empty category", () => {
    configStore.getState().setFeatureFlags({ [COMPOSIO_MCP_APPS_FLAG]: true });
    state.composioError = new ApiError("unavailable", 503, "Service Unavailable");
    renderTab();
    expect(screen.queryByTestId("composio-tab")).toBeNull();
    expect(screen.queryByRole("heading", { name: "Composio" })).toBeNull();
  });

  it("keeps GitHub connection and master-off states independent", () => {
    state.github.data.installations = [{ account_login: "acme" }];
    state.workspace.settings.github_enabled = false;
    renderTab();
    const github = screen.getByRole("link", { name: "GitHub" });
    expect(within(github).getByText("Connected")).toBeInTheDocument();
    expect(within(github).getByText("Features off")).toBeInTheDocument();
    expect(github).toHaveAccessibleDescription(expect.stringContaining("Connected Features off"));
  });

  it.each(["installations", "membership"])("does not report disconnection on a failed %s query", (source) => {
    if (source === "installations") state.github.isError = true;
    else state.members.isError = true;
    renderTab();
    const github = screen.getByRole("link", { name: "GitHub" });
    expect(within(github).getByText("Status unavailable")).toBeInTheDocument();
    expect(within(github).queryByText("Not connected")).toBeNull();
  });

  it("shows loading until membership is known and keeps installation queries gated", () => {
    state.members = { data: [], isPending: true, isError: false };
    renderTab();
    expect(within(screen.getByRole("link", { name: "GitHub" })).getByText("Loading...")).toBeInTheDocument();
    expect(state.calls.find((opts) => opts.queryKey.includes("github"))?.enabled).toBe(false);
  });

  it("renders the approved Chinese labels", () => {
    renderTab("tab=integrations", "zh-Hans");
    expect(screen.getByRole("heading", { name: "代码托管" })).toBeInTheDocument();
    expect(screen.getByText("孚信")).toBeInTheDocument();
    expect(screen.getAllByText("待规划")).toHaveLength(4);
  });
});
