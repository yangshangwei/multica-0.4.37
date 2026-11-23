import { randomUUID } from "node:crypto";
import pg from "pg";
import { expect, type Page } from "@playwright/test";
import type { CollaborationComment, CollaborationCommentReceipt, CollaborationComments, CollaborationIssue,
  CollaborationOperation, CollaborationRunHandoff, CollaborationRunRecords } from "@multica/core/collaboration";
import { seedOfflineHandoff, type HandoffActor } from "./collaboration-offline-handoff";

function observations(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("The fixture needs current human write authority");
  return { expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
}

/** Human setup uses password sessions and public APIs. Only completed run history
 * is seeded, following collaboration-records.ts; this proves authorized HTTP/UI
 * projection, not checkpoint ingestion, provider execution, or termination proof.
 */
export async function seedRuntimeHandoff(options: { olderComments?: number; observedThrough?: number | null } = {}) {
  const fixture = await seedOfflineHandoff();
  const root = `/api/collaboration/issues/${fixture.issueId}`;
  const before = await fixture.creator.client.requestJSON<CollaborationIssue>(root);
  const handoffNote = "Continue from the retained database decision; verify the migration before rollout.";
  const transferred = await fixture.creator.client.requestJSON<CollaborationOperation>(`${root}/handoffs`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { ...observations(before),
      target_user_id: fixture.recipient.id, purpose: "transfer", note: handoffNote },
  });
  expect(transferred.code).toBe("transferred");
  const issue = await fixture.recipient.client.requestJSON<CollaborationIssue>(root);
  expect(issue.handler.user_id).toBe(fixture.recipient.id);
  const commentInput = observations(issue);
  const postComment = async (content: string) => {
    const receipt = await fixture.recipient.client.requestJSON<CollaborationCommentReceipt>(`${root}/comments`, {
      method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { content, target_versions: [],
        expected_lifecycle_epoch: commentInput.expected_lifecycle_epoch, expected_participant_epoch: commentInput.expected_participant_epoch,
        expected_account_epoch: commentInput.expected_account_epoch },
    });
    expect(receipt.targets).toEqual([]);
    return receipt.comment_id;
  };
  const olderCommentIds: string[] = [];
  for (let index = 0; index < (options.olderComments ?? 0); index++) {
    olderCommentIds.push(await postComment(`Retained discussion ${index + 1}: verify the existing migration before rollout.`));
  }
  // Go's safe JSON encoding expands ampersands to six bytes. This stays within
  // the write request limit while exceeding the serialized comment-page budget.
  const longContent = `Long source begins. ${"&".repeat(12_000)} Source end: recover the original migration decision.`;
  expect(Buffer.byteLength(longContent)).toBeLessThanOrEqual(65_536);
  const longCommentId = await postComment(longContent);

  const sessionId = issue.session.id;
  const targetId = randomUUID(), channelId = randomUUID(), batchId = randomUUID(), commandId = randomUUID(), groupId = randomUUID();
  const leaderId = randomUUID(), memberId = randomUUID(), runtimeId = randomUUID(), grantId = randomUUID();
  const installationId = randomUUID(), bindingId = randomUUID(), priorRunId = randomUUID(), runId = randomUUID(), streamId = randomUUID();
  const input = "Original requester: preserve the database decision and verify its migration.";
  const priorResult = "Earlier provider result: use PostgreSQL advisory locks; Redis was rejected because its state is not durable.";
  const checkpointBodies = ["Retained finding: the migration needs CREATE INDEX CONCURRENTLY.", "Retained next step: check the existing deployment before rollout."];
  const roster = [leaderId, memberId].map((agent, index) => ({ role: index === 0 ? "leader" : "member", agent_id: agent,
    runtime_id: runtimeId, execution_workspace_id: fixture.workspace.id, owner_user_id: fixture.creator.id, grant_id: grantId, grant_epoch: "1" }));
  const prompt = [{ command_id: commandId, author_user_id: fixture.creator.id, participant_epoch: "1", account_epoch: "1",
    lifecycle_epoch: issue.lifecycle_epoch, body: input, attachment_ids: [] }];
  const identity = { deployment_id: randomUUID(), installation_id: installationId, issue_id: fixture.issueId, session_id: sessionId,
    target_id: targetId, channel_id: channelId, batch_id: batchId, group_id: groupId, run_id: runId,
    fence: "1", claim_generation: "1", launch_nonce: randomUUID() };
  const attempt = { version: 1, identity, attempt_seq: "1", attempt_nonce: randomUUID(), stream_id: streamId };
  const observedThrough = options.observedThrough === undefined ? 5 : options.observedThrough;
  const database = new pg.Client(process.env.DATABASE_URL);
  await database.connect();
  try {
    await database.query("BEGIN");
    await database.query("INSERT INTO collaboration_channel (id,session_id,root_agent_id,group_generation,next_batch_seq,next_command_seq,pause_reason) VALUES ($1,$2,$3,1,2,2,'fixture_paused')", [channelId, sessionId, leaderId]);
    await database.query("INSERT INTO collaboration_target (id,session_id,kind,resource_id,roster,channel_id,group_generation,pause_reason) VALUES ($1,$2,'squad',$3,$4::jsonb,$5,1,'fixture_paused')", [targetId, sessionId, randomUUID(), JSON.stringify(roster), channelId]);
    await database.query("INSERT INTO collaboration_batch (id,session_id,channel_id,effective_target_id,seq,state,root_command_id,group_id,started_at) VALUES ($1,$2,$3,$4,1,'in_flight',$5,$6,now())", [batchId, sessionId, channelId, targetId, commandId, groupId]);
    await database.query("INSERT INTO collaboration_command (id,session_id,target_id,input_event_id,channel_id,batch_id,group_id,seq,author_user_id,participant_epoch,author_auth_epoch,lifecycle_epoch,content,attachments,state) VALUES ($1,$2,$3,$4,$5,$6,$7,1,$8,1,1,$9,$10,'[]'::jsonb,'in_flight')", [commandId, sessionId, targetId, randomUUID(), channelId, batchId, groupId, fixture.creator.id, issue.lifecycle_epoch, input]);
    await database.query("INSERT INTO collaboration_execution_group (id,session_id,target_id,command_id,generation,target_epoch,config_version,roster,channel_id,batch_id,state) VALUES ($1,$2,$3,$4,1,1,1,$5::jsonb,$6,$7,'active')", [groupId, sessionId, targetId, commandId, JSON.stringify(roster), channelId, batchId]);
    for (const run of [{ id: priorRunId, role: "leader", parent: null }, { id: runId, role: "continuation", parent: priorRunId }]) {
      await database.query(`INSERT INTO collaboration_run
        (id,session_id,target_id,group_id,command_id,step_key,step_kind,parent_run_id,agent_id,target_epoch,config_version,
         grant_id,grant_epoch,resource_epoch,execution_workspace_id,runtime_id,installation_id,binding_id,binding_epoch,
         channel_id,batch_id,resource_owner_user_id,progress_state,business_outcome,proof_state,authorized_attempt_seq,sealed_attempt_seq,started_at,settled_at,prompt_snapshot,recording_incomplete)
        VALUES ($1,$2,$3,$4,$5,$6,$6,$7,$8,1,1,$9,1,1,$10,$11,$12,$13,1,$14,$15,$16,'settled','failure','exited_proven',1,1,now(),now(),$17::jsonb,$18)`,
      [run.id, sessionId, targetId, groupId, commandId, run.role, run.parent, leaderId, grantId, fixture.workspace.id, runtimeId,
        installationId, bindingId, channelId, batchId, fixture.creator.id, JSON.stringify(prompt), run.id === runId]);
    }
    await database.query("INSERT INTO collaboration_group_input (group_id,command_id,channel_id,batch_id,seq,consuming_run_id) VALUES ($1,$2,$3,$4,1,$5)", [groupId, commandId, channelId, batchId, priorRunId]);
    await database.query("UPDATE collaboration_command SET state='failed' WHERE id=$1", [commandId]);
    await database.query("UPDATE collaboration_batch SET state='failed',settled_at=now() WHERE id=$1", [batchId]);
    await database.query("UPDATE collaboration_execution_group SET state='failed',expansion_sealed_at=now(),settled_at=now() WHERE id=$1", [groupId]);
    for (const event of [
      { run: priorRunId, seq: 1, kind: "outcome", payload: { identity: { issue_id: fixture.issueId, run_id: priorRunId }, outcome: "failure", body: priorResult } },
      { run: runId, seq: 1, kind: "record_checkpoint", payload: { ...attempt, sequence: 1, message_offset: 0,
        messages: checkpointBodies.map(body => ({ kind: "message", role: "assistant", body })) } },
      { run: runId, seq: 2, kind: "recording_seal", payload: { ...attempt, sequence: 1, observed_through: observedThrough, acknowledged_through: 2, reason: "stopped" } },
    ]) {
      await database.query("INSERT INTO collaboration_run_event (issue_id,run_id,producer_event_id,seq,kind,payload) VALUES ($1,$2,$3,$4,$5,$6::jsonb)",
        [fixture.issueId, event.run, `retained-fixture:${event.kind}`, event.seq, event.kind, JSON.stringify(event.payload)]);
    }
    await database.query("COMMIT");
  } catch (error) { await database.query("ROLLBACK"); throw error; }
  finally { await database.end(); }

  const handoffPath = `${root}/runs/${runId}/handoff?execution_view_version=6`;
  const recordsPath = `${root}/runs/${runId}/records?execution_view_version=6`;
  const handoff = await fixture.recipient.client.requestJSON<CollaborationRunHandoff>(handoffPath);
  const records = await fixture.recipient.client.requestJSON<CollaborationRunRecords>(recordsPath);
  expect(handoff).toMatchObject({ issue_id: fixture.issueId, run_id: runId, mode: "live",
    authority: { handler_id: fixture.recipient.id }, handoff: { purpose: "transfer", from_id: fixture.creator.id, to_id: fixture.recipient.id } });
  expect(handoff.results.items.find(item => item.run_id === priorRunId)).toMatchObject({ result_state: "present", source_kind: "provider_result", final_answer_state: "unknown" });
  expect(records.recording).toMatchObject({ status: "partial", retention: { retained_through: 2, observed_through: observedThrough, sealed: true, reason: "stopped" } });
  expect(records.items.filter(item => item.kind === "message").map(item => item.id)).toEqual([`record:${streamId}:0`, `record:${streamId}:1`]);
  return { ...fixture, root, runId, priorRunId, groupId, streamId, input, priorResult, checkpointBodies, handoffNote, observedThrough,
    longCommentId, longContent, olderCommentIds, totalComments: olderCommentIds.length + 1, handoffPath, recordsPath, handoff, records,
    path: `${fixture.accountPath}#run-${runId}` };
}
export type RuntimeHandoffFixture = Awaited<ReturnType<typeof seedRuntimeHandoff>>;

export async function assertRuntimeHandoff(page: Page, fixture: RuntimeHandoffFixture) {
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  const records = page.locator(`[id="run-${fixture.runId}"]`).getByRole("region", { name: "Run records", exact: true });
  const handoff = records.getByRole("region", { name: "Handoff context", exact: true });
  await expect(handoff).toBeVisible();
  await expect(handoff.getByText("Current source context.", { exact: true })).toBeVisible();
  await expect(handoff.getByText(fixture.handoffNote, { exact: true })).toBeVisible();
  await expect(handoff.getByText(fixture.priorResult, { exact: true })).toBeVisible();
  await expect(handoff.getByText("No result recorded", { exact: true })).toBeVisible();
  await expect(handoff.getByText("Earlier reasons were not recorded.", { exact: true })).toBeVisible();
  const handler = handoff.locator("dl > div").filter({ has: page.getByText("Handler", { exact: true }) }).locator("dd");
  await expect(handler).toContainText(fixture.recipient.name);
  await expect(handler).toContainText(fixture.recipient.id);
  await expect(records.getByText(fixture.observedThrough === null ? "Retained 2 messages. The remaining recording is unknown."
    : `Retained 2 of ${fixture.observedThrough} observed messages.`, { exact: true })).toBeVisible();
  await expect(records.getByText("Partial recording", { exact: true })).toBeVisible();
  await expect(records.getByText(fixture.input, { exact: true })).toBeVisible();
  for (const body of fixture.checkpointBodies) await expect(records.getByText(body, { exact: true })).toBeVisible();
  return { records, handoff };
}

export async function recoverLongComment(page: Page, fixture: RuntimeHandoffFixture) {
  const comments = page.getByRole("heading", { name: "Comments", exact: true }).locator("..");
  await expect(comments.getByText(`Loaded 1 of ${fixture.totalComments} comments`, { exact: true })).toBeVisible();
  const comment = page.locator(`[id="comment-${fixture.longCommentId}"]`);
  await expect(comment.getByText("This comment is longer than the page budget. Load its full text to read or edit it.", { exact: true })).toBeVisible();
  await expect(comment.getByRole("button", { name: "Edit comment", exact: true })).toHaveCount(0);
  const pending = page.waitForResponse(response => new URL(response.url()).pathname === `${fixture.root}/comments/${fixture.longCommentId}` && response.request().method() === "GET");
  await comment.getByRole("button", { name: "Load full text", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const full: CollaborationComment = await response.json();
  expect(full).toMatchObject({ id: fixture.longCommentId, content: fixture.longContent });
  await expect(comment.getByText(fixture.longContent, { exact: true })).toBeVisible();
  await expect(comment.getByRole("button", { name: "Edit comment", exact: true })).toBeVisible();
  return { comments, comment, full };
}

export async function readCommentPage(fixture: RuntimeHandoffFixture, cursor?: string) {
  const query = new URLSearchParams({ limit: "50", max_bytes: "65536" });
  if (cursor) query.set("cursor", cursor);
  return fixture.recipient.client.requestJSON<CollaborationComments>(`${fixture.root}/comments?${query}`);
}

export async function removeRuntimeRecipient(fixture: RuntimeHandoffFixture, actor: HandoffActor = fixture.creator) {
  const current = await actor.client.requestJSON<CollaborationIssue>(fixture.root);
  return actor.client.requestJSON<CollaborationOperation>(`${fixture.root}/participants/${fixture.recipient.id}/remove`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: observations(current),
  });
}
