import { cloneElement, type ReactElement, type ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { configStore } from "@multica/core/config";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import enLayout from "../locales/en/layout.json";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { isDesktopShell } from "../platform/local-directory";
import { HelpLauncher } from "./help-launcher";

// The download entry is gated on the desktop-shell probe, which reads a
// preload-injected bridge that jsdom never has. Mock it so both platforms are
// reachable from the same suite.
vi.mock("../platform/local-directory", () => ({
  isDesktopShell: vi.fn(() => false),
}));

// react-i18next isn't initialised in the views test env, so resolve the
// selector against the real en/layout.json to assert on actual copy.
vi.mock("../i18n", () => ({
  useT: () => ({
    t: (
      sel: (r: typeof enLayout) => string,
      vars?: Record<string, string>,
    ) => {
      const template = sel(enLayout);
      return vars
        ? template.replace(/\{\{(\w+)\}\}/g, (_, key) => String(vars[key] ?? ""))
        : template;
    },
  }),
}));

// Follows the app-sidebar.test.tsx convention of flattening the Base UI
// dropdown primitives to plain children so the menu content is always in
// the DOM, instead of exercising the real portal/open-state interaction.
//
// The mock deliberately preserves ONE real invariant: DropdownMenuLabel wraps
// Base UI's Menu.GroupLabel, whose useMenuGroupRootContext() throws when it has
// no Menu.Group ancestor. A plain-<div> mock silently swallowed that contract,
// which is exactly how MUL-4819 shipped — a version row rendered outside a
// DropdownMenuGroup crashed the whole app (no error boundary above the sidebar)
// the moment the Help menu opened. Mirroring the throw here keeps the guard.
// The group context lives inside the factory so it survives vi.mock hoisting.
vi.mock("@multica/ui/components/ui/dropdown-menu", async () => {
  const { createContext, useContext } = await import("react");
  const GroupContext = createContext(false);
  return {
    DropdownMenu: ({ children }: { children: ReactNode }) => <>{children}</>,
    DropdownMenuContent: ({ children }: { children: ReactNode }) => <>{children}</>,
    // Base UI's `render` prop swaps in a caller-supplied element (here the
    // <a>) and adopts the item's children. Flattening it to a bare fragment —
    // as this mock originally did — would drop the anchor entirely and make an
    // href assertion silently unfalsifiable.
    DropdownMenuItem: ({
      children,
      render,
    }: {
      children: ReactNode;
      render?: ReactElement;
    }) => (render ? cloneElement(render, undefined, children) : <>{children}</>),
    DropdownMenuGroup: ({ children }: { children: ReactNode }) => (
      <GroupContext.Provider value={true}>{children}</GroupContext.Provider>
    ),
    DropdownMenuLabel: ({ children }: { children: ReactNode }) => {
      if (!useContext(GroupContext)) {
        throw new Error(
          "Base UI: MenuGroupRootContext is missing. Menu group parts must be used within <Menu.Group>.",
        );
      }
      return <div>{children}</div>;
    },
    DropdownMenuSeparator: () => null,
    DropdownMenuTrigger: ({ children }: { children: ReactNode }) => <>{children}</>,
  };
});

// The Docs entry is an in-app destination now, so it needs both the workspace
// slug it addresses and a navigation adapter for AppLink. Rendering without
// them is a real case too — see the last test — so the providers live in a
// helper rather than a blanket wrapper.
function navAdapter(): NavigationAdapter {
  return {
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    pathname: "/acme/issues",
    searchParams: new URLSearchParams(),
    hash: "",
    getShareableUrl: (path: string) => path,
  };
}

function renderHelp(slug: string | null = "acme") {
  const ui = (
    <NavigationProvider value={navAdapter()}>
      <HelpLauncher />
    </NavigationProvider>
  );
  return render(
    slug ? (
      <WorkspaceSlugProvider slug={slug}>{ui}</WorkspaceSlugProvider>
    ) : (
      ui
    ),
  );
}

beforeEach(() => {
  vi.mocked(isDesktopShell).mockReturnValue(false);
});

afterEach(() => {
  configStore.getState().setServerVersion("");
});

describe("HelpLauncher", () => {
  it("does not show a version row when the server omits it", () => {
    renderHelp();
    expect(screen.queryByText(/Server version/)).not.toBeInTheDocument();
  });

  it("shows the server version once /api/config resolves it", () => {
    configStore.getState().setServerVersion("1.2.3");
    renderHelp();
    expect(screen.getByText("Server version 1.2.3")).toBeInTheDocument();
  });

  // MUL-4819: the version row's DropdownMenuLabel must sit inside a
  // DropdownMenuGroup. Rendering it bare made Base UI's Menu.GroupLabel throw
  // on open, unmounting the whole app (black screen, no error) because no error
  // boundary sits above the sidebar. Rendering here must not throw.
  it("renders the version row without a missing-group crash", () => {
    configStore.getState().setServerVersion("9.9.9");
    expect(() => renderHelp()).not.toThrow();
    expect(screen.getByText("Server version 9.9.9")).toBeInTheDocument();
  });

  // MUL-6462: after web onboarding the desktop download CTA was unreachable —
  // no entry anywhere in the app, so users had to remember the URL or detour
  // through the marketing site. The Help menu is the persistent home for it.
  it("links to the download page on web", () => {
    renderHelp();
    const link = screen.getByRole("link", { name: /Desktop app/ });
    expect(link).toHaveAttribute("href", "https://multica.ai/download");
  });

  // AppSidebar is shared: apps/desktop renders the same component tree. Without
  // this gate the desktop app would offer to download the desktop app.
  it("hides the download entry inside the desktop shell", () => {
    vi.mocked(isDesktopShell).mockReturnValue(true);
    renderHelp();
    expect(screen.queryByText("Desktop app")).not.toBeInTheDocument();
    // The rest of the menu is unaffected by the gate.
    expect(screen.getByText("Docs")).toBeInTheDocument();
  });

  // The docs entry was `https://multica.ai/docs` — dead on any deployment
  // without public internet, which is the entire reason in-app docs exist. It is
  // now a workspace-scoped in-app route, and carries no external-link glyph
  // because nothing leaves the app.
  it("points the docs entry at the in-app reader", () => {
    renderHelp("acme");
    const link = screen.getByRole("link", { name: /Docs/ });
    expect(link).toHaveAttribute("href", "/acme/docs");
  });

  // Change log and Desktop app still point at release assets that genuinely are
  // not part of this deployment, so those two stay external.
  it("keeps the change log external", () => {
    renderHelp();
    expect(screen.getByRole("link", { name: /Change log/ })).toHaveAttribute(
      "href",
      "https://multica.ai/changelog",
    );
  });

  // The menu must not be the thing that throws if it is ever mounted outside a
  // workspace route: the entry is dropped, the rest of the menu still renders.
  it("omits the docs entry with no workspace in scope, without crashing", () => {
    expect(() => renderHelp(null)).not.toThrow();
    expect(screen.queryByText("Docs")).not.toBeInTheDocument();
    expect(screen.getByText("Change log")).toBeInTheDocument();
  });

  it("does not include the removed Discord entry", () => {
    renderHelp();
    expect(screen.queryByText("Discord")).not.toBeInTheDocument();
  });
});
