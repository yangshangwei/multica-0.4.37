import React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiError, setApiInstance } from "@multica/core/api";
import type { ApiClient } from "@multica/core/api/client";
import { useProjectAccessStore } from "@multica/core/projects";
import { projectKeys } from "@multica/core/projects/queries";
import type { Project } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { ProjectsPage } from "./projects-page";

const mocks = vi.hoisted(() => ({
  realQueries: false,
  projects: [] as Project[] | undefined,
  projectError: null as Error | null,
  projectFetching: false,
  refetchProjects: vi.fn(),
  members: [] as Array<{ user_id: string; name: string; role: string }>,
  agents: [] as Array<{ id: string; name: string; archived_at: string | null }>,
  pins: [] as Array<{ item_type: string; item_id: string }>,
  updateProject: vi.fn(),
  deleteProject: vi.fn(),
  createPin: vi.fn(),
  deletePin: vi.fn(),
  openModal: vi.fn(),
  projectViewState: {
    viewMode: "compact",
    sortField: "name",
    sortDirection: "asc",
    hiddenColumns: [] as string[],
    filters: { statuses: [] as string[], priorities: [] as string[], leads: [] as string[] },
    setViewMode: vi.fn(),
    toggleSort: vi.fn(),
    setSortField: vi.fn(),
    setSortDirection: vi.fn(),
    toggleColumn: vi.fn(),
    toggleFilter: vi.fn(),
    clearFilters: vi.fn(),
  },
}));

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return {
  ...actual,
  useQuery: (options: { queryKey?: readonly unknown[] }) => {
    if (mocks.realQueries) return actual.useQuery(options as Parameters<typeof actual.useQuery>[0]);
    const key = options.queryKey?.[0];
    if (key === "projects") {
      return {
        data: mocks.projects,
        isLoading: false,
        error: mocks.projectError,
        isError: !!mocks.projectError,
        isFetching: mocks.projectFetching,
        refetch: mocks.refetchProjects,
      };
    }
    if (key === "members") {
      return { data: mocks.members, isLoading: false };
    }
    if (key === "agents") {
      return { data: mocks.agents, isLoading: false };
    }
    if (key === "pins") {
      return { data: mocks.pins, isLoading: false };
    }
    return { data: [], isLoading: false };
  },
}; });

vi.mock("@multica/core/projects", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/projects")>();
  return {
  ...actual,
  projectListOptions: (wsId: string) => mocks.realQueries ? actual.projectListOptions(wsId) : { queryKey: ["projects"] },
  useUpdateProject: () => ({ mutate: mocks.updateProject }),
  useDeleteProject: () => ({ mutate: mocks.deleteProject }),
  useProjectViewStore: (selector: (state: unknown) => unknown) =>
    selector(mocks.projectViewState),
}; });

vi.mock("@multica/core/pins", () => ({
  pinListOptions: () => ({ queryKey: ["pins"] }),
  useCreatePin: () => ({ mutate: mocks.createPin }),
  useDeletePin: () => ({ mutate: mocks.deletePin }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    projectDetail: (id: string) => `/test-workspace/projects/${id}`,
    memberDetail: (id: string) => `/test-workspace/members/${id}`,
    agentDetail: (id: string) => `/test-workspace/agents/${id}`,
  }),
}));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ user: { id: "user-1" } }),
}));

vi.mock("@multica/core/workspace/queries", () => ({
  memberListOptions: () => ({ queryKey: ["members"] }),
  agentListOptions: () => ({ queryKey: ["agents"] }),
}));

vi.mock("@multica/core/workspace/hooks", () => ({
  useActorName: () => ({
    getActorName: () => "Test Lead",
    getActorInitials: () => "TL",
    getActorAvatarUrl: () => null,
  }),
}));

vi.mock("@multica/core/modals", () => ({
  useModalStore: {
    getState: () => ({ open: mocks.openModal }),
  },
}));

vi.mock("@multica/ui/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
  DropdownMenuTrigger: ({ render }: { render: React.ReactNode }) => (
    <>{render}</>
  ),
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DropdownMenuGroup: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DropdownMenuLabel: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DropdownMenuItem: ({
    children,
    onClick,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
  }) => (
    <button type="button" onClick={onClick}>
      {children}
    </button>
  ),
  DropdownMenuCheckboxItem: ({
    children,
    onCheckedChange,
  }: {
    children: React.ReactNode;
    onCheckedChange?: () => void;
  }) => (
    <button type="button" onClick={onCheckedChange}>
      {children}
    </button>
  ),
  DropdownMenuRadioGroup: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DropdownMenuRadioItem: ({
    children,
    onClick,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
  }) => (
    <button type="button" onClick={onClick}>
      {children}
    </button>
  ),
  DropdownMenuSeparator: () => <hr />,
  DropdownMenuSub: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
  DropdownMenuSubContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DropdownMenuSubTrigger: ({ children }: { children: React.ReactNode }) => (
    <button type="button">{children}</button>
  ),
}));

vi.mock("@multica/ui/components/ui/popover", () => ({
  Popover: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  PopoverTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  PopoverContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
}));

vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ render }: { render: React.ReactNode }) => <>{render}</>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => (
    <div role="tooltip">{children}</div>
  ),
}));

const PROJECT: Project = {
  id: "project-1",
  workspace_id: "workspace-1",
  title: "Launch Plan",
  description: null,
  icon: null,
  status: "in_progress",
  priority: "high",
  lead_type: null,
  lead_id: null,
  start_date: null,
  due_date: null,
  created_at: "2026-06-01T00:00:00Z",
  updated_at: "2026-06-01T00:00:00Z",
  issue_count: 3,
  done_count: 1,
  resource_count: 0,
};

function makeAdapter(
  overrides: Partial<NavigationAdapter> = {},
): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/test-workspace/projects",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (p) => p,
    ...overrides,
  };
}

function renderProjects(adapter = makeAdapter()) {
  const view = renderWithI18n(
    <NavigationProvider value={adapter}>
      <ProjectsPage />
    </NavigationProvider>,
  );
  return { ...view, adapter };
}

function projectRow() {
  const row = screen.getByText(PROJECT.title).closest('[role="row"]');
  if (!row) throw new Error("project row not found");
  return row as HTMLElement;
}

beforeEach(() => {
  mocks.realQueries = false;
  useProjectAccessStore.setState({ denied: {}, epochs: {}, deleted: {} });
  mocks.projects = [PROJECT];
  mocks.projectError = null;
  mocks.projectFetching = false;
  mocks.refetchProjects.mockReset();
  mocks.members = [
    { user_id: "user-1", name: "User One", role: "admin" },
  ];
  mocks.agents = [];
  mocks.pins = [];
  mocks.updateProject.mockClear();
  mocks.deleteProject.mockClear();
  mocks.createPin.mockClear();
  mocks.deletePin.mockClear();
  mocks.openModal.mockClear();
  mocks.projectViewState.viewMode = "compact";
  mocks.projectViewState.sortField = "name";
  mocks.projectViewState.sortDirection = "asc";
  mocks.projectViewState.hiddenColumns = [];
  mocks.projectViewState.filters = { statuses: [], priorities: [], leads: [] };
});

const clients: QueryClient[] = [];
afterEach(() => { for (const client of clients.splice(0)) client.clear(); });

function renderQueryProjects(listProjects: ReturnType<typeof vi.fn>, cached?: Project[]) {
  mocks.realQueries = true;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  clients.push(client);
  setApiInstance({ listProjects } as unknown as ApiClient);
  if (cached) client.setQueryData(projectKeys.list("workspace-1"), { projects: cached });
  client.setQueryData(["members"], mocks.members);
  client.setQueryData(["agents"], mocks.agents);
  client.setQueryData(["pins"], mocks.pins);
  const adapter = makeAdapter();
  const view = renderWithI18n(
    <QueryClientProvider client={client}>
      <NavigationProvider value={adapter}>
        <ProjectsPage />
      </NavigationProvider>
    </QueryClientProvider>,
  );
  return { ...view, client, adapter };
}

describe("ProjectsPage compact row navigation", () => {
  it("guides a new workspace into its first project", async () => {
    const user = userEvent.setup();
    mocks.projects = [];
    renderProjects();
    expect(screen.getByText("Choose a repository and an execution squad to get your first issue moving.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Create your first project" }));
    expect(mocks.openModal).toHaveBeenCalledWith("create-project");
  });

  it("provides a keyboard title link after the row selection control", async () => {
    const user = userEvent.setup();
    const { adapter } = renderProjects();
    const row = projectRow();
    const link = within(row).getByRole("link", { name: PROJECT.title });
    expect(link).toHaveAttribute("href", "/test-workspace/projects/project-1");

    within(row).getByRole("checkbox", { name: "Select Launch Plan" }).focus();
    await user.tab();
    expect(link).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(adapter.push).toHaveBeenCalledExactlyOnceWith("/test-workspace/projects/project-1");
  });

  it("navigates once for each title click and desktop modifier click", async () => {
    const user = userEvent.setup();
    const push = vi.fn();
    const openInNewTab = vi.fn();
    renderProjects(makeAdapter({ push, openInNewTab }));
    const link = screen.getByRole("link", { name: PROJECT.title });
    await user.click(link);
    expect(push).toHaveBeenCalledExactlyOnceWith("/test-workspace/projects/project-1");

    fireEvent.click(link, { metaKey: true });
    fireEvent.click(link, { ctrlKey: true });
    fireEvent(link, new MouseEvent("auxclick", { bubbles: true, button: 1, cancelable: true }));
    expect(openInNewTab).toHaveBeenCalledTimes(3);
    for (const nth of [1, 2, 3]) {
      expect(openInNewTab).toHaveBeenNthCalledWith(nth, "/test-workspace/projects/project-1", PROJECT.title);
    }
    expect(push).toHaveBeenCalledTimes(1);
  });

  it("leaves native web title modifier clicks to the anchor without opening a second tab", () => {
    const { adapter } = renderProjects();
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    const link = screen.getByRole("link", { name: PROJECT.title });
    const click = new MouseEvent("click", { bubbles: true, metaKey: true, cancelable: true });
    const middleClick = new MouseEvent("auxclick", { bubbles: true, button: 1, cancelable: true });
    fireEvent(link, click);
    fireEvent(link, middleClick);
    expect(click.defaultPrevented).toBe(false);
    expect(middleClick.defaultPrevented).toBe(false);
    expect(adapter.push).not.toHaveBeenCalled();
    expect(open).not.toHaveBeenCalled();
    open.mockRestore();
  });

  it("navigates from the row surface", async () => {
    const user = userEvent.setup();
    const push = vi.fn();
    renderProjects(makeAdapter({ push }));

    await user.click(projectRow());

    expect(push).toHaveBeenCalledWith("/test-workspace/projects/project-1");
    expect(push).toHaveBeenCalledTimes(1);
  });

  it("does not navigate when inline controls are clicked", async () => {
    const user = userEvent.setup();
    const push = vi.fn();
    renderProjects(makeAdapter({ push }));
    const row = projectRow();

    await user.click(within(row).getByRole("checkbox", { name: "Select Launch Plan" }));
    await user.click(within(row).getByRole("button", { name: "Project actions" }));
    await user.click(within(row).getAllByRole("button", { name: "In Progress" })[0]!);
    await user.click(within(row).getAllByRole("button", { name: "High" })[0]!);
    await user.click(within(row).getByRole("button", { name: "—" }));

    expect(push).not.toHaveBeenCalled();
  });

  it("uses the rowLink modifier and middle-click paths when openInNewTab is available", () => {
    const push = vi.fn();
    const openInNewTab = vi.fn();
    renderProjects(makeAdapter({ push, openInNewTab }));
    const row = projectRow();

    fireEvent.click(row, { metaKey: true });
    fireEvent.click(row, { ctrlKey: true });
    const middleClick = new MouseEvent("auxclick", {
      bubbles: true,
      button: 1,
      cancelable: true,
    });
    row.dispatchEvent(middleClick);

    expect(middleClick.defaultPrevented).toBe(true);
    expect(openInNewTab).toHaveBeenCalledTimes(3);
    expect(openInNewTab).toHaveBeenNthCalledWith(1, "/test-workspace/projects/project-1", "Launch Plan");
    expect(openInNewTab).toHaveBeenNthCalledWith(2, "/test-workspace/projects/project-1", "Launch Plan");
    expect(openInNewTab).toHaveBeenNthCalledWith(3, "/test-workspace/projects/project-1", "Launch Plan");
    expect(push).not.toHaveBeenCalled();
  });

  // Web (no adapter): the row is a <div>, so nothing native catches a
  // modifier or middle click — rowLink opens the browser tab itself instead
  // of navigating in place (MUL-5456).
  it("has a single rowLink path for modifier and middle clicks without openInNewTab", () => {
    const push = vi.fn();
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    renderProjects(makeAdapter({ push }));
    const row = projectRow();

    fireEvent.click(row, { metaKey: true });
    fireEvent.click(row, { ctrlKey: true });
    const middleClick = new MouseEvent("auxclick", {
      bubbles: true,
      button: 1,
      cancelable: true,
    });
    row.dispatchEvent(middleClick);

    expect(middleClick.defaultPrevented).toBe(true);
    expect(open).toHaveBeenCalledTimes(3);
    for (const nth of [1, 2, 3]) {
      expect(open).toHaveBeenNthCalledWith(
        nth,
        "/test-workspace/projects/project-1",
        "_blank",
        "noopener,noreferrer",
      );
    }
    expect(push).not.toHaveBeenCalled();
    open.mockRestore();
  });
});

describe("ProjectsPage selection and recovery", () => {
  it("offers one named checkbox per selection with keyboard mixed-state behavior", async () => {
    const user = userEvent.setup();
    mocks.projects = [PROJECT, { ...PROJECT, id: "project-2", title: "Release Checklist" }];
    const { adapter } = renderProjects();
    const all = screen.getByRole("checkbox", { name: "Select all visible projects" });
    const first = screen.getByRole("checkbox", { name: "Select Launch Plan" });
    const second = screen.getByRole("checkbox", { name: "Select Release Checklist" });
    expect(screen.getAllByRole("checkbox")).toHaveLength(3);
    expect(first.closest("button")).toBeNull();
    expect(first.tabIndex).toBe(0);
    all.focus();
    await user.keyboard(" ");
    expect(all).toBeChecked();
    expect(first).toBeChecked();
    expect(second).toBeChecked();
    first.focus();
    await user.keyboard(" ");
    expect(first).not.toBeChecked();
    expect(all).toBePartiallyChecked();
    expect(first).toHaveFocus();
    expect(adapter.push).not.toHaveBeenCalled();
  });

  it("shows retry for an initial failure instead of inviting creation in an empty list", async () => {
    mocks.projects = undefined;
    mocks.projectError = new Error("Offline");
    renderProjects();
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load projects");
    expect(screen.queryByText("No projects yet")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Create your first project" })).not.toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "Retry" }));
    expect(mocks.refetchProjects).toHaveBeenCalledOnce();
  });

  it("retains cached rows, filters, and selection after a failed refresh", async () => {
    const user = userEvent.setup();
    const view = renderProjects();
    const row = projectRow();
    await user.click(within(row).getByRole("checkbox", { name: "Select Launch Plan" }));
    await user.type(screen.getByRole("textbox", { name: "Search projects..." }), "Launch");
    mocks.projectError = new Error("Offline");
    view.rerender(<NavigationProvider value={view.adapter}><ProjectsPage /></NavigationProvider>);
    expect(screen.getByRole("alert")).toHaveTextContent("Could not refresh projects");
    expect(projectRow()).toBe(row);
    expect(screen.getByRole("checkbox", { name: "Select Launch Plan" })).toBeChecked();
    expect(screen.getByRole("textbox", { name: "Search projects..." })).toHaveValue("Launch");
    expect(screen.queryByText("No projects yet")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(mocks.refetchProjects).toHaveBeenCalledOnce();
  });
});

describe("ProjectsPage real query recovery", () => {
  it("recovers from a cold 500 through Retry and the production query projection", async () => {
    const listProjects = vi.fn()
      .mockRejectedValueOnce(new ApiError("Unavailable", 500, "Internal Server Error"))
      .mockResolvedValue({ projects: [PROJECT] });
    const { adapter } = renderQueryProjects(listProjects);
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not load projects");
    expect(screen.queryByText("No projects yet")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Create your first project" })).not.toBeInTheDocument();

    await userEvent.setup().click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("link", { name: PROJECT.title })).toHaveAttribute("href", "/test-workspace/projects/project-1");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(listProjects).toHaveBeenCalledTimes(2);
    expect(listProjects).toHaveBeenLastCalledWith(undefined, expect.objectContaining({ workspaceId: "workspace-1", signal: expect.any(AbortSignal) }));
    expect(adapter.push).not.toHaveBeenCalled();
    expect(adapter.replace).not.toHaveBeenCalled();
  });

  it("keeps filtered rows and keyboard selection mounted through cached 500 and a delayed retry", async () => {
    const projects = [PROJECT, { ...PROJECT, id: "project-2", title: "Release Checklist", status: "completed" as const }];
    mocks.projectViewState.filters.statuses = [PROJECT.status];
    const response = { projects };
    let finishRetry!: (value: typeof response) => void;
    const retryResponse = new Promise<typeof response>((resolve) => { finishRetry = resolve; });
    const listProjects = vi.fn()
      .mockRejectedValueOnce(new ApiError("Unavailable", 500, "Internal Server Error"))
      .mockReturnValueOnce(retryResponse);
    const { client, adapter } = renderQueryProjects(listProjects, projects);
    const user = userEvent.setup();
    const row = projectRow();
    const selection = screen.getByRole("checkbox", { name: "Select Launch Plan" });
    selection.focus();
    await user.keyboard(" ");
    const search = screen.getByRole("textbox", { name: "Search projects..." });
    await user.type(search, "Launch");
    await act(async () => { await client.refetchQueries({ queryKey: projectKeys.list("workspace-1"), exact: true }); });

    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh projects");
    expect(projectRow()).toBe(row);
    expect(selection).toBeChecked();
    expect(screen.getByRole("textbox", { name: "Search projects..." })).toBe(search);
    expect(search).toHaveValue("Launch");
    expect(search).toHaveFocus();
    expect(screen.queryByRole("link", { name: "Release Checklist" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(screen.getByRole("button", { name: "Retry" })).toBeDisabled();
    expect(projectRow()).toBe(row);
    expect(selection).toBeChecked();

    await act(async () => { finishRetry(response); });
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(projectRow()).toBe(row);
    expect(selection).toBeChecked();
    expect(screen.getByRole("textbox", { name: "Search projects..." })).toBe(search);
    expect(search).toHaveValue("Launch");
    expect(mocks.projectViewState.filters.statuses).toEqual([PROJECT.status]);
    expect(listProjects).toHaveBeenCalledTimes(2);
    expect(adapter.push).not.toHaveBeenCalled();
    expect(adapter.replace).not.toHaveBeenCalled();
  });

  it("hides cached rows and selection when a real refresh revokes workspace access", async () => {
    const listProjects = vi.fn().mockRejectedValue(new ApiError("Forbidden", 403, "Forbidden"));
    const { client } = renderQueryProjects(listProjects, [PROJECT]);
    await userEvent.setup().click(screen.getByRole("checkbox", { name: "Select Launch Plan" }));
    await act(async () => { await client.refetchQueries({ queryKey: projectKeys.list("workspace-1"), exact: true }); });

    expect(await screen.findByRole("alert")).toHaveTextContent("You no longer have access to this project.");
    expect(screen.queryByRole("link", { name: PROJECT.title })).not.toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: "Select Launch Plan" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
    expect(client.getQueryData(projectKeys.list("workspace-1"))).toBeUndefined();
    expect(listProjects).toHaveBeenCalledOnce();
  });
});

vi.mock("./project-property-recovery", () => ({ useProjectPropertyEditor: (project: Project) => ({ send: (data: object) => mocks.updateProject({ id: project.id, ...data }), recovery: null }) }));
