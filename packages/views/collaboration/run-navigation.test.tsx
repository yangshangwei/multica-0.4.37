import { useState } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { parseCollaborationIssue } from "@multica/core/collaboration";
import { collaborationExecutionIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { accountPath, accountRouteKey, parseAccountPath } from "@multica/core/paths";
import { NavigationProvider } from "../navigation";
import { ScrollRestorationProvider, type ScrollRestorationAdapter } from "../platform/scroll-restoration";
import { renderWithI18n } from "../test/i18n";
import { CollaborationNotificationsPage } from "./notifications-page";

const fixture = collaborationExecutionIssueFixture;
const execution = fixture.execution;
const settledAt = "2026-10-19T00:00:00Z";
const issue = parseCollaborationIssue({ ...fixture, execution: { ...execution,
  channels: [{ ...execution.channels[0], active_group_id: null }],
  targets: [{ ...execution.targets[0], active_group_id: null }],
  commands: [{ ...execution.commands[0], state: "failed" }],
  groups: [{ ...execution.groups[0], state: "failed", forced_disposition: null, settled_at: settledAt }],
  runs: [{ ...execution.runs[0], progress_state: "settled", proof_state: "exited_proven", business_outcome: "failure", forced_disposition: null, settled_at: settledAt }],
} })!;
const run = issue.execution!.runs[0]!;
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
vi.mock("@multica/core/auth", () => {
  const state = { user: { id: "00000000-0000-4000-8000-000000000001" } };
  return { useAuthStore: Object.assign((selector: (value: typeof state) => unknown) => selector(state), { getState: () => state }) };
});
vi.mock("@multica/core/collaboration", async (original) => ({ ...await original<typeof import("@multica/core/collaboration")>(), useCollaborationIssue: () => ({ data: issue }) }));
vi.mock("../rich-content", () => ({ RichContent: ({ content }: { content: string }) => <p>{content}</p> }));
vi.mock("./workflow", () => ({ CollaborationWorkflow: () => null }));
vi.mock("./activity", () => ({ CollaborationActivity: () => null }));
vi.mock("./discussion-actions", () => ({ CollaborationIssueReactions: () => null }));
vi.mock("./attachment-panel", () => ({ CollaborationAttachmentPanel: () => null, CollaborationCommentAttachments: () => null }));
vi.mock("./field-summary", () => ({ CollaborationFieldSummary: () => null }));
vi.mock("./comment-list", () => ({ CollaborationCommentList: () => null }));
vi.mock("./comment-composer", () => ({ CollaborationCommentComposer: () => <textarea aria-label="Retained draft" defaultValue="" /> }));
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); });

it("reopens and focuses a run from the same result notification after hiding its anchored records", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const destination = accountPath({ view: "notifications", notificationView: "all", notificationId: issue.id, issueId: issue.id, runId: run.id });
  const replace = vi.fn();
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async (url: string) => new Response(JSON.stringify(url.includes("/collaboration-notifications") ? {
    view: "all", items: [{ id: issue.id, issue_id: issue.id, event_seq: "10", kind: "ai_failed", is_mention: false, source_kind: "issue", source_id: null,
      run_id: run.id, issue_title: issue.title, can_open: true, created_at: settledAt, read_at: null, archived_at: null }], unread_count: 1, next_cursor: null,
  } : url.includes("/records") ? {
    protocol_version: 1, issue_id: issue.id, run,
    resource: { agent_id: run.agent_id, runtime_id: issue.id, resource_owner_user_id: issue.id, execution_workspace_id: issue.id, grant_id: issue.id,
      provider: null, model: null, account_source: null },
    recording: { status: "not_recorded", inputs_recorded: false }, usage: null,
    items: [{ id: "result:1", kind: "result", outcome: "failure", body: "Result opened again" }], next_cursor: null,
  } : { issue_id: issue.id, items: [], next_cursor: null }))));
  function RoutedDetail() {
    const [path, setPath] = useState(destination);
    const url = new URL(path, "https://server.example");
    const current = parseAccountPath(path);
    if (!current || current.view !== "notifications") throw new Error("Invalid notification test route");
    return <NavigationProvider value={{ pathname: url.pathname, searchParams: url.searchParams, hash: url.hash, push: setPath,
      replace: (next) => { replace(next); setPath(next); }, back: vi.fn(), getShareableUrl: (next) => `https://server.example${next}` }}>
      <CollaborationNotificationsPage destination={current} />
    </NavigationProvider>;
  }
  renderWithI18n(<QueryClientProvider client={client}><RoutedDetail /></QueryClientProvider>);
  expect(await screen.findByText("Result opened again")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: `Hide records for ${run.id}` }));
  expect(screen.queryByText("Result opened again")).not.toBeInTheDocument();
  await userEvent.click(await screen.findByRole("link", { name: issue.title }));
  expect(await screen.findByText("Result opened again")).toBeInTheDocument();
  expect(document.getElementById(`run-${run.id}`)).toHaveFocus();
  expect(replace).toHaveBeenCalledWith(accountPath({ view: "notifications", notificationView: "all", notificationId: issue.id, issueId: issue.id }));
  expect(screen.getByRole("link", { name: issue.title })).toHaveAttribute("aria-current", "true");
});

it("restores distinct same-issue notification history offsets without remounting the list, detail or draft", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const secondId = "00000000-0000-4000-8000-000000000009";
  const first = accountPath({ view: "notifications", notificationView: "all", notificationId: issue.id, issueId: issue.id, anchor: "description" });
  const second = accountPath({ view: "notifications", notificationView: "all", notificationId: secondId, issueId: issue.id });
  let renderedPath = first;
  let navigationPath = first;
  let history = false;
  let navigationKey = 0;
  const offsets = new Map<string, Record<string, { top: number; height: number }>>();
  const restoration: ScrollRestorationAdapter = { get: (key) => offsets.get(accountRouteKey(navigationPath)!)?.[key] };
  vi.stubGlobal("fetch", vi.fn(async (input: string) => new Response(JSON.stringify(input.includes("/collaboration-notifications") ? {
    view: "all", items: [
      { id: issue.id, issue_id: issue.id, event_seq: "10", kind: "issue_edited", is_mention: false, source_kind: "body", source_id: null,
        issue_title: "First notification", can_open: true, created_at: settledAt, read_at: null, archived_at: null },
      { id: secondId, issue_id: issue.id, event_seq: "9", kind: "assigned", is_mention: false, source_kind: "issue", source_id: null,
        issue_title: "Second notification", can_open: true, created_at: settledAt, read_at: null, archived_at: null },
    ], unread_count: 2, next_cursor: null,
  } : { issue_id: issue.id, items: [], next_cursor: null }))));
  function content() {
    const current = parseAccountPath(renderedPath);
    if (!current || current.view !== "notifications") throw new Error("Invalid notification test route");
    const url = new URL(navigationPath, "https://server.example");
    return <QueryClientProvider client={client}><NavigationProvider value={{ pathname: url.pathname, searchParams: url.searchParams, hash: url.hash,
      isHistoryTraversal: history, navigationKey, push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => path }}>
      <ScrollRestorationProvider adapter={restoration}><CollaborationNotificationsPage destination={current} /></ScrollRestorationProvider>
    </NavigationProvider></QueryClientProvider>;
  }
  let view!: ReturnType<typeof renderWithI18n>;
  await act(async () => { view = renderWithI18n(content()); });
  await screen.findByRole("link", { name: "First notification" });
  const draft = await screen.findByRole("textbox", { name: "Retained draft" });
  await userEvent.type(draft, "An unsent same-issue draft");
  const list = document.querySelector<HTMLElement>("[data-tab-scroll-root=account-notifications-list]")!;
  const detail = document.querySelector<HTMLElement>("[data-tab-scroll-root=account-detail]")!;
  list.scrollTop = 137;
  detail.scrollTop = 420;
  offsets.set(accountRouteKey(first)!, { "account-notifications-list": { top: 137, height: 1800 }, "account-detail": { top: 420, height: 2400 } });
  offsets.set(accountRouteKey(second)!, { "account-notifications-list": { top: 281, height: 1800 }, "account-detail": { top: 740, height: 2400 } });
  await act(async () => { renderedPath = navigationPath = second; navigationKey += 1; view.rerender(content()); });
  expect(list.scrollTop).toBe(137);
  expect(detail.scrollTop).toBe(420);
  list.scrollTop = 281;
  detail.scrollTop = 740;
  offsets.set(accountRouteKey(second)!, { "account-notifications-list": { top: 281, height: 1800 }, "account-detail": { top: 740, height: 2400 } });
  draft.focus();
  // The native adapter can publish the destination before its router renders.
  await act(async () => { navigationPath = first; history = true; navigationKey += 1; view.rerender(content()); });
  expect(list.scrollTop).toBe(281);
  expect(detail.scrollTop).toBe(740);
  await act(async () => { renderedPath = first; view.rerender(content()); });
  await waitFor(() => expect(list.scrollTop).toBe(137));
  expect(detail.scrollTop).toBe(420);
  expect(draft).toHaveFocus();
  await act(async () => { renderedPath = navigationPath = second; navigationKey += 1; view.rerender(content()); });
  await waitFor(() => expect(list.scrollTop).toBe(281));
  expect(detail.scrollTop).toBe(740);
  // A zero entry and a missing entry both mean the top of this destination.
  offsets.set(accountRouteKey(first)!, { "account-notifications-list": { top: 0, height: 1800 } });
  await act(async () => { renderedPath = navigationPath = first; navigationKey += 1; view.rerender(content()); });
  await waitFor(() => expect(list.scrollTop).toBe(0));
  expect(detail.scrollTop).toBe(0);
  expect(document.querySelector("[data-tab-scroll-root=account-notifications-list]")).toBe(list);
  expect(document.querySelector("[data-tab-scroll-root=account-detail]")).toBe(detail);
  expect(screen.getByRole("textbox", { name: "Retained draft" })).toBe(draft);
  expect(draft).toHaveValue("An unsent same-issue draft");
  await act(async () => { history = false; navigationKey += 1; view.rerender(content()); });
  await waitFor(() => expect(document.getElementById("description")).toHaveFocus());
  expect(list.scrollTop).toBe(0);
});
