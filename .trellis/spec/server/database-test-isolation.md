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

Keep API pool budgeting separate from the environment of the test process.
`cmd/server/dbstats_test.go` verifies unset `DATABASE_MAX_CONNS` /
`DATABASE_MIN_CONNS` precedence, so globally exporting those keys changes the
behavior under test. Likewise, globally exporting `MULTICA_TASK_CONFIG_ROOT` or
`MULTICA_TASK_WORKSPACES_ROOT` overrides tests that intentionally select their
own `HOME`. Use a private package-level `HOME` with explicit `GOCACHE`,
`GOMODCACHE` and `GOPATH` when account isolation is needed; let individual tests
override their home and task-root variables. Retain an original contaminated
run and give corrected full-package replays a fresh database and run ID.

Claim payload tests that reuse the handler's intended-online runtime must
establish a current heartbeat before claiming. The production freshness fence
is `service.RuntimeClaimFreshnessSeconds` (150 seconds). Refresh only the
specific positive fixture; never add a global heartbeat loop or relax the fence
for stale/offline negative tests. A targeted pass or a later heartbeat-age log
does not reconstruct a cleaned-up runtime row from an earlier failed run.

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

## Performance attribution after a failed gate

Preserve the original frozen test, threshold, warmup and failed evidence.
A task-private Go overlay may add a separately named diagnostic test on a new
explicit migrated database; the virtual path must not exist on disk afterward,
and source/frozen-test hashes must match before and after. A passing diagnostic
is not a passing P95 gate and does not replace an original skipped/failed identity.

Mark the exact request context before collecting pgx query/batch/acquire timings.
Exclude fixture creation, start and preview when reporting closure cost. Sum
complete batch spans once; per-statement batch events are counts, not extra
duration. Query timings include transport, decoding and client processing, not
pure server execution. Do not label untraced residual time as CPU time.

The existing iterationCapacityTracer counts the whole handler-pool workload;
the P95 test does not reset it per endpoint. Its one-millisecond observer samples
only PostgreSQL Lock waits. Zero observed samples cannot rule out short locks,
I/O, CPU, pool waits or allocation. Reassemble Go JSON Output fragments before
parsing an embedded I1_P95_JSON record.

Record the actual PostgreSQL container image architecture as well as the host,
engine, server/toolchain versions and shared connection budget. A fresh database
on an amd64 image in an aarch64 engine is a different environment from native
ARM64; this does not itself establish which mechanism caused a latency failure.
Do not restart another session's shared cluster to standardize a benchmark.

## Prepared-plan and statistics diagnostics

### Scope and capture contract

Use this when EXPLAIN on a fresh fixture suggests repeated work but production
uses pgx cached statements. An `EXPLAIN ...` prefix creates a different statement;
it does not capture the production statement's actual cached execution plan.
On an explicitly owned diagnostic backend, session-only `LOAD 'auto_explain'`,
notice-level JSON and pgx `OnNotice` can capture actual canonical `SendBatch`
SQL/parameters/plans. Never change shared logging or restart another service.

### Signatures and identity

Record exact SQL/arguments and backend PID with the same session's
`pg_prepared_statements`: `name`, `statement`, `prepare_time`, `from_sql`,
`generic_plans`, `custom_plans`. Counters count choices, not plan-shape identity.
Keep `plan_cache_mode=auto` and index/planner switches unchanged. For a deliberate
cache-reset control, use `pgx.Conn.DeallocateAll(ctx)` on that backend outside
business transactions; raw server DEALLOCATE alone leaves pgx caches inconsistent.
Names can repeat after reset; compare preparation times and reset counters.

### Statistics and rollback contract

Capture actual counts, `pg_class.reltuples/relpages`, `pg_stats`, table analyze
counters/timestamps and valid index definitions before each phase. Finish all
no-statistics baselines before an explicitly recorded task-only ANALYZE. If
separating cache staleness from statistics, add a fresh no-statistics reprepare
control before ANALYZE rather than assuming they are independent variables.
Rollback does not reset WAL, buffers, tuple versions or all relation/analyze
metadata. In the recorded PG17 diagnostic, pg_stats rows reverted while
reltuples/relpages and cumulative analyze tracking persisted. Do not assume a
savepoint restores an uncontaminated target-phase statistics baseline.

### Validation and errors

| Evidence | Valid conclusion |
| --- | --- |
| EXPLAIN-prefixed SQL only | Instrumented alternate statement; cached production plan unknown |
| Same-session actual plan plus SQL/args/counters | Observed prepared execution for this backend/fixture |
| Fresh no-statistics reprepare retains scan/sort | Cache staleness alone is insufficient for this fixture |
| ANALYZE changes access using an existing index | Statistics/replanning direction; no missing-index proof |
| Logical rollback fingerprint equals | Covered business rows restored; physical/statistics reset unproved |
| Diagnostic PASS | Diagnostic completed; original frozen failure remains unchanged |

### Cases and required assertions

Good: same canonical SQL/arguments, actual plan notices, baseline/cache/reprepare
controls and explicit ANALYZE on fresh owned data; consume every result and Close.
Base: retain an honest alternate-EXPLAIN limitation when actual capture is absent.
Bad: replace a failing P95 with a passing diagnostic or insert ANALYZE/index DDL
into product requests based only on a fresh fixture.

Verify SQL/parameter populations, prepared/counter identity, full logical rollback,
source/frozen-test hashes, zero owned connections and instrumentation limitations.
Typed read transport probes must reuse generated scanners and distinguish missing
rows, server errors, Rows.Err and batch-close handling. Actual lock-wait/new-RC-
snapshot evidence and existing owner revocation/CAS/rollback regressions remain
separate from proof of an unimplemented optimization.

Wrong: sum nested plan times, subtract instrumented spans from a different endpoint
run, or infer generic-plan failure solely from a generic counter.
Correct: report inclusive node work once, separate plan/client spans, and retain
original tail-plan/shared-load uncertainty. Task evidence: branch-functional-
regression research/perf-cached-reprepare-diagnosis-1.md and
research/read-semantics-diagnosis-1.md.

## Token lookup failure diagnostics through existing DBTX

### 1. Scope / trigger

Use when a concrete *db.Queries handler field appears to block error-path
testing. `db.New(DBTX)` is an existing seam; a product interface refactor is not
required. Keep an original skipped placeholder and separately identify a
private diagnostic when the campaign freezes its inventory.

### 2. Signatures

Wrap DBTX.QueryRow and the returned pgx.Row.Scan, forwarding canonical SQL,
arguments and all generated destinations unchanged. The current public
HandleAutopilotWebhook path consumes GetWebhookTriggerByToken before request
body/persistence. Follow existing testutil request/URL-param/Call/Want/JSON
helpers.

### 3. Contracts

Allocate a fresh owned database before the handler TestMain. Separate diagnostic
pools use Config() copies and MaxConns1/MinConns0; never close global testPool.
Connection-local search_path=pg_catalog makes the unqualified canonical token
query fail without DDL or SQL rewriting. A PrepareError wrapping PgError42P01
is a real server prepare/parse failure, not evidence of SELECT execution.
Closing an owned pool exercises client acquisition failure, not server error.

### 4. Validation / error matrix

| Condition | Current expected witness |
| --- | --- |
| Server prepare/parse error, non-ErrNoRows | 500/internal error; one canonical lookup; no body/write or bad-credential debit |
| Separate closed-pool acquisition error | 500/internal error; disclose client failure boundary |
| Healthy lookup returns pgx.ErrNoRows | 404/webhook not found; bad-credential debit |
| Owned absolute limiter rejects first | 429/positive Retry-After; zero DB and body calls |

### 5. Good / base / bad cases

Good: real owned-pool failure plus exact query/Scan/error witness. Base: record
a researched seam without claiming an unexecuted test passed. Bad: close
testPool, rewrite canonical SQL, add a product compatibility layer, or invent a
passing contract for an explicitly unimplemented capability.

### 6. Required assertions

Require standard error JSON without token/row leakage, exact query/argument and
Scan witnesses, zero request Exec/Query/SendBatch/body reads, unchanged owned
webhook_delivery/autopilot_run/agent_task_queue fingerprints, and healthy global
testPool after faults. Record actual selected race terminals, source/frozen and
virtual-overlay identity, all owned-pool closes and final DBconnections0.
The lookup-error proof does not establish valid-token signing/delivery/worker
or Provider acceptance.

### 7. Wrong vs correct

Wrong: describe SQLSTATE42P01 as an executor-run SELECT when pgx reports a
PrepareError. Correct: preserve the outer error and unwrapped SQLSTATE and name
the exact failure phase. Reference: task11-02-branch-functional-regression,
research/webhook-lookup-diagnostic-1.md and its separately retained originalskip.

## Test state must not inherit a repository

1. **Scope:** package tests that discover Git roots or write agent/editor sidecars, especially `internal/daemon/execenv`; applies to launcher-selected `HOME`, `TMPDIR`, `TMP`, and `TEMP`.
2. **Signature:** resolve the dedicated state directory, then run `git -C <state-root> rev-parse --show-toplevel` before any test. Keep explicit `GOCACHE`, `GOMODCACHE`, and `GOPATH`.
3. **Contract:** the directory must exist outside every checkout/worktree and return nonzero with empty Git-root output. Give each package private child directories. Evidence and read-only binaries may remain in a checkout; execution state may not. Container tests use their own executable tmpfs.
4. **Errors:** a Git root, an unexpected successful probe, or an unresolved directory refuses execution. Database isolation does not prevent filesystem writes to a discovered ancestor.
5. **Cases:** external dedicated directory passes; `<repo>/.omx/private/tmp` and any linked worktree child fail even when unique to the run.
6. **Verification:** exercise both rejection paths; replay the affected original package on a new database without changing assertions. Preserve the initial failure and any sidecar observation. Without a pre-run snapshot, do not assume the sidecar was new or reconstruct its former contents.
7. **Wrong/correct:** per-package directories inside `.omx/private` still inherit the repository. Put the state root outside Git, prove that boundary first, and retain raw first-run evidence.

Evidence: `11-04-core-business-e2e-acceptance/research/backend-tmp-isolation.md`; original execenv had 655 PASS / 5 FAIL, then 660 PASS on a fresh database with the Git-external state root. This does not retroactively make the original full run successful.

## Compiler scratch and secure artifact fixture roots

### 1. Scope / trigger

Apply when a disk-capacity failure moves Go compilation or package fixtures to
another volume. An external directory outside Git can still violate artifact
root security through one of its ancestors.

### 2. Signatures

- `GOTMPDIR` selects Go compiler scratch. In the current local Go1.27 toolchain,
  `testing.T.TempDir()` also directly calls
  `os.MkdirTemp(os.Getenv("GOTMPDIR"), pattern)` (`src/testing/testing.go:1613`).
  `TMPDIR` controls `os.TempDir()`; it does not override a nonempty `GOTMPDIR`
  for these fixtures. Check the actual toolchain implementation rather than
  assuming Go1.26/Linux CI and local Go1.27 choose the same directory.
- `artifact.New(Config)` in `internal/artifact/packager.go` calls `openRoots`;
  `openSecureRoot(path, mode)` in `internal/artifact/files_unix.go` opens and pins
  every ancestor with `O_NOFOLLOW`, `Lstat` and `Fstat`.
- Preserve explicit `HOME`, `TMPDIR`, `TMP`, `TEMP`, `GOCACHE`, `GOMODCACHE` and
  `GOPATH` in the actual package child, not only the parent recorder.

### 3. Contracts

Artifact ancestors must be directories, not symlinks, and owned by the effective
user or root. Writable group/other bits are rejected unless the ancestor is
root-owned with the sticky bit. The pinned staging/output leaves additionally
require the effective user and exact modes `0500`/`0700`. A private `0700` leaf
below a user-owned `0775` mount does not satisfy this contract.

Keep fixture `HOME`, `TMPDIR` and, for Go1.27, `GOTMPDIR` on a verified private
chain. A single `go test` command uses the latter for compilation and
`t.TempDir()`; changing only `TMPDIR` leaves fixtures on the rejected mount.
Task-owned `GOCACHE` and logs may use a separate volume. If compiler scratch
must differ from fixture scratch, register separate compile-only and actual
test-binary environments/commands before executing them; do not silently change
the admitted test command. Retain source, failed configurations and logs and
allocate a new configuration/run ID after correction. Do not chmod a shared
mount or weaken artifact validation.

```python
child_config.update({
    "HOME": task_home,
    "TMPDIR": fixture_temp,
    "TMP": fixture_temp,
    "TEMP": fixture_temp,
    "GOTMPDIR": fixture_temp,  # Go1.27 T.TempDir also uses this key.
    "GOCACHE": task_compiler_cache,
})
```

### 4. Validation and error matrix

| Observation | Valid result / next action |
| --- | --- |
| Linker ENOSPC before package/test events | Compilation failed; selected tests are NOT_RUN, not PASS or a reproduced product failure |
| Recorder ENOSPC with missing raw/terminal evidence | NOT_RECORDED; audit owned processes and retain the database, without inferring execution from empty cleanup |
| `artifact.New` returns ErrDisabled before the target assertion | Inspect actual root paths, ancestor UID/modes and the opening boundary; this is not proof that the target guard failed |
| External fixture path traverses user-owned `0775` | Restore a trusted fixture path; preserve the rejection and shared mount permissions |
| TMPDIR is trusted but Go1.27 GOTMPDIR traverses `0775` | `t.TempDir()` still selects the rejected path; correct the actual selected key |
| Later capacity or narrow diagnostic is healthy | Current prerequisite evidence only; it does not assign the earlier failure's cause |

### 5. Good / base / bad cases

Good: actual toolchain-selected fixture roots pass both Git and ancestor checks,
with task cache/logs separately bound and a fresh owned package database. Base:
retain a static chain mismatch without claiming a missing syscall trace. Bad:
move every temporary key to one external mount and label fixture refusal a
product regression.

### 6. Required assertion points

Record the actual child configuration hash, toolchain version, `os.TempDir()`
and `t.TempDir()` selection, path/UID/mode chain, leaf modes, current capacity
and complete source bytes/modes before and after execution.
Re-run the original fixture and target assertion with unchanged deadlines and
inventory. A static ancestry audit is a prerequisite, not the runtime pass.
Keep compilation time separate from Go's package test timeout. Reuse a database
only when a compile-before-execution failure and matching before/after state
prove that fixtures never started; otherwise retain it and allocate a new one.

### 7. Wrong vs correct

Wrong: a `0700` child under `/Volumes/<user-owned-0775-mount>` is automatically
trusted, or a trusted `TMPDIR` makes Go1.27 `t.TempDir()` ignore `GOTMPDIR`.
Correct: inspect every ancestor and the actual toolchain-selected key, then
verify fixture initialization and the original target assertion. Keep cache
capacity handling separate from this artifact-root security contract.

Evidence: PAY-241 S2 `research/server-increment-2d-resume-r05-results.md/json`
retains the linker/recorder ENOSPC and artifact-root setup failures separately
from the original two r03 race verification gaps. Installed Go1.27
`src/testing/testing.go` SHA256 is
`8d2437cfdcfb82fd8807e5063783de1d50f70e589e80cad10e17d8ef23f724d9`;
the static source witness does not make an unexecuted profile or original
campaign pass. Subsequent corrected-profile results remain separately bound.
