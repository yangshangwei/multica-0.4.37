import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { RESOURCES } from "@multica/views/locales";
import { SidebarVersion, desktopAppVersion } from "./sidebar-version";

const { push } = vi.hoisted(() => ({ push: vi.fn() }));

vi.mock("@multica/views/navigation", () => ({
  useNavigation: () => ({ push }),
}));

function setAppVersion(version: string) {
  Object.defineProperty(window, "desktopAPI", {
    configurable: true,
    value: { appInfo: { version, os: "macos" } },
  });
}

// Keep i18n, paths and menu primitives real: a plain button can navigate while
// being unreachable by menu keys and leaving the popup open after selection.
async function renderVersion(slug: string | null = "acme") {
  const view = render(
    <I18nProvider locale="en" resources={RESOURCES}>
      <WorkspaceSlugProvider slug={slug}>
        <DropdownMenu>
          <DropdownMenuTrigger>Help</DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem>Feedback</DropdownMenuItem>
            <SidebarVersion />
          </DropdownMenuContent>
        </DropdownMenu>
      </WorkspaceSlugProvider>
    </I18nProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "Help" }));
  await screen.findByRole("menu");
  return view;
}

describe("SidebarVersion", () => {
  beforeEach(() => {
    push.mockReset();
    setAppVersion("0.4.40");
  });

  it("shows the running desktop version as a menu item", async () => {
    await renderVersion();

    expect(
      screen.getByRole("menuitem", { name: "Desktop version 0.4.40" }),
    ).toBeInTheDocument();
  });

  it("closes the menu when opening settings updates by click", async () => {
    await renderVersion();

    fireEvent.click(screen.getByText("Desktop version 0.4.40"));

    expect(push).toHaveBeenCalledWith("/acme/settings?tab=updates");
    await waitFor(() => expect(screen.queryByRole("menu")).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Help" })).toHaveAttribute("aria-expanded", "false");
  });

  it("reaches the version with arrow keys and selects it with Enter", async () => {
    await renderVersion();

    fireEvent.keyDown(screen.getByRole("menu"), { key: "Home" });
    const feedback = screen.getByRole("menuitem", { name: "Feedback" });
    await waitFor(() => expect(feedback).toHaveFocus());

    fireEvent.keyDown(feedback, { key: "ArrowDown" });
    const version = screen.getByText("Desktop version 0.4.40");
    await waitFor(() => expect(version).toHaveFocus());
    fireEvent.keyDown(version, { key: "Enter" });
    fireEvent.keyUp(version, { key: "Enter" });

    expect(push).toHaveBeenCalledWith("/acme/settings?tab=updates");
    await waitFor(() => expect(screen.queryByRole("menu")).not.toBeInTheDocument());
  });

  // A dev build's version is a full `git describe` string, wider than the help
  // menu, so the row wraps. The tooltip is then the only place the "check for
  // updates" call to action survives alongside the commit and dirty marker —
  // which is exactly what a bug report needs.
  it("keeps the whole dev version in the tooltip", async () => {
    setAppVersion("0.4.37-2-gabc1234-dirty");
    await renderVersion();

    expect(screen.getByText("Desktop version 0.4.37-2-gabc1234-dirty")).toHaveAttribute(
      "title",
      expect.stringContaining("0.4.37-2-gabc1234-dirty"),
    );
  });

  // `fetchAppInfo` in the preload falls back to the literal "unknown" when the
  // synchronous IPC fails, and to "" if main ever answers with an empty string.
  // Neither is worth a menu row — "unknown" reads as a broken build.
  it.each(["unknown", ""])("renders nothing for version %j", async (version) => {
    setAppVersion(version);
    await renderVersion();

    expect(screen.queryByText(/Desktop version/)).not.toBeInTheDocument();
  });

  // The sidebar only mounts under a resolved workspace, so this is defensive:
  // nothing above it is an error boundary, and a version row is not worth
  // risking a blank window over.
  it("stays unclickable without a workspace in context", async () => {
    await renderVersion(null);

    expect(screen.getByText("Desktop version 0.4.40")).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /Desktop version/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Desktop version/ })).not.toBeInTheDocument();
  });
});

// The layout gates the help menu's version slot on this helper: a null return
// must suppress the slot itself, not just the row — HelpLauncher can't tell an
// element that renders null from a real one, so the two disagreeing would hang
// the version block's separator over nothing.
describe("desktopAppVersion", () => {
  it("returns the reported version", () => {
    setAppVersion("0.4.40");
    expect(desktopAppVersion()).toBe("0.4.40");
  });

  it.each(["unknown", ""])("returns null for %j", (version) => {
    setAppVersion(version);
    expect(desktopAppVersion()).toBeNull();
  });

  it("returns null when the preload bridge is missing", () => {
    Reflect.deleteProperty(window, "desktopAPI");
    expect(desktopAppVersion()).toBeNull();
  });
});
