# Collaboration performance sampling

## 1. Scope / trigger

Use for `e2e/collaboration-performance.spec.ts`, its three performance fixtures,
and task-owned performance launchers. This suite measures the platform through
real account HTTP/WebSocket sessions and a controlled Provider. It does not
measure model quality or external payment, CI, deployment or rollback correctness.
A rehearsal proves only its frozen source and configuration. Final acceptance
must rerun on the final candidate after its feature dependencies stabilize.

## 2. Signatures

- Pure accounting: `e2e/fixtures/collaboration-performance-stats.ts`.
- Live orchestration: `e2e/fixtures/collaboration-performance.ts`.
- Regression entry: `node --experimental-strip-types --test e2e/fixtures/collaboration-performance-stats.node-test.mjs`.
- Live entry: `pnpm exec playwright test e2e/collaboration-performance.spec.ts -g '<phase>:' --workers=1 --retries=0`, in order `prepare`, `propagation`, `writeback`.
- Opt-in: `COLLABORATION_PERF_ACCEPTANCE=1`, `COLLABORATION_PERF_RUN_DIR`,
  `COLLABORATION_PERF_MANIFEST`, explicit owned `DATABASE_URL`, API origin and
  `MULTICA_DEPLOYMENT_ID`. Writeback additionally pins the host daemon, Provider,
  CLI and Provider-source paths/hashes.
- Heartbeat: enrolled owner-scoped `mdt_` credential, `POST /api/daemon/heartbeat`
  with `{runtime_id, supports_batch_import:true}`. The HTTP success ACK is
  `{status:"ok", ...}`; it does **not** include `runtime_id`.

## 3. Contracts

The manifest must exactly match `PERFORMANCE_PROTOCOL`: 100 distinct humans,
one authenticated account socket each, 5 excluded warmup events, 100 formal
events, and 1 excluded warmup run plus 100 independent formal runs. Register via
public routes and respect the server's fixed 30-per-300-second windows. Four
windows can finish in about 10–15 minutes depending on the first window's phase;
record actual timestamps instead of imposing an assumed 15-minute minimum.

Propagation starts before the status write and ends only after the observer's
matching `collaboration:changed` frame (`event_seq=S+1`) and its own authenticated
GET prove the intended status, sequence and sufficient revision. Keep early
frames received before the write ACK. `workbench_changed`, another sequence, or
a read alone cannot substitute for the matching event. Keep relay-time revision
anomalies separately; do not use frame revision as event identity.

Writeback uses the same 100-participant issue and keeps all 100 sockets open.
Its result reader is neither the triggerer nor the resource owner. Business
latency starts before `host.act` and requires the exact attributed Agent comment.
Finalization starts before release and requires, in one read round, settled
success, `host_exit_observed`, one launch, the run's result and its comment link.
Queue/start time and the secondary hint path are informational.

Use one monotonic clock and retain UTC alongside it. The formal propagation
denominator is always 10,000. Each writeback metric has denominator 100. Missing,
late and unrun samples remain in the denominator as infinity for quantiles;
nearest-rank p95 uses `ceil(0.95*N)`. Passing requires no missing/unrun samples,
pooled **and per-event slowest-observer** p95 at most 2s, and both writeback p95s
at most 5s. Never calculate a passing distribution from survivors alone.

The sampler owns its test runtime's heartbeat: one awaited initial request,
then 15 seconds after each completed request, bounded by a 10-second request
timeout. Before each trigger, an owner-authorized runtime read checks identity,
status and freshness outside the timed intervals. Keep sanitized heartbeat
times/statuses, never tokens or raw error bodies. A failure remains recorded
even if a later beat succeeds. Stop and join the loop during teardown.

At each run boundary, reconnects must leave exactly 100 open sockets. Capture
socket identity, connection count and close observation for the run. A disconnect
or replacement during measurement invalidates that run even if it recovers;
retain observed timestamps/diagnostic latency, mark both metric rows missing and
retain the unrun remainder. Do not replace the existing connection manager.

Keep the existing 150s runtime freshness fence, 300s host-fixture process context,
30s metric cutoff and default pool settings. A heartbeat does not extend the
host fixture's process lifetime. Record daemon start immediately before starting
it, plus readiness and final elapsed times.

Bind source, binaries, actual health PID/commit, deployment and database identity.
Use external private state and unique run directories; preserve every failed run.
After sample files exist, a rerun needs a new ID/directory and fresh prepare.
Restore the gate's **allocation-time** flags, not merely the state at phase entry,
while keeping its epoch monotonic. Verify owned processes/listeners and actual
database connections separately. An unrecorded API descriptor limit stays unknown;
the sampler's limit is not a substitute.

## 4. Validation and error matrix

| Condition | Required result |
| --- | --- |
| No explicit opt-in | Three skips; no setup or network activity |
| Opt-in with mismatched/non-owned environment or protocol | Fail before sampling; never silently skip |
| Real heartbeat `{status:"ok"}` | Accept; identity belongs to the credential/request and owner read |
| Wrong owner, expired credential, malformed/failed heartbeat | Sanitized failure; no passing run |
| Fewer than 100 sockets after reconnect | `observer_unavailable`; missing current pair, remainder NOT_RUN |
| Disconnect or replacement during a run | `observer_disconnected`; recovery cannot revive its samples |
| Unknown write result | Keep original key, stop overlap, retain complete denominator |
| Duplicate event or receipt | Count anomaly/replay without adding a second sample/effect |
| Setup, daemon, socket, heartbeat or gate cleanup failure | Attempt remaining cleanup, preserve evidence and fail the phase |
| A successful phase with an HTTP error in API logs | Retain the error and exact eventual outcome; do not claim every request succeeded |

## 5. Good / base / bad cases

Good: r01's runtime-offline failure remains intact; corrected r02 uses a fresh
database, identities and source overlay and finishes all samples. Its conclusion
remains a tool rehearsal.

Base: normal repository checks leave the opt-in unset and skip this expensive
suite; the pure regression suite still checks its accounting and lifetime rules.

Bad: a fake heartbeat ACK invents `runtime_id`, 99 sockets still produce PASS,
or a failed run's remaining rows disappear before p95 calculation.

## 6. Required tests and assertion points

- Integer nearest-rank, infinity/missing handling, full denominators, warmup
  exclusion, duplicate identity, exact sequence, early-frame retention and aborts.
- Actual HTTP ACK shape, owner-only daemon credential, bounded requests,
  single-flight/sticky failure and cancellation joining an in-flight heartbeat.
- A complete 101-run fake success path as well as setup/cleanup failure paths.
  Negative-only tests missed the incompatible heartbeat success assertion.
- Boundary 99-socket rejection and timed disconnect/recovery counterexamples;
  preserve every raw pair and forbid recovery from upgrading a missing result.
- Independent arithmetic from raw samples, without importing the accounting
  implementation. Cross-check run/comment/Agent/launch identities against a
  separate read-only database snapshot where available; never store bearer tokens.
- Source/configuration hashes before and after checks, real phase exits, original
  failures, actual gate restoration, process exit and database-connection evidence.

Evidence: PAY-241 S5 `research/harness-fix-review.{md,json}` and
`research/rehearsal-r02-review.{md,json}` record the 31-test corrected harness and
79 independent rehearsal checks. r01 remains failed. Neither report grants
TC-24 or complete business acceptance.

## 7. Wrong vs correct

Wrong: assert a response field present only in a fake, count open sockets as
informational, or equate a successful phase with zero HTTP failures.

Correct: test the real ACK contract and a full success path; enforce the complete
load during measurement; preserve a transient HTTP failure separately from the
run's eventual verified settlement. Recompute all registered samples and retain
unmeasured environment details as limitations.
