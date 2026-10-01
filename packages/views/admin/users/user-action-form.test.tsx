import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import { ApiError } from "@multica/core/api";
import { AdminUserActionForm } from "./user-action-form";
import en from "../../locales/en/admin.json";

const state = vi.hoisted(() => ({
  mutateAsync: vi.fn(), reset: vi.fn(), find: vi.fn(), close: vi.fn(), refresh: vi.fn(),
  Uncertain: class extends Error {},
}));
vi.mock("@multica/core/admin", () => ({
  useAdminUserMutation: () => ({ mutateAsync: state.mutateAsync, reset: state.reset, isPending: false }),
  findAdminUserOperation: (...args: unknown[]) => state.find(...args),
  AdminUserOperationUncertainError: state.Uncertain,
}));
const user = { id: "target", name: "Example account", username: "example", platformRole: null, status: "active" as const, authVersion: 4, workspaceCount: 0, createdAt: "2026-10-01T00:00:00Z", allowedActions: ["role"] };
const scope = { apiScope: "server", userId: "actor", organizationId: "organization" };
function mount() {
  return render(<I18nProvider locale="en" resources={{ en: { admin: en } }}><AdminUserActionForm user={user} action="role" scope={scope} onClose={state.close} onRefresh={state.refresh} /></I18nProvider>);
}
beforeEach(() => vi.clearAllMocks());

describe("administrative account confirmation", () => {
  it("preserves the operation key and chosen role after an unknown result", async () => {
    state.mutateAsync.mockRejectedValueOnce(new state.Uncertain("unknown result"));
    mount();
    fireEvent.change(screen.getByLabelText("Platform role"), { target: { value: "super_admin" } });
    fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Promote named operator" } });
    fireEvent.change(screen.getByLabelText("Your current password"), { target: { value: "current-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Confirm change" }));
    await screen.findByRole("alert");
    expect(screen.getByLabelText("Your current password")).toHaveValue("");
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    state.mutateAsync.mockResolvedValueOnce({ id: "operation", targetId: "target", state: "applied" });
    fireEvent.change(screen.getByLabelText("Your current password"), { target: { value: "current-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Confirm change" }));
    await screen.findByText("The change was applied.");
    const [first, second] = state.mutateAsync.mock.calls.map((call) => call[0]);
    expect(second.key).toBe(first.key);
    expect(second.body.role).toBe("super_admin");
    expect(second.body.reason).toBe("Promote named operator");
  });
  it("keeps password errors in the confirmation form and clears the secret", async () => {
    state.mutateAsync.mockRejectedValueOnce(new ApiError("Invalid password", 403, "Forbidden", { code: "password_verification_failed" }));
    mount();
    fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Access review" } });
    fireEvent.change(screen.getByLabelText("Your current password"), { target: { value: "wrong-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Confirm change" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Your password could not be verified"));
    expect(screen.getByLabelText("Your current password")).toHaveValue("");
    expect(screen.getByLabelText("Reason")).toHaveValue("Access review");
    expect(state.close).not.toHaveBeenCalled();
    expect(state.reset).toHaveBeenCalled();
  });
});
