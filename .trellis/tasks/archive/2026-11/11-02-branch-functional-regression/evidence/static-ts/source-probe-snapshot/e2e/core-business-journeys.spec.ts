import { randomUUID } from "node:crypto";
import { expect, type Page } from "@playwright/test";
import {
  authenticate, call, callJSON, createAgent, executionLog, issueTasks, join, openAs, openIssue, SimulatedDaemon, sql, test,
  type InboxItem, type Issue,
} from "./fixtures/core-journeys";

// Core business journeys that no other spec carries end to end: the agent
// execution loop driven over the real daemon protocol (G1), cross-user realtime
// collaboration and notification (G2), member management through the UI (G3),
// and My Issues + search (G4). Assertions check the browser and the server.
type Member = { id: string; user_id: string; role: string };

test.use({ locale: "en-US", viewport: { width: 1440, height: 1000 }, trace: "retain-on-failure", screenshot: "only-on-failure" });

test.describe("G1 agent execution loop", () => {
  test("G1-01 assigning an agent queues one task that the daemon runs to a live completed reply", async ({ page, owner }) => {
    const daemon = new SimulatedDaemon(owner);
    await daemon.register();
    const agent = await createAgent(owner, daemon.runtimeId);
    const issue = await owner.api.createIssue(`Core loop ${randomUUID().slice(0, 6)}`, { status: "todo" }) as Issue;
    await authenticate(page, owner);
    await openIssue(page, owner.workspace, issue);

    // Assign through the browser picker and require exactly one queued task.
    // Assigning an agent asks for confirmation before anything is written.
    await page.getByRole("button", { name: "Unassigned", exact: true }).first().click();
    await page.locator("button[data-picker-item]").filter({ hasText: agent.name }).first().click();
    const confirmation = page.getByRole("dialog", { name: "Confirm assignment?", exact: true });
    await expect(confirmation).toBeVisible();
    expect(await issueTasks(issue.id)).toEqual([]);
    const assigned = page.waitForResponse((r) => new URL(r.url()).pathname === `/api/issues/${issue.id}` && r.request().method() === "PUT");
    await confirmation.getByRole("button", { name: "Confirm assignment", exact: true }).click();
    expect((await assigned).ok()).toBe(true);
    await expect.poll(async () => (await issueTasks(issue.id)).map((t) => t.status)).toEqual(["queued"]);
    const log = executionLog(page);
    await expect(log.getByText("Execution log", { exact: true })).toBeVisible();
    await expect(log.getByText("Queued", { exact: true })).toBeVisible();

    // Claim → start → progress/messages → complete, with no page reload.
    const claimed = await daemon.claim();
    const [queued] = await issueTasks(issue.id);
    expect(claimed?.id).toBe(queued!.id);
    await daemon.start(claimed!.id);
    await expect(log.locator(".sr-only", { hasText: "Working" })).toHaveCount(1);
    await daemon.progress(claimed!.id, "Reading the issue");
    await daemon.messages(claimed!.id, "Simulated agent transcript line");
    const reply = `Core loop finished ${randomUUID().slice(0, 8)}`;
    await daemon.complete(claimed!.id, reply);

    await expect(page.getByText(reply, { exact: true })).toBeVisible();
    await expect(log.getByText("Queued", { exact: true })).toHaveCount(0);
    await log.getByRole("button", { name: "Show past runs (1)", exact: true }).click();
    await expect(log.locator(".sr-only", { hasText: "Completed" })).toHaveCount(1);
    expect((await issueTasks(issue.id)).map((t) => t.status)).toEqual(["completed"]);
    const comments = await sql<{ content: string }>(
      "SELECT content FROM comment WHERE issue_id = $1 AND author_type = 'agent' AND author_id = $2", [issue.id, agent.id]);
    expect(comments.map((c) => c.content)).toEqual([reply]);
    expect(await daemon.claim()).toBeNull();

    await page.reload();
    await expect(page.getByText(reply, { exact: true })).toBeVisible({ timeout: 30_000 });
  });

  test("G1-02 a failed run notifies the creator, retries exactly once and can be stopped before it runs", async ({ page, owner }) => {
    const daemon = new SimulatedDaemon(owner);
    await daemon.register();
    const agent = await createAgent(owner, daemon.runtimeId);
    const issue = await owner.api.createIssue(`Core failure ${randomUUID().slice(0, 6)}`, {
      status: "todo", assignee_type: "agent", assignee_id: agent.id,
    }) as Issue;
    await expect.poll(async () => (await issueTasks(issue.id)).length).toBe(1);
    await authenticate(page, owner);
    await openIssue(page, owner.workspace, issue);
    const log = executionLog(page);

    const first = await daemon.claim();
    await daemon.start(first!.id);
    await daemon.fail(first!.id, "Simulated agent error");
    await log.getByRole("button", { name: "Show past runs (1)", exact: true }).click();
    // The past row explains the failure with the localized reason, not raw error text.
    await expect(log.locator(".sr-only", { hasText: "Agent execution error" })).toHaveCount(1);
    await expect(log.getByText("Simulated agent error")).toHaveCount(0);
    // A non-retryable failure must not schedule an automatic retry.
    expect((await issueTasks(issue.id)).map((t) => t.status)).toEqual(["failed"]);
    await expect.poll(async () => (await callJSON<InboxItem[]>(owner, "/api/inbox"))
      .filter((item) => item.issue_id === issue.id && item.type === "task_failed").length).toBe(1);

    // Row actions are revealed on hover, as for a pointer user.
    const runRow = (text: string) => log.locator("div.group\\/execution-log-row").filter({ has: page.locator(".sr-only", { hasText: text }) });
    const rerun = page.waitForResponse((r) => new URL(r.url()).pathname === `/api/issues/${issue.id}/rerun` && r.request().method() === "POST");
    await runRow("Agent execution error").hover();
    await runRow("Agent execution error").getByRole("button", { name: "Retry task", exact: true }).click();
    expect((await rerun).ok()).toBe(true);
    await expect.poll(async () => (await issueTasks(issue.id)).map((t) => t.status)).toEqual(["failed", "queued"]);
    const [failed, retry] = await issueTasks(issue.id);
    // A manual retry is a new run that records which run it repeats.
    expect(retry!.rerun_of_task_id).toBe(failed!.id);
    await expect(log.getByText("Queued", { exact: true })).toBeVisible();

    const queuedRow = log.locator("div.group\\/execution-log-row").filter({ hasText: "Queued" });
    await queuedRow.hover();
    await queuedRow.getByRole("button", { name: "Cancel task", exact: true }).click();
    const confirm = page.getByRole("alertdialog", { name: "Stop this task?", exact: true });
    await confirm.getByRole("button", { name: "Stop task", exact: true }).click();
    await expect.poll(async () => (await issueTasks(issue.id)).map((t) => t.status)).toEqual(["failed", "cancelled"]);
    await expect(log.getByText("Queued", { exact: true })).toHaveCount(0);
    expect(await daemon.claim()).toBeNull();
  });
});
test("G2 a teammate's status change and mention reach the owner's open page and inbox without reload", async ({ browser, owner, member }) => {
  await join(owner, member);
  const issue = await owner.api.createIssue(`Core realtime ${randomUUID().slice(0, 6)}`, { status: "todo" }) as Issue;
  const ownerSide = await openAs(browser, owner);
  const memberSide = await openAs(browser, member);
  try {
    await openIssue(ownerSide.page, owner.workspace, issue);
    await openIssue(memberSide.page, owner.workspace, issue);
    await expect(ownerSide.page.getByRole("button", { name: "Todo", exact: true })).toBeVisible();

    // The member changes status through their own browser.
    await memberSide.page.getByRole("button", { name: "Todo", exact: true }).click();
    await memberSide.page.locator("button[data-picker-item]").filter({ hasText: /^In Progress$/ }).click();
    await expect(ownerSide.page.getByRole("button", { name: "In Progress", exact: true })).toBeVisible();

    const body = `Please review ${randomUUID().slice(0, 6)}`;
    await callJSON(member, `/api/issues/${issue.id}/comments`, "POST",
      { content: `[@${owner.name}](mention://member/${owner.userId}) ${body}` }, owner.workspace.id);
    await expect(ownerSide.page.getByText(body)).toBeVisible();

    // Server fan-out: the owner, not the actor, is notified of both events.
    await expect.poll(async () => (await callJSON<InboxItem[]>(owner, "/api/inbox"))
      .filter((item) => item.issue_id === issue.id).map((item) => item.type).sort())
      .toEqual(expect.arrayContaining(["mentioned", "status_changed"]));
    expect((await callJSON<InboxItem[]>(member, "/api/inbox", "GET", undefined, owner.workspace.id))
      .filter((item) => item.issue_id === issue.id)).toEqual([]);

    // The owner's inbox lists the items; selecting the mention marks it read.
    await ownerSide.page.goto(`/${owner.workspace.slug}/inbox`, { waitUntil: "domcontentloaded" });
    const mention = (await callJSON<InboxItem[]>(owner, "/api/inbox"))
      .find((item) => item.issue_id === issue.id && item.type === "mentioned")!;
    expect(mention.read).toBe(false);
    // The list keeps one row per issue: its newest notification, the mention.
    const rows = ownerSide.page.getByRole("listitem").filter({
      has: ownerSide.page.getByRole("button", { name: issue.title, exact: true }),
    });
    await expect(rows).toHaveCount(1, { timeout: 30_000 });
    await expect(rows).toContainText("Mentioned");
    const marked = ownerSide.page.waitForResponse((r) => new URL(r.url()).pathname === `/api/inbox/${mention.id}/read`);
    await rows.getByRole("button", { name: issue.title, exact: true }).click();
    expect((await marked).ok()).toBe(true);
    await ownerSide.page.reload();
    await expect.poll(async () => (await callJSON<InboxItem[]>(owner, "/api/inbox"))
      .find((item) => item.id === mention.id)?.read).toBe(true);
  } finally {
    await ownerSide.context.close();
    await memberSide.context.close();
  }
});

test("G3 members invite, role change and removal work through the settings UI with matching server permissions", async ({ browser, owner, member }) => {
  const ownerSide = await openAs(browser, owner);
  const memberSide = await openAs(browser, member);
  const membersTab = (page: Page) => page.goto(`/${owner.workspace.slug}/settings?tab=members`, { waitUntil: "domcontentloaded" });
  const roleOf = async () => (await callJSON<Member[]>(owner, `/api/workspaces/${owner.workspace.id}/members`))
    .find((m) => m.user_id === member.userId)?.role;
  try {
    await membersTab(ownerSide.page);
    await ownerSide.page.getByRole("textbox", { name: "user@company.com", exact: true }).fill(member.email);
    await ownerSide.page.getByRole("button", { name: "Invite", exact: true }).click();
    await expect(ownerSide.page.getByText(member.email).first()).toBeVisible();
    const pending = await callJSON<Array<{ id: string; email?: string; status: string; role: string }>>(
      owner, `/api/workspaces/${owner.workspace.id}/invitations`);
    const invitation = pending.find((item) => item.status === "pending")!;
    expect(invitation).toMatchObject({ role: "member" });

    await memberSide.page.goto(`/invite/${invitation.id}`, { waitUntil: "domcontentloaded" });
    await memberSide.page.getByRole("button", { name: "Accept & Join", exact: true }).click();
    await expect.poll(roleOf).toBe("member");

    // A plain member cannot manage members: no invite form, and the API agrees.
    await membersTab(memberSide.page);
    await expect(memberSide.page.getByText(owner.name).first()).toBeVisible({ timeout: 30_000 });
    await expect(memberSide.page.getByRole("heading", { name: "Invite member", exact: true })).toHaveCount(0);
    expect((await call(member, `/api/workspaces/${owner.workspace.id}/members`, "POST",
      { email: `blocked-${randomUUID().slice(0, 6)}@multica.ai`, role: "member" }, owner.workspace.id)).status).toBe(403);

    const row = () => ownerSide.page.locator("div.flex.items-center.gap-3").filter({ hasText: member.name });
    await ownerSide.page.reload();
    await row().getByRole("button").click();
    await ownerSide.page.getByRole("menuitem", { name: /Change role/ }).hover();
    await ownerSide.page.getByRole("menuitem", { name: /^Admin/ }).click();
    await expect(ownerSide.page.getByText("Role updated", { exact: true })).toBeVisible();
    await expect.poll(roleOf).toBe("admin");
    await memberSide.page.reload();
    await expect(memberSide.page.getByRole("heading", { name: "Invite member", exact: true })).toBeVisible({ timeout: 30_000 });

    await row().getByRole("button").click();
    await ownerSide.page.getByRole("menuitem", { name: /Remove from workspace/ }).click();
    const confirm = ownerSide.page.getByRole("alertdialog").or(ownerSide.page.getByRole("dialog"))
      .filter({ hasText: `Remove ${member.name}` });
    await confirm.getByRole("button", { name: "Confirm", exact: true }).click();
    await expect(ownerSide.page.getByText("Member removed", { exact: true })).toBeVisible();
    await expect.poll(roleOf).toBeUndefined();
    expect([403, 404]).toContain((await call(member, "/api/issues", "GET", undefined, owner.workspace.id)).status);
  } finally {
    await ownerSide.context.close();
    await memberSide.context.close();
  }
});

test("G4 My Issues lists only my assignments and search opens a teammate's issue by identifier", async ({ page, owner, member }) => {
  await join(owner, member);
  const tag = randomUUID().slice(0, 6);
  const mine = await owner.api.createIssue(`Core mine ${tag}`, { status: "todo", assignee_type: "member", assignee_id: owner.userId }) as Issue;
  const theirs = await owner.api.createIssue(`Core theirs ${tag}`, { status: "todo", assignee_type: "member", assignee_id: member.userId }) as Issue;
  await authenticate(page, owner);

  await page.goto(`/${owner.workspace.slug}/my-issues`, { waitUntil: "domcontentloaded" });
  await expect(page.getByText(mine.title, { exact: true }).first()).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText(theirs.title, { exact: true })).toHaveCount(0);

  await page.getByRole("button", { name: /Search\.\.\./ }).first().click();
  const palette = page.getByRole("dialog", { name: "Search", exact: true });
  await palette.getByPlaceholder("Type a command or search...").fill(theirs.identifier);
  const hit = palette.getByRole("option").filter({ hasText: theirs.title });
  await expect(hit).toBeVisible();
  await hit.click();
  await expect(page).toHaveURL(new RegExp(`/${owner.workspace.slug}/issues/${theirs.identifier}$`));
  await expect(page.getByRole("button", { name: theirs.title, exact: true })).toBeVisible();
});
