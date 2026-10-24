import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { configureAdminControlStorage } from "@multica/core/admin";
import { adminCollaborationDetailFixture as administrationIssue } from "@multica/core/admin/test-fixtures";
import { accountPath, parseAccountPath } from "@multica/core/paths";
import { NavigationProvider, type NavigationAdapter } from "../navigation";
import { renderWithI18n } from "../test/i18n";
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

function mount(view: "current" | "participating" = "current", locale: "en" | "zh-Hans" = "en") {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  const nav: NavigationAdapter = { push: vi.fn(), replace: vi.fn(), back: vi.fn(), pathname: "/account", searchParams: new URLSearchParams(), hash: "", getShareableUrl: (path) => path };
  return renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={nav}><AccountWorkbenchPage destination={{ view, issueId: id, commentId: null }} /></NavigationProvider></QueryClientProvider>, { locale });
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
  expect(screen.getByText("This issue is outside the current list. You can continue viewing it here.")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Resume all" })).toBeNull();
  expect(fetch.mock.calls.every(([url]) => !String(url).includes("/api/workspaces"))).toBe(true);
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

it("renders persisted activity with the original actor and before/after values", async () => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async (url: string) => new Response(JSON.stringify(
    url.includes("workbench") ? { items: [], total: 0, next_cursor: null }
      : url.includes("/activity?") ? { issue_id: id, next_cursor: null, items: [{ id, issue_id: id, event_seq: "9007199254740993", kind: "issue_edited", actor: { ...account, name: "Original editor" }, created_at: "2026-10-18T00:00:00Z", details_available: true,
        target: null, handoff: null, comment_id: null, changes: [{ field: "title", before: "Previous title", after: "Updated title" }] }] }
      : url.includes("/comments?") ? { issue_id: id, items: [], next_cursor: null } : issue,
  ))));
  mount();
  expect(await screen.findByText("Original editor")).toBeInTheDocument();
  expect(screen.getByText("Previous title")).toBeInTheDocument();
  expect(screen.getByText("Updated title")).toBeInTheDocument();
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
