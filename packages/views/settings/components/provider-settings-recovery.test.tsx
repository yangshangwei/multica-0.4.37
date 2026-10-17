import type { ComponentType, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { ApiError } from "@multica/core/api";
import type { MemberWithUser } from "@multica/core/types";
import { memberListOptions, workspaceKeys, workspaceMcpServersOptions } from "@multica/core/workspace/queries";
import { githubInstallationsOptions } from "@multica/core/github";
import { vcsConnectionsOptions } from "@multica/core/vcs";
import { telegramInstallationsOptions } from "@multica/core/telegram";
import { dingtalkKeys } from "@multica/core/dingtalk";
import { composioKeys } from "@multica/core/composio";
import enCommon from "../../locales/en/common.json";
import enSettings from "../../locales/en/settings.json";
import enAgents from "../../locales/en/agents.json";

const apiMocks = vi.hoisted(() => ({
  listMembers: vi.fn(),
  listAgents: vi.fn(),
  listGitHubInstallations: vi.fn(),
  listVCSConnections: vi.fn(),
  listSlackInstallations: vi.fn(),
  listLarkInstallations: vi.fn(),
  listDingTalkInstallations: vi.fn(),
  listDingTalkGroups: vi.fn(),
  forgetDingTalkGroup: vi.fn(),
  listWecomInstallations: vi.fn(),
  listTelegramInstallations: vi.fn(),
  listComposioToolkits: vi.fn(),
  listComposioConnections: vi.fn(),
  listWorkspaceMcpServers: vi.fn(),
  listMcpServerTemplates: vi.fn(),
  connectVCS: vi.fn(),
  updateWorkspaceMcpServer: vi.fn(),
  getBaseUrl: () => "https://settings.test",
}));

vi.mock("@multica/core/api", async (importOriginal) => ({
  ...await importOriginal<typeof import("@multica/core/api")>(),
  api: apiMocks,
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/paths", async (importOriginal) => {
  const { paths } = await importOriginal<typeof import("@multica/core/paths")>();
  return {
    useCurrentWorkspace: () => ({ id: "workspace-1", name: "Acme", slug: "acme", settings: {} }),
    useWorkspacePaths: () => paths.workspace("acme"),
  };
});
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  return {
    useAuthStore: Object.assign(
      (selector: (value: typeof state) => unknown) => selector(state),
      { getState: () => state },
    ),
  };
});
vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({ getAgentName: (id: string) => `Agent ${id}` }),
}));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => null }));
vi.mock("../../navigation/context", () => ({
  useNavigation: () => ({
    pathname: "/acme/settings", searchParams: new URLSearchParams("tab=integrations"),
    hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(),
    getShareableUrl: (path: string) => path,
  }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

import { SlackTab } from "./slack-tab";
import { LarkTab } from "./lark-tab";
import { DingTalkTab } from "./dingtalk-tab";
import { WecomTab } from "./wecom-tab";
import { TelegramTab } from "./telegram-tab";
import { GitHubTab } from "./github-tab";
import { VCSTab } from "./vcs-tab";
import { ComposioTab } from "./composio-tab";
import { McpTab } from "./mcp-tab";

const members: MemberWithUser[] = [{
  id: "member-1", workspace_id: "workspace-1", user_id: "user-1", role: "owner",
  created_at: "2026-10-01T00:00:00Z", name: "Test owner", email: "owner@settings.test", avatar_url: null,
}];
const installedBot = {
  id: "installation-1", agent_id: "agent-1", status: "active",
  installed_at: "2026-10-01T00:00:00Z", bot_username: "team_bot",
};
const sharedServer = {
  id: "server-1", workspace_id: "workspace-1", name: "team-tools", transport: "stdio",
  created_at: "2026-10-01T00:00:00Z", updated_at: "2026-10-01T00:00:00Z",
};
const recentGroup = {
  conversation_id: "cid-recent", conversation_title: "Recent engineering",
  bots: [{ installation_id: installedBot.id, agent_id: installedBot.agent_id, bot_name: "Team bot", bot_identity_issue: "" }],
};
const inactiveGroup = { ...recentGroup, conversation_id: "cid-inactive", conversation_title: "Historical engineering" };
const recentGroups = {
  group_discovery_supported: true, groups: [recentGroup], inactive_group_counts: { [installedBot.id]: 1 },
};
const inactiveGroups = { group_discovery_supported: true, groups: [inactiveGroup] };
const clients: QueryClient[] = [];

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

function renderTab(Tab: ComponentType, knownMember = true) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity, staleTime: Infinity } },
  });
  clients.push(client);
  if (knownMember) client.setQueryData(memberListOptions("workspace-1").queryKey, members);
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>
        <I18nProvider locale="en" resources={{ en: { common: enCommon, settings: enSettings, agents: enAgents } }}>
          {children}
        </I18nProvider>
      </QueryClientProvider>
    );
  }
  return { client, ...render(<Tab />, { wrapper: Wrapper }) };
}

beforeEach(() => {
  vi.resetAllMocks();
  apiMocks.listMembers.mockResolvedValue(members);
  apiMocks.listAgents.mockResolvedValue([]);
  for (const read of [apiMocks.listSlackInstallations, apiMocks.listLarkInstallations,
    apiMocks.listDingTalkInstallations, apiMocks.listWecomInstallations, apiMocks.listTelegramInstallations]) {
    read.mockResolvedValue({ configured: true, install_supported: true, installations: [] });
  }
  apiMocks.listDingTalkGroups.mockResolvedValue({ group_discovery_supported: false, groups: [] });
  apiMocks.listGitHubInstallations.mockResolvedValue({ configured: true, can_manage: true, installations: [] });
  apiMocks.listVCSConnections.mockResolvedValue({ configured: true, can_manage: true, connections: [] });
  apiMocks.listWorkspaceMcpServers.mockResolvedValue([sharedServer]);
  apiMocks.listMcpServerTemplates.mockResolvedValue([]);
  apiMocks.listComposioToolkits.mockResolvedValue([{ slug: "notion", name: "Notion", connectable: true }]);
  apiMocks.listComposioConnections.mockResolvedValue([]);
});

afterEach(() => {
  cleanup();
  clients.splice(0).forEach((client) => client.clear());
});

describe("provider settings query wiring", () => {
  // The query-state matrix is canonical in settings-query-feedback.test.tsx.
  // These rows catch each provider's configuration branch running before a read succeeds.
  it.each([
    ["slack", SlackTab, apiMocks.listSlackInstallations],
    ["lark", LarkTab, apiMocks.listLarkInstallations],
    ["dingtalk", DingTalkTab, apiMocks.listDingTalkInstallations],
    ["wecom", WecomTab, apiMocks.listWecomInstallations],
    ["telegram", TelegramTab, apiMocks.listTelegramInstallations],
  ] as const)("keeps %s failures distinct from a genuinely unconfigured provider", async (provider, Tab, read) => {
    const user = userEvent.setup();
    read.mockRejectedValue(new Error("Read unavailable"));
    renderTab(Tab);

    const alert = await screen.findByRole("alert");
    expect(screen.queryByText(enSettings[provider].not_enabled_title)).not.toBeInTheDocument();
    expect(screen.queryByText(enSettings[provider].empty_title)).not.toBeInTheDocument();
    read.mockResolvedValue({ configured: false, installations: [] });
    await user.click(within(alert).getByRole("button", { name: "Retry" }));

    expect(await screen.findByText(enSettings[provider].not_enabled_title)).toBeVisible();
    expect(read).toHaveBeenCalledTimes(2);
  });

  it("recovers a failed permission read without claiming that a Slack connection is read-only", async () => {
    const user = userEvent.setup();
    apiMocks.listMembers.mockRejectedValue(new Error("Membership unavailable"));
    apiMocks.listSlackInstallations.mockResolvedValue({ configured: true, install_supported: true, installations: [installedBot] });
    renderTab(SlackTab, false);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(enSettings.query_feedback.permissions_load_failed);
    expect(screen.queryByRole("button", { name: "Disconnect" })).not.toBeInTheDocument();
    apiMocks.listMembers.mockResolvedValue(members);
    await user.click(within(alert).getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("button", { name: "Disconnect" })).toBeEnabled();
  });

  it("waits for GitHub membership and does not turn a failed read into contact-admin advice", async () => {
    const user = userEvent.setup();
    apiMocks.listMembers.mockRejectedValue(new Error("Membership unavailable"));
    renderTab(GitHubTab, false);

    const alert = await screen.findByRole("alert");
    expect(screen.queryByText(enSettings.github.contact_admin_to_connect)).not.toBeInTheDocument();
    expect(apiMocks.listGitHubInstallations).not.toHaveBeenCalled();
    apiMocks.listMembers.mockResolvedValue(members);
    await user.click(within(alert).getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("button", { name: "Connect GitHub" })).toBeEnabled();
  });

  it("keeps the GitHub cold-error Retry node mounted and busy until installation recovery", async () => {
    const user = userEvent.setup();
    apiMocks.listGitHubInstallations.mockRejectedValue(new Error("GitHub unavailable"));
    renderTab(GitHubTab);
    const alert = await screen.findByRole("alert");
    expect(screen.queryByText(enSettings.github.contact_admin_to_connect)).not.toBeInTheDocument();
    const retry = within(alert).getByRole("button", { name: "Retry" });
    const response = deferred<{ configured: boolean; can_manage: boolean; installations: never[] }>();
    apiMocks.listGitHubInstallations.mockReturnValue(response.promise);
    await user.click(retry);
    expect(screen.getByRole("button", { name: "Retrying..." })).toBe(retry);
    expect(retry).toBeDisabled();
    await act(async () => response.resolve({ configured: true, can_manage: true, installations: [] }));
    expect(await screen.findByRole("button", { name: "Connect GitHub" })).toBeEnabled();
  });

  it("does not show the VCS contact-admin hint before a successful permission/configuration read", async () => {
    const user = userEvent.setup();
    apiMocks.listVCSConnections.mockRejectedValue(new Error("VCS unavailable"));
    renderTab(VCSTab);
    const alert = await screen.findByRole("alert");
    expect(screen.queryByText(enSettings.vcs.contact_admin)).not.toBeInTheDocument();
    apiMocks.listVCSConnections.mockResolvedValue({ configured: true, can_manage: false, connections: [] });
    await user.click(within(alert).getByRole("button", { name: "Retry" }));
    expect(await screen.findByText(enSettings.vcs.contact_admin)).toBeVisible();
  });

  it("retains the same focused VCS token field across a failed refresh and retry", async () => {
    const user = userEvent.setup();
    const { client } = renderTab(VCSTab);
    const token = await screen.findByLabelText(enSettings.vcs.form_token_label);
    await user.type(token, "test-draft-token");
    apiMocks.listVCSConnections.mockRejectedValue(new Error("Refresh unavailable"));
    await act(async () => client.invalidateQueries({ queryKey: vcsConnectionsOptions("workspace-1").queryKey }));
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.getByLabelText(enSettings.vcs.form_token_label)).toBe(token);
    expect(token).toHaveValue("test-draft-token");
    expect(token).toHaveFocus();
    apiMocks.listVCSConnections.mockResolvedValue({ configured: true, can_manage: true, connections: [] });
    await user.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(screen.getByLabelText(enSettings.vcs.form_token_label)).toBe(token);
  });

  it("keeps a Telegram disconnect confirmation and its focused button after a transient refresh failure", async () => {
    const user = userEvent.setup();
    apiMocks.listTelegramInstallations.mockResolvedValue({ configured: true, installations: [installedBot] });
    const { client } = renderTab(TelegramTab);
    const disconnect = await screen.findByRole("button", { name: "Disconnect" });
    await user.click(disconnect);
    const dialog = screen.getByRole("alertdialog");
    const cancel = within(dialog).getByRole("button", { name: "Cancel" });
    cancel.focus();
    apiMocks.listTelegramInstallations.mockRejectedValue(new Error("Refresh unavailable"));
    await act(async () => client.invalidateQueries({ queryKey: telegramInstallationsOptions("workspace-1").queryKey }));
    await screen.findByText("Could not refresh Telegram settings. Previously loaded settings are still shown.");
    expect(disconnect.isConnected).toBe(true);
    expect(screen.getByRole("alertdialog")).toBe(dialog);
    expect(cancel).toHaveFocus();
  });

  it("hides cached GitHub management controls after definitive permission loss", async () => {
    const { client } = renderTab(GitHubTab);
    await screen.findByRole("button", { name: "Connect GitHub" });
    apiMocks.listGitHubInstallations.mockRejectedValue(new ApiError("Forbidden", 403, "Forbidden"));
    await act(async () => client.invalidateQueries({ queryKey: githubInstallationsOptions("workspace-1").queryKey }));
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Connect GitHub" })).not.toBeInTheDocument();
    expect(screen.queryByRole("switch")).not.toBeInTheDocument();
  });
});

describe("retained MCP and Composio editing", () => {
  it("keeps the exact MCP rename input and focus after a failed server refresh", async () => {
    const user = userEvent.setup();
    const { client } = renderTab(McpTab);
    await user.click(await screen.findByRole("button", { name: "Rename server team-tools" }));
    const input = screen.getByRole("textbox", { name: "Rename server" });
    await user.clear(input);
    await user.type(input, "retained-name");
    apiMocks.listWorkspaceMcpServers.mockRejectedValue(new Error("Refresh unavailable"));
    await act(async () => client.invalidateQueries({ queryKey: workspaceMcpServersOptions("workspace-1").queryKey }));
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.getByRole("textbox", { name: "Rename server" })).toBe(input);
    expect(input).toHaveValue("retained-name");
    expect(input).toHaveFocus();
  });

  it("removes the MCP editor when confirmed workspace access is lost", async () => {
    const user = userEvent.setup();
    const { client } = renderTab(McpTab);
    await user.click(await screen.findByRole("button", { name: "Rename server team-tools" }));
    apiMocks.listMembers.mockRejectedValue(new ApiError("Forbidden", 403, "Forbidden"));
    await act(async () => client.invalidateQueries({ queryKey: memberListOptions("workspace-1").queryKey }));
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.queryByRole("textbox", { name: "Rename server" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Add custom server" })).not.toBeInTheDocument();
  });

  it("preserves the focused Composio search node and filtered cards during catalog refresh failure", async () => {
    const user = userEvent.setup();
    const { client } = renderTab(ComposioTab);
    const search = await screen.findByPlaceholderText(enSettings.composio.search_placeholder);
    await user.type(search, "Notion");
    apiMocks.listComposioToolkits.mockRejectedValue(new Error("Refresh unavailable"));
    await act(async () => client.invalidateQueries({ queryKey: composioKeys.toolkits() }));
    expect(await screen.findByRole("alert")).toBeVisible();
    expect(screen.getByRole("textbox", { name: "Search Composio toolkits" })).toBe(search);
    expect(search).toHaveFocus();
    expect(search).toHaveValue("Notion");
    expect(screen.getByText("Notion", { exact: true })).toBeVisible();
  });

  it("blocks Composio connection writes until existing connections can be loaded", async () => {
    const user = userEvent.setup();
    apiMocks.listComposioConnections.mockRejectedValue(new Error("Connections unavailable"));
    renderTab(ComposioTab);
    const alert = await screen.findByRole("alert");
    expect(screen.getByRole("button", { name: "Connect" })).toBeDisabled();
    apiMocks.listComposioConnections.mockResolvedValue([]);
    await user.click(within(alert).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Connect" })).toBeEnabled());
  });

  it("associates each VCS webhook value and copy action with its visible field label", async () => {
    const user = userEvent.setup();
    apiMocks.connectVCS.mockResolvedValue({ id: "new-vcs", webhook_url: "https://hooks.test/vcs", webhook_secret: "test-webhook-secret" });
    renderTab(VCSTab);
    await user.type(await screen.findByLabelText("Instance URL"), "https://forgejo.test");
    await user.type(screen.getByLabelText("Access token"), "test-token");
    await user.click(screen.getByRole("button", { name: "Connect" }));
    expect(await screen.findByRole("textbox", { name: "Webhook URL" })).toHaveValue("https://hooks.test/vcs");
    expect(screen.getByRole("textbox", { name: "Webhook secret" })).toHaveValue("test-webhook-secret");
    expect(screen.getByRole("button", { name: "Copy Webhook URL" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Copy Webhook secret" })).toBeEnabled();
  });
});

describe("DingTalk group recovery", () => {
  it("does not restore a member's denied agent-filtered installations after a failed retry", async () => {
    const user = userEvent.setup();
    apiMocks.listMembers.mockResolvedValue([{ ...members[0], role: "member" }]);
    apiMocks.listAgents.mockResolvedValue([{ id: installedBot.agent_id }]);
    apiMocks.listDingTalkInstallations.mockResolvedValue({ configured: true, installations: [installedBot] });
    const { client } = renderTab(DingTalkTab, false);
    await screen.findByText(`Agent ${installedBot.agent_id}`);
    apiMocks.listAgents.mockRejectedValueOnce(new ApiError("denied", 403, "Forbidden"));
    await act(async () => client.invalidateQueries({ queryKey: workspaceKeys.agents("workspace-1"), exact: true }));
    const alert = await screen.findByRole("alert");
    expect(screen.queryByText(`Agent ${installedBot.agent_id}`)).not.toBeInTheDocument();
    apiMocks.listAgents.mockRejectedValueOnce(new Error("offline"));
    await user.click(within(alert).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Retry" })).toBeEnabled());
    expect(screen.queryByText(`Agent ${installedBot.agent_id}`)).not.toBeInTheDocument();
  });

  async function mountGroups(kind: "active" | "inactive", user: ReturnType<typeof userEvent.setup>) {
    apiMocks.listDingTalkInstallations.mockResolvedValue({ configured: true, installations: [installedBot] });
    apiMocks.listDingTalkGroups.mockImplementation(async (_workspace: string, params?: { activity?: string }) =>
      params?.activity === "inactive" ? inactiveGroups : recentGroups,
    );
    const view = renderTab(DingTalkTab);
    await screen.findByText(recentGroup.conversation_title);
    if (kind === "inactive") {
      await user.click(screen.getByRole("button", { name: /long inactive/i }));
      await screen.findByText(inactiveGroup.conversation_title);
    }
    return view;
  }

  it.each(["active", "inactive"] as const)("retains the same focused %s group node during refresh failure and retry", async (kind) => {
    const user = userEvent.setup();
    const { client } = await mountGroups(kind, user);
    const group = kind === "active" ? recentGroup : inactiveGroup;
    const key = kind === "active" ? dingtalkKeys.groups("workspace-1") : dingtalkKeys.inactiveGroups("workspace-1", installedBot.id);
    const title = screen.getByText(group.conversation_title);
    const identifier = screen.getByLabelText(`DingTalk group conversation ID ${group.conversation_id}`);
    act(() => identifier.focus());
    apiMocks.listDingTalkGroups.mockRejectedValueOnce(new ApiError("offline", 500, "Unavailable"));
    await act(async () => client.invalidateQueries({ queryKey: key, exact: true }));
    const retry = await screen.findByRole("button", { name: "Retry" });
    expect(title).toBeInTheDocument();
    expect(screen.getByText(group.conversation_title)).toBe(title);
    expect(identifier).toHaveFocus();
    expect(screen.getByRole("alert")).toBeVisible();

    const response = deferred<typeof inactiveGroups>();
    apiMocks.listDingTalkGroups.mockReturnValueOnce(response.promise);
    await user.click(retry);
    expect(retry).toBeDisabled();
    expect(screen.getByText(group.conversation_title)).toBe(title);
    await act(async () => response.resolve(kind === "active" ? recentGroups : inactiveGroups));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(screen.getByText(group.conversation_title)).toBe(title);
  });

  it.each(["active", "inactive"] as const)("does not restore denied %s groups after a transient retry", async (kind) => {
    const user = userEvent.setup();
    const { client } = await mountGroups(kind, user);
    const group = kind === "active" ? recentGroup : inactiveGroup;
    const key = kind === "active" ? dingtalkKeys.groups("workspace-1") : dingtalkKeys.inactiveGroups("workspace-1", installedBot.id);
    apiMocks.listDingTalkGroups.mockRejectedValueOnce(new ApiError("denied", 403, "Forbidden"));
    await act(async () => client.invalidateQueries({ queryKey: key, exact: true }));
    const retry = await screen.findByRole("button", { name: "Retry" });
    expect(screen.queryByText(group.conversation_title)).not.toBeInTheDocument();
    expect(client.getQueryData(key)).toBeUndefined();
    apiMocks.listDingTalkGroups.mockRejectedValueOnce(new Error("offline"));
    await user.click(retry);
    await waitFor(() => expect(screen.getByRole("button", { name: "Retry" })).toBeEnabled());
    expect(screen.queryByText(group.conversation_title)).not.toBeInTheDocument();
    expect(client.getQueryData(key)).toBeUndefined();
  });

  it("closes a denied inactive group's confirmation while keeping authorized active groups forgettable", async () => {
    const user = userEvent.setup();
    const { client } = await mountGroups("inactive", user);
    const inactiveRow = screen.getAllByTestId("dingtalk-group-item").find((row) =>
      within(row).queryByText(inactiveGroup.conversation_title),
    );
    if (!inactiveRow) throw new Error("The inactive group must be loaded");
    await user.click(within(inactiveRow).getByRole("button", { name: enSettings.dingtalk.group_forget }));
    expect(screen.getByRole("alertdialog")).toBeVisible();

    apiMocks.listDingTalkGroups.mockRejectedValueOnce(new ApiError("denied", 403, "Forbidden"));
    await act(async () => client.invalidateQueries({
      queryKey: dingtalkKeys.inactiveGroups("workspace-1", installedBot.id), exact: true,
    }));
    await screen.findByRole("alert");
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(screen.queryByText(inactiveGroup.conversation_title)).not.toBeInTheDocument();

    const recentRow = screen.getByTestId("dingtalk-group-item");
    expect(within(recentRow).getByText(recentGroup.conversation_title)).toBeVisible();
    const removal = deferred<void>();
    apiMocks.forgetDingTalkGroup.mockReturnValueOnce(removal.promise);
    await user.click(within(recentRow).getByRole("button", { name: enSettings.dingtalk.group_forget }));
    await user.click(within(screen.getByRole("alertdialog")).getByRole("button", {
      name: enSettings.dingtalk.group_forget_confirm,
    }));
    await waitFor(() => expect(apiMocks.forgetDingTalkGroup).toHaveBeenCalledExactlyOnceWith(
      "workspace-1", installedBot.id, recentGroup.conversation_id,
    ));
    await act(async () => removal.resolve());
  });
});
