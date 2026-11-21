import { openNewCollaborationIssue } from "./fixtures/sidebar-navigation";
import { randomUUID } from "node:crypto";
import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import type { CollaborationCommentReceipt, CollaborationDeliveryHistory, CollaborationIssue, CollaborationMentionTargets } from "@multica/core/collaboration";
import { hostControl, hostEnvironment, hostAuthority, hostMode, hostRuntimeConfigured, issueRoot, openHostAcceptanceGate, sendHostComment, versioned,
  type HostIssue, type HostVisit } from "./fixtures/collaboration-host-runtime";
import { decideClosureResult, simpleWorkflow } from "./fixtures/collaboration-multi-account-closure";
import { directedFormThroughUI } from "./fixtures/collaboration-handoff-chain";
import { selectCollaborationRecipient } from "./fixtures/collaboration-offline-handoff";
import { commentThroughUI, gapControlBody, gapWorkflowObservations, lifecycleThroughUI } from "./fixtures/collaboration-control-gaps";
import { sendSquadComment } from "./fixtures/collaboration-host-squad";
import { saveMemoryInUI } from "./fixtures/collaboration-memory";
import { approveKnowledge, chooseCandidate, fillKnowledge, observeKnowledgeHints, sourcePanel, submitKnowledge } from "./fixtures/collaboration-knowledge";
import { revealCollaborationDisclosure } from "./fixtures/collaboration-disclosures";
import {
  bizActor, bizAgent, bizRuntimeOwner, bizSquad, changePolicyInUI, closeOwned, consentInUI, controlTargetInUI, Evidence, notifications, offerAndAdopt, openActor, rawCall,
  readIssue, removeParticipantInUI, sha256, showIssue, uploadDraft, waitRun, withdrawInUI, workbench,
  type BizActor, type BizRuntimeOwner, type BizTarget,
} from "./fixtures/collaboration-business-scenarios";

// Three business closures across separate accounts and workspaces, each on real
// host pollers with the deterministic host Provider. Agent success and business
// completion are asserted separately; queued inputs never count as execution.

test.use({ actionTimeout: 20_000, viewport: { width: 1440, height: 1100 }, screenshot: "only-on-failure", trace: "off" });

const hostIssue = (issue: { id: string; title: string }, target: BizTarget, agent: { id: string; name: string }): HostIssue => ({
  id: issue.id, title: issue.title, description: "", targetId: target.targetId, channelId: target.channelId, agentId: agent.id, name: agent.name, attachment: null });

async function createInUI(page: Page, baseURL: string, input: { workspaceId: string; title: string; description: string; policy: "manual" | "assisted";
  recipient: BizActor; file?: { name: string; body: string } }) {
  await page.goto(new URL("/account", baseURL).href);
  await openNewCollaborationIssue(page);
  const creation = page.getByRole("dialog", { name: "New collaboration issue", exact: true });
  await creation.getByRole("combobox", { name: "Source workspace", exact: true }).selectOption(input.workspaceId);
  await creation.getByLabel("Title", { exact: true }).fill(input.title);
  await creation.getByLabel("Description", { exact: true }).fill(input.description);
  await creation.getByLabel("AI participation", { exact: true }).selectOption(input.policy);
  await selectCollaborationRecipient(page, creation, input.recipient);
  if (input.file) {
    await creation.getByLabel("Attach files", { exact: true }).setInputFiles({ name: input.file.name, mimeType: "text/plain", buffer: Buffer.from(input.file.body) });
    await expect(creation.getByRole("checkbox", { name: `Include ${input.file.name}`, exact: true })).toBeChecked();
  }
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/collaboration/issues" && response.request().method() === "POST");
  await creation.getByRole("button", { name: "Create collaboration issue", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(201);
  const created: CollaborationIssue = await response.json();
  return { id: created.id, title: input.title };
}

/** A human comment that shares one file, sent through the real composer. */
async function commentWithFileInUI(page: Page, issueId: string, text: string, file: { name: string; body: string }) {
  const composer = page.getByRole("form", { name: "Comments", exact: true });
  await composer.getByLabel("Add comment", { exact: true }).fill(text);
  const uploadPending = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/collaboration/attachments/drafts" && response.request().method() === "POST");
  await composer.getByLabel("Attach files", { exact: true }).setInputFiles({ name: file.name, mimeType: "text/plain", buffer: Buffer.from(file.body) });
  const upload = await uploadPending;
  expect(upload.status()).toBe(201);
  const uploaded: { id: string } = await upload.json();
  await expect(composer.getByRole("checkbox", { name: `Include ${file.name}`, exact: true })).toBeChecked();
  const preview = composer.getByRole("button", { name: "Preview delivery", exact: true });
  if (await preview.isVisible()) await preview.click();
  const sent = page.waitForResponse((response) => new URL(response.url()).pathname === `${issueRoot(issueId)}/comments` && response.request().method() === "POST");
  await composer.getByRole("button", { name: "Send comment", exact: true }).click();
  const response = await sent;
  expect(response.status()).toBe(201);
  const receipt: CollaborationCommentReceipt = await response.json();
  expect(receipt.attachment_ids).toEqual([uploaded.id]);
  expect(receipt.targets).toEqual([]);
  return { attachmentId: uploaded.id, receipt };
}

async function noNewVisit(owner: BizRuntimeOwner, count: number, settleMs = 6_000) {
  // A negative fact needs a bounded observation window; the poller cycles in well under this.
  await new Promise((resolve) => setTimeout(resolve, settleMs));
  expect(owner.daemon.visits).toHaveLength(count);
}

async function finish(info: { ev: Evidence; owners: BizRuntimeOwner[]; contexts: BrowserContext[]; errors: string[]; restoreGate: () => Promise<void>; passed: boolean }) {
  const { ev, owners, contexts, errors, restoreGate, passed } = info;
  ev.data.page_errors = errors;
  ev.data.provider_observations = owners.map((owner) => ({ owner_id: owner.actor.id, runtime_id: owner.runtime.runtimeId,
    visits: owner.daemon.evidence().map(({ identity, inputs, agent_id, agent_owner_id, resource_owner_id, cli_context, attachment_hashes, knowledge_observation,
      started_ms, released_ms, closed_ms, replies }) => ({ identity, inputs, agent_id, agent_owner_id, resource_owner_id, step_kind: cli_context.step_kind,
      squad_id: cli_context.squad_id, dispatch_comment: cli_context.dispatch_comment, attachment_hashes, knowledge_observation, started_ms, released_ms, closed_ms, replies })) }));
  const closed = await closeOwned(owners, contexts, passed);
  let gateRestored = false;
  try { await restoreGate(); gateRestored = true; } catch (error) { closed.failures.push(error instanceof Error ? error.message : "gate restore failed"); }
  await ev.write(passed && closed.failures.length === 0 ? "passed" : "failed", { cleanup: closed.cleanup, cleanup_failures: closed.failures, gate_restored: gateRestored });
  expect(closed.failures, "owned daemons exit cleanly with acknowledged host proofs; gate restored").toEqual([]);
}

async function failureShots(ev: Evidence, contexts: BrowserContext[], error: unknown) {
  ev.data.failure = error instanceof Error ? error.message : String(error);
  for (const [index, context] of contexts.entries()) {
    for (const [pageIndex, page] of context.pages().entries()) await ev.shot(page, `failure-actor-${index + 1}-page-${pageIndex + 1}`);
  }
}

test("S2 support ticket: manual intake becomes AI-assisted, files reach the agent, and a reopened ticket is reworked", async ({ browser }, info) => {
  test.setTimeout(600_000);
  test.skip(!hostRuntimeConfigured(), "Requires the task-owned password API, production Web and host Provider binaries");
  const env = hostEnvironment();
  const baseURL = info.project.use.baseURL!;
  const restoreGate = await openHostAcceptanceGate();
  const contexts: BrowserContext[] = [], owners: BizRuntimeOwner[] = [], errors: string[] = [];
  const ev = new Evidence("s2-support-ticket", info);
  let passed = false;
  try {
    const suffix = randomUUID().replaceAll("-", "").slice(0, 10);
    const S = await bizActor(env.apiUrl, "support", "Support lead S", suffix);
    const D = await bizActor(env.apiUrl, "developer", "Developer D", suffix);
    const X = await bizActor(env.apiUrl, "outsider", "Unrelated account X", suffix);
    const supportWs = await S.client.ensureWorkspace("Customer support center", `biz-support-${suffix}`);
    if (!supportWs) throw new Error("Support workspace missing");
    const dev = await bizRuntimeOwner(D, env, "R&D platform", `biz-rnd-${suffix}`);
    owners.push(dev);
    const agent = await bizAgent(dev, `Triage agent ${suffix}`);
    ev.data.topology = { support: { account_id: S.id, workspace_id: supportWs.id }, developer: { account_id: D.id, workspace_id: dev.workspace.id,
      runtime_id: dev.runtime.runtimeId, installation_id: dev.runtime.installationId, agent_id: agent.id }, unrelated_account_id: X.id };
    const s = await openActor(browser, S, contexts, errors), d = await openActor(browser, D, contexts, errors);

    // 1. Manual intake with the customer's log.
    const log = { name: "customer-log.txt", body: `Ticket ${suffix}: ERROR export timed out after 30s for account ACME-42 (synthetic)\n` };
    const issue = await createInUI(s, baseURL, { workspaceId: supportWs.id, title: `Customer export fails ${suffix}`, policy: "manual", recipient: D, file: log,
      description: "Synthetic support ticket: a customer's CSV export times out. Reproduce, fix and confirm with support." });
    const root = issueRoot(issue.id);
    let current = await readIssue(S, issue.id);
    expect(current).toMatchObject({ ai_policy: "manual", handler: { user_id: D.id }, creator: { user_id: S.id } });
    expect(await workbench(D, "current")).toContain(issue.id);
    expect((await notifications(D, issue.id)).map((item) => item.kind)).toContain("assigned");
    await showIssue(s, baseURL, issue); await showIssue(d, baseURL, issue);
    ev.record("support creates a manual ticket for development with the customer log", { issue_id: issue.id, ai_policy: current.ai_policy });

    // 2. Information round trip; the sample arrives as a shared comment file.
    const asked = await directedFormThroughUI(d, issue.id, "request_information", "Which export format and date range does the customer use?", S);
    expect(asked.receipt.code).toBe("information_requested");
    expect((await readIssue(S, issue.id)).handler.user_id).toBe(S.id);
    const answered = await directedFormThroughUI(s, issue.id, "return_information", "CSV, last 90 days. A sample export follows in the comments.");
    expect(answered.receipt.code).toBe("information_returned");
    expect((await readIssue(S, issue.id)).handler.user_id).toBe(D.id);
    const sample = { name: "sample-export.csv", body: `account,rows\nACME-42,184000\n# ${suffix}\n` };
    await showIssue(s, baseURL, issue);
    const shared = await commentWithFileInUI(s, issue.id, "Sample export from the customer (synthetic).", sample);
    const files = await S.client.requestJSON<{ attachments: { id: string; name: string }[] }>(`${root}/attachments`);
    const logFile = files.attachments.find((item) => item.name === log.name);
    expect(files.attachments.map((item) => item.id).sort()).toEqual([logFile?.id, shared.attachmentId].sort());
    ev.record("information requested and returned; sample shared as a comment file", { requested: asked.receipt.code, returned: answered.receipt.code,
      files: files.attachments });

    // 3. Manual policy refuses AI admission.
    current = await readIssue(D, issue.id);
    const manualOffer = await rawCall(env.apiUrl, D, `${root}/resources/offers`, "POST", { ...hostAuthority(current), kind: "agent", resource_id: agent.id,
      execution_workspace_id: dev.workspace.id, roster: [{ role: "agent", agent_id: agent.id, runtime_id: dev.runtime.runtimeId,
        execution_workspace_id: dev.workspace.id, owner_user_id: D.id }] });
    expect(manualOffer).toMatchObject({ status: 409, body: { code: "task_manual" } });
    expect((await D.client.requestJSON<CollaborationMentionTargets>(versioned(`${root}/mention-targets`))).targets).toEqual([]);
    ev.record("manual policy refuses resource offer", { status: manualOffer.status, code: manualOffer.body?.code });

    // 4. The initiator turns on AI assistance; nothing resumes by itself.
    const policy = await changePolicyInUI(s, issue.id, "assisted");
    current = await readIssue(S, issue.id);
    expect(current.ai_policy).toBe("assisted");
    expect(current.session.queue_state).toBe("paused");
    const developerPolicy = await rawCall(env.apiUrl, D, `${root}/ai-policy`, "POST", {
      ...gapWorkflowObservations(await readIssue(D, issue.id)), ai_policy: "manual" });
    expect(developerPolicy.status).toBe(403);
    ev.record("initiator switched to assisted; queue stays paused; the handler cannot change the policy", { receipt: policy.receipt.code, developer_status: developerPolicy.status });

    // 5. Offer, adopt, owner consent; an instruction waits until explicit Resume.
    const target = await offerAndAdopt(dev, S, issue.id, "agent", agent.id, [{ role: "agent", agentId: agent.id }]);
    await showIssue(d, baseURL, issue);
    await consentInUI(d, issue, target, agent);
    expect((await readIssue(S, issue.id)).session.queue_state).toBe("paused");
    await showIssue(d, baseURL, issue);
    const first = await commentThroughUI(d, issue.id, "Reproduce the export timeout with both shared files and propose a fix.", [agent.name]);
    const firstCommand = first.receipt.targets[0]?.command_id;
    expect(first.receipt.targets).toEqual([expect.objectContaining({ target_id: target.targetId, state: "waiting" })]);
    await noNewVisit(dev, 0);
    await hostControl(d, hostIssue(issue, target, agent), "resume", "issue");
    const visit1 = await dev.daemon.waitForVisit(1);
    expect(visit1).toMatchObject({ agent_id: agent.id, resource_owner_id: D.id, identity: { issue_id: issue.id, target_id: target.targetId } });
    expect(visit1.inputs.map((input) => [input.command_id, input.author_user_id])).toEqual([[firstCommand, D.id]]);
    expect(visit1.attachment_hashes).toEqual({ [logFile!.id]: sha256(log.body), [shared.attachmentId]: sha256(sample.body) });
    dev.daemon.release(visit1);
    await waitRun(D, issue.id, visit1.identity.run_id!, { progress_state: "settled", proof_state: "host_exit_observed", launch_count: "1", business_outcome: "success", execution_mode: hostMode });
    expect((await readIssue(S, issue.id)).status_category).not.toBe("done");
    ev.record("agent ran only after explicit Resume and received both shared files", { run_id: visit1.identity.run_id, attachment_hashes: visit1.attachment_hashes });

    // 6. Delivery and acceptance.
    const submitted = await directedFormThroughUI(d, issue.id, "submit_acceptance", "Fixed: export streams in pages; timeout no longer reproduces.", undefined,
      `Synthetic run ${visit1.identity.run_id}; customer log and sample both checked.`);
    expect(submitted.receipt.code).toBe("acceptance_submitted");
    const accepted = await decideClosureResult(s, issue.id);
    current = await readIssue(S, issue.id);
    expect(current.status_category).toBe("done");
    expect(current.capabilities.submit_command).toBe(false);
    await ev.shot(s, "s2-first-acceptance");
    ev.record("first delivery accepted", { submission_id: accepted.submission_id });

    // 7. Regression: reopen keeps AI held until an explicit Resume.
    const reopened = await lifecycleThroughUI(s, issue.id, "reopen");
    current = await readIssue(S, issue.id);
    expect(current.status_category).not.toBe("done");
    // Reopen pauses the target's execution lane; it does not resume by itself.
    expect(current.execution!.channels.find((lane) => lane.id === target.channelId)).toMatchObject({ queue_state: "paused" });
    ev.data.reopened_lane = current.execution!.channels.find((lane) => lane.id === target.channelId);
    // Acceptance handed responsibility to support; reopening does not return it to development.
    expect(current.handler.user_id).toBe(S.id);
    expect((await readIssue(D, issue.id)).capabilities.submit_acceptance).toBe(false);
    const rework = await simpleWorkflow(s, issue.id, "transfer", D, "Customer reproduced the issue; return to development for rework.");
    current = await readIssue(D, issue.id);
    expect(current.handler.user_id).toBe(D.id);
    expect(current.capabilities.submit_acceptance).toBe(true);
    expect(current.delivery?.latest).toMatchObject({ id: accepted.submission_id, decision: { kind: "accepted" } });
    expect(current.execution!.channels.find((lane) => lane.id === target.channelId)).toMatchObject({ queue_state: "paused" });
    expect(dev.daemon.visits).toHaveLength(1);
    ev.record("reopen preserved support's responsibility; support explicitly transferred rework to development without resuming AI", {
      receipt: rework.receipt.code, from_user_id: S.id, to_user_id: D.id, prior_submission_id: accepted.submission_id });
    await showIssue(d, baseURL, issue);
    const second = await commentThroughUI(d, issue.id, "Customer reports the timeout again for 180-day exports; extend the fix.", [agent.name]);
    expect(second.receipt.targets).toEqual([expect.objectContaining({ target_id: target.targetId, state: "waiting" })]);
    await noNewVisit(dev, 1);
    await hostControl(d, hostIssue(issue, target, agent), "resume", "issue");
    const visit2 = await dev.daemon.waitForVisit(2);
    expect(visit2.inputs.map((input) => [input.command_id, input.author_user_id])).toEqual([[second.receipt.targets[0]!.command_id, D.id]]);
    dev.daemon.release(visit2);
    await waitRun(D, issue.id, visit2.identity.run_id!, { progress_state: "settled", proof_state: "host_exit_observed", business_outcome: "success" });
    expect((await readIssue(S, issue.id)).status_category).not.toBe("done");
    const resubmitted = await directedFormThroughUI(d, issue.id, "submit_acceptance", "Rework: 180-day exports now stream; regression closed.", undefined,
      `Synthetic run ${visit2.identity.run_id}.`);
    expect(resubmitted.receipt.code).toBe("acceptance_submitted");
    const reaccepted = await decideClosureResult(s, issue.id);
    current = await readIssue(S, issue.id);
    expect(current.status_category).toBe("done");
    const history = await S.client.requestJSON<CollaborationDeliveryHistory>(`${root}/acceptance-submissions?limit=100`);
    expect(history.items.map((item) => [item.id, item.author_user_id, item.decision?.kind]).sort())
      .toEqual([[accepted.submission_id, D.id, "accepted"], [reaccepted.submission_id, D.id, "accepted"]].sort());
    expect((await rawCall(env.apiUrl, X, versioned(root))).status).toBe(404);
    expect(dev.daemon.visits).toHaveLength(2);
    await ev.shot(s, "s2-reaccepted-support"); await ev.shot(d, "s2-reaccepted-developer");
    ev.data.runs = [visit1.identity.run_id, visit2.identity.run_id];
    ev.data.delivery_history = history.items;
    ev.record("reopened ticket reworked and accepted again; unrelated account denied", { reopened: reopened.receipt.code, final_status: current.status });
    expect(errors).toEqual([]);
    passed = true;
  } catch (error) { await failureShots(ev, contexts, error); throw error; }
  finally { await finish({ ev, owners, contexts, errors, restoreGate, passed }); }
});

test("S3 governance: live withdrawal really stops work, removal and cancellation fence AI, and the issue still closes", async ({ browser }, info) => {
  test.setTimeout(600_000);
  test.skip(!hostRuntimeConfigured(), "Requires the task-owned password API, production Web and host Provider binaries");
  const env = hostEnvironment();
  const baseURL = info.project.use.baseURL!;
  const restoreGate = await openHostAcceptanceGate();
  const contexts: BrowserContext[] = [], owners: BizRuntimeOwner[] = [], errors: string[] = [];
  const ev = new Evidence("s3-governance", info);
  let passed = false;
  try {
    const suffix = randomUUID().replaceAll("-", "").slice(0, 10);
    const L = await bizActor(env.apiUrl, "lead", "Program lead L", suffix);
    const O1 = await bizActor(env.apiUrl, "partnerone", "Partner O1", suffix);
    const O2 = await bizActor(env.apiUrl, "partnertwo", "Partner O2", suffix);
    const leadWs = await L.client.ensureWorkspace("Program office", `biz-program-${suffix}`);
    if (!leadWs) throw new Error("Lead workspace missing");
    const p1 = await bizRuntimeOwner(O1, env, "Partner one studio", `biz-p1-${suffix}`); owners.push(p1);
    const p2 = await bizRuntimeOwner(O2, env, "Partner two studio", `biz-p2-${suffix}`); owners.push(p2);
    const a1 = await bizAgent(p1, `Partner one agent ${suffix}`), a2 = await bizAgent(p2, `Partner two agent ${suffix}`);
    ev.data.topology = { lead: { account_id: L.id, workspace_id: leadWs.id },
      partners: [[O1, p1, a1], [O2, p2, a2]].map(([actor, owner, agent]) => ({ account_id: (actor as BizActor).id, workspace_id: (owner as BizRuntimeOwner).workspace.id,
        runtime_id: (owner as BizRuntimeOwner).runtime.runtimeId, agent_id: (agent as { id: string }).id })) };
    const l = await openActor(browser, L, contexts, errors), o1 = await openActor(browser, O1, contexts, errors), o2 = await openActor(browser, O2, contexts, errors);
    const issue = await createInUI(l, baseURL, { workspaceId: leadWs.id, title: `Partner integration governance ${suffix}`, policy: "assisted", recipient: O1,
      description: "Synthetic program: two partners contribute agents; the lead keeps control of access and lifecycle." });
    const root = issueRoot(issue.id);
    await showIssue(l, baseURL, issue);
    await simpleWorkflow(l, issue.id, "invite", O2);
    const t1 = await offerAndAdopt(p1, L, issue.id, "agent", a1.id, [{ role: "agent", agentId: a1.id }]);
    const t2 = await offerAndAdopt(p2, L, issue.id, "agent", a2.id, [{ role: "agent", agentId: a2.id }]);
    await showIssue(o1, baseURL, issue); await consentInUI(o1, issue, t1, a1);
    await showIssue(o2, baseURL, issue); await consentInUI(o2, issue, t2, a2);
    await showIssue(l, baseURL, issue);
    await hostControl(l, hostIssue(issue, t1, a1), "resume", "issue");
    ev.record("lead created the issue; both partners offered, were adopted, consented; lead resumed", { issue_id: issue.id, targets: [t1.targetId, t2.targetId] });

    // 1. O1's first instruction is actually running; the second waits behind it.
    await showIssue(o1, baseURL, issue);
    const A1 = await sendHostComment(o1, hostIssue(issue, t1, a1), "Integrate the partner one payment adapter (synthetic).", O1.id);
    const v1 = await p1.daemon.waitForVisit(1);
    expect(v1.inputs.map((input) => input.command_id)).toEqual([A1.commandId]);
    const A2 = await sendHostComment(o1, hostIssue(issue, t1, a1), "Then add retry metrics for the adapter.", O1.id);
    let current = await readIssue(L, issue.id);
    expect(current.execution!.commands.find((item) => item.id === A2.commandId)).toMatchObject({ state: "waiting", group_id: null });

    // 2. O1 withdraws while the process runs. Its daemon is suspended, so the stop is requested but not yet confirmed.
    p1.daemon.freeze();
    let withdrawn: Awaited<ReturnType<typeof withdrawInUI>>;
    try {
      await showIssue(o1, baseURL, issue);
      withdrawn = await withdrawInUI(o1, issue.id, a1.name);
      expect(withdrawn.code).toBe("grant_revoked");
      expect(withdrawn.channels).toEqual([expect.objectContaining({ channel_id: t1.channelId, code: "stop_requested", stopped_runs: 1, unsettled_runs: 1 })]);
      expect(withdrawn.target_holds).toEqual([expect.objectContaining({ target_id: t1.targetId, hold_reason: "grant_required" })]);
      current = await readIssue(L, issue.id);
      expect(current.execution!.runs.find((run) => run.id === v1.identity.run_id)).toMatchObject({ forced_disposition: "stopped", settled_at: null });
      expect(current.execution!.commands.find((item) => item.id === A2.commandId)).toMatchObject({ state: "waiting", group_id: null });
      expect(current.execution!.targets.find((item) => item.id === t2.targetId)?.hold_reason).toBeNull();
      ev.record("O1 withdrew during an actual run: stop requested, target held, queued input kept, O2 untouched", { receipt: withdrawn });

      // 3. O2 keeps working on its own runtime meanwhile.
      await showIssue(o2, baseURL, issue);
      const B1 = await sendHostComment(o2, hostIssue(issue, t2, a2), "Partner two: validate the shared schema (synthetic).", O2.id);
      const w1 = await p2.daemon.waitForVisit(1);
      expect(w1.inputs.map((input) => [input.command_id, input.author_user_id])).toEqual([[B1.commandId, O2.id]]);
      p2.daemon.release(w1);
      await waitRun(L, issue.id, w1.identity.run_id!, { progress_state: "settled", proof_state: "host_exit_observed", business_outcome: "success" });

      // 4. Re-consent alone does not resume, and Resume is fenced while termination is unconfirmed.
      await showIssue(o1, baseURL, issue);
      await consentInUI(o1, issue, t1, a1);
      current = await readIssue(O1, issue.id);
      expect(current.execution!.targets.find((item) => item.id === t1.targetId)?.hold_reason).not.toBeNull();
      const fenced = await rawCall(env.apiUrl, O1, `${root}/control`, "POST", { ...gapControlBody(current, "resume", "target", t1.targetId), execution_view_version: 6 });
      ev.record("re-consent kept the hold; Resume before confirmed termination", { status: fenced.status, code: fenced.body?.code });
      expect(fenced.body?.code).toBe("stop_unconfirmed");
      await noNewVisit(p1, 1, 2_000);
    } finally { p1.daemon.thaw(); }

    // 5. The runtime observes the stop and proves the actual process ended without being released.
    const stoppedRun = await waitRun(L, issue.id, v1.identity.run_id!, { progress_state: "settled", proof_state: "host_exit_observed", forced_disposition: "stopped", launch_count: "1" });
    expect(v1.released_ms).toBeNull();
    await expect.poll(() => v1.closed_ms, { timeout: 30_000 }).not.toBeNull();
    current = await readIssue(L, issue.id);
    expect(current.execution!.commands.find((item) => item.id === A1.commandId)?.state).toBe("stopped");
    await showIssue(o1, baseURL, issue);
    await controlTargetInUI(o1, issue.id, t1, "resume");
    const v2 = await p1.daemon.waitForVisit(2);
    expect(v2.inputs.map((input) => [input.command_id, input.author_user_id])).toEqual([[A2.commandId, O1.id]]);
    p1.daemon.release(v2);
    await waitRun(L, issue.id, v2.identity.run_id!, { progress_state: "settled", proof_state: "host_exit_observed", business_outcome: "success" });
    ev.record("termination proven, explicit Resume ran the kept input once", { stopped_run: stoppedRun.id, resumed_run: v2.identity.run_id });

    // 6. The lead removes O2 while O2's run is active and another input waits.
    await showIssue(o2, baseURL, issue);
    const B2 = await sendHostComment(o2, hostIssue(issue, t2, a2), "Partner two: migrate the reporting job (synthetic).", O2.id);
    const w2 = await p2.daemon.waitForVisit(2);
    expect(w2.inputs.map((input) => input.command_id)).toEqual([B2.commandId]);
    const B3 = await sendHostComment(o2, hostIssue(issue, t2, a2), "Partner two: then archive the old job.", O2.id);
    const removed = await removeParticipantInUI(l, issue.id, { id: O2.id, name: O2.name });
    await waitRun(L, issue.id, w2.identity.run_id!, { progress_state: "settled", proof_state: "host_exit_observed", forced_disposition: "stopped" });
    expect(w2.released_ms).toBeNull();
    current = await readIssue(L, issue.id);
    expect(current.execution!.commands.find((item) => item.id === B2.commandId)?.state).toBe("stopped");
    expect(current.execution!.commands.find((item) => item.id === B3.commandId)?.state).toBe("cancelled");
    const lost = await rawCall(env.apiUrl, O2, versioned(root));
    expect(lost.status).toBe(404);
    expect(lost.text).not.toContain(issue.title);
    const o2Notes = await notifications(O2, issue.id);
    expect(o2Notes.find((item) => item.kind === "removed")).toMatchObject({ can_open: false, issue_title: null });
    expect((await L.client.requestJSON<CollaborationMentionTargets>(versioned(`${root}/mention-targets`))).targets.map((item) => item.target_id)).not.toContain(t2.targetId);
    ev.record("removing O2 stopped its run, cancelled its waiting input, revoked access; notice is content-free", { receipt: removed.code,
      denied_status: lost.status, removed_notice: o2Notes.find((item) => item.kind === "removed") });

    // 7. Cancellation fences AI; only the initiator reopens, and work then closes normally.
    const cancelled = await lifecycleThroughUI(l, issue.id, "cancel");
    current = await readIssue(O1, issue.id);
    expect(current.status_category).toBe("cancelled");
    expect(current.capabilities.submit_command).toBe(false);
    const closedResume = await rawCall(env.apiUrl, O1, `${root}/control`, "POST", { ...gapControlBody(current, "resume", "issue"), execution_view_version: 6 });
    expect(closedResume.status).toBeGreaterThanOrEqual(400);
    const partnerReopen = await rawCall(env.apiUrl, O1, `${root}/lifecycle`, "POST", { ...gapWorkflowObservations(current),
      action: "reopen", status: "todo", status_options_version: "1" });
    expect(partnerReopen.status).toBe(403);
    ev.record("cancelled: AI submission off, Resume refused, a partner cannot reopen", { cancelled: cancelled.receipt.code, resume_status: closedResume.status,
      resume_code: closedResume.body?.code, partner_reopen_status: partnerReopen.status });
    await lifecycleThroughUI(l, issue.id, "reopen");
    await showIssue(o1, baseURL, issue);
    await hostControl(o1, hostIssue(issue, t1, a1), "resume", "issue");
    const A3 = await sendHostComment(o1, hostIssue(issue, t1, a1), "Finish the adapter handover notes (synthetic).", O1.id);
    const v3 = await p1.daemon.waitForVisit(3);
    expect(v3.inputs.map((input) => input.command_id)).toEqual([A3.commandId]);
    p1.daemon.release(v3);
    await waitRun(L, issue.id, v3.identity.run_id!, { progress_state: "settled", proof_state: "host_exit_observed", business_outcome: "success" });
    expect((await readIssue(L, issue.id)).status_category).not.toBe("done");
    const delivered = await directedFormThroughUI(o1, issue.id, "submit_acceptance", "Adapter integrated with retry metrics and handover notes.", undefined,
      `Runs ${v2.identity.run_id} and ${v3.identity.run_id}; withdrawn run ${v1.identity.run_id} stopped.`);
    expect(delivered.receipt.code).toBe("acceptance_submitted");
    await decideClosureResult(l, issue.id);
    const final = await readIssue(L, issue.id);
    expect(final.status_category).toBe("done");
    expect([p1.daemon.visits.length, p2.daemon.visits.length]).toEqual([3, 2]);
    await ev.shot(l, "s3-final-lead"); await ev.shot(o1, "s3-final-partner-one"); await ev.shot(o2, "s3-removed-partner-two");
    ev.data.runs = { partner_one: p1.daemon.visits.map((visit) => visit.identity.run_id), partner_two: p2.daemon.visits.map((visit) => visit.identity.run_id) };
    ev.record("reopened, resumed, delivered and accepted", { final_status: final.status });
    expect(errors).toEqual([]);
    passed = true;
  } catch (error) { await failureShots(ev, contexts, error); throw error; }
  finally { await finish({ ev, owners, contexts, errors, restoreGate, passed }); }
});

test("S1 outsourcing: client, vendor manager and workspace-less engineer close an issue with a vendor squad and reuse approved knowledge", async ({ browser }, info) => {
  test.setTimeout(720_000);
  test.skip(!hostRuntimeConfigured(), "Requires the task-owned password API, production Web and host Provider binaries");
  test.skip(process.env.QA_KNOWLEDGE_CONTEXT !== "on", "Requires the task API restarted with MULTICA_KNOWLEDGE_CONTEXT_ENABLED=true");
  const env = hostEnvironment();
  const baseURL = info.project.use.baseURL!;
  const restoreGate = await openHostAcceptanceGate();
  const contexts: BrowserContext[] = [], owners: BizRuntimeOwner[] = [], errors: string[] = [];
  const ev = new Evidence("s1-outsourcing", info);
  let passed = false;
  try {
    const suffix = randomUUID().replaceAll("-", "").slice(0, 10);
    const A = await bizActor(env.apiUrl, "clienta", "Client lead A", suffix);
    const B = await bizActor(env.apiUrl, "vendorb", "Vendor manager B", suffix);
    const C = await bizActor(env.apiUrl, "vendorc", "Vendor engineer C", suffix);
    const Y = await bizActor(env.apiUrl, "rivaly", "Rival vendor Y", suffix);
    const clientWs = await A.client.ensureWorkspace("Client product office", `biz-client-${suffix}`);
    if (!clientWs) throw new Error("Client workspace missing");
    const vendor = await bizRuntimeOwner(B, env, "Vendor delivery center", `biz-vendor-${suffix}`); owners.push(vendor);
    const architect = await bizAgent(vendor, `Vendor architect ${suffix}`), developer = await bizAgent(vendor, `Vendor developer ${suffix}`);
    const squad = await bizSquad(vendor, `Vendor delivery squad ${suffix}`, architect.id, [developer.id]);
    await Y.client.ensureWorkspace("Rival vendor", `biz-rival-${suffix}`);
    expect(await C.client.getWorkspaces()).toEqual([]);
    ev.data.topology = { client: { account_id: A.id, workspace_id: clientWs.id }, vendor_manager: { account_id: B.id, workspace_id: vendor.workspace.id,
      runtime_id: vendor.runtime.runtimeId, installation_id: vendor.runtime.installationId, squad_id: squad.id, leader_id: architect.id, member_id: developer.id },
      vendor_engineer: { account_id: C.id, workspaces: [] }, rival_account_id: Y.id };
    const a = await openActor(browser, A, contexts, errors), b = await openActor(browser, B, contexts, errors), c = await openActor(browser, C, contexts, errors);
    observeKnowledgeHints(c);

    // 1. The client hands phase one to the vendor manager.
    const spec = { name: "phase-one-requirements.md", body: `# Phase one ${suffix}\nPaginate the order export; keep totals stable. (synthetic)\n` };
    const issue = await createInUI(a, baseURL, { workspaceId: clientWs.id, title: `Order export pagination phase one ${suffix}`, policy: "assisted", recipient: B, file: spec,
      description: "Synthetic outsourcing: the vendor delivers paginated order exports; the client accepts the result." });
    const root = issueRoot(issue.id);
    expect(await workbench(B, "current")).toContain(issue.id);
    expect((await notifications(B, issue.id)).map((item) => item.kind)).toContain("assigned");

    // 2. Vendor-internal relay to an engineer who belongs to no workspace.
    await showIssue(b, baseURL, issue);
    await simpleWorkflow(b, issue.id, "invite", C);
    await simpleWorkflow(b, issue.id, "transfer", C, "Implement with our delivery squad and submit to the client.");
    expect((await readIssue(A, issue.id)).handler.user_id).toBe(C.id);
    expect(await workbench(C, "current")).toContain(issue.id);
    expect(await workbench(B, "current")).not.toContain(issue.id);
    expect(await workbench(B, "participating")).toContain(issue.id);
    expect((await notifications(C, issue.id)).map((item) => item.kind)).toEqual(expect.arrayContaining(["participant_added", "transferred"]));
    expect(await C.client.getWorkspaces()).toEqual([]);
    ev.record("client assigned the vendor manager; the manager relayed to a workspace-less engineer", { issue_id: issue.id });

    // 3. The vendor's squad: B offers, C adopts, B consents each roster resource, C resumes.
    const S = await offerAndAdopt(vendor, C, issue.id, "squad", squad.id, [{ role: "leader", agentId: architect.id }, { role: "member", agentId: developer.id }]);
    expect((await C.client.requestJSON<CollaborationMentionTargets>(versioned(`${root}/mention-targets`))).targets.map((item) => item.target_id)).not.toContain(S.targetId);
    await showIssue(b, baseURL, issue);
    await consentInUI(b, issue, S, architect);
    await showIssue(b, baseURL, issue);
    await consentInUI(b, issue, S, developer);
    await showIssue(c, baseURL, issue);
    await hostControl(c, hostIssue(issue, S, architect), "resume", "issue");
    ev.record("vendor squad offered by its owner, adopted by the engineer, consented per resource, resumed", { target_id: S.targetId });

    // 4. Leader -> member -> leader continuation, each a real run on B's runtime.
    const pick = { kind: "squad" as const, resourceId: squad.id, name: S.name };
    const order = await sendSquadComment(c, { issue: { id: issue.id } }, "Deliver paginated order exports per the attached requirements.", [pick], async () => undefined);
    const orderCommand = order.byTarget(S.targetId).command_id!;
    const leader = await vendor.daemon.waitForVisit(1);
    expect(leader).toMatchObject({ agent_id: architect.id, resource_owner_id: B.id, cli_context: { step_kind: "leader", squad_id: squad.id } });
    expect(leader.inputs.map((input) => [input.command_id, input.author_user_id])).toEqual([[orderCommand, C.id]]);
    const dispatchText = `[@${developer.name}](mention://agent/${developer.id}) implement cursor pagination for the export and report back`;
    const [dispatch] = await vendor.daemon.act(leader, [{ kind: "comment", request_id: randomUUID(), content: dispatchText }]);
    const memberRunId = (dispatch!.result as { run_ids: string[] }).run_ids[0]!;
    vendor.daemon.release(leader);
    const member = await vendor.daemon.waitForVisit(2);
    expect(member).toMatchObject({ agent_id: developer.id, resource_owner_id: B.id, identity: { run_id: memberRunId },
      cli_context: { step_kind: "member", dispatch_comment: dispatchText } });
    const [reply] = await vendor.daemon.act(member, [{ kind: "comment", request_id: randomUUID(), content: "Cursor pagination implemented; totals verified on synthetic data." }]);
    const continuationRunId = (reply!.result as { continuation_run_id: string | null }).continuation_run_id;
    expect(continuationRunId).not.toBeNull();
    vendor.daemon.release(member);
    const continuation = await vendor.daemon.waitForVisit(3);
    expect(continuation).toMatchObject({ agent_id: architect.id, identity: { run_id: continuationRunId }, cli_context: { step_kind: "continuation", squad_id: squad.id } });
    const summary = "Squad summary: cursor pagination delivered; export totals are stable across pages.";
    await vendor.daemon.act(continuation, [{ kind: "comment", request_id: randomUUID(), content: summary },
      { kind: "squad_activity", request_id: randomUUID(), outcome: "action", reason: "Integrated the member implementation" }]);
    vendor.daemon.release(continuation);
    await waitRun(C, issue.id, continuationRunId!, { progress_state: "settled", proof_state: "host_exit_observed", business_outcome: "success" });
    const groupId = continuation.identity.group_id!;
    await expect.poll(async () => (await readIssue(C, issue.id)).execution!.groups.find((group) => group.id === groupId)?.state, { timeout: 30_000 }).toBe("succeeded");
    expect((await readIssue(A, issue.id)).status_category).not.toBe("done");
    ev.record("squad delegated, member wrote back, leader continued; business still open", { runs: [leader.identity.run_id, memberRunId, continuationRunId], group_id: groupId });

    // 5. First delivery is returned by the client.
    const first = await directedFormThroughUI(c, issue.id, "submit_acceptance", "Phase one: paginated export delivered by the vendor squad.", undefined,
      `Squad group ${groupId}.`);
    expect(first.receipt.code).toBe("acceptance_submitted");
    const returned = await decideClosureResult(a, issue.id, C, "Add accessibility acceptance notes for the export screen before final delivery.");
    expect((await readIssue(A, issue.id)).handler.user_id).toBe(C.id);
    await ev.shot(a, "s1-first-delivery-returned");

    // 6. Revision by the squad leader alone, then knowledge from the shared context.
    await showIssue(c, baseURL, issue);
    const fix = await sendSquadComment(c, { issue: { id: issue.id } }, "Add the accessibility acceptance notes the client asked for.", [pick], async () => undefined);
    const leader2 = await vendor.daemon.waitForVisit(4);
    expect(leader2.inputs.map((input) => input.command_id)).toEqual([fix.byTarget(S.targetId).command_id]);
    await vendor.daemon.act(leader2, [{ kind: "comment", request_id: randomUUID(), content: "Accessibility notes added: keyboard paging and screen-reader totals." },
      { kind: "squad_activity", request_id: randomUUID(), outcome: "action", reason: "Leader handled the revision without delegation" }]);
    vendor.daemon.release(leader2);
    await waitRun(C, issue.id, leader2.identity.run_id!, { progress_state: "settled", proof_state: "host_exit_observed", business_outcome: "success" });
    const conclusion = `Export pagination rule ${suffix}: use cursor pages and recompute totals per page; document keyboard paging for accessibility.`;
    await c.getByRole("group", { name: "Issue resources", exact: true }).getByRole("button", { name: "Shared context", exact: true }).click();
    await saveMemoryInUI(c, conclusion, "0");
    await c.keyboard.press("Escape");
    await expect(c.getByRole("dialog").filter({ has: c.getByRole("group", { name: "Browse issue resources", exact: true }) })).toBeHidden();
    await revealCollaborationDisclosure(c, "Knowledge candidates");
    await sourcePanel(c).getByRole("button", { name: "Propose knowledge", exact: true }).click();
    await fillKnowledge(c, `Export pagination rule ${suffix}`, conclusion, "Shared context");
    const proposed = await submitKnowledge(c);
    expect(proposed.sent.target).toMatchObject({ kind: "workspace", workspace_id: clientWs.id });
    const candidateId = proposed.receipt.candidate_id;
    const vendorApproval = await rawCall(env.apiUrl, B, `/api/collaboration/knowledge-candidates/${candidateId}/approve`, "POST", { protocol_version: 1,
      request_id: randomUUID(), expected_version: proposed.receipt.version, expected_state_revision: proposed.receipt.state_revision, expected_content_sha256: sha256(conclusion) });
    expect(vendorApproval).toMatchObject({ status: 403, body: { code: "knowledge_permission_denied" } });
    const second = await directedFormThroughUI(c, issue.id, "submit_acceptance", "Phase one revised: accessibility notes added.", undefined,
      `Leader run ${leader2.identity.run_id}; first return retained.`);
    expect(second.receipt.code).toBe("acceptance_submitted");
    ev.record("revision by the leader; engineer proposed knowledge to the client workspace; the vendor manager cannot approve it",
      { candidate_id: candidateId, target: proposed.sent.target, sources: proposed.sent.sources, vendor_approval_status: vendorApproval.status,
        vendor_approval_code: vendorApproval.body?.code });

    // 7. The client approves the exact knowledge version and accepts the revision.
    await showIssue(a, baseURL, issue);
    await revealCollaborationDisclosure(a, "Knowledge candidates");
    await chooseCandidate(a, `Export pagination rule ${suffix}`);
    await approveKnowledge(a, A.client, candidateId);
    const accepted = await decideClosureResult(a, issue.id);
    const closed = await readIssue(A, issue.id);
    expect(closed.status_category).toBe("done");
    const history = await A.client.requestJSON<CollaborationDeliveryHistory>(`${root}/acceptance-submissions?limit=100`);
    expect(history.items.find((item) => item.id === returned.submission_id)).toMatchObject({ author_user_id: C.id, decision: { kind: "returned", actor_user_id: A.id, target_user_id: C.id } });
    expect(history.items.find((item) => item.id === accepted.submission_id)).toMatchObject({ author_user_id: C.id, decision: { kind: "accepted", actor_user_id: A.id } });
    await ev.shot(a, "s1-phase-one-accepted-client"); await ev.shot(c, "s1-phase-one-accepted-engineer");
    ev.record("client approved the knowledge and accepted the revised delivery", { accepted: accepted.submission_id, final_status: closed.status });

    // 8. The rival vendor sees nothing.
    const rival = await rawCall(env.apiUrl, Y, versioned(root));
    expect(rival.status).toBe(404);
    expect(rival.text).not.toContain(issue.title);

    // 9. Phase two in the same client workspace: the vendor's agent run receives the approved knowledge.
    const phaseTwo = await A.client.requestJSON<CollaborationIssue>("/api/collaboration/issues", { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      source_workspace_id: clientWs.id, handler_user_id: B.id, title: `Order export phase two ${suffix}`, ai_policy: "assisted",
      description: "Synthetic phase two: extend the export to refunds.", attachment_ids: [] } });
    const issue2 = { id: phaseTwo.id, title: phaseTwo.title };
    const T2 = await offerAndAdopt(vendor, A, issue2.id, "agent", architect.id, [{ role: "agent", agentId: architect.id }]);
    await showIssue(b, baseURL, issue2);
    await consentInUI(b, issue2, T2, architect);
    await showIssue(b, baseURL, issue2);
    await hostControl(b, hostIssue(issue2, T2, architect), "resume", "issue");
    const kickoff = await sendHostComment(b, hostIssue(issue2, T2, architect), "Plan the refund export using our approved pagination rule.", B.id);
    const visit = await vendor.daemon.waitForVisit(5);
    expect(visit.inputs.map((input) => input.command_id)).toEqual([kickoff.commandId]);
    const observed = visit.knowledge_observation;
    ev.data.phase_two = { issue_id: issue2.id, run_id: visit.identity.run_id, knowledge_observation: observed };
    vendor.daemon.release(visit);
    await waitRun(B, issue2.id, visit.identity.run_id!, { progress_state: "settled", proof_state: "host_exit_observed", business_outcome: "success" });
    expect(observed?.section_present).toBe(true);
    expect(observed?.contributions).toEqual(expect.arrayContaining([expect.objectContaining({ candidate_id: candidateId, source_issue_id: issue.id,
      content_sha256: sha256(conclusion) })]));
    expect((await rawCall(env.apiUrl, Y, versioned(issueRoot(issue2.id)))).status).toBe(404);
    ev.record("phase-two run received the approved contribution", { run_id: visit.identity.run_id, contributions: observed?.contributions });
    expect(errors).toEqual([]);
    passed = true;
  } catch (error) { await failureShots(ev, contexts, error); throw error; }
  finally { await finish({ ev, owners, contexts, errors, restoreGate, passed }); }
});
