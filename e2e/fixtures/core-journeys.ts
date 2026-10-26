import { randomUUID } from "node:crypto";
import pg from "pg";
import { expect, test as base, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { TestApiClient } from "../fixtures";

// Shared helpers for the core business journey specs (G1–G8 and the workflow
// journeys): real accounts and workspaces, server-state reads over the API or
// SQL, and a daemon that speaks the real HTTP protocol without running any
// agent CLI.
export type Workspace = { id: string; slug: string; name: string };
export type Actor = { api: TestApiClient; workspace: Workspace; userId: string; name: string; email: string };
export type Issue = { id: string; identifier: string; title: string; status: string };
export type TaskRow = { id: string; status: string; rerun_of_task_id: string | null };
export type InboxItem = { id: string; type: string; read: boolean; issue_id: string | null; title: string };

export const API_BASE = process.env.NEXT_PUBLIC_API_URL || `http://localhost:${process.env.PORT || "8080"}`;
const DATABASE_URL = process.env.DATABASE_URL;

export async function createActor(label: string): Promise<Actor> {
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

export const test = base.extend<{ owner: Actor; member: Actor }>({
  owner: async ({}, use) => {
    const actor = await createActor("owner");
    try { await use(actor); } finally { await actor.api.deleteFeatureWorkspace(actor.workspace.id); }
  },
  member: async ({}, use) => {
    const actor = await createActor("member");
    try { await use(actor); } finally { await actor.api.deleteFeatureWorkspace(actor.workspace.id); }
  },
});

export async function authenticate(page: Page, actor: Actor) {
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

export async function openAs(browser: Browser, actor: Actor): Promise<{ context: BrowserContext; page: Page }> {
  const context = await browser.newContext({ locale: "en-US", viewport: { width: 1440, height: 1000 } });
  const page = await context.newPage();
  await authenticate(page, actor);
  return { context, page };
}

export function call(actor: Actor, path: string, method = "GET", body?: unknown, workspaceId = actor.workspace.id) {
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

export async function callJSON<T>(actor: Actor, path: string, method = "GET", body?: unknown, workspaceId = actor.workspace.id): Promise<T> {
  const response = await call(actor, path, method, body, workspaceId);
  const text = await response.text();
  if (!response.ok) throw new Error(`${method} ${path} -> ${response.status}: ${text}`);
  return (text ? JSON.parse(text) : undefined) as T;
}

export async function sql<T extends pg.QueryResultRow>(text: string, values: unknown[]): Promise<T[]> {
  if (!DATABASE_URL) throw new Error("DATABASE_URL is required for server-state assertions");
  const client = new pg.Client(DATABASE_URL);
  await client.connect();
  try { return (await client.query<T>(text, values)).rows; } finally { await client.end(); }
}

export const issueTasks = (issueId: string) =>
  sql<TaskRow>("SELECT id, status, rerun_of_task_id::text FROM agent_task_queue WHERE issue_id = $1 ORDER BY created_at", [issueId]);

/** The owner invites the member through the API; the member accepts. */
export async function join(owner: Actor, member: Actor, role = "member") {
  const invitation = await callJSON<{ id: string }>(owner, `/api/workspaces/${owner.workspace.id}/members`, "POST", { email: member.email, role });
  await callJSON(member, `/api/invitations/${invitation.id}/accept`, "POST", undefined, member.workspace.id);
}

/** The claim payload; extra fields (auth_token, chat_session_id, ...) are kept for callers that need them. */
export type ClaimedTask = { id: string } & Record<string, unknown>;

/**
 * A daemon speaking the real HTTP protocol with the owner's session. It never
 * runs an agent CLI or a model: each lifecycle step is an explicit call.
 */
export class SimulatedDaemon {
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
  async claim(): Promise<ClaimedTask | null> {
    await this.heartbeat();
    return (await this.post<{ task: ClaimedTask | null }>(`/runtimes/${this.runtimeId}/tasks/claim`)).task;
  }
  start(taskId: string) { return this.post(`/tasks/${taskId}/start`); }
  progress(taskId: string, summary: string) { return this.post(`/tasks/${taskId}/progress`, { summary, step: 1, total: 2 }); }
  messages(taskId: string, content: string) {
    return this.post(`/tasks/${taskId}/messages`, { messages: [{ seq: 1, type: "text", content }] });
  }
  complete(taskId: string, output: string) { return this.post(`/tasks/${taskId}/complete`, { output }); }
  fail(taskId: string, error: string) { return this.post(`/tasks/${taskId}/fail`, { error, failure_reason: "agent_error" }); }
}

export async function createAgent(owner: Actor, runtimeId: string) {
  return owner.api.requestJSON<{ id: string; name: string }>("/api/agents", {
    method: "POST",
    body: { name: `Core runner ${randomUUID().slice(0, 6)}`, runtime_id: runtimeId, visibility: "workspace" },
  });
}

export async function openIssue(page: Page, workspace: Workspace, issue: Issue) {
  await page.goto(`/${workspace.slug}/issues/${issue.id}`, { waitUntil: "domcontentloaded" });
  await expect(page.getByRole("button", { name: issue.title, exact: true })).toBeVisible({ timeout: 30_000 });
}

export function executionLog(page: Page) {
  return page.locator("div.\\@container\\/execution-log");
}
