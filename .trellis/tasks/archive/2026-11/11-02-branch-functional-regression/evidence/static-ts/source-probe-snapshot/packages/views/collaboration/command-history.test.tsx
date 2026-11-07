import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationKeys, collaborationScope, parseCollaborationComment, parseCollaborationIssue, useCollaborationIssue,
  type CollaborationScope } from "@multica/core/collaboration";
import { collaborationExecutionIssueFixture as issueFixture, collaborationCommandHistoryFixture as historyFixture,
  collaborationUnknownCancellationFixture } from "@multica/core/collaboration/test-fixtures";
import { NavigationProvider } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { CollaborationExecutionPanel } from "./execution-panel";
import { CollaborationCommentList } from "./comment-list";
import { CollaborationCommandHistory } from "./command-history";

const issueId = issueFixture.id;
const waiting = issueFixture.execution.commands[1]!;
const target = issueFixture.execution.targets[0]!;
const original = { ...historyFixture.items[0]!, id: waiting.id, session_id: issueFixture.session.id,
  target_id: waiting.target_id, target: { kind: target.kind, resource_id: target.resource_id }, input_event_id: waiting.input_event_id,
  channel_id: waiting.channel_id, batch_id: waiting.batch_id, group_id: null, seq: waiting.sequence };
const file = { id: original.attachment_ids[0]!, draft_id: issueId, issue_id: issueId, intended_issue_id: issueId,
  source_workspace_id: issueId, uploader_user_id: original.author_user_id, name: "original.txt", content_type: "text/plain",
  size_bytes: 8, version: "9007199254740993", created_at: original.created_at, delivered_at: original.created_at };
const comment = parseCollaborationComment({ id: original.comment_id, issue_id: issueId, author_type: "member", author: original.author,
  content: "Edited current comment, different from the original instruction.", type: "comment", parent_id: null, comment_revision: "2",
  attachment_ids: [], created_at: original.created_at, updated_at: original.created_at, deleted_at: null, resolved_at: null,
  resolved_by: null, can_edit: false, can_resolve: false, reactions: [] })!;
const clients: QueryClient[] = [];
const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => auth } }));
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); auth.user.id = issueId; localStorage.clear(); vi.unstubAllGlobals(); });

function setup(viewerId = issueId) {
  auth.user.id = viewerId;
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  clients.push(client);
  setApiInstance(new ApiClient("https://server.example"));
  return { client, scope: collaborationScope(auth.user.id) };
}

function navigation() {
  return { pathname: "/account", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(),
    getShareableUrl: (path: string) => `https://server.example${path}` };
}

// Canonical malformed/provenance/paging/access matrices live in core's
// command-history-{schema,queries}.test.ts. This path proves actual Cancel,
// Query invalidation, immutable rendering and a fresh mounted shared view.
it("keeps a cancelled original input visible after refresh while its current comment is edited and later deleted", async () => {
  const f = setup();
  let cancelled = false;
  let deleted = false;
  const currentIssue = () => ({ ...issueFixture, write_context: { participant_epoch: "1", account_epoch: "1" }, execution: {
    ...issueFixture.execution, commands: issueFixture.execution.commands.map((item) => item.id === waiting.id
      ? { ...item, author_user_id: original.author_user_id, state: cancelled ? "cancelled" : "waiting" } : item),
  } });
  f.client.setQueryData(collaborationKeys.issue(f.scope, issueId), parseCollaborationIssue(currentIssue())!);
  const fetch = vi.fn(async (url: string, init?: RequestInit) => {
    const path = new URL(url).pathname;
    if (path.endsWith(`/commands/${waiting.id}/cancel`)) {
      cancelled = true;
      return new Response(JSON.stringify({ protocol_version: 2, issue_id: issueId, request_id: new Headers(init?.headers).get("Idempotency-Key"),
        action: "cancel_command", status: 200, code: "command_cancelled", scope: "target", target_id: waiting.target_id,
        command_id: waiting.id, command_sequence: waiting.sequence, channel_id: waiting.channel_id, batch_id: waiting.batch_id, group_id: null,
        queue_state: "paused", control_version: target.control.control_version, group_settled: false, run_id: null, run_settled: false,
        grant_id: null, channels: [], target_holds: [] }));
    }
    if (path.endsWith("/commands")) return new Response(JSON.stringify({ ...historyFixture,
      items: [cancelled ? original : { ...original, state: "waiting", reason: null, cancellation: null }] }));
    if (path.endsWith("/attachments")) return new Response(JSON.stringify({ issue_id: issueId, attachments: [file] }));
    if (path === `/api/collaboration/issues/${issueId}`) return new Response(JSON.stringify(currentIssue()));
    throw new Error(`Unexpected history test request: ${path}`);
  });
  vi.stubGlobal("fetch", fetch);
  function Detail({ scope }: { scope: CollaborationScope }) {
    const query = useCollaborationIssue(scope, issueId);
    if (!query.data) return null;
    return <><CollaborationExecutionPanel scope={scope} issue={query.data} />
      <CollaborationCommentList scope={scope} issue={query.data} comments={[{ ...comment, deleted_at: deleted ? original.created_at : null }]} onReply={vi.fn()} /></>;
  }
  const mount = () => renderWithI18n(<QueryClientProvider client={f.client}><NavigationProvider value={navigation()}><Detail scope={f.scope} /></NavigationProvider></QueryClientProvider>);
  const view = mount();
  expect(await screen.findByText(comment.content)).toBeInTheDocument();
  expect(fetch.mock.calls.some(([url]) => new URL(url).pathname.endsWith("/commands"))).toBe(false);
  const disclosure = screen.getByRole("button", { name: "Command history" });
  expect(disclosure).toHaveAttribute("aria-expanded", "false");
  await userEvent.click(disclosure);
  const history = await screen.findByRole("region", { name: "Command history" });
  expect(await within(history).findByText(original.content)).toBeInTheDocument();
  expect(within(history).getByText(original.author.name)).toBeInTheDocument();
  expect(await within(history).findByRole("button", { name: "Download original.txt" })).toBeInTheDocument();
  expect(within(history).getByRole("link", { name: "Open original comment" })).toHaveAttribute("href", `/account?view=participating&issue=${issueId}#comment-${original.comment_id}`);
  await userEvent.click(screen.getByRole("button", { name: "Cancel waiting input 2" }));
  expect(await within(history).findByText("Cancelled")).toBeInTheDocument();
  expect(within(history).getByText("Cancelled by a participant")).toBeInTheDocument();
  expect(within(history).getByText(original.cancellation.actor.name)).toBeInTheDocument();
  expect(within(history).getByText("Cancelled at").nextElementSibling?.querySelector("time"))
    .toHaveAttribute("datetime", original.cancellation.created_at);
  await waitFor(() => expect(screen.queryByRole("button", { name: "Cancel waiting input 2" })).not.toBeInTheDocument());
  expect(within(history).getByText(original.content)).toBeInTheDocument();
  expect(within(history).queryByText(comment.content)).not.toBeInTheDocument();
  expect(within(history).queryByRole("button", { name: /cancel|stop|resume/i })).not.toBeInTheDocument();
  view.unmount();
  f.client.clear();
  deleted = true;
  mount();
  await userEvent.click(await screen.findByRole("button", { name: "Command history" }));
  const refreshed = await screen.findByRole("region", { name: "Command history" });
  expect(await within(refreshed).findByText(original.content)).toBeInTheDocument();
  expect(within(refreshed).getByText("Cancelled")).toBeInTheDocument();
  expect(await within(refreshed).findByRole("button", { name: "Download original.txt" })).toBeInTheDocument();
  expect(fetch.mock.calls.filter(([url, request]) => request?.method === "POST" && url.includes(`/commands/${waiting.id}/cancel`))).toHaveLength(1);
  expect(fetch.mock.calls.some(([url]) => url.endsWith("/control"))).toBe(false);
});

it("shows source-only readers missing legacy facts and recorded automatic cancellation without control rights", async () => {
  const f = setup(original.author_user_id);
  const legacy = { ...original, author: null, comment_id: null, input_event_seq: null, reason: "participant_removed", cancellation: collaborationUnknownCancellationFixture };
  const otherId = "00000000-0000-4000-8000-000000000088";
  const automatic = { ...original, id: otherId, input_event_id: otherId, input_event_seq: "1", seq: "3", reason: "credentials_revoked",
    content: "A separate automatically cancelled input", created_at: "2026-10-20T08:00:00Z",
    cancellation: { ...original.cancellation, kind: "automatic", actor_user_id: null, actor: null } };
  const issue = parseCollaborationIssue({ ...issueFixture, write_context: null,
    capabilities: Object.fromEntries(Object.keys(issueFixture.capabilities).map((key) => [key, key === "read" || key === "reference"])) })!;
  vi.stubGlobal("fetch", vi.fn(async (url: string) => new Response(JSON.stringify(new URL(url).pathname.endsWith("/attachments")
    ? { issue_id: issueId, attachments: [file] } : { ...historyFixture, items: [legacy, automatic] }))));
  renderWithI18n(<QueryClientProvider client={f.client}><NavigationProvider value={navigation()}>
    <CollaborationExecutionPanel scope={f.scope} issue={issue} />
  </NavigationProvider></QueryClientProvider>, { locale: "zh-Hans" });
  await userEvent.click(screen.getByRole("button", { name: "指令历史" }));
  const history = await screen.findByRole("region", { name: "指令历史" });
  expect(await within(history).findByText("取消详情未记录。")).toBeInTheDocument();
  const legacyRow = history.querySelector<HTMLElement>(`[data-command-id="${legacy.id}"]`)!;
  const missingTime = within(legacyRow).getByText("取消时间").nextElementSibling;
  expect(missingTime).toHaveTextContent("未记录");
  expect(missingTime?.querySelector("time")).toBeNull();
  expect(within(legacyRow).getByText("取消操作者").nextElementSibling).toHaveTextContent("未记录");
  expect(within(history).getByText("自动取消")).toBeInTheDocument();
  expect(within(history).getByText("指令作者的凭据已撤销")).toBeInTheDocument();
  expect(within(history).queryByText("系统")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: /停止|恢复|取消等待/ })).not.toBeInTheDocument();
});

it("removes open private history when loading more confirms access loss", async () => {
  const f = setup();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ ...historyFixture,
    items: [{ ...original, attachment_ids: [] }], next_cursor: "next-page" })))
    .mockResolvedValueOnce(new Response(JSON.stringify({ code: "not_found" }), { status: 404 })));
  renderWithI18n(<QueryClientProvider client={f.client}><NavigationProvider value={navigation()}>
    <CollaborationCommandHistory scope={f.scope} issueId={issueId} />
  </NavigationProvider></QueryClientProvider>);
  await userEvent.click(screen.getByRole("button", { name: "Command history" }));
  const history = await screen.findByRole("region", { name: "Command history" });
  expect(await within(history).findByText(original.content)).toBeInTheDocument();
  await userEvent.click(within(history).getByRole("button", { name: "Load more commands" }));
  expect(await within(history).findByText("You no longer have access to this issue.")).toBeInTheDocument();
  expect(within(history).queryByText(original.content)).not.toBeInTheDocument();
  expect(within(history).queryByRole("link", { name: "Open original comment" })).not.toBeInTheDocument();
});
