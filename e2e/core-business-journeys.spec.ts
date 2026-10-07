import { randomUUID } from "node:crypto";
import pg from "pg";
import { expect, test as base, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

// Core business journeys that no other spec carries end to end: the agent
// execution loop driven over the real daemon protocol (G1), cross-user realtime
// collaboration and notification (G2), member management through the UI (G3),
// and My Issues + search (G4). Assertions check the browser and the server.
type Workspace = { id: string; slug: string; name: string };
type Actor = { api: TestApiClient; workspace: Workspace; userId: string; name: string; email: string };
type Issue = { id: string; identifier: string; title: string; status: string };
type TaskRow = { id: string; status: string; rerun_of_task_id: string | null };
type Member = { id: string; user_id: string; role: string };
type InboxItem = { id: string; type: string; read: boolean; issue_id: string | null; title: string };

const API_BASE = process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`;
const DATABASE_URL = process.env.DATABASE_URL;

async function createActor(label: string): Promise<Actor> {
  const api = new TestApiClient();
  const slug = `core-${label}-${randomUUID().slice(0, 10)}`;
  const name = `Core ${label} ${slug.slice(-4)}`;
  await api.login(`${slug}@multica.ai`, name);
  const workspace = await api.ensureWorkspace(`Core ${label}`, slug);
  expect(workspace.slug, "fixture must own its workspace").toBe(slug);
  await api.markUserOnboarded();
  const user = await api.requestJSON<{ id: string }>("/api/me", { method: "PATCH", body: { language: "en" } });
  return { api, workspace, userId: user.id, name, email: api.getEmail() };
}

const test = base.extend<{ owner: Actor; member: Actor }>({
  owner: async ({}, use) => {
    const actor = await createActor("owner");
    try { await use(actor); } finally { await actor.api.deleteFeatureWorkspace(actor.workspace.id); }
  },
  member: async ({}, use) => {
    const actor = await createActor("member");
    try { await use(actor); } finally { await actor.api.deleteFeatureWorkspace(actor.workspace.id); }
  },
});

test.use({ locale: "en-US", viewport: { width: 1440, height: 1000 }, trace: "retain-on-failure", screenshot: "only-on-failure" });

async function authenticate(page: Page, actor: Actor) {
  const token = actor.api.getToken();
  if (!token) throw new Error("Missing fixture session");
  await page.addInitScript((value) => {
    localStorage.setItem("multica_token", value);
    localStorage.setItem("multica-locale", "en");
    localStorage.setItem("multica:chat:isOpen", "false");
    document.cookie = "multica_logged_in=1; path=/; SameSite=Lax";
    document.cookie = "multica-locale=en; path=/; SameSite=Lax";
  }, token);
}

async function openAs(browser: Browser, actor: Actor): Promise<{ context: BrowserContext; page: Page }> {
  const context = await browser.newContext({ locale: "en-US", viewport: { width: 1440, height: 1000 } });
  const page = await context.newPage();
  await authenticate(page, actor);
  return { context, page };
}

function call(actor: Actor, path: string, method = "GET", body?: unknown, workspaceId = actor.workspace.id) {
  return fetch(`${API_BASE}${path}`, {
    method,
    headers: {
      Authorization: `Bearer ${actor.api.getToken()}`,
      "Content-Type": "application/json",
      "X-Workspace-ID": workspaceId,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

async function callJSON<T>(actor: Actor, path: string, method = "GET", body?: unknown, workspaceId = actor.workspace.id): Promise<T> {
  const response = await call(actor, path, method, body, workspaceId);
  const text = await response.text();
  if (!response.ok) throw new Error(`${method} ${path} -> ${response.status}: ${text}`);
  return (text ? JSON.parse(text) : undefined) as T;
}

async function sql<T extends pg.QueryResultRow>(text: string, values: unknown[]): Promise<T[]> {
  if (!DATABASE_URL) throw new Error("DATABASE_URL is required for server-state assertions");
  const client = new pg.Client(DATABASE_URL);
  await client.connect();
  try { return (await client.query<T>(text, values)).rows; } finally { await client.end(); }
}

const issueTasks = (issueId: string) =>
  sql<TaskRow>("SELECT id, status, rerun_of_task_id::text FROM agent_task_queue WHERE issue_id = $1 ORDER BY created_at", [issueId]);

/** The owner invites the member through the API; the member accepts. */
async function join(owner: Actor, member: Actor, role = "member") {
  const invitation = await callJSON<{ id: string }>(owner, `/api/workspaces/${owner.workspace.id}/members`, "POST", { email: member.email, role });
  await callJSON(member, `/api/invitations/${invitation.id}/accept`, "POST", undefined, member.workspace.id);
}

/**
 * A daemon speaking the real HTTP protocol with the owner's session. It never
 * runs an agent CLI or a model: each lifecycle step is an explicit call.
 */
class SimulatedDaemon {
  readonly daemonId = randomUUID();
  runtimeId = "";
  constructor(private readonly actor: Actor) {}

  private async post<T>(path: string, body?: unknown): Promise<T> {
    const response = await fetch(`${API_BASE}/api/daemon${path}`, {
      method: "POST",
      headers: { Authorization: `Bearer ${this.actor.api.getToken()}`, "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const text = await response.text();
    if (!response.ok) throw new Error(`daemon ${path} -> ${response.status}: ${text}`);
    return (text ? JSON.parse(text) : undefined) as T;
  }

  async register() {
    const registered = await this.post<{ runtimes: Array<{ id: string }> }>("/register", {
      workspace_id: this.actor.workspace.id,
      daemon_id: this.daemonId,
      device_name: "core-journeys-simulated-daemon",
      cli_version: "0.6.0",
      runtimes: [{ name: "Simulated Codex", type: "codex", version: "0.0.0-e2e", status: "online" }],
    });
    expect(registered.runtimes).toHaveLength(1);
    this.runtimeId = registered.runtimes[0]!.id;
    return this.runtimeId;
  }

  heartbeat() { return this.post("/heartbeat", { runtime_id: this.runtimeId }); }
  async claim() {
    await this.heartbeat();
    return (await this.post<{ task: { id: string } | null }>(`/runtimes/${this.runtimeId}/tasks/claim`)).task;
  }
  start(taskId: string) { return this.post(`/tasks/${taskId}/start`); }
  progress(taskId: string, summary: string) { return this.post(`/tasks/${taskId}/progress`, { summary, step: 1, total: 2 }); }
  messages(taskId: string, content: string) {
    return this.post(`/tasks/${taskId}/messages`, { messages: [{ seq: 1, type: "text", content }] });
  }
  complete(taskId: string, output: string) { return this.post(`/tasks/${taskId}/complete`, { output }); }
  fail(taskId: string, error: string) { return this.post(`/tasks/${taskId}/fail`, { error, failure_reason: "agent_error" }); }
}

async function createAgent(owner: Actor, runtimeId: string) {
  return owner.api.requestJSON<{ id: string; name: string }>("/api/agents", {
    method: "POST",
    body: { name: `Core runner ${randomUUID().slice(0, 6)}`, runtime_id: runtimeId, visibility: "workspace" },
  });
}

async function openIssue(page: Page, workspace: Workspace, issue: Issue) {
  await page.goto(`/${workspace.slug}/issues/${issue.id}`, { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible({ timeout: 30_000 });
}

function executionLog(page: Page) {
  return page.locator("div.\\@container\\/execution-log");
}
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
    const rows = ownerSide.page.locator('[role="button"]').filter({ hasText: issue.title });
    await expect(rows).toHaveCount(1, { timeout: 30_000 });
    await expect(rows).toContainText("Mentioned");
    const marked = ownerSide.page.waitForResponse((r) => new URL(r.url()).pathname === `/api/inbox/${mention.id}/read`);
    await rows.click();
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
