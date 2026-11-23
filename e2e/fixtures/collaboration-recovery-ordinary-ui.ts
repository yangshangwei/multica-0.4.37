import { createHash, randomUUID } from "node:crypto";
import { expect, type Browser, type Locator, type Request, type Response, type TestInfo } from "@playwright/test";
import pg from "pg";
import { z } from "zod";
import { CreateIssueResponseSchema } from "@multica/core/api/schemas";
import { CollaborationIssueSchema } from "@multica/core/collaboration";
import {
  HeldRequest, RecoveryEvidence, openOrdinaryChild, recoveryEnvironmentReady, seedRecovery, withSessions,
  type RecoveryPlatform, type RecoverySession,
} from "./collaboration-recovery";

const now = () => new Date().toISOString();
const digest = (value: unknown) => createHash("sha256").update(JSON.stringify(value)).digest("hex");
const ordinaryIssueSchema = CreateIssueResponseSchema.extend({
  id: z.uuid(), workspace_id: z.uuid(), identifier: z.string().min(1), title: z.string().min(1),
  revision: z.number().int().positive().safe(), priority: z.enum(["none", "low", "medium", "high", "urgent"]),
  assignee_type: z.string().nullable(), assignee_id: z.uuid().nullable(), parent_issue_id: z.uuid(),
  collaboration: z.null().nullish(),
});
type OrdinaryIssue = z.infer<typeof ordinaryIssueSchema>;
export const ordinaryUISubcaseNames = ["priority-first", "assignee-first", "versionlessAPI"] as const;
type Subcase = typeof ordinaryUISubcaseNames[number];
export const ordinaryUIJourneyNames = ["Web", "Desktop"].map((platform) =>
  `${platform} ordinary child priority and assignee UI retain observed numeric revisions with a separate versionlessAPI control`);
const conflictMessage = "The issue or comment changed. The latest version was refreshed; compare it before saving again.";

export function recoveryOrdinaryUIEnvironmentReady() {
  return recoveryEnvironmentReady() && process.env.PAY241_S3_ORDINARY_UI_CAS_ACCEPTANCE === "1";
}

export async function seedRecoveryOrdinaryUI(evidence: RecoveryEvidence) {
  if (!recoveryOrdinaryUIEnvironmentReady()) throw new Error("Ordinary UI CAS requires its explicit password/source/database allocation");
  const recovery = await seedRecovery(evidence);
  const children = {} as Record<Subcase, OrdinaryIssue>;
  for (const subcase of ordinaryUISubcaseNames) {
    const child = ordinaryIssueSchema.parse(await recovery.creator.client.createIssue(`Ordinary UI ${subcase} ${randomUUID().slice(0, 8)}`, {
      parent_issue_id: recovery.issueId, status: "todo", priority: "low", assignee_type: "member", assignee_id: recovery.creator.id,
      description: "Genuine ordinary field pickers preserve the protected collaboration parent and execution history.",
    }));
    expect(child).toMatchObject({ workspace_id: recovery.workspace.id, parent_issue_id: recovery.issueId,
      priority: "low", assignee_type: "member", assignee_id: recovery.creator.id });
    children[subcase] = child;
    evidence.checkpoint("distinct_ordinary_child_seeded", { subcase, child });
  }
  expect(new Set(Object.values(children).map((child) => child.id)).size).toBe(3);
  evidence.set({ child_ids: Object.fromEntries(ordinaryUISubcaseNames.map((name) => [name, children[name].id])),
    UI_client_authority: "Two separately authenticated sessions for the same valid source member",
    planned_UI_orders: 2, planned_UI_original_PUTs: 4, planned_UI_fresh_PUTs: 2, planned_versionless_API_PUTs: 1 });
  return { recovery, children };
}
type OrdinaryUIFixture = Awaited<ReturnType<typeof seedRecoveryOrdinaryUI>>;

interface ChildRead {
  sequence: number; at: string; path: string; status: number; issue: OrdinaryIssue;
  response_bytes: number; response_sha256: string; authentication: Authentication;
}
interface Authentication { cookie: boolean; csrf: boolean; bearer: boolean }
interface ChildWrite {
  sequence: number; at: string; path: string; method: string; body: Record<string, unknown>; body_sha256: string;
  authentication: Authentication | null; status: number | null; response_body: unknown;
}
const authentication = (headers: Record<string, string>): Authentication => ({
  cookie: Boolean(headers.cookie), csrf: Boolean(headers["x-csrf-token"]), bearer: Boolean(headers.authorization),
});

// Observe the actual app's GET and PUT traffic. No route changes, synthetic
// requests, cache edits or token injection are used to obtain a UI observation.
class ChildTraffic {
  readonly reads: ChildRead[] = [];
  readonly writes: ChildWrite[] = [];
  readonly errors: { type: string; path: string; method: string }[] = [];
  private requests = new Map<Request, ChildWrite>();
  private track(pending: Promise<void>) {
    this.session.pendingReads.add(pending);
    void pending.finally(() => this.session.pendingReads.delete(pending));
  }
  private request = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (request.method() !== "PUT" || path !== `/api/issues/${this.child.id}`) return;
    try {
      const body = z.record(z.string(), z.unknown()).parse(request.postDataJSON());
      const fact: ChildWrite = { sequence: this.writes.length + 1, at: now(), path, method: "PUT", body,
        body_sha256: digest(body), authentication: null, status: null, response_body: null };
      this.requests.set(request, fact); this.writes.push(fact);
      this.track(request.allHeaders().then((headers) => { fact.authentication = authentication(headers); })
        .catch((error: unknown) => { this.errors.push({ type: error instanceof Error ? error.name : "request_headers_error", path, method: "PUT" }); }));
    } catch (error) {
      this.errors.push({ type: error instanceof Error ? error.name : "request_body_error", path, method: "PUT" });
    }
  };
  private response = (response: Response) => {
    const request = response.request(), path = new URL(response.url()).pathname;
    const childRead = request.method() === "GET" && [`/api/issues/${this.child.id}`, `/api/issues/${this.child.identifier}`].includes(path);
    const write = this.requests.get(request);
    if (!childRead && !write) return;
    this.track((async () => {
      try {
        const bytes = await response.body(), body: unknown = JSON.parse(bytes.toString());
        if (write) { write.status = response.status(); write.response_body = body; }
        if (childRead) {
          const issue = ordinaryIssueSchema.parse(body);
          expect(issue.id).toBe(this.child.id); expect(issue.workspace_id).toBe(this.child.workspace_id);
          const headers = await request.allHeaders();
          this.reads.push({ sequence: this.reads.length + 1, at: now(), path, status: response.status(), issue,
            response_bytes: bytes.length, response_sha256: createHash("sha256").update(bytes).digest("hex"),
            authentication: authentication(headers) });
        }
      } catch (error) {
        this.errors.push({ type: error instanceof Error ? error.name : "response_read_error", path, method: request.method() });
      }
    })());
  };
  constructor(readonly session: RecoverySession, readonly child: OrdinaryIssue, readonly role: "priority" | "assignee") {
    session.page.on("request", this.request); session.page.on("response", this.response);
  }
  async observed(revision: number, afterSequence = 0) {
    const find = () => this.reads.find((fact) => fact.sequence > afterSequence && fact.status === 200 && fact.issue.revision === revision);
    await expect.poll(() => find() !== undefined, { message: `The ${this.role} UI must receive an actual child GET at numeric revision ${revision}` }).toBe(true);
    const fact = find()!;
    expect(this.errors).toEqual([]); assertAuthentication(this.session.platform, fact.authentication, false);
    return fact;
  }
  async settle() { await Promise.allSettled([...this.session.pendingReads]); }
  save(evidence: RecoveryEvidence, stage: string) {
    evidence.checkpoint(stage, { role: this.role, actor_id: this.session.actor.id, platform: this.session.platform,
      actual_GETs: this.reads, actual_PUTs: this.writes, observation_errors: this.errors });
  }
}

function assertAuthentication(platform: RecoveryPlatform, value: Authentication, mutation: boolean) {
  if (platform === "Desktop") expect(value.bearer).toBe(true);
  else { expect(value.cookie).toBe(true); if (mutation) expect(value.csrf).toBe(true); expect(value.bearer).toBe(false); }
}

function propertyRow(session: RecoverySession, field: "priority" | "assignee"): Locator {
  const label = session.page.getByText(field === "priority" ? "Priority" : "Assignee", { exact: true });
  return label.locator("..");
}
function propertyButton(session: RecoverySession, field: "priority" | "assignee", name: string): Locator {
  const row = propertyRow(session, field);
  // An assigned trigger includes avatar initials/name and the member-name
  // span. Match that exact rendered label within its owning property button.
  return field === "assignee" ? row.getByRole("button").filter({ has: session.page.getByText(name, { exact: true }) })
    : row.getByRole("button", { name, exact: true });
}
async function recordPropertyRow(session: RecoverySession, field: "priority" | "assignee", expectedLabel: string, evidence: RecoveryEvidence) {
  const row = propertyRow(session, field);
  const metadata = await row.evaluateAll((elements) => ({ total_rows: elements.length, rows: elements.slice(0, 8).map((element) => ({
    text: element.textContent?.slice(0, 512) ?? "",
    native_button_count: element.querySelectorAll("button").length,
    buttons: [...element.querySelectorAll<HTMLButtonElement>("button")].slice(0, 8).map((button) => ({
      text: button.textContent?.slice(0, 256) ?? "", aria_label: button.getAttribute("aria-label"),
      aria_labelledby: button.getAttribute("aria-labelledby"), aria_describedby: button.getAttribute("aria-describedby"),
      role: button.getAttribute("role"), slot: button.getAttribute("data-slot"), disabled: button.disabled,
      rendered_spans: [...button.querySelectorAll("span")].slice(0, 16).map((span) => span.textContent?.slice(0, 128) ?? ""),
    })),
  })) }));
  const accessibility = metadata.total_rows === 1 ? await row.ariaSnapshot() : null;
  evidence.checkpoint("actual_property_row_before_field_selector_assertion", { field, expected_rendered_label: expectedLabel,
    actor_id: session.actor.id, platform: session.platform, native_pid: session.lifecycle.native_pid, metadata, accessibility });
}
async function displayedFields(session: RecoverySession, priority: "low" | "high", assignee: "member" | "unassigned", evidence: RecoveryEvidence) {
  const priorityLabel = priority === "low" ? "Low" : "High", assigneeLabel = assignee === "member" ? session.actor.name : "Unassigned";
  await recordPropertyRow(session, "priority", priorityLabel, evidence); await recordPropertyRow(session, "assignee", assigneeLabel, evidence);
  const priorityButton = propertyButton(session, "priority", priorityLabel), assigneeButton = propertyButton(session, "assignee", assigneeLabel);
  await expect(priorityButton).toHaveCount(1); await expect(assigneeButton).toHaveCount(1);
  await expect(priorityButton).toBeVisible(); await expect(assigneeButton).toBeVisible();
  return { priority: await priorityButton.innerText(), assignee: await assigneeButton.innerText() };
}
async function pickerChoice(session: RecoverySession, field: "priority" | "assignee", evidence: RecoveryEvidence) {
  await recordPropertyRow(session, field, field === "priority" ? "Low" : session.actor.name, evidence);
  const trigger = propertyButton(session, field, field === "priority" ? "Low" : session.actor.name);
  await expect(trigger).toHaveCount(1);
  await expect(trigger).toBeVisible(); await expect(trigger).toBeEnabled(); await trigger.click();
  const list = session.page.getByRole("listbox", { name: field === "priority" ? "Priority" : "Assignee", exact: true });
  const popup = session.page.locator('[data-slot="popover-content"]').filter({ has: list });
  await expect(popup).toHaveCount(1); await expect(popup).toBeVisible();
  const choice = list.getByRole("option", { name: field === "priority" ? "High" : "Unassigned", exact: true });
  await expect(choice).toBeVisible(); await expect(choice).toBeEnabled(); await choice.click();
  await expect(popup).toBeHidden();
}

const auditQueries = {
  parent_issue: "SELECT i.id,encode(sha256(convert_to(to_jsonb(i)::text,'UTF8')),'hex') AS state_sha256 FROM issue i WHERE i.workspace_id=$1 AND i.id=$2",
  collaboration: "SELECT c.issue_id,encode(sha256(convert_to(to_jsonb(c)::text,'UTF8')),'hex') AS state_sha256 FROM issue_collaboration c JOIN issue i ON i.id=c.issue_id WHERE i.workspace_id=$1 AND c.issue_id=$2",
  sessions: "SELECT s.id,s.issue_id,encode(sha256(convert_to(to_jsonb(s)::text,'UTF8')),'hex') AS state_sha256 FROM collaboration_session s JOIN issue i ON i.id=s.issue_id WHERE i.workspace_id=$1 AND s.issue_id=$2 ORDER BY s.id",
  events: "SELECT e.id,e.issue_id,e.event_seq::text,e.kind,e.actor_user_id,e.created_at::text,encode(sha256(convert_to(e.payload::text,'UTF8')),'hex') AS payload_sha256 FROM collaboration_event e JOIN issue i ON i.id=e.issue_id WHERE i.workspace_id=$1 AND e.issue_id=$2 ORDER BY e.event_seq,e.id LIMIT 129",
  operations: "SELECT o.id,o.scope_id,o.actor_user_id,o.request_id,o.kind,o.payload_hash,o.result_code,o.completed_at::text,encode(sha256(convert_to(o.result::text,'UTF8')),'hex') AS result_sha256 FROM collaboration_operation o JOIN issue i ON i.id=o.scope_id WHERE i.workspace_id=$1 AND o.scope_kind='issue' AND o.scope_id=$2 ORDER BY o.id LIMIT 129",
  handoffs: "SELECT h.id,h.issue_id,encode(sha256(convert_to(to_jsonb(h)::text,'UTF8')),'hex') AS state_sha256 FROM issue_handoff h JOIN issue i ON i.id=h.issue_id WHERE i.workspace_id=$1 AND h.issue_id=$2 ORDER BY h.id LIMIT 129",
  commands: "SELECT c.id,s.issue_id FROM collaboration_command c JOIN collaboration_session s ON s.id=c.session_id JOIN issue i ON i.id=s.issue_id WHERE i.workspace_id=$1 AND s.issue_id=$2 ORDER BY c.id LIMIT 129",
  groups: "SELECT g.id,s.issue_id FROM collaboration_execution_group g JOIN collaboration_session s ON s.id=g.session_id JOIN issue i ON i.id=s.issue_id WHERE i.workspace_id=$1 AND s.issue_id=$2 ORDER BY g.id LIMIT 129",
  runs: "SELECT r.id,s.issue_id FROM collaboration_run r JOIN collaboration_session s ON s.id=r.session_id JOIN issue i ON i.id=s.issue_id WHERE i.workspace_id=$1 AND s.issue_id=$2 ORDER BY r.id LIMIT 129",
  dispatches: "SELECT q.id,q.issue_id FROM agent_task_queue q JOIN issue i ON i.id=q.issue_id WHERE i.workspace_id=$1 AND q.issue_id=ANY($2::uuid[]) ORDER BY q.id LIMIT 129",
};

async function protectedAudit(fixture: OrdinaryUIFixture, evidence: RecoveryEvidence, stage: string) {
  const database = new pg.Client({ connectionString: process.env.DATABASE_URL!, connectionTimeoutMillis: 5_000 });
  const rows: Record<string, Record<string, unknown>[]> = {};
  let primary: { error: unknown } | undefined, cleanupFailure: { error: unknown } | undefined;
  try {
    await database.connect();
    await database.query("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY");
    await database.query("SET LOCAL statement_timeout='5s'");
    await database.query("SET LOCAL idle_in_transaction_session_timeout='10s'");
    await database.query("SET LOCAL TIME ZONE 'UTC'");
    const identity = await database.query<{ database: string; read_only: string }>(
      "SELECT current_database() AS database,current_setting('transaction_read_only') AS read_only");
    expect(identity.rows).toEqual([{ database: process.env.TASK_DATABASE_NAME, read_only: "on" }]);
    for (const [name, sql] of Object.entries(auditQueries)) {
      const ids = name === "dispatches" ? [fixture.recovery.issueId, ...Object.values(fixture.children).map((child) => child.id)] : fixture.recovery.issueId;
      const bindings = [fixture.recovery.workspace.id, ids];
      rows[name] = (await database.query<Record<string, unknown>>(sql, bindings)).rows;
      evidence.checkpoint(`${stage}_SQL_${name}`, { sql, bindings, rows: rows[name], rows_sha256: digest(rows[name]), row_limit: 128 });
      expect(rows[name]!.length, `The full ${name} inventory must fit its stated bound`).toBeLessThanOrEqual(128);
    }
    await database.query("COMMIT");
    for (const name of ["parent_issue", "collaboration", "sessions"]) expect(rows[name]).toHaveLength(1);
    for (const name of ["commands", "groups", "runs", "dispatches"]) expect(rows[name]).toEqual([]);
    evidence.checkpoint(stage, { database: process.env.TASK_DATABASE_NAME, rows_sha256: digest(rows),
      counts: Object.fromEntries(Object.entries(rows).map(([name, value]) => [name, value.length])), no_new_run_command_dispatch: true });
  } catch (error) {
    primary = { error };
    evidence.checkpoint(`${stage}_audit_failed`, { error_type: error instanceof Error ? error.name : "unknown_error" });
    const rollback = await Promise.allSettled([database.query("ROLLBACK")]);
    evidence.checkpoint(`${stage}_rollback_outcome`, { outcomes: rollback.map((result) => result.status) });
  } finally {
    try { await database.end(); evidence.checkpoint(`${stage}_audit_connection_closed`); }
    catch (error) {
      cleanupFailure = { error };
      evidence.checkpoint(`${stage}_audit_close_failed`, { error_type: error instanceof Error ? error.name : "unknown_error", primary_failure_preserved: primary !== undefined });
    }
  }
  if (primary) throw primary.error;
  if (cleanupFailure) throw cleanupFailure.error;
  return rows;
}

async function genuineUIOrder(browser: Browser, platform: RecoveryPlatform, info: TestInfo, evidence: RecoveryEvidence,
  fixture: OrdinaryUIFixture, child: OrdinaryIssue, first: "priority" | "assignee") {
  return withSessions(browser, platform, fixture.recovery, info, evidence, async (open) => {
    const prioritySession = await open(fixture.recovery.creator), assigneeSession = await open(fixture.recovery.creator);
    const priorityTraffic = new ChildTraffic(prioritySession, child, "priority"), assigneeTraffic = new ChildTraffic(assigneeSession, child, "assignee");
    const path = `/api/issues/${child.id}`, revision = child.revision;
    const priority = new HeldRequest(prioritySession, path, "PUT", "GENUINE_PRIORITY_UI_OBSERVATION");
    const assignee = new HeldRequest(assigneeSession, path, "PUT", "GENUINE_ASSIGNEE_UI_OBSERVATION");
    const holds = [priority, assignee], choices: Promise<void>[] = [];
    let primary: { error: unknown } | undefined, releaseFailed = false;
    let result: Record<string, unknown> | undefined;
    try {
      await openOrdinaryChild(prioritySession, child); await openOrdinaryChild(assigneeSession, child);
      const baselineReads = await Promise.all([priorityTraffic.observed(revision), assigneeTraffic.observed(revision)]);
      for (const read of baselineReads) expect(read.issue).toMatchObject({ priority: "low", assignee_type: "member", assignee_id: fixture.recovery.creator.id });
      const displayed = await Promise.all([displayedFields(prioritySession, "low", "member", evidence), displayedFields(assigneeSession, "low", "member", evidence)]);
      evidence.checkpoint("both_actual_child_GETs_and_displayed_baseline", { revision, baselineReads, displayed, first,
        separate_authenticated_UI_sessions: true, same_valid_source_member: fixture.recovery.creator.id });
      await priority.install(); await assignee.install();
      evidence.checkpoint("two_holds_installed_before_either_genuine_picker_choice", { path });
      choices.push(pickerChoice(prioritySession, "priority", evidence), pickerChoice(assigneeSession, "assignee", evidence));
      for (const choice of choices) void choice.catch(() => undefined);
      await Promise.all(choices);
      const [priorityInput, assigneeInput] = await Promise.all([priority.captured(), assignee.captured()]);
      evidence.checkpoint("both_genuine_UI_PUTs_captured_before_forward", { revision, priorityInput, assigneeInput });
      expect(priorityInput.body).toEqual({ priority: "high", expected_revision: revision });
      expect(assigneeInput.body).toEqual({ assignee_type: null, assignee_id: null, expected_revision: revision });
      for (const input of [priorityInput, assigneeInput]) {
        expect(input.path).toBe(path); expect(input.method).toBe("PUT"); assertAuthentication(platform, input.authentication, true);
      }
      const winner = first === "priority" ? priority : assignee, loser = first === "priority" ? assignee : priority;
      const loserSession = first === "priority" ? assigneeSession : prioritySession;
      const loserTraffic = first === "priority" ? assigneeTraffic : priorityTraffic;
      const loserBaseline = first === "priority" ? baselineReads[1]! : baselineReads[0]!;
      const won = await winner.forward();
      evidence.checkpoint("named_winner_actual_response", { first, won });
      expect(won.status).toBe(200);
      const winnerIssue = ordinaryIssueSchema.parse(won.body);
      const winnerFields = { revision: revision + 1, priority: first === "priority" ? "high" : "low",
        assignee_type: first === "priority" ? "member" : null, assignee_id: first === "priority" ? fixture.recovery.creator.id : null,
        parent_issue_id: fixture.recovery.issueId };
      expect(winnerIssue).toMatchObject(winnerFields);
      const lost = await loser.forward();
      evidence.checkpoint("original_loser_actual_response", { first, lost });
      expect(lost.status).toBe(409);
      expect(lost.body).toEqual({ error: "resource changed since it was loaded", code: "revision_conflict", resource_type: "issue",
        resource_id: child.id, expected_revision: revision, actual_revision: revision + 1 });
      const toast = loserSession.page.getByText(conflictMessage, { exact: true });
      await expect(toast).toBeVisible();
      evidence.checkpoint("existing_visible_revision_conflict_feedback", { text: await toast.innerText(), original_expected_revision: revision });
      const refreshed = await loserTraffic.observed(revision + 1, loserBaseline.sequence);
      expect(refreshed.issue).toMatchObject(winnerFields);
      const afterWinner = ordinaryIssueSchema.parse(await fixture.recovery.creator.client.requestJSON(path));
      expect(afterWinner).toMatchObject(winnerFields);
      const refreshedDisplay = await displayedFields(loserSession, first === "priority" ? "high" : "low", first === "priority" ? "member" : "unassigned", evidence);
      evidence.checkpoint("loser_actual_authoritative_GET_refresh_preserves_winner_and_losing_field", { refreshed, afterWinner, refreshedDisplay });
      await priority.close(); await assignee.close();
      await priorityTraffic.settle(); await assigneeTraffic.settle();
      expect(priorityTraffic.writes).toHaveLength(1); expect(assigneeTraffic.writes).toHaveLength(1);
      const fresh = new HeldRequest(loserSession, path, "PUT", "GENUINE_EXPLICIT_FRESH_UI_OBSERVATION");
      holds.push(fresh); await fresh.install();
      evidence.checkpoint("fresh_hold_after_observed_R_plus_1_before_explicit_picker_reopen", { refreshed_revision: refreshed.issue.revision });
      await pickerChoice(loserSession, first === "priority" ? "assignee" : "priority", evidence);
      const freshInput = await fresh.captured();
      const freshBody = first === "priority" ? { assignee_type: null, assignee_id: null, expected_revision: revision + 1 }
        : { priority: "high", expected_revision: revision + 1 };
      evidence.checkpoint("new_genuine_UI_intent_after_explicit_picker_reopen", { freshInput, original_loser: loser.fact });
      expect(freshInput.body).toEqual(freshBody); assertAuthentication(platform, freshInput.authentication, true);
      expect(freshInput.body_sha256).not.toBe(loser.fact!.body_sha256);
      const freshResult = await fresh.forward();
      evidence.checkpoint("fresh_actual_response", { freshInput, freshResult });
      expect(freshResult.status).toBe(200);
      const finalFields = { id: child.id, revision: revision + 2, priority: "high", assignee_type: null,
        assignee_id: null, parent_issue_id: fixture.recovery.issueId };
      expect(ordinaryIssueSchema.parse(freshResult.body)).toMatchObject(finalFields);
      const final = ordinaryIssueSchema.parse(await fixture.recovery.creator.client.requestJSON(path));
      expect(final).toMatchObject(finalFields);
      const converged = await Promise.all([displayedFields(prioritySession, "high", "unassigned", evidence), displayedFields(assigneeSession, "high", "unassigned", evidence)]);
      await priorityTraffic.settle(); await assigneeTraffic.settle();
      expect(priorityTraffic.errors).toEqual([]); expect(assigneeTraffic.errors).toEqual([]);
      expect(priorityTraffic.writes).toHaveLength(first === "priority" ? 1 : 2);
      expect(assigneeTraffic.writes).toHaveLength(first === "priority" ? 2 : 1);
      const expectedStatuses = first === "priority" ? [[200], [409, 200]] : [[409, 200], [200]];
      expect([priorityTraffic.writes.map((write) => write.status), assigneeTraffic.writes.map((write) => write.status)]).toEqual(expectedStatuses);
      expect(prioritySession.writes.filter((write) => write.path !== path || write.method !== "PUT")).toEqual([]);
      expect(assigneeSession.writes.filter((write) => write.path !== path || write.method !== "PUT")).toEqual([]);
      evidence.checkpoint("both_actual_UI_sessions_converged_after_only_one_explicit_fresh_PUT", { final, converged,
        genuine_original_PUTs: 2, genuine_fresh_PUTs: 1, automatic_resends: 0, route_continue_only: true, UI_revision_injection: false });
      result = { child_id: child.id, first, initial_revision: revision, final_revision: final.revision,
        genuine_UI_original_PUTs: 2, genuine_UI_fresh_PUTs: 1, winner: won, original_conflict: lost, fresh: freshResult };
    } catch (error) { primary = { error }; }
    finally {
      const releases = await Promise.allSettled(holds.map((hold) => hold.close()));
      releaseFailed = releases.some((entry) => entry.status === "rejected");
      const clicks = await Promise.allSettled(choices);
      await priorityTraffic.settle(); await assigneeTraffic.settle();
      priorityTraffic.save(evidence, "retained_priority_UI_traffic_after_hold_collection");
      assigneeTraffic.save(evidence, "retained_assignee_UI_traffic_after_hold_collection");
      evidence.checkpoint("all_original_and_fresh_holds_released_and_collected", {
        holds: holds.map((hold) => ({ original_input: hold.fact, actual_result: hold.result })),
        releases: releases.map((result) => result.status), clicks: clicks.map((result) => result.status), primary_failure_preserved: primary !== undefined,
      });
    }
    if (primary) throw primary.error;
    if (releaseFailed) throw new Error("An owned ordinary UI hold failed to close");
    if (result === undefined) throw new Error("The ordinary UI ordering has no completed result");
    return result;
  });
}

async function versionlessAPI(fixture: OrdinaryUIFixture, child: OrdinaryIssue, evidence: RecoveryEvidence) {
  const token = fixture.recovery.creator.client.getToken();
  if (!token) throw new Error("The direct compatibility control requires the valid fixture member credential");
  const body = { priority: "high" };
  expect(Object.hasOwn(body, "expected_revision")).toBe(false);
  // This direct API request is deliberately separate from genuine picker proof.
  const response = await fetch(`${fixture.recovery.apiUrl}/api/issues/${child.id}`, { method: "PUT", headers: {
    "Content-Type": "application/json", Authorization: `Bearer ${token}`, "X-Workspace-ID": fixture.recovery.workspace.id,
  }, body: JSON.stringify(body) });
  const result: unknown = await response.json();
  evidence.checkpoint("separate_direct_versionless_API_compatibility_control", { path: `/api/issues/${child.id}`, method: "PUT",
    body, body_sha256: digest(body), actor_id: fixture.recovery.creator.id, authentication: { bearer: true }, status: response.status, result,
    scope: "Direct API omission compatibility only; zero UI CAS requests", genuine_UI_PUTs: 0 });
  expect(response.status).toBe(200);
  const current = ordinaryIssueSchema.parse(result);
  expect(current).toMatchObject({ id: child.id, revision: child.revision + 1, priority: "high", assignee_type: "member",
    assignee_id: fixture.recovery.creator.id, parent_issue_id: fixture.recovery.issueId });
  // UpdateIssue deliberately omits labels; detail GET supplies their authoritative list.
  expect(result).not.toHaveProperty("labels");
  const authoritative = ordinaryIssueSchema.parse(await fixture.recovery.creator.client.requestJSON(`/api/issues/${child.id}`));
  evidence.checkpoint("versionless_API_authoritative_detail", { child_id: child.id, authoritative });
  expect(authoritative.labels).toEqual([]);
  expect(authoritative).toEqual({ ...current, labels: [] });
  return { child_id: child.id, status: response.status, initial_revision: child.revision, final_revision: current.revision,
    direct_versionless_API_PUTs: 1, genuine_UI_PUTs: 0 };
}

export async function exerciseRecoveryOrdinaryUI(browser: Browser, platform: RecoveryPlatform, info: TestInfo,
  evidence: RecoveryEvidence, fixture: OrdinaryUIFixture, subcase: Subcase) {
  const child = fixture.children[subcase], parentPath = `/api/collaboration/issues/${fixture.recovery.issueId}`;
  // Native sessions log in through the shared client. Keep the comparator's
  // original credential so its authority-derived context version is comparable.
  const parentReaderToken = fixture.recovery.creator.client.getToken();
  if (!parentReaderToken) throw new Error("The parent comparison requires its original fixture member credential");
  const parentReaderTokenHash = createHash("sha256").update(parentReaderToken).digest("hex");
  const readParent = async (stage: "before" | "after") => {
    const response = await fetch(`${fixture.recovery.apiUrl}${parentPath}`, { headers: {
      "Content-Type": "application/json", Authorization: `Bearer ${parentReaderToken}`,
      "X-Workspace-ID": fixture.recovery.workspace.id,
    } });
    evidence.checkpoint("parent_comparison_original_credential_read", { stage, path: parentPath,
      actor_id: fixture.recovery.creator.id, credential_sha256: parentReaderTokenHash, status: response.status });
    expect(response.status).toBe(200);
    return CollaborationIssueSchema.parse(await response.json());
  };
  const parentBefore = await readParent("before");
  const before = await protectedAudit(fixture, evidence, "before_ordinary_field_subcase");
  let primary: { error: unknown } | undefined, retentionFailure: { error: unknown } | undefined;
  let result: Awaited<ReturnType<typeof genuineUIOrder>> | Awaited<ReturnType<typeof versionlessAPI>> | undefined;
  let afterAudit: Awaited<ReturnType<typeof protectedAudit>> | undefined, parentAfter: typeof parentBefore | undefined;
  try {
    evidence.set({ active_phase: subcase, original_numeric_revision: child.revision, child_id: child.id });
    result = subcase === "versionlessAPI" ? await versionlessAPI(fixture, child, evidence)
      : await genuineUIOrder(browser, platform, info, evidence, fixture, child, subcase === "priority-first" ? "priority" : "assignee");
  } catch (error) { primary = { error }; }
  finally {
    const retained = await Promise.allSettled([
      protectedAudit(fixture, evidence, "after_ordinary_field_subcase"),
      readParent("after"),
    ]);
    evidence.checkpoint("retained_parent_and_execution_after_subcase", { outcomes: retained.map((result) => result.status),
      parent_before: parentBefore, parent_after: retained[1]!.status === "fulfilled" ? retained[1]!.value : null,
      primary_failure_preserved: primary !== undefined });
    if (retained[0]!.status === "fulfilled") afterAudit = retained[0]!.value;
    else retentionFailure = { error: retained[0]!.reason };
    if (retained[1]!.status === "fulfilled") parentAfter = retained[1]!.value;
    else retentionFailure ??= { error: retained[1]!.reason };
  }
  if (primary) throw primary.error;
  if (retentionFailure) throw retentionFailure.error;
  expect(afterAudit).toEqual(before); expect(parentAfter).toEqual(parentBefore);
  evidence.set({ parent_collaboration_history_session_execution_unchanged: true, new_runs: 0, new_commands: 0, new_dispatches: 0 });
  if (result === undefined) throw new Error("The ordinary UI subcase has no completed result");
  return result;
}
