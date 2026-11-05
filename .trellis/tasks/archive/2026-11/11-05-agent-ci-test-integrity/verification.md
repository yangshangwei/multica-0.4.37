# CI and database precondition verification

Implementation worktree: `/Volumes/artisan/code/2026/mc-agent-rnd-p1-20261105`.
Baseline: `f1d35b66af1013b6147c0e167500cacae203b7a8`.
Tools observed: Go `go1.27.0 darwin/arm64`; Node `v22.23.2`. Hosted workflows continue to select Go `1.26.x`, Node `22`.

## Implemented boundary

- Release verify now declares a private PG17/pgvector test database and Redis service, verifies the actual database name, and migrates before complete Go tests. Existing release dependency graph, vulnerability scan and tag override remain intact.
- The complete Go wrapper rejects missing/whitespace DATABASE_URL before `go list` or any test command.
- Handler/router TestMain now exits 2 for invalid/unreachable explicit database settings, with fixed credential-free diagnostics and a 5-second connection probe. Normal fixture/test contexts remain unbounded as before.
- Testutil no longer falls back to localhost. Absent database configuration permits pure HTTP helper tests and explicitly skips DB fixture tests; explicit parse/connect/seed failure exits 2.
- Backend CI path filters include Release and its regression inputs; explicit Node22 runs the workflow and offline database precondition checks. Release also runs the precondition check.

## Red evidence

Preserved under `research/`; no failed output was rewritten with green output.

- `wrapper-red.log`: the newly added absent-config case failed because `go list` and both `go test` invocations occurred.
- `workflow-red.log`: 5 tests total, 3 existing graph tests passed, 2 new tests failed (missing PG service; Release file omitted from backend paths).
- `database-preconditions-red.log`: 12 named child scenarios plus parent. Handler/router absent/whitespace passed (4 cases); configured malformed/unreachable paths and optional testutil fallback guards failed (8 child cases, plus parent). Baseline elapsed 38.85 seconds. Optional testutil absence cases were refused by an explicit source guard before binary execution, preventing the old hardcoded localhost fallback from running.
- The outer first red-run shell attempted to assign zsh's readonly `status` after the actual commands; that reporting-shell error did not alter the saved test output. Subsequent commands use `ci_check_exit`. This setup error is not counted as a product failure.

## Green evidence

| Command | Result | Evidence |
| --- | --- | --- |
| `bash scripts/test-go.test.sh` | PASS; valid fake configuration keeps package split/race flags; unknown option and absent/whitespace config cannot invoke Go | `research/wrapper-green.log` |
| `node --test scripts/changelog-workflow.test.mjs` | 5 passed, 0 failed, 0 skipped | `research/workflow-green.log` |
| `bash scripts/go-test-with-agent-cli-guard.sh -- node --test scripts/go-test-database-preconditions.test.mjs` | 12 named scenarios + parent = 13 passed, 0 failed, 0 skipped; 36.65 seconds | `research/database-preconditions-green.log` |
| `GOMAXPROCS=2 GOPROXY=off GOSUMDB=off go -C server vet -p 2 ./internal/handler ./cmd/server ./internal/testutil` | exit0 | `research/go-vet.log` (empty successful output) |
| `bash -n scripts/test-go.sh scripts/test-go.test.sh` | exit0 | source syntax check |
| `node --check scripts/go-test-database-preconditions.test.mjs` and `scripts/changelog-workflow.test.mjs` | exit0 | syntax checks |
| `gofmt` for the three changed Go test files; `git diff --check` | no findings | format/whitespace checks |
| Python PyYAML safe_load of CI and Release | 13 and 10 jobs parsed | YAML structural check |

The offline matrix compiles each real test binary with `go test -p 2 -c`; it never uses `go test -run '^$'` as a compile substitute. It then executes startup rejection against synthetic configuration. Child environments clear inherited `PG*`, `DATABASE_URL`, and `REDIS_TEST_URL`; database URLs refer only to a nonexistent Unix socket below a private Git-external temporary directory, with no passfile/service credentials and read-only PGOPTIONS. Optional testutil covers a passing real HTTP helper and an explicitly skipped database builder. A synthetic credential sentinel is checked absent from diagnostics. State directories and binaries are removed by the test cleanup.

## Limits and follow-up

- No live database/service was started or contacted; no real model, credentials, full Go suite, release tag, remote CI or publication ran.
- Positive DB fixture/setup/seed behavior was not exercised. The seed-error branch now fails closed by code inspection, but was not separately fault-injected with a healthy fake/live database.
- Full per-package/per-test execution inventory and legacy service per-test skip behavior remain later scope. This patch fixes three TestMain boundaries plus the complete-suite wrapper, not every conditional skip in the repository.
- GitHub hosted execution with Go1.26 and real ephemeral services remains the integration acceptance step; local offline checks do not establish that the entire Release workflow has run successfully.
- No commit was made by this implementer; parent owns independent review, specs, task state and atomic commits.
