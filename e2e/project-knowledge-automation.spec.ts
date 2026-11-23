import { expect, test } from "@playwright/test";
import { writeFileSync } from "node:fs";
import { CollaborationIssueSchema } from "@multica/core/collaboration";
import { revealCollaborationDisclosure } from "./fixtures/collaboration-disclosures";
import { nativeScreenshot } from "./fixtures/collaboration-native";
import { journeyCopy, nativeJourneyEvidence, openJourneyAction, refreshJourneyIssue, submitJourneyAction,
  type DetailJourneySession } from "./fixtures/collaboration-detail-journey";
import { captureAutomation, executeConsumer, knowledgeCandidate, knowledgeJobs, modelCalls, modelControl,
  openAutomationSession, ownedQuery, providerCaptures, refuseOldContinuation, seedAutomationJourney, startProductionDaemon,
  type AutomationFixture } from "./fixtures/project-knowledge-automation";

test.use({ actionTimeout: 15_000, trace: "off", screenshot: "off" });

async function submitAndAccept(fixture: AutomationFixture, handler: DetailJourneySession, creator: DetailJourneySession,
  issueId: string, result = fixture.result) {
  await handler.openAccount(issueId);
  const form = await openJourneyAction(handler, issueId, "submit_acceptance");
  const copy = journeyCopy("en");
  await form.form.getByLabel(copy.result, { exact: true }).fill(result);
  await form.form.getByLabel(copy.basis, { exact: true }).fill(fixture.basis);
  const submitted = await submitJourneyAction(form);
  expect(submitted.receipt.code).toBe("acceptance_submitted");
  await creator.openAccount(issueId);
  const acceptance = await submitJourneyAction(await openJourneyAction(creator, issueId, "complete"));
  expect(acceptance.receipt.code).toBe("completed");
  const current = await refreshJourneyIssue(creator, issueId);
  expect(current.status_category).toBe("done");
  expect(current.delivery?.latest?.result).toBe(result);
  expect(current.delivery?.latest?.decision?.kind).toBe("accepted");
  return { submitted, acceptance, current };
}

for (const platform of ["web", "desktop"] as const) {
  test(`${platform}: acceptance, automatic project knowledge, actual daemon input, reopen and fresh reacceptance proof`, async ({ browser }, info) => {
    test.setTimeout(480_000);
    test.skip(!process.env.P6_RUN_DIRECTORY, "Requires the active task's isolated production runtime");
    const fixture = await seedAutomationJourney(platform);
    const sessions: DetailJourneySession[] = [];
    let daemon: Awaited<ReturnType<typeof startProductionDaemon>> | undefined;
    const evidence: Record<string, unknown> = { platform, marker: fixture.marker, issue_id: fixture.shared.issue.id,
      project_id: fixture.project.id, workspace_id: fixture.workspace.id, policy: fixture.policy, runtime: fixture.provenance,
      authentication: "Local password accounts registered and authenticated by real API; Web HttpOnly/CSRF cookies, native password JWT and daemon production managed installation handoff" };
    const captures: Awaited<ReturnType<typeof captureAutomation>>[] = [];
    let failed = true;
    try {
      const creator = await openAutomationSession(browser, fixture, platform, fixture.creator);
      sessions.push(creator);
      const handler = await openAutomationSession(browser, fixture, platform, fixture.handler);
      sessions.push(handler);
      await modelControl("hold", fixture.marker);
      evidence.first_acceptance = await test.step("UI acceptance commits while the controlled model is held", async () => {
        const result = await submitAndAccept(fixture, handler, creator, fixture.shared.issue.id);
        await expect.poll(async () => (await modelCalls()).filter((call) => call.marker === fixture.marker).length).toBe(1);
        const region = creator.page.getByRole("region", { name: "Project knowledge", exact: true });
        await expect(region).toContainText("Organizing knowledge", { timeout: 15_000 });
        expect((await knowledgeJobs(fixture)).items[0]).toMatchObject({ phase: "generating", counts: { saved: 0 } });
        captures.push(await captureAutomation(creator, info, "accepted-processing"));
        return result;
      });
      await modelControl("release", fixture.marker);
      const firstJob = await test.step("The real worker automatically adopts the supported delivery", async () => {
        await expect.poll(async () => (await knowledgeJobs(fixture)).items[0]?.phase, { timeout: 90_000 }).toBe("completed");
        const job = (await knowledgeJobs(fixture)).items[0]!;
        expect(job.counts).toMatchObject({ saved: 1, pending: 0 });
        expect(job.attempts).toHaveLength(2);
        expect(job.attempts.every((attempt) => attempt.state === "succeeded" && attempt.model === "p6-local-controlled-v1" && attempt.usage.status === "known")).toBe(true);
        await refreshJourneyIssue(creator, fixture.shared.issue.id);
        await expect(creator.page.getByRole("region", { name: "Project knowledge", exact: true })).toContainText("1 saved");
        await revealCollaborationDisclosure(creator.page, "Knowledge candidates");
        const candidate = await knowledgeCandidate(fixture);
        expect(candidate).toMatchObject({ state: "approved", proposal_origin: "automation", decision_origin: "automation",
          proposed_by_user_id: null, approved_by_user_id: null, usage_status: "active", content: fixture.result,
          automation: { job_id: job.id }, evidence: [], sources: [{ kind: "delivery", issue_id: fixture.shared.issue.id, submission_id: job.submission_id }] });
        evidence.candidate = candidate;
        if (candidate.availability !== "available" || !("automation" in candidate) || !candidate.automation) throw new Error("Expected available automatic knowledge");
        const candidates = creator.page.getByRole("region", { name: "Knowledge candidates", exact: true });
        await candidates.getByRole("button").filter({ hasText: candidate.summary }).click();
        const review = candidates.getByRole("region", { name: "Review candidate", exact: true });
        await expect(review).toContainText("Server AI");
        await expect(review).toContainText("System rule");
        await expect(review.getByText("Rule version", { exact: true })).toBeVisible();
        await expect(review.getByText(candidate.automation.rule_version, { exact: true })).toBeVisible();
        await expect(review).toContainText(fixture.result);
        await expect(review.getByText("Verified by proposer", { exact: true })).toHaveCount(0);
        const source = review.getByRole("link", { name: `Delivery ${job.submission_id.slice(0, 8)} · acceptance ${job.accepted_event_seq}`, exact: true });
        await expect(source).toBeVisible();
        expect(await source.getAttribute("href")).toContain(job.submission_id);
        expect(await source.getAttribute("href")).toContain(fixture.shared.issue.id);
        captures.push(await captureAutomation(creator, info, "automatically-saved", "Review candidate"));
        const drawer = creator.page.getByRole("dialog", { name: "Knowledge candidates", exact: true });
        await drawer.getByRole("button", { name: "Close resources", exact: true }).click();
        await expect(drawer).toBeHidden();
        return job;
      });
      daemon = await startProductionDaemon(fixture);
      evidence.daemon = { pid: daemon.pid, command: daemon.command, agent_id: daemon.agentId, runtime_id: daemon.runtimeId,
        daemon_id: daemon.daemonID, managed_identity: daemon.managedIdentity, production_binary: process.env.P6_BINARY, fake_provider: process.env.MULTICA_CLAUDE_PATH };
      const first = await test.step("Production daemon receives the exact approved body and bound proof", async () => {
        const execution = await executeConsumer(fixture, daemon!.agentId, "first");
        const candidate = await knowledgeCandidate(fixture);
        expect(execution.entry.candidate_id).toBe(candidate.candidate_id);
        expect(execution.entry.version).toBe(candidate.version);
        expect(execution.entry.validity).toMatchObject({ accepted_event_seq: firstJob.accepted_event_seq, submission_id: firstJob.submission_id });
        evidence.first_execution = execution;
        return execution;
      });
      await test.step("UI reopen immediately pauses the automatic knowledge and refuses the old session", async () => {
        const result = await submitJourneyAction(await openJourneyAction(creator, fixture.shared.issue.id, "reopen"));
        expect(result.receipt.code).toBe("reopened");
        const candidate = await knowledgeCandidate(fixture);
        expect(candidate).toMatchObject({ candidate_id: first.entry.candidate_id, state: "approved", usage_status: "paused_review" });
        await refreshJourneyIssue(creator, fixture.shared.issue.id);
        await expect(creator.page.getByRole("region", { name: "Project knowledge", exact: true })).toContainText("paused");
        await revealCollaborationDisclosure(creator.page, "Knowledge candidates");
        await expect(creator.page.getByRole("region", { name: "Knowledge candidates", exact: true })).toContainText("Paused for review");
        evidence.reopen = { result, candidate, refused: await refuseOldContinuation(fixture, first.issueId) };
        captures.push(await captureAutomation(creator, info, "reopened-paused", "Knowledge candidates"));
        const drawer = creator.page.getByRole("dialog", { name: "Knowledge candidates", exact: true });
        await drawer.getByRole("button", { name: "Close resources", exact: true }).click();
        await expect(drawer).toBeHidden();
      });
      const renewed = await test.step("UI reacceptance fully reviews and restores the same candidate with a new proof", async () => {
        const current = CollaborationIssueSchema.parse(await fixture.creator.client.requestJSON(`/api/collaboration/issues/${fixture.shared.issue.id}`));
        if (current.handler.user_id === fixture.creator.id) {
          const form = await openJourneyAction(creator, current.id, "complete", { self: true });
          await form.form.getByLabel(journeyCopy("en").result, { exact: true }).fill(fixture.result);
          await form.form.getByLabel(journeyCopy("en").basis, { exact: true }).fill(fixture.basis);
          evidence.reacceptance = await submitJourneyAction(form);
        } else evidence.reacceptance = await submitAndAccept(fixture, handler, creator, current.id);
        await expect.poll(async () => (await knowledgeJobs(fixture)).items.find((job) => job.id !== firstJob.id)?.phase,
          { timeout: 90_000 }).toBe("completed");
        const job = (await knowledgeJobs(fixture)).items.find((item) => item.id !== firstJob.id)!;
        expect(job.counts).toMatchObject({ restored: 1, saved: 0, pending: 0 });
        expect(job.submission_id).not.toBe(firstJob.submission_id);
        const candidate = await knowledgeCandidate(fixture);
        expect(candidate).toMatchObject({ candidate_id: first.entry.candidate_id, version: first.entry.version, state: "approved", usage_status: "active" });
        await refreshJourneyIssue(creator, current.id);
        await expect(creator.page.getByRole("region", { name: "Project knowledge", exact: true })).toContainText("1 restored");
        captures.push(await captureAutomation(creator, info, "reaccepted-restored"));
        evidence.restored_job = job;
        return job;
      });
      await test.step("Restoration does not revive old input; a new execution binds the current proof", async () => {
        evidence.restored_old_session_refusal = await refuseOldContinuation(fixture, first.issueId);
        const fresh = await executeConsumer(fixture, daemon!.agentId, "fresh");
        expect(fresh.entry.candidate_id).toBe(first.entry.candidate_id);
        expect(fresh.entry.version).toBe(first.entry.version);
        expect(BigInt(fresh.entry.validity.revision)).toBeGreaterThan(BigInt(first.entry.validity.revision));
        expect(fresh.entry.validity).toMatchObject({ accepted_event_seq: renewed.accepted_event_seq, submission_id: renewed.submission_id });
        expect(fresh.capture.session_id).not.toBe(first.capture.session_id);
        evidence.fresh_execution = fresh;
      });
      await test.step("Accepted no-project and manual tasks create zero automatic work or model calls", async () => {
        const callsBefore = (await modelCalls()).length;
        const executionsBefore = providerCaptures().length;
        const negativeIssues = [fixture.noProject.issue.id, fixture.manual.issue.id];
        const workspaceSnapshot = async () => ({
          base: await ownedQuery("SELECT context FROM workspace WHERE id=$1", [fixture.workspace.id]),
          target: await ownedQuery("SELECT target_id,knowledge_revision FROM collaboration_knowledge_target WHERE target_kind='workspace' AND workspace_id=$1 ORDER BY target_id", [fixture.workspace.id]),
          automatic: await ownedQuery("SELECT id,version,state,state_revision,published_target_revision FROM collaboration_knowledge_candidate WHERE target_kind='workspace' AND target_workspace_id=$1 AND proposal_origin='automation' ORDER BY id", [fixture.workspace.id]),
        });
        const workspaceBefore = await workspaceSnapshot();
        expect(workspaceBefore.base).toHaveLength(1);
        evidence.no_project = await submitAndAccept(fixture, handler, creator, fixture.noProject.issue.id, `No-project result ${fixture.marker}`);
        evidence.manual = await submitAndAccept(fixture, handler, creator, fixture.manual.issue.id, `Manual-policy result ${fixture.marker}`);
        expect((await knowledgeJobs(fixture, fixture.noProject.issue.id)).items).toEqual([]);
        expect((await knowledgeJobs(fixture, fixture.manual.issue.id)).items).toEqual([]);
        const rows = await ownedQuery("SELECT count(*)::int AS count FROM collaboration_knowledge_job WHERE source_issue_id=ANY($1::uuid[])",
          [[fixture.noProject.issue.id, fixture.manual.issue.id]]);
        expect(rows[0]!.count).toBe(0);
        const candidateRows = await ownedQuery("SELECT count(*)::int AS count FROM collaboration_knowledge_candidate WHERE source_issue_id=ANY($1::uuid[]) AND proposal_origin='automation'", [negativeIssues]);
        expect(candidateRows[0]!.count).toBe(0);
        const workspaceAfter = await workspaceSnapshot();
        expect(workspaceAfter).toEqual(workspaceBefore);
        evidence.zero_automatic_candidates_and_workspace_writes = { candidates: candidateRows[0]!.count, workspaceBefore, workspaceAfter };
        expect((await modelCalls()).length).toBe(callsBefore);
        expect(providerCaptures()).toHaveLength(executionsBefore);
        evidence.zero_automatic_calls = { callsBefore, callsAfter: (await modelCalls()).length, jobs: rows[0]!.count };
      });
      evidence.model_calls = (await modelCalls()).filter((call) => call.marker === fixture.marker);
      evidence.native = await nativeJourneyEvidence(creator);
      evidence.native_handler = await nativeJourneyEvidence(handler);
      for (const session of sessions) expect(session.errors).toEqual([]);
      failed = false;
    } finally {
      await modelControl("release", fixture.marker);
      evidence.failed = failed;
      evidence.captures = captures;
      if (failed) for (const [index, session] of sessions.entries()) {
        if (session.page.isClosed()) continue;
        writeFileSync(info.outputPath(`failure-${index}.aria.txt`), await session.page.locator("body").ariaSnapshot());
        const path = info.outputPath(`failure-${index}.png`);
        if (session.native) await nativeScreenshot(session.native.app, session.page, path);
        else await session.page.screenshot({ path });
      }
      writeFileSync(info.outputPath("journey-evidence.json"), JSON.stringify(evidence, null, 2), { mode: 0o600 });
      await daemon?.stop();
      for (const session of sessions) await session.close();
    }
  });
}
