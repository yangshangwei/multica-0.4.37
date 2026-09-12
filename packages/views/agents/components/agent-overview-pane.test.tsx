// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Agent, AgentRuntime } from "@multica/core/types";
import { configStore } from "@multica/core/config";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../locales/en/common.json";
import enAgents from "../../locales/en/agents.json";
import {
  NavigationProvider,
  type NavigationAdapter,
} from "../../navigation";

const TEST_RESOURCES = { en: { common: enCommon, agents: enAgents } };

// AgentOverviewPane pulls in ActorIssuesPanel which in turn touches the api
// layer. The test only cares about which top-of-pane tab buttons render,
// not what each tab does, so we stub the heavy children.
vi.mock("./tabs/activity-tab", () => ({
  ActivityTab: () => <div>activity-tab</div>,
  AgentPerformanceSummary: () => <div>performance-summary</div>,
}));
vi.mock("./agent-overview-summary", () => ({
  AgentOverviewSummary: () => <div>agent-overview-summary</div>,
}));
vi.mock("./agent-access-settings", () => ({
  AgentAccessSettings: () => <div>agent-access-settings</div>,
}));
vi.mock("./tabs/instructions-tab", () => ({
  InstructionsTab: () => <div>instructions-tab</div>,
}));
vi.mock("./tabs/skills-tab", () => ({
  SkillsTab: () => <div>skills-tab</div>,
}));
vi.mock("./tabs/env-tab", () => ({
  EnvTab: () => <div>env-tab</div>,
}));
vi.mock("./tabs/custom-args-tab", () => ({
  CustomArgsTab: () => <div>custom-args-tab</div>,
}));
vi.mock("./tabs/mcp-config-tab", () => ({
  McpConfigTab: () => <div>mcp-config-tab</div>,
}));
vi.mock("./tabs/integrations-tab", () => ({
  IntegrationsTab: () => <div>integrations-tab</div>,
}));
vi.mock("../../common/actor-issues-panel", () => ({
  ActorIssuesPanel: () => <div>actor-issues-panel</div>,
}));

// The pane now reads workspace context to decide whether the Integrations
// tab is worth showing (it queries Lark installations to learn whether the
// deployment has the feature configured). Provide a stable workspace id and
// a listing query backed by a ref so each test can flip `configured`.
const larkListingRef = vi.hoisted(() => ({
  current: { installations: [] as unknown[], configured: false },
}));
const slackListingRef = vi.hoisted(() => ({
  current: { installations: [] as unknown[], configured: false },
}));
const dingtalkListingRef = vi.hoisted(() => ({
  current: { installations: [] as unknown[], configured: false },
}));
const wecomListingRef = vi.hoisted(() => ({
  current: { installations: [] as unknown[], configured: false },
}));
const telegramListingRef = vi.hoisted(() => ({
  current: { installations: [] as unknown[], configured: false },
}));
const providerQueryCalls = vi.hoisted(() => [] as string[]);
vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));
vi.mock("@multica/core/lark", () => ({
  larkInstallationsOptions: () => ({
    queryKey: ["lark", "installations"],
    queryFn: () => {
      providerQueryCalls.push("lark");
      return Promise.resolve(larkListingRef.current);
    },
  }),
}));
vi.mock("@multica/core/slack", () => ({
  slackInstallationsOptions: () => ({
    queryKey: ["slack", "installations"],
    queryFn: () => {
      providerQueryCalls.push("slack");
      return Promise.resolve(slackListingRef.current);
    },
  }),
}));
vi.mock("@multica/core/dingtalk", () => ({
  dingtalkInstallationsOptions: () => ({
    queryKey: ["dingtalk", "installations"],
    queryFn: () => {
      providerQueryCalls.push("dingtalk");
      return Promise.resolve(dingtalkListingRef.current);
    },
  }),
}));
vi.mock("@multica/core/wecom", () => ({
  wecomInstallationsOptions: () => ({
    queryKey: ["wecom", "installations"],
    queryFn: () => {
      providerQueryCalls.push("wecom");
      return Promise.resolve(wecomListingRef.current);
    },
  }),
}));
vi.mock("@multica/core/telegram", () => ({
  telegramInstallationsOptions: () => ({
    queryKey: ["telegram", "installations"],
    queryFn: () => {
      providerQueryCalls.push("telegram");
      return Promise.resolve(telegramListingRef.current);
    },
  }),
}));

import { AgentOverviewPane, type DetailTab } from "./agent-overview-pane";

const baseAgent: Agent = {
  id: "agent-1",
  workspace_id: "ws-1",
  runtime_id: "runtime-1",
  name: "Agent",
  description: "",
  instructions: "",
  avatar_url: null,
  runtime_mode: "local",
  runtime_config: {},
  custom_args: [],
  visibility: "workspace",
  permission_mode: "public_to",
  invocation_targets: [{ target_type: "workspace", target_id: null }],
  status: "idle",
  max_concurrent_tasks: 1,
  model: "",
  owner_id: "user-1",
  skills: [],
  created_at: "2026-05-28T00:00:00Z",
  updated_at: "2026-05-28T00:00:00Z",
  archived_at: null,
  archived_by: null,
};

function makeRuntime(provider: string): AgentRuntime {
  return {
    id: "runtime-1",
    workspace_id: "ws-1",
    daemon_id: null,
    name: "Runtime",
    runtime_mode: "local",
    provider,
    launch_header: "",
    status: "online",
    device_info: "",
    metadata: {},
    owner_id: null,
    visibility: "private",
    last_seen_at: null,
    created_at: "2026-05-28T00:00:00Z",
    updated_at: "2026-05-28T00:00:00Z",
  };
}

function renderPane(
  runtimes: AgentRuntime[],
  {
    canEdit = true,
    view,
    navIntent,
    onNavIntentHandled,
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    }),
  }: {
    canEdit?: boolean;
    view?: DetailTab;
    navIntent?: DetailTab;
    onNavIntentHandled?: () => void;
    queryClient?: QueryClient;
  } = {},
) {
  const navigation: NavigationAdapter = {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/agents/agent-1",
    searchParams: new URLSearchParams(view ? { view } : {}),
    hash: "",
    getShareableUrl: (path) => path,
  };
  const result = render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <NavigationProvider value={navigation}>
        <QueryClientProvider client={queryClient}>
          <AgentOverviewPane
            agent={baseAgent}
            runtime={runtimes[0] ?? null}
            owner={null}
            runtimes={runtimes}
            members={[]}
            onUpdate={vi.fn().mockResolvedValue(undefined)}
            canEdit={canEdit}
            navIntent={navIntent}
            onNavIntentHandled={onNavIntentHandled}
          />
        </QueryClientProvider>
      </NavigationProvider>
    </I18nProvider>,
  );
  return { ...result, navigation, queryClient };
}

function openCapabilities() {
  fireEvent.click(screen.getByRole("tab", { name: /^Capabilities$/i }));
}

function openSettings() {
  fireEvent.click(screen.getByRole("tab", { name: /^Settings$/i }));
}

beforeEach(() => {
  configStore.getState().setAuthConfig({
    allowSignup: true,
    messagingIntegrationsEnabled: true,
  });
  providerQueryCalls.length = 0;
  larkListingRef.current = { installations: [], configured: false };
  slackListingRef.current = { installations: [], configured: false };
  dingtalkListingRef.current = { installations: [], configured: false };
  wecomListingRef.current = { installations: [], configured: false };
  telegramListingRef.current = { installations: [], configured: false };
});

describe("AgentOverviewPane MCP tab visibility", () => {
  it.each([
    ["Claude", "claude"],
    ["Codex", "codex"],
    ["Cursor", "cursor"],
    ["Hermes", "hermes"],
    ["Kimi", "kimi"],
    ["Kiro", "kiro"],
    ["OpenCode", "opencode"],
    ["OpenClaw", "openclaw"],
    ["Oh My Pi", "omp"],
  ])("renders the MCP tab when the agent runs on the %s runtime", (_label, provider) => {
    renderPane([makeRuntime(provider)]);
    openCapabilities();
    expect(screen.getByRole("tab", { name: /^MCP$/i })).toBeInTheDocument();
  });

  it("hides the MCP tab for providers whose backend does not read mcp_config", () => {
    // Saving an MCP config on e.g. Gemini would be a silent no-op at run
    // time — that's the bug this hiding logic is meant to prevent.
    renderPane([makeRuntime("gemini")]);
    openCapabilities();
    expect(
      screen.queryByRole("tab", { name: /^MCP$/i }),
    ).not.toBeInTheDocument();
  });

  it("keeps the MCP tab visible when the runtime row hasn't loaded yet", () => {
    // Empty runtimes[] mimics the brief window between the page mounting and
    // the runtimes query resolving. Hiding the tab would flicker it off and
    // then back on, which reads as a bug.
    renderPane([]);
    openCapabilities();
    expect(screen.getByRole("tab", { name: /^MCP$/i })).toBeInTheDocument();
  });
});

describe("AgentOverviewPane Integrations tab visibility", () => {
  it("does not fetch messaging providers when disabled by deployment", () => {
    configStore.getState().setAuthConfig({
      allowSignup: true,
      messagingIntegrationsEnabled: false,
    });

    renderPane([makeRuntime("claude")]);

    expect(providerQueryCalls).toEqual([]);
  });

  it("hides the disabled Integrations tab even when configured providers remain cached", () => {
    configStore.getState().setAuthConfig({
      allowSignup: true,
      messagingIntegrationsEnabled: false,
    });
    const queryClient = new QueryClient();
    for (const channel of ["lark", "slack", "dingtalk", "wecom", "telegram"]) {
      queryClient.setQueryData([channel, "installations"], {
        installations: [],
        configured: true,
      });
    }

    renderPane([makeRuntime("claude")], { queryClient });
    openCapabilities();

    expect(screen.queryByRole("tab", { name: /^Integrations$/i })).not.toBeInTheDocument();
    expect(providerQueryCalls).toEqual([]);
  });

  it("normalizes a saved Integrations URL to Overview when messaging is disabled", () => {
    configStore.getState().setAuthConfig({
      allowSignup: true,
      messagingIntegrationsEnabled: false,
    });

    const { navigation } = renderPane([makeRuntime("claude")], {
      view: "integrations",
    });

    expect(screen.getByRole("tab", { name: /^Overview$/i })).toHaveAttribute("aria-selected", "true");
    expect(navigation.replace).toHaveBeenCalledWith("/acme/agents/agent-1");
    expect(screen.queryByText("integrations-tab")).not.toBeInTheDocument();
  });

  it("recovers from an already selected Integrations tab when the policy disables it", async () => {
    larkListingRef.current = { installations: [], configured: true };
    const { navigation } = renderPane([makeRuntime("claude")]);
    openCapabilities();
    fireEvent.click(await screen.findByRole("tab", { name: /^Integrations$/i }));
    expect(screen.getByRole("tab", { name: /^Integrations$/i })).toHaveAttribute("aria-selected", "true");

    act(() => {
      configStore.getState().setAuthConfig({
        allowSignup: true,
        messagingIntegrationsEnabled: false,
      });
    });

    expect(screen.getByRole("tab", { name: /^Overview$/i })).toHaveAttribute("aria-selected", "true");
    expect(navigation.replace).toHaveBeenLastCalledWith("/acme/agents/agent-1");
    expect(screen.queryByText("integrations-tab")).not.toBeInTheDocument();
  });

  it("resolves an imperative Integrations intent to Overview when messaging is disabled", () => {
    configStore.getState().setAuthConfig({
      allowSignup: true,
      messagingIntegrationsEnabled: false,
    });
    const onNavIntentHandled = vi.fn();

    const { navigation } = renderPane([makeRuntime("claude")], {
      view: "work",
      navIntent: "integrations",
      onNavIntentHandled,
    });

    expect(screen.getByRole("tab", { name: /^Overview$/i })).toHaveAttribute("aria-selected", "true");
    expect(navigation.replace).toHaveBeenLastCalledWith("/acme/agents/agent-1");
    expect(onNavIntentHandled).toHaveBeenCalled();
  });

  it("shows the Integrations tab once the deployment has Lark configured", async () => {
    larkListingRef.current = { installations: [], configured: true };
    renderPane([makeRuntime("claude")]);
    openCapabilities();
    expect(
      await screen.findByRole("tab", { name: /^Integrations$/i }),
    ).toBeInTheDocument();
  });

  it("shows the Integrations tab when only Slack is configured (Lark off)", async () => {
    // Regression: the tab gate must consider Slack too, not just Lark —
    // a Slack-only deployment was hiding the tab (and its bind entry).
    slackListingRef.current = { installations: [], configured: true };
    renderPane([makeRuntime("claude")]);
    openCapabilities();
    expect(
      await screen.findByRole("tab", { name: /^Integrations$/i }),
    ).toBeInTheDocument();
  });

  it("shows the Integrations tab when only Telegram is configured", async () => {
    telegramListingRef.current = { installations: [], configured: true };
    renderPane([makeRuntime("claude")]);
    openCapabilities();
    expect(
      await screen.findByRole("tab", { name: /^Integrations$/i }),
    ).toBeInTheDocument();
  });

  it("hides the Integrations tab when no channel integration is configured", () => {
    // Default refs are configured:false; the tab must not appear on a
    // deployment without any channel integration, the common case.
    renderPane([makeRuntime("claude")]);
    openCapabilities();
    expect(
      screen.queryByRole("tab", { name: /^Integrations$/i }),
    ).not.toBeInTheDocument();
  });
});

describe("AgentOverviewPane Settings navigation", () => {
  it("gives Access its own settings tab", () => {
    renderPane([makeRuntime("claude")]);
    openSettings();
    expect(screen.getByRole("tab", { name: /^Access$/i })).toBeInTheDocument();
  });
});

describe("AgentOverviewPane Environment tab visibility", () => {
  it("shows the Environment tab to someone who can manage the agent", () => {
    renderPane([makeRuntime("claude")]);
    openSettings();
    expect(
      screen.getByRole("tab", { name: /^Environment$/i }),
    ).toBeInTheDocument();
  });

  it("hides the Environment tab from users who cannot manage the agent", () => {
    // The env endpoints admit the agent owner or a workspace owner/admin
    // (MUL-5438) — the rule `canEdit` already encodes. Anyone else who opens
    // the tab hits a guaranteed 403 on "Reveal & edit".
    renderPane([makeRuntime("claude")], { canEdit: false });
    openSettings();
    expect(
      screen.queryByRole("tab", { name: /^Environment$/i }),
    ).not.toBeInTheDocument();
  });
});
