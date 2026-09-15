import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
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

// Real i18n and real `paths`: the point of this suite is that the key exists in
// the bundle and the pushed path matches what the settings route actually
// registers, neither of which a mocked translator or path builder would catch.
function renderVersion(slug: string | null = "acme") {
  return render(
    <I18nProvider locale="en" resources={RESOURCES}>
      <WorkspaceSlugProvider slug={slug}>
        <SidebarVersion />
      </WorkspaceSlugProvider>
    </I18nProvider>,
  );
}

describe("SidebarVersion", () => {
  beforeEach(() => {
    push.mockReset();
    setAppVersion("0.4.40");
  });

  it("shows the running desktop version", () => {
    renderVersion();

    expect(
      screen.getByRole("button", { name: "Desktop version 0.4.40" }),
    ).toBeInTheDocument();
  });

  it("opens the updates tab of settings when clicked", () => {
    renderVersion();

    fireEvent.click(screen.getByRole("button", { name: "Desktop version 0.4.40" }));

    expect(push).toHaveBeenCalledWith("/acme/settings?tab=updates");
  });

  // A dev build's version is a full `git describe` string, wider than the help
  // menu, so the row wraps. The tooltip is then the only place the "check for
  // updates" call to action survives alongside the commit and dirty marker —
  // which is exactly what a bug report needs.
  it("keeps the whole dev version in the tooltip", () => {
    setAppVersion("0.4.37-2-gabc1234-dirty");
    renderVersion();

    expect(screen.getByRole("button")).toHaveAttribute(
      "title",
      expect.stringContaining("0.4.37-2-gabc1234-dirty"),
    );
  });

  // `fetchAppInfo` in the preload falls back to the literal "unknown" when the
  // synchronous IPC fails, and to "" if main ever answers with an empty string.
  // Neither is worth a menu row — "unknown" reads as a broken build.
  it.each(["unknown", ""])("renders nothing for version %j", (version) => {
    setAppVersion(version);
    const { container } = renderVersion();

    expect(container).toBeEmptyDOMElement();
  });

  // The sidebar only mounts under a resolved workspace, so this is defensive:
  // nothing above it is an error boundary, and a version row is not worth
  // risking a blank window over.
  it("stays unclickable without a workspace in context", () => {
    renderVersion(null);

    expect(screen.getByText("Desktop version 0.4.40")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
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
