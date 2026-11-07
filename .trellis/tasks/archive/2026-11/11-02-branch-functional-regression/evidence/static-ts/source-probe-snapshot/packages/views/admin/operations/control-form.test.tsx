import { beforeEach, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { ApiError } from "@multica/core/api";
import { AdminControlUncertainError, type AdminControlInput } from "@multica/core/admin";
import { NavigationProvider } from "../../navigation";
import en from "../../locales/en/admin.json";
import { AdminControlForm, type AdminControlTarget } from "./control-form";
const state = vi.hoisted(() => ({ submit: vi.fn(), lookup: vi.fn(), refresh: vi.fn(), close: vi.fn(), reset: vi.fn() }));
vi.mock("@multica/core/admin", async original => ({ ...await original<typeof import("@multica/core/admin")>(),
  useAdminControlMutation: () => ({ mutateAsync: state.submit, isPending: false, reset: state.reset }),
  useAdminControlLookup: () => ({ mutateAsync: state.lookup, isPending: false, reset: state.reset }),
  useAdminOperation: (_scope: unknown, _id: string, initialData: unknown) => ({ data: initialData, isError: false, isFetching: false, refetch: vi.fn() }),
}));
function mount(restoredInput?: AdminControlInput, target: AdminControlTarget = { id: "target", action: "cancel", fence: { runtimeId: null, dispatchedAt: null, targetVersion: "5" } }, searchParams = new URLSearchParams()) {
  return render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><NavigationProvider value={{ pathname: "/admin/tasks/target", searchParams, hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}>
    <AdminControlForm scope={{ apiScope: "scope", userId: "actor", organizationId: "organization" }} target={target} restoredInput={restoredInput} onClose={state.close} onRefresh={state.refresh} />
  </NavigationProvider></I18nProvider>);
}
beforeEach(() => vi.resetAllMocks());
it.each<AdminControlTarget>([
  { id: "target", action: "cancel", fence: { runtimeId: null, dispatchedAt: null, targetVersion: "5" } },
  { id: "target", action: "admission", admission: "stopped", version: "5" },
  { id: "target", action: "alert", alertAction: "acknowledge", version: "5" },
])("identifies and focuses a whitespace-only reason for $action without sending a request", target => {
  mount(undefined, target);
  const reason = screen.getByLabelText("Reason");
  fireEvent.change(reason, { target: { value: "   " } });
  screen.getByRole("button", { name: "Confirm operation" }).focus();
  fireEvent.click(screen.getByRole("button", { name: "Confirm operation" }));
  expect(state.submit).not.toHaveBeenCalled();
  expect(reason).toHaveAttribute("aria-invalid", "true");
  expect(reason).toHaveAccessibleDescription("Enter a reason. Spaces alone are not enough.");
  expect(reason).toHaveFocus();
  expect(screen.getByRole("alert")).toHaveTextContent("Enter a reason");
  fireEvent.change(reason, { target: { value: "Operator request" } });
  expect(reason).not.toHaveAttribute("aria-invalid", "true");
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});
// Protocol/state matrices are canonical in core/admin/operation-*.test.ts.
it("retains the original key and frozen request after a lost response and empty lookup", async () => {
  state.submit.mockRejectedValueOnce(new AdminControlUncertainError("ignored"));
  state.lookup.mockResolvedValueOnce(null);
  mount();
  fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Operator request" } });
  fireEvent.click(screen.getByRole("button", { name: "Confirm operation" }));
  await screen.findByText(/The outcome is uncertain/);
  const original = state.submit.mock.calls[0]?.[0];
  expect(screen.getByLabelText("Reason")).toHaveAttribute("readonly");
  expect(screen.getByRole("button", { name: "Close" })).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: "Check original request" }));
  await screen.findByText(/No receipt is visible yet/);
  state.submit.mockRejectedValueOnce(new AdminControlUncertainError(original.key));
  fireEvent.click(screen.getByRole("button", { name: "Retry original request" }));
  await waitFor(() => expect(state.submit).toHaveBeenCalledTimes(2));
  expect(state.submit.mock.calls[1]?.[0]).toEqual(original);
  expect(original.body.expectedExecutionFence).toEqual({ runtimeId: null, dispatchedAt: null, targetVersion: "5" });
  expect(screen.queryByLabelText(/Password/)).not.toBeInTheDocument();
});
it("shows server application independently of an unconfirmed process stop", async () => {
  state.submit.mockResolvedValueOnce({ id: "operation", targetId: "target", kind: "task.cancel", state: "applied", confirmation: "unconfirmed", reconciliationState: "unconfirmed", resultCode: "awaiting_daemon_confirmation", acceptedAt: null, updatedAt: null });
  mount();
  fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Operator request" } });
  fireEvent.click(screen.getByRole("button", { name: "Confirm operation" }));
  expect(await screen.findByText("Applied on server")).toBeInTheDocument();
  expect(screen.getByText("Process stop unconfirmed")).toBeInTheDocument();
  expect(screen.queryByText("Operation completed")).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Open receipt" }).getAttribute("href")).toMatch(/^\/admin\/operations\/operation\?/);
});
it("retains the investigation timezone through confirmation and receipt navigation", async () => {
  state.submit.mockResolvedValueOnce({ id: "operation", targetId: "target", kind: "task.cancel", state: "applied", confirmation: "unconfirmed", reconciliationState: "unconfirmed", resultCode: "awaiting_daemon_confirmation", acceptedAt: null, updatedAt: null });
  const returnTo = "/admin/tasks?status=running&timezone=Asia%2FShanghai";
  mount(undefined, { id: "target", action: "cancel", fence: { runtimeId: "runtime", dispatchedAt: "2026-10-02T01:02:00Z", targetVersion: "5" } }, new URLSearchParams({ timezone: "Asia/Shanghai", return_to: returnTo }));
  expect(screen.getByText(/9:02/)).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Operator request" } });
  fireEvent.click(screen.getByRole("button", { name: "Confirm operation" }));
  const link = await screen.findByRole("link", { name: "Open receipt" });
  const receipt = new URL(link.getAttribute("href")!, "https://example.test");
  expect(receipt.searchParams.get("timezone")).toBe("Asia/Shanghai");
  expect(receipt.searchParams.get("return_to")).toBe(returnTo);
});
it("requires target refresh after a stale-version conflict", async () => {
  state.submit.mockRejectedValueOnce(new ApiError("Stale", 409, "Conflict"));
  mount();
  fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Operator request" } });
  fireEvent.click(screen.getByRole("button", { name: "Confirm operation" }));
  await screen.findByText(/The target changed/);
  expect(screen.getByRole("button", { name: "Confirm operation" })).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: "Refresh target and close" }));
  expect(state.refresh).toHaveBeenCalled();
  await waitFor(() => expect(state.close).toHaveBeenCalled());
});
it("reconciles a restored request before enabling a same-key retry", async () => {
  const restored: AdminControlInput = { id: "target", key: "original-key", action: "cancel", body: { expectedExecutionFence: { runtimeId: null, dispatchedAt: null, targetVersion: "5" }, reason: "Original reason" } };
  let resolveLookup!: (value: null) => void;
  state.lookup.mockReturnValueOnce(new Promise<null>(resolve => { resolveLookup = resolve; }));
  mount(restored);
  expect(state.lookup).toHaveBeenCalledWith(restored);
  expect(screen.getByRole("button", { name: "Submitting..." })).toBeDisabled();
  expect(screen.getByLabelText("Reason")).toHaveValue("Original reason");
  resolveLookup(null);
  await screen.findByText(/No receipt is visible yet/);
  state.submit.mockRejectedValueOnce(new AdminControlUncertainError(restored.key));
  fireEvent.click(screen.getByRole("button", { name: "Retry original request" }));
  await waitFor(() => expect(state.submit).toHaveBeenCalledWith(restored));
});
it("restores a frozen alert acknowledgement through the shared original-key lookup", async () => {
  const input: AdminControlInput = { id: "target", key: "original-alert-key", action: "alert", body: { action: "acknowledge", expectedVersion: "2", reason: "Investigating" } };
  state.lookup.mockResolvedValueOnce(null);
  mount(input, { id: "target", action: "alert", alertAction: "acknowledge", version: "2" });
  await screen.findByText(/No receipt is visible yet/);
  expect(screen.getByRole("heading", { name: "Acknowledge alert" })).toBeInTheDocument();
  expect(screen.getByLabelText("Reason")).toHaveValue("Investigating");
  state.submit.mockRejectedValueOnce(new AdminControlUncertainError(input.key));
  fireEvent.click(screen.getByRole("button", { name: "Retry original request" }));
  await waitFor(() => expect(state.submit).toHaveBeenCalledWith(input));
});
it("shows the alert receipt and returns to that alert without claiming condition recovery", async () => {
  state.submit.mockResolvedValueOnce({ id: "operation", targetId: "target", kind: "alert.acknowledge", state: "succeeded", confirmation: "not_required", reconciliationState: "complete", resultCode: "alert_acknowledged" });
  render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><NavigationProvider value={{ pathname: "/admin/alerts/target", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}><AdminControlForm scope={{ apiScope: "scope", userId: "actor", organizationId: "organization" }} target={{ id: "target", action: "alert", alertAction: "acknowledge", version: "2" }} onClose={state.close} onRefresh={state.refresh} /></NavigationProvider></I18nProvider>);
  fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Investigating" } });
  fireEvent.click(screen.getByRole("button", { name: "Confirm operation" }));
  expect(await screen.findByText("The alert was acknowledged. Recovery remains a separate observation.")).toBeInTheDocument();
});
it("leaves a conclusively rejected old admission request only after the current target is refreshed", async () => {
  const restored: AdminControlInput = { id: "target", key: "original-key", action: "admission", body: { admission: "stopped", expectedAdmissionVersion: "5", reason: "Original reason" } };
  state.lookup.mockResolvedValueOnce(null);
  state.submit.mockRejectedValueOnce(new ApiError("Obsolete version", 409, "Conflict", { code: "admission_version_conflict" }));
  let finishRefresh!: () => void;
  state.refresh.mockReturnValueOnce(new Promise<void>(resolve => { finishRefresh = resolve; }));
  mount(restored, { id: "target", action: "admission", admission: "stopped", version: "5" });
  await screen.findByText(/No receipt is visible yet/);
  fireEvent.click(screen.getByRole("button", { name: "Retry original request" }));
  await screen.findByText(/The target changed/);
  expect(screen.queryByRole("button", { name: "Check original request" })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Refresh target and close" }));
  expect(state.close).not.toHaveBeenCalled();
  finishRefresh();
  await waitFor(() => expect(state.close).toHaveBeenCalledOnce());
});
