# I1 migration recovery and final backend evidence

2026-10-07, branch `codex/projects-p1`, base HEAD `115b4cd28` plus the retained
working tree. Exact source hashes, per-test results and log hashes are in
[migration-recovery-evidence.json](migration-recovery-evidence.json).

## Scope and ownership

The audit created a new, unused local database
`multica_i1_recovery_20261007_1125` on PostgreSQL17.11. Connection credentials
were privately derived from the existing check environment; none were written
to evidence or printed. A unique database comment marked ownership. Migration
recovery used unique private schemas; later the same owned DB received the
normal complete application schema solely for handler regression tests.

The database was removed after its tests: ownership comment matched, there were
zero live sessions, and a subsequent `pg_database` query verified absence.
`.omx/logs/i1-verification/migration-recovery-cleanup.json` records those checks.
No existing development/check database was altered or dropped.

## New migration regression

`server/cmd/migrate/migrate_iteration_recovery_test.go` reuses `runMigrations`,
`hooksForDirection`, the actual550–566 migration SQL, and existing test pool/
catalog helpers. Production migrations and migration runner are unchanged.

`TestIterationMigrationsRecoverPartialInvalidIndex` proves:

1. Real550–552 execute and produce exactly3 ledger rows.
2. Two deliberately conflicting active fixture rows make the real553 unique
   concurrent index build fail23505. PostgreSQL leaves the real index INVALID;
   553 is absent from the ledger and554 has not executed.
3. The fixture changes one active row to planned, preserving both rows. A
   negative control without cleanup hooks fails42P07 on the invalid leftover,
   proving that simply retrying the SQL is insufficient.
4. Retrying with the real registered hooks repairs the index and completes
   all17 ledger rows. All16 I1 indexes report valid/ready/live, and the two
   fixture rows remain one active and one planned.
5. Another complete up is a true no-op: index OIDs, definitions, validity and
   ledger applied_at values are identical.
6. Used I1 data refuses the first real down step withP0001, without any catalog
   or ledger mutation.

`TestIterationMigrationsPartialEmptyRollbackPreservesSharedTimezone` checks
prefixes of1,3,4 and11 actual files. Each empty-I1 prefix rolls back fully,
leaves the existing issue and `workspace.planning_timezone=Asia/Shanghai`,
removes I1-owned columns/tables, and then accepts the complete17-file upgrade.

Guarded command (the credential-bearing URL was passed through the process
environment, not logged):

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 \
  ./cmd/migrate -run '^TestIterationMigrations' -count=1 -json
```

Result: **PASS**,2 top-level tests,4 subtests,6 passing test events,0 skips,
0 failures, package2.223s. Raw:
`.omx/logs/i1-verification/migration-recovery-final.jsonl`.

This is a controlled local maintenance rehearsal. It does not certify online
schema rollout, production-data conflict repair, or safe downgrade to a server
that predates iteration facts. Existing all-step used/down/concurrent-writer
and full P1-preservation tests remain covered by the accepted full check.

## Two literal acceptance gaps filled

The existing pure A–J example only commented that F's external completion was
not an input, and the current DB test deleted F. It did not directly execute
AC12's outside completion. Existing iteration tests also had no actual parent
relation for AC13. The existing real-path canonical test in
`server/internal/handler/iteration_history_test.go` now:

- Creates B with `parent_issue_id=A`; checks each appears exactly once in both
  original and current history, preserving the A–J statistics.
- Moves F out through the real complete operation, then completes it with the
  ordinary HTTP handler. It verifies F is now done/unassigned while source D5,
  OD4 and event count remain unchanged. Later deletion still retains its
  original identity/title.

The dedicated current-participation deletion test still runs alongside this
fixture, retaining coverage of active deletion that directly creates a leave.
No production source changed.

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 \
  -parallel 2 ./internal/handler \
  -run '^(TestIterationHistory|TestIterationDeletionEventKeepsMembershipTransition)' \
  -count=1 -json
```

Result: **PASS**,15 top-level tests,21 passing test events including subtests,
0 skips,0 failures, package3.053s. Raw:
`.omx/logs/i1-verification/acceptance-history-final.jsonl`.

The first attempt used nonexistent fixture column `parent_id`, producing
PostgreSQL42703 before the scenario could run. This test-authoring error was
corrected to the actual `parent_issue_id`; its failure log remains in
`.omx/logs/i1-verification/acceptance-history-fixture-failure.jsonl`. It is not
a product regression or evidence that the initial run passed.

## Static checks and handoff

`gofmt -l` for both changed test files produced no paths;
`go -C server vet ./cmd/migrate ./internal/handler` and `git diff --check` exited0.
Raw: `.omx/logs/i1-verification/backend-audit-static-final.log`.

The accepted complete check remains the baseline for unchanged production
Go/TS. These fresh narrow race/static checks cover this audit's two test files.
No broad suite or benchmark was rerun, and no source changed after these
passing checks. The complete29-row map and explicit composed-proof limitations
are in [backend-final-audit.md](backend-final-audit.md).

Final UG/FCG/VG status, Web/Electron visual evidence, skip classifications and
whole-worktree provenance remain the main session's responsibility. The release
default stays false; nothing was committed, pushed, merged or deployed.
