import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminAlertDetailPage } from "./alert-detail-page";
const state = vi.hoisted(() => ({ role: "platform_observer", status: "open", rule: "queue_timeout" }));
vi.mock("@multica/core/admin", async original => ({ ...await original<typeof import("@multica/core/admin")>(),
  useAdminObservationScope: () => ({ scope: { apiScope: "scope", userId: "actor", organizationId: "org" }, identity: { role: state.role }, enabled: true }),
  useAdminControlDraft: () => ({ loading: false, error: false, draft: null, reload: vi.fn() }),
  useAdminAlert: () => ({ isPending: false, isError: false, refetch: vi.fn(), data: { id: "alert", organizationId: "org", rule: state.rule, status: state.status, severity: "warning", subjectKind: "task", subjectId: "task", firstSeenAt: "2026-10-01T00:00:00Z", lastSeenAt: "2026-10-02T00:00:00Z", occurrenceCount: 1, version: "1", allowedActions: ["acknowledge", "assign", "close"], assigneeId: null, acknowledgedAt: null, resolvedAt: null, closedAt: null, conditionActive: true, resolutionCode: null, relatedTaskId: null } }),
}));
function mount() { return render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><NavigationProvider value={{ pathname: "/admin/alerts/alert", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}><AdminAlertDetailPage id="alert" /></NavigationProvider></I18nProvider>); }
beforeEach(() => { state.role = "platform_observer"; state.status = "open"; state.rule = "queue_timeout"; });
it("shows condition and acknowledgement separately without observer write controls", () => {
  mount();
  expect(screen.getByText("Still active")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Acknowledge alert" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Assign administrator" })).not.toBeInTheDocument();
});
it("keeps an unresolved queue alert open even when allowed actions drift", () => {
  state.role = "super_admin"; mount();
  expect(screen.getByRole("button", { name: "Acknowledge alert" })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Close alert" })).not.toBeInTheDocument();
});
it("shows a closed failed execution as a historical fact, not an active live condition", () => {
  state.rule = "execution_failed"; state.status = "closed"; mount();
  expect(screen.getByText("Recorded failure")).toBeInTheDocument();
  expect(screen.getByText("Closing this alert records its handling; it does not change the failed execution.")).toBeInTheDocument();
  expect(screen.queryByText("Still active")).not.toBeInTheDocument();
  expect(screen.queryByText("Recovered", { exact: true })).not.toBeInTheDocument();
});
