import { useEffect } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { defaultScheduler, notifyManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import type { CollaborationNotification, CollaborationNotificationView } from "@multica/core/collaboration";
import { accountPath, type AccountDestination, type AccountNotificationsDestination } from "@multica/core/paths";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { ScrollRestorationProvider, type ScrollRestorationAdapter } from "../platform/scroll-restoration";
import { renderWithI18n } from "../test/i18n";
import { AccountWorkbenchPage } from "./account-workbench-page";

// Core owns wire/filter matrices in api/collaboration-notifications.test.ts;
// this suite owns page wiring, notification identity, actions and recovery.
const viewerId = "00000000-0000-4000-8000-000000000001";
const issueId = "00000000-0000-4000-8000-000000000002";
const firstId = "00000000-0000-4000-8000-000000000003";
const secondId = "00000000-0000-4000-8000-000000000004";
const commentId = "00000000-0000-4000-8000-000000000005";
const secondCommentId = "00000000-0000-4000-8000-000000000006";
const runId = "00000000-0000-4000-8000-000000000007";
const bodyId = "00000000-0000-4000-8000-000000000008";
const timestamp = "2026-11-21T10:00:00Z";
const detailMount = vi.hoisted(() => vi.fn());
const realtime = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/auth", () => {
  const state = { status: "authenticated", user: { id: "00000000-0000-4000-8000-000000000001", name: "Viewer" } };
  return { useAuthStore: Object.assign((selector: (value: typeof state) => unknown) => selector(state), { getState: () => state }) };
});
vi.mock("@multica/core/realtime", async (original) => ({ ...await original<typeof import("@multica/core/realtime")>(), useCollaborationRealtime: realtime }));
vi.mock("./collaboration-detail", () => ({ CollaborationDetail: ({ destination }: { destination: AccountDestination }) => {
  useEffect(() => { detailMount(destination.issueId); }, [destination.issueId]);
  return <div data-testid="collaboration-preview"><p>{destination.issueId}</p><p data-testid="preview-target">{destination.runId ?? destination.commentId ?? destination.anchor ?? "issue"}</p><input aria-label="Preview draft" defaultValue="" /></div>;
} }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); vi.unstubAllGlobals(); vi.clearAllMocks(); });

function notification(overrides: Partial<CollaborationNotification> = {}): CollaborationNotification {
  return { id: firstId, issue_id: issueId, event_seq: "10", kind: "assigned", is_mention: false,
    source_kind: "issue", source_id: null, issue_title: "Shared issue", can_open: true,
    created_at: timestamp, read_at: null, archived_at: null, ...overrides };
}
function destination(overrides: Partial<AccountNotificationsDestination> = {}): AccountNotificationsDestination {
  return { view: "notifications", notificationView: "unread", notificationId: null, issueId: null, commentId: null, ...overrides };
}
function page(items: CollaborationNotification[], view: CollaborationNotificationView = "unread", nextCursor: string | null = null, unreadCount = 1) {
  return new Response(JSON.stringify({ items, view, unread_count: unreadCount, next_cursor: nextCursor }));
}
function mount(initial = destination(), locale: "en" | "zh-Hans" = "en", restoration: ScrollRestorationAdapter = { get: () => undefined }) {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const push = vi.fn();
  const content = (current: AccountNotificationsDestination, intent: Partial<NavigationAdapter> = {}) => {
    const url = new URL(accountPath(current), "https://server.example");
    const nav: NavigationAdapter = { pathname: url.pathname, searchParams: url.searchParams, hash: url.hash,
      push, replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => path, ...intent };
    return <QueryClientProvider client={client}><NavigationProvider value={nav}><ScrollRestorationProvider adapter={restoration}>
      <AccountWorkbenchPage destination={current} navigationInSidebar />
    </ScrollRestorationProvider></NavigationProvider></QueryClientProvider>;
  };
  const rendered = renderWithI18n(content(initial), { locale });
  return { ...rendered, client, push, navigate: (current: AccountNotificationsDestination, intent: Partial<NavigationAdapter> = {}) => rendered.rerender(content(current, intent)) };
}
function row(id: string) {
  const element = document.querySelector<HTMLElement>(`[data-notification-id="${id}"]`);
  if (!element) throw new Error(`Missing notification row ${id}`);
  return within(element);
}

it("dispatches the unread page with named filters and an unknown loading count", async () => {
  let resolve!: (response: Response) => void;
  const fetch = vi.fn((_input: string) => new Promise<Response>((ready) => { resolve = ready; }));
  vi.stubGlobal("fetch", fetch);
  mount();
  expect(screen.getByRole("heading", { name: "Collaboration notifications" })).toBeInTheDocument();
  const filters = screen.getByRole("navigation", { name: "Notification views" });
  expect(within(filters).getByRole("link", { name: "Unread" })).toHaveAttribute("aria-current", "page");
  expect(within(filters).getByRole("link", { name: "All" })).toHaveAttribute("href", "/account?view=notifications&filter=all");
  expect(within(filters).getByRole("link", { name: "Archived" })).toHaveAttribute("href", "/account?view=notifications&filter=archived");
  expect(screen.getByRole("status", { name: "Loading notifications..." })).toBeInTheDocument();
  expect(screen.queryByText("0 unread")).not.toBeInTheDocument();
  expect(screen.queryByText("No unread notifications.")).not.toBeInTheDocument();
  await act(async () => resolve(page([notification()])));
  expect(await screen.findByRole("link", { name: "Shared issue" })).toBeInTheDocument();
  expect(screen.getByText("1 unread")).toBeInTheDocument();
  expect(fetch.mock.calls.every(([url]) => String(url).includes("/api/me/collaboration-notifications"))).toBe(true);
});

it.each([
  ["unread", "No unread notifications."],
  ["all", "No notifications to review."],
  ["archived", "No archived notifications."],
] as const)("shows the complete %s empty state while preserving filter navigation", async (view, empty) => {
  vi.stubGlobal("fetch", vi.fn(async () => page([], view, null, 0)));
  mount(destination({ notificationView: view }));
  expect(await screen.findByText(empty)).toBeInTheDocument();
  expect(screen.getByRole("navigation", { name: "Notification views" })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
});

it.each(["unread", "all", "archived"] as const)("opens the exact %s notification from its description, date and card padding", async (notificationView) => {
  const item = notification({ source_kind: "comment", source_id: commentId,
    read_at: notificationView === "all" ? timestamp : null, archived_at: notificationView === "archived" ? timestamp : null });
  const fetch = vi.fn(async (_input: string, _options?: RequestInit) => page([item], notificationView));
  vi.stubGlobal("fetch", fetch);
  const view = mount(destination({ notificationView }));
  const title = await screen.findByRole("link", { name: "Shared issue" });
  const card = title.closest("li")!;
  const href = `/account?view=notifications&filter=${notificationView}&notification=${firstId}&issue=${issueId}#comment-${commentId}`;
  for (const target of [card.querySelector("p")!, card.querySelector("time")!, card]) {
    await userEvent.click(target);
    expect(view.push).toHaveBeenCalledExactlyOnceWith(href);
    view.push.mockClear();
  }
  expect(fetch.mock.calls.every(([, options]) => options?.method !== "POST")).toBe(true);
});

it("preserves title middle-click behavior and isolates notification action middle clicks", async () => {
  const fetch = vi.fn(async (_input: string, _options?: RequestInit) => page([notification()]));
  vi.stubGlobal("fetch", fetch);
  const view = mount();
  const title = await screen.findByRole("link", { name: "Shared issue" });
  const open = vi.spyOn(window, "open").mockReturnValue(null);
  try {
    const titleClick = new MouseEvent("auxclick", { bubbles: true, button: 1, cancelable: true });
    fireEvent(title, titleClick);
    expect(titleClick.defaultPrevented).toBe(false);
    for (const action of ["Mark read", "Archive"]) {
      fireEvent(row(firstId).getByRole("button", { name: action }), new MouseEvent("auxclick", { bubbles: true, button: 1, cancelable: true }));
    }
    expect(open).not.toHaveBeenCalled();
    expect(view.push).not.toHaveBeenCalled();
    expect(fetch.mock.calls.every(([, options]) => options?.method !== "POST")).toBe(true);
  } finally { open.mockRestore(); }
});

it("retries a failed later page without losing loaded rows or calling a write", async () => {
  let laterReads = 0;
  const fetch = vi.fn(async (input: string, _options?: RequestInit) => {
    if (new URL(input).searchParams.has("cursor")) {
      laterReads += 1;
      return laterReads === 1 ? new Response("{}", { status: 503 }) : page([notification({ id: secondId, issue_title: "Older notification", event_seq: "9" })]);
    }
    return page([notification()], "unread", "older-page");
  });
  vi.stubGlobal("fetch", fetch);
  mount();
  expect(await screen.findByRole("link", { name: "Shared issue" })).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Load more" }));
  expect(await screen.findByText("Could not load collaboration. Try again.")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Shared issue" })).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
  expect(await screen.findByRole("link", { name: "Older notification" })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
  expect(laterReads).toBe(2);
  expect(fetch.mock.calls.every(([, options]) => options?.method !== "POST")).toBe(true);
});

it("keeps the list and same-issue draft mounted across exact notification selections", async () => {
  const items = [notification({ source_kind: "comment", source_id: commentId, issue_title: "First comment" }),
    notification({ id: secondId, event_seq: "9", source_kind: "comment", source_id: secondCommentId, issue_title: "Second comment" }),
    notification({ id: runId, event_seq: "8", kind: "ai_failed", run_id: runId, issue_title: "Recorded run" }),
    notification({ id: bodyId, event_seq: "7", source_kind: "body", issue_title: "Issue requirements" })];
  vi.stubGlobal("fetch", vi.fn(async () => page(items, "unread", null, 4)));
  const view = mount();
  const first = await screen.findByRole("link", { name: "First comment" });
  const list = document.querySelector<HTMLElement>("[data-tab-scroll-root=account-notifications-list]")!;
  list.scrollTop = 144;
  expect(first).toHaveAttribute("href", `/account?view=notifications&filter=unread&notification=${firstId}&issue=${issueId}#comment-${commentId}`);
  expect(screen.getByRole("link", { name: "Recorded run" })).toHaveAttribute("href", `/account?view=notifications&filter=unread&notification=${runId}&issue=${issueId}#run-${runId}`);
  expect(screen.getByRole("link", { name: "Issue requirements" })).toHaveAttribute("href", `/account?view=notifications&filter=unread&notification=${bodyId}&issue=${issueId}#description`);
  await userEvent.click(first);
  expect(view.push).toHaveBeenCalledExactlyOnceWith(`/account?view=notifications&filter=unread&notification=${firstId}&issue=${issueId}#comment-${commentId}`);
  view.navigate(destination({ notificationId: firstId, issueId, commentId }));
  const draft = screen.getByRole("textbox", { name: "Preview draft" });
  await userEvent.type(draft, "Keep this draft");
  await userEvent.click(screen.getByRole("link", { name: "Second comment" }));
  expect(view.push).toHaveBeenCalledTimes(2);
  view.navigate(destination({ notificationId: secondId, issueId, commentId: secondCommentId }));
  expect(document.querySelector("[data-tab-scroll-root=account-notifications-list]")).toBe(list);
  expect(list.scrollTop).toBe(144);
  expect(screen.getByRole("textbox", { name: "Preview draft" })).toBe(draft);
  expect(draft).toHaveValue("Keep this draft");
  expect(screen.getByTestId("preview-target")).toHaveTextContent(secondCommentId);
  expect(screen.getByRole("link", { name: "Second comment" })).toHaveAttribute("aria-current", "true");
  expect(detailMount).toHaveBeenCalledTimes(1);
  expect(screen.getByRole("link", { name: "Back to notifications" })).toHaveAttribute("href", "/account?view=notifications&filter=unread");
  await userEvent.click(screen.getByRole("link", { name: "All" }));
  expect(view.push).toHaveBeenLastCalledWith("/account?view=notifications&filter=all");
});

it("restores the notification list after data arrives without consuming it again on selection", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => page([notification()])));
  const get = vi.fn((key: string) => key === "account-notifications-list" ? { top: 88, height: 1200 } : undefined);
  const view = mount(destination(), "en", { get });
  await screen.findByRole("link", { name: "Shared issue" });
  const list = document.querySelector<HTMLElement>("[data-tab-scroll-root=account-notifications-list]")!;
  expect(list.scrollTop).toBe(88);
  list.scrollTop = 128;
  view.navigate(destination({ notificationId: firstId, issueId }));
  expect(list.scrollTop).toBe(128);
  expect(get.mock.calls.filter(([key]) => key === "account-notifications-list")).toHaveLength(1);
});

it("preserves selected-detail scrolling during unrelated notification reads", async () => {
  const fetch = vi.fn(async () => page([notification()]));
  vi.stubGlobal("fetch", fetch);
  mount(destination({ notificationId: firstId, issueId }), "en", {
    get: (key) => key === "account-detail" ? { top: 42, height: 1200 } : undefined,
  });
  await screen.findByRole("link", { name: "Shared issue" });
  const detail = document.querySelector<HTMLElement>("[data-tab-scroll-root=account-detail]")!;
  detail.scrollTop = 190;
  await userEvent.click(screen.getByRole("button", { name: "Refresh notifications" }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
  await waitFor(() => expect(screen.getByRole("button", { name: "Refresh notifications" })).toBeEnabled());
  expect(document.querySelector("[data-tab-scroll-root=account-detail]")).toBe(detail);
  expect(detail.scrollTop).toBe(190);
});

it("consumes each matching history list offset once with a stable adapter, including zero", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => page([notification(), notification({ id: secondId, event_seq: "9" })])));
  let active = firstId;
  const offsets = new Map([[firstId, 88], [secondId, 214]]);
  const view = mount(destination({ notificationId: firstId, issueId }), "en", {
    get: (key) => key === "account-notifications-list" ? { top: offsets.get(active) ?? 0, height: 1800 } : undefined,
  });
  await screen.findAllByRole("link", { name: "Shared issue" });
  const list = document.querySelector<HTMLElement>("[data-tab-scroll-root=account-notifications-list]")!;
  expect(list.scrollTop).toBe(88);
  list.scrollTop = 163;
  active = secondId;
  view.navigate(destination({ notificationId: secondId, issueId }), { navigationKey: 1 });
  expect(list.scrollTop).toBe(163);
  active = firstId;
  view.navigate(destination({ notificationId: firstId, issueId }), { navigationKey: 2, isHistoryTraversal: true });
  expect(list.scrollTop).toBe(88);
  list.scrollTop = 179;
  await userEvent.click(screen.getByRole("button", { name: "Refresh notifications" }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Refresh notifications" })).toBeEnabled());
  expect(list.scrollTop).toBe(179);
  active = secondId;
  view.navigate(destination({ notificationId: secondId, issueId }), { navigationKey: 3, isHistoryTraversal: true });
  expect(list.scrollTop).toBe(214);
  offsets.set(firstId, 0);
  active = firstId;
  view.navigate(destination({ notificationId: firstId, issueId }), { navigationKey: 4, isHistoryTraversal: true });
  expect(list.scrollTop).toBe(0);
  expect(document.querySelector("[data-tab-scroll-root=account-notifications-list]")).toBe(list);
});

it("opens an exact target beyond the first page and preserves its preview when the row loads", async () => {
  vi.stubGlobal("fetch", vi.fn(async (input: string) => new URL(input).searchParams.has("cursor")
    ? page([notification({ id: secondId, event_seq: "9", kind: "ai_failed", run_id: runId, issue_title: "Older run" })])
    : page([notification()], "unread", "older-page")));
  mount(destination({ notificationId: secondId, issueId, runId }));
  await screen.findByRole("link", { name: "Shared issue" });
  const preview = screen.getByRole("textbox", { name: "Preview draft" });
  await userEvent.type(preview, "Keep the deep-linked draft");
  expect(screen.getByTestId("preview-target")).toHaveTextContent(runId);
  expect(screen.queryByText("Issue unavailable")).not.toBeInTheDocument();
  expect(screen.queryByText("This notification is outside the current view. You can continue viewing its issue here.")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Load more" }));
  expect(await screen.findByRole("link", { name: "Older run" })).toHaveAttribute("aria-current", "true");
  expect(screen.getByRole("textbox", { name: "Preview draft" })).toBe(preview);
  expect(preview).toHaveValue("Keep the deep-linked draft");
  expect(detailMount).toHaveBeenCalledTimes(1);
});

it("refuses a known notification's mismatched issue route before reading or subscribing", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => page([notification()])));
  const view = mount(destination({ notificationId: firstId, issueId: viewerId }));
  await screen.findByRole("link", { name: "Shared issue" });
  expect(screen.queryByTestId("collaboration-preview")).not.toBeInTheDocument();
  expect(realtime.mock.calls.every(([, selected]) => selected === null)).toBe(true);
  view.navigate(destination({ notificationId: firstId, issueId }));
  expect(screen.getByTestId("collaboration-preview")).toBeInTheDocument();
  expect(realtime).toHaveBeenLastCalledWith(expect.anything(), issueId);
});

it("keeps an empty intermediate page recoverable rather than declaring the view empty", async () => {
  vi.stubGlobal("fetch", vi.fn(async (input: string) => new URL(input).searchParams.has("cursor")
    ? page([notification()]) : page([], "unread", "older-page")));
  mount();
  const more = await screen.findByRole("button", { name: "Load more" });
  expect(screen.queryByText("No unread notifications.")).not.toBeInTheDocument();
  await userEvent.click(more);
  expect(await screen.findByRole("link", { name: "Shared issue" })).toBeInTheDocument();
});

it("keeps removed and unavailable notifications content-free even with a stale issue target", async () => {
  const items = [notification({ can_open: false, issue_title: null }),
    notification({ id: secondId, event_seq: "9", kind: "removed", can_open: false, issue_title: null }),
    notification({ id: runId, event_seq: "8", kind: "future_event", issue_title: "Other activity" })];
  const fetch = vi.fn(async (_input: string) => page(items, "unread", null, 3));
  vi.stubGlobal("fetch", fetch);
  const view = mount(destination({ notificationId: firstId, issueId, commentId }));
  await screen.findByRole("link", { name: "Other activity" });
  expect(row(firstId).getByRole("link", { name: "Issue unavailable" })).toHaveAttribute("href", `/account?view=notifications&filter=unread&notification=${firstId}`);
  expect(row(secondId).getByText("You were removed from this collaboration issue.")).toBeInTheDocument();
  expect(row(secondId).getByRole("button", { name: "Archive" })).toBeEnabled();
  expect(row(runId).getByText("Activity recorded")).toBeInTheDocument();
  expect(screen.queryByText("future_event")).not.toBeInTheDocument();
  expect(screen.queryByTestId("collaboration-preview")).not.toBeInTheDocument();
  expect(detailMount).not.toHaveBeenCalled();
  expect(realtime.mock.calls.every(([, selected]) => selected === null)).toBe(true);
  await userEvent.click(row(firstId).getByRole("link", { name: "Issue unavailable" }).closest("li")!);
  expect(view.push).toHaveBeenCalledExactlyOnceWith(`/account?view=notifications&filter=unread&notification=${firstId}`);
  view.navigate(destination({ notificationId: secondId, issueId, runId }));
  expect(screen.queryByTestId("collaboration-preview")).not.toBeInTheDocument();
  expect(fetch.mock.calls.every(([url]) => String(url).includes("/collaboration-notifications"))).toBe(true);
});

it("awaits read confirmation, retains the selected preview and recovers focus when its unread row leaves", async () => {
  let readAt: string | null = null;
  let confirm!: () => void;
  const fetch = vi.fn(async (_input: string, options?: RequestInit) => {
    if (options?.method === "POST") return new Promise<Response>((ready) => { confirm = () => { readAt = timestamp; ready(new Response(JSON.stringify({ id: firstId, read_at: readAt, archived_at: null }))); }; });
    return page([...(readAt ? [] : [notification()]), notification({ id: secondId, event_seq: "9", issue_title: "Still unread" })], "unread", null, readAt ? 1 : 2);
  });
  vi.stubGlobal("fetch", fetch);
  const view = mount(destination({ notificationId: firstId, issueId }));
  await screen.findByRole("link", { name: "Shared issue" });
  await userEvent.click(row(firstId).getByRole("button", { name: "Mark read" }));
  expect(row(firstId).getByText("Marking read...")).toBeInTheDocument();
  expect(row(firstId).getByRole("button", { name: "Archive" })).toBeDisabled();
  expect(row(secondId).getByRole("button", { name: "Mark read" })).toBeEnabled();
  expect(screen.getByRole("link", { name: "Shared issue" })).toBeInTheDocument();
  await act(async () => confirm());
  await waitFor(() => expect(screen.queryByRole("link", { name: "Shared issue" })).not.toBeInTheDocument());
  expect(screen.getByTestId("collaboration-preview")).toBeInTheDocument();
  expect(screen.getByText("This notification is outside the current view. You can continue viewing its issue here.")).toBeInTheDocument();
  expect(screen.getByText("1 unread")).toBeInTheDocument();
  await waitFor(() => expect(document.querySelector("[data-tab-scroll-root=account-notifications-list]")).toHaveFocus());
  expect(fetch.mock.calls.filter(([, options]) => options?.method === "POST")).toHaveLength(1);
  expect(view.push).not.toHaveBeenCalled();
});

it("recovers to a stable list before a confirmed archive's final Query projection removes its row", async () => {
  let confirm!: () => void;
  let publishProjection!: () => void;
  let reads = 0;
  const fetch = vi.fn(async (_input: string, options?: RequestInit) => {
    if (options?.method === "POST") return new Promise<Response>((ready) => {
      confirm = () => ready(new Response(JSON.stringify({ id: firstId, read_at: null, archived_at: timestamp })));
    });
    reads += 1;
    return reads === 1 ? page([notification()], "all") : new Promise<Response>((ready) => {
      publishProjection = () => ready(page([], "all", null, 0));
    });
  });
  vi.stubGlobal("fetch", fetch);
  mount(destination({ notificationView: "all", notificationId: firstId, issueId }));
  const link = await screen.findByRole("link", { name: "Shared issue" });
  const originalRow = link.closest("li")!;
  const list = document.querySelector<HTMLElement>("[data-tab-scroll-root=account-notifications-list]")!;
  const archive = row(firstId).getByRole("button", { name: "Archive" });
  archive.focus();
  expect(archive).toHaveFocus();
  await userEvent.keyboard("{Enter}");
  await act(async () => confirm());
  await waitFor(() => expect(row(firstId).queryByRole("button", { name: "Archive" })).not.toBeInTheDocument());
  expect(originalRow.isConnected).toBe(true);
  expect(document.body).toHaveFocus();
  // The confirmed refetch can resolve before React commits its Query observer update.
  const notifications: Array<() => void> = [];
  notifyManager.setScheduler((callback) => notifications.push(callback));
  try {
    await act(async () => publishProjection());
    await waitFor(() => expect(document.body).not.toHaveFocus());
    expect(originalRow.isConnected).toBe(true);
    await act(async () => notifications.splice(0).forEach((callback) => callback()));
    await waitFor(() => expect(originalRow.isConnected).toBe(false));
    expect(list).toHaveFocus();
    expect(screen.getByTestId("collaboration-preview")).toBeInTheDocument();
    expect(fetch.mock.calls.filter(([, options]) => options?.method === "POST")).toHaveLength(1);
  } finally {
    notifyManager.setScheduler(defaultScheduler);
    await act(async () => notifications.splice(0).forEach((callback) => callback()));
  }
});

it("keeps stable recovery after a confirmed archive when refetch fails, without retrying the write", async () => {
  let archived = false;
  let failRead = true;
  const fetch = vi.fn(async (_input: string, options?: RequestInit) => {
    if (options?.method === "POST") {
      archived = true;
      return new Response(JSON.stringify({ id: firstId, read_at: null, archived_at: timestamp }));
    }
    if (archived && failRead) return new Response("{}", { status: 503 });
    return page(archived ? [] : [notification()], "all", null, archived ? 0 : 1);
  });
  vi.stubGlobal("fetch", fetch);
  mount(destination({ notificationView: "all", notificationId: firstId, issueId }));
  await screen.findByRole("link", { name: "Shared issue" });
  const archive = row(firstId).getByRole("button", { name: "Archive" });
  archive.focus();
  await userEvent.keyboard("{Enter}");
  await screen.findByText("Could not load collaboration. Try again.");
  expect(row(firstId).queryByRole("button", { name: "Archive" })).not.toBeInTheDocument();
  expect(row(firstId).getByRole("button", { name: "Mark read" })).toBeInTheDocument();
  expect(screen.queryByText("Could not archive this notification. Try again.")).not.toBeInTheDocument();
  const list = document.querySelector<HTMLElement>("[data-tab-scroll-root=account-notifications-list]")!;
  await waitFor(() => expect(list).toHaveFocus());
  failRead = false;
  await userEvent.click(screen.getByRole("button", { name: "Refresh notifications" }));
  await waitFor(() => expect(screen.queryByRole("link", { name: "Shared issue" })).not.toBeInTheDocument());
  expect(fetch.mock.calls.filter(([, options]) => options?.method === "POST")).toHaveLength(1);
});

it("preserves a live preview control's focus while a confirmed archive removes the notification", async () => {
  let archived = false;
  let confirm!: () => void;
  vi.stubGlobal("fetch", vi.fn(async (_input: string, options?: RequestInit) => {
    if (options?.method === "POST") return new Promise<Response>((ready) => {
      confirm = () => { archived = true; ready(new Response(JSON.stringify({ id: firstId, read_at: null, archived_at: timestamp }))); };
    });
    return page(archived ? [] : [notification()], "all", null, archived ? 0 : 1);
  }));
  mount(destination({ notificationView: "all", notificationId: firstId, issueId }));
  await screen.findByRole("link", { name: "Shared issue" });
  const archive = row(firstId).getByRole("button", { name: "Archive" });
  archive.focus();
  await userEvent.keyboard("{Enter}");
  const draft = screen.getByRole("textbox", { name: "Preview draft" });
  await userEvent.type(draft, "Keep editing while the notification settles");
  await act(async () => confirm());
  await waitFor(() => expect(screen.queryByRole("link", { name: "Shared issue" })).not.toBeInTheDocument());
  expect(draft).toHaveFocus();
  expect(draft).toHaveValue("Keep editing while the notification settles");
});

it("recovers within the surviving All row after confirmed read", async () => {
  let readAt: string | null = null;
  vi.stubGlobal("fetch", vi.fn(async (_input: string, options?: RequestInit) => {
    if (options?.method === "POST") {
      readAt = timestamp;
      return new Response(JSON.stringify({ id: firstId, read_at: readAt, archived_at: null }));
    }
    return page([notification({ read_at: readAt })], "all", null, readAt ? 0 : 1);
  }));
  mount(destination({ notificationView: "all", notificationId: firstId, issueId }));
  const link = await screen.findByRole("link", { name: "Shared issue" });
  const read = row(firstId).getByRole("button", { name: "Mark read" });
  read.focus();
  await userEvent.keyboard("{Enter}");
  await waitFor(() => expect(row(firstId).queryByRole("button", { name: "Mark read" })).not.toBeInTheDocument());
  await waitFor(() => expect(link).toHaveFocus());
  expect(row(firstId).getByRole("button", { name: "Archive" })).toBeEnabled();
});

it.each([
  ["Mark read", "Could not mark this notification as read. Try again.", "Retry marking read"],
  ["Archive", "Could not archive this notification. Try again.", "Retry archiving"],
] as const)("keeps a failed %s visible and requires an explicit write retry after query recovery", async (action, error, retry) => {
  let failRead = false;
  let writes = 0;
  const fetch = vi.fn(async (_input: string, options?: RequestInit) => {
    if (options?.method === "POST") { writes += 1; return new Response("{}", { status: 503 }); }
    return failRead ? new Response("{}", { status: 503 }) : page([notification()]);
  });
  vi.stubGlobal("fetch", fetch);
  const view = mount();
  await screen.findByRole("link", { name: "Shared issue" });
  await userEvent.click(row(firstId).getByRole("button", { name: action }));
  expect(await screen.findByText(error)).toBeInTheDocument();
  failRead = true;
  await userEvent.click(screen.getByRole("button", { name: "Refresh notifications" }));
  expect(await screen.findByText("Could not load collaboration. Try again.")).toBeInTheDocument();
  failRead = false;
  await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await waitFor(() => expect(screen.queryByText("Could not load collaboration. Try again.")).not.toBeInTheDocument());
  expect(screen.getByText(error)).toBeInTheDocument();
  expect(writes).toBe(1);
  const open = vi.spyOn(window, "open").mockReturnValue(null);
  try {
    fireEvent(row(firstId).getByRole("button", { name: retry }), new MouseEvent("auxclick", { bubbles: true, button: 1, cancelable: true }));
    expect(open).not.toHaveBeenCalled();
    expect(writes).toBe(1);
  } finally { open.mockRestore(); }
  await userEvent.click(row(firstId).getByRole("button", { name: retry }));
  await waitFor(() => expect(writes).toBe(2));
  expect(view.push).not.toHaveBeenCalled();
});

it("awaits archive confirmation without marking read and keeps the archived unread fact", async () => {
  let archivedAt: string | null = null;
  let confirm!: () => void;
  const fetch = vi.fn(async (input: string, options?: RequestInit) => {
    if (options?.method === "POST") return new Promise<Response>((ready) => { confirm = () => { archivedAt = timestamp; ready(new Response(JSON.stringify({ id: firstId, read_at: null, archived_at: archivedAt }))); }; });
    const view = new URL(input).searchParams.get("view") === "archived" ? "archived" : "all";
    return page(view === "all" ? archivedAt ? [] : [notification()] : [notification({ archived_at: archivedAt })], view, null, archivedAt ? 0 : 1);
  });
  vi.stubGlobal("fetch", fetch);
  const view = mount(destination({ notificationView: "all", notificationId: firstId, issueId }));
  await screen.findByRole("link", { name: "Shared issue" });
  await userEvent.click(row(firstId).getByRole("button", { name: "Archive" }));
  expect(row(firstId).getByText("Archiving...")).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Shared issue" })).toBeInTheDocument();
  await act(async () => confirm());
  await waitFor(() => expect(screen.queryByRole("link", { name: "Shared issue" })).not.toBeInTheDocument());
  expect(screen.getByTestId("collaboration-preview")).toBeInTheDocument();
  view.navigate(destination({ notificationView: "archived" }));
  await screen.findByRole("link", { name: "Shared issue" });
  expect(row(firstId).getByText("Unread", { selector: ".sr-only" })).toBeInTheDocument();
  expect(row(firstId).getByRole("button", { name: "Mark read" })).toBeEnabled();
  expect(row(firstId).queryByRole("button", { name: "Archive" })).not.toBeInTheDocument();
  expect(fetch.mock.calls.filter(([, options]) => options?.method === "POST").map(([input]) => new URL(input).pathname)).toEqual([`/api/me/collaboration-notifications/${firstId}/archive`]);
  expect(view.push).not.toHaveBeenCalled();
});

it("shows an unavailable filtered read instead of a false empty view on an older server", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ items: [], unread_count: 0, next_cursor: null }))));
  mount();
  expect(await screen.findByText("This collaboration response is unavailable. Refresh to check again.")).toBeInTheDocument();
  expect(screen.queryByText("No unread notifications.")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Refresh" })).toBeEnabled();
  expect(screen.getByRole("navigation", { name: "Notification views" })).toBeInTheDocument();
});

it("localizes recipient assignment, filters, timestamps and the list-return action", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => page([notification({ issue_title: "产品派单给开发" })])));
  mount(destination({ notificationId: firstId, issueId }), "zh-Hans");
  expect(await screen.findByRole("heading", { name: "协作通知" })).toBeInTheDocument();
  expect(screen.getByRole("navigation", { name: "通知视图" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "全部" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "已归档" })).toBeInTheDocument();
  expect(await screen.findByText("任务已分配给你")).toBeInTheDocument();
  expect(screen.queryByText("已选择自己")).not.toBeInTheDocument();
  const time = document.querySelector("time")!;
  expect(time).toHaveAttribute("dateTime", timestamp);
  expect(time).toHaveAttribute("title", new Date(timestamp).toLocaleString("zh-Hans"));
  expect(screen.getByRole("link", { name: "返回通知列表" })).toHaveAttribute("href", "/account?view=notifications&filter=unread");
});
