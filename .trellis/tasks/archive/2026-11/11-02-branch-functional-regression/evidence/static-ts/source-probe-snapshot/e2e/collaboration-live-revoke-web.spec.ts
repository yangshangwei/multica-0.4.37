import { test, expect } from "@playwright/test";
import type { CollaborationResources } from "@multica/core/collaboration";
import { assertUntouchedLanes, clickControl, controlEnvironmentReady, panelFor, seedCollaborationControlJourney, targetRow } from "./fixtures/collaboration-control-journey";
import {
  commentThroughUI, consentThroughUI, mentionNames, openGapSession, queuedInput, readGapHistory, readGapIssue, reloadGapPage,
  showGapHistory, withdrawThroughUI, type GapSession,
} from "./fixtures/collaboration-control-gaps";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });

/** Gap 3 (C05, R25/AC37/AC38). The run holds a real server permit whose
 * termination proof is withheld, so "withdrawn" can never mean "ended". */
test("Web withdrawing a grant during a permitted run stops that work, holds only its target, keeps the queued input, and needs proof plus explicit Resume", async ({ browser }, info) => {
  test.setTimeout(300_000);
  test.skip(!controlEnvironmentReady(), "Requires the task-owned password/managed deployment with execution enabled and matching production Web");
  const webUrl = info.project.use.baseURL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(webUrl).hostname);
  const fixture = await seedCollaborationControlJourney();
  const root = `/api/collaboration/issues/${fixture.issueId}`;
  const sessions: GapSession[] = [];
  try {
    const creator = await openGapSession(browser, fixture, "creator", webUrl);
    sessions.push(creator);
    const resumed = await clickControl(creator.page, fixture, panelFor(creator.page).getByRole("button", { name: "Resume all", exact: true }), 200);
    expect(resumed.receipt).toMatchObject({ code: "resumed", queue_state: "active" });
    const names = await mentionNames(fixture.creator, fixture.issueId);
    const [nameA, nameB, nameS] = [names.nameOf(fixture.agentA), names.nameOf(fixture.agentB), names.nameOf(fixture.squadS)];
    const a1 = queuedInput(await commentThroughUI(creator.page, fixture.issueId, "Live revoke A1 runs under a permit", [nameA]), fixture.agentA);
    const pending = await fixture.simulatedDaemon.permitPendingInput(fixture.issueId, fixture.agentA, a1.commandId, a1.batchId);
    const a2Comment = await commentThroughUI(creator.page, fixture.issueId, "Live revoke A2 waits behind the running group", [nameA]);
    const a2 = queuedInput(a2Comment, fixture.agentA);
    const live = await readGapIssue(fixture);
    expect(live.execution.runs).toEqual([expect.objectContaining({ id: pending.runId, progress_state: "launch_authorized", forced_disposition: null, settled_at: null })]);
    const resourcesBefore = await fixture.owner.requestJSON<CollaborationResources>(`${root}/resources`);

    // 1. The owner withdraws A's authorization in Manage AI resources (AC37).
    const owner = await openGapSession(browser, fixture, "owner", webUrl);
    sessions.push(owner);
    const withdrawn = await withdrawThroughUI(owner.page, fixture, fixture.agentA);
    expect(withdrawn.receipt).toMatchObject({ status: 200, code: "grant_revoked", action: "revoke_grant", issue_id: fixture.issueId });
    expect(withdrawn.receipt.channels).toEqual([expect.objectContaining({ channel_id: fixture.agentA.channelId, code: "stop_requested",
      group_id: pending.groupId, group_settled: false, stopped_runs: 1, unsettled_runs: 1 })]);
    expect(withdrawn.receipt.target_holds).toEqual([expect.objectContaining({ target_id: fixture.agentA.targetId, hold_reason: "grant_required" })]);
    await expect(panelFor(owner.page).getByRole("status").filter({ hasText: "Authorization withdrawn. This does not confirm that running work has ended." })).toBeVisible();

    // 2. Only A's work stops and only A is held; the queued input and every other lane remain (AC37).
    const revoked = await readGapIssue(fixture);
    expect(revoked.execution.runs).toEqual([expect.objectContaining({ id: pending.runId, forced_disposition: "stopped", proof_state: "unknown", progress_state: "ending", settled_at: null })]);
    expect(revoked.execution.groups.find((group) => group.id === pending.groupId)).toMatchObject({ forced_disposition: "stopped", state: "ending", settled_at: null });
    expect(revoked.execution.targets.find((target) => target.id === fixture.agentA.targetId)?.hold_reason).toBe("grant_required");
    expect(revoked.session).toEqual(live.session);
    for (const other of [fixture.agentB, fixture.squadS]) {
      expect(revoked.execution.channels.find((lane) => lane.id === other.channelId)).toEqual(live.execution.channels.find((lane) => lane.id === other.channelId));
      expect(revoked.execution.targets.find((target) => target.id === other.targetId)).toEqual(live.execution.targets.find((target) => target.id === other.targetId));
    }
    expect(revoked.execution.commands.find((item) => item.id === a1.commandId)?.state).toBe("in_flight");
    expect(revoked.execution.commands.find((item) => item.id === a2.commandId)).toMatchObject({ state: "waiting", group_id: null });
    const resourcesAfter = await fixture.owner.requestJSON<CollaborationResources>(`${root}/resources`);
    expect(resourcesAfter.targets.find((item) => item.target_id === fixture.agentA.targetId)?.authorized).toBe(false);
    for (const other of [fixture.agentB, fixture.squadS]) {
      expect(resourcesAfter.targets.find((item) => item.target_id === other.targetId)).toEqual(resourcesBefore.targets.find((item) => item.target_id === other.targetId));
    }
    // The participants are untouched: a resource withdrawal removes nobody (AC38).
    expect((await readGapIssue(fixture, "owner")).capabilities.control_ai).toBe(true);
    await reloadGapPage(creator.page, fixture.issueId, fixture.title);
    await expect(targetRow(creator.page, fixture.agentA).getByText("Resource authorization required", { exact: true })).toBeVisible();
    await expect(creator.page.locator(`[id="run-${pending.runId}"]`)).toContainText("Termination unconfirmed");
    await expect(creator.page.locator(`[id="run-${pending.runId}"]`)).not.toContainText("Termination confirmed");
    await showGapHistory(creator.page, a2.commandId, a2.marker, "Waiting");

    // 3. A leaves the @ candidates and an old A selection is refused by the server (AC37).
    const catalogue = (await mentionNames(fixture.creator, fixture.issueId)).catalogue;
    expect(catalogue.targets.map((item) => item.target_id).sort()).toEqual([fixture.agentB.targetId, fixture.squadS.targetId].sort());
    const aiTargets = creator.page.getByRole("form", { name: "Comments", exact: true }).getByRole("group", { name: "AI targets", exact: true });
    await expect(aiTargets.getByRole("button", { name: nameB, exact: true })).toBeVisible();
    await expect(aiTargets.getByRole("button", { name: nameS, exact: true })).toBeVisible();
    await expect(aiTargets.getByRole("button", { name: nameA, exact: true })).toHaveCount(0);
    const oldSelection = a2Comment.content.match(/\[@[^\]]*\]\(mention:\/\/agent\/[0-9a-f-]+\)/i)?.[0];
    if (!oldSelection?.includes(fixture.agentA.resourceId)) throw new Error("The earlier UI comment did not carry the A selection");
    const stale = await commentThroughUI(creator.page, fixture.issueId, "Live revoke reuses the old A selection", [], `${oldSelection} `);
    expect(stale.preview.targets).toEqual([expect.objectContaining({ kind: "agent", resource_id: fixture.agentA.resourceId, state: "blocked", code: "target_unavailable", command_id: null })]);
    expect(stale.receipt.targets).toEqual([expect.objectContaining({ resource_id: fixture.agentA.resourceId, state: "blocked", code: "target_unavailable", command_id: null })]);
    const afterStale = await readGapIssue(fixture);
    expect({ commands: afterStale.execution.commands, groups: afterStale.execution.groups, runs: afterStale.execution.runs })
      .toEqual({ commands: revoked.execution.commands, groups: revoked.execution.groups, runs: revoked.execution.runs });

    // 4. Resume is refused while the grant is missing (grant_required).
    await reloadGapPage(creator.page, fixture.issueId, fixture.title);
    const held = await clickControl(creator.page, fixture, targetRow(creator.page, fixture.agentA).getByRole("button", { name: "Resume target", exact: true }), 409);
    expect(held.receipt).toMatchObject({ code: "grant_required", scope: "target", target_id: fixture.agentA.targetId });
    expect((await readGapIssue(fixture)).execution).toEqual(afterStale.execution);

    // 5. Re-consent keeps A held and the queue waiting; the barrier still refuses (AC38).
    const reconsent = await consentThroughUI(owner.page, fixture, fixture.agentA);
    const reconsented = await readGapIssue(fixture);
    expect(reconsented.execution.targets.find((target) => target.id === fixture.agentA.targetId)?.hold_reason).toBe("grant_required");
    expect({ commands: reconsented.execution.commands, runs: reconsented.execution.runs }).toEqual({ commands: afterStale.execution.commands, runs: afterStale.execution.runs });
    assertUntouchedLanes(fixture, afterStale, reconsented, fixture.agentA);
    await reloadGapPage(creator.page, fixture.issueId, fixture.title);
    const barrier = await clickControl(creator.page, fixture, targetRow(creator.page, fixture.agentA).getByRole("button", { name: "Resume target", exact: true }), 409);
    expect(barrier.receipt).toMatchObject({ code: "stop_unconfirmed", scope: "target", target_id: fixture.agentA.targetId });
    await expect(panelFor(creator.page).getByRole("status", { name: "Control result", exact: true })).toContainText("Termination unconfirmed");
    expect((await readGapIssue(fixture)).execution).toEqual(reconsented.execution);
    await reloadGapPage(creator.page, fixture.issueId, fixture.title);
    const whole = await clickControl(creator.page, fixture, panelFor(creator.page).getByRole("button", { name: "Resume all", exact: true }), 200);
    expect(whole.receipt).toMatchObject({ code: "partially_resumed", scope: "issue" });
    expect(whole.receipt.channels.find((lane) => lane.channel_id === fixture.agentA.channelId)).toMatchObject({ code: "stop_unconfirmed", unsettled_runs: 1, group_id: pending.groupId });
    expect(whole.receipt.target_holds).toEqual([expect.objectContaining({ target_id: fixture.agentA.targetId, code: "stop_unconfirmed", hold_reason: "grant_required" })]);
    const laneNumber = reconsented.execution.channels.findIndex((lane) => lane.id === fixture.agentA.channelId) + 1;
    const result = panelFor(creator.page).getByRole("status", { name: "Control result", exact: true });
    await expect(result.getByText(`Execution lane ${laneNumber} · Termination unconfirmed`, { exact: true })).toBeVisible();
    await expect(result.getByText("1 targets remain held.", { exact: true })).toBeVisible();
    await expect(result.getByText(`Agent ${fixture.agentA.resourceId} · Termination unconfirmed`, { exact: true })).toBeVisible();
    await expect(result).not.toContainText("Termination confirmed");
    // A revoke never pauses lane A, so the headline must count lanes by their
    // Resume result: lane A is still behind its stop barrier.
    const resumedLanes = whole.receipt.channels.filter((lane) => lane.code === "resumed" || lane.code === "already_active").length;
    expect(resumedLanes).toBe(whole.receipt.channels.length - 1);
    await expect(result.getByText(`${resumedLanes} lanes resumed; 1 remain paused.`, { exact: true })).toBeVisible();
    const wholeResultText = await result.innerText();
    const final = await readGapIssue(fixture);
    expect(final.execution).toEqual(reconsented.execution);
    expect(final.execution.commands.find((item) => item.id === a2.commandId)).toMatchObject({ state: "waiting", group_id: null });
    expect((await readGapHistory(fixture.creator, fixture.issueId)).items.find((item) => item.id === a2.commandId)).toMatchObject({ state: "waiting", cancellation: null });
    expect(await fixture.owner.countIssueDispatches(fixture.issueId)).toBe(0);
    for (const session of sessions) expect(session.errors).toEqual([]);
    await info.attach("live-revoke-evidence", { contentType: "application/json", body: JSON.stringify({ issue_id: fixture.issueId, pending, inputs: { a1, a2 },
      withdrawn: withdrawn.receipt, revoked_execution: revoked.execution, stale_selection: stale.receipt, held_resume: held.receipt,
      reconsent: reconsent.receipt, barrier_resume: barrier.receipt, whole_resume: whole.receipt, whole_resume_ui_text: wholeResultText,
      scope: "Simulated managed daemon: real public claim/permit, no provider process, no outcome and no termination proof" }, null, 2) });
  } finally {
    await fixture.simulatedDaemon.disconnect();
    for (const session of sessions) await session.close().catch(() => undefined);
  }
});
