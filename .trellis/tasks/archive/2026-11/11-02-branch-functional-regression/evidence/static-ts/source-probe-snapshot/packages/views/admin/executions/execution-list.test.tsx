import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminExecutionListPage } from "./execution-list";
const state = vi.hoisted(() => ({
  data: {
    items: [{
      id: "task", source: "issue", status: "failed", attempt: 2, title: null as string | null, contentAccess: false, contentUrl: null as string | null, createdAt: "2026-10-01T00:00:00Z", usage: null
    }], nextCursor: "cursor", asOf: "2026-10-01T00:00:00Z"
  }, isPending: false, isError: false, refetch: vi.fn(), push: vi.fn(), replace: vi.fn()
}));
vi.mock("@multica/core/admin", async original => ({
  ...await original<typeof import("@multica/core/admin")>(),
  useAdminExecutions: () => state, executionStatuses: ["queued", "failed", "unknown"], executionSources: ["issue", "chat", "unknown"]
}));
function mount(params = new URLSearchParams()) {
  return render(<I18nProvider locale="en" resources={{ en: { admin: en } }}>
    <NavigationProvider value={{
      pathname: "/admin/tasks", searchParams: params, hash: "", push: state.push, replace: state.replace, back: vi.fn(), getShareableUrl: p => p
    }}>
      <AdminExecutionListPage />
    </NavigationProvider>
  </I18nProvider>);
}
beforeEach(() => {
  vi.clearAllMocks();
  state.isPending = false;
  state.isError = false;
  state.data.items[0]!.title = null;
  state.data.items[0]!.contentAccess = false;
  state.data.items[0]!.contentUrl = null;
});
it("renders restricted metadata and navigates using the opaque next cursor", () => {
  mount();
  expect(screen.getByText("Restricted task")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Restricted task" }).getAttribute("href")).toMatch(/^\/admin\/tasks\/task\?/);
  fireEvent.click(screen.getByRole("button", { name: "Next page" }));
  expect(state.push).toHaveBeenCalledWith("/admin/tasks?cursor=cursor");
});
it("leads with the permitted task title and carries filters, cursor and timezone into details", () => {
  const item = state.data.items[0]!;
  item.title = "Investigate intermittent queue failures in the deployment region";
  item.contentAccess = true;
  item.contentUrl = "/workspace/issues/task";
  const params = new URLSearchParams("status=failed&cursor=cursor&timezone=Asia%2FShanghai");
  mount(params);
  const columns = screen.getAllByRole("columnheader");
  expect(columns[0]).toHaveTextContent("Business task");
  expect(columns[1]).toHaveTextContent("Status");
  const link = screen.getByRole("link", { name: item.title });
  const target = new URL(link.getAttribute("href")!, "https://example.test");
  expect(target.searchParams.get("timezone")).toBe("Asia/Shanghai");
  expect(target.searchParams.get("return_to")).toBe(`/admin/tasks?${params}`);
  expect(screen.getByText("task")).toBeInTheDocument();
});
it("submits filters through the navigation adapter and resets pagination", () => {
  mount();
  fireEvent.change(screen.getByLabelText("Task ID or business identifier"), { target: { value: "ABC-1" } });
  fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));
  expect(state.push.mock.calls[0]?.[0]).toContain("q=ABC-1");
});
it("shows retryable errors without pretending an empty result", () => {
  state.isError = true;
  mount();
  expect(screen.getByRole("alert")).toHaveTextContent("Cannot load this view");
  fireEvent.click(screen.getByRole("button", { name: "Try again" }));
  expect(state.refetch).toHaveBeenCalledOnce();
});
