import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import i18next from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { RESOURCES } from "@multica/views/locales";

import { CloseBehaviorPrompt } from "./close-behavior-prompt";

/**
 * The prompt listens to window.closeBehaviorAPI. We install a hoisted mock
 * so each test can drive the IPC event emitter and capture respond() calls.
 */

const mocks = vi.hoisted(() => {
  type PromptHandler = (req: { requestId: string }) => void;
  const state: {
    handler: PromptHandler | null;
    responded: { requestId: string; action: string; remember: boolean }[];
    traySupported: boolean;
  } = {
    handler: null,
    responded: [],
    traySupported: true,
  };
  return {
    state,
    send: (requestId: string) => {
      state.handler?.({ requestId });
    },
    reset: () => {
      state.handler = null;
      state.responded = [];
      state.traySupported = true;
    },
    closeBehaviorAPI: {
      get: vi.fn(async () => "ask" as const),
      set: vi.fn(async () => ({ ok: true as const })),
      isTraySupported: vi.fn(async () => state.traySupported),
      onPrompt: vi.fn((handler: PromptHandler) => {
        state.handler = handler;
        return () => {
          if (state.handler === handler) state.handler = null;
        };
      }),
      respond: vi.fn(
        (requestId: string, result: { action: string; remember: boolean }) => {
          state.responded.push({ requestId, ...result });
        },
      ),
    },
  };
});

beforeEach(() => {
  mocks.reset();
  window.closeBehaviorAPI = mocks.closeBehaviorAPI;
});

afterEach(() => {
  // @ts-expect-error - deliberately removing a property from Window for test isolation
  delete window.closeBehaviorAPI;
  mocks.reset();
});

// Boot a real i18next against the published RESOURCES so t($ => $) selectors
// resolve. Use the selector-API enabled instance shape that matches
// packages/views runtime config.
async function makeI18n() {
  const i18n = i18next.createInstance();
  await i18n.use(initReactI18next).init({
    lng: "en",
    fallbackLng: "en",
    resources: { en: RESOURCES.en },
    ns: ["desktop", "common"],
    defaultNS: "common",
    interpolation: { escapeValue: false },
  });
  return i18n;
}

// Mount CloseBehaviorPrompt inside an act() boundary. The prompt itself has no
// render on mount; it only appears when main sends an IPC prompt event.
async function mount() {
  const i18n = await makeI18n();
  let container!: ReturnType<typeof render>;
  await act(async () => {
    container = render(
      <I18nextProvider i18n={i18n}>
        <CloseBehaviorPrompt />
      </I18nextProvider>,
    );
  });
  return container;
}

describe("CloseBehaviorPrompt", () => {
  it("does not render a dialog before main sends a prompt", async () => {
    await mount();
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });

  it("renders the dialog after a prompt arrives", async () => {
    await mount();
    await act(async () => {
      mocks.send("req-1");
    });
    expect(await screen.findByText("Close Multica?")).toBeTruthy();
  });

  it("Quit click responds with action=quit + remember=false by default", async () => {
    await mount();
    await act(async () => {
      mocks.send("req-2");
    });
    const quitButton = await screen.findByRole("button", { name: "Quit" });
    await act(async () => {
      quitButton.click();
    });
    expect(mocks.state.responded).toEqual([
      { requestId: "req-2", action: "quit", remember: false },
    ]);
  });

  it("Minimize click + remember=true persists the choice", async () => {
    await mount();
    await act(async () => {
      mocks.send("req-3");
    });
    const rememberCheckbox = await screen.findByRole("checkbox");
    await act(async () => {
      rememberCheckbox.click();
    });
    const minimize = await screen.findByRole("button", {
      name: "Minimize to tray",
    });
    await act(async () => {
      minimize.click();
    });
    expect(mocks.state.responded).toEqual([
      { requestId: "req-3", action: "minimize", remember: true },
    ]);
  });

  it("Cancel click responds with action=ask (re-ask next time)", async () => {
    await mount();
    await act(async () => {
      mocks.send("req-4");
    });
    const cancel = await screen.findByRole("button", { name: "Cancel" });
    await act(async () => {
      cancel.click();
    });
    expect(mocks.state.responded).toEqual([
      { requestId: "req-4", action: "ask", remember: false },
    ]);
  });

  it("hides the Minimize button when the tray is unsupported", async () => {
    mocks.state.traySupported = false;
    await mount();
    await act(async () => {
      mocks.send("req-5");
    });
    // Dialog opens
    expect(await screen.findByText("Close Multica?")).toBeTruthy();
    // Give the lazy isTraySupported() resolution a tick.
    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.queryByRole("button", { name: "Minimize to tray" })).toBeNull();
  });

  it("unsubscribes the prompt listener on unmount", async () => {
    const mounted = await mount();
    expect(mocks.state.handler).not.toBeNull();
    await act(async () => {
      mounted.unmount();
    });
    expect(mocks.state.handler).toBeNull();
  });
});
