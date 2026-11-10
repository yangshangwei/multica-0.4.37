# P2 ordinary writers: bounded implementation report

Date: 2026-11-11. Implementer: native `trellis-implement` / `implement_observer_core`.
Scope: ordinary update/move/per-item batch, ordinary creation/comment-child,
project changes/deletion, and migrations 789–791. This is not all-writer
acceptance, usable skip, full M1 completion, or public enforce activation.

## Changes and purpose

- `handler/issue.go`: strict additive completion/project-change controls; actual
  candidate derived from the locked issue after nullable refresh and description
  merge; effective new attachment detection; original-transaction Prepare,
  state-checked replay or Finish and one commit. Original authority checks stay
  in place. Replay returns current issue plus historical receipt and skips all
  publish/dispatch/parent-wakeup effects. A typed commit-unknown error does not
  unwrap into the old lock retry loop.
- Single responses add an optional `completion_receipt`. Legacy expected revision
  still rejects stale ordinary writes; a supplied completion uses the exact
  binding and restores a consumed original operation before that normal write
  rejection, preserving reopened/current state rather than writing old done.
- Batch retains old `issue_ids/updates/updated` and adds ordered results. Shared
  completion is rejected; per-item bindings are strict and unique. The first
  rejected/unknown item stops the loop, with known committed prefix retained,
  later not_attempted, and non-2xx response. Actual committed parent/stage events
  are processed at the common end even on early stop. Replayed items report their
  original committed revision but do not publish effects again.
- `issue_move.go` validates new controls before map re-encoding can hide duplicate
  fields, then forwards them to the same actual update transaction.
- `service/issue.go` checks the resolved project policy after parent-project
  inheritance in `CreateInTx`, so ordinary and comment-child done creation are
  refused under enforce. `source_context.go` and ordinary create map the domain
  refusal without a generic 500 or partial persisted issue.
- `project.go` verifies current policy inside the existing deletion transaction;
  enforce requires a previous explicit policy-off operation. Its existing
  collaboration-project deletion refusal remains intact.
- `service/delivery_project_change.go` locks current source/target policies,
  requires prior reopening when either side is enforce and rejects migration
  combined with completion. Enforce exit is current owner/admin human personally
  with a reason, intersected with the writer's existing permission. Accepted
  controlled scope adjustments retain actor, issue/source/target policy and
  resource revisions, and reason in the same transaction as the issue mutation.
  Genuine database/lock failures remain dependency/conflict failures rather than
  being flattened into authorization denial.
- 789 creates retained `delivery_project_change` without FK/implicit indexes;
  790 is one CONCURRENTLY unique index statement; 791 validates its exact usable
  shape. Highest down and table-drop both refuse retained records under
  ACCESS EXCLUSIVE NOWAIT. No historical migration was edited.

`updateIssueAtomically`'s third return is now a typed outcome. Its independent
iteration caller already discards that value, so its file required no change.
No new observer, batch ledger, automatic write retry, dependency, deployment or
external interface beyond the reviewed additive wire was introduced.

## Test-first history and actual runs

All DB execution below belongs to the main session's dedicated newly allocated
package databases. This lane added tests and compiled but did not allocate,
migrate or run a DB test itself.

- `p2-ordinary-red01`: exit 1, 13 failed named terminals. Actual update/move,
  batch and project boundary cases demonstrated missing enforcement. The three
  create-mode subcases hit a fixture counter collision: `dbfx.Issue` had inserted
  a parent number without advancing the workspace allocator. Those failures
  are fixture failures, not proof of D3 behavior. Their raw log remains retained.
- Corrected only the create fixture to create its parent through actual POST.
  `p2-create-red02`: enforce expected409/actual201 is the genuine D3 red;
  observe/off positive controls passed. Named totals: 2 pass / 2 fail, including
  the parent failure; not four independent behavior failures.
- `p2-ordinary-green01`: first four top-level boundary tests, 13 named pass,
  zero fail/skip. Confirms original red boundaries after implementation.
- `p2-ordinary-iter02`: expanded ten top-level tests, 34 named pass, zero fail/skip.
  Includes actual existing controlled-tar/receipt pipeline, new human
  authorization, real UpdateIssue state+terminal consumption, post-done edit,
  reopen then original-key recovery, mixed title/attachment refusal, commit-before
  and commit-after uncertainty with original GET, custom done, archived done
  content edit, actual comment-child creation refusal, owner scope-change record,
  member refusal, and strict control/batch-map rejection.
- `ordinary-build01` and `ordinary-build02`: full backend build passed.
- `ordinary-test-compile01` and `ordinary-test-compile02`: handler `test -c`
  passed without running TestMain.
- `ordinary-migration-test-compile01`: migration `test -c` passed.
- `gofmt` and scoped `git diff --check` passed.

The handler positive uses actual existing `deliverySuccess`/artifact creation,
approval/execution and real delivery/authorization APIs; it does not seed a
successful artifact or completion terminal. The helper uses handler identity
fixtures, so actual authentication/HTTP writer and tar-byte/SHA end-to-end proof
still belongs to the root's router suite. Test-only enforce policy rows are
explicit internal fixtures while public policy activation remains closed.

## New tests added after the 34-pass run (not yet executed)

- `TestOrdinaryDeliveryBatchCommittedParentNotification`: reuse the existing
  actual staged-parent fixture, commit one complete stage before a refused item,
  require exactly one parent notification and pending wake pinned to its exact
  comment, and unchanged not-attempted later children.
- `TestOrdinaryDeliveryEnteringEnforceProject`: an open task can enter with a
  retained adjustment and is subsequently completion-gated; an already done
  task cannot enter without prior reopening.
- `TestDeliveryProjectChangeMigrationsUpgradeAndEmptyRollback`: executes actual
  789–791, verifies no FK/implicit index, replay and empty maintenance downgrade.
- `TestDeliveryProjectChangeMigrationsRetainAdjustment`: highest and table-drop
  guards preserve an actual retained row and catalog.
- `TestDeliveryProjectChangeMigrationsFenceConcurrentAdjustment`: uncommitted
  real insert blocks both downgrade guards; after commit the row is retained.
- `TestDeliveryProjectChangeMigrationsRejectMalformedIndex`: actual readiness
  migration refuses a same-name index with wrong column order.

The handler inventory is now twelve top-level `TestOrdinaryDelivery*` tests;
no new total is reported as passed until the main session runs it. Final normal
and race each need a separate fresh DB, alongside existing batch/custom status,
project scope/concurrency, authorization/observer compatibility and actual
migration suites. Do not sum rounds or normal/race as distinct coverage.

## Retained implementation/lookup failures

- While this lane was between coherent edits, another lane's
  `p1-router-iter01` compiled against an imported-but-not-yet-used pgconn in
  `issue.go`; zero tests executed. Root retained that original failure. The final
  code uses pgconn for batch error classification and `ordinary-build01` passed.
- Root's full vet identified unkeyed literals for externally defined
  `service.DeliveryCompletionError`; this lane changed its owned occurrences to
  keyed Status/Code fields. The original vet log remains root-owned evidence;
  this report does not claim a later full vet result.
- A few source reads guessed absent names (`delivery_completion_authorization_test.go`,
  `issue_edit_autosave_test.go`, migration `main_test.go`, and workspace deletion
  paths). `rg --files`/symbol searches located the real files before retry.
  These were read-only lookup failures, not product red or database actions.

## Integration and remaining obligations

P1 ledger owns its core, schema and generated SQL. It was asked to register the
790 concurrent-index cleanup and retained-workspace manifest; source inspection
shows those registrations present, but their final verification is root-owned.
This lane did not edit ledger/generated/registry files or any other lane's code.

P3 collaboration must call the same project-change helper in its own source-edit
transaction; VCS, collaboration/self/R38 completion and full consumer identities
remain later work. The main session owns full independent review, API/skill/spec
and route updates, final source/run freezing, safe local commits/integration and
M1/parent status. This lane made no commit/push/deployment/provider call.

## Source bindings at lane freeze

| File | SHA-256 |
| --- | --- |
| `server/internal/handler/issue.go` | `d727116aefa4dcda5ee62ae5bfe2e77f5aa2179d0b21f4eb080f848c0d8f07ef` |
| `server/internal/handler/issue_move.go` | `d24a2721b68acc381d80f7389aa786ad71c13dbec2e9adb9df79af69b227de45` |
| `server/internal/handler/source_context.go` | `153a7993c9d6b49f2f7cea158cd2749bfaad9f5380e5a2fc892582f50635cef1` |
| `server/internal/handler/project.go` | `55edfaf97cd153601a9154bbd189d09eb54e907ea9c6f2a99b644cd237beee1e` |
| `server/internal/service/issue.go` | `d1f7c8f5aa6e621e117cf97e1f4f2da496ccaeba9dabb7f3fdef12e97cff8ba2` |
| `server/internal/service/delivery_project_change.go` | `300ea7b9853af80f7dfc29963e810e0360ae052940f499a370e42937a1d9b62c` |
| `server/internal/handler/issue_delivery_enforcement_test.go` | `d6c2a3a554180c154a0a1cf42d29a65442a215654d2f4140ba256cf23e7bad7e` |
| `server/cmd/migrate/delivery_project_change_test.go` | `027f9199049db43b7871fe62667a4881070dc2965632bf49eb03dd595d62f394` |
| `server/migrations/789_delivery_project_change.down.sql` | `baa5fd611a00da9e5b0b744519c8e32f60c8f891b2d3594f2e4a837228b8857a` |
| `server/migrations/789_delivery_project_change.up.sql` | `e84c4d6aa15dee0c2ad409e9b53a4b5485401b0a6e7b3542158094fac118e6dc` |
| `server/migrations/790_delivery_project_change_operation.down.sql` | `b51e7adb87c066a16e40ab60cc7be39cf1211280bb75e179c807be3129059d7b` |
| `server/migrations/790_delivery_project_change_operation.up.sql` | `246219700964d72250cb7ff8ee34bfaaf52f3baa30e3af56ea2308ce24bd96f0` |
| `server/migrations/791_delivery_project_change_ready.down.sql` | `fd2e87c93bb0b0ac0da98a68845dcecb543f88ae25a6d7c86d3f760e48eed424` |
| `server/migrations/791_delivery_project_change_ready.up.sql` | `9b4b8129a6673bf6de04c4743a8daf1f8b7f4f1bbdce9b21c687504319928dd5` |
