import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue, readCollaborationDrafts } from "@multica/core/collaboration";
import { collaborationCommandHistoryFixture, collaborationControlReceiptFixture, collaborationExecutionIssueFixture, collaborationHostExecutionFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationExecutionPanel } from "./execution-panel";
import { NavigationProvider } from "../navigation";

const id = collaborationExecutionIssueFixture.id;
const emptyHistory = { ...collaborationCommandHistoryFixture, items: [], next_cursor: null };
const emptyExecution = { ...collaborationHostExecutionFixture, channels: [], targets: [], commands: [], groups: [], runs: [] };
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); });

function mount(selectedRunId?: string, execution = collaborationExecutionIssueFixture.execution, aiPolicy?: "manual" | "assisted") {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const issue = parseCollaborationIssue({ ...collaborationExecutionIssueFixture, execution, ai_policy: aiPolicy, write_context: { participant_epoch: "1", account_epoch: "1" } })!;
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={{ pathname: "/account", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => `https://server.example${path}` }}><CollaborationExecutionPanel scope={scope} issue={issue} selectedRunId={selectedRunId} /></NavigationProvider></QueryClientProvider>);
  return { scope, issue };
}

it("does not mark enabled host execution as unavailable", () => {
  mount(undefined, collaborationHostExecutionFixture);
  expect(screen.queryByText("Shared AI execution is unavailable.")).not.toBeInTheDocument();
  expect(screen.queryByText("New AI execution is disabled.")).not.toBeInTheDocument();
  expect(screen.getByText("Standard host execution")).toBeInTheDocument();
});

it("distinguishes disabled new execution from retained host execution facts", () => {
  mount(undefined, { ...collaborationHostExecutionFixture,
    execution_gate: { ...collaborationHostExecutionFixture.execution_gate, enabled: false } });
  expect(screen.getByText("New AI execution is disabled.")).toBeInTheDocument();
  expect(screen.queryByText("Shared AI execution is unavailable.")).not.toBeInTheDocument();
  expect(screen.getByText("Execution state unconfirmed; cannot resume yet")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Stop all" })).toBeEnabled();
});

it("retains the unavailable message for a legacy execution view", () => {
  mount();
  expect(screen.getByText("Shared AI execution is unavailable.")).toBeInTheDocument();
});

function recordedPage() {
  const run = parseCollaborationIssue(collaborationExecutionIssueFixture)!.execution!.runs[1]!;
  return {
    protocol_version: 1, issue_id: id, run,
    resource: { agent_id: run.agent_id, runtime_id: id, resource_owner_user_id: id, execution_workspace_id: id, grant_id: id,
      provider: "Fixture provider", model: "Recorded model", account_source: "runtime_configuration" },
    recording: { status: "partial", inputs_recorded: true },
    usage: { input_tokens: "9007199254740993", output_tokens: "0", cache_read_tokens: null, cache_creation_tokens: null, total_tokens: null, cost_usd: "0.0000000001" },
    items: [
      { id: "input:one", kind: "input", command_id: run.command_id, author_user_id: id, body: "Frozen question", attachment_ids: [] },
      { id: "message:one", kind: "message", role: "assistant", body: "Recorded answer" },
      { id: "call:one", kind: "tool_call", call_id: "call-1", name: "read_file", body: "Recorded tool arguments" },
      { id: "tool:one", kind: "tool_result", call_id: "call-1", body: "Recorded tool output", is_error: false },
    ],
    next_cursor: "records-page-two",
  };
}

function stubRecordedReads(...responses: Response[]) {
  const records = vi.fn((_url: string) => Promise.resolve(responses.shift()!));
  const fetch = vi.fn((url: string) => {
    const path = new URL(url).pathname;
    if (path.endsWith("/records")) return records(url);
    // Handoff has an independent response queue; its boundary/error matrix is
    // covered in core/run-handoff-queries and the handoff component suite.
    if (path.endsWith("/handoff")) {
      const empty = { total: 0, returned: 0, truncated: false, items: [] };
      return Promise.resolve(new Response(JSON.stringify({ protocol_version: 1, issue_id: id, run_id: path.split("/").at(-2), mode: "live", event_seq: "1",
        authority: { status: "in_progress", handler_id: id }, assignment: { role: "agent", step_key: "agent" }, handoff: null, information_request: null,
        memory: { version: "0", availability: "absent" }, inputs: empty, results: empty,
        wakeup_causes: { ...empty, completeness: "unknown" }, sources: [], missing: [] })));
    }
    if (path.endsWith("/commands")) return Promise.resolve(new Response(JSON.stringify(emptyHistory)));
    if (path === `/api/collaboration/issues/${id}`) return Promise.resolve(new Response(JSON.stringify(collaborationExecutionIssueFixture)));
    throw new Error(`Unexpected read: ${path}`);
  });
  vi.stubGlobal("fetch", fetch);
  return { fetch, records };
}

// Core owns record parsing, access epochs and page-cursor matrices. These tests
// exercise lazy disclosure, exact displayed telemetry and user-driven recovery.
it("opens recorded content lazily, retains the group context and retries the next page without losing the first", async () => {
  const first = recordedPage();
  const second = { ...first, next_cursor: null, items: [
    { id: "comment:one", kind: "comment", comment_id: id, body: "Original recorded comment" },
    { id: "activity:one", kind: "squad_activity", outcome: "no_action", body: "Recorded squad activity" },
    { id: "result:one", kind: "result", outcome: "success", body: "Recorded final result" },
  ] };
  const { fetch, records: recordRequests } = stubRecordedReads(new Response(JSON.stringify(first)),
    new Response(JSON.stringify({ error: "temporarily unavailable" }), { status: 503 }), new Response(JSON.stringify(second)));
  mount();
  const summary = screen.getByText("AI execution", { selector: "summary" });
  expect(summary.closest("details")).toHaveAttribute("open");
  await userEvent.click(summary);
  expect(summary.closest("details")).not.toHaveAttribute("open");
  await userEvent.click(summary);
  expect(summary.closest("details")).toHaveAttribute("open");
  expect(recordRequests).not.toHaveBeenCalled();
  expect(fetch.mock.calls.some(([url]) => new URL(url).pathname.endsWith("/handoff"))).toBe(false);
  expect(screen.queryByText("Frozen question")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: `View records for ${first.run.id}` }));
  expect(await screen.findByText("Frozen question")).toBeInTheDocument();
  const records = screen.getByRole("region", { name: "Run records" });
  expect(within(records).getByText("9007199254740993")).toBeInTheDocument();
  expect(within(records).getByText("USD 0.0000000001")).toBeInTheDocument();
  expect(within(records).getByText("0")).toBeInTheDocument();
  expect(within(records).getByText("Partial recording")).toBeInTheDocument();
  expect(within(records).getByText("Group status").nextElementSibling).toHaveTextContent("Waiting for termination");
  expect(within(records).getByText("Run outcome").nextElementSibling).toHaveTextContent("Succeeded");
  expect(within(records).getByText("Recorded tool output")).toBeInTheDocument();
  await userEvent.click(within(records).getByRole("button", { name: "Load more records" }));
  expect(await within(records).findByRole("alert")).toBeInTheDocument();
  expect(within(records).getByText("Recorded answer")).toBeInTheDocument();
  await userEvent.click(within(records).getByRole("button", { name: "Refresh" }));
  expect(await within(records).findByText("Recorded final result")).toBeInTheDocument();
  expect(within(records).getAllByText("Frozen question")).toHaveLength(1);
  expect(within(records).queryByRole("button", { name: "Load more records" })).not.toBeInTheDocument();
  expect(within(records).getByRole("link", { name: "Open comment" })).toHaveAttribute("href", `/account?view=participating&issue=${id}#comment-${id}`);
  expect(recordRequests.mock.calls[1]?.[0]).toContain("cursor=records-page-two");
  expect(recordRequests.mock.calls[2]?.[0]).toContain("cursor=records-page-two");
});

it("discovers retained inputs behind an empty manual execution snapshot without rendering their bodies before history opens", async () => {
  const original = { ...collaborationCommandHistoryFixture.items[0]!, attachment_ids: [] };
  const fetch = vi.fn(async (url: string) => {
    const path = new URL(url).pathname;
    if (path.endsWith("/commands")) return new Response(JSON.stringify({ ...emptyHistory, items: [original] }));
    throw new Error(`Unexpected discovery request: ${path}`);
  });
  vi.stubGlobal("fetch", fetch);
  mount(undefined, emptyExecution, "manual");
  const historyButton = await screen.findByRole("button", { name: "Command history" });
  expect(screen.queryByText(original.content)).not.toBeInTheDocument();
  expect(fetch.mock.calls.some(([url]) => new URL(url).pathname.endsWith("/commands"))).toBe(true);
  expect(screen.getByRole("button", { name: "Stop all" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "Resume all" })).toBeDisabled();
  await userEvent.click(historyButton);
  const history = screen.getByRole("region", { name: "Command history" });
  expect(await within(history).findByText(original.content)).toBeInTheDocument();
  expect(within(history).getByText(original.author.name)).toBeInTheDocument();
  expect(within(history).getByText("Cancelled")).toBeInTheDocument();
  expect(fetch.mock.calls.every(([url]) => new URL(url).pathname.endsWith("/commands"))).toBe(true);
});

it("omits the empty manual console only after history confirms no retained input", async () => {
  const fetch = vi.fn(async (url: string) => {
    const path = new URL(url).pathname;
    if (path.endsWith("/commands")) return new Response(JSON.stringify(emptyHistory));
    throw new Error(`Unexpected discovery request: ${path}`);
  });
  vi.stubGlobal("fetch", fetch);
  mount(undefined, emptyExecution, "manual");
  await waitFor(() => expect(screen.queryByText("AI execution", { selector: "summary" })).not.toBeInTheDocument());
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole("button", { name: "Command history" })).not.toBeInTheDocument();
});

it("keeps empty-snapshot history discovery errors visible instead of asserting there is no AI history", async () => {
  vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({ error: "Unavailable" }), { status: 503 })));
  mount(undefined, emptyExecution, "manual");
  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(screen.getByText("AI execution", { selector: "summary" }).closest("details")).toHaveAttribute("open");
  expect(screen.getByRole("button", { name: "Refresh" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "Command history" })).toBeInTheDocument();
});

it("opens a linked run with a legacy result while showing missing input, recording and usage explicitly", async () => {
  const page = recordedPage();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...page,
    recording: { status: "not_recorded", inputs_recorded: false }, usage: null,
    resource: { ...page.resource, provider: null, model: null, account_source: null },
    items: [{ id: "legacy:result", kind: "result", outcome: "success", body: "Legacy final body" }], next_cursor: null,
  }))));
  mount(page.run.id);
  expect(await screen.findByText("Legacy final body")).toBeInTheDocument();
  expect(screen.getByText("Process not recorded")).toBeInTheDocument();
  expect(screen.getByText("The inputs read by this run were not recorded.")).toBeInTheDocument();
  expect(screen.getByText("Usage was not recorded.")).toBeInTheDocument();
  expect(screen.queryByText("0")).not.toBeInTheDocument();
});

it("removes the open transcript when a later page confirms access loss", async () => {
  const page = recordedPage();
  stubRecordedReads(new Response(JSON.stringify(page)),
    new Response(JSON.stringify({ code: "not_found", error: "Not found" }), { status: 404 }));
  mount(page.run.id);
  expect(await screen.findByText("Frozen question")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Load more records" }));
  expect(await screen.findByText("You no longer have access to this issue.")).toBeInTheDocument();
  expect(screen.getAllByRole("alert")).toHaveLength(1);
  await waitFor(() => expect(screen.queryByText("Frozen question")).not.toBeInTheDocument());
  expect(screen.queryByText("Recorded tool output")).not.toBeInTheDocument();
});

it("shows each partial Resume decision while preserving the live paused snapshot", async () => {
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async (_url: string, init: RequestInit) => new Response(JSON.stringify({ ...collaborationControlReceiptFixture, request_id: new Headers(init.headers).get("Idempotency-Key") }))));
  mount();
  await userEvent.click(screen.getByRole("button", { name: "Resume all" }));
  const receipt = await screen.findByRole("status", { name: "Control result" });
  expect(within(receipt).getByText("1 lanes resumed; 1 remain paused.")).toBeInTheDocument();
  expect(within(receipt).getByText(/Execution lane 2 · Termination unconfirmed/)).toBeInTheDocument();
  expect(within(receipt).getByText(/Resource authorization required/)).toBeInTheDocument();
  expect(screen.getAllByText("Paused").length).toBeGreaterThan(0);
});

it("counts a lane still behind its stop barrier as paused even when its queue was never paused", async () => {
  // A grant withdrawal stops the run without pausing its lane, so a later
  // Resume returns stop_unconfirmed on an active lane. The headline must
  // follow each lane's result, not its queue state.
  const [resumed, blocked] = collaborationControlReceiptFixture.channels;
  const activeBlocked = { ...blocked!, queue_state: "active" };
  const receipt = { ...collaborationControlReceiptFixture, channels: [resumed!, activeBlocked],
    targets: collaborationControlReceiptFixture.targets.map((target) => target.channel_id === activeBlocked.channel_id ? { ...target, queue_state: "active" } : target) };
  vi.stubGlobal("fetch", vi.fn().mockImplementation(async (_url: string, init: RequestInit) => new Response(JSON.stringify({ ...receipt, request_id: new Headers(init.headers).get("Idempotency-Key") }))));
  mount();
  await userEvent.click(screen.getByRole("button", { name: "Resume all" }));
  const result = await screen.findByRole("status", { name: "Control result" });
  expect(within(result).getByText("1 lanes resumed; 1 remain paused.")).toBeInTheDocument();
  expect(within(result).getByText(/Execution lane 2 · Termination unconfirmed/)).toBeInTheDocument();
});

it("recovers a waiting cancellation using its original key without turning it into Stop", async () => {
  const waiting = collaborationExecutionIssueFixture.execution.commands[1]!;
  let requestId = "";
  const cancellations = vi.fn(async (_url: string, init?: RequestInit) => {
    requestId = new Headers(init?.headers).get("Idempotency-Key")!;
    throw new TypeError("Connection lost");
  });
  const recoveries = vi.fn(async (_url: string, _init?: RequestInit) => new Response(JSON.stringify({
    protocol_version: 2, issue_id: id, request_id: requestId, action: "cancel_command", status: 409, code: "command_not_waiting", redacted: true,
  })));
  const fetch = vi.fn(async (url: string, init?: RequestInit) => {
    const path = new URL(url).pathname;
    if (path === `/api/collaboration/issues/${id}/commands/${waiting.id}/cancel`) return cancellations(url, init);
    if (path === `/api/collaboration/issues/${id}/commands/operations/${requestId}`) return recoveries(url, init);
    if (path.endsWith("/commands")) return new Response(JSON.stringify(emptyHistory));
    throw new Error(`Unexpected cancellation request: ${path}`);
  });
  vi.stubGlobal("fetch", fetch);
  const { scope } = mount();
  await userEvent.click(screen.getByRole("button", { name: "Cancel waiting input 2" }));
  expect(await screen.findByText(/The result is not confirmed/)).toBeInTheDocument();
  expect(cancellations).toHaveBeenCalledTimes(1);
  expect(cancellations.mock.calls[0]?.[1]?.method).toBe("POST");
  expect(readCollaborationDrafts({ ...scope, issueId: id })[0]?.requestId).toBe(requestId);
  await userEvent.click(screen.getByRole("button", { name: "Check original result" }));
  expect(await screen.findByText(/This input is no longer waiting/)).toBeInTheDocument();
  expect(recoveries).toHaveBeenCalledTimes(1);
  expect(recoveries.mock.calls[0]?.[0]).toContain(`/commands/operations/${requestId}`);
  expect(recoveries.mock.calls[0]?.[1]?.method ?? "GET").toBe("GET");
  expect(cancellations).toHaveBeenCalledTimes(1);
  expect(readCollaborationDrafts({ ...scope, issueId: id })).toEqual([]);
  expect(fetch.mock.calls.some(([url]) => String(url).endsWith("/control"))).toBe(false);
});
