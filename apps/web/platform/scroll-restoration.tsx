"use client";

import { useLayoutEffect, useMemo, type ReactNode } from "react";
import { usePathname, useSearchParams } from "next/navigation";
import { accountRouteKey } from "@multica/core/paths";
import {
  ScrollRestorationProvider,
  type ScrollRestorationAdapter,
} from "@multica/views/platform";
import { useWebNavigationOwner } from "./navigation-owner";
import { useWebScrollCaptureRegistration } from "./navigation";

/**
 * Web half of the MUL-4741 scroll-restoration protocol (desktop's half lives
 * in the tab coordinator). App-owned containers need route-specific offsets;
 * native history restoration can also move inner scrollers to old fragments.
 * The notification navigation owner temporarily disables that competing
 * browser behavior while this provider supplies the authoritative mementos.
 *
 * A capture-phase scroll listener continuously records marked containers.
 * Account navigation also snapshots the departing committed route, since
 * retained containers can carry an offset without firing another scroll event.
 * Both write the same `routeKey::containerKey` map. Serving uses desktop's
 * protocol — views read the offset at mount through `useRestoredScrollOffset` /
 * `useRestoredScrollRef`.
 *
 * Workspace pathname/accountRouteKey (not history entry) matches desktop's memento
 * semantics: returning to a route — via back or via a fresh link — restores
 * where you last were on it. The map is module state: per browser tab (web
 * tabs are real tabs), reset on full reload where the browser's own
 * restoration takes over.
 */
const savedOffsets = new Map<string, { top: number; height: number }>();
let currentOwner: string | null = null;

/**
 * Generic view-state entries (the desktop memento's second cargo — e.g.
 * "this route's comment-highlight deep link already landed"). Views write
 * through `setViewState` when the state changes and read back at mount.
 * Same lifetime as the offsets: module state, per browser tab, reset on
 * full reload.
 */
const savedViewState = new Map<string, string>();

/**
 * Keys recently served through `adapter.get` are write-suppressed briefly:
 * the restoring container assigns `scrollTop` at attach time, and if its
 * content height isn't final yet the browser clamps the assignment and fires
 * a scroll event — without suppression that clamped value (or a clamp to 0)
 * would overwrite the very memento being restored. A real user scroll within
 * the window is lost, which is the cheap side of the trade.
 */
const suppressedUntil = new Map<string, number>();
const RESTORE_SUPPRESS_MS = 1000;

function mementoKey(pathname: string, containerKey: string): string {
  return `${pathname}::${containerKey}`;
}

export function WebScrollRestorationProvider({
  children,
}: {
  children: ReactNode;
}) {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const owner = useWebNavigationOwner();
  const registerCapture = useWebScrollCaptureRegistration();
  const query = searchParams.toString();
  const accountKey = accountRouteKey(`${pathname}${query ? `?${query}` : ""}`);
  const routeKey = accountKey ?? pathname;
  // Isolate reads during the identity-changing render, before child ref/effect
  // restoration can run. Old adapters also check ownership before writes.
  if (currentOwner !== owner) {
    savedOffsets.clear();
    savedViewState.clear();
    suppressedUntil.clear();
    currentOwner = owner;
  }
  useLayoutEffect(() => {
    const recordScroll = (el: HTMLElement, settledDeparture = false) => {
      if (!owner || currentOwner !== owner) return;
      const raw = el.getAttribute("data-tab-scroll-root");
      if (raw === null) return;
      const key = mementoKey(routeKey, raw || "main");
      const suppressed = suppressedUntil.get(key);
      if (!settledDeparture && suppressed !== undefined) {
        if (performance.now() < suppressed) return;
        suppressedUntil.delete(key);
      }
      if (el.scrollTop <= 0) {
        // Back at the top is the default state — a stale offset would
        // otherwise resurrect on the next visit.
        savedOffsets.delete(key);
      } else {
        savedOffsets.set(key, { top: el.scrollTop, height: el.scrollHeight });
      }
    };
    const capture = (expectedPath?: string) => {
      if (!owner || currentOwner !== owner || accountKey === null) return;
      if (expectedPath !== undefined && accountRouteKey(expectedPath) !== accountKey) return;
      const physicalPath = window.location.pathname + window.location.search + window.location.hash;
      const settledDeparture = expectedPath !== undefined && accountRouteKey(physicalPath) === accountKey;
      document.querySelectorAll<HTMLElement>('[data-tab-scroll-root^="account-"]').forEach((el) => recordScroll(el, settledDeparture));
    };
    const unregisterCapture = accountKey === null ? undefined : registerCapture?.(capture);
    const onScroll = (e: Event) => {
      if (e.target instanceof HTMLElement) recordScroll(e.target);
    };
    // Scroll events don't bubble, but they do propagate on the capture
    // phase, so one window listener observes every container.
    window.addEventListener("scroll", onScroll, {
      capture: true,
      passive: true,
    });
    return () => {
      window.removeEventListener("scroll", onScroll, { capture: true });
      unregisterCapture?.();
    };
  }, [owner, routeKey, accountKey, registerCapture]);

  const adapter = useMemo<ScrollRestorationAdapter>(
    () => ({
      get(containerKey) {
        if (!owner || currentOwner !== owner) return undefined;
        const key = mementoKey(routeKey, containerKey);
        const saved = savedOffsets.get(key);
        if (saved) {
          // The caller is about to restore this offset — shield the memento
          // from the clamped scroll events the restore itself can produce.
          suppressedUntil.set(key, performance.now() + RESTORE_SUPPRESS_MS);
        }
        return saved;
      },
      getViewState(entryKey) {
        if (!owner || currentOwner !== owner) return undefined;
        return savedViewState.get(
          mementoKey(routeKey, entryKey),
        );
      },
      setViewState(entryKey, value, routePathname) {
        if (!owner || currentOwner !== owner) return;
        const key = mementoKey(routePathname ? accountRouteKey(routePathname) ?? routePathname : routeKey, entryKey);
        if (value === undefined) savedViewState.delete(key);
        else savedViewState.set(key, value);
      },
    }),
    // Render-time reads must follow Next's route, which can differ from
    // window.location while a navigation is being prepared or committed.
    [routeKey, owner],
  );

  return (
    <ScrollRestorationProvider adapter={adapter}>
      {children}
    </ScrollRestorationProvider>
  );
}
