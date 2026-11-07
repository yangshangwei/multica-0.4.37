import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import { AdminUserDetailPage } from "./user-detail-page";
import en from "../../locales/en/admin.json";

const state = vi.hoisted(() => ({ role: "platform_observer", userId: "actor", status: "active", allowedActions: [] as string[] }));
vi.mock("@multica/core/admin", async original => ({
  ...await original<typeof import("@multica/core/admin")>(),
  adminApiScope: () => "server",
  useAdminAccess: () => ({ identity: { userId: state.userId, organizationId: "org", role: state.role } }),
  useAdminUser: () => ({ isPending: false, isError: false, refetch: vi.fn(), data: {
    user: { id: "account", name: "Example account", username: "example", status: state.status, platformRole: "platform_observer", workspaceCount: 1, createdAt: "2026-10-01T00:00:00Z", allowedActions: state.allowedActions },
    memberships: [{ workspaceId: "workspace", workspaceName: "Example workspace", role: "member" }],
    membershipsTruncated: false,
  } }),
}));
vi.mock("./user-action-form", () => ({ AdminUserActionForm: ({ action }: { action: string }) => <div data-testid="action-form">{action}</div> }));
beforeEach(() => { state.role = "platform_observer"; state.userId = "actor"; state.status = "active"; state.allowedActions = []; });
function mount(search = "") {
  return render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
    <NavigationProvider value={{ push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/admin/users/account", searchParams: new URLSearchParams(search), hash: "", getShareableUrl: path => path }}><AdminUserDetailPage id="account" /></NavigationProvider>
  </I18nProvider>);
}
it("retains the list's timezone and administrator return destination", () => {
  const params = new URLSearchParams({ timezone: "Asia/Shanghai", return_to: "/admin/administrators?role=platform_observer&cursor=next&timezone=Asia%2FShanghai" });
  mount(params.toString());
  expect(screen.getByRole("link", { name: /Back to/ })).toHaveAttribute("href", params.get("return_to"));
  expect(screen.getByText(/Asia\/Shanghai/)).toHaveTextContent(/8:00/);
  expect(screen.getByRole("heading", { name: "Workspace memberships" })).toBeInTheDocument();
});
it("uses a safe account-list fallback for an unrelated return path", () => {
  mount("return_to=https%3A%2F%2Funtrusted.invalid");
  expect(screen.getByRole("link", { name: /Back to/ })).toHaveAttribute("href", "/admin/users");
});
it("explains a read-only observer's disabled reset action even if a response advertises it", () => {
  state.allowedActions = ["recover-password"];
  mount();
  expect(screen.getByRole("button", { name: "Reset password" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Reset password" })).toHaveAccessibleDescription(en.users.resetReadOnly);
  expect(screen.queryByTestId("action-form")).not.toBeInTheDocument();
});
it("blocks self password reset locally and explains where to change it", () => {
  state.role = "super_admin"; state.userId = "account"; state.allowedActions = ["recover-password", "role"];
  mount();
  expect(screen.getByRole("button", { name: "Reset password" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Reset password" })).toHaveAccessibleDescription(en.users.resetSelf);
  expect(screen.getByRole("button", { name: "Change platform role" })).toBeEnabled();
});
it("explains a server-disallowed reset without bypassing deployment restrictions", () => {
  state.role = "super_admin"; state.status = "disabled"; state.allowedActions = ["role"];
  mount();
  expect(screen.getByRole("button", { name: "Reset password" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Reset password" })).toHaveAccessibleDescription(en.users.resetUnavailable);
});
it("keeps unknown account status read-only", () => {
  state.role = "super_admin"; state.status = "unknown"; state.allowedActions = ["recover-password"];
  mount();
  expect(screen.getByRole("button", { name: "Reset password" })).toHaveAccessibleDescription(en.users.resetUnknown);
  expect(screen.getByRole("button", { name: "Reset password" })).toBeDisabled();
});
it("opens the existing recovery form for an allowed reset", () => {
  state.role = "super_admin"; state.allowedActions = ["recover-password"];
  mount();
  fireEvent.click(screen.getByRole("button", { name: "Reset password" }));
  expect(screen.getByTestId("action-form")).toHaveTextContent("recover-password");
});
