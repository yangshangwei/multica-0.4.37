import { cloneElement, type ReactElement, type ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { configStore } from "@multica/core/config";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import enLayout from "../locales/en/layout.json";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { HelpLauncher } from "./help-launcher";

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

  // Intranet trim: the desktop-download entry pointed at multica.ai release
  // assets, which an intranet deployment cannot reach. It is gone entirely
  // rather than left as a dead link — on web and desktop alike.
  it("does not include the desktop download entry", () => {
    renderHelp();
    expect(screen.queryByText("Desktop app")).not.toBeInTheDocument();
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

  it("opens changelog from the configured deployment inside the app", () => {
    renderHelp();
    expect(screen.getByRole("link", { name: "Changelog" })).toHaveAttribute("href", "/acme/changelog");
    expect(screen.getByRole("link", { name: "Changelog" })).not.toHaveAttribute("target");
  });

  // The menu must not be the thing that throws if it is ever mounted outside a
  // workspace route: the entry is dropped, the rest of the menu still renders.
  it("omits the docs entry with no workspace in scope, without crashing", () => {
    expect(() => renderHelp(null)).not.toThrow();
    expect(screen.queryByText("Docs")).not.toBeInTheDocument();
    expect(screen.queryByText("Changelog")).not.toBeInTheDocument();
    expect(screen.getByText("Feedback")).toBeInTheDocument();
  });

  it("does not include the removed Discord entry", () => {
    renderHelp();
    expect(screen.queryByText("Discord")).not.toBeInTheDocument();
  });
});
