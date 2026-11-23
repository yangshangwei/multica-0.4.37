import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import type { AuthRecoveryState } from "@multica/core/auth";
import type { SupportedLocale } from "@multica/core/i18n";
import { copyText } from "@multica/ui/lib/clipboard";
import { RESOURCES } from "@multica/views/locales";
import { DesktopAuthRecoveryPage } from "./auth-recovery";

vi.mock("@multica/ui/lib/clipboard", () => ({ copyText: vi.fn() }));
const retry = vi.fn();
const changeServer = vi.fn();
const failure = {
  kind: "server" as const,
  status: 503,
  occurredAt: 1_792_000_000_000,
  message: "fixture-secret-body",
  stack: "fixture-secret-stack",
};
const recovery: AuthRecoveryState = { isRetrying: false, nextRetryAt: null, failure };

function page(state = recovery, locale: SupportedLocale = "en") {
  return (
    <I18nProvider locale={locale} resources={RESOURCES}>
      <DesktopAuthRecoveryPage recovery={state} onRetry={retry} onChangeServer={changeServer} />
    </I18nProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(copyText).mockResolvedValue(true);
  Object.defineProperty(window, "desktopAPI", {
    configurable: true,
    value: {
      appInfo: { version: "0.4.37" },
      windowContext: { kind: "main" },
      runtimeConfig: {
        ok: true,
        config: { apiUrl: "https://user:fixture-password@server.example.com/api?token=fixture-secret#fixture-fragment" },
      },
    },
  });
});
afterEach(() => { cleanup(); vi.useRealTimers(); });

describe("DesktopAuthRecoveryPage", () => {
  it("shows a usable manual recovery without claiming a saved login or an unscheduled retry", () => {
    render(page());
    expect(screen.getByRole("heading", { name: "Cannot connect to Multica" })).toBeInTheDocument();
    expect(screen.getByText("https://server.example.com/api", { selector: "p" })).toBeVisible();
    expect(screen.queryByText(/session is still saved|retry automatically|retry in/i)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry now" }));
    expect(retry).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByRole("button", { name: "Change server" }));
    expect(changeServer).toHaveBeenCalledOnce();
  });

  it("uses the scheduled deadline and keeps Change server usable during retry", () => {
    vi.useFakeTimers();
    vi.setSystemTime(1_792_000_000_000);
    const state = { ...recovery, nextRetryAt: Date.now() + 5_000 };
    const { rerender } = render(page(state));
    expect(screen.getByText("Retrying in 5 seconds")).toBeInTheDocument();
    act(() => { vi.advanceTimersByTime(2_000); });
    expect(screen.getByText("Retrying in 3 seconds")).toBeInTheDocument();

    rerender(page({ ...state, isRetrying: true, nextRetryAt: null }));
    expect(screen.getByRole("button", { name: "Retrying..." })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Change server" })).toBeEnabled();
    expect(screen.queryByText(/retrying in/i)).not.toBeInTheDocument();
  });

  it("copies only allowlisted connection facts and reports clipboard success and failure", async () => {
    render(page());
    fireEvent.click(screen.getByText("Connection details"));
    fireEvent.click(screen.getByRole("button", { name: "Copy details" }));
    expect(await screen.findByText("Copied")).toBeInTheDocument();
    const copied = vi.mocked(copyText).mock.calls[0][0];
    expect(copied).toContain("Server: https://server.example.com/api");
    expect(copied).toContain("Client version: 0.4.37");
    expect(copied).toContain("HTTP status: 503");
    expect(copied).not.toMatch(/fixture-|token=|stack|password|user:/i);
    expect(document.body.textContent).not.toMatch(/fixture-|token=|stack|password/i);

    vi.mocked(copyText).mockResolvedValue(false);
    fireEvent.click(screen.getByRole("button", { name: "Copy details" }));
    expect(await screen.findByText("Could not copy. Select the details and copy them manually.")).toBeInTheDocument();
    expect(screen.getByText("HTTP status")).toBeVisible();
  });

  it("localizes the actions and factual failure description in Chinese", () => {
    render(page({ ...recovery, failure: { kind: "timeout", occurredAt: failure.occurredAt } }, "zh-Hans"));
    expect(screen.getByRole("button", { name: "立即重试" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "更换服务器" })).toBeInTheDocument();
    expect(screen.getByText("服务器未在规定时间内响应。", { selector: "p" })).toBeVisible();
    expect(screen.getByText("连接详情")).toBeInTheDocument();
  });
});
