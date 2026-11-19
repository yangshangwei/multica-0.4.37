import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue, type CollaborationRunRecordItem, type CollaborationRunRecords } from "@multica/core/collaboration";
import { collaborationExecutionIssueFixture } from "@multica/core/collaboration/test-fixtures";
import { NavigationProvider } from "../navigation";
import { renderWithI18n } from "../test/i18n";
import { CollaborationRunDisclosure } from "./run-records";

// Core attachment-schema/run-records-schema tests own provenance and output bounds;
// attachment-panel.test.tsx owns metadata failure, missing-file and access matrices.
// This suite checks retained record references through the real shared file controls.
const auth = vi.hoisted(() => ({ user: { id: "00000000-0000-4000-8000-000000000001" } }));
vi.mock("@multica/core/auth", () => ({ useAuthStore: Object.assign(
  (selector?: (state: typeof auth) => unknown) => selector ? selector(auth) : auth,
  { getState: () => auth },
) }));

const issue = parseCollaborationIssue(collaborationExecutionIssueFixture)!;
const run = issue.execution!.runs[1]!;
const group = issue.execution!.groups.find((value) => value.id === run.group_id)!;
const time = "2026-11-19T00:00:00Z";
const inputFile = { id: "00000000-0000-4000-8000-0000000000a1", draft_id: "00000000-0000-4000-8000-0000000000a2",
  issue_id: issue.id, intended_issue_id: issue.id, source_workspace_id: issue.source_workspace.id, uploader_user_id: auth.user.id,
  name: "human-input.txt", content_type: "text/plain", size_bytes: 11, version: "1", created_at: time, delivered_at: time };
const outputFile = { ...inputFile, id: "00000000-0000-4000-8000-0000000000b1", draft_id: null, uploader_user_id: null,
  uploader_agent_id: run.agent_id, run_id: run.id, attempt_seq: "1", name: "agent-output.txt", size_bytes: 14 };
const commentId = "00000000-0000-4000-8000-0000000000c1";
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
afterEach(() => { client.clear(); localStorage.clear(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

function mount(items: CollaborationRunRecordItem[]) {
  const api = new ApiClient("https://server.example");
  setApiInstance(api);
  const page: CollaborationRunRecords = { protocol_version: 1, issue_id: issue.id, run,
    resource: { agent_id: run.agent_id, runtime_id: issue.id, resource_owner_user_id: auth.user.id,
      execution_workspace_id: issue.source_workspace.id, grant_id: issue.id, provider: null, model: null, account_source: null },
    recording: { status: "partial", inputs_recorded: true }, usage: null, items, next_cursor: null };
  vi.spyOn(api, "getCollaborationRunRecords").mockResolvedValue(page);
  vi.spyOn(api, "getCollaborationRunHandoff").mockResolvedValue(null);
  vi.spyOn(api, "getCollaborationIssue").mockResolvedValue(issue);
  const getFiles = vi.spyOn(api, "getCollaborationAttachments").mockResolvedValue({ issue_id: issue.id, attachments: [inputFile, outputFile] });
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={{ pathname: "/account", searchParams: new URLSearchParams(),
    hash: `#run-${run.id}`, push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: (path) => `https://server.example${path}` }}>
    <ol><CollaborationRunDisclosure scope={collaborationScope(auth.user.id)} issue={issue} run={run} group={group} selectedRunId={run.id} /></ol>
  </NavigationProvider></QueryClientProvider>);
  return { api, getFiles };
}

it("renders Agent comment and frozen result outputs with the existing protected preview and download controls", async () => {
  const { api, getFiles } = mount([
    { id: "comment:output", kind: "comment", comment_id: commentId, body: "Original Agent comment", attachment_ids: [outputFile.id] },
    { id: "result:output", kind: "result", outcome: "success", body: "Original frozen result", attachment_ids: [outputFile.id] },
  ]);
  const commentRow = (await screen.findByText("Original Agent comment")).closest<HTMLElement>("[data-record-id]")!;
  const resultRow = (await screen.findByText("Original frozen result")).closest<HTMLElement>("[data-record-id]")!;
  expect(await within(commentRow).findByRole("button", { name: "Preview agent-output.txt" })).toBeVisible();
  expect(within(resultRow).getByRole("button", { name: "Download agent-output.txt" })).toBeVisible();
  expect(within(commentRow).getByRole("link", { name: "Open comment" })).toHaveAttribute("href", `/account?view=participating&issue=${issue.id}#comment-${commentId}`);
  expect(screen.queryByText("human-input.txt")).not.toBeInTheDocument();
  expect(getFiles).toHaveBeenCalledTimes(1);

  const bytes = vi.spyOn(api, "getCollaborationAttachmentBytes").mockImplementation(async () => new Response("original bytes").blob());
  await userEvent.click(within(resultRow).getByRole("button", { name: "Preview agent-output.txt" }));
  expect(await screen.findByText("original bytes")).toBeVisible();
  expect(bytes).toHaveBeenCalledWith(issue.id, outputFile.id, "content", expect.objectContaining({ signal: expect.anything() }));
  await userEvent.click(screen.getByRole("button", { name: "Close preview" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());

  const capability = { issue_id: issue.id, attachment_id: outputFile.id, version: outputFile.version,
    url: `https://server.example/api/collaboration/issues/${issue.id}/attachments/${outputFile.id}/capability?cap=synthetic`, expires_at: new Date(Date.now() + 30_000).toISOString() };
  const getCapability = vi.spyOn(api, "getCollaborationAttachmentCapability").mockResolvedValue(capability);
  const download = vi.fn().mockResolvedValue(undefined);
  vi.stubGlobal("desktopAPI", { downloadURL: download });
  await userEvent.click(within(commentRow).getByRole("button", { name: "Download agent-output.txt" }));
  await vi.waitFor(() => expect(download).toHaveBeenCalledWith(capability.url));
  expect(getCapability).toHaveBeenCalledWith(issue.id, outputFile.id, outputFile.version, "download");
});

it("keeps existing frozen input files and legacy comment/result bodies without inferring unrelated outputs", async () => {
  mount([
    { id: "input:legacy", kind: "input", command_id: run.command_id, author_user_id: auth.user.id, body: "Frozen human input", attachment_ids: [inputFile.id] },
    { id: "comment:legacy", kind: "comment", comment_id: commentId, body: "Legacy retained comment" },
    { id: "result:legacy", kind: "result", outcome: "failure", body: "Legacy retained result" },
  ]);
  const inputRow = (await screen.findByText("Frozen human input")).closest<HTMLElement>("[data-record-id]")!;
  expect(await within(inputRow).findByRole("button", { name: "Preview human-input.txt" })).toBeVisible();
  expect(within(inputRow).getByRole("button", { name: "Download human-input.txt" })).toBeVisible();
  expect(within(inputRow).getByText(inputFile.id)).toBeVisible();
  expect(screen.getByText("Legacy retained comment")).toBeVisible();
  expect(screen.getByText("Legacy retained result")).toBeVisible();
  expect(screen.queryByText("agent-output.txt")).not.toBeInTheDocument();
  expect(screen.getAllByRole("region", { name: "Attachments" })).toHaveLength(1);
});

it("does not request file metadata for legacy records with no attachment references", async () => {
  const { getFiles } = mount([
    { id: "input:empty", kind: "input", command_id: run.command_id, author_user_id: auth.user.id, body: "Text input", attachment_ids: [] },
    { id: "comment:empty", kind: "comment", comment_id: commentId, body: "Legacy text comment" },
    { id: "result:empty", kind: "result", outcome: "success", body: "Legacy text result" },
  ]);
  expect(await screen.findByText("Legacy text result")).toBeVisible();
  expect(screen.queryByRole("region", { name: "Attachments" })).not.toBeInTheDocument();
  expect(getFiles).not.toHaveBeenCalled();
});
