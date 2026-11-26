# Boundary and diagnostic design

## Smallest behavior gap and owner
The legitimate M1 authorization setup in TestStageCompletionIndependentConsumerAuthorityAndMinimalReceipt/wrong_consumer is interrupted by a 55P03 on github_pull_request. It never reaches the wrong-consumer refusal assertion. delivery_evidence.go deliberately uses a short global SHARE NOWAIT fence on four PR/link relations to exclude inserts and updates whose writers lack a shared issue lock. That safety fence is correct under contention and must remain.

Expected write set: this child task and db-lock evidence/support; service stage-completion/PR fixture files only if captured interference is fixture-owned. delivery_evidence.go changes require an evidenced product defect and parent coordination. Do not modify unrelated service behavior or fixtures globally to make tests pass.

## Original evidence → hypotheses → discriminating tests
Original raw: /Volumes/artisan/code/2026/multica-0.4.37/.omx/private/qa-work/main-post-merge-e2e-20261124-080504/backend/runs/race/986dfe9cbf73/test.jsonl, 2026-11-24T02:49:02.225Z; original PostgreSQL log 02:49:02.224Z reports SHARE NOWAIT refusal on github_pull_request. No WARNING: DATA RACE and no holder identity retained. Neighboring scenarios and later isolated narrow runs passed; none closes the full-package failure.

H1: leaked fixture/application writer. Capture RowExclusiveLock/stronger granted holders joined to pg_stat_activity, transaction age and SQL; examine fixture goroutine lifetimes and cleanup.
H2: PostgreSQL maintenance. Capture backend_type, query and ShareUpdateExclusiveLock/AccessExclusiveLock on the exact relation; do not assume autovacuum from elapsed suite time.
H3: another package/process. Fresh per-package database plus exact connection inventory/application_name proves who can touch these relations.
H4: own transaction/wrapper interference. Record requester PID and first raw pg error against actual SQL, then compare retained holder PID and transaction.

Use passive lock sampling throughout a full baseline package run and immediate diagnostic snapshot at failure if needed. NOWAIT has no wait queue, so pg_blocking_pids alone is insufficient; enumerate granted conflicting modes. A deterministic controlled two-backend conflict may establish fence behavior, but is labeled induced and not claimed as the historical holder.

## Fix and rollback
Choose after evidence. Synchronize or isolate an evidenced test writer at its owner; do not retry authorization or remove table/credential checks. If maintenance is actually captured, propose test-only fixture isolation to parent before changing database maintenance options. Preserve failure DB and run. Revert only this lane's minimal delta if validation fails; never reset shared checkout or other agents' files.
