import { randomUUID } from "node:crypto";
import { writeFileSync } from "node:fs";
import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import type { CollaborationIssue, CollaborationMentionTargets, CollaborationResources } from "@multica/core/collaboration";
import { hostAuthority, hostControl, hostEnvironment, hostPanel, hostRuntimeConfigured, issueRoot, openHostAcceptanceGate, openHostConsent, sendHostComment, versioned } from "./fixtures/collaboration-host-runtime";
import { closureActor, createClosureResource, decideClosureResult, offerClosureResource, rawClosureRequest, readClosureIssue, simpleWorkflow, type ClosureActor, type ClosureResource } from "./fixtures/collaboration-multi-account-closure";
import { selectCollaborationRecipient } from "./fixtures/collaboration-offline-handoff";
import { directedFormThroughUI } from "./fixtures/collaboration-handoff-chain";

test.use({ actionTimeout: 20_000, viewport: { width: 1440, height: 1100 }, screenshot: "only-on-failure", trace: "off" });

test("order export: independent owner queues merge authors and preserve AI across transfer", async ({ browser }, info) => {
  test.setTimeout(600_000);
  test.skip(!hostRuntimeConfigured(), "Requires explicit task-owned password API, production Web and host Provider binaries");
  const env = hostEnvironment();
  const config = await fetch(`${env.apiUrl}/api/config`);
  expect(await config.json()).toMatchObject({ auth_mode: "password", managed_installation_supported: true, deployment_id: env.deploymentId });
  const restoreGate = await openHostAcceptanceGate();
  const contexts: BrowserContext[] = [];
  const resources: ClosureResource[] = [];
  const errors: string[] = [];
  const steps: { name: string; at: string; facts: unknown }[] = [];
  const evidence: Record<string, unknown> = { scope: "Three owners, three private workspaces, three actual host pollers and deterministic Provider processes; one shared issue",
    exclusions: ["commercial model reasoning or credentials", "physical second machine", "OS sandbox", "product repair"], steps };
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
    evidence.topology = resources.map(resource => ({ account_id: resource.actor.id, account_name: resource.actor.name,
      workspace_id: resource.workspace.id, agent_id: resource.agent.id, runtime_id: resource.runtime.runtimeId, installation_id: resource.runtime.installationId }));
    evidence.unrelated_account_id = X.id;
    const privateIssue = await P.client.requestJSON<{ id: string; title: string }>("/api/issues", { method: "POST", body: {
      title: `Private control order issue ${suffix}`, description: "Not shared with the collaboration participants.", status: "todo",
    } });
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
    await p.getByRole("button", { name: "New collaboration issue", exact: true }).click();
    const creation = p.getByRole("dialog", { name: "New collaboration issue", exact: true });
    await creation.getByRole("combobox", { name: "Source workspace", exact: true }).selectOption(product.workspace.id);
    const title = `订单导出 Excel ${suffix}`;
    await creation.getByLabel("Title", { exact: true }).fill(title);
    await creation.getByLabel("Description", { exact: true }).fill("模拟验收：按字段清单导出 Excel，支持日期筛选，保留测试依据。本次只验证协作编排，不实现导出服务。");
    await creation.getByLabel("AI participation", { exact: true }).selectOption("assisted");
    await selectCollaborationRecipient(p, creation, D);
    await creation.getByLabel("Attach files", { exact: true }).setInputFiles({ name: "synthetic-order-rules.txt", mimeType: "text/plain",
      buffer: Buffer.from("订单导出字段：订单号、金额、日期。支持日期筛选、空结果和日期边界。\n") });
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

    const requested = await directedFormThroughUI(d, id, "request_information", "请补充导出字段和列顺序", P);
    expect((await read()).handler.user_id).toBe(P.id);
    await directedFormThroughUI(p, id, "return_information", "订单号、金额、日期；保持此列顺序");
    expect((await read()).handler.user_id).toBe(D.id);
    record("supplement returned to requesting developer", requested.receipt);
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

    const firstCommand = await sendHostComment(d, targets[1]!.hostIssue, "按字段清单实现订单导出 Excel 接口", D.id);
    const coder = await development.daemon.waitForVisit(1);
    const qaCommand = await sendHostComment(d, targets[2]!.hostIssue, "编写订单导出 Excel 测试用例", D.id);
    const qa = await quality.daemon.waitForVisit(1);
    expect(coder.closed_ms).toBeNull(); expect(qa.closed_ms).toBeNull();
    expect(coder.resource_owner_id).toBe(D.id); expect(qa.resource_owner_id).toBe(Q.id);
    expect((await read()).handler.user_id).toBe(D.id);
    const a = await sendHostComment(p, targets[1]!.hostIssue, "增加按日期筛选", P.id);
    const b = await sendHostComment(q, targets[1]!.hostIssue, "补充空结果和日期边界", Q.id);
    expect(a.batchId).toBe(b.batchId);
    expect(development.daemon.visits).toHaveLength(1);
    const activeBefore = (await read()).execution!.runs.map(r => r.id);
    await simpleWorkflow(d, id, "transfer", Q, "交给测试验证，AI 保持运行");
    expect((await read()).handler.user_id).toBe(Q.id);
    expect((await read()).execution!.runs.map(r => r.id)).toEqual(activeBefore);
    expect(coder.closed_ms).toBeNull(); expect(qa.closed_ms).toBeNull();
    record("two owners execute in parallel; two authors queue one batch; live transfer leaves execution intact", {firstCommand,qaCommand,a,b, active_run_ids: activeBefore});
    const settle = async (visit: typeof coder) => {
      await expect.poll(async () => (await read()).execution!.runs.find(r => r.id === visit.identity.run_id), {timeout:40000})
        .toMatchObject({progress_state:"settled",proof_state:"host_exit_observed",business_outcome:"success"});
    };
    development.daemon.release(coder); await settle(coder);
    const merged = await development.daemon.waitForVisit(2);
    expect(merged.inputs.map(i => [i.command_id,i.author_user_id])).toEqual([[a.commandId,P.id],[b.commandId,Q.id]]);
    expect(merged.resource_owner_id).toBe(D.id);
    development.daemon.release(merged);quality.daemon.release(qa);
    await settle(merged);await settle(qa);
    expect((await read()).handler.user_id).toBe(Q.id);
    expect((await read()).status_category).not.toBe("done");
    await show(q);
    await directedFormThroughUI(q,id,"submit_acceptance","导出编排模拟完成",undefined,"两路执行和多作者合批已核对；模拟 Provider 不代表实际 Excel 实现");
    expect((await read()).handler.user_id).toBe(P.id);
    expect((await read()).status_category).toBe("in_review");
    await show(p); await decideClosureResult(p,id);
    expect((await read()).status_category).toBe("done");
    expect((await read()).capabilities.submit_command).toBe(false);
    expect(development.daemon.visits).toHaveLength(2); expect(quality.daemon.visits).toHaveLength(1);
    await p.screenshot({path:info.outputPath("excel-completed.png"),fullPage:true});
    evidence.final_status_category="done"; evidence.run_count=3;
    evidence.exclusions=["real model reasoning", "actual Excel implementation", "physical second machine", "single-comment mixed mentions (separate test)"];
    record("merged batch executed once under developer owner; QA submitted and initiator completed",{runs:[coder.identity.run_id,qa.identity.run_id,merged.identity.run_id]});
    expect(errors).toEqual([]);passed=true;
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
