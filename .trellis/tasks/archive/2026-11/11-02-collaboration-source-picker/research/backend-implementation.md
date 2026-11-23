# Complete parent identifier search

## Changes

- `server/pkg/db/queries/collaboration_source_fields.sql` adds an exact, case-insensitive raw search comparison with the actual source workspace prefix plus task number.
- `server/internal/service/collaboration_source_fields.go` resolves the existing display prefix once and supplies it both to the query and parent identifier formatting.
- `server/pkg/db/generated/collaboration_source_fields.sql.go` was regenerated with the repository-pinned sqlc v1.31.1. No other generated files changed.
- `server/internal/service/collaboration_source_fields_test.go` owns the canonical query matrix and recent-window regression. It covers title/bare number/UUID/full identifier, case and whitespace, incorrect prefixes, numeric prefixes, both legacy prefix fallbacks, terminal/admitted parents, source-workspace filtering, pending exclusion, numeric-title matching preservation and an older task beyond the recent-200 window.

## Evidence

Before the repair, both new top-level regressions failed against the original SQL: 15 complete-identifier variants returned no matches and full-ID search did not find the older task. Title, bare-number, UUID, wrong-prefix and default window assertions passed. Full output: `backend-regression-red.log`.

After the repair, this guarded service run passed all 13 selected top-level tests, including the new matrix/window tests and existing source authority, writer-order, cycle, atomicity and iteration/creation tests:

```sh
bash scripts/go-test-with-agent-cli-guard.sh go -C server test ./internal/service \
  -run '^TestCollaboration(ParentSearchCharacterLimit|SourceParentSearch|SourceStructure|CreationInitial|IterationAssignment|IterationCompleted)' \
  -count=1 -v
go -C server vet ./internal/service ./pkg/db/generated
gofmt -l server/internal/service/collaboration_source_fields.go \
  server/internal/service/collaboration_source_fields_test.go \
  server/pkg/db/generated/collaboration_source_fields.sql.go
git diff --check -- server/pkg/db/queries/collaboration_source_fields.sql \
  server/internal/service/collaboration_source_fields.go \
  server/internal/service/collaboration_source_fields_test.go \
  server/pkg/db/generated/collaboration_source_fields.sql.go
```

All commands exited 0; gofmt and whitespace checks produced no findings. Full test output: `backend-regression-green.log`.

## Isolation and remaining validation

Tests ran on fresh task-owned database `multica_source_picker_service_20261102_8663cb98`, migrated through the current code. Its private connection JSON is outside the repository; `backend-database-allocation.json` records the path and ownership without credentials. The test pgx pool is capped at four connections. After tests, the database has zero remaining test connections. The PostgreSQL 17.11 cluster was probed at 38/100 connections before allocation; no unowned processes or databases were changed.

The service database and evidence remain available for coordinator review. Browser and HTTP boundary validation are the coordinator's independent lane; this service run does not claim those surfaces or a full backend suite.
