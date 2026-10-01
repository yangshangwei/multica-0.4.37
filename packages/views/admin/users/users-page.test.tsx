import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import { AdminUsersPage } from "./users-page";
import en from "../../locales/en/admin.json";

vi.mock("@multica/core/admin", () => ({
  adminApiScope: () => "server",
  useAdminAccess: () => ({ identity: { userId: "actor", organizationId: "org", role: "super_admin" } }),
  useAdminUsers: () => ({ isPending: false, isError: false, refetch: vi.fn(), data: {
    items: [], nextCursor: "next-page", asOf: "2026-10-01T00:00:00Z", dataQuality: "partial",
    registration: { enabled: true, approvalRequired: false },
  } }),
}));

describe("account directory partial scans", () => {
  it("keeps continuation available without declaring a complete empty result", () => {
    render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
      <NavigationProvider value={{ push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/admin/users", searchParams: new URLSearchParams("status=disabled"), hash: "", getShareableUrl: (path) => path }}>
        <AdminUsersPage />
      </NavigationProvider>
    </I18nProvider>);
    expect(screen.getByText("These results are incomplete. Continue to the next page to find more matching accounts.")).toBeVisible();
    expect(screen.queryByText("No accounts match these filters")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Next page" })).toBeEnabled();
  });
});
