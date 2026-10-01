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
function mount(restoredInput?: AdminControlInput, target: AdminControlTarget = { id: "target", action: "cancel", fence: { runtimeId: null, dispatchedAt: null, targetVersion: "5" } }) {
  return render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><NavigationProvider value={{ pathname: "/admin/tasks/target", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: p => p }}>
    <AdminControlForm scope={{ apiScope: "scope", userId: "actor", organizationId: "organization" }} target={target} restoredInput={restoredInput} onClose={state.close} onRefresh={state.refresh} />
  </NavigationProvider></I18nProvider>);
}
beforeEach(() => vi.resetAllMocks());
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
  expect(screen.getByRole("link", { name: "Open receipt" })).toHaveAttribute("href", "/admin/operations/operation");
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
