import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test, type Page } from "@playwright/test";
import type { CollaborationAttachment, CollaborationComments, CollaborationMemory, CollaborationRunRecords } from "@multica/core/collaboration";
import { assertHostVisit, authorizeHostIssue, hostAccountPath, hostControl, hostPanel, hostRuntimeConfigured, issueRoot,
  openHostConsent, readHostIssue, seedHostRuntime, sendHostComment, versioned, type HostVisit } from "./fixtures/collaboration-host-runtime";
import { rawClosureRequest } from "./fixtures/collaboration-multi-account-closure";
import { nativeScreenshot, openDesktop, type NativeHarness } from "./fixtures/collaboration-native";

interface TimeoutVisit extends HostVisit {
  result_comment_id: string;
  result_text: string;
  memory_written: { version: string; run_id: string; code: string };
}

test.use({ actionTimeout: 20_000, viewport: { width: 1440, height: 1100 }, screenshot: "only-on-failure", trace: "off" });

test("actual host timeout retains its failure and recovers committed receipts after a same-ledger restart", async ({ page }, info) => {
  test.setTimeout(240_000);
  test.skip(!hostRuntimeConfigured(), "Requires the explicit password API, production Web/Desktop and closure Provider binaries");
  const timeoutMs = 15_000;
  const fixture = await seedHostRuntime({ agentTimeoutMs: timeoutMs, holdReceiptACKs: true });
  const { daemon, main, webActor } = fixture;
  const root = issueRoot(main.id);
  const evidence: Record<string, unknown> = {
    scope: "Actual test-owned process execution timeout, retained original receipts, explicit fresh retry, Web and Electron renderer",
    execution_timeout_ms: timeoutMs, provider_callback_timeout_ms: 150_000,
    exclusions: ["real model account or reasoning", "payment or fulfillment effects", "production deployment", "installed Desktop Main",
      "Agent output attachment upload: this scenario retains one delivered input attachment"],
  };
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  let profile: string | undefined;
  let passed = false;
  let bodyFailure: unknown;
  const cleanupFailures: string[] = [];
  const errors: string[] = [];
  const originalReceiptHashes = new Map<string, string>();
  page.on("pageerror", error => errors.push(error.message));
  const records = (runId: string) => webActor.client.requestJSON<CollaborationRunRecords>(versioned(`${root}/runs/${runId}/records?limit=100`));
  const run = async (runId: string) => (await readHostIssue(fixture)).execution!.runs.find(item => item.id === runId);
  const inspectFailure = async (target: Page, visit: TimeoutVisit, recordId: string) => {
    await expect(target.getByRole("heading", { name: main.title, exact: true })).toBeVisible();
    const comment = target.locator(`[id="comment-${visit.result_comment_id}"]`);
    await expect(comment).toContainText(visit.result_text);
    await expect(comment.locator("xpath=./div[time]").getByText(main.name, { exact: true })).toBeVisible();
    const disclosure = hostPanel(target).locator(`[id="run-${visit.identity.run_id}"]`);
    const button = disclosure.getByRole("button", { name: `View records for ${visit.identity.run_id}`, exact: true });
    if (await button.count()) await button.click();
    const result = disclosure.getByRole("region", { name: "Run records", exact: true });
    await expect(result).toContainText("claude timed out after 15s");
    await expect(result).toContainText("Direct launches: 1");
    const entry = result.locator(`[data-record-id="${recordId}"]`);
    await expect(entry).toContainText("claude timed out after 15s");
    await entry.scrollIntoViewIfNeeded();
    await expect(entry).toBeVisible();
  };
  try {
    expect(fixture.main.attachment).not.toBeNull();
    await page.context().addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
    await webActor.authenticateWeb(page);
    await page.goto(hostAccountPath(main.id));
    await openHostConsent(page, main);
    await authorizeHostIssue(page, fixture, main);
    await hostControl(page, main, "resume", "issue");
    const originalPost = page.waitForResponse(response => new URL(response.url()).pathname === `${root}/comments` && response.request().method() === "POST");
    const command = await sendHostComment(page, main, "S3 controlled timeout: retain this partial CLI result and the original request.", webActor.id);
    const originalResponse = await originalPost;
    const originalInput: unknown = originalResponse.request().postDataJSON();
    const visit = await daemon.waitForVisit(1) as TimeoutVisit;
    assertHostVisit(visit, fixture, main, [command]);
    expect(visit.result_comment_id).toBe(visit.identity.run_id);
    expect(visit.memory_written).toMatchObject({ code: "memory_updated", run_id: visit.identity.run_id });
    evidence.original_request = { request_id: command.requestId, command_id: command.commandId, receipt: command.receipt };
    evidence.original_run_id = visit.identity.run_id;

    // Do not release the Provider or disconnect its callback. The actual
    // configured execution deadline must end this already observed process.
    await expect.poll(() => run(visit.identity.run_id!), { timeout: 40_000 }).toMatchObject({
      progress_state: "settled", proof_state: "host_exit_observed", business_outcome: "failure", launch_count: "1", forced_disposition: null,
    });
    await expect.poll(async () => (await readHostIssue(fixture)).execution!.commands.find(item => item.id === command.commandId)?.state)
      .toBe("failed");
    await expect.poll(() => visit.closed_ms).not.toBeNull();
    expect(visit.released_ms).toBeNull();
    expect(visit.closed_ms! - visit.started_ms).toBeLessThan(150_000);
    const originalRecords = await records(visit.identity.run_id!);
    expect(originalRecords.items.filter(item => item.kind === "result"))
      .toEqual([expect.objectContaining({ outcome: "failure", body: "claude timed out after 15s" })]);
    expect(originalRecords.items.filter(item => item.kind === "comment"))
      .toEqual([expect.objectContaining({ comment_id: visit.result_comment_id, body: visit.result_text })]);
    await expect.poll(() => daemon.ledgerEvidence().find(item => item.run_id === visit.identity.run_id)).toMatchObject({
      phase: "terminal_proof_persisted", outcome_recorded: false, outcome_rejected: false, proof_state: "host_exit_observed", sealed: true,
    });
    const pending = daemon.ledgerEvidence().find(item => item.run_id === visit.identity.run_id)!;
    expect(pending.outcome_event_id).toMatch(/^[0-9a-f-]{36}$/);
    expect(pending.proof_receipt_id).toMatch(/^[0-9a-f-]{36}$/);
    const snapshot = async () => {
      const [comments, attachments, memory, detail, record] = await Promise.all([
        webActor.client.requestJSON<CollaborationComments>(`${root}/comments?limit=100`),
        webActor.client.requestJSON<{ attachments: CollaborationAttachment[] }>(`${root}/attachments`),
        webActor.client.requestJSON<CollaborationMemory>(`${root}/memory`), readHostIssue(fixture), records(visit.identity.run_id!),
      ]);
      expect(comments.next_cursor).toBeNull();
      expect(record.next_cursor).toBeNull();
      return { comments: comments.items.map(item => ({ id: item.id, content: item.content, revision: item.comment_revision, author_type: item.author_type })),
        attachments: attachments.attachments.map(item => ({ id: item.id, version: item.version, bytes: item.size_bytes })),
        memory: { version: memory.version, content: memory.content },
        commands: detail.execution!.commands, runs: detail.execution!.runs.map(item => ({ id: item.id, command_id: item.command_id, outcome: item.business_outcome, launches: item.launch_count })),
        results: record.items.filter(item => item.kind === "input" || item.kind === "comment" || item.kind === "result") };
    };
    const beforeRestart = await snapshot();
    expect(beforeRestart.comments.filter(item => item.author_type === "agent")).toHaveLength(1);
    expect(beforeRestart.attachments).toEqual([expect.objectContaining({ id: main.attachment!.id })]);
    expect(beforeRestart.commands).toHaveLength(1);
    const stopped = await daemon.stopPreservingState();
    expect(stopped.exit_code).toBe(0);
    expect(stopped.errors).toEqual([]);
    const firstProcess = stopped.messages.find(item => "claims" in item);
    if (!firstProcess || !("http" in firstProcess)) throw new Error("The first poller did not retain its actual HTTP observations");
    const held = firstProcess.http.filter(fact => fact.run_id === visit.identity.run_id && fact.acknowledgement_held);
    expect(new Set(held.map(fact => fact.operation))).toEqual(new Set(["outcome", "proof"]));
    for (const fact of held) {
      expect(fact.status).toBeGreaterThanOrEqual(200);
      expect(fact.status).toBeLessThan(300);
      expect(fact.request_sha256).toMatch(/^[0-9a-f]{64}$/);
      expect(fact.response_sha256).toMatch(/^[0-9a-f]{64}$/);
      const identity = `${fact.operation}:${fact.event_id ?? fact.receipt_id}`;
      const previous = originalReceiptHashes.get(identity);
      if (previous) expect(fact.request_sha256).toBe(previous);
      originalReceiptHashes.set(identity, fact.request_sha256!);
    }
    evidence.post_commit_ack_barrier = { pending_ledger: pending, stopped_process: stopped };
    evidence.restart = await daemon.restart();
    await expect.poll(() => daemon.ledgerEvidence().find(item => item.run_id === visit.identity.run_id), { timeout: 30_000 })
      .toMatchObject({ phase: "proof_acknowledged", outcome_recorded: true, outcome_rejected: false,
        identity_sha256: pending.identity_sha256, outcome_event_id: pending.outcome_event_id, outcome_sha256: pending.outcome_sha256,
        proof_receipt_id: pending.proof_receipt_id, proof_sha256: pending.proof_sha256, launches: pending.launches });
    expect(await snapshot()).toEqual(beforeRestart);
    const replay = await rawClosureRequest(fixture.apiUrl, webActor, `${root}/comments`, "POST", originalInput, command.requestId);
    expect(replay).toMatchObject({ status: 201, body: command.receipt });
    expect(await snapshot()).toEqual(beforeRestart);
    expect(daemon.visits).toHaveLength(1);
    evidence.stable_business_effects = beforeRestart;

    await page.reload();
    const failureRecordId = originalRecords.items.find(item => item.kind === "result")!.id;
    await inspectFailure(page, visit, failureRecordId);
    await page.screenshot({ path: info.outputPath("web-retained-timeout.png"), fullPage: true });
    profile = mkdtempSync(join(tmpdir(), "multica-timeout-renderer-"));
    native = await openDesktop(profile, { token: fixture.desktopActor.client.getToken()!, deepLink: `multica://account?view=participating&issue=${main.id}` });
    await native.start();
    await inspectFailure(native.page, visit, failureRecordId);
    await nativeScreenshot(native.app, native.page, info.outputPath("desktop-retained-timeout.png"));
    expect(await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
    expect(native.accountHeaders.length).toBeGreaterThan(0);
    expect(native.accountHeaders.every(headers => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
    evidence.desktop = { account_id: fixture.desktopActor.id, actual_renderer: true, installed_main: false, daemon_starts: 0 };

    // A fresh human instruction is a new run. It must not replace or retry the
    // failed original automatically when its receipts become available.
    await page.goto(hostAccountPath(main.id));
    let ready = await readHostIssue(fixture);
    if (ready.session.queue_state === "paused") {
      await hostControl(page, main, "resume", "issue");
      ready = await readHostIssue(fixture);
    }
    if (ready.execution!.targets[0]!.control.queue_state === "paused") await hostControl(page, main, "resume", "target");
    const retry = await sendHostComment(page, main, "S3 explicit fresh retry: keep the previous timeout failure as history.", webActor.id);
    const retryVisit = await daemon.waitForVisit(2) as TimeoutVisit;
    assertHostVisit(retryVisit, fixture, main, [retry]);
    expect(retryVisit.identity.run_id).not.toBe(visit.identity.run_id);
    expect(retry.commandId).not.toBe(command.commandId);
    daemon.release(retryVisit);
    await expect.poll(() => run(retryVisit.identity.run_id!), { timeout: 30_000 })
      .toMatchObject({ business_outcome: "success", progress_state: "settled", proof_state: "host_exit_observed", launch_count: "1" });
    expect((await records(visit.identity.run_id!)).items).toEqual(originalRecords.items);
    expect(await run(visit.identity.run_id!)).toMatchObject({ business_outcome: "failure", proof_state: "host_exit_observed", launch_count: "1" });
    const final = await readHostIssue(fixture);
    expect(final.execution!.commands.map(item => ({ id: item.id, state: item.state })))
      .toEqual(expect.arrayContaining([{ id: command.commandId, state: "failed" }, { id: retry.commandId, state: "succeeded" }]));
    expect(final.execution!.commands).toHaveLength(2);
    expect(final.execution!.runs).toHaveLength(2);
    expect(final.status_category).not.toBe("done");
    evidence.explicit_retry = { request_id: retry.requestId, command_id: retry.commandId, run_id: retryVisit.identity.run_id, old_failure_retained: true };
    evidence.final_runs = final.execution!.runs;
    expect(errors).toEqual([]);
    expect(native.errors).toEqual([]);
    passed = true;
  } catch (error) {
    bodyFailure = error;
    evidence.failure = error instanceof Error ? error.message : "Unknown timeout acceptance failure";
    await page.screenshot({ path: info.outputPath("failure-timeout-web.png"), fullPage: true }).catch(() => undefined);
    if (native) await nativeScreenshot(native.app, native.page, info.outputPath("failure-timeout-desktop.png")).catch(() => undefined);
  } finally {
    evidence.provider_observations = daemon.evidence();
    try {
      const cleanup = await daemon.close();
      evidence.cleanup = cleanup;
      if (passed) {
        expect(cleanup.exit_code).toBe(0);
        expect(cleanup.errors).toEqual([]);
        for (const item of cleanup.ledger) expect(item).toMatchObject({ phase: "proof_acknowledged", proof_state: "host_exit_observed", outcome_recorded: true });
        const processes = cleanup.messages.filter(item => "claims" in item);
        expect(processes).toHaveLength(2);
        const recovered = processes[1]!;
        const original = recovered.http.filter(fact => fact.run_id === evidence.original_run_id);
        expect(original.some(fact => fact.operation === "claim" || fact.operation === "permit")).toBe(false);
        expect(original.filter(fact => fact.operation === "outcome" || fact.operation === "proof"))
          .toEqual(expect.arrayContaining([expect.objectContaining({ operation: "outcome" }), expect.objectContaining({ operation: "proof" })]));
        for (const fact of original) {
          expect(fact.request_sha256).toBe(originalReceiptHashes.get(`${fact.operation}:${fact.event_id ?? fact.receipt_id}`));
          expect(fact.status).toBeGreaterThanOrEqual(200);
          expect(fact.status).toBeLessThan(300);
          expect(fact.acknowledgement_held).not.toBe(true);
        }
      }
      evidence.gate_restored = true;
    } catch (error) { cleanupFailures.push(error instanceof Error ? error.message : "Daemon or gate cleanup failed"); }
    let rendererClosed = native === undefined;
    if (native) {
      try { await native.app.close(); rendererClosed = true; } catch (error) { cleanupFailures.push(error instanceof Error ? error.message : "Renderer cleanup failed"); }
    }
    if (profile && rendererClosed) {
      try { rmSync(profile, { recursive: true, force: true }); } catch (error) { cleanupFailures.push(error instanceof Error ? error.message : "Renderer profile cleanup failed"); }
    }
    evidence.page_errors = errors;
    evidence.cleanup_failures = cleanupFailures;
    evidence.result = passed && cleanupFailures.length === 0 ? "passed" : "failed";
    const output = info.outputPath("timeout-recovery-evidence.json");
    writeFileSync(output, JSON.stringify(evidence, null, 2));
    await info.attach("timeout-recovery-evidence", { path: output, contentType: "application/json" });
  }
  if (bodyFailure !== undefined) throw bodyFailure;
  expect(cleanupFailures, "Timeout acceptance cleanup must preserve its original body result").toEqual([]);
});
