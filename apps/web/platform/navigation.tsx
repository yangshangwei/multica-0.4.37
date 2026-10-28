"use client";

import { createContext, Suspense, useCallback, useContext, useEffect, useMemo, useState, useSyncExternalStore } from "react";
import { useRouter, usePathname, useSearchParams } from "next/navigation";
import {
  NavigationProvider,
  type NavigationAdapter,
} from "@multica/views/navigation";
import { canGoBackInApp } from "./in-app-history";

const ScopeResumeContext = createContext<(path: string) => void>(() => undefined);

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
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [intent, setIntent] = useState({ path: "", resume: false, key: 0 });
  const push = useCallback((path: string) => {
    setIntent((previous) => ({ path, resume: false, key: previous.key + 1 }));
    router.push(path);
  }, [router]);
  const replace = useCallback((path: string) => {
    setIntent((previous) => ({ path, resume: false, key: previous.key + 1 }));
    router.replace(path);
  }, [router]);
  const resume = useCallback((path: string) => {
    setIntent((previous) => ({ path, resume: true, key: previous.key + 1 }));
    router.push(path, { scroll: false });
  }, [router]);
  const hash = useSyncExternalStore(
    subscribeToHash,
    () => window.location.hash,
    () => "",
  );
  useInternalLinkHandler(push);
  const query = searchParams.toString();
  const route = pathname + (query ? `?${query}` : "");
  const location = route + hash;
  const targetsRenderedRoute = intent.path.split("#")[0] === route;
  const [committedIntent, setCommittedIntent] = useState({ location, resume: false, key: 0 });
  // Next may keep the outgoing route mounted while a push is pending. Its
  // detail must retain the old focus intent until the destination renders.
  // Adjust during render so incoming children receive the intent on first mount.
  if (committedIntent.location !== location || (targetsRenderedRoute && committedIntent.key !== intent.key)) {
    setCommittedIntent({ location, resume: targetsRenderedRoute && intent.resume, key: intent.key });
  }
  const clearIntent = useCallback(() => {
    setIntent((previous) => ({ path: "", resume: false, key: previous.key + 1 }));
  }, []);
  useEffect(() => {
    window.addEventListener("popstate", clearIntent);
    return () => window.removeEventListener("popstate", clearIntent);
  }, [clearIntent]);

  const adapter: NavigationAdapter = useMemo(() => ({
    push,
    replace,
    back: () => { clearIntent(); router.back(); },
    forward: () => { clearIntent(); router.forward(); },
    canGoBack: canGoBackInApp,
    pathname,
    searchParams: new URLSearchParams(searchParams.toString()),
    hash,
    isScopeResume: committedIntent.resume,
    navigationKey: committedIntent.key,
    getShareableUrl: (path: string) =>
      typeof window === "undefined" ? path : window.location.origin + path,
    // router.prefetch is a no-op in dev mode by Next.js design; in production
    // it warms the RSC payload + route chunk so the next push() commits with
    // no network round-trip. Safe to call repeatedly — Next dedupes internally.
    prefetch: (path: string) => {
      router.prefetch(path);
    },
  }), [push, replace, clearIntent, router, pathname, searchParams, hash, committedIntent.resume, committedIntent.key]);

  return <ScopeResumeContext.Provider value={resume}><NavigationProvider value={adapter}>{children}</NavigationProvider></ScopeResumeContext.Provider>;
}

export function WebNavigationProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <Suspense>
      <NavigationProviderInner>{children}</NavigationProviderInner>
    </Suspense>
  );
}
