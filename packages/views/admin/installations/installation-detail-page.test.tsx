import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminInstallationDetailPage } from "./installation-detail-page";
const axis = {
  state: "unknown", freshness: "unknown", observedAt: null, source: "none", reasonCode: "missing"
};
vi.mock("@multica/core/admin", () => ({
  useAdminAccess: () => ({status: "ready", identity: {role: "platform_observer"}}),
  useAdminInstallation: () => ({
    isPending: false, isError: false, refetch: vi.fn(), data: {
      installation: {
        id: "install", displayName: "Managed laptop", deploymentId: "deployment", responsibleUserId: null, desktopVersion: null, os: null, groups: [], lifecycle: "active", runtimeCount: 0, clientActivity: axis, daemonReachability: axis, executionReadiness: axis
      }, asOf: "2026-10-01T00:00:00Z", bindings: [], runtimes: [], users: [], detailsTruncated: false
    }
  })
}));
it("presents unknown evidence and a scoped execution link without machine controls", () => {
  render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
    <NavigationProvider value={{
      pathname: "/admin/installations/install", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p
    }}>
      <AdminInstallationDetailPage id="install" />
    </NavigationProvider>
  </I18nProvider>);
  expect(screen.getAllByText("Unknown")).toHaveLength(3);
  expect(screen.getByText("No verified daemon binding yet.")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "View executions" })).toHaveAttribute("href", "/admin/tasks?installation_id=install");
  expect(screen.queryByRole("button", { name: /stop|upgrade|shell/i })).not.toBeInTheDocument();
});
