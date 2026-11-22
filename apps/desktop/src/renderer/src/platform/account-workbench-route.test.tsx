import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { AccountDestination } from "@multica/core/paths";

const state = vi.hoisted(() => ({
  user: { requires_account_setup: false, requires_password_change: false } as {
    requires_account_setup: boolean;
    requires_password_change: boolean;
  } | null,
  location: { pathname: "/account", search: "?view=participating", hash: "" },
  setCurrentWorkspace: vi.fn(),
  page: vi.fn(),
}));

vi.mock("react-router-dom", () => ({ useLocation: () => state.location }));
vi.mock("@multica/core/auth", () => ({
  useAuthStore: Object.assign(
    (selector: (value: { user: typeof state.user }) => unknown) => selector({ user: state.user }),
    { getState: () => ({ user: state.user }) },
  ),
}));
vi.mock("@multica/core/platform", () => ({ setCurrentWorkspace: state.setCurrentWorkspace }));
vi.mock("@multica/views/i18n", () => ({ useT: () => ({ t: (selector: (value: unknown) => string) => selector({ page: { title: "Collaboration" }, notifications: { title: "Collaboration notifications" } }) }) }));
vi.mock("@multica/views/collaboration", () => ({
  AccountWorkbenchPage: ({ navigationInSidebar, destination }: {
    navigationInSidebar: boolean;
    destination: AccountDestination;
  }) => {
    state.page(destination);
    return <main data-testid="workbench" data-sidebar-navigation={navigationInSidebar} data-view={destination.view} />;
  },
}));

const { DesktopAccountWorkbenchRoute } = await import("./account-workbench-route");

beforeEach(() => {
  state.user = { requires_account_setup: false, requires_password_change: false };
  state.location = { pathname: "/account", search: "?view=participating", hash: "" };
  state.setCurrentWorkspace.mockClear();
  state.page.mockClear();
});
afterEach(cleanup);

it("uses the unified sidebar without restoring a workspace header for account content", () => {
  render(<DesktopAccountWorkbenchRoute />);
  expect(screen.getByTestId("workbench")).toHaveAttribute("data-sidebar-navigation", "true");
  expect(screen.getByTestId("workbench")).toHaveAttribute("data-view", "participating");
  expect(state.setCurrentWorkspace).toHaveBeenCalledExactlyOnceWith(null, null);
});

it("keeps notification selection in account scope and presents its distinct native title", () => {
  const issueId = "00000000-0000-4000-8000-000000000001";
  const notificationId = "00000000-0000-4000-8000-000000000002";
  state.location = { pathname: "/account", search: `?view=notifications&filter=unread&notification=${notificationId}&issue=${issueId}`, hash: "#description" };
  const view = render(<DesktopAccountWorkbenchRoute />);
  expect(state.page).toHaveBeenLastCalledWith({ view: "notifications", notificationView: "unread", notificationId, issueId, commentId: null, anchor: "description" });
  expect(document.title).toBe("Collaboration notifications");
  expect(state.setCurrentWorkspace).toHaveBeenCalledExactlyOnceWith(null, null);
  state.location = { pathname: "/account", search: "?view=current", hash: "" };
  view.rerender(<DesktopAccountWorkbenchRoute />);
  expect(document.title).toBe("Collaboration");
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
