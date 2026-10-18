import { afterEach, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue } from "@multica/core/collaboration";
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
