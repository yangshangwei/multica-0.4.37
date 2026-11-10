# Independent planning recheck — 2026-11-10

Reviewer: native `trellis-check` / `review_qa_fix`.
Source baseline: `b10da8fec9dab057f9830bd336369d42e27b8c85`.
Scope: revised planning artifacts and source inspection only. No product edits, database, filesystem adapter, model, service or deployment execution. The original `plan-review.md` is unchanged.

## Verdict

**REVISE before activating the complete execution child.** The contradictory creation/claim/status descriptions are resolved, and the pure canonical-spec/packager portion is concrete enough to implement. Two remaining integration contracts must be recorded before the whole child is called planning-ready: exact authority/deletion locking and retention of the human approval decision across existing workspace teardown.

These are bounded source-backed follow-ups, not a request to redesign the adapter or add another approval flow. No implementation or runnable permit is established by this review.

## Original blocker coverage

| Original blocker | Recheck |
| --- | --- |
| 1. Freeze before approval | Addressed: pending approval and unclaimed row are created atomically; canonical bytes, digest domains, limits, idempotency and DB expiry are explicit. Digest-bound human decision cannot replace scope. |
| 2. Separate approval/execution states | Core addressed: claim updates the existing row to unknown before invoking; one definite commit owner invokes; success and legacy executed commit together; bound cancel/self-report are guarded. Workspace-teardown interaction remains B2 below. |
| 3. Current authority/provenance | Verified task context, current scope, Operator ceiling and declared-revision limitation are specified. Concrete lock plan remains B1 below. |
| 4. Filesystem boundary | Addressed at design level: trusted operator roots, descriptor-relative no-follow opening, bounded verified snapshot, deterministic USTAR, exclusive publication and uncertain cleanup preserved as unknown. Unsupported platforms fail closed. |
| 5. Migration/index safety | Addressed: separate concurrent unique indexes; a highest-version down fence refuses retained rows before any index drop; final table drop repeats NOWAIT/empty check in one DO block; multi-file rollback remains maintenance-only. |
| 6. Restricted reads/recovery | Core addressed: broad queue contains bounded safe summaries, restricted detail exposes the spec, no invented auditor role, no rerun after unknown. Retained human decision facts still need B2. |

All ten original positive/negative cases are represented in `implement.md:51`–`60`. The concurrency and retention cases below must be added to that matrix; no listed test has yet been executed for this child.

## B1 — Replace the unverified lock-order placeholder

`design.md:67` invokes “the repository's existing principal/task/issue lock order” and explicitly postpones comparing it with cancellation and workspace deletion until before activation. `implement.md:8` leaves that comparison unchecked. Source does not provide the named universal order:

- `server/internal/handler/task_capability.go:107` locks the live token, then joined principals, then invocation targets.
- `server/pkg/db/queries/task_token.sql:16` uses a joined task/agent/runtime/issue/member/user query with `NOWAIT`, explicitly to avoid adding a lock-order wait; it does not establish a generic sequence of individually locked principal/task/issue rows.
- `server/internal/handler/workspace.go:1146` takes the workspace deletion fence first, then deletes tasks, tokens, approvals and other rows.

Current reads inside a transaction alone do not prove that revocation/deletion cannot cross the admission write. Human initiator/approver membership and account rows may differ from the task originator/runtime-owner rows covered by the existing helper.

Required planning closure:

1. Record the actual lock graph for human create/decision/claim, task-token claim, cancellation and nonce-based finalization. Include the workspace fence and each authority row that can independently revoke the operation; distinguish unclaimed admission from postclaim internal result recording.
2. Preserve the existing `NOWAIT` conflict behavior or prove another consistent order. Define rollback/refusal on conflict; never retry an uncertain commit or invoke after it.
3. Add two-connection cases for membership/password or task-token revocation racing claim, workspace deletion racing create/claim, and cancellation racing claim. Assert the linearized result and physical invocation count, not only HTTP status.

A suitable narrow direction is a workspace existence/deletion fence before creating or admitting a controlled row, followed by the documented live token/current-principal locks and approval/execution locks, with NOWAIT on cross-path row acquisition. The implementer must verify the exact SQL order rather than treating this direction as already proven.

## B2 — Preserve the decision when workspace deletion removes legacy approvals

`server/pkg/db/queries/workspace_delete.sql:344` unconditionally deletes `agent_approval_request` rows for the workspace. `server/migrations/452_agent_approval_request.up.sql` confirms that this is deliberate application cleanup, with no foreign key retaining the row.

The new design keeps execution rows when referenced entities disappear (`design.md:125`), but its stored fields (`design.md:71`–`75`) do not explicitly retain the human `decided_by`, `decided_at` and decision record. The human decision remains in the legacy row. Success finalization also requires changing that same row to executed (`design.md:95`). Consequently, current teardown can erase the approval fact and make an already admitted execution unable to atomically finalize. “Frozen identities are retained” is not an executable resolution of this path.

Required planning closure:

- Choose and state one concrete handling of **controlled** approval rows in workspace teardown. The minimal consistent direction is to retain bound approval rows alongside execution receipts while leaving unbound approval cleanup unchanged; missing workspace authority still denies public reads/admission. This preserves the existing atomic success contract and avoids inventing a second approval history store.
- Add the necessary `workspace_delete.sql`/handler integration to expected files and the compatibility matrix, with the B1 fence preventing a late controlled creation from escaping deletion's observation.
- Test deletion before claim (zero invocation), deletion after admitted claim (retained approval/claim facts and explicitly defined result handling), deletion after success/failure/unknown (no security-evidence purge), and unaffected legacy cleanup. Do not introduce FK/cascade or automatically delete output to unblock teardown.

Copying immutable decision facts into another retained record is an alternative, but it would also need to resolve the existing “success updates both rows or neither” contract when the approval row disappears. Do not silently select that alternative during implementation.

## Confirmed source and compatibility details

- `canManageAgent` admits current workspace owner/admin or agent owner; the plan correctly does not invent a separate auditor role.
- Existing approval queue/get responses expose the plan to ordinary members. Restricted detail plus a safe generated legacy plan is necessary and is now specified.
- The legacy execution endpoint accepts self-report only above the Operator ceiling; the plan correctly retains unbound behavior while rejecting controlled self-report.
- Claim ambiguity means zero adapter invocation; receipt ambiguity means caller-visible unknown with read-only recovery of an actually committed original receipt. No second run follows from expiry, a file, or replay.
- Migration runner files are deliberately executed outside one transaction. The new highest down fence plus maintenance requirement correctly addresses ordering without pretending its lock covers later files.

## Verification and reviewed identities

Planning/source inspection only. Product lint, typecheck, tests, migrations and adapter execution were not run and are not claimed to pass. No artifact beyond this review was changed.

| File | SHA-256 reviewed |
| --- | --- |
| `prd.md` | `be007e1e2461dec836de1bfab616a102eabf21fd66ca6011cd4079ea11fc5ba6` |
| `design.md` | `e1606f14cf3566ee1067785c8cfb4903c7ea1175d65abb753a53f7a5c923fece` |
| `implement.md` | `e1208e9bff04827bf86b57373cbbff214101691220aa729df3e825f840ec86f7` |
| Original `plan-review.md` | `b004e7b94fca155ae91f1f70ab6ff69715ec8b8af679fc7197456bc840d08ba4` |

## Second pass — 2026-11-10

**Latest verdict: PLANNING CONTRACTS READY.** B1 and B2 are closed in the revised design and execution plan. The earlier REVISE verdict above remains the historical result for its recorded hashes. No remaining blocking planning finding was identified in this second pass.

This verdict permits the main session to use the reviewed artifacts for its normal implementation activation. It does not mean the controlled executor exists, that a permit is runnable, that the matrix passed, or that this child/M1 is complete. Main retains task-status ownership.

### B1 closure

The design now states an explicit graph instead of assuming a universal repository order:

- Workspace KEY SHARE NOWAIT fences creation/admission against the existing workspace FOR UPDATE deletion path.
- Verified task-token, joined subject graph, invocation-target, independent human account/membership/credential and applicable managed-binding locks are identified. Human initiating/approving/claiming principals are not silently reduced to the runtime owner or task originator.
- SHARE NOWAIT is required for role/account writes; the existing KEY SHARE membership helper and waiting PAT helper are explicitly excluded as unchanged substitutes. Current facts and the original verified credential version are rechecked under the transaction.
- Cross-path inversions fail with whole-transaction rollback; implicit waits are bounded. Ambiguous commit never causes retry or adapter invocation. Finalization contention leaves unknown rather than scheduling another run or result retry.
- Cancellation has its own current-manager contract. Internal result recording locks only retained approval then execution and checks the original nonce/spec/state; it does not reacquire deleted workspace/task/agent/principal rows or grant fresh authority.

Source checks confirm the referenced mechanisms: `collaboration_authority.sql:54` is workspace KEY SHARE NOWAIT; `task_token.sql:16` is the joined NOWAIT subject graph; invocation-target writers lock the agent including empty-set insertion; password revocation operates under the user-row lock; task cancellation retains chat-session-before-task ordering. `LockKnowledgePAT` currently waits, matching the plan's requirement for a separate NOWAIT equivalent on this path. The installation namespace and binding locks are existing primitives, not an invented parallel lock namespace.

Implementation cases C1–C4 now require both actual commit orders, original credential/version checks and physical invocation counts for independent-principal revoke, token/password/PAT revoke, workspace deletion and cancellation. Those tests remain to be written and run.

### B2 closure

Workspace teardown now explicitly retains an approval whenever an execution row has its `approval_id`, regardless of result state or a malformed execution workspace field. The original human decision fields therefore survive with the canonical execution record. Unbound approval cleanup remains unchanged; neither output files nor execution receipts enter a teardown delete arm.

The existing workspace FOR UPDATE fence and new creation KEY SHARE fence establish the needed ordering: a controlled create commits before the deletion statement observes its binding, or refuses without inserting. This is compatible with the current READ COMMITTED teardown and introduces no FK/cascade or second decision-history store.

After workspace deletion, former owners and all other public callers fail live workspace authority. The only continuing operation is the already admitted local packager's original owner: nonce-only internal finalization may atomically persist success and the retained legacy executed marker, without recreating a workspace, emitting a deleted-workspace event, or invoking again. A lost owner or uncertain operation remains unknown. Cases D1–D4 explicitly cover deletion before claim, after definite claim, after terminal/unknown results and unaffected legacy cleanup.

### Second-pass verification boundaries and identities

Read the latest design/implementation artifacts and checked the relevant existing SQL, handler, auth and cancellation code. Reconfirmed that the original six blockers and ten-case matrix remain represented, with C1–C4/D1–D4 added. The highest-version migration down fence still precedes concurrent index deletion and does not replace maintenance mode.

Only this review file was appended. No product source, task state, database or service was changed. No product lint/typecheck/test, migration or adapter execution was run in this planning review. Actual SQL/concurrency behavior must still be demonstrated during implementation.

| File | SHA-256 reviewed in second pass |
| --- | --- |
| `prd.md` | `be007e1e2461dec836de1bfab616a102eabf21fd66ca6011cd4079ea11fc5ba6` |
| `design.md` | `17968351f9d37e57d2fb357c6e31f7e7e7d144877cbed14d6b30aa5451e27d06` |
| `implement.md` | `537059cd0367236eb5a8433ab09ee215fa210b8b98fc3c836bec13a487245d46` |
| Original `plan-review.md` | `b004e7b94fca155ae91f1f70ab6ff69715ec8b8af679fc7197456bc840d08ba4` |
