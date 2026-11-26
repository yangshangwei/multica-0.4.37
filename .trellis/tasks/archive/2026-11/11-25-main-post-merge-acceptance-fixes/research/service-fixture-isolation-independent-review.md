# Research: Independent service fixture isolation review

- Query: Does reserving old invalidation jobs avoid cross-test mutation without weakening production worker/fairness behavior?
- Scope: internal; read-only code review, service lane supplied RED/GREEN evidence
- Date: 2026-11-25

## Findings

**No remaining actionable source finding in the reviewed fixture-only change; owning-file and complete-package execution remain pending.** This is not full service acceptance or historical NOWAIT closure.

Reviewed `server/internal/service/collaboration_knowledge_invalidation_test.go` SHA256: 518c7c251649ac42c7d120720cce3c1a1e2d2a00abb1fd5f72d788b1b387f663. The change introduces a fixture-owned transaction that reserves currently pending scan and hint rows with SELECT FOR UPDATE SKIP LOCKED, instead of running global old work during fixture bootstrap. Production code and SQL are unchanged.

### Resolved review finding

The first revision of the new regression left its outer `previous` fixture unisolated. After its nested fixture released reservations, final `finishKnowledgeTestScans(previous, worker)` would again process every older package job, recreating the costly coupling in the regression itself. The service implementer added `isolateKnowledgeTestHistory(t, previous)` **before** seeding the regression-owned scans/candidates. Re-read confirms the corrected order. The nested fixture now skips the outer reservation and locks only the new previous-fixture rows; final outer drain sees its own seeded jobs while still excluding package history.

The two administrative fixture paths bypassed the ordinary constructor and invoked the global worker. They now explicitly isolate after registering cleanup and before their tested mutation. This avoids mixed isolation semantics without changing admin assertions.

### Checked invariants

- No pending/candidate history is deleted, marked done or rewritten by isolation. Queries only reserve row locks; rollback releases them. The regression snapshots all selected job/candidate/target columns, including cursors/revisions.
- Worker selection already uses FOR UPDATE SKIP LOCKED for both scans and target hints (`server/pkg/db/queries/collaboration_knowledge.sql:151,198`). Reserved old work is skipped through the existing tested concurrent-consumer contract, not a new production filter.
- Normal fixture reserves at most one connection. Empty selection rolls back immediately. Each `newCollaborationFixture` owns its own pgx pool (`collaboration_test.go:30–42`); no parallel test in this file shares a one-connection pool. A retained reservation plus worker and target-lock transactions fit the existing default pool. Parent must still budget aggregate connections across live lanes.
- Cleanup registration is LIFO. The reservation rollback is registered after original fixture cleanup hooks and before test-owned teardown, so it occurs before old fixture cleanup/pool.Close. Later per-test cleanup targets test-created rows, not reserved history. Rollback errors are reported, unlike the preexisting generic SQL cleanup helper; this change adds no silent cleanup path.
- The fairness test creates both fixture objects before enqueueing either fixture's test work, so the second reservation cannot hide a first fixture's intended busy scan. It retains the assertion that a busy target yields to an independent scan, then completes after release.
- Direct admin source-purge setup creates its memory baseline before isolation, then creates the tested purge/event afterward. The tested new work stays visible. Global all_sources tests still execute their real intended fanout; this fixture repair does not optimize away their assertions.
- The new regression proves own-job completion/publication while unrelated history is unchanged, and proves reserved previous-fixture jobs become processable after the nested scope's cleanup. The corrected outer reservation also exercises nesting.

### Verification still required

The service lane reported first RED (unrelated history changed) and first GREEN (own-job flow and release) before the review adjustment. Re-run the corrected regression, all20 owning-file top-level tests (normal/race where applicable), then supported-toolchain full service normal/race at the unchanged40m/p1/parallel2 on fresh databases. Preserve original timeout/expiry/NOWAIT failures. Failure must be investigated instead of extending budgets or classifying narrow passes as full closure.

## Files found / related specs

- `server/internal/service/collaboration_knowledge_invalidation_test.go`: sole reviewed implementation file.
- `server/internal/service/collaboration_knowledge_invalidation.go`: actual bounded scan/delivery worker.
- `server/pkg/db/queries/collaboration_knowledge.sql`: unchanged SKIP LOCKED selection.
- `server/internal/service/collaboration_test.go`, `collaboration_queue_test.go`, `collaboration_memory_test.go`: fixture pools and cleanup ownership.
- `server/internal/testutil/db.go:95`: cleanup hook registration.
- `.trellis/spec/server/database-test-isolation.md`: retained invalidation history and fresh package DB contracts.

## Caveats / Not Found

Reviewer did not run database tests or inspect live rows. Only source and provided evidence shape were reviewed. Full suite may reveal additional direct worker constructors or unrelated latency issues; this is not proof of an initiating shared CLI/Host cause. Complete-package acceptance and final resource audit remain the parent/service owners' next steps.

## 2026-11-26 final source binding addendum

Current fixture SHA256 d21a2367336d69cb049736a4c5df349307b20207bfd0f21ab6be5d4bbe3b367c. The later change supplies active nested t to snapshot(t), avoiding parent FailNow use; SQL and assertions are unchanged. Owning-file raw now confirms20top/29named PASS, exit0,10.557s. Linux full normal4631PASS/noFAIL/noSKIP/no unfinished accepted in the continuation review. Full race remains pending at this addendum.
