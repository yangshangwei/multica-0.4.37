import { afterEach, expect, it, vi } from "vitest";
import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { configureAdminControlStorage } from "@multica/core/admin";
import { adminCollaborationDetailFixture as administrationIssue } from "@multica/core/admin/test-fixtures";
import { accountPath, parseAccountPath } from "@multica/core/paths";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { ScrollRestorationProvider, type ScrollRestorationAdapter } from "../platform/scroll-restoration";
import { AccountWorkbenchPage } from "./account-workbench-page";

const id = "00000000-0000-4000-8000-000000000001";
vi.mock("@multica/core/auth", () => {
  const state = { status: "authenticated", user: { id: "00000000-0000-4000-8000-000000000001", name: "Viewer" } };
  return { useAuthStore: Object.assign((selector: (value: typeof state) => unknown) => selector(state), { getState: () => state }) };
});
const account = { user_id: id, name: "Handler", avatar_url: "", external: true };
const caps = { reference: true, read: true, comment: false, control_ai: true, submit_command: true, invite: false, transfer: false,
  remove_participants: false, leave: false, edit: false, edit_source_structure: false, reorder: false, change_status: false,
  complete: false, cancel: false, reopen: false, resolve_thread: false, react: false };
const issue = { id, title: "Outside current list", description: "Shared history", status: "blocked", status_category: "blocked", priority: "none",
  source_workspace: { id, name: "Source", identifier: "SRC-1" }, creator: account, handler: account,
  collaboration_revision: "1", lifecycle_epoch: "1", session: { id, queue_state: "paused", control_version: "1" }, capabilities: caps };
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
vi.mock("../rich-content", () => ({ RichContent: ({ content }: { content: string }) => <p>{content}</p> }));

function mount(view: "current" | "participating" = "current", locale: "en" | "zh-Hans" = "en", issueId: string | null = id,
  restoration: ScrollRestorationAdapter = { get: () => undefined }) {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const nav: NavigationAdapter = { push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/account", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path };
  return renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={nav}><ScrollRestorationProvider adapter={restoration}>
    <AccountWorkbenchPage destination={{ view, issueId, commentId: null }} />
  </ScrollRestorationProvider></NavigationProvider></QueryClientProvider>, { locale });
}

it("opens selected detail independently of an empty current list without workspace requests", async () => {
  const fetch = vi.fn().mockImplementation(async (url: string) => new Response(JSON.stringify(
    url.includes("/api/me/workbench") ? { items: [], total: 0, next_cursor: null }
      : url.includes("/comments?") || url.includes("/activity?") ? { issue_id: id, items: [], next_cursor: null }
      : url.includes("/mention-targets") ? { protocol_version: 2, issue_id: id, participants: [], can_submit_commands: false, targets: [] } : issue,
  ), { status: 200 }));
  vi.stubGlobal("fetch", fetch);
  mount();
  expect(await screen.findByRole("heading", { name: "Outside current list" })).toBeInTheDocument();
  expect(screen.getByText("No issues need your handling.")).toBeInTheDocument();
  expect(within(screen.getByRole("region", { name: "Collaboration issues" })).getByRole("link", { name: "View participating issues" }))
    .toHaveAttribute("href", accountPath({ view: "participating", issueId: id }));
  expect(screen.getByText("This issue is outside the current list. You can continue viewing it here.")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Resume all" })).toBeNull();
  expect(fetch.mock.calls.every(([url]) => !String(url).includes("/api/workspaces"))).toBe(true);
});

it("searches trimmed text across explicitly loaded pages and clears only search while retaining sorting and saved list state", async () => {
  const first = { ...issue, title: "Zulu task", updated_at: "2026-11-04T10:00:00Z" };
  const second = { ...first, id: "00000000-0000-4000-8000-000000000002", title: "Alpha task" };
  const third = { ...first, id: "00000000-0000-4000-8000-000000000003", title: "Beta task" };
  const fetch = vi.fn(async (url: string) => new Response(JSON.stringify(url.includes("/api/me/workbench")
    ? new URL(url).searchParams.has("cursor") ? { items: [third], total: 3, next_cursor: null }
      : { items: [first, second], total: 3, next_cursor: "next-page" }
    : { items: [], unread_count: 0, next_cursor: null })));
  vi.stubGlobal("fetch", fetch);
  const state = new Map([["account-search", "   Beta   "], ["account-sort", "title"]]);
  const setViewState = vi.fn((key: string, value: string | undefined) => { if (value !== undefined) state.set(key, value); });
  const restoration = { get: () => ({ top: 72, height: 1000 }), getViewState: (key: string) => state.get(key), setViewState };
  const view = mount("participating", "en", null, restoration);
  const list = screen.getByRole("region", { name: "Collaboration issues" });
  expect(await within(list).findByText("2 of 3 issues loaded")).toBeInTheDocument();
  expect(within(list).getByText("Matches: 0")).toBeInTheDocument();
  expect(within(list).getByText("Search covers loaded issues. Load more to include the rest.")).toBeInTheDocument();
  expect(within(list).getByText("No loaded issues match your search.")).toBeInTheDocument();
  expect(list.querySelector("[data-tab-scroll-root=account-list]")?.scrollTop).toBe(72);
  expect(fetch.mock.calls.filter(([url]) => url.includes("workbench"))).toHaveLength(1);
  await userEvent.click(within(list).getByRole("button", { name: "Load more" }));
  expect(await within(list).findByRole("link", { name: /Beta task/ })).toBeInTheDocument();
  expect(within(list).getByText("3 issues")).toBeInTheDocument();
  expect(within(list).getByText("Matches: 1")).toBeInTheDocument();
  expect(within(list).queryByText("Search covers loaded issues. Load more to include the rest.")).not.toBeInTheDocument();
  await userEvent.click(within(list).getByRole("button", { name: "Clear search" }));
  expect(within(list).getByRole("textbox", { name: "Search loaded issues" })).toHaveValue("");
  expect(within(list).getByRole("textbox", { name: "Search loaded issues" })).toHaveFocus();
  expect(within(list).getByRole("combobox", { name: "Sort" })).toHaveValue("title");
  expect(within(list).getAllByRole("link").map(link => link.textContent)).toEqual([
    expect.stringContaining("Alpha task"), expect.stringContaining("Beta task"), expect.stringContaining("Zulu task"),
  ]);
  expect(setViewState).toHaveBeenCalledExactlyOnceWith("account-search", "", undefined);
  expect(fetch.mock.calls.filter(([url]) => url.includes("workbench"))).toHaveLength(2);
  view.unmount();
  mount("participating", "en", null, restoration);
  expect(screen.getByRole("textbox", { name: "Search loaded issues" })).toHaveValue("");
  expect(screen.getByRole("combobox", { name: "Sort" })).toHaveValue("title");
});

it("keeps the initial count unknown and does not call an incomplete empty page an empty queue", async () => {
  let resolve!: (response: Response) => void;
  vi.stubGlobal("fetch", vi.fn((url: string) => url.includes("workbench") ? new Promise<Response>(ready => { resolve = ready; })
    : Promise.resolve(new Response(JSON.stringify({ items: [], unread_count: 0, next_cursor: null })))));
  mount("current", "en", null);
  const list = screen.getByRole("region", { name: "Collaboration issues" });
  expect(within(list).getByText("Loading collaboration...")).toBeInTheDocument();
  expect(within(list).queryByText("0 issues")).not.toBeInTheDocument();
  await act(async () => resolve(new Response(JSON.stringify({ items: [], total: 1, next_cursor: "next-page" }))));
  expect(await within(list).findByText("0 of 1 issues loaded")).toBeInTheDocument();
  expect(within(list).queryByText("No issues need your handling.")).not.toBeInTheDocument();
  expect(within(list).getByRole("button", { name: "Load more" })).toBeEnabled();
});

it("does not hide selected detail or filter rows for whitespace-only Chinese search", async () => {
  vi.stubGlobal("fetch", vi.fn(async (url: string) => new Response(JSON.stringify(url.includes("workbench")
    ? { items: [{ ...issue, title: "已加载任务", updated_at: "2026-11-04T10:00:00Z" }], total: 1, next_cursor: null }
    : url.includes("/comments?") || url.includes("/activity?") ? { issue_id: id, items: [], next_cursor: null } : issue))));
  mount("participating", "zh-Hans");
  const list = screen.getByRole("region", { name: "协作任务" });
  expect(await screen.findByRole("heading", { name: "Outside current list" })).toBeInTheDocument();
  const search = within(list).getByRole("textbox", { name: "搜索已加载的任务" });
  fireEvent.change(search, { target: { value: "不存在" } });
  expect(within(list).getByText("匹配结果：0")).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "Outside current list" })).toBeInTheDocument();
  fireEvent.change(search, { target: { value: "   " } });
  expect(within(list).getByRole("link", { name: /已加载任务/ })).toBeInTheDocument();
  await userEvent.click(within(list).getByRole("button", { name: "清空搜索" }));
  expect(search).toHaveFocus();
  expect(search).toHaveValue("");
});

it("clears denied detail and keeps the account list usable", async () => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async (url: string) => url.includes("/api/me/workbench")
    ? new Response(JSON.stringify({ items: [], total: 0, next_cursor: null }))
    : new Response(JSON.stringify({ error: "PRIVATE_DENIED_BODY" }), { status: 403 })));
  mount("participating");
  expect(await screen.findByText("You no longer have access to this issue.")).toBeInTheDocument();
  const list = screen.getByRole("region", { name: "Collaboration issues" });
  await userEvent.click(within(list).getByRole("button", { name: "Refresh" }));
  await waitFor(() => expect(screen.getByText("No participating issues yet.")).toBeInTheDocument());
  expect(screen.queryByText("PRIVATE_DENIED_BODY")).toBeNull();
  expect(screen.queryByText("Shared history")).toBeNull();
});

it("provides translated workbench navigation", async () => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async (url: string) => new Response(JSON.stringify(url.includes("workbench") ? { items: [], total: 0, next_cursor: null } : issue))));
  mount("current", "zh-Hans");
  expect(screen.getByRole("link", { name: "当前处理" })).toHaveAttribute("href", `/account?view=current&issue=${id}`);
  expect(screen.getByRole("link", { name: "我参与的" })).toBeInTheDocument();
});

it.each(["en", "zh-Hans"] as const)("renders persisted activity in %s with the original actor and before/after values", async locale => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async (url: string) => new Response(JSON.stringify(
    url.includes("workbench") ? { items: [], total: 0, next_cursor: null }
      : url.includes("/activity?") ? { issue_id: id, next_cursor: null, items: [{ id, issue_id: id, event_seq: "9007199254740993", kind: "issue_edited", actor: { ...account, name: "Original editor" }, created_at: "2026-10-18T00:00:00Z", details_available: true,
        target: null, handoff: null, comment_id: null, changes: [{ field: "title", before: "Previous title", after: "Updated title" }] }] }
      : url.includes("/comments?") ? { issue_id: id, items: [], next_cursor: null } : issue,
  ))));
  mount("current", locale);
  expect(await screen.findByText("Original editor")).toBeInTheDocument();
  expect(screen.getByText("Previous title")).toBeInTheDocument();
  expect(screen.getByText("Updated title")).toBeInTheDocument();
  expect(screen.getByText(new Date("2026-10-18T00:00:00Z").toLocaleString(locale))).toHaveAttribute("datetime", "2026-10-18T00:00:00Z");
});

it("keeps account administration on dedicated metadata reads and administration links", async () => {
  const api = new ApiClient("https://server.example");
  setApiInstance(api);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  configureAdminControlStorage({ getItem: () => null, setItem: vi.fn(), removeItem: vi.fn(), keys: () => [] });
  vi.spyOn(api, "getAdminMe").mockResolvedValue({ userId: id, organizationId: administrationIssue.scope,
    role: "super_admin", supported: true, allowedActions: [] });
  vi.spyOn(api, "getAdminCollaborationIssues").mockResolvedValue({ scope: administrationIssue.scope, items: [administrationIssue], nextCursor: null });
  vi.spyOn(api, "getAdminCollaborationIssue").mockResolvedValue(administrationIssue);
  vi.spyOn(api, "getAdminCollaborationMemoryVersions").mockResolvedValue({ scope: administrationIssue.scope, issueId: administrationIssue.issueId,
    currentVersion: "0", items: [], nextCursor: null });
  const ordinaryFetch = vi.fn().mockRejectedValue(new Error("Ordinary reads must not mount during administration"));
  vi.stubGlobal("fetch", ordinaryFetch);
  const content = (path: string) => {
    const destination = parseAccountPath(path);
    if (!destination) throw new Error("Invalid administration test route");
    const nav: NavigationAdapter = { push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/account",
      searchParams: new URL(path, "https://server.example").searchParams, hash: "", getShareableUrl: value => value };
    return <QueryClientProvider client={client}><NavigationProvider value={nav}>
      <AccountWorkbenchPage destination={destination} />
    </NavigationProvider></QueryClientProvider>;
  };
  const view = renderWithI18n(content(accountPath({ administration: true })));
  const detailPath = accountPath({ administration: true, issueId: administrationIssue.issueId });
  expect(await screen.findByRole("link", { name: administrationIssue.identifier })).toHaveAttribute("href", detailPath);
  expect(ordinaryFetch).not.toHaveBeenCalled();
  view.rerender(content(detailPath));
  expect(await screen.findByRole("heading", { name: administrationIssue.identifier })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Back to collaborations" })).toHaveAttribute("href", accountPath({ administration: true }));
  expect(api.getAdminCollaborationIssue).toHaveBeenCalledWith(administrationIssue.issueId, { signal: expect.any(AbortSignal) });
  expect(ordinaryFetch).not.toHaveBeenCalled();
  expect(screen.queryByRole("region", { name: "Collaboration issues" })).not.toBeInTheDocument();
});
