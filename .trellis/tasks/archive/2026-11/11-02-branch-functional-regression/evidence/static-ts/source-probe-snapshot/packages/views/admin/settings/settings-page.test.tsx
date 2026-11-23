import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { AdminCollaborationGate, AdminControlDraft, AdminControlInput } from "@multica/core/admin";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import zh from "../../locales/zh-Hans/admin.json";
import { AdminSettingsPage } from "./settings-page";

const scope = "11111111-1111-4111-8111-111111111111";
const closed: AdminCollaborationGate = { scope, asOf: "2026-10-27T00:00:00Z", creationEnabled: false, executionEnabled: false, hostExecutionEnabled: true, executionEpoch: "8", allowedActions: ["open_creation", "open_execution"] };
const state = vi.hoisted(() => ({
  role: "super_admin", gate: null as AdminCollaborationGate | null | undefined, gateError: false,
  submit: vi.fn(), lookup: vi.fn(), refetch: vi.fn(), draft: null as AdminControlDraft | null,
}));
vi.mock("@multica/core/admin", async original => ({ ...await original<typeof import("@multica/core/admin")>(),
  adminApiScope: () => "scope",
  useAdminAccess: () => ({ status: "ready", identity: { userId: "actor", organizationId: scope, role: state.role } }),
  useAdminSettings: () => ({ isPending: false, isError: false, refetch: vi.fn(), data: { asOf: "2026-10-27T00:00:00Z", dataQuality: "complete", configuration: { authMode: "password", registrationEnabled: false, registrationPolicy: "disabled", managedInstallationsEnabled: true, workspaceCreationEnabled: true, collaborationMemoryMaxBytes: 131072, readOnly: true, source: "deployment" }, retention: { confirmedOperationsDays: 30, alertsDays: 30, auditDays: 90, automaticDeletionEnabled: false, policySource: "configured" }, refreshIntervalsSeconds: { list: 15, detail: 5 } } }),
  useAdminCollaborationGate: () => ({ data: state.gate, isError: state.gateError, refetch: state.refetch }),
  useAdminControlDraft: () => ({ loading: false, error: false, draft: state.draft, reload: vi.fn() }),
  useAdminControlMutation: () => ({ mutateAsync: state.submit, isPending: false, reset: vi.fn() }),
  useAdminControlLookup: () => ({ mutateAsync: state.lookup, isPending: false, reset: vi.fn() }),
  useAdminOperation: (_scope: unknown, _id: string, initialData: unknown) => ({ data: initialData, isError: false, isFetching: false, refetch: vi.fn() }),
}));
function mount(locale: "en" | "zh-Hans" = "en") {
  return render(<I18nProvider locale={locale} resources={{ en: { admin: en }, "zh-Hans": { admin: zh } }}><NavigationProvider value={{ pathname: "/admin/settings", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}>
    <AdminSettingsPage />
  </NavigationProvider></I18nProvider>);
}
function section() { return screen.getByRole("region", { name: en.settings.gate.title }); }
const receipt = { id: "operation", organizationId: scope, actorId: "actor", targetId: scope, kind: "collaboration.gate", state: "succeeded" as const, resultCode: "collaboration_creation_opened", confirmation: "not_required" as const, reconciliationState: "complete" as const, rootOperationId: null, version: 2, acceptedAt: null, updatedAt: null, appliedAt: null, confirmedAt: null, ackDeadline: null, targetKind: "collaboration_gate" };
beforeEach(() => { vi.resetAllMocks(); state.role = "super_admin"; state.gate = closed; state.gateError = false; state.draft = null; });

it("opens collaboration creation through a reasoned confirmation addressed to this server's scope", async () => {
  state.submit.mockResolvedValueOnce(receipt);
  mount();
  expect(within(section()).getAllByText("Closed")).toHaveLength(2);
  expect(within(section()).getByText("Standard host execution")).toBeVisible();
  fireEvent.click(within(section()).getByRole("button", { name: "Open new collaboration issues" }));
  expect(screen.getByRole("heading", { name: "Open new collaboration issues" })).toBeVisible();
  expect(screen.getByText(en.settings.gate.target)).toBeVisible();
  expect(screen.queryByText(scope)).not.toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Pilot collaboration" } });
  fireEvent.click(screen.getByRole("button", { name: "Confirm operation" }));
  await screen.findByText(en.operations.results.collaboration_creation_opened);
  const input = state.submit.mock.calls[0]?.[0] as AdminControlInput;
  expect(input).toMatchObject({ id: scope, action: "collaboration_gate", body: { action: "open_creation", reason: "Pilot collaboration" } });
  await waitFor(() => expect(state.refetch).toHaveBeenCalled());
});

it("shows state but no controls to an observer", () => {
  state.role = "platform_observer";
  state.gate = { ...closed, allowedActions: [] };
  mount();
  expect(within(section()).getAllByText("Closed")).toHaveLength(2);
  expect(within(section()).queryByRole("button")).not.toBeInTheDocument();
});

it("warns when creation is open without execution and never offers closing execution", () => {
  state.gate = { ...closed, creationEnabled: true, allowedActions: ["close_creation", "open_execution"] };
  const view = mount();
  expect(within(section()).getByText(en.settings.gate.creationWithoutExecution)).toBeVisible();
  view.unmount();
  state.gate = { ...closed, creationEnabled: true, executionEnabled: true, allowedActions: ["close_creation"] };
  mount();
  expect(within(section()).queryByText(en.settings.gate.creationWithoutExecution)).not.toBeInTheDocument();
  expect(within(section()).getByText(en.settings.gate.closeExecutionUnavailable)).toBeVisible();
  expect(within(section()).getAllByRole("button").map(button => button.textContent)).toEqual(["Close new collaboration issues"]);
});

it("keeps the last state and controls when a refresh fails, and omits the section on an older server", () => {
  state.gateError = true;
  const view = mount("zh-Hans");
  const region = screen.getByRole("region", { name: zh.settings.gate.title });
  expect(within(region).getByRole("alert")).toHaveTextContent(zh.settings.gate.error);
  expect(within(region).getByRole("button", { name: "开放新建协作任务" })).toBeVisible();
  view.unmount();
  state.gate = null;
  mount();
  expect(screen.queryByRole("region", { name: en.settings.gate.title })).not.toBeInTheDocument();
  expect(screen.getByText("Automatic deletion is not enabled.")).toBeVisible();
});

it("freezes the epoch seen when opening execution even if a refresh moves it", async () => {
  state.submit.mockResolvedValueOnce({ ...receipt, resultCode: "collaboration_execution_opened" });
  const view = mount();
  fireEvent.click(within(section()).getByRole("button", { name: "Open AI execution" }));
  state.gate = { ...closed, executionEpoch: "9" };
  view.rerender(<I18nProvider locale="en" resources={{ en: { admin: en } }}><NavigationProvider value={{ pathname: "/admin/settings", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}><AdminSettingsPage /></NavigationProvider></I18nProvider>);
  fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Pilot agents" } });
  fireEvent.click(screen.getByRole("button", { name: "Confirm operation" }));
  await screen.findByText(en.operations.results.collaboration_execution_opened);
  expect(state.submit.mock.calls[0]?.[0]).toMatchObject({ body: { action: "open_execution", expectedExecutionEpoch: "8", reason: "Pilot agents" } });
});

it("restores an unconfirmed gate request with its original reason and key before allowing a retry", async () => {
  const input: AdminControlInput = { id: scope, key: "22222222-2222-4222-8222-222222222222", action: "collaboration_gate", body: { action: "open_execution", expectedExecutionEpoch: "8", reason: "Saved before the reload" } };
  state.draft = { scope: { server: "https://test.invalid", userId: "actor", organizationId: scope }, input, createdAt: 1 };
  state.lookup.mockResolvedValueOnce(null);
  mount();
  expect(screen.getByRole("heading", { name: "Open AI execution" })).toBeVisible();
  await screen.findByText(en.operations.notRecorded);
  expect(state.lookup).toHaveBeenCalledWith(input);
  expect(screen.getByLabelText("Reason")).toHaveValue("Saved before the reload");
  expect(screen.getByLabelText("Reason")).toHaveAttribute("readonly");
  expect(screen.getByText(new RegExp(input.key))).toBeVisible();
  expect(screen.getByRole("button", { name: en.operations.retryOriginal })).toBeEnabled();
  expect(screen.getByRole("button", { name: en.operations.check })).toBeVisible();
});

it("shows the committed refusal when the switches changed before the request", async () => {
  state.submit.mockResolvedValueOnce({ ...receipt, state: "failed", resultCode: "collaboration_gate_conflict" });
  mount();
  fireEvent.click(within(section()).getByRole("button", { name: "Open new collaboration issues" }));
  fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Prepared earlier" } });
  fireEvent.click(screen.getByRole("button", { name: "Confirm operation" }));
  expect(await screen.findByText(en.operations.results.collaboration_gate_conflict)).toBeVisible();
  expect(screen.getByText(en.operations.states.failed)).toBeVisible();
});
