"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState, type FocusEventHandler, type RefObject } from "react";

/**
 * Owns the floating window's composer-focus nonce.
 *
 * `focusRequest` is handed to ChatInput, which pulls keyboard focus into the
 * editor every time the number changes; `0` is inert. Callers bump it for the
 * moments that mean "you are about to type something new" — a fresh chat, an
 * agent switch, a project-context change.
 *
 * Opening the window is one of those moments (MUL-5522). Wait for its measured
 * bounds before focusing, because a not-yet-visible window is inert. Closing
 * returns focus to the opener only while focus still belongs to chat, including
 * its portalled controls. The FAB remounts on close and is the fallback when
 * the original opener has gone away.
 *
 * The ref is seeded with the mount-time value so only a real closed → open
 * transition focuses. ChatWindow stays mounted while closed and `isOpen` is
 * restored from storage, so treating mount as an open event would let a
 * persisted "open" preference steal focus from whatever page the user loaded.
 */
export function useChatInputFocus(
  isOpen: boolean,
  windowRef: RefObject<HTMLElement | null>,
  isVisible: boolean,
): {
  focusRequest: number;
  requestInputFocus: () => void;
  onFocusCapture: FocusEventHandler<HTMLElement>;
} {
  const [focusRequest, setFocusRequest] = useState(0);
  const requestInputFocus = useCallback(() => setFocusRequest((n) => n + 1), []);

  const wasOpenRef = useRef(isOpen);
  const openerRef = useRef<HTMLElement | null>(null);
  const lastFocusedRef = useRef<HTMLElement | null>(null);
  const blurredFocusRef = useRef<{ target: HTMLElement } | null>(null);
  const pendingFocusRef = useRef(false);
  const onFocusCapture = useCallback<FocusEventHandler<HTMLElement>>((event) => {
    if (isOpen && event.target instanceof HTMLElement) {
      lastFocusedRef.current = event.target;
      blurredFocusRef.current = null;
    }
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    const onFocusIn = (event: FocusEvent) => {
      if (windowRef.current?.hasAttribute("inert")) return;
      // React capture records portalled descendants before this document
      // listener runs. Once focus leaves chat, a later blur to body must not
      // revive that stale ownership when the window closes.
      if (event.target instanceof Node && event.target !== lastFocusedRef.current && !windowRef.current?.contains(event.target)) {
        lastFocusedRef.current = null;
        blurredFocusRef.current = null;
      }
    };
    const onFocusOut = (event: FocusEvent) => {
      const target = lastFocusedRef.current;
      if (!target || event.target !== target || event.relatedTarget !== null || windowRef.current?.hasAttribute("inert")) return;
      lastFocusedRef.current = null;
      // Removing the focused header can blur it before React applies inert
      // to the retained root. Keep this commit's candidate until mutations
      // settle; the closing layout effect only reclaims a detached target.
      const blurred = { target };
      blurredFocusRef.current = blurred;
      queueMicrotask(() => {
        if (blurredFocusRef.current === blurred) blurredFocusRef.current = null;
      });
    };
    document.addEventListener("focusin", onFocusIn);
    document.addEventListener("focusout", onFocusOut);
    return () => {
      document.removeEventListener("focusin", onFocusIn);
      document.removeEventListener("focusout", onFocusOut);
    };
  }, [isOpen, windowRef]);

  useLayoutEffect(() => {
    const wasOpen = wasOpenRef.current;
    wasOpenRef.current = isOpen;
    if (isOpen && !wasOpen) {
      const active = document.activeElement;
      openerRef.current = active instanceof HTMLElement && active !== document.body && !windowRef.current?.contains(active)
        ? active
        : null;
      pendingFocusRef.current = true;
    }
    if (!isOpen && wasOpen) {
      pendingFocusRef.current = false;
      const active = document.activeElement;
      // Applying inert may already have blurred the composer to body. React's
      // focus capture also records descendants rendered through a portal.
      const removedWhileClosing = blurredFocusRef.current !== null && !blurredFocusRef.current.target.isConnected;
      const ownsFocus = windowRef.current?.contains(active) || active === lastFocusedRef.current ||
        (active === document.body && (lastFocusedRef.current !== null || removedWhileClosing));
      if (ownsFocus) {
        const opener = openerRef.current;
        const target = opener?.isConnected && !opener.closest("[inert], [hidden], [aria-hidden='true']") && !opener.matches(":disabled")
          ? opener
          : document.querySelector<HTMLElement>("[data-chat-launcher]");
        target?.focus({ preventScroll: true });
      }
      openerRef.current = null;
      lastFocusedRef.current = null;
      blurredFocusRef.current = null;
    }
    if (isVisible && pendingFocusRef.current) {
      pendingFocusRef.current = false;
      // A disabled composer cannot take focus; the named window remains a
      // useful starting point for its other actions in that case.
      windowRef.current?.focus({ preventScroll: true });
      requestInputFocus();
    }
  }, [isOpen, isVisible, requestInputFocus, windowRef]);

  return { focusRequest, requestInputFocus, onFocusCapture };
}
