# Canonical SQL and read transport diagnostic

**The diagnostic passed. QA-PERF-001 remains FAIL: original P95 2,171ms > 2,000ms.** No product SQL, index, authorization, lock order, threshold or dependency was changed. The frozen P95 gate was not rerun.

## Execution and ownership

- Candidate/source: 7af9ab9905c98c77bb091223ebe687c639475bb4, isolated Go checkout `/Volumes/artisan/code/2026/mc-functional-go-7af9ab990-20261102`.
- Fresh database: `multica_func7af9_next_sql_diag_1`, cloned from the recorded empty migrated template (751 ledger records). Actual template/database identities, zero template business rows/other connections and cluster headroom 60/100 were checked before execution. No shared service was restarted.
- `TestIterationCanonicalSQLPlanDiagnostic` ran once, PASS, exit 0. The new-file-only Go overlay retained the preexisting fixture patch and frozen test hash. Agent CLI guard, isolated HOME, no real Provider calls and no automatic retries.
- The original production HTTP preview/apply and transaction owner executed a complete 1,000-issue source-active → target-planned closure: ten projects, five priorities, one owner assignee and three of six labels per issue. Canonical workspace/member/catalog/I1/iteration/issue/reference/participation locks and seven ordered write batches remained in place.
- Before each selected production batch, its exact generated SQL and 1,000 original parameter sets were replayed through `EXPLAIN (ANALYZE, BUFFERS, WAL, SETTINGS, FORMAT JSON)` in a savepoint. Each savepoint was rolled back before the real batch. Source leave still precedes target join; CAS follows both scopes' event/participation/revision phases. Business fingerprints before/after each replay match.
- The actual closure asserted all 1,000 issues reached the target with rollover count exactly one. Its snapshot assertion checked 1,000 entries with three labels and a non-null priority; it did not compare every label ID/priority value. Do not broaden this to exact snapshot-value parity.

Raw evidence: `evidence/next-perf-sql-plan-1/`. `run.json` records commands, exits, process IDs and hashes; `schema-indexes-triggers.json` records actual catalog state; each `plans-*.json` retains every SQL text, argument and plan. `analysis-summary.json` and `verification.json` are coordinator-derived summaries with raw fingerprints. The source/test diff and overlay virtual-path absence were verified; the database ended with zero connections.

## Actual plans and index costs

The following are sums of PostgreSQL's **instrumented EXPLAIN Execution Time**, not original batch latency or a replacement endpoint measurement. Planning and client round trips are separate. The replay also changes tuple versions, WAL and cache even though business changes are rolled back.

| Canonical SQL | Statements | Execution sum ms | Planning sum ms | Observed access |
| --- | --- | --- | --- | --- |
| Source `LeaveDeletedIssueIterationParticipationBatch` | 1,000 | 651.403 | 32.674 | Participation index lookup; last-event bitmap scan + top-N sort for every update |
| Target `JoinIterationParticipationBatch` | 1,000 | 221.459 | 32.546 | Last-event bitmap scan + top-N sort for every insert; identity index arbitrates conflict |
| Issue `SetIssueCurrentIterationBatch` | 1,000 | 85.479 | 40.580 | `idx_issue_workspace_id_keyset`; revision/admission filter, one matched row |
| `ListActiveTasksByIssue` | 1,000 | 8.962 | 108.380 | `idx_agent_task_queue_issue_id_keyset`; no active rows in this fixture |
| `LockIssueIterationParticipation` | 2,000 | 13.893 | 29.137 | `iteration_participation_issue` + `LockRows`; 1,000 source rows and 1,000 absent targets |

### Source leave and target join

The actual event indexes are valid/ready/live unique B-trees `iteration_event_id(id)` and `iteration_event_sequence(iteration_id, sequence)`. The latter is the index used in both last-event subqueries. The participation indexes are `iteration_participation_identity(workspace_id, iteration_id, issue_id)` and `iteration_participation_issue(workspace_id, issue_id, iteration_id)`; both are valid, ready and live.

Each source subquery scans 3,003 event rows, filters workspace and sorts to return the highest sequence. Across 1,000 statements this is **3,003,000 event rows**; the sort-node total time is 628.430ms and includes its child scan time. Do not sum nested node times again. Each target subquery scans/sorts 1,001 events, totaling **1,001,000 event rows**; its inclusive sort-node sum is 205.299ms. These are concrete repeated work in this diagnostic and broadly consistent with the previous slow leave/join phases, without proving which cached plan ran in the original tail samples.

The plan did not choose a backward ordered scan of the existing sequence index. This alone does not establish that an extra index is required. All four inspected tables had `reltuples=-1`; event bitmap estimates of 43/57 rows versus actual 3,003/1,001 show an estimate mismatch. This EXPLAIN SQL is a distinct prepared text from the actual batched statement. Cached/generic plan behavior and original-run statistics were not captured. A stats/cache-controlled comparison of the same canonical SQL is the next discriminant before proposing DDL or changing last-event semantics.

### Issue CAS and write amplification

CAS locates exactly one row through `(workspace_id,id)` and retains the expected revision and admission filter. There is no broad issue scan in the observed plan. The actual issue table has **22 valid indexes**, including partial `(workspace_id,current_iteration_id)`; CAS changes that indexed value. Its replay produced 22,525 WAL records / **2,712,509 bytes**, versus 4,025 / 1,173,458 for source leave and 4,034 / 427,702 for target join. All inspected write plans report zero shared-read blocks; this is a cached diagnostic, not proof of no I/O elsewhere. EXPLAIN reported no invoked triggers for these three writes; the issue project's UPDATE trigger is not selected by this CAS.

The catalog and WAL establish real index/write work, but not each index's CPU/WAL contribution. Do not identify an unnecessary index or attribute all of the earlier 325.193ms CAS batch to index maintenance from these observations. RETURNING full-row transport/decoding, the unchanged cache plan and extra replay versions remain unseparated.

## Controlled read transport comparison

After the original 3,000 reads had acquired the complete lock set and before the first source-event batch, the diagnostic replayed the **same ordered SQL and arguments** four times: serial → pipeline → pipeline → serial. Each replay used a savepoint; all 3,000 result digests matched individually, the complete result contained 1,000 participation rows and zero active tasks, and all business fingerprints remained equal. Every row result, `Rows.Err` and batch-close error was consumed/checked.

| Mode | First ms | Second ms |
| --- | --- | --- |
| Serial | 641.410 | 746.562 |
| Pipeline | 29.665 | 36.465 |

The means are 693.986ms versus 33.065ms, a 660.921ms difference in this controlled replay. PostgreSQL's separate read EXPLAIN execution sums total only 22.855ms; its planning costs are also instrumented and cannot be subtracted from the transport spans as if they were the same execution.

This supports transport/client scheduling as a material direction for the existing serial-read loop, alongside the independent write-side event scan. It does **not** prove an isolated network RTT or a safe production rewrite: these locks were already held, active-task results were empty, targets absent, and no concurrent writer was introduced. Generic `Rows.Values` digesting also does not exercise the production typed scan/`ErrNoRows` control flow during replay. The original real closure still used that production flow. First-lock waiting, RC fresh snapshots, revocation, stale/CAS and error propagation require their own regression proof before any implementation.

## Next work and unchanged acceptance contract

1. If continuing diagnostics, inspect canonical prepared/cached last-event plans and row statistics on a new controlled fixture before/after explicitly recorded task-only ANALYZE, without forcing indexes or changing SQL. Distinguish the fresh-fixture estimate issue from a missing index. Preserve full lock order and rollback facts.
2. The smallest plausible product optimization scope, **if separately authorized**, is the existing complete-set active-task/participation preparation boundary and generated result handling. Keep every canonical read, source/target ordering, result/no-row/error check, fresh authorization/snapshot and rollback contract. No new abstraction or index is yet justified by this result.
3. Keep source/target repeated event retrieval as a separate measured opportunity. Do not simply remove it: clamped event time, sequence, revisions, swapped scopes and atomic error behavior are required.
4. Reuse existing membership SQL-parity, closure/rollback/CAS/snapshot/race tests and add meaningful first-acquisition/empty/nonempty/no-row/error coverage for any future implementation. Only a separately identified product candidate would justify one new original frozen P95 run. Preserve original 2,171ms FAIL regardless of later results.

No CPU/allocation profile was needed for this batch: the database and controlled transport results already discriminate actionable directions. The original 242MB process-wide allocation signal remains a signal, not a CPU root cause.

## Limits, verification and child availability

PostgreSQL reports 17.11 x86_64; the earlier environment inspection identifies the amd64 image on an aarch64 Docker engine. Go is 1.27.0 darwin/arm64. No shared-host load or original tail-sample execution plan was reconstructed. This diagnostic is not P95, an acceptance inventory increment, a product fix or proof that both opportunities can be added as independent endpoint savings.

The native implementation child completed compile and diagnostic tool work, then failed with 429 before producing its report. The initial review child also failed with 429 before a final review artifact. Exact errors are retained in `evidence/next-sql-session-1/agent-dispatch.json`. This report and `verification.json` are the coordinator's fallback analysis; independent review, if successfully completed later, is recorded separately and never implied by these files.

The existing DB-free `TestIterationMembershipBatchSQLMatchesCanonical` was freshly run with the Agent CLI guard and no DATABASE_URL: PASS, exit 0 (`evidence/next-sql-parity-1/`). It is a focused canonical-contract check, not an additional original Go inventory result. Full product lint/typecheck/suites were not repeated because only task-private diagnostics and task records changed. No product/dependency change, simplification/refactor, commit, remote dispatch or publication.
