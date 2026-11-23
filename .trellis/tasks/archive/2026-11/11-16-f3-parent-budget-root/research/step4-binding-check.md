# Step 4 independent check: budget root and member binding

Date: 2026-11-18. Checker: the `trellis-check` agent for step 4 of
`.trellis/tasks/11-16-f3-parent-budget-root` (design sections 5, 8, 9.1 to 9.5).

Product code under review: worktree `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116`,
branch `codex/f3-budget-root-20261116`, HEAD `86fea651cc5d09360327b6cf0f8357c1f360070d`,
everything uncommitted. I did not touch `server/internal/budget/` or migrations 866 to 880.
All paths below are relative to that worktree's `server/` unless they start with `/`.

**What "the tree" means here:** the worktree as it stood at 13:59:00 on 2026-11-18 (hashes in
section 16). You replaced `internal/budget/` at 12:17:40, before that time, and step 5b began to
change step 4 files at 14:48:49, after it. Section 18 lists what I did after your rulings.

## 0. Verdict

**Step 4 passes, with one low-severity product fix. The open points of section 0.2 have all been
ruled since; one new point is open (section 18, R4).**

- Every exit row (B01 to B13, B57, B59, B60, the service half of B14, B56 as updated) is now pinned
  by a test that went red under a mutation of the product code, with two exceptions that cannot be
  pinned at step 4: the binding half of B07 and B08 is vacuous until step 5b adds attempts
  (section 1).
- Seam completeness holds: no statement, Go caller, raw SQL string, trigger, rule or database
  function creates an issue, writes `parent_issue_id` or deletes an issue outside the inventory
  (section 5). A new static guard now fails when that stops being true.
- No path that reaches a binding seam can run under REPEATABLE READ (section 12). Most of them are
  READ COMMITTED only because the server default says so; an experiment shows what happens when it
  does not (section 12.3).
- No issue writer locks or writes an `issue_budget_root` row; `Enable` follows the order of 9.2;
  the management operations lock only the root (section 9).

### 0.1 What I changed

| # | File | Change |
| --- | --- | --- |
| 1 | `internal/service/issue_budget.go:557-561` | The only product change. A negative `ExpectedPolicyRevision` now answers the refusal `invalid_request`; it used to return a plain error from the digest encoder (defect D1). |
| 2 | `internal/service/issue_budget_test.go` | 8 new tests, 1 new subtest, a hang fix, two raw INSERTs replaced by `fx.Insert`, one extra cross-workspace call (section 3). |
| 3 | `internal/handler/issue_budget_binding_test.go:218-228` | Three updates that keep the parent, so that the "only when the parent changes" guard has a red run. |
| 4 | `internal/handler/issue_budget_concurrency_test.go` (new) | The concurrency and deadlock stress test asked for in item 5. |
| 5 | `internal/service/issue_budget_writers_test.go` (new) | A static inventory guard for item 1: fails on a new issue writer, on a removed seam, and on raw issue-writing SQL in Go. No database. |

**The two new files are mine; neither is in the implementation report.** The untracked
`internal/service/issue_budget_writers_test.go` you asked about (9,401 bytes, written 2026-11-18
05:12:24) is change 5. What it pins, why it had to be added and what it cannot see are in
section 3; its 18 red runs are in 4.5. `internal/handler/issue_budget_concurrency_test.go`
(11,049 bytes, 01:12:01) is change 4. Both are yours to keep or delete: nothing else depends on
either file.

### 0.2 Needs your decision

**Status on 2026-11-18, 15:20: you have ruled on all eleven points** (`design.md`, `implement.md`;
checked in 18.1). They stay below as I wrote them, for the record. What is open now is in 18.2:
R4, a reading that bears on the delay decision you kept for 5b, and the nuance at the end of R3.

1. **Isolation by default only** (N1, section 12). `Enable`, the four management operations and
   seven of the eight transactions that reach a seam (six `Begin` sites) are READ COMMITTED only
   through `default_transaction_isolation`. Under a `repeatable read` default the `Enable` /
   child-create race leaves an unbound child with no error, and a same-key retry of a management
   operation that waited for the root lock fails with `40001` (both reproduced, 12.3). The 5b
   runtime assertion sits in the member-row branch and covers neither `Enable` nor `manage`.
   Either state the level in `transact` or widen the 5b assertion.
2. **Mixed-version window** (N2). An instance without the seams can create a child under an
   enabled parent during a rolling deploy. Design section 6 does not mention it.
3. **`invalid_request` for a negative revision** (D1). My choice, by analogy with the negative
   limit. `policy_revision_conflict` is the alternative.
4. **Tests that later steps must change**, missing from design 10.13: the exact `wantEnable` list
   (5b reorder), the stress test's "0 observations" (5c), and all five statement lists of
   `…TransactionsFollowTheLockOrder` once step 7 adds the in-transaction membership read
   (section 13.2).
5. **Digest golden vector** (N6): none exists; 10.1 changes the digest in 5b. Pin one before step 7
   hands keys to callers.
6. **Index proposal** (section 8.4): not needed at the sizes measured; recorded, not added.
7. **The static guard** (change 5) is an addition you did not ask for. It is one self-contained
   file; delete it if you do not want an executable inventory.
8. **Design section 5 rule 5 lists three REPEATABLE READ write transactions; there are four**
   (N18, section 12.2). The fourth, `internal/service/collaboration_stage_evidence_read.go:21`,
   reaches no seam. The list in the rule is yours to complete.
9. **Design 10.2, the supplement of 2026-11-18, names one entry that puts a parent into a
   quick-create task; there are two** (N19). The second,
   `internal/handler/source_context.go:887`, runs the same two agent checks, so the premise of the
   rule holds. The sentence, and the step 8 spec it announces, should name both.
10. **One database is kept, on purpose**: `f3_budget_s4c_plan`, the 60,023-row one, because you
    measured the 5b form of the in-flight scan on it after my section 8 (design 10.2). The other
    scratch databases are dropped. The drop command for this one is in section 16.
11. **The reachability tool** of 12.2 lives only in `/tmp/f3-s4c/cg/`. Say so if you want it kept
    with the task. (Since: you copied it to `research/step4-reachability/`; the three source files
    are byte-identical to mine, 18.1.)

## 1. Verdict per exit row

"Pinned" means: at least one mutation of the product code that breaks the row's behaviour made the
named test fail (section 4). S = `internal/service/issue_budget_test.go`,
H = `internal/handler/issue_budget_binding_test.go`; every test name starts with `TestIssueBudget`
unless written in full. "(new)" marks a test I added.

| Row | What step 4 owes | Test that fails when it is wrong | Mutants that turned it red | Verdict |
| --- | --- | --- | --- | --- |
| B01 | Two concurrent enables of one issue leave one root; the other gets the replay or a conflict | S `…EnableIsOnePerIssueAndReplaysByOriginalKey`, S `…EnableConcurrentlyOnOneIssue` | `enable-no-fence`, `enable-no-slot-check`, `enable-replay-ignored`, `replay-conflict-ignored` | Pinned |
| B02 | Parent and child enabled concurrently: the later one gets `root_conflict`, no issue in two roots | S `…EnableParentAndChildConcurrently`, S `…EnableRefusesOverlappingRoots`; the contested trees of the stress test | `enable-no-descendant-conflict`, `enable-no-own-member-check` | Pinned |
| B03 | The identity slot is taken for life; reopen is the same root with a higher policy revision | S `…CloseAndReopenKeepTheSameRoot`, S `…DeletedRootIssueKeepsTheSlot`, H `…DeleteRootIssueTombstonesTheRoot` | `reopen-deleted-allowed`, `policy-update-*`, `enable-issue-read-before-slot` | Pinned |
| B04 | A child and a grandchild each get a member row inside their own create transaction | S `…CreateInheritsTheParentRoot`, H `…EveryParentCapableWriterBinds/C1` | `seam-create-removed`, `create-bind-source-reparent`, `create-wrong-bound-parent` | Pinned |
| B05 | Enable racing a child create: the child ends with exactly one member row, in either order | S `…EnableRacingChildCreate` (3 subtests), the stress test | `create-no-fence`, `enable-no-fence`, `seam-create-removed` | Pinned (after the hang fix D2) |
| B06 | Enable freezes every descendant at once and records each in-flight task | S `…EnableFreezesDescendantsAndRecordsInFlightTasks`, S `…InFlightScanUsesTheContextParentOnlyWithoutAnIssue` (new) | `inflight-*`, `enable-no-inflight-scan`, `covered-*` | Pinned. The matrix calls the record `pre_activation_unknown`; the design and the code write `coverage_gap` / `in_flight_legacy`. The "lifetime total unknown" display is a read-side matter of later steps |
| B07 | Binding half only: a retry (new task id) stays in the same root | S `…TaskRetryAndReclaimLeaveTheBindingAlone` | none: no step 4 code runs on a retry | **Vacuous at step 4.** The real pin arrives with attempts in 5b |
| B08 | Binding half only: a reclaim (generation + 1) stays in the same root | same test | none | **Vacuous at step 4**, as B07 |
| B09 | A business reopen writes nothing to the budget tables | H `…BusinessReopenWritesNothing` | `absent-status-change-*` (3) | Pinned |
| B10 | Clearing the parent, moving under a rootless parent or changing project keeps the member row | H `…MoveKeepsTheMemberRow`, S `…ReparentNeverRewritesAMemberRow` | `absent-move-*` (3), `reparent-rule1-tombstones-own` | Pinned |
| B11 | A move under a parent of another root is not refused and records `would_refuse` / `root_conflict` | H `…MoveIntoAnotherRootIsRecordedNotRefused` | `reparent-rule1-*`, `seam-reparent-always` | Pinned |
| B12 | Deleting a member leaves a tombstone; its descendants stay in the root | H `…DeleteMemberLeavesATombstone`, H `…BatchDeleteTombstonesEveryMember`, `TestDeleteMemberIssueKeepsBudgetRows` | `seam-tombstone-removed`, `absent-delete-*` (7) | Pinned. Convergence of the reservation (B36, B37) is step 5 |
| B13 | Deleting the root issue tombstones the root, keeps members and ledger, and no surviving member can get another root | H `…DeleteRootIssueTombstonesTheRoot`, S `…DeletedRootIssueKeepsTheSlot`, S `…DeletedRootIssueRefusesOnlyReopen` (new) | `absent-delete-closes-root`, `admission-ignores-tombstone`, `reopen-deleted-allowed`, `reopen-open-check-before-deleted` | Pinned. `would_refuse` / `root_deleted` on later execution is step 5 |
| B14 | Service half: a lookup from another workspace finds nothing | S `…IsScopedToTheWorkspace` | `scope-lock-root`, `scope-get-root`, `scope-get-member`, `scope-root-by-issue`, `scope-tombstone-member`, `enable-cross-workspace-issue` | Pinned. The auth half is step 7 |
| B56 | Deleting a member issue tombstones its member row; every other budget row stays | `TestDeleteMemberIssueKeepsBudgetRows` | `absent-delete-removes-attempts`, `-removes-entries`, `-releases-reservation`, `-cancels-attempt`, `-tombstones-root-member`, `-removes-member-row` | Pinned |
| B57 | Children detached by a parent delete keep their member row and the original parent id | H `…DeleteRootIssueTombstonesTheRoot`, H `…EveryParentCapableWriterBinds/S4` | `absent-detach-tombstones-children`, `-deletes-child-members`, `-rewrites-bound-parent` | Pinned |
| B59 | The walk is not capped at 10 levels; a loop refuses the enable and writes nothing | S `…EnableWalksPastTenLevelsAndRefusesALoop` (subtest "a loop outranks a member below the issue" is new) | `walk-depth-limit-10`, `enable-cycle-not-refused`, `enable-conflict-outranks-cycle` | Pinned |
| B60 | The 5 parent-capable creators and the 4 parent writes each bind | H `…EveryParentCapableWriterBinds` (C1, C2, C4, S3 with the batch entry, S4, S6), S `…CollaborationWritersBindMembers` (C7, C8, S5), S `…CollaborationEditBindsOnlyWhenTheParentChanges` (new), S `…WriterInventoryIsComplete` (new, static) | `seam-create-removed`, `seam-reparent-handler-removed`, `seam-reparent-collab-removed`, `seam-tombstone-removed`, `absent-teardown-*` (4), 18 guard cases | Pinned |

Rows whose expected column also describes ledger amounts (A, R, H in B03, B07, B08, B10, B12, B13)
are checked here only for what step 4 writes: root, member, operation and observation rows.

## 2. Defects and notes

Severity as used here: **Medium** = wrong budget data is possible under a condition outside the
code (configuration, deploy order), or a missing test hid real behaviour; **Low** = wrong error
class or a test-only problem; **Note** = nothing to do at step 4. Nothing reached High (wrong budget
data on a product path today): `IssueBudgetService` has no production caller yet (N13).

### 2.1 Fixed

| ID | Severity | Where | What, and how to reproduce | Fix |
| --- | --- | --- | --- | --- |
| D1 | Low | `internal/service/issue_budget.go`, `manage` (the line that is now `:562`) | A negative `ExpectedPolicyRevision` reached the intent digest, where `budget.Count.MarshalJSON` rejects it. `Close`, `Reopen`, `SetLimit` and `SetMode` then returned a plain error, not a refusal; a handler would turn that into a 500. Reproduce: `Close` with `ExpectedPolicyRevision: -1`. Red run `negrev_red`: exit 1, 1 PASS / 1 FAIL: `close against revision -1: got encode budget intent: json: error calling MarshalJSON for type *budget.Count: budget: negative amount, want refusal "invalid_request"` | `:557-561` answer `invalid_request` before the transaction. Test S `…NegativePolicyRevisionIsAMalformedRequest`; green run exit 0, 2 / 0 / 0. Revision 0 still answers `policy_revision_conflict`. The refusal code is my choice (0.2 point 3) |
| D2 | Low (test) | `internal/service/issue_budget_test.go`, `…EnableRacingChildCreate/enable_commits_first` | A failed assertion left the gated `Enable` parked before its commit, holding a pooled connection, so closing the pool waited until the package timeout. Reproduce on the original test file: mutant `create-no-fence` ran into the 15-minute timeout with 7 passes and no `--- FAIL` line | `:819-831`: a cleanup registered after the fixture releases the gate, cancels the context and waits for both goroutines. The same mutant now fails all three subtests in seconds |
| D3 | Medium (test gap) | `issue_budget.go`, `issue_budget_binding.go`, `pkg/db/queries/issue_budget.sql` | 20 distinct mutants survived every suite as delivered: 19 in the service suite, and `seam-reparent-always`, which only the handler suite can see (list and the per-suite count of 22 in 4.2): the loop-over-conflict ranking, the quick-create half of the in-flight scan, the no-op limit, the tombstone-before-open order of reopen, three of the digest fields, the stored reason, six request-format checks, the workspace scope of the tombstone, the operator outside the digest, and both "only when the parent changes" guards | 8 new tests, 1 new subtest, 3 added calls (section 3). All 20 are killed now |
| D4 | Nit (rule 7) | `issue_budget_test.go`, original `:974-978` | Two open-coded `INSERT INTO issue_budget_attempt` in a new test | Replaced by `fx.Insert` (`:998-1007`) |
| D5 | Medium (test gap) | B60 | No delivered test is able to fail when a new issue writer, a second caller of an existing writer, or raw issue-writing SQL appears without a seam: the per-path assertions cover only the paths that exist today | `internal/service/issue_budget_writers_test.go`; 18 red cases (4.5) |
| D6 | Test gap | item 5 | No concurrency test crossed Enable, management, create, move and delete on one tree | `internal/handler/issue_budget_concurrency_test.go` (9.4) |

### 2.2 Not fixed

| ID | Severity | Where | What | Why it is left |
| --- | --- | --- | --- | --- |
| N1 | Medium, configuration-dependent | `internal/service/issue_budget.go:241-255` (`transact`), and the openers `internal/service/issue.go:278`, `internal/handler/lifecycle_handoff_transaction.go:249`, `internal/handler/triage.go:220`, `internal/service/collaboration.go:165`, `internal/handler/issue.go:3940`, `:4878` | These transactions name no isolation level. Under a `repeatable read` default, `Enable` racing a child create leaves the child with no member row and no error, and a same-key retry in `manage` that waited for the root lock fails with `40001` where 9.4 promises the replay (both reproduced, 12.3). A third silent form, measured after your ruling: a delete that waited for the fence while `Enable` bound the issue commits without the tombstone (run `rre_rr`, 12.3) | A level statement changes the exact statement lists of the lock-order test, and design rule 5 assigns the runtime assertion to 5b. Your decision (0.2 point 1). **Ruled 2026-11-18** (design section 5, supplement): `transact` states READ COMMITTED in 5b; the other openers keep the assertion; the server default becomes a deployment precondition. That ruling covers the `manage` retry. The delete form is a third residual case next to the two the supplement lists (section 18, R3) |
| N2 | Medium, deploy-dependent | design section 6 | During a rolling deploy an instance built before step 4 can create or move a child under a parent that a new instance has just enabled. The child gets no member row and no observation. B60's first wording asked for reconciliation of exactly this class (`coverage_gap` / `unfenced_writer`); the wording was dropped when the inventory was completed. Reproduce: old binary and new binary on one database, enable on the new one, `POST /api/issues` with the parent on the old one. **Added after your ruling:** the same old instance also deletes a member issue without the tombstone, and that is certain, not a race, because it has no seam at all; when the issue is the root issue the root is never marked deleted (5.4) | Not a code defect of step 4. Options, all mine: keep enabling behind a switch until every instance is new; or run the anti-join in 5.4 once after rollout; or accept and document. **Ruled 2026-11-18** (design section 6): no switch; a root is enabled only after every instance has the seams, and the 5.4 check runs once after an upgrade or a rollback. The check needs both queries of 5.4; the design names only the first (section 18, R2) |
| N3 | Low | `pkg/db/queries/issue_budget.sql:149-179` against `internal/service/task.go:7488` | The quick-create branch of the in-flight scan does not exclude tasks with a chat session or an autopilot run, which `parseQuickCreateContext` does. No product path writes `type = 'quick_create'` into such a task | No reachable case; changing the query belongs with 5b, which rewrites the scan order anyway. Ruled since I wrote this: design 10.2 (supplement of 2026-11-18) aligns the branch with `parseQuickCreateContext` in 5b |
| N4 | Low, unproven | `pkg/db/queries/issue_budget.sql:149-179`; `internal/service/task.go:1397`, `:1648` | The scan filters on `agent.workspace_id`. I did not prove that no queue row pairs an agent of one workspace with an issue of another: `CreateAgentTask` locks both owners' workspace rows but does not compare them, and two enqueue paths resolve the agent with plain `GetAgent`. If such a row existed the scan would miss it (a missing `coverage_gap`), never leak one | Outside step 4; still unproven (section 17). Ruled since I wrote this: design 10.2 (supplement of 2026-11-18) moves the issue branch of the scan, and the finalize lookup, to the issue's workspace in 5b. The quick-create branch keeps the agent join; N19 is about its premise |
| N5 | Note | S `…TaskRetryAndReclaimLeaveTheBindingAlone` | Vacuous at step 4: no step 4 code runs on a retry or a reclaim, so no mutant can turn it red | Becomes real with attempts in 5b |
| N6 | Note for 5b | `issue_budget.go:206-224` | The intent digest has no golden vector. 10.1 adds `attempt_reserve_ticks` to it, so every digest changes in 5b; nothing is stored in production yet | Pin one vector in 5b, before step 7 exposes keys |
| N7 | Note | `pkg/db/queries/issue_budget.sql:6`, `:36`, `:74`, `:83`; `issue_budget.go:283-284` | The root, attempt, operation and observation inserts bind a plain `@id` on `NOT NULL` columns without a default; the `pkg/dbid` package comment asks for `COALESCE(sqlc.narg('id')::uuid, gen_random_uuid())` with a default kept, and the guard list `v7Writes` does not include the budget tables. The operation id is stored inside the result JSON before the insert. An entropy failure yields NULL, the insert fails and the transaction rolls back: fail closed | Convention only; no inconsistent row is possible |
| N8 | Note | `issue_budget.go:497` and `rootState` | A root with no member row for its root issue yields a plain error, not a refusal | An integrity error no product path can produce |
| N9 | Note | `issue_budget_binding.go:128-140` | Rule 2 of 9.3 binds an unbound descendant that sits below a descendant of another root to the new parent's root. No test pins it. The state needs an unbound issue under a bound one, which only N2 or a fixture produces | Unreachable |
| N10 | Note | `issue_budget_binding.go:56-75` | A child created under a surviving member of a tombstoned root is bound to that root | Consistent with B13 |
| N11 | Note | `internal/handler/issue_budget_delete_test.go` | The test builds its attempts and entries through the `internal/budget` API, which another agent is changing | Re-run it after that package is copied over. Done: you copied the package at 12:17 and report 26 / 0 / 0 for the handler budget selection (`implement.md`); my own re-run on the tree with the new package at 13:58 gives the same, plain and with `-race` (15.5) |
| N12 | Note | `research/e2-writer-inventory.md:83`, `:86` | Stale line numbers: S4 is at `internal/handler/issue.go:4971` (not `:4964`), S7 at `:5028` (not `:5016`) | Task doc, not mine to edit |
| N13 | Note | `issue_budget.go` | `NewIssueBudgetService` is called only from tests. At step 4 no root can exist in production; each seam costs one indexed lookup or one empty UPDATE (section 14) | By design until step 7 |
| N14 | Note | `issue_budget.go:409`, `:415` | An issue that is both a member and inside a parent loop answers `root_conflict`: the own-member check precedes the walk. 9.2 ranks the loop only against conflicts the walk finds | Consistent with 9.2 |
| N15 | Note | `internal/handler/issue.go:3221`, `internal/service/issue.go:303-310`, `:395` | `POST /api/issues` takes the fence twice. Harmless, but it is why no HTTP-level test can kill `create-no-fence`; the service suite does | None needed |
| N16 | Note | `issue_budget.go:492` | `Reopen` reads the root issue's member row without a lock while a delete may be tombstoning it. Every interleaving equals the serial order "reopen, then delete": the delete never reads the admission state, and readers still report `tombstoned` | No defect |
| N17 | Note | `issue_budget.go:228-238` | A whitespace-only key is accepted (the reason is trimmed, the key is not). 9.2 asks for 1 to 200 bytes, and the step 7 HTTP layer accepts only canonical UUIDs | Consistent with the design |
| N18 | Note for the design | design section 5 rule 5; `internal/service/collaboration_stage_evidence_read.go:21` | Rule 5 names three REPEATABLE READ write transactions that must not call a binding function. A fourth statement sets REPEATABLE READ without READ ONLY: the stage evidence read, with three callers, all of them read views (12.2). PostgreSQL would accept a write there; only the call graph keeps it away from a seam, and it reaches none today | A fact in the design, not a rule; yours to complete. The 5b assertion covers it either way |
| N19 | Note for the design | design 10.2, supplement of 2026-11-18; `internal/handler/source_context.go:887` | The supplement keeps the agent join on the quick-create branch of the in-flight scan on the premise that a quick-create task with a parent has its agent in the parent's workspace, and it names the quick-create handler of `internal/handler/issue.go` (`:2813`) as the only entry that passes a parent. A second entry does: the agent mode of the comment sub-issue route passes `capture.SourceIssueID` as the parent (`source_context.go:887`). It checks the agent with `validateAssigneePair` (`:813`) and `GetAgentInWorkspace` (`:817`) against the workspace in which the source context was built (`:495`), so the premise holds there too. The Slack entry passes no parent (`internal/integrations/slack/slash_command.go:285`); the manual retry copies the context (8.1). The context does carry the `workspace_id` the supplement relies on (`internal/service/task.go:1575`, written at `:1678`) | A fact in the design, not a rule; yours to correct. The step 8 spec the supplement announces should name both entries |

## 3. What I changed, in detail

**Product, one file.** `internal/service/issue_budget.go`: five added lines and nothing else
(`diff` against the delivered file prints exactly `556a557,561`). Delivered sha256
`045d5c23622b91116601658981bd7f7a907ef96f7dbda7312584cc707ebc0ab6`, now
`7e89b859a0c0c46cba8f5aee14ec5367fbe7b24b96019e0494bfaca0999766f1`.

```go
	// A revision is a count: a negative one is malformed, not stale, and the
	// intent digest has no encoding for it.
	if p.ExpectedPolicyRevision < 0 {
		return IssueBudgetOperationResult{}, issueBudgetError(IssueBudgetInvalidRequest)
	}
```

**Tests.** `internal/service/issue_budget_test.go` grew from 22 to 30 top-level tests (485 lines
added, 14 replaced):

| Test (line) | Pins |
| --- | --- |
| `…InFlightScanUsesTheContextParentOnlyWithoutAnIssue` (`:1026`) | The quick-create half of the scan: found through the context parent in `waiting_local_directory` too; a task that has an issue is attributed to that issue only |
| `…EnableWalksPastTenLevelsAndRefusesALoop`, subtest "a loop outranks a member below the issue" | 9.2: a loop and a member conflict together answer `parent_cycle` |
| `…NegativePolicyRevisionIsAMalformedRequest` (`:1302`) | D1 |
| `…SetLimitToTheCurrentLimitIsRefused` (`:1433`) | 9.4: a no-op answers `state_conflict` and consumes no revision |
| `…DeletedRootIssueRefusesOnlyReopen` (`:1465`) | 9.4: a tombstoned root refuses only reopen, and `root_deleted` is checked before "already open" |
| `…OriginalKeyStandsForTheWholeIntent` (`:1506`) | The digest covers kind, mode, limit, revision and reason; the stored result carries the reason |
| `…OriginalKeyReplaysForAnotherOperator` (`:1568`) | The operator is not part of the intent (the code comment says so; nothing tested it) |
| `…MalformedRequestsSendNoStatement` (`:1604`) | 9.2: every format refusal happens before any statement, for all five operations |
| `…CollaborationEditBindsOnlyWhenTheParentChanges` (`:2067`) | 9.3: the collaboration edit reaches the seam only when the parent changed |

Smaller edits in the same file: the hang fix (D2), `fx.Insert` for the attempt rows (D4), a
cross-workspace `TombstoneIssueBudgetMember` call in `…IsScopedToTheWorkspace`, an optional column
override on the `quickCreateTask` fixture, and a transaction counter on the statement log.

`internal/handler/issue_budget_binding_test.go:218-228`: three updates that keep the parent (a
retitle, the same parent again together with a priority change, a status change) inside
`…MoveIntoAnotherRootIsRecordedNotRefused`, after which the rows of both roots must be unchanged.

**New files, both mine, neither in the implementation report.**

- `internal/handler/issue_budget_concurrency_test.go` (259 lines, 11,049 bytes), one test,
  `TestIssueBudgetConcurrentWritersKeepEveryTreeBound`. Item 5 asked for a deadlock stress test
  where one is meaningful. It crosses `Enable`, the management operations, create, move and
  delete on shared trees and then asserts the invariants (section 9.4). Before it, no test ran
  two different kinds of writer against one tree (D6).
- `internal/service/issue_budget_writers_test.go` (246 lines, 9,401 bytes), one test,
  `TestIssueBudgetWriterInventoryIsComplete`. This is the untracked file you asked about. It
  needs no database: it parses `pkg/db/queries/*.sql` and every production Go file of the module.
  - What it pins: the eight queries that create an issue, write `parent_issue_id` or delete an
    issue; their 12 production call sites, each with the seam its calling function must also call
    or the reason it needs none; that a creator without a seam passes no parent; and that no Go
    string literal inserts, deletes or re-parents an issue.
  - Why it had to be added: B60 is a claim about every writer, and the delivered tests assert it
    path by path, for the paths that exist today. A writer that does not exist yet has no such
    test: a new issue-writing query, a second caller of an existing one, a creator that starts
    passing a parent, or raw SQL would pass unnoticed (D5). The 18 cases of 4.5 show what the
    guard reports for each kind of change. I ran only the guard against those 18 changes, not the
    delivered suites; the four `seam-removed-*` cases among them are also caught by the per-path
    tests (the `seam-*` mutants of section 4).
  - What it cannot see is in 5.3. The list of call sites is written out, not derived, so a new
    writer shows up in review as an edit to that list.
  - Cost of keeping it: 0.35 s per run, and one line to edit whenever a call site of those eight
    queries is added or moves to another function. Deleting it changes no other result in this
    report.

## 4. Mutation results

### 4.1 Method

Mutants are built with `go test -overlay`: the mutated copy of a product file lives under `/tmp`,
the overlay maps the worktree path to it, and the worktree file is never written. "Restore and show
it is byte-identical" therefore reduces to the unchanged sha256 of every product file (section 16):
the only product file whose hash differs from the delivered one is `issue_budget.go`, by the fix
above. Each run used a database freshly cloned from a migrated empty template, `-count=1 -v`,
output to a file, exit code recorded. KILLED = at least one `--- FAIL`; SURVIVED = exit 0. An
identity mutant (no change) is the control and must survive.

128 distinct mutants plus the control; every one is killed by at least one suite now.

### 4.2 First pass, against the suites as delivered

| Suite | Runs | Control | Killed | Survived | Other |
| --- | --- | --- | --- | --- | --- |
| Service (`-run TestIssueBudget`) | 104 | 22 / 0 / 0 | 83 | 19 | `create-no-fence` hung to the 15-minute timeout (D2) |
| Handler (budget selection) | 38 | 25 / 0 / 0 | 34 | 3 | |

Survivors of the service suite, and the test that kills each now:

| Mutant | Behaviour that was unpinned | Killed now by |
| --- | --- | --- |
| `enable-conflict-outranks-cycle` | A loop outranks a member conflict (9.2) | the new subtest of `…EnableWalksPastTenLevelsAndRefusesALoop` |
| `inflight-qc-no-waiting` | A quick-create task in `waiting_local_directory` is in flight | `…InFlightScanUsesTheContextParentOnlyWithoutAnIssue` |
| `inflight-qc-linked-included` | A task with an issue is not also counted through its context | same |
| `setlimit-noop-allowed` | The same limit is refused | `…SetLimitToTheCurrentLimitIsRefused` |
| `reopen-open-check-before-deleted` | `root_deleted` before "already open" | `…DeletedRootIssueRefusesOnlyReopen` |
| `manage-digest-no-reason`, `-no-mode`, `-no-kind` | Digest fields | `…OriginalKeyStandsForTheWholeIntent` |
| `manage-result-without-reason` | The reason is stored in the result | same |
| `req-key-utf8-unchecked`, `req-key-nul-unchecked`, `req-reason-length-unchecked`, `req-reason-limit-2001`, `req-reason-utf8-unchecked`, `req-reason-nul-unchecked` | Request format (9.2) | `…MalformedRequestsSendNoStatement` |
| `scope-tombstone-member` | The tombstone is scoped to the workspace | `…IsScopedToTheWorkspace` (added call) |
| `manage-digest-includes-actor`, `enable-digest-includes-actor` | The operator is outside the digest | `…OriginalKeyReplaysForAnotherOperator` |
| `seam-reparent-collab-always` | Collaboration edit binds only on a parent change | `…CollaborationEditBindsOnlyWhenTheParentChanges` |

Survivors of the handler suite: `seam-reparent-always` (now killed there, 24 / 1, by
`…MoveIntoAnotherRootIsRecordedNotRefused`); `binding-locks-root-on-delete` and
`scope-tombstone-member` (invisible over HTTP; killed in the service suite by
`…BindingNeverTouchesARootRow` and `…IsScopedToTheWorkspace`).

How the two counts relate. Per suite, 19 + 3 = 22 runs survived. As distinct mutants that no
delivered suite killed, that is **20**: `scope-tombstone-member` survived in both suites and is
counted once, and `binding-locks-root-on-delete` was already killed by the delivered service suite
(`…BindingNeverTouchesARootRow` is the implementer's test), so it was never a gap. D3 is about
the 20.

### 4.3 Re-run against the new tests

Service control 29 / 0 / 0 at that point; 22 service mutants re-run, 22 killed: the 19 survivors
above, `create-no-fence` (by all three subtests of `…EnableRacingChildCreate`), `enable-no-fence`
(25 / 4) and `manage-takes-fence-after-root` (28 / 1, by `…TransactionsFollowTheLockOrder`). Handler
control 25 / 0 / 0; `seam-reparent-always` killed. D1's test has its own red and green run (2.1).

### 4.4 The stress test judged alone

`TestIssueBudgetConcurrentWritersKeepEveryTreeBound`, each line is independent runs on fresh
databases:

| Mutant | Runs | Result | Reading |
| --- | --- | --- | --- |
| `control-identity` | 3 | 3 survived | Stable |
| `enable-no-fence` (Enable skips the fence) | 5 | 5 killed | It sees a missed binding |
| `stress-lock-inversion` (management takes the fence after the root lock, and the delete seam locks the root) | 5 | 5 killed, `deadlock detected (SQLSTATE 40P01)` | It sees a real lock cycle |
| `manage-takes-fence-after-root` alone | 2 | 2 survived | Half a cycle is not a deadlock; the lock-order test kills it |
| `binding-locks-root-on-delete` alone | 2 | 2 survived | Same; `…BindingNeverTouchesARootRow` kills it |
| `create-no-fence` | 3 | 3 survived | `POST /api/issues` takes the fence twice (N15); the service suite kills it |

So the stress test is a detector for an actual cycle and for a missing Enable fence, not for an
order violation that has not yet met its counterpart. The statement-order tests remain the primary
guard for the lock order.

### 4.5 The inventory guard

`TestIssueBudgetWriterInventoryIsComplete` reads the source tree relative to its working
directory, so its red runs execute the prebuilt test binary inside a throwaway copy of the Go and
SQL sources (`/tmp/f3-s4c/guard/run-guard.sh`); the worktree is not written. Control: exit 0.
All 18 cases: exit 1 with a specific message.

| Case | Message class |
| --- | --- |
| `unlisted-go-writer`, `unlisted-go-reparent`, `unlisted-go-delete`, `second-call-in-listed-function`, `listed-caller-gone` | "the production callers of the issue writers changed" |
| `seam-removed-create`, `seam-removed-reparent-handler`, `seam-removed-reparent-collab`, `seam-removed-delete` | "… no longer calls <seam>" |
| `autopilot-passes-parent`, `onboarding-passes-parent` | "… passes the parent … without calling BindIssueBudgetOnCreate" |
| `new-sql-parent-writer`, `new-sql-delete`, `new-sql-insert`, `sql-writer-no-longer-writes` | "query … is an issue … write outside the inventory" |
| `raw-sql-delete-in-go`, `raw-sql-parent-in-go`, `raw-sql-insert-in-go` | "… writes issue rows in a string literal" |

## 5. Seam completeness (item 1)

Result: **no writer outside the inventory, and no seam is missing.** Line numbers are the current
ones in the worktree.

### 5.1 Statements

| Label | Query (statement at) | Production callers | Seam |
| --- | --- | --- | --- |
| S1 | `CreateIssue` (`pkg/db/queries/issue.sql:145`) | `internal/service/issue.go:554` in `CreateInTx`; `internal/handler/onboarding_shim.go:275`, `:446` | `BindIssueBudgetOnCreate` at `internal/service/issue.go:579`. Onboarding passes `emptyUUID` |
| S2 | `CreateIssueWithOrigin` (`issue.sql:281`) | `internal/service/issue.go:532` in `CreateInTx`; `internal/service/autopilot.go:790` | Same call at `:579`. Autopilot passes `pgtype.UUID{}` (`:806`) |
| S3 | `UpdateIssue` (`issue.sql:236`) | `internal/handler/issue.go:4112` only | `BindIssueBudgetOnReparent` at `:4119`, under the guard `issue.ParentIssueID != current.ParentIssueID` at `:4116` |
| S4 | `DetachDirectChildIssues` (`issue.sql:134`) | `internal/handler/issue.go:4971` only | None needed: a cleared parent keeps the member row (9.3, B57) |
| S5 | `UpdateCollaborationIssueStructure` (`collaboration_source_fields.sql:27`) | `internal/service/collaboration_edit.go:116` only | `BindIssueBudgetOnReparent` at `:124` |
| S6 | `PrepareWorkspaceDeletionLinks` (`workspace_delete.sql:254`) | `internal/handler/workspace.go:1204` only | None, by design: workspace deletion writes no budget row (section 5 of the design) |
| S7 | `DeleteIssue` (`issue.sql:364`) | `internal/handler/issue.go:5028` only | `TombstoneIssueBudgetMember` at `:5025`, before the delete |
| S8 | `DeleteWorkspaceIssueRoots` (`workspace_delete.sql:567`) | `internal/handler/workspace.go:1300` only | None, by design |
| P5 | Foreign key `issue.parent_issue_id` to `issue`, `ON DELETE SET NULL` | Whatever deletes a parent row | Clears the parent, as S4 |

### 5.2 Entry points

| ID | Entry | Path to the statement | Parent possible | Seam reached | Pinned by |
| --- | --- | --- | --- | --- | --- |
| C1 | `POST /api/issues` (members, agent CLI, quick-create agents) | `internal/handler/issue.go:3199`, `IssueService.Create`, `CreateInTx` (`internal/service/issue.go:315`) | Yes | `:579` | H `…EveryParentCapableWriterBinds/C1` |
| C2 | Comment sub-issue | `internal/handler/source_context.go:746`, `Create` | Yes | `:579` | H same, `/C2` |
| C3 | Channel `/issue` command | `internal/integrations/channel/engine/router.go:1085`, `Create` | No (params built at `:1055`) | `:579`, returns at once | not needed |
| C4 | Lifecycle handoff follow-up | `internal/handler/lifecycle_handoff_transaction.go:405`, `CreateInTx` | Yes (`:398`) | `:579` | H same, `/C4` |
| C5 | Triage item | `internal/handler/triage.go:609`, `CreateInTx` | No | `:579`, returns at once | not needed |
| C6 | Triage CSV import | `internal/handler/triage_import.go:443`, then C5's function | No | same | not needed |
| C7 | Collaboration create | `internal/service/collaboration.go:315`, `CreateInTx` | Yes | `:579` | S `…CollaborationWritersBindMembers` |
| C8 | Stage children | `internal/service/collaboration_stage_actions.go:577`, `CreateInTx` | Yes | `:579` | S same |
| C9 | Autopilot `create_issue` | `internal/service/autopilot.go:790`, direct | No | None | static guard: the literal must pass no parent |
| C10 | Onboarding with a runtime | `internal/handler/onboarding_shim.go:275`, direct | No | None | static guard |
| C11 | Onboarding without a runtime | `internal/handler/onboarding_shim.go:446`, direct | No | None | static guard |
| P1 | `PUT /api/issues/{id}`, `POST …/move`, `POST /api/issues/batch-update` | all three end in `updateIssueAtomicallyOnce`, `internal/handler/issue.go:4112` | n/a | `:4119` | H same, `/S3` (includes the batch entry), `…MoveKeepsTheMemberRow`, `…MoveIntoAnotherRootIsRecordedNotRefused` |
| P2 | Issue delete detaches the direct children | `internal/handler/issue.go:4971` | clears | None needed | H same, `/S4`; `…DeleteRootIssueTombstonesTheRoot` |
| P3 | Collaboration structure edit | `internal/service/collaboration_edit.go:116` | n/a | `:124` | S `…CollaborationWritersBindMembers` (S5), `…CollaborationEditBindsOnlyWhenTheParentChanges` |
| P4 | Workspace delete clears links | `internal/handler/workspace.go:1204` | clears | None, by design | H same, `/S6`; `TestWorkspaceDeletionLeavesBudgetRowsUntouched` |
| D1 | `DELETE /api/issues/{id}`, `POST /api/issues/batch-delete` | `deleteIssuesAndCollectAttachmentURLs`, `internal/handler/issue.go:5028` | n/a | `:5025` | H `…DeleteMemberLeavesATombstone`, `…BatchDeleteTombstonesEveryMember` |
| D2 | Workspace delete | `internal/handler/workspace.go:1300` | n/a | None, by design | `TestWorkspaceDeletionLeavesBudgetRowsUntouched` |

The paths you named:

- **Batch update**: `BatchUpdateIssues` calls `updateIssueAtomically` for each issue
  (`internal/handler/issue.go:5656`); same statement and seam as P1. The S3 subtest drives it.
- **Collaboration structure edits**: P3.
- **Autopilot `create_issue`** and **onboarding**: C9 to C11 insert directly, without `CreateInTx`,
  and pass an empty parent. Nothing but the new static guard would notice if one of them started
  passing a parent.
- **Quick-create completion**: the quick-create agent creates its issue through `POST /api/issues`
  (C1). Completion only links the task to the issue (`LinkTaskToIssue`,
  `pkg/db/queries/agent.sql:530`); it writes the task row and no issue parent.
- **Every caller of the `DeleteIssue` query**: one, `internal/handler/issue.go:5028`.
- The `current_iteration_id` branch of the update (`updateIssueIterationInTx`,
  `internal/handler/issue_iteration_assignment.go:241`) commits early and writes no parent.

### 5.3 How I looked

1. Every `-- name:` block in `pkg/db/queries/*.sql`, classified: inserts into `issue`, deletes
   from `issue`, updates `issue` and names `parent_issue_id`. Exactly the eight queries S1 to S8.
2. Every production call of those eight names (`grep`, then by syntax tree): the 12 call sites in
   5.1 plus `h.UpdateIssue(w, r)` in `internal/handler/issue_move.go:149`, which is the HTTP handler.
3. Every string literal in production Go: none inserts, deletes or re-parents an issue.
4. Migrations after 865: none touches issue rows.
5. The migrated database (880): two triggers on `issue` (`trg_issue_project_dirty_hourly`,
   `trg_issue_delete_dirty_hourly`, both mark hourly usage dirty), no rule, one function that names
   `parent_issue_id` (`lock_issue_execution`, which reads it from a task context), no function that
   writes `issue`. Foreign keys from `issue`: parent `SET NULL`, project `SET NULL`, workspace
   `CASCADE`; none from or to an `issue_budget_*` table.

Steps 1 to 3 are now `TestIssueBudgetWriterInventoryIsComplete`
(`internal/service/issue_budget_writers_test.go`): a pinned list of the eight queries and the 12
call sites, each with its seam or the reason it needs none. It asserts that the calling function
still calls the seam, and that a creator without a seam passes no parent. Its limits: it matches
calls by method name, so a query called through a method value is not seen; it cannot see a string
built by concatenation; triggers and migrations stay a manual check.

### 5.4 Reconciliation query for N2

With the seams in place, "the parent is a member" implies "the child is a member". The stress test
asserts it after every run. If you want a one-off check after a rolling deploy (N2), this anti-join
lists the violations. On 65,031 issues and 57,000 member rows it took 89 ms without locks
(parallel hash join, then an index-only probe of `issue_budget_member_issue_key`) and returned 0.

```sql
SELECT c.workspace_id, c.id AS issue_id, c.parent_issue_id, pm.budget_root_id
FROM issue_budget_member pm
JOIN issue c ON c.workspace_id = pm.workspace_id AND c.parent_issue_id = pm.issue_id
LEFT JOIN issue_budget_member cm ON cm.workspace_id = c.workspace_id AND cm.issue_id = c.id
WHERE cm.issue_id IS NULL;
```

**That query is only half of the check** (added 2026-11-18 13:30, after your rulings; section 18).
It finds a create or a move that missed its binding. It cannot find a **delete that missed its
tombstone**: the issue is gone, so there is no child row to join. Such a delete leaves a member row
with `tombstoned_at IS NULL` for an issue that no longer exists; when the issue was the root issue,
the root is never marked deleted, and `Reopen` is no longer refused with `root_deleted` (B13): the
refusal reads that one column (`internal/service/issue_budget.go:499`). Two ways to get there:

- an instance built before step 4 deletes a member issue. It has no tombstone seam, so this is
  certain, not a race (N2);
- under a `repeatable read` default, a delete that waited for the fence while `Enable` bound the
  issue. Measured with the real handler, run `rre_rr` in 12.3: `DELETE` answered 204, the issue is
  gone, its member row is there with `tombstoned=false`.

The second half of the check, also read-only. On the same data it took 60 to 71 ms over three runs
(`Hash Anti Join` over two sequential scans, 2,531 buffers; the plan is in
`/tmp/f3-s4c/fin/plan-q2.txt`) and returned 0. The first query, timed again in the same session,
took 61 ms:

```sql
SELECT m.workspace_id, m.issue_id, m.budget_root_id, m.bind_source
FROM issue_budget_member m
LEFT JOIN issue i ON i.workspace_id = m.workspace_id AND i.id = m.issue_id
WHERE m.tombstoned_at IS NULL AND i.id IS NULL;
```

Both are detectors, not repairs: one row in either result means the window was open. Neither lists
everything below the first hole (an unbound child's own children have an unbound parent, so the
first query does not list them).

## 6. The three tests that assert absence (item 2)

Each now has red runs. The mutants add a write where the product must write nothing.

| Test | Mutant (what the product wrongly did) | Result |
| --- | --- | --- |
| H `…WritesNothingForATreeWithoutARoot` | `absent-create-rootless` (a create under an unbound parent writes a member row) | 24 / 1, this test fails |
| | `absent-move-rootless` (a move under an unbound parent writes an observation) | 24 / 1, this test fails |
| | `absent-delete-rootless` (a delete writes an observation) | 24 / 1, this test fails |
| Detaching children keeps member rows: H `…DeleteMemberLeavesATombstone`, `…DeleteRootIssueTombstonesTheRoot`, `…EveryParentCapableWriterBinds/S4` | `absent-detach-tombstones-children` | 22 / 3, all three fail |
| | `absent-detach-deletes-child-members` | 22 / 3 |
| | `absent-detach-rewrites-bound-parent` | 22 / 3 |
| `TestWorkspaceDeletionLeavesBudgetRowsUntouched` | `absent-teardown-tombstones-members` | 23 / 2 (this test and subtest S6) |
| | `absent-teardown-deletes-roots` | 23 / 2 |
| | `absent-teardown-deletes-members` | 23 / 2 |
| | `absent-teardown-deletes-observations` | 24 / 1 |

The other absence tests were given red runs in the same pass: `…BusinessReopenWritesNothing`
(`absent-status-change-*`, 3 mutants), `…MoveKeepsTheMemberRow` (`absent-move-*`, 3),
`TestDeleteMemberIssueKeepsBudgetRows` and the tombstone tests (`absent-delete-*`, 7).

## 7. `ListIssueBudgetDescendants` (item 3)

Environment for sections 7 and 8: PostgreSQL 17.11 in the shared container (amd64 emulated on
arm64, so absolute times are pessimistic), `shared_buffers` 128MB, `work_mem` 4MB,
`random_page_cost` 4, `jit` on. pgx caches prepared statements, so both the custom plan and the
generic plan were captured (`PREPARE`, then `plan_cache_mode = force_generic_plan`). Scratch
database: 65,031 issues (15,000 in the target workspace), 57,000 member rows (12,000 in the target
workspace). Raw output: `/tmp/f3-s4c/plan/walk-members.out`.

### 7.1 Termination and correctness

Why it terminates: an issue has one parent, so walking down from a start node S, every node other
than S is produced by exactly one row (its parent's). The only way to meet a node twice is to come
back to S, and that row carries `cycle = true` and is not expanded (`WHERE NOT w.cycle`). A loop
that does not contain S cannot be reached at all: a node on a loop has all its ancestors on that
loop, so S, being an ancestor, would be on it. The measured cases agree:

| Case | Result |
| --- | --- |
| (a1) A loop elsewhere in the workspace | 2 rows, no `cycle` flag |
| (a2) A loop among the start's ancestors | 2 rows, no flag |
| (a3) A sibling tree that hangs below a loop | 1 row, no flag |
| (a4) The start is the tail below a loop it is not part of | 0 rows |
| (b) Loop of three through the start, with a side branch | 4 rows; the start comes back once, at depth 3, `cycle = t` |
| (b2) Loop of two through the start, with a tail below it | 5 rows; the start at depth 2, `cycle = t`; the tail is walked once |
| (c) The start is its own parent, with one ordinary child | 2 rows; the start at depth 1, `cycle = t` |
| (d) A chain 5,000 deep | 5,000 rows, deepest 5000, 0 flags, 5,000 distinct issues; 7.1 ms, 15,005 buffer hits |

The existing 10-level cap of the cycle check does not apply here; the test
`…EnableWalksPastTenLevelsAndRefusesALoop` binds a chain of 15 levels and the mutant
`walk-depth-limit-10` kills it.

### 7.2 Plan on a tree of 2,999 descendants (4-ary, depth 6)

Custom plan, second run:

```text
 Sort  (cost=605.66..605.71 rows=21 width=53) (actual time=6.633..6.728 rows=2999 loops=1)
   Sort Key: w.depth, w.id
   Sort Method: quicksort  Memory: 261kB
   Buffers: shared hit=15825
   CTE walk
     ->  Recursive Union  (cost=0.29..427.64 rows=21 width=37) (actual time=0.003..3.068 rows=2999 loops=1)
           Buffers: shared hit=6828
           ->  Index Scan using idx_issue_parent on issue i  (cost=0.29..8.33 rows=1 width=37) (actual time=0.003..0.004 rows=4 loops=1)
                 Index Cond: (parent_issue_id = '50000000-0000-4000-8000-000000000001'::uuid)
                 Filter: (workspace_id = '10000000-0000-4000-8000-000000000001'::uuid)
                 Buffers: shared hit=3
           ->  Nested Loop  (cost=0.29..41.91 rows=2 width=37) (actual time=0.173..0.461 rows=499 loops=6)
                 Buffers: shared hit=6825
                 ->  WorkTable Scan on walk w_1  (cost=0.00..0.20 rows=5 width=20) (actual time=0.000..0.031 rows=500 loops=6)
                       Filter: (NOT cycle)
                 ->  Index Scan using idx_issue_parent on issue c  (cost=0.29..8.33 rows=1 width=32) (actual time=0.001..0.001 rows=1 loops=2999)
                       Index Cond: (parent_issue_id = w_1.id)
                       Filter: (workspace_id = '10000000-0000-4000-8000-000000000001'::uuid)
                       Buffers: shared hit=6825
   ->  Nested Loop Left Join  (cost=0.41..177.56 rows=21 width=53) (actual time=0.006..6.313 rows=2999 loops=1)
         Buffers: shared hit=15825
         ->  CTE Scan on walk w  (cost=0.00..0.42 rows=21 width=37) (actual time=0.004..3.513 rows=2999 loops=1)
               Buffers: shared hit=6828
         ->  Index Scan using issue_budget_member_issue_key on issue_budget_member m  (cost=0.41..8.43 rows=1 width=32) (actual time=0.001..0.001 rows=0 loops=2999)
               Index Cond: ((workspace_id = '10000000-0000-4000-8000-000000000001'::uuid) AND (issue_id = w.id))
               Buffers: shared hit=8997
 Planning Time: 0.190 ms
 Execution Time: 6.854 ms
```

| Variant | Estimated cost | Time | Buffers |
| --- | --- | --- | --- |
| Custom plan | 605.66 (21 rows estimated, 2,999 actual) | 6.84 to 6.85 ms | 15,825 hits |
| Generic plan | 520.69 (11 rows estimated) | 6.89 ms | same shape |
| Same tree, empty member table | | 4.15 ms | 6,828 |
| A leaf (the common case) | | 0.020 ms | 2 |

- **`issue.parent_issue_id` is indexed**: `idx_issue_parent (parent_issue_id)` serves both the
  anchor and the recursive step. `idx_issue_workspace_parent (workspace_id, parent_issue_id)` also
  exists and was not chosen; the workspace is applied as a filter. Both are index scans, one probe
  per walked issue. There is no sequential scan of `issue`.
- The row estimate is off by two orders of magnitude (recursive CTEs always are), but the plan
  does not depend on it: with these indexes a nested loop is the right shape at any size.
- The cost is linear in the size of the subtree: about 2.3 buffer hits per issue for the walk and
  3 per issue for the member probe.

### 7.3 What `Enable` holds the fence for on this tree

`InsertIssueBudgetMembers` for the same 2,999 issues, with 57,000 rows already in the member
table, rolled back (`/tmp/f3-s4c/plan/bulk-insert.out`): 13.9 ms cold (18,191 hits, 18 reads),
9.6 ms warm (21,081 hits). Together with the walk (about 7 ms) and the in-flight scan (1 to 8 ms,
section 8) that is 25 to 35 ms of server time over 14 statements, during which the workspace fence
blocks every issue write of that workspace. This is the sum of the statements timed in SQL, not an
end-to-end `Enable` through Go (section 17).

## 8. `ListIssueBudgetInFlightTasks` (item 4)

### 8.1 The query against what the product writes

| Point | Product | Query | Match |
| --- | --- | --- | --- |
| Key names | `QuickCreateContext` (`internal/service/task.go:1571-1594`): `type`, `parent_issue_id`, … | `t.context->>'type'`, `t.context->>'parent_issue_id'` | Yes |
| `type` value | `QuickCreateContextType = "quick_create"` | `= 'quick_create'` | Yes |
| uuid casing | Written with `util.UUIDToString` (`task.go:1674-1699`): lowercase, hyphenated | `scope.issue_id::text = lower(…)`; `uuid::text` is lowercase and hyphenated | Yes. The `lower()` is defensive and costs the expression-index match discussed in 8.4 |
| "No issue yet" | `LinkTaskToIssue` (`pkg/db/queries/agent.sql:530`, guarded by `issue_id IS NULL`) runs when the task completes, so an in-flight quick-create task has no `issue_id` | `t.issue_id IS NULL` on the context branch | Yes. The first branch (by `issue_id`) and the second can never return the same task |
| Retry | `CreateManualQuickCreateRetryTask` (`agent.sql:662`) copies the context | same branch | Yes |
| In-flight statuses | `dispatched`, `waiting_local_directory`, `running` | same list in both branches | Yes |

Two differences that do not matter today: the query does not exclude chat or autopilot tasks
(N3), and it scopes by the agent's workspace (N4). On the test data the query returned the six
expected rows: four issue tasks and two quick-create tasks, in all three statuses.

### 8.2 Plan, 60,023 queue rows, 23 in flight

Queue: completed 55,800, failed 3,000, cancelled 1,200, running 11, dispatched 10,
waiting_local_directory 2; 3,905 rows carry a quick-create context; heap 12 MB (1,533 pages).
Scope: the 3,000 issue ids of the tree. Custom plan, second run, 15 of 300 agents in the workspace:

```text
 Sort  (cost=620.57..620.61 rows=16 width=50) (actual time=0.711..0.713 rows=6 loops=1)
   Buffers: shared hit=102
   ->  Append  (cost=7.79..620.25 rows=16 width=50) (actual time=0.085..0.706 rows=6 loops=1)
         ->  Nested Loop  (cost=7.79..338.13 rows=1 width=50) (actual time=0.085..0.132 rows=4 loops=1)
               ->  Seq Scan on agent a  (cost=0.00..11.75 rows=15 width=16) (actual time=0.008..0.025 rows=15 loops=1)
                     Filter: (workspace_id = '10000000-0000-4000-8000-000000000001'::uuid)
                     Rows Removed by Filter: 285
               ->  Index Scan using idx_agent_task_queue_agent on agent_task_queue t  (cost=7.79..21.75 rows=1 width=65) (actual time=0.007..0.007 rows=0 loops=15)
                     Index Cond: ((agent_id = a.id) AND (status = ANY ('{dispatched,waiting_local_directory,running}'::text[])))
                     Filter: (issue_id = ANY ('{...3000 ids...}'::uuid[]))
                     Buffers: shared hit=43
         ->  Hash Join  (cost=225.64..282.04 rows=15 width=50) (actual time=0.158..0.573 rows=2 loops=1)
               Hash Cond: ((scope.issue_id)::text = lower((t_1.context ->> 'parent_issue_id'::text)))
               ->  Function Scan on unnest scope  (cost=0.00..30.00 rows=3000 width=16) (actual time=0.097..0.194 rows=3000 loops=1)
               ->  Hash  (cost=225.62..225.62 rows=1 width=152) (actual time=0.053..0.054 rows=3 loops=1)
                     ->  Nested Loop  (cost=0.29..225.62 rows=1 width=152) (actual time=0.023..0.045 rows=3 loops=1)
                           ->  Seq Scan on agent a_1  (cost=0.00..11.75 rows=15 width=16) (actual time=0.002..0.016 rows=15 loops=1)
                           ->  Index Scan using idx_agent_task_queue_agent on agent_task_queue t_1  (cost=0.29..14.25 rows=1 width=168) (actual time=0.002..0.002 rows=0 loops=15)
                                 Index Cond: ((agent_id = a_1.id) AND (status = ANY ('{dispatched,waiting_local_directory,running}'::text[])))
                                 Filter: ((issue_id IS NULL) AND ((context ->> 'type'::text) = 'quick_create'::text))
                                 Buffers: shared hit=43
 Planning Time: 2.113 ms
 Execution Time: 0.748 ms
```

**It does not scan the queue.** The unindexed `lower(t.context->>'parent_issue_id')` is evaluated
only on the in-flight rows that the `(agent_id, status)` index returned (3 here), then hashed
against the scope.

### 8.3 Cost in every layout measured

| Layout | Plan | Estimated cost | Time warm (cold) | Buffers |
| --- | --- | --- | --- | --- |
| 60k rows, 15 of 300 agents in the workspace, 3,000 ids | custom | 620.57 | 0.75 ms (2.1) | 102 |
| same, one id (a reparent of a single issue) | custom | 243.41 | 0.10 ms | 54 |
| same, 3,000 ids | generic | 328.24 | 5.57 ms | 1,390 |
| 60k rows, all 300 agents in one workspace, 3,000 ids | custom | 1750.54 | 1.5 ms (3.2) | 172 |
| same, one id | custom | 855.92 | 0.53 ms | 90 |
| same, 3,000 ids | generic | 940.75 | 7.73 ms | 1,423 |
| 300k rows (heap 60 MB), one workspace, 3,000 ids | custom, parallel (2 workers) | 4628.46 | 30.3 ms (51.8) | 1,348 |
| same, one id | custom | | 0.83 ms | 1,012 |
| same, 3,000 ids | generic | | 15.9 ms | 5,389 |

Three things to know:

1. **The generic plan reads the task history of the scope issues.** It goes through
   `idx_agent_task_queue_issue_id_keyset (issue_id, id)`, which covers every status: 2,481 rows
   fetched and 2,477 discarded by the status filter at 60k rows; 7,772 fetched and 7,768 discarded
   at 300k. Its quick-create branch is a nested loop that compares every in-flight quick-create
   task with every scope id. The server keeps the generic plan of a prepared statement from its
   sixth execution on a connection when the generic estimate is not higher than the average custom
   one; here it is lower (328 against 621), and the pool sets no exec mode, so pgx prepares and
   caches statements by default. A long-lived connection will therefore end up on this plan.
2. **In a single-workspace deployment the `(agent_id, status)` index is read end to end**, because
   every agent matches; the index is 75 pages at 60k rows and 305 at 300k.
3. **At 300k rows the planner chose a parallel plan** whose 30 ms is mostly worker start-up; with
   the indexes below the same query takes 0.66 ms.

All of this runs while the caller holds the workspace fence: `Enable`, and a reparent that binds
at least one issue. `BindIssueBudgetOnCreate` does not run the scan.

### 8.4 Index proposal (not added)

No existing index is keyed by issue and restricted to the in-flight statuses
(`admin_alert_inflight_installation_idx` has the predicate but is keyed by the installation). Tried
on the 300k-row scratch clone only:

```sql
CREATE INDEX CONCURRENTLY ... ON agent_task_queue (issue_id)
    WHERE status IN ('dispatched', 'waiting_local_directory', 'running');
CREATE INDEX CONCURRENTLY ... ON agent_task_queue (lower(context->>'parent_issue_id'))
    WHERE status IN ('dispatched', 'waiting_local_directory', 'running')
      AND issue_id IS NULL
      AND context->>'type' = 'quick_create';
```

| With both, 300k rows | Estimated cost | Time | Buffers |
| --- | --- | --- | --- |
| Custom, 3,000 ids | 90.03 | 0.66 ms | 25 |
| Custom, one id | | 0.04 ms | 15 |
| Generic, 3,000 ids | | 5.96 ms | 25 (the time is the 5 tasks × 3,000 ids comparison) |

Assessment: **not needed at tens of thousands of rows**; every measured variant stays under 8 ms.
It is worth adding when a single-workspace deployment reaches a queue of millions of rows, because
the scan then holds the fence for the length of an index pass. If adopted: two single-statement
`CONCURRENTLY` migrations, and the second index is usable only while the query keeps the exact
expression `lower(context->>'parent_issue_id')` and the three predicates.

## 9. Lock order, read from the code (item 5)

### 9.1 Who touches a root row

`issue_budget_root` appears in five queries of `pkg/db/queries/issue_budget.sql` and in no other
query file; no production Go string names a budget table. All Go callers are in
`internal/service/issue_budget.go`:

| Query | Kind | Callers |
| --- | --- | --- |
| `InsertIssueBudgetRoot` | insert | `:440` (`Enable`) |
| `LockIssueBudgetRoot` | `FOR UPDATE`, the only row lock in the file | `:572` (`manage`) |
| `UpdateIssueBudgetRootPolicy` | update | `:598` (`manage`) |
| `GetIssueBudgetRootByIssue` | plain read | `:375` (`Enable`, the slot), `:609` (reader) |
| `GetIssueBudgetRoot` | plain read | `:615`, `:650` (readers) |

`internal/service/issue_budget_binding.go` sends exactly seven queries, none of them on a root:
`GetIssueBudgetMember` (`:192`), `InsertIssueBudgetMember` (`:64`), `InsertIssueBudgetMembers`
(`:212`), `InsertIssueBudgetObservation` (`:277`), `ListIssueBudgetDescendants` (`:117`),
`ListIssueBudgetInFlightTasks` (`:235`), `TombstoneIssueBudgetMember` (`:180`). The query file
contains no advisory lock. **No issue-writer path locks, reads or writes an `issue_budget_root`
row.** Not even the unlocked root read that rule 1 allows is used yet (it arrives with the mode
switch in 5c). Pinned by `…BindingNeverTouchesARootRow` (a statement log over every seam), which
the mutants `binding-locks-root-on-create`, `binding-locks-root-on-reparent` and
`binding-locks-root-on-delete` turn red.

### 9.2 Order per path

| Path | Locks and budget statements, in the order the code sends them | Design |
| --- | --- | --- |
| `Enable` (`issue_budget.go:344-475`) | workspace row `FOR KEY SHARE` (`:363`) → fence (`:369`) → slot read (`:375`) → issue read (`:390`) → formal (`:399`) and collaboration (`:402`) checks → own member (`:409`) → walk (`:415`), loop before conflict (`:423-434`) → activation time (`:436`) → root insert (`:440`) → root member (`:450`) → members (`:459`) → in-flight scan and observations (`:464`) → operation row (`:473`) | 9.2, same order. The position of the time sample is not in 9.2; 5b moves it (13.1) |
| Management (`manage`, `:551-605`) | root row `FOR UPDATE` (`:572`) → replay lookup (`:581`) → revision compare (`:584`) → decision (`:595`; reopen reads the root issue's member row, no lock) → root update (`:598`) → operation row (`:602`). No fence, no workspace lock | 9.4, same order |
| Create (`CreateInTx`, `internal/service/issue.go`) | the caller's locks → fence (`:395`) → issue insert (`:532` or `:554`) → `BindIssueBudgetOnCreate` (`:579`): parent member read, member insert | rule 1 |
| Update (`updateIssueAtomicallyOnce`, `internal/handler/issue.go`) | `lockIssueWriteFences` (`:3947`: workspace `FOR KEY SHARE`, subscriber, active member, status catalogue, fence) → issue row locks → `UpdateIssue` (`:4112`) → guard (`:4116`) → `BindIssueBudgetOnReparent` (`:4119`): member reads, walk, member inserts, observations, in-flight scan | rule 1, 9.3 |
| Collaboration edit | `mutateCollaborationWorkflow`: READ COMMITTED (`collaboration_workflow.go:464`) → workflow locks → fence (`:513`) → `UpdateCollaborationIssueStructure` (`collaboration_edit.go:116`) → `BindIssueBudgetOnReparent` (`:124`) | rule 1, 9.3 |
| Delete (`deleteIssuesAndCollectAttachmentURLs`, `internal/handler/issue.go`) | `lockIssueWriteFences` (`:4884`) → delete row locks → detach children (`:4971`) → `TombstoneIssueBudgetMember` (`:5025`) → `DeleteIssue` (`:5028`) | rule 1 |
| Workspace delete (`internal/handler/workspace.go:1125`) | workspace row `FOR UPDATE`; no fence; no budget statement | rule 5, last sentence |

Pinned by `…TransactionsFollowTheLockOrder`, which compares the exact statement list of `Enable`
and of each management operation. Mutants that turn it red: `enable-no-workspace-lock`,
`enable-no-fence`, `enable-issue-read-before-slot`, `enable-no-slot-check`,
`enable-no-own-member-check`, `enable-no-inflight-scan`, `manage-takes-fence`,
`manage-takes-fence-after-root`. `manage-revision-before-replay` is killed by
`…ManagementReplaysByOriginalKey`.

### 9.3 Why step 4 cannot deadlock

A wait-for cycle needs a transaction that waits while holding something another waits for.

- `Enable` holds the workspace key-share lock and the fence. After the fence it only reads without
  locks and inserts budget rows. Every unique key it inserts under (root per issue, member per
  issue, operation per scope and key) is contended only by writers of the same workspace, and all
  of those hold the same fence. It waits on nothing after the fence.
- Management holds one root row. It reads without locks and inserts one operation row whose key
  is scoped to that root, so only another management of the same root contends, and that one is
  behind the root lock. It never takes the fence.
- Issue writers hold the fence and their issue rows, and never lock a root.
- Workspace deletion conflicts with the others on the workspace row, which `Enable` and the issue
  writers take first, holding nothing else.

So no transaction holds a root row while waiting for the fence, and none holds the fence while
waiting for a root row. Step 5 changes this (finalize and the usage ingress lock roots, and the
binding lock is added); the order fence → binding lock → root of 10.10 is then what has to hold.

### 9.4 The stress test

`TestIssueBudgetConcurrentWritersKeepEveryTreeBound`
(`internal/handler/issue_budget_concurrency_test.go`), about 1.6 s. Twelve trees of eight fixture
issues, one tree at a time. On each tree the following are released at once, up to eight in
flight, each in its own transaction on its own pooled connection:

- `Enable` on the top issue, started after 0 to 5 of the issue writes have answered (a different
  number per tree, so it lands at the head, in the middle and near the tail of the writers); on
  every second tree a competing `Enable` on a child, of which exactly one may win;
- on the root that was created: `Close`, `Reopen`, `SetMode off`, `SetMode observe`, while the
  issue writes are still running;
- `POST /api/issues` under the top issue, under a child, and under a child that is being deleted
  (201 or 400);
- `PUT /api/issues/{id}` moving an outside subtree under a member, and moving an outside issue in
  and out again;
- `DELETE /api/issues/{id}` of a leaf member and of a member that has a child.

Afterwards it asserts: every request got one of its expected answers (a deadlock that PostgreSQL
breaks would come back as a 5xx or a plain error); exactly one root per tree; no issue under a
bound parent lacks a member row of the same root; a member row has a tombstone exactly when its
issue is gone; no issue of the tree is a member of another root; the root has the member row of
its root issue; no observation exists; the root is open, observing, at policy revision 5, with
exactly 5 operation rows.

What it can and cannot detect is measured in 4.4: it fails on an actual lock cycle and on a missing
`Enable` fence; it does not fail on one half of an inversion. It is meaningful as a regression net
for step 5, where the second half of a cycle becomes possible. It was run with `-race` (section 15).

## 10. Management operations (item 6)

Order of answers in `manage`, from the code: `invalid_request` (before the transaction) →
`not_found` (the lock finds no root in this workspace) → replay or `operation_conflict` →
`policy_revision_conflict` → `state_conflict` or `root_deleted` → write.

| Claim | Code | Test | Mutant killed |
| --- | --- | --- | --- |
| The replay lookup precedes the revision compare | `:581` before `:584` | `…ManagementReplaysByOriginalKey` (a replay carrying the old revision returns the original result) | `manage-revision-before-replay` |
| A stale revision is refused, never refreshed | `:584` | `…ManagementRequiresTheCurrentPolicyRevision` | `manage-no-revision-check` |
| A no-op answers `state_conflict` and consumes no revision | close `:480-481`, reopen `:502-503`, limit `:517-518`, mode `:532-533` | `…CloseAndReopenKeepTheSameRoot`, `…ModeSwitchIssuesANewEpochOnEveryReturn`, `…SetLimitToTheCurrentLimitIsRefused` (new); each compares the rows before and after the refusal | `close-noop-allowed`, `reopen-noop-allowed`, `setmode-noop-allowed`, `setlimit-noop-allowed` |
| The same key with another intent answers `operation_conflict` | `:270-271` | `…ManagementReplaysByOriginalKey`, `…OriginalKeyStandsForTheWholeIntent` (new) | `replay-conflict-ignored` |
| The digest covers every field that changes the outcome | `issueBudgetIntent`, `:206-215` | one mutant per field, below | |
| A tombstoned root refuses only reopen, before "already open" | `:499-500` before `:502-503` | `…DeletedRootIssueKeepsTheSlot`, `…DeletedRootIssueRefusesOnlyReopen` (new) | `reopen-deleted-allowed`, `reopen-open-check-before-deleted` |
| Every return to `observe` issues a new epoch; `off` does not | `:536-538` | `…ModeSwitchIssuesANewEpochOnEveryReturn` | `setmode-no-new-epoch`, `setmode-epoch-on-every-switch` |
| Only `off` and `observe` are accepted | `SetMode` | same | `setmode-any-mode-allowed` |
| The amounts and the limit survive close and reopen | `UpdateIssueBudgetRootPolicy` | `…CloseAndReopenKeepTheSameRoot` | `policy-update-zeroes-amounts`, `policy-update-drops-limit-on-admission`, `policy-update-no-revision-increment` |

Digest fields:

| Field | Mutant that drops it | Killed by |
| --- | --- | --- |
| Kind | `manage-digest-no-kind` | `…OriginalKeyStandsForTheWholeIntent` (new) |
| Expected policy revision | `manage-digest-no-revision` | `…ManagementReplaysByOriginalKey` |
| Limit | `manage-digest-no-limit`, `enable-digest-no-limit` | `…ManagementReplaysByOriginalKey`, `…EnableIsOnePerIssueAndReplaysByOriginalKey` |
| Mode | `manage-digest-no-mode` | `…OriginalKeyStandsForTheWholeIntent` (new) |
| Reason | `manage-digest-no-reason`, `enable-digest-no-reason` | same; `…EnableIsOnePerIssueAndReplaysByOriginalKey` |
| Workspace, scope kind, scope id | not run | They are the lookup key of the replay, so two requests that reach the comparison always agree on them; dropping one cannot change an answer |

Two things worth a conscious decision, both pinned now:

- **The operator is not in the digest.** A second operator who sends the same key and intent gets
  the first operator's result as a replay (`…OriginalKeyReplaysForAnotherOperator`). This matches
  the code comment and B41 ("another admin reads it by the original key"); step 7's in-transaction
  membership check runs before the replay lookup and is what keeps a demoted operator out.
- **A refused request leaves no operation row**, so its key is still free and may later carry a
  different intent. That follows from "a no-op writes nothing" in 9.4.

## 11. Repository rules (item 7)

| Rule | Check | Result |
| --- | --- | --- |
| No `FOREIGN KEY` / `REFERENCES`, no cascades | `grep` over migrations 866 to 880; `pg_constraint` on the migrated database | The only match is a comment. The six tables carry 55 CHECK constraints and nothing else; no foreign key from or to them |
| Every index `CONCURRENTLY`, one statement per file | `grep` over the same files | 13 index migrations, one `CREATE … INDEX CONCURRENTLY` each; 866 and 880 create none. (Owned by the step 3 check; confirmed only) |
| Handler UUID rules | Read of the two handler edits (`internal/handler/issue.go:4116-4122`, `:5023-5027`) | Both pass ids of rows the handler loaded and locked (`issue.WorkspaceID`, `issue.ID`); no new request-boundary parsing. Tests use `parseUUID` on fixture ids |
| English code comments | `LC_ALL=C grep '[^ -~<tab>]'` over the 9 step 4 source and test files I read or wrote | 0 non-ASCII bytes |
| DB-backed tests build rows through `internal/testutil` | `grep` for `INSERT INTO`, `RETURNING id`, `httptest.NewRecorder` in the six budget test files | None left (D4 removed the two that were there). The remaining raw statements are `UPDATE` / `DELETE` through `fx.Exec` for states no helper offers: ledger amounts on a root row, parent loops, and "what the delete transaction does" in service-level fixtures |
| Helpers assert no product rule | Read of the helpers in the budget test files | They insert, call and compare; the expected values are passed in by each test |
| Default tests never resolve an agent CLI | All my runs of other packages went through `scripts/go-test-with-agent-cli-guard.sh` | 0 "unexpected agent CLI invocation" lines |

## 12. Transaction isolation (item 8)

Result: **no REPEATABLE READ transaction can reach a binding seam** (8b; by reading and by a
call-graph check, 12.2). Nine transactions reach a seam or belong to the budget service. One of
them states READ COMMITTED; the other eight are READ COMMITTED **only because the server default
says so** (8a, 8c; finding N1, experiment in 12.3).

How a transaction gets its level here. Production code hands the `pgxpool.Pool` to every service
(`cmd/server/router.go:473`). `Pool.Begin` is `BeginTx(ctx, pgx.TxOptions{})`
(pgx v5.9.2 `pgxpool/pool.go:790-792`) and sends the bare word `begin` (`tx.go:56-59`), so the
level is the session's `default_transaction_isolation`. `Begin` on a `pgx.Tx` sends
`savepoint sp_N` (`tx.go:165-171`, `pgxpool/tx.go:17-19`) and stays in the outer transaction at
its level. A level is stated either by `SET TRANSACTION ISOLATION LEVEL …` as the first statement
or, in one place, by `pgx.TxOptions{IsoLevel: …}` (`internal/iteration/transaction.go:28`).

### 12.1 Every path, its transaction and its level (8a)

Labels as in section 5. "Default" means: no level stated, so `default_transaction_isolation`.

| Path | Transaction begins at | Fence taken at | Level | Seam |
| --- | --- | --- | --- | --- |
| C1 `POST /api/issues`, C2 comment sub-issue, C3 channel `/issue` | `IssueService.createOnce`, `internal/service/issue.go:278` | `:307` (through the options) and `:395` in `CreateInTx` | Default | create, `:579` (C3 passes no parent) |
| C4 lifecycle handoff follow-up | `Handler.writeLifecycleHandoffInTx`, `internal/handler/lifecycle_handoff_transaction.go:249` | `:275` | Default | create, through `CreateInTx` at `:405` |
| C5 triage item | `beginTriageWrite`, `internal/handler/triage.go:220`, called from `CreateTriageItem` (`:674`) | `:253` | Default | create, through `:609`; no parent |
| C6 triage CSV import | the same helper, called from `commitTriageImportRow` (`internal/handler/triage_import.go:381`); the row runs in a savepoint (`:439`) | `triage.go:253` | Default (the savepoint keeps the outer level) | create, through `:443` and `triage.go:609`; no parent |
| C7 collaboration create | `CollaborationService.Create`, `internal/service/collaboration.go:165` | in `CreateInTx` | Default | create, through `:315` |
| C8 stage children | `mutateCollaborationWorkflow`, `internal/service/collaboration_workflow.go:428` | `:513` | **READ COMMITTED, stated** (`:464`, first statement) | create, through `collaboration_stage_actions.go:577` |
| C9 autopilot `create_issue` | `dispatchCreateIssue`, `internal/service/autopilot.go:681` | `:710` | Default | none (direct insert at `:790`, no parent) |
| C10, C11 onboarding | `internal/handler/onboarding_shim.go:173`, `:393` | helper at `:529` | Default | none (direct inserts at `:275`, `:446`, no parent) |
| P1 update, move, batch update | `updateIssueAtomicallyOnce`, `internal/handler/issue.go:3940`; a batch runs one transaction per issue (`:5656`) | `lockIssueWriteFences`, `:3947` | Default | reparent, `:4119` |
| P3 collaboration structure edit | `mutateCollaborationWorkflow`, as C8 | `collaboration_workflow.go:513` | **READ COMMITTED, stated** | reparent, `internal/service/collaboration_edit.go:124` |
| D1 delete and batch delete (one transaction for the whole list), with P2 the detach of direct children | `deleteIssuesAndCollectAttachmentURLs`, `internal/handler/issue.go:4878` | `:4884` | Default | tombstone, `:5025` |
| D2, P4 workspace deletion (clears links, deletes issue roots, then `DeleteWorkspace` at `internal/handler/workspace.go:1365`, whose cascade removes what is left) | `internal/handler/workspace.go:1125` | none, by design | Default | none, by design |
| Project deletion | `runProjectDeleteTransaction` (`internal/handler/project_write_fence.go:83-84`), called at `internal/handler/project.go:902` | project locks | READ COMMITTED, stated (`project_write_fence.go:117`) | none: it deletes no issue; it only clears `issue.project_id` (`pkg/db/queries/project.sql:123`) |
| `Enable` | `IssueBudgetService.transact`, `internal/service/issue_budget.go:242` | `:369` | Default | is the binder |
| `Close`, `Reopen`, `SetLimit`, `SetMode` | the same `transact` | none (root lock only) | Default | none |

The two openers you asked me to settle:

- **`internal/handler/triage_import.go:443`.** The value passed down is `nested`, created by
  `tx.Begin(ctx)` at `:439` on the `pgx.Tx` that `beginTriageWrite` returned at `:381`. That is a
  savepoint of the outer transaction, not a new transaction. The outer one begins at
  `internal/handler/triage.go:220` with no level and takes the fence at `:253`. So: default level,
  and the import never passes a parent (`triage.go:609` builds the parameters without one).
- **`DecideStageProposal` to `materializeStageChildren`** (`collaboration_stage_actions.go:514`,
  `:577`). The transaction is the one `mutateCollaborationWorkflow` begins at
  `collaboration_workflow.go:428`; `workflowInTx` states READ COMMITTED at `:464` before any
  other statement. `materializeStageChildren` wraps the children in the SQL savepoint
  `stage_materialization` and calls `issues.CreateInTx(ctx, t.tx, …)` with that same transaction.
  This is the only seam transaction whose level does not depend on the server.

Bulk paths. Batch delete is one transaction for the whole list; batch update is one transaction
per issue; workspace deletion is the only path that removes or re-parents issues in bulk without
a seam, and design section 5 says it must not have one. Project deletion deletes no issue.
Nothing else writes `issue.parent_issue_id` or deletes an issue (section 5).

### 12.2 No REPEATABLE READ transaction reaches a seam (8b)

**Inventory.** Every place where production Go names a level (`grep` for `ISOLATION LEVEL`,
`IsoLevel`, `pgx.RepeatableRead`, `pgx.ReadCommitted`, `pgx.Serializable`, `transaction_isolation`,
`BeginTx(`, `TxOptions`, `SESSION CHARACTERISTICS`, and for `BEGIN` / `START TRANSACTION` strings):

- No SERIALIZABLE, no session-level setting, no hand-written `BEGIN`.
- REPEATABLE READ **without** READ ONLY: four statements, below.
- REPEATABLE READ READ ONLY: twelve statements. `internal/handler/collaboration.go:120`,
  `issue_table_query.go:43`, `task_capability.go:166`; `internal/service/admin_collaboration_view.go:91`,
  `collaboration_comments.go:109`, `collaboration_creation_options.go:37`,
  `collaboration_fields.go:66`, `collaboration_knowledge_view.go:241`,
  `collaboration_lifecycle.go:122`, `collaboration_notifications.go:157`,
  `collaboration_queue.go:464`, `collaboration_switch_view.go:79`.
- READ COMMITTED, stated: ten statements. `internal/handler/iteration_operations.go:63`,
  `project_update_notifications.go:75`, `project_write_fence.go:117`, `squad.go:585`;
  `internal/iteration/transaction.go:28`; `internal/service/collaboration_workflow.go:464`,
  `issue_status_write.go:42`, `iteration.go:42`, `iteration_notifications.go:73`,
  `task_start.go:28`.

| REPEATABLE READ, not READ ONLY | Statement | Code that runs inside | Module functions reachable (precise / with interface methods) | Seam or issue-writing query reached |
| --- | --- | --- | --- | --- |
| `Handler.iterationHistoryReadAttempt` | `internal/handler/iteration_history.go:54` | 2 closures: `ListIterations` (`iteration_list.go:96`), `readIterationHistoryView` (`iteration_history.go:93`) | 70 / 71 | none |
| `Handler.runProjectTransaction`, through `projectTransactionOnce` | `internal/handler/project_write_fence.go:115`, chosen at `:75` | 14 closures: `project_timezone.go:50`, `:62`, `:107`; `project.go:117`, `:647`, `:827`; `project_health.go:113`, `:189`; `project_update.go:558`, `:615`, `:835`, `:903`, `:985`; `report_snapshot.go:275` | 548 / 551 | none |
| `IterationService.previewIterationOnce` | `internal/service/iteration_lifecycle_preview.go:101` | its own body | 118 / 119 | none |
| `CollaborationService.readCollaborationStageEvidence` | `internal/service/collaboration_stage_evidence_read.go:21` | 3 closures: `collaboration_stage_review_view.go:134`, `collaboration_test_report_view.go:266`, `:352` | 264 / 269 | none |

Design section 5 rule 5 names the first three. **The fourth is not in the rule** (N18). Its comment
says why it is not READ ONLY ("M1's current-authority observer must hold permission row locks"),
so PostgreSQL would not refuse a write there: only the call graph keeps it away from a seam.

**By reading.** The chain above each seam is short and closed:

- `BindIssueBudgetOnCreate` has one caller, `IssueService.CreateInTx` (`internal/service/issue.go:579`).
  `CreateInTx` has five: `issue.go:315` (`createOnce`), `lifecycle_handoff_transaction.go:405`,
  `triage.go:609` (`createTriageItemInTx`, itself called from `CreateTriageItem` and
  `triage_import.go:443`), `collaboration.go:315` (`createCollaborationIssueInTx`, reached only
  from `CollaborationService.Create` at `:170`), `collaboration_stage_actions.go:577`.
- `BindIssueBudgetOnReparent` has two callers: `internal/handler/issue.go:4119` and
  `internal/service/collaboration_edit.go:124`.
- `TombstoneIssueBudgetMember` has one: `internal/handler/issue.go:5025`.

Each of those functions either begins its own transaction (12.1) or receives one from a caller
listed in 12.1. None of the sixteen REPEATABLE READ transactions is among them.

**By machine.** Reading does not rule out a path through an interface or a callback, so I checked
reachability on the whole server module. The tool is a 509-line Go program
(`/tmp/f3-s4c/cg/main.go`, not in the worktree; `golang.org/x/tools` v0.48.0 from the module
cache, built with `GOPROXY=off`). It builds SSA for `./...` and a VTA call graph seeded by CHA:
124,853 functions, 34,715 of them in the module. A whole-program walk is useless here, because
through the standard library every function reaches every other one. A seam needs the
transaction's `*db.Queries`, which only module code can pass on, so the walk stays in module code:

1. calls to module functions, static or through interfaces and function values as VTA resolves them;
2. every function value a reached function creates or names (so a closure handed to library code
   is followed);
3. in a second, more conservative mode, every method of a module type that a reached function
   converts to an interface.

Roots for each transaction: the function that names the level, every function value handed to it
at its call sites (through forwarded parameters too), and its callers when it returns the
transaction. Targets: the four seam functions and the eight issue-writing queries of 5.1.

| Group | Functions that name the level | Result |
| --- | --- | --- |
| REPEATABLE READ, not READ ONLY | 5 (4 transactions; `runProjectTransaction` and `projectTransactionOnce` are one) | No target reached in either mode |
| REPEATABLE READ READ ONLY | 12 | No seam function in either mode, no target at all in the precise mode. In the interface mode three of them (`beginIssueTableSnapshot`, `taskCapabilityRead`, `CreationOptions`) mark the eight query methods, only because a `*db.Queries` is converted to an interface there and that mode marks every method of the type. It is not a call; and PostgreSQL refuses a write in a READ ONLY transaction |
| READ COMMITTED, stated | 12 | Only `workflowInTx` reaches a seam (C8 and P3) |

Backward, as a cross-check: of the 190 module functions that can reach `BindIssueBudgetOnCreate`,
those that begin a transaction or name a level are `createOnce`, `writeLifecycleHandoffInTx`,
`CreateTriageItem` (through `beginTriageWrite`), `commitTriageImportRow`,
`CollaborationService.Create`, `mutateCollaborationWorkflow` and `workflowInTx`. For
`BindIssueBudgetOnReparent` (173 functions): `updateIssueAtomicallyOnce`,
`mutateCollaborationWorkflow`, `workflowInTx`. For `TombstoneIssueBudgetMember` (14):
`deleteIssuesAndCollectAttachmentURLs`. That is exactly the list of 12.1.

Limits of the machine check: reflection, `unsafe`, `go:linkname` and cgo are not modelled. A
callback registered outside the transaction's own code would not be followed; the production pool
registers none (no tracer, no connection hook; the only one in the module is `OnNotice` in
`cmd/migrate/main.go:935`). Nothing in the tree repeats this check after today: the static guard
pins the call sites of the eight queries and their seams, not the level of their transactions.
The step 5b assertion is the durable guard. Say so if you want the tool kept with the task.

### 12.3 The server default (8c), and what happens without it

**What sets the level today.** Nothing in the product; it is the PostgreSQL built-in default.

- Database (the shared container, PostgreSQL 17.11): `default_transaction_isolation` is
  `read committed` with source `default`; `pg_db_role_setting` has 0 rows, so no database or role
  overrides it.
- Pool: `cmd/server/dbstats.go:83` and `internal/dbstartup/startup.go:87-126` build it with
  `pgxpool.ParseConfig(DATABASE_URL)` and set sizes, the connect timeout and the dialer. No
  `RuntimeParams`, no connection hook, no tracer, no level.
- No migration and no query changes a session default.
- Nothing checks it either: no startup probe reads the setting.

**What can change it without a code change.** `postgresql.conf` or `ALTER SYSTEM`;
`ALTER DATABASE … SET`; `ALTER ROLE … SET`; and the connection itself: pgx forwards every
`DATABASE_URL` parameter it does not know to the server as a runtime parameter
(pgx `pgconn/config.go:340-378`) and honours `PGOPTIONS` (`:537`). In the tracked files every
`DATABASE_URL` example carries only `sslmode`, and no compose file, Helm template, workflow or
script sets the default level (`PGOPTIONS` appears only in test guards and in one script, for
`statement_timeout` and read-only mode). I cannot see what a deployed instance is set to
(section 17).

**Experiment.** A scratch clone with
`ALTER DATABASE … SET default_transaction_isolation TO 'repeatable read'` (a new session reported
`repeatable read`), then the two budget selections and one throwaway test, product code unchanged
(`/tmp/f3-s4c/run-rr.sh`):

| Run | Exit | PASS / FAIL / SKIP | What failed |
| --- | --- | --- | --- |
| `rr_svc`: `./internal/service/ -run TestIssueBudget` (30 tests at that time) | 1 | 27 / 3 / 0 | `…EnableConcurrentlyOnOneIssue` (3 subtests), `…EnableParentAndChildConcurrently`, `…EnableRacingChildCreate` (3 subtests) |
| `rr_hdl`: the handler budget selection | 1 | 24 / 2 / 0 | `…ConcurrentWritersKeepEveryTreeBound`, and `TestDeleteIssueIterationSeesJoinAfterFenceWait`, which is not a budget test |
| `rrm_rr`: a throwaway test supplied through `-overlay`, two `Close` calls with one key, the second started while the first holds the root lock | 0 | 1 / 0 / 0 (the test only logs what it saw) | The waiting call: `lock budget root: ERROR: could not serialize access due to concurrent update (SQLSTATE 40001)` |
| `rrm_rc`: the same test on a clone with the built-in default (control) | 0 | 1 / 0 / 0 | Nothing: the waiting call returns the stored result, `replayed=true`, same operation id |

Five more runs, made at 12:42 and 13:15 after your rulings, to settle two sentences of mine that
you took into `design.md` (section 18, R1 and R3). Same recipe; the changed sources reached the
compiler through `-overlay` only (`/tmp/f3-s4c/fin/run-rrd.sh`, `/tmp/f3-s4c/fin/run-rre.sh`):

| Run | Exit | PASS / FAIL / SKIP | What it showed |
| --- | --- | --- | --- |
| `rrd_noseam_rr`: `TestDeleteIssueIterationSeesJoinAfterFenceWait` alone, on a copy of `internal/handler/issue.go` **without the five lines of the tombstone seam** and with one log line before the 500 | 1 | 0 / 1 / 0 | `zz delete cause error="ERROR: could not serialize access due to concurrent update (SQLSTATE 40001)"`, then `expected 204, got 500` |
| `rrd_asis_rr`: the same test, the seam in place, only the log line added | 1 | 0 / 1 / 0 | The same cause, the same 500 |
| `rrd_noseam_rc`: the first variant on the built-in default (control) | 0 | 1 / 0 / 0 | Nothing |
| `rre_rr`: a throwaway test, the real `DeleteIssue` handler for a child issue, started while `Enable` of its parent holds the fence (one backend seen blocked by `Enable`'s pid before `Enable` went on) | 0 | 1 / 0 / 0 (the test only logs what it saw) | Nothing fails: `enable: member_count=2`, `DELETE answered 204`, `issue rows left=0; member row present=true source="activation" tombstoned=false`, `member rows of the root that are not tombstoned and have no issue: 1` |
| `rre_rc`: the same on the built-in default (control) | 0 | 1 / 0 / 0 | `tombstoned=true`; `… have no issue: 0` |

What the failures say, in three classes:

1. **Loud, no wrong data.** Two enables of one issue: `the caller that ran second: got create
   budget root: ERROR: duplicate key value violates unique constraint
   "issue_budget_root_issue_key" (SQLSTATE 23505), want refusal "already_enabled"`. Parent and
   child enabled together: `got bind root issue: ERROR: duplicate key value violates unique
   constraint "issue_budget_member_issue_key" (SQLSTATE 23505), want refusal "root_conflict"`.
   The unique indexes hold the invariant; the caller gets an error where the design promises a
   refusal or a replay. The management operations belong here too (`rrm_rr`): a same-key retry
   that queued behind the original fails with `40001`; afterwards there is one close operation
   row, the policy revision is 2, and the next retry replays.
2. **Silent, wrong data.** `Enable` racing a child create, both commit, no error anywhere:
   `create_commits_first`: `issue … has no member row, want {… Source:activation …}`;
   `enable_commits_first`: `issue … has no member row, want {… Source:create …}`;
   `either_order`: `round 0: the child has 0 member rows, want exactly 1`. The child sits under
   an enabled parent with no member row and no observation.
   The delete has the same hole (`rre_rr`, added after your rulings): a delete that waited for
   the fence while `Enable` bound the issue answers 204, the issue is gone, and its member row
   stays `tombstoned=false`. `Enable` writes no issue row, so the delete meets no concurrent
   update and nothing fails; its tombstone `UPDATE` (`pkg/db/queries/issue_budget.sql:116-122`)
   simply matches no row in a snapshot taken before `Enable` committed. The second query of 5.4
   finds such a row; the first does not.
3. **The rest of the product depends on the default too.** The stress test stopped at
   `POST /api/issues answered 500: {"error":"failed to create issue: allocate issue number:
   ERROR: could not serialize access due to concurrent update (SQLSTATE 40001)"}`, and
   `TestDeleteIssueIterationSeesJoinAfterFenceWait` (`internal/handler/issue_delete_iteration_test.go:130`)
   at `DELETE /api/issues/<id>: expected 204, got 500: {"error":"failed to delete issue"}`. That
   request deletes an **issue** whose iteration membership another transaction changed while the
   delete waited for the fence; no iteration is deleted. Both are code from before step 4.
   Measured since (`rrd_noseam_rr`, `rrd_asis_rr`): the error behind that 500 is `40001`, and it
   is the same with the tombstone seam removed, so step 4 has no part in it. The log gives the
   SQLSTATE, not the statement; by reading, it is the first statement that locks the issue row
   after the wait, `LockIssueDeleteRows` (`internal/handler/issue.go:4898`), 127 lines before the
   seam (`:5025`): the test's other transaction updated that row and committed. So a
   deployment with that default already misbehaves under concurrent writes, which makes it likely
   to be noticed, but through unrelated errors and only under load.

Why class 2 happens. Under REPEATABLE READ the snapshot is taken by the first statement of the
transaction and the fence wait comes after it. In `Enable` the first statement is
`LockWorkspaceForChatSessionCreate` (`issue_budget.go:363`), the fence is at `:369`, the walk
later: a child committed while `Enable` waited is not in the walk. In the create transaction the
snapshot also predates the fence wait, so the lookup of the parent's member row does not see what
`Enable` has just committed. The fence still orders the two writers; each of them reads the past.

**What the step 5b assertion covers.** Rule 5 puts it after a member row was read, in that branch
only. Under a wrong default:

- The create that lost the race finds no member row (that is the bug), so the assertion does not
  run in the transaction that makes the hole.
- The delete that lost the race reads no member row either (`rre_rr`): its tombstone is an
  `UPDATE` that matches nothing, and a seam that asserts only "after a member row was read" has
  nothing to assert on. Checking the level before the tombstone statement would catch it, at the
  price of one more round trip in every delete transaction, with or without a root, which rule 5
  excludes on purpose. Yours to weigh; I only record that the delete is a third case next to the
  two residual cases the supplement of rule 5 lists. (You have since added it there as the third
  and ruled against the extra round trip: 18.2, R3.)
- It runs in the first later create, move or delete that does find a member row. From then on the
  deployment is loud on every budgeted tree. That is useful: the wrong default cannot stay hidden.
- `Enable` and the management operations have no such branch. `Enable` reads the issue's own
  member row only to refuse (`:409`); on its normal path there is none. Its stale walk is the
  other half of class 2, and its class 1 errors stay raw `23505`s.

**Recommendation, yours to rule; I changed nothing here** (you told me not to restructure a
caller's transaction, and a level statement changes the pinned statement lists). **Ruled
2026-11-18**, as I read `design.md` (section 5 supplement, 12.7) and `implement.md`: point 1 is
taken for 5b; point 2 stays as designed; point 3 is not done in this task and goes to the user as
an option; point 4 is taken. The four points as I wrote them:

1. State the level in `transact`, as `collaboration_workflow.go:464` and
   `internal/service/task_start.go:28` do: one statement, first in the transaction. It closes the
   `Enable` half, and it settles `manage`: its first statement is the root lock, and under
   REPEATABLE READ a `FOR UPDATE` that waited for a committed update fails with `40001` (measured,
   `rrm_rr` above), so a same-key retry that queued behind the original gets an error where 9.4
   promises the replay. Cost: one round trip per budget operation, and one more line in each of
   the five exact statement lists of `…TransactionsFollowTheLockOrder`. The alternative is to run
   the 5b assertion in `transact` too.
2. Keep the 5b assertion for the seven seam transactions as designed. It cannot prevent the first
   hole there; preventing it would mean stating the level at six `Begin` sites that are not budget
   code (`internal/service/issue.go:278`, `internal/handler/lifecycle_handoff_transaction.go:249`,
   `internal/handler/triage.go:220`, `internal/service/collaboration.go:165`,
   `internal/handler/issue.go:3940`, `:4878`).
3. An option outside F3, mine: a startup probe that reads `SHOW default_transaction_isolation`
   once and refuses to serve when it is not `read committed`. It would cover every transaction
   that names no level, including the two non-budget failures above.
4. For step 7: construct `IssueBudgetService` with the pool. A `pgx.Tx` also satisfies
   `TxStarter` (`internal/service/autopilot.go:34-36`); `transact` would then run as a savepoint
   at the caller's level, and its commit would only release the savepoint. Today the constructor
   is called only from tests, always with the pool.

## 13. What step 4 leaves to the later steps

### 13.1 The activation sample against the in-flight scan (implementation report 13.4)

**The implementer's reasoning is right. I confirmed it from the code and reproduced it.** Not
fixed, as you ruled: step 5b reorders it.

From the code. `Enable` samples the activation time at `issue_budget.go:436`
(`iteration.SampleBusinessTime`, which is `SELECT clock_timestamp()`,
`internal/iteration/transaction.go:100-107`), then inserts the root (`:440`), the root member
(`:450`) and the descendants (`:459`), and only then scans (`:464`). `activated_at` is `NOT NULL`
without a default (`migrations/866_issue_budget.up.sql:48`, under the comment "Costs are
attributed exactly from this point on"), so the stored value is that sample. The scan is one READ
COMMITTED statement and returns what is in flight when it starts. A task that was in flight at
the sample and commits its terminal status during the three inserts is not returned: no
observation. It has no attempt either, at step 4 because nothing writes one, and after 5b because
its claim predates the root.

By a run. A throwaway test, given to `go test` through `-overlay` and never in the worktree
(`/tmp/f3-s4c/act/zz_activation_window_test.go`, 98 lines): a `TxStarter` wrapper stops `Enable`
before a named statement and, on another pooled connection, completes a task that was `running`
on a child of the issue. Run `act_window`: exit 0, 1 PASS / 0 FAIL / 0 SKIP (both subtests pass).

| Case | The hook fires before | Task finished | Gap observations | Attempts | `in_flight_gap_count` returned |
| --- | --- | --- | --- | --- | --- |
| The task ends after the sample, before the scan | `InsertIssueBudgetRoot` | 37.113 ms after `activated_at` | **0** | 0 | 0 |
| Control: the task ends after the scan | `InsertIssueBudgetObservation` | 47.82 ms after `activated_at` | 1 | 0 | 1 |

So the window is the three inserts between `:436` and `:464`. On the tree of section 7 the
descendant insert alone took 9.6 to 13.9 ms of server time, so the window grows with the tree.

The 5b order (binding lock, scan, sample, inserts; design 10.10) removes it, and I agree with the
argument that the remaining error is conservative:

- A task that is terminal before the scan ended before the sample: correctly outside.
- A task in flight at the scan gets a gap. If it ends between the scan and the sample, the gap
  was not needed; nothing is lost.
- Nothing can start being attributed between the scan and the sample: finalize needs the shared
  binding lock, and `Enable` holds it exclusively until its commit. A finalize that already held
  the shared lock makes `Enable` wait for the exclusive one, so its task is in flight at the scan.

What the reorder needs from the step 4 code, checked: the scan takes an array of issue ids, not a
join on member rows (`pkg/db/queries/issue_budget.sql:149-179`), so it can run before the member
inserts; the root id is generated in Go (`dbid.NewV7()`), so the observation rows can name it
before the root row exists; no budget table has a foreign key. The one edit is in
`recordIssueBudgetInFlightTasks` (`issue_budget_binding.go:234`), which scans and inserts in one
helper and has to be split. The same helper serves reparent rule 2, where no instant is stored.

The other two points of 13.4 also hold. `bound_at` and every `created_at` default to `now()`,
the transaction start (`866_issue_budget.up.sql:66`, `:49`, `:104`, `:190`, `:210`); on the update
path the transaction begins at `internal/handler/issue.go:3940`, before the fences. No production
code reads `bound_at`, and `activated_at` is only copied into the result (`issue_budget.go:470`).

### 13.2 Design section 10 against the step 4 code

Read against the design as it stood at 08:12 on 2026-11-18 (126,065 bytes).

**Nothing in step 4 makes a rule of section 10 impossible.**

- The binding functions take a `*db.Queries` on the caller's transaction, so the binding lock
  (10.10), the mode read (10.6) and the isolation assertion (rule 5) can each be one new query.
- The two places 10.10 names for the exclusive lock exist as distinct points: in `Enable` between
  the last refusal check and the sample (`issue_budget.go:434` to `:436`); in reparent rule 2
  between the walk (`issue_budget_binding.go:117`) and the member insert (`:141`). The walk runs
  under the fence, so the tree cannot change between the walk and the lock.
- A refused or replayed `Enable` returns before that point, as 10.10 requires.
- The 5b order of `Enable` needs only the split described in 13.1.
- Every observation goes through one helper (`insertIssueBudgetObservation`,
  `issue_budget_binding.go:267`, four callers), so "write no observation while the root is `off`"
  (10.6) has one place to be enforced, next to each caller's own mode read.

**Awkward, not blocking.**

1. **10.7 needs the executing task at the create seam, and the seam cannot receive it.**
   `BindIssueBudgetOnCreate(ctx, q, issue)` sees only the issue row. `IssueCreateParams` carries
   the caller-reported origin (`OriginType`, `OriginID`, validated at `internal/service/issue.go:587-595`),
   which 10.7 rules out as the identity; there is no field for the task id resolved from the
   credential. The change is a new field plus a new parameter (or a second call next to `:579`);
   one call site. The static guard checks that the seam is called, not what is passed to it.
2. **`recordIssueBudgetInFlightTasks` scans and writes in one call** (13.1). With 10.6 the
   reparent path should also skip the scan when the root is `off`.
3. **`transact` has no place for a level statement or an assertion today** without changing the
   pinned statement lists (12.3).

**Step 4 tests that later steps must change.** 10.13 lists two of them, under 5c. The others are
not in 10.13; the compiler will not find them, and a failing exact list is easy to "fix" by
pasting the new output, so they are worth naming in the step briefs:

| Step | Test | Why it must change |
| --- | --- | --- |
| 5b | S `…TransactionsFollowTheLockOrder`, the `Enable` list (`issue_budget_test.go:1704-1719`) | The list is exact. 5b adds the binding lock and moves `ListIssueBudgetInFlightTasks` before `SELECT clock_timestamp()`. The new list must be written from 10.10, not copied from the output |
| 5b | S `…SetLimitToTheCurrentLimitIsRefused` (`:1433`) | 10.1: a no-op is now "limit and reserve both unchanged". The same limit with another reserve must be accepted |
| 5b | S `…OriginalKeyStandsForTheWholeIntent` (`:1506`) | 10.1 puts `attempt_reserve_ticks` into the digest. The test walks the digest fields one by one; without a case for the reserve the new field is unpinned (the kind of gap D3 closed). Every digest value changes; there is no golden vector (N6) |
| 5b or wherever the level is settled | S `…TransactionsFollowTheLockOrder`, all five lists | A level statement or an assertion in `transact` adds a statement to each (12.3) |
| 5c | H `…ConcurrentWritersKeepEveryTreeBound` (`internal/handler/issue_budget_concurrency_test.go:248`) | It asserts 0 observations and performs `SetMode off` then `observe` on every tree (`:159`, `:162`). 10.6 makes each such return write one `observation_off` |
| 5c | S `…BindingNeverTouchesARootRow` (`:1758`), S `…TransactionsFollowTheLockOrder` | The two adjustments 10.13 already lists |
| 7 | S `…TransactionsFollowTheLockOrder`, all five lists | Design 12.1 (B41) adds a membership read inside each write operation, before the replay lookup and, by rule 3, before the root lock |
| any | S `…WriterInventoryIsComplete` | It pins the seam function names and the 12 call sites. A renamed seam or a new caller of one of the eight queries needs an entry there. That is its purpose |

### 13.3 Requeue and wakeup after a refused claim (design 10.10)

You asked whether my reading of the same code disagrees with yours. **It agrees.** Nothing here
is step 4 code; this is a reading, not a run.

Server. `RequeueTaskAfterClaimFailure` (`internal/service/task.go:3872`) sends one compare-and-set
statement (`pkg/db/queries/agent.sql:946-963`: back to `queued`, matched on `dispatched_at` and
`claim_generation`, only while `dispatched` and not started), then calls `notifyTaskAvailable`
(`:3885`, `:6978`). That one runs `EmptyClaim.Bump` (`:6995`) before `Wakeup.NotifyTaskAvailable`
(`:6999`), and the comment above it states that the order is deliberate. An "empty" verdict is
written only where the candidate query returned nothing (`:3692` single claim, `:4081` batch),
each against a version sampled before the query (`:3680`, `:4044`). A claim that found a task
and was then refused writes none.

Daemon. Wakeups enter `taskWakeups` (capacity 256, `internal/daemon/daemon.go:2064`). The poll
loop turns each into a non-blocking send on the capacity-1 channel `wakeup` (`:4781`, `:4814-4817`,
`signalPollerWakeup` at `:4961-4966`). The poller takes it in `sleepWithContextOrWakeup` (`:4955`,
`internal/daemon/helpers.go:95`) or while it waits for a task slot (`:5011`). A wakeup that
arrives during a claim request is therefore kept, and the next round starts when the request
returns.

Four additions, mine:

1. Without a wakeup connection the retry waits for the poll interval ("task wakeup websocket
   unavailable; polling fallback remains active", `internal/daemon/wakeup.go:72`). The refused
   task is not lost; it is late by up to one interval.
2. When the compare-and-set matches nothing (another handler reclaimed the row), the function
   returns an error and sends no wakeup (`:3879-3881`). Correct, and worth knowing for the 5b case
   that pins this path: assert on the row, not on the wakeup.
3. **The retry is not throttled.** While a binder holds the exclusive lock, or queues for it,
   each lap is: the claim, the refused finalize, the requeue statement, `ReconcileAgentStatus`
   (`:3883`), a `task:queued` broadcast to the workspace's clients (`:3884`), a cache bump, a
   wakeup and one info log line; the daemon comes back as fast as its HTTP round trip. The loop
   lasts as long as the binding transaction. For `Enable` that is short (25 to 35 ms of server
   time on the 3,000-issue tree, section 7.3). For reparent rule 2 it is the rest of the update
   transaction, including any lock that transaction waits for after the seam, and 10.10 itself
   says it may wait for a task row there. I would have 5b measure laps per refused claim in the
   lock-queue case of 10.9, and decide then whether the empty answer should carry a short delay.
4. `RequeueTaskAfterClaimFailure` has nine call sites in `internal/handler/daemon.go` (`:1890`,
   `:1900`, `:1927`, `:2111`, `:2260`, `:3167`, `:3304`, `:3447`, `:3612`). I did not map them to
   the two finalize call points of 10.2 (section 17).

## 14. Impact radius

The dispatch asked for an impact radius by levels L1 to L5. The repository defines no such scale,
so the five levels below are my own reading of it.

| Level (my definition) | What step 4 touches | How I know | What is left |
| --- | --- | --- | --- |
| **L1** lines changed inside existing functions | 23 lines added, none removed, in four functions: `IssueService.CreateInTx` (+5, `internal/service/issue.go:577-581`), `updateIssueAtomicallyOnce` (+7, `internal/handler/issue.go:4116-4122`), `deleteIssuesAndCollectAttachmentURLs` (+5, `:5023-5027`), the `EditIssue` closure (+6, `internal/service/collaboration_edit.go:122-127`). Plus my own +5 in `manage` (D1) | `git diff --stat -- server`: 171 insertions, 0 deletions in 7 tracked files; the other 148 lines are step 3 (models, `sqlc.yaml`, `cmd/migrate`, one test) | Each seam returns its error to the caller. A failing budget statement now fails the issue write it sits in |
| **L2** new files | `internal/service/issue_budget.go` (665 lines: `Enable`, `Close`, `Reopen`, `SetLimit`, `SetMode`, four readers), `issue_budget_binding.go` (281 lines: three seams), `pkg/db/queries/issue_budget.sql` (18 queries) with its generated file | Read in full; 128 mutants (section 4) | N1, and the notes of 2.2 |
| **L3** callers and the packages that link them | The eleven entry points behind the four functions: C1 to C8, P1, P3, D1 (5.2). On a tree without a root, which is every tree in production today (N13): a create without a parent sends nothing more; a create with a parent sends one indexed lookup; a parent change one lookup (none when the parent is cleared); a delete one `UPDATE` per issue that matches no row. No existing path takes a new lock at step 4, so there is no new wait and no new edge in the lock graph (9.3) | 35 of the 85 packages of the module link an edited package in their build or test build (`go list -test -deps`). Run green: 30 of them (15.4), among them `internal/service`, `internal/handler`, `cmd/server`, `internal/daemon`, `cmd/multica`, `internal/scheduler` and the six channel packages; 2 have no tests | Three packages that link only `pkg/db/generated`, where the change is additive (one new file, 111 added model lines), were not run: `cmd/migrate`, `internal/migrations`, `cmd/backfill_codex_usage_cache`. They build and pass `go vet` (section 17) |
| **L4** other layers | None at step 4. No route, request field or response field: the service has no handler until step 7, so installed desktop clients, the mobile app and the CLI see no difference. No daemon protocol field, no WebSocket event, no TypeScript, no built-in skill text. The schema is step 3's | `git status`: no file outside `server/` belongs to step 4; the three edited product files contain only the lines above | Step 5b adds the first daemon-visible field (`budget_attempt_id`) |
| **L5** system and deployment | (a) Order: the binary needs migrations 866 to 880. `/readyz` and `/healthz` answer 503 `out_of_date` until the ledger has them (`cmd/server/health.go:24`, `:175`); a new binary that serves anyway fails loudly on every delete and on every create with a parent. (b) Mixed versions: an old instance has no seams (N2). (c) The server's default isolation level (N1). (d) Rollback: clean at step 4, because no root can exist; once step 7 exposes `Enable`, a rolled-back binary recreates (b). (e) Load: the statements under L3; `Enable` holds the workspace fence for its walk and inserts (7.3) | Read of the readiness code; experiment 12.3; measurements in sections 7 and 8 | N1 and N2 are the two points that reach this level |

## 15. Commands, exit codes and counts

Everything ran from `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server` with
`TMPDIR='/private/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/'` exported. Toolchain: go1.27.0
darwin/arm64 against a module that says `go 1.26.6`; PostgreSQL 17.11 in `multica-postgres-1`,
amd64 under emulation.

**Databases.** The template every run was cloned from:

```text
docker exec multica-postgres-1 psql -U multica -d postgres -c "CREATE DATABASE f3_budget_s4c_tmpl TEMPLATE f3_budget_base_865"
DATABASE_URL='postgres://multica:multica@127.0.0.1:5432/f3_budget_s4c_tmpl?sslmode=disable' go run ./cmd/migrate up
```

The log of the second command ends with `up  880_issue_budget_ready` and `Done.`. Each DB-backed
run below cloned that template into a database of its own (`f3_budget_s4c_run_<tag>`), ran one
selection and dropped the clone. One run at a time. No test connected to `f3_budget_base_865`.

**How a run is counted.** `go test … -count=1 -v > <log> 2>&1`, then `echo $?`, then `grep -c` for
`^--- PASS`, `^--- FAIL` and `^--- SKIP` on the log. No run was piped into `tail`. Logs:
`/tmp/f3-s4c/runs/<tag>.txt`.

### 15.1 The final state

The last write of mine to the worktree was at 2026-11-18 05:12:24 (section 16). Every run in this
subsection is later than the last change to anything it compiled.

**One thing changed afterwards, and not by me:** at 12:17:40 you replaced `internal/budget/` with
the 5a package (section 16). Every run in the two tables below compiled against the 12-file
version that was in the tree until then. Six files outside the package import it, all of them
budget files (list in 15.5); the build, the vet and the selections that cross it were run again
on the new one, with the same counts: 15.5. The full-package runs were not repeated (section 17).

| Check | Command | Exit | Output |
| --- | --- | --- | --- |
| Build | `go build ./...` | 0 | empty |
| Vet | `go vet ./...` | 0 | empty |
| Vet, the tagged daemon files | `go vet -tags knowledgeintegration ./internal/daemon/` | 0 | empty |
| Format | `gofmt -l` on the 12 Go files of section 16 | 0 | empty |

These four were repeated at 10:19, after the last test run.

| Tag | `go test` arguments (before `-count=1 -v`) | Under the CLI guard | Exit | PASS / FAIL / SKIP | Package line |
| --- | --- | --- | --- | --- | --- |
| `guard_green` | `./internal/service/ -run '^TestIssueBudgetWriterInventoryIsComplete$'` | no | 0 | 1 / 0 / 0 | `ok … 1.246s` |
| `final2_svc_budget` | `./internal/service/ -run TestIssueBudget -timeout 20m` | no | 0 | 31 / 0 / 0 | `ok … 7.092s` |
| `final2_svc_budget_race` | the same with `-race -timeout 30m` | no | 0 | 31 / 0 / 0, no `DATA RACE` | `ok … 12.401s` |
| `final_hdl_budget` | `./internal/handler/ -run <handler budget selection> -timeout 20m` | no | 0 | 26 / 0 / 0 | `ok … 4.519s` |
| `final_hdl_budget_race` | the same with `-race -timeout 30m` | no | 0 | 26 / 0 / 0, no `DATA RACE` | `ok … 6.413s` |
| `final2_svc_full` | `./internal/service/ -timeout 40m` | no | 0 | 1644 / 0 / 18 | `ok … 910.187s` |
| `final_hdl_full` | `./internal/handler/ -timeout 40m` | no | 0 | 3028 / 0 / 53 | `ok … 170.066s` |
| `g_svc_full` | `./internal/service/` | yes | 0 | 1644 / 0 / 18 | `ok … 1276.649s` |
| `g_hdl_full` | `./internal/handler/` | yes | 0 | 3028 / 0 / 53 | `ok … 208.810s` |
| `g_quick` | `./pkg/db/generated/ ./pkg/dbid/ ./internal/iteration/ ./internal/testutil/` | yes | 0 | 59 / 0 / 0 | four `ok` lines |
| `g_scheduler` | `./internal/scheduler/` | yes | 0 | 18 / 0 / 0 | `ok … 2.174s` |
| `g_integrations` | `./internal/integrations/channel/engine/` and the `slack`, `lark`, `dingtalk`, `telegram`, `wecom` packages | yes | 0 | 1169 / 0 / 3 | six `ok` lines |
| `g_cmd_server` | `./cmd/server/` | yes | 0 | 477 / 0 / 9 | `ok … 65.189s` |
| `g_daemon` | `./internal/daemon/` | yes | 0 | 905 / 0 / 2 | `ok … 75.190s` |
| `g_cmd_multica` | `./cmd/multica/` | yes | 0 | 490 / 0 / 0 | `ok … 3.184s` |
| `g_rest` | the 16 tested packages that link only `pkg/db/generated` and need no database of their own name (list in 15.4), plus the 2 without tests | yes | 0 | 422 / 0 / 24 | 16 `ok` lines, 2 × `[no test files]` |

The handler budget selection is the regular expression

```text
TestIssueBudget|TestDeleteMemberIssueKeepsBudgetRows|TestWorkspaceDeletion|TestDeleteIssue|TestBatchDeleteIssues
```

"Under the CLI guard" means the repository's own entry point:
`bash scripts/go-test-with-agent-cli-guard.sh -- go test -p 1 -parallel 2 <packages> -count=1 -v -timeout 40m`.
It puts a sentinel for each of the 26 agent command names first on `PATH` and exits 1 with
`unexpected agent CLI invocation` when one is executed. All nine guarded logs contain 0 such
lines. The budget selections and the first two full runs went through plain `go test`; that is why
I repeated both full packages under the guard at the end (`g_svc_full`, `g_hdl_full`, 09:47 to
10:12), with the same counts.

Against the implementation report's counts (1635 / 0 / 18 and 3027 / 0 / 53): 9 more service tests
and 1 more handler test, all mine (section 3). The known `artifact_authority_busy` flake did not
occur in any run.

### 15.2 Skips

None is a database skip, and no budget test is skipped. The reason printed before each
`--- SKIP`, tallied from the logs:

| Run | Skips | Reasons |
| --- | --- | --- |
| `internal/service` (both full runs) | 18 | 18 × `REDIS_TEST_URL not set` |
| `internal/handler` (both full runs) | 53 | 46 × `REDIS_TEST_URL not set`; 2 × opt-in `MULTICA_RUN_I1_P95=1`; one each: the wire fixture export, `the catalog ships no HTTP recipes`, `MULTICA_TRIAGE_PERF=1`, `MULTICA_RUN_SKILLS_SH_INTEGRATION=1`, the 500-branch stub |
| `g_integrations` | 3 | 3 × `REDIS_TEST_URL not set` |
| `g_cmd_server` | 9 | 6 × `requires explicit Linux namespace integration environment`; `requires isolated REDIS_TEST_URL`; `opt-in M1 browser fixture`; `dedicated coordinator-owned stage harness only` |
| `g_daemon` | 2 | `requires real Linux kernel containment`; `windows-only behaviour` |
| `g_rest` | 24 | 23 × `REDIS_TEST_URL not set`; `MULTICA_TEST_REDIS_URL not configured` |

### 15.3 Runs that are evidence, not verification of the final state

| Tag | What | Exit | PASS / FAIL / SKIP |
| --- | --- | --- | --- |
| `negrev_red` | D1's test against the delivered `issue_budget.go` | 1 | 1 / 1 / 0 |
| `negrev_green` | the same after the fix | 0 | 2 / 0 / 0 |
| `rr_svc`, `rr_hdl` | the two budget selections on a `repeatable read` default (12.3) | 1 and 1 | 27 / 3 / 0 and 24 / 2 / 0 |
| `rrm_rr`, `rrm_rc` | the throwaway management replay test on that default, and its control (12.3) | 0 and 0 | 1 / 0 / 0 each; the test logs, it does not judge |
| `act_window` | the throwaway test for the activation sample (13.1), two subtests | 0 | 1 / 0 / 0 |
| mutation batches | 128 mutants and the identity control, each through `-overlay` on a fresh clone (section 4) | 1 per killed mutant, 0 for the control | in 4.2 to 4.4 |
| guard cases | 18 changed copies of the sources and the unchanged control (4.5) | 1 each, control 0 | in 4.5 |
| `rrd_noseam_rr`, `rrd_asis_rr` | 12:42, after your rulings: `TestDeleteIssueIterationSeesJoinAfterFenceWait` alone on a `repeatable read` default, without and with the tombstone seam, one log line added (12.3) | 1 and 1 | 0 / 1 / 0 each; cause `40001` in both |
| `rrd_noseam_rc` | its control on the built-in default | 0 | 1 / 0 / 0 |
| `rre_rr`, `rre_rc` | 13:15: the throwaway test of the real delete handler racing `Enable`, on that default and its control (12.3) | 0 and 0 | 1 / 0 / 0 each; the test logs, it does not judge |

The throwaway tests (`/tmp/f3-s4c/act/`, `/tmp/f3-s4c/rrm/`) were compiled into the service
package through `-overlay` and never existed in the worktree. The same holds for the last five
runs: the two changed copies of `internal/handler/issue.go` are in `/tmp/f3-s4c/fin/rrd/`, the
throwaway handler test in `/tmp/f3-s4c/fin/rre/`. Those five ran on two databases of their own,
`f3_budget_s4c_rrt` (cloned from `f3_budget_base_865`, migrated to 880) and `f3_budget_s4c_rrd`
(cloned from it per run); both scripts dropped both and printed `left behind: 0`. They compiled
against the new `internal/budget`.

### 15.4 Which packages, and how I chose them

`go list -test -deps` for each of the 85 packages of the module (`/tmp/f3-s4c/revdeps.sh`): 35
link `internal/service`, `internal/handler` or `pkg/db/generated` in their build or test build.

- Run green at the final state, 30 of them. The 14 of the runs above: `internal/service`,
  `internal/handler`, `cmd/server`, `internal/daemon`, `cmd/multica`, `internal/scheduler`, the
  six channel packages, `internal/iteration`, `pkg/db/generated`. And the 16 of `g_rest`, all of
  which link only `pkg/db/generated`: `cmd/backfill_issue_last_activity`, `internal/auth`,
  `internal/collaboration`, `internal/daemonws`, `internal/integrations/composio`,
  `internal/integrations/ghsnapshot`, `internal/issueguard`, `internal/issuestatus`,
  `internal/logger`, `internal/metrics`, `internal/middleware`, `internal/projecthealth`,
  `internal/realtime`, `internal/reportsnapshot`, `internal/seatcapacity`, `internal/util`.
- No test files, 2: `cmd/backfill_task_usage_hourly`, `internal/admission`.
- Not run by me, 3: `cmd/migrate`, `internal/migrations`, `cmd/backfill_codex_usage_cache`
  (section 17). `implement.md` records your runs of the first two: `./cmd/migrate/...`
  67 / 0 / 1 and `./internal/migrations/... -run TestIssueBudget` 3 / 0 / 0.
- `g_quick` also ran `pkg/dbid` and `internal/testutil`, which link none of the three.

`internal/daemon` and `cmd/multica` were missing from every earlier run of this step, mine
included, until the reverse-dependency list showed that both link `internal/service`.

### 15.5 After `internal/budget` was replaced

You copied the 5a package into this tree at 12:17:40 (21 files where there were 12; section 16).
Importers of that package outside itself, from `grep` on the import path: one product file,
`internal/service/issue_budget.go`, and five test files, `internal/service/issue_budget_test.go`
and the handler tests `issue_budget_delete_test.go`, `issue_budget_queries_test.go`,
`issue_budget_schema_test.go`, `issue_budget_teardown_test.go`. `IssueBudgetService` has no
production caller (N13); it is constructed in three test files only. The seven test files that
import the package or construct the service hold 45 top-level tests (30 in the service package, 15
in the handler package), and every one of them matches one of the two budget selections, so those
selections are where code of the package can run.

Re-run on the tree as it stood at 13:57 (`/tmp/f3-s4c/fin/run-post.sh`; a fresh
`f3_budget_s4c_tmpl` cloned from `f3_budget_base_865` and migrated, log ending in
`up  880_issue_budget_ready` and `Done.`; one clone per selection; all dropped at the end):

| Check | Command | Exit | Output |
| --- | --- | --- | --- |
| Build | `go build ./...` | 0 | empty |
| Vet | `go vet ./...` | 0 | empty |
| Vet, the tagged daemon files | `go vet -tags knowledgeintegration ./internal/daemon/` | 0 | empty |
| Format | `gofmt -l` on the 12 Go files of section 16 | 0 | empty |

| Tag | `go test` arguments (before `-count=1 -v`) | Exit | PASS / FAIL / SKIP | Package line |
| --- | --- | --- | --- | --- |
| `post_guard` | `./internal/service/ -run '^TestIssueBudgetWriterInventoryIsComplete$'` | 0 | 1 / 0 / 0 | `ok … 1.359s` |
| `post_svc_budget` | `./internal/service/ -run TestIssueBudget -timeout 20m` | 0 | 31 / 0 / 0 | `ok … 7.610s` |
| `post_svc_budget_race` | the same with `-race -timeout 30m` | 0 | 31 / 0 / 0, no `DATA RACE` | `ok … 13.204s` |
| `post_hdl_budget` | `./internal/handler/ -run <handler budget selection> -timeout 20m` | 0 | 26 / 0 / 0 | `ok … 3.881s` |
| `post_hdl_budget_race` | the same with `-race -timeout 30m` | 0 | 26 / 0 / 0, no `DATA RACE` | `ok … 5.458s` |

The counts equal those of 15.1 and those you record for your own re-run after the copy. The script
took the hashes of section 16 before and after: no file of the tree changed during the run. These
five went through plain `go test`, not through the CLI guard. The same 57 tests ran under the
guard inside `g_svc_full` and `g_hdl_full` (15.1), on the old package, with 0 guard hits.

## 16. Files and databases after my work

`shasum -a 256`, taken after the last test run of 15.1 and taken again at 13:21, 13:43, 13:57 and
13:59:00, before and after the runs of 15.5: the same 13 values each time. Every value equals the
one recorded when the file was last written. Mutation testing and the throwaway tests went through
`go test -overlay`, so neither ever wrote into the worktree.

**This section describes the tree as it stood at 13:59:00 on 2026-11-18. It has moved since, and
not through me.** At 15:02 my next reading showed step 5b at work in the same worktree:
`migrations/866_issue_budget.up.sql` was written at 14:48:49 and `pkg/db/queries/issue_budget.sql`
at 15:01:36 and again at 15:01:49 (9,523 bytes then, 9,173 in the table below). Nothing in this
report covers a change made after 13:59:00, and I ran nothing on the tree after that time.

**Step 4 files** (paths under the worktree's `server/`; times are local, 2026):

```text
sha256                                                            bytes   written         file
7e89b859a0c0c46cba8f5aee14ec5367fbe7b24b96019e0494bfaca0999766f1   27998  11-18 01:40:50  internal/service/issue_budget.go                    changed by me (D1)
1051237eb61c758333c2b4d97418819d15374c0efa48a91ad48d6acb7747b838   10850  11-17 07:10:51  internal/service/issue_budget_binding.go
bff7630b400ba472063030b87b598f0bb5bd1f7f9e5e2d7249e89f9fd4cdacc0   92750  11-18 01:37:04  internal/service/issue_budget_test.go               changed by me
b47ea055363dadae5eaf63e8586e1a79eb81c4ac18c49b328dfba84d5a8da1d9    9401  11-18 05:12:24  internal/service/issue_budget_writers_test.go       new, mine
f5c093a6b9ed2424650654dc849d3ccab0021b99929b3ba1022cac89fbaf28e3   42579  11-17 07:24:03  internal/service/issue.go
3d38a34c4148beaa9de28eab775cbe05787f5972ac9c52f4dc6043df4217ff46    7648  11-17 07:26:05  internal/service/collaboration_edit.go
9cbf723c6c7a32fbd8c162dd61be1c8249f197a193b108eb2c5d601c0e68b43d  223746  11-17 07:26:05  internal/handler/issue.go
a2690bc59a5820f94398214d6fa46b65a7d8e4e4f42aa646977e6aedb6ee67a7   27920  11-18 00:05:02  internal/handler/issue_budget_binding_test.go       changed by me
2456c5a24fc4c27c5acce93271e1480cdb1ed6819fad13acf564dd5fe36cde80    8758  11-17 07:38:58  internal/handler/issue_budget_delete_test.go
98b648c9dbb05648b312f14aa0217e72b815c799194a6a838a385727cdcc63bf    4138  11-17 05:21:34  internal/handler/issue_budget_teardown_test.go
b01d7e3aea2d1e2e3b148ed8b0fed0723393e829ecf5beae60c7db24a1cfaced   11049  11-18 01:12:01  internal/handler/issue_budget_concurrency_test.go   new, mine
1af100040ea5fcb67616a23f579e32c815cf910d146402768b90835fe38742c5    9173  11-17 06:41:57  pkg/db/queries/issue_budget.sql
cec3944dca0c3546e3369d46c4fd639376af64511a3c6e5b0ce805f0d52b55c6   29377  11-17 07:44:23  pkg/db/generated/issue_budget.sql.go
```

The three delivered files I changed, as they were delivered (copies in `/tmp/f3-s4c/pristine/`):

```text
045d5c23622b91116601658981bd7f7a907ef96f7dbda7312584cc707ebc0ab6  internal/service/issue_budget.go
8df1d32fd8bbc418c0741b24db00037ded2d099784bd107a09eed78b9a9fd83e  internal/service/issue_budget_test.go
e504fa5b29a24cebb4affc3f83b60f1fb2fe5c73bc0a51cd7e68e2f2f263e989  internal/handler/issue_budget_binding_test.go
```

The other eight delivered files have the hash I first recorded for them (seven at 2026-11-17
15:00, the teardown test at 17:35). In the whole worktree the files written since 2026-11-17
08:00, which is before the check was dispatched, are exactly the five marked above
(`find . -type f -newermt '2026-11-17 08:00:00'`, `.git` and `node_modules` excluded).

**Not touched by me: `internal/budget/`. Replaced by you at 12:17:40.** Two versions matter to
this report.

The version every run of 15.1 and every mutation run compiled against: 12 files written between
11-16 21:19:52 and 11-17 00:08:39. `implement.md` says you kept a copy in
`/tmp/f3-main-s5a/v3/f3-budget-before/`.

```text
6db8462df4606ab587b1ab9e50a8b5ee0eade386927bb5893e9f80ebc3a5f26a  decimal.go
2c95daf7225b4142b29926f45344c4aea10cab0bbd9718fd1c6f91071acb4db4  decimal_test.go
8ff64ed04f05cd43425406ef2c94760f9d1f2b43eb129b02b3cc590c77745320  edge_cases_test.go
9b65677fda4765f87e4322422863199620b46e57bbd44737e19f05c1b267066a  invariants_test.go
43f872b9f4d45e61e7f8981389b1d2c26af1668e996fdd9c017500fa856cfde7  ledger.go
4f1922425ba50ddfb77f7e8a0ccabdac41625b25b099b0e13788e79bff2cdc98  ledger_test.go
6f966865a33f70b83a217d89bd33e30f7a28615c706fd4f8da957ce7d7f70c33  model.go
a2b4cd68128b4877d39620b61419973b0f6dec0d254e683c837b86125379b31a  price.go
b57dd9df6dd8d072ac2a50f762cc294a684b00e29e81b8b4c225cfbc12005646  price_test.go
aaac212c787f403c89670c0dbb18287380440244d561badada6f23f7e82473c2  scale_test.go
90e000c435e29e47029abf1d05900c12387c004becd118749d7c3b99250ed2d9  usage.go
e323602f135a9d73f87fc561c60b8c5dde732ce1a4c50abc983cc8099e4399a2  usage_test.go
```

The version that is in the tree now, and that 15.5 and the last five runs of 15.3 compiled
against: 21 files, all with the modification time 11-18 12:17:40. Six of the 12 above are
byte-identical in it (`decimal.go`, `decimal_test.go`, `price.go`, `price_test.go`,
`scale_test.go`, `usage.go`), six changed, nine are new. Their concatenation in sorted name order
hashes to `a22f4121834bfc297b4a617fb5688b2bc95f5a4b34926019847bdb05cc73fd15`. I read none of it
(section 17).

```text
cdfcd6643f4dcd514cc60c9da352e686ef31a4cbad934991104163185bb459da  bench_test.go          new
6db8462df4606ab587b1ab9e50a8b5ee0eade386927bb5893e9f80ebc3a5f26a  decimal.go
2c95daf7225b4142b29926f45344c4aea10cab0bbd9718fd1c6f91071acb4db4  decimal_test.go
f5218993e3ad238afe980bc05c8464e0c222dc17746b52b894eb0af25c503776  edge_cases_test.go     changed
565687875a8a2acbca72a7869191318e3da4becd7c2ce775982455efccb825c0  hold_test.go           new
def6475f37df70f09d368ecda25324227dc659153302e9a1763631924cbebc80  holdset_test.go        new
a581a2fa5f375fb23c3d5a32e6c78c582835fc87e589e4892f7bf7df0c1b037d  invariants_test.go     changed
a6a7cf5e1afaa22f45653e56ec844f74c4aaf80eed68072c6a78523f554b148d  ledger.go              changed
1031daac8b542512784b9be6ac79469d968a1d5a8f0daa4991de9b8d80fff249  ledger_test.go         changed
6b7dafbc841d5a36e60567d13c09fcf1635aaff5d5c35270f1b3da38fc9d3cf2  model.go               changed
a2b4cd68128b4877d39620b61419973b0f6dec0d254e683c837b86125379b31a  price.go
b57dd9df6dd8d072ac2a50f762cc294a684b00e29e81b8b4c225cfbc12005646  price_test.go
1959ca56583c0cc92781a04c8dc41db508306b4125a0746af854c78772129f67  readonly_test.go       new
589f1a2f13fc292cb342e564cb099073730bdc23a1042beb7ffb9f3dfbc67ea9  reservation_test.go    new
aaac212c787f403c89670c0dbb18287380440244d561badada6f23f7e82473c2  scale_test.go
b54a82608c00c8a555fcb1365179870d96d587fe23f718126ab10f7b280768cd  stored.go              new
b216c299a68fe2aac7cb559823e2fc3b32d65d0728ed8c4a7826033e5e0873bc  stored_test.go         new
fa062fe047e52bd402db0e14373b0b237d7c6f2ae6072cb990c7259efe01b531  unsized_test.go        new
90e000c435e29e47029abf1d05900c12387c004becd118749d7c3b99250ed2d9  usage.go
fcbd12daff38b47225e2be496f9eae2123621aecc8a917cde6dfb39d98dbd97a  usage_test.go          changed
425bf91e48a686d96d32d13f97af937ef59bdcb24d6dfa0ea569c673e7daa6d8  with_test.go           new
```

**Not touched by me: migrations 866 to 880.** 30 files, all untracked, written between 11-17
02:14:09 and 05:24:51, so none is in the `find` list above; none renamed or renumbered. Their
concatenation in sorted name order hashes to `082fb2ea66b51adb00e7603551a83111eb67a9273449b76587f7df5ff2ae7c0c`
(first taken 11-18 08:31, the same at 13:59:00). I did not record a digest of them at the start of
the check; the modification times are the proof for the hours before that. After 13:59:00 the 5b
work rewrote `866_issue_budget.up.sql` (14:48:49, 12,262 bytes); the digest I read at 14:56 is
`f9fc3ac549c235a56e6b07597fa68856d663719d0ceccb15feee635a0f931596`. Still 30 files, same names.

**Worktree.** `git status --porcelain` prints 52 lines: the 30 migration files, 7 modified tracked
files and 15 other untracked entries, two of which are my new files. (Still 52 at 15:04: the new
files of `internal/budget/` sit inside one untracked directory entry.) HEAD
`86fea651cc5d09360327b6cf0f8357c1f360070d`, branch `codex/f3-budget-root-20261116`. I committed,
stashed, reset and pushed nothing and switched no branch. No `zz_*` file and no test binary of
mine is in the tree. Nothing was written to the shared checkout except this report.

**Databases.** One database of mine is left, kept on purpose; you have ruled to keep it
(`implement.md`). At 14:56 `SELECT datname FROM pg_database WHERE datname LIKE 'f3%'` returns two
names:

| Database | Whose | State |
| --- | --- | --- |
| `f3_budget_base_865` | yours | 19 MB. Used by me only as the `TEMPLATE` of a `CREATE DATABASE`: by hand for `f3_budget_s4c_main` and `f3_budget_s4c_tmpl` at the start, and once per run by three scripts, for `f3_budget_s4c_rrt` in the two evidence scripts (12:41, 13:15) and for the second `f3_budget_s4c_tmpl` of 15.5 (13:57). No other script of mine names it, and nothing of mine ever connected to it |
| `f3_budget_s4c_plan` | mine, **kept on purpose** | 136 MB, migrated to 880: 22 workspaces (20 with 15 agents each, 2 without agents), 65,031 issues, 60,023 queue rows of which 23 are in flight, 57,000 member rows and no root row, no probe index (counted again at 15:10, no session connected). It is the data set of sections 7 and 8 and of the two queries of 5.4. Since 11:00 I ran only the two `SELECT`s of 5.4 on it. Its seed scripts, `/tmp/f3-s4c/plan/load.sql` and `members.sql`, do not survive a reboot. Its schema is the 866 to 880 of 13:59:00: a clone of it does not have what 5b has changed in `866_issue_budget.up.sql` since |

What I created and dropped. The proof for all of them is the listing above: no other
`f3_budget_s4c_%` name exists.

- At 11:00, by hand, `DROP DATABASE IF EXISTS <name>` without `FORCE`, no session connected to
  any `f3%` database, each answering `DROP DATABASE`: `f3_budget_s4c_main`, `f3_budget_s4c_tmpl`,
  `f3_budget_s4c_plan1`.
- By the script that created them (`WITH (FORCE)`, on my own clone only): every per-run clone
  (`f3_budget_s4c_run_<tag>`, `f3_budget_s4c_rr`, `f3_budget_s4c_mut`), the two of the evidence
  scripts (`f3_budget_s4c_rrt`, `f3_budget_s4c_rrd`, each created and dropped twice, 12:41 and
  13:15), and the second `f3_budget_s4c_tmpl` with its four `f3_budget_s4c_run_post_*` clones
  (13:57 to 13:59).

`f3_budget_s4v_tmpl`, which my listing of 11:09 showed, was yours (`implement.md` names it as the
template of your own re-run); it is gone from the listing and I never connected to it or dropped
it.

To drop the one I kept:

```text
docker exec multica-postgres-1 psql -U multica -d postgres -c "DROP DATABASE IF EXISTS f3_budget_s4c_plan"
```

## 17. What I did not verify

**Outside step 4, by the plan.**

- The auth half of B14 and every HTTP, CLI and permission rule of design section 12 (step 7).
  `IssueBudgetService` has no production caller yet.
- B07 and B08 beyond "no step 4 code runs on a retry or a reclaim" (step 5b).
- Design sections 10 and 11 as implementations. I checked only that the step 4 code does not
  stand in their way (13.2).
- `internal/budget`: I read neither the version that was in this tree until 12:17:40 nor the one
  you copied over it (section 16). Against the new one I ran only what 15.5 lists: build, vet and
  the two budget selections.
- Anything written into the tree after 13:59:00 on 2026-11-18. Step 5b is changing step 4 files
  there now (section 16). My verdict is about the tree of section 16, not about the tree you see
  when you read this.
- Migrations 866 to 880 beyond what item 7 asked for: no foreign key, every index
  `CONCURRENTLY` in a file of its own.

**Not run.**

- **Nothing of mine ran on Go 1.26.** Every command of section 15 used the machine's go1.27.0;
  `go.mod` says `go 1.26.6` and CI builds on 1.26.x. You found this and re-ran build, vet and the
  budget selections on go1.26.9 (`implement.md`); the whole packages have not run on 1.26 at all.
  When I was ready to repeat 15.5 on 1.26.9 (15:02) the 5b work had begun to change the tree, so
  a run would no longer have described step 4, and I did not start it.
- The two whole packages after the `internal/budget` copy: 15.5 repeats only the budget
  selections. The counts 1644 / 0 / 18 and 3028 / 0 / 53 belong to the tree with the old package.
- `cmd/migrate` and `internal/migrations` (step 3's tests; the second builds and runs the migrate
  binary), and `cmd/backfill_codex_usage_cache` (its integration test creates databases under
  names I cannot prefix). All three build and pass `go vet`. `implement.md` records your runs of
  the first two.
- The 50 packages that link none of the edited packages: built and vetted only, apart from
  `pkg/dbid` and `internal/testutil`.
- `make check`, `pnpm typecheck`, `pnpm test`, Playwright, desktop, mobile. Step 4 has no file
  outside `server/`.
- Anything that needs Redis: `REDIS_TEST_URL` was unset in every run (the skips of 15.2).
- A real daemon, a real agent CLI, a provider call: none, as instructed.
- `-race` on whole packages: only on the two budget selections.
- The delivered suites against the 18 guard cases of 4.5: only the guard ran against them.

**Measured only in part.**

- Query plans: one server (PostgreSQL 17.11, amd64 emulated on arm64) and synthetic data, up to
  65,031 issues and 300,000 queue rows. No production-sized data and no other server version.
- `Enable` on the 3,000-issue tree: its statements were timed in SQL (7.3); no `Enable` call on
  that tree was timed through Go.
- The stress test: 12 trees of eight issues per run. Finalize and the usage entry do not exist
  yet, so the half of the lock graph they add is not exercised (4.4).
- The mixed-version window (N2): reasoned from the code. I built no binary from before step 4 to
  run it. That covers both halves: the child an old instance creates under a member, and the
  member issue an old instance deletes without the tombstone (5.4).
- A `repeatable read` default: the cases of 12.3. Of the management operations only `Close` ran
  there; `Reopen`, `SetLimit` and `SetMode` share `manage` with it. Of the silent forms, `Enable`
  against a create and `Enable` against the delete of a child were run; a move into the tree
  racing `Enable`, and the delete of the root issue itself, are by reasoning from the same
  statements. The statement that fails with `40001` in the unrelated delete test was read, not
  logged (12.3, class 3).
- The loop of section 18, R4 (a claim refused again and again under a wrong default): read from
  the code, not run. Finalize does not exist yet.

**Not proven.**

- That no queue row pairs an agent of one workspace with an issue of another (N4).
- What `default_transaction_isolation` a deployed instance has (12.3). I can see the shared
  development container only.
- The call-graph check does not model reflection, `unsafe`, `go:linkname` or cgo. The static
  guard does not see method values, concatenated SQL, triggers or migrations (5.3, 12.2).
- Which of the nine call sites of `RequeueTaskAfterClaimFailure` belong to the two finalize call
  points of 10.2 (13.3).
- The implementation report line by line. Where it and this report differ, this one states what
  I ran or read.

**The design as I read it.** `design.md` in full at 127,137 bytes (written 2026-11-18 08:27:58);
sections 11 and 12 only where they touch step 4. After that by `diff` against the copy I had
read, three times: 133,959 bytes (11:16:33), 135,070 (12:48:02) and 136,752 (14:51:59), and
`implement.md` at 47,509 (12:31:39) and 51,374 (14:51:59). Every changed line that answers this
report was checked against the code or against a run (section 18). The change of 12:48 (a 5b rule
for `runInTx` without a transaction, 10.2) is not about step 4 and I did not review it. A change
to either file after 14:51:59 is not reflected here.

## 18. After your rulings

You ruled on the eleven points of 0.2 on the morning of 2026-11-18 (`design.md` as written at
11:16:33, and `implement.md`) and ticked step 4. This section is what I did and found after that,
so that you can read it without diffing the whole report. The verdict of section 0 did not change.

### 18.1 Your rulings, checked against the code

I read every line of `design.md` and `implement.md` that answers this report (17, last
paragraph). All of them state the facts as I found them:

| Where | What it now says | Checked against |
| --- | --- | --- |
| design section 5, rule 5 | Four REPEATABLE READ transactions without READ ONLY, the fourth being `readCollaborationStageEvidence` (N18) | 12.2 |
| design section 5, supplement | `transact` states READ COMMITTED in 5b; the other openers keep the assertion; the server default is a deployment precondition (N1) | 12.1, 12.3 |
| design section 6 | No switch; enable only after every instance has the seams; both queries of 5.4 after an upgrade or a rollback (N2) | 5.4 |
| design 9.4 | A negative `expected_policy_revision` is `invalid_request`, decided before any statement (D1) | `internal/service/issue_budget.go:557-561`; the test also asserts 0 transactions and 0 statements |
| design 10.2 | Two entries pass a parent into a quick-create task; both run `validateAssigneePair` and `GetAgentInWorkspace`; `enqueueQuickCreateTask` itself checks nothing (N19) | `internal/handler/issue.go:2708`, `:2720`, `:2813`; `internal/handler/source_context.go:813`, `:817`, `:887`; `internal/service/task.go:1648` |
| design 12.7 | `IssueBudgetService` is built with the pool; inside a savepoint the level statement is refused or has no effect | Reproduced on PostgreSQL 17.11: in a savepoint of a REPEATABLE READ transaction, `ERROR:  SET TRANSACTION ISOLATION LEVEL must not be called in a subtransaction`; in a savepoint of a READ COMMITTED transaction it is accepted and changes nothing |
| design 10.13, what step 4 leaves to 5b, 5c and step 7 | Five statement lists; the guard kept; the index not added, with the figures of 8.3 | `internal/service/issue_budget_test.go:1690-1753`; 8.3 |

`research/step4-reachability/` holds `main.go`, `go.mod` and `go.sum` byte-identical to my
originals in `/tmp/f3-s4c/cg/` (`main.go`: sha256
`15b5b1f849d441d66ddf4c467c8caea826cff2f645a16490be5556b917a14477`).

### 18.2 Found after the rulings

**R1. "Deleting an iteration returns 500" was my ambiguity; it is an issue delete, and step 4 has
no part in it.** Corrected in 12.3 (class 3) and measured: the cause is `40001`, with and without
the tombstone seam (`rrd_asis_rr`, `rrd_noseam_rr`). You have since corrected design section 5
and `implement.md` (14:51:59); the new sentence matches the test source and both runs.

**R2. The reconciliation needs a second query.** The anti-join of 5.4 finds a missed binding and
cannot find a missed tombstone. An instance from before step 4 misses the tombstone on every
delete of a member issue; for the root issue that leaves the root unmarked, and `Reopen` is no
longer refused with `root_deleted`. Query, plan and times are in 5.4. Adopted in design section 6
(14:51:59); the two timings there are the ones of 5.4.

**R3. A third silent form under a wrong default: a delete racing `Enable`.** Measured with the
real handler (`rre_rr`, 12.3): 204, the issue gone, the member row not tombstoned. The 5b
assertion has nothing to assert on, because the tombstone is an `UPDATE` that reads no row first.
Adopted as the third residual case of design section 5, with your ruling not to pay a round trip
per delete for it. One nuance on the sentence that no later write will be loud: that holds for the
deleted issue. The tree itself turns loud at the next seam that does find a member row, as in the
other two cases; what stays silent for good is this one row.

**R4. Under a wrong default a refused claim never stops being refused. A reading, not a run:
finalize does not exist yet.** The supplement of design section 5 counts on that state being
loud: every claim that reads a member row fails and is requeued again and again. 13.3, point 3,
describes one lap of that loop and that nothing throttles it: the requeue statement,
`ReconcileAgentStatus`, a `task:queued` broadcast to the workspace's clients, a cache bump, a
wakeup and a log line (`internal/service/task.go:3872-3890`), and a daemon whose sleep after a
failed or short claim is ended at once by that wakeup (`internal/daemon/daemon.go:4892-4902`,
`:4955`). No counter and no delay limits it: the requeue statement
(`pkg/db/queries/agent.sql:946-963`) only puts the row back.

In the lock-queue case of 10.9 the loop ends with the binding transaction. In this case it ends
when somebody changes the server default. Until then every queued task of a budgeted tree spins
at the speed of a round trip, on every daemon that holds a wakeup connection. Two things I did
not read and 5b can measure: how many laps per second that is, and whether a task that is always
refused keeps the head of its runtime's queue, so that tasks behind it wait too. The product
already loops like this when one of the nine existing causes persists, so the pattern is not
new; the way into it, through configuration, is.

This bears on the decision you kept for yourself in 10.13 (a short delay after a refused claim):
the measurement planned there covers the bounded case only. Options, yours: measure this case as
well (a scratch database with `ALTER DATABASE … SET default_transaction_isolation` gives it for
free); or have an isolation failure answer the claim without the requeue's wakeup, so that it
falls back to the poll interval; or accept it, since such a deployment is outside the
precondition.

**R5. 22 and 20 are both right.** `implement.md` quotes 22 survivals from 4.2 (service 19,
handler 3). Those are runs per suite. As distinct mutants that no delivered suite killed it is 20;
D3 is about the 20, and 4.2 now says how the two relate.

**R6. `internal/budget` was replaced after my final-state runs.** Everything in 15.1 compiled
against the 12-file package; you copied the 21-file one over it at 12:17:40. I repeated build,
vet, the guard and the two budget selections on the new one at 13:57, plain and with `-race`:
the same counts, 31 / 0 / 0 and 26 / 0 / 0 (15.5). The whole packages were not repeated (17).

**R7. The tree moved again at 14:48:49, through step 5b.** Sections 15 and 16 describe the tree
of 13:59:00. `migrations/866_issue_budget.up.sql` and `pkg/db/queries/issue_budget.sql` have
changed since; I ran nothing after that and did not look at the changes. Two consequences for
you: the hashes of section 16 are the reference for "step 4 as checked", and a clone of
`f3_budget_s4c_plan` has the budget tables of that reference, not those of 5b.

**R8. Go 1.26.** Your finding, not mine: every run of this report used go1.27.0. I add only that
I could not repeat 15.5 on 1.26.9 any more, for the reason in R7 (17).

### 18.3 What changed in this report after your snapshot of 11:32

`implement.md` says you diffed this report at about 14:30 against a snapshot of 11:32 and names
the five runs of 12.3 and the second query of 5.4 as read. I do not know which of the later edits
that reading included, so here is every place that changed after 11:32, in the order of the
report. No verdict and no defect changed. Three statements of mine were corrected: the number of
statement lists in 0.2 point 4 (five, not two), the sentence about the iteration test in 12.3
(R1), and the count in D3, which now says how 22 and 20 relate (R5). Where an older statement
became untrue through a later event, the event is named next to it.

| Place | Change |
| --- | --- |
| 0.2 | Point 4 says five statement lists, not two. Point 11 and a pointer to this section |
| 2.1 D3, 4.2 | 22 survivals per suite against 20 distinct mutants (R5) |
| 2.2 N1, N2, N11 | Your rulings noted; the delete form (N1); the delete by an old instance (N2); the re-runs after the package copy (N11) |
| 5.4 | The second query, its plan and times, and what neither query lists |
| 12.3 | A second table with five runs; the delete form in class 2; the test and the measured cause in class 3; one more bullet on what the 5b assertion covers; your ruling noted above the four recommendations |
| 15.1, 15.3, 15.4 | The package copy named in 15.1; five evidence rows and their scratch locations in 15.3; who ran `cmd/migrate` and `internal/migrations` in 15.4 |
| 15.5 | New: build, vet and the budget selections on the new `internal/budget` |
| 16 | Stated as of 13:59:00. Both versions of `internal/budget`; the migration digest before and after 5b touched 866; the database table and the list of what was created and dropped |
| 17 | The new package, the moving tree, Go 1.26, the whole packages after the copy, what of 12.3 is reasoned and what is run, R4 as a reading; the versions of `design.md` and `implement.md` read |
| 18 | New |
