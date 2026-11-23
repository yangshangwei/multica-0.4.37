import { act, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  Sidebar,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
  useSidebar,
} from "@multica/ui/components/ui/sidebar";
import { renderWithI18n } from "../test/i18n";

describe("left sidebar resizing", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute("data-sidebar-resizing");
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("previews width directly and commits only when the pointer is released", () => {
    const stableConsumerRender = vi.fn();
    const setItem = vi.spyOn(Storage.prototype, "setItem");

    function StableSidebarConsumer() {
      useSidebar();
      stableConsumerRender();
      return null;
    }

    const { container } = renderWithI18n(
      <SidebarProvider>
        <Sidebar>
          <StableSidebarConsumer />
          <SidebarRail />
        </Sidebar>
      </SidebarProvider>,
    );

    const wrapper = container.querySelector<HTMLElement>("[data-slot='sidebar-wrapper']")!;
    const sidebarContainer = container.querySelector<HTMLElement>("[data-slot='sidebar-container']")!;
    const sidebarGap = container.querySelector<HTMLElement>("[data-slot='sidebar-gap']")!;
    const sidebar = container.querySelector<HTMLElement>("[data-slot='sidebar']")!;
    const rail = container.querySelector<HTMLButtonElement>("[data-slot='sidebar-rail']")!;
    const setPointerCapture = vi.fn();
    const releasePointerCapture = vi.fn();
    rail.setPointerCapture = setPointerCapture;
    rail.hasPointerCapture = vi.fn(() => true);
    rail.releasePointerCapture = releasePointerCapture;

    vi.spyOn(sidebarContainer, "getBoundingClientRect").mockReturnValue({
      bottom: 0,
      height: 0,
      left: 0,
      right: 256,
      top: 0,
      width: 256,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    });

    fireEvent.pointerDown(rail, {
      button: 0,
      clientX: 256,
      isPrimary: true,
      pointerId: 7,
    });

    expect(setPointerCapture).toHaveBeenCalledWith(7);
    expect(rail).toHaveClass("cursor-ew-resize");
    expect(wrapper).toHaveAttribute("data-sidebar-resizing", "true");
    expect(document.documentElement).toHaveAttribute("data-sidebar-resizing", "true");

    fireEvent.pointerMove(document, { buttons: 1, clientX: 280, pointerId: 7 });
    fireEvent.pointerMove(document, { buttons: 1, clientX: 300, pointerId: 7 });

    expect(sidebarGap.style.width).toBe("300px");
    expect(sidebarContainer.style.width).toBe("300px");
    expect(wrapper.style.getPropertyValue("--sidebar-width")).toBe("256px");
    expect(setItem).not.toHaveBeenCalled();
    expect(stableConsumerRender).toHaveBeenCalledTimes(1);

    fireEvent.pointerUp(document, { pointerId: 7 });

    expect(sidebarGap.style.width).toBe("");
    expect(sidebarContainer.style.width).toBe("");
    expect(wrapper.style.getPropertyValue("--sidebar-width")).toBe("300px");
    expect(setItem).toHaveBeenCalledTimes(1);
    expect(setItem).toHaveBeenCalledWith("sidebar_width", "300");
    expect(releasePointerCapture).toHaveBeenCalledWith(7);
    expect(wrapper).not.toHaveAttribute("data-sidebar-resizing");
    expect(document.documentElement).not.toHaveAttribute("data-sidebar-resizing");
    expect(stableConsumerRender).toHaveBeenCalledTimes(1);

    fireEvent.click(rail);
    expect(sidebar).toHaveAttribute("data-state", "expanded");
  });

  it("restores the committed width and cursor state when pointer capture is cancelled", () => {
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const { container } = renderWithI18n(
      <SidebarProvider>
        <Sidebar>
          <SidebarRail />
        </Sidebar>
      </SidebarProvider>,
    );

    const wrapper = container.querySelector<HTMLElement>("[data-slot='sidebar-wrapper']")!;
    const sidebarContainer = container.querySelector<HTMLElement>("[data-slot='sidebar-container']")!;
    const sidebarGap = container.querySelector<HTMLElement>("[data-slot='sidebar-gap']")!;
    const rail = container.querySelector<HTMLButtonElement>("[data-slot='sidebar-rail']")!;
    rail.setPointerCapture = vi.fn();
    rail.hasPointerCapture = vi.fn(() => true);
    rail.releasePointerCapture = vi.fn();

    vi.spyOn(sidebarContainer, "getBoundingClientRect").mockReturnValue({
      bottom: 0,
      height: 0,
      left: 0,
      right: 256,
      top: 0,
      width: 256,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    });

    fireEvent.pointerDown(rail, {
      button: 0,
      clientX: 256,
      isPrimary: true,
      pointerId: 8,
    });
    fireEvent.pointerMove(document, { buttons: 1, clientX: 320, pointerId: 8 });
    fireEvent.pointerCancel(document, { pointerId: 8 });

    expect(sidebarGap.style.width).toBe("");
    expect(sidebarContainer.style.width).toBe("");
    expect(wrapper.style.getPropertyValue("--sidebar-width")).toBe("256px");
    expect(setItem).not.toHaveBeenCalled();
    expect(wrapper).not.toHaveAttribute("data-sidebar-resizing");
    expect(document.documentElement).not.toHaveAttribute("data-sidebar-resizing");
  });

  it("preserves the expanded width when a collapsed icon rail receives a pointer gesture", () => {
    localStorage.setItem("sidebar_width", "320");
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const { container } = renderWithI18n(
      <SidebarProvider defaultOpen={false}>
        <Sidebar variant="inset" collapsible="icon">
          <SidebarTrigger />
          <SidebarRail />
        </Sidebar>
      </SidebarProvider>,
    );
    const wrapper = container.querySelector<HTMLElement>("[data-slot='sidebar-wrapper']")!;
    const sidebarContainer = container.querySelector<HTMLElement>("[data-slot='sidebar-container']")!;
    const rail = container.querySelector<HTMLButtonElement>("[data-slot='sidebar-rail']")!;
    vi.spyOn(sidebarContainer, "getBoundingClientRect").mockReturnValue(new DOMRect(0, 0, 64, 800));

    fireEvent.pointerDown(rail, { button: 0, clientX: 64, isPrimary: true, pointerId: 9 });
    fireEvent.pointerMove(document, { buttons: 1, clientX: 100, pointerId: 9 });
    fireEvent.pointerUp(document, { pointerId: 9 });

    expect(wrapper).not.toHaveAttribute("data-sidebar-resizing");
    expect(setItem).not.toHaveBeenCalled();
    fireEvent.click(container.querySelector("[data-slot='sidebar-trigger']")!);
    expect(container.querySelector("[data-slot='sidebar']")).toHaveAttribute("data-state", "expanded");
    expect(wrapper.style.getPropertyValue("--sidebar-width")).toBe("320px");
  });

  it("cancels an active width preview when the sidebar folds to its icon rail", () => {
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const { container } = renderWithI18n(
      <SidebarProvider>
        <Sidebar variant="inset" collapsible="icon">
          <SidebarTrigger />
          <SidebarRail />
        </Sidebar>
      </SidebarProvider>,
    );
    const wrapper = container.querySelector<HTMLElement>("[data-slot='sidebar-wrapper']")!;
    const sidebarContainer = container.querySelector<HTMLElement>("[data-slot='sidebar-container']")!;
    const gap = container.querySelector<HTMLElement>("[data-slot='sidebar-gap']")!;
    const rail = container.querySelector<HTMLButtonElement>("[data-slot='sidebar-rail']")!;
    rail.setPointerCapture = vi.fn();
    rail.hasPointerCapture = vi.fn(() => true);
    rail.releasePointerCapture = vi.fn();
    vi.spyOn(sidebarContainer, "getBoundingClientRect").mockReturnValue(new DOMRect(0, 0, 256, 800));

    fireEvent.pointerDown(rail, { button: 0, clientX: 256, isPrimary: true, pointerId: 10 });
    fireEvent.pointerMove(document, { buttons: 1, clientX: 300, pointerId: 10 });
    expect(gap.style.width).toBe("300px");
    fireEvent.click(container.querySelector("[data-slot='sidebar-trigger']")!);

    expect(gap.style.width).toBe("");
    expect(sidebarContainer.style.width).toBe("");
    expect(wrapper).not.toHaveAttribute("data-sidebar-resizing");
    expect(document.documentElement).not.toHaveAttribute("data-sidebar-resizing");
    fireEvent.pointerUp(document, { pointerId: 10 });
    expect(setItem).not.toHaveBeenCalled();
    expect(wrapper.style.getPropertyValue("--sidebar-width")).toBe("256px");
    expect(rail.releasePointerCapture).toHaveBeenCalledWith(10);
  });

  it("cancels the width preview when the in-flow sidebar becomes a compact sheet", () => {
    const mediaEvents = new EventTarget();
    vi.stubGlobal("innerWidth", 1440);
    vi.spyOn(window, "matchMedia").mockImplementation((query) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: mediaEvents.addEventListener.bind(mediaEvents),
      removeEventListener: mediaEvents.removeEventListener.bind(mediaEvents),
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: mediaEvents.dispatchEvent.bind(mediaEvents),
    }));
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const { container } = renderWithI18n(
      <SidebarProvider>
        <Sidebar variant="inset" collapsible="icon"><SidebarRail /></Sidebar>
      </SidebarProvider>,
    );
    const wrapper = container.querySelector<HTMLElement>("[data-slot='sidebar-wrapper']")!;
    const sidebarContainer = container.querySelector<HTMLElement>("[data-slot='sidebar-container']")!;
    const gap = container.querySelector<HTMLElement>("[data-slot='sidebar-gap']")!;
    const rail = container.querySelector<HTMLButtonElement>("[data-slot='sidebar-rail']")!;
    vi.spyOn(sidebarContainer, "getBoundingClientRect").mockReturnValue(new DOMRect(0, 0, 256, 800));

    fireEvent.pointerDown(rail, { button: 0, clientX: 256, isPrimary: true, pointerId: 11 });
    fireEvent.pointerMove(document, { buttons: 1, clientX: 300, pointerId: 11 });
    expect(gap.style.width).toBe("300px");
    act(() => {
      vi.stubGlobal("innerWidth", 900);
      mediaEvents.dispatchEvent(new Event("change"));
    });

    expect(container.querySelector("[data-slot='sidebar-container']")).toBeNull();
    expect(gap.style.width).toBe("");
    expect(sidebarContainer.style.width).toBe("");
    expect(wrapper).not.toHaveAttribute("data-sidebar-resizing");
    expect(document.documentElement).not.toHaveAttribute("data-sidebar-resizing");
    fireEvent.pointerUp(document, { pointerId: 11 });
    expect(setItem).not.toHaveBeenCalled();
  });
});
