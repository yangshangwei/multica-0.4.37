import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import type { Agent, AgentRoleTemplate } from "@multica/core/types";
import type { AgentActivity } from "@multica/core/agents";
import { renderWithI18n } from "../../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { AgentsPage } from "./agents-page";

// These tests pin the `listReady` render gate (MUL-4511): the Agents list must
// not paint real rows until the auxiliary queries the active sort field /
// filter depends on have landed, or it sorts on placeholder values
// (lastActiveDays null→Infinity, runCount 0) and visibly re-orders when each
// query resolves. The gate waits per need — nothing for name/created,
// run-counts for runs, activity + run-counts for the default lastActive,
// presence when an availability filter is active — and never blocks the empty
// state on those queries.

const mocks = vi.hoisted(() => ({
  agents: [] as Agent[],
  agentsLoading: false,
  templates: [] as AgentRoleTemplate[],
  templatesError: false,
  templatesPending: false,
  refetchTemplates: vi.fn(),
  runCounts: [] as Array<{ agent_id: string; run_count: number }>,
  runCountsPending: false,
  activity: {
    byAgent: new Map<string, AgentActivity>(),
    loading: false,
  },
  presence: {
    byAgent: new Map<string, unknown>(),
    loading: false,
  },
  viewState: {
    scope: "all",
    sortField: "lastActive" as string,
    sortDirection: "desc" as string,
    hiddenColumns: ["model", "created"] as string[],
    filters: {
      availability: [] as string[],
      runtimes: [] as string[],
      owners: [] as string[],
      models: [] as string[],
      access: [] as string[],
    },
    setScope: vi.fn(),
    toggleSort: vi.fn(),
    setSortField: vi.fn(),
    setSortDirection: vi.fn(),
    toggleColumn: vi.fn(),
    toggleFilter: vi.fn(),
    clearFilters: vi.fn(),
  },
}));

vi.mock("../create/use-role-templates", () => ({
  useRoleTemplates: () => ({
    data: mocks.templates,
    isLoading: false,
    isPending: mocks.templatesPending,
    isError: mocks.templatesError,
    refetch: mocks.refetchTemplates,
  }),
}));

vi.mock("@tanstack/react-query", () => ({
  // Identity, which is what the real one is for our purposes: callers build an
  // options object and hand it straight to the useQuery stub below. Needed because
  // the header's approval-count query is declared with queryOptions().
  queryOptions: (options: unknown) => options,
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    const key = options.queryKey?.[0];
    if (key === "agents") {
      return {
        data: mocks.agents,
        isLoading: mocks.agentsLoading,
        error: null,
        refetch: vi.fn(),
      };
    }
    if (key === "agent-run-counts") {
      return { data: mocks.runCounts, isPending: mocks.runCountsPending };
    }
    return { data: [], isLoading: false, isPending: false };
  },
  useQueryClient: () => ({ invalidateQueries: vi.fn() }),
}));

// The list virtualizes; render every row so DOM order reflects sort order.
vi.mock("@tanstack/react-virtual", () => ({
  useVirtualizer: ({ count }: { count: number }) => ({
    getVirtualItems: () =>
      Array.from({ length: count }, (_, index) => ({
        index,
        key: index,
        start: index * 64,
        end: (index + 1) * 64,
        size: 64,
      })),
    getTotalSize: () => count * 64,
  }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("@multica/core/agents", () => ({
  isAgentRuntimeBound: (agent: { runtime_id: string; runtime_bound?: boolean }) =>
    agent.runtime_bound !== false && agent.runtime_id.length > 0,
  agentRunCounts30dOptions: () => ({ queryKey: ["agent-run-counts"] }),
  useWorkspaceActivityMap: () => mocks.activity,
  useWorkspacePresenceMap: () => mocks.presence,
  VISIBILITY_TOOLTIP: { private: "Private", workspace: "Workspace" },
  effectiveAccessScope: (pm: unknown, it: unknown) => {
    if (pm !== "public_to") return "owner-only";
    if ((Array.isArray(it) ? it : []).some((t) => (t as {target_type?: string})?.target_type === "workspace")) return "workspace";
    return "specific-people";
  },
  ALL_ACCESS_SCOPES: ["workspace", "specific-people", "owner-only"],
}));

vi.mock("@multica/core/agents/stores", () => ({
  useAgentsViewStore: (selector: (state: unknown) => unknown) =>
    selector(mocks.viewState),
  AGENT_DEFAULT_HIDDEN_COLUMNS: ["model", "created"],
  AGENT_SCOPES: ["mine", "all", "archived"],
}));

vi.mock("@multica/core/api", () => ({
  api: { archiveAgent: vi.fn(), restoreAgent: vi.fn() },
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ user: { id: "user-1" } }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

// Derived from the real path factory rather than a hand-written literal: a
// stub that lists only the routes this page used at the time silently breaks
// when the page starts linking somewhere new.
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return {
    ...actual,
    useWorkspacePaths: () => actual.paths.workspace("test-workspace"),
  };
});

vi.mock("@multica/core/workspace/queries", () => ({
  agentListOptions: () => ({ queryKey: ["agents"] }),
  memberListOptions: () => ({ queryKey: ["members"] }),
  workspaceKeys: { agents: (wsId: string) => ["agents", wsId] },
}));

vi.mock("@multica/core/runtimes", () => ({
  runtimeListOptions: () => ({ queryKey: ["runtimes"] }),
}));

// View-layer children with heavy / portal deps — stub to keep the test focused
// on the gate, not on avatars, row menus, the toolbar, or tooltip portals.
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => null }));
vi.mock("./agent-row-actions", () => ({ AgentRowActions: () => null }));
vi.mock("./agent-list-toolbar", () => ({
  AgentListToolbar: () => <div data-testid="agent-list-toolbar" />,
  countActiveFilterDimensions: () => 0,
}));
vi.mock("../presence", () => ({ availabilityConfig: {} }));
vi.mock("@multica/ui/components/ui/skeleton", () => ({
  Skeleton: (props: Record<string, unknown>) => (
    <div data-testid="skeleton" {...props} />
  ),
}));
vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => (
    <div role="tooltip">{children}</div>
  ),
}));

const BASE_AGENT: Agent = {
  id: "agent-base",
  workspace_id: "workspace-1",
  runtime_id: "runtime-1",
  name: "Base Agent",
  description: "",
  instructions: "",
  avatar_url: null,
  runtime_mode: "cloud",
  runtime_config: {},
  custom_args: [],
  visibility: "workspace",
  permission_mode: "private",
  invocation_targets: [],
  status: "idle",
  max_concurrent_tasks: 1,
  model: "claude",
  owner_id: "user-1",
  skills: [],
  created_at: "2026-06-01T00:00:00Z",
  updated_at: "2026-06-01T00:00:00Z",
  archived_at: null,
  archived_by: null,
};

function makeAgent(over: Partial<Agent>): Agent {
  return { ...BASE_AGENT, ...over };
}

// Build a 30-bucket activity series whose most-recent bucket with runs is
// `daysAgo` days back — `lastActiveDaysAgo` reads exactly this.
function activityLastActive(daysAgo: number): AgentActivity {
  const buckets = Array.from({ length: 30 }, () => ({ total: 0, failed: 0 }));
  buckets[29 - daysAgo] = { total: 1, failed: 0 };
  return { buckets, daysSinceCreated: 30 };
}

const ALPHA = makeAgent({ id: "a-alpha", name: "Alpha Agent" });
const BETA = makeAgent({ id: "a-beta", name: "Beta Agent" });

function makeAdapter(
  overrides: Partial<NavigationAdapter> = {},
): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/test-workspace/agents",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (p) => p,
    ...overrides,
  };
}

function renderPage(adapter = makeAdapter()) {
  return renderWithI18n(
    <NavigationProvider value={adapter}>
      <AgentsPage />
    </NavigationProvider>,
  );
}

/** Beta before Alpha in document order? */
function betaPrecedesAlpha(): boolean {
  const alpha = screen.getByText("Alpha Agent");
  const beta = screen.getByText("Beta Agent");
  return Boolean(
    beta.compareDocumentPosition(alpha) & Node.DOCUMENT_POSITION_FOLLOWING,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.agents = [ALPHA, BETA];
  mocks.agentsLoading = false;
  mocks.templates = [];
  mocks.templatesError = false;
  mocks.templatesPending = false;
  mocks.runCounts = [];
  mocks.runCountsPending = false;
  mocks.activity = { byAgent: new Map(), loading: false };
  mocks.presence = { byAgent: new Map(), loading: false };
  mocks.viewState.scope = "all";
  mocks.viewState.sortField = "lastActive";
  mocks.viewState.sortDirection = "desc";
  mocks.viewState.hiddenColumns = ["model", "created"];
  mocks.viewState.filters = {
    availability: [],
    runtimes: [],
    owners: [],
    models: [],
    access: [],
  };
});

const REVIEW_TEMPLATE: AgentRoleTemplate = {
  key: "code-reviewer", version: 1, name: "Reviewer", title: "Code reviewer",
  description: "Review changes and report actionable findings.", instructions: "Review the diff.",
  autonomy_level: "contributor", avatar_emoji: "", max_concurrent_tasks: 1, skill_names: [],
};

describe("AgentsPage built-in catalog", () => {
  it("shows pending catalogs as loading even when the network request is paused", () => {
    mocks.templatesPending = true;
    renderPage();
    const catalog = screen.getByRole("region", { name: "Built-in agents" });
    fireEvent.click(within(catalog).getByRole("button", { name: /^Built-in agents/ }));
    expect(within(catalog).getByRole("status")).toHaveTextContent("Loading built-in agents...");
    expect(within(catalog).queryByText("No built-in agents are available.")).not.toBeInTheDocument();
  });

  it("keeps templates collapsed in an empty workspace and opens the existing flow on request", () => {
    mocks.agents = [];
    mocks.templates = [REVIEW_TEMPLATE];
    const adapter = makeAdapter();
    renderPage(adapter);

    const catalog = screen.getByRole("region", { name: "Built-in agents" });
    const toggle = within(catalog).getByRole("button", { name: "Built-in agents 1" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(within(catalog).queryByRole("button", { name: "View template" })).not.toBeInTheDocument();
    fireEvent.click(toggle);
    expect(within(catalog).getByText(REVIEW_TEMPLATE.description)).toBeInTheDocument();
    expect(screen.getByText("No agents yet")).toBeInTheDocument();
    expect(within(screen.getByRole("heading", { level: 1 }).parentElement!).queryByText("1")).not.toBeInTheDocument();
    fireEvent.click(within(catalog).getByRole("button", { name: "View template" }));
    expect(adapter.push).toHaveBeenCalledWith("/test-workspace/agents/new/template?template=code-reviewer");
  });

  it("opens only a live instance with matching template identity, including a renamed instance", () => {
    mocks.templates = [REVIEW_TEMPLATE];
    mocks.agents = [
      makeAgent({ id: "same-name", name: "Reviewer" }),
      makeAgent({ id: "archived", name: "Old reviewer", template_key: "code-reviewer", archived_at: "2026-09-01T00:00:00Z" }),
      makeAgent({ id: "renamed", name: "Payments reviewer", template_key: "code-reviewer" }),
    ];
    const adapter = makeAdapter();
    renderPage(adapter);

    fireEvent.click(screen.getByRole("button", { name: /^Built-in agents/ }));
    fireEvent.click(within(screen.getByRole("region", { name: "Built-in agents" })).getByRole("button", { name: "Open agent" }));
    expect(adapter.push).toHaveBeenCalledWith("/test-workspace/agents/renamed");
  });

  it("does not label a same-name custom agent as a template instance and lets a failed catalog retry", () => {
    mocks.templates = [REVIEW_TEMPLATE];
    mocks.agents = [makeAgent({ name: "Reviewer" })];
    const view = renderPage();
    fireEvent.click(screen.getByRole("button", { name: /^Built-in agents/ }));
    expect(within(screen.getByRole("region", { name: "Built-in agents" })).queryByRole("button", { name: "Open agent" })).not.toBeInTheDocument();

    mocks.templatesError = true;
    view.rerender(<NavigationProvider value={makeAdapter()}><AgentsPage /></NavigationProvider>);
    fireEvent.click(within(screen.getByRole("region", { name: "Built-in agents" })).getByRole("button", { name: "Retry" }));
    expect(mocks.refetchTemplates).toHaveBeenCalledOnce();
    expect(screen.getByTestId("agent-list-toolbar")).toBeInTheDocument();
    expect(screen.getByText("Reviewer")).toBeInTheDocument();
  });
});

describe("AgentsPage listReady gate", () => {
  it("shows only a skeleton (no real rows) while lastActive deps are pending", () => {
    // Default lastActive sort depends on activity + run-counts.
    mocks.activity = { byAgent: new Map(), loading: true };
    mocks.runCountsPending = true;

    renderPage();

    expect(screen.queryByText("Alpha Agent")).not.toBeInTheDocument();
    expect(screen.queryByText("Beta Agent")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
  });

  it("renders rows in the resolved lastActive order once deps land", () => {
    // Alpha active 5d ago, Beta active today → lastActive desc puts Beta first,
    // the opposite of the name-order fallback the ungated list would show.
    mocks.activity = {
      byAgent: new Map<string, AgentActivity>([
        [ALPHA.id, activityLastActive(5)],
        [BETA.id, activityLastActive(0)],
      ]),
      loading: false,
    };
    mocks.runCounts = [
      { agent_id: ALPHA.id, run_count: 0 },
      { agent_id: BETA.id, run_count: 0 },
    ];
    mocks.runCountsPending = false;

    renderPage();

    expect(screen.getByText("Alpha Agent")).toBeInTheDocument();
    expect(screen.getByText("Beta Agent")).toBeInTheDocument();
    expect(betaPrecedesAlpha()).toBe(true);
  });

  it("renders rows immediately for name sort without waiting on activity/run-counts", () => {
    mocks.viewState.sortField = "name";
    mocks.viewState.sortDirection = "asc";
    // Auxiliary queries are still in flight — name sort must not wait on them.
    mocks.activity = { byAgent: new Map(), loading: true };
    mocks.runCountsPending = true;
    mocks.presence = { byAgent: new Map(), loading: true };

    renderPage();

    expect(screen.getByText("Alpha Agent")).toBeInTheDocument();
    expect(screen.getByText("Beta Agent")).toBeInTheDocument();
    // name asc → Alpha before Beta.
    expect(betaPrecedesAlpha()).toBe(false);
  });

  it("shows a skeleton (not a false empty/false result) while an availability filter waits on presence", () => {
    // Availability filter needs presence; sort by name so ONLY presence gates.
    // Ungated, presence-null rows would all be filtered out → a false "no
    // matches" state. Gated, we hold on a skeleton instead.
    mocks.viewState.sortField = "name";
    mocks.viewState.filters.availability = ["online"];
    mocks.presence = { byAgent: new Map(), loading: true };

    renderPage();

    expect(screen.queryByText("Alpha Agent")).not.toBeInTheDocument();
    expect(screen.queryByText("Beta Agent")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
  });

  it("shows the empty state without blocking on auxiliary queries when there are no agents", () => {
    mocks.agents = [];
    // All auxiliary queries pending — the empty state must not wait on them.
    mocks.activity = { byAgent: new Map(), loading: true };
    mocks.runCountsPending = true;
    mocks.presence = { byAgent: new Map(), loading: true };

    renderPage();

    expect(screen.getByText("No agents yet")).toBeInTheDocument();
    expect(screen.queryByTestId("skeleton")).not.toBeInTheDocument();
  });
});
