"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
  type RefObject,
} from "react";
import { animate, motion, useMotionValue } from "motion/react";
import type { Layout, PanelSize } from "react-resizable-panels";
import {
  isEditableShortcutTarget,
  shortcutMatchesEvent,
  useShortcut,
} from "@multica/core/shortcuts";
import { isImeComposing } from "@multica/core/utils";
import { cn } from "@multica/ui/lib/utils";
import { PopupVisibilityProvider } from "@multica/ui/components/ui/popup-visibility";

export const rightSidebarPanelMotionProps = {
  "data-right-sidebar-panel": "true",
  style: { overflowX: "hidden" },
} as const;

const RIGHT_SIDEBAR_PANEL_TRANSITION_MS = 220;
const RIGHT_SIDEBAR_PANEL_SETTLE_MS = RIGHT_SIDEBAR_PANEL_TRANSITION_MS + 80;

const rightSidebarTransition = {
  type: "spring",
  stiffness: 420,
  damping: 38,
  mass: 0.8,
} as const;

const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";
function subscribeToReducedMotion(onChange: () => void) {
  const query = window.matchMedia(REDUCED_MOTION_QUERY);
  query.addEventListener("change", onChange);
  return () => query.removeEventListener("change", onChange);
}
function getReducedMotionPreference() {
  return window.matchMedia(REDUCED_MOTION_QUERY).matches;
}

export function getAnimatedRightSidebarInitialOpen(
  defaultOpen: boolean,
  defaultLayout: Layout | undefined,
  panelId = "sidebar",
) {
  const restoredSize = defaultLayout?.[panelId];
  return typeof restoredSize === "number" ? restoredSize > 0 : defaultOpen;
}

function isRightSidebarPanelOpen(size: PanelSize) {
  return size.asPercentage > 0 || size.inPixels > 0;
}

export function useAnimatedRightSidebarState(initialOpen: boolean) {
  const [open, setOpen] = useState(initialOpen);
  const [visualOpen, setVisualOpen] = useState(initialOpen);
  const [motionEnabled, setMotionEnabled] = useState(false);
  const toggleTargetRef = useRef<boolean | null>(null);
  const settleTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const beginToggle = useCallback((nextOpen: boolean) => {
    toggleTargetRef.current = nextOpen;
    setMotionEnabled(true);
    setOpen(nextOpen);
    setVisualOpen(nextOpen);

    if (settleTimeoutRef.current) {
      clearTimeout(settleTimeoutRef.current);
    }

    settleTimeoutRef.current = setTimeout(() => {
      toggleTargetRef.current = null;
      settleTimeoutRef.current = null;
      setMotionEnabled(false);
    }, RIGHT_SIDEBAR_PANEL_SETTLE_MS);
  }, []);

  const handleResize = useCallback((size: PanelSize) => {
    const nextOpen = isRightSidebarPanelOpen(size);
    const toggleTarget = toggleTargetRef.current;

    if (toggleTarget === null) {
      setOpen(nextOpen);
      setVisualOpen(nextOpen);
      return;
    }

    setOpen(toggleTarget);
  }, []);

  useEffect(() => {
    return () => {
      if (settleTimeoutRef.current) {
        clearTimeout(settleTimeoutRef.current);
      }
    };
  }, []);

  return { open, visualOpen, motionEnabled, beginToggle, handleResize };
}

/** Toggle the right sidebar only for the visible detail page that owns it. */
export function useRightSidebarShortcut(
  targetRef: RefObject<HTMLElement | null>,
  onToggle: () => void,
) {
  const shortcut = useShortcut("toggleRightSidebar");

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.repeat || isImeComposing(event)) return;
      if (isEditableShortcutTarget(event.target)) return;
      if (!shortcutMatchesEvent(shortcut, event)) return;

      const target = targetRef.current;
      if (!target || target.getClientRects().length === 0) return;

      event.preventDefault();
      onToggle();
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onToggle, shortcut, targetRef]);
}

export function AnimatedRightSidebar({
  open,
  motionEnabled,
  children,
  className,
  returnFocusRef,
}: {
  open: boolean;
  motionEnabled?: boolean;
  children: ReactNode;
  className?: string;
  returnFocusRef?: RefObject<HTMLElement | null>;
}) {
  const panelRef = useRef<HTMLDivElement | null>(null);
  const lastFocusedRef = useRef<HTMLElement | null>(null);
  const removedFocusRef = useRef<HTMLElement | null>(null);
  const wasOpenRef = useRef(open);
  const reducedMotion = useSyncExternalStore(subscribeToReducedMotion, getReducedMotionPreference, () => false);
  const opacity = useMotionValue(open ? 1 : 0);
  const x = useMotionValue(open ? 0 : 12);

  useLayoutEffect(() => {
    const nextOpacity = open ? 1 : 0;
    const nextX = open || reducedMotion ? 0 : 12;
    if (reducedMotion || !motionEnabled) {
      // Jump also stops a spring already running when the OS preference changes.
      opacity.jump(nextOpacity);
      x.jump(nextX);
      return;
    }
    const fade = animate(opacity, nextOpacity, rightSidebarTransition);
    const slide = animate(x, nextX, rightSidebarTransition);
    return () => { fade.stop(); slide.stop(); };
  }, [open, motionEnabled, reducedMotion, opacity, x]);

  useEffect(() => {
    if (!open) return;
    const onFocusIn = (event: FocusEvent) => {
      if (panelRef.current?.hasAttribute("inert")) return;
      if (event.target instanceof Node && event.target !== lastFocusedRef.current && !panelRef.current?.contains(event.target)) {
        lastFocusedRef.current = null;
        removedFocusRef.current = null;
      }
    };
    const onFocusOut = (event: FocusEvent) => {
      const previous = lastFocusedRef.current;
      if (!previous || event.target !== previous || event.relatedTarget !== null || panelRef.current?.hasAttribute("inert")) return;
      lastFocusedRef.current = null;
      removedFocusRef.current = previous;
      queueMicrotask(() => { if (removedFocusRef.current === previous) removedFocusRef.current = null; });
    };
    document.addEventListener("focusin", onFocusIn);
    document.addEventListener("focusout", onFocusOut);
    return () => {
      document.removeEventListener("focusin", onFocusIn);
      document.removeEventListener("focusout", onFocusOut);
    };
  }, [open]);

  useLayoutEffect(() => {
    if (wasOpenRef.current && !open) {
      const active = document.activeElement;
      const removedWhileClosing = removedFocusRef.current !== null && !removedFocusRef.current.isConnected;
      const ownedFocus = panelRef.current?.contains(active) || active === lastFocusedRef.current ||
        (active === document.body && (lastFocusedRef.current !== null || removedWhileClosing));
      const target = returnFocusRef?.current;
      if (ownedFocus && target?.isConnected && !target.closest("[inert], [hidden], [aria-hidden='true']") && !target.matches(":disabled")) {
        target.focus({ preventScroll: true });
      }
      lastFocusedRef.current = null;
      removedFocusRef.current = null;
    }
    wasOpenRef.current = open;
  }, [open, returnFocusRef]);

  return (
    <PopupVisibilityProvider visible={open}>
      <motion.div
        ref={panelRef}
        aria-hidden={!open || undefined}
        inert={!open}
        onFocusCapture={(event) => {
          if (open && event.target instanceof HTMLElement) {
            lastFocusedRef.current = event.target;
            removedFocusRef.current = null;
          }
        }}
        style={reducedMotion ? { opacity: open ? 1 : 0, transform: "none" } : { opacity, x }}
        className={cn(
          "h-full overflow-x-hidden overflow-y-auto border-l",
          !open && "pointer-events-none",
          className,
        )}
        initial={false}
      >
        <div className="p-4">{children}</div>
      </motion.div>
    </PopupVisibilityProvider>
  );
}
