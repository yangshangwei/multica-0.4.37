import { test, expect } from "@playwright/test";
import type { CollaborationParticipants, CollaborationResources, CollaborationWorkbench } from "@multica/core/collaboration";
import { clickControl, controlEnvironmentReady, panelFor, seedCollaborationControlJourney, targetRow } from "./fixtures/collaboration-control-journey";
import {
  commentThroughUI, consentThroughUI, gapClient, gapControlBody, gapWorkflowObservations, inviteThroughUI, leaveThroughUI, mentionNames,
  openGapSession, queuedInput, rawGapRequest, readGapHistory, readGapIssue, reloadGapPage, seedPeerGrant, showGapHistory, type GapSession,
} from "./fixtures/collaboration-control-gaps";

test.use({ actionTimeout: 15_000, screenshot: "only-on-failure", trace: "retain-on-failure" });

/** Gap 2 (C08/C05, R30/R31, AC47-AC50). The owner of A, B and squad S is an
 * invited participant who is neither creator nor handler. */
test("Web Leave issue refuses creator and handler; an owner's departure revokes every grant, cancels only their waiting input, stops running work, and rejoining restores nothing", async ({ browser }, info) => {
  test.setTimeout(300_000);
  test.skip(!controlEnvironmentReady(), "Requires the task-owned password/managed deployment with execution enabled and matching production Web");
  const webUrl = info.project.use.baseURL!;
  expect(["localhost", "127.0.0.1"]).toContain(new URL(webUrl).hostname);
  const fixture = await seedCollaborationControlJourney();
  const root = `/api/collaboration/issues/${fixture.issueId}`;
  const peer = await seedPeerGrant(fixture);
  const sessions: GapSession[] = [];
  try {
    const creator = await openGapSession(browser, fixture, "creator", webUrl);
    sessions.push(creator);
    const resumed = await clickControl(creator.page, fixture, panelFor(creator.page).getByRole("button", { name: "Resume all", exact: true }), 200);
    expect(resumed.receipt).toMatchObject({ code: "resumed", queue_state: "active" });
    const names = await mentionNames(fixture.creator, fixture.issueId);
    const [nameA, nameB] = [names.nameOf(fixture.agentA), names.nameOf(fixture.agentB)];

    // Creator's A1 runs under a real permit; creator's B1 and the owner's two inputs wait.
    const a1 = queuedInput(await commentThroughUI(creator.page, fixture.issueId, "Leave journey A1 on the owner's resource", [nameA]), fixture.agentA);
    const pending = await fixture.simulatedDaemon.permitPendingInput(fixture.issueId, fixture.agentA, a1.commandId, a1.batchId);
    const b1 = queuedInput(await commentThroughUI(creator.page, fixture.issueId, "Leave journey creator B1 must survive", [nameB]), fixture.agentB);
    const owner = await openGapSession(browser, fixture, "owner", webUrl);
    sessions.push(owner);
    const ownerComment = await commentThroughUI(owner.page, fixture.issueId, "Leave journey owner inputs for A and B", [nameA, nameB]);
    const ownerA = queuedInput(ownerComment, fixture.agentA);
    const ownerB = queuedInput(ownerComment, fixture.agentB);
    const live = await readGapIssue(fixture);
    expect(live.execution.runs).toEqual([expect.objectContaining({ id: pending.runId, progress_state: "launch_authorized", forced_disposition: null, settled_at: null })]);
    for (const input of [b1, ownerA, ownerB]) expect(live.execution.commands.find((item) => item.id === input.commandId)).toMatchObject({ state: "waiting", group_id: null });

    // 1. Creator and handler cannot leave: no button, and the server refuses (AC47).
    const handler = await openGapSession(browser, fixture, "handler", webUrl);
    sessions.push(handler);
    for (const session of [creator, handler]) {
      await expect(session.page.getByRole("button", { name: "Transfer", exact: true })).toBeVisible();
      await expect(session.page.getByRole("button", { name: "Leave issue", exact: true })).toHaveCount(0);
    }
    await expect(owner.page.getByRole("button", { name: "Leave issue", exact: true })).toBeVisible();
    const participantsBefore = await fixture.creator.requestJSON<CollaborationParticipants>(`${root}/participants`);
    const beforeRefusals = await readGapIssue(fixture);
    const refusals: unknown[] = [];
    for (const [role, code] of [["creator", "creator_cannot_leave"], ["handler", "handler_must_transfer"]] as const) {
      const view = await readGapIssue(fixture, role);
      expect(view.capabilities.leave).toBe(false);
      const refused = await rawGapRequest(fixture, role, "POST", `${root}/leave`, gapWorkflowObservations(view));
      expect(refused.status).toBe(409);
      expect(refused.body).toMatchObject({ code, status: 409 });
      expect(await gapClient(fixture, role).requestJSON(`${root}/operations/${refused.requestId}`)).toMatchObject({ code });
      refusals.push(refused);
    }
    const afterRefusals = await readGapIssue(fixture);
    expect({ revision: afterRefusals.collaboration_revision, handler: afterRefusals.handler, execution: afterRefusals.execution })
      .toEqual({ revision: beforeRefusals.collaboration_revision, handler: beforeRefusals.handler, execution: beforeRefusals.execution });
    expect((await fixture.creator.requestJSON<CollaborationParticipants>(`${root}/participants`)).participants).toEqual(participantsBefore.participants);

    // 2. The owner leaves in the UI and immediately loses the issue (AC47, AC48).
    await reloadGapPage(owner.page, fixture.issueId, fixture.title);
    const ownerBeforeLeave = await readGapIssue(fixture, "owner");
    expect(ownerBeforeLeave.capabilities).toMatchObject({ leave: true, control_ai: true });
    const left = await leaveThroughUI(owner.page, fixture.issueId);
    expect(left.receipt.code).toBe("participant_left");
    // Sub-panels briefly show their own denial while the detail refetches; the
    // settled page replaces the whole detail with exactly one denial.
    await expect(owner.page.getByRole("heading", { name: fixture.title, exact: true })).toHaveCount(0);
    await expect(owner.page.getByText("You no longer have access to this issue.", { exact: true })).toHaveCount(1);
    await expect(owner.page.getByText("You no longer have access to this issue.", { exact: true })).toBeVisible();
    const ownerList = owner.page.getByRole("region", { name: "Collaboration issues", exact: true });
    await expect(ownerList.getByRole("link", { name: new RegExp(peer.title) })).toHaveCount(1);
    await expect(ownerList.getByRole("link", { name: new RegExp(fixture.title) })).toHaveCount(0);
    await expect(ownerList.getByText("1 issues", { exact: true })).toBeVisible();
    expect((await rawGapRequest(fixture, "owner", "GET", root)).status).toBe(404);
    expect((await rawGapRequest(fixture, "owner", "GET", `${root}/mention-targets`)).status).toBe(404);
    const ownerLists = await fixture.owner.requestJSON<CollaborationWorkbench>("/api/me/workbench?view=participating");
    expect(ownerLists.items.map((item) => item.id)).not.toContain(fixture.issueId);
    expect(ownerLists.items.map((item) => item.id)).toContain(peer.issueId);
    // A voluntary leaver keeps only their own minimal receipt (collaboration_workflow.go:604-606).
    expect(await fixture.owner.requestJSON(`${root}/operations/${left.requestId}`)).toMatchObject({ code: "participant_left" });
    // The departed owner can no longer control the issue (AC48).
    const staleStop = await rawGapRequest(fixture, "owner", "POST", `${root}/control`, gapControlBody(ownerBeforeLeave, "stop", "target", fixture.agentA.targetId));
    expect(staleStop.status).toBe(404);

    // 3. Every grant is revoked, only the owner's waiting inputs are cancelled,
    // and the running A work is asked to stop without any proof (AC48, AC49).
    const departed = await readGapIssue(fixture);
    expect((await fixture.creator.requestJSON<CollaborationParticipants>(`${root}/participants`)).participants.map((item) => item.user_id)).not.toContain(fixture.ownerId);
    const resources = await fixture.creator.requestJSON<CollaborationResources>(`${root}/resources`);
    expect(resources.targets.map((item) => [item.target_id, item.authorized, item.resources.every((resource) => !resource.consented)]).sort())
      .toEqual(fixture.targets.map((target) => [target.targetId, false, true]).sort());
    expect(departed.execution.targets.every((target) => target.hold_reason === "grant_required")).toBe(true);
    expect(departed.execution.runs).toEqual([expect.objectContaining({ id: pending.runId, forced_disposition: "stopped", proof_state: "unknown", progress_state: "ending", settled_at: null })]);
    expect(departed.execution.commands.find((item) => item.id === a1.commandId)?.state).toBe("in_flight");
    expect(departed.execution.commands.find((item) => item.id === b1.commandId)).toMatchObject({ state: "waiting", group_id: null });
    // Losing a resource holds its targets; only losing a consumed input's author
    // pauses a lane (collaboration_invalidation.go:13-16). A1's author stayed.
    expect(departed.execution.channels.find((lane) => lane.id === fixture.agentA.channelId)?.queue_state).toBe(live.execution.channels.find((lane) => lane.id === fixture.agentA.channelId)?.queue_state);
    const departedHistory = await readGapHistory(fixture.creator, fixture.issueId);
    for (const input of [ownerA, ownerB]) {
      expect(departedHistory.items.find((item) => item.id === input.commandId)).toMatchObject({ state: "cancelled", reason: "participant_left", content: input.content,
        author_user_id: fixture.ownerId, cancellation: { kind: "automatic", actor_user_id: fixture.ownerId } });
    }
    expect((await mentionNames(fixture.creator, fixture.issueId)).catalogue.targets).toEqual([]);
    // Leaving this issue does not withdraw the owner's grant on another issue (AC50).
    expect((await fixture.owner.requestJSON<CollaborationResources>(`/api/collaboration/issues/${peer.issueId}/resources`)).targets
      .find((item) => item.target_id === peer.targetId)).toEqual(peer.grant);
    await reloadGapPage(creator.page, fixture.issueId, fixture.title);
    for (const input of [ownerA, ownerB]) await showGapHistory(creator.page, input.commandId, input.marker, "Cancelled", "The author left");
    await showGapHistory(creator.page, b1.commandId, b1.marker, "Waiting");
    for (const target of fixture.targets) await expect(targetRow(creator.page, target).getByText("Resource authorization required", { exact: true })).toBeVisible();
    await expect(creator.page.locator(`[id="run-${pending.runId}"]`)).toContainText("Termination unconfirmed");
    const composer = creator.page.getByRole("form", { name: "Comments", exact: true });
    await expect(composer.getByRole("group", { name: "People", exact: true })).toBeVisible();
    await expect(composer.getByRole("group", { name: "AI targets", exact: true })).toHaveCount(0);
    const activity = await fixture.creator.requestJSON<{ items: { kind: string; actor: { user_id: string } | null }[] }>(`${root}/activity?limit=100`);
    expect(activity.items.filter((item) => item.kind === "participant_left").map((item) => item.actor?.user_id)).toEqual([fixture.ownerId]);

    // 4. Rejoining restores participation only (AC50).
    const ownerName = (await fixture.owner.requestJSON<{ name: string }>("/api/me")).name;
    const reinvited = await inviteThroughUI(creator.page, fixture.issueId, { id: fixture.ownerId, name: ownerName });
    const ownerResources = await fixture.owner.requestJSON<CollaborationResources>(`${root}/resources`);
    expect(ownerResources.targets.every((item) => item.authorized === false && item.resources.every((resource) => resource.can_consent && !resource.consented))).toBe(true);
    const rejoined = await readGapIssue(fixture);
    expect({ commands: rejoined.execution.commands, groups: rejoined.execution.groups, runs: rejoined.execution.runs, targets: rejoined.execution.targets })
      .toEqual({ commands: departed.execution.commands, groups: departed.execution.groups, runs: departed.execution.runs, targets: departed.execution.targets });
    const rejoinedHistory = await readGapHistory(fixture.creator, fixture.issueId);
    for (const input of [ownerA, ownerB]) expect(rejoinedHistory.items.find((item) => item.id === input.commandId)).toEqual(departedHistory.items.find((item) => item.id === input.commandId));
    await reloadGapPage(owner.page, fixture.issueId, fixture.title);

    await reloadGapPage(creator.page, fixture.issueId, fixture.title);
    const heldResume = await clickControl(creator.page, fixture, targetRow(creator.page, fixture.agentA).getByRole("button", { name: "Resume target", exact: true }), 409);
    expect(heldResume.receipt).toMatchObject({ code: "grant_required", scope: "target", target_id: fixture.agentA.targetId });
    expect((await readGapIssue(fixture)).execution).toEqual(rejoined.execution);

    // Re-consent alone neither releases the hold nor passes the stop barrier (AC38, AC50).
    const reconsent = await consentThroughUI(owner.page, fixture, fixture.agentA);
    const reconsented = await readGapIssue(fixture);
    expect(reconsented.execution.targets.find((item) => item.id === fixture.agentA.targetId)?.hold_reason).toBe("grant_required");
    expect({ commands: reconsented.execution.commands, runs: reconsented.execution.runs }).toEqual({ commands: rejoined.execution.commands, runs: rejoined.execution.runs });
    expect((await mentionNames(fixture.creator, fixture.issueId)).catalogue.targets.map((item) => item.target_id)).toEqual([fixture.agentA.targetId]);
    await reloadGapPage(creator.page, fixture.issueId, fixture.title);
    const barrier = await clickControl(creator.page, fixture, targetRow(creator.page, fixture.agentA).getByRole("button", { name: "Resume target", exact: true }), 409);
    expect(barrier.receipt).toMatchObject({ code: "stop_unconfirmed", scope: "target", target_id: fixture.agentA.targetId });
    await expect(panelFor(creator.page).getByRole("status", { name: "Control result", exact: true })).toContainText("Termination unconfirmed");
    const final = await readGapIssue(fixture);
    expect(final.execution).toEqual(reconsented.execution);
    expect(final.execution.runs.map((run) => run.id)).toEqual([pending.runId]);
    expect(final.execution.commands.find((item) => item.id === b1.commandId)?.state).toBe("waiting");
    expect(await fixture.owner.countIssueDispatches(fixture.issueId)).toBe(0);
    for (const session of sessions) expect(session.errors).toEqual([]);
    await info.attach("leave-evidence", { contentType: "application/json", body: JSON.stringify({ issue_id: fixture.issueId, peer_issue_id: peer.issueId, pending,
      inputs: { a1, b1, ownerA, ownerB }, refusals, left: left.receipt, departed_execution: departed.execution, reinvited: reinvited.receipt,
      held_resume: heldResume.receipt, reconsent: reconsent.receipt, barrier_resume: barrier.receipt,
      scope: "Simulated managed daemon: real public claim/permit, no provider process, no outcome and no termination proof" }, null, 2) });
  } finally {
    await fixture.simulatedDaemon.disconnect();
    for (const session of sessions) await session.close().catch(() => undefined);
  }
});
