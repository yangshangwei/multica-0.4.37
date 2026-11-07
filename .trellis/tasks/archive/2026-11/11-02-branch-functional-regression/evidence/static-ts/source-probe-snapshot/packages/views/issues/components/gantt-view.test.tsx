import { createStore } from "zustand/vanilla";
import { describe, expect, it, vi, beforeAll, afterAll, beforeEach, afterEach } from "vitest";
import { act, fireEvent, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  type IssueViewState,
  viewStoreSlice,
} from "@multica/core/issues/stores/view-store";
import { ViewStoreProvider } from "@multica/core/issues/stores/view-store-context";
import type { Issue } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { IssueContextMenuProvider } from "../actions";

vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-query")>()),
  useQuery: () => ({ data: [] }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

// The row's start / due range lives in a tooltip. This test is about how the
// dates are formatted, not about hover timing, so render the content inline.
vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => children,
  TooltipTrigger: ({ render }: { render: React.ReactNode }) => render,
  TooltipContent: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("../../navigation", () => ({
  AppLink: ({
    children,
    href,
    newTabTitle: _newTabTitle,
    ...props
  }: {
    newTabTitle?: string;
  } & React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a>,
  useNavigation: () => ({ push: vi.fn(), pathname: "/issues" }),
  resolveClickIntent: () => "push",
  useIntentNavigate: () => () => {},
  NavigationProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return {
    ...actual,
    useWorkspaceSlug: () => "acme",
    useRequiredWorkspaceSlug: () => "acme",
    useWorkspacePaths: () => actual.paths.workspace("acme"),
  };
});

import { GanttView } from "./gantt-view";

const ISSUE = {
  id: "issue-1",
  identifier: "MUL-1",
  number: 1,
  title: "Ship the thing",
  description: "",
  status: "todo",
  priority: "medium",
  workspace_id: "workspace-1",
  project_id: null,
  parent_issue_id: null,
  assignee_id: null,
  assignee_type: null,
  creator_id: "user-1",
  creator_type: "member",
  labels: [],
  position: 0,
  stage: null,
  start_date: "2026-03-02",
  due_date: "2026-03-06",
  created_at: "2026-03-01T00:00:00Z",
  updated_at: "2026-03-01T00:00:00Z",
} as unknown as Issue;

function renderGantt(locale: "en" | "zh-Hans", issues: Issue[] = [ISSUE]) {
  const store = createStore<IssueViewState>()(viewStoreSlice);
  const result = renderWithI18n(
    <ViewStoreProvider store={store}>
      <IssueContextMenuProvider>
        <GanttView issues={issues} />
      </IssueContextMenuProvider>
    </ViewStoreProvider>,
    { locale },
  );
  return { ...result, store };
}

function mockViewport() {
  vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(900);
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(900);
  vi.spyOn(HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(360);
  vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(360);
}

describe("GanttView date localization", () => {
  beforeAll(() => {
    mockViewport();
    // The browser speaks English while the UI does not. Dates must follow the
    // UI language, not this — see useLocale in packages/views/i18n.
    vi.stubGlobal("navigator", {
      ...globalThis.navigator,
      language: "en-US",
      languages: ["en-US"],
    });
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-03-04T12:00:00Z"));
  });

  afterAll(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("formats axis dates in the UI language, not the browser's", () => {
    const { container, unmount } = renderGantt("zh-Hans");

    expect(screen.getByText("2026年3月")).toBeTruthy();
    // Nothing may fall back to the browser language.
    expect(container.textContent ?? "").not.toMatch(/Mar\b/);

    unmount();
  });

  it("formats the row's start / due range in the UI language", () => {
    const { unmount } = renderGantt("zh-Hans");

    const range = screen.getByText(/2026年3月2日/);
    expect(range.textContent).toContain("2026年3月6日");

    unmount();
  });

  it("still formats in English when the UI is English", () => {
    const { container } = renderGantt("en");

    expect(screen.getByText("Mar 2026")).toBeTruthy();
    expect(container.textContent ?? "").not.toMatch(/年/);
  });
});

describe("GanttView visible window", () => {
  let rowCount = 1;
  const originalScrollTo = HTMLElement.prototype.scrollTo;

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-03-04T12:00:00Z"));
    rowCount = 1;
    // Keep the real virtualizer. Only provide the viewport/layout APIs that
    // jsdom lacks, so scroll events must actually change the rendered window.
    mockViewport();
    vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockImplementation(
      () => 56 + rowCount * 36,
    );
    HTMLElement.prototype.scrollTo = function (options?: ScrollToOptions | number, y?: number) {
      const next = typeof options === "number" ? y ?? this.scrollTop : options?.top ?? this.scrollTop;
      if (next !== this.scrollTop) {
        this.scrollTop = next;
        this.dispatchEvent(new Event("scroll"));
      }
    };
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
    HTMLElement.prototype.scrollTo = originalScrollTo;
  });

  function manyIssues(count: number): Issue[] {
    rowCount = count;
    return Array.from({ length: count }, (_, index) => ({
      ...ISSUE,
      id: `issue-${index + 1}`,
      identifier: `MUL-${index + 1}`,
      title: `Scheduled issue ${index + 1}`,
      position: index,
    }));
  }

  it("bounds date DOM across a ten-year range and reaches the last dates at every zoom", () => {
    const { container } = renderGantt("en", [{
      ...ISSUE,
      start_date: "2020-01-01",
      due_date: "2030-12-31",
    }]);
    const scroller = container.querySelector<HTMLDivElement>(".overflow-auto")!;

    expect(container.querySelectorAll("*").length).toBeLessThan(750);
    expect(screen.queryByText("Dec 2030")).not.toBeInTheDocument();
    for (const zoom of ["Day", "Week", "Month"]) {
      fireEvent.click(screen.getByRole("button", { name: zoom }));
      const canvas = scroller.firstElementChild as HTMLElement;
      const width = Number.parseFloat(canvas.style.minWidth);
      expect(width).toBeGreaterThan(20_000);
      fireEvent.scroll(scroller, { target: { scrollLeft: width - 900 } });
      expect(screen.getByText("Dec 2030")).toBeInTheDocument();
      expect(container.querySelectorAll("*").length).toBeLessThan(750);
      fireEvent.scroll(scroller, { target: { scrollLeft: 0 } });
      expect(screen.getByText("Jan 2020")).toBeInTheDocument();
    }
  });

  it("keeps the UTC today column centered beyond the sticky issue labels when zoom changes", () => {
    const { container } = renderGantt("en", [{
      ...ISSUE,
      start_date: "2026-03-04",
      due_date: "2026-03-04",
    }]);
    const scroller = container.querySelector<HTMLDivElement>(".overflow-auto")!;
    for (const [zoom, width] of [["Day", 36], ["Week", 14], ["Month", 6]] as const) {
      fireEvent.click(screen.getByRole("button", { name: zoom }));
      const bar = screen.getByRole("link", { name: "MUL-1: Ship the thing" });
      expect(bar).toHaveStyle({ width: `${width}px` });
      expect(Number.parseFloat(bar.style.left) - scroller.scrollLeft).toBeCloseTo((900 - 320) / 2);
    }
  });

  it("bounds a thousand rows while scrolling and sorting still reaches every issue", () => {
    const issues = manyIssues(1000);
    const { container, store } = renderGantt("en", issues);
    const scroller = container.querySelector<HTMLDivElement>(".overflow-auto")!;
    const issueLink = (number: number) => screen.queryByRole("link", {
      name: new RegExp(`MUL-${number}\\s*Scheduled issue ${number}$`),
    });

    expect(issueLink(1)).toBeInTheDocument();
    expect(issueLink(1000)).not.toBeInTheDocument();
    expect(container.querySelectorAll("*").length).toBeLessThan(2000);
    fireEvent.scroll(scroller, { target: { scrollTop: 999 * 36 } });
    expect(issueLink(1000)).toBeInTheDocument();
    expect(issueLink(1)).not.toBeInTheDocument();
    expect(container.querySelectorAll("*").length).toBeLessThan(2000);

    act(() => store.setState({ sortBy: "position", sortDirection: "desc" }));
    fireEvent.scroll(scroller, { target: { scrollTop: 0 } });
    expect(issueLink(1000)).toBeInTheDocument();
    expect(issueLink(1)).not.toBeInTheDocument();
    expect(screen.getAllByRole("link").length).toBeLessThan(60);
    fireEvent.scroll(scroller, { target: { scrollTop: 999 * 36 } });
    expect(issueLink(1)).toBeInTheDocument();
  });

  it("retains a focused row and supports keyboard access beyond the mounted window", () => {
    const { container } = renderGantt("en", manyIssues(1000));
    const scroller = container.querySelector<HTMLDivElement>(".overflow-auto")!;
    const first = screen.getByRole("link", { name: /MUL-1\s*Scheduled issue 1$/ });
    act(() => first.focus());
    fireEvent.scroll(scroller, { target: { scrollTop: 500 * 36 } });
    expect(first).toHaveFocus();
    expect(container.querySelectorAll("*").length).toBeLessThan(2000);

    fireEvent.keyDown(first, { key: "End" });
    const last = screen.getByRole("link", { name: /MUL-1000\s*Scheduled issue 1000$/ });
    expect(last).toHaveFocus();
    expect(scroller.scrollTop).toBeGreaterThan(900 * 36);
    fireEvent.keyDown(last, { key: "Home" });
    expect(screen.getByRole("link", { name: /MUL-1\s*Scheduled issue 1$/ })).toHaveFocus();
  });

  it("continues Tab and Shift+Tab across the row window", async () => {
    const user = userEvent.setup();
    renderGantt("en", manyIssues(100));
    const lastMountedRow = screen.getAllByRole("listitem").at(-1)!;
    const lastPosition = Number(lastMountedRow.getAttribute("aria-posinset"));
    const lastLink = within(lastMountedRow).getAllByRole("link").at(-1)!;
    act(() => lastLink.focus());

    await user.tab();
    const nextPosition = lastPosition + 1;
    expect(screen.getByRole("link", {
      name: new RegExp(`MUL-${nextPosition}\\s*Scheduled issue ${nextPosition}$`),
    })).toHaveFocus();
    await user.tab({ shift: true });
    expect(lastLink).toHaveFocus();
  });

  it("focuses Home on an already mounted first row without leaving a pending focus request", async () => {
    const user = userEvent.setup();
    renderGantt("en", manyIssues(100));
    const firstRow = screen.getAllByRole("listitem")[0]!;
    const links = within(firstRow).getAllByRole("link");
    act(() => links.at(-1)!.focus());

    fireEvent.keyDown(links.at(-1)!, { key: "Home" });
    expect(links[0]).toHaveFocus();
    await user.click(screen.getByRole("button", { name: "Month" }));
    expect(screen.getByRole("button", { name: "Month" })).toHaveFocus();
  });
});
