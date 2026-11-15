# Iteration closure: semantics and reuse review

Scope: read-only support for the core acceptance owner. Candidate `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`; all code anchors below refer to `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104`. The source manifest records exact bytes. No database, test, build, browser, Provider or product edit was performed. A delegated researcher encountered an external429 before producing an artifact; the coordinating session completed this review directly.

## Decision

**A transport-only active-task read batch is the smallest plausible first product candidate, conditional on the planned current-candidate phase measurement.** It touches an unconditional repeated read after the complete authority/row/reference boundary. Participation preparation is a separate, more complex candidate; do not combine both into one unmeasured optimization. Neither this review nor old diagnostic passes establish a P95 improvement. The existing2155.025ms versus2000ms failure remains.

Reuse existing transaction ownership, canonical sqlc rows, batch transport and SQL-equivalence conventions. Do not remove reads, add an index, run ANALYZE in product requests, replace snapshots from live data or change retry policy merely to reach the threshold.

## Authority and snapshot timeline

| Boundary | Current source contract | Consequence for a read optimization |
|---|---|---|
| Human transport | `handler/iteration_management.go:23` resolves a human member, rejects machine-maintenance credentials; authorization callback resolves again on the borrowed transaction. `:41` checks the complete issue set belongs to that workspace. | Keep current authorization in the transaction; cached/preflight membership or machine issue rights cannot substitute. Service tests with no-op AuthorizeIssues are not authorization proof. |
| Apply transaction | `iteration/operation_execute.go:26`: workspace KEY SHARE → subscriber fence → active member SHARE → current authorize → shared status catalog → I1 workspace fence → durable operation lookup → mutation → stored result. | Do not pipeline any protected business read ahead of successful authorization/I1 acquisition. A replay still reauthorizes before using the durable result. |
| Preview transaction | `service/iteration_lifecycle_preview.go:91` uses RR READ WRITE with bounded retries; Apply uses RC in `iteration/transaction.go:23`. | Shared preparation code must retain caller isolation. Separate SQL statements after an RC wait can see later commits; one CTE/SELECT cannot be presumed equivalent. Preview remains a coherent RR view. |
| Complete-set locks | `iteration_lifecycle_preview.go:172` includes actual source IDs, sorts iteration UUIDs, locks iterations, then `:223` locks complete issue rows ordered by ID. `:239` authorizes the full set before `CaptureIssueReferences` at `:263`. | Do not use client move order, preflight source IDs or partially authorized rows. No extra connections/goroutines for borrowed-tx work. |
| References | `iteration/history_capture.go:40` validates unique same-workspace rows, locks references when writing, reads labels/reference facts and preserves unavailable historical names separately. | The smallest read batch starts after these existing gates, not before them. Keep SHARE NOWAIT, typed reference errors and current grant observations. |
| Task reads | `iteration_lifecycle_preview.go:285` reads active task rows for each ordered issue after coordinator references. `queries/agent.sql:2510` includes queued/dispatched/running/waiting_local_directory, ordered by created_at DESC. Only running rows increment the preview count. | Preserve canonical SQL and row mapping. Do not replace the query with cached counts, all statuses, or COUNT(*) without a separately reviewed contract. |
| Business clock/hash | `iteration_lifecycle_preview.go:441` samples after preparation; `:458` copies issues and zeros RunningExecutionCount only for the confirmation hash. `:476` compares the expected hash before history projection. | Running counts still belong to the visible preview. Their deliberate exclusion from the hash allows execution completion without falsifying task facts; it is not permission to omit the read or force execution state. Preserve stale-hash precedence. |

`handler/issue.go:3732` already batches two separate canonical lock statements after fences, with typed result mapping and checked Close. `handler/iteration_pipeline_diagnostic_test.go:19` is a real post-wait snapshot protocol proof, while `:139` tests ordinary issue-writer lock order. These are reuse patterns; they do not by themselves prove lifecycle complete-set scheduling.

## Candidate A: active-task transport only

**Proposed boundary, not implementation:** replace the repeated transport at `iteration_lifecycle_preview.go:290` with one ordered batch of the exact `ListActiveTasksByIssue` query, after all existing issue/reference authorization. Keep one result group per input issue. Leave membership preparation, status resolution, history, writes and clocks intact.

The repository currently has sqlc batch generation in `generated/batch.go`, but no active-task batch query. Prefer a narrowly added `:batchmany` equivalent of the canonical query with generated scanners and an equivalence test, rather than a generic DBTX interception/cache or handwritten untyped `SELECT *` decoder. Generate via the existing sqlc configuration; never hand-edit generated Go. A helper, if needed, belongs beside the borrowed iteration preparation and should return results aligned with the supplied ordered issues, not invent a new cache/service/interface layer.

Required behavior:

- Queue only after the full ordered lock/authority boundary, using the same `pgx.Tx`; no pool acquisition inside the borrower.
- Preserve exact statement order and task population, including empty groups, mixed task statuses and current issue identity. Do not deduplicate issue input silently; the upstream complete-set owner owns uniqueness/order.
- Consume and close each Rows result, check Scan and Rows.Err, then check BatchResults.Close. Preserve the first substantive query/decode error and propagate Close-only failure. No partial preview is published and no business mutation follows failure.
- Do not issue another query on that connection while batch results remain open. Do not let a helper begin/commit/retry the transaction.
- Explicitly test that unrelated refetch/error handling cannot turn a missing result into an empty task group. The ordinary no-row many-query result is empty; protocol/decode failure is not.
- Because this replaces per-query round trips, call scheduling changes. A passing local row-population comparison alone is insufficient: exercise the actual lifecycle boundary with real post-lock concurrent completion and whole-operation rollback.

Conditional next files: a narrow canonical batch SQL entry, regenerated sqlc files, `service/iteration_lifecycle_preview.go`, a small domain-local consumer only if needed, and meaningful service/handler regressions. No public API, migration, UI, execution scheduler, global settings or dependency is required.

## Candidate B: participation reads need a separate design

`iteration/membership.go:29–78` is not a flat list of independent reads:

1. Validate admission, locked source/workspace and allowed planned/active iterations; resolve built-in/custom status facts.
2. An unchanged source/target returns before participation reads. Terminal target eligibility can fail before any participation lock.
3. A source participation row is mandatory and must be currently joined. Missing source is an error, not an empty default; `HasStartedCurrentParticipation` supplies the source's sticky started fact.
4. A target participation row may be missing (`pgx.ErrNoRows` is permitted there only). An existing current target is invalid. An active target additionally reads HasIterationActiveJoin, including when its participation row was absent; reentry is existing in_original OR that event witness.

A fully queued source+target batch executes target locks even when the source's local validation would have failed. Across issues, grouping all source locks then all target locks also differs from the current per-issue order. This can change waiting, contention and which error surfaces. It is not automatically an authority leak under rollback, but it is not a proved equivalent optimization. Preserve error precedence/lock scheduling or explicitly specify and validate an acceptable change before adoption.

Custom-status reuse is also separate: `record.go:51` resolves custom entries through the current transaction, while historical capture resolves status/category after reference/catalog gates. Built-in todo has no extra category query in the failing workload. Caching/reusing historical categories would not explain this workload and expands the contract unnecessarily.

## Existing batch writes and event-last reads

`iteration/membership_batch.go:18` already uses seven ordered phases: append source events, leave source participation, advance source revisions, append target events, join target participation, advance target revisions, issue CAS. All source leaves precede target joins, including swapped-source cases. This is already transport batching, not7000 independent client round trips.

`queries/iteration_membership_batch.sql:4/17/31` repeats latest-event lookup with sequence-descending limit1. Event append assigns next sequence and clamps occurred_at; leave and join derive their timestamps from the transaction's latest event. Replacing these with a cached maximum, unordered bulk INSERT/UPDATE, fewer revision increments or a precomputed timestamp changes the write/history contract. Existing `TestIterationMembershipBatchSQLMatchesCanonical` compares normalized canonical SQL; the intentional leave-query RETURNING ID exists for affected-row checking.

Generated callbacks in `generated/batch.go` do not eliminate caller error ownership: `membership_batch.go:96–146` records first errors, explicitly checks Close, and maps issue-CAS ErrNoRows to iteration_preview_stale. Missing leave/source row remains a failure. Never turn no-row into success, drop Close errors, or commit successful earlier phases after a later failure.

The existing event-sequence index already supports the desired lookup direction. The old7af9 diagnostic showed statistics/replanning affecting canonical access in its own fixture; it does not prove current1a32 tail plans or authorize an index/global planner change. Measure actual canonical batch phases first. If dominant, capture current backend's actual prepared statements/plans and inputs, not EXPLAIN-prefixed substitutes.

## Snapshot, retry and unknown outcomes

- `iteration_lifecycle_apply.go:54` freezes closure before membership release. `iteration_closure.go:125` reads existing events, clamps logical time, appends the close marker, loads history, stages immutable snapshot, then closes the active row so atomic handoff can obtain the active slot.
- `iteration_closure.go:254` persists that staged snapshot only after membership/handoff writes, with final processed time. The earlier prepare history and freeze history observe different intended states; removing one as a duplicate is not justified by matching function names.
- `iteration/transaction.go:23` retries whole RC attempts at most three times for its named database conflicts/request collision. Each attempt reauthorizes and reacquires locks. Operation identity is allocated outside this loop. All iteration/entity/event/snapshot/outbox/operation writes share the owner commit.
- An ambiguous commit is returned for original-request reconciliation. A batch helper must not resubmit writes, add its own retry loop, reuse prepared facts across attempts or perform external effects before commit.

## Required validation before and after a product change

| Proof | Existing reusable entry | New boundary-specific requirement |
|---|---|---|
| Authority and lock order | `TestIterationClosureRealMemberRevocationRaces`; `TestUpdateIssueIterationReadPipelineLockOrder`; `TestIterationPipelinePostFenceSnapshot` | Run lifecycle end/disable at actual first acquisition, revoke-first versus closure-first, no protected read/write before authorization; preserve RC observation after a real wait. |
| Typed task semantics | `TestIterationClosureManagementPreservesRunningTask`; `TestIterationClosureDaemonCompletionBarrierDoesNotRestartExecution` | Empty/nonempty/mixed task groups and true per-issue order; exact typed values; concurrent completion yields current facts without starting/stopping tasks. |
| Participation matrix (if B chosen) | `PrepareMembershipChange` current branches; historical read-semantics probe is reference only | Missing/current/left source and target; absent target allowed; active reentry; terminal confirmation; source==target; custom status; error precedence under a real wait. |
| Transport failures | Existing generated scanners / batch error paths | Inject query, malformed decode, delayed Rows.Err and Close-only failure into the implemented consumer; preserve first error and complete rollback. |
| Atomicity and immutable history | `TestIterationClosureBatchCASFailureRollsBackAllPhases`; `TestIterationClosureFinalSnapshotFailureRollsBackHandoff`; `TestIterationClosureBatchMultiTargetHandoff`; `TestIterationLifecycleBatchSwappedSources` | No partial membership/event/counter/snapshot/outbox/operation changes for actual new-path failure. |
| Identity and time | `TestIterationClosureRollbackReplayAndFrozenDigest`; `TestIterationClosurePreservesSampledClockAndFinalProcessingTime`; `TestIterationClosureRolloverEventsMatchCommittedCounters` | Same request replay, stable operation ID, one sample after all required locks, preserved sequence/clamping and final processed time. |
| Performance | Current planned one-closure attribution probe then original frozen P95 gate on a new product candidate/database | First establish which phase costs matter; instrumented diagnosis is not P95. Retain2000ms threshold,5warmup+30samples and originalfailure; no repeated unchanged full gate to select a passing sample. |

Do not rerun old protocol probes and call that new consumer verification. Existing historical rows/scans and isolated protocol proofs make a candidate plausible, but malformed decode/delayed Rows.Err/real implemented scheduling still need dedicated assertions.

## Delivery and limits

This review supplies a conditional optimization boundary and semantic regression checklist to the existing performance owner. It intentionally does not change the frozen product, authorize a new database/window, alter the441case ledger or claim resolved performance. Current P95 and Linux failures remain with the core owner. No cleanup is required from this slice because it opened no runtime resource.
