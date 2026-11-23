import { createElement, createRef, useLayoutEffect, useRef } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { act, render, renderHook, screen } from "@testing-library/react";
import { useChatInputFocus } from "./use-chat-input-focus";

const detachedElements: HTMLElement[] = [];
afterEach(() => { for (const element of detachedElements.splice(0)) element.remove(); });

function focusElements() {
  const origin = document.createElement("button");
  const other = document.createElement("button");
  const launcher = document.createElement("button");
  launcher.dataset.chatLauncher = "";
  const window = document.createElement("div");
  window.tabIndex = -1;
  const composer = document.createElement("textarea");
  window.append(composer);
  document.body.append(origin, other, launcher, window);
  detachedElements.push(origin, other, launcher, window);
  return { origin, other, launcher, composer, windowRef: { current: window } };
}

function FocusBoundary({ open }: { open: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  const { onFocusCapture } = useChatInputFocus(open, ref, open);
  return createElement("div", { ref, tabIndex: -1, onFocusCapture },
    createElement("textarea", { "aria-label": "Boundary composer" }),
  );
}

function UnmountBlurButton() {
  const ref = useRef<HTMLButtonElement>(null);
  // Chromium blurs a focused button while React removes its header, before
  // the retained window receives inert. jsdom needs that native step supplied.
  useLayoutEffect(() => () => { ref.current?.blur(); }, []);
  return createElement("button", { ref, type: "button" }, "Minimize");
}

function HeaderFocusBoundary({ open }: { open: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  const { onFocusCapture } = useChatInputFocus(open, ref, open);
  return createElement("div", { ref, tabIndex: -1, onFocusCapture, inert: !open },
    open ? createElement(UnmountBlurButton) : null,
  );
}

describe("useChatInputFocus", () => {
  it("stays inert on mount, whether the window starts closed or open", () => {
    const windowRef = createRef<HTMLElement>();
    expect(renderHook(() => useChatInputFocus(false, windowRef, false)).result.current.focusRequest).toBe(0);
    // A persisted "open" preference must not steal focus from the page the user
    // just loaded — ChatWindow is mounted (hidden) even while closed.
    expect(renderHook(() => useChatInputFocus(true, windowRef, true)).result.current.focusRequest).toBe(0);
  });

  it("requests focus on every closed → open transition", () => {
    const windowRef = createRef<HTMLElement>();
    const { result, rerender } = renderHook(
      ({ isOpen }: { isOpen: boolean }) => useChatInputFocus(isOpen, windowRef, isOpen),
      { initialProps: { isOpen: false } },
    );

    rerender({ isOpen: true });
    expect(result.current.focusRequest).toBe(1);

    // Re-renders that don't change `isOpen` must not re-focus: the user may
    // have clicked into the message list or the agent picker since.
    rerender({ isOpen: true });
    expect(result.current.focusRequest).toBe(1);

    rerender({ isOpen: false });
    expect(result.current.focusRequest).toBe(1);

    rerender({ isOpen: true });
    expect(result.current.focusRequest).toBe(2);
  });

  it("lets callers bump the nonce for new chats and agent switches", () => {
    const windowRef = createRef<HTMLElement>();
    const { result } = renderHook(() => useChatInputFocus(true, windowRef, true));

    act(() => result.current.requestInputFocus());
    expect(result.current.focusRequest).toBe(1);
    act(() => result.current.requestInputFocus());
    expect(result.current.focusRequest).toBe(2);
  });

  it("waits for a requested window to become visible before focusing the composer", () => {
    const { origin, windowRef } = focusElements();
    origin.focus();
    const { result, rerender } = renderHook(
      ({ isOpen, visible }) => useChatInputFocus(isOpen, windowRef, visible),
      { initialProps: { isOpen: false, visible: false } },
    );
    rerender({ isOpen: true, visible: false });
    expect(result.current.focusRequest).toBe(0);
    expect(origin).toHaveFocus();
    rerender({ isOpen: true, visible: true });
    expect(result.current.focusRequest).toBe(1);
  });

  it("does not turn a persisted open window's initial measurement into a focus request", () => {
    const { origin, windowRef } = focusElements();
    origin.focus();
    const { result, rerender } = renderHook(
      ({ visible }) => useChatInputFocus(true, windowRef, visible),
      { initialProps: { visible: false } },
    );
    rerender({ visible: true });
    expect(result.current.focusRequest).toBe(0);
    expect(origin).toHaveFocus();
  });

  it("restores the shortcut's original focus when closing from chat", () => {
    const { origin, composer, windowRef } = focusElements();
    const { rerender } = renderHook(({ open }) => useChatInputFocus(open, windowRef, open), { initialProps: { open: false } });
    origin.focus();
    rerender({ open: true });
    composer.focus();
    rerender({ open: false });
    expect(origin).toHaveFocus();
  });

  it("leaves unrelated focus alone when the window closes", () => {
    const { origin, other, windowRef } = focusElements();
    const { rerender } = renderHook(({ open }) => useChatInputFocus(open, windowRef, open), { initialProps: { open: false } });
    origin.focus();
    rerender({ open: true });
    other.focus();
    rerender({ open: false });
    expect(other).toHaveFocus();
  });

  it("does not claim body focus after the user has left chat", () => {
    const { origin, other } = focusElements();
    const view = render(createElement(FocusBoundary, { open: false }));
    origin.focus();
    view.rerender(createElement(FocusBoundary, { open: true }));
    screen.getByRole("textbox", { name: "Boundary composer" }).focus();
    other.focus();
    other.blur();
    expect(document.body).toHaveFocus();
    view.rerender(createElement(FocusBoundary, { open: false }));
    expect(document.body).toHaveFocus();
  });

  it("leaves body focus alone when the visible composer has already blurred", () => {
    const { origin } = focusElements();
    const view = render(createElement(FocusBoundary, { open: false }));
    origin.focus();
    view.rerender(createElement(FocusBoundary, { open: true }));
    const composer = screen.getByRole("textbox", { name: "Boundary composer" });
    composer.focus();
    composer.blur();
    expect(document.body).toHaveFocus();
    view.rerender(createElement(FocusBoundary, { open: false }));
    expect(document.body).toHaveFocus();
  });

  it("restores focus when applying inert has already blurred the composer", () => {
    const { origin } = focusElements();
    const view = render(createElement(FocusBoundary, { open: false }));
    origin.focus();
    view.rerender(createElement(FocusBoundary, { open: true }));
    const composer = screen.getByRole("textbox", { name: "Boundary composer" });
    composer.focus();
    composer.parentElement?.setAttribute("inert", "");
    composer.blur();
    view.rerender(createElement(FocusBoundary, { open: false }));
    expect(origin).toHaveFocus();
  });

  it("restores the launcher when the header blurs during removal before the root becomes inert", () => {
    const { origin, launcher } = focusElements();
    const view = render(createElement(HeaderFocusBoundary, { open: false }));
    origin.focus();
    view.rerender(createElement(HeaderFocusBoundary, { open: true }));
    origin.remove();
    const minimize = screen.getByRole("button", { name: "Minimize" });
    const blurEvents: Array<{ connected: boolean; inert: boolean }> = [];
    minimize.addEventListener("focusout", () => {
      blurEvents.push({ connected: minimize.isConnected, inert: minimize.parentElement?.hasAttribute("inert") ?? false });
    });
    minimize.focus();
    view.rerender(createElement(HeaderFocusBoundary, { open: false }));
    expect(blurEvents).toEqual([{ connected: true, inert: false }]);
    expect(minimize).not.toBeInTheDocument();
    expect(launcher).toHaveFocus();
  });

  it("does not reclaim a header blur from an earlier interaction when closing later", async () => {
    const { origin } = focusElements();
    const view = render(createElement(HeaderFocusBoundary, { open: false }));
    origin.focus();
    view.rerender(createElement(HeaderFocusBoundary, { open: true }));
    const minimize = screen.getByRole("button", { name: "Minimize" });
    minimize.focus();
    await act(async () => { minimize.blur(); });
    expect(minimize).toBeInTheDocument();
    expect(document.body).toHaveFocus();
    view.rerender(createElement(HeaderFocusBoundary, { open: false }));
    expect(document.body).toHaveFocus();
  });

  it("falls back to the remounted launcher when the opener no longer exists", () => {
    const { origin, composer, launcher, windowRef } = focusElements();
    const { rerender } = renderHook(({ open }) => useChatInputFocus(open, windowRef, open), { initialProps: { open: false } });
    origin.focus();
    rerender({ open: true });
    origin.remove();
    composer.focus();
    rerender({ open: false });
    expect(launcher).toHaveFocus();
  });
});
