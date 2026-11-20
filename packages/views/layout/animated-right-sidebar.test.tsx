import { createRef, useState } from "react";
import { act, fireEvent, render, renderHook, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Popover, PopoverContent, PopoverTrigger } from "@multica/ui/components/ui/popover";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@multica/ui/components/ui/dropdown-menu";
import { Dialog, DialogContent, DialogTitle } from "@multica/ui/components/ui/dialog";
import { AlertDialog, AlertDialogContent, AlertDialogTitle } from "@multica/ui/components/ui/alert-dialog";
import { HoverCard, HoverCardContent, HoverCardTrigger } from "@multica/ui/components/ui/hover-card";
import { Tooltip, TooltipContent, TooltipTrigger } from "@multica/ui/components/ui/tooltip";
import { Sheet, SheetContent, SheetTitle, SheetTrigger } from "@multica/ui/components/ui/sheet";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  configureShortcutPlatform,
  useShortcutStore,
} from "@multica/core/shortcuts";

import {
  AnimatedRightSidebar,
  getAnimatedRightSidebarInitialOpen,
  useAnimatedRightSidebarState,
  useRightSidebarShortcut,
} from "./animated-right-sidebar";

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

afterEach(() => {
  vi.restoreAllMocks();
  configureShortcutPlatform(null);
  useShortcutStore.getState().resetAll();
});

describe("animated right sidebar accessibility", () => {
  it("keeps retained content inert and hidden from accessibility while collapsed", () => {
    const view = render(<AnimatedRightSidebar open><input aria-label="Draft" defaultValue="Keep me" /></AnimatedRightSidebar>);
    const input = screen.getByRole("textbox", { name: "Draft" });
    fireEvent.change(input, { target: { value: "Unsent draft" } });

    view.rerender(<AnimatedRightSidebar open={false}><input aria-label="Draft" defaultValue="Keep me" /></AnimatedRightSidebar>);

    expect(screen.queryByRole("textbox", { name: "Draft" })).not.toBeInTheDocument();
    expect(input.closest("[inert]")).not.toBeNull();
    view.rerender(<AnimatedRightSidebar open><input aria-label="Draft" defaultValue="Keep me" /></AnimatedRightSidebar>);
    expect(screen.getByRole("textbox", { name: "Draft" })).toBe(input);
    expect(input).toHaveValue("Unsent draft");
  });

  it("returns focus to the toggle only when the closing panel owned focus", () => {
    const toggleRef = createRef<HTMLButtonElement>();
    const ui = (open: boolean) => <>
      <button ref={toggleRef}>Toggle properties</button>
      <button>Page action</button>
      <AnimatedRightSidebar open={open} returnFocusRef={toggleRef}>
        <button>Property action</button>
      </AnimatedRightSidebar>
    </>;
    const view = render(ui(true));
    screen.getByRole("button", { name: "Property action" }).focus();
    view.rerender(ui(false));
    expect(toggleRef.current).toHaveFocus();

    view.rerender(ui(true));
    expect(toggleRef.current).toHaveFocus();
    screen.getByRole("button", { name: "Property action" }).focus();
    screen.getByRole("button", { name: "Page action" }).focus();
    view.rerender(ui(false));
    expect(screen.getByRole("button", { name: "Page action" })).toHaveFocus();
  });

  it.each(["at launch", "during a transition"])("snaps content when reduced motion is enabled %s", async (when) => {
    reducedMotion = when === "at launch";
    const view = render(<AnimatedRightSidebar open motionEnabled><button>Property action</button></AnimatedRightSidebar>);
    const panel = view.container.firstElementChild as HTMLElement;
    view.rerender(<AnimatedRightSidebar open={false} motionEnabled><button>Property action</button></AnimatedRightSidebar>);

    if (when === "during a transition") {
      act(() => {
        reducedMotion = true;
        mediaEvents.dispatchEvent(new Event("change"));
      });
    }

    await waitFor(() => {
      expect(panel.style.opacity).toBe("0");
      expect(panel.style.transform).toBe("none");
    });
  });

  it.each(["popover", "menu"])("dismisses an uncontrolled %s and allows a fresh opening without resurrecting it", async (kind) => {
    const user = userEvent.setup();
    const toggleRef = createRef<HTMLButtonElement>();
    const ui = (open: boolean) => <>
      <button ref={toggleRef}>Toggle properties</button>
      <AnimatedRightSidebar open={open} returnFocusRef={toggleRef}>
        {kind === "popover" ? <Popover>
          <PopoverTrigger>Open property</PopoverTrigger>
          <PopoverContent><button>Popup action</button></PopoverContent>
        </Popover> : <DropdownMenu modal={false}>
          <DropdownMenuTrigger>Open property</DropdownMenuTrigger>
          <DropdownMenuContent><DropdownMenuItem>Popup action</DropdownMenuItem></DropdownMenuContent>
        </DropdownMenu>}
      </AnimatedRightSidebar>
    </>;
    const view = render(ui(true));
    await user.click(screen.getByRole("button", { name: "Open property" }));
    const action = await screen.findByRole(kind === "menu" ? "menuitem" : "button", { name: "Popup action" });
    act(() => action.focus());
    view.rerender(ui(false));
    expect(screen.queryByRole(kind === "menu" ? "menuitem" : "button", { name: "Popup action" })).not.toBeInTheDocument();
    await waitFor(() => expect(toggleRef.current).toHaveFocus());

    view.rerender(ui(true));
    expect(screen.queryByRole(kind === "menu" ? "menuitem" : "button", { name: "Popup action" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Open property" }));
    expect(await screen.findByRole(kind === "menu" ? "menuitem" : "button", { name: "Popup action" })).toBeInTheDocument();
  });

  it("notifies a controlled popup through Base UI and does not reopen it on visibility alone", async () => {
    const close = vi.fn();
    function Controlled({ visible }: { visible: boolean }) {
      const [open, setOpen] = useState(true);
      return <AnimatedRightSidebar open={visible}>
        <Popover open={open} onOpenChange={(next, details) => { close(next, details.reason); setOpen(next); }}>
          <PopoverTrigger>Open property</PopoverTrigger>
          <PopoverContent><button>Popup action</button></PopoverContent>
        </Popover>
      </AnimatedRightSidebar>;
    }
    const view = render(<Controlled visible />);
    expect(await screen.findByRole("button", { name: "Popup action" })).toBeInTheDocument();
    view.rerender(<Controlled visible={false} />);
    expect(close).toHaveBeenCalledWith(false, "imperative-action");
    view.rerender(<Controlled visible />);
    expect(screen.queryByRole("button", { name: "Popup action" })).not.toBeInTheDocument();
  });

  it("honors a cancelled fresh trigger after suppressing a controlled popup", async () => {
    const user = userEvent.setup();
    const ui = (visible: boolean) => <AnimatedRightSidebar open={visible}>
      <Popover open onOpenChange={(next, details) => { if (next) details.cancel(); }}>
        <PopoverTrigger>Open property</PopoverTrigger>
        <PopoverContent><button>Popup action</button></PopoverContent>
      </Popover>
    </AnimatedRightSidebar>;
    const view = render(ui(true));
    expect(await screen.findByRole("button", { name: "Popup action" })).toBeVisible();
    view.rerender(ui(false));
    view.rerender(ui(true));
    await user.click(screen.getByRole("button", { name: "Open property" }));
    expect(screen.queryByRole("button", { name: "Popup action" })).not.toBeInTheDocument();
  });

  it("suppresses a controlled dialog even when its pending owner refuses closure", async () => {
    const close = vi.fn();
    const ui = (visible: boolean) => <AnimatedRightSidebar open={visible}>
      <Dialog open onOpenChange={close}>
        <DialogContent><DialogTitle>Pending property</DialogTitle><input aria-label="Pending draft" defaultValue="Keep" /></DialogContent>
      </Dialog>
    </AnimatedRightSidebar>;
    const view = render(ui(true));
    expect(await screen.findByRole("dialog", { name: "Pending property" })).toBeInTheDocument();
    view.rerender(ui(false));
    expect(screen.queryByRole("dialog", { name: "Pending property" })).not.toBeInTheDocument();
    view.rerender(ui(true));
    expect(screen.queryByRole("dialog", { name: "Pending property" })).not.toBeInTheDocument();
  });

  it("dismisses the project squad sheet and its nested select when the sidebar closes", async () => {
    const user = userEvent.setup();
    function SquadManager({ visible }: { visible: boolean }) {
      const [open, setOpen] = useState(false);
      return <AnimatedRightSidebar open={visible}>
        <Sheet open={open} onOpenChange={setOpen}>
          <SheetTrigger>Manage project squad</SheetTrigger>
          <SheetContent>
            <SheetTitle>Project squad</SheetTitle>
            <Select defaultValue="local" items={[{ value: "local", label: "Local runtime" }]}>
              <SelectTrigger aria-label="Runtime"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="local">Local runtime</SelectItem></SelectContent>
            </Select>
          </SheetContent>
        </Sheet>
      </AnimatedRightSidebar>;
    }
    const view = render(<SquadManager visible />);
    await user.click(screen.getByRole("button", { name: "Manage project squad" }));
    expect(await screen.findByRole("dialog", { name: "Project squad" })).toBeVisible();
    await user.click(screen.getByRole("combobox", { name: "Runtime" }));
    expect(await screen.findByRole("option", { name: "Local runtime" })).toBeVisible();

    view.rerender(<SquadManager visible={false} />);
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Project squad" })).not.toBeInTheDocument());
    expect(screen.queryByRole("option", { name: "Local runtime" })).not.toBeInTheDocument();
    view.rerender(<SquadManager visible />);
    expect(screen.queryByRole("dialog", { name: "Project squad" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Manage project squad" }));
    expect(await screen.findByRole("dialog", { name: "Project squad" })).toBeVisible();
  });

  it.each([false, true])("closes a sidebar select without changing its selected value (controlled open: %s)", async (controlled) => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    const ui = (visible: boolean) => <AnimatedRightSidebar open={visible}>
      <Select items={[{ value: "local", label: "Local runtime" }]} defaultValue="local"
        open={controlled ? true : undefined} onValueChange={onValueChange}>
        <SelectTrigger aria-label="Runtime"><SelectValue /></SelectTrigger>
        <SelectContent><SelectItem value="local">Local runtime</SelectItem></SelectContent>
      </Select>
    </AnimatedRightSidebar>;
    const view = render(ui(true));
    if (!controlled) await user.click(screen.getByRole("combobox", { name: "Runtime" }));
    expect(await screen.findByRole("option", { name: "Local runtime" })).toBeVisible();
    view.rerender(ui(false));
    await waitFor(() => expect(screen.queryByRole("option", { name: "Local runtime" })).not.toBeInTheDocument());
    view.rerender(ui(true));
    expect(screen.queryByRole("option", { name: "Local runtime" })).not.toBeInTheDocument();
    const trigger = screen.getByRole("combobox", { name: "Runtime" });
    expect(trigger).toHaveTextContent("Local runtime");
    await user.click(trigger);
    expect(await screen.findByRole("option", { name: "Local runtime" })).toBeVisible();
    expect(onValueChange).not.toHaveBeenCalled();
  });

  it("closes a sidebar alert dialog and respects a consumer's native actions ref", async () => {
    const actionsRef = createRef<{ close(): void; unmount(): void }>();
    const onOpenChange = vi.fn();
    const ui = (visible: boolean) => <AnimatedRightSidebar open={visible}>
      <AlertDialog defaultOpen actionsRef={actionsRef} onOpenChange={onOpenChange}>
        <AlertDialogContent><AlertDialogTitle>Blocked action</AlertDialogTitle></AlertDialogContent>
      </AlertDialog>
    </AnimatedRightSidebar>;
    const view = render(ui(true));
    expect(await screen.findByRole("alertdialog", { name: "Blocked action" })).toBeInTheDocument();
    expect(actionsRef.current?.close).toBeTypeOf("function");
    view.rerender(ui(false));
    expect(screen.queryByRole("alertdialog", { name: "Blocked action" })).not.toBeInTheDocument();
    expect(onOpenChange).toHaveBeenCalledWith(false, expect.objectContaining({ reason: "imperative-action" }));
  });

  it("does not auto-open a hidden-first default-open popup when the sidebar appears", async () => {
    const user = userEvent.setup();
    const ui = (visible: boolean) => <AnimatedRightSidebar open={visible}>
      <Popover defaultOpen>
        <PopoverTrigger>Open property</PopoverTrigger>
        <PopoverContent><button>Popup action</button></PopoverContent>
      </Popover>
    </AnimatedRightSidebar>;
    const view = render(ui(false));
    view.rerender(ui(true));
    expect(screen.queryByRole("button", { name: "Popup action" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Open property" }));
    expect(await screen.findByRole("button", { name: "Popup action" })).toBeInTheDocument();
  });

  it.each(["hover card", "tooltip"])("dismisses a default-open %s with the retained surface", async (kind) => {
    const close = vi.fn();
    const ui = (visible: boolean) => <AnimatedRightSidebar open={visible}>
      {kind === "hover card" ? <HoverCard defaultOpen onOpenChange={close}>
        <HoverCardTrigger>Profile</HoverCardTrigger>
        <HoverCardContent><button>Profile action</button></HoverCardContent>
      </HoverCard> : <Tooltip defaultOpen onOpenChange={close}>
        <TooltipTrigger>Help</TooltipTrigger>
        <TooltipContent>Property help</TooltipContent>
      </Tooltip>}
    </AnimatedRightSidebar>;
    const view = render(ui(true));
    const popup = kind === "hover card"
      ? await screen.findByRole("button", { name: "Profile action" })
      : await screen.findByText("Property help");
    expect(popup).toBeVisible();
    view.rerender(ui(false));
    expect(close).toHaveBeenCalledWith(false, expect.objectContaining({ reason: "imperative-action" }));
    view.rerender(ui(true));
    if (kind === "hover card") {
      expect(screen.queryByRole("button", { name: "Profile action" })).not.toBeInTheDocument();
    } else {
      await waitFor(() => expect(screen.queryByText("Property help")).not.toBeInTheDocument());
    }
  });
});

describe("animated right sidebar state", () => {
  it("uses a restored collapsed layout before falling back to the default", () => {
    expect(
      getAnimatedRightSidebarInitialOpen(true, {
        content: 100,
        sidebar: 0,
      }),
    ).toBe(false);
  });

  it("uses a restored expanded layout before falling back to the default", () => {
    expect(
      getAnimatedRightSidebarInitialOpen(false, {
        content: 70,
        sidebar: 30,
      }),
    ).toBe(true);
  });

  it("falls back to the caller default when no sidebar layout was restored", () => {
    expect(getAnimatedRightSidebarInitialOpen(true, undefined)).toBe(true);
    expect(getAnimatedRightSidebarInitialOpen(false, { content: 100 })).toBe(false);
  });

  it("treats a non-zero layout percentage as open even before pixels are measured", () => {
    const { result } = renderHook(() => useAnimatedRightSidebarState(false));

    act(() => {
      result.current.handleResize({
        asPercentage: 30,
        inPixels: 0,
      });
    });

    expect(result.current.open).toBe(true);
    expect(result.current.visualOpen).toBe(true);
    expect(result.current.motionEnabled).toBe(false);
  });

  it("enables motion only for an explicit toggle window", () => {
    vi.useFakeTimers();
    try {
      const { result } = renderHook(() => useAnimatedRightSidebarState(false));

      expect(result.current.motionEnabled).toBe(false);

      act(() => {
        result.current.beginToggle(true);
      });

      expect(result.current.open).toBe(true);
      expect(result.current.visualOpen).toBe(true);
      expect(result.current.motionEnabled).toBe(true);

      act(() => {
        vi.runAllTimers();
      });

      expect(result.current.motionEnabled).toBe(false);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("right sidebar shortcut", () => {
  it("toggles the visible detail sidebar on Mod+/", () => {
    configureShortcutPlatform("macos");
    const onToggle = vi.fn();
    const target = document.createElement("div");
    vi.spyOn(target, "getClientRects").mockReturnValue(
      [{}] as unknown as DOMRectList,
    );
    const targetRef = { current: target };
    const { unmount } = renderHook(() =>
      useRightSidebarShortcut(targetRef, onToggle),
    );
    const event = new KeyboardEvent("keydown", {
      key: "/",
      metaKey: true,
      cancelable: true,
    });

    document.dispatchEvent(event);

    expect(onToggle).toHaveBeenCalledTimes(1);
    expect(event.defaultPrevented).toBe(true);
    unmount();
  });

  it("leaves the shortcut unclaimed when its detail page is hidden", () => {
    configureShortcutPlatform("macos");
    const onToggle = vi.fn();
    const target = document.createElement("div");
    const targetRef = { current: target };
    const { unmount } = renderHook(() =>
      useRightSidebarShortcut(targetRef, onToggle),
    );
    const event = new KeyboardEvent("keydown", {
      key: "/",
      metaKey: true,
      cancelable: true,
    });

    document.dispatchEvent(event);

    expect(onToggle).not.toHaveBeenCalled();
    expect(event.defaultPrevented).toBe(false);
    unmount();
  });
});
