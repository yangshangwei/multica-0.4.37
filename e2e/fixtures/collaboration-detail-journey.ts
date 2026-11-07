import { execFileSync } from "node:child_process";
import { createHash, randomUUID } from "node:crypto";
import { mkdtempSync, readFileSync, realpathSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { isAbsolute, join, relative, resolve } from "node:path";
import pg from "pg";
import {
  expect, type Browser, type Frame, type Locator, type Page, type Request, type Route, type TestInfo,
} from "@playwright/test";
import {
  CollaborationCommentReceiptSchema, CollaborationCommentSchema, CollaborationDeliveryHistorySchema,
  CollaborationIssueSchema, CollaborationOperationSchema, CollaborationStatusOptionsSchema,
  CollaborationTriggerPreviewSchema,
  type CollaborationIssue, type CollaborationStatusAction, type CollaborationSubmission,
} from "@multica/core/collaboration";
import { accountPath as collaborationAccountPath } from "@multica/core/paths";
import { TestApiClient } from "../fixtures";
import { gapWorkflowObservations } from "./collaboration-control-gaps";
import { selectHandoffRecipient } from "./collaboration-offline-handoff";
import { chooseView } from "./collaboration-source-surface";
import { deepLink, nativeScreenshot, openDesktop, selectDesktopWorkspace, type NativeHarness } from "./collaboration-native";

const checkout = realpathSync(resolve(import.meta.dirname, "../.."));
const issuePath = (id: string) => `/api/collaboration/issues/${id}`;
const accountPath = (id: string) => collaborationAccountPath({ view: "participating", issueId: id });
const sha256 = (bytes: string | Buffer) => createHash("sha256").update(bytes).digest("hex");

export type DetailJourneyPlatform = "web" | "desktop";
export type DetailJourneyLocale = "en" | "zh-Hans";

interface ServiceProof {
  listener_pid: number;
  commit: string;
  source_id: string;
  command: string;
}

export interface DetailJourneyProvenance {
  task: string;
  source_root: string;
  api_url: string;
  production_web_url: string;
  renderer_url: string;
  database: string;
  api: ServiceProof;
  web: ServiceProof & { mode: "production"; build_id: string };
  renderer: ServiceProof & { mode: "production"; artifact_hashes: Record<string, string> };
  source_hashes: Record<string, string>;
}

function object(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== "object" || Array.isArray(value)) throw new Error("Invalid detail journey record");
  return value as Record<string, unknown>;
}

function textField(value: Record<string, unknown>, key: string): string {
  const result = value[key];
  if (typeof result !== "string" || result.trim() === "") throw new Error("Incomplete detail journey provenance");
  return result;
}

function origin(value: string): string {
  let url: URL;
  try { url = new URL(value); } catch { throw new Error("Invalid detail journey endpoint"); }
  if (!["http:", "https:"].includes(url.protocol) || !["localhost", "127.0.0.1", "[::1]"].includes(url.hostname)
    || url.username || url.password || url.search || url.hash || url.pathname !== "/" || url.port === "0") {
    throw new Error("Detail journey requires credential-free loopback origins");
  }
  return url.origin;
}

function serviceProof(value: unknown): ServiceProof {
  const row = object(value);
  const listener = row.listener_pid;
  const commit = textField(row, "commit");
  const source = textField(row, "source_id");
  const command = textField(row, "command");
  if (!Number.isSafeInteger(listener) || typeof listener !== "number" || listener <= 0
    || !/^[a-f0-9]{7,40}$/i.test(commit) || !/^[a-f0-9]{64}$/i.test(source)
    || /Bearer\s|postgres(?:ql)?:\/\/|(?:PASSWORD|TOKEN)=\S/i.test(command)) {
    throw new Error("Invalid or credential-bearing detail journey launch identity");
  }
  return { listener_pid: listener, commit, source_id: source, command };
}

function checkedHashes(value: unknown): Record<string, string> {
  const files = object(value);
  const result: Record<string, string> = {};
  for (const [path, digest] of Object.entries(files)) {
    const candidate = resolve(checkout, path);
    if (isAbsolute(path) || relative(checkout, candidate).startsWith("..") || typeof digest !== "string"
      || !/^[a-f0-9]{64}$/i.test(digest)) throw new Error("Invalid detail journey source or artifact identity");
    if (sha256(readFileSync(candidate)) !== digest) throw new Error("Detail journey source or artifact changed after build");
    result[path] = digest;
  }
  if (Object.keys(result).length === 0) throw new Error("Detail journey requires pinned source and artifact bytes");
  return result;
}

function assertListener(url: string, expected: number): void {
  const port = new URL(url).port || (url.startsWith("https:") ? "443" : "80");
  let listeners: string;
  try {
    listeners = execFileSync("lsof", ["-nP", "-t", `-iTCP:${port}`, "-sTCP:LISTEN"], { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
  } catch { throw new Error("The recorded detail journey listener is unavailable"); }
  const actual = [...new Set(listeners.trim().split(/\s+/).map(Number))];
  if (actual.length !== 1 || actual[0] !== expected) throw new Error("Detail journey listener ownership does not match");
}

/** The root session supplies this permit after building and proving its listeners.
 * A configured mismatch fails before TestApiClient's login can touch a database. */
export async function detailJourneyProvenance(): Promise<DetailJourneyProvenance> {
  const filename = process.env.COLLABORATION_DETAIL_PROVENANCE_FILE;
  if (!filename || !isAbsolute(filename)) throw new Error("Explicit detail journey runtime provenance is required");
  const task = process.env.COLLABORATION_DETAIL_TASK;
  const allocationFilename = process.env.COLLABORATION_DETAIL_ALLOCATION_FILE;
  if (!task || !/^\.trellis\/tasks\/[a-zA-Z0-9][a-zA-Z0-9._-]*$/.test(task)
    || !allocationFilename || !isAbsolute(allocationFilename)) {
    throw new Error("Explicit current detail journey task and allocation are required");
  }
  const raw = object(JSON.parse(readFileSync(filename, "utf8")));
  if (raw.runtime_permitted !== true || raw.task !== task || realpathSync(textField(raw, "source_root")) !== checkout) {
    throw new Error("Detail journey runtime permit does not match this task and checkout");
  }
  const allocationBytes = readFileSync(allocationFilename);
  if (realpathSync(textField(raw, "allocation_file")) !== realpathSync(allocationFilename)
    || raw.allocation_sha256 !== sha256(allocationBytes)) {
    throw new Error("Detail journey allocation differs from the permitted record");
  }
  const allocation = object(JSON.parse(allocationBytes.toString("utf8")));
  if (allocation.task !== task || realpathSync(textField(allocation, "source_root")) !== checkout) {
    throw new Error("Detail journey allocation does not belong to this task and checkout");
  }
  const apiUrl = origin(textField(raw, "api_url"));
  const webUrl = origin(textField(raw, "production_web_url"));
  const rendererUrl = origin(textField(raw, "renderer_url"));
  const database = textField(raw, "database");
  for (const [actual, allocated] of [[apiUrl, "api_origin"], [webUrl, "web_origin"], [rendererUrl, "renderer_origin"], [database, "database"]]) {
    if (actual !== allocation[allocated!]) throw new Error("Detail journey services differ from the recorded task allocation");
  }
  if (!process.env.COLLABORATION_ELECTRON_SOURCE_ROOT || !isAbsolute(process.env.COLLABORATION_ELECTRON_SOURCE_ROOT)
    || origin(process.env.COLLABORATION_E2E_API_URL ?? "") !== apiUrl || origin(process.env.NEXT_PUBLIC_API_URL ?? "") !== apiUrl
    || origin(process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN ?? "") !== webUrl
    || origin(process.env.COLLABORATION_ELECTRON_RENDERER_URL ?? "") !== rendererUrl
    || realpathSync(process.env.COLLABORATION_ELECTRON_SOURCE_ROOT ?? "") !== checkout
    || !process.env.MULTICA_DEV_VERIFICATION_CODE?.trim()) throw new Error("Detail journey environment differs from its permit");
  let dbUrl: URL;
  try { dbUrl = new URL(process.env.DATABASE_URL ?? ""); } catch { throw new Error("Explicit detail journey database configuration is required"); }
  if (!["postgres:", "postgresql:"].includes(dbUrl.protocol) || !["localhost", "127.0.0.1", "[::1]"].includes(dbUrl.hostname)
    || decodeURIComponent(dbUrl.pathname.slice(1)) !== database || ["multica", "postgres", "template0", "template1"].includes(database)) {
    throw new Error("Detail journey database differs from the explicit allocation");
  }
  const api = serviceProof(raw.api);
  const web = serviceProof(raw.web);
  const renderer = serviceProof(raw.renderer);
  const webRecord = object(raw.web);
  const rendererRecord = object(raw.renderer);
  if (webRecord.mode !== "production" || rendererRecord.mode !== "production") throw new Error("Detail journey requires production client artifacts");
  const buildId = textField(webRecord, "build_id");
  if (readFileSync(join(checkout, "apps/web/.next/BUILD_ID"), "utf8").trim() !== buildId) throw new Error("Detail journey Web build identity changed");
  const sourceHashes = checkedHashes(raw.source_hashes);
  for (const file of ["server/internal/service/collaboration_delivery.go", "packages/core/collaboration/delivery-schema.ts",
    "packages/views/collaboration/collaboration-detail.tsx", "packages/views/collaboration/workflow.tsx", "packages/views/collaboration/comment-composer.tsx"]) {
    if (!sourceHashes[file]) throw new Error("Detail journey requires the current product source identities");
  }
  const artifactHashes = checkedHashes(rendererRecord.artifact_hashes);
  for (const file of ["apps/desktop/out/renderer/index.html", "apps/desktop/out/preload/index.js"]) {
    if (!artifactHashes[file]) throw new Error("Detail journey requires the exact renderer and preload artifacts");
  }
  const commit = execFileSync("git", ["rev-parse", "HEAD"], { cwd: checkout, encoding: "utf8" }).trim();
  for (const proof of [api, web, renderer]) {
    if (!commit.startsWith(proof.commit)) throw new Error("Detail journey listener commit differs from the checkout");
  }
  assertListener(apiUrl, api.listener_pid);
  assertListener(webUrl, web.listener_pid);
  assertListener(rendererUrl, renderer.listener_pid);
  const [healthResponse, configResponse, rendererResponse, webResponse] = await Promise.all([
    fetch(`${apiUrl}/health`), fetch(`${apiUrl}/api/config`), fetch(rendererUrl), fetch(`${webUrl}/login`),
  ]);
  if (![healthResponse, configResponse, rendererResponse, webResponse].every((response) => response.status === 200)) {
    throw new Error("Detail journey services are not ready");
  }
  const health = object(await healthResponse.json());
  const config = object(await configResponse.json());
  if (health.pid !== api.listener_pid || health.commit !== api.commit || config.auth_mode !== "legacy") {
    throw new Error("Detail journey API launch or authentication identity does not match");
  }
  if (sha256(Buffer.from(await rendererResponse.arrayBuffer())) !== artifactHashes["apps/desktop/out/renderer/index.html"]) {
    throw new Error("The native listener is not serving the pinned renderer");
  }
  const webHtml = await webResponse.text();
  if (!webHtml.includes('apiBaseUrl\\\":\\\"' + apiUrl) || !webHtml.includes('wsUrl\\\":\\\"' + apiUrl.replace(/^http/, "ws") + "/ws")) {
    throw new Error("The production Web runtime points at different API or WebSocket services");
  }
  const db = new pg.Client({ connectionString: process.env.DATABASE_URL, options: "-c default_transaction_read_only=on -c statement_timeout=5000" });
  await db.connect();
  try {
    const current = await db.query<{ database: string; creation_enabled: boolean; enabled: boolean; host_execution_enabled: boolean }>(
      "SELECT current_database() AS database, creation_enabled, enabled, host_execution_enabled FROM collaboration_execution_gate",
    );
    if (current.rows.length !== 1 || current.rows[0]?.database !== database || current.rows[0]?.creation_enabled !== true
      || current.rows[0]?.enabled !== false || current.rows[0]?.host_execution_enabled !== false) {
      throw new Error("Detail journey database or manual-only deployment gates do not match");
    }
  } finally { await db.end(); }
  return { task, source_root: checkout, api_url: apiUrl, production_web_url: webUrl, renderer_url: rendererUrl, database, api,
    web: { ...web, mode: "production", build_id: buildId }, renderer: { ...renderer, mode: "production", artifact_hashes: artifactHashes }, source_hashes: sourceHashes };
}

async function journeyActor(provenance: DetailJourneyProvenance, suffix: string, role: string) {
  const client = new TestApiClient();
  const email = `detail-${role}-${suffix}@example.invalid`;
  const name = `Detail ${role} ${suffix}`;
  await client.login(email, name);
  await client.markUserOnboarded();
  const user = await client.requestJSON<{ id: string; name: string }>("/api/me");
  expect(user.name).toBe(name);
  return { client, id: user.id, name, async authenticateWeb(page: Page) {
    const sent = await page.request.post(`${provenance.api_url}/auth/send-code`, { data: { email } });
    expect(sent.status()).toBe(200);
    const verified = await page.request.post(`${provenance.api_url}/auth/verify-code`, { data: { email, code: process.env.MULTICA_DEV_VERIFICATION_CODE } });
    expect(verified.status()).toBe(200);
    const response = object(await verified.json());
    expect(object(response.user).id).toBe(user.id);
    const cookies = await page.context().cookies(provenance.api_url);
    expect(cookies.some((cookie) => cookie.name === "multica_auth" && cookie.httpOnly && cookie.value.length > 0)).toBe(true);
    expect(cookies.some((cookie) => cookie.name === "multica_csrf" && !cookie.httpOnly && cookie.value.length > 0)).toBe(true);
  } };
}
export type DetailJourneyActor = Awaited<ReturnType<typeof journeyActor>>;

export interface OrdinaryJourneyIssue { id: string; identifier: string; title: string; status: string; revision: number; stage: number | null }

export async function seedDetailJourney() {
  const provenance = await detailJourneyProvenance();
  const suffix = randomUUID().replaceAll("-", "").slice(0, 12);
  const creator = await journeyActor(provenance, suffix, "initiator");
  const handler = await journeyActor(provenance, suffix, "external-handler");
  expect(await handler.client.getWorkspaces()).toEqual([]);
  const workspace = await creator.client.ensureWorkspace("Detail journey source", `detail-${suffix}`);
  expect(workspace.slug).toBe(`detail-${suffix}`);
  expect(await creator.client.requestJSON("/api/collaboration/creation-options")).toMatchObject({ creation_enabled: true });
  const parent = await creator.client.requestJSON<OrdinaryJourneyIssue>("/api/issues", { method: "POST", body: {
    title: `Ordinary parent ${suffix}`, status: "in_progress", assignee_type: "member", assignee_id: creator.id,
  } });
  const requirements = Array.from({ length: 24 }, (_, index) => `Requirement ${index + 1}: preserve the original task identity, explain the requested change, and provide a reproducible outcome with supporting evidence. 长内容仍应换行并保留可读的顺序。`).join("\n\n");
  const create = async (title: string, handlerId: string, description: string, parentId?: string) => {
    const requestId = randomUUID();
    const input = { source_workspace_id: workspace.id, handler_user_id: handlerId, ai_policy: "manual", title, description,
      attachment_ids: [], ...(parentId ? { parent_issue_id: parentId } : {}) };
    const issue = CollaborationIssueSchema.parse(await creator.client.requestJSON("/api/collaboration/issues", {
      method: "POST", body: input, headers: { "Idempotency-Key": requestId },
    }));
    expect(issue).toMatchObject({ ai_policy: "manual", source_workspace: { id: workspace.id }, creator: { user_id: creator.id }, handler: { user_id: handlerId } });
    return { issue, requestId, input };
  };
  const shared = await create(`Manual delivery journey ${suffix}: preserve earlier results through revision and complete the same source task after exact acceptance · 跨空间协作与长标题`, handler.id, requirements, parent.id);
  const self = await create(`Self delivery journey ${suffix}`, creator.id, "Record a text-only result and its basis, then complete without a self-transfer.");
  return { provenance, creator, handler, workspace, parent, shared, self, requirements };
}
export type DetailJourneyFixture = Awaited<ReturnType<typeof seedDetailJourney>>;

export const readJourneyIssue = async (actor: DetailJourneyActor, id: string) => CollaborationIssueSchema.parse(await actor.client.requestJSON(issuePath(id)));
export const readJourneyHistory = async (actor: DetailJourneyActor, id: string) => CollaborationDeliveryHistorySchema.parse(await actor.client.requestJSON(`${issuePath(id)}/acceptance-submissions?limit=100`));
export const readJourneyComment = async (actor: DetailJourneyActor, id: string, commentId: string) => CollaborationCommentSchema.parse(await actor.client.requestJSON(`${issuePath(id)}/comments/${commentId}`));

export function journeyCopy(locale: DetailJourneyLocale) {
  const collaboration = object(JSON.parse(readFileSync(join(checkout, `packages/views/locales/${locale}/collaboration.json`), "utf8")));
  const layout = object(JSON.parse(readFileSync(join(checkout, `packages/views/locales/${locale}/layout.json`), "utf8")));
  const read = (group: string, key: string) => textField(object(collaboration[group]), key);
  return {
    actions: { change_status: read("delivery", "start"), request_information: read("workflow", "requestInformation"),
      return_information: read("workflow", "returnInformation"), submit_acceptance: read("delivery", "submit"),
      return_acceptance: read("delivery", "return"), complete: read("delivery", "accept"), reopen: read("workflow", "reopen") },
    selfComplete: read("delivery", "selfComplete"), workflow: read("workflow", "title"), more: read("delivery", "more"),
    confirm: read("workflow", "confirm"), close: read("workflow", "dismiss"), status: read("detail", "status"), note: read("workflow", "note"),
    recipient: read("workflow", "recipient"), selected: read("workflow", "confirmRecipient"), requestedBy: read("workflow", "requestedBy"),
    result: read("delivery", "result"), basis: read("delivery", "basis"), revisionReason: read("delivery", "revisionReason"),
    author: read("delivery", "author"), submittedAt: read("delivery", "submittedAt"), decisionBy: read("delivery", "decidedBy"),
    decisionAt: read("delivery", "decisionAt"), returned: textField(object(object(collaboration.delivery).decisions), "returned"),
    accepted: textField(object(object(collaboration.delivery).decisions), "accepted"), history: read("delivery", "history"),
    sourceComment: read("delivery", "sourceComment"), useComment: read("delivery", "useComment"), delivery: read("delivery", "title"),
    requirements: read("detail", "description"), comments: read("detail", "comments"), commentLabel: read("composer", "label"),
    publish: read("composer", "send"), preview: read("composer", "preview"), commentConfirmed: read("composer", "sent"),
    editComment: read("commentActions", "edit"), editText: read("commentActions", "editText"), saveComment: read("commentActions", "save"),
    deleteComment: read("commentActions", "delete"), confirmDelete: read("commentActions", "confirmDelete"), deleted: read("detail", "deleted"),
    unknown: read("workflow", "unknown"), recover: read("workflow", "recover"), manual: read("policy", "manual"),
    ai: read("execution", "title"), external: read("detail", "external"), handler: read("detail", "handler"),
    context: read("detail", "context"), permissionContext: read("participants", "permissionContext"),
    navigation: textField(object(layout.global_navigation), "label"), workspaces: textField(object(layout.sidebar), "workspaces_label"),
    collaboration: read("page", "title"), participants: read("participants", "title"), refresh: read("page", "refresh"),
  };
}

export interface DetailJourneySession {
  page: Page;
  platform: DetailJourneyPlatform;
  locale: DetailJourneyLocale;
  native?: Awaited<ReturnType<typeof openDesktop>>;
  errors: string[];
  openAccount(id: string): Promise<void>;
  openSource(id: string, title: string): Promise<void>;
  close(): Promise<void>;
}

export async function openJourneySession(browser: Browser, fixture: DetailJourneyFixture, platform: DetailJourneyPlatform,
  actor: DetailJourneyActor, id = fixture.shared.issue.id): Promise<DetailJourneySession> {
  if (platform === "web") {
    const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
    try {
      const page = await context.newPage();
      const errors: string[] = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await actor.authenticateWeb(page);
      await page.addInitScript(() => {
        if (!localStorage.getItem("multica-locale")) localStorage.setItem("multica-locale", "en");
        localStorage.setItem("multica:chat:isOpen", "false");
      });
      const navigate = async (path: string) => {
        if (page.url() === "about:blank") {
          await page.goto(`${fixture.provenance.production_web_url}${path}`);
        } else {
          expect(new URL(page.url()).origin).toBe(fixture.provenance.production_web_url);
          // Shared content uses this production bridge for in-app links.
          // A full document navigation would discard tab-lifetime scope history.
          await page.evaluate((target) => window.dispatchEvent(new CustomEvent("multica:navigate", { detail: { path: target } })), path);
        }
      };
      const session: DetailJourneySession = { page, platform, locale: "en", errors,
        openAccount: async (issueId) => {
          await navigate(accountPath(issueId));
          await expect(page).toHaveURL(`${fixture.provenance.production_web_url}${accountPath(issueId)}`);
        },
        openSource: async (issueId) => {
          const source = await actor.client.requestJSON<OrdinaryJourneyIssue>(`/api/issues/${issueId}`);
          await navigate(`/${fixture.workspace.slug}/issues/${issueId}`);
          await expect(page).toHaveURL(`${fixture.provenance.production_web_url}/${fixture.workspace.slug}/issues/${source.identifier}`);
        }, close: () => context.close() };
      await session.openAccount(id);
      return session;
    } catch (error) { await context.close(); throw error; }
  }
  const profile = mkdtempSync(join(tmpdir(), "multica-detail-journey-"));
  let native: Awaited<ReturnType<typeof openDesktop>> | undefined;
  try {
    native = await openDesktop(profile, { token: actor.client.getToken()!, deepLink: `multica://account${accountPath(id).slice("/account".length)}` });
    await native.start();
    const owned = native;
    const session: DetailJourneySession = { page: owned.page, platform, locale: "en", native: owned, errors: owned.errors,
      openAccount: (issueId) => deepLink(owned.app, accountPath(issueId)),
      openSource: async (issueId, title) => {
        await selectDesktopWorkspace(owned.page, fixture.workspace);
        await owned.page.getByRole("link", { name: /^(Issues|Workspace issues)$/, exact: true }).and(owned.page.locator('[data-sidebar="menu-button"]')).click();
        await chooseView(owned.page, "List");
        const link = owned.page.locator("a").filter({ hasText: title }).first();
        const source = await actor.client.requestJSON<OrdinaryJourneyIssue>(`/api/issues/${issueId}`);
        const href = await link.getAttribute("href");
        if (!href) throw new Error("Native source issue entry has no task link");
        expect([`/${fixture.workspace.slug}/issues/${issueId}`, `/${fixture.workspace.slug}/issues/${source.identifier}`]).toContain(new URL(href, fixture.provenance.api_url).pathname);
        await link.click();
        await expect(owned.page.getByRole("heading", { name: title, exact: true })).toBeVisible();
        await expect.poll(() => owned.page.evaluate(({ slug, path }) => {
          const state = JSON.parse(localStorage.getItem("multica_tabs") ?? "null")?.state;
          const group = state?.byWorkspace?.[slug];
          return state?.activeScope === "workspace" && state?.activeWorkspaceSlug === slug
            && group?.tabs.find((tab: { id: string; url: string }) => tab.id === group.activeTabId)?.url === path;
        }, { slug: fixture.workspace.slug, path: `/${fixture.workspace.slug}/issues/${source.identifier}` })).toBe(true);
      }, close: async () => {
        try { await owned.app.close(); } finally { rmSync(profile, { recursive: true, force: true }); }
      } };
    return session;
  } catch (error) {
    try { if (native) await native.app.close(); } finally { rmSync(profile, { recursive: true, force: true }); }
    throw error;
  }
}

/** Parse the response the newly loaded renderer actually consumed. This follows
 * refreshDirectedIssue's navigation fence without its extra cookie-only GET. */
export async function refreshJourneyIssue(session: DetailJourneySession, id: string): Promise<CollaborationIssue> {
  const { page } = session;
  let navigated = false;
  const requests = new WeakSet<Request>();
  const navigation = (frame: Frame) => { if (frame === page.mainFrame()) navigated = true; };
  const request = (entry: Request) => {
    if (navigated && entry.method() === "GET" && new URL(entry.url()).pathname === issuePath(id)) requests.add(entry);
  };
  page.on("framenavigated", navigation);
  page.on("request", request);
  try {
    const fetched = page.waitForResponse((response) => requests.has(response.request()) && response.status() === 200);
    await page.reload();
    const issue = CollaborationIssueSchema.parse(await (await fetched).json());
    expect(issue.id).toBe(id);
    await expect(page.getByRole("heading", { name: issue.title, exact: true })).toBeVisible();
    return issue;
  } finally { page.off("framenavigated", navigation); page.off("request", request); }
}

/** Use the existing issue-scoped refresh control without discarding the
 * production browser-tab scope history that the next assertion resumes. */
export async function refreshJourneyIssueInApp(session: DetailJourneySession, id: string): Promise<CollaborationIssue> {
  const { page } = session;
  const copy = journeyCopy(session.locale);
  const participants = page.getByRole("button", { name: copy.participants, exact: true });
  await expect(participants).toBeVisible();
  if (await participants.getAttribute("aria-expanded") !== "true") await participants.click();
  const contentId = await participants.getAttribute("aria-controls");
  if (!contentId) throw new Error("The participants refresh has no current panel");
  const panel = page.locator(`[id="${contentId}"]`);
  const refresh = panel.getByRole("button", { name: copy.refresh, exact: true });
  await expect(refresh).toBeEnabled();
  const requests = new WeakSet<Request>();
  const observe = (request: Request) => {
    if (request.method() === "GET" && new URL(request.url()).pathname === issuePath(id)) requests.add(request);
  };
  page.on("request", observe);
  try {
    const pending = page.waitForResponse((response) => requests.has(response.request()));
    await refresh.click();
    const response = await pending;
    expect(response.status()).toBe(200);
    const issue = CollaborationIssueSchema.parse(await response.json());
    expect(issue.id).toBe(id);
    await expect(page.getByRole("heading", { name: issue.title, exact: true })).toBeVisible();
    await participants.click();
    await expect(participants).toHaveAttribute("aria-expanded", "false");
    return issue;
  } finally { page.off("request", observe); }
}

type JourneyAction = Exclude<CollaborationStatusAction, "cancel">;
export interface JourneyActionForm {
  session: DetailJourneySession;
  baseline: CollaborationIssue;
  form: Locator;
  action: JourneyAction;
  label: string;
  options: ReturnType<typeof CollaborationStatusOptionsSchema.parse>;
}

export async function openJourneyAction(session: DetailJourneySession, id: string, action: JourneyAction,
  settings: { self?: boolean; commentId?: string } = {}): Promise<JourneyActionForm> {
  const baseline = await refreshJourneyIssue(session, id);
  const copy = journeyCopy(session.locale);
  const label = settings.self ? copy.selfComplete : copy.actions[action];
  const workflow = session.page.getByRole("region", { name: copy.workflow, exact: true });
  const optionsPending = session.page.waitForResponse((response) => new URL(response.url()).pathname === `${issuePath(id)}/status-options`
    && new URL(response.url()).searchParams.get("action") === action && response.request().method() === "GET");
  if (settings.commentId) {
    const button = session.page.locator(`#comment-${settings.commentId}`).getByRole("button", { name: copy.useComment, exact: true });
    await button.focus();
    await button.press("Enter");
  } else {
    let trigger = workflow.getByRole("button", { name: label, exact: true });
    if (!await trigger.isVisible()) {
      await workflow.getByRole("button", { name: copy.more, exact: true }).click();
      trigger = session.page.getByRole("menuitem", { name: label, exact: true });
    }
    await trigger.focus();
    await expect(trigger).toBeFocused();
    await trigger.press("Enter");
  }
  const optionsResponse = await optionsPending;
  expect(optionsResponse.status()).toBe(200);
  const options = CollaborationStatusOptionsSchema.parse(await optionsResponse.json());
  expect(options.action).toBe(action);
  const form = session.page.getByRole("form", { name: label, exact: true });
  await expect(form).toBeVisible();
  const status = form.getByLabel(copy.status, { exact: true });
  const selected = action === "change_status" ? options.options.find((option) => option.category === "in_progress")?.key : options.default_status;
  if (!selected) throw new Error("Journey action has no offered status");
  await status.selectOption(selected);
  await expect(status).toHaveValue(selected);
  return { session, baseline, form, action, label, options };
}

export async function chooseJourneyRecipient(opened: JourneyActionForm, actor: DetailJourneyActor) {
  if (opened.session.locale !== "en") throw new Error("The existing exact-recipient helper uses the English form");
  await selectHandoffRecipient(opened.session.page, opened.form, { id: actor.id, name: actor.name });
}

function actionEndpoint(id: string, action: JourneyAction): string {
  switch (action) {
    case "change_status": return issuePath(id);
    case "request_information": return `${issuePath(id)}/handoffs`;
    case "return_information": return `${issuePath(id)}/information-returns`;
    case "submit_acceptance": return `${issuePath(id)}/acceptance-submissions`;
    case "return_acceptance": return `${issuePath(id)}/acceptance-returns`;
    case "complete": case "reopen": return `${issuePath(id)}/lifecycle`;
  }
}

function journeyAuthentication(request: Request, session: DetailJourneySession) {
  const headers = request.headers();
  const evidence = { bearer: Boolean(headers.authorization), csrf: Boolean(headers["x-csrf-token"]) };
  if (session.platform === "web") {
    expect(evidence.bearer).toBe(false);
    expect(evidence.csrf).toBe(true);
  }
  return evidence;
}

export async function submitJourneyAction(opened: JourneyActionForm) {
  const { session, form, action, baseline } = opened;
  const copy = journeyCopy(session.locale);
  const endpoint = actionEndpoint(baseline.id, action);
  const method = action === "change_status" ? "PATCH" : "POST";
  const responsePending = session.page.waitForResponse((response) => new URL(response.url()).pathname === endpoint && response.request().method() === method);
  const label = ["change_status", "submit_acceptance", "return_acceptance", "complete"].includes(action) ? opened.label : copy.confirm;
  const button = form.getByRole("button", { name: label, exact: true });
  await expect(button).toBeEnabled();
  await button.focus();
  await button.press("Enter");
  const response = await responsePending;
  expect(response.status()).toBe(200);
  const receipt = CollaborationOperationSchema.parse(await response.json());
  const input: Record<string, unknown> = response.request().postDataJSON();
  const requestId = response.request().headers()["idempotency-key"];
  if (!requestId) throw new Error("The workflow omitted its original request identity");
  expect(requestId).toMatch(/^[a-f0-9-]{36}$/i);
  expect(input).toMatchObject({ ...gapWorkflowObservations(baseline), status_options_version: opened.options.status_options_version });
  await expect(form).toHaveCount(0);
  return { receipt, input, requestId, status: response.status(), authentication: journeyAuthentication(response.request(), session) };
}

export async function publishJourneyComment(session: DetailJourneySession, id: string, content: string) {
  const copy = journeyCopy(session.locale);
  const form = session.page.getByRole("form", { name: copy.comments, exact: true });
  const textarea = form.getByLabel(copy.commentLabel, { exact: true });
  await textarea.fill(content);
  await expect(textarea).toHaveValue(content);
  const writes: { path: string; method: string }[] = [];
  const observer = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (path.startsWith(issuePath(id)) && request.method() !== "GET") writes.push({ path, method: request.method() });
  };
  session.page.on("request", observer);
  try {
    const previewPending = session.page.waitForResponse((response) => new URL(response.url()).pathname === `${issuePath(id)}/comments/trigger-preview` && response.request().method() === "POST");
    const commentPending = session.page.waitForResponse((response) => new URL(response.url()).pathname === `${issuePath(id)}/comments` && response.request().method() === "POST");
    const publish = form.getByRole("button", { name: copy.publish, exact: true });
    await expect(publish).toBeEnabled();
    await publish.focus();
    await publish.press("Enter");
    const previewResponse = await previewPending;
    expect(previewResponse.status()).toBe(200);
    const preview = CollaborationTriggerPreviewSchema.parse(await previewResponse.json());
    const response = await commentPending;
    expect(response.status()).toBe(201);
    const receipt = CollaborationCommentReceiptSchema.parse(await response.json());
    expect(response.request().postDataJSON()).toMatchObject({ content });
    const requestId = response.request().headers()["idempotency-key"];
    if (!requestId) throw new Error("The composer omitted its original request identity");
    expect(requestId).toMatch(/^[a-f0-9-]{36}$/i);
    await expect(form.getByRole("status").filter({ hasText: copy.commentConfirmed })).toBeVisible();
    await expect(textarea).toHaveValue("");
    const input: Record<string, unknown> = response.request().postDataJSON();
    return { receipt, preview, requestId, writes, input, authentication: journeyAuthentication(response.request(), session) };
  } finally { session.page.off("request", observer); }
}

/** Lose only the response to a real committed submission. Recovery is the
 * original GET; the request observer proves reload did not submit it again. */
export async function submitJourneyWithLostResponse(opened: JourneyActionForm) {
  if (opened.action !== "submit_acceptance") throw new Error("The lost-response case is a formal result submission");
  const { page } = opened.session;
  const copy = journeyCopy(opened.session.locale);
  const endpoint = actionEndpoint(opened.baseline.id, opened.action);
  const matches = (url: URL) => url.pathname === endpoint;
  const posts: Request[] = [];
  const observe = (request: Request) => {
    if (new URL(request.url()).pathname === endpoint && request.method() === "POST") posts.push(request);
  };
  type Captured = Awaited<ReturnType<typeof submitJourneyAction>>;
  let resolveCapture!: (value: Captured) => void;
  let rejectCapture!: (reason: unknown) => void;
  const captured = new Promise<Captured>((done, fail) => { resolveCapture = done; rejectCapture = fail; });
  const lose = async (route: Route) => {
    if (route.request().method() !== "POST") { await route.continue(); return; }
    try {
      const response = await route.fetch();
      expect(response.status()).toBe(200);
      const receipt = CollaborationOperationSchema.parse(await response.json());
      const requestId = route.request().headers()["idempotency-key"];
      if (!requestId) throw new Error("The result submission omitted its original request identity");
      const input: Record<string, unknown> = route.request().postDataJSON();
      // Finish the active route before unregistering it, preserving the real
      // transport failure and the server's committed original receipt.
      await route.abort("failed");
      resolveCapture({ receipt, input, requestId, status: response.status(), authentication: journeyAuthentication(route.request(), opened.session) });
    } catch (error) { await route.abort("failed").catch(() => undefined); rejectCapture(error); }
  };
  page.on("request", observe);
  await page.route(matches, lose);
  try {
    const button = opened.form.getByRole("button", { name: opened.label, exact: true });
    await expect(button).toBeEnabled();
    await button.focus();
    await button.press("Enter");
    const original = await captured;
    expect(original.input).toMatchObject({ ...gapWorkflowObservations(opened.baseline), status_options_version: opened.options.status_options_version });
    await expect(page.getByText(copy.unknown, { exact: true })).toBeVisible();
    await page.unroute(matches, lose);
    const afterReload = await refreshJourneyIssue(opened.session, opened.baseline.id);
    const recover = page.getByRole("button", { name: copy.recover, exact: true });
    await expect(recover).toBeVisible();
    const recoveredResponse = page.waitForResponse((response) => new URL(response.url()).pathname === `${issuePath(opened.baseline.id)}/operations/${original.requestId}`
      && response.request().method() === "GET");
    await recover.focus();
    await recover.press("Enter");
    const response = await recoveredResponse;
    expect(response.status()).toBe(200);
    const recovered = CollaborationOperationSchema.parse(await response.json());
    expect(recovered).toEqual(original.receipt);
    await expect(recover).toHaveCount(0);
    expect(posts).toHaveLength(1);
    return { ...original, recovered, afterReload, postCount: posts.length, transport: "real POST committed; response aborted; original GET recovered" };
  } finally { await page.unroute(matches, lose); page.off("request", observe); }
}

export async function changeJourneySourceComment(session: DetailJourneySession, id: string, commentId: string, content: string) {
  const copy = journeyCopy(session.locale);
  const row = session.page.locator(`#comment-${commentId}`);
  await row.getByRole("button", { name: copy.editComment, exact: true }).click();
  const editor = row.getByRole("form", { name: copy.editComment, exact: true });
  await expect(editor).toBeVisible();
  await expect.poll(() => editor.evaluate((element) => element === document.activeElement || element.contains(document.activeElement))).toBe(true);
  const textarea = editor.getByLabel(copy.editText, { exact: true });
  await textarea.focus();
  await textarea.fill(content);
  const editedPending = session.page.waitForResponse((response) => new URL(response.url()).pathname === `${issuePath(id)}/comments/${commentId}` && response.request().method() === "PATCH");
  await editor.getByRole("button", { name: copy.saveComment, exact: true }).press("Enter");
  const editedResponse = await editedPending;
  expect(editedResponse.status()).toBe(200);
  const edited = CollaborationCommentReceiptSchema.parse(await editedResponse.json());
  await expect(editor).toHaveCount(0);
  await expect(row.getByRole("button", { name: copy.editComment, exact: true })).toBeFocused();
  await row.getByRole("button", { name: copy.deleteComment, exact: true }).click();
  const deletion = row.getByRole("form", { name: copy.deleteComment, exact: true });
  const deletedPending = session.page.waitForResponse((response) => new URL(response.url()).pathname === `${issuePath(id)}/comments/${commentId}` && response.request().method() === "DELETE");
  await deletion.getByRole("button", { name: copy.confirmDelete, exact: true }).press("Enter");
  const deletedResponse = await deletedPending;
  expect(deletedResponse.status()).toBe(200);
  const deleted = CollaborationCommentReceiptSchema.parse(await deletedResponse.json());
  await expect(row.getByText(copy.deleted, { exact: true })).toBeVisible();
  await expect(row).toBeFocused();
  return { edited, deleted };
}

export function submissionRecord(session: DetailJourneySession, id: string) {
  return session.page.locator(`#delivery [data-submission-id="${id}"]`);
}

export async function visibleJourneySubmission(session: DetailJourneySession, submission: CollaborationSubmission) {
  const copy = journeyCopy(session.locale);
  const record = submissionRecord(session, submission.id);
  await expect(record).toHaveCount(1);
  await expect(record).toBeVisible();
  await expect(record.locator("dt").filter({ hasText: copy.author }).locator("..").locator("dd")).toHaveText(submission.author!.name);
  await expect(record.locator("time").first()).toHaveAttribute("datetime", submission.created_at);
  await expect(record.getByRole("heading", { name: copy.result, exact: true })).toBeVisible();
  await expect(record.getByRole("heading", { name: copy.basis, exact: true })).toBeVisible();
  if (submission.decision) {
    const decision = submission.decision;
    await expect(record.getByText(decision.kind === "accepted" ? copy.accepted : copy.returned, { exact: true })).toBeVisible();
    await expect(record.locator("dt").filter({ hasText: copy.decisionBy }).locator("..").locator("dd")).toHaveText(decision.actor!.name);
    await expect(record.locator("time").last()).toHaveAttribute("datetime", decision.created_at);
    if (decision.reason) await expect(record).toContainText(decision.reason);
  }
  return record;
}

export async function switchJourneyLocale(session: DetailJourneySession, actor: DetailJourneyActor, id: string, locale: DetailJourneyLocale) {
  await actor.client.requestJSON("/api/me", { method: "PATCH", body: { language: locale } });
  await session.page.evaluate((value) => {
    localStorage.setItem("multica-locale", value);
    document.cookie = `multica-locale=${value}; path=/; SameSite=Lax`;
  }, locale);
  session.locale = locale;
  await refreshJourneyIssue(session, id);
  await expect(session.page.getByRole("heading", { name: journeyCopy(locale).delivery, exact: true })).toBeVisible();
}

export interface JourneyCaptureTarget {
  description: string;
  anchors: Locator[];
}

export async function captureJourney(session: DetailJourneySession, info: TestInfo, name: string, width: number,
  target: JourneyCaptureTarget) {
  const { page } = session;
  let nativeContent: { width: number; height: number; zoom: number } | null = null;
  if (session.native) {
    const window = await session.native.app.browserWindow(page);
    try {
      nativeContent = await window.evaluate((nativeWindow, size) => {
        nativeWindow.webContents.setZoomFactor(1);
        nativeWindow.setContentSize(size.width, size.height);
        const [contentWidth, contentHeight] = nativeWindow.getContentSize();
        return { width: contentWidth, height: contentHeight, zoom: nativeWindow.webContents.getZoomFactor() };
      }, { width, height: 900 });
    }
    finally { await window.dispose(); }
  } else await page.setViewportSize({ width, height: 900 });
  await expect.poll(() => page.evaluate(() => innerWidth)).toBe(width);
  await page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise<void>((done) => requestAnimationFrame(() => requestAnimationFrame(() => done())));
    await Promise.all(document.getAnimations().filter((animation) => animation.effect?.getComputedTiming().iterations !== Infinity)
      .map((animation) => animation.finished.catch(() => undefined)));
  });
  expect(target.anchors.length, target.description).toBeGreaterThan(0);
  for (const anchor of target.anchors) {
    await expect(anchor, target.description).toHaveCount(1);
    await expect(anchor, target.description).toBeVisible();
  }
  const root = page.locator('[data-tab-scroll-root="account-detail"]');
  const readGeometry = async () => {
    const viewport = await root.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      const left = Math.max(0, bounds.left + element.clientLeft);
      const top = Math.max(0, bounds.top + element.clientTop);
      const right = Math.min(innerWidth, bounds.left + element.clientLeft + element.clientWidth);
      const bottom = Math.min(innerHeight, bounds.top + element.clientTop + element.clientHeight);
      return { width: innerWidth, height: innerHeight, dpr: devicePixelRatio, documentWidth: document.documentElement.scrollWidth,
        rootWidth: element.clientWidth, rootScrollWidth: element.scrollWidth, scrollTop: element.scrollTop,
        contentViewport: { left, top, right, bottom, width: right - left, height: bottom - top } };
    });
    const anchors = await Promise.all(target.anchors.map((anchor) => anchor.evaluate((element) => {
      if (!element.closest('[data-tab-scroll-root="account-detail"]')) throw new Error("Capture target must belong to the detail content viewport");
      const rect = element.getBoundingClientRect();
      return { tag: element.tagName.toLowerCase(), text: element.textContent?.trim() ?? "",
        bounds: { left: rect.left, top: rect.top, right: rect.right, bottom: rect.bottom, width: rect.width, height: rect.height } };
    })));
    const left = Math.min(...anchors.map((anchor) => anchor.bounds.left));
    const top = Math.min(...anchors.map((anchor) => anchor.bounds.top));
    const right = Math.max(...anchors.map((anchor) => anchor.bounds.right));
    const bottom = Math.max(...anchors.map((anchor) => anchor.bounds.bottom));
    return { ...viewport, target: { description: target.description, anchors,
      bounds: { left, top, right, bottom, width: right - left, height: bottom - top } } };
  };
  const initial = await readGeometry();
  const margin = 16;
  expect(initial.target.bounds.height, target.description).toBeLessThanOrEqual(initial.contentViewport.height - margin * 2);
  expect(initial.target.bounds.width, target.description).toBeLessThanOrEqual(initial.contentViewport.width);
  await root.evaluate((element, delta) => { element.scrollTop += delta; }, initial.target.bounds.top - initial.contentViewport.top - margin);
  // Native capturePage reads the last composed frame. Wait after the final
  // scroll, rather than recording new DOM geometry beside the preceding paint.
  await page.evaluate(() => new Promise<void>((done) => requestAnimationFrame(() => requestAnimationFrame(() => done()))));
  const contained = (geometry: Awaited<ReturnType<typeof readGeometry>>) => geometry.target.anchors.every(({ text, bounds }) => text.length > 0
    && bounds.left >= geometry.contentViewport.left - 1 && bounds.right <= geometry.contentViewport.right + 1
    && bounds.top >= geometry.contentViewport.top - 1 && bounds.bottom <= geometry.contentViewport.bottom + 1);
  await expect.poll(async () => contained(await readGeometry()), { message: target.description }).toBe(true);
  const geometry = await readGeometry();
  expect(contained(geometry), target.description).toBe(true);
  expect(geometry.documentWidth).toBeLessThanOrEqual(geometry.width + 1);
  expect(geometry.rootScrollWidth).toBeLessThanOrEqual(geometry.rootWidth + 1);
  if (nativeContent) expect(nativeContent).toEqual({ width: geometry.width, height: geometry.height, zoom: 1 });
  const path = info.outputPath(`${session.platform}-${name}.png`);
  if (session.native) await nativeScreenshot(session.native.app, page, path);
  else await page.screenshot({ path, fullPage: false, scale: "css" });
  const png = readFileSync(path);
  const image = { width: png.readUInt32BE(16), height: png.readUInt32BE(20) };
  const scale = image.width / geometry.width;
  expect(session.native ? [1, geometry.dpr] : [1]).toContain(scale);
  expect(image.height).toBe(geometry.height * scale);
  const after = await readGeometry();
  expect({ width: after.width, height: after.height, dpr: after.dpr }).toEqual({ width: geometry.width, height: geometry.height, dpr: geometry.dpr });
  expect(after.scrollTop).toBe(geometry.scrollTop);
  expect(after.target).toEqual(geometry.target);
  expect(contained(after), target.description).toBe(true);
  await info.attach(`${session.platform}-${name}`, { path, contentType: "image/png" });
  return { file: `${session.platform}-${name}.png`, locale: session.locale, ...geometry, nativeContent, image, pixelScale: scale,
    targetContainedBefore: true, targetContainedAfter: true };
}

export async function assertJourneyScopeRestoration(session: DetailJourneySession, id: string, title: string) {
  const copy = journeyCopy(session.locale);
  const detail = session.page.locator('[data-tab-scroll-root="account-detail"]');
  await expect.poll(() => detail.evaluate((element) => element.scrollHeight - element.clientHeight)).toBeGreaterThan(700);
  await detail.evaluate((element) => { element.scrollTop = 620; });
  await session.page.evaluate(() => new Promise<void>((done) => requestAnimationFrame(() => requestAnimationFrame(() => done()))));
  const saved = await detail.evaluate((element) => element.scrollTop);
  const nav = session.page.getByRole("navigation", { name: copy.navigation, exact: true });
  await nav.getByRole("button", { name: copy.workspaces, exact: true }).click();
  await expect(nav.getByRole("button", { name: copy.workspaces, exact: true })).toHaveAttribute("aria-current", "page");
  await expect(session.page.getByRole("heading", { name: title, exact: true })).toBeVisible();
  await nav.getByRole("button", { name: copy.collaboration, exact: true }).click();
  await expect(nav.getByRole("button", { name: copy.collaboration, exact: true })).toHaveAttribute("aria-current", "page");
  await expect(session.page.getByRole("heading", { name: title, exact: true })).toBeVisible();
  await expect.poll(() => detail.evaluate((element) => element.scrollTop)).toBe(saved);
  if (session.native) {
    await expect.poll(() => session.page.evaluate(() => {
      const state = JSON.parse(localStorage.getItem("multica_tabs") ?? "null")?.state;
      return state?.accountGroup?.tabs.length;
    })).toBe(1);
  } else expect(new URL(session.page.url()).searchParams.get("issue")).toBe(id);
  return { issue_id: id, saved, restored: await detail.evaluate((element) => element.scrollTop), used: "application scope switches" };
}

export async function nativeJourneyEvidence(session: DetailJourneySession) {
  if (!session.native) return null;
  expect(session.native.accountHeaders.every((headers) => !headers["x-workspace-id"] && !headers["x-workspace-slug"])).toBe(true);
  const evidence = await session.native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness }).desktopAccountAcceptance.evidence);
  expect(evidence.daemonStarts).toBe(0);
  return { platform: process.platform, main_scope: "selected production routing/IPC functions with fixture services", renderer: "current production bundle and preload",
    daemon_starts: evidence.daemonStarts, provider_account_access: false, installed_main: false,
    fixture_sha256: sha256(readFileSync(join(checkout, "e2e/fixtures/collaboration-account-electron.mjs"))) };
}
