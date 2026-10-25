import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test } from "@playwright/test";
import { captureNativeSection, nativeScreenshot, openDesktop, type NativeHarness } from "./fixtures/collaboration-native";
import { observeCrossClientWrites } from "./fixtures/collaboration-cross-client-execution";
import {
  assertHostExecutionAvailable, assertHostVisit, authorizeHostIssue, captureHostWebSection, hostAccountPath, hostControl, hostMode, hostPanel, hostRuntimeConfigured, inspectHostRecords,
  openHostConsent, readHostHistory, readHostIssue, seedHostRuntime, sendHostComment, waitHostSettlement,
} from "./fixtures/collaboration-host-runtime";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "off" });

test("host Web and actual Electron share one ordinary Agent, preserve authors, and explicitly resume after scoped Stop", async ({ page }, testInfo) => {
  test.setTimeout(300_000);
  test.skip(!hostRuntimeConfigured(), "Requires explicit task-owned host daemon/provider/CLI executables; configured runs fail if any environment input is missing");
  const webUrl = testInfo.project.use.baseURL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(webUrl).hostname);
  const fixture = await seedHostRuntime();
  const profile = mkdtempSync(join(tmpdir(), "multica-host-electron-"));
  const webErrors: string[] = [];
  page.on("pageerror", (error) => webErrors.push(error.message));
  const webRequests = observeCrossClientWrites(page, fixture.main.id, "web", fixture.webActor.id);
  let desktopRequests: ReturnType<typeof observeCrossClientWrites> | undefined;
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  let daemonClosed = false;
  const evidence: Record<string, unknown> = {
    exercised: ["shared authorized Agent from Web and Electron", "per-input authors", "ordinary Claude Backend and native process lifecycle",
      "v3 permits and terminal receipts", "actual narrow Multica CLI context", "explicit attachment bytes", "separate issue workdir/session/context",
      "one real machine slot", "target Stop", "paused follow-ups", "explicit Resume without replaying stopped input", "v6 UI end and cleanup facts",
      "enabled v6 execution does not display unavailable or new-execution-disabled banners"],
    exclusions: ["commercial Provider accounts", "Windows/Linux native execution when running on macOS", "packaged full Electron Main", "Git preparation", "squad orchestration"],
    environment: { api: fixture.apiUrl, production_web: webUrl, electron_renderer: process.env.COLLABORATION_ELECTRON_RENDERER_URL,
      native_os: process.platform, native_arch: process.arch, provider: "test-owned Claude-protocol executable", daemon: "production collaboration poller/HostRunner in test binary" },
    readiness: fixture.daemon.ready,
    identities: { shared_issue_id: fixture.main.id, separate_issue_id: fixture.separate.id, agent_id: fixture.main.agentId,
      web_actor_id: fixture.webActor.id, electron_actor_id: fixture.desktopActor.id, runtime_id: fixture.runtime.runtimeId,
      shared_target_id: fixture.main.targetId, separate_target_id: fixture.separate.targetId, resource_workspace_id: fixture.workspace.id },
    sources: [
      ["journey", "e2e/collaboration-host-runtime.spec.ts"], ["fixture", "e2e/fixtures/collaboration-host-runtime.ts"],
      ["provider source", "e2e/fixtures/collaboration-host-provider.go"], ["daemon source", "server/internal/daemon/collaboration_host_router_test.go"],
      ["native daemon binary", process.env.COLLABORATION_HOST_DAEMON!], ["native provider binary", process.env.COLLABORATION_HOST_PROVIDER!],
      ["current Multica CLI", process.env.COLLABORATION_HOST_CLI!],
    ].map(([name, path]) => ({ name, sha256: createHash("sha256").update(readFileSync(path!)).digest("hex") })),
  };
  try {
    await fixture.webActor.authenticateWeb(page);
    await page.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
    await page.goto(new URL(hostAccountPath(fixture.main.id), webUrl).href);
    const consentPanel = await openHostConsent(page, fixture.main);
    const consentForm = consentPanel.locator("form").filter({ has: page.getByRole("heading", { name: "Allow this issue to use this agent", exact: true }) });
    evidence.web_permission_capture = await captureHostWebSection(page, consentForm, testInfo.outputPath("web-host-permissions.png"));
    await authorizeHostIssue(page, fixture, fixture.main);
    evidence.initial_resume = await hostControl(page, fixture.main, "resume", "issue");
    const webActivePanel = await assertHostExecutionAvailable(page);
    await webActivePanel.getByRole("heading", { name: "AI execution", exact: true }).scrollIntoViewIfNeeded();
    await page.screenshot({ path: testInfo.outputPath("web-host-active-banner.png"), fullPage: false });
    const initial = await sendHostComment(page, fixture.main, "Host initial instruction from Web", fixture.webActor.id);
    const first = await fixture.daemon.waitForVisit(1);
    assertHostVisit(first, fixture, fixture.main, [initial]);
    expect(first.resumed).toBe(false);

    native = await openDesktop(profile, { token: fixture.desktopActor.client.getToken()!,
      deepLink: `multica://account${hostAccountPath(fixture.main.id).slice("/account".length)}` });
    await native.start();
    await expect(native.page.getByRole("heading", { name: fixture.main.title, exact: true })).toBeVisible();
    const nativeActivePanel = await assertHostExecutionAvailable(native.page);
    evidence.active_captures = await captureNativeSection(native.app, native.page, nativeActivePanel, "host-active-banner", (name) => testInfo.outputPath(name));
    desktopRequests = observeCrossClientWrites(native.page, fixture.main.id, "electron", fixture.desktopActor.id);
    const interrupted = await sendHostComment(native.page, fixture.main, "Host instruction from Electron to stop after its real launch", fixture.desktopActor.id);
    expect(interrupted.batchId).not.toBe(initial.batchId);
    expect(fixture.daemon.visits).toHaveLength(1);
    fixture.daemon.release(first);
    await waitHostSettlement(fixture, fixture.main, first);
    const second = await fixture.daemon.waitForVisit(2);
    assertHostVisit(second, fixture, fixture.main, [interrupted]);
    expect(second.resumed).toBe(true);
    expect(second.session_hash).toBe(first.session_hash);
    expect(second.workdir_hash).toBe(first.workdir_hash);
    expect(second.temp_hash).not.toBe(first.temp_hash);

    await native.page.reload();
    await expect(native.page.getByRole("heading", { name: fixture.main.title, exact: true })).toBeVisible();
    const beforeStop = await readHostIssue(fixture);
    const stopped = await hostControl(native.page, fixture.main, "stop", "target");
    expect(stopped).toMatchObject({ code: "stop_requested", target_id: fixture.main.targetId });
    await expect(hostPanel(native.page).getByRole("status", { name: "Control result", exact: true }))
      .toContainText("Stop requested; waiting for the runtime");
    const stoppedRun = await waitHostSettlement(fixture, fixture.main, second, true);
    await expect.poll(() => second.closed_ms).not.toBeNull();
    expect(second.released_ms).toBeNull();
    expect(stoppedRun.forced_disposition).toBe("stopped");
    const afterStop = await readHostIssue(fixture);
    expect(afterStop.session).toEqual(beforeStop.session);
    expect(afterStop.execution!.channels[0]).toMatchObject({ id: fixture.main.channelId, queue_state: "paused", active_group_id: null });
    expect(afterStop.execution!.commands.find((command) => command.id === interrupted.commandId)?.state).toBe("stopped");
    evidence.scoped_stop = { receipt: stopped, run: stoppedRun, session_before: beforeStop.session, session_after: afterStop.session };

    await native.page.reload();
    await expect(native.page.getByRole("heading", { name: fixture.main.title, exact: true })).toBeVisible();
    const stoppedView = await inspectHostRecords(native.page, [first, second], [initial, interrupted], interrupted.commandId);
    evidence.stopped_captures = await captureNativeSection(native.app, native.page, stoppedView.last, "host-stopped", (name) => testInfo.outputPath(name));

    // Both clients append new inputs after the real Stop. Receipt arrival and
    // reload must not revive the interrupted command or consume these inputs.
    await page.reload();
    await expect(page.getByRole("heading", { name: fixture.main.title, exact: true })).toBeVisible();
    const nextWeb = await sendHostComment(page, fixture.main, "Host next valid input from Web after Stop", fixture.webActor.id);
    const nextElectron = await sendHostComment(native.page, fixture.main, "Host next valid input from Electron after Stop", fixture.desktopActor.id);
    expect(nextElectron.batchId).toBe(nextWeb.batchId);
    const paused = await readHostIssue(fixture);
    expect(paused.execution!.runs).toHaveLength(2);
    expect(paused.execution!.channels[0]?.queue_state).toBe("paused");
    for (const comment of [nextWeb, nextElectron]) expect(paused.execution!.commands.find((command) => command.id === comment.commandId))
      .toMatchObject({ state: "waiting", group_id: null, author_user_id: comment.actorId });

    // The same actual Agent is separately authorized on another issue. Its
    // successful execution proves target Stop did not pause the whole runtime.
    await page.goto(new URL(hostAccountPath(fixture.separate.id), webUrl).href);
    await openHostConsent(page, fixture.separate);
    await authorizeHostIssue(page, fixture, fixture.separate);
    await hostControl(page, fixture.separate, "resume", "issue");
    const separateInput = await sendHostComment(page, fixture.separate, "Host independent issue has only its own context", fixture.webActor.id);
    const third = await fixture.daemon.waitForVisit(3);
    assertHostVisit(third, fixture, fixture.separate, [separateInput]);
    expect(third.resumed).toBe(false);
    expect(third.workdir_hash).not.toBe(first.workdir_hash);
    expect(third.session_hash).not.toBe(first.session_hash);
    expect(third.brief_hash).not.toBe(first.brief_hash);
    expect(third.cli_context.description).not.toContain(fixture.main.description);
    expect((await readHostIssue(fixture)).execution).toEqual(paused.execution);
    fixture.daemon.release(third);
    await waitHostSettlement(fixture, fixture.separate, third);
    const separateHistory = await readHostHistory(fixture, fixture.separate, [third], [separateInput], null);
    evidence.separate_issue = separateHistory;
    expect((await readHostIssue(fixture)).execution).toEqual(paused.execution);

    await native.page.reload();
    await expect(native.page.getByRole("heading", { name: fixture.main.title, exact: true })).toBeVisible();
    expect(fixture.daemon.visits).toHaveLength(3);
    const resumed = await hostControl(native.page, fixture.main, "resume", "target");
    expect(resumed).toMatchObject({ code: "resumed", target_id: fixture.main.targetId });
    await expect(hostPanel(native.page).getByRole("status", { name: "Control result", exact: true }))
      .toContainText("Resumed; the next eligible batch will be processed");
    const fourth = await fixture.daemon.waitForVisit(4);
    assertHostVisit(fourth, fixture, fixture.main, [nextWeb, nextElectron]);
    expect(fourth.workdir_hash).toBe(first.workdir_hash);
    expect(fourth.inputs.map((input) => input.command_id)).not.toContain(interrupted.commandId);
    expect(new Set(fixture.daemon.visits.map((visit) => visit.temp_hash)).size).toBe(4);
    fixture.daemon.release(fourth);
    await waitHostSettlement(fixture, fixture.main, fourth);
    const mainVisits = [first, second, fourth], mainComments = [initial, interrupted, nextWeb, nextElectron];
    const final = await readHostHistory(fixture, fixture.main, mainVisits, mainComments, interrupted.commandId);
    evidence.final = final;
    evidence.explicit_resume = resumed;
    const current = await readHostIssue(fixture);
    expect(current.execution!.runs).toHaveLength(3);
    expect(current.execution!.channels[0]).toMatchObject({ queue_state: "active", active_group_id: null });
    expect((await readHostIssue(fixture, fixture.main, fixture.desktopActor)).execution).toEqual(current.execution);

    await page.goto(new URL(hostAccountPath(fixture.main.id), webUrl).href);
    await expect(page.getByRole("heading", { name: fixture.main.title, exact: true })).toBeVisible();
    const webRecords = await inspectHostRecords(page, mainVisits, mainComments, interrupted.commandId);
    evidence.web_ended_capture = await captureHostWebSection(page, webRecords.last, testInfo.outputPath("web-host-ended.png"));
    await native.page.reload();
    await expect(native.page.getByRole("heading", { name: fixture.main.title, exact: true })).toBeVisible();
    const desktopRecords = await inspectHostRecords(native.page, mainVisits, mainComments, interrupted.commandId);
    evidence.ended_captures = await captureNativeSection(native.app, native.page, desktopRecords.last, "host-ended", (name) => testInfo.outputPath(name));
    expect(fixture.daemon.visits).toHaveLength(4);
    expect(fixture.daemon.errors).toEqual([]);
    const summary = await fixture.daemon.close();
    daemonClosed = true;
    evidence.daemon = summary;
    expect(summary.exit_code).toBe(0);
    expect(summary.stderr).toBe("");
    const stats = summary.messages.find((item) => "claims" in item);
    expect(stats).toMatchObject({ claims: 4, invalid_capacity: false, available_slots: 1, active_tasks: 0 });
    if (!stats || !("http" in stats)) throw new Error("Native daemon did not publish its real transport summary");
    expect(summary.ledger).toHaveLength(4);
    for (const visit of fixture.daemon.visits) {
      const run = summary.ledger.find((item) => item.run_id === visit.identity.run_id);
      if (!run) throw new Error("The original native run ledger was not retained");
      expect(run).toMatchObject({ version: 2, phase: "proof_acknowledged", namespace_present: false, sealed: true, proof_state: "host_exit_observed",
        launches: [{ attempt_seq: "1", kind: "provider", phase: "attempt_reaped", process_observed: true, terminal_accepted: true, authorized: true,
          terminal_kind: "managed_launch_ended" }] });
      const records = [...final.records, ...separateHistory.records].find((item) => item.run.id === visit.identity.run_id)!;
      expect(run.cleanup).toEqual(records.run.descendant_cleanup);
      const facts = stats.http.filter((fact) => fact.run_id === visit.identity.run_id);
      expect([...new Set(facts.map((fact) => fact.operation))]).toEqual(["claim", "permit", "termination", "proof"]);
      const permits = facts.filter((fact) => fact.operation === "permit");
      expect(permits.length).toBeGreaterThan(0);
      expect(permits.every((fact) => fact.attempt_seq === "1")).toBe(true);
      // Preparation and BeforeLaunch may replay the original permit. They
      // must retain one launch identity and yield only one physical start.
      expect(new Set(facts.filter((fact) => fact.launch_hash).map((fact) => fact.launch_hash)).size).toBe(1);
      expect(facts.every((fact) => fact.protocol_version === 3 && fact.containment === hostMode && fact.status === 200)).toBe(true);
      expect(facts.find((fact) => fact.operation === "termination")).toMatchObject({ kind: "managed_launch_ended", attempt_seq: "1" });
      expect(facts.find((fact) => fact.operation === "proof")).toMatchObject({ state: "host_exit_observed" });
    }
    expect(webErrors).toEqual([]);
    expect(native.errors).toEqual([]);
    expect(native.accountHeaders.length).toBeGreaterThan(0);
    expect(native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
    expect(await fixture.webActor.client.countIssueDispatches(fixture.main.id)).toBe(0);
    expect(await fixture.webActor.client.countIssueDispatches(fixture.separate.id)).toBe(0);
    expect(await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
    const writes = [...webRequests.writes, ...desktopRequests.writes];
    const sends = writes.filter((write) => write.path.endsWith("/comments"));
    expect(sends).toHaveLength(4);
    expect(sends.filter((write) => write.client === "web")).toHaveLength(2);
    expect(sends.filter((write) => write.client === "electron")).toHaveLength(2);
    expect(new Set(sends.map((write) => write.request_id)).size).toBe(4);
  } catch (error) {
    if (native && !native.page.isClosed()) await nativeScreenshot(native.app, native.page, testInfo.outputPath("electron-host-failure.png")).catch(() => undefined);
    throw error;
  } finally {
    evidence.provider_observations = fixture.daemon.evidence();
    if (!daemonClosed) evidence.daemon = await fixture.daemon.close();
    evidence.web_errors = webErrors;
    evidence.electron_errors = native?.errors ?? [];
    evidence.client_writes = [...webRequests.writes, ...(desktopRequests?.writes ?? [])];
    const encoded = JSON.stringify(evidence, null, 2);
    expect(encoded).not.toMatch(/\b(?:mcb_|mct_|mdt_|mat_)[A-Za-z0-9_.-]+/);
    const evidencePath = testInfo.outputPath("host-runtime-evidence.json");
    writeFileSync(evidencePath, encoded, { mode: 0o600 });
    await testInfo.attach("host-runtime-evidence", { contentType: "application/json", path: evidencePath });
    webRequests.dispose();
    desktopRequests?.dispose();
    await native?.app.close().catch(() => undefined);
    rmSync(profile, { recursive: true, force: true });
  }
});
