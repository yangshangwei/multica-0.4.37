// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { useRestoredScrollOffset, useRestoredViewState, useViewStateWriter } from "@multica/views/platform";
import { WebScrollRestorationProvider } from "./scroll-restoration";

const route = vi.hoisted(() => ({ pathname: "/", query: "", owner: "viewer" }));
vi.mock("next/navigation", () => ({ usePathname: () => route.pathname, useSearchParams: () => new URLSearchParams(route.query) }));
vi.mock("./navigation-owner", () => ({ useWebNavigationOwner: () => route.owner }));
let testIdentity = 0;
beforeEach(() => { route.pathname = window.location.pathname; route.query = ""; route.owner = `viewer-${++testIdentity}`; });

function ScrollProbe({ containerKey }: { containerKey: string }) {
  const restored = useRestoredScrollOffset(containerKey);
  return <output data-testid="restored">{restored ?? "none"}</output>;
}

/** Dispatch a real scroll event from a marked container element. */
function scrollContainer(el: HTMLElement, top: number) {
  Object.defineProperty(el, "scrollTop", { configurable: true, value: top });
  Object.defineProperty(el, "scrollHeight", {
    configurable: true,
    value: 2000,
  });
  el.dispatchEvent(new Event("scroll", { bubbles: false }));
}

describe("WebScrollRestorationProvider", () => {
  it("keeps account view and selected issue offsets separate", () => {
    route.pathname = "/account";
    const first = "view=current&issue=00000000-0000-4000-8000-000000000001";
    route.query = first;
    const page = () => <WebScrollRestorationProvider><div data-tab-scroll-root="account-detail" data-testid="container" /><ScrollProbe containerKey="account-detail" /></WebScrollRestorationProvider>;
    const view = render(page());
    scrollContainer(screen.getByTestId("container"), 480);
    route.query = "view=participating&issue=00000000-0000-4000-8000-000000000002";
    view.rerender(page());
    expect(screen.getByTestId("restored")).toHaveTextContent("none");
    scrollContainer(screen.getByTestId("container"), 720);
    route.query = first;
    view.rerender(page());
    expect(screen.getByTestId("restored")).toHaveTextContent("480");
  });

  it("clears private mementos on identity change, including switching back", () => {
    const page = () => <WebScrollRestorationProvider><div data-tab-scroll-root="private" data-testid="container" /><ScrollProbe containerKey="private" /></WebScrollRestorationProvider>;
    const firstOwner = route.owner;
    const view = render(page());
    scrollContainer(screen.getByTestId("container"), 480);
    route.owner = "another-server-or-viewer";
    view.rerender(page());
    expect(screen.getByTestId("restored")).toHaveTextContent("none");
    route.owner = firstOwner;
    view.rerender(page());
    expect(screen.getByTestId("restored")).toHaveTextContent("none");
  });

  it("captures against the committed render when browser history still names the old route", () => {
    route.pathname = "/other/issues";
    const page = () => <WebScrollRestorationProvider><div data-tab-scroll-root="main" data-testid="container" /><ScrollProbe containerKey="main" /></WebScrollRestorationProvider>;
    const view = render(page());
    scrollContainer(screen.getByTestId("container"), 620);
    view.rerender(page());
    expect(screen.getByTestId("restored")).toHaveTextContent("620");
    route.pathname = window.location.pathname;
    view.rerender(page());
    expect(screen.getByTestId("restored")).toHaveTextContent("none");
  });
  it("clears only the named origin entry while preserving current-route writes", () => {
    const origin = "/acme/autopilots";
    function ViewStateProbe({ focusKey }: { focusKey: string }) {
      const focus = useRestoredViewState("pending-focus");
      const group = useRestoredViewState("template-group");
      const write = useViewStateWriter();
      return <>
        <output data-testid="focus">{focus ?? "none"}</output>
        <output data-testid="group">{group ?? "none"}</output>
        <button onClick={() => {
          write("pending-focus", focusKey);
          write("template-group", "maintenance");
        }}>Save current route</button>
        <button onClick={() => write("pending-focus", undefined, origin)}>Clear origin focus</button>
      </>;
    }
    const page = (focusKey: string) => <WebScrollRestorationProvider><ViewStateProbe focusKey={focusKey} /></WebScrollRestorationProvider>;
    route.pathname = origin;
    const view = render(page("source-card"));
    fireEvent.click(screen.getByRole("button", { name: "Save current route" }));

    route.pathname = "/acme/autopilots/new/template";
    view.rerender(page("current-card"));
    fireEvent.click(screen.getByRole("button", { name: "Save current route" }));
    fireEvent.click(screen.getByRole("button", { name: "Clear origin focus" }));
    view.rerender(page("current-card"));
    expect(screen.getByTestId("focus")).toHaveTextContent("current-card");

    route.pathname = origin;
    view.rerender(page("source-card"));
    expect(screen.getByTestId("focus")).toHaveTextContent("none");
    expect(screen.getByTestId("group")).toHaveTextContent("maintenance");
  });

  it("reads the rendered route's view state before browser history commits", () => {
    function ViewStateProbe() {
      const value = useRestoredViewState("template-group");
      const write = useViewStateWriter();
      return <button onClick={() => write("template-group", "maintenance")}>{value ?? "all"}</button>;
    }
    const view = render(<WebScrollRestorationProvider><ViewStateProbe /></WebScrollRestorationProvider>);
    fireEvent.click(screen.getByRole("button"));
    // Next can render the destination while window.location still names the
    // previous entry. Restoration must follow the rendered pathname.
    route.pathname = "/autopilots/new/template";
    view.rerender(<WebScrollRestorationProvider><ViewStateProbe /></WebScrollRestorationProvider>);
    expect(screen.getByRole("button")).toHaveTextContent("all");
    route.pathname = window.location.pathname;
    view.rerender(<WebScrollRestorationProvider><ViewStateProbe /></WebScrollRestorationProvider>);
    expect(screen.getByRole("button")).toHaveTextContent("maintenance");
  });

  it("serves a captured container offset back for the same pathname", () => {
    const view = render(
      <WebScrollRestorationProvider>
        <div data-tab-scroll-root="main" data-testid="container" />
        <ScrollProbe containerKey="main" />
      </WebScrollRestorationProvider>,
    );

    scrollContainer(screen.getByTestId("container"), 480);

    // Remount — the same pull the view performs after navigating back.
    view.rerender(
      <WebScrollRestorationProvider>
        <ScrollProbe containerKey="main" />
      </WebScrollRestorationProvider>,
    );
    expect(screen.getByTestId("restored").textContent).toBe("480");
  });

  it("clears the saved offset when the container scrolls back to the top", () => {
    const view = render(
      <WebScrollRestorationProvider>
        <div data-tab-scroll-root="cleared" data-testid="container" />
        <ScrollProbe containerKey="cleared" />
      </WebScrollRestorationProvider>,
    );

    scrollContainer(screen.getByTestId("container"), 480);
    scrollContainer(screen.getByTestId("container"), 0);

    view.rerender(
      <WebScrollRestorationProvider>
        <ScrollProbe containerKey="cleared" />
      </WebScrollRestorationProvider>,
    );
    expect(screen.getByTestId("restored").textContent).toBe("none");
  });

  it("shields a just-served memento from the clamped scroll events a restore produces", () => {
    const view = render(
      <WebScrollRestorationProvider>
        <div data-tab-scroll-root="shielded" data-testid="container" />
        <ScrollProbe containerKey="shielded" />
      </WebScrollRestorationProvider>,
    );

    scrollContainer(screen.getByTestId("container"), 480);

    // Remount serves 480 (and arms the write shield) …
    view.rerender(
      <WebScrollRestorationProvider>
        <div data-tab-scroll-root="shielded" data-testid="container" />
        <ScrollProbe containerKey="shielded" />
      </WebScrollRestorationProvider>,
    );
    expect(screen.getByTestId("restored").textContent).toBe("480");

    // … so the browser clamping the restore assignment (content not yet at
    // full height → scrollTop lands at 100) must not overwrite the memento.
    scrollContainer(screen.getByTestId("container"), 100);
    view.rerender(
      <WebScrollRestorationProvider>
        <ScrollProbe containerKey="shielded" />
      </WebScrollRestorationProvider>,
    );
    expect(screen.getByTestId("restored").textContent).toBe("480");
  });

  it("ignores scrolls from unmarked elements", () => {
    render(
      <WebScrollRestorationProvider>
        <div data-testid="plain" />
        <ScrollProbe containerKey="plain" />
      </WebScrollRestorationProvider>,
    );

    scrollContainer(screen.getByTestId("plain"), 480);
    expect(screen.getByTestId("restored").textContent).toBe("none");
  });
});
