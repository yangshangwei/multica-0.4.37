# Database test isolation

## 1. Scope / trigger

The handler and server-router test packages initialize shared fixtures in `TestMain`. A test-name filter does not suppress this initialization. Run these packages only against an explicitly selected, task-owned database.

## 2. Commands and entry points

- `server/internal/handler/handler_test.go:TestMain`
- `server/cmd/server/integration_test.go:TestMain`
- Execute tests after loading the task's private environment file; verify the database identity without printing its credentials.
- Compile without execution with `go -C server build ./...` or `go -C server test -c -o "$TASK_TEST_BINARY" ./internal/handler`.

## 3. Environment and execution contract

Both entry points require nonblank `DATABASE_URL` before creating a pool, running fixture cleanup/setup, or constructing a router. Missing or whitespace-only values print a content-free configuration error and exit **2**. Do not restore an implicit localhost database fallback.

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
| `-run '^$'` | Still executes `TestMain`; never use as a compile-only check |
| `go build` / `go test -c` | Compiles without executing `TestMain` |
| Explicit but unreachable database | Existing connection handling remains; exit status alone never proves tests ran |
| Reused whole-suite database with retained history | Inspect queued work and immutable timestamps; preserve it and run final verification on a fresh owned database |
| Secondary source fixture teardown | No pending ordinary event may reference a removed issue; protected knowledge history remains unchanged |

## 5. Good / base / bad cases

- Good: load the recorded private test environment, verify its database name, execute one suite, and confirm all expected names actually ran.
- Base: use `go build` or `go test -c` when only compilation is needed.
- Bad: an empty test filter against an older revision with no explicit environment can initialize and clean the default database even though no test body runs. Never rerun that unsafe path merely to obtain red evidence.

## 6. Required regressions

`TestHandlerTestMainRequiresExplicitDatabase` and `TestRouterTestMainRequiresExplicitDatabase` execute newly guarded test binaries with absent and whitespace-only configuration. They require exit 2 and the expected configuration error, without initialization output.

The child processes have a bounded context, a nonexistent socket, and forced `PGOPTIONS` with `default_transaction_read_only=on` and a statement timeout. Config-only pgx checks prove the read-only option survives an explicit URI; they do not connect. This also protects against a future reintroduced literal fallback. Parent suites still require a dedicated database.

For secondary-source cleanup regressions, run
`TestKnowledgePreparationSourceRevocationBothCommitOrders` and
`TestKnowledgeServerFailureLogsUsePersistedState`, audit pending orphan events,
then run existing collaboration memory/realtime dispatch tests in a second
process against the same owned database. Retain the ordered red/green evidence
and prove protected candidate/invalidation hashes were not changed by cleanup.

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
