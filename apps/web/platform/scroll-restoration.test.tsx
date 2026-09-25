// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { useRestoredScrollOffset, useRestoredViewState, useViewStateWriter } from "@multica/views/platform";
import { WebScrollRestorationProvider } from "./scroll-restoration";

const route = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("next/navigation", () => ({ usePathname: () => route.pathname }));
beforeEach(() => { route.pathname = window.location.pathname; });

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
