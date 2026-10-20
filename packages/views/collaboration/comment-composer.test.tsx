import { afterEach, expect, it, vi } from "vitest";
import { act, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, finishCollaborationDeliveryDraft, parseCollaborationComment, parseCollaborationIssue, readCollaborationDeliveryRecords, saveCollaborationDeliveryRecord } from "@multica/core/collaboration";
import { renderWithI18n } from "../test/i18n";
import { CollaborationCommentComposer } from "./comment-composer";

const id = "00000000-0000-4000-8000-000000000001";
const target = "00000000-0000-4000-8000-000000000002";
const agent = "00000000-0000-4000-8000-000000000003";
const eventId = "00000000-0000-4000-8000-000000000004";
const account = { user_id: id, name: "Participant", avatar_url: "", external: true };
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const client = new QueryClient();
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); });
const issue = parseCollaborationIssue({ id, title: "Issue", description: "Body", status: "todo", status_category: "todo", priority: "none", source_workspace: { id, name: "Source", identifier: "SRC" }, creator: account, handler: account,
  collaboration_revision: "1", lifecycle_epoch: "1", write_context: { participant_epoch: "2", account_epoch: "3" }, session: { id, queue_state: "paused", control_version: "1" },
  capabilities: { reference: true, read: true, comment: true, control_ai: false, submit_command: false, invite: false, transfer: false, remove_participants: false, leave: false, edit: false, edit_source_structure: false, reorder: false, change_status: false, complete: false, cancel: false, reopen: false, resolve_thread: false, react: false } })!;

it("invalidates grouping when another form updates the shared draft", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const candidate = { target_id: target, kind: "agent", resource_id: agent, name: "Leader", target_epoch: "1", config_version: "1", hold_reason: null, hold_version: "1" };
  const fetch = vi.fn().mockImplementation(async (url: string) => new Response(JSON.stringify(url.endsWith("mention-targets")
    ? { protocol_version: 2, issue_id: id, participants: [], can_submit_commands: true, targets: [{ ...candidate, queue_state: "active", waiting_reason: "waiting", execution_available: false }] }
    : { protocol_version: 2, issue_id: id, lifecycle_epoch: "1", recipients: [], targets: [{ ...candidate, state: "waiting", code: "command_queued", command_id: null,
      grouping: { kind: "pending_batch", target_ids: [target], has_existing_inputs: false, follows_pending_batch: false } }] })));
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentComposer scope={scope} issue={issue} /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Leader" }));
  await user.click(screen.getByRole("button", { name: "Preview delivery" }));
  expect(await screen.findByRole("region", { name: "AI grouping preview" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Send comment" })).toBeEnabled();

  // Another mounted form writes through the shared store, without this
  // composer's local change handler or mutation reset running.
  const form = readCollaborationDeliveryRecords(scope).find((record) => record.kind === "form" && record.purpose === "comment");
  if (!form || form.kind !== "form") throw new Error("Missing shared comment form");
  const changed = `${form.content} Changed in another form`;
  act(() => saveCollaborationDeliveryRecord({ ...form, content: changed }));
  expect(screen.getByLabelText("Add comment")).toHaveValue(changed);
  expect(screen.queryByRole("region", { name: "AI grouping preview" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Send comment" })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Preview delivery" }));
  expect(await screen.findByRole("region", { name: "AI grouping preview" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Send comment" })).toBeEnabled();
  act(() => {
    finishCollaborationDeliveryDraft(form.scope, form.draftId);
    saveCollaborationDeliveryRecord({ ...form, draftId: crypto.randomUUID(), content: changed });
  });
  expect(screen.getByLabelText("Add comment")).toHaveValue(changed);
  expect(screen.queryByRole("region", { name: "AI grouping preview" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Send comment" })).toBeDisabled();
  expect(fetch.mock.calls.filter(([url]) => String(url).endsWith("/comments"))).toHaveLength(0);
});

// The malformed/reciprocal grouping matrix belongs to core/content-schema.test.ts.
it("shows merged and later pending batches, then requires a fresh preview after suppression and editing", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const squadId = "00000000-0000-4000-8000-000000000005";
  const laterId = "00000000-0000-4000-8000-000000000006";
  const direct = { target_id: target, kind: "agent", resource_id: agent, name: "Leader", target_epoch: "1", config_version: "1", hold_reason: null, hold_version: "1", state: "waiting", code: "command_waiting", command_id: null };
  const squad = { ...direct, target_id: squadId, kind: "squad", resource_id: squadId, name: "Build squad" };
  const later = { ...squad, target_id: laterId, resource_id: laterId, name: "Review squad" };
  const candidates = [direct, squad, later];
  const fetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => {
    if (url.endsWith("mention-targets")) return new Response(JSON.stringify({ protocol_version: 2, issue_id: id, participants: [], can_submit_commands: true,
      targets: candidates.map((item) => ({ ...item, queue_state: "active", waiting_reason: "waiting", execution_available: false })) }));
    const body = JSON.parse(String(init.body));
    const firstBatch = [target, body.suppressed_target_ids.includes(squadId) ? laterId : squadId].filter((value) => !body.suppressed_target_ids.includes(value));
    const targets = candidates.map((item) => {
      if (body.suppressed_target_ids.includes(item.target_id)) return { ...item, state: "suppressed" };
      const merged = firstBatch.includes(item.target_id);
      return { ...item, grouping: { kind: "pending_batch", target_ids: merged ? firstBatch : [item.target_id], has_existing_inputs: merged, follows_pending_batch: !merged } };
    });
    return new Response(JSON.stringify({ protocol_version: 2, issue_id: id, lifecycle_epoch: "1", write_context: { participant_epoch: "2", account_epoch: "3" }, recipients: [], targets }));
  });
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentComposer scope={scope} issue={issue} /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Leader" }));
  await user.click(screen.getByRole("button", { name: "Build squad" }));
  await user.click(screen.getByRole("button", { name: "Review squad" }));
  const original = `[@Leader](mention://agent/${agent}) [@Build squad](mention://squad/${squadId}) [@Review squad](mention://squad/${laterId})`;
  await user.click(screen.getByRole("button", { name: "Preview delivery" }));
  const groups = within(await screen.findByRole("region", { name: "AI grouping preview" }));
  expect(groups.getByText("Leader, Build squad")).toBeInTheDocument();
  expect(groups.getByText("Expected to merge into one squad execution.")).toBeInTheDocument();
  expect(groups.getByText("Includes earlier waiting input.")).toBeInTheDocument();
  expect(groups.getByText("Follows an earlier batch in the same agent's queue.")).toBeInTheDocument();
  expect(groups.getByText("Preview only. Sending rechecks permissions and queue state.")).toBeInTheDocument();
  expect(screen.queryByText("Joined the current squad; waiting to be read.")).not.toBeInTheDocument();
  await user.click(screen.getByRole("checkbox", { name: /Build squad/ }));
  expect(screen.queryByRole("region", { name: "AI grouping preview" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Send comment" })).toBeDisabled();
  expect(screen.getByLabelText("Add comment")).toHaveValue(original);
  await user.click(screen.getByRole("button", { name: "Preview delivery" }));
  const updated = within(await screen.findByRole("region", { name: "AI grouping preview" }));
  expect(updated.queryByText(/Build squad/)).not.toBeInTheDocument();
  expect(updated.getByText("Leader, Review squad")).toBeInTheDocument();
  expect(updated.getByText("Expected to merge into one squad execution.")).toBeInTheDocument();
  expect(updated.queryByText("Follows an earlier batch in the same agent's queue.")).not.toBeInTheDocument();
  await user.type(screen.getByLabelText("Add comment"), " More detail");
  expect(screen.queryByRole("region", { name: "AI grouping preview" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Send comment" })).toBeDisabled();
  expect(fetch.mock.calls.filter(([url]) => String(url).endsWith("/comments"))).toHaveLength(0);
});

it("describes a possible squad continuation in Chinese before any comment is sent", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const candidate = { target_id: target, kind: "agent", resource_id: agent, name: "Leader", target_epoch: "1", config_version: "1", hold_reason: null, hold_version: "1" };
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async (url: string) => new Response(JSON.stringify(url.endsWith("mention-targets")
    ? { protocol_version: 2, issue_id: id, participants: [], can_submit_commands: true, targets: [{ ...candidate, queue_state: "active", waiting_reason: "waiting", execution_available: false }] }
    : { protocol_version: 2, issue_id: id, lifecycle_epoch: "1", recipients: [], targets: [{ ...candidate, state: "waiting", code: "command_waiting", command_id: null,
      grouping: { kind: "squad_continuation", target_ids: [target], has_existing_inputs: true, follows_pending_batch: false } }] }))));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentComposer scope={scope} issue={issue} /></QueryClientProvider>, { locale: "zh-Hans" });
  await userEvent.click(await screen.findByRole("button", { name: "Leader" }));
  await userEvent.click(screen.getByRole("button", { name: "预览发送" }));
  const groups = within(await screen.findByRole("region", { name: "AI 分组预览" }));
  expect(groups.getByText("预计并入当前 AI小队的下一次队长续作。")).toBeInTheDocument();
  expect(groups.getByText("包含已在等待队长读取的输入。")).toBeInTheDocument();
  expect(screen.queryByText("已并入当前小队，等待读取。")).not.toBeInTheDocument();
});

it("inserts only issue-authorized mentions, previews suppression, and confirms server delivery", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const candidate = { target_id: target, kind: "agent", resource_id: agent, target_epoch: "1", config_version: "1", hold_reason: null, hold_version: "1", state: "waiting", code: "command_waiting", command_id: null };
  const fetch = vi.fn().mockImplementation(async (url: string, init: RequestInit) => {
    if (url.endsWith("mention-targets")) return new Response(JSON.stringify({ protocol_version: 2, issue_id: id, participants: [account], can_submit_commands: false,
      targets: [{ target_id: target, kind: "agent", resource_id: agent, name: "Authorized agent", target_epoch: "1", config_version: "1", queue_state: "paused", hold_reason: null, hold_version: "1", waiting_reason: "queue_paused", execution_available: false }] }));
    const body = JSON.parse(String(init.body));
    if (url.endsWith("trigger-preview")) return new Response(JSON.stringify({ protocol_version: 2, issue_id: id, lifecycle_epoch: "1", write_context: { participant_epoch: "2", account_epoch: "3" }, recipients: [], targets: [{ ...candidate, state: body.suppressed_target_ids.length ? "suppressed" : "waiting" }] }));
    return new Response(JSON.stringify({ protocol_version: 2, issue_id: id, status: 201, code: "comment_created", comment_id: eventId, comment_revision: "1", event_id: eventId, event_seq: "1", targets: [{ ...candidate, state: "suppressed" }] }));
  });
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentComposer scope={scope} issue={issue} /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Authorized agent" }));
  expect(screen.getByLabelText("Add comment")).toHaveValue(`[@Authorized agent](mention://agent/${agent})`);
  await user.click(screen.getByRole("button", { name: "Preview delivery" }));
  await user.click(await screen.findByRole("checkbox"));
  expect(screen.getByRole("button", { name: "Send comment" })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Preview delivery" }));
  await user.click(screen.getByRole("button", { name: "Send comment" }));
  expect(await screen.findByText("Comment confirmed.")).toBeInTheDocument();
  const post = fetch.mock.calls.find(([url]) => String(url).endsWith("/comments"))!;
  expect(JSON.parse(String(post[1].body))).toMatchObject({ suppressed_target_ids: [target], expected_participant_epoch: "2" });
  expect(fetch.mock.calls.every(([url]) => !String(url).includes("/api/workspaces"))).toBe(true);
});

it("identifies notification recipients and rejected targets instead of calling them waiting", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async (url: string) => new Response(JSON.stringify(url.endsWith("mention-targets")
    ? { protocol_version: 2, issue_id: id, participants: [account], can_submit_commands: false, targets: [] }
    : { protocol_version: 2, issue_id: id, lifecycle_epoch: "1", recipients: [{ recipient: { ...account, name: "Notified participant" }, is_mention: true }],
      targets: [{ target_id: null, kind: "agent", resource_id: agent, state: "blocked", code: "target_unavailable", command_id: null, hold_reason: null, hold_version: null }] }))));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentComposer scope={scope} issue={issue} /></QueryClientProvider>);
  await userEvent.type(screen.getByLabelText("Add comment"), "Comment with a revoked target");
  await userEvent.click(screen.getByRole("button", { name: "Preview delivery" }));
  expect(await screen.findByText("Notified participant")).toBeInTheDocument();
  expect(screen.getByText("Target is unavailable or its configuration changed")).toBeInTheDocument();
  expect(screen.queryByText("Waiting")).toBeNull();
});

it("replies to an AI comment using its stable anchor without selecting any AI targets", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const replyTo = parseCollaborationComment({ id: target, issue_id: id, author_type: "agent", author: null, content: "Earlier AI output", type: "comment", parent_id: null,
    comment_revision: "1", created_at: "2026-10-19T00:00:00Z", updated_at: "2026-10-19T00:00:00Z", deleted_at: null, resolved_at: null, resolved_by: null, can_edit: false, can_resolve: true, reactions: [] })!;
  const fetch = vi.fn().mockImplementation(async (url: string, _init: RequestInit) => {
    if (url.endsWith(`/comments/${target}`)) return new Response(JSON.stringify(replyTo));
    if (url.endsWith("mention-targets")) return new Response(JSON.stringify({ protocol_version: 2, issue_id: id, participants: [account], can_submit_commands: false, targets: [] }));
    if (url.endsWith("trigger-preview")) return new Response(JSON.stringify({ protocol_version: 2, issue_id: id, lifecycle_epoch: "1", recipients: [], targets: [] }));
    return new Response(JSON.stringify({ protocol_version: 2, issue_id: id, status: 201, code: "comment_created", comment_id: eventId, comment_revision: "1", event_id: eventId, event_seq: "1", targets: [] }));
  });
  vi.stubGlobal("fetch", fetch);
  const onSent = vi.fn();
  const initial = renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentComposer scope={scope} issue={issue} replyTo={replyTo} onSent={onSent} /></QueryClientProvider>);
  await userEvent.type(screen.getByLabelText("Add comment"), "Thanks for the explanation");
  initial.unmount();
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentComposer scope={scope} issue={issue} onSent={onSent} /></QueryClientProvider>);
  expect(await screen.findByText("Replying to Agent")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Preview delivery" }));
  await userEvent.click(await screen.findByRole("button", { name: "Send comment" }));
  expect(await screen.findByText("Comment confirmed.")).toBeInTheDocument();
  for (const [url, init] of fetch.mock.calls) {
    if (url.endsWith("mention-targets") || url.endsWith(`/comments/${target}`)) continue;
    expect(JSON.parse(String(init.body))).toMatchObject({ parent_id: target, content: "Thanks for the explanation", suppressed_target_ids: [] });
    if (url.endsWith("/comments")) expect(JSON.parse(String(init.body)).target_versions).toEqual([]);
  }
  expect(onSent).toHaveBeenCalledOnce();
});
