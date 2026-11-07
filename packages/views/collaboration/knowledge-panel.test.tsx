// Core owns wire/lease matrices; this suite tests shared controls and their real query/action wiring.
import { webcrypto } from "node:crypto";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, ApiError, setApiInstance } from "@multica/core/api";
import { projectKeys } from "@multica/core/projects/queries";
import type { Project, Workspace } from "@multica/core/types";
import * as collaboration from "@multica/core/collaboration";
import { collaborationKnowledgeKeys, collaborationScope, parseCollaborationComment, parseCollaborationComments, parseCollaborationIssue,
  parseCollaborationKnowledgeCandidates, parseCollaborationKnowledgeDetail, parseCollaborationKnowledgeTargetList, CollaborationExecutionSchema, parseCollaborationRunRecords, readPendingKnowledgeOperations, expireKnowledgeProjections,
  type CollaborationKnowledgeCandidate, type CollaborationKnowledgeReceipt, type CollaborationScope } from "@multica/core/collaboration";
import { collaborationIssueFixture, collaborationExecutionFixture, knowledgeCandidateFixture, knowledgeUnavailableCandidateFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { CollaborationKnowledgePanel, ManagedKnowledgePanel } from "./knowledge-panel";

const auth = vi.hoisted(() => ({ userId: "00000000-0000-4000-8000-000000000001" }));
vi.mock("@multica/core/auth", () => ({ useAuthStore: Object.assign((selector: (state: unknown) => unknown) => selector({ user: { id: auth.userId }, status: "authenticated" }),
  { getState: () => ({ user: { id: auth.userId }, status: "authenticated" }) }) }));
vi.mock("../rich-content", () => ({ RichContent: ({ content }: { content: string }) => <p>{content}</p> }));
vi.mock("../navigation", () => ({ AppLink: (props: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a {...props} /> }));

const baseIssue = parseCollaborationIssue(collaborationIssueFixture)!;
const projectId = "00000000-0000-4000-8000-000000000099";
const issue = { ...baseIssue, source_structure: { issue_revision: "1", project: { id: projectId, name: "Lawful current project" }, parent_issue: null, iteration: null, stage: null } };
const commentId = "00000000-0000-4000-8000-000000000098";
const otherWorkspace: Workspace = { id: "00000000-0000-4000-8000-000000000090", name: "Visible destination workspace", slug: "knowledge-destination",
  description: null, context: null, settings: {}, repos: [], issue_prefix: "KD", avatar_url: null, created_at: "2026-11-01T00:00:00Z", updated_at: "2026-11-01T00:00:00Z" };
const otherProject: Project = { id: "00000000-0000-4000-8000-000000000091", workspace_id: otherWorkspace.id, title: "Visible destination project",
  description: null, icon: null, status: "in_progress", priority: "none", lead_type: null, lead_id: null, start_date: null, due_date: null,
  created_at: "2026-11-01T00:00:00Z", updated_at: "2026-11-01T00:00:00Z", issue_count: 0, done_count: 0, resource_count: 0 };
let api: ApiClient, client: QueryClient, scope: CollaborationScope;
let candidate: CollaborationKnowledgeCandidate;
let historyTruncated = false;
const lease = () => { const start = Date.now(); return { protocol_version: 1, projection_checked_at: new Date(start).toISOString(), valid_until: new Date(start + 30_000).toISOString(), next_expiry_at: null }; };
const summary = () => candidate.availability === "available" ? Object.fromEntries(Object.entries(candidate).filter(([key]) => key !== "content")) : candidate;
const sourcePage = () => parseCollaborationKnowledgeCandidates({ ...lease(), issue_id: issue.id, items: [summary()], next_cursor: null, capabilities: { can_propose: true } });
const detail = () => parseCollaborationKnowledgeDetail({ ...lease(), candidate, history: [], history_truncated: historyTruncated });

beforeEach(() => {
  localStorage.clear(); auth.userId = issue.handler.user_id; historyTruncated = false;
  Object.defineProperty(globalThis.crypto, "subtle", { configurable: true, value: webcrypto.subtle });
  api = new ApiClient("https://knowledge-ui.example"); setApiInstance(api); scope = collaborationScope(auth.userId);
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  candidate = { ...knowledgeCandidateFixture, source_issue_id: issue.id, source_workspace_id: issue.source_workspace.id,
    target: { kind: "workspace", workspace_id: issue.source_workspace.id }, sources: [{ kind: "memory", issue_id: issue.id, version: "7", sha256: "a".repeat(64) }] };
  vi.spyOn(api, "getCollaborationKnowledgeCandidates").mockImplementation(async () => sourcePage());
  vi.spyOn(api, "getCollaborationKnowledgeCandidate").mockImplementation(async () => detail());
  vi.spyOn(api, "getCollaborationKnowledgeTarget").mockImplementation(async (target) => parseCollaborationKnowledgeTargetList({ ...lease(), target, knowledge_revision: "1", items: [summary()], next_cursor: null }));
  vi.spyOn(api, "getCollaborationKnowledgeOperation").mockResolvedValue(null);
  vi.spyOn(api, "getCollaborationMemory").mockResolvedValue(null);
  vi.spyOn(api, "getCollaborationMemoryVersions").mockResolvedValue(null);
  const comment = parseCollaborationComment({ id: commentId, issue_id: issue.id, author_type: "member", author: issue.handler, content: "A readable comment source", type: "comment",
    parent_id: null, comment_revision: "3", attachment_ids: [], created_at: "2026-11-01T00:00:00Z", updated_at: "2026-11-01T00:00:00Z", deleted_at: null,
    resolved_at: null, resolved_by: null, can_edit: false, can_resolve: false, reactions: [] })!;
  vi.spyOn(api, "getCollaborationComments").mockResolvedValue(parseCollaborationComments({ issue_id: issue.id, items: [comment], next_cursor: null }));
  vi.spyOn(api, "getCollaborationComment").mockResolvedValue(comment);
});
afterEach(() => { cleanup(); client.clear(); localStorage.clear(); vi.useRealTimers(); vi.restoreAllMocks(); });
function mount(target = false, currentIssue = issue, collapsed = false) {
  const content = target
    ? <ManagedKnowledgePanel workspaceId={issue.source_workspace.id} />
    : <CollaborationKnowledgePanel scope={scope} issue={{ ...currentIssue, handler: { ...currentIssue.handler, external: true } }} />;
  return renderWithI18n(<QueryClientProvider client={client}>{collapsed ? <details><summary>Knowledge</summary>{content}</details> : content}</QueryClientProvider>);
}
async function openReview() {
  await userEvent.click(await screen.findByRole("button", { name: /PRIVATE SUMMARY/ }));
  await screen.findByText("PRIVATE BODY");
}
function receipt(requestId: string, action: CollaborationKnowledgeReceipt["action"]): CollaborationKnowledgeReceipt {
  return { protocol_version: 1, request_id: requestId, candidate_id: candidate.candidate_id, version: candidate.availability === "restricted" ? "1" : candidate.version,
    state_revision: "2", action, code: `knowledge_${action === "approve" ? "approved" : action === "reject" ? "rejected" : action === "withdraw" ? "withdrawn" : "proposed"}`,
    state: action === "approve" ? "approved" : action === "reject" ? "rejected" : action === "withdraw" ? "withdrawn" : "pending", target_knowledge_revision: "1" };
}

it("recovers after rapid candidate switches overlap authorization refreshes", async () => {
  const first = candidate;
  const second = { ...first, candidate_id: "00000000-0000-4000-8000-000000000092",
    lineage_id: "00000000-0000-4000-8000-000000000092", summary: "Second candidate", content: "Second conclusion" };
  const row = (value: CollaborationKnowledgeCandidate) => Object.fromEntries(Object.entries(value).filter(([key]) => key !== "content"));
  vi.mocked(api.getCollaborationKnowledgeCandidates).mockImplementation(async () => parseCollaborationKnowledgeCandidates({
    ...lease(), issue_id: issue.id, items: [row(first), row(second)], next_cursor: null, capabilities: { can_propose: true },
  }));
  const requests: { id: string; signal?: AbortSignal; resolve: () => void }[] = [];
  vi.mocked(api.getCollaborationKnowledgeCandidate).mockImplementation((id, options) => new Promise((resolve, reject) => {
    const signal = options?.signal;
    signal?.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")), { once: true });
    requests.push({ id, signal, resolve: () => resolve(parseCollaborationKnowledgeDetail({
      ...lease(), candidate: id === first.candidate_id ? first : second, history: [], history_truncated: false,
    })) });
  }));
  mount(); await userEvent.click(await screen.findByRole("button", { name: /PRIVATE SUMMARY/ }));
  await waitFor(() => expect(requests).toHaveLength(1));
  await userEvent.click(screen.getByRole("button", { name: /Second candidate/ }));
  await waitFor(() => expect(requests).toHaveLength(2));
  expect(requests[0]!.signal?.aborted).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: /PRIVATE SUMMARY/ }));
  await waitFor(() => expect(requests).toHaveLength(3));
  expect(requests[1]!.signal?.aborted).toBe(true);
  act(() => { expireKnowledgeProjections(client, scope); expireKnowledgeProjections(client, scope); });
  await waitFor(() => expect(requests).toHaveLength(4));
  await act(async () => { for (const request of requests) request.resolve(); });
  await screen.findByText("PRIVATE BODY");
  expect(screen.getByRole("button", { name: /Second candidate/ })).toBeInTheDocument();
  expect(screen.queryAllByText("Loading knowledge…")).toHaveLength(0);
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  expect(screen.getByRole("button", { name: "Approve exact version" })).toBeEnabled();
});

it("reloads the list and opens a newly submitted candidate for confirmation", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockImplementation(async (_issue, input) => {
    candidate = { ...knowledgeCandidateFixture, source_issue_id: issue.id, source_workspace_id: issue.source_workspace.id,
      candidate_id: input.candidate_id, lineage_id: input.candidate_id, version: "1", target: input.target,
      summary: input.summary, content: input.content, sources: input.sources, applicability: input.applicability,
      evidence: input.evidence, expires_at: input.expires_at };
    return { ...receipt(input.request_id, "propose"), candidate_id: input.candidate_id };
  });
  mount(); await openReview();
  await userEvent.click(screen.getByRole("button", { name: "Propose knowledge" })); await fillProposal();
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await screen.findByText("Candidate proposed. It is not active until approved.");
  await screen.findByRole("button", { name: /Human conclusion/ });
  await screen.findByRole("checkbox", { name: /I reviewed/ });
  expect(send).toHaveBeenCalledOnce();
  expect(screen.queryByText("Loading knowledge…")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  expect(screen.getByRole("button", { name: "Approve exact version" })).toBeEnabled();
});

it("offers recovery for erased idle queries and restores the list and exact review", async () => {
  mount(); await openReview();
  await act(async () => { expireKnowledgeProjections(client, scope, false); });
  await waitFor(() => expect(screen.queryByText("PRIVATE BODY")).not.toBeInTheDocument());
  expect(screen.queryByRole("checkbox", { name: /I reviewed/ })).not.toBeInTheDocument();
  expect(screen.queryAllByText("Loading knowledge…")).toHaveLength(0);
  const review = screen.getByRole("region", { name: "Review candidate" });
  const refresh = screen.getAllByRole("button", { name: "Refresh" });
  expect(refresh).toHaveLength(2);
  await userEvent.click(refresh.find((button) => !review.contains(button))!);
  await screen.findByRole("button", { name: /PRIVATE SUMMARY/ });
  let finish!: () => void;
  vi.mocked(api.getCollaborationKnowledgeCandidate).mockImplementationOnce(() => new Promise((resolve) => { finish = () => resolve(detail()); }));
  await userEvent.click(within(review).getByRole("button", { name: "Refresh" }));
  await within(review).findByText("Loading knowledge…");
  expect(within(review).queryByRole("button", { name: "Refresh" })).not.toBeInTheDocument();
  await act(async () => finish());
  await screen.findByText("PRIVATE BODY");
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  expect(screen.getByRole("button", { name: "Approve exact version" })).toBeEnabled();
});

it("requires keyboard confirmation of the displayed version and awaits the exact approval receipt", async () => {
  let finish!: (value: CollaborationKnowledgeReceipt) => void;
  const send = vi.spyOn(api, "approveCollaborationKnowledge").mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
  mount(); await openReview();
  const button = screen.getByRole("button", { name: "Approve exact version" }); expect(button).toBeDisabled();
  expect(screen.getByRole("link", { name: /Shared context.*Version 7/ })).toHaveAttribute("href", expect.stringContaining("#memory-v7"));
  screen.getByRole("checkbox", { name: /I reviewed this exact version/ }).focus(); await userEvent.keyboard(" ");
  expect(button).toBeEnabled(); await userEvent.click(button);
  await waitFor(() => expect(send).toHaveBeenCalledOnce()); expect(screen.queryByText("Decision recorded.")).not.toBeInTheDocument();
  const input = send.mock.calls[0]![1];
  expect(input).toMatchObject({ expected_state_revision: "1", expected_version: "9007199254740993", expected_content_sha256: "b".repeat(64) });
  await act(async () => { finish(receipt(input.request_id, "approve")); });
  await screen.findByText("Decision recorded.");
});

it("resets review confirmation for a changed identity and removes body on source unavailability", async () => {
  mount(); await openReview(); await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  expect(screen.getByRole("button", { name: "Approve exact version" })).toBeEnabled();
  candidate = { ...candidate, state_revision: "2" };
  await act(async () => { await client.invalidateQueries({ queryKey: collaborationKnowledgeKeys.candidate(scope, candidate.candidate_id) }); });
  await waitFor(() => expect(screen.getByRole("button", { name: "Approve exact version" })).toBeDisabled());
  candidate = { ...knowledgeUnavailableCandidateFixture, source_issue_id: issue.id, source_workspace_id: issue.source_workspace.id, target: { kind: "workspace", workspace_id: issue.source_workspace.id } };
  await act(async () => { await client.invalidateQueries({ queryKey: collaborationKnowledgeKeys.candidate(scope, candidate.candidate_id) }); });
  await screen.findByText("Source material is unavailable. Its text and references are hidden.");
  expect(screen.queryByText("PRIVATE BODY")).not.toBeInTheDocument(); expect(screen.queryByRole("button", { name: "Approve exact version" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Reject candidate" })).toBeDisabled();
  await userEvent.type(screen.getByRole("textbox", { name: "Decision reason" }), "The required source was removed");
  expect(screen.getByRole("button", { name: "Reject candidate" })).toBeEnabled();
});

// M4: Core erases the detail on every refresh (knowledge-cache.test.ts owns that); typed input must outlive the erasure.
async function refreshDetailHeld() {
  let release: (() => void) | undefined;
  vi.mocked(api.getCollaborationKnowledgeCandidate).mockImplementationOnce(() => new Promise((resolve) => { release = () => resolve(detail()); }));
  act(() => { void client.invalidateQueries({ queryKey: collaborationKnowledgeKeys.candidate(scope, candidate.candidate_id) }); });
  await waitFor(() => expect(screen.queryByText("PRIVATE BODY")).not.toBeInTheDocument());
  expect(screen.queryByRole("textbox", { name: "Decision reason" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Approve exact version" })).not.toBeInTheDocument();
  await waitFor(() => expect(release).toBeDefined());
  await act(async () => { release!(); });
  await screen.findByText("PRIVATE BODY");
}

it("keeps typed decision input and review confirmation across an unchanged refresh but resets them for a new revision", async () => {
  mount(); await openReview();
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  await userEvent.type(screen.getByRole("textbox", { name: "Decision reason" }), "Needs a source check");
  await refreshDetailHeld();
  expect(screen.getByRole("checkbox", { name: /I reviewed/ })).toBeChecked();
  expect(screen.getByRole("textbox", { name: "Decision reason" })).toHaveValue("Needs a source check");
  expect(screen.getByRole("button", { name: "Approve exact version" })).toBeEnabled();
  candidate = { ...candidate, state_revision: "2" };
  act(() => { void client.invalidateQueries({ queryKey: collaborationKnowledgeKeys.candidate(scope, candidate.candidate_id) }); });
  await waitFor(() => expect(screen.getByRole("checkbox", { name: /I reviewed/ })).not.toBeChecked());
  expect(screen.getByRole("textbox", { name: "Decision reason" })).toHaveValue("");
  expect(screen.getByRole("button", { name: "Approve exact version" })).toBeDisabled();
});

it("resets typed decision input when a held refresh returns a different version", async () => {
  mount(); await openReview();
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  await userEvent.type(screen.getByRole("textbox", { name: "Decision reason" }), "Typed for the old version");
  if (candidate.availability !== "available") throw new Error("Expected available fixture");
  candidate = { ...candidate, version: "9007199254740994", content_sha256: "c".repeat(64) };
  await refreshDetailHeld();
  expect(screen.getByRole("checkbox", { name: /I reviewed/ })).not.toBeChecked();
  expect(screen.getByRole("textbox", { name: "Decision reason" })).toHaveValue("");
  expect(screen.getByRole("button", { name: "Approve exact version" })).toBeDisabled();
});

it("recovers a lost result through the enclosing auxiliary disclosure after remount with the original key and no second POST", async () => {
  const send = vi.spyOn(api, "approveCollaborationKnowledge").mockRejectedValue(new TypeError("Lost response"));
  const mounted = mount(); await openReview(); await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  await userEvent.click(screen.getByRole("button", { name: "Approve exact version" }));
  await screen.findByRole("button", { name: "Retry same request" }); const key = send.mock.calls[0]![1].request_id;
  mounted.unmount(); mount(false, issue, true);
  await waitFor(() => expect(screen.getByText("Knowledge", { selector: "summary" }).closest("details")).toHaveAttribute("open"));
  expect(screen.queryByRole("button", { name: "Retry same request" })).not.toBeInTheDocument();
  vi.mocked(api.getCollaborationKnowledgeOperation).mockResolvedValue(receipt(key, "approve"));
  await userEvent.click(await screen.findByRole("button", { name: "Check original result" }));
  await screen.findByText("Decision recorded.");
  expect(api.getCollaborationKnowledgeOperation).toHaveBeenCalledWith(candidate.candidate_id, key); expect(send).toHaveBeenCalledOnce();
  expect(readPendingKnowledgeOperations(scope)).toEqual([]);
});

async function fillProposal() {
  fireEvent.change(screen.getByRole("textbox", { name: "Summary" }), { target: { value: "Human conclusion" } });
  fireEvent.change(screen.getByRole("textbox", { name: "Conclusion (Markdown)" }), { target: { value: "A new explicitly written conclusion" } });
  await userEvent.click(screen.getByRole("combobox", { name: "Sources" }));
  await userEvent.click(await screen.findByRole("option", { name: "Comment" }));
  await userEvent.click(await screen.findByRole("checkbox", { name: /Comment.*Version 3/ }));
}

async function openOtherDestination() {
  mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  await userEvent.click(screen.getByRole("combobox", { name: "Destination" }));
  await userEvent.click(await screen.findByRole("option", { name: "Another visible destination" }));
  return screen.getByRole("form", { name: "Propose knowledge" });
}

it("announces workspace loading and permits local retry to an empty catalog while preserving the proposal", async () => {
  let rejectRead!: (error: Error) => void;
  const workspaces = vi.spyOn(api, "listWorkspaces").mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectRead = reject; })).mockResolvedValue([]);
  const projects = vi.spyOn(api, "listProjects");
  const form = await openOtherDestination();
  expect(within(form).getByText("Loading workspaces...")).toHaveAttribute("role", "status");
  await act(async () => rejectRead(new TypeError("Offline")));
  expect(await within(form).findByText("Could not load workspaces. Refresh to try again.")).toBeVisible();
  await userEvent.click(within(form).getByRole("button", { name: "Refresh" }));
  expect(await within(form).findByText("No visible workspaces are available.")).toBeVisible();
  expect(within(form).getByRole("textbox", { name: "Summary" })).toHaveValue("Human conclusion");
  expect(within(form).getByRole("textbox", { name: "Conclusion (Markdown)" })).toHaveValue("A new explicitly written conclusion");
  expect(within(form).getByRole("checkbox", { name: /Comment.*Version 3/ })).toBeChecked();
  expect(workspaces).toHaveBeenCalledTimes(2); expect(projects).not.toHaveBeenCalled();
});

it("locally retries a failed selected project, retaining proposal fields and requiring an available target before submit", async () => {
  vi.spyOn(api, "listWorkspaces").mockResolvedValue([otherWorkspace]);
  let finishRead!: () => void;
  const projects = vi.spyOn(api, "listProjects").mockImplementationOnce(() => new Promise((resolve) => {
    finishRead = () => resolve({ projects: [otherProject], total: 1 });
  })).mockResolvedValue({ projects: [otherProject], total: 1 });
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockImplementation(async (_issue, input) => ({ ...receipt(input.request_id, "propose"), candidate_id: input.candidate_id }));
  const form = await openOtherDestination();
  await userEvent.click(within(form).getByRole("combobox", { name: "Workspace" }));
  await userEvent.click(await screen.findByRole("option", { name: otherWorkspace.name }));
  expect(within(form).getByText("Loading projects...")).toHaveAttribute("role", "status");
  await act(async () => finishRead());
  await userEvent.click(within(form).getByRole("combobox", { name: "Project" }));
  await userEvent.click(await screen.findByRole("option", { name: otherProject.title }));
  projects.mockRejectedValueOnce(new TypeError("Offline"));
  await act(async () => { await client.invalidateQueries({ queryKey: projectKeys.list(otherWorkspace.id) }); });
  expect(await within(form).findByText("Could not load projects. Refresh to try again.")).toBeVisible();
  expect(within(form).getByRole("combobox", { name: "Project" })).not.toHaveTextContent(otherProject.title);
  await userEvent.click(within(form).getByRole("button", { name: "Submit candidate" }));
  expect(send).not.toHaveBeenCalled();
  await userEvent.click(within(form).getByRole("button", { name: "Refresh" }));
  await waitFor(() => expect(within(form).getByRole("combobox", { name: "Project" })).toHaveTextContent(otherProject.title));
  expect(within(form).getByRole("textbox", { name: "Summary" })).toHaveValue("Human conclusion");
  expect(within(form).getByRole("textbox", { name: "Conclusion (Markdown)" })).toHaveValue("A new explicitly written conclusion");
  expect(within(form).getByRole("checkbox", { name: /Comment.*Version 3/ })).toBeChecked();
  expect(send).not.toHaveBeenCalled();
  await userEvent.click(within(form).getByRole("button", { name: "Submit candidate" }));
  await waitFor(() => expect(send).toHaveBeenCalledOnce());
  expect(send.mock.calls[0]![1].target).toEqual({ kind: "project", workspace_id: otherWorkspace.id, project_id: otherProject.id });
  expect(projects).toHaveBeenCalledTimes(3);
});

it("does not submit a removed destination after resolving sources or silently retarget it to the workspace", async () => {
  vi.spyOn(api, "listWorkspaces").mockResolvedValue([otherWorkspace]);
  const projects = vi.spyOn(api, "listProjects").mockResolvedValue({ projects: [otherProject], total: 1 });
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockImplementation(async (_issue, input) => ({ ...receipt(input.request_id, "propose"), candidate_id: input.candidate_id }));
  const form = await openOtherDestination();
  await userEvent.click(within(form).getByRole("combobox", { name: "Workspace" }));
  await userEvent.click(await screen.findByRole("option", { name: otherWorkspace.name }));
  await userEvent.click(within(form).getByRole("combobox", { name: "Project" }));
  await userEvent.click(await screen.findByRole("option", { name: otherProject.title }));
  const comment = await api.getCollaborationComment(issue.id, commentId);
  let release!: () => void;
  vi.mocked(api.getCollaborationComment).mockImplementationOnce(() => new Promise((resolve) => { release = () => resolve(comment); }));
  await userEvent.click(within(form).getByRole("button", { name: "Submit candidate" }));
  await waitFor(() => expect(release).toBeDefined());
  projects.mockResolvedValue({ projects: [], total: 0 });
  await act(async () => { await client.invalidateQueries({ queryKey: projectKeys.list(otherWorkspace.id) }); });
  expect(await within(form).findByText("No visible projects in this workspace.")).toBeVisible();
  expect(within(form).getByRole("combobox", { name: "Project" })).toHaveTextContent("Choose a project");
  await act(async () => release());
  await within(form).findByRole("button", { name: "Submit candidate" });
  expect(send).not.toHaveBeenCalled();
  expect(within(form).getByRole("textbox", { name: "Summary" })).toHaveValue("Human conclusion");
  expect(within(form).getAllByText("This destination is unavailable. Choose an available workspace or project.").length).toBeGreaterThan(0);
});

it("lets an external handler propose from comments to the lawful current project without a private catalog", async () => {
  const workspaces = vi.spyOn(api, "listWorkspaces"); const projects = vi.spyOn(api, "listProjects");
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockImplementation(async (_issue, input) => ({ ...receipt(input.request_id, "propose"), candidate_id: input.candidate_id }));
  mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  await userEvent.click(screen.getByRole("combobox", { name: "Destination" }));
  await userEvent.click(await screen.findByRole("option", { name: "Source project: Lawful current project" }));
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await waitFor(() => expect(send).toHaveBeenCalledOnce());
  const input = send.mock.calls[0]![1];
  expect(input.target).toEqual({ kind: "project", workspace_id: issue.source_workspace.id, project_id: projectId });
  expect(input.sources).toEqual([expect.objectContaining({ kind: "comment", comment_id: commentId, version: "3", sha256: expect.stringMatching(/^[a-f0-9]{64}$/) })]);
  expect(input.evidence).toEqual([]); expect(workspaces).not.toHaveBeenCalled(); expect(projects).not.toHaveBeenCalled();
});

it.each(["Propose correction", "Propose workspace version"])("%s creates a new immutable proposal without copying old body into the form", async (action) => {
  candidate = { ...knowledgeCandidateFixture, state: "approved", source_issue_id: issue.id, source_workspace_id: issue.source_workspace.id,
    target: { kind: "project", workspace_id: issue.source_workspace.id, project_id: projectId }, sources: [{ kind: "memory", issue_id: issue.id, version: "7", sha256: "a".repeat(64) }] };
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockImplementation(async (_issue, input) => ({ ...receipt(input.request_id, "propose"), candidate_id: input.candidate_id }));
  mount(); await openReview(); await userEvent.click(screen.getByRole("button", { name: action }));
  expect(screen.getByRole("textbox", { name: "Conclusion (Markdown)" })).toHaveValue(""); await fillProposal();
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await waitFor(() => expect(send).toHaveBeenCalledOnce()); const input = send.mock.calls[0]![1];
  expect(input.candidate_id).not.toBe(candidate.candidate_id);
  expect(input.supersedes_candidate_id).toBe(action === "Propose correction" ? candidate.candidate_id : null);
  expect(input.derived_from_candidate_id).toBe(action === "Propose workspace version" ? candidate.candidate_id : null);
  expect(input.target.kind).toBe(action === "Propose workspace version" ? "workspace" : "project");
});

it("allows only content-free target withdrawal and shows a known refusal as a refusal", async () => {
  candidate = { candidate_id: knowledgeCandidateFixture.candidate_id, availability: "restricted", state: "approved", state_revision: "2", capabilities: { can_withdraw: true } };
  const send = vi.spyOn(api, "withdrawCollaborationKnowledge").mockImplementation(async (_id, input) => ({ ...receipt(input.request_id, "withdraw"), code: "knowledge_revision_conflict", state: "approved" }));
  mount(true); await userEvent.click(await screen.findByRole("button", { name: /Restricted contribution/ }));
  await screen.findByRole("textbox", { name: "Decision reason" });
  expect(screen.queryByRole("button", { name: "Reject candidate" })).not.toBeInTheDocument(); expect(screen.queryByText("PRIVATE BODY")).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "Open source collaboration" })).not.toBeInTheDocument();
  await userEvent.type(screen.getByRole("textbox", { name: "Decision reason" }), "Stop using this contribution");
  await userEvent.click(screen.getByRole("button", { name: "Withdraw contribution" }));
  await screen.findByText(/The request was refused/); expect(screen.queryByText("Decision recorded.")).not.toBeInTheDocument();
  expect(send.mock.calls[0]![1]).not.toHaveProperty("expected_content_sha256"); expect(api.getCollaborationKnowledgeCandidates).not.toHaveBeenCalled();
});

it("keeps unsupported knowledge separate from the surrounding collaboration and reports truncated history", async () => {
  vi.mocked(api.getCollaborationKnowledgeCandidates).mockResolvedValueOnce(null);
  mount(); await screen.findByText("Knowledge is unavailable on this server. Existing context is unchanged.");
  expect(screen.getByRole("heading", { name: "Knowledge candidates" })).toBeInTheDocument(); expect(screen.queryByText(/No candidates yet/)).not.toBeInTheDocument();
  historyTruncated = true; await userEvent.click(screen.getByRole("button", { name: "Refresh" })); await openReview();
  await userEvent.click(screen.getByText("Version history"));
  expect(screen.getByText(/Only 50 versions are shown/)).toBeInTheDocument();
});

it("stops interval polling an unavailable knowledge interface until an explicit refresh", async () => {
  vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
  vi.mocked(api.getCollaborationKnowledgeCandidates).mockResolvedValue(null);
  mount(); await screen.findByText("Knowledge is unavailable on this server. Existing context is unchanged.");
  await act(async () => { await vi.advanceTimersByTimeAsync(60_000); });
  expect(api.getCollaborationKnowledgeCandidates).toHaveBeenCalledOnce();
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await waitFor(() => expect(api.getCollaborationKnowledgeCandidates).toHaveBeenCalledTimes(2));
});

it("restarts at the first page when a later page loses its authorization boundary", async () => {
  const boundary = lease();
  vi.mocked(api.getCollaborationKnowledgeCandidates).mockImplementation(async (_issueId, params) =>
    parseCollaborationKnowledgeCandidates({ ...boundary, issue_id: issue.id, items: params?.cursor ? [] : [summary()],
      next_cursor: params?.cursor ? null : "page2", capabilities: { can_propose: true } }));
  mount();
  await userEvent.click(await screen.findByRole("button", { name: "Next page" }));
  await waitFor(() => expect(api.getCollaborationKnowledgeCandidates).toHaveBeenCalledWith(issue.id, { cursor: "page2", limit: undefined }, expect.anything()));
  await act(async () => { expireKnowledgeProjections(client, scope); });
  await waitFor(() => expect(vi.mocked(api.getCollaborationKnowledgeCandidates).mock.calls.filter(([, params]) => !params?.cursor)).toHaveLength(2));
  expect(await screen.findByRole("button", { name: "Next page" })).toBeInTheDocument();
});

it("picks an actual recorded run as a source without memory or a raw JSON form", async () => {
  const execution = CollaborationExecutionSchema.parse(collaborationExecutionFixture); const run = execution.runs[1]!;
  const recordId = `event:${commentId}:message-0`;
  const records = parseCollaborationRunRecords({ protocol_version: 1, issue_id: issue.id, run,
    resource: { agent_id: run.agent_id, runtime_id: projectId, resource_owner_user_id: auth.userId, execution_workspace_id: issue.source_workspace.id, grant_id: commentId,
      provider: "recorded-provider", model: "recorded-model", account_source: "runtime_configuration" },
    recording: { status: "complete", inputs_recorded: true }, usage: null,
    items: [{ id: recordId, kind: "message", role: "assistant", body: "Observed test output" }], next_cursor: null });
  vi.spyOn(api, "getCollaborationRunRecords").mockResolvedValue(records);
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockImplementation(async (_issue, input) => ({ ...receipt(input.request_id, "propose"), candidate_id: input.candidate_id }));
  mount(false, { ...issue, execution }); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Summary" }), { target: { value: "Run finding" } });
  fireEvent.change(screen.getByRole("textbox", { name: "Conclusion (Markdown)" }), { target: { value: "A conclusion based on this run" } });
  await userEvent.click(screen.getByRole("combobox", { name: "Sources" })); await userEvent.click(await screen.findByRole("option", { name: "Run record" }));
  await userEvent.click(screen.getByRole("combobox", { name: "Choose a recorded run" }));
  await userEvent.click(await screen.findByRole("option", { name: new RegExp(run.id.slice(-8)) }));
  await userEvent.click(await screen.findByRole("checkbox", { name: /Observed test output/ }));
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await waitFor(() => expect(send).toHaveBeenCalledOnce());
  expect(send.mock.calls[0]![1].sources).toEqual([expect.objectContaining({ kind: "run_record", run_id: run.id, record_id: recordId })]);
});

it("pages all candidate versions with the original page authorization boundary", async () => {
  const boundary = lease();
  vi.mocked(api.getCollaborationKnowledgeCandidates).mockImplementation(async (_issueId, params) => parseCollaborationKnowledgeCandidates({
    ...boundary, issue_id: issue.id, capabilities: { can_propose: true }, next_cursor: params?.cursor ? null : "older",
    items: [{ ...summary(), summary: params?.cursor ? "Older candidate version" : "Latest candidate version", version: params?.cursor ? "1" : "2" }],
  }));
  mount(); await screen.findByRole("button", { name: /Latest candidate version/ });
  await userEvent.click(screen.getByRole("button", { name: "Next page" }));
  await screen.findByRole("button", { name: /Older candidate version/ });
  expect(api.getCollaborationKnowledgeCandidates).toHaveBeenLastCalledWith(issue.id, expect.objectContaining({ cursor: "older" }), expect.anything());
  await userEvent.click(screen.getByRole("button", { name: "Back to latest" }));
  await screen.findByRole("button", { name: /Latest candidate version/ });
});

it("keeps a selected source project pinned and requires a new selection if the relationship changes", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockImplementation(async (_issue, input) => ({ ...receipt(input.request_id, "propose"), candidate_id: input.candidate_id }));
  const view = (current: typeof issue) => <QueryClientProvider client={client}><CollaborationKnowledgePanel scope={scope} issue={current} /></QueryClientProvider>;
  const mounted = renderWithI18n(view(issue));
  await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  await userEvent.click(screen.getByRole("combobox", { name: "Destination" }));
  await userEvent.click(await screen.findByRole("option", { name: "Source project: Lawful current project" }));
  const nextProject = { id: "00000000-0000-4000-8000-000000000097", name: "Changed project" };
  mounted.rerender(view({ ...issue, source_structure: { ...issue.source_structure, project: nextProject } }));
  await screen.findByText("The selected source project changed. Choose the destination again.");
  expect(screen.getByRole("textbox", { name: "Summary" })).toHaveValue("Human conclusion");
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" })); expect(send).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("combobox", { name: "Destination" }));
  await userEvent.click(await screen.findByRole("option", { name: "Source project: Changed project" }));
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await waitFor(() => expect(send).toHaveBeenCalledOnce());
  expect(send.mock.calls[0]![1].target).toMatchObject({ kind: "project", project_id: nextProject.id });
});

it("unlocks a definitively rejected first proposal for editing and a new explicit request", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockRejectedValueOnce(new ApiError("invalid", 400, "Bad Request", { code: "invalid_knowledge_request", error: "invalid" }))
    .mockImplementation(async (_issue, input) => ({ ...receipt(input.request_id, "propose"), candidate_id: input.candidate_id }));
  mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await waitFor(() => expect(screen.getByRole("textbox", { name: "Summary" })).toBeEnabled());
  expect(screen.getByRole("button", { name: "Cancel" })).toBeEnabled();
  expect(readPendingKnowledgeOperations(scope)).toEqual([]);
  const first = send.mock.calls[0]![1];
  fireEvent.change(screen.getByRole("textbox", { name: "Summary" }), { target: { value: "Explicit correction" } });
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await waitFor(() => expect(send).toHaveBeenCalledTimes(2));
  expect(send.mock.calls[1]![1].request_id).not.toBe(first.request_id); expect(send.mock.calls[1]![1].candidate_id).not.toBe(first.candidate_id);
});

it("keeps a prior lost proposal locked to its original identity after a later rejection and not-found recovery", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockRejectedValueOnce(new TypeError("Lost response"))
    .mockRejectedValue(new ApiError("invalid", 400, "Bad Request", { code: "invalid_knowledge_request", error: "invalid" }));
  mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await userEvent.click(await screen.findByRole("button", { name: "Retry same request" }));
  await waitFor(() => expect(send).toHaveBeenCalledTimes(2));
  await userEvent.click(screen.getByRole("button", { name: "Check original result" }));
  await screen.findByText(/No confirmed result yet/);
  expect(screen.getByRole("textbox", { name: "Summary" })).toBeDisabled(); expect(send.mock.calls[0]).toEqual(send.mock.calls[1]);
  expect(readPendingKnowledgeOperations(scope)[0]?.requestId).toBe(send.mock.calls[0]![1].request_id);
});

it("shows known success when receipt cleanup fails instead of calling the write uncertain", async () => {
  const send = vi.spyOn(api, "approveCollaborationKnowledge").mockImplementation(async (_id, input) => receipt(input.request_id, "approve"));
  const remove = vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => { throw new Error("cleanup failed"); });
  try {
    mount(); await openReview(); await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
    await userEvent.click(screen.getByRole("button", { name: "Approve exact version" }));
    await screen.findByText("Decision recorded.");
    expect(screen.queryByRole("button", { name: "Retry same request" })).not.toBeInTheDocument();
    expect(screen.queryByText("Unresolved knowledge request")).not.toBeInTheDocument(); expect(send).toHaveBeenCalledOnce();
  } finally { remove.mockRestore(); }
});

it("unlocks the human draft after storage fails before POST", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge");
  mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  const save = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("storage full"); });
  try {
    await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
    await screen.findByText("The original request could not be saved. Check local storage and try again.");
    expect(send).not.toHaveBeenCalled(); expect(screen.getByRole("textbox", { name: "Summary" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeEnabled(); expect(screen.queryByText("Unresolved knowledge request")).not.toBeInTheDocument();
  } finally { save.mockRestore(); }
});

// M5: only this panel's own in-memory intent closes its proposal form; ownership never comes from a render-time closure.
it("checking the panel's own lost proposal closes its locked form without another POST", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockRejectedValue(new TypeError("Lost response"));
  mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await screen.findByRole("button", { name: "Retry same request" });
  const lost = send.mock.calls[0]![1];
  vi.mocked(api.getCollaborationKnowledgeOperation).mockResolvedValue({ ...receipt(lost.request_id, "propose"), candidate_id: lost.candidate_id });
  await userEvent.click(screen.getByRole("button", { name: "Check original result" }));
  await waitFor(() => expect(screen.queryByRole("form", { name: "Propose knowledge" })).not.toBeInTheDocument());
  expect(send).toHaveBeenCalledOnce();
});

it("a review decision leaves an open proposal draft intact and only the panel's own proposal closes it", async () => {
  vi.spyOn(api, "approveCollaborationKnowledge").mockImplementation(async (_id, input) => receipt(input.request_id, "approve"));
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockImplementation(async (_issue, input) => ({ ...receipt(input.request_id, "propose"), candidate_id: input.candidate_id }));
  mount(); await openReview();
  await userEvent.click(screen.getByRole("button", { name: "Propose knowledge" })); await fillProposal();
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  await userEvent.click(screen.getByRole("button", { name: "Approve exact version" }));
  await screen.findByText("Decision recorded.");
  expect(screen.getByRole("textbox", { name: "Summary" })).toHaveValue("Human conclusion");
  expect(screen.getByRole("checkbox", { name: /Comment.*Version 3/ })).toBeChecked();
  await screen.findByRole("button", { name: /PRIVATE SUMMARY/ });
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await waitFor(() => expect(send).toHaveBeenCalledOnce());
  await waitFor(() => expect(screen.queryByRole("form", { name: "Propose knowledge" })).not.toBeInTheDocument());
  await waitFor(() => expect(api.getCollaborationKnowledgeCandidate).toHaveBeenCalledWith(send.mock.calls[0]![1].candidate_id, expect.anything()));
});

it("recovering an older stored proposal keeps a new draft open while it opens the recovered candidate", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockRejectedValue(new TypeError("Lost response"));
  const mounted = mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await screen.findByRole("button", { name: "Retry same request" }); const lost = send.mock.calls[0]![1];
  mounted.unmount();
  // The lost proposal was applied, so this panel's list now holds it.
  vi.mocked(api.getCollaborationKnowledgeCandidates).mockImplementation(async () => parseCollaborationKnowledgeCandidates({ ...lease(), issue_id: issue.id,
    items: [summary(), { ...summary(), candidate_id: lost.candidate_id, summary: "Recovered proposal" }], next_cursor: null, capabilities: { can_propose: true } }));
  mount(); await screen.findByRole("button", { name: /Recovered proposal/ });
  await userEvent.click(screen.getByRole("button", { name: "Propose knowledge" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Summary" }), { target: { value: "Fresh draft" } });
  vi.mocked(api.getCollaborationKnowledgeOperation).mockResolvedValue({ ...receipt(lost.request_id, "propose"), candidate_id: lost.candidate_id });
  await userEvent.click(screen.getByRole("button", { name: "Check original result" }));
  await screen.findByText("Candidate proposed. It is not active until approved.");
  expect(screen.getByRole("textbox", { name: "Summary" })).toHaveValue("Fresh draft");
  await waitFor(() => expect(api.getCollaborationKnowledgeCandidate).toHaveBeenCalledWith(lost.candidate_id, expect.anything()));
});

it("recovering a request from the same source opens it even when it is outside the loaded list page", async () => {
  const send = vi.spyOn(api, "approveCollaborationKnowledge").mockRejectedValue(new TypeError("Lost response"));
  const mounted = mount(); await openReview(); await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  await userEvent.click(screen.getByRole("button", { name: "Approve exact version" }));
  await screen.findByRole("button", { name: "Retry same request" }); const key = send.mock.calls[0]![1].request_id;
  mounted.unmount();
  vi.mocked(api.getCollaborationKnowledgeCandidates).mockImplementation(async () => parseCollaborationKnowledgeCandidates({ ...lease(), issue_id: issue.id,
    items: [], next_cursor: null, capabilities: { can_propose: true } }));
  mount(); await screen.findByRole("button", { name: "Propose knowledge" });
  await userEvent.click(screen.getByRole("button", { name: "Propose knowledge" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Summary" }), { target: { value: "Fresh draft" } });
  vi.mocked(api.getCollaborationKnowledgeOperation).mockResolvedValue(receipt(key, "approve"));
  await userEvent.click(screen.getByRole("button", { name: "Check original result" }));
  expect(await screen.findByRole("region", { name: "Review candidate" })).toBeInTheDocument();
  expect(screen.getByRole("textbox", { name: "Summary" })).toHaveValue("Fresh draft");
  expect(send).toHaveBeenCalledOnce();
});

it("recovering a request whose candidate belongs to another source does not open it here", async () => {
  const send = vi.spyOn(api, "approveCollaborationKnowledge").mockRejectedValue(new TypeError("Lost response"));
  const mounted = mount(); await openReview(); await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  await userEvent.click(screen.getByRole("button", { name: "Approve exact version" }));
  await screen.findByRole("button", { name: "Retry same request" }); const key = send.mock.calls[0]![1].request_id;
  mounted.unmount();
  const recovered = receipt(key, "approve"), otherId = "00000000-0000-4000-8000-000000000096";
  vi.mocked(api.getCollaborationKnowledgeCandidates).mockImplementation(async () => parseCollaborationKnowledgeCandidates({ ...lease(), issue_id: issue.id,
    items: [{ ...summary(), candidate_id: otherId, lineage_id: otherId, summary: "Unrelated candidate" }], next_cursor: null, capabilities: { can_propose: true } }));
  mount(); await screen.findByRole("button", { name: /Unrelated candidate/ });
  const detailReads = vi.mocked(api.getCollaborationKnowledgeCandidate).mock.calls.length;
  vi.mocked(api.getCollaborationKnowledgeCandidate).mockImplementation(async () => parseCollaborationKnowledgeDetail({ ...lease(),
    candidate: { ...knowledgeCandidateFixture, source_issue_id: otherId, sources: knowledgeCandidateFixture.sources.map((source) => ({ ...source, issue_id: otherId })) },
    history: [], history_truncated: false }));
  vi.mocked(api.getCollaborationKnowledgeOperation).mockResolvedValue(recovered);
  await userEvent.click(screen.getByRole("button", { name: "Check original result" }));
  await screen.findByText("Decision recorded.");
  expect(screen.queryByRole("region", { name: "Review candidate" })).not.toBeInTheDocument();
  expect(screen.getByText("Open this candidate from the knowledge list it belongs to.")).toBeInTheDocument();
  expect(api.getCollaborationKnowledgeCandidate).toHaveBeenCalledTimes(detailReads + 1);
  expect(api.getCollaborationKnowledgeOperation).toHaveBeenCalledWith(recovered.candidate_id, key);
});

// L12: every panel lists every stored request of this viewer; knowledge-actions.test.ts owns dismissKnowledgeOperation's contract.
it("lets a human remove a stored identity only after a check finds no result, without any request", async () => {
  const send = vi.spyOn(api, "approveCollaborationKnowledge").mockRejectedValue(new TypeError("Lost response"));
  const first = mount(); await openReview(); await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  await userEvent.click(screen.getByRole("button", { name: "Approve exact version" }));
  await screen.findByRole("button", { name: "Retry same request" });
  await userEvent.click(screen.getByRole("button", { name: "Check original result" }));
  await screen.findByText(/No confirmed result yet/);
  // The panel's own in-memory intent is never dismissable.
  expect(screen.queryByRole("button", { name: "Remove local entry" })).not.toBeInTheDocument();
  first.unmount(); mount();
  await screen.findByText("Unresolved knowledge request");
  expect(screen.getByText("Requests started from other collaborations or destinations are listed here too.")).toBeInTheDocument();
  // No check has run in this mount yet.
  expect(screen.queryByRole("button", { name: "Remove local entry" })).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Check original result" }));
  await screen.findByText(/No confirmed result yet/);
  const dismiss = await screen.findByRole("button", { name: "Remove local entry" });
  expect(dismiss).toHaveAccessibleDescription(/only deletes the recovery entry on this device\. It does not cancel the request/);
  const remove = vi.spyOn(Storage.prototype, "removeItem").mockImplementationOnce(() => { throw new Error("storage locked"); });
  await userEvent.click(dismiss);
  await screen.findByText("The local recovery entry could not be removed.");
  expect(readPendingKnowledgeOperations(scope)).toHaveLength(1); expect(screen.getByText("Unresolved knowledge request")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Remove local entry" }));
  await waitFor(() => expect(screen.queryByText("Unresolved knowledge request")).not.toBeInTheDocument());
  expect(screen.queryByText("The local recovery entry could not be removed.")).not.toBeInTheDocument();
  expect(readPendingKnowledgeOperations(scope)).toEqual([]); expect(remove).toHaveBeenCalledTimes(2);
  expect(send).toHaveBeenCalledOnce(); expect(api.getCollaborationKnowledgeOperation).toHaveBeenCalledTimes(2);
});

// L8: submit joins a list refresh instead of blaming the source; each failure stage reports its own cause.
const sourceChangedText = "A selected source is unavailable. Review the selection and try again.";
it("reports a selected source that disappears as source-changed and preserves the draft", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge");
  mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  vi.mocked(api.getCollaborationComment).mockResolvedValueOnce(null);
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await screen.findByText(sourceChangedText);
  expect(screen.getByRole("textbox", { name: "Summary" })).toHaveValue("Human conclusion");
  expect(send).not.toHaveBeenCalled();
});

it("reports lost authority at intent preparation as proposal-unavailable and preserves the draft", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge");
  mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  vi.spyOn(collaboration, "prepareKnowledgeProposal").mockImplementationOnce(() => { throw new collaboration.CollaborationUnavailableError(); });
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await screen.findByText("Proposing knowledge is not available right now. Your draft is kept.");
  expect(screen.queryByText(sourceChangedText)).not.toBeInTheDocument();
  expect(screen.getByRole("textbox", { name: "Summary" })).toHaveValue("Human conclusion");
  expect(send).not.toHaveBeenCalled();
});

function holdSourceList(page: () => ReturnType<typeof sourcePage>) {
  let release: (() => void) | undefined;
  vi.mocked(api.getCollaborationKnowledgeCandidates).mockImplementationOnce(() => new Promise((resolve) => { release = () => resolve(page()); }));
  act(() => { void client.invalidateQueries({ queryKey: collaborationKnowledgeKeys.source(scope, issue.id) }); });
  return async () => { await waitFor(() => expect(release).toBeDefined()); await act(async () => { release!(); }); };
}

it("submit waits for a refreshing source list instead of reporting a changed source", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockImplementation(async (_issue, input) => ({ ...receipt(input.request_id, "propose"), candidate_id: input.candidate_id }));
  mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  const releaseList = holdSourceList(sourcePage);
  await waitFor(() => expect(screen.queryByRole("button", { name: /PRIVATE SUMMARY/ })).not.toBeInTheDocument());
  const digest = vi.spyOn(crypto.subtle, "digest");
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await waitFor(() => expect(digest).toHaveBeenCalled());
  await new Promise((resolve) => setTimeout(resolve, 50));
  expect(send).not.toHaveBeenCalled(); expect(screen.queryByText(sourceChangedText)).not.toBeInTheDocument();
  await releaseList();
  await waitFor(() => expect(send).toHaveBeenCalledOnce());
  expect(screen.queryByText(sourceChangedText)).not.toBeInTheDocument();
});

it("reports local validation failure as invalid input, not as an unavailable source", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge");
  mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  // The click-time precheck accepts NUL; only the schema inside prepareKnowledgeProposal rejects it.
  fireEvent.change(screen.getByRole("textbox", { name: "Summary" }), { target: { value: `Human${String.fromCharCode(0)}conclusion` } });
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await screen.findByText("Check the text, sources, destination and expiry before submitting.");
  expect(screen.queryByText(sourceChangedText)).not.toBeInTheDocument(); expect(send).not.toHaveBeenCalled();
});

it("fails closed with a proposal-unavailable notice when the refreshed list withdraws the grant", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge");
  mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  const releaseList = holdSourceList(() => parseCollaborationKnowledgeCandidates({ ...lease(), issue_id: issue.id, items: [summary()], next_cursor: null, capabilities: { can_propose: false } }));
  await waitFor(() => expect(screen.queryByRole("button", { name: /PRIVATE SUMMARY/ })).not.toBeInTheDocument());
  await userEvent.click(screen.getByRole("button", { name: "Submit candidate" }));
  await releaseList();
  await screen.findByText("Proposing knowledge is not available right now. Your draft is kept.");
  expect(send).not.toHaveBeenCalled(); expect(screen.getByRole("textbox", { name: "Summary" })).toHaveValue("Human conclusion");
  expect(screen.queryByText(sourceChangedText)).not.toBeInTheDocument();
});

// L9: limits count UTF-8 bytes (knowledge-schema.test.ts owns the boundary matrix); "知" is 3 bytes but 1 maxLength unit.
const overLimitText = (max: number, used: number) => `This is over the ${max}-byte limit (${used} bytes). Shorten it before submitting; your text is kept.`;
const utf8 = (count: number) => "知".repeat(Math.floor(count / 3)) + "a".repeat(count % 3);

it("attributes over-byte summary, conclusion and evidence to the field instead of a source failure", async () => {
  const send = vi.spyOn(api, "proposeCollaborationKnowledge").mockImplementation(async (_issue, input) => ({ ...receipt(input.request_id, "propose"), candidate_id: input.candidate_id }));
  mount(); await userEvent.click(await screen.findByRole("button", { name: "Propose knowledge" })); await fillProposal();
  const digest = vi.spyOn(crypto.subtle, "digest");
  const submit = screen.getByRole("button", { name: "Submit candidate" });
  for (const [name, max, used] of [["Summary", 2048, 2100], ["Conclusion (Markdown)", 65_536, 65_538], ["Evidence note (optional)", 2048, 2049]] as const) {
    const field = screen.getByRole("textbox", { name });
    fireEvent.change(field, { target: { value: utf8(used) } });
    expect(await screen.findByText(overLimitText(max, used))).toHaveAttribute("role", "alert");
    expect(field).toHaveAttribute("aria-invalid", "true"); expect(field).toHaveAccessibleDescription(new RegExp(`${max}-byte limit`));
    expect(field).toHaveValue(utf8(used)); expect(submit).toBeDisabled();
    // An exactly-full field is within the limit.
    fireEvent.change(field, { target: { value: utf8(max) } });
    await waitFor(() => expect(screen.queryByText(/-byte limit/)).not.toBeInTheDocument());
    expect(field).not.toHaveAttribute("aria-invalid", "true");
  }
  fireEvent.change(screen.getByRole("textbox", { name: "Summary" }), { target: { value: utf8(2049) } });
  fireEvent.submit(screen.getByRole("form", { name: "Propose knowledge" }));
  await new Promise((resolve) => setTimeout(resolve, 50));
  // Blocked before source resolution: no hash, no request, no source-unavailable blame.
  expect(digest).not.toHaveBeenCalled(); expect(send).not.toHaveBeenCalled(); expect(screen.queryByText(sourceChangedText)).not.toBeInTheDocument();
  fireEvent.change(screen.getByRole("textbox", { name: "Summary" }), { target: { value: utf8(2048) } });
  await userEvent.click(submit);
  await waitFor(() => expect(send).toHaveBeenCalledOnce());
  expect(send.mock.calls[0]![1]).toMatchObject({ summary: utf8(2048), content: utf8(65_536), evidence: [expect.objectContaining({ reference: utf8(2048) })] });
});

it("explains an over-byte decision reason instead of the generic invalid message", async () => {
  const reject = vi.spyOn(api, "rejectCollaborationKnowledge"), withdraw = vi.spyOn(api, "withdrawCollaborationKnowledge");
  mount(); await openReview();
  const reason = screen.getByRole("textbox", { name: "Decision reason" });
  fireEvent.change(reason, { target: { value: "知".repeat(400) } });
  expect(await screen.findByText(overLimitText(1000, 1200))).toHaveAttribute("role", "alert");
  expect(reason).toHaveAttribute("aria-invalid", "true"); expect(reason).toHaveAccessibleDescription(/1000-byte limit/);
  expect(screen.getByRole("button", { name: "Reject candidate" })).toBeDisabled(); expect(screen.getByRole("button", { name: "Withdraw contribution" })).toBeDisabled();
  // Approval sends no reason, so an over-limit reason does not block it.
  await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  expect(screen.getByRole("button", { name: "Approve exact version" })).toBeEnabled();
  expect(reject).not.toHaveBeenCalled(); expect(withdraw).not.toHaveBeenCalled();
  expect(screen.queryByText("Check the text, sources, destination and expiry before submitting.")).not.toBeInTheDocument();
  fireEvent.change(reason, { target: { value: utf8(1000) } });
  await waitFor(() => expect(screen.queryByText(/-byte limit/)).not.toBeInTheDocument());
  expect(screen.getByRole("button", { name: "Reject candidate" })).toBeEnabled();
});

it("does not repeat approval actor/time as an identical latest decision", async () => {
  if (candidate.availability !== "available") throw new Error("Expected available fixture");
  const approver = "00000000-0000-4000-8000-000000000072", at = "2026-11-01T01:00:00Z";
  candidate = { ...candidate, state: "approved", approved_by_user_id: approver, approved_at: at, decision: { actor_user_id: approver, at, reason: null } };
  mount(); await openReview(); const review = screen.getByRole("region", { name: "Review candidate" });
  expect(within(review).getAllByText(approver)).toHaveLength(1); expect(within(review).getByText("Approved by")).toBeInTheDocument();
  expect(within(review).queryByText("Latest decision by")).not.toBeInTheDocument(); expect(within(review).queryByText("Latest decision at")).not.toBeInTheDocument();
});

it("renders supplied proposer, approval and latest decision facts without a directory lookup", async () => {
  const proposer = "00000000-0000-4000-8000-000000000071", approver = "00000000-0000-4000-8000-000000000072", decider = "00000000-0000-4000-8000-000000000073";
  if (candidate.availability !== "available") throw new Error("Expected available fixture");
  candidate = { ...candidate, proposed_by_user_id: proposer, state: "withdrawn", approved_by_user_id: approver,
    approved_at: "2026-11-01T01:00:00Z", decision: { actor_user_id: decider, at: "2026-11-01T02:00:00Z", reason: "Withdrawn after the later review" } };
  const members = vi.spyOn(api, "listMembers");
  mount(); await openReview(); const review = screen.getByRole("region", { name: "Review candidate" });
  expect(within(review).getByText(proposer)).toBeInTheDocument(); expect(within(review).getByText(approver)).toBeInTheDocument(); expect(within(review).getByText(decider)).toBeInTheDocument();
  expect(within(review).getByText("Approved by")).toBeInTheDocument(); expect(within(review).getByText("Latest decision by")).toBeInTheDocument();
  expect(within(review).getByText("Withdrawn after the later review")).toBeInTheDocument(); expect(members).not.toHaveBeenCalled();
  candidate = { ...knowledgeUnavailableCandidateFixture, source_issue_id: issue.id, source_workspace_id: issue.source_workspace.id,
    target: { kind: "workspace", workspace_id: issue.source_workspace.id }, approved_by_user_id: approver, approved_at: "2026-11-01T01:00:00Z",
    state: "withdrawn", decision: { actor_user_id: decider, at: "2026-11-01T02:00:00Z" } };
  await act(async () => { await client.invalidateQueries({ queryKey: collaborationKnowledgeKeys.candidate(scope, candidate.candidate_id) }); });
  await screen.findByText("Source material is unavailable. Its text and references are hidden.");
  expect(within(review).queryByText("Withdrawn after the later review")).not.toBeInTheDocument(); expect(within(review).getByText(decider)).toBeInTheDocument();
});

it("reports list failures and unresolved writes without inventing candidate totals", async () => {
  const status = vi.fn();
  vi.mocked(api.getCollaborationKnowledgeCandidates).mockRejectedValueOnce(new Error("Offline"));
  renderWithI18n(<QueryClientProvider client={client}><CollaborationKnowledgePanel scope={scope} issue={issue} onStatusChange={status} /></QueryClientProvider>);
  await waitFor(() => expect(status).toHaveBeenLastCalledWith({ attention: true }));
  await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
  await screen.findByRole("button", { name: /PRIVATE SUMMARY/ });
  await waitFor(() => expect(status).toHaveBeenLastCalledWith({ attention: false }));
  const send = vi.spyOn(api, "approveCollaborationKnowledge").mockRejectedValue(new TypeError("Lost response"));
  await openReview(); await userEvent.click(screen.getByRole("checkbox", { name: /I reviewed/ }));
  await userEvent.click(screen.getByRole("button", { name: "Approve exact version" }));
  await screen.findByRole("button", { name: "Retry same request" });
  expect(send).toHaveBeenCalledOnce();
  await waitFor(() => expect(status).toHaveBeenLastCalledWith({ attention: true }));
  expect(status.mock.calls.every(([value]) => !("count" in value))).toBe(true);
});
