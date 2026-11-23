# Triage backend peer review

Final resolution (2026-10-05): **all BR1–BR5 findings are closed** on the delivered source. The independent final audit checked context/status binding, nonblocking outbox guards, replay ordering, infrastructure-error preservation and resource-parent locks. Full Go race tests and current Web/native acceptance pass. The original findings below are retained as review history; the parent verification ledger and `.omx/reports/triage-t1-acceptance-final-audit.md` give the final disposition.

2026-10-05. Read-only review of `triage.go`, `triage_actions.go`, `triage_queries.go`, `triage_notifications.go`, the frozen API contract and full T1 PRD. This reviewer owns CSV import implementation and deliberately excluded that code from this peer review. No implementation files were edited during this review. Findings were sent to the backend owner and parent while implementation continued.

This is a source/transaction review, not a claim that passing targeted tests prove all requirements. Line references describe the review snapshot and may move as the owner applies fixes.

## Open findings at snapshot

### BR-1 — P1: execution retry does not bind all actual execution inputs

`server/internal/handler/triage_actions.go:272` fingerprints issue title/description, project/assignee, agent RuntimeConfig and Composio allowlist, plus project resources. It does not bind agent `McpConfig`, `CustomEnv`, `CustomArgs`, `Instructions`, or model selection. These are independent stored fields and are consumed when creating the daemon claim response (`server/internal/handler/daemon.go:2256` through the Agent response construction around2317).

A user can accept-and-execute, receive an accepted-but-start-failed action, change these execution inputs, then retry the old action. Both fingerprint comparisons still pass, and the attempt uses changed tools/configuration under the earlier decision. The frozen design states that a changed execution context requires a fresh explicit normal run decision.

The issue status is also omitted. Marking the accepted issue cancelled/done while its first startup is failed does not change admission_status and therefore still passes both retry checks. `EnqueuePreparedIssueTaskInTx` checks formal admission, not this status transition.

**Required proof:** failed-start retry after each material configuration change returns409 and enqueues nothing; cancelling the issue does likewise. Existing no-change and known-terminal-task replay controls must remain green. Bind a canonical set of relevant inputs rather than all volatile row fields.

### BR-2 — P2: outbox delivery and summary retry acquire the same locks in opposite order

The updated `beginTriageWrite` now takes the actor's subscriber/revocation advisory guard before its member row (`triage.go:227`). Summary writes subsequently UPSERT the stable existing `triage_notification` row.

Delivery still locks pending outbox rows first (`triage_notifications.go:158`, `FOR UPDATE SKIP LOCKED`) and only then takes each recipient's subscriber guard (`:215`). For an importer/operator receiving their own batch summary:

1. Summary retry holds the subscriber guard and waits to update the outbox row.
2. Delivery holds that outbox row and waits for the subscriber guard.

This is a direct two-transaction deadlock, independent of membership revocation. `SKIP LOCKED` does not help once delivery acquired the row first. Align lock ordering, or use explicit NOWAIT with rollback/retry rather than waiting with an inverted lock held.

**Required proof:** controlled concurrent batch-summary retry and delivery complete without deadlock; exactly one inbox identity and latest cumulative totals remain. Also check reviewer/recipient guards, not only the actor.

### BR-3 — P2: time-dependent validation runs before idempotent action replay

`actOnTriageItem` calls `triageValidateAction` before finding an existing request key (`triage_actions.go:277`). A successful snooze request replayed after its snooze time is reached now fails the future-time check with400, even though its immutable action already exists.

Relative-time validation must run for a new action after the existing request-key branch. Replay should recover the stored action/current item, with the same tombstone behavior if deleted, rather than reinterpret an old successful intention using today's clock.

**Required proof:** replay an already-recorded snooze after its time passes; action ID and action count remain unchanged, response is successful. A genuinely new past-dated snooze remains400.

### BR-4 — P2: infrastructure read failures are mislabeled as deleted results

Manual intake replay (`triage.go:642`), action replay (`triage_actions.go:313`) and execution retry (`triage_actions.go:523`) translate every `triageItem` error into409 with a deleted-result message. That loader performs issue, triage annotation, workspace and label queries; a database/query failure is not proof the item was deleted.

Only an explicit scoped404/no-row result should become a durable deleted-result409. Preserve5xx for infrastructure failures as required by the API error contract, so clients do not show an existing issue as permanently deleted.

**Required proof:** inject a non-no-row loader/database error during replay and observe5xx without writes; actual issue deletion still returns409 with its stable ID.

### BR-5 — P2: project resource check is not fenced against resource writes

`triageExecutionSnapshot` hashes the project_resource aggregate without locking those rows or taking a shared project-resource guard. The project is held `FOR SHARE`, but `CreateProjectResource`/`UpdateProjectResource` directly write resources without taking a conflicting project lock. A resource can therefore change after the final fingerprint read and before enqueue commits.

This weakens the stated guarantee that the original project/resource selection is rechecked inside the final transaction. The fix must share a guard with add/update/delete resource writers (locking existing resource rows alone cannot prevent an added row). Scope coordination with the parent is needed because those handlers are outside the backend lane's original files.

**Required proof:** pause immediately after final fingerprint read, attempt a concurrent resource replacement/addition, then verify either serialization with unchanged authorized context or a409/no-enqueue result.

## Findings fixed during the review (source rechecked)

- **BR-F1, P1 — Observer agent intake bypass:** dedicated `CreateTriageItem` originally omitted the Contributor autonomy gate applied by `CreateIssue`. `triage.go:611` now calls `requireAgentAutonomy(...AutonomyContributor,"create issues")`. Backend owner must retain Observer denial and Contributor control tests.
- **BR-F2, P2 — original membership/owned-agent revocation inversion:** `beginTriageWrite` originally held the member row before acquiring an owned candidate agent while revocation locks owned agents before deleting the member. Actor subscriber/revocation guard now precedes member/agent access. BR-2 describes the additional outbox inversion introduced/exposed by this ordering and remains separate.
- **BR-F3, P2 — notifications invisible to active clients:** delivery originally committed inbox rows without events; core triage invalidation does not invalidate inbox queries. `triage_notifications.go:245–254` now publishes `protocol.EventInboxNew` only after commit.
- **BR-F4, P2 — no operator summary for review batches:** `CommitTriageBatch` now calls `triageBatchSummary` (`triage_queries.go:430`), using the durable outbox identity rather than relying only on HTTP output.
- **BR-F5, P2 — new reviewer missed future snooze reminder:** reassignment while snoozed now schedules the latest snooze action's due identity for the new reviewer (`triage_notifications.go:90–103`). Existing old-recipient delivery is invalidated by the current reviewer/time check.
- Candidate agent/squad row locking and unconditional current permission-mode visibility checks are now present in `triageValidateCandidates`; intake also locks requested issue labels. These address prior integration feedback.

## Other observations / lower-priority follow-up

- UUID tombstone replay is implemented. When the action endpoint was called by human-readable issue identifier, deletion prevents `triageResolveID` before request-key lookup, returning404 rather than the frozen409/stable-ID deleted result. Current core clients use UUIDs, so this is a compatibility edge rather than a main UI blocker.
- Batch preview duplicate detection keys the raw input string, whereas commit resolves UUID/identifier aliases before rejecting duplicates. A preview can therefore call two aliases valid and later reject the commit. Normalize both consistently when tightening the endpoint.
- Batch summary's comment calls the key a request-key set, but it currently hashes the whole ordered payload. Reordering the same row requests creates another summary identity. Clarify whether an ordered request is intentionally a new batch attempt or canonicalize the stable set.

## Positive implementation checks

These were inspected in source; they are not a substitute for the parent integration gate:

- Settings default off; admin/owner writes use revision CAS and the same settings fence as pending-producing writes. Disable counts all pending, including snoozed.
- Decisions are human-only and recheck current membership. Dedicated agent intake remains available subject to the restored existing autonomy gate.
- Acceptance fields, admission transition, revision and immutable action snapshot share one transaction. Batch preview rolls its mutations back before any action/outbox commit. Batch commit enforces its server action allowlist and supports row-level results.
- Ordinary acceptance emits triage cache events and no ordinary assignment/created execution event. It does not inherit a default project squad into an unset assignee.
- Explicit execution commits accepted+pending intent before remote preparation, then atomically records task ID with enqueue. Known task IDs short-circuit repeated execution even after terminal status. Failed startup preserves accepted admission and a durable execution error.
- Action history queries immutable snapshots, preserving prior rounds and deleted issue history. Reopen preserves first-entered time and advances the round. Deletion SQL retains action/intake/import request identities.
- Notifications are durable, recipient membership is checked at delivery, and current reviewer/snooze-time/admission checks suppress obsolete scheduled reminders. Delivery failure does not roll back acceptance.

## Handoff

Backend owner is actively fixing the open findings. Parent should re-read current source and test evidence before closing each item; this artifact is a timestamped review, not a frozen assertion that still applies after subsequent fixes. No database tests were rerun by this read-only peer pass, and no migration/browser proof is included here.
