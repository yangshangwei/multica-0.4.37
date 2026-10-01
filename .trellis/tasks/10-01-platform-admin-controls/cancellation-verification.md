# S04 cancellation lane verification

Worktree: `/Volumes/artisan/code/2026/multica-platform-admin`. Upstream integrated S02/S03/S05 commit: `a38158def`. This document covers the bounded cancellation service/handler/coordinator lane; admission, native/browser integration and capacity remain parent/S07 responsibilities.

## Implemented

- `service/admin_operation.go`: atomic cancellation/operation/audit, stable actor/key recovery, one cancellation root with separate actor receipts, current-binding ACK confirmation, effective-root reads, bounded recovery and follower repair.
- `service/task_cancellation.go` and the agreed `task.go` extraction: existing user/server cancellation semantics and chat pointer writes reused inside caller-owned transactions; durable chat finalization and its effects marker commit together.
- Claim-only rejection helpers atomically validate task/runtime/dispatched_at/claim_generation before cancellation or failure. The guard applies only to the original mutation transaction; current-claim chat effects and deferred automatic retry remain intact.
- `handler/admin_operation.go`, `admin_execution.go`, `admin_auth.go` and daemon status/ACK methods: cancellation route contract, full-precision execution fences, safe metadata/receipt DTOs, strict explicit ACK decoding, legacy cleanup without fabricated confirmation.
- SQL sources `admin_operation.sql` and the narrow `admin_execution.sql` projection. Parent owns migrations, generated SQL, claim/reclaim guards, router and coordinator startup wiring.

## Verified behavior

- Never-dispatched queued/deferred tasks cancel without daemon confirmation. Running/dispatched/waiting execution cancellation is immediate but remains applied until explicit stopped evidence from the current bound daemon.
- Different actors recover their own operation IDs under their own original keys while sharing one root. Followers show effective root confirmation before their own audit catches up, then bounded reconciliation persists the missing phases.
- Task/audit failure rolls back the operation; uncertain commit is recoverable by original key. Chat effect marker failure rolls back the chat outcome, and retry creates one outcome.
- not_observed and legacy cleanup do not confirm stopping. Wrong task/runtime/time/epoch and revoked bindings cannot confirm. Existing branch/error fields are not overwritten and private diagnostics do not enter admin audit/DTOs.
- ACK audit failure rolls back confirmation and legacy delivery writes. Deadline changes only confirmation/reconciliation to unconfirmed; it never restores a task or reports process exit.
- Completion/cancellation races keep the winning terminal result. Retry task IDs are unaffected. Queued state-version ABA is rejected; status-only progress after dispatch keeps the same execution fence valid.
- Definitive fence conflict has a committed failed receipt and request/failed audit, no applied_at or schedule; same-key replay cannot later apply even if the task advances to the previously supplied version. Failed requests never become cancellation roots.
- Stale claim-generation cancellation/reason/failure helpers do not mutate a reclaimed delivery. Valid current cancellation still finalizes chat; valid runtime-offline failure still creates the existing deferred retry.

## Evidence and review repairs

- Existing cancellation baseline passed before extraction (service1.225s).
- Early cancellation service race passed21.276s and legacy/new handler race passed9.089s.
- Independent review found three defects, each reproduced before repair: two-field in-flight fences rejected/status advance conflicted; a locked follower permanently unscheduled its root; malformed explicit ACK with branch_name42 was accepted and could confirm. Combined repaired cancellation service/handler race passed24.842s/4.131s. Additional tests cover trailing JSON and equivalent in-flight fence replay.
- Claim helper stale/current tests passed5.295s after correcting the test expectation to the existing runtime-offline deferred-retry policy.
- Broader service race passed76.654s with `-run '^Test(AdminCancellation|ClaimRejection|PlatformAdmin|Cancel|TaskCancel)'`, including the S01 shared operation/auth regression set.
- `go vet ./internal/service ./internal/handler` and `git diff --check` passed before the final isolated handler run.
- All Go execution used explicit `DATABASE_URL=postgres://multica:multica@localhost:5432/multica_platform_admin_s01_test?sslmode=disable` and `go -C server` from the worktree root. Cancellation fixtures recreate the actual state-version trigger because LIKE INCLUDING ALL does not copy triggers.

An overlapping broad handler run was invalidated by the existing fixed-slug TestMain fixture being deleted by another handler process. The parent and this lane now serialize handler runs against this database. This was a test-environment collision, not a reason to alter product behavior. The final isolated handler result will be appended below.

Final isolated handler race passed11.436s with:

```text
go -C server test -race ./internal/handler -run '^Test(AdminCancellation|AckTaskCancelled|CancelTask_|CancelAndPin_|PinTaskSession_|CancelTaskByUser_|AdminExecutions|ClaimTaskByRuntime_|ClaimTasksByRuntime_|FinalizeTaskClaim|SetTaskDeliveredCommentIDs)' -count=1
```

This includes the root's generation/provenance repair, every claim-only handler replacement, two-field in-flight cancellation, malformed/trailing explicit ACK rejection and definitive failed-receipt lookup. The handler database reservation was released after completion. Raw final output is copied to `.omx/reports/platform-admin/s04-cancellation/handler-race.log`.

## Remaining integration limits

The lane has not run a production S04 browser flow or controlled a real installed daemon/provider. Fake/local Go tests do not prove an offline process stopped. Parent owns final source/build provenance, independent client acceptance, full admission verification and S07 load/native acceptance. No commit or deployment was performed by this subagent.
