import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type { AuthRecoveryState } from "@multica/core/auth";
import { RESOURCES } from "@multica/views/locales";
import { DesktopConnectionRecoveryBoundary } from "./connection-recovery-boundary";

const recovery: AuthRecoveryState = {
  isRetrying: false,
  nextRetryAt: null,
  failure: { kind: "network", occurredAt: 1_792_000_000_000 },
};
let openEditor: () => void;
const unsubscribe = vi.fn();
const handoff = vi.fn();
const save = vi.fn();

function boundary(state: AuthRecoveryState | null, child = "Recovered application", onEditorOpenChange?: (open: boolean) => void) {
  return (
    <I18nProvider locale="en" resources={RESOURCES}>
      <DesktopConnectionRecoveryBoundary recovery={state} onRetry={vi.fn()} onEditorOpenChange={onEditorOpenChange}>
        <p>{child}</p>
      </DesktopConnectionRecoveryBoundary>
    </I18nProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  handoff.mockResolvedValue({ ok: true });
  save.mockResolvedValue({ ok: false, message: "Cannot connect" });
  Object.defineProperty(window, "desktopAPI", {
    configurable: true,
    value: {
      systemLocale: "en-US",
      appInfo: { version: "0.4.37" },
      windowContext: { kind: "main" },
      runtimeConfig: { ok: true, config: { apiUrl: "https://old.example.com" } },
      onRuntimeConfigEditorOpen: (callback: () => void) => {
        openEditor = callback;
        return unsubscribe;
      },
      openRuntimeConfigEditor: handoff,
      testRuntimeConfig: vi.fn().mockResolvedValue({ ok: false, message: "Cannot connect" }),
      saveRuntimeConfig: save,
    },
  });
});
afterEach(cleanup);

describe("DesktopConnectionRecoveryBoundary", () => {
  it.each(["button", "IPC"])("reports editor opening synchronously for %s and closes only on Back", (entry) => {
    const events: { open: boolean; editorMounted: boolean }[] = [];
    const onEditorOpenChange = vi.fn((open: boolean) => events.push({ open, editorMounted: screen.queryByLabelText("Backend address") !== null }));
    render(boundary(recovery, "Recovered application", onEditorOpenChange));
    if (entry === "button") fireEvent.click(screen.getByRole("button", { name: "Change server" }));
    else act(() => openEditor());
    expect(events[0]).toEqual({ open: true, editorMounted: false });
    expect(onEditorOpenChange).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(onEditorOpenChange).toHaveBeenLastCalledWith(false);
  });

  it("keeps unsaved endpoint input mounted when background recovery succeeds", () => {
    const { rerender, unmount } = render(boundary(recovery));
    fireEvent.click(screen.getByRole("button", { name: "Change server" }));
    const address = screen.getByLabelText("Backend address");
    expect(address).toHaveValue("https://old.example.com");
    fireEvent.change(address, { target: { value: "https://unsaved.example.com" } });

    rerender(boundary(null, "Current application"));
    act(() => openEditor());
    expect(screen.getByLabelText("Backend address")).toBe(address);
    expect(address).toHaveValue("https://unsaved.example.com");
    expect(screen.getByRole("button", { name: "Save and connect" })).toBeEnabled();
    expect(screen.getByText(/sign-in will be verified/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByText("Current application")).toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();
    expect(window.desktopAPI.runtimeConfig).toEqual({
      ok: true, config: { apiUrl: "https://old.example.com" },
    });
    unmount();
    expect(unsubscribe).toHaveBeenCalledOnce();
  });

  it("opens the editor from a main-process request even after recovery has ended", () => {
    render(boundary(null));
    act(() => openEditor());
    expect(screen.getByLabelText("Backend address")).toHaveValue("https://old.example.com");
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByText("Recovered application")).toBeInTheDocument();
  });

  it("hands child-window editing to main instead of mounting a local editor", async () => {
    window.desktopAPI.windowContext = {
      kind: "issue", scope: "workspace", workspaceSlug: "team", issueId: "issue-1", path: "/team/issues/issue-1", title: "Issue 1",
    };
    render(boundary(recovery));
    fireEvent.click(screen.getByRole("button", { name: "Change server" }));
    expect(handoff).toHaveBeenCalledOnce();
    expect(await screen.findByText("Server settings opened in the main window.")).toBeInTheDocument();
    expect(screen.queryByLabelText("Backend address")).not.toBeInTheDocument();
    expect(save).not.toHaveBeenCalled();
  });

  it("leaves child recovery actionable when the main-window handoff fails", async () => {
    window.desktopAPI.windowContext = {
      kind: "issue", scope: "workspace", workspaceSlug: "team", issueId: "issue-1", path: "/team/issues/issue-1", title: "Issue 1",
    };
    handoff.mockResolvedValue({ ok: false });
    render(boundary(recovery));
    fireEvent.click(screen.getByRole("button", { name: "Change server" }));
    expect(await screen.findByText("Could not open server settings. Please try again.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Change server" })).toBeEnabled();
  });
});
