import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, fireEvent, within, waitFor } from "@testing-library/react";
import { copyText } from "@multica/ui/lib/clipboard";
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
vi.mock("@multica/core/admin", async original => ({ ...await original<typeof import("@multica/core/admin")>(), useAdminInstallations: () => state }));
vi.mock("@multica/ui/lib/clipboard", () => ({ copyText: vi.fn() }));
beforeEach(() => {
  state.isPending = false;
  state.isError = false;
  vi.clearAllMocks();
  state.data.items[0]!.id = "install";
  state.data.items[0]!.displayName = "Laptop";
});
it("keeps an unnamed installation's full identity accessible while shortening its visible UUID", async () => {
  const id = "3bfac87d-e035-4a99-a619-e810b395a052";
  state.data.items[0]!.id = id;
  state.data.items[0]!.displayName = "";
  vi.mocked(copyText).mockResolvedValue(true);
  render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><NavigationProvider value={{ pathname: "/admin/installations", searchParams: new URLSearchParams(), hash: "", push: state.push, replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}><AdminInstallationsPage /></NavigationProvider></I18nProvider>);
  const link = screen.getByRole("link", { name: id });
  expect(link).toHaveTextContent("3bfac87d...a052");
  expect(link).not.toHaveTextContent(id);
  expect(link).toHaveAttribute("title", id);
  fireEvent.click(screen.getByText("Installation ID", { selector: "summary" }));
  fireEvent.click(screen.getByRole("button", { name: "Copy installation ID" }));
  expect(copyText).toHaveBeenCalledWith(id);
  expect(await screen.findByText("Installation ID copied")).toBeInTheDocument();
});
it("keeps the full ID selectable when copying fails", async () => {
  vi.mocked(copyText).mockResolvedValue(false);
  render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><NavigationProvider value={{ pathname: "/admin/installations", searchParams: new URLSearchParams(), hash: "", push: state.push, replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}><AdminInstallationsPage /></NavigationProvider></I18nProvider>);
  expect(screen.getByRole("link", { name: "Laptop" })).toBeInTheDocument();
  fireEvent.click(screen.getByText("Installation ID", { selector: "summary" }));
  fireEvent.click(screen.getByRole("button", { name: "Copy installation ID" }));
  await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Select and copy the full installation ID below"));
  expect(screen.getByText("install", { selector: "code" })).toHaveClass("select-all");
  expect(screen.queryByText("Installation ID copied")).not.toBeInTheDocument();
});
it("preserves installation filters and the chosen time zone in detail navigation", () => {
  render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><NavigationProvider value={{ pathname: "/admin/installations", searchParams: new URLSearchParams("lifecycle=active&timezone=Asia%2FShanghai&cursor=page2"), hash: "", push: state.push, replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}><AdminInstallationsPage /></NavigationProvider></I18nProvider>);
  const link = new URL(screen.getByRole("link", { name: "Laptop" }).getAttribute("href")!, "https://test.invalid");
  expect(link.searchParams.get("timezone")).toBe("Asia/Shanghai");
  expect(link.searchParams.get("return_to")).toBe("/admin/installations?lifecycle=active&timezone=Asia%2FShanghai&cursor=page2");
  const applied = screen.getByRole("status", { name: "Applied filters" });
  expect(applied).toHaveTextContent("Lifecycle:Active");
  expect(applied).toHaveTextContent("Asia/Shanghai");
  expect(applied).not.toHaveTextContent("page2");
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
  const detail = new URL(screen.getByRole("link", { name: "Laptop" }).getAttribute("href")!, "https://test.invalid");
  expect(detail.pathname).toBe("/admin/installations/install");
  expect(detail.searchParams.get("return_to")).toBe("/admin/installations");
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
  expect(screen.getByText(/Last observed/).querySelector("time")).toHaveAttribute("datetime", "2026-10-01T00:00:00Z");
  fireEvent.click(screen.getByRole("button", { name: "Try again" }));
  expect(state.refetch).toHaveBeenCalledOnce();
});
