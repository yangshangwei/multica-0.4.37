import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminUnassociatedRuntimesPage } from "./unassociated-page";
vi.mock("@multica/core/admin", () => ({
  useAdminUnassociatedRuntimes: () => ({
    isPending: false, isError: false, refetch: vi.fn(), data: {
      items: [{
        id: "runtime", workspaceId: "workspace", provider: "codex", status: "offline", lastSeenAt: null
      }], asOf: "2026-10-01T00:00:00Z", nextCursor: null, dataQuality: "complete"
    }
  })
}));
it("keeps legacy runtimes distinct and identifies the status as last reported", () => {
  render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
    <NavigationProvider value={{
      pathname: "/admin/installations/unassociated", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p
    }}>
      <AdminUnassociatedRuntimesPage />
    </NavigationProvider>
  </I18nProvider>);
  expect(screen.getByRole("columnheader", { name: "Last reported status" })).toBeInTheDocument();
  expect(screen.getByText("runtime")).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "runtime" })).not.toBeInTheDocument();
  expect(screen.getByText(/Names, IP addresses/)).toBeInTheDocument();
});
