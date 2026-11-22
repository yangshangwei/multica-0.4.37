import { createHash, randomUUID } from "node:crypto";
import { expect, type Browser, type CDPSession, type Page, type Request, type Route, type TestInfo } from "@playwright/test";
import { z } from "zod";
import {
  CollaborationAttachmentSchema, CollaborationAttachmentsSchema, CollaborationCommentInputSchema,
  CollaborationCommentReceiptSchema, CollaborationCommentsSchema, CollaborationCommandHistorySchema,
  CollaborationDraftAttachmentsSchema, CollaborationIssueSchema,
  CollaborationMentionTargetsSchema, CollaborationNotificationReceiptSchema, CollaborationNotificationsSchema, CollaborationParticipantsSchema,
  CollaborationResourceReceiptSchema, CollaborationResourcesSchema, CollaborationTriggerPreviewSchema,
  parseCollaborationAttachmentCapability, readCollaborationDeliveryRecords,
  type CollaborationAttachment, type CollaborationCommentInput, type CollaborationCommentReceipt,
  type CollaborationIssue, type CollaborationNotification, type CollaborationUploadDraft,
} from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";
import { seedCollaborationControlJourney, type CollaborationControlJourney, type ControlTarget } from "./collaboration-control-journey";
import { commentMenuItem } from "./collaboration-comment-menu";
import { revealCollaborationDisclosure, revealCollaborationMentions } from "./collaboration-disclosures";
import {
  HeldRequest, RecoveryEvidence, authenticatedSocket, disconnectObserver,
  recoveryEnvironmentReady, withSessions, type RecoveryActor, type RecoveryPlatform, type RecoverySession, type RecoverySurface,
} from "./collaboration-recovery";
import { openCollaborationNotifications, notificationPath, notificationRow } from "./collaboration-notifications";

const object = z.record(z.string(), z.unknown());
const sha = (bytes: string | Buffer) => createHash("sha256").update(bytes).digest("hex");
const digest = (value: unknown) => sha(JSON.stringify(value));
const now = () => new Date().toISOString();
const uploadPath = "/api/collaboration/attachments/drafts";
const rootPath = (fixture: { issueId: string }) => `/api/collaboration/issues/${fixture.issueId}`;
const composer = (page: Page) => page.getByRole("form", { name: "Comments", exact: true });

export function recoveryExtrasEnvironmentReady() {
  return recoveryEnvironmentReady() && process.env.PAY241_S3_RECOVERY_EXTRAS_A_ACCEPTANCE === "1" && Boolean(process.env.MULTICA_DEPLOYMENT_ID);
}

type ExtraCase = { name: string; status: "NOT_RUN" | "RUNNING" | "PASSED" | "FAILED"; result: unknown };
function caseInventory(evidence: RecoveryEvidence, names: readonly string[]) {
  const inventory: ExtraCase[] = names.map((name) => ({ name, status: "NOT_RUN", result: null }));
  evidence.set({ inventory, planned_subcases: inventory.length, retries: 0, business_case_count: null });
  return async <T>(name: string, exercise: () => Promise<T>): Promise<T> => {
    const entry = inventory.find((item) => item.name === name);
    if (!entry || entry.status !== "NOT_RUN") throw new Error("Each planned recovery subcase must start exactly once");
    entry.status = "RUNNING"; evidence.set({ inventory, active_phase: name });
    try { const result = await exercise(); entry.result = result; entry.status = "PASSED"; return result; }
    catch (error) {
      entry.status = "FAILED";
      evidence.checkpoint("subcase_primary_failure", { subcase: name, error_type: error instanceof Error ? error.name : "unknown_error" });
      throw error;
    } finally {
      evidence.set({ inventory, actual_started_subcases: inventory.filter((item) => item.status !== "NOT_RUN").length });
    }
  };
}

interface ExtrasFixture {
  control: CollaborationControlJourney;
  issueId: string;
  surface: RecoverySurface;
  actors: { creator: RecoveryActor; handler: RecoveryActor; owner: RecoveryActor };
}
const account = z.object({ id: z.uuid(), name: z.string(), email: z.string().optional() });
async function seedExtras(evidence: RecoveryEvidence): Promise<ExtrasFixture> {
  if (!recoveryExtrasEnvironmentReady()) throw new Error("Extras-A requires its separately authorized managed password allocation");
  expect(new URL(process.env.DATABASE_URL!).pathname.slice(1)).toBe(process.env.TASK_DATABASE_NAME);
  const control = await seedCollaborationControlJourney();
  const actor = async (client: TestApiClient, id: string, authenticateWeb: RecoveryActor["authenticateWeb"]): Promise<RecoveryActor> => {
    const current = account.parse(await client.requestJSON("/api/me"));
    expect(current.id).toBe(id); expect(client.getToken()).toBeTruthy();
    return { client, id, name: current.name, authenticateWeb };
  };
  const actors = { creator: await actor(control.creator, control.creatorId, control.authenticateWeb),
    handler: await actor(control.handler, control.handlerId, control.authenticateHandlerWeb),
    owner: await actor(control.owner, control.ownerId, control.authenticateOwnerWeb) };
  const fixture = { control, issueId: control.issueId, actors,
    surface: { issueId: control.issueId, title: control.title, apiUrl: control.apiUrl,
      accountPath: `/account?view=participating&issue=${control.issueId}` } };
  const issue = await readIssue(fixture);
  assertNoExecution(issue);
  evidence.set({ issue_id: fixture.issueId, source_workspace_id: control.sourceWorkspace.id,
    execution_workspace_id: control.executionWorkspace.id, api_url: control.apiUrl, database: process.env.TASK_DATABASE_NAME,
    deployment_id: process.env.MULTICA_DEPLOYMENT_ID, actors: Object.fromEntries(Object.entries(actors).map(([role, actor]) => [role, actor.id])),
    runtime: { id: control.runtimeId, installation_id: control.installationId, binding_id: control.bindingId, state: "offline" },
    targets: control.targets, creation_enabled: true, execution_enabled: false, provider_processes: 0,
    simulated_permits: 0, real_model_execution: false, installed_main_execution: false });
  evidence.checkpoint("public_password_enrollment_offer_adoption_owner_consent", { issue });
  return fixture;
}

async function newPasswordActor(fixture: ExtrasFixture, role: string): Promise<RecoveryActor> {
  const client = new TestApiClient();
  const username = `s3extra${role}${randomUUID().replaceAll("-", "").slice(0, 12)}`;
  const password = `S3-extra-${randomUUID()}!`;
  const registered = account.parse(await client.registerPassword(username, password, `Recovery ${role}`));
  expect(object.parse(await client.requestJSON("/api/me/onboarding/complete", { method: "POST", body: {} })).onboarded_at).not.toBeNull();
  return { client, id: registered.id, name: registered.name,
    async login() { expect((await client.loginPassword(username, password)).id).toBe(registered.id); return client.getToken()!; },
    async authenticateWeb(page) {
      const response = await page.request.post(`${fixture.surface.apiUrl}/auth/login`, { data: { username, password } });
      expect(response.status()).toBe(200); expect(account.parse(object.parse(await response.json()).user).id).toBe(registered.id);
      const cookies = await page.context().cookies(fixture.surface.apiUrl);
      expect(cookies.some((cookie) => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
      expect(cookies.some((cookie) => cookie.name === "multica_csrf" && !cookie.httpOnly)).toBe(true);
    } };
}
async function readIssue(fixture: ExtrasFixture, actor = fixture.actors.creator) {
  return CollaborationIssueSchema.parse(await actor.client.requestJSON(`${rootPath(fixture)}?execution_view_version=6`));
}
function authority(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("This public write requires an actual current collaboration participant");
  return { expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
}
function assertNoExecution(issue: CollaborationIssue) {
  if (!issue.execution) throw new Error("The current execution projection is required to count recovery effects");
  expect(issue.execution.execution_gate.enabled).toBe(false);
  expect(issue.session.queue_state).toBe("paused"); expect(issue.execution.groups).toEqual([]); expect(issue.execution.runs).toEqual([]);
  expect(issue.execution.commands.every((command) => command.group_id === null)).toBe(true);
}
async function invite(fixture: ExtrasFixture, actor: RecoveryActor, evidence: RecoveryEvidence) {
  const requestId = randomUUID();
  const receipt = object.parse(await fixture.actors.creator.client.requestJSON(`${rootPath(fixture)}/participants`, {
    method: "POST", headers: { "Idempotency-Key": requestId }, body: { ...authority(await readIssue(fixture)), target_user_id: actor.id },
  }));
  expect(receipt.code).toBe("participant_added");
  const participants = CollaborationParticipantsSchema.parse(await fixture.actors.creator.client.requestJSON(`${rootPath(fixture)}/participants`));
  expect(participants.participants.some((item) => item.user_id === actor.id)).toBe(true);
  evidence.checkpoint("real_participant_join", { actor_id: actor.id, request_id: requestId, receipt, participants });
  return receipt;
}
async function notificationPages(fixture: ExtrasFixture, actors: RecoveryActor[]) {
  const result: Record<string, CollaborationNotification[]> = {};
  for (const actor of actors) {
    const items: CollaborationNotification[] = [], seen = new Set<string>(); let cursor: string | null = null;
    do {
      const query = new URLSearchParams({ include_archived: "true", limit: "100" }); if (cursor) query.set("cursor", cursor);
      const page = CollaborationNotificationsSchema.parse(await actor.client.requestJSON(`/api/me/collaboration-notifications?${query}`));
      for (const item of page.items) { expect(seen.has(item.id)).toBe(false); seen.add(item.id); items.push(item); }
      if (page.next_cursor) expect(page.next_cursor).not.toBe(cursor); cursor = page.next_cursor;
    } while (cursor);
    result[actor.id] = items.filter((item) => item.issue_id === fixture.issueId);
  }
  return result;
}
async function readComments(fixture: ExtrasFixture, actor = fixture.actors.creator) {
  const rows: z.infer<typeof CollaborationCommentsSchema>["items"] = [], seen = new Set<string>(); let cursor: string | null = null;
  do {
    const query = new URLSearchParams({ limit: "50", max_bytes: "65536" }); if (cursor) query.set("cursor", cursor);
    const page = CollaborationCommentsSchema.parse(await actor.client.requestJSON(`${rootPath(fixture)}/comments?${query}`));
    for (const item of page.items) { expect(seen.has(item.id)).toBe(false); seen.add(item.id); rows.push(item); }
    if (page.next_cursor) expect(page.next_cursor).not.toBe(cursor); cursor = page.next_cursor;
  } while (cursor);
  return rows;
}
async function commands(fixture: ExtrasFixture) {
  const rows: z.infer<typeof CollaborationCommandHistorySchema>["items"] = [], seen = new Set<string>(); let cursor: string | null = null;
  do {
    const query = new URLSearchParams({ limit: "100" }); if (cursor) query.set("cursor", cursor);
    const page = CollaborationCommandHistorySchema.parse(await fixture.actors.creator.client.requestJSON(`${rootPath(fixture)}/commands?${query}`));
    for (const item of page.items) { expect(seen.has(item.id)).toBe(false); seen.add(item.id); rows.push(item); }
    if (page.next_cursor) expect(page.next_cursor).not.toBe(cursor); cursor = page.next_cursor;
  } while (cursor);
  return rows;
}
const commentFacts = (rows: Awaited<ReturnType<typeof readComments>>) => rows.map((row) => ({ id: row.id, content: row.content,
  author_user_id: row.author?.user_id ?? null, revision: row.comment_revision, parent_id: row.parent_id, deleted_at: row.deleted_at, attachment_ids: row.attachment_ids }));
async function state(fixture: ExtrasFixture, actors: RecoveryActor[]) {
  const issue = await readIssue(fixture); assertNoExecution(issue);
  return { issue: { id: issue.id, title: issue.title, handler: issue.handler, creator: issue.creator,
    collaboration_revision: issue.collaboration_revision, lifecycle_epoch: issue.lifecycle_epoch, status: issue.status, session: issue.session },
    comments: commentFacts(await readComments(fixture)), commands: await commands(fixture), notifications: await notificationPages(fixture, actors) };
}
async function clickResponse(session: RecoverySession, path: string, method: string, action: () => Promise<unknown>) {
  const wait = session.page.waitForResponse((response) => new URL(response.url()).pathname === path && response.request().method() === method, { timeout: 15_000 });
  const [received, acted] = await Promise.allSettled([wait, action()]);
  if (acted.status === "rejected") throw acted.reason;
  if (received.status === "rejected") throw received.reason;
  return received.value;
}
async function preview(session: RecoverySession, fixture: ExtrasFixture) {
  const response = await clickResponse(session, `${rootPath(fixture)}/comments/trigger-preview`, "POST",
    () => composer(session.page).getByRole("button", { name: "Preview delivery", exact: true }).click());
  expect(response.status()).toBe(200);
  const value = CollaborationTriggerPreviewSchema.parse(await response.json());
  await expect(composer(session.page).getByRole("button", { name: "Send comment", exact: true })).toBeEnabled();
  return value;
}
interface SentComment { requestId: string; input: CollaborationCommentInput; receipt: CollaborationCommentReceipt }
async function sendComment(session: RecoverySession, fixture: ExtrasFixture, evidence: RecoveryEvidence,
  beforeForward?: () => Promise<void>): Promise<SentComment> {
  const held = new HeldRequest(session, `${rootPath(fixture)}/comments`, "POST"); await held.install();
  const clicked = composer(session.page).getByRole("button", { name: "Send comment", exact: true }).click(); void clicked.catch(() => undefined);
  try {
    const fact = await held.captured(); const input = CollaborationCommentInputSchema.parse(fact.body);
    const requestId = z.uuid().parse(fact.request_id);
    evidence.checkpoint("original_UI_comment_before_forward", { request_id: requestId, input, input_sha256: digest(input) });
    await beforeForward?.();
    const response = await held.forward(); await clicked; expect(response.status).toBe(201);
    const receipt = CollaborationCommentReceiptSchema.parse(response.body);
    expect(receipt).toMatchObject({ issue_id: fixture.issueId, status: 201, code: "comment_created" });
    evidence.checkpoint("actual_UI_comment_committed", { request_id: requestId, input, receipt, browser_status: response.status });
    await expect(composer(session.page).getByText("Comment confirmed.", { exact: true })).toBeVisible();
    await expect(session.page.locator(`[id="comment-${receipt.comment_id}"]`)).toBeVisible();
    return { requestId, input, receipt };
  } finally { await held.close(); await Promise.allSettled([clicked]); }
}
async function replayComment(fixture: ExtrasFixture, original: SentComment, evidence: RecoveryEvidence) {
  const receipt = CollaborationCommentReceiptSchema.parse(await fixture.actors.creator.client.requestJSON(`${rootPath(fixture)}/comments`, {
    method: "POST", headers: { "Idempotency-Key": original.requestId }, body: original.input,
  }));
  expect(receipt).toEqual(original.receipt);
  const recovered = CollaborationCommentReceiptSchema.parse(await fixture.actors.creator.client.requestJSON(`${rootPath(fixture)}/comments/operations/${original.requestId}`));
  expect(recovered).toEqual(original.receipt);
  evidence.checkpoint("same_original_request_replay_and_GET", { request_id: original.requestId, replay: receipt, GET: recovered });
}
async function selectMixed(session: RecoverySession, fixture: ExtrasFixture, targets: ControlTarget[], repeat = false) {
  const catalogue = CollaborationMentionTargetsSchema.parse(await fixture.actors.creator.client.requestJSON(`${rootPath(fixture)}/mention-targets?execution_view_version=6`));
  await composer(session.page).getByLabel("Add comment", { exact: true }).fill(`Mixed recovery input ${randomUUID()}`);
  await revealCollaborationMentions(composer(session.page));
  const human = catalogue.participants.find((person) => person.user_id === fixture.actors.handler.id);
  if (!human) throw new Error("The actual mention catalogue lacks the current human handler");
  for (let n = 0; n < (repeat ? 2 : 1); n++) await composer(session.page).getByRole("group", { name: "People", exact: true })
    .getByRole("button", { name: human.name, exact: true }).click();
  for (const target of targets) {
    const item = catalogue.targets.find((candidate) => candidate.target_id === target.targetId);
    if (!item) throw new Error("The actually consented target is absent from the current typed mention catalogue");
    for (let n = 0; n < (repeat ? 2 : 1); n++) await composer(session.page).getByRole("group", { name: "AI targets", exact: true })
      .getByRole("button", { name: item.name, exact: true }).click();
  }
  const content = await composer(session.page).getByLabel("Add comment", { exact: true }).inputValue();
  expect(content.split(`mention://member/${human.user_id}`).length - 1).toBe(repeat ? 2 : 1);
  for (const target of targets) expect(content.split(`mention://${target.kind}/${target.resourceId}`).length - 1).toBe(repeat ? 2 : 1);
  return { content, catalogue };
}
function eventRecipients(notifications: Record<string, CollaborationNotification[]>, receipt: CollaborationCommentReceipt) {
  return Object.entries(notifications).flatMap(([actorId, items]) => items.filter((item) => item.event_seq === receipt.event_seq)
    .map((item) => ({ actor_id: actorId, id: item.id, is_mention: item.is_mention, source_id: item.source_id, source_kind: item.source_kind, kind: item.kind })));
}
async function editComment(session: RecoverySession, fixture: ExtrasFixture, commentId: string, content: string) {
  const row = session.page.locator(`[id="comment-${commentId}"]`); await (await commentMenuItem(row, "Edit comment")).click();
  await row.getByLabel("Edit comment text", { exact: true }).fill(content);
  const response = await clickResponse(session, `${rootPath(fixture)}/comments/${commentId}`, "PATCH",
    () => row.getByRole("button", { name: "Save comment", exact: true }).click());
  expect(response.status()).toBe(200); const receipt = CollaborationCommentReceiptSchema.parse(await response.json());
  expect(receipt.targets).toEqual([]); await expect(row.getByRole("form", { name: "Edit comment", exact: true })).toHaveCount(0);
  return { request_id: z.uuid().parse((await response.request().allHeaders())["idempotency-key"]), receipt,
    input: object.parse(response.request().postDataJSON()) };
}

export async function exerciseRecoveryNotifications(browser: Browser, platform: RecoveryPlatform, info: TestInfo, evidence: RecoveryEvidence) {
  const run = caseInventory(evidence, ["mixed_repeat_replay_reconnect", "late_join_and_edit_delta", "target_withdrawal_after_preview"]);
  const fixture = await seedExtras(evidence), outsider = await newPasswordActor(fixture, "outsider");
  const actors = [...Object.values(fixture.actors), outsider]; let original: SentComment;
  return withSessions(browser, platform, fixture.surface, info, evidence, async (open) => {
    const author = await open(fixture.actors.creator), human = await open(fixture.actors.handler);
    await run("mixed_repeat_replay_reconnect", async () => {
      const before = await state(fixture, actors);
      const selected = await selectMixed(author, fixture, [fixture.control.agentA], true), viewed = await preview(author, fixture);
      expect(viewed.targets).toHaveLength(1); expect(viewed.targets[0]).toMatchObject({ target_id: fixture.control.agentA.targetId, state: "waiting", command_id: null });
      expect(viewed.recipients.map((item) => [item.recipient.user_id, item.is_mention]).sort()).toEqual(
        [[fixture.actors.handler.id, true], [fixture.actors.owner.id, false]].sort());
      original = await sendComment(author, fixture, evidence); expect(original.input.content).toBe(selected.content);
      expect(original.receipt.targets).toHaveLength(1);
      const queued = original.receipt.targets[0]!; expect(queued).toMatchObject({ target_id: fixture.control.agentA.targetId, state: "waiting" });
      const after = await state(fixture, actors);
      expect(after.comments).toHaveLength(before.comments.length + 1); expect(after.commands).toHaveLength(before.commands.length + 1);
      expect(after.commands.find((item) => item.id === queued.command_id)).toMatchObject({ input_event_id: original.receipt.event_id,
        comment_id: original.receipt.comment_id, content: original.input.content, state: "waiting", group_id: null, author_user_id: fixture.actors.creator.id });
      const recipients = eventRecipients(after.notifications, original.receipt);
      expect(recipients.map((item) => [item.actor_id, item.is_mention]).sort()).toEqual([[fixture.actors.handler.id, true], [fixture.actors.owner.id, false]].sort());
      for (const item of recipients) expect(item).toMatchObject({ kind: "comment_created", source_kind: "comment", source_id: original.receipt.comment_id });
      const notification = after.notifications[fixture.actors.handler.id]!.find((item) => item.event_seq === original.receipt.event_seq);
      if (!notification) throw new Error("The original human mention has no exact current notification identity");
      await openCollaborationNotifications(human.page, "all");
      const row = notificationRow(human.page, notification.id);
      await expect(row).toHaveCount(1); await expect(row.getByText("Mentioned you", { exact: true })).toBeVisible();
      const link = row.getByRole("link", { name: fixture.surface.title, exact: true });
      const href = await link.getAttribute("href");
      if (!href) throw new Error("The original human mention has no notification page link");
      const destination = new URL(href, fixture.surface.apiUrl);
      expect(destination.pathname + destination.search + destination.hash).toBe(notificationPath("all", notification.id, fixture.issueId, `#comment-${original.receipt.comment_id}`));
      expect(notification.read_at).toBeNull();
      const readResponse = await clickResponse(human, `/api/me/collaboration-notifications/${notification.id}/read`, "POST",
        () => row.getByRole("button", { name: "Mark read", exact: true }).click());
      expect(readResponse.status()).toBe(200);
      const readReceipt = CollaborationNotificationReceiptSchema.parse(await readResponse.json());
      expect(readReceipt).toMatchObject({ id: notification.id, archived_at: notification.archived_at });
      expect(readReceipt.read_at).not.toBeNull();
      evidence.checkpoint("original_notification_explicit_UI_read_acknowledged", { notification_id: notification.id,
        original_event_id: original.receipt.event_id, original_comment_id: original.receipt.comment_id,
        method: "POST", status: readResponse.status(), receipt: readReceipt, automatic_read_on_navigation_claim: false });
      await expect.poll(async () => (await notificationPages(fixture, actors))[fixture.actors.handler.id]!
        .find((item) => item.id === notification.id)?.read_at).toBe(readReceipt.read_at);
      await expect(row.getByRole("button", { name: "Mark read", exact: true })).toHaveCount(0);
      await expect(row).toHaveCount(1); await link.click();
      await expect(row).toHaveAttribute("data-selected", "true");
      await expect(human.page.getByRole("heading", { name: "Collaboration notifications", exact: true })).toBeVisible();
      await expect(human.page.locator('[data-slot="popover-content"]').filter({ has: human.page.getByText("Notifications", { exact: true }) })).toHaveCount(0);
      const comment = human.page.locator(`[id="comment-${original.receipt.comment_id}"]`);
      await expect(comment).toBeVisible(); await expect(comment).toBeFocused();
      const frozen = await state(fixture, actors); await replayComment(fixture, original, evidence); expect(await state(fixture, actors)).toEqual(frozen);
      await Promise.allSettled([...author.pendingReads]); const beforeUIWrites = author.writes.length;
      const socket = await authenticatedSocket(author, String(evidence.value.started_at));
      await disconnectObserver(author, socket, evidence, "notifications_original_socket_offline");
      const reconnectAt = now(); await author.context.setOffline(false);
      const replacement = await authenticatedSocket(author, reconnectAt, socket.id);
      await replayComment(fixture, original, evidence); expect(await state(fixture, actors)).toEqual(frozen);
      await Promise.allSettled([...author.pendingReads]); expect(author.writes).toHaveLength(beforeUIWrites);
      expect(await fixture.control.owner.countIssueDispatches(fixture.issueId)).toBe(0);
      return { original, recipients, notification_denominator: 2, command_denominator: 1, explicit_notification_read: readReceipt,
        distinct_reauthenticated_socket: replacement.id,
        old_socket_id: socket.id, recovery_comment_input_notification_increments: 0, recovery_UI_POSTs: 0, ordinary_dispatches: 0 };
    });
    await run("late_join_and_edit_delta", async () => {
      const late = await newPasswordActor(fixture, "latejoin"); actors.push(late);
      const frozenRecipients = eventRecipients(await notificationPages(fixture, actors), original.receipt);
      const joined = await invite(fixture, late, evidence); const before = await state(fixture, actors);
      await replayComment(fixture, original, evidence); expect(await state(fixture, actors)).toEqual(before);
      expect(eventRecipients(await notificationPages(fixture, actors), original.receipt)).toEqual(frozenRecipients);
      const edited = await editComment(author, fixture, original.receipt.comment_id,
        `${original.input.content}\nA new explicit human mention [@${late.name}](mention://member/${late.id})`);
      const delta = eventRecipients(await notificationPages(fixture, actors), edited.receipt);
      expect(delta.map((item) => [item.actor_id, item.is_mention])).toEqual([[late.id, true]]);
      const prose = await editComment(author, fixture, original.receipt.comment_id, `${String(edited.input.content)}\nSurrounding prose changed; original AI input stays frozen.`);
      expect(eventRecipients(await notificationPages(fixture, actors), prose.receipt)).toEqual([]);
      expect(await commands(fixture)).toEqual(before.commands);
      expect(eventRecipients(await notificationPages(fixture, actors), original.receipt)).toEqual(frozenRecipients);
      assertNoExecution(await readIssue(fixture));
      return { late_actor_id: late.id, joined, original_request_id: original.requestId, frozenRecipients, edited, prose,
        edit_mention_notification_denominator: 1, prose_notification_denominator: 0, new_AI_inputs_runs_launches: 0 };
    });
    await run("target_withdrawal_after_preview", async () => {
      const beforeCommands = await commands(fixture);
      await selectMixed(author, fixture, [fixture.control.agentA, fixture.control.agentB]); const viewed = await preview(author, fixture);
      expect(viewed.targets.map((item) => item.target_id).sort()).toEqual([fixture.control.agentA.targetId, fixture.control.agentB.targetId].sort());
      let revoked: unknown;
      const sent = await sendComment(author, fixture, evidence, async () => {
        const resources = CollaborationResourcesSchema.parse(await fixture.actors.owner.client.requestJSON(`${rootPath(fixture)}/resources`));
        const resource = resources.targets.find((item) => item.target_id === fixture.control.agentA.targetId)?.resources[0];
        if (!resource?.grant_id || !resource.grant_epoch) throw new Error("The real previewed target has no current owner grant");
        const requestId = randomUUID();
        const observed = authority(await readIssue(fixture, fixture.actors.owner));
        const body = { expected_lifecycle_epoch: observed.expected_lifecycle_epoch,
          expected_participant_epoch: observed.expected_participant_epoch, expected_account_epoch: observed.expected_account_epoch,
          expected_grant_epoch: resource.grant_epoch };
        const path = `${rootPath(fixture)}/grants/${resource.grant_id}/revoke`;
        evidence.checkpoint("original_owner_grant_request_before_dispatch", { request_id: requestId, path, method: "POST", body,
          actor_id: fixture.actors.owner.id, target_id: fixture.control.agentA.targetId, grant_id: resource.grant_id, grant_epoch: resource.grant_epoch });
        revoked = CollaborationResourceReceiptSchema.parse(await fixture.actors.owner.client.requestJSON(path, {
          method: "POST", headers: { "Idempotency-Key": requestId }, body,
        }));
        expect(revoked).toMatchObject({ status: 200, code: "grant_revoked" });
        evidence.checkpoint("actual_owner_grant_withdrawal_after_original_UI_preview", { request_id: requestId, grant_id: resource.grant_id, revoked });
      });
      expect(sent.receipt.targets).toHaveLength(2);
      expect(sent.receipt.targets.find((item) => item.resource_id === fixture.control.agentA.resourceId)).toMatchObject({ state: "blocked", code: "target_unavailable", command_id: null });
      const accepted = sent.receipt.targets.find((item) => item.resource_id === fixture.control.agentB.resourceId)!;
      expect(accepted).toMatchObject({ target_id: fixture.control.agentB.targetId, state: "waiting" });
      const after = await state(fixture, actors);
      expect(after.commands).toHaveLength(beforeCommands.length + 1);
      expect(after.commands.find((item) => item.id === accepted.command_id)).toMatchObject({ input_event_id: sent.receipt.event_id, content: sent.input.content, state: "waiting", group_id: null });
      for (const old of beforeCommands) {
        const retained = after.commands.find((item) => item.id === old.id);
        if (!retained) throw new Error("Grant withdrawal removed an original immutable input identity");
        const identity = (item: typeof old) => ({ id: item.id, channel_id: item.channel_id, target_id: item.target_id, batch_id: item.batch_id,
          input_event_id: item.input_event_id, input_event_seq: item.input_event_seq, author_user_id: item.author_user_id, content: item.content, attachment_ids: item.attachment_ids });
        expect(identity(retained)).toEqual(identity(old));
      }
      const recipients = eventRecipients(after.notifications, sent.receipt);
      expect(recipients).toHaveLength(actors.length - 2); expect(recipients.some((item) => item.actor_id === outsider.id)).toBe(false);
      await replayComment(fixture, sent, evidence); expect(await state(fixture, actors)).toEqual(after);
      await expect(composer(author.page).getByText(/Target is unavailable or its configuration changed/)).toBeVisible();
      expect(await fixture.control.owner.countIssueDispatches(fixture.issueId)).toBe(0);
      return { preview: viewed, original_request_id: sent.requestId, sent, revoked, recipients, old_commands_before: beforeCommands,
        commands_after: after.commands, accepted_new_command_count: 1, refused_new_command_count: 0, recovery_effect_increments: 0, physical_launches: 0 };
    });
    evidence.checkpoint("complete_notification_inventory", { final: await state(fixture, actors) });
    return { issue_id: fixture.issueId, platform, cases: 3, runtime_processes: 0 };
  });
}

interface UploadFact { at: string; request_id: string; draft_id: string; issue_id: string; name: string; content_type: string;
  size: number; sha256: string; multipart_sha256: string; authentication: { cookie: boolean; bearer: boolean; csrf: boolean } }
async function uploadFact(request: Request, evidence: RecoveryEvidence, stage: string): Promise<UploadFact> {
  const raw = request.postDataBuffer(), headers = await request.allHeaders();
  evidence.checkpoint("multipart_observation_before_forward", { upload_stage: stage, method: request.method(), path: new URL(request.url()).pathname,
    request_id: headers["idempotency-key"] ?? null, content_type: headers["content-type"] ?? null,
    raw_observed: raw !== null, raw_bytes: raw?.length ?? null, raw_sha256: raw === null ? null : sha(raw) });
  if (raw === null) throw new Error("The actual intercepted upload multipart body is not observable");
  if (!headers["content-type"]?.includes("multipart/form-data")) throw new Error("The actual intercepted upload has no multipart Content-Type");
  const form = await new Response(new Uint8Array(raw), { headers: { "Content-Type": headers["content-type"] } }).formData();
  expect([...form.keys()].sort()).toEqual(["draft_id", "file", "issue_id"]);
  const file = form.get("file"); if (file === null || typeof file === "string") throw new Error("The original multipart file is absent");
  return { at: now(), request_id: z.uuid().parse(headers["idempotency-key"]), draft_id: z.uuid().parse(form.get("draft_id")),
    issue_id: z.uuid().parse(form.get("issue_id")), name: file.name, content_type: file.type, size: file.size,
    sha256: sha(Buffer.from(await file.arrayBuffer())), multipart_sha256: sha(raw),
    authentication: { cookie: Boolean(headers.cookie), bearer: Boolean(headers.authorization), csrf: Boolean(headers["x-csrf-token"]) } };
}

/** Capture multipart bytes before forwarding. Only deliberate committed ACK
 * loss fetches upstream; ordinary replies continue the unchanged UI request. */
class HeldUpload {
  fact: UploadFact | null = null;
  response: { status: number; attachment: CollaborationAttachment } | null = null;
  private capturedRoute = false;
  private decision: "commit_then_lose" | "continue" | "abort" | null = null;
  private failure: unknown;
  private installed = false;
  private release!: () => void;
  private end!: () => void;
  private gate = new Promise<void>((resolve) => { this.release = resolve; });
  private terminal = new Promise<void>((resolve) => { this.end = resolve; });
  private matches = (url: URL) => url.pathname === uploadPath;
  private handler = async (route: Route) => {
    if (route.request().method() !== "POST") { await route.continue(); return; }
    if (this.capturedRoute) { this.failure = new Error("One upload intent sent an unexpected extra POST before recovery"); await route.abort("failed"); return; }
    this.capturedRoute = true;
    try {
      this.fact = await uploadFact(route.request(), this.evidence, this.captureStage);
      this.evidence.checkpoint(this.captureStage, { upload: this.fact });
      await this.gate;
      if (this.decision === "abort") { await route.abort("failed"); return; }
      if (this.decision === "continue") {
        const responseWait = this.session.page.waitForResponse((response) => response.request() === route.request(), { timeout: 15_000 });
        const [continued, received] = await Promise.allSettled([route.continue(), responseWait]);
        if (continued.status === "rejected") throw continued.reason;
        if (received.status === "rejected") throw received.reason;
        const response = received.value, body: unknown = await response.json();
        this.evidence.checkpoint("actual_upload_result_after_unchanged_continue", { request_id: this.fact.request_id, status: response.status(), body,
          same_request_identity: true, continued_POSTs: 1 });
        this.response = { status: response.status(), attachment: CollaborationAttachmentSchema.parse(body) };
        return;
      }
      const upstream = await route.fetch({ maxRetries: 0, timeout: 15_000 });
      try {
        const body: unknown = await upstream.json();
        this.evidence.checkpoint("actual_upload_result_before_losing_ACK", { status: upstream.status(), body });
        this.response = { status: upstream.status(), attachment: CollaborationAttachmentSchema.parse(body) };
        await route.abort("failed");
      } finally { await upstream.dispose(); }
    } catch (error) { this.failure = error; await route.abort("failed").catch(() => undefined); }
    finally { this.end(); }
  };
  constructor(private session: RecoverySession, private evidence: RecoveryEvidence, private captureStage = "original_actual_multipart_upload") {}
  async install() { await this.session.page.route(this.matches, this.handler); this.installed = true; }
  async captured() {
    await expect.poll(() => { if (this.failure) throw this.failure; return this.fact !== null; },
      { message: "Capture the original UI multipart key, draft and file fingerprint" }).toBe(true);
    return this.fact!;
  }
  async forward(loseResponse = true) {
    await this.captured(); if (this.decision) throw new Error("The original upload has already reached a terminal decision");
    this.decision = loseResponse ? "commit_then_lose" : "continue"; this.release(); await this.terminal;
    if (this.failure) throw this.failure; if (!this.response) throw new Error("The actual upload API result was not retained"); return this.response;
  }
  async close() {
    if (!this.decision) { this.decision = "abort"; this.release(); }
    if (this.capturedRoute) await this.terminal;
    if (!this.installed) return; this.installed = false;
    if (this.session.page.isClosed()) return;
    try { await this.session.page.unroute(this.matches, this.handler); }
    catch (error) { this.session.cleanup_errors.push({ action: "held_upload_unroute", at: now(), error_type: error instanceof Error ? error.name : "unknown_error" }); }
  }
}

async function continueUpload(session: RecoverySession, evidence: RecoveryEvidence, stage: string, action: () => Promise<unknown>, validate: (fact: UploadFact) => void) {
  const held = new HeldUpload(session, evidence, stage); await held.install();
  const chosen = Promise.resolve().then(action); void chosen.catch(() => undefined);
  try {
    const fact = await held.captured(); validate(fact);
    const response = await held.forward(false); await chosen;
    return { fact, response };
  } finally { try { await held.close(); } finally { await Promise.allSettled([chosen]); } }
}

async function deliveryRecords(session: RecoverySession) {
  const entries = await session.page.evaluate(() => Object.keys(localStorage).filter((key) => key.startsWith("multica_collaboration_delivery:"))
    .map((key) => [key, localStorage.getItem(key)!] as const));
  const own = entries.find(([, raw]) => {
    const scope = object.parse(object.parse(JSON.parse(raw)).scope), context = object.parse(scope.context);
    return scope.userId === session.actor.id && context.kind === "issue" && context.issueId === session.fixture.issueId;
  });
  if (!own) return [];
  const scope = object.parse(object.parse(JSON.parse(own[1])).scope), context = object.parse(scope.context), values = new Map(entries);
  const storage = { getItem: (key: string) => values.get(key) ?? null, keys: () => [...values.keys()],
    setItem() { throw new Error("Acceptance observation must not modify a saved original intent"); },
    removeItem() { throw new Error("Acceptance observation must not remove a saved original intent"); } };
  return readCollaborationDeliveryRecords({ server: z.string().parse(scope.server), userId: session.actor.id,
    context: { kind: "issue", issueId: session.fixture.issueId, sourceWorkspaceId: z.uuid().parse(context.sourceWorkspaceId) } }, storage);
}
async function uploadRecords(session: RecoverySession): Promise<CollaborationUploadDraft[]> {
  return (await deliveryRecords(session)).filter((value): value is CollaborationUploadDraft => value.kind === "upload");
}
async function uploadPOSTCount(session: RecoverySession) {
  await Promise.allSettled([...session.pendingReads]); return session.writes.filter((write) => write.path === uploadPath && write.method === "POST").length;
}
function uploadIdentity(fact: UploadFact) {
  return { request_id: fact.request_id, draft_id: fact.draft_id, issue_id: fact.issue_id, name: fact.name,
    content_type: fact.content_type, size: fact.size, sha256: fact.sha256 };
}
async function lostUpload(session: RecoverySession, fixture: ExtrasFixture, evidence: RecoveryEvidence, name: string, bytes: Buffer) {
  const held = new HeldUpload(session, evidence); await held.install();
  const chosen = composer(session.page).getByLabel("Attach files", { exact: true }).setInputFiles({ name, mimeType: "text/plain", buffer: bytes });
  void chosen.catch(() => undefined);
  try {
    const fact = await held.captured(); expect(fact).toMatchObject({ issue_id: fixture.issueId, name, size: bytes.length, sha256: sha(bytes) });
    const saved = await uploadRecords(session); expect(saved).toHaveLength(1);
    expect(saved[0]).toMatchObject({ requestId: fact.request_id, draftId: fact.draft_id,
      file: { name, type: "text/plain", size: bytes.length, sha256: sha(bytes) } });
    evidence.checkpoint("original_upload_fingerprint_is_persisted_before_API_commit", { fact, saved });
    const response = await held.forward(); await chosen; expect(response.status).toBe(201);
    expect(response.attachment).toMatchObject({ draft_id: fact.draft_id, issue_id: null, intended_issue_id: fixture.issueId, uploader_user_id: session.actor.id });
    await expect(composer(session.page).getByText(`Upload of ${name} is not confirmed. Check its result or retry with the same file.`, { exact: true })).toBeVisible();
    expect(await uploadRecords(session)).toEqual(saved);
    return { fact, saved: saved[0]!, attachment: response.attachment };
  } finally { await held.close(); await Promise.allSettled([chosen]); }
}
async function showConfirmedSelection(session: RecoverySession, original: Awaited<ReturnType<typeof lostUpload>>) {
  await expect(composer(session.page).getByRole("checkbox", { name: `Include ${original.attachment.name}`, exact: true })).toBeChecked();
  await expect(composer(session.page).getByRole("button", { name: "Check upload", exact: true })).toHaveCount(0);
  expect(await uploadRecords(session)).toEqual([]);
  const form = (await deliveryRecords(session)).find((value) => value.kind === "form" && value.draftId === original.fact.draft_id);
  if (!form || form.kind !== "form") throw new Error("The confirmed upload has no original current composer form");
  expect(form.purpose).toBe("comment"); expect(form.selectedAttachmentIds).toEqual([original.attachment.id]);
}
async function deliverUploadedComment(session: RecoverySession, fixture: ExtrasFixture, evidence: RecoveryEvidence,
  attachment: CollaborationAttachment, bytes: Buffer, content: string) {
  await composer(session.page).getByLabel("Add comment", { exact: true }).fill(content); const viewed = await preview(session, fixture);
  expect(viewed.targets).toEqual([]); const sent = await sendComment(session, fixture, evidence);
  expect(sent.receipt.attachment_ids).toEqual([attachment.id]); expect(sent.input.draft_id).toBe(attachment.draft_id);
  const listed = CollaborationAttachmentsSchema.parse(await session.actor.client.requestJSON(`${rootPath(fixture)}/attachments`));
  const file = listed.attachments.find((item) => item.id === attachment.id);
  if (!file) throw new Error("The exact confirmed original upload was not bound to the explicit comment");
  expect(file).toMatchObject({ id: attachment.id, version: attachment.version, issue_id: fixture.issueId, draft_id: attachment.draft_id });
  const read = await session.page.request.get(`${fixture.surface.apiUrl}${rootPath(fixture)}/attachments/${file.id}/content`, {
    headers: session.platform === "Desktop" ? { Authorization: `Bearer ${session.actor.client.getToken()}` } : {},
  });
  expect(read.status()).toBe(200); expect(await read.body()).toEqual(bytes);
  const before = await state(fixture, Object.values(fixture.actors)); await replayComment(fixture, sent, evidence);
  expect(await state(fixture, Object.values(fixture.actors))).toEqual(before);
  expect((await deliveryRecords(session)).some((record) => record.draftId === attachment.draft_id)).toBe(false);
  return { sent, file, plaintext_sha256: sha(bytes), explicit_comment_replay_effects: 0 };
}

export async function exerciseRecoveryUploads(browser: Browser, platform: RecoveryPlatform, info: TestInfo, evidence: RecoveryEvidence) {
  const run = caseInventory(evidence, ["committed_upload_GET_reload", "same_bytes_unknown_retry", "changed_reselection_matching_retry"]);
  const fixture = await seedExtras(evidence), tuples: unknown[] = [], attachmentIds: string[] = [], draftIds: string[] = [];
  return withSessions(browser, platform, fixture.surface, info, evidence, async (open) => {
    const author = await open(fixture.actors.creator);
    for (const name of ["committed_upload_GET_reload", "same_bytes_unknown_retry", "changed_reselection_matching_retry"] as const) {
      await run(name, async () => {
        const filename = `${name}.txt`, bytes = Buffer.from(`Original upload ${name} ${randomUUID()}`);
        const original = await lostUpload(author, fixture, evidence, filename, bytes), beforePOSTs = await uploadPOSTCount(author);
        if (name !== "same_bytes_unknown_retry") {
          await author.page.reload(); await expect(author.page.getByRole("heading", { name: fixture.surface.title, exact: true })).toBeVisible();
          expect(await uploadRecords(author)).toEqual([original.saved]);
          await expect(composer(author.page).getByRole("button", { name: "Retry upload", exact: true })).toHaveCount(0);
        }
        let recovered: unknown, changedFingerprint: string | null = null;
        if (name === "committed_upload_GET_reload") {
          const path = `${uploadPath}/${original.fact.draft_id}/operations/${original.fact.request_id}`;
          const response = await clickResponse(author, path, "GET", () => composer(author.page).getByRole("button", { name: "Check upload", exact: true }).click());
          expect(response.status()).toBe(200); recovered = CollaborationAttachmentSchema.parse(await response.json());
          expect(recovered).toEqual(original.attachment); expect(await uploadPOSTCount(author)).toBe(beforePOSTs);
        } else {
          if (name === "changed_reselection_matching_retry") {
            const changed = Buffer.from(bytes); changed[0] = changed[0] === 65 ? 66 : 65; changedFingerprint = sha(changed);
            expect(changedFingerprint).not.toBe(sha(bytes)); expect(changed.length).toBe(bytes.length);
            await composer(author.page).getByLabel("Select the same file again", { exact: true }).setInputFiles({ name: filename, mimeType: "text/plain", buffer: changed });
            await expect(composer(author.page).getByText("Choose the original file with the same name and contents.", { exact: true })).toBeVisible();
            expect(await uploadPOSTCount(author)).toBe(beforePOSTs); expect(await uploadRecords(author)).toEqual([original.saved]);
            evidence.checkpoint("same_name_size_changed_bytes_reselection_refused_without_POST_or_key_replacement", { original: original.fact, changed_sha256: changedFingerprint, POSTs: beforePOSTs });
            // The picker stores the chosen File before validating it. A real
            // reload preserves the original intent and exposes reselect again.
            await author.page.reload(); await expect(author.page.getByRole("heading", { name: fixture.surface.title, exact: true })).toBeVisible();
            expect(await uploadRecords(author)).toEqual([original.saved]);
          }
          const { response, fact: retry } = await continueUpload(author, evidence, "actual_retry_multipart_before_forward", () => name === "same_bytes_unknown_retry"
            ? composer(author.page).getByRole("button", { name: "Retry upload", exact: true }).click()
            : composer(author.page).getByLabel("Select the same file again", { exact: true }).setInputFiles({ name: filename, mimeType: "text/plain", buffer: bytes }),
          (fact) => { expect(uploadIdentity(fact)).toEqual(uploadIdentity(original.fact)); });
          expect(response.status).toBe(201); recovered = response.attachment;
          expect(recovered).toEqual(original.attachment);
          expect(uploadIdentity(retry)).toEqual(uploadIdentity(original.fact));
          expect(await uploadPOSTCount(author)).toBe(beforePOSTs + 1);
          evidence.checkpoint("original_key_and_matching_file_bytes_converge_on_first_attachment", { original: original.fact, retry, actual_status: response.status, recovered });
        }
        await showConfirmedSelection(author, original);
        const draft = CollaborationDraftAttachmentsSchema.parse(await fixture.actors.creator.client.requestJSON(`${uploadPath}/${original.fact.draft_id}`));
        expect(draft.attachments.map((file) => file.id)).toEqual([original.attachment.id]);
        const delivered = await deliverUploadedComment(author, fixture, evidence, original.attachment, bytes, `Explicit delivery ${name}`);
        attachmentIds.push(original.attachment.id); draftIds.push(original.fact.draft_id);
        const result = { case: name, original_upload: original.fact, saved_original: original.saved, winning_attachment: original.attachment,
          recovered, changed_sha256: changedFingerprint, delivered, one_attachment_per_original_upload_tuple: true,
          required_main_ciphertext_row_object_reference_audit: { uploader: author.actor.id, draft: original.fact.draft_id,
            request: original.fact.request_id, attachment: original.attachment.id, plaintext_sha256: sha(bytes) } };
        tuples.push(result); evidence.set({ upload_tuples: tuples }); return result;
      });
    }
    expect(new Set(attachmentIds).size).toBe(3); expect(new Set(draftIds).size).toBe(3);
    const files = CollaborationAttachmentsSchema.parse(await fixture.actors.creator.client.requestJSON(`${rootPath(fixture)}/attachments`));
    expect(files.attachments.map((file) => file.id).sort()).toEqual([...attachmentIds].sort());
    expect(await readComments(fixture)).toHaveLength(3); expect(await commands(fixture)).toEqual([]); assertNoExecution(await readIssue(fixture));
    expect(await fixture.control.owner.countIssueDispatches(fixture.issueId)).toBe(0);
    evidence.checkpoint("complete_three_distinct_original_upload_tuples", { attachment_ids: attachmentIds, draft_ids: draftIds, files,
      upload_tuples: tuples, count: 3, ordinary_dispatches: 0, physical_launches: 0 });
    return { issue_id: fixture.issueId, platform, upload_tuples: tuples, cases: 3 };
  });
}

async function normalUpload(session: RecoverySession, fixture: ExtrasFixture, evidence: RecoveryEvidence, name: string, bytes: Buffer) {
  const { fact, response } = await continueUpload(session, evidence, "authorized_multipart_before_revocation", () => composer(session.page).getByLabel("Attach files", { exact: true })
    .setInputFiles({ name, mimeType: "text/plain", buffer: bytes }),
  (fact) => { expect(fact).toMatchObject({ issue_id: fixture.issueId, name, sha256: sha(bytes), size: bytes.length }); });
  expect(response.status).toBe(201); const attachment = response.attachment;
  expect(attachment).toMatchObject({ draft_id: fact.draft_id, intended_issue_id: fixture.issueId, uploader_user_id: session.actor.id });
  await expect(composer(session.page).getByRole("checkbox", { name: `Include ${name}`, exact: true })).toBeChecked();
  evidence.checkpoint("authorized_before_revocation_actual_upload", { fact, attachment, browser_status: response.status });
  return { fact, attachment };
}
async function attachmentPreview(session: RecoverySession, file: CollaborationAttachment, bytes: Buffer) {
  await revealCollaborationDisclosure(session.page, "Attachments");
  await session.page.getByRole("region", { name: "Attachments", exact: true }).getByRole("button", { name: `Preview ${file.name}`, exact: true }).click();
  await expect(session.page.getByRole("dialog", { name: file.name, exact: true }).locator("pre")).toHaveText(bytes.toString());
}
async function rawActor(fixture: ExtrasFixture, actor: RecoveryActor, path: string, method = "GET") {
  const response = await fetch(`${fixture.surface.apiUrl}${path}`, { method, headers: { Authorization: `Bearer ${actor.client.getToken()!}` }, redirect: "error" });
  const bytes = Buffer.from(await response.arrayBuffer()); let body: unknown = null;
  if (response.headers.get("content-type")?.includes("application/json") && bytes.length) body = JSON.parse(bytes.toString());
  return { at: now(), path, method, status: response.status, redirected: response.redirected,
    body_bytes: bytes.length, body_sha256: sha(bytes), body,
    private_headers: { cache_control: response.headers.get("cache-control"), referrer_policy: response.headers.get("referrer-policy") } };
}
async function capabilities(fixture: ExtrasFixture, actor: RecoveryActor, file: CollaborationAttachment, bytes: Buffer, evidence: RecoveryEvidence) {
  const result = [];
  for (const intent of ["content", "download"] as const) {
    const cap = parseCollaborationAttachmentCapability(await actor.client.requestJSON(`${rootPath(fixture)}/attachments/${file.id}/capabilities`, {
      method: "POST", body: { intent },
    }), fixture.surface.apiUrl, fixture.issueId, file.id, file.version);
    if (!cap) throw new Error("The real authorized native capability does not match its current file and expiry");
    const response = await fetch(cap.url, { redirect: "error" }); expect(response.status).toBe(200);
    expect(Buffer.from(await response.arrayBuffer())).toEqual(bytes); result.push({ intent, capability: cap });
  }
  evidence.checkpoint("real_native_capabilities_authorized_before_revoke", { actor_id: actor.id,
    capabilities: result.map(({ intent, capability }) => ({ intent, issue_id: capability.issue_id, attachment_id: capability.attachment_id,
      version: capability.version, expires_at: capability.expires_at, URL_sha256: sha(capability.url), status_before: 200 })) });
  return result;
}
async function denyOldCapabilities(values: Awaited<ReturnType<typeof capabilities>>, evidence: RecoveryEvidence, stage: string) {
  const facts = [];
  for (const { intent, capability } of values) {
    const before = Date.now(); expect(before).toBeLessThan(Date.parse(capability.expires_at));
    const response = await fetch(capability.url, { redirect: "error" }); const body = Buffer.from(await response.arrayBuffer());
    expect(response.status).toBe(404); expect(Date.now()).toBeLessThan(Date.parse(capability.expires_at));
    expect(response.headers.get("cache-control")).toContain("no-store"); expect(response.headers.get("referrer-policy")).toBe("no-referrer");
    facts.push({ intent, status: response.status, checked_at: now(), expires_at: capability.expires_at, URL_sha256: sha(capability.url),
      body_sha256: sha(body), body_bytes: body.length, refused_before_real_expiry: true });
  }
  evidence.checkpoint(stage, { old_capabilities: facts }); return facts;
}
async function removeParticipant(fixture: ExtrasFixture, actor: RecoveryActor, evidence: RecoveryEvidence) {
  const requestId = randomUUID();
  const receipt = object.parse(await fixture.actors.creator.client.requestJSON(`${rootPath(fixture)}/participants/${actor.id}/remove`, {
    method: "POST", headers: { "Idempotency-Key": requestId }, body: authority(await readIssue(fixture)),
  }));
  expect(receipt.code).toBe("participant_removed"); evidence.checkpoint("real_last_participation_withdrawal", { actor_id: actor.id, request_id: requestId, receipt });
  return receipt;
}
async function deniedUI(session: RecoverySession, fixture: ExtrasFixture, privateTexts: string[]) {
  await expect(session.page.getByText("You no longer have access to this issue.", { exact: true })).toBeVisible({ timeout: 35_000 });
  await expect(session.page.getByRole("heading", { name: fixture.surface.title, exact: true })).toHaveCount(0);
  await expect(session.page.getByRole("dialog")).toHaveCount(0); await expect(composer(session.page)).toHaveCount(0);
  for (const text of privateTexts) await expect(session.page.getByText(text, { exact: true })).toHaveCount(0);
  expect(await deliveryRecords(session)).toEqual([]);
}
function protectedPaths(fixture: ExtrasFixture, file: CollaborationAttachment, commentId: string, draft?: Awaited<ReturnType<typeof normalUpload>>) {
  const root = rootPath(fixture);
  const paths = [`${root}?execution_view_version=6`, `${root}/comments?limit=50&max_bytes=65536`, `${root}/comments/${commentId}`,
    `${root}/comments/${commentId}/history`, `${root}/activity?limit=50`, `${root}/commands?limit=100`, `${root}/attachments`,
    `${root}/attachments/${file.id}`, `${root}/attachments/${file.id}/content`, `${root}/attachments/${file.id}/download`, `${root}/stage`, `${root}/memory`];
  if (draft) paths.push(`${uploadPath}/${draft.fact.draft_id}`, `${uploadPath}/${draft.fact.draft_id}/operations/${draft.fact.request_id}`);
  return paths;
}
async function protectedReads(fixture: ExtrasFixture, actor: RecoveryActor, paths: string[], status: 200 | 404, evidence: RecoveryEvidence, stage: string) {
  const results = [];
  for (const path of paths) {
    const result = await rawActor(fixture, actor, path); results.push(result);
    evidence.checkpoint(`${stage}_endpoint`, { actor_id: actor.id, read: result });
    expect(result.status, `Original authorized endpoint ${path} must return its actual current ${status}`).toBe(status);
    expect(result.redirected).toBe(false);
    if (status === 404) expect(JSON.stringify(result.body)).not.toContain(fixture.surface.title);
  }
  return results;
}

/** Pause a real authorized comment response after the server produced it.
 * ContinueResponse carries no replacement body, headers or status. */
class PausedCommentResponse {
  private cdp: CDPSession | null = null;
  private decision: "release" | "abort" | null = null;
  private release!: () => void;
  private gate = new Promise<void>((resolve) => { this.release = resolve; });
  private pending = new Set<Promise<void>>();
  private cancelledNetworkIds = new Set<string>();
  private first = false;
  fact: { at: string; request_id: string; network_id: string | null; status: number; body_sha256: string; page: z.infer<typeof CollaborationCommentsSchema>;
    released_at: string | null; cancelled_at_fence: boolean; terminal_error: string | null } | null = null;
  private failure: unknown;
  constructor(private session: RecoverySession, private evidence: RecoveryEvidence) {}
  async install() {
    if (process.env.PAY241_S3_RESPONSE_STAGE_HOLD_ACCEPTED !== "1") throw new Error("Main must first accept the natural native response-stage transport probe");
    this.cdp = await this.session.context.newCDPSession(this.session.page);
    this.cdp.on("Fetch.requestPaused", this.handler);
    this.cdp.on("Network.loadingFailed", (event) => { if (event.canceled || event.errorText === "net::ERR_ABORTED") this.cancelledNetworkIds.add(event.requestId); });
    await this.cdp.send("Network.enable");
    await this.cdp.send("Fetch.enable", { patterns: [{ urlPattern: `${this.session.fixture.apiUrl}${rootPath(this.session.fixture)}/comments?*`, requestStage: "Response" }] });
  }
  private handler = (event: { requestId: string; networkId?: string; responseStatusCode?: number; request: { method: string; url: string } }) => {
    const pending = this.handle(event).catch((error) => { this.failure = error; }); this.pending.add(pending);
    void pending.finally(() => this.pending.delete(pending));
  };
  private async handle(event: Parameters<PausedCommentResponse["handler"]>[0]) {
    const cdp = this.cdp!;
    if (this.first || event.request.method !== "GET" || event.responseStatusCode !== 200) { await cdp.send("Fetch.continueResponse", { requestId: event.requestId }); return; }
    this.first = true;
    try {
      const response = await cdp.send("Fetch.getResponseBody", { requestId: event.requestId });
      const body = Buffer.from(response.body, response.base64Encoded ? "base64" : "utf8");
      this.fact = { at: now(), request_id: event.requestId, network_id: event.networkId ?? null, status: event.responseStatusCode,
        body_sha256: sha(body), page: CollaborationCommentsSchema.parse(JSON.parse(body.toString())), released_at: null,
        cancelled_at_fence: false, terminal_error: null };
      this.evidence.checkpoint("real_authorized_comment_GET200_paused_after_server_body", { held: this.fact, replacement_status_headers_body: false });
      await this.gate;
      if (this.decision === "abort") { await cdp.send("Fetch.failRequest", { requestId: event.requestId, errorReason: "Aborted" }); return; }
      try { await cdp.send("Fetch.continueResponse", { requestId: event.requestId }); this.fact.released_at = now(); }
      catch (error) {
        this.fact.terminal_error = error instanceof Error ? error.name : "unknown_error";
        this.fact.cancelled_at_fence = this.fact.network_id !== null && this.cancelledNetworkIds.has(this.fact.network_id);
        if (!this.fact.cancelled_at_fence) throw error;
      }
    } catch (error) { await cdp.send("Fetch.failRequest", { requestId: event.requestId, errorReason: "Aborted" }).catch(() => undefined); throw error; }
  }
  async captured() {
    await expect.poll(() => this.fact !== null || this.failure !== undefined, { timeout: 35_000, message: "An actual current UI comment refetch must reach authorized GET200 before revoke" }).toBe(true);
    if (this.failure) throw this.failure; return this.fact!;
  }
  async forward() {
    await this.captured(); if (this.decision) throw new Error("The original authorized response already has a terminal decision");
    this.decision = "release"; this.release(); await Promise.allSettled([...this.pending]);
    if (this.failure) throw this.failure;
    if (this.fact?.network_id && this.cancelledNetworkIds.has(this.fact.network_id)) this.fact.cancelled_at_fence = true;
    this.evidence.checkpoint("natural_old_authorized_response_terminal_after_access_fence", { held: this.fact, native_fulfill_used: false }); return this.fact!;
  }
  async close() {
    if (!this.decision) { this.decision = "abort"; this.release(); }
    await Promise.allSettled([...this.pending]);
    if (!this.cdp) return;
    const cdp = this.cdp; this.cdp = null; cdp.off("Fetch.requestPaused", this.handler);
    const closed = await Promise.allSettled([cdp.send("Fetch.disable")]); const detached = await Promise.allSettled([cdp.detach()]);
    for (const result of [...closed, ...detached]) if (result.status === "rejected") this.session.cleanup_errors.push({ action: "response_stage_CDP_cleanup", at: now(),
      error_type: result.reason instanceof Error ? result.reason.name : "unknown_error" });
  }
}

async function joinSource(fixture: ExtrasFixture, actor: RecoveryActor, evidence: RecoveryEvidence) {
  const link = z.object({ id: z.uuid(), code: z.string().min(1), workspace_id: z.uuid(), role: z.literal("admin") }).parse(
    await fixture.actors.creator.client.requestJSON(`/api/workspaces/${fixture.control.sourceWorkspace.id}/share-links`, {
      method: "POST", body: { role: "admin", expires_in: 1, max_uses: 1 },
    }));
  const joined = z.object({ workspace_id: z.uuid(), member: z.object({ id: z.uuid(), user_id: z.uuid(), role: z.literal("admin") }) }).parse(
    await actor.client.requestJSON("/api/share-links/join", { method: "POST", body: { code: link.code } }));
  expect(joined.workspace_id).toBe(fixture.control.sourceWorkspace.id); expect(joined.member.user_id).toBe(actor.id);
  const fact = { actor_id: actor.id, link_id: link.id, share_code_sha256: sha(link.code), bounded_uses: 1, source_workspace_id: joined.workspace_id,
    member_id: joined.member.id, role: joined.member.role, password_account_email_not_fabricated: true };
  evidence.checkpoint("actual_public_source_membership_join", fact); return fact;
}
async function removeSource(fixture: ExtrasFixture, membership: Awaited<ReturnType<typeof joinSource>>, evidence: RecoveryEvidence) {
  const result = await rawActor(fixture, fixture.actors.creator, `/api/workspaces/${membership.source_workspace_id}/members/${membership.member_id}`, "DELETE");
  expect(result.status).toBe(204); evidence.checkpoint("real_original_source_member_withdrawal", { membership, result }); return result;
}

export async function exerciseRecoveryRevocations(browser: Browser, platform: RecoveryPlatform, info: TestInfo, evidence: RecoveryEvidence) {
  const run = caseInventory(evidence, ["participant_last_basis_offline_and_delayed_read", "source_only_membership_loss", "source_loss_participation_survives"]);
  const fixture = await seedExtras(evidence);
  return withSessions(browser, platform, fixture.surface, info, evidence, async (open) => {
    const author = await open(fixture.actors.creator), reader = await open(fixture.actors.handler);
    const bytes = Buffer.from(`Private retained revoke content ${randomUUID()}`), privateComment = `Private recovery discussion ${randomUUID()}`;
    const initial = await normalUpload(author, fixture, evidence, "revocation-retained.txt", bytes);
    const delivered = await deliverUploadedComment(author, fixture, evidence, initial.attachment, bytes, privateComment);
    await run("participant_last_basis_offline_and_delayed_read", async () => {
      const before = await readIssue(fixture, fixture.actors.handler);
      await composer(reader.page).getByLabel("Add comment", { exact: true }).fill(`Unsent private participant draft ${randomUUID()}`);
      const draft = await normalUpload(reader, fixture, evidence, "revoked-unbound-draft.txt", Buffer.from(`Unbound private draft ${randomUUID()}`));
      const reads = protectedPaths(fixture, delivered.file, delivered.sent.receipt.comment_id, draft);
      await protectedReads(fixture, reader.actor, reads, 200, evidence, "participant_authorized_before_offline");
      await attachmentPreview(reader, delivered.file, bytes); const old = await capabilities(fixture, reader.actor, delivered.file, bytes, evidence);
      const oldSocket = await authenticatedSocket(reader, String(evidence.value.started_at));
      await disconnectObserver(reader, oldSocket, evidence, "participant_original_socket_and_HTTP_offline");
      const removed = await removeParticipant(fixture, reader.actor, evidence);
      await protectedReads(fixture, reader.actor, reads, 404, evidence, "last_participant_basis_denied_while_offline");
      const oldAfterRemove = await denyOldCapabilities(old, evidence, "old_native_content_and_download_denied_after_remove");
      const onlineAt = now(); await reader.context.setOffline(false);
      await deniedUI(reader, fixture, [bytes.toString(), privateComment, draft.attachment.name]);
      const notifications = await notificationPages(fixture, [reader.actor]);
      expect(notifications[reader.actor.id]!.some((item) => item.event_seq === delivered.sent.receipt.event_seq)).toBe(true);
      for (const item of notifications[reader.actor.id]!) expect(item).toMatchObject({ can_open: false, issue_title: null, source_kind: "issue", source_id: null });
      const joined = await invite(fixture, reader.actor, evidence), afterJoin = await readIssue(fixture, reader.actor);
      expect(afterJoin.write_context?.participant_epoch).not.toBe(before.write_context?.participant_epoch);
      const oldAfterJoin = await denyOldCapabilities(old, evidence, "old_native_stamps_remain_invalid_after_new_participant_epoch");
      await reader.page.reload(); await expect(reader.page.getByRole("heading", { name: fixture.surface.title, exact: true })).toBeVisible();
      const renewedSocket = await authenticatedSocket(reader, onlineAt, oldSocket.id);
      await protectedReads(fixture, reader.actor, protectedPaths(fixture, delivered.file, delivered.sent.receipt.comment_id), 200, evidence, "new_participation_current_reads_restored");
      const delayed = new PausedCommentResponse(reader, evidence);
      try {
        await delayed.install(); await reader.page.reload(); const held = await delayed.captured();
        expect(held.page.items.some((item) => item.id === delivered.sent.receipt.comment_id && item.content === privateComment)).toBe(true);
        const denialAt = now(); const secondRemoval = await removeParticipant(fixture, reader.actor, evidence);
        await deniedUI(reader, fixture, [bytes.toString(), privateComment]);
        const natural = await delayed.forward(); await Promise.allSettled([...reader.pendingReads]);
        await deniedUI(reader, fixture, [bytes.toString(), privateComment]);
        expect(natural.status).toBe(200); expect(natural.released_at !== null || natural.cancelled_at_fence).toBe(true);
        evidence.checkpoint("late_authorized_body_did_not_rehydrate_after_current_denial", { denial_at: denialAt, held: natural,
          second_removal: secondRemoval, outcome: natural.cancelled_at_fence ? "actual_access_cancellation_before_client_body" : "original_response_continued_after_denial",
          server_slow_storage_barrier_claim: false, fabricated_native_fulfill_status_claim: false });
        return { removed, joined, before_participant_epoch: before.write_context?.participant_epoch, after_participant_epoch: afterJoin.write_context?.participant_epoch,
          old_socket_id: oldSocket.id, renewed_socket_id: renewedSocket.id, oldAfterRemove, oldAfterJoin, reads, delayed: natural,
          original_pending_draft: draft, all_private_UI_and_draft_pointers_cleared: true };
      } finally { await delayed.close(); }
    });
    await run("source_only_membership_loss", async () => {
      const source = await newPasswordActor(fixture, "sourceonly"), membership = await joinSource(fixture, source, evidence);
      const issue = await readIssue(fixture, source); expect(issue.write_context ?? null).toBeNull();
      expect(issue.capabilities).toMatchObject({ read: true, comment: true, reorder: true, edit_source_structure: false });
      const workspacePath = `/api/workspaces/${membership.source_workspace_id}`;
      const workspaceBefore = await rawActor(fixture, source, workspacePath); expect(workspaceBefore.status).toBe(200);
      evidence.checkpoint("source_only_current_read_and_membership_basis", { actor_id: source.id, participant_epoch: null,
        source_member_id: membership.member_id, capabilities_before: issue.capabilities, source_workspace_before: workspaceBefore });
      const sourceSession = await open(source), paths = protectedPaths(fixture, delivered.file, delivered.sent.receipt.comment_id);
      await protectedReads(fixture, source, paths, 200, evidence, "source_only_authorized_before_membership_loss");
      const old = await capabilities(fixture, source, delivered.file, bytes, evidence);
      const socket = await authenticatedSocket(sourceSession, String(evidence.value.started_at));
      await disconnectObserver(sourceSession, socket, evidence, "source_only_socket_and_HTTP_offline");
      const removed = await removeSource(fixture, membership, evidence);
      await protectedReads(fixture, source, paths, 404, evidence, "source_only_last_basis_denied");
      const oldDenied = await denyOldCapabilities(old, evidence, "old_source_member_native_stamps_denied");
      await sourceSession.context.setOffline(false); await deniedUI(sourceSession, fixture, [privateComment, bytes.toString()]);
      const workspaces = await source.client.getWorkspaces(); expect(workspaces.some((workspace) => workspace.id === membership.source_workspace_id)).toBe(false);
      const workspaceAfter = await rawActor(fixture, source, workspacePath); expect([403, 404]).toContain(workspaceAfter.status);
      evidence.checkpoint("source_only_last_read_and_reorder_basis_withdrawn", { actor_id: source.id, removed,
        source_workspace_before: workspaceBefore, source_workspace_after: workspaceAfter, private_endpoint_status_after: 404,
        manager_structure_edit_was_never_granted: true, remaining_participant_basis: false });
      return { actor_id: source.id, membership, removed, authorized_participant_epoch: null, source_read_basis_was_real: true,
        authorized_capabilities_before: issue.capabilities, source_workspace_before_status: workspaceBefore.status,
        source_workspace_after_status: workspaceAfter.status, last_read_and_reorder_basis_removed: true, paths, oldDenied };
    });
    await run("source_loss_participation_survives", async () => {
      const survivor = await newPasswordActor(fixture, "survivor"), membership = await joinSource(fixture, survivor, evidence);
      await invite(fixture, survivor, evidence); const before = await readIssue(fixture, survivor);
      expect(before.capabilities).toMatchObject({ read: true, comment: true, reorder: true, edit_source_structure: false });
      expect(before.write_context).toBeTruthy(); expect([before.creator.user_id, before.handler.user_id]).not.toContain(survivor.id);
      const session = await open(survivor), draftText = `Surviving independent participation draft ${randomUUID()}`;
      await composer(session.page).getByLabel("Add comment", { exact: true }).fill(draftText);
      const saved = await deliveryRecords(session); expect(saved.some((record) => record.kind === "form" && record.content === draftText)).toBe(true);
      const old = await capabilities(fixture, survivor, delivered.file, bytes, evidence);
      const workspacePath = `/api/workspaces/${membership.source_workspace_id}`;
      const workspaceBefore = await rawActor(fixture, survivor, workspacePath); expect(workspaceBefore.status).toBe(200);
      const fieldsBefore = await rawActor(fixture, survivor, `${rootPath(fixture)}/field-options`); expect(fieldsBefore.status).toBe(403);
      evidence.checkpoint("nonmanager_participant_source_membership_before_loss", { actor_id: survivor.id,
        participant_epoch: before.write_context?.participant_epoch, source_member_id: membership.member_id,
        capabilities_before: before.capabilities, source_workspace_before: workspaceBefore, field_options_before: fieldsBefore });
      const removed = await removeSource(fixture, membership, evidence), current = await readIssue(fixture, survivor);
      expect(current.write_context?.participant_epoch).toBe(before.write_context?.participant_epoch);
      expect(current.capabilities).toMatchObject({ read: true, comment: true, reorder: false, edit_source_structure: false });
      const oldDenied = await denyOldCapabilities(old, evidence, "old_dual_basis_native_stamps_denied_even_with_surviving_participation");
      await protectedReads(fixture, survivor, protectedPaths(fixture, delivered.file, delivered.sent.receipt.comment_id), 200, evidence, "independent_participation_current_reads_survive_source_loss");
      const sourceWorkspace = await rawActor(fixture, survivor, workspacePath); expect([403, 404]).toContain(sourceWorkspace.status);
      const fields = await rawActor(fixture, survivor, `${rootPath(fixture)}/field-options`); expect(fields.status).toBe(403);
      await expect(session.page.getByRole("heading", { name: fixture.surface.title, exact: true })).toBeVisible();
      await expect(composer(session.page).getByLabel("Add comment", { exact: true })).toHaveValue(draftText); expect(await deliveryRecords(session)).toEqual(saved);
      await attachmentPreview(session, delivered.file, bytes);
      const filePreview = session.page.getByRole("dialog", { name: delivered.file.name, exact: true });
      await filePreview.getByRole("button", { name: "Close preview", exact: true }).click();
      await expect(filePreview).toBeHidden();
      const drawer = session.page.getByRole("dialog", { name: "Attachments", exact: true });
      await drawer.getByRole("button", { name: "Close resources", exact: true }).click();
      await expect(drawer).toBeHidden();
      await preview(session, fixture); const response = await clickResponse(session, `${rootPath(fixture)}/comments`, "POST",
        () => composer(session.page).getByRole("button", { name: "Send comment", exact: true }).click());
      expect(response.status()).toBe(201); const receipt = CollaborationCommentReceiptSchema.parse(await response.json()); expect(receipt.targets).toEqual([]);
      expect((await readComments(fixture, survivor)).some((comment) => comment.id === receipt.comment_id && comment.content === draftText)).toBe(true);
      const newCapabilities = await capabilities(fixture, survivor, delivered.file, bytes, evidence);
      return { actor_id: survivor.id, membership, removed, stable_participant_epoch: current.write_context?.participant_epoch, oldDenied,
        source_workspace_before_status: workspaceBefore.status, source_workspace_status: sourceWorkspace.status,
        manager_field_options_before_status: fieldsBefore.status, manager_field_options_after_status: fields.status,
        reorder_before: before.capabilities.reorder, reorder_after: current.capabilities.reorder,
        manager_structure_edit_was_never_granted: true, current_comments_and_draft_survived: true, new_comment_receipt: receipt,
        fresh_capability_identities: newCapabilities.map(({ intent, capability }) => ({ intent, attachment_id: capability.attachment_id, version: capability.version, URL_sha256: sha(capability.url) })) };
    });
    assertNoExecution(await readIssue(fixture)); expect(await commands(fixture)).toEqual([]); expect(await fixture.control.owner.countIssueDispatches(fixture.issueId)).toBe(0);
    evidence.checkpoint("complete_human_read_basis_inventory", { issue: await readIssue(fixture), runtime_reader_revocation: "NOT_RUN_EXTRAS_B_REQUIRES_ACTUAL_RUN_PERMIT",
      actual_run_record_handoff_ID_denominator: 0, provider_processes: 0, ordinary_dispatches: 0 });
    return { issue_id: fixture.issueId, platform, cases: 3, controlled_runtime_read_claim: false };
  });
}
