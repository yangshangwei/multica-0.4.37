import { randomUUID } from "node:crypto";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import type { CollaborationRunRecords, CollaborationStageCandidateRef } from "@multica/core/collaboration";
import { CollaborationIssueSchema } from "../packages/core/collaboration/schema";
import { CollaborationTestReportSchema, CollaborationTestReportsSchema, verifyCollaborationTestReportLogs } from "../packages/core/collaboration/stage-test-report-schema";
import { hostControl, hostEnvironment, hostPanel, issueRoot, openHostAcceptanceGate, openHostConsent, sendHostComment, versioned, type HostVisit } from "./fixtures/collaboration-host-runtime";
import { closureActor, offerClosureResource, readClosureIssue, workflowAuthority, type ClosureActor } from "./fixtures/collaboration-multi-account-closure";
import { changePolicyInUI } from "./fixtures/collaboration-business-scenarios";
import { nativeScreenshot, openDesktop, type NativeHarness } from "./fixtures/collaboration-native";
import {
  StageSourceFixture, confirmStageScope, createStageGitInput, createStageTestResource, joinStageSource, loseStageReply, readCode, readProposal, readStage, readStagePolicy,
  showStage, stageAcceptanceConfigured, stageEnvironment, stageHash, stageObservations, stagePanel, stageRefusal,
  stageRequest, stageRoot, stageSuccess, stageUI, type StageHTTPFact,
} from "./fixtures/collaboration-stage-approvals";

test.use({ actionTimeout: 20_000, viewport: { width: 1440, height: 1100 }, screenshot: "only-on-failure", trace: "off" });

function requireObject(value: unknown): asserts value is Record<string, unknown> {
  if (value === null || typeof value !== "object" || Array.isArray(value)) throw new Error("Fixture returned a non-object receipt");
}

// Canonical boundary matrices remain in Core and server tests. This real UI
// journey composes role, version, original-key and source-privacy behavior.
test("stage decisions and prepared test reports retain human and runtime history across Web and Desktop", async ({ browser }, info) => {
  test.setTimeout(480_000);
  test.skip(!stageAcceptanceConfigured(), "Requires explicit stage acceptance, task-owned password API, production Web and matching Desktop build");
  const env = stageEnvironment();
  expect(hostEnvironment()).toEqual({ apiUrl: env.apiUrl, deploymentId: env.deploymentId });
  expect(info.project.use.baseURL).toBe(env.webUrl);
  const contexts: BrowserContext[] = [];
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  let restoreGate: (() => Promise<void>) | undefined;
  let originalGate: Awaited<ReturnType<StageSourceFixture["executionGate"]>> | undefined;
  const runtimes: Awaited<ReturnType<typeof createStageTestResource>>[] = [];
  const source = new StageSourceFixture();
  const steps: { name: string; at: string; facts: unknown }[] = [];
  const http: StageHTTPFact[] = [];
  const errors: string[] = [];
  const evidence: Record<string, unknown> = {
    scope: "A01/A02/A03/A04/A05/A08/A10 composed subset; shared Web and isolated Electron renderer",
    signatures: "synthetic test accounts; not business-person sign-off",
    connector: "synthetic local PR and CI observations; no remote provider verification",
    not_covered: ["A06 release approval", "A07 rollback", "A09 complete writer inventory",
      "complete A11 platform acceptance", "A12 independent phase acceptance", "real payment/fulfillment", "commercial models", "installed Desktop Main"],
    environment: { database: env.database, deployment_id: env.deploymentId, web_origin: new URL(env.webUrl).origin,
      desktop_renderer_origin: new URL(process.env.COLLABORATION_ELECTRON_RENDERER_URL!).origin },
    steps, http, outcome: "running",
  };
  const persist = () => writeFileSync(info.outputPath("stage-approvals-evidence.json"), `${JSON.stringify(evidence, null, 2)}\n`, { mode: 0o600 });
  const record = (name: string, facts: unknown) => { steps.push({ name, at: new Date().toISOString(), facts }); persist(); };
  const retain = (fact: StageHTTPFact) => { http.push(fact); persist(); return fact; };
  persist();
  try {
    const configResponse = await fetch(`${env.apiUrl}/api/config`, { redirect: "error" });
    expect(configResponse.status).toBe(200);
    expect(await configResponse.json()).toMatchObject({ auth_mode: "password", deployment_id: env.deploymentId });
    record("isolated database identity", await source.open(env.database));
    const suffix = randomUUID().replaceAll("-", "").slice(0, 10);
    const gitInput = createStageGitInput(suffix);
    record("actual task-owned Git commits and bundle", { initial_head: gitInput.initialHead, head: gitInput.head,
      bundle_digest: gitInput.config.bundle_digest, size_bytes: gitInput.config.size_bytes, object_format: gitInput.config.object_format,
      marker_sha256: gitInput.marker_sha256, git_executable_sha256: gitInput.git_executable_sha256 });
    const register = async (label: string) => {
      const actor = await closureActor(env.apiUrl, label, suffix);
      const username = `host${label.toLowerCase()}${suffix}`;
      record("public synthetic password registration", { actor_id: actor.id, username });
      return { ...actor, username };
    };
    const P = await register("StageProduct");
    const D = await register("StageDeveloper");
    const R = await register("StageReviewer");
    const Q = await register("StageQuality");
    const L = await register("StageRelease");
    const E = await register("StageExternal");
    const X = await register("StageOutsider");
    record("synthetic email-only setup for public invitations; not public profile acceptance", await source.prepareSyntheticInvitationEmails([D, R, Q, L]));
    const workspace = await P.client.ensureWorkspace(`Synthetic stage source ${suffix}`, `stage-source-${suffix}`);
    const sourceMembers = [];
    for (const actor of [D, R, Q, L]) sourceMembers.push(await joinStageSource(P, actor, workspace.id));
    const project = await P.client.requestJSON<{ id: string }>("/api/projects", { method: "POST", body: { title: `Synthetic PAY-241 stage ${suffix}` } });
    const title = `Synthetic payment collaboration stage ${suffix}`;
    const created = retain(await stageRequest(env.apiUrl, P, "/api/collaboration/issues", "POST", {
      source_workspace_id: workspace.id, project_id: project.id, handler_user_id: D.id, title,
      description: "Synthetic stage acceptance. Real payment and deployment evidence is not claimed.", ai_policy: "manual",
    }));
    expect(created.status).toBe(201);
    const id = CollaborationIssueSchema.parse(created.response_body).id;
    expect(id).toMatch(/^[0-9a-f-]{36}$/);
    evidence.issue_id = id;
    evidence.project_id = project.id;
    evidence.actors = { product_business_owner: P.id, implementer_handler: D.id, code_reviewer: R.id, qa_approver: Q.id,
      release_approver: L.id, external_participant: E.id, unrelated: X.id, source_workspace: workspace.id, source_members: sourceMembers };
    for (const actor of [R, Q, L, E]) {
      const invitation = retain(await stageRequest(env.apiUrl, P, `${issueRoot(id)}/participants`, "POST", {
        ...workflowAuthority(await readClosureIssue(P, id)), target_user_id: actor.id,
      }));
      expect(invitation.status).toBe(200);
      expect(invitation.response_body).toMatchObject({ code: "participant_added" });
    }
    const openWeb = async (actor: ClosureActor) => {
      const context = await browser.newContext(); contexts.push(context);
      await context.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
      const page = await context.newPage();
      page.on("pageerror", (error) => errors.push(error.message));
      await actor.authenticateWeb(page);
      await showStage(page, env.webUrl, id, title);
      return page;
    };
    const p = await openWeb(P), d = await openWeb(D), external = await openWeb(E);
    const show = (page: Page) => showStage(page, env.webUrl, id, title);
    const policy = await readStagePolicy(P, project.id, workspace.id);
    expect(policy).toMatchObject({ enabled: false, activation_available: true });
    expect(await source.effects(id)).toEqual({ children: 0, mapped_children: 0, commands: 0, runs: 0 });
    await stagePanel(p).getByRole("button", { name: "Project stage policy", exact: true }).click();
    const policyPanel = p.getByRole("region", { name: "Project stage policy", exact: true });
    await policyPanel.getByLabel("Reason", { exact: true }).fill("Synthetic acceptance: require exact human stage decisions.");
    const enabled = retain(await stageUI(p, P, `/api/projects/${project.id}/stage-policy`, "PUT",
      () => policyPanel.getByRole("button", { name: "Enable stage gates", exact: true }).click()));
    expect(stageSuccess(enabled, "revise_policy")).toMatchObject({ project_id: project.id, enabled: true });
    const policyRecovery = retain(await stageRequest(env.apiUrl, P, `/api/projects/${project.id}/stage-policy/operations/${enabled.request_id}`,
      "GET", undefined, undefined, { "X-Workspace-ID": workspace.id }));
    stageSuccess(policyRecovery, "revise_policy");
    expect(policyRecovery.response_body).toEqual(enabled.response_body);
    expect(await source.effects(id)).toEqual({ children: 0, mapped_children: 0, commands: 0, runs: 0 });
    record("P2 enables policy through the actual shared UI without configuring roles, approving children or dispatching", { policy: enabled.response_body });

    const roles = { business_owner_user_id: P.id, implementer_user_id: D.id, product_approver_user_id: P.id,
      code_reviewer_user_id: R.id, qa_approver_user_id: Q.id, release_approver_user_id: L.id };
    const selfRole = retain(await stageRequest(env.apiUrl, P, `${stageRoot(id)}/roles`, "POST", {
      ...await stageObservations(P, id, "roles"), roles: { ...roles, code_reviewer_user_id: D.id },
    }));
    stageRefusal(selfRole, 409, "stage_self_review_forbidden");
    const qaRelease = retain(await stageRequest(env.apiUrl, P, `${stageRoot(id)}/roles`, "POST", {
      ...await stageObservations(P, id, "roles"), roles: { ...roles, release_approver_user_id: Q.id },
    }));
    stageRefusal(qaRelease, 409, "stage_qa_release_conflict");
    await show(p);
    await stagePanel(p).getByRole("button", { name: "Configure roles", exact: true }).click();
    const roleForm = p.getByRole("form", { name: "Configure roles", exact: true });
    for (const [name, actor] of [["Business owner", P], ["Implementer", D], ["Product approver", P], ["Code reviewer", R], ["QA approver", Q], ["Release approver", L]] as const) {
      await roleForm.getByLabel(name, { exact: true }).selectOption(actor.id);
    }
    const configured = retain(await stageUI(p, P, `${stageRoot(id)}/roles`, "POST", () => roleForm.getByRole("button", { name: "Save roles", exact: true }).click()));
    stageSuccess(configured, "configure_roles");
    await expect(roleForm).toHaveCount(0);
    expect((await readStage(P, id)).roles?.business_owner?.user_id).toBe(P.id);
    expect((await readClosureIssue(P, id)).handler.user_id).toBe(D.id);
    retain(await confirmStageScope(p, P, id, await readStage(P, id)));
    record("A01 role separation and stable owner differ from current handler", { roles, handler_id: D.id, rejected: [selfRole.response_body, qaRelease.response_body] });

    const fileProposal = async (summary: string, childTitle: string) => {
      await show(d);
      await stagePanel(d).getByRole("button", { name: "File a proposal", exact: true }).click();
      const form = d.getByRole("form", { name: "File a proposal", exact: true });
      await form.getByLabel("Summary", { exact: true }).fill(summary);
      await form.getByLabel("Title", { exact: true }).fill(childTitle);
      await form.getByLabel("Description", { exact: true }).fill("Synthetic proposed work; filing starts nothing.");
      const fact = retain(await stageUI(d, D, `${stageRoot(id)}/proposals`, "POST", () => form.getByRole("button", { name: "File proposal", exact: true }).click()));
      await expect(form).toHaveCount(0);
      return stageSuccess(fact, "create_proposal");
    };
    const summary = `Synthetic duplicate callback review ${suffix}`;
    const proposal = await fileProposal(summary, `Synthetic approved follow-up ${suffix}`);
    expect(await source.effects(id)).toEqual({ children: 0, mapped_children: 0, commands: 0, runs: 0 });
    await show(p);
    await stagePanel(p).getByRole("button", { name: `Open proposal: ${summary}`, exact: true }).click();
    await p.locator(`[data-proposal-id="${proposal.proposal_id}"]`).getByRole("button", { name: "Modify and accept", exact: true }).click();
    const modify = p.getByRole("form", { name: "Modify and accept · step 1 of 2", exact: true });
    await modify.getByLabel("Description", { exact: true }).fill("Synthetic correction retained in version 2 before acceptance.");
    const revision = retain(await stageUI(p, P, `${stageRoot(id)}/proposals/${proposal.proposal_id}/versions`, "POST",
      () => modify.getByRole("button", { name: "Save new version", exact: true }).click()));
    expect(stageSuccess(revision, "revise_proposal").parent_candidate_id).toBe(proposal.candidate_id);
    const accept = p.getByRole("form", { name: "Modify and accept · step 2 of 2", exact: true });
    await accept.getByRole("checkbox", { name: "I reviewed version 2 as shown here", exact: true }).check();
    const accepted = retain(await stageUI(p, P, `${stageRoot(id)}/proposals/${proposal.proposal_id}/decisions`, "POST",
      () => accept.getByRole("button", { name: "Accept version 2", exact: true }).click()));
    const acceptedIdentity = stageSuccess(accepted, "decide_proposal");
    expect(acceptedIdentity).toMatchObject({ decision: "accepted", version: "2", candidate_id: stageSuccess(revision, "revise_proposal").candidate_id });
    await expect(accept).toHaveCount(0);
    const acceptedEffects = await source.effects(id);
    expect(acceptedEffects).toEqual({ children: 1, mapped_children: 1, commands: 0, runs: 0 });
    const original = retain(await stageRequest(env.apiUrl, P, `${issueRoot(id)}/operations/${accepted.request_id}`));
    stageSuccess(original, "decide_proposal");
    expect(original.response_body).toEqual(accepted.response_body);
    const replay = retain(await stageRequest(env.apiUrl, P, accepted.path, "POST", accepted.request_body, accepted.request_id!));
    stageSuccess(replay, "decide_proposal");
    expect(replay.request_sha256).toBe(accepted.request_sha256);
    expect(replay.response_body).toEqual(accepted.response_body);
    expect(await source.effects(id)).toEqual(acceptedEffects);
    const detail = await readProposal(P, id, proposal.proposal_id);
    expect(detail.versions.map((version) => version.version)).toEqual(["2", "1"]);
    expect(detail.children).toHaveLength(1);
    expect(detail.decisions).toHaveLength(1);
    const rejectSummary = `Synthetic rejected follow-up ${suffix}`;
    const rejectedProposal = await fileProposal(rejectSummary, `Must remain an uncreated proposal ${suffix}`);
    await show(p);
    await stagePanel(p).getByRole("button", { name: `Open proposal: ${rejectSummary}`, exact: true }).click();
    await p.locator(`[data-proposal-id="${rejectedProposal.proposal_id}"]`).getByRole("button", { name: "Reject", exact: true }).click();
    const reject = p.getByRole("form", { name: "Reject version 1", exact: true });
    await reject.getByLabel("Reason", { exact: true }).fill("Synthetic reviewer rejects duplicate scope.");
    const rejected = retain(await stageUI(p, P, `${stageRoot(id)}/proposals/${rejectedProposal.proposal_id}/decisions`, "POST",
      () => reject.getByRole("button", { name: "Reject version 1", exact: true }).click()));
    expect(stageSuccess(rejected, "decide_proposal").decision).toBe("rejected");
    expect(await source.effects(id)).toEqual(acceptedEffects);
    record("A02 exact modified version accepted once; rejected proposal and replay start no work", { proposal_id: proposal.proposal_id,
      created_child_ids: detail.children.map((child) => child.issue_id), rejected_proposal_id: rejectedProposal.proposal_id, effects: acceptedEffects });

    const privateDescription = `SYNTHETIC_PRIVATE_CHILD_DESCRIPTION_${suffix}`;
    const manual = await P.client.requestJSON<{ id: string; title: string }>("/api/issues", { method: "POST", body: {
      title: `Synthetic manual source child ${suffix}`, description: privateDescription, status: "todo", project_id: project.id, parent_issue_id: id,
    } });
    const manualBefore = (await readStage(P, id)).children.items.find((child) => child.issue_id === manual.id);
    expect(manualBefore).toMatchObject({ origin: "none", creator_type: "member", execution_gated: false, can_open: true });
    const pr = await source.seedPullRequest(workspace.id, manual.id, suffix, gitInput.initialHead);
    record("synthetic connector seed and preserved human manual-child control", { pr, manual_id: manual.id });
    await show(p);
    retain(await confirmStageScope(p, P, id, await readStage(P, id), { childId: manual.id, prId: pr.prId, head: pr.head }));
    const createCode = async (head: string, revise: boolean) => {
      await show(p);
      await stagePanel(p).getByRole("button", { name: revise ? "Revise code candidate" : "Create code candidate", exact: true }).click();
      const form = p.getByRole("form", { name: revise ? "Revise code candidate" : "Create code candidate", exact: true });
      if (!revise) await form.getByLabel("Sub-issue", { exact: true }).selectOption(manual.id);
      await form.getByLabel("Pull request and head", { exact: true }).selectOption(JSON.stringify({ provider: "github", pull_request_id: pr.prId, head_sha: head }));
      const fact = retain(await stageUI(p, P, `${stageRoot(id)}/code-candidates`, "POST", () => form.getByRole("button", { name: "Save code candidate", exact: true }).click()));
      await expect(form).toHaveCount(0);
      const value = stageSuccess(fact, "create_code_candidate");
      return { candidate_id: value.candidate_id, version: value.version, content_digest: value.content_digest };
    };
    const code = await createCode(pr.head, false);
    const advanceAPI = async (candidate: CollaborationStageCandidateRef, target: string, decisionIds: string[] = []) => retain(await stageRequest(env.apiUrl, P,
      `${stageRoot(id)}/advance`, "POST", { ...await stageObservations(P, id), candidate, target_stage: target, decision_ids: decisionIds }));
    record("synthetic CI missing", await source.observeCI(pr.prId, "missing"));
    stageRefusal(await advanceAPI(code, "code_review"), 409, "stage_ci_missing");
    record("synthetic CI failed", await source.observeCI(pr.prId, "failed"));
    stageRefusal(await advanceAPI(code, "code_review"), 409, "stage_ci_failed");
    expect((await readStage(P, id)).stage?.current).toBe("development");
    record("synthetic CI passed", await source.observeCI(pr.prId, "passed"));
    const moveUI = async (target: "Code review" | "Development" | "QA" | "Release") => {
      await show(p);
      await stagePanel(p).getByRole("button", { name: `Move to ${target}`, exact: true }).click();
      const form = p.getByRole("form", { name: `Move to ${target}`, exact: true });
      const fact = retain(await stageUI(p, P, `${stageRoot(id)}/advance`, "POST", () => form.getByRole("button", { name: "Confirm stage change", exact: true }).click()));
      stageSuccess(fact, "advance_stage");
      await expect(form).toHaveCount(0);
      return fact;
    };
    await moveUI("Code review");
    stageRefusal(await advanceAPI(code, "qa"), 409, "stage_dependency_invalid");
    const evidenceForReview = await readCode(R, id, code.candidate_id);
    const selfReview = retain(await stageRequest(env.apiUrl, D, `${stageRoot(id)}/decisions`, "POST", {
      ...await stageObservations(D, id), step: "code_review", decision: "approve", candidate: code,
      evidence: { ci_digest: evidenceForReview.observed.ci.ci_digest, test_report_id: null, build: null, deployment_id: null, rollback_target_digest: null },
      depends_on: [], reason: "Synthetic implementer cannot self-approve", expires_at: evidenceForReview.signable[0]!.max_expires_at,
    }));
    stageRefusal(selfReview, 403, "stage_role_required");
    const forgedHuman = retain(await stageRequest(env.apiUrl, R, `${stageRoot(id)}/decisions`, "POST", selfReview.request_body,
      undefined, { "X-Agent-ID": randomUUID() }));
    stageRefusal(forgedHuman, 403, "forbidden");
    record("A03/A04 failed CI and absence of reviewer signature block stage advancement", { candidate: code, self_review_refusal: selfReview.response_body });

    const outsider = retain(await stageRequest(env.apiUrl, X, stageRoot(id)));
    stageRefusal(outsider, 404, "not_found");
    const externalView = await readStage(E, id);
    const hiddenChild = externalView.children.items.find((child) => child.issue_id === manual.id);
    expect(hiddenChild).toMatchObject({ can_open: false, content_digest: null });
    expect(JSON.stringify(externalView)).not.toContain(privateDescription);
    const protectedCode = await readCode(E, id, code.candidate_id);
    expect(protectedCode).toMatchObject({ candidate: { baseline: null }, blocked_evidence: 1, decisions: [], test_reports: [], execution_facts: [],
      observed: { pull_request: { state: "unavailable", head_sha: null }, ci: { state: "unavailable", ci_digest: null, head_sha: null, observed_at: null, check_count: 0 } } });
    expect(protectedCode.signable.every((entry) => !entry.allowed && entry.max_expires_at === null)).toBe(true);
    await show(external);
    await expect(stagePanel(external).getByText("You cannot currently read the source evidence. No source details or approvals are shown.", { exact: true })).toBeVisible();
    await expect(stagePanel(external).getByRole("button", { name: "Approve code review", exact: true })).toHaveCount(0);
    record("A10 source-denied reads hide child digest and R4 evidence; unrelated account is denied", { source_denied: protectedCode, child: hiddenChild, outsider_status: outsider.status });

    const profile = mkdtempSync(join(tmpdir(), "multica-stage-reviewer-"));
    evidence.desktop_profile_sha256 = stageHash(profile);
    native = await openDesktop(profile, { token: R.client.getToken()!, deepLink: `multica://account?view=participating&issue=${id}` });
    await native.start();
    const r = native.page;
    await expect(r.getByRole("heading", { name: title, exact: true })).toBeVisible();
    const openReview = async (decision: "approve" | "return") => {
      await r.reload();
      const label = decision === "approve" ? "Approve code review" : "Return from code review";
      await stagePanel(r).getByRole("button", { name: label, exact: true }).click();
      const form = r.getByRole("form", { name: label, exact: true });
      await form.getByLabel("Reason", { exact: true }).fill(decision === "approve" ? "Synthetic human code review of the exact candidate and CI." : "Synthetic reviewer requests rework; prior evidence remains retained.");
      await form.getByRole("checkbox", { name: "I reviewed this exact candidate and evidence", exact: true }).check();
      return form;
    };
    const review = await openReview("approve");
    const lost = await loseStageReply(r, R, `${stageRoot(id)}/decisions`, () => review.getByRole("button", { name: "Record approval", exact: true }).click(), retain);
    const firstDecision = stageSuccess(lost, "decide_stage");
    expect(firstDecision).toMatchObject({ decision: "approved", candidate_id: code.candidate_id });
    await expect(review.getByText("Unconfirmed stage requests", { exact: true })).toBeVisible();
    const recoveryRequests: { method: string; path: string }[] = [];
    const observeRecovery = (request: import("@playwright/test").Request) => {
      const path = new URL(request.url()).pathname;
      if (path.startsWith(issueRoot(id))) recoveryRequests.push({ method: request.method(), path });
    };
    r.on("request", observeRecovery);
    const recoveryPending = r.waitForResponse((response) => new URL(response.url()).pathname === `${issueRoot(id)}/operations/${lost.request_id}` && response.request().method() === "GET");
    await review.getByRole("button", { name: "Check original result", exact: true }).click();
    const recovered = await recoveryPending;
    expect(recovered.status()).toBe(200);
    expect(await recovered.json()).toEqual(lost.response_body);
    await expect(review.getByText("Unconfirmed stage requests", { exact: true })).toHaveCount(0);
    r.off("request", observeRecovery);
    expect(recoveryRequests.some((request) => request.path.endsWith(`/operations/${lost.request_id}`))).toBe(true);
    expect(recoveryRequests.every((request) => request.method === "GET")).toBe(true);
    await r.getByRole("dialog").getByRole("button", { name: "Close", exact: true }).click();
    const originalBody = lost.request_body;
    if (originalBody === null || typeof originalBody !== "object" || Array.isArray(originalBody)) throw new Error("Captured stage request is not an object");
    const changedBody = { ...originalBody, reason: "Changed original intent must not overwrite a decision." };
    const changed = retain(await stageRequest(env.apiUrl, R, lost.path, "POST", changedBody, lost.request_id!));
    stageRefusal(changed, 409, "idempotency_conflict");
    expect((await readCode(R, id, code.candidate_id)).decisions).toHaveLength(1);
    record("Desktop lost a real successful reply and recovered only by original-key GET", { actual_server_receipt: lost, recovery_requests: recoveryRequests });

    await stagePanel(r).getByRole("button", { name: "Revoke approval", exact: true }).click();
    const revoke = r.getByRole("form", { name: "Revoke approval", exact: true });
    await revoke.getByLabel("Reason", { exact: true }).fill("Synthetic reviewer withdraws the original approval.");
    const revoked = retain(await stageUI(r, R, `${stageRoot(id)}/decisions/${firstDecision.decision_id}/revocations`, "POST",
      () => revoke.getByRole("button", { name: "Confirm revocation", exact: true }).click()));
    expect(stageSuccess(revoked, "revoke_decision").decision_id).toBe(firstDecision.decision_id);
    stageRefusal(await advanceAPI(code, "qa", [firstDecision.decision_id]), 409, "stage_decision_revoked");
    const returnForm = await openReview("return");
    const returned = retain(await stageUI(r, R, `${stageRoot(id)}/decisions`, "POST", () => returnForm.getByRole("button", { name: "Record return", exact: true }).click()));
    expect(stageSuccess(returned, "decide_stage").decision).toBe("returned");
    await moveUI("Development");
    const nextHead = gitInput.head;
    record("synthetic new PR head", await source.observeCI(pr.prId, "passed", nextHead));
    stageRefusal(await advanceAPI(code, "code_review"), 409, "stage_head_changed");
    expect((await readCode(P, id, code.candidate_id)).candidate.state).toBe("stale");
    const nextCode = await createCode(nextHead, true);
    expect(nextCode.version).toBe("2");
    expect(nextCode.candidate_id).not.toBe(code.candidate_id);
    await moveUI("Code review");
    const secondReview = await openReview("approve");
    const secondApproval = retain(await stageUI(r, R, `${stageRoot(id)}/decisions`, "POST", () => secondReview.getByRole("button", { name: "Record approval", exact: true }).click()));
    const secondDecision = stageSuccess(secondApproval, "decide_stage");
    const oldMember = sourceMembers.find((member) => member.actor_id === R.id)!;
    const removed = retain(await stageRequest(env.apiUrl, P, `/api/workspaces/${workspace.id}/members/${oldMember.member_id}`, "DELETE",
      undefined, undefined, { "X-Workspace-ID": workspace.id }));
    expect(removed.status).toBe(204);
    stageRefusal(await advanceAPI(nextCode, "qa", [secondDecision.decision_id]), 409, "stage_signer_changed");
    await r.reload();
    await expect(stagePanel(r).getByText("You cannot currently read the source evidence. No source details or approvals are shown.", { exact: true })).toBeVisible();
    expect((await readCode(R, id, nextCode.candidate_id)).candidate.baseline).toBeNull();
    const rejoined = await joinStageSource(P, R, workspace.id);
    expect(rejoined.member_id).not.toBe(oldMember.member_id);
    stageRefusal(await advanceAPI(nextCode, "qa", [secondDecision.decision_id]), 409, "stage_signer_changed");
    const freshReview = await openReview("approve");
    const freshApproval = retain(await stageUI(r, R, `${stageRoot(id)}/decisions`, "POST", () => freshReview.getByRole("button", { name: "Record approval", exact: true }).click()));
    expect(stageSuccess(freshApproval, "decide_stage").decision_id).not.toBe(secondDecision.decision_id);
    await moveUI("QA");
    const finalStage = await readStage(P, id);
    expect(finalStage.stage?.current).toBe("qa");
    const finalCode = await readCode(P, id, nextCode.candidate_id);
    expect(finalCode.decisions.find((decision) => decision.decision_id === secondDecision.decision_id)?.state).toBe("signer_changed");
    expect(finalCode.decisions.some((decision) => decision.decision_id === stageSuccess(freshApproval, "decide_stage").decision_id && decision.state === "valid")).toBe(true);
    expect((await readCode(P, id, code.candidate_id)).decisions.map((decision) => decision.decision_id)).toEqual(expect.arrayContaining([firstDecision.decision_id, stageSuccess(returned, "decide_stage").decision_id]));
    record("A08 revocation, rework, changed code and removed/rejoined source membership never revive old approvals", {
      original_candidate: code, next_candidate: nextCode, old_member_id: oldMember.member_id, new_member_id: rejoined.member_id,
      retained_decision_ids: finalCode.decisions.map((decision) => decision.decision_id), stage: finalStage.stage,
    });
    const effects = await source.effects(id);
    expect(effects.commands).toBe(0); expect(effects.runs).toBe(0); expect(effects.children).toBe(2);
    record("human stage actions created no execution", effects);

    // A05 starts only after the real code-review journey reached QA. Runtime
    // configuration is registered before consent; no run/report row is seeded.
    const gateBefore = await source.executionGate();
    originalGate = gateBefore;
    record("execution gate before controlled A05 runs", gateBefore);
    restoreGate = await openHostAcceptanceGate();
    await show(p);
    record("explicit creator AI policy change", await changePolicyInUI(p, id, "assisted"));
    const q = await openWeb(Q);
    const executionWorkspace = await Q.client.requestJSON<{ id: string; name: string; slug: string }>("/api/workspaces", {
      method: "POST", body: { name: `Synthetic QA execution ${suffix}`, slug: `stage-qa-execution-${suffix}` },
    });
    Q.client.setWorkspaceId(executionWorkspace.id);
    Q.client.setWorkspaceSlug(executionWorkspace.slug);
    record("QA owner created its independent execution workspace", { owner_id: Q.id, workspace: executionWorkspace });
    const startResource = async (prepared: boolean) => {
      const resource = await createStageTestResource(Q, executionWorkspace, env.apiUrl, env.deploymentId,
        `${prepared ? "Prepared" : "Bare"} synthetic QA ${suffix}`, record, prepared ? gitInput.config : undefined);
      runtimes.push(resource);
      const offered = await offerClosureResource(resource, P, await readClosureIssue(P, id));
      await show(q);
      await openHostConsent(q, offered.hostIssue);
      const consent = retain(await stageUI(q, Q, `${issueRoot(id)}/targets/${offered.hostIssue.targetId}/consents`, "POST",
        () => hostPanel(q).getByRole("button", { name: "Authorize for this issue", exact: true }).click()));
      expect(consent.status).toBe(201);
      await show(p);
      const current = await readClosureIssue(P, id);
      if (current.session.queue_state === "paused") record("explicit issue resume", await hostControl(p, offered.hostIssue, "resume", "issue"));
      const target = (await readClosureIssue(P, id)).execution?.targets.find((item) => item.id === offered.hostIssue.targetId);
      if (target?.control.queue_state === "paused") record("explicit target resume", await hostControl(p, offered.hostIssue, "resume", "target"));
      record("controlled resource consented", { prepared, runtime_id: resource.runtime.runtimeId, agent_id: resource.agent.id,
        target_id: offered.hostIssue.targetId, ready: resource.daemon.ready });
      return { resource, issue: offered.hostIssue };
    };
    const settle = async (visit: HostVisit, outcome: "success" | "failure") => {
      await expect.poll(async () => (await readClosureIssue(P, id)).execution?.runs.find((run) => run.id === visit.identity.run_id), { timeout: 30_000 })
        .toMatchObject({ progress_state: "settled", proof_state: "host_exit_observed", business_outcome: outcome, execution_mode: "host_process_supervision_v1" });
      const run = (await readClosureIssue(P, id)).execution!.runs.find((item) => item.id === visit.identity.run_id)!;
      record("actual controlled run settled", run);
      return run;
    };
    const bare = await startResource(false);
    await bare.resource.beforeTrigger();
    await show(p);
    const bareComment = await sendHostComment(p, bare.issue, "Synthetic bare-host subject absence control.", P.id);
    record("bare host ordinary typed comment", bareComment);
    const bareVisit = await bare.resource.daemon.waitForVisit(1);
    const bareSubject = (await bare.resource.daemon.act(bareVisit, [{ kind: "test_subject" }]))[0]?.result;
    expect(bareSubject).toMatchObject({ available: false, code: "test_subject_unavailable" });
    record("bare host cannot supply a typed test subject", bareSubject);
    bare.resource.daemon.release(bareVisit);
    await settle(bareVisit, "success");
    const bareReports = CollaborationTestReportsSchema.parse(await P.client.requestJSON<unknown>(`${issueRoot(id)}/runs/${bareVisit.identity.run_id}/test-reports`));
    expect(bareReports.items).toEqual([]);
    await bare.resource.close();
    await show(q);
    await expect(stagePanel(q).getByRole("button", { name: "Approve QA", exact: true })).toHaveCount(0);
    record("successful bare execution is not QA evidence", { run_id: bareVisit.identity.run_id, reports: bareReports });

    const prepared = await startResource(true);
    const testReference = { candidate_id: nextCode.candidate_id, candidate_version: nextCode.version, candidate_digest: nextCode.content_digest };
    let preparedWorkspace: { workdirHash: string; sessionKey: string } | undefined;
    const submitPrepared = async (fault: "none" | "duplicate_effect", visitCount: number) => {
      await prepared.resource.beforeTrigger();
      await show(p);
      const before: string[] = [];
      const notice = (request: import("@playwright/test").Request) => { if (request.method() === "POST" && new URL(request.url()).pathname === `${issueRoot(id)}/comments`) before.push(request.url()); };
      p.on("request", notice);
      await stagePanel(p).getByRole("button", { name: "Prepare test input", exact: true }).click();
      await expect(p.getByRole("region", { name: "Test input", exact: true })).toContainText(nextCode.candidate_id);
      p.off("request", notice);
      expect(before).toEqual([]);
      let command: Awaited<ReturnType<typeof sendHostComment>> | undefined;
      const comment = retain(await stageUI(p, P, `${issueRoot(id)}/comments`, "POST", async () => {
        command = await sendHostComment(p, prepared.issue, `Synthetic prepared test ${visitCount}; injected fault ${fault}.`, P.id);
      }));
      expect(comment.status).toBe(201);
      expect(comment.request_body).toMatchObject({ test_subject: testReference, execution_view_version: 6 });
      if (!command) throw new Error("The C1 comment did not produce its actual command receipt");
      const visit = await prepared.resource.daemon.waitForVisit(visitCount);
      expect(visit.inputs.map((input) => input.command_id)).toContain(command.commandId);
      expect(visit.workdir_files["stage-regression.json"]).toBe(gitInput.marker_sha256);
      expect(visit.workdir_files).not.toHaveProperty("untracked-private.txt");
      const requestId = randomUUID();
      const receipt = (await prepared.resource.daemon.act(visit, [{ kind: "test_report", request_id: requestId, fault }]))[0];
      const result = receipt?.result;
      requireObject(result); requireObject(result.submit);
      expect(result.submit).toEqual(result.receipt);
      expect(result.submit).toMatchObject({ version: 1, issue_id: id, run_id: visit.identity.run_id, request_id: requestId, accepted: true });
      expect(result.subject).toMatchObject({ version: 1, issue_id: id, run_id: visit.identity.run_id,
        subject: { ...testReference, source_revision: gitInput.head, artifact_digest: null } });
      expect(result.source_file_sha256).toBe(gitInput.marker_sha256);
      const reportId = result.submit.report_id;
      if (typeof reportId !== "string") throw new Error("The actual report ACK omitted its server report identity");
      const path = `${issueRoot(id)}/runs/${visit.identity.run_id}/test-reports/${reportId}`;
      const read = async () => CollaborationTestReportSchema.parse(await P.client.requestJSON<unknown>(path));
      const unsettled = await read();
      expect(unsettled).toMatchObject({ report_id: reportId, agent_id: prepared.resource.agent.id, source_kind: "runtime_reported",
        reported_result: fault === "none" ? "passed" : "failed", applicability: "execution_unsettled", subject: { ...testReference, source_revision: gitInput.head, artifact_digest: null } });
      expect(await verifyCollaborationTestReportLogs(unsettled)).toBe(true);
      const ledger = prepared.resource.daemon.ledgerEvidence().find((item) => item.run_id === visit.identity.run_id);
      const providerLaunch = ledger?.launches.filter((launch: { kind: string; authorized: boolean }) => launch.kind === "provider" && launch.authorized);
      expect(providerLaunch).toHaveLength(1);
      expect(providerLaunch?.[0]?.attempt_seq).toBe(unsettled.attempt);
      if (!ledger) throw new Error("The prepared provider run has no owned launch ledger");
      const preparationLaunches = ledger.launches.filter((launch: { kind: string }) => launch.kind === "preparation");
      if (visitCount === 1) {
        expect(preparationLaunches.length).toBeGreaterThan(0);
        expect(visit.workdir_hash).not.toBe(bareVisit.workdir_hash);
        preparedWorkspace = { workdirHash: visit.workdir_hash, sessionKey: ledger.session_key };
      } else {
        // The same configuration retains its committed Git workspace. A new
        // business run needs one provider launch, not another extraction.
        expect(preparedWorkspace).toBeDefined();
        expect(preparationLaunches).toHaveLength(0);
        expect(ledger.launches).toHaveLength(1);
        expect(visit.workdir_hash).toBe(preparedWorkspace!.workdirHash);
        expect(ledger.session_key).toBe(preparedWorkspace!.sessionKey);
        expect(unsettled.attempt).toBe("1");
      }
      record("actual CLI T1/T2/T3 before provider completion", { comment, command, visit: { run_id: visit.identity.run_id, inputs: visit.inputs,
        cli_sha256: visit.cli_hash, workdir_hash: visit.workdir_hash, prepared_files: visit.workdir_files }, result, report: unsettled, ledger });
      prepared.resource.daemon.release(visit, fault === "none" ? "success" : "failure");
      const run = await settle(visit, fault === "none" ? "success" : "failure");
      const report = await read();
      expect(report.applicability).toBe(fault === "none" ? "usable" : "execution_failed");
      expect(report.report_digest).toBe(unsettled.report_digest);
      expect(report.cases).toEqual(unsettled.cases); expect(report.logs).toEqual(unsettled.logs);
      const records = await P.client.requestJSON<CollaborationRunRecords>(versioned(`${issueRoot(id)}/runs/${visit.identity.run_id}/records?limit=100`));
      expect(records.preparation?.git).toMatchObject({ object_format: "sha1", commit_oid: gitInput.head });
      if (run.launch_count === undefined) throw new Error("Prepared run omitted its actual launch inventory");
      expect(BigInt(run.launch_count)).toBe(BigInt(ledger.launches.length));
      if (visitCount === 1) expect(BigInt(run.launch_count)).toBeGreaterThan(1n);
      else expect(BigInt(run.launch_count)).toBe(1n);
      record("prepared execution and retained report are distinct facts", { run, report, preparation: records.preparation });
      return { report, read, path, visit, command };
    };
    const failedTest = await submitPrepared("duplicate_effect", 1);
    const reportUI = async (page: Page, runId: string, reportId: string) => {
      await stagePanel(page).getByRole("button", { name: `Browse reports for run ${runId}`, exact: true }).click();
      const history = page.getByRole("region", { name: "Reports for this run", exact: true });
      await expect(history).toContainText(reportId);
      await history.getByRole("button", { name: "Open report", exact: true }).click();
      const detail = history.getByRole("region", { name: "Test report detail", exact: true });
      await expect(detail).toContainText(reportId);
      await expect(detail).toContainText(gitInput.head);
      return detail;
    };
    await show(q);
    const failedDetail = await reportUI(q, failedTest.report.run_id, failedTest.report.report_id);
    await expect(failedDetail).toContainText("duplicate-callback · Failed");
    await expect(failedDetail).toContainText("injected_fault=duplicate_effect");
    await expect(failedDetail).toContainText("Execution failed");
    await expect(failedDetail).toContainText("No build is bound to this candidate.");
    const deniedReport = retain(await stageRequest(env.apiUrl, E, failedTest.path));
    stageRefusal(deniedReport, 404, "not_found");
    expect(JSON.stringify(deniedReport.response_body)).not.toContain("duplicate-callback");
    const approveFailed = retain(await stageRequest(env.apiUrl, Q, `${stageRoot(id)}/decisions`, "POST", { ...await stageObservations(Q, id),
      step: "qa", decision: "approve", candidate: nextCode, evidence: { ci_digest: null, test_report_id: failedTest.report.report_id, build: null, deployment_id: null, rollback_target_digest: null },
      depends_on: [stageSuccess(freshApproval, "decide_stage").decision_id], reason: "Synthetic negative control: failed test must not approve QA", expires_at: new Date(Date.now() + 60_000).toISOString() }));
    stageRefusal(approveFailed, 409, "stage_test_report_unusable");
    await stagePanel(q).getByRole("button", { name: "Return from QA", exact: true }).click();
    const qaReturn = q.getByRole("form", { name: "Return from QA", exact: true });
    await qaReturn.getByLabel("Test report", { exact: true }).selectOption(failedTest.report.report_id);
    await qaReturn.getByLabel("Reason", { exact: true }).fill("Synthetic QA return: keep the failed run, duplicate-callback case and original logs.");
    await qaReturn.getByRole("checkbox", { name: "I reviewed this exact candidate and evidence", exact: true }).check();
    const qaReturned = retain(await stageUI(q, Q, `${stageRoot(id)}/decisions`, "POST", () => qaReturn.getByRole("button", { name: "Record return", exact: true }).click()));
    expect(stageSuccess(qaReturned, "decide_stage")).toMatchObject({ step: "qa", decision: "returned", candidate_id: nextCode.candidate_id });
    await moveUI("Development");
    await moveUI("Code review");
    const reviewRetest = await openReview("approve");
    const retestReview = retain(await stageUI(r, R, `${stageRoot(id)}/decisions`, "POST", () => reviewRetest.getByRole("button", { name: "Record approval", exact: true }).click()));
    await moveUI("QA");
    await show(p);
    const retryTarget = (await readClosureIssue(P, id)).execution?.targets.find((target) => target.id === prepared.issue.targetId);
    if (retryTarget?.control.queue_state === "paused") record("explicit retry target resume", await hostControl(p, prepared.issue, "resume", "target"));
    const passedTest = await submitPrepared("none", 2);
    expect(passedTest.report.run_id).not.toBe(failedTest.report.run_id);
    expect(passedTest.report.report_id).not.toBe(failedTest.report.report_id);
    expect(passedTest.command.requestId).not.toBe(failedTest.command.requestId);
    expect(passedTest.report.subject.binding_digest).not.toBe(failedTest.report.subject.binding_digest);
    const immutable = (report: typeof failedTest.report) => stageHash(JSON.stringify({ report_id: report.report_id, report_digest: report.report_digest,
      received_at: report.received_at, subject: report.subject, cases: report.cases, logs: report.logs }));
    expect(immutable(await failedTest.read())).toBe(immutable(failedTest.report));
    const beforeQAApproval = await readCode(P, id, nextCode.candidate_id);
    expect(beforeQAApproval.decisions.find((decision) => decision.decision_id === stageSuccess(qaReturned, "decide_stage").decision_id))
      .toMatchObject({ actor_user_id: Q.id, step: "qa", decision: "returned", reason: "Synthetic QA return: keep the failed run, duplicate-callback case and original logs." });
    expect(beforeQAApproval.decisions.filter((decision) => decision.step === "qa" && decision.decision === "approved")).toEqual([]);
    expect((await readStage(P, id)).stage?.current).toBe("qa");
    await r.reload();
    await expect(await reportUI(r, passedTest.report.run_id, passedTest.report.report_id)).toContainText("duplicate-callback · Passed");
    await show(q);
    await stagePanel(q).getByRole("button", { name: "Approve QA", exact: true }).click();
    const qaApprove = q.getByRole("form", { name: "Approve QA", exact: true });
    await qaApprove.getByLabel("Test report", { exact: true }).selectOption(passedTest.report.report_id);
    await qaApprove.getByLabel("Preceding approval", { exact: true }).selectOption(stageSuccess(retestReview, "decide_stage").decision_id);
    await expect(qaApprove.getByRole("region", { name: "Test report detail", exact: true })).toContainText(passedTest.report.report_id);
    await qaApprove.getByLabel("Reason", { exact: true }).fill("Synthetic human QA approval of this exact prepared-source report; no build or release approved.");
    await qaApprove.getByRole("checkbox", { name: "I reviewed this exact candidate and evidence", exact: true }).check();
    const qaApproved = retain(await stageUI(q, Q, `${stageRoot(id)}/decisions`, "POST", () => qaApprove.getByRole("button", { name: "Record approval", exact: true }).click()));
    expect(stageSuccess(qaApproved, "decide_stage")).toMatchObject({ step: "qa", decision: "approved", candidate_id: nextCode.candidate_id });
    await moveUI("Release");
    expect((await readStage(P, id)).stage?.current).toBe("release");
    expect((await readCode(P, id, nextCode.candidate_id)).execution_facts).toEqual([]);
    record("A05 retained failure, explicit new prepared run and human QA decision", { failed: failedTest.report, passed: passedTest.report,
      qa_return: qaReturned.response_body, qa_approval: qaApproved.response_body, source_only_artifact: null, release_execution_recorded: false });
    await prepared.resource.close();
    const afterTests = await source.effects(id);
    expect(afterTests).toMatchObject({ children: 2, commands: 3, runs: 3 });
    expect(native.accountHeaders.length).toBeGreaterThan(0);
    expect(native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
    expect(await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
    expect(native.errors).toEqual([]); expect(errors).toEqual([]);
    await p.screenshot({ path: info.outputPath("stage-web-release.png"), fullPage: true });
    await nativeScreenshot(native.app, r, info.outputPath("stage-desktop-review.png"));
    record("shared Web/Desktop subset completed with three controlled runs", { effects: afterTests, desktop_account_requests: native.accountHeaders.length, installed_main: false });
    evidence.outcome = "passed_subset";
  } finally {
    const cleanupErrors: string[] = [];
    for (const runtime of runtimes.toReversed()) await runtime.close().catch(() => cleanupErrors.push("Controlled host cleanup failed"));
    if (restoreGate) {
      await restoreGate().catch(() => cleanupErrors.push("Execution gate restore failed"));
      try {
        const after = await source.executionGate();
        record("execution gate after controlled cleanup", after);
        if (!originalGate) cleanupErrors.push("Original gate evidence is missing");
        else {
          expect(after).toMatchObject({ enabled: originalGate.enabled, creation_enabled: originalGate.creation_enabled, host_execution_enabled: originalGate.host_execution_enabled });
          expect(BigInt(after.epoch)).toBeGreaterThanOrEqual(BigInt(originalGate.epoch));
        }
      } catch { cleanupErrors.push("Execution gate did not match its recorded original state"); }
    }
    if (native) await native.app.close().catch(() => cleanupErrors.push("Electron fixture close failed"));
    for (const context of contexts) await context.close().catch(() => cleanupErrors.push("Web context close failed"));
    await source.close().catch(() => cleanupErrors.push("Owned SQL connection close failed"));
    evidence.cleanup = { errors: cleanupErrors, database_retained: true, synthetic_history_retained: true };
    evidence.page_errors = errors;
    if (evidence.outcome === "running") evidence.outcome = "failed_before_subset_completed";
    persist();
    await info.attach("stage-approvals-evidence", { path: info.outputPath("stage-approvals-evidence.json"), contentType: "application/json" });
    expect(cleanupErrors).toEqual([]);
  }
});
