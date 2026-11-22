import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { AccountDestination } from "@multica/core/paths";
import { AccountWorkbenchRoute } from "./account-workbench-route";

const state = vi.hoisted(() => ({
  navigation: { pathname: "/account", searchParams: new URLSearchParams(), hash: "", replace: vi.fn() },
  page: vi.fn(),
}));
vi.mock("@multica/views/navigation", async (original) => ({ ...await original<typeof import("@multica/views/navigation")>(), useNavigation: () => state.navigation }));
vi.mock("@multica/views/i18n", () => ({ useT: () => ({ t: (selector: (value: unknown) => string) => selector({ page: { title: "Collaboration" }, notifications: { title: "Collaboration notifications" } }) }) }));
vi.mock("@multica/views/collaboration", () => ({ AccountWorkbenchPage: (props: { destination: AccountDestination; administrationHref: string; navigationInSidebar: boolean }) => {
  state.page(props);
  return <p>Workbench content</p>;
} }));

beforeEach(() => {
  state.navigation.searchParams = new URLSearchParams();
  state.navigation.hash = "";
  state.navigation.replace.mockClear();
  state.page.mockClear();
});

it("keeps explicit account view and anchored issue selection while the sidebar owns navigation", () => {
  const issueId = "00000000-0000-4000-8000-000000000001";
  const runId = "00000000-0000-4000-8000-000000000002";
  state.navigation.searchParams = new URLSearchParams({ view: "participating", issue: issueId });
  state.navigation.hash = `#run-${runId}`;
  const view = render(<AccountWorkbenchRoute />);
  expect(state.page).toHaveBeenLastCalledWith({
    destination: { view: "participating", issueId, commentId: null, runId },
    administrationHref: "/admin/collaboration", navigationInSidebar: true,
  });
  state.navigation.searchParams = new URLSearchParams({ view: "current" });
  state.navigation.hash = "";
  view.rerender(<AccountWorkbenchRoute />);
  expect(state.page).toHaveBeenLastCalledWith({
    destination: { view: "current", issueId: null, commentId: null },
    administrationHref: "/admin/collaboration", navigationInSidebar: true,
  });
  expect(state.navigation.replace).not.toHaveBeenCalled();
});

it("normalizes an invalid account destination without mounting private content", () => {
  state.navigation.searchParams = new URLSearchParams({ view: "invalid" });
  render(<AccountWorkbenchRoute />);
  expect(screen.queryByText("Workbench content")).toBeNull();
  expect(state.navigation.replace).toHaveBeenCalledWith("/account?view=current");
});

it("dispatches notification filter and exact selection while updating the document title", () => {
  const issueId = "00000000-0000-4000-8000-000000000001";
  const notificationId = "00000000-0000-4000-8000-000000000002";
  state.navigation.searchParams = new URLSearchParams({ view: "notifications", filter: "archived", notification: notificationId, issue: issueId });
  state.navigation.hash = "#description";
  const view = render(<AccountWorkbenchRoute />);
  expect(state.page).toHaveBeenLastCalledWith({
    destination: { view: "notifications", notificationView: "archived", notificationId, issueId, commentId: null, anchor: "description" },
    administrationHref: "/admin/collaboration", navigationInSidebar: true,
  });
  expect(document.title).toBe("Collaboration notifications | Multica");
  state.navigation.searchParams = new URLSearchParams({ view: "participating" });
  state.navigation.hash = "";
  view.rerender(<AccountWorkbenchRoute />);
  expect(document.title).toBe("Collaboration | Multica");
  expect(state.navigation.replace).not.toHaveBeenCalled();
});
