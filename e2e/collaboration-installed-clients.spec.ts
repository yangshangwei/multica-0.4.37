import { randomUUID } from "node:crypto";
import { expect, test } from "@playwright/test";
import type {
  CollaborationCommandHistory, CollaborationCommentReceipt, CollaborationControlInput, CollaborationControlReceipt,
  CollaborationIssue, CollaborationOperation, CollaborationParticipants, CollaborationResources,
} from "@multica/core/collaboration";
import { controlEnvironmentReady, seedCollaborationControlJourney } from "./fixtures/collaboration-control-journey";
import { invokeInstalledCLI, verifyInstalledCLI } from "./fixtures/collaboration-installed-clients";

function authority(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("Current participant authority is required");
  return { expected_lifecycle_epoch: issue.lifecycle_epoch, expected_participant_epoch: issue.write_context.participant_epoch,
    expected_account_epoch: issue.write_context.account_epoch };
}

function controlInput(issue: CollaborationIssue, action: "stop" | "resume"): CollaborationControlInput {
  const execution = issue.execution;
  if (!execution) throw new Error("Current execution observations are required");
  const targets = execution.targets.map((target) => ({ target_id: target.id, expected_target_epoch: target.target_epoch,
    expected_config_version: target.config_version, expected_control_version: target.control.control_version,
    observed_group_id: target.active_group_id, observed_group_generation: target.group_generation }));
  const base = { ...authority(issue), scope: "issue" as const, expected_control_version: execution.control.control_version,
    expected_target_set_version: execution.target_set_version };
  return action === "stop" ? { ...base, action, targets }
    : { ...base, action, expected_execution_epoch: execution.execution_gate.epoch,
      targets: targets.map((target, index) => ({ ...target, expected_hold_version: execution.targets[index]!.hold_version })) };
}

test("cached v0.6.7 CLI cannot bypass paused, unconfirmed-stop or removed-author commands", async ({ browserName: _browserName }, info) => {
  test.skip(!controlEnvironmentReady() || !process.env.COLLABORATION_INSTALLED_CLI_RELAY,
    "Requires the existing isolated password API, exact cached OCI package and explicit Linux stdio relay");
  test.setTimeout(240_000);
  const provenance = await verifyInstalledCLI();
  const fixture = await seedCollaborationControlJourney();
  const root = `/api/collaboration/issues/${fixture.issueId}`;
  const read = () => fixture.creator.requestJSON<CollaborationIssue>(root);
  const commandHistory = () => fixture.creator.requestJSON<CollaborationCommandHistory>(`${root}/commands?limit=100`);
  const controls: { requestId: string; input: CollaborationControlInput; receipt: CollaborationControlReceipt }[] = [];
  const executions: { stage: string; actor: string; evidence: unknown }[] = [];
  const snapshot = async () => {
    const issue = await read();
    return { issue: { id: issue.id, creator: issue.creator, handler: issue.handler, collaboration_revision: issue.collaboration_revision,
      lifecycle_epoch: issue.lifecycle_epoch, status: issue.status, status_category: issue.status_category,
      session: issue.session, execution: issue.execution },
    history: await commandHistory(),
    participants: await fixture.creator.requestJSON<CollaborationParticipants>(`${root}/participants`),
    resources: await fixture.creator.requestJSON<CollaborationResources>(`${root}/resources`),
    ordinary_dispatches: await fixture.owner.countIssueDispatches(fixture.issueId) };
  };
  const control = async (action: "stop" | "resume") => {
    const issue = await read();
    const input = controlInput(issue, action);
    const requestId = randomUUID();
    const receipt = await fixture.creator.requestJSON<CollaborationControlReceipt>(`${root}/control`, {
      method: "POST", headers: { "Idempotency-Key": requestId }, body: input });
    controls.push({ requestId, input, receipt });
    return receipt;
  };
  const addInput = async (actor: typeof fixture.creator, marker: string) => {
    const issue = await actor.requestJSON<CollaborationIssue>(root);
    const target = issue.execution!.targets.find((item) => item.id === fixture.agentA.targetId)!;
    const requestId = randomUUID();
    const receipt = await actor.requestJSON<CollaborationCommentReceipt>(`${root}/comments`, { method: "POST",
      headers: { "Idempotency-Key": requestId }, body: { ...authority(issue), content: `[@A](mention://agent/${fixture.agentA.resourceId}) ${marker}`,
        target_versions: [{ target_id: target.id, expected_target_epoch: target.target_epoch, expected_config_version: target.config_version }],
        attachment_ids: [], parent_id: null, suppressed_target_ids: [] } });
    expect(receipt).toMatchObject({ status: 201, code: "comment_created", issue_id: fixture.issueId });
    const targetResult = receipt.targets[0];
    if (!targetResult?.command_id || !targetResult.batch_id) throw new Error("The public comment must retain one exact queued input");
    return { requestId, receipt, commandId: targetResult.command_id, batchId: targetResult.batch_id };
  };
  const oldCommands = async (stage: string, actor: typeof fixture.creator, actorId: string, workspaceId = fixture.sourceWorkspace.id) => {
    const before = await snapshot();
    for (const [args, method, path] of [
      [["issue", "update", fixture.issueId, "--status", "done", "--output", "json"], "PUT", `/api/issues/${fixture.issueId}`],
      [["issue", "comment", "add", fixture.issueId, "--content", `old CLI ${stage} must remain inert`, "--output", "json"], "POST", `/api/issues/${fixture.issueId}/comments`],
      [["issue", "rerun", fixture.issueId, "--output", "json"], "POST", `/api/issues/${fixture.issueId}/rerun`],
    ] as const) {
      const result = await invokeInstalledCLI({ apiUrl: fixture.apiUrl, token: actor.getToken()!, workspaceId,
        args: [...args], expected: { method, path } });
      expect(result.exitCode).not.toBe(0);
      expect(result.requests[0]!.status).toBe(409);
      expect(await snapshot()).toEqual(before);
      executions.push({ stage, actor: actorId, evidence: result.evidence });
    }
    return before;
  };
  try {
    // A successful ordinary command proves authentication, routing and the
    // exact old executable work; refusals are not a broken relay or fake CLI.
    const ordinary = await fixture.creator.requestJSON<{ id: string }>("/api/issues", { method: "POST", body: {
      title: `Ordinary cached CLI positive ${randomUUID()}`, status: "todo", priority: "none" } });
    const positive = await invokeInstalledCLI({ apiUrl: fixture.apiUrl, token: fixture.creator.getToken()!, workspaceId: fixture.sourceWorkspace.id,
      args: ["issue", "update", ordinary.id, "--status", "done", "--output", "json"], expected: { method: "PUT", path: `/api/issues/${ordinary.id}` } });
    expect(positive.exitCode).toBe(0);
    expect(positive.requests[0]!.status).toBe(200);
    expect(await fixture.creator.requestJSON(`/api/issues/${ordinary.id}`)).toMatchObject({ id: ordinary.id, status: "done" });
    executions.push({ stage: "ordinary-positive", actor: fixture.creatorId, evidence: positive.evidence });

    const pausedInput = await addInput(fixture.creator, "Paused queue original input");
    const paused = await read();
    expect(paused.session.queue_state).toBe("paused");
    expect(paused.execution!.commands.find((item) => item.id === pausedInput.commandId)).toMatchObject({ state: "waiting" });
    const pausedBefore = await oldCommands("paused", fixture.creator, fixture.creatorId);

    expect(await control("resume")).toMatchObject({ code: "resumed", queue_state: "active" });
    const pending = await fixture.simulatedDaemon.permitPendingInput(fixture.issueId, fixture.agentA, pausedInput.commandId, pausedInput.batchId);
    expect((await read()).execution!.runs.find((run) => run.id === pending.runId)).toMatchObject({ progress_state: "launch_authorized" });
    expect(await control("stop")).toMatchObject({ code: "stop_requested", queue_state: "paused", group_settled: false });
    const unconfirmed = await read();
    expect(unconfirmed.execution!.runs.find((run) => run.id === pending.runId)).toMatchObject({ proof_state: "unknown", settled_at: null });
    const unconfirmedBefore = await oldCommands("stop-unconfirmed", fixture.creator, fixture.creatorId);

    const removedInput = await addInput(fixture.handler, "Removed author's unconsumed original input");
    // Keep source membership valid to reach the actual old write boundary
    // after collaboration removal, rather than fail in tenant middleware.
    const share = await fixture.creator.requestJSON<{ code: string }>(`/api/workspaces/${fixture.sourceWorkspace.id}/share-links`, {
      method: "POST", body: { role: "member", max_uses: 1 } });
    await fixture.handler.requestJSON("/api/share-links/join", { method: "POST", body: { code: share.code } });
    const removalBaseline = await read();
    const removalKey = randomUUID();
    const removal = await fixture.creator.requestJSON<CollaborationOperation>(`${root}/participants/${fixture.handlerId}/remove`, {
      method: "POST", headers: { "Idempotency-Key": removalKey },
      body: { ...authority(removalBaseline), expected_collaboration_revision: removalBaseline.collaboration_revision } });
    expect(removal.code).toBe("participant_removed");
    const cancelled = (await commandHistory()).items.find((item) => item.id === removedInput.commandId);
    expect(cancelled).toMatchObject({ state: "cancelled", reason: "participant_removed", author_user_id: fixture.handlerId,
      cancellation: { kind: "automatic", actor_user_id: fixture.creatorId } });
    const removedBefore = await oldCommands("participant-removed", fixture.handler, fixture.handlerId);
    expect((await commandHistory()).items.find((item) => item.id === removedInput.commandId)).toEqual(cancelled);
    for (const original of controls) {
      expect(await fixture.creator.requestJSON(`${root}/control/operations/${original.requestId}`)).toEqual(original.receipt);
    }
    const removedRecovery = await fetch(`${fixture.apiUrl}${root}/comments/operations/${removedInput.requestId}`, {
      headers: { Authorization: `Bearer ${fixture.handler.getToken()!}` } });
    expect(removedRecovery.status).toBe(200);
    // Original-key content recovery has its own wire shape. The surviving
    // source read basis may retain the original receipt; no fabricated
    // redacted flag or request ID is assumed.
    const ownReceipt = await removedRecovery.json();
    expect(ownReceipt).toEqual(removedInput.receipt);
    expect(ownReceipt).not.toHaveProperty("content");
    expect((await read()).execution).toEqual(removedBefore.issue.execution);
    expect((await read()).session.queue_state).toBe("paused");
    expect(await fixture.owner.countIssueDispatches(fixture.issueId)).toBe(0);
    await info.attach("installed-old-cli-boundary", { contentType: "application/json", body: Buffer.from(JSON.stringify({
      provenance, issue_id: fixture.issueId, paused_input: pausedInput.commandId, pending_permit: pending,
      paused: pausedBefore, unconfirmed: unconfirmedBefore, removed: removedBefore,
      cancelled_original: cancelled, executions, controls, removal_request_id: removalKey,
      scope: "Actual cached old CLI HTTP writes; synthetic public Claim/Permit with proof withheld; no daemon/provider process",
      full_installed_desktop_or_daemon_matrix: false, production_gate_change: false, provider_account_access: false,
    }, null, 2)) });
  } finally {
    // An unconfirmed permit/barrier remains real. Never invent a proof during
    // teardown; the fixture only marks its own synthetic runtime offline.
    await fixture.simulatedDaemon.disconnect();
  }
});
