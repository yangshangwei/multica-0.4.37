import { act, cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { User } from "@multica/core/types";
import { useAuthStore } from "@multica/core/auth";
import { renderWithI18n } from "../../test/i18n";

const updateMe = vi.hoisted(() => vi.fn());
const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...await importOriginal<typeof import("@multica/core/api")>(),
  api: { updateMe, getBaseUrl: () => "https://api.example.com" },
}));
vi.mock("sonner", () => ({ toast: notices }));
vi.mock("@multica/core/auth", async () => {
  const { create } = await import("zustand");
  return {
    useAuthStore: create<{ user: User | null; setUser: (user: User) => void }>((set) => ({
      user: null, setUser: (user) => set({ user }),
    })),
  };
});
vi.mock("@multica/core/hooks/use-file-upload", () => ({
  useFileUpload: () => ({ upload: async () => ({ link: "/avatar.png" }) }),
}));
// Cropping geometry is covered by common/avatar-crop.test.ts. Keep the real
// upload control here to verify the settings caller's persistence contract.
vi.mock("../../common/avatar-crop-dialog", () => ({
  AvatarCropDialog: ({ open, busy, onCropped }: {
    open: boolean; busy: boolean; onCropped: (file: File) => void;
  }) => open ? <button disabled={busy} onClick={() => onCropped(new File(["image"], "avatar.png", { type: "image/png" }))}>Save cropped avatar</button> : null,
}));

import { AccountTab } from "./account-tab";

const account: User = {
  id: "user-1", name: "Alex", email: "alex@example.com", avatar_url: null,
  profile_description: "Original profile", timezone: null, language: "en",
  onboarded_at: null, onboarding_questionnaire: {}, starter_content_state: null,
  created_at: "2026-10-14T00:00:00Z", updated_at: "2026-10-14T00:00:00Z",
};

beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers({ shouldAdvanceTime: true });
  useAuthStore.setState({ user: account });
  updateMe.mockImplementation(async (patch: Partial<User>) => ({ ...account, ...patch }));
});
afterEach(() => { cleanup(); vi.useRealTimers(); });

describe("AccountTab save recovery", () => {
  it("explains an empty required name and saves the retained description after correction", async () => {
    renderWithI18n(<AccountTab />);
    const name = screen.getByRole("textbox", { name: "Name" });
    const description = screen.getByRole("textbox", { name: "About you" });
    fireEvent.change(name, { target: { value: "   " } });
    fireEvent.change(description, { target: { value: "Keep this profile draft" } });
    fireEvent.blur(description);
    act(() => vi.advanceTimersByTime(1000));

    expect(updateMe).not.toHaveBeenCalled();
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(name).toHaveAccessibleDescription("Enter your name to save these changes.");
    expect(screen.getByRole("status")).toHaveTextContent("Unsaved changes");
    expect(description).toHaveValue("Keep this profile draft");

    fireEvent.change(name, { target: { value: "Alex Updated" } });
    fireEvent.blur(name);
    await waitFor(() => expect(updateMe).toHaveBeenCalledWith({ name: "Alex Updated", profile_description: "Keep this profile draft" }));
    expect(screen.getByRole("textbox", { name: "About you" })).toBe(description);
    await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent("Saved"));
    expect(name).toHaveAttribute("aria-invalid", "false");
    expect(useAuthStore.getState().user?.profile_description).toBe("Keep this profile draft");
  });

  it("retains the crop workflow and reports no success when avatar persistence fails", async () => {
    updateMe.mockRejectedValueOnce(new Error("Profile avatar failed"));
    const { container } = renderWithI18n(<AccountTab />);
    const input = container.querySelector<HTMLInputElement>('input[type="file"]');
    if (!input) throw new Error("Avatar input missing");
    fireEvent.change(input, { target: { files: [new File(["image"], "avatar.png", { type: "image/png" })] } });
    fireEvent.click(screen.getByRole("button", { name: "Save cropped avatar" }));

    await waitFor(() => expect(notices.error).toHaveBeenCalledWith("Profile avatar failed"));
    expect(notices.error).toHaveBeenCalledTimes(1);
    expect(notices.success).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Save cropped avatar" })).toBeEnabled();
    expect(useAuthStore.getState().user?.avatar_url).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Save cropped avatar" }));
    await waitFor(() => expect(screen.queryByRole("button", { name: "Save cropped avatar" })).not.toBeInTheDocument());
    expect(useAuthStore.getState().user?.avatar_url).toBe("/avatar.png");
  });
});
