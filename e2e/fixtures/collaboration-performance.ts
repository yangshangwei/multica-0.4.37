/**
 * PAY-241 S5 sampling harness: 100 independent password accounts, each with its
 * own token and its own account WebSocket, observing one collaboration issue.
 *
 * Nothing here runs unless the coordinator opts in and provides an isolated,
 * pre-registered run (see guardEnvironment). The harness only uses public API
 * routes; it never writes user or participant rows, never clears a rate limit
 * and never changes the execution gate outside the existing host fixture.
 * Statistics, correlation and verdicts live in collaboration-performance-stats.
 */
import { execFile } from "node:child_process";
import { createHash, randomBytes, randomUUID } from "node:crypto";
import {
  closeSync, existsSync, fsyncSync, mkdirSync, openSync, readFileSync, realpathSync, renameSync, statSync, writeSync,
} from "node:fs";
import { arch, cpus, freemem, loadavg, platform, release, totalmem } from "node:os";
import { dirname, isAbsolute, join, sep } from "node:path";
import { monitorEventLoopDelay } from "node:perf_hooks";
import { promisify } from "node:util";
import type { CollaborationIssue } from "@multica/core/collaboration";
import pg from "pg";
import { TestApiClient } from "../fixtures";
import {
  adoptHostTarget, enrollHostRuntime, hostAuthority, hostMode, issueRoot, openHostAcceptanceGate, startHostDaemon, versioned, waitForHostRuntime,
  type HostActor, type HostRuntime, type HostVisit,
} from "./collaboration-host-runtime";
import {
  PERFORMANCE_PROTOCOL, RUNTIME_LIVENESS, accountPropagation, accountWriteback, barrierSatisfied, classifyObserverSample, createFrameLedger, expectedEventSeq,
  failedEventRows, matchExpected, notRunEventRows, phaseJitterMs, pickWritebackReaders, preregistrationMismatches, propagationVerdict,
  recordFrame, registrationWaitMs, reportQuantiles, revisionAnomaly, runtimeAvailability, summarize, writebackVerdict,
  type AuthoritativeRead, type FrameArrival, type FrameLedger, type PropagationRow, type QuantileReport, type WritebackRow, type WriteOutcome,
} from "./collaboration-performance-stats";

const execute = promisify(execFile);
type Json = Record<string, unknown>;
export type PerformancePhase = "prepare" | "propagation" | "writeback";

// One monotonic origin for every interval in this process. UTC is recorded
// next to it for correlating logs and is never subtracted.
const clockOrigin = process.hrtime.bigint();
export const monotonicMs = (): number => Number(process.hrtime.bigint() - clockOrigin) / 1_000_000;
const utc = (): string => new Date().toISOString();
const sleep = (ms: number) => new Promise<void>((resolve) => { setTimeout(resolve, ms); });

const isObject = (value: unknown): value is Json => typeof value === "object" && value !== null && !Array.isArray(value);
const child = (value: unknown, key: string): unknown => (isObject(value) ? value[key] : undefined);
const text = (value: unknown): string | null => (typeof value === "string" && value !== "" ? value : null);
const decimal = (value: unknown): string | null => (typeof value === "string" && /^(0|[1-9][0-9]*)$/.test(value) ? value : null);
/** Server codes become file content; keep only the expected shape so no free text is copied. */
const safeCode = (value: unknown): string => (typeof value === "string" && /^[a-z0-9_]{1,64}$/.test(value) ? value : "unrecognized");
const sha256 = (value: Buffer | string): string => createHash("sha256").update(value).digest("hex");
const sha256File = (path: string): string => sha256(readFileSync(path));
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const LOOPBACK = ["localhost", "127.0.0.1", "[::1]"];
const DEFAULT_DATABASES = ["", "multica", "postgres", "template0", "template1"];

// ------------------------------------------------------------ environment

export interface GateState { enabled: boolean; creation_enabled: boolean; host_execution_enabled: boolean; epoch: string }
export interface PerformanceManifest {
  schema_version: 1; run_id: string; purpose: "rehearsal" | "acceptance"; seed: number;
  protocol: unknown; source: unknown;
  api: { url: string; pid: number; commit: string; deployment_id: string; pool_max_conns: number; pool_min_conns: number;
    descriptor_limit: number | null; redis: boolean; replicas: number };
  database: { name: string };
  gate_original: GateState;
  binaries?: { provider_source_sha256: string; provider_sha256: string; daemon_sha256: string; cli_sha256: string };
}
export interface PerformanceEnvironment {
  phase: PerformancePhase; runDir: string; privateDir: string; manifest: PerformanceManifest; manifestSha256: string;
  apiUrl: string; wsUrl: string; databaseUrl: string; deploymentId: string;
}

/** Without the explicit key every test skips, so `make check` or a whole-directory Playwright run never starts sampling. */
export function performanceOptIn(): { enabled: boolean; reason: string } {
  return process.env.COLLABORATION_PERF_ACCEPTANCE === "1" ? { enabled: true, reason: "" }
    : { enabled: false, reason: "Requires COLLABORATION_PERF_ACCEPTANCE=1 and a coordinator-allocated isolated S5 run" };
}

function insideGitWorkTree(directory: string): boolean {
  for (let current = realpathSync(directory); ; current = dirname(current)) {
    if (existsSync(join(current, ".git"))) return true;
    if (dirname(current) === current) return false;
  }
}

function parseManifest(raw: unknown, problems: string[]): PerformanceManifest | null {
  const before = problems.length;
  const api = child(raw, "api"), gate = child(raw, "gate_original"), binaries = child(raw, "binaries");
  const need = (condition: boolean, name: string) => { if (!condition) problems.push(`manifest ${name} is missing or invalid`); };
  need(child(raw, "schema_version") === 1, "schema_version");
  need(/^[A-Za-z0-9._-]{1,80}$/.test(String(child(raw, "run_id") ?? "")), "run_id");
  need(child(raw, "purpose") === "rehearsal" || child(raw, "purpose") === "acceptance", "purpose");
  need(Number.isSafeInteger(child(raw, "seed")) && Number(child(raw, "seed")) >= 0, "seed");
  for (const key of preregistrationMismatches(child(raw, "protocol"))) problems.push(`manifest protocol.${key} differs from the registered protocol`);
  need(child(raw, "source") !== undefined && child(raw, "source") !== null, "source");
  need(text(child(api, "url")) !== null && text(child(api, "commit")) !== null && text(child(api, "deployment_id")) !== null, "api.url/commit/deployment_id");
  need(Number.isSafeInteger(child(api, "pid")) && Number(child(api, "pid")) > 0, "api.pid");
  need(Number.isSafeInteger(child(api, "pool_max_conns")) && Number.isSafeInteger(child(api, "pool_min_conns")), "api.pool_max_conns/pool_min_conns");
  need(child(api, "descriptor_limit") === null || Number.isSafeInteger(child(api, "descriptor_limit")), "api.descriptor_limit");
  need(typeof child(api, "redis") === "boolean" && Number.isSafeInteger(child(api, "replicas")), "api.redis/replicas");
  need(text(child(child(raw, "database"), "name")) !== null, "database.name");
  need(typeof child(gate, "enabled") === "boolean" && typeof child(gate, "creation_enabled") === "boolean"
    && typeof child(gate, "host_execution_enabled") === "boolean" && decimal(child(gate, "epoch")) !== null, "gate_original");
  if (binaries !== undefined) {
    for (const key of ["provider_source_sha256", "provider_sha256", "daemon_sha256", "cli_sha256"]) {
      need(/^[0-9a-f]{64}$/.test(String(child(binaries, key) ?? "")), `binaries.${key}`);
    }
  }
  return problems.length === before ? raw as PerformanceManifest : null;
}

/**
 * Synchronous refusal before any network or database access. With the opt-in
 * present a violation fails loudly instead of skipping: a skipped measurement
 * could be mistaken for one that had nothing to do.
 */
export function guardEnvironment(phase: PerformancePhase): PerformanceEnvironment {
  if (!performanceOptIn().enabled) throw new Error("Performance harness refused: the explicit opt-in is absent");
  const problems: string[] = [];
  if (typeof WebSocket !== "function" || Number(process.versions.node.split(".")[0]) < 22) problems.push("Node 22+ with the native WebSocket is required");
  const runDir = process.env.COLLABORATION_PERF_RUN_DIR ?? "";
  const manifestPath = process.env.COLLABORATION_PERF_MANIFEST ?? "";
  let manifest: PerformanceManifest | null = null;
  let manifestSha256 = "";
  if (!isAbsolute(runDir) || !existsSync(runDir) || !statSync(runDir).isDirectory()) problems.push("COLLABORATION_PERF_RUN_DIR must be an existing absolute directory");
  else {
    if ((statSync(runDir).mode & 0o077) !== 0) problems.push("the run directory must be private (mode 0700)");
    if (insideGitWorkTree(runDir)) problems.push("the run directory must be outside every Git work tree");
    const inside = manifestPath.startsWith(runDir + sep) || manifestPath.startsWith(realpathSync(runDir) + sep);
    if (!isAbsolute(manifestPath) || !inside || !existsSync(manifestPath) || !statSync(manifestPath).isFile()) {
      problems.push("COLLABORATION_PERF_MANIFEST must be a file inside the run directory");
    }
    else {
      const bytes = readFileSync(manifestPath);
      manifestSha256 = sha256(bytes);
      try { manifest = parseManifest(JSON.parse(bytes.toString("utf8")), problems); } catch { problems.push("the manifest is not JSON"); }
    }
  }
  const origin = (value: string): string | null => { try { return new URL(value).origin; } catch { return null; } };
  const parse = (key: string): URL | null => {
    // Key names only: a connection string is never echoed into an error.
    const value = process.env[key];
    if (!value) { problems.push(`${key} is required`); return null; }
    try {
      const url = new URL(value);
      if (!LOOPBACK.includes(url.hostname)) { problems.push(`${key} must be a loopback address`); return null; }
      return url;
    } catch { problems.push(`${key} is not a URL`); return null; }
  };
  const database = parse("DATABASE_URL"), api = parse("NEXT_PUBLIC_API_URL");
  const databaseName = database ? decodeURIComponent(database.pathname.slice(1)) : "";
  if (database && DEFAULT_DATABASES.includes(databaseName)) problems.push("DATABASE_URL names a default database");
  if (process.env.COLLABORATION_E2E_API_URL && api && origin(process.env.COLLABORATION_E2E_API_URL) !== api.origin) {
    problems.push("COLLABORATION_E2E_API_URL and NEXT_PUBLIC_API_URL differ");
  }
  if (api && !["http:", "https:"].includes(api.protocol)) problems.push("NEXT_PUBLIC_API_URL must be http(s)");
  if (manifest) {
    if (database && databaseName !== manifest.database.name) problems.push("DATABASE_URL is not the database recorded in the manifest");
    if (api && origin(manifest.api.url) !== api.origin) problems.push("NEXT_PUBLIC_API_URL is not the API recorded in the manifest");
    // The registered deployment is one in-memory hub with at most the product default pool. A larger pool or
    // another topology is a different measurement and must not be reported as this one.
    if (manifest.api.pool_max_conns > 25) problems.push("the API pool is above the product default of 25");
    if (manifest.api.redis || manifest.api.replicas !== 1) problems.push("the registered deployment is a single API process without Redis");
    if (process.env.MULTICA_DEPLOYMENT_ID !== manifest.api.deployment_id) problems.push("MULTICA_DEPLOYMENT_ID is not the deployment recorded in the manifest");
    if (phase === "writeback") {
      const expected = manifest.binaries;
      if (!expected) problems.push("manifest binaries are required for write-back");
      const files: [string, string | undefined][] = [["COLLABORATION_HOST_PROVIDER", expected?.provider_sha256], ["COLLABORATION_HOST_DAEMON", expected?.daemon_sha256],
        ["COLLABORATION_HOST_CLI", expected?.cli_sha256], ["COLLABORATION_PERF_PROVIDER_SOURCE", expected?.provider_source_sha256]];
      for (const [key, hash] of files) {
        const path = process.env[key] ?? "";
        if (!isAbsolute(path) || !existsSync(path) || !statSync(path).isFile()) problems.push(`${key} must name an explicit absolute file`);
        else if (hash !== undefined && sha256File(path) !== hash) problems.push(`${key} does not have the hash recorded in the manifest`);
      }
      const source = process.env.COLLABORATION_PERF_PROVIDER_SOURCE ?? "";
      // The three Provider fixtures answer --fixture-kind identically. Only the host
      // one implements scripted actions; the closure one writes before its callback.
      if (isAbsolute(source) && existsSync(source) && !readFileSync(source, "utf8").includes("fixture actions were invalid")) {
        problems.push("COLLABORATION_PERF_PROVIDER_SOURCE is not the scripted-action host Provider source");
      }
    }
  }
  if (problems.length > 0 || !manifest || !database || !api) throw new Error(`Performance harness refused: ${problems.join("; ")}`);
  const privateDir = join(runDir, "private");
  mkdirSync(privateDir, { recursive: true, mode: 0o700 });
  return { phase, runDir, privateDir, manifest, manifestSha256, apiUrl: api.origin, wsUrl: api.origin.replace(/^http/, "ws"),
    databaseUrl: process.env.DATABASE_URL!, deploymentId: manifest.api.deployment_id };
}

// ------------------------------------------------------------------ files

function writeExclusive(path: string, value: unknown): void {
  // "wx": evidence from an earlier attempt is never overwritten; a rerun needs a new run directory.
  const fd = openSync(path, "wx", 0o600);
  try { writeSync(fd, `${JSON.stringify(value, null, 2)}\n`); fsyncSync(fd); } finally { closeSync(fd); }
}

function replaceAtomically(path: string, value: unknown): void {
  const temporary = `${path}.${process.pid}.tmp`;
  const fd = openSync(temporary, "w", 0o600);
  try { writeSync(fd, `${JSON.stringify(value, null, 2)}\n`); fsyncSync(fd); } finally { closeSync(fd); }
  renameSync(temporary, path);
}

export interface LineLog { path: string; append(rows: readonly object[]): void; close(): void }
/** Append-only JSON lines, flushed per batch. Raw samples are never rewritten. */
function openLineLog(path: string, exclusive: boolean): LineLog {
  const fd = openSync(path, exclusive ? "ax" : "a", 0o600);
  let closed = false;
  return { path,
    append(rows) { for (const row of rows) writeSync(fd, `${JSON.stringify(row)}\n`); fsyncSync(fd); },
    close() { if (!closed) { closed = true; fsyncSync(fd); closeSync(fd); } } };
}

function readLines(path: string): Json[] {
  return readFileSync(path, "utf8").split("\n").filter((line) => line !== "").map((line, index) => {
    const row: unknown = JSON.parse(line);
    if (!isObject(row)) throw new Error(`${path}:${index + 1} is not a JSON object`);
    return row;
  });
}

// ------------------------------------------------------------------- HTTP

export interface TimedResponse { status: number; body: unknown; start_ms: number; end_ms: number; retry_after: string | null }

/** One request with the caller's own token. Network failures reject: the caller decides what an unknown outcome means. */
async function request(env: PerformanceEnvironment, token: string | null, method: string, path: string,
  options: { body?: unknown; key?: string; timeoutMs?: number; signal?: AbortSignal; workspaceId?: string } = {}): Promise<TimedResponse> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  if (options.key) headers["Idempotency-Key"] = options.key;
  if (options.workspaceId) headers["X-Workspace-ID"] = options.workspaceId;
  const start = monotonicMs();
  const timeout = AbortSignal.timeout(options.timeoutMs ?? 30_000);
  const response = await fetch(`${env.apiUrl}${path}`, { method, headers, redirect: "error", cache: "no-store",
    body: options.body === undefined ? undefined : JSON.stringify(options.body), signal: options.signal ? AbortSignal.any([options.signal, timeout]) : timeout });
  const raw = await response.text();
  const end = monotonicMs();
  let body: unknown = null;
  try { body = raw === "" ? null : JSON.parse(raw); } catch { body = null; }
  return { status: response.status, body, start_ms: start, end_ms: end, retry_after: response.headers.get("retry-after") };
}

async function withDatabase<T>(env: PerformanceEnvironment, action: (db: pg.Client) => Promise<T>): Promise<T> {
  const db = new pg.Client(env.databaseUrl);
  await db.connect();
  try { return await action(db); } finally { await db.end(); }
}

/** Read-only. The harness itself never updates the gate; write-back delegates that to the existing host fixture. */
export async function readGate(env: PerformanceEnvironment): Promise<GateState> {
  return withDatabase(env, async (db) => {
    const result = await db.query<GateState>(
      "SELECT enabled, creation_enabled, host_execution_enabled, epoch::text AS epoch FROM collaboration_execution_gate WHERE id=1");
    const gate = result.rows[0];
    if (!gate) throw new Error("The isolated database has no collaboration execution gate");
    return gate;
  });
}

/** Live identity: the API, its deployment and the database must be the ones the manifest registered. */
export async function verifyLiveEnvironment(env: PerformanceEnvironment): Promise<Json> {
  const problems: string[] = [];
  const config = await request(env, null, "GET", "/api/config");
  if (config.status !== 200 || child(config.body, "auth_mode") !== "password") problems.push("the API is not in password mode");
  if (child(config.body, "deployment_id") !== env.deploymentId) problems.push("the API deployment differs from the manifest");
  if (env.phase === "prepare" && child(config.body, "password_signup_available") !== true) problems.push("password registration is not available");
  if (env.phase === "writeback" && child(config.body, "managed_installation_supported") !== true) problems.push("managed installations are not supported");
  const health = await request(env, null, "GET", "/health");
  if (health.status !== 200 || child(health.body, "status") !== "ok") problems.push("the API health check failed");
  if (child(health.body, "pid") !== env.manifest.api.pid) problems.push("the API process differs from the manifest");
  if (child(health.body, "commit") !== env.manifest.api.commit) problems.push("the API commit differs from the manifest");
  const database = await withDatabase(env, async (db) => (await db.query<{ name: string }>("SELECT current_database() AS name")).rows[0]?.name);
  if (database !== env.manifest.database.name) problems.push("the connected database differs from the manifest");
  if (problems.length > 0) throw new Error(`Performance harness refused: ${problems.join("; ")}`);
  return { api_pid: child(health.body, "pid"), api_commit: child(health.body, "commit"), api_started_at: child(health.body, "started_at"),
    auth_mode: "password", deployment_id: env.deploymentId, database: database ?? null };
}

/**
 * Host, harness, API and PostgreSQL facts recorded before and after a phase.
 * Everything is reported; nothing here is ever subtracted from a sample.
 */
export async function captureResources(env: PerformanceEnvironment, eventLoop: ReturnType<typeof monitorEventLoopDelay> | null): Promise<Json> {
  const limits = platform() === "win32" ? null
    : await execute("/bin/sh", ["-c", "ulimit -n; ulimit -Hn"], { timeout: 5000 }).then((result) => result.stdout.trim().split("\n"), () => null);
  const apiLimits = platform() === "linux" && existsSync(`/proc/${env.manifest.api.pid}/limits`)
    ? readFileSync(`/proc/${env.manifest.api.pid}/limits`, "utf8").split("\n").find((line) => line.startsWith("Max open files")) ?? null : null;
  const realtime = await request(env, null, "GET", "/health/realtime").then((response) => (response.status === 200 ? response.body : { status: response.status }), () => null);
  const postgres = await withDatabase(env, async (db) => {
    const version = (await db.query<{ version: string }>("SELECT version() AS version")).rows[0]?.version ?? null;
    const maximum = (await db.query<{ max_connections: string }>("SHOW max_connections")).rows[0]?.max_connections ?? null;
    // Aggregate counts only: no query text, user names or client addresses.
    const connections = (await db.query<{ database: string; connections: string }>(
      "SELECT datname AS database, count(*)::text AS connections FROM pg_stat_activity WHERE datname IS NOT NULL GROUP BY datname ORDER BY datname")).rows;
    return { version, max_connections: maximum, connections_by_database: connections,
      total_connections: connections.reduce((sum, row) => sum + Number(row.connections), 0) };
  });
  const nanos = (value: number) => Math.round(value / 1000) / 1000;
  return { at_utc: utc(), at_ms: monotonicMs(),
    host: { platform: platform(), release: release(), arch: arch(), cpu_count: cpus().length, cpu_model: cpus()[0]?.model ?? null,
      total_memory_bytes: totalmem(), free_memory_bytes: freemem(), load_average: loadavg() },
    harness: { pid: process.pid, node: process.version, descriptor_soft_limit: limits?.[0] ?? null, descriptor_hard_limit: limits?.[1] ?? null,
      resident_bytes: process.memoryUsage().rss },
    api: { pid: env.manifest.api.pid, descriptor_limit_declared: env.manifest.api.descriptor_limit, descriptor_limit_observed: apiLimits,
      pool_max_conns_declared: env.manifest.api.pool_max_conns, pool_min_conns_declared: env.manifest.api.pool_min_conns,
      redis_declared: env.manifest.api.redis, replicas_declared: env.manifest.api.replicas, realtime_metrics: realtime },
    postgres,
    harness_event_loop_delay_ms: eventLoop === null ? null : { min: nanos(eventLoop.min), mean: nanos(eventLoop.mean), p50: nanos(eventLoop.percentile(50)),
      p95: nanos(eventLoop.percentile(95)), p99: nanos(eventLoop.percentile(99)), max: nanos(eventLoop.max),
      note: "Reported only. Harness scheduling delay is part of every sample and is never subtracted." } };
}

// ---------------------------------------------------------------- prepare

interface AccountRecord {
  actor_index: number; username: string; password: string; name: string; user_id: string | null;
  registration: "planned" | "attempted" | "registered";
}
interface PrepareState {
  schema_version: 1; run_id: string; suffix: string; created_at_utc: string;
  accounts: AccountRecord[];
  /** Wall-clock times of every registration request, because the server window is wall-clock aligned. */
  registration_attempt_wall_ms: number[];
  onboarded: boolean;
  source_workspace: { id: string; slug: string } | null;
  issue: { creation_request_id: string; id: string | null } | null;
  invites: { actor_index: number; request_id: string; confirmed: boolean; code: string | null }[];
  completed_at_utc: string | null;
}
export interface PreparedRun {
  issueId: string; sourceWorkspaceId: string; suffix: string;
  accounts: { actor_index: number; username: string; password: string; user_id: string }[];
}

const statePath = (env: PerformanceEnvironment) => join(env.privateDir, "prepare-state.json");

function loadState(env: PerformanceEnvironment): PrepareState | null {
  if (!existsSync(statePath(env))) return null;
  if ((statSync(statePath(env)).mode & 0o077) !== 0) throw new Error("The private prepare state must be mode 0600");
  const state = JSON.parse(readFileSync(statePath(env), "utf8")) as PrepareState;
  if (state.schema_version !== 1 || state.run_id !== env.manifest.run_id || state.accounts.length !== PERFORMANCE_PROTOCOL.observers) {
    throw new Error("The private prepare state does not belong to this run");
  }
  return state;
}

/** The completed preparation of this run; measurement phases refuse anything less. */
export function loadPreparedRun(env: PerformanceEnvironment): PreparedRun {
  const state = loadState(env);
  if (!state?.completed_at_utc || !state.issue?.id || !state.source_workspace) throw new Error("Preparation for this run is not complete: NOT_RUN");
  const accounts = state.accounts.map(({ actor_index, username, password, user_id }) => {
    if (user_id === null) throw new Error("Preparation recorded an account without an identity: NOT_RUN");
    return { actor_index, username, password, user_id };
  });
  if (new Set(accounts.map((account) => account.user_id)).size !== PERFORMANCE_PROTOCOL.observers) throw new Error("Preparation did not yield 100 distinct accounts: NOT_RUN");
  return { issueId: state.issue.id, sourceWorkspaceId: state.source_workspace.id, suffix: state.suffix, accounts };
}

interface IssueView {
  id: string; status: string; collaboration_revision: string; lifecycle_epoch: string; event_seq: string;
  handler_user_id: string; creator_user_id: string; participant_epoch: string | null; account_epoch: string | null;
  can_read: boolean; can_change_status: boolean; can_transfer: boolean; queue_state: string | null;
}

/** Fields used as evidence are checked explicitly; a malformed projection is not a successful read. */
function parseIssue(body: unknown): IssueView | null {
  const id = text(child(body, "id")), status = text(child(body, "status"));
  const revision = decimal(child(body, "collaboration_revision")), lifecycle = decimal(child(body, "lifecycle_epoch"));
  const seq = decimal(child(child(body, "realtime"), "event_seq"));
  const handler = text(child(child(body, "handler"), "user_id")), creator = text(child(child(body, "creator"), "user_id"));
  if (!id || !status || !revision || !lifecycle || !seq || !handler || !creator) return null;
  const context = child(body, "write_context"), capabilities = child(body, "capabilities");
  return { id: id.toLowerCase(), status, collaboration_revision: revision, lifecycle_epoch: lifecycle, event_seq: seq,
    handler_user_id: handler.toLowerCase(), creator_user_id: creator.toLowerCase(),
    participant_epoch: decimal(child(context, "participant_epoch")), account_epoch: decimal(child(context, "account_epoch")),
    can_read: child(capabilities, "read") === true, can_change_status: child(capabilities, "change_status") === true,
    can_transfer: child(capabilities, "transfer") === true, queue_state: text(child(child(body, "session"), "queue_state")) };
}

const issuePath = (issueId: string) => versioned(issueRoot(issueId));

async function readIssue(env: PerformanceEnvironment, token: string, issueId: string): Promise<IssueView> {
  const response = await request(env, token, "GET", issuePath(issueId));
  const view = response.status === 200 ? parseIssue(response.body) : null;
  if (!view || view.id !== issueId) throw new Error(`The authoritative issue read failed (${response.status})`);
  return view;
}

function workflowAuthority(view: IssueView): Json {
  if (view.participant_epoch === null || view.account_epoch === null) throw new Error("The actor has no current participant write context");
  return { expected_collaboration_revision: view.collaboration_revision, expected_lifecycle_epoch: view.lifecycle_epoch,
    expected_participant_epoch: view.participant_epoch, expected_account_epoch: view.account_epoch };
}

/**
 * Public registration of 100 accounts and public invitation into one issue.
 * Not timed. Resumable: every request identity is persisted before it is sent,
 * and an unknown outcome is settled by looking that identity up, never by
 * switching to a new username or idempotency key.
 */
export async function prepareRun(env: PerformanceEnvironment): Promise<PreparedRun> {
  const log = openLineLog(join(env.runDir, "prepare-log.jsonl"), false);
  const note = (event: string, detail: Json = {}) => log.append([{ at_utc: utc(), event, ...detail }]);
  try {
    let state = loadState(env);
    if (state === null) {
      const suffix = randomBytes(5).toString("hex");
      state = { schema_version: 1, run_id: env.manifest.run_id, suffix, created_at_utc: utc(), registration_attempt_wall_ms: [], onboarded: false,
        source_workspace: null, issue: null, invites: [], completed_at_utc: null,
        accounts: Array.from({ length: PERFORMANCE_PROTOCOL.observers }, (_unused, index) => {
          const label = String(index).padStart(3, "0");
          return { actor_index: index, username: `s5${suffix}_p${label}`, password: `S5-${randomBytes(18).toString("base64url")}`,
            name: `S5 observer ${label}`, user_id: null, registration: "planned" as const };
        }) };
      replaceAtomically(statePath(env), state);
      note("state_created", { suffix });
    }
    const current = state;
    const save = () => replaceAtomically(statePath(env), current);
    if (current.completed_at_utc !== null) { note("already_complete"); return loadPreparedRun(env); }

    // A registration whose response was lost: the same credentials either sign in or they do not.
    const settleUnknown = async (account: AccountRecord): Promise<boolean> => {
      const probe = await request(env, null, "POST", "/auth/login", { body: { username: account.username, password: account.password } });
      note("registration_outcome_probe", { actor_index: account.actor_index, status: probe.status });
      const id = text(child(child(probe.body, "user"), "id"));
      if (probe.status === 200 && id) { account.user_id = id.toLowerCase(); account.registration = "registered"; save(); return true; }
      if (probe.status === 401) { account.registration = "planned"; save(); return false; }
      throw new Error(`Registration outcome for actor ${account.actor_index} could not be settled (${probe.status})`);
    };
    for (const account of current.accounts) {
      if (account.registration === "attempted" && await settleUnknown(account)) continue;
      while (account.registration !== "registered") {
        const wait = registrationWaitMs(current.registration_attempt_wall_ms, Date.now());
        if (wait > 0) { note("registration_window_wait", { actor_index: account.actor_index, wait_ms: wait }); await sleep(wait); continue; }
        account.registration = "attempted";
        current.registration_attempt_wall_ms.push(Date.now());
        save();
        const response = await request(env, null, "POST", "/auth/register",
          { body: { username: account.username, password: account.password, name: account.name } }).catch(() => null);
        note("registration_attempt", { actor_index: account.actor_index, status: response?.status ?? null });
        if (response === null) { await sleep(2000); await settleUnknown(account); continue; }
        if (response.status === 201) {
          const id = text(child(child(response.body, "user"), "id"));
          if (!id || !text(child(response.body, "token"))) throw new Error("Registration returned an unusable session");
          account.user_id = id.toLowerCase(); account.registration = "registered"; save();
        } else if (response.status === 429 || response.status === 503) {
          // The limit is honoured, never cleared: wait out exactly what the server asks for. Both answers are
          // sent before any account row is written, so the same username can be attempted again.
          const seconds = Number(response.retry_after);
          account.registration = "planned"; save();
          const fallback = response.status === 429 ? PERFORMANCE_PROTOCOL.registrationWindowMs : 1000;
          await sleep((Number.isFinite(seconds) && seconds > 0 ? seconds * 1000 : fallback) + PERFORMANCE_PROTOCOL.registrationMarginMs);
        } else if (response.status >= 500) {
          await sleep(2000);
          await settleUnknown(account);
        } else throw new Error(`Registration for actor ${account.actor_index} was refused (${response.status} ${safeCode(child(response.body, "code"))})`);
        await sleep(400);
      }
    }

    const owner = current.accounts[0];
    if (!owner?.user_id) throw new Error("Actor 0 has no identity");
    const ownerClient = new TestApiClient();
    await ownerClient.loginPassword(owner.username, owner.password);
    const ownerToken = ownerClient.getToken();
    if (!ownerToken) throw new Error("Actor 0 has no session");
    if (!current.onboarded) {
      const onboarded = await request(env, ownerToken, "POST", "/api/me/onboarding/complete", { body: {} });
      if (onboarded.status !== 200 || child(onboarded.body, "onboarded_at") === null) throw new Error(`Onboarding failed (${onboarded.status})`);
      current.onboarded = true; save();
    }
    if (current.source_workspace === null) {
      const workspace = await ownerClient.ensureWorkspace("PAY-241 S5 source", `s5-source-${current.suffix}`);
      current.source_workspace = { id: workspace.id, slug: workspace.slug }; save();
    }
    const options = await request(env, ownerToken, "GET", "/api/collaboration/creation-options");
    if (options.status !== 200 || child(options.body, "creation_enabled") !== true) {
      throw new Error("Collaboration creation is not enabled in the task database; the harness does not open the gate for this phase");
    }
    if (current.issue === null) { current.issue = { creation_request_id: randomUUID(), id: null }; save(); }
    if (current.issue.id === null) {
      const key = current.issue.creation_request_id;
      const recover = () => request(env, ownerToken, "GET", `/api/collaboration/creations/${key}?source_workspace_id=${current.source_workspace!.id}`);
      let created = await recover().catch(() => null);
      if (created?.status !== 200) {
        created = await request(env, ownerToken, "POST", "/api/collaboration/issues", { key, body: {
          source_workspace_id: current.source_workspace.id, handler_user_id: owner.user_id, title: `PAY-241 S5 sampling ${current.suffix}`,
          description: "Synthetic sampling issue for 100 test accounts. No business data, model or payment is involved.", attachment_ids: [] } })
          .catch(() => recover());
      }
      const id = text(child(created.body, "id"));
      if (![200, 201].includes(created.status) || !id) throw new Error(`The sampling issue could not be created (${created.status})`);
      current.issue.id = id.toLowerCase(); save();
      note("issue_created", { issue_id: current.issue.id });
    }
    const issueId = current.issue.id;
    const operations = `${issueRoot(issueId)}/operations`;
    for (const account of current.accounts.slice(1)) {
      const recorded = current.invites.find((item) => item.actor_index === account.actor_index);
      if (recorded?.confirmed) continue;
      // The request identity is on disk before the request exists; a rerun looks it up instead of inventing a new one.
      const invite = recorded ?? { actor_index: account.actor_index, request_id: randomUUID(), confirmed: false, code: null };
      if (!recorded) { current.invites.push(invite); save(); }
      let outcome: TimedResponse | null = null;
      if (recorded) outcome = await request(env, ownerToken, "GET", `${operations}/${invite.request_id}`).then((response) => (response.status === 200 ? response : null));
      if (outcome === null) {
        const key = invite.request_id;
        const body = { ...workflowAuthority(await readIssue(env, ownerToken, issueId)), target_user_id: account.user_id };
        outcome = await request(env, ownerToken, "POST", `${issueRoot(issueId)}/participants`, { key, body })
          .catch(async () => { await sleep(2000); return request(env, ownerToken, "GET", `${operations}/${key}`); });
      }
      const code = safeCode(child(outcome.body, "code"));
      note("invite", { actor_index: account.actor_index, request_id: invite.request_id, status: outcome.status, code });
      if (outcome.status !== 200 || code !== "participant_added" && code !== "already_participant") {
        throw new Error(`Inviting actor ${account.actor_index} was refused (${outcome.status} ${code})`);
      }
      invite.confirmed = true; invite.code = code; save();
    }
    await verifyParticipants(env, ownerToken, issueId, current.accounts.map((account) => account.user_id ?? ""));
    writeExclusive(join(env.runDir, "setup-identities.json"), { run_id: current.run_id, issue_id: issueId, source_workspace_id: current.source_workspace.id,
      setup: "public /auth/register and public participant invitations; no direct user or participant rows",
      identities: current.accounts.map(({ actor_index, user_id, username }) => ({ actor_index, user_id, username })) });
    current.completed_at_utc = utc(); save();
    note("complete");
    return loadPreparedRun(env);
  } finally { log.close(); }
}

async function verifyParticipants(env: PerformanceEnvironment, token: string, issueId: string, expected: string[]): Promise<void> {
  const response = await request(env, token, "GET", `${issueRoot(issueId)}/participants`);
  const participants = child(response.body, "participants");
  if (response.status !== 200 || !Array.isArray(participants)) throw new Error(`The participant list could not be read (${response.status})`);
  const active = participants.filter((item) => child(item, "account_active") === true).map((item) => String(child(item, "user_id")).toLowerCase());
  const wanted = new Set(expected);
  if (wanted.size !== PERFORMANCE_PROTOCOL.observers || participants.length !== wanted.size || active.length !== wanted.size
    || new Set(active).size !== wanted.size || active.some((id) => !wanted.has(id))) {
    throw new Error("The participant list is not exactly the 100 active prepared accounts: NOT_RUN");
  }
}

// --------------------------------------------------------------- sessions

export interface ObserverSession {
  actor_index: number; user_id: string; username: string; client: TestApiClient; token: string;
  socket: WebSocket | null; socket_id: string; open: boolean; closed_ms: number | null; close_code: number | null;
  ledger: FrameLedger;
  /** The event sequence already covered by this observer's authoritative read after subscribing. */
  snapshot_seq: bigint;
  waiters: Set<() => void>; frames_received: number; connections: number;
}
export interface SessionSet { issueId: string; sessions: ObserverSession[]; close(): Promise<void> }

/** First-frame authentication, then an explicit issue subscription. `open` alone means nothing. */
async function connectObserver(env: PerformanceEnvironment, session: ObserverSession, issueId: string): Promise<void> {
  const socket = new WebSocket(`${env.wsUrl}/ws?scope=account`);
  session.socket = socket; session.socket_id = randomUUID(); session.open = false; session.closed_ms = null; session.close_code = null;
  session.connections += 1;
  socket.addEventListener("message", (event) => {
    const at = monotonicMs();
    if (session.socket !== socket) return;
    session.frames_received += 1;
    let frame: unknown = null;
    try { frame = JSON.parse(String(event.data)); } catch { /* counted by recordFrame as not_an_object */ }
    if (recordFrame(session.ledger, frame, at).kind === "first") for (const wake of [...session.waiters]) wake();
  });
  socket.addEventListener("close", (event) => {
    if (session.socket !== socket) return;
    session.open = false; session.closed_ms = monotonicMs(); session.close_code = event.code;
    for (const wake of [...session.waiters]) wake();
  });
  try {
    await new Promise<void>((resolve, reject) => {
      let stage: "auth" | "subscribe" = "auth";
      const finish = (error?: Error) => {
        clearTimeout(timer);
        socket.removeEventListener("message", onMessage); socket.removeEventListener("close", onClose);
        if (error) reject(error); else resolve();
      };
      const timer = setTimeout(() => finish(new Error(`Observer ${session.actor_index} was not acknowledged in time (${stage})`)), 20_000);
      const onClose = () => finish(new Error(`Observer ${session.actor_index} was closed during ${stage}`));
      const onMessage = (event: MessageEvent) => {
        let frame: unknown = null;
        try { frame = JSON.parse(String(event.data)); } catch { return; }
        const type = child(frame, "type"), payload = child(frame, "payload");
        if (type === "auth_error") finish(new Error(`Observer ${session.actor_index} failed account authentication`));
        else if (type === "subscribe_error") finish(new Error(`Observer ${session.actor_index} was refused the issue subscription`));
        else if (stage === "auth" && type === "auth_ack") {
          stage = "subscribe";
          socket.send(JSON.stringify({ type: "subscribe", payload: { scope: "issue", id: issueId } }));
        } else if (stage === "subscribe" && type === "subscribe_ack" && child(payload, "scope") === "issue"
          && String(child(payload, "id")).toLowerCase() === issueId) finish();
      };
      socket.addEventListener("message", onMessage);
      socket.addEventListener("close", onClose);
      socket.addEventListener("open", () => socket.send(JSON.stringify({ type: "auth", payload: { token: session.token } })), { once: true });
    });
  } catch (error) { socket.close(); throw error; }
  if (session.socket !== socket || socket.readyState !== WebSocket.OPEN || session.closed_ms !== null) {
    throw new Error(`Observer ${session.actor_index} closed right after subscribing`);
  }
  session.open = true;
  // The authoritative cursor after subscribing: frames at or before it were delivered before this socket existed.
  const view = await readIssue(env, session.token, issueId);
  if (!view.can_read) throw new Error(`Observer ${session.actor_index} cannot read the issue`);
  session.snapshot_seq = BigInt(view.event_seq);
}

/**
 * 100 real logins, 100 sockets, 100 subscriptions, 100 authoritative reads. Any
 * observer that is not ready makes the whole run NOT_RUN; none is dropped.
 */
export async function openObserverSessions(env: PerformanceEnvironment, prepared: PreparedRun): Promise<SessionSet> {
  const sessions: ObserverSession[] = [];
  const close = async () => {
    for (const session of sessions) { const socket = session.socket; session.socket = null; session.open = false; socket?.close(); }
    await sleep(200);
  };
  try {
    for (const account of prepared.accounts) {
      const client = new TestApiClient();
      const started = monotonicMs();
      const user = await client.loginPassword(account.username, account.password);
      const token = client.getToken();
      const me = token ? await request(env, token, "GET", "/api/me") : null;
      if (!token || user.id.toLowerCase() !== account.user_id || me?.status !== 200 || String(child(me.body, "id")).toLowerCase() !== account.user_id) {
        throw new Error(`Actor ${account.actor_index} did not sign in as its prepared identity: NOT_RUN`);
      }
      sessions.push({ actor_index: account.actor_index, user_id: account.user_id, username: account.username, client, token, socket: null, socket_id: "",
        open: false, closed_ms: null, close_code: null, ledger: createFrameLedger(prepared.issueId), snapshot_seq: 0n, waiters: new Set(),
        frames_received: 0, connections: 0 });
      // At most four sign-ins per second, far below the 300 per minute per address.
      await sleep(Math.max(0, 250 - (monotonicMs() - started)));
    }
    if (new Set(sessions.map((session) => session.token)).size !== PERFORMANCE_PROTOCOL.observers) throw new Error("The 100 sessions do not hold 100 distinct tokens: NOT_RUN");
    const pending = [...sessions];
    await Promise.all(Array.from({ length: 10 }, async () => {
      for (let session = pending.shift(); session !== undefined; session = pending.shift()) await connectObserver(env, session, prepared.issueId);
    }));
    const owner = sessions[0];
    if (!owner) throw new Error("No sessions were opened");
    await verifyParticipants(env, owner.token, prepared.issueId, sessions.map((session) => session.user_id));
    if (sessions.some((session) => !session.open) || new Set(sessions.map((session) => session.socket_id)).size !== PERFORMANCE_PROTOCOL.observers
      || new Set(sessions.map((session) => session.user_id)).size !== PERFORMANCE_PROTOCOL.observers) {
      throw new Error("Fewer than 100 distinct ready observers: NOT_RUN");
    }
    return { issueId: prepared.issueId, sessions, close };
  } catch (error) { await close(); throw error; }
}

function waitUntil(session: ObserverSession, deadlineAtMs: number, ready: () => boolean): Promise<boolean> {
  return new Promise((resolve) => {
    if (ready()) { resolve(true); return; }
    if (!session.open) { resolve(false); return; }
    const settle = (value: boolean) => { session.waiters.delete(check); clearTimeout(timer); resolve(value); };
    const check = () => { if (ready()) settle(true); else if (!session.open) settle(false); };
    const timer = setTimeout(() => settle(ready()), Math.max(0, deadlineAtMs - monotonicMs()));
    session.waiters.add(check);
  });
}

/** Reconnection happens only here, between rounds. Whatever a closed socket missed stays missing. */
async function reconnectClosed(env: PerformanceEnvironment, set: SessionSet): Promise<Json[]> {
  const events: Json[] = [];
  for (const session of set.sessions.filter((item) => !item.open)) {
    const closedMs = session.closed_ms, code = session.close_code;
    const reconnected = await connectObserver(env, session, set.issueId).then(() => true, () => false);
    events.push({ actor_index: session.actor_index, closed_ms: closedMs, close_code: code, reconnected, at_ms: monotonicMs(), socket_id: reconnected ? session.socket_id : null });
  }
  return events;
}

// ------------------------------------------------------------ propagation

interface StatusWrite extends WriteOutcome { http_status: number | null; code: string; state_known: boolean; start_ms: number }

/**
 * The timed status write. An unknown outcome is looked up under its original
 * key and never resent under a new one in the same event sequence.
 */
async function writeStatus(env: PerformanceEnvironment, writer: ObserverSession, issueId: string, requestId: string, body: Json, start: number): Promise<StatusWrite> {
  const lookup = () => request(env, writer.token, "GET", `${issueRoot(issueId)}/operations/${requestId}`).catch(() => null);
  const response = await request(env, writer.token, "PATCH", issueRoot(issueId), { key: requestId, body, timeoutMs: PERFORMANCE_PROTOCOL.propagationDeadlineMs + 5000 }).catch(() => null);
  if (response !== null && response.status < 500) {
    const code = safeCode(child(response.body, "code")), revision = decimal(child(response.body, "collaboration_revision"));
    if (response.status === 200 && code === "status_changed" && revision !== null) {
      return { ok: true, receipt_revision: revision, ack_ms: response.end_ms, http_status: 200, code, state_known: true, start_ms: start };
    }
    return { ok: false, ack_ms: response.end_ms, http_status: response.status, code, state_known: true, start_ms: start,
      error_code: response.status === 200 ? `write_code_${code}` : `write_http_${response.status}` };
  }
  // No usable answer. A transport failure may still be in flight, so look twice before calling the state unknown.
  let recovered = await lookup();
  if (response === null && recovered?.status !== 200) { await sleep(2000); recovered = await lookup(); }
  const known = recovered?.status === 200 || response !== null && recovered?.status === 404;
  return { ok: false, http_status: response?.status ?? null, code: safeCode(child(recovered?.body, "code")), state_known: known, start_ms: start,
    error_code: recovered?.status === 200 ? "write_outcome_recovered_by_original_key" : known ? `write_http_${response?.status ?? 0}` : "write_outcome_unknown" };
}

interface Observation { arrival: FrameArrival | null; read: (AuthoritativeRead & { start_ms: number }) | null; attempts: number; first_read_stale: boolean }

/** Wait for this observer's own matching frame, then read with this observer's own token. Never a poll in place of the frame. */
async function observeEvent(env: PerformanceEnvironment, session: ObserverSession, issueId: string, expected: bigint, intended: string,
  deadlineAtMs: number, write: Promise<StatusWrite>): Promise<Observation> {
  await waitUntil(session, deadlineAtMs, () => matchExpected(session.ledger, expected) !== null);
  const arrival = matchExpected(session.ledger, expected);
  const observation: Observation = { arrival, read: null, attempts: 0, first_read_stale: false };
  if (arrival === null) return observation;
  while (monotonicMs() < deadlineAtMs) {
    observation.attempts += 1;
    const response = await request(env, session.token, "GET", issuePath(issueId), { timeoutMs: PERFORMANCE_PROTOCOL.propagationDeadlineMs }).catch(() => null);
    const view = response?.status === 200 ? parseIssue(response.body) : null;
    if (response && view) {
      observation.read = { start_ms: response.start_ms, end_ms: response.end_ms, status: view.status, collaboration_revision: view.collaboration_revision, event_seq: view.event_seq };
      if (view.status === intended && BigInt(view.event_seq) >= expected) {
        const outcome = await write;
        if (!outcome.ok || outcome.receipt_revision === undefined || BigInt(view.collaboration_revision) >= BigInt(outcome.receipt_revision)) return observation;
      }
    }
    if (observation.attempts === 1) observation.first_read_stale = true;
    await sleep(20);
  }
  return observation;
}

export interface PropagationOutcome { summary: Json; pass: boolean; aborted: string | null }

/**
 * 5 warm-up and 100 formal status changes, serial. Formal event k is written by
 * actor k after a public transfer; every event owes one sample per observer.
 */
export async function runPropagation(env: PerformanceEnvironment, set: SessionSet): Promise<PropagationOutcome> {
  const { warmupEvents, formalEvents, propagationDeadlineMs: deadline } = PERFORMANCE_PROTOCOL;
  const samples = openLineLog(join(env.runDir, "propagation-samples.jsonl"), true);
  const rounds = openLineLog(join(env.runDir, "propagation-rounds.jsonl"), true);
  const { issueId, sessions } = set;
  const creator = sessions[0];
  if (!creator) throw new Error("No sessions were opened");
  const startedUtc = utc();
  let aborted: string | null = null;
  let roundFailures = 0;
  try {
    const optionsFor = async (session: ObserverSession) => {
      const response = await request(env, session.token, "GET", `${issueRoot(issueId)}/status-options?action=change_status`);
      const options = child(response.body, "options"), version = text(child(response.body, "status_options_version"));
      if (response.status !== 200 || !Array.isArray(options) || !version) return null;
      return { version, keys: options.map((option) => ({ key: String(child(option, "key")), category: String(child(option, "category")) })) };
    };
    const initial = await optionsFor(creator);
    const pick = (category: string) => initial?.keys.find((option) => option.key === category)?.key ?? initial?.keys.find((option) => option.category === category)?.key;
    const pair = [pick("todo"), pick("in_progress")];
    const [first, second] = pair;
    if (!first || !second || first === second) throw new Error("The source workspace does not offer distinct todo and in_progress statuses: NOT_RUN");
    // Preparation ended long ago, but let two relay periods pass so no earlier frame is still in flight.
    await sleep(2 * PERFORMANCE_PROTOCOL.relayPeriodMs);

    const plan: { phase: "warmup" | "formal"; event_index: number; writer: number }[] = [
      ...Array.from({ length: warmupEvents }, (_unused, index) => ({ phase: "warmup" as const, event_index: index, writer: 0 })),
      ...Array.from({ length: formalEvents }, (_unused, index) => ({ phase: "formal" as const, event_index: index, writer: index })),
    ];
    for (const [ordinal, round] of plan.entries()) {
      const base = { run_id: env.manifest.run_id, phase: round.phase, event_index: round.event_index, issue_id: issueId, deadline_ms: deadline };
      if (aborted !== null) {
        // The remainder was planned and is owed: it is recorded as NOT_RUN, not dropped.
        if (round.phase === "formal") samples.append(notRunEventRows(round.event_index, aborted).map((row) => ({ ...base, ...row, recorded_at_utc: utc() })));
        continue;
      }
      const writer = sessions[round.writer];
      if (!writer) throw new Error("The plan names an actor that has no session");
      const record: Json = { ...base, kind: "round", writer_index: round.writer, writer_user_id: writer.user_id, started_at_utc: utc() };
      let sampled = false;
      const fail = (code: string, stateKnown: boolean) => {
        roundFailures += 1;
        sampled = true;
        samples.append(failedEventRows(round.phase, round.event_index, code).map((row) => ({ ...base, ...row,
          writer_user_id: writer.user_id, observer_user_id: sessions[row.observer_index]?.user_id ?? null, recorded_at_utc: utc() })));
        rounds.append([{ ...record, failed: code, state_known: stateKnown, ended_at_utc: utc() }]);
        if (!stateKnown) aborted = `aborted_after_${round.phase}_${round.event_index}_${code}`;
      };
      try {
        record.reconnects = await reconnectClosed(env, set);
        record.open_sockets = sessions.filter((session) => session.open).length;
        let view = await readIssue(env, creator.token, issueId);
        if (round.phase === "formal" && view.handler_user_id !== writer.user_id && round.writer !== 0) {
          // Not timed: the creator hands responsibility to this round's writer through the public route.
          const key = randomUUID();
          const transfer = await request(env, creator.token, "POST", `${issueRoot(issueId)}/handoffs`, { key, body: { ...workflowAuthority(view),
            target_user_id: writer.user_id, purpose: "transfer", note: `S5 sampling round ${round.event_index}` } })
            .catch(async () => { await sleep(2000); return request(env, creator.token, "GET", `${issueRoot(issueId)}/operations/${key}`).catch(() => null); });
          const code = safeCode(child(transfer?.body, "code"));
          record.transfer = { request_id: key, http_status: transfer?.status ?? null, code };
          if (transfer?.status !== 200 || code !== "transferred" && code !== "already_handler") {
            fail(`transfer_${transfer === null ? "outcome_unknown" : code}`, transfer !== null);
            continue;
          }
        }
        // Quiescence: every observer has the previous event (transfer or last status) before the timed write.
        const barrierStart = monotonicMs();
        let before = await readIssue(env, writer.token, issueId);
        let unsatisfied: number[] = [];
        for (let attempt = 1; attempt <= 3; attempt += 1) {
          const seq = BigInt(before.event_seq);
          const reached = await Promise.all(sessions.map((session) => waitUntil(session, barrierStart + deadline, () => barrierSatisfied(session.ledger, seq, session.snapshot_seq))));
          unsatisfied = reached.flatMap((ok, index) => (ok ? [] : [index]));
          view = await readIssue(env, writer.token, issueId);
          record.barrier = { event_seq: before.event_seq, attempts: attempt, wait_ms: monotonicMs() - barrierStart, unsatisfied_observers: unsatisfied,
            stable: view.event_seq === before.event_seq };
          if (view.event_seq === before.event_seq) break;
          before = view;
        }
        before = view;
        const options = await optionsFor(writer);
        const intended = before.status === first ? second : first;
        if (!before.can_change_status || !options?.keys.some((option) => option.key === intended)) { fail("writer_cannot_change_status", true); continue; }
        const expected = expectedEventSeq(before.event_seq);
        const jitter = phaseJitterMs(env.manifest.seed, ordinal);
        const requestId = randomUUID();
        Object.assign(record, { request_id: requestId, before_revision: before.collaboration_revision, before_event_seq: before.event_seq, before_status: before.status,
          expected_event_seq: expected.toString(), intended_status: intended, status_options_version: options.version, pre_write_jitter_ms: jitter });
        // The original key is on disk before the request exists.
        rounds.append([{ ...record, kind: "round_planned" }]);
        await sleep(jitter);

        const requestStart = monotonicMs();
        const write = writeStatus(env, writer, issueId, requestId, { ...workflowAuthority(before), status: intended, status_options_version: options.version }, requestStart);
        const observations = await Promise.all(sessions.map((session) => observeEvent(env, session, issueId, expected, intended, requestStart + deadline, write)));
        const outcome = await write;
        // Captured before the boundary reconnect, which replaces a dropped observer's socket.
        const openAtEnd = sessions.map((session) => session.open);
        const socketIds = sessions.map((session) => session.socket_id);
        // Round boundary: a dropped observer reconnects and reads, which is recovery and stays a missing sample.
        const recovered = new Map<number, number | null>();
        for (const event of await reconnectClosed(env, set)) {
          const session = sessions[Number(event.actor_index)];
          const reread = session && event.reconnected === true ? await request(env, session.token, "GET", issuePath(issueId)).catch(() => null) : null;
          const seen = reread?.status === 200 ? parseIssue(reread.body) : null;
          recovered.set(Number(event.actor_index), reread && seen?.status === intended && BigInt(seen.event_seq) >= expected ? reread.end_ms - requestStart : null);
        }
        const after = await readIssue(env, writer.token, issueId).catch(() => null);
        const rows = sessions.map((session, index) => {
          const seen = observations[index] ?? { arrival: null, read: null, attempts: 0, first_read_stale: false };
          const decision = classifyObserverSample({ request_start_ms: requestStart, deadline_ms: deadline, socket_open: openAtEnd[index] === true,
            expected_event_seq: expected, intended_status: intended, write: outcome, arrival: seen.arrival, read: seen.read });
          return { ...base, observer_index: index, writer_index: round.writer, writer_user_id: writer.user_id, request_id: requestId,
            before_revision: before.collaboration_revision, before_event_seq: before.event_seq, expected_event_seq: expected.toString(), intended_status: intended,
            pre_write_jitter_ms: jitter, request_start_ms: requestStart, write_ack_ms: outcome.ack_ms ?? null, write_http_status: outcome.http_status,
            receipt_revision: outcome.receipt_revision ?? null, event_id: seen.arrival?.event_id ?? null, frame_revision: seen.arrival?.frame_revision ?? null,
            observer_user_id: session.user_id, socket_id: socketIds[index] ?? null, first_event_ms: seen.arrival?.first_ms ?? null,
            read_start_ms: seen.read?.start_ms ?? null, read_end_ms: seen.read?.end_ms ?? null, observed_revision: seen.read?.collaboration_revision ?? null,
            observed_event_seq: seen.read?.event_seq ?? null, observed_status: seen.read?.status ?? null,
            latency_ms: decision.latency_ms, event_latency_ms: decision.event_latency_ms, read_attempts: seen.attempts, first_read_stale: seen.first_read_stale,
            duplicate_frames: seen.arrival?.duplicates ?? 0, result: decision.result, error_code: decision.error_code,
            correlation_anomaly: seen.arrival !== null && outcome.receipt_revision !== undefined && revisionAnomaly(seen.arrival.frame_revision, outcome.receipt_revision),
            recovered_by_reconnect_read_ms: recovered.get(index) ?? null, recorded_at_utc: utc() };
        });
        samples.append(rows);
        sampled = true;
        const sequenceAnomaly = after !== null && BigInt(after.event_seq) !== expected;
        rounds.append([{ ...record, kind: "round", request_start_ms: requestStart, write: outcome, after_event_seq: after?.event_seq ?? null,
          after_status: after?.status ?? null, event_sequence_anomaly: sequenceAnomaly,
          converged: rows.filter((row) => row.result === "converged").length, ended_at_utc: utc() }]);
        if (!outcome.ok) roundFailures += 1;
        if (!outcome.state_known || after === null) aborted = `aborted_after_${round.phase}_${round.event_index}_state_unknown`;
      } catch (error) {
        // The state is no longer known. If this event has no rows yet it still owes all of them.
        const code = `round_error_${error instanceof Error ? error.name.toLowerCase().replace(/[^a-z0-9_]/g, "") : "unknown"}`;
        if (sampled) aborted = `aborted_after_${round.phase}_${round.event_index}_${code}`;
        else fail(code, false);
      }
    }
  } finally { samples.close(); rounds.close(); }

  // The verdict is recounted from the file that was written, not from memory.
  const raw = readLines(samples.path);
  const accounting = accountPropagation(raw as unknown as PropagationRow[]);
  const verdict = propagationVerdict(accounting);
  const formal = raw.filter((row) => row.phase === "formal");
  const converged = formal.filter((row) => row.result === "converged");
  const finite = (values: unknown[]): QuantileReport | null => {
    const numbers = values.filter((value): value is number => typeof value === "number" && Number.isFinite(value));
    return numbers.length === 0 ? null : reportQuantiles(summarize(numbers));
  };
  const perEventAck = new Map<number, number>();
  for (const row of formal) {
    if (typeof row.write_ack_ms === "number" && typeof row.request_start_ms === "number") perEventAck.set(Number(row.event_index), row.write_ack_ms - row.request_start_ms);
  }
  const errorCodes: Record<string, number> = {};
  for (const row of formal) if (row.result !== "converged") errorCodes[String(row.error_code)] = (errorCodes[String(row.error_code)] ?? 0) + 1;
  const summary: Json = {
    run_id: env.manifest.run_id, purpose: env.manifest.purpose, source: env.manifest.source, manifest_sha256: env.manifestSha256,
    started_at_utc: startedUtc, ended_at_utc: utc(), issue_id: issueId,
    scope: "Serial writes on one issue; API state convergence per observer (matching WebSocket frame, then that observer's own authoritative GET). "
      + "Not concurrent-write load, not React Query or pixel rendering, not multi-replica or cross-region.",
    protocol: PERFORMANCE_PROTOCOL, jitter_seed: env.manifest.seed, aborted, round_failures: roundFailures,
    denominator: { planned: accounting.denominator, rows_in_file: accounting.formal_rows, converged: accounting.converged, missing: accounting.missing,
      not_run: accounting.not_run, filled_not_run: accounting.filled_not_run, over_deadline_reclassified: accounting.over_deadline_reclassified,
      warmup_rows_kept_and_excluded: accounting.warmup_rows },
    verdict: { pass: verdict.pass, conditions: verdict.conditions, budget_ms: verdict.budget_ms, deadline_ms: verdict.deadline_ms, missing: verdict.missing },
    pooled: reportQuantiles(verdict.pooled), slowest_observer_per_event: reportQuantiles(verdict.slowest_per_event),
    excluding_writer: reportQuantiles(verdict.excluding_writer),
    per_observer: verdict.per_observer.map((item, index) => ({ observer_index: index, user_id: sessions[index]?.user_id ?? null, ...reportQuantiles(item) })),
    observers_over_budget: verdict.observers_over_budget.map((index) => ({ observer_index: index, user_id: sessions[index]?.user_id ?? null,
      p95_ms: reportQuantiles(verdict.per_observer[index] ?? verdict.pooled).p95_ms, disposition: "open: to be examined by the coordinator" })),
    segments_informational: { note: "Over converged samples only; never used for the verdict.",
      write_response_ms: finite([...perEventAck.values()]), event_arrival_ms: finite(converged.map((row) => row.event_latency_ms)),
      read_ms: finite(converged.map((row) => (typeof row.read_end_ms === "number" && typeof row.read_start_ms === "number" ? row.read_end_ms - row.read_start_ms : null))),
      write_response_to_visible_ms: finite(converged.map((row) => (typeof row.read_end_ms === "number" && typeof row.write_ack_ms === "number" ? row.read_end_ms - row.write_ack_ms : null))) },
    non_converged_by_code: errorCodes,
    correlation_anomalies: formal.filter((row) => row.correlation_anomaly === true).length,
    // Another writer or more than one event per status change would break the S+1 correlation; it is counted, never hidden.
    event_sequence_anomalies: readLines(rounds.path).filter((row) => row.kind === "round" && row.event_sequence_anomaly === true).length,
    barrier_timeouts: readLines(rounds.path).filter((row) => {
      const waiting = child(row.barrier, "unsatisfied_observers");
      return row.kind === "round" && Array.isArray(waiting) && waiting.length > 0;
    }).length,
    duplicate_frames: formal.reduce((sum, row) => sum + Number(row.duplicate_frames ?? 0), 0),
    first_read_stale: formal.filter((row) => row.first_read_stale === true).length,
    sessions: { distinct_users: new Set(sessions.map((session) => session.user_id)).size, total_connections: sessions.reduce((sum, session) => sum + session.connections, 0),
      frames_received: sessions.reduce((sum, session) => sum + session.frames_received, 0) },
    samples_file: { name: "propagation-samples.jsonl", lines: raw.length, sha256: sha256File(samples.path) },
    rounds_file: { name: "propagation-rounds.jsonl", sha256: sha256File(rounds.path) },
  };
  writeExclusive(join(env.runDir, "summary-propagation.json"), summary);
  writeExclusive(join(env.runDir, "failures-propagation.json"), { run_id: env.manifest.run_id, aborted,
    non_converged: formal.filter((row) => row.result !== "converged"), warmup_non_converged: raw.filter((row) => row.phase === "warmup" && row.result !== "converged") });
  return { summary, pass: verdict.pass, aborted };
}

// -------------------------------------------------------------- write-back

type HostDaemon = Awaited<ReturnType<typeof startHostDaemon>>;
export interface WritebackOutcome { summary: Json; pass: boolean; aborted: string | null }

interface RuntimeHeartbeatIO {
  send(token: string, body: { runtime_id: string; supports_batch_import: true }, signal: AbortSignal): Promise<{ status: number; body: unknown }>;
  record(row: Json): void;
  now(): number;
  wallNow(): number;
}

/** The production daemon's owner-scoped HTTP heartbeat, with one bounded request in flight. */
export function startRuntimeHeartbeat(runtime: HostRuntime, ownerId: string, io: RuntimeHeartbeatIO) {
  if (runtime.credential.principal_user_id !== ownerId || !runtime.credential.daemon_token.startsWith("mdt_")) {
    throw new Error("runtime_heartbeat_owner_mismatch");
  }
  const expiresAt = Date.parse(runtime.credential.expires_at);
  if (!Number.isFinite(expiresAt) || expiresAt <= io.wallNow() + RUNTIME_LIVENESS.heartbeatRequestTimeoutMs) {
    throw new Error("runtime_heartbeat_credential_expired");
  }
  const controller = new AbortController();
  let stopped = false, failure: string | null = null, timer: ReturnType<typeof setTimeout> | undefined;
  let wake: (() => void) | undefined;
  let markReady: () => void = () => {};
  const ready = new Promise<void>((resolve) => { markReady = resolve; });
  const done = (async () => {
    let sequence = 0;
    while (!stopped) {
      sequence += 1;
      const start = io.now();
      const base = { sequence, runtime_id: runtime.runtimeId, owner_user_id: ownerId, credential_kind: "managed_daemon",
        started_at_utc: new Date(io.wallNow()).toISOString(), start_ms: start };
      io.record({ ...base, kind: "heartbeat_start" });
      let status: number | null = null, code: string | null = null;
      try {
        if (expiresAt <= io.wallNow() + RUNTIME_LIVENESS.heartbeatRequestTimeoutMs) code = "runtime_heartbeat_credential_expired";
        else {
          const response = await io.send(runtime.credential.daemon_token, { runtime_id: runtime.runtimeId, supports_batch_import: true }, controller.signal);
          status = response.status;
          if (status !== 200) code = `runtime_heartbeat_http_${status}`;
          // HTTP heartbeat replies omit runtime_id; identity is bound by the request and managed credential.
          else if (child(response.body, "status") !== "ok" || child(response.body, "runtime_gone") === true) code = "runtime_heartbeat_invalid_ack";
        }
      } catch { code = stopped ? "heartbeat_cancelled" : "runtime_heartbeat_transport_failed"; }
      if (code !== null && code !== "heartbeat_cancelled") failure ??= code;
      io.record({ ...base, kind: "heartbeat_result", end_ms: io.now(), http_status: status, error_code: code,
        ended_at_utc: new Date(io.wallNow()).toISOString() });
      markReady();
      if (!stopped) await new Promise<void>((resolve) => { wake = resolve; timer = setTimeout(resolve, RUNTIME_LIVENESS.heartbeatIntervalMs); });
    }
  })().catch(() => { failure ??= "runtime_heartbeat_log_failed"; }).finally(() => { markReady(); });
  return { ready, failure: () => failure,
    async stop() { stopped = true; clearTimeout(timer); wake?.(); controller.abort(); await done; } };
}

async function authoritativeIssue(env: PerformanceEnvironment, session: ObserverSession, issueId: string): Promise<CollaborationIssue> {
  const response = await request(env, session.token, "GET", issuePath(issueId));
  if (response.status !== 200 || parseIssue(response.body) === null) throw new Error(`The authoritative issue read failed (${response.status})`);
  // Used only to build version-bound request bodies the way the client does; evidence fields are checked separately.
  return response.body as CollaborationIssue;
}

async function resumeExecution(env: PerformanceEnvironment, owner: ObserverSession, issueId: string, targetId: string): Promise<Json[]> {
  const receipts: Json[] = [];
  for (const scope of ["issue", "target"] as const) {
    const issue = await authoritativeIssue(env, owner, issueId);
    const execution = issue.execution;
    if (!execution) throw new Error("The issue has no execution projection");
    const target = execution.targets.find((item) => item.id === targetId);
    if (!target) throw new Error("The adopted target is not visible");
    if (scope === "target" && issue.session.queue_state === "active" && target.control.queue_state === "active") break;
    const targets = execution.targets.filter((item) => scope === "issue" || item.id === targetId).map((item) => ({ target_id: item.id,
      expected_target_epoch: item.target_epoch, expected_config_version: item.config_version, expected_control_version: item.control.control_version,
      observed_group_id: item.active_group_id, observed_group_generation: item.group_generation, expected_hold_version: item.hold_version }));
    const body = { ...hostAuthority(issue), action: "resume", scope, expected_execution_epoch: execution.execution_gate.epoch, targets,
      ...(scope === "issue" ? { expected_control_version: execution.control.control_version, expected_target_set_version: execution.target_set_version } : {}) };
    const key = randomUUID();
    const response = await request(env, owner.token, "POST", `${issueRoot(issueId)}/control`, { key, body });
    receipts.push({ scope, request_id: key, http_status: response.status, code: safeCode(child(response.body, "code")), queue_state: child(response.body, "queue_state") });
    if (response.status !== 200 || child(response.body, "queue_state") !== "active") throw new Error(`Resume (${scope}) was refused (${response.status})`);
  }
  return receipts;
}

interface Poll { converged: boolean; attempts: number; previous_read_start_ms: number | null; read_start_ms: number | null; read_end_ms: number | null; last_status: number | null }

/** Repeated authoritative reads, 25 ms apart. One write-back is one sample however many reads it took. */
async function pollUntil(deadlineAtMs: number, attempt: () => Promise<{ ok: boolean; start_ms: number; end_ms: number; status: number | null }>): Promise<Poll> {
  const poll: Poll = { converged: false, attempts: 0, previous_read_start_ms: null, read_start_ms: null, read_end_ms: null, last_status: null };
  while (monotonicMs() < deadlineAtMs) {
    const result = await attempt();
    poll.attempts += 1;
    poll.previous_read_start_ms = poll.read_start_ms;
    poll.read_start_ms = result.start_ms; poll.read_end_ms = result.end_ms; poll.last_status = result.status;
    if (result.ok) { poll.converged = result.end_ms <= deadlineAtMs; return poll; }
    await sleep(Math.max(0, 25 - (monotonicMs() - result.start_ms)));
  }
  return poll;
}

/**
 * One warm-up and 100 formal controlled runs on the same 100-participant issue,
 * with the 100 observer sockets connected. Each run yields one business-result
 * sample and one finalization sample; queue/start time is reported, not judged.
 */
export async function runWriteback(env: PerformanceEnvironment, set: SessionSet): Promise<WritebackOutcome> {
  const { writebackFormalRuns: formalRuns, writebackDeadlineMs: deadline } = PERFORMANCE_PROTOCOL;
  const samples = openLineLog(join(env.runDir, "writeback-samples.jsonl"), true);
  const runs = openLineLog(join(env.runDir, "writeback-runs.jsonl"), true);
  const { issueId, sessions } = set;
  const root = issueRoot(issueId);
  const owner = sessions[0];
  if (!owner) throw new Error("No sessions were opened");
  const startedUtc = utc();
  let aborted: string | null = null;
  let daemon: HostDaemon | undefined;
  let heartbeat: ReturnType<typeof startRuntimeHeartbeat> | undefined;
  let daemonResult: Json | null = null;
  const setup: Json = {};
  const plan: { phase: "warmup" | "formal"; run_index: number; trigger: number }[] = [{ phase: "warmup", run_index: 0, trigger: 0 },
    ...Array.from({ length: formalRuns }, (_unused, index) => ({ phase: "formal" as const, run_index: index, trigger: index }))];
  let gateBefore: GateState;
  let restoreGate: () => Promise<void>;
  try {
    gateBefore = await readGate(env);
    // The existing host fixture opens the gate for this phase only and restores the exact prior booleans.
    restoreGate = await openHostAcceptanceGate();
  } catch (error) { samples.close(); runs.close(); throw error; }
  try {
    const suffix = randomBytes(5).toString("hex");
    const workspace = await request(env, owner.token, "POST", "/api/workspaces", { body: { name: "PAY-241 S5 private execution", slug: `s5-exec-${suffix}` } });
    const workspaceId = text(child(workspace.body, "id")), workspaceSlug = text(child(workspace.body, "slug"));
    if (![200, 201].includes(workspace.status) || !workspaceId || !workspaceSlug) throw new Error(`The execution workspace could not be created (${workspace.status})`);
    owner.client.setWorkspaceId(workspaceId);
    owner.client.setWorkspaceSlug(workspaceSlug);
    const runtime = await enrollHostRuntime(owner.client, owner.user_id, workspaceId, env.apiUrl, env.deploymentId);
    Object.assign(setup, { runtime_id: runtime.runtimeId, runtime_owner_user_id: owner.user_id, liveness: RUNTIME_LIVENESS,
      managed_credential_expires_at: runtime.credential.expires_at });
    heartbeat = startRuntimeHeartbeat(runtime, owner.user_id, {
      now: monotonicMs, wallNow: Date.now,
      send: (token, body, signal) => request(env, token, "POST", "/api/daemon/heartbeat", {
        body, signal, timeoutMs: RUNTIME_LIVENESS.heartbeatRequestTimeoutMs,
      }),
      record: (row) => runs.append([{ run_id: env.manifest.run_id, ...row }]),
    });
    await heartbeat.ready;
    if (heartbeat.failure()) throw new Error(heartbeat.failure()!);
    setup.daemon_start_requested_ms = monotonicMs();
    daemon = await startHostDaemon(runtime, env.apiUrl);
    const daemonReadyAt = monotonicMs();
    setup.daemon_ready_ms = daemonReadyAt;
    const host = daemon;
    await waitForHostRuntime(owner.client, runtime.runtimeId);
    const agent = await owner.client.requestJSON<{ id: string; name: string }>("/api/agents", { method: "POST",
      body: { name: `S5 host fixture agent ${suffix}`, runtime_id: runtime.runtimeId, permission_mode: "private" } });
    const noBrowser = (): never => { throw new Error("The performance harness has no browser or installed client"); };
    const ownerActor: HostActor = { client: owner.client, id: owner.user_id, authenticateInstalled: noBrowser, authenticateWeb: async () => noBrowser() };
    const target = await adoptHostTarget(ownerActor, owner.user_id, issueId, workspaceId, "agent", agent.id, [{ role: "agent", agentId: agent.id, runtime }]);
    Object.assign(setup, { execution_workspace_id: workspaceId, runtime_id: runtime.runtimeId, agent_id: agent.id, target_id: target.targetId, execution_mode: hostMode,
      resume: await resumeExecution(env, owner, issueId, target.targetId), poller_note: "Test poller: 50 ms poll interval, capacity 1. Not a production daemon." });
    const mention = `[@${agent.name}](mention://agent/${agent.id})`;
    const commentPath = (id: string) => `${root}/comments/${id}`;
    const observeRuntime = async () => {
      const response = await request(env, owner.token, "GET", "/api/runtimes?owner=me", {
        workspaceId, timeoutMs: RUNTIME_LIVENESS.heartbeatRequestTimeoutMs,
      }).catch(() => null);
      return { ...runtimeAvailability(response?.status === 200 ? response.body : null, runtime, owner.user_id, Date.now()),
        http_status: response?.status ?? null, read_start_ms: response?.start_ms ?? null, read_end_ms: response?.end_ms ?? null, at_utc: utc() };
    };

    for (const item of plan) {
      const base = { run_id: env.manifest.run_id, phase: item.phase, run_index: item.run_index, issue_id: issueId, deadline_ms: deadline };
      let emitted = false;
      const emit = (business: Json, finalization: Json) => {
        if (emitted) return;
        emitted = true;
        samples.append([{ ...base, metric: "business", ...business, recorded_at_utc: utc() }, { ...base, metric: "finalization", ...finalization, recorded_at_utc: utc() }]);
      };
      const lost = (result: "missing" | "not_run", code: string): Json => ({ result, latency_ms: null, error_code: code });
      if (aborted !== null) { emit(lost("not_run", aborted), lost("not_run", aborted)); continue; }
      const trigger = sessions[item.trigger];
      const picked = pickWritebackReaders(item.trigger, 0, sessions.length);
      const reader = sessions[picked.reader], hinted = sessions[picked.hint];
      if (!trigger || !reader || !hinted) throw new Error("The plan names an actor that has no session");
      const record: Json = { ...base, trigger_user_id: trigger.user_id, reader_user_id: reader.user_id, hint_user_id: hinted.user_id, started_at_utc: utc() };
      const stop = (code: string) => {
        aborted = `aborted_after_${item.phase}_${item.run_index}_${code}`;
        emit({ ...lost("missing", code), ...record }, { ...lost("missing", code), ...record });
        runs.append([{ ...record, failed: code, ended_at_utc: utc() }]);
      };
      try {
        record.reconnects = await reconnectClosed(env, set);
        record.open_sockets = sessions.filter((session) => session.open).length;
        if (record.open_sockets !== PERFORMANCE_PROTOCOL.observers) { stop("observer_unavailable"); continue; }
        const observerConnections = sessions.map((session) => ({ socket_id: session.socket_id, connections: session.connections, closed_ms: session.closed_ms }));
        const disconnectedObservers = () => sessions.flatMap((session, index) => {
          const before = observerConnections[index];
          return !session.open || session.socket_id !== before?.socket_id || session.connections !== before?.connections
            || session.closed_ms !== before?.closed_ms ? [index] : [];
        });
        if (host.errors.length > 0) { stop("host_fixture_error"); continue; }
        // Trigger: this run's own account posts a mention bound to the current target versions.
        const issue = await authoritativeIssue(env, trigger, issueId);
        const current = issue.execution?.targets.find((entry) => entry.id === target.targetId);
        if (!current || issue.session.queue_state !== "active") { stop("queue_not_active"); continue; }
        // Owner read and durable log precede the trigger clock; no liveness I/O is awaited inside either measured write-back interval.
        const runtimeBefore = await observeRuntime();
        record.runtime_before_trigger = runtimeBefore;
        record.daemon_elapsed_since_ready_ms = monotonicMs() - daemonReadyAt;
        runs.append([{ ...record, kind: "runtime_before_trigger" }]);
        if (runtimeBefore.error_code) { stop(runtimeBefore.error_code); continue; }
        if (heartbeat.failure()) { stop(heartbeat.failure()!); continue; }
        if (monotonicMs() - daemonReadyAt >= RUNTIME_LIVENESS.hostHarnessLifetimeMs) { stop("host_fixture_lifetime_elapsed"); continue; }
        if (disconnectedObservers().length > 0) { stop("observer_unavailable"); continue; }
        const commandKey = randomUUID();
        const triggerStart = monotonicMs();
        const posted = await request(env, trigger.token, "POST", `${root}/comments`, { key: commandKey, body: { ...hostAuthority(issue),
          content: `${mention} S5 controlled write-back ${item.phase} ${item.run_index}`, parent_id: null, suppressed_target_ids: [], attachment_ids: [],
          target_versions: [{ target_id: current.id, expected_target_epoch: current.target_epoch, expected_config_version: current.config_version }] } });
        const postedTargets = child(posted.body, "targets");
        const queued: unknown = Array.isArray(postedTargets) ? postedTargets[0] : undefined;
        const commandId = text(child(queued, "command_id"));
        Object.assign(record, { trigger_request_id: commandKey, trigger_http_status: posted.status, trigger_code: safeCode(child(posted.body, "code")), command_id: commandId });
        if (posted.status !== 201 || child(queued, "state") !== "waiting" || !commandId) { stop("trigger_refused"); continue; }
        let visit: HostVisit | undefined;
        while (visit === undefined && monotonicMs() < triggerStart + deadline && host.errors.length === 0) {
          visit = host.visits.find((entry) => entry.inputs.some((input) => input.command_id === commandId));
          if (visit === undefined) await sleep(2);
        }
        const visitObserved = monotonicMs();
        const executionRun = visit?.identity.run_id;
        if (!visit || !executionRun || !UUID.test(executionRun) || visit.agent_id !== agent.id || visit.identity.issue_id !== issueId) {
          const runtimeAfter = await observeRuntime();
          record.runtime_after_missing_launch = runtimeAfter;
          stop(runtimeAfter.error_code ?? heartbeat.failure() ?? (host.errors.length > 0 ? "host_fixture_error"
            : monotonicMs() - daemonReadyAt >= RUNTIME_LIVENESS.hostHarnessLifetimeMs ? "host_fixture_lifetime_elapsed" : "no_provider_launch"));
          continue;
        }
        Object.assign(record, { execution_run_id: executionRun, queue_start_ms: visitObserved - triggerStart,
          queue_start_note: "Reported separately, not part of the 5 s judgement; includes the 50 ms test poller." });

        // The Provider must not have written anything yet. A fixture that writes before its callback would give a near-zero latency.
        const requestId = randomUUID();
        const content = `S5 controlled result ${item.phase} ${item.run_index} ${requestId}`;
        const [byRequest, byRun] = await Promise.all([request(env, reader.token, "GET", commentPath(requestId)), request(env, reader.token, "GET", commentPath(executionRun))]);
        let prewritten = byRequest.status !== 404 || byRun.status !== 404 || "result_comment_id" in visit;
        if (item.phase === "warmup") {
          const listed = await request(env, reader.token, "GET", `${root}/comments?limit=100`);
          const items = child(listed.body, "items");
          prewritten ||= listed.status !== 200 || !Array.isArray(items) || items.some((entry) => child(entry, "author_type") === "agent");
        }
        Object.assign(record, { request_id: requestId, content_sha256: sha256(content), precheck: { by_request_id: byRequest.status, by_run_id: byRun.status } });
        if (prewritten) { stop("provider_wrote_before_instruction"); continue; }

        // Business result: t0 precedes the instruction; the end is the first authoritative read by another participant.
        const frameCount = hinted.ledger.arrivals.size;
        const t0 = monotonicMs();
        const acting = host.act(visit, [{ kind: "comment", request_id: requestId, content }]).then((receipts) => ({ receipts, at_ms: monotonicMs() }), () => null);
        const matches = (body: unknown) => String(child(body, "id")).toLowerCase() === requestId && String(child(body, "issue_id")).toLowerCase() === issueId
          && child(body, "content") === content && child(body, "author_type") === "agent"
          && String(child(child(body, "author_agent"), "agent_id")).toLowerCase() === agent.id.toLowerCase();
        let hintClosed = false;
        const hintPath = (async (): Promise<number | null> => {
          // Secondary: from t0 to a change frame followed by this observer's own read of the comment.
          let seen = frameCount;
          while (await waitUntil(hinted, t0 + deadline, () => hintClosed || hinted.ledger.arrivals.size > seen)) {
            if (hinted.ledger.arrivals.size === seen) return null;
            seen = hinted.ledger.arrivals.size;
            const response = await request(env, hinted.token, "GET", commentPath(requestId)).catch(() => null);
            if (response?.status === 200 && matches(response.body)) return response.end_ms - t0;
          }
          return null;
        })();
        const business = await pollUntil(t0 + deadline, async () => {
          const response = await request(env, reader.token, "GET", commentPath(requestId)).catch(() => null);
          return { ok: response?.status === 200 && matches(response.body), start_ms: response?.start_ms ?? monotonicMs(), end_ms: response?.end_ms ?? monotonicMs(), status: response?.status ?? null };
        });
        const acted = await acting;
        const receiptMatches = acted?.receipts.length === 1 && acted.receipts[0]?.request_id === requestId && !acted.receipts[0]?.error;
        if (!acted || !receiptMatches) {
          hintClosed = true;
          for (const wake of [...hinted.waiters]) wake();
          emit({ ...lost("missing", "provider_action_failed"), ...record, t0_ms: t0 }, { ...lost("missing", "provider_action_failed"), ...record });
          runs.append([{ ...record, failed: "provider_action_failed", ended_at_utc: utc() }]);
          aborted = `aborted_after_${item.phase}_${item.run_index}_provider_action_failed`;
          continue;
        }

        // Finalization: t1 precedes release; the end is one round of reads showing the settled run and its recorded result.
        const t1 = monotonicMs();
        host.release(visit);
        let attributed = false;
        const finalization = await pollUntil(t1 + deadline, async () => {
          const [detail, records] = await Promise.all([request(env, reader.token, "GET", issuePath(issueId)).catch(() => null),
            request(env, reader.token, "GET", versioned(`${root}/runs/${executionRun}/records?limit=100`)).catch(() => null)]);
          const listed = child(child(detail?.body, "execution"), "runs"), items = child(records?.body, "items");
          const run = Array.isArray(listed) ? listed.find((entry) => child(entry, "id") === executionRun) : undefined;
          const settled = child(run, "progress_state") === "settled" && child(run, "proof_state") === "host_exit_observed"
            && child(run, "launch_count") === "1" && child(run, "business_outcome") === "success";
          const resulted = Array.isArray(items) && items.some((entry) => child(entry, "kind") === "result" && child(entry, "body") === `Host fixture result for ${executionRun}`);
          attributed = Array.isArray(items) && items.some((entry) => child(entry, "kind") === "comment" && child(entry, "comment_id") === requestId);
          return { ok: detail?.status === 200 && records?.status === 200 && settled && resulted, start_ms: Math.min(detail?.start_ms ?? t1, records?.start_ms ?? t1),
            end_ms: Math.max(detail?.end_ms ?? monotonicMs(), records?.end_ms ?? monotonicMs()), status: detail?.status ?? null };
        });
        hintClosed = true;
        for (const wake of [...hinted.waiters]) wake();
        const hintLatency = await hintPath;
        const disconnected = disconnectedObservers();
        Object.assign(record, { disconnected_observer_indices: disconnected, open_sockets_after: sessions.filter((session) => session.open).length });
        const row = (poll: Poll, start: number, converged: boolean, code: string | null): Json => ({ ...record, t0_ms: start, read_attempts: poll.attempts,
          previous_read_start_ms: poll.previous_read_start_ms, read_start_ms: poll.read_start_ms, read_end_ms: poll.read_end_ms, last_http_status: poll.last_status,
          observed_latency_ms: poll.converged && poll.read_end_ms !== null ? poll.read_end_ms - start : null,
          result: converged ? "converged" : "missing", latency_ms: converged && poll.read_end_ms !== null ? poll.read_end_ms - start : null, error_code: converged ? null : code });
        // The comment must belong to this run in the run's own records, or the business sample does not count.
        const businessOk = business.converged && attributed && disconnected.length === 0;
        const finalizationOk = finalization.converged && disconnected.length === 0;
        emit({ ...row(business, t0, businessOk, disconnected.length > 0 ? "observer_disconnected"
          : business.converged ? "comment_not_attributed_to_run" : "comment_not_visible"),
          provider_receipt_ms: acted.at_ms, comment_id: requestId, hint_path_latency_ms: hintLatency },
        row(finalization, t1, finalizationOk, disconnected.length > 0 ? "observer_disconnected" : "run_not_settled"));
        runs.append([{ ...record, business_converged: businessOk, finalization_converged: finalizationOk, comment_attributed_to_run: attributed,
          hint_path_latency_ms: hintLatency, ended_at_utc: utc() }]);
        // A run that did not settle would overlap the next one on a capacity-one runtime.
        if (disconnected.length > 0) aborted = `aborted_after_${item.phase}_${item.run_index}_observer_disconnected`;
        else if (!finalization.converged) aborted = `aborted_after_${item.phase}_${item.run_index}_run_not_settled`;
      } catch (error) {
        stop(`run_error_${error instanceof Error ? error.name.toLowerCase().replace(/[^a-z0-9_]/g, "") : "unknown"}`);
      }
    }
  } catch (error) {
    const reason = error instanceof Error && /^runtime_heartbeat_[a-z0-9_]+$/.test(error.message) ? error.message : "writeback_setup_or_harness_failed";
    aborted ??= reason;
    runs.append([{ kind: "harness_failure", error_code: reason, at_utc: utc() }]);
  } finally {
    // Cleanup failures are kept in the summary and fail the test; they never hide the samples.
    const failures: string[] = [];
    await set.close().catch(() => { failures.push("socket_close_failed"); });
    if (daemon) {
      await daemon.close().then((result) => { daemonResult = { exit_code: result.exit_code, errors: result.errors.length, ledger_runs: result.ledger.length,
        ledger_phases: [...new Set(result.ledger.map((entry) => String(entry.phase)))] };
        if (result.exit_code !== 0 || result.errors.length > 0) failures.push("daemon_close_unclean");
      }, () => { failures.push("daemon_close_failed"); });
    }
    // Keep liveness through run settlement, then settle cancellation before restoring the gate or closing its log.
    if (heartbeat) {
      await heartbeat.stop();
      if (heartbeat.failure()) failures.push(heartbeat.failure()!);
      setup.heartbeat = { stopped: true, stopped_at_utc: utc(), stopped_ms: monotonicMs(), failure: heartbeat.failure() };
    }
    await restoreGate().catch(() => { failures.push("gate_restore_failed"); });
    const gateAfter = await readGate(env).catch(() => null);
    const restored = gateAfter !== null && gateAfter.enabled === gateBefore.enabled && gateAfter.creation_enabled === gateBefore.creation_enabled
      && gateAfter.host_execution_enabled === gateBefore.host_execution_enabled && BigInt(gateAfter.epoch) >= BigInt(gateBefore.epoch);
    if (!restored) failures.push("gate_not_restored");
    Object.assign(setup, { gate: { before: gateBefore, after: gateAfter, restored, allocation_original: env.manifest.gate_original }, cleanup_failures: failures, daemon: daemonResult });
    // A setup/heartbeat failure still owes every planned sample. Keep successful rows and fill only absent pairs.
    const emitted = new Set(readLines(samples.path).map((row) => `${row.phase}:${row.run_index}:${row.metric}`));
    for (const item of plan) for (const metric of ["business", "finalization"]) {
      if (!emitted.has(`${item.phase}:${item.run_index}:${metric}`)) samples.append([{ run_id: env.manifest.run_id, phase: item.phase,
        run_index: item.run_index, issue_id: issueId, metric, deadline_ms: deadline, result: "not_run", latency_ms: null,
        error_code: aborted ?? "harness_did_not_emit_sample", recorded_at_utc: utc() }]);
    }
    samples.close(); runs.close();
  }

  const raw = readLines(samples.path);
  const accounting = accountWriteback(raw as unknown as WritebackRow[]);
  const verdict = writebackVerdict(accounting);
  const formal = raw.filter((row) => row.phase === "formal");
  const informational = (values: unknown[]): QuantileReport | null => {
    const numbers = values.filter((value): value is number => typeof value === "number" && Number.isFinite(value));
    return numbers.length === 0 ? null : reportQuantiles(summarize(numbers));
  };
  const gate = (name: "business" | "finalization") => ({ pass: verdict[name].pass, budget_ms: verdict[name].budget_ms, denominator: verdict[name].denominator,
    rows_in_file: accounting[name].rows, converged: accounting[name].converged, not_run: accounting[name].not_run,
    filled_not_run: accounting[name].filled_not_run, over_deadline_reclassified: accounting[name].over_deadline_reclassified, ...reportQuantiles(verdict[name].summary) });
  const cleanupFailures = Array.isArray(setup.cleanup_failures) ? setup.cleanup_failures : ["cleanup_not_recorded"];
  const pass = verdict.pass && cleanupFailures.length === 0;
  const summary: Json = {
    run_id: env.manifest.run_id, purpose: env.manifest.purpose, source: env.manifest.source, manifest_sha256: env.manifestSha256,
    started_at_utc: startedUtc, ended_at_utc: utc(), issue_id: issueId,
    scope: "Deterministic host Provider fixture through the production poller path and run-local CLI, on the 100-participant issue with 100 sockets connected. "
      + "Controlled finalization convergence, not model reasoning, model queueing or a production daemon.",
    protocol: PERFORMANCE_PROTOCOL, aborted, setup, pass,
    business_result: gate("business"), finalization: gate("finalization"), warmup_rows_kept_and_excluded: accounting.warmup_rows,
    queue_start_informational: informational(formal.filter((row) => row.metric === "business").map((row) => row.queue_start_ms)),
    hint_path_informational: informational(formal.filter((row) => row.metric === "business").map((row) => row.hint_path_latency_ms)),
    provider: { source_sha256: env.manifest.binaries?.provider_source_sha256 ?? null, binary_sha256: env.manifest.binaries?.provider_sha256 ?? null,
      daemon_sha256: env.manifest.binaries?.daemon_sha256 ?? null, cli_sha256: env.manifest.binaries?.cli_sha256 ?? null },
    samples_file: { name: "writeback-samples.jsonl", lines: raw.length, sha256: sha256File(samples.path) },
    runs_file: { name: "writeback-runs.jsonl", sha256: sha256File(runs.path) },
  };
  writeExclusive(join(env.runDir, "summary-writeback.json"), summary);
  writeExclusive(join(env.runDir, "failures-writeback.json"), { run_id: env.manifest.run_id, aborted, cleanup_failures: cleanupFailures,
    non_converged: raw.filter((row) => row.result !== "converged") });
  return { summary, pass, aborted };
}

// ------------------------------------------------------------- phase shell

/**
 * Identity, resources and gate recorded around one phase. A sample file from an
 * earlier attempt makes the phase refuse: a new complete run needs a new number.
 * An attempt that never reached its first sample (NOT_RUN) may be repeated here.
 */
export async function withPhaseEvidence<T>(env: PerformanceEnvironment, harnessFiles: string[], body: () => Promise<T>): Promise<T> {
  for (const name of [`${env.phase}-samples.jsonl`, `summary-${env.phase}.json`]) {
    if (existsSync(join(env.runDir, name))) {
      throw new Error(`Performance harness refused: ${name} already exists. A measurement that started is kept as it is; use a new run directory and number.`);
    }
  }
  const live = await verifyLiveEnvironment(env);
  const eventLoop = monitorEventLoopDelay({ resolution: 10 });
  eventLoop.enable();
  // One set of identity and resource files per attempt, so a setup failure that took no sample is also kept.
  const stamp = `-${utc().replace(/[-:.]/g, "")}`;
  const gateBefore = await readGate(env);
  writeExclusive(join(env.runDir, `observed-${env.phase}${stamp}.json`), { run_id: env.manifest.run_id, phase: env.phase, at_utc: utc(), manifest_sha256: env.manifestSha256,
    live, protocol: PERFORMANCE_PROTOCOL, harness_sha256: Object.fromEntries(harnessFiles.map((file) => [file.split(sep).slice(-2).join("/"), sha256File(file)])),
    gate_before: gateBefore, gate_allocation_original: env.manifest.gate_original });
  writeExclusive(join(env.runDir, `resources-${env.phase}${stamp}-before.json`), await captureResources(env, null));
  let failure: unknown;
  let failed = false;
  let result: T | undefined;
  try { result = await body(); } catch (error) { failed = true; failure = error; }
  eventLoop.disable();
  const gateAfter = await readGate(env).catch(() => null);
  const resources = await captureResources(env, eventLoop).catch(() => ({ error: "resource snapshot failed" }));
  // Prepare and propagation never touch the gate; write-back must leave the same booleans with a monotonic epoch.
  const unchanged = gateAfter !== null && gateAfter.enabled === gateBefore.enabled && gateAfter.creation_enabled === gateBefore.creation_enabled
    && gateAfter.host_execution_enabled === gateBefore.host_execution_enabled
    && (env.phase === "writeback" ? BigInt(gateAfter.epoch) >= BigInt(gateBefore.epoch) : gateAfter.epoch === gateBefore.epoch);
  writeExclusive(join(env.runDir, `resources-${env.phase}${stamp}-after.json`), { ...resources, gate_before: gateBefore, gate_after: gateAfter, gate_unchanged: unchanged });
  if (failed) throw failure;
  if (!unchanged) throw new Error("The collaboration execution gate differs from its state before this phase");
  return result as T;
}
