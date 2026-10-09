import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { useSidebar } from "@multica/ui/components/ui/sidebar";
import { RESOURCES } from "@multica/views/locales";

// The shell resolves the mocked `getCurrentSlug()` against the workspace list
// before mounting workspace-scoped chrome, so the list has to contain it or
// the sidebar under test never renders. Gating behaviour itself is covered by
// desktop-layout.workspace-gate.test.tsx.
const WORKSPACES = [{ id: "ws-1", slug: "acme" }];

let reducedMotion = false;
let mediaEvents = new EventTarget();

beforeEach(() => {
  reducedMotion = false;
  mediaEvents = new EventTarget();
  vi.spyOn(window, "matchMedia").mockImplementation((query) => ({
    media: query,
    get matches() { return query === "(prefers-reduced-motion: reduce)" && reducedMotion; },
    onchange: null,
    addEventListener: mediaEvents.addEventListener.bind(mediaEvents),
    removeEventListener: mediaEvents.removeEventListener.bind(mediaEvents),
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: mediaEvents.dispatchEvent.bind(mediaEvents),
  }));
});

afterEach(() => vi.restoreAllMocks());

// The shell is the only thing under test here, so everything it mounts around
// the sidebar is stubbed out. What survives is the pair that has to agree:
// `WindowToolbar`'s own trigger, and the `hasExternalTrigger` the provider
// publishes to every page header inside the canvas.
vi.mock("@/hooks/use-tab-history", () => ({
  useTabHistory: () => ({
    canGoBack: false,
    canGoForward: false,
    goBack: vi.fn(),
    goForward: vi.fn(),
  }),
  useNavigationInputBindings: () => {},
}));

vi.mock("@/platform/navigation", () => ({
  DesktopNavigationProvider: ({ children }: { children: ReactNode }) => (
    <>{children}</>
  ),
  routeContentLinkPath: vi.fn(),
}));

vi.mock("@multica/core/paths", () => ({
  WorkspaceSlugProvider: ({ children }: { children: ReactNode }) => (
    <>{children}</>
  ),
  paths: { workspace: () => ({ inbox: () => "/acme/inbox" }) },
  useCurrentWorkspace: () => null,
}));

vi.mock("@multica/core/platform", () => ({
  getCurrentSlug: () => "acme",
  subscribeToCurrentSlug: () => () => {},
}));

vi.mock("@multica/core/workspace", () => ({
  workspaceListOptions: () => ({
    queryKey: ["workspace-list"],
    queryFn: async () => WORKSPACES,
  }),
}));

vi.mock("@multica/views/navigation", () => ({
  useNavigation: () => ({ push: vi.fn() }),
}));

vi.mock("@multica/views/platform", () => ({
  useDesktopUnreadBadge: () => {},
}));

vi.mock("@multica/views/layout", () => ({
  AppSidebar: () => null,
  GlobalShortcuts: () => null,
  NavigationProgress: () => null,
}));

vi.mock("@multica/views/modals/registry", () => ({ ModalRegistry: () => null }));
vi.mock("@multica/views/search", () => ({
  SearchCommand: () => null,
  SearchTrigger: () => null,
}));
vi.mock("@multica/views/chat", () => ({ FloatingChat: () => null }));
vi.mock("./tab-bar", () => ({ TabBar: () => null }));
vi.mock("./window-overlay", () => ({ WindowOverlay: () => null }));

// Stands in for whatever page the active tab is showing. Reports the one fact
// a `PageHeader` reads before deciding to render its own fallback trigger.
vi.mock("./tab-content", () => ({
  TabContent: () => {
    const { hasExternalTrigger } = useSidebar();
    return (
      <div data-testid="page-content" data-external-trigger={hasExternalTrigger} />
    );
  },
}));

const { DesktopShell } = await import("./desktop-layout");

function renderShell() {
  (
    window as unknown as { desktopAPI: Record<string, unknown> }
  ).desktopAPI = {
    onNavigationGesture: () => () => {},
    onInboxOpen: () => () => {},
  };

  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  qc.setQueryData(["workspace-list"], WORKSPACES);

  return render(
    <QueryClientProvider client={qc}>
      <I18nProvider locale="en" resources={RESOURCES}>
        <DesktopShell />
      </I18nProvider>
    </QueryClientProvider>,
  );
}

describe("DesktopShell sidebar trigger", () => {
  // The window toolbar parks a trigger beside the traffic lights that never
  // scrolls away, so nothing inside the canvas may add a second one. Desktop
  // windows sit below `xl`, exactly the band where `PageHeader`'s fallback
  // trigger renders, so every page used to stack an identical icon 50px under
  // this one — and a third when a list/detail surface brought its own header
  // along (MUL-6218).
  it("keeps exactly one trigger and tells page headers not to add another", () => {
    const { container, getByTestId } = renderShell();

    expect(container.querySelectorAll("[data-slot='sidebar-trigger']")).toHaveLength(1);
    expect(getByTestId("page-content")).toHaveAttribute(
      "data-external-trigger",
      "true",
    );
  });

  it.each(["at launch", "while running"])("applies final geometry when reduced motion is enabled %s", async (when) => {
    reducedMotion = when === "at launch";
    const { container, getByTestId } = renderShell();
    const header = container.querySelector("header")!;
    const dragRegion = header.firstElementChild as HTMLElement;
    const canvas = getByTestId("page-content").parentElement!;
    const trigger = container.querySelector("[data-slot='sidebar-trigger']")!;

    if (when === "while running") {
      act(() => {
        reducedMotion = true;
        mediaEvents.dispatchEvent(new Event("change"));
      });
    }

    for (const [padding, margin] of [[184, 8], [0, 2]]) {
      fireEvent.click(trigger);
      for (let frame = 0; frame < 5; frame++) {
        await act(async () => {
          await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
        });
        expect(header.style.paddingLeft).toBe(`${padding}px`);
        expect(dragRegion.style.left).toBe(`${padding}px`);
        expect(canvas.style.marginLeft).toBe(`${margin}px`);
      }
    }
  });

  it("finishes an in-flight animation when reduced motion is enabled and restores ordinary motion afterward", async () => {
    const { container, getByTestId } = renderShell();
    const header = container.querySelector("header")!;
    const dragRegion = header.firstElementChild as HTMLElement;
    const canvas = getByTestId("page-content").parentElement!;
    const trigger = container.querySelector("[data-slot='sidebar-trigger']")!;
    const nextFrame = async () => {
      await act(async () => {
        await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
      });
    };

    fireEvent.click(trigger);
    for (let frame = 0; frame < 5 && !Number.parseFloat(header.style.paddingLeft); frame++) {
      await nextFrame();
    }
    expect(Number.parseFloat(header.style.paddingLeft)).toBeGreaterThan(0);
    expect(Number.parseFloat(header.style.paddingLeft)).toBeLessThan(184);

    act(() => {
      reducedMotion = true;
      mediaEvents.dispatchEvent(new Event("change"));
    });
    await nextFrame();
    expect(header.style.paddingLeft).toBe("184px");
    expect(dragRegion.style.left).toBe("184px");
    expect(canvas.style.marginLeft).toBe("8px");

    act(() => {
      reducedMotion = false;
      mediaEvents.dispatchEvent(new Event("change"));
    });
    fireEvent.click(trigger);
    await nextFrame();
    expect(Number.parseFloat(header.style.paddingLeft)).toBeGreaterThan(0);
    expect(Number.parseFloat(header.style.paddingLeft)).toBeLessThan(184);
  });
});
