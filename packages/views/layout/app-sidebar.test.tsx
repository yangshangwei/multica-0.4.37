import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { cloneElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@multica/core/api";
import { useSidebarPreferences } from "@multica/core/navigation";
import type { Workspace } from "@multica/core/types";
import { useModalStore } from "@multica/core/modals";
import { openCreateIssueWithPreference } from "@multica/core/issues/stores/create-mode-store";
import { AppSidebar } from "./app-sidebar";
import { renderWithI18n } from "../test/i18n";

const { authState, eventCallbacks, invalidateQueries, logout, queryLog, iterationCapability, triageSettings, appForeground, chatSessions, chatStore, detail, deletePin, inboxItems, menuState, navigation, navigationPush, notificationUnread, pins, sidebarState, summary, workspaces } = vi.hoisted(() => ({
  authState: { user: { id: "user-1", name: "Alex", email: "alex@example.test" }, status: "authenticated" },
  eventCallbacks: new Map<string, () => void>(),
  invalidateQueries: vi.fn(),
  logout: vi.fn(),
  queryLog: [] as string[],
  appForeground: { current: true },
  iterationCapability: { current: { supported: true, manual: true, enabled: false } },
  triageSettings: { current: { enabled: false } },
  sidebarState: { setOpenMobile: vi.fn(), state: "expanded" as "expanded" | "collapsed", isCompact: false, hasExternalTrigger: false },
  menuState: { real: false },
  chatSessions: { current: [] as { id?: string; unread_count?: number }[] },
  chatStore: { current: { activeSessionId: null as string | null, isOpen: false } },
  detail: { current: { isPending: false, isError: false, data: null as unknown, error: null as unknown } },
  deletePin: vi.fn(),
  inboxItems: { current: [] as { id: string; read: boolean }[] },
  navigation: { current: { pathname: "/acme/issues", search: "" } as { pathname: string; search?: string } },
  navigationPush: vi.fn(),
  notificationUnread: { current: undefined as number | undefined },
  summary: { current: [] as { workspace_id: string; count: number }[] },
  workspaces: {
    current: [] as { id: string; name: string; slug: string; avatar_url: string | null }[],
  },
  pins: {
    current: [
      {
        id: "pin-1",
        workspace_id: "ws-1",
        user_id: "user-1",
        item_type: "issue" as const,
        item_id: "issue-1",
        position: 0,
        created_at: "2026-05-06T00:00:00Z",
      },
    ],
  },
}));

vi.mock("@multica/core/collaboration", () => ({
  useCollaborationScope: () => ({ apiScope: "test", server: "http://127.0.0.1:8080", userId: "user-1" }),
  collaborationKeys: { notifications: () => ["collaboration", "notifications"] },
  collaborationNotificationsOptions: () => ({ queryKey: ["collaboration", "notifications"] }),
  isCollaborationScopeCurrent: () => true,
}));
vi.mock("@multica/core/realtime", () => ({
  useWSEvent: (event: string, callback: () => void) => { eventCallbacks.set(event, callback); },
  useWSReconnect: (callback: () => void) => { eventCallbacks.set("reconnect", callback); },
}));
vi.mock("../collaboration/creation-form", () => ({
  CollaborationCreationEntry: ({ sourceWorkspaceId, open }: { sourceWorkspaceId?: string; open?: boolean }) => open ? <div data-testid="collaboration-creation" data-workspace-id={sourceWorkspaceId} /> : null,
}));
vi.mock("@dnd-kit/core", () => ({
  DndContext: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  PointerSensor: vi.fn(),
  closestCenter: vi.fn(),
  useSensor: vi.fn(),
  useSensors: vi.fn(),
}));
vi.mock("@dnd-kit/sortable", () => ({
  SortableContext: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useSortable: () => ({ attributes: {}, listeners: {}, setNodeRef: vi.fn() }),
  verticalListSortingStrategy: vi.fn(),
}));
vi.mock("@dnd-kit/utilities", () => ({ CSS: { Transform: { toString: () => undefined } } }));
vi.mock("@multica/ui/components/ui/sidebar", () => ({
  Sidebar: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SidebarContent: ({ children }: { children: React.ReactNode }) => <div data-slot="sidebar-content">{children}</div>,
  SidebarFooter: ({ children }: { children: React.ReactNode }) => <div data-slot="sidebar-footer">{children}</div>,
  SidebarGroup: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SidebarGroupContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SidebarGroupLabel: ({ children, render, ...props }: React.ComponentProps<"div"> & { render?: React.ReactElement<React.ComponentProps<"div">> }) => {
    if (render) return cloneElement(render, props, children);
    return <div {...props}>{children}</div>;
  },
  SidebarHeader: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SidebarMenu: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SidebarMenuButton: ({
    children,
    isActive,
    render,
    tooltip,
    ...props
  }: React.HTMLAttributes<HTMLElement> & {
    isActive?: boolean;
    tooltip?: string;
    render?: React.ReactElement<React.HTMLAttributes<HTMLElement> & { href?: string }>;
  }) => {
    const attributes = { ...props, "data-tooltip": tooltip, "data-active": isActive ? "true" : undefined, "data-href": render?.props.href };
    return render ? cloneElement(render, attributes, children) : <button type="button" {...attributes}>{children}</button>;
  },
  SidebarMenuItem: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SidebarRail: () => null,
  SidebarTrigger: ({ onClick, ...props }: React.ComponentProps<"button">) => <button type="button" data-slot="sidebar-trigger" {...props} onClick={(event) => {
    onClick?.(event);
    sidebarState.state = sidebarState.state === "expanded" ? "collapsed" : "expanded";
  }}>Toggle sidebar</button>,
  useSidebar: () => sidebarState,
}));
vi.mock("@multica/ui/components/ui/dropdown-menu", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/ui/components/ui/dropdown-menu")>();
  const mocks = {
  DropdownMenu: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuContent: ({ children, "data-sidebar-more": more }: { children: React.ReactNode; "data-sidebar-more"?: string }) => <div data-sidebar-more={more}>{children}</div>,
  DropdownMenuGroup: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuItem: ({ children, onClick, disabled, render, "aria-description": ariaDescription, "aria-current": current }: React.HTMLAttributes<HTMLElement> & { disabled?: boolean; render?: React.ReactElement<React.HTMLAttributes<HTMLElement>> }) => render ? cloneElement(render, { onClick, role: "menuitem", "aria-description": ariaDescription, "aria-current": current }, children) : <button type="button" onClick={onClick} disabled={disabled} aria-description={ariaDescription}>{children}</button>,
  DropdownMenuLabel: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuSeparator: () => null,
  DropdownMenuTrigger: ({ render, children, ...props }: React.ComponentProps<"button"> & { render?: React.ReactElement }) => render ? <>{render}</> : <button type="button" {...props}>{children}</button>,
  DropdownMenuSub: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuSubContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuSubTrigger: ({ children }: { children: React.ReactNode }) => <button type="button">{children}</button>,
  };
  function choose<Props extends object>(Mock: React.ComponentType<Props>, Real: React.ComponentType<Props>) {
    function SelectedMenuPrimitive(props: Props) { return menuState.real ? <Real {...props} /> : <Mock {...props} />; }
    return SelectedMenuPrimitive;
  }
  return {
    DropdownMenu: choose(mocks.DropdownMenu, actual.DropdownMenu),
    DropdownMenuContent: choose(mocks.DropdownMenuContent, actual.DropdownMenuContent),
    DropdownMenuGroup: choose(mocks.DropdownMenuGroup, actual.DropdownMenuGroup),
    DropdownMenuItem: choose(mocks.DropdownMenuItem, actual.DropdownMenuItem),
    DropdownMenuLabel: choose(mocks.DropdownMenuLabel, actual.DropdownMenuLabel),
    DropdownMenuSeparator: choose(mocks.DropdownMenuSeparator, actual.DropdownMenuSeparator),
    DropdownMenuTrigger: choose(mocks.DropdownMenuTrigger, actual.DropdownMenuTrigger),
    DropdownMenuSub: choose(mocks.DropdownMenuSub, actual.DropdownMenuSub),
    DropdownMenuSubContent: choose(mocks.DropdownMenuSubContent, actual.DropdownMenuSubContent),
    DropdownMenuSubTrigger: choose(mocks.DropdownMenuSubTrigger, actual.DropdownMenuSubTrigger),
  };
});
vi.mock("@multica/ui/components/ui/collapsible", async () => {
  const { createContext, useContext } = await import("react");
  const Context = createContext<{ open: boolean; onOpenChange: (open: boolean) => void }>({ open: false, onOpenChange: (_open: boolean) => undefined });
  return {
    Collapsible: ({ children, open = false, onOpenChange = () => undefined }: { children: React.ReactNode; open?: boolean; onOpenChange?: (open: boolean) => void }) => <Context.Provider value={{ open, onOpenChange }}>{children}</Context.Provider>,
    CollapsibleContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    CollapsibleTrigger: ({ children, ...props }: React.ComponentProps<"button">) => {
      const { open, onOpenChange } = useContext(Context);
      return <button type="button" {...props} aria-expanded={open} onClick={() => onOpenChange(!open)}>{children}</button>;
    },
  };
});
vi.mock("@multica/ui/components/ui/tooltip", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/ui/components/ui/tooltip")>();
  const mocks = {
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children, render, ...props }: React.ComponentProps<"button"> & { render?: React.ReactElement<React.ComponentProps<"button">> }) => render ? cloneElement(render, props, children) : <button type="button" {...props}>{children}</button>,
  };
  function choose<Props extends object>(Mock: React.ComponentType<Props>, Real: React.ComponentType<Props>) {
    function SelectedTooltipPrimitive(props: Props) { return menuState.real ? <Real {...props} /> : <Mock {...props} />; }
    return SelectedTooltipPrimitive;
  }
  return {
    Tooltip: choose(mocks.Tooltip, actual.Tooltip),
    TooltipContent: choose(mocks.TooltipContent, actual.TooltipContent),
    TooltipTrigger: choose(mocks.TooltipTrigger, actual.TooltipTrigger),
  };
});
vi.mock("../common/use-app-foreground", () => ({
  useAppForeground: () => appForeground.current,
}));
vi.mock("./help-launcher", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./help-launcher")>();
  return {
    HelpLauncher: (props: React.ComponentProps<typeof actual.HelpLauncher>) => menuState.real
      ? <actual.HelpLauncher {...props} /> : <div data-testid="help-launcher">{props?.versionSlot}</div>,
  };
});
vi.mock("../auth", () => ({ useLogout: () => logout }));
vi.mock("../issues/components/status-icon", () => ({ StatusIcon: () => <span /> }));
vi.mock("../navigation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../navigation")>()),
  AppLink: ({ children, href, newTabTitle: _newTabTitle, ...props }: React.ComponentProps<"a"> & { newTabTitle?: string }) => <a href={href} {...props}>{children}</a>,
  useNavigation: () => ({ pathname: navigation.current.pathname, push: navigationPush, searchParams: new URLSearchParams(navigation.current.search ?? ""), hash: "" }),
}));
vi.mock("../projects/components/project-icon", () => ({ ProjectIcon: () => <span /> }));
vi.mock("../workspace/workspace-avatar", () => ({ WorkspaceAvatar: () => <span /> }));
vi.mock("@multica/ui/components/common/actor-avatar", () => ({ ActorAvatar: () => <span /> }));

vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector: (state: typeof authState) => unknown) => selector(authState),
    { getState: () => authState },
  ),
}));
// Callable-store shape (selectorFn + getState) per the repo testing rules.
vi.mock("@multica/core/chat", () => ({
  useChatStore: Object.assign(
    (selector: (state: { activeSessionId: string | null; isOpen: boolean }) => unknown) =>
      selector(chatStore.current),
    { getState: () => chatStore.current },
  ),
}));
vi.mock("@multica/core/paths", async (importOriginal) => ({
  // Spread the real module so pure helpers (resolveRouteIconName, used by the
  // nav to derive each item's icon from its href) stay intact; only the
  // workspace/context hooks below are stubbed to control routes in tests.
  ...(await importOriginal<typeof import("@multica/core/paths")>()),
  useCurrentWorkspace: () => ({ id: "ws-1", name: "Acme", slug: "acme" }),
  useWorkspacePaths: () => ({
    inbox: () => "/acme/inbox",
    chat: () => "/acme/chat",
    myIssues: () => "/acme/my-issues",
    issues: () => "/acme/issues",
    projects: () => "/acme/projects",
    autopilots: () => "/acme/autopilots",
    agents: () => "/acme/agents",
    squads: () => "/acme/squads",
    usage: () => "/acme/usage",
    runtimes: () => "/acme/runtimes",
    skills: () => "/acme/skills",
    marketplace: () => "/acme/marketplace",
    mcp: () => "/acme/mcp",
    settings: () => "/acme/settings",
    iterations: () => "/acme/iterations",
    triage: () => "/acme/triage",
    issueDetail: (id: string) => `/acme/issues/${id}`,
    projectDetail: (id: string) => `/acme/projects/${id}`,
  }),
}));
vi.mock("@multica/core/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/api")>();
  const client = new actual.ApiClient("http://127.0.0.1:8080");
  return {
    ...actual,
    api: client,
    getApi: () => client,
  };
});
vi.mock("@multica/core/inbox/queries", () => ({
  deduplicateInboxItems: (items: unknown[]) => items,
  inboxKeys: { list: () => ["inbox"], unreadSummary: () => ["inbox", "unread-summary"] },
  inboxUnreadSummaryOptions: () => ({ queryKey: ["inbox", "unread-summary"] }),
  hasOtherWorkspaceUnread: (
    entries: { workspace_id: string; count: number }[],
    currentWsId: string | null,
  ) => entries.some((s) => s.workspace_id !== currentWsId && s.count > 0),
  unreadWorkspaceIds: (entries: { workspace_id: string; count: number }[]) =>
    new Set(entries.filter((s) => s.count > 0).map((s) => s.workspace_id)),
}));
vi.mock("@multica/core/issues/queries", () => ({ issueDetailOptions: () => ({ queryKey: ["issue"] }) }));
vi.mock("@multica/core/issues/stores/create-mode-store", () => ({
  useCreateModeStore: { getState: () => ({ lastMode: "agent" }) },
  openCreateIssueWithPreference: vi.fn(),
}));
vi.mock("@multica/core/issues/stores/draft-store", () => ({ useIssueDraftStore: () => false }));
vi.mock("@multica/core/pins/mutations", () => ({ useDeletePin: () => ({ mutate: deletePin }), useReorderPins: () => ({ mutate: vi.fn() }) }));
vi.mock("@multica/core/pins/queries", () => ({ pinListOptions: () => ({ queryKey: ["pins"] }) }));
vi.mock("@multica/core/projects/queries", () => ({ projectDetailOptions: () => ({ queryKey: ["project"] }) }));
vi.mock("@multica/core/workspace/queries", () => ({
  myInvitationListOptions: () => ({ queryKey: ["invitations"] }),
  workspaceKeys: { myInvitations: () => ["invitations"] },
  workspaceListOptions: () => ({ queryKey: ["workspaces"] }),
}));
vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-query")>()),
  useMutation: () => ({ isPending: false, mutate: vi.fn() }),
  useInfiniteQuery: () => ({ data: notificationUnread.current === undefined ? undefined : { pages: [{ unread_count: notificationUnread.current }] } }),
  useQuery: ({ queryKey }: { queryKey: readonly unknown[] }) => {
    queryLog.push(String(queryKey[0]));
    if (queryKey[0] === "iterations") return { data: iterationCapability.current };
    if (queryKey[0] === "triage" && queryKey[2] === "settings") return { data: triageSettings.current };
    if (queryKey[0] === "pins") return { data: pins.current };
    if (queryKey[0] === "issue") return detail.current;
    if (queryKey[0] === "inbox" && queryKey[1] === "unread-summary") return { data: summary.current };
    if (queryKey[0] === "inbox") return { data: inboxItems.current };
    if (queryKey[0] === "workspaces") return { data: workspaces.current };
    if (queryKey[0] === "chat" && queryKey[2] === "sessions") return { data: chatSessions.current };
    return { data: [] };
  },
  useQueryClient: () => ({ fetchQuery: vi.fn(), invalidateQueries }),
}));

beforeEach(() => {
  useModalStore.getState().close();
  vi.mocked(openCreateIssueWithPreference).mockClear();
  useSidebarPreferences.setState({ groups: { work: true, ai: false, resources: false, collaboration: false } });
  navigation.current = { pathname: "/acme/issues" };
  navigationPush.mockClear();
  notificationUnread.current = undefined;
  sidebarState.state = "expanded";
  sidebarState.isCompact = false;
  sidebarState.hasExternalTrigger = false;
  menuState.real = false;
});

describe("common icon navigation", () => {
  beforeEach(() => {
    sidebarState.state = "collapsed";
    detail.current = { isPending: false, isError: false, data: null, error: null };
    chatSessions.current = [];
    chatStore.current = { activeSessionId: null, isOpen: false };
    appForeground.current = true;
    summary.current = [];
    workspaces.current = [];
  });

  it("keeps common actions independent of collapsed directory preferences", () => {
    useSidebarPreferences.setState({ groups: { work: false, ai: false, resources: false, collaboration: false } });
    renderWithI18n(<AppSidebar />);
    for (const [name, href] of [["My Issues", "/acme/my-issues"], ["Workspace issues", "/acme/issues"], ["Projects", "/acme/projects"], ["Agents", "/acme/agents"]]) {
      const link = screen.getByRole("link", { name });
      expect(link).toHaveAttribute("href", href);
      expect(link).toHaveAttribute("data-tooltip", `${name} · Acme`);
    }
    expect(screen.getByRole("link", { name: "Workspace issues" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("button", { name: "More navigation" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Expand collaboration navigation" })).not.toBeInTheDocument();
    expect(useSidebarPreferences.getState().groups).toEqual({ work: false, ai: false, resources: false, collaboration: false });
    fireEvent.click(screen.getByRole("button", { name: "New Issue" }));
    expect(openCreateIssueWithPreference).toHaveBeenCalledOnce();
  });

  it("keeps secondary native destinations and capability gates in More", () => {
    const view = renderWithI18n(<AppSidebar />);
    for (const [name, href] of [["Inbox", "/acme/inbox"], ["Squads", "/acme/squads"], ["MCP", "/acme/mcp"], ["Analytics", "/acme/usage"], ["Participating", "/account?view=participating"]]) {
      expect(screen.getByRole("menuitem", { name })).toHaveAttribute("href", href);
    }
    expect(screen.queryByRole("menuitem", { name: "Iterations" })).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Triage" })).not.toBeInTheDocument();
    iterationCapability.current = { supported: true, manual: true, enabled: true };
    triageSettings.current = { enabled: true };
    view.rerender(<AppSidebar />);
    expect(screen.getByRole("menuitem", { name: "Iterations" })).toHaveAttribute("href", "/acme/iterations");
    expect(screen.getByRole("menuitem", { name: "Triage" })).toHaveAttribute("href", "/acme/triage");
    iterationCapability.current.enabled = false;
    triageSettings.current.enabled = false;
  });

  it("keeps positive chat and collaboration unread feedback with stable names", () => {
    notificationUnread.current = 3;
    chatSessions.current = [{ id: "chat-1", unread_count: 2 }];
    const view = renderWithI18n(<AppSidebar />);
    const notification = screen.getByRole("link", { name: "Collaboration notifications" });
    expect(notification).toHaveAttribute("href", "/account?view=notifications&filter=unread");
    expect(notification.querySelector("number-flow-react")).toHaveAttribute("aria-label", "3");
    expect(screen.getByRole("link", { name: "AI Chat" })).toHaveAccessibleDescription("Acme · 2 unread messages");
    notificationUnread.current = undefined;
    view.rerender(<AppSidebar />);
    expect(screen.getByRole("link", { name: "Collaboration notifications" }).querySelector("number-flow-react")).toBeNull();
  });

  it("marks More when a secondary destination is current", () => {
    navigation.current = { pathname: "/acme/mcp" };
    renderWithI18n(<AppSidebar />);
    expect(screen.getByRole("button", { name: "More navigation" })).toHaveAttribute("aria-current", "location");
    expect(screen.getByRole("menuitem", { name: "MCP" })).toHaveAttribute("aria-current", "page");
  });

  it("keeps full navigation in a compact sheet with parked collapsed desktop state", () => {
    sidebarState.isCompact = true;
    renderWithI18n(<AppSidebar />);
    expect(screen.getByRole("button", { name: "Expand collaboration navigation" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Start collaboration" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "More navigation" })).not.toBeInTheDocument();
  });

  it("scrolls compact start-work actions with the directory while the footer stays reachable", () => {
    sidebarState.isCompact = true;
    renderWithI18n(<AppSidebar />);
    const create = screen.getByRole("button", { name: "New Issue" });
    const directory = screen.getByRole("button", { name: "Expand collaboration navigation" });
    const scrollport = create.closest('[data-slot="sidebar-content"]');
    expect(scrollport).not.toBeNull();
    expect(directory.closest('[data-slot="sidebar-content"]')).toBe(scrollport);
    expect(screen.getByRole("button", { name: "Alex" }).closest('[data-slot="sidebar-content"]')).toBeNull();
  });

  it("keeps account access and disabled workspace actions with no workspace", () => {
    navigation.current = { pathname: "/account", search: "view=participating" };
    queryLog.length = 0;
    renderWithI18n(<AppSidebar navigationWorkspace={null} />);
    expect(screen.getByRole("button", { name: "New Issue" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "AI Chat" })).toBeDisabled();
    expect(screen.getByRole("link", { name: "Collaboration notifications" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Participating" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Projects" })).not.toBeInTheDocument();
    expect(queryLog.every((key) => key === "workspaces" || key === "invitations")).toBe(true);
  });

  it("uses the retained collaboration owner when opened from More", () => {
    const view = renderWithI18n(<AppSidebar />);
    fireEvent.click(screen.getByRole("button", { name: "Start collaboration" }));
    expect(screen.getByTestId("collaboration-creation")).toHaveAttribute("data-workspace-id", "ws-1");
    sidebarState.state = "expanded";
    view.rerender(<AppSidebar />);
    expect(screen.getByTestId("collaboration-creation")).toBeInTheDocument();
  });

  it("opens resolved pin destinations with native menu navigation and Escape focus", async () => {
    menuState.real = true;
    detail.current = { isPending: false, isError: false, data: { identifier: "MUL-123", title: "Keep this pin", status: "todo" }, error: null };
    renderWithI18n(<AppSidebar />);
    const more = screen.getByRole("button", { name: "More navigation" });
    more.focus();
    await userEvent.keyboard("{Enter}");
    const pinned = await screen.findByRole("menuitem", { name: "Pinned" });
    await userEvent.hover(pinned);
    const pin = await screen.findByRole("menuitem", { name: "Keep this pin" });
    expect(pin).toHaveAttribute("href", "/acme/issues/issue-1");
    expect(pin).not.toHaveAttribute("draggable", "true");
    await userEvent.keyboard("{Escape}{Escape}");
    expect(more).toHaveFocus();
  });

  it.each([
    ["Help", "Help"],
    ["Alex", "Alex"],
    ["Toggle sidebar", "Toggle left sidebar"],
  ])("shows the %s icon name on real keyboard focus", async (action, label) => {
    menuState.real = true;
    renderWithI18n(<AppSidebar />);
    const control = screen.getByRole("button", { name: action });
    control.focus();
    await userEvent.tab({ shift: true });
    await userEvent.tab();
    expect(control).toHaveFocus();
    expect(await screen.findByText(label, { selector: '[data-slot="tooltip-content"]' })).toBeVisible();
    expect(control.querySelector("button")).toBeNull();
  });

  it("shows Help on hover while the icon rail keeps its original menu action", async () => {
    menuState.real = true;
    renderWithI18n(<AppSidebar />);
    const control = screen.getByRole("button", { name: "Help" });
    await userEvent.hover(control);
    expect(await screen.findByText("Help", { selector: '[data-slot="tooltip-content"]' })).toBeVisible();
    control.focus();
    await userEvent.keyboard("{Enter}");
    expect(await screen.findByRole("menuitem", { name: "Feedback" })).toBeVisible();
  });

  it("preserves the native sidebar trigger slot and click action through its tooltip", async () => {
    menuState.real = true;
    const view = renderWithI18n(<AppSidebar />);
    const trigger = screen.getByRole("button", { name: "Toggle sidebar" });
    expect(trigger).toHaveAttribute("data-slot", "sidebar-trigger");
    await userEvent.click(trigger);
    expect(sidebarState.state).toBe("expanded");
    view.rerender(<AppSidebar />);
    expect(screen.queryByRole("button", { name: "More navigation" })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Toggle sidebar" }));
    expect(sidebarState.state).toBe("collapsed");
  });
});

it("offers all three start-work entries directly before the existing directories", () => {
  renderWithI18n(<AppSidebar />);
  const group = screen.getByRole("group", { name: "Start work" });
  const create = within(group).getByRole("button", { name: "New Issue" });
  const collaborate = within(group).getByRole("button", { name: "Start collaboration" });
  const chat = within(group).getByRole("link", { name: "AI Chat" });
  expect(create).toBeEnabled();
  expect(collaborate).toBeEnabled();
  expect(chat).toHaveAttribute("href", "/acme/chat");
  expect(create).toHaveTextContent("Acme");
  expect(chat).toHaveTextContent("Acme");
  expect(collaborate).toHaveTextContent("Cross-workspace");
  expect(screen.queryByRole("button", { name: "New" })).not.toBeInTheDocument();
  expect(screen.getAllByRole("link", { name: "AI Chat" })).toHaveLength(1);
});

it("opens ordinary creation immediately in the actual workspace", () => {
  renderWithI18n(<AppSidebar />);
  fireEvent.click(within(screen.getByRole("group", { name: "Start work" })).getByRole("button", { name: "New Issue" }));
  expect(openCreateIssueWithPreference).toHaveBeenCalledOnce();
  expect(navigationPush).not.toHaveBeenCalled();
});

it("toggles collaboration from its title, Beta marker and arrow without navigating", () => {
  const { container } = renderWithI18n(<AppSidebar />);
  expect(container.querySelector('[data-href="/account?view=current"]')).not.toBeInTheDocument();
  expect(container.querySelector('[data-href="/account?view=participating"]')).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Expand collaboration navigation" })).toHaveAttribute("aria-expanded", "false");
  fireEvent.click(screen.getByText("Collaboration", { exact: true }));
  expect(screen.getByRole("button", { name: "Collapse collaboration navigation" })).toHaveAttribute("aria-expanded", "true");
  expect(container.querySelector('[data-href="/account?view=current"]')).toBeInTheDocument();
  expect(useSidebarPreferences.getState().groups.collaboration).toBe(true);
  fireEvent.click(screen.getByText("Collaboration", { exact: true }));
  expect(screen.getByRole("button", { name: "Expand collaboration navigation" })).toHaveAttribute("aria-expanded", "false");
  expect(container.querySelector('[data-href="/account?view=current"]')).not.toBeInTheDocument();
  fireEvent.click(within(screen.getByRole("button", { name: "Expand collaboration navigation" })).getByText("Beta"));
  const expanded = screen.getByRole("button", { name: "Collapse collaboration navigation" });
  expect(expanded).toHaveAttribute("aria-expanded", "true");
  const arrow = expanded.querySelector("svg");
  expect(arrow).not.toBeNull();
  fireEvent.click(arrow!);
  expect(screen.getByRole("button", { name: "Expand collaboration navigation" })).toHaveAttribute("aria-expanded", "false");
  expect(navigationPush).not.toHaveBeenCalled();
});

it("expands collaboration when clicking the collapsed unread count", () => {
  notificationUnread.current = 3;
  renderWithI18n(<AppSidebar />);
  fireEvent.click(screen.getByRole("status", { name: "Collaboration notifications 3 unread" }));
  expect(screen.getByRole("button", { name: "Collapse collaboration navigation" })).toHaveAttribute("aria-expanded", "true");
  expect(useSidebarPreferences.getState().groups.collaboration).toBe(true);
  expect(navigationPush).not.toHaveBeenCalled();
});

it("describes Beta maturity on collaboration disclosure and creation", () => {
  renderWithI18n(<AppSidebar />);
  const description = "For cross-workspace collaboration. This feature is in Beta, and some functionality is still being developed.";
  for (const name of ["Expand collaboration navigation", "Start collaboration"]) {
    expect(screen.getByRole("button", { name })).toHaveAccessibleDescription(description);
  }
});

it("discloses collaboration without navigation and remembers the manual choice", async () => {
  const view = renderWithI18n(<AppSidebar />);
  screen.getByRole("button", { name: "Expand collaboration navigation" }).focus();
  await userEvent.keyboard("{Enter}");
  expect(screen.getByRole("button", { name: "Collapse collaboration navigation" })).toHaveAttribute("aria-expanded", "true");
  await userEvent.keyboard(" ");
  expect(screen.getByRole("button", { name: "Expand collaboration navigation" })).toHaveAttribute("aria-expanded", "false");
  await userEvent.keyboard("{Enter}");
  expect(screen.getByRole("button", { name: "Collapse collaboration navigation" })).toHaveAttribute("aria-expanded", "true");
  expect(navigationPush).not.toHaveBeenCalled();
  expect(useSidebarPreferences.getState().groups.collaboration).toBe(true);
  view.unmount();
  renderWithI18n(<AppSidebar />);
  expect(screen.getByRole("button", { name: "Collapse collaboration navigation" })).toHaveAttribute("aria-expanded", "true");
});

it("reveals collaboration on account routes and resets manual collapse on a new list", async () => {
  navigation.current = { pathname: "/account", search: "view=current&issue=00000000-0000-4000-8000-000000000001" };
  const view = renderWithI18n(<AppSidebar />);
  expect(screen.getByRole("button", { name: "Collapse collaboration navigation" })).toHaveAttribute("aria-expanded", "true");
  expect(useSidebarPreferences.getState().groups.collaboration).toBe(false);
  await userEvent.click(screen.getByRole("button", { name: "Collapse collaboration navigation" }));
  expect(screen.getByRole("button", { name: "Expand collaboration navigation" })).toHaveAttribute("aria-expanded", "false");
  navigation.current = { pathname: "/account", search: "view=participating&issue=00000000-0000-4000-8000-000000000001" };
  view.rerender(<AppSidebar />);
  expect(screen.getByRole("button", { name: "Collapse collaboration navigation" })).toHaveAttribute("aria-expanded", "true");
  expect(useSidebarPreferences.getState().groups.collaboration).toBe(false);
  navigation.current = { pathname: "/acme/issues" };
});

it("keeps collapsed unread feedback live without displaying unknown or zero as unread", () => {
  const view = renderWithI18n(<AppSidebar />);
  expect(screen.queryByLabelText("Collaboration notifications 3 unread")).not.toBeInTheDocument();
  notificationUnread.current = 3;
  view.rerender(<AppSidebar />);
  expect(screen.getByRole("status", { name: "Collaboration notifications 3 unread" })).toHaveTextContent("3");
  notificationUnread.current = 0;
  view.rerender(<AppSidebar />);
  expect(screen.queryByLabelText("Collaboration notifications 0 unread")).not.toBeInTheDocument();
});

it("offers a native notification page link with truthful unread feedback and no popover trigger", async () => {
  const view = renderWithI18n(<AppSidebar />);
  await userEvent.click(screen.getByRole("button", { name: "Expand collaboration navigation" }));
  const link = screen.getByRole("link", { name: "Collaboration notifications" });
  expect(link).toHaveAttribute("href", "/account?view=notifications&filter=unread");
  expect(link).not.toHaveAttribute("aria-haspopup");
  expect(link).not.toHaveTextContent("0");
  expect(screen.queryByRole("button", { name: "Collaboration notifications" })).not.toBeInTheDocument();
  notificationUnread.current = 3;
  view.rerender(<AppSidebar />);
  expect(link).toHaveTextContent("3");
});

it("keeps Notifications selected for an archived notification detail without selecting either workbench list", () => {
  notificationUnread.current = 3;
  navigation.current = { pathname: "/account", search: "view=notifications&filter=archived&notification=00000000-0000-4000-8000-000000000002&issue=00000000-0000-4000-8000-000000000001" };
  const { container } = renderWithI18n(<AppSidebar navigationWorkspace={null} />);
  const link = screen.getByRole("link", { name: "Collaboration notifications" });
  expect(link).toHaveAttribute("data-active", "true");
  expect(link).toHaveAttribute("aria-current", "location");
  expect(link).toHaveAttribute("href", "/account?view=notifications&filter=unread");
  expect(container.querySelector('[data-href="/account?view=current"]')).not.toHaveAttribute("data-active");
  expect(container.querySelector('[data-href="/account?view=participating"]')).not.toHaveAttribute("data-active");
  fireEvent.click(screen.getByRole("button", { name: "Collapse collaboration navigation" }));
  expect(screen.queryByRole("link", { name: "Collaboration notifications" })).not.toBeInTheDocument();
  expect(screen.getByRole("status", { name: "Collaboration notifications 3 unread" })).toHaveTextContent("3");
});

it("exposes explicit collaboration views before workspace navigation", () => {
  const { container } = renderWithI18n(<AppSidebar />);
  fireEvent.click(screen.getByRole("button", { name: "Expand collaboration navigation" }));
  const links = Array.from(container.querySelectorAll("[data-href]"), (link) => link.getAttribute("data-href"));
  expect(links.slice(1, 4)).toEqual(["/account?view=current", "/account?view=participating", "/account?view=notifications&filter=unread"]);
  expect(screen.getByRole("navigation", { name: "Application navigation" })).toBeInTheDocument();
});

it("keeps account navigation usable without a workspace or workspace providers", () => {
  navigation.current = { pathname: "/account", search: "view=participating" };
  queryLog.length = 0;
  eventCallbacks.clear();
  logout.mockClear();
  const { container } = renderWithI18n(<AppSidebar navigationWorkspace={null} />);
  expect(container.querySelector('[data-href="/account?view=participating"]')).toHaveAttribute("data-active", "true");
  expect(container.querySelector('[data-href="/acme/issues"]')).not.toBeInTheDocument();
  expect(screen.getByText("Choose a workspace to access its issues and resources.")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "New Issue" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "New Issue" })).toHaveTextContent("Open a workspace to create an issue");
  expect(screen.getByRole("button", { name: "AI Chat" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "AI Chat" })).toHaveTextContent("Choose a workspace to start AI Chat");
  expect(screen.getByRole("button", { name: "AI Chat" })).toHaveAccessibleDescription("Choose a workspace to start AI Chat");
  expect(screen.queryByRole("link", { name: "AI Chat" })).not.toBeInTheDocument();
  expect(queryLog.every((key) => key === "workspaces" || key === "invitations")).toBe(true);
  expect(eventCallbacks.size).toBe(0);
  expect(screen.getByRole("button", { name: "Alex" })).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Log out" }));
  expect(logout).toHaveBeenCalledOnce();
  navigation.current = { pathname: "/acme/issues" };
});

it("refreshes only collaboration notifications from the workspace sidebar without duplicate issue subscriptions", () => {
  navigation.current = { pathname: "/acme/issues" };
  eventCallbacks.clear();
  invalidateQueries.mockClear();
  renderWithI18n(<AppSidebar />);
  expect([...eventCallbacks.keys()]).toEqual(["collaboration:workbench_changed", "reconnect"]);
  eventCallbacks.get("collaboration:workbench_changed")!();
  expect(invalidateQueries).toHaveBeenCalledExactlyOnceWith({ queryKey: ["collaboration", "notifications"] }, { cancelRefetch: false });
});

describe("collaboration creation workspace", () => {
  const workspace: Workspace = {
    id: "ws-2", name: "Other workspace", slug: "other", description: null, context: null,
    settings: {}, repos: [], issue_prefix: "OTHER", avatar_url: null,
    created_at: "2026-11-22T00:00:00Z", updated_at: "2026-11-22T00:00:00Z",
  };

  it("defaults to the selected sidebar workspace from account scope", () => {
    navigation.current = { pathname: "/account", search: "view=participating" };
    renderWithI18n(<AppSidebar navigationWorkspace={workspace} />);

    expect(screen.getByRole("button", { name: "New Issue" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Start collaboration" }));
    expect(screen.getByTestId("collaboration-creation")).toHaveAttribute("data-workspace-id", workspace.id);
  });

  it("follows a changed sidebar workspace while staying in account scope", () => {
    navigation.current = { pathname: "/account", search: "view=participating" };
    const view = renderWithI18n(<AppSidebar navigationWorkspace={workspace} />);
    const selected = { ...workspace, id: "ws-3", name: "Selected workspace", slug: "selected" };
    view.rerender(<AppSidebar navigationWorkspace={selected} />);

    fireEvent.click(screen.getByRole("button", { name: "Start collaboration" }));
    expect(screen.getByTestId("collaboration-creation")).toHaveAttribute("data-workspace-id", selected.id);
    expect(screen.getByRole("button", { name: "New Issue" })).toBeEnabled();
  });

  it("queues ordinary creation for the displayed workspace before navigation without preview chat reads", () => {
    navigation.current = { pathname: "/account", search: "view=participating" };
    queryLog.length = 0;
    renderWithI18n(<AppSidebar navigationWorkspace={workspace} />);
    fireEvent.click(screen.getByRole("button", { name: "New Issue" }));
    expect(navigationPush).toHaveBeenCalledExactlyOnceWith("/other/issues");
    expect(openCreateIssueWithPreference).not.toHaveBeenCalled();
    expect(useModalStore.getState().workspaceIssueCreation).toMatchObject({
      workspaceId: workspace.id, workspaceSlug: workspace.slug,
      originPath: "/account?view=participating", targetPath: "/other/issues", userId: "user-1",
    });
    expect(queryLog).not.toContain("chat");
    expect(screen.getByRole("link", { name: "AI Chat" })).toHaveAttribute("href", "/other/chat");
  });

  it("cancels a waiting create request when the committed account destination changes", () => {
    navigation.current = { pathname: "/account", search: "view=participating" };
    const view = renderWithI18n(<AppSidebar navigationWorkspace={workspace} />);
    fireEvent.click(screen.getByRole("button", { name: "New Issue" }));
    expect(useModalStore.getState().workspaceIssueCreation).not.toBeNull();
    navigation.current = { pathname: "/account", search: "view=current" };
    view.rerender(<AppSidebar navigationWorkspace={workspace} />);
    expect(useModalStore.getState().workspaceIssueCreation).toBeNull();
  });

  it("supersedes a waiting create request with an explicit AI Chat action", () => {
    navigation.current = { pathname: "/account", search: "view=participating" };
    renderWithI18n(<AppSidebar navigationWorkspace={workspace} />);
    fireEvent.click(screen.getByRole("button", { name: "New Issue" }));
    const chat = screen.getByRole("link", { name: "AI Chat" });
    chat.addEventListener("click", (event) => event.preventDefault(), { once: true });
    fireEvent.click(chat);
    expect(useModalStore.getState().workspaceIssueCreation).toBeNull();
  });
});

describe("PinRow", () => {
  beforeEach(() => {
    deletePin.mockReset();
    navigation.current.pathname = "/acme/issues";
    detail.current = { isPending: false, isError: false, data: null, error: null };
    summary.current = [];
    workspaces.current = [];
  });

  it("unpins missing details", async () => {
    detail.current = { isPending: false, isError: true, data: null, error: new ApiError("missing", 404, "Not Found") };
    render(<AppSidebar />);
    await waitFor(() => expect(deletePin).toHaveBeenCalledTimes(1));
  });

  it("ignores non-404 errors", async () => {
    detail.current = { isPending: false, isError: true, data: null, error: new ApiError("error", 500, "Server Error") };
    render(<AppSidebar />);
    await waitFor(() => expect(deletePin).not.toHaveBeenCalled());
  });

  it("renders loaded details", async () => {
    detail.current = { isPending: false, isError: false, data: { identifier: "MUL-123", title: "Keep this pin", status: "todo" }, error: null };
    render(<AppSidebar />);
    expect(await screen.findByText("Keep this pin")).toBeInTheDocument();
    expect(screen.queryByText("MUL-123 Keep this pin")).not.toBeInTheDocument();
  });

  it("does not also highlight the parent workspace nav for an active pin", async () => {
    navigation.current.pathname = "/acme/issues/issue-1";
    detail.current = {
      isPending: false,
      isError: false,
      data: { identifier: "MUL-123", title: "Keep this pin", status: "todo" },
      error: null,
    };

    const { container } = render(<AppSidebar />);

    expect((await screen.findByText("Keep this pin")).closest("a")).toHaveAttribute(
      "data-active",
      "true",
    );
    expect(container.querySelector('[data-href="/acme/issues"]')).not.toHaveAttribute("data-active");
  });
});

// On a phone the sidebar is a Sheet laid over the page, so a nav tap that
// leaves it open renders the destination underneath and reads as a dead tap.
describe("mobile sheet dismissal", () => {
  beforeEach(() => {
    sidebarState.setOpenMobile.mockClear();
    navigation.current = { pathname: "/acme/issues" };
  });

  it("dismisses the sheet once the route changes", () => {
    const { rerender } = render(<AppSidebar />);
    sidebarState.setOpenMobile.mockClear();

    navigation.current = { pathname: "/acme/inbox" };
    rerender(<AppSidebar />);

    expect(sidebarState.setOpenMobile).toHaveBeenCalledWith(false);
  });

  it("dismisses on an account query-only view change", () => {
    navigation.current = { pathname: "/account", search: "view=current" };
    const { rerender } = render(<AppSidebar navigationWorkspace={null} />);
    sidebarState.setOpenMobile.mockClear();
    navigation.current = { pathname: "/account", search: "view=participating" };
    rerender(<AppSidebar navigationWorkspace={null} />);
    expect(sidebarState.setOpenMobile).toHaveBeenCalledWith(false);
  });

  it("dismisses on notification filter and selection changes", () => {
    navigation.current = { pathname: "/account", search: "view=notifications&filter=unread" };
    const { rerender } = render(<AppSidebar navigationWorkspace={null} />);
    sidebarState.setOpenMobile.mockClear();
    navigation.current = { pathname: "/account", search: "view=notifications&filter=all" };
    rerender(<AppSidebar navigationWorkspace={null} />);
    expect(sidebarState.setOpenMobile).toHaveBeenCalledExactlyOnceWith(false);
    sidebarState.setOpenMobile.mockClear();
    navigation.current.search += "&notification=00000000-0000-4000-8000-000000000002";
    rerender(<AppSidebar navigationWorkspace={null} />);
    expect(sidebarState.setOpenMobile).toHaveBeenCalledExactlyOnceWith(false);
  });

  // Closing on `pathname` rather than per-link keeps every route out of the
  // sidebar covered at once — nav groups, pins, and the switcher's own push.
  it("does not re-dismiss while the route holds still", () => {
    const { rerender } = render(<AppSidebar />);
    sidebarState.setOpenMobile.mockClear();

    rerender(<AppSidebar />);

    expect(sidebarState.setOpenMobile).not.toHaveBeenCalled();
  });
});

describe("workspace-switcher unread dot", () => {
  beforeEach(() => {
    summary.current = [];
    workspaces.current = [];
  });

  // The aggregate switcher dot is the only `.ring-sidebar` span in the tree
  // (DraftDot is null when there's no draft, and there are no invitations).
  const dot = (container: HTMLElement) => container.querySelector("span.bg-brand.ring-sidebar");

  it("shows a dot when another workspace has unread inbox items", () => {
    summary.current = [{ workspace_id: "ws-2", count: 3 }];
    const { container } = render(<AppSidebar />);
    expect(dot(container)).not.toBeNull();
  });

  it("does not show a dot when only the active workspace has unread", () => {
    // Active workspace is ws-1 (see useCurrentWorkspace mock).
    summary.current = [{ workspace_id: "ws-1", count: 3 }];
    const { container } = render(<AppSidebar />);
    expect(dot(container)).toBeNull();
  });

  it("does not show a dot when no workspace has unread", () => {
    summary.current = [];
    const { container } = render(<AppSidebar />);
    expect(dot(container)).toBeNull();
  });
});

describe("workspace-switcher dropdown per-workspace dot", () => {
  beforeEach(() => {
    summary.current = [];
    // Active workspace is ws-1 (see useCurrentWorkspace mock); "Other" is ws-2.
    workspaces.current = [
      { id: "ws-1", name: "Active WS", slug: "active", avatar_url: null },
      { id: "ws-2", name: "Other WS", slug: "other", avatar_url: null },
    ];
  });

  // Row dots are brand dots WITHOUT the aggregate avatar dot's `ring-sidebar`.
  const rowDots = (container: HTMLElement) =>
    container.querySelectorAll("span.bg-brand:not(.ring-sidebar)");

  it("dots the specific other workspace that has unread", () => {
    summary.current = [{ workspace_id: "ws-2", count: 3 }];
    const { container } = render(<AppSidebar />);
    // Exactly one row dot, sitting right after the "Other WS" name; the active
    // row shows the check, not a dot.
    expect(rowDots(container)).toHaveLength(1);
    expect(screen.getByText("Other WS").nextElementSibling?.className).toContain("bg-brand");
    expect(screen.getByText("Active WS").nextElementSibling?.className ?? "").not.toContain("bg-brand");
  });

  it("does not dot a workspace whose unread count is zero", () => {
    summary.current = [{ workspace_id: "ws-2", count: 0 }];
    const { container } = render(<AppSidebar />);
    expect(rowDots(container)).toHaveLength(0);
  });

  it("never dots the active workspace even when it has unread", () => {
    summary.current = [{ workspace_id: "ws-1", count: 5 }];
    const { container } = render(<AppSidebar />);
    expect(rowDots(container)).toHaveLength(0);
  });
});

describe("start-work navigation — AI Chat", () => {
  beforeEach(() => {
    chatSessions.current = [];
    inboxItems.current = [];
    navigation.current = { pathname: "/acme/issues" };
    chatStore.current = { activeSessionId: null, isOpen: false };
    appForeground.current = true;
  });

  // The mocked SidebarMenuButton exposes the AppLink target as `data-href`
  // and renders the label + badge as its children.
  const chatNav = (container: HTMLElement) =>
    container.querySelector<HTMLElement>('[data-href="/acme/chat"]');
  const chatBadge = (container: HTMLElement) =>
    chatNav(container)?.querySelector("number-flow-react") ?? null;

  it("keeps persistent Inbox and Chat counters static", () => {
    inboxItems.current = [{ id: "inbox-1", read: false }];
    chatSessions.current = [{ id: "chat-1", unread_count: 2 }];
    const { container } = render(<AppSidebar />);
    const inboxBadge = container
      .querySelector<HTMLElement>('[data-href="/acme/inbox"]')
      ?.querySelector("number-flow-react") as (HTMLElement & { animated?: boolean }) | null;
    const currentChatBadge = chatBadge(container) as (HTMLElement & { animated?: boolean }) | null;

    expect(inboxBadge?.animated).toBe(false);
    expect(currentChatBadge?.animated).toBe(false);
  });

  it("renders a Chat nav link to the workspace chat route", () => {
    const { container } = render(<AppSidebar />);
    expect(chatNav(container)).not.toBeNull();
  });

  it("badges the Chat nav with the summed unread_count of chat sessions", () => {
    chatSessions.current = [{ id: "a", unread_count: 3 }, { id: "b", unread_count: 2 }, { id: "c", unread_count: 0 }];
    const { container } = renderWithI18n(<AppSidebar />);
    expect(chatBadge(container)).toHaveAttribute("aria-label", "5");
    expect(chatNav(container)).toHaveAccessibleName("AI Chat");
    expect(chatNav(container)).toHaveAccessibleDescription("Acme · 5 unread messages");
  });

  it("announces a singular unread message while retaining the AI Chat action name", () => {
    chatSessions.current = [{ id: "a", unread_count: 1 }];
    renderWithI18n(<AppSidebar />);
    expect(screen.getByRole("link", { name: "AI Chat" })).toHaveAccessibleDescription("Acme · 1 unread message");
  });

  it("announces unread messages in Chinese without changing the action name", () => {
    chatSessions.current = [{ id: "a", unread_count: 5 }];
    renderWithI18n(<AppSidebar />, { locale: "zh-Hans" });
    expect(screen.getByRole("link", { name: "AI 对话" })).toHaveAccessibleDescription("Acme · 5 条未读消息");
  });

  it("shows no Chat unread badge when every session is read", () => {
    chatSessions.current = [{ id: "a", unread_count: 0 }, { id: "b" }];
    const { container } = renderWithI18n(<AppSidebar />);
    expect(chatBadge(container)).toBeNull();
    expect(chatNav(container)).toHaveAccessibleDescription("Acme");
  });

  it("excludes the session being viewed on the chat page from the badge", () => {
    // The thread list zeroes the open session's row badge; the aggregate
    // must follow, or a reply landing in the open conversation flashes a
    // count with no matching row.
    chatSessions.current = [{ id: "a", unread_count: 2 }, { id: "b", unread_count: 3 }];
    navigation.current = { pathname: "/acme/chat" };
    chatStore.current = { activeSessionId: "a", isOpen: false };
    const { container } = render(<AppSidebar />);
    expect(chatBadge(container)).toHaveAttribute("aria-label", "3");
  });

  it("excludes the viewed session when the floating chat window is open off-route", () => {
    chatSessions.current = [{ id: "a", unread_count: 2 }, { id: "b", unread_count: 3 }];
    navigation.current = { pathname: "/acme/issues" };
    chatStore.current = { activeSessionId: "a", isOpen: true };
    const { container } = render(<AppSidebar />);
    expect(chatBadge(container)).toHaveAttribute("aria-label", "3");
  });

  it("still counts a remembered selection when no chat surface is showing it", () => {
    // activeSessionId persists after the chat page closes; with both
    // surfaces closed nothing will auto mark-read, so the badge must count.
    chatSessions.current = [{ id: "a", unread_count: 2 }, { id: "b", unread_count: 3 }];
    navigation.current = { pathname: "/acme/issues" };
    chatStore.current = { activeSessionId: "a", isOpen: false };
    const { container } = render(<AppSidebar />);
    expect(chatBadge(container)).toHaveAttribute("aria-label", "5");
  });

  it("counts the active session while the floating window is open but the app is backgrounded", () => {
    // A reply landing while the app is not in the foreground is NOT auto
    // marked-read (MUL-4485), so its unread must still badge — otherwise the
    // notification is silently eaten while the user is away.
    chatSessions.current = [{ id: "a", unread_count: 2 }, { id: "b", unread_count: 3 }];
    navigation.current = { pathname: "/acme/issues" };
    chatStore.current = { activeSessionId: "a", isOpen: true };
    appForeground.current = false;
    const { container } = render(<AppSidebar />);
    expect(chatBadge(container)).toHaveAttribute("aria-label", "5");
  });

  it("counts the active session on the chat route while the app is backgrounded", () => {
    chatSessions.current = [{ id: "a", unread_count: 2 }, { id: "b", unread_count: 3 }];
    navigation.current = { pathname: "/acme/chat" };
    chatStore.current = { activeSessionId: "a", isOpen: false };
    appForeground.current = false;
    const { container } = render(<AppSidebar />);
    expect(chatBadge(container)).toHaveAttribute("aria-label", "5");
  });
});

// The four sections and their order ARE the sidebar's information architecture:
// personal → pinned → Work → AI Team, with Analytics and Settings parked in the
// footer. The SidebarGroup primitives are mocked to fragments here, so document
// order of the rendered links is what proves group membership. The group *labels*
// are not asserted: `t($ => $.sidebar.work_group)` is typechecked against
// locales/en/layout.json by i18n/resources-types.ts, and locales/parity.test.ts
// covers the other three bundles — this suite never initialises i18next.
describe("nav structure", () => {
  function navHrefs(container: HTMLElement) {
    return Array.from(container.querySelectorAll("[data-href]")).map((el) =>
      el.getAttribute("data-href"),
    );
  }

  beforeEach(() => {
    navigation.current = { pathname: "/acme/issues" };
    // data: null hides the default pin, leaving only the nav rows.
    detail.current = { isPending: false, isError: false, data: null, error: null };
    inboxItems.current = [];
    chatSessions.current = [];
    chatStore.current = { activeSessionId: null, isOpen: false };
    summary.current = [];
    workspaces.current = [];
  });

  it.each([
    ["/acme/issues", "page"],
    ["/acme/issues/issue-2", "location"],
    ["/acme/projects", null],
  ])("announces the workspace navigation state for %s", (pathname, current) => {
    navigation.current = { pathname };
    const { container } = render(<AppSidebar />);
    const row = container.querySelector('[data-href="/acme/issues"]');
    expect(row?.getAttribute("aria-current")).toBe(current);
  });

  it("renders every section in order", () => {
    const { container } = render(<AppSidebar />);
    fireEvent.click(screen.getByRole("button", { name: "Expand collaboration navigation" }));
    expect(navHrefs(container)).toEqual([
      // Start work
      "/acme/chat",
      // account collaboration
      "/account?view=current",
      "/account?view=participating",
      "/account?view=notifications&filter=unread",
      // personal
      "/acme/inbox",
      "/acme/my-issues",
      // Work
      "/acme/issues",
      "/acme/projects",
      // AI Team
      "/acme/agents",
      "/acme/squads",
      "/acme/autopilots",
      // Resources & connections
      "/acme/skills",
      "/acme/mcp",
      "/acme/runtimes",
      "/acme/marketplace",
      // footer
      "/acme/usage",
      "/acme/settings",
    ]);
  });

  // Analytics and Settings belong to SidebarFooter, not SidebarContent: the
  // footer sits outside the scroll area, so no number of pins can push Settings
  // out of reach. SidebarFooter is a fragment here, so the evidence is that both
  // rows precede the footer's own help row in document order.
  it("puts Analytics and Settings in the footer, above the help row", () => {
    const { container } = render(<AppSidebar />);
    const footerRow = container.querySelector("[data-testid='sidebar-footer-row']");
    expect(footerRow).not.toBeNull();

    for (const href of ["/acme/usage", "/acme/settings"]) {
      const row = container.querySelector(`[data-href="${href}"]`);
      expect(row, href).not.toBeNull();
      expect(
        row!.compareDocumentPosition(footerRow!) & Node.DOCUMENT_POSITION_FOLLOWING,
        `${href} should render before the footer help row`,
      ).toBeTruthy();
    }
  });

  it.each([
    ["/acme/settings/general", "/acme/settings"],
    ["/acme/mcp", "/acme/mcp"],
    ["/acme/marketplace", "/acme/marketplace"],
  ])("highlights only the current navigation entry for %s", (pathname, href) => {
    navigation.current = { pathname };
    const { container } = render(<AppSidebar />);
    const activeItems = container.querySelectorAll('[data-href][data-active="true"]');
    expect(activeItems).toHaveLength(1);
    expect(activeItems[0]).toHaveAttribute("data-href", href);
  });
});

// Build info rides inside the left help menu, beside the server version; the
// right footer control owns account actions. AppSidebar only forwards the
// version slot — its rendering contract is covered in help-launcher.test.tsx.
describe("version slot", () => {
  it("forwards the platform's build info to the help menu", () => {
    render(<AppSidebar versionSlot={<span>v0.4.40</span>} />);
    expect(screen.getByTestId("help-launcher")).toHaveTextContent("v0.4.40");
  });

  it("leaves the help menu without build info on web", () => {
    render(<AppSidebar />);
    expect(screen.getByTestId("help-launcher")).toBeEmptyDOMElement();
  });
});

it.each([
  { supported: true, manual: true, enabled: true, visible: true },
  { supported: true, manual: true, enabled: false, visible: false },
  { supported: true, manual: false, enabled: true, visible: false },
  { supported: false, manual: true, enabled: true, visible: false },
])("shows daily iteration navigation only when confirmed usable: %j", ({ visible, ...capability }) => {
  iterationCapability.current = capability;
  const { container } = render(<AppSidebar />);
  expect(!!container.querySelector('[data-href="/acme/iterations"]')).toBe(visible);
});

// Triage is a low-frequency destination, so it closes the Work group after the
// daily issues / projects / iterations rows.
it("lists triage last in the work group", () => {
  iterationCapability.current = { supported: true, manual: true, enabled: true };
  triageSettings.current = { enabled: true };
  const workHrefs = ["/acme/issues", "/acme/projects", "/acme/iterations", "/acme/triage"];
  const { container } = render(<AppSidebar />);
  const rendered = Array.from(container.querySelectorAll("[data-href]"), (link) => link.getAttribute("data-href"));
  expect(rendered.filter((href) => workHrefs.includes(href ?? ""))).toEqual(workHrefs);
});
