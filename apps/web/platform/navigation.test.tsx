/**
 * MUL-5208 — the web half of the `multica:navigate` bridge.
 *
 * Shared content (comments, chat, issue descriptions) fires this event whenever
 * a link resolves to an in-app destination, including an absolute URL on this
 * deployment's own origin. Desktop answers it by opening a tab; the web must
 * answer it with a router push, or those links silently do nothing.
 */
import { describe, expect, it, vi, afterEach, beforeEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { startTransition, useEffect, useSyncExternalStore } from "react";

const router = vi.hoisted(() => ({
  push: vi.fn(),
  replace: vi.fn(),
  back: vi.fn(),
  forward: vi.fn(),
  prefetch: vi.fn(),
}));
const route = vi.hoisted((): { pathname: string; query: string; pending: Promise<void> | null } => ({ pathname: "/acme/issues", query: "", pending: null }));

vi.mock("next/navigation", () => ({
  useRouter: () => router,
  usePathname: () => route.pathname,
  useSearchParams: () => {
    if (route.pending) throw route.pending;
    return new URLSearchParams(route.query);
  },
}));

import { WebNavigationProvider, useWebScopeResume } from "./navigation";
import { AppLink, currentPath, useNavigation, type NavigationAdapter } from "@multica/views/navigation";

function navigate(path: string) {
  window.dispatchEvent(
    new CustomEvent("multica:navigate", { detail: { path } }),
  );
}

function renderAdapter(): () => NavigationAdapter {
  let adapter: NavigationAdapter | null = null;
  function Probe() {
    adapter = useNavigation();
    return null;
  }
  render(
    <WebNavigationProvider>
      <Probe />
    </WebNavigationProvider>,
  );
  return () => adapter!;
}

beforeEach(() => {
  router.push.mockReset();
  router.replace.mockReset();
  route.pathname = "/acme/issues";
  route.query = "";
  route.pending = null;
  window.history.replaceState(null, "", route.pathname);
  window.history.scrollRestoration = "auto";
});

describe("Web account fragment navigation", () => {
  const base = "/account?view=participating&issue=00000000-0000-4000-8000-000000000001";
  const first = `${base}#run-00000000-0000-4000-8000-000000000002`;
  const next = `${base}#run-00000000-0000-4000-8000-000000000003`;
  afterEach(() => { vi.restoreAllMocks(); window.history.replaceState(null, "", "/"); });

  function setup() {
    window.history.replaceState(null, "", first);
    route.pathname = "/account";
    route.query = base.split("?")[1]!;
    let adapter!: NavigationAdapter;
    function Probe() {
      adapter = useNavigation();
      return <><AppLink href={next}>Open source run</AppLink><output>{currentPath(adapter)}</output></>;
    }
    render(<WebNavigationProvider><Probe /></WebNavigationProvider>);
    return () => adapter;
  }

  it("opens a real source AppLink with one fragment and restores anchors on Back/Forward", async () => {
    const adapter = setup();
    const originalKey = adapter().navigationKey;
    const push = vi.spyOn(window.history, "pushState");
    // Next 16.2.6 seeds its route cache with the initial hash and appends the
    // next hash. The platform must use the supported History API for this swap.
    fireEvent.click(screen.getByRole("link", { name: "Open source run" }));
    expect(push).toHaveBeenCalledExactlyOnceWith(null, "", next);
    expect(router.push).not.toHaveBeenCalled();
    expect(screen.getByRole("status")).toHaveTextContent(next);
    expect(adapter().navigationKey).not.toBe(originalKey);
    expect(adapter().isScopeResume).toBe(false);
    const nextKey = adapter().navigationKey;
    fireEvent.click(screen.getByRole("link", { name: "Open source run" }));
    expect(push).toHaveBeenCalledTimes(1);
    expect(adapter().navigationKey).not.toBe(nextKey);
    act(() => window.history.back());
    await waitFor(() => expect(currentPath(adapter())).toBe(first));
    act(() => window.history.forward());
    await waitFor(() => expect(currentPath(adapter())).toBe(next));
  });

  it("replaces or removes a fragment without adding history, retaining the selected issue and view", () => {
    const adapter = setup();
    const replace = vi.spyOn(window.history, "replaceState");
    const push = vi.spyOn(window.history, "pushState");
    act(() => adapter().replace(next));
    expect(replace).toHaveBeenLastCalledWith(null, "", next);
    expect(currentPath(adapter())).toBe(next);
    act(() => adapter().replace(base));
    expect(replace).toHaveBeenLastCalledWith(null, "", base);
    expect(currentPath(adapter())).toBe(base);
    expect(push).not.toHaveBeenCalled();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("keeps path/query changes and another origin on the normal router path", () => {
    const adapter = setup();
    const push = vi.spyOn(window.history, "pushState");
    for (const destination of ["/account?view=current", "/acme/issues#description", `https://other.example${next}`]) {
      act(() => adapter().push(destination));
      expect(router.push).toHaveBeenLastCalledWith(destination);
    }
    expect(push).not.toHaveBeenCalled();
    expect(currentPath(adapter())).toBe(first);
  });
});

describe("Web query fragment ownership", () => {
  const issue = "00000000-0000-4000-8000-000000000001";
  const sourceQuery = `view=notifications&filter=all&notification=00000000-0000-4000-8000-000000000002&issue=${issue}`;
  const unavailable = "/account?view=notifications&filter=all&notification=00000000-0000-4000-8000-000000000003";
  afterEach(() => { vi.restoreAllMocks(); window.history.replaceState(null, "", "/"); });

  it.each(["push", "replace"] as const)("leaves notification %s scrolling with its list/detail owners while keeping workspace scrolling", (method) => {
    const adapter = renderAdapter();
    for (const target of ["/account?view=notifications&filter=unread", unavailable, `/account?${sourceQuery}#description`]) {
      act(() => adapter()[method](target));
      expect(router[method]).toHaveBeenLastCalledWith(target, { scroll: false });
    }
    act(() => adapter()[method]("/acme/projects"));
    expect(router[method]).toHaveBeenLastCalledWith("/acme/projects");
  });

  it.each([
    ["comment", "#comment-00000000-0000-4000-8000-000000000004"],
    ["run", "#run-00000000-0000-4000-8000-000000000005"],
    ["body", "#description"],
  ])("does not attach the outgoing %s fragment to an incoming unavailable notification", (_kind, fragment) => {
    const source = `/account?${sourceQuery}${fragment}`;
    window.history.replaceState(null, "", source);
    route.pathname = "/account";
    route.query = sourceQuery;
    let adapter!: NavigationAdapter;
    const published: string[] = [];
    function Probe() { adapter = useNavigation(); published.push(currentPath(adapter)); return null; }
    const page = () => <WebNavigationProvider><Probe /></WebNavigationProvider>;
    const view = render(page());
    const outgoingKey = adapter.navigationKey;
    act(() => adapter.push(unavailable));
    expect(currentPath(adapter)).toBe(source);
    expect(adapter.navigationKey).toBe(outgoingKey);
    expect(router.push).toHaveBeenLastCalledWith(unavailable, { scroll: false });

    // Next can render the new query before the browser drops the old hash.
    route.query = new URL(unavailable, window.location.href).searchParams.toString();
    view.rerender(page());
    expect(window.location.hash).toBe(fragment);
    expect(currentPath(adapter)).toBe(unavailable);
    expect(adapter.hash).toBe("");
    expect(adapter.navigationKey).not.toBe(outgoingKey);
    expect(adapter.isHistoryTraversal).toBe(false);
    expect(adapter.isScopeResume).toBe(false);
    expect(new Set(published)).toEqual(new Set([source, unavailable]));

    window.history.replaceState(null, "", unavailable);
    view.rerender(page());
    expect(currentPath(adapter)).toBe(unavailable);
  });

  it("retains the outgoing fragment when the browser reaches an explicit target before Next", () => {
    const source = `/account?${sourceQuery}#description`;
    const target = `${unavailable}&issue=${issue}#run-00000000-0000-4000-8000-000000000005`;
    window.history.replaceState(null, "", source);
    route.pathname = "/account";
    route.query = sourceQuery;
    let adapter!: NavigationAdapter;
    function Probe() { adapter = useNavigation(); return null; }
    const page = () => <WebNavigationProvider><Probe /></WebNavigationProvider>;
    const view = render(page());
    const outgoingKey = adapter.navigationKey;
    act(() => adapter.push(target));
    act(() => {
      window.history.replaceState(null, "", target);
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(currentPath(adapter)).toBe(source);
    expect(adapter.navigationKey).toBe(outgoingKey);
    route.query = new URL(target, window.location.href).searchParams.toString();
    view.rerender(page());
    expect(currentPath(adapter)).toBe(target);
    expect(adapter.navigationKey).not.toBe(outgoingKey);
  });

  it("reads later manual hash changes after an explicit query has committed", () => {
    const source = `/account?${sourceQuery}#description`;
    const target = `${unavailable}&issue=${issue}`;
    window.history.replaceState(null, "", source);
    route.pathname = "/account";
    route.query = sourceQuery;
    let adapter!: NavigationAdapter;
    function Probe() { adapter = useNavigation(); return null; }
    const page = () => <WebNavigationProvider><Probe /></WebNavigationProvider>;
    const view = render(page());
    act(() => adapter.push(target));
    route.query = new URL(target, window.location.href).searchParams.toString();
    view.rerender(page());
    expect(currentPath(adapter)).toBe(target);
    act(() => {
      window.history.replaceState(null, "", `${target}#discussion`);
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(currentPath(adapter)).toBe(`${target}#discussion`);
  });
});

describe("Web notification browser scroll ownership", () => {
  afterEach(() => vi.restoreAllMocks());

  it.each(["auto", "manual"] as const)("restores the prior %s browser scroll mode when notifications exit", (previous) => {
    const workspace = "/acme/issues";
    const notifications = "/account?view=notifications&filter=all&notification=00000000-0000-4000-8000-000000000002&issue=00000000-0000-4000-8000-000000000001#description";
    const add = vi.spyOn(window, "addEventListener");
    window.history.scrollRestoration = previous;
    const page = () => <WebNavigationProvider>{null}</WebNavigationProvider>;
    const view = render(page());
    const renderRoute = (path: string) => {
      const url = new URL(path, window.location.href);
      route.pathname = url.pathname;
      route.query = url.searchParams.toString();
      view.rerender(page());
    };
    expect(window.history.scrollRestoration).toBe(previous);
    // A rendered target cannot claim the outgoing physical history entry.
    renderRoute(notifications);
    expect(window.history.scrollRestoration).toBe(previous);
    window.history.pushState(null, "", notifications);
    view.rerender(page());
    expect(window.history.scrollRestoration).toBe("manual");
    renderRoute(workspace);
    expect(window.history.scrollRestoration).toBe("manual");
    window.history.pushState(null, "", workspace);
    view.rerender(page());
    expect(window.history.scrollRestoration).toBe(previous);

    // Back can update the physical entry before Next renders its query.
    act(() => {
      window.history.replaceState(null, "", notifications);
      window.dispatchEvent(new PopStateEvent("popstate"));
    });
    expect(window.history.scrollRestoration).toBe("manual");
    renderRoute(notifications);
    window.history.pushState(null, "", workspace);
    view.rerender(page());
    expect(window.history.scrollRestoration).toBe(previous);
    renderRoute(workspace);
    window.history.pushState(null, "", notifications);
    view.rerender(page());
    expect(window.history.scrollRestoration).toBe("manual");
    renderRoute(notifications);
    const historySubscriptions = add.mock.calls.filter(([event, , options]) => event === "popstate" && typeof options === "object" && options?.capture === true);
    expect(historySubscriptions).toHaveLength(1);
    view.unmount();
    expect(window.history.scrollRestoration).toBe(previous);
  });
});

describe("Web same-resource URL normalization", () => {
  afterEach(() => { vi.restoreAllMocks(); window.history.replaceState(null, "", "/"); });

  it("replaces the address without router navigation or a new history entry, retaining query and fragment", () => {
    const original = "/acme/issues/00000000-0000-4000-8000-000000000001?from=inbox#comment-7";
    const canonical = "/acme/issues/ACM-1?from=inbox#comment-7";
    window.history.replaceState(null, "", original);
    route.pathname = original.split("?")[0]!;
    route.query = "from=inbox";
    const adapter = renderAdapter();
    const navigationKey = adapter().navigationKey;
    const replace = vi.spyOn(window.history, "replaceState");
    const historyLength = window.history.length;

    act(() => adapter().replace(canonical, { preserveView: true }));

    expect(replace).toHaveBeenCalledExactlyOnceWith(null, "", canonical);
    expect(window.location.pathname + window.location.search + window.location.hash).toBe(canonical);
    expect(window.history.length).toBe(historyLength);
    expect(router.replace).not.toHaveBeenCalled();
    expect(adapter().navigationKey).toBe(navigationKey);
    expect(adapter().isScopeResume).toBe(false);
  });

  it("uses normal routing for ordinary replacements and another origin", () => {
    window.history.replaceState(null, "", "/acme/issues/ACM-1");
    route.pathname = "/acme/issues/ACM-1";
    const adapter = renderAdapter();
    const replace = vi.spyOn(window.history, "replaceState");
    act(() => adapter().replace("/acme/issues/ACM-2"));
    expect(router.replace).toHaveBeenLastCalledWith("/acme/issues/ACM-2");
    act(() => adapter().replace("https://other.example/acme/issues/ACM-1", { preserveView: true }));
    expect(router.replace).toHaveBeenLastCalledWith("https://other.example/acme/issues/ACM-1");
    expect(replace).not.toHaveBeenCalled();
  });
});

describe("Web scope resume intent", () => {
  it("retains the outgoing focus intent until the resumed destination renders", () => {
    route.pathname = "/account";
    route.query = "view=participating&issue=00000000-0000-4000-8000-000000000001";
    const account = `${route.pathname}?${route.query}#description`;
    let adapter!: NavigationAdapter;
    let resume!: (path: string) => void;
    function Probe() { adapter = useNavigation(); resume = useWebScopeResume(); return null; }
    const page = () => <WebNavigationProvider><Probe /></WebNavigationProvider>;
    const view = render(page());
    act(() => adapter.push(account));
    const explicitKey = adapter.navigationKey;
    act(() => resume("/acme/projects/p1"));
    expect(adapter.navigationKey).toBe(explicitKey);
    expect(adapter.isScopeResume).toBe(false);

    route.pathname = "/acme/projects/p1";
    route.query = "";
    view.rerender(page());
    expect(adapter.navigationKey).not.toBe(explicitKey);
    expect(adapter.isScopeResume).toBe(true);
    const resumedKey = adapter.navigationKey;

    act(() => resume(account));
    expect(adapter.navigationKey).toBe(resumedKey);
    expect(adapter.isScopeResume).toBe(true);
    route.pathname = "/account";
    route.query = "view=participating&issue=00000000-0000-4000-8000-000000000001";
    view.rerender(page());
    expect(adapter.navigationKey).not.toBe(resumedKey);
    expect(adapter.isScopeResume).toBe(true);
  });

  it("keeps resume distinct from repeated explicit opens of the same anchor", () => {
    route.pathname = "/account";
    route.query = "view=participating&issue=00000000-0000-4000-8000-000000000001";
    const path = `${route.pathname}?${route.query}#description`;
    let adapter!: NavigationAdapter;
    let resume!: (path: string) => void;
    function Probe() { adapter = useNavigation(); resume = useWebScopeResume(); return null; }
    render(<WebNavigationProvider><Probe /></WebNavigationProvider>);
    act(() => resume(path));
    expect(router.push).toHaveBeenLastCalledWith(path, { scroll: false });
    expect(adapter.isScopeResume).toBe(true);
    const resumedKey = adapter.navigationKey;
    act(() => adapter.push(path));
    expect(router.push).toHaveBeenLastCalledWith(path);
    expect(adapter.isScopeResume).toBe(false);
    expect(adapter.navigationKey).not.toBe(resumedKey);
    const explicitKey = adapter.navigationKey;
    act(() => adapter.push(path));
    expect(adapter.navigationKey).not.toBe(explicitKey);
  });
});

describe("Web history traversal intent", () => {
  const issue = "00000000-0000-4000-8000-000000000001";
  const first = `/account?view=notifications&filter=all&notification=00000000-0000-4000-8000-000000000002&issue=${issue}#description`;
  const second = `/account?view=notifications&filter=all&notification=00000000-0000-4000-8000-000000000003&issue=${issue}#run-00000000-0000-4000-8000-000000000004`;
  afterEach(() => { vi.restoreAllMocks(); window.history.replaceState(null, "", "/"); });

  it("registers history before an earlier router passive subscription", () => {
    const routerPopstate = () => undefined;
    const add = vi.spyOn(window, "addEventListener");
    function NextRouterSubscription() {
      useEffect(() => {
        window.addEventListener("popstate", routerPopstate);
        return () => window.removeEventListener("popstate", routerPopstate);
      }, []);
      return null;
    }
    // A preceding router effect must not install its listener ahead of the
    // app's history owner. Assert registration order, not dispatch phases.
    render(<><NextRouterSubscription /><WebNavigationProvider>{null}</WebNavigationProvider></>);
    const registrations = add.mock.calls.filter(([event]) => event === "popstate");
    const routerIndex = registrations.findIndex(([, listener]) => listener === routerPopstate);
    const historyIndex = registrations.findIndex(([, , options]) => typeof options === "object" && options?.capture === true);
    expect(historyIndex).toBeGreaterThanOrEqual(0);
    expect(routerIndex).toBeGreaterThanOrEqual(0);
    expect(historyIndex).toBeLessThan(routerIndex);
  });

  it("owns history before the router subscribes while the inner route is suspended", async () => {
    const outgoing = second.split("#")[0]!;
    window.history.replaceState(null, "", outgoing);
    route.pathname = "/account";
    route.query = new URL(outgoing, window.location.href).searchParams.toString();
    let resolve!: () => void;
    route.pending = new Promise<void>((ready) => { resolve = ready; });
    const add = vi.spyOn(window, "addEventListener");
    let adapter: NavigationAdapter | undefined;
    function Probe() { adapter = useNavigation(); return null; }
    const routerPopstate = () => { route.query = new URL(window.location.href).searchParams.toString(); };
    function NextRouterSubscription() {
      useEffect(() => {
        window.addEventListener("popstate", routerPopstate);
        return () => window.removeEventListener("popstate", routerPopstate);
      }, []);
      return null;
    }
    try {
      render(<><NextRouterSubscription /><WebNavigationProvider><Probe /></WebNavigationProvider></>);
      expect(adapter).toBeUndefined();
      const registrations = add.mock.calls.filter(([event]) => event === "popstate");
      const routerIndex = registrations.findIndex(([, listener]) => listener === routerPopstate);
      const historyIndex = registrations.findIndex(([, , options]) => typeof options === "object" && options?.capture === true);
      expect(historyIndex).toBeGreaterThanOrEqual(0);
      expect(historyIndex).toBeLessThan(routerIndex);
      await act(async () => {
        startTransition(() => {
          window.history.replaceState(null, "", first);
          const event = new PopStateEvent("popstate");
          registrations.forEach(([, listener]) => {
            if (typeof listener === "function") listener.call(window, event);
            else listener?.handleEvent(event);
          });
        });
      });
      expect(adapter).toBeUndefined();
      await act(async () => { route.pending = null; resolve(); });
      expect(adapter?.isHistoryTraversal).toBe(true);
      expect(adapter && currentPath(adapter)).toBe(first);
    } finally {
      await act(async () => { route.pending = null; resolve(); });
    }
  });

  it("commits native history intent before a synchronous external route render", async () => {
    const outgoing = second.split("#")[0]!;
    window.history.replaceState(null, "", outgoing);
    route.pathname = "/account";
    route.query = new URL(outgoing, window.location.href).searchParams.toString();
    const add = vi.spyOn(window, "addEventListener");
    const listeners = new Set<() => void>();
    const subscribe = (onChange: () => void) => { listeners.add(onChange); return () => { listeners.delete(onChange); }; };
    const incoming: boolean[] = [];
    function Probe() {
      const adapter = useNavigation();
      if (currentPath(adapter) === first) incoming.push(adapter.isHistoryTraversal === true);
      return null;
    }
    function ExternalRouter() {
      useSyncExternalStore(subscribe, () => route.query);
      return <WebNavigationProvider><Probe /></WebNavigationProvider>;
    }
    const routerPopstate = () => {
      route.query = new URL(window.location.href).searchParams.toString();
      listeners.forEach((onChange) => onChange());
    };
    function NextRouterSubscription() {
      useEffect(() => {
        window.addEventListener("popstate", routerPopstate);
        return () => window.removeEventListener("popstate", routerPopstate);
      }, []);
      return null;
    }
    render(<><NextRouterSubscription /><ExternalRouter /></>);
    const nativeListeners = add.mock.calls.filter(([event]) => event === "popstate").map(([, listener]) => listener);
    // Use the browser-observed Window registration order instead of jsdom's
    // synthetic capture ordering. External snapshots still render synchronously
    // while a transition state update is pending; do not flush that state here.
    await act(async () => {
      startTransition(() => {
        window.history.replaceState(null, "", first);
        const event = new PopStateEvent("popstate");
        nativeListeners.forEach((listener) => {
          if (typeof listener === "function") listener.call(window, event);
          else listener?.handleEvent(event);
        });
      });
    });
    expect(incoming.length).toBeGreaterThan(0);
    expect(incoming).not.toContain(false);
  });

  it("keeps the outgoing intent and fragment until the actual popstate destination renders", () => {
    window.history.replaceState(null, "", second);
    route.pathname = "/account";
    route.query = new URL(second, window.location.href).searchParams.toString();
    let adapter!: NavigationAdapter;
    function Probe() { adapter = useNavigation(); return null; }
    const page = () => <WebNavigationProvider><Probe /></WebNavigationProvider>;
    const view = render(page());
    act(() => adapter.push(second));
    const outgoingKey = adapter.navigationKey;
    act(() => adapter.back());
    expect(adapter.navigationKey).toBe(outgoingKey);
    expect(adapter.isHistoryTraversal).toBe(false);
    act(() => {
      window.history.replaceState(null, "", first);
      window.dispatchEvent(new PopStateEvent("popstate"));
    });
    expect(currentPath(adapter)).toBe(second);
    expect(adapter.navigationKey).toBe(outgoingKey);
    expect(adapter.isHistoryTraversal).toBe(false);
    route.query = new URL(first, window.location.href).searchParams.toString();
    view.rerender(page());
    expect(currentPath(adapter)).toBe(first);
    expect(adapter.isHistoryTraversal).toBe(true);
    expect(adapter.isScopeResume).toBe(false);
    expect(adapter.navigationKey).not.toBe(outgoingKey);
    const returnedKey = adapter.navigationKey;
    act(() => adapter.forward?.());
    expect(adapter.navigationKey).toBe(returnedKey);
    expect(adapter.isHistoryTraversal).toBe(true);
  });

  it("clears committed history intent for explicit opens, replacements and scope switches", () => {
    window.history.replaceState(null, "", first);
    route.pathname = "/account";
    route.query = new URL(first, window.location.href).searchParams.toString();
    let adapter!: NavigationAdapter;
    let resume!: (path: string) => void;
    function Probe() { adapter = useNavigation(); resume = useWebScopeResume(); return null; }
    render(<WebNavigationProvider><Probe /></WebNavigationProvider>);
    const pop = () => act(() => window.dispatchEvent(new PopStateEvent("popstate")));
    pop();
    expect(adapter.isHistoryTraversal).toBe(true);
    act(() => adapter.push(first));
    expect(adapter.isHistoryTraversal).toBe(false);
    pop();
    act(() => adapter.replace(first, { preserveView: true }));
    expect(adapter.isHistoryTraversal).toBe(false);
    pop();
    act(() => adapter.replace(first));
    expect(adapter.isHistoryTraversal).toBe(false);
    pop();
    act(() => resume(first));
    expect(adapter.isHistoryTraversal).toBe(false);
    expect(adapter.isScopeResume).toBe(true);
  });
});

describe("WebNavigationProvider internal link bridge", () => {
  it("pushes the path a content link resolved to", () => {
    render(<WebNavigationProvider>{null}</WebNavigationProvider>);

    navigate("/acme/issues/MUL-1");

    expect(router.push).toHaveBeenCalledWith("/acme/issues/MUL-1");
  });

  it("ignores an event without a path", () => {
    render(<WebNavigationProvider>{null}</WebNavigationProvider>);

    window.dispatchEvent(new CustomEvent("multica:navigate", { detail: {} }));

    expect(router.push).not.toHaveBeenCalled();
  });

  it("stops listening once unmounted", () => {
    const { unmount } = render(
      <WebNavigationProvider>{null}</WebNavigationProvider>,
    );

    unmount();
    navigate("/acme/issues/MUL-1");

    expect(router.push).not.toHaveBeenCalled();
  });
});

/**
 * `canGoBack` decides whether a page whose subject was just deleted steps back
 * or replaces with a fallback. A wrong `true` walks the user out of Multica,
 * so the adapter must expose the browser's own answer and nothing derived.
 */
describe("WebNavigationProvider canGoBack", () => {
  const win = window as unknown as { navigation?: unknown };

  afterEach(() => {
    delete win.navigation;
  });

  it("passes through the Navigation API's answer", () => {
    win.navigation = { canGoBack: true };

    expect(renderAdapter()().canGoBack!()).toBe(true);
  });

  it("reads the answer live rather than freezing it at render", () => {
    win.navigation = { canGoBack: true };
    const adapter = renderAdapter();

    win.navigation = { canGoBack: false };

    expect(adapter().canGoBack!()).toBe(false);
  });

  // Regression (PR review, twice): a count of `push` calls stood in for real
  // history here. It could not — Next drops a push to `replaceState` when the
  // canonical URL is unchanged, so a push that committed no entry still made
  // this claim `true`. Calling push must not move this answer at all.
  it("is unmoved by a push that committed no history entry", () => {
    win.navigation = { canGoBack: false };
    const adapter = renderAdapter();

    adapter().push("/acme/issues");

    expect(router.push).toHaveBeenCalledWith("/acme/issues");
    expect(adapter().canGoBack!()).toBe(false);
  });

  it("reports false where the browser cannot answer, so callers use the fallback", () => {
    expect(renderAdapter()().canGoBack!()).toBe(false);
  });
});

/**
 * MUL-6784 — the fragment is the only part of the current location Next.js
 * never hands to React: `usePathname()` drops it. Shared views rebuild share
 * and feedback links from the adapter, so a missing `hash` silently turns a
 * `#comment-…` deep link into a link to the whole page.
 */
describe("WebNavigationProvider hash", () => {
  afterEach(() => {
    window.location.hash = "";
  });

  it("reports the fragment of the page being rendered", () => {
    window.location.hash = "#comment-c1";

    expect(renderAdapter()().hash).toBe("#comment-c1");
  });

  it('reports "" when the location has no fragment', () => {
    expect(renderAdapter()().hash).toBe("");
  });

  it("re-reads the fragment when it changes after mount", async () => {
    const adapter = renderAdapter();

    await act(async () => {
      window.location.hash = "#comment-c2";
      // jsdom dispatches hashchange in a later task, like the browser.
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    expect(adapter().hash).toBe("#comment-c2");
  });
});
