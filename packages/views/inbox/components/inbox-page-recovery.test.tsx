import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { inboxKeys } from "@multica/core/inbox/queries";
import { useInboxFilterStore } from "@multica/core/inbox/filter-store";
import type { InboxItem } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { NavigationProvider } from "../../navigation";
import { InboxPage } from "./inbox-page";

const mocks = vi.hoisted(() => ({
  listInbox: vi.fn(),
  listArchivedInbox: vi.fn(),
  replace: vi.fn(),
  compact: false,
}));

vi.mock("@multica/core/api", async (importOriginal) => ({
  ...await importOriginal<typeof import("@multica/core/api")>(),
  api: { listInbox: mocks.listInbox, listArchivedInbox: mocks.listArchivedInbox },
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    inbox: () => "/acme/inbox",
    issueDetail: (id: string) => `/acme/issues/${id}`,
    triage: () => "/acme/triage",
  }),
}));
vi.mock("@multica/ui/hooks/use-mobile", () => ({ useIsCompact: () => mocks.compact }));
vi.mock("react-resizable-panels", () => ({
  useDefaultLayout: () => ({ defaultLayout: undefined, onLayoutChanged: vi.fn() }),
}));
vi.mock("@multica/ui/components/ui/resizable", () => ({
  ResizablePanelGroup: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  ResizablePanel: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  ResizableHandle: () => null,
}));
vi.mock("../../issues/components", () => ({
  // The issue editor's own suite owns editing. This stand-in proves whether
  // inbox recovery preserves its child's mounted instance and unsent input.
  IssueDetail: ({ leadingAction }: { leadingAction?: ReactNode }) => <section aria-label="Issue detail">
    {leadingAction}
    <textarea aria-label="Reply draft" defaultValue="" />
  </section>,
  StatusIcon: () => null,
  issueHighlightMementoKey: (id: string) => `highlight:${id}`,
}));
vi.mock("./inbox-list", () => ({
  InboxList: ({ items, view, onSelect }: { items: InboxItem[]; view: string; onSelect: (item: InboxItem) => void }) => <div>
    {items.map((item) => <button key={item.id} onClick={() => onSelect(item)}>{item.title}</button>)}
    {items.length === 0 && <p>{view === "archived" ? "No archived notifications" : "No notifications"}</p>}
  </div>,
}));
vi.mock("./inbox-filter-menu", () => ({ InboxFilterMenu: () => null }));
vi.mock("./inbox-list-item", () => ({ useTimeAgo: () => () => "just now" }));
vi.mock("./inbox-detail-label", () => ({ useTypeLabels: () => ({}) }));
vi.mock("./inbox-context-menu", () => ({ InboxContextMenuProvider: ({ children }: { children: ReactNode }) => children }));
vi.mock("../../modals/use-issue-limit-upgrade-prompt", () => ({ useIssueLimitUpgradePrompt: () => vi.fn() }));

const clients: QueryClient[] = [];
const notification: InboxItem = {
  id: "notification-1", workspace_id: "workspace-1", recipient_type: "member", recipient_id: "member-1",
  actor_type: "agent", actor_id: "agent-1", type: "new_comment", severity: "info", issue_id: "issue-1",
  title: "Review release checklist", body: null, issue_status: null, issue_priority: null,
  read: true, archived: false, created_at: "2026-10-09T00:00:00Z", details: null,
};

function renderInbox(archived: boolean, cached = false, selected = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  clients.push(client);
  const queryKey = archived ? inboxKeys.archived("workspace-1") : inboxKeys.list("workspace-1");
  client.setQueryData(archived ? inboxKeys.list("workspace-1") : inboxKeys.archived("workspace-1"), []);
  if (cached) client.setQueryData(queryKey, [{ ...notification, archived }]);
  const params = new URLSearchParams();
  if (archived) params.set("view", "archived");
  if (selected) params.set("issue", notification.issue_id!);
  const view = renderWithI18n(
    <QueryClientProvider client={client}>
      <NavigationProvider value={{ push: vi.fn(), replace: mocks.replace, back: vi.fn(), pathname: "/acme/inbox", searchParams: params, hash: "", getShareableUrl: (path) => path }}>
        <InboxPage />
      </NavigationProvider>
    </QueryClientProvider>,
  );
  return { ...view, client, queryKey };
}

beforeEach(() => {
  mocks.listInbox.mockReset().mockResolvedValue([]);
  mocks.listArchivedInbox.mockReset().mockResolvedValue([]);
  mocks.replace.mockReset();
  mocks.compact = false;
  useInboxFilterStore.setState({ filtersByWorkspace: {} });
});
afterEach(() => { for (const client of clients.splice(0)) client.clear(); });

describe("InboxPage query recovery", () => {
  it.each([false, true])("retries a cold failure and preserves its selected URL (archived: %s)", async (archived) => {
    const request = archived ? mocks.listArchivedInbox : mocks.listInbox;
    request.mockRejectedValueOnce(new Error("Unavailable")).mockResolvedValue([{ ...notification, archived }]);
    renderInbox(archived, false, true);
    expect(await screen.findByRole("alert")).toHaveTextContent(/load.*notifications/i);
    expect(screen.queryByText("No notifications")).not.toBeInTheDocument();
    expect(screen.queryByText("No archived notifications")).not.toBeInTheDocument();
    expect(screen.queryByText("Your inbox is empty")).not.toBeInTheDocument();
    expect(mocks.replace).not.toHaveBeenCalled();
    await userEvent.setup().click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("button", { name: notification.title })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Reply draft" })).toBeInTheDocument();
    expect(request).toHaveBeenCalledTimes(2);
    expect(mocks.replace).not.toHaveBeenCalled();
  });

  it.each([false, true])("retains cached notifications and an open reply through refresh failure (archived: %s)", async (archived) => {
    const user = userEvent.setup();
    const request = archived ? mocks.listArchivedInbox : mocks.listInbox;
    request.mockRejectedValueOnce(new Error("Offline")).mockResolvedValue([{ ...notification, archived }]);
    const view = renderInbox(archived, true, true);
    const row = screen.getByRole("button", { name: notification.title });
    const editor = screen.getByRole("textbox", { name: "Reply draft" });
    await user.type(editor, "Keep my unsent reply");
    await act(async () => { await view.client.refetchQueries({ queryKey: view.queryKey, exact: true }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh notifications");
    expect(screen.getByRole("button", { name: notification.title })).toBe(row);
    expect(screen.getByRole("textbox", { name: "Reply draft" })).toBe(editor);
    expect(editor).toHaveValue("Keep my unsent reply");
    expect(editor).toHaveFocus();
    expect(mocks.replace).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(screen.getByRole("textbox", { name: "Reply draft" })).toBe(editor);
    expect(editor).toHaveValue("Keep my unsent reply");
    expect(request).toHaveBeenCalledTimes(2);
  });

  it("offers retry in the compact detail without discarding its reply", async () => {
    mocks.compact = true;
    mocks.listInbox.mockRejectedValue(new Error("Offline"));
    const view = renderInbox(false, true, true);
    const editor = screen.getByRole("textbox", { name: "Reply draft" });
    await userEvent.setup().type(editor, "Compact reply");
    await act(async () => { await view.client.refetchQueries({ queryKey: view.queryKey, exact: true }); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not refresh notifications");
    expect(screen.getByRole("button", { name: "Retry" })).toBeEnabled();
    expect(screen.getByRole("textbox", { name: "Reply draft" })).toBe(editor);
    expect(editor).toHaveValue("Compact reply");
  });

  it("keeps a successful empty response as the ordinary empty state", async () => {
    renderInbox(false);
    expect(await screen.findByText("No notifications")).toBeInTheDocument();
    expect(screen.getByText("Your inbox is empty")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
