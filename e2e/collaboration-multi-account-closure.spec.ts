import { openNewCollaborationIssue } from "./fixtures/sidebar-navigation";
import { expectCommentMenuWithout } from "./fixtures/collaboration-comment-menu";
import { randomUUID } from "node:crypto";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test, type BrowserContext, type Locator, type Page } from "@playwright/test";
import type { CollaborationComments, CollaborationDeliveryHistory, CollaborationIssue, CollaborationMemory, CollaborationMentionTargets, CollaborationResources, CollaborationRunRecords } from "@multica/core/collaboration";
import { hostAuthority, hostControl, hostEnvironment, hostPanel, hostRuntimeConfigured, issueRoot, openHostAcceptanceGate, openHostConsent, sendHostComment, versioned, type HostVisit } from "./fixtures/collaboration-host-runtime";
import { closureActor, createClosureResource, decideClosureResult, offerClosureResource, rawClosureRequest, readClosureIssue, simpleWorkflow, type ClosureActor, type ClosureResource } from "./fixtures/collaboration-multi-account-closure";
import { selectCollaborationRecipient } from "./fixtures/collaboration-offline-handoff";
import { directedFormThroughUI } from "./fixtures/collaboration-handoff-chain";
import { commentThroughUI } from "./fixtures/collaboration-control-gaps";
import { saveMemoryInUI } from "./fixtures/collaboration-memory";
import { nativeScreenshot, openDesktop, type NativeHarness } from "./fixtures/collaboration-native";

interface ClosureVisit extends HostVisit {
  memory_input: { version: number; content: string };
  memory_written: { version: string; code: string; run_id: string };
  result_comment_id: string;
  result_text: string;
  instruction_files_clean: boolean;
}

test.use({ actionTimeout: 20_000, viewport: { width: 1440, height: 1100 }, screenshot: "only-on-failure", trace: "off" });

test("three account owners and three native agents close one cross-workspace issue after revision", async ({ browser }, info) => {
  test.setTimeout(600_000);
  test.skip(!hostRuntimeConfigured(), "Requires explicit task-owned password API, production Web and closure Provider binaries");
  const env = hostEnvironment();
  const config = await fetch(`${env.apiUrl}/api/config`);
  expect(await config.json()).toMatchObject({ auth_mode: "password", managed_installation_supported: true, deployment_id: env.deploymentId });
  const restoreGate = await openHostAcceptanceGate();
  const contexts: BrowserContext[] = [];
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  let nativeProfile: string | undefined;
  const resources: ClosureResource[] = [];
  const errors: string[] = [];
  const steps: { name: string; at: string; facts: unknown }[] = [];
  const evidence: Record<string, unknown> = { scope: "Three owners, three private workspaces, three actual host pollers and deterministic Provider processes; one shared issue",
    exclusions: ["commercial model reasoning or credentials", "physical second machine", "OS sandbox", "product repair", "installed Desktop Main"], steps };
  const record = (name: string, facts: unknown) => steps.push({ name, at: new Date().toISOString(), facts });
  let passed = false;
  try {
    const suffix = randomUUID().replaceAll("-", "").slice(0, 10);
    const P = await closureActor(env.apiUrl, "Product", suffix);
    const D = await closureActor(env.apiUrl, "Development", suffix);
    const Q = await closureActor(env.apiUrl, "Quality", suffix);
    const X = await closureActor(env.apiUrl, "Unrelated", suffix);
    for (const [actor, label] of [[P, "product"], [D, "development"], [Q, "quality"]] as const) {
      resources.push(await createClosureResource(actor, env.apiUrl, env.deploymentId, `closure-${label}-${suffix}`));
    }
    const [product, development, quality] = resources as [ClosureResource, ClosureResource, ClosureResource];
    // The existing name-only owner update accepts TEXT names. Keep the same
    // Agent identity while exercising wrapping with a 104-character word.
    const qualityAgentId = quality.agent.id;
    quality.agent = await Q.client.requestJSON<{ id: string; name: string }>(`/api/agents/${qualityAgentId}`, {
      method: "PUT", body: { name: "QualityPaymentCallbackAudit".repeat(4) },
    });
    expect(quality.agent.id).toBe(qualityAgentId);
    expect(quality.agent.name).toBe("QualityPaymentCallbackAudit".repeat(4));
    evidence.topology = resources.map(resource => ({ account_id: resource.actor.id, account_name: resource.actor.name,
      workspace_id: resource.workspace.id, agent_id: resource.agent.id, runtime_id: resource.runtime.runtimeId, installation_id: resource.runtime.installationId }));
    evidence.unrelated_account_id = X.id;
    const privateIssue = await P.client.requestJSON<{ id: string; title: string }>("/api/issues", { method: "POST", body: {
      title: `Private control order issue ${suffix}`, description: "Not shared with the collaboration participants.", status: "todo",
    } });
    const privateBefore = await P.client.requestJSON<Record<string, unknown>>(`/api/issues/${privateIssue.id}`);
    evidence.private_control_issue_id = privateIssue.id;
    const open = async (actor: ClosureActor): Promise<Page> => {
      const context = await browser.newContext(); contexts.push(context);
      await context.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
      const page = await context.newPage();
      page.on("pageerror", error => errors.push(error.message));
      await actor.authenticateWeb(page);
      await page.goto(new URL("/account", info.project.use.baseURL!).href);
      return page;
    };
    const p = await open(P), d = await open(D), q = await open(Q);
    await openNewCollaborationIssue(p);
    const creation = p.getByRole("dialog", { name: "New collaboration issue", exact: true });
    await creation.getByRole("combobox", { name: "Source workspace", exact: true }).selectOption(product.workspace.id);
    const title = `Order duplicate-charge collaboration ${suffix}`;
    await creation.getByLabel("Title", { exact: true }).fill(title);
    await creation.getByLabel("Description", { exact: true }).fill("Synthetic acceptance: one order is charged at most once. Preserve duplicate-callback evidence and handoff notes. This fixture tests orchestration, not a real payment service.");
    await creation.getByLabel("AI participation", { exact: true }).selectOption("assisted");
    await selectCollaborationRecipient(p, creation, D);
    await creation.getByLabel("Attach files", { exact: true }).setInputFiles({ name: "synthetic-order-rules.txt", mimeType: "text/plain",
      buffer: Buffer.from("Synthetic order ORDER-1. Repeated callback must not duplicate the charge.\n") });
    await expect(creation.getByRole("checkbox", { name: "Include synthetic-order-rules.txt", exact: true })).toBeChecked();
    const createdPending = p.waitForResponse(response => new URL(response.url()).pathname === "/api/collaboration/issues" && response.request().method() === "POST");
    await creation.getByRole("button", { name: "Create collaboration issue", exact: true }).click();
    const createdResponse = await createdPending;
    expect(createdResponse.status()).toBe(201);
    const created: CollaborationIssue = await createdResponse.json();
    const id = created.id, root = issueRoot(id);
    evidence.issue_id = id;
    const read = (actor: ClosureActor = P) => readClosureIssue(actor, id);
    const show = async (page: Page) => {
      await page.goto(new URL(`/account?view=participating&issue=${id}`, info.project.use.baseURL!).href);
      await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible();
    };
    await show(p);
    const invitation = await simpleWorkflow(p, id, "invite", Q);
    await show(d); await show(q);
    expect((await read()).handler.user_id).toBe(D.id);
    const recoveredCreation = await P.client.requestJSON<CollaborationIssue>(`/api/collaboration/creations/${createdResponse.request().headers()["idempotency-key"]}?source_workspace_id=${product.workspace.id}`);
    expect(recoveredCreation.id).toBe(id);
    const replayInvitation = await rawClosureRequest(env.apiUrl, P, `${root}/participants`, "POST", invitation.input, invitation.request_id);
    expect(replayInvitation.status).toBe(200);
    expect(replayInvitation.body).toEqual(invitation.receipt);
    for (const resource of resources) expect((await resource.actor.client.getWorkspaces()).map(workspace => workspace.id)).toEqual([resource.workspace.id]);
    const denied = await rawClosureRequest(env.apiUrl, X, versioned(root));
    expect(denied.status).toBe(404);
    expect(JSON.stringify(denied.body)).not.toContain(title);
    for (const actor of [D, Q]) {
      const privateCatalog = await rawClosureRequest(env.apiUrl, actor, `${root}/resources/options?execution_workspace_id=${product.workspace.id}`);
      expect(privateCatalog.status).toBe(404);
      const privateRead = await fetch(`${env.apiUrl}/api/issues/${privateIssue.id}`, { headers: {
        Authorization: `Bearer ${actor.client.getToken()!}`, "X-Workspace-ID": product.workspace.id,
      } });
      expect(privateRead.status).toBe(404);
      const deniedBody: unknown = await privateRead.json();
      expect(deniedBody).toMatchObject({ code: "workspace_access_denied" });
      expect(JSON.stringify(deniedBody)).not.toContain(privateIssue.title);
    }
    const attachments = await P.client.requestJSON<{ attachments: { id: string }[] }>(`${root}/attachments`);
    expect(attachments.attachments).toHaveLength(1);
    expect((await rawClosureRequest(env.apiUrl, X, `${root}/attachments/${attachments.attachments[0]!.id}`)).status).toBe(404);
    record("created and invited; unrelated access denied; source membership unchanged; original receipts recovered", { id, invitation: invitation.receipt, attachment_ids: attachments.attachments.map(item => item.id) });

    const targets: Awaited<ReturnType<typeof offerClosureResource>>[] = [];
    for (const resource of resources) targets.push(await offerClosureResource(resource, P, await read()));
    expect((await P.client.requestJSON<CollaborationMentionTargets>(versioned(`${root}/mention-targets`))).targets).toEqual([]);
    expect(resources.flatMap(resource => resource.daemon.visits)).toHaveLength(0);
    // P is the manager but cannot sign D's resource grant.
    const forged = await rawClosureRequest(env.apiUrl, P, `${root}/targets/${targets[1]!.hostIssue.targetId}/consents`, "POST", {
      ...targets[1]!.consentBody, ...hostAuthority(await read(P)),
    });
    expect(forged.status).toBe(403);
    record("unconsented targets unavailable and proxy owner consent denied", { proxy_status: forged.status });
    const authorize = async (resource: ClosureResource, page: Page, target: typeof targets[number]) => {
      await show(page);
      await openHostConsent(page, target.hostIssue);
      const pending = page.waitForResponse(response => new URL(response.url()).pathname === `${root}/targets/${target.hostIssue.targetId}/consents` && response.request().method() === "POST");
      await hostPanel(page).getByRole("button", { name: "Authorize for this issue", exact: true }).click();
      const response = await pending;
      expect(response.status()).toBe(201);
      expect(await response.json()).toMatchObject({ code: "resource_consented", target_id: target.hostIssue.targetId });
      expect((await resource.actor.client.requestJSON<CollaborationResources>(versioned(`${root}/resources`))).targets.find(item => item.target_id === target.hostIssue.targetId)?.authorized).toBe(true);
    };
    await authorize(product, p, targets[0]!);
    await authorize(development, d, targets[1]!);
    await authorize(quality, q, targets[2]!);
    expect((await read()).session.queue_state).toBe("paused");
    expect(resources.flatMap(resource => resource.daemon.visits)).toHaveLength(0);
    await show(p);
    await hostControl(p, targets[0]!.hostIssue, "resume", "issue");
    record("all three actual owners authorized; explicit Resume enabled execution", { target_ids: targets.map(item => item.hostIssue.targetId) });

    const ordinary = await commentThroughUI(p, id, "Ordinary human note: this is deterministic synthetic QA.");
    expect(ordinary.receipt.targets).toEqual([]);
    expect((await read()).execution!.commands).toHaveLength(0);
    await p.getByRole("group", { name: "Issue resources", exact: true }).getByRole("button", { name: "Shared context", exact: true }).click();
    await saveMemoryInUI(p, "Goal: prevent duplicate charges for synthetic ORDER-1. Acceptance needs callback replay evidence.", "0");
    await p.keyboard.press("Escape");
    const staleWrite = await rawClosureRequest(env.apiUrl, D, `${root}/memory`, "PUT", { expected_version: "0", content: "Must not replace the creator's latest context." });
    expect(staleWrite.status).toBe(409);
    record("plain comment created no execution and stale memory update rejected", { comment_id: ordinary.receipt.comment_id, stale_memory_status: staleWrite.status });

    const runs: { run_id: string; comment_id: string; command_id: string; owner_id: string; agent_id: string; agent_name: string; base_version: string; written_version: string; result: string }[] = [];
    const commentAuthorHeader = (comment: Locator) => comment.locator("xpath=./div/div[time]");
    const checkAgentAuthor = async (page: Page, commentId: string, name: string, reply = false) => {
      const comment = page.locator(`[id="comment-${commentId}"]`);
      const header = commentAuthorHeader(comment);
      await expect(header.getByText(name, { exact: true })).toBeVisible();
      await expect(header.getByText("Agent", { exact: true })).toBeVisible();
      await expectCommentMenuWithout(comment, ["Edit comment", "Delete comment"]);
      if (reply) {
        await comment.getByRole("button", { name: "Reply", exact: true }).click();
        const composer = page.getByRole("form", { name: "Comments", exact: true });
        const hint = composer.getByText(`Replying to ${name}`, { exact: true });
        await expect(hint).toBeVisible();
        await expect(hint.locator("..").getByText("Agent", { exact: true })).toBeVisible();
        await composer.getByRole("button", { name: "Cancel reply", exact: true }).click();
        await expect(hint).toHaveCount(0);
      }
    };
    const checkWrappedAuthor = async (page: Page, capture: (stage: string) => Promise<void>) => {
      const measure = async (text: Locator) => {
        await text.scrollIntoViewIfNeeded();
        await expect(text).toBeVisible();
        const geometry = await text.evaluate(element => {
          const range = document.createRange(); range.selectNodeContents(element);
          const boxes = [...range.getClientRects()].filter(box => box.height > 0);
          const boundary = element.closest('li[id^="comment-"], form')!;
          const bounds = boundary.getBoundingClientRect();
          return { lines: new Set(boxes.map(box => Math.round(box.top))).size,
            text_left: Math.min(...boxes.map(box => box.left)), text_right: Math.max(...boxes.map(box => box.right)),
            boundary_left: bounds.left, boundary_right: bounds.right, boundary_width: boundary.clientWidth,
            boundary_scroll_width: boundary.scrollWidth, viewport_width: innerWidth, document_width: document.documentElement.scrollWidth };
        });
        expect(geometry.lines).toBeGreaterThan(1);
        expect(geometry.text_left).toBeGreaterThanOrEqual(geometry.boundary_left - 1);
        expect(geometry.text_right).toBeLessThanOrEqual(geometry.boundary_right + 1);
        expect(geometry.boundary_scroll_width).toBeLessThanOrEqual(geometry.boundary_width + 1);
        expect(geometry.document_width).toBeLessThanOrEqual(geometry.viewport_width + 1);
        return geometry;
      };
      const comment = page.locator(`[id="comment-${runs[4]!.comment_id}"]`);
      const author = await measure(commentAuthorHeader(comment).getByText(quality.agent.name, { exact: true }));
      await capture("author");
      await comment.getByRole("button", { name: "Reply", exact: true }).click();
      const composer = page.getByRole("form", { name: "Comments", exact: true });
      try {
        const reply = await measure(composer.getByText(`Replying to ${quality.agent.name}`, { exact: true }));
        await capture("reply");
        return { agent_id: quality.agent.id, name_length: quality.agent.name.length, comment_id: runs[4]!.comment_id, author, reply };
      } finally { await composer.getByRole("button", { name: "Cancel reply", exact: true }).click(); }
    };
    const execute = async (resource: ClosureResource, target: typeof targets[number], page: Page, sender: ClosureActor, instruction: string) => {
      await show(page);
      const before = await P.client.requestJSON<CollaborationMemory>(`${root}/memory`);
      const index = resource.daemon.visits.length + 1;
      const originalPost = runs.length === 0 ? page.waitForResponse(response => new URL(response.url()).pathname === `${root}/comments`
        && response.request().method() === "POST") : null;
      const command = await sendHostComment(page, target.hostIssue, instruction, sender.id);
      if (originalPost) {
        const originalResponse = await originalPost;
        const originalInput: unknown = originalResponse.request().postDataJSON();
        const replay = await rawClosureRequest(env.apiUrl, sender, `${root}/comments`, "POST", originalInput, command.requestId);
        expect(replay.status).toBe(201);
        expect(replay.body).toEqual(command.receipt);
        const commands = (await read()).execution!.commands;
        expect(commands.filter(item => item.id === command.commandId)).toHaveLength(1);
        record("same-key AI comment replay retained the original command and receipt", { request_id: command.requestId, command_id: command.commandId, replay_status: replay.status });
      }
      const visit = await resource.daemon.waitForVisit(index) as ClosureVisit;
      expect(visit).toMatchObject({ agent_id: resource.agent.id, agent_owner_id: resource.actor.id, resource_owner_id: resource.actor.id, instruction_files_clean: true,
        identity: { issue_id: id, target_id: target.hostIssue.targetId, installation_id: resource.runtime.installationId },
        memory_input: { version: Number(before.version), content: before.content } });
      expect(visit.inputs.map(input => [input.command_id, input.author_user_id])).toEqual([[command.commandId, sender.id]]);
      expect(visit.memory_written).toMatchObject({ code: "memory_updated", run_id: visit.identity.run_id, version: String(BigInt(before.version) + 1n) });
      expect(visit.result_comment_id).toBe(visit.identity.run_id);
      resource.daemon.release(visit);
      await expect.poll(async () => (await read()).execution!.runs.find(run => run.id === visit.identity.run_id), { timeout: 40_000 })
        .toMatchObject({ progress_state: "settled", proof_state: "host_exit_observed", launch_count: "1", business_outcome: "success" });
      const after = await P.client.requestJSON<CollaborationMemory>(`${root}/memory`);
      expect(after.content).toContain(before.content);
      expect(after.content).toContain(instruction);
      expect(after.version).toBe(visit.memory_written.version);
      expect(after.author).toMatchObject({ kind: "agent", agent: { agent_id: resource.agent.id, run_id: visit.identity.run_id },
        resource_owner: { user_id: resource.actor.id } });
      const comments = await P.client.requestJSON<CollaborationComments>(`${root}/comments`);
      // Human author stays null; the minimal Agent summary must agree with
      // the trusted run identity and its separate execution records.
      const resultComment = comments.items.find(item => item.id === visit.result_comment_id);
      expect(resultComment).toMatchObject({ id: visit.result_comment_id,
        issue_id: id, author_type: "agent", author: null, content: visit.result_text });
      expect(resultComment?.author_agent).toEqual({ agent_id: resource.agent.id, name: resource.agent.name });
      const records = await P.client.requestJSON<CollaborationRunRecords>(versioned(`${root}/runs/${visit.identity.run_id}/records?limit=100`));
      expect(records.resource).toMatchObject({ agent_id: resource.agent.id, runtime_id: resource.runtime.runtimeId, resource_owner_user_id: resource.actor.id, execution_workspace_id: resource.workspace.id });
      expect(records.run).toMatchObject({ id: visit.identity.run_id, agent_id: resource.agent.id, proof_state: "host_exit_observed", launch_count: "1" });
      expect(records.items.filter(item => item.kind === "comment"))
        .toEqual([expect.objectContaining({ kind: "comment", comment_id: visit.result_comment_id, body: visit.result_text })]);
      expect((await read()).status_category).not.toBe("done");
      const fact = { run_id: visit.identity.run_id!, comment_id: visit.result_comment_id, command_id: command.commandId,
        owner_id: resource.actor.id, agent_id: resource.agent.id, agent_name: resource.agent.name,
        base_version: before.version, written_version: after.version, result: visit.result_text };
      runs.push(fact); record("actual Agent launch, CLI context/comment write and observed termination", fact);
      await show(page);
      await expect(page.locator(`[id="comment-${visit.result_comment_id}"]`)).toContainText(instruction);
      await checkAgentAuthor(page, visit.result_comment_id, resource.agent.name);
      await page.screenshot({ path: info.outputPath(`run-${runs.length}-${resource.agent.id}.png`), fullPage: true });
    };
    await execute(product, targets[0]!, p, P, "Requirements v1: repeated callbacks must reuse one charge; preserve the callback ID.");

    // Withdraw a settled resource, re-consent, and prove this does not imply Resume.
    await show(p);
    const manage = hostPanel(p).getByRole("button", { name: "Manage AI resources", exact: true });
    if (await manage.getAttribute("aria-expanded") !== "true") await manage.click();
    const revokePending = p.waitForResponse(response => response.url().includes("/grants/") && response.url().endsWith("/revoke") && response.request().method() === "POST");
    await p.getByRole("button", { name: `Withdraw authorization for ${product.agent.name}`, exact: true }).click();
    await p.getByRole("button", { name: "Withdraw authorization", exact: true }).click();
    const revoke = await revokePending;
    expect(revoke.status()).toBe(200);
    expect((await P.client.requestJSON<CollaborationMentionTargets>(versioned(`${root}/mention-targets`))).targets.map(item => item.target_id)).not.toContain(targets[0]!.hostIssue.targetId);
    await authorize(product, p, targets[0]!);
    const reconsented = await read();
    expect(reconsented.execution!.targets.find(item => item.id === targets[0]!.hostIssue.targetId)?.hold_reason).not.toBeNull();
    expect(product.daemon.visits).toHaveLength(1);
    await show(p);
    await hostControl(p, targets[0]!.hostIssue, "resume", "issue");
    record("owner withdrawal blocked eligibility; re-consent retained hold until explicit Resume", { target_id: targets[0]!.hostIssue.targetId });

    const requested = await directedFormThroughUI(d, id, "request_information", "Does a retry retain the first callback's charge ID?", P);
    expect(requested.receipt.code).toBe("information_requested");
    expect((await read()).handler.user_id).toBe(P.id);
    const returned = await directedFormThroughUI(p, id, "return_information", "Yes. Duplicate retries retain the original charge ID and must be logged.");
    expect(returned.receipt.code).toBe("information_returned");
    expect((await read()).handler.user_id).toBe(D.id);
    record("information requested and returned to original handler", { requested: requested.receipt, returned: returned.receipt });
    await execute(development, targets[1]!, d, D, "Implementation v1: deduplicate by order ID; callback audit evidence is still missing.");
    await simpleWorkflow(d, id, "transfer", Q, "Validate v1 and submit findings for product review.");
    expect((await read()).handler.user_id).toBe(Q.id);
    await execute(quality, targets[2]!, q, Q, "QA v1: duplicate charge prevention passes, but callback audit evidence is missing; revision required.");
    const submitted1 = await directedFormThroughUI(q, id, "submit_acceptance", "Version 1 implemented; QA found missing callback audit evidence.", undefined,
      `Synthetic verification run ${runs[2]!.run_id}; first three real simulated Agent runs and shared context are retained.`);
    expect(submitted1.receipt.code).toBe("acceptance_submitted");
    expect((await read()).status_category).toBe("in_review");
    const revision = await decideClosureResult(p, id, D, "Add callback audit evidence and repeat QA before final delivery.");
    expect((await read()).handler.user_id).toBe(D.id);
    record("first delivery returned to development by initiator", { first_submission_id: revision.submission_id, revision: revision.receipt });
    await p.screenshot({ path: info.outputPath("first-delivery-returned.png"), fullPage: true });
    await execute(development, targets[1]!, d, D, "Implementation v2: preserve callback ID and charge ID in audit evidence; duplicate replay remains idempotent.");
    // D can invoke Q's consented resource without transferring ownership or responsibility.
    await execute(quality, targets[2]!, d, D, "QA v2: duplicate replay and callback audit evidence both pass; ready for product acceptance.");
    // Read the same five results in every human session before completion,
    // while Reply remains available. These checks create no new comments.
    for (const [actor, page] of [[P, p], [D, d], [Q, q]] as const) {
      await show(page);
      const comments = await actor.client.requestJSON<CollaborationComments>(`${root}/comments`);
      expect(comments.items.filter(item => item.author_type === "agent").map(item => item.id).sort())
        .toEqual(runs.map(run => run.comment_id).sort());
      for (const run of runs) {
        expect(comments.items.find(item => item.id === run.comment_id)?.author_agent)
          .toEqual({ agent_id: run.agent_id, name: run.agent_name });
        await checkAgentAuthor(page, run.comment_id, run.agent_name, true);
      }
      const human = commentAuthorHeader(page.locator(`[id="comment-${ordinary.receipt.comment_id}"]`));
      await expect(human.getByText(P.name, { exact: true })).toBeVisible();
      await expect(human.getByText("Agent", { exact: true })).toHaveCount(0);
      record("Web participant saw all Agent authors and matching reply hints", { account_id: actor.id, comment_ids: runs.map(run => run.comment_id) });
    }
    const webViewport = q.viewportSize()!;
    try {
      await q.setViewportSize({ width: 390, height: 1100 });
      record("Narrow Web long Agent author and reply wrap without horizontal overflow", await checkWrappedAuthor(q,
        async stage => { await q.screenshot({ path: info.outputPath(`web-narrow-agent-${stage}.png`), fullPage: true }); }));
    } finally { await q.setViewportSize(webViewport); }
    nativeProfile = mkdtempSync(join(tmpdir(), "multica-closure-author-"));
    native = await openDesktop(nativeProfile, { token: Q.client.getToken()!, deepLink: `multica://account?view=participating&issue=${id}` });
    await native.start();
    await expect(native.page.getByRole("heading", { name: title, exact: true })).toBeVisible();
    for (const run of runs) await checkAgentAuthor(native.page, run.comment_id, run.agent_name, true);
    const nativeHuman = commentAuthorHeader(native.page.locator(`[id="comment-${ordinary.receipt.comment_id}"]`));
    await expect(nativeHuman.getByText(P.name, { exact: true })).toBeVisible();
    await expect(nativeHuman.getByText("Agent", { exact: true })).toHaveCount(0);
    expect(native.accountHeaders.length).toBeGreaterThan(0);
    expect(native.accountHeaders.every(headers => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
    expect(await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
    expect(native.errors).toEqual([]);
    const nativeWindow = await native.app.browserWindow(native.page);
    const originalWindow = await nativeWindow.evaluate(window => ({ size: window.getContentSize(), zoom: window.webContents.getZoomFactor() }));
    try {
      await nativeWindow.evaluate(window => { window.setContentSize(900, 1000); window.webContents.setZoomFactor(2); });
      await expect.poll(() => native!.page.evaluate(() => innerWidth)).toBe(450);
      record("Electron at 200 percent long Agent author and reply wrap without horizontal overflow", await checkWrappedAuthor(native.page,
        async stage => { await nativeScreenshot(native!.app, native!.page, info.outputPath(`desktop-agent-${stage}.png`)); }));
    } finally {
      try { await nativeWindow.evaluate((window, original) => { window.webContents.setZoomFactor(original.zoom); window.setContentSize(original.size[0]!, original.size[1]!); }, originalWindow); }
      finally { await nativeWindow.dispose(); }
    }
    record("Electron renderer read the same issue with Agent and human authors and matching reply hints", {
      account_id: Q.id, issue_id: id, comment_ids: runs.map(run => run.comment_id), account_requests: native.accountHeaders.length,
      workspace_headers_present: false, daemon_starts: 0, installed_full_main: false,
    });
    const submitted2 = await directedFormThroughUI(d, id, "submit_acceptance", "Version 2 complete: duplicate prevention and callback audit evidence pass synthetic QA.", undefined,
      `Verification ${runs[4]!.run_id}; revised implementation ${runs[3]!.run_id}; prior rejection retained.`);
    expect(submitted2.receipt.code).toBe("acceptance_submitted");
    expect((await read()).status_category).toBe("in_review");
    expect((await read(D)).capabilities.complete).toBe(false);
    const completion = await decideClosureResult(p, id);
    expect(completion.submission_id).not.toBe(revision.submission_id);
    const final = await read();
    expect(final).toMatchObject({ id, source_workspace: { id: product.workspace.id }, creator: { user_id: P.id }, status_category: "done" });
    expect(final.execution!.runs).toHaveLength(5);
    const history = await P.client.requestJSON<CollaborationDeliveryHistory>(`${root}/acceptance-submissions?limit=100`);
    expect(history.items).toHaveLength(2);
    expect(history.items.find(item => item.id === revision.submission_id)).toMatchObject({ author_user_id: Q.id, decision: { kind: "returned", actor_user_id: P.id, target_user_id: D.id } });
    expect(history.items.find(item => item.id === completion.submission_id)).toMatchObject({ author_user_id: D.id, decision: { kind: "accepted", actor_user_id: P.id } });
    for (const [actor, page] of [[P, p], [D, d], [Q, q]] as const) {
      await show(page);
      expect((await read(actor)).status_category).toBe("done");
      expect((await read(actor)).capabilities.submit_command).toBe(false);
      await expect(page.getByText("Version 2 complete: duplicate prevention and callback audit evidence pass synthetic QA.", { exact: true }).first()).toBeVisible();
      await page.screenshot({ path: info.outputPath(`completed-${actor.id}.png`), fullPage: true });
    }
    const recovered = await P.client.requestJSON(`${root}/operations/${completion.request_id}`);
    expect(recovered).toMatchObject({ code: "completed" });
    expect(resources.map(resource => resource.daemon.visits.length)).toEqual([1, 2, 2]);
    expect(new Set(resources.map(resource => resource.daemon.visits[0]!.workdir_hash)).size).toBe(3);
    expect(new Set(resources.map(resource => resource.daemon.visits[0]!.session_hash)).size).toBe(3);
    expect(await P.client.requestJSON(`/api/issues/${privateIssue.id}`)).toEqual(privateBefore);
    record("unshared control issue remained byte-for-byte unchanged", { issue_id: privateIssue.id });
    expect(errors).toEqual([]);
    evidence.runs = runs; evidence.delivery_history = history; evidence.final_status = final.status; evidence.final_status_category = final.status_category;
    record("initiator accepted the exact second result; all three Web sessions agree", { completion: completion.receipt, current_issue_id: final.id, run_count: 5 });
    passed = true;
  } catch (error) {
    evidence.failure = error instanceof Error ? error.message : "Unknown test failure";
    // These manually created contexts close in finally, before Playwright's
    // automatic failure capture. Preserve each visible actor before cleanup.
    for (const [index, context] of contexts.entries()) {
      for (const [pageIndex, page] of context.pages().entries()) {
        try { await page.screenshot({ path: info.outputPath(`failure-actor-${index + 1}-page-${pageIndex + 1}.png`), fullPage: true, timeout: 5000 }); }
        catch { /* A closed/crashed page cannot provide visual evidence. */ }
      }
    }
    if (native) {
      try { await nativeScreenshot(native.app, native.page, info.outputPath("failure-desktop-authors.png")); }
      catch { /* A closed native compositor cannot provide a screenshot. */ }
    }
    throw error;
  } finally {
    evidence.page_errors = errors;
    evidence.provider_observations = resources.map(resource => ({ owner_id: resource.actor.id, runtime_id: resource.runtime.runtimeId, visits: resource.daemon.evidence() }));
    const cleanup: unknown[] = [];
    const cleanupFailures: string[] = [];
    for (const resource of [...resources].reverse()) {
      try {
        const summary = await resource.daemon.close();
        cleanup.push({ owner_id: resource.actor.id, summary });
        if (passed) {
          expect(summary.exit_code).toBe(0);
          expect(summary.errors).toEqual([]);
          expect(summary.ledger).toHaveLength(resource.daemon.visits.length);
          for (const visit of resource.daemon.visits) {
            expect(summary.ledger.find(item => item.run_id === visit.identity.run_id))
              .toMatchObject({ phase: "proof_acknowledged", sealed: true, proof_state: "host_exit_observed", namespace_present: false });
          }
        }
      } catch (error) {
        const message = error instanceof Error ? error.message : "Unknown daemon cleanup failure";
        cleanupFailures.push(message);
        cleanup.push({ owner_id: resource.actor.id, error: message });
      }
    }
    try { await restoreGate(); evidence.gate_restored = true; }
    catch (error) { evidence.gate_restored = false; cleanupFailures.push(error instanceof Error ? error.message : "Gate restoration failed"); }
    for (const context of contexts) {
      try { await context.close(); }
      catch (error) { cleanupFailures.push(error instanceof Error ? error.message : "Browser cleanup failed"); }
    }
    if (native) {
      evidence.native_page_errors = native.errors;
      try {
        expect(native.errors).toEqual([]);
        expect(await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
      } catch (error) { cleanupFailures.push(error instanceof Error ? error.message : "Native final checks failed"); }
      try { await native.app.close(); evidence.native_closed = true; }
      catch (error) { cleanupFailures.push(error instanceof Error ? error.message : "Native cleanup failed"); }
    }
    if (nativeProfile) {
      try { rmSync(nativeProfile, { recursive: true, force: true }); evidence.native_profile_removed = true; }
      catch (error) { cleanupFailures.push(error instanceof Error ? error.message : "Native profile cleanup failed"); }
    }
    evidence.cleanup = cleanup;
    evidence.cleanup_failures = cleanupFailures;
    evidence.result = passed && cleanupFailures.length === 0 ? "passed" : "failed";
    const encoded = JSON.stringify(evidence, null, 2);
    expect(encoded).not.toMatch(/\b(?:mcb_|mct_|mdt_|mat_)[A-Za-z0-9_.-]+/);
    const path = info.outputPath("multi-account-closure-evidence.json");
    writeFileSync(path, encoded, { mode: 0o600 });
    await info.attach("multi-account-closure-evidence", { path, contentType: "application/json" });
    expect(cleanupFailures, "Owned runtime cleanup and final proof acknowledgement").toEqual([]);
  }
});
