// Counter-example tests for the PAY-241 S5 sampling statistics. Each case is a
// way the measurement could look better than it is; the pure functions must
// refuse it. Run with:
//   node --experimental-strip-types --test e2e/fixtures/collaboration-performance-stats.node-test.mjs
import assert from "node:assert/strict";
import { Buffer } from "node:buffer";
import test from "node:test";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { createRequire } from "node:module";
import process from "node:process";
import { setImmediate } from "node:timers";
import { URL } from "node:url";
import { runInNewContext } from "node:vm";
import ts from "typescript";
import * as stats from "./collaboration-performance-stats.ts";
import {
  PERFORMANCE_PROTOCOL, accountPropagation, accountWriteback, barrierSatisfied, classifyObserverSample, createFrameLedger,
  expectedEventSeq, failedEventRows, matchExpected, nearestRank, nearestRankIndex, notRunEventRows, phaseJitterMs,
  pickWritebackReaders, preregistrationMismatches, propagationVerdict, recordFrame, registrationWaitMs, reportQuantiles,
  revisionAnomaly, summarize, writebackVerdict,
} from "./collaboration-performance-stats.ts";

const { AbortController, AbortSignal } = globalThis;

// Compile the actual harness without importing Playwright or its DB fixtures.
// Every external boundary is either a Node builtin or an explicit test double;
// unexpected imports and all network requests fail closed.
function heartbeatHarness({ fetch = () => { throw new Error("Unexpected network request"); }, modules = {},
  WebSocket = class { constructor() { throw new Error("Test socket connection refused"); } } } = {}) {
  const require = createRequire(import.meta.url);
  const jobs = new Map();
  let nextJob = 0;
  const exports = {};
  const source = ts.transpileModule(readFileSync(new URL("./collaboration-performance.ts", import.meta.url), "utf8"), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, esModuleInterop: true },
  }).outputText;
  runInNewContext(source, { exports, process, Buffer, AbortController, AbortSignal, WebSocket,
    setTimeout: (fn, ms) => { const id = ++nextJob; jobs.set(id, { fn, ms }); return id; },
    clearTimeout: (id) => jobs.delete(id),
    fetch,
    require: (name) => {
      if (name.startsWith("node:")) return require(name);
      if (name === "./collaboration-performance-stats") return stats;
      if (name in modules) return modules[name];
      if (["pg", "../fixtures", "./collaboration-host-runtime"].includes(name)) return {};
      throw new Error(`Unexpected harness import: ${name}`);
    },
  }, { filename: "collaboration-performance.test.cjs" });
  return { exports, jobs, tick() { const entry = jobs.entries().next().value; assert.ok(entry, "a heartbeat must be scheduled"); jobs.delete(entry[0]); entry[1].fn(); } };
}

const heartbeatRuntime = () => ({ runtimeId: "11111111-2222-4333-8444-555555555555", workspaceId: "workspace", daemonId: "daemon",
  credential: { principal_user_id: "owner", daemon_token: "mdt_synthetic_owner_only", expires_at: "2026-11-15T00:00:00.000Z" } });
const flush = () => new Promise((resolve) => setImmediate(resolve));

test("heartbeat uses the enrolled owner's scoped credential, repeats below freshness, and stops its timer", async () => {
  const h = heartbeatHarness(), runtime = heartbeatRuntime(), records = [], calls = [];
  const io = { now: () => 10, wallNow: () => Date.parse("2026-11-14T00:00:00Z"), record: (row) => records.push(row),
    // DaemonHeartbeat intentionally omits runtime_id from the HTTP acknowledgement.
    send: async (token, body, signal) => { calls.push({ token, body, signal }); return { status: 200, body: { status: "ok" } }; } };
  const heartbeat = h.exports.startRuntimeHeartbeat(runtime, "owner", io);
  await heartbeat.ready;
  assert.equal(heartbeat.failure(), null);
  assert.equal(calls[0].token, runtime.credential.daemon_token);
  assert.equal(calls[0].body.runtime_id, runtime.runtimeId);
  assert.equal(calls[0].body.supports_batch_import, true);
  assert.equal(h.jobs.values().next().value.ms, 15000);
  assert.ok(15000 + 10000 < 150000);
  h.tick(); await flush();
  assert.equal(calls.length, 2);
  assert.equal(records.filter((row) => row.kind === "heartbeat_result").length, 2);
  assert.ok(records.every((row) => row.owner_user_id === "owner"));
  assert.ok(!JSON.stringify(records).includes(runtime.credential.daemon_token));
  await heartbeat.stop(); await heartbeat.stop();
  assert.equal(h.jobs.size, 0);
});

test("heartbeat refuses another owner or an expired credential before any transport call", async () => {
  for (const mismatch of ["owner", "expired"]) {
    const h = heartbeatHarness(), runtime = heartbeatRuntime(); let sent = 0;
    if (mismatch === "owner") runtime.credential.principal_user_id = "reader";
    if (mismatch === "expired") runtime.credential.expires_at = "2026-11-13T00:00:00Z";
    assert.throws(() => h.exports.startRuntimeHeartbeat(runtime, "owner", {
      now: () => 0, wallNow: () => Date.parse("2026-11-14T00:00:00Z"), record: () => {}, send: async () => { sent += 1; },
    }), /runtime_heartbeat_(owner_mismatch|credential_expired)/);
    assert.equal(sent, 0); assert.equal(h.jobs.size, 0);
  }
});

test("heartbeat records HTTP, malformed acknowledgement, and transport failures without leaking response content", async () => {
  for (const result of ["http", "wrong_status", "gone", "network"]) {
    const h = heartbeatHarness(), records = [], runtime = heartbeatRuntime();
    const heartbeat = h.exports.startRuntimeHeartbeat(runtime, "owner", {
      now: () => 12, wallNow: () => Date.parse("2026-11-14T00:00:00Z"), record: (row) => records.push(row),
      send: async () => {
        if (result === "network") throw new Error("private credential");
        return { status: result === "http" ? 403 : 200, body: {
          status: result === "wrong_status" ? "unknown" : "ok", runtime_gone: result === "gone", private: "private credential" } };
      },
    });
    await heartbeat.ready;
    assert.match(heartbeat.failure(), /^runtime_heartbeat_/);
    assert.ok(!JSON.stringify(records).includes("private credential"));
    assert.equal(records.filter((row) => row.kind === "heartbeat_result").length, 1);
    await heartbeat.stop(); assert.equal(h.jobs.size, 0);
  }
});

test("heartbeat cleanup aborts and awaits an in-flight request without treating cancellation as a failed beat", async () => {
  const h = heartbeatHarness(), runtime = heartbeatRuntime(), records = [];
  let rejectRequest, signal;
  const heartbeat = h.exports.startRuntimeHeartbeat(runtime, "owner", {
    now: () => 20, wallNow: () => Date.parse("2026-11-14T00:00:00Z"), record: (row) => records.push(row),
    send: async (_token, _body, currentSignal) => { signal = currentSignal; return new Promise((_resolve, reject) => { rejectRequest = reject; }); },
  });
  let stopped = false;
  const stopping = heartbeat.stop().then(() => { stopped = true; });
  await flush();
  assert.equal(signal.aborted, true); assert.equal(stopped, false);
  rejectRequest(new Error("cancelled"));
  await stopping; await heartbeat.ready;
  assert.equal(stopped, true); assert.equal(heartbeat.failure(), null);
  assert.equal(h.jobs.size, 0);
  assert.equal(records.at(-1).error_code, "heartbeat_cancelled");
});

test("heartbeat remains single-flight and keeps a failed beat after a later successful retry", async () => {
  const h = heartbeatHarness(), runtime = heartbeatRuntime(); let complete, calls = 0;
  const heartbeat = h.exports.startRuntimeHeartbeat(runtime, "owner", {
    now: () => 20, wallNow: () => Date.parse("2026-11-14T00:00:00Z"), record: () => {},
    send: async () => { calls += 1; return new Promise((resolve) => { complete = resolve; }); },
  });
  assert.equal(calls, 1); assert.equal(h.jobs.size, 0);
  complete({ status: 503, body: null }); await heartbeat.ready;
  assert.equal(heartbeat.failure(), "runtime_heartbeat_http_503");
  h.tick(); await flush();
  assert.equal(calls, 2); assert.equal(h.jobs.size, 0);
  complete({ status: 200, body: { status: "ok" } }); await flush();
  assert.equal(heartbeat.failure(), "runtime_heartbeat_http_503");
  await heartbeat.stop(); assert.equal(h.jobs.size, 0);
});

test("heartbeat log failure settles readiness and cleanup instead of leaking a background rejection", async () => {
  const h = heartbeatHarness();
  const heartbeat = h.exports.startRuntimeHeartbeat(heartbeatRuntime(), "owner", {
    now: () => 0, wallNow: () => Date.parse("2026-11-14T00:00:00Z"), record: () => { throw new Error("disk unavailable"); },
    send: async () => { assert.fail("a failed start log must not send"); },
  });
  await heartbeat.ready;
  assert.equal(heartbeat.failure(), "runtime_heartbeat_log_failed");
  await heartbeat.stop(); assert.equal(h.jobs.size, 0);
});

test("writeback heartbeat and offline failures keep both full denominators and finish all cleanup", async () => {
  for (const mode of ["heartbeat_refused", "offline", "offline_socket_close_failed", "offline_daemon_close_failed", "offline_gate_restore_failed", "daemon_setup_failed"]) {
    const directory = mkdtempSync("/tmp/multica-s5-heartbeat-test-");
    try {
      const events = [], requests = [], runtime = heartbeatRuntime();
      runtime.credential.expires_at = new Date(Date.now() + 86400000).toISOString();
      const gate = { enabled: true, creation_enabled: true, host_execution_enabled: false, epoch: "4" };
      const target = { id: "target", target_epoch: "1", config_version: "1", control: { queue_state: "active", control_version: "1" }, hold_version: "1" };
      const issue = { id: ISSUE, status: "todo", collaboration_revision: "1", lifecycle_epoch: "1", realtime: { event_seq: "1" },
        creator: { user_id: "owner" }, handler: { user_id: "owner" }, capabilities: { read: true }, session: { queue_state: "active" },
        execution: { targets: [target], execution_gate: { epoch: "4" }, control: { control_version: "1" }, target_set_version: "1" } };
      const h = heartbeatHarness({
        fetch: async (url, options) => {
          requests.push({ url, options });
          let status = 200, body;
          if (url.endsWith("/api/workspaces")) body = { id: runtime.workspaceId, slug: "execution" };
          else if (url.endsWith("/api/daemon/heartbeat")) {
            assert.equal(options.headers.Authorization, `Bearer ${runtime.credential.daemon_token}`);
            assert.equal(JSON.parse(options.body).runtime_id, runtime.runtimeId);
            assert.equal(options.signal.aborted, false);
            status = mode === "heartbeat_refused" ? 403 : 200;
            body = { status: "ok" };
          } else if (url.includes("/api/runtimes?owner=me")) {
            assert.equal(options.headers.Authorization, "Bearer owner_human_synthetic");
            assert.equal(options.headers["X-Workspace-ID"], runtime.workspaceId);
            body = [{ id: runtime.runtimeId, owner_id: "owner", workspace_id: runtime.workspaceId, daemon_id: runtime.daemonId,
              status: "offline", last_seen_at: new Date().toISOString() }];
          } else if (url.endsWith("/control")) body = { queue_state: "active" };
          else if (url.includes(`/issues/${ISSUE}?`)) body = issue;
          else assert.fail(`Unexpected fake request ${url}`);
          return { status, text: async () => JSON.stringify(body), headers: new Map() };
        },
        modules: {
          pg: { Client: class { async connect() {} async end() {} async query() { return { rows: [gate] }; } } },
          "./collaboration-host-runtime": {
            issueRoot: (id) => `/api/collaboration/issues/${id}`, versioned: (path) => `${path}?execution_view_version=6`, hostAuthority: () => ({}),
            openHostAcceptanceGate: async () => async () => { events.push("gate_restored"); if (mode === "offline_gate_restore_failed") throw new Error("restore failed"); },
            enrollHostRuntime: async () => runtime, waitForHostRuntime: async () => {}, adoptHostTarget: async () => ({ targetId: target.id }),
            startHostDaemon: async () => {
              if (mode === "daemon_setup_failed") throw new Error("synthetic startup failure");
              return { errors: [], visits: [], close: async () => { events.push("daemon_closed");
                if (mode === "offline_daemon_close_failed") throw new Error("daemon close failed");
                return { exit_code: 0, errors: [], ledger: [] }; } };
            },
          },
        },
      });
      const client = { setWorkspaceId() {}, setWorkspaceSlug() {}, requestJSON: async () => ({ id: "agent", name: "Agent" }) };
      const sessions = Array.from({ length: 100 }, (_unused, index) => ({ open: true, client, user_id: index === 0 ? "owner" : `reader${index}`,
        token: index === 0 ? "owner_human_synthetic" : `reader_token_${index}` }));
      const outcome = await h.exports.runWriteback({ runDir: directory, apiUrl: "http://fake.invalid", manifestSha256: "test", deploymentId: "test",
        manifest: { run_id: "test", purpose: "rehearsal", gate_original: gate } }, { issueId: ISSUE, sessions,
        close: async () => { events.push("sockets_closed"); if (mode === "offline_socket_close_failed") throw new Error("close failed"); } });
      assert.equal(outcome.pass, false); assert.ok(outcome.aborted);
      assert.equal(outcome.summary.business_result.denominator, 100);
      assert.equal(outcome.summary.finalization.denominator, 100);
      const samples = readFileSync(join(directory, "writeback-samples.jsonl"), "utf8").trim().split("\n").map(JSON.parse);
      assert.equal(samples.length, 202);
      assert.equal(samples.filter((row) => row.phase === "formal").length, 200);
      assert.ok(samples.every((row) => row.result === "not_run" || row.result === "missing"));
      if (mode.startsWith("offline")) assert.equal(samples[0].error_code, "runtime_offline");
      assert.equal(requests.filter((entry) => entry.url.endsWith("/comments")).length, 0);
      assert.equal(events[0], "sockets_closed"); assert.equal(events.at(-1), "gate_restored");
      if (mode.startsWith("offline")) assert.equal(events[1], "daemon_closed");
      if (mode === "offline_socket_close_failed") assert.ok(outcome.summary.setup.cleanup_failures.includes("socket_close_failed"));
      if (mode === "offline_daemon_close_failed") assert.ok(outcome.summary.setup.cleanup_failures.includes("daemon_close_failed"));
      if (mode === "offline_gate_restore_failed") assert.ok(outcome.summary.setup.cleanup_failures.includes("gate_restore_failed"));
      assert.equal(h.jobs.size, 0, "heartbeat timer must be gone after failed writeback");
      const log = readFileSync(join(directory, "writeback-runs.jsonl"), "utf8");
      assert.ok(!log.includes(runtime.credential.daemon_token)); assert.ok(!log.includes("owner_human_synthetic"));
    } finally { rmSync(directory, { recursive: true, force: true }); }
  }
});

async function successfulWritebackScenario({ disconnect = null } = {}) {
  const directory = mkdtempSync("/tmp/multica-s5-writeback-success-");
  try {
    const runtime = heartbeatRuntime();
    runtime.credential.expires_at = new Date(Date.now() + 86400000).toISOString();
    const gate = { enabled: true, creation_enabled: true, host_execution_enabled: false, epoch: "4" };
    const target = { id: "target", target_epoch: "1", config_version: "1", control: { queue_state: "active", control_version: "1" }, hold_version: "1" };
    const client = { setWorkspaceId() {}, setWorkspaceSlug() {}, requestJSON: async () => ({ id: "agent", name: "Agent" }) };
    const sessions = Array.from({ length: 100 }, (_unused, index) => ({ actor_index: index, open: true, client,
      user_id: index === 0 ? "owner" : `reader${index}`, token: `synthetic_token_${index}`, ledger: createFrameLedger(ISSUE), waiters: new Set(),
      socket_id: `socket_${index}`, connections: 1, closed_ms: null }));
    if (disconnect === "boundary") { sessions[99].open = false; sessions[99].closed_ms = 1; }
    const issue = { id: ISSUE, status: "todo", collaboration_revision: "1", lifecycle_epoch: "1", realtime: { event_seq: "1" },
      creator: { user_id: "owner" }, handler: { user_id: "owner" }, capabilities: { read: true }, session: { queue_state: "active" },
      execution: { targets: [target], runs: [], execution_gate: { epoch: "4" }, control: { control_version: "1" }, target_set_version: "1" } };
    const comments = new Map(), records = new Map(), events = [], visits = [], requests = [];
    let triggerIndex = 0, activeTrigger = 0, activeRun;
    const daemon = { errors: [], visits,
      act: async (visit, actions) => {
        assert.equal(visit.identity.run_id, activeRun);
        const [action] = actions;
        assert.equal(actions.length, 1);
        comments.set(action.request_id, { id: action.request_id, issue_id: ISSUE, content: action.content,
          author_type: "agent", author_agent: { agent_id: "agent" } });
        records.set(activeRun, [{ kind: "comment", comment_id: action.request_id }]);
        for (const session of sessions) {
          recordFrame(session.ledger, changed(triggerIndex), 0);
          for (const wake of [...session.waiters]) wake();
        }
        return [{ request_id: action.request_id }];
      },
      release: (visit) => {
        assert.equal(visit.identity.run_id, activeRun);
        issue.execution.runs.push({ id: activeRun, progress_state: "settled", proof_state: "host_exit_observed", launch_count: "1", business_outcome: "success" });
        records.get(activeRun).push({ kind: "result", body: `Host fixture result for ${activeRun}` });
        if ((disconnect === "timed" || disconnect === "recovered") && triggerIndex === 2) {
          sessions[99].open = false; sessions[99].closed_ms = 1;
          if (disconnect === "recovered") {
            sessions[99].open = true; sessions[99].closed_ms = null;
            sessions[99].socket_id = "replacement_socket"; sessions[99].connections += 1;
          }
        }
      },
      close: async () => { events.push("daemon_closed"); return { exit_code: 0, errors: [], ledger: visits.map(() => ({ phase: "proof_acknowledged" })) }; },
    };
    const h = heartbeatHarness({ fetch: async (url, options) => {
      const path = new URL(url).pathname;
      requests.push({ path, method: options.method, token: options.headers.Authorization });
      let status = 200, body;
      if (path === "/api/workspaces") body = { id: runtime.workspaceId, slug: "execution" };
      else if (path === "/api/daemon/heartbeat") {
        assert.equal(options.headers.Authorization, `Bearer ${runtime.credential.daemon_token}`);
        assert.deepEqual(JSON.parse(options.body), { runtime_id: runtime.runtimeId, supports_batch_import: true });
        body = { status: "ok" };
      } else if (path === "/api/runtimes") {
        assert.equal(options.headers.Authorization, "Bearer synthetic_token_0");
        assert.equal(options.headers["X-Workspace-ID"], runtime.workspaceId);
        body = [{ id: runtime.runtimeId, owner_id: "owner", workspace_id: runtime.workspaceId, daemon_id: runtime.daemonId,
          status: "online", last_seen_at: new Date().toISOString() }];
      } else if (path.endsWith("/control")) body = { queue_state: "active" };
      else if (path.endsWith("/comments") && options.method === "POST") {
        activeTrigger = Math.max(0, triggerIndex - 1);
        // Warmup is actor 0; formal run k is actor k.
        assert.equal(options.headers.Authorization, `Bearer synthetic_token_${activeTrigger}`);
        const commandId = `command_${triggerIndex}`;
        activeRun = `00000000-1111-4222-8333-${String(++triggerIndex).padStart(12, "0")}`;
        visits.push({ agent_id: "agent", identity: { run_id: activeRun, issue_id: ISSUE }, inputs: [{ command_id: commandId }] });
        status = 201; body = { targets: [{ state: "waiting", command_id: commandId }] };
      } else if (path.endsWith("/comments")) body = { items: [...comments.values()] };
      else if (path.includes("/comments/")) {
        assert.notEqual(options.headers.Authorization, "Bearer synthetic_token_0");
        assert.notEqual(options.headers.Authorization, `Bearer synthetic_token_${activeTrigger}`);
        body = comments.get(path.split("/").at(-1)); status = body ? 200 : 404;
      } else if (path.endsWith("/records")) body = { items: records.get(path.split("/").at(-2)) ?? [] };
      else if (path === `/api/collaboration/issues/${ISSUE}`) body = issue;
      else assert.fail(`Unexpected fake request ${url}`);
      return { status, text: async () => JSON.stringify(body ?? null), headers: new Map() };
    }, modules: {
      pg: { Client: class { async connect() {} async end() {} async query() { return { rows: [gate] }; } } },
      "./collaboration-host-runtime": {
        issueRoot: (id) => `/api/collaboration/issues/${id}`, versioned: (path) => `${path}${path.includes("?") ? "&" : "?"}execution_view_version=6`, hostAuthority: () => ({}),
        openHostAcceptanceGate: async () => async () => { events.push("gate_restored"); }, enrollHostRuntime: async () => runtime,
        waitForHostRuntime: async () => {}, adoptHostTarget: async () => ({ targetId: target.id }), startHostDaemon: async () => daemon,
      },
    } });
    const outcome = await h.exports.runWriteback({ runDir: directory, apiUrl: "http://fake.invalid", manifestSha256: "test", deploymentId: "test",
      manifest: { run_id: "test", purpose: "rehearsal", gate_original: gate } }, { issueId: ISSUE, sessions,
      close: async () => { events.push("sockets_closed"); } });
    const rows = (name) => readFileSync(join(directory, name), "utf8").trim().split("\n").map(JSON.parse);
    assert.equal(h.jobs.size, 0, "all heartbeat and hint timers must be settled");
    assert.deepEqual(events, ["sockets_closed", "daemon_closed", "gate_restored"]);
    return { outcome, samples: rows("writeback-samples.jsonl"), runs: rows("writeback-runs.jsonl"), requests, visits };
  } finally { rmSync(directory, { recursive: true, force: true }); }
}

test("actual HTTP heartbeat acknowledgement permits 101 independently attributed writeback runs and full cleanup", async () => {
  const { outcome, samples, runs, visits } = await successfulWritebackScenario();
  assert.equal(outcome.pass, true); assert.equal(outcome.aborted, null);
  assert.equal(samples.length, 202);
  assert.equal(samples.filter((row) => row.phase === "formal").length, 200);
  assert.equal(new Set(visits.map((visit) => visit.identity.run_id)).size, 101);
  assert.equal(new Set(samples.filter((row) => row.phase === "formal").map((row) => row.trigger_user_id)).size, 100);
  assert.ok(samples.every((row) => row.result === "converged"));
  assert.ok(samples.every((row) => row.runtime_before_trigger.read_end_ms <= row.t0_ms && row.read_start_ms >= row.t0_ms
    && row.read_end_ms >= row.read_start_ms && row.latency_ms === row.read_end_ms - row.t0_ms));
  assert.equal(runs.filter((row) => row.kind === "runtime_before_trigger").length, 101);
  assert.ok(runs.filter((row) => row.kind === "runtime_before_trigger").every((row) => row.open_sockets === 100));
});

test("writeback cannot pass after a disconnected observer fails its boundary reconnect", async () => {
  const { outcome, samples, requests } = await successfulWritebackScenario({ disconnect: "boundary" });
  assert.equal(outcome.pass, false);
  assert.equal(samples.length, 202);
  assert.equal(samples.filter((row) => row.phase === "formal").length, 200);
  assert.equal(samples[0].error_code, "observer_unavailable");
  assert.ok(samples.filter((row) => row.phase === "formal").every((row) => row.result === "not_run"));
  assert.equal(requests.filter((entry) => entry.path.endsWith("/comments") && entry.method === "POST").length, 0);
});

test("a timed observer disconnect stays missing after recovery and retains its measured timestamps", async () => {
  for (const disconnect of ["timed", "recovered"]) {
    const { outcome, samples, requests } = await successfulWritebackScenario({ disconnect });
    assert.equal(outcome.pass, false);
    assert.equal(samples.length, 202);
    const firstFormal = samples.filter((row) => row.phase === "formal" && row.run_index === 0);
    assert.equal(firstFormal.length, 2);
    assert.ok(firstFormal.every((row) => row.result === "missing" && row.error_code === "observer_disconnected" && row.latency_ms === null));
    assert.ok(firstFormal.every((row) => row.observed_latency_ms === row.read_end_ms - row.t0_ms && row.observed_latency_ms >= 0));
    assert.ok(firstFormal.every((row) => row.disconnected_observer_indices.join() === "99"));
    assert.ok(samples.filter((row) => row.phase === "formal" && row.run_index > 0).every((row) => row.result === "not_run"));
    assert.ok(samples.filter((row) => row.phase === "warmup").every((row) => row.result === "converged"));
    assert.equal(requests.filter((entry) => entry.path.endsWith("/comments") && entry.method === "POST").length, 2);
  }
});

test("offline and stale runtime reads cannot become a generic no-provider launch or a passing sample", () => {
  const now = Date.parse("2026-11-14T00:00:00Z"), runtime = heartbeatRuntime();
  const row = { id: runtime.runtimeId, workspace_id: runtime.workspaceId, daemon_id: runtime.daemonId, owner_id: "owner",
    status: "online", last_seen_at: new Date(now).toISOString() };
  assert.equal(stats.runtimeAvailability([row], runtime, "owner", now).error_code, null);
  assert.equal(stats.runtimeAvailability([{ ...row, status: "offline" }], runtime, "owner", now).error_code, "runtime_offline");
  assert.equal(stats.runtimeAvailability([{ ...row, last_seen_at: new Date(now - 150000).toISOString() }], runtime, "owner", now).error_code, "runtime_offline");
  assert.equal(stats.runtimeAvailability([{ ...row, owner_id: "reader" }], runtime, "owner", now).error_code, "runtime_identity_mismatch");
  assert.equal(stats.runtimeAvailability(null, runtime, "owner", now).error_code, "runtime_state_unavailable");
  const rows = ["business", "finalization"].map((metric) => ({ phase: "formal", run_index: 0, metric,
    result: "missing", latency_ms: null, error_code: "runtime_offline" }));
  const accounting = accountWriteback(rows);
  assert.equal(accounting.business.denominator, 100);
  assert.equal(accounting.business.filled_not_run, 99);
  assert.equal(writebackVerdict(accounting).business.summary.p95, Infinity);
  assert.equal(writebackVerdict(accounting).pass, false);
});

const ISSUE = "11111111-2222-4333-8444-555555555555";
const OTHER = "99999999-2222-4333-8444-555555555555";
const EVENT = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee";
const changed = (seq, overrides = {}) => ({ type: "collaboration:changed", event_id: `${EVENT}:issue`,
  payload: { issue_id: ISSUE, event_seq: String(seq), collaboration_revision: "7", authority_epoch: "1" }, ...overrides });
const fill = (count, value) => Array.from({ length: count }, () => value);

/** A complete formal matrix; `latency(event, observer)` returns ms or null for a missing sample. */
function formalRows(latency, events = PERFORMANCE_PROTOCOL.formalEvents, observers = PERFORMANCE_PROTOCOL.observers) {
  const rows = [];
  for (let event = 0; event < events; event += 1) {
    for (let observer = 0; observer < observers; observer += 1) {
      const value = latency(event, observer);
      rows.push({ phase: "formal", event_index: event, observer_index: observer,
        result: value === null ? "missing" : "converged", latency_ms: value });
    }
  }
  return rows;
}

test("nearest-rank uses the integer rank ceil(N*95/100) for N=100, 10000 and 20", () => {
  assert.equal(nearestRankIndex(100, 95), 95);
  assert.equal(nearestRankIndex(10000, 95), 9500);
  assert.equal(nearestRankIndex(20, 95), 19);
  assert.equal(nearestRankIndex(101, 95), 96);
  assert.equal(nearestRankIndex(1, 95), 1);
  // 0.95 * 100 is 94.99999999999999 in binary floating point; a float rank would pick 95 only by luck of rounding.
  assert.equal(nearestRankIndex(7, 95), 7);
  const ascending = Array.from({ length: 100 }, (_unused, index) => index + 1);
  assert.equal(nearestRank(ascending.toReversed(), 95), 95);
  assert.equal(nearestRank(Array.from({ length: 20 }, (_unused, index) => index + 1), 95), 19);
  assert.throws(() => nearestRank([], 95), /at least one/);
  assert.throws(() => nearestRank([1, Number.NaN], 95), /latency/);
  assert.throws(() => nearestRank([1, -1], 95), /latency/);
  assert.throws(() => nearestRankIndex(100, 95.5), /percentile/);
});

test("a missing sample enters the percentile as +Infinity instead of shrinking the denominator", () => {
  const five = [...fill(95, 100), ...fill(5, Number.POSITIVE_INFINITY)];
  assert.equal(nearestRank(five, 95), 100);
  assert.equal(summarize(five).count, 100);
  assert.equal(summarize(five).missing, 5);
  assert.equal(summarize(five).max, Number.POSITIVE_INFINITY);
});

test("more than five percent missing makes p95 unbounded", () => {
  const six = [...fill(94, 100), ...fill(6, Number.POSITIVE_INFINITY)];
  assert.equal(nearestRank(six, 95), Number.POSITIVE_INFINITY);
  const report = reportQuantiles(summarize(six));
  assert.equal(report.p95_ms, null);
  assert.equal(report.p95_finite, false);
  assert.equal(report.missing, 6);
});

test("any missing sample fails the sync gate even when every percentile is finite and fast", () => {
  const rows = formalRows((event, observer) => (event === 42 && observer === 7 ? null : 120));
  const verdict = propagationVerdict(accountPropagation(rows));
  assert.equal(verdict.denominator, 10000);
  assert.equal(verdict.missing, 1);
  assert.equal(verdict.pooled.p95, 120);
  assert.equal(verdict.conditions.pooled_p95_within_budget, true);
  assert.equal(verdict.conditions.zero_missing, false);
  assert.equal(verdict.pass, false);
});

test("a duplicate event_id is counted and never yields a second arrival", () => {
  const ledger = createFrameLedger(ISSUE);
  assert.equal(recordFrame(ledger, changed(8), 1000).kind, "first");
  const again = recordFrame(ledger, changed(8), 1900);
  assert.equal(again.kind, "duplicate");
  const arrival = matchExpected(ledger, 8n);
  assert.equal(arrival.first_ms, 1000);
  assert.equal(arrival.duplicates, 1);
  assert.equal(ledger.arrivals.size, 1);
  // The same sequence under another event id is an anomaly, still not a second sample.
  recordFrame(ledger, changed(8, { event_id: "bbbbbbbb-bbbb-4ccc-8ddd-eeeeeeeeeeee:issue" }), 2000);
  assert.equal(matchExpected(ledger, 8n).first_ms, 1000);
  assert.equal(matchExpected(ledger, 8n).conflicting_event_ids, 1);
});

test("only collaboration:changed with exactly sequence S+1 for this issue matches", () => {
  const ledger = createFrameLedger(ISSUE);
  const expected = expectedEventSeq("9");
  assert.equal(expected, 10n);
  const ignored = [
    { type: "collaboration:workbench_changed", event_id: `${EVENT}:account:x`, payload: {} },
    { type: "collaboration:workbench_changed", event_id: `${EVENT}:issue`, payload: changed(10).payload },
    { type: "collaboration:access_changed", event_id: `${EVENT}:access`, payload: changed(10).payload },
    changed(10, { event_id: `${EVENT}:account:x` }),
    changed(10, { event_id: undefined }),
    changed(10, { payload: { ...changed(10).payload, issue_id: OTHER } }),
    changed(10, { payload: { ...changed(10).payload, status: "todo" } }),
    changed(10, { payload: { ...changed(10).payload, event_seq: 10 } }),
    changed(10, { payload: { ...changed(10).payload, event_seq: "010" } }),
    changed(10, { payload: { ...changed(10).payload, event_seq: "0" } }),
    { type: "subscribe_ack", payload: { scope: "issue", id: ISSUE } },
    "collaboration:changed", null, 17,
  ];
  for (const frame of ignored) assert.equal(recordFrame(ledger, frame, 500).kind, "ignored");
  assert.equal(matchExpected(ledger, expected), null);
  // Neighbouring sequences are recorded for the barrier but are not the expected event.
  recordFrame(ledger, changed(9), 600);
  recordFrame(ledger, changed(11), 700);
  assert.equal(matchExpected(ledger, expected), null);
  recordFrame(ledger, changed(10, { payload: { ...changed(10).payload, issue_id: ISSUE.toUpperCase() } }), 800);
  assert.equal(matchExpected(ledger, expected).first_ms, 800);
  // Sequences compare as integers: "10" follows "9" although it sorts before it as text.
  assert.throws(() => expectedEventSeq("9.5"), /event sequence/);
  assert.throws(() => expectedEventSeq("-1"), /event sequence/);
});

test("a frame that arrives before the write response keeps its original arrival time", () => {
  const ledger = createFrameLedger(ISSUE);
  recordFrame(ledger, changed(10), 1040);
  // The write acknowledgement comes later; nobody was waiting when the frame landed.
  const arrival = matchExpected(ledger, expectedEventSeq("9"));
  assert.equal(arrival.first_ms, 1040);
  const sample = classifyObserverSample({ request_start_ms: 1000, deadline_ms: 10000, socket_open: true, expected_event_seq: 10n,
    intended_status: "in_progress", write: { ok: true, receipt_revision: "8", ack_ms: 1100 }, arrival,
    read: { end_ms: 1090, status: "in_progress", collaboration_revision: "8", event_seq: "10" } });
  assert.deepEqual(sample, { result: "converged", latency_ms: 90, event_latency_ms: 40, error_code: null });
});

test("without the matching frame an authoritative read alone is never a converged sample", () => {
  const base = { request_start_ms: 0, deadline_ms: 10000, socket_open: true, expected_event_seq: 10n, intended_status: "todo",
    write: { ok: true, receipt_revision: "8", ack_ms: 30 } };
  const goodRead = { end_ms: 400, status: "todo", collaboration_revision: "8", event_seq: "10" };
  assert.deepEqual(classifyObserverSample({ ...base, arrival: null, read: goodRead }),
    { result: "missing", latency_ms: null, event_latency_ms: null, error_code: "no_matching_frame" });
  assert.equal(classifyObserverSample({ ...base, socket_open: false, arrival: null, read: goodRead }).error_code, "socket_closed");
  const arrival = { event_seq: "10", event_id: `${EVENT}:issue`, frame_revision: "8", authority_epoch: "1", first_ms: 200, duplicates: 0, conflicting_event_ids: 0 };
  assert.equal(classifyObserverSample({ ...base, arrival, read: null }).error_code, "read_not_converged");
  assert.equal(classifyObserverSample({ ...base, arrival, read: { ...goodRead, status: "in_progress" } }).error_code, "read_not_converged");
  assert.equal(classifyObserverSample({ ...base, arrival, read: { ...goodRead, collaboration_revision: "7" } }).error_code, "read_not_converged");
  assert.equal(classifyObserverSample({ ...base, arrival, read: { ...goodRead, event_seq: "9" } }).error_code, "read_not_converged");
  assert.equal(classifyObserverSample({ ...base, arrival, read: { ...goodRead, end_ms: 10001 } }).error_code, "deadline_exceeded");
  assert.equal(classifyObserverSample({ ...base, arrival, read: goodRead, write: { ok: false, error_code: "write_http_409" } }).error_code, "write_http_409");
  // 2-10 s is late, not missing: the real latency is kept and judged against the budget.
  assert.deepEqual(classifyObserverSample({ ...base, arrival, read: { ...goodRead, end_ms: 4321 } }),
    { result: "converged", latency_ms: 4321, event_latency_ms: 200, error_code: null });
});

test("warm-up rows are retained but never enter or replace the formal denominator", () => {
  const warmup = [];
  for (let event = 0; event < PERFORMANCE_PROTOCOL.warmupEvents; event += 1) {
    for (let observer = 0; observer < 100; observer += 1) warmup.push({ phase: "warmup", event_index: event, observer_index: observer, result: "converged", latency_ms: 1 });
  }
  const accounting = accountPropagation([...warmup, ...formalRows(() => 300)]);
  assert.equal(accounting.warmup_rows, 500);
  assert.equal(accounting.denominator, 10000);
  assert.equal(propagationVerdict(accounting).pooled.p50, 300);
  // Relabelling a slow formal event as warm-up leaves a hole that counts as not run.
  const relabelled = formalRows(() => 300).map((row) => (row.event_index === 3 ? { ...row, phase: "warmup", event_index: 4 } : row));
  const cheated = accountPropagation(relabelled);
  assert.equal(cheated.denominator, 10000);
  assert.equal(cheated.filled_not_run, 100);
  assert.equal(propagationVerdict(cheated).missing, 100);
  assert.equal(propagationVerdict(cheated).pass, false);
});

test("an aborted run keeps the 10,000 denominator and reports the remainder as NOT_RUN", () => {
  const rows = [...formalRows(() => 150, 40), ...Array.from({ length: 60 }, (_unused, offset) => notRunEventRows(40 + offset, "aborted_state_unknown")).flat()];
  const accounting = accountPropagation(rows);
  assert.equal(accounting.denominator, 10000);
  assert.equal(accounting.converged, 4000);
  assert.equal(accounting.not_run, 6000);
  const verdict = propagationVerdict(accounting);
  assert.equal(verdict.missing, 6000);
  assert.equal(verdict.pooled.p95, Number.POSITIVE_INFINITY);
  assert.equal(verdict.pass, false);
  // The same holds when the file simply stops: absent pairs are filled, not dropped.
  const truncated = accountPropagation(formalRows(() => 150, 40));
  assert.equal(truncated.denominator, 10000);
  assert.equal(truncated.filled_not_run, 6000);
  assert.equal(propagationVerdict(truncated).pass, false);
});

test("a failed write or failed preparation marks all 100 samples of that event non-converged", () => {
  const failed = failedEventRows("formal", 12, "write_http_409");
  assert.equal(failed.length, 100);
  assert.deepEqual(new Set(failed.map((row) => row.observer_index)).size, 100);
  for (const row of failed) assert.deepEqual({ result: row.result, latency_ms: row.latency_ms, error_code: row.error_code },
    { result: "missing", latency_ms: null, error_code: "write_http_409" });
  const rows = [...formalRows(() => 90).filter((row) => row.event_index !== 12), ...failed];
  const verdict = propagationVerdict(accountPropagation(rows));
  assert.equal(verdict.missing, 100);
  assert.equal(verdict.pooled.p95, 90);
  assert.equal(verdict.slowest_per_event.max, Number.POSITIVE_INFINITY);
  assert.equal(verdict.pass, false);
});

test("pooled p95 and per-event slowest-observer p95 are judged separately", () => {
  // One observer per event takes 3 s: 1% of the pool, yet the slowest observer of every event.
  const oneSlow = propagationVerdict(accountPropagation(formalRows((event, observer) => (observer === event ? 3000 : 200))));
  assert.equal(oneSlow.missing, 0);
  assert.equal(oneSlow.pooled.p95, 200);
  assert.equal(oneSlow.conditions.pooled_p95_within_budget, true);
  assert.equal(oneSlow.slowest_per_event.p95, 3000);
  assert.equal(oneSlow.conditions.slowest_observer_p95_within_budget, false);
  assert.equal(oneSlow.pass, false);
  // Five observers time out on every event: pooled p95 still looks fine.
  const fiveLost = propagationVerdict(accountPropagation(formalRows((_event, observer) => (observer < 5 ? null : 200))));
  assert.equal(fiveLost.pooled.p95, 200);
  assert.equal(fiveLost.missing, 500);
  assert.equal(fiveLost.pass, false);
  // A fixed slow account is listed even when both pooled gates pass.
  const fixedSlow = propagationVerdict(accountPropagation(formalRows((event, observer) => (observer === 17 && event < 6 ? 2500 : 200))));
  assert.equal(fixedSlow.pass, false);
  assert.equal(fixedSlow.slowest_per_event.p95, 2500);
  const rare = propagationVerdict(accountPropagation(formalRows((event, observer) => (observer === 17 && event < 5 ? 2500 : 200))));
  assert.equal(rare.pass, true);
  assert.deepEqual(rare.observers_over_budget, []);
  const habitual = propagationVerdict(accountPropagation(formalRows((event, observer) => (observer === 17 && event % 10 < 1 ? 1900 : observer === 23 ? 2100 : 200))));
  assert.deepEqual(habitual.observers_over_budget, [23]);
  // Exactly at the budget passes; one millisecond over does not.
  assert.equal(propagationVerdict(accountPropagation(formalRows(() => 2000))).pass, true);
  assert.equal(propagationVerdict(accountPropagation(formalRows(() => 2001))).pass, false);
  // The writer's own sample is reported apart: 9,900 samples exclude it.
  assert.equal(rare.excluding_writer.count, 9900);
});

test("accounting refuses duplicated pairs, impossible indices and relabelled late samples", () => {
  const rows = formalRows(() => 100);
  assert.throws(() => accountPropagation([...rows, { ...rows[0] }]), /duplicate/);
  assert.throws(() => accountPropagation([{ ...rows[0], observer_index: 100 }]), /observer_index/);
  assert.throws(() => accountPropagation([{ ...rows[0], event_index: 100 }]), /event_index/);
  assert.throws(() => accountPropagation([{ ...rows[0], phase: "rehearsal" }]), /phase/);
  // A sixth warm-up event means the registered warm-up count was changed after the fact.
  assert.throws(() => accountPropagation([{ ...rows[0], phase: "warmup", event_index: 5 }]), /event_index/);
  assert.throws(() => accountPropagation([{ ...rows[0], latency_ms: null }]), /latency/);
  assert.throws(() => accountPropagation([{ ...rows[0], result: "ok" }]), /result/);
  // "Converged" after the 10 s cut-off is not convergence within the cut-off.
  const late = accountPropagation(rows.map((row, index) => (index === 0 ? { ...row, latency_ms: 10001 } : row)));
  assert.equal(late.over_deadline_reclassified, 1);
  assert.equal(propagationVerdict(late).missing, 1);
});

test("the two write-back gates are independent and each needs a complete denominator", () => {
  const rows = (business, finalization) => Array.from({ length: 100 }, (_unused, run) => [
    { phase: "formal", run_index: run, metric: "business", result: business(run) === null ? "missing" : "converged", latency_ms: business(run) },
    { phase: "formal", run_index: run, metric: "finalization", result: finalization(run) === null ? "missing" : "converged", latency_ms: finalization(run) },
  ]).flat();
  const good = writebackVerdict(accountWriteback(rows(() => 400, () => 900)));
  assert.deepEqual([good.business.pass, good.finalization.pass, good.pass], [true, true, true]);
  // A fast business metric cannot stand in for a slow finalization.
  const slowFinal = writebackVerdict(accountWriteback(rows(() => 50, (run) => (run < 6 ? 5001 : 900))));
  assert.deepEqual([slowFinal.business.pass, slowFinal.finalization.pass, slowFinal.pass], [true, false, false]);
  assert.equal(slowFinal.finalization.summary.p95, 5001);
  // One missing finalization fails its gate although p95 is finite.
  const lost = writebackVerdict(accountWriteback(rows(() => 50, (run) => (run === 99 ? null : 900))));
  assert.equal(lost.finalization.summary.p95, 900);
  assert.equal(lost.finalization.missing, 1);
  assert.deepEqual([lost.business.pass, lost.finalization.pass, lost.pass], [true, false, false]);
  // The warm-up run never fills a formal slot, and an aborted remainder stays in the denominator.
  const partial = rows(() => 400, () => 900).filter((row) => row.run_index < 30);
  const warmup = [{ phase: "warmup", run_index: 0, metric: "business", result: "converged", latency_ms: 1 },
    { phase: "warmup", run_index: 0, metric: "finalization", result: "converged", latency_ms: 1 }];
  const aborted = accountWriteback([...warmup, ...partial]);
  assert.equal(aborted.business.denominator, 100);
  assert.equal(aborted.business.filled_not_run, 70);
  assert.equal(aborted.warmup_rows, 2);
  assert.equal(writebackVerdict(aborted).business.summary.p95, Number.POSITIVE_INFINITY);
  assert.equal(writebackVerdict(aborted).pass, false);
  assert.throws(() => accountWriteback([...rows(() => 1, () => 1), { phase: "formal", run_index: 0, metric: "business", result: "converged", latency_ms: 1 }]), /duplicate/);
  assert.throws(() => accountWriteback([{ phase: "formal", run_index: 0, metric: "hint", result: "converged", latency_ms: 1 }]), /metric/);
  // Exactly five seconds passes; a late sample past the 30 s cut-off is missing.
  assert.equal(writebackVerdict(accountWriteback(rows(() => 5000, () => 5000))).pass, true);
  assert.equal(writebackVerdict(accountWriteback(rows((run) => (run === 0 ? 30001 : 100), () => 100))).business.missing, 1);
});

test("phase jitter is seeded, reproducible, inside [0,500) and not phase locked", () => {
  const first = Array.from({ length: 105 }, (_unused, ordinal) => phaseJitterMs(20261114, ordinal));
  const again = Array.from({ length: 105 }, (_unused, ordinal) => phaseJitterMs(20261114, ordinal));
  assert.deepEqual(first, again);
  for (const value of first) assert.ok(Number.isInteger(value) && value >= 0 && value < 500);
  assert.ok(new Set(first).size > 60);
  // Every 100 ms slice of the relay period is visited.
  assert.equal(new Set(first.map((value) => Math.floor(value / 100))).size, 5);
  assert.notDeepEqual(first, Array.from({ length: 105 }, (_unused, ordinal) => phaseJitterMs(20261115, ordinal)));
  assert.throws(() => phaseJitterMs(1.5, 0), /seed/);
  assert.throws(() => phaseJitterMs(1, -1), /ordinal/);
});

test("registration pacing never plans more than 30 attempts in one server window", () => {
  const window = PERFORMANCE_PROTOCOL.registrationWindowMs;
  const start = 40 * window;
  const attempts = Array.from({ length: 30 }, (_unused, index) => start + index * 1000);
  assert.equal(registrationWaitMs(attempts.slice(0, 29), start + 30_000), 0);
  // The 31st attempt waits for the next aligned window, plus a margin.
  assert.equal(registrationWaitMs(attempts, start + 30_000), window - 30_000 + PERFORMANCE_PROTOCOL.registrationMarginMs);
  // Attempts from the previous window do not count against the new one.
  assert.equal(registrationWaitMs(attempts, start + window + 1), 0);
  // The limiter window is aligned to the wall clock, not to the first attempt.
  const late = Array.from({ length: 30 }, (_unused, index) => start + window - 30_000 + index * 1000);
  assert.equal(registrationWaitMs(late, start + window - 500), 500 + PERFORMANCE_PROTOCOL.registrationMarginMs);
});

test("write-back readers are never the triggering account or the resource owner", () => {
  for (let run = 0; run < 100; run += 1) {
    const { reader, hint } = pickWritebackReaders(run, 0, 100);
    assert.equal(new Set([run, 0, reader, hint]).size, run === 0 ? 3 : 4);
    assert.ok(reader >= 0 && reader < 100 && hint >= 0 && hint < 100);
  }
  assert.throws(() => pickWritebackReaders(1, 0, 3), /participants/);
  assert.deepEqual(pickWritebackReaders(99, 0, 100), { reader: 1, hint: 2 });
});

test("the quiescence barrier accepts a delivered frame or a snapshot that already covers the sequence", () => {
  const ledger = createFrameLedger(ISSUE);
  assert.equal(barrierSatisfied(ledger, 9n, 8n), false);
  assert.equal(barrierSatisfied(ledger, 9n, 9n), true);
  recordFrame(ledger, changed(9), 10);
  assert.equal(barrierSatisfied(ledger, 9n, 3n), true);
  assert.equal(barrierSatisfied(ledger, 10n, 3n), false);
});

test("the relay-time revision in a frame is recorded as an anomaly, never used to match", () => {
  // A transfer relayed after the status write carries the later revision (F01).
  assert.equal(revisionAnomaly("12", "12"), false);
  assert.equal(revisionAnomaly("13", "12"), true);
  assert.equal(revisionAnomaly("9", "10"), true);
  const ledger = createFrameLedger(ISSUE);
  recordFrame(ledger, changed(10, { payload: { ...changed(10).payload, collaboration_revision: "99" } }), 5);
  assert.equal(matchExpected(ledger, 10n).frame_revision, "99");
});

test("a manifest that changes the pre-registered protocol is rejected", () => {
  assert.deepEqual(preregistrationMismatches({ ...PERFORMANCE_PROTOCOL }), []);
  assert.deepEqual(preregistrationMismatches({ ...PERFORMANCE_PROTOCOL, observers: 50 }), ["observers"]);
  assert.deepEqual(preregistrationMismatches({ ...PERFORMANCE_PROTOCOL, warmupEvents: 20, propagationBudgetMs: 5000 }).sort(),
    ["propagationBudgetMs", "warmupEvents"]);
  const { formalEvents: _dropped, ...partial } = PERFORMANCE_PROTOCOL;
  assert.deepEqual(preregistrationMismatches(partial), ["formalEvents"]);
  assert.deepEqual(preregistrationMismatches({ ...PERFORMANCE_PROTOCOL, extra: 1 }), ["extra"]);
  assert.ok(preregistrationMismatches(null).length > 0);
  assert.deepEqual({ observers: PERFORMANCE_PROTOCOL.observers, formal: PERFORMANCE_PROTOCOL.formalEvents, warmup: PERFORMANCE_PROTOCOL.warmupEvents,
    cutoff: PERFORMANCE_PROTOCOL.propagationDeadlineMs, budget: PERFORMANCE_PROTOCOL.propagationBudgetMs,
    runs: PERFORMANCE_PROTOCOL.writebackFormalRuns, writebackBudget: PERFORMANCE_PROTOCOL.writebackBudgetMs },
  { observers: 100, formal: 100, warmup: 5, cutoff: 10000, budget: 2000, runs: 100, writebackBudget: 5000 });
});
