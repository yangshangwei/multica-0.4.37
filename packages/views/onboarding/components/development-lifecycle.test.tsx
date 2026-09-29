import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import enOnboarding from "../../locales/en/onboarding.json";
import zhOnboarding from "../../locales/zh-Hans/onboarding.json";
import { DevelopmentLifecycle } from "./development-lifecycle";

const resources = {
  en: { onboarding: enOnboarding },
  "zh-Hans": { onboarding: zhOnboarding },
};

function Lifecycle({ locale = "en" }: { locale?: "en" | "zh-Hans" }) {
  return (
    <I18nProvider locale={locale} resources={resources}>
      <DevelopmentLifecycle />
    </I18nProvider>
  );
}

describe("DevelopmentLifecycle", () => {
  let media: MediaQueryList;
  let changes: EventTarget;

  beforeEach(() => {
    changes = new EventTarget();
    media = {
      matches: false,
      media: "(prefers-reduced-motion: reduce)",
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(changes.addEventListener.bind(changes)),
      removeEventListener: vi.fn(changes.removeEventListener.bind(changes)),
      dispatchEvent: changes.dispatchEvent.bind(changes),
    };
    vi.spyOn(window, "matchMedia").mockReturnValue(media);
  });

  afterEach(() => vi.restoreAllMocks());

  it("keeps the final collaboration and human decisions accessible during the entrance", () => {
    render(<Lifecycle />);
    const graph = screen.getByRole("list", { name: "ADLC" });

    expect(within(graph).getAllByRole("listitem")).toHaveLength(6);
    expect(within(graph).getByText("Agent review")).toBeInTheDocument();
    expect(within(graph).getByText("You set direction")).toBeInTheDocument();
    expect(within(graph).getByText("You decide")).toBeInTheDocument();
    expect(screen.getByText("Orchestrate agents")).toBeInTheDocument();
    expect(graph).toHaveAttribute("data-motion", "entering");
    expect(within(graph).getByText("Requirements").closest("[aria-hidden]"))
      .toHaveAttribute("aria-hidden", "true");
  });

  it("settles after the morph and never replays when its parent rerenders", () => {
    const { rerender } = render(<Lifecycle />);
    const graph = screen.getByRole("list", { name: "ADLC" });
    const stage = within(graph).getAllByRole("listitem")[0]!;
    const end = new Event("animationend", { bubbles: true });
    Object.defineProperty(end, "animationName", { value: "adlc-node-morph" });
    fireEvent(stage, end);

    expect(graph).toHaveAttribute("data-motion", "complete");
    rerender(<Lifecycle />);
    expect(graph).toHaveAttribute("data-motion", "complete");
    expect(window.matchMedia).toHaveBeenCalledTimes(1);
  });

  it("shows the final diagram immediately for reduced motion", () => {
    Object.defineProperty(media, "matches", { value: true });
    render(<Lifecycle locale="zh-Hans" />);

    expect(screen.getByRole("list", { name: "ADLC" }))
      .toHaveAttribute("data-motion", "complete");
    expect(screen.getByText("智能体评审")).toBeInTheDocument();
    expect(screen.getByText("你做决策")).toBeInTheDocument();
  });

  it("ends active movement when reduced motion is enabled and removes its listener on unmount", () => {
    const { unmount } = render(<Lifecycle />);
    const graph = screen.getByRole("list", { name: "ADLC" });
    Object.defineProperty(media, "matches", { value: true, configurable: true });
    act(() => changes.dispatchEvent(new Event("change")));
    expect(graph).toHaveAttribute("data-motion", "complete");

    Object.defineProperty(media, "matches", { value: false });
    act(() => changes.dispatchEvent(new Event("change")));
    expect(graph).toHaveAttribute("data-motion", "complete");

    unmount();
    expect(media.removeEventListener).toHaveBeenCalledWith(
      "change",
      vi.mocked(media.addEventListener).mock.calls[0]![1],
    );
  });
});
