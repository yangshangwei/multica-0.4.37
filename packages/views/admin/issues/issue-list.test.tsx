import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminIssueListPage } from "./issue-list";
vi.mock("@multica/core/admin", async original => ({
  ...await original<typeof import("@multica/core/admin")>(),
  executionStatuses: [], executionSources: [], useAdminIssues: () => ({
    isPending: false, isError: false, refetch: vi.fn(), data: {
      items: [{
        id: "issue", workspaceId: "workspace", identifier: "ABC-1", status: "todo", executionCount: 3, title: null, contentAccess: false, createdAt: "2026-09-20T00:00:00Z"
      }], nextCursor: null, asOf: "2026-10-01T00:00:00Z", timeFrom: "2026-09-01T00:00:00Z", timeTo: "2026-10-01T00:00:00Z"
    }
  })
}));
it("keeps business tasks distinct and carries the count window into execution drilldown", () => {
  render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
    <NavigationProvider value={{
      pathname: "/admin/issues", searchParams: new URLSearchParams("timezone=Asia%2FShanghai"), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p
    }}>
      <AdminIssueListPage />
    </NavigationProvider>
  </I18nProvider>);
  expect(screen.getByText("ABC-1")).toBeInTheDocument();
  const link = screen.getByRole("link", { name: "3" });
  const target = new URL(link.getAttribute("href")!, "https://example.test");
  expect(target.searchParams.get("time_from")).toBe("2026-09-01T00:00:00Z");
  expect(target.searchParams.get("time_to")).toBe("2026-10-01T00:00:00Z");
  expect(target.searchParams.get("timezone")).toBe("Asia/Shanghai");
  expect(screen.getByText("Restricted task")).toBeInTheDocument();
});
