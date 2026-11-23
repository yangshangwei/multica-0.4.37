import { randomUUID } from "node:crypto";
import { expect, type Locator, type Page } from "@playwright/test";
import type {
  CollaborationCommandHistory, CollaborationIssue, CollaborationOperation, CollaborationResourceReceipt, CollaborationResources,
} from "@multica/core/collaboration";
import { assertProviderInput, type SentCrossClientComment } from "./collaboration-cross-client-execution";
import {
  createLifecycleRuntimeTransport, lifecycleComment, lifecycleControl, lifecycleIssuePath, lifecycleTargetRow,
  readLifecycleIssue, refreshLifecyclePage, seedLifecycleControls, showLifecycleHistory, startLifecycleDaemons,
  type LifecycleFixture,
} from "./collaboration-lifecycle-controls";

type Fixture = Awaited<ReturnType<typeof seedLifecycleControls>>;
type Target = LifecycleFixture["targetA"];
type Actor = LifecycleFixture["webActor"];
type Capture = (stage: string, page: Page, focus: Locator) => Promise<void>;
const panelFor = (page: Page) => page.getByRole("region", { name: "AI execution", exact: true });
const resourceRoot = (page: Page, name: string) => panelFor(page).locator("li")
  .filter({ has: page.getByRole("heading", { name, exact: true }) }).first();

async function resourcesOpen(page: Page) {
  const toggle = panelFor(page).getByRole("button", { name: "Manage AI resources", exact: true });
  if (await toggle.getAttribute("aria-expanded") !== "true") await toggle.click();
  await expect(panelFor(page).getByRole("button", { name: "Refresh resources", exact: true })).toBeVisible();
}

/** Registration and every consent use the shared public forms. Resource IDs
 * come from public agent/squad creation, never a target/channel database seed. */
async function registerLaneTarget(page: Page, fixture: LifecycleFixture, kind: "agent" | "squad", resourceId: string) {
  await resourcesOpen(page);
  await panelFor(page).getByRole("button", { name: "Add AI target", exact: true }).click();
  const form = page.getByRole("form", { name: "Add AI target", exact: true });
  await form.getByLabel("Resource workspace", { exact: true }).selectOption(fixture.targetA.runtime.workspaceId);
  const select = form.getByLabel("AI target", { exact: true });
  await expect(select).toBeEnabled();
  await select.selectOption(`${kind}:${resourceId}`);
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${lifecycleIssuePath(fixture.issueId)}/targets`
    && response.request().method() === "POST");
  await form.getByRole("button", { name: "Register target", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(201);
  const receipt: CollaborationResourceReceipt = await response.json();
  if ("redacted" in receipt || !receipt.target_id) throw new Error("Public lane registration returned no exact target");
  const registered = await fixture.webActor.client.requestJSON<CollaborationResources>(`${lifecycleIssuePath(fixture.issueId)}/resources`);
  const resource = registered.targets.find((item) => item.target_id === receipt.target_id)!;
  expect(resource).toMatchObject({ kind, resource_id: resourceId, authorized: false });
  const issue = await readLifecycleIssue(fixture);
  const channelId = issue.execution!.targets.find((item) => item.id === resource.target_id)?.channel_id;
  if (!channelId) throw new Error("The public registered target has no real lane");
  const target: Target = { ...fixture.targetA, name: resource.name, resourceId, targetId: resource.target_id, channelId };
  const consents: Awaited<ReturnType<typeof consentLaneResource>>[] = [];
  for (const part of resource.resources) {
    expect(part.owner_user_id).toBe(fixture.webActor.id);
    consents.push(await consentLaneResource(page, fixture, target, part.agent_id));
  }
  const authorized = await fixture.webActor.client.requestJSON<CollaborationResources>(`${lifecycleIssuePath(fixture.issueId)}/resources`);
  expect(authorized.targets.find((item) => item.target_id === target.targetId)?.authorized).toBe(true);
  return { target, receipt, requestId: response.request().headers()["idempotency-key"], consents };
}

async function consentLaneResource(page: Page, fixture: LifecycleFixture, target: Target, agentId: string) {
  // Each owner consent advances its own hold observation. Mount a fresh form
  // rather than use a stale sibling consent selection from the prior revision.
  await refreshLifecyclePage(page, fixture);
  await resourcesOpen(page);
  const resources = await fixture.webActor.client.requestJSON<CollaborationResources>(`${lifecycleIssuePath(fixture.issueId)}/resources`);
  const resource = resources.targets.find((item) => item.target_id === target.targetId)!.resources.find((item) => item.agent_id === agentId)!;
  await resourceRoot(page, target.name).getByRole("button", { name: `Authorize ${resource.agent_name}`, exact: true }).click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: `Authorize ${resource.agent_name}`, exact: true }) });
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${lifecycleIssuePath(fixture.issueId)}/targets/${target.targetId}/consents`
    && response.request().method() === "POST");
  await form.getByRole("button", { name: "Confirm authorization", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(201);
  const receipt: CollaborationResourceReceipt = await response.json();
  expect(receipt).toMatchObject({ code: "resource_consented", target_id: target.targetId });
  return { receipt, requestId: response.request().headers()["idempotency-key"], agentId };
}

async function withdrawLaneResource(page: Page, fixture: LifecycleFixture, target: Target, agentId: string) {
  await refreshLifecyclePage(page, fixture);
  await resourcesOpen(page);
  const resources = await fixture.webActor.client.requestJSON<CollaborationResources>(`${lifecycleIssuePath(fixture.issueId)}/resources`);
  const resource = resources.targets.find((item) => item.target_id === target.targetId)!.resources.find((item) => item.agent_id === agentId)!;
  await resourceRoot(page, target.name).getByRole("button", { name: `Withdraw authorization for ${resource.agent_name}`, exact: true }).click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: "Withdraw this authorization?", exact: true }) });
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${lifecycleIssuePath(fixture.issueId)}/grants/${resource.grant_id}/revoke`
    && response.request().method() === "POST");
  await form.getByRole("button", { name: "Withdraw authorization", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  return { receipt: await response.json() as CollaborationResourceReceipt, requestId: response.request().headers()["idempotency-key"] };
}

async function assertLaneVisible(page: Page, issue: CollaborationIssue, target: Target) {
  const current = issue.execution!.targets.find((item) => item.id === target.targetId)!;
  const row = lifecycleTargetRow(page, target.resourceId);
  await expect(row).toBeVisible();
  const lane = row.locator("..").locator("..");
  await expect(lane.locator(":scope > div").getByText(current.control.queue_state === "active" ? "Active" : "Paused", { exact: true })).toBeVisible();
  await expect(row.getByText(current.hold_reason === null ? "Target released" : current.hold_reason === "grant_required"
    ? "Resource authorization required" : "Awaiting initial authorization", { exact: true })).toBeVisible();
  return lane;
}

function assertOtherLane(before: CollaborationIssue, after: CollaborationIssue, target: Target) {
  expect(after.execution!.targets.find((item) => item.id === target.targetId)).toEqual(before.execution!.targets.find((item) => item.id === target.targetId));
  expect(after.execution!.channels.find((item) => item.id === target.channelId)).toEqual(before.execution!.channels.find((item) => item.id === target.channelId));
}

async function observedLaneControl(page: Page, fixture: LifecycleFixture, action: "stop" | "resume", target?: Target, status = 200) {
  // A prior Stop can keep the same visible Paused label while advancing its
  // version. Explicitly reread that baseline before the next human intent;
  // never retry a refused operation with modified observations or a new key.
  await refreshLifecyclePage(page, fixture);
  const issue = await readLifecycleIssue(fixture);
  const observed = await lifecycleControl(page, fixture, action, target, status);
  expect(observed.input.expected_lifecycle_epoch).toBe(issue.lifecycle_epoch);
  if (target) {
    const current = issue.execution!.targets.find((item) => item.id === target.targetId)!;
    expect(observed.input.targets).toEqual([expect.objectContaining({ target_id: target.targetId,
      expected_target_epoch: current.target_epoch, expected_config_version: current.config_version,
      expected_control_version: current.control.control_version })]);
  } else {
    expect(observed.input).toMatchObject({ scope: "issue", expected_control_version: issue.session.control_version,
      expected_target_set_version: issue.execution!.target_set_version });
  }
  return observed;
}

async function sendTypedLaneInput(page: Page, fixture: LifecycleFixture, target: Target, actor: Actor, marker: string,
  client: "web" | "electron", kind: "agent" | "squad" = "agent") {
  if (kind === "agent") return lifecycleComment(page, fixture, target, actor, marker, client);
  const composer = page.getByRole("form", { name: "Comments", exact: true });
  await composer.getByLabel("Add comment", { exact: true }).fill(marker);
  await composer.getByRole("group", { name: "AI targets", exact: true }).getByRole("button", { name: target.name, exact: true }).click();
  const content = await composer.getByLabel("Add comment", { exact: true }).inputValue();
  expect(content).toContain(`mention://squad/${target.resourceId}`);
  const previewPending = page.waitForResponse((response) => new URL(response.url()).pathname === `${lifecycleIssuePath(fixture.issueId)}/comments/trigger-preview`
    && response.request().method() === "POST");
  await composer.getByRole("button", { name: "Preview delivery", exact: true }).click();
  const previewResponse = await previewPending;
  expect(previewResponse.status()).toBe(200);
  const preview: SentCrossClientComment["preview"] = await previewResponse.json();
  expect(preview.targets).toEqual([expect.objectContaining({ target_id: target.targetId, kind, state: "waiting" })]);
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${lifecycleIssuePath(fixture.issueId)}/comments`
    && response.request().method() === "POST");
  await composer.getByRole("button", { name: "Send comment", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(201);
  const receipt: SentCrossClientComment["receipt"] = await response.json();
  const command = receipt.targets[0];
  const requestId = response.request().headers()["idempotency-key"];
  if (!requestId || !command?.command_id || !command.batch_id || !command.command_sequence) throw new Error("Actual squad comment has no retained input identity");
  expect(command).toMatchObject({ target_id: target.targetId, channel_id: target.channelId, state: "waiting" });
  return { client, actorId: actor.id, targetId: target.targetId, content, marker, requestId, receipt,
    commandId: command.command_id, batchId: command.batch_id, sequence: command.command_sequence, preview };
}

export async function exerciseSameLaneControls(seed: Fixture, creatorPage: Page, otherPage: Page, client: "web" | "electron", capture: Capture) {
  const { fixture, independent, peerId, peerBefore } = seed;
  const api = fixture.webActor.client;
  const squad = await api.requestJSON<{ id: string }>("/api/squads", { method: "POST", headers: { "X-Workspace-ID": fixture.targetA.runtime.workspaceId,
    "X-Workspace-Slug": "" }, body: { name: `Same lane squad S ${randomUUID().slice(0, 8)}`, leader_id: fixture.targetA.resourceId } });
  await api.requestJSON(`/api/squads/${squad.id}/members`, { method: "POST", headers: { "X-Workspace-ID": fixture.targetA.runtime.workspaceId,
    "X-Workspace-Slug": "" }, body: { member_type: "agent", member_id: independent.resourceId, role: "member" } });
  const registered = await registerLaneTarget(creatorPage, fixture, "squad", squad.id);
  const targetS = registered.target;
  expect(targetS.channelId).toBe(fixture.targetA.channelId);
  const initial = await readLifecycleIssue(fixture);
  expect(initial.execution!.targets).toHaveLength(4);
  expect(initial.execution!.channels).toHaveLength(3);
  const resumedAll = await observedLaneControl(creatorPage, fixture, "resume");
  expect(resumedAll.receipt).toMatchObject({ code: "resumed", queue_state: "active" });
  const inputA = await sendTypedLaneInput(creatorPage, fixture, fixture.targetA, fixture.webActor, "AC34 direct A remains queued", client);
  const inputS = await sendTypedLaneInput(creatorPage, fixture, targetS, fixture.webActor, "AC34 squad S shares actual A lane", client, "squad");
  const active = await readLifecycleIssue(fixture);
  const stoppedA = await observedLaneControl(creatorPage, fixture, "stop", fixture.targetA);
  expect(stoppedA.receipt.targets.map((target) => target.target_id).sort()).toEqual([fixture.targetA.targetId, targetS.targetId].sort());
  expect(stoppedA.receipt.channels).toEqual([expect.objectContaining({ channel_id: targetS.channelId, queue_state: "paused" })]);
  const pausedA = await readLifecycleIssue(fixture);
  expect(pausedA.session).toEqual(active.session);
  assertOtherLane(active, pausedA, fixture.targetB);
  assertOtherLane(active, pausedA, independent);
  const aliasLane = await assertLaneVisible(creatorPage, pausedA, fixture.targetA);
  await expect(aliasLane).toContainText(targetS.resourceId);
  await expect(aliasLane).toContainText(fixture.targetA.resourceId);
  await expect(aliasLane).toContainText("Squad");
  await assertLaneVisible(creatorPage, pausedA, targetS);
  await capture("same-lane-stop-a-and-s", creatorPage, aliasLane);
  const resumedS = await observedLaneControl(creatorPage, fixture, "resume", targetS);
  expect(resumedS.receipt.targets.map((target) => target.target_id).sort()).toEqual([fixture.targetA.targetId, targetS.targetId].sort());
  const sharedActive = await readLifecycleIssue(fixture);
  assertOtherLane(pausedA, sharedActive, fixture.targetB);
  assertOtherLane(pausedA, sharedActive, independent);
  const withdrawnS = await withdrawLaneResource(creatorPage, fixture, targetS, independent.resourceId);
  const heldS = await readLifecycleIssue(fixture);
  expect(heldS.execution!.targets.find((target) => target.id === targetS.targetId)?.hold_reason).toBe("grant_required");
  expect(heldS.execution!.targets.find((target) => target.id === fixture.targetA.targetId)?.hold_reason).toBeNull();
  const stoppedAgain = await observedLaneControl(creatorPage, fixture, "stop", fixture.targetA);
  const resumedA = await observedLaneControl(creatorPage, fixture, "resume", fixture.targetA);
  const aliasHeld = await readLifecycleIssue(fixture);
  expect(aliasHeld.execution!.targets.find((target) => target.id === targetS.targetId)).toMatchObject({ hold_reason: "grant_required", control: { queue_state: "active" } });
  assertOtherLane(sharedActive, aliasHeld, fixture.targetB);
  await assertLaneVisible(creatorPage, aliasHeld, targetS);
  const reconsentS = await consentLaneResource(creatorPage, fixture, targetS, independent.resourceId);
  expect((await readLifecycleIssue(fixture)).execution!.targets.find((target) => target.id === targetS.targetId)?.hold_reason).toBe("grant_required");
  const explicitResumeS = await observedLaneControl(creatorPage, fixture, "resume", targetS);
  const beforeB = await readLifecycleIssue(fixture);
  const stopB = await observedLaneControl(creatorPage, fixture, "stop", fixture.targetB);
  const pausedB = await readLifecycleIssue(fixture);
  assertOtherLane(beforeB, pausedB, fixture.targetA);
  assertOtherLane(beforeB, pausedB, targetS);
  const resumeB = await observedLaneControl(creatorPage, fixture, "resume", fixture.targetB);
  const independentB = await readLifecycleIssue(fixture);
  assertOtherLane(pausedB, independentB, fixture.targetA);
  assertOtherLane(pausedB, independentB, targetS);
  await assertLaneVisible(creatorPage, independentB, fixture.targetB);
  const wholeStop = await observedLaneControl(creatorPage, fixture, "stop");
  const wholePaused = await readLifecycleIssue(fixture);
  expect(wholePaused.session.queue_state).toBe("paused");
  expect(wholePaused.execution!.channels.every((lane) => lane.queue_state === "paused")).toBe(true);
  const agentD = await api.requestJSON<{ id: string }>("/api/agents", { method: "POST", headers: { "X-Workspace-ID": fixture.targetA.runtime.workspaceId,
    "X-Workspace-Slug": "" }, body: { name: `AC34 new whole-paused target D ${randomUUID().slice(0, 8)}`, runtime_id: fixture.targetA.runtime.runtimeId, permission_mode: "private" } });
  const newTarget = await registerLaneTarget(creatorPage, fixture, "agent", agentD.id);
  const afterRegistration = await readLifecycleIssue(fixture);
  const rejectedNew = await observedLaneControl(creatorPage, fixture, "resume", newTarget.target, 409);
  expect(rejectedNew.receipt.code).toBe("issue_paused");
  expect((await readLifecycleIssue(fixture)).execution).toEqual(afterRegistration.execution);
  const added = await sendTypedLaneInput(creatorPage, fixture, newTarget.target, fixture.webActor, "AC34 input added after whole pause", client);
  expect(added.preview.targets[0]?.waiting_reason).toBe("issue_paused");
  await refreshLifecyclePage(otherPage, fixture);
  const beforeClientSwitch = await readLifecycleIssue(fixture);
  const rejectedOtherClient = await observedLaneControl(otherPage, fixture, "resume", targetS, 409);
  expect(rejectedOtherClient.receipt.code).toBe("issue_paused");
  expect((await readLifecycleIssue(fixture)).execution).toEqual(beforeClientSwitch.execution);
  await expect(panelFor(otherPage).getByRole("status", { name: "Control result", exact: true })).toContainText("Issue paused; explicit resume required");
  await capture("whole-pause-client-switch-refused", otherPage, panelFor(otherPage).getByRole("status", { name: "Control result", exact: true }));
  const finalResume = await observedLaneControl(creatorPage, fixture, "resume");
  expect(finalResume.receipt).toMatchObject({ code: "resumed", queue_state: "active" });
  const final = await readLifecycleIssue(fixture);
  expect(final.execution!.targets.every((target) => target.control.queue_state === "active" && target.hold_reason === null)).toBe(true);
  expect(final.execution!.runs).toEqual([]);
  for (const command of [inputA, inputS, added]) expect(final.execution!.commands.find((item) => item.id === command.commandId)?.state).toBe("waiting");
  await refreshLifecyclePage(otherPage, fixture);
  for (const currentPage of [creatorPage, otherPage]) for (const target of [fixture.targetA, targetS, fixture.targetB, newTarget.target]) await assertLaneVisible(currentPage, final, target);
  expect(await api.requestJSON(lifecycleIssuePath(peerId))).toEqual(peerBefore);
  expect(await api.countIssueDispatches(fixture.issueId)).toBe(0);
  const publicRegistration = (value: typeof registered) => ({ receipt: value.receipt, requestId: value.requestId, consents: value.consents,
    target: { target_id: value.target.targetId, resource_id: value.target.resourceId, channel_id: value.target.channelId, name: value.target.name } });
  return { registered: publicRegistration(registered), inputA, inputS, resumedAll, stoppedA, paused_execution: pausedA.execution, resumedS, withdrawnS, held_execution: heldS.execution,
    stoppedAgain, resumedA, alias_held_execution: aliasHeld.execution, reconsentS, explicitResumeS, stopB, resumeB, wholeStop,
    whole_paused_execution: wholePaused.execution, newTarget: publicRegistration(newTarget), rejectedNew, added, rejectedOtherClient, finalResume, final_execution: final.execution,
    independent_peer_issue: peerId, peer_unchanged: true,
    scope: "Actual public Web/Electron controls and registration/consent with offline test-owned resources; no process/proof inference. Canonical service/Linux scope evidence is separate." };
}

async function closeThroughUI(page: Page, fixture: LifecycleFixture, action: "complete" | "cancel") {
  const label = action === "complete" ? "Complete issue" : "Cancel issue";
  await page.getByRole("button", { name: label, exact: true }).click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: label, exact: true }) });
  await form.getByLabel("Status", { exact: true }).selectOption(action === "complete" ? "done" : "cancelled");
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${lifecycleIssuePath(fixture.issueId)}/lifecycle`
    && response.request().method() === "POST");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  expect(receipt.code).toBe(action === "complete" ? "completed" : "cancelled");
  const requestId = response.request().headers()["idempotency-key"];
  if (!requestId) throw new Error("Lifecycle close omitted its original request identity");
  expect(response.request().postDataJSON()).toMatchObject({ action, status: action === "complete" ? "done" : "cancelled" });
  await expect(form).toHaveCount(0);
  return { receipt, requestId, input: response.request().postDataJSON() as Record<string, unknown> };
}

export async function exerciseLiveClose(seed: Fixture, creatorPage: Page, handlerPage: Page, client: "web" | "electron", action: "complete" | "cancel",
  controls: { proxy: Awaited<ReturnType<typeof createLifecycleRuntimeTransport>>; daemons: Awaited<ReturnType<typeof startLifecycleDaemons>>; capture: Capture }) {
  const { fixture, peerId, peerBefore } = seed;
  const read = () => readLifecycleIssue(fixture);
  const history = () => fixture.webActor.client.requestJSON<CollaborationCommandHistory>(`${lifecycleIssuePath(fixture.issueId)}/commands?limit=100`);
  const otherClient = client === "web" ? "electron" : "web";
  const originalA = await lifecycleComment(handlerPage, fixture, fixture.targetA, fixture.desktopActor, `AC57 ${action} live handler A`, otherClient);
  const visitA = await controls.daemons.waitForVisit("A", 0);
  assertProviderInput(visitA, fixture, fixture.targetA, [originalA]);
  controls.proxy.holdProof(visitA.input.identity.run_id);
  const originalB = await lifecycleComment(creatorPage, fixture, fixture.targetB, fixture.webActor, `AC57 ${action} live creator B`, client);
  const visitB = await controls.daemons.waitForVisit("B", 0);
  assertProviderInput(visitB, fixture, fixture.targetB, [originalB]);
  controls.proxy.holdProof(visitB.input.identity.run_id);
  const waiting: SentCrossClientComment[] = [];
  for (const target of [fixture.targetA, fixture.targetB]) {
    waiting.push(await lifecycleComment(creatorPage, fixture, target, fixture.webActor, `AC57 ${action} creator waits in ${target.lane}`, client));
    waiting.push(await lifecycleComment(handlerPage, fixture, target, fixture.desktopActor, `AC57 ${action} handler waits in ${target.lane}`, otherClient));
  }
  const permitted = await read();
  for (const visit of [visitA, visitB]) expect(permitted.execution!.runs.find((run) => run.id === visit.input.identity.run_id))
    .toMatchObject({ proof_state: "unknown", settled_at: null, forced_disposition: null });
  for (const command of waiting) expect(permitted.execution!.commands.find((item) => item.id === command.commandId)?.state).toBe("waiting");
  await refreshLifecyclePage(creatorPage, fixture);
  const closed = await closeThroughUI(creatorPage, fixture, action);
  for (const visit of [visitA, visitB]) await controls.proxy.waitForHeldProof(visit.input.identity.run_id);
  const closing = await read();
  expect(closing).toMatchObject({ status_category: action === "complete" ? "done" : "cancelled", session: { queue_state: "closed" } });
  expect(closing.execution!.channels.every((lane) => lane.queue_state === "paused")).toBe(true);
  for (const visit of [visitA, visitB]) expect(closing.execution!.runs.find((run) => run.id === visit.input.identity.run_id))
    .toMatchObject({ proof_state: "unknown", settled_at: null, forced_disposition: "stopped", progress_state: "ending" });
  const reason = action === "complete" ? "issue_completed" : "issue_cancelled";
  const reasonLabel = action === "complete" ? "The collaboration was completed" : "The collaboration was cancelled";
  const cancelledHistory = await history();
  for (const command of waiting) {
    const entry = cancelledHistory.items.find((item) => item.id === command.commandId)!;
    expect(entry).toMatchObject({ state: "cancelled", reason, content: command.content, author_user_id: command.actorId,
      target_id: command.targetId, batch_id: command.batchId, seq: command.sequence, group_id: null,
      cancellation: { kind: "automatic", actor_user_id: fixture.webActor.id, event_id: expect.any(String), event_seq: expect.any(String), created_at: expect.any(String) } });
    expect(entry.cancellation!.event_id).not.toBe(entry.input_event_id);
    expect(BigInt(entry.cancellation!.event_seq!)).toBeGreaterThan(BigInt(entry.input_event_seq!));
  }
  for (const currentPage of [creatorPage, handlerPage]) {
    await refreshLifecyclePage(currentPage, fixture);
    await expect(panelFor(currentPage).locator(":scope > p").filter({ hasText: /^Closed$/ })).toBeVisible();
    await expect(currentPage.getByRole("button", { name: "Resume all", exact: true })).toBeDisabled();
    for (const visit of [visitA, visitB]) {
      const disclosure = currentPage.locator(`[id="run-${visit.input.identity.run_id}"]`);
      await expect(disclosure).toContainText("Termination unconfirmed");
      await expect(disclosure).toContainText("Waiting for termination");
      await expect(disclosure).not.toContainText("Termination confirmed");
      await disclosure.getByRole("button", { name: `View records for ${visit.input.identity.run_id}`, exact: true }).click();
      await expect(disclosure.getByRole("region", { name: "Run records", exact: true })).toContainText("Stop accepted; termination remains unconfirmed");
    }
    for (const command of waiting) await showLifecycleHistory(currentPage, command, "Cancelled", reasonLabel);
    await controls.capture(`${action}-closed-proof-unconfirmed`, currentPage, currentPage.locator(`[id="run-${visitB.input.identity.run_id}"]`));
  }
  controls.proxy.releaseProof(visitA.input.identity.run_id);
  await controls.proxy.waitForAcceptedProof(visitA.input.identity.run_id);
  const partialProof = await read();
  expect(partialProof.session).toEqual(closing.session);
  expect(partialProof.execution!.runs.find((run) => run.id === visitB.input.identity.run_id)).toMatchObject({ proof_state: "unknown", settled_at: null });
  expect((await history()).items.filter((item) => waiting.some((command) => command.commandId === item.id)))
    .toEqual(cancelledHistory.items.filter((item) => waiting.some((command) => command.commandId === item.id)));
  controls.proxy.releaseProof(visitB.input.identity.run_id);
  await controls.proxy.waitForAcceptedProof(visitB.input.identity.run_id);
  await expect.poll(async () => (await read()).execution!.runs.every((run) => run.proof_state === "exited_proven" && run.progress_state === "settled"), { timeout: 30_000 }).toBe(true);
  const final = await read();
  expect(final.session).toEqual(closing.session);
  expect(final.status_category).toBe(closing.status_category);
  expect(final.execution!.channels.every((lane) => lane.queue_state === "paused")).toBe(true);
  expect(final.execution!.runs).toHaveLength(2);
  for (const command of [originalA, originalB]) expect(final.execution!.commands.find((item) => item.id === command.commandId)?.state).toBe("stopped");
  const finalHistory = await history();
  for (const command of waiting) expect(finalHistory.items.find((item) => item.id === command.commandId)).toEqual(cancelledHistory.items.find((item) => item.id === command.commandId));
  for (const currentPage of [creatorPage, handlerPage]) {
    await refreshLifecyclePage(currentPage, fixture);
    for (const visit of [visitA, visitB]) await expect(currentPage.locator(`[id="run-${visit.input.identity.run_id}"]`)).toContainText("Termination confirmed");
    for (const command of waiting) await showLifecycleHistory(currentPage, command, "Cancelled", reasonLabel);
    for (const command of [originalA, originalB]) await showLifecycleHistory(currentPage, command, "Stopped");
  }
  for (const visit of [visitA, visitB]) {
    const attempts = controls.proxy.evidence().attempts.filter((attempt) => attempt.path === `/api/collaboration/runtime/runs/${visit.input.identity.run_id}/proof`);
    expect(attempts.some((attempt) => attempt.status === 503)).toBe(true);
    expect(attempts.some((attempt) => attempt.status === 200)).toBe(true);
    expect(new Set(attempts.map((attempt) => attempt.body_sha256)).size).toBe(1);
    expect(visit.released_ms).toBeNull();
  }
  expect(await fixture.webActor.client.requestJSON(lifecycleIssuePath(peerId))).toEqual(peerBefore);
  expect(await fixture.webActor.client.countIssueDispatches(fixture.issueId)).toBe(0);
  return { action, originalA, originalB, waiting, closed, permitted_execution: permitted.execution, closing_execution: closing.execution,
    partial_proof_execution: partialProof.execution, final_execution: final.execution, closed_session: final.session,
    cancelled_history_before_proof: cancelledHistory, cancelled_history_after_proof: finalHistory,
    runtime_transport: controls.proxy.evidence(), provider_observations: controls.daemons.evidence(), peer_unchanged: true,
    scope: "Actual Web/Electron lifecycle UI, two genuine permitted contained Linux synthetic runs, original natural sealed proofs held at transport and subsequently accepted; no real provider account" };
}

export { seedLifecycleControls, createLifecycleRuntimeTransport, startLifecycleDaemons };
