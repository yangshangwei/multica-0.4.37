import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue, readCollaborationDrafts } from "@multica/core/collaboration";
import { collaborationControlReceiptFixture, collaborationExecutionIssueFixture, collaborationHostExecutionFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationExecutionPanel } from "./execution-panel";
import { NavigationProvider } from "../navigation";

const id = collaborationExecutionIssueFixture.id;
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); });

function mount(selectedRunId?: string, execution = collaborationExecutionIssueFixture.execution) {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const issue = parseCollaborationIssue({ ...collaborationExecutionIssueFixture, execution, write_context: { participant_epoch: "1", account_epoch: "1" } })!;
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

// Core owns record parsing, access epochs and page-cursor matrices. These tests
// exercise lazy disclosure, exact displayed telemetry and user-driven recovery.
it("opens recorded content lazily, retains the group context and retries the next page without losing the first", async () => {
  const first = recordedPage();
  const second = { ...first, next_cursor: null, items: [
    { id: "comment:one", kind: "comment", comment_id: id, body: "Original recorded comment" },
    { id: "activity:one", kind: "squad_activity", outcome: "no_action", body: "Recorded squad activity" },
    { id: "result:one", kind: "result", outcome: "success", body: "Recorded final result" },
  ] };
  const fetch = vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(first)))
    .mockResolvedValueOnce(new Response(JSON.stringify({ error: "temporarily unavailable" }), { status: 503 }))
    .mockResolvedValueOnce(new Response(JSON.stringify(second)));
  vi.stubGlobal("fetch", fetch);
  mount();
  expect(fetch).not.toHaveBeenCalled();
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
  expect(fetch.mock.calls[1]?.[0]).toContain("cursor=records-page-two");
  expect(fetch.mock.calls[2]?.[0]).toContain("cursor=records-page-two");
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
  vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response(JSON.stringify(page)))
    .mockResolvedValueOnce(new Response(JSON.stringify({ code: "not_found", error: "Not found" }), { status: 404 })));
  mount(page.run.id);
  expect(await screen.findByText("Frozen question")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Load more records" }));
  expect(await screen.findByText("You no longer have access to this issue.")).toBeInTheDocument();
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

it("recovers a waiting cancellation using its original key without turning it into Stop", async () => {
  const waiting = collaborationExecutionIssueFixture.execution.commands[1]!;
  let requestId = "";
  const fetch = vi.fn().mockImplementationOnce(async (_url: string, init: RequestInit) => {
    requestId = new Headers(init.headers).get("Idempotency-Key")!;
    throw new TypeError("Connection lost");
  }).mockImplementationOnce(async () => new Response(JSON.stringify({
    protocol_version: 2, issue_id: id, request_id: requestId, action: "cancel_command", status: 409, code: "command_not_waiting", redacted: true,
  })));
  vi.stubGlobal("fetch", fetch);
  const { scope } = mount();
  await userEvent.click(screen.getByRole("button", { name: "Cancel waiting input 2" }));
  expect(await screen.findByText(/The result is not confirmed/)).toBeInTheDocument();
  expect(fetch.mock.calls[0]?.[0]).toContain(`/commands/${waiting.id}/cancel`);
  await userEvent.click(screen.getByRole("button", { name: "Check original result" }));
  expect(await screen.findByText(/This input is no longer waiting/)).toBeInTheDocument();
  expect(fetch.mock.calls[1]?.[0]).toContain(`/commands/operations/${requestId}`);
  expect(fetch.mock.calls[1]?.[1]?.method ?? "GET").toBe("GET");
  expect(readCollaborationDrafts({ ...scope, issueId: id })).toEqual([]);
  expect(fetch.mock.calls.some(([url]) => String(url).endsWith("/control"))).toBe(false);
});
