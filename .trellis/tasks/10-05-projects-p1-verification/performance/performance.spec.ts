import { test } from "@playwright/test";
import { randomUUID, createHash } from "node:crypto";
import { appendFileSync, closeSync, existsSync, mkdirSync, openSync, readFileSync, writeFileSync } from "node:fs";
import { execFileSync, spawn, spawnSync, type ChildProcess } from "node:child_process";
import { join, resolve } from "node:path";
import { performance } from "node:perf_hooks";
import pg from "pg";
import { TestApiClient, type TestIssueStatus } from "../../../../e2e/fixtures";

const root = process.cwd();
const dir = resolve(root, process.env.PERF_DIRECTORY ?? ".omx/projects-p1-performance");
const label = process.env.PERF_LABEL ?? "current";
const mode = process.env.PERF_MODE ?? "full";
const apiBase = process.env.NEXT_PUBLIC_API_URL!;
const budget = JSON.parse(readFileSync(join(root, ".trellis/tasks/10-05-projects-p1-verification/performance/budget.json"), "utf8"));
const output = join(dir, `${label}-${mode}`);
const sleep = (ms: number) => new Promise<void>((done) => setTimeout(done, Math.max(ms, 0)));
const states: TestIssueStatus[] = ["done", "done", "done", "done", "cancelled", "blocked", "in_review", "todo", "todo", "todo"];
type Session = { workspace: { id: string; slug: string }; owner: string; project: string; projects: string[]; token: string; issueIDs: string[]; targetIDs: string[] };
type Sample = { ms: number; status: number; bytes: number; error?: string };
type Endpoint = { name: string; path: (s: Session) => string };
let child: ChildProcess | undefined;
let logFd: number | undefined;

function save(name: string, value: unknown) { writeFileSync(`${output}-${name}.json`, JSON.stringify(value, null, 2)); }
function emit(value: unknown) { console.log(JSON.stringify(value)); appendFileSync(`${output}-events.jsonl`, `${JSON.stringify(value)}\n`); }
function quantile(values: number[], fraction: number) { const sorted = [...values].sort((a, b) => a - b); return sorted.length ? sorted[Math.max(0, Math.ceil(sorted.length * fraction) - 1)] : null; }
function summary(samples: Sample[]) { const ok = samples.filter((s) => s.status === 200 && !s.error); return { samples: samples.length, p50_ms: quantile(ok.map((s) => s.ms), .5), p95_ms: quantile(ok.map((s) => s.ms), .95), max_ms: Math.max(...samples.map((s) => s.ms)), errors: samples.length - ok.length, mean_response_bytes: ok.length ? ok.reduce((n, s) => n + s.bytes, 0) / ok.length : null }; }
function headers(session: Session) { return { Authorization: `Bearer ${session.token}`, "X-Workspace-ID": session.workspace.id, "Content-Type": "application/json" }; }
async function request(session: Session, path: string) {
  const start = performance.now();
  try {
    const response = await fetch(`${apiBase}${path}`, { headers: headers(session), signal: AbortSignal.timeout(30_000) });
    const body = await response.text();
    return { sample: { ms: performance.now() - start, status: response.status, bytes: Buffer.byteLength(body) }, body: JSON.parse(body) };
  } catch (error) { return { sample: { ms: performance.now() - start, status: 0, bytes: 0, error: String(error) }, body: null }; }
}

async function startServer(logging = false) {
  if (child) throw new Error("Owned server is already running");
  try { await fetch(`${apiBase}/health`, { signal: AbortSignal.timeout(500) }); throw new Error(`Port is occupied: ${new URL(apiBase).port}`); } catch (error) { if (String(error).includes("Port is occupied")) throw error; }
  const binary = join(dir, `${label}-server`);
  const dbURL = new URL(process.env.DATABASE_URL!);
  if (logging) dbURL.searchParams.set("options", "-c log_min_duration_statement=0");
  logFd = openSync(join(dir, `${label}-api.log`), "a");
  const start = performance.now();
  child = spawn(binary, [], { cwd: label === "baseline" ? join(dir, "baseline-src/server") : join(root, "server"), env: { ...process.env, DATABASE_URL: dbURL.toString() }, stdio: ["ignore", logFd, logFd] });
  for (let n = 0; n < 100; n++) {
    if (child.exitCode !== null || child.signalCode !== null) throw new Error(`Owned API exited ${child.exitCode}`);
    try {
      const health = await (await fetch(`${apiBase}/health`, { signal: AbortSignal.timeout(1000) })).json();
      if (health.pid !== child.pid) throw new Error(`Listener pid ${health.pid} differs from owned pid ${child.pid}`);
      const proof = { ...health, ready_ms: performance.now() - start, binary_sha256: createHash("sha256").update(readFileSync(binary)).digest("hex"), logging };
      emit({ phase: "api_started", proof }); return proof;
    } catch (error) { if (String(error).includes("differs from owned")) throw error; await sleep(100); }
  }
  throw new Error("Owned API did not become healthy");
}
async function stopServer() {
  if (!child) return;
  const owned = child; child = undefined;
  owned.kill("SIGTERM");
  await Promise.race([new Promise<void>((done) => owned.once("exit", () => done())), sleep(10_000)]);
  if (owned.exitCode === null && owned.signalCode === null) { owned.kill("SIGKILL"); await new Promise<void>((done) => owned.once("exit", () => done())); }
  if (logFd !== undefined) { closeSync(logFd); logFd = undefined; }
}

async function fixture(db: pg.Client): Promise<Session> {
  const file = join(dir, `${label}-session.json`);
  if (existsSync(file)) return JSON.parse(readFileSync(file, "utf8"));
  const api = new TestApiClient();
  const slug = `p1-performance-${label}-${randomUUID().slice(0, 8)}`;
  await api.login(`${slug}@example.invalid`, "P1 performance fixture");
  const workspace = await api.ensureWorkspace("P1 isolated performance", slug);
  if (workspace.slug !== slug) throw new Error("Fixture workspace is not isolated");
  const owner = (await api.requestJSON<{ id: string }>("/api/me")).id;
  const project = await api.requestJSON<{ id: string }>("/api/projects", { method: "POST", body: { title: "P1 10000 formal tasks", status: "in_progress", lead_type: "member", lead_id: owner } });
  const projects = [project.id, ...(await db.query<{ id: string }>("INSERT INTO project(workspace_id,title,status,lead_type,lead_id) SELECT $1,'P1 performance '||n,'planned','member',$2 FROM generate_series(1,499) n RETURNING id", [workspace.id, owner])).rows.map((row) => row.id)];
  const formalCount = 10_000 + 499 * 10;
  const rows = await api.seedTableIssues(Array.from({ length: formalCount + 300 }, (_, n) => ({ title: `Performance issue ${n}`, status: states[n % states.length] })));
  await db.query("UPDATE issue i SET project_id=f.project_id,admission_status=f.admission_status,due_date=CASE WHEN f.ordinal % 3=0 THEN CURRENT_DATE ELSE CURRENT_DATE-1 END,assignee_type=CASE WHEN f.ordinal % 2=0 THEN 'member' END,assignee_id=CASE WHEN f.ordinal % 2=0 THEN $4::uuid END FROM unnest($1::uuid[],$2::uuid[],$3::text[]) WITH ORDINALITY f(id,project_id,admission_status,ordinal) WHERE i.id=f.id AND i.workspace_id=$5", [rows.map((r) => r.id), rows.map((_, n) => n < 10_000 || n >= formalCount ? project.id : projects[1 + Math.floor((n - 10_000) / 10)]), rows.map((_, n) => n < formalCount ? "not_required" : ["pending", "rejected", "duplicate"][Math.floor((n - formalCount) / 100)]), owner, workspace.id]);
  await db.query("ANALYZE issue"); await db.query("ANALYZE project");
  const session: Session = { workspace, owner, project: project.id, projects, token: api.getToken()!, issueIDs: rows.map((r) => r.id), targetIDs: rows.slice(0, 10_000).map((r) => r.id) };
  writeFileSync(file, JSON.stringify(session), { mode: 0o600 });
  save("fixture", { workspace_id: workspace.id, project_id: project.id, project_count: projects.length, target_formal: 10_000, target_nonformal: 300, other_projects_formal_each: 10, total_formal: formalCount, total_issues: rows.length });
  emit({ phase: "fixture_ready", projects: 500, target_formal: 10_000, excluded: 300 });
  return session;
}

function endpoints(): Endpoint[] {
  const common: Endpoint[] = [{ name: "legacy_detail", path: (s) => `/api/projects/${s.project}` }, { name: "legacy_list_500", path: () => "/api/projects" }];
  if (label === "baseline") return common;
  return [...common, { name: "overview", path: (s) => `/api/projects/${s.project}/overview` }, ...["blocked", "overdue", "unassigned", "in_review"].map((signal) => ({ name: `risk_${signal}`, path: (s: Session) => `/api/projects/${s.project}/health/issues?signal=${signal}&limit=50` }))];
}

async function explain(db: pg.Client, session: Session) {
  const queries = {
    formal_inputs: "SELECT id,status,due_date,assignee_type,assignee_id FROM issue WHERE workspace_id=$1 AND project_id=$2 AND admission_status IN ('not_required','accepted') ORDER BY id ASC",
    legacy_scope: "SELECT project_id,count(*)::bigint,count(*) FILTER(WHERE status=ANY(ARRAY['done','cancelled']))::bigint FROM issue WHERE workspace_id=$1 AND project_id=$2 AND admission_status IN ('not_required','accepted') GROUP BY project_id",
  };
  const plans: Record<string, unknown> = {};
  for (const [name, sql] of Object.entries(queries)) plans[name] = (await db.query(`EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) ${sql}`, [session.workspace.id, session.project])).rows;
  save("explain", plans);
}

async function staticMeasure(session: Session) {
  const results: Record<string, unknown> = {};
  for (const endpoint of endpoints()) {
    const path = endpoint.path(session);
    for (let n = 0; n < budget.warmup_per_endpoint; n++) await request(session, path);
    for (const concurrency of budget.concurrency) {
      let cursor = 0; const samples: Sample[] = []; const started = performance.now();
      await Promise.all(Array.from({ length: concurrency }, async () => {
        while (cursor++ < budget.requests_per_endpoint_and_concurrency) {
          const response = await request(session, path); samples.push(response.sample);
          if (response.sample.status === 200 && endpoint.name.startsWith("risk_")) {
            const signal = endpoint.name.slice(5);
            if (response.body.total !== response.body.overview.statistics.counts[signal] || response.body.snapshot_version !== response.body.overview.statistics.snapshot_version) response.sample.error = "count/page version mismatch";
          }
        }
      }));
      const stats = { ...summary(samples), elapsed_ms: performance.now() - started, concurrency, warmup: budget.warmup_per_endpoint, budget_pass: summary(samples).errors === 0 && (summary(samples).p95_ms ?? Infinity) <= budget.http_p95_ms };
      results[`${endpoint.name}_c${concurrency}`] = stats;
      save(`${endpoint.name}-c${concurrency}-samples`, samples);
      emit({ phase: "static", endpoint: endpoint.name, ...stats });
      save("static-summary", results);
    }
  }
}

async function queryObserve(db: pg.Client, session: Session) {
  const records = [];
  for (const endpoint of endpoints()) {
    await request(session, endpoint.path(session));
    const started = new Date().toISOString();
    for (let n=0;n<5;n++) await request(session, endpoint.path(session));
    await sleep(150);
    const ended = new Date().toISOString();
    const pids = (await db.query("SELECT pid FROM pg_stat_activity WHERE datname=current_database() AND backend_type='client backend'")).rows.map((row)=>row.pid);
    const command = ["logs","--timestamps","--since",started,"--until",ended,"multica-postgres-1"];
    const logs = spawnSync("docker", command, {encoding:"utf8", maxBuffer:20*1024*1024});
    if(logs.status!==0) throw new Error("Could not read owned query logging window");
    const histogram: Record<string,number> = {};
    for(const line of (logs.stdout+logs.stderr).split("\n")) {
      const pid = line.match(/ \[(\d+)\]/); if(!pid || !pids.includes(Number(pid[1]))) continue;
      if(!/duration:.*(?:execute |statement:)/.test(line)) continue;
      const named = line.match(/-- name: ([A-Za-z0-9_]+)/);
      const key = named?.[1] ?? line.replace(/^.*(?:execute [^:]+:|statement:)\s*/, "").slice(0,180);
      histogram[key]=(histogram[key]??0)+1;
    }
    const total=Object.values(histogram).reduce((sum,n)=>sum+n,0);
    records.push({endpoint:endpoint.name,requests:5,started,ended,pids,statement_count:total,statements_per_request:total/5,histogram,command});
    emit({phase:"query_observation",...records.at(-1)});
  }
  save("query-observation",records);
}

async function resetFixture(db:pg.Client,session:Session) {
  await db.query("UPDATE issue i SET admission_status='not_required',due_date=CASE WHEN f.ordinal % 3=0 THEN CURRENT_DATE ELSE CURRENT_DATE-1 END,assignee_type=CASE WHEN f.ordinal % 2=0 THEN 'member' END,assignee_id=CASE WHEN f.ordinal % 2=0 THEN $3::uuid END,revision=revision+1 FROM unnest($1::uuid[]) WITH ORDINALITY f(id,ordinal) WHERE i.id=f.id AND i.workspace_id=$2", [session.targetIDs,session.workspace.id,session.owner]);
}

async function activeMeasure(db: pg.Client, session: Session) {
  for (const rate of budget.active_change_rates_per_second) {
    await resetFixture(db,session);
    const start = performance.now(); const deadline = start + budget.active_duration_seconds_each * 1000;
    let writes = 0; let writeErrors = 0; let stopping = false;
    const writer = (async () => {
      if (!rate) return;
      while (!stopping && performance.now() < deadline) {
        const issue = session.targetIDs[5 + (writes % 100) * 10];
        try {
          switch (writes % 3) {
            case 0: await db.query("UPDATE issue SET due_date=CASE WHEN due_date=CURRENT_DATE THEN CURRENT_DATE-1 ELSE CURRENT_DATE END,revision=revision+1 WHERE workspace_id=$1 AND id=$2", [session.workspace.id, issue]); break;
            case 1: await db.query("UPDATE issue SET assignee_type=CASE WHEN assignee_id IS NULL THEN 'member' END,assignee_id=CASE WHEN assignee_id IS NULL THEN $3::uuid END,revision=revision+1 WHERE workspace_id=$1 AND id=$2", [session.workspace.id, issue, session.owner]); break;
            case 2: await db.query("UPDATE issue SET admission_status=CASE WHEN admission_status='pending' THEN 'not_required' ELSE 'pending' END,revision=revision+1 WHERE workspace_id=$1 AND id=$2", [session.workspace.id, issue]); break;
          }
        } catch { writeErrors++; }
        writes++; await sleep(start + writes * 1000 / rate - performance.now());
      }
    })();
    const samples: any[] = []; let cursor: string | null = null; let version: string | null = null; let previousWasFirst = false; let consecutiveResets = 0; let maxResets = 0;
    for (let operation = 0; performance.now() < deadline; operation++) {
      const secondAttempt = previousWasFirst && cursor !== null;
      const query = new URLSearchParams({ signal: "blocked", limit: "50" });
      if (cursor) query.set("cursor", cursor); if (version) query.set("snapshot_version", version);
      const requestedLastID: string | null = cursor ? JSON.parse(Buffer.from(cursor,"base64url").toString("utf8")).last_id : null;
      const result = await request(session, `/api/projects/${session.project}/health/issues?${query}`);
      const firstID: string | null = result.body?.items?.[0]?.id ?? null;
      const lastID: string | null = result.body?.items?.at(-1)?.id ?? null;
      const continued = requestedLastID !== null && firstID !== null && firstID > requestedLastID;
      const restarted = requestedLastID !== null && firstID !== null && firstID <= requestedLastID;
      const terminal = requestedLastID !== null && result.sample.status === 200 && result.body?.items?.length === 0 && result.body.next_cursor === null;
      const item = { operation, since_start_ms: performance.now() - start, ...result.sample, requested_last_id:requestedLastID,first_id:firstID,last_id:lastID,continued_from_cursor:continued,actual_restart:restarted,terminal_suffix:terminal, second_page_attempt: secondAttempt, second_page_success: secondAttempt && result.sample.status === 200 && continued, refreshed: result.body?.refreshed ?? null, total: result.body?.total ?? null, snapshot_version: result.body?.snapshot_version ?? null };
      if (result.sample.status === 200) {
        if (result.body.total !== result.body.overview.statistics.counts.blocked || result.body.snapshot_version !== result.body.overview.statistics.snapshot_version) item.error = "same version inconsistency";
        if(requestedLastID!==null && result.body.items.some((row:any)=>row.id<=requestedLastID)) item.error="nonempty continuation did not strictly advance";
        if(result.body.items.some((row:any)=>!["not_required","accepted"].includes(row.admission_status))) item.error="nonformal row in current risk page";
        if(result.body.items.some((row:any,index:number)=>index>0 && row.id<=result.body.items[index-1].id)) item.error="risk IDs not strictly ordered";
        previousWasFirst = cursor === null || restarted;
        cursor = result.body.next_cursor; version = result.body.snapshot_version;
        consecutiveResets = restarted ? consecutiveResets + 1 : 0; maxResets = Math.max(maxResets, consecutiveResets);
      }
      samples.push(item); appendFileSync(`${output}-active-${rate}-samples.jsonl`, `${JSON.stringify(item)}\n`);
      if (operation % 60 === 0) emit({ phase: "active_progress", rate, elapsed_s: (performance.now() - start) / 1000, operations: samples.length, writes, refreshed: samples.filter((s) => s.refreshed).length });
      await sleep(start + (operation + 1) * 1000 - performance.now());
    }
    stopping = true; await writer;
    const attempts = samples.filter((s) => s.second_page_attempt && !s.terminal_suffix); const successes = attempts.filter((s) => s.second_page_success);
    const stats = { ...summary(samples), rate_per_second: rate, elapsed_seconds: (performance.now() - start) / 1000, writes, write_errors: writeErrors, refreshed: samples.filter((s) => s.refreshed).length, refresh_ratio: samples.filter((s) => s.refreshed).length / samples.length, second_page_requests: samples.filter(s=>s.second_page_attempt).length, terminal_suffixes: samples.filter(s=>s.terminal_suffix).length, nonempty_continuations: samples.filter(s=>s.requested_last_id!==null&&s.first_id!==null).length, strictly_advancing_continuations: samples.filter(s=>s.continued_from_cursor).length, second_page_attempts: attempts.length, second_page_successes: successes.length, second_page_success_ratio: attempts.length ? successes.length / attempts.length : null, second_page_p95_ms: quantile(successes.map((s) => s.ms), .95), max_consecutive_resets: maxResets };
    save(`active-${rate}-summary`, stats); emit({ phase: "active_complete", ...stats });
  }
}

test("P1 isolated actual HTTP performance and active pagination", async () => {
  if (!process.env.DATABASE_URL || !["baseline", "current"].includes(label) || !["localhost", "127.0.0.1"].includes(new URL(apiBase).hostname)) throw new Error("Use the task-owned performance environment");
  mkdirSync(dir, { recursive: true });
  const db = new pg.Client(process.env.DATABASE_URL); await db.connect();
  const metadata = { at: new Date().toISOString(), label, mode, api_base: apiBase, budget, host: execFileSync("uname", ["-a"], { encoding: "utf8" }).trim(), memory_bytes: Number(execFileSync("sysctl", ["-n", "hw.memsize"], { encoding: "utf8" })), cpu: execFileSync("sysctl", ["-n", "machdep.cpu.brand_string"], { encoding: "utf8" }).trim(), go: execFileSync("go", ["version"], { encoding: "utf8" }).trim(), postgres: (await db.query("SELECT version(),current_database() AS database")).rows[0] };
  save("metadata", metadata);
  try {
    await startServer(mode === "queries"); const session = await fixture(db);
    if(mode!=="queries") await resetFixture(db,session);
    await explain(db, session);
    const legacy = await request(session, `/api/projects/${session.project}`);
    if (mode!=="queries" && (legacy.body?.issue_count !== 10000 || legacy.body?.done_count !== 5000)) throw new Error(`Fixture scope is not 10000/5000: ${JSON.stringify(legacy.body)}`);
    if (mode === "queries") { await queryObserve(db,session); save("complete",{at:new Date().toISOString(),complete:true}); return; }
    if (mode !== "active") {
      await stopServer(); const cold = [];
      for (let n = 0; n < budget.cold_process_samples; n++) { const proof = await startServer(); const result = await request(session, label === "baseline" ? `/api/projects/${session.project}` : `/api/projects/${session.project}/overview`); cold.push({ ...result.sample, proof }); if (n < budget.cold_process_samples - 1) await stopServer(); }
      save("cold-process-samples", cold);
      await staticMeasure(session);
    }
    if (label === "current" && mode !== "static") await activeMeasure(db, session);
    save("complete", { at: new Date().toISOString(), complete: true });
  } finally { await stopServer(); await db.end(); }
});
