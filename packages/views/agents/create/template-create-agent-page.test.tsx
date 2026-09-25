// @vitest-environment jsdom

import type { ReactNode } from "react";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Agent, AgentRoleTemplate } from "@multica/core/types";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { ScrollRestorationProvider } from "../../platform/scroll-restoration";
import { renderWithI18n } from "../../test/i18n";
import enAgents from "../../locales/en/agents.json";

const mockListTemplates = vi.hoisted(() => vi.fn());
const mockListAgents = vi.hoisted(() => vi.fn());
const mockCreateAgent = vi.hoisted(() => vi.fn());
const mockAddSquadMember = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

// Partial mock: the module also exports WORKSPACE_PAGES, which the skill picker
// reads at import time through the configuration panel's dependency graph.
vi.mock("@multica/core/paths", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/paths")>()),
  useWorkspacePaths: () => ({
    agents: () => "/acme/agents",
    newAgent: () => "/acme/agents/new",
    newAgentTemplate: () => "/acme/agents/new/template",
    agentDetail: (id: string) => `/acme/agents/${id}`,
    squadDetail: (id: string) => `/acme/squads/${id}`,
  }),
}));

vi.mock("@multica/core/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/api")>()),
  api: {
    listAgentRoleTemplates: mockListTemplates,
    listAgents: mockListAgents,
    createAgentFromTemplate: mockCreateAgent,
    addSquadMember: mockAddSquadMember,
  },
}));

// Configuration and submission have their own canonical suites. Keep this
// suite on the picker, route transitions and non-mutating navigation.
vi.mock("./agent-configuration-panel", () => ({ AgentConfigurationPanel: () => null }));
vi.mock("./use-create-agent-form", async () => {
  const { EMPTY_AGENT_DRAFT } = await import("@multica/core/agents");
  return {
    useCreateAgentForm: () => ({
      draft: EMPTY_AGENT_DRAFT, setDraft: vi.fn(), selectedRuntime: null,
      runtimes: [], runtimesLoading: false, members: [], currentUserId: "user-1", draftReady: false,
    }),
  };
});

import { TemplateCreateAgentPage } from "./template-create-agent-page";

const TEMPLATES: AgentRoleTemplate[] = [
  {
    key: "product-analyst",
    version: 1,
    name: "Product Analyst",
    title: "Product Analyst",
    description: "Turns a vague request into a decidable one.",
    autonomy_level: "observer",
    avatar_emoji: "🔍",
    max_concurrent_tasks: 3,
    skill_names: ["multica-requirement-clarification"],
    instructions: "# Product Analyst",
  },
  {
    key: "release-engineer",
    version: 2,
    name: "Release Engineer",
    title: "Release Engineer",
    description: "Prepares the release and the rollback.",
    autonomy_level: "operator",
    avatar_emoji: "🚦",
    max_concurrent_tasks: 1,
    skill_names: ["multica-release-check"],
    instructions: "# Release Engineer",
  },
  {
    // A level this client has never heard of: the card must still render.
    key: "future-role",
    version: 1,
    name: "Future Role",
    title: "Future Role",
    description: "Ships with a newer server.",
    autonomy_level: "supervisor",
    avatar_emoji: "🛰️",
    max_concurrent_tasks: 1,
    skill_names: [],
    instructions: "# Future",
  },
];

function agent(id: string, name: string, overrides: Partial<Agent> = {}): Agent {
  return {
    id, name, workspace_id: "ws-1", runtime_id: "runtime-1", description: "My saved description",
    instructions: "My saved instructions", avatar_url: null, runtime_mode: "local", runtime_config: {},
    custom_args: [], visibility: "private", permission_mode: "private", invocation_targets: [],
    status: "idle", max_concurrent_tasks: 1, model: "", owner_id: "user-1", skills: [],
    created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z",
    archived_at: null, archived_by: null, template_key: "product-analyst", ...overrides,
  };
}

function makeNavigation(overrides: Partial<NavigationAdapter> = {}): NavigationAdapter {
  return {
    push: vi.fn(), replace: vi.fn(), back: vi.fn(),
    pathname: "/acme/agents/new/template", searchParams: new URLSearchParams(),
    hash: "", getShareableUrl: (path) => path, ...overrides,
  };
}

function renderPicker(navigation = makeNavigation(), wrap: (children: ReactNode) => ReactNode = (children) => children) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  function Page() {
    return (
      <QueryClientProvider client={queryClient}>
        <NavigationProvider value={navigation}>
          {wrap(<TemplateCreateAgentPage />)}
        </NavigationProvider>
      </QueryClientProvider>
    );
  }
  return { ...renderWithI18n(<Page />), navigation, Page };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockListTemplates.mockResolvedValue(TEMPLATES);
  mockListAgents.mockResolvedValue([]);
});

describe("TemplateCreateAgentPage role picker", () => {
  it("offers explicit creation when there are no active instances", async () => {
    renderPicker();
    const card = await screen.findByRole("listitem", { name: "Product Analyst" });
    expect(await within(card).findByText("No agents created from this template.")).toBeVisible();
    expect(within(card).getByRole("link", { name: "Create agent" })).toHaveAttribute(
      "href", "/acme/agents/new/template?template=product-analyst",
    );
    expect(mockListAgents).toHaveBeenCalledWith({ workspace_id: "ws-1", include_archived: true });
    expect(mockCreateAgent).not.toHaveBeenCalled();
  });

  it("opens a renamed instance and keeps creating another an explicit choice", async () => {
    mockListAgents.mockResolvedValue([
      agent("payments", "Payments analyst"),
      agent("archived", "Retired analyst", { archived_at: "2026-09-24" }),
      agent("custom", "Product Analyst", { template_key: undefined }),
    ]);
    const { navigation } = renderPicker(makeNavigation({ searchParams: new URLSearchParams("squad=squad-7") }));
    const card = await screen.findByRole("listitem", { name: "Product Analyst" });
    const link = await within(card).findByRole("link", { name: "Open Payments analyst" });
    expect(link).toHaveAttribute("href", "/acme/agents/payments");
    expect(within(card).getByText("1 existing agent")).toBeVisible();
    expect(within(card).queryByText("Retired analyst")).not.toBeInTheDocument();
    expect(within(card).getByRole("link", { name: "Create another" })).toHaveAttribute(
      "href", "/acme/agents/new/template?squad=squad-7&template=product-analyst",
    );
    link.focus();
    await userEvent.setup().keyboard("{Enter}");
    expect(navigation.push).toHaveBeenCalledExactlyOnceWith("/acme/agents/payments");
    expect(mockCreateAgent).not.toHaveBeenCalled();
    expect(mockAddSquadMember).not.toHaveBeenCalled();
  });

  it("links every matching active instance with native new-tab navigation", async () => {
    mockListAgents.mockResolvedValue([
      agent("payments", "Payments analyst"), agent("checkout", "Checkout analyst"),
      agent("other", "Release specialist", { template_key: "release-engineer" }),
    ]);
    const navigation = makeNavigation({ openInNewTab: vi.fn() });
    renderPicker(navigation);
    const card = await screen.findByRole("listitem", { name: "Product Analyst" });
    const payments = await within(card).findByRole("link", { name: "Open Payments analyst" });
    const checkout = within(card).getByRole("link", { name: "Open Checkout analyst" });
    expect(within(card).getByText("2 existing agents")).toBeVisible();
    expect(checkout).toHaveAttribute("href", "/acme/agents/checkout");
    expect(within(card).getAllByRole("link")).toHaveLength(3);
    fireEvent.click(payments, { metaKey: true });
    expect(navigation.openInNewTab).toHaveBeenCalledWith("/acme/agents/payments", "Payments analyst");
    expect(navigation.push).not.toHaveBeenCalled();
    expect(mockCreateAgent).not.toHaveBeenCalled();
  });

  it("does not report zero instances while they are loading", async () => {
    mockListAgents.mockReturnValue(new Promise<Agent[]>(() => {}));
    renderPicker();
    await screen.findByRole("listitem", { name: "Product Analyst" });
    expect(screen.getByRole("status")).toHaveTextContent("Loading existing agents...");
    expect(screen.queryByText("No agents created from this template.")).not.toBeInTheDocument();
  });

  it("retries an instance load failure without claiming there are no agents", async () => {
    mockListAgents.mockRejectedValueOnce(new Error("offline")).mockResolvedValue([agent("payments", "Payments analyst")]);
    renderPicker();
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not load existing agents.");
    expect(screen.queryByText("No agents created from this template.")).not.toBeInTheDocument();
    await userEvent.setup().click(within(alert).getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("link", { name: "Open Payments analyst" })).toBeVisible();
    expect(mockCreateAgent).not.toHaveBeenCalled();
  });

  it("lists the roles the server ships, with their autonomy levels", async () => {
    renderPicker();

    expect(
      await screen.findByRole("listitem", { name: "Product Analyst" }),
    ).toBeTruthy();
    expect(screen.getByRole("listitem", { name: "Release Engineer" })).toBeTruthy();
    // The badge is the level's label, not the raw wire value.
    expect(screen.getByText("Observer")).toBeTruthy();
    expect(screen.getByText("Operator")).toBeTruthy();
  });

  it("renders a role whose autonomy level this client does not know", async () => {
    renderPicker();

    // The card must be pickable. Only the badge is dropped — showing an
    // untranslated level, or claiming a limit we cannot describe, would be worse.
    expect(
      await screen.findByRole("listitem", { name: "Future Role" }),
    ).toBeTruthy();
    expect(screen.queryByText("supervisor")).toBeNull();
  });

  it("navigates to the same route with the picked role in the query", async () => {
    const { navigation } = renderPicker();
    const user = userEvent.setup();

    await user.click(
      within(await screen.findByRole("listitem", { name: "Release Engineer" })).getByRole("link", { name: "Create agent" }),
    );

    // Same route, `?template=` added: going back to the role list is the route
    // without the param, which is why the role is not a path segment.
    await waitFor(() =>
      expect(navigation.push).toHaveBeenCalledWith(
        "/acme/agents/new/template?template=release-engineer",
      ),
    );
  });

  it("keeps the squad context across the hop", async () => {
    const { navigation } = renderPicker(makeNavigation({ searchParams: new URLSearchParams("squad=squad-7") }));
    const user = userEvent.setup();

    await user.click(
      within(await screen.findByRole("listitem", { name: "Product Analyst" })).getByRole("link", { name: "Create agent" }),
    );

    await waitFor(() =>
      expect(navigation.push).toHaveBeenCalledWith(
        "/acme/agents/new/template?squad=squad-7&template=product-analyst",
      ),
    );
  });

  it("says so when the server ships no templates", async () => {
    mockListTemplates.mockResolvedValue([]);
    renderPicker();

    expect(
      await screen.findByText(enAgents.role_templates.empty),
    ).toBeTruthy();
  });

  it("reports a failed load instead of rendering an empty grid", async () => {
    mockListTemplates.mockRejectedValue(new Error("offline"));
    renderPicker();

    expect(
      await screen.findByText(enAgents.role_templates.load_failed),
    ).toBeTruthy();
    mockListTemplates.mockResolvedValue(TEMPLATES);
    await userEvent.setup().click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("listitem", { name: "Product Analyst" })).toBeVisible();
  });

  it("returns from configuration to the picker without losing the squad context", async () => {
    const { navigation } = renderPicker(makeNavigation({ searchParams: new URLSearchParams("squad=squad-7&template=product-analyst") }));
    await screen.findByText("Product Analyst");
    await userEvent.setup().click(screen.getByRole("button", { name: "Go back" }));
    expect(navigation.replace).toHaveBeenCalledWith("/acme/agents/new/template?squad=squad-7");
    expect(mockCreateAgent).not.toHaveBeenCalled();
    expect(mockAddSquadMember).not.toHaveBeenCalled();
  });

  it("restores the picker scroll after instance data loads, including an explicit zero", async () => {
    const viewState = new Map<string, string>();
    const adapter = {
      get: () => ({ top: 900, height: 1400 }),
      getViewState: (key: string) => viewState.get(key),
      setViewState: (key: string, value: string | undefined) => {
        if (value === undefined) viewState.delete(key);
        else viewState.set(key, value);
      },
    };
    const wrap = (children: ReactNode) => <ScrollRestorationProvider adapter={adapter}>{children}</ScrollRestorationProvider>;
    const first = renderPicker(makeNavigation(), wrap);
    const card = await screen.findByRole("listitem", { name: "Product Analyst" });
    await within(card).findByText("No agents created from this template.");
    const gallery = screen.getByRole("main");
    expect(gallery).toHaveAttribute("data-tab-scroll-root", "agent-role-templates");
    gallery.scrollTop = 320;
    await userEvent.setup().click(within(card).getByRole("link", { name: "Create agent" }));
    first.unmount();

    let resolveAgents!: (agents: Agent[]) => void;
    mockListAgents.mockReturnValue(new Promise<Agent[]>((resolve) => { resolveAgents = resolve; }));
    const second = renderPicker(makeNavigation(), wrap);
    await screen.findByRole("listitem", { name: "Product Analyst" });
    expect(screen.getByRole("main").scrollTop).toBe(0);
    resolveAgents([agent("payments", "Payments analyst")]);
    const existing = await screen.findByRole("link", { name: "Open Payments analyst" });
    await waitFor(() => expect(screen.getByRole("main").scrollTop).toBe(320));
    screen.getByRole("main").scrollTop = 0;
    await userEvent.setup().click(existing);
    second.unmount();

    mockListAgents.mockResolvedValue([]);
    renderPicker(makeNavigation(), wrap);
    await screen.findAllByText("No agents created from this template.");
    expect(screen.getByRole("main").scrollTop).toBe(0);
  });
});
