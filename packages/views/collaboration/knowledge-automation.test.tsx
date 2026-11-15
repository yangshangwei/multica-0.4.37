import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, expireKnowledgeProjections, parseCollaborationIssue, type CollaborationIssue, type KnowledgeAutomationJob, type ProjectKnowledgePolicy } from "@multica/core/collaboration";
import { collaborationIssueFixture, knowledgePolicyFixture, knowledgeJobFixture, knowledgeJobsFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { ProjectKnowledgePolicy as PolicyPanel } from "../projects/components/project-knowledge-policy";
import { KnowledgeAutomationStatus } from "./knowledge-automation-status";

// Core owns exhaustive schema, scope, lease and recovery matrices. These tests
// exercise the shared controls and actual Query/Mutation wiring.
const userId = "00000000-0000-4000-8000-000000000001";
vi.mock("@multica/core/auth", () => ({ useAuthStore: Object.assign((selector: (value: unknown) => unknown) => selector({ user: { id: userId } }),
  { getState: () => ({ user: { id: userId } }) }) }));
let api: ApiClient, client: QueryClient;
beforeEach(() => { localStorage.clear(); api = new ApiClient("https://knowledge-ui.example"); setApiInstance(api); client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); });
afterEach(() => { expireKnowledgeProjections(client, collaborationScope(userId), false); client.clear(); localStorage.clear(); vi.restoreAllMocks(); });
const lease = () => { const now = Date.now(); return { protocol_version: 2 as const, projection_checked_at: new Date(now).toISOString(), valid_until: new Date(now + 30000).toISOString(), next_expiry_at: null }; };
function mount(content: React.ReactNode, locale: "en" | "zh-Hans" = "en") { return renderWithI18n(<QueryClientProvider client={client}>{content}</QueryClientProvider>, { locale }); }
function acceptedIssue(): CollaborationIssue {
  const actor = collaborationIssueFixture.creator;
  const issue = parseCollaborationIssue({ ...collaborationIssueFixture, ai_policy: "assisted", status: "done", status_category: "done",
    source_structure: { issue_revision: "1", project: { id: knowledgePolicyFixture.project_id, name: "Project" }, parent_issue: null, iteration: null, stage: null },
    delivery: { current: null, has_history: true, latest: { id: knowledgeJobFixture.submission_id, issue_id: collaborationIssueFixture.id, event_seq: "10",
      lifecycle_epoch: "1", author_user_id: actor.user_id, author: actor, created_at: "2026-11-14T00:00:00Z", result: "Delivered result", basis: "Recorded basis",
      attachment_ids: [], source_comment_id: null, evidence_state: "available", decision: { kind: "accepted", submission_id: knowledgeJobFixture.submission_id,
        event_seq: "11", actor_user_id: actor.user_id, actor, created_at: "2026-11-14T00:00:01Z", reason: null, target_user_id: null } } } });
  if (!issue) throw new Error("Invalid accepted issue fixture");
  return issue;
}

it("shows the default policy and confirms a lost update through its original GET without another PUT", async () => {
  let current: ProjectKnowledgePolicy = { ...knowledgePolicyFixture, ...lease() };
  let requestId = "";
  vi.spyOn(api, "getProjectKnowledgePolicy").mockImplementation(async () => ({ ...current, ...lease() }));
  const write = vi.spyOn(api, "updateProjectKnowledgePolicy").mockImplementation(async (_workspace, _project, input) => {
    requestId = input.request_id;
    current = { ...current, revision: "1", is_enabled: input.is_enabled, max_candidates: input.max_candidates, origin: "human", changed_by_user_id: userId, updated_at: new Date().toISOString() };
    throw new TypeError("Lost response");
  });
  const recover = vi.spyOn(api, "getProjectKnowledgePolicyOperation").mockImplementation(async () => ({ protocol_version: 2, request_id: requestId,
    project_id: current.project_id, code: "knowledge_policy_updated", revision: "1" }));
  mount(<PolicyPanel workspaceId={current.workspace_id} projectId={current.project_id} />);
  expect(await screen.findByRole("switch", { name: "Organize knowledge after acceptance" })).toBeChecked();
  expect(screen.getByText("Using the default policy")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("switch", { name: "Organize knowledge after acceptance" }));
  await userEvent.click(screen.getByRole("button", { name: "Save policy" }));
  await screen.findByText("Unconfirmed request");
  expect(write).toHaveBeenCalledTimes(1);
  await userEvent.click(screen.getByRole("button", { name: "Check original result" }));
  await screen.findByText("Policy saved.");
  await waitFor(() => expect(screen.getByRole("switch", { name: "Organize knowledge after acceptance" })).not.toBeChecked());
  expect(recover).toHaveBeenCalledExactlyOnceWith(current.project_id, requestId);
  expect(write).toHaveBeenCalledTimes(1);
});

it("shows paused automatic knowledge and actual zero usage without claiming verified evidence", async () => {
  const issue = acceptedIssue(), open = vi.fn();
  const job = { ...knowledgeJobFixture, issue_id: issue.id, source_current: false, counts: { ...knowledgeJobFixture.counts, saved: 2 },
    attempts: [{ stage: "generating" as const, state: "succeeded" as const, model: "actual-model", usage: { status: "known" as const, input_tokens: 0, output_tokens: 0, total_tokens: 0 } }] };
  vi.spyOn(api, "getCollaborationKnowledgeJobs").mockImplementation(async () => ({ ...knowledgeJobsFixture, ...lease(), issue_id: issue.id, items: [job] }));
  mount(<KnowledgeAutomationStatus scope={collaborationScope(userId)} issue={issue} onOpenKnowledge={open} />);
  expect(await screen.findByText("Knowledge from this acceptance is paused until it is reviewed again.")).toBeInTheDocument();
  expect(screen.getByText("2 saved")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Review knowledge" }));
  expect(open).toHaveBeenCalledOnce();
  await userEvent.click(screen.getByText("Processing details"));
  expect(screen.getByText(/actual-model/)).toBeInTheDocument();
  const usage = screen.getByText("Input tokens").parentElement!;
  expect(within(usage).getByText("0")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Retry organization" })).not.toBeInTheDocument();
});

it("does not request automatic status or show processing for no-project or manual tasks", () => {
  const read = vi.spyOn(api, "getCollaborationKnowledgeJobs");
  const issue = acceptedIssue();
  const result = mount(<KnowledgeAutomationStatus scope={collaborationScope(userId)} issue={{ ...issue, ai_policy: "manual" }} />);
  expect(screen.queryByRole("region", { name: "Project knowledge" })).not.toBeInTheDocument();
  result.rerender(<QueryClientProvider client={client}><KnowledgeAutomationStatus scope={collaborationScope(userId)} issue={{ ...issue,
    source_structure: { ...issue.source_structure!, project: null } }} /></QueryClientProvider>);
  expect(screen.queryByRole("region", { name: "Project knowledge" })).not.toBeInTheDocument();
  expect(read).not.toHaveBeenCalled();
});

it("sends an authorized retry with the displayed job revision and waits for its receipt", async () => {
  const issue = acceptedIssue();
  let job: KnowledgeAutomationJob = { ...knowledgeJobFixture, issue_id: issue.id, phase: "review_pending", can_retry: true };
  vi.spyOn(api, "getCollaborationKnowledgeJobs").mockImplementation(async () => ({ ...knowledgeJobsFixture, ...lease(), issue_id: issue.id, items: [job] }));
  const send = vi.spyOn(api, "retryCollaborationKnowledgeJob").mockImplementation(async (_issue, jobId, input) => {
    job = { ...job, phase: "queued", can_retry: false, revision: "2" };
    return { protocol_version: 2, request_id: input.request_id, issue_id: issue.id, job_id: jobId, revision: "2", code: "knowledge_job_retried" };
  });
  mount(<KnowledgeAutomationStatus scope={collaborationScope(userId)} issue={issue} />);
  await userEvent.click(await screen.findByRole("button", { name: "Retry organization" }));
  await screen.findByText("Organization retry requested.");
  expect(send).toHaveBeenCalledOnce();
  expect(send.mock.calls[0]?.[2]).toMatchObject({ expected_revision: "1" });
  expect(send.mock.calls[0]?.[2].request_id).toMatch(/^[0-9a-f-]{36}$/);
});
