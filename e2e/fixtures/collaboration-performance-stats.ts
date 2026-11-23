/**
 * Pure statistics for the PAY-241 S5 sampling harness: percentiles over the
 * complete denominator, frame correlation, denominator accounting and verdicts.
 *
 * No I/O and no clock access: every time is passed in by the caller, taken from
 * one monotonic clock. Only erasable type syntax is used so the counter-example
 * tests can run this file with `node --experimental-strip-types`.
 */

/** Registered before any run. A manifest that disagrees is refused, not adopted. */
export const PERFORMANCE_PROTOCOL = Object.freeze({
  observers: 100,
  warmupEvents: 5,
  formalEvents: 100,
  propagationDeadlineMs: 10_000,
  propagationBudgetMs: 2_000,
  relayPeriodMs: 500,
  writebackWarmupRuns: 1,
  writebackFormalRuns: 100,
  writebackDeadlineMs: 30_000,
  writebackBudgetMs: 5_000,
  registrationWindowMs: 300_000,
  registrationPerWindow: 30,
  registrationMarginMs: 2_000,
});
export type PerformanceProtocol = typeof PERFORMANCE_PROTOCOL;

/** Existing product/fixture lifetimes, not tunable performance budgets. */
export const RUNTIME_LIVENESS = Object.freeze({ heartbeatIntervalMs: 15_000, heartbeatRequestTimeoutMs: 10_000,
  runtimeFreshnessMs: 150_000, hostHarnessLifetimeMs: 300_000 });

/** Project only bounded liveness facts from the owner's runtime-list response. */
export function runtimeAvailability(body: unknown, runtime: { runtimeId: string; workspaceId: string; daemonId: string }, ownerId: string, nowMs: number) {
  const rows = Array.isArray(body) ? body.filter((row): row is Record<string, unknown> => typeof row === "object" && row !== null && row.id === runtime.runtimeId) : [];
  const row = rows.length === 1 ? rows[0] : undefined;
  const status = row?.status === "online" || row?.status === "offline" ? row.status : "unknown";
  const lastSeen = typeof row?.last_seen_at === "string" ? Date.parse(row.last_seen_at) : Number.NaN;
  const age = Number.isFinite(lastSeen) ? nowMs - lastSeen : null;
  let error: string | null = null;
  if (!row) error = "runtime_state_unavailable";
  else if (row.owner_id !== ownerId || row.workspace_id !== runtime.workspaceId || row.daemon_id !== runtime.daemonId) error = "runtime_identity_mismatch";
  else if (status === "offline" || (age !== null && age >= RUNTIME_LIVENESS.runtimeFreshnessMs)) error = "runtime_offline";
  else if (status !== "online" || age === null) error = "runtime_state_unavailable";
  return { runtime_id: runtime.runtimeId, owner_user_id: ownerId, status, last_seen_at: Number.isFinite(lastSeen) ? new Date(lastSeen).toISOString() : null,
    last_seen_age_ms: age, error_code: error };
}

/** Keys present, absent or different from the registered protocol. */
export function preregistrationMismatches(candidate: unknown): string[] {
  const registered: Record<string, number> = PERFORMANCE_PROTOCOL;
  if (typeof candidate !== "object" || candidate === null || Array.isArray(candidate)) return Object.keys(registered);
  const supplied = new Map(Object.entries(candidate));
  const mismatched = Object.keys(registered).filter((key) => supplied.get(key) !== registered[key]);
  for (const key of supplied.keys()) if (!(key in registered)) mismatched.push(key);
  return mismatched;
}

// ---------------------------------------------------------------- percentiles

/** 1-based nearest rank, ceil(N*p/100) in integer arithmetic. */
export function nearestRankIndex(count: number, percentile: number): number {
  if (!Number.isSafeInteger(count) || count < 1) throw new RangeError("A percentile needs at least one sample");
  if (!Number.isSafeInteger(percentile) || percentile < 1 || percentile > 100) throw new RangeError("The percentile must be an integer from 1 to 100");
  return Math.floor((count * percentile + 99) / 100);
}

/** Non-converged samples are passed as +Infinity; they are never removed. */
export function nearestRank(values: readonly number[], percentile: number): number {
  const rank = nearestRankIndex(values.length, percentile);
  for (const value of values) {
    if (typeof value !== "number" || Number.isNaN(value) || value < 0) throw new RangeError("A latency must be a non-negative number or +Infinity");
  }
  const sorted = [...values].sort((left, right) => (left === right ? 0 : left < right ? -1 : 1));
  const selected = sorted[rank - 1];
  if (selected === undefined) throw new RangeError("The percentile rank is outside the sample");
  return selected;
}

export interface QuantileSummary { count: number; missing: number; p50: number; p95: number; p99: number; max: number }

export function summarize(values: readonly number[]): QuantileSummary {
  return { count: values.length, missing: values.filter((value) => value === Number.POSITIVE_INFINITY).length,
    p50: nearestRank(values, 50), p95: nearestRank(values, 95), p99: nearestRank(values, 99), max: nearestRank(values, 100) };
}

export interface QuantileReport {
  count: number; missing: number;
  p50_ms: number | null; p95_ms: number | null; p99_ms: number | null; max_ms: number | null;
  p95_finite: boolean;
}

/** JSON has no Infinity: an unbounded percentile is null with an explicit flag, never 0. */
export function reportQuantiles(summary: QuantileSummary): QuantileReport {
  const finite = (value: number) => (Number.isFinite(value) ? value : null);
  return { count: summary.count, missing: summary.missing, p50_ms: finite(summary.p50), p95_ms: finite(summary.p95),
    p99_ms: finite(summary.p99), max_ms: finite(summary.max), p95_finite: Number.isFinite(summary.p95) };
}

// -------------------------------------------------------------- preparation

/**
 * Stateless seeded delay in [0, relayPeriodMs). The relay scans every 500 ms and
 * each barrier ends shortly after a scan, so an immediate write would always land
 * at the same phase of that period and narrow the distribution.
 */
export function phaseJitterMs(seed: number, ordinal: number): number {
  if (!Number.isSafeInteger(seed) || seed < 0) throw new RangeError("The jitter seed must be a non-negative integer");
  if (!Number.isSafeInteger(ordinal) || ordinal < 0) throw new RangeError("The jitter ordinal must be a non-negative integer");
  let state = ((seed % 0x1_0000_0000) + Math.imul(ordinal + 1, 0x9e3779b9)) >>> 0;
  state = Math.imul(state ^ (state >>> 15), state | 1);
  state ^= state + Math.imul(state ^ (state >>> 7), state | 61);
  const unit = ((state ^ (state >>> 14)) >>> 0) / 0x1_0000_0000;
  return Math.floor(unit * PERFORMANCE_PROTOCOL.relayPeriodMs);
}

/**
 * The server counts registrations per IP in fixed windows aligned to the wall
 * clock. Returns how long to wait before the next attempt may be sent.
 */
export function registrationWaitMs(attemptWallClockMs: readonly number[], nowWallClockMs: number): number {
  const window = PERFORMANCE_PROTOCOL.registrationWindowMs;
  const current = Math.floor(nowWallClockMs / window);
  const used = attemptWallClockMs.filter((at) => Math.floor(at / window) === current).length;
  if (used < PERFORMANCE_PROTOCOL.registrationPerWindow) return 0;
  return (current + 1) * window - nowWallClockMs + PERFORMANCE_PROTOCOL.registrationMarginMs;
}

/** Two distinct participants that are neither the triggering account nor the resource owner. */
export function pickWritebackReaders(runIndex: number, ownerIndex: number, total: number): { reader: number; hint: number } {
  const excluded = new Set([runIndex, ownerIndex]);
  if (total - excluded.size < 2) throw new RangeError("Too few participants for independent write-back readers");
  const picked: number[] = [];
  for (let offset = 1; picked.length < 2; offset += 1) {
    const candidate = (runIndex + offset) % total;
    if (!excluded.has(candidate)) { picked.push(candidate); excluded.add(candidate); }
  }
  const [reader, hint] = picked;
  if (reader === undefined || hint === undefined) throw new RangeError("Too few participants for independent write-back readers");
  return { reader, hint };
}

// --------------------------------------------------------- frame correlation

const DECIMAL = /^[1-9][0-9]*$/;
const CHANGE_KEYS = ["authority_epoch", "collaboration_revision", "event_seq", "issue_id"];

export interface FrameArrival {
  event_seq: string; event_id: string;
  /** The revision at relay time, not at event time. Recorded, never matched on. */
  frame_revision: string;
  authority_epoch: string;
  first_ms: number; duplicates: number; conflicting_event_ids: number;
}
export interface FrameLedger { issue_id: string; arrivals: Map<string, FrameArrival>; ignored: Map<string, number> }
export type FrameDisposition =
  | { kind: "first"; arrival: FrameArrival }
  | { kind: "duplicate"; arrival: FrameArrival }
  | { kind: "ignored"; reason: string };

export function createFrameLedger(issueId: string): FrameLedger {
  return { issue_id: issueId.toLowerCase(), arrivals: new Map(), ignored: new Map() };
}

function ignore(ledger: FrameLedger, reason: string): FrameDisposition {
  ledger.ignored.set(reason, (ledger.ignored.get(reason) ?? 0) + 1);
  return { kind: "ignored", reason };
}

/**
 * Records every issue-scope change hint with its first arrival time, whether or
 * not a round is waiting for it, so a frame that beats the write response keeps
 * its original time. Anything else is counted by reason only; no content is kept.
 */
export function recordFrame(ledger: FrameLedger, frame: unknown, atMs: number): FrameDisposition {
  if (typeof frame !== "object" || frame === null || Array.isArray(frame)) return ignore(ledger, "not_an_object");
  const { type, event_id: eventId, payload } = frame as { type?: unknown; event_id?: unknown; payload?: unknown };
  if (type !== "collaboration:changed") return ignore(ledger, typeof type === "string" && /^[a-z_:]{1,64}$/.test(type) ? `type:${type}` : "type:unrecognized");
  if (typeof eventId !== "string" || !eventId.endsWith(":issue")) return ignore(ledger, "not_issue_scope_event");
  if (typeof payload !== "object" || payload === null || Array.isArray(payload)) return ignore(ledger, "malformed_payload");
  const fields = payload as Record<string, unknown>;
  if (Object.keys(fields).sort().join() !== CHANGE_KEYS.join()) return ignore(ledger, "malformed_payload");
  const { issue_id: issueId, event_seq: seq, collaboration_revision: revision, authority_epoch: epoch } = fields;
  if (typeof issueId !== "string" || typeof seq !== "string" || typeof revision !== "string" || typeof epoch !== "string"
    || !DECIMAL.test(seq) || !DECIMAL.test(revision) || !DECIMAL.test(epoch)) return ignore(ledger, "malformed_payload");
  if (issueId.toLowerCase() !== ledger.issue_id) return ignore(ledger, "other_issue");
  const known = ledger.arrivals.get(seq);
  if (known) {
    if (known.event_id === eventId) known.duplicates += 1;
    else known.conflicting_event_ids += 1;
    return { kind: "duplicate", arrival: known };
  }
  const arrival: FrameArrival = { event_seq: seq, event_id: eventId, frame_revision: revision, authority_epoch: epoch,
    first_ms: atMs, duplicates: 0, conflicting_event_ids: 0 };
  ledger.arrivals.set(seq, arrival);
  return { kind: "first", arrival };
}

/** The sequence of the one event a status write produces after a quiescent read of `before`. */
export function expectedEventSeq(before: string): bigint {
  if (!/^(0|[1-9][0-9]*)$/.test(before)) throw new RangeError("The authoritative event sequence must be a decimal string");
  return BigInt(before) + 1n;
}

export function matchExpected(ledger: FrameLedger, expected: bigint): FrameArrival | null {
  return ledger.arrivals.get(expected.toString()) ?? null;
}

/** An observer is quiescent at `seq` once it received that frame or subscribed after it was committed. */
export function barrierSatisfied(ledger: FrameLedger, seq: bigint, snapshotSeq: bigint): boolean {
  return seq <= snapshotSeq || ledger.arrivals.has(seq.toString());
}

export function revisionAnomaly(frameRevision: string, receiptRevision: string): boolean {
  return BigInt(frameRevision) !== BigInt(receiptRevision);
}

// ---------------------------------------------------------- sample decisions

export type SampleResult = "converged" | "missing" | "not_run";
export interface WriteOutcome { ok: boolean; receipt_revision?: string; ack_ms?: number; error_code?: string }
export interface AuthoritativeRead { end_ms: number; status: string; collaboration_revision: string; event_seq: string }
export interface ObserverDecisionInput {
  request_start_ms: number; deadline_ms: number; socket_open: boolean;
  expected_event_seq: bigint; intended_status: string;
  write: WriteOutcome; arrival: FrameArrival | null; read: AuthoritativeRead | null;
}
export interface ObserverDecision { result: "converged" | "missing"; latency_ms: number | null; event_latency_ms: number | null; error_code: string | null }

export function readSatisfies(read: AuthoritativeRead, intendedStatus: string, expectedSeq: bigint, receiptRevision: string): boolean {
  return read.status === intendedStatus && DECIMAL.test(read.collaboration_revision) && /^(0|[1-9][0-9]*)$/.test(read.event_seq)
    && BigInt(read.collaboration_revision) >= BigInt(receiptRevision) && BigInt(read.event_seq) >= expectedSeq;
}

/**
 * One observer of one event. The matching frame is required: a poll that happens
 * to see the new state is recovery, not realtime propagation.
 */
export function classifyObserverSample(input: ObserverDecisionInput): ObserverDecision {
  const missing = (code: string, eventLatency: number | null = null): ObserverDecision =>
    ({ result: "missing", latency_ms: null, event_latency_ms: eventLatency, error_code: code });
  if (!input.write.ok || input.write.receipt_revision === undefined) return missing(input.write.error_code ?? "write_failed");
  if (input.arrival === null) return missing(input.socket_open ? "no_matching_frame" : "socket_closed");
  const eventLatency = input.arrival.first_ms - input.request_start_ms;
  if (eventLatency > input.deadline_ms) return missing("deadline_exceeded", eventLatency);
  if (input.read === null || !readSatisfies(input.read, input.intended_status, input.expected_event_seq, input.write.receipt_revision)) {
    return missing("read_not_converged", eventLatency);
  }
  const latency = input.read.end_ms - input.request_start_ms;
  if (latency > input.deadline_ms) return missing("deadline_exceeded", eventLatency);
  return { result: "converged", latency_ms: latency, event_latency_ms: eventLatency, error_code: null };
}

// ------------------------------------------------- propagation denominator

export interface PropagationRow {
  phase: string; event_index: number; observer_index: number; result: string; latency_ms: number | null; error_code?: string | null;
}

/** A failed write or failed preparation: the event still owes all of its samples. */
export function failedEventRows(phase: "warmup" | "formal", eventIndex: number, errorCode: string): PropagationRow[] {
  return Array.from({ length: PERFORMANCE_PROTOCOL.observers }, (_unused, observer) =>
    ({ phase, event_index: eventIndex, observer_index: observer, result: "missing", latency_ms: null, error_code: errorCode }));
}

/** Planned formal events that an aborted run never reached. */
export function notRunEventRows(eventIndex: number, reason: string): PropagationRow[] {
  return Array.from({ length: PERFORMANCE_PROTOCOL.observers }, (_unused, observer) =>
    ({ phase: "formal", event_index: eventIndex, observer_index: observer, result: "not_run", latency_ms: null, error_code: reason }));
}

function validIndex(value: unknown, limit: number): value is number {
  return typeof value === "number" && Number.isSafeInteger(value) && value >= 0 && value < limit;
}

function sampleLatency(row: { result: string; latency_ms: number | null }, deadlineMs: number): { value: number; reclassified: boolean } {
  if (row.result === "missing" || row.result === "not_run") return { value: Number.POSITIVE_INFINITY, reclassified: false };
  if (row.result !== "converged") throw new RangeError(`Unknown sample result ${JSON.stringify(row.result)}`);
  if (typeof row.latency_ms !== "number" || !Number.isFinite(row.latency_ms) || row.latency_ms < 0) {
    throw new RangeError("A converged sample needs a finite non-negative latency");
  }
  return row.latency_ms > deadlineMs ? { value: Number.POSITIVE_INFINITY, reclassified: true } : { value: row.latency_ms, reclassified: false };
}

export interface PropagationAccounting {
  denominator: number; events: number; observers: number;
  warmup_rows: number; formal_rows: number;
  converged: number; missing: number; not_run: number;
  /** Planned pairs with no row at all: filled as not run so the denominator stays whole. */
  filled_not_run: number;
  over_deadline_reclassified: number;
  /** latency[event][observer], +Infinity for every non-converged pair. */
  matrix: number[][];
}

/**
 * Rebuilds the complete formal matrix from raw rows. Warm-up rows are counted
 * and otherwise ignored; a formal pair may appear once; an absent pair is a
 * sample that was owed and not delivered.
 */
export function accountPropagation(rows: readonly PropagationRow[]): PropagationAccounting {
  const { observers, formalEvents: events, warmupEvents, propagationDeadlineMs } = PERFORMANCE_PROTOCOL;
  const matrix: number[][] = Array.from({ length: events }, () => Array.from({ length: observers }, () => Number.NaN));
  const warmupSeen = new Set<string>();
  const accounting: PropagationAccounting = { denominator: events * observers, events, observers, warmup_rows: 0, formal_rows: 0,
    converged: 0, missing: 0, not_run: 0, filled_not_run: 0, over_deadline_reclassified: 0, matrix };
  for (const row of rows) {
    if (row.phase !== "warmup" && row.phase !== "formal") throw new RangeError(`Unknown sample phase ${JSON.stringify(row.phase)}`);
    if (!validIndex(row.event_index, row.phase === "warmup" ? warmupEvents : events)) throw new RangeError("event_index is outside the registered plan");
    if (!validIndex(row.observer_index, observers)) throw new RangeError("observer_index is outside the registered plan");
    const sample = sampleLatency(row, propagationDeadlineMs);
    const key = `${row.event_index}:${row.observer_index}`;
    if (row.phase === "warmup") {
      if (warmupSeen.has(key)) throw new RangeError(`duplicate warm-up sample ${key}`);
      warmupSeen.add(key);
      accounting.warmup_rows += 1;
      continue;
    }
    const line = matrix[row.event_index];
    if (line === undefined || !Number.isNaN(line[row.observer_index])) throw new RangeError(`duplicate formal sample ${key}`);
    line[row.observer_index] = sample.value;
    accounting.formal_rows += 1;
    if (sample.reclassified) { accounting.over_deadline_reclassified += 1; accounting.missing += 1; }
    else if (row.result === "converged") accounting.converged += 1;
    else if (row.result === "missing") accounting.missing += 1;
    else accounting.not_run += 1;
  }
  for (const line of matrix) {
    for (let observer = 0; observer < observers; observer += 1) {
      if (Number.isNaN(line[observer])) { line[observer] = Number.POSITIVE_INFINITY; accounting.filled_not_run += 1; }
    }
  }
  return accounting;
}

export interface PropagationVerdict {
  denominator: number; missing: number; budget_ms: number; deadline_ms: number;
  pooled: QuantileSummary;
  /** Distribution over events of the slowest observer of each event. */
  slowest_per_event: QuantileSummary;
  per_observer: QuantileSummary[];
  observers_over_budget: number[];
  /** Formal event k is written by observer k; its own sample is reported apart. */
  excluding_writer: QuantileSummary;
  conditions: { zero_missing: boolean; pooled_p95_within_budget: boolean; slowest_observer_p95_within_budget: boolean };
  pass: boolean;
}

/** All three conditions are required; none can stand in for another. */
export function propagationVerdict(accounting: PropagationAccounting): PropagationVerdict {
  const budget = PERFORMANCE_PROTOCOL.propagationBudgetMs;
  const pooledValues = accounting.matrix.flat();
  const pooled = summarize(pooledValues);
  const slowest = summarize(accounting.matrix.map((line) => nearestRank(line, 100)));
  const perObserver = Array.from({ length: accounting.observers }, (_unused, observer) =>
    summarize(accounting.matrix.map((line) => line[observer] ?? Number.POSITIVE_INFINITY)));
  const excludingWriter = summarize(accounting.matrix.flatMap((line, event) => line.filter((_value, observer) => observer !== event)));
  const conditions = { zero_missing: pooled.missing === 0 && pooledValues.length === accounting.denominator,
    pooled_p95_within_budget: pooled.p95 <= budget, slowest_observer_p95_within_budget: slowest.p95 <= budget };
  return { denominator: accounting.denominator, missing: pooled.missing, budget_ms: budget, deadline_ms: PERFORMANCE_PROTOCOL.propagationDeadlineMs,
    pooled, slowest_per_event: slowest, per_observer: perObserver,
    observers_over_budget: perObserver.flatMap((summary, observer) => (summary.p95 > budget ? [observer] : [])),
    excluding_writer: excludingWriter, conditions,
    pass: conditions.zero_missing && conditions.pooled_p95_within_budget && conditions.slowest_observer_p95_within_budget };
}

// --------------------------------------------------- write-back denominator

export interface WritebackRow { phase: string; run_index: number; metric: string; result: string; latency_ms: number | null; error_code?: string | null }
export interface WritebackMetricAccounting {
  denominator: number; rows: number; converged: number; missing: number; not_run: number; filled_not_run: number;
  over_deadline_reclassified: number; values: number[];
}
export interface WritebackAccounting { warmup_rows: number; business: WritebackMetricAccounting; finalization: WritebackMetricAccounting }

export function accountWriteback(rows: readonly WritebackRow[]): WritebackAccounting {
  const { writebackFormalRuns: runs, writebackWarmupRuns, writebackDeadlineMs } = PERFORMANCE_PROTOCOL;
  const metric = (): WritebackMetricAccounting => ({ denominator: runs, rows: 0, converged: 0, missing: 0, not_run: 0, filled_not_run: 0,
    over_deadline_reclassified: 0, values: Array.from({ length: runs }, () => Number.NaN) });
  const accounting: WritebackAccounting = { warmup_rows: 0, business: metric(), finalization: metric() };
  const warmupSeen = new Set<string>();
  for (const row of rows) {
    if (row.phase !== "warmup" && row.phase !== "formal") throw new RangeError(`Unknown sample phase ${JSON.stringify(row.phase)}`);
    if (row.metric !== "business" && row.metric !== "finalization") throw new RangeError(`Unknown write-back metric ${JSON.stringify(row.metric)}`);
    if (!validIndex(row.run_index, row.phase === "warmup" ? writebackWarmupRuns : runs)) throw new RangeError("run_index is outside the registered plan");
    const sample = sampleLatency(row, writebackDeadlineMs);
    const key = `${row.metric}:${row.run_index}`;
    if (row.phase === "warmup") {
      if (warmupSeen.has(key)) throw new RangeError(`duplicate warm-up sample ${key}`);
      warmupSeen.add(key);
      accounting.warmup_rows += 1;
      continue;
    }
    const target = accounting[row.metric];
    if (!Number.isNaN(target.values[row.run_index])) throw new RangeError(`duplicate formal sample ${key}`);
    target.values[row.run_index] = sample.value;
    target.rows += 1;
    if (sample.reclassified) { target.over_deadline_reclassified += 1; target.missing += 1; }
    else if (row.result === "converged") target.converged += 1;
    else if (row.result === "missing") target.missing += 1;
    else target.not_run += 1;
  }
  for (const target of [accounting.business, accounting.finalization]) {
    for (let run = 0; run < runs; run += 1) {
      if (Number.isNaN(target.values[run])) { target.values[run] = Number.POSITIVE_INFINITY; target.filled_not_run += 1; }
    }
  }
  return accounting;
}

export interface WritebackGate { denominator: number; missing: number; budget_ms: number; summary: QuantileSummary; pass: boolean }
export interface WritebackVerdict { business: WritebackGate; finalization: WritebackGate; pass: boolean }

/** Business result and finalization are separate gates over separate complete denominators. */
export function writebackVerdict(accounting: WritebackAccounting): WritebackVerdict {
  const budget = PERFORMANCE_PROTOCOL.writebackBudgetMs;
  const gate = (metric: WritebackMetricAccounting): WritebackGate => {
    const summary = summarize(metric.values);
    return { denominator: metric.denominator, missing: summary.missing, budget_ms: budget, summary,
      pass: metric.values.length === metric.denominator && summary.missing === 0 && summary.p95 <= budget };
  };
  const business = gate(accounting.business);
  const finalization = gate(accounting.finalization);
  return { business, finalization, pass: business.pass && finalization.pass };
}
