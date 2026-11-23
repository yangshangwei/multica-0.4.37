import { StrictMode } from "react";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { pickLocale } from "@multica/core/i18n";
import { I18nProvider, LocaleAdapterProvider, UserLocaleSync } from "@multica/core/i18n/react";
import { RESOURCES } from "@multica/views/locales";
import { DesktopConnectionRecoveryBoundary } from "../components/connection-recovery-boundary";
import { createDesktopLocaleAdapter, createDesktopLocaleReload } from "./i18n-adapter";

const auth = vi.hoisted(() => ({ user: null as { language: string } | null }));
vi.mock("@multica/core/auth", async (importOriginal) => ({
  ...await importOriginal<typeof import("@multica/core/auth")>(),
  useAuthStore: Object.assign(
    (selector: (state: typeof auth) => unknown) => selector(auth),
    { getState: () => auth },
  ),
}));

let openEditor: () => void;
beforeEach(() => {
  auth.user = null;
  localStorage.clear();
  Object.defineProperty(window, "desktopAPI", {
    configurable: true,
    value: {
      systemLocale: "en-US",
      windowContext: { kind: "main" },
      runtimeConfig: { ok: true, config: { apiUrl: "https://old.example.com" } },
      onRuntimeConfigEditorOpen: (callback: () => void) => {
        openEditor = callback;
        return vi.fn();
      },
    },
  });
});
afterEach(cleanup);

describe("desktop locale reload", () => {
  it("coalesces language refreshes during editing and flushes once when Back reports closing", () => {
    const reload = vi.fn();
    const gate = createDesktopLocaleReload(reload);
    gate.onEditorOpenChange(true);
    gate.reload();
    gate.reload();
    gate.onEditorOpenChange(true);
    expect(reload).not.toHaveBeenCalled();
    gate.onEditorOpenChange(false);
    expect(reload).toHaveBeenCalledOnce();
    gate.onEditorOpenChange(false);
    expect(reload).toHaveBeenCalledOnce();
    gate.reload();
    expect(reload).toHaveBeenCalledTimes(2);
  });

  it("persists a recovered user's language without dropping the real editor's input", () => {
    const reload = vi.fn();
    const gate = createDesktopLocaleReload(reload);
    const adapter = createDesktopLocaleAdapter("en-US", gate.reload);
    const tree = () => <StrictMode>
      <I18nProvider locale="en" resources={RESOURCES}>
        <LocaleAdapterProvider adapter={adapter}>
          <UserLocaleSync />
          <DesktopConnectionRecoveryBoundary recovery={null} onRetry={vi.fn()} onEditorOpenChange={gate.onEditorOpenChange}>
            <p>Recovered application</p>
          </DesktopConnectionRecoveryBoundary>
        </LocaleAdapterProvider>
      </I18nProvider>
    </StrictMode>;
    const { rerender } = render(tree());
    act(() => openEditor());
    const address = screen.getByLabelText("Backend address");
    fireEvent.change(address, { target: { value: "https://unsaved.example.com" } });

    auth.user = { language: "zh-Hans" };
    rerender(tree());
    expect(adapter.getUserChoice()).toBe("zh-Hans");
    expect(reload).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Backend address")).toBe(address);
    expect(address).toHaveValue("https://unsaved.example.com");
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(reload).toHaveBeenCalledOnce();
    expect(pickLocale(adapter)).toBe("zh-Hans");
    expect(screen.getByText("Recovered application")).toBeInTheDocument();
  });

  it("keeps locale reload immediate when no server editor is open", () => {
    auth.user = { language: "zh-Hans" };
    const reload = vi.fn();
    const gate = createDesktopLocaleReload(reload);
    const adapter = createDesktopLocaleAdapter("en-US", gate.reload);
    render(<I18nProvider locale="en" resources={RESOURCES}>
      <LocaleAdapterProvider adapter={adapter}><UserLocaleSync /></LocaleAdapterProvider>
    </I18nProvider>);
    expect(adapter.getUserChoice()).toBe("zh-Hans");
    expect(reload).toHaveBeenCalledOnce();
  });

  it("does not flush a deferred refresh during StrictMode cleanup or editor disposal", () => {
    const reload = vi.fn();
    const gate = createDesktopLocaleReload(reload);
    const { unmount } = render(<StrictMode><I18nProvider locale="en" resources={RESOURCES}>
      <DesktopConnectionRecoveryBoundary recovery={null} onRetry={vi.fn()} onEditorOpenChange={gate.onEditorOpenChange}>
        <p>Recovered application</p>
      </DesktopConnectionRecoveryBoundary>
    </I18nProvider></StrictMode>);
    act(() => openEditor());
    gate.reload();
    unmount();
    expect(reload).not.toHaveBeenCalled();
  });
});
