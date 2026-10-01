import { expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminHealthPage } from "../health/health-page";
import { AdminSettingsPage } from "../settings/settings-page";
import { AdminWorkspacesPage } from "../workspaces/workspaces-page";
import { AdminAuditPage } from "../audit/audit-page";
import type { ReactNode } from "react";
vi.mock("@multica/core/admin", async original => ({ ...await original<typeof import("@multica/core/admin")>(),
  useAdminObservationScope: () => ({ scope: { userId: "actor" } }),
  useAdminHealth: () => ({ isPending: false, isError: false, refetch: vi.fn(), data: { asOf: "2026-10-02T00:00:00Z", dataQuality: "partial", detectorState: "unavailable", sources: [{ name: "liveness", state: "unknown", checkedAt: null, code: "not_observed" }] } }),
  useAdminSettings: () => ({ isPending: false, isError: false, refetch: vi.fn(), data: { asOf: "2026-10-02T00:00:00Z", dataQuality: "partial", configuration: { authMode: "password", registrationEnabled: false, registrationPolicy: "disabled", managedInstallationsEnabled: true, workspaceCreationEnabled: true, source: "deployment" }, retention: { confirmedOperationsDays: null, alertsDays: null, auditDays: null, automaticDeletionEnabled: false, policySource: "unknown" }, refreshIntervalsSeconds: { list: 15, detail: 5 } } }),
  useAdminWorkspaces: () => ({ isPending: false, isError: false, refetch: vi.fn(), data: { asOf: "2026-10-02T00:00:00Z", dataQuality: "complete", nextCursor: null, window: { timeFrom: "2026-10-01T00:00:00Z", timeTo: "2026-10-02T00:00:00Z", timezone: "Asia/Shanghai" }, items: [{ id: "workspace", name: "Example workspace", memberCount: 2, executionCount: 4, createdAt: "2026-09-01T00:00:00Z" }] } }),
  useAdminAudit: () => ({ isPending: false, isError: false, refetch: vi.fn(), data: { asOf: "2026-10-02T00:00:00Z", dataQuality: "partial", nextCursor: null, items: ["actor", "other"].map(actorUserId => ({ id: actorUserId, operationId: actorUserId, actorUserId, actorKind: "user", actorDisplayName: null, actorSnapshotQuality: "unknown", targetKind: "task", targetId: "target", action: "task.cancel", phase: "applied", resultCode: "cancelled_before_dispatch", requestId: "request", reason: "Operator request", beforeState: { status: "queued" }, afterState: { status: "cancelled" }, createdAt: "2026-10-02T00:00:00Z" })) } }),
}));
function mount(child: ReactNode) { return render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><NavigationProvider value={{ pathname: "/admin", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}>{child}</NavigationProvider></I18nProvider>); }
it("does not turn unobserved liveness into an all-offline or healthy claim", () => { mount(<AdminHealthPage />); expect(screen.getByText("Unavailable", { exact: true })).toBeInTheDocument(); expect(screen.queryByText("Healthy", { exact: true })).not.toBeInTheDocument(); });
it("keeps retention unknown and exposes no settings mutation controls", () => { mount(<AdminSettingsPage />); expect(screen.getByText("Automatic deletion is not enabled.")).toBeInTheDocument(); expect(screen.queryByRole("textbox")).not.toBeInTheDocument(); expect(screen.queryByText("365")).not.toBeInTheDocument(); });
it("links workspace execution counts with the same returned window", () => { mount(<AdminWorkspacesPage />); const href = screen.getByRole("link", { name: "View executions" }).getAttribute("href")!; const params = new URL(href, "https://test.invalid").searchParams; expect(params.get("workspace_id")).toBe("workspace"); expect(params.get("time_from")).toBe("2026-10-01T00:00:00Z"); expect(params.get("timezone")).toBe("Asia/Shanghai"); });
it("links only the current actor's operation and preserves missing historical names", () => { mount(<AdminAuditPage />); const links = screen.getAllByRole("link", { name: "Open own operation receipt" }); expect(links).toHaveLength(1); expect(links[0]).toHaveAttribute("href", "/admin/operations/actor"); expect(screen.getAllByText("Historical actor name was not captured")).toHaveLength(2); });
