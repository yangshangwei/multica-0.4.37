import { useEffect, useRef, useState } from "react";

type FocusTarget = () => HTMLElement | null | undefined;

function focusLost(target: HTMLElement) {
  const active = document.activeElement;
  return !active || active === document.body || !active.isConnected || active.closest("[hidden]") !== null ||
    // Base UI restores removed-control focus to its popup. Only recover within
    // that same modal; an unrelated dialog or live control retains its focus.
    active.matches('[data-slot="dialog-content"][role="dialog"]') && target.closest('[data-slot="dialog-content"]') === active;
}

/**
 * Moves focus to the first available target when the control that held it
 * was removed or hidden. Runs after the pending render so the new DOM exists;
 * focus that is still on a live control is left alone.
 */
export function recoverFocus(...targets: FocusTarget[]) {
  setTimeout(() => requestAnimationFrame(() => {
    for (const target of targets) {
      const element = target();
      if (!element?.isConnected || !focusLost(element)) continue;
      element.focus({ preventScroll: true });
      if (document.activeElement === element) return;
    }
  }));
}

/**
 * For inline forms opened by a button: focus the form when it mounts and
 * return focus to the opener when it closes, unless focus already moved on.
 * A new opening identity deliberately reveals an already-mounted form again.
 */
export function useOpenedForm<T extends HTMLElement>({ opening, reveal = false }: { opening?: unknown; reveal?: boolean } = {}) {
  const ref = useRef<T>(null);
  const [initialOpener] = useState(() => typeof document === "undefined" ? null : document.activeElement as HTMLElement | null);
  const opened = useRef({ identity: opening, opener: initialOpener });
  useEffect(() => {
    if (opened.current.identity !== opening) {
      opened.current = { identity: opening, opener: document.activeElement instanceof HTMLElement ? document.activeElement : null };
    }
    const opener = opened.current.opener;
    const target = ref.current;
    if (reveal && target) {
      for (let ancestor = target.parentElement; ancestor; ancestor = ancestor.parentElement) {
        if (ancestor instanceof HTMLDetailsElement) ancestor.open = true;
      }
      target.scrollIntoView({ block: "nearest" });
    }
    target?.focus({ preventScroll: reveal });
    return () => recoverFocus(() => {
      if (opened.current.identity !== opening) return null;
      if (reveal && opener?.isConnected) opener.scrollIntoView({ block: "nearest" });
      return opener;
    });
  }, [opening, reveal]);
  return ref;
}
