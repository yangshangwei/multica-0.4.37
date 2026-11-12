# Collaboration scheduling and proof recovery

> 2026-10-26: ordinary host execution has a separate [implementation contract](./collaboration-host-execution.md) and default-closed admission. The Linux namespace guarantees below apply only to the retained legacy branch; do not apply them to host execution. Platform/account acceptance remains tracked by the active runtime task.

## 1. Scope / Trigger

Apply when changing runtime polling, candidate pagination, retained proof
recovery, runtime deletion, or their transaction boundaries. Listing work must
give an eligible successor an actual attempt when an earlier object keeps
failing. Scanning does not establish permission, capacity, or a successful run.

## 2. Signatures

- `GET /api/collaboration/runtime/runs?runtime_id=<uuid>` retains the existing
  `{run_ids: string[]}` response. Installed daemon decoders reject unknown fields.
- `collaborationRuntimeScanPage` owns a short runtime/cursor transaction.
  `collaboration_runtime_scan_cursor` is keyed by
  `(runtime_id, binding_id, scan_kind)`; migration 730 creates the table and 731
  creates its unique index concurrently.
- Waiting/wake pages contain at most 16 objects. A candidate page contains at
  most **one** run, matching `collaborationPoller.dispatch` consumption.
- `Ledger.BeginProofRecovery()`, `ProofRecovery.Next()` and `Close()` coordinate
  a bounded round through `.collaboration-proof-recovery.cursor` and `.lock`
  in the shared installation ledger directory.

## 3. Contracts

Acquire `LockAgentRuntime`, re-authorize the daemon under that row lock, then
ensure/lock the scan cursor. Keep the order runtime → cursor. Commit the scan
position and release both locks before any wake or promotion transaction. The
same runtime fence already precedes runtime/workspace teardown; without it, a
previously authorized scan can insert a cursor after deletion commits.

Waiting uses `(issue_id, target_id)`; wake uses `(group.created_at, group.id)`;
candidates use `(run.created_at, run.id)` within one current priority tier.
Wrap at most once per page. Only offered rows advance the cursor; an empty page
caused by a changed priority must preserve the old tier's position.

Select the highest eligible tier over the full runtime/binding scope before
rotation: reserved, reserved continuation, unreserved, unreserved continuation.
Recheck the highest tier in the page-query snapshot after waiting for the cursor
lock. A lower tier never fills an empty stale priority page. Pending derived-run
existence uses the complete eligibility predicate, independently of page size.

The server advances one candidate opportunity per List because the daemon
attempts one run. Advancing a 16-item page while attempting its first item can
cycle between 1 and 17 in pages `[1..16]` and `[17,18]`, never attempting 18.
Increasing candidate page size requires a matching consumption contract and
composed regression; a second local last-ID cursor does not repair this mismatch.

Only named object refusals, invalid-object-ledger errors and exhausted
rolled-back object contention are isolated. Preserve global daemon identity,
request cancellation, unclassified DB errors and uncertain commit failures.
Log only object IDs and fixed error codes. A cursor is never an operation receipt
or permission to replay an uncertain mutation.

Proof recovery validates the entire ledger inventory before even accepting a
busy shared recovery lock. Its versioned private cursor stores the last attempted
SHA-256 identity key; validate file ownership, permissions, links, size and JSON.
Persist and fsync that position **before** taking the run lock or making a proof
request. HTTP failure and `ErrBusy` both consume an attempt and advance position.
Keep these sidecars outside the `.json` run inventory.

At most eight proof attempts share one ten-second context deadline. Retained
pre-burn recovery keeps that context through sealing and proof upload; it must
not call the foreground abandonment path that deliberately detaches shutdown
cancellation. Original receipt, proof bytes and attempt nonce remain unchanged.
An unknown run still occupies its machine slot and update barrier. Recovery
neither invents proof nor resumes a queue.

Runtime/workspace deletion removes matching scan rows in the existing atomic
delete statement, under the runtime fence. Retained business runs, reservations
and termination evidence keep their existing lifetime.

## 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Persistent object-specific waiting/wake failure | Later eligible object gets a bounded scanning opportunity |
| Failed first candidate | Next List advances one actual attempt opportunity |
| Reservation appears while scan waits | Stale lower tier returns no candidate; its prior position survives |
| Runtime deleted or binding changed during wait | Locked re-authorization refuses; no orphan cursor |
| Unknown commit or global identity/DB/context error | Propagate; never classify as a successful/durable object refusal |
| First eight proofs fail or have peer-held run locks | Later proof still gets a turn after restart |
| Corrupt ledger/cursor or failed cursor persistence | Fail closed; preserve capacity and launch barriers |
| Proof ACK arrives after Stop | Settle original run only; explicit Resume remains required |

## 5. Good / Base / Bad Cases

- Good: seventeen persistent Claim refusals are followed by a real eighteenth
  Claim/Permit, with reconstructed pollers between each one-item response.
- Base: old clients keep consuming the original `run_ids` response; no metadata
  negotiation or ordinary-task fallback is necessary.
- Bad: proving that all IDs eventually appear in pages while never checking
  which IDs the daemon actually attempts.

## 6. Tests Required

- `collaboration_runtime_scan_test.go`: failed waiting/wake prefixes, real
  List→Claim progress, full-scope derived checks, priority before/after cursor
  locks, preserved empty-page position, shared pollers and uncertain commits.
- `collaboration_runtime_scan_delete_test.go`: actual runtime/workspace deletion
  in both orders, plus binding changes while the scan waits for its row fence.
- Existing FIFO cancellation tests must pause the transaction that reached
  `LockCollaborationIssue`, not every Commit: preliminary cursor transactions
  intentionally do not own the source lock.
- Hold rollback and lost-reply tests select the transaction that executed
  `SetCollaborationTargetHold`. A first-Commit injector now hits cursor metadata
  and cannot prove business rollback or uncertain-hold recovery. Require one
  selected pre-commit rejection with an unchanged business snapshot, or one
  committed hold/audit with the original waiting input and no run/reservation;
  a subsequent poll must preserve that exact state.
- `collaboration_candidates_test.go`: HTTP Claim/Permit over eighteen single-item
  pages, restart, legacy list order and unknown-launch capacity retention.
- `collaboration_recovery_test.go` / `collaborationrun/recovery_test.go`: nine
  proofs, real cross-process locks, lost ACK, shared deadline, all pre-burn
  phases, malformed files and unchanged authoritative identities/barriers.

## 7. Wrong vs Correct

Wrong: `LIMIT 16` advances past sixteen candidates while one dispatch consumes
only the first, or each retained proof creates its own detached timeout.

Correct: persist one candidate opportunity per dispatch and keep one caller-bound
deadline through the complete bounded proof round. Preserve every unconfirmed
execution until its original termination protocol supplies proof.
