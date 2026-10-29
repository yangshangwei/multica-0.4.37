import { useEffect, useRef, useState } from "react";

type FocusTarget = () => HTMLElement | null | undefined;

function focusLost() {
  const active = document.activeElement;
  return !active || active === document.body || !active.isConnected || active.closest("[hidden]") !== null;
}

/**
 * Moves focus to the first available target when the control that held it
 * was removed or hidden. Runs after the pending render so the new DOM exists;
 * focus that is still on a live control is left alone.
 */
export function recoverFocus(...targets: FocusTarget[]) {
  setTimeout(() => requestAnimationFrame(() => {
    if (!focusLost()) return;
    for (const target of targets) {
      const element = target();
      if (!element?.isConnected) continue;
      element.focus({ preventScroll: true });
      if (document.activeElement === element) return;
    }
  }));
}

/**
 * For inline forms opened by a button: focus the form when it mounts and
 * return focus to the opener when it closes, unless focus already moved on.
 */
export function useOpenedForm<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [opener] = useState(() => typeof document === "undefined" ? null : document.activeElement as HTMLElement | null);
  useEffect(() => {
    ref.current?.focus();
    return () => recoverFocus(() => opener);
  }, [opener]);
  return ref;
}
