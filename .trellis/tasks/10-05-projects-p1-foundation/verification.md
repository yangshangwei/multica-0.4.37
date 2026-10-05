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

## Pending foundation integration

- Delete impact and atomic full cleanup, automation pause/trigger disable, data preservation and failure injection.
- All association writer barriers and both delete race orders (separate association lane).
- Health/progress query proposals, router/worker/DTO statistics integration, final broader regression/static checks.
- Full migration rehearsal and old-router compatibility certification belong to parent verification. No release/push/deployment occurred.
