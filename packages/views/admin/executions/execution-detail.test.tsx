import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminExecutionDetailPage } from "./execution-detail";
const state = vi.hoisted(() => ({
  isPending: false, isError: false, refetch: vi.fn(), data: {
    id: "run", workspaceId: "workspace", agentId: "agent", runtimeId: null, accountableUserId: null, submittedInstallationId: null, executionInstallationId: null, provider: null, model: null, failureCode: null, parentTaskId: "parent", retryOfTaskId: "retry", rerunOfTaskId: null, status: "failed", source: "chat", attempt: 2, createdAt: "2026-10-01T00:00:00Z", dispatchedAt: null, startedAt: null, completedAt: null, usage: null, contentAccess: false, contentUrl: null
  }
}));
vi.mock("@multica/core/admin", () => ({
  useAdminExecution: () => state, executionStatuses: [], executionSources: []
}));
it("shows actual attempt links, missing usage and restricted content", () => {
  render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
    <NavigationProvider value={{
      pathname: "/admin/tasks/run", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p
    }}>
      <AdminExecutionDetailPage id="run" />
    </NavigationProvider>
  </I18nProvider>);
  expect(screen.getByText("No usage has been reported for this execution.")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "retry" })).toHaveAttribute("href", "/admin/tasks/retry");
  expect(screen.queryByRole("link", { name: "Open original content" })).not.toBeInTheDocument();
  expect(screen.getAllByText("Not reported").length).toBeGreaterThan(0);
});
