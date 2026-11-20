# Step 4 — budget root and member binding: implementation report

Date: 2026-11-17 (revised 13:55 +0800; the first version, written 11:59, is
replaced by this one). Worktree `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116`,
branch `codex/f3-budget-root-20261116`, HEAD `86fea651c`, 0 commits ahead.
Nothing committed, stashed or pushed. No migration was added or edited
(866–880 last modified 05:24, before this step started).
`server/internal/budget/` is untouched.

**Addendum, 15:36 +0800: section 13.** Sections 1–12 are the 13:55 text and
are unchanged. The decisions they ask for have since been made in `design.md`
and `implement.md`; 13.1 says where. 13.2–13.4 are new: one mismatch between
the design text and the step-4 code, the isolation levels §10.6 asks about,
and a note on stored timestamps.

## Status

- **Delivered.** Design §9 at the Go service layer: enable, member inheritance
  on create, reparent, delete tombstone, close / reopen / limit / mode, and the
  minimal reads. No handler endpoint, route, CLI, frontend, attempt,
  reservation or usage ingress. Nothing from §10 is implemented.
- **Exit rows.** B01–B13, B57, B59, B60, the service half of B14 and the
  updated B56 each have a named test (section 3).
- **Verification.** Build, vet (whole module) and gofmt are clean. The new
  tests pass on a fresh clone with nothing skipped (22 service, 25 handler);
  `-race` is clean on the service tests. Full `internal/service`: exit 0,
  1635 / 0 / 18, but only with a symlink-free `TMPDIR` (6.1). Full
  `internal/handler`: exit 0 and 3027 / 0 / 53 in 4 of 5 runs with the change.
- **One failure not fully closed (6.2).** The fifth handler run failed one
  delivery-evidence test on a table-lock conflict over the pull-request
  tables. Step 4 never names those tables, and the same failure appears with
  the change removed whenever such a lock is held. Who held it in that one run
  is not identified, so I am not calling it pre-existing.
- **Decisions wanted.** Section 8, items 1 (an interleaving that the new
  off-mode rule in §9.3 leaves uncovered), 2 (where the binding lock goes) and
  6 (whether the evidence in 6.2 is enough).
- **Design text.** `design.md` changed twice while this step ran. §9 as I
  re-read it at 13:14 (file time 12:28:03) already states the rules listed in
  section 7 as D1–D8; code and §9 agree on them.

## 1. Files changed

All paths are under `server/` in the worktree.

| File | Change |
| --- | --- |
| `internal/service/issue_budget.go` | New. `IssueBudgetService` (`Enable`, `Close`, `Reopen`, `SetLimit`, `SetMode`, `GetRootByIssue`, `GetRoot`, `GetMember`, `ListObservations`), `IssueBudgetError` and its codes, the intent digest, the operation record. |
| `internal/service/issue_budget_binding.go` | New. `BindIssueBudgetOnCreate`, `BindIssueBudgetOnReparent`, `TombstoneIssueBudgetMember`; they take the caller's `*db.Queries` and open no transaction. |
| `internal/service/issue.go` | +5 lines at :579, in `CreateInTx` after the insert: `BindIssueBudgetOnCreate`. |
| `internal/handler/issue.go` | +12 lines. :4116–4122, after `qtx.UpdateIssue`, only when `issue.ParentIssueID != current.ParentIssueID`: `BindIssueBudgetOnReparent` (the batch update reaches the same statement). :5025, in `deleteIssuesAndCollectAttachmentURLs` right before `qtx.DeleteIssue` (:5028): `TombstoneIssueBudgetMember` (single and batch delete share this loop). |
| `internal/service/collaboration_edit.go` | +6 lines at :122–127, after `UpdateCollaborationIssueStructure`, only when the parent changed: `BindIssueBudgetOnReparent`. |
| `pkg/db/queries/issue_budget.sql` | 8 queries appended: `GetIssueBudgetRoot`, `UpdateIssueBudgetRootPolicy`, `InsertIssueBudgetMembers`, `TombstoneIssueBudgetMember`, `ListIssueBudgetDescendants`, `ListIssueBudgetInFlightTasks`, `GetIssueBudgetOperationByKey`, `ListIssueBudgetObservations`. |
| `pkg/db/generated/issue_budget.sql.go` | Regenerated with sqlc v1.31.1. A second regeneration leaves it and `models.go` byte-identical (sha1 `1ec9ef2b…`, `fabf0395…`). |
| `internal/service/issue_budget_test.go` | New, 22 tests (1662 lines). |
| `internal/handler/issue_budget_binding_test.go` | New, 8 tests (508 lines). |
| `internal/handler/issue_budget_delete_test.go` | `TestDeleteMemberIssueKeepsBudgetRows` (B56) updated: the deleted child's member row now carries `tombstoned_at`; every other assertion is unchanged. |

`DeleteIssue` has one product caller (`internal/handler/issue.go:5028`). The
only other issue delete is the workspace teardown
(`pkg/db/queries/workspace_delete.sql:567`), which by §9.1 writes nothing.

Not mine, already modified by steps 1–3 and left as they were:
`cmd/migrate/main.go`, `internal/handler/workspace_delete_manifest_test.go`,
`pkg/db/generated/models.go`, `sqlc.yaml`.

Every green run in section 5 was made against these versions: the last source
edit is 07:38:58 and the earliest green run 07:56:14. The red runs used the
earlier states they name. First 16 hex digits of sha256:

```
045d5c23622b9111  internal/service/issue_budget.go
1051237eb61c7583  internal/service/issue_budget_binding.go
8df1d32fd8bbc418  internal/service/issue_budget_test.go
e504fa5b29a24ceb  internal/handler/issue_budget_binding_test.go
2456c5a24fc4c27c  internal/handler/issue_budget_delete_test.go
f5c093a6b9ed2424  internal/service/issue.go
9cbf723c6c7a32fb  internal/handler/issue.go
3d38a34c4148beaa  internal/service/collaboration_edit.go
1af100040ea5fcb6  pkg/db/queries/issue_budget.sql
cec3944dca0c3546  pkg/db/generated/issue_budget.sql.go
```

## 2. Eligibility predicate

An issue may become a root issue when both hold:

1. `admission.Formal(issue.AdmissionStatus)` — the admission status is
   `not_required` or `accepted`.
2. `IsCollaborationIssue(issue.ID)` is false.

Otherwise `Enable` returns `not_eligible` and writes nothing.

Evidence:

- `server/internal/admission/admission.go:1` — "Package admission owns the
  formal-work boundary independently of execution status"; `:13` —
  `func Formal(status string) bool { return status == "not_required" || status == "accepted" }`.
- `server/pkg/db/queries/collaboration.sql:86-87` — `IsCollaborationIssue` is
  `SELECT EXISTS (SELECT 1 FROM issue_collaboration WHERE issue_id = $1)`.
- `server/internal/service/delivery_completion.go:481-493` — `completionOrdinary`
  already defines "ordinary" as exactly these two checks, in this order. I
  reused the definition instead of writing a second one.
- Contract
  `.trellis/tasks/11-12-agent-squad-followup-roadmap/research/f3-minimum-budget-contract-20261113.md:17`
  — only ordinary tasks with provable execution attribution; independent
  collaboration / host runs are not automatically included child executions.
  `:147` — management needs edit permission on the "current ordinary root
  task".
- `design.md:252` now states the same predicate and the two consequences
  below. When I implemented it the mapping was left to the implementer.

Consequences:

- The predicate applies to the root issue only. Every descendant is bound
  whatever its kind, so a collaboration issue created or moved under a member
  gets a member row (C7, C8, S5 in B60 require exactly that). Membership is not
  attribution: which executions of such an issue get an attempt is step 5's
  rule.
- A pending-triage issue cannot be a root until it is accepted.

`IsCollaborationIssue` is the one statement in `Enable` without a
`workspace_id` filter. It runs only on the id that `GetIssueInWorkspace` just
resolved in the caller's workspace. Every budget query carries `workspace_id`.

## 3. Behaviour → test

Service tests are in `internal/service/issue_budget_test.go`, handler tests in
`internal/handler/issue_budget_binding_test.go` unless stated. The matrix is
`.trellis/tasks/11-12-agent-squad-followup-roadmap/research/f3-acceptance-matrix-20261113.md`.

| Row | Tests |
| --- | --- |
| B01 | `TestIssueBudgetEnableIsOnePerIssueAndReplaysByOriginalKey`; `TestIssueBudgetEnableConcurrentlyOnOneIssue` (`same_key_and_intent`, `same_key,_other_intent`, `other_key`) |
| B02 | `TestIssueBudgetEnableParentAndChildConcurrently`; `TestIssueBudgetEnableRefusesOverlappingRoots` |
| B03 | `TestIssueBudgetCloseAndReopenKeepTheSameRoot`; `TestIssueBudgetDeletedRootIssueKeepsTheSlot` |
| B04 | `TestIssueBudgetCreateInheritsTheParentRoot` |
| B05 | `TestIssueBudgetEnableRacingChildCreate` (`create_commits_first`, `enable_commits_first`, `either_order`) |
| B06 | `TestIssueBudgetEnableFreezesDescendantsAndRecordsInFlightTasks` (also the §9.5 covered judgement) |
| B07, B08 | `TestIssueBudgetTaskRetryAndReclaimLeaveTheBindingAlone` — binding half only; the amounts are step 5 |
| B09 | handler `TestIssueBudgetBusinessReopenWritesNothing` |
| B10 | handler `TestIssueBudgetMoveKeepsTheMemberRow`; service `TestIssueBudgetReparentNeverRewritesAMemberRow` |
| B11 | handler `TestIssueBudgetMoveIntoAnotherRootIsRecordedNotRefused`; service `TestIssueBudgetReparentNeverRewritesAMemberRow` |
| B12 | handler `TestIssueBudgetDeleteMemberLeavesATombstone`, `TestIssueBudgetBatchDeleteTombstonesEveryMember` |
| B13 | handler `TestIssueBudgetDeleteRootIssueTombstonesTheRoot`; service `TestIssueBudgetDeletedRootIssueKeepsTheSlot`. Not covered: later executions marked `would_refuse:root_deleted` (needs attempts, step 5) |
| B14 (service half) | `TestIssueBudgetIsScopedToTheWorkspace` |
| B57 | handler `TestIssueBudgetDeleteRootIssueTombstonesTheRoot`; `TestIssueBudgetEveryParentCapableWriterBinds/S4_DetachDirectChildIssues` |
| B59 | `TestIssueBudgetEnableWalksPastTenLevelsAndRefusesALoop` (`fifteen_levels`, `parent_loop`, `its_own_parent`); `TestIssueBudgetReparentThroughAParentLoopStillBinds` |
| B60 | handler `TestIssueBudgetEveryParentCapableWriterBinds` — subtests `C1 POST /api/issues`, `C2 POST /api/comments/{id}/sub-issues`, `C4 lifecycle handoff follow-up`, `S3 UpdateIssue` (single and batch), `S4 DetachDirectChildIssues`, `S6 PrepareWorkspaceDeletionLinks`; service `TestIssueBudgetCollaborationWritersBindMembers` — C7 `CollaborationService.Create`, C8 accepted stage proposal, S5 `UpdateCollaborationIssueStructure` through `EditIssue`. Each path is named in the assertion message and in the header comment of both tests. |
| B56 | `internal/handler/issue_budget_delete_test.go` `TestDeleteMemberIssueKeepsBudgetRows` |

Also covered, without a matrix row of their own:

| Rule | Tests |
| --- | --- |
| §5 statement order of every new transaction | `TestIssueBudgetTransactionsFollowTheLockOrder` |
| §5 rule 1 and the cost for a tree without a root | `TestIssueBudgetBindingNeverTouchesARootRow`; handler `TestIssueBudgetWritesNothingForATreeWithoutARoot` |
| §9.2 eligibility and invalid requests | `TestIssueBudgetEnableRequiresAnOrdinaryFormalIssue` |
| §9.3 rule 2 | `TestIssueBudgetReparentBindsAnUnboundSubtree` |
| §9.4 revision, replay, epoch | `TestIssueBudgetManagementRequiresTheCurrentPolicyRevision`, `TestIssueBudgetManagementReplaysByOriginalKey`, `TestIssueBudgetModeSwitchIssuesANewEpochOnEveryReturn` |

## 4. Lock order, statement by statement

The sequences below are asserted by `TestIssueBudgetTransactionsFollowTheLockOrder`
through a `TxStarter` wrapper that records every statement a transaction sends.

### 4.1 `Enable` (own transaction)

| # | Statement | Lock | Refusal decided here |
| --- | --- | --- | --- |
| 1 | `LockWorkspaceForChatSessionCreate` | workspace row `FOR KEY SHARE`, blocking | no row → `not_found` |
| 2 | `iteration.WorkspaceFenceSQL` | advisory `iteration:<ws>`, exclusive, blocking | — |
| 3 | `GetIssueBudgetRootByIssue` | none | a root exists → statement 3a, then the transaction ends |
| 3a | `GetIssueBudgetOperationByKey('issue', issue, key)` | none | same digest → stored result; other digest → `operation_conflict`; no row → `already_enabled` |
| 4 | `GetIssueInWorkspace` | none | `not_found` |
| 5 | `IsCollaborationIssue` (after `admission.Formal` in Go) | none | `not_eligible` |
| 6 | `GetIssueBudgetMember` on the issue | none | `root_conflict` |
| 7 | `ListIssueBudgetDescendants` | none | a loop → `parent_cycle`; a bound descendant → `root_conflict` |
| 8 | `SELECT clock_timestamp()` | none | — |
| 9 | `InsertIssueBudgetRoot` | the new row | — |
| 10 | `InsertIssueBudgetMember` (`root`) | new row | — |
| 11 | `InsertIssueBudgetMembers` (`activation`; skipped without descendants) | new rows | — |
| 12 | `ListIssueBudgetInFlightTasks` | none: a plain read of `agent_task_queue` and `agent` | — |
| 13 | `InsertIssueBudgetObservation`, one per in-flight task | new rows | — |
| 14 | `InsertIssueBudgetOperation` | new row | — |

Against §5: the fence is taken before the root row is inserted (rule 3). After
statement 9 only budget tables are written and no row lock is requested
(rule 2); statement 12 reads task tables without locking. Statements 9–11
cannot wait on a unique key, because every writer of those keys holds the
fence. Statement 3 precedes the issue read so that the slot stays taken, and a
replay still answers, after the root issue is deleted.

### 4.2 `Close`, `Reopen`, `SetLimit`, `SetMode` (own transaction each)

| # | Statement | Lock | Refusal decided here |
| --- | --- | --- | --- |
| 1 | `LockIssueBudgetRoot` | root row `FOR UPDATE`; the first and only lock | no row → `not_found` |
| 2 | `GetIssueBudgetOperationByKey('root', root, key)` | none | same digest → stored result; other digest → `operation_conflict` |
| — | compare `expected_policy_revision` in Go | — | `policy_revision_conflict` |
| 3 | `Reopen` only: `GetIssueBudgetMember` on the root issue | none | tombstoned → `root_deleted` |
| — | state rule in Go | — | nothing would change → `state_conflict` |
| 4 | `UpdateIssueBudgetRootPolicy` | the row already held | — |
| 5 | `InsertIssueBudgetOperation` | new row | — |

No fence, no issue, task, user, agent, workspace or runtime row is read or
locked (rule 2). A refusal sends no write.

### 4.3 Binding functions (inside the caller's transaction)

They add no lock request. The caller already holds the workspace `KEY SHARE`
and the fence: `CreateInTx` at `service/issue.go:389` and `:395`; the issue
update and delete handlers through `lockIssueWriteFences`
(`handler/issue.go:3868`, called at `:3947` for update and `:4884` for
delete); the collaboration workflow at `service/collaboration_workflow.go:486`
and `:513`.

| Function | Statements, in order |
| --- | --- |
| `BindIssueBudgetOnCreate` | no parent: nothing. Else `GetIssueBudgetMember(parent)`; if bound, `InsertIssueBudgetMember` (`create`). |
| `BindIssueBudgetOnReparent` | parent cleared: nothing. Else `GetIssueBudgetMember(new parent)`; if unbound, stop. `GetIssueBudgetMember(x)`. x bound, same root: stop. x bound, other root: one `InsertIssueBudgetObservation` (`would_refuse`/`root_conflict`) on x's root. x unbound: `ListIssueBudgetDescendants` → `InsertIssueBudgetMembers` (`reparent`: x and every unbound descendant) → one `InsertIssueBudgetObservation` (`would_refuse`/`root_conflict`, on the parent's root) per descendant bound to another root → one `coverage_gap`/`parent_cycle` if the walk met a loop → `ListIssueBudgetInFlightTasks` → one `coverage_gap`/`in_flight_at_bind` per task. |
| `TombstoneIssueBudgetMember` | one `UPDATE issue_budget_member SET tombstoned_at = now() … AND tombstoned_at IS NULL`. |

No statement names a root table, so rule 1 holds by construction;
`TestIssueBudgetBindingNeverTouchesARootRow` fails if one is added. For a tree
without a root the cost is one member lookup on create, one (or none) on
reparent, and one update that matches nothing on delete.

### 4.4 Refusal codes

`IssueBudgetError.Code`; nothing is written when one is returned.

| Code | Returned by | When |
| --- | --- | --- |
| `invalid_request` | all five, before any statement | an invalid workspace, target or actor id; a key that is empty, over 200 bytes, not UTF-8 or contains NUL; a reason that is empty after trimming, over 2000 bytes, not UTF-8 or contains NUL; a negative limit (zero is accepted, as the column check `limit_ticks >= 0` allows); a mode other than `off` or `observe` |
| `not_found` | all five and the reads | the workspace, issue or root is missing or belongs to another workspace; `GetMember` on an unbound issue |
| `not_eligible` | `Enable` | section 2 |
| `already_enabled` | `Enable` | the issue is, or once was, a root issue and the key was never used for it |
| `operation_conflict` | all five | the key was used with another intent |
| `root_conflict` | `Enable` | the issue is a member of another root, or a descendant has a member row |
| `parent_cycle` | `Enable` | the descendant walk meets a loop |
| `policy_revision_conflict` | the four management operations | `expected_policy_revision` is not the current one |
| `root_deleted` | `Reopen` | the root issue's member row is tombstoned |
| `state_conflict` | the four management operations | the request would change nothing |

## 5. Commands and results

Counts are top-level `--- PASS` / `--- FAIL` / `--- SKIP` lines. Outputs are in
`/tmp/f3-s4/`. Every database was cloned from `f3_budget_base_865` and migrated
to `880_issue_budget_ready` with `go run ./cmd/migrate up` (exit 0 each time).
One DB-backed suite ran at a time. The Postgres container did not restart
(`StartedAt` `2026-11-11T06:53:08Z` before and after).

```bash
cd /Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server
# DB-backed commands were prefixed with
#   DATABASE_URL='postgres://multica:multica@127.0.0.1:5432/<database>?sslmode=disable'
# and, where a table says "resolved", with
#   TMPDIR='/private/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/'
# Each ended with:  > /tmp/f3-s4/<file> 2>&1; echo "exit=$?"

B1  go build ./...
B2  go vet ./...
B3  gofmt -l <the 9 touched Go files>
B4  go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate

T1  go test ./internal/service/ -run 'TestIssueBudget' -count=1 -v
T2  go test -race ./internal/service/ -run 'TestIssueBudget' -count=1 -v
T3  go test ./internal/handler/ -run 'TestIssueBudget|TestDeleteMemberIssueKeepsBudgetRows|TestWorkspaceDeletion|TestDeleteIssue|TestBatchDeleteIssues' -count=1 -v
T4  go test -overlay /tmp/f3-s4/overlay-unwired.json ./internal/handler/ -run 'TestIssueBudget|TestDeleteMemberIssueKeepsBudgetRows' -count=1 -v

P1  go test ./internal/service/... -count=1 -v -timeout 40m
P2  go test ./internal/handler/... -count=1 -v -timeout 40m
P3  go test -overlay /tmp/f3-s4/overlay-handler-head.json ./internal/handler/... -count=1 -v -timeout 40m
P4  go test ./internal/budget/... -count=1 -v
P5  go test ./cmd/migrate/... -count=1 -v
P6  go test ./internal/migrations/... -run TestIssueBudget -count=1 -v

S1  go test ./internal/service/ -run '^(TestStageCompletion|TestStageExecution|TestStageExitContinuation|TestStageArtifactReport)' -count=1 -v -timeout 40m
S2  S1 with -overlay /tmp/f3-s4/overlay-service-head.json

H1  go test ./internal/handler/ -run '^TestDeliveryCompletionHintFencesPolicyAndStatusUntilCommit$' -count=<n> -v
H2  H1 with -overlay /tmp/f3-s4/overlay-handler-head.json
H3  go test ./internal/handler/ -run '^TestDeliveryCompletionHint' -count=10 -v
```

Static checks, re-run after the last edit:

| Command | Exit | Result | File |
| --- | --- | --- | --- |
| B1 | 0 | no output | `final2-build.txt` |
| B2 | 0 | no output; every package, test files included | `final3-vet-all.txt` |
| B3 | 0 | no file listed | `final2-gofmt.txt` |
| B4 | 0 | generated files unchanged | — |

Red runs (database `f3_budget_s4_dev`, default `TMPDIR`):

| Command | State of the code | Exit | PASS / FAIL / SKIP | File |
| --- | --- | --- | --- | --- |
| T1 | service methods stubbed | 1 | 0 / 22 / 0 | `red-service.txt` |
| T1 | service implemented, seams not wired | 1 | 18 / 4 / 0 | `red2-service.txt` |
| T4 | the three seam files replaced by their HEAD copies | 1 | 7 / 6 / 0 | `red-handler-unwired.txt` |

In the T4 run the 6 failures are five of my binding tests and the updated B56
test. The 7 passes are the four step-3 tests and three of mine that assert an
absence (section 12).

New tests on the fresh clone `f3_budget_s4_new` (default `TMPDIR`):

| Command | Exit | PASS / FAIL / SKIP | File |
| --- | --- | --- | --- |
| T1 | 0 | 22 / 0 / 0 (+9 subtests, none skipped) | `new-service.txt` |
| T3 | 0 | 25 / 0 / 0 (+12 subtests, none skipped) | `new-handler.txt` |
| T2 | 0 | 22 / 0 / 0, no `DATA RACE` | `new-service-race.txt` |

Regression:

| Command | `TMPDIR` | Database | Exit | PASS / FAIL / SKIP | File |
| --- | --- | --- | --- | --- | --- |
| P1 | default | `f3_budget_s4_svc` | 1 | 1590 / 45 / 18 (6.1) | `full-service.txt` |
| P1 | resolved | `f3_budget_s4_svc2`, fresh | 0 | 1635 / 0 / 18 | `full-service-realtmp.txt` |
| P2 | resolved | `f3_budget_s4_hdl`, fresh | 1 | 3026 / 1 / 53 (6.2) | `full-handler.txt` |
| P2 | resolved | `f3_budget_s4_hdl`, reused | 0 | 3027 / 0 / 53 | `full-handler-2.txt` |
| P2 | resolved | `f3_budget_s4_hdl3`, fresh | 0 | 3027 / 0 / 53 | `full-handler-3.txt` |
| P2 | resolved | `f3_budget_s4_hdl4`, fresh | 0 | 3027 / 0 / 53 | `full-handler-4.txt` |
| P2 | resolved | `f3_budget_s4_hdl5`, fresh | 0 | 3027 / 0 / 53 | `full-handler-5.txt` |
| P4 | default | none | 0 | 70 / 0 / 0 | `budget-pure.txt` |
| P5 | default | `f3_budget_s4_mig` | 0 | 67 / 0 / 1 | `cmd-migrate.txt` |
| P6 | default | `f3_budget_s4_mig` | 0 | 3 / 0 / 0 | `migrations-budget.txt` |

The same handler package with the change removed (evidence for 6.2):

| Command | `TMPDIR` | Database | Exit | PASS / FAIL / SKIP | File |
| --- | --- | --- | --- | --- | --- |
| P3 | resolved | `f3_budget_s4_ovl1`, fresh | 0 | 3018 / 0 / 53 | `full-handler-head-1.txt` |
| P3 | resolved | `f3_budget_s4_ovl2`, fresh | 0 | 3018 / 0 / 53 | `full-handler-head-2.txt` |
| P3 | resolved | `f3_budget_s4_ovl3`, fresh | 0 | 3018 / 0 / 53 | `full-handler-head-3.txt` |

`overlay-handler-head.json` replaces `handler/issue.go`, `service/issue.go` and
`service/collaboration_edit.go` with their `git show HEAD:` copies (sha256
equal to HEAD's) and maps `issue_budget.go`, `issue_budget_binding.go`,
`issue_budget_test.go`, `issue_budget_binding_test.go` and
`issue_budget_delete_test.go` to `""`. That removes 9 tests, hence 3018. The
eight appended sqlc queries stay compiled in; nothing calls them.

Skips, all named:

- Service, 18: every one prints `REDIS_TEST_URL not set`
  (`TestEmptyClaimCache_*` ×6, `TestReclaimCheckCache_*` ×6,
  `TestClaimTasksForRuntimes_*` ×2, `TestNotifyTask*` ×4).
- Handler, 53: 46 print `REDIS_TEST_URL not set`; `TestIterationI1P95` and
  `TestIterationI1DisableP95` (opt-in `MULTICA_RUN_I1_P95`);
  `TestFetchFromSkillsSh_AnthropicPptxIntegration`
  (`MULTICA_RUN_SKILLS_SH_INTEGRATION`); `TestTriageQueueScaleBaseline`
  (`MULTICA_TRIAGE_PERF`); `TestMcpMarketPublicHTTPRecipeLifecycle` ("the
  catalog ships no HTTP recipes"); `TestTriageWireFixtures` ("wire fixture
  export requested only by cross-package verification");
  `TestWebhookHandler_DBErrorOnTokenLookupReturns500`.
- `cmd/migrate`, 1: `TestCommentContentBigramRequirementMatchesRealPGBigm`
  ("Postgres does not provide pg_bigm").
- No budget test skipped in any run: 22 service and 13 handler budget tests
  pass in the full runs.

Databases: every `f3_budget_s4_*` database is dropped (`_dev`, `_new`, `_svc`,
`_svc2`, `_hdl`, `_hdl3`–`_hdl5`, `_ovl1`–`_ovl3`, `_mig`, `_vac`, `_vac2`).
`SELECT datname FROM pg_database WHERE datname LIKE 'f3_budget%'` returns only
`f3_budget_base_865` (checked 13:56). No experiment process or backend is left.

## 6. Failures seen in the regression runs

### 6.1 45 `TestStage*` failures in the first full service run — environment

All 45 failing top-level tests match
`^(TestStageCompletion|TestStageExecution|TestStageExitContinuation|TestStageArtifactReport)`
and print `artifact adapter unavailable` (95 times in the log).

Cause: the fixtures build artifact roots under `t.TempDir()`
(`internal/service/collaboration_test_report_artifact_test.go:45`). The default
`TMPDIR` here is `/var/folders/3n/…/T/`, and `/var` is a symlink to
`/private/var`. `artifact.openSecureRoot`
(`internal/artifact/files_unix.go:74-79`) `Lstat`s every path component and
refuses a symlink; `artifact.New` (`internal/artifact/packager.go:50-53`)
reports that as `ErrDisabled`, "artifact adapter unavailable"
(`internal/artifact/spec.go:35`). Setting `TMPDIR` to the resolved path removes
the symlinked component; that is the only thing the "realtmp" runs change.
`make check` does not hit this because `scripts/check.sh:175-176` pins `TMPDIR`
to `~/.multica/dev-tmp`.

Evidence that it is not the change:

| Run | Exit | PASS / FAIL / SKIP | File |
| --- | --- | --- | --- |
| S1, with the change, `TMPDIR` resolved | 0 | 55 / 0 / 0 | `stage-realtmp.txt` |
| S2, default `TMPDIR`, change removed by `-overlay` | 1 | 10 / 45 / 0, the same 45 names (`diff` empty) | `stage-head-overlay.txt` |
| P1, with the change, `TMPDIR` resolved | 0 | 1635 / 0 / 18 | `full-service-realtmp.txt` |

`overlay-service-head.json` replaces `service/issue.go`,
`service/collaboration_edit.go` and `handler/issue.go` with their HEAD copies
and maps `issue_budget.go`, `issue_budget_binding.go` and
`issue_budget_test.go` to `""`. A first attempt without the handler copy did
not build (`undefined: service.TombstoneIssueBudgetMember`) and is not counted.

To reproduce the pass:

```bash
cd /Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server
TMPDIR='/private/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/' \
DATABASE_URL='postgres://multica:multica@127.0.0.1:5432/<clone>?sslmode=disable' \
go test ./internal/service/ -run '^(TestStageCompletion|TestStageExecution|TestStageExitContinuation|TestStageArtifactReport)' -count=1 -v -timeout 40m
```

Drop the `TMPDIR=` line to reproduce the 45 failures. On another account the
value is `python3 -c 'import os;print(os.path.realpath(os.environ["TMPDIR"]))'`
plus a trailing slash.

### 6.2 One handler failure in the first full handler run — a table-lock conflict

```
=== RUN   TestDeliveryCompletionHintFencesPolicyAndStatusUntilCommit/issue
    delivery_completion_hint_test.go:416: POST /api/issues/bdf86547-7bce-48c3-9092-9418b9bd320e/delivery-evidence/evaluations: expected 200, got 409: {"error":"artifact_authority_busy"}
```

**What failed.** Line 416 is `f.evaluation(t, "POST", key, f.req, 200)`, the
subtest's first evaluation request, right after its fixture is built and before
it installs its barrier or opens a second transaction. The Postgres log for
that instant
(`/tmp/f3-s4/pg-window-run1.log:1395-1396`, UTC):

```
2026-11-17 00:31:17.268 UTC [97521] ERROR:  could not obtain lock on relation "github_pull_request"
2026-11-17 00:31:17.268 UTC [97521] STATEMENT:  LOCK TABLE github_pull_request, issue_pull_request, issue_vcs_pull_request, vcs_pull_request IN SHARE MODE NOWAIT
```

That statement is `internal/service/delivery_evidence.go:118`; the comment
above it (`:115-117`) says contention returns 409 instead of waiting.
`internal/handler/agent_artifact.go:54-55` maps SQLSTATE 55P03 to 409
`artifact_authority_busy`. All three files are unmodified at HEAD. A passing
run logs exactly three failed table locks, all provoked on purpose by later
tests (runs 3–5: three each, `pg-window-runs345.log`); run 1 logs four, and the
extra one is the pair above.

The statement fails only while another session holds a lock that conflicts
with `SHARE` on one of the four tables: a writer inside an open transaction
(`ROW EXCLUSIVE`), or `VACUUM`, `ANALYZE` or autovacuum
(`SHARE UPDATE EXCLUSIVE`).

**Step 4 and those tables.** `grep -i 'pull_request\|LOCK TABLE'` over
`issue_budget.go`, `issue_budget_binding.go`, the query file, the generated
file and the three seam diffs matches nothing, and the step-4 product files
start no goroutine. The seams run inside the caller's transaction and add
statements on `issue_budget_member` and `issue_budget_observation` only. Step 4
cannot be the holder of that lock. It can only shift timing.

**Reruns.**

| Run | Exit | PASS / FAIL / SKIP | File |
| --- | --- | --- | --- |
| H1 `-count=1`, on the clone that had just failed | 0 | 1 / 0 / 0 | `hint-alone.txt` |
| H3 | 0 | 120 / 0 / 0 | `hint-x10.txt` |
| full package with the change, 5 runs | 1 failed, 4 passed | section 5 | `full-handler*.txt` |
| full package with the change removed, 3 runs | 3 passed | section 5 | `full-handler-head-*.txt` |

**The same failure with the change removed.** Left alone it did not recur, so
I provoked the lock from a second session on the clone `f3_budget_s4_vac2`
(`/tmp/f3-s4/vacuum-experiment-2.sh`) and ran the test both ways.

| Lock held meanwhile | Code | Run | Exit | Result | File |
| --- | --- | --- | --- | --- | --- |
| `LOCK TABLE github_pull_request IN SHARE UPDATE EXCLUSIVE MODE`, open transaction | with the change | H1 `-count=3` | 1 | 0 / 3 / 0; 12 of 12 subtests fail, all at `:416` with the 409 above | `hint-held-with-change.txt` |
| same | change removed | H2 `-count=3` | 1 | 0 / 3 / 0; 12 of 12 fail, all at `:416` with the 409 | `hint-held-head.txt` |
| none | with the change | H1 `-count=3` | 0 | 3 / 0 / 0 | `hint-vac2-control-mid.txt` |
| endless `VACUUM ANALYZE github_pull_request` | with the change | H1 `-count=20` | 1 | 0 / 20 / 0; 79 of 80 subtests fail: 66 at `:416` with the 409, 13 at `:430` "hint did not reach commit" | `hint-vacloop-with-change.txt` |
| same | change removed | H2 `-count=20` | 1 | 0 / 20 / 0; 75 of 80 fail: 66 at `:416`, 9 at `:430` | `hint-vacloop-head.txt` |
| none | with the change | H1 `-count=3` | 0 | 3 / 0 / 0 | `hint-vac2-control-after.txt` |

`pg_locks` showed the lock granted before, between and after the first two
runs (`vac2-A-evidence.txt`). `vacuum_count` went 34889 → 252836 → 477102
around the two loop runs and was still rising two seconds later
(`vac2-B-evidence.txt`).

**What this shows.** The failure is a lock conflict on tables step 4 never
touches, and it appears identically without the change whenever a conflicting
lock is held. Nothing in the subtest's own flow holds one at line 416: the
fixture writes those tables with synchronous autocommit inserts
(`internal/handler/delivery_evidence_test.go:21-26`) and the delivery code
starts no goroutine. Autovacuum does take that lock during this package: in
each of
runs 3–5 `pg_stat_all_tables` shows `autovacuum_count` 1 and
`autoanalyze_count` 2 for `github_pull_request` and 1 and 1 for
`issue_pull_request` (`hdl-3-autovacuum.txt` to `hdl-5-autovacuum.txt`; server
settings `autovacuum = on`, naptime 60 s, both thresholds 50).

**What it does not show.** Who held the lock in run 1: the server logs errors
only (`log_statement = none`, `log_autovacuum_min_duration` 10 min), and I had
dropped that clone before reading its statistics. Nor that the unprovoked rate
is the same with and without the change: 1 in 5 against 0 in 3 cannot tell
those apart. So I do not call it pre-existing. My assessment is a timing flake
in the delivery-evidence fence, with autovacuum the likely holder.

**If it recurs.** Keep the clone, read `last_autovacuum` and `last_autoanalyze`
for the four tables from `pg_stat_all_tables`, and compare them with the time
of the failed `LOCK TABLE` in
`docker logs multica-postgres-1 --since … --until …` (UTC).

**Outside this task.** The same mechanism reaches production: while autovacuum,
`VACUUM` or `ANALYZE` runs on one of those four tables, a delivery-evidence
evaluation answers 409 `artifact_authority_busy`. The `VACUUM` loop above is
that case. It is existing behaviour at HEAD; whoever owns delivery evidence
should know.

## 7. Deviations from design §9

D1–D8 are departures from the §9 text I was dispatched with. The lead has
since written each into §9. I checked the current text against the code, and
they agree. D9 is still not in §9.

| # | §9 as dispatched | What the code does | Why | §9 now |
| --- | --- | --- | --- | --- |
| D1 | in-flight tasks are those `dispatched` or `running` | also `waiting_local_directory` | A daemon holds the task in that state after the claim, and `pkg/db/queries/agent.sql:327` treats the three together. | §9.2 names all three |
| D2 | the transaction starts with the fence | workspace row `FOR KEY SHARE` first, then the fence | It is the precondition every other fence holder takes (`service/issue.go:389`, `collaboration_workflow.go:486`). A missing workspace is `not_found`. | §9.2 states it |
| D3 | same key and same intent → original result, "otherwise `already_enabled`" | same key with another intent → `operation_conflict`; an unknown key → `already_enabled` | Matrix B01 separates "conflict (different intent)", and §9.4 uses the same rule. | §9.2 states it |
| D4 | no mention of a reason | a reason is required (trimmed, 1–2000 bytes), is part of the intent digest and is stored in the operation result | Contract line 147 requires an explicit reason. No column was needed. | §9.2 states it |
| D5 | no rule for a no-op | closing a closed root, reopening an open one, setting the mode or the limit it already has → `state_conflict`, no write | Otherwise a no-op would spend a policy revision and invalidate every other caller's `expected_policy_revision`. | §9.4 states it |
| D6 | "lock root → check → update" | the replay lookup precedes the revision check | A replay always carries a stale revision: the original request moved it. | §9.4 states it |
| D7 | reopen of a tombstoned root is refused | `root_deleted` is checked before the open/closed rule; close, limit and mode stay allowed on a tombstoned root | The design refused only reopen. | §9.4 states it |
| D8 | `root_conflict` listed before `parent_cycle` | a loop through the issue outranks a conflict below it | The walk result is not a tree, so "a descendant is bound" is not a reliable statement about it. | §9.2 states it |
| D9 | no format for the key, no sign rule for the limit | a key is 1–200 bytes, UTF-8, no NUL; a negative limit is refused and zero accepted; otherwise `invalid_request` | The operation table stores the key; the limit column allows zero. | not in §9 |

Unchanged from the design and worth stating: member rows are written for
descendants of every kind; S4 and S6 write nothing; reparent never refuses the
move; one `root_conflict` observation is written per foreign descendant. §9.5's
covered judgement is made at read time, in SQL, as a `covered` column of
`ListIssueBudgetObservations`.

## 8. Open questions for the lead

1. **§9.3's new off-mode rule leaves one interleaving uncovered.** §9.3 now
   says that while a root is `off` the binding writes member rows and no
   observation, to be done in step 5. Step 4 writes its three observations in
   every mode, as §9.3 notes. Under the planned rule, by reading and not by
   test:

   1. Root R is `off`. Issue x is unbound and has a task T that a daemon
      already holds. T has no attempt, because it was claimed while x was
      unbound.
   2. Transaction B moves x under a member of R (rule 2). It reads R's mode
      without a lock, sees `off`, writes x's member row and no
      `in_flight_at_bind`. It has not committed.
   3. Transaction M runs `SetMode(R, observe)`. It takes the root lock, which
      B does not hold. Its §10.6 scan lists in-flight tasks of R's committed
      members, so it does not see x or T. M commits.
   4. B commits.

   R is now `observe`, x is a member, and T runs with neither an attempt nor a
   gap observation: the "unknown shown as 0" case §10.10 exists to rule out.
   §10.10 says the `off → observe` scan does not need the binding lock. That
   holds against finalize, not against a binder deciding on an unlocked read.

   Ways out: (a) keep `coverage_gap` observations unconditional in the binder,
   as step 4 does, suppress only `would_refuse` while off, and have readers
   group by `(task_id, claim_generation)`, since the same task can then carry
   both `in_flight_at_bind` and `in_flight_legacy`; (b) have `off → observe`
   take the binding lock in shared mode before the root lock, and have the
   binder read the mode after its exclusive lock; (c) leave the binder
   mode-blind and filter by mode or epoch when reading.

   Whichever is chosen: a mode read in the binder is allowed by the body of §5
   rule 1 (no lock, no update), but `TestIssueBudgetBindingNeverTouchesARootRow`
   fails on any statement naming a root table and must be relaxed; and §9.3
   cites §10.6 for the rule, while §10.6 describes only the scan.
2. **Where the binding lock goes.** §9.2 says before the in-flight scan;
   §10.10 says before the member rows are written. By my reading both close
   the window: rows written before the lock are invisible to a finalize until
   commit, and what matters is that the scan runs under the lock. Later means
   a shorter exclusive hold and fewer requeued claims. I suggest taking it
   after the refusal checks, so a refused or replayed enable takes no exclusive
   lock at all, and in reparent only inside rule 2. The cost for a tree without
   a root stays unchanged, because rule 2 is reached only under a bound parent.
3. **References in the design.** §9.2 cites §10.11, which does not exist.
   §10.6 lists `dispatched / running` while §9.2 lists three states and §10.6
   says it is the same scan.
4. **B06 wording.** The matrix names the result `pre_activation_unknown`
   (reason `in_flight_legacy`). Step 4 stores it as an observation
   `coverage_gap`/`in_flight_legacy` per §9.2 and touches no root projection
   column; turning uncovered gaps into "lifetime total unknown" is a read-side
   rule for step 5 or 7.
5. **Actor.** Operations are stored with `actor_type = 'member'` and the user
   id. The service takes an already-resolved actor and checks no permission.
6. **Section 6.2.** Whether the evidence there is enough to accept the step, or
   more unprovoked runs are wanted.

Nothing else in §10 contradicts what step 4 built. Section 9 lists what §10
will change in step-4 files.

## 9. Where design §10 touches step-4 code

Nothing below is implemented. `design.md` changed on disk while this step ran:
§5 rule 4 was amended and §10.10 added, and the file as last modified at
12:28:03 carries D1–D8 in §9. The step-4 code is the scan-only version that
§10.10 corrects.

| Design | Step-4 code affected |
| --- | --- |
| §10.10 binding lock | `Enable` and rule 2 of `BindIssueBudgetOnReparent` get one more statement. `wantEnable` in `TestIssueBudgetTransactionsFollowTheLockOrder` must gain it. See open question 2 for the position. This answers a race I had found independently: the scan cannot see a claim committed after it. |
| §9.3 off-mode rule | `BindIssueBudgetOnReparent` must learn the root's mode; `TestIssueBudgetBindingNeverTouchesARootRow` and the tests that expect observations regardless of mode change with it. See open question 1 first. |
| §10.1 `attempt_reserve_ticks` | `SetLimit` signature, `issueBudgetIntent`, `UpdateIssueBudgetRootPolicy`, `issueBudgetResult`, and the no-op rule D5, which must then compare both values. Adding a field to `issueBudgetIntent` changes every digest; that is free now and needs `omitempty` or a new domain string once operations exist in a deployed database. |
| §10.6 scan on `off` → `observe` | `SetMode` reads members and task tables after the root lock. Allowed by §5 rule 2 (plain reads), but the comment on `manage` and the asserted `SetMode` sequence say "budget tables only" and must change. It needs a query listing the members of a root. |
| §10.7 `attribution_difference` | `BindIssueBudgetOnCreate` needs the executing task id, so `CreateInTx` must receive it. It adds a lookup for task-created issues, which changes the "one lookup" count asserted for rootless trees. |
| §10.8 schema | `InsertIssueBudgetRoot` and the generated code are regenerated with the new column. |

## 10. Not verified

- Packages outside the list in section 5 were not tested: `cmd/server`,
  `internal/daemon` and the rest. They compile and vet clean (B1, B2).
- `-race` was run on the new service tests only, not on the handler tests or a
  full package.
- Three handler tests assert an absence and pass with the seams unwired
  (section 12). No mutation was run against them.
- No query plan was measured for `ListIssueBudgetDescendants` or
  `ListIssueBudgetInFlightTasks`. The quick-create branch of the second joins on
  `lower(context->>'parent_issue_id')` with no supporting index. Both run only
  when a binding actually happens, never for a tree without a root.
- Lock order is shown by the recorded statement sequences and by two-connection
  tests that observe `pg_blocking_pids`; there is no deadlock stress test.
- The handler package was run only with `TMPDIR` resolved.
- The holder of the lock in 6.2 is not identified.
- Open questions 1 and 2 are reasoning over the design text; no test backs
  them.
- Nothing was run on Windows or Linux.

## 11. Proposed changes to `design.md` and `implement.md`

I edited neither. The §9 changes I proposed in the first version of this
report are already in the file. What remains:

- `implement.md`, verification: hand-run Go tests need `TMPDIR` resolved
  (6.1), or `internal/service` reports 45 unrelated failures.
- §9.3 and §10.6: decide open question 1 and write the rule where §9.3 points.
- §9.2 and §10.10: one position for the binding lock (open question 2).
- §9.2: the reference to §10.11. §10.6: the three in-flight states.
- §9.2: the key format and the sign rule for the limit (D9), if wanted.

## 12. Process notes

- `issue_budget_binding.go` was written before its tests. Red was then shown
  against stubbed service methods (22 FAIL) and against unwired seams (4 FAIL).
- The handler seams were wired before the handler tests existed. Their red run
  uses `-overlay` with HEAD copies of the seam files (6 FAIL, each for the
  missing binding).
- `TestIssueBudgetBusinessReopenWritesNothing` (B09),
  `TestIssueBudgetMoveKeepsTheMemberRow` (B10) and
  `TestIssueBudgetWritesNothingForATreeWithoutARoot` assert that nothing is
  written, so they pass with the seams unwired and have no red run. They guard
  against a later change that starts writing.
- One `go vet` run failed during development on a test of mine comparing an int
  status with a string (`vet1.txt`); fixed before any run reported above.
- A first lock experiment (`vacuum-experiment.sh`) is not counted: its finite
  `VACUUM` loop ended before the change-removed run started, so that run's
  pass proves nothing. Its with-change run did show the 6.2 signature.
- The second experiment ran about 500,000 `VACUUM ANALYZE` statements against
  its own clone in the shared Postgres container over about two and a half
  minutes (12:49–12:52). Other sessions' tests may have run slower meanwhile.
- `gofmt -l` lists `internal/service/empty_claim_cache.go` at HEAD. I did not
  touch it.
- `git stash list` shows 10 entries. All predate this step; none is mine.

## 13. Addendum, 2026-11-17 15:36 +0800

Written after the lead reviewed the 13:55 text. Read against `design.md` as
last modified 15:08:25 (66,911 bytes, sha256 `20de56b2178bbcbf…`) and
`implement.md` as last modified 14:45:31. No product file changed: the ten
hashes in section 1 were recomputed at 15:18 and match, and no file under
`server/` is newer than the 13:58 report.

### 13.1 Closed since 13:55

| Item above | Now |
| --- | --- |
| 8.1, the off-mode interleaving | §10.6 drops the scan on `off → observe` and writes one `coverage_gap`/`observation_off` per return to observe, never covered. By my reading this closes the interleaving, on the two conditions §10.6 states: the binder reads the mode in a new statement after the binding lock, and at READ COMMITTED (13.3). |
| 8.2, position of the binding lock | §9.2 and §10.10 agree. Enable: after every refusal check, before the root insert. Reparent: only in rule 2, after the walk, before the mode read and the member rows. |
| 8.3, references | §10.11 exists; §10.6 no longer lists task states. |
| 8.4, B06 wording | §10.12. |
| 8.6, the failure in 6.2 | Accepted as environmental in the `implement.md` progress log. |
| D9 | In §9.2, with one addition the code does not have (13.2). |
| Section 11, `TMPDIR` note | `implement.md:46`. |

8.5 is a statement, not a question: operations carry `actor_type = 'member'`,
and permission is step 7.

### 13.2 Design text the step-4 code does not match

**`Enable` takes no mode.** §9.2's request format now says the switch may only
be `off` or `observe`, and §10.6 gives a rule for a root enabled as `off`. The
§9.2 I was dispatched with (read 06:07:59) names no mode for enable, and the
code has none:

- `IssueBudgetEnableParams` (`issue_budget.go:110-121`) carries a key, a limit,
  a reason and the actor.
- `InsertIssueBudgetRoot` (`pkg/db/queries/issue_budget.sql:4-7`) does not set
  `mode`, so every root starts as `observe`, the column default.
- The `off`/`observe` check exists in `SetMode` only (`issue_budget.go:528`).
- The in-flight scan in `Enable` is unconditional (`issue_budget.go:464`).

Today a root is `off` only after a `SetMode`. Either enable gains a mode in 5c
(the params, the insert query and the scan condition change; `Mode` is already
a field of the intent digest and is empty for enable), or the two sentences go.

**Rows of section 9 that now mean something else.**

- "§10.6 scan on `off → observe`": there is no scan. `SetMode` to `observe`
  gains one `InsertIssueBudgetObservation`. That is a budget table, so the
  comment on `manage` (only budget tables after the root lock) stays true.
  `TestIssueBudgetTransactionsFollowTheLockOrder` runs only `observe → off`
  and needs an `off → observe` case.
- "§9.3 off-mode rule": rule 2 writes all three of its observations to the
  parent's root (`issue_budget_binding.go:143-172`), so one mode read after the
  binding lock serves them. Rule 1 writes to the issue's own root
  (`:105-114`); its read needs no lock and belongs in that branch only.
  `TestIssueBudgetBindingNeverTouchesARootRow` must be narrowed to "never
  locks or updates a root row".

### 13.3 Isolation level on the binder paths

§10.6 asks for it and §5 rule 5 (added 15:08) now states it. I find the same,
by reading and not by a test:

| Path | Transaction starts at | Level |
| --- | --- | --- |
| `Enable` | `issue_budget.go:242`, `s.TxStarter.Begin(ctx)` | server default |
| reparent, issue update | `handler/issue.go:3940`, `h.TxStarter.Begin(ctx)`; the first statement is the fence batch (`lockIssueWriteFences`, called `:3947`); the file has no isolation statement | server default |
| reparent, collaboration edit | `EditIssue` → `mutateCollaborationWorkflow` → `workflowInTx`, whose first statement is `SET TRANSACTION ISOLATION LEVEL READ COMMITTED` (`collaboration_workflow.go:464`) | READ COMMITTED, explicit |

The server default in this container is `read committed`
(`SHOW default_transaction_isolation`), and `pg_db_role_setting` is empty, so
no database or role overrides it. No Go code sets a pool-wide default;
`dbstartup.ParsePoolConfig` passes the URL through, so a deployment can change
it. Two of the three paths therefore meet the condition by default, not by
statement. The assertion planned for 5b turns a wrong default into an error.

For the create and delete seams I looked only at the direct openers. Plain
`Begin`: `service/issue.go:278`, `service/collaboration.go:165`,
`handler/lifecycle_handoff_transaction.go:249`, `handler/triage.go:220`
(create) and `handler/issue.go:4878` (delete). Not traced:
`handler/triage_import.go:443`, which passes a nested transaction, and the
opener behind `DecideStageProposal` → `materializeStageChildren`
(`collaboration_stage_actions.go:514`, `:577`).

### 13.4 Stored timestamps once the binding lock exists

Reasoning, not tested. While a binder holds the lock, a claim committed before
it can only move from in flight to terminal. The scan misses a task that turns
terminal before the scan statement runs. An instant stored as "attributed from
here on" must therefore not be earlier than the scan.

- `Enable` samples `activated_at` before the root insert and scans last
  (4.1). A task in flight at `activated_at` that ends before the scan gets
  neither an attempt nor a gap, although it ran after the activation point
  (migration 866: "Costs are attributed exactly from this point on"). The
  window is three statements wide. Scanning first (lock → scan → sample →
  inserts) makes the error conservative: a task ending between scan and sample
  gets a gap it did not need. Nothing forces today's order: the root id is
  generated in Go and no table has a foreign key.
- Member rows take `bound_at` from the column default `now()`, which is the
  transaction start; on the issue-update path that precedes the fences.
  Nothing in step 4 reads it as an attribution boundary. If step 7 shows it as
  one, the same applies.
- The `created_at` of an `observation_off` row will likewise be the start of
  the switch's transaction, while the period it stands for ends at that
  transaction's commit; the wait for the root lock lies between the two.
  §10.12 never compares a claim time with it. A later slice that confirms a
  gap by time should not use `created_at`.

### 13.5 State at this revision

- Databases: only `f3_budget_base_865` (checked 15:22).
- Worktree: HEAD `86fea651c`; nothing committed, stashed or pushed. I have not
  written to it since 07:44, and not while the independent check runs.
- Outside the task: a session hook required compacting the shared memory index
  (`MEMORY.md`, 23,979 → 16,452 bytes, all 108 entries kept). Each shortened
  entry's original text is appended to its topic file.
