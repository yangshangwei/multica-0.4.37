import { test, expect } from "@playwright/test";
import { clickControl, controlEnvironmentReady, panelFor, seedCollaborationControlJourney, targetRow } from "./fixtures/collaboration-control-journey";
import {
  commentThroughUI, gapControlBody, gapWorkflowObservations, lifecycleThroughUI, mentionNames, openGapSession, queuedInput, rawGapRequest,
  readGapHistory, readGapIssue, reloadGapPage, showGapHistory, type GapSession,
} from "./fixtures/collaboration-control-gaps";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });

/** Gap 1 (C09, R35/AC58). The simulated daemon obtains a real permit and then
 * withholds every outcome and proof, so the R17 barrier is genuinely unproven. */
test("Web closed issue refuses @agent and Resume, only the creator reopens, and reopening revives nothing or the stop barrier", async ({ browser }, info) => {
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
    const page = creator.page;
    const resumed = await clickControl(page, fixture, panelFor(page).getByRole("button", { name: "Resume all", exact: true }), 200);
    expect(resumed.receipt).toMatchObject({ code: "resumed", queue_state: "active" });
    const names = await mentionNames(fixture.creator, fixture.issueId);
    const nameA = names.nameOf(fixture.agentA);
    const nameB = names.nameOf(fixture.agentB);

    // Live work: A1 obtains a real permit; A2 waits behind it; B1 waits in its own lane.
    const a1 = queuedInput(await commentThroughUI(page, fixture.issueId, "Terminal journey A1 runs under a permit", [nameA]), fixture.agentA);
    const pending = await fixture.simulatedDaemon.permitPendingInput(fixture.issueId, fixture.agentA, a1.commandId, a1.batchId);
    const a2 = queuedInput(await commentThroughUI(page, fixture.issueId, "Terminal journey A2 waits behind A1", [nameA]), fixture.agentA);
    const b1 = queuedInput(await commentThroughUI(page, fixture.issueId, "Terminal journey B1 waits in lane B", [nameB]), fixture.agentB);
    const live = await readGapIssue(fixture);
    expect(live.execution.runs).toEqual([expect.objectContaining({ id: pending.runId, progress_state: "launch_authorized", proof_state: "unknown", forced_disposition: null, settled_at: null })]);
    expect(live.execution.commands.find((item) => item.id === a1.commandId)?.state).toBe("in_flight");
    for (const input of [a2, b1]) expect(live.execution.commands.find((item) => item.id === input.commandId)).toMatchObject({ state: "waiting", group_id: null });

    // Close through the creator's UI (AC57): waiting inputs are cancelled; the
    // permitted run is asked to stop but its termination stays unconfirmed.
    await reloadGapPage(page, fixture.issueId, fixture.title);
    const completed = await lifecycleThroughUI(page, fixture.issueId, "complete");
    const closed = await readGapIssue(fixture);
    expect(closed).toMatchObject({ status_category: "done", session: { queue_state: "closed" } });
    expect(closed.execution.runs).toEqual([expect.objectContaining({ id: pending.runId, forced_disposition: "stopped", proof_state: "unknown", progress_state: "ending", settled_at: null })]);
    // A business close never settles an in-flight input; only the stop/proof protocol does.
    expect(closed.execution.commands.find((item) => item.id === a1.commandId)?.state).toBe("in_flight");
    const closedHistory = await readGapHistory(fixture.creator, fixture.issueId);
    for (const input of [a2, b1]) {
      expect(closedHistory.items.find((item) => item.id === input.commandId)).toMatchObject({ state: "cancelled", reason: "issue_completed", content: input.content,
        author_user_id: fixture.creatorId, cancellation: { kind: "automatic", actor_user_id: fixture.creatorId } });
    }
    await reloadGapPage(page, fixture.issueId, fixture.title);
    await expect(panelFor(page).locator(":scope > p").filter({ hasText: /^Closed$/ })).toBeVisible();
    await expect(panelFor(page).getByRole("button", { name: "Resume all", exact: true })).toBeDisabled();
    await expect(panelFor(page).getByRole("button", { name: "Stop all", exact: true })).toBeEnabled();
    await expect(targetRow(page, fixture.agentA).getByRole("button", { name: "Resume target", exact: true })).toBeDisabled();
    await expect(page.locator(`[id="run-${pending.runId}"]`)).toContainText("Termination unconfirmed");
    await expect(page.locator(`[id="run-${pending.runId}"]`)).toContainText("Waiting for termination");
    for (const input of [a2, b1]) await showGapHistory(page, input.commandId, input.marker, "Cancelled", "The collaboration was completed");

    // 1. A new @A comment on the closed issue is a comment only (AC58, AC93).
    const closedMention = await commentThroughUI(page, fixture.issueId, "Terminal journey: please try again after completion", [nameA]);
    expect(closedMention.preview.targets).toEqual([expect.objectContaining({ target_id: fixture.agentA.targetId, state: "blocked", code: "issue_closed", command_id: null })]);
    expect(closedMention.receipt.targets).toEqual([expect.objectContaining({ target_id: fixture.agentA.targetId, state: "blocked", code: "issue_closed", command_id: null })]);
    await expect(page.getByRole("form", { name: "Comments", exact: true }).getByRole("status").filter({ hasText: `${nameA} · Closed` })).toBeVisible();
    const afterMention = await readGapIssue(fixture);
    expect({ commands: afterMention.execution.commands, groups: afterMention.execution.groups, runs: afterMention.execution.runs })
      .toEqual({ commands: closed.execution.commands, groups: closed.execution.groups, runs: closed.execution.runs });
    expect(afterMention.session).toEqual(closed.session);

    // 2. The server refuses Resume itself, not only the disabled buttons (AC58).
    for (const [scope, targetId] of [["issue", undefined], ["target", fixture.agentB.targetId]] as const) {
      const probe = await rawGapRequest(fixture, "creator", "POST", `${root}/control`, gapControlBody(await readGapIssue(fixture), "resume", scope, targetId));
      expect(probe.status).toBe(409);
      expect(probe.body).toMatchObject({ status: 409, code: "issue_closed", action: "resume", scope });
      expect(await fixture.creator.requestJSON(`${root}/control/operations/${probe.requestId}`)).toEqual(probe.body);
    }
    const afterProbes = await readGapIssue(fixture);
    expect(afterProbes.execution).toEqual(afterMention.execution);
    expect(afterProbes.session).toEqual(afterMention.session);

    // 3. The handler cannot reopen: no button, and both server routes refuse (AC55, AC56).
    const handler = await openGapSession(browser, fixture, "handler", webUrl);
    sessions.push(handler);
    await expect(panelFor(handler.page).getByRole("button", { name: "Stop all", exact: true })).toBeEnabled();
    await expect(handler.page.getByRole("button", { name: "Reopen issue", exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Reopen issue", exact: true })).toBeVisible();
    expect((await rawGapRequest(fixture, "handler", "GET", `${root}/status-options?action=reopen`)).status).toBe(403);
    const creatorOptions = await fixture.creator.requestJSON<{ default_status: string; status_options_version: string }>(`${root}/status-options?action=reopen`);
    const handlerView = await readGapIssue(fixture, "handler");
    const refusedReopen = await rawGapRequest(fixture, "handler", "POST", `${root}/lifecycle`, {
      ...gapWorkflowObservations(handlerView), action: "reopen", status: creatorOptions.default_status, status_options_version: creatorOptions.status_options_version });
    expect(refusedReopen.status).toBe(403);
    expect(refusedReopen.body).toMatchObject({ code: "forbidden", status: 403 });
    expect(await fixture.handler.requestJSON(`${root}/operations/${refusedReopen.requestId}`)).toMatchObject({ code: "forbidden" });
    const afterRefusal = await readGapIssue(fixture);
    expect({ status: afterRefusal.status, lifecycle_epoch: afterRefusal.lifecycle_epoch, collaboration_revision: afterRefusal.collaboration_revision,
      session: afterRefusal.session, execution: afterRefusal.execution })
      .toEqual({ status: afterProbes.status, lifecycle_epoch: afterProbes.lifecycle_epoch, collaboration_revision: afterProbes.collaboration_revision,
        session: afterProbes.session, execution: afterProbes.execution });

    // 4. The creator reopens in the UI: everything is paused and nothing revives (AC58).
    await reloadGapPage(page, fixture.issueId, fixture.title);
    const reopened = await lifecycleThroughUI(page, fixture.issueId, "reopen");
    const reopenedIssue = await readGapIssue(fixture);
    expect(["backlog", "todo", "in_progress", "in_review", "blocked"]).toContain(reopenedIssue.status_category);
    expect(reopenedIssue.status).toBe(reopened.options.default_status);
    expect(reopenedIssue.session.queue_state).toBe("paused");
    expect(BigInt(reopenedIssue.lifecycle_epoch)).toBe(BigInt(closed.lifecycle_epoch) + 1n);
    expect(reopenedIssue.execution.channels.every((lane) => lane.queue_state === "paused" && lane.pause_reason === "issue_reopened")).toBe(true);
    for (const target of reopenedIssue.execution.targets) {
      const before = closed.execution.targets.find((item) => item.id === target.id)!;
      // Reopen pauses lanes; it neither releases nor imposes a resource hold.
      expect({ hold_reason: target.hold_reason, hold_version: target.hold_version }).toEqual({ hold_reason: before.hold_reason, hold_version: before.hold_version });
      expect(target.control.queue_state).toBe("paused");
    }
    expect({ commands: reopenedIssue.execution.commands, groups: reopenedIssue.execution.groups, runs: reopenedIssue.execution.runs })
      .toEqual({ commands: closed.execution.commands, groups: closed.execution.groups, runs: closed.execution.runs });
    const reopenedHistory = await readGapHistory(fixture.creator, fixture.issueId);
    for (const input of [a2, b1]) expect(reopenedHistory.items.find((item) => item.id === input.commandId)).toEqual(closedHistory.items.find((item) => item.id === input.commandId));
    await reloadGapPage(page, fixture.issueId, fixture.title);
    await expect(panelFor(page).locator(":scope > p").filter({ hasText: /^Paused$/ })).toBeVisible();
    await expect(page.getByRole("button", { name: "Reopen issue", exact: true })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Complete issue", exact: true })).toBeVisible();
    for (const input of [a2, b1]) await showGapHistory(page, input.commandId, input.marker, "Cancelled", "The collaboration was completed");

    // 5. Resume all after reopen: the unconfirmed run still holds lane A (R17).
    const partial = await clickControl(page, fixture, panelFor(page).getByRole("button", { name: "Resume all", exact: true }), 200);
    expect(partial.receipt).toMatchObject({ code: "partially_resumed", scope: "issue", queue_state: "active" });
    expect(partial.receipt.channels.find((lane) => lane.channel_id === fixture.agentA.channelId)).toMatchObject({ code: "stop_unconfirmed", queue_state: "paused", unsettled_runs: 1 });
    for (const other of [fixture.agentB, fixture.squadS]) {
      expect(partial.receipt.channels.find((lane) => lane.channel_id === other.channelId)).toMatchObject({ code: "resumed", queue_state: "active", unsettled_runs: 0 });
    }
    const result = panelFor(page).getByRole("status", { name: "Control result", exact: true });
    await expect(result.getByText("2 lanes resumed; 1 remain paused.", { exact: true })).toBeVisible();
    await expect(result.getByText("0 targets remain held.", { exact: true })).toBeVisible();
    const afterResume = await readGapIssue(fixture);
    const laneNumber = afterResume.execution.channels.findIndex((lane) => lane.id === fixture.agentA.channelId) + 1;
    await expect(result.getByText(`Execution lane ${laneNumber} · Termination unconfirmed`, { exact: true })).toBeVisible();
    await expect(result).not.toContainText(/all (?:targets |lanes )?resumed/i);
    expect(afterResume.session.queue_state).toBe("active");
    expect(afterResume.execution.channels.find((lane) => lane.id === fixture.agentA.channelId)).toEqual(reopenedIssue.execution.channels.find((lane) => lane.id === fixture.agentA.channelId));
    expect({ commands: afterResume.execution.commands, groups: afterResume.execution.groups, runs: afterResume.execution.runs })
      .toEqual({ commands: closed.execution.commands, groups: closed.execution.groups, runs: closed.execution.runs });
    const refusedTarget = await clickControl(page, fixture, targetRow(page, fixture.agentA).getByRole("button", { name: "Resume target", exact: true }), 409);
    expect(refusedTarget.receipt).toMatchObject({ code: "stop_unconfirmed", scope: "target", target_id: fixture.agentA.targetId });
    expect((await readGapIssue(fixture)).execution).toEqual(afterResume.execution);
    expect(await fixture.owner.countIssueDispatches(fixture.issueId)).toBe(0);
    for (const session of sessions) expect(session.errors).toEqual([]);
    await info.attach("terminal-reopen-evidence", { contentType: "application/json", body: JSON.stringify({ issue_id: fixture.issueId, pending,
      inputs: { a1, a2, b1 }, completed: completed.receipt, closed_execution: closed.execution, closed_mention: closedMention.receipt,
      refused_reopen_by_handler: refusedReopen, reopened: reopened.receipt, reopened_execution: reopenedIssue.execution,
      partial_resume: partial.receipt, refused_target_resume: refusedTarget.receipt,
      scope: "Simulated managed daemon: real public claim/permit, no provider process, no outcome and no termination proof" }, null, 2) });
  } finally {
    // Never invent a proof; only mark the test-owned runtime offline.
    await fixture.simulatedDaemon.disconnect();
    for (const session of sessions) await session.close().catch(() => undefined);
  }
});
