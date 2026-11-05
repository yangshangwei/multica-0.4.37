# Database test isolation

## 1. Scope / trigger

The handler and server-router test packages initialize shared fixtures in `TestMain`. A test-name filter does not suppress this initialization. Run these packages only against an explicitly selected, task-owned database.

## 2. Commands and entry points

- `server/internal/handler/handler_test.go:TestMain`
- `server/cmd/server/integration_test.go:TestMain`
- Execute tests after loading the task's private environment file; verify the database identity without printing its credentials.
- Compile without execution with `go -C server build ./...` or `go -C server test -c -o "$TASK_TEST_BINARY" ./internal/handler`.

## 3. Environment and execution contract

Both mandatory entry points require nonblank `DATABASE_URL` before creating a pool, running fixture cleanup/setup, or constructing a router. Missing or whitespace-only values print a content-free configuration error and exit **2**. Invalid configuration and an unreachable explicit database also fail with a nonzero exit and sanitized diagnostics. Bound the connection probe, not the subsequent test/fixture lifetime. Never print a raw connection error that can contain credentials. Do not restore an implicit localhost database fallback.

`scripts/test-go.sh` checks the same nonblank requirement before `go list` or any package invocation. `internal/testutil` may run pure HTTP helpers without a database, with explicit DB-test skips and no fallback; once `DATABASE_URL` is provided, parsing, connection and seed failures must fail the package. Release verification provisions its own database and Redis, proves the database identity, and applies migrations before invoking this wrapper.

An explicit URL is not proof of ownership. The caller must select its recorded test database, keep credentials in the private environment file, and serialize service/handler/router suites that share a database. Browser fixtures use their own database. Different workspace IDs do not isolate singleton gates or package-global fixture cleanup.

Migration commands need the same boundary before execution. The migration CLI
still has a local `multica` fallback when `DATABASE_URL` is absent; do not mistake
its successful exit for task isolation. Check both configuration and actual
database identity in the same shell/process environment before invoking Go:

```sh
: "${DATABASE_URL:?explicit task database required}"
: "${TASK_DATABASE_NAME:?recorded task database name required}"
test "$(psql "$DATABASE_URL" -Atqc 'SELECT current_database()')" = "$TASK_DATABASE_NAME" || exit 2
go -C server run ./cmd/migrate up
```

The expected name comes from the allocation record, never from whatever URL was
inherited. Confirm the client executable exists before running this sequence;
do not print credentials or source a dotenv file as shell code. Use one fresh
database per DB-backed package for joint acceptance, even when the old repository
entry point serializes several packages against one URL. A coordinator may
allocate/migrate a database and give a child only the exact package/test permit;
that child must not create, migrate, drop or switch databases.

If a command reaches an unintended database, stop writes to it, preserve the
command and a bounded read-only ledger/schema snapshot, and disclose the incident.
Do not infer pre-command contents from the current ledger or attempt speculative
down migrations/cleanup. An after-incident dump is not a pre-incident backup.

### Shared connection budgets

Fresh databases still share the PostgreSQL cluster's `max_connections`.
Budget every live task API pool, direct fixture connection and external client;
four default API pools of25 can exhaust a100-connection cluster before other
clients are counted. Do not start extra environments during a full check merely
because their databases and ports differ.

Before concurrent startup, record read-only aggregate connection counts without
SQL text or credentials. Use explicit task-only `DATABASE_MAX_CONNS` and
`DATABASE_MIN_CONNS` overrides when appropriate, and serialize API/browser stages
when headroom is uncertain. Never raise shared PostgreSQL limits or terminate
unowned connections to obtain a pass. Stop owned idle APIs after their proof is
captured; retain their database and logs. A capacity failure remains failed and
requires a newly numbered run after the environment correction.

### Private environment registry identity

An application environment file and a development registry are separate inputs.
Loading the former does not select `MULTICA_DEV_HOME`. Without that key,
`scripts/dev-env.sh up` can allocate another worktree database and different
ports in the default registry, even when `DATABASE_URL` was explicit.
Also set `ENV_FILE` explicitly: `up` loads that file before looking up an
existing manifest. A later manifest lookup does not undo values already loaded
from a stale default `.env.worktree`.

For an existing task environment, obtain the registry path and name from its
allocation record. Confirm its manifest already identifies the expected checkout,
database and API/Web ports before restarting:

```sh
: "${TASK_REGISTRY:?recorded private registry required}"
: "${TASK_ENV_NAME:?recorded environment name required}"
: "${TASK_SOURCE_ENV:?recorded startup environment file required}"
test -f "$TASK_REGISTRY/envs/$TASK_ENV_NAME/manifest.env" || exit 2
test -f "$TASK_SOURCE_ENV" || exit 2
MULTICA_DEV_HOME="$TASK_REGISTRY" ENV_FILE="$TASK_SOURCE_ENV" \
  bash scripts/dev-env.sh up \
  --name "$TASK_ENV_NAME" --components api,web --web-mode production
```

Record the selected registry with configuration fingerprints. Before any browser
or fixture write, verify the actual database, API commit/deployment identity,
production Web origin and listener ownership against that allocation. Process
exit 0 is insufficient. For Web, inspect the rendered browser `apiBaseUrl` and
`wsUrl` as well as server rewrites: a healthy API and correct proxy can coexist
with HTML pointing the browser at a stopped API port. If startup used another
registry or environment file, record the mismatch,
stop only its owned processes and retain its database/logs; correct the registry
in a separately recorded startup. Do not silently switch the expected identity.

Run each complete database suite against a newly created, migrated task database.
Knowledge candidate and invalidation history is deliberately retained: repeated
full runs can multiply `all_sources` scan work beyond a test's fixed drain budget.
Keep a failed database for diagnosis; create another empty owned database without
cloning its rows. Do not delete protected history or increase product limits to
make a repeat green. Dedicated Redis must likewise have one test owner at a time.

Fixtures creating a second collaboration source must register exact-source
transient collaboration cleanup before ordinary issue/workspace teardown. A bare
workspace DELETE does not clean soft collaboration relationships. Preserve
knowledge candidate/invalidation history; remove test-owned pending ordinary
collaboration events with their source rows so a later global dispatcher cannot
select a missing issue. Cleanup is test infrastructure, not a production delete
or retention policy.

## 4. Validation and error matrix

| Input / operation | Required result |
| --- | --- |
| `DATABASE_URL` absent or whitespace | Exit 2 before pool/setup/router; no database connection |
| Explicit task database | Normal fixture lifecycle and selected tests; record actual run/pass/skip inventory |
| Application env loaded but private registry absent or mismatched | Reject fixture/browser entry; retain startup evidence and verify the recorded allocation before retry |
| Registry matches but rendered API/WS addresses differ | Reject browser entry; explicitly select the startup `ENV_FILE`, retaining the original failed source/configuration evidence |
| `-run '^$'` | Still executes `TestMain`; never use as a compile-only check |
| `go build` / `go test -c` | Compiles without executing `TestMain` |
| Explicit but malformed/unreachable database | Nonzero exit before tests/fixtures, sanitized error; never convert setup failure into success |
| testutil with no database configured | Pure helper tests may run; DB cases explicitly skip, no implicit connection |
| Reused whole-suite database with retained history | Inspect queued work and immutable timestamps; preserve it and run final verification on a fresh owned database |
| Secondary source fixture teardown | No pending ordinary event may reference a removed issue; protected knowledge history remains unchanged |

## 5. Good / base / bad cases

- Good: load the recorded private test environment, verify its database name, execute one suite, and confirm all expected names actually ran.
- Base: use `go build` or `go test -c` when only compilation is needed.
- Bad: an empty test filter against an older revision with no explicit environment can initialize and clean the default database even though no test body runs. Never rerun that unsafe path merely to obtain red evidence.

## 6. Required regressions

`scripts/go-test-database-preconditions.test.mjs` compiles actual handler/router/testutil binaries with `go test -c`, then runs only deliberately isolated negative startup cases. Strip inherited PostgreSQL connection settings, use synthetic credentials and a nonexistent task-owned Unix socket, and assert nonzero failures without printing the synthetic credential. Its no-database testutil case is intentionally different from the two mandatory packages. Wrapper tests must prove blank input runs no `go list` or test command; release graph tests pin services, explicit database identity and migration ordering.

These checks do not replace full named-test inventory: per-test opt-in skips and unreachable helpers remain separate coverage concerns.


`TestHandlerTestMainRequiresExplicitDatabase` and `TestRouterTestMainRequiresExplicitDatabase` execute newly guarded test binaries with absent and whitespace-only configuration. They require exit 2 and the expected configuration error, without initialization output.

The child processes have a bounded context, a nonexistent socket, and forced `PGOPTIONS` with `default_transaction_read_only=on` and a statement timeout. Config-only pgx checks prove the read-only option survives an explicit URI; they do not connect. This also protects against a future reintroduced literal fallback. Parent suites still require a dedicated database.

For secondary-source cleanup regressions, run
`TestKnowledgePreparationSourceRevocationBothCommitOrders` and
`TestKnowledgeServerFailureLogsUsePersistedState`, audit pending orphan events,
then run existing collaboration memory/realtime dispatch tests in a second
process against the same owned database. Retain the ordered red/green evidence
and prove protected candidate/invalidation hashes were not changed by cleanup.

Task launcher verification must also assert that the recorded registry manifest,
actual database and API/Web listeners agree before the first fixture write.
An unrelated newly allocated worktree database is a setup failure, not a valid
substitute for the permitted database.

## 7. Wrong vs correct

Wrong compile probe:

```sh
go -C server test ./internal/handler -run '^$'
```

Correct compile probe:

```sh
go -C server build ./internal/handler
go -C server test -c -o "$TASK_TEST_BINARY" ./internal/handler
```

For an accidental execution, preserve command/time/environment evidence and perform only a bounded read-only audit. Current absence of reserved fixture rows cannot prove their previous contents were untouched; do not perform speculative cleanup or restoration.

Wrong: silently reuse a prior full-suite database or let a second source's rows
escape the fixture's cleanup scope. Correct: register every exact source fixture,
use a fresh migrated database per complete run, and inspect original failed state
before classifying a failure as environment-related.

## Test state must not inherit a repository

1. **Scope:** package tests that discover Git roots or write agent/editor sidecars, especially `internal/daemon/execenv`; applies to launcher-selected `HOME`, `TMPDIR`, `TMP`, and `TEMP`.
2. **Signature:** resolve the dedicated state directory, then run `git -C <state-root> rev-parse --show-toplevel` before any test. Keep explicit `GOCACHE`, `GOMODCACHE`, and `GOPATH`.
3. **Contract:** the directory must exist outside every checkout/worktree and return nonzero with empty Git-root output. Give each package private child directories. Evidence and read-only binaries may remain in a checkout; execution state may not. Container tests use their own executable tmpfs.
4. **Errors:** a Git root, an unexpected successful probe, or an unresolved directory refuses execution. Database isolation does not prevent filesystem writes to a discovered ancestor.
5. **Cases:** external dedicated directory passes; `<repo>/.omx/private/tmp` and any linked worktree child fail even when unique to the run.
6. **Verification:** exercise both rejection paths; replay the affected original package on a new database without changing assertions. Preserve the initial failure and any sidecar observation. Without a pre-run snapshot, do not assume the sidecar was new or reconstruct its former contents.
7. **Wrong/correct:** per-package directories inside `.omx/private` still inherit the repository. Put the state root outside Git, prove that boundary first, and retain raw first-run evidence.

Evidence: `11-04-core-business-e2e-acceptance/research/backend-tmp-isolation.md`; original execenv had 655 PASS / 5 FAIL, then 660 PASS on a fresh database with the Git-external state root. This does not retroactively make the original full run successful.
