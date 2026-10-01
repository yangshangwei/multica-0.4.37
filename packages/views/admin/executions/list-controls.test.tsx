import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminExecutionFilters, AdminListPagination } from "./list-controls";
const push = vi.fn();
function mount(params: URLSearchParams, pagination?: URLSearchParams) { return render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><NavigationProvider value={{ pathname: "/admin/tasks", searchParams: params, hash: "", push, replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}>{pagination ? <AdminListPagination cursor="next" asOf="2026-10-02T00:00:00Z" params={pagination} refresh={vi.fn()} /> : <AdminExecutionFilters />}</NavigationProvider></I18nProvider>); }
beforeEach(() => push.mockClear());
it("preserves the finished reporting basis when filters change", () => {
  mount(new URLSearchParams("time_basis=finished&status=failed&time_from=2026-10-01T00%3A00%3A00Z&time_to=2026-10-02T00%3A00%3A00Z"));
  fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));
  expect(new URL(push.mock.calls[0]![0], "https://test.invalid").searchParams.get("time_basis")).toBe("finished");
});
it.each([["queued", "", true], ["failed", "", false], ["queued", "2026-10-01T00:00:00Z", false]])("keeps current-state scope only for a legal %s filter (%s)", (status, from, current) => {
  mount(new URLSearchParams("state_scope=current&time_basis=created&status=queued"));
  fireEvent.change(screen.getByLabelText("Status"), { target: { value: status } });
  if (from) fireEvent.change(screen.getByLabelText(en.executions.from), { target: { value: from } });
  fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));
  expect(new URL(push.mock.calls[0]![0], "https://test.invalid").searchParams.has("state_scope")).toBe(current);
});
it("makes copied cursor links carry the effective pinned window", () => {
  const effective = new URLSearchParams("time_from=2026-10-01T00%3A00%3A00Z&time_to=2026-10-02T00%3A00%3A00Z&timezone=Asia%2FShanghai");
  mount(new URLSearchParams(), effective);
  fireEvent.click(screen.getByRole("button", { name: "Next page" }));
  const params = new URL(push.mock.calls[0]![0], "https://test.invalid").searchParams;
  expect(params.get("time_from")).toBe(effective.get("time_from"));
  expect(params.get("timezone")).toBe("Asia/Shanghai");
  expect(params.get("cursor")).toBe("next");
});
