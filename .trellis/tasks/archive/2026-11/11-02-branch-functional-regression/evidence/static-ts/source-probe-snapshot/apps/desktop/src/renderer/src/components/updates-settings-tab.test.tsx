import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";

const mocks = vi.hoisted(() => ({
  getPreferences: vi.fn(),
  setAutomaticUpdates: vi.fn(),
  checkForUpdates: vi.fn(),
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}));

const translations = {
  auto_save: { toast_saved: "Settings saved" },
  desktop: {
    updates: {
      title: "Updates",
      description: "Update preferences",
      current_version: "Current version",
      automatic_updates_title: "Automatic background updates",
      automatic_updates_description: "Download updates in the background",
      automatic_updates_save_failed: "Failed to save update settings",
      check_section_title: "Check for updates",
      check_section_description: "Check manually",
      up_to_date: "Up to date",
      downloading: "Downloading v{{version}}",
      check_now: "Check now",
      checking: "Checking",
      check_failed: "Could not check for updates. Try again.",
    },
  },
};

vi.mock("@multica/views/i18n", () => ({
  useT: () => ({
    t: (
      selector: (resources: typeof translations) => string,
      values?: Record<string, string>,
    ) => {
      const template = selector(translations);
      return Object.entries(values ?? {}).reduce(
        (result, [key, value]) => result.replace(`{{${key}}}`, value),
        template,
      );
    },
  }),
}));

vi.mock("sonner", () => ({
  toast: {
    success: mocks.toastSuccess,
    error: mocks.toastError,
  },
}));

import { UpdatesSettingsTab } from "./updates-settings-tab";

describe("UpdatesSettingsTab", () => {
  beforeEach(() => {
    mocks.getPreferences.mockReset().mockResolvedValue({
      automaticUpdates: true,
    });
    mocks.setAutomaticUpdates.mockReset();
    mocks.checkForUpdates.mockReset();
    mocks.toastSuccess.mockReset();
    mocks.toastError.mockReset();

    Object.defineProperty(window, "desktopAPI", {
      configurable: true,
      value: { appInfo: { version: "1.2.3" } },
    });
    Object.defineProperty(window, "updater", {
      configurable: true,
      value: {
        getPreferences: mocks.getPreferences,
        setAutomaticUpdates: mocks.setAutomaticUpdates,
        checkForUpdates: mocks.checkForUpdates,
      },
    });
  });

  it("loads the persisted preference and saves changes from the switch", async () => {
    mocks.getPreferences.mockResolvedValue({ automaticUpdates: false });
    mocks.setAutomaticUpdates.mockResolvedValue({ automaticUpdates: true });
    render(<UpdatesSettingsTab />);

    const toggle = screen.getByRole("switch", {
      name: "Automatic background updates",
    });
    // The switch renders as <span role="switch">, so jest-dom's toBeEnabled()
    // treats it as always enabled and does not actually wait for getPreferences
    // to resolve. Wait on the persisted value being reflected instead, which
    // deterministically holds until the loaded preference (false) is applied.
    await waitFor(() => expect(toggle).not.toBeChecked());

    fireEvent.click(toggle);

    await waitFor(() => {
      expect(mocks.setAutomaticUpdates).toHaveBeenCalledWith(true);
      expect(toggle).toBeChecked();
    });
    expect(mocks.toastSuccess).toHaveBeenCalledWith("Settings saved", {
      id: "settings-auto-save",
    });
  });

  it.each([
    [{ ok: true, available: false }, "Up to date"],
    [{ ok: true, available: true, latestVersion: "1.2.4" }, "Downloading v1.2.4"],
    [{ ok: false, error: "Update service unavailable" }, "Update service unavailable"],
  ])("announces a manual update result in the same live region without moving focus", async (result, message) => {
    let finish!: (value: unknown) => void;
    mocks.checkForUpdates.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    render(<UpdatesSettingsTab />);
    const status = screen.getByRole("status");
    const button = screen.getByRole("button", { name: "Check now" });
    button.focus();
    fireEvent.click(button);
    // Native disabled buttons lose focus in Chromium; jsdom does not model that blur.
    expect(button).not.toBeDisabled();
    expect(button).toHaveAttribute("aria-disabled", "true");
    await act(async () => finish(result));
    expect(screen.getByRole("status")).toBe(status);
    expect(status).toHaveTextContent(String(message));
    expect(button).toHaveFocus();
  });

  it("rejects repeated activation synchronously while keeping the busy button focusable", async () => {
    let finish!: (value: unknown) => void;
    mocks.checkForUpdates.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    render(<UpdatesSettingsTab />);
    const button = screen.getByRole("button", { name: "Check now" });
    button.focus();

    act(() => {
      fireEvent.click(button);
      fireEvent.click(button);
    });

    expect(mocks.checkForUpdates).toHaveBeenCalledOnce();
    expect(button).not.toBeDisabled();
    expect(button).toHaveAttribute("aria-disabled", "true");
    expect(button.tabIndex).toBe(0);
    expect(button).toHaveFocus();
    fireEvent.click(button);
    expect(mocks.checkForUpdates).toHaveBeenCalledOnce();

    await act(async () => finish({ ok: true, available: false }));
    expect(button).toHaveAttribute("aria-disabled", "false");
    expect(button).toHaveFocus();
  });

  it("does not reclaim focus after the user moves to another control during a check", async () => {
    let finish!: (value: unknown) => void;
    mocks.checkForUpdates.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
    render(<><UpdatesSettingsTab /><button>Another action</button></>);
    const button = screen.getByRole("button", { name: "Check now" });
    button.focus();
    fireEvent.click(button);
    const nextAction = screen.getByRole("button", { name: "Another action" });
    nextAction.focus();

    await act(async () => finish({ ok: true, available: false }));

    expect(screen.getByRole("status")).toHaveTextContent("Up to date");
    expect(nextAction).toHaveFocus();
  });

  it("announces an IPC failure and enables another manual check", async () => {
    mocks.checkForUpdates.mockRejectedValue(new Error("IPC unavailable"));
    render(<UpdatesSettingsTab />);
    const button = screen.getByRole("button", { name: "Check now" });
    fireEvent.click(button);
    await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent("Could not check for updates. Try again."));
    expect(button).toHaveAttribute("aria-disabled", "false");

    mocks.checkForUpdates.mockResolvedValue({ ok: true, available: false });
    fireEvent.click(button);
    await waitFor(() => expect(screen.getByRole("status")).toHaveTextContent("Up to date"));
    expect(mocks.checkForUpdates).toHaveBeenCalledTimes(2);
  });
});
