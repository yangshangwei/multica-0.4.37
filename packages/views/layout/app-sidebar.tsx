"use client";

import { iterationCapabilitiesOptions } from "@multica/core/iterations";
import { triageSettingsOptions, triageCountsOptions } from "@multica/core/triage";

import { issueStatusCategory } from "@multica/core/issues";
import React, { useCallback, useEffect, useRef, useState } from "react";
import { cn } from "@multica/ui/lib/utils";
import { useScrollFade } from "@multica/ui/hooks/use-scroll-fade";
import { AppLink, useNavigation, currentPath } from "../navigation";
import { HelpLauncher } from "./help-launcher";
import { NavigationGroup, useNavigationGroupDisclosure } from "./sidebar-navigation-group";
import {
  DndContext,
  PointerSensor,
  useSensor,
  useSensors,
  closestCenter,
  type DragEndEvent,
} from "@dnd-kit/core";
import { SortableContext, verticalListSortingStrategy, useSortable, arrayMove } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Layers,
  ChevronDown,
  ChevronRight,
  ChevronUp,
  LogOut,
  Plus,
  Check,
  SquarePen,
  MessageSquare,
  UsersRound,
  Bell,
  Inbox,
  RefreshCw,
  X,
} from "lucide-react";
import { WorkspaceAvatar } from "../workspace/workspace-avatar";
import { ActorAvatar } from "@multica/ui/components/common/actor-avatar";
import { Tooltip, TooltipTrigger, TooltipContent } from "@multica/ui/components/ui/tooltip";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@multica/ui/components/ui/collapsible";
import { CappedNumberFlow } from "@multica/ui/components/ui/number-flow";
import { StatusIcon } from "../issues/components/status-icon";
import { useIssueDraftStore } from "@multica/core/issues/stores/draft-store";
import { openCreateIssueWithPreference } from "@multica/core/issues/stores/create-mode-store";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from "@multica/ui/components/ui/sidebar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { useAuthStore } from "@multica/core/auth";
import { issueViewDetailOptions } from "@multica/core/issue-views/queries";
import {
  issueViewContainerKey,
  useActiveIssueViewStore,
} from "@multica/core/issue-views/active-view-store";
import { useCurrentWorkspace, useWorkspacePaths, paths, accountPath, parseAccountPath } from "@multica/core/paths";
import { workspaceListOptions, myInvitationListOptions, workspaceKeys } from "@multica/core/workspace/queries";
import { resolvePublicFileUrl } from "@multica/core/workspace/avatar-url";
import { useQuery, useMutation, useQueryClient, useInfiniteQuery } from "@tanstack/react-query";
import { inboxKeys, deduplicateInboxItems, inboxUnreadSummaryOptions, hasOtherWorkspaceUnread, unreadWorkspaceIds } from "@multica/core/inbox/queries";
import { chatSessionsOptions } from "@multica/core/chat/queries";
import { countUnreadChatMessages } from "@multica/core/chat/unread";
import { useChatStore } from "@multica/core/chat";
import { api, ApiError, getApi } from "@multica/core/api";
import { useModalStore } from "@multica/core/modals";
import { useConfigStore } from "@multica/core/config";
import { pinListOptions } from "@multica/core/pins/queries";
import { useDeletePin, useReorderPins } from "@multica/core/pins/mutations";
import { issueDetailOptions } from "@multica/core/issues/queries";
import { projectDetailOptions } from "@multica/core/projects/queries";
import type { PinnedItem, Workspace } from "@multica/core/types";
import { useLogout } from "../auth";
import { ProjectIcon } from "../projects/components/project-icon";
import { routeIconForPath } from "./route-icon-components";
import { useT } from "../i18n";
import {
  useShortcut,
} from "@multica/core/shortcuts";
import { ShortcutKeycaps } from "../common/shortcut-keycaps";
import { useAppForeground } from "../common/use-app-foreground";
import { collaborationKeys, collaborationNotificationsOptions, isCollaborationScopeCurrent, useCollaborationScope, type CollaborationScope } from "@multica/core/collaboration";
import { useWSEvent, useWSReconnect } from "@multica/core/realtime";
import { useSidebarPreferences } from "@multica/core/navigation";
import { CollaborationCreationEntry } from "../collaboration/creation-form";

// Top-level nav items stay active when the user is on a child route
// (e.g. "Projects" stays lit on /:slug/projects/:id). Pinned items keep
// strict equality elsewhere — a pinned project shouldn't highlight on
// sub-pages of itself.
function isNavActive(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(href + "/");
}

// Stable empty arrays for query defaults. Using an inline `= []` default on
// `useQuery` creates a new array reference on every render when `data` is
// undefined (e.g. query disabled or loading) — which in turn breaks any
// `useEffect`/`useMemo` that depends on the value, and can trigger infinite
// re-render loops when the effect itself calls `setState`.
const EMPTY_PINS: PinnedItem[] = [];
const EMPTY_WORKSPACES: Awaited<ReturnType<typeof api.listWorkspaces>> = [];
const EMPTY_INVITATIONS: Awaited<ReturnType<typeof api.listMyInvitations>> = [];
const EMPTY_INBOX: Awaited<ReturnType<typeof api.listInbox>> = [];
const EMPTY_INBOX_SUMMARY: Awaited<ReturnType<typeof api.getInboxUnreadSummary>> = [];
const PRODUCT_NAME = "Multica";

// Nav items reference WorkspacePaths method names so they can be resolved
// against the current workspace slug at render time (see AppSidebar body).
// Only parameterless paths are valid nav destinations.
type NavKey =
  | "iterations"
  | "triage"
  | "inbox"
  | "myIssues"
  | "issues"
  | "projects"
  | "autopilots"
  | "agents"
  | "squads"
  | "usage"
  | "runtimes"
  | "skills"
  | "marketplace"
  | "mcp"
  | "settings";

// Static schema (key only) — labels resolved at render via useT("layout"),
// icons derived from the destination path via routeIconForPath.
type NavLabelKey =
  | "iterations"
  | "triage"
  | "inbox"
  | "my_issues"
  | "issues"
  | "projects"
  | "autopilots"
  | "agents"
  | "squads"
  | "usage"
  | "runtimes"
  | "skills"
  | "marketplace"
  | "mcp"
  | "settings";

// Nav icons are NOT declared here: they are derived from each item's
// destination path at render time, so the sidebar and the desktop tab bar
// always agree. See route-icon-components.tsx.
type NavItem = { key: NavKey; labelKey: NavLabelKey };

const personalNav: NavItem[] = [
  { key: "inbox", labelKey: "inbox" },
  { key: "myIssues", labelKey: "my_issues" },
];

const workNav: NavItem[] = [
  { key: "issues", labelKey: "issues" },
  { key: "projects", labelKey: "projects" },
  { key: "iterations", labelKey: "iterations" },
  { key: "triage", labelKey: "triage" },
];

const aiTeamNav: NavItem[] = [
  { key: "agents", labelKey: "agents" },
  { key: "squads", labelKey: "squads" },
  { key: "autopilots", labelKey: "autopilots" },
];

const resourceNav: NavItem[] = [
  { key: "skills", labelKey: "skills" },
  { key: "mcp", labelKey: "mcp" },
  { key: "runtimes", labelKey: "runtimes" },
  { key: "marketplace", labelKey: "marketplace" },
];

// Analytics and Settings live in the footer, outside SidebarContent's scroll
// area: enough pins overflow the nav, and Settings has to stay one click away
// instead of being scrolled off the bottom.
const bottomNav: NavItem[] = [
  { key: "usage", labelKey: "usage" },
  { key: "settings", labelKey: "settings" },
];

function DraftDot() {
  const hasDraft = useIssueDraftStore((s) => s.hasDraft());
  if (!hasDraft) return null;
  return <span className="absolute top-0 right-0 size-1.5 rounded-full bg-brand" />;
}

/**
 * One top-level nav row. Every nav group renders through this, which is what
 * keeps their active and hover treatment identical across the four sections.
 * The icon is derived from the destination path rather than passed in, so the
 * sidebar and the desktop tab bar always agree — see route-icon-components.tsx.
 */
function NavRow({
  href,
  label,
  isActive,
  trailing,
  scopeCaption,
  description,
}: {
  href: string;
  label: string;
  isActive: boolean;
  /** Right-aligned adornment, e.g. an unread count. */
  trailing?: React.ReactNode;
  /** Supporting scope text stays out of the action's stable accessible name. */
  scopeCaption?: string;
  /** Additional accessible detail, such as a positive unread-message count. */
  description?: string;
}) {
  const { pathname } = useNavigation();
  const Icon = routeIconForPath(href);
  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        isActive={isActive}
        aria-current={isActive ? (pathname === href ? "page" : "location") : undefined}
        aria-label={scopeCaption ? label : undefined}
        aria-description={description ?? scopeCaption}
        title={scopeCaption ?? label}
        render={<AppLink href={href} />}
        className={cn("text-muted-foreground hover:not-data-active:bg-sidebar-accent/70 data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground",
          scopeCaption && "h-auto min-h-10 py-1.5 pointer-coarse:min-h-11 data-active:font-medium")}
      >
        <Icon />
        {scopeCaption ? <span className="flex min-w-0 flex-1 flex-col items-start">
          <span className="max-w-full truncate">{label}</span>
          <span aria-hidden="true" className="max-w-full truncate text-caption font-normal">{scopeCaption}</span>
        </span> : <span className="min-w-0 flex-1 truncate">{label}</span>}
        {trailing && <span className="shrink-0">{trailing}</span>}
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

/**
 * Presentational pin row. The `label` and `iconNode` are computed by the
 * parent `PinRow` from cached issue / project detail queries — keeping
 * this component dumb means the dnd-kit / navigation wiring lives in
 * one place and the data flow is explicit.
 */
function SortablePinItem({
  pin,
  href,
  pathname,
  onUnpin,
  label,
  iconNode,
  onNavigate,
  isActiveOverride,
}: {
  pin: PinnedItem;
  href: string;
  pathname: string;
  onUnpin: () => void;
  label: string;
  iconNode: React.ReactNode;
  /** Runs on a real click (not a drag-release) before navigation. */
  onNavigate?: () => void;
  /** Overrides the plain path comparison (view pins carry extra state). */
  isActiveOverride?: boolean;
}) {
  const { t } = useT("layout");
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: pin.id });
  const wasDragged = useRef(false);

  useEffect(() => {
    if (isDragging) wasDragged.current = true;
  }, [isDragging]);

  const style = { transform: CSS.Transform.toString(transform), transition };
  const isActive = isActiveOverride ?? pathname === href;

  return (
    <SidebarMenuItem
      ref={setNodeRef}
      style={style}
      className={cn("group/pin", isDragging && "opacity-30")}
      {...attributes}
      {...listeners}
    >
      <SidebarMenuButton
        size="sm"
        aria-current={isActive ? "page" : undefined}
        isActive={isActive}
        render={<AppLink href={href} newTabTitle={label} draggable={false} />}
        onClick={(event) => {
          if (wasDragged.current) {
            wasDragged.current = false;
            event.preventDefault();
            return;
          }
          onNavigate?.();
        }}
        className={cn(
          "text-muted-foreground hover:not-data-active:bg-sidebar-accent/70 data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground",
          isDragging && "pointer-events-none",
        )}
      >
        {iconNode}
        <span
          className="min-w-0 flex-1 overflow-hidden whitespace-nowrap"
          style={{
            maskImage: "linear-gradient(to right, black calc(100% - 12px), transparent)",
            WebkitMaskImage: "linear-gradient(to right, black calc(100% - 12px), transparent)",
          }}
        >{label}</span>
        <Tooltip>
          <TooltipTrigger
            render={<span role="button" />}
            className="hidden size-2.5 shrink-0 items-center justify-center rounded-sm text-muted-foreground group-hover/pin:flex hover:text-foreground"
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              onUnpin();
            }}
          >
            <X className="size-1" />
          </TooltipTrigger>
          <TooltipContent side="top" sideOffset={4}>{t(($) => $.sidebar.unpin_tooltip)}</TooltipContent>
        </Tooltip>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

/**
 * Smart wrapper that resolves a pin's display data (label + status/icon)
 * from the issue / project detail query cache. Both queries are declared
 * unconditionally with `enabled` gates so the hook order stays stable
 * regardless of `pin.item_type`.
 *
 * Loading: render a flat skeleton so the sidebar height doesn't jump.
 * Missing (deleted item / 404): render nothing — the row hides itself
 * until the user unpins manually or a server-side cascade catches up.
 */
function PinRow({
  pin,
  href,
  pathname,
  onUnpin,
  wsId,
}: {
  pin: PinnedItem;
  href: string;
  pathname: string;
  onUnpin: () => void;
  wsId: string;
}) {
  const isIssue = pin.item_type === "issue";
  const isView = pin.item_type === "view";
  const p = useWorkspacePaths();
  const setActiveView = useActiveIssueViewStore((s) => s.setActive);
  const issueQuery = useQuery({
    ...issueDetailOptions(wsId, pin.item_id),
    enabled: isIssue,
  });
  const projectQuery = useQuery({
    ...projectDetailOptions(wsId, pin.item_id),
    enabled: pin.item_type === "project",
  });
  const viewQuery = useQuery({
    ...issueViewDetailOptions(wsId, pin.item_id),
    enabled: isView,
  });

  const triggeredRef = useRef(false);
  useEffect(() => {
    // Views are exempt from 404-auto-unpin: an installed desktop client
    // talking to an older backend without the view endpoints sees 404 for
    // every view pin — auto-unpinning would permanently delete them all.
    // A deleted view's row simply hides instead.
    if (isView) return;
    const err = isIssue ? issueQuery.error : projectQuery.error;
    if (err instanceof ApiError && err.status === 404 && !triggeredRef.current) {
      triggeredRef.current = true;
      onUnpin();
    }
  }, [isIssue, isView, issueQuery.error, onUnpin, projectQuery.error]);

  const activeViewByContainer = useActiveIssueViewStore((s) => s.active);
  if (isView) {
    if (viewQuery.isPending) return <PinSkeleton />;
    if (viewQuery.isError || !viewQuery.data) return null;
    const view = viewQuery.data;
    // One resolved scope drives the path AND the container key so an
    // unrecognised scope_type from a newer backend degrades coherently.
    const scopeType: "workspace" | "my" | "project" =
      view.scope_type === "my"
        ? "my"
        : view.scope_type === "project" && view.scope_id
          ? "project"
          : "workspace";
    const viewPath =
      scopeType === "my"
        ? p.myIssues()
        : scopeType === "project"
          ? p.projectDetail(view.scope_id!)
          : p.issues();
    const containerKey = issueViewContainerKey(wsId, {
      scope_type: scopeType,
      scope_id: scopeType === "project" ? view.scope_id : null,
    });
    return (
      <SortablePinItem
        pin={pin}
        // ?view= keeps a web reload on the view for the surfaces that mount
        // the URL-sync hook (/issues, /my-issues). Project pages don't sync
        // yet — there the query is inert and reload falls back to the plain
        // page; click-through activation still works everywhere.
        href={`${viewPath}?view=${view.id}`}
        pathname={pathname}
        onUnpin={onUnpin}
        label={view.name}
        iconNode={<Layers className="!size-3.5 shrink-0" />}
        // Active only when this exact view is open on its surface — the
        // path alone also matches the plain tab.
        isActiveOverride={
          pathname === viewPath && activeViewByContainer[containerKey] === view.id
        }
        onNavigate={() => setActiveView(containerKey, view.id)}
      />
    );
  }

  if (isIssue) {
    if (issueQuery.isPending) return <PinSkeleton />;
    if (issueQuery.isError || !issueQuery.data) return null;
    const issue = issueQuery.data;
    const label = issue.title;
    const iconNode = (
      /* Override parent [&_svg]:size-4 — pinned items need smaller icons to match sm size */
      <StatusIcon
        status={issue.status}
        category={issueStatusCategory(issue) ?? undefined}
        className="!size-3.5 shrink-0"
      />
    );
    return (
      <SortablePinItem
        pin={pin}
        href={href}
        pathname={pathname}
        onUnpin={onUnpin}
        label={label}
        iconNode={iconNode}
      />
    );
  }

  if (projectQuery.isPending) return <PinSkeleton />;
  if (projectQuery.isError || !projectQuery.data) return null;
  const project = projectQuery.data;
  const iconNode = <ProjectIcon project={project} size="sm" />;
  return (
    <SortablePinItem
      pin={pin}
      href={href}
      pathname={pathname}
      onUnpin={onUnpin}
      label={project.title}
      iconNode={iconNode}
    />
  );
}

function PinSkeleton() {
  return (
    <SidebarMenuItem>
      <div className="flex h-7 w-full items-center gap-2 px-2">
        <div className="size-3.5 shrink-0 rounded-sm bg-sidebar-accent/40" />
        <div className="h-3 w-24 rounded bg-sidebar-accent/40" />
      </div>
    </SidebarMenuItem>
  );
}

interface AppSidebarProps {
  topSlot?: React.ReactNode;
  searchSlot?: React.ReactNode;
  versionSlot?: React.ReactNode;
  headerClassName?: string;
  headerStyle?: React.CSSProperties;
  /** A navigation preview only; never installs a workspace route/provider. */
  navigationWorkspace?: Workspace | null;
  onWorkspaceSelect?: (slug: string) => void;
  workspaceListUnavailable?: boolean;
  onRetryWorkspaces?: () => void;
}

export function AppSidebar({ topSlot, searchSlot, versionSlot, headerClassName, headerStyle,
  navigationWorkspace, onWorkspaceSelect, workspaceListUnavailable = false, onRetryWorkspaces }: AppSidebarProps = {}) {
  const { t } = useT("layout");
  const navigation = useNavigation();
  const routeWorkspace = useCurrentWorkspace();
  const workspace = navigationWorkspace === undefined ? routeWorkspace : navigationWorkspace;
  const activeWorkspace = workspace !== null && routeWorkspace?.id === workspace.id && isNavActive(navigation.pathname, `/${workspace.slug}`);
  const { data: workspaces = EMPTY_WORKSPACES } = useQuery(workspaceListOptions());
  const { data: invitations = EMPTY_INVITATIONS } = useQuery(myInvitationListOptions());
  const creationDisabled = useConfigStore((state) => state.workspaceCreationDisabled);
  const selectWorkspace = (slug: string) => {
    useModalStore.getState().cancelWorkspaceIssueCreation();
    if (onWorkspaceSelect) onWorkspaceSelect(slug);
    else navigation.push(paths.workspace(slug).issues());
  };
  const pickerProps = { workspace, workspaces, invitations, creationDisabled, selectWorkspace };
  const { setOpenMobile } = useSidebar();
  const fullPath = currentPath(navigation);
  useEffect(() => { setOpenMobile(false); }, [fullPath, setOpenMobile]);
  useEffect(() => { void useSidebarPreferences.persist.rehydrate(); }, []);
  const scroll = useRef<HTMLDivElement>(null);
  const fade = useScrollFade(scroll, 24);
  return <Sidebar variant="inset">
    {topSlot}
    <nav aria-label={t(($) => $.global_navigation.label)} className="flex min-h-0 flex-1 flex-col" onClickCapture={(event) => {
      if (event.target instanceof Element && event.target.closest("a[href]")) {
        useModalStore.getState().cancelWorkspaceIssueCreation();
      }
    }}>
      <SidebarHeader className={cn("py-3", headerClassName)} style={headerStyle}>
        <div className="flex h-8 items-center px-2 text-body font-semibold">{PRODUCT_NAME}</div>
        {activeWorkspace && searchSlot && <SidebarMenu><SidebarMenuItem>{searchSlot}</SidebarMenuItem></SidebarMenu>}
        <SidebarStartWork workspace={workspace} workspaceActive={activeWorkspace} />
      </SidebarHeader>
      <SidebarContent ref={scroll} style={fade}>
        <CollaborationNavigation workspaceActive={activeWorkspace} />
        <div data-sidebar-scope="workspace" className="min-w-0">
          {activeWorkspace && workspace ? <WorkspaceNavigation workspace={workspace} pickerProps={pickerProps} /> : <>
            <WorkspacePicker {...pickerProps} />
            {workspace && <WorkspacePreview workspace={workspace} />}
          </>}
          {!workspace && <SidebarGroup><SidebarGroupContent>
            {workspaceListUnavailable ? <SidebarMenu><SidebarMenuItem><SidebarMenuButton onClick={onRetryWorkspaces}>
              <RefreshCw aria-hidden /><span>{t(($) => $.sidebar.retry_workspaces)}</span>
            </SidebarMenuButton></SidebarMenuItem></SidebarMenu> : <p className="px-2 py-2 text-caption text-muted-foreground">{t(($) => $.sidebar.no_workspace_selected)}</p>}
          </SidebarGroupContent></SidebarGroup>}
        </div>
      </SidebarContent>
      <SidebarFooter className="p-2">
        {workspace && <SidebarMenu className="gap-0.5">{bottomNav.map((item) => {
          const href = paths.workspace(workspace.slug)[item.key]();
          return <NavRow key={item.key} href={href} label={t(($) => $.nav[item.labelKey])} isActive={isNavActive(navigation.pathname, href)} />;
        })}</SidebarMenu>}
        <div data-testid="sidebar-footer-row" className="flex min-w-0 items-center gap-2">
          <HelpLauncher versionSlot={versionSlot} />
          <SidebarAccountMenu />
        </div>
      </SidebarFooter>
    </nav>
    <SidebarRail />
  </Sidebar>;
}

function SidebarAccountMenu() {
  const { t } = useT("layout");
  const user = useAuthStore((state) => state.user);
  const logout = useLogout();
  const name = user?.name || user?.email || PRODUCT_NAME;
  const initials = (user?.name || user?.email || "U").charAt(0).toUpperCase();
  const avatarUrl = resolvePublicFileUrl(user?.avatar_url);
  return <DropdownMenu>
    <DropdownMenuTrigger
      aria-label={name}
      title={name}
      className="ml-auto inline-flex h-8 min-w-0 max-w-full items-center gap-2 rounded-md px-2 text-body font-medium text-sidebar-foreground outline-hidden transition-colors hover:bg-sidebar-accent data-popup-open:bg-sidebar-accent focus-visible:ring-2 focus-visible:ring-sidebar-ring pointer-coarse:h-11 pointer-coarse:min-w-11"
    >
      <span aria-hidden="true"><ActorAvatar name={name} initials={initials} avatarUrl={avatarUrl} size="sm" /></span>
      <span className="min-w-0 truncate">{name}</span>
      <ChevronUp aria-hidden className="size-3 shrink-0 text-muted-foreground" />
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" side="top" sideOffset={8} className="w-64 max-w-[calc(100vw-2rem)]">
      <div className="flex items-center gap-2.5 px-2 py-2"><ActorAvatar name={name} initials={initials} avatarUrl={avatarUrl} size="lg" />
        <div className="min-w-0"><p className="truncate text-body font-medium">{user?.name}</p><p className="truncate text-caption text-muted-foreground">{user?.email}</p></div>
      </div>
      <DropdownMenuSeparator /><DropdownMenuItem variant="destructive" onClick={logout}><LogOut aria-hidden />{t(($) => $.sidebar.log_out)}</DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>;
}

function SidebarStartWork({ workspace, workspaceActive }: { workspace: Workspace | null; workspaceActive: boolean }) {
  const { t } = useT("layout");
  const { t: collaboration } = useT("collaboration");
  const navigation = useNavigation();
  const fullPath = currentPath(navigation);
  const scope = useCollaborationScope();
  const [collaborationOpen, setCollaborationOpen] = useState(false);
  const collaborationTrigger = useRef<HTMLButtonElement>(null);
  const shortcut = useShortcut("createIssue");
  const pendingCreation = useModalStore((state) => state.workspaceIssueCreation);
  const userId = useAuthStore((state) => state.user?.id);
  const authStatus = useAuthStore((state) => state.status);
  const workspaceCaption = workspace?.name ?? t(($) => $.sidebar.open_workspace_to_create);
  const chatCaption = workspace?.name ?? t(($) => $.sidebar.open_workspace_to_chat);

  useEffect(() => {
    if (!pendingCreation) return;
    const client = getApi();
    const viewer = useAuthStore.getState();
    // A preview may retain the handoff, but only ModalRegistry can consume it.
    useModalStore.getState().reconcileWorkspaceIssueCreation(pendingCreation, {
      api: client, sessionScope: client.getSessionScope(),
      userId: viewer.status === "authenticated" ? viewer.user?.id ?? null : null,
      path: fullPath,
    });
  }, [pendingCreation, fullPath, userId, authStatus]);

  const createIssue = () => {
    if (!workspace) return;
    if (workspaceActive) {
      openCreateIssueWithPreference();
      return;
    }
    const viewer = useAuthStore.getState();
    if (viewer.status !== "authenticated" || !viewer.user) return;
    const client = getApi();
    const targetPath = paths.workspace(workspace.slug).issues();
    useModalStore.getState().queueWorkspaceIssueCreation({
      api: client, sessionScope: client.getSessionScope(), userId: viewer.user.id,
      workspaceId: workspace.id, workspaceSlug: workspace.slug,
      originPath: fullPath, targetPath,
    });
    navigation.push(targetPath);
  };

  return <>
    <div role="group" aria-label={t(($) => $.sidebar.start_work)} className="min-w-0">
      <SidebarGroupLabel>{t(($) => $.sidebar.start_work)}</SidebarGroupLabel>
      <SidebarMenu className="gap-1">
        <SidebarMenuItem><SidebarMenuButton disabled={!workspace} onClick={createIssue}
          aria-label={t(($) => $.sidebar.new_issue)}
          aria-description={workspace ? workspace.name : t(($) => $.sidebar.open_workspace_to_create)}
          title={workspaceCaption}
          className="h-auto min-h-10 py-1.5 text-muted-foreground pointer-coarse:min-h-11">
          <SquarePen aria-hidden />
          <span className="flex min-w-0 flex-1 flex-col items-start">
            <span className="relative w-full truncate pr-2">{t(($) => $.sidebar.new_issue)}{workspaceActive && <DraftDot />}</span>
            <span aria-hidden="true" className="max-w-full truncate text-caption font-normal">{workspaceCaption}</span>
          </span>
          {workspaceActive && shortcut && <ShortcutKeycaps shortcut={shortcut} decorative className="pointer-events-none ms-auto shrink-0" />}
        </SidebarMenuButton></SidebarMenuItem>
        <SidebarMenuItem><SidebarMenuButton ref={collaborationTrigger}
          aria-label={t(($) => $.sidebar.start_collaboration)} aria-description={collaboration(($) => $.beta.description)}
          className="h-auto min-h-10 py-1.5 text-muted-foreground pointer-coarse:min-h-11"
          onClick={() => { useModalStore.getState().cancelWorkspaceIssueCreation(); setCollaborationOpen(true); }}>
          <UsersRound aria-hidden />
          <span className="flex min-w-0 flex-1 flex-col items-start">
            <span className="max-w-full truncate">{t(($) => $.sidebar.start_collaboration)}</span>
            <span aria-hidden="true" className="max-w-full truncate text-caption font-normal">{t(($) => $.sidebar.cross_workspace)}</span>
          </span>
          <span aria-hidden="true" className="shrink-0 rounded bg-muted px-1 py-0.5 text-micro font-normal text-muted-foreground">{collaboration(($) => $.beta.label)}</span>
        </SidebarMenuButton></SidebarMenuItem>
        {workspace ? workspaceActive ? <ActiveWorkspaceChatLink workspace={workspace} /> : <WorkspaceChatLink workspace={workspace} />
          : <SidebarMenuItem><SidebarMenuButton disabled aria-label={t(($) => $.nav.chat)}
            aria-description={t(($) => $.sidebar.open_workspace_to_chat)} className="h-auto min-h-10 py-1.5 text-muted-foreground pointer-coarse:min-h-11">
            <MessageSquare aria-hidden />
            <span className="flex min-w-0 flex-1 flex-col items-start"><span>{t(($) => $.nav.chat)}</span>
              <span aria-hidden="true" className="max-w-full truncate text-caption font-normal">{chatCaption}</span></span>
          </SidebarMenuButton></SidebarMenuItem>}
      </SidebarMenu>
    </div>
    <CollaborationCreationEntry key={`${scope.apiScope}:${scope.userId}:${workspace?.id ?? "account"}`} scope={scope} sourceWorkspaceId={workspace?.id} open={collaborationOpen} onOpenChange={setCollaborationOpen} hideTrigger returnFocus={collaborationTrigger} />
  </>;
}

function WorkspaceChatLink({ workspace, unread }: { workspace: Workspace; unread?: number }) {
  const { t } = useT("layout");
  const { pathname } = useNavigation();
  const href = paths.workspace(workspace.slug).chat();
  const hasUnread = unread !== undefined && unread > 0;
  const description = hasUnread
    ? `${workspace.name} · ${t(($) => $.sidebar.chat_unread, { count: unread })}`
    : workspace.name;
  return <NavRow href={href} label={t(($) => $.nav.chat)} scopeCaption={workspace.name} isActive={isNavActive(pathname, href)}
    description={description}
    trailing={hasUnread ? <CappedNumberFlow value={unread} animated={false} className="ml-auto text-caption" /> : undefined} />;
}

/** Only a real workspace route observes its unread messages and live selection. */
function ActiveWorkspaceChatLink({ workspace }: { workspace: Workspace }) {
  const { pathname } = useNavigation();
  const { data: chatSessions } = useQuery(chatSessionsOptions(workspace.id));
  const activeSessionId = useChatStore((state) => state.activeSessionId);
  const floatingChatOpen = useChatStore((state) => state.isOpen);
  const appForeground = useAppForeground();
  const href = paths.workspace(workspace.slug).chat();
  // Foreground reading excludes the current thread; background replies stay unread.
  const viewedSessionId = appForeground && (floatingChatOpen || isNavActive(pathname, href)) ? activeSessionId : null;
  const unread = React.useMemo(() => chatSessions ? countUnreadChatMessages(chatSessions, viewedSessionId) : undefined,
    [chatSessions, viewedSessionId]);
  return <WorkspaceChatLink workspace={workspace} unread={unread} />;
}

function WorkspaceNotificationRealtime({ scope }: { scope: CollaborationScope }) {
  const client = useQueryClient();
  const refresh = useCallback(() => {
    if (isCollaborationScopeCurrent(scope)) void client.invalidateQueries({ queryKey: collaborationKeys.notifications(scope) }, { cancelRefetch: false });
  }, [client, scope]);
  useWSEvent("collaboration:workbench_changed", refresh);
  useWSReconnect(refresh);
  return null;
}

function CollaborationNavigation({ workspaceActive }: { workspaceActive: boolean }) {
  const { t } = useT("layout");
  const { t: collaboration } = useT("collaboration");
  const navigation = useNavigation();
  const scope = useCollaborationScope();
  const destination = parseAccountPath(currentPath(navigation));
  const disclosure = useNavigationGroupDisclosure("collaboration", destination !== null && !destination.administration);
  const client = useQueryClient();
  const notifications = useInfiniteQuery({ ...collaborationNotificationsOptions(client, scope), enabled: scope.userId !== "" });
  const unread = notifications.data?.pages[0]?.unread_count;
  const notificationsSelected = destination?.view === "notifications";
  const notificationsLabel = collaboration(($) => $.notifications.title);
  return <SidebarGroup data-sidebar-scope="collaboration">
    {workspaceActive && <WorkspaceNotificationRealtime scope={scope} />}
    <Collapsible {...disclosure}>
      <Tooltip>
        <SidebarGroupLabel aria-label={disclosure.open ? collaboration(($) => $.page.collapseNavigation) : collaboration(($) => $.page.expandNavigation)} aria-description={collaboration(($) => $.beta.description)} render={<TooltipTrigger render={<CollapsibleTrigger />} />} className="h-auto min-h-8 w-full min-w-0 cursor-pointer gap-2 text-left hover:bg-sidebar-accent/70 hover:text-sidebar-accent-foreground pointer-coarse:min-h-11">
          <span className="flex min-w-0 flex-1 items-center gap-2"><span className="min-w-0 truncate">{collaboration(($) => $.page.title)}</span><span aria-hidden="true" className="shrink-0 rounded bg-sidebar-accent/50 px-1 py-0.5 text-micro font-normal">{collaboration(($) => $.beta.label)}</span></span>
          {!disclosure.open && unread !== undefined && unread > 0 && <output aria-label={`${t(($) => $.sidebar.collaboration_notifications)} ${collaboration(($) => $.page.unread, { count: unread })}`} className="shrink-0 text-caption text-muted-foreground"><CappedNumberFlow value={unread} animated={false} /></output>}
          <ChevronRight aria-hidden className={cn("size-3 transition-transform duration-150 motion-reduce:transition-none", disclosure.open && "rotate-90")} />
        </SidebarGroupLabel>
        <TooltipContent side="right">{collaboration(($) => $.beta.description)}</TooltipContent>
      </Tooltip>
      {disclosure.open && <CollapsibleContent><SidebarGroupContent><SidebarMenu className="gap-0.5">
      {(["current", "participating"] as const).map((view) => {
        const Icon = view === "current" ? Inbox : UsersRound;
        const selected = destination !== null && !destination.administration && destination.view === view;
        return <SidebarMenuItem key={view}><SidebarMenuButton render={<AppLink href={accountPath({ view })} />} isActive={selected} aria-current={selected ? destination.issueId ? "location" : "page" : undefined}
          className="text-muted-foreground data-active:bg-sidebar-accent data-active:font-medium data-active:text-sidebar-accent-foreground data-active:hover:bg-sidebar-accent">
          <Icon aria-hidden /><span>{view === "current" ? collaboration(($) => $.page.current) : collaboration(($) => $.page.participating)}</span>
        </SidebarMenuButton></SidebarMenuItem>;
      })}
      <SidebarMenuItem><SidebarMenuButton render={<AppLink href={accountPath({ view: "notifications" })} />} isActive={notificationsSelected} aria-current={notificationsSelected ? destination.notificationId ? "location" : "page" : undefined}
        aria-label={notificationsLabel} title={notificationsLabel} className="text-muted-foreground data-active:bg-sidebar-accent data-active:font-medium data-active:text-sidebar-accent-foreground data-active:hover:bg-sidebar-accent">
        <Bell aria-hidden /><span className="min-w-0 flex-1 truncate">{t(($) => $.sidebar.collaboration_notifications)}</span>{unread !== undefined && unread > 0 && <CappedNumberFlow value={unread} animated={false} className="ml-auto text-caption" />}
      </SidebarMenuButton></SidebarMenuItem>
      </SidebarMenu></SidebarGroupContent></CollapsibleContent>}
    </Collapsible>
  </SidebarGroup>;
}

interface WorkspacePickerProps {
  workspace: Workspace | null;
  workspaces: readonly Workspace[];
  invitations: Awaited<ReturnType<typeof api.listMyInvitations>>;
  creationDisabled: boolean;
  selectWorkspace(slug: string): void;
  otherUnread?: boolean;
  unreadIds?: ReadonlySet<string>;
}

function WorkspacePicker({ workspace, workspaces, invitations, creationDisabled, selectWorkspace, otherUnread = false, unreadIds }: WorkspacePickerProps) {
  const { t } = useT("layout");
  const { push } = useNavigation();
  const client = useQueryClient();
  const accept = useMutation({ mutationFn: (id: string) => api.acceptInvitation(id), onSuccess: async (_, id) => {
    const invitation = invitations.find((entry) => entry.id === id);
    void client.invalidateQueries({ queryKey: workspaceKeys.myInvitations() });
    const loaded = await client.fetchQuery({ ...workspaceListOptions(), staleTime: 0 });
    const joined = loaded.find((entry) => entry.id === invitation?.workspace_id);
    if (joined) selectWorkspace(joined.slug);
  } });
  const decline = useMutation({ mutationFn: (id: string) => api.declineInvitation(id), onSuccess: () => { void client.invalidateQueries({ queryKey: workspaceKeys.myInvitations() }); } });
  return <SidebarGroup className="pb-0">
    <SidebarGroupLabel>{t(($) => $.sidebar.workspaces_label)}</SidebarGroupLabel>
    <div className="flex min-w-0 items-center gap-1">
      {workspace && <SidebarMenuButton aria-label={workspace.name} onClick={() => selectWorkspace(workspace.slug)} className="min-w-0 flex-1 font-medium">
        <span aria-hidden="true" className="relative"><WorkspaceAvatar name={workspace.name} avatarUrl={workspace.avatar_url} size="sm" />
          {(invitations.length > 0 || otherUnread) && <span className="absolute -top-0.5 -right-0.5 size-2 rounded-full bg-brand ring-1 ring-sidebar" />}
        </span><span className="truncate">{workspace.name}</span>
      </SidebarMenuButton>}
      <DropdownMenu><DropdownMenuTrigger render={<SidebarMenuButton aria-label={t(($) => $.sidebar.switch_workspace)} title={t(($) => $.sidebar.switch_workspace)} className={workspace ? "w-9 shrink-0 justify-center pointer-coarse:min-w-11" : "text-muted-foreground"}>
        {!workspace && <span className="flex-1">{t(($) => $.sidebar.switch_workspace)}</span>}<ChevronDown aria-hidden className="size-4" />
      </SidebarMenuButton>} />
        <DropdownMenuContent align="start" className="min-w-56 max-w-[calc(100vw-2rem)]">
          <DropdownMenuGroup><DropdownMenuLabel>{t(($) => $.sidebar.workspaces_label)}</DropdownMenuLabel>
            {workspaces.map((entry) => <DropdownMenuItem key={entry.id} onClick={() => selectWorkspace(entry.slug)}>
              <WorkspaceAvatar name={entry.name} avatarUrl={entry.avatar_url} size="sm" /><span className="min-w-0 flex-1 truncate">{entry.name}</span>
              {entry.id === workspace?.id ? <Check aria-hidden className="size-3.5" /> : unreadIds?.has(entry.id) && <span className="size-2 rounded-full bg-brand" />}
            </DropdownMenuItem>)}
            {!creationDisabled && <DropdownMenuItem onClick={() => push(paths.newWorkspace())}><Plus aria-hidden />{t(($) => $.sidebar.create_workspace)}</DropdownMenuItem>}
          </DropdownMenuGroup>
          {invitations.length > 0 && <><DropdownMenuSeparator /><DropdownMenuGroup><DropdownMenuLabel>{t(($) => $.sidebar.pending_invitations_label)}</DropdownMenuLabel>
            {invitations.map((invitation) => <div key={invitation.id} className="flex min-w-0 items-center gap-2 px-2 py-1.5"><span className="min-w-0 flex-1 truncate text-body">{invitation.workspace_name ?? t(($) => $.sidebar.invitation_workspace_fallback)}</span>
              <button type="button" disabled={accept.isPending} className="min-h-8 rounded px-2 text-caption hover:bg-accent pointer-coarse:min-h-11" onClick={() => accept.mutate(invitation.id)}>{t(($) => $.sidebar.invitation_join)}</button>
              <button type="button" disabled={decline.isPending} className="min-h-8 rounded px-2 text-caption text-muted-foreground hover:bg-accent pointer-coarse:min-h-11" onClick={() => decline.mutate(invitation.id)}>{t(($) => $.sidebar.invitation_decline)}</button>
            </div>)}
          </DropdownMenuGroup></>}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  </SidebarGroup>;
}

function WorkspaceLinks({ workspace, personalCounts, pinned, isActivePinnedRoute = false, iterationEnabled = false, triageEnabled = false, triageCount }: {
  workspace: Workspace; personalCounts?: { inbox: number }; pinned?: React.ReactNode; isActivePinnedRoute?: boolean;
  iterationEnabled?: boolean; triageEnabled?: boolean; triageCount?: number;
}) {
  const { t } = useT("layout");
  const { pathname } = useNavigation();
  const p = paths.workspace(workspace.slug);
  const renderRows = (items: NavItem[]) => <SidebarMenu className="gap-0.5">{items.map((item) => {
    const href = p[item.key]();
    const count = item.key === "inbox" ? personalCounts?.inbox : item.key === "triage" ? triageCount : undefined;
    return <NavRow key={item.key} href={href} label={item.key === "issues" ? t(($) => $.sidebar.workspace_issues) : t(($) => $.nav[item.labelKey])}
      isActive={!(isActivePinnedRoute && (item.key === "issues" || item.key === "projects")) && isNavActive(pathname, href)}
      trailing={count !== undefined && (count > 0 || item.key === "triage") ? <CappedNumberFlow value={count} animated={false} className="ml-auto text-caption" /> : undefined} />;
  })}</SidebarMenu>;
  return <>
    <SidebarGroup><SidebarGroupContent>{renderRows(personalNav)}</SidebarGroupContent></SidebarGroup>
    {pinned}
    <NavigationGroup group="work" label={t(($) => $.sidebar.work_group)} active={workNav.some((item) => isNavActive(pathname, p[item.key]()))}>
      {renderRows(workNav.filter((item) => (item.key !== "iterations" || iterationEnabled) && (item.key !== "triage" || triageEnabled)))}
    </NavigationGroup>
    <NavigationGroup group="ai" label={t(($) => $.sidebar.ai_team_group)} active={aiTeamNav.some((item) => isNavActive(pathname, p[item.key]()))}>{renderRows(aiTeamNav)}</NavigationGroup>
    <NavigationGroup group="resources" label={t(($) => $.sidebar.resources_group)} active={resourceNav.some((item) => isNavActive(pathname, p[item.key]()))}>{renderRows(resourceNav)}</NavigationGroup>
  </>;
}

/** Cached capabilities may reveal links, but account scope never requests them. */
function WorkspacePreview({ workspace }: { workspace: Workspace }) {
  const { data: iteration } = useQuery({ ...iterationCapabilitiesOptions(workspace.id), enabled: false });
  const { data: triage } = useQuery({ ...triageSettingsOptions(workspace.id), enabled: false });
  return <WorkspaceLinks workspace={workspace}
    iterationEnabled={iteration?.supported === true && iteration.manual === true && iteration.enabled === true}
    triageEnabled={triage?.enabled === true} />;
}

/** Workspace hooks and implicit-header mutations mount only under the active route. */
function WorkspaceNavigation({ workspace, pickerProps }: { workspace: Workspace; pickerProps: WorkspacePickerProps }) {
  const { t } = useT("layout");
  const { pathname } = useNavigation();
  const userId = useAuthStore((state) => state.user?.id);
  const p = useWorkspacePaths();
  const wsId = workspace.id;
  const { data: iterationCapability } = useQuery({ ...iterationCapabilitiesOptions(wsId ?? ""), enabled: !!wsId });
  const { data: triageSettings } = useQuery({ ...triageSettingsOptions(wsId ?? ""), enabled: !!wsId });
  const { data: triageCounts } = useQuery({ ...triageCountsOptions(wsId ?? ""), enabled: !!wsId && triageSettings?.enabled === true });
  const { data: inboxItems = EMPTY_INBOX } = useQuery({
    queryKey: wsId ? inboxKeys.list(wsId) : ["inbox", "disabled"],
    queryFn: () => api.listInbox(),
    enabled: !!wsId,
  });
  const unreadCount = React.useMemo(
    () => deduplicateInboxItems(inboxItems).filter((i) => !i.read).length,
    [inboxItems],
  );
  // Cross-workspace unread summary backs the workspace-switcher dot. One
  // shared cache entry across workspaces; gated on an active workspace since
  // the endpoint resolves through the workspace-member middleware.
  const { data: unreadSummary = EMPTY_INBOX_SUMMARY } = useQuery({
    ...inboxUnreadSummaryOptions(),
    enabled: !!wsId,
  });
  const otherWorkspaceUnread = React.useMemo(
    () => hasOtherWorkspaceUnread(unreadSummary, wsId),
    [unreadSummary, wsId],
  );
  // Which workspaces have unread, so the switcher dropdown can point at the
  // specific one(s) rather than just the aggregate avatar dot.
  const unreadWsIds = React.useMemo(() => unreadWorkspaceIds(unreadSummary), [unreadSummary]);
  const { data: pinnedItems = EMPTY_PINS } = useQuery({
    ...pinListOptions(wsId ?? "", userId ?? ""),
    enabled: !!wsId && !!userId,
  });
  const deletePin = useDeletePin();
  const reorderPins = useReorderPins();
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }));
  const getPinHref = useCallback(
    (pin: PinnedItem) =>
      pin.item_type === "issue"
        ? p.issueDetail(pin.item_id)
        : pin.item_type === "project"
          ? p.projectDetail(pin.item_id)
          // Views know their target only after their detail loads — the row
          // resolves its own href; this placeholder never renders as a link.
          : "",
    [p],
  );

  // Local presentational copy of pinnedItems for drop-animation stability.
  // Follows TQ at rest; frozen during a drag gesture so a mid-drag cache
  // write (our own optimistic update, or a WS refetch) cannot reorder the
  // DOM under dnd-kit while its drop animation is still interpolating.
  const [localPinned, setLocalPinned] = useState<PinnedItem[]>(pinnedItems);
  const [localPinnedWsId, setLocalPinnedWsId] = useState<string | null>(wsId ?? null);
  const isDraggingRef = useRef(false);
  useEffect(() => {
    if (!isDraggingRef.current) {
      setLocalPinned(pinnedItems);
    }
  }, [pinnedItems]);
  useEffect(() => {
    setLocalPinnedWsId(wsId ?? null);
  }, [wsId]);
  const visiblePinned = localPinnedWsId === (wsId ?? null) ? localPinned : EMPTY_PINS;
  // View pins are absent here (their href resolves async): while a view
  // pin is active the plain nav row for its surface stays highlighted too.
  // Accepted — suppressing it would need every view detail lifted up here.
  const isActivePinnedRoute = visiblePinned.some((pin) => pathname === getPinHref(pin));

  const handleDragStart = useCallback(() => {
    isDraggingRef.current = true;
  }, []);
  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      isDraggingRef.current = false;
      const { active, over } = event;
      if (!over || active.id === over.id) return;
      const oldIndex = localPinned.findIndex((p) => p.id === active.id);
      const newIndex = localPinned.findIndex((p) => p.id === over.id);
      if (oldIndex === -1 || newIndex === -1) return;
      const reordered = arrayMove(localPinned, oldIndex, newIndex);
      setLocalPinned(reordered);
      reorderPins.mutate(reordered);
    },
    [localPinned, reorderPins],
  );


  const pinned = <>{visiblePinned.length > 0 && (
            <Collapsible defaultOpen>
              <SidebarGroup className="group/pinned">
                <SidebarGroupLabel
                  render={<CollapsibleTrigger />}
                  className="group/trigger cursor-pointer hover:bg-sidebar-accent/70 hover:text-sidebar-accent-foreground"
                >
                  <span>{t(($) => $.sidebar.pinned_label)}</span>
                  <ChevronRight className="!size-3 ml-1 stroke-[2.5] transition-transform duration-200 group-data-[panel-open]/trigger:rotate-90" />
                  <span className="ml-auto text-micro text-muted-foreground opacity-0 transition-opacity group-hover/pinned:opacity-100">{visiblePinned.length}</span>
                </SidebarGroupLabel>
                <CollapsibleContent>
                  <SidebarGroupContent>
                    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
                      <SortableContext items={visiblePinned.map((p) => p.id)} strategy={verticalListSortingStrategy}>
                        <SidebarMenu className="gap-0.5">
                          {visiblePinned.map((pin: PinnedItem) => (
                            <PinRow
                              key={pin.id}
                              pin={pin}
                              href={getPinHref(pin)}
                              pathname={pathname}
                              onUnpin={() => deletePin.mutate({ itemType: pin.item_type, itemId: pin.item_id })}
                              wsId={wsId ?? ""}
                            />
                          ))}
                        </SidebarMenu>
                      </SortableContext>
                    </DndContext>
                  </SidebarGroupContent>
                </CollapsibleContent>
              </SidebarGroup>
            </Collapsible>
          )}

</>;
  return <>
    <WorkspacePicker {...pickerProps} otherUnread={otherWorkspaceUnread} unreadIds={unreadWsIds} />
    <WorkspaceLinks workspace={workspace} personalCounts={{ inbox: unreadCount }} pinned={pinned} isActivePinnedRoute={isActivePinnedRoute}
      iterationEnabled={iterationCapability?.supported === true && iterationCapability.manual === true && iterationCapability.enabled === true}
      triageEnabled={triageSettings?.enabled === true} triageCount={triageCounts?.ready} />
  </>;
}
