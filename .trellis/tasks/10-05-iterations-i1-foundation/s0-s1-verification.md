# S0/S1 implementation evidence

## S0 baseline (2026-10-06)

- Checkout: `/Volumes/artisan/code/2026/multica-0.4.37`, `codex/projects-p1`, HEAD `07e8bcbc1`; initially clean. No main/detached checkout changes.
- W01–W17 re-inspected in `../10-05-iterations-i1/research/writer-inventory.md`. W01 preserves default `Begin` isolation and four-attempt 55P03 budget; P1 project writes preserve RR and deletion RC. All remain unintegrated at baseline.
- `make status` / `make list` found stopped verification environments and an unrelated human-owned T1 environment. None is used for these checks.
- Created local task-only PostgreSQL database `multica_i1_s1_20261006`; ran `go -C server run ./cmd/migrate up` to 566 (exit 0). Log `/tmp/multica-i1-s1-migrate.log`. This is not a new migration recovery certification.
- Task pointer activated with `TRELLIS_CONTEXT_ID=codex-i1-20261006`; parent context manifests validated.

## Ordinary write performance, thresholds fixed before implementation

Apple M4 arm64, 16 GiB host RAM; local container PostgreSQL 17.11 aarch64 through localhost TCP. Warm, small fixture workspace, no iteration settings/participants, one distinct issue per writer; actual HTTP UpdateIssue title writes, no project/attachment or agent execution. This is a regression probe, not populated workspace or production capacity certification.

Command (DATABASE_URL points exclusively at the database above):

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/handler \
  -run '^$' -bench '^BenchmarkIterationOrdinaryWrite$' -benchtime=300x -count=3
```

Stable benchmark source: `server/internal/handler/iteration_write_benchmark_test.go`. Each run performs 300 successful writes. Instrumentation counts statements inside the owning transaction, excluding Begin/Commit and pre/post-transaction reads. Lock statement time includes round trip and execution, not exclusively server-side waiting. Query/row instrumentation does not separately time streamed Query rows. No skip counts as a pass.

| Writers | Baseline writes/s (three runs) | HTTP P95 ms | Tx queries/write | Lock-statement ms/write |
| --- | --- | --- | --- | --- |
| 1 | 498.4 / 544.4 / 535.5 | 3.296 / 2.753 / 2.852 | 3 | 0.6891 / 0.6104 / 0.6340 |
| 4 | 1034 / 1140 / 1272 | 6.200 / 5.000 / 4.428 | 3 | 1.338 / 1.227 / 1.089 |

Log: `/tmp/multica-i1-s0-benchmark-instrumented.log`, exit 0. Initial uninstrumented runs also passed; `/tmp/multica-i1-s0-benchmark.log` is retained, not mixed into the comparison.

Engineering acceptance for the same benchmark/environment, frozen now before production edits: median throughput at least 70% of baseline for one writer (374.85/s), at least 50% for four writers (570/s); median P95 at most twice baseline (5.704/10 ms) and every run P95 at most 20 ms. Query increase must be enumerated and lock costs reported. A failure blocks performance acceptance and FG until corrected or explicitly recorded as unresolved; default disabled does not waive it. Server-side lock-wait sampling and populated-workspace load remain required for FG.

First wired measurement (`/tmp/i1-s1-performance.log`, benchmark command above) failed the four-writer gate despite zero HTTP errors: median single 389.9 writes/s, P95 3.891 ms; median four 461.8 writes/s, P95 11.16 ms. Eight transaction statements/write; lock statement time single 1.298/1.343/1.466 ms and four 8.503/6.963/6.718 ms. This is a failed acceptance result, not hidden by the benchmark process's exit 0. The follow-up batches ordered pre-issue locks to reduce network round trips while retaining all SQL statements, ownership and lock order. Benchmark instrumentation counts queued statements and measures an all-lock batch through its final Close; that span includes network/decoding and is not a pure server lock-wait metric.

## S1

Real database RED: `TestUpdateIssueIterationFacts` called HTTP UpdateIssue successfully but found zero iteration events instead of one. `TestUpdateIssueIterationNoFactChange` passed for unchanged title, description-only and priority-only updates. Command: guarded `go -C server test ./internal/handler -run 'TestUpdateIssueIteration' -count=1 -v`, isolated DATABASE_URL as above. Exit 1 for the missing fact; `/tmp/i1-s1-red.log`. Production recorder work began only after this failure.

`pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=1 --force` completed with 15/15 tasks, exit 0, no cache hits; `/tmp/multica-i1-s1-static.log`. Existing warnings remain. No frontend code changes in this slice.

`pnpm exec turbo run test --filter='!@multica/mobile' --concurrency=1 --force -- --maxWorkers=2` completed with 5/5 tasks and 10,054 tests (core 2,635; docs 62; views 6,118; web 282; desktop 957), exit 0, no cache hits; `/tmp/multica-i1-s1-ts-tests.log`. Existing jsdom canvas warnings do not represent browser/Electron E2E coverage.

The first expanded S1 run found an invalid custom-status fixture color (`issue_status_color_check`), separate from the expected initial RED. The fixture must use the existing color vocabulary before its category assertion supplies evidence. This failed run is preserved at `/tmp/i1-s1-fixture-failure.log`, rather than treated as a recorder failure.

Independent read-only review found a task-originator authorization race in unlocked task reads used for agent assignment. Resolved by holding the existing task SHARE NOWAIT lock while it supplies authorization, preserving terminal-task content-only writes. `TestUpdateIssueIterationActorTaskFence` proves busy-task rollback, terminal private-assignment refusal and permitted content edits; `TestUpdateIssueIterationRetryIdentity` proves stable identity and exactly one committed event after a rolled-back 55P03 attempt. Final read-only review found no S1 blocker.

## Final S1 result

S1 is verified; FG remains **not passed**. The S1 commit contains this record and the implementation. W01 includes UpdateIssue, per-item BatchUpdateIssues and MoveIssue; other writer paths remain for S2. No capability, production environment, push, deployment, lifecycle, closure or UI was enabled/implemented.

| Requirement | Real database evidence |
| --- | --- |
| Issue/facts/event/scope commit together; old omitted fields retained | `TestUpdateIssueIterationFacts`: title/category/project/assignee, one event, issue/scope versions, original participation, pointer and rollover=2 |
| Original O/facts retained; category/started transitions | `TestUpdateIssueIterationTransitions`, `ArchivedCustomAndNullFacts`: cancel/reopen/status, started persists through reopen, archived custom category resolves, null references serialize as null |
| No spurious scope activity | `TestUpdateIssueIterationNoFactChange`, `AttachmentOnlyAndBatch`: no-op/description/priority/attachment-only unchanged scope; batch records its status fact |
| Failure rolls everything back | `TestUpdateIssueIterationAtomicFailure`: injected event write or attachment bind failure leaves title/revision/attachment binding/started/events/scope unchanged |
| CAS single winner | `TestUpdateIssueIterationConcurrentRevision`: two connections, HTTP 200/409, one event and one issue/scope increment |
| Concurrent join is observed and fence precedes issue/attachment | `TestUpdateIssueIterationJoinBeforeWriter`: another connection owns I1 fence, acquires attachment/issue NOWAIT, joins and commits, writer records one fact with post-join sample. Batch-start barrier proves no attachment/issue lock before batch; explicit queue order and SQL equivalence check additionally verify ordering within batch |
| Reauthorization and retry identity | `RevokedBeforeTransaction`, `ActorTaskFence`, `RetryIdentity`: 403 after revoke without revision disclosure; task NOWAIT and terminal behavior; rollback then one stable operation/event |
| Sequence/time and alternate W01 paths | `SequenceClampsClock`, `MovePath`: latest-index sequence, clamped occurred_at with original sampled_at, actual MoveIssue event |
| Execution unchanged by recorder | `DoesNotChangeExecution` plus existing reassign/cancel single/batch regression tests |

Final independent commands (same isolated DATABASE_URL, each exit 0):

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 \
  ./internal/iteration ./internal/migrations ./cmd/migrate -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 \
  ./internal/handler -run 'Test(UpdateIssueIteration|IssueWriteFenceSQL|RevisionConflictsPreserveLatestIssueAndComment|TextBaselinesIgnoreUnrelatedAggregateRevisionChanges|ConcurrentRevisionWritesHaveExactlyOneWinner|NoOpIssue|IssueUpdateAndAttachmentBindingExcludeInterleavingMutation|IssueUpdateRollsBackWhenAttachmentBindingFails|ProjectAssociation|UpdateIssueReassign|BatchUpdateIssueReassign|UpdateIssueCancelStatus|BatchUpdateIssueCancelStatus)' -count=1 -json
go -C server vet -p 2 ./...
go -C server build -p 2 ./...
make sqlc
git diff --check
```

- Foundation: 554 passing test entries, 97 top-level; one existing `TestCommentContentBigramRequirementMatchesRealPGBigm` skipped (not counted as passed). `/tmp/multica-i1-s1-foundation-final.jsonl`.
- Handler: 78 passing entries, 30 top-level, no failures/skips. Includes P1 project-association busy rollback/deletion wins and issue attachment/CAS regressions. `/tmp/multica-i1-s1-handler-final.jsonl`.
- Build/vet logs: `/tmp/multica-i1-s1-build.log`, `/tmp/multica-i1-s1-vet.log`. sqlc regeneration matched SHA256 of all 81 already-generated files; `/tmp/multica-i1-s1-sqlc.log` and `/tmp/multica-i1-s1-sqlc-before.json`.
- Final perf `/tmp/i1-s1-performance-batch5.log`: single 453.2/469.2/477.5 writes/s, P95 3.994/3.314/3.343 ms; four 633.5/663.1/678.2 writes/s, P95 9.211/8.293/8.277 ms. All frozen gates pass. Median lock-statement time single 0.8251 ms, four 4.398 ms. Eight SQL statements remain; the initial five locks share one network batch. The four-lock intermediate result (569.9/s, just below threshold) remains in `/tmp/i1-s1-performance-batched.log`.

## S2 handoff and limits

- W02–W16 still need their own actual production-path facts/no-event assertions and transaction integration; W17 exemption needs a no-event test. S1 does not prove those paths or FG.
- Borrowed recorder currently handles unchanged membership in planned/active iterations. S2 must extend deletion/actual execution start and lifecycle later supplies membership changes; do not silently use the S1 helper for unsupported transitions.
- Preserve shared SQL constant and authoritative query equivalence test when changing batched fences. The batch still consumes queued later locks after an absent member; 403 can wait for them until request context cancellation. It never writes business data or bypasses authorization.
- Pending FG: operation replay/query authorization, settings/capability, explicit-field compatibility, cleanup/revocation, all writer lock races, server-side wait sampling and populated workspace performance. Do not start lifecycle/history or client work yet.
- Full Go repository tests, Web/Electron E2E and remote CI were not run in S1. Mobile was excluded from root TS scripts; no response DTO/client fields changed here. I1 remains disabled.
