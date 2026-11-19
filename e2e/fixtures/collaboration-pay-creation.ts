import { createHash, randomUUID } from "node:crypto";
import { expect, type Browser, type Locator, type Page, type Request, type Response, type TestInfo } from "@playwright/test";
import pg from "pg";
import { z } from "zod";
import { CollaborationActivitySchema, CollaborationCreationInputSchema, CollaborationCreationOptionsSchema,
  CollaborationCreationResultSchema, CollaborationIssueSchema, CollaborationOperationSchema,
  type CollaborationIssue } from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";
import { selectCollaborationRecipient } from "./collaboration-offline-handoff";
import { revealCollaborationDisclosure } from "./collaboration-disclosures";
import { nativeScreenshot } from "./collaboration-native";
import { RecoveryEvidence, withSessions, type RecoveryActor, type RecoveryPlatform, type RecoverySession, type RecoverySurface } from "./collaboration-recovery";

const now = () => new Date().toISOString();
const sha = (value: string | Buffer) => createHash("sha256").update(value).digest("hex");
const object = z.record(z.string(), z.unknown());
const objects = z.array(object);
const issuePath = (id: string) => `/api/collaboration/issues/${id}`;
const timestamp = (value: string) => BigInt(Date.parse(value.replace(/\.\d+/, ""))) * 1_000_000n +
  BigInt((value.match(/\.(\d+)/)?.[1] ?? "").padEnd(9, "0"));
const roles = ["product", "developer", "qa"] as const;
type Role = typeof roles[number];
interface Fixture {
  surface: RecoverySurface;
  workspace: { id: string; slug: string; name: string };
  project: { id: string; title: string };
  actors: Record<Role, RecoveryActor>;
  title: string; paragraphs: string[]; description: string; start: string; due: string;
}

export const payCreationJourneys = (["Web", "Desktop"] as const).map((platform) =>
  `${platform} creates one complete PAY issue through real controls and preserves its original creation identity`);

export function payCreationEnvironmentReady() {
  return process.env.PAY241_TC01_CREATION_ACCEPTANCE === "1" && process.env.E2E_PASSWORD_AUTH === "1" &&
    [process.env.COLLABORATION_E2E_API_URL, process.env.NEXT_PUBLIC_API_URL, process.env.DATABASE_URL,
      process.env.TASK_DATABASE_NAME, process.env.COLLABORATION_ELECTRON_RENDERER_URL,
      process.env.COLLABORATION_ELECTRON_SOURCE_ROOT].every(Boolean);
}

// Password credentials remain in closures, never in retained request/evidence records.
async function passwordActor(apiUrl: string, role: Role, suffix: string): Promise<RecoveryActor> {
  const client = new TestApiClient(), username = `tc01${role}${suffix}`, password = `TC01-${randomUUID()}!`;
  const name = `TC01 ${role} ${suffix}`, user = await client.registerPassword(username, password, name);
  await client.requestJSON("/api/me/onboarding", { method: "PATCH", body: { questionnaire: { source: ["friends_colleagues"] } } });
  const completed = z.object({ id: z.uuid(), onboarded_at: z.string().nullable() }).parse(
    await client.requestJSON("/api/me/onboarding/complete", { method: "POST", body: {} }));
  expect(completed.id).toBe(user.id); expect(completed.onboarded_at).not.toBeNull();
  return { client, id: user.id, name,
    async login() { expect((await client.loginPassword(username, password)).id).toBe(user.id); return client.getToken()!; },
    async authenticateWeb(page: Page) {
      const response = await page.request.post(`${apiUrl}/auth/login`, { data: { username, password } });
      expect(response.status()).toBe(200);
      expect(z.object({ user: z.object({ id: z.uuid() }) }).parse(await response.json()).user.id).toBe(user.id);
      await page.addInitScript(() => { localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false"); });
    } };
}

async function seed(platform: RecoveryPlatform, evidence: RecoveryEvidence): Promise<Fixture> {
  if (!payCreationEnvironmentReady()) throw new Error("TC-01 requires its explicit owned password allocation");
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
      name: `TC01 payment ${platform} ${suffix}`, slug: `tc01-${platform.toLowerCase()}-${suffix}`, issue_prefix: "PAY",
    } }));
  actors.product.client.setWorkspaceId(workspace.id); actors.product.client.setWorkspaceSlug(workspace.slug);
  const project = z.object({ id: z.uuid(), title: z.string() }).parse(await actors.product.client.requestJSON("/api/projects", {
    method: "POST", body: { title: `Payment callback recovery ${suffix}` },
  }));
  expect(await actors.developer.client.getWorkspaces()).toEqual([]); expect(await actors.qa.client.getWorkspaces()).toEqual([]);
  const start = new Date().toISOString().slice(0, 10), due = new Date(Date.now() + 7 * 86400_000).toISOString().slice(0, 10);
  const paragraphs = [
    "Impact: A successful customer payment leaves the original order awaiting payment. Duplicate charging or fulfillment must not occur.",
    "Reproduction: Create a sandbox order, deliver its successful payment callback, and compare the payment, order and fulfillment identities.",
    "Business priority: P1 is the supplied scenario label. The selected product priority is High; their equivalence is not established.",
    `Responsibilities: Product ${actors.product.name} records acceptance. Developer ${actors.developer.name} is the sole current handler. QA ${actors.qa.name} verifies results; these names do not grant stage roles.`,
    `Schedule: Start ${start}; due ${due}.`,
    "Acceptance criteria: The original order becomes paid after the successful callback. Three duplicate callbacks cause no duplicate charge or fulfillment. A callback delayed by 45 seconds and a recovered consumer failure retain traceable recovery evidence.",
  ];
  evidence.set({ api_url: apiUrl, database, source_workspace_id: workspace.id, project, start_date: start, due_date: due,
    actors: roles.map((role) => ({ role, id: actors[role].id, name: actors[role].name })),
    real_payment_execution: false, real_model_execution: false, installed_main_execution: false, root_seeded_by_API: false,
    source_membership: { product: true, developer: false, qa: false }, selected_priority: "high", P1_enum_mapping: null,
    occupational_roles_configured: false, description: paragraphs.join("\n\n") });
  return { actors, workspace, project, title: `PAY-241 scenario: payment succeeds but order awaits payment ${suffix}`,
    paragraphs, description: paragraphs.join("\n\n"), start, due,
    surface: { apiUrl, issueId: "", title: "Collaboration", accountPath: "/account?view=participating" } };
}

interface WriteFact { at: string; path: string; method: string; request_id: string | null; body: unknown }
class Traffic {
  readonly details: { at: string; body_sha256: string; issue: CollaborationIssue }[] = [];
  readonly options: z.infer<typeof CollaborationCreationOptionsSchema>[] = [];
  readonly writes: WriteFact[] = [];
  readonly errors: string[] = [];
  constructor(readonly session: RecoverySession) { session.page.on("request", this.request); session.page.on("response", this.response); }
  private track(promise: Promise<void>) {
    const pending = promise.catch((error) => { this.errors.push(error instanceof Error ? error.name : "traffic_error"); });
    this.session.pendingReads.add(pending); void pending.finally(() => this.session.pendingReads.delete(pending));
  }
  private request = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (!["POST", "PATCH", "PUT", "DELETE"].includes(request.method()) || !path.startsWith("/api/collaboration/")) return;
    this.track((async () => {
      const headers = await request.allHeaders();
      this.writes.push({ at: now(), path, method: request.method(), request_id: headers["idempotency-key"] ?? null,
        body: request.postData() === null ? null : request.postDataJSON() });
    })());
  };
  private response = (response: Response) => {
    const path = new URL(response.url()).pathname;
    if (response.request().method() !== "GET" || response.status() !== 200) return;
    const detail = this.session.fixture.issueId !== "" && path === issuePath(this.session.fixture.issueId);
    if (!detail && path !== "/api/collaboration/creation-options") return;
    this.track((async () => {
      const bytes = await response.body();
      if (detail) this.details.push({ at: now(), body_sha256: sha(bytes), issue: CollaborationIssueSchema.parse(JSON.parse(bytes.toString())) });
      else this.options.push(CollaborationCreationOptionsSchema.parse(JSON.parse(bytes.toString())));
    })());
  };
  async settle() { await Promise.allSettled([...this.session.pendingReads]); expect(this.errors).toEqual([]); }
  close() { this.session.page.off("request", this.request); this.session.page.off("response", this.response); }
}

const sql: Record<string, string> = {
  workspace: "SELECT id,issue_prefix,issue_counter FROM workspace WHERE id=$1",
  issues: "SELECT id,workspace_id,number,title,description,status,priority,start_date::text,due_date::text,creator_type,creator_id,assignee_type,assignee_id,project_id,parent_issue_id,created_at::text,updated_at::text,revision::text FROM issue WHERE workspace_id=$1 ORDER BY id",
  collaboration: "SELECT to_jsonb(c) AS row FROM issue_collaboration c JOIN issue i ON i.id=c.issue_id WHERE i.workspace_id=$1 ORDER BY c.issue_id",
  sessions: "SELECT to_jsonb(s) AS row FROM collaboration_session s JOIN issue i ON i.id=s.issue_id WHERE i.workspace_id=$1 ORDER BY s.id",
  participants: "SELECT to_jsonb(p) AS row FROM issue_participant p JOIN issue i ON i.id=p.issue_id WHERE i.workspace_id=$1 ORDER BY p.issue_id,p.user_id",
  events: "SELECT e.id,e.issue_id,e.event_seq::text,e.actor_user_id,e.kind,e.created_at::text,e.payload FROM collaboration_event e JOIN issue i ON i.id=e.issue_id WHERE i.workspace_id=$1 ORDER BY e.event_seq,e.id",
  operations: "SELECT o.id,o.scope_kind,o.scope_id,o.actor_user_id,o.request_id,o.kind,o.payload_hash,o.result_code,o.result,o.resolved_issue_id,o.created_at::text,o.completed_at::text FROM collaboration_operation o WHERE o.scope_kind='create' AND o.scope_id=$1 OR o.scope_kind='issue' AND o.scope_id IN(SELECT id FROM issue WHERE workspace_id=$1) ORDER BY o.id",
  commands: "SELECT c.id FROM collaboration_command c JOIN collaboration_session s ON s.id=c.session_id JOIN issue i ON i.id=s.issue_id WHERE i.workspace_id=$1 ORDER BY c.id",
  runs: "SELECT r.id FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id JOIN issue i ON i.id=s.issue_id WHERE i.workspace_id=$1 ORDER BY r.id",
  dispatches: "SELECT q.id FROM agent_task_queue q JOIN issue i ON i.id=q.issue_id WHERE i.workspace_id=$1 ORDER BY q.id",
  comments: "SELECT c.id FROM comment c JOIN issue i ON i.id=c.issue_id WHERE i.workspace_id=$1 ORDER BY c.id",
  notifications: "SELECT n.id,n.issue_id,n.recipient_user_id,n.event_seq::text,n.kind,n.read_at::text,n.archived_at::text FROM collaboration_notification n JOIN issue i ON i.id=n.issue_id WHERE i.workspace_id=$1 ORDER BY n.id",
};
interface Audit { server_clock: string; rows: Record<string, Record<string, unknown>[]> }
async function audit(fixture: Fixture, evidence: RecoveryEvidence, stage: string): Promise<Audit> {
  const client = new pg.Client({ connectionString: process.env.DATABASE_URL!, connectionTimeoutMillis: 5_000 });
  try {
    await client.connect(); await client.query("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY");
    await client.query("SET LOCAL statement_timeout='5s'"); await client.query("SET LOCAL idle_in_transaction_session_timeout='10s'");
    await client.query("SET LOCAL TIME ZONE 'UTC'");
    const identity = z.object({ database: z.string(), read_only: z.string(), server_clock: z.string() }).parse((await client.query(
      "SELECT current_database() AS database,current_setting('transaction_read_only') AS read_only,clock_timestamp()::text AS server_clock")).rows[0]);
    expect(identity.database).toBe(process.env.TASK_DATABASE_NAME); expect(identity.read_only).toBe("on");
    const rows: Audit["rows"] = {};
    for (const [name, query] of Object.entries(sql)) rows[name] = objects.parse((await client.query(query, [fixture.workspace.id])).rows);
    await client.query("COMMIT");
    evidence.checkpoint(stage, { ...identity, rows, queries: sql });
    return { server_clock: identity.server_clock, rows };
  } finally { await client.end(); }
}

async function sessionRequest(session: RecoverySession, path: string, method = "GET", body?: unknown, key?: string) {
  const result = await session.page.evaluate(async ({ apiUrl, platform, path, method, body, key }) => {
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (platform === "Desktop") {
      const token = localStorage.getItem("multica_token"); if (!token) throw new Error("Missing native password session");
      headers.Authorization = `Bearer ${token}`;
    } else if (method !== "GET") {
      const csrf = document.cookie.split(";").map((value) => value.trim()).find((value) => value.startsWith("multica_csrf="));
      if (!csrf) throw new Error("Missing password CSRF cookie"); headers["X-CSRF-Token"] = decodeURIComponent(csrf.slice("multica_csrf=".length));
    }
    if (key) headers["Idempotency-Key"] = key;
    const response = await fetch(`${apiUrl}${path}`, { method, headers, credentials: "include",
      ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
    return { status: response.status, body: await response.json() as unknown };
  }, { apiUrl: session.fixture.apiUrl, platform: session.platform, path, method, body, key });
  return { at: now(), path, method, request_id: key ?? null, ...result };
}

function assertIssue(issue: CollaborationIssue, fixture: Fixture) {
  expect(issue).toMatchObject({ title: fixture.title, description: fixture.description, ai_policy: "manual", priority: "high",
    status: "todo", status_category: "todo", start_date: fixture.start, due_date: fixture.due,
    source_workspace: { id: fixture.workspace.id, name: fixture.workspace.name },
    source_structure: { project: { id: fixture.project.id, name: fixture.project.title } },
    creator: { user_id: fixture.actors.product.id, name: fixture.actors.product.name },
    handler: { user_id: fixture.actors.developer.id, name: fixture.actors.developer.name }, session: { queue_state: "paused" } });
  expect(issue.execution?.commands ?? []).toEqual([]); expect(issue.execution?.runs ?? []).toEqual([]);
  expect(issue.execution?.targets ?? []).toEqual([]);
}

async function field(container: Locator, label: string, value: string) {
  const row = container.locator("dl > div").filter({ has: container.page().getByText(label, { exact: true }) });
  await expect(row).toHaveCount(1); await expect(row.locator("dd")).toHaveText(value);
}

async function observe(session: RecoverySession, traffic: Traffic, fixture: Fixture, info: TestInfo, evidence: RecoveryEvidence) {
  await expect.poll(() => traffic.details.some((read) => read.issue.id === fixture.surface.issueId && read.issue.description === fixture.description)).toBe(true);
  const read = traffic.details.findLast((item) => item.issue.id === fixture.surface.issueId)!; assertIssue(read.issue, fixture);
  const root = session.page.getByRole("heading", { name: fixture.title, exact: true }).locator("../../..");
  await expect(root.locator("#description [data-rich-content] > p")).toHaveText(fixture.paragraphs);
  const header = root.locator(":scope > header");
  await field(header, "Status", "Todo");
  const handler = header.locator("dl > div").filter({ has: session.page.getByText("Handler", { exact: true }) }).locator("dd");
  await expect(handler).toContainText(fixture.actors.developer.name);
  await expect(handler).not.toContainText(fixture.actors.product.name); await expect(handler).not.toContainText(fixture.actors.qa.name);
  const handlerName = await handler.innerText();
  await field(header, "Due date", fixture.due);
  await session.page.getByRole("group", { name: "Issue resources", exact: true }).getByRole("button", { name: "Issue details", exact: true }).click();
  const drawer = session.page.getByRole("dialog", { name: "Issue details", exact: true }); await expect(drawer).toBeVisible();
  await field(drawer, "Creator", fixture.actors.product.name);
  const fields = drawer.getByRole("region", { name: "Issue fields", exact: true });
  await field(fields, "Priority", "High"); await field(fields, "Start date", fixture.start);
  await field(fields, "Due date", fixture.due); await field(fields, "Project", fixture.project.title);
  const screenshot = info.outputPath(`${session.platform.toLowerCase()}-${session.actor.id}-creation-fields.png`);
  if (session.native) await nativeScreenshot(session.native.app, session.page, screenshot);
  else await session.page.screenshot({ path: screenshot, animations: "disabled" });
  evidence.checkpoint("actual_session_created_issue_and_fields", { actor_id: session.actor.id, UI_response: read,
    description_paragraphs: fixture.paragraphs, status_label: "Todo", handler_name: handlerName,
    detail_fields: await fields.innerText(), screenshot });
  await drawer.getByRole("button", { name: "Close resources", exact: true }).click(); await expect(drawer).toBeHidden();
}

export async function exercisePayCreation(browser: Browser, platform: RecoveryPlatform, info: TestInfo, evidence: RecoveryEvidence) {
  const fixture = await seed(platform, evidence), initial = await audit(fixture, evidence, "before_any_root_creation");
  for (const name of Object.keys(sql).filter((name) => name !== "workspace")) expect(initial.rows[name]).toEqual([]);
  await withSessions(browser, platform, fixture.surface, info, evidence, async (open) => {
    const sessions: RecoverySession[] = [], traffic: Traffic[] = [];
    try {
      const product = await open(fixture.actors.product); sessions.push(product); const productTraffic = new Traffic(product); traffic.push(productTraffic);
      await product.page.getByRole("button", { name: "New collaboration issue", exact: true }).click();
      const dialog = product.page.getByRole("dialog", { name: "New collaboration issue", exact: true });
      await expect(dialog.getByLabel("Source workspace", { exact: true })).toHaveValue(fixture.workspace.id);
      const title = dialog.getByLabel("Title", { exact: true }), submit = dialog.getByRole("button", { name: "Create collaboration issue", exact: true });
      await expect(title).toHaveAttribute("required", ""); await title.fill("   "); await expect(submit).toBeDisabled();
      await expect(dialog.getByText("Add a title to create the issue.", { exact: true })).toBeVisible(); await title.press("Enter");
      await productTraffic.settle(); expect(productTraffic.writes).toEqual([]);
      const prevented = await audit(fixture, evidence, "blank_title_did_not_create_or_reserve"); expect(prevented.rows).toEqual(initial.rows);
      evidence.checkpoint("actual_blank_title_prevention", { title: "   ", disabled: true, UI_creation_POSTs: 0, phantom_issues: 0 });
      await title.fill(fixture.title); await dialog.getByLabel("Description", { exact: true }).fill(fixture.description);
      await selectCollaborationRecipient(product.page, dialog, fixture.actors.developer);
      await expect(dialog.getByRole("status").filter({ hasText: `Assigned to ${fixture.actors.developer.name}` })).toBeVisible();
      await dialog.getByLabel("AI participation", { exact: true }).selectOption("manual");
      await dialog.getByRole("button", { name: "Initial fields", exact: true }).click();
      await dialog.getByLabel("Status", { exact: true }).selectOption("todo");
      await dialog.getByLabel("Priority", { exact: true }).selectOption("high");
      await dialog.getByLabel("Start date", { exact: true }).fill(fixture.start); await dialog.getByLabel("Due date", { exact: true }).fill(fixture.due);
      await dialog.getByLabel("Project", { exact: true }).selectOption(fixture.project.id); await expect(submit).toBeEnabled();
      await productTraffic.settle();
      const catalog = productTraffic.options.findLast((options) => options.source?.id === fixture.workspace.id &&
        options.stage_policy_observation?.state === "known" && options.stage_policy_observation.project_id === fixture.project.id);
      expect(catalog?.stage_policy_observation).toMatchObject({ state: "known", managed: false });
      expect(catalog?.fields?.statuses.find((status) => status.key === "todo")).toMatchObject({ key: "todo", category: "todo" });
      evidence.checkpoint("actual_creation_catalog_and_complete_form", { catalog, status_value: await dialog.getByLabel("Status", { exact: true }).inputValue(),
        title: await title.inputValue(), description: await dialog.getByLabel("Description", { exact: true }).inputValue() });
      const [response] = await Promise.all([
        product.page.waitForResponse((item) => new URL(item.url()).pathname === "/api/collaboration/issues" && item.request().method() === "POST"), submit.click(),
      ]);
      const headers = await response.request().allHeaders(), key = z.string().min(1).parse(headers["idempotency-key"]);
      const body = CollaborationCreationInputSchema.parse(response.request().postDataJSON()), raw: unknown = await response.json();
      evidence.checkpoint("actual_UI_creation_response", { request_id: key, body, status: response.status(), response: raw,
        authentication: { cookie: Boolean(headers.cookie), csrf: Boolean(headers["x-csrf-token"]), bearer: Boolean(headers.authorization) } });
      expect(response.status()).toBe(201); const issue = CollaborationIssueSchema.parse(raw); assertIssue(issue, fixture);
      expect(body).toMatchObject({ title: fixture.title, description: fixture.description, source_workspace_id: fixture.workspace.id,
        handler_user_id: fixture.actors.developer.id, status: "todo", priority: "high", project_id: fixture.project.id,
        start_date: fixture.start, due_date: fixture.due, ai_policy: "manual" });
      expect(issue.collaboration_revision).toBe("1"); expect(issue.realtime?.event_seq).toBe("1");
      fixture.surface.issueId = issue.id; fixture.surface.title = issue.title; fixture.surface.accountPath = `/account?view=participating&issue=${issue.id}`;
      await expect(dialog).toBeHidden(); await expect(product.page.getByRole("heading", { name: fixture.title, exact: true })).toBeVisible();
      await productTraffic.settle(); expect(productTraffic.writes.filter((write) => write.path === "/api/collaboration/issues")).toHaveLength(1);
      const created = await audit(fixture, evidence, "after_exactly_one_actual_UI_creation");
      expect(created.rows.issues).toHaveLength(1); expect(created.rows.collaboration).toHaveLength(1);
      expect(created.rows.sessions).toHaveLength(1); expect(created.rows.events).toHaveLength(1); expect(created.rows.operations).toHaveLength(1);
      const stored = object.parse(created.rows.issues![0]), event = object.parse(created.rows.events![0]), operation = object.parse(created.rows.operations![0]);
      expect(stored).toMatchObject({ id: issue.id, creator_type: "member", creator_id: fixture.actors.product.id, assignee_type: "member",
        assignee_id: fixture.actors.developer.id, title: body.title, description: body.description, project_id: fixture.project.id,
        status: "todo", priority: "high", start_date: fixture.start, due_date: fixture.due });
      const identifier = `PAY-${z.number().int().positive().parse(stored.number)}`;
      expect(issue.source_workspace.identifier).toBe(identifier); expect(identifier).not.toBe("PAY-241");
      expect(event).toMatchObject({ issue_id: issue.id, actor_user_id: fixture.actors.product.id, event_seq: "1", kind: "created",
        payload: { handler_user_id: fixture.actors.developer.id } });
      expect(operation).toMatchObject({ scope_kind: "create", scope_id: fixture.workspace.id, actor_user_id: fixture.actors.product.id,
        request_id: key, kind: "create", result_code: "created", result: null, resolved_issue_id: issue.id });
      expect(z.string().parse(operation.payload_hash)).toMatch(/^[a-f0-9]{64}$/);
      const issueTime = z.string().parse(stored.created_at), eventTime = z.string().parse(event.created_at);
      for (const value of [issueTime, eventTime, z.string().parse(operation.created_at), z.string().parse(operation.completed_at)]) {
        expect(timestamp(value)).toBeGreaterThanOrEqual(timestamp(prevented.server_clock));
        expect(timestamp(value)).toBeLessThanOrEqual(timestamp(created.server_clock));
      }
      expect(timestamp(issueTime)).toBeLessThanOrEqual(timestamp(eventTime));
      expect(timestamp(eventTime)).toBeLessThanOrEqual(timestamp(z.string().parse(operation.completed_at)));
      evidence.set({ issue_id: issue.id, generated_identifier: identifier, original_request_id: key, original_creation_operation: operation,
        created_event: event, issue_created_at_UTC: issueTime, created_event_at_UTC: eventTime, issue_DTO_has_created_at: false });
      const query = `source_workspace_id=${fixture.workspace.id}`;
      // Creation POST uses the server's default execution projection. Recover
      // that same protocol to compare full DTOs, not unlike v5/v6 envelopes.
      const recovery = await sessionRequest(product, `/api/collaboration/creations/${key}?${query}`);
      expect(recovery.status).toBe(200); expect(CollaborationIssueSchema.parse(recovery.body)).toEqual(issue);
      const outcome = await sessionRequest(product, `/api/collaboration/creations/${key}/result?${query}`);
      expect(outcome.status).toBe(200); expect(CollaborationCreationResultSchema.parse(outcome.body)).toEqual({ request_id: key,
        source_workspace_id: fixture.workspace.id, actor_user_id: fixture.actors.product.id, outcome: "created", code: "created" });
      const replay = await sessionRequest(product, "/api/collaboration/issues", "POST", body, key);
      expect(replay.status).toBe(201); expect(CollaborationIssueSchema.parse(replay.body)).toEqual(issue);
      const afterRecovery = await audit(fixture, evidence, "after_original_key_GET_result_and_API_replay"); expect(afterRecovery.rows).toEqual(created.rows);
      evidence.checkpoint("original_identity_recovery_and_replay_API_controls", { recovery, outcome, replay, issue_id: issue.id,
        original_operation_unchanged: true, duplicate_effects: 0, UI_ACK_loss_recovery_claimed: false });
      const current = CollaborationIssueSchema.parse(await fixture.actors.product.client.requestJSON(issuePath(issue.id)));
      const context = current.write_context; if (!context) throw new Error("The product creator lacks participant authority");
      const inviteKey = randomUUID();
      const invitation = CollaborationOperationSchema.parse(await fixture.actors.product.client.requestJSON(`${issuePath(issue.id)}/participants`, {
        method: "POST", headers: { "Idempotency-Key": inviteKey }, body: { expected_collaboration_revision: current.collaboration_revision,
          expected_lifecycle_epoch: current.lifecycle_epoch, expected_participant_epoch: context.participant_epoch,
          expected_account_epoch: context.account_epoch, target_user_id: fixture.actors.qa.id },
      }));
      expect(invitation.code).toBe("participant_added"); evidence.checkpoint("explicit_QA_participant_API_setup_after_creation_baseline", { request_id: inviteKey, invitation });
      const viewing = await audit(fixture, evidence, "before_three_authorized_session_views");
      expect(viewing.rows.events).toHaveLength(2); expect(viewing.rows.operations).toHaveLength(2);
      expect(viewing.rows.operations!.find((item) => item.id === operation.id)).toEqual(operation);
      expect(viewing.rows.events!.find((item) => item.id === event.id)).toEqual(event);
      for (const role of ["developer", "qa"] as const) {
        const session = await open(fixture.actors[role]); sessions.push(session); traffic.push(new Traffic(session));
      }
      expect(new Set(sessions.map((session) => session.context)).size).toBe(3);
      if (platform === "Desktop") expect(new Set(sessions.map((session) => session.lifecycle.native_pid)).size).toBe(3);
      for (const [index, session] of sessions.entries()) {
        await session.page.reload(); await observe(session, traffic[index]!, fixture, info, evidence);
      }
      const drawer = await revealCollaborationDisclosure(product.page, "Activity");
      const history = CollaborationActivitySchema.parse((await sessionRequest(product, `${issuePath(issue.id)}/activity`)).body);
      expect(history.next_cursor).toBeNull(); expect(history.items).toHaveLength(2);
      const original = history.items.find((entry) => entry.kind === "created")!;
      expect(original).toMatchObject({ id: event.id, issue_id: issue.id, event_seq: "1", actor: { user_id: fixture.actors.product.id, name: fixture.actors.product.name } });
      expect(timestamp(original.created_at)).toBe(timestamp(eventTime));
      const row = drawer.getByRole("region", { name: "Activity", exact: true }).locator(":scope > ol > li").filter({ hasText: "Issue created" });
      await expect(row).toHaveCount(1); await expect(row.getByText(fixture.actors.product.name, { exact: true })).toBeVisible();
      await expect(row.locator("time")).toHaveAttribute("datetime", original.created_at);
      evidence.checkpoint("actual_Activity_creation_actor_and_time", { original_event: original, SQL_issue_created_at: issueTime,
        SQL_event_created_at: eventTime, UTC_event_nanoseconds: timestamp(original.created_at).toString(), actor_and_datetime_visible: true });
      await drawer.getByRole("button", { name: "Close resources", exact: true }).click(); await expect(drawer).toBeHidden();
      const final = await audit(fixture, evidence, "after_all_observer_reads"); expect(final.rows).toEqual(viewing.rows);
      for (const name of ["commands", "runs", "dispatches", "comments"]) expect(final.rows[name]).toEqual([]);
      for (const [index, observed] of traffic.entries()) {
        await observed.settle();
        if (index > 0) { expect(observed.writes).toEqual([]); expect(observed.session.writes).toEqual([]); }
        else {
          expect(observed.writes.filter((write) => write.path === "/api/collaboration/issues").map((write) => write.request_id)).toEqual([key, key]);
          expect(observed.writes.filter((write) => write.path !== "/api/collaboration/issues").every((write) =>
            ["/api/collaboration/recipients/search", "/api/collaboration/recipients/resolve"].includes(write.path))).toBe(true);
        }
      }
      evidence.set({ completed_sessions: 3, actual_UI_creations: 1, empty_title_preventions: 1, original_key_GETs: 1, result_GETs: 1,
        same_key_API_replays: 1, observer_business_writes: 0, new_commands: 0, new_runs: 0, new_dispatches: 0,
        current_requirement_disposition: "positive_creation_subscope_only", global_mandatory_fields_proven: false, P1_mapping_proven: false });
      await info.attach(`${platform.toLowerCase()}-pay-creation-summary`, { contentType: "text/markdown", body:
        `${platform}: one actual UI-created issue ${identifier}; three independent password sessions render its complete requirements, handler, status, project and dates.\n\n` +
        "Empty title prevents creation; original-key GET/result GET/same-body API replay preserve the original issue, operation and effects. No observer business writes or Agent dispatch. P1 mapping, mandatory business fields and occupational authorization remain outside this positive example.\n" });
    } finally {
      for (const observed of traffic) {
        await Promise.allSettled([...observed.session.pendingReads]);
        evidence.checkpoint("retained_session_traffic_before_cleanup", { actor_id: observed.session.actor.id, writes: observed.writes,
          detail_responses: observed.details, creation_options: observed.options, errors: observed.errors });
        observed.close();
      }
    }
  });
}
