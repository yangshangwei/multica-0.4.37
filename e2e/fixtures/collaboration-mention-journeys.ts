import { randomUUID } from "node:crypto";
import { expect, type Locator, type Page, type Route } from "@playwright/test";
import type {
  CollaborationActivity, CollaborationCommentInput, CollaborationCommentReceipt, CollaborationComments,
  CollaborationIssue, CollaborationMentionTargets, CollaborationNotification, CollaborationParticipants,
  CollaborationResourceIdentityView, CollaborationResourceOptions, CollaborationResources,
  CollaborationTriggerPreview, CollaborationWorkbench,
} from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";
import { seedCollaborationResourceOfferJourney } from "./collaboration-resource-offers";

interface ResourceReceipt {
  issue_id: string; status: number; code: string; request_id: string;
  offer_id: string | null; offer_version: string | null; target_id: string | null;
}
interface MentionTarget {
  kind: "agent" | "squad"; resourceId: string; name: string; targetId: string;
}
interface NotificationsPage { items: CollaborationNotification[]; next_cursor: string | null }
export interface MentionJourneyAdapter {
  navigate(page: Page, path: string): Promise<void>;
  capture?(stage: string, page: Page, focus: Locator): Promise<void>;
}

export const mentionEnvironmentReady = () => Boolean(process.env.COLLABORATION_E2E_API_URL && process.env.DATABASE_URL
  && process.env.MULTICA_DEPLOYMENT_ID && process.env.NEXT_PUBLIC_API_URL);
const issuePath = (fixture: { issueId: string }) => `/api/collaboration/issues/${fixture.issueId}`;
export const mentionAccountPath = (fixture: { issueId: string }, view: "current" | "participating" = "participating") =>
  `/account?view=${view}&issue=${fixture.issueId}`;

function authority(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("Mention journey requires a current participant write context");
  return { expected_lifecycle_epoch: issue.lifecycle_epoch, expected_participant_epoch: issue.write_context.participant_epoch,
    expected_account_epoch: issue.write_context.account_epoch };
}

/** Only runtime/enrollment setup is synthetic, via the existing isolated fixture.
 * All targets require real offers, organizer adoption and complete owner consent.
 * No hold is cleared and no daemon, provider or run capability is started. */
export async function seedCollaborationMentionJourney() {
  const fixture = await seedCollaborationResourceOfferJourney();
  const root = issuePath(fixture);
  const suffix = randomUUID().slice(0, 8);
  const outside = new TestApiClient();
  const outsideEmail = `mention-outside-${suffix}@example.invalid`;
  const outsideName = `Outside squad member X ${suffix}`;
  await outside.login(outsideEmail, outsideName);
  await outside.markUserOnboarded();
  const outsideId = (await outside.requestJSON<{ id: string }>("/api/me")).id;
  const invitation = await fixture.owner.requestJSON<{ id: string }>(`/api/workspaces/${fixture.executionWorkspace.id}/members`, {
    method: "POST", body: { email: outsideEmail, role: "member" },
  });
  await outside.requestJSON(`/api/invitations/${invitation.id}/accept`, { method: "POST" });

  const createAgent = (name: string) => fixture.owner.requestJSON<{ id: string; name: string }>("/api/agents", {
    method: "POST", body: { name, runtime_id: fixture.runtimeId, permission_mode: "private" },
  });
  const agentB = await createAgent(`Independent agent B ${suffix}`);
  const leader = await createAgent(`Squad S leader ${suffix}`);
  const member = await createAgent(`Squad S member ${suffix}`);
  const squad = await fixture.owner.requestJSON<{ id: string; name: string }>("/api/squads", {
    method: "POST", body: { name: `Explicit squad S ${suffix}`, leader_id: leader.id },
  });
  for (const [memberType, memberId] of [["agent", member.id], ["member", outsideId]] as const) {
    await fixture.owner.requestJSON(`/api/squads/${squad.id}/members`, {
      method: "POST", body: { member_type: memberType, member_id: memberId, role: "member" },
    });
  }
  const options = await fixture.owner.requestJSON<CollaborationResourceOptions>(
    `${root}/resources/offers/options?execution_workspace_id=${fixture.executionWorkspace.id}`);
  const targets: MentionTarget[] = [];
  for (const [kind, resourceId] of [["agent", fixture.agentId], ["agent", agentB.id], ["squad", squad.id]] as const) {
    const option = options.items.find((item) => item.kind === kind && item.resource_id === resourceId);
    if (!option?.available) throw new Error(`The selected ${kind} is unavailable in the owner's offer inventory`);
    expect(option.resources.map((resource) => resource.agent_id).sort()).toEqual(
      kind === "squad" ? [leader.id, member.id].sort() : [resourceId]);
    const offered = await fixture.owner.requestJSON<ResourceReceipt>(`${root}/resources/offers`, {
      method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
        ...authority(await fixture.owner.requestJSON<CollaborationIssue>(root)), kind, resource_id: resourceId,
        execution_workspace_id: fixture.executionWorkspace.id,
        roster: option.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) =>
          ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id })),
      },
    });
    expect(offered).toMatchObject({ issue_id: fixture.issueId, status: 201, code: "resource_offered", target_id: null });
    if (!offered.offer_id || !offered.offer_version) throw new Error("The owner offer has no durable identity");
    const resources = await fixture.creator.requestJSON<CollaborationResources>(`${root}/resources`);
    const adopted = await fixture.creator.requestJSON<ResourceReceipt>(`${root}/resources/offers/${offered.offer_id}/adopt`, {
      method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
        ...authority(await fixture.creator.requestJSON<CollaborationIssue>(root)), expected_offer_version: offered.offer_version,
        expected_target_set_version: resources.target_set_version,
      },
    });
    expect(adopted).toMatchObject({ issue_id: fixture.issueId, status: 201, code: "resource_offer_adopted", offer_id: offered.offer_id });
    if (!adopted.target_id) throw new Error("Adoption has no target identity");
    const registered = await fixture.owner.requestJSON<CollaborationResources>(`${root}/resources`);
    const target = registered.targets.find((item) => item.target_id === adopted.target_id);
    if (!target) throw new Error("The adopted target is missing from the owner's resource view");
    expect(target.authorized).toBe(false);
    for (const resource of target.resources) {
      const beforeConsent = await fixture.creator.requestJSON<CollaborationMentionTargets>(`${root}/mention-targets`);
      expect(beforeConsent.targets.map((item) => item.target_id)).not.toContain(target.target_id);
      expect(resource.owner_user_id).toBe(fixture.ownerId);
      const identity = await fixture.owner.requestJSON<CollaborationResourceIdentityView>(
        `${root}/resources/identity?target_id=${target.target_id}&agent_id=${resource.agent_id}`);
      expect(identity.identity?.kind).toBe("enrolled");
      const consent = await fixture.owner.requestJSON<ResourceReceipt>(`${root}/targets/${target.target_id}/consents`, {
        method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
          ...authority(await fixture.owner.requestJSON<CollaborationIssue>(root)), agent_id: resource.agent_id,
          expected_target_epoch: target.target_epoch, expected_config_version: target.config_version, identity: identity.identity,
        },
      });
      expect(consent).toMatchObject({ issue_id: fixture.issueId, status: 201, code: "resource_consented", target_id: target.target_id });
    }
    targets.push({ kind, resourceId, name: target.name, targetId: target.target_id });
  }
  const [agentA, independentB, squadS] = targets;
  if (!agentA || !independentB || !squadS) throw new Error("The mixed mention fixture requires A, B and S");
  const mentions = await fixture.creator.requestJSON<CollaborationMentionTargets>(`${root}/mention-targets`);
  expect(mentions.targets.map((target) => target.resource_id).sort()).toEqual([agentA.resourceId, independentB.resourceId, squadS.resourceId].sort());
  expect(mentions.participants.map((person) => person.user_id).sort()).toEqual([fixture.creatorId, fixture.handlerId, fixture.ownerId].sort());
  const handlerName = mentions.participants.find((person) => person.user_id === fixture.handlerId)?.name;
  if (!handlerName) throw new Error("The effective human H is missing from the mention picker");
  const journey = { ...fixture, agentA, agentB: independentB, squadS, targets, outside, outsideId, outsideName, handlerName };
  const initial = await readJourneyState(journey);
  expect(initial.comments.items).toEqual([]);
  assertHeldExecution(initial.issue, 0);
  await assertOutsideRemainsOutside(journey);
  return journey;
}
export type CollaborationMentionJourney = Awaited<ReturnType<typeof seedCollaborationMentionJourney>>;

function actors(fixture: Pick<CollaborationMentionJourney, "creator" | "creatorId" | "handler" | "handlerId" | "owner" | "ownerId" | "outside" | "outsideId">) {
  return [[fixture.creator, fixture.creatorId], [fixture.handler, fixture.handlerId], [fixture.owner, fixture.ownerId], [fixture.outside, fixture.outsideId]] as const;
}
async function readNotifications(fixture: CollaborationMentionJourney) {
  const entries = await Promise.all(actors(fixture).map(async ([actor, id]) => {
    const page = await actor.requestJSON<NotificationsPage>("/api/me/collaboration-notifications?include_archived=true&limit=100");
    expect(page.next_cursor).toBeNull();
    return [id, page.items.filter((item) => item.issue_id === fixture.issueId)] as const;
  }));
  return Object.fromEntries(entries);
}
async function readJourneyState(fixture: CollaborationMentionJourney) {
  const root = issuePath(fixture);
  const [issue, comments, participants, notifications, current, participating] = await Promise.all([
    fixture.creator.requestJSON<CollaborationIssue>(root),
    fixture.creator.requestJSON<CollaborationComments>(`${root}/comments`),
    fixture.creator.requestJSON<CollaborationParticipants>(`${root}/participants`),
    readNotifications(fixture),
    fixture.handler.requestJSON<CollaborationWorkbench>("/api/me/workbench?view=current"),
    fixture.creator.requestJSON<CollaborationWorkbench>("/api/me/workbench?view=participating"),
  ]);
  expect(current.items.map((item) => item.id)).toEqual([fixture.issueId]);
  expect(participating.items.map((item) => item.id)).toEqual([fixture.issueId]);
  expect(participating.total).toBe(1);
  return { issue, comments, participants: participants.participants, notifications,
    current: current.items.map(({ id, handler, status, status_category }) => ({ id, handler, status, status_category })) };
}
type JourneyState = Awaited<ReturnType<typeof readJourneyState>>;

function assertHeldExecution(issue: CollaborationIssue, commands: number) {
  expect(issue.session.queue_state).toBe("paused");
  if (!issue.execution) throw new Error("The held execution projection is missing");
  expect(issue.execution.runs).toEqual([]);
  expect(issue.execution.groups).toEqual([]);
  expect(issue.execution.commands).toHaveLength(commands);
  expect(issue.execution.commands.every((command) => command.state === "waiting" && command.group_id === null)).toBe(true);
  expect(issue.execution.channels.every((channel) => channel.queue_state === "paused" && channel.active_group_id === null)).toBe(true);
  expect(issue.execution.targets.every((target) => target.hold_reason !== null && target.control.queue_state === "paused" && target.active_group_id === null)).toBe(true);
}
function assertHumanState(before: JourneyState, after: JourneyState) {
  expect(after.issue.id).toBe(before.issue.id);
  expect(after.issue.creator).toEqual(before.issue.creator);
  expect(after.issue.handler).toEqual(before.issue.handler);
  expect(after.issue.status).toBe(before.issue.status);
  expect(after.issue.status_category).toBe(before.issue.status_category);
  expect(after.issue.session).toEqual(before.issue.session);
  expect(after.participants).toEqual(before.participants);
  expect(after.current).toEqual(before.current);
}
function addedNotifications(before: JourneyState["notifications"], after: JourneyState["notifications"], actorId: string) {
  const oldIds = new Set(before[actorId]!.map((item) => item.id));
  return after[actorId]!.filter((item) => !oldIds.has(item.id));
}
async function assertOutsideRemainsOutside(fixture: CollaborationMentionJourney) {
  const participating = await fixture.outside.requestJSON<CollaborationWorkbench>("/api/me/workbench?view=participating");
  expect(participating.items).toEqual([]);
  const response = await fetch(`${fixture.apiUrl}${issuePath(fixture)}`, { headers: { Authorization: `Bearer ${fixture.outside.getToken()}` } });
  expect(response.status).toBe(404);
  expect(await response.text()).not.toContain(fixture.title);
  expect((await readNotifications(fixture))[fixture.outsideId]).toEqual([]);
}
const composerFor = (page: Page) => page.getByRole("form", { name: "Comments", exact: true });
async function openCreator(page: Page, fixture: CollaborationMentionJourney, adapter: MentionJourneyAdapter) {
  await adapter.navigate(page, mentionAccountPath(fixture));
  await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
}
async function selectMentions(page: Page, fixture: CollaborationMentionJourney, targets: MentionTarget[]) {
  const composer = composerFor(page);
  await composer.getByRole("group", { name: "People", exact: true }).getByRole("button", { name: fixture.handlerName, exact: true }).click();
  for (const target of targets) {
    await composer.getByRole("group", { name: "AI targets", exact: true }).getByRole("button", { name: target.name, exact: true }).click();
  }
  const content = await composer.getByLabel("Add comment", { exact: true }).inputValue();
  expect(content).toContain(`mention://member/${fixture.handlerId}`);
  for (const target of targets) expect(content).toContain(`mention://${target.kind}/${target.resourceId}`);
  return content;
}
async function previewComment(page: Page, fixture: CollaborationMentionJourney) {
  const responsePending = page.waitForResponse((response) => new URL(response.url()).pathname === `${issuePath(fixture)}/comments/trigger-preview`
    && response.request().method() === "POST");
  await composerFor(page).getByRole("button", { name: "Preview delivery", exact: true }).click();
  const response = await responsePending;
  expect(response.status()).toBe(200);
  const preview: CollaborationTriggerPreview = await response.json();
  expect(preview.issue_id).toBe(fixture.issueId);
  await expect(composerFor(page).getByRole("button", { name: "Send comment", exact: true })).toBeEnabled();
  return preview;
}
function assertPreview(preview: CollaborationTriggerPreview, fixture: CollaborationMentionJourney, targets: MentionTarget[], suppressed: string[] = []) {
  expect(preview.targets).toHaveLength(targets.length);
  for (const target of targets) {
    expect(preview.targets.find((item) => item.resource_id === target.resourceId)).toMatchObject({
      target_id: target.targetId, kind: target.kind, state: suppressed.includes(target.targetId) ? "suppressed" : "waiting", command_id: null,
    });
  }
  expect(preview.recipients.map((item) => [item.recipient.user_id, item.is_mention]).sort()).toEqual(
    [[fixture.handlerId, true], [fixture.ownerId, false]].sort());
}

async function sendPreviewedComment(page: Page, fixture: CollaborationMentionJourney, beforeForward?: () => Promise<void>) {
  const path = `${issuePath(fixture)}/comments`;
  const pattern = `**${path}`;
  let barrierResolve: () => void = () => undefined;
  let barrierReject: (reason: unknown) => void = () => undefined;
  const barrier = beforeForward ? new Promise<void>((resolve, reject) => { barrierResolve = resolve; barrierReject = reject; }) : Promise.resolve();
  let forwarded = 0;
  const intercept = async (route: Route) => {
    if (route.request().method() !== "POST") return route.continue();
    try {
      expect(++forwarded).toBe(1);
      await beforeForward!();
      // Continue the user's original request only after the real owner write.
      // No response or product state is mocked at this boundary.
      await route.continue();
      barrierResolve();
    } catch (error) {
      await route.abort("failed");
      barrierReject(error);
    }
  };
  if (beforeForward) await page.route(pattern, intercept);
  try {
    const responsePending = page.waitForResponse((response) => new URL(response.url()).pathname === path && response.request().method() === "POST");
    const [response] = await Promise.all([responsePending,
      composerFor(page).getByRole("button", { name: "Send comment", exact: true }).click(), barrier]);
    expect(response.status()).toBe(201);
    const receipt: CollaborationCommentReceipt = await response.json();
    expect(receipt).toMatchObject({ issue_id: fixture.issueId, status: 201, code: "comment_created" });
    const requestId = response.request().headers()["idempotency-key"];
    expect(requestId).toMatch(/^[0-9a-f-]{36}$/i);
    if (!requestId) throw new Error("The comment write omitted its original request key");
    const body: CollaborationCommentInput = response.request().postDataJSON();
    await expect(composerFor(page).getByText("Comment confirmed.", { exact: true })).toBeVisible();
    await expect(page.locator(`[id="comment-${receipt.comment_id}"]`)).toBeVisible();
    return { receipt, body, requestId };
  } finally {
    if (beforeForward) await page.unroute(pattern, intercept);
  }
}
type SentComment = Awaited<ReturnType<typeof sendPreviewedComment>>;

async function assertOneComment(fixture: CollaborationMentionJourney, before: JourneyState, sent: SentComment, queued: MentionTarget[]) {
  const after = await readJourneyState(fixture);
  assertHumanState(before, after);
  assertHeldExecution(after.issue, queued.length);
  expect(after.comments.items).toHaveLength(1);
  expect(after.comments.items[0]).toMatchObject({ id: sent.receipt.comment_id, issue_id: fixture.issueId,
    author: { user_id: fixture.creatorId }, content: sent.body.content });
  const commands = after.issue.execution!.commands;
  for (const target of queued) {
    const outcome = sent.receipt.targets.find((item) => item.kind === target.kind && item.resource_id === target.resourceId);
    expect(outcome).toMatchObject({ target_id: target.targetId, state: "waiting", command_sequence: "1" });
    expect(commands.find((command) => command.id === outcome!.command_id)).toMatchObject({ target_id: target.targetId,
      channel_id: outcome!.channel_id, batch_id: outcome!.batch_id, sequence: outcome!.command_sequence,
      input_event_id: sent.receipt.event_id, author_user_id: fixture.creatorId, state: "waiting", group_id: null });
  }
  expect(new Set(commands.map((command) => command.channel_id)).size).toBe(queued.length);
  for (const [actorId, count, mentioned] of [[fixture.creatorId, 0, false], [fixture.handlerId, 1, true],
    [fixture.ownerId, 1, false], [fixture.outsideId, 0, false]] as const) {
    const added = addedNotifications(before.notifications, after.notifications, actorId);
    expect(added).toHaveLength(count);
    for (const item of added) expect(item).toMatchObject({ issue_id: fixture.issueId, kind: "comment_created", source_kind: "comment",
      source_id: sent.receipt.comment_id, event_seq: sent.receipt.event_seq, is_mention: mentioned, can_open: true, read_at: null, archived_at: null });
  }
  expect(await fixture.owner.countIssueDispatches(fixture.issueId)).toBe(0);
  await assertOutsideRemainsOutside(fixture);
  return after;
}

async function notificationRow(page: Page, hash: string) {
  await page.getByRole("button", { name: /^Notifications/ }).click();
  const popup = page.locator('[data-slot="popover-content"]').filter({ has: page.getByText("Notifications", { exact: true }) });
  await expect(popup).toBeVisible();
  const row = popup.getByRole("listitem").filter({ has: page.locator(`a[href$="${hash}"]`) });
  await expect(row).toHaveCount(1);
  await expect(row.getByText("Mentioned you", { exact: true })).toBeVisible();
  return { row, popup };
}
async function openLocatedNotification(page: Page, fixture: CollaborationMentionJourney, hash: string) {
  const { row, popup } = await notificationRow(page, hash);
  const link = row.getByRole("link", { name: fixture.title, exact: true });
  const href = await link.getAttribute("href");
  if (!href) throw new Error("The located notification has no navigation link");
  const destination = new URL(href, page.url());
  expect(destination.pathname).toBe("/account");
  expect(destination.searchParams.get("issue")).toBe(fixture.issueId);
  expect(destination.hash).toBe(hash);
  await link.click();
  await expect(popup).toBeHidden();
  const anchor = page.locator(`[id="${hash.slice(1)}"]`);
  await expect(anchor).toBeFocused();
  await expect(anchor).toBeVisible();
  return anchor;
}

// Backend content tests own the full permission matrix; these journeys own
// real Web/native picker, transport, partial-outcome and located-link wiring.
export async function exerciseMixedMentionJourney(page: Page, humanPage: Page, fixture: CollaborationMentionJourney, adapter: MentionJourneyAdapter) {
  await openCreator(page, fixture, adapter);
  const before = await readJourneyState(fixture);
  const content = await selectMentions(page, fixture, fixture.targets);
  const preview = await previewComment(page, fixture);
  assertPreview(preview, fixture, fixture.targets);
  for (const target of preview.targets) {
    const current = before.issue.execution!.targets.find((item) => item.id === target.target_id)!;
    expect(target).toMatchObject({ target_epoch: current.target_epoch, config_version: current.config_version,
      hold_reason: current.hold_reason, hold_version: current.hold_version, waiting_reason: "issue_paused" });
  }
  await expect(composerFor(page).getByText("2 notification recipients", { exact: true })).toBeVisible();
  await expect(composerFor(page).getByText("Issue paused; explicit resume required", { exact: true })).toHaveCount(3);
  expect(await readJourneyState(fixture)).toEqual(before);
  const targetList = composerFor(page).getByRole("list").filter({ has: page.getByRole("checkbox") });
  await adapter.capture?.("mixed-mention-preview", page, targetList);
  const sent = await sendPreviewedComment(page, fixture);
  expect(sent.body.content).toBe(content);
  expect(sent.receipt.targets).toHaveLength(3);
  const after = await assertOneComment(fixture, before, sent, fixture.targets);
  await adapter.navigate(humanPage, mentionAccountPath(fixture, "current"));
  await expect(humanPage.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  await openLocatedNotification(humanPage, fixture, `#comment-${sent.receipt.comment_id}`);
  return { issue_id: fixture.issueId, request_id: sent.requestId, receipt: sent.receipt,
    human_notification_ids: addedNotifications(before.notifications, after.notifications, fixture.handlerId).map((item) => item.id),
    outside_squad_member_id: fixture.outsideId, runtime_scope: "synthetic offline enrollment; no execution" };
}

export async function exercisePartialMentionRefusal(page: Page, humanPage: Page, fixture: CollaborationMentionJourney, adapter: MentionJourneyAdapter) {
  await openCreator(page, fixture, adapter);
  const before = await readJourneyState(fixture);
  const targets = [fixture.agentA, fixture.agentB];
  const content = await selectMentions(page, fixture, targets);
  assertPreview(await previewComment(page, fixture), fixture, targets);
  const selectedA = composerFor(page).getByRole("checkbox", { name: new RegExp(fixture.agentA.name) });
  await expect(selectedA).toBeChecked();
  // Changing suppression retires this preview; inspect the next preview's state.
  await selectedA.click();
  await expect(composerFor(page).getByRole("button", { name: "Send comment", exact: true })).toBeDisabled();
  await expect(composerFor(page).getByLabel("Add comment", { exact: true })).toHaveValue(content);
  assertPreview(await previewComment(page, fixture), fixture, targets, [fixture.agentA.targetId]);
  await expect(selectedA).not.toBeChecked();
  await expect(composerFor(page).getByText("AI trigger suppressed", { exact: true })).toBeVisible();
  await selectedA.click();
  const preview = await previewComment(page, fixture);
  assertPreview(preview, fixture, targets);
  await expect(selectedA).toBeChecked();
  expect(await readJourneyState(fixture)).toEqual(before);

  let revokedGrantId: string | null = null;
  const sent = await sendPreviewedComment(page, fixture, async () => {
    const root = issuePath(fixture);
    const resources = await fixture.owner.requestJSON<CollaborationResources>(`${root}/resources`);
    const resource = resources.targets.find((target) => target.target_id === fixture.agentA.targetId)?.resources[0];
    if (!resource?.grant_id || !resource.grant_epoch) throw new Error("A has no current owner grant to revoke");
    revokedGrantId = resource.grant_id;
    const revoked = await fixture.owner.requestJSON<ResourceReceipt>(`${root}/grants/${resource.grant_id}/revoke`, {
      method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
        ...authority(await fixture.owner.requestJSON<CollaborationIssue>(root)), expected_grant_epoch: resource.grant_epoch,
      },
    });
    expect(revoked).toMatchObject({ issue_id: fixture.issueId, status: 200, code: "grant_revoked" });
    const current = await fixture.owner.requestJSON<CollaborationResources>(`${root}/resources`);
    expect(current.targets.find((target) => target.target_id === fixture.agentA.targetId)?.authorized).toBe(false);
    expect(current.targets.find((target) => target.target_id === fixture.agentB.targetId)?.authorized).toBe(true);
    expect(current.targets.filter((target) => target.target_id !== fixture.agentA.targetId)).toEqual(
      resources.targets.filter((target) => target.target_id !== fixture.agentA.targetId));
  });
  expect(revokedGrantId).not.toBeNull();
  expect(sent.body.content).toBe(content);
  expect(sent.body.target_versions.map((target) => target.target_id).sort()).toEqual(targets.map((target) => target.targetId).sort());
  expect(sent.receipt.targets).toHaveLength(2);
  expect(sent.receipt.targets.find((target) => target.resource_id === fixture.agentA.resourceId)).toMatchObject({
    kind: "agent", state: "blocked", code: "target_unavailable", command_id: null,
  });
  const result = composerFor(page).getByRole("status").filter({ hasText: "Comment confirmed." });
  const refusal = result.getByText(/Target is unavailable or its configuration changed/);
  await expect(refusal).toBeVisible();
  await expect(refusal).toContainText(new RegExp(`${fixture.agentA.name}|${fixture.agentA.resourceId}`));
  await expect(result.getByText(new RegExp(fixture.agentB.name))).toBeVisible();
  await adapter.capture?.("partial-mention-refusal", page, result);
  const after = await assertOneComment(fixture, before, sent, [fixture.agentB]);
  const replay = await fixture.creator.requestJSON<CollaborationCommentReceipt>(`${issuePath(fixture)}/comments`, {
    method: "POST", headers: { "Idempotency-Key": sent.requestId }, body: sent.body,
  });
  expect(replay).toEqual(sent.receipt);
  expect(await readJourneyState(fixture)).toEqual(after);

  await composerFor(page).getByLabel("Add comment", { exact: true }).fill(content);
  const unavailablePreview = await previewComment(page, fixture);
  expect(unavailablePreview.targets).toHaveLength(2);
  expect(unavailablePreview.recipients).toEqual(preview.recipients);
  expect(unavailablePreview.targets.find((target) => target.resource_id === fixture.agentA.resourceId)).toMatchObject({
    kind: "agent", target_id: null, state: "blocked", code: "target_unavailable", command_id: null,
  });
  expect(unavailablePreview.targets.find((target) => target.resource_id === fixture.agentB.resourceId)).toMatchObject({
    kind: "agent", target_id: fixture.agentB.targetId, state: "waiting", code: "command_queued", command_id: null,
    waiting_reason: "issue_paused",
  });
  const unavailableA = composerFor(page).getByRole("checkbox", { name: new RegExp(`${fixture.agentA.name}|${fixture.agentA.resourceId}`) });
  await expect(unavailableA).toBeVisible();
  await expect(unavailableA).toBeDisabled();
  await expect(unavailableA).not.toBeChecked();
  await expect(unavailableA).toHaveAccessibleName(/Target is unavailable or its configuration changed/);
  await expect(composerFor(page).getByLabel("Add comment", { exact: true })).toHaveValue(content);
  expect(await readJourneyState(fixture)).toEqual(after);
  const previewTargets = composerFor(page).getByRole("list").filter({ has: page.getByRole("checkbox") });
  await adapter.capture?.("unavailable-mention-preview", page, previewTargets);

  await adapter.navigate(humanPage, mentionAccountPath(fixture, "current"));
  await expect(humanPage.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  await openLocatedNotification(humanPage, fixture, `#comment-${sent.receipt.comment_id}`);
  return { issue_id: fixture.issueId, preview, request_id: sent.requestId, revoked_grant_id: revokedGrantId, receipt: sent.receipt,
    replay_identical: true, unavailable_preview: unavailablePreview, unavailable_preview_read_only: true,
    human_notification_ids: addedNotifications(before.notifications, after.notifications, fixture.handlerId).map((item) => item.id) };
}

const personMention = (id: string, name: string) => `[@${name}](mention://member/${id})`;
function editOnlyMentions(fixture: CollaborationMentionJourney) {
  return [personMention(fixture.creatorId, "Creator P"), personMention(fixture.outsideId, fixture.outsideName),
    ...fixture.targets.map((target) => `[@${target.name}](mention://${target.kind}/${target.resourceId})`)].join(" ");
}

/** Called only after H's first browser context/application has closed. */
export async function sendOfflineHumanMention(page: Page, fixture: CollaborationMentionJourney, adapter: MentionJourneyAdapter) {
  await openCreator(page, fixture, adapter);
  const before = await readJourneyState(fixture);
  await selectMentions(page, fixture, []);
  const input = composerFor(page).getByLabel("Add comment", { exact: true });
  await input.fill(`${await input.inputValue()} Durable offline comment. ${personMention(fixture.creatorId, "Creator P")} ${personMention(fixture.outsideId, fixture.outsideName)}`);
  assertPreview(await previewComment(page, fixture), fixture, []);
  const sent = await sendPreviewedComment(page, fixture);
  const after = await assertOneComment(fixture, before, sent, []);
  return { sent, before, after };
}

async function editComment(page: Page, fixture: CollaborationMentionJourney, commentId: string, content: string) {
  const row = page.locator(`[id="comment-${commentId}"]`);
  await row.getByRole("button", { name: "Edit comment", exact: true }).click();
  await row.getByLabel("Edit comment text", { exact: true }).fill(content);
  const responsePending = page.waitForResponse((response) => new URL(response.url()).pathname === `${issuePath(fixture)}/comments/${commentId}`
    && response.request().method() === "PATCH");
  await row.getByRole("button", { name: "Save comment", exact: true }).click();
  const response = await responsePending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationCommentReceipt = await response.json();
  expect(receipt).toMatchObject({ issue_id: fixture.issueId, comment_id: commentId, targets: [] });
  await expect(row.getByRole("form", { name: "Edit comment", exact: true })).toHaveCount(0);
  expect((await fixture.creator.requestJSON<CollaborationComments>(`${issuePath(fixture)}/comments`)).items[0]?.content).toBe(content);
  return receipt;
}
async function editBody(page: Page, fixture: CollaborationMentionJourney, description: string) {
  await page.getByRole("button", { name: "Edit issue", exact: true }).click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: "Edit issue", exact: true }) });
  await form.getByLabel("Description", { exact: true }).fill(description);
  const responsePending = page.waitForResponse((response) => new URL(response.url()).pathname === issuePath(fixture) && response.request().method() === "PATCH");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  expect((await responsePending).status()).toBe(200);
  await expect(form).toHaveCount(0);
  expect((await fixture.creator.requestJSON<CollaborationIssue>(issuePath(fixture))).description).toBe(description);
}
function assertEditRecipients(fixture: CollaborationMentionJourney, before: JourneyState, after: JourneyState, recipientId: string | null) {
  assertHumanState(before, after);
  assertHeldExecution(after.issue, 0);
  expect(after.issue.execution).toEqual(before.issue.execution);
  for (const [, actorId] of actors(fixture)) {
    const added = addedNotifications(before.notifications, after.notifications, actorId);
    expect(added).toHaveLength(actorId === recipientId ? 1 : 0);
    for (const item of added) expect(item).toMatchObject({ issue_id: fixture.issueId, kind: "mention_added", is_mention: true, can_open: true });
  }
}
async function assertBodyEvent(fixture: CollaborationMentionJourney, notification: CollaborationNotification, before: string, after: string) {
  expect(notification.source_kind).toBe("body");
  expect(notification.source_id).toMatch(/^[0-9a-f-]{36}$/i);
  const history = await fixture.creator.requestJSON<CollaborationActivity>(`${issuePath(fixture)}/activity?limit=100`);
  expect(history.items.find((event) => event.id === notification.source_id)).toMatchObject({ issue_id: fixture.issueId,
    kind: "issue_edited", event_seq: notification.event_seq, actor: { user_id: fixture.creatorId },
    changes: expect.arrayContaining([{ field: "description", before, after }]) });
}

export async function exerciseDurableMentionNotifications(page: Page, humanPage: Page, fixture: CollaborationMentionJourney,
  offline: Awaited<ReturnType<typeof sendOfflineHumanMention>>, adapter: MentionJourneyAdapter) {
  await adapter.navigate(humanPage, mentionAccountPath(fixture, "current"));
  await expect(humanPage.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  const commentId = offline.sent.receipt.comment_id;
  const hash = `#comment-${commentId}`;
  const comment = await openLocatedNotification(humanPage, fixture, hash);
  await expect(comment).toContainText("Durable offline comment.");
  await adapter.capture?.("durable-mention-comment", humanPage, comment);
  const notification = addedNotifications(offline.before.notifications, offline.after.notifications, fixture.handlerId)[0]!;
  const { row, popup } = await notificationRow(humanPage, hash);
  for (const [action, label] of [["read", "Mark read"], ["archive", "Archive"]] as const) {
    const responsePending = humanPage.waitForResponse((response) => new URL(response.url()).pathname === `/api/me/collaboration-notifications/${notification.id}/${action}`
      && response.request().method() === "POST");
    await row.getByRole("button", { name: label, exact: true }).click();
    const response = await responsePending;
    expect(response.status()).toBe(200);
    expect(await response.json()).toMatchObject({ id: notification.id, [action === "read" ? "read_at" : "archived_at"]: expect.any(String) });
    const current = await readJourneyState(fixture);
    assertHumanState(offline.after, current);
    assertHeldExecution(current.issue, 0);
    if (action === "read") await expect(row.getByRole("button", { name: "Mark read", exact: true })).toHaveCount(0);
  }
  await expect(row).toHaveCount(0);
  await humanPage.getByRole("button", { name: /^Notifications/ }).click();
  await expect(popup).toBeHidden();

  let before = await readJourneyState(fixture);
  const firstEdit = `${offline.sent.body.content} Added owner D. ${personMention(fixture.ownerId, "Owner D")} ${editOnlyMentions(fixture)}`;
  const editReceipt = await editComment(page, fixture, commentId, firstEdit);
  let after = await readJourneyState(fixture);
  assertEditRecipients(fixture, before, after, fixture.ownerId);
  const ownerCommentNotification = addedNotifications(before.notifications, after.notifications, fixture.ownerId)[0]!;
  expect(ownerCommentNotification).toMatchObject({ source_kind: "comment", source_id: commentId, event_seq: editReceipt.event_seq });
  before = after;
  await editComment(page, fixture, commentId, `${firstEdit} Only surrounding wording changes.`);
  after = await readJourneyState(fixture);
  assertEditRecipients(fixture, before, after, null);

  before = after;
  const firstBody = `Located body update. ${personMention(fixture.handlerId, fixture.handlerName)} ${editOnlyMentions(fixture)}`;
  await editBody(page, fixture, firstBody);
  after = await readJourneyState(fixture);
  assertEditRecipients(fixture, before, after, fixture.handlerId);
  const humanBodyNotification = addedNotifications(before.notifications, after.notifications, fixture.handlerId)[0]!;
  await assertBodyEvent(fixture, humanBodyNotification, before.issue.description, firstBody);
  const body = await openLocatedNotification(humanPage, fixture, "#description");
  await expect(body).toContainText("Located body update.");
  await adapter.capture?.("durable-mention-body", humanPage, body);

  before = after;
  const secondBody = `${firstBody} ${personMention(fixture.ownerId, "Owner D")}`;
  await editBody(page, fixture, secondBody);
  after = await readJourneyState(fixture);
  assertEditRecipients(fixture, before, after, fixture.ownerId);
  const ownerBodyNotification = addedNotifications(before.notifications, after.notifications, fixture.ownerId)[0]!;
  await assertBodyEvent(fixture, ownerBodyNotification, firstBody, secondBody);
  before = after;
  await editBody(page, fixture, `${secondBody} Only surrounding wording changes.`);
  after = await readJourneyState(fixture);
  assertEditRecipients(fixture, before, after, null);
  expect(after.comments.items).toHaveLength(1);
  expect(after.comments.items[0]?.id).toBe(commentId);
  await assertOutsideRemainsOutside(fixture);
  expect(await fixture.owner.countIssueDispatches(fixture.issueId)).toBe(0);
  return { issue_id: fixture.issueId, offline_comment_id: commentId, comment_notification_id: notification.id,
    read_and_archived: after.notifications[fixture.handlerId]!.find((item) => item.id === notification.id),
    comment_edit_receipt: editReceipt, owner_comment_notification: ownerCommentNotification,
    human_body_notification: humanBodyNotification, owner_body_notification: ownerBodyNotification,
    retained_participant_ids: after.participants.map((person) => person.user_id), ai_commands: 0, ai_runs: 0 };
}
