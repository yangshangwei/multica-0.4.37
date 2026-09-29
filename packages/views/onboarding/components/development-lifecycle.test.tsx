import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

function endAnimation(element: Element, animationName: string) {
  // jsdom exposes WebkitAnimation but no AnimationEvent, so React listens for
  // the prefixed event. Browsers with AnimationEvent use the standard name.
  const eventName = "AnimationEvent" in window ? "animationend" : "webkitAnimationEnd";
  const event = new Event(eventName, { bubbles: true });
  Object.defineProperty(event, "animationName", { value: animationName });
  fireEvent(element, event);
}

function completeTransition(figure: HTMLElement) {
  const feedbackSignal = figure.querySelector(".adlc-feedback-signal");
  expect(feedbackSignal).toBeInTheDocument();
  endAnimation(feedbackSignal!, "adlc-feedback-travel");
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
    const figure = screen.getByRole("figure", {
      name: "From traditional to agentic software development",
    });
    const graph = screen.getByRole("list", { name: "ADLC" });

    expect(within(graph).getAllByRole("listitem")).toHaveLength(6);
    expect(within(graph).getByText("Agent review")).toBeInTheDocument();
    expect(within(graph).getByText("You set direction")).toBeInTheDocument();
    expect(within(graph).getByText("You decide")).toBeInTheDocument();
    expect(screen.getByText("Orchestrate agents")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "ADLC" })).toBeInTheDocument();
    expect(figure).toHaveAttribute("data-motion", "entering");
    expect(screen.getByRole("button", { name: "Replay transition" }))
      .toHaveAttribute("aria-disabled", "true");
    expect(within(graph).getByText("Requirements").closest("[aria-hidden]"))
      .toHaveAttribute("aria-hidden", "true");
  });

  it("waits for the feedback signal to finish before enabling replay", () => {
    render(<Lifecycle />);
    const figure = screen.getByRole("figure");
    const graph = screen.getByRole("list", { name: "ADLC" });
    const stage = within(graph).getAllByRole("listitem")[0]!;

    endAnimation(stage, "adlc-node-morph");
    expect(figure).toHaveAttribute("data-motion", "entering");
    expect(screen.getByRole("button", { name: "Replay transition" }))
      .toHaveAttribute("aria-disabled", "true");

    completeTransition(figure);
    expect(figure).toHaveAttribute("data-motion", "complete");
    expect(screen.getByRole("button", { name: "Replay transition" }))
      .not.toHaveAttribute("aria-disabled", "true");
  });

  it("does not restart the entrance when its parent rerenders", () => {
    const { rerender } = render(<Lifecycle />);
    const figure = screen.getByRole("figure");
    const graph = screen.getByRole("list", { name: "ADLC" });

    rerender(<Lifecycle />);
    expect(screen.getByRole("list", { name: "ADLC" })).toBe(graph);
    expect(figure).toHaveAttribute("data-motion", "entering");

    completeTransition(figure);
    rerender(<Lifecycle />);
    expect(screen.getByRole("list", { name: "ADLC" })).toBe(graph);
    expect(figure).toHaveAttribute("data-motion", "complete");
    expect(window.matchMedia).toHaveBeenCalledTimes(1);
  });

  it.each([
    { locale: "en" as const, name: "Replay transition" },
    { locale: "zh-Hans" as const, name: "重播跃迁" },
  ])("replays from the keyboard in $locale without losing button focus", async ({ locale, name }) => {
    const user = userEvent.setup();
    render(<Lifecycle locale={locale} />);
    const figure = screen.getByRole("figure");
    const originalGraph = screen.getByRole("list", { name: "ADLC" });
    completeTransition(figure);
    const replay = screen.getByRole("button", { name });

    await user.tab();
    expect(replay).toHaveFocus();
    await user.keyboard("{Enter}");

    const replayingGraph = screen.getByRole("list", { name: "ADLC" });
    expect(figure).toHaveAttribute("data-motion", "entering");
    expect(replayingGraph).not.toBe(originalGraph);
    expect(screen.getByRole("button", { name })).toBe(replay);
    expect(replay).toHaveFocus();
    expect(replay).toHaveAttribute("aria-disabled", "true");
    // Native disabled buttons lose focus in Chromium during replay.
    expect(replay).not.toHaveAttribute("disabled");

    await user.keyboard("{Enter}");
    expect(screen.getByRole("list", { name: "ADLC" })).toBe(replayingGraph);
    expect(figure).toHaveAttribute("data-motion", "entering");
    expect(replay).toHaveFocus();

    completeTransition(figure);
    expect(figure).toHaveAttribute("data-motion", "complete");
    expect(replay).not.toHaveAttribute("aria-disabled", "true");
    expect(replay).toHaveFocus();
  });

  it("shows the final diagram immediately for reduced motion", () => {
    Object.defineProperty(media, "matches", { value: true });
    render(<Lifecycle locale="zh-Hans" />);

    expect(screen.getByRole("figure", { name: "从传统软件研发到智能体协作研发" }))
      .toHaveAttribute("data-motion", "complete");
    expect(screen.getByText("智能体评审")).toBeInTheDocument();
    expect(screen.getByText("你做决策")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重播跃迁" })).not.toBeInTheDocument();
  });

  it("ends active movement when reduced motion is enabled and removes its listener on unmount", () => {
    const { unmount } = render(<Lifecycle />);
    const figure = screen.getByRole("figure");
    Object.defineProperty(media, "matches", { value: true, configurable: true });
    act(() => changes.dispatchEvent(new Event("change")));
    expect(figure).toHaveAttribute("data-motion", "complete");
    expect(screen.queryByRole("button", { name: "Replay transition" })).not.toBeInTheDocument();

    Object.defineProperty(media, "matches", { value: false });
    act(() => changes.dispatchEvent(new Event("change")));
    expect(figure).toHaveAttribute("data-motion", "complete");
    expect(screen.getByRole("button", { name: "Replay transition" }))
      .not.toHaveAttribute("aria-disabled", "true");

    unmount();
    expect(media.removeEventListener).toHaveBeenCalledWith(
      "change",
      vi.mocked(media.addEventListener).mock.calls[0]![1],
    );
  });
});
