# Actual prepared plans and task-only statistics comparison

**The diagnostic passed; QA-PERF-001 remains FAIL at 2,171ms > 2,000ms.** No frozen P95 rerun, product SQL, index, dependency, threshold or planner-switch change was made. This report is the implementation child's evidence analysis; coordinator verification and independent review must be recorded separately.

The new evidence now shows actual protocol-prepared execution plans, rather than an EXPLAIN-prefixed substitute. In this fresh fixture, unchanged cached/generic plans repeatedly bitmap-scan and sort the event stream. After explicit task-only ANALYZE, the same prepared identities use a backward scan of the existing `iteration_event_sequence` index and stop after one event. This is evidence for a statistics-refresh/replanning mechanism in the diagnostic, and does not establish the original P95 tail-sample cause or a need for another index.

## Identity and execution

- Candidate `7af9ab9905c98c77bb091223ebe687c639475bb4`, application version `0.6.8-rc.20261102`; isolated Go source `/Volumes/artisan/code/2026/mc-functional-go-7af9ab990-20261102`.
- New database `multica_func7af9_next_cached_diag_1`, cloned only after verifying the recorded template's zero users/workspaces/issues, zero other template connections, 751 migrations and shared capacity 45/100 used. No default database, other service or global PostgreSQL setting was changed.
- One `TestIterationCanonicalCachedPlanDiagnostic`, PASS, exit0; no test retry. Task HOME and the repository Agent CLI guard remained enabled. No real Provider or account action occurred.
- New-file-only overlay; existing daemon fixture patch, source product trees and frozen test SHA-256 remain identical. The virtual target was never written into the source checkout.
- PostgreSQL17.11 x86_64 and Go1.27.0 darwin/arm64. Existing engine architecture evidence remains applicable; this run did not restart the cluster or standardize shared load.

`run.json` retains commands, process IDs, exits, stream/test/overlay/runner hashes, database allocation/identity and connections-after0. The formatted diagnostic and runner are copied as text. `database-operations.jsonl` contains sanitized exact allocation/preflight/cleanup operations. No credentials are in the evidence.

## Controlled phases and capture method

The original production handlers, preview confirmation, RC transaction owner, authorization and complete lock sequence prepare the same 1,000-issue source-active → target-planned workload: ten projects, an owner assignee, five priorities and three of six labels. The original production code executes seven ordered 1,000-statement write batches and uses generated typed consumers. At the successful Commit boundary, a task wrapper explicitly rolls the entire transaction back. It asserts 1,000 target issues with rollover1 and 1,000 snapshot scope entries with three labels/non-null priority before rollback. There is no committed endpoint/durable-result acceptance claim from this diagnostic.

The same backend then replays the captured complete SQL/argument sequence twice, each in a new transaction ending in rollback:

1. **Original**: actual production typed consumer, no ANALYZE, existing cache warmed only by actual setup.
2. **Cache control**: all 3,058 original commands (3,051 standalone calls plus seven 1,000-statement batches) with the identical original SQL and arguments; no ANALYZE or cache reset. This distinguishes an additional cached replay from the later statistics experiment.
3. **After ANALYZE**: identical full sequence and arguments. Immediately before each selected source-leave/target-join batch, explicitly execute `ANALYZE public.iteration_event`, `ANALYZE public.iteration_participation`, `ANALYZE public.issue`, and `ANALYZE public.agent_task_queue`, eight statements in total. Both no-statistics source/target baselines finished before any task ANALYZE. The second ANALYZE set refreshes target-phase statistics after target events exist.

The dedicated backend loads `auto_explain`, sets notice-level JSON output with ANALYZE/BUFFERS/WAL/SETTINGS and timing, and enables logging only around the selected actual `SendBatch` calls. All 6,000 raw notices contain the exact canonical SQL and actual bound parameters; every parameter set was recomputed against the captured generated batch arguments. There is no `EXPLAIN ...` replacement text. No forced plan, enable-index switch, SQL rewrite, server restart or global log setting was used. The setup commands are saved in `diagnostic-metrics.json`.

`pg_prepared_statements` is captured on that same backend before/after each batch, including names, original SQL, parameter types, `from_sql=false`, preparation time and generic/custom counters. This view is session-local, and its counters count plan selections, not physical plan identities. See [PostgreSQL17 prepared-statement view](https://www.postgresql.org/docs/17/view-pg-prepared-statements.html). Session `auto_explain` execution instrumentation can itself add overhead; see the [official module documentation](https://www.postgresql.org/docs/17/auto-explain.html).

## Actual observed access paths

All rows below contain exactly1,000 actual selected executions. Instrumented notice-duration sums and client batch spans include different instrumentation/transport work; neither is an uninstrumented endpoint latency or P95.

| Phase | Source leave notice duration ms | Target join notice duration ms | Source event rows visited | Target event rows visited | Event access |
| --- | ---: | ---: | ---: | ---: | --- |
| Original production typed flow | 670.134 | 249.110 | 3,003,000 | 1,001,000 | Bitmap Heap/Index Scan + Sort each time |
| Same-statistics cache control | 694.490 | 248.412 | 3,003,000 | 1,001,000 | Same bitmap scans + Sort |
| After explicit ANALYZE | 23.320 | 12.105 | 1,000 | 1,000 | Existing sequence index, Backward, no Sort |

Client-observed selected batch spans were726.066/309.086ms (original source/target),746.577/305.980ms (cache control), and78.636/60.362ms (after ANALYZE). These are diagnostic spans with auto_explain notices and generic replay consumption; do not subtract them from the earlier2,396.832ms closure or the original2,171msP95 to predict additive gains.

The two relevant prepared identities retain the same names and preparation times across all three phases:

- Source leave: `stmtcache_fd8c64203557ce94f2f5c79ba22b6be31c05cdb9f50faaf9`; first selected batch chooses5custom/995generic, cache control and after ANALYZE each choose0custom/1,000generic.
- Target join: `stmtcache_84dba38cba0b1c2162d0567112bfd1c1d6b70152a433555e`; setup already chose5custom/995generic, and all three selected batches choose0custom/1,000generic.
- All selected plans and state snapshots identify backendPID2643; `plan_cache_mode=auto` throughout. No new prepared identity was substituted to obtain the post-ANALYZE path.

The first two passes have zero `pg_stats` rows for the four tables, all `reltuples=-1`, and no prior ANALYZE/autoanalyze timestamps. Source has3,003 matching events (3,004 total table rows) and target1,001 (4,004 total). The no-statistics source generic plan estimates45 event-index matches; target estimates12. Both heap estimates are1 despite the larger actual populations. After ANALYZE,53 column-statistic rows are present; reltuples become issue1,000/task0/participation1,000 and events3,004 at source then4,004 at target. Event index estimates become1,502/2,002, while the actual backward scan stops after1. The index definitions and valid/ready/live state remain unchanged.

[PostgreSQL17 PREPARE](https://www.postgresql.org/docs/17/sql-prepare.html) documents custom/generic selection and replanning when planner statistics update. The observed same-name generic plans changing shape after ANALYZE are consistent with that mechanism. The experiment does not independently separate refreshed statistics from the invalidation/replanning that ANALYZE triggers: a no-statistics DEALLOCATE/reprepare control was not run. Cache warming alone did not change the path in the saved control; missing-index necessity is unsupported.

## Rollback and physical/statistics effects

Strict fingerprints of issues, participation, events, iterations, snapshots and operations match before/after **all three full transaction rollbacks**. No business change from the measured closure remains. Notification rollback follows the same transaction but is not included in that fingerprint. Snapshot validation covers cardinality/label-count/non-null priority, not exact label IDs/priority values.

Rollback is not a physical reset. Raw state shows heap growth/dead tuples, and after the ANALYZE transaction rolls back the column statistics disappear (`pg_stats` rows0), while `pg_class` retains reltuples/relpages and cumulative ANALYZE tracking reports two analyzes per table. The retained event reltuples4,004 are an estimate from the rolled-back measured phase, while the actual retained fixture count is2,003. Cache/WAL and physical tuple effects remain. This is precisely why a savepoint-only source ANALYZE cannot be assumed to restore an uncontaminated later target baseline.

The test does not perform VACUUM or attempt to undo these physical effects; the new task database is retained for diagnosis with zero residual connections.

## What may change next, and what is still unproved

1. The demonstrated repeated-scan mechanism can be removed in this diagnostic by statistics refresh/replanning while preserving canonical SQL and the existing sequence index. Before any product/index proposal, inspect whether the real deployment/tail samples have current statistics and the same actual cached path. Do not automatically add an index or arrange product ANALYZE.
2. The prior3,000-read ABBA evidence remains a separate direction. The smallest plausible product boundary, only if separately authorized, is complete-set active-task/participation preparation and result consumption. Preserve each canonical read, source/target ordering, first-lock waits, fresh RC snapshots, current authorization, typed Scan/ErrNoRows/error/Close behavior, CAS failure and whole-operation rollback. This run does not implement that optimization.
3. Source/target last-event retrieval must retain clamped event time, sampled time, sequence and revision behavior. No removal or precomputation is justified solely by the measured plan.
4. Future regression must include actual first acquisition, blocked-then-fresh statement visibility, revocation in both commit orders, nonempty task/participation sets and missing rows, typed decode/Rows.Err/batch-close failures, stale CAS and partial-write rollback, swapped scopes, exact snapshot/time facts and unchanged running tasks. Existing product tests and a separately reviewed implementation are required; a generic raw replay is insufficient.
5. Only a newly identified, separately scoped product candidate justifies a new execution of the original frozen P95 protocol. Preserve original2171msFAIL permanently; a diagnostic pass does not close it.

The original typed production flow was exercised once. Cache-control/ANALYZE replays consume generic Rows.Values and all errors/Close results but do not rerun service decisions or typed error branches. In particular replayed clock reads produce later values, while writes deliberately reuse captured original business-time arguments. No concurrency, first-lock behavior or future optimization equivalence is claimed. Original tail-sample plans, backend selection, cache/statistics history and shared-host load remain unknown.

## Verification and files

Raw evidence: `evidence/next-perf-cached-plan-1/actual-plan-notices.json`, `actual-batch-phases.json`, `canonical-command-sequence.json`, state/rollback snapshots, six parsed plan populations, `analysis-summary.json`, and `verification.json`. All6,000 SQL texts/parameter vectors, exact seven generated batches, counters, index paths, arithmetic and saved command stream hashes reconcile. The new diagnostic compiled and executed successfully; source/frozen-test hashes and additive-overlay absence were checked; DBconnections0.

Only new ignored private diagnostic files, this research report and new task evidence were written. No product/refactor simplification, dependency, commit, remote publication or commercial call. Full product lint/typecheck/E2E/DEB/macOS suites were not repeated. Relevant checks were gofmt, compile, the actual diagnostic, Python/JSON/hash/SQL-parameter checks and unchanged-source verification. Broader coordinator cleanup and independent evidence review remain separate responsibilities.
