import { useEffect, useLayoutEffect, useRef, useSyncExternalStore } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { animate, motion, useMotionValue, type MotionStyle } from "motion/react";
import { useQuery } from "@tanstack/react-query";
import { cn } from "@multica/ui/lib/utils";
import {
  useNavigationInputBindings,
  useTabHistory,
} from "@/hooks/use-tab-history";
import {
  SidebarProvider,
  SidebarTrigger,
  useSidebar,
} from "@multica/ui/components/ui/sidebar";
import { ModalRegistry } from "@multica/views/modals/registry";
import {
  AppSidebar,
  GlobalShortcuts,
  NavigationProgress,
} from "@multica/views/layout";
import { SearchCommand, SearchTrigger } from "@multica/views/search";
import { FloatingChat } from "@multica/views/chat";
import { WorkspaceSlugProvider, paths, useCurrentWorkspace } from "@multica/core/paths";
import { workspaceListOptions } from "@multica/core/workspace";
import {
  useNavigation,
  type LinkClickIntent,
} from "@multica/views/navigation";
import { getCurrentSlug, subscribeToCurrentSlug } from "@multica/core/platform";
import { useDesktopUnreadBadge } from "@multica/views/platform";
import {
  DesktopNavigationProvider,
  routeContentLinkPath,
} from "@/platform/navigation";
import { TabBar } from "./tab-bar";
import { TabContent } from "./tab-content";
import { SidebarVersion, desktopAppVersion } from "./sidebar-version";
import { WindowOverlay } from "./window-overlay";
import { UpdateNotificationNavigationBridge } from "./update-notification";

const TOP_BAR_HEIGHT_CLASS = "h-12";
const WINDOW_TOOLBAR_CLEARANCE = 184;
const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";
const toolbarMotion = {
  type: "spring",
  stiffness: 420,
  damping: 38,
  mass: 0.8,
} as const;

// Motion's hook snapshots at mount; the desktop shell also needs to respond
// when the OS preference changes while the window stays open.
function subscribeToReducedMotion(onChange: () => void) {
  const query = window.matchMedia(REDUCED_MOTION_QUERY);
  query.addEventListener("change", onChange);
  return () => query.removeEventListener("change", onChange);
}

function getReducedMotionPreference() {
  return window.matchMedia(REDUCED_MOTION_QUERY).matches;
}

function useShellGeometry(target: number, shouldReduceMotion: boolean) {
  const value = useMotionValue(target);

  useLayoutEffect(() => {
    if (shouldReduceMotion) {
      // Changing transition options does not cancel an in-flight spring
      // whose target is unchanged. Jump also stops it and resets velocity.
      value.jump(target);
      return;
    }
    if (value.get() === target) return;
    const animation = animate(value, target, toolbarMotion);
    return () => animation.stop();
  }, [target, shouldReduceMotion, value]);

  return value;
}

function WindowToolbar() {
  const { canGoBack, canGoForward, goBack, goForward } = useTabHistory();
  const navButtonClassName =
    "flex size-7 items-center justify-center rounded-md text-faint-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground disabled:pointer-events-none disabled:opacity-30";

  return (
    <div
      className={cn(
        "fixed left-0 top-0 z-30 flex w-[184px] shrink-0 items-center px-3",
        TOP_BAR_HEIGHT_CLASS,
      )}
      style={{ WebkitAppRegion: "drag" } as React.CSSProperties}
    >
      <div
        className="flex items-center gap-1 pl-[70px]"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
      >
        <SidebarTrigger
          className="size-7 text-faint-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
          style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
        />
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={goBack}
            disabled={!canGoBack}
            aria-label="Go back"
            title="Go back"
            className={navButtonClassName}
            style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
          >
            <ChevronLeft className="size-4" />
          </button>
          <button
            type="button"
            onClick={goForward}
            disabled={!canGoForward}
            aria-label="Go forward"
            title="Go forward"
            className={navButtonClassName}
            style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
          >
            <ChevronRight className="size-4" />
          </button>
        </div>
      </div>
    </div>
  );
}

function SidebarTopSpacer() {
  return <div className={cn("shrink-0", TOP_BAR_HEIGHT_CLASS)} />;
}

function useNativeNavigationGestures() {
  const { goBack, goForward } = useTabHistory();

  useEffect(() => {
    return window.desktopAPI.onNavigationGesture((gesture) => {
      if (gesture === "back") {
        goBack();
      } else {
        goForward();
      }
    });
  }, [goBack, goForward]);
}


// The main area's top bar doubles as a window drag region. When the sidebar
// is not occupying main-flow width, leave room for the fixed window toolbar
// so tabs do not land beneath the traffic lights / navigation controls.
function MainTopBar({ shouldReduceMotion }: { shouldReduceMotion: boolean }) {
  const { state, isCompact } = useSidebar();
  const sidebarHidden = state === "collapsed" || isCompact;
  const toolbarOffset = useShellGeometry(
    sidebarHidden ? WINDOW_TOOLBAR_CLEARANCE : 0,
    shouldReduceMotion,
  );

  return (
    <motion.header
      style={{ paddingLeft: toolbarOffset }}
      className={cn("relative shrink-0 flex items-center gap-2", TOP_BAR_HEIGHT_CLASS)}
    >
      <motion.div
        aria-hidden
        className="absolute inset-y-0 right-0"
        style={{ left: toolbarOffset, WebkitAppRegion: "drag" } as MotionStyle}
      />
      <div className="relative z-10 flex h-full min-w-0 max-w-full items-center">
        <TabBar />
      </div>
    </motion.header>
  );
}

// The canvas hugs the expanded sidebar with a hairline gap. When the sidebar
// leaves the main flow, the left margin must grow to mirror the fixed mr-2 so
// the floating canvas sits symmetrically inside the window frame.
function MainCanvas({ children, shouldReduceMotion }: {
  children: React.ReactNode;
  shouldReduceMotion: boolean;
}) {
  const { state, isCompact } = useSidebar();
  const sidebarHidden = state === "collapsed" || isCompact;
  const marginLeft = useShellGeometry(sidebarHidden ? 8 : 2, shouldReduceMotion);

  return (
    <motion.div
      style={{ marginLeft }}
      className="relative flex flex-1 min-h-0 flex-col overflow-hidden mr-2 mb-2 rounded-xl bg-page-canvas ring-1 ring-surface-border shadow-[var(--surface-shadow)]"
    >
      {children}
    </motion.div>
  );
}

function useInternalLinkHandler() {
  useEffect(() => {
    const handler = (e: Event) => {
      const detail = (
        e as CustomEvent<{ path?: string; disposition?: LinkClickIntent }>
      ).detail;
      if (!detail?.path) return;
      routeContentLinkPath(detail.path, detail.disposition);
    };
    window.addEventListener("multica:navigate", handler);
    return () => window.removeEventListener("multica:navigate", handler);
  }, []);
}

/**
 * Bridge between the renderer and the Electron main process for inbox-level
 * OS integration. Mounted inside WorkspaceSlugProvider so it can resolve the
 * current workspace's id for the badge hook.
 *
 * Two responsibilities:
 *   1. Mirror the unread inbox count onto the dock/taskbar badge.
 *   2. When the user clicks an OS notification, open the notified
 *      workspace's inbox focused on that item. The route uses the `slug`
 *      that the notification was *emitted* with — not the currently active
 *      workspace — so a notification from workspace A always opens A's
 *      inbox even if the user has since switched to workspace B. Marking
 *      the row read is handled by InboxPage's selected-item effect, which
 *      covers both click-to-select and URL-param-select paths.
 *
 * The click routes through `useNavigation().push` — NOT the
 * `multica:navigate` event, whose handler `openTab`s into the ACTIVE
 * workspace's tab group. The navigation adapter detects a cross-workspace
 * path and translates it into `switchWorkspace(slug, path)`, so clicking a
 * workspace-A notification while B is active performs a real workspace
 * switch instead of mounting A's inbox inside B's tab group (#3766).
 */
function DesktopInboxBridge() {
  const workspace = useCurrentWorkspace();
  useDesktopUnreadBadge(workspace?.id ?? null);
  const { push } = useNavigation();
  // The adapter identity changes with the active tab's location; the ref
  // keeps the main-process subscription stable across navigations.
  const pushRef = useRef(push);
  useEffect(() => {
    pushRef.current = push;
  }, [push]);

  useEffect(() => {
    return window.desktopAPI.onInboxOpen(({ slug, issueKey }) => {
      if (!slug) return;
      const inboxPath = `${paths.workspace(slug).inbox()}?issue=${encodeURIComponent(issueKey)}`;
      pushRef.current(inboxPath);
    });
  }, []);

  return null;
}

export function DesktopShell() {
  useInternalLinkHandler();
  useNativeNavigationGestures();
  useNavigationInputBindings();
  const shouldReduceMotion = useSyncExternalStore(
    subscribeToReducedMotion,
    getReducedMotionPreference,
    () => false,
  );

  // Reactive read of current workspace slug from the platform singleton.
  // On first mount, it is null until WorkspaceRouteLayout (inside the tab
  // router) sets it. Once set, the sidebar and other shell-level components
  // can resolve workspace-scoped paths via useWorkspacePaths().
  const currentSlug = useSyncExternalStore(
    subscribeToCurrentSlug,
    getCurrentSlug,
    () => null,
  );
  // Chrome gates on "the slug still resolves to a workspace", NOT on "the
  // singleton is non-null" (MUL-6231 / #7021). The singleton is mutable
  // process state that no single owner keeps in lockstep with the workspace
  // list, so after the active workspace is deleted it can still hold the dead
  // slug for a beat. Everything below mounts workspace-scoped components —
  // SearchCommand calls useWorkspaceId(), which THROWS when the workspace is
  // gone from the list. Nothing above this in the desktop tree is an error
  // boundary, so that throw used to unmount the whole renderer and leave a
  // blank, unresponsive window.
  //
  // Deriving from the list cache makes this the same gate web uses
  // (DashboardGuard's `!workspace` check in packages/views/layout), so both
  // shells drop workspace-scoped chrome on exactly the same signal instead of
  // diverging. TabContent stays outside the gate: it must always render so
  // the tab router can mount WorkspaceRouteLayout, which is what populates
  // the singleton in the first place.
  const { data: workspaces = [] } = useQuery(workspaceListOptions());
  const slug =
    currentSlug && workspaces.some((w) => w.slug === currentSlug)
      ? currentSlug
      : null;

  return (
    <DesktopNavigationProvider>
      {/* WorkspaceSlugProvider accepts null — components that need slug
          use useWorkspaceSlug() (nullable) or useRequiredWorkspaceSlug()
          (throws). TabContent MUST always render so the tab router can
          mount WorkspaceRouteLayout, which calls setCurrentWorkspace()
          to populate the slug. The sidebar gates on the resolved slug
          (see above) to avoid the useRequiredWorkspaceSlug and
          useWorkspaceId throws. Zero-workspace users see the
          window-level overlay (new-workspace flow) triggered by
          IndexRedirect, not a route. */}
      <WorkspaceSlugProvider slug={slug}>
        <UpdateNotificationNavigationBridge workspaceSlug={slug} />
        <DesktopInboxBridge />
        <div className="flex h-screen bg-app-shell">
          {/* bg-app-shell is the wrapper's non-inset fill, so it also owns the
              non-inset half of --sidebar-wrapper-fill. sidebar.tsx supplies the
              inset half of both. Anything that has to paint an opaque layer
              over this wrapper (the tab flares) reads the variable rather than
              re-deriving which of the two is in play. */}
          {/* hasExternalTrigger: WindowToolbar below parks a SidebarTrigger
              beside the traffic lights, where it is always reachable. Page
              headers inside the canvas must not add their own fallback one on
              top of it — desktop windows sit below `xl`, exactly where that
              fallback renders, so every page showed a second identical icon
              50px under this one (MUL-6218). */}
          <SidebarProvider
            hasExternalTrigger
            className="flex-1 bg-app-shell [--sidebar-wrapper-fill:var(--app-shell)]"
          >
            {slug && <GlobalShortcuts />}
            {slug && <WindowToolbar />}
            {slug && (
              <AppSidebar
                topSlot={<SidebarTopSpacer />}
                searchSlot={<SearchTrigger />}
                versionSlot={desktopAppVersion() ? <SidebarVersion /> : undefined}
              />
            )}
            {/* Right side: header + content container */}
            <div className="flex flex-1 min-w-0 flex-col">
              <MainTopBar shouldReduceMotion={shouldReduceMotion} />
              <MainCanvas shouldReduceMotion={shouldReduceMotion}>
                {/* Same indicator, same anchor as web: DashboardLayout puts it
                    at the top of SidebarInset, and MainCanvas is desktop's
                    equivalent relative/overflow-hidden content box. Desktop
                    used to have no navigation feedback at all — a click just
                    froze until the destination committed (MUL-6404). */}
                <NavigationProgress />
                <TabContent />
                {slug && <FloatingChat />}
              </MainCanvas>
            </div>
          </SidebarProvider>
        </div>
        {slug && <ModalRegistry />}
        {slug && <SearchCommand />}
        <WindowOverlay />
      </WorkspaceSlugProvider>
    </DesktopNavigationProvider>
  );
}
