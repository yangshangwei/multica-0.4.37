import { lstatSync, realpathSync } from "node:fs";
import { createConnection, type Socket } from "node:net";
import { dirname, isAbsolute, normalize, relative, sep } from "node:path";

export const STAGE_CONTROL_FRAME_BYTES = 64 * 1024;
export const STAGE_CONTROL_TIMEOUT_MS = 20_000;
type Candidate = { candidate_id: string; version: string; content_digest: string };
type ExecutionIdentity = { issue_id: string; approval_decision_id: string; candidate: Candidate; target_digest: string };
export type StageControlRequest = { request_id: string } & (
  | { action: "source.configure"; workspace_id: string; deployment_integration_id: string; metric_integration_id: string;
      environment: string; remote_instance: string; remote_tenant: string; remote_target: string; conditional_version: string;
      current_artifact_digest: string | null; query_id: string; query_digest: string; rule_id: string; rule_digest: string;
      metric_freshness_seconds: number; config_revision?: string }
  | { action: "source.behavior"; behavior: "succeeded" | "failed" | "unknown" | "not_started" | "panic" | "delayed";
      delay_ms?: number; external_operation_id?: string }
  | ({ action: "execution.prepare" | "execution.receipt" } & ExecutionIdentity)
  | { action: "execution.observe"; intent_id: string; native_id: string; native_version: string;
      result: "succeeded" | "failed" | "unknown" | "not_started"; external_operation_id?: string | null }
  | { action: "metrics.observe"; deployment_id: string; native_id: string; native_version: string;
      condition: "anomaly" | "healthy" | "unknown"; window_start: string; window_end: string; observed_at: string; valid_until: string; summary?: string }
);
export interface StageControlInvocation {
  intent_id: string; kind: "deployment" | "rollback"; target_digest: string; artifact_digest: string;
  external_request_key: string; conditional_version: string;
}
export type StageControlReply = { request_id: string; calls: { count: number; last: StageControlInvocation | null } } & (
  | { ok: true; result: Record<string, unknown> }
  | { ok: false; code: string }
);

function object(value: unknown): asserts value is Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) throw new Error("Invalid stage control object");
}
function keys(value: Record<string, unknown>, required: string[], optional: string[] = []) {
  if (required.some((key) => !Object.hasOwn(value, key)) || Object.keys(value).some((key) => !required.includes(key) && !optional.includes(key))) {
    throw new Error("Invalid stage control fields");
  }
}
function text(value: unknown, maximum = 256): asserts value is string {
  if (typeof value !== "string" || value.length === 0 || value.includes("\0") || Buffer.byteLength(value, "utf8") > maximum ||
    [...value].some((char) => char.length === 1 && char.charCodeAt(0) >= 0xd800 && char.charCodeAt(0) <= 0xdfff)) throw new Error("Invalid stage control text");
}
function uuid(value: unknown) {
  if (typeof value !== "string" || !/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(value) || value === "00000000-0000-0000-0000-000000000000") throw new Error("Invalid stage control identity");
}
function digest(value: unknown) {
  if (typeof value !== "string" || !/^[0-9a-f]{64}$/.test(value)) throw new Error("Invalid stage control digest");
}
function version(value: unknown) {
  if (typeof value !== "string" || !/^[1-9][0-9]{0,18}$/.test(value) || BigInt(value) > 9223372036854775807n) throw new Error("Invalid stage control version");
}
function integer(value: unknown, minimum: number, maximum: number) {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value < minimum || value > maximum) throw new Error("Invalid stage control integer");
}
function timestamp(value: unknown) {
  if (typeof value !== "string" || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,6})?Z$/.test(value) || !Number.isFinite(Date.parse(value))) throw new Error("Invalid stage control time");
}
function choice(value: unknown, values: string[]) {
  if (typeof value !== "string" || !values.includes(value)) throw new Error("Invalid stage control choice");
}

/** Encode the frozen test-only union; E2E never owns the shutdown action. */
export function encodeStageControlRequest(value: StageControlRequest): string {
  object(value); uuid(value.request_id);
  const envelope = ["request_id", "action"];
  switch (value.action) {
    case "source.configure":
      keys(value, [...envelope, "workspace_id", "deployment_integration_id", "metric_integration_id", "environment", "remote_instance", "remote_tenant", "remote_target",
        "conditional_version", "current_artifact_digest", "query_id", "query_digest", "rule_id", "rule_digest", "metric_freshness_seconds"], ["config_revision"]);
      uuid(value.workspace_id); uuid(value.deployment_integration_id); uuid(value.metric_integration_id);
      if (value.deployment_integration_id === value.metric_integration_id) throw new Error("Stage execution and metric sources must differ");
      text(value.environment, 128); text(value.remote_instance); text(value.remote_tenant); text(value.remote_target);
      text(value.query_id); text(value.rule_id); digest(value.query_digest); digest(value.rule_digest); version(value.conditional_version);
      if (value.current_artifact_digest !== null) digest(value.current_artifact_digest);
      if (value.config_revision !== undefined) version(value.config_revision);
      integer(value.metric_freshness_seconds, 1, 3600);
      break;
    case "source.behavior":
      keys(value, [...envelope, "behavior"], ["delay_ms", "external_operation_id"]);
      choice(value.behavior, ["succeeded", "failed", "unknown", "not_started", "panic", "delayed"]);
      if (value.delay_ms !== undefined) integer(value.delay_ms, 0, 5000);
      if (value.external_operation_id !== undefined) text(value.external_operation_id);
      break;
    case "execution.prepare": case "execution.receipt":
      keys(value, [...envelope, "issue_id", "approval_decision_id", "candidate", "target_digest"]);
      uuid(value.issue_id); uuid(value.approval_decision_id); digest(value.target_digest); object(value.candidate);
      keys(value.candidate, ["candidate_id", "version", "content_digest"]);
      uuid(value.candidate.candidate_id); version(value.candidate.version); digest(value.candidate.content_digest);
      break;
    case "execution.observe":
      keys(value, [...envelope, "intent_id", "native_id", "native_version", "result"], ["external_operation_id"]);
      uuid(value.intent_id); text(value.native_id); version(value.native_version); choice(value.result, ["succeeded", "failed", "unknown", "not_started"]);
      if (value.external_operation_id !== undefined && value.external_operation_id !== null) text(value.external_operation_id);
      if ((value.result === "succeeded" || value.result === "failed") && !value.external_operation_id) throw new Error("Terminal stage execution needs its native identity");
      break;
    case "metrics.observe":
      keys(value, [...envelope, "deployment_id", "native_id", "native_version", "condition", "window_start", "window_end", "observed_at", "valid_until"], ["summary"]);
      uuid(value.deployment_id); text(value.native_id); version(value.native_version); choice(value.condition, ["anomaly", "healthy", "unknown"]);
      for (const time of [value.window_start, value.window_end, value.observed_at, value.valid_until]) timestamp(time);
      if (value.summary !== undefined && value.summary !== "") text(value.summary, 2000);
      break;
    default: throw new Error("Unsupported stage control action");
  }
  const encoded = JSON.stringify(value);
  if (Buffer.byteLength(encoded, "utf8") > STAGE_CONTROL_FRAME_BYTES) throw new Error("Stage control frame is too large");
  return `${encoded}\n`;
}

/** JSON.parse alone silently accepts duplicate keys, including escaped aliases. */
function strictJSON(raw: string): unknown {
  const decoded: unknown = JSON.parse(raw);
  const tokens = raw.match(/"(?:[^"\\]|\\.)*"|[{}[\]:,]|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|true|false|null/g) ?? [];
  let index = 0;
  const visit = () => {
    const token = tokens[index++];
    if (token === "{") {
      const seen = new Set<string>();
      while (tokens[index] !== "}") {
        const key: string = JSON.parse(tokens[index++]!);
        if (seen.has(key)) throw new Error("Duplicate stage control field");
        seen.add(key); index++; visit();
        if (tokens[index] !== ",") break;
        index++;
      }
      index++;
    } else if (token === "[") {
      while (tokens[index] !== "]") { visit(); if (tokens[index] !== ",") break; index++; }
      index++;
    }
  };
  visit();
  if (index !== tokens.length) throw new Error("Invalid stage control JSON");
  return decoded;
}

export function parseStageControlReply(raw: string, expectedRequest: string): StageControlReply {
  if (Buffer.byteLength(raw, "utf8") > STAGE_CONTROL_FRAME_BYTES) throw new Error("Stage control reply is too large");
  const value = strictJSON(raw); object(value); uuid(value.request_id);
  if (value.request_id !== expectedRequest) throw new Error("Stage control reply identity mismatch");
  if (value.ok === true) { keys(value, ["request_id", "ok", "result", "calls"]); object(value.result); }
  else if (value.ok === false) { keys(value, ["request_id", "ok", "code", "calls"]); text(value.code, 128); }
  else throw new Error("Invalid stage control outcome");
  object(value.calls); keys(value.calls, ["count", "last"]); integer(value.calls.count, 0, Number.MAX_SAFE_INTEGER);
  const last = value.calls.last;
  if (last !== null) {
    object(last); keys(last, ["intent_id", "kind", "target_digest", "artifact_digest", "external_request_key", "conditional_version"]);
    uuid(last.intent_id); choice(last.kind, ["deployment", "rollback"]); digest(last.target_digest); digest(last.artifact_digest);
    text(last.external_request_key); text(last.conditional_version, 256);
  }
  // The closed envelope and every invocation field have been checked above.
  return value as StageControlReply;
}

function privateSocket(path: string) {
  if (!isAbsolute(path) || normalize(path) !== path || path.includes("\0") || Buffer.byteLength(path, "utf8") > 103 || realpathSync(dirname(path)) !== dirname(path)) throw new Error("Stage control socket is not a canonical private path");
  const within = relative(process.cwd(), path);
  if (!within.startsWith(`..${sep}`) && within !== "..") throw new Error("Stage control socket must be outside the checkout");
  const directory = lstatSync(dirname(path)), socket = lstatSync(path);
  if (!directory.isDirectory() || directory.mode % 0o1000 !== 0o700 || !socket.isSocket() || socket.mode % 0o1000 !== 0o600 ||
    directory.uid !== process.getuid?.() || socket.uid !== process.getuid?.()) throw new Error("Stage control socket is not privately owned");
}

/** One explicit send, one correlated reply. A failed send never replays prepare. */
export class StageExecutionControl {
  private readonly sockets = new Set<Socket>();
  private readonly pending = new Set<string>();
  private closed = false;
  constructor(private readonly path: string) {}

  assertReady() {
    try { privateSocket(this.path); }
    catch { throw new Error("Stage control requires an existing privately owned socket outside the checkout (maximum 103 path bytes)"); }
    if (this.closed) throw new Error("Stage control is closed");
  }

  async send(request: StageControlRequest): Promise<StageControlReply> {
    const frame = encodeStageControlRequest(request);
    this.assertReady();
    if (this.closed || this.pending.has(request.request_id)) throw new Error("Stage control request is already pending or closed");
    this.pending.add(request.request_id);
    return new Promise((resolve, reject) => {
      let done = false, bytes = Buffer.alloc(0);
      const socket = createConnection(this.path);
      this.sockets.add(socket);
      const finish = (value?: StageControlReply) => {
        if (done) return;
        done = true; clearTimeout(timer); this.pending.delete(request.request_id); this.sockets.delete(socket); socket.destroy();
        if (value) resolve(value);
        else reject(new Error(`Stage control result unconfirmed; retain original request ${request.request_id}`));
      };
      const timer = setTimeout(() => finish(), STAGE_CONTROL_TIMEOUT_MS);
      socket.once("connect", () => socket.write(frame));
      socket.on("error", () => finish()); socket.once("close", () => finish());
      socket.on("data", (chunk: Buffer) => {
        if (bytes.length + chunk.length > STAGE_CONTROL_FRAME_BYTES + 1) { finish(); return; }
        bytes = Buffer.concat([bytes, chunk]);
        const end = bytes.indexOf(10);
        if (end < 0) return;
        try {
          if (end !== bytes.length - 1) throw new Error("Extra stage control data");
          const raw = new TextDecoder("utf-8", { fatal: true }).decode(bytes.subarray(0, end));
          finish(parseStageControlReply(raw, request.request_id));
        } catch { finish(); }
      });
    });
  }

  close() { this.closed = true; for (const socket of this.sockets) socket.destroy(); }
}
