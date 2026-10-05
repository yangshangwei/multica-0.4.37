import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { AdminOperation } from "@multica/core/admin";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminOperationDetailPage } from "./operation-detail-page";

const state = vi.hoisted(() => ({
  data: undefined as AdminOperation | undefined,
  isPending: true, isFetching: true, isError: false, refetch: vi.fn(),
}));
vi.mock("@multica/core/admin", async original => ({
  ...await original<typeof import("@multica/core/admin")>(),
  useAdminOperation: () => state,
  useAdminAccess: () => ({ status: "ready", identity: { userId: "actor", organizationId: "organization" } }),
  adminApiScope: () => "scope",
}));
function mount() {
  return render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
    <NavigationProvider value={{ pathname: "/admin/operations/operation", searchParams: new URLSearchParams({ timezone: "Asia/Shanghai", return_to: "/admin/tasks?status=running&timezone=Asia%2FShanghai" }), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}>
      <AdminOperationDetailPage id="operation" />
    </NavigationProvider>
  </I18nProvider>);
}
beforeEach(() => {
  vi.clearAllMocks();
  state.data = undefined;
  state.isPending = true;
  state.isFetching = true;
  state.isError = false;
});
it("names the standalone page and announces the first receipt request", () => {
  mount();
  expect(screen.getByRole("heading", { level: 1, name: "Operation receipt" })).toBeInTheDocument();
  expect(screen.getByRole("status")).toHaveTextContent("Loading operation receipt…");
  expect(screen.getByRole("button", { name: "Refresh status" })).toBeDisabled();
});
it("offers a retry for the first failure without claiming a previous receipt exists", () => {
  state.isPending = false;
  state.isFetching = false;
  state.isError = true;
  mount();
  expect(screen.getByRole("alert")).toHaveTextContent("Cannot load this receipt. Refresh to try again.");
  expect(screen.queryByText(/last receipt below/)).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Refresh status" }));
  expect(state.refetch).toHaveBeenCalledOnce();
});
it("retains known receipt data on refresh failure and labels its selected timezone", () => {
  state.isPending = false;
  state.isFetching = false;
  state.isError = true;
  state.data = {
    id: "operation", organizationId: "organization", targetId: "task", actorId: null,
    targetKind: "task", kind: "task.cancel", state: "applied", resultCode: "awaiting_daemon_confirmation",
    confirmation: "unconfirmed", reconciliationState: "unconfirmed", rootOperationId: null, version: 1,
    acceptedAt: "2026-10-02T01:02:00Z", updatedAt: null, appliedAt: null, confirmedAt: null, ackDeadline: null,
  };
  mount();
  expect(screen.getByRole("alert")).toHaveTextContent("The last receipt below is retained");
  expect(screen.getByText("Applied on server")).toBeInTheDocument();
  expect(document.querySelector("time")).toHaveTextContent("Asia/Shanghai");
  const target = new URL(screen.getByRole("link", { name: "Back to target" }).getAttribute("href")!, "https://example.test");
  expect(target.pathname).toBe("/admin/tasks/task");
  expect(target.searchParams.get("timezone")).toBe("Asia/Shanghai");
  expect(target.searchParams.get("return_to")).toBe("/admin/tasks?status=running&timezone=Asia%2FShanghai");
});
