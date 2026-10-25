import { execFile } from "node:child_process";
import { writeFileSync } from "node:fs";
import { promisify } from "node:util";
import { test, expect } from "@playwright/test";
import type {
  CollaborationCommentInput, CollaborationCommentReceipt, CollaborationControlInput, CollaborationControlReceipt,
  CollaborationIssue, CollaborationTriggerPreview,
} from "@multica/core/collaboration";
import { dockerTaskURL, InstalledMainClient, type InstalledMainMetadata, type InstalledMainWrite } from "./fixtures/collaboration-installed-main";
import {
  assertHostVisit, authorizeHostIssue, hostAccountPath, hostControl, hostMode, openHostConsent, readHostHistory, readHostIssue,
  seedHostRuntime, sendHostComment, waitHostSettlement, type HostComment, type HostFixture, type HostIssue,
} from "./fixtures/collaboration-host-runtime";

const execute = promisify(execFile);
type InstalledComment = Omit<InstalledMainWrite<CollaborationCommentReceipt, CollaborationCommentInput>, "resultText"> & {
  content: string; preview: CollaborationTriggerPreview;
};

function hostComment(write: InstalledComment, issue: HostIssue, marker: string, actorId: string): HostComment {
  expect(write.input).toMatchObject({ execution_view_version: 6, content: write.content });
  expect(write.requestId).toMatch(/^[0-9a-f-]{36}$/);
  expect(write.receipt).toMatchObject({ status: 201, code: "comment_created", issue_id: issue.id });
  expect(write.preview.targets).toEqual([expect.objectContaining({ target_id: issue.targetId, resource_id: issue.agentId, state: "waiting" })]);
  expect(write.receipt.targets).toHaveLength(1);
  const target = write.receipt.targets[0];
  if (!target?.command_id || !target.batch_id || !target.command_sequence) throw new Error("Packaged Main did not retain the input identity");
  expect(target).toMatchObject({ state: "waiting", target_id: issue.targetId, channel_id: issue.channelId });
  return { commandId: target.command_id, batchId: target.batch_id, sequence: target.command_sequence,
    content: write.content, marker, actorId, targetId: issue.targetId, requestId: write.requestId, receipt: write.receipt };
}

test.use({ actionTimeout: 20_000, trace: "off", screenshot: "off", video: "off" });

test("Linux packaged full Main controls a real native host run and explicitly resumes the shared Agent", async ({ page }, testInfo) => {
  test.setTimeout(480_000);
  const image = process.env.COLLABORATION_HOST_INSTALLED_IMAGE;
  // This separate opt-in must be checked before package mounts, credentials,
  // API fixtures or runtime enrollment. Never pull an absent image implicitly.
  test.skip(!image, "Requires an explicitly selected local host-acceptance packaged Main image");
  const inspected = await execute("docker", ["image", "inspect", "--format", "{{.Id}}", image!], { timeout: 10_000 })
    .then(({ stdout }) => stdout.trim(), () => null);
  if (inspected === null) throw new Error("The explicitly selected packaged Main image is unavailable locally; this fixture never pulls an image");
  expect(inspected).toMatch(/^sha256:[0-9a-f]{64}$/);
  const packageHash = process.env.COLLABORATION_HOST_INSTALLED_ASAR_SHA256;
  expect(packageHash, "Pin the current task-owned app.asar").toMatch(/^[0-9a-f]{64}$/);
  expect(process.env.COLLABORATION_INSTALLED_APP_VOLUME || process.env.COLLABORATION_INSTALLED_APP_DIR).toBeTruthy();
  const apiURL = process.env.COLLABORATION_E2E_API_URL!;
  const webURL = String(testInfo.project.use.baseURL);
  const native = new InstalledMainClient({ image });
  let fixture: HostFixture | undefined;
  let daemonClosed = false;
  const evidence: Record<string, unknown> = { client: "Linux arm64 packaged production Main/preload/renderer", image_id: inspected,
    scope: "Packaged Linux client controls the separately running native host fixture; this does not establish Linux daemon execution support.",
    realProviderAccounts: false, nativeIPCStubs: false, authInjection: false, homeOverride: false };
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  try {
    const boot = await native.command<InstalledMainMetadata>("init", { apiUrl: dockerTaskURL(apiURL), appUrl: dockerTaskURL(webURL), executionViewVersion: 6 }, 180_000);
    expect(boot.main).toMatchObject({ packaged: true, name: "Multica", platform: "linux", arch: "arm64", electron: "39.8.7" });
    expect(boot.artifactSha256).toBe(packageHash);
    expect(boot.rendererUrl).toMatch(/^file:\/\/\/opt\/multica\/resources\/app\.asar\/out\/renderer\/index\.html/);
    expect(boot.bundledCLI?.version).toBeTruthy();
    evidence.boot = boot;
    fixture = await seedHostRuntime();
    const f = fixture, issue = f.main;
    evidence.runtime = f.daemon.ready;
    expect(f.daemon.ready.os).toBe(process.platform === "win32" ? "windows" : process.platform);
    evidence.identities = { issue_id: issue.id, agent_id: issue.agentId, owner_user_id: f.webActor.id,
      packaged_participant_id: f.desktopActor.id, runtime_id: f.runtime.runtimeId, target_id: issue.targetId };
    evidence.login = await f.desktopActor.authenticateInstalled((credentials) => native.command("login", credentials));
    expect(evidence.login).toMatchObject({ status: 200, userId: f.desktopActor.id, actualPasswordForm: true });
    const opened = await native.command<InstalledMainMetadata>("navigate", { path: hostAccountPath(issue.id), issueId: issue.id, title: issue.title });
    expect(opened.tabs).toMatchObject({ activeScope: "account", activeWorkspaceSlug: null });
    expect(opened.tabs?.accountTabs).toHaveLength(1);
    expect(await native.command("read", { path: "/api/workspaces" })).toEqual([]);

    await f.webActor.authenticateWeb(page);
    await page.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
    await page.goto(new URL(hostAccountPath(issue.id), webURL).href);
    await openHostConsent(page, issue);
    await authorizeHostIssue(page, f, issue);
    await hostControl(page, issue, "resume", "issue");
    const initial = await sendHostComment(page, issue, "Packaged host initial input from Web owner", f.webActor.id);
    const first = await f.daemon.waitForVisit(1);
    assertHostVisit(first, f, issue, [initial]);

    const send = async (marker: string) => hostComment(await native.command<InstalledComment>("comment", {
      issueId: issue.id, resourceId: issue.agentId, targetName: issue.name, marker,
    }), issue, marker, f.desktopActor.id);
    const interrupted = await send("Packaged participant input that will be stopped after launch");
    f.daemon.release(first);
    await waitHostSettlement(f, issue, first);
    const second = await f.daemon.waitForVisit(2);
    assertHostVisit(second, f, issue, [interrupted]);
    expect(second.session_hash).toBe(first.session_hash);
    expect(second.workdir_hash).toBe(first.workdir_hash);
    const control = (action: "stop" | "resume") => native.command<InstalledMainWrite<CollaborationControlReceipt, CollaborationControlInput>>("control", {
      issueId: issue.id, resourceId: issue.agentId, action, status: 200,
    });
    const stop = await control("stop");
    expect(stop.input).toMatchObject({ action: "stop", scope: "target", execution_view_version: 6 });
    expect(stop.receipt).toMatchObject({ code: "stop_requested", target_id: issue.targetId, queue_state: "paused" });
    await waitHostSettlement(f, issue, second, true);
    expect(second.released_ms).toBeNull();
    await expect.poll(() => second.closed_ms).not.toBeNull();
    evidence.stop = stop;

    const capture = async (name: string) => {
      const image = await native.command<{ png: string; noHorizontalOverflow: boolean; width: number; height: number }>("capture");
      expect(image.noHorizontalOverflow).toBe(true);
      const path = testInfo.outputPath(name);
      writeFileSync(path, Buffer.from(image.png, "base64"));
      await testInfo.attach(name, { contentType: "image/png", path });
      return { name, width: image.width, height: image.height, noHorizontalOverflow: image.noHorizontalOverflow };
    };
    evidence.stopped_display = await native.command("host-records", { issueId: issue.id, runId: second.identity.run_id, stopped: true, markers: [interrupted.marker] });
    evidence.stopped_capture = await capture("installed-host-stopped.png");

    await page.reload();
    await expect(page.getByRole("heading", { name: issue.title, exact: true })).toBeVisible();
    const nextWeb = await sendHostComment(page, issue, "Packaged host next input from Web after Stop", f.webActor.id);
    const nextPackage = await send("Packaged host next input from the installed participant after Stop");
    expect(nextPackage.batchId).toBe(nextWeb.batchId);
    const paused = await readHostIssue(f);
    expect(paused.execution!.channels[0]?.queue_state).toBe("paused");
    expect(paused.execution!.runs).toHaveLength(2);
    const restarted = await native.command<InstalledMainMetadata>("restart", { issueId: issue.id, title: issue.title });
    expect(restarted.artifactSha256).toBe(boot.artifactSha256);
    expect(restarted.main.userData).toBe(boot.main.userData);
    expect(restarted.native.daemon.state).toBe("stopped");
    const restored = await native.command<CollaborationIssue>("read", { path: `/api/collaboration/issues/${issue.id}` });
    expect(restored.execution?.protocol_version).toBe(6);
    expect(restored.execution).toEqual(paused.execution);
    expect(f.daemon.visits).toHaveLength(2);
    evidence.restart_while_paused = restarted;

    const resume = await control("resume");
    expect(resume.input).toMatchObject({ action: "resume", scope: "target", execution_view_version: 6 });
    expect(resume.receipt).toMatchObject({ code: "resumed", target_id: issue.targetId, queue_state: "active" });
    const third = await f.daemon.waitForVisit(3);
    assertHostVisit(third, f, issue, [nextWeb, nextPackage]);
    expect(third.inputs.some((input) => input.command_id === interrupted.commandId)).toBe(false);
    f.daemon.release(third);
    await waitHostSettlement(f, issue, third);
    evidence.resume = resume;
    evidence.final = await readHostHistory(f, issue, [first, second, third], [initial, interrupted, nextWeb, nextPackage], interrupted.commandId);
    evidence.ended_display = await native.command("host-records", { issueId: issue.id, runId: third.identity.run_id,
      stopped: false, markers: [nextWeb.marker, nextPackage.marker, `Host fixture result for ${third.identity.run_id}`] });
    evidence.ended_capture = await capture("installed-host-ended.png");
    for (const comment of [initial, interrupted, nextWeb, nextPackage]) {
      await native.command("history", { commandId: comment.commandId, marker: comment.marker, state: comment.commandId === interrupted.commandId ? "Stopped" : "Succeeded" });
    }
    const status = await native.command<InstalledMainMetadata>("status");
    expect(status.errors).toEqual([]);
    expect(status.accountRequests.some((request) => request.method === "POST")).toBe(true);
    expect(status.accountRequests.every((request) => !request.workspaceId && !request.workspaceSlug)).toBe(true);
    expect(status.accountRequests.filter((request) => request.method === "POST").every((request) => request.clientOS === "linux" && request.clientVersion === boot.main.version)).toBe(true);
    expect(await f.webActor.client.countIssueDispatches(issue.id)).toBe(0);
    const summary = await f.daemon.close();
    daemonClosed = true;
    evidence.daemon = summary;
    expect(summary.exit_code).toBe(0);
    expect(summary.messages.find((message) => "claims" in message)).toMatchObject({ claims: 3, invalid_capacity: false, available_slots: 1, active_tasks: 0 });
    expect(summary.ledger).toHaveLength(3);
    for (const run of summary.ledger) expect(run).toMatchObject({ version: 2, sealed: true, phase: "proof_acknowledged", proof_state: "host_exit_observed",
      launches: [{ kind: "provider", process_observed: true, terminal_accepted: true, authorized: true, terminal_kind: "managed_launch_ended" }] });
    expect(summary.messages.filter((message) => "http" in message).flatMap((message) => "http" in message ? message.http : [])
      .every((fact) => fact.protocol_version === 3 && fact.containment === hostMode && fact.status === 200)).toBe(true);
    evidence.packaged_status = status;
    expect(errors).toEqual([]);
  } finally {
    if (fixture) {
      evidence.provider_observations = fixture.daemon.evidence();
      if (!daemonClosed) evidence.daemon = await fixture.daemon.close();
    }
    evidence.transport = native.transportEvidence;
    evidence.web_errors = errors;
    await native.close();
    const encoded = JSON.stringify(evidence, null, 2);
    expect(encoded).not.toMatch(/\b(?:mcb_|mct_|mdt_|mat_)[A-Za-z0-9_.-]+/);
    const path = testInfo.outputPath("installed-host-evidence.json");
    writeFileSync(path, encoded, { mode: 0o600 });
    await testInfo.attach("installed-host-evidence", { contentType: "application/json", path });
  }
});
