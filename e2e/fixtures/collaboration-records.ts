import { randomUUID } from "node:crypto";
import pg from "pg";
import { expect, type Page } from "@playwright/test";
import { TestApiClient } from "../fixtures";

type IssueReference = { id: string; title: string; identifier: string };
type Snapshot = { collaboration_revision: string; lifecycle_epoch: string; write_context: { participant_epoch: string; account_epoch: string } };

export interface CollaborationRecordFixture {
  source: TestApiClient;
  member: TestApiClient;
  viewer: TestApiClient;
  viewerId: string;
  workspace: { id: string; slug: string; name: string };
  issue: IssueReference;
  child: IssueReference;
  runId: string;
  leaderRunId: string;
  groupId: string;
  attachmentId: string;
  input: string;
  result: string;
}

/** Public human setup plus deliberately seeded, retained run history.
 * This fixture proves UI/HTTP reads, not runtime ingestion or settlement.
 * No engine task, live runtime, executable or provider account is created.
 */
export async function seedCollaborationRecordJourney(): Promise<CollaborationRecordFixture> {
  for (const value of [process.env.COLLABORATION_E2E_API_URL, process.env.DATABASE_URL]) {
    if (!value || !["127.0.0.1", "localhost"].includes(new URL(value).hostname)) throw new Error("Record fixtures require explicitly isolated loopback endpoints");
  }
  const key = randomUUID().slice(0, 10);
  const source = new TestApiClient();
  await source.login(`record-source-${key}@example.invalid`, "Record source owner");
  const workspace = await source.ensureWorkspace("Record source workspace", `record-source-${key}`);
  await source.markUserOnboarded();
  const ownerId = (await source.requestJSON<{ id: string }>("/api/me")).id;
  const member = new TestApiClient();
  const memberEmail = `record-member-${key}@example.invalid`;
  await member.login(memberEmail, "Source member outside collaboration");
  await member.markUserOnboarded();
  const invitation = await source.requestJSON<{ id: string }>(`/api/workspaces/${workspace.id}/members`, { method: "POST", body: { email: memberEmail, role: "member" } });
  await member.requestJSON(`/api/invitations/${invitation.id}/accept`, { method: "POST" });
  member.setWorkspaceId(workspace.id);
  member.setWorkspaceSlug(workspace.slug);
  const viewer = new TestApiClient();
  await viewer.login(`record-viewer-${key}@example.invalid`, "Outside record viewer");
  await viewer.markUserOnboarded();
  const viewerId = (await viewer.requestJSON<{ id: string }>("/api/me")).id;

  const draftId = randomUUID();
  const upload = new FormData();
  upload.set("draft_id", draftId);
  upload.set("source_workspace_id", workspace.id);
  upload.set("file", new Blob(["Frozen attachment for the recorded input."], { type: "text/plain" }), "recorded-input.txt");
  const uploaded = await fetch(`${process.env.COLLABORATION_E2E_API_URL}/api/collaboration/attachments/drafts`, { method: "POST", headers: {
    Authorization: `Bearer ${source.getToken()!}`, "Idempotency-Key": randomUUID(),
  }, body: upload });
  if (!uploaded.ok) throw new Error(`Record fixture upload refused (${uploaded.status})`);
  const file: { id: string } = await uploaded.json();
  const created = await source.requestJSON<{ id: string }>("/api/collaboration/issues", { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    source_workspace_id: workspace.id, handler_user_id: viewerId, title: `Recorded collaboration ${key}`,
    description: "Shared description with independently authorized sub-issues and retained execution history.",
    draft_id: draftId, attachment_ids: [file.id],
  } });
  const issue = await source.requestJSON<IssueReference>(`/api/issues/${created.id}`);
  const child: IssueReference = await member.createIssue(`Ordinary child ${key}`, { parent_issue_id: issue.id,
    description: "PRIVATE_ORDINARY_CHILD_BODY_DO_NOT_PROJECT", status: "todo" });

  const sessionId = (await source.requestJSON<{ session: { id: string } }>(`/api/collaboration/issues/${issue.id}`)).session.id;
  const targetId = randomUUID(), channelId = randomUUID(), batchId = randomUUID(), commandId = randomUUID(), groupId = randomUUID();
  const squadId = randomUUID(), leaderAgentId = randomUUID(), memberAgentId = randomUUID(), runtimeId = randomUUID(), grantId = randomUUID();
  const installationId = randomUUID(), bindingId = randomUUID(), leaderRunId = randomUUID(), memberRunId = randomUUID(), runId = randomUUID();
  const input = "Frozen input read by this run before later issue edits.";
  const result = "Recorded final result: review found an unresolved issue.";
  const roster = [
    { role: "leader", agent_id: leaderAgentId, runtime_id: runtimeId, execution_workspace_id: workspace.id, owner_user_id: ownerId, grant_id: grantId, grant_epoch: "1" },
    { role: "member", agent_id: memberAgentId, runtime_id: randomUUID(), execution_workspace_id: workspace.id, owner_user_id: ownerId, grant_id: randomUUID(), grant_epoch: "1" },
  ];
  const targetRoster = roster.map((resource) => ({ role: resource.role, agent_id: resource.agent_id, runtime_id: resource.runtime_id,
    execution_workspace_id: resource.execution_workspace_id, owner_user_id: resource.owner_user_id }));
  const prompt = [{ command_id: commandId, author_user_id: viewerId, participant_epoch: "1", account_epoch: "1", lifecycle_epoch: "1", body: input, attachment_ids: [file.id] }];
  const messages: Array<Record<string, unknown>> = Array.from({ length: 58 }, (_, index) => ({ kind: "message", role: "assistant",
    body: `Recorded step ${index + 1}. ${index === 1 ? "long_recorded_identifier_".repeat(12) : "Source-owned process content from a synthetic UI fixture."}` }));
  messages.push({ kind: "tool_call", call_id: "fixture-call", name: "read_file", body: '{"path":"delivered-input.txt"}' },
    { kind: "tool_result", call_id: "fixture-call", body: "Recorded tool result. This text is displayed without execution.", is_error: false });
  const db = new pg.Client(process.env.DATABASE_URL);
  await db.connect();
  try {
    await db.query("BEGIN");
    await db.query("INSERT INTO collaboration_channel (id,session_id,root_agent_id,group_generation,next_batch_seq,next_command_seq,pause_reason) VALUES ($1,$2,$3,1,2,2,'fixture_paused')", [channelId, sessionId, leaderAgentId]);
    await db.query("INSERT INTO collaboration_target (id,session_id,kind,resource_id,roster,channel_id,group_generation,pause_reason) VALUES ($1,$2,'squad',$3,$4::jsonb,$5,1,'fixture_paused')", [targetId, sessionId, squadId, JSON.stringify(targetRoster), channelId]);
    await db.query("INSERT INTO collaboration_batch (id,session_id,channel_id,effective_target_id,seq,state,root_command_id,group_id,started_at) VALUES ($1,$2,$3,$4,1,'in_flight',$5,$6,now())", [batchId, sessionId, channelId, targetId, commandId, groupId]);
    await db.query("INSERT INTO collaboration_command (id,session_id,target_id,input_event_id,channel_id,batch_id,group_id,seq,author_user_id,participant_epoch,author_auth_epoch,lifecycle_epoch,content,attachments,state) VALUES ($1,$2,$3,$4,$5,$6,$7,1,$8,1,1,1,$9,$10::jsonb,'in_flight')", [commandId, sessionId, targetId, randomUUID(), channelId, batchId, groupId, viewerId, input, JSON.stringify([file.id])]);
    await db.query("INSERT INTO collaboration_execution_group (id,session_id,target_id,command_id,generation,target_epoch,config_version,roster,channel_id,batch_id,state) VALUES ($1,$2,$3,$4,1,1,1,$5::jsonb,$6,$7,'active')", [groupId, sessionId, targetId, commandId, JSON.stringify(roster), channelId, batchId]);
    for (const run of [
      { id: leaderRunId, role: "leader", parent: null, agent: leaderAgentId, outcome: "success", prompt },
      { id: memberRunId, role: "member", parent: leaderRunId, agent: memberAgentId, outcome: "failure", prompt: null },
      { id: runId, role: "continuation", parent: leaderRunId, agent: leaderAgentId, outcome: "failure", prompt },
    ]) {
      await db.query(`INSERT INTO collaboration_run
        (id,session_id,target_id,group_id,command_id,step_key,step_kind,parent_run_id,agent_id,target_epoch,config_version,
         grant_id,grant_epoch,resource_epoch,execution_workspace_id,runtime_id,installation_id,binding_id,binding_epoch,
         channel_id,batch_id,resource_owner_user_id,progress_state,business_outcome,proof_state,authorized_attempt_seq,sealed_attempt_seq,started_at,settled_at,prompt_snapshot)
        VALUES ($1,$2,$3,$4,$5,$6,$6,$7,$8,1,1,$9,1,1,$10,$11,$12,$13,1,$14,$15,$16,'settled',$17,'exited_proven',1,1,now(),now(),$18::jsonb)`,
      [run.id, sessionId, targetId, groupId, commandId, run.role, run.parent, run.agent, grantId, workspace.id, runtimeId, installationId, bindingId, channelId, batchId, ownerId, run.outcome, run.prompt === null ? null : JSON.stringify(run.prompt)]);
    }
    // Consumption is recorded while the group can still read inputs. Freeze
    // that relationship before seeding terminal history; the transaction keeps
    // these intermediate fixture states invisible to schedulers and clients.
    await db.query("INSERT INTO collaboration_group_input (group_id,command_id,channel_id,batch_id,seq,consuming_run_id) VALUES ($1,$2,$3,$4,1,$5)", [groupId, commandId, channelId, batchId, leaderRunId]);
    await db.query("UPDATE collaboration_command SET state='failed' WHERE id=$1", [commandId]);
    await db.query("UPDATE collaboration_batch SET state='failed',settled_at=now() WHERE id=$1", [batchId]);
    await db.query("UPDATE collaboration_execution_group SET state='failed',expansion_sealed_at=now(),settled_at=now() WHERE id=$1", [groupId]);
    const resource = { provider: "Synthetic provider", model: "fixture-model", account_source: "runtime_configuration" };
    await db.query("INSERT INTO collaboration_run_event (issue_id,run_id,producer_event_id,seq,kind,payload) VALUES ($1,$2,'resource-snapshot',1,'resource_snapshot',$3::jsonb)", [issue.id, runId, JSON.stringify(resource)]);
    const outcome = { identity: { issue_id: issue.id, run_id: runId }, outcome: "failure", body: result, record: { version: 1, completeness: "complete", messages,
      usage: { input_tokens: "9007199254740993", output_tokens: "0", cache_read_tokens: null, cache_creation_tokens: null, total_tokens: null, cost_usd: "0.0010203040" } } };
    await db.query("INSERT INTO collaboration_run_event (issue_id,run_id,producer_event_id,seq,kind,payload) VALUES ($1,$2,'outcome-fixture',2,'outcome',$3::jsonb)", [issue.id, runId, JSON.stringify(outcome)]);
    await db.query("INSERT INTO collaboration_run_event (issue_id,run_id,producer_event_id,seq,kind,payload) VALUES ($1,$2,'legacy-outcome-fixture',1,'outcome',$3::jsonb)", [issue.id, leaderRunId, JSON.stringify({ identity: { issue_id: issue.id, run_id: leaderRunId }, outcome: "success", body: "Legacy leader result; the later group failed." })]);
    const sequence = await db.query<{ event_seq: string }>("UPDATE collaboration_session SET event_seq=event_seq+1 WHERE id=$1 RETURNING event_seq", [sessionId]);
    await db.query("INSERT INTO collaboration_event (issue_id,event_seq,actor_user_id,kind,payload,outbox_state) VALUES ($1,$2,NULL,'ai_failed',$3::jsonb,'delivered')", [issue.id, sequence.rows[0]!.event_seq, JSON.stringify({ group_id: groupId, run_id: runId })]);
    await db.query("INSERT INTO collaboration_notification (recipient_user_id,issue_id,event_seq,kind,source_kind,source_id,run_id) SELECT unnest($1::uuid[]),$2,$3,'ai_failed','issue',NULL,$4", [[ownerId, viewerId], issue.id, sequence.rows[0]!.event_seq, runId]);
    await db.query("COMMIT");
  } catch (error) { await db.query("ROLLBACK"); throw error; }
  finally { await db.end(); }
  return { source, member, viewer, viewerId, workspace, issue, child, runId, leaderRunId, groupId, attachmentId: file.id, input, result };
}

export function recordedAccountPath(fixture: CollaborationRecordFixture, withRun = false) {
  return `/account?view=participating&issue=${fixture.issue.id}${withRun ? `#run-${fixture.runId}` : ""}`;
}

export async function detachRecordedChild(page: Page) {
  await page.getByRole("button", { name: "Issue actions", exact: true }).click();
  await page.getByRole("menuitem", { name: "Relations", exact: true }).click();
  await page.getByRole("menuitem", { name: "Remove parent issue", exact: true }).click();
}

export async function attachRecordedChild(page: Page, fixture: CollaborationRecordFixture) {
  await page.getByRole("button", { name: "Issue actions", exact: true }).click();
  await page.getByRole("menuitem", { name: "Relations", exact: true }).click();
  await page.getByRole("menuitem", { name: "Set parent issue...", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Set parent issue", exact: true });
  await dialog.getByPlaceholder("Search issues...").fill(fixture.issue.title);
  await dialog.getByRole("option", { name: new RegExp(fixture.issue.title) }).click();
  await expect(dialog).toBeHidden();
}

export async function exerciseRecordedPages(page: Page, fixture: CollaborationRecordFixture) {
  const run = page.locator(`[id="run-${fixture.runId}"]`);
  await expect(run.getByRole("region", { name: "Run records", exact: true })).toBeVisible();
  const records = run.getByRole("region", { name: "Run records", exact: true });
  await expect(records.getByText(fixture.input, { exact: true })).toBeVisible();
  await expect(records.getByText("Complete recording", { exact: true })).toBeVisible();
  await expect(records.getByText("9007199254740993", { exact: true })).toBeVisible();
  await expect(records.getByText("USD 0.0010203040", { exact: true })).toBeVisible();
  await expect(records.getByText("Group status", { exact: true }).locator("..")).toHaveText(/Failed/);
  await records.getByText("Recorded resource", { exact: true }).click();
  await expect(records.getByText("Synthetic provider", { exact: true })).toBeVisible();
  await expect(records.getByText("Runtime configuration", { exact: true })).toBeVisible();
  await expect(records.getByRole("button", { name: "Preview recorded-input.txt", exact: true })).toBeVisible();
  await records.getByRole("button", { name: "Preview recorded-input.txt", exact: true }).click();
  const preview = page.getByRole("dialog", { name: "recorded-input.txt", exact: true });
  await expect(preview.getByText("Frozen attachment for the recorded input.", { exact: true })).toBeVisible();
  await preview.getByRole("button", { name: "Close preview", exact: true }).click();
  await expect(records.getByText(fixture.result, { exact: true })).toHaveCount(0);
  await records.getByRole("button", { name: "Load more records", exact: true }).click();
  await expect(records.getByText(fixture.result, { exact: true })).toBeVisible();
  await expect(records.getByText("Recorded tool result. This text is displayed without execution.", { exact: true })).toBeVisible();
  await expect(records.getByRole("button", { name: "Load more records", exact: true })).toHaveCount(0);
  return records;
}

export async function showRecordedUsage(page: Page, runId: string) {
  const usage = page.locator(`[id="run-${runId}"]`).getByRole("region", { name: "Recorded usage", exact: true });
  await usage.evaluate((element) => {
    const root = element.closest<HTMLElement>('[data-tab-scroll-root="account-detail"]');
    if (root) root.scrollTop += element.getBoundingClientRect().top - root.getBoundingClientRect().top - 12;
  });
  await expect(usage).toBeInViewport({ ratio: 1 });
}

export async function removeRecordViewer(fixture: CollaborationRecordFixture) {
  const current = await fixture.source.requestJSON<Snapshot>(`/api/collaboration/issues/${fixture.issue.id}`);
  await fixture.source.requestJSON(`/api/collaboration/issues/${fixture.issue.id}/participants/${fixture.viewerId}/remove`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    expected_collaboration_revision: current.collaboration_revision, expected_lifecycle_epoch: current.lifecycle_epoch,
    expected_participant_epoch: current.write_context.participant_epoch, expected_account_epoch: current.write_context.account_epoch,
  } });
}
