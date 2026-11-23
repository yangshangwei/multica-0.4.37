import { forwardRef, useImperativeHandle, useState } from "react";
import { act, fireEvent, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { buildIssueStatusCatalog } from "@multica/core/issue-statuses";
import type { InboxItem } from "@multica/core/types";
import { NavigationProvider, type NavigationAdapter } from "../../navigation";
import { renderWithI18n } from "../../test/i18n";
import { InboxList } from "./inbox-list";

// Model Virtuoso's mount boundary and deferred destination mount. Browser
// acceptance owns actual geometry; these tests exercise real row controls and
// focus ownership while the virtualizer removes and replaces their DOM nodes.
const virtual = vi.hoisted(() => ({
  scrollIntoView: vi.fn(),
  windowSize: Infinity,
  deferMount: false,
  mountPending: null as (() => void) | null,
}));

vi.mock("react-virtuoso", () => ({
  Virtuoso: forwardRef(function MockVirtuoso(
    { data, itemContent }: {
      data: InboxItem[];
      itemContent: (index: number, item: InboxItem) => React.ReactNode;
    },
    ref: React.Ref<unknown>,
  ) {
    const [start, setStart] = useState(0);
    useImperativeHandle(ref, () => ({
      scrollIntoView: (options: { index: number }) => {
        virtual.scrollIntoView(options);
        const mount = () => setStart(Math.floor(options.index / virtual.windowSize) * virtual.windowSize || 0);
        if (virtual.deferMount) virtual.mountPending = mount;
        else mount();
      },
    }));
    return <div>{data.slice(start, start + virtual.windowSize).map((item, index) => (
      <div key={item.id}>{itemContent(start + index, item)}</div>
    ))}</div>;
  }),
}));

vi.mock("@multica/core/issue-statuses/hooks", () => ({
  useIssueStatuses: () => buildIssueStatusCatalog(undefined),
}));
vi.mock("../../issues/components", () => ({ StatusIcon: () => null }));
vi.mock("../../issues/components/issue-agent-activity-indicator", () => ({ IssueAgentActivityIndicator: () => null }));
vi.mock("../../common/actor-avatar", () => ({ ActorAvatar: () => <span /> }));
vi.mock("./inbox-detail-label", () => ({ InboxDetailLabel: () => <span>New comment</span> }));
vi.mock("./inbox-row-menu", () => ({
  InboxRowMenu: () => <button type="button" onClick={(event) => event.stopPropagation()}>Notification actions</button>,
}));

function item(id: string, overrides: Partial<InboxItem> = {}): InboxItem {
  return {
    id, workspace_id: "workspace-1", recipient_type: "member", recipient_id: "member-1",
    actor_type: "agent", actor_id: "agent-1", type: "new_comment", severity: "info",
    issue_id: `issue-${id}`, title: `Issue ${id}`, body: null, issue_status: null,
    read: true, archived: false, created_at: "2026-06-15T08:00:00Z", details: null,
    ...overrides,
  };
}
const items = [item("a"), item("b"), item("c"), item("d")];
const adapter: NavigationAdapter = {
  push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/", hash: "",
  searchParams: new URLSearchParams(), getShareableUrl: (path) => path,
};

function renderList(selectedKey: string, onSelect = vi.fn(), data = items) {
  const onAction = vi.fn();
  function Harness() {
    const [selected, setSelected] = useState(selectedKey);
    return <NavigationProvider value={adapter}>
      <button type="button" onClick={() => setSelected("")}>Dismiss detail</button>
      <InboxList items={data} view="inbox" selectedKey={selected} archivedCount={0}
        onSelect={(next) => { onSelect(next); setSelected(next.issue_id ?? next.id); }}
        onAction={onAction} onOpenArchived={vi.fn()} />
    </NavigationProvider>;
  }
  const utils = renderWithI18n(<Harness />);
  const scroller = screen.getByRole("list", { name: "Inbox" });
  return { ...utils, scroller, onSelect, onAction };
}
const primary = (id: string) => screen.getByRole("button", { name: `Issue ${id}` });
function press(target: HTMLElement, key: string, init: KeyboardEventInit = {}) {
  return !fireEvent.keyDown(target, { key, ...init });
}

beforeEach(() => {
  virtual.scrollIntoView.mockClear();
  virtual.windowSize = Infinity;
  virtual.deferMount = false;
  virtual.mountPending = null;
});

describe("InboxList keyboard navigation", () => {
  it("moves current state and DOM focus with arrow keys", () => {
    const { onSelect } = renderList("issue-a");
    primary("a").focus();
    expect(press(primary("a"), "ArrowDown")).toBe(true);
    expect(onSelect).toHaveBeenLastCalledWith(items[1]);
    expect(primary("b")).toHaveFocus();
    expect(primary("b")).toHaveAttribute("aria-current", "true");
    expect(primary("a")).not.toHaveAttribute("aria-current");
    expect(press(primary("b"), "ArrowUp")).toBe(true);
    expect(primary("a")).toHaveFocus();
    expect(primary("a")).toHaveAttribute("aria-current", "true");
    expect(virtual.scrollIntoView).toHaveBeenCalledWith({ index: 1 });
  });

  it.each([["ArrowDown", "a"], ["ArrowUp", "d"]])("enters an unselected list with %s", (key, id) => {
    const { scroller } = renderList("");
    scroller.focus();
    press(scroller, key!);
    expect(primary(id!)).toHaveFocus();
    expect(primary(id!)).toHaveAttribute("aria-current", "true");
  });

  it("stops at the ends and retains row focus", () => {
    const { onSelect } = renderList("issue-d");
    primary("d").focus();
    expect(press(primary("d"), "ArrowDown")).toBe(true);
    expect(onSelect).not.toHaveBeenCalled();
    expect(primary("d")).toHaveFocus();
  });

  it("starts at the focused row after Tab, not the prior selection", () => {
    renderList("issue-a");
    primary("c").focus();
    press(primary("c"), "ArrowDown");
    expect(primary("d")).toHaveFocus();
    expect(primary("d")).toHaveAttribute("aria-current", "true");
  });

  it("ignores modified keys and IME composition", () => {
    const { onSelect } = renderList("issue-a");
    primary("a").focus();
    for (const modifier of [{ shiftKey: true }, { metaKey: true }, { ctrlKey: true }, { altKey: true }, { keyCode: 229 }]) {
      expect(press(primary("a"), "ArrowDown", modifier)).toBe(false);
    }
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("keeps archive, menu and text-field input independent", async () => {
    const { scroller, onSelect, onAction } = renderList("issue-a");
    const row = primary("b").closest('[role="listitem"]') as HTMLElement;
    const archive = within(row).getByRole("button", { name: "Archive" });
    const menu = within(row).getByRole("button", { name: "Notification actions" });
    for (const control of [archive, menu]) {
      control.focus();
      expect(press(control, "ArrowDown")).toBe(false);
      expect(control).toHaveFocus();
    }
    await userEvent.setup().click(archive);
    expect(onAction).toHaveBeenCalledWith("b");
    const input = document.createElement("input");
    scroller.appendChild(input);
    expect(press(input, "ArrowDown")).toBe(false);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("focuses the real row on click and leaves detail dismissal focus alone", async () => {
    renderList("");
    fireEvent.click(primary("b"));
    expect(primary("b")).toHaveFocus();
    expect(primary("b")).toHaveAttribute("aria-current", "true");
    const dismiss = screen.getByRole("button", { name: "Dismiss detail" });
    await userEvent.setup().click(dismiss);
    expect(dismiss).toHaveFocus();
    expect(primary("b")).not.toHaveAttribute("aria-current");
  });

  it("keeps notification-only identity and exposes virtual list positions", () => {
    renderList("notice", vi.fn(), [item("notice", { issue_id: null })]);
    const selection = primary("notice");
    expect(selection).toHaveAttribute("aria-current", "true");
    const row = selection.closest('[role="listitem"]');
    expect(row).toHaveAttribute("aria-posinset", "1");
    expect(row).toHaveAttribute("aria-setsize", "1");
  });

  it("mounts and focuses the next and previous rows across virtual boundaries", () => {
    virtual.windowSize = 2;
    renderList("issue-b");
    primary("b").focus();
    press(primary("b"), "ArrowDown");
    expect(screen.queryByRole("button", { name: "Issue b" })).not.toBeInTheDocument();
    expect(primary("c")).toHaveFocus();
    expect(primary("c")).toHaveAttribute("aria-current", "true");
    expect(primary("c").closest('[role="listitem"]')).toHaveAttribute("aria-posinset", "3");
    press(primary("c"), "ArrowUp");
    expect(primary("b")).toHaveFocus();
    expect(primary("b")).toHaveAttribute("aria-current", "true");
  });

  it("does not reclaim focus when a pending virtual row mounts after detail dismissal", async () => {
    virtual.windowSize = 2;
    virtual.deferMount = true;
    renderList("issue-b");
    primary("b").focus();
    press(primary("b"), "ArrowDown");
    const dismiss = screen.getByRole("button", { name: "Dismiss detail" });
    await userEvent.setup().click(dismiss);
    act(() => virtual.mountPending?.());
    expect(primary("c")).toBeInTheDocument();
    expect(dismiss).toHaveFocus();
    expect(primary("c")).not.toHaveAttribute("aria-current");
  });
});
