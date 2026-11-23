import { createHash, randomUUID } from "node:crypto";
import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { isAbsolute, join } from "node:path";
import { expect, type Page, type Response } from "@playwright/test";
import pg from "pg";
import {
  CollaborationStageCodeCandidateSchema, CollaborationStageConfirmationSchema, CollaborationStagePolicySchema,
  CollaborationStageProposalDetailSchema, CollaborationStageViewSchema, collaborationStageErrorCode,
  type CollaborationStageConfirmation, type CollaborationStageView,
} from "../../packages/core/collaboration/stage-schema";
import { enrollHostRuntime, issueRoot, startHostDaemon, waitForHostRuntime, type HostGitInput } from "./collaboration-host-runtime";
import { readClosureIssue, workflowAuthority, type ClosureActor } from "./collaboration-multi-account-closure";
import { startRuntimeHeartbeat } from "./collaboration-performance";
import { RUNTIME_LIVENESS, runtimeAvailability } from "./collaboration-performance-stats";

export const stageAcceptanceConfigured = () => process.env.COLLABORATION_STAGE_ACCEPTANCE !== undefined;
export const stageRoot = (id: string) => `${issueRoot(id)}/stage`;
export const stagePanel = (page: Page) => page.getByRole("region", { name: "Stage gates", exact: true });
export const stageHash = (value: string) => createHash("sha256").update(value).digest("hex");

/** No implicit endpoint, database, production-build or native-origin fallback. */
export function stageEnvironment() {
  if (process.env.COLLABORATION_STAGE_ACCEPTANCE !== "1") throw new Error("Stage acceptance requires COLLABORATION_STAGE_ACCEPTANCE=1");
  const keys = ["DATABASE_URL", "TASK_DATABASE_NAME", "COLLABORATION_E2E_API_URL", "NEXT_PUBLIC_API_URL", "PLAYWRIGHT_BASE_URL",
    "MULTICA_DEPLOYMENT_ID", "COLLABORATION_ELECTRON_RENDERER_URL", "COLLABORATION_ELECTRON_SOURCE_ROOT"] as const;
  const missing = keys.filter((key) => !process.env[key]);
  if (missing.length) throw new Error(`Stage acceptance requires: ${missing.join(", ")}`);
  expect(process.env.E2E_PASSWORD_AUTH).toBe("1");
  for (const key of ["DATABASE_URL", "COLLABORATION_E2E_API_URL", "NEXT_PUBLIC_API_URL", "PLAYWRIGHT_BASE_URL", "COLLABORATION_ELECTRON_RENDERER_URL"] as const) {
    expect(["localhost", "127.0.0.1", "[::1]"]).toContain(new URL(process.env[key]!).hostname);
  }
  const database = process.env.TASK_DATABASE_NAME!;
  expect(["", "postgres", "multica", "template0", "template1"]).not.toContain(database);
  expect(decodeURIComponent(new URL(process.env.DATABASE_URL!).pathname.slice(1))).toBe(database);
  expect(new URL(process.env.NEXT_PUBLIC_API_URL!).origin).toBe(new URL(process.env.COLLABORATION_E2E_API_URL!).origin);
  return { apiUrl: process.env.COLLABORATION_E2E_API_URL!, webUrl: process.env.PLAYWRIGHT_BASE_URL!,
    deploymentId: process.env.MULTICA_DEPLOYMENT_ID!, database };
}

export interface StageHTTPFact {
  actor_id: string;
  method: string;
  path: string;
  request_id: string | null;
  request_body: unknown;
  request_sha256: string | null;
  status: number;
  response_body: unknown;
  response_sha256: string;
  at: string;
}

function parseBody(text: string): unknown {
  if (text === "") return null;
  try { return JSON.parse(text); }
  catch { return text; }
}

export async function stageRequest(apiUrl: string, actor: ClosureActor, path: string, method = "GET", body?: unknown,
  key: string = randomUUID(), headers: Record<string, string> = {}): Promise<StageHTTPFact> {
  const text = body === undefined ? undefined : JSON.stringify(body);
  const response = await fetch(`${apiUrl}${path}`, { method, redirect: "error", headers: {
    Authorization: `Bearer ${actor.client.getToken()!}`, "Content-Type": "application/json",
    ...(method === "GET" ? {} : { "Idempotency-Key": key }), ...headers,
  }, body: text });
  const responseText = await response.text();
  return { actor_id: actor.id, method, path, request_id: method === "GET" ? null : key,
    request_body: body ?? null, request_sha256: text === undefined ? null : stageHash(text), status: response.status,
    response_body: parseBody(responseText), response_sha256: stageHash(responseText), at: new Date().toISOString() };
}

async function responseFact(response: Response, actor: ClosureActor): Promise<StageHTTPFact> {
  const request = response.request(), text = request.postData(), responseText = await response.text();
  return { actor_id: actor.id, method: request.method(), path: new URL(request.url()).pathname,
    request_id: request.headers()["idempotency-key"] ?? null, request_body: text === null ? null : parseBody(text),
    request_sha256: text === null ? null : stageHash(text), status: response.status(),
    response_body: parseBody(responseText), response_sha256: stageHash(responseText), at: new Date().toISOString() };
}

export async function stageUI(page: Page, actor: ClosureActor, path: string, method: "POST" | "PUT", action: () => Promise<unknown>) {
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === path && response.request().method() === method);
  await action();
  return responseFact(await pending, actor);
}

export function stageSuccess<A extends CollaborationStageConfirmation["action"]>(fact: StageHTTPFact, action: A): Extract<CollaborationStageConfirmation, { action: A }> {
  expect(fact.status).toBe(200);
  const body = fact.response_body;
  if (body === null || typeof body !== "object" || !("stage_confirmation" in body)) throw new Error("Stage success omitted its confirmation");
  const confirmation = CollaborationStageConfirmationSchema.parse(body.stage_confirmation);
  if (confirmation.action !== action) throw new Error(`Expected ${action}, got ${confirmation.action}`);
  // The discriminant is verified above; callers still inspect the actual server-created identities.
  return confirmation as Extract<CollaborationStageConfirmation, { action: A }>;
}

export function stageRefusal(fact: StageHTTPFact, status: number, code: string) {
  expect(fact.status).toBe(status);
  expect(collaborationStageErrorCode(fact.response_body)).toBe(code);
  expect(fact.response_body).not.toHaveProperty("stage_confirmation");
}

export const readStage = async (actor: ClosureActor, id: string) =>
  CollaborationStageViewSchema.parse(await actor.client.requestJSON<unknown>(stageRoot(id)));
export const readCode = async (actor: ClosureActor, id: string, candidateId: string) =>
  CollaborationStageCodeCandidateSchema.parse(await actor.client.requestJSON<unknown>(`${stageRoot(id)}/code-candidates/${candidateId}`));
export const readProposal = async (actor: ClosureActor, id: string, proposalId: string) =>
  CollaborationStageProposalDetailSchema.parse(await actor.client.requestJSON<unknown>(`${stageRoot(id)}/proposals/${proposalId}`));
export const readStagePolicy = async (actor: ClosureActor, projectId: string, workspaceId: string) =>
  CollaborationStagePolicySchema.parse(await actor.client.requestJSON<unknown>(`/api/projects/${projectId}/stage-policy`, { headers: { "X-Workspace-ID": workspaceId } }));

export async function stageObservations(actor: ClosureActor, id: string, fields: "policy" | "roles" | "stage" = "stage") {
  const issue = await readClosureIssue(actor, id), view = await readStage(actor, id);
  expect(view.collaboration_revision).toBe(issue.collaboration_revision);
  expect(view.managed).toBe(true);
  const policy = { ...workflowAuthority(issue), expected_policy_revision: view.policy.policy_revision };
  if (fields === "policy") return policy;
  const roles = { ...policy, expected_role_revision: view.role_revision };
  if (fields === "roles") return roles;
  if (!view.stage) throw new Error("Managed issue omitted its stage");
  return { ...roles, expected_stage_revision: view.stage.stage_revision };
}

export async function joinStageSource(owner: ClosureActor, actor: ClosureActor, workspaceId: string) {
  const user = await actor.client.requestJSON<{ email: string }>("/api/me");
  expect(user.email).toBeTruthy();
  const invitation = await owner.client.requestJSON<{ id: string }>(`/api/workspaces/${workspaceId}/members`, {
    method: "POST", body: { email: user.email, role: "member" }, headers: { "X-Workspace-ID": workspaceId },
  });
  await actor.client.requestJSON(`/api/invitations/${invitation.id}/accept`, { method: "POST" });
  actor.client.setWorkspaceId(workspaceId);
  const members = await owner.client.requestJSON<{ id: string; user_id: string; role: string }[]>(`/api/workspaces/${workspaceId}/members`);
  const member = members.find((item) => item.user_id === actor.id);
  expect(member?.role).toBe("member");
  if (!member) throw new Error("Accepted source invitation did not create the member");
  return { invitation_id: invitation.id, member_id: member.id, actor_id: actor.id };
}

export async function showStage(page: Page, webUrl: string, id: string, title: string) {
  await page.goto(new URL(`/account?view=participating&issue=${id}`, webUrl).href);
  await expect(page.getByRole("heading", { name: title, exact: true })).toBeVisible();
  await expect(stagePanel(page)).toBeVisible();
}

export async function confirmStageScope(page: Page, actor: ClosureActor, issueId: string, view: CollaborationStageView,
  code?: { childId: string; prId: string; head: string }) {
  await stagePanel(page).getByRole("button", { name: "Confirm existing sub-issues", exact: true }).click();
  const form = page.getByRole("form", { name: "Confirm existing sub-issues", exact: true });
  for (const child of view.children.items) {
    if (!child.can_open || child.content_digest === null) throw new Error("Fixture cannot confirm a protected child");
    await form.getByRole("checkbox", { name: `${child.identifier} ${child.title}`, exact: true }).check();
    await form.getByLabel(`Code version for ${child.identifier}`, { exact: true }).selectOption(code?.childId === child.issue_id
      ? JSON.stringify({ provider: "github", pull_request_id: code.prId, head_sha: code.head }) : "none");
  }
  await form.getByLabel("Note (optional)", { exact: true }).fill("Synthetic human scope confirmation; no work is dispatched.");
  const label = view.children.items.length === 0 ? "Confirm that no sub-issue is carried over"
    : `Confirm ${view.children.items.length} sub-issue${view.children.items.length === 1 ? "" : "s"}`;
  const fact = await stageUI(page, actor, `${stageRoot(issueId)}/scope-confirmations`, "POST", () => form.getByRole("button", { name: label, exact: true }).click());
  stageSuccess(fact, "confirm_scope");
  await expect(form).toHaveCount(0);
  return fact;
}

/** Forward a real API call, retain its exact reply, then lose only the browser's reply. */
export async function loseStageReply(page: Page, actor: ClosureActor, path: string, action: () => Promise<unknown>, retain: (fact: StageHTTPFact) => void) {
  let resolveFact!: (value: StageHTTPFact) => void, rejectFact!: (reason: unknown) => void;
  const pending = new Promise<StageHTTPFact>((resolve, reject) => { resolveFact = resolve; rejectFact = reject; });
  const pattern = (url: URL) => url.pathname === path;
  await page.route(pattern, async (route) => {
    if (route.request().method() !== "POST") { await route.continue(); return; }
    try {
      const request = route.request(), text = request.postData();
      const response = await route.fetch({ maxRetries: 0, maxRedirects: 0 });
      const responseText = await response.text();
      const fact: StageHTTPFact = { actor_id: actor.id, method: request.method(), path, request_id: request.headers()["idempotency-key"] ?? null,
        request_body: text === null ? null : parseBody(text), request_sha256: text === null ? null : stageHash(text), status: response.status(),
        response_body: parseBody(responseText), response_sha256: stageHash(responseText), at: new Date().toISOString() };
      retain(fact);
      await route.abort("failed");
      resolveFact(fact);
    } catch (error) { rejectFact(error); await route.abort("failed").catch(() => undefined); }
  }, { times: 1 });
  try { await action(); return await pending; }
  finally { await page.unroute(pattern); }
}

/** Actual isolated local Git objects; no source snapshot or commit is forged. */
export function createStageGitInput(suffix: string) {
  const executable = process.env.COLLABORATION_STAGE_GIT;
  if (!executable || !isAbsolute(executable) || !statSync(executable).isFile()) throw new Error("Stage tests require an explicit Git executable");
  const directory = mkdtempSync(join(tmpdir(), "multica-stage-git-"));
  const env = { PATH: process.env.PATH, HOME: directory, GIT_CONFIG_NOSYSTEM: "1", GIT_CONFIG_GLOBAL: "/dev/null",
    GIT_CONFIG_SYSTEM: "/dev/null", GIT_TERMINAL_PROMPT: "0", GIT_AUTHOR_DATE: "2026-11-14T00:00:00Z", GIT_COMMITTER_DATE: "2026-11-14T00:00:00Z" };
  const inherited = spawnSync(executable, ["-C", directory, "rev-parse", "--show-toplevel"], { env, encoding: "utf8" });
  if (inherited.status === 0 || inherited.stdout.trim()) throw new Error("Stage Git fixture inherited a checkout");
  const git = (...args: string[]) => execFileSync(executable, ["-c", "core.hooksPath=/dev/null", "-c", "user.name=Synthetic stage fixture",
    "-c", "user.email=stage-fixture@example.invalid", "-c", "gc.auto=0", "-c", "maintenance.auto=false", ...args],
  { env, cwd: directory, encoding: "utf8", timeout: 20_000, maxBuffer: 1 << 20 }).trim();
  const repository = join(directory, "source");
  git("init", "--template=", "--initial-branch=stage-fixture", repository);
  const marker = (revision: number) => JSON.stringify({ version: 1, revision, callbacks: [`callback-${suffix}`, `callback-${suffix}`], expected_effects: 1 });
  const commit = (revision: number) => {
    writeFileSync(join(repository, "stage-regression.json"), marker(revision), { mode: 0o600 });
    git("-C", repository, "add", "stage-regression.json");
    git("-C", repository, "commit", "-m", `Make synthetic test source revision ${revision} reviewable\n\nConstraint: Local fixture only; no remote repository\nTested: Source will be checked by the controlled provider`);
    return git("-C", repository, "rev-parse", "HEAD");
  };
  const initialHead = commit(1), head = commit(2), bundle = join(directory, "source.bundle");
  git("-C", repository, "bundle", "create", bundle, "--all");
  const bytes = readFileSync(bundle);
  const config: HostGitInput = { executable, bundle_file: bundle, bundle_digest: createHash("sha256").update(bytes).digest("hex"),
    size_bytes: bytes.length, object_format: "sha1", commit_oid: head };
  // The prepared directory must come from the selected bundle, never this dirty working tree.
  writeFileSync(join(repository, "stage-regression.json"), "PRIVATE_DIRTY_SOURCE_MUST_NOT_BE_COPIED", { mode: 0o600 });
  writeFileSync(join(repository, "untracked-private.txt"), "PRIVATE_UNTRACKED_SOURCE_MUST_NOT_BE_COPIED", { mode: 0o600 });
  return { directory, initialHead, head, config, marker_sha256: stageHash(marker(2)), git_executable_sha256: createHash("sha256").update(readFileSync(executable)).digest("hex") };
}

/** Reuse the owner-scoped S5 heartbeat; it changes no runtime/poller deadlines. */
export async function createStageTestResource(actor: ClosureActor, workspace: { id: string; name: string; slug: string }, apiUrl: string,
  deploymentId: string, name: string, record: (name: string, facts: unknown) => void, git?: HostGitInput) {
  const runtime = await enrollHostRuntime(actor.client, actor.id, workspace.id, apiUrl, deploymentId);
  const start = performance.now();
  const heartbeat = startRuntimeHeartbeat(runtime, actor.id, {
    now: () => performance.now() - start, wallNow: Date.now,
    async send(token, body, signal) {
      const response = await fetch(`${apiUrl}/api/daemon/heartbeat`, { method: "POST", redirect: "error",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json", "X-Client-Capabilities": "managed_installation_v1" },
        body: JSON.stringify(body), signal: AbortSignal.any([signal, AbortSignal.timeout(RUNTIME_LIVENESS.heartbeatRequestTimeoutMs)]) });
      return { status: response.status, body: await response.json() };
    }, record: (facts) => record("owner runtime heartbeat", facts),
  });
  let daemon: Awaited<ReturnType<typeof startHostDaemon>> | undefined;
  try {
    await heartbeat.ready;
    if (heartbeat.failure()) throw new Error(heartbeat.failure()!);
    const startedAt = performance.now();
    daemon = await startHostDaemon(runtime, apiUrl, false, { git, testReport: true });
    await waitForHostRuntime(actor.client, runtime.runtimeId);
    const agent = await actor.client.requestJSON<{ id: string; name: string }>("/api/agents", { method: "POST", body: {
      name, runtime_id: runtime.runtimeId, permission_mode: "private",
    } });
    const current = daemon;
    let closed = false;
    return { actor, workspace, runtime, daemon: current, agent,
      async beforeTrigger() {
        if (closed || heartbeat.failure()) throw new Error(heartbeat.failure() ?? "Stage runtime is closed");
        if (performance.now() - startedAt >= RUNTIME_LIVENESS.hostHarnessLifetimeMs) throw new Error("Host fixture lifetime elapsed");
        let status: number | null = null, body: unknown = null;
        try {
          const response = await fetch(`${apiUrl}/api/runtimes?owner=me`, { method: "GET", redirect: "error",
            headers: { Authorization: `Bearer ${actor.client.getToken()!}`, "X-Workspace-ID": workspace.id },
            signal: AbortSignal.timeout(RUNTIME_LIVENESS.heartbeatRequestTimeoutMs) });
          status = response.status;
          if (status === 200) body = await response.json();
        } catch {
          record("runtime before explicit test trigger", { runtime_id: runtime.runtimeId, owner_user_id: actor.id,
            http_status: status, error_code: "runtime_availability_transport_failed" });
          throw new Error("Stage runtime availability read failed");
        }
        const observed = runtimeAvailability(body, runtime, actor.id, Date.now());
        record("runtime before explicit test trigger", { ...observed, http_status: status });
        if (status !== 200 || observed.error_code) throw new Error(observed.error_code ?? `Stage runtime availability read failed (${status})`);
      },
      async close() {
        if (closed) return;
        closed = true;
        try { record("controlled host cleanup", await current.close()); }
        finally { await heartbeat.stop(); }
        if (heartbeat.failure()) throw new Error(heartbeat.failure()!);
      },
    };
  } catch (error) {
    try { if (daemon) record("failed setup host cleanup", await daemon.close()); }
    finally { await heartbeat.stop(); }
    throw error;
  }
}

/** Synthetic connector/email setup only. Membership and stage facts use API/UI. */
export class StageSourceFixture {
  private readonly db: pg.Client;
  private expectedDatabase: string | null = null;

  constructor() {
    if (!process.env.DATABASE_URL?.trim()) throw new Error("Stage source fixtures require an explicit task database");
    this.db = new pg.Client({ connectionString: process.env.DATABASE_URL, connectionTimeoutMillis: 5_000 });
  }

  async open(expectedDatabase: string) {
    await this.db.connect();
    const identity = await this.db.query<{ name: string }>("SELECT current_database() AS name");
    expect(identity.rows[0]?.name).toBe(expectedDatabase);
    this.expectedDatabase = expectedDatabase;
    return { database: identity.rows[0]!.name, synthetic_connector: true };
  }

  /** Password registration omits email; public invitations currently require it. */
  async prepareSyntheticInvitationEmails(actors: { id: string; username: string }[]) {
    if (!this.expectedDatabase) throw new Error("Email fixture setup requires a verified task database");
    const identity = await this.db.query<{ name: string }>("SELECT current_database() AS name");
    expect(identity.rows[0]?.name).toBe(this.expectedDatabase);
    const prepared: { actor_id: string; username: string; email: string; source: string }[] = [];
    await this.db.query("BEGIN");
    try {
      for (const actor of actors) {
        const email = `${actor.username}@example.invalid`;
        const updated = await this.db.query<{ id: string; email: string }>(`UPDATE "user" SET email=$2
          WHERE id=$1 AND email IS NULL AND EXISTS (
            SELECT 1 FROM user_password_credential credential WHERE credential.user_id="user".id AND credential.username=$3)
          RETURNING id,email`, [actor.id, email, actor.username]);
        if (updated.rowCount !== 1 || updated.rows[0]?.id !== actor.id || updated.rows[0]?.email !== email) {
          throw new Error("Synthetic email setup refused an existing email or mismatched registration identity");
        }
        prepared.push({ actor_id: actor.id, username: actor.username, email, source: "synthetic_fixture_email_only" });
      }
      await this.db.query("COMMIT");
    } catch (error) { await this.db.query("ROLLBACK"); throw error; }
    return prepared;
  }

  async seedPullRequest(workspaceId: string, childId: string, suffix: string, head: string) {
    if (!/^[0-9a-f]{40}$/.test(head)) throw new Error("Synthetic PR must name the actual fixture Git commit");
    const prId = randomUUID(), installationId = randomUUID();
    const remoteInstallation = Number.parseInt(suffix.slice(0, 7), 16) + 1;
    await this.db.query("BEGIN");
    try {
      await this.db.query(`INSERT INTO github_installation(id,workspace_id,installation_id,account_login,account_type)
        VALUES($1,$2,$3,$4,'User')`, [installationId, workspaceId, remoteInstallation, `synthetic-stage-${suffix}`]);
      await this.db.query(`INSERT INTO github_pull_request(id,workspace_id,installation_id,repo_owner,repo_name,pr_number,title,state,html_url,pr_created_at,pr_updated_at,head_sha)
        VALUES($1,$2,$3,'synthetic-stage',$4,1,$5,'open','https://example.invalid/synthetic-stage',clock_timestamp(),clock_timestamp(),$6)`,
      [prId, workspaceId, remoteInstallation, `private-repo-${suffix}`, `Synthetic private PR ${suffix}`, head]);
      await this.db.query("INSERT INTO issue_pull_request(issue_id,pull_request_id,reference_only) VALUES($1,$2,false)", [childId, prId]);
      await this.db.query("COMMIT");
    } catch (error) { await this.db.query("ROLLBACK"); throw error; }
    return { prId, head, installationId, remoteInstallation, source_kind: "synthetic_connector" as const };
  }

  async observeCI(prId: string, result: "missing" | "failed" | "passed", head?: string) {
    await this.db.query("BEGIN");
    try {
      if (head) await this.db.query("UPDATE github_pull_request SET head_sha=$2,pr_updated_at=clock_timestamp() WHERE id=$1", [prId, head]);
      await this.db.query("UPDATE github_pull_request SET snapshot_head_sha=head_sha,snapshot_fetched_at=clock_timestamp(),snapshot_complete=true WHERE id=$1", [prId]);
      await this.db.query("DELETE FROM github_pull_request_check_run WHERE pr_id=$1", [prId]);
      if (result !== "missing") await this.db.query(`INSERT INTO github_pull_request_check_run(pr_id,head_sha,ordinal,name,status,conclusion)
        SELECT id,head_sha,0,'Synthetic stage acceptance check','completed',$2 FROM github_pull_request WHERE id=$1`, [prId, result === "passed" ? "success" : "failure"]);
      await this.db.query("COMMIT");
    } catch (error) { await this.db.query("ROLLBACK"); throw error; }
    return { pr_id: prId, result, head: head ?? null, source_kind: "synthetic_connector", observed_at: new Date().toISOString() };
  }

  async effects(issueId: string) {
    const result = await this.db.query<{ children: number; mapped_children: number; commands: number; runs: number }>(`SELECT
      (SELECT count(*)::int FROM issue WHERE parent_issue_id=$1) AS children,
      (SELECT count(*)::int FROM collaboration_stage_child WHERE root_issue_id=$1 AND origin='materialized') AS mapped_children,
      (SELECT count(*)::int FROM collaboration_command command JOIN collaboration_session session ON session.id=command.session_id WHERE session.issue_id=$1) AS commands,
      (SELECT count(*)::int FROM collaboration_run run JOIN collaboration_session session ON session.id=run.session_id WHERE session.issue_id=$1) AS runs`, [issueId]);
    return result.rows[0]!;
  }

  async executionGate() {
    const result = await this.db.query<{ enabled: boolean; creation_enabled: boolean; host_execution_enabled: boolean; epoch: string }>(
      "SELECT enabled,creation_enabled,host_execution_enabled,epoch::text FROM collaboration_execution_gate WHERE id=1");
    if (result.rows.length !== 1) throw new Error("Owned execution gate is missing");
    return result.rows[0]!;
  }

  async close() { await this.db.end(); }
}
