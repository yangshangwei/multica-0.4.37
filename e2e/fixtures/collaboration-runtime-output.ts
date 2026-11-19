import { execFile } from "node:child_process";
import { createHash, randomUUID } from "node:crypto";
import { existsSync, lstatSync, readFileSync, readdirSync, realpathSync, rmSync } from "node:fs";
import { createServer as createHTTPServer, request as httpRequest, type IncomingHttpHeaders } from "node:http";
import { createServer as createTCPServer, type Socket } from "node:net";
import { isAbsolute, join, relative } from "node:path";
import { promisify } from "node:util";
import { expect, type Browser, type Locator, type TestInfo } from "@playwright/test";
import pg from "pg";
import { z } from "zod";
import {
  CollaborationAttachmentSchema, CollaborationAttachmentsSchema, CollaborationCommentSchema,
  CollaborationCommandHistorySchema, CollaborationControlReceiptSchema, CollaborationOperationSchema, CollaborationResourceReceiptSchema,
  CollaborationResourcesSchema, parseCollaborationAttachmentCapability,
} from "@multica/core/collaboration";
import {
  createHostActor, enrollHostRuntime, hostEnvironment, hostMode, issueRoot, openHostAcceptanceGate,
  startHostDaemon, versioned, waitForHostRuntime, type HostAction, type HostIssue, type HostVisit,
} from "./collaboration-host-runtime";
import {
  RecoveryEvidence, authenticatedSocket, withSessions, type RecoveryActor, type RecoveryPlatform, type RecoverySession,
} from "./collaboration-recovery";
import {
  actorFor, assertVisit, audit, control, createRuntimeIssue, inventory, notifications, originalInputReceipt,
  readIssue, records, recoverOriginal, recoveryRuntimeEnvironmentReady, refusal, runtimeTransport, safeError,
  submitInput, surface, waitSettlement, wireJSON, type OriginalInput, type RuntimeFixture, type WireReply,
} from "./collaboration-recovery-runtime";

const object = z.record(z.string(), z.unknown());
const sha = (bytes: string | Buffer) => createHash("sha256").update(bytes).digest("hex");
const digest = (value: unknown) => sha(JSON.stringify(value));
const now = () => new Date().toISOString();
const execute = promisify(execFile);
const positive = z.string().regex(/^[1-9][0-9]*$/);
const hash = z.string().regex(/^[a-f0-9]{64}$/);
const stateRoot = "/Users/artisan/.multica/acceptance/pay241-s3-output-20261119-tm89sjgy";
const outputCases = [
  "agent_file_unknown_response_original_cli_retry", "same_key_same_bytes_concurrent_replay",
  "changed_bytes_metadata_retry_and_mixed_race", "failed_result_frozen_files_and_both_shared_renderers",
  "explicit_new_success_preserves_old_failure", "explicit_stop_preserves_output_and_denies_late_token",
  "author_withdrawal_denies_original_output_token", "grant_withdrawal_denies_original_output_token",
] as const;

const agentAttachment = z.object({
  id: z.uuid(), draft_id: z.null(), issue_id: z.uuid(), intended_issue_id: z.null(), source_workspace_id: z.uuid(),
  uploader_user_id: z.null(), uploader_agent_id: z.uuid(), run_id: z.uuid(), attempt_seq: positive,
  name: z.string().min(1), content_type: z.string().min(1), size_bytes: z.number().int().nonnegative(),
  version: positive, created_at: z.iso.datetime({ offset: true }), delivered_at: z.iso.datetime({ offset: true }),
});
const outputReceipt = z.strictObject({ comment_id: z.uuid(), run_ids: z.array(z.uuid()), continuation_run_id: z.uuid().nullable(), attachment: agentAttachment });
type OutputReceipt = z.infer<typeof outputReceipt>;
const actionReceipt = z.object({ kind: z.literal("comment"), request_id: z.uuid(), mode: z.literal("agent_output_v1"),
  file: z.object({ name: z.string(), sha256: hash, size_bytes: z.number().int(), regular_file: z.literal(true), generated: z.boolean(), cwd_sha256: hash }),
  cli: z.object({ success: z.boolean(), exit_code: z.number().int(), stdout_sha256: hash, stdout_bytes: z.number().int().nonnegative(),
    stderr_sha256: hash, stderr_bytes: z.number().int().nonnegative() }), result: outputReceipt.optional(),
});
const processObservation = z.strictObject({
  mode: z.literal("agent_output_v1"), issue_id: z.uuid(), run_id: z.uuid(), request_id: z.uuid(),
  provider_pid: z.number().int().positive(), provider_parent_pid: z.number().int().positive(),
  cli_pid: z.number().int().positive(), cli_parent_pid: z.number().int().positive(), provider_sha256: hash, cli_sha256: hash,
  run_context_sha256: hash, business_token_sha256: hash, cwd_sha256: hash,
  file_name: z.string().min(1), file_sha256: hash, file_bytes: z.number().int().nonnegative(), regular_file: z.literal(true),
  command: z.array(z.string()).max(16), phase: z.enum(["started", "completed"]), captured_at: z.iso.datetime({ offset: true }),
  exit_code: z.number().int().optional(),
});
type ProcessObservation = z.infer<typeof processObservation> & { observed_at: string };

export function runtimeOutputEnvironmentReady() {
  return recoveryRuntimeEnvironmentReady() && process.env.PAY241_S3_OUTPUT_ACCEPTANCE === "1" &&
    Boolean(process.env.COLLABORATION_OUTPUT_STORAGE_ROOT);
}

async function processObserver(evidence: RecoveryEvidence) {
  const sockets = new Set<Socket>(), errors: string[] = [], facts: ProcessObservation[] = [];
  const server = createTCPServer((socket) => {
    sockets.add(socket); socket.setEncoding("utf8"); socket.setTimeout(5_000, () => socket.destroy());
    let body = "", received = false;
    socket.on("error", () => errors.push("Output process observation socket failed"));
    socket.once("close", () => sockets.delete(socket));
    socket.on("data", (chunk: string) => {
      body += chunk;
      if (body.length > 16 * 1024) { errors.push("Output process observation exceeded its bound"); socket.destroy(); return; }
      const end = body.indexOf("\n");
      if (end < 0) return;
      try {
        if (received || body.slice(end + 1).trim()) throw new Error("Process observation was not one frame");
        received = true;
        const fact = { ...processObservation.parse(JSON.parse(body.slice(0, end))), observed_at: now() };
        expect(fact.cli_parent_pid).toBe(fact.provider_pid);
        expect(fact.cli_pid).not.toBe(fact.provider_pid);
        expect(fact.command).toEqual(["issue", "comment", "add", fact.issue_id, "--content", "[test-owned content]", "--request-id", fact.request_id,
          "--attachment", fact.file_name, "--output", "json"]);
        facts.push(fact);
        evidence.checkpoint(`actual_output_cli_${fact.phase}`, fact);
        socket.end();
      } catch { errors.push("Output process observation was invalid"); socket.destroy(); }
    });
  });
  await new Promise<void>((resolve, reject) => { server.once("error", reject); server.listen(0, "127.0.0.1", resolve); });
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("The output process observer did not bind");
  return {
    address: `127.0.0.1:${address.port}`,
    async invocation(runId: string, requestId: string, ordinal: number) {
      await expect.poll(() => facts.filter((fact) => fact.run_id === runId && fact.request_id === requestId && fact.phase === "completed").length,
        { timeout: 30_000 }).toBe(ordinal);
      expect(errors).toEqual([]);
      const completed = facts.filter((fact) => fact.run_id === runId && fact.request_id === requestId && fact.phase === "completed")[ordinal - 1]!;
      const started = facts.find((fact) => fact.cli_pid === completed.cli_pid && fact.phase === "started");
      expect(started).toBeDefined();
      if (!started) throw new Error("The original output CLI start observation was not retained");
      expect(started).not.toHaveProperty("exit_code");
      expect(Date.parse(started.captured_at)).toBeLessThanOrEqual(Date.parse(completed.captured_at));
      for (const field of ["provider_pid", "provider_parent_pid", "cli_parent_pid", "provider_sha256", "cli_sha256", "business_token_sha256",
        "run_context_sha256", "file_name", "file_sha256", "file_bytes", "cwd_sha256"] as const) expect(completed[field]).toBe(started[field]);
      return { started, completed };
    },
    evidence: () => ({ facts: structuredClone(facts), errors: [...errors] }),
    async close() {
      for (const socket of sockets) socket.destroy();
      await new Promise<void>((resolve) => server.close(() => resolve()));
    },
  };
}

interface MultipartPart { headers: string; name: string; filename: string | null; data: Buffer }
interface MultipartInput { boundary: string; parts: MultipartPart[]; comment_id: string; content: string; parent_id: string | null; file: MultipartPart }
function multipart(bytes: Buffer, contentType: string): MultipartInput {
  const boundary = /(?:^|;)\s*boundary=(?:"([^"]+)"|([^;\s]+))/.exec(contentType)?.slice(1).find(Boolean);
  if (!contentType.startsWith("multipart/form-data;") || !boundary || boundary.length > 100) throw new Error("The actual CLI request was not bounded multipart");
  const frames = bytes.toString("latin1").split(`--${boundary}`);
  if (frames.shift() !== "" || frames.pop() !== "--\r\n") throw new Error("The actual CLI multipart framing was invalid");
  const parts = frames.map((frame) => {
    if (!frame.startsWith("\r\n") || !frame.endsWith("\r\n")) throw new Error("The actual CLI multipart part was invalid");
    const split = frame.indexOf("\r\n\r\n"), headers = frame.slice(2, split);
    if (split < 2 || headers.length > 8192) throw new Error("The actual CLI multipart headers were invalid");
    const disposition = /^Content-Disposition: form-data; name="([^"]+)"(?:; filename="([^"]+)")?$/im.exec(headers);
    if (!disposition?.[1]) throw new Error("The actual CLI multipart disposition was invalid");
    return { headers, name: disposition[1], filename: disposition[2] ?? null, data: Buffer.from(frame.slice(split + 4, -2), "latin1") };
  });
  if (parts.length < 3 || parts.length > 4 || new Set(parts.map((part) => part.name)).size !== parts.length) throw new Error("The actual CLI multipart field inventory was invalid");
  const files = parts.filter((part) => part.filename !== null);
  if (files.length !== 1 || files[0]!.name !== "file" || parts.some((part) => !["comment_id", "content", "parent_id", "file"].includes(part.name))) {
    throw new Error("The actual CLI multipart was not one file and its original comment fields");
  }
  const field = (name: string) => parts.find((part) => part.name === name)?.data.toString("utf8");
  const commentId = z.uuid().parse(field("comment_id")), content = z.string().min(1).parse(field("content"));
  const parent = field("parent_id");
  return { boundary, parts, comment_id: commentId, content, parent_id: parent === undefined ? null : z.uuid().parse(parent), file: files[0]! };
}
function multipartFact(input: MultipartInput) {
  return { comment_id: input.comment_id, content_sha256: sha(input.content), parent_id: input.parent_id,
    file_name: input.file.filename!, file_sha256: sha(input.file.data), file_bytes: input.file.data.length,
    declared_type: /^Content-Type:\s*(.+)$/im.exec(input.file.headers)?.[1] ?? null, field_names: input.parts.map((part) => part.name) };
}
interface OriginalOutputWire { path: string; authorization: string; headers: IncomingHttpHeaders; bytes: Buffer; input: MultipartInput; winner?: WireReply }
interface OutputWireFact {
  at: string; source: "GENUINE_CLI" | "SAME_AGENT_TRANSPORT_REPLAY"; run_id: string; request_id: string;
  request_sha256: string; authorization_sha256: string; business_token_sha256: string; input: ReturnType<typeof multipartFact>;
  race_id: string | null; forward_started_at: string | null; status: number | null; response_sha256: string | null; acknowledgement_lost: boolean;
}

/** Only output traffic is observed here; the existing transport owns permits and proof recovery. */
async function outputTransport(apiUrl: string, evidence: RecoveryEvidence) {
  const origin = new URL(apiUrl), sockets = new Set<Socket>(), errors: string[] = [], facts: OutputWireFact[] = [];
  const passthroughFailures: { at: string; path: string; error_type: string }[] = [];
  if (origin.protocol !== "http:" || !["localhost", "127.0.0.1"].includes(origin.hostname)) throw new Error("Output observation requires the owned loopback API");
  const originals = new Map<string, OriginalOutputWire>(), drop = new Set<string>(), replayDigests = new Map<string, number>();
  const races = new Map<string, { id: string; arrivals: number; wait: Promise<void>; release(): void }>();
  let closing = false;
  const key = (runId: string, requestId: string) => `${runId}:${requestId}`;
  const call = (url: URL, method: string, headers: IncomingHttpHeaders, bytes: Buffer): Promise<WireReply> => new Promise((resolve, reject) => {
    const forwarded: IncomingHttpHeaders = { ...headers, host: url.host, "content-length": String(bytes.length) };
    delete forwarded["transfer-encoding"];
    const request = httpRequest(url, { method, agent: false, headers: forwarded, timeout: 15_000 }, (response) => {
      const chunks: Buffer[] = []; let size = 0;
      response.on("data", (chunk: Buffer) => {
        size += chunk.length;
        if (size > 4 * 1024 * 1024) { response.destroy(new Error("Output response exceeded its bound")); return; }
        chunks.push(chunk);
      });
      response.once("end", () => resolve({ status: response.statusCode ?? 0, headers: response.headers, bytes: Buffer.concat(chunks) }));
      response.once("error", reject); response.once("aborted", () => reject(new Error("Output response aborted")));
    });
    request.once("error", reject); request.once("timeout", () => request.destroy(new Error("Output request timed out"))); request.end(bytes);
  });
  const server = createHTTPServer(async (request, response) => {
    let fact: OutputWireFact | undefined;
    try {
      const path = request.url ?? "/", method = request.method ?? "GET", match = /^\/api\/collaboration\/runtime\/runs\/([a-f0-9-]{36})\/comments\/attachments$/.exec(new URL(path, origin).pathname);
      const chunks: Buffer[] = []; let size = 0;
      for await (const chunk of request) {
        const bytes = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk); size += bytes.length;
        if (size > 2 * 1024 * 1024) throw new Error("Output request exceeded the controlled fixture bound"); chunks.push(bytes);
      }
      const bytes = Buffer.concat(chunks);
      let original: OriginalOutputWire | undefined;
      if (match?.[1] && method === "POST") {
        const runId = z.uuid().parse(match[1]), authorization = z.string().regex(/^Bearer mcb_[a-f0-9]{64}$/).parse(request.headers.authorization);
        const input = multipart(bytes, z.string().parse(request.headers["content-type"]));
        const identity = key(runId, input.comment_id), requestHash = sha(bytes), replays = replayDigests.get(requestHash) ?? 0;
        if (replays) replayDigests.set(requestHash, replays - 1);
        fact = { at: now(), source: replays ? "SAME_AGENT_TRANSPORT_REPLAY" : "GENUINE_CLI", run_id: runId, request_id: input.comment_id,
          request_sha256: requestHash, authorization_sha256: sha(authorization), business_token_sha256: sha(authorization.slice("Bearer ".length)),
          input: multipartFact(input), race_id: null, forward_started_at: null, status: null, response_sha256: null, acknowledgement_lost: false };
        facts.push(fact);
        original = originals.get(identity);
        if (!original) { original = { path, authorization, headers: { ...request.headers }, bytes, input }; originals.set(identity, original); }
        const race = races.get(identity);
        if (race) {
          fact.race_id = race.id; race.arrivals++;
          if (race.arrivals === 2) race.release();
          if (race.arrivals > 2) throw new Error("The output race received an unexpected third request");
          await race.wait;
        }
      }
      if (response.destroyed) return;
      if (fact) fact.forward_started_at = now();
      const upstream = await call(new URL(path, origin), method, request.headers, bytes);
      if (fact) {
        fact.status = upstream.status; fact.response_sha256 = sha(upstream.bytes);
        if (upstream.status === 201 && original) original.winner ??= upstream;
        if (upstream.status === 201 && drop.delete(key(fact.run_id, fact.request_id))) {
          fact.acknowledgement_lost = true;
          evidence.checkpoint("actual_agent_output_committed_reply_dropped_before_cli_recovery", { ...fact,
            receipt: outputReceipt.parse(wireJSON(upstream)), response_bytes: upstream.bytes.length });
          response.destroy(); return;
        }
      }
      response.writeHead(upstream.status, upstream.headers); response.end(upstream.bytes);
    } catch (error) {
      if (!closing && !response.destroyed) {
        if (fact) errors.push(`Output forwarding failed: ${safeError(error).type}`);
        else passthroughFailures.push({ at: now(), path: new URL(request.url ?? "/", origin).pathname, error_type: safeError(error).type });
      }
      response.destroy();
    }
  });
  server.on("connection", (socket) => { sockets.add(socket); socket.once("close", () => sockets.delete(socket)); });
  await new Promise<void>((resolve, reject) => { server.once("error", reject); server.listen(0, "127.0.0.1", resolve); });
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("The output transport did not bind");
  const proxyUrl = `http://127.0.0.1:${address.port}`;
  const originalFor = (runId: string, requestId: string) => {
    const original = originals.get(key(runId, requestId));
    if (!original) throw new Error("No genuine original Agent multipart request was observed"); return original;
  };
  const rewritten = (original: OriginalOutputWire, change: { bytes?: Buffer; name?: string; requestId?: string }) => {
    if (!change.bytes && !change.name && !change.requestId) return original.bytes;
    return Buffer.concat([...original.input.parts.flatMap((part) => {
      const headers = part === original.input.file && change.name ? part.headers.replace(/filename="[^"]+"/, `filename="${change.name}"`) : part.headers;
      return [Buffer.from(`--${original.input.boundary}\r\n${headers}\r\n\r\n`, "latin1"),
        part.name === "comment_id" && change.requestId ? Buffer.from(z.uuid().parse(change.requestId))
          : part === original.input.file && change.bytes ? change.bytes : part.data, Buffer.from("\r\n")];
    }), Buffer.from(`--${original.input.boundary}--\r\n`)]);
  };
  const replay = async (runId: string, requestId: string, change: { bytes?: Buffer; name?: string; requestId?: string } = {}) => {
    const original = originalFor(runId, requestId), bytes = rewritten(original, change), requestHash = sha(bytes);
    replayDigests.set(requestHash, (replayDigests.get(requestHash) ?? 0) + 1);
    const reply = await call(new URL(original.path, proxyUrl), "POST", original.headers, bytes);
    return { reply, identity: { run_id: runId, request_id: change.requestId ?? requestId, request_sha256: requestHash,
      business_token_sha256: sha(original.authorization.slice("Bearer ".length)), status: reply.status, response_sha256: sha(reply.bytes) } };
  };
  return {
    apiUrl: proxyUrl,
    dropOriginalReply(runId: string, requestId: string) { drop.add(key(runId, requestId)); },
    original(runId: string, requestId: string) {
      const original = originalFor(runId, requestId);
      if (!original.winner) throw new Error("The actual original Agent output was not committed");
      return { input: multipartFact(original.input), request_sha256: sha(original.bytes), receipt: outputReceipt.parse(wireJSON(original.winner)),
        authorization_sha256: sha(original.authorization), business_token_sha256: sha(original.authorization.slice("Bearer ".length)) };
    },
    replay,
    async humanWrite(runId: string, requestId: string, humanToken: string) {
      const original = originalFor(runId, requestId);
      return call(new URL(original.path, origin), "POST", { ...original.headers, authorization: `Bearer ${humanToken}` }, original.bytes);
    },
    async concurrent(runId: string, requestId: string, changedBytes?: Buffer) {
      const identity = key(runId, requestId);
      if (races.has(identity)) throw new Error("An output race is already active");
      let release!: () => void;
      const wait = new Promise<void>((resolve) => { release = resolve; }), race = { id: randomUUID(), arrivals: 0, wait, release };
      races.set(identity, race);
      const timer = setTimeout(release, 10_000);
      try {
        const results = await Promise.allSettled([replay(runId, requestId), replay(runId, requestId, changedBytes ? { bytes: changedBytes } : {})]);
        expect(race.arrivals).toBe(2);
        for (const result of results) if (result.status === "rejected") throw result.reason;
        expect(errors).toEqual([]);
        const completed = facts.filter((fact) => fact.race_id === race.id);
        expect(completed).toHaveLength(2); expect(completed.every((fact) => fact.forward_started_at !== null)).toBe(true);
        expect(Date.parse(completed[0]!.forward_started_at!)).toBeGreaterThanOrEqual(Date.parse(completed[1]!.at));
        expect(Date.parse(completed[1]!.forward_started_at!)).toBeGreaterThanOrEqual(Date.parse(completed[0]!.at));
        evidence.checkpoint("actual_original_agent_multipart_pair_barrier", { race_id: race.id, facts: completed });
        return results.map((result) => { if (result.status === "rejected") throw result.reason; return result.value; });
      } finally { clearTimeout(timer); release(); races.delete(identity); }
    },
    facts: (runId: string, requestId: string) => structuredClone(facts.filter((fact) => fact.run_id === runId && fact.request_id === requestId)),
    evidence: () => ({ api_url: proxyUrl, facts: structuredClone(facts), errors: [...errors], passthrough_failures: structuredClone(passthroughFailures) }),
    releaseAll() { for (const race of races.values()) race.release(); races.clear(); drop.clear(); },
    async close() {
      closing = true; for (const race of races.values()) race.release(); races.clear(); drop.clear();
      for (const socket of sockets) socket.destroy();
      await new Promise<void>((resolve) => server.close(() => resolve())); originals.clear(); replayDigests.clear();
    },
  };
}

interface GeneratedOutput {
  issue: HostIssue; runId: string; requestId: string; content: string; name: string; bytes: Buffer;
  receipt: OutputReceipt; invocation: Awaited<ReturnType<Awaited<ReturnType<typeof processObserver>>["invocation"]>>;
}
interface OutputFixture extends RuntimeFixture {
  output: Awaited<ReturnType<typeof outputTransport>>;
  processes: Awaited<ReturnType<typeof processObserver>>;
  generated: GeneratedOutput[];
}

async function readDatabase<T>(action: (database: pg.Client) => Promise<T>): Promise<T> {
  const database = new pg.Client(process.env.DATABASE_URL!);
  await database.connect();
  try {
    await database.query("BEGIN READ ONLY"); await database.query("SET LOCAL statement_timeout='5s'");
    const current = await database.query<{ database: string }>("SELECT current_database() AS database");
    expect(current.rows[0]?.database).toBe(process.env.TASK_DATABASE_NAME);
    const value = await action(database); await database.query("COMMIT"); return value;
  } catch (error) { await database.query("ROLLBACK").catch(() => undefined); throw error; }
  finally { await database.end(); }
}
async function gateSnapshot() {
  return readDatabase(async (database) => {
    const result = await database.query<{ enabled: boolean; creation_enabled: boolean; host_execution_enabled: boolean; epoch: string }>(
      "SELECT enabled,creation_enabled,host_execution_enabled,epoch::text FROM collaboration_execution_gate WHERE id=1");
    const row = result.rows[0]; if (!row) throw new Error("The task database has no execution gate"); return row;
  });
}
function providerFacts(fixture: Pick<OutputFixture, "daemon">) {
  return fixture.daemon.visits.map((visit) => ({ identity: visit.identity, agent_id: visit.agent_id, agent_owner_id: visit.agent_owner_id,
    resource_owner_id: visit.resource_owner_id, execution_mode: visit.execution_mode, protocol_version: visit.protocol_version,
    started_ms: visit.started_ms, released_ms: visit.released_ms, closed_ms: visit.closed_ms, cli_hash: visit.cli_hash,
    workdir_hash: visit.workdir_hash, temp_hash: visit.temp_hash, session_hash: visit.session_hash, brief_hash: visit.brief_hash,
    resumed: visit.resumed, cli_context_sha256: digest(visit.cli_context), attachment_hashes: visit.attachment_hashes,
    workdir_files: visit.workdir_files, inputs: visit.inputs.map((input) => ({ ...input, body: undefined, body_sha256: sha(input.body) })),
    action_receipt_sha256: visit.replies.map((reply) => digest(reply)) }));
}

/** A small setup adapter; gate, native daemon, runtime transport and ledger owners are reused. */
async function withOutputRuntime<T>(evidence: RecoveryEvidence, exercise: (fixture: OutputFixture) => Promise<T>): Promise<T> {
  if (!runtimeOutputEnvironmentReady()) throw new Error("Agent output requires its exact separately permitted task allocation");
  const env = hostEnvironment(), databaseName = new URL(process.env.DATABASE_URL!).pathname.slice(1);
  expect(databaseName).toBe(process.env.TASK_DATABASE_NAME);
  expect(databaseName).toMatch(/^multica_pay241_s3o_20261119_tm89sjgy_[a-z0-9_]+$/);
  const storageRoot = process.env.COLLABORATION_OUTPUT_STORAGE_ROOT!;
  if (!isAbsolute(storageRoot) || !realpathSync(storageRoot).startsWith(`${stateRoot}/`)) throw new Error("Output storage observation requires the exact task-owned local root");
  expect(realpathSync(storageRoot)).toBe(realpathSync(process.env.LOCAL_UPLOAD_DIR!));
  const beforeGate = await gateSnapshot(), restoreGate = await openHostAcceptanceGate();
  let daemon: RuntimeFixture["daemon"] | undefined, transport: RuntimeFixture["transport"] | undefined;
  let output: OutputFixture["output"] | undefined, processes: OutputFixture["processes"] | undefined;
  let fixture: OutputFixture | undefined, result: { ok: true; value: T } | { ok: false; error: unknown };
  const cleanup: { owner: string; status: "fulfilled" | "rejected"; error?: ReturnType<typeof safeError> }[] = [];
  evidence.checkpoint("output_original_gate_before_owned_setup", { database: databaseName, gate: beforeGate });
  try {
    const config = await fetch(`${env.apiUrl}/api/config`, { redirect: "error" }); expect(config.status).toBe(200);
    expect(await config.json()).toMatchObject({ auth_mode: "password", managed_installation_supported: true, deployment_id: env.deploymentId });
    const readiness = await execute(process.env.COLLABORATION_HOST_PROVIDER!, ["--fixture-output-capabilities"], { timeout: 5_000, maxBuffer: 16 * 1024 });
    expect(JSON.parse(readiness.stdout)).toEqual({ mode: "agent_output_v1", regular_file_generation: true, bound_comment_attachment_cli: true, process_start_observation: true });
    expect(readiness.stderr).toBe("");
    const suffix = randomUUID().replaceAll("-", "").slice(0, 12), actors: RecoveryActor[] = [];
    const owner = await createHostActor(env.apiUrl, "Owner", suffix), handler = await createHostActor(env.apiUrl, "Handler", suffix);
    const participant = await createHostActor(env.apiUrl, "Viewer", suffix), outsider = await createHostActor(env.apiUrl, "Outside", suffix);
    for (const actor of [owner, handler, participant, outsider]) {
      const current = z.object({ id: z.uuid(), name: z.string() }).parse(await actor.client.requestJSON("/api/me")); expect(current.id).toBe(actor.id);
      actors.push({ client: actor.client, id: actor.id, name: current.name, async authenticateWeb(page) {
        await actor.authenticateWeb(page); await page.addInitScript(() => {
          localStorage.setItem("multica-locale", "en"); localStorage.setItem("multica:chat:isOpen", "false");
        });
      } });
    }
    const workspaceSchema = z.object({ id: z.uuid(), slug: z.string() });
    const sourceWorkspace = workspaceSchema.parse(await owner.client.requestJSON("/api/workspaces", { method: "POST", body: {
      name: "S3 real Agent output source", slug: `s3-output-source-${suffix}` } }));
    const workspace = workspaceSchema.parse(await owner.client.requestJSON("/api/workspaces", { method: "POST", body: {
      name: "S3 separate Agent output resource", slug: `s3-output-resource-${suffix}` } }));
    expect(sourceWorkspace.id).not.toBe(workspace.id);
    owner.client.setWorkspaceId(workspace.id); owner.client.setWorkspaceSlug(workspace.slug);
    const runtime = await enrollHostRuntime(owner.client, owner.id, workspace.id, env.apiUrl, env.deploymentId);
    evidence.checkpoint("actual_output_runtime_identity_before_daemon_start", { runtime_id: runtime.runtimeId, installation_id: runtime.installationId,
      binding_id: runtime.credential.binding_id, binding_epoch: runtime.credential.binding_epoch,
      actor_ids: { owner: owner.id, handler: handler.id, participant: participant.id, outsider: outsider.id },
      source_workspace_id: sourceWorkspace.id, execution_workspace_id: workspace.id });
    processes = await processObserver(evidence);
    transport = await runtimeTransport(env.apiUrl);
    // Put output observation outside the original owner so intentional output
    // ACK loss does not become a forwarding error in the accepted transport.
    output = await outputTransport(transport.apiUrl, evidence);
    daemon = await startHostDaemon(runtime, output.apiUrl, false, { testReport: true });
    await waitForHostRuntime(owner.client, runtime.runtimeId);
    fixture = { ...env, owner, handler, participant, outsider, actors, sourceWorkspace, workspace, runtime, daemon, transport, output, processes,
      issues: [], generated: [] };
    const sourcePaths = ["e2e/collaboration-runtime-output.spec.ts", "e2e/fixtures/collaboration-runtime-output.ts",
      "e2e/fixtures/collaboration-recovery-runtime.ts", "e2e/fixtures/collaboration-host-provider.go", "e2e/fixtures/collaboration-host-runtime.ts",
      "e2e/fixtures/collaboration-recovery.ts", "e2e/fixtures/collaboration-native.ts", "packages/core/collaboration/attachment-schema.ts",
      "packages/core/collaboration/run-records-schema.ts", "packages/views/collaboration/run-records.tsx",
      process.env.COLLABORATION_HOST_DAEMON!, process.env.COLLABORATION_HOST_PROVIDER!, process.env.COLLABORATION_HOST_CLI!];
    evidence.set({ database: databaseName, api_url: env.apiUrl, deployment_id: env.deploymentId, runtime_id: runtime.runtimeId,
      installation_id: runtime.installationId, binding_id: runtime.credential.binding_id, daemon_readiness: daemon.ready,
      source_workspace_id: sourceWorkspace.id, execution_workspace_id: workspace.id,
      provider_output_readiness_sha256: sha(readiness.stdout), sources: sourcePaths.map((path) => ({ path, sha256: sha(readFileSync(path)) })),
      real_model_execution: false, installed_main_execution: false, execution_mode: hostMode });
    result = { ok: true, value: await exercise(fixture) };
  } catch (error) {
    result = { ok: false, error };
    let ledger: unknown;
    try { ledger = daemon?.ledgerEvidence() ?? []; } catch (failure) { ledger = { observation_error: safeError(failure) }; }
    evidence.checkpoint("output_primary_failure_before_owned_teardown", { original_error: safeError(error),
      providers: fixture ? providerFacts(fixture) : [], original_ledger: ledger, output_transport: output?.evidence() ?? null,
      runtime_transport: transport?.evidence() ?? null, process_observations: processes?.evidence() ?? null });
  } finally {
    output?.releaseAll(); transport?.releaseAll();
    for (const [owner, action] of [["controlled_daemon", async () => {
      if (!daemon) return;
      const summary = await daemon.close(); evidence.set({ daemon_final: summary, providers: fixture ? providerFacts(fixture) : [] });
      expect(summary.exit_code).toBe(0); expect(summary.errors).toEqual([]); expect(summary.stderr).toBe("");
      const capacity = summary.messages.filter((item) => "claims" in item);
      expect(capacity.length).toBeGreaterThan(0);
      expect(capacity.every((item) => item.invalid_capacity === false && item.available_slots === 1 && item.active_tasks === 0)).toBe(true);
      expect(capacity.reduce((sum, item) => sum + item.claims, 0)).toBe(daemon.visits.length);
      expect(summary.ledger).toHaveLength(daemon.visits.length);
    }], ["output_loopback_transport", async () => {
      if (!output) return;
      const observed = output.evidence(); evidence.set({ output_transport: observed }); await output.close(); expect(observed.errors).toEqual([]);
      // Intentional original outcome/proof ACK loss belongs to the old owner.
      for (const failure of observed.passthrough_failures) {
        const match = /^\/api\/collaboration\/runtime\/runs\/([a-f0-9-]{36})\/(outcome|proof)$/.exec(failure.path);
        expect(match).not.toBeNull();
        expect(transport?.runFacts(match![1]!).some((fact) => fact.operation === match![2] && fact.acknowledgement_lost)).toBe(true);
      }
    }], ["original_runtime_transport", async () => {
      if (!transport) return; const observed = transport.evidence(); evidence.set({ runtime_transport: observed }); await transport.close(); expect(observed.errors).toEqual([]);
    }], ["output_process_observer", async () => {
      if (!processes) return; const observed = processes.evidence(); evidence.set({ process_observations: observed }); await processes.close(); expect(observed.errors).toEqual([]);
    }], ["execution_gate", async () => {
      await restoreGate(); const after = await gateSnapshot();
      evidence.checkpoint("output_exact_original_gate_restored", { before: beforeGate, after });
      expect({ ...after, epoch: beforeGate.epoch }).toEqual(beforeGate); expect(BigInt(after.epoch)).toBeGreaterThanOrEqual(BigInt(beforeGate.epoch));
    }]] as const) {
      try { await action(); cleanup.push({ owner, status: "fulfilled" }); }
      catch (error) { cleanup.push({ owner, status: "rejected", error: safeError(error) }); }
      evidence.set({ runtime_cleanup: cleanup });
    }
  }
  if (!result.ok) throw result.error;
  expect(cleanup.every((item) => item.status === "fulfilled")).toBe(true); return result.value;
}

async function outputAudit(fixture: OutputFixture, evidence: RecoveryEvidence, stage: string) {
  const business = await audit(fixture, evidence, `${stage}_original_runtime_owner`);
  const output = await readDatabase(async (database) => {
    const ids = fixture.issues.map((issue) => issue.id);
    const files = await database.query<Record<string, unknown>>(`SELECT a.id,a.issue_id,a.source_workspace_id,a.draft_id,a.uploader_user_id,
      a.uploader_agent_id,a.run_id,a.attempt_seq::text,a.intended_issue_id,a.upload_request_id,a.name,a.content_type,a.size_bytes::text,
      a.version::text,a.created_at::text,a.delivered_at::text,a.discarded_at::text,a.content_sha256,a.storage_key,
      octet_length(a.encryption_key) AS encryption_key_bytes,r.agent_id AS actual_run_agent_id,r.authorized_attempt_seq::text AS actual_authorized_attempt,
      r.session_id,s.issue_id AS actual_session_issue_id,i.workspace_id AS actual_source_workspace_id
      FROM collaboration_attachment a JOIN collaboration_run r ON r.id=a.run_id JOIN collaboration_session s ON s.id=r.session_id
      JOIN issue i ON i.id=s.issue_id WHERE a.issue_id=ANY($1::uuid[]) ORDER BY a.created_at,a.id`, [ids]);
    const refs = await database.query<Record<string, unknown>>(`SELECT f.attachment_id,f.reference_kind,f.reference_id,f.reference_revision::text,f.removed_at::text
      FROM collaboration_attachment_reference f JOIN collaboration_attachment a ON a.id=f.attachment_id
      WHERE a.issue_id=ANY($1::uuid[]) AND a.uploader_agent_id IS NOT NULL
      ORDER BY f.attachment_id,f.reference_kind,f.reference_id,f.reference_revision`, [ids]);
    const events = await database.query<{ id: string; issue_id: string; run_id: string; producer_event_id: string; kind: string; payload_json: string }>(
      `SELECT id,issue_id,run_id,producer_event_id,kind,payload::text AS payload_json FROM collaboration_run_event
       WHERE issue_id=ANY($1::uuid[]) AND kind IN ('comment','outcome') ORDER BY run_id,seq,id`, [ids]);
    const stored = events.rows.map((event) => {
      const raw = object.parse(JSON.parse(event.payload_json));
      if (event.kind === "outcome") {
        const outcome = z.object({ identity: z.object({ run_id: z.uuid(), issue_id: z.uuid() }), event_id: z.uuid(), attempt_seq: positive,
          outcome: z.enum(["success", "failure"]), output_attachment_ids: z.array(z.uuid()).max(32).optional() }).parse(raw);
        expect(outcome.identity).toMatchObject({ run_id: event.run_id, issue_id: event.issue_id });
        return { id: event.id, issue_id: event.issue_id, run_id: event.run_id, producer_event_id: event.producer_event_id, kind: event.kind,
          payload_sha256: sha(event.payload_json), attempt_seq: outcome.attempt_seq, output_attachment_ids: outcome.output_attachment_ids ?? null, outcome: outcome.outcome };
      }
      const input = z.object({ comment_id: z.uuid(), content: z.string(), parent_id: z.uuid().nullable(), attachment: z.object({ name: z.string(),
        content_type: z.string(), size_bytes: z.number().int().nonnegative(), content_sha256: hash, attempt_seq: positive }).optional() }).parse(raw.input);
      const receipt = input.attachment ? outputReceipt.parse(raw.receipt) : null;
      return { id: event.id, issue_id: event.issue_id, run_id: event.run_id, producer_event_id: event.producer_event_id, kind: event.kind,
        payload_sha256: sha(event.payload_json), comment_id: input.comment_id, content_sha256: sha(input.content), parent_id: input.parent_id,
        fingerprint: input.attachment ?? null, receipt };
    });
    const storageRoot = realpathSync(process.env.COLLABORATION_OUTPUT_STORAGE_ROOT!), privateDirectory = join(storageRoot, "private", "collaboration");
    const physical = files.rows.map<Record<string, unknown>>((row) => {
      const storedKey = z.string().parse(row.storage_key), fileId = z.uuid().parse(row.id);
      expect(storedKey).toBe(`private/collaboration/${fileId}`);
      const path = join(storageRoot, storedKey), stat = lstatSync(path);
      expect(stat.isSymbolicLink()).toBe(false); expect(stat.isFile()).toBe(true);
      expect(relative(storageRoot, realpathSync(path))).toBe(storedKey);
      const ciphertext = readFileSync(path), plaintextSize = Number(z.string().parse(row.size_bytes));
      expect(ciphertext.length).toBeGreaterThan(plaintextSize); expect(sha(ciphertext)).not.toBe(row.content_sha256);
      expect(row.encryption_key_bytes).toBe(32);
      const { storage_key: _storageKey, ...safe } = row;
      return { ...safe, storage_key_sha256: sha(storedKey), object_present: true, ciphertext_sha256: sha(ciphertext), ciphertext_bytes: ciphertext.length };
    });
    const allRows = await database.query<{ storage_key: string }>("SELECT storage_key FROM collaboration_attachment WHERE encryption_key IS NOT NULL ORDER BY storage_key");
    const objectNames = existsSync(privateDirectory) ? readdirSync(privateDirectory).sort() : [];
    expect(objectNames.every((name) => z.uuid().safeParse(name).success && lstatSync(join(privateDirectory, name)).isFile())).toBe(true);
    const objectHashes = objectNames.map((name) => sha(`private/collaboration/${name}`)).sort();
    expect(objectHashes).toEqual(allRows.rows.map((row) => sha(row.storage_key)).sort());
    return { files: physical, refs: refs.rows, events: stored, global_private_objects: objectHashes,
      global_private_object_count: objectNames.length, global_encrypted_row_count: allRows.rows.length };
  });
  for (const file of fixture.generated) {
    const row = output.files.find((row) => row.id === file.receipt.attachment.id); expect(row).toBeDefined();
    expect(row).toMatchObject({ issue_id: file.issue.id, source_workspace_id: fixture.sourceWorkspace.id,
      draft_id: null, uploader_user_id: null, uploader_agent_id: file.issue.agentId, run_id: file.runId, attempt_seq: "1",
      intended_issue_id: null, upload_request_id: null, name: file.name, content_type: "text/plain; charset=utf-8",
      size_bytes: String(file.bytes.length), version: "1", discarded_at: null, content_sha256: sha(file.bytes),
      actual_run_agent_id: file.issue.agentId, actual_authorized_attempt: "1", actual_session_issue_id: file.issue.id,
      actual_source_workspace_id: fixture.sourceWorkspace.id, object_present: true });
    expect(row!.delivered_at).not.toBeNull();
    expect(output.refs.filter((ref) => ref.attachment_id === file.receipt.attachment.id)).toEqual([
      expect.objectContaining({ reference_kind: "comment", reference_id: file.requestId, reference_revision: "1", removed_at: null }),
      expect.objectContaining({ reference_kind: "issue", reference_id: file.issue.id, reference_revision: "1", removed_at: null }),
    ]);
    const event = output.events.find((event) => event.run_id === file.runId && event.producer_event_id === `comment:${file.requestId}`);
    expect(event).toMatchObject({ kind: "comment", comment_id: file.requestId, content_sha256: sha(file.content), parent_id: null,
      fingerprint: { name: file.name, content_type: "text/plain; charset=utf-8", size_bytes: file.bytes.length, content_sha256: sha(file.bytes), attempt_seq: "1" }, receipt: file.receipt });
  }
  const counts: Record<string, number> = { ...business.counts, agent_outputs: output.files.length, output_refs: output.refs.length,
    recorded_outcomes: output.events.filter((event) => event.kind === "outcome").length };
  const snapshot = { business, output, counts };
  evidence.checkpoint(stage, snapshot); return snapshot;
}

type OutputAction = Extract<HostAction, { kind: "comment" }> & { mode: "agent_output_v1"; path: string; generate: boolean;
  file_content: string; process_callback: string };
function assertOutputVisit(fixture: OutputFixture, issue: HostIssue, visit: HostVisit, input: OriginalInput) {
  const earlier = fixture.generated.filter((file) => file.issue.id === issue.id);
  if (earlier.length === 0) { assertVisit(fixture, issue, visit, input); return; }
  // Later explicit runs receive all our small delivered history files through
  // the real frozen preparation. The legacy helper's single-file case stays exact.
  expect(visit).toMatchObject({ protocol_version: 3, execution_mode: hostMode, agent_id: issue.agentId,
    agent_owner_id: fixture.owner.id, resource_owner_id: fixture.owner.id, cli_hash: fixture.daemon.cliHash,
    identity: { issue_id: issue.id, target_id: issue.targetId, channel_id: issue.channelId, batch_id: input.batchId },
    cli_context: { issue_id: issue.id, title: issue.title, description: issue.description } });
  const original = { command_id: input.commandId, author_user_id: input.actorId, body: input.content, attachment_ids: [] };
  expect(visit.inputs).toEqual([original]);
  expect(visit.cli_context.inputs.map(({ command_id, author_user_id, body, attachment_ids }) => ({ command_id, author_user_id, body, attachment_ids }))).toEqual([original]);
  if (!issue.attachment) throw new Error("The output fixture lost its original human input file identity");
  const expected = { [issue.attachment.id]: issue.attachment.sha256,
    ...Object.fromEntries(earlier.map((file) => [file.receipt.attachment.id, sha(file.bytes)])) };
  expect(visit.attachment_hashes).toEqual(expected);
}
function outputAction(fixture: OutputFixture, requestId: string, name: string, bytes: Buffer, content: string, generate: boolean): OutputAction {
  return { kind: "comment", request_id: requestId, content, mode: "agent_output_v1", path: name, generate,
    file_content: bytes.toString("utf8"), process_callback: fixture.processes.address };
}
async function invokeOutput(fixture: OutputFixture, issue: HostIssue, visit: HostVisit, action: OutputAction, ordinal: number) {
  const runId = z.uuid().parse(visit.identity.run_id), replies = await fixture.daemon.act(visit, [action]);
  expect(replies).toHaveLength(1);
  const reply = actionReceipt.parse(replies[0]), observed = await fixture.processes.invocation(runId, action.request_id, ordinal);
  const credentials = fixture.transport.credentialEvidence(runId);
  expect(observed.started).toMatchObject({ issue_id: issue.id, run_id: runId, request_id: action.request_id,
    cli_sha256: fixture.daemon.cliHash, provider_sha256: sha(readFileSync(process.env.COLLABORATION_HOST_PROVIDER!)),
    business_token_sha256: credentials.business_token_sha256, cwd_sha256: visit.workdir_hash,
    file_name: action.path, file_sha256: sha(action.file_content), file_bytes: Buffer.byteLength(action.file_content), regular_file: true });
  expect(reply.file).toMatchObject({ name: action.path, sha256: sha(action.file_content), size_bytes: Buffer.byteLength(action.file_content),
    regular_file: true, generated: action.generate, cwd_sha256: visit.workdir_hash });
  expect(observed.completed.exit_code).toBe(reply.cli.exit_code);
  return { reply, observed, credentials };
}
function committedOutput(fixture: OutputFixture, issue: HostIssue, visit: HostVisit, action: OutputAction,
  actual: Awaited<ReturnType<typeof invokeOutput>>, evidence: RecoveryEvidence): GeneratedOutput {
  expect(actual.reply.cli).toMatchObject({ success: true, exit_code: 0, stderr_bytes: 0 });
  const receipt = outputReceipt.parse(actual.reply.result), runId = z.uuid().parse(visit.identity.run_id);
  const metadata = agentAttachment.parse(CollaborationAttachmentSchema.parse(receipt.attachment));
  expect(receipt).toMatchObject({ comment_id: action.request_id, run_ids: [], continuation_run_id: null });
  expect(metadata).toMatchObject({ draft_id: null, uploader_user_id: null, uploader_agent_id: issue.agentId, run_id: runId, attempt_seq: "1",
    issue_id: issue.id, source_workspace_id: fixture.sourceWorkspace.id, intended_issue_id: null,
    name: action.path, content_type: "text/plain; charset=utf-8", size_bytes: Buffer.byteLength(action.file_content), version: "1" });
  expect(Date.parse(metadata.delivered_at)).toBeGreaterThanOrEqual(Date.parse(metadata.created_at));
  const original = fixture.output.original(runId, action.request_id);
  expect(original.receipt).toEqual(receipt); expect(original.business_token_sha256).toBe(actual.credentials.business_token_sha256);
  expect(original.input).toMatchObject({ comment_id: action.request_id, content_sha256: sha(action.content), parent_id: null,
    file_name: action.path, file_sha256: sha(action.file_content), file_bytes: Buffer.byteLength(action.file_content) });
  const file = { issue, runId, requestId: action.request_id, content: action.content, name: action.path, bytes: Buffer.from(action.file_content),
    receipt, invocation: actual.observed };
  fixture.generated.push(file);
  evidence.checkpoint("actual_agent_run_file_and_typed_winning_cli_receipt", { issue_id: issue.id, run_id: runId, request_id: action.request_id,
    file_name: file.name, file_sha256: sha(file.bytes), content_sha256: sha(file.content), receipt, original, invocation: actual.observed,
    credentials: actual.credentials, declared_derived_runs: 0, declared_continuation_run: null });
  return file;
}
async function generateOutput(fixture: OutputFixture, issue: HostIssue, visit: HostVisit, label: string, evidence: RecoveryEvidence) {
  const requestId = randomUUID(), name = `agent-output-${requestId}.txt`, bytes = Buffer.from(`Synthetic Agent ${label} output for ${visit.identity.run_id}. Original immutable bytes.\n`);
  const action = outputAction(fixture, requestId, name, bytes, `Controlled Agent ${label} output file ${name}.`, true);
  const actual = await invokeOutput(fixture, issue, visit, action, 1); return committedOutput(fixture, issue, visit, action, actual, evidence);
}
async function runtimeFileRead(fixture: OutputFixture, file: GeneratedOutput, expected: 200 | 403, evidence: RecoveryEvidence, stage: string) {
  const reply = await fixture.transport.read(file.runId, `attachments/${file.receipt.attachment.id}/content?version=${file.receipt.attachment.version}`);
  expect(reply.status).toBe(expected); expect(reply.headers["cache-control"]).toContain("no-store");
  expect(reply.headers.location).toBeUndefined();
  if (expected === 200) {
    expect(reply.bytes).toEqual(file.bytes); expect(reply.headers["x-multica-run-id"]).toBe(file.runId);
    expect(reply.headers["x-multica-attachment-id"]).toBe(file.receipt.attachment.id); expect(reply.headers["x-multica-attachment-version"]).toBe("1");
  }
  const fact = { issue_id: file.issue.id, run_id: file.runId, attachment_id: file.receipt.attachment.id, version: file.receipt.attachment.version,
    status: reply.status, response_sha256: sha(reply.bytes), response_bytes: reply.bytes.length,
    business_token_sha256: fixture.transport.credentialEvidence(file.runId).business_token_sha256,
    ...(expected === 403 ? { refusal: refusal(reply) } : { plaintext_sha256: sha(file.bytes) }) };
  evidence.checkpoint(stage, fact); return fact;
}
async function humanFileRead(fixture: OutputFixture, file: GeneratedOutput, actor: RecoveryActor, expected: 200 | 404,
  evidence: RecoveryEvidence, stage: string) {
  const facts = [];
  for (const suffix of ["", "/content", "/download"] as const) {
    const path = `${issueRoot(file.issue.id)}/attachments/${file.receipt.attachment.id}${suffix}`;
    const response = await fetch(`${fixture.apiUrl}${path}`, { redirect: "error", headers: { Authorization: `Bearer ${actor.client.getToken()!}` } });
    const bytes = Buffer.from(await response.arrayBuffer()); expect(response.status).toBe(expected);
    expect(response.headers.get("cache-control")).toContain("no-store"); expect(response.headers.get("location")).toBeNull();
    if (expected === 200) {
      if (suffix === "") expect(agentAttachment.parse(CollaborationAttachmentSchema.parse(JSON.parse(bytes.toString())))).toEqual(file.receipt.attachment);
      else expect(bytes).toEqual(file.bytes);
    }
    facts.push({ actor_id: actor.id, path, status: response.status, response_sha256: sha(bytes), response_bytes: bytes.length });
  }
  evidence.checkpoint(stage, { issue_id: file.issue.id, attachment_id: file.receipt.attachment.id, facts }); return facts;
}
async function originalOutputDenied(fixture: OutputFixture, file: GeneratedOutput, evidence: RecoveryEvidence, stage: string) {
  const before = await outputAudit(fixture, evidence, `${stage}_before`);
  const original = await fixture.output.replay(file.runId, file.requestId), changed = await fixture.output.replay(file.runId, file.requestId,
    { bytes: Buffer.from(file.bytes.toString().replace("Synthetic", "Changed!!")) });
  const freshKey = randomUUID(), fresh = await fixture.output.replay(file.runId, file.requestId, { requestId: freshKey });
  const denials = [original, changed, fresh].map((result) => {
    expect(result.reply.status).toBe(403); return { ...result.identity, refusal: refusal(result.reply) };
  });
  const read = await runtimeFileRead(fixture, file, 403, evidence, `${stage}_original_runtime_bytes_denied`);
  const after = await outputAudit(fixture, evidence, `${stage}_after`); expect(after).toEqual(before);
  evidence.checkpoint(stage, { original_request_id: file.requestId, new_late_request_id: freshKey, denials, read,
    before_counts: before.counts, after_counts: after.counts }); return { denials, read, fresh_request_id: freshKey };
}

async function previewFile(session: RecoverySession, container: Locator, file: GeneratedOutput) {
  const preview = container.getByRole("button", { name: `Preview ${file.name}`, exact: true }); await expect(preview).toHaveCount(1); await preview.click();
  const dialog = session.page.getByRole("dialog", { name: file.name, exact: true });
  await expect(dialog.locator("pre")).toHaveText(file.bytes.toString());
  await dialog.getByRole("button", { name: "Close preview", exact: true }).click(); await expect(dialog).toHaveCount(0);
  return { file_id: file.receipt.attachment.id, filename: file.name, plaintext_sha256: sha(file.bytes), actual_platform: session.platform };
}
async function downloadFile(session: RecoverySession, container: Locator, file: GeneratedOutput, stage: string) {
  const button = container.getByRole("button", { name: `Download ${file.name}`, exact: true }); await expect(button).toHaveCount(1);
  if (!session.native) {
    const pending = session.page.waitForEvent("download"); await button.click(); const download = await pending;
    expect(await download.failure()).toBeNull(); expect(download.suggestedFilename()).toBe(file.name);
    const path = await download.path(); if (!path) throw new Error("The actual Web download had no retained file");
    const bytes = readFileSync(path); expect(bytes).toEqual(file.bytes);
    return { stage, platform: session.platform, state: "completed", filename: download.suggestedFilename(), sha256: sha(bytes), bytes: bytes.length };
  }
  if (!session.lifecycle.profile) throw new Error("The native file download has no owned profile");
  const path = join(session.lifecycle.profile, `${randomUUID()}-output-download.txt`);
  await session.native.app.evaluate(({ session: nativeSession }, output) => {
    const pending = new Promise<{ state: string; filename: string }>((resolve) => {
      nativeSession.defaultSession.once("will-download", (_event, item) => {
        item.setSavePath(output); item.once("done", (_event, state) => resolve({ state, filename: item.getFilename() }));
      });
    });
    (globalThis as unknown as { pay241OutputDownload: typeof pending }).pay241OutputDownload = pending;
  }, path);
  await button.click();
  const result = await session.native.app.evaluate(() => Promise.race([
    (globalThis as unknown as { pay241OutputDownload: Promise<{ state: string; filename: string }> }).pay241OutputDownload,
    new Promise<never>((_resolve, reject) => setTimeout(() => reject(new Error("The native output download did not complete")), 15_000)),
  ]));
  expect(result).toEqual({ state: "completed", filename: file.name }); const bytes = readFileSync(path); expect(bytes).toEqual(file.bytes); rmSync(path);
  return { stage, platform: session.platform, ...result, sha256: sha(bytes), bytes: bytes.length, actual_native_will_download: true };
}
async function currentComment(fixture: OutputFixture, file: GeneratedOutput) {
  const comment = CollaborationCommentSchema.parse(await fixture.owner.client.requestJSON(`${issueRoot(file.issue.id)}/comments/${file.requestId}`));
  expect(comment).toMatchObject({ id: file.requestId, author_type: "agent", author: null,
    author_agent: { agent_id: file.issue.agentId, name: file.issue.name }, content: file.content, attachment_ids: [file.receipt.attachment.id], comment_revision: "1" });
  return { id: comment.id, author_type: comment.author_type, agent_id: comment.author_agent?.agent_id, attachment_ids: comment.attachment_ids,
    content_sha256: sha(comment.content), comment_revision: comment.comment_revision };
}
async function showOutputFiles(fixture: OutputFixture, session: RecoverySession, file: GeneratedOutput, expectedOutcome: "failure" | "success",
  evidence: RecoveryEvidence, stage: string) {
  const comment = await currentComment(fixture, file), page = await records(fixture, file.issue, file.runId);
  const result = page.items.filter((item) => item.kind === "result"); expect(result).toHaveLength(1);
  expect(result[0]).toMatchObject({ outcome: expectedOutcome, attachment_ids: [file.receipt.attachment.id] });
  expect(page.items.filter((item) => item.kind === "comment" && item.comment_id === file.requestId))
    .toEqual([expect.objectContaining({ attachment_ids: [file.receipt.attachment.id], body: file.content })]);
  const list = CollaborationAttachmentsSchema.parse(await session.actor.client.requestJSON(`${issueRoot(file.issue.id)}/attachments`));
  expect(agentAttachment.parse(list.attachments.find((item) => item.id === file.receipt.attachment.id))).toEqual(file.receipt.attachment);
  await session.navigate(`${surface(fixture, file.issue).accountPath}#run-${file.runId}`);
  await expect(session.page.getByRole("heading", { name: file.issue.title, exact: true })).toBeVisible();
  const publicComment = session.page.locator(`[id="comment-${file.requestId}"]`); await expect(publicComment).toContainText(file.issue.name);
  const commentPreview = await previewFile(session, publicComment, file), commentDownload = await downloadFile(session, publicComment, file, `${stage}-comment`);
  const disclosure = session.page.locator(`[id="run-${file.runId}"]`), toggle = disclosure.getByRole("button", { name: new RegExp(`^(?:View|Hide) records for ${file.runId}$`) });
  const originalToggleLabel = await toggle.getAttribute("aria-label"); if (await toggle.getAttribute("aria-expanded") !== "true") await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "true"); await expect(toggle).toHaveAccessibleName(`Hide records for ${file.runId}`);
  const panel = disclosure.getByRole("region", { name: "Run records", exact: true });
  await expect(panel).toBeVisible(); await expect(panel).toContainText("Direct launches: 1"); await expect(panel).toContainText("Standard host execution");
  await expect(panel).toContainText(expectedOutcome === "failure" ? `Synthetic controlled test execution failed for ${file.runId}` : `Host fixture result for ${file.runId}`);
  const resultEntry = panel.locator(`[data-record-id="${result[0]!.id}"]`);
  const resultPreview = await previewFile(session, resultEntry, file), resultDownload = await downloadFile(session, resultEntry, file, `${stage}-result`);
  const facts = { stage, platform: session.platform, run_id: file.runId, attachment_id: file.receipt.attachment.id, comment,
    original_toggle_label: originalToggleLabel, expanded_toggle_label: `Hide records for ${file.runId}`, result_record_id: result[0]!.id,
    result_attachment_ids: result[0]!.attachment_ids, commentPreview, commentDownload, resultPreview, resultDownload, records_sha256: digest(page) };
  evidence.checkpoint("actual_shared_comment_and_frozen_result_file_preview_download", facts); return facts;
}

async function settleOutput(fixture: OutputFixture, file: GeneratedOutput, visit: HostVisit, evidence: RecoveryEvidence) {
  await fixture.transport.waitForHeldProof(file.runId);
  const beforeProof = await readIssue(fixture, file.issue);
  expect(beforeProof.execution!.runs.find((run) => run.id === file.runId)).toMatchObject({ proof_state: "unknown", settled_at: null });
  fixture.transport.releaseProof(file.runId); await fixture.transport.waitForCommittedProof(file.runId);
  await waitSettlement(fixture, file.issue, visit);
  const originalNotices = await notifications(fixture, file.issue);
  const recovered = await recoverOriginal(fixture, file.issue, visit, evidence);
  const snapshot = await outputAudit(fixture, evidence, "output_original_proof_recovery_complete");
  const notices = await notifications(fixture, file.issue);
  expect(notices).toEqual(originalNotices);
  evidence.checkpoint("output_proof_and_original_recipient_identities", { run_id: file.runId, recovery: recovered,
    recorded_outcome: fixture.transport.recordedOutcome(file.runId), notices: Object.values(notices).flat().filter((notice) => notice.run_id === file.runId),
    snapshot_counts: snapshot.counts, physical_provider_count: fixture.daemon.visits.length });
  return { recovered, snapshot, notices, records: await records(fixture, file.issue, file.runId) };
}
async function frozenOutput(fixture: OutputFixture, file: GeneratedOutput, expectedOutcome: "failure" | "success", evidence: RecoveryEvidence) {
  expect(fixture.transport.recordedOutcome(file.runId)).toBe(true);
  const page = await records(fixture, file.issue, file.runId), snapshot = await outputAudit(fixture, evidence, "actual_server_owned_frozen_output_ids");
  expect(page.run.business_outcome).toBe(expectedOutcome);
  expect(page.items.filter((item) => item.kind === "result")).toEqual([
    expect.objectContaining({ outcome: expectedOutcome, attachment_ids: [file.receipt.attachment.id] }),
  ]);
  expect(snapshot.output.events.filter((event) => event.run_id === file.runId && event.kind === "outcome"))
    .toEqual([expect.objectContaining({ outcome: expectedOutcome, attempt_seq: "1", output_attachment_ids: [file.receipt.attachment.id] })]);
  const ack = await fixture.transport.replay(file.runId, "outcome"); expect(ack.response.status).toBe(200);
  expect(object.parse(wireJSON(ack.response))).toEqual({ code: "outcome_recorded" });
  evidence.checkpoint("original_outcome_ack_keeps_unchanged_request_and_no_private_refs", { ...ack.identity, run_id: file.runId,
    frozen_output_ids: [file.receipt.attachment.id], records_sha256: digest(page) }); return page;
}
async function withOutputSessions<T>(browser: Browser, platform: RecoveryPlatform, fixture: OutputFixture, issue: HostIssue, info: TestInfo,
  evidence: RecoveryEvidence, label: string, exercise: Parameters<typeof withSessions<T>>[5]) {
  try { return await withSessions(browser, platform, surface(fixture, issue), info, evidence, exercise); }
  finally { evidence.checkpoint("output_client_scope_cleanup_retained", { scope: label, issue_id: issue.id,
    sessions: evidence.value.sessions ?? [], cleanup: evidence.value.all_sessions_cleanup ?? [] }); }
}

interface RetainedOutput {
  file: GeneratedOutput; page: Awaited<ReturnType<typeof records>>;
  notices: Awaited<ReturnType<typeof notifications>>;
  file_facts: unknown; comment_event: unknown; outcome_event: unknown;
}
async function retainOutput(fixture: OutputFixture, file: GeneratedOutput, evidence: RecoveryEvidence): Promise<RetainedOutput> {
  const snapshot = await outputAudit(fixture, evidence, "original_output_and_result_facts_frozen_for_later_comparison");
  return { file, page: await records(fixture, file.issue, file.runId), notices: await notifications(fixture, file.issue),
    file_facts: snapshot.output.files.find((row) => row.id === file.receipt.attachment.id),
    comment_event: snapshot.output.events.find((event) => event.run_id === file.runId && event.producer_event_id === `comment:${file.requestId}`),
    outcome_event: snapshot.output.events.find((event) => event.run_id === file.runId && event.kind === "outcome") ?? null };
}
async function assertRetained(fixture: OutputFixture, retained: RetainedOutput[], evidence: RecoveryEvidence, stage: string) {
  const snapshot = await outputAudit(fixture, evidence, stage);
  for (const old of retained) {
    expect(await records(fixture, old.file.issue, old.file.runId)).toEqual(old.page);
    expect(snapshot.output.files.find((row) => row.id === old.file.receipt.attachment.id)).toEqual(old.file_facts);
    expect(snapshot.output.events.find((event) => event.run_id === old.file.runId && event.producer_event_id === `comment:${old.file.requestId}`)).toEqual(old.comment_event);
    expect(snapshot.output.events.find((event) => event.run_id === old.file.runId && event.kind === "outcome") ?? null).toEqual(old.outcome_event);
    const current = await notifications(fixture, old.file.issue);
    expect(Object.values(current).flat().filter((item) => item.run_id === old.file.runId))
      .toEqual(Object.values(old.notices).flat().filter((item) => item.run_id === old.file.runId));
    expect(await currentComment(fixture, old.file)).toMatchObject({ id: old.file.requestId, agent_id: old.file.issue.agentId,
      attachment_ids: [old.file.receipt.attachment.id], content_sha256: sha(old.file.content), comment_revision: "1" });
  }
  return snapshot;
}
function terminalRecipients(fixture: OutputFixture, file: GeneratedOutput, notices: Awaited<ReturnType<typeof notifications>>,
  kind: "ai_failed" | "ai_completed" | "ai_stopped") {
  const recipients = [fixture.owner.id, fixture.handler.id, fixture.participant.id];
  const items = Object.values(notices).flat().filter((item) => item.run_id === file.runId);
  expect(items).toHaveLength(3); expect(new Set(items.map((item) => item.id)).size).toBe(3);
  expect(new Set(items.map((item) => item.event_seq)).size).toBe(1);
  for (const recipient of recipients) expect(notices[recipient]!.filter((item) => item.run_id === file.runId))
    .toEqual([expect.objectContaining({ kind, issue_id: file.issue.id, run_id: file.runId, can_open: true })]);
  return items.map((item) => ({ id: item.id, event_seq: item.event_seq, kind: item.kind,
    recipient_user_id: recipients.find((recipient) => notices[recipient]!.some((notice) => notice.id === item.id)) }));
}
function workflowAuthority(issue: Awaited<ReturnType<typeof readIssue>>) {
  if (!issue.write_context) throw new Error("The output controller has no current workflow authority");
  return { expected_collaboration_revision: issue.collaboration_revision, expected_lifecycle_epoch: issue.lifecycle_epoch,
    expected_participant_epoch: issue.write_context.participant_epoch, expected_account_epoch: issue.write_context.account_epoch };
}

async function withdrawOutput(browser: Browser, platform: RecoveryPlatform, info: TestInfo, evidence: RecoveryEvidence,
  fixture: OutputFixture, authorWithdrawal: boolean, retained: RetainedOutput[], inputs: OriginalInput[]) {
  const label = authorWithdrawal ? "output-author-withdrawal" : "output-grant-withdrawal";
  const issue = await createRuntimeIssue(fixture, label, evidence), expectedVisits = fixture.daemon.visits.length + 1;
  return withOutputSessions(browser, platform, fixture, issue, info, evidence, label, async (open) => {
    const actor = actorFor(fixture, authorWithdrawal ? fixture.participant : fixture.handler), session = await open(actor);
    await authenticatedSocket(session, "");
    const controller = authorWithdrawal ? await open(actorFor(fixture, fixture.handler)) : session;
    if (controller !== session) await authenticatedSocket(controller, "");
    await control(controller, issue, "resume", evidence, "issue");
    const input = await submitInput(session, issue, `S3 actual ${label} input author`, evidence); inputs.push(input);
    const visit = await fixture.daemon.waitForVisit(expectedVisits); assertOutputVisit(fixture, issue, visit, input);
    const runId = z.uuid().parse(visit.identity.run_id); fixture.transport.holdProof(runId); fixture.transport.loseReceiptACKs(runId);
    const file = await generateOutput(fixture, issue, visit, label, evidence);
    const liveRead = await runtimeFileRead(fixture, file, 200, evidence, `${label}_original_mcb_file_read_before_withdrawal`);
    await humanFileRead(fixture, file, actor, 200, evidence, `${label}_current_human_file_before_withdrawal`);
    await session.page.reload(); await expect(session.page.getByRole("heading", { name: issue.title, exact: true })).toBeVisible();
    await authenticatedSocket(session, "");
    await session.page.locator(`[id="comment-${file.requestId}"]`).getByRole("button", { name: `Preview ${file.name}`, exact: true }).click();
    const dialog = session.page.getByRole("dialog", { name: file.name, exact: true }); await expect(dialog.locator("pre")).toHaveText(file.bytes.toString());
    const rawCapability: unknown = authorWithdrawal ? await actor.client.requestJSON(`${issueRoot(issue.id)}/attachments/${file.receipt.attachment.id}/capabilities`,
      { method: "POST", body: { intent: "content" } }) : null;
    const oldCapability = authorWithdrawal ? parseCollaborationAttachmentCapability(rawCapability, fixture.apiUrl, issue.id,
      file.receipt.attachment.id, file.receipt.attachment.version) : null;
    if (authorWithdrawal && !oldCapability) throw new Error("The actual new Agent output has no correlated original human capability");
    const before = await outputAudit(fixture, evidence, `${label}_live_output_before_current_authority_loss`);
    expect(before.business.rows.runs!.find((row) => row.id === runId)).toMatchObject({ proof_state: "unknown", settled_at: null });
    fixture.daemon.freeze(); let frozen = true, primaryError: unknown, thawError: unknown;
    let withdrawal: unknown, requestId: string | undefined, denial: Awaited<ReturnType<typeof originalOutputDenied>> | undefined;
    try {
      const observed = workflowAuthority(await readIssue(fixture, issue)); requestId = randomUUID();
      if (authorWithdrawal) {
        withdrawal = CollaborationOperationSchema.parse(await fixture.owner.client.requestJSON(`${issueRoot(issue.id)}/participants/${actor.id}/remove`,
          { method: "POST", headers: { "Idempotency-Key": requestId }, body: observed }));
        expect(withdrawal).toMatchObject({ code: "participant_removed" });
      } else {
        const resources = CollaborationResourcesSchema.parse(await fixture.owner.client.requestJSON(versioned(`${issueRoot(issue.id)}/resources`)));
        const resource = resources.targets.find((item) => item.target_id === issue.targetId)?.resources[0];
        if (!resource?.grant_id || !resource.grant_epoch) throw new Error("The actual output run has no original live grant");
        const body = { execution_view_version: 6, expected_lifecycle_epoch: observed.expected_lifecycle_epoch,
          expected_participant_epoch: observed.expected_participant_epoch, expected_account_epoch: observed.expected_account_epoch,
          expected_grant_epoch: resource.grant_epoch };
        withdrawal = CollaborationResourceReceiptSchema.parse(await fixture.owner.client.requestJSON(`${issueRoot(issue.id)}/grants/${resource.grant_id}/revoke`,
          { method: "POST", headers: { "Idempotency-Key": requestId }, body }));
        expect(withdrawal).toMatchObject({ code: "grant_revoked", status: 200 });
        evidence.checkpoint("actual_output_grant_withdrawal_request", { request_id: requestId, grant_id: resource.grant_id, body, receipt: withdrawal });
      }
      evidence.checkpoint("actual_authority_withdrawal_while_original_output_provider_remains_live", { run_id: runId, request_id: requestId,
        actor_id: actor.id, author_withdrawal: authorWithdrawal, receipt: withdrawal, provider_closed_ms: visit.closed_ms, provider_released_ms: visit.released_ms });
      expect(visit.closed_ms).toBeNull(); expect(visit.released_ms).toBeNull();
      denial = await originalOutputDenied(fixture, file, evidence, `${label}_original_mcb_upload_replay_new_key_and_read403`);
      if (authorWithdrawal) {
        await humanFileRead(fixture, file, actor, 404, evidence, "withdrawn_output_author_has_no_metadata_content_or_download");
        expect(Date.now()).toBeLessThan(Date.parse(oldCapability!.expires_at));
        const response = await fetch(oldCapability!.url, { redirect: "error" }); expect(response.status).toBe(404);
        const bytes = Buffer.from(await response.arrayBuffer());
        const observedAt = now();
        expect(Date.parse(observedAt)).toBeLessThan(Date.parse(oldCapability!.expires_at));
        evidence.checkpoint("original_human_capability_for_new_agent_output_denied_before_expiry", { attachment_id: file.receipt.attachment.id,
          capability_url_sha256: sha(oldCapability!.url), expires_at: oldCapability!.expires_at, observed_at: observedAt, status: response.status,
          response_bytes: bytes.length, response_sha256: sha(bytes) });
        await expect(dialog).toHaveCount(0); await expect(session.page.locator(`[id="comment-${file.requestId}"]`)).toHaveCount(0);
      } else {
        await expect(dialog.locator("pre")).toHaveText(file.bytes.toString());
        await dialog.getByRole("button", { name: "Close preview", exact: true }).click(); await expect(dialog).toHaveCount(0);
      }
      await humanFileRead(fixture, file, actorFor(fixture, fixture.owner), 200, evidence, `${label}_independent_current_human_still_reads_output`);
      const after = await outputAudit(fixture, evidence, `${label}_denied_new_output_actions_before_actual_proof`);
      expect(after.business.rows.runs!.find((row) => row.id === runId)).toMatchObject({ proof_state: "unknown", settled_at: null });
      expect(after.business.rows.capabilities!.find((row) => row.run_id === runId)?.revoked_at).not.toBeNull();
      expect(after.output.files).toEqual(before.output.files); expect(after.output.refs).toEqual(before.output.refs);
      if (authorWithdrawal) {
        const prior = before.business.rows.participants!.find((row) => row.issue_id === issue.id && row.user_id === actor.id)!;
        const removed = after.business.rows.participants!.find((row) => row.issue_id === issue.id && row.user_id === actor.id)!;
        expect(removed.epoch).toBe((BigInt(z.string().parse(prior.epoch)) + 1n).toString()); expect(removed.revoked_at).not.toBeNull();
      } else {
        const prior = before.business.rows.grants!.find((row) => row.issue_id === issue.id)!;
        const removed = after.business.rows.grants!.find((row) => row.id === prior.id)!;
        expect(removed.epoch).toBe((BigInt(z.string().parse(prior.epoch)) + 1n).toString()); expect(removed.revoked_at).not.toBeNull();
      }
      fixture.daemon.thaw(); frozen = false;
    } catch (error) { primaryError = error; evidence.checkpoint("output_withdrawal_primary_failure_before_thaw", { run_id: runId, error: safeError(error) }); }
    finally {
      if (frozen) try { fixture.daemon.thaw(); }
      catch (error) { thawError = error; evidence.checkpoint("output_withdrawal_owned_thaw_failed", { run_id: runId, error: safeError(error), primary_preserved: primaryError !== undefined }); }
    }
    if (primaryError !== undefined) throw primaryError; if (thawError !== undefined) throw thawError;
    const settled = await settleOutput(fixture, file, visit, evidence);
    expect(settled.recovered.run.forced_disposition).toBe("stopped"); expect(fixture.transport.recordedOutcome(runId)).toBe(false);
    expect(settled.records.items.filter((item) => item.kind === "result")).toEqual([]);
    expect(settled.snapshot.output.events.filter((event) => event.run_id === runId && event.kind === "outcome")).toEqual([]);
    await originalOutputDenied(fixture, file, evidence, `${label}_after_original_proof403`);
    const inputReceipt = await originalInputReceipt(issue, actor, input, !authorWithdrawal);
    if (authorWithdrawal) expect(inputReceipt.targets).toEqual([]);
    await assertRetained(fixture, retained, evidence, `${label}_all_old_failed_success_and_stop_output_facts_preserved`);
    retained.push(await retainOutput(fixture, file, evidence));
    return { issue_id: issue.id, run_id: runId, original_input_request_id: input.requestId, original_command_id: input.commandId,
      output_request_id: file.requestId, attachment_id: file.receipt.attachment.id, live_read: liveRead, withdrawal_request_id: requestId,
      withdrawal, current_denials: denial, original_proof_recovery: settled.recovered, outcome_recorded: false,
      result_attachment_ids: null, output_comment_and_issue_refs_retained: true, original_input_receipt: inputReceipt };
  });
}

export async function exerciseRuntimeOutput(browser: Browser, platform: RecoveryPlatform, info: TestInfo, evidence: RecoveryEvidence) {
  // Persist the complete denominator before any gate, database, daemon or client setup.
  const run = inventory(evidence, outputCases);
  evidence.set({ platform, planned_intentional_runs: 5, planned_physical_launches: 5, planned_original_inputs: 5,
    planned_agent_output_files: 5, planned_output_cli_invocations: 6, planned_recorded_outcomes: 2, planned_recovery_launches: 0,
    planned_permit_http_calls: 10, planned_original_permit_identities: 5,
    owning_layer_only: { O03: "new-key database commit barriers and losing-object races", O04: "JSON/file UUID aliasing and newer-attempt reuse",
      O05: "required-write rollback and uncertain business commit markers", O06: "Storage-blocked withdrawal/slow read fences",
      O08: "32 cap, deterministic order, outcome-first rollback and stored-decoder matrix", O09: "CLI local-path and malformed receipt matrix" } });
  return withOutputRuntime(evidence, async (fixture) => {
    const retained: RetainedOutput[] = [], inputs: OriginalInput[] = [];
    const issue = await createRuntimeIssue(fixture, "agent-output-history", evidence);
    await withOutputSessions(browser, platform, fixture, issue, info, evidence, "output-history-failure-success-stop", async (open) => {
      const actor = actorFor(fixture, fixture.handler), session = await open(actor); await authenticatedSocket(session, "");
      await control(session, issue, "resume", evidence, "issue");
      const input = await submitInput(session, issue, "S3 original failed Agent file output input", evidence); inputs.push(input);
      const visit = await fixture.daemon.waitForVisit(1); assertOutputVisit(fixture, issue, visit, input);
      const runId = z.uuid().parse(visit.identity.run_id); fixture.transport.holdProof(runId); fixture.transport.loseReceiptACKs(runId);
      let first: GeneratedOutput | undefined;
      await run(outputCases[0], async () => {
        const requestId = randomUUID(), name = `agent-output-${requestId}.txt`, bytes = Buffer.from(`Synthetic Agent original output for ${runId}. Preserve these exact bytes and original comment UUID.\n`);
        const action = outputAction(fixture, requestId, name, bytes, `Controlled Agent original output file ${name}.`, true);
        fixture.output.dropOriginalReply(runId, requestId);
        const unknown = await invokeOutput(fixture, issue, visit, action, 1);
        expect(unknown.reply.cli.success).toBe(false); expect(unknown.reply.cli.exit_code).toBeGreaterThan(0); expect(unknown.reply.result).toBeUndefined();
        const lost = fixture.output.facts(runId, requestId); expect(lost).toHaveLength(1);
        expect(lost[0]).toMatchObject({ source: "GENUINE_CLI", status: 201, acknowledgement_lost: true });
        const original = fixture.output.original(runId, requestId), before = await outputAudit(fixture, evidence, "committed_unknown_original_agent_output_before_retry");
        expect(before.counts.agent_outputs).toBe(1); expect(before.counts.output_refs).toBe(2); expect(before.counts.comments).toBe(2);
        const retry = await invokeOutput(fixture, issue, visit, { ...action, generate: false }, 2);
        first = committedOutput(fixture, issue, visit, action, retry, evidence);
        expect(first.receipt).toEqual(original.receipt);
        expect(retry.observed.started.provider_pid).toBe(unknown.observed.started.provider_pid);
        expect(retry.observed.started.cli_pid).not.toBe(unknown.observed.started.cli_pid);
        const attempts = fixture.output.facts(runId, requestId); expect(attempts).toHaveLength(2);
        expect(attempts[1]!.input).toEqual(attempts[0]!.input); expect(attempts.map((fact) => fact.business_token_sha256)).toEqual([original.business_token_sha256, original.business_token_sha256]);
        expect(await outputAudit(fixture, evidence, "original_key_same_cli_same_file_retry_preserves_winner")).toEqual(before);
        const live = await runtimeFileRead(fixture, first, 200, evidence, "original_current_mcb_reads_its_real_committed_output");
        return { original_request_id: requestId, original_unknown_cli: unknown.reply.cli, recovered_receipt: first.receipt,
          original_file_sha256: sha(bytes), unknown_invocation: unknown.observed, retry_invocation: retry.observed, live_read: live, requests: attempts };
      });
      if (!first) throw new Error("The original output case did not retain its winning identity");
      const originalFile = first;
      await run(outputCases[1], async () => {
        const before = await outputAudit(fixture, evidence, "same_original_key_concurrent_pair_before");
        const pair = await fixture.output.concurrent(runId, originalFile.requestId);
        expect(pair.map((result) => result.reply.status)).toEqual([201, 201]);
        for (const result of pair) expect(outputReceipt.parse(wireJSON(result.reply))).toEqual(originalFile.receipt);
        expect(await outputAudit(fixture, evidence, "same_original_key_concurrent_pair_after")).toEqual(before);
        return { original_request_id: originalFile.requestId, winner_attachment_id: originalFile.receipt.attachment.id, pair: pair.map((result) => result.identity),
          retained_object_count: 1, retained_output_refs: 2 };
      });
      await run(outputCases[2], async () => {
        const before = await outputAudit(fixture, evidence, "changed_original_key_bytes_and_name_before"), changedBytes = Buffer.from(originalFile.bytes);
        changedBytes[0] = changedBytes[0] === 0x58 ? 0x59 : 0x58;
        const changed = await fixture.output.replay(runId, originalFile.requestId, { bytes: changedBytes });
        const renamed = await fixture.output.replay(runId, originalFile.requestId, { name: `changed-${originalFile.name}` });
        for (const result of [changed, renamed]) expect(refusal(result.reply)).toMatchObject({ status: 409, code: "comment_conflict" });
        const pair = await fixture.output.concurrent(runId, originalFile.requestId, changedBytes);
        expect(pair.map((result) => result.reply.status).sort()).toEqual([201, 409]);
        expect(outputReceipt.parse(wireJSON(pair.find((result) => result.reply.status === 201)!.reply))).toEqual(originalFile.receipt);
        expect(refusal(pair.find((result) => result.reply.status === 409)!.reply)).toMatchObject({ status: 409, code: "comment_conflict" });
        expect(await outputAudit(fixture, evidence, "changed_retry_and_mixed_byte_pair_preserve_all_original_effects")).toEqual(before);
        const read = await runtimeFileRead(fixture, originalFile, 200, evidence, "mixed_byte_race_winner_original_plaintext_unchanged");
        return { original_request_id: originalFile.requestId, winner_attachment_id: originalFile.receipt.attachment.id,
          original_file_sha256: sha(originalFile.bytes), changed_file_sha256: sha(changedBytes), changed: { ...changed.identity, refusal: refusal(changed.reply) },
          renamed: { ...renamed.identity, refusal: refusal(renamed.reply) }, pair: pair.map((result) => result.identity), winner_read: read };
      });
      await run(outputCases[3], async () => {
        fixture.daemon.release(visit, "failure"); const settled = await settleOutput(fixture, originalFile, visit, evidence);
        await frozenOutput(fixture, originalFile, "failure", evidence);
        const recipients = terminalRecipients(fixture, originalFile, settled.notices, "ai_failed");
        const shown = await showOutputFiles(fixture, session, originalFile, "failure", evidence, "failed-output-after-original-recovery");
        await humanFileRead(fixture, originalFile, actorFor(fixture, fixture.owner), 200, evidence, "current_creator_reads_real_failed_agent_output");
        await humanFileRead(fixture, originalFile, actorFor(fixture, fixture.outsider), 404, evidence, "outsider_cannot_read_real_agent_output_metadata_content_download");
        const missing = await fetch(`${fixture.apiUrl}${issueRoot(issue.id)}/attachments/${randomUUID()}/content`, {
          redirect: "error", headers: { Authorization: `Bearer ${fixture.outsider.client.getToken()!}` } });
        expect(missing.status).toBe(404); await missing.arrayBuffer();
        const unsupported = await fixture.output.humanWrite(runId, originalFile.requestId, fixture.owner.client.getToken()!);
        expect(refusal(unsupported)).toMatchObject({ status: 401, code: "invalid_collaboration_capability" });
        const receipt = await originalInputReceipt(issue, actor, input, true);
        const late = await originalOutputDenied(fixture, originalFile, evidence, "old_failed_output_token_cannot_replay_upload_read_or_late_new_comment_after_proof");
        retained.push(await retainOutput(fixture, originalFile, evidence));
        return { run_id: runId, outcome_recorded: true, frozen_attachment_ids: [originalFile.receipt.attachment.id], original_input_receipt: receipt,
          recipients, shown, original_proof_recovery: settled.recovered, human_runtime_writer_denied: refusal(unsupported), missing_outsider_status: missing.status, late };
      });
      await run(outputCases[4], async () => {
        const next = await submitInput(session, issue, "S3 explicit new successful Agent output input", evidence); inputs.push(next);
        expect(next.requestId).not.toBe(input.requestId); expect(next.commandId).not.toBe(input.commandId);
        const currentVisit = await fixture.daemon.waitForVisit(2); assertOutputVisit(fixture, issue, currentVisit, next);
        const currentRun = z.uuid().parse(currentVisit.identity.run_id); expect(currentRun).not.toBe(runId);
        fixture.transport.holdProof(currentRun); fixture.transport.loseReceiptACKs(currentRun);
        const file = await generateOutput(fixture, issue, currentVisit, "explicit-success", evidence);
        expect(file.receipt.attachment.id).not.toBe(originalFile.receipt.attachment.id);
        await runtimeFileRead(fixture, file, 200, evidence, "explicit_new_run_current_output_read200");
        await originalOutputDenied(fixture, originalFile, evidence, "new_successful_run_does_not_revive_old_failed_output_token");
        fixture.daemon.release(currentVisit); const settled = await settleOutput(fixture, file, currentVisit, evidence);
        await frozenOutput(fixture, file, "success", evidence); const recipients = terminalRecipients(fixture, file, settled.notices, "ai_completed");
        const shown = await showOutputFiles(fixture, session, file, "success", evidence, "new-success-output-after-original-recovery");
        await originalInputReceipt(issue, actor, next, true);
        await assertRetained(fixture, retained, evidence, "new_success_does_not_rewrite_old_failed_comment_file_result_or_notice");
        retained.push(await retainOutput(fixture, file, evidence));
        return { original_failed_run_id: runId, explicit_success_run_id: currentRun, original_request_id: next.requestId,
          attachment_id: file.receipt.attachment.id, frozen_attachment_ids: [file.receipt.attachment.id], outcome_recorded: true, recipients, shown,
          old_failed_records_sha256: digest(retained[0]!.page), original_proof_recovery: settled.recovered };
      });
      await run(outputCases[5], async () => {
        const next = await submitInput(session, issue, "S3 independent explicit Stop with real Agent output", evidence); inputs.push(next);
        const currentVisit = await fixture.daemon.waitForVisit(3); assertOutputVisit(fixture, issue, currentVisit, next);
        const currentRun = z.uuid().parse(currentVisit.identity.run_id); fixture.transport.holdProof(currentRun); fixture.transport.loseReceiptACKs(currentRun);
        const file = await generateOutput(fixture, issue, currentVisit, "explicit-stop", evidence);
        await runtimeFileRead(fixture, file, 200, evidence, "independent_stop_current_original_output_read200_before_stop");
        await session.page.reload(); await expect(session.page.getByRole("heading", { name: issue.title, exact: true })).toBeVisible();
        const stop = await control(session, issue, "stop", evidence); expect(stop.receipt.code).toBe("stop_requested");
        const settled = await settleOutput(fixture, file, currentVisit, evidence);
        expect(settled.recovered.run.forced_disposition).toBe("stopped"); expect(fixture.transport.recordedOutcome(currentRun)).toBe(false);
        expect(settled.records.items.filter((item) => item.kind === "result")).toEqual([]);
        expect(settled.snapshot.output.events.filter((event) => event.run_id === currentRun && event.kind === "outcome")).toEqual([]);
        const recipients = terminalRecipients(fixture, file, settled.notices, "ai_stopped");
        const late = await originalOutputDenied(fixture, file, evidence, "explicit_stop_then_original_proof_denies_old_output_token403");
        const replay = CollaborationControlReceiptSchema.parse(await actor.client.requestJSON(`${issueRoot(issue.id)}/control`, { method: "POST", headers: { "Idempotency-Key": stop.requestId }, body: stop.input }));
        expect(replay).toEqual(stop.receipt); await originalInputReceipt(issue, actor, next, true);
        await humanFileRead(fixture, file, actor, 200, evidence, "human_output_read_remains_authorized_after_independent_stop");
        await session.page.reload(); const comment = session.page.locator(`[id="comment-${file.requestId}"]`);
        const shown = { preview: await previewFile(session, comment, file), download: await downloadFile(session, comment, file, "explicit-stop-comment") };
        await assertRetained(fixture, retained, evidence, "independent_stop_keeps_old_failed_and_successful_outputs_immutable");
        retained.push(await retainOutput(fixture, file, evidence));
        return { run_id: currentRun, attachment_id: file.receipt.attachment.id, original_input_request_id: next.requestId,
          original_stop: stop, original_stop_replay: replay, original_proof_recovery: settled.recovered, outcome_recorded: false,
          result_attachment_ids: null, original_comment_and_issue_refs_retained: true, recipients, late, shown };
      });
    });
    await run(outputCases[6], () => withdrawOutput(browser, platform, info, evidence, fixture, true, retained, inputs));
    await run(outputCases[7], () => withdrawOutput(browser, platform, info, evidence, fixture, false, retained, inputs));
    const final = await assertRetained(fixture, retained, evidence, "complete_five_agent_output_runs_original_files_results_and_failures");
    expect(fixture.daemon.visits).toHaveLength(5); expect(fixture.generated).toHaveLength(5); expect(retained).toHaveLength(5);
    expect(inputs).toHaveLength(5); expect(new Set(inputs.map((input) => input.requestId)).size).toBe(5); expect(new Set(inputs.map((input) => input.commandId)).size).toBe(5);
    expect(final.counts).toMatchObject({ runs: 5, groups: 5, commands: 5, attempts: 5, comments: 10, uploads: 8,
      agent_outputs: 5, output_refs: 10, recorded_outcomes: 2 });
    expect(final.business.rows.comments!.filter((row) => row.author_type === "agent")).toHaveLength(5);
    expect(final.business.rows.comments!.filter((row) => row.author_type === "member")).toHaveLength(5);
    expect(final.business.rows.reservations).toHaveLength(5);
    for (const row of final.business.rows.reservations!) expect(row.released_at).not.toBeNull();
    expect(final.business.rows.capabilities).toHaveLength(5);
    for (const row of final.business.rows.capabilities!) expect(row.revoked_at).not.toBeNull();
    for (const row of final.business.rows.runs!) expect(row).toMatchObject({ proof_state: "host_exit_observed", progress_state: "settled", authorized_attempt_seq: "1" });
    const processes = fixture.processes.evidence(); expect(processes.errors).toEqual([]);
    expect(processes.facts.filter((fact) => fact.phase === "started")).toHaveLength(6);
    expect(processes.facts.filter((fact) => fact.phase === "completed")).toHaveLength(6);
    expect(new Set(processes.facts.map((fact) => fact.provider_pid)).size).toBe(5);
    expect(new Set(processes.facts.map((fact) => fact.cli_pid)).size).toBe(6);
    const history = await fixture.owner.client.requestJSON(`${issueRoot(issue.id)}/commands?limit=100`);
    const parsedHistory = CollaborationCommandHistorySchema.parse(history); expect(parsedHistory.next_cursor).toBeNull(); expect(parsedHistory.items).toHaveLength(3);
    expect(parsedHistory.items.map((item) => ({ id: item.id, state: item.state }))).toEqual([
      { id: inputs[2]!.commandId, state: "stopped" }, { id: inputs[1]!.commandId, state: "succeeded" }, { id: inputs[0]!.commandId, state: "failed" },
    ]);
    const transport = fixture.transport.evidence();
    expect(transport.facts.filter((fact) => fact.operation === "claim" && fact.status === 200)).toHaveLength(5);
    const permitFacts = transport.facts.filter((fact) => fact.operation === "permit");
    const permitOriginals = transport.originals.filter((original) => original.identity.endsWith(":permit"));
    const visitRunIds = fixture.daemon.visits.map((visit) => z.uuid().parse(visit.identity.run_id)).sort();
    expect(visitRunIds).toHaveLength(5); expect(new Set(visitRunIds).size).toBe(5);
    expect(permitFacts).toHaveLength(10); expect(permitOriginals).toHaveLength(5);
    expect([...new Set(permitFacts.map((fact) => z.uuid().parse(fact.run_id)))].sort()).toEqual(visitRunIds);
    expect(permitOriginals.map((original) => z.uuid().parse(original.identity.slice(0, -":permit".length))).sort()).toEqual(visitRunIds);
    const permitPairs = visitRunIds.map((runId) => {
      const originals = permitOriginals.filter((original) => original.identity === `${runId}:permit`);
      const pair = permitFacts.filter((fact) => fact.run_id === runId);
      expect(originals).toHaveLength(1); expect(pair).toHaveLength(2);
      const original = originals[0]!, path = `/api/collaboration/runtime/runs/${runId}/permit`;
      expect(original).toMatchObject({ method: "POST", path, first_committed_status: 200 });
      const fingerprints = pair.map((fact) => ({ run_id: fact.run_id, operation: fact.operation, method: fact.method, path: fact.path,
        authorization_sha256: hash.parse(fact.authorization_sha256), request_sha256: hash.parse(fact.request_sha256),
        response_sha256: hash.parse(fact.response_sha256), status: fact.status, forwarded: fact.forwarded,
        acknowledgement_lost: fact.acknowledgement_lost, proof_request_held: fact.proof_request_held }));
      expect(fingerprints[0]).toEqual(fingerprints[1]);
      expect(fingerprints[0]).toEqual({ run_id: runId, operation: "permit", method: "POST", path,
        authorization_sha256: hash.parse(original.authorization_sha256), request_sha256: hash.parse(original.request_sha256),
        response_sha256: hash.parse(original.first_committed_response_sha256), status: 200, forwarded: true,
        acknowledgement_lost: false, proof_request_held: false });
      return { run_id: runId, original_identity: original.identity, http_calls: pair.length, fingerprint: fingerprints[0] };
    });
    evidence.checkpoint("complete_original_permit_http_identity_and_provider_denominator", { permit_http_calls: permitFacts.length,
      original_permit_identities: permitOriginals.length, physical_provider_launches: fixture.daemon.visits.length,
      provider_run_ids: visitRunIds, pairs: permitPairs });
    expect(transport.errors).toEqual([]); expect(fixture.output.evidence().errors).toEqual([]);
    return { platform, final, intentional_runs: 5, physical_provider_launches: 5, output_cli_invocations: 6, recovery_launches: 0,
      permit_http_calls: permitFacts.length, original_permit_identities: permitOriginals.length,
      output_attachment_ids: fixture.generated.map((file) => file.receipt.attachment.id), original_output_request_ids: fixture.generated.map((file) => file.requestId),
      original_run_ids: fixture.generated.map((file) => file.runId), recorded_result_runs: retained.filter((item) => item.outcome_event !== null).map((item) => item.file.runId),
      nonrecorded_stop_or_withdrawal_runs: retained.filter((item) => item.outcome_event === null).map((item) => item.file.runId),
      command_history_states: parsedHistory.items.map((item) => ({ command_id: item.id, state: item.state })), inputs: inputs.map((input) => ({ request_id: input.requestId, command_id: input.commandId })) };
  });
}
