import { createHash, randomUUID } from "node:crypto";
import { expect, type Browser, type Page, type Request, type Response, type TestInfo } from "@playwright/test";
import pg from "pg";
import { z } from "zod";
import { CommentSchema, CreateIssueResponseSchema, InboxItemListSchema, SubscribersListSchema } from "@multica/core/api/schemas";
import { CollaborationIssueSchema } from "@multica/core/collaboration";
import { TestApiClient } from "../fixtures";
import { revealCollaborationDisclosure } from "./collaboration-disclosures";
import { selectDesktopWorkspace } from "./collaboration-native";
import {
  RecoveryEvidence, disconnectObserver, recoveryEnvironmentReady, withSessions,
  type RecoveryActor, type RecoveryPlatform, type RecoverySession, type RecoverySurface,
} from "./collaboration-recovery";

const sha = (value: string) => createHash("sha256").update(value).digest("hex");
const digest = (value: unknown) => sha(JSON.stringify(value));
const now = () => new Date().toISOString();
const object = z.record(z.string(), z.unknown());
const ordinaryIssueSchema = CreateIssueResponseSchema.extend({
  id: z.uuid(), workspace_id: z.uuid(), parent_issue_id: z.uuid(), collaboration: z.null().nullish(),
  revision: z.number().int().positive(),
});
const commentSchema = CommentSchema.extend({ id: z.uuid(), issue_id: z.uuid(), author_id: z.uuid(), revision: z.number().int().positive() });
const inboxSchema = InboxItemListSchema.element.extend({
  id: z.uuid(), workspace_id: z.uuid(), recipient_id: z.uuid(), issue_id: z.uuid().nullish(),
  actor_type: z.string().nullish(), actor_id: z.uuid().nullish(),
  details: z.object({ comment_id: z.uuid().optional() }).loose().nullish(),
});
const subscriberRowSchema = z.object({
  issue_id: z.uuid(), workspace_id: z.uuid(), user_type: z.string(), user_id: z.uuid(), reason: z.string(),
  created_at: z.string(), unsubscribed_at: z.string().nullable(), opt_out_scope: z.string().nullable(), member_id: z.uuid().nullable(),
});
const inboxRowSchema = z.object({
  id: z.uuid(), workspace_id: z.uuid(), recipient_type: z.string(), recipient_id: z.uuid(), type: z.string(),
  issue_id: z.uuid(), actor_type: z.string().nullable(), actor_id: z.uuid().nullable(), comment_id: z.uuid().nullable(),
  read: z.boolean(), archived: z.boolean(), title_sha256: z.string(), body_sha256: z.string().nullable(),
  details_sha256: z.string().nullable(), created_at: z.string(),
});
const commentRowSchema = z.object({
  id: z.uuid(), issue_id: z.uuid(), workspace_id: z.uuid(), author_type: z.string(), author_id: z.uuid(), type: z.string(),
  revision: z.string(), parent_id: z.uuid().nullable(), source_task_id: z.uuid().nullable(), quick_action_id: z.uuid().nullable(),
  content_sha256: z.string(), created_at: z.string(), updated_at: z.string(), deleted_at: z.string().nullable(),
});

export const subscriptionJourneyNames = ["Web", "Desktop"].map((platform) =>
  `${platform} ordinary subscription preserves exact comment delivery through reconnect, reload and unsubscribe`);

export function recoverySubscriptionsEnvironmentReady() {
  return recoveryEnvironmentReady() && process.env.PAY241_S3_SUBSCRIPTION_ACCEPTANCE === "1";
}

type OrdinaryIssue = z.infer<typeof ordinaryIssueSchema>;
type OrdinaryComment = z.infer<typeof commentSchema>;
type Inbox = z.infer<typeof inboxSchema>;
type SubscriberRow = z.infer<typeof subscriberRowSchema>;
type InboxRow = z.infer<typeof inboxRowSchema>;
type AuditRows = Record<string, Record<string, unknown>[]>;
interface Audit {
  rows: AuditRows;
  subscribers: SubscriberRow[];
  inbox: InboxRow[];
  comments: z.infer<typeof commentRowSchema>[];
}
interface SubscriptionFixture {
  surface: RecoverySurface;
  workspace: { id: string; slug: string; name: string };
  child: OrdinaryIssue;
  actors: { commenter: RecoveryActor; watcher: RecoveryActor; nonwatcher: RecoveryActor; optout: RecoveryActor };
}
interface WriteFact {
  at: string; actor_id: string; method: string; path: string; payload_sha256: string | null;
  request_id: string | null; workspace_id: string | null; workspace_slug: string | null;
  authentication: { cookie: boolean; csrf: boolean; bearer: boolean };
}
interface SocketBinding { id: string; at: string; workspace_slug: string | null; scope: string | null }

// Only synthetic account credentials live in this closure; evidence keeps IDs.
async function passwordActor(apiUrl: string, suffix: string, role: string, evidence: RecoveryEvidence): Promise<RecoveryActor> {
  const client = new TestApiClient();
  const username = `s3sub${role}${suffix}`, password = `S3-subscription-${randomUUID()}!`, name = `S3 subscription ${role} ${suffix}`;
  const registered = await client.registerPassword(username, password, name);
  evidence.checkpoint("actual_password_actor_registered", { role, actor_id: registered.id });
  await client.requestJSON("/api/me/onboarding", { method: "PATCH", body: { questionnaire: { source: ["friends_colleagues"] } } });
  const onboarded = z.object({ id: z.uuid(), onboarded_at: z.string().nullable() }).parse(
    await client.requestJSON("/api/me/onboarding/complete", { method: "POST", body: {} }));
  expect(onboarded.id).toBe(registered.id); expect(onboarded.onboarded_at).not.toBeNull();
  evidence.checkpoint("actual_password_actor_onboarded", { role, actor_id: onboarded.id });
  expect(await client.getWorkspaces()).toEqual([]);
  return { client, id: registered.id, name,
    async login() { expect((await client.loginPassword(username, password)).id).toBe(registered.id); return client.getToken()!; },
    async authenticateWeb(page: Page) {
      const response = await page.request.post(`${apiUrl}/auth/login`, { data: { username, password } });
      expect(response.status()).toBe(200);
      expect(z.object({ user: z.object({ id: z.uuid() }) }).parse(await response.json()).user.id).toBe(registered.id);
      const cookies = await page.context().cookies(apiUrl);
      expect(cookies.some((cookie) => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
      expect(cookies.some((cookie) => cookie.name === "multica_csrf" && !cookie.httpOnly)).toBe(true);
      await page.addInitScript(() => {
        localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false");
      });
    } };
}

async function seedSubscriptions(platform: RecoveryPlatform, evidence: RecoveryEvidence): Promise<SubscriptionFixture> {
  if (!recoverySubscriptionsEnvironmentReady()) throw new Error("Ordinary subscriptions require their explicit password allocation");
  evidence.set({ active_phase: "public_password_and_source_setup" });
  const apiUrl = process.env.COLLABORATION_E2E_API_URL!;
  for (const value of [apiUrl, process.env.NEXT_PUBLIC_API_URL!, process.env.DATABASE_URL!]) {
    expect(["localhost", "127.0.0.1"]).toContain(new URL(value).hostname);
  }
  expect(new URL(apiUrl).origin).toBe(new URL(process.env.NEXT_PUBLIC_API_URL!).origin);
  const databaseName = decodeURIComponent(new URL(process.env.DATABASE_URL!).pathname.slice(1));
  expect(databaseName).toBe(process.env.TASK_DATABASE_NAME);
  expect(["", "multica", "postgres", "template0", "template1"]).not.toContain(databaseName);
  const configuration = await fetch(`${apiUrl}/api/config`);
  expect(configuration.status).toBe(200); expect(await configuration.json()).toMatchObject({ auth_mode: "password" });
  const suffix = `${Date.now().toString(36)}${randomUUID().replaceAll("-", "").slice(0, 4)}`;
  const actors = { commenter: await passwordActor(apiUrl, suffix, "commenter", evidence), watcher: await passwordActor(apiUrl, suffix, "watcher", evidence),
    nonwatcher: await passwordActor(apiUrl, suffix, "nonwatcher", evidence), optout: await passwordActor(apiUrl, suffix, "optout", evidence) };
  expect(new Set(Object.values(actors).map((actor) => actor.id)).size).toBe(4);
  // Prefixes are globally unique on this deployment, independently of slugs.
  const issuePrefix = `S${Date.now().toString(36).slice(-7)}${randomUUID().slice(0, 2)}`.toUpperCase();
  const workspace = z.object({ id: z.uuid(), slug: z.string(), name: z.string() }).parse(
    await actors.commenter.client.requestJSON("/api/workspaces", { method: "POST", body: {
      name: `S3 ordinary subscriptions ${platform} ${suffix}`, slug: `s3-subscriptions-${platform.toLowerCase()}-${suffix}`, issue_prefix: issuePrefix,
    } }));
  evidence.set({ source_workspace_id: workspace.id });
  actors.commenter.client.setWorkspaceId(workspace.id); actors.commenter.client.setWorkspaceSlug(workspace.slug);
  const link = z.object({ id: z.uuid(), code: z.string().min(1), workspace_id: z.uuid(), role: z.literal("member") }).parse(
    await actors.commenter.client.requestJSON(`/api/workspaces/${workspace.id}/share-links`, {
      method: "POST", body: { role: "member", expires_in: 1, max_uses: 3 },
    }));
  const memberships: Record<string, string> = {};
  for (const actor of [actors.watcher, actors.nonwatcher, actors.optout]) {
    const joined = z.object({ workspace_id: z.uuid(), member: z.object({ id: z.uuid(), user_id: z.uuid(), role: z.literal("member") }) }).parse(
      await actor.client.requestJSON("/api/share-links/join", { method: "POST", body: { code: link.code } }));
    expect(joined.workspace_id).toBe(workspace.id); expect(joined.member.user_id).toBe(actor.id);
    memberships[actor.id] = joined.member.id;
    actor.client.setWorkspaceId(workspace.id); actor.client.setWorkspaceSlug(workspace.slug);
    expect((await actor.client.getWorkspaces()).map((item) => item.id)).toEqual([workspace.id]);
  }
  const creationKey = randomUUID();
  const parent = CollaborationIssueSchema.parse(await actors.commenter.client.requestJSON("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": creationKey }, body: {
      source_workspace_id: workspace.id, handler_user_id: actors.commenter.id,
      title: `S3 subscription parent ${suffix}`, description: "Synthetic parent for an ordinary subscription journey.", attachment_ids: [],
    },
  }));
  evidence.set({ parent_issue_id: parent.id });
  const child = ordinaryIssueSchema.parse(await actors.commenter.client.createIssue(`S3 ordinary subscription child ${suffix}`, {
    parent_issue_id: parent.id, status: "todo", assignee_type: null, assignee_id: null,
    description: "Synthetic ordinary child with no Agent or execution request.",
  }));
  evidence.set({ child_issue_id: child.id });
  expect(child.parent_issue_id).toBe(parent.id); expect(child.workspace_id).toBe(workspace.id); expect(child.collaboration == null).toBe(true);
  expect(child.creator_id).toBe(actors.commenter.id); expect(child.creator_id).not.toBe(actors.watcher.id);
  for (const [action, subscribed] of [["subscribe", true], ["unsubscribe", false]] as const) {
    expect(await actors.optout.client.requestJSON(`/api/issues/${child.id}/${action}`, { method: "POST", body: {} })).toEqual({ subscribed });
  }
  evidence.set({ platform, database: databaseName, api_url: apiUrl, source_workspace_id: workspace.id,
    parent_issue_id: parent.id, child_issue_id: child.id, child_collaboration: null,
    child_collaboration_wire_present: Object.hasOwn(child, "collaboration"), child_title_sha256: sha(child.title),
    actors: Object.fromEntries(Object.entries(actors).map(([role, actor]) => [role, actor.id])),
    planned_comments: 2, registration_accounts: 4, registration_limiter_preserved: true,
    literal_event_replay: "SEPARATE_GO_LISTENER_ACCEPTANCE", API_key_replay: "NOT_RUN", ordinary_composer_idempotency_key: "NOT_ADDED",
    provider_processes: 0, real_model_execution: false, installed_main_execution: false });
  evidence.checkpoint("public_source_membership_and_ordinary_child_setup", { parent_creation_request_id: creationKey,
    source_member_ids: memberships, share_link_id: link.id, share_code_sha256: sha(link.code), share_max_uses: 3,
    optout_control_API_actions: ["subscribe", "unsubscribe"], retained_fixture_ids: [parent.id, child.id] });
  return { surface: { apiUrl, issueId: parent.id, title: parent.title, accountPath: `/account?view=participating&issue=${parent.id}` },
    workspace, child, actors };
}

const auditQueries = {
  issues: `SELECT i.id,i.workspace_id,i.parent_issue_id,i.creator_type,i.creator_id,i.status,i.priority,i.revision::text,
    encode(sha256(convert_to(i.title,'UTF8')),'hex') AS title_sha256,
    encode(sha256(convert_to(i.description,'UTF8')),'hex') AS description_sha256,
    EXISTS(SELECT 1 FROM issue_collaboration c WHERE c.issue_id=i.id) AS collaboration
    FROM issue i WHERE i.workspace_id=$1 AND i.id=ANY($2::uuid[]) ORDER BY i.id LIMIT 129`,
  subscribers: `SELECT s.issue_id,i.workspace_id,s.user_type,s.user_id,s.reason,s.created_at::text,s.unsubscribed_at::text,s.opt_out_scope,m.id AS member_id
    FROM issue_subscriber s JOIN issue i ON i.id=s.issue_id
    LEFT JOIN member m ON s.user_type='member' AND m.workspace_id=i.workspace_id AND m.user_id=s.user_id
    WHERE i.workspace_id=$1 AND s.issue_id=ANY($2::uuid[]) ORDER BY s.issue_id,s.user_type,s.user_id LIMIT 129`,
  inbox: `SELECT id,workspace_id,recipient_type,recipient_id,type,severity,issue_id,actor_type,actor_id,
    details->>'comment_id' AS comment_id,encode(sha256(convert_to(title,'UTF8')),'hex') AS title_sha256,
    encode(sha256(convert_to(body,'UTF8')),'hex') AS body_sha256,encode(sha256(convert_to(details::text,'UTF8')),'hex') AS details_sha256,
    read,archived,created_at::text FROM inbox_item WHERE workspace_id=$1 AND issue_id=ANY($2::uuid[]) ORDER BY issue_id,created_at,id LIMIT 129`,
  comments: `SELECT id,issue_id,workspace_id,author_type,author_id,type,revision::text,parent_id,source_task_id,quick_action_id,
    encode(sha256(convert_to(content,'UTF8')),'hex') AS content_sha256,created_at::text,updated_at::text,deleted_at::text
    FROM comment WHERE workspace_id=$1 AND issue_id=ANY($2::uuid[]) ORDER BY issue_id,created_at,id LIMIT 129`,
  activities: `SELECT a.id,a.issue_id,encode(sha256(convert_to(to_jsonb(a)::text,'UTF8')),'hex') AS state_sha256
    FROM activity_log a JOIN issue i ON i.id=a.issue_id WHERE i.workspace_id=$1 AND a.issue_id=ANY($2::uuid[]) ORDER BY a.issue_id,a.id LIMIT 129`,
  collaboration: `SELECT c.issue_id,encode(sha256(convert_to(to_jsonb(c)::text,'UTF8')),'hex') AS state_sha256
    FROM issue_collaboration c JOIN issue i ON i.id=c.issue_id WHERE i.workspace_id=$1 AND c.issue_id=ANY($2::uuid[]) ORDER BY c.issue_id LIMIT 129`,
  sessions: `SELECT s.id,s.issue_id,encode(sha256(convert_to(to_jsonb(s)::text,'UTF8')),'hex') AS state_sha256
    FROM collaboration_session s JOIN issue i ON i.id=s.issue_id WHERE i.workspace_id=$1 AND s.issue_id=ANY($2::uuid[]) ORDER BY s.issue_id,s.id LIMIT 129`,
  events: `SELECT e.id,e.issue_id,e.event_seq::text,e.actor_user_id,e.kind,e.created_at::text,
    encode(sha256(convert_to(e.payload::text,'UTF8')),'hex') AS payload_sha256 FROM collaboration_event e JOIN issue i ON i.id=e.issue_id
    WHERE i.workspace_id=$1 AND e.issue_id=ANY($2::uuid[]) ORDER BY e.issue_id,e.event_seq,e.id LIMIT 129`,
  operations: `SELECT o.id,o.scope_id,o.actor_user_id,o.request_id,o.kind,o.payload_hash,o.result_code,o.completed_at::text,
    encode(sha256(convert_to(o.result::text,'UTF8')),'hex') AS result_sha256 FROM collaboration_operation o JOIN issue i ON i.id=o.scope_id
    WHERE i.workspace_id=$1 AND o.scope_kind='issue' AND o.scope_id=ANY($2::uuid[]) ORDER BY o.scope_id,o.id LIMIT 129`,
  notifications: `SELECT n.id,n.issue_id,n.recipient_user_id,n.event_seq::text,n.kind,n.source_kind,n.source_id,n.run_id,n.read_at::text,n.archived_at::text
    FROM collaboration_notification n JOIN issue i ON i.id=n.issue_id WHERE i.workspace_id=$1 AND n.issue_id=ANY($2::uuid[])
    ORDER BY n.issue_id,n.event_seq,n.recipient_user_id,n.id LIMIT 129`,
  commands: `SELECT c.id,s.issue_id FROM collaboration_command c JOIN collaboration_session s ON s.id=c.session_id JOIN issue i ON i.id=s.issue_id
    WHERE i.workspace_id=$1 AND s.issue_id=ANY($2::uuid[]) ORDER BY s.issue_id,c.id LIMIT 129`,
  groups: `SELECT g.id,s.issue_id FROM collaboration_execution_group g JOIN collaboration_session s ON s.id=g.session_id JOIN issue i ON i.id=s.issue_id
    WHERE i.workspace_id=$1 AND s.issue_id=ANY($2::uuid[]) ORDER BY s.issue_id,g.id LIMIT 129`,
  runs: `SELECT r.id,s.issue_id FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id JOIN issue i ON i.id=s.issue_id
    WHERE i.workspace_id=$1 AND s.issue_id=ANY($2::uuid[]) ORDER BY s.issue_id,r.id LIMIT 129`,
  dispatches: `SELECT q.id,q.issue_id FROM agent_task_queue q JOIN issue i ON i.id=q.issue_id
    WHERE i.workspace_id=$1 AND q.issue_id=ANY($2::uuid[]) ORDER BY q.issue_id,q.id LIMIT 129`,
  attachments: `SELECT a.id,a.issue_id FROM attachment a JOIN issue i ON i.id=a.issue_id
    WHERE i.workspace_id=$1 AND a.issue_id=ANY($2::uuid[]) ORDER BY a.issue_id,a.id LIMIT 129`,
  collaboration_attachments: `SELECT a.id,a.issue_id FROM collaboration_attachment a JOIN issue i ON i.id=a.issue_id
    WHERE i.workspace_id=$1 AND a.issue_id=ANY($2::uuid[]) ORDER BY a.issue_id,a.id LIMIT 129`,
  delegation_receipts: `SELECT request_key,issue_id,comment_id,state FROM comment_delegation_receipt
    WHERE workspace_id=$1 AND issue_id=ANY($2::uuid[]) ORDER BY issue_id,request_key LIMIT 129`,
};

async function audit(fixture: SubscriptionFixture, evidence: RecoveryEvidence, phase: string): Promise<Audit> {
  evidence.set({ active_phase: phase });
  const database = new pg.Client({ connectionString: process.env.DATABASE_URL!, connectionTimeoutMillis: 5_000 });
  let primary: unknown;
  let cleanupFailure: { error: unknown } | undefined;
  let result: Audit;
  try {
    await database.connect();
    await database.query("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY");
    await database.query("SET LOCAL statement_timeout='5s'");
    await database.query("SET LOCAL idle_in_transaction_session_timeout='10s'");
    await database.query("SET LOCAL TIME ZONE 'UTC'");
    const identity = await database.query<{ database: string; read_only: string }>(
      "SELECT current_database() AS database,current_setting('transaction_read_only') AS read_only");
    expect(identity.rows).toEqual([{ database: process.env.TASK_DATABASE_NAME, read_only: "on" }]);
    const rows: AuditRows = {};
    const bindings = [fixture.workspace.id, [fixture.surface.issueId, fixture.child.id]];
    for (const [name, sql] of Object.entries(auditQueries)) {
      const result = await database.query<Record<string, unknown>>(sql, bindings);
      rows[name] = result.rows;
      evidence.checkpoint(`${phase}_SQL_${name}`, { sql, bindings, rows: result.rows, rows_sha256: digest(result.rows), row_limit: 128 });
      expect(result.rows.length, `The complete ${name} inventory must fit its explicit bound`).toBeLessThanOrEqual(128);
    }
    await database.query("COMMIT");
    for (const name of ["commands", "groups", "runs", "dispatches", "attachments", "collaboration_attachments", "delegation_receipts"]) expect(rows[name]).toEqual([]);
    expect(rows.issues?.filter((row) => row.id === fixture.child.id)).toMatchObject([{ parent_issue_id: fixture.surface.issueId, collaboration: false }]);
    expect(rows.collaboration?.map((row) => row.issue_id)).toEqual([fixture.surface.issueId]);
    result = { rows, subscribers: z.array(subscriberRowSchema).parse(rows.subscribers),
      inbox: z.array(inboxRowSchema).parse(rows.inbox), comments: z.array(commentRowSchema).parse(rows.comments) };
    evidence.checkpoint(phase, { database: process.env.TASK_DATABASE_NAME, counts: Object.fromEntries(Object.entries(rows).map(([name, items]) => [name, items.length])),
      rows_sha256: digest(rows), no_dispatch: true, ordinary_comment_and_collaboration_effects_separate: true });
  } catch (error) {
    primary = error;
    evidence.checkpoint(`${phase}_primary_audit_failure`, { error_type: error instanceof Error ? error.name : "unknown_error" });
    try { await database.query("ROLLBACK"); }
    catch (rollback) { evidence.checkpoint(`${phase}_rollback_failure`, { error_type: rollback instanceof Error ? rollback.name : "unknown_error" }); }
    throw error;
  } finally {
    try { await database.end(); evidence.checkpoint(`${phase}_audit_connection_closed`); }
    catch (cleanup) {
      cleanupFailure = { error: cleanup };
      evidence.checkpoint(`${phase}_audit_connection_cleanup_failure`, { error_type: cleanup instanceof Error ? cleanup.name : "unknown_error", primary_failure_preserved: primary !== undefined });
    }
  }
  if (cleanupFailure) throw cleanupFailure.error;
  return result;
}

function assertParentPreserved(fixture: SubscriptionFixture, before: Audit, after: Audit) {
  for (const name of ["collaboration", "sessions", "events", "operations", "notifications"]) expect(after.rows[name]).toEqual(before.rows[name]);
  expect(after.rows.issues?.filter((row) => row.id === fixture.surface.issueId)).toEqual(before.rows.issues?.filter((row) => row.id === fixture.surface.issueId));
  expect(after.subscribers.filter((row) => row.issue_id === fixture.surface.issueId)).toEqual([]);
  expect(after.comments.filter((row) => row.issue_id === fixture.surface.issueId)).toEqual([]);
  expect(after.inbox.filter((row) => row.issue_id === fixture.surface.issueId)).toEqual([]);
}

function subscribersFor(fixture: SubscriptionFixture, value: Audit, actor: RecoveryActor) {
  return value.subscribers.filter((row) => row.issue_id === fixture.child.id && row.user_type === "member" && row.user_id === actor.id);
}
function assertControls(fixture: SubscriptionFixture, value: Audit, optout: SubscriberRow[]) {
  expect(subscribersFor(fixture, value, fixture.actors.nonwatcher)).toEqual([]);
  expect(subscribersFor(fixture, value, fixture.actors.optout)).toEqual(optout);
  expect(subscribersFor(fixture, value, fixture.actors.commenter)).toMatchObject([{ unsubscribed_at: null }]);
  for (const actor of Object.values(fixture.actors)) {
    for (const row of subscribersFor(fixture, value, actor)) expect(row.member_id).not.toBeNull();
  }
}

function isIssueTableQueryRead(fact: WriteFact) {
  return fact.method === "POST" && (fact.path === "/api/issues/table/rows" || fact.path === "/api/issues/table/facets");
}

function observeOrdinary(session: RecoverySession, evidence: RecoveryEvidence) {
  const writes: WriteFact[] = [], sockets: SocketBinding[] = [], pending = new Set<Promise<void>>();
  const onRequest = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (!path.startsWith("/api/") || ["GET", "HEAD", "OPTIONS"].includes(request.method())) return;
    const observed = (async () => {
      const headers = await request.allHeaders();
      writes.push({ at: now(), actor_id: session.actor.id, method: request.method(), path,
        payload_sha256: request.postData() === null ? null : sha(request.postData()!), request_id: headers["idempotency-key"] ?? null,
        workspace_id: headers["x-workspace-id"] ?? null, workspace_slug: headers["x-workspace-slug"] ?? null,
        authentication: { cookie: Boolean(headers.cookie), csrf: Boolean(headers["x-csrf-token"]), bearer: Boolean(headers.authorization) } });
    })().catch((error) => { session.errors.push(`Ordinary request observation failed: ${error instanceof Error ? error.name : "unknown_error"}`); });
    pending.add(observed); session.pendingReads.add(observed);
    void observed.finally(() => { pending.delete(observed); session.pendingReads.delete(observed); });
  };
  const onSocket = (event: { requestId: string; url: string }) => {
    const url = new URL(event.url);
    expect(url.searchParams.has("token")).toBe(false);
    sockets.push({ id: event.requestId, at: now(), workspace_slug: url.searchParams.get("workspace_slug"), scope: url.searchParams.get("scope") });
  };
  session.page.on("request", onRequest); session.cdp.on("Network.webSocketCreated", onSocket);
  return { writes, sockets, async settle() { await Promise.all([...pending]); }, async finish() {
    await Promise.all([...pending]);
    session.page.off("request", onRequest); session.cdp.off("Network.webSocketCreated", onSocket);
    evidence.checkpoint("complete_ordinary_client_transport_inventory", { actor_id: session.actor.id, writes,
      query_read_facts: writes.filter(isIssueTableQueryRead), socket_bindings: sockets });
  } };
}
type OrdinaryObserver = ReturnType<typeof observeOrdinary>;

async function workspaceSocket(session: RecoverySession, fixture: SubscriptionFixture, observer: OrdinaryObserver, after: string, previousId?: string) {
  const find = () => [...session.sockets].reverse().find((socket) => socket.created_at >= after && socket.id !== previousId && socket.closed_at === null &&
    observer.sockets.some((binding) => binding.id === socket.id && binding.workspace_slug === fixture.workspace.slug) &&
    session.handshakes.some((handshake) => handshake.request_id === socket.id && handshake.status === 101) &&
    (session.platform === "Web" || socket.frames.some((frame) => frame.direction === "received" && frame.type === "auth_ack")));
  await expect.poll(() => find() !== undefined, { timeout: 35_000, message: "The actual workspace socket must complete its authenticated handshake" }).toBe(true);
  const socket = find()!;
  if (session.platform === "Desktop") {
    expect(socket.frames.findIndex((frame) => frame.direction === "sent" && frame.type === "auth")).toBe(0);
    expect(socket.frames.findIndex((frame) => frame.direction === "received" && frame.type === "auth_ack")).toBeGreaterThan(0);
  } else expect(socket.frames.some((frame) => frame.direction === "sent" && frame.type === "auth")).toBe(false);
  // Ordinary workspace sockets join by the real workspace_slug handshake.
  // They do not use collaboration's persisted participant or issue subscribe_ack.
  return socket;
}

async function openChild(session: RecoverySession, fixture: SubscriptionFixture) {
  const drawer = await revealCollaborationDisclosure(session.page, "Sub-issues");
  const link = drawer.getByRole("link", { name: `${fixture.child.identifier} ${fixture.child.title}`, exact: true });
  await expect(link).toBeVisible(); await link.click();
  await expect(session.page.getByRole("button", { name: fixture.child.title, exact: true })).toBeVisible();
  await expect(session.page.getByRole("button", { name: /^(Subscribe|Unsubscribe)$/, exact: true })).toBeVisible();
}

async function openInbox(session: RecoverySession, fixture: SubscriptionFixture) {
  if (session.platform === "Desktop") await selectDesktopWorkspace(session.page, fixture.workspace);
  const link = session.page.locator('[data-sidebar="sidebar"]').getByRole("link", { name: /^Inbox(?:\s+\d+)?$/ });
  await expect(link).toHaveCount(1); await expect(link).toBeVisible();
  const destination = await link.getAttribute("href");
  expect(new URL(destination!, session.page.url()).pathname).toBe(`/${fixture.workspace.slug}/inbox`);
  await link.click();
  await expect(session.page.getByRole("heading", { name: "Inbox", exact: true })).toBeVisible();
}

async function uiResponse(session: RecoverySession, fixture: SubscriptionFixture, path: string, method: string, action: () => Promise<unknown>,
  evidence: RecoveryEvidence, phase: string, expectedStatus = 200): Promise<Response> {
  evidence.set({ active_phase: phase });
  const [response] = await Promise.all([session.page.waitForResponse((value) => new URL(value.url()).pathname === path && value.request().method() === method,
    { timeout: 15_000 }), action()]);
  const headers = await response.request().allHeaders();
  expect(response.status()).toBe(expectedStatus);
  // The production composer/inbox client uses the route's slug header; the
  // TestApiClient audits below bind the same workspace by its explicit UUID.
  expect(headers["x-workspace-slug"]).toBe(fixture.workspace.slug);
  if (headers["x-workspace-id"]) expect(headers["x-workspace-id"]).toBe(fixture.workspace.id);
  const authentication = { cookie: Boolean(headers.cookie), csrf: Boolean(headers["x-csrf-token"]), bearer: Boolean(headers.authorization) };
  if (session.platform === "Web") expect(authentication).toEqual({ cookie: true, csrf: true, bearer: false });
  else expect(authentication.bearer).toBe(true);
  evidence.checkpoint(phase, { actor_id: session.actor.id, path, method, status: response.status(),
    request_id: headers["idempotency-key"] ?? null, payload_sha256: sha(response.request().postData() ?? ""),
    workspace_id: headers["x-workspace-id"] ?? null, workspace_slug: headers["x-workspace-slug"], authentication });
  return response;
}

function editorAncestorObservation(editor: Element) {
  const ancestors: { depth: number; tag: string; classes: string; role: string | null; send_count: number }[] = [];
  let ancestor = editor.parentElement;
  for (let depth = 0; ancestor && depth < 16; depth++, ancestor = ancestor.parentElement) {
    ancestors.push({ depth, tag: ancestor.tagName.toLowerCase(), classes: (ancestor.getAttribute("class") ?? "").slice(0, 256),
      role: ancestor.getAttribute("role")?.slice(0, 64) ?? null, send_count: ancestor.querySelectorAll('button[aria-label="Send"]').length });
  }
  return { editor_connected: editor.isConnected, editor_tag: editor.tagName.toLowerCase(),
    editor_classes: (editor.getAttribute("class") ?? "").slice(0, 256), editor_role: editor.getAttribute("role")?.slice(0, 64) ?? null,
    placeholder_node_count: editor.querySelectorAll("[data-placeholder]").length + Number(editor.hasAttribute("data-placeholder")),
    ancestors, ancestor_limit: 16, class_max_chars: 256, role_max_chars: 64, ancestors_truncated: ancestor !== null };
}

async function submitComment(session: RecoverySession, fixture: SubscriptionFixture, evidence: RecoveryEvidence, phase: string): Promise<OrdinaryComment> {
  const content = `Ordinary unmentioned ${phase.replaceAll("_", " ")} ${randomUUID()}`;
  const shell = session.page.getByTestId("comment-composer-shell");
  if (await shell.isVisible()) await shell.click();
  const emptyEditor = session.page.locator('.ProseMirror[data-placeholder="Leave a comment..."], .ProseMirror:has([data-placeholder="Leave a comment..."])');
  await expect(emptyEditor).toHaveCount(1); await expect(emptyEditor).toBeVisible();
  const originalEditor = await emptyEditor.elementHandle();
  if (!originalEditor) throw new Error("The original empty main editor is unavailable");
  evidence.checkpoint(`${phase}_main_editor_ancestors_before_fill`, { actor_id: session.actor.id, issue_id: fixture.child.id,
    ...await originalEditor.evaluate(editorAncestorObservation) });
  // Retain the original owner while typing removes placeholders and other editors may exist.
  const owners = await originalEditor.$$('xpath=ancestor::*[not(self::html) and not(self::body) and .//button[@aria-label="Send"]][1]');
  expect(owners).toHaveLength(1);
  const [composer] = owners;
  if (!composer) throw new Error("The original main editor has no unique submit owner");
  expect(await composer.evaluate((node, original) => node.isConnected && node.contains(original), originalEditor)).toBe(true);
  expect(await composer.$$eval('.ProseMirror[contenteditable="true"]', (nodes) => nodes.length)).toBe(1);
  expect(await composer.$eval('.ProseMirror[contenteditable="true"]', (node, original) => node === original && node.isConnected, originalEditor)).toBe(true);
  expect(await composer.$$eval('button[aria-label="Send"]', (nodes) => nodes.length)).toBe(1);
  await originalEditor.fill(content);
  evidence.checkpoint(`${phase}_main_editor_ancestors_after_fill`, { actor_id: session.actor.id, issue_id: fixture.child.id,
    ...await originalEditor.evaluate(editorAncestorObservation) });
  expect(await composer.evaluate((node, original) => node.isConnected && node.contains(original), originalEditor)).toBe(true);
  expect(await composer.$$eval('.ProseMirror[contenteditable="true"]', (nodes) => nodes.length)).toBe(1);
  expect(await composer.$eval('.ProseMirror[contenteditable="true"]', (node, original) => node === original && node.isConnected, originalEditor)).toBe(true);
  const sends = await composer.$$('button[aria-label="Send"]');
  expect(sends).toHaveLength(1);
  const [send] = sends;
  if (!send) throw new Error("The original main editor submit owner has no unique Send button");
  expect(await send.evaluate((node, owner) => node.isConnected && owner.contains(node), composer)).toBe(true);
  await expect.poll(() => send.isEnabled()).toBe(true);
  const response = await uiResponse(session, fixture, `/api/issues/${fixture.child.id}/comments`, "POST",
    () => send.click(), evidence, phase, 201);
  await Promise.all([originalEditor.dispose(), composer.dispose(), send.dispose()]);
  const request = object.parse(response.request().postDataJSON());
  expect(z.string().parse(request.content).trim()).toBe(content); expect(response.request().headers()["idempotency-key"]).toBeUndefined();
  const comment = commentSchema.parse(await response.json());
  expect(comment).toMatchObject({ issue_id: fixture.child.id, author_type: "member", author_id: fixture.actors.commenter.id,
    type: "comment", parent_id: null, attachments: [] });
  expect(comment.content.trim()).toBe(content); expect(comment.source_task_id == null).toBe(true);
  const body = session.page.locator(`#comment-body-${comment.id}`);
  await expect.poll(async () => sha((await body.textContent() ?? "").trim())).toBe(sha(content));
  evidence.checkpoint(`${phase}_original_comment_identity`, { id: comment.id, issue_id: comment.issue_id, actor_id: comment.author_id,
    revision: comment.revision, content_sha256: sha(comment.content), created_at: comment.created_at,
    transport: "ACTUAL_UNKEYED_ORDINARY_COMPOSER", mentions: 0, attachments: 0 });
  return comment;
}

function inboxFact(item: Inbox) {
  return { id: item.id, issue_id: item.issue_id, recipient_type: item.recipient_type, recipient_id: item.recipient_id,
    type: item.type, actor_type: item.actor_type, actor_id: item.actor_id, comment_id: item.details?.comment_id ?? null,
    read: item.read, archived: item.archived, created_at: item.created_at, title_sha256: sha(item.title), body_sha256: item.body == null ? null : sha(item.body) };
}
async function recipientInboxes(fixture: SubscriptionFixture, evidence: RecoveryEvidence, phase: string) {
  const result: Record<string, Inbox[]> = {};
  for (const actor of Object.values(fixture.actors)) {
    const items = z.array(inboxSchema).parse(await actor.client.requestJSON("/api/inbox"));
    result[actor.id] = items.filter((item) => item.issue_id === fixture.child.id);
    expect(result[actor.id]!.every((item) => item.workspace_id === fixture.workspace.id && item.recipient_id === actor.id && item.recipient_type === "member")).toBe(true);
  }
  evidence.checkpoint(phase, { recipients: Object.fromEntries(Object.entries(result).map(([id, items]) => [id, items.map(inboxFact)])),
    complete_recipient_actor_ids: Object.values(fixture.actors).map((actor) => actor.id) });
  return result;
}
function assertCommentDelivery(fixture: SubscriptionFixture, value: Audit, comment: OrdinaryComment, recipients: string[]) {
  const rows = value.inbox.filter((row) => row.comment_id === comment.id);
  expect(rows.map((row) => row.recipient_id).sort()).toEqual([...recipients].sort());
  expect(new Set(rows.map((row) => row.id)).size).toBe(rows.length);
  for (const row of rows) expect(row).toMatchObject({ issue_id: fixture.child.id, workspace_id: fixture.workspace.id, type: "new_comment",
    recipient_type: "member", actor_type: "member", actor_id: fixture.actors.commenter.id });
  expect(value.comments.filter((row) => row.id === comment.id)).toMatchObject([{ issue_id: fixture.child.id, workspace_id: fixture.workspace.id,
    author_type: "member", author_id: fixture.actors.commenter.id, revision: `${comment.revision}`, content_sha256: sha(comment.content), deleted_at: null }]);
}

async function assertInboxAnchor(session: RecoverySession, fixture: SubscriptionFixture, notice: Inbox, comment: OrdinaryComment) {
  const row = session.page.getByRole("listitem").filter({ has: session.page.getByRole("button", { name: fixture.child.title, exact: true }) });
  await expect(row).toHaveCount(1);
  await expect(row.getByRole("button", { name: fixture.child.title, exact: true })).toHaveAttribute("data-inbox-selection", notice.id);
  const body = session.page.locator(`#comment-body-${comment.id}`);
  await expect(body).toBeVisible(); await expect(body).toBeInViewport();
  await expect.poll(async () => sha((await body.textContent() ?? "").trim())).toBe(sha(comment.content.trim()));
  await expect(row.getByRole("button", { name: fixture.child.title, exact: true })).toHaveAttribute("aria-current", "true");
}

export async function exerciseRecoverySubscriptions(browser: Browser, platform: RecoveryPlatform, info: TestInfo, evidence: RecoveryEvidence) {
  const fixture = await seedSubscriptions(platform, evidence);
  const before = await audit(fixture, evidence, "seeded_complete_rows");
  const optout = subscribersFor(fixture, before, fixture.actors.optout);
  expect(optout).toMatchObject([{ reason: "manual", opt_out_scope: "issue", unsubscribed_at: expect.any(String) }]);
  expect(subscribersFor(fixture, before, fixture.actors.watcher)).toEqual([]); assertControls(fixture, before, optout);
  return withSessions(browser, platform, fixture.surface, info, evidence, async (open) => {
    const author = await open(fixture.actors.commenter), watcher = await open(fixture.actors.watcher);
    const authorObserver = observeOrdinary(author, evidence), watcherObserver = observeOrdinary(watcher, evidence);
    try {
      const navigationStarted = now();
      await openChild(author, fixture); await openChild(watcher, fixture);
      const initialSocket = await workspaceSocket(watcher, fixture, watcherObserver, navigationStarted);
      const subscribed = await uiResponse(watcher, fixture, `/api/issues/${fixture.child.id}/subscribe`, "POST",
        () => watcher.page.getByRole("button", { name: "Subscribe", exact: true }).click(), evidence, "actual_Subscribe_control");
      expect(await subscribed.json()).toEqual({ subscribed: true });
      await expect(watcher.page.getByRole("button", { name: "Unsubscribe", exact: true })).toBeVisible();
      const subscribedRows = await audit(fixture, evidence, "manual_subscription_rows");
      const manual = subscribersFor(fixture, subscribedRows, fixture.actors.watcher);
      expect(manual).toMatchObject([{ reason: "manual", unsubscribed_at: null, opt_out_scope: null, member_id: expect.any(String) }]);
      const active = SubscribersListSchema.parse(await fixture.actors.watcher.client.requestJSON(`/api/issues/${fixture.child.id}/subscribers`));
      expect(active.map((item) => item.user_id).sort()).toEqual([fixture.actors.commenter.id, fixture.actors.watcher.id].sort());
      assertControls(fixture, subscribedRows, optout); assertParentPreserved(fixture, before, subscribedRows);

      const first = await submitComment(author, fixture, evidence, "first_comment");
      await expect.poll(async () => (await recipientInboxes(fixture, evidence, "first_comment_recipient_poll"))[fixture.actors.watcher.id]?.map((item) => item.details?.comment_id))
        .toEqual([first.id]);
      const inboxes = await recipientInboxes(fixture, evidence, "first_comment_all_API_recipients");
      for (const actor of [fixture.actors.commenter, fixture.actors.nonwatcher, fixture.actors.optout]) expect(inboxes[actor.id]).toEqual([]);
      const notice = inboxes[fixture.actors.watcher.id]![0]!;
      expect(notice).toMatchObject({ type: "new_comment", read: false, archived: false, details: { comment_id: first.id } });
      const delivered = await audit(fixture, evidence, "first_comment_all_persisted_recipients");
      assertCommentDelivery(fixture, delivered, first, [fixture.actors.watcher.id]);
      expect(delivered.inbox).toHaveLength(1); expect(delivered.inbox[0]?.id).toBe(notice.id);
      assertControls(fixture, delivered, optout); assertParentPreserved(fixture, before, delivered);

      await openInbox(watcher, fixture);
      const row = watcher.page.getByRole("listitem").filter({ has: watcher.page.getByRole("button", { name: fixture.child.title, exact: true }) });
      await expect(row).toHaveCount(1); await expect(row).toContainText(first.content);
      const read = await uiResponse(watcher, fixture, `/api/inbox/${notice.id}/read`, "POST",
        () => row.getByRole("button", { name: fixture.child.title, exact: true }).click(), evidence, "actual_original_inbox_selection");
      const readNotice = inboxSchema.parse(await read.json());
      expect(readNotice).toMatchObject({ id: notice.id, read: true, archived: false, details: { comment_id: first.id } });
      await assertInboxAnchor(watcher, fixture, readNotice, first);
      const readRows = await audit(fixture, evidence, "original_notice_read_state");
      expect(readRows.inbox).toEqual(delivered.inbox.map((item) => ({ ...item, read: true })));

      await authorObserver.settle(); await watcherObserver.settle();
      const observerWrites = watcherObserver.writes.length, authorWrites = authorObserver.writes.length;
      const currentSocket = watcher.sockets.find((socket) => socket.id === initialSocket.id && socket.closed_at === null)
        ?? await workspaceSocket(watcher, fixture, watcherObserver, navigationStarted);
      const disconnectedAt = now();
      await disconnectObserver(watcher, currentSocket, evidence, "actual_ordinary_socket_offline");
      await watcher.context.setOffline(false);
      const reconnected = await workspaceSocket(watcher, fixture, watcherObserver, disconnectedAt, currentSocket.id);
      await assertInboxAnchor(watcher, fixture, readNotice, first);
      if (platform === "Web") {
        await expect.poll(() => new URL(watcher.page.url()).searchParams.get("issue")).toBe(fixture.child.id);
      }
      const reloadAt = now();
      const [listed] = await Promise.all([watcher.page.waitForResponse((response) => new URL(response.url()).pathname === "/api/inbox" && response.request().method() === "GET" && response.status() === 200,
        { timeout: 15_000 }), watcher.page.reload({ waitUntil: "domcontentloaded" })]);
      const reloaded = await workspaceSocket(watcher, fixture, watcherObserver, reloadAt, reconnected.id);
      const renderedList = z.array(inboxSchema).parse(await listed.json()).filter((item) => item.issue_id === fixture.child.id);
      expect(renderedList.map(inboxFact)).toEqual([inboxFact(readNotice)]);
      await assertInboxAnchor(watcher, fixture, readNotice, first);
      await authorObserver.settle(); await watcherObserver.settle();
      expect(watcherObserver.writes).toHaveLength(observerWrites); expect(authorObserver.writes).toHaveLength(authorWrites);
      const recovered = await audit(fixture, evidence, "genuine_reconnect_reload_complete_rows");
      expect(recovered.rows).toEqual(readRows.rows);
      evidence.checkpoint("genuine_reconnect_reload_kept_original_notice", { comment_id: first.id, inbox_id: notice.id, read: true, archived: false,
        original_socket_id: currentSocket.id, reconnect_socket_id: reconnected.id, reload_socket_id: reloaded.id,
        all_observed_socket_ids: watcher.sockets.map((socket) => socket.id), additional_business_writes: 0,
        workspace_handshake: true, literal_event_replay_claimed: false });

      const unsubscribeButton = watcher.page.getByRole("button", { name: "Unsubscribe", exact: true })
        .and(watcher.page.locator('button:not([aria-haspopup="menu"])'));
      await expect(unsubscribeButton).toBeVisible();
      const unsubscribed = await uiResponse(watcher, fixture, `/api/issues/${fixture.child.id}/unsubscribe`, "POST",
        () => unsubscribeButton.click(), evidence, "actual_Unsubscribe_control");
      expect(await unsubscribed.json()).toEqual({ subscribed: false });
      await expect(watcher.page.getByRole("button", { name: "Subscribe", exact: true })).toBeVisible();
      const unsubscribedRows = await audit(fixture, evidence, "manual_subscription_tombstone");
      const tombstone = subscribersFor(fixture, unsubscribedRows, fixture.actors.watcher);
      expect(tombstone).toMatchObject([{ reason: "manual", opt_out_scope: "issue", unsubscribed_at: expect.any(String), created_at: manual[0]!.created_at }]);
      expect(unsubscribedRows.inbox).toEqual(readRows.inbox);
      const second = await submitComment(author, fixture, evidence, "second_comment_after_unsubscribe");
      expect(second.id).not.toBe(first.id);
      const final = await audit(fixture, evidence, "unsubscribe_distinct_second_comment_complete_rows");
      assertCommentDelivery(fixture, final, first, [fixture.actors.watcher.id]); assertCommentDelivery(fixture, final, second, []);
      expect(final.inbox).toEqual(readRows.inbox); expect(final.comments.map((comment) => comment.id).sort()).toEqual([first.id, second.id].sort());
      expect(subscribersFor(fixture, final, fixture.actors.watcher)).toEqual(tombstone);
      assertControls(fixture, final, optout); assertParentPreserved(fixture, before, final);
      const finalInboxes = await recipientInboxes(fixture, evidence, "final_all_API_recipients");
      expect(finalInboxes[fixture.actors.watcher.id]?.map(inboxFact)).toEqual([inboxFact(readNotice)]);
      for (const actor of [fixture.actors.commenter, fixture.actors.nonwatcher, fixture.actors.optout]) expect(finalInboxes[actor.id]).toEqual([]);
      expect(SubscribersListSchema.parse(await fixture.actors.watcher.client.requestJSON(`/api/issues/${fixture.child.id}/subscribers`))
        .map((item) => item.user_id)).toEqual([fixture.actors.commenter.id]);
      await authorObserver.settle(); await watcherObserver.settle();
      expect(authorObserver.writes.filter((write) => write.path === `/api/issues/${fixture.child.id}/comments` && write.method === "POST")).toHaveLength(2);
      expect(watcherObserver.writes.filter((write) => !isIssueTableQueryRead(write)).map((write) => `${write.method} ${write.path}`)).toEqual([
        `POST /api/issues/${fixture.child.id}/subscribe`, `POST /api/inbox/${notice.id}/read`, `POST /api/issues/${fixture.child.id}/unsubscribe`,
      ]);
      return { platform, parent_issue_id: fixture.surface.issueId, child_issue_id: fixture.child.id, original_comment_id: first.id,
        second_comment_id: second.id, inbox_id: notice.id, first_recipients: [fixture.actors.watcher.id], second_recipients: [],
        manual_subscribe_UI_writes: 1, manual_unsubscribe_UI_writes: 1, original_notice_read_UI_writes: 1,
        ordinary_composer_comments: 2, genuine_offline_reconnects: 1, actual_reloads: 1,
        literal_event_replay: "SEPARATE_GO_LISTENER_ACCEPTANCE", API_key_replay: "NOT_RUN", no_dispatch: true,
        retained_business_rows: Object.fromEntries(Object.entries(final.rows).map(([name, rows]) => [name, rows.length])) };
    } finally {
      const cleanup = await Promise.allSettled([authorObserver.finish(), watcherObserver.finish()]);
      evidence.checkpoint("ordinary_observer_cleanup", { outcomes: cleanup.map((result) => result.status === "fulfilled" ? { status: result.status }
        : { status: result.status, error_type: result.reason instanceof Error ? result.reason.name : "unknown_error" }) });
      if (cleanup.some((result) => result.status === "rejected")) watcher.cleanup_errors.push({ type: "ordinary_observer_cleanup_failed" });
    }
  });
}
