import { randomUUID } from "node:crypto";
import { expect, type Locator, type Page } from "@playwright/test";
import { authenticate, call, callJSON, join, openAs, sql, test, type Actor, type InboxItem, type Issue } from "./fixtures/core-journeys";
import { boardCard, cardColumns, dragBoard, statusColumn } from "./fixtures/collaboration-source-surface";

// Daily issue workflows that had no E2E: list-view bulk status/delete and a
// board drag between status columns (G-D); assigning a teammate through the UI,
// their live unread badge and the inbox bulk actions (G-E). Every write is the
// product's own request; assertions read the visible result and the server.
test.use({ locale: "en-US", viewport: { width: 1440, height: 1000 }, trace: "retain-on-failure", screenshot: "only-on-failure" });

const pathOf = (url: string) => new URL(url).pathname;

async function openIssues(page: Page, actor: Actor, slug = actor.workspace.slug) {
  await page.goto(`/${slug}/issues`, { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("button", { name: "Board", exact: true })).toBeVisible({ timeout: 30_000 });
}

async function switchView(page: Page, from: "Board" | "List", to: "Board" | "List") {
  await page.getByRole("button", { name: from, exact: true }).click();
  await page.getByRole("menuitemradio", { name: to, exact: true }).click();
  await expect(page.getByRole("button", { name: to, exact: true })).toBeVisible();
}

const issueStatus = async (owner: Actor, id: string) => (await call(owner, `/api/issues/${id}`)).status === 404
  ? "deleted" : (await callJSON<Issue>(owner, `/api/issues/${id}`)).status;

test.describe("G-D bulk edits and board moves", () => {
  test("G-D1 list-view bulk status change and bulk delete write once each and survive reload", async ({ page, owner }) => {
    const tag = randomUUID().slice(0, 6);
    const issues: Issue[] = [];
    for (const n of [1, 2, 3, 4]) issues.push(await owner.api.createIssue(`Bulk ${tag} ${n}`, { status: "todo" }) as Issue);
    const [a, b, c, control] = issues as [Issue, Issue, Issue, Issue];
    await authenticate(page, owner);
    await openIssues(page, owner);
    await switchView(page, "Board", "List");
    for (const issue of issues) await expect(page.getByText(issue.title, { exact: true })).toBeVisible();

    // Row checkboxes are revealed on hover/focus; they are real checkboxes.
    const select = (issue: Issue) => page.getByRole("checkbox", { name: `Select ${issue.identifier}`, exact: true });
    const toolbar = page.locator("div.shadow-lg").filter({ has: page.getByRole("button", { name: "Clear selection", exact: true }) });
    for (const issue of [a, b, c]) await select(issue).check();
    await expect(toolbar.getByText("3 selected", { exact: true })).toBeVisible();
    await expect(select(control)).not.toBeChecked();

    const updated = page.waitForResponse((r) => pathOf(r.url()) === "/api/issues/batch-update" && r.request().method() === "POST");
    await toolbar.getByRole("button", { name: "Status", exact: true }).click();
    await page.locator("button[data-picker-item]").filter({ hasText: /^In Progress$/ }).click();
    const update = await updated;
    expect(update.status()).toBe(200);
    expect(update.request().postDataJSON()).toEqual({ issue_ids: expect.arrayContaining([a.id, b.id, c.id]), updates: { status: "in_progress" } });
    expect((update.request().postDataJSON() as { issue_ids: string[] }).issue_ids).toHaveLength(3);
    expect(await update.json()).toEqual({ updated: 3 });
    await expect(page.getByText("Updated 3 issues", { exact: true })).toBeVisible();
    for (const issue of [a, b, c]) expect(await issueStatus(owner, issue.id)).toBe("in_progress");
    expect(await issueStatus(owner, control.id)).toBe("todo");

    // After reload the three rows are grouped under In Progress: selecting that
    // whole group selects exactly them, and the control stays in Todo.
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByRole("button", { name: "List", exact: true })).toBeVisible({ timeout: 30_000 });
    await page.getByRole("checkbox", { name: "Select all loaded issues in In Progress", exact: true }).check();
    await expect(toolbar.getByText("3 selected", { exact: true })).toBeVisible();
    for (const issue of [a, b, c]) await expect(select(issue)).toBeChecked();
    await expect(select(control)).not.toBeChecked();
    await toolbar.getByRole("button", { name: "Clear selection", exact: true }).click();
    await expect(toolbar).toHaveCount(0);

    // Bulk delete is permanent: cancelling the dialog must send nothing.
    let deletePosts = 0;
    page.on("request", (request) => {
      if (request.method() === "POST" && pathOf(request.url()) === "/api/issues/batch-delete") deletePosts += 1;
    });
    await select(a).check();
    await select(b).check();
    await expect(toolbar.getByText("2 selected", { exact: true })).toBeVisible();
    await toolbar.getByRole("button", { name: "Delete", exact: true }).click();
    const dialog = page.getByRole("alertdialog", { name: "Delete 2 issues?", exact: true });
    await expect(dialog).toContainText("This action cannot be undone.");
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(toolbar.getByText("2 selected", { exact: true })).toBeVisible();
    expect(deletePosts).toBe(0);
    for (const issue of issues) expect(await issueStatus(owner, issue.id)).not.toBe("deleted");

    const deleted = page.waitForResponse((r) => pathOf(r.url()) === "/api/issues/batch-delete" && r.request().method() === "POST");
    await toolbar.getByRole("button", { name: "Delete", exact: true }).click();
    await page.getByRole("alertdialog", { name: "Delete 2 issues?", exact: true }).getByRole("button", { name: "Delete", exact: true }).click();
    const removal = await deleted;
    expect(removal.status()).toBe(200);
    expect((removal.request().postDataJSON() as { issue_ids: string[] }).issue_ids.sort()).toEqual([a.id, b.id].sort());
    expect(await removal.json()).toEqual({ deleted: 2 });
    expect(deletePosts).toBe(1);
    await expect(page.getByText("Deleted 2 issues", { exact: true })).toBeVisible();
    await expect(toolbar).toHaveCount(0);
    expect(await issueStatus(owner, a.id)).toBe("deleted");
    expect(await issueStatus(owner, b.id)).toBe("deleted");
    expect(await issueStatus(owner, c.id)).toBe("in_progress");
    expect(await issueStatus(owner, control.id)).toBe("todo");

    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.getByText(c.title, { exact: true })).toBeVisible({ timeout: 30_000 });
    await expect(page.getByText(control.title, { exact: true })).toBeVisible();
    await expect(page.getByText(a.title, { exact: true })).toHaveCount(0);
    await expect(page.getByText(b.title, { exact: true })).toHaveCount(0);
  });

  test("G-D2 dragging a board card into another status column moves it once and it stays there after reload", async ({ page, owner }) => {
    const issue = await owner.api.createIssue(`Board drag ${randomUUID().slice(0, 6)}`, { status: "todo" }) as Issue;
    await authenticate(page, owner);
    await openIssues(page, owner);
    await expect(boardCard(page, issue.title)).toBeVisible();
    expect(await cardColumns(page, issue.title)).toEqual(["board:status:todo"]);

    let moves = 0;
    page.on("request", (request) => {
      if (request.method() !== "GET" && pathOf(request.url()).startsWith(`/api/issues/${issue.id}`)) moves += 1;
    });
    const moved = page.waitForResponse((r) => pathOf(r.url()) === `/api/issues/${issue.id}/move` && r.request().method() === "POST");
    await dragBoard(page, boardCard(page, issue.title), statusColumn(page, "In Progress"), async () => {
      // dnd-kit moves the card into the hovered column before the drop.
      expect(await cardColumns(page, issue.title)).toEqual(["board:status:in_progress"]);
      const target = await statusColumn(page, "In Progress").boundingBox();
      const card = await boardCard(page, issue.title).boundingBox();
      if (!target || !card) throw new Error("The drag needs the moved card and its new column");
      // Release in the empty space below the moved card.
      await page.mouse.move(target.x + target.width / 2, target.y + Math.min(target.height - 20, card.height + 50), { steps: 8 });
      await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    });
    const move = await moved;
    expect(move.status()).toBe(200);
    expect(move.request().postDataJSON()).toMatchObject({ status: "in_progress", before_id: null, after_id: null });
    expect(await move.json()).toMatchObject({ id: issue.id, status: "in_progress" });
    expect(moves).toBe(1);
    expect(await issueStatus(owner, issue.id)).toBe("in_progress");

    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(boardCard(page, issue.title)).toBeVisible({ timeout: 30_000 });
    expect(await cardColumns(page, issue.title)).toEqual(["board:status:in_progress"]);
  });
});
/** The sidebar Inbox row; its unread count renders only when it is above zero. */
const inboxNav = (page: Page) => page.locator('a[data-sidebar="menu-button"][title="Inbox"]');
const inboxBadge = (page: Page): Locator => inboxNav(page).locator("number-flow-react");

test("G-E1 assigning a teammate notifies them live; inbox bulk read/archive clear the badge until they are reassigned away", async ({ browser, owner, member }) => {
  test.setTimeout(150_000);
  await join(owner, member);
  const tag = randomUUID().slice(0, 6);
  const first = await owner.api.createIssue(`Hand-off ${tag} A`, { status: "todo" }) as Issue;
  const second = await owner.api.createIssue(`Hand-off ${tag} B`, { status: "todo" }) as Issue;
  const ownerSide = await openAs(browser, owner);
  const memberSide = await openAs(browser, member);
  const memberInbox = () => callJSON<InboxItem[]>(member, "/api/inbox", "GET", undefined, owner.workspace.id);
  const unread = async () => (await callJSON<{ count: number }>(member, "/api/inbox/unread-count", "GET", undefined, owner.workspace.id)).count;
  try {
    // The member's browser sits on the owner's workspace with an authenticated socket.
    let memberSocket = false;
    memberSide.page.on("websocket", (socket) => {
      if (new URL(socket.url()).searchParams.get("workspace_slug") !== owner.workspace.slug) return;
      socket.on("framereceived", ({ payload }) => {
        try { if (JSON.parse(String(payload)).type === "auth_ack") memberSocket = true; } catch { /* heartbeat */ }
      });
    });
    await openIssues(memberSide.page, member, owner.workspace.slug);
    await expect(memberSide.page.getByText(first.title, { exact: true }).first()).toBeVisible();
    await expect.poll(() => memberSocket, { message: "the member's workspace socket must authenticate first" }).toBe(true);
    await expect(inboxNav(memberSide.page)).toBeVisible();
    await expect(inboxBadge(memberSide.page)).toHaveCount(0);
    expect(await memberInbox()).toEqual([]);

    // Member assignment applies directly; only agent/squad assignment asks for
    // confirmation (packages/views/issues/actions/run-confirm-gate.ts).
    const assign = async (issue: Issue) => {
      await ownerSide.page.goto(`/${owner.workspace.slug}/issues/${issue.id}`, { waitUntil: "domcontentloaded" });
      await expect(ownerSide.page.getByRole("button", { name: issue.title, exact: true })).toBeVisible({ timeout: 30_000 });
      const written = ownerSide.page.waitForResponse((r) => pathOf(r.url()) === `/api/issues/${issue.id}` && r.request().method() === "PUT");
      await ownerSide.page.getByRole("button", { name: "Unassigned", exact: true }).first().click();
      await ownerSide.page.locator("button[data-picker-item]").filter({ hasText: member.name }).first().click();
      const response = await written;
      expect(response.status()).toBe(200);
      expect(response.request().postDataJSON()).toMatchObject({ assignee_type: "member", assignee_id: member.userId });
      await expect(ownerSide.page.getByRole("dialog", { name: "Confirm assignment?", exact: true })).toHaveCount(0);
      expect(await callJSON<{ assignee_type: string; assignee_id: string }>(owner, `/api/issues/${issue.id}`))
        .toMatchObject({ assignee_type: "member", assignee_id: member.userId });
    };

    await assign(first);
    await expect.poll(async () => (await memberInbox()).map((item) => [item.type, item.issue_id, item.read]))
      .toEqual([["issue_assigned", first.id, false]]);
    expect(await unread()).toBe(1);
    // Without a reload, the inbox:new event lights the member's badge.
    await expect(inboxBadge(memberSide.page)).toHaveAttribute("aria-label", "1");

    await assign(second);
    await expect.poll(async () => (await memberInbox()).filter((item) => item.type === "issue_assigned").map((item) => item.issue_id).sort())
      .toEqual([first.id, second.id].sort());
    expect(await unread()).toBe(2);
    await expect(inboxBadge(memberSide.page)).toHaveAttribute("aria-label", "2");
    // The actor is never notified of their own assignment.
    expect((await callJSON<InboxItem[]>(owner, "/api/inbox")).filter((item) => item.type === "issue_assigned")).toEqual([]);

    await memberSide.page.goto(`/${owner.workspace.slug}/my-issues`, { waitUntil: "domcontentloaded" });
    await expect(memberSide.page.getByText(first.title, { exact: true }).first()).toBeVisible({ timeout: 30_000 });
    await expect(memberSide.page.getByText(second.title, { exact: true }).first()).toBeVisible();

    // The inbox lists one row per issue; bulk actions live in the header menu.
    const inbox = memberSide.page;
    await inbox.goto(`/${owner.workspace.slug}/inbox`, { waitUntil: "domcontentloaded" });
    const row = (issue: Issue) => inbox.getByRole("listitem").filter({ has: inbox.getByRole("button", { name: issue.title, exact: true }) });
    await expect(row(first)).toHaveCount(1, { timeout: 30_000 });
    await expect(row(second)).toHaveCount(1);
    // Row copy comes from inbox-detail-label.tsx ("Assigned to {name}" / "Removed assignee").
    await expect(row(first)).toContainText(`Assigned to ${member.name}`);
    const bulk = async (label: string, path: string) => {
      const done = inbox.waitForResponse((r) => pathOf(r.url()) === path && r.request().method() === "POST");
      await inbox.getByRole("button", { name: "Inbox bulk actions", exact: true }).click();
      await inbox.getByRole("menuitem", { name: label, exact: true }).click();
      const response = await done;
      expect(response.status()).toBe(200);
      return response.json() as Promise<{ count: number }>;
    };

    expect(await bulk("Mark all as read", "/api/inbox/mark-all-read")).toEqual({ count: 2 });
    await expect(inboxBadge(inbox)).toHaveCount(0);
    expect(await unread()).toBe(0);
    expect((await memberInbox()).map((item) => item.read)).toEqual([true, true]);
    await inbox.reload({ waitUntil: "domcontentloaded" });
    await expect(row(first)).toHaveCount(1, { timeout: 30_000 });
    await expect(inboxBadge(inbox)).toHaveCount(0);

    expect(await bulk("Archive all", "/api/inbox/archive-all")).toEqual({ count: 2 });
    await expect(inbox.getByText("No notifications", { exact: true })).toBeVisible();
    expect(await memberInbox()).toEqual([]);
    const archived = () => callJSON<InboxItem[]>(member, "/api/inbox/archived", "GET", undefined, owner.workspace.id);
    expect((await archived()).map((item) => item.issue_id).sort()).toEqual([first.id, second.id].sort());
    await inbox.getByRole("button", { name: /^Archived/ }).click();
    await expect(row(first)).toHaveCount(1);
    await expect(row(second)).toHaveCount(1);

    // The owner takes A back: the previous member assignee gets "unassigned"
    // (notification_listeners.go:728-736), which returns A to the active inbox
    // and lights the badge again. Clearing to nobody is G-E2 below.
    await ownerSide.page.goto(`/${owner.workspace.slug}/issues/${first.id}`, { waitUntil: "domcontentloaded" });
    await expect(ownerSide.page.getByRole("button", { name: first.title, exact: true })).toBeVisible({ timeout: 30_000 });
    const reassigned = ownerSide.page.waitForResponse((r) => pathOf(r.url()) === `/api/issues/${first.id}` && r.request().method() === "PUT");
    await ownerSide.page.getByRole("button", { name: member.name }).first().click();
    await ownerSide.page.locator("button[data-picker-item]").filter({ hasText: owner.name }).first().click();
    const takeBack = await reassigned;
    expect(takeBack.status()).toBe(200);
    expect(takeBack.request().postDataJSON()).toMatchObject({ assignee_type: "member", assignee_id: owner.userId });
    await expect.poll(async () => (await memberInbox()).map((item) => [item.type, item.issue_id, item.read]))
      .toEqual([["unassigned", first.id, false]]);
    expect(await unread()).toBe(1);
    await expect(inboxBadge(inbox)).toHaveAttribute("aria-label", "1");
    // Active and archived lists stay mutually exclusive per issue.
    expect((await archived()).map((item) => item.issue_id)).toEqual([second.id]);
    await inbox.goto(`/${owner.workspace.slug}/inbox`, { waitUntil: "domcontentloaded" });
    await expect(row(first)).toHaveCount(1, { timeout: 30_000 });
    await expect(row(first)).toContainText("Removed assignee");
    await expect(row(second)).toHaveCount(0);

    await inbox.goto(`/${owner.workspace.slug}/my-issues`, { waitUntil: "domcontentloaded" });
    await expect(inbox.getByText(second.title, { exact: true }).first()).toBeVisible({ timeout: 30_000 });
    await expect(inbox.getByText(first.title, { exact: true })).toHaveCount(0);
  } finally {
    await ownerSide.context.close();
    await memberSide.context.close();
  }
});

// The picker's "Unassigned" option sends explicit nulls. Regression: the server
// used to read JSON null as "field absent" and publish assignee_changed=false,
// so the previous assignee got no "unassigned" notice and no activity was
// written. The flag is pinned in issue_assignee_clear_event_test.go.
test("G-E2 clearing a teammate's assignment to nobody notifies them and records the change", async ({ browser, owner, member }) => {
  await join(owner, member);
  const issue = await owner.api.createIssue(`Cleared hand-off ${randomUUID().slice(0, 6)}`, {
    status: "todo", assignee_type: "member", assignee_id: member.userId,
  }) as Issue;
  const memberInbox = () => callJSON<InboxItem[]>(member, "/api/inbox", "GET", undefined, owner.workspace.id);
  await expect.poll(async () => (await memberInbox()).map((item) => item.type)).toEqual(["issue_assigned"]);
  expect(await callJSON(member, "/api/inbox/mark-all-read", "POST", undefined, owner.workspace.id)).toEqual({ count: 1 });
  const ownerSide = await openAs(browser, owner);
  const memberSide = await openAs(browser, member);
  try {
    await openIssues(memberSide.page, member, owner.workspace.slug);
    await expect(inboxBadge(memberSide.page)).toHaveCount(0);
    await ownerSide.page.goto(`/${owner.workspace.slug}/issues/${issue.id}`, { waitUntil: "domcontentloaded" });
    await expect(ownerSide.page.getByRole("button", { name: issue.title, exact: true })).toBeVisible({ timeout: 30_000 });
    const cleared = ownerSide.page.waitForResponse((r) => pathOf(r.url()) === `/api/issues/${issue.id}` && r.request().method() === "PUT");
    await ownerSide.page.getByRole("button", { name: member.name }).first().click();
    await ownerSide.page.locator("button[data-picker-item]").filter({ hasText: /^Unassigned$/ }).click();
    const response = await cleared;
    expect(response.status()).toBe(200);
    expect(response.request().postDataJSON()).toMatchObject({ assignee_type: null, assignee_id: null });
    expect(await callJSON(owner, `/api/issues/${issue.id}`)).toMatchObject({ assignee_type: null, assignee_id: null });

    // Intended: one unread "unassigned" row for the previous assignee and an
    // assignee_changed activity naming them as the previous assignee.
    await expect.poll(async () => ({
      inbox: (await memberInbox()).map((item) => [item.type, item.read]),
      activity: (await sql<{ action: string; from_id: string | null }>(
        "SELECT action, details->>'from_id' AS from_id FROM activity_log WHERE issue_id = $1 AND action = 'assignee_changed'", [issue.id])),
    })).toEqual({ inbox: [["unassigned", false], ["issue_assigned", true]], activity: [{ action: "assignee_changed", from_id: member.userId }] });
    await expect(inboxBadge(memberSide.page)).toHaveAttribute("aria-label", "1");
  } finally {
    await ownerSide.context.close();
    await memberSide.context.close();
  }
});
