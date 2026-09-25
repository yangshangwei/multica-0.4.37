import React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import type { Agent, AgentRoleTemplate, Squad, SquadTemplate, SquadMember } from "@multica/core/types";
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
  squads: [] as Squad[],
  squadTemplates: [] as SquadTemplate[],
  squadMembers: [] as SquadMember[],
  membershipPending: false,
  membershipError: false,
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
    groupBy: "none",
    setGroupBy: vi.fn(),
    sortField: "lastActive" as string,
    sortDirection: "desc" as string,
    hiddenColumns: ["model", "created"] as string[],
    filters: {
      availability: [] as string[],
      runtimes: [] as string[],
      owners: [] as string[],
      models: [] as string[],
      access: [] as string[],
      roles: [] as string[],
      squads: [] as string[],
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
  useSquadTemplates: () => ({ data: mocks.squadTemplates, isPending: false, isError: false, refetch: vi.fn() }),
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
  useQueries: ({ queries }: { queries: unknown[] }) => queries.map(() => ({
    data: mocks.membershipPending || mocks.membershipError ? undefined : mocks.squadMembers,
    isPending: mocks.membershipPending, isError: mocks.membershipError, refetch: vi.fn(),
  })),
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
    if (key === "squads") return { data: mocks.squads, isPending: false, isError: false, refetch: vi.fn() };
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

vi.mock("@multica/core/agents", async (importOriginal) => ({
  ...await importOriginal<typeof import("@multica/core/agents")>(),
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
  squadListOptions: () => ({ queryKey: ["squads"] }),
  squadMembersOptions: (_ws: string, id: string) => ({ queryKey: ["squad-members", id] }),
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
  mocks.squads = [];
  mocks.squadTemplates = [];
  mocks.squadMembers = [];
  mocks.membershipPending = false;
  mocks.membershipError = false;
  mocks.templates = [];
  mocks.templatesError = false;
  mocks.templatesPending = false;
  mocks.runCounts = [];
  mocks.runCountsPending = false;
  mocks.activity = { byAgent: new Map(), loading: false };
  mocks.presence = { byAgent: new Map(), loading: false };
  mocks.viewState.scope = "all";
  mocks.viewState.groupBy = "none";
  mocks.viewState.sortField = "lastActive";
  mocks.viewState.sortDirection = "desc";
  mocks.viewState.hiddenColumns = ["model", "created"];
  mocks.viewState.filters = {
    availability: [],
    runtimes: [],
    owners: [],
    models: [],
    access: [],
    roles: [],
    squads: [],
  };
});

const REVIEW_TEMPLATE: AgentRoleTemplate = {
  key: "code-reviewer", version: 1, name: "Reviewer", title: "Code reviewer",
  description: "Review changes and report actionable findings.", instructions: "Review the diff.",
  autonomy_level: "contributor", avatar_emoji: "", max_concurrent_tasks: 1, skill_names: [],
};

const RELEASE_SQUAD: Squad = {
  id: "release", workspace_id: "workspace-1", name: "Release readiness", description: "", instructions: "",
  avatar_url: null, leader_id: "lead", creator_id: "user-1", created_at: "2026-01-01", updated_at: "2026-01-01",
  archived_at: null, archived_by: null, agent_member_ids: ["lead", "agent-alpha"],
};

describe("AgentsPage discovery", () => {
  it("renders a shared expert once with links to both squads", () => {
    mocks.templates = [REVIEW_TEMPLATE];
    mocks.agents = [makeAgent({ id: "agent-alpha", name: "Payments expert", template_key: "code-reviewer" })];
    mocks.squads = [RELEASE_SQUAD, { ...RELEASE_SQUAD, id: "review", name: "Merge review" }];
    mocks.viewState.groupBy = "role";
    renderPage();
    expect(screen.getAllByRole("link", { name: "Payments expert" })).toHaveLength(1);
    expect(screen.getByRole("link", { name: "Release readiness" })).toHaveAttribute("href", "/test-workspace/squads/release");
    expect(screen.getByRole("link", { name: "Merge review" })).toHaveAttribute("href", "/test-workspace/squads/review");
    expect(screen.getByText("Role template: Code reviewer")).toBeInTheDocument();
  });

  it("waits for a selected legacy squad roster and reports failure instead of no matches", () => {
    mocks.squads = [{ ...RELEASE_SQUAD, agent_member_ids: undefined }];
    mocks.viewState.filters.squads = ["release"];
    mocks.membershipPending = true;
    const view = renderPage();
    expect(screen.queryByText("Alpha Agent")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("skeleton").length).toBeGreaterThan(0);
    mocks.membershipPending = false;
    mocks.membershipError = true;
    view.rerender(<NavigationProvider value={makeAdapter()}><AgentsPage /></NavigationProvider>);
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load squad members.");
    expect(screen.queryByText("Alpha Agent")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("keeps templates in the creation flow instead of the saved directory", () => {
    mocks.templates = [REVIEW_TEMPLATE];
    renderPage();
    expect(screen.queryByRole("region", { name: "Built-in agents" })).not.toBeInTheDocument();
    expect(screen.getByText("Alpha Agent")).toBeInTheDocument();
  });

  it("keeps the empty directory focused on creating its first agent", () => {
    mocks.agents = [];
    mocks.templates = [REVIEW_TEMPLATE];
    renderPage();
    expect(screen.getByText("No agents yet")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Built-in agents" })).not.toBeInTheDocument();
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
