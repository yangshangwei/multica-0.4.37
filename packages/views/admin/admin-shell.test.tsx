import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../navigation";
import { AdminShell } from "./admin-shell";
import en from "../locales/en/admin.json";

const state = vi.hoisted(() => ({
  status: "ready",
  identity: { userId: "user", organizationId: "organization", role: "platform_observer", allowedActions: [], supported: true },
  retry: vi.fn(),
  logout: vi.fn(),
}));
vi.mock("@multica/core/admin", () => ({ useAdminAccess: () => state }));
vi.mock("../auth/use-logout", () => ({ useLogout: () => state.logout }));

function mount(pathname = "/admin") {
  return render(
    <I18nProvider locale="en" resources={{ en: { admin: en } }}>
      <NavigationProvider value={{ push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname, searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path }}>
        <AdminShell><p>Authorized detail</p></AdminShell>
      </NavigationProvider>
    </I18nProvider>,
  );
}

beforeEach(() => { state.status = "ready"; vi.clearAllMocks(); });

describe("administration shell", () => {
  // Parsing and revocation matrices live in core/admin/*.test.ts(x).
  it("renders the current role and organization, with only implemented destinations", () => {
    mount();
    expect(screen.getByRole("heading", { name: "Administration access" })).toBeInTheDocument();
    expect(screen.getByText("platform_observer")).toBeInTheDocument();
    expect(screen.getByText("organization")).toBeInTheDocument();
    expect(screen.getByText("Authorized detail")).toBeInTheDocument();
    expect(screen.getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual(["/admin", "/login", "/admin", "/admin/users", "/admin/installations", "/admin/tasks", "/admin/administrators"]);
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
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(state.retry).toHaveBeenCalledOnce();
  });
});


it("gives child pages the main heading and marks their navigation", () => {
  mount("/admin/users/account-id");
  expect(screen.queryByRole("heading", { name: "Administration access" })).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Accounts and organization" })).toHaveAttribute("aria-current", "page");
  expect(screen.getByText("Authorized detail")).toBeInTheDocument();
});
