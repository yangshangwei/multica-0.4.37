import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../navigation";
import { AdminShell } from "./admin-shell";
import en from "../locales/en/admin.json";

const state = vi.hoisted(() => ({
  status: "ready",
  identity: { userId: "user", organizationId: "organization", role: "platform_observer", allowedActions: [], supported: true },
  retry: vi.fn(),
  logout: vi.fn(),
  isMobile: false,
}));
vi.mock("@multica/core/admin", () => ({ useAdminAccess: () => state }));
vi.mock("../auth/use-logout", () => ({ useLogout: () => state.logout }));
vi.mock("@multica/ui/hooks/use-mobile", () => ({ useIsMobile: () => state.isMobile }));

function shell(pathname = "/admin") {
  return (
    <I18nProvider locale="en" resources={{ en: { admin: en } }}>
      <NavigationProvider value={{ push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname, searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path }}>
        <AdminShell><p>Authorized detail</p></AdminShell>
      </NavigationProvider>
    </I18nProvider>
  );
}
function mount(pathname = "/admin") { return render(shell(pathname)); }

beforeEach(() => { state.status = "ready"; state.isMobile = false; vi.clearAllMocks(); });

describe("administration shell", () => {
  // Parsing and revocation matrices live in core/admin/*.test.ts(x).
  it("renders the current role and organization, with only implemented destinations", () => {
    mount();
    expect(screen.getByRole("link", { name: "Overview" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByText("Administration access").closest("details")).toBeInTheDocument();
    expect(screen.getByText("platform_observer")).toBeInTheDocument();
    expect(screen.getByText("organization")).toBeInTheDocument();
    expect(screen.getByText("Authorized detail")).toBeInTheDocument();
    const sidebar = screen.getByRole("complementary");
    const navigation = within(sidebar).getByRole("navigation", { name: en.shell.navigation });
    expect(within(navigation).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual(["/admin", "/admin/users", "/admin/installations", "/admin/tasks", "/admin/alerts", "/admin/settings"]);
    expect(within(sidebar).getByRole("link", { name: "Return to Multica" })).toHaveAttribute("href", "/login");
    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));
    expect(state.logout).toHaveBeenCalledOnce();
  });

  it.each([
    ["denied", "Administration access denied"],
    ["unsupported", "Administration is unavailable on this server"],
    ["unavailable", "Cannot reach platform administration"],
  ])("removes protected content and names the %s state", (status, title) => {
    state.status = status;
    mount();
    expect(screen.getByRole("alert")).toHaveTextContent(title);
    expect(screen.queryByText("Authorized detail")).not.toBeInTheDocument();
    expect(screen.queryByText("organization")).not.toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: en.shell.navigation })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(state.retry).toHaveBeenCalledOnce();
  });
});


it("gives child pages the main heading and marks their navigation", () => {
  mount("/admin/users/account-id");
  expect(screen.queryByRole("heading", { name: "Administration access" })).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Accounts" })).toHaveAttribute("aria-current", "page");
  expect(screen.getAllByRole("link").filter(link => link.getAttribute("aria-current") === "page")).toHaveLength(1);
  expect(screen.getByText("Authorized detail")).toBeInTheDocument();
});

it("keeps the selected child with its parent in the sidebar", () => {
  mount("/admin/settings");
  const sidebar = screen.getByRole("complementary");
  const parent = within(sidebar).getByRole("link", { name: "System management" });
  const group = parent.closest("li")!;
  expect(within(group).getByRole("link", { name: "Deployment settings" })).toHaveAttribute("aria-current", "page");
  expect(within(group).getByRole("link", { name: "Administrators" })).toHaveAttribute("href", "/admin/administrators");
  expect(within(group).getByRole("link", { name: "Resource publishing" })).toHaveAttribute("href", "/admin/resources");
  expect(within(group).getByRole("link", { name: "Audit trail" })).toHaveAttribute("href", "/admin/audit");
});

it("opens compact navigation at the current destination and restores focus on Escape", async () => {
  state.isMobile = true;
  const user = userEvent.setup();
  mount("/admin/settings");
  expect(screen.queryByRole("navigation", { name: en.shell.navigation })).not.toBeInTheDocument();
  const trigger = screen.getByRole("button", { name: "Open navigation" });
  await user.click(trigger);
  const dialog = await screen.findByRole("dialog", { name: en.shell.navigation });
  await waitFor(() => expect(within(dialog).getByRole("link", { name: "Deployment settings" })).toHaveFocus());
  await user.keyboard("{Escape}");
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(trigger).toHaveFocus();
});

it("closes the compact menu after choosing a destination and after returning to desktop", async () => {
  state.isMobile = true;
  const user = userEvent.setup();
  const view = mount("/admin/settings");
  await user.click(screen.getByRole("button", { name: "Open navigation" }));
  await user.click(within(await screen.findByRole("dialog")).getByRole("link", { name: "Administrators" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  await user.click(screen.getByRole("button", { name: "Open navigation" }));
  state.isMobile = false;
  view.rerender(shell("/admin/settings"));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(within(screen.getByRole("complementary")).getByRole("link", { name: "Deployment settings" })).toHaveAttribute("aria-current", "page");
});

it("dismisses compact navigation when access is lost so the denial is visible", async () => {
  state.isMobile = true;
  const user = userEvent.setup();
  const view = mount("/admin/settings");
  await user.click(screen.getByRole("button", { name: "Open navigation" }));
  expect(await screen.findByRole("dialog")).toBeInTheDocument();
  state.status = "denied";
  view.rerender(shell("/admin/settings"));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(screen.getByRole("alert")).toHaveTextContent("Administration access denied");
});

it("marks resource publishing as the selected system destination", () => {
  mount("/admin/resources");
  expect(screen.getByRole("link", { name: "Resource publishing" })).toHaveAttribute("aria-current", "page");
  expect(screen.getByRole("link", { name: "System management" })).toBeInTheDocument();
});
