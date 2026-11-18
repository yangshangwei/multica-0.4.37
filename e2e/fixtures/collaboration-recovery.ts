import { createHash, randomUUID } from "node:crypto";
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, type Browser, type BrowserContext, type CDPSession, type Locator, type Page, type Request,
  type Route, type TestInfo } from "@playwright/test";
import { z } from "zod";
import { CollaborationActivitySchema, CollaborationCommentReceiptSchema, CollaborationCommentSchema, CollaborationFieldOptionsSchema,
  CollaborationCommentsSchema, CollaborationIssueSchema, CollaborationOperationSchema, deriveCollaborationCommentThreads,
  type CollaborationComment, type CollaborationComments, type CollaborationIssue } from "@multica/core/collaboration";
import { seedOfflineHandoff, selectHandoffRecipient, type HandoffActor, type OfflineHandoffFixture } from "./collaboration-offline-handoff";
import { deepLink, nativeScreenshot, openDesktop, type NativeHarness } from "./collaboration-native";
import { revealCollaborationDisclosure } from "./collaboration-disclosures";
import { revealCollaborationEditingAction } from "./collaboration-editing";

export type RecoveryPlatform = "Web" | "Desktop";
type JSONRecord = Record<string, unknown>;
const object = z.record(z.string(), z.unknown());
const ordinaryIssue = z.object({ id: z.uuid(), identifier: z.string(), title: z.string(), revision: z.number().int().positive(),
  priority: z.enum(["none", "low", "medium", "high", "urgent"]), status: z.string(),
  assignee_type: z.string().nullable(), assignee_id: z.uuid().nullable(), parent_issue_id: z.uuid().nullable(),
  collaboration: z.unknown().nullish() });
type OrdinaryIssue = z.infer<typeof ordinaryIssue>;
const issuePath = (id: string) => `/api/collaboration/issues/${id}`;
const digest = (value: unknown) => createHash("sha256").update(JSON.stringify(value)).digest("hex");
const now = () => new Date().toISOString();

export function recoveryEnvironmentReady(): boolean {
  return process.env.PAY241_S3_RECOVERY_ACCEPTANCE === "1" && process.env.E2E_PASSWORD_AUTH === "1" &&
    [process.env.COLLABORATION_E2E_API_URL, process.env.NEXT_PUBLIC_API_URL, process.env.DATABASE_URL,
      process.env.TASK_DATABASE_NAME, process.env.COLLABORATION_ELECTRON_RENDERER_URL,
      process.env.COLLABORATION_ELECTRON_SOURCE_ROOT].every(Boolean);
}

export class RecoveryEvidence {
  readonly value: JSONRecord = { started_at: now(), status: "RUNNING", checkpoints: [] };
  constructor(readonly info: TestInfo, readonly name: string) { this.save(); }
  set(values: JSONRecord) { Object.assign(this.value, values); this.save(); }
  checkpoint(stage: string, values: JSONRecord = {}) {
    (this.value.checkpoints as JSONRecord[]).push({ at: now(), stage, ...structuredClone(values) }); this.save();
  }
  save() { writeFileSync(this.info.outputPath(`${this.name}.json`), `${JSON.stringify(this.value, null, 2)}\n`, { mode: 0o600 }); }
}

function observations(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("The current recovery actor has no participant observations");
  return { expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
}
async function currentIssue(fixture: OfflineHandoffFixture, actor = fixture.creator): Promise<CollaborationIssue> {
  return CollaborationIssueSchema.parse(await actor.client.requestJSON(issuePath(fixture.issueId)));
}
function executionFacts(issue: CollaborationIssue) {
  return { session: issue.session, execution: issue.execution, lifecycle_epoch: issue.lifecycle_epoch };
}
function parentFacts(issue: CollaborationIssue) {
  return { id: issue.id, title: issue.title, description: issue.description, status: issue.status, priority: issue.priority,
    creator: issue.creator, handler: issue.handler, collaboration_revision: issue.collaboration_revision,
    source_workspace: issue.source_workspace, delivery: issue.delivery, current_information_request: issue.current_information_request,
    ordinary_children: issue.ordinary_children, ...executionFacts(issue) };
}

export async function seedRecovery(evidence: RecoveryEvidence): Promise<OfflineHandoffFixture> {
  for (const value of [process.env.COLLABORATION_E2E_API_URL!, process.env.NEXT_PUBLIC_API_URL!, process.env.DATABASE_URL!]) {
    expect(["127.0.0.1", "localhost"]).toContain(new URL(value).hostname);
  }
  expect(new URL(process.env.DATABASE_URL!).pathname.slice(1)).toBe(process.env.TASK_DATABASE_NAME);
  const fixture = await seedOfflineHandoff();
  evidence.set({ issue_id: fixture.issueId, source_workspace_id: fixture.workspace.id,
    actors: { creator: fixture.creator.id, external_handler: fixture.recipient.id }, api_url: fixture.apiUrl,
    database: process.env.TASK_DATABASE_NAME, real_model_execution: false, installed_main_execution: false });
  let issue = await currentIssue(fixture);
  const editKey = randomUUID(), inviteKey = randomUUID(), transferKey = randomUUID();
  await fixture.creator.client.requestJSON(issuePath(fixture.issueId), { method: "PATCH", headers: { "Idempotency-Key": editKey },
    body: { ...observations(issue), priority: "low" } });
  issue = await currentIssue(fixture);
  await fixture.creator.client.requestJSON(`${issuePath(fixture.issueId)}/participants`, { method: "POST", headers: { "Idempotency-Key": inviteKey },
    body: { ...observations(issue), target_user_id: fixture.recipient.id } });
  issue = await currentIssue(fixture);
  await fixture.creator.client.requestJSON(`${issuePath(fixture.issueId)}/handoffs`, { method: "POST", headers: { "Idempotency-Key": transferKey },
    body: { ...observations(issue), target_user_id: fixture.recipient.id, purpose: "transfer", note: "Initial explicit recovery assignment" } });
  issue = await currentIssue(fixture, fixture.recipient);
  expect(issue.handler.user_id).toBe(fixture.recipient.id);
  expect(issue.capabilities.edit).toBe(true);
  expect(issue.capabilities.transfer).toBe(true);
  expect(await fixture.recipient.client.getWorkspaces()).toEqual([]);
  expect(issue.execution?.runs ?? []).toEqual([]);
  expect(issue.execution?.commands ?? []).toEqual([]);
  evidence.checkpoint("seeded_current_password_accounts", { seed_keys: { edit: editKey, invite: inviteKey, transfer: transferKey },
    issue, external_source_membership: false });
  return fixture;
}

interface BusinessRequestFact { at: string; source: string; actor_id: string; platform: RecoveryPlatform; path: string; method: string;
  request_id: string | null; payload_kind: "json" | "multipart" | "none" | "other";
  authentication: { cookie: boolean; csrf: boolean; bearer: boolean } }
interface MutationFact extends BusinessRequestFact { body: JSONRecord; body_sha256: string; header_diagnostics: JSONRecord }
interface HTTPFact { at: string; status: number; body: unknown; header_diagnostics?: JSONRecord; operation_schema?: JSONRecord }
interface SocketFact { id: string; url: string; created_at: string; closed_at: string | null;
  frames: { at: string; direction: "sent" | "received"; type: string; scope?: string; id?: string }[] }
interface HandshakeFact { request_id: string; at: string; status: number }
interface CommentPageFact { at: string; cursor: string | null; limit: number; max_bytes: number; status: number;
  response_bytes: number; page: CollaborationComments }
type NativeSession = Awaited<ReturnType<typeof openDesktop>>;
interface SessionLifecycle { profile: string | null; native_pid: number | null; page_closed_at: string | null; context_closed_at: string | null;
  native_exit: { at: string; code: number | null; signal: string | null } | null; close_started_at: string | null; close_ended_at: string | null }
interface RecoverySession { page: Page; context: BrowserContext; platform: RecoveryPlatform; actor: HandoffActor;
  fixture: OfflineHandoffFixture; native?: NativeSession; errors: string[]; writes: BusinessRequestFact[]; lifecycle: SessionLifecycle;
  cleanup_errors: JSONRecord[]; console_facts: JSONRecord[]; request_failures: JSONRecord[]; response_facts: JSONRecord[];
  sockets: SocketFact[]; handshakes: HandshakeFact[]; pages: CommentPageFact[]; read_errors: string[];
  pendingReads: Set<Promise<void>>; cdp: CDPSession; close(): Promise<void>; navigate(path: string): Promise<void> }

function mutationFact(session: RecoverySession, request: Request, source: string, headers = request.headers()): MutationFact {
  const body = object.parse(request.postDataJSON());
  return { at: now(), source, actor_id: session.actor.id, platform: session.platform, path: new URL(request.url()).pathname,
    method: request.method(), request_id: headers["idempotency-key"] ?? null, payload_kind: "json", body, body_sha256: digest(body),
    header_diagnostics: { actual: safeHTTPHeaders(headers), provisional: safeHTTPHeaders(request.headers()),
      actual_header_names: Object.keys(headers).sort(), provisional_header_names: Object.keys(request.headers()).sort() },
    authentication: { cookie: Boolean(headers.cookie), csrf: Boolean(headers["x-csrf-token"]), bearer: Boolean(headers.authorization) } };
}

function safeOrigin(value: string | undefined) {
  if (!value) return null;
  if (value === "*" || value === "null") return value;
  try { const url = new URL(value); return ["127.0.0.1", "localhost"].includes(url.hostname) ? url.origin : "[other origin]"; }
  catch { return "[invalid origin]"; }
}
function safeHTTPHeaders(headers: Record<string, string>) {
  return { origin: safeOrigin(headers.origin), content_type: headers["content-type"]?.split(";", 1)[0] ?? null,
    content_encoding: headers["content-encoding"]?.slice(0, 64) ?? null, allow_origin: safeOrigin(headers["access-control-allow-origin"]),
    allow_credentials: headers["access-control-allow-credentials"] ?? null, cache_control: headers["cache-control"]?.slice(0, 128) ?? null,
    cors_header_present: Object.hasOwn(headers, "access-control-allow-origin"),
    auth_header_present: Boolean(headers.authorization), cookie_header_present: Boolean(headers.cookie), csrf_header_present: Boolean(headers["x-csrf-token"]) };
}
function schemaFact(schema: z.ZodType, body: unknown): JSONRecord {
  const parsed = schema.safeParse(body);
  if (!parsed.success) return { valid: false, issues: parsed.error.issues.map((issue) => ({ code: issue.code, path: issue.path.join(".") })) };
  const value = object.safeParse(parsed.data);
  return { valid: true, summary: value.success ? Object.fromEntries(["id", "issue_id", "code", "status", "collaboration_revision"]
    .filter((key) => Object.hasOwn(value.data, key)).map((key) => [key, value.data[key]])) : null };
}

function errorFact(error: unknown) {
  const value = error instanceof Error ? error : new Error(String(error));
  const message = value.message.replace(/Bearer\s+[^\s"']+/gi, "Bearer [redacted]")
    .replace(/[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}/g, "[redacted-token]")
    .replace(/\b(?:mdt_|mat_|mcn_|mul_|mrd_)[A-Za-z0-9_.-]{8,}/g, "[redacted-capability]")
    .replace(/([?&](?:cap|token|session)=)[^\s&#"']+/gi, "$1[redacted]");
  return { type: value.name, message: message.slice(0, 4096) };
}

async function observeSession(session: RecoverySession) {
  const { page, fixture } = session;
  // The proxy returns the real browser WebSocket instance and preserves its
  // production event handlers. Its only test action closes that physical socket.
  await session.context.addInitScript(() => {
    const NativeSocket = window.WebSocket;
    const instances: WebSocket[] = [];
    Object.defineProperty(window, "pay241RecoveryTransport", { value: { closeOpenSockets() {
      const selected = instances.filter((socket) => socket.readyState === NativeSocket.OPEN);
      for (const socket of selected) socket.close(4001, "S3 offline recovery");
      return { count: selected.length, all_native: selected.every((socket) => socket instanceof NativeSocket) };
    } } });
    window.WebSocket = new Proxy(NativeSocket, { construct(target, args) {
      const socket = Reflect.construct(target, args) as WebSocket; instances.push(socket); return socket;
    } });
  });
  page.on("pageerror", (error) => session.errors.push(errorFact(error).message));
  page.on("console", (message) => {
    const text = message.text();
    if (["warning", "error"].includes(message.type()) && /CORS|cross.origin|net::|ERR_|fetch|collaboration|server session/i.test(text)) {
      session.console_facts.push({ at: now(), type: message.type(), text: errorFact(new Error(text)).message });
    }
  });
  page.on("requestfailed", (request) => {
    const url = new URL(request.url());
    if (!url.pathname.startsWith("/api/")) return;
    const provisional = request.headers();
    session.request_failures.push({ at: now(), origin: url.origin, path: url.pathname, method: request.method(),
      request_id: provisional["idempotency-key"] ?? null, client_request_id: provisional["x-request-id"] ?? null,
      failure: errorFact(new Error(request.failure()?.errorText ?? "request failed")).message,
      header_diagnostics: safeHTTPHeaders(provisional) });
  });
  page.on("response", (response) => {
    const url = new URL(response.url()), request = response.request();
    if (!url.pathname.startsWith(issuePath(fixture.issueId))) return;
    const pending = (async () => {
      const fact: JSONRecord = { at: now(), origin: url.origin, path: url.pathname, method: request.method(), status: response.status() };
      try {
        const headers = await response.allHeaders(), requestHeaders = await request.allHeaders();
        Object.assign(fact, { request_id: requestHeaders["idempotency-key"] ?? null, client_request_id: requestHeaders["x-request-id"] ?? null,
          response_headers: safeHTTPHeaders(headers), request_headers: safeHTTPHeaders(requestHeaders) });
        const schema = url.pathname === issuePath(fixture.issueId) ? request.method() === "GET" ? CollaborationIssueSchema : CollaborationOperationSchema
          : url.pathname === `${issuePath(fixture.issueId)}/field-options` ? CollaborationFieldOptionsSchema
            : url.pathname.endsWith("/handoffs") || url.pathname.includes("/operations/") ? CollaborationOperationSchema : null;
        if (schema && headers["content-type"]?.includes("application/json")) {
          const bytes = await response.body(), body: unknown = JSON.parse(bytes.toString());
          Object.assign(fact, { body_bytes: bytes.length, body_sha256: createHash("sha256").update(bytes).digest("hex"), schema: schemaFact(schema, body),
            ...(schema === CollaborationOperationSchema ? { operation_body: body } : {}) });
        }
      } catch (error) { fact.response_read_error = errorFact(error); }
      session.response_facts.push(fact);
    })();
    session.pendingReads.add(pending);
    void pending.finally(() => session.pendingReads.delete(pending));
  });
  page.on("request", (request) => {
    const path = new URL(request.url()).pathname;
    if (["GET", "HEAD", "OPTIONS"].includes(request.method()) ||
      !path.startsWith("/api/collaboration/") && !path.startsWith("/api/me/collaboration-notifications/") && !/^\/api\/issues\/[^/]+$/.test(path)) return;
    const pending = request.allHeaders().then((headers) => {
      const type = headers["content-type"] ?? "";
      // This broad observer records upload/creation and other collaboration
      // writes too, without saving raw authentication or multipart/file bytes.
      session.writes.push({ at: now(), source: "ALL_COLLABORATION_BUSINESS_REQUEST_METADATA", actor_id: session.actor.id,
        platform: session.platform, path, method: request.method(), request_id: headers["idempotency-key"] ?? null,
        payload_kind: type.includes("multipart/") ? "multipart" : type.includes("application/json") ? "json" : request.postData() === null ? "none" : "other",
        authentication: { cookie: Boolean(headers.cookie), csrf: Boolean(headers["x-csrf-token"]), bearer: Boolean(headers.authorization) } });
    }).catch((error) => { session.errors.push(`Business request metadata unavailable: ${errorFact(error).type}`); });
    session.pendingReads.add(pending);
    void pending.finally(() => session.pendingReads.delete(pending));
  });
  session.cdp.on("Network.webSocketCreated", (event) => {
    const url = new URL(event.url);
    expect(url.searchParams.has("token")).toBe(false);
    session.sockets.push({ id: event.requestId, url: `${url.origin}${url.pathname}`, created_at: now(), closed_at: null, frames: [] });
  });
  for (const [eventName, direction] of [["Network.webSocketFrameSent", "sent"], ["Network.webSocketFrameReceived", "received"]] as const) {
    session.cdp.on(eventName, (event) => {
      const fact = session.sockets.find((socket) => socket.id === event.requestId);
      if (!fact) return;
      let frame: JSONRecord;
      try { frame = object.parse(JSON.parse(event.response.payloadData)); } catch { return; }
      if (typeof frame.type !== "string") return;
      // Never retain auth or business frame payloads. Only subscribe identity
      // and protocol frame types are necessary for this ordering proof.
      const value = frame.type === "subscribe" || frame.type === "subscribe_ack" ? object.safeParse(frame.payload) : null;
      fact.frames.push({ at: now(), direction, type: frame.type,
        ...(value?.success && typeof value.data.scope === "string" && typeof value.data.id === "string"
          ? { scope: value.data.scope, id: value.data.id } : {}) });
    });
  }
  session.cdp.on("Network.webSocketClosed", (event) => {
    const fact = session.sockets.find((socket) => socket.id === event.requestId);
    if (fact) fact.closed_at = now();
  });
  session.cdp.on("Network.webSocketHandshakeResponseReceived", (event) => {
    session.handshakes.push({ request_id: event.requestId, at: now(), status: event.response.status });
  });
  await session.cdp.send("Network.enable");
  page.on("response", (response) => {
    const url = new URL(response.url());
    if (url.pathname !== `${issuePath(fixture.issueId)}/comments` || response.request().method() !== "GET" || response.status() !== 200) return;
    const pending = (async () => {
      try {
        const bytes = await response.body();
        session.pages.push({ at: now(), cursor: url.searchParams.get("cursor"), limit: Number(url.searchParams.get("limit")),
          max_bytes: Number(url.searchParams.get("max_bytes")), status: response.status(), response_bytes: bytes.length,
          page: CollaborationCommentsSchema.parse(JSON.parse(bytes.toString())) });
      } catch (error) { session.read_errors.push(error instanceof Error ? error.name : "comment_page_read_error"); }
    })();
    session.pendingReads.add(pending);
    void pending.finally(() => session.pendingReads.delete(pending));
  });
}

async function openSession(browser: Browser, platform: RecoveryPlatform, actor: HandoffActor, fixture: OfflineHandoffFixture,
  info: TestInfo, evidence: RecoveryEvidence): Promise<RecoverySession> {
  let native: NativeSession | undefined, profile: string | undefined, context: BrowserContext | undefined, page: Page | undefined,
    cdp: CDPSession | undefined, session: RecoverySession | undefined, closing: Promise<void> | undefined;
  const lifecycle: SessionLifecycle = { profile: null, native_pid: null, page_closed_at: null, context_closed_at: null,
    native_exit: null, close_started_at: null, close_ended_at: null };
  const close = () => closing ??= (async () => {
    lifecycle.close_started_at = now();
    if (session) await Promise.allSettled([...session.pendingReads]);
    if (cdp) await cdp.detach().catch(() => undefined);
    try { if (native) await native.app.close(); else if (context) await context.close(); }
    finally {
      if (profile) rmSync(profile, { recursive: true, force: true });
      lifecycle.close_ended_at = now();
    }
  })();
  try {
    if (platform === "Desktop") {
      const root = process.env.PAY241_S3_PROFILE_ROOT ?? tmpdir();
      mkdirSync(root, { recursive: true, mode: 0o700 });
      profile = mkdtempSync(join(root, "pay241-s3-client-")); lifecycle.profile = profile;
      await actor.login();
      native = await openDesktop(profile, { token: actor.client.getToken()! });
      const childProcess = native.app.process(); lifecycle.native_pid = childProcess.pid ?? null;
      childProcess.once("exit", (code, signal) => { lifecycle.native_exit = { at: now(), code, signal }; });
      context = native.app.context(); page = native.page;
    } else {
      context = await browser.newContext(); page = await context.newPage();
      await actor.authenticateWeb(page);
      const cookies = await context.cookies(fixture.apiUrl);
      expect(cookies.some((cookie) => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
      expect(cookies.some((cookie) => cookie.name === "multica_csrf" && !cookie.httpOnly)).toBe(true);
      await context.addInitScript(() => localStorage.removeItem("multica_token"));
    }
    page.on("close", () => { lifecycle.page_closed_at = now(); });
    context.on("close", () => { lifecycle.context_closed_at = now(); });
    cdp = await context.newCDPSession(page);
    const activePage = page, activeNative = native;
    session = { page, context, actor, fixture, platform, native, lifecycle, errors: [], cleanup_errors: [], console_facts: [], request_failures: [], response_facts: [], writes: [], sockets: [], handshakes: [],
      pages: [], read_errors: [], pendingReads: new Set(), cdp, close,
      async navigate(path) {
        if (activeNative) await deepLink(activeNative.app, path);
        else await activePage.goto(new URL(path, info.project.use.baseURL).href);
      } };
    await observeSession(session);
    if (native) await native.start();
    await session.navigate(fixture.accountPath);
    await expect(page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
    const principal = await platformRequest(session, "/api/me");
    expect(principal.status).toBe(200); expect(object.parse(principal.body).id).toBe(actor.id);
    return session;
  } catch (error) {
    const original = errorFact(error);
    const cleanup = await Promise.allSettled([close()]);
    evidence.checkpoint("client_acquisition_failed_with_partial_cleanup", { actor_id: actor.id, platform, original_error: original, lifecycle,
      cleanup: cleanup.map((result) => result.status === "fulfilled" ? { status: result.status } : { status: result.status, error: errorFact(result.reason) }) });
    throw error;
  }
}

async function platformRequest(session: RecoverySession, path: string, method = "GET", body?: JSONRecord, requestId?: string,
  workspaceId?: string): Promise<HTTPFact> {
  const result = await session.page.evaluate(async ({ apiUrl, platform, path, method, body, requestId, workspaceId }) => {
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (platform === "Desktop") {
      const token = localStorage.getItem("multica_token");
      if (!token) throw new Error("The current native password session is unavailable");
      headers.Authorization = `Bearer ${token}`;
    } else if (method !== "GET") {
      const csrf = document.cookie.split(";").map((value) => value.trim()).find((value) => value.startsWith("multica_csrf="));
      if (!csrf) throw new Error("The current browser password CSRF cookie is unavailable");
      headers["X-CSRF-Token"] = decodeURIComponent(csrf.slice("multica_csrf=".length));
    }
    if (requestId) headers["Idempotency-Key"] = requestId;
    if (workspaceId) headers["X-Workspace-ID"] = workspaceId;
    const response = await fetch(`${apiUrl}${path}`, { method, headers, credentials: "include",
      ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
    return { status: response.status, body: await response.json() as unknown };
  }, { apiUrl: session.fixture.apiUrl, platform: session.platform, path, method, body, requestId, workspaceId });
  return { at: now(), ...result };
}

async function withSessions<T>(browser: Browser, platform: RecoveryPlatform, fixture: OfflineHandoffFixture, info: TestInfo,
  evidence: RecoveryEvidence, exercise: (open: (actor: HandoffActor) => Promise<RecoverySession>) => Promise<T>): Promise<T> {
  const sessions: RecoverySession[] = [];
  let outcome: { ok: true; value: T } | { ok: false; error: unknown };
  let cleanupFailed = false;
  try {
    const result = await exercise(async (actor) => { const session = await openSession(browser, platform, actor, fixture, info, evidence); sessions.push(session); return session; });
    for (const session of sessions) {
      expect(session.errors).toEqual([]);
      if (session.native) expect(await session.native.app.evaluate(() => (globalThis as unknown as { desktopAccountAcceptance: NativeHarness })
        .desktopAccountAcceptance.evidence.daemonStarts)).toBe(0);
    }
    outcome = { ok: true, value: result };
  } catch (error) {
    outcome = { ok: false, error };
    const pendingIdentities = await Promise.all(sessions.map(async (session) => ({ actor_id: session.actor.id,
      pending: session.page.isClosed() ? null : await pendingWorkflowIdentities(session).catch(() => null) })));
    evidence.checkpoint("original_failure_before_owned_teardown", { original_error: errorFact(error), active_phase: evidence.value.active_phase ?? null,
      pending_identities: pendingIdentities,
      sessions: sessions.map((session) => ({ actor_id: session.actor.id, page_closed: session.page.isClosed(), lifecycle: session.lifecycle })) });
    for (const [index, session] of sessions.entries()) if (!session.page.isClosed()) {
      try {
        const file = info.outputPath(`${evidence.name}-failure-${index}.png`);
        if (session.native) await nativeScreenshot(session.native.app, session.page, file);
        else await session.page.screenshot({ path: file, animations: "disabled" });
      } catch { /* Keep the original failure if its window cannot be captured. */ }
    }
  } finally {
    for (const session of sessions) await Promise.allSettled([...session.pendingReads]);
    const cleanup = await Promise.allSettled(sessions.map((session) => session.close()));
    evidence.set({ sessions: sessions.map((session) => ({ platform: session.platform, actor_id: session.actor.id,
      writes: session.writes, sockets: session.sockets, handshakes: session.handshakes, comment_pages: session.pages,
      console_facts: session.console_facts, request_failures: session.request_failures, response_facts: session.response_facts,
      page_errors: session.errors, comment_page_errors: session.read_errors, lifecycle: session.lifecycle, cleanup_errors: session.cleanup_errors })),
      all_sessions_cleanup: cleanup.map((result) => result.status === "fulfilled" ? { status: result.status } : { status: result.status, error: errorFact(result.reason) }) });
    cleanupFailed = cleanup.some((result) => result.status === "rejected") || sessions.some((session) => session.cleanup_errors.length > 0);
  }
  if (!outcome.ok) throw outcome.error;
  if (cleanupFailed) throw new Error("An owned recovery client cleanup failed; all cleanup outcomes are retained separately");
  return outcome.value;
}

/** Each original browser request waits here before any server mutation. The
 * decision either forwards its unchanged request or aborts it during teardown. */
class HeldRequest {
  fact: MutationFact | null = null;
  result: HTTPFact | null = null;
  private route: Route | null = null;
  private decision: "forward" | "lose_response" | "abort" | null = null;
  private failure: unknown;
  private installed = false;
  private decide!: () => void;
  private ended!: () => void;
  private gate = new Promise<void>((resolve) => { this.decide = resolve; });
  private terminal = new Promise<void>((resolve) => { this.ended = resolve; });
  private matches = (url: URL) => url.pathname === this.path;
  private handler = async (route: Route) => {
    if (route.request().method() !== this.method) { await route.continue(); return; }
    if (this.route) { await route.abort("failed"); this.failure = new Error("The original intent was submitted more than once"); return; }
    this.route = route;
    try {
      this.fact = mutationFact(this.session, route.request(), this.source, await route.request().allHeaders());
      await this.gate;
      if (this.decision === "abort") { await route.abort("failed"); return; }
      if (this.decision === "forward") {
        const responseWait = this.session.page.waitForResponse((response) => response.request() === route.request(), { timeout: 15_000 });
        const [continued, received] = await Promise.allSettled([route.continue(), responseWait]);
        if (continued.status === "rejected") throw continued.reason;
        if (received.status === "rejected") throw received.reason;
        const response = received.value;
        const body: unknown = await response.json();
        this.result = { at: now(), status: response.status(), body, header_diagnostics: safeHTTPHeaders(await response.allHeaders()),
          operation_schema: schemaFact(CollaborationOperationSchema, body) };
        return;
      }
      const response = await route.fetch({ maxRetries: 0, timeout: 15_000 });
      try {
        const body: unknown = await response.json();
        this.result = { at: now(), status: response.status(), body, header_diagnostics: safeHTTPHeaders(response.headers()),
          operation_schema: schemaFact(CollaborationOperationSchema, body) };
        await route.abort("failed");
      } finally { await response.dispose(); }
    } catch (error) { this.failure = error; await route.abort("failed").catch(() => undefined); }
    finally { this.ended(); }
  };
  constructor(private session: RecoverySession, private path: string, private method: string, private source = "UI_BUILT_UNCHANGED") {}
  async install() { await this.session.page.route(this.matches, this.handler); this.installed = true; }
  async captured(): Promise<MutationFact> {
    await expect.poll(() => this.fact !== null, { message: "The actual current client must build its request before either server write" }).toBe(true);
    return this.fact!;
  }
  async forward(loseResponse = false): Promise<HTTPFact> {
    await this.captured();
    if (this.decision) throw new Error("The original held request already has a terminal decision");
    this.decision = loseResponse ? "lose_response" : "forward"; this.decide(); await this.terminal;
    if (this.failure) throw this.failure;
    if (!this.result) throw new Error("The actual API did not return an original request result");
    return this.result;
  }
  async close() {
    if (!this.decision) { this.decision = "abort"; this.decide(); }
    if (this.route) await this.terminal;
    if (!this.installed) return;
    this.installed = false;
    if (this.session.page.isClosed()) return;
    try { await this.session.page.unroute(this.matches, this.handler); }
    catch (error) { this.session.cleanup_errors.push({ at: now(), action: "held_request_unroute", path: this.path, method: this.method,
      page_closed: this.session.page.isClosed(), error: errorFact(error) }); }
  }
}

async function workflowForm(session: RecoverySession, action: "edit" | "transfer"): Promise<Locator> {
  const name = action === "edit" ? "Edit issue" : "Transfer";
  if (action === "edit") await (await revealCollaborationEditingAction(session.page, "Edit issue")).click();
  else {
    const button = session.page.getByRole("button", { name, exact: true });
    if (await button.isVisible()) await button.click();
    else { await revealCollaborationDisclosure(session.page, "More actions"); await session.page.getByRole("menuitem", { name, exact: true }).click(); }
  }
  const form = session.page.getByRole("form", { name, exact: true });
  await expect(form).toBeVisible();
  return form;
}

async function recoverOriginal(session: RecoverySession, original: MutationFact, evidence: RecoveryEvidence) {
  if (!original.request_id) throw new Error("The UI omitted an original request ID");
  const path = `${issuePath(session.fixture.issueId)}/operations/${original.request_id}`;
  const before = await platformRequest(session, path);
  expect(before.status).toBe(200);
  const saved = CollaborationOperationSchema.parse(before.body);
  evidence.checkpoint("original_GET_before_fresh_intent", { request_id: original.request_id, receipt: before });
  return { path, saved, before };
}

async function pendingWorkflowIdentities(session: RecoverySession) {
  const values = await session.page.evaluate(({ userId, issueId }) => {
    const isObject = (value: unknown): value is Record<string, unknown> => typeof value === "object" && value !== null && !Array.isArray(value);
    const result = [];
    for (const key of Object.keys(localStorage)) {
      if (!key.includes("multica_collaboration_draft:")) continue;
      let raw: unknown;
      try { raw = JSON.parse(localStorage.getItem(key) ?? "null"); } catch { continue; }
      if (!isObject(raw) || raw.kind !== "operation" || !isObject(raw.scope) || !isObject(raw.payload)) continue;
      const scope = raw.scope, action = raw.payload;
      if (scope.userId !== userId || scope.issueId !== issueId || action.domain !== undefined) continue;
      const text = (value: unknown) => typeof value === "string" ? value : null;
      result.push({ request_id: text(raw.requestId), payload_request_id: text(action.requestId), action: text(action.action),
        server: text(scope.server), user_id: text(scope.userId), issue_id: text(scope.issueId),
        expected_collaboration_revision: text(action.expectedCollaborationRevision), expected_lifecycle_epoch: text(action.expectedLifecycleEpoch),
        expected_participant_epoch: text(action.expectedParticipantEpoch), expected_account_epoch: text(action.expectedAccountEpoch),
        target_user_id: text(action.targetUserId) });
    }
    return result;
  }, { userId: session.actor.id, issueId: session.fixture.issueId });
  return z.array(z.object({ request_id: z.string().nullable(), payload_request_id: z.string().nullable(), action: z.string().nullable(),
    server: z.string().nullable(), user_id: z.string().nullable(), issue_id: z.string().nullable(),
    expected_collaboration_revision: z.string().nullable(), expected_lifecycle_epoch: z.string().nullable(),
    expected_participant_epoch: z.string().nullable(), expected_account_epoch: z.string().nullable(), target_user_id: z.string().nullable() })).parse(values);
}

/** A direct GET verifies the remote ledger but does not acknowledge a saved
 * local intent. Settle that exact intent through its real UI recovery action. */
async function settlePendingWorkflowUI(session: RecoverySession, original: MutationFact, expected: unknown, evidence: RecoveryEvidence) {
  if (!original.request_id) throw new Error("Original workflow recovery requires its retained request key");
  const recover = session.page.getByRole("button", { name: "Check original result", exact: true }).filter({ visible: true });
  let identities: Awaited<ReturnType<typeof pendingWorkflowIdentities>> = [];
  let pending = false;
  await expect.poll(async () => {
    identities = await pendingWorkflowIdentities(session);
    pending = identities.some((entry) => entry.request_id === original.request_id);
    return !pending || await recover.count() === 1 && await recover.isVisible() && await recover.isEnabled();
  }, { message: "The original UI intent must already be retired or expose its exact current recovery control" }).toBe(true);
  const before = await platformRequest(session, issuePath(session.fixture.issueId));
  expect(before.status).toBe(200);
  const rights = CollaborationIssueSchema.parse(before.body).capabilities;
  evidence.checkpoint("original_UI_pending_state_before_fresh_intent", { request_id: original.request_id, identities,
    current_rights: { read: rights.read, edit: rights.edit, transfer: rights.transfer }, requires_UI_recovery: pending });
  if (!pending) return;
  expect(identities).toHaveLength(1);
  expect(identities[0]).toMatchObject({ request_id: original.request_id, payload_request_id: original.request_id,
    user_id: session.actor.id, issue_id: session.fixture.issueId,
    expected_collaboration_revision: original.body.expected_collaboration_revision,
    expected_lifecycle_epoch: original.body.expected_lifecycle_epoch,
    expected_participant_epoch: original.body.expected_participant_epoch, expected_account_epoch: original.body.expected_account_epoch });
  const path = `${issuePath(session.fixture.issueId)}/operations/${original.request_id}`;
  await Promise.allSettled([...session.pendingReads]);
  const count = session.writes.length;
  const responsePending = session.page.waitForResponse((response) => new URL(response.url()).pathname === path && response.request().method() === "GET", { timeout: 15_000 });
  const click = recover.click();
  let response: Awaited<typeof responsePending>;
  try { [, response] = await Promise.all([click, responsePending]); }
  finally { await Promise.allSettled([click, responsePending]); }
  expect(response.status()).toBe(200);
  const receipt = CollaborationOperationSchema.parse(await response.json());
  expect(receipt).toEqual(CollaborationOperationSchema.parse(expected));
  await expect.poll(async () => (await pendingWorkflowIdentities(session)).some((entry) => entry.request_id === original.request_id)).toBe(false);
  await expect(recover).toHaveCount(0);
  await Promise.allSettled([...session.pendingReads]);
  expect(session.writes).toHaveLength(count);
  evidence.checkpoint("original_UI_intent_settled_by_exact_GET_without_POST", { request_id: original.request_id, receipt,
    pending_after: await pendingWorkflowIdentities(session), recovery_mutations: 0 });
  // A recovered refusal reopens the original form with its old observation.
  // Dismiss that form explicitly before a separate fresh intent is prepared.
  const name = identities[0]!.action === "transfer" ? "Transfer" : "Edit issue";
  const restored = session.page.getByRole("form", { name, exact: true });
  if (await restored.isVisible()) {
    if (identities[0]!.action === "transfer") await expect(restored.getByLabel("Note", { exact: true })).toHaveValue(String(original.body.note));
    evidence.checkpoint("original_refusal_form_restored_with_old_observation", { request_id: original.request_id, action: identities[0]!.action,
      original_expected_collaboration_revision: identities[0]!.expected_collaboration_revision,
      preserved_note_sha256: identities[0]!.action === "transfer" ? digest(await restored.getByLabel("Note", { exact: true }).inputValue()) : null });
    await restored.getByRole("button", { name: "Close", exact: true }).click();
    await expect(restored).toHaveCount(0);
    evidence.checkpoint("original_refusal_form_dismissed_before_new_intent", { request_id: original.request_id, recovery_mutations: 0 });
  }
}

async function recoverLostUIResponse(session: RecoverySession, original: MutationFact, expected: unknown) {
  await expect(session.page.getByText("The result is not confirmed. Keep this request and check its original outcome.", { exact: true })).toBeVisible();
  await Promise.allSettled([...session.pendingReads]);
  const count = session.writes.length;
  await session.page.reload();
  await expect(session.page.getByRole("heading", { name: session.fixture.title, exact: true })).toBeVisible();
  const recover = session.page.getByRole("button", { name: "Check original result", exact: true });
  await expect(recover).toBeVisible();
  const path = `${issuePath(session.fixture.issueId)}/operations/${original.request_id}`;
  const pending = session.page.waitForResponse((response) => new URL(response.url()).pathname === path && response.request().method() === "GET", { timeout: 15_000 });
  const click = recover.click();
  let response: Awaited<typeof pending>;
  try { [, response] = await Promise.all([click, pending]); }
  finally { await Promise.allSettled([click, pending]); }
  expect(response.status()).toBe(200);
  expect(CollaborationOperationSchema.parse(await response.json())).toEqual(CollaborationOperationSchema.parse(expected));
  await expect(recover).toHaveCount(0);
  await Promise.allSettled([...session.pendingReads]);
  expect(session.writes).toHaveLength(count);
}

export async function exerciseRecoveryPriority(browser: Browser, platform: RecoveryPlatform, info: TestInfo,
  evidence: RecoveryEvidence, first: "priority" | "transfer") {
  const fixture = await seedRecovery(evidence);
  return withSessions(browser, platform, fixture, info, evidence, async (open) => {
    const before = await currentIssue(fixture);
    const sourceBefore = ordinaryIssue.parse(await fixture.creator.client.requestJSON(`/api/issues/${fixture.issueId}`));
    const prioritySession = await open(fixture.recipient), transferSession = await open(fixture.recipient), creatorSession = await open(fixture.creator);
    const priorityForm = await workflowForm(prioritySession, "edit"), transferForm = await workflowForm(transferSession, "transfer");
    await priorityForm.getByLabel("Priority", { exact: true }).selectOption("high");
    await selectHandoffRecipient(transferSession.page, transferForm, fixture.creator);
    await transferForm.getByLabel("Note", { exact: true }).fill("Explicit original priority recovery handoff");
    const priority = new HeldRequest(prioritySession, issuePath(fixture.issueId), "PATCH");
    const transfer = new HeldRequest(transferSession, `${issuePath(fixture.issueId)}/handoffs`, "POST");
    await priority.install(); await transfer.install();
    try {
      await priorityForm.getByRole("button", { name: "Confirm", exact: true }).click();
      await transferForm.getByRole("button", { name: "Confirm", exact: true }).click();
      const priorityInput = await priority.captured(), transferInput = await transfer.captured();
      for (const original of [priorityInput, transferInput]) {
        expect(original.body).toMatchObject({ expected_collaboration_revision: before.collaboration_revision,
          expected_lifecycle_epoch: before.lifecycle_epoch });
        expect(original.request_id).toMatch(/^[a-f0-9-]{36}$/i);
        expect(original.authentication).toMatchObject(platform === "Web" ? { cookie: true, csrf: true, bearer: false } : { bearer: true });
      }
      expect(priorityInput.request_id).not.toBe(transferInput.request_id);
      expect(priorityInput.body).toMatchObject({ title: before.title, description: before.description, priority: "high" });
      expect(transferInput.body).toMatchObject({ purpose: "transfer", target_user_id: fixture.creator.id });
      evidence.checkpoint("both_UI_requests_captured_before_first_forward", { first, priority: priorityInput, transfer: transferInput });
      const winner = first === "priority" ? priority : transfer, loser = first === "priority" ? transfer : priority;
      const won = await winner.forward(true);
      evidence.checkpoint("real_winner_committed_response_lost", { request: winner.fact, response: won });
      expect(won.status).toBe(200);
      expect(object.parse(won.body).code).toBe(first === "priority" ? "issue_edited" : "transferred");
      const lost = await loser.forward();
      evidence.checkpoint("real_following_writer_refused", { request: loser.fact, response: lost });
      expect(lost.status).toBe(first === "priority" ? 409 : 403);
      expect(object.parse(lost.body)).toMatchObject({ code: first === "priority" ? "stale_revision" : "forbidden", status: lost.status,
        collaboration_revision: String(BigInt(before.collaboration_revision) + 1n) });
      await priority.close(); await transfer.close();
      const afterWinner = await currentIssue(fixture);
      expect(afterWinner).toMatchObject({ priority: first === "priority" ? "high" : "low",
        handler: { user_id: first === "priority" ? fixture.recipient.id : fixture.creator.id },
        collaboration_revision: String(BigInt(before.collaboration_revision) + 1n) });
      expect(executionFacts(afterWinner)).toEqual(executionFacts(before));
      const savedPriority = await recoverOriginal(prioritySession, priorityInput, evidence);
      const savedTransfer = await recoverOriginal(transferSession, transferInput, evidence);
      expect(savedPriority.saved).toEqual(CollaborationOperationSchema.parse(first === "priority" ? won.body : lost.body));
      expect(savedTransfer.saved).toEqual(CollaborationOperationSchema.parse(first === "priority" ? lost.body : won.body));
      evidence.checkpoint("both_original_GETs_match_the_actual_POST_outcomes", { priority: savedPriority.saved, transfer: savedTransfer.saved });
      await recoverLostUIResponse(first === "priority" ? prioritySession : transferSession, winner.fact!, won.body);
      evidence.checkpoint("lost_response_recovered_by_UI_original_GET", { request_id: winner.fact!.request_id, no_automatic_repost: true });
      await settlePendingWorkflowUI(prioritySession, priorityInput, first === "priority" ? won.body : lost.body, evidence);
      await settlePendingWorkflowUI(transferSession, transferInput, first === "priority" ? lost.body : won.body, evidence);
      if (first === "priority") {
        const same = await platformRequest(prioritySession, priorityInput.path, "PATCH", priorityInput.body, priorityInput.request_id!);
        const changed = await platformRequest(prioritySession, priorityInput.path, "PATCH", { ...priorityInput.body, priority: "urgent" }, priorityInput.request_id!);
        expect(same.status).toBe(200); expect(same.body).toEqual(won.body);
        expect(changed.status).toBe(409); expect(object.parse(changed.body).code).toBe("idempotency_conflict");
        expect((await currentIssue(fixture)).collaboration_revision).toBe(afterWinner.collaboration_revision);
        evidence.checkpoint("same_original_key_converges_changed_priority_refuses", { same, changed });
      }

      // The remaining field is a separate human intent under a new current
      // observation, not a deferred retry of the immutable refusal.
      const freshSession = first === "priority" ? transferSession : creatorSession;
      evidence.set({ active_phase: "fresh_intent_reload", fresh_actor_id: freshSession.actor.id });
      evidence.checkpoint("fresh_intent_before_reload", { actor_id: freshSession.actor.id, page_closed: freshSession.page.isClosed(), lifecycle: freshSession.lifecycle });
      await freshSession.page.reload();
      await expect(freshSession.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
      evidence.set({ active_phase: "fresh_intent_open_form" });
      evidence.checkpoint("fresh_intent_after_reload", { actor_id: freshSession.actor.id, lifecycle: freshSession.lifecycle });
      const freshForm = await workflowForm(freshSession, first === "priority" ? "transfer" : "edit");
      evidence.set({ active_phase: "fresh_intent_select_fields" });
      if (first === "priority") {
        await selectHandoffRecipient(freshSession.page, freshForm, fixture.creator);
        await freshForm.getByLabel("Note", { exact: true }).fill("Fresh explicit handoff after priority conflict");
      } else await freshForm.getByLabel("Priority", { exact: true }).selectOption("high");
      const fresh = new HeldRequest(freshSession, first === "priority" ? `${issuePath(fixture.issueId)}/handoffs` : issuePath(fixture.issueId),
        first === "priority" ? "POST" : "PATCH");
      await fresh.install();
      try {
        evidence.set({ active_phase: "fresh_intent_confirm" });
        evidence.checkpoint("fresh_intent_before_confirm", { actor_id: freshSession.actor.id, page_closed: freshSession.page.isClosed(), lifecycle: freshSession.lifecycle });
        await freshForm.getByRole("button", { name: "Confirm", exact: true }).click();
        const input = await fresh.captured();
        expect(input.body.expected_collaboration_revision).toBe(afterWinner.collaboration_revision);
        expect([priorityInput.request_id, transferInput.request_id]).not.toContain(input.request_id);
        const result = await fresh.forward();
        evidence.checkpoint("fresh_authorized_actual_response_before_UI_completion", { input, result,
          pending: await pendingWorkflowIdentities(freshSession), response_facts: freshSession.response_facts,
          request_failures: freshSession.request_failures, console_facts: freshSession.console_facts });
        expect(result.status).toBe(200);
        await expect(freshForm).toHaveCount(0);
        evidence.checkpoint("fresh_authorized_UI_intent_committed", { input, result });
      } finally { await fresh.close(); }
      const final = await currentIssue(fixture);
      expect(final).toMatchObject({ priority: "high", handler: { user_id: fixture.creator.id },
        collaboration_revision: String(BigInt(before.collaboration_revision) + 2n), title: before.title, description: before.description,
        status: before.status });
      expect(executionFacts(final)).toEqual(executionFacts(before));
      const sourceFinal = ordinaryIssue.parse(await fixture.creator.client.requestJSON(`/api/issues/${fixture.issueId}`));
      expect(sourceFinal.revision).toBe(sourceBefore.revision + 2);
      for (const [session, original, saved] of [[prioritySession, priorityInput, savedPriority], [transferSession, transferInput, savedTransfer]] as const) {
        const recovered = await platformRequest(session, saved.path);
        expect(recovered.status).toBe(200); expect(CollaborationOperationSchema.parse(recovered.body)).toEqual(saved.saved);
        const refusedOldPost = await platformRequest(session, original.path, original.method, original.body, original.request_id!);
        expect(refusedOldPost.status).toBe(403); expect(object.parse(refusedOldPost.body).code).toBe("forbidden");
        const wrongActor = await platformRequest(creatorSession, saved.path);
        expect(wrongActor.status).toBe(404);
        evidence.checkpoint("original_success_or_refusal_immutable_after_fresh_intent", { request_id: original.request_id, recovered, refusedOldPost, wrongActor });
      }
      const activity = CollaborationActivitySchema.parse(await fixture.creator.client.requestJSON(`${issuePath(fixture.issueId)}/activity?limit=50`));
      expect(activity.next_cursor).toBeNull();
      const additions = activity.items.filter((entry) => BigInt(entry.event_seq) > BigInt(before.realtime?.event_seq ?? "0"));
      expect(additions.map((entry) => entry.kind).sort()).toEqual(["issue_edited", "transferred"]);
      expect(new Set(additions.map((entry) => entry.id)).size).toBe(2);
      expect(await fixture.recipient.client.getWorkspaces()).toEqual([]);
      const finalForm = await workflowForm(creatorSession, "edit");
      await expect(finalForm.getByLabel("Priority", { exact: true })).toHaveValue("high");
      await finalForm.getByRole("button", { name: "Close", exact: true }).click();
      evidence.set({ final_issue: final, source_final: sourceFinal, new_activity: additions, original_receipts: { priority: savedPriority.saved, transfer: savedTransfer.saved },
        original_denominator: 2, fresh_denominator: 1, UI_requests_built_before_forward: true, original_execution_unchanged: true });
      return { issue_id: fixture.issueId, order: first, status: "PASSED", original_requests: 2, fresh_requests: 1 };
    } finally { await priority.close(); await transfer.close(); }
  });
}

async function openOrdinaryChild(session: RecoverySession, child: OrdinaryIssue) {
  await session.navigate(session.fixture.accountPath);
  await expect(session.page.getByRole("heading", { name: session.fixture.title, exact: true })).toBeVisible();
  const drawer = await revealCollaborationDisclosure(session.page, "Sub-issues");
  const link = drawer.getByRole("link", { name: `${child.identifier} ${child.title}`, exact: true });
  await expect(link).toBeVisible(); await link.click();
  await expect(session.page.getByRole("button", { name: child.title, exact: true })).toBeVisible();
}

async function ordinaryUIUpdate(session: RecoverySession, child: OrdinaryIssue, field: "priority" | "assignee") {
  const page = session.page;
  let select: Locator;
  if (field === "priority") {
    await page.getByRole("button", { name: "High", exact: true }).click();
    const popup = page.locator('[data-slot="popover-content"]').filter({ has: page.locator('[data-picker-item]') });
    await expect(popup).toBeVisible();
    select = popup.getByRole("button", { name: "Medium", exact: true });
  } else {
    await page.getByRole("button", { name: "Unassigned", exact: true }).click();
    const popup = page.locator('[data-slot="popover-content"]').filter({ has: page.getByRole("textbox", { name: "Filter options", exact: true }) });
    await expect(popup).toBeVisible();
    await popup.getByRole("textbox", { name: "Filter options", exact: true }).fill(session.actor.name);
    select = popup.locator("button[data-picker-item]").filter({ has: page.getByText(session.actor.name, { exact: true }) });
  }
  await expect(select).toBeVisible();
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `/api/issues/${child.id}` && response.request().method() === "PUT", { timeout: 15_000 });
  const click = select.click();
  let response: Awaited<typeof pending>;
  try { [, response] = await Promise.all([click, pending]); }
  finally { await Promise.allSettled([click, pending]); }
  expect(response.status()).toBe(200);
  const input = mutationFact(session, response.request(), "ORDINARY_UI_OPTIONAL_VERSION_COMPATIBILITY", await response.request().allHeaders());
  expect(Object.hasOwn(input.body, "expected_revision")).toBe(false);
  const result = ordinaryIssue.parse(await response.json());
  return { input, result, scope: "Actual UI intentionally omitted expected_revision; this is not numeric UI CAS" };
}

export async function exerciseRecoveryOrdinary(browser: Browser, platform: RecoveryPlatform, info: TestInfo,
  evidence: RecoveryEvidence, first: "priority" | "assignee") {
  const fixture = await seedRecovery(evidence);
  const child = ordinaryIssue.parse(await fixture.creator.client.createIssue(`Ordinary recovery child ${randomUUID().slice(0, 8)}`, {
    parent_issue_id: fixture.issueId, priority: "low", assignee_type: "member", assignee_id: fixture.creator.id,
    description: "Ordinary child field updates preserve the collaboration root and history.",
  }));
  expect(child.collaboration == null).toBe(true); expect(child.parent_issue_id).toBe(fixture.issueId);
  const parent = parentFacts(await currentIssue(fixture));
  const activityBefore = await fixture.creator.client.requestJSON(`${issuePath(fixture.issueId)}/activity?limit=50`);
  evidence.checkpoint("ordinary_child_explicit_numeric_API_boundary", { child, parent, first, UI_contract: "optional expected_revision" });
  return withSessions(browser, platform, fixture, info, evidence, async (open) => {
    const prioritySession = await open(fixture.creator), assigneeSession = await open(fixture.creator);
    await openOrdinaryChild(prioritySession, child); await openOrdinaryChild(assigneeSession, child);
    const path = `/api/issues/${child.id}`;
    const priorityBody = { priority: "high", expected_revision: child.revision };
    const assigneeBody = { assignee_type: null, assignee_id: null, expected_revision: child.revision };
    const priority = new HeldRequest(prioritySession, path, "PUT", "EXPLICIT_PLATFORM_API_NUMERIC_CAS");
    const assignee = new HeldRequest(assigneeSession, path, "PUT", "EXPLICIT_PLATFORM_API_NUMERIC_CAS");
    const sends: Promise<HTTPFact>[] = [];
    await priority.install(); await assignee.install();
    try {
      sends.push(platformRequest(prioritySession, path, "PUT", priorityBody, undefined, fixture.workspace.id));
      sends.push(platformRequest(assigneeSession, path, "PUT", assigneeBody, undefined, fixture.workspace.id));
      for (const send of sends) void send.catch(() => undefined);
      const inputs = await Promise.all([priority.captured(), assignee.captured()]);
      for (const input of inputs) expect(input.body.expected_revision).toBe(child.revision);
      evidence.checkpoint("both_explicit_API_requests_captured_at_same_numeric_revision", { inputs });
      const winner = first === "priority" ? priority : assignee, loser = first === "priority" ? assignee : priority;
      const won = await winner.forward(), lost = await loser.forward();
      const responses = await Promise.all(sends);
      evidence.checkpoint("ordinary_API_numeric_CAS_actual_commit_order", { first, won, lost, platform_responses: responses });
      expect(won.status).toBe(200); expect(lost.status).toBe(409);
      expect(lost.body).toEqual({ error: "resource changed since it was loaded", code: "revision_conflict", resource_type: "issue",
        resource_id: child.id, expected_revision: child.revision, actual_revision: child.revision + 1 });
      const afterWinner = ordinaryIssue.parse(await fixture.creator.client.requestJSON(path));
      expect(afterWinner).toMatchObject({ revision: child.revision + 1, priority: first === "priority" ? "high" : "low",
        assignee_id: first === "priority" ? fixture.creator.id : null, parent_issue_id: fixture.issueId });
      expect(parentFacts(await currentIssue(fixture))).toEqual(parent);
      await priority.close(); await assignee.close();
      const freshBody = first === "priority" ? { ...assigneeBody, expected_revision: afterWinner.revision } : { ...priorityBody, expected_revision: afterWinner.revision };
      const fresh = await platformRequest(first === "priority" ? assigneeSession : prioritySession, path, "PUT", freshBody, undefined, fixture.workspace.id);
      expect(fresh.status).toBe(200);
      const afterFresh = ordinaryIssue.parse(fresh.body);
      expect(afterFresh).toMatchObject({ revision: child.revision + 2, priority: "high", assignee_id: null, parent_issue_id: fixture.issueId });
      evidence.checkpoint("separate_fresh_numeric_API_intent", { freshBody, fresh });
      await openOrdinaryChild(prioritySession, afterFresh);
      const compatibilityPriority = await ordinaryUIUpdate(prioritySession, afterFresh, "priority");
      const compatibilityAssignee = await ordinaryUIUpdate(prioritySession, compatibilityPriority.result, "assignee");
      const final = ordinaryIssue.parse(await fixture.creator.client.requestJSON(path));
      expect(final).toMatchObject({ priority: "medium", assignee_type: "member", assignee_id: fixture.creator.id,
        revision: child.revision + 4, parent_issue_id: fixture.issueId });
      await expect(prioritySession.page.getByRole("button", { name: "Medium", exact: true })).toBeVisible();
      expect(parentFacts(await currentIssue(fixture))).toEqual(parent);
      expect(await fixture.creator.client.requestJSON(`${issuePath(fixture.issueId)}/activity?limit=50`)).toEqual(activityBefore);
      evidence.set({ child_id: child.id, final_child: final, numeric_CAS: { planned_original_requests: 2, original_requests: inputs,
        winning_response: won, conflict: lost, fresh }, UI_compatibility: { planned_requests: 2, actual: [compatibilityPriority, compatibilityAssignee] },
        parent_business_and_execution_unchanged: true, no_UI_revision_injection: true });
      return { issue_id: fixture.issueId, child_id: child.id, order: first, status: "PASSED", explicit_API_CAS_requests: 2, UI_compatibility_requests: 2 };
    } finally {
      await priority.close(); await assignee.close(); await Promise.allSettled(sends);
    }
  });
}

function commentFacts(comment: CollaborationComment) {
  return { id: comment.id, issue_id: comment.issue_id, author_type: comment.author_type, author_id: comment.author?.user_id ?? null,
    author_agent_id: comment.author_agent?.agent_id ?? null, content: comment.content, parent_id: comment.parent_id,
    comment_revision: comment.comment_revision, attachment_ids: comment.attachment_ids, created_at: comment.created_at,
    updated_at: comment.updated_at, deleted_at: comment.deleted_at };
}
function compareUICommentTuple(left: CollaborationComment, right: CollaborationComment) {
  const time = new Date(left.created_at).getTime() - new Date(right.created_at).getTime();
  return time || left.id.localeCompare(right.id);
}

function persistedMicroseconds(timestamp: string): bigint {
  const match = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,6}))?(Z|[+-]\d{2}:\d{2})$/.exec(timestamp);
  if (!match) throw new Error("The persisted comment timestamp is not an exact supported microsecond instant");
  const seconds = Date.parse(`${match[1]}${match[3]}`);
  if (!Number.isFinite(seconds) || seconds % 1000 !== 0) throw new Error("The persisted comment timestamp has invalid whole seconds");
  return BigInt(seconds) * 1000n + BigInt((match[2] ?? "").padEnd(6, "0"));
}
function comparePersistedCommentTuple(left: CollaborationComment, right: CollaborationComment) {
  const leftAt = persistedMicroseconds(left.created_at), rightAt = persistedMicroseconds(right.created_at);
  return leftAt < rightAt ? -1 : leftAt > rightAt ? 1 : left.id.localeCompare(right.id);
}

async function writeBatchComment(fixture: OfflineHandoffFixture, content: string, evidence: RecoveryEvidence, parentId: string | null = null) {
  const current = await currentIssue(fixture), requestId = randomUUID();
  const observed = observations(current);
  const input = { content, parent_id: parentId, suppressed_target_ids: [], target_versions: [],
    execution_view_version: 6, expected_lifecycle_epoch: observed.expected_lifecycle_epoch,
    expected_participant_epoch: observed.expected_participant_epoch, expected_account_epoch: observed.expected_account_epoch };
  const receipt = CollaborationCommentReceiptSchema.parse(await fixture.creator.client.requestJSON(`${issuePath(fixture.issueId)}/comments`, {
    method: "POST", headers: { "Idempotency-Key": requestId }, body: input,
  }));
  expect(receipt.status).toBe(201); expect(receipt.event_id).toBeTruthy(); expect(receipt.event_seq).toBeTruthy(); expect(receipt.targets).toEqual([]);
  const row = { request_id: requestId, input, input_sha256: digest(input), receipt };
  evidence.checkpoint("one_original_comment_committed", row);
  return row;
}

async function authenticatedSocket(session: RecoverySession, after: string, previousId?: string): Promise<SocketFact> {
  const find = () => [...session.sockets].reverse().find((socket) => socket.created_at >= after && socket.id !== previousId && socket.closed_at === null &&
    socket.frames.some((frame) => frame.direction === "received" && frame.type === "subscribe_ack" && frame.scope === "issue" && frame.id === session.fixture.issueId));
  await expect.poll(() => find() !== undefined, { timeout: 35_000, message: "A distinct live application socket must authenticate and subscribe to the issue" }).toBe(true);
  const socket = find()!;
  const handshake = session.handshakes.find((item) => item.request_id === socket.id);
  expect(handshake?.status).toBe(101);
  const sent = socket.frames.findIndex((frame) => frame.direction === "sent" && frame.type === "subscribe" && frame.scope === "issue" && frame.id === session.fixture.issueId);
  const acknowledged = socket.frames.findIndex((frame) => frame.direction === "received" && frame.type === "subscribe_ack" && frame.scope === "issue" && frame.id === session.fixture.issueId);
  expect(sent).toBeGreaterThanOrEqual(0); expect(acknowledged).toBeGreaterThan(sent);
  if (session.platform === "Desktop") {
    const auth = socket.frames.findIndex((frame) => frame.direction === "sent" && frame.type === "auth");
    const authAck = socket.frames.findIndex((frame) => frame.direction === "received" && frame.type === "auth_ack");
    expect(auth).toBe(0); expect(authAck).toBeGreaterThan(auth); expect(sent).toBeGreaterThan(authAck);
  } else {
    // Cookie authority is established before the actual successful upgrade.
    // This server deliberately sends auth_ack only for first-message token auth.
    expect(socket.frames.some((frame) => frame.direction === "sent" && frame.type === "auth")).toBe(false);
    expect(handshake!.at <= socket.frames[sent]!.at).toBe(true);
    const principal = await platformRequest(session, "/api/me");
    expect(principal.status).toBe(200); expect(object.parse(principal.body).id).toBe(session.actor.id);
  }
  return socket;
}

async function disconnectObserver(session: RecoverySession, socket: SocketFact, evidence: RecoveryEvidence, phase: string) {
  await Promise.allSettled([...session.pendingReads]);
  await session.context.setOffline(true);
  const close = await session.page.evaluate(() => (window as unknown as { pay241RecoveryTransport: {
    closeOpenSockets(): { count: number; all_native: boolean } } }).pay241RecoveryTransport.closeOpenSockets());
  expect(close.count).toBeGreaterThanOrEqual(0); expect(close.all_native).toBe(true);
  await expect.poll(() => socket.closed_at !== null, { message: "The original physical browser socket must close" }).toBe(true);
  expect(await session.page.evaluate(() => navigator.onLine)).toBe(false);
  evidence.checkpoint(phase, { old_socket_id: socket.id, old_socket_closed_at: socket.closed_at, physical_close: close,
    observer_HTTP_offline: true, application_onclose_and_reconnect_handlers_preserved: true });
}

async function walkCommentPages(session: RecoverySession, expected: CollaborationComment[], after: string, evidence: RecoveryEvidence, phase: string) {
  const total = expected.length;
  await expect(session.page.locator("#discussion [data-comment-total]")).toHaveText(String(total));
  const head = () => [...session.pages].reverse().find((item) => item.at >= after && item.cursor === null && item.page.total === total);
  await expect.poll(() => head() !== undefined, { message: "The current application must read the full batch's first authorized page" }).toBe(true);
  const chain: CommentPageFact[] = [head()!], cursors = new Set<string>();
  while (chain.at(-1)!.page.next_cursor !== null) {
    const previous = chain.at(-1)!, cursor = previous.page.next_cursor!;
    expect(cursors.has(cursor)).toBe(false); cursors.add(cursor);
    const find = () => session.pages.find((item) => item.at >= previous.at && item.cursor === cursor && item.page.total === total);
    const more = session.page.locator("#discussion").getByRole("button", { name: "Load more", exact: true });
    if (!find() && await more.isVisible()) { await expect(more).toBeEnabled(); await more.click(); }
    await expect.poll(() => find() !== undefined, { message: "Every actual Load more/refetch cursor must return its authorized page" }).toBe(true);
    chain.push(find()!);
    evidence.checkpoint(`${phase}_next_page`, { page_number: chain.length, page: chain.at(-1) });
  }
  expect(chain.length).toBeGreaterThan(1);
  const rows = chain.flatMap((entry) => entry.page.items);
  expect(rows).toHaveLength(total);
  expect(new Set(rows.map((row) => row.id)).size).toBe(total);
  for (const entry of chain) {
    expect(entry.limit).toBe(50); expect(entry.max_bytes).toBe(65536); expect(entry.page.total).toBe(total);
    expect(entry.page.returned).toBe(entry.page.items.length); expect(entry.response_bytes).toBe(entry.page.bytes);
    expect(entry.response_bytes).toBeLessThanOrEqual(entry.max_bytes);
    expect(entry.page.items.map(commentFacts)).toEqual([...entry.page.items].sort(comparePersistedCommentTuple).map(commentFacts));
    for (const row of entry.page.items) expect(row.content_state).toBe(row.deleted_at === null ? "complete" : "deleted");
  }
  // Each page is chronological but pages arrive newest first. The quiescent
  // complete pass is compared in canonical tuple order, never concatenation order.
  expect([...rows].sort(comparePersistedCommentTuple).map(commentFacts)).toEqual([...expected].sort(comparePersistedCommentTuple).map(commentFacts));
  const grouped = deriveCollaborationCommentThreads([...expected].sort(compareUICommentTuple)).flatMap((thread) => thread.items.map((item) => item.comment.id));
  const dom = session.page.locator('#discussion li[id^="comment-"]');
  await expect(dom).toHaveCount(total);
  expect(await dom.evaluateAll((elements) => elements.map((element) => element.id.slice("comment-".length)))).toEqual(grouped);
  for (const row of expected) {
    const element = session.page.locator(`[id="comment-${row.id}"]`);
    if (row.deleted_at) await expect(element.getByText("Comment deleted", { exact: true })).toBeVisible();
    else await expect(element.getByText(row.content, { exact: true })).toBeVisible();
  }
  await expect(session.page.locator("#discussion").getByRole("button", { name: "Load more", exact: true })).toHaveCount(0);
  expect(session.read_errors).toEqual([]);
  evidence.checkpoint(`${phase}_complete_quiescent_pass`, { chain, all_unique_persisted_ids: rows.map((row) => row.id),
    canonical_tuple_order: [...rows].sort(comparePersistedCommentTuple).map((row) => row.id), DOM_thread_order: grouped,
    persisted_tuple_precision: "microseconds", DOM_grouping_precision: "current UI millisecond comparator",
    denominator: total, no_duplicate_raw_page_IDs: true, all_complete_bodies_or_tombstones: true,
    cross_request_snapshot_claim: false });
  return chain;
}

export async function exerciseRecoveryBatch(browser: Browser, platform: RecoveryPlatform, info: TestInfo, evidence: RecoveryEvidence) {
  const fixture = await seedRecovery(evidence);
  const prefix = `S3-${platform}-${randomUUID().slice(0, 8)}`;
  const baseline = await writeBatchComment(fixture, `${prefix} baseline retained before disconnect`, evidence);
  return withSessions(browser, platform, fixture, info, evidence, async (open) => {
    const observer = await open(fixture.recipient);
    await expect(observer.page.locator(`[id="comment-${baseline.receipt.comment_id}"]`)).toBeVisible();
    const before = await currentIssue(fixture);
    const originalSocket = await authenticatedSocket(observer, "0000");
    await disconnectObserver(observer, originalSocket, evidence, "observer_physically_disconnected_before_batch");
    await Promise.allSettled([...observer.pendingReads]);
    const writesBefore = observer.writes.length;
    const batch = [];
    for (let index = 1; index <= 55; index++) batch.push(await writeBatchComment(fixture, `${prefix} comment ${String(index).padStart(2, "0")}`, evidence));
    expect(new Set(batch.map((row) => row.request_id)).size).toBe(55);
    expect(new Set(batch.map((row) => row.receipt.comment_id)).size).toBe(55);
    expect(new Set(batch.map((row) => row.receipt.event_id)).size).toBe(55);
    const edited = batch[3]!, deleted = batch[10]!;
    const editKey = randomUUID(), deleteKey = randomUUID();
    const edit = CollaborationCommentReceiptSchema.parse(await fixture.creator.client.requestJSON(`${issuePath(fixture.issueId)}/comments/${edited.receipt.comment_id}`, {
      method: "PATCH", headers: { "Idempotency-Key": editKey }, body: { content: `${prefix} comment 04 edited while offline`, expected_comment_revision: edited.receipt.comment_revision },
    }));
    const deletion = CollaborationCommentReceiptSchema.parse(await fixture.creator.client.requestJSON(`${issuePath(fixture.issueId)}/comments/${deleted.receipt.comment_id}`, {
      method: "DELETE", headers: { "Idempotency-Key": deleteKey }, body: { expected_comment_revision: deleted.receipt.comment_revision },
    }));
    expect(edit.status).toBe(200); expect(deletion.status).toBe(200); expect(edit.comment_revision).toBe("2"); expect(deletion.comment_revision).toBe("2");
    const reply = await writeBatchComment(fixture, `${prefix} reply preserves the deleted parent`, evidence, deleted.receipt.comment_id);
    const fieldKey = randomUUID(), issueBeforeField = await currentIssue(fixture);
    const field = await fixture.creator.client.requestJSON(issuePath(fixture.issueId), { method: "PATCH", headers: { "Idempotency-Key": fieldKey },
      body: { ...observations(issueBeforeField), priority: "high" } });
    const expected: CollaborationComment[] = [];
    for (const row of [baseline, ...batch, reply]) expected.push(CollaborationCommentSchema.parse(await fixture.creator.client.requestJSON(
      `${issuePath(fixture.issueId)}/comments/${row.receipt.comment_id}`)));
    expect(expected).toHaveLength(57); expect(expected.find((row) => row.id === deleted.receipt.comment_id)?.deleted_at).not.toBeNull();
    const watermark = await currentIssue(fixture);
    expect(watermark.priority).toBe("high"); expect(executionFacts(watermark)).toEqual(executionFacts(before));
    evidence.checkpoint("writer_quiescent_complete_batch_watermark", { batch_unique_comments: 55, additional_baseline: baseline,
      additional_reply: reply, edits: [{ request_id: editKey, receipt: edit }], deletions: [{ request_id: deleteKey, receipt: deletion }],
      final_field: { request_id: fieldKey, receipt: field }, expected_rows: expected.map(commentFacts), watermark,
      persistent_ID_denominator: 57, original_batch_denominator: 55 });
    const reconnectAt = now();
    await observer.context.setOffline(false);
    const recoveredSocket = await authenticatedSocket(observer, reconnectAt, originalSocket.id);
    evidence.checkpoint("new_application_socket_reauthenticated", { old_socket_id: originalSocket.id, new_socket: recoveredSocket,
      authentication: platform === "Web" ? "real password Cookie validated before 101 upgrade; current principal and authorized issue subscribe_ack" : "first-message token auth_ack before issue subscribe and subscribe_ack" });
    const firstPass = await walkCommentPages(observer, expected, reconnectAt, evidence, "first_recovery");
    const current = await platformRequest(observer, issuePath(fixture.issueId));
    expect(current.status).toBe(200);
    expect(CollaborationIssueSchema.parse(current.body)).toMatchObject({ priority: "high", collaboration_revision: watermark.collaboration_revision,
      handler: { user_id: fixture.recipient.id }, title: fixture.title });
    await Promise.allSettled([...observer.pendingReads]);
    expect(observer.writes).toHaveLength(writesBefore);
    for (const original of [baseline, ...batch, reply]) {
      const recovered = CollaborationCommentReceiptSchema.parse(await fixture.creator.client.requestJSON(
        `${issuePath(fixture.issueId)}/comments/operations/${original.request_id}`));
      expect(recovered).toEqual(original.receipt);
    }
    evidence.checkpoint("all_original_comment_GET_receipts_retained", { original_request_ids: [baseline, ...batch, reply].map((row) => row.request_id),
      original_event_ids: [baseline, ...batch, reply].map((row) => row.receipt.event_id), count: 57 });
    await disconnectObserver(observer, recoveredSocket, evidence, "second_disconnect_no_new_intent");
    const secondReconnectAt = now();
    await observer.context.setOffline(false);
    const secondSocket = await authenticatedSocket(observer, secondReconnectAt, recoveredSocket.id);
    const secondPass = await walkCommentPages(observer, expected, secondReconnectAt, evidence, "second_recovery");
    await Promise.allSettled([...observer.pendingReads]);
    expect(observer.writes).toHaveLength(writesBefore);
    const finalRows = [];
    for (const row of expected) finalRows.push(CollaborationCommentSchema.parse(await fixture.creator.client.requestJSON(
      `${issuePath(fixture.issueId)}/comments/${row.id}`)));
    expect(finalRows.map(commentFacts)).toEqual(expected.map(commentFacts));
    expect(executionFacts(await currentIssue(fixture))).toEqual(executionFacts(before));
    evidence.set({ batch_denominator: 55, control_denominator: 2, persistent_ID_denominator: 57, writer_ledger: batch,
      first_pass_pages: firstPass.length, second_pass_pages: secondPass.length, distinct_socket_ids: [originalSocket.id, recoveredSocket.id, secondSocket.id],
      observer_recovery_mutations: 0, final_rows_sha256: digest(finalRows.map(commentFacts)), no_recovery_dispatch: true,
      no_comment_loss_or_duplicate_raw_page_IDs: true });
    return { issue_id: fixture.issueId, status: "PASSED", batch_comments: 55, controls: 2, persistent_ids: 57, socket_ids: [originalSocket.id, recoveredSocket.id, secondSocket.id] };
  });
}
