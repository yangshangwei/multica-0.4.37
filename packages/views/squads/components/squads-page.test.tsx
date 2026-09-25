import { useState } from "react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, screen, within } from "@testing-library/react";
import type { MemberWithUser, Squad, SquadTemplate } from "@multica/core/types";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { SquadsPage } from "./squads-page";

const mocks = vi.hoisted(() => ({
  squads: [] as Squad[],
  members: [] as MemberWithUser[],
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
    : { data: options.queryKey[0] === "members" ? mocks.members : [], isLoading: false, refetch: vi.fn() },
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
const CURRENT_MEMBER: MemberWithUser = {
  id: "membership-1", workspace_id: "ws-1", user_id: "user-1", role: "member",
  created_at: "2026-09-01T00:00:00Z", name: "Ada", email: "ada@example.com", avatar_url: null,
};

function adapter(): NavigationAdapter {
  return { push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/acme/squads", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path };
}
function renderPage(navigation = adapter()) {
  function Page() {
    const [searchParams, setSearchParams] = useState(navigation.searchParams);
    return (
      <NavigationProvider value={{
        ...navigation,
        searchParams,
        replace: (href) => {
          navigation.replace(href);
          setSearchParams(new URL(href, "http://localhost").searchParams);
        },
      }}>
        <SquadsPage />
      </NavigationProvider>
    );
  }
  return renderWithI18n(<Page />);
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.squads = [];
  mocks.members = [CURRENT_MEMBER];
  mocks.squadsError = false;
  mocks.templates = [TEMPLATE];
  mocks.templatesError = false;
  mocks.viewState.scope = "all";
  mocks.viewState.filters = { leaders: [], creators: [] };
});

describe("SquadsPage discovery and management", () => {
  it("defaults to saved squads and switches to a separate template view in the URL", async () => {
    const user = userEvent.setup();
    mocks.squads = [SQUAD];
    const navigation = adapter();
    renderPage(navigation);
    expect(screen.getByRole("tab", { name: "Workspace squads" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("link", { name: SQUAD.name })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Apply to project" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Squad templates" }));
    expect(navigation.replace).toHaveBeenCalledWith("/acme/squads?view=templates");
    expect(screen.getByRole("button", { name: "Apply to project" })).toBeVisible();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: "Workspace squads" }));
    expect(navigation.replace).toHaveBeenLastCalledWith("/acme/squads");
    expect(screen.getByRole("link", { name: SQUAD.name })).toBeVisible();
  });

  it("opens bookmarked templates and preserves unrelated query state when switching", async () => {
    const user = userEvent.setup();
    const navigation = { ...adapter(), searchParams: new URLSearchParams("view=templates&context=project") };
    renderPage(navigation);
    expect(screen.getByRole("tab", { name: "Squad templates" })).toHaveAttribute("aria-selected", "true");
    await user.click(screen.getByRole("tab", { name: "Workspace squads" }));
    expect(navigation.replace).toHaveBeenCalledWith("/acme/squads?context=project");
  });

  it("preserves template and custom creation in one page action", async () => {
    const user = userEvent.setup();
    mocks.squads = [SQUAD];
    renderPage();
    screen.getByRole("button", { name: "New Squad" }).focus();
    await user.keyboard("{ArrowDown}");
    await user.click(screen.getByRole("menuitem", { name: "Create from template" }));
    expect(mocks.openModal).toHaveBeenCalledWith("staff-squad-template");
    screen.getByRole("button", { name: "New Squad" }).focus();
    await user.keyboard("{ArrowDown}");
    await user.click(screen.getByRole("menuitem", { name: "Create custom squad" }));
    expect(mocks.openModal).toHaveBeenCalledWith("create-squad");
  });

  it("provides a native squad link and only navigates once for keyboard and pointer activation", async () => {
    const user = userEvent.setup();
    mocks.squads = [SQUAD];
    const navigation = adapter();
    renderPage(navigation);
    const link = screen.getByRole("link", { name: SQUAD.name });
    expect(link).toHaveAttribute("href", "/acme/squads/squad-1");
    link.focus();
    await user.keyboard("{Enter}");
    expect(navigation.push).toHaveBeenCalledExactlyOnceWith("/acme/squads/squad-1");
    vi.mocked(navigation.push).mockClear();
    await user.click(link);
    expect(navigation.push).toHaveBeenCalledExactlyOnceWith("/acme/squads/squad-1");
  });

  it("preserves native modifier-link navigation without invoking the row fallback", () => {
    mocks.squads = [SQUAD];
    const navigation = adapter();
    const open = vi.spyOn(window, "open").mockImplementation(() => null);
    renderPage(navigation);
    fireEvent(screen.getByRole("link", { name: SQUAD.name }), new MouseEvent("auxclick", { button: 1, bubbles: true, cancelable: true }));
    expect(navigation.push).not.toHaveBeenCalled();
    expect(open).not.toHaveBeenCalled();
    open.mockRestore();
  });

  it("limits regular members' row actions to squads they created", () => {
    const otherSquad = { ...SQUAD, id: "squad-2", name: "Release delivery", creator_id: "user-2" };
    mocks.squads = [SQUAD, otherSquad];
    renderPage();

    expect(within(screen.getByRole("row", { name: new RegExp(SQUAD.name) })).getByRole("button", { name: "Squad actions" })).toBeInTheDocument();
    expect(within(screen.getByRole("row", { name: new RegExp(otherSquad.name) })).queryByRole("button", { name: "Squad actions" })).not.toBeInTheDocument();
  });

  it.each(["admin", "owner"] as const)("allows workspace %ss to manage squads created by another member", (role) => {
    mocks.members = [{ ...CURRENT_MEMBER, role }];
    mocks.squads = [{ ...SQUAD, creator_id: "user-2" }];
    renderPage();

    expect(screen.getByRole("button", { name: "Squad actions" })).toBeInTheDocument();
  });

  it.each(["pointer", "keyboard"] as const)("opens nested row actions and the archive confirmation by %s without navigating", async (input) => {
    const user = userEvent.setup();
    const navigation = adapter();
    mocks.squads = [SQUAD];
    renderPage(navigation);

    const actions = screen.getByRole("button", { name: "Squad actions" });
    if (input === "pointer") {
      await user.click(actions);
    } else {
      actions.focus();
      await user.keyboard("{ArrowDown}");
    }
    const archive = await screen.findByRole("menuitem", { name: "Archive" });
    expect(archive).toBeVisible();
    expect(navigation.push).not.toHaveBeenCalled();

    if (input === "pointer") {
      await user.click(archive);
    } else {
      archive.focus();
      await user.keyboard("{Enter}");
    }
    const dialog = screen.getByRole("dialog", { name: "Archive this squad?" });
    expect(dialog).toHaveTextContent(SQUAD.name);
    expect(within(dialog).getByRole("button", { name: "Archive" })).toBeVisible();
    expect(navigation.push).not.toHaveBeenCalled();

    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog", { name: "Archive this squad?" })).not.toBeInTheDocument();
    expect(navigation.push).not.toHaveBeenCalled();
  });

  it("exposes scope selection and allows clearing filters using the keyboard", async () => {
    const user = userEvent.setup();
    mocks.squads = [SQUAD];
    mocks.viewState.filters = { leaders: [SQUAD.leader_id], creators: [] };
    renderPage();
    expect(screen.getByRole("button", { name: /^All/, pressed: true })).toHaveAttribute("aria-pressed", "true");
    const mine = screen.getByRole("button", { name: /^Created by me/ });
    expect(mine).toHaveAttribute("aria-pressed", "false");
    await user.click(mine);
    expect(mocks.viewState.setScope).toHaveBeenCalledWith("mine");
    const clear = screen.getByRole("button", { name: "Clear filters" });
    expect(clear.tagName).toBe("BUTTON");
    expect(clear.parentElement?.closest("button")).toBeNull();
    clear.focus();
    await user.keyboard("{Enter}");
    expect(mocks.viewState.clearFilters).toHaveBeenCalledOnce();
    expect(screen.getByRole("button", { name: "Display" })).toBeInTheDocument();
  });

  it("keeps empty-state creation available and never counts templates as saved squads", () => {
    renderPage();
    expect(screen.getByText("No squads yet. Create one to get started.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create from template" })).toBeInTheDocument();
    expect(within(screen.getByRole("heading", { level: 1 }).parentElement!).queryByText("1")).not.toBeInTheDocument();
  });

  it("retains retry and template navigation when the saved squad query fails", async () => {
    const user = userEvent.setup();
    mocks.squadsError = true;
    renderPage();
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load squads.");
    expect(screen.queryByText("No squads yet. Create one to get started.")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(mocks.refetchSquads).toHaveBeenCalledOnce();
    await user.click(screen.getByRole("tab", { name: "Squad templates" }));
    expect(screen.getByRole("button", { name: "Apply to project" })).toBeInTheDocument();
  });
});
