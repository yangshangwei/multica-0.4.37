# Verification — 小阿孚 display-name rollout

Date: 2026-09-28. Branch: `codex/rename-mika-xiaoafu`.
Worktree: `/Volumes/artisan/code/2026/multica-afu-rename`.

## Result and review

Implementation passes the scoped acceptance review. English and Chinese creation
use 小阿孚; system identity remains `mika`, API paths and UUID relationships are
unchanged. Current names enter system instructions and new openings. Shared
Web/desktop labels, bootstrap errors, new chat titles and install-runtime issues
use the new display name. No dependency or schema object was added.

Existing builtin agents migrate only when their old name/default description or
onboarding title matches the design. Tests preserve custom names, descriptions,
instructions, titles, ordinary same-named agents, messages, kickoff records,
issue assignments and task snapshots. Active and archived collisions leave the
conflicted agent unchanged, while unrelated workspaces proceed. Replaying the
migration preserves already-migrated content and timestamps; down preserves
editable content, including post-migration changes.

The final review found and fixed a reporting gap: pgx ignored PostgreSQL NOTICE
messages in the actual migration command pool. `TestMigrationPoolReportsSkippedRowNotices`
first failed with an empty captured log; it passes after the command-specific
connection handler forwards notices to slog. The shared startup timeout parser
is retained. The parent approved the additional `cmd/migrate` implementation and
test scope. A second review found no remaining task-scope correctness issue.

## Evidence

All database commands explicitly source this worktree's `.env.worktree` and use
`multica_multica_afu_rename_313`; no shared application service was started,
stopped or targeted. Migration tests create and remove isolated schemas. Go test
commands use `scripts/go-test-with-agent-cli-guard.sh`; no real agent CLI runs.

| Check | Result |
| --- | --- |
| `pnpm --filter @multica/core exec vitest run onboarding/mika.test.ts onboarding/use-bootstrap-mika.test.ts` | 2 files, 15 tests passed |
| `pnpm --filter @multica/views exec vitest run workspace/welcome-after-onboarding.test.tsx onboarding/onboarding-flow-completion.test.tsx onboarding/steps/step-runtime-connect.test.tsx --maxWorkers=2` | 3 files, 15 tests passed, including both locales |
| `go test ./internal/handler ./internal/service ./internal/migrations -run 'Mika\|ComposeMika\|BuiltinSkills\|TaskBuiltinSkills\|OnboardingScope\|BuiltinAgentDisplayName\|Migration' -count=1 -race` | All three packages passed against PostgreSQL; includes bilingual creation, idempotent reuse, customized content, scope authorization and migration lint |
| `go test ./cmd/migrate -run 'MigrationPoolReports\|RunMigrationsConcurrent\|EveryConcurrent' -count=1 -race` | Passed; real PostgreSQL notice output, concurrent migration runners and index cleanup registration |
| `go run ./cmd/migrate up` | Applied `457_builtin_agent_display_name` to the full worktree database successfully |
| `pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=2 --force` | 15 tasks passed; existing lint warnings remain in unrelated files |
| `go vet -p 2 ./cmd/migrate ./internal/handler ./internal/service ./internal/migrations` | Passed |
| `pnpm check:ui-exports`, `git diff --check`, gofmt inspection | Passed |
| Four changed locale JSON files parsed and scanned | No legacy display name remains in values |
| Migration SQL static inspection | No foreign keys, indexes, schema additions, identity changes or historical-message rewrites |
| `python3 .trellis/scripts/task.py validate .trellis/tasks/09-28-rename-mika-xiaoafu` | Both context manifests valid |

Final-review runs are fresh. Earlier implementation was inherited after an
interruption; no unrecorded historical red-phase run is claimed. The NOTICE fix
has an observed failing and passing PostgreSQL regression.

## Compatibility, recovery and limits

- A name collision is reported with the builtin agent/workspace IDs. Resolve it
  through the ordinary editing interface, then rerun the reviewed idempotent
  `457_builtin_agent_display_name.up.sql`. The migration ledger does not retry
  already-recorded skipped rows. Do not delete ledger entries to force replay.
- Down intentionally does not reverse owner-editable content. Old code already
  supports arbitrary saved names; individual restored names require inspection.
- Installed desktop clients need a new build for bundled copy. Old clients may
  still submit old default titles; those stay valid and do not change identity.
  Name-based scripts should use 小阿孚 or the stable agent UUID.
- Historical messages/tasks and custom content can still say Mika by design.
- Full browser E2E, packaged desktop smoke, production-scale migration timing,
  complete repository test suites and external deployment were not run. The
  scoped real-database/race tests, shared component tests and all nonmobile
  lint/typecheck tasks cover the changed behavior.
- This branch does not touch the original checkout's skill-market changes.
  The parent agent owns local integration and its final combined verification;
  the PRD integration checkbox remains pending until that step completes.

## Parent integration verification

Fast-forwarded local main to `dd63e2a41`, preserving project/market edits. The combined repository passed 8,717 tests and all 15 nonmobile lint/typecheck tasks. Parent reran `go test -p 2 ./cmd/migrate ./internal/migrations -run 'MigrationPoolReports|BuiltinAgentDisplayName' -count=1` against the isolated verification database; both packages passed. No external deployment or push.
