# Iteration closure phase attribution — 2026-11-02

**The diagnostic completed successfully. QA-PERF-001 remains FAIL; the frozen P95 gate was not rerun or changed.**

One real HTTP closure processed 1,000 todo issues from an active source iteration into a planned target. It used the candidate's production handlers, transaction owner, SQL, complete preview confirmation, membership writer and snapshot builder. The population retained the original workload shape: ten projects, an owner assignee, five priorities, and three of six labels per issue. Exact rollover and all 1,000 frozen priority/label facts were asserted after the write.

## Evidence and isolation

- Candidate: 7af9ab9905c98c77bb091223ebe687c639475bb4.
- Source: /Volumes/artisan/code/2026/mc-functional-go-7af9ab990-20261102.
- New database: multica_func7af9_next_perf_diag_1, cloned only after verifying the recorded template had zero users/workspaces/issues, zero other template connections, and at least 25 connections of shared cluster headroom. Its migration ledger had 751 records.
- Test: TestIterationClosurePhaseDiagnostic, exactly one execution, PASS, exit 0. The Agent CLI guard remained enabled, with a separate task HOME and no real Provider calls.
- The Go overlay adds one virtual test file only. It does not replace any existing source or frozen test. The actual source diff, preexisting daemon fixture patch and frozen test SHA-256 all remained unchanged; the virtual test path never existed on disk in the checkout.
- Commands, timestamps, exit codes and hashes: evidence/next-perf-probe-1/run.json. Raw Go events: diagnostic.stdout. Metrics: phase-metrics.json. The formatted test and host driver are preserved as text with the evidence.

The private test compiled first using go test -c -overlay with package compilation concurrency two. Execution used the compiled binary through the repository Agent CLI guard and go tool test2json, with one test worker, one execution and a five-minute diagnostic bound. It did not enable the P95 opt-in or execute the original 35-cycle test.

## Measured result

The first closure in the fresh diagnostic database took **2,396.832ms**. This is one operation, not a P95 or an acceptance pass.

| Non-overlapping client-observed category | Count | Total ms | Share of handler time |
| --- | --- | --- | --- |
| Standalone Query/QueryRow/Exec spans | 3,053 statements | 986.856 | 41.17% |
| Complete membership batch spans | 7 phases / 7,000 statements | 1,279.292 | 53.37% |
| Traced connection acquisition | 1 acquisition | 0.002 | <0.01% |
| Unattributed handler residual | Request/response, projection, allocation and instrumentation outside these spans | 130.683 | 5.45% |

Query and batch spans are pgx client timings. They include transport, decoding and client result processing; they do not isolate pure PostgreSQL execution. Batch result events contribute only counts, never another duration, so they are not added twice.

### Standalone query groups

| Query | Calls | Total ms |
| --- | --- | --- |
| LockIssueIterationParticipation | 2,000 | 504.426 |
| ListActiveTasksByIssue | 1,000 | 226.987 |
| InsertIterationSnapshot | 1 | 106.105 |
| ListAllIterationEvents | 4 | 44.630 |
| ListIterationHistoryLabels | 2 | 20.671 |
| ListIterationHistoryIssueReferences | 3 | 20.327 |
| ListIterationParticipations | 3 | 13.124 |
| Commit | 1 | 4.042 |

The 3,000 serial active-task/participation queries account for **731.413ms (30.52%)** of this handler operation. Their expected counts are now measured, rather than inferred only from code.

### Ordered membership pipeline

| Phase | Statements | Complete batch ms |
| --- | --- | --- |
| Source AppendIterationLifecycleEventBatch | 1,000 | 26.377 |
| Source LeaveDeletedIssueIterationParticipationBatch | 1,000 | 623.604 |
| Source AdvanceIterationScopeRevisionBatch | 1,000 | 33.127 |
| Target AppendIterationLifecycleEventBatch | 1,000 | 29.311 |
| Target JoinIterationParticipationBatch | 1,000 | 197.020 |
| Target AdvanceIterationScopeRevisionBatch | 1,000 | 44.661 |
| SetIssueCurrentIterationBatch | 1,000 | 325.193 |

The source leave, issue CAS and target join batches dominate the measured pipeline. The writer is already pipelined; these results do not support describing it as 7,000 independent application round trips.

The pool reported zero empty-acquire waits; all batch/acquire error counters were zero. Process-wide counters around the closure increased by 242,007,280 allocated bytes and 15 GCs. Those counters are a useful allocation signal, not a per-goroutine CPU/allocation profile. The residual must not be labeled pure CPU time.

## What this establishes

1. Closure cost is concentrated in actual database-facing statement work and its client processing. Pool saturation is not a material contributor to this diagnostic.
2. The sequential read mechanism described in research/perf-code-diagnosis-1.md is real and contributes a measured 731ms here.
3. Existing batch writes contribute an even larger 1,279ms. The slowest single phase is the source participation leave at 624ms; optimizing only the read loop would leave substantial write cost.
4. No query, authorization rule, lock order, revision/event ordering, snapshot fact, threshold or retry was changed to obtain this evidence.

This does **not** identify the exact cause of the original two slow measured samples. The original gate used a repeated carryover workload and a concurrent one-millisecond Lock-state observer. This diagnostic uses its first closure, does not flush OS/PG caches, adds query/batch/acquire tracing, and deliberately does not start that observer. Shared host/PG load is not reconstructed from the old run. A one-operation result cannot establish a tail distribution, the observer's causal impact, or a fixed product.

Runtime environment: Go 1.27.0 darwin/arm64, ten reported CPUs; PostgreSQL 17.11 x86_64. A separate coordinator inspection owns actual PostgreSQL container/platform evidence. Architecture difference alone is not an emulation diagnosis.

## Concrete next work

Before any separately scoped product optimization, use the measured order above:

1. Inspect plans, indexes and PostgreSQL execution cost for the source leave, issue CAS and target join statements under the same 1,000-row transaction. A task-only diagnostic can EXPLAIN the canonical statements inside a rollback on disposable fixture rows; do not substitute faster different SQL and call that a fix.
2. Measure a transport-only comparison for the 2,000 participation locks and 1,000 active-task reads at a safe complete-set boundary. Preserve all canonical SQL, lock/result/error checks and fresh-snapshot/authorization semantics.
3. If database spans are reduced, profile history/snapshot allocations separately before reusing intermediate projections. The 242MB allocation signal alone does not authorize removing duplicate-looking reads.
4. A later product candidate must retain the existing closure/rollback/CAS/snapshot/race coverage and rerun the original frozen P95 test once in a recorded new exclusive database. It must keep the current failed run and cannot use this passing diagnostic as its acceptance result.

No product fix is part of this test campaign.

## Cleanup and verification

The diagnostic database has **zero residual connections**. The binary/driver process audit found no owned probe process remaining. The database, private HOME, binary, overlay and raw evidence are retained for reproducibility.

Verification parsed terminal Go events, checked every command stdout/stderr hash, matched the metric/test/driver hashes, checked exact 1,000/2,000/7-phase/7,000-statement counts, and confirmed source/frozen-test integrity. See evidence/next-perf-probe-1/verification.json.

Only task-private diagnostic code and new task evidence/research were added. There was no product or dependency change, no product simplification/refactor, and no commit/publication. Product lint/typecheck/full suites were not repeated; the actual Go diagnostic compilation and execution were the relevant checks.
