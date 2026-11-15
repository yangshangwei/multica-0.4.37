import { afterEach, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { collaborationScope, parseCollaborationIssue } from "@multica/core/collaboration";
import { collaborationIssueFixture, stageCodeViewFixture, stageCodeCandidateRefFixture, stageIds, stageIssueContextFixture, stagePullRequestFixture,
  stageViewManagedFixture, stageConfirmations, stageCodeEvidenceFixture, stageCodeReviewDecisionFixture, stageTestReportSummaryFixture,
  stageExecutionViewFixture, stageExecutionTargetFixture, stageExecutionIds, stageExecutionBuildFixture, stageReportDetailFixture } from "@multica/core/collaboration/test-fixtures";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider } from "../navigation";
import { CollaborationStagePanel } from "./stage-panel";

// Core stage-code suites own the strict schema, authority, evidence and recovery matrices.
vi.mock("@multica/core/auth", () => ({ useAuthStore: { getState: () => ({ user: { id: "00000000-0000-4000-8000-000000000203" } }) } }));
const clients: QueryClient[] = [];
afterEach(() => { clients.splice(0).forEach((client) => client.clear()); localStorage.clear(); vi.unstubAllGlobals(); vi.restoreAllMocks(); });
const issue = parseCollaborationIssue({ ...collaborationIssueFixture, ...stageIssueContextFixture })!;
const view = { ...stageViewManagedFixture, stage: { current: "code_review", stage_revision: "2" },
  roles: { ...stageViewManagedFixture.roles, code_reviewer: stageViewManagedFixture.roles.product_approver },
  code_candidates: { items: [{ ...stageCodeCandidateRefFixture, state: "current" }], truncated: false } };
const issuePath = `/api/collaboration/issues/${stageIds.issue}`;
function mount(stage: unknown = view, code: unknown = stageCodeViewFixture, write?: (url: string, body: unknown) => unknown, reportResponse?: () => Response) {
  const writes: { url: string; body: unknown }[] = [];
  vi.spyOn(Date, "now").mockReturnValue(Date.parse("2026-11-14T03:05:00Z"));
  vi.stubGlobal("fetch", vi.fn(async (url: string, init: RequestInit = {}) => {
    const path = new URL(url).pathname;
    if (init.method === "POST") { const body: unknown = JSON.parse(String(init.body)); writes.push({ url: path, body }); return new Response(JSON.stringify(write?.(path, body) ?? { code: "unknown" }), { status: 200 }); }
    if (path === issuePath) return new Response(JSON.stringify({ ...collaborationIssueFixture, ...stageIssueContextFixture }));
    if (path === `${issuePath}/runs/${stageIds.run}/test-reports/${stageTestReportSummaryFixture.report_id}`) return reportResponse?.() ?? new Response(JSON.stringify({ ...stageReportDetailFixture,
      report_id: stageTestReportSummaryFixture.report_id, subject: { ...stageReportDetailFixture.subject, artifact_digest: stageExecutionBuildFixture.artifact_digest },
      reported_result: "passed", applicability: "usable", case_count: 3, failed_case_count: 0, cases: ["callback", "refund", "total"].map((id) => ({ id, outcome: "passed", log_index: 1 })) }));
    if (path === `${issuePath}/stage`) return new Response(JSON.stringify(stage));
    if (path === `${issuePath}/stage/proposals`) return new Response(JSON.stringify({ issue_id: stageIds.issue, items: [], next_cursor: null }));
    if (path.startsWith(`${issuePath}/stage/code-candidates/`)) { const value: unknown = typeof code === "function" ? await code(path) : code; return value === null ? new Response(null, { status: 404 }) : new Response(JSON.stringify(value)); }
    throw new Error(`Unexpected request ${path}`);
  }));
  setApiInstance(new ApiClient("https://server.example"));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); clients.push(client);
  const navigation = { pathname: "/account", searchParams: new URLSearchParams(), hash: "", push: vi.fn(), replace: vi.fn(), back: vi.fn(), getShareableUrl: (path: string) => path };
  renderWithI18n(<QueryClientProvider client={client}><NavigationProvider value={navigation}><CollaborationStagePanel scope={collaborationScope(stageIds.product)} issue={issue} /></NavigationProvider></QueryClientProvider>);
  return writes;
}
it("discovers code candidates after reload and shows exact CI plus review affordance", async () => {
  mount();
  const section = await screen.findByRole("region", { name: "Code and approvals" });
  expect(await within(section).findByText(stagePullRequestFixture.head_sha)).toBeVisible();
  expect(within(section).getByText("CI passed")).toBeVisible();
  expect(within(section).getByRole("button", { name: "Approve code review" })).toBeEnabled();
});
it("keeps an absent R4 unavailable and offers no approval", async () => {
  mount(view, null);
  const section = await screen.findByRole("region", { name: "Code and approvals" });
  expect(await within(section).findByText("Code evidence is unavailable on this server.")).toBeVisible();
  expect(within(section).queryByRole("button", { name: "Approve code review" })).not.toBeInTheDocument();
});
it("creates a code candidate from the observed child PR and opens its returned identity", async () => {
  const writes = mount({ ...view, code_candidates: { items: [], truncated: false } }, stageCodeViewFixture,
    () => ({ code: "stage_code_candidate_created", collaboration_revision: "9007199254740994", stage_confirmation: stageConfirmations.create_code_candidate }));
  await userEvent.click(await screen.findByRole("button", { name: "Create code candidate" }));
  const dialog = await screen.findByRole("dialog", { name: "Create code candidate" });
  await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Sub-issue" }), stageIds.child1);
  await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Pull request and head" }), JSON.stringify(stagePullRequestFixture));
  await userEvent.click(within(dialog).getByRole("button", { name: "Save code candidate" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(writes).toHaveLength(1);
  expect(writes[0]).toMatchObject({ url: `${issuePath}/stage/code-candidates`, body: { child_issue_id: stageIds.child1, supersedes: null, pull_request: stagePullRequestFixture, artifact: null } });
});
it("signs the exact code candidate with an explicit approval and bounded expiry", async () => {
  const writes = mount(view, stageCodeViewFixture, (_url, body) => ({ code: "stage_decision_recorded", collaboration_revision: "9007199254740994",
    stage_confirmation: { ...stageConfirmations.decide_stage, step: "code_review", expires_at: (body as { expires_at: string }).expires_at } }));
  await userEvent.click(await screen.findByRole("button", { name: "Approve code review" }));
  const dialog = await screen.findByRole("dialog", { name: "Approve code review" });
  await userEvent.click(within(dialog).getByRole("checkbox", { name: "I reviewed this exact candidate and evidence" }));
  await userEvent.click(within(dialog).getByRole("button", { name: "Record approval" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(writes[0]).toMatchObject({ url: `${issuePath}/stage/decisions`, body: { candidate: stageCodeCandidateRefFixture,
    decision: "approve", step: "code_review", evidence: stageCodeEvidenceFixture, expected_stage_revision: "2" } });
  expect(writes).toHaveLength(1);
});

it("shows only the anonymous source-denied state, with no private report or approval detail", async () => {
  const denied = { ...stageCodeViewFixture, candidate: { ...stageCodeViewFixture.candidate, baseline: null }, blocked_evidence: 1,
    observed: { pull_request: { state: "unavailable", head_sha: null }, ci: { state: "unavailable", ci_digest: null, head_sha: null, observed_at: null, check_count: 0 } },
    signable: stageCodeViewFixture.signable.map((entry) => ({ ...entry, allowed: false, max_expires_at: null, blockers: [{ code: "stage_evidence_unavailable", stage: "qa" }] })) };
  mount(view, denied);
  const section = await screen.findByRole("region", { name: "Code and approvals" });
  expect(await within(section).findByText("You cannot currently read the source evidence. No source details or approvals are shown.")).toBeVisible();
  expect(within(section).queryByText(stagePullRequestFixture.head_sha)).not.toBeInTheDocument();
  expect(within(section).queryByRole("region", { name: "Test reports" })).not.toBeInTheDocument();
  expect(within(section).queryByRole("button", { name: "Approve code review" })).not.toBeInTheDocument();
});
it("allows QA to return an execution-failed report without manufacturing an approval", async () => {
  const qaView = { ...view, stage: { current: "qa", stage_revision: "3" }, roles: { ...stageViewManagedFixture.roles, qa_approver: stageViewManagedFixture.roles.product_approver } };
  const failed = { ...stageCodeViewFixture, test_reports: [{ ...stageTestReportSummaryFixture, reported_result: "failed", failed_case_count: 1, applicability: "execution_failed" }],
    signable: stageCodeViewFixture.signable.map((entry) => ({ ...entry, allowed: false, max_expires_at: null })) };
  const writes = mount(qaView, failed, () => ({ code: "stage_decision_recorded", collaboration_revision: "9007199254740994",
    stage_confirmation: { ...stageConfirmations.decide_stage, step: "qa", decision: "returned", expires_at: null } }));
  expect(await screen.findByText(/^Reported failed/)).toBeVisible();
  expect(screen.queryByRole("button", { name: "Approve QA" })).not.toBeInTheDocument();
  await userEvent.click(await screen.findByRole("button", { name: "Return from QA" }));
  const dialog = await screen.findByRole("dialog", { name: "Return from QA" });
  await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Test report" }), stageTestReportSummaryFixture.report_id);
  await userEvent.type(within(dialog).getByRole("textbox", { name: "Reason" }), "Fix the failed callback case");
  await userEvent.click(within(dialog).getByRole("checkbox", { name: "I reviewed this exact candidate and evidence" }));
  await userEvent.click(within(dialog).getByRole("button", { name: "Record return" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(writes[0]).toMatchObject({ body: { step: "qa", decision: "return", expires_at: null, depends_on: [], evidence: { test_report_id: stageTestReportSummaryFixture.report_id } } });
  expect(writes).toHaveLength(1);
});
it.each(["revoke", "advance"] as const)("wires %s to the exact selected decision", async (action) => {
  const decision = { ...stageCodeReviewDecisionFixture, actor_user_id: stageIds.product };
  const writes = mount(view, { ...stageCodeViewFixture, decisions: [decision] }, () => ({
    code: action === "revoke" ? "stage_decision_revoked" : "stage_advanced", collaboration_revision: "9007199254740994",
    stage_confirmation: action === "revoke" ? stageConfirmations.revoke_decision : { ...stageConfirmations.advance_stage, stage_revision: "3" },
  }));
  const title = action === "revoke" ? "Revoke approval" : "Move to QA";
  await userEvent.click(await screen.findByRole("button", { name: title }));
  const dialog = await screen.findByRole("dialog", { name: title });
  if (action === "revoke") await userEvent.type(within(dialog).getByRole("textbox", { name: "Reason" }), "Approval needs correction");
  await userEvent.click(within(dialog).getByRole("button", { name: action === "revoke" ? "Confirm revocation" : "Confirm stage change" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(writes).toHaveLength(1);
  expect(writes[0]?.url).toBe(action === "revoke" ? `${issuePath}/stage/decisions/${stageIds.decision}/revocations` : `${issuePath}/stage/advance`);
  if (action === "advance") expect(writes[0]?.body).toMatchObject({ candidate: stageCodeCandidateRefFixture, decision_ids: [stageIds.decision], target_stage: "qa" });
});
it.each(["release", "rollback"] as const)("records %s approval against existing build/deployment evidence without executing it", async (step) => {
  const configured = { ...view, stage: { current: step === "release" ? "release" : "verification", stage_revision: "4" },
    roles: { ...stageViewManagedFixture.roles, release_approver: stageViewManagedFixture.roles.product_approver } };
  const record = { ...stageExecutionViewFixture, decisions: stageExecutionViewFixture.decisions.map((d) => d.step === "release" ? { ...d, actor_user_id: stageIds.product } : d),
    signable: stageExecutionViewFixture.signable.map((s) => ({ ...s, allowed: s.step === step, max_expires_at: s.step === step ? "2026-11-14T03:20:00Z" : null })) };
  const writes = mount(configured, record, (_url, body) => ({ code: "stage_decision_recorded", collaboration_revision: "9007199254740994",
    stage_confirmation: { ...stageConfirmations.decide_stage, step, expires_at: (body as { expires_at: string }).expires_at } }));
  const title = step === "release" ? "Approve release" : "Approve rollback";
  await userEvent.click(await screen.findByRole("button", { name: title }));
  const dialog = await screen.findByRole("dialog", { name: title });
  if (step === "release") await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Deployment target" }), stageExecutionTargetFixture.target_digest);
  else {
    await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Original deployment" }), stageExecutionIds.fact);
    await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Metric observation" }), stageExecutionIds.observation);
  }
  await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Preceding approval" }), step === "release" ? stageExecutionIds.qa : stageExecutionIds.release);
  await userEvent.click(within(dialog).getByRole("checkbox", { name: "I reviewed this exact candidate and evidence" }));
  await userEvent.click(within(dialog).getByRole("button", { name: "Record approval" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(writes).toHaveLength(1);
  expect(writes[0]).toMatchObject({ url: `${issuePath}/stage/decisions`, body: { step, decision: "approve", depends_on: [step === "release" ? stageExecutionIds.qa : stageExecutionIds.release],
    evidence: step === "release" ? { build: stageExecutionBuildFixture, deployment_id: null }
      : { build: null, deployment_id: stageExecutionIds.fact, rollback_target_digest: "b".repeat(64), rollback_observation: { observation_id: stageExecutionIds.observation, observation_digest: "f".repeat(64) } } } });
});

it("shows pending execution separately and never offers its approval for revocation", async () => {
  const configured = { ...view, stage: { current: "release", stage_revision: "4" }, roles: { ...stageViewManagedFixture.roles, release_approver: stageViewManagedFixture.roles.product_approver } };
  const record = { ...stageExecutionViewFixture, execution_facts: [], rollback_observations: [],
    execution_operations: [{ ...stageExecutionViewFixture.execution_operations[0], state: "pending", fact_id: null, external_operation_id: null }],
    decisions: [{ ...stageExecutionViewFixture.decisions[0], actor_user_id: stageIds.product }] };
  mount(configured, record);
  const operations = await screen.findByRole("region", { name: "Execution requests" });
  expect(within(operations).getByText("Pending confirmation")).toBeVisible();
  expect(screen.queryByRole("button", { name: "Revoke approval" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Move to Verification" })).not.toBeInTheDocument();
});

it("disables a selected rollback metric after ambiguity while retaining the unsent reason", async () => {
  const configured = { ...view, stage: { current: "verification", stage_revision: "4" }, roles: { ...stageViewManagedFixture.roles, release_approver: stageViewManagedFixture.roles.product_approver } };
  const record = structuredClone({ ...stageExecutionViewFixture, decisions: stageExecutionViewFixture.decisions.map((d) => d.step === "release" ? { ...d, actor_user_id: stageIds.product } : d) });
  const writes = mount(configured, record);
  await userEvent.click(await screen.findByRole("button", { name: "Approve rollback" }));
  const dialog = await screen.findByRole("dialog", { name: "Approve rollback" });
  await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Original deployment" }), stageExecutionIds.fact);
  await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Metric observation" }), stageExecutionIds.observation);
  await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Preceding approval" }), stageExecutionIds.release);
  await userEvent.type(within(dialog).getByRole("textbox", { name: "Reason" }), "Investigate the anomaly");
  await userEvent.click(within(dialog).getByRole("checkbox", { name: "I reviewed this exact candidate and evidence" }));
  expect(within(dialog).getByRole("button", { name: "Record approval" })).toBeEnabled();
  record.rollback_observations[0]!.state = "ambiguous";
  await clients.at(-1)!.refetchQueries();
  await waitFor(() => expect(within(dialog).getByRole("button", { name: "Record approval" })).toBeDisabled());
  expect(within(dialog).getByRole("textbox", { name: "Reason" })).toHaveValue("Investigate the anomaly");
  expect(writes).toEqual([]);
});

it("erases release report and target evidence after T4 artifact denial even while R1 children remain readable", async () => {
  const configured = { ...view, stage: { current: "release", stage_revision: "4" }, roles: { ...stageViewManagedFixture.roles, release_approver: stageViewManagedFixture.roles.product_approver } };
  let denied = false;
  const report = { ...stageReportDetailFixture, report_id: stageTestReportSummaryFixture.report_id,
    subject: { ...stageReportDetailFixture.subject, artifact_digest: stageExecutionBuildFixture.artifact_digest }, reported_result: "passed", applicability: "usable", case_count: 3, failed_case_count: 0,
    cases: ["callback", "refund", "total"].map((id) => ({ id, outcome: "passed", log_index: 1 })) };
  const writes = mount(configured, stageExecutionViewFixture, undefined, () => denied ? new Response(JSON.stringify({ code: "not_found", error: "not_found" }), { status: 404 }) : new Response(JSON.stringify(report)));
  await userEvent.click(await screen.findByRole("button", { name: "Approve release" }));
  const dialog = await screen.findByRole("dialog", { name: "Approve release" });
  await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Deployment target" }), stageExecutionTargetFixture.target_digest);
  await userEvent.selectOptions(within(dialog).getByRole("combobox", { name: "Preceding approval" }), stageExecutionIds.qa);
  expect(await within(dialog).findByText("ok")).toBeVisible();
  denied = true;
  await clients.at(-1)!.refetchQueries({ predicate: (query) => query.queryKey.includes("test-reports") });
  await waitFor(() => expect(screen.queryByText("ok")).not.toBeInTheDocument());
  expect(screen.queryByRole("button", { name: "Record approval" })).not.toBeInTheDocument();
  expect(screen.queryByRole("combobox", { name: "Deployment target" })).not.toBeInTheDocument();
  expect(writes).toEqual([]);
});

it("erases an open decision form's source snapshot when source read authority is lost", async () => {
  const record = structuredClone(stageCodeViewFixture);
  const writes = mount(view, record);
  await userEvent.click(await screen.findByRole("button", { name: "Approve code review" }));
  const dialog = await screen.findByRole("dialog", { name: "Approve code review" });
  await userEvent.type(within(dialog).getByRole("textbox", { name: "Reason" }), "Private review draft");
  Object.assign(record, { candidate: { ...record.candidate, baseline: null }, blocked_evidence: 1,
    observed: { pull_request: { state: "unavailable", head_sha: null }, ci: { state: "unavailable", ci_digest: null, head_sha: null, observed_at: null, check_count: 0 } },
    signable: record.signable.map((s) => ({ ...s, allowed: false, max_expires_at: null, blockers: [{ code: "stage_evidence_unavailable", stage: "code_review" }] })) });
  await clients.at(-1)!.refetchQueries();
  const unavailable = await screen.findByRole("dialog", { name: "Code candidate" });
  expect(within(unavailable).queryByDisplayValue("Private review draft")).not.toBeInTheDocument();
  expect(within(unavailable).queryByRole("button", { name: "Record approval" })).not.toBeInTheDocument();
  expect(screen.queryByText(stagePullRequestFixture.head_sha)).not.toBeInTheDocument();
  expect(writes).toEqual([]);
});

it("C04 a refreshed stale R1 discovery disables an open approval despite a still-current R4", async () => {
  const stage = structuredClone(view);const writes = mount(stage);
  await userEvent.click(await screen.findByRole("button", { name: "Approve code review" }));
  const dialog = await screen.findByRole("dialog", { name: "Approve code review" });
  await userEvent.type(within(dialog).getByRole("textbox", { name: "Reason" }), "Retain this draft");
  await userEvent.click(within(dialog).getByRole("checkbox", { name: "I reviewed this exact candidate and evidence" }));
  const save = within(dialog).getByRole("button", { name: "Record approval" });expect(save).toBeEnabled();
  stage.code_candidates.items[0]!.state = "stale";
  await clients.at(-1)!.refetchQueries({ predicate: (q) => q.queryKey.at(-1) === "view" });
  await waitFor(() => expect(save).toBeDisabled());
  expect(within(dialog).getByRole("textbox", { name: "Reason" })).toHaveValue("Retain this draft");expect(writes).toEqual([]);
});
it("C03 the current reviewer can submit a reason-only return without source evidence", async () => {
  const denied = { ...stageCodeViewFixture, candidate: { ...stageCodeViewFixture.candidate, baseline: null }, blocked_evidence: 1,
    observed: { pull_request: { state: "unavailable", head_sha: null }, ci: { state: "unavailable", ci_digest: null, head_sha: null, observed_at: null, check_count: 0 } },
    signable: stageCodeViewFixture.signable.map((s) => ({ ...s, allowed: false, max_expires_at: null, blockers: [{ code: "stage_evidence_unavailable", stage: "code_review" }] })) };
  const writes = mount(view, denied, () => ({ code: "stage_decision_recorded", collaboration_revision: "12", stage_confirmation: {
    ...stageConfirmations.decide_stage, step: "code_review", decision: "returned", expires_at: null } }));
  await userEvent.click(await screen.findByRole("button", { name: "Return from code review" }));
  const dialog = await screen.findByRole("dialog", { name: "Return from code review" });
  await userEvent.type(within(dialog).getByRole("textbox", { name: "Reason" }), "Please supply readable evidence");
  await userEvent.click(within(dialog).getByRole("checkbox", { name: "I confirm returning this candidate" }));
  await userEvent.click(within(dialog).getByRole("button", { name: "Record return" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(writes).toHaveLength(1);expect(writes[0]?.body).toMatchObject({ decision: "return", expires_at: null, depends_on: [],
    evidence: { ci_digest: null, test_report_id: null, build: null, deployment_id: null, rollback_target_digest: null } });
});
it("C02 a denied candidate cannot expose a sibling's private cached evidence while its new read is pending", async () => {
  const candidateB = { ...stageCodeViewFixture, candidate: { ...stageCodeViewFixture.candidate, candidate_id: stageIds.candidate2, content_digest: "b".repeat(64) },
    decisions: [{ ...stageCodeReviewDecisionFixture, actor_user_id: stageIds.product, reason: "PRIVATE sibling review" }] };
  const stage = { ...view, code_candidates: { items: [...view.code_candidates.items, { candidate_id: stageIds.candidate2, version: "1", content_digest: "b".repeat(64), state: "current" }], truncated: false } };
  const denied = (record: typeof stageCodeViewFixture) => ({ ...record, candidate: { ...record.candidate, baseline: null }, blocked_evidence: 1,
    observed: { pull_request: { state: "unavailable", head_sha: null }, ci: { state: "unavailable", ci_digest: null, head_sha: null, observed_at: null, check_count: 0 } },
    decisions: [], test_reports: [], execution_facts: [], signable: record.signable.map((s) => ({ ...s, allowed: false, max_expires_at: null, blockers: [{ code: "stage_evidence_unavailable", stage: "code_review" }] })) });
  let deniedSource = false, hold = false;
  let release!: (value: unknown) => void;
  mount(stage, async (path: string) => {
    if (path.endsWith(stageIds.candidate2)) {
      if (hold) return new Promise((resolve) => { release = resolve; });
      return candidateB;
    }
    return deniedSource ? denied(stageCodeViewFixture) : stageCodeViewFixture;
  });
  const section = await screen.findByRole("region", { name: "Code and approvals" });
  const select = within(section).getByRole("combobox", { name: "Code candidate" });
  await userEvent.selectOptions(select, stageIds.candidate2);
  expect(await within(section).findByText("PRIVATE sibling review")).toBeVisible();
  await userEvent.selectOptions(select, stageIds.candidate1);
  deniedSource = true;
  await clients.at(-1)!.refetchQueries({ predicate: (q) => q.queryKey.at(-1) === stageIds.candidate1 });
  await within(section).findByText("You cannot currently read the source evidence. No source details or approvals are shown.");
  hold = true;await userEvent.selectOptions(select, stageIds.candidate2);
  await waitFor(() => expect(release).toBeDefined());
  expect(within(section).queryByText("PRIVATE sibling review")).not.toBeInTheDocument();
  release({ ...denied(stageCodeViewFixture), candidate: { ...stageCodeViewFixture.candidate, candidate_id: stageIds.candidate2, content_digest: "b".repeat(64), baseline: null } });
  await within(section).findByText("You cannot currently read the source evidence. No source details or approvals are shown.");
});
