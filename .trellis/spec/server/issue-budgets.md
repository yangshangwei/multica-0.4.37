# Parent-task budget observation

## 1. Scope / trigger

Read before changing ordinary issue budget roots, issue binding, task claim finalization, usage telemetry, daemon journal recovery, or `multica issue budget`. F3-S1 observes only: exceeding a limit, closing a root, or unknown costs never authorize an execution block. Enforcement and reliable settlement require separate contracts.

## 2. Signatures

- Human workspace API: `POST/GET /api/issues/{id}/budget`, `GET /api/issues/{id}/budget/operations/{key}`, `GET /api/issue-budgets/{rootId}`, `POST /api/issue-budgets/{rootId}/operations`, `GET /api/issue-budgets/{rootId}/operations/{key}`, and paginated `observations` / `attempts`. There is no operation collection GET.
- Daemon API: `POST /api/daemon/budget/attempts/{attemptId}/usage` and `GET .../usage/{batchKey}`. Capability: `budget-usage-v1`; claim payload: `budget_attempt_id`.
- CLI: `issue budget enable`, `show`, `set-limit`, `set-mode`, `close`, `reopen`, `operation`, `observations`, `attempts`. Writes require original `--request-key`, `--reason`, `--yes`; management requires `--expect-revision`.
- Six no-FK tables: `issue_budget_root`, `member`, `attempt`, `entry`, `operation`, `observation` (all have the `issue_budget_` prefix). Each concurrent index has its own migration and interruption-cleanup registration.

## 3. Contracts

One ordinary parent occupies its budget-root identity for life. Member rows are immutable attribution; reparenting, project changes, retries and deletion do not refund or transfer spending. Source deletion retains minimal economic IDs/tombstones. New children bind inside their creation transaction. Quick-create execution attributes to the stored context parent; each newly created issue binds to its actual parent. A different root emits an attribution difference using the server-verified task credential, never a caller-provided origin/header.

Issue mutations use the workspace `iteration:<ws>` fence. Giving an existing issue its first binding takes the exclusive workspace budget binding lock; a newly created child binds before its transaction becomes visible and needs no binding lock. Claim finalization tries the shared side and retries a busy binding through the existing claim path. Budget root locks are last: no blocking issue/task lock after them. Binding/finalize writes require READ COMMITTED. Management opens READ COMMITTED and rechecks current owner/admin membership before replay or writes. Read projections own REPEATABLE READ READ ONLY transactions and do not publish cache state. A writer may cache the already-committed prefix after loading it under the root lock; its newly appended entries are published only after successful commit.

USD ticks are signed-int64-safe nonnegative integers at public boundaries (`1 tick = 10^-10 USD`), serialized as canonical decimal strings. Actual, estimated, reserved and unknown-hold remain separate. Estimates never become actual or consume reservations. Unknown or no configured limit means `remaining_ticks=null`; unavailable ledgers expose null amounts, never stored unverifiable projections. Rollups and legacy `task_usage`/hourly data do not add budget consumption.

The v1 usage batch is immutable and one-per-attempt. Raw-byte SHA-256 binds original batch key; retries preserve bytes and key. Decode rejects duplicate/unknown/case-alias keys, invalid UTF-8/control characters, missing/null required fields, noncanonical counters and trailing data. Limits: 256KiB, 32 invocations, 256 usage items. Root lock serializes batch, claim and management writes. Prior-key replay precedes mode/ledger checks. Off mode records no new usage operation; unavailable ledger returns retryable 503. No production correction or settlement endpoint exists. Lifecycle completion/cancellation/deletion/loss moves reservations to unknown hold without releasing them.

Production adapter evidence currently contains exactly one row, `claude` / normalizer `1`: fresh usage has proven invocation scope, resumed usage does not, and cost is not proven provider-reported. The current Claude adapter supplies no cost; a future supplied amount under this row remains an estimate. Resumed/custom/unknown adapters remain ambiguous unless separately proven. Scope and cost are independent: quarantined usage retains its cost classification and any snapshot but contributes no amount. Production S1 therefore adds no actual spend; only proven fresh Claude usage contributes estimates. Model-based estimates freeze exact rational price snapshots on each entry; readers replay stored evidence instead of repricing history. `Evidence` and `Prices` injection is internal/test configuration; HTTP always constructs the production evidence table and metrics price source, and the strict wire rejects caller-supplied evidence or prices.

The root totals exclude server-initiated model calls (recommendations, description assistance, chat titles and collaboration knowledge automation). These have no attributed daemon task and are not an additional item in the frozen three-entry `not_applicable` list.

Management admits verified human JWT/PAT only, no agent/task hints, and current workspace owner/admin. Preserve deny-only hints before Auth sanitizes headers. Gate machine credentials before the generic capability guard; normalize workspace membership denials before they reach budget handlers. All budget responses carry `Cache-Control: no-store`, including auth failures. Management-handler errors contain fixed codes; authentication retains its own error body, and daemon errors follow the usage protocol. Core logs fixed text for budget errors. Deleted root reads contain no issue title/body.

Daemon journal is initialized after exclusive profile startup and before advertising capability. Persist invocation intent before provider launch; observe execution even when persistence fails. Seal and retry the original batch bytes; cancellation of telemetry does not change task outcome. Recovery never touches running attempts, never redirects reports, and sends only to the recorded server/workspace. Retention is seven days: if expiration wins before a late direct send, it can remove the sealed local record with a warning; server hold remains unknown. Journal/account-profile changes may lose telemetry but cannot prove settlement.

### Deployment and retained-history rollback

The database deployment must use `default_transaction_isolation=read committed`.
Budget-owned management transactions declare that level explicitly. Existing
issue writers retain their opening behavior; the member-present assertion does
not detect every first-binding or deletion race under a different default.
Verify the setting in the application's database/role session before enabling a
root. A wrong default can silently miss a binding or tombstone and can cause
repeated claim requeues. This feature does not add a server-wide startup check.

Enable roots only after every server instance has the budget seams. An older
instance can create/move a child without binding it or delete a member without
its tombstone. Rolling back to such a binary reopens both gaps. Stop observation
with the human `set-mode off` operation, retaining all economic history; do not
use destructive down migrations as the operational rollback. The down migrations
refuse when any budget table is populated, before removing indexes and again
before dropping tables. F3 migrations are 869–885 in this delivery; recheck
number allocation when integrating another branch.

After an upgrade or rollback, run both read-only detectors below in the target
database. Both should return no rows for workspaces that still exist. The second
query excludes deleted workspaces because workspace teardown deliberately retains
their budget audit rows without updating them. These queries identify gaps, never
repair or reassign historical spending; the first does not enumerate descendants
below the first missing binding.

```sql
SELECT c.workspace_id, c.id AS issue_id, c.parent_issue_id, pm.budget_root_id
FROM issue_budget_member pm
JOIN issue c ON c.workspace_id = pm.workspace_id AND c.parent_issue_id = pm.issue_id
LEFT JOIN issue_budget_member cm ON cm.workspace_id = c.workspace_id AND cm.issue_id = c.id
WHERE cm.issue_id IS NULL;

SELECT m.workspace_id, m.issue_id, m.budget_root_id, m.bind_source
FROM issue_budget_member m
JOIN workspace w ON w.id = m.workspace_id
LEFT JOIN issue i ON i.workspace_id = m.workspace_id AND i.id = m.issue_id
WHERE m.tombstoned_at IS NULL AND i.id IS NULL;
```

Quick-create's in-flight scan relies on both parent-capable entry points (ordinary
quick-create and agent-mode comment-to-child) verifying the agent in the context
parent's workspace. Preserve those checks when adding another entry point.

## 4. Validation and error matrix

| Input/state | Result |
| --- | --- |
| Management task/cloud credentials, ordinary member or missing member | 403 `budget_forbidden`, no budget-table access |
| Daemon credential on a human management route | 401 from ordinary authentication, with `no-store` |
| Foreign/missing root | Identical 404 `not_found` |
| Stale policy revision or original key changed intent | 409; no substitution/retry with a new key |
| Committed original operation | Same receipt; query marks stale against current policy |
| Original operation not visible | `unknown` plus original retry path |
| Malformed usage / body too large | 400 / 413; entire batch rejected |
| Duplicate usage bytes/key | Original receipts, exactly one committed batch |
| Second usage key | 409 `batch_conflict` |
| Ledger unreadable | Daemon 503 retry; human null-valued unavailable projection |
| Cancel/timeout/process exit without complete evidence | Unknown hold; no refund |
| Off then observe | Persist membership; one `observation_off` gap per resumed interval |

## 5. Good / base / bad cases

Good: reserve100, actual30, loss of contact -> A30/R0/H70, remaining unknown. Base: never-enabled issue tree writes no budget rows. Bad: fold an estimate into actual, release a reservation on task deletion, switch roots after a retry, or treat a missing original receipt as permission to generate another key.

## 6. Required tests

- `internal/budget`: integer boundaries, replay, cumulative/delta/correction, hold/settlement evidence and frozen pricing.
- Service `TestIssueBudget*`: binding concurrency/lock ordering, finalize rollback/legacy capability, lifecycle, atomic usage/recovery, off-mode interleavings, verified quick-create attribution, membership recheck and consistent read snapshots.
- Handler `TestIssueBudget*`, `TestDaemonBudgetUsage*`, claim-budget suite: real writers, human credential matrix, old daemon, missing/transient authorization and legacy usage independence.
- `cmd/server/issue_budget_integration_test.go`: full auth/no-store gates, original receipts, actual issue deletion/tombstones and claim-to-usage-to-show.
- Daemon `TestBudgetUsage*`: intent durability, immutable original-byte recovery, concurrent create/recovery, transport timeout, redirects, response truncation, server identity and retention order.
- CLI `TestIssueBudget*` plus Core `issue-budget-schemas.test.ts`: human-only local commands, exact decimal conversion, unknown outcome recovery and malformed-response/log privacy.
- Preserve M1 completion and F2 snapshot named regressions. DB-backed acceptance uses fresh databases per package and normal/race run; skips are not evidence.

## 7. Wrong vs correct

Wrong: report `limit - actual` as spendable while an old invocation is unknown, or reconstruct attribution from the current issue tree.

Correct: use immutable member/attempt identities and replayed actual/reservation/hold state; expose unknown explicitly and retain the original operation key.

Known S1 bounds: lifecycle scans process up to500 roots with a five-second per-root deadline. A persistently unreadable/oversized root can remain open and delay later roots; occupancy remains retained. Snapshot/checkpoint support, fair recovery of these roots, account-bound journal retention and trustworthy settlement belong before S2 enforcement.
