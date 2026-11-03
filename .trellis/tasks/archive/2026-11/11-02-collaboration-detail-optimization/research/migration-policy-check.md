# Migration 764 policy regression

Active task: `.trellis/tasks/11-02-collaboration-detail-optimization`.
Completed 2026-11-03; coverage-only continuation of the Server check.

## Change

Added only `server/internal/migrations/collaboration_ai_policy_test.go`.
The fixture follows the existing target-hold/attachment migration harness: one
isolated schema, a constraints/defaults clone of `public.issue_collaboration`,
then removal of the new column to represent pre-764 storage. It reads and executes
the actual `764_collaboration_ai_policy.up.sql` and `.down.sql` files.

`TestCollaborationAIPolicyMigrationPreservesDefaultAndGuardsDowngrade` verifies:

- Existing rows acquire assisted policy while all other retained fields remain
  identical, including an exact revision above JavaScript's safe integer range.
- Explicit manual policy persists; unknown/empty policies fail with SQLSTATE
  `23514`, and null fails with `23502`, without changing retained rows.
- Actual downgrade refuses a retained manual row with `P0001` and preserves the
  policy column and complete row snapshots.
- An explicit change to the other permitted policy permits later maintenance;
  the test retains the tasks rather than deleting them to make downgrade pass.
- An uncommitted manual-policy writer causes immediate `55P03` through the
  actual `NOWAIT` table lock, within a five-second bounded request.
- With no manual row retained, downgrade removes the column and preserves every
  other field. Reapplying actual upgrade restores assisted for existing and
  subsequently inserted tasks.

No production, R38, policy semantics, migration runner, existing tests, Core,
Views, e2e, spec or commit was changed.

## Verification

- Guarded compile-only: pass.
- `gofmt -l`, diff whitespace check and guarded package `go vet`: pass.
- Scoped guarded race execution: exit 0, **4 named passes**, 0 failures, 0 skips
  (one top-level transition test and three invalid-value subtests).

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- \
  go -C server test -json -race -p 2 -parallel 2 -count=1 -timeout 90s \
  ./internal/migrations \
  -run '^TestCollaborationAIPolicyMigrationPreservesDefaultAndGuardsDowngrade$'
```

The process parsed private configuration as data, used a private 0700 HOME and
the explicit caches in that configuration. No ambient provider CLI was invoked.
Compile handle `73122` and race handle `26667` are both closed.

## Database and retained evidence

One fresh database was created with explicit authorization:
`multica_detail_migration_policy_20261103_ace92a0e`, cloned from
`multica_detail_final_template_20261103_101054`. No existing database was modified
or dropped. The initial setup refused before SQL because `psql` was absent from
PATH; the corrected setup used the task launcher's installed
`/opt/homebrew/opt/libpq/bin/psql` (resolved client 18.6), without installing tools
or changing the source configuration.

The template and clone both had zero public issues/collaborations, 752 migration
ledger rows, maximum version `764_collaboration_ai_policy` and ledger MD5
`936ca02557727d7a7fc06f8f183d4571`. Pre-allocation cluster usage was 54/100
connections. Post-run audit confirms the same public counts, ledger and policy
column, **zero owned schemas**, **zero connections to the owned test database**,
and unchanged test source. The database and private logs are retained.

Private evidence directory:
`/tmp/multica-detail-verification-20261103/migration-policy-check-ace92a0e`.

- `allocation.json`: explicit clone identity, counts, ledger and connection budget.
- `compile-result.json` / `compile.log`: guarded compile result.
- `migration-policy-race-1-result.json`: exact command and named inventory.
- `migration-policy-race-1.jsonl`: raw Go JSON output.
- `vet.log`: guarded package vet output.
- `post-run-audit.json`: unchanged public identity, schema cleanup and zero owned
  database connections.
- `environment.json`: private 0600 configuration; credentials are not copied
  into this report.

Verified test SHA-256:
`5265ac5a954aa8ac71de8ff5f0cbacf1cc2367014ebe05979f160de441d84257`.

This targeted pass completes the canonical migration coverage gap. The root's
remaining complete Go package and repository gates remain separately owned.
