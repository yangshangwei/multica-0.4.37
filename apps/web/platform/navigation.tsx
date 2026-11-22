"use client";

import { createContext, Suspense, useCallback, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState, useSyncExternalStore, type Dispatch, type SetStateAction } from "react";
import { flushSync } from "react-dom";
import { useRouter, usePathname, useSearchParams } from "next/navigation";
import { parseAccountPath } from "@multica/core/paths";
import {
  NavigationProvider,
  type NavigationAdapter,
} from "@multica/views/navigation";
import { canGoBackInApp } from "./in-app-history";

const ScopeResumeContext = createContext<(path: string) => void>(() => undefined);
type NavigationIntent = { path: string; resume: boolean; history: boolean; key: number };
type ScrollCapture = (expectedPath?: string) => void;
const ScrollCaptureRegistrationContext = createContext<((capture: ScrollCapture) => () => void) | null>(null);

/** The scroll owner registers its current committed account snapshot. */
export function useWebScrollCaptureRegistration() {
  return useContext(ScrollCaptureRegistrationContext);
}

/** Global scope controls resume; ordinary navigation remains an explicit open. */
export function useWebScopeResume() {
  return useContext(ScopeResumeContext);
}

/**
 * Web half of the `multica:navigate` bridge — the event shared content
 * (comments, chat, issue descriptions) fires when a link resolves to an in-app
 * destination. A plain click ("push") is a router push in place. A modifier
 * click normally never reaches here on web — real anchors leave it to the
 * browser — but the editor must intercept every click (contenteditable
 * anchors don't navigate natively), and for those `window.open` is the
 * closest the web can get: JS cannot open a background tab, so both tab
 * dispositions land as a foreground browser tab.
 */
function useInternalLinkHandler(push: (path: string) => void) {
  useEffect(() => {
    const handler = (e: Event) => {
      const detail = (
        e as CustomEvent<{ path?: string; disposition?: string }>
      ).detail;
      const path = detail?.path;
      if (!path) return;
      if (
        detail?.disposition === "background-tab" ||
        detail?.disposition === "foreground-tab"
      ) {
        window.open(
          window.location.origin + path,
          "_blank",
          "noopener,noreferrer",
        );
        return;
      }
      push(path);
    };
    window.addEventListener("multica:navigate", handler);
    return () => window.removeEventListener("multica:navigate", handler);
  }, [push]);
}

/**
 * The fragment is client-only state Next.js never surfaces: `usePathname()`
 * drops it, and a `router.replace("/x#y")` mutates `window.location` without
 * a render of its own. Reading it through an external store re-reads the URL
 * on every render and re-renders on the events that change it behind React's
 * back, so `adapter.hash` is never a stale copy.
 */
function subscribeToHash(onStoreChange: () => void): () => void {
  window.addEventListener("hashchange", onStoreChange);
  window.addEventListener("popstate", onStoreChange);
  return () => {
    window.removeEventListener("hashchange", onStoreChange);
    window.removeEventListener("popstate", onStoreChange);
  };
}

function NavigationProviderInner({
  children,
  intent,
  setIntent,
  syncBrowserScrollMode,
  captureBeforeNavigation,
}: {
  children: React.ReactNode;
  intent: NavigationIntent;
  setIntent: Dispatch<SetStateAction<NavigationIntent>>;
  syncBrowserScrollMode: () => void;
  captureBeforeNavigation: ScrollCapture;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const query = searchParams.toString();
  const route = pathname + (query ? `?${query}` : "");
  // Next writes the physical entry in an insertion effect. Read it after each
  // commit so a rendered-query/physical-URL gap never selects the wrong mode.
  useLayoutEffect(syncBrowserScrollMode);
  const navigateFragment = useCallback((path: string, method: "pushState" | "replaceState") => {
    const current = new URL(window.location.href);
    const target = new URL(path, current);
    if (target.origin !== current.origin || target.pathname + target.search !== route ||
      current.pathname + current.search !== route || (!target.hash && !current.hash)) return null;
    // Next 16.2.6's seeded segment cache retains the initial fragment and
    // appends the next one. Its supported History API keeps same-route hash
    // changes out of that cache, while preserving Back/Forward and route state.
    // The intent update also lets the hash store observe this history write.
    const destination = target.pathname + target.search + target.hash;
    if (target.href !== current.href) window.history[method](null, "", destination);
    return destination;
  }, [route]);
  const push = useCallback((path: string) => {
    captureBeforeNavigation(route);
    const fragment = navigateFragment(path, "pushState");
    setIntent((previous) => ({ path: fragment ?? path, resume: false, history: false, key: previous.key + 1 }));
    if (fragment === null) {
      // Notifications own their inner scrollers and exact-source focus.
      if (parseAccountPath(path)?.view === "notifications") router.push(path, { scroll: false });
      else router.push(path);
    }
  }, [router, navigateFragment, setIntent, captureBeforeNavigation, route]);
  const replace = useCallback((path: string, options?: { preserveView?: boolean }) => {
    captureBeforeNavigation(route);
    if (options?.preserveView) {
      const target = new URL(path, window.location.href);
      if (target.origin === window.location.origin) {
        // Next's supported History API keeps the current route tree mounted.
        // A router.replace of a UUID with its identifier would remount the
        // dynamic segment and discard a picker or editor already in use.
        window.history.replaceState(null, "", target.pathname + target.search + target.hash);
        setIntent((previous) => ({ ...previous, path: target.pathname + target.search + target.hash, history: false }));
        return;
      }
    }
    const fragment = navigateFragment(path, "replaceState");
    setIntent((previous) => ({ path: fragment ?? path, resume: false, history: false, key: previous.key + 1 }));
    if (fragment === null) {
      if (parseAccountPath(path)?.view === "notifications") router.replace(path, { scroll: false });
      else router.replace(path);
    }
  }, [router, navigateFragment, setIntent, captureBeforeNavigation, route]);
  const resume = useCallback((path: string) => {
    captureBeforeNavigation(route);
    const fragment = navigateFragment(path, "pushState");
    setIntent((previous) => ({ path: fragment ?? path, resume: true, history: false, key: previous.key + 1 }));
    if (fragment === null) router.push(path, { scroll: false });
  }, [router, navigateFragment, setIntent, captureBeforeNavigation, route]);
  const hash = useSyncExternalStore(
    subscribeToHash,
    () => window.location.hash,
    () => "",
  );
  useInternalLinkHandler(push);
  const browserRoute = typeof window === "undefined" ? route : window.location.pathname + window.location.search;
  const [committedIntent, setCommittedIntent] = useState({ location: route + hash, resume: false, history: false, key: 0 });
  const targetsRenderedRoute = intent.path.split("#")[0] === route;
  const newIntent = committedIntent.key !== intent.key || committedIntent.history !== intent.history;
  const committedRoute = committedIntent.location.split("#")[0];
  const committedHashIndex = committedIntent.location.indexOf("#");
  const committedHash = committedHashIndex < 0 ? "" : committedIntent.location.slice(committedHashIndex);
  const targetHashIndex = intent.path.indexOf("#");
  const targetHash = targetHashIndex < 0 ? "" : intent.path.slice(targetHashIndex);
  const awaitingTarget = newIntent && !targetsRenderedRoute && committedRoute === route;
  // Next's query and the browser URL can commit in either order. A fragment
  // belongs to its matching rendered route, including a target's empty hash.
  const renderedHash = awaitingTarget ? committedHash
    : targetsRenderedRoute && (newIntent || browserRoute !== route) ? targetHash
      : browserRoute === route ? hash : committedRoute === route ? committedHash : "";
  const location = route + renderedHash;
  // Next may keep the outgoing route mounted while a push is pending. Its
  // detail must retain the old focus intent until the destination renders.
  // Adjust during render so incoming children receive the intent on first mount.
  if (!awaitingTarget && (committedIntent.location !== location || (targetsRenderedRoute && newIntent))) {
    setCommittedIntent({ location, resume: targetsRenderedRoute && intent.resume, history: targetsRenderedRoute && intent.history, key: intent.key });
  }

  const adapter: NavigationAdapter = useMemo(() => ({
    push,
    replace,
    back: () => router.back(),
    forward: () => router.forward(),
    canGoBack: canGoBackInApp,
    pathname,
    searchParams: new URLSearchParams(searchParams.toString()),
    hash: renderedHash,
    isScopeResume: committedIntent.resume,
    isHistoryTraversal: committedIntent.history,
    navigationKey: committedIntent.key,
    getShareableUrl: (path: string) =>
      typeof window === "undefined" ? path : window.location.origin + path,
    // router.prefetch is a no-op in dev mode by Next.js design; in production
    // it warms the RSC payload + route chunk so the next push() commits with
    // no network round-trip. Safe to call repeatedly — Next dedupes internally.
    prefetch: (path: string) => {
      router.prefetch(path);
    },
  }), [push, replace, router, pathname, searchParams, renderedHash, committedIntent.resume, committedIntent.history, committedIntent.key]);

  return <ScopeResumeContext.Provider value={resume}><NavigationProvider value={adapter}>{children}</NavigationProvider></ScopeResumeContext.Provider>;
}

export function WebNavigationProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [intent, setIntent] = useState<NavigationIntent>({ path: "", resume: false, history: false, key: 0 });
  const scrollCapture = useRef<ScrollCapture | null>(null);
  const registerScrollCapture = useCallback((capture: ScrollCapture) => {
    scrollCapture.current = capture;
    return () => { if (scrollCapture.current === capture) scrollCapture.current = null; };
  }, []);
  const captureBeforeNavigation = useCallback((expectedPath?: string) => scrollCapture.current?.(expectedPath), []);
  const originalBrowserScrollMode = useRef<History["scrollRestoration"] | null>(null);
  const syncBrowserScrollMode = useCallback(() => {
    const original = originalBrowserScrollMode.current;
    if (original === null) return;
    const physicalPath = window.location.pathname + window.location.search + window.location.hash;
    const mode = parseAccountPath(physicalPath)?.view === "notifications" ? "manual" : original;
    if (window.history.scrollRestoration !== mode) window.history.scrollRestoration = mode;
  }, []);
  const recordHistory = useCallback(() => {
    captureBeforeNavigation();
    const target = window.location.pathname + window.location.search + window.location.hash;
    syncBrowserScrollMode();
    // The URL snapshot can render synchronously before a queued state update.
    // Commit return intent before that render can replay an old source anchor.
    flushSync(() => setIntent((previous) => ({ path: target, resume: false, history: true, key: previous.key + 1 })));
  }, [syncBrowserScrollMode, captureBeforeNavigation]);
  useLayoutEffect(() => {
    // Keep the original policy across repeated visits and inherited entry modes.
    originalBrowserScrollMode.current ??= window.history.scrollRestoration;
    syncBrowserScrollMode();
    // Native Window history dispatch follows listener registration order.
    // The non-suspending owner subscribes before Next's passive history hook.
    window.addEventListener("popstate", recordHistory, { capture: true });
    return () => {
      window.removeEventListener("popstate", recordHistory, { capture: true });
      const original = originalBrowserScrollMode.current;
      if (original !== null) window.history.scrollRestoration = original;
    };
  }, [recordHistory, syncBrowserScrollMode]);
  return (
    <ScrollCaptureRegistrationContext.Provider value={registerScrollCapture}>
      <Suspense>
        <NavigationProviderInner intent={intent} setIntent={setIntent} syncBrowserScrollMode={syncBrowserScrollMode} captureBeforeNavigation={captureBeforeNavigation}>{children}</NavigationProviderInner>
      </Suspense>
    </ScrollCaptureRegistrationContext.Provider>
  );
}
