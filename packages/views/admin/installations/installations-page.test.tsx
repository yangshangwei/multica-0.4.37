import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminInstallationsPage } from "./installations-page";
const state = vi.hoisted(() => ({
  push: vi.fn(), refetch: vi.fn(), isPending: false, isError: false, data: {
    items: [{
      id: "install", displayName: "Laptop", lifecycle: "active", runtimeCount: 2, desktopVersion: null, os: null, clientActivity: {
        state: "inactive", freshness: "stale", observedAt: null
      }, daemonReachability: {
        state: "reachable", freshness: "fresh", observedAt: null
      }, executionReadiness: {
        state: "environment_unavailable", freshness: "fresh", observedAt: null
      }
    }], asOf: "2026-10-01T00:00:00Z", nextCursor: "opaque", dataQuality: "complete"
  }
}));
vi.mock("@multica/core/admin", () => ({ useAdminInstallations: () => state }));
beforeEach(() => {
  state.isPending = false;
  state.isError = false;
  vi.clearAllMocks();
});
it("shows client, daemon and engine independently without management controls", () => {
  render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
    <NavigationProvider value={{
      pathname: "/admin/installations", searchParams: new URLSearchParams(), hash: "", push: state.push, replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p
    }}>
      <AdminInstallationsPage />
    </NavigationProvider>
  </I18nProvider>);
  expect(within(screen.getByRole("table")).getByText("Inactive")).toBeInTheDocument();
  expect(screen.getByText("Reachable")).toBeInTheDocument();
  expect(screen.getByText("Environment unavailable")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Laptop" })).toHaveAttribute("href", "/admin/installations/install");
  expect(screen.queryByRole("button", { name: /stop|shell|upgrade/i })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Next page" }));
  expect(state.push).toHaveBeenCalledWith("/admin/installations?cursor=opaque");
});
it("keeps the last observation time during a database outage without showing stale health", () => {
  state.isError = true;
  render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
    <NavigationProvider value={{
      pathname: "/admin/installations", searchParams: new URLSearchParams(), hash: "", push: state.push, replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p
    }}>
      <AdminInstallationsPage />
    </NavigationProvider>
  </I18nProvider>);
  expect(screen.getByRole("alert")).toHaveTextContent("Cannot load installation metadata");
  expect(screen.queryByRole("table")).not.toBeInTheDocument();
  expect(screen.getByText(/Last observed/)).toHaveTextContent("2026-10-01");
  fireEvent.click(screen.getByRole("button", { name: "Try again" }));
  expect(state.refetch).toHaveBeenCalledOnce();
});
