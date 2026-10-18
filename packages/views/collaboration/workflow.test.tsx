import { expect, it, vi } from "vitest";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue, readCollaborationDrafts } from "@multica/core/collaboration";
import { renderWithI18n } from "../test/i18n";
import { CollaborationWorkflow } from "./workflow";
import { afterEach } from "vitest";

const id = "00000000-0000-4000-8000-000000000001";
const recipientId = "00000000-0000-4000-8000-000000000002";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const client = new QueryClient();
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); });
const account = { user_id: id, name: "Creator", avatar_url: "", external: true };
const caps = { reference: true, read: true, comment: true, control_ai: true, submit_command: true, invite: true, transfer: true, remove_participants: true, leave: false, edit: true, edit_source_structure: false, reorder: false, change_status: true, complete: true, cancel: true, reopen: false, resolve_thread: true, react: true };
const issue = parseCollaborationIssue({ id, title: "Transfer me", description: "Body", status: "todo", status_category: "todo", priority: "none", source_workspace: { id, name: "Source", identifier: "SRC-1" }, creator: account, handler: account,
  collaboration_revision: "9007199254740993", lifecycle_epoch: "1", write_context: { participant_epoch: "2", account_epoch: "3" }, session: { id, queue_state: "paused", control_version: "1" }, capabilities: caps })!;

it("resolves one exact recipient, preserves an unknown transfer, and recovers its original receipt", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ issue_id: id, source_workspace_id: id, recipient: { ...account, user_id: recipientId, name: "New handler" } })))
    .mockRejectedValueOnce(new TypeError("Connection lost"))
    .mockResolvedValueOnce(new Response(JSON.stringify({ code: "transferred", collaboration_revision: "9007199254740994", status: 200 })));
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={scope} issue={issue} /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Transfer" }));
  expect(screen.getByRole("button", { name: "Confirm" })).toBeDisabled();
  await user.type(screen.getByLabelText("Exact recipient"), "new-handler");
  await user.click(screen.getByRole("button", { name: "Find account" }));
  expect(await screen.findByText("Selected account: New handler")).toBeInTheDocument();
  await user.type(screen.getByLabelText("Note"), "Keep original note");
  await user.click(screen.getByRole("button", { name: "Confirm" }));
  expect(await screen.findByText(/The result is not confirmed/)).toBeInTheDocument();
  const [draft] = readCollaborationDrafts({ ...scope, issueId: id });
  expect(draft?.payload).toMatchObject({ targetUserId: recipientId, note: "Keep original note", expectedCollaborationRevision: "9007199254740993" });
  await user.click(screen.getByRole("button", { name: "Check original result" }));
  await waitFor(() => expect(readCollaborationDrafts({ ...scope, issueId: id })).toEqual([]));
  expect(fetch.mock.calls[2]?.[0]).toContain(`/operations/${draft!.requestId}`);
  expect(fetch.mock.calls[2]?.[1].method ?? "GET").toBe("GET");
  expect(screen.getByText("Action confirmed.")).toBeInTheDocument();
});

it("discards an in-flight exact lookup after the recipient text changes", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  let finish!: (response: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockImplementation(() => new Promise<Response>((resolve) => { finish = resolve; })));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={scope} issue={issue} /></QueryClientProvider>);
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Transfer" }));
  await user.type(screen.getByLabelText("Exact recipient"), "previous-account");
  await user.click(screen.getByRole("button", { name: "Find account" }));
  await user.clear(screen.getByLabelText("Exact recipient"));
  await user.type(screen.getByLabelText("Exact recipient"), "new-account");
  await act(async () => finish(new Response(JSON.stringify({ issue_id: id, source_workspace_id: id, recipient: { ...account, user_id: recipientId, name: "Previous account" } }))));
  expect(screen.queryByText("Selected account: Previous account")).toBeNull();
  expect(screen.getByRole("button", { name: "Confirm" })).toBeDisabled();
});

it("submits edits against their captured revision after a background refresh", async () => {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const editable = { ...issue, start_date: null, due_date: null };
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ status: 409, code: "stale_revision", collaboration_revision: "9007199254740994" }), { status: 409 }));
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
});
