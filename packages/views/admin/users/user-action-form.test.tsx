import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { SupportedLocale } from "@multica/core/i18n";
import { ApiError } from "@multica/core/api";
import type { AdminAccountAction, AdminUser } from "@multica/core/admin";
import { AdminUserActionForm } from "./user-action-form";
import en from "../../locales/en/admin.json";
import zh from "../../locales/zh-Hans/admin.json";

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
function mount(action: AdminAccountAction | "role" = "role", overrides: Partial<AdminUser> = {}, locale: SupportedLocale = "en") {
  return render(<I18nProvider locale={locale} resources={{ en: { admin: en }, "zh-Hans": { admin: zh } }}><AdminUserActionForm user={{ ...user, ...overrides }} action={action} scope={scope} onClose={state.close} onRefresh={state.refresh} /></I18nProvider>);
}
beforeEach(() => vi.resetAllMocks());

function fillReset(temporaryPassword = "temporary-password", confirmation = temporaryPassword) {
  fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "Help the account regain access" } });
  fireEvent.change(screen.getByLabelText("Target account's temporary password"), { target: { value: temporaryPassword } });
  fireEvent.change(screen.getByLabelText("Confirm temporary password"), { target: { value: confirmation } });
  fireEvent.change(screen.getByLabelText("Your current administrator password"), { target: { value: "administrator-password" } });
}

describe("administrative account confirmation", () => {
  it("rejects a whitespace-only reason with focused field feedback before any mutation", () => {
    mount();
    const reason = screen.getByLabelText("Reason");
    fireEvent.change(reason, { target: { value: "   " } });
    fireEvent.change(screen.getByLabelText("Your current password"), { target: { value: "current-password" } });
    fireEvent.click(screen.getByRole("button", { name: "Confirm change" }));
    expect(state.mutateAsync).not.toHaveBeenCalled();
    expect(reason).toHaveAttribute("aria-invalid", "true");
    expect(reason).toHaveAccessibleDescription(en.operations.reasonRequired);
    expect(reason).toHaveFocus();
    fireEvent.change(reason, { target: { value: "Access review" } });
    expect(reason).not.toHaveAttribute("aria-invalid", "true");
  });
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

describe("administrator password reset", () => {
  it("explains the target impact and distinguishes both passwords with accessible visibility controls", () => {
    mount("recover-password");
    expect(screen.getByRole("heading", { name: "Reset password: Example account" })).toBeInTheDocument();
    expect(screen.getByText(en.users.resetNotice)).toBeInTheDocument();
    expect(screen.getByText(en.users.processNotice)).toBeInTheDocument();
    const password = screen.getByLabelText("Target account's temporary password");
    const confirmation = screen.getByLabelText("Confirm temporary password");
    const adminPassword = screen.getByLabelText("Your current administrator password");
    expect(password).toHaveAccessibleDescription(en.users.tempPasswordHint);
    expect(adminPassword).toHaveAccessibleDescription(en.users.adminPasswordHint);
    expect(password).toHaveAttribute("type", "password");
    expect(confirmation).toHaveAttribute("type", "password");
    fireEvent.click(screen.getByRole("button", { name: "Show temporary passwords" }));
    expect(password).toHaveAttribute("type", "text");
    expect(confirmation).toHaveAttribute("type", "text");
    expect(adminPassword).toHaveAttribute("type", "password");
    fireEvent.click(screen.getByRole("button", { name: "Hide temporary passwords" }));
    expect(password).toHaveAttribute("type", "password");
    expect(confirmation).toHaveAttribute("type", "password");
    expect(state.mutateAsync).not.toHaveBeenCalled();
  });

  it("rejects mismatched confirmation before submission and focuses the field", () => {
    mount("recover-password");
    fillReset("temporary-password", "different-password");
    fireEvent.click(screen.getByRole("button", { name: "Reset password" }));
    const confirmation = screen.getByLabelText("Confirm temporary password");
    expect(state.mutateAsync).not.toHaveBeenCalled();
    expect(confirmation).toHaveFocus();
    expect(confirmation).toHaveAttribute("aria-invalid", "true");
    expect(confirmation).toHaveAccessibleDescription(en.users.tempPasswordMismatch);
    fireEvent.change(confirmation, { target: { value: "temporary-password" } });
    expect(confirmation).not.toHaveAttribute("aria-invalid", "true");
  });

  it("requires confirmation before sending a reset", () => {
    mount("recover-password");
    fillReset("temporary-password", "");
    expect(screen.getByLabelText("Confirm temporary password")).toBeRequired();
    fireEvent.click(screen.getByRole("button", { name: "Reset password" }));
    expect(state.mutateAsync).not.toHaveBeenCalled();
  });

  it("clears every password after an uncertain result and permits confirmation again with the same operation key", async () => {
    state.mutateAsync.mockRejectedValueOnce(new state.Uncertain("unknown result"));
    mount("recover-password", { username: null, status: "setup_required" });
    fireEvent.change(screen.getByLabelText("Initial username"), { target: { value: "new_user" } });
    fillReset();
    const inputs = [screen.getByLabelText("Target account's temporary password"), screen.getByLabelText("Confirm temporary password"), screen.getByLabelText("Your current administrator password")];
    fireEvent.click(screen.getByRole("button", { name: "Show temporary passwords" }));
    fireEvent.click(screen.getByRole("button", { name: "Reset password" }));
    await screen.findByText(en.users.uncertain);
    for (const input of inputs) {
      expect(input).toHaveValue("");
      expect(input).toHaveAttribute("type", "password");
      expect(input).not.toHaveAttribute("readonly");
    }
    expect(screen.getByLabelText("Reason")).toHaveAttribute("readonly");
    expect(screen.getByLabelText("Initial username")).toHaveAttribute("readonly");
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(screen.getByText(en.users.resetRetryNotice)).toBeInTheDocument();
    state.mutateAsync.mockResolvedValueOnce({ id: "operation", targetId: "target", state: "applied" });
    fillReset();
    fireEvent.click(screen.getByRole("button", { name: "Reset password" }));
    await screen.findByText(en.users.resetSucceeded);
    const [first, second] = state.mutateAsync.mock.calls.map((call) => call[0]);
    expect(second).toEqual(first);
    expect(second.body).toEqual({ expectedAuthVersion: 4, reason: "Help the account regain access", password: "administrator-password", temporaryPassword: "temporary-password", username: "new_user" });
    for (const input of inputs) expect(input).toHaveValue("");
    expect(screen.getByText(en.users.recoveryNotice)).toBeInTheDocument();
    expect(screen.queryByText("temporary-password")).not.toBeInTheDocument();
  });

  it("resolves an uncertain reset through its receipt without re-entering any secrets", async () => {
    state.mutateAsync.mockRejectedValueOnce(new state.Uncertain("unknown result"));
    state.find.mockResolvedValueOnce({ targetId: "target", kind: "user.password.recover", state: "applied" });
    mount("recover-password");
    fillReset();
    fireEvent.click(screen.getByRole("button", { name: "Reset password" }));
    await screen.findByText(en.users.uncertain);
    fireEvent.click(screen.getByRole("button", { name: "Check operation status" }));
    await screen.findByText(en.users.resetSucceeded);
    expect(state.find).toHaveBeenCalledWith(scope, state.mutateAsync.mock.calls[0]?.[0].key);
    expect(state.mutateAsync).toHaveBeenCalledTimes(1);
    expect(state.refresh).toHaveBeenCalledOnce();
  });

  it("keeps a taken initial username editable without locking the form as a version conflict", async () => {
    state.mutateAsync.mockRejectedValueOnce(new ApiError("Taken", 409, "Conflict", { code: "username_taken" }));
    mount("recover-password", { username: null });
    fireEvent.change(screen.getByLabelText("Initial username"), { target: { value: "existing_user" } });
    fillReset();
    fireEvent.click(screen.getByRole("button", { name: "Reset password" }));
    await screen.findByText(en.users.usernameTaken);
    expect(screen.getByRole("button", { name: "Reset password" })).toBeEnabled();
    expect(screen.getByLabelText("Initial username")).not.toHaveAttribute("readonly");
    expect(screen.getByLabelText("Confirm temporary password")).toHaveValue("");
  });

  it("states that resetting a disabled account does not restore it, including after success", async () => {
    state.mutateAsync.mockResolvedValueOnce({ state: "applied" });
    mount("recover-password", { status: "disabled" });
    expect(screen.getByText(en.users.resetDisabledNotice)).toBeInTheDocument();
    fillReset();
    fireEvent.click(screen.getByRole("button", { name: "Reset password" }));
    await screen.findByText(en.users.resetSucceeded);
    expect(screen.getByText(en.users.resetDisabledNotice)).toBeInTheDocument();
  });

  it("provides the reset controls and guidance in Chinese", () => {
    mount("recover-password", {}, "zh-Hans");
    expect(screen.getByRole("button", { name: "重置密码" })).toBeInTheDocument();
    expect(screen.getByLabelText("目标账号的临时密码")).toBeInTheDocument();
    expect(screen.getByLabelText("确认临时密码")).toBeInTheDocument();
    expect(screen.getByLabelText("你当前的管理员密码")).toBeInTheDocument();
    expect(screen.getByText(zh.users.resetNotice)).toBeInTheDocument();
  });
});
