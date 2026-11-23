import { afterEach, expect, it, vi } from "vitest";
import { createHash } from "node:crypto";
import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue, parseCollaborationStageView, parseCollaborationStageCodeCandidate, collaborationStageKeys,
  collaborationMentionTargetsOptions, parseCollaborationTestReport,
  newCollaborationDeliveryForm, saveCollaborationDeliveryRecord, readCollaborationDeliveryRecords, type CollaborationTestReportRead, type CollaborationTestReport } from "@multica/core/collaboration";
import { collaborationIssueFixture, collaborationHostExecutionFixture, stageIds, stageIssueContextFixture, stageViewManagedFixture, stageCodeCandidateRefFixture,
  stageCodeViewFixture, stageCodeReviewDecisionFixture, stageReportDetailFixture, stageReportPageFixture, stageTestSubjectFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { StageCodePanel } from "./stage-code-panel";
import { CollaborationCommentComposer } from "./comment-composer";
import { StageTestReportHistory, StageTestReportReader } from "./stage-test-report";
import { StageCodeForms } from "./stage-code-forms";
import { Dialog, DialogContent } from "@multica/ui/components/ui/dialog";
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000203" } }) } }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); localStorage.clear(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });
const issue = parseCollaborationIssue({ ...collaborationIssueFixture, ...stageIssueContextFixture, execution: collaborationHostExecutionFixture, ai_policy: "assisted" })!;
const view = parseCollaborationStageView({ ...stageViewManagedFixture, stage: { current: "qa", stage_revision: "3" },
  roles: { ...stageViewManagedFixture.roles, qa_approver: stageViewManagedFixture.roles.product_approver },
  code_candidates: { items: [{ ...stageCodeCandidateRefFixture, state: "current" }], truncated: false } })!;
const record = parseCollaborationStageCodeCandidate({ ...stageCodeViewFixture, decisions: [stageCodeReviewDecisionFixture] })!;
function setup() {
  vi.spyOn(Date, "now").mockReturnValue(Date.parse("2026-11-14T03:05:00Z"));
  const api = new ApiClient("https://server.example"); setApiInstance(api);
  const scope = collaborationScope(stageIds.product);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); clients.push(client);
  vi.spyOn(api, "getCollaborationStage").mockResolvedValue({ state: "available", value: view });
  vi.spyOn(api, "getCollaborationStageCodeCandidate").mockResolvedValue({ state: "available", value: record });
  return { api, client, scope };
}
const target = { target_id: stageIds.agent, kind: "agent" as const, resource_id: stageIds.agent, target_epoch: "1", config_version: "1", hold_reason: null, hold_version: "1",
  state: "waiting" as const, code: "waiting", command_id: null, channel_id: null, batch_id: null, group_id: null };
it("prepares the existing comment, confirms a typed target, and sends the exact original test subject once", async () => {
  const { api, client, scope } = setup();
  vi.spyOn(api, "getCollaborationMentionTargets").mockResolvedValue({ protocol_version: 2, execution_view_version: 6, issue_id: issue.id, participants: [], can_submit_commands: true,
    targets: [{ target_id: stageIds.agent, kind: "agent", resource_id: stageIds.agent, name: "Test runner", target_epoch: "1", config_version: "1", queue_state: "active", hold_reason: null, hold_version: "1", waiting_reason: "", execution_available: false }] });
  const preview = vi.spyOn(api, "previewCollaborationComment").mockResolvedValue({ protocol_version: 2, execution_view_version: 6, issue_id: issue.id, lifecycle_epoch: issue.lifecycle_epoch,
    write_context: issue.write_context, recipients: [], targets: [target] });
  const send = vi.spyOn(api, "createCollaborationComment").mockResolvedValue({ protocol_version: 2, status: 201, code: "comment_created", issue_id: issue.id, comment_id: stageIds.comment,
    comment_revision: "1", event_id: stageIds.request, event_seq: "1", targets: [{ ...target, command_id: stageIds.request2, command_sequence: "1" }] });
  const form = newCollaborationDeliveryForm({ ...scope, context: { kind: "issue", issueId: issue.id, sourceWorkspaceId: issue.source_workspace.id } }, "comment");
  saveCollaborationDeliveryRecord({ ...form, content: "Exercise delayed callbacks" });
  renderWithI18n(<QueryClientProvider client={client}><StageCodePanel scope={scope} issue={issue} view={view} locked={false} selectedId={stageIds.candidate1} onSelect={vi.fn()} onOpen={vi.fn()} accountName={(id) => id} /><CollaborationCommentComposer scope={scope} issue={issue} /></QueryClientProvider>);
  await userEvent.click(await screen.findByRole("button", { name: "Prepare test input" }));
  expect(await screen.findByRole("region", { name: "Test input" })).toBeVisible();
  expect(screen.getByLabelText("Add comment")).toHaveValue("Exercise delayed callbacks");
  expect(send).not.toHaveBeenCalled();
  await userEvent.click(await screen.findByRole("button", { name: "Test runner" }));
  await userEvent.click(screen.getByRole("button", { name: "Send comment" }));
  await waitFor(() => expect(preview).toHaveBeenCalledOnce());
  expect(preview.mock.calls[0]?.[1]).not.toHaveProperty("test_subject");
  expect(send).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button", { name: "Send comment" }));
  await waitFor(() => expect(send).toHaveBeenCalledOnce());
  expect(send.mock.calls[0]?.[2]).toMatchObject({ test_subject: stageTestSubjectFixture, execution_view_version: 6,
    content: `Exercise delayed callbacks [@Test runner](mention://agent/${stageIds.agent})` });
  await waitFor(() => expect(screen.queryByRole("region", { name: "Test input" })).not.toBeInTheDocument());
});
it.each(["candidate", "mention authority"])("keeps the original prepared subject and input when %s changes", async (change) => {
  const { api, client, scope } = setup();
  vi.spyOn(api, "getCollaborationMentionTargets").mockResolvedValue({ protocol_version: 2, issue_id: issue.id, participants: [], can_submit_commands: true, targets: [] });
  const form = newCollaborationDeliveryForm({ ...scope, context: { kind: "issue", issueId: issue.id, sourceWorkspaceId: issue.source_workspace.id } }, "comment");
  saveCollaborationDeliveryRecord({ ...form, content: "Keep this input", testSubject: stageTestSubjectFixture });
  renderWithI18n(<QueryClientProvider client={client}><CollaborationCommentComposer scope={scope} issue={issue} /></QueryClientProvider>);
  await waitFor(() => expect(screen.getByRole("button", { name: "Send comment" })).toBeEnabled());
  act(() => {
    if (change === "candidate") client.setQueryData(collaborationStageKeys.view(scope, issue.id), { state: "available", value: { ...view, code_candidates: { items: [], truncated: false } } });
    else client.setQueryData(collaborationMentionTargetsOptions(client, scope, issue.id).queryKey,
      { protocol_version: 2, issue_id: issue.id, participants: [], can_submit_commands: false, targets: [] });
  });
  await waitFor(() => expect(screen.getByRole("button", { name: "Send comment" })).toBeDisabled());
  expect(screen.getByLabelText("Add comment")).toHaveValue("Keep this input");
  expect(readCollaborationDeliveryRecords(scope).find((entry) => entry.kind === "form")).toMatchObject({ testSubject: stageTestSubjectFixture });
});
it("B01 fresh R1 source denial hides cached private reports and retains the prepared comment without sending", async () => {
  const { api, client, scope } = setup();
  vi.spyOn(api, "getCollaborationMentionTargets").mockResolvedValue({ protocol_version: 2, issue_id: issue.id, participants: [], can_submit_commands: true, targets: [] });
  const readReport = vi.spyOn(api, "getCollaborationTestReport").mockResolvedValue({ state: "available", value: parseCollaborationTestReport(stageReportDetailFixture)! });
  const send = vi.spyOn(api, "createCollaborationComment");
  const form = newCollaborationDeliveryForm({ ...scope, context: { kind: "issue", issueId: issue.id, sourceWorkspaceId: issue.source_workspace.id } }, "comment");
  saveCollaborationDeliveryRecord({ ...form, content: "Keep original test input", testSubject: stageTestSubjectFixture });
  renderWithI18n(<QueryClientProvider client={client}>
    <StageCodePanel scope={scope} issue={issue} view={view} locked={false} selectedId={stageIds.candidate1} onSelect={vi.fn()} onOpen={vi.fn()} accountName={(id) => id} />
    <CollaborationCommentComposer scope={scope} issue={issue} />
    <StageTestReportReader scope={scope} issue={issue} report={{ run_id: stageIds.run, report_id: stageReportDetailFixture.report_id }} />
  </QueryClientProvider>);
  await screen.findByText("Expected paid, observed pending");
  await waitFor(() => expect(screen.getByRole("button", { name: "Send comment" })).toBeEnabled());
  const denied = parseCollaborationStageView({ ...view, children: { ...view.children,
    items: view.children.items.map((child) => ({ ...child, can_open: false, content_digest: null })) } })!;
  vi.mocked(api.getCollaborationStage).mockResolvedValue({ state: "available", value: denied });
  vi.mocked(api.getCollaborationStageCodeCandidate).mockImplementation(() => new Promise(() => undefined));
  readReport.mockImplementation(() => new Promise(() => undefined));
  await act(() => client.refetchQueries({ queryKey: collaborationStageKeys.view(scope, issue.id), exact: true }));
  await waitFor(() => expect(screen.queryByText("Expected paid, observed pending")).not.toBeInTheDocument());
  expect(screen.queryByText(stageCodeViewFixture.candidate.baseline.pull_request.head_sha)).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Send comment" })).toBeDisabled();
  expect(screen.getByLabelText("Add comment")).toHaveValue("Keep original test input");
  expect(readCollaborationDeliveryRecords(scope).find((entry) => entry.kind === "form")).toMatchObject({ testSubject: stageTestSubjectFixture });
  expect(send).not.toHaveBeenCalled();
});
it.each(["en", "zh-Hans"] as const)("opens a failed run report with plain-text retained logs in %s", async (locale) => {
  const { client, scope } = setup();
  const body = '<script>throw Error("untrusted")</script> https://external.invalid/log /private/path';
  const detailResponse = { ...stageReportDetailFixture, logs: [stageReportDetailFixture.logs[0], { kind: "inline", body, body_sha256: createHash("sha256").update(body).digest("hex") }] };
  const fetch = vi.fn(async (url: string) => new Response(JSON.stringify(url.includes(stageReportDetailFixture.report_id) ? detailResponse : stageReportPageFixture)));
  vi.stubGlobal("fetch", fetch);
  renderWithI18n(<QueryClientProvider client={client}><StageTestReportHistory scope={scope} issue={issue} runId={stageIds.run} /></QueryClientProvider>, { locale });
  await userEvent.click(await screen.findByRole("button", { name: locale === "en" ? "Open report" : "打开报告" }));
  const detail = await screen.findByRole("region", { name: locale === "en" ? "Test report detail" : "测试报告详情" });
  expect(within(detail).getByText("Expected paid, observed pending")).toBeVisible();
  expect(within(detail).getByText(/callback-delay/)).toBeVisible();
  expect(within(detail).queryAllByRole("link")).toHaveLength(0);
  expect(within(detail).getByText(body)).toBeVisible();
  expect(detail.querySelector("script")).toBeNull();
  expect(fetch.mock.calls.every(([url]) => url.startsWith(`https://server.example/api/collaboration/issues/${issue.id}/runs/`))).toBe(true);
});
it("requires current full report detail before enabling the explicit QA approval", async () => {
  const { api, client, scope } = setup();
  const report = parseCollaborationTestReport({ ...stageReportDetailFixture, reported_result: "passed", applicability: "usable", failed_case_count: 0,
    cases: stageReportDetailFixture.cases.map((testCase) => ({ ...testCase, outcome: "passed" })) })!;
  const code = { ...record, test_reports: [{ ...report, run_id: stageIds.run }], signable: record.signable.map((entry) => entry.step === "qa" ? { ...entry, allowed: true, max_expires_at: "2026-11-14T03:25:00Z" } : entry) };
  vi.mocked(api.getCollaborationStageCodeCandidate).mockResolvedValue({ state: "available", value: code });
  let resolve!: (value: CollaborationTestReportRead<CollaborationTestReport>) => void;
  vi.spyOn(api, "getCollaborationTestReport").mockImplementation(() => new Promise((done) => { resolve = done; }));
  const submit = vi.fn();
  renderWithI18n(<QueryClientProvider client={client}><Dialog open><DialogContent><StageCodeForms scope={scope} issue={issue} view={view} locked={false} feedback={null} submit={submit}
    intent={{ kind: "code", action: "decide", candidateId: stageIds.candidate1, step: "qa", decision: "approve" }} /></DialogContent></Dialog></QueryClientProvider>);
  await userEvent.selectOptions(await screen.findByRole("combobox", { name: "Test report" }), report.report_id);
  await userEvent.selectOptions(screen.getByRole("combobox", { name: "Preceding approval" }), stageIds.decision);
  await userEvent.click(screen.getByRole("checkbox", { name: "I reviewed this exact candidate and evidence" }));
  expect(screen.getByRole("button", { name: "Record approval" })).toBeDisabled();
  await act(async () => resolve({ state: "available", value: report }));
  await waitFor(() => expect(screen.getByRole("button", { name: "Record approval" })).toBeEnabled());
  await userEvent.click(screen.getByRole("button", { name: "Record approval" }));
  expect(submit).toHaveBeenCalledOnce();
  expect(submit.mock.calls[0]?.[0]).toMatchObject({ evidence: { test_report_id: report.report_id } });
});
