# P1 foundation implementation evidence

## FG — 2026-10-05

FG is ready for health/progress/UI implementation. This is not completion of foundation or P1.

- Worktree: `/Volumes/artisan/code/2026/multica-projects-p1`; isolated `.env.worktree` database, no production writes.
- Existing baseline from parent: Go `TestProject` passed (2.037s); core project tests 5 files / 52 tests passed.
- New RED: `TestProjectDescriptionRequiresRevisionAndPreservesNewerText` returned 200 instead of required 428; `TestProjectFinalDateOrderValidatedOnlyOnDateEdits` returned 200 instead of 422 before implementation.
- Schema: migrations 536–549 applied successfully from verified ledger 535. Five new P1 tables, project revision/description revision/in-progress provenance, workspace planning timezone. Thirteen indexes each use their own single `CREATE [UNIQUE] INDEX CONCURRENTLY` file; no added FK/cascade or implicit PK indexes. Every down locks all protected tables and refuses retained P1 data/configuration/version changes. Full fresh-schema/up/down/recovery exercises remain verification lane work.
- Generated SQL: `make sqlc` passed. Added tenant-scoped association `FOR SHARE` / `FOR SHARE NOWAIT` queries and state audit writes; workspace deletion explicitly owns P1 rows. Project execution-squad writes also advance project revision without advancing description revision.
- Project update: RR lock-internal field merge; description 428/CAS409; property revision CAS; true no-op preserves versions; date edits validate final order; status audit and transition clock with no issue mutation/enqueue. Search and normal DTOs include versions.
- Planning timezone: URL-workspace scoped member GET, human owner/admin PUT; IANA validation, nullable UTC default, dedicated column survives settings replacement. Static workspace capability route currently advertises description CAS/timezone only; overview/updates remain false until their implementation is integrated.
- Shared transaction: `runProjectTransaction(ctx, workspaceID, actorUserID pgtype.UUID, func(pgx.Tx,*db.Queries) error)` establishes RR/RW before any read, then workspace KEY SHARE → subscriber revocation fence → active member SHARE. SQLSTATE 40001/40P01/55P03 retries the entire rolled-back transaction at most three attempts (25/75ms plus jitter). Callbacks publish only after commit.
- Shared handler helpers: `projectHumanActor`, `projectErr`, `projectAPIError`, `writeProjectAPIError` in `project_write_fence.go`; `LockProjectForAssociation` and `LockProjectForAssociationNowait` both accept `{ID, WorkspaceID}` and return `db.Project`.

Verification commands (root environment exported, Go cwd `server/`, agent CLI guard enabled for handler tests):

```text
../scripts/go-test-with-agent-cli-guard.sh -- go test ./internal/handler -run 'TestProject(DescriptionRequires|FinalDateOrder|PlanningTimezone|Capabilities|StateAudit|TransactionRetries)' -count=1
  PASS (6 tests: CAS/no-op/old attributes, dates, timezone/roles/machine actor, member capabilities, status isolation, RR whole retry)
go test ./cmd/server -run '^$'
  PASS (router/server compilation; no tests selected)
go vet ./internal/handler ./cmd/server
  PASS
git diff --check
  PASS
```

An earlier broad `TestProject` run passed before the association lane added WIP tests; a later broad run included its unfinished fixture/red tests and failed. Those failures are not recorded as FG passes. The explicit FG filter is authoritative until the association lane is ready.

## Lifecycle and shared integration — 2026-10-05

- Added administrator delete-impact with total/formal counts and expected-revision delete protection. Deletion explicitly detaches issues, disables all related triggers, pauses non-archived automation with `pause_reason=project_deleted`, preserves archived status, clears chat/view/resource/P1 notification payload/history rows, and deletes the project atomically. No task status or execution history is changed.
- RED → GREEN: deleting an active automation originally left it active after the project FK silently detached it. `TestProjectDeletePausesAutomationAndOwnsProgress` now proves paused/archived/trigger semantics, retained running task and issue, cleared chat and P1 rows.
- Nine injected cleanup failures independently prove complete rollback of project/issue/automation/trigger/progress. `TestProjectDeleteImpactFormalAdmissionAndRevision` proves formal admission counts, stale-preview rejection, and machine actor refusal.
- The association lane found a real RR deletion race: a chat creator held only a shared project lock, so an already-created RR snapshot waiting on that lock did not see the newly committed chat row. The former deletion returned 204 while leaving the reference. Deletion now uses an explicit **READ COMMITTED** variant of the same authorization/retry transaction helper; after obtaining the exclusive project lock, each sweep sees all previously committed associations. Health, progress, CAS and statistics still use RR. The original failing chat barrier is preserved; association verification now passes all 15 writer paths in both ordering directions. See `association-verification.md` and commit `bb1702575`.
- Resource create/update/delete SQL now materializes the workspace fence before its stronger project NO KEY UPDATE fence. Execution-squad configuration takes the workspace fence first. The original T1 resource/execution snapshot serialization regression still passes.
- Foundation integrated all health/progress SQL and regenerated sqlc once per change; registered overview/risk/history/preview/publish/correct routes and progress notification worker next to the T1 worker using the same shutdown context. Shared protocol events use `project:update_published`, `project:update_corrected`, `project:planning_timezone_changed`.
- List/search/detail/create/update/squad responses consistently carry completed/cancelled/open counts and completeness. List/search batch project IDs; collection uses the shared health count implementation in one RR transaction. Successful collection preserves legacy `issue_count=N`, `done_count=F+C`; failures omit unknown new counts and mark incomplete instead of supplying zero.
- `FF_PROJECTS_P1=false` (feature key `projects_p1`) disables new progress write intentions and overview/update discovery. CAS and planning-timezone support remain advertised; history reads and already committed exact request replay remain available with current authorization. Progress owns the new-write/replay checks and tests. No data or schema rollback is performed.
- Additional concurrency proof: two simultaneous description updates using revision 1 produce exactly one success and one conflict, with persisted revision 2; revocation committed after the initial RR workspace read but before the member fence prevents the pending edit. A new regression caught and restored the pre-existing 400 response for invalid project CHECK values (instead of retryable 503).
- Every migration down guard now acquires protected table locks in workspace → project → P1 table order, all NOWAIT. Catalog and retained-data checks still run inside the same protected DO statement. No new up migration after 549.
- Migration verification reproduced an interrupted unique-index build leaving an INVALID index. Without recovery registration, a retry skipped it via IF NOT EXISTS and falsely recorded the migration. All thirteen 537–549 indexes are now registered in the existing production runner `concurrentIndexCleanups`, preserving valid indexes while cleaning only failed leftovers. The existing `TestEveryConcurrentUpBuildHasCleanup` first failed on all thirteen omissions and passes after registration; the independent runner/corrected-ledger exercise is tracked by migration verification.

Commands and observed results:

```text
../scripts/go-test-with-agent-cli-guard.sh -- go test -race ./internal/handler -run '^TestProject(DeletePauses|DeleteFailure|DeleteImpact|DescriptionRequires|FinalDateOrder|PlanningTimezone|Capabilities|StateAudit|TransactionRetries|UpdateConstraint|ConcurrentDescription|WriteRechecksMember)' -count=1
  PASS, 13 top-level tests plus 9 fault-injection subtests, 2.588s
# Existing project dates/validation/stats/resources/squad canonical files, 64 exact test names:
../scripts/go-test-with-agent-cli-guard.sh -- go test ./internal/handler -run '<64 exact existing project test names>' -count=1
  PASS, 2.087s
../scripts/go-test-with-agent-cli-guard.sh -- go test ./internal/service -run '^TestTriageProjectResourceWritesSerializeWithExecutionSnapshot$' -count=1
  PASS, 0.493s
../scripts/go-test-with-agent-cli-guard.sh -- go test ./cmd/server -run '^Test.*(Project|Route|Workspace)' -count=1
  PASS, 4.939s
go vet ./...
  PASS after progress test editing settled (an earlier attempt overlapped its missing-import intermediate state)
make sqlc
  PASS
../scripts/go-test-with-agent-cli-guard.sh -- go test ./cmd/migrate -run '^TestEveryConcurrentUpBuildHasCleanup$' -count=1
  PASS
git diff --check
  PASS
```

Foundation owns the original `.env.worktree` database. The parent created separate health/progress/association/verification databases after discovering that handler `TestMain` uses stable fixture identities; tests against one database must never run concurrently in multiple Go test processes. No ambient agent CLI was executed.

## Remaining parent verification

Source and local foundation tests are complete, including the association lane. Parent-level migration fresh/upgrade/index-failure/down rehearsals, actual old-router compatibility, whole-product checks, Web/Desktop workflow verification, final independent review and release/deployment certification remain the parent task's responsibility. I1 real iteration-history integration remains explicitly deferred to I1. No push, merge, release or production deployment occurred.
