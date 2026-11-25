import type { ReactNode } from "react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { cleanup, fireEvent, render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { RESOURCES } from "@multica/views/locales";

/**
 * Regression guard for MUL-6231 / #7021: deleting the last workspace blanked
 * the desktop client.
 *
 * The shell used to decide whether to mount workspace-scoped chrome from the
 * `getCurrentSlug()` singleton alone. That singleton is mutable process state
 * with no owner keeping it in lockstep with the workspace list, so after the
 * active workspace was deleted it kept returning the dead slug — and the shell
 * kept `SearchCommand` mounted, whose `useWorkspaceId()` THROWS once the
 * workspace leaves the list. Nothing above the shell was an error boundary, so
 * that throw unmounted the entire renderer: a blank, unresponsive window.
 *
 * The invariant these tests lock is "a slug the workspace list no longer
 * contains mounts no workspace-scoped chrome", which is the same signal web's
 * DashboardGuard already gates on.
 */

const state = vi.hoisted(() => ({
  activeScope: "workspace" as "account" | "workspace",
  currentSlug: "acme" as string | null,
  activeWorkspaceSlug: "acme" as string | null,
  lastWorkspaceSlug: "acme" as string | null,
  switchWorkspace: vi.fn(),
  resumeAccount: vi.fn(),
  retryWorkspaceList: vi.fn(),
  workspaceError: false,
  wsList: [] as { id: string; slug: string; name: string }[] | undefined,
}));

vi.mock("@/stores/tab-store", () => ({
  useTabStore: Object.assign(
    (selector: (value: typeof state) => unknown) => selector(state),
    { getState: () => state },
  ),
}));

vi.mock("@/hooks/use-tab-history", () => ({
  useTabHistory: () => ({
    canGoBack: false,
    canGoForward: false,
    goBack: vi.fn(),
    goForward: vi.fn(),
  }),
  useNavigationInputBindings: () => {},
}));

vi.mock("@/platform/navigation", () => ({
  DesktopNavigationProvider: ({ children }: { children: ReactNode }) => (
    <>{children}</>
  ),
  routeContentLinkPath: vi.fn(),
  resumeDesktopScope: state.resumeAccount,
}));

vi.mock("@multica/core/paths", () => ({
  WorkspaceSlugProvider: ({ children, slug }: { children: ReactNode; slug: string | null }) => (
    <div data-testid="workspace-context" data-workspace-slug={slug ?? "none"}>{children}</div>
  ),
  paths: { workspace: () => ({ inbox: () => "/acme/inbox" }) },
  useCurrentWorkspace: () => null,
}));

vi.mock("@multica/core/platform", () => ({
  getCurrentSlug: () => state.currentSlug,
  subscribeToCurrentSlug: () => () => {},
}));

vi.mock("@multica/core/workspace", () => ({
  workspaceListOptions: () => ({
    queryKey: ["workspace-list"],
    queryFn: async () => {
      if (state.workspaceError) throw new Error("Directory unavailable");
      return state.wsList ?? new Promise<never>(() => {});
    },
  }),
}));

vi.mock("@multica/views/navigation", () => ({
  useNavigation: () => ({ push: vi.fn() }),
}));

vi.mock("@multica/views/platform", () => ({
  useDesktopUnreadBadge: () => {},
}));

// Each workspace-scoped component gets a marker so the assertions can tell
// which of them the shell decided to mount.
vi.mock("@multica/views/layout", () => ({
  AppSidebar: ({ navigationWorkspace, onWorkspaceSelect, onAccountResume, workspaceListUnavailable, onRetryWorkspaces }: {
    navigationWorkspace?: { slug: string } | null;
    onWorkspaceSelect?: (slug: string) => void;
    onAccountResume?: () => void;
    workspaceListUnavailable?: boolean;
    onRetryWorkspaces?: () => void;
  }) => <nav data-testid="app-sidebar" data-navigation-workspace={navigationWorkspace?.slug ?? "none"}>
    <button type="button" onClick={() => onWorkspaceSelect?.("other")}>Resume other workspace</button>
    <button onClick={onAccountResume}>Return to previous collaboration</button>
    {workspaceListUnavailable && <button type="button" onClick={onRetryWorkspaces}>Retry workspaces</button>}
  </nav>,
  GlobalShortcuts: () => <div data-testid="global-shortcuts" />,
  NavigationProgress: () => <div data-testid="navigation-progress" />,
}));

vi.mock("@multica/views/modals/registry", () => ({
  ModalRegistry: () => <div data-testid="modal-registry" />,
  FeedbackModalHost: () => <div data-testid="feedback-modal-host" />,
}));

// Stands in for the real SearchCommand, which calls useWorkspaceId() at the
// top of its body. Mounting it without a resolvable workspace is precisely
// the crash this gate prevents, so the stub throws the same way.
vi.mock("@multica/views/search", () => ({
  SearchCommand: () => {
    const resolved = state.wsList?.some((w) => w.slug === state.currentSlug);
    if (!resolved) {
      throw new Error(
        "useWorkspaceId: no workspace selected — ensure component renders inside a workspace route",
      );
    }
    return <div data-testid="search-command" />;
  },
  SearchTrigger: () => null,
}));

vi.mock("@multica/views/chat", () => ({
  FloatingChat: () => <div data-testid="floating-chat" />,
}));

vi.mock("./tab-bar", () => ({ TabBar: () => null }));
vi.mock("./window-overlay", () => ({ WindowOverlay: () => null }));
vi.mock("./tab-content", () => ({
  TabContent: () => <div data-testid="tab-content" />,
}));

const { DesktopShell } = await import("./desktop-layout");

function renderShell() {
  (
    window as unknown as { desktopAPI: Record<string, unknown> }
  ).desktopAPI = {
    onNavigationGesture: () => () => {},
    onInboxOpen: () => () => {},
  };

  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  if (state.wsList !== undefined) qc.setQueryData(["workspace-list"], state.wsList);

  return render(
    <QueryClientProvider client={qc}>
      <I18nProvider locale="en" resources={RESOURCES}>
        <DesktopShell onRetryWorkspaceList={state.retryWorkspaceList} />
      </I18nProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.activeScope = "workspace";
  state.currentSlug = "acme";
  state.activeWorkspaceSlug = "acme";
  state.lastWorkspaceSlug = "acme";
  state.wsList = [{ id: "ws-1", slug: "acme", name: "Acme" }];
  state.workspaceError = false;
  state.switchWorkspace.mockClear();
  state.retryWorkspaceList.mockClear();
});

describe("DesktopShell workspace gating", () => {
  it("drops workspace chrome before a late route mirror clears on account entry", () => {
    state.activeScope = "account";

    const { queryByTestId } = renderShell();

    expect(queryByTestId("tab-content")).not.toBeNull();
    expect(queryByTestId("app-sidebar")).not.toBeNull();
    expect(queryByTestId("search-command")).toBeNull();
    expect(queryByTestId("global-shortcuts")).toBeNull();
    expect(queryByTestId("feedback-modal-host")).not.toBeNull();
    expect(queryByTestId("modal-registry")).toBeNull();
  });

  it("keeps the sidebar toggle and top-row native clearance in account scope", () => {
    state.activeScope = "account";

    const { container, getByRole } = renderShell();

    expect(container.querySelector("header")).toHaveStyle({ paddingLeft: "0px" });
    expect(getByRole("button", { name: "Toggle left sidebar" })).toBeInTheDocument();
    expect(container.querySelector("[data-slot=window-toolbar]")).toHaveClass("left-0", "top-0", "pl-23");
  });

  it("mounts workspace-scoped chrome while the slug resolves", () => {
    const { queryByTestId } = renderShell();

    expect(queryByTestId("app-sidebar")).not.toBeNull();
    expect(queryByTestId("search-command")).not.toBeNull();
    expect(queryByTestId("global-shortcuts")).not.toBeNull();
    expect(queryByTestId("modal-registry")).not.toBeNull();
    expect(queryByTestId("feedback-modal-host")).toBeNull();
    expect(queryByTestId("floating-chat")).not.toBeNull();
  });

  it("drops workspace-scoped chrome when the singleton still points at a deleted workspace", () => {
    // Exactly the post-delete window: the list refetch has landed empty, but
    // nothing has cleared the slug singleton yet.
    state.wsList = [];

    const { queryByTestId } = renderShell();

    expect(queryByTestId("app-sidebar")).not.toBeNull();
    expect(queryByTestId("search-command")).toBeNull();
    expect(queryByTestId("global-shortcuts")).toBeNull();
    expect(queryByTestId("modal-registry")).toBeNull();
    expect(queryByTestId("floating-chat")).toBeNull();
  });

  it("keeps TabContent mounted with no workspace so the tab router can still resolve one", () => {
    state.wsList = [];

    const { queryByTestId } = renderShell();

    expect(queryByTestId("tab-content")).not.toBeNull();
  });

  it.each(["workspace", "account"] as const)("keeps one directory inside the sidebar shell in %s scope", (scope) => {
    state.activeScope = scope;
    if (scope === "account") state.wsList = [];
    const { getByTestId, container } = renderShell();
    const sidebar = getByTestId("app-sidebar");
    const canvas = getByTestId("tab-content").parentElement!;
    const sidebarWrapper = container.querySelector('[data-slot="sidebar-wrapper"]')!;
    expect(canvas.contains(sidebar)).toBe(false);
    expect(sidebarWrapper.contains(sidebar)).toBe(true);
    expect(sidebarWrapper).toHaveClass("min-h-0");
    expect(container.querySelector('[data-slot="global-scope-bar"]')).toBeNull();
  });

  it("shows the remembered workspace only as navigation while account context remains null", () => {
    state.activeScope = "account";
    state.activeWorkspaceSlug = null;
    state.lastWorkspaceSlug = "other";
    state.wsList = [{ id: "ws-1", slug: "acme", name: "Acme" }, { id: "ws-2", slug: "other", name: "Other" }];
    const { getByTestId, getByRole, queryByTestId } = renderShell();
    expect(getByTestId("workspace-context")).toHaveAttribute("data-workspace-slug", "none");
    expect(getByTestId("app-sidebar")).toHaveAttribute("data-navigation-workspace", "other");
    expect(queryByTestId("search-command")).toBeNull();
    expect(queryByTestId("modal-registry")).toBeNull();
    fireEvent.click(getByRole("button", { name: "Resume other workspace" }));
    expect(state.switchWorkspace).toHaveBeenCalledExactlyOnceWith("other");
  });

  it("uses an available workspace if the remembered one was removed", () => {
    state.activeScope = "account";
    state.lastWorkspaceSlug = "gone";
    const { getByTestId } = renderShell();
    expect(getByTestId("app-sidebar")).toHaveAttribute("data-navigation-workspace", "acme");
  });

  it("keeps account navigation available while the workspace directory is pending", () => {
    state.activeScope = "account";
    state.wsList = undefined;
    const { getByTestId, queryByTestId } = renderShell();
    expect(getByTestId("app-sidebar")).toHaveAttribute("data-navigation-workspace", "none");
    expect(getByTestId("tab-content")).toBeInTheDocument();
    expect(queryByTestId("search-command")).toBeNull();
  });

  it("retains account navigation and independent retry when the workspace directory fails", async () => {
    state.activeScope = "account";
    state.wsList = undefined;
    state.workspaceError = true;
    const { getByRole, getByTestId } = renderShell();
    await waitFor(() => expect(getByRole("button", { name: "Retry workspaces" })).toBeVisible());
    expect(getByTestId("tab-content")).toBeInTheDocument();
    fireEvent.click(getByRole("button", { name: "Retry workspaces" }));
    expect(state.retryWorkspaceList).toHaveBeenCalledOnce();
  });

  // The shell had no navigation feedback at all before MUL-6404 — the bar
  // only ever shipped inside web's DashboardLayout. It sits beside TabContent
  // in the canvas and, like it, is not workspace-gated: a cold workspace
  // resolve is exactly when the wait is longest.
  it("mounts the navigation progress bar, workspace or not", () => {
    expect(renderShell().queryByTestId("navigation-progress")).not.toBeNull();

    cleanup();
    state.wsList = [];

    expect(renderShell().queryByTestId("navigation-progress")).not.toBeNull();
  });

  it("drops chrome for a slug belonging to a workspace the user lost access to", () => {
    state.currentSlug = "gone";
    state.wsList = [{ id: "ws-1", slug: "acme", name: "Acme" }];

    const { queryByTestId } = renderShell();

    expect(queryByTestId("app-sidebar")).not.toBeNull();
    expect(queryByTestId("search-command")).toBeNull();
  });
});

it("wires collaboration return to the existing no-path native resume operation", () => {
  state.resumeAccount.mockClear();
  const view = renderShell();
  fireEvent.click(view.getByRole("button", { name: "Return to previous collaboration" }));
  expect(state.resumeAccount).toHaveBeenCalledExactlyOnceWith("account");
  expect(state.switchWorkspace).not.toHaveBeenCalled();
});
