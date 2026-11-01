# Reused final-database diagnosis — 2026-11-01

**The five failures are explained by accumulated test fixtures, not a confirmed production regression. Use a fresh empty database for the final whole-run gate and repair the two fixture cleanup seams. Do not delete protected knowledge history to make the reused database fit the test helper's budget.**

Only read-only SQL and source/log inspection were performed against `multica_knowledge_final_20261101_e7059940`. Every connection set `default_transaction_read_only=on`, a five-second statement timeout and 100ms lock timeout; `transaction_read_only=on` was verified. No test, worker/drainer call, row/advisory lock, data mutation, Provider call, source edit or sqlc generation was performed. This report and its JSON companion are the only written files.

## 1. Fixed fixture drain budget exceeded by retained history

`finishKnowledgeTestScans` in `server/internal/service/collaboration_knowledge_invalidation_test.go:98` calls `ScanPending(...,100)` at most 100 times. That permits **10,000 short scan transactions**, not 10,000 complete scans. The first call in `newKnowledgeInvalidationFixture` drains the **global old backlog before creating this test's pending-only candidate**.

Production `ScanPending` intentionally counts bounded progress transactions. A job spends one transaction freezing its upper bound, one per ever-published candidate, and one marking completion. `NextKnowledgeScanCandidate` uses retained routing without a source/target existence join; deleted parents are processed without resurrection, but still consume a cursor step. Candidate versions cannot be deleted and fixture cleanup preserves pending invalidations.

At `TestKnowledgePendingChangesDoNotSignalTarget`'s second-run start, **09:21:57.509281 +08:00**, retained timestamps establish:

- 238 candidates already had immutable approval timestamps.
- 98 `all_sources` scans were pending; **53 were not yet started**.
- Those 53 jobs alone required at least **53 × (238 + start + completion) = 12,720 transactions**. This excludes 45 already-started scans and all source-specific work.
- All 98 scans pending at that start completed **after** the failed test ended at 09:22:10.004520. This shows continued cursor progress, rather than a permanently stuck scan.

The test's 12.5-second failure therefore does not show a pending proposal emitting a target hint. Its global warmup exhausted a fixed per-run fixture allowance. On the first run, the same test passed in 2.9 seconds.

At the initial read-only observation, the database held 593 retained candidates, 380 ever-published; all 593 source issues and target workspaces had already been removed by fixture cleanup. Retention is expected. It multiplies later `all_sources` work and makes a repeatedly reused database unsuitable for an unconditional fixed-budget fixture warmup.

## 2. Old realtime warmups are blocked by an orphan at the queue head

`LockNextCollaborationRealtimeEvent` selects the oldest pending event joined to `issue_collaboration`, ordered by `created_at,id`. It does not join the live `issue` table. In `CollaborationRealtime.deliverNext`, every non-memory event later calls `GetIssue` before marking the event delivered. A missing issue returns `pgx.ErrNoRows`; rollback preserves the same oldest pending event for the next call.

The exact first blocker is event **`ef4c4ac5-ee2e-44d4-b9e9-3d6234be6fb3`**, kind `created`, at **09:13:26.319396 +08:00**, for absent issue **`01a194ab-2435-74f1-a53f-0d925b6a3b99`**. Its `issue_collaboration` row still exists. The event timestamp falls inside first-run `TestKnowledgePreparationSourceRevocationBothCommitOrders/source_revocation` (09:13:26.142704–09:13:26.509904).

The first run's memory/realtime tests had already passed at 09:08:19–09:08:56. The second run encountered the leftover head before it could drain its own fixture events:

| Test | Second-run failure |
| --- | --- |
| `TestCollaborationMemoryRealtimePublishesOnlyIssueScope` | 09:22:51.920818, initial global drain, `no rows in result set` |
| `TestCollaborationRealtimeOutboxRetriesWithoutPrivatePayloads` | 09:23:28.937282, initial drain |
| `TestCollaborationRealtimeRetainsFailedSourceInvalidation` | 09:23:29.071198, retry still sees old head |
| `TestCollaborationRealtimeRemovalReachesRevokedAccount` | 09:23:29.222028, initial drain |

The stable read-only catalogue contains **18 orphan source groups / 84 eligible pending events**, including 66 non-memory events that can take this failure path. The companion JSON lists every source ID, timestamp and event-kind set. There were 86 pending events overall; two do not join this orphan queue path.

## 3. Exact fixture causes

**External preparation source:** `newPreparationExternalFixture` in `task_knowledge_preparation_boundary_test.go:24` creates an additional workspace and collaboration source through `svc.Create`, then memory, proposal/approval and responsibility changes. Its inherited `collaborationFixture` cleanup only scopes service-owned rows to the original `f.workspace`, not this second source workspace. The first-run source IDs are:

- `01a194ab-2435-74f1-a53f-0d925b6a3b99`: source-revocation-first case.
- `01a194ab-25a7-7e19-84b3-687901fab2bc`: disclosure-first case.
- `01a194ab-276b-7119-ac01-1de3f6099042`: source authority changes between build/finalize.

**RR2 logging subprocess:** `TestKnowledgeServerLogProbe` in `task_knowledge_logging_test.go:99` starts from an ordinary `testutil.Fixture`, then creates a collaboration source through its service for six nonlegacy modes. Those service-created rows have no corresponding fixed-source cleanup. The first-run six sources were created at 09:13:44.686805–09:13:45.914641, matching the nonlegacy `TestKnowledgeServerFailureLogsUsePersistedState` subprocesses. All first/second-run IDs are in the JSON and were sent to their owner.

`Fixture.Workspace` registers a raw `DELETE FROM workspace WHERE id=...` through `deleteOnCleanup`. The read-only PostgreSQL catalogue confirms the legacy `issue_workspace_id_fkey` has **ON DELETE CASCADE**, so the issue disappears during ordinary fixture teardown. New collaboration tables intentionally have no cascade; their metadata and events remain. This differs from the actual production workspace-delete path, which refuses source workspaces containing collaboration records before teardown.

## 4. Recommended disposition

Keep e705 unchanged as failed-state evidence. Main's new, empty, migrated database is appropriate for the final whole-run gate; do not copy prior fixtures into it. Protecting candidate/invalidation history is correct, so old global scan work must not be eliminated by deleting those rows or weakening worker policy.

The assigned fixture owner should register fixed source-ID cleanup for each extra service-created source's legacy events, collaboration/session/participant/handoff/notification/memory/operation rows before ordinary parent cleanup, using existing fixture helpers. Preserve protected candidate and invalidation records. This prevents future orphan queue poisoning; it does not make a fixed 10,000-step warmup sufficient for arbitrarily accumulated protected history.

For future repeat-run testing, either isolate fixture namespaces/causes or drain against an observed workload with an honest bound. No production change is justified by these observations alone. `knowledge_launch_leaf` owns the separate C2 cleanup reproduction; its reported red/green evidence belongs under `research/fixture-cleanup-2026-11-01/` and was not rerun against e705 by this agent.

The first service package had an unrelated builtin-template failure; the five tests discussed here **individually passed** in that first run. The report does not relabel the entire original package run as green. Exact run intervals, first-run source hashes, read-only observations and the workload reconstruction are retained in the JSON companion.
