import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminExecutionFilters, AdminListPagination, formatAdminTime } from "./list-controls";
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
  if (from) fireEvent.change(screen.getByLabelText(en.executions.from), { target: { value: from.replace("Z", "") } });
  fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));
  expect(new URL(push.mock.calls[0]![0], "https://test.invalid").searchParams.has("state_scope")).toBe(current);
});
it("identifies and focuses an invalid filter without navigating", async () => {
  mount(new URLSearchParams("timezone=UTC"));
  const zone = screen.getByLabelText(en.executions.timezone);
  fireEvent.change(zone, { target: { value: "Mars/Phobos" } });
  fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));
  expect(push).not.toHaveBeenCalled();
  expect(zone).toHaveAttribute("aria-invalid", "true");
  expect(zone).toHaveAccessibleDescription(en.filters.errors.invalidTimezone);
  await waitFor(() => expect(zone).toHaveFocus());
});
it("does not give non-execution pagination an execution-specific notice", () => {
  mount(new URLSearchParams(), new URLSearchParams("timezone=UTC"));
  expect(screen.queryByText(en.executions.live_notice)).not.toBeInTheDocument();
});
it("always identifies the display time zone", () => {
  expect(formatAdminTime("2026-10-02T01:02:00Z")).toContain("UTC");
  expect(formatAdminTime("2026-10-02T01:02:00Z", "Asia/Shanghai")).toContain("Asia/Shanghai");
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
