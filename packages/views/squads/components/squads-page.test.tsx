import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import type { Squad, SquadTemplate } from "@multica/core/types";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { SquadsPage } from "./squads-page";

const mocks = vi.hoisted(() => ({
  squads: [] as Squad[],
  squadsError: false,
  refetchSquads: vi.fn(),
  templates: [] as SquadTemplate[],
  templatesError: false,
  refetchTemplates: vi.fn(),
  openModal: vi.fn(),
  viewState: {
    scope: "all", sortField: "name", sortDirection: "asc", hiddenColumns: [],
    filters: { leaders: [] as string[], creators: [] as string[] },
    setScope: vi.fn(), toggleSort: vi.fn(), setSortField: vi.fn(), setSortDirection: vi.fn(),
    toggleColumn: vi.fn(), toggleFilter: vi.fn(), clearFilters: vi.fn(),
  },
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { queryKey: unknown[] }) => options.queryKey[0] === "squads"
    ? { data: mocks.squads, isLoading: false, error: mocks.squadsError ? new Error("failed") : null, refetch: mocks.refetchSquads }
    : { data: [], isLoading: false, refetch: vi.fn() },
  useQueryClient: () => ({ invalidateQueries: vi.fn() }),
  useMutation: () => ({ isPending: false, mutate: vi.fn() }),
}));
vi.mock("@multica/core/workspace/queries", () => ({
  squadListOptions: () => ({ queryKey: ["squads"] }),
  agentListOptions: () => ({ queryKey: ["agents"] }),
  memberListOptions: () => ({ queryKey: ["members"] }),
  workspaceKeys: { squads: (wsId: string) => ["squads", wsId] },
}));
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "user-1" } };
  return { useAuthStore: Object.assign((selector: (value: typeof state) => unknown) => selector(state), { getState: () => state }) };
});
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return {
    ...actual,
    useCurrentWorkspace: () => ({ id: "ws-1", slug: "acme" }),
    useWorkspacePaths: () => actual.paths.workspace("acme"),
  };
});
vi.mock("@multica/core/modals", () => {
  const state = { open: mocks.openModal };
  return { useModalStore: Object.assign((selector: (value: typeof state) => unknown) => selector(state), { getState: () => state }) };
});
vi.mock("@multica/core/squads/stores", () => ({
  useSquadsViewStore: Object.assign((selector: (state: typeof mocks.viewState) => unknown) => selector(mocks.viewState), { getState: () => mocks.viewState }),
  SQUAD_SCOPES: ["mine", "all"], SQUAD_DEFAULT_HIDDEN_COLUMNS: [],
}));
vi.mock("../../agents/create/use-role-templates", () => ({
  useSquadTemplates: () => ({ data: mocks.templates, isLoading: false, isPending: false, isError: mocks.templatesError, refetch: mocks.refetchTemplates }),
}));
vi.mock("../../projects/components/use-squad-for-project-dialog", () => ({
  UseSquadForProjectDialog: ({ template }: { template: SquadTemplate }) => <div role="dialog" aria-label={template.title} />,
}));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => null }));
vi.mock("@multica/ui/components/common/actor-avatar", () => ({ ActorAvatar: () => null }));

const TEMPLATE: SquadTemplate = {
  key: "feature-delivery", version: 1, name: "Feature delivery", title: "Feature delivery",
  description: "Design, build, review, and verify a feature.", avatar_emoji: "", instructions: "Coordinate the work.",
  leader: { template_key: "tech-lead", title: "Tech lead", name: "Lead", role: "Coordinate", autonomy_level: "coordinator", avatar_emoji: "" },
  members: [],
};
const SQUAD: Squad = {
  id: "squad-1", workspace_id: "ws-1", name: "Payments delivery", description: "Our own workflow",
  instructions: "Keep our conventions.", avatar_url: null, leader_id: "lead-1", creator_id: "user-1",
  created_at: "2026-09-01T00:00:00Z", updated_at: "2026-09-01T00:00:00Z", archived_at: null, archived_by: null,
};

function adapter(): NavigationAdapter {
  return { push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/acme/squads", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path };
}
function renderPage(navigation = adapter()) {
  return renderWithI18n(<NavigationProvider value={navigation}><SquadsPage /></NavigationProvider>);
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.squads = [];
  mocks.squadsError = false;
  mocks.templates = [TEMPLATE];
  mocks.templatesError = false;
  mocks.viewState.filters = { leaders: [], creators: [] };
});

describe("SquadsPage built-in catalog", () => {
  it("keeps templates collapsed above the empty list and preserves template and custom creation", () => {
    renderPage();
    const catalog = screen.getByRole("region", { name: "Built-in squads" });
    const toggle = within(catalog).getByRole("button", { name: "Built-in squads 1" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(within(catalog).queryByRole("button", { name: "Use for project" })).not.toBeInTheDocument();
    fireEvent.click(toggle);
    expect(within(catalog).getByText(TEMPLATE.description)).toBeInTheDocument();
    expect(screen.getByText("No squads yet. Create one to get started.")).toBeInTheDocument();
    expect(within(screen.getByRole("heading", { level: 1 }).parentElement!).queryByText("1")).not.toBeInTheDocument();
    fireEvent.click(within(catalog).getByRole("button", { name: "Use for project" }));
    expect(screen.getByRole("dialog", { name: TEMPLATE.title })).toBeInTheDocument();
    expect(mocks.openModal).not.toHaveBeenCalled();
    fireEvent.click(screen.getAllByRole("button", { name: "New Squad" })[0]!);
    expect(mocks.openModal).toHaveBeenCalledWith("create-squad");
  });

  it("links a renamed template instance by metadata and keeps the catalog above filters", () => {
    mocks.squads = [
      { ...SQUAD, id: "same-name", name: TEMPLATE.name },
      { ...SQUAD, template_key: TEMPLATE.key },
    ];
    const navigation = adapter();
    renderPage(navigation);
    const catalog = screen.getByRole("region", { name: "Built-in squads" });
    fireEvent.click(within(catalog).getByRole("button", { name: /^Built-in squads/ }));
    fireEvent.click(within(catalog).getByRole("button", { name: "Open squad" }));
    expect(navigation.push).toHaveBeenCalledWith("/acme/squads/squad-1");
    const filter = screen.getByRole("button", { name: "Filter" });
    expect(catalog.compareDocumentPosition(filter) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.getAllByText("Our own workflow")).toHaveLength(2);
  });

  it("does not treat a matching name as a template instance", () => {
    mocks.squads = [{ ...SQUAD, name: TEMPLATE.name }];
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /^Built-in squads/ }));
    expect(within(screen.getByRole("region", { name: "Built-in squads" })).queryByRole("button", { name: "Open squad" })).not.toBeInTheDocument();
  });

  it("offers retry when the catalog fails without hiding saved squads", () => {
    mocks.templatesError = true;
    mocks.squads = [SQUAD];
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /^Built-in squads/ }));
    fireEvent.click(within(screen.getByRole("region", { name: "Built-in squads" })).getByRole("button", { name: "Retry" }));
    expect(mocks.refetchTemplates).toHaveBeenCalledOnce();
    expect(screen.getByText(SQUAD.name)).toBeInTheDocument();
  });

  it("distinguishes a failed instance query from an empty workspace and retains the catalog", () => {
    mocks.squadsError = true;
    renderPage();
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load squads.");
    expect(screen.queryByText("No squads yet. Create one to get started.")).not.toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Built-in squads" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(mocks.refetchSquads).toHaveBeenCalledOnce();
  });
});
