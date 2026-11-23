# No-statistics reprepare control

**The diagnostic passed. QA-PERF-001 remains FAIL: 2,171ms > 2,000ms; the frozen P95 protocol was not rerun.** This is coordinator execution/verification after two native followup notifications arrived empty; it is not an independent review. The completed native run1 and read-semantics evidence remain unchanged.

## Identity and hypothesis

Candidate7af9ab9905c98c77bb091223ebe687c639475bb4, version0.6.8-rc.20261102. New run `evidence/next-perf-cached-plan-2/`, new explicit database `multica_func7af9_next_cached_diag_2`, same isolated Go checkout and preserved daemon fixture patch. Pre-execution hypothesis and boundary are in `research/cached-reprepare-control-plan-1.md` and the private `pre-execution-plan.json`.

Run1 left a specific gap: ANALYZE refreshes statistics and invalidates cached plans together. A fresh no-statistics prepare determines whether stale prepared identity alone suffices to explain the repeated scan in this fixture. The new control was recorded before execution, not selected from performance retries. No original P95, completed E2E/DEB/Mac gate, product SQL/index/dependency or forced planner mode was executed or changed.

The same reviewed overlay adds one virtual test only. One `TestIterationCanonicalCachedReprepareDiagnostic` compiled/executed, PASS, exit0. Fresh empty template identity, zero template user/workspace/issue rows/connections,751 migrations and shared headroom were checked before clone. Task HOME, Agent CLI guard and count1 remain enabled. Source/frozen-test/overlay-target integrity and DBconnections0 are recorded in run.json.

## Actual capture and control

The original typed production mutation flow runs once on one dedicated backend and rolls back at its Commit boundary. All3,058 captured commands, exact canonical SQL/arguments, seven ordered1,000-statement write batches and production locks are then replayed in three complete transactions ending in rollback: cache control; no-statistics reprepare control; after explicit task-only ANALYZE at source and target phases. The latter replays consume generic values, not service decisions. Original sampled business-time arguments remain fixed even when replayed clock reads return later values.

Between cache-control rollback and reprepare control, `pgx.Conn.DeallocateAll` resets both client caches and server prepared statements on this dedicated backend, outside any business transaction. Its actual implementation was checked in local pgx/v5@v5.9.2 conn.go:397. Raw `prepared-reset.json` proves identical backendPID7356, no selected prepared statements after reset, identical no-statistics tables/settings/counts, and zero pg_stats rows. It does not reset physical buffers, tuples or WAL. Prepared names are deterministic SQL hashes, so new preparation is established by new prepare_time and reset custom/generic counters, not a name change.

All8,000 actual canonical SendBatch plans and original bound parameter vectors were captured through backend-only auto_explain notices. Each batch contains1,000 sourceleave or targetjoin executions; SQL, arguments, notice ranges, prepared counters and node arithmetic were checked. No EXPLAIN-prefixed substitute, index/SQL rewrite, global configuration, shared restart or forced custom/generic mode was used. Plan_cache_mode remains auto.

| Phase | Source notice duration sum ms | Target notice duration sum ms | Source event rows | Target event rows | Access |
| --- | ---: | ---: | ---: | ---: | --- |
| Original typed production flow | 710.095 | 234.867 | 3,003,000 | 1,001,000 | Bitmap scan plus Sort |
| Cache replay, no ANALYZE | 690.996 | 244.794 | 3,003,000 | 1,001,000 | Bitmap scan plus Sort |
| Fresh prepare, still no ANALYZE | 667.284 | 236.153 | 3,003,000 | 1,001,000 | Bitmap scan plus Sort |
| Explicit task-only ANALYZE | 16.908 | 11.184 | 1,000 | 1,000 | Existing sequence index Backward; no Sort |

Fresh preparation selects5custom/995generic for each query. After ANALYZE the same newly prepared identities each select1,000generic plans with the ordered index path. Generic selection itself therefore does not imply the poor access path. All three no-statistics phases have reltuples=-1 and zero column-statistic rows; the analyzed phases have recorded table/column statistics and unchanged valid/ready/live indexes.

These are instrumented plan-duration sums; auto_explain node timing and notice delivery add overhead. They are not endpoint timing, P95 or a prediction that these savings add to the previous transport delta. Cache replay/reprepare changes physical versions and warmth despite rollback. No original tail-sample plan or load is reconstructed.

## Root-cause evidence, minimum future scope and regression contract

In this controlled fixture, neither cache reuse/warming nor a safe fresh prepare removes the repeated event scan. Updating planner statistics plus replanning does remove it using the existing index. This narrows the diagnosed mechanism to the no-statistics planner inputs/replanning outcome; it does not establish a missing index, a generic-plan defect, a single responsible statistic or the original2171ms tail cause. ANALYZE touched four tables, so their marginal contributions were not independently estimated.

The smallest next diagnostic for a real deployment is read-only capture of actual plan/statistics/backend identity when the symptom occurs; freshness and actual access path should precede any DDL or product optimization proposal. Do not embed ANALYZE in product requests or change shared maintenance policy based on this task.

The separate prior ABBA and new typed/protocol/current-product regressions support a possible complete-set read-preparation transport boundary. If product work is later explicitly authorized, retain all canonical separate statements, sorted fences and source/target order, RC reads after waits, current membership authority, complete typed task/participation rows, ErrNoRows/error/Close handling, active-target reentry and stale CAS/whole-operation rollback. No SQL/index/dependency change is currently justified. `research/read-semantics-diagnosis-1.md` maps executed regressions and remaining typed-decode/delayed-Rows.Err/implemented-scheduling gaps.

Four complete rollback fingerprints match for issues/participation/events/iterations/snapshots/operations. Notification rows are outside this fingerprint; no durable commit acceptance is claimed. As in run1, rollback removes pg_stats rows but may retain changed reltuples/relpages and cumulative ANALYZE tracking, while physical WAL/cache/dead versions persist. No VACUUM/reset was used; the new DB is retained with zero connections.

Verification files: run.json, analysis-summary.json, verification.json, actual-plan-notices.json, actual-batch-phases.json, canonical-command-sequence.json, prepared-reset.json and four rollback files. Coordinator combined terminal/hash/typed checks are in `evidence/next-cached-session-1/coordinator-verification.json`. Independent review is recorded separately. Full product lint/typecheck/suites were not rerun: only additive task-private diagnostics and task records changed, with actual Go compilation/execution and syntax/format/evidence checks.
