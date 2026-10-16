# S04 cancellation integration contract

Implementation handoff, 2026-10-02. The bounded cancellation lane is implemented and under final integrated verification; parent owns the admission and rollout lanes. This supplements the approved terminal/security designs without changing their product scope.

## Ownership and agreed schema

service_impl owns new `service/admin_operation.go`, new SQL source `admin_operation.sql`, cancellation tests, the `CancelTaskWithResult` transaction/post-commit extraction, cancellation/status/ACK handler methods, the narrow `CreateOperationInTx` action allowlist, `adminOperationResponse` and effective-root query wiring. Parent owns migrations/sqlc, admission service, claim/reclaim methods in `task.go`, router, shared exports and admission DTO wiring. Client lifecycle/ACK transport remains frontend_impl's lane.

Parent-approved schema additions:

- `agent_task_queue.state_version bigint default 1`, incremented by a BEFORE UPDATE trigger only when status, runtime_id or dispatched_at actually changes. LIKE INCLUDING ALL fixtures must explicitly recreate this trigger.
- `agent_task_queue.execution_admission_version bigint null`, owned by the admission lane.
- `admin_operation.execution_runtime_id uuid`, `execution_dispatched_at timestamptz`, `next_reconcile_at timestamptz`, `effects_completed_at timestamptz`.
- Existing `admin_operation.execution_fence bigint` stores the pre-cancellation task state_version. Existing installation/task/binding/root/confirmation/deadline columns remain authoritative.
- Managed reclaim preserves dispatched_at and refreshes only the lease. Legacy reclaim behavior stays unchanged.

The task's cancellation UPDATE increments state_version. ACK and follower resolution therefore validate the operation's captured execution tuple/root fact, not equality with the task's later state_version. Once dispatched, runtime_id + dispatched_at alone define the execution fence; target_version is optional and excluded from matching and hashing. Never-dispatched queued/deferred requests require target_version; a subsequent cancellation of their terminal root accepts the original queued version or current terminal version. A retry task always has its own ID.

## Service and lock boundaries

`CancelTaskInTx` must reuse the existing user-initiated cancellation SQL, delegated-failure receipt acknowledgement, queued chat handling and chat pointer advance. The legacy public wrapper opens its transaction and runs the same helper. Its post-commit result describes whether any transition happened and what idempotent finalization, runtime reconciliation, token revocation, broadcast and capacity wakeup remain. Do not call a public transaction-owning cancel wrapper inside an admin transaction.

Administrative cancellation locks the actor and rechecks current role/version through S01, then uses the existing chat-session-before-task order before reading the target fence and selecting the cancellation root. If installation/binding locking is required, it precedes chat/task locks and follows the shared credential/installation order. No network call occurs while locks are held. Target organization derives from task -> agent.workspace_id -> organization_workspace; a platform role grants no content access.

A definitive execution_fence_conflict commits a failed operation with request/failed audits before returning409. Same-key replay and lookup return that failed receipt, preventing a delayed original request from later applying and giving the UI an authoritative way to clear an uncertain draft. Failed receipts have no applied timestamp, effect work or daemon schedule and are excluded from cancellation-root lookup.

Each actor/key keeps its own operation and request audit. Only the root changes the task or performs the original cancellation effects. Followers reference a root, never another follower. Root confirmation/audit is atomic; bounded follower synchronization may lag, so reads expose effective root state alongside the actor's own operation/audit timestamps.

`CreateOperationInTx` retains the S01 actor lock, non-secret hash and replay/conflict checks. Extend its controlled allowlist with `task.cancel` targeting `task` and initial `applied`; coordinate parent admission values `installation.admission`, target `installation`, result `admission_stopped|admission_accepting`, initial `applied`. No credentials enter payload hashes.

## SQL query contracts

Names below are intended source-query names, finalized when implementation starts.

| Query | Inputs and invariants |
| --- | --- |
| `GetAdminCancellationTaskForUpdate` | Organization + task ID; join agent and organization_workspace; `FOR UPDATE OF task` only after the chat-session lock. Return internal row/fence metadata, never serialize task content. |
| `SetAdminCancellationOperation` | Actor-owned operation ID/org + captured task/runtime/dispatched/state version + optional installation/binding/epoch + optional root; set resolved state/result/confirmation/deadline/next scan in the same transaction as cancellation and request/applied audits. |
| `FindAdminCancellationRoot` | Organization, task ID, runtime and dispatched time using `IS NOT DISTINCT FROM`; kind `task.cancel`, root_operation_id NULL; target task lock elects the root. Never use a unique target/fence constraint. |
| `GetAdminCancellationRootForUpdate` | Root ID/org/task/tuple; validate root type and binding before locking. ACK order is source credential/current binding, chat/task, then operation. |
| `ConfirmAdminCancellationRoot` | Root ID + version CAS + state applied; set succeeded/confirmed/result and confirmed_at, increment version, queue follower reconciliation. No rewrite of an existing main terminal result. |
| `RecordAdminCancellationPhase` | Append allowlisted snapshots/codes only. Use the existing operation+phase unique index with conflict no-op to deduplicate each root/follower phase and repeated daemon evidence. Never copy branch/path/error into the admin audit. |
| `LeaseDueAdminCancellations` | Root operations only, indexed next_reconcile_at, deterministic order, LIMIT <= 100 and `FOR UPDATE SKIP LOCKED`; move the next scan before releasing locks. No network or task-side effects while operation locks remain held. |
| `MarkAdminCancellationEffectsComplete` | Root/version CAS; set effects_completed_at without replacing confirmation or terminal result. Replays use the existing idempotent task finalizers/token deletion/cache invalidation; metrics keep task-ID deduplication. |
| `MarkAdminCancellationUnconfirmed` | Only applied roots past ack_deadline; keep main state applied and the task cancelled, change reconciliation/confirmation evidence and audit once. Never expire an already-applied cancellation. |
| `ListAdminCancellationFollowers` / `SyncAdminCancellationFollower` | Bounded direct followers of a root, stable order and row/version CAS; copy only the root's effective outcome, preserve each actor/reason/key, append that follower's own phase audit. Restart resumes incomplete followers. |
| `HasUnreconciledAdminCancellationFollowers` | Nonlocking existence check before clearing a terminal root's schedule; a short SKIP LOCKED batch does not prove that every follower was repaired. |
| `GetAdminOperationWithRoot` | Actor/org scoped operation lookup plus a validated direct root join; return actor's receipt with effective root confirmation. The key lookup remains fixed to the current actor. |
| `GetTaskCancellationMetadata` | Exact task/current execution tuple; expose root ID/binding epoch/fence/deadline only through the existing authorized daemon status endpoint. Legacy/unrelated root rows cannot be inferred from claimed client IDs. |

Ordinary concurrent-index families owned by parent: task/fence/kind root lookup, root/state followers, next_reconcile_at/state/id due scan, target installation/accepted_at/id. All indexes need independent single-statement migrations. No foreign keys or cascades.

## HTTP and daemon contract

Admin cancel accepts a stable Idempotency-Key, nonempty reason and expected_execution_fence. For dispatched work the fence is runtime_id + dispatched_at; queued/deferred also use the server-issued target version. The task's organization, runtime and binding are derived server-side. Reply 202 with the persisted operation and current task status, distinguishing applied from confirmed.

Preserve the daemon status response's `status` string and add optional `cancellation` metadata: `{ operation_id: root, binding_epoch: decimal string, execution_fence: { runtime_id, dispatched_at }, ack_deadline }`. HTTP/WS claim payloads carry an immutable execution_fence.

ACK extends the existing optional body with operation_id, binding_epoch, execution_fence and outcome. `stopped` requires real process-completion evidence; `not_observed` after a restart records evidence only and never confirms stopping. Legacy ACKs continue branch/error/durable-directory and chat cleanup; management confirmation requires a unique task/current-binding/execution-fence match. Missing proof remains unconfirmed.

Explicit management receipts require a complete, correctly typed JSON group with no trailing JSON. Empty/legacy-only bodies keep cleanup compatibility but never confirm management cancellation. Runtime metadata must advertise admin_cancel_ack_v1 before the server promises confirmation; otherwise the result is confirmation_unavailable.

Parent migration499 adds a separate claim_generation for delivery races while preserving the execution tuple. Claim-only payload rejection uses CancelClaimedTask, CancelClaimedTaskWithReason and FailClaimedTask. Their expected generation is checked under the original task-write transaction only; normal chat finalization and automatic retries do not inherit that guard. Root owns handler callsite replacement and finalization/requeue/receipt generation CAS.

Existing `AckTaskCancelled` currently performs branch/error/durable-directory writes independently. Move those existing non-overwrite/status-CAS writes and any root confirmation/audit into one transaction, then run deferred chat finalization and rebroadcast after commit. Completed/failed tasks remain unchanged by late ACKs. Do not weaken current authentication to let revoked bindings send an ACK.

## Regression cases

1. Before-dispatch queued and deferred cancellation: task/pointer/operation/audits atomic; succeeded with cancelled_before_dispatch and no fabricated daemon confirmation.
2. Running/dispatched/waiting_local_directory: immediate cancelled status plus applied/awaiting confirmation; offline behavior identical at commit.
3. Complete-vs-cancel and cancellation-vs-pin: both serial orders, session/task lock order, no terminal result overwrite; retry/new task unaffected.
4. Fence conflicts: runtime switch, new dispatched timestamp, queued/deferred stale state_version, task requeue ABA; managed reclaim preserves the original fence.
5. Two admins cancel the same fence, both lose responses, recover distinct receipts by their own original keys, and share one root/task mutation. Changed non-secret same-key payload returns 409.
6. Failure injection at task update, chat pointer, operation update, request/applied audit and commit; no partial target or ledger writes and no precommit notifications.
7. ACK stopped vs not_observed, forged root, wrong task/runtime/time/epoch, revoked or rebound credential, legacy ACK, late complete/fail, duplicate and delayed terminal evidence; branch/error/durable-directory values never overwritten.
8. ACK audit failure rolls back both legacy field writes and confirmation; same valid evidence retry succeeds once.
9. Deadline advances only unconfirmed evidence, never restores task state or marks process stopped; reconnect still sees cancellation metadata.
10. Crash after cancellation commit before notifications/effects marker, and after root confirmation before followers synchronize; bounded scanner and effective-root reads recover truthful results. Two scanners produce one phase audit per operation.
11. Read observer, stale actor version, revoked role, foreign organization and private-content DTO boundaries remain enforced on POST, polling and status paths.

Root continues to own admission races and native/capacity S07 acceptance. Targeted S04 success must not be presented as completion of those separate responsibilities.
