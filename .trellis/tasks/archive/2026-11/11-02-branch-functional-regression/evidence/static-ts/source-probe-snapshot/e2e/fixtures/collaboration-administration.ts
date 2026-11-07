import { randomUUID } from "node:crypto";
import pg from "pg";
import { expect, type Locator, type Page, type Request } from "@playwright/test";
import { TestApiClient } from "../fixtures";

type Action = "complete" | "cancel" | "reopen";
interface AccountSummary { user_id: string; name: string; avatar_url: string; disabled: boolean }
interface AdministrationIssue {
  issue_id: string; identifier: string; source_workspace_id: string; source_name: string;
  status: string; status_category: string; collaboration_revision: string;
  creator: AccountSummary; handler: AccountSummary; queue_state: string; unsettled_runs: number;
}
interface AdministrationDetail extends AdministrationIssue {
  scope: string; successor_required: boolean;
  actions: { action: Action; default_status: string; status_options_version: string; options: { key: string; name: string }[] }[];
}
interface LifecycleBody {
  action: Action; expected_collaboration_revision: string; expected_status: string;
  target_status: string; status_options_version: string; reason: string; successor_user_id?: string;
}
interface LifecycleReceipt {
  operation: { id: string; actor_id: string; organization_id: string; target_kind: string; target_id: string;
    kind: string; state: string; result_code: string; confirmation: string };
  result: { issue_id: string; request_id: string; action: Action; code: string; collaboration_revision: string;
    creator_user_id: string; previous_handler_user_id: string; handler_user_id: string;
    previous_status: string; status: string; queue_state: string; control_version: string; unsettled_runs: number };
}
interface CreatedIssue {
  id: string; identifier: string; collaboration_revision: string; lifecycle_epoch: string;
  write_context: { participant_epoch: string; account_epoch: string };
}

export function administrationEnvironment() {
  const { DATABASE_URL: databaseUrl, COLLABORATION_E2E_API_URL: apiUrl, NEXT_PUBLIC_API_URL: browserApi } = process.env;
  if (process.env.E2E_PLATFORM_ADMIN !== "1" || process.env.E2E_PASSWORD_AUTH !== "1" || !databaseUrl || !apiUrl || !browserApi
    || !process.env.E2E_PLATFORM_ADMIN_USERNAME || !process.env.E2E_PLATFORM_ADMIN_PASSWORD) {
    throw new Error("Requires the isolated password/admin deployment and injected synthetic administrator");
  }
  for (const value of [databaseUrl, apiUrl, browserApi]) {
    if (!["localhost", "127.0.0.1"].includes(new URL(value).hostname)) throw new Error("Administration acceptance requires loopback test services");
  }
  if (["", "multica", "postgres", "template0", "template1"].includes(new URL(databaseUrl).pathname.slice(1))) {
    throw new Error("Administration acceptance refuses a default database");
  }
  if (new URL(apiUrl).origin !== new URL(browserApi).origin) throw new Error("Fixture and browser API origins differ");
  return { apiUrl, databaseUrl };
}

/** Password APIs create only synthetic actors. Creation, invitation and all later
 * account/lifecycle writes use the real API. The coordinator retains these rows
 * in the isolated database for inspection; no provider or runtime is created. */
export async function seedCollaborationAdministration() {
  const environment = administrationEnvironment();
  const configuration = await fetch(`${environment.apiUrl}/api/config`);
  expect(configuration.status).toBe(200);
  expect(await configuration.json()).toMatchObject({ auth_mode: "password" });
  const administrator = new TestApiClient();
  const adminAccount = await administrator.loginPassword(process.env.E2E_PLATFORM_ADMIN_USERNAME!, process.env.E2E_PLATFORM_ADMIN_PASSWORD!);
  const identity = await administrator.requestJSON<{ user_id: string; organization_id: string; role: string; supported: boolean }>("/api/admin/me");
  expect(identity).toMatchObject({ user_id: adminAccount.id, role: "super_admin", supported: true });
  expect(await administrator.getWorkspaces()).toEqual([]);

  const suffix = randomUUID().replaceAll("-", "").slice(0, 16);
  const creator = new TestApiClient();
  const creatorAccount = await creator.registerPassword(`s5p${suffix}`, `S5-${randomUUID()}!`, "S5 original creator P");
  const handler = new TestApiClient();
  const handlerAccount = await handler.registerPassword(`s5h${suffix}`, `S5-${randomUUID()}!`, "S5 original handler H");
  const successor = new TestApiClient();
  const successorName = "S5 eligible participant C";
  const successorAccount = await successor.registerPassword(`s5c${suffix}`, `S5-${randomUUID()}!`, successorName);
  expect(await creator.getWorkspaces()).toEqual([]);
  const sourceWorkspace = await creator.ensureWorkspace("S5 synthetic source", `s5-source-${suffix}`);
  expect(await handler.getWorkspaces()).toEqual([]);
  expect(await successor.getWorkspaces()).toEqual([]);
  const options = await creator.requestJSON<{ creation_enabled: boolean }>("/api/collaboration/creation-options");
  if (options.creation_enabled !== true) throw new Error("The coordinator must enable creation in this isolated acceptance database");
  const creationKey = randomUUID();
  const privateTitle = `Private S5 collaboration ${suffix}`;
  const privateBody = `Synthetic private content ${randomUUID()}`;
  const created = await creator.requestJSON<CreatedIssue>("/api/collaboration/issues", {
    method: "POST", headers: { "Idempotency-Key": creationKey }, body: {
      source_workspace_id: sourceWorkspace.id, handler_user_id: handlerAccount.id,
      title: privateTitle, description: privateBody, attachment_ids: [],
    },
  });
  const invitationKey = randomUUID();
  await creator.requestJSON(`/api/collaboration/issues/${created.id}/participants`, {
    method: "POST", headers: { "Idempotency-Key": invitationKey }, body: {
      target_user_id: successorAccount.id, expected_collaboration_revision: created.collaboration_revision,
      expected_lifecycle_epoch: created.lifecycle_epoch, expected_participant_epoch: created.write_context.participant_epoch,
      expected_account_epoch: created.write_context.account_epoch,
    },
  });

  const fixture = { ...environment, administrator, successor, administratorId: adminAccount.id, organizationId: identity.organization_id,
    creatorId: creatorAccount.id, handlerId: handlerAccount.id, successorId: successorAccount.id, successorName,
    sourceWorkspace, issueId: created.id, creationKey, invitationKey, privateTitle, privateBody };
  return fixture;
}
export type AdministrationFixture = Awaited<ReturnType<typeof seedCollaborationAdministration>>;
export const administrationWebPath = (fixture: AdministrationFixture) => `/admin/collaboration?issue=${fixture.issueId}`;
export const administrationAccountPath = (fixture: AdministrationFixture) => `/account?view=current&administration=collaboration&issue=${fixture.issueId}`;
const adminIssuePath = (fixture: AdministrationFixture) => `/api/admin/collaboration/issues/${fixture.issueId}`;
const creatorRestoredMessage = "The creator's account is enabled again. New administration changes are unavailable; you can still check an earlier request.";

export async function loginAdministrationWeb(page: Page, destination: string) {
  await page.goto(destination);
  await expect.poll(() => new URL(page.url()).pathname).toBe("/login");
  await expect(page.locator("#password-username")).toBeVisible();
  await expect(page.locator("#password-value")).toBeVisible();
  // Do not retain a trace or expose a fill failure's credential-bearing call log.
  try {
    await page.locator("#password-username").fill(process.env.E2E_PLATFORM_ADMIN_USERNAME!);
    await page.locator("#password-value").fill(process.env.E2E_PLATFORM_ADMIN_PASSWORD!);
  } catch { throw new Error("The synthetic administrator password form could not be filled"); }
  const login = page.waitForResponse(response => new URL(response.url()).pathname === "/auth/login" && response.request().method() === "POST");
  await page.locator('button[type="submit"]').click();
  expect((await login).status()).toBe(200);
  await expect(page).toHaveURL(destination);
  expect((await page.context().cookies()).some(cookie => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
  expect(await page.evaluate(() => localStorage.getItem("multica_token"))).toBeNull();
}

async function accountAction(fixture: AdministrationFixture, userId: string, action: "disable" | "restore") {
  if (![fixture.creatorId, fixture.handlerId].includes(userId)) throw new Error("Only this journey's synthetic accounts can be changed");
  const current = await fixture.administrator.requestJSON<{ user: { auth_version: number }; scope: string }>(`/api/admin/users/${userId}`);
  expect(current.scope).toBe(fixture.organizationId);
  expect(Number.isSafeInteger(current.user.auth_version) && current.user.auth_version > 0).toBe(true);
  const key = randomUUID();
  const operation = await fixture.administrator.requestJSON<{ id: string; actor_id: string; target_id: string; kind: string; state: string }>(`/api/admin/users/${userId}/${action}`, {
    method: "POST", headers: { "Idempotency-Key": key }, body: {
      expected_auth_version: current.user.auth_version, password: process.env.E2E_PLATFORM_ADMIN_PASSWORD,
      reason: `S5 synthetic collaboration acceptance: ${action} original ${userId === fixture.creatorId ? "creator" : "handler"}`,
    },
  });
  expect(operation).toMatchObject({ actor_id: fixture.administratorId, target_id: userId, kind: `user.${action}`, state: "applied" });
  return { key, operationId: operation.id, userId, action };
}

function assertMinimalMetadata(fixture: AdministrationFixture, issue: AdministrationIssue, detail = false) {
  expect(Object.keys(issue).sort()).toEqual([
    "issue_id", "identifier", "source_workspace_id", "source_name", "status", "status_category", "collaboration_revision",
    "creator", "handler", "queue_state", "unsettled_runs", ...(detail ? ["scope", "actions", "successor_required"] : []),
  ].sort());
  for (const account of [issue.creator, issue.handler]) expect(Object.keys(account).sort()).toEqual(["avatar_url", "disabled", "name", "user_id"]);
  expect(issue).toMatchObject({ issue_id: fixture.issueId, source_workspace_id: fixture.sourceWorkspace.id,
    creator: { user_id: fixture.creatorId, disabled: true }, unsettled_runs: 0 });
  expect(JSON.stringify(issue).includes(fixture.privateTitle) || JSON.stringify(issue).includes(fixture.privateBody)).toBe(false);
}

async function detail(fixture: AdministrationFixture) {
  const current = await fixture.administrator.requestJSON<AdministrationDetail>(adminIssuePath(fixture));
  assertMinimalMetadata(fixture, current, true);
  expect(current.scope).toBe(fixture.organizationId);
  return current;
}

async function assertNoOrdinaryAccess(fixture: AdministrationFixture) {
  expect(await fixture.administrator.getWorkspaces()).toEqual([]);
  const results: { path: string; status: number }[] = [];
  for (const path of [`/api/collaboration/issues/${fixture.issueId}`, `/api/collaboration/issues/${fixture.issueId}/comments`, `/api/issues/${fixture.issueId}`]) {
    const response = await fetch(`${fixture.apiUrl}${path}`, { headers: {
      Authorization: `Bearer ${fixture.administrator.getToken()}`, ...(path.startsWith("/api/issues/") ? { "X-Workspace-ID": fixture.sourceWorkspace.id } : {}),
    } });
    expect([403, 404]).toContain(response.status);
    results.push({ path, status: response.status });
  }
  return results;
}

async function assertOriginalReceipt(fixture: AdministrationFixture, receipt: LifecycleReceipt, key: string, body: LifecycleBody) {
  const code = { complete: "completed", cancel: "cancelled", reopen: "reopened" }[body.action];
  expect(receipt.operation).toMatchObject({ organization_id: fixture.organizationId, actor_id: fixture.administratorId,
    target_kind: "collaboration_issue", target_id: fixture.issueId, kind: `collaboration.${body.action}`,
    state: "applied", result_code: code, confirmation: "not_required" });
  expect(receipt.result).toMatchObject({ issue_id: fixture.issueId, request_id: key, action: body.action, code,
    creator_user_id: fixture.creatorId, previous_handler_user_id: fixture.handlerId,
    handler_user_id: body.successor_user_id ?? fixture.handlerId, previous_status: body.expected_status, status: body.target_status,
    queue_state: body.action === "reopen" ? "paused" : "closed", unsettled_runs: 0 });
  expect(BigInt(receipt.result.collaboration_revision)).toBeGreaterThan(BigInt(body.expected_collaboration_revision));
  expect(BigInt(receipt.result.control_version)).toBeGreaterThan(0n);
  expect(await fixture.administrator.requestJSON(`${adminIssuePath(fixture)}/operations/${key}`)).toEqual(receipt);
}

/** Read-only, exact-issue checks corroborate the real UI/API writes. Failure/race
 * atomicity and retained unknown-proof matrices remain in the backend suite. */
async function committedLedger(fixture: AdministrationFixture, receipt: LifecycleReceipt, reason: string) {
  const db = new pg.Client(fixture.databaseUrl);
  await db.connect();
  try {
    await db.query("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY");
    const currentDatabase = await db.query<{ name: string }>("SELECT current_database() AS name");
    expect(currentDatabase.rows[0]?.name).toBe(new URL(fixture.databaseUrl).pathname.slice(1));
    const issue = await db.query<{ creator_id: string; assignee_id: string; status: string; queue_state: string }>(
      "SELECT i.creator_id, i.assignee_id, i.status, s.queue_state FROM issue i JOIN collaboration_session s ON s.issue_id=i.id WHERE i.id=$1", [fixture.issueId]);
    expect(issue.rows).toEqual([{ creator_id: fixture.creatorId, assignee_id: fixture.successorId, status: receipt.result.status, queue_state: "paused" }]);
    const participants = await db.query<{ user_id: string }>("SELECT user_id FROM issue_participant WHERE issue_id=$1 ORDER BY user_id", [fixture.issueId]);
    expect(participants.rows.map(row => row.user_id)).toEqual([fixture.creatorId, fixture.handlerId, fixture.successorId].sort());
    const handoffs = await db.query<{ id: string; actor_user_id: string; from_user_id: string; to_user_id: string }>(
      "SELECT id, actor_user_id, from_user_id, to_user_id FROM issue_handoff WHERE issue_id=$1 AND purpose='admin_reopen'", [fixture.issueId]);
    expect(handoffs.rows).toHaveLength(1);
    expect(handoffs.rows[0]).toMatchObject({ actor_user_id: fixture.administratorId, from_user_id: fixture.handlerId, to_user_id: fixture.successorId });
    const audit = await db.query<{ id: string }>(
      "SELECT id FROM admin_audit_event WHERE operation_id=$1 AND actor_user_id=$2 AND reason=$3 AND after_state->>'creator_user_id'=$4 AND after_state->>'handler_user_id'=$5 AND after_state->>'queue_state'='paused'",
      [receipt.operation.id, fixture.administratorId, reason, fixture.creatorId, fixture.successorId]);
    expect(audit.rows).toHaveLength(1);
    const history = await db.query<{ id: string; handoff_id: string }>(
      "SELECT id, payload->>'handoff_id' AS handoff_id FROM collaboration_event WHERE issue_id=$1 AND actor_user_id=$2 AND kind='admin_reopened' AND payload->>'request_id'=$3 AND payload->>'operation_id'=$4",
      [fixture.issueId, fixture.administratorId, receipt.result.request_id, receipt.operation.id]);
    expect(history.rows).toHaveLength(1);
    expect(history.rows[0]?.handoff_id).toBe(handoffs.rows[0]?.id);
    const operations = await db.query<{ count: number }>(
      "SELECT count(*)::int AS count FROM admin_operation WHERE target_id=$1 AND actor_id=$2 AND kind IN ('collaboration.complete','collaboration.cancel','collaboration.reopen')",
      [fixture.issueId, fixture.administratorId]);
    expect(operations.rows[0]?.count).toBe(4);
    await db.query("COMMIT");
    return { issue: issue.rows[0], participantIds: participants.rows.map(row => row.user_id), handoff: handoffs.rows[0],
      auditId: audit.rows[0]!.id, eventId: history.rows[0]!.id, lifecycleOperationCount: operations.rows[0]!.count };
  } catch (error) { await db.query("ROLLBACK"); throw error; }
  finally { await db.end(); }
}

export interface AdministrationJourneyAdapter {
  remount(): Promise<void>;
  dropFinalResponse?: boolean;
  capture(stage: string, page: Page, focus: Locator): Promise<void>;
}

export async function exerciseCollaborationAdministration(page: Page, fixture: AdministrationFixture, adapter: AdministrationJourneyAdapter) {
  const root = adminIssuePath(fixture);
  const lifecycle = `${root}/lifecycle`;
  const writes: { key: string; body: LifecycleBody }[] = [];
  const requests: { path: string; method: string; workspaceHeader: boolean }[] = [];
  const observe = (request: Request) => {
    const path = new URL(request.url()).pathname;
    if (path.startsWith("/api/admin/collaboration")) {
      const headers = request.headers();
      requests.push({ path, method: request.method(), workspaceHeader: !!headers["x-workspace-id"] || !!headers["x-workspace-slug"] });
      if (path === lifecycle && request.method() === "POST") writes.push({ key: headers["idempotency-key"] ?? "", body: request.postDataJSON() });
    }
  };
  page.on("request", observe);
  const controls = page.getByRole("group", { name: "Collaboration administration", exact: true });
  const receipts: { key: string; receipt: LifecycleReceipt }[] = [];
  const accountOperations: { key: string; operationId: string; userId: string; action: string }[] = [];
  try {
    const initial = await fetch(`${fixture.apiUrl}${root}`, { headers: { Authorization: `Bearer ${fixture.administrator.getToken()}` } });
    expect(initial.status).toBe(409);
    expect(await initial.json()).toMatchObject({ code: "creator_not_disabled" });
    await expect(page.getByText(creatorRestoredMessage, { exact: true })).toBeVisible();
    await expect(controls.getByRole("button", { name: "Complete task", exact: true })).toHaveCount(0);
    const beforeAccess = await assertNoOrdinaryAccess(fixture);
    accountOperations.push(await accountAction(fixture, fixture.creatorId, "disable"));
    const refresh = page.waitForResponse(response => new URL(response.url()).pathname === root && response.request().method() === "GET" && response.status() === 200);
    await page.getByRole("button", { name: "Refresh task details", exact: true }).click();
    await refresh;
    const firstDetail = await detail(fixture);
    await expect(page.getByRole("heading", { name: firstDetail.identifier, exact: true })).toBeVisible();
    const listing = await fixture.administrator.requestJSON<{ scope: string; items: AdministrationIssue[]; next_cursor: string | null }>("/api/admin/collaboration/issues?limit=100");
    expect(Object.keys(listing).sort()).toEqual(["items", "next_cursor", "scope"]);
    expect(listing.scope).toBe(fixture.organizationId);
    const listed = listing.items.find(item => item.issue_id === fixture.issueId);
    expect(listed).toBeDefined();
    assertMinimalMetadata(fixture, listed!);
    await expect(page.getByText(fixture.privateTitle, { exact: true })).toHaveCount(0);
    await expect(page.getByText(fixture.privateBody, { exact: true })).toHaveCount(0);
    await adapter.capture("disabled-creator", page, controls.locator(".."));

    async function change(action: Action, needsSuccessor = false) {
      const trigger = controls.getByRole("button", { name: { complete: "Complete task", cancel: "Cancel task", reopen: "Reopen task" }[action], exact: true });
      await expect(trigger).toBeEnabled();
      const snapshot = await detail(fixture);
      expect(snapshot.successor_required).toBe(needsSuccessor);
      const option = snapshot.actions.find(item => item.action === action);
      expect(option).toBeDefined();
      await trigger.click();
      const form = controls.getByRole("region", { name: { complete: "Complete task", cancel: "Cancel task", reopen: "Reopen task" }[action], exact: true });
      await expect(form).toBeVisible();
      await expect(form.getByLabel("New task status", { exact: true })).toHaveValue(option!.default_status);
      const reason = needsSuccessor ? "Reopen with the existing eligible participant C" : `S5 synthetic ${action} with original handler H`;
      await form.getByLabel("Reason", { exact: true }).fill(reason);
      if (needsSuccessor) {
        const candidates = await fixture.administrator.requestJSON<{ scope: string; issue_id: string; collaboration_revision: string; items: { user_id: string; name: string; avatar_url: string; external: boolean }[]; next_cursor: string | null }>(`${root}/reopen-candidates`);
        expect(candidates).toMatchObject({ scope: fixture.organizationId, issue_id: fixture.issueId, collaboration_revision: snapshot.collaboration_revision, next_cursor: null });
        expect(candidates.items).toEqual([{ user_id: fixture.successorId, name: fixture.successorName, avatar_url: "", external: true }]);
        const candidate = form.getByRole("radio", { name: new RegExp(fixture.successorName) });
        await expect(candidate).toBeEnabled();
        await expect(form.getByRole("radio", { checked: true })).toHaveCount(0);
        const before = writes.length;
        await form.getByRole("button", { name: "Confirm change", exact: true }).click();
        await expect(form.getByRole("alert")).toHaveText("Choose a participant to handle the reopened task.");
        await expect(form.getByRole("group", { name: "New handler", exact: true })).toBeFocused();
        expect(writes).toHaveLength(before);
        await adapter.capture("explicit-successor", page, form);
        await candidate.check();
      } else {
        await expect(form.getByRole("group", { name: "New handler", exact: true })).toHaveCount(0);
      }
      const before = writes.length;
      let receipt: LifecycleReceipt;
      if (needsSuccessor && adapter.dropFinalResponse) {
        let committed: LifecycleReceipt | undefined;
        await page.route(`**${lifecycle}`, async route => {
          const response = await route.fetch();
          expect(response.status()).toBe(200);
          committed = await response.json();
          await route.abort("failed");
        });
        const failed = page.waitForEvent("requestfailed", request => new URL(request.url()).pathname === lifecycle && request.method() === "POST");
        const recovery = page.waitForResponse(response => new URL(response.url()).pathname.startsWith(`${root}/operations/`) && response.request().method() === "GET");
        await form.getByRole("button", { name: "Confirm change", exact: true }).click();
        await failed;
        const recovered = await recovery;
        await page.unroute(`**${lifecycle}`);
        if (!committed) throw new Error("The dropped response did not contain a committed real result");
        receipt = committed;
        expect(new URL(recovered.url()).pathname).toBe(`${root}/operations/${receipt.result.request_id}`);
        expect(recovered.status()).toBe(200);
        expect(await recovered.json()).toEqual(receipt);
      } else {
        const response = page.waitForResponse(value => new URL(value.url()).pathname === lifecycle && value.request().method() === "POST");
        await form.getByRole("button", { name: "Confirm change", exact: true }).click();
        const changed = await response;
        expect(changed.status()).toBe(200);
        expect(changed.headers()["cache-control"]).toContain("no-store");
        receipt = await changed.json();
      }
      expect(writes).toHaveLength(before + 1);
      const write = writes[before]!;
      expect(write.key).toMatch(/^[0-9a-f-]{36}$/);
      expect(write.body).toEqual({ action, expected_collaboration_revision: snapshot.collaboration_revision, expected_status: snapshot.status,
        target_status: option!.default_status, status_options_version: option!.status_options_version, reason,
        ...(needsSuccessor ? { successor_user_id: fixture.successorId } : {}) });
      await assertOriginalReceipt(fixture, receipt, write.key, write.body);
      const recorded = controls.getByRole("region", { name: "Recorded administration change", exact: true });
      await expect(recorded).toBeVisible();
      await expect(recorded.getByRole("status")).toHaveText({ complete: "Task completed", cancel: "Task cancelled", reopen: "Task reopened with execution paused" }[action]);
      await recorded.locator("summary").click();
      await expect(recorded.getByText(write.key, { exact: true })).toBeVisible();
      receipts.push({ key: write.key, receipt });
      return { receipt, reason };
    }

    await change("complete");
    await controls.getByRole("button", { name: "Close receipt", exact: true }).click();
    await change("reopen");
    await controls.getByRole("button", { name: "Close receipt", exact: true }).click();
    await change("cancel");
    await controls.getByRole("button", { name: "Close receipt", exact: true }).click();
    accountOperations.push(await accountAction(fixture, fixture.handlerId, "disable"));
    const refreshed = page.waitForResponse(response => new URL(response.url()).pathname === root && response.request().method() === "GET" && response.status() === 200);
    await page.getByRole("button", { name: "Refresh task details", exact: true }).click();
    await refreshed;
    const final = await change("reopen", true);
    const ledger = await committedLedger(fixture, final.receipt, final.reason);
    const successorIssue = await fixture.successor.requestJSON<{ creator: { user_id: string }; handler: { user_id: string }; session: { queue_state: string }; execution: { runs: unknown[]; groups: unknown[]; commands: unknown[] } }>(`/api/collaboration/issues/${fixture.issueId}`);
    expect(successorIssue).toMatchObject({ creator: { user_id: fixture.creatorId }, handler: { user_id: fixture.successorId }, session: { queue_state: "paused" }, execution: { runs: [], groups: [], commands: [] } });
    const workbench = await fixture.successor.requestJSON<{ items: { id: string }[] }>("/api/me/workbench?view=current");
    expect(workbench.items.map(item => item.id)).toContain(fixture.issueId);
    const beforeRestore = await detail(fixture);
    const complete = beforeRestore.actions.find(action => action.action === "complete")!;
    accountOperations.push(await accountAction(fixture, fixture.creatorId, "restore"));
    const refusedKey = randomUUID();
    const refused = await fetch(`${fixture.apiUrl}${lifecycle}`, { method: "POST", headers: {
      Authorization: `Bearer ${fixture.administrator.getToken()}`, "Content-Type": "application/json", "Idempotency-Key": refusedKey,
    }, body: JSON.stringify({ action: "complete", expected_collaboration_revision: beforeRestore.collaboration_revision,
      expected_status: beforeRestore.status, target_status: complete.default_status, status_options_version: complete.status_options_version,
      reason: "S5 refusal after the original creator was restored" }) });
    expect(refused.status).toBe(409);
    expect(await refused.json()).toMatchObject({ code: "creator_not_disabled" });
    const restoredRead = page.waitForResponse(response => new URL(response.url()).pathname === root && response.request().method() === "GET");
    const restoredReceipt = page.waitForResponse(response => new URL(response.url()).pathname === `${root}/operations/${final.receipt.result.request_id}` && response.request().method() === "GET");
    await adapter.remount();
    expect((await restoredRead).status()).toBe(409);
    const recovered = await restoredReceipt;
    expect(recovered.status()).toBe(200);
    expect(await recovered.json()).toEqual(final.receipt);
    await expect(page.getByText(creatorRestoredMessage, { exact: true })).toBeVisible();
    const retained = controls.getByRole("region", { name: "Recorded administration change", exact: true });
    await expect(retained).toBeVisible();
    await retained.locator("summary").click();
    await expect(retained.getByText(final.receipt.result.request_id, { exact: true })).toBeVisible();
    await expect(retained.getByText(fixture.creatorId, { exact: true })).toBeVisible();
    await expect(retained.getByText(fixture.successorId, { exact: true })).toBeVisible();
    for (const name of ["Complete task", "Cancel task", "Reopen task", "Confirm change", "Retry original request"]) {
      await expect(controls.getByRole("button", { name, exact: true })).toHaveCount(0);
    }
    expect(writes).toHaveLength(4);
    expect(new Set(writes.map(write => write.key)).size).toBe(4);
    expect(requests.every(request => !request.workspaceHeader)).toBe(true);
    expect(await committedLedger(fixture, final.receipt, final.reason)).toEqual(ledger);
    const afterAccess = await assertNoOrdinaryAccess(fixture);
    await adapter.capture("restored-creator-retained-receipt", page, controls.locator(".."));
    return { issueId: fixture.issueId, sourceWorkspaceId: fixture.sourceWorkspace.id, organizationId: fixture.organizationId,
      administratorId: fixture.administratorId, creatorId: fixture.creatorId, originalHandlerId: fixture.handlerId, successorId: fixture.successorId,
      setup: { creationKey: fixture.creationKey, invitationKey: fixture.invitationKey, accountOperations },
      receipts, ledger, beforeAccess, afterAccess, refusedAfterRestoration: { key: refusedKey, status: refused.status },
      uiLifecyclePosts: writes.length, responseDroppedAfterCommit: adapter.dropFinalResponse === true, requests, providerRuns: 0 };
  } finally { page.off("request", observe); }
}
