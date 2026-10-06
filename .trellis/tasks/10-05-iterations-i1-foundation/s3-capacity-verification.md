# S3 foundation writer capacity evidence

Status: **the bounded FG writer-capacity probe passes after the read-pipeline
optimization**. The initial failures remain below. The post-change runs meet
the original frozen thresholds, with a narrow populated four-writer P95 margin.
This is not lifecycle/history/closure acceptance or FG approval.

## Measurement plan frozen before runs (2026-10-06)

- Use only the task-owned PostgreSQL database `multica_i1_perf_20261006c`.
- Call the real `Handler.UpdateIssue` title-edit path through HTTP request and
  response objects, including its transaction, authorization, recorder, and
  post-commit handling. Do not substitute recorder-only or synthetic endpoints.
- Four workspaces, each with 1,200 issues: 400 active, 400 planned, 400
  unassociated. Each workspace contains 10,000 events in each of its active,
  planned, and completed iterations: 120,000 events total before measurement.
  Active/planned participants have full before facts; completed participation
  retains original facts and a leave time. The fixture is warm after explicit
  VACUUM (ANALYZE) and warmup, and all data creation is outside timing. Vacuum
  removes dead tuples from earlier fixture runs before the measured workload.
- Compare ordinary unassociated edits against alternating active/planned edits
  on the **same populated database**, with one and four concurrent writers in
  one workspace. The other three workspaces supply index/history population.
  Each scenario has three independent 300-write samples and zero HTTP errors.
- Retain the existing ordinary small-workspace benchmark and its original
  thresholds unchanged. For the new populated unassociated case, use those
  same floors: median writes/s >= 374.85 (one) / 570 (four), median HTTP P95
  <= 5.704 / 10 ms, each run HTTP P95 <= 20 ms.
- For populated associated edits, freeze median throughput >= 50% of the paired
  populated unassociated baseline, median HTTP P95 <= 2.5 times that baseline,
  and every run HTTP P95 <= 40 ms. This bounds additional recorder round trips
  and workspace serialization; it is not a claim that recording is free.
- Enumerate transaction statement counts: expected eight for unassociated and
  twelve for title-only associated updates; require <= twelve for the latter.
  Begin/Commit and reads outside the owning transaction are excluded from this
  stable counter and reported separately via pgx tracing.
- Sample `pg_stat_activity.wait_event_type='Lock'` and `pg_blocking_pids` using
  an independent connection, nominal 1 ms cadence. Report actual sample count,
  mean sample interval, blocked observations, event names, and estimated
  blocked-backend-ms/write. Treatment median estimated blocked-backend-ms/write
  must be <= 2.5 times paired baseline + 2 ms. The additive allowance covers
  sub-sample short waits; this sampled estimate is not exact wait duration.
  A controlled real blocker must appear by backend PID in `pg_blocking_pids`
  before its release, so a broken monitor cannot masquerade as zero waits.
  Root review accepted these engineering ratios before runs. Treat the sampled
  bound as diagnostic when a baseline lacks sufficient blocked observations;
  zero sampled waits cannot pass a pure lock-wait duration gate.
- Record EXPLAIN (ANALYZE, BUFFERS) for actual indexed current-iteration,
  participation and latest-event lookups over this fixture, without forced
  index settings. Record measured relation sizes, PostgreSQL version, hardware,
  and DB settings. No production capacity extrapolation from localhost.

These limits are defined before any timed measurement. Root integration owns
acceptance review. Failed limits remain blockers; process exit zero alone does
not pass a performance gate. The PRD's suggested 1,000-task detail/preview P95
of two seconds is not used as an acceptance threshold here.

## Remaining scope

Lifecycle detail/full preview, start/move, end/handoff, whole-workspace disable
(including greater-than-limit 413 atomicity), cold cache, production network,
and Web/Desktop E2E require their later gate fixtures and measurements.

## Environment and preparatory checks

- Host measured in this session: Apple M4 arm64, 17,179,869,184 bytes RAM
  (16 GiB), Darwin 25.2.0; Go 1.27.0 darwin/arm64.
- Guarded handler test compilation passed after authoring the fixture and
  sampler (`go -C server test -c ./internal/handler -o
  /tmp/multica-i1-capacity-handler.test`). This compiles tests only; it does not
  establish database correctness or performance.
- HTTP P95 times the handler call. Throughput additionally includes request
  construction and response recording, matching the ordinary probe's boundary.
  Tracer statements cover this handler's dedicated pool, including Begin and
  Commit; the stable transaction counter counts SQL statements in SendBatch
  individually. Observation and fixture queries use separate connections and
  are excluded from both counters.

## Database correctness before timed runs

On the isolated database above, both commands exited zero:

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 \
  ./internal/handler -run '^TestIterationWriterServerLockWait$' -count=1 -v
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -p 2 \
  ./internal/handler -run '^$' -bench '^BenchmarkIterationPopulatedWrite$' \
  -benchtime=1x -count=1
```

- Lock proof: PostgreSQL reported the exact holder PID in `pg_blocking_pids`
  while the writer waited on an advisory lock. The actual benchmark sampler
  recorded 12 blocked observations. No event existed before release; exactly
  one existed afterward. Race detector passed. Log:
  `/tmp/multica-i1-capacity-lock-proof.log`.
- The first lock-proof attempt failed HTTP404 because the test's timeout
  context replaced Chi's route context. Restoring the route parameter after
  timeout context injection fixed the fixture; production code did not change.
- Population smoke: all four benchmark cases passed with 4,800 issues and
  120,000 seed events; timed edits produced exactly the expected events and
  scope increments. Transaction statements were eight/twelve; traced handler
  statements were twelve/sixteen. Log:
  `/tmp/multica-i1-capacity-fixture-smoke.log`. The 1x timing values are explicitly
  excluded from performance acceptance; this run only validates the harness.

## Initial timed measurements (2026-10-06, exclusive agent benchmark window)

Root explicitly started the benchmark window after stopping concurrent heavy
agent commands. The following were run sequentially, once each, on the isolated
database; both exited zero, with no failed requests or event/scope assertions.
No reruns were used to select nicer numbers. Working tree base: `e683c08d9`;
the new benchmark/evidence files were uncommitted during measurement.

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -p 2 \
  ./internal/handler -run '^$' -bench '^BenchmarkIterationPopulatedWrite$' \
  -benchtime=300x -count=3 -v
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -p 2 \
  ./internal/handler -run '^$' -bench '^BenchmarkIterationOrdinaryWrite$' \
  -benchtime=300x -count=3 -v
```

PostgreSQL 17.11 Debian aarch64, localhost TCP, READ COMMITTED, shared_buffers
128 MB; measured iteration_event total relation size 142 MB, including indexes
and reusable relation pages. Host/Go are listed above; benchmark reports ten
logical CPUs. Data was vacuumed/analyzed then warmed; these are not cold-cache
measurements. Logs: `/tmp/multica-i1-capacity-populated.log` (18.183 s),
`/tmp/multica-i1-capacity-ordinary.log` (4.787 s). Parsed numeric evidence:
`/tmp/multica-i1-capacity-metrics.json`.

| Probe | Writers | Writes/s, three runs | HTTP P95 ms, three runs | Tx / all traced statements |
| --- | --- | --- | --- | --- |
| Populated, unassociated | 1 | 420.1 / 402.1 / 442.6 | 3.890 / 4.192 / 3.565 | 8 / 12 |
| Populated, active/planned | 1 | 294.6 / 242.7 / 286.7 | 5.355 / 8.653 / 6.115 | 12 / 16 |
| Populated, unassociated | 4 | 445.7 / 563.3 / 401.9 | 12.59 / 10.17 / 16.22 | 8 / 12 |
| Populated, active/planned | 4 | 350.8 / 370.7 / 360.1 | 16.88 / 14.12 / 16.03 | 12 / 16 |
| Original small fixture | 1 | 438.9 / 454.5 / 391.2 | 3.558 / 3.305 / 4.096 | 8 / not instrumented |
| Original small fixture | 4 | 498.4 / 555.5 / 547.1 | 11.30 / 12.02 / 10.58 | 8 / not instrumented |

Transaction statement cost is fixed for this title-only fixture: the existing
eight statements plus current participation lock, database wall-clock sample,
event append, and scope revision increment. The handler pool trace adds Begin,
Commit, and two reads outside the transaction. Server-internal trigger SQL and
network-protocol preparation are not separately counted.

| Frozen check | Observed comparison | Result |
| --- | --- | --- |
| Populated unassociated, one writer | Median 420.1/s >= 374.85; P95 3.890 <= 5.704 ms | Pass |
| Populated unassociated, four writers | Median 445.7/s < 570; P95 12.59 > 10 ms | **Fail** |
| Associated one-writer relative cost | 286.7/420.1 = 68.25% >= 50%; P95 6.115/3.890 = 1.572 <= 2.5 | Pass |
| Associated four-writer relative cost | 360.1/445.7 = 80.79% >= 50%; P95 16.03/12.59 = 1.273 <= 2.5 | Pass, against a failing baseline |
| Per-run P95 and associated SQL bound | All unassociated < 20 ms; all associated < 40 ms and exactly 12 tx statements/write | Pass |
| Original small fixture, one writer | Median 438.9/s >= 374.85; P95 3.558 <= 5.704 ms | Pass |
| Original small fixture, four writers | Median 547.1/s < 570; P95 11.30 > 10 ms | **Fail** |

All 3,600 populated timed edits and 1,800 original-probe edits returned HTTP200.
Associated edits added exactly one event and one scope increment per write;
unassociated edits added neither. The relative treatment pass does not override
either absolute baseline failure. These results supersede earlier passing
ordinary benchmarks at this checkpoint; the justified post-change runs below
establish the final result.

## Server waits and query plans

The one-writer scenarios collected 667–1,210 samples per run and observed no
blocked backends. This is diagnostic evidence of no *sampled* contention; it is
not a measured zero-duration lock-wait bound and does not pass one.

| Populated four-writer probe | Samples, three runs | Blocked observations | Blocker links | Estimated blocked-backend ms/write |
| --- | --- | --- | --- | --- |
| Unassociated | 582 / 526 / 720 | 1,252 / 1,107 / 1,489 | 2,022 / 1,732 / 2,282 | 5.023 / 3.705 / 5.105 |
| Active/planned | 843 / 803 / 832 | 2,075 / 1,952 / 2,013 | 3,702 / 3,456 / 3,556 | 6.905 / 6.472 / 6.669 |

All observed waits were PostgreSQL `advisory`; actual mean sample intervals
were 1.001–1.157 ms. The four-writer sample budget was substantial and the
diagnostic bound holds: median 6.669 <= 2.5 × 5.023 + 2 = 14.5575 ms/write.
Observed waiting can be on the subscriber fence or I1 fence; no attribution
to I1 alone is made from `wait_event` names. The separate controlled test
proves I1's exact blocker and the sampler itself. Client lock-statement median
durations were 6.888 versus 8.658 ms/write for the populated four-writer cases;
these include round trips/execution and are deliberately distinct from server
wait estimates.

EXPLAIN (ANALYZE, BUFFERS), using production-equivalent lookup SQL and no forced
planner settings, is captured in the populated log:

- Current iteration: `idx_issue_workspace_id_keyset` resolves one issue, joined
  to the small twelve-row iteration table; execution 0.022 ms.
- Current participation: `iteration_participation_issue` index scan resolves
  one participation; execution 0.010 ms.
- Latest event: backward `iteration_event_sequence` scan returns one row from
  the populated history; execution 0.012 ms, with workspace predicate retained.

These lookup plans do not show an unbounded history scan on the measured path.
They do not explain away the measured four-writer regression: lock serialization,
transport/handler costs, and environment variation still need investigation by
the owning implementation lane. No production/index optimization was made in
the initial evidence-only slice. This was the blocking result before the
subsequent bounded fix; thresholds were not relaxed.

## Post-change acceptance: one fewer round trip under the write fences

Root authorized the bounded optimization after diagnosis and a two-connection
protocol proof. `lockIssueReadRows` batches the separate current-iteration and
issue locking statements **after** the original fence batch and status check,
only when there are no attachments. SQL count, lock order, error contract and
business logic stay the same. The original attachment path is preserved.
Plan, protocol evidence, and 83 passing race regression entries are in
`s3-write-pipeline-verification.md`.

The same commands, fixtures, thresholds and exclusive-window procedure were
run once after that code change, populated first and original small second.
Both exited zero. Logs: `/tmp/multica-i1-capacity-populated-pipeline.log`
(15.707 s), `/tmp/multica-i1-capacity-ordinary-pipeline.log` (3.888 s); numeric
artifact `/tmp/multica-i1-capacity-pipeline-metrics.json`. Hardware, PostgreSQL,
shared_buffers and the 142 MB event relation measurement matched the initial
run. No acceptance threshold or benchmark scenario changed.

| Probe | Writers | Writes/s, three runs | HTTP P95 ms, three runs | Tx / all traced statements |
| --- | --- | --- | --- | --- |
| Populated, unassociated | 1 | 485.5 / 442.5 / 469.1 | 3.253 / 4.052 / 3.816 | 8 / 12 |
| Populated, active/planned | 1 | 333.5 / 341.5 / 354.8 | 4.567 / 4.519 / 4.075 | 12 / 16 |
| Populated, unassociated | 4 | 644.8 / 667.2 / 584.2 | 10.73 / 8.527 / 9.963 | 8 / 12 |
| Populated, active/planned | 4 | 373.8 / 402.9 / 377.1 | 16.69 / 13.47 / 14.80 | 12 / 16 |
| Original small fixture | 1 | 568.4 / 539.3 / 489.4 | 3.124 / 3.047 / 3.134 | 8 / not instrumented |
| Original small fixture | 4 | 686.4 / 767.5 / 696.7 | 9.218 / 7.171 / 9.168 | 8 / not instrumented |

- Populated baseline medians: 469.1/s and 3.816 ms (one writer), 644.8/s and
  9.963 ms (four writers). Both pass the existing absolute floors/ceilings.
  The four-writer P95 is only 0.037 ms below its median ceiling; this is a
  passing local regression probe with limited headroom, not a production SLA.
- Associated median throughput is 72.80% (one) and 58.48% (four) of baseline;
  median P95 ratios are 1.184 and 1.485. Both pass the frozen 50%/2.5× limits.
  Every individual P95 is below its 20/40 ms ceiling; SQL remains 8/12.
- Original small medians: 539.3/s and 3.124 ms (one), 696.7/s and 9.168 ms
  (four). All original thresholds pass unchanged.
- All 3,600 populated and 1,800 original timed requests succeeded. Populated
  event and scope-delta assertions also passed in every case.

Post-change four-writer server samples:

| Probe | Samples | Blocked observations | Blocker links | Estimated blocked-backend ms/write |
| --- | --- | --- | --- | --- |
| Unassociated | 462 / 448 / 511 | 920 / 876 / 1,020 | 1,405 / 1,315 / 1,555 | 3.062 / 2.913 / 3.406 |
| Active/planned | 798 / 742 / 789 | 1,941 / 1,786 / 1,916 | 3,421 / 3,140 / 3,382 | 6.395 / 5.889 / 6.349 |

All were advisory waits; mean sample cadence was 1.004–1.008 ms. The sampled
bound passes with substantial observations: 6.349 <= 2.5 × 3.062 + 2 = 9.655
ms/write. Single-writer observations remain zero (614–896 samples/run), so that
case remains diagnostic only, without a claimed pure wait-time bound.
Four-writer client lock-statement median fell from 6.888 to 4.226 ms for the
populated baseline; its sampled server wait estimate fell from 5.023 to 3.062
ms/write. This is consistent with less time under the serialized locks.

Lookup plans remained indexed, with measured execution times 0.028 ms for
current iteration, 0.016 ms for participation and 0.009 ms for latest event.
No additional capacity claim is made for lifecycle details/previews, closure,
whole-workspace disable, cold caches or production network conditions.
