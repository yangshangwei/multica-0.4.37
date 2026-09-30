import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SidebarProvider, useSidebar } from "@multica/ui/components/ui/sidebar";
import { configStore } from "@multica/core/config";
import {
  BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG,
  PLUGINS_V1_FLAG,
} from "@multica/core/feature-flags";
import { renderWithI18n } from "../../test/i18n";

// This file tests the settings SHELL — the chrome around the tabs — so every
// tab panel is stubbed out. Their contents have their own test files.
const stub = vi.hoisted(
  () => (name: string) => () => ({ [name]: () => <div>{name}</div> }),
);
vi.mock("./account-tab", stub("AccountTab"));
vi.mock("./preferences-tab", stub("PreferencesTab"));
vi.mock("./tokens-tab", stub("TokensTab"));
vi.mock("./workspace-tab", stub("WorkspaceTab"));
vi.mock("./members-tab", stub("MembersTab"));
vi.mock("./repositories-tab", stub("RepositoriesTab"));
vi.mock("./integrations-tab", stub("IntegrationsTab"));
vi.mock("./notifications-tab", stub("NotificationsTab"));
vi.mock("./labels-tab", stub("LabelsTab"));
vi.mock("./properties-tab", stub("PropertiesTab"));
vi.mock("./quick-actions-tab", stub("QuickActionsTab"));
vi.mock("./keyboard-shortcuts-tab", stub("KeyboardShortcutsTab"));
vi.mock("./plugins-tab", stub("PluginsTab"));
vi.mock("./billing-tab", stub("BillingTab"));
vi.mock("./mcp-tab", stub("McpTab"));

const replace = vi.fn();
const navigationState = { search: "" };
vi.mock("../../navigation", () => ({
  useNavigation: () => ({
    searchParams: new URLSearchParams(navigationState.search),
    hash: "",
    pathname: "/acme/settings",
    replace,
  }),
}));

const layout = { compact: false };
vi.mock("@multica/ui/hooks/use-mobile", () => ({
  useIsMobile: () => layout.compact,
  useIsCompact: () => layout.compact,
}));

import { SettingsPage } from "./settings-page";

function NavStateProbe() {
  const { openMobile } = useSidebar();
  return <div data-testid="nav-open">{String(openMobile)}</div>;
}

function trigger() {
  return screen.getByRole("button", { name: "Toggle left sidebar" });
}

beforeEach(() => {
  layout.compact = false;
  navigationState.search = "";
  configStore.getState().setFeatureFlags({});
  replace.mockClear();
});

describe("SettingsPage nav trigger", () => {
  it("opens the nav from settings at compact widths", () => {
    layout.compact = true;
    // Settings builds its own chrome instead of a PageHeader, so without this
    // control a touch user who lands here has no way back to the nav at all —
    // the keyboard shortcut is not an answer on a tablet.
    renderWithI18n(
      <SidebarProvider>
        <NavStateProbe />
        <SettingsPage />
      </SidebarProvider>,
    );

    expect(screen.getByTestId("nav-open").textContent).toBe("false");

    fireEvent.click(trigger());

    expect(screen.getByTestId("nav-open").textContent).toBe("true");
  });

  it("hides the trigger only where the nav is a permanent column", () => {
    // The nav is in-flow from `xl` up, so the control is CSS-gated rather than
    // unmounted — jsdom applies no stylesheet, hence the class assertion.
    renderWithI18n(
      <SidebarProvider>
        <SettingsPage />
      </SidebarProvider>,
    );

    expect(trigger().className).toContain("xl:hidden");
  });

  it("still renders standalone, without a sidebar around it", () => {
    // Desktop mounts settings inside its own shell; the trigger has to no-op
    // rather than throw when there is no SidebarProvider above it.
    renderWithI18n(<SettingsPage />);

    expect(
      screen.queryByRole("button", { name: "Toggle left sidebar" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Settings")).toBeInTheDocument();
  });
});

describe("SettingsPage nav groups", () => {
  // The full ordering matrix — groups, items, flag gating, injected tabs —
  // is pinned canonically in settings-nav.test.ts (node environment). This
  // suite covers only what the DOM shell adds on top: the headings render,
  // collapsed tabs are gone, and legacy URLs land on their new home.
  it("renders the four group headings", () => {
    renderWithI18n(<SettingsPage />);

    expect(screen.getByText("Personal")).toBeInTheDocument();
    expect(screen.getByText("Workspace")).toBeInTheDocument();
    expect(screen.getByText("Issues")).toBeInTheDocument();
    expect(screen.getByText("Connections")).toBeInTheDocument();
  });

  it("names the vertical tab list and exposes labelled groups", () => {
    renderWithI18n(<SettingsPage />);

    const navigation = screen.getByRole("tablist", { name: "Settings" });
    expect(navigation).toHaveAttribute("aria-orientation", "vertical");
    const personal = within(navigation).getByRole("group", { name: "Personal" });
    expect(within(personal).getByRole("tab", { name: "Profile" })).toBeInTheDocument();
    expect(within(navigation).getByRole("heading", { name: "Personal" })).toBeInTheDocument();
  });

  it("moves focus with vertical arrows across group boundaries before activation", async () => {
    const user = userEvent.setup();
    renderWithI18n(<SettingsPage />);

    act(() => screen.getByRole("tab", { name: "API Tokens" }).focus());
    await user.keyboard("{ArrowDown}");
    expect(screen.getByRole("tab", { name: "General" })).toHaveFocus();
    expect(replace).not.toHaveBeenCalled();
    await user.keyboard("{Enter}");
    expect(replace).toHaveBeenCalledWith("/acme/settings?tab=workspace");
    await user.keyboard("{ArrowUp}");
    expect(screen.getByRole("tab", { name: "API Tokens" })).toHaveFocus();
  });

  it("no longer offers the collapsed Issue/Chat/GitHub/Labs tabs", () => {
    renderWithI18n(<SettingsPage />);

    for (const name of ["Issue", "Chat", "GitHub", "Labs"]) {
      expect(screen.queryByRole("tab", { name })).not.toBeInTheDocument();
    }
  });

  it("lands the legacy ?tab=github URL on the Integrations tab", () => {
    // The Go backend redirects GitHub App installs back to
    // /settings?tab=github (handler/github.go, githubSettingsURL); GitHub
    // settings now live inside Integrations.
    navigationState.search = "tab=github";

    renderWithI18n(<SettingsPage />);

    expect(screen.getByText("IntegrationsTab")).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "Integrations" }),
    ).toHaveAttribute("aria-selected", "true");
  });

  it.each(["labs", "chat", "issue"])(
    "falls back to the default tab for ?tab=%s",
    (legacy) => {
      navigationState.search = `tab=${legacy}`;

      renderWithI18n(<SettingsPage />);

      expect(screen.getByText("AccountTab")).toBeInTheDocument();
    },
  );
});

describe("SettingsPage compact directory", () => {
  beforeEach(() => {
    layout.compact = true;
    navigationState.search = "tab=preferences&from=shortcut";
  });

  it("shows the current destination and selects a grouped entry without losing query context", async () => {
    const user = userEvent.setup();
    const view = renderWithI18n(<SettingsPage />);
    const directory = screen.getByRole("button", { name: "Settings directory: Preferences" });
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
    expect(screen.getByRole("tabpanel", { name: "Preferences" })).toBeInTheDocument();

    await user.click(directory);
    const dialog = screen.getByRole("dialog", { name: "Settings directory" });
    const navigation = within(dialog).getByRole("tablist", { name: "Settings directory" });
    expect(navigation).toHaveAttribute("aria-orientation", "vertical");
    expect(within(navigation).getByRole("tab", { name: "Preferences" })).toHaveFocus();
    const connections = within(navigation).getByRole("group", { name: "Connections" });
    await user.click(within(connections).getByRole("tab", { name: "MCP" }));
    expect(replace).toHaveBeenCalledWith("/acme/settings?tab=mcp&from=shortcut");
    navigationState.search = "tab=mcp&from=shortcut";
    view.rerender(<SettingsPage />);

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Settings directory: MCP" })).toHaveFocus();
    expect(screen.getByRole("tabpanel", { name: "MCP" })).toBeInTheDocument();
  });

  it("dismisses on Escape or the current tab and restores focus to the directory button", async () => {
    const user = userEvent.setup();
    renderWithI18n(<SettingsPage />);
    const directory = screen.getByRole("button", { name: "Settings directory: Preferences" });

    await user.click(directory);
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(directory).toHaveFocus();
    expect(replace).not.toHaveBeenCalled();

    await user.click(directory);
    await user.click(screen.getByRole("tab", { name: "Preferences" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(directory).toHaveFocus();
    expect(replace).not.toHaveBeenCalled();
  });

  it("localizes the directory name and close control", async () => {
    const user = userEvent.setup();
    renderWithI18n(<SettingsPage />, { locale: "zh-Hans" });
    await user.click(screen.getByRole("button", { name: "设置目录: 偏好设置" }));
    expect(screen.getByRole("dialog", { name: "设置目录" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "关闭设置目录" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("closes an open directory when resizing to desktop and does not reopen on return", async () => {
    const user = userEvent.setup();
    const view = renderWithI18n(<SettingsPage />);
    await user.click(screen.getByRole("button", { name: "Settings directory: Preferences" }));

    layout.compact = false;
    view.rerender(<SettingsPage />);
    expect(screen.getByRole("tablist", { name: "Settings" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Preferences" })).toHaveFocus();

    layout.compact = true;
    view.rerender(<SettingsPage />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Settings directory: Preferences" })).toHaveAttribute("aria-expanded", "false");
  });
});

describe("SettingsPage Plugin feature flag", () => {
  it("hides Plugins and falls back from a direct tab URL when disabled", () => {
    navigationState.search = "tab=plugins";

    renderWithI18n(<SettingsPage />);

    expect(screen.queryByRole("tab", { name: "Plugins" })).not.toBeInTheDocument();
    expect(screen.queryByText("PluginsTab")).not.toBeInTheDocument();
    expect(screen.getByText("AccountTab")).toBeInTheDocument();
  });

  it("shows and mounts Plugins when explicitly enabled", () => {
    navigationState.search = "tab=plugins";
    configStore.getState().setFeatureFlags({ [PLUGINS_V1_FLAG]: true });

    renderWithI18n(<SettingsPage />);

    expect(screen.getByRole("tab", { name: "Plugins" })).toBeInTheDocument();
    expect(screen.getByText("PluginsTab")).toBeInTheDocument();
  });
});

describe("SettingsPage workspace subscription feature flag", () => {
  it("hides Billing and falls back to Workspace General from a direct URL", () => {
    navigationState.search = "tab=billing";

    renderWithI18n(<SettingsPage />);

    expect(
      screen.queryByRole("tab", { name: "Billing" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("BillingTab")).not.toBeInTheDocument();
    expect(screen.getByText("WorkspaceTab")).toBeInTheDocument();
  });

  it("shows and mounts Billing only when explicitly enabled", () => {
    navigationState.search = "tab=billing";
    configStore.getState().setFeatureFlags({
      [BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG]: true,
    });

    renderWithI18n(<SettingsPage />);

    expect(screen.getByRole("tab", { name: "Billing" })).toBeInTheDocument();
    expect(screen.getByText("BillingTab")).toBeInTheDocument();
  });
});
