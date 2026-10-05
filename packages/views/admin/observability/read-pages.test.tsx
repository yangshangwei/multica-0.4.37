import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import zh from "../../locales/zh-Hans/admin.json";
import { AdminHealthPage } from "../health/health-page";
import { AdminSettingsPage } from "../settings/settings-page";
import { AdminWorkspacesPage } from "../workspaces/workspaces-page";
import { AdminAuditPage } from "../audit/audit-page";
import type { ReactNode } from "react";
const state = vi.hoisted(() => ({ actorName: null as string | null, copyText: vi.fn(), auditAction: "task.cancel", auditResult: "cancelled_before_dispatch", auditPhase: "applied", targetKind: "task", targetId: "target" }));
vi.mock("@multica/ui/lib/clipboard", () => ({ copyText: state.copyText }));
beforeEach(() => { state.actorName = null; state.copyText.mockReset().mockResolvedValue(true); state.auditAction = "task.cancel"; state.auditResult = "cancelled_before_dispatch"; state.auditPhase = "applied"; state.targetKind = "task"; state.targetId = "target"; });
vi.mock("@multica/core/admin", async original => ({ ...await original<typeof import("@multica/core/admin")>(),
  useAdminObservationScope: () => ({ scope: { userId: "actor" } }),
  useAdminHealth: () => ({ isPending: false, isError: false, refetch: vi.fn(), data: { asOf: "2026-10-02T00:00:00Z", dataQuality: "partial", detectorState: "unavailable", sources: [{ name: "liveness", state: "unknown", checkedAt: null, code: "not_observed" }] } }),
  useAdminSettings: () => ({ isPending: false, isError: false, refetch: vi.fn(), data: { asOf: "2026-10-02T00:00:00Z", dataQuality: "partial", configuration: { authMode: "password", registrationEnabled: false, registrationPolicy: "disabled", managedInstallationsEnabled: true, workspaceCreationEnabled: true, source: "deployment" }, retention: { confirmedOperationsDays: null, alertsDays: null, auditDays: null, automaticDeletionEnabled: false, policySource: "unknown" }, refreshIntervalsSeconds: { list: 15, detail: 5 } } }),
  useAdminWorkspaces: () => ({ isPending: false, isError: false, refetch: vi.fn(), data: { asOf: "2026-10-02T00:00:00Z", dataQuality: "complete", nextCursor: null, window: { timeFrom: "2026-10-01T00:00:00Z", timeTo: "2026-10-02T00:00:00Z", timezone: "Asia/Shanghai" }, items: [{ id: "workspace", name: "Example workspace", memberCount: 2, executionCount: 4, createdAt: "2026-09-01T00:00:00Z" }] } }),
  useAdminAudit: () => ({ isPending: false, isError: false, refetch: vi.fn(), data: { asOf: "2026-10-02T00:00:00Z", dataQuality: "partial", nextCursor: null, items: ["actor", "other"].map(actorUserId => ({ id: actorUserId, operationId: actorUserId, actorUserId, actorKind: "user", actorDisplayName: state.actorName, actorSnapshotQuality: "unknown", targetKind: state.targetKind, targetId: state.targetId, action: state.auditAction, phase: state.auditPhase, resultCode: state.auditResult, requestId: "request", reason: "Operator request", beforeState: { status: "queued" }, afterState: { status: "cancelled" }, createdAt: "2026-10-02T00:00:00Z" })) } }),
}));
function mount(child: ReactNode, locale: "en" | "zh-Hans" = "en") { return render(<I18nProvider locale={locale} resources={{ en: { admin: en }, "zh-Hans": { admin: zh } }}><NavigationProvider value={{ pathname: "/admin", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}>{child}</NavigationProvider></I18nProvider>); }
it("does not turn unobserved liveness into an all-offline or healthy claim", () => { mount(<AdminHealthPage />); expect(screen.getByText("Unavailable", { exact: true })).toBeInTheDocument(); expect(screen.queryByText("Healthy", { exact: true })).not.toBeInTheDocument(); });
it("keeps retention unknown and exposes no settings mutation controls", () => { mount(<AdminSettingsPage />); expect(screen.getByText("Automatic deletion is not enabled.")).toBeInTheDocument(); expect(screen.queryByRole("textbox")).not.toBeInTheDocument(); expect(screen.queryByText("365")).not.toBeInTheDocument(); });
it("links workspace execution counts with the same returned window", () => { mount(<AdminWorkspacesPage />); for (const link of screen.getAllByRole("link", { name: "View executions" })) { const href = link.getAttribute("href")!; const params = new URL(href, "https://test.invalid").searchParams; expect(params.get("workspace_id")).toBe("workspace"); expect(params.get("time_from")).toBe("2026-10-01T00:00:00Z"); expect(params.get("timezone")).toBe("Asia/Shanghai"); } });
it("links only the current actor's operation and preserves missing historical names", () => { mount(<AdminAuditPage />); for (const summary of screen.getAllByText("Event details")) fireEvent.click(summary); const links = screen.getAllByRole("link", { name: "Open own operation receipt" }); expect(links).toHaveLength(1); expect(new URL(links[0]!.getAttribute("href")!, "https://test.invalid").pathname).toBe("/admin/operations/actor"); expect(screen.getAllByText("Historical actor name was not captured")).toHaveLength(2); });

it("keeps workspace counts and its full identifier reachable in the compact list", () => {
  mount(<AdminWorkspacesPage />);
  const list = screen.getByRole("list", { name: "Workspaces" });
  expect(within(list).getByText("Example workspace")).toBeVisible();
  expect(within(list).getByText("2")).toBeVisible();
  expect(within(list).getByText("4")).toBeVisible();
  const details = list.querySelector("details")!;
  fireEvent.click(within(list).getByText("Workspace details"));
  expect(details).toHaveAttribute("open");
  expect(within(details).getByText("workspace")).toBeVisible();
  expect(within(details).getByText(/Asia\/Shanghai/)).toBeVisible();
});

it("does not show execution-only pagination notices on workspace or audit data", () => {
  const view = mount(<AdminWorkspacesPage />);
  expect(screen.queryByText(en.executions.live_notice)).not.toBeInTheDocument();
  view.unmount();
  mount(<AdminAuditPage />);
  expect(screen.queryByText(en.executions.live_notice)).not.toBeInTheDocument();
});

it("keeps audit request IDs and snapshots inside collapsed event details", () => {
  mount(<AdminAuditPage />);
  expect(screen.getAllByRole("heading", { name: "Cancel execution" })[0]).toBeVisible();
  expect(screen.getAllByText("request")[0]).not.toBeVisible();
  expect(screen.getAllByText("queued", { exact: false })[0]).not.toBeVisible();
  fireEvent.click(screen.getAllByText("Event details")[0]!);
  expect(screen.getAllByText("request")[0]).toBeVisible();
});

it("localizes known audit codes while retaining raw codes and full targets in details", () => {
  state.actorName = "历史管理员";
  state.auditAction = "installation.enroll";
  state.auditResult = "applied";
  state.targetKind = "installation";
  state.targetId = "87654321-1234-5678-9012-123456789012";
  mount(<AdminAuditPage />, "zh-Hans");
  expect(screen.getAllByRole("heading", { name: "终端注册" })[0]).toBeVisible();
  expect(screen.getAllByText("历史管理员")[0]).toBeVisible();
  expect(screen.getAllByText("已应用")[0]).toBeVisible();
  expect(screen.getAllByText("终端", { exact: true })[0]).toBeVisible();
  expect(screen.getAllByText("87654321…9012")[0]).toBeVisible();
  expect(screen.getAllByText("installation.enroll")[0]).not.toBeVisible();
  expect(screen.getAllByText(state.targetId)[0]).not.toBeVisible();
  fireEvent.click(screen.getAllByText(zh.audit.details)[0]!);
  const details = screen.getAllByText(zh.audit.details)[0]!.closest("details")!;
  expect(within(details).getByText("installation.enroll")).toBeVisible();
  expect(within(details).getAllByText("applied")).toHaveLength(2);
  expect(within(details).getByText(state.targetId)).toBeVisible();
  expect(screen.queryByText("操作成功", { exact: true })).not.toBeInTheDocument();
});

it("falls back to raw unknown audit codes instead of presenting them as a known result", () => {
  state.auditAction = "future.action";
  state.auditResult = "future_result";
  state.auditPhase = "future_phase";
  state.targetKind = "future_target";
  mount(<AdminAuditPage />);
  expect(screen.getAllByRole("heading", { name: "future.action" })[0]).toBeVisible();
  expect(screen.getAllByText("future_result")[0]).toBeVisible();
  expect(screen.getAllByText("future_target")[0]).toBeVisible();
  fireEvent.click(screen.getAllByText("Event details")[0]!);
  expect(screen.getAllByText("future_phase")[0]).toBeVisible();
  expect(screen.queryByText("Applied", { exact: true })).not.toBeInTheDocument();
});

it("shows the actor's historical name separately from the copyable filtering ID", async () => {
  state.actorName = "Historical name";
  mount(<AdminAuditPage />);
  expect(screen.getAllByText("Historical name")[0]).toBeVisible();
  fireEvent.click(screen.getAllByText("Event details")[0]!);
  const copy = screen.getAllByRole("button", { name: "Copy actor ID" })[0]!;
  fireEvent.click(copy);
  expect(state.copyText).toHaveBeenCalledWith("actor");
  expect(await screen.findByText("Actor ID copied")).toBeVisible();
  expect(screen.getAllByText("Actor ID").some(label => label.nextElementSibling?.textContent?.includes("actor"))).toBe(true);
});

it("keeps the actor ID available for manual copying when clipboard access fails", async () => {
  state.actorName = "Historical name";
  state.copyText.mockResolvedValue(false);
  mount(<AdminAuditPage />);
  fireEvent.click(screen.getAllByText("Event details")[0]!);
  fireEvent.click(screen.getAllByRole("button", { name: "Copy actor ID" })[0]!);
  expect(await screen.findByRole("alert")).toHaveTextContent("Could not copy. Select and copy the actor ID below.");
  expect(screen.getByText("actor", { exact: true })).toBeVisible();
  expect(screen.queryByText("Actor ID copied")).not.toBeInTheDocument();
});
