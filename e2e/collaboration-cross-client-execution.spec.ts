import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, expect } from "@playwright/test";
import type { CollaborationCommandCancelReceipt } from "@multica/core/collaboration";
import { deepLink, nativeScreenshot, openDesktop, responseEvidence, type NativeHarness } from "./fixtures/collaboration-native";
import {
  assertProviderInput, crossClientAccountPath, crossClientEnvironmentReady, inspectCrossClientRecords, observeCrossClientWrites,
  readCrossClientIssue, resumeCrossClientIssue, seedCrossClientExecution, sendCrossClientComment, startCrossClientDaemons,
  verifyCrossClientFinal, waitForCrossClientSettlement, type CrossClientDaemons,
} from "./fixtures/collaboration-cross-client-execution";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });

test("AC33 Web and actual Electron collaborators execute independent A/B namespaces and one coalesced A follow-up", async ({ page }, testInfo) => {
  test.setTimeout(300_000);
  test.skip(!crossClientEnvironmentReady(), "Requires explicit task-owned password API/Web/renderer and static Linux daemon/provider executables");
  const webUrl = testInfo.project.use.baseURL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(webUrl).hostname);
  const fixture = await seedCrossClientExecution();
  const profile = mkdtempSync(join(tmpdir(), "multica-ac33-native-"));
  const webErrors: string[] = [];
  page.on("pageerror", (error) => webErrors.push(error.message));
  const webRequests = observeCrossClientWrites(page, fixture.issueId, "web", fixture.webActor.id);
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  let daemons: CrossClientDaemons | undefined;
  let desktopRequests: ReturnType<typeof observeCrossClientWrites> | undefined;
  const evidence: Record<string, unknown> = {
    criterion: "AC33", issue_id: fixture.issueId,
    web_actor_id: fixture.webActor.id, electron_actor_id: fixture.desktopActor.id,
    environment: { api: fixture.apiUrl, production_web: webUrl, electron_renderer: process.env.COLLABORATION_ELECTRON_RENDERER_URL,
      electron_os: process.platform, execution_os: "Linux arm64 Docker with actual kernel PID namespaces", provider: "explicit stdlib test-created executable" },
    source: ["e2e/fixtures/collaboration-cross-client-execution.ts", "e2e/collaboration-cross-client-execution.spec.ts",
      "e2e/fixtures/collaboration-cross-client-provider.go", process.env.COLLABORATION_CROSS_CLIENT_DAEMON!, process.env.COLLABORATION_CROSS_CLIENT_PROVIDER!]
      .map((path) => ({ path, sha256: createHash("sha256").update(readFileSync(path)).digest("hex") })),
    resources: fixture.targets.map((target) => ({ lane: target.lane, agent_id: target.resourceId, target_id: target.targetId,
      channel_id: target.channelId, owner_id: target.ownerId, runtime_id: target.runtime.runtimeId, workspace_id: target.runtime.workspaceId,
      installation_id: target.runtime.installationId, binding_id: target.runtime.credential.binding_id })),
  };
  try {
    await fixture.webActor.authenticateWeb(page);
    await page.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
    await page.goto(new URL(crossClientAccountPath(fixture.issueId), webUrl).href);
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    native = await openDesktop(profile, { token: fixture.desktopActor.client.getToken()!,
      deepLink: `multica://account${crossClientAccountPath(fixture.issueId).slice("/account".length)}` });
    await native.start();
    await expect(native.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    desktopRequests = observeCrossClientWrites(native.page, fixture.issueId, "electron", fixture.desktopActor.id);
    const initial = await readCrossClientIssue(fixture);
    expect(initial.session.queue_state).toBe("paused");
    expect(initial.execution!.commands).toEqual([]);
    expect(initial.execution!.runs).toEqual([]);
    evidence.resume = await resumeCrossClientIssue(page, fixture);
    const resumed = await readCrossClientIssue(fixture);
    expect(resumed.session.queue_state).toBe("active");
    expect(resumed.execution!.targets.every((target) => target.hold_reason === null)).toBe(true);
    // Reload the second viewer after the deliberate initial Resume. Thereafter
    // comments/cancellation go through each actual client's shared controls.
    await native.page.reload();
    await expect(native.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    daemons = await startCrossClientDaemons(fixture);
    const a1 = await sendCrossClientComment(page, fixture, fixture.targetA, "AC33 A initial held open", fixture.webActor.id, "web");
    const aFirst = await daemons.waitForVisit("A", 0);
    assertProviderInput(aFirst, fixture, fixture.targetA, [a1]);
    const b1 = await sendCrossClientComment(native.page, fixture, fixture.targetB, "AC33 B completes while A is busy", fixture.desktopActor.id, "electron");
    const bFirst = await daemons.waitForVisit("B", 0);
    assertProviderInput(bFirst, fixture, fixture.targetB, [b1]);
    expect(aFirst.released_ms).toBeNull();
    expect(aFirst.closed_ms).toBeNull();
    expect(bFirst.namespace_id).not.toBe(aFirst.namespace_id);
    expect(bFirst.started_ms).toBeGreaterThan(aFirst.started_ms);
    daemons.release(bFirst);
    await waitForCrossClientSettlement(fixture, bFirst.input.identity.run_id);
    const overlap = await readCrossClientIssue(fixture);
    expect(overlap.execution!.runs.find((run) => run.id === aFirst.input.identity.run_id))
      .toMatchObject({ proof_state: "unknown", business_outcome: null, settled_at: null });
    expect(overlap.execution!.commands.find((command) => command.id === a1.commandId)?.state).toBe("in_flight");
    expect(overlap.execution!.commands.find((command) => command.id === b1.commandId)?.state).toBe("succeeded");
    expect(aFirst.released_ms).toBeNull();
    expect(aFirst.closed_ms).toBeNull();
    evidence.overlap = { execution: overlap.execution, provider_observations: daemons.evidence() };

    // Both actual collaborators append to A while its first run is still
    // blocked. These immutable inputs must form one subsequent batch.
    const a2 = await sendCrossClientComment(native.page, fixture, fixture.targetA, "AC33 A follow-up from Electron collaborator B", fixture.desktopActor.id, "electron");
    const a3 = await sendCrossClientComment(page, fixture, fixture.targetA, "AC33 A remove only this waiting input", fixture.webActor.id, "web");
    const a4 = await sendCrossClientComment(page, fixture, fixture.targetA, "AC33 A follow-up from Web collaborator A", fixture.webActor.id, "web");
    expect(a2.batchId).not.toBe(a1.batchId);
    expect([a3.batchId, a4.batchId]).toEqual([a2.batchId, a2.batchId]);
    expect([a1.sequence, a2.sequence, a3.sequence, a4.sequence]).toEqual(["1", "2", "3", "4"]);
    const waiting = await readCrossClientIssue(fixture);
    expect(waiting.execution!.groups).toHaveLength(2);
    expect(waiting.execution!.runs).toHaveLength(2);
    for (const command of [a2, a3, a4]) {
      expect(waiting.execution!.commands.find((item) => item.id === command.commandId))
        .toMatchObject({ state: "waiting", group_id: null, batch_id: a2.batchId, channel_id: fixture.targetA.channelId, author_user_id: command.actorId });
    }
    expect(daemons.visits).toHaveLength(2);
    await native.page.reload();
    await expect(native.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    const cancelPath = `/api/collaboration/issues/${fixture.issueId}/commands/${a3.commandId}/cancel`;
    const cancelPending = native.page.waitForResponse((response) => new URL(response.url()).pathname === cancelPath && response.request().method() === "POST");
    await native.page.getByRole("region", { name: "AI execution", exact: true }).getByRole("button", { name: `Cancel waiting input ${a3.sequence}`, exact: true }).click();
    const cancelResponse = await cancelPending;
    expect(cancelResponse.status()).toBe(200);
    const cancelled: CollaborationCommandCancelReceipt = await cancelResponse.json();
    expect(cancelled).toMatchObject({ status: 200, action: "cancel_command", code: "command_cancelled", command_id: a3.commandId });
    const afterCancel = await readCrossClientIssue(fixture);
    expect(afterCancel.session).toEqual(waiting.session);
    for (const key of ["channels", "targets", "groups", "runs"] as const) expect(afterCancel.execution![key]).toEqual(waiting.execution![key]);
    expect(afterCancel.execution!.commands.filter((item) => item.id !== a3.commandId)).toEqual(waiting.execution!.commands.filter((item) => item.id !== a3.commandId));
    expect(afterCancel.execution!.commands.find((item) => item.id === a3.commandId)).toMatchObject({ state: "cancelled", group_id: null, batch_id: a2.batchId });
    evidence.single_cancellation = { receipt: cancelled, before: waiting.execution, after: afterCancel.execution };
    expect(aFirst.released_ms).toBeNull();
    daemons.release(aFirst);
    const aFollowup = await daemons.waitForVisit("A", 1);
    assertProviderInput(aFollowup, fixture, fixture.targetA, [a2, a4]);
    expect(aFollowup.input.identity.group_id).not.toBe(aFirst.input.identity.group_id);
    expect(aFollowup.input.identity.run_id).not.toBe(aFirst.input.identity.run_id);
    // Linux may reuse a destroyed namespace inode. Distinct simultaneous A/B
    // namespaces were proven above; new work is identified by its new run/group.
    expect(aFollowup.started_ms).toBeGreaterThan(aFirst.released_ms!);
    expect(bFirst.closed_ms).not.toBeNull();
    expect(bFirst.closed_ms!).toBeLessThan(aFirst.released_ms!);
    await waitForCrossClientSettlement(fixture, aFirst.input.identity.run_id);
    daemons.release(aFollowup);
    await waitForCrossClientSettlement(fixture, aFollowup.input.identity.run_id);
    const comments = [a1, b1, a2, a3, a4];
    const visits = [aFirst, bFirst, aFollowup];
    const final = await verifyCrossClientFinal(fixture, visits, comments, a3);
    evidence.final = final;
    evidence.sends = comments;
    evidence.provider_observations = daemons.evidence();

    // Each real viewer independently reloads durable, source-owned history and
    // result projections. Refreshing cannot cause another provider invocation.
    await page.reload();
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    const webPanel = await inspectCrossClientRecords(page, visits, comments, a3);
    await webPanel.screenshot({ path: testInfo.outputPath("web-cross-client-results.png"), animations: "disabled" });
    await deepLink(native.app, crossClientAccountPath(fixture.issueId));
    await native.page.reload();
    await expect(native.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    await inspectCrossClientRecords(native.page, visits, comments, a3);
    await nativeScreenshot(native.app, native.page, testInfo.outputPath("electron-cross-client-results.png"));
    expect(daemons.callbackErrors).toEqual([]);
    expect(daemons.visits.map((visit) => visit.lane)).toEqual(["A", "B", "A"]);
    expect((await verifyCrossClientFinal(fixture, visits, comments, a3)).issue.execution).toEqual(final.issue.execution);
    const stats = await daemons.close();
    daemons = undefined;
    evidence.daemon_summaries = stats;
    for (const [index, expectedClaims] of [2, 1].entries()) {
      expect(stats[index]!.exit_code).toBe(0);
      expect(stats[index]!.messages).toEqual([{ ready: true, max_concurrent_tasks: 1 },
        { claims: expectedClaims, invalid_capacity: false, available_slots: 1, active_tasks: 0 }]);
    }
    const writes = [...webRequests.writes, ...desktopRequests.writes];
    const sends = writes.filter((write) => write.path === `/api/collaboration/issues/${fixture.issueId}/comments`);
    expect(sends).toHaveLength(5);
    expect(sends.filter((write) => write.client === "web")).toHaveLength(3);
    expect(sends.filter((write) => write.client === "electron")).toHaveLength(2);
    expect(new Set(sends.map((write) => write.request_id)).size).toBe(5);
    expect(writes.filter((write) => write.path.endsWith("/control"))).toHaveLength(1);
    expect(writes.filter((write) => write.path.endsWith("/cancel"))).toHaveLength(1);
    evidence.client_writes = writes;
    expect(webErrors).toEqual([]);
    expect(native.errors).toEqual([]);
    expect(native.accountHeaders.length).toBeGreaterThan(0);
    expect(native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
    expect(await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
  } catch (error) {
    if (daemons) evidence.provider_observations = daemons.evidence();
    if (native && !native.page.isClosed()) {
      try {
        await nativeScreenshot(native.app, native.page, testInfo.outputPath("electron-cross-client-failure.png"));
        await testInfo.attach("electron-failure-responses", { contentType: "application/json", body: JSON.stringify(responseEvidence.get(native.app) ?? [], null, 2) });
        await testInfo.attach("electron-failure-page", { contentType: "text/plain", body: await native.page.locator("body").innerText() });
      } catch { /* Keep the original assertion failure if the window closed. */ }
    }
    throw error;
  } finally {
    if (daemons) evidence.daemon_summaries = await daemons.close();
    evidence.web_errors = webErrors;
    evidence.client_writes = [...webRequests.writes, ...(desktopRequests?.writes ?? [])];
    await testInfo.attach("cross-client-execution-identities", { contentType: "application/json", body: JSON.stringify(evidence, null, 2) });
    webRequests.dispose();
    desktopRequests?.dispose();
    await native?.app.close().catch(() => undefined);
    rmSync(profile, { recursive: true, force: true });
  }
});
