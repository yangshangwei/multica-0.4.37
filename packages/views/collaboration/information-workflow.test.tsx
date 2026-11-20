import { afterEach, expect, it, vi } from "vitest";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue, readCollaborationDrafts } from "@multica/core/collaboration";
import { collaborationIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationWorkflow } from "./workflow";

// Core information-schema/action tests own malformed data and original-key
// recovery matrices. This suite owns accessible shared form wiring.
vi.mock("../rich-content", () => ({ RichContent: ({ content }: { content: string }) => <p>{content}</p> }));
const actor = collaborationIssueFixture.creator.user_id;
const recipient = "00000000-0000-4000-8000-000000000002";
const repairId = "00000000-0000-4000-8000-000000000003";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); localStorage.clear(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
const json = (data: unknown) => new Response(JSON.stringify(data));
const context = { protocol_version: 1, context_version: "a".repeat(64), active_request_id: null,
  historical_origin: null, pending_submission: null, human_selection_allowed: true };
function informationIssue(repair: boolean, patch: Record<string, unknown> = {}) {
  const value = parseCollaborationIssue({ ...collaborationIssueFixture, status: repair ? "blocked" : "in_progress", status_category: repair ? "blocked" : "in_progress",
    write_context: { participant_epoch: "2", account_epoch: "3" },
    capabilities: { ...collaborationIssueFixture.capabilities, complete: false, submit_acceptance: false,
      request_information: !repair, transfer: !repair, change_status: !repair },
    information_workflow: { protocol_version: 1, managed: true, context_version: context.context_version, active_request_id: null, record_state: "none",
      actions: { request: { available: !repair, code: repair ? "information_repair_required" : null },
        return: { available: false, code: "information_request_not_active" }, repair: { available: repair, code: repair ? null : "information_repair_not_applicable" } } }, ...patch });
  if (!value) throw new Error("Invalid shared information fixture");
  return value;
}
function mount(repair: boolean, issue = informationIssue(repair)) {
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); clients.push(client);
  return renderWithI18n(<QueryClientProvider client={client}><CollaborationWorkflow scope={collaborationScope(actor)} issue={issue} /></QueryClientProvider>);
}
async function selectRecipient(user: ReturnType<typeof userEvent.setup>, form: HTMLElement) {
  await user.click(within(form).getByRole("button", { name: "Recipient" }));
  await user.click(await screen.findByRole("option", { name: new RegExp(recipient) }));
}
function recipientPage() { return { issue_id: collaborationIssueFixture.id, source_workspace_id: collaborationIssueFixture.source_workspace.id,
  scope: "related", source_member: false, recent: [], next_cursor: null,
  recipients: [{ user_id: recipient, name: "Information recipient", avatar_url: "", external: null }] }; }

it("requires and sends the four managed information fields without changing the zoned ETA", async () => {
  const fetch = vi.fn(async (input: string, init?: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("status-options")) return json({ action: "request_information", status_options_version: "b".repeat(64), default_status: "blocked", options: [{ key: "blocked", name: "Blocked", category: "blocked" }] });
    if (url.pathname.endsWith("recipients/search")) return json(recipientPage());
    if (url.pathname.endsWith("handoffs") && init?.method === "POST") return json({ code: "information_requested", collaboration_revision: "9007199254740994" });
    throw new Error(`Unexpected shared test path ${url.pathname}`);
  }); vi.stubGlobal("fetch", fetch);
  mount(false); const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Request information" }));
  const form = screen.getByRole("form", { name: "Request information" });
  const confirm = within(form).getByRole("button", { name: "Confirm" });
  fireEvent.change(within(form).getByLabelText("Reason"), { target: { value: "Need the original receipt" } });
  await selectRecipient(user, form);
  expect(confirm).toBeDisabled();
  fireEvent.change(within(form).getByLabelText("Unblock condition"), { target: { value: "Verify the original receipt" } });
  expect(confirm).toBeDisabled();
  fireEvent.change(within(form).getByLabelText("Estimated resolution time"), { target: { value: "2026-11-20T18:00:00.123456+08:00" } });
  await waitFor(() => expect(confirm).toBeEnabled());
  await user.click(confirm);
  await screen.findByText("Action confirmed.");
  const writes = fetch.mock.calls.filter(([url, init]) => init?.method === "POST" && new URL(url).pathname.endsWith("/handoffs"));
  expect(writes).toHaveLength(1);
  expect(JSON.parse(String(writes[0]?.[1]?.body))).toMatchObject({ purpose: "information", target_user_id: recipient, note: "Need the original receipt",
    unblock_condition: "Verify the original receipt", estimated_resolution_at: "2026-11-20T18:00:00.123456+08:00" });
});

it("requires explicit repair basis, status and recipient before one atomic repair", async () => {
  const fetch = vi.fn(async (input: string, init?: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("status-options")) return json({ action: "repair_information", status_options_version: "b".repeat(64), default_status: "",
      options: [{ key: "todo", name: "To do", category: "todo" }], information_repair_context: context });
    if (url.pathname.endsWith("recipients/search")) return json(recipientPage());
    if (url.pathname.endsWith("handoffs") && init?.method === "POST") return json({ code: "information_repaired", collaboration_revision: "9007199254740994",
      information_repair_confirmation: { repair_handoff_id: repairId, prior_active_request_id: null, status: "todo", handler_user_id: recipient, superseded_count: "0" } });
    throw new Error(`Unexpected shared test path ${url.pathname}`);
  }); vi.stubGlobal("fetch", fetch);
  mount(true); const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Repair information block" }));
  const form = screen.getByRole("form", { name: "Repair information block" });
  const status = within(form).getByRole("combobox", { name: "Status" });
  await waitFor(() => expect(status).toHaveValue(""));
  fireEvent.change(within(form).getByLabelText("Repair reason"), { target: { value: "Restore an available handler" } });
  await selectRecipient(user, form);
  const confirm = within(form).getByRole("button", { name: "Confirm" });
  expect(confirm).toBeDisabled();
  await user.selectOptions(status, "todo");
  expect(confirm).toBeDisabled();
  await user.click(within(form).getByRole("radio", { name: "Choose a working status" }));
  await waitFor(() => expect(confirm).toBeEnabled());
  await user.click(confirm);
  await screen.findByText("Action confirmed.");
  const writes = fetch.mock.calls.filter(([url, init]) => init?.method === "POST" && new URL(url).pathname.endsWith("/handoffs"));
  expect(writes).toHaveLength(1);
  expect(new URL(writes[0]![0]).pathname).toMatch(/\/handoffs$/);
  expect(JSON.parse(String(writes[0]?.[1]?.body))).toMatchObject({ purpose: "information_repair", expected_status: "todo", target_user_id: recipient,
    information_repair: { context_version: context.context_version, active_request_id: null, basis: { kind: "human_selected" } } });
  expect(fetch.mock.calls.some(([url]) => /commands|resume|acceptance-submissions/.test(new URL(url).pathname))).toBe(false);
});

it("wires pending repair with closed generic capabilities to one explicit creator and submission", async () => {
  const submissionId = "00000000-0000-4000-8000-000000000006";
  const submissionEventId = "00000000-0000-4000-8000-000000000007";
  const submission = { id: submissionId, issue_id: collaborationIssueFixture.id, event_seq: "1", lifecycle_epoch: "1",
    author_user_id: recipient, author: null, created_at: "2026-11-18T08:00:00Z", result: "Retained result", basis: "Retained basis",
    attachment_ids: [], source_comment_id: null, evidence_state: "available", decision: null };
  const pendingContext = { ...context, human_selection_allowed: false,
    pending_submission: { submission_id: submissionId, submission_event_id: submissionEventId, creator_user_id: actor } };
  const fetch = vi.fn(async (input: string, init?: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("status-options")) return json({ action: "repair_information", status_options_version: "b".repeat(64), default_status: "",
      options: [{ key: "in_review", name: "In review", category: "in_review" }], information_repair_context: pendingContext });
    if (url.pathname.endsWith("handoffs") && init?.method === "POST") return json({ code: "information_repaired", collaboration_revision: "9007199254740994",
      information_repair_confirmation: { repair_handoff_id: repairId, prior_active_request_id: null, status: "in_review", handler_user_id: actor, superseded_count: "0" } });
    throw new Error(`Unexpected pending repair test path ${url.pathname}`);
  }); vi.stubGlobal("fetch", fetch);
  const pending = informationIssue(true, { delivery: { current: submission, latest: submission, has_history: true } });
  mount(true, pending); const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Repair information block" }));
  const form = screen.getByRole("form", { name: "Repair information block" });
  fireEvent.change(within(form).getByLabelText("Repair reason"), { target: { value: "Restore the exact pending submission" } });
  await user.click(within(form).getByRole("radio", { name: new RegExp("Restore this pending submission") }));
  await user.selectOptions(within(form).getByRole("combobox", { name: "Status" }), "in_review");
  const confirm = within(form).getByRole("button", { name: "Confirm" });
  expect(confirm).toBeDisabled();
  await user.click(within(form).getByRole("button", { name: "Use the creator" }));
  await waitFor(() => expect(confirm).toBeEnabled());
  await user.click(confirm);
  await screen.findByText("Action confirmed.");
  const writes = fetch.mock.calls.filter(([url, init]) => new URL(url).pathname.endsWith("/handoffs") && init?.method === "POST");
  expect(writes).toHaveLength(1);
  expect(JSON.parse(String(writes[0]?.[1]?.body))).toMatchObject({ purpose: "information_repair", target_user_id: actor, expected_status: "in_review",
    information_repair: { context_version: context.context_version, active_request_id: null,
      basis: { kind: "pending_submission", submission_id: submissionId, submission_event_id: submissionEventId } } });
  expect(pending.delivery?.current).toEqual(submission);
  expect(fetch.mock.calls.some(([url]) => /commands|resume|acceptance-submissions/.test(new URL(url).pathname))).toBe(false);
});

it("preserves the original repair after a recovered refusal and requires an explicit creator selection for new pending context", async () => {
  const submissionId = "00000000-0000-4000-8000-000000000006";
  const submissionEventId = "00000000-0000-4000-8000-000000000007";
  const pendingContext = { ...context, context_version: "c".repeat(64), human_selection_allowed: false,
    pending_submission: { submission_id: submissionId, submission_event_id: submissionEventId, creator_user_id: actor } };
  let latest = false;
  let firstWrite = true;
  const fetch = vi.fn(async (input: string, init?: RequestInit) => {
    const url = new URL(input);
    if (url.pathname.endsWith("status-options")) return json({ action: "repair_information", status_options_version: (latest ? "d" : "b").repeat(64), default_status: "",
      options: latest ? [{ key: "in_review", name: "In review", category: "in_review" }] : [{ key: "todo", name: "To do", category: "todo" }], information_repair_context: latest ? pendingContext : context });
    if (url.pathname.endsWith("recipients/search")) return json(recipientPage());
    if (url.pathname.includes("/operations/")) return json({ status: 409, code: "information_context_changed", collaboration_revision: "9007199254740994" });
    if (url.pathname.endsWith("handoffs") && init?.method === "POST") {
      if (firstWrite) { firstWrite = false; throw new TypeError("Lost original response"); }
      return json({ code: "information_repaired", collaboration_revision: "9007199254740995", information_repair_confirmation: {
        repair_handoff_id: repairId, prior_active_request_id: null, status: "in_review", handler_user_id: actor, superseded_count: "0" } });
    }
    throw new Error(`Unexpected retained repair test path ${url.pathname}`);
  }); vi.stubGlobal("fetch", fetch);
  const user = userEvent.setup(); const initial = mount(true);
  await user.click(screen.getByRole("button", { name: "Repair information block" }));
  let form = screen.getByRole("form", { name: "Repair information block" });
  fireEvent.change(within(form).getByLabelText("Repair reason"), { target: { value: "Keep the original repair reason" } });
  await selectRecipient(user, form);
  await user.selectOptions(within(form).getByRole("combobox", { name: "Status" }), "todo");
  await user.click(within(form).getByRole("radio", { name: "Choose a working status" }));
  await user.click(within(form).getByRole("button", { name: "Confirm" }));
  await within(form).findByRole("button", { name: "Check original result" });
  const original = readCollaborationDrafts({ ...collaborationScope(actor), issueId: collaborationIssueFixture.id })[0];
  expect(original).toBeDefined();
  initial.unmount(); latest = true;
  const current = informationIssue(true);
  mount(true, { ...current, collaboration_revision: "9007199254740994", information_workflow: { ...current.information_workflow!, context_version: pendingContext.context_version } });
  await user.click(screen.getByRole("button", { name: "Check original result" }));
  form = await screen.findByRole("form", { name: "Repair information block" });
  await waitFor(() => expect(within(form).getByRole("combobox", { name: "Status" })).toHaveValue("todo"));
  expect(within(form).getByLabelText("Repair reason")).toHaveValue("Keep the original repair reason");
  expect(within(form).getByText(context.context_version)).toBeInTheDocument();
  expect(form).toHaveTextContent(recipient);
  expect(within(form).getByRole("button", { name: "Confirm" })).toBeDisabled();
  expect(fetch.mock.calls.filter(([url, init]) => new URL(url).pathname.endsWith("/handoffs") && init?.method === "POST")).toHaveLength(1);
  expect(fetch.mock.calls.find(([url]) => new URL(url).pathname.includes("/operations/"))?.[0]).toContain(`/operations/${original!.requestId}`);
  await user.click(within(form).getByRole("button", { name: "Keep input and use current version" }));
  expect(within(form).getByRole("combobox", { name: "Status" })).toHaveValue("todo");
  expect(within(form).getByLabelText("Repair reason")).toHaveValue("Keep the original repair reason");
  await user.click(within(form).getByRole("radio", { name: new RegExp("Restore this pending submission") }));
  await user.selectOptions(within(form).getByRole("combobox", { name: "Status" }), "in_review");
  expect(within(form).getByRole("button", { name: "Confirm" })).toBeDisabled();
  await user.click(within(form).getByRole("button", { name: "Use the creator" }));
  await waitFor(() => expect(within(form).getByRole("button", { name: "Confirm" })).toBeEnabled());
  await user.click(within(form).getByRole("button", { name: "Confirm" }));
  await screen.findByText("Action confirmed.");
  const writes = fetch.mock.calls.filter(([url, init]) => new URL(url).pathname.endsWith("/handoffs") && init?.method === "POST");
  expect(writes).toHaveLength(2);
  expect(JSON.parse(String(writes[1]?.[1]?.body))).toMatchObject({ target_user_id: actor, expected_status: "in_review", information_repair: {
    context_version: pendingContext.context_version, active_request_id: null, basis: { kind: "pending_submission", submission_id: submissionId, submission_event_id: submissionEventId } } });
});
