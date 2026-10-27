import { execFile, spawn } from "node:child_process";
import { createHash, generateKeyPairSync, randomUUID, sign } from "node:crypto";
import { chmodSync, copyFileSync, mkdtempSync, readdirSync, readFileSync, rmSync, statSync } from "node:fs";
import { createServer, type Socket } from "node:net";
import { tmpdir } from "node:os";
import { isAbsolute, join } from "node:path";
import { promisify } from "node:util";
import { expect, type Locator, type Page } from "@playwright/test";
import type {
  CollaborationCommandHistory, CollaborationCommentInput, CollaborationCommentReceipt, CollaborationControlReceipt, CollaborationIssue,
  CollaborationMentionTargets, CollaborationResourceIdentityView, CollaborationResourceOptions, CollaborationResources, CollaborationRunRecords, CollaborationTriggerPreview,
} from "@multica/core/collaboration";
import pg from "pg";
import { TestApiClient } from "../fixtures";

const execute = promisify(execFile);
export const hostMode = "host_process_supervision_v1";
export const hostAccountPath = (id: string) => `/account?view=participating&issue=${id}`;
export const issueRoot = (id: string) => `/api/collaboration/issues/${id}`;
export const versioned = (path: string) => `${path}${path.includes("?") ? "&" : "?"}execution_view_version=6`;
export const hostRuntimeConfigured = () => ["COLLABORATION_HOST_DAEMON", "COLLABORATION_HOST_PROVIDER", "COLLABORATION_HOST_CLI"]
  .some((key) => process.env[key] !== undefined);

interface Challenge { challenge_id: string; signature_payload: string; body_payload: string }
interface ManagedCredential {
  binding_id: string; binding_epoch: string; capability_version: string; daemon_token: string;
  expires_at: string; principal_user_id: string; auth_version: string;
}
export interface HostRuntime {
  runtimeId: string; workspaceId: string; daemonId: string; deploymentId: string; installationId: string;
  runtimeConfigEpoch: string; credential: ManagedCredential;
}
interface ResourceReceipt { status: number; code: string; offer_id: string | null; offer_version: string | null; target_id: string | null }
interface HostInput { command_id: string; author_user_id: string; body: string; attachment_ids: string[] }
interface HostRuntimeComment { id: string; author_type: string; author_id: string; content: string; parent_id: string | null }
export interface HostVisit {
  identity: Record<string, string>;
  inputs: HostInput[];
  agent_id: string; agent_owner_id: string; resource_owner_id: string;
  protocol_version: number; execution_mode: string;
  workdir_hash: string; temp_hash: string; session_hash: string; brief_hash: string; cli_hash: string; resumed: boolean;
  cli_context: { issue_id: string; agent_id: string; title: string; description: string; inputs: HostInput[];
    squad_id: string | null; step_kind: string; roster: { agent_id: string; name: string; role: string }[];
    comments: HostRuntimeComment[]; dispatch_comment: string };
  attachment_hashes: Record<string, string>;
  /** Relative path to SHA-256 for every regular file in the issue workdir, excluding `.git`. */
  workdir_files: Record<string, string>;
  started_ms: number; released_ms: number | null; closed_ms: number | null;
  socket: Socket;
  replies: HostActionReply[];
}
/** A business write the fake provider performs through its run-local CLI. */
export type HostAction =
  | { kind: "comment"; request_id: string; content: string }
  | { kind: "squad_activity"; request_id: string; outcome: "action" | "no_action" | "failed"; reason: string }
  | { kind: "write_file"; path: string; content: string };
export interface HostActionReceipt { kind: string; request_id?: string; result?: unknown; error?: string; sha256?: string }
interface HostActionReply { receipts: HostActionReceipt[] }
export interface HostIssue {
  id: string; title: string; description: string; targetId: string; channelId: string; agentId: string; name: string;
  attachment: { id: string; name: string; sha256: string } | null;
}
interface HostHTTPFact { operation: string; run_id: string; protocol_version: number; containment: string; attempt_seq?: string; launch_hash?: string; kind?: string; state?: string; status: number }
interface HostDaemonSummary { claims: number; invalid_capacity: boolean; available_slots: number; active_tasks: number; http: HostHTTPFact[] }
interface HostReady { ready: true; protocol_version: number; execution_mode: string; preparation_version: number; runtime_config_epoch: string; max_concurrent_tasks: number; os: string; arch: string }

interface CollaborationGateState {
  enabled: boolean;
  creation_enabled: boolean;
  host_execution_enabled: boolean;
  epoch: string;
}

export function hostEnvironment() {
  const keys =["DATABASE_URL", "COLLABORATION_E2E_API_URL", "NEXT_PUBLIC_API_URL", "MULTICA_DEPLOYMENT_ID", "PLAYWRIGHT_BASE_URL",
    "COLLABORATION_ELECTRON_RENDERER_URL", "COLLABORATION_HOST_DAEMON", "COLLABORATION_HOST_PROVIDER", "COLLABORATION_HOST_CLI"];
  const missing = keys.filter((key) => !process.env[key]);
  if (process.env.E2E_PASSWORD_AUTH !== "1") missing.push("E2E_PASSWORD_AUTH=1");
  if (missing.length) throw new Error(`Configured host acceptance requires: ${missing.join(", ")}`);
  for (const key of ["DATABASE_URL", "COLLABORATION_E2E_API_URL", "NEXT_PUBLIC_API_URL", "PLAYWRIGHT_BASE_URL", "COLLABORATION_ELECTRON_RENDERER_URL"]) {
    expect(["localhost", "127.0.0.1"]).toContain(new URL(process.env[key]!).hostname);
  }
  expect(["", "multica", "postgres", "template0", "template1"]).not.toContain(new URL(process.env.DATABASE_URL!).pathname.slice(1));
  expect(new URL(process.env.COLLABORATION_E2E_API_URL!).origin).toBe(new URL(process.env.NEXT_PUBLIC_API_URL!).origin);
  for (const key of ["COLLABORATION_HOST_DAEMON", "COLLABORATION_HOST_PROVIDER", "COLLABORATION_HOST_CLI"]) {
    if (!isAbsolute(process.env[key]!) || !statSync(process.env[key]!).isFile()) throw new Error(`${key} must name an explicit built fixture executable`);
  }
  return { apiUrl: process.env.COLLABORATION_E2E_API_URL!, deploymentId: process.env.MULTICA_DEPLOYMENT_ID! };
}

/**
 * Packaged acceptance owns an isolated database, but the production defaults
 * keep collaboration creation and host execution closed. Open those gates
 * explicitly for the fixture and restore the exact prior state after the
 * daemon closes so a setup failure cannot leave the task environment open.
 */
export async function openHostAcceptanceGate(): Promise<() => Promise<void>> {
  const databaseUrl = process.env.DATABASE_URL;
  if (!databaseUrl) throw new Error("Host acceptance requires an explicit isolated DATABASE_URL");
  const db = new pg.Client(databaseUrl);
  await db.connect();
  let previous: CollaborationGateState | undefined;
  try {
    await db.query("BEGIN");
    const result = await db.query<CollaborationGateState>(
      "SELECT enabled, creation_enabled, host_execution_enabled, epoch FROM collaboration_execution_gate WHERE id=1 FOR UPDATE",
    );
    previous = result.rows[0];
    if (!previous) throw new Error("The isolated database has no collaboration execution gate");
    if (!previous.enabled || !previous.creation_enabled || !previous.host_execution_enabled) {
      await db.query(
        "UPDATE collaboration_execution_gate SET enabled=true, creation_enabled=true, host_execution_enabled=true, epoch=epoch+1 WHERE id=1",
      );
    }
    await db.query("COMMIT");
  } catch (error) {
    await db.query("ROLLBACK").catch(() => undefined);
    throw error;
  } finally {
    await db.end();
  }

  const original = previous;
  let restored = false;
  return async () => {
    if (restored) return;
    restored = true;
    const restore = new pg.Client(databaseUrl);
    await restore.connect();
    try {
      await restore.query("BEGIN");
      const result = await restore.query<CollaborationGateState>(
        "SELECT enabled, creation_enabled, host_execution_enabled, epoch FROM collaboration_execution_gate WHERE id=1 FOR UPDATE",
      );
      const current = result.rows[0];
      if (!current) throw new Error("The isolated database lost its collaboration execution gate");
      if (current.enabled !== original.enabled || current.creation_enabled !== original.creation_enabled
        || current.host_execution_enabled !== original.host_execution_enabled) {
        await restore.query(
          "UPDATE collaboration_execution_gate SET enabled=$1, creation_enabled=$2, host_execution_enabled=$3, epoch=epoch+1 WHERE id=1",
          [original.enabled, original.creation_enabled, original.host_execution_enabled],
        );
      }
      await restore.query("COMMIT");
    } catch (error) {
      await restore.query("ROLLBACK").catch(() => undefined);
      throw error;
    } finally {
      await restore.end();
    }
  };
}

export async function waitForHostRuntime(owner: TestApiClient, runtimeId: string) {
  await expect.poll(async () => {
    const runtimes = await owner.requestJSON<{ id: string; status: string }[]>("/api/runtimes?owner=me");
    return runtimes.find((runtime) => runtime.id === runtimeId)?.status ?? "missing";
  }, { timeout: 30_000, message: "The host runtime must be online before creating the packaged acceptance issue" }).toBe("online");
}

export function hostAuthority(issue: CollaborationIssue) {
  if (!issue.write_context) throw new Error("Host acceptance requires a current human participant");
  return { expected_lifecycle_epoch: issue.lifecycle_epoch, expected_participant_epoch: issue.write_context.participant_epoch,
    expected_account_epoch: issue.write_context.account_epoch, execution_view_version: 6 };
}

export async function enrollHostRuntime(owner: TestApiClient, ownerId: string, workspaceId: string, apiUrl: string, deploymentId: string): Promise<HostRuntime> {
  const keys = generateKeyPairSync("ed25519");
  const publicKey = keys.publicKey.export({ format: "der", type: "spki" }).subarray(-32).toString("base64url");
  const proof = (challenge: Challenge) => ({ challenge_id: challenge.challenge_id,
    signature_payload: challenge.signature_payload, body_payload: challenge.body_payload,
    signature: sign(null, Buffer.from(challenge.signature_payload, "base64url"), keys.privateKey).toString("base64url") });
  const challenge = await owner.requestJSON<Challenge>("/api/installations/challenges", { method: "POST", body: {
    purpose: "enroll", public_key: publicKey, desktop_version: "0.4.40", os: process.platform === "darwin" ? "macos" : process.platform === "win32" ? "windows" : "linux",
  } });
  expect(JSON.parse(Buffer.from(challenge.signature_payload, "base64url").toString("utf8")))
    .toMatchObject({ user_id: ownerId, deployment_id: deploymentId });
  const installation = await owner.requestJSON<{ installation_id: string }>("/api/installations/enroll", { method: "POST", body: proof(challenge) });
  const pat = await owner.requestJSON<{ token: string }>("/api/tokens", { method: "POST", body: { name: "Host acceptance runtime enrollment", expires_in_days: 1 } });
  const daemonId = randomUUID();
  const binding = await owner.requestJSON<Challenge>("/api/installations/challenges", { method: "POST", body: {
    purpose: "bind", installation_id: installation.installation_id, workspace_id: workspaceId, daemon_id: daemonId,
    public_key: publicKey, expected_binding_epoch: null,
  } });
  const daemonJSON = async <T>(path: string, token: string, body?: unknown): Promise<T> => {
    const response = await fetch(`${apiUrl}${path}`, { method: body === undefined ? "GET" : "POST", redirect: "error", headers: {
      Authorization: `Bearer ${token}`, "Content-Type": "application/json", "X-Client-Capabilities": "managed_installation_v1",
    }, body: body === undefined ? undefined : JSON.stringify(body) });
    if (!response.ok) throw new Error(`Host enrollment ${path} refused (${response.status})`);
    return response.json();
  };
  const credential = await daemonJSON<ManagedCredential>("/api/daemon/installation-bindings", pat.token, proof(binding));
  expect(credential.principal_user_id).toBe(ownerId);
  if (!credential.daemon_token.startsWith("mdt_")) throw new Error("Managed enrollment did not issue a scoped daemon credential");
  const registration = await daemonJSON<{ runtimes: { id: string }[] }>("/api/daemon/register", credential.daemon_token, {
    workspace_id: workspaceId, daemon_id: daemonId, device_name: "Host acceptance native runtime", cli_version: "0.4.40-test", launched_by: "desktop",
    runtimes: [{ name: "Test-owned host Claude", type: "claude", version: "fixture-1", status: "online" }],
  });
  expect(registration.runtimes).toHaveLength(1);
  const runtimeId = registration.runtimes[0]!.id;
  const capabilities = await daemonJSON<{ runtime_config_epoch: string; registration: null }>(
    `/api/collaboration/runtime/capabilities?runtime_id=${runtimeId}`, credential.daemon_token);
  expect(capabilities.registration).toBeNull();
  expect(capabilities.runtime_config_epoch).toMatch(/^[1-9][0-9]*$/);
  return { runtimeId, workspaceId, daemonId, deploymentId, installationId: installation.installation_id,
    runtimeConfigEpoch: capabilities.runtime_config_epoch, credential };
}

export async function startHostDaemon(runtime: HostRuntime, apiUrl: string, readyOnly = false) {
  const directory = mkdtempSync(join(tmpdir(), "multica-host-runtime-"));
  const extension = process.platform === "win32" ? ".exe" : "";
  const provider = join(directory, `fixture-provider${extension}`);
  const cli = join(directory, `fixture-multica${extension}`);
  copyFileSync(process.env.COLLABORATION_HOST_PROVIDER!, provider);
  copyFileSync(process.env.COLLABORATION_HOST_CLI!, cli);
  const cliHash = createHash("sha256").update(readFileSync(cli)).digest("hex");
  chmodSync(provider, 0o700);
  chmodSync(cli, 0o700);
  const kind = await execute(provider, ["--fixture-kind"], { timeout: 5000 });
  expect(kind.stdout.trim()).toBe("multica-host-e2e-provider-v1");
  const visits: HostVisit[] = [];
  const errors: string[] = [];
  const sockets = new Set<Socket>();
  const started = process.hrtime.bigint();
  const now = () => Number(process.hrtime.bigint() - started) / 1_000_000;
  const server = createServer((socket) => {
    sockets.add(socket);
    socket.setEncoding("utf8");
    let body = "";
    let complete = false;
    socket.on("error", () => errors.push("Provider callback socket failed"));
    socket.on("close", () => {
      sockets.delete(socket);
      const visit = visits.find((item) => item.socket === socket);
      if (visit) visit.closed_ms = now();
    });
    socket.on("data", (chunk: string) => {
      body += chunk;
      if (body.length > 1_000_000) { errors.push("Provider callback exceeded its limit"); socket.destroy(); return; }
      if (complete) {
        // Later lines answer scripted actions, in the order they were sent.
        const visit = visits.find((item) => item.socket === socket);
        for (let end = body.indexOf("\n"); end >= 0; end = body.indexOf("\n")) {
          try { visit?.replies.push(JSON.parse(body.slice(0, end))); } catch { errors.push("Provider action reply was not JSON"); }
          body = body.slice(end + 1);
        }
        return;
      }
      const end = body.indexOf("\n");
      if (end < 0) return;
      complete = true;
      const first = body.slice(0, end);
      body = body.slice(end + 1);
      try {
        const visit: Omit<HostVisit, "socket" | "started_ms" | "released_ms" | "closed_ms" | "replies"> = JSON.parse(first);
        expect(visit.protocol_version).toBe(3);
        expect(visit.execution_mode).toBe(hostMode);
        expect(visit.identity.deployment_id).toBe(runtime.deploymentId);
        expect(visit.identity.installation_id).toBe(runtime.installationId);
        expect(visit.identity).not.toHaveProperty("launch_nonce");
        for (const hash of [visit.workdir_hash, visit.temp_hash, visit.session_hash, visit.brief_hash]) expect(hash).toMatch(/^[0-9a-f]{64}$/);
        if (visits.some((item) => item.identity.run_id === visit.identity.run_id)) throw new Error("A run launched more than once");
        visits.push({ ...visit, socket, started_ms: now(), released_ms: null, closed_ms: null, replies: [] });
      } catch { errors.push("Provider callback identity or payload was invalid"); socket.destroy(); }
    });
  });
  await new Promise<void>((resolve, reject) => { server.once("error", reject); server.listen(0, "127.0.0.1", resolve); });
  const address = server.address();
  if (address === null || typeof address === "string") throw new Error("Host callback has no loopback port");
  const env: NodeJS.ProcessEnv = { HOME: directory, USERPROFILE: directory, XDG_CONFIG_HOME: directory, CLAUDE_CONFIG_DIR: directory,
    COLLABORATION_HOST_E2E_CALLBACK: `127.0.0.1:${address.port}` };
  if (readyOnly) env.COLLABORATION_HOST_READY_ONLY = "1";
  for (const key of ["PATH", "TMPDIR", "TMP", "TEMP", "SYSTEMROOT", "SystemRoot", "WINDIR", "COMSPEC"]) {
    if (process.env[key]) env[key] = process.env[key];
  }
  const child = spawn(process.env.COLLABORATION_HOST_DAEMON!, ["__multica_collaboration_host_router_poller"], { env, stdio: "pipe" });
  const messages: (HostReady | HostDaemonSummary)[] = [];
  let output = "";
  let stderr = "";
  child.stdout.setEncoding("utf8");
  child.stderr.setEncoding("utf8");
  child.stdout.on("data", (chunk: string) => {
    output += chunk;
    while (output.includes("\n")) {
      const end = output.indexOf("\n");
      try { messages.push(JSON.parse(output.slice(0, end))); } catch { errors.push("Daemon summary was not JSON"); }
      output = output.slice(end + 1);
    }
  });
  child.stderr.on("data", (chunk: string) => { stderr += chunk.slice(0, 8000 - stderr.length); });
  child.stdin.on("error", () => errors.push("Native daemon fixture input closed unexpectedly"));
  child.on("error", () => errors.push("Native daemon fixture could not start"));
  const exit = new Promise<number | null>((resolve) => child.once("close", resolve));
  child.stdin.write(`${JSON.stringify({ ServerURL: apiUrl, WorkspaceID: runtime.workspaceId, RuntimeID: runtime.runtimeId,
    DaemonID: runtime.daemonId, DeploymentID: runtime.deploymentId, InstallationID: runtime.installationId,
    RuntimeConfigEpoch: runtime.runtimeConfigEpoch, Directory: directory, Executable: provider, CLIExecutable: cli, Credential: runtime.credential })}\n`);
  let closed = false;
  const close = async () => {
    if (closed) throw new Error("Host daemon cleanup was invoked twice");
    closed = true;
    // A suspended daemon cannot read EOF; resuming an already running one is a no-op.
    if (child.exitCode === null) child.kill("SIGCONT");
    child.stdin.end();
    const killTimer = setTimeout(() => child.kill("SIGKILL"), 20_000);
    const exitCode = await exit;
    clearTimeout(killTimer);
    for (const socket of sockets) socket.destroy();
    await new Promise<void>((resolve) => server.close(() => resolve()));
    const ledger = hostLedgerEvidence(join(directory, ".multica", "management", runtime.deploymentId, "collaboration-runs", runtime.installationId));
    // The local ledger contains retained credentials. Only its allowlisted
    // summary is attached; the test-owned directory is removed after shutdown.
    rmSync(directory, { recursive: true, force: true });
    return { exit_code: exitCode, messages, ledger, errors, stderr };
  };
  try {
    await expect.poll(() => ({ errors, exit: child.exitCode, ready: messages.find((item): item is HostReady => "ready" in item) !== undefined }),
      { timeout: 20_000, message: "The real daemon must register host capabilities before consent" }).toEqual({ errors: [], exit: null, ready: true });
    const ready = messages.find((item): item is HostReady => "ready" in item)!;
    expect(ready).toMatchObject({ protocol_version: 3, execution_mode: hostMode, preparation_version: 2, max_concurrent_tasks: 1,
      os: process.platform === "win32" ? "windows" : process.platform, arch: process.arch === "x64" ? "amd64" : process.arch });
    expect(BigInt(ready.runtime_config_epoch)).toBeGreaterThan(BigInt(runtime.runtimeConfigEpoch));
    return { ready, visits, errors, close, cliHash,
      async waitForVisit(count: number) {
        await expect.poll(() => ({ errors, exit: child.exitCode, count: visits.length }), { timeout: 30_000 })
          .toEqual({ errors: [], exit: null, count });
        return visits[count - 1]!;
      },
      release(visit: HostVisit) {
        if (visit.released_ms !== null || visit.socket.destroyed) throw new Error("Provider cannot be released twice or after Stop");
        visit.released_ms = now();
        visit.socket.write("r");
      },
      /**
       * Suspends the native daemon process so the server cannot receive its
       * termination facts; thaw lets it observe Stop and report them. This is a
       * runtime that has not yet confirmed termination, not a forged proof.
       */
      freeze() {
        if (closed || child.exitCode !== null || !child.kill("SIGSTOP")) throw new Error("Native daemon could not be suspended");
      },
      thaw() {
        if (closed || child.exitCode !== null || !child.kill("SIGCONT")) throw new Error("Native daemon could not be resumed");
      },
      /** Has the running provider perform business writes through its run-local CLI, then keep the turn open. */
      async act(visit: HostVisit, actions: HostAction[]) {
        if (visit.released_ms !== null || visit.socket.destroyed) throw new Error("Provider cannot act after release or Stop");
        const before = visit.replies.length;
        visit.socket.write(`a${JSON.stringify(actions)}\n`);
        await expect.poll(() => ({ errors, closed: visit.socket.destroyed, replies: visit.replies.length }), { timeout: 30_000 })
          .toEqual({ errors: [], closed: false, replies: before + 1 });
        const receipts = visit.replies[before]!.receipts;
        expect(receipts.map((receipt) => receipt.error ?? null)).toEqual(actions.map(() => null));
        return receipts;
      },
      evidence: () => visits.map(({ socket: _socket, ...visit }) => visit),
    };
  } catch (error) {
    const result = await close();
    if (result.stderr) throw new Error(`Host daemon readiness failed: ${result.stderr.trim()}`, { cause: error });
    throw error;
  }
}

function hostLedgerEvidence(directory: string) {
  let files: string[];
  try { files = readdirSync(directory).filter((name) => name.endsWith(".json")); } catch { return []; }
  return files.map((name) => {
    const record = JSON.parse(readFileSync(join(directory, name), "utf8"));
    return { version: record.version, run_id: record.identity.run_id, issue_id: record.identity.issue_id, phase: record.phase,
      session_key: record.host?.session_key, namespace_present: Boolean(record.namespace_id), sealed: record.host?.stopped,
      proof_state: record.proof?.state, cleanup: record.proof?.cleanup,
      launches: (record.host?.launches ?? []).map((launch: { attempt_seq: string; kind: string; phase: string; process_instance?: string; pid?: number;
        scope?: string; terminal_accepted?: boolean; authorized?: boolean; termination?: { kind: string; cleanup: unknown } }) => ({
        attempt_seq: launch.attempt_seq, kind: launch.kind, phase: launch.phase, process_observed: Boolean(launch.process_instance && launch.pid),
        scope: launch.scope, terminal_accepted: launch.terminal_accepted, authorized: launch.authorized,
        terminal_kind: launch.termination?.kind, cleanup: launch.termination?.cleanup,
      })) };
  });
}

/** A synthetic password account, onboarded, that can sign in to Web or an installed client. */
export async function createHostActor(apiUrl: string, label: string, suffix: string) {
  const client = new TestApiClient();
  const username = `host${label.toLowerCase()}${suffix}`, password = `Host-${randomUUID()}!`;
  const user = await client.registerPassword(username, password, `Host collaborator ${label}`);
  const onboarded = await client.requestJSON<{ onboarded_at: string | null }>("/api/me/onboarding/complete", { method: "POST", body: {} });
  expect(onboarded.onboarded_at).not.toBeNull();
  return { client, id: user.id,
    authenticateInstalled: (login: (credentials: { username: string; password: string; userId: string }) => Promise<unknown>) =>
      login({ username, password, userId: user.id }),
    async authenticateWeb(page: Page) {
      const response = await page.request.post(`${apiUrl}/auth/login`, { data: { username, password } });
      expect(response.status()).toBe(200);
      const cookies = await page.context().cookies(apiUrl);
      expect(cookies.some((cookie) => cookie.name === "multica_auth" && cookie.httpOnly)).toBe(true);
      expect(cookies.some((cookie) => cookie.name === "multica_csrf" && !cookie.httpOnly)).toBe(true);
    } };
}
export type HostActor = Awaited<ReturnType<typeof createHostActor>>;

/**
 * Public offer, adoption and per-resource owner consent for one agent or squad
 * target, each resource bound to the managed identity of its own host runtime.
 */
export async function adoptHostTarget(owner: HostActor, ownerId: string, issueId: string, workspaceId: string,
  kind: "agent" | "squad", resourceId: string, expected: { role: string; agentId: string; runtime: HostRuntime }[]) {
  const root = issueRoot(issueId);
  const current = () => owner.client.requestJSON<CollaborationIssue>(versioned(root));
  const options = await owner.client.requestJSON<CollaborationResourceOptions>(versioned(`${root}/resources/offers/options?execution_workspace_id=${workspaceId}`));
  const option = options.items.find((item) => item.kind === kind && item.resource_id === resourceId);
  if (!option?.available) throw new Error(`The host ${kind} ${resourceId} is not offerable (${option?.reason ?? "missing"})`);
  expect(option.resources.map((item) => ({ role: item.role, agent_id: item.agent_id, runtime_id: item.runtime_id })).sort((a, b) => a.agent_id.localeCompare(b.agent_id)))
    .toEqual(expected.map((item) => ({ role: item.role, agent_id: item.agentId, runtime_id: item.runtime.runtimeId })).sort((a, b) => a.agent_id.localeCompare(b.agent_id)));
  for (const item of option.resources) expect(item).toMatchObject({ owner_user_id: ownerId, execution_mode: hostMode });
  const offer = await owner.client.requestJSON<ResourceReceipt>(`${root}/resources/offers`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    ...hostAuthority(await current()), kind, resource_id: resourceId, execution_workspace_id: workspaceId,
    roster: option.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) => ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id })),
  } });
  expect(offer).toMatchObject({ status: 201, code: "resource_offered" });
  const before = await owner.client.requestJSON<CollaborationResources>(versioned(`${root}/resources`));
  const adoption = await owner.client.requestJSON<ResourceReceipt>(`${root}/resources/offers/${offer.offer_id}/adopt`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
    ...hostAuthority(await current()), expected_offer_version: offer.offer_version, expected_target_set_version: before.target_set_version,
  } });
  expect(adoption).toMatchObject({ status: 201, code: "resource_offer_adopted" });
  const target = (await owner.client.requestJSON<CollaborationResources>(versioned(`${root}/resources`))).targets.find((item) => item.target_id === adoption.target_id);
  if (!target) throw new Error("Adopted host target is not visible");
  for (const resource of target.resources) {
    const runtime = expected.find((item) => item.agentId === resource.agent_id)!.runtime;
    const identity = await owner.client.requestJSON<CollaborationResourceIdentityView>(versioned(`${root}/resources/identity?target_id=${target.target_id}&agent_id=${resource.agent_id}`));
    expect(identity.identity?.installation_id).toBe(runtime.installationId);
    const consent = await owner.client.requestJSON<{ code: string }>(`${root}/targets/${target.target_id}/consents`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
      ...hostAuthority(await current()), agent_id: resource.agent_id, expected_target_epoch: target.target_epoch,
      expected_config_version: target.config_version, identity: identity.identity,
    } });
    expect(consent.code).toBe("resource_consented");
  }
  return { targetId: adoption.target_id!, resourceId, name: option.name };
}

export async function seedHostRuntime() {
  const env = hostEnvironment();
  const restoreGate = await openHostAcceptanceGate();
  let daemon: Awaited<ReturnType<typeof startHostDaemon>> | undefined;
  const closeCurrentDaemon = async () => {
    const current = daemon;
    daemon = undefined;
    if (current) await current.close();
  };
  try {
    const configuration = await fetch(`${env.apiUrl}/api/config`);
    expect(configuration.status).toBe(200);
    expect(await configuration.json()).toMatchObject({ auth_mode: "password", managed_installation_supported: true, deployment_id: env.deploymentId });
    const suffix = randomUUID().replaceAll("-", "").slice(0, 12);
    const webActor = await createHostActor(env.apiUrl, "Web", suffix), desktopActor = await createHostActor(env.apiUrl, "Electron", suffix);
    const sourceWorkspace = await webActor.client.ensureWorkspace("Host acceptance source", `host-source-${suffix}`);
    const workspace = await webActor.client.requestJSON<{ id: string; slug: string }>("/api/workspaces", { method: "POST", body: {
      name: "Private host acceptance resource", slug: `host-resource-${suffix}`,
    } });
    expect(workspace.id).not.toBe(sourceWorkspace.id);
    webActor.client.setWorkspaceId(workspace.id);
    webActor.client.setWorkspaceSlug(workspace.slug);
    const runtime = await enrollHostRuntime(webActor.client, webActor.id, workspace.id, env.apiUrl, env.deploymentId);
    const delayDaemon = process.env.COLLABORATION_DELAY_HOST_DAEMON === "1";
    daemon = await startHostDaemon(runtime, env.apiUrl, delayDaemon);
    await waitForHostRuntime(webActor.client, runtime.runtimeId);
    expect(await webActor.client.requestJSON<{ creation_enabled: boolean }>("/api/collaboration/creation-options"))
      .toMatchObject({ creation_enabled: true });
    const agent = await webActor.client.requestJSON<{ id: string; name: string }>("/api/agents", { method: "POST", body: {
      name: "Shared host fixture agent", runtime_id: runtime.runtimeId, permission_mode: "private",
    } });
    const issues: HostIssue[] = [];
    for (const label of ["shared", "separate"] as const) {
      const draftId = randomUUID();
      let attachment: HostIssue["attachment"] = null;
      if (label === "shared" && process.env.COLLABORATION_SKIP_HOST_ATTACHMENT !== "1") {
        const body = "Explicit host acceptance attachment. No private files or account data.\n";
        const upload = new FormData();
        upload.set("draft_id", draftId);
        upload.set("source_workspace_id", sourceWorkspace.id);
        upload.set("file", new Blob([body], { type: "text/plain" }), "host-input.txt");
        const response = await fetch(`${env.apiUrl}/api/collaboration/attachments/drafts`, { method: "POST", headers: {
          Authorization: `Bearer ${webActor.client.getToken()!}`, "Idempotency-Key": randomUUID(),
        }, body: upload });
        expect(response.status).toBe(201);
        const file: { id: string; name: string } = await response.json();
        attachment = { id: file.id, name: file.name, sha256: createHash("sha256").update(body).digest("hex") };
      }
      const issue = await webActor.client.requestJSON<CollaborationIssue>("/api/collaboration/issues", { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
        source_workspace_id: sourceWorkspace.id, handler_user_id: desktopActor.id, title: `Host ${label} execution ${suffix}`,
        description: `Test-owned ${label} context ${suffix}. Ordinary fake Claude with native lifecycle observations.`,
        ...(attachment ? { draft_id: draftId } : {}), attachment_ids: attachment ? [attachment.id] : [],
      } });
      const root = issueRoot(issue.id);
      const options = await webActor.client.requestJSON<CollaborationResourceOptions>(versioned(`${root}/resources/offers/options?execution_workspace_id=${workspace.id}`));
      const option = options.items.find((item) => item.kind === "agent" && item.resource_id === agent.id);
      if (!option?.available) throw new Error("The registered ordinary host resource is not offerable");
      expect(option.resources).toHaveLength(1);
      expect(option.resources[0]).toMatchObject({ agent_id: agent.id, runtime_id: runtime.runtimeId, owner_user_id: webActor.id,
        execution_mode: hostMode, provider: "claude", provider_version: "fixture-1", account_source: "runtime_configuration" });
      const current = () => webActor.client.requestJSON<CollaborationIssue>(versioned(root));
      const offer = await webActor.client.requestJSON<ResourceReceipt>(`${root}/resources/offers`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
        ...hostAuthority(await current()), kind: "agent", resource_id: agent.id, execution_workspace_id: workspace.id,
        roster: option.resources.map(({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id }) => ({ role, agent_id, runtime_id, execution_workspace_id, owner_user_id })),
      } });
      expect(offer).toMatchObject({ status: 201, code: "resource_offered", target_id: null });
      const resources = await webActor.client.requestJSON<CollaborationResources>(versioned(`${root}/resources`));
      const adoption = await webActor.client.requestJSON<ResourceReceipt>(`${root}/resources/offers/${offer.offer_id}/adopt`, { method: "POST", headers: { "Idempotency-Key": randomUUID() }, body: {
        ...hostAuthority(await current()), expected_offer_version: offer.offer_version, expected_target_set_version: resources.target_set_version,
      } });
      expect(adoption).toMatchObject({ status: 201, code: "resource_offer_adopted" });
      const target = (await current()).execution?.targets.find((item) => item.id === adoption.target_id);
      if (!target?.channel_id) throw new Error("Public adoption did not create a target channel");
      issues.push({ id: issue.id, title: issue.title, description: issue.description, targetId: target.id,
        channelId: target.channel_id, agentId: agent.id, name: agent.name, attachment });
    }
    if (delayDaemon) {
      await closeCurrentDaemon();
      daemon = await startHostDaemon(runtime, env.apiUrl);
      await waitForHostRuntime(webActor.client, runtime.runtimeId);
    }
    if (!daemon) throw new Error("The host daemon disappeared before the packaged acceptance fixture was ready");
    const rawClose = daemon.close;
    let closePromise: ReturnType<typeof rawClose> | undefined;
    const close = async () => {
      closePromise ??= (async () => {
        try { return await rawClose(); } finally { await restoreGate(); }
      })();
      return closePromise;
    };
    return { ...env, webActor, desktopActor, sourceWorkspace, workspace, runtime, daemon: { ...daemon, close }, main: issues[0]!, separate: issues[1]! };
  } catch (error) {
    try { await closeCurrentDaemon(); } finally { await restoreGate(); }
    throw error;
  }
}
export type HostFixture = Awaited<ReturnType<typeof seedHostRuntime>>;

export async function readHostIssue(fixture: HostFixture, issue: HostIssue = fixture.main, actor = fixture.webActor) {
  const current = await actor.client.requestJSON<CollaborationIssue>(versioned(issueRoot(issue.id)));
  expect(current.execution?.protocol_version).toBe(6);
  expect(current.execution?.execution_gate.enabled).toBe(true);
  expect(current.creator.user_id).toBe(fixture.webActor.id);
  expect(current.handler.user_id).toBe(fixture.desktopActor.id);
  expect(current.execution?.targets).toHaveLength(1);
  return current;
}

export const hostPanel = (page: Page) => page.getByRole("region", { name: "AI execution", exact: true });

export async function assertHostExecutionAvailable(page: Page) {
  const panel = hostPanel(page);
  await expect(panel).toBeVisible();
  await expect(panel).not.toContainText("Shared AI execution is unavailable.");
  await expect(panel).not.toContainText("New AI execution is disabled.");
  return panel;
}

/** Element screenshots still obey ancestor scroll clipping. Fit and verify the
 * actual subject instead of attaching a tall image with an invisible bottom. */
export async function captureHostWebSection(page: Page, focus: Locator, path: string) {
  const height = await focus.evaluate((element) => Math.ceil(element.getBoundingClientRect().height) + 360);
  await page.setViewportSize({ width: 1440, height: Math.max(1100, Math.min(height, 4096)) });
  await focus.scrollIntoViewIfNeeded();
  const geometry = await focus.evaluate((element) => {
    const box = element.getBoundingClientRect();
    let top = 0, bottom = innerHeight;
    for (let parent = element.parentElement; parent; parent = parent.parentElement) {
      if (/auto|scroll|hidden|clip/.test(getComputedStyle(parent).overflowY)) {
        const clip = parent.getBoundingClientRect();
        top = Math.max(top, clip.top);
        bottom = Math.min(bottom, clip.bottom);
      }
    }
    return { width: innerWidth, height: innerHeight, top: box.top, bottom: box.bottom, clipTop: top, clipBottom: bottom };
  });
  expect(geometry.top).toBeGreaterThanOrEqual(geometry.clipTop - 1);
  expect(geometry.bottom).toBeLessThanOrEqual(geometry.clipBottom + 1);
  await focus.screenshot({ path, animations: "disabled" });
  return geometry;
}

export async function openHostConsent(page: Page, issue: HostIssue) {
  await expect(page.getByRole("heading", { name: issue.title, exact: true })).toBeVisible();
  const panel = hostPanel(page);
  const manage = panel.getByRole("button", { name: "Manage AI resources", exact: true });
  if ((await manage.getAttribute("aria-expanded")) !== "true") await manage.click();
  await panel.getByRole("button", { name: `Authorize ${issue.name}`, exact: true }).click();
  await expect(panel.getByRole("heading", { name: "Allow this issue to use this agent", exact: true })).toBeVisible();
  await expect(panel).toContainText("The directory is not an operating-system sandbox");
  await expect(panel).toContainText("does not guarantee removal of background processes outside the managed scope");
  await expect(panel).toContainText("Standard host execution");
  await expect(panel).toContainText("claude fixture-1");
  await assertHostExecutionAvailable(page);
  return panel;
}

export async function authorizeHostIssue(page: Page, fixture: HostFixture, issue: HostIssue) {
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${issueRoot(issue.id)}/targets/${issue.targetId}/consents`
    && response.request().method() === "POST");
  await hostPanel(page).getByRole("button", { name: "Authorize for this issue", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(201);
  expect(response.request().postDataJSON()).toMatchObject({ execution_view_version: 6, agent_id: issue.agentId });
  expect(await response.json()).toMatchObject({ code: "resource_consented", target_id: issue.targetId });
  const resources = await fixture.webActor.client.requestJSON<CollaborationResources>(versioned(`${issueRoot(issue.id)}/resources`));
  expect(resources.targets[0]).toMatchObject({ target_id: issue.targetId, authorized: true });
  expect((await readHostIssue(fixture, issue)).session.queue_state).toBe("paused");
  for (const actor of [fixture.webActor, fixture.desktopActor]) {
    const mentions = await actor.client.requestJSON<CollaborationMentionTargets>(versioned(`${issueRoot(issue.id)}/mention-targets`));
    expect(mentions.targets.map((target) => target.target_id)).toEqual([issue.targetId]);
  }
}

export async function hostControl(page: Page, issue: HostIssue, action: "stop" | "resume", scope: "target" | "issue") {
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${issueRoot(issue.id)}/control` && response.request().method() === "POST");
  const label = `${action === "stop" ? "Stop" : "Resume"} ${scope === "issue" ? "all" : "target"}`;
  await hostPanel(page).getByRole("button", { name: label, exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(200);
  expect(response.request().postDataJSON()).toMatchObject({ action, scope, execution_view_version: 6 });
  const receipt: CollaborationControlReceipt = await response.json();
  expect(receipt).toMatchObject({ action, scope, queue_state: action === "resume" ? "active" : "paused" });
  if (scope === "target") expect(receipt.target_id).toBe(issue.targetId);
  return receipt;
}

export async function sendHostComment(page: Page, issue: HostIssue, marker: string, actorId: string) {
  const composer = page.getByRole("form", { name: "Comments", exact: true });
  await composer.getByLabel("Add comment", { exact: true }).fill(marker);
  await composer.getByRole("group", { name: "AI targets", exact: true }).getByRole("button", { name: issue.name, exact: true }).click();
  const content = await composer.getByLabel("Add comment", { exact: true }).inputValue();
  expect(content).toContain(`mention://agent/${issue.agentId}`);
  const previewPending = page.waitForResponse((response) => new URL(response.url()).pathname === `${issueRoot(issue.id)}/comments/trigger-preview`
    && response.request().method() === "POST");
  await composer.getByRole("button", { name: "Preview delivery", exact: true }).click();
  const previewResponse = await previewPending;
  expect(previewResponse.status()).toBe(200);
  expect(previewResponse.request().postDataJSON()).toMatchObject({ execution_view_version: 6 });
  const preview: CollaborationTriggerPreview = await previewResponse.json();
  expect(preview.targets).toEqual([expect.objectContaining({ target_id: issue.targetId, resource_id: issue.agentId, state: "waiting", hold_reason: null })]);
  await expect(composer.getByRole("button", { name: "Send comment", exact: true })).toBeEnabled();
  const pending = page.waitForResponse((response) => new URL(response.url()).pathname === `${issueRoot(issue.id)}/comments` && response.request().method() === "POST");
  await composer.getByRole("button", { name: "Send comment", exact: true }).click();
  const response = await pending;
  expect(response.status()).toBe(201);
  const input: CollaborationCommentInput = response.request().postDataJSON();
  expect(input).toMatchObject({ content, execution_view_version: 6 });
  expect(input.target_versions.map((target) => target.target_id)).toEqual([issue.targetId]);
  const receipt: CollaborationCommentReceipt = await response.json();
  expect(receipt).toMatchObject({ status: 201, issue_id: issue.id, code: "comment_created" });
  expect(receipt.targets).toHaveLength(1);
  const target = receipt.targets[0];
  if (!target?.command_id || !target.batch_id || !target.command_sequence) throw new Error("Actual host UI send did not create a command");
  expect(target).toMatchObject({ target_id: issue.targetId, channel_id: issue.channelId, state: "waiting" });
  await expect(composer.getByLabel("Add comment", { exact: true })).toHaveValue("");
  return { commandId: target.command_id, batchId: target.batch_id, sequence: target.command_sequence, content, marker, actorId,
    targetId: issue.targetId, requestId: response.request().headers()["idempotency-key"]!, receipt };
}
export type HostComment = Awaited<ReturnType<typeof sendHostComment>>;

export function assertHostVisit(visit: HostVisit, fixture: HostFixture, issue: HostIssue, comments: HostComment[]) {
  expect(visit.cli_hash).toBe(fixture.daemon.cliHash);
  expect(visit.identity).toMatchObject({ issue_id: issue.id, target_id: issue.targetId, channel_id: issue.channelId, batch_id: comments[0]!.batchId });
  expect(visit).toMatchObject({ agent_id: issue.agentId, agent_owner_id: fixture.webActor.id, resource_owner_id: fixture.webActor.id,
    cli_context: { issue_id: issue.id, agent_id: issue.agentId, title: issue.title, description: issue.description } });
  const inputs = comments.map((comment) => ({ command_id: comment.commandId, author_user_id: comment.actorId, body: comment.content, attachment_ids: [] }));
  expect(visit.inputs).toEqual(inputs);
  expect(visit.cli_context.inputs.map(({ command_id, author_user_id, body, attachment_ids }) => ({ command_id, author_user_id, body, attachment_ids }))).toEqual(inputs);
  expect(visit.attachment_hashes).toEqual(issue.attachment ? { [issue.attachment.id]: issue.attachment.sha256 } : {});
}

export async function waitHostSettlement(fixture: HostFixture, issue: HostIssue, visit: HostVisit, stopped = false) {
  await expect.poll(async () => (await readHostIssue(fixture, issue)).execution!.runs.find((run) => run.id === visit.identity.run_id), { timeout: 30_000 })
    .toMatchObject({ progress_state: "settled", proof_state: "host_exit_observed", execution_mode: hostMode, launch_count: "1",
      ...(stopped ? {} : { business_outcome: "success", forced_disposition: null }) });
  const current = await readHostIssue(fixture, issue);
  const run = current.execution!.runs.find((entry) => entry.id === visit.identity.run_id)!;
  expect(process.platform === "win32" ? ["windows_job", "direct_process"] : ["process_group"]).toContain(run.descendant_cleanup?.scope);
  expect(["best_effort", "unverified", "tracked_scope_complete"]).toContain(run.descendant_cleanup?.state);
  return run;
}

export async function readHostHistory(fixture: HostFixture, issue: HostIssue, visits: HostVisit[], comments: HostComment[], stoppedCommand: string | null) {
  const history = await fixture.webActor.client.requestJSON<CollaborationCommandHistory>(`${issueRoot(issue.id)}/commands?limit=100`);
  expect(history.items).toHaveLength(comments.length);
  for (const comment of comments) expect(history.items.find((item) => item.id === comment.commandId)).toMatchObject({
    author_user_id: comment.actorId, target_id: issue.targetId, content: comment.content,
    state: comment.commandId === stoppedCommand ? "stopped" : "succeeded",
  });
  const records: CollaborationRunRecords[] = [];
  for (const visit of visits) {
    const record = await fixture.webActor.client.requestJSON<CollaborationRunRecords>(versioned(`${issueRoot(issue.id)}/runs/${visit.identity.run_id}/records?limit=100`));
    expect(record.run).toMatchObject({ id: visit.identity.run_id, proof_state: "host_exit_observed", execution_mode: hostMode, launch_count: "1" });
    expect(record.resource).toMatchObject({ agent_id: issue.agentId, runtime_id: fixture.runtime.runtimeId,
      resource_owner_user_id: fixture.webActor.id, execution_workspace_id: fixture.workspace.id });
    const expected = comments.filter((comment) => visit.inputs.some((input) => input.command_id === comment.commandId));
    expect(record.items.filter((item) => item.kind === "input").map(({ command_id, author_user_id, body }) => ({ command_id, author_user_id, body })))
      .toEqual(expected.map((comment) => ({ command_id: comment.commandId, author_user_id: comment.actorId, body: comment.content })));
    if (!expected.some((comment) => comment.commandId === stoppedCommand)) {
      expect(record.items.filter((item) => item.kind === "result")).toEqual([expect.objectContaining({ outcome: "success", body: `Host fixture result for ${visit.identity.run_id}` })]);
      expect(record.usage).toMatchObject({ input_tokens: "7", output_tokens: "3" });
    }
    records.push(record);
  }
  return { history, records };
}

export async function inspectHostRecords(page: Page, visits: HostVisit[], comments: HostComment[], stoppedCommand: string) {
  const panel = await assertHostExecutionAvailable(page);
  await expect(panel).toContainText("does not guarantee removal of background processes outside the managed scope");
  const historyButton = panel.getByRole("button", { name: "Command history", exact: true });
  if (await historyButton.getAttribute("aria-expanded") !== "true") await historyButton.click();
  const history = panel.getByRole("region", { name: "Command history", exact: true });
  for (const comment of comments) {
    const entry = history.locator(`[data-command-id="${comment.commandId}"]`);
    await expect(entry).toContainText(comment.marker);
    await expect(entry).toContainText(comment.commandId === stoppedCommand ? "Stopped" : "Succeeded");
    await entry.locator("details > summary").click();
    await expect(entry).toContainText(comment.actorId);
  }
  let last: Locator = panel;
  for (const visit of visits) {
    const stopped = visit.inputs.some((input) => input.command_id === stoppedCommand);
    const disclosure = panel.locator(`[id="run-${visit.identity.run_id}"]`);
    await expect(disclosure).toContainText(stopped ? "Managed execution stopped" : "Managed execution ended");
    await expect(disclosure).not.toContainText("Termination confirmed");
    await disclosure.getByRole("button", { name: `View records for ${visit.identity.run_id}`, exact: true }).click();
    const records = disclosure.getByRole("region", { name: "Run records", exact: true });
    await expect(records).toContainText("Direct launches: 1");
    await expect(records).toContainText("Standard host execution");
    await expect(records).toContainText("Observed cleanup scope");
    if (!stopped) await expect(records).toContainText(`Host fixture result for ${visit.identity.run_id}`);
    last = disclosure;
  }
  return { panel, last };
}
