import { createHash, randomUUID } from "node:crypto";
import { expect, type Browser, type Locator, type Page, type Request, type Response, type TestInfo } from "@playwright/test";
import pg from "pg";
import { z } from "zod";
import { CollaborationActivitySchema, CollaborationIssueSchema, CollaborationOperationSchema,
  type CollaborationActivityEntry, type CollaborationIssue, type CollaborationOperation } from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";
import { revealCollaborationEditingAction } from "./collaboration-editing";
import { revealCollaborationDisclosure } from "./collaboration-disclosures";
import { nativeScreenshot } from "./collaboration-native";
import { RecoveryEvidence, withSessions, type RecoveryActor, type RecoveryPlatform, type RecoverySession, type RecoverySurface } from "./collaboration-recovery";

const now = () => new Date().toISOString();
const sha = (value: string | Buffer) => createHash("sha256").update(value).digest("hex");
const timestamp = (value: string) => BigInt(Date.parse(value.replace(/\.\d+/, ""))) * 1_000_000n +
  BigInt((value.match(/\.(\d+)/)?.[1] ?? "").padEnd(9, "0"));
function utcTimestamp(value: string) {
  const nanoseconds = timestamp(value);
  const fraction = (nanoseconds % 1_000_000_000n + 1_000_000_000n) % 1_000_000_000n;
  const wholeSecond = new Date(Number((nanoseconds - fraction) / 1_000_000n)).toISOString().slice(0, -5);
  return `${wholeSecond}.${fraction.toString().padStart(9, "0")}Z`;
}
const rootPath = (issueId: string) => `/api/collaboration/issues/${issueId}`;
const roles = ["product", "developer", "qa"] as const;
type Role = typeof roles[number];
type DocumentVersion = "V1" | "V2" | "V3";
const versions: DocumentVersion[] = ["V1", "V2", "V3"];
const paragraphs = (version: DocumentVersion) => [
  "Impact: A customer payment succeeds while the order still displays awaiting payment. The same order must retain one payment and one fulfillment.",
  "Reproduction: Create one sandbox order, accept its successful payment callback, then inspect the order and fulfillment records.",
  {
    V1: "Acceptance criteria V1: A successful callback changes the original order to paid and preserves the original payment identity.",
    V2: "Acceptance criteria V2: A successful callback changes the original order to paid; three deliveries of that same callback cause no duplicate charge or fulfillment.",
    V3: "Acceptance criteria V3: A successful callback changes the original order to paid; three duplicate deliveries remain idempotent, and a callback delayed by 45 seconds converges to paid with its recovery history retained.",
  }[version],
];
const description = (version: DocumentVersion) => paragraphs(version).join("\n\n");

export const requirementsHistoryJourneys = (["Web", "Desktop"] as const).map((platform) =>
  `${platform} three authorized sessions synchronize acceptance criteria and reopen complete V1 V2 V3 history`);

export function requirementsHistoryEnvironmentReady() {
  return process.env.PAY241_TC02_REQUIREMENTS_ACCEPTANCE === "1" && process.env.E2E_PASSWORD_AUTH === "1" &&
    [process.env.COLLABORATION_E2E_API_URL, process.env.NEXT_PUBLIC_API_URL, process.env.DATABASE_URL,
      process.env.TASK_DATABASE_NAME, process.env.COLLABORATION_ELECTRON_RENDERER_URL,
      process.env.COLLABORATION_ELECTRON_SOURCE_ROOT].every(Boolean);
}

interface Fixture {
  surface: RecoverySurface;
  workspace: { id: string; slug: string; name: string };
  actors: Record<Role, RecoveryActor>;
  creationKey: string;
  inviteKeys: string[];
}

// Credentials are synthetic and stay in closures; retained evidence contains only account identities.
async function passwordActor(apiUrl: string, role: Role, suffix: string): Promise<RecoveryActor> {
  const client = new TestApiClient(), username = `tc02${role}${suffix}`;
  const password = `TC02-${randomUUID()}!`, name = `TC02 ${role} ${suffix}`;
  const user = await client.registerPassword(username, password, name);
  await client.requestJSON("/api/me/onboarding", { method: "PATCH", body: { questionnaire: { source: ["friends_colleagues"] } } });
  const onboarded = z.object({ id: z.uuid(), onboarded_at: z.string().nullable() }).parse(
    await client.requestJSON("/api/me/onboarding/complete", { method: "POST", body: {} }));
  expect(onboarded.id).toBe(user.id); expect(onboarded.onboarded_at).not.toBeNull();
  return { client, id: user.id, name,
    async login() { expect((await client.loginPassword(username, password)).id).toBe(user.id); return client.getToken()!; },
    async authenticateWeb(page: Page) {
      const response = await page.request.post(`${apiUrl}/auth/login`, { data: { username, password } });
      expect(response.status()).toBe(200);
      expect(z.object({ user: z.object({ id: z.uuid() }) }).parse(await response.json()).user.id).toBe(user.id);
      await page.addInitScript(() => {
        localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false");
      });
    } };
}

function observations(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("The product actor must have current workflow authority");
  return { expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
}

async function seed(platform: RecoveryPlatform, evidence: RecoveryEvidence): Promise<Fixture> {
  if (!requirementsHistoryEnvironmentReady()) throw new Error("TC-02 requires its explicit owned password allocation");
  const apiUrl = process.env.COLLABORATION_E2E_API_URL!;
  for (const value of [apiUrl, process.env.NEXT_PUBLIC_API_URL!, process.env.DATABASE_URL!]) {
    expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
  }
  expect(new URL(apiUrl).origin).toBe(new URL(process.env.NEXT_PUBLIC_API_URL!).origin);
  const database = decodeURIComponent(new URL(process.env.DATABASE_URL!).pathname.slice(1));
  expect(database).toBe(process.env.TASK_DATABASE_NAME);
  expect(["", "multica", "postgres", "template0", "template1"]).not.toContain(database);
  const configuration = await fetch(`${apiUrl}/api/config`);
  expect(configuration.status).toBe(200); expect(await configuration.json()).toMatchObject({ auth_mode: "password" });
  const suffix = randomUUID().replaceAll("-", "").slice(0, 12);
  const actors = { product: await passwordActor(apiUrl, "product", suffix), developer: await passwordActor(apiUrl, "developer", suffix),
    qa: await passwordActor(apiUrl, "qa", suffix) };
  expect(new Set(roles.map((role) => actors[role].id)).size).toBe(3);
  const workspace = z.object({ id: z.uuid(), slug: z.string(), name: z.string() }).parse(
    await actors.product.client.requestJSON("/api/workspaces", { method: "POST", body: {
      name: `TC02 requirements ${platform} ${suffix}`, slug: `tc02-${platform.toLowerCase()}-${suffix}`, issue_prefix: `R${suffix.slice(0, 8)}`.toUpperCase(),
    } }));
  actors.product.client.setWorkspaceId(workspace.id); actors.product.client.setWorkspaceSlug(workspace.slug);
  expect(await actors.product.client.requestJSON("/api/collaboration/creation-options")).toMatchObject({ creation_enabled: true });
  const creationKey = randomUUID();
  let issue = CollaborationIssueSchema.parse(await actors.product.client.requestJSON("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": creationKey }, body: {
      source_workspace_id: workspace.id, handler_user_id: actors.product.id, title: `TC02 payment acceptance ${suffix}`,
      description: description("V1"), attachment_ids: [],
    },
  }));
  const inviteKeys: string[] = [];
  for (const role of ["developer", "qa"] as const) {
    const key = randomUUID(); inviteKeys.push(key);
    await actors.product.client.requestJSON(`${rootPath(issue.id)}/participants`, { method: "POST", headers: { "Idempotency-Key": key },
      body: { ...observations(issue), target_user_id: actors[role].id } });
    issue = CollaborationIssueSchema.parse(await actors.product.client.requestJSON(rootPath(issue.id)));
    expect(await actors[role].client.getWorkspaces()).toEqual([]);
    expect(CollaborationIssueSchema.parse(await actors[role].client.requestJSON(rootPath(issue.id))).capabilities.read).toBe(true);
  }
  expect(issue.description).toBe(description("V1"));
  evidence.set({ issue_id: issue.id, source_workspace_id: workspace.id, api_url: apiUrl, database,
    actors: roles.map((role) => ({ role, id: actors[role].id, name: actors[role].name })),
    setup: { creation: "public_product_API", creation_key: creationKey, invite_keys: inviteKeys, events: ["created", "participant_added", "participant_added"] },
    descriptions: Object.fromEntries(versions.map((version) => [version, description(version)])),
    real_payment_execution: false, real_model_execution: false, installed_main_execution: false });
  return { actors, workspace, creationKey, inviteKeys,
    surface: { apiUrl, issueId: issue.id, title: issue.title, accountPath: `/account?view=participating&issue=${issue.id}` } };
}

interface DetailRead { at: string; body_sha256: string; description: string; collaboration_revision: string; event_seq: string | null }
interface Write { at: string; method: string; path: string; request_id: string | null; body: unknown }

class Traffic {
  readonly details: DetailRead[] = [];
  readonly activities: z.infer<typeof CollaborationActivitySchema>[] = [];
  readonly writes: Write[] = [];
  readonly errors: string[] = [];
  constructor(readonly session: RecoverySession) {
    session.page.on("response", this.response); session.page.on("request", this.request);
  }
  private track(promise: Promise<void>) {
    const guarded = promise.catch((error) => { this.errors.push(error instanceof Error ? error.name : "traffic_error"); });
    this.session.pendingReads.add(guarded); void guarded.finally(() => this.session.pendingReads.delete(guarded));
  }
  private request = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (!path.startsWith("/api/") || ["GET", "HEAD", "OPTIONS"].includes(request.method())) return;
    this.track((async () => {
      const headers = await request.allHeaders();
      this.writes.push({ at: now(), path, method: request.method(), request_id: headers["idempotency-key"] ?? null,
        body: request.postData() === null ? null : request.postDataJSON() });
    })());
  };
  private response = (response: Response) => {
    const url = new URL(response.url()), path = rootPath(this.session.fixture.issueId);
    if (response.request().method() !== "GET" || response.status() !== 200 || ![path, `${path}/activity`].includes(url.pathname)) return;
    // Diagnostic limit=1 requests are retained separately, never mistaken for a UI Activity read.
    if (url.searchParams.get("limit") === "1") return;
    this.track((async () => {
      const bytes = await response.body();
      if (url.pathname === path) {
        const issue = CollaborationIssueSchema.parse(JSON.parse(bytes.toString()));
        this.details.push({ at: now(), body_sha256: sha(bytes), description: issue.description,
          collaboration_revision: issue.collaboration_revision, event_seq: issue.realtime?.event_seq ?? null });
      } else this.activities.push(CollaborationActivitySchema.parse(JSON.parse(bytes.toString())));
    })());
  };
  async settle() { await Promise.allSettled([...this.session.pendingReads]); expect(this.errors).toEqual([]); }
  close() { this.session.page.off("response", this.response); this.session.page.off("request", this.request); }
}

async function documentDOM(content: Locator, version: DocumentVersion) {
  await expect(content).toBeVisible();
  await expect(content.locator(":scope > p")).toHaveText(paragraphs(version));
  return { paragraphs: await content.locator(":scope > p").allTextContents(), full_text: await content.innerText() };
}

async function observeVersion(session: RecoverySession, traffic: Traffic, version: DocumentVersion, revision: string, evidence: RecoveryEvidence) {
  await expect.poll(() => traffic.details.some((item) => item.description === description(version) && item.collaboration_revision === revision),
    { message: `${session.actor.name} must receive the exact current document and revision through its UI request` }).toBe(true);
  const dom = await documentDOM(session.page.locator("#description [data-rich-content]"), version);
  const read = traffic.details.findLast((item) => item.description === description(version) && item.collaboration_revision === revision)!;
  evidence.checkpoint("actual_UI_description_and_response_revision", { actor_id: session.actor.id, platform: session.platform,
    version, observation: read, DOM: dom, revision_shown_as_UI_label: false });
}

async function sessionGet(session: RecoverySession, path: string) {
  const result = await session.page.evaluate(async ({ apiUrl, path, platform }) => {
    const headers: Record<string, string> = {};
    if (platform === "Desktop") {
      const token = localStorage.getItem("multica_token");
      if (!token) throw new Error("The native password session is absent");
      headers.Authorization = `Bearer ${token}`;
    }
    const response = await fetch(`${apiUrl}${path}`, { headers, credentials: "include" });
    return { status: response.status, body: await response.json() as unknown };
  }, { apiUrl: session.fixture.apiUrl, path, platform: session.platform });
  expect(result.status).toBe(200); return result.body;
}

interface EditFact { request_id: string; body: Record<string, unknown>; receipt: CollaborationOperation; from: DocumentVersion; to: DocumentVersion }
async function edit(session: RecoverySession, from: DocumentVersion, to: DocumentVersion, evidence: RecoveryEvidence): Promise<EditFact> {
  await (await revealCollaborationEditingAction(session.page, "Edit issue")).click();
  const form = session.page.getByRole("form", { name: "Edit issue", exact: true });
  await expect(form).toBeVisible(); await expect(form.getByLabel("Description", { exact: true })).toHaveValue(description(from));
  await form.getByLabel("Description", { exact: true }).fill(description(to));
  const [response] = await Promise.all([
    session.page.waitForResponse((item) => new URL(item.url()).pathname === rootPath(session.fixture.issueId) && item.request().method() === "PATCH"),
    form.getByRole("button", { name: "Confirm", exact: true }).click(),
  ]);
  const request = response.request(), headers = await request.allHeaders();
  const requestId = z.string().min(1).parse(headers["idempotency-key"]);
  const body = z.record(z.string(), z.unknown()).parse(request.postDataJSON());
  const raw: unknown = await response.json();
  evidence.checkpoint("original_actual_Edit_issue_response", { from, to, request_id: requestId, body, status: response.status(), response: raw });
  expect(response.status()).toBe(200);
  const receipt = CollaborationOperationSchema.parse(raw); expect(receipt.code).toBe("issue_edited");
  expect(body.description).toBe(description(to));
  expect(BigInt(receipt.collaboration_revision)).toBe(BigInt(z.string().parse(body.expected_collaboration_revision)) + 1n);
  await expect(form).toHaveCount(0);
  const recovered = CollaborationOperationSchema.parse(await sessionGet(session, `${rootPath(session.fixture.issueId)}/operations/${requestId}`));
  expect(recovered).toEqual(receipt);
  return { request_id: requestId, body, receipt, from, to };
}

const storedEvent = z.object({ id: z.uuid(), issue_id: z.uuid(), event_seq: z.string(), actor_user_id: z.uuid(), kind: z.string(),
  created_at: z.string(), payload: z.record(z.string(), z.unknown()) });
const storedOperation = z.object({ id: z.uuid(), actor_user_id: z.uuid(), request_id: z.string(), kind: z.string(), result_code: z.string(),
  result: z.unknown(), completed_at: z.string() });
interface Audit { events: z.infer<typeof storedEvent>[]; operations: z.infer<typeof storedOperation>[]; passive: Record<string, unknown[]> }
const passiveQueries: Record<string, string> = {
  commands: "SELECT c.id FROM collaboration_command c JOIN collaboration_session s ON s.id=c.session_id WHERE s.issue_id=$1 ORDER BY c.id",
  runs: "SELECT r.id FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id WHERE s.issue_id=$1 ORDER BY r.id",
  dispatches: "SELECT id FROM agent_task_queue WHERE issue_id=$1 ORDER BY id",
  comments: "SELECT id FROM comment WHERE issue_id=$1 ORDER BY id",
  notifications: "SELECT id,recipient_user_id,event_seq::text,kind,read_at::text,archived_at::text FROM collaboration_notification WHERE issue_id=$1 ORDER BY id",
};

async function audit(fixture: Fixture, evidence: RecoveryEvidence, phase: string): Promise<Audit> {
  const database = new pg.Client({ connectionString: process.env.DATABASE_URL!, connectionTimeoutMillis: 5_000 });
  try {
    await database.connect(); await database.query("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY");
    await database.query("SET LOCAL statement_timeout='5s'"); await database.query("SET LOCAL idle_in_transaction_session_timeout='10s'");
    await database.query("SET LOCAL TIME ZONE 'UTC'");
    expect((await database.query("SELECT current_database() AS database,current_setting('transaction_read_only') AS read_only")).rows)
      .toEqual([{ database: process.env.TASK_DATABASE_NAME, read_only: "on" }]);
    const events = z.array(storedEvent).parse((await database.query(
      "SELECT id,issue_id,event_seq::text,actor_user_id,kind,created_at::text,payload FROM collaboration_event WHERE issue_id=$1 ORDER BY event_seq", [fixture.surface.issueId])).rows);
    const operations = z.array(storedOperation).parse((await database.query(
      "SELECT id,actor_user_id,request_id,kind,result_code,result,completed_at::text FROM collaboration_operation WHERE scope_kind='issue' AND scope_id=$1 ORDER BY id", [fixture.surface.issueId])).rows);
    const passive: Record<string, unknown[]> = {};
    for (const [name, sql] of Object.entries(passiveQueries)) passive[name] = (await database.query(sql, [fixture.surface.issueId])).rows;
    await database.query("COMMIT");
    const result = { events, operations, passive };
    evidence.checkpoint(phase, { database: process.env.TASK_DATABASE_NAME, read_only: true, ...result });
    return result;
  } finally { await database.end(); }
}

async function completeHistory(session: RecoverySession, evidence: RecoveryEvidence) {
  const entries: CollaborationActivityEntry[] = [], cursors = new Set<string>();
  let cursor: string | null = null;
  do {
    const path = `${rootPath(session.fixture.issueId)}/activity?limit=1${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`;
    const page = CollaborationActivitySchema.parse(await sessionGet(session, path));
    expect(page.issue_id).toBe(session.fixture.issueId); expect(page.items).toHaveLength(1);
    evidence.checkpoint("complete_session_activity_page", { actor_id: session.actor.id, cursor, limit: 1, page });
    entries.push(...page.items); cursor = page.next_cursor;
    expect(entries.length, "The five registered creation/invitation/edit events must exhaust history").toBeLessThanOrEqual(5);
    if (cursor !== null) { expect(cursors.has(cursor)).toBe(false); cursors.add(cursor); }
  } while (cursor !== null);
  expect(entries).toHaveLength(5); expect(new Set(entries.map((entry) => entry.id)).size).toBe(5);
  expect(new Set(entries.map((entry) => entry.event_seq)).size).toBe(5);
  expect(entries.map((entry) => entry.kind)).toEqual(["issue_edited", "issue_edited", "participant_added", "participant_added", "created"]);
  return entries;
}

function verifyPersistedHistory(fixture: Fixture, baseline: Audit, final: Audit, edits: EditFact[], entries: CollaborationActivityEntry[]) {
  const times: { event_id: string; API_raw: string; SQL_raw: string; UTC: string; UTC_nanoseconds: string }[] = [];
  expect(final.events.slice(0, baseline.events.length)).toEqual(baseline.events);
  expect(final.events).toHaveLength(5); expect(final.operations).toHaveLength(4);
  expect(final.passive).toEqual(baseline.passive);
  expect(final.events.map((event) => event.id).reverse()).toEqual(entries.map((entry) => entry.id));
  for (const [index, event] of final.events.entries()) {
    expect(event.event_seq).toBe(String(index + 1));
    const entry = entries.find((item) => item.id === event.id)!;
    expect(entry.event_seq).toBe(event.event_seq); expect(entry.actor?.user_id).toBe(event.actor_user_id);
    expect(entry.actor?.name).toBe(fixture.actors.product.name);
    expect(timestamp(entry.created_at)).toBe(timestamp(event.created_at));
    expect(utcTimestamp(entry.created_at)).toBe(utcTimestamp(event.created_at));
    times.push({ event_id: entry.id, API_raw: entry.created_at, SQL_raw: event.created_at,
      UTC: utcTimestamp(entry.created_at), UTC_nanoseconds: timestamp(entry.created_at).toString() });
    if (event.kind !== "issue_edited") continue;
    const fact = edits[index - 3]!;
    expect(event.actor_user_id).toBe(fixture.actors.product.id); expect(event.payload.request_id).toBe(fact.request_id);
    const before = z.object({ description: z.string() }).parse(event.payload.before);
    const after = z.object({ description: z.string() }).parse(event.payload.after);
    expect(before.description).toBe(description(fact.from)); expect(after.description).toBe(description(fact.to));
    expect(entry.details_available).toBe(true);
    expect(entry.changes).toEqual([{ field: "description", before: before.description, after: after.description }]);
    const operation = final.operations.filter((item) => item.request_id === fact.request_id);
    expect(operation).toHaveLength(1); expect(operation[0]).toMatchObject({ actor_user_id: fixture.actors.product.id, kind: "edit_issue", result_code: "issue_edited" });
    expect(event.payload.receipt).toEqual(operation[0]!.result);
    expect(CollaborationOperationSchema.parse(operation[0]!.result)).toMatchObject(fact.receipt);
  }
  for (const original of baseline.operations) expect(final.operations.find((item) => item.id === original.id)).toEqual(original);
  return times;
}

async function activityDOM(session: RecoverySession, traffic: Traffic, entries: CollaborationActivityEntry[], edits: EditFact[], info: TestInfo, evidence: RecoveryEvidence) {
  const drawer = await revealCollaborationDisclosure(session.page, "Activity");
  const region = drawer.getByRole("region", { name: "Activity", exact: true }); await expect(region).toBeVisible();
  // This five-event history fits the production page size. Retained responses
  // can include older pre-edit pages; only the latest current page backs the UI.
  await expect.poll(() => traffic.activities.at(-1)?.items).toEqual(entries);
  expect(traffic.activities.at(-1)?.next_cursor).toBeNull();
  const more = region.getByRole("button", { name: "Load more", exact: true });
  await expect(more).toHaveCount(0);
  const rows = region.locator(":scope > ol > li"); await expect(rows).toHaveCount(entries.length);
  for (const [index, entry] of entries.entries()) {
    const row = rows.nth(index);
    await expect(row.locator("time")).toHaveAttribute("datetime", entry.created_at);
    await expect(row.getByText(entry.actor!.name, { exact: true })).toBeVisible();
    if (entry.kind !== "issue_edited") continue;
    const change = entry.changes.find((item) => item.field === "description")!;
    const fact = edits.find((edit) => change.before === description(edit.from) && change.after === description(edit.to));
    if (!fact) throw new Error("The visible edit event does not match either original description edit");
    await expect(row.getByText("Issue edited", { exact: true })).toBeVisible();
    const contents = row.locator("[data-rich-content]"); await expect(contents).toHaveCount(2);
    const before = await documentDOM(contents.nth(0), fact.from), after = await documentDOM(contents.nth(1), fact.to);
    await row.scrollIntoViewIfNeeded();
    const screenshot = info.outputPath(`${session.platform.toLowerCase()}-${session.actor.id}-${fact.from}-${fact.to}-activity.png`);
    if (session.native) await nativeScreenshot(session.native.app, session.page, screenshot);
    else await session.page.screenshot({ path: screenshot, animations: "disabled" });
    evidence.checkpoint("actual_Activity_historical_document_reopened", { actor_id: session.actor.id, event_id: entry.id, event_seq: entry.event_seq,
      event_actor: entry.actor, created_at: entry.created_at, before, after, screenshot,
      identity_binding: "ordered_actual_Activity_response_and_DOM_actor_time_complete_text" });
  }
  await drawer.getByRole("button", { name: "Close resources", exact: true }).click(); await expect(drawer).toBeHidden();
}

export async function exerciseRequirementsHistory(browser: Browser, platform: RecoveryPlatform, info: TestInfo, evidence: RecoveryEvidence) {
  const fixture = await seed(platform, evidence), baseline = await audit(fixture, evidence, "setup_baseline");
  expect(baseline.events.map((event) => event.kind)).toEqual(["created", "participant_added", "participant_added"]);
  expect(baseline.operations.map((operation) => operation.request_id).sort()).toEqual([...fixture.inviteKeys].sort());
  for (const name of ["commands", "runs", "dispatches", "comments"]) expect(baseline.passive[name]).toEqual([]);
  await withSessions(browser, platform, fixture.surface, info, evidence, async (open) => {
    const sessions: RecoverySession[] = [], traffic: Traffic[] = [];
    try {
      for (const role of roles) { const session = await open(fixture.actors[role]); sessions.push(session); traffic.push(new Traffic(session)); }
      expect(new Set(sessions.map((session) => session.context)).size).toBe(3);
      if (platform === "Desktop") expect(new Set(sessions.map((session) => session.lifecycle.native_pid)).size).toBe(3);
      const initial = CollaborationIssueSchema.parse(await fixture.actors.product.client.requestJSON(rootPath(fixture.surface.issueId)));
      for (const [index, session] of sessions.entries()) {
        await session.page.reload();
        await observeVersion(session, traffic[index]!, "V1", initial.collaboration_revision, evidence);
      }
      const edits: EditFact[] = [];
      for (const [from, to] of [["V1", "V2"], ["V2", "V3"]] as const) {
        evidence.set({ active_phase: `${from}_to_${to}_actual_Edit_issue` });
        const fact = await edit(sessions[0]!, from, to, evidence); edits.push(fact);
        for (const [index, session] of sessions.entries()) await observeVersion(session, traffic[index]!, to, fact.receipt.collaboration_revision, evidence);
      }
      expect(new Set(edits.map((fact) => fact.request_id)).size).toBe(2);
      const final = await audit(fixture, evidence, "after_two_explicit_edits");
      let canonical: CollaborationActivityEntry[] | undefined;
      for (const [index, session] of sessions.entries()) {
        const entries = await completeHistory(session, evidence);
        const times = verifyPersistedHistory(fixture, baseline, final, edits, entries);
        evidence.checkpoint("original_event_times_equal_at_UTC_nanosecond_precision", { actor_id: session.actor.id, times });
        if (canonical) expect(entries).toEqual(canonical); else canonical = entries;
        await activityDOM(session, traffic[index]!, entries, edits, info, evidence);
      }
      for (const fact of edits) expect(CollaborationOperationSchema.parse(await sessionGet(sessions[0]!,
        `${rootPath(fixture.surface.issueId)}/operations/${fact.request_id}`))).toEqual(fact.receipt);
      const afterViews = await audit(fixture, evidence, "after_all_history_views_and_original_receipt_reads");
      expect(afterViews).toEqual(final);
      for (const [index, observed] of traffic.entries()) {
        await observed.settle();
        evidence.checkpoint("complete_session_business_request_inventory", { role: roles[index], actor_id: observed.session.actor.id,
          writes: observed.writes, detail_responses: observed.details, activity_responses: observed.activities });
        if (index > 0) { expect(observed.writes).toEqual([]); expect(observed.session.writes).toEqual([]); }
        else {
          expect(observed.writes).toHaveLength(2);
          expect(observed.writes.map((write) => ({ path: write.path, method: write.method, request_id: write.request_id })))
            .toEqual(edits.map((fact) => ({ path: rootPath(fixture.surface.issueId), method: "PATCH", request_id: fact.request_id })));
        }
      }
      evidence.set({ completed_observations: 9, completed_Activity_sessions: 3, complete_history_pages_per_session: 5,
        explicit_UI_edits: edits, observer_business_writes: 0, new_commands: 0, new_runs: 0, new_dispatches: 0,
        V1_source: "public_creation_setup_and_first_edit_before", version_table_added: false });
      await info.attach(`${platform.toLowerCase()}-requirements-history-summary`, { contentType: "text/markdown", body:
        `${platform}: product, developer and QA independently observed exact V1, V2 and V3 descriptions with their UI response revisions (9 observations).\n\n` +
        "Two real Edit issue actions produced exactly two distinct persisted events and original operation receipts. All three actual Activity drawers showed complete V1→V2 and V2→V3 text, actors and UTC timestamps; each session traversed all five events with limit=1.\n\n" +
        "Observer business writes and new commands/runs/dispatches: zero. Creation and invitation are explicit API setup. This is synthetic account collaboration acceptance, not payment execution, real model output or installed Desktop Main acceptance.\n" });
    } finally {
      for (const observed of traffic) {
        await Promise.allSettled([...observed.session.pendingReads]);
        evidence.checkpoint("retained_session_traffic_before_cleanup", { actor_id: observed.session.actor.id,
          writes: observed.writes, detail_responses: observed.details, activity_responses: observed.activities, observation_errors: observed.errors });
        observed.close();
      }
    }
  });
}
