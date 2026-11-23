import { randomUUID } from "node:crypto";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test, type Browser, type BrowserContext, type TestInfo } from "@playwright/test";
import { CollaborationIssueSchema } from "../packages/core/collaboration/schema";
import { CollaborationTestReportSchema, verifyCollaborationTestReportLogs } from "../packages/core/collaboration/stage-test-report-schema";
import { CompletionAuthorizationSchema } from "../packages/core/issues/completion-schema";
import { closureActor, offerClosureResource, readClosureIssue, workflowAuthority, type ClosureActor } from "./fixtures/collaboration-multi-account-closure";
import { changePolicyInUI } from "./fixtures/collaboration-business-scenarios";
import { hostControl, hostPanel, issueRoot, openHostAcceptanceGate, openHostConsent, sendHostComment } from "./fixtures/collaboration-host-runtime";
import { nativeScreenshot, openDesktop } from "./fixtures/collaboration-native";
import { TestApiClient } from "./fixtures";
import { StageSourceFixture, createStageGitInput, createStageTestResource, joinStageSource, loseStageReply, readCode,
  readStage, readStagePolicy, showStage, stageAcceptanceConfigured, stageEnvironment, stageHash, stageObservations, stagePanel,
  stageRefusal, stageRequest, stageRoot, stageSuccess, stageUI, type StageHTTPFact } from "./fixtures/collaboration-stage-approvals";
import { StageExecutionControl, type StageControlRequest } from "./fixtures/collaboration-stage-control";
import { StageExecutionFixture, configureStageExecution, promoteStageArtifactReader, selectedRelease, stageAdvanceAPI,
  stageApprovalUI, stageControlSucceeded, stageCurrent, stageMoveUI, stageObject, stageString, stageSubmissionRequest,
  setStageM1Policy, stageAuthorizeM1, stageCompleteBody, stageArtifactEnvironment,
  stageCompletionReview, stageResponseFact, loseStageCompletionBeforeForward,
  type StageEvidenceRecorder } from "./fixtures/collaboration-stage-execution";

test.use({ actionTimeout: 20_000, viewport: { width: 1440, height: 1100 }, screenshot: "only-on-failure", trace: "off" });

/** Each scenario owns its actors and artifacts; the parent owns API/harness lifetime. */
class StageJourney {
  readonly env = stageEnvironment();
  readonly contexts: BrowserContext[] = [];
  readonly source = new StageSourceFixture();
  readonly fixture = new StageExecutionFixture();
  readonly runtimes: Awaited<ReturnType<typeof createStageTestResource>>[] = [];
  readonly errors: string[] = [];
  readonly steps: { name: string; at: string; facts: unknown }[] = [];
  readonly http: StageHTTPFact[] = [];
  readonly control: StageExecutionControl;
  readonly evidence: Record<string, unknown>;
  native?: Awaited<ReturnType<typeof openDesktop>>;
  restoreGate?: () => Promise<void>;
  originalGate?: Awaited<ReturnType<StageSourceFixture["executionGate"]>>;
  private sourceOpen = false;
  private fixtureOpen = false;
  constructor(private readonly info: TestInfo, scope: string) {
    if (info.config.workers !== 1 || info.project.retries !== 0 || info.retry !== 0) throw new Error("Stage execution requires one worker and no automatic retries");
    const socket = process.env.COLLABORATION_STAGE_CONTROL_SOCKET;
    if (!socket) throw new Error("Stage execution requires the coordinator's private control socket");
    this.control = new StageExecutionControl(socket);
    this.control.assertReady(); stageArtifactEnvironment();
    this.evidence = { scope, outcome: "running", signatures: "synthetic test accounts, not business-person sign-off",
      execution_source: "controlled test-only adapter; no real deployment, metrics, payment or model account",
      artifact_source: "actual local packager HTTP and tar bytes; ordinary producer setup is synthetic",
      not_covered: ["real S4 integrations", "installed Desktop Main", "all actual backend writer paths", "complete A12 independent phase acceptance"],
      database: this.env.database, deployment_id: this.env.deploymentId, steps: this.steps, http: this.http };
    this.persist();
  }
  persist = () => writeFileSync(this.info.outputPath("stage-execution-evidence.json"), `${JSON.stringify(this.evidence, null, 2)}\n`, { mode: 0o600 });
  record: StageEvidenceRecorder = (name, facts) => { this.steps.push({ name, at: new Date().toISOString(), facts }); this.persist(); };
  retain = (fact: StageHTTPFact) => { this.http.push(fact); this.persist(); return fact; };
  async open(browser: Browser, actor: ClosureActor, issueId: string, title: string) {
    const context = await browser.newContext(); this.contexts.push(context);
    await context.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
    const page = await context.newPage(); page.on("pageerror", (error) => this.errors.push(error.message));
    await actor.authenticateWeb(page); await showStage(page, this.env.webUrl, issueId, title);
    return page;
  }
  async close() {
    const failures: string[] = [];
    for (const resource of this.runtimes) try { await resource.close(); } catch { failures.push("controlled_host_cleanup"); }
    if (this.restoreGate) {
      try {
        await this.restoreGate();
        const after = await this.source.executionGate();
        if (!this.originalGate) throw new Error("Original gate was not retained");
        expect({ enabled: after.enabled, creation_enabled: after.creation_enabled, host_execution_enabled: after.host_execution_enabled })
          .toEqual({ enabled: this.originalGate.enabled, creation_enabled: this.originalGate.creation_enabled, host_execution_enabled: this.originalGate.host_execution_enabled });
        expect(BigInt(after.epoch)).toBeGreaterThanOrEqual(BigInt(this.originalGate.epoch));
        this.record("original gate booleans restored with monotonic epoch", after);
      } catch { failures.push("execution_gate_restore"); }
    }
    if (this.native) {
      this.errors.push(...this.native.errors);
      try { await this.native.app.close(); } catch { failures.push("native_cleanup"); }
    }
    for (const context of this.contexts) try { await context.close(); } catch { failures.push("browser_cleanup"); }
    this.control.close();
    if (this.fixtureOpen) try { await this.fixture.close(); } catch { failures.push("fixture_database_close"); }
    if (this.sourceOpen) try { await this.source.close(); } catch { failures.push("source_database_close"); }
    this.evidence.cleanup_failures = failures; this.evidence.page_errors = this.errors; this.persist();
    expect(failures).toEqual([]); expect(this.errors).toEqual([]);
  }

  async releaseReady(browser: Browser, slotBase: number) {
    const config = await fetch(`${this.env.apiUrl}/api/config`, { signal: AbortSignal.timeout(10_000), redirect: "error" });
    expect(config.status).toBe(200); expect(await config.json()).toMatchObject({ auth_mode: "password", deployment_id: this.env.deploymentId });
    this.sourceOpen = true; this.record("verified task database", await this.source.open(this.env.database));
    this.fixtureOpen = true; await this.fixture.open(this.env.database);
    const suffix = randomUUID().replaceAll("-", "").slice(0, 10);
    const register = async (label: string) => ({ ...await closureActor(this.env.apiUrl, label, suffix), username: `host${label.toLowerCase()}${suffix}` });
    const P = await register("ReleaseProduct"), D = await register("ReleaseDeveloper"), R = await register("ReleaseReviewer"),
      Q = await register("ReleaseQuality"), L = await register("ReleaseApprover"), M = await register("ReleaseMember");
    this.record("public accounts and synthetic invitation email bridge", await this.source.prepareSyntheticInvitationEmails([D, R, Q, L, M]));
    const workspace = await P.client.ensureWorkspace(`Synthetic stage execution ${suffix}`, `stage-execution-${suffix}`);
    const members = [];
    for (const actor of [D, R, Q, L, M]) {
      const member = await joinStageSource(P, actor, workspace.id); members.push(member);
      if ([R.id, Q.id, L.id].includes(actor.id)) this.retain(await promoteStageArtifactReader(P, this.env.apiUrl, workspace.id, member.member_id, actor.id));
    }
    const project = await P.client.requestJSON<{ id: string }>("/api/projects", { method: "POST", body: { title: `Synthetic stage release ${suffix}` } });
    const title = `Synthetic build release and rollback ${suffix}`;
    const created = this.retain(await stageRequest(this.env.apiUrl, P, "/api/collaboration/issues", "POST", {
      source_workspace_id: workspace.id, project_id: project.id, handler_user_id: D.id, title,
      description: "Controlled stage verification. Real payment, CI and deployment are not claimed.", ai_policy: "manual",
    }));
    expect(created.status).toBe(201);
    const id = CollaborationIssueSchema.parse(created.response_body).id;
    for (const actor of [R, Q, L, M]) {
      const added = this.retain(await stageRequest(this.env.apiUrl, P, `${issueRoot(id)}/participants`, "POST", {
        ...workflowAuthority(await readClosureIssue(P, id)), target_user_id: actor.id }));
      expect(added.status).toBe(200);
    }
    const policy = await readStagePolicy(P, project.id, workspace.id);
    expect(policy.activation_available).toBe(true);
    stageSuccess(this.retain(await stageRequest(this.env.apiUrl, P, `/api/projects/${project.id}/stage-policy`, "PUT", {
      expected_policy_revision: policy.policy_revision, enabled: true, reason: "Synthetic exact stage approval and final completion verification",
    }, undefined, { "X-Workspace-ID": workspace.id })), "revise_policy");
    const roles = { business_owner_user_id: P.id, implementer_user_id: D.id, product_approver_user_id: P.id,
      code_reviewer_user_id: R.id, qa_approver_user_id: Q.id, release_approver_user_id: L.id };
    stageSuccess(this.retain(await stageRequest(this.env.apiUrl, P, `${stageRoot(id)}/roles`, "POST", {
      ...await stageObservations(P, id, "roles"), roles })), "configure_roles");
    const child = await P.client.requestJSON<{ id: string; identifier: string }>("/api/issues", { method: "POST", body: {
      title: `Synthetic source child ${suffix}`, parent_issue_id: id, status: "in_progress", description: "Actual fixture Git source and local tar build",
    } });
    const git = createStageGitInput(suffix);
    const pr = await this.source.seedPullRequest(workspace.id, child.id, suffix, git.head);
    this.record("synthetic current-head complete CI", await this.source.observeCI(pr.prId, "passed"));
    const producer = await this.fixture.ordinaryProducer(workspace.id, child.id, P.id, suffix);
    const previousBuild = await this.fixture.packageBuild(P, this.env.apiUrl, producer, git, git.initialHead, slotBase, this.record, this.retain);
    const build = await this.fixture.packageBuild(P, this.env.apiUrl, producer, git, git.head, slotBase + 1, this.record, this.retain);
    expect(build.artifactDigest).not.toBe(previousBuild.artifactDigest);
    const view = await readStage(P, id);
    const selected = view.children.items.find((item) => item.issue_id === child.id);
    if (!selected?.can_open || !selected.content_digest) throw new Error("The exact source child is not readable for scope confirmation");
    stageSuccess(this.retain(await stageRequest(this.env.apiUrl, P, `${stageRoot(id)}/scope-confirmations`, "POST", {
      ...await stageObservations(P, id, "roles"), expected_scope_revision: view.scope_revision, reason: "Carry forward this exact synthetic Git source",
      children: [{ issue_id: child.id, content_digest: selected.content_digest,
        code: { pull_request: { provider: "github", pull_request_id: pr.prId, head_sha: git.head } } }],
    })), "confirm_scope");
    const code = stageSuccess(this.retain(await stageRequest(this.env.apiUrl, P, `${stageRoot(id)}/code-candidates`, "POST", {
      ...await stageObservations(P, id, "roles"), child_issue_id: child.id, supersedes: null,
      pull_request: { provider: "github", pull_request_id: pr.prId, head_sha: git.head }, artifact: { approval_id: build.approvalId },
    })), "create_code_candidate");
    const candidate = { candidate_id: code.candidate_id, version: code.version, content_digest: code.content_digest };
    const sources = await this.fixture.executionSources(workspace.id, suffix);
    expect(Object.values(sources)).not.toContain(pr.installationId);
    const configured = await configureStageExecution(this.control, workspace.id, sources, previousBuild.artifactDigest, suffix);
    this.record("registered controlled target with actual previous artifact", { sources, target_digest: configured.target,
      previous_artifact_digest: previousBuild.artifactDigest, calls_before: configured.callBaseline });
    this.evidence.issue_id = id; this.evidence.project_id = project.id; this.evidence.candidate = candidate;
    this.evidence.actors = { product: P.id, implementer: D.id, reviewer: R.id, qa: Q.id, release: L.id, ordinary_source_member: M.id, members };
    stageSuccess(this.retain(await stageAdvanceAPI(this.env.apiUrl, P, id, candidate, "code_review")), "advance_stage");
    const p = await this.open(browser, P, id, title), r = await this.open(browser, R, id, title), q = await this.open(browser, Q, id, title);
    const reviewForm = await stageApprovalUI(r, "code_review", {}, "Synthetic human review of actual Git/build and controlled CI evidence");
    const review = stageSuccess(this.retain(await stageUI(r, R, `${stageRoot(id)}/decisions`, "POST", reviewForm.submit)), "decide_stage");
    stageSuccess(this.retain(await stageAdvanceAPI(this.env.apiUrl, P, id, candidate, "qa", [review.decision_id])), "advance_stage");
    this.originalGate = await this.source.executionGate(); this.record("execution gate before prepared report", this.originalGate);
    this.restoreGate = await openHostAcceptanceGate();
    await showStage(p, this.env.webUrl, id, title); this.record("explicit assisted policy", await changePolicyInUI(p, id, "assisted"));
    const executionWorkspace = await Q.client.requestJSON<{ id: string; name: string; slug: string }>("/api/workspaces", { method: "POST", body: {
      name: `Synthetic release QA ${suffix}`, slug: `stage-release-qa-${suffix}` } });
    Q.client.setWorkspaceId(executionWorkspace.id); Q.client.setWorkspaceSlug(executionWorkspace.slug);
    const resource = await createStageTestResource(Q, executionWorkspace, this.env.apiUrl, this.env.deploymentId,
      `Synthetic build-bound QA ${suffix}`, this.record, git.config);
    this.runtimes.push(resource);
    const offered = await offerClosureResource(resource, P, await readClosureIssue(P, id));
    await showStage(q, this.env.webUrl, id, title); await openHostConsent(q, offered.hostIssue);
    const consent = this.retain(await stageUI(q, Q, `${issueRoot(id)}/targets/${offered.hostIssue.targetId}/consents`, "POST",
      () => hostPanel(q).getByRole("button", { name: "Authorize for this issue", exact: true }).click()));
    expect(consent.status).toBe(201);
    await showStage(p, this.env.webUrl, id, title);
    if ((await readClosureIssue(P, id)).session.queue_state === "paused") this.record("explicit issue resume", await hostControl(p, offered.hostIssue, "resume", "issue"));
    if ((await readClosureIssue(P, id)).execution?.targets.find((item) => item.id === offered.hostIssue.targetId)?.control.queue_state === "paused") {
      this.record("explicit target resume", await hostControl(p, offered.hostIssue, "resume", "target"));
    }
    await resource.beforeTrigger(); await showStage(p, this.env.webUrl, id, title);
    await stagePanel(p).getByRole("button", { name: "Prepare test input", exact: true }).click();
    let command: Awaited<ReturnType<typeof sendHostComment>> | undefined;
    const comment = this.retain(await stageUI(p, P, `${issueRoot(id)}/comments`, "POST", async () => {
      command = await sendHostComment(p, offered.hostIssue, "Synthetic build-bound regression using the exact prepared Git source.", P.id);
    }));
    expect(comment.status).toBe(201);
    expect(comment.request_body).toMatchObject({ test_subject: { candidate_id: candidate.candidate_id, candidate_version: candidate.version, candidate_digest: candidate.content_digest } });
    if (!command) throw new Error("Build-bound comment did not retain its actual command identity");
    const visit = await resource.daemon.waitForVisit(1);
    expect(visit.inputs.map((input) => input.command_id)).toContain(command.commandId);
    expect(visit.workdir_files["stage-regression.json"]).toBe(git.marker_sha256); expect(visit.workdir_files).not.toHaveProperty("untracked-private.txt");
    const reportKey = randomUUID();
    const reportOutput = (await resource.daemon.act(visit, [{ kind: "test_report", request_id: reportKey, fault: "none" }]))[0]?.result;
    stageObject(reportOutput); stageObject(reportOutput.submit);
    expect(reportOutput.submit).toEqual(reportOutput.receipt);
    expect(reportOutput.subject).toMatchObject({ subject: { candidate_id: candidate.candidate_id, source_revision: git.head, artifact_digest: build.artifactDigest } });
    const reportId = stageString(reportOutput.submit.report_id, "report ID");
    resource.daemon.release(visit);
    await expect.poll(async () => (await readClosureIssue(P, id)).execution?.runs.find((run) => run.id === visit.identity.run_id), { timeout: 30_000 })
      .toMatchObject({ progress_state: "settled", proof_state: "host_exit_observed", business_outcome: "success" });
    const reportPath = `${issueRoot(id)}/runs/${visit.identity.run_id}/test-reports/${reportId}`;
    const report = CollaborationTestReportSchema.parse(await P.client.requestJSON<unknown>(reportPath));
    expect(report).toMatchObject({ report_id: reportId, reported_result: "passed", applicability: "usable",
      subject: { candidate_id: candidate.candidate_id, source_revision: git.head, artifact_digest: build.artifactDigest } });
    expect(await verifyCollaborationTestReportLogs(report)).toBe(true);
    this.record("actual prepared CLI report binds the real build", { report, report_output: reportOutput, command, cli_sha256: visit.cli_hash,
      prepared_files: visit.workdir_files, ledger: resource.daemon.ledgerEvidence() });
    expect((await readStage(M, id)).children.items.find((item) => item.issue_id === child.id)?.can_open).toBe(true);
    const privateEvidence = await readCode(M, id, candidate.candidate_id);
    expect(privateEvidence).toMatchObject({ candidate: { baseline: null }, blocked_evidence: 1, execution_targets: [], execution_operations: [], rollback_observations: [] });
    stageRefusal(this.retain(await stageRequest(this.env.apiUrl, M, reportPath)), 404, "not_found");
    await showStage(q, this.env.webUrl, id, title);
    const qaForm = await stageApprovalUI(q, "qa", { report: reportId, dependency: review.decision_id }, "Synthetic QA reviewed this exact build-bound prepared report");
    const qa = stageSuccess(this.retain(await stageUI(q, Q, `${stageRoot(id)}/decisions`, "POST", qaForm.submit)), "decide_stage");
    await showStage(p, this.env.webUrl, id, title); this.retain(await stageMoveUI(p, P, id, "Release"));
    await resource.close();
    const profile = mkdtempSync(join(tmpdir(), "multica-stage-release-"));
    this.evidence.desktop_profile_sha256 = stageHash(profile);
    this.native = await openDesktop(profile, { token: L.client.getToken()!, deepLink: `multica://account?view=participating&issue=${id}` });
    await this.native.start();
    const l = this.native.page; await expect(l.getByRole("heading", { name: title, exact: true })).toBeVisible();
    return { id, title, suffix, P, D, R, Q, L, M, p, q, l, workspace, project, child, candidate, build, previousBuild, report, qa, review, configured, git };
  }
}

// The canonical concurrency/expiry/alias matrix is in server tests. This journey
// composes actual auth, prepared CLI reporting, human UI signatures and producer calls.
test("release and rollback keep exact builds, human decisions and controlled execution separate on Web and Desktop", async ({ browser }, info) => {
  test.setTimeout(480_000);
  test.skip(!stageAcceptanceConfigured(), "Requires the final stage API/production Web/Desktop and private controlled-source harness");
  const j = new StageJourney(info, "A06/A07/A08/A10 and shared A11 release/rollback journey; all-writer/A12 evidence remains separately required");
  try {
    const f = await j.releaseReady(browser, 0), { id, P, Q, L, candidate } = f;
    const body = { ...await stageObservations(Q, id), step: "release", decision: "approve", candidate,
      evidence: { ci_digest: null, test_report_id: null, build: { artifact_digest: f.build.artifactDigest,
        environment: f.configured.request.environment, target_digest: f.configured.target }, deployment_id: null, rollback_target_digest: null },
      depends_on: [f.qa.decision_id], reason: "QA role cannot authorize release", expires_at: new Date(Date.now() + 60_000).toISOString() };
    stageRefusal(j.retain(await stageRequest(j.env.apiUrl, Q, `${stageRoot(id)}/decisions`, "POST", body)), 403, "stage_role_required");
    const behavior = await j.control.send({ request_id: randomUUID(), action: "source.behavior", behavior: "unknown", external_operation_id: `deploy/provider:${f.suffix}` });
    stageControlSucceeded(behavior); expect(behavior.calls.count).toBe(f.configured.callBaseline);
    const releaseForm = await stageApprovalUI(f.l, "release", { dependency: f.qa.decision_id, target: f.configured.target },
      "Synthetic release approval of the exact build, QA report and controlled target");
    await expect(releaseForm.form.getByRole("region", { name: "Test report detail", exact: true })).toContainText(f.report.report_id);
    const lost = await loseStageReply(f.l, L, `${stageRoot(id)}/decisions`, releaseForm.submit, j.retain);
    const release = stageSuccess(lost, "decide_stage");
    expect(release).toMatchObject({ step: "release", decision: "approved", candidate_id: candidate.candidate_id });
    const recovering: string[] = [];
    const observeRecovery = (request: import("@playwright/test").Request) => { if (new URL(request.url()).pathname.startsWith(issueRoot(id))) recovering.push(request.method()); };
    f.l.on("request", observeRecovery);
    const recoverReply = f.l.waitForResponse((response) => new URL(response.url()).pathname === `${issueRoot(id)}/operations/${lost.request_id}` && response.request().method() === "GET");
    await releaseForm.form.getByRole("button", { name: "Check original result", exact: true }).click();
    const recovered = await recoverReply; expect(recovered.status()).toBe(200); expect(await recovered.json()).toEqual(lost.response_body);
    f.l.off("request", observeRecovery); expect(recovering.length).toBeGreaterThan(0); expect(recovering.every((method) => method === "GET")).toBe(true);
    stageRefusal(j.retain(await stageAdvanceAPI(j.env.apiUrl, P, id, candidate, "verification", [release.decision_id])), 409, "stage_execution_fact_missing");
    expect((await readCode(P, id, candidate.candidate_id)).execution_facts).toEqual([]);
    const original: Extract<StageControlRequest, { action: "execution.prepare" | "execution.receipt" }> = {
      request_id: randomUUID(), action: "execution.prepare", issue_id: id, approval_decision_id: release.decision_id, candidate, target_digest: f.configured.target,
    };
    const admitted = await j.control.send(original), admittedResult = stageControlSucceeded(admitted);
    const intent = stageString(admittedResult.intent_id, "intent ID");
    expect(admittedResult).toMatchObject({ issue_id: id, request_id: original.request_id, approval_decision_id: release.decision_id, state: "unknown", fact_id: null });
    expect(admitted.calls).toMatchObject({ count: f.configured.callBaseline + 1, last: { intent_id: intent, kind: "deployment",
      target_digest: f.configured.target, artifact_digest: f.build.artifactDigest } });
    const history = await j.control.send({ ...original, action: "execution.receipt" });
    expect(stageControlSucceeded(history)).toEqual(admittedResult); expect(history.calls).toEqual(admitted.calls);
    const unknown = await readCode(P, id, candidate.candidate_id);
    expect(unknown.execution_operations).toEqual([expect.objectContaining({ intent_id: intent, state: "unknown", fact_id: null })]);
    expect(unknown.execution_facts).toEqual([]);
    stageRefusal(j.retain(await stageRequest(j.env.apiUrl, L, `${stageRoot(id)}/decisions/${release.decision_id}/revocations`, "POST", {
      ...await stageObservations(L, id, "roles"), reason: "A committed execution intent cannot be externally cancelled by revoking approval",
    })), 409, "stage_decision_not_revocable");
    stageRefusal(j.retain(await stageAdvanceAPI(j.env.apiUrl, P, id, candidate, "verification", [release.decision_id])), 409, "stage_execution_fact_missing");
    await f.l.reload(); await expect(f.l.getByRole("region", { name: "Execution requests", exact: true })).toContainText("Execution outcome unknown");
    await expect(f.l.getByRole("region", { name: "Execution facts", exact: true })).toContainText("No deployment or rollback result is recorded.");
    j.record("approval and one uncertain intent do not imply deployment or cancellation", { release: lost, original, admitted, history, unknown });
    const observed = await j.control.send({ request_id: randomUUID(), action: "execution.observe", intent_id: intent,
      native_id: `deployment-observation-${f.suffix}`, native_version: "1", result: "succeeded", external_operation_id: `deploy/provider:${f.suffix}` });
    stageControlSucceeded(observed); expect(observed.calls).toEqual(admitted.calls);
    expect(await stageCurrent(P, id)).toBe("release");
    const success = selectedRelease(await readCode(P, id, candidate.candidate_id), release.decision_id);
    expect(success.operation.state).toBe("succeeded"); expect(success.fact).toMatchObject({ kind: "deployment", result: "succeeded",
      operation_id: `deploy/provider:${f.suffix}`, artifact_digest: f.build.artifactDigest, previous_artifact_digest: f.previousBuild.artifactDigest });
    await showStage(f.p, j.env.webUrl, id, f.title); j.retain(await stageMoveUI(f.p, P, id, "Verification"));
    expect(await stageCurrent(P, id)).toBe("verification");
    const now = Date.now(), metricWindow = { window_start: new Date(now - 5000).toISOString(), window_end: new Date(now).toISOString(),
      observed_at: new Date(now).toISOString(), valid_until: new Date(now + 120_000).toISOString() };
    const anomaly = await j.control.send({ request_id: randomUUID(), action: "metrics.observe", deployment_id: success.fact.fact_id,
      native_id: `metric-anomaly-${f.suffix}`, native_version: "1", condition: "anomaly", ...metricWindow,
      summary: "Synthetic registered metric rule observed an anomaly; this is not a real PAY metric." });
    const anomalyResult = stageControlSucceeded(anomaly), anomalyId = stageString(anomalyResult.observation_id, "metric ID");
    await f.l.reload();
    const rollbackForm = await stageApprovalUI(f.l, "rollback", { dependency: release.decision_id, deployment: success.fact.fact_id, observation: anomalyId },
      "Synthetic rollback approval for this metric, original deployment and immediately previous actual artifact");
    const rollback = stageSuccess(j.retain(await stageUI(f.l, L, `${stageRoot(id)}/decisions`, "POST", rollbackForm.submit)), "decide_stage");
    expect(rollback.step).toBe("rollback");
    const contradictory = await j.control.send({ request_id: randomUUID(), action: "metrics.observe", deployment_id: success.fact.fact_id,
      native_id: `metric-healthy-same-version-${f.suffix}`, native_version: "1", condition: "healthy", ...metricWindow, summary: "Contradictory authenticated content at the same native version" });
    stageControlSucceeded(contradictory);
    const ambiguous = await readCode(P, id, candidate.candidate_id);
    expect(ambiguous.rollback_observations?.filter((item) => item.state === "ambiguous").length).toBeGreaterThanOrEqual(2);
    const blocked = await j.control.send({ request_id: randomUUID(), action: "execution.prepare", issue_id: id,
      approval_decision_id: rollback.decision_id, candidate, target_digest: f.configured.target });
    expect(blocked.ok).toBe(false);
    if (blocked.ok) throw new Error("Ambiguous metrics admitted a rollback");
    expect(["stage_rollback_observation_changed", "stage_rollback_observation_unavailable", "stage_evidence_changed"]).toContain(blocked.code);
    expect(blocked.calls.count).toBe(admitted.calls.count);
    await f.l.reload(); await expect(f.l.getByRole("region", { name: "Rollback evidence", exact: true })).toContainText("Conflicting observations");
    j.record("equal native version ambiguity invalidates an already signed rollback", { anomaly, contradictory, rollback, blocked, ambiguous });
    const freshTime = Date.now();
    const fresh = await j.control.send({ request_id: randomUUID(), action: "metrics.observe", deployment_id: success.fact.fact_id,
      native_id: `metric-newer-${f.suffix}`, native_version: "2", condition: "anomaly", window_start: new Date(freshTime - 5000).toISOString(),
      window_end: new Date(freshTime).toISOString(), observed_at: new Date(freshTime).toISOString(), valid_until: new Date(freshTime + 120_000).toISOString(),
      summary: "A strictly newer controlled observation resolves the ambiguous native version" });
    const freshId = stageString(stageControlSucceeded(fresh).observation_id, "fresh metric ID");
    await f.l.reload();
    const finalRollbackForm = await stageApprovalUI(f.l, "rollback", { dependency: release.decision_id, deployment: success.fact.fact_id, observation: freshId },
      "Explicit fresh rollback approval; previous rejected intent and original deployment stay in history");
    const finalRollback = stageSuccess(j.retain(await stageUI(f.l, L, `${stageRoot(id)}/decisions`, "POST", finalRollbackForm.submit)), "decide_stage");
    expect(finalRollback.decision_id).not.toBe(rollback.decision_id);
    stageControlSucceeded(await j.control.send({ request_id: randomUUID(), action: "source.behavior", behavior: "succeeded", external_operation_id: `rollback/provider:${f.suffix}` }));
    const rollbackRequest: typeof original = { ...original, request_id: randomUUID(), approval_decision_id: finalRollback.decision_id };
    const rolledBack = await j.control.send(rollbackRequest), rollbackResult = stageControlSucceeded(rolledBack);
    expect(rollbackResult).toMatchObject({ kind: "rollback", state: "succeeded", approval_decision_id: finalRollback.decision_id });
    expect(rolledBack.calls).toMatchObject({ count: f.configured.callBaseline + 2, last: { intent_id: rollbackResult.intent_id,
      kind: "rollback", target_digest: f.configured.target, artifact_digest: f.previousBuild.artifactDigest } });
    const final = await readCode(P, id, candidate.candidate_id);
    expect(final.execution_facts).toHaveLength(2);
    expect(final.execution_facts).toEqual(expect.arrayContaining([expect.objectContaining({ fact_id: success.fact.fact_id, artifact_digest: f.build.artifactDigest }),
      expect.objectContaining({ fact_id: rollbackResult.fact_id, kind: "rollback", result: "succeeded", artifact_digest: f.previousBuild.artifactDigest,
        approval_decision_id: finalRollback.decision_id, operation_id: `rollback/provider:${f.suffix}` })]));
    expect(await stageCurrent(P, id)).toBe("verification");
    expect((await readClosureIssue(P, id)).status).not.toBe("done");
    const replay = await j.control.send({ ...rollbackRequest, action: "execution.receipt" });
    expect(stageControlSucceeded(replay)).toEqual(rollbackResult); expect(replay.calls).toEqual(rolledBack.calls);
    await f.l.reload(); await expect(f.l.getByRole("region", { name: "Execution facts", exact: true })).toContainText(`rollback/provider:${f.suffix}`);
    await nativeScreenshot(j.native!.app, j.native!.page, info.outputPath("stage-rollback-desktop.png"));
    j.record("exact approved rollback executed once without completing the collaboration", { original_deployment: success.fact, fresh_metric: fresh,
      rollback_approval: finalRollback, request: rollbackRequest, rolled_back: rolledBack, original_receipt: replay, final });
    j.evidence.outcome = "passed";
  } catch (error) { j.evidence.outcome = "failed"; j.evidence.failure = error instanceof Error && error.name === "TimeoutError" ? "scenario_timeout" : "scenario_failed_see_original_runner_log"; throw error; }
  finally { j.persist(); await j.close(); }
});

test("a late authenticated deployment stays historical and a fresh approval cannot endorse it", async ({ browser }, info) => {
  test.setTimeout(480_000);
  test.skip(!stageAcceptanceConfigured(), "Requires final stage sources and the task-owned private control bridge");
  const j = new StageJourney(info, "A06/A08 late result and original-request recovery across actual public approval and private production consumer");
  try {
    const f = await j.releaseReady(browser, 2), { id, L, P, candidate } = f;
    stageControlSucceeded(await j.control.send({ request_id: randomUUID(), action: "source.behavior", behavior: "unknown", external_operation_id: `late/provider:${f.suffix}` }));
    const form = await stageApprovalUI(f.l, "release", { dependency: f.qa.decision_id, target: f.configured.target },
      "Synthetic short-lived approval; later evidence must not extend this signature");
    const clock = await readCode(L, id, candidate.candidate_id);
    const expires = new Date(Date.parse(clock.checked_at) + 15_000).toISOString().slice(0, 19);
    await form.form.getByLabel("Approval expires (UTC)", { exact: true }).fill(expires);
    const approval = stageSuccess(j.retain(await stageUI(f.l, L, `${stageRoot(id)}/decisions`, "POST", form.submit)), "decide_stage");
    const original: Extract<StageControlRequest, { action: "execution.prepare" | "execution.receipt" }> = {
      request_id: randomUUID(), action: "execution.prepare", issue_id: id, approval_decision_id: approval.decision_id, candidate, target_digest: f.configured.target,
    };
    const admitted = await j.control.send(original), before = stageControlSucceeded(admitted);
    expect(before.state).toBe("unknown"); expect(admitted.calls.count).toBe(f.configured.callBaseline + 1);
    await expect.poll(async () => (await readCode(P, id, candidate.candidate_id)).decisions.find((d) => d.decision_id === approval.decision_id)?.state,
      { timeout: 30_000 }).toBe("expired");
    const late = await j.control.send({ request_id: randomUUID(), action: "execution.observe", intent_id: stageString(before.intent_id, "original intent"),
      native_id: `late-observation-${f.suffix}`, native_version: "1", result: "succeeded", external_operation_id: `late/provider:${f.suffix}` });
    stageControlSucceeded(late); expect(late.calls).toEqual(admitted.calls);
    const history = await readCode(P, id, candidate.candidate_id);
    expect(history.execution_facts).toEqual([expect.objectContaining({ approval_decision_id: approval.decision_id, result: "succeeded", artifact_digest: f.build.artifactDigest })]);
    expect(history.decisions.find((d) => d.decision_id === approval.decision_id)?.state).toBe("expired");
    stageRefusal(j.retain(await stageAdvanceAPI(j.env.apiUrl, P, id, candidate, "verification", [approval.decision_id])), 409, "stage_decision_expired");
    expect(await stageCurrent(P, id)).toBe("release");
    const receipt = await j.control.send({ ...original, action: "execution.receipt" });
    expect(stageControlSucceeded(receipt)).toMatchObject({ intent_id: before.intent_id, state: "succeeded" });
    expect(receipt.calls).toEqual(admitted.calls);
    await f.l.reload();
    const freshForm = await stageApprovalUI(f.l, "release", { dependency: f.qa.decision_id, target: f.configured.target },
      "Explicit new approval is not an endorsement of the old deployment; no new execution is requested here");
    const fresh = stageSuccess(j.retain(await stageUI(f.l, L, `${stageRoot(id)}/decisions`, "POST", freshForm.submit)), "decide_stage");
    expect(fresh.decision_id).not.toBe(approval.decision_id);
    stageRefusal(j.retain(await stageAdvanceAPI(j.env.apiUrl, P, id, candidate, "verification", [fresh.decision_id])), 409, "stage_execution_fact_missing");
    const finalReceipt = await j.control.send({ ...original, action: "execution.receipt" });
    expect(finalReceipt.calls).toEqual(admitted.calls);
    expect((await readCode(P, id, candidate.candidate_id)).execution_facts).toEqual(history.execution_facts);
    expect(await stageCurrent(P, id)).toBe("release");
    j.record("late original fact is retained without renewing approval, endorsement, advance or another invocation", {
      original, approval, admitted, late, receipt, history, fresh_approval: fresh, final_receipt: finalReceipt,
    });
    j.evidence.outcome = "passed";
  } catch (error) { j.evidence.outcome = "failed"; j.evidence.failure = error instanceof Error && error.name === "TimeoutError" ? "scenario_timeout" : "scenario_failed_see_original_runner_log"; throw error; }
  finally { j.persist(); await j.close(); }
});

test("stage evidence gates real submission, M1 completion and administrative or project exits", async ({ browser }, info) => {
  test.setTimeout(480_000);
  test.skip(!stageAcceptanceConfigured(), "Requires final stage/M1 writer integration and the task-owned password/admin environment");
  if (process.env.E2E_PLATFORM_ADMIN !== "1" || !process.env.E2E_PLATFORM_ADMIN_USERNAME || !process.env.E2E_PLATFORM_ADMIN_PASSWORD) {
    throw new Error("Stage completion acceptance requires the separately allocated synthetic administrator");
  }
  const j = new StageJourney(info, "A09 representative actual submit/self-complete/R38/project exits plus valid M1 enforce consumption; full writer inventory is backend evidence");
  try {
    const f = await j.releaseReady(browser, 4), { id, P, D, L, candidate } = f;
    const selfCreated = j.retain(await stageRequest(j.env.apiUrl, P, "/api/collaboration/issues", "POST", {
      source_workspace_id: f.workspace.id, project_id: f.project.id, handler_user_id: P.id,
      title: `Synthetic unfinished self-completion ${f.suffix}`, description: "This managed root has no reviewed stage evidence", ai_policy: "manual",
    }));
    expect(selfCreated.status).toBe(201);
    const selfId = CollaborationIssueSchema.parse(selfCreated.response_body).id;
    const selfChild = await P.client.requestJSON<{ id: string }>("/api/issues", { method: "POST", body: {
      title: `Synthetic self-root source ${f.suffix}`, parent_issue_id: selfId, status: "in_progress" } });
    const selfSuffix = randomUUID().replaceAll("-", "").slice(0, 10);
    const selfPR = await j.source.seedPullRequest(f.workspace.id, selfChild.id, selfSuffix, f.git.head);
    j.record("self-root synthetic PR/CI", await j.source.observeCI(selfPR.prId, "passed"));
    const selfProducer = await j.fixture.ordinaryProducer(f.workspace.id, selfChild.id, P.id, selfSuffix);
    const selfBuild = await j.fixture.packageBuild(P, j.env.apiUrl, selfProducer, f.git, f.git.head, 6, j.record, j.retain);
    for (const mode of ["off", "observe", "enforce"] as const) {
      const policy = await setStageM1Policy(P, j.env.apiUrl, f.workspace.id, f.project.id, mode); j.retain(policy.fact);
      const beforeSubmit = await j.fixture.rootEffects(id);
      const submission = j.retain(await stageRequest(j.env.apiUrl, D, `${issueRoot(id)}/acceptance-submissions`, "POST",
        await stageSubmissionRequest(D, id, `Synthetic ${mode} submission without an executed deployment`)));
      stageRefusal(submission, 409, "stage_incomplete"); expect(submission.response_body).toHaveProperty("stage_blockers");
      expect(await j.fixture.rootEffects(id)).toEqual(beforeSubmit);
      const beforeComplete = await j.fixture.rootEffects(selfId);
      let completeBody = await stageCompleteBody(P, selfId, { result: "Synthetic self result still has no stage evidence" });
      let completeKey = randomUUID(), authorizationId: string | undefined;
      if (mode === "enforce") {
        const authorized = await stageAuthorizeM1(j.fixture, P, j.env.apiUrl, { issueId: selfId, workspaceId: f.workspace.id,
          projectId: f.project.id, policyRevision: policy.revision, build: selfBuild, result: "Synthetic self result still has no stage evidence" });
        j.retain(authorized.fact); authorizationId = authorized.authorizationId; completeBody = authorized.body; completeKey = authorized.operationId;
      }
      const completed = j.retain(await stageRequest(j.env.apiUrl, P, `${issueRoot(selfId)}/lifecycle`, "POST", completeBody, completeKey));
      stageRefusal(completed, 409, "stage_incomplete"); expect(completed.response_body).toHaveProperty("stage_blockers");
      expect(await j.fixture.rootEffects(selfId)).toEqual(beforeComplete);
      if (authorizationId) expect(await P.client.requestJSON(`/api/delivery-completion-authorizations/${authorizationId}`)).toMatchObject({ state: "issued" });
      j.record("M1 mode does not bypass missing stage evidence", { mode, submission, completed, authorization_id: authorizationId ?? null });
    }
    j.retain((await setStageM1Policy(P, j.env.apiUrl, f.workspace.id, f.project.id, "off")).fact);
    const otherProject = await P.client.requestJSON<{ id: string }>("/api/projects", { method: "POST", body: { title: `Synthetic exit target ${f.suffix}` } });
    for (const projectId of [null, otherProject.id]) {
      const before = await j.fixture.rootEffects(id);
      const denied = j.retain(await stageRequest(j.env.apiUrl, P, issueRoot(id), "PATCH", {
        ...workflowAuthority(await readClosureIssue(P, id)), project_id: projectId }));
      stageRefusal(denied, 409, "stage_policy_exit_required"); expect(await j.fixture.rootEffects(id)).toEqual(before);
    }
    const impact = j.retain(await stageRequest(j.env.apiUrl, P, `/api/projects/${f.project.id}/delete-impact`, "GET", undefined, undefined, { "X-Workspace-ID": f.workspace.id }));
    expect(impact.status).toBe(200); expect(JSON.stringify(impact.response_body)).toContain("stage_policy_exit_required");
    stageRefusal(j.retain(await stageRequest(j.env.apiUrl, P, `/api/projects/${f.project.id}`, "DELETE", undefined, undefined,
      { "X-Workspace-ID": f.workspace.id })), 409, "stage_policy_exit_required");
    const unmanagedDelete = j.retain(await stageRequest(j.env.apiUrl, P, `/api/projects/${otherProject.id}`, "DELETE", undefined, undefined, { "X-Workspace-ID": f.workspace.id }));
    expect(unmanagedDelete.status).toBe(204);

    const finalPolicy = await setStageM1Policy(P, j.env.apiUrl, f.workspace.id, f.project.id, "enforce");
    j.retain(finalPolicy.fact);

    const releaseForm = await stageApprovalUI(f.l, "release", { dependency: f.qa.decision_id, target: f.configured.target },
      "Synthetic exact release before final M1 enforce acceptance");
    const release = stageSuccess(j.retain(await stageUI(f.l, L, `${stageRoot(id)}/decisions`, "POST", releaseForm.submit)), "decide_stage");
    stageControlSucceeded(await j.control.send({ request_id: randomUUID(), action: "source.behavior", behavior: "succeeded", external_operation_id: `final/provider:${f.suffix}` }));
    const execution = await j.control.send({ request_id: randomUUID(), action: "execution.prepare", issue_id: id,
      approval_decision_id: release.decision_id, candidate, target_digest: f.configured.target });
    expect(stageControlSucceeded(execution)).toMatchObject({ state: "succeeded" });
    expect(execution.calls.count).toBe(f.configured.callBaseline + 1);
    await showStage(f.p, j.env.webUrl, id, f.title); j.retain(await stageMoveUI(f.p, P, id, "Verification"));
    const d = await j.open(browser, D, id, f.title);
    await d.getByRole("button", { name: "Submit result", exact: true }).click();
    const submissionForm = d.getByRole("form", { name: "Submit result", exact: true });
    await submissionForm.getByLabel("Result", { exact: true }).fill("Synthetic exact build and controlled deployment verified");
    await submissionForm.getByLabel("Basis", { exact: true }).fill("Actual Git-bound CLI report, human QA/release and matching controlled execution fact");
    const submitted = j.retain(await stageUI(d, D, `${issueRoot(id)}/acceptance-submissions`, "POST", () => submissionForm.getByRole("button", { name: "Submit result", exact: true }).click()));
    expect(submitted.status).toBe(200); stageObject(submitted.response_body); stageObject(submitted.response_body.delivery_confirmation);
    const submissionId = stageString(submitted.response_body.delivery_confirmation.submission_id, "exact submission ID");
    const authorizationPath = "/api/delivery-completion-authorizations", lifecyclePath = `${issueRoot(id)}/lifecycle`;
    const webWrites: { path: string; key: string | null }[] = [], desktopWrites: { path: string; key: string | null }[] = [];
    const track = (writes: typeof webWrites) => (request: import("@playwright/test").Request) => {
      const path = new URL(request.url()).pathname;
      // The POST completion-preview is a read. Only authority/final writes count here.
      if (request.method() === "POST" && [authorizationPath, lifecyclePath].includes(path)) writes.push({ path, key: request.headers()["idempotency-key"] ?? null });
    };
    const observeWeb = track(webWrites); f.p.on("request", observeWeb);
    j.record("shared M1 transport trace", { web_writes: webWrites, desktop_writes: desktopWrites });
    await showStage(f.p, j.env.webUrl, id, f.title);
    const selected = { issueId: id, childId: f.child.id, approvalId: f.build.approvalId, artifactDigest: f.build.artifactDigest,
      sourceRevision: f.build.sourceRevision, submissionId };
    const webReview = await stageCompletionReview(f.p, P, selected, j.retain);
    expect(webReview.preview.policy_revision).toBe(finalPolicy.revision);
    await webReview.gate.getByRole("textbox", { name: "Authorization reason", exact: true }).fill("Synthetic human reviewed this exact child build and submitted result in the shared Web form");
    await webReview.gate.getByRole("button", { name: "Use latest expiry", exact: true }).click();
    const [authorizeResponse] = await Promise.all([
      f.p.waitForResponse((response) => new URL(response.url()).pathname === authorizationPath && response.request().method() === "POST", { timeout: 20_000 }),
      webReview.gate.getByRole("button", { name: "Authorize exact completion", exact: true }).click(),
    ]);
    const authorized = j.retain(await stageResponseFact(authorizeResponse, P));
    expect(authorized.status).toBe(201);
    const issued = CompletionAuthorizationSchema.parse(authorized.response_body);
    expect(issued).toMatchObject({ issue_id: id, state: "issued", target_status: "done" });
    expect(authorized.request_body).toMatchObject(webReview.preview.authorization_binding!);
    expect(authorized.request_body).toMatchObject({ operation_id: issued.operation_id, evidence: { issue_id: f.child.id,
      approval_id: f.build.approvalId, expected_artifact_digest: f.build.artifactDigest }, target: { kind: "collaboration_submission", submission_id: submissionId, acceptance_note: "" } });
    await expect(webReview.gate.getByText("The selected authorization is ready for this completion.", { exact: true })).toBeVisible();
    const beforeCompletion = await j.fixture.rootEffects(id);
    const interrupted = await loseStageCompletionBeforeForward(f.p, P, lifecyclePath,
      () => webReview.form.getByRole("button", { name: "Accept and complete", exact: true }).click());
    expect(interrupted.request_id).toBe(issued.operation_id);
    expect(interrupted.request_body).toMatchObject({ action: "complete", submission_id: submissionId,
      completion: { authorization_id: issued.authorization_id, operation_id: issued.operation_id, expected_issue_revision: issued.issue_revision } });
    j.record("Web final attempt interrupted before API; exact original intent retained", { authorized, interrupted });
    const originalPath = `${issueRoot(id)}/operations/${issued.operation_id}`, grantPath = `${authorizationPath}/${issued.authorization_id}`;
    const notFoundResponse = f.p.waitForResponse((response) => new URL(response.url()).pathname === originalPath && response.request().method() === "GET", { timeout: 20_000 });
    const [notFoundHTTP] = await Promise.all([notFoundResponse, f.p.getByRole("button", { name: "Check original result", exact: true }).click()]);
    const notFound = j.retain(await stageResponseFact(notFoundHTTP, P));
    expect(notFound.status).toBe(404);
    await expect(f.p.getByRole("button", { name: "Check original result", exact: true })).toBeEnabled();
    expect(await j.fixture.rootEffects(id)).toEqual(beforeCompletion);
    expect(webWrites.filter((write) => write.path === authorizationPath)).toHaveLength(1);
    expect(webWrites.filter((write) => write.path === lifecyclePath)).toEqual([{ path: lifecyclePath, key: issued.operation_id }]);
    j.record("real original GET404 did not authorize or submit again", { not_found: notFound, original_operation_id: issued.operation_id, web_writes: webWrites });

    // A fresh native profile uses P's actual authenticated account. It does not
    // copy Web storage or invent a completion request/authorization identifier.
    if (!j.native) throw new Error("The release Desktop fixture is missing");
    j.errors.push(...j.native.errors); await j.native.app.close(); j.native = undefined;
    const completionProfile = mkdtempSync(join(tmpdir(), "multica-stage-completion-"));
    j.evidence.completion_desktop_profile_sha256 = stageHash(completionProfile);
    j.native = await openDesktop(completionProfile, { token: P.client.getToken()!, deepLink: `multica://account?view=participating&issue=${id}` });
    await j.native.start();
    const desktop = j.native.page, observeDesktop = track(desktopWrites); desktop.on("request", observeDesktop);
    await expect(desktop.getByRole("heading", { name: f.title, exact: true })).toBeVisible();
    const desktopReview = await stageCompletionReview(desktop, P, selected, j.retain);
    expect(desktopReview.preview.authorization_binding).toEqual(webReview.preview.authorization_binding);
    await desktopReview.gate.getByRole("combobox", { name: "Authorization method", exact: true }).selectOption("existing");
    await desktopReview.gate.getByRole("textbox", { name: "Authorization ID", exact: true }).fill(issued.authorization_id);
    const referenceResponse = desktop.waitForResponse((response) => new URL(response.url()).pathname === grantPath && response.request().method() === "GET", { timeout: 20_000 });
    const [referenceHTTP] = await Promise.all([referenceResponse, desktopReview.gate.getByRole("button", { name: "Read authorization", exact: true }).click()]);
    const referenced = j.retain(await stageResponseFact(referenceHTTP, P));
    expect(referenced.status).toBe(200);
    expect(CompletionAuthorizationSchema.parse(referenced.response_body)).toMatchObject({ authorization_id: issued.authorization_id,
      operation_id: issued.operation_id, state: "issued", issue_id: id });
    await expect(desktopReview.gate.getByText("The selected authorization is ready for this completion.", { exact: true })).toBeVisible();
    const consumedResponse = desktop.waitForResponse((response) => new URL(response.url()).pathname === grantPath && response.request().method() === "GET", { timeout: 20_000 });
    const [completed, consumed] = await Promise.all([
      desktop.waitForResponse((response) => new URL(response.url()).pathname === lifecyclePath && response.request().method() === "POST", { timeout: 20_000 })
        .then((response) => stageResponseFact(response, P)).then(j.retain),
      consumedResponse.then((response) => stageResponseFact(response, P)).then(j.retain),
      desktopReview.form.getByRole("button", { name: "Accept and complete", exact: true }).click(),
    ]);
    expect(completed.status).toBe(200); expect(completed.response_body).toMatchObject({ code: "completed" });
    expect(completed.request_id).toBe(interrupted.request_id);
    expect(completed.request_body).toEqual(interrupted.request_body); expect(completed.request_sha256).toBe(interrupted.request_sha256);
    expect(consumed.status).toBe(200);
    const authorization = CompletionAuthorizationSchema.parse(consumed.response_body);
    expect(authorization).toMatchObject({ authorization_id: issued.authorization_id, operation_id: issued.operation_id, state: "consumed",
      issue_id: id, completion_receipt: { authorization_id: issued.authorization_id, operation_id: issued.operation_id, issue_id: id, submission_id: submissionId } });
    expect(authorization.completion_receipt?.collaboration_revision).toBe((BigInt(String(interrupted.request_body.expected_collaboration_revision)) + 1n).toString());
    await expect(desktopReview.form).toHaveCount(0);
    expect(desktopWrites.filter((write) => write.path === authorizationPath)).toEqual([]);
    expect(desktopWrites.filter((write) => write.path === lifecyclePath)).toEqual([{ path: lifecyclePath, key: issued.operation_id }]);
    j.record("Desktop explicit shared-form completion consumed the original Web authorization", {
      web_preview: webReview.fact, desktop_preview: desktopReview.fact, reference_read: referenced, completed, consumed,
    });

    const recoveredResponse = f.p.waitForResponse((response) => new URL(response.url()).pathname === originalPath && response.request().method() === "GET", { timeout: 20_000 });
    const webConsumedResponse = f.p.waitForResponse((response) => new URL(response.url()).pathname === grantPath && response.request().method() === "GET", { timeout: 20_000 });
    const [recovered, webConsumed] = await Promise.all([
      recoveredResponse.then((response) => stageResponseFact(response, P)).then(j.retain),
      webConsumedResponse.then((response) => stageResponseFact(response, P)).then(j.retain),
      f.p.getByRole("button", { name: "Check original result", exact: true }).click(),
    ]);
    expect(recovered.status).toBe(200); expect(recovered.response_body).toEqual(completed.response_body);
    expect(webConsumed.status).toBe(200);
    expect(CompletionAuthorizationSchema.parse(webConsumed.response_body).completion_receipt).toEqual(authorization.completion_receipt);
    await expect(f.p.getByRole("button", { name: "Check original result", exact: true })).toHaveCount(0);
    expect(webWrites.filter((write) => write.path === authorizationPath)).toHaveLength(1);
    expect(webWrites.filter((write) => write.path === lifecyclePath)).toEqual([{ path: lifecyclePath, key: issued.operation_id }]);
    expect((await readClosureIssue(P, id)).status).toBe("done");
    f.p.off("request", observeWeb); desktop.off("request", observeDesktop);
    await nativeScreenshot(j.native.app, desktop, info.outputPath("stage-m1-desktop-completed.png"));
    j.record("shared Web and Desktop M1 journey restored only the original intent", { submitted, authorized, interrupted,
      not_found: notFound, completed, consumed, recovered, web_consumed: webConsumed, execution,
      final_effects: await j.fixture.rootEffects(id), web_writes: webWrites, desktop_writes: desktopWrites,
      actual_final_api_writes: 1, interrupted_browser_attempts: 1, storage_copied_between_platforms: false });

    // R38's stage gate is independently exercised in off mode; the valid M1
    // enforce binding above covers the separate final-consumer requirement.
    j.retain((await setStageM1Policy(P, j.env.apiUrl, f.workspace.id, f.project.id, "off")).fact);
    const administrator = new TestApiClient();
    const admin = await administrator.loginPassword(process.env.E2E_PLATFORM_ADMIN_USERNAME!, process.env.E2E_PLATFORM_ADMIN_PASSWORD!);
    const authority = await administrator.requestJSON<{ role: string; supported: boolean; organization_id: string }>("/api/admin/me");
    expect(authority).toMatchObject({ role: "super_admin", supported: true });
    let disabled = false;
    const changeAccount = async (action: "disable" | "restore") => {
      const account = await administrator.requestJSON<{ user: { auth_version: number } }>(`/api/admin/users/${P.id}`);
      const key = randomUUID();
      const result = await administrator.requestJSON<{ id: string; state: string }>(`/api/admin/users/${P.id}/${action}`, { method: "POST", headers: { "Idempotency-Key": key },
        body: { expected_auth_version: account.user.auth_version, password: process.env.E2E_PLATFORM_ADMIN_PASSWORD,
          reason: `Synthetic stage acceptance ${action} of this scenario's creator only` } });
      expect(result.state).toBe("applied"); j.record("synthetic creator administration", { request_id: key, operation_id: result.id, actor_id: admin.id, target_id: P.id, action });
    };
    try {
      await changeAccount("disable"); disabled = true;
      const path = `/api/admin/collaboration/issues/${selfId}`;
      const detail = await administrator.requestJSON<{ status: string; collaboration_revision: string; actions: { action: string; default_status: string; status_options_version: string }[] }>(path);
      const option = detail.actions.find((item) => item.action === "complete");
      if (!option) throw new Error("Synthetic administrator has no current complete option");
      const before = await j.fixture.rootEffects(selfId);
      const response = await fetch(`${j.env.apiUrl}${path}/lifecycle`, { method: "POST", redirect: "error", signal: AbortSignal.timeout(20_000), headers: {
        Authorization: `Bearer ${administrator.getToken()!}`, "Content-Type": "application/json", "Idempotency-Key": randomUUID() }, body: JSON.stringify({
        action: "complete", expected_collaboration_revision: detail.collaboration_revision, expected_status: detail.status,
        target_status: option.default_status, status_options_version: option.status_options_version, reason: "R38 must not exempt an unfinished managed root from stage evidence",
      }) });
      expect(response.status).toBe(409);
      const refusal: unknown = await response.json(); expect(refusal).toMatchObject({ code: "stage_incomplete" });
      expect(refusal).toHaveProperty("stage_blockers"); expect(await j.fixture.rootEffects(selfId)).toEqual(before);
      j.record("R38 cannot bypass missing stage evidence", { issue_id: selfId, status: response.status, refusal, actor_id: admin.id });
    } finally { if (disabled) await changeAccount("restore"); }
    j.evidence.outcome = "passed_representative_writers";
  } catch (error) { j.evidence.outcome = "failed"; j.evidence.failure = error instanceof Error && error.name === "TimeoutError" ? "scenario_timeout" : "scenario_failed_see_original_runner_log"; throw error; }
  finally { j.persist(); await j.close(); }
});
