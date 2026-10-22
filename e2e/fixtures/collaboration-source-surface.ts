import { randomUUID, createHash } from "node:crypto";
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, statSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, isAbsolute } from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { Client } from "pg";
import { expect, type Locator, type Page, type Request } from "@playwright/test";
import type { CollaborationCommentReceipt, CollaborationIssue } from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";

const execute = promisify(execFile);
const rootFor = (id: string) => `/api/collaboration/issues/${id}`;

interface SourceAccount { user_id: string; name: string; avatar_url: string; external: boolean }
interface SourceIssue { id: string; title: string; status: string; position: number; revision: number;
  collaboration?: { creator: SourceAccount; handler: SourceAccount; has_external_collaborators: boolean } }

function environment() {
  const apiUrl = process.env.COLLABORATION_E2E_API_URL;
  const database = process.env.DATABASE_URL;
  for (const value of [apiUrl, database, process.env.NEXT_PUBLIC_API_URL]) {
    if (!value || !["localhost", "127.0.0.1"].includes(new URL(value).hostname)) throw new Error("Source acceptance requires task-owned loopback services");
  }
  if (process.env.E2E_PASSWORD_AUTH !== "1") throw new Error("Source acceptance requires password sessions");
  expect(new URL(apiUrl!).origin).toBe(new URL(process.env.NEXT_PUBLIC_API_URL!).origin);
  return { apiUrl: apiUrl!, database: database! };
}

async function actor(apiUrl: string, suffix: string, role: string) {
  const client = new TestApiClient();
  const username = `s6source${role}${suffix}`;
  const password = `Source-${randomUUID()}!`;
  const name = role === "external" ? `External source handler ${suffix}` : `Source ${role} ${suffix}`;
  const user = await client.registerPassword(username, password, name);
  expect((await client.requestJSON<{ onboarded_at: string | null }>("/api/me/onboarding/complete", { method: "POST", body: {} })).onboarded_at).not.toBeNull();
  return { client, id: user.id, name, async authenticate(page: Page) {
    const response = await page.request.post(`${apiUrl}/auth/login`, { data: { username, password } });
    expect(response.status()).toBe(200);
    const cookies = await page.context().cookies(apiUrl);
    expect(cookies.some((cookie) => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
    expect(cookies.some((cookie) => cookie.name === "multica_csrf" && !cookie.httpOnly)).toBe(true);
    await page.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
  } };
}

export async function seedSourceSurface() {
  const env = environment();
  const suffix = randomUUID().replaceAll("-", "").slice(0, 12);
  const creator = await actor(env.apiUrl, suffix, "creator");
  const external = await actor(env.apiUrl, suffix, "external");
  const observer = await actor(env.apiUrl, suffix, "observer");
  expect(await external.client.getWorkspaces()).toEqual([]);
  const workspace = await creator.client.ensureWorkspace("Source collaboration acceptance", `source-${suffix}`);
  // Password-only accounts have no email. This fixture owns an ordinary source
  // member fact; workspace invitation UX is outside the source surface slice.
  const members = new Client({ connectionString: env.database });
  await members.connect();
  try {
    const inserted = await members.query("INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'member') RETURNING id", [workspace.id, observer.id]);
    expect(inserted.rows).toHaveLength(1);
  } finally { await members.end(); }
  expect((await observer.client.getWorkspaces()).some((item) => item.id === workspace.id)).toBe(true);
  const project = await creator.client.requestJSON<{ id: string; title: string }>("/api/projects", { method: "POST", body: { title: "Source timeline project" } });
  const dates = { start_date: "2026-10-22", due_date: "2026-11-02" };
  const shared = await creator.client.requestJSON<CollaborationIssue>("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { source_workspace_id: workspace.id, handler_user_id: external.id,
      title: `Shared source identity ${suffix}`, description: "Private source collaboration body.", project_id: project.id, ...dates, attachment_ids: [] },
  });
  const first = await creator.client.requestJSON<SourceIssue>("/api/issues", { method: "POST", body: {
    title: `Ordinary reorder first ${suffix}`, status: "todo", assignee_type: "member", assignee_id: creator.id, project_id: project.id, ...dates,
  } });
  const second = await creator.client.requestJSON<SourceIssue>("/api/issues", { method: "POST", body: {
    title: `Ordinary reorder second ${suffix}`, status: "todo", assignee_type: "member", assignee_id: creator.id, project_id: project.id, ...dates,
  } });
  // Owns the ordinary cross-status drag so the shared Todo column order is undisturbed.
  const control = await creator.client.requestJSON<SourceIssue>("/api/issues", { method: "POST", body: {
    title: `Ordinary status control ${suffix}`, status: "backlog", assignee_type: "member", assignee_id: creator.id, project_id: project.id, ...dates,
  } });
  const comment = await external.client.requestJSON<CollaborationCommentReceipt>(`${rootFor(shared.id)}/comments`, {
    method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: { content: "External source comment with retained identity",
      expected_lifecycle_epoch: shared.lifecycle_epoch, expected_participant_epoch: "1", expected_account_epoch: "1", target_versions: [] },
  });
  expect(comment.targets).toEqual([]);
  const reads = await creator.client.requestJSON<{ issues: SourceIssue[] }>("/api/issues?include_closed=true&limit=100");
  const source = reads.issues.find((issue) => issue.id === shared.id)!;
  expect(source.collaboration?.handler).toMatchObject({ user_id: external.id, name: external.name, external: true });
  for (const issue of [source, first, second]) {
    await creator.client.requestJSON(`/api/issues/${issue.id}`, { method: "PUT", body: { position: [source, first, second].indexOf(issue) * 100,
      expected_revision: issue.revision } });
  }
  expect(source.status).toBe("todo");
  return { ...env, creator, external, observer, workspace, project, shared, comment, first, second, control,
    listPath: `/${workspace.slug}/issues`, projectPath: `/${workspace.slug}/projects/${project.id}?section=issues`, detailPath: `/${workspace.slug}/issues/${shared.id}` };
}
export type SourceSurface = Awaited<ReturnType<typeof seedSourceSurface>>;

export async function sourceSnapshot(fixture: SourceSurface) {
  const client = new Client({ connectionString: fixture.database });
  await client.connect();
  try {
    const result = await client.query(`SELECT jsonb_build_object(
      'revision',c.revision,'lifecycle_epoch',c.lifecycle_epoch,'assignee',i.assignee_id,'status',i.status,
      'events',(SELECT jsonb_agg(to_jsonb(e)-'outbox_state' ORDER BY e.event_seq) FROM collaboration_event e WHERE e.issue_id=i.id),
      'notifications',(SELECT jsonb_agg(to_jsonb(n) ORDER BY n.id) FROM collaboration_notification n WHERE n.issue_id=i.id),
      'session',(SELECT to_jsonb(s) FROM collaboration_session s WHERE s.issue_id=i.id)
      ) AS snapshot FROM issue i JOIN issue_collaboration c ON c.issue_id=i.id WHERE i.id=$1`, [fixture.shared.id]);
    expect(result.rows).toHaveLength(1);
    return result.rows[0]!.snapshot;
  } finally { await client.end(); }
}

export async function sourceCurrent(fixture: SourceSurface, id = fixture.shared.id) {
  return fixture.creator.client.requestJSON<SourceIssue>(`/api/issues/${id}`);
}

export async function chooseView(page: Page, view: "List" | "Board" | "Gantt" | "Table") {
  const button = page.getByRole("button", { name: /^(List|Board|Table|Swimlane|Gantt)$/ }).last();
  await button.click();
  await page.getByRole("menuitemradio", { name: view, exact: true }).click();
}

export async function boardDisplay(page: Page, grouping: "Status" | "Assignee") {
  await page.getByRole("button", { name: "Display", exact: true }).click();
  const group = page.getByRole("combobox", { name: "Grouping", exact: true });
  await group.click();
  await page.getByRole("option", { name: grouping, exact: true }).click();
  const ordering = page.getByRole("combobox", { name: "Ordering", exact: true });
  await ordering.click();
  await page.getByRole("option", { name: "Manual", exact: true }).click();
  const assignee = page.getByRole("button", { name: "Assignee", exact: true }).last();
  if (await assignee.getAttribute("aria-pressed") !== "true") await assignee.click();
  await page.keyboard.press("Escape");
}

// Cards inside a column scroll root; the DragOverlay portal clone is excluded.
export const boardCard = (page: Page, title: string) => page.locator('[data-tab-scroll-root^="board:"] [data-board-card]').filter({ hasText: title }).first();

/** Droppable scroll root of a status column, found through its labelled header action. */
export function statusColumn(page: Page, status: "Backlog" | "Todo" | "In Progress" | "In Review" | "Done" | "Cancelled") {
  return page.getByRole("button", { name: `Actions for ${status} column`, exact: true }).first()
    .locator('xpath=ancestor::div[.//*[starts-with(@data-tab-scroll-root,"board:")]][1]//*[starts-with(@data-tab-scroll-root,"board:")]').first();
}

/** Every column holding a visible copy of the card; an optimistic move would change it. */
export async function cardColumns(page: Page, title: string) {
  return page.locator('[data-tab-scroll-root^="board:"]').evaluateAll((roots, text) => roots
    .filter((root) => [...root.querySelectorAll("[data-board-card]")].some((card) => card.textContent?.includes(text)))
    .map((root) => root.getAttribute("data-tab-scroll-root")), title);
}

/** Records non-GET writes to the shared issue's ordinary and collaboration endpoints. */
export function observeSharedWrites(page: Page, fixture: SourceSurface) {
  const writes: { method: string; path: string; body: string }[] = [];
  const listener = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (request.method() === "GET") return;
    if (path === `/api/issues/${fixture.shared.id}` || path.startsWith(`/api/issues/${fixture.shared.id}/`) || path.startsWith(`${rootFor(fixture.shared.id)}/`) || path === rootFor(fixture.shared.id)) {
      writes.push({ method: request.method(), path, body: request.postData() ?? "" });
    }
  };
  page.on("request", listener);
  return { writes, dispose: () => page.off("request", listener) };
}

/**
 * Desktop's main window projects a tab-owned MemoryRouter (initial "/"), so a
 * renderer URL cannot select a workspace route and `multica://` deep links only
 * open account paths. Navigate through the same sidebar/row controls a user
 * clicks, as `project-p1-desktop.ts::openNativeProject` does.
 */
export async function navigateNative(page: Page, fixture: SourceSurface, path: string) {
  const sidebar = (name: string) => page.getByRole("link", { name: name === "Issues" ? /^(Issues|Workspace issues)$/ : name, exact: true }).and(page.locator('[data-sidebar="menu-button"]'));
  if (path === fixture.listPath) {
    if (await sidebar("Issues").count() === 0) {
      await page.getByRole("combobox", { name: "Workspaces", exact: true }).selectOption(fixture.workspace.slug);
    }
    await sidebar("Issues").click();
  } else if (path === fixture.projectPath) {
    await sidebar("Projects").click();
    await page.getByText(fixture.project.title, { exact: true }).first().click();
    await page.getByRole("button", { name: "Issues", exact: true }).last().click();
  } else if (path === fixture.detailPath) {
    await sidebar("Issues").click();
    await chooseView(page, "List");
    await page.locator("a").filter({ hasText: fixture.shared.title }).first().click();
  } else {
    throw new Error(`No native navigation is defined for ${path}`);
  }
}
export async function dragBoard(page: Page, source: Locator, destination: Locator, checkWhileOver?: () => Promise<void>) {
  await source.scrollIntoViewIfNeeded();
  const from = await source.boundingBox();
  if (!from) throw new Error("Source board drag requires a visible source");
  await page.mouse.move(from.x + from.width / 2, from.y + 20);
  await page.mouse.down();
  await page.mouse.move(from.x + from.width / 2 + 12, from.y + 20, { steps: 4 });
  // Reveal distant columns only after grabbing the source; scrolling first
  // can put the source offscreen and turn a terminal drop into a local reorder.
  await destination.scrollIntoViewIfNeeded();
  const to = await destination.boundingBox();
  if (!to) throw new Error("Source board drag requires a visible destination");
  await page.mouse.move(to.x + to.width / 2, to.y + Math.min(35, to.height / 2), { steps: 14 });
  await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
  // An ordinary card moves optimistically and can become the pointer's new
  // collision target during drag-over. Re-sample the actual destination after
  // layout settles before releasing so the drop observes the current column.
  const settled = await destination.boundingBox();
  if (!settled) throw new Error("Source board destination disappeared during drag");
  await page.mouse.move(settled.x + settled.width / 2 + 1, settled.y + Math.min(35, settled.height / 2) + 1, { steps: 2 });
  // Native sorting transforms can still be mid-transition. Complete the
  // user pointer path at the current destination rather than the stale box.
  await page.evaluate(async () => {
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
    await Promise.all(document.getAnimations().filter((animation) => animation.effect?.getComputedTiming().iterations !== Infinity)
      .map((animation) => animation.finished.catch(() => undefined)));
  });
  const final = await destination.boundingBox();
  if (!final) throw new Error("Source board destination disappeared before drop");
  await page.mouse.move(final.x + final.width / 2, final.y + Math.min(60, final.height / 2), { steps: 2 });
  await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => resolve())));
  await checkWhileOver?.();
  await page.mouse.up();
}

export async function runReorderCLI(fixture: SourceSurface, beforeId: string) {
  const binary = process.env.COLLABORATION_SOURCE_CLI;
  if (!binary || !isAbsolute(binary) || !statSync(binary).isFile()) throw new Error("Explicit freshly built source acceptance CLI is required");
  const isolated = mkdtempSync(join(tmpdir(), "multica-source-cli-"));
  try {
    mkdirSync(join(isolated, ".multica"), { mode: 0o700 });
    const config = { server_url: fixture.apiUrl, workspace_id: fixture.workspace.id, token: fixture.creator.client.getToken()! };
    writeFileSync(join(isolated, ".multica/config.json"), JSON.stringify(config), { mode: 0o600 });
    const env: NodeJS.ProcessEnv = { ...process.env, MULTICA_TASK_CONFIG_ROOT: isolated, MULTICA_SERVER_URL: fixture.apiUrl, MULTICA_WORKSPACE_ID: fixture.workspace.id,
      MULTICA_TOKEN: fixture.creator.client.getToken()! };
    for (const key of Object.keys(env)) if (key.startsWith("MULTICA_COLLABORATION_") || ["MULTICA_AGENT_ID", "MULTICA_TASK_ID", "MULTICA_DAEMON_PORT"].includes(key)) delete env[key];
    const result = await execute(binary, ["issue", "reorder", fixture.shared.id, "--before", beforeId], { env, cwd: isolated, timeout: 30_000 });
    const issue: unknown = JSON.parse(result.stdout);
    expect(issue).toMatchObject({ id: fixture.shared.id });
    return { command: ["<task-owned-built-cli>", "issue", "reorder", fixture.shared.id, "--before", beforeId],
      binary_sha256: createHash("sha256").update(readFileSync(binary)).digest("hex"), result: issue };
  } finally { rmSync(isolated, { recursive: true, force: true }); }
}
