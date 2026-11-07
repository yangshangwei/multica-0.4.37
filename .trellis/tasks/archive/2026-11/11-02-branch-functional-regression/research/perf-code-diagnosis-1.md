# Research: iteration carryover P95 code path

- Query: Which actual candidate code paths can account for the 1,000-task closure cost, what does the existing instrumentation measure, and what is the smallest discriminating diagnostic?
- Scope: internal; read-only source and saved evidence. No tests, service/database operations, product changes or Git operations.
- Date: 2026-11-02
- Candidate supplied by the active task: 7af9ab9905c98c77bb091223ebe687c639475bb4.

## Findings

### Current conclusion

The code proves a linear sequential read mechanism: a successful 1,000-task carryover calls the active-task query 1,000 times and individually locks source and target participation 2,000 times before its bulk writes. The bulk membership writer is already pipelined. This identifies concrete optimization/measurement candidates, but the saved aggregate tracer does not prove which stage caused the measured 171ms excess. QA-PERF-001 remains FAIL and its historical causal attribution remains unresolved.

### Exact request and transaction path

1. ApplyIterationOperation decodes and canonicalizes the envelope/draft, calls the service, then publishes the committed result and serializes HTTP JSON: server/internal/handler/iteration_management.go:159, :178 and :86.
2. IterationService.Apply normalizes the draft and requires a full preview hash; its mutation calls prepareLifecycle with lock=true, freezes closure, commits memberships, then persists the staged snapshot: server/internal/service/iteration_lifecycle_apply.go:13, :31, :32, :54, :59 and :127.
3. lifecycleOperation delegates to iteration.RunOperation using the production transaction starter: server/internal/service/iteration_lifecycle_metadata.go:111 and :119.
4. RunOperation owns the READ COMMITTED transaction, locks workspace/subscriber/member/catalog/I1 fences, reauthorizes, loads a durable prior result before mutable state, performs mutation, and saves the durable result before commit: server/internal/iteration/operation_execute.go:26, :40, :42, :48, :51, :57, :60, :63, :66, :74 and :85. The starter sets RC explicitly in server/internal/service/iteration.go:33-46.

The timed handler therefore includes decoding, pool acquisition/transaction setup, authorization and locks, data/history projection, writes and commit, after-commit publication and response JSON. It is not a query-only timer.

### Proven sequential read costs for this population

| Stage | Code mechanism | Cost for the recorded fixture |
| --- | --- | --- |
| Complete issue set | Sorted iteration IDs and sorted issue locking query | Set-based, not 1,000 per-issue locks; iteration_lifecycle_preview.go:197-205 and :227-230; queries/iteration_lifecycle.sql:68-71 |
| Current display references | CaptureIssueReferences deduplicates projects/members and reads labels/references for the full ID set | Ten project reference locks plus a member-set lock and bulk label/reference reads in this fixture; history_capture.go:40-81 and :124-158 |
| Active execution count | Each issue synchronously calls ListActiveTasksByIssue, then counts only running tasks | Exactly 1,000 query method calls per successful prepareLifecycle; iteration_lifecycle_preview.go:290-304 |
| Membership preparation | prepareClosure calls PrepareMembershipChange for each issue; source and target participation are synchronously read FOR UPDATE | Exactly 2,000 individual participation lookup calls for 1,000 source-active → target-planned moves; iteration_closure.go:43 and :102-109; membership.go:53-67; queries/iteration.sql:66-69 |
| Status category | issueFacts only fetches a status row for a non-built-in status | No additional status lookup for the recorded all-todo fixture; record.go:51-63 |

ListActiveTasksByIssue selects complete active task rows, although this caller uses only the running count; canonical SQL is server/pkg/db/queries/agent.sql:2510-2518. This is a demonstrated N-query mechanism even when every result is empty. The count is deliberately zeroed only in the comparison hash, at iteration_lifecycle_preview.go:458-461; removing the reads would change the preview response and must not be treated as a free diagnostic fix.

The 3,000 sequential method calls are a code-derived per-successful-closure count, not a measured network-packet count or a claim about their total duration. Extra transaction/references/history queries remain.

### Writes already use seven pipeline phases

For each actual source-to-target move, CommitMembershipChanges prepares source event, source leave, source scope revision, target event, target join, target scope revision and issue CAS update. It executes these seven phases via sqlc batch methods and consumes/closes all results:

- server/internal/iteration/membership_batch.go:29-35, :57-94, :97-147.
- server/pkg/db/generated/batch.go:105-123 shows one SendBatch call queues the entire event parameter slice; result consumption is :126-143.
- Canonical batch SQL: server/pkg/db/queries/iteration_membership_batch.sql:4-55.

A 1,000-task carryover therefore executes 7,000 membership SQL statements through seven bulk phase invocations, rather than 7,000 independent application round trips. It still incurs PostgreSQL work: source and target scope rows each receive 1,000 increments, events take ordered sequence lookups, and all issue rows return through CAS. The saved tracer cannot distinguish server execution cost from transport or decoding cost.

Do not propose simply adding batching to this writer as though it were currently unbatched. Any further aggregation must establish equivalent final revisions/event ordering/error semantics under the existing rollback contract.

### Repeated history and snapshot work is another measurable candidate

prepareLifecycle computes LoadHistory for both source and target after hash confirmation: iteration_lifecycle_preview.go:478-487. freezeClosure then fetches the source event stream, appends the end event, and loads source history again before building a snapshot: iteration_closure.go:135-180.

Each live LoadHistory fetches the iteration, all participations/events/current issues, bulk historical labels/references, and reprojects persisted facts/statistics/chart: history.go:61-70, :102-128 and :182-194. The snapshot is marshalled and inserted once only after membership writes: iteration_closure.go:252-279.

Repeated source projection and allocation are demonstrated code paths; their duration or share of the original failure is unmeasured. The recorded samples rotate through new iterations, so do not claim that the same source iteration accumulates all 35 cycles' history.

### What the original tracer and wait sampler actually establish

The saved I1_P95_JSON reports 643,369 traced SQL statements and 157,298 lock-state samples, with zero observed Lock waits. This is consistent with the code, but it is not a per-closure attribution:

- iterationCapacityTracer increments for each QueryStart and adds Batch.Len at BatchStart; it records no query/stage duration: iteration_capacity_benchmark_test.go:119-132.
- The traced pool is attached before initial period creation/move and the whole 35-cycle sequence. The P95 test never resets its counter: iteration_i1_p95_test.go:29-35, :53-86 and :143.
- The total includes operations outside each endpoint timer: next-period creation, start preview+apply, and the 1,000 per-issue GetIssue calls building the next draft. The helper performs preview and apply: iteration_management_test.go:53-65. Fixture dbfx operations and the observer use the original pool rather than the newly instrumented handler pool.
- Closure timing begins just before ApplyIterationOperation and ends after its response decode: iteration_i1_p95_test.go:119-121. The setup and follow-up SQL assertions are outside that interval.
- The wait sampler polls only wait_event_type='Lock' for the matching application_name: iteration_capacity_benchmark_test.go:159-176. Zero samples do not exclude short missed locks, I/O, CPU scheduling, connection-pool acquisition, application allocation/JSON, or the observer's own resource impact.
- The initial three EXPLAIN plans are obtained before measured cycles and are only selected member/event/label SQL: iteration_i1_p95_test.go:76-84. They are not a profile of all production queries or lock/batch stages.

Go JSON output fragments long log lines. To parse I1_P95_JSON, concatenate Output strings for TestIterationI1P95 in order before extracting the newline-terminated JSON record. Parsing each Output fragment separately fails on this log.

### Environment comparison requiring coordinator verification

The frozen protocol references PostgreSQL 17.11 aarch64 on the ARM Mac's local VM: archived performance-protocol.md:6-9. This run records PostgreSQL 17.11 x86_64 while the Go test runs darwin/arm64. This establishes a different PostgreSQL execution architecture; it does not by itself prove QEMU/emulation. A read-only inspection of the actual database container image/platform is warranted before comparing historical ~1.1s to the current measurement. Neither environment differences nor an older pass can reclassify the present FAIL.

## Smallest discriminating diagnostic

One fresh, task-owned migrated database and one explicitly labeled diagnostic fixture can reproduce the same 1,000-task successful closure without rerunning the 35-cycle acceptance gate to obtain green.

1. Keep production SQL, lock order, population, frozen benchmark source/thresholds and original raw evidence unchanged.
2. Add instrumentation only through a task-private copied test or Go overlay. Set a context marker around exactly one real closure request, separate from fixture creation/start/preview.
3. Capture QueryStart/QueryEnd grouped by canonical query name, with counts, total elapsed and errors; capture BatchStart/BatchEnd per complete phase. Measure pool acquire, transaction commit and handler residual time separately. Do not sum per-query timings nested inside batch timings twice.
4. Report expected counts: 1,000 active-task reads, 2,000 source/target participation reads, seven membership batch phases/7,000 statements, plus identified constant/bulk history/ref queries.
5. If sequential reads dominate, a later isolated A/B transport diagnostic can pipeline the same ordered SQL at a safe complete-set boundary. Preserve source/target validation, all result checks and transaction/fresh-snapshot semantics. This is a diagnostic/fix follow-up design, not an implemented or approved product change.
6. If batches/history dominate, profile PostgreSQL statement work and CPU/allocation by stage before selecting any product edit. Record host/PG architecture and shared load; no arbitrary timeout, warmup or threshold changes.

Acceptance for diagnosis is an attributable stage-cost result or an honest unresolved hypothesis. A single diagnostic operation cannot establish a P95 acceptance pass.

## Follow-up fix and verification contract

If a product performance fix is separately brought into scope, preserve:

- Current authorization and whole-set bounds, sorted workspace/catalog/I1/iteration/issue/reference/participation locks, RC write and RR preview contracts, fresh snapshot after fence waits.
- Preview hash facts and original request identity, one atomic commit/durable result, stale/CAS error propagation, source leaves before target joins, exact rollover and scope revisions.
- Snapshot priority/labels/identifier facts and single final insertion, original sampled event time and clamped logical time, outbox/notifications after commit, no agent invocation or restart.

Relevant existing tests, which were located but not executed by this research agent:

- iteration/membership_batch_test.go:12, TestIterationMembershipBatchSQLMatchesCanonical.
- service/iteration_closure_test.go:22, :125, :443, :465, :497, :517, :561, :649, :712, :785 and :853: closure/rollback/frozen snapshot/time/multi-target/swapped sources/CAS failure/recovery/running task preservation/rollover facts.
- service/iteration_lifecycle_test.go:511: complete 1,000-member preview and limit.
- handler/iteration_closure_writers_test.go:133, :255, :325 and :435: actual writer/revoke/catalog/commit-response races.
- handler/iteration_i1_p95_test.go:24 and :161: original frozen P95 scenarios, run only for a justified later acceptance candidate on a new isolated database without concurrent agent builds/tests.

## Files found

- server/internal/handler/iteration_i1_p95_test.go — frozen population, timing boundaries, aggregate counters.
- server/internal/handler/iteration_capacity_benchmark_test.go — pgx query/batch counter and approximate Lock-only sampler.
- server/internal/handler/iteration_management.go — HTTP boundary, authorization and postcommit result.
- server/internal/service/iteration_lifecycle_apply.go — actual persistent closure owner.
- server/internal/service/iteration_lifecycle_preview.go — complete-set preparation and per-issue active task reads.
- server/internal/service/iteration_closure.go — membership preparation, freeze/reproject and final snapshot.
- server/internal/iteration/membership.go and membership_batch.go — individual participation reads and existing phased bulk writer.
- server/internal/iteration/history.go and history_capture.go — canonical historical projection and bulk display facts.
- server/pkg/db/queries/iteration*.sql, agent.sql and generated/batch.go — authoritative SQL, locks and batch transport.
- .trellis/tasks/archive/2026-10/10-05-iterations-i1-verification/performance-protocol.md — historical frozen metrics and measurement constraints, not current pass evidence.

## External references, versions and related specs

No external web references were used; this was internal code/evidence research. go.mod declares Go 1.26.6 and pgx/v5 5.9.2; the saved execution used Go 1.27.0 and PostgreSQL 17.11 x86_64. Actual batch behavior was checked in repository-generated code rather than inferred from a third-party tutorial.

Related specs: .trellis/spec/server/iterations.md (transaction ownership, complete-set capture, batch ordering, snapshot and outbox invariants); .trellis/spec/server/database-test-isolation.md (new explicit owned DB, TestMain behavior and aggregate connection budget).

## Caveats / Not Found

- No grouped runtime latency, CPU/allocation profile, original shared-host load record or exact pool-acquisition attribution exists in the read evidence.
- The original zero Lock samples do not establish absence of all contention.
- Current PostgreSQL image architecture/emulation and historical source parity were not inspected by this code-research agent; the parent coordinator owns environment inspection.
- No full-suite, benchmark or new diagnostic was run, and no product/root/spec/task metadata files were changed.
