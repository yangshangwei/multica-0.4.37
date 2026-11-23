# Step 5d: tests of the finalize budget step (service half)

Lane "step 5d" of `.trellis/tasks/11-16-f3-parent-budget-root`. Brief: `/tmp/f3-main-s5d/brief-5d.md`.
Worktree: `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116` (nothing committed by this lane).

## 1. State

Complete. The two rules of the brief's section 3 (`member_deleted`, `reservation_overflow`) are in the code,
each written test-first. 30 tests named `TestIssueBudgetFinalize…` cover F01 to F22 (F19 by two existing
tests) and the rules of 3.3. They pass on go1.27.0, on go1.26.9 and with `-race`, and the whole
`internal/service` package passes with them: 1,708 tests, 0 failed, 18 skipped for a missing Redis. The 15
mutants of the brief are killed in all 17 variants; of 20 more, 19 are killed and one survives on a branch
no test can reach. Two items are open: one needs a ruling (section 6: a claim refused without a transaction
keeps its task token) and one case could not be built (section 7, item 1: the issue row missing inside the
transaction). Nothing is committed. The five databases of the lane are dropped.

## 2. The two code additions

Paths are under `server/internal/service/` of the worktree. Each was written test-first; the red and the
green log of each are in `/tmp/f3-s5d/logs/`. A `diff` of the two production files against the copies taken
when the lane started shows these hunks and nothing else.

### 3.1 `member_deleted`

| What | Where |
| --- | --- |
| Kind constant `issueBudgetObservationAttributionDifference = "attribution_difference"` | `issue_budget_binding.go:63` (the one constant; `gofmt` re-aligned the two above it) |
| Reason constant `issueBudgetReasonMemberDeleted = "member_deleted"` | `issue_budget_finalize.go:103-105` |
| The rule: `member` tombstoned and `member.IssueID != root.RootIssueID` gives one `attribution_difference` / `member_deleted` with an empty detail, through `observe`, not `refuse`, so it is not in `would_refuse` | `issue_budget_finalize.go:547-556`, directly behind the `root_deleted` switch (536-546) and in front of the limit block (557) |

- Red: `f11-before.log`. `TestIssueBudgetFinalizeAttributesAQuickCreateTaskThroughItsContextParent` failed in
  the case "the context names a member below the root, which was deleted": observations got `[]`, want
  `attribution_difference` / `member_deleted`.
- Green: `f11-after.log` (16 passed, 0 failed).

### 3.2 `reservation_overflow`

| What | Where |
| --- | --- |
| Reason constant `issueBudgetReasonReservationOverflow = "reservation_overflow"` | `issue_budget_finalize.go:96` |
| `overflowed` | `issue_budget_finalize.go:384-386` |
| The fallback: on `errors.Is(err, budget.ErrOverflow)` with a sized reserve, the reserve entry becomes unsized, the attempt's `reserved_ticks` NULL, the result `unsized` / `null`, one error log line (root, attempt, task, claim generation, declared reserve), and `appendEntries` is called once more with the same hold entries in front | `issue_budget_finalize.go:443-472` |
| The comment that the fallback relies on `appendEntries` asking the pure package before it sends a statement | `issue_budget_finalize.go:451-457` |
| The gap `coverage_gap` / `reservation_overflow`, detail `{"attempt_reserve_ticks":"<decimal>"}`, behind `ledger_unreadable` and `legacy_protocol` and in front of every judgement | `issue_budget_finalize.go:508-515` |

`this_reservation` and `this_reservation_ticks` of the `limit` detail (lines 592-593, not changed) read
`result.Reservation` and `reserved`, which the fallback sets to `unsized` and 0 (lines 463 and 461).

- Red: `f20-before.log`. Both F20 tests failed with `append budget entries: budget: amount exceeds the signed
  64-bit range`.
- Green: `f20-after.log` (18 passed, 0 failed).
- "Any other error of `appendEntries` fails the finalize as before" has a test of its own
  (`…DoesNotReserveWithoutAnAmountForAnotherFailureOfItsEntry`), added after an extra mutant survived
  (section 4, E7).
- A second `ErrOverflow` has no test and can have none: an unsized reserve adds no amount, and a hold moves
  an amount from reserved to held without changing their sum.

## 3. Tests

30 tests, all named `TestIssueBudgetFinalize…`, in three files under `server/internal/service/`:
`issue_budget_finalize_test.go` (helpers and the ordinary rows of a claim, 13 tests),
`issue_budget_finalize_edge_test.go` (claims off the ordinary path, 12 tests) and
`issue_budget_finalize_locks_test.go` (two-connection cases, 5 tests). Every test drives the real
`FinalizeTaskClaim` of a `TaskService` built by `claims(tx)` against the database; the golden-vector test
(F21) is the one that needs no database.

Shared comparisons, all spelled out in the helpers and not taken from the code under test:
`f.requireAttempt` (every column of the attempt row), `f.requireClaimOperation` (scope `attempt` / attempt
id, key `claim:<task id>:<generation>`, kind `attempt_open`, actor `runtime` / runtime id, the intent digest
recomputed from the spelled-out input, the result JSON key by key), `f.requireClaimEntry` (seq, entry id,
kind, attempt, source `finalize`, operation id, stored form, digest, receipt), `f.requireRootLedger`
(revision and the six figures), `requireIssueBudgetClaimObservations` (kind, reason, issue, task, attempt,
generation, detail map), `requireIssueBudgetStep` (the sqlc names of the statements behind `CreateTaskToken`,
in order). In the names below the prefix `TestIssueBudgetFinalize` is written `…`.

| Id | Test | Compared as a literal |
| --- | --- | --- |
| F01 | `…WritesTheAttemptItsReservationAndItsReceipt` | Attempt row (open, reserved 100, protocol 1, policy revision 1, epoch 1); entry `reserve/<attempt>` at seq 1; root at revision 1 with R=100; operation row and result JSON; no observation; one task token; the 11 statements of the step in order; one transaction; `BudgetAttemptID` equal to the row id; the cache at revision 0 before the commit and at revision 1 after it |
| F02 | `…ReservesWithoutAnAmountOnARootThatDeclaresNoReserve` | Unsized reserve entry, `reserved_ticks` NULL, result `unsized` / `null`; with a limit one `would_refuse` / `unbounded_unknown` with `{"ledger.reservation_unsized":"1"}`; without a limit no observation |
| F03 | `…CountsEveryTaskOfAnIssue` | Two tasks of one issue: two attempts on one root, nothing superseded, R=200 at revision 2 |
| F04 | `…SupersedesTheEarlierClaimOfATask`, `…SupersedesAnEarlierClaimThatReservedNoAmount` | Second and third claim of a task: the earlier attempt `superseded_unproven`; entry `hold/<earlier>/superseded_unproven` in front of the new reserve; R=100, H=100 then H=200; `superseded_attempt_ids` names the one attempt before; the unsized half with its figures |
| F05 | (i) `…ThatFailsInTheBudgetStepLeavesNothing`; (ii) `…RefusedForABindingInProgressLeavesNothing` | (i) unique violation `23505` on `issue_budget_attempt`; the 8 statements up to the failing one; budget tables unchanged; 0 tokens; the cache at revision 0; the next claim a first attempt. (ii) `errors.Is(err, ErrIssueBudgetBindingInProgress)` for a task of an issue and for a quick-create task while a reparent binder holds the lock (advisory locks read from `pg_locks`); the step is the try lock alone; 0 tokens; budget tables unchanged; the root not in the cache; the next two claims first attempts, entries at seq 1 and 2, R=200 |
| F06 | sequential `…JudgesTheLimitWithItsOwnReservation`; two connections `…JudgesTwoClaimsThatQueueOnTheRootLockOneAfterTheOther` | Sequential: limit 200 not refused, limit 199 refused, with the six detail keys; the same with A=30, R=100, H=70. Two connections: a third connection holds the root row; the first finalization waits behind the holder and the second behind the first (`pg_blocking_pids`); both stopped at `LockIssueBudgetRoot`; no attempt committed; after the release both return nil, both attempts written, exactly one `limit` observation (`reserved_ticks` 200, `limit_ticks` 150) on the second |
| F07 | `…RecordsEveryReasonARootWouldRefuseFor` | `root_closed`, `root_deleted`, `unbounded_unknown`, `limit` each alone and all four, one observation each in that order and the same order in `would_refuse`; a root without a limit records the first two only; every finalization returns nil |
| F08 | `…LeavesATreeWithoutARootAlone` | No budget row; `BudgetAttemptID` not valid; one token; two statements for a task of an issue (with and without a declared protocol), none for a task without an issue; no log line |
| F09 | `…OnALedgerThatCannotBeLoaded` | For each of the four causes (`stored_form`, `replay`, `sequence`, `projection`): no entry, root row untouched, attempt with `reserved_ticks` NULL, one `coverage_gap` / `ledger_unreadable`, result `none` with `ledger_unavailable` true; `root_closed` and `root_deleted` judged, `limit` not, `unbounded_unknown` with `"ledger":"unavailable"` on a root with a limit; the earlier attempt stays open; one error log line; after the repair the next claim closes both earlier attempts and holds only the one that had a reservation |
| F10 | `…RecordsADaemonWithoutTheProtocolAsAGap` | Not declared: `protocol_version` NULL, one `coverage_gap` / `legacy_protocol` with attempt, task and generation, and on a root with a limit `unbounded_unknown` with `"observation.legacy_protocol"` counting every uncovered gap; declared: version 1 and no gap |
| F11 | `…AttributesAQuickCreateTaskThroughItsContextParent`, `…LeavesAQuickCreateTaskThatNamesNoMemberAlone` | Attempt with `issue_id` NULL and `context_parent_issue_id` set, on the parent's root, step starting with `TryLockIssueBudgetBindingShared`; parent deleted: `root_deleted` alone when it is the root issue, `member_deleted` alone when it is another member (3.1), both when a deleted member sits below a deleted root issue; no parent, a parent outside every root, ids that do not parse: nothing written, with one warning for the ids |
| F12 | `…AttributesATaskRunFromAnotherWorkspaceToTheWorkspaceOfItsIssue` | Agent and runtime of another workspace: every row in the workspace of the issue (and of the context, for a quick-create task), nothing in the running workspace; R=200 after the two claims. No rule of `FinalizeTaskClaim` refuses such a claim before the budget step |
| F13 | `…NeedsReadCommittedWhereThereIsARoot` | REPEATABLE READ and SERIALIZABLE: `errors.Is(err, ErrIssueBudgetIsolation)` for a task of a member issue, a quick-create task and a task under a root switched off; the step ends at `GetIssueBudgetTransactionIsolation`; 0 tokens; tables unchanged. Outside every root: success, two statements or none, the level never asked. At READ COMMITTED the same claim is a first attempt |
| F14 | `…WithoutATransactionFailsTheClaimOfAMemberIssue` | `errors.Is(err, ErrIssueBudgetClaimNeedsTransaction)` for five claims (member issue, root issue, root switched off, two quick-create); the step is `GetIssue`, `GetIssueBudgetMember` (or the second alone); budget tables unchanged; four claims outside every root succeed with one token each; no log line |
| F15 | `…WritesNothingForARootSwitchedOff` | Four statements, no row, no attempt id; observed again, the next claim is a first attempt at policy revision 3, epoch 2 |
| F16 | `…RefusedWhileARootIsEnabledIsAttributedAtItsNextClaim`, `…ThatReadNoMemberMakesEnablingWaitAndIsRecordedAsAGap` | Order 1: `Enable` holds the binding lock and the fence, the finalization is refused at the try lock, the scan records `in_flight_legacy` for generation 1, the claim at generation 2 gets its attempt, the gap is listed as not covered and the claim's `unbounded_unknown` detail is `{"observation.in_flight_legacy":"1"}`. Order 2: the finalization holds the shared side alone, `Enable` waits at `LockIssueBudgetBinding`, a second finalization behind the waiting binder is refused, and after the commit `Enable` records both tasks as gaps (2 members, 2 gaps), no attempt |
| F17 | `…AndASetLimitOnOneRootCommitInTheOrderTheyLockIt` | Both orders, queued behind a held root row: both return nil. Claim first: reserved 100 at policy revision 1, no observation. Change first: reserved 70 at policy revision 2, `limit` observation with `reserved_ticks` 70, `limit_ticks` 50. In both: policy revision 2, epoch 1, ledger revision 1 |
| F18 | `…ThatDoesNotCommitPublishesNothing`, `…ServedFromTheCacheWritesWhatAFullReplayGives` | A commit that does not go through, on a root no claim has read and on one the cache holds: the 11 statements were sent, tables unchanged, 0 tokens, the cache still what was committed, the next claim a first attempt. Four claims in a row: the first three send no `ListIssueBudgetEntryFormsAfter`; after each, root row, cached ledger and a replay of the decoded entry rows equal the same literal figures; after `forget` the load is sent behind the root lock |
| F19 | none added | Pinned by `TestIssueBudgetEnableWaitsForAClaimThatReadUnbound` (`issue_budget_binding_lock_test.go:273`) and `TestIssueBudgetReparentWaitsForAClaimThatReadUnbound` (`:360`). Order 2 of F16 pins the same queue with real finalizations |
| F20 | `…ReservesWithoutAnAmountWhenItsReserveDoesNotFit`, `…KeepsTheHoldWhenTheReserveOfAReclaimDoesNotFit`, `…DoesNotReserveWithoutAnAmountForAnotherFailureOfItsEntry` | Reserve 5e18: the first claim sized, the second unsized with `reserved_ticks` NULL, result `unsized` / `null`, one `coverage_gap` / `reservation_overflow` with `{"attempt_reserve_ticks":"5000000000000000000"}`, the `limit` detail with `this_reservation` `unsized` and ticks 0, one error log line with its five fields, nil returned. At a re-claim the hold entry is kept in front of the unsized reserve. Another failure of the entry (`23505` on `issue_budget_entry`): the finalization fails, one entry statement sent, no log line, nothing written |
| F21 | `…IntentDigestMatchesGoldenVectors` | The two digests of the brief, over the input spelled out in the test |
| F22 | `…GoesOnWhenTheRootRowOfAMemberIsGone` | No attempt, nil returned, four statements, one error log line with root, issue, task and generation |
| 3.3 | `…WithoutATransactionGoesOnWhenTheIssueRowIsGone` | The issue row deleted between two statements of a finalization without a transaction: nil returned, no attempt, the step is `GetIssue` alone, one warning with task and issue |

## 4. Mutants

One at a time, with `/tmp/f3-s5d/mutants/mutate.py` (an exact textual replacement on a copy of the final
`issue_budget_finalize.go`; every anchor must occur exactly once) and `run-mutants.sh` (apply, run
`-run '^TestIssueBudgetFinalize'`, list the failing tests, restore the file, compare its SHA-256 with
`a7f2ccf18c3cecce74e7fd395fa2eebd95269470f78bc4562be8ec19b430e4a9`). Every mutant compiled, none timed out,
and the file had the recorded hash after each. Logs: `/tmp/f3-s5d/logs/mutant-<id>.log`;
`/bin/bash /tmp/f3-s5d/mutants/summarize.sh` prints the failing tests of each from the logs.

All 15 of the brief are killed. Mutants 1 and 5 were run in two variants each: the code has two try locks,
and "before this claim's reserve" can be read as the judgement alone or as the whole projection. "Failed /
run" counts top-level tests; 29 tests existed until the extra mutant E7 added the thirtieth.

| # | Mutation | Failed / run | Tests that failed |
| --- | --- | --- | --- |
| 1a | The try lock of a quick-create task always counts as acquired | 1 / 29 | `…RefusedForABindingInProgressLeavesNothing` |
| 1b | The try lock of a task of an issue always counts as acquired | 3 / 29 | `…RefusedForABindingInProgressLeavesNothing`, `…RefusedWhileARootIsEnabledIsAttributedAtItsNextClaim`, `…ThatReadNoMemberMakesEnablingWaitAndIsRecordedAsAGap` |
| 2 | An earlier attempt is closed but gets no hold entry | 7 / 29 | `…SupersedesTheEarlierClaimOfATask`, `…SupersedesAnEarlierClaimThatReservedNoAmount`, `…KeepsTheHoldWhenTheReserveOfAReclaimDoesNotFit`, `…ServedFromTheCacheWritesWhatAFullReplayGives`, `…OnALedgerThatCannotBeLoaded`, `…RecordsEveryReasonARootWouldRefuseFor`, `…JudgesTheLimitWithItsOwnReservation` |
| 3 | An earlier attempt gets its hold but is not closed | 5 / 29 | `…SupersedesTheEarlierClaimOfATask`, `…SupersedesAnEarlierClaimThatReservedNoAmount`, `…KeepsTheHoldWhenTheReserveOfAReclaimDoesNotFit`, `…OnALedgerThatCannotBeLoaded`, `…RecordsEveryReasonARootWouldRefuseFor` |
| 4 | `limit` judged with `>=` in place of `>` | 1 / 29 | `…JudgesTheLimitWithItsOwnReservation` |
| 5a | `limit` judged on the ledger before this claim's entries (the judgement alone) | 9 / 29 | `…JudgesTheLimitWithItsOwnReservation`, `…JudgesTwoClaimsThatQueueOnTheRootLockOneAfterTheOther`, `…AndASetLimitOnOneRootCommitInTheOrderTheyLockIt`, `…RecordsEveryReasonARootWouldRefuseFor`, `…ServedFromTheCacheWritesWhatAFullReplayGives`, `…OnALedgerThatCannotBeLoaded`, `…WritesNothingForARootSwitchedOff`, `…ReservesWithoutAnAmountWhenItsReserveDoesNotFit`, `…KeepsTheHoldWhenTheReserveOfAReclaimDoesNotFit` |
| 5b | The projection taken before this claim's entries (judgement, detail figures and unknown counts) | 10 / 29 | The nine of 5a and `…ReservesWithoutAnAmountOnARootThatDeclaresNoReserve` |
| 6 | The `legacy_protocol` observation left out | 3 / 29 | `…RecordsADaemonWithoutTheProtocolAsAGap`, `…RecordsEveryReasonARootWouldRefuseFor`, `…ReservesWithoutAnAmountWhenItsReserveDoesNotFit` |
| 7 | `root_deleted` judged on the claim's own member row | 3 / 29 | `…RecordsEveryReasonARootWouldRefuseFor`, `…AttributesAQuickCreateTaskThroughItsContextParent`, `…OnALedgerThatCannotBeLoaded` |
| 8 | `member_deleted` written also when the member row is the root issue's | 1 / 29 | `…AttributesAQuickCreateTaskThroughItsContextParent` |
| 9 | The operation key built without the claim generation | 22 / 29 | Every test in which a claim writes its operation row, each at the comparison of that row. The seven that passed write none |
| 10 | A ledger that cannot be loaded treated as mode `off` (nothing written) | 1 / 29 | `…OnALedgerThatCannotBeLoaded` |
| 11 | `Committed()` called inside the transaction, before the commit | 2 / 29 | `…WritesTheAttemptItsReservationAndItsReceipt` (the cache holds revision 1 while the transaction is open), `…ThatDoesNotCommitPublishesNothing` (the cache holds revision 1 after a lost commit) |
| 12 | The overflow fallback removed | 2 / 29 | `…ReservesWithoutAnAmountWhenItsReserveDoesNotFit`, `…KeepsTheHoldWhenTheReserveOfAReclaimDoesNotFit` |
| 13 | The workspace of a task of an issue taken from the runtime | 16 / 29 | `…AttributesATaskRunFromAnotherWorkspaceToTheWorkspaceOfItsIssue` on the attribution itself (the claim gets no attempt); the other 15 on the extra `GetAgentRuntime` in the statements of the step |
| 14 | The path without a transaction returns nil for a member issue | 1 / 29 | `…WithoutATransactionFailsTheClaimOfAMemberIssue` |
| 15 | The operation row written directly behind the attempt row, before the observations, so its `would_refuse` is empty | 17 / 29 | Nine fail first on the operation row, among them `…RecordsEveryReasonARootWouldRefuseFor`, `…JudgesTheLimitWithItsOwnReservation` and `…JudgesTwoClaimsThatQueueOnTheRootLockOneAfterTheOther`; eight fail first on the statement order, among them `…WritesTheAttemptItsReservationAndItsReceipt` |

Twenty more, on the two additions of this lane and on the rules of 3.3. Nineteen are killed; E20 survives:

| # | Mutation | Failed / run | Tests that failed |
| --- | --- | --- | --- |
| E1 | The `reservation_overflow` observation left out | 2 / 29 | Both sized-reserve F20 tests (`…ReservesWithoutAnAmountWhenItsReserveDoesNotFit`, `…KeepsTheHoldWhenTheReserveOfAReclaimDoesNotFit`) |
| E2 | `member_deleted` recorded as a `would_refuse` | 1 / 29 | `…AttributesAQuickCreateTaskThroughItsContextParent` |
| E3 | The attempt row keeps the sized reserve after the fallback | 2 / 29 | The same two F20 tests |
| E4 | `this_reservation_ticks` keeps the sized amount after the fallback | 2 / 29 | The same two F20 tests |
| E5 | The overflow logged below error level | 2 / 29 | The same two F20 tests |
| E6 | The result keeps `sized` after the fallback | 2 / 29 | The same two F20 tests |
| E7 | The fallback taken for any error of `appendEntries` | 1 / 30 | `…DoesNotReserveWithoutAnAmountForAnotherFailureOfItsEntry` |
| E8 | `member_deleted` judged on the root issue's member row | 3 / 29 | `…AttributesAQuickCreateTaskThroughItsContextParent`, `…OnALedgerThatCannotBeLoaded`, `…RecordsEveryReasonARootWouldRefuseFor` |
| E9 | `reservation_overflow` written behind the judgements | 2 / 29 | The same two F20 tests |
| E10 | The fallback sends the unsized reserve without the hold entries in front of it | 1 / 30 | `…KeepsTheHoldWhenTheReserveOfAReclaimDoesNotFit` |
| E11 | The isolation level not required | 16 / 30 | `…NeedsReadCommittedWhereThereIsARoot` on the missing error; the other 15 on the statements of the step |
| E12 | A root switched off is observed | 1 / 30 | `…WritesNothingForARootSwitchedOff` |
| E13 | A member row without its root row fails the finalization | 1 / 30 | `…GoesOnWhenTheRootRowOfAMemberIsGone` |
| E14 | The path without a transaction fails when the issue row is gone | 1 / 30 | `…WithoutATransactionGoesOnWhenTheIssueRowIsGone` |
| E15 | The `ledger_unreadable` observation left out | 1 / 30 | `…OnALedgerThatCannotBeLoaded` |
| E16 | The superseded attempt ids not put into the result | 7 / 30 | Both `…Supersedes…` tests, `…KeepsTheHoldWhenTheReserveOfAReclaimDoesNotFit`, `…ServedFromTheCacheWritesWhatAFullReplayGives`, `…OnALedgerThatCannotBeLoaded`, `…RecordsEveryReasonARootWouldRefuseFor`, `…JudgesTheLimitWithItsOwnReservation` |
| E17 | `root_closed` left out | 2 / 30 | `…RecordsEveryReasonARootWouldRefuseFor`, `…OnALedgerThatCannotBeLoaded` |
| E18 | The attempt of a quick-create task written with `issue_id` | 3 / 30 | `…AttributesAQuickCreateTaskThroughItsContextParent`, `…AttributesATaskRunFromAnotherWorkspaceToTheWorkspaceOfItsIssue`, `…RefusedForABindingInProgressLeavesNothing` |
| E19 | The path without a transaction does not look for the member row of a quick-create task | 1 / 30 | `…WithoutATransactionFailsTheClaimOfAMemberIssue` |
| E20 | The path with a transaction fails when the issue row is gone (`issue_budget_finalize.go:245-248`) | 0 / 30 | **None: survives.** No test can reach the branch (section 7, item 1) |

E7 survived its first run (29 passed, none failed): no test told the overflow apart from another failure of
the entry. Brief 3.2 says any other error fails the finalize as before, so a test was missing.
`…DoesNotReserveWithoutAnAmountForAnotherFailureOfItsEntry` was added and E7 run again; the log on disk is
the second run. The log of the first run was overwritten by it.

E20 was run last, to show what section 7 says: 30 passed, none failed, and the file was restored to the
recorded hash. Of the 37 mutants (17 for the 15 of the brief, 20 more) it is the one no test kills.

## 5. Verification

All of it ran on the final bytes of the five files. When the first run below started (19:28:44 local) the
production file had last been written at 19:24:39 (the restore after mutant E19, hash checked) and the
newest test file at 19:18:21. The static part was run again at 20:01, after the package run. Mutant E20 was
run after that; its restore wrote the same bytes back (hash checked again), and no file was edited since.
Every command ran in `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server` with `TMPDIR` set as the
brief says. Toolchains: `go version go1.27.0 darwin/arm64` (default) and `go version go1.26.9 darwin/arm64`
(`GOTOOLCHAIN=go1.26.9 GOPROXY=off`, from the module cache).

Final hashes (SHA-256):

- `issue_budget_finalize.go` `a7f2ccf18c3cecce74e7fd395fa2eebd95269470f78bc4562be8ec19b430e4a9`
- `issue_budget_binding.go` `d332fbf76b0900f102066a9a94ec67ab43eadf8d231f308849f196bc148ee122`
- `issue_budget_finalize_test.go` `c827ff07c300b52fc2e50758f11c7c122861ed872da0635735878e5da214d4b2`
- `issue_budget_finalize_edge_test.go` `b60932ac3ec14eee24ba66669ca687a89873d2c0f73d2f1fdaf960443182b848`
- `issue_budget_finalize_locks_test.go` `09e7f28476275ac656eb2969c74aaa84552687e52ff8ac3dea8a99adaeb4d248`

Static part (`/bin/bash /tmp/f3-s5d/verify-static.sh`, log `verify-static.log`):

| Command | Toolchain | Exit | Result |
| --- | --- | --- | --- |
| `gofmt -l` over the five files | go1.27.0 `gofmt` | 0 | Nothing listed |
| `gofmt -l` over the five files | go1.26.9 `gofmt` (the binary in the toolchain's `bin/`; `GOTOOLCHAIN` does not switch `gofmt`) | 0 | Nothing listed |
| `go build ./...` | go1.27.0 | 0 | No output |
| `go vet ./internal/service/ ./internal/handler/` | go1.27.0 | 0 | No output |
| `go build ./...` | go1.26.9 | 0 | No output |
| `go vet ./internal/service/ ./internal/handler/` | go1.26.9 | 0 | No output |

Test runs, each on a clone of `f3_budget_s5v_tmpl` made for that run and used by nothing else, one run at a
time. `run.sh` is `DATABASE_URL=postgres://…/<database> go test ./internal/service/ -run <pattern> -count=1 -v
<more> > <log> 2>&1`, followed by the exit code and the counts of `--- PASS`, `--- FAIL` and `--- SKIP` lines.
"Tests" are top-level tests; "with subtests" counts every `--- PASS` line.

| Command | Toolchain | Database | Exit | Tests passed (with subtests) | Failed | Skipped | Time |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `F3_DB=f3_budget_s5d_b /bin/bash /tmp/f3-s5d/run.sh verify-default '^TestIssueBudget' -timeout 20m` | go1.27.0 | `f3_budget_s5d_b` | 0 | 95 (169) | 0 | 0 | 19.6 s |
| `F3_DB=f3_budget_s5d_c GOTOOLCHAIN=go1.26.9 GOPROXY=off /bin/bash /tmp/f3-s5d/run.sh verify-go1269 '^TestIssueBudget' -timeout 20m` | go1.26.9 | `f3_budget_s5d_c` | 0 | 95 (169) | 0 | 0 | 21.6 s |
| `F3_DB=f3_budget_s5d_d /bin/bash /tmp/f3-s5d/run.sh verify-race '^TestIssueBudget' -timeout 30m -race` | go1.27.0, `-race` | `f3_budget_s5d_d` | 0 | 95 (169) | 0 | 0 | 30.4 s |
| `F3_DB=f3_budget_s5d_e /bin/bash /tmp/f3-s5d/run.sh verify-package '.' -timeout 30m` (the whole package) | go1.27.0 | `f3_budget_s5d_e` | 0 | 1,708 (4,102) | 0 | 18 | 1,067.6 s |

- In each of the four runs 30 of the tests passed are `TestIssueBudgetFinalize…`: all of this lane's, none
  skipped.
- The `-race` run printed no `WARNING: DATA RACE`.
- The 18 skips of the package run are the Redis-backed tests of `task_notify_test.go`,
  `task_batch_claim_test.go`, `reclaim_check_cache_test.go` and `empty_claim_cache_test.go`, each with
  `REDIS_TEST_URL not set`. None is a budget test; this lane starts no Redis.
- The package run had no failure, so nothing had to be proved pre-existing. Of the two failures the brief
  expects on this machine, `TestGitEnv*` is not in this package, and the sweeper tests of the package passed.
- Earlier evidence that the two-connection tests do not depend on timing: the five tests of the locks file
  eight times in a row, 40 passed, 0 failed (`locks1x8.log`).
- The last run of the lane, after the restore of mutant E20 and before the databases were dropped:
  `/bin/bash /tmp/f3-s5d/run.sh final-finalize '^TestIssueBudgetFinalize' -timeout 300s` on
  `f3_budget_s5d_a` (the used work database, not a fresh clone), go1.27.0: exit 0, 30 passed, 0 failed,
  0 skipped (`final-finalize.log`).

## 6. Needs a ruling, and deviations from the brief

### Needs a ruling

1. **A claim refused on a service without a `TxStarter` keeps its task token.** F14 of the brief says such a
   claim "writes nothing". The budget step writes nothing, but `FinalizeTaskClaim` creates the task token
   (`task.go:3841`) before it reaches the step (`task.go:3871`), and without a transaction nothing takes the
   token back. Measured with a scratch test mapped in through `go test -overlay` and never placed in the tree
   (`measure1.log`): the error is `budget attempt requires a transaction`, one row is left in `task_token`,
   and the task is still `dispatched` at generation 1. The statements up to the refusal were
   `GetAgentRuntime`, `GetAgentRuntime`, `LockTaskForClaimFinalization`, `LockIssueExecution`, `GetAgent`,
   `CreateTaskToken`, `GetIssue`, `GetIssueBudgetMember`. Production builds the service with a `TxStarter`
   (`internal/handler/handler.go:560`), so only tests reach the path. Refusing earlier means a check in
   `task.go` in front of `CreateTaskToken`, a file outside this lane. Smallest safe step taken: no code
   change; F14 pins the error, the missing attempt and the unchanged budget tables of the five refused claims,
   and pins their token count in neither direction. To rule: leave it as a path only tests reach, or refuse
   before the token is created.

Not a ruling, but a consequence of the outcome 3.3 told this lane to pin (F16, first order): the gap the
binder recorded for the refused generation is never covered in this slice. The count of uncovered gaps is
taken for the whole root (`CountIssueBudgetUncoveredGaps`, `pkg/db/queries/issue_budget.sql:300`), so every
later claim on that root, while it has a limit, records `unbounded_unknown` and counts that gap under
`observation.in_flight_legacy`.

### Deviations from the brief

1. **Three test files, not two.** The brief allows a second file past about 1,500 lines; the
   single-connection cases alone are 2,897 lines. The cases off the ordinary path (F09, F11, F12, F13, F14,
   F18, F20) are in `issue_budget_finalize_edge_test.go` (1,360 lines), the rest in
   `issue_budget_finalize_test.go` (1,537), the two-connection cases in
   `issue_budget_finalize_locks_test.go` (652).
2. **`requireIssueBudgetAttempt` became `f.requireAttempt`.** The helper inherited from the stopped lane
   copied `got.WorkspaceID` into its expectation, so it compared the workspace with itself. The method
   expects the fixture's workspace, which in F12 is the workspace of the issue and not the one the agent and
   the runtime belong to. No test used the old helper.
3. **Statement order is compared for the budget step, not for the whole transaction** (F01). The step is
   every statement behind the single `CreateTaskToken`; the helper fails when a budget statement comes before
   it or when there is not exactly one. The statements in front of the token belong to rules that are not this
   lane's and change with them. F01 also pins that the claim ran in one transaction.
4. **"Publishes nothing to the cache" is pinned as "nothing of the claim".** The load under the root lock
   publishes the committed ledger, in a transaction that later fails too. So F01, F05 (i) and F18 require the
   cache to hold exactly what is committed (revision 0, or the revision before the lost commit), and F05 (ii),
   where the refusal comes before any load, requires the root to be absent from the cache.
5. **F13 runs SERIALIZABLE as well as REPEATABLE READ.**
6. **F16, first order, holds the binder with a statement log and `beforeCommit`** around the real `Enable`,
   not with `inBindingHeld`: the test needs the binder to have run its scan before the finalization arrives,
   and to commit on a signal. `inBindingHeld` is used in F05 (ii), with a reparent binder.
7. **The error log line of 3.2 has a fifth field**, `attempt_reserve_ticks`, beside root, attempt, task and
   claim generation.
8. **Two tests beyond the list:** `…DoesNotReserveWithoutAnAmountForAnotherFailureOfItsEntry` (for the
   surviving mutant E7) and `…WithoutATransactionGoesOnWhenTheIssueRowIsGone` (rule 3.3, see section 7).
9. **Mutants 1 and 5 in two variants each, and 20 more** (section 4).

One precision of the brief's list of statements, not a deviation: the ledger load (step 5) is absent not
only when the cache holds the root's revision, but also on a cold cache when the root is at revision 0,
because there is no entry to read (`issue_budget_ledger.go:399`). That is why the first claim of F01 sends
11 statements and none of them is `ListIssueBudgetEntryFormsAfter`. F18 pins the load where it is sent: after
`forget`, on a root at revision 4, behind `LockIssueBudgetRoot`.

Brief and design: no disagreement was noticed in the ranges of the design this lane read (10.2 with its
supplements, 10.5, 10.9, 10.10, 10.12).

No existing test was changed, weakened, skipped or deleted. The five files of section 5 are the only files
of the worktree written since the lane started (`find -newer` against the copy taken at 13:03:09).

## 7. Not done

1. **Rule 3.3, "an ordinary task whose issue row is not found", in the path with a transaction.** The case
   cannot be built there; older tables and an earlier step of the same transaction forbid it:
   - `lock_issue_execution` (`migrations/650_collaboration_legacy_execution_fence.up.sql:42`, called for
     every claim at `task.go:3828`, before the token and the budget step) reads the issue of the task
     `FOR KEY SHARE NOWAIT`. A missing row ends the claim there as blocked. A row that is found stays locked
     until the commit, and a `DELETE` of it waits for that lock.
   - `agent_task_queue_issue_id_fkey` is `FOREIGN KEY (issue_id) REFERENCES issue(id) ON DELETE CASCADE`
     (`migrations/001_init.up.sql:130`), and the finalization of a task with a runtime locks its task row
     first (`LockTaskForClaimFinalization`, `SELECT … FOR UPDATE`, `pkg/db/queries/managed_admission.sql:4`).
     A delete of the issue that committed earlier took the task row with it; one that comes later waits at
     the cascade.

   So `TryLockIssueBudgetBindingSharedForIssue`, which reads the issue by primary key and nothing else,
   always finds its row, and the branch of the transaction path (`issue_budget_finalize.go:245-248`) has no
   test. A mutant of it survives (E20, section 4). Its twin in the path without a transaction (`:301-304`,
   the same `warnClaimIssueMissing`) is reachable, because another session can delete the issue between two
   statements, and is pinned by `…WithoutATransactionGoesOnWhenTheIssueRowIsGone` (mutant E14). A test could
   reach the first branch only by sending a `DELETE` of the issue through the finalization's own transaction;
   no code does that, so it would bend the case and was not written. The other half of the rule, a
   quick-create context whose ids do not parse, is pinned by `…LeavesAQuickCreateTaskThatNamesNoMemberAlone`.
2. **The token count of a claim refused without a transaction** (F14, "writes nothing"): not pinned; see
   section 6, "Needs a ruling".
3. **F19:** no test added, as the brief allows; the two existing tests are named in section 3.
4. **A second `ErrOverflow`** (3.2): no test, and none can exist (section 2).
5. **Outside this lane and not touched:** the handler half of the claim (step 5e), `internal/daemon/**`,
   `internal/budget/**`, migrations, query files. No accessor was needed in any of them.
6. **Not run, because the brief does not ask for it:** the whole package on go1.26.9 or with `-race`; any
   other package's tests. Nothing was committed.

## 8. Databases

All five were clones of `f3_budget_s5v_tmpl`, made with
`CREATE DATABASE <name> TEMPLATE f3_budget_s5v_tmpl`. No test was connected to the template or to any other
database; one DB-backed run at a time.

| Database | Used for | Dropped |
| --- | --- | --- |
| `f3_budget_s5d_a` | Every run while the tests were written, the 37 mutants, the last run | Yes |
| `f3_budget_s5d_b` | `^TestIssueBudget` on go1.27.0 | Yes |
| `f3_budget_s5d_c` | `^TestIssueBudget` on go1.26.9 | Yes |
| `f3_budget_s5d_d` | `^TestIssueBudget` with `-race` | Yes |
| `f3_budget_s5d_e` | The whole package | Yes |

Each `DROP DATABASE` answered `DROP DATABASE` with exit 0. A listing afterwards shows no database named
`f3_budget_s5d_%`, and the template still there. No other database was created, changed or dropped.

## 9. Hooks or reminders that asked for an edit outside this lane's files

None asked for one. What arrived and was not acted on:

- The Trellis SessionStart hook asked for a one-line acknowledgment in the first reply (given; no file is
  involved). The workflow it prints ends with a spec update, a journal entry and a commit. None was made.
- System reminders carried the brief of another lane (`/tmp/f3-main-s5e/brief-5e.md`, step 5e). It was not
  acted on; neither that lane's tree nor its databases were touched.
- The standing instructions of the session offer a memory directory under `~/.claude/projects/…/memory/`.
  Nothing was written there, and nothing under `~/.claude/**`.

STATUS: complete
