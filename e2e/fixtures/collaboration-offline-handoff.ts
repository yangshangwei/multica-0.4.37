import { randomUUID } from "node:crypto";
import { expect, type Page } from "@playwright/test";
import type { CollaborationIssue, CollaborationWorkbench, CollaborationOperation } from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";

interface Notification {
  id: string; issue_id: string; event_seq: string; kind: string; issue_title: string | null;
  source_kind: "issue" | "comment" | "body"; source_id: string | null;
  can_open: boolean; read_at: string | null; archived_at: string | null;
}
interface NotificationPage { items: Notification[]; unread_count: number; next_cursor: string | null }

export function offlineHandoffEnvironment() {
  if (process.env.E2E_PASSWORD_AUTH !== "1") throw new Error("Offline handoff requires the owned password acceptance environment");
  const apiUrl = process.env.COLLABORATION_E2E_API_URL;
  for (const value of [apiUrl, process.env.NEXT_PUBLIC_API_URL, process.env.DATABASE_URL]) {
    if (!value || !["localhost", "127.0.0.1"].includes(new URL(value).hostname)) throw new Error("Offline handoff requires explicit loopback task services");
  }
  expect(new URL(apiUrl!).origin).toBe(new URL(process.env.NEXT_PUBLIC_API_URL!).origin);
  expect(["", "multica", "postgres", "template0", "template1"]).not.toContain(new URL(process.env.DATABASE_URL!).pathname.slice(1));
  return { apiUrl: apiUrl! };
}

async function passwordActor(apiUrl: string, username: string, name: string) {
  const client = new TestApiClient();
  const password = `Offline-${randomUUID()}!`;
  const user = await client.registerPassword(username, password, name);
  expect((await client.requestJSON<{ onboarded_at: string | null }>("/api/me/onboarding/complete", { method: "POST", body: {} })).onboarded_at).not.toBeNull();
  let logins = 0;
  return { client, id: user.id, name, get logins() { return logins; },
    async login() {
      const loggedIn = await client.loginPassword(username, password);
      expect(loggedIn.id).toBe(user.id);
      logins += 1;
      return client.getToken()!;
    },
    async authenticateWeb(page: Page) {
      const response = await page.request.post(`${apiUrl}/auth/login`, { data: { username, password } });
      expect(response.status()).toBe(200);
      expect((await response.json()).user.id).toBe(user.id);
      expect((await page.context().cookies(apiUrl)).some((cookie) => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
      logins += 1;
      await page.addInitScript(() => {
        localStorage.setItem("multica-locale", "en");
        localStorage.setItem("multica:chat:isOpen", "false");
      });
    },
  };
}
export type HandoffActor = Awaited<ReturnType<typeof passwordActor>>;

export async function seedOfflineHandoff() {
  const { apiUrl } = offlineHandoffEnvironment();
  const configuration = await fetch(`${apiUrl}/api/config`);
  expect(configuration.status).toBe(200);
  expect(await configuration.json()).toMatchObject({ auth_mode: "password" });
  const suffix = randomUUID().replaceAll("-", "").slice(0, 12);
  const creator = await passwordActor(apiUrl, `s6offp${suffix}`, "Offline creator P");
  const recipient = await passwordActor(apiUrl, `s6offh${suffix}`, "Offline external handler H");
  expect(await recipient.client.getWorkspaces()).toEqual([]);
  const workspace = await creator.client.ensureWorkspace("Offline handoff source", `offline-${suffix}`);
  expect(await creator.client.requestJSON("/api/collaboration/creation-options")).toMatchObject({ creation_enabled: true });
  const issue = await creator.client.requestJSON<CollaborationIssue>("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      source_workspace_id: workspace.id, handler_user_id: creator.id,
      title: `Offline handoff ${suffix}`, description: "Private offline handoff discussion and retained responsibility.", attachment_ids: [],
    },
  });
  expect(issue.handler.user_id).toBe(creator.id);
  return { apiUrl, creator, recipient, workspace, issueId: issue.id, title: issue.title,
    accountPath: `/account?view=participating&issue=${issue.id}` };
}
export type OfflineHandoffFixture = Awaited<ReturnType<typeof seedOfflineHandoff>>;
export interface HandoffSession { page: Page; navigate(path: string): Promise<void>; close(): Promise<void> }
export interface HandoffClients { open(actor: HandoffActor, path: string): Promise<HandoffSession> }

function observations(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("Handoff requires the current participant observations");
  return { expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
}
const issuePath = (fixture: OfflineHandoffFixture) => `/api/collaboration/issues/${fixture.issueId}`;
const issueFor = (fixture: OfflineHandoffFixture, actor = fixture.creator) => actor.client.requestJSON<CollaborationIssue>(issuePath(fixture));
export async function handoffNotifications(actor: HandoffActor, issueId: string) {
  const result = await actor.client.requestJSON<NotificationPage>("/api/me/collaboration-notifications?include_archived=true&limit=100");
  expect(result.next_cursor).toBeNull();
  return result.items.filter((notification) => notification.issue_id === issueId);
}
async function lists(actor: HandoffActor, issueId: string) {
  const current = await actor.client.requestJSON<CollaborationWorkbench>("/api/me/workbench?view=current");
  const participating = await actor.client.requestJSON<CollaborationWorkbench>("/api/me/workbench?view=participating");
  return { current, participating, currentIds: current.items.map((item) => item.id), participatingIds: participating.items.map((item) => item.id), issueId };
}
async function assertLists(actor: HandoffActor, fixture: OfflineHandoffFixture, current: boolean, participating: boolean) {
  await expect.poll(async () => { const result = await lists(actor, fixture.issueId); return [result.currentIds, result.participatingIds]; })
    .toEqual([current ? [fixture.issueId] : [], participating ? [fixture.issueId] : []]);
  return lists(actor, fixture.issueId);
}
function assertExecutionAndBusiness(before: CollaborationIssue, after: CollaborationIssue) {
  expect(after).toMatchObject({ id: before.id, title: before.title, description: before.description, status: before.status,
    creator: before.creator, source_workspace: before.source_workspace, session: before.session, execution: before.execution });
  expect(after.execution?.runs ?? []).toEqual([]);
  expect(after.execution?.targets ?? []).toEqual([]);
  expect(after.execution?.commands ?? []).toEqual([]);
}

async function uiWorkflow(session: HandoffSession, fixture: OfflineHandoffFixture, actor: HandoffActor,
  action: "invite" | "transfer", target: HandoffActor, note = "") {
  const page = session.page;
  await page.getByRole("button", { name: action === "invite" ? "Add participant" : "Transfer", exact: true }).click();
  const form = page.locator("form").filter({ has: page.getByRole("heading", { name: action === "invite" ? "Add participant" : "Transfer", exact: true }) });
  await form.getByLabel("Exact recipient", { exact: true }).fill(target.id);
  await form.getByRole("button", { name: "Find account", exact: true }).click();
  await expect(form.getByText(`Selected account: ${target.name}`, { exact: true })).toBeVisible();
  if (action === "transfer") await form.getByLabel("Note", { exact: true }).fill(note);
  const path = `${issuePath(fixture)}/${action === "invite" ? "participants" : "handoffs"}`;
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === path && response.request().method() === "POST");
  await form.getByRole("button", { name: "Confirm", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  const receipt: CollaborationOperation = await response.json();
  expect(receipt.code).toBe(action === "invite" ? "participant_added" : "transferred");
  const key = response.request().headers()["idempotency-key"]!;
  expect(key).toBeTruthy();
  const body: unknown = response.request().postDataJSON();
  await expect(form).toHaveCount(0);
  const beforeReplay = await issueFor(fixture);
  const beforeNotifications = await handoffNotifications(fixture.recipient, fixture.issueId);
  let replay: CollaborationOperation;
  if (action === "transfer" && actor.id !== fixture.creator.id) {
    const refused = await fetch(`${fixture.apiUrl}${path}`, { method: "POST", headers: {
      "Content-Type": "application/json", Authorization: `Bearer ${actor.client.getToken()!}`, "Idempotency-Key": key,
    }, body: JSON.stringify(body) });
    expect(refused.status).toBe(403);
    replay = await actor.client.requestJSON<CollaborationOperation>(`${issuePath(fixture)}/operations/${key}`);
    expect(replay.code).toBe(receipt.code);
    expect(replay.collaboration_revision).toBe(receipt.collaboration_revision);
  } else {
    replay = await actor.client.requestJSON<CollaborationOperation>(path, { method: "POST", body, headers: { "Idempotency-Key": key } });
    expect(replay).toEqual(receipt);
  }
  expect(await issueFor(fixture)).toEqual(beforeReplay);
  expect(await handoffNotifications(fixture.recipient, fixture.issueId)).toEqual(beforeNotifications);
  const activity = await actor.client.requestJSON<{ items: { kind: string; handoff: { note: string } | null }[] }>(`${issuePath(fixture)}/activity`);
  if (action === "transfer") expect(activity.items.filter((item) => item.kind === "transferred" && item.handoff?.note === note)).toHaveLength(1);
  return { request_id: key, body, receipt, original_key_replay: replay, current_issue: beforeReplay };
}

async function popupFor(page: Page) {
  await page.getByRole("button", { name: /^Notifications/ }).click();
  const popup = page.locator('[data-slot="popover-content"]').filter({ has: page.getByText("Notifications", { exact: true }) });
  await expect(popup).toBeVisible();
  return popup;
}
async function archiveTransfer(session: HandoffSession, fixture: OfflineHandoffFixture, notification: Notification) {
  const page = session.page;
  const popup = await popupFor(page);
  const row = popup.getByRole("listitem").filter({ has: page.getByText("Transferred", { exact: true }) });
  await expect(row).toHaveCount(1);
  const link = row.getByRole("link", { name: fixture.title, exact: true });
  const href = (await link.getAttribute("href"))!;
  expect(new URL(href, fixture.apiUrl).searchParams.get("issue")).toBe(fixture.issueId);
  const before = await issueFor(fixture);
  for (const [action, label] of [["read", "Mark read"], ["archive", "Archive"]] as const) {
    const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `/api/me/collaboration-notifications/${notification.id}/${action}` && response.request().method() === "POST");
    await row.getByRole("button", { name: label, exact: true }).click();
    const response = await pending;
    expect(response.status()).toBe(200);
    expect(await response.json()).toMatchObject({ id: notification.id, [action === "read" ? "read_at" : "archived_at"]: expect.any(String) });
    expect(await issueFor(fixture)).toEqual(before);
    await assertLists(fixture.recipient, fixture, false, true);
  }
  await expect(row).toHaveCount(0);
  await page.getByRole("button", { name: /^Notifications/ }).click();
  await expect(popup).toBeHidden();
  return { notification_id: notification.id, old_href: href, state: (await handoffNotifications(fixture.recipient, fixture.issueId)).find((item) => item.id === notification.id) };
}

async function checkVisibleLists(session: HandoffSession, fixture: OfflineHandoffFixture, actor: HandoffActor, current: boolean) {
  await session.navigate("/account?view=current");
  const region = session.page.getByRole("region", { name: "Collaboration issues", exact: true });
  await expect(region.getByRole("link", { name: new RegExp(fixture.title) })).toHaveCount(current ? 1 : 0);
  await expect(region.getByText(current ? "1 issues" : "0 issues", { exact: true })).toBeVisible();
  await session.page.getByRole("link", { name: "Participating", exact: true }).click();
  await expect(region.getByRole("link", { name: new RegExp(fixture.title) })).toHaveCount(1);
  await region.getByRole("link", { name: new RegExp(fixture.title) }).click();
  await expect(session.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  await assertLists(actor, fixture, current, true);
}

export async function exerciseOfflineHandoff(fixture: OfflineHandoffFixture, clients: HandoffClients) {
  const source = await clients.open(fixture.creator, fixture.accountPath);
  const priorRecipient = await clients.open(fixture.recipient, "/account?view=current");
  await expect(priorRecipient.page.getByText("No issues need your handling.", { exact: true })).toBeVisible();
  await priorRecipient.close();
  expect(priorRecipient.page.isClosed()).toBe(true);
  const loginBeforeInvite = fixture.recipient.logins;
  const original = await issueFor(fixture);
  const invite = await uiWorkflow(source, fixture, fixture.creator, "invite", fixture.recipient);
  const invited = await issueFor(fixture);
  expect(invited.handler.user_id).toBe(fixture.creator.id);
  assertExecutionAndBusiness(original, invited);
  const invitedSourceLists = await assertLists(fixture.creator, fixture, true, true);
  const invitedRecipientLists = await assertLists(fixture.recipient, fixture, false, true);
  const assigned = await uiWorkflow(source, fixture, fixture.creator, "transfer", fixture.recipient, "Offline assignment requires no acceptance step.");
  const afterAssignment = await issueFor(fixture);
  expect(afterAssignment.handler.user_id).toBe(fixture.recipient.id);
  assertExecutionAndBusiness(original, afterAssignment);
  const assignedSourceLists = await assertLists(fixture.creator, fixture, false, true);
  const assignedRecipientLists = await assertLists(fixture.recipient, fixture, true, true);
  expect(fixture.recipient.logins).toBe(loginBeforeInvite);
  expect(await fixture.recipient.client.getWorkspaces()).toEqual([]);
  const noticeBeforeLogin = await handoffNotifications(fixture.recipient, fixture.issueId);
  expect(noticeBeforeLogin.filter((item) => item.kind === "participant_added")).toHaveLength(1);
  expect(noticeBeforeLogin.filter((item) => item.kind === "transferred")).toHaveLength(1);
  expect(await handoffNotifications(fixture.creator, fixture.issueId)).toEqual([]);
  const commentAuthority = observations(await issueFor(fixture));
  const comment = await fixture.creator.client.requestJSON<{ comment_id: string }>(`${issuePath(fixture)}/comments`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      expected_lifecycle_epoch: commentAuthority.expected_lifecycle_epoch, expected_participant_epoch: commentAuthority.expected_participant_epoch,
      expected_account_epoch: commentAuthority.expected_account_epoch, target_versions: [],
      content: "Retained discussion available after offline assignment and notification archive." },
  });
  const active = await clients.open(fixture.recipient, `/account?view=current&issue=${fixture.issueId}`);
  expect(fixture.recipient.logins).toBe(loginBeforeInvite + 1);
  await expect(active.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
  await expect(active.page.getByRole("button", { name: /Accept (invitation|assignment)|Join collaboration/i })).toHaveCount(0);
  await checkVisibleLists(active, fixture, fixture.recipient, true);
  const accountIssue = await issueFor(fixture, fixture.recipient);
  expect(accountIssue.capabilities.transfer).toBe(true);
  expect(accountIssue.execution?.targets ?? []).toEqual([]);
  expect(await fixture.recipient.client.requestJSON(`${issuePath(fixture)}/mention-targets`)).toMatchObject({ targets: [] });
  const beforeTransferNotices = await handoffNotifications(fixture.recipient, fixture.issueId);
  const transferBack = await uiWorkflow(active, fixture, fixture.recipient, "transfer", fixture.creator, "Former handler hands responsibility back to creator.");
  const afterTransfer = await issueFor(fixture);
  expect(afterTransfer.handler.user_id).toBe(fixture.creator.id);
  assertExecutionAndBusiness(original, afterTransfer);
  expect(await handoffNotifications(fixture.recipient, fixture.issueId)).toEqual(beforeTransferNotices);
  const transferredRecipientLists = await assertLists(fixture.recipient, fixture, false, true);
  const transferNotification = beforeTransferNotices.find((item) => item.kind === "transferred")!;
  const archived = await archiveTransfer(active, fixture, transferNotification);
  await active.close();
  expect(active.page.isClosed()).toBe(true);
  const fresh = await clients.open(fixture.recipient, "/account?view=current");
  expect(fixture.recipient.logins).toBe(loginBeforeInvite + 2);
  expect(fresh.page).not.toBe(active.page);
  await checkVisibleLists(fresh, fixture, fixture.recipient, false);
  const reopenedRecipientLists = await assertLists(fixture.recipient, fixture, false, true);
  await expect(fresh.page.locator(`[id="comment-${comment.comment_id}"]`)).toContainText("Retained discussion available after offline assignment");
  const popup = await popupFor(fresh.page);
  const commentRow = popup.getByRole("listitem").filter({ has: fresh.page.locator(`a[href$="#comment-${comment.comment_id}"]`) });
  await expect(commentRow).toHaveCount(1);
  const commentHref = (await commentRow.getByRole("link").getAttribute("href"))!;
  await commentRow.getByRole("link").click();
  await expect(popup).toBeHidden();
  await expect(fresh.page.locator(`[id="comment-${comment.comment_id}"]`)).toBeFocused();
  expect((await handoffNotifications(fixture.recipient, fixture.issueId)).find((item) => item.id === transferNotification.id)).toMatchObject({ read_at: expect.any(String), archived_at: expect.any(String), can_open: true });

  const removal = await fixture.creator.client.requestJSON<CollaborationOperation>(`${issuePath(fixture)}/participants/${fixture.recipient.id}/remove`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: observations(await issueFor(fixture)),
  });
  expect(removal.code).toBe("participant_removed");
  await fresh.page.reload();
  await expect(fresh.page.getByText("You no longer have access to this issue.", { exact: true })).toBeVisible();
  await expect(fresh.page.getByRole("heading", { name: fixture.title, exact: true })).toHaveCount(0);
  await expect(fresh.page.getByText("Retained discussion available after offline assignment and notification archive.", { exact: true })).toHaveCount(0);
  const removedRecipientLists = await assertLists(fixture.recipient, fixture, false, false);
  const denied = await fetch(`${fixture.apiUrl}${issuePath(fixture)}`, { headers: { Authorization: `Bearer ${fixture.recipient.client.getToken()!}` } });
  expect(denied.status).toBe(404);
  const redacted = await handoffNotifications(fixture.recipient, fixture.issueId);
  expect(redacted.filter((item) => item.kind === "removed")).toHaveLength(1);
  expect(redacted.every((item) => item.can_open === false && item.issue_title === null && item.source_id === null)).toBe(true);
  await fresh.navigate("/account?view=participating");
  await expect(fresh.page.getByRole("region", { name: "Collaboration issues", exact: true }).getByText("0 issues", { exact: true })).toBeVisible();
  const deniedPopup = await popupFor(fresh.page);
  await expect(deniedPopup.getByText("You were removed from this collaboration issue.", { exact: true })).toBeVisible();
  await expect(deniedPopup.getByRole("link", { name: fixture.title, exact: true })).toHaveCount(0);
  await fresh.page.getByRole("button", { name: /^Notifications/ }).click();
  await fresh.navigate(new URL(archived.old_href, fixture.apiUrl).pathname + new URL(archived.old_href, fixture.apiUrl).search);
  await expect(fresh.page.getByText("You no longer have access to this issue.", { exact: true })).toBeVisible();
  expect(await fixture.recipient.client.getWorkspaces()).toEqual([]);
  const final = await issueFor(fixture);
  expect(final.handler.user_id).toBe(fixture.creator.id);
  assertExecutionAndBusiness(original, final);
  await fresh.close();
  await source.close();
  return { issue_id: fixture.issueId, creator_id: fixture.creator.id, recipient_id: fixture.recipient.id,
    source_workspace_id: fixture.workspace.id, invite, assignment: assigned, transfer_back: transferBack, archived,
    recipient_login_calls: fixture.recipient.logins, clients_closed_before_send: true, fresh_login_after_send_and_archive: true,
    workbench_snapshots: { after_invite: { creator: invitedSourceLists, recipient: invitedRecipientLists },
      after_assignment: { creator: assignedSourceLists, recipient: assignedRecipientLists },
      after_transfer_back: transferredRecipientLists, after_archive_and_fresh_login: reopenedRecipientLists,
      after_removal: removedRecipientLists },
    notification_snapshot_before_login: noticeBeforeLogin, no_acceptance_step: true,
    source_memberships: [], retained_comment_id: comment.comment_id, retained_comment_href: commentHref,
    removal, removed_notifications: redacted, final_issue: final,
    covered_notifications: ["participant_added", "transferred", "comment_created", "removed"],
    remaining_AC83_kinds: ["information_requested", "information_returned", "acceptance_submitted", "ai_completed", "ai_failed", "ai_stopped"] };
}
