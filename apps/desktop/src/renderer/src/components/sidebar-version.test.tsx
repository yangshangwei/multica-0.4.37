import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { WorkspaceSlugProvider } from "@multica/core/paths";
import { RESOURCES } from "@multica/views/locales";
import { SidebarVersion } from "./sidebar-version";

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

    expect(screen.getByRole("button", { name: "v0.4.40" })).toBeInTheDocument();
  });

  it("opens the updates tab of settings when clicked", () => {
    renderVersion();

    fireEvent.click(screen.getByRole("button", { name: "v0.4.40" }));

    expect(push).toHaveBeenCalledWith("/acme/settings?tab=updates");
  });

  // A dev build's version is a full `git describe` string, far too wide for the
  // sidebar, so the label truncates. The tooltip is then the only place the
  // commit and dirty marker survive — which is exactly what a bug report needs.
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
  // Neither is worth a sidebar row — "vunknown" reads as a broken build.
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

    expect(screen.getByText("v0.4.40")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
