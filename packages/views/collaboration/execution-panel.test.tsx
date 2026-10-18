import { afterEach, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue, readCollaborationDrafts } from "@multica/core/collaboration";
import { collaborationControlReceiptFixture, collaborationExecutionIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationExecutionPanel } from "./execution-panel";

const id = collaborationExecutionIssueFixture.id;
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }) } }));
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
afterEach(() => { client.clear(); localStorage.clear(); vi.unstubAllGlobals(); });

function mount() {
  setApiInstance(new ApiClient("https://server.example"));
  const scope = collaborationScope(id);
  const issue = parseCollaborationIssue({ ...collaborationExecutionIssueFixture, write_context: { participant_epoch: "1", account_epoch: "1" } })!;
  renderWithI18n(<QueryClientProvider client={client}><CollaborationExecutionPanel scope={scope} issue={issue} /></QueryClientProvider>);
  return { scope, issue };
}

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
