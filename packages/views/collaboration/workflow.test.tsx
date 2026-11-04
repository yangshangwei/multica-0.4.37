import { expect, it, vi } from "vitest";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue, readCollaborationDrafts } from "@multica/core/collaboration";
import { renderWithI18n } from "../test/i18n";
import { CollaborationWorkflow } from "./workflow";
import { afterEach } from "vitest";

vi.mock("../rich-content", () => ({ RichContent: ({ content }: { content: string }) => <p>{content}</p> }));

const id = "00000000-0000-4000-8000-000000000001";
const recipientId = "00000000-0000-4000-8000-000000000002";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const client = new QueryClient();
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); });
const account = { user_id: id, name: "Creator", avatar_url: "", external: true };
const caps = { reference: true, read: true, comment: true, control_ai: true, submit_command: true, invite: true, transfer: true, remove_participants: true, leave: false, edit: true, edit_source_structure: false, reorder: false, change_status: true, complete: true, cancel: true, reopen: false, resolve_thread: true, react: true };
const issue = parseCollaborationIssue({ id, title: "Transfer me", description: "Body", status: "todo", status_category: "todo", priority: "none", source_workspace: { id, name: "Source", identifier: "SRC-1" }, creator: account, handler: account,
  collaboration_revision: "9007199254740993", lifecycle_epoch: "1", write_context: { participant_epoch: "2", account_epoch: "3" }, session: { id, queue_state: "paused", control_version: "1" }, capabilities: caps })!;

const submissionId = "00000000-0000-4000-8000-000000000009";
const submission = { id: submissionId, issue_id: id, event_seq: "8", lifecycle_epoch: "1", author_user_id: recipientId,
  author: { ...account, user_id: recipientId, name: "Result author" }, created_at: "2026-11-03T00:00:00Z", result: "Delivered outcome",
  basis: "Verified against the requested checks", attachment_ids: [], source_comment_id: null, evidence_state: "available" as const, decision: null };

// Server/Core own the permission, identity and lifecycle matrices. These tests
// verify the human forms send their explicit result and selected decision.
it("requires a result and basis for text-only delivery and preserves the input across dismissal", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const source = { ...issue, creator: submission.author, status: "in_progress", status_category: "in_progress" as const,
    ai_policy: "manual" as const, delivery: { current: null, latest: null, has_history: false },
    capabilities: { ...issue.capabilities, submit_acceptance: true, complete: false, return_acceptance: false, change_ai_policy: false } };
  const fetch = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>().mockImplementation(async (url) => new Response(JSON.stringify(new URL(url).pathname.endsWith("status-options")
    ? { action: "submit_acceptance", status_options_version: "a".repeat(64), default_status: "in_review", options: [{ key: "in_review", name: "In review", category: "in_review" }] }
    : { code: "acceptance_submitted", status: 200, collaboration_revision: "9007199254740994", delivery_confirmation: { submission_id: submissionId, attachment_ids: [] } })));
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={collaborationScope(id)} issue={source} /></QueryClientProvider>);
  const permissions = screen.getByLabelText("Current permissions");
  expect(permissions).toHaveTextContent("Current handler");
  expect(permissions).toHaveTextContent("Submit result");
  expect(permissions).not.toHaveTextContent("Accept and complete");
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: /Submit (result|for acceptance)/ }));
  await user.type(await screen.findByLabelText("Result"), "A text-only result");
  const form = screen.getByRole("form", { name: "Submit result" });
  expect(within(form).getByRole("button", { name: "Submit result" })).toBeDisabled();
  await user.type(screen.getByLabelText("Basis"), "Checked the requested behavior");
  await user.click(within(form).getByRole("button", { name: "Close" }));
  await user.click(screen.getByRole("button", { name: "Submit result" }));
  expect(screen.getByLabelText("Result")).toHaveValue("A text-only result");
  expect(screen.getByLabelText("Basis")).toHaveValue("Checked the requested behavior");
  await user.click(within(screen.getByRole("form", { name: "Submit result" })).getByRole("button", { name: "Submit result" }));
  expect(await screen.findByText("Action confirmed.")).toBeInTheDocument();
  const write = fetch.mock.calls.find(([url]) => new URL(url).pathname.endsWith("acceptance-submissions"));
  expect(JSON.parse(String(write?.[1]?.body))).toMatchObject({ note: "A text-only result", basis: "Checked the requested behavior", attachment_ids: [] });
});

it("accepts the exact observed submission and shows its author before completion", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const source = { ...issue, status: "in_review", status_category: "in_review" as const, ai_policy: "manual" as const,
    delivery: { current: submission, latest: submission, has_history: true }, capabilities: { ...issue.capabilities, return_acceptance: true, change_ai_policy: true } };
  const fetch = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>().mockImplementation(async (url) => new Response(JSON.stringify(new URL(url).pathname.endsWith("status-options")
    ? { action: "complete", status_options_version: "a".repeat(64), default_status: "done", options: [{ key: "done", name: "Done", category: "done" }] }
    : { code: "completed", status: 200, collaboration_revision: "9007199254740994", delivery_confirmation: { submission_id: submissionId, attachment_ids: [] } })));
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={collaborationScope(id)} issue={source} /></QueryClientProvider>);
  const permissions = screen.getByLabelText("Current permissions");
  expect(permissions).toHaveTextContent("Accept and complete");
  expect(permissions).not.toHaveTextContent("Record result and complete");
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: /Accept and complete|^Complete$/ }));
  const form = await screen.findByRole("form", { name: "Accept and complete" });
  expect(within(form).getByText("Result author")).toBeInTheDocument();
  expect(within(form).getByText("Delivered outcome")).toBeInTheDocument();
  expect(within(form).queryByLabelText("Result")).not.toBeInTheDocument();
  await user.click(within(form).getByRole("button", { name: "Accept and complete" }));
  expect(await screen.findByText("Action confirmed.")).toBeInTheDocument();
  const write = fetch.mock.calls.find(([url]) => new URL(url).pathname.endsWith("lifecycle"));
  expect(JSON.parse(String(write?.[1]?.body))).toMatchObject({ action: "complete", submission_id: submissionId });
});

it("returns the exact result with a reason and defaults to its actual author", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const source = { ...issue, status: "in_review", status_category: "in_review" as const, ai_policy: "manual" as const,
    delivery: { current: submission, latest: submission, has_history: true }, capabilities: { ...issue.capabilities, return_acceptance: true, change_ai_policy: true } };
  const fetch = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>().mockImplementation(async (url) => new Response(JSON.stringify(new URL(url).pathname.endsWith("status-options")
    ? { action: "return_acceptance", status_options_version: "a".repeat(64), default_status: "in_progress", options: [{ key: "in_progress", name: "In progress", category: "in_progress" }] }
    : { code: "acceptance_returned", status: 200, collaboration_revision: "9007199254740994" })));
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={collaborationScope(id)} issue={source} /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Return for revision" }));
  const form = await screen.findByRole("form", { name: "Return for revision" });
  expect(within(form).getByText("Selected account: Result author")).toBeInTheDocument();
  expect(within(form).getByRole("button", { name: "Return for revision" })).toBeDisabled();
  await user.type(within(form).getByLabelText("Revision reason"), "Cover the missing case");
  await user.click(within(form).getByRole("button", { name: "Return for revision" }));
  expect(await screen.findByText("Action confirmed.")).toBeInTheDocument();
  const write = fetch.mock.calls.find(([url]) => new URL(url).pathname.endsWith("acceptance-returns"));
  expect(JSON.parse(String(write?.[1]?.body))).toMatchObject({ submission_id: submissionId, target_user_id: recipientId, note: "Cover the missing case" });
});

it("records a self-handled result and completes without a transfer form", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const source = { ...issue, ai_policy: "manual" as const, delivery: { current: null, latest: null, has_history: false } };
  const fetch = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>().mockImplementation(async (url) => new Response(JSON.stringify(new URL(url).pathname.endsWith("status-options")
    ? { action: "complete", status_options_version: "a".repeat(64), default_status: "done", options: [{ key: "done", name: "Done", category: "done" }] }
    : { code: "completed", status: 200, collaboration_revision: "9007199254740994", delivery_confirmation: { submission_id: submissionId, attachment_ids: [] } })));
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={collaborationScope(id)} issue={source} /></QueryClientProvider>);
  const permissions = screen.getByLabelText("Current permissions");
  expect(permissions).toHaveTextContent("Record result and complete");
  expect(permissions).not.toHaveTextContent("Accept and complete");
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Record result and complete" }));
  const form = await screen.findByRole("form", { name: "Record result and complete" });
  expect(within(form).queryByRole("button", { name: "Recipient" })).not.toBeInTheDocument();
  await user.type(within(form).getByLabelText("Result"), "Finished my own work");
  await user.type(within(form).getByLabelText("Basis"), "Checked the requested result");
  await user.click(within(form).getByRole("button", { name: "Record result and complete" }));
  expect(await screen.findByText("Action confirmed.")).toBeInTheDocument();
  const write = fetch.mock.calls.find(([url]) => new URL(url).pathname.endsWith("lifecycle"));
  expect(JSON.parse(String(write?.[1]?.body))).toMatchObject({ action: "complete", note: "Finished my own work", basis: "Checked the requested result", attachment_ids: [] });
  expect(fetch.mock.calls.some(([url]) => new URL(url).pathname.endsWith("handoffs"))).toBe(false);
});

it("shows the current-viewer context beside Participants and opens the scoped list by keyboard in Chinese", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const participant = { ...account, user_id: recipientId, name: "Current participant", account_active: true };
  const source = { ...issue, creator: participant, handler: participant, write_context: null,
    capabilities: { ...issue.capabilities, comment: false, edit: false, complete: false } };
  const fetch = vi.fn(async (url: string) => {
    if (new URL(url).pathname.endsWith("/participants")) return new Response(JSON.stringify({
      protocol_version: 1, issue_id: id, collaboration_revision: source.collaboration_revision, participants: [participant],
    }));
    throw new Error("Unexpected participant context request");
  });
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={collaborationScope(id)} issue={source} /></QueryClientProvider>, { locale: "zh-Hans" });
  const permissions = screen.getByLabelText("当前权限");
  expect(within(permissions).getByText("查看者")).toBeInTheDocument();
  expect(permissions).not.toHaveTextContent("验收并完成");
  expect(permissions).not.toHaveTextContent("编辑任务");
  expect(fetch).not.toHaveBeenCalled();
  const trigger = screen.getByRole("button", { name: "参与者" });
  trigger.focus();
  await userEvent.setup().keyboard("{Enter}");
  expect(trigger).toHaveAttribute("aria-expanded", "true");
  const list = await screen.findByRole("list", { name: "参与者" });
  expect(await within(list).findByText("Current participant")).toBeInTheDocument();
  expect(within(list).getByText("发起人")).toBeInTheDocument();
  expect(within(list).getByText("当前处理人")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "移除 Current participant" })).not.toBeInTheDocument();
  expect(fetch).toHaveBeenCalledTimes(1);
});

async function chooseExactRecipient(user: ReturnType<typeof userEvent.setup>, value: string) {
  await user.click(screen.getByRole("button", { name: "Recipient" }));
  await user.type(screen.getByPlaceholderText("Search recipients"), "recipient");
  await user.click(await screen.findByRole("button", { name: "Find by full username, email, or account ID" }));
  await user.type(screen.getByLabelText("Find by full username, email, or account ID"), value);
  await user.click(screen.getByRole("button", { name: "Find account" }));
  await user.click(await screen.findByRole("button", { name: "Use this account" }));
}

it("resolves one exact recipient, preserves an unknown transfer, and recovers its original receipt", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const fetch = vi.fn<(input: string, init?: RequestInit) => Promise<Response>>(async (input) => {
    if (input.includes("/recipients/search")) return new Response(JSON.stringify({ issue_id: id, source_workspace_id: id, scope: "directory", source_member: true, recent: [], recipients: [], next_cursor: null }));
    if (input.includes("/recipients/resolve")) return new Response(JSON.stringify({ issue_id: id, source_workspace_id: id, recipient: { ...account, user_id: recipientId, name: "New handler" } }));
    if (input.endsWith("/handoffs")) throw new TypeError("Connection lost");
    if (input.includes("/operations/")) return new Response(JSON.stringify({ code: "transferred", collaboration_revision: "9007199254740994", status: 200 }));
    throw new Error(`Unexpected request ${input}`);
  });
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={scope} issue={issue} /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Transfer" }));
  expect(screen.getByRole("button", { name: "Confirm" })).toBeDisabled();
  await chooseExactRecipient(user, "new-handler");
  expect(await screen.findByText("Selected account: New handler")).toBeInTheDocument();
  await user.type(screen.getByLabelText("Note"), "Keep original note");
  await user.click(screen.getByRole("button", { name: "Confirm" }));
  expect(await screen.findByText(/The result is not confirmed/)).toBeInTheDocument();
  const [draft] = readCollaborationDrafts({ ...scope, issueId: id });
  expect(draft?.payload).toMatchObject({ targetUserId: recipientId, note: "Keep original note", expectedCollaborationRevision: "9007199254740993" });
  await user.click(screen.getByRole("button", { name: "Check original result" }));
  await waitFor(() => expect(readCollaborationDrafts({ ...scope, issueId: id })).toEqual([]));
  const recoveryCall = fetch.mock.calls.find(([input]) => input.includes(`/operations/${draft!.requestId}`));
  expect(recoveryCall?.[0]).toContain(`/operations/${draft!.requestId}`);
  expect(recoveryCall?.[1]?.method ?? "GET").toBe("GET");
  expect(screen.getByText("Action confirmed.")).toBeInTheDocument();
});

it("discards an in-flight exact lookup after the recipient text changes", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  let finish!: (response: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockImplementation((input: string) => input.includes("/recipients/search")
    ? Promise.resolve(new Response(JSON.stringify({ issue_id: id, source_workspace_id: id, scope: "directory", source_member: true, recent: [], recipients: [], next_cursor: null })))
    : new Promise<Response>((resolve) => { finish = resolve; })));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={scope} issue={issue} /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Transfer" }));
  await user.click(screen.getByRole("button", { name: "Recipient" }));
  await user.type(screen.getByPlaceholderText("Search recipients"), "recipient");
  await user.click(await screen.findByRole("button", { name: "Find by full username, email, or account ID" }));
  const exact = screen.getByLabelText("Find by full username, email, or account ID");
  await user.type(exact, "previous-account");
  await user.click(screen.getByRole("button", { name: "Find account" }));
  await user.clear(exact);
  await user.type(exact, "new-account");
  await act(async () => finish(new Response(JSON.stringify({ issue_id: id, source_workspace_id: id, recipient: { ...account, user_id: recipientId, name: "Previous account" } }))));
  expect(screen.queryByText("Selected account: Previous account")).toBeNull();
  expect(screen.getByRole("button", { name: "Confirm" })).toBeDisabled();
});

// The receipt/error and access-epoch matrices live in core/collaboration/operation.test.ts.
it("shows a real stale-revision refusal immediately and keeps input for an explicit new submit", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const editable = { ...issue, start_date: null, due_date: null };
  const fetch = vi.fn()
    .mockResolvedValueOnce(new Response(JSON.stringify({ error: "stale_revision", code: "stale_revision", collaboration_revision: "9007199254740994" }), { status: 409 }))
    .mockResolvedValueOnce(new Response(JSON.stringify({ code: "issue_edited", collaboration_revision: "9007199254740995" })));
  vi.stubGlobal("fetch", fetch);
  const ui = (value: typeof editable) => <QueryClientProvider client={client}><CollaborationWorkflow scope={scope} issue={value} /></QueryClientProvider>;
  const { rerender } = renderWithI18n(ui(editable));
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Edit issue" }));
  await user.clear(screen.getByLabelText("Title"));
  await user.type(screen.getByLabelText("Title"), "My retained title");
  rerender(ui({ ...editable, title: "Someone else's title", description: "New remote body", collaboration_revision: "9007199254740994" }));
  await user.click(screen.getByRole("button", { name: "Confirm" }));
  await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
  expect(JSON.parse(fetch.mock.calls[0]?.[1]?.body)).toMatchObject({ title: "My retained title", expected_collaboration_revision: "9007199254740993" });
  expect(screen.getByLabelText("Title")).toHaveValue("My retained title");
  expect(await screen.findByText(/The server did not apply this action/)).toBeInTheDocument();
  expect(screen.queryByText("Could not load collaboration. Try again.")).toBeNull();
  expect(screen.queryByText(/The result is not confirmed/)).toBeNull();
  expect(screen.queryByRole("button", { name: "Check original result" })).toBeNull();
  expect(readCollaborationDrafts({ ...scope, issueId: id })).toEqual([]);
  expect(fetch).toHaveBeenCalledTimes(1);
  await user.click(screen.getByRole("button", { name: "Keep input and use current version" }));
  expect(screen.getByLabelText("Title")).toHaveValue("My retained title");
  expect(fetch).toHaveBeenCalledTimes(1);
  await user.click(screen.getByRole("button", { name: "Confirm" }));
  expect(await screen.findByText("Action confirmed.")).toBeInTheDocument();
  expect(fetch).toHaveBeenCalledTimes(2);
  expect(JSON.parse(fetch.mock.calls[1]?.[1]?.body)).toMatchObject({ title: "My retained title", expected_collaboration_revision: "9007199254740994" });
  expect(new Headers(fetch.mock.calls[1]?.[1]?.headers).get("Idempotency-Key")).not.toBe(new Headers(fetch.mock.calls[0]?.[1]?.headers).get("Idempotency-Key"));
});

it("removes the selected participant only after confirmation with the captured revision", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const participant = { ...account, user_id: recipientId, name: "Current developer", account_active: false };
  const fetch = vi.fn().mockImplementation(async (url: string) => url.endsWith("/participants")
    ? new Response(JSON.stringify({ protocol_version: 1, issue_id: id, collaboration_revision: issue.collaboration_revision, participants: [{ ...account, account_active: true }, participant] }))
    : new Response(JSON.stringify({ status: 409, code: "stale_revision", collaboration_revision: "9007199254740994" }), { status: 409 }));
  vi.stubGlobal("fetch", fetch);
  const original = { ...issue, handler: participant };
  const ui = (value: typeof original) => <QueryClientProvider client={client}><CollaborationWorkflow scope={scope} issue={value} /></QueryClientProvider>;
  const { rerender } = renderWithI18n(ui(original));
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Participants" }));
  await user.click(await screen.findByRole("button", { name: "Remove Current developer" }));
  expect(screen.getByText("Account unavailable")).toBeInTheDocument();
  expect(screen.getByRole("alertdialog", { name: "Remove Current developer?" })).toBeInTheDocument();
  expect(screen.getByText(/Handling will return to the creator/)).toBeInTheDocument();
  expect(fetch.mock.calls.filter(([, init]) => init?.method === "POST")).toHaveLength(0);
  rerender(ui({ ...original, collaboration_revision: "9007199254740994" }));
  await user.click(screen.getByRole("button", { name: "Remove participant" }));
  await waitFor(() => expect(fetch.mock.calls.some(([url]) => url.endsWith(`/participants/${recipientId}/remove`))).toBe(true));
  const call = fetch.mock.calls.find(([url]) => url.endsWith(`/participants/${recipientId}/remove`));
  expect(JSON.parse(call?.[1]?.body)).toMatchObject({ expected_collaboration_revision: "9007199254740993" });
  expect(await screen.findByText(/The server did not apply this action/)).toBeInTheDocument();
});

// The person returning information answers the open request, so the return
// form shows that request's note and requester, and says when the saved status
// is gone and the issue falls back to another one.
function askedIssue(request: { note: string; return_status: string; return_status_fallback: boolean }) {
  const requester = { ...account, user_id: recipientId, name: "Requester" };
  return parseCollaborationIssue({ id, title: "Needs input", description: "Body", status: "blocked", status_category: "blocked", priority: "none", source_workspace: { id, name: "Source", identifier: "SRC-1" }, creator: requester, handler: account,
    collaboration_revision: "9007199254740993", lifecycle_epoch: "1", write_context: { participant_epoch: "2", account_epoch: "3" }, session: { id, queue_state: "paused", control_version: "1" },
    capabilities: { ...caps, return_information: true },
    current_information_request: { id: "00000000-0000-4000-8000-000000000009", requester, recipient: account, created_at: "2026-10-29T00:00:00Z", ...request } })!;
}

async function openReturnForm(issue: ReturnType<typeof askedIssue>) {
  setApiInstance(new ApiClient("https://server.example"));
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async () => new Response(JSON.stringify({ action: "return_information", status_options_version: "a".repeat(64), default_status: "in_progress",
    options: [{ key: "in_progress", name: "In progress", category: "in_progress" }] }))));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={collaborationScope(id)} issue={issue} /></QueryClientProvider>);
  await userEvent.setup().click(screen.getByRole("button", { name: "Return information" }));
}

it("shows the open information request to the person returning it", async () => {
  const asked = askedIssue({ note: "Which database version did you test against?", return_status: "in_progress", return_status_fallback: false });
  expect(asked.current_information_request?.note).toBe("Which database version did you test against?");
  await openReturnForm(asked);
  expect(await screen.findByText(/Which database version did you test against\?/)).toBeInTheDocument();
  expect(screen.getByText("Requested by Requester")).toBeInTheDocument();
  expect(screen.getByLabelText("Status")).toHaveValue("in_progress");
  expect(screen.getByLabelText("Note")).toHaveAccessibleDescription(/Which database version did you test against\?/);
  expect(screen.queryByText(/no longer available/)).toBeNull();
});

it("says where the issue returns when the request's saved status is gone", async () => {
  await openReturnForm(askedIssue({ note: "", return_status: "in_progress", return_status_fallback: true }));
  expect(await screen.findByText("The status saved with this request is no longer available, so the issue returns to In progress.")).toBeInTheDocument();
  expect(screen.getByText("The request has no note.")).toBeInTheDocument();
});
