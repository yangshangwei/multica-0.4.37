import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import type { AdminExecution } from "@multica/core/admin";
import { copyText } from "@multica/ui/lib/clipboard";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminExecutionDetailPage } from "./execution-detail";
const state = vi.hoisted(() => {
  const defaults: AdminExecution = {
    id: "run", workspaceId: "workspace", agentId: "agent", issueId: null, runtimeId: null, chatSessionId: null, autopilotRunId: null,
    accountableUserId: null, submittedInstallationId: null, executionInstallationId: null, provider: null, model: null,
    failureCode: null, parentTaskId: "parent", retryOfTaskId: "retry", rerunOfTaskId: null, status: "failed", source: "chat", attempt: 2,
    createdAt: "2026-10-01T00:00:00Z", dispatchedAt: null, startedAt: null, completedAt: null, usage: null,
    contentAccess: false, contentUrl: null, title: null, stateVersion: null, executionFence: null, allowedActions: []
  };
  return { isPending: false, isError: false, refetch: vi.fn(), defaults, data: { ...defaults } };
});
vi.mock("@multica/core/admin", async original => ({
  ...await original<typeof import("@multica/core/admin")>(),
  useAdminExecution: () => state, useAdminAccess: () => ({status: "ready", identity: {role: "platform_observer"}}), executionStatuses: [], executionSources: []
}));
vi.mock("@multica/ui/lib/clipboard", () => ({ copyText: vi.fn() }));
function mount(params = new URLSearchParams()) {
  return render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
    <NavigationProvider value={{
      pathname: "/admin/tasks/run", searchParams: params, hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p
    }}>
      <AdminExecutionDetailPage id="run" />
    </NavigationProvider>
  </I18nProvider>);
}
beforeEach(() => {
  vi.clearAllMocks();
  state.data = { ...state.defaults };
});
it("shows actual attempt links, missing usage and restricted content", () => {
  mount();
  expect(screen.getByText("No usage has been reported for this execution.")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "retry" }).getAttribute("href")).toMatch(/^\/admin\/tasks\/retry\?/);
  expect(screen.queryByRole("link", { name: "Open original content" })).not.toBeInTheDocument();
  expect(screen.getAllByText("Not reported").length).toBeGreaterThan(0);
});
it("shows the full permitted title and keeps list context across details and lineage", () => {
  const title = "Investigate the deployment queue and preserve the complete diagnostic context for this execution";
  state.data.title = title;
  state.data.contentAccess = true;
  state.data.contentUrl = "/workspace/issues/issue";
  const returnTo = "/admin/tasks?status=failed&cursor=cursor&timezone=Asia%2FShanghai";
  mount(new URLSearchParams({ timezone: "Asia/Shanghai", return_to: returnTo }));
  expect(screen.getByText(title)).toBeInTheDocument();
  expect(screen.getByRole("heading", { level: 1, name: "Execution details" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Open original content" })).toHaveAttribute("href", "/workspace/issues/issue");
  expect(screen.getByRole("link", { name: "Back to executions" })).toHaveAttribute("href", returnTo);
  expect(document.querySelector("time")).toHaveTextContent("Asia/Shanghai");
  const lineage = new URL(screen.getByRole("link", { name: "retry" }).getAttribute("href")!, "https://example.test");
  expect(lineage.searchParams.get("timezone")).toBe("Asia/Shanghai");
  expect(lineage.searchParams.get("return_to")).toBe(returnTo);
});
it("does not reveal a title when original-content access is restricted", () => {
  state.data.title = "Private task title";
  state.data.contentUrl = "/workspace/issues/private-issue";
  mount();
  expect(screen.queryByText("Private task title")).not.toBeInTheDocument();
  expect(screen.getByText("Restricted task")).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "Open original content" })).not.toBeInTheDocument();
});
it("copies the full execution ID and announces the outcome", async () => {
  vi.mocked(copyText).mockResolvedValueOnce(true);
  mount();
  fireEvent.click(screen.getByText("Technical details"));
  fireEvent.click(screen.getByRole("button", { name: "Copy execution ID" }));
  expect(await screen.findByRole("button", { name: "Execution ID copied" })).toBeInTheDocument();
  expect(copyText).toHaveBeenCalledWith("run");
});
it("keeps full technical identifiers and raw failure code reachable without obscuring the summary", () => {
  Object.assign(state.data, {
    runtimeId: "runtime-id", accountableUserId: "owner-id", submittedInstallationId: "submitted-id", executionInstallationId: "target-id",
    provider: "test-provider", model: "test-model", failureCode: "REMOTE_RUNTIME_TIMEOUT",
  });
  mount();
  expect(screen.getByText("Failed")).toBeVisible();
  expect(screen.getByText("Conversation")).toBeVisible();
  expect(screen.getByRole("heading", { name: "Time records" })).toBeVisible();
  expect(screen.getByRole("heading", { name: "Reported usage" })).toBeVisible();
  expect(screen.getByRole("link", { name: "parent" })).toBeVisible();
  const disclosure = screen.getByText("Technical details").closest("details")!;
  expect(disclosure).not.toHaveAttribute("open");
  const identifiers = ["run", "workspace", "agent", "runtime-id", "owner-id", "submitted-id", "target-id", "test-provider", "test-model", "REMOTE_RUNTIME_TIMEOUT"];
  for (const value of identifiers) expect(within(disclosure).getByText(value)).not.toBeVisible();
  fireEvent.click(screen.getByText("Technical details"));
  for (const value of identifiers) expect(within(disclosure).getByText(value)).toBeVisible();
});
it("keeps the full execution ID selectable when copying fails", async () => {
  state.data.id = "00000000-0000-4000-8000-000000000001";
  vi.mocked(copyText).mockResolvedValueOnce(false);
  mount();
  fireEvent.click(screen.getByText("Technical details"));
  fireEvent.click(screen.getByRole("button", { name: "Copy execution ID" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Could not copy. Select and copy the execution ID below.");
  expect(screen.getByText(state.data.id)).toBeVisible();
  expect(screen.getByText(state.data.id)).toHaveClass("select-all");
  expect(copyText).toHaveBeenCalledWith(state.data.id);
});
