import assert from "node:assert/strict";
import { createHash, randomUUID } from "node:crypto";
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { isAbsolute, join, relative, resolve } from "node:path";
import { expect, type Browser, type BrowserContext, type Page, type Response, type TestInfo } from "@playwright/test";
import pg from "pg";
import { CommentSchema, CreateIssueResponseSchema, InboxItemListSchema, InboxSnoozeReceiptSchema } from "../../packages/core/api/schemas";
import { TestApiClient } from "../fixtures";
import { nativeScreenshot, openDesktop, type NativeHarness } from "./collaboration-native";

type ObjectValue = Record<string, unknown>;
const object = (value: unknown): ObjectValue => {
  assert(value !== null && typeof value === "object" && !Array.isArray(value), "Expected an object response");
  return value as ObjectValue;
};
const string = (value: unknown): string => { assert(typeof value === "string" && value.length > 0, "Expected a nonempty string"); return value; };
const stamp = () => new Date().toISOString();
const digest = (value: string) => createHash("sha256").update(value).digest("hex");

export type InboxNotice = ReturnType<typeof InboxItemListSchema.parse>[number];
export type SnoozeReceipt = ReturnType<typeof InboxSnoozeReceiptSchema.parse>;
export type SnoozeIssue = ReturnType<typeof CreateIssueResponseSchema.parse>;
export type SnoozeComment = ReturnType<typeof CommentSchema.parse>;
export type SnoozeLocale = "en" | "zh-Hans";
export interface SnoozeWorkspace { id: string; slug: string; name: string }

interface Allocation {
  privateRoot: string;
  database: string;
  databaseUrl: string;
  apiUrl: string;
  webUrl: string;
  rendererUrl: string;
  sourceRoot: string;
  runId: string;
}

/** A configured run fails admission; an absent opt-in never starts fixtures. */
export async function requireSnoozeAllocation(): Promise<Allocation> {
  assert.equal(process.env.INBOX_SNOOZE_E2E_RUNTIME_READY, "1", "Inbox snooze requires the coordinator's runtime-ready permit");
  const allocationPath = string(process.env.INBOX_SNOOZE_E2E_ALLOCATION);
  assert(isAbsolute(allocationPath), "The allocation path must be absolute");
  const allocation = object(JSON.parse(readFileSync(allocationPath, "utf8")));
  const privateRoot = string(allocation.private_root);
  const privateEnvironment = object(JSON.parse(readFileSync(string(allocation.private_environment_file), "utf8")));
  const urls = object(privateEnvironment.urls);
  const ports = object(allocation.runtime_ports);
  const database = string(object(allocation.databases).api);
  const databaseUrl = string(urls.api);
  const sourceRoot = string(allocation.source_root);
  assert.equal(resolve(sourceRoot), resolve(import.meta.dirname, "../.."), "The renderer and test checkout must match");
  assert.equal(process.env.TASK_DATABASE_NAME, database, "The selected database must match the recorded API database");
  assert(process.env.DATABASE_URL === databaseUrl, "The database connection must match the private allocation");
  const apiEnvironment = object(privateEnvironment.api_env);
  const apiUrl = new URL(string(apiEnvironment.NEXT_PUBLIC_API_URL)).origin;
  assert(["127.0.0.1", "localhost"].includes(new URL(apiUrl).hostname), "The API must use the allocated loopback listener");
  assert.equal(new URL(apiUrl).port, String(ports.api), "The API port must match the allocation");
  const webUrl = new URL(string(apiEnvironment.FRONTEND_ORIGIN)).origin;
  const rendererUrl = new URL(string(object(privateEnvironment.e2e_env).COLLABORATION_ELECTRON_RENDERER_URL)).origin;
  for (const [url, port] of [[webUrl, ports.web], [rendererUrl, ports.renderer]]) {
    assert(["127.0.0.1", "localhost"].includes(new URL(string(url)).hostname), "Clients must use allocated loopback listeners");
    assert.equal(new URL(string(url)).port, String(port), "Client ports must match the allocation");
  }
  for (const [actual, expected] of [
    [process.env.NEXT_PUBLIC_API_URL, apiUrl], [process.env.COLLABORATION_E2E_API_URL, apiUrl],
    [process.env.PLAYWRIGHT_BASE_URL, webUrl], [process.env.COLLABORATION_ELECTRON_RENDERER_URL, rendererUrl],
  ]) assert.equal(new URL(string(actual)).origin, expected, "The client origin must match the recorded runtime allocation");
  assert.equal(resolve(string(process.env.COLLABORATION_ELECTRON_SOURCE_ROOT)), resolve(sourceRoot));
  assert(isAbsolute(string(process.env.PLAYWRIGHT_BROWSERS_PATH)), "The coordinator must bind its verified browser cache");
  assert.equal(decodeURIComponent(new URL(databaseUrl).pathname.slice(1)), database);
  assert(!["multica", "postgres", "template0", "template1"].includes(database), "A private API database is required");
  const databaseClient = new pg.Client({ connectionString: databaseUrl, connectionTimeoutMillis: 3000 });
  try {
    await databaseClient.connect();
    const result = await databaseClient.query<{ name: string }>("SELECT current_database() AS name");
    assert.equal(result.rows[0]?.name, database, "The connected database must match the allocation");
  } catch {
    throw new Error("The owned snooze API database could not be verified");
  } finally { await databaseClient.end(); }
  const configuration = await fetch(`${apiUrl}/api/config`);
  expect(configuration.status).toBe(200);
  expect(await configuration.json()).toMatchObject({ auth_mode: "password" });
  return { privateRoot, database, databaseUrl, apiUrl, webUrl, rendererUrl, sourceRoot, runId: string(allocation.run_id) };
}

export class SnoozeEvidence {
  readonly facts: ObjectValue[] = [];
  readonly screenshots: string[] = [];
  constructor(readonly info: TestInfo, readonly allocation: Allocation) {
    const output = relative(allocation.privateRoot, info.outputDir);
    assert(output && !output.startsWith("..") && !isAbsolute(output), "Snooze evidence must stay in the task's private output directory");
    mkdirSync(info.outputDir, { recursive: true, mode: 0o700 });
    this.record("run_identity", { run_id: allocation.runId, test: info.title, source_root: allocation.sourceRoot,
      api_origin: allocation.apiUrl, web_origin: allocation.webUrl, renderer_origin: allocation.rendererUrl,
      database: allocation.database, retries: info.retry, worker: info.workerIndex });
  }
  record(phase: string, data: ObjectValue = {}) {
    this.facts.push({ at: stamp(), phase, ...data });
    writeFileSync(this.info.outputPath("inbox-snooze-evidence.json"), `${JSON.stringify({ facts: this.facts, screenshots: this.screenshots }, null, 2)}\n`, { mode: 0o600 });
  }
  async attach() {
    await this.info.attach("inbox-snooze-evidence", { path: this.info.outputPath("inbox-snooze-evidence.json"), contentType: "application/json" });
  }
}

export interface SnoozeActor {
  id: string;
  name: string;
  client: TestApiClient;
  authenticate(page: Page): Promise<void>;
}

export async function createSnoozeActors(allocation: Allocation) {
  const suffix = randomUUID().replaceAll("-", "").slice(0, 12);
  const actor = async (role: "owner" | "author"): Promise<SnoozeActor> => {
    const client = new TestApiClient();
    const username = `snooze_${role}_${suffix}`;
    const password = `Snooze-${randomUUID()}!`;
    const name = `Snooze ${role} ${suffix}`;
    const registered = await client.registerPassword(username, password, name);
    await client.requestJSON("/api/me/onboarding", { method: "PATCH", body: { questionnaire: { source: ["friends_colleagues"] } } });
    const onboarded = object(await client.requestJSON("/api/me/onboarding/complete", { method: "POST", body: {} }));
    expect(onboarded.id).toBe(registered.id); expect(onboarded.onboarded_at).not.toBeNull();
    string(onboarded.onboarded_at);
    return { id: registered.id, name, client,
      async authenticate(page) {
        const response = await page.request.post(`${allocation.apiUrl}/auth/login`, { data: { username, password } });
        expect(response.status()).toBe(200);
        expect(object(object(await response.json()).user).id).toBe(registered.id);
        const cookies = await page.context().cookies(allocation.apiUrl);
        expect(cookies.some((cookie) => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
        expect(cookies.some((cookie) => cookie.name === "multica_csrf" && !cookie.httpOnly)).toBe(true);
      } };
  };
  const owner = await actor("owner"), author = await actor("author");
  expect(owner.id).not.toBe(author.id);
  return { owner, author };
}

export interface SnoozeFixture {
  allocation: Allocation;
  actors: Awaited<ReturnType<typeof createSnoozeActors>>;
  workspace: SnoozeWorkspace;
  other: SnoozeWorkspace;
  evidence: SnoozeEvidence;
}

export async function createSnoozeFixture(allocation: Allocation, actors: SnoozeFixture["actors"], evidence: SnoozeEvidence): Promise<SnoozeFixture> {
  const suffix = randomUUID().replaceAll("-", "").slice(0, 12);
  await actors.owner.client.requestJSON("/api/me", { method: "PATCH", body: { language: "en" } });
  const createWorkspace = async (label: string): Promise<SnoozeWorkspace> => {
    const created = object(await actors.owner.client.requestJSON("/api/workspaces", { method: "POST", body: {
      name: `Inbox snooze ${label} ${suffix}`, slug: `snooze-${label}-${suffix}`, issue_prefix: `Z${randomUUID().replaceAll("-", "").slice(0, 8)}`.toUpperCase(),
    } }));
    return { id: string(created.id), slug: string(created.slug), name: string(created.name) };
  };
  const workspace = await createWorkspace("source"), other = await createWorkspace("other");
  selectApiWorkspace(actors.owner, workspace);
  const link = object(await actors.owner.client.requestJSON(`/api/workspaces/${workspace.id}/share-links`, {
    method: "POST", body: { role: "member", expires_in: 1, max_uses: 1 },
  }));
  const joined = object(await actors.author.client.requestJSON("/api/share-links/join", { method: "POST", body: { code: string(link.code) } }));
  expect(joined.workspace_id).toBe(workspace.id); expect(object(joined.member).user_id).toBe(actors.author.id);
  selectApiWorkspace(actors.author, workspace);
  evidence.record("registered_onboarded_actors_and_public_workspaces", { actors: { owner: actors.owner.id, author: actors.author.id },
    source_workspace: workspace, other_workspace: other, author_member_id: string(object(joined.member).id),
    share_link_id: string(link.id), share_code_sha256: digest(string(link.code)), fixture_source: "TestApiClient public API" });
  return { allocation, actors, workspace, other, evidence };
}

export function selectApiWorkspace(actor: SnoozeActor, workspace: SnoozeWorkspace) {
  actor.client.setWorkspaceId(workspace.id); actor.client.setWorkspaceSlug(workspace.slug);
}

export async function seedSnoozeIssue(fixture: SnoozeFixture, label: string, comments = 1) {
  const { actors, workspace, evidence } = fixture;
  selectApiWorkspace(actors.author, workspace); selectApiWorkspace(actors.owner, workspace);
  const issue = CreateIssueResponseSchema.parse(await actors.author.client.requestJSON("/api/issues", { method: "POST", body: {
    title: `Snooze ${label} ${randomUUID().slice(0, 8)}`, status: "todo", assignee_type: null, assignee_id: null,
    description: "A synthetic ordinary task for the inbox snooze journey.",
  } }));
  expect(issue.workspace_id).toBe(workspace.id); expect(issue.creator_id).toBe(actors.author.id);
  expect(await actors.owner.client.requestJSON(`/api/issues/${issue.id}/subscribe`, { method: "POST", body: {} })).toEqual({ subscribed: true });
  const sources: SnoozeComment[] = [];
  for (let index = 0; index < comments; index++) sources.push(await postSnoozeComment(fixture, issue, `Original ${label} comment ${index + 1}`));
  await expect.poll(async () => (await listSnoozeNotices(fixture, "inbox")).filter((item) => item.issue_id === issue.id).length).toBe(comments);
  const notices = (await listSnoozeNotices(fixture, "inbox")).filter((item) => item.issue_id === issue.id);
  evidence.record("original_group_sources", { issue_id: issue.id, title: issue.title,
    comment_ids: sources.map((comment) => comment.id), notification_ids: notices.map((item) => item.id), notices: notices.map(noticeFact) });
  return { issue, sources, notices };
}

export async function postSnoozeComment(fixture: SnoozeFixture, issue: SnoozeIssue, content: string) {
  selectApiWorkspace(fixture.actors.author, fixture.workspace);
  const comment = CommentSchema.parse(await fixture.actors.author.client.requestJSON(`/api/issues/${issue.id}/comments`, { method: "POST", body: { content } }));
  expect(comment.issue_id).toBe(issue.id); expect(comment.author_id).toBe(fixture.actors.author.id);
  fixture.evidence.record("public_comment_source", { issue_id: issue.id, comment_id: comment.id, actor_id: comment.author_id,
    content_sha256: digest(comment.content), revision: comment.revision, created_at: comment.created_at });
  return comment;
}

export async function listSnoozeNotices(fixture: SnoozeFixture, view: "inbox" | "snoozed" | "archived", workspace = fixture.workspace) {
  selectApiWorkspace(fixture.actors.owner, workspace);
  const items = InboxItemListSchema.parse(await fixture.actors.owner.client.requestJSON(`/api/inbox${view === "inbox" ? "" : `/${view}`}`));
  expect(items.every((item) => item.workspace_id === workspace.id && item.recipient_id === fixture.actors.owner.id)).toBe(true);
  return items;
}

export function noticeFact(item: InboxNotice) {
  const details = item.details !== null && typeof item.details === "object" ? object(item.details) : {};
  return { id: item.id, workspace_id: item.workspace_id, issue_id: item.issue_id, recipient_id: item.recipient_id,
    type: item.type, comment_id: details.comment_id ?? null, mention_scope: details.mention_scope ?? null,
    read: item.read, archived: item.archived, created_at: item.created_at, snoozed_until: item.snoozed_until ?? null,
    snooze_revision: item.snooze_revision ?? 0, reminded_at: item.reminded_at ?? null, reminder_reason: item.reminder_reason ?? null };
}

function receiptFact(receipt: SnoozeReceipt) {
  return { workspace_id: receipt.workspace_id, group_kind: receipt.group_kind, group_id: receipt.group_id,
    revision: receipt.revision, snoozed_until: receipt.snoozed_until, reminded_at: receipt.reminded_at,
    reminder_reason: receipt.reminder_reason ?? null, undo_available: typeof receipt.undo_token === "string" };
}

/** Observe the real transport, retaining resource identities instead of credentials or message bodies. */
export class SnoozeTraffic {
  readonly responses: ObjectValue[] = [];
  readonly events: ObjectValue[] = [];
  readonly errors: string[] = [];
  readonly principalIds: string[] = [];
  private pending = new Set<Promise<void>>();
  constructor(readonly page: Page, readonly evidence: SnoozeEvidence, readonly client: string) {
    page.on("response", (response) => {
      const path = new URL(response.url()).pathname;
      if (!path.startsWith("/api/inbox") && path !== "/api/me") return;
      const read = (async () => {
        const fact: ObjectValue = { client, at: stamp(), path, method: response.request().method(), status: response.status() };
        const headers = await response.request().allHeaders();
        fact.workspace_slug = headers["x-workspace-slug"] ?? null; fact.workspace_id = headers["x-workspace-id"] ?? null;
        fact.authentication = { cookie: Boolean(headers.cookie), csrf: Boolean(headers["x-csrf-token"]), bearer: Boolean(headers.authorization) };
        if (response.status() === 200) {
          if (path === "/api/me") { const id = string(object(await response.json()).id); this.principalIds.push(id); fact.actor_id = id; }
          else if (["/api/inbox", "/api/inbox/snoozed", "/api/inbox/archived"].includes(path)) fact.items = InboxItemListSchema.parse(await response.json()).map(noticeFact);
          else if (/\/(snooze|unsnooze|snooze\/undo)$/.test(path)) fact.receipt = receiptFact(InboxSnoozeReceiptSchema.parse(await response.json()));
          else if (path === "/api/inbox/unread-count" || path === "/api/inbox/unread-summary") fact.projection = await response.json();
        }
        this.responses.push(fact); evidence.record("actual_inbox_response", fact);
      })().catch((error) => {
        const message = error instanceof Error ? error.message : "";
        const reason = message.includes("No resource with given identifier") ? "body_resource_unavailable"
          : /closed|disposed/i.test(message) ? "client_closed"
          : /cancel|abort/i.test(message) ? "response_cancelled"
          : error instanceof Error && error.name === "ZodError" ? "response_schema_failed" : "response_observation_failed";
        this.errors.push(`${path}:${reason}`);
        evidence.record("traffic_observation_failed", { client, path, status: response.status(), method: response.request().method(),
          reason, error_type: error instanceof Error ? error.name : "unknown", message_sha256: digest(message) });
      });
      this.pending.add(read); void read.finally(() => this.pending.delete(read));
    });
    page.on("websocket", (socket) => {
      const url = new URL(socket.url());
      evidence.record("actual_socket_opened", { client, origin: url.origin, path: url.pathname });
      socket.on("framereceived", ({ payload }) => {
        try {
          const message = object(JSON.parse(typeof payload === "string" ? payload : payload.toString()));
          const type = string(message.type);
          if (!type.startsWith("inbox:")) return;
          const body = object(message.payload);
          const event = { client, at: stamp(), type, workspace_id: body.workspace_id, recipient_id: body.recipient_id,
            group_kind: body.group_kind, group_id: body.group_id, revision: body.revision, reason: body.reason,
            notification_id: body.id, issue_id: body.issue_id, notification_type: body.type };
          this.events.push(event); evidence.record("actual_personal_event", event);
        } catch { this.errors.push("socket_observation_failed"); }
      });
    });
    page.on("pageerror", (error) => { this.errors.push(`pageerror:${digest(error.message)}`); });
  }
  async settle() { await Promise.all([...this.pending]); expect(this.errors).toEqual([]); }
}

export interface SnoozeSession {
  page: Page;
  context: BrowserContext;
  native?: Awaited<ReturnType<typeof openDesktop>>;
  traffic: SnoozeTraffic;
  capture(name: string): Promise<void>;
  close(): Promise<void>;
}

export async function openSnoozeSession(browser: Browser, fixture: SnoozeFixture, platform: "web" | "desktop", options: {
  locale?: SnoozeLocale; touch?: boolean; width?: number; height?: number;
} = {}): Promise<SnoozeSession> {
  const { allocation, evidence, actors } = fixture;
  const locale = options.locale ?? "en";
  let native: SnoozeSession["native"];
  let context: BrowserContext;
  let page: Page;
  if (platform === "desktop") {
    const profiles = join(allocation.privateRoot, "snooze-native-profiles");
    mkdirSync(profiles, { recursive: true, mode: 0o700 });
    const profile = mkdtempSync(join(profiles, "session-"));
    native = await openDesktop(profile, { token: string(actors.owner.client.getToken()), serverUrl: allocation.apiUrl });
    context = native.app.context(); page = native.page;
    evidence.record("native_profile_identity", { profile, pid: native.app.process().pid });
  } else {
    context = await browser.newContext({ locale: locale === "en" ? "en-US" : "zh-CN", timezoneId: "Asia/Shanghai",
      hasTouch: options.touch ?? false, viewport: { width: options.width ?? 1440, height: options.height ?? 1000 } });
    page = await context.newPage();
    await actors.owner.authenticate(page);
  }
  await context.addInitScript(({ language, desktop, seed }) => {
    if (!desktop) localStorage.removeItem("multica_token");
    if (localStorage.getItem("inbox-snooze-client-seed") === seed) return;
    localStorage.setItem("inbox-snooze-client-seed", seed);
    localStorage.setItem("multica-locale", language);
    localStorage.setItem("multica:chat:isOpen", "false");
    localStorage.setItem("theme", "light");
  }, { language: locale, desktop: platform === "desktop", seed: randomUUID() });
  await context.addCookies([{ name: "multica-locale", value: locale, url: platform === "desktop" ? allocation.rendererUrl : allocation.webUrl }]);
  const traffic = new SnoozeTraffic(page, evidence, `${platform}-${randomUUID().slice(0, 8)}`);
  try {
    if (native) {
      await native.start();
      await expect(page.getByRole("navigation", { name: "Application navigation", exact: true })).toBeVisible();
      await page.evaluate((path) => window.dispatchEvent(new CustomEvent("multica:navigate", { detail: { path } })), `/${fixture.workspace.slug}/inbox`);
    } else await page.goto(`${allocation.webUrl}/${fixture.workspace.slug}/inbox`, { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: locale === "en" ? "Inbox" : "收件箱", exact: true })).toBeVisible();
    await expect.poll(() => traffic.principalIds.includes(actors.owner.id)).toBe(true);
  } catch (error) {
    const failurePath = evidence.info.outputPath(`${platform}-acquisition-failed.png`);
    try {
      if (native) await nativeScreenshot(native.app, page, failurePath); else await page.screenshot({ path: failurePath });
      evidence.screenshots.push(failurePath);
    } catch { evidence.record("acquisition_capture_failed", { platform }); }
    evidence.record("client_acquisition_failed", { platform, pathname: new URL(page.url()).pathname, observed_principals: traffic.principalIds,
      response_observation_errors: traffic.errors, error_type: error instanceof Error ? error.name : "unknown" });
    if (native) await native.app.close(); else await context.close();
    throw error;
  }
  return { page, context, native, traffic,
    async capture(name) {
      await page.evaluate(async () => { await document.fonts.ready; await new Promise<void>((done) => requestAnimationFrame(() => requestAnimationFrame(() => done()))); });
      const path = evidence.info.outputPath(`${platform}-${name}.png`);
      if (native) await nativeScreenshot(native.app, page, path); else await page.screenshot({ path, fullPage: false });
      evidence.screenshots.push(path); evidence.record("actual_product_screenshot", { platform, path,
        viewport: await page.evaluate(() => ({ width: innerWidth, height: innerHeight, devicePixelRatio })) });
    },
    async close() {
      try {
        await traffic.settle();
        if (native) {
          const state = await native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence);
          evidence.record("native_runtime_final", { daemon_starts: state.daemonStarts, native_issue_requests: state.nativeIssueRequests });
          expect(state.daemonStarts).toBe(0); expect(native.errors).toEqual([]);
        }
      } finally { if (native) await native.app.close(); else await context.close(); }
    } };
}

export const inboxRow = (page: Page, issue: SnoozeIssue) => page.getByRole("listitem").filter({ has: page.getByRole("button", { name: issue.title, exact: true }) });
export const snoozePicker = (page: Page) => page.getByRole("dialog", { name: /^(Snooze notification|Change time|延后通知|更改时间)$/ });

/** The command palette calls the real ThemeProvider setter and keeps route state intact. */
export async function setSnoozeTheme(session: SnoozeSession, evidence: SnoozeEvidence, theme: "light" | "dark", locale: SnoozeLocale = "en") {
  const observe = () => session.page.evaluate(() => {
    const html = document.documentElement, style = getComputedStyle(html), body = getComputedStyle(document.body);
    return { classes: html.className, dataTheme: html.getAttribute("data-theme"), inlineColorScheme: html.style.colorScheme,
      colorScheme: style.colorScheme, bodyColor: body.color, bodyBackground: body.backgroundColor,
      foregroundToken: style.getPropertyValue("--foreground").trim(), backgroundToken: style.getPropertyValue("--background").trim(),
      surfaceToken: style.getPropertyValue("--surface-raised").trim(), preference: localStorage.getItem("theme") };
  });
  await expect(snoozePicker(session.page)).toHaveCount(0);
  await expect.poll(() => session.page.evaluate(() => !document.activeElement?.closest('[data-slot="popover-content"]'))).toBe(true);
  await session.page.evaluate(() => new Promise<void>((done) => requestAnimationFrame(() => requestAnimationFrame(() => done()))));
  const focus = await session.page.evaluate(() => ({ tag: document.activeElement?.tagName,
    name: document.activeElement?.getAttribute("aria-label"), platform: navigator.platform }));
  evidence.record("theme_command_focus", focus);
  evidence.record("theme_before_product_command", { requested: theme, ...await observe() });
  await session.page.keyboard.press("ControlOrMeta+k");
  const palette = session.page.getByRole("dialog", { name: locale === "en" ? "Search" : "搜索", exact: true });
  await expect(palette).toBeVisible();
  const label = locale === "en" ? `Switch to ${theme === "dark" ? "Dark" : "Light"} Theme` : `切换到${theme === "dark" ? "深" : "浅"}色主题`;
  await palette.getByRole("combobox").fill(label);
  const action = palette.getByRole("option", { name: new RegExp(`^${label}`) });
  await expect(action).toHaveCount(1); await action.click();
  await expect(palette).toHaveCount(0);
  await expect.poll(() => session.page.evaluate((value) => document.documentElement.classList.contains(value) &&
    !document.documentElement.classList.contains(value === "dark" ? "light" : "dark"), theme)).toBe(true);
  const settled = await observe();
  expect(settled.preference).toBe(theme); expect(settled.colorScheme).toContain(theme);
  evidence.record("theme_after_product_command", { requested: theme, ...settled });
}

export async function observeNativeTabs(session: SnoozeSession, fixture: SnoozeFixture) {
  assert(session.native, "Tab persistence belongs to the actual native client");
  const raw: unknown = await session.page.evaluate((slug) => {
    const saved = JSON.parse(localStorage.getItem("multica_tabs") ?? "null");
    const group = saved?.state?.byWorkspace?.[slug];
    if (!group || !Array.isArray(group.tabs)) throw new Error("The native workspace tab group has not been persisted");
    return { active: group.activeTabId, tabs: group.tabs.map((tab: { id: string; url: string }) =>
      ({ id: tab.id, url: tab.url })) };
  }, fixture.workspace.slug);
  const result = object(raw);
  assert(Array.isArray(result.tabs), "The native tab observation must contain its complete tab list");
  const tabs = result.tabs.map((value: unknown) => {
    const tab = object(value);
    return { id: string(tab.id), url: string(tab.url) };
  });
  const active = string(result.active);
  expect(tabs.some((tab) => tab.id === active)).toBe(true);
  fixture.evidence.record("actual_native_persisted_tabs", { active_tab_id: active, workspace_id: fixture.workspace.id, tabs });
  return { active, tabs };
}

export async function uiSnoozeResponse(session: SnoozeSession, fixture: SnoozeFixture, path: string, action: () => Promise<unknown>, status = 200): Promise<Response> {
  const [response] = await Promise.all([session.page.waitForResponse((value) => new URL(value.url()).pathname === path && value.request().method() === "POST"), action()]);
  expect(response.status()).toBe(status);
  const headers = await response.request().allHeaders();
  if (/\/(snooze|unsnooze|snooze\/undo)$/.test(path)) {
    expect(headers["x-workspace-id"]).toBe(fixture.workspace.id);
    expect(headers["x-workspace-slug"]).toBe("");
  } else expect(headers["x-workspace-slug"]).toBe(fixture.workspace.slug);
  if (session.native) expect(Boolean(headers.authorization)).toBe(true);
  else expect({ cookie: Boolean(headers.cookie), csrf: Boolean(headers["x-csrf-token"]) }).toEqual({ cookie: true, csrf: true });
  fixture.evidence.record("actual_UI_write", { path, status: response.status(), actor_id: fixture.actors.owner.id,
    workspace_slug: headers["x-workspace-slug"], workspace_id: headers["x-workspace-id"] ?? null,
    request_body_sha256: digest(response.request().postData() ?? "") });
  return response;
}

export async function openRowSnooze(session: SnoozeSession, issue: SnoozeIssue, change = false) {
  const row = inboxRow(session.page, issue);
  await expect(row).toHaveCount(1);
  await row.hover();
  const trigger = row.getByRole("button", { name: change ? "Change time" : "Snooze", exact: true });
  await trigger.click(); await expect(snoozePicker(session.page)).toBeVisible();
  return trigger;
}

export async function chooseSnoozePreset(session: SnoozeSession, fixture: SnoozeFixture, issue: SnoozeIssue, id: string, change = false) {
  await openRowSnooze(session, issue, change);
  const response = await uiSnoozeResponse(session, fixture, `/api/inbox/${id}/snooze`, () =>
    snoozePicker(session.page).getByRole("button", { name: /^In 1 hour/ }).click());
  const receipt = InboxSnoozeReceiptSchema.parse(await response.json());
  expect(receipt).toMatchObject({ group_kind: "issue", group_id: issue.id, workspace_id: fixture.workspace.id });
  await expect(snoozePicker(session.page)).toHaveCount(0);
  return receipt;
}

/** Time control targets only a persisted plan created through the actual UI. */
export async function makeSnoozeDue(fixture: SnoozeFixture, issue: SnoozeIssue) {
  const client = new pg.Client({ connectionString: fixture.allocation.databaseUrl, connectionTimeoutMillis: 3000 });
  try {
    await client.connect();
    const identity = await client.query<{ name: string }>("SELECT current_database() AS name");
    assert.equal(identity.rows[0]?.name, fixture.allocation.database);
    const result = await client.query<{ snoozed_until: Date }>(`
      UPDATE inbox_snooze SET snoozed_until = clock_timestamp() - interval '1 second'
      WHERE workspace_id = $1 AND recipient_type = 'member' AND recipient_id = $2
        AND group_kind = 'issue' AND group_id = $3 AND snoozed_until > clock_timestamp()
      RETURNING snoozed_until`, [fixture.workspace.id, fixture.actors.owner.id, issue.id]);
    expect(result.rowCount).toBe(1);
    fixture.evidence.record("owned_exact_plan_time_control", { issue_id: issue.id, actor_id: fixture.actors.owner.id,
      workspace_id: fixture.workspace.id, database: identity.rows[0]?.name, due_at: result.rows[0]?.snoozed_until.toISOString() });
  } catch (error) {
    fixture.evidence.record("time_control_failed", { error_type: error instanceof Error ? error.name : "unknown" });
    throw new Error("Owned exact-plan time control failed");
  } finally { await client.end(); }
}

export async function snoozeStateFact(fixture: SnoozeFixture, issue: SnoozeIssue) {
  const client = new pg.Client({ connectionString: fixture.allocation.databaseUrl, connectionTimeoutMillis: 3000 });
  try {
    await client.connect();
    const identity = await client.query<{ name: string }>("SELECT current_database() AS name");
    assert.equal(identity.rows[0]?.name, fixture.allocation.database);
    const result = await client.query<{ revision: string; snoozed_until: Date | null; reminded_at: Date | null; reminder_reason: string | null }>(`
      SELECT revision::text, snoozed_until, reminded_at, reminder_reason FROM inbox_snooze
      WHERE workspace_id = $1 AND recipient_type = 'member' AND recipient_id = $2 AND group_kind = 'issue' AND group_id = $3`,
    [fixture.workspace.id, fixture.actors.owner.id, issue.id]);
    expect(result.rows).toHaveLength(1);
    const row = result.rows[0]!;
    const fact = { issue_id: issue.id, workspace_id: fixture.workspace.id, actor_id: fixture.actors.owner.id, revision: row.revision,
      snoozed_until: row.snoozed_until?.toISOString() ?? null, reminded_at: row.reminded_at?.toISOString() ?? null, reminder_reason: row.reminder_reason };
    fixture.evidence.record("owned_persisted_plan_projection", fact);
    return fact;
  } catch {
    throw new Error("Owned exact-plan observation failed");
  } finally { await client.end(); }
}

export async function closeSnoozeJourney(fixture: SnoozeFixture, sessions: SnoozeSession[]) {
  const outcomes = await Promise.allSettled(sessions.map((session) => session.close()));
  fixture.evidence.record("client_cleanup", { outcomes: outcomes.map((result) => ({ status: result.status,
    error_type: result.status === "rejected" ? (result.reason instanceof Error ? result.reason.name : "unknown") : null })) });
  await fixture.evidence.attach();
  const failed = outcomes.find((result) => result.status === "rejected");
  if (failed?.status === "rejected") throw failed.reason;
}
