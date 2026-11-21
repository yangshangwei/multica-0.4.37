import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
  user: { requires_account_setup: false, requires_password_change: false } as {
    requires_account_setup: boolean;
    requires_password_change: boolean;
  } | null,
  location: { pathname: "/account", search: "?view=participating", hash: "" },
  setCurrentWorkspace: vi.fn(),
}));

vi.mock("react-router-dom", () => ({ useLocation: () => state.location }));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector: (value: { user: typeof state.user }) => unknown) => selector({ user: state.user }),
    { getState: () => ({ user: state.user }) },
  ),
}));
vi.mock("@multica/core/platform", () => ({ setCurrentWorkspace: state.setCurrentWorkspace }));
vi.mock("@multica/views/i18n", () => ({ useT: () => ({ t: () => "Collaboration" }) }));
vi.mock("@multica/views/collaboration", () => ({
  AccountWorkbenchPage: ({ navigationInSidebar, destination }: {
    navigationInSidebar: boolean;
    destination: { view: string };
  }) => <main data-testid="workbench" data-sidebar-navigation={navigationInSidebar} data-view={destination.view} />,
}));

const { DesktopAccountWorkbenchRoute } = await import("./account-workbench-route");

beforeEach(() => {
  state.user = { requires_account_setup: false, requires_password_change: false };
  state.location = { pathname: "/account", search: "?view=participating", hash: "" };
  state.setCurrentWorkspace.mockClear();
});
afterEach(cleanup);

it("uses the unified sidebar without restoring a workspace header for account content", () => {
  render(<DesktopAccountWorkbenchRoute />);
  expect(screen.getByTestId("workbench")).toHaveAttribute("data-sidebar-navigation", "true");
  expect(screen.getByTestId("workbench")).toHaveAttribute("data-view", "participating");
  expect(state.setCurrentWorkspace).toHaveBeenCalledExactlyOnceWith(null, null);
});

it.each(["signed-out", "setup", "password"])("retains the %s account guard", (guard) => {
  state.user = guard === "signed-out" ? null : {
    requires_account_setup: guard === "setup",
    requires_password_change: guard === "password",
  };
  render(<DesktopAccountWorkbenchRoute />);
  expect(screen.queryByTestId("workbench")).not.toBeInTheDocument();
  expect(state.setCurrentWorkspace).toHaveBeenCalledWith(null, null);
});
