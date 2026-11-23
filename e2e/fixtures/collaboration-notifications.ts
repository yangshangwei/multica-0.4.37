import { randomUUID } from "node:crypto";
import pg from "pg";
import { expect, type Page } from "@playwright/test";
import { CollaborationCommentReceiptSchema, CollaborationNotificationsSchema } from "../../packages/core/collaboration/content-schema";
import { CollaborationIssueSchema } from "../../packages/core/collaboration/schema";
import type { CollaborationNotificationView } from "../../packages/core/collaboration/notification-view";
import { seedCollaborationRecordJourney } from "./collaboration-records";
import { openSidebarNavigation } from "./sidebar-navigation";

export const notificationList = (page: Page) => page.locator('[data-tab-scroll-root="account-notifications-list"]');
export const notificationRow = (page: Page, id: string) => notificationList(page).locator(`[data-notification-id="${id}"]`);
export const notificationViews = (page: Page) => page.getByRole("navigation", { name: "Notification views", exact: true });

/** Enter through the actual sidebar, including its initially collapsed group. */
export async function openCollaborationNotifications(page: Page, view: CollaborationNotificationView = "all") {
  const navigation = await openSidebarNavigation(page);
  const directory = navigation.locator('[data-sidebar-scope="collaboration"]');
  const link = directory.getByRole("link", { name: "Collaboration notifications", exact: true });
  if (!await link.isVisible()) await directory.getByRole("button", { name: "Expand collaboration navigation", exact: true }).click();
  await link.click();
  await expect(page.getByRole("heading", { name: "Collaboration notifications", exact: true })).toBeVisible();
  await expect(page.locator('[data-slot="popover-content"]').filter({ has: page.getByText("Notifications", { exact: true }) })).toHaveCount(0);
  if (view !== "unread") await notificationViews(page).getByRole("link", { name: view === "all" ? "All" : "Archived", exact: true }).click();
  await expect(notificationViews(page).getByRole("link", { name: view === "all" ? "All" : view === "archived" ? "Archived" : "Unread", exact: true })).toHaveAttribute("aria-current", "page");
  await expect(notificationList(page)).toBeVisible();
  return notificationList(page);
}

export function notificationPath(view: CollaborationNotificationView, notificationId?: string, issueId?: string, hash = "") {
  const query = new URLSearchParams({ view: "notifications", filter: view });
  if (notificationId) query.set("notification", notificationId);
  if (issueId) query.set("issue", issueId);
  return `/account?${query}${hash}`;
}

interface NotificationFact {
  id: string;
  issue_id: string;
  kind: string;
  source_kind: string;
  source_id: string | null;
  run_id: string | null;
  created_at: Date;
  read_at: Date | null;
  archived_at: Date | null;
}

/** Verify the recorded API database before TestApiClient can perform any write. */
async function notificationDatabase() {
  const expected = process.env.TASK_DATABASE_NAME;
  const configured = process.env.DATABASE_URL;
  if (!expected || !configured || ["multica", "postgres", "template0", "template1"].includes(expected)) {
    throw new Error("Notification acceptance requires its recorded task API database");
  }
  const url = new URL(configured);
  if (!["postgres:", "postgresql:"].includes(url.protocol) || !["localhost", "127.0.0.1"].includes(url.hostname)
    || url.hash !== "" || ["host", "hostaddr", "port", "dbname", "service"].some((key) => url.searchParams.has(key))
    || decodeURIComponent(url.pathname.slice(1)) !== expected) throw new Error("Notification fixture database differs from its allocation");
  const apiUrl = process.env.COLLABORATION_E2E_API_URL;
  if (!apiUrl || new URL(apiUrl).origin !== new URL(process.env.NEXT_PUBLIC_API_URL ?? "").origin) {
    throw new Error("Notification fixture API differs from TestApiClient's configured origin");
  }
  const db = new pg.Client({ connectionString: configured, connectionTimeoutMillis: 5000 });
  await db.connect();
  try {
    expect((await db.query<{ database: string }>("SELECT current_database() AS database")).rows).toEqual([{ database: expected }]);
  } catch (error) {
    await db.end();
    throw error;
  }
  return db;
}

/** Public human setup plus synthetic retained notification/run history.
 * These rows exercise paging and navigation, without an agent process or provider.
 * The task-owned database retains this history for acceptance inspection.
 */
export async function seedCollaborationNotifications() {
  const db = await notificationDatabase();
  try {
    const records = await seedCollaborationRecordJourney();
    expect(await records.viewer.getWorkspaces()).toEqual([]);
    const key = randomUUID().slice(0, 8);
    const workspace = await records.viewer.ensureWorkspace("Notification return workspace", `notification-return-${key}`);
    const ordinary = await records.viewer.createIssue(`Return to workspace ${key}`);
    const title = `Notification review ${key}: ${"preserve exact discussion and recorded execution targets ".repeat(4)}${"long_notification_identifier_".repeat(4)}`;
    const description = Array.from({ length: 65 }, (_, index) => `Notification requirement ${index + 1}. Retain the selected notification, preserve independent list and detail scrolling, and keep the original source target available.`).join("\n\n");
    const issue = CollaborationIssueSchema.parse(await records.source.requestJSON("/api/collaboration/issues", { method: "POST",
      headers: { "Idempotency-Key": randomUUID() }, body: { source_workspace_id: records.workspace.id,
        handler_user_id: records.viewerId, title, description, ai_policy: "manual", attachment_ids: [] } }));
    const commentBody = `Exact notification comment ${key}. Opening this target must not mark it read.`;
    const comment = CollaborationCommentReceiptSchema.parse(await records.source.requestJSON(`/api/collaboration/issues/${issue.id}/comments`, {
      method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { content: commentBody, parent_id: null,
        suppressed_target_ids: [], expected_lifecycle_epoch: issue.lifecycle_epoch,
        expected_participant_epoch: issue.write_context!.participant_epoch, expected_account_epoch: issue.write_context!.account_epoch, target_versions: [] },
    }));
    const bodyId = randomUUID(), issueNoticeId = randomUUID(), unavailableId = randomUUID(), unavailableIssueId = randomUUID();
    await db.query("BEGIN");
    const session = await db.query<{ event_seq: string }>("UPDATE collaboration_session SET event_seq=event_seq+117 WHERE issue_id=$1 RETURNING event_seq::text", [issue.id]);
    const firstSequence = BigInt(session.rows[0]!.event_seq) - 116n;
    let index = 0;
    for (const [state, count] of [["unread", 55], ["read", 8], ["archived", 53]] as const) {
      for (let offset = 0; offset < count; offset++, index++) {
        const id = index === 0 ? bodyId : index === 1 ? issueNoticeId : randomUUID();
        const sourceId = index === 0 ? randomUUID() : null;
        const kind = index === 0 ? "issue_edited" : "information_requested";
        const sourceKind = index === 0 ? "body" : "issue";
        await db.query(`INSERT INTO collaboration_notification
          (id,recipient_user_id,issue_id,event_seq,kind,source_kind,source_id,created_at,read_at,archived_at)
          VALUES ($1,$2,$3,$4,$5,$6,$7,now()-interval '10 minutes'-($8::int/2)*interval '1 second',
            CASE WHEN $9='read' THEN now() ELSE NULL END, CASE WHEN $9='archived' THEN now() ELSE NULL END)`,
        [id, records.viewerId, issue.id, (firstSequence + BigInt(index)).toString(), kind, sourceKind, sourceId, index, state]);
      }
    }
    await db.query(`INSERT INTO collaboration_notification (id,recipient_user_id,issue_id,event_seq,kind,source_kind,created_at)
      VALUES ($1,$2,$3,1,'removed','issue',now()-interval '11 minutes')`, [unavailableId, records.viewerId, unavailableIssueId]);
    await db.query("COMMIT");
    const rows = (await db.query<NotificationFact>(`SELECT id,issue_id,kind,source_kind,source_id,run_id,created_at,read_at,archived_at
      FROM collaboration_notification WHERE recipient_user_id=$1 ORDER BY created_at DESC,id DESC`, [records.viewerId])).rows;
    const runNotice = rows.find((row) => row.run_id === records.runId);
    const commentNotice = rows.find((row) => row.source_id === comment.comment_id);
    if (!runNotice || !commentNotice) throw new Error("Public and retained fixture notifications did not preserve their exact targets");
    return { records, workspace, ordinary, issue, title, description, commentBody, commentId: comment.comment_id,
      runNotice, commentNotice, bodyId, issueNoticeId, unavailableId, unavailableIssueId, rows };
  } catch (error) {
    await db.query("ROLLBACK");
    throw error;
  } finally { await db.end(); }
}

export async function notificationPage(fixture: Awaited<ReturnType<typeof seedCollaborationNotifications>>, view: CollaborationNotificationView, cursor?: string) {
  const query = new URLSearchParams({ view, limit: "50" });
  if (cursor) query.set("cursor", cursor);
  return CollaborationNotificationsSchema.parse(await fixture.records.viewer.requestJSON(`/api/me/collaboration-notifications?${query}`));
}
