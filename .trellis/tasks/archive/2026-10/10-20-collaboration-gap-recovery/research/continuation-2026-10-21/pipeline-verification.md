# Continuation pipeline evidence — 2026-10-21

The earlier ten-minute unified service timeout was resolved with disjoint,
long-budget executions. This is evidence for the explicitly executed stages;
it is not a claim that a new monolithic `make check` or the entire Playwright
inventory completed.

## Go execution boundary

Commands ran from `server/`, using Go 1.26.6 and the repository agent CLI guard.
All test databases are task-owned clones with migration 737. The initial
`psql` lookup failed because its installed directory was absent from PATH;
database preparation then used `/opt/homebrew/opt/libpq/bin/psql` explicitly.
No production database was selected.

`go-service-identities.txt` records all 1153 service top-level identities.
`go-service-partitions.json` assigns each exactly once:

| Run | Selected top-level identities | Final result |
| --- | ---: | --- |
| `go-service-partition-1.jsonl` | 109 | 397 including subtests passed; zero failure/skip; 203.245 s |
| `go-service-partition-2.jsonl` | 354 | 913 including subtests passed; zero failure/skip; 302.457 s |
| `go-service-partition-3.jsonl` | 690 | 672 top-level passed and 18 Redis cases initially skipped |
| `go-service-redis.jsonl` | 18 original omissions | All 18 passed on an independent Redis instance |

The union contains **1153 distinct passing top-level service tests**. Nested,
repeated and historical executions are not added to this union. Every partition
uses `-race -p 1 -parallel 1 -count=1 -timeout=30m`; the exact selector and database
alias are retained in the command manifests and `run-go-partition.py`.

Partition 2's first attempt is preserved as `go-service-partition-2-first.*`.
Its sole failed assertion was the stale ordinal hold fault injector, independently
reproduced and corrected in [the hold report](hold-unknown-commit.md). The final
partition is green. Selected pre-commit rejection and lost commit reply now hit
`SetCollaborationTargetHold`, and retain full rollback/one-audit/no-run assertions.
Moving that injector to its own support file was checked without changing its
body. An independent `git archive HEAD server` plus only the two commit candidates
also passed the two hold cases, five results including subtests, under race.
The self-contained commit is `b67c8253f`.

The other Go packages ran in `go-other-packages.jsonl` and
`go-remaining-packages.jsonl`, using separate task databases and the same race,
serial and 30-minute budgets. Together with service they account for all 81
packages returned by the current `go list ./...`; no package reports failure.
The first detached remaining-package launcher ended before running tests and
was replaced with a tracked foreground execution; only the completed result is
counted. An additional **73 Redis tests passed** in `go-other-redis.jsonl`, using
`REDIS_TEST_URL` and `MULTICA_TEST_REDIS_URL` against the new task-only Redis at
loopback port 57244. It is separate from API Redis because these fixtures flush
their logical databases.

Non-Redis skips retain their actual identities/reasons in
`go-execution-summary.json`: Linux/Windows/root-only behavior, pg_bigm availability,
live GitHub integration, explicit performance/fixture-export switches, and existing
documented test limitations. They are not passing evidence. Real agent integration
remains uninvoked; the guard prevents ambient provider CLI execution.

`go vet -p 2 ./...` passed with empty output. The full Go results precede the
later, locally scoped command-history projection fix found by AC33; that fix has
its own failure-first and adjacent final-source verification.

## Actual Linux supervisors

Current source was cross-compiled with `GOOS=linux GOARCH=arm64 CGO_ENABLED=0`:

```sh
go test -c -o <task-temp>/continuation-linux/router.test ./cmd/server
go test -c -o <task-temp>/continuation-linux/daemon.test ./internal/daemon
go build -o <task-temp>/continuation-linux/multica-cli ./cmd/multica
```

`linux-binaries.json` records exact SHA-256 identities. `run-linux-routes.py`
runs them in the task-named Linux container against independent
`service_s6_linux_routes`, with explicit namespace privileges and no host/home
mounts. An initial Docker bind failed before tests because that daemon could not
see the host temporary path; `linux-router-first.*` preserves exit 125. The
successful launcher creates the container, copies only the three test-built
binaries, attaches, and lets the container remove itself.

All **six** current Linux tests ran and passed with zero skips:

- `TestCollaborationSquadCLIAndDaemonCapacityOneRoutesLinux`
- `TestCollaborationPartialResumeRoutesLinux`
- `TestCollaborationSingleAgentRuntimeRoutesLinux`
- `TestCollaborationHeldSquadAllowsDirectAgentRoutesLinux`
- `TestCollaborationRuntimeProofSurvivesOwnerDisableLinux`
- `TestCollaborationSquadCapacityOneRoutesLinux`

`linux-router.log`, `linux-router.jsonl` and `linux-router-command.json` preserve
the real provider/supervisor/HTTP/proof path. The partial-Resume case runs two
namespaces concurrently, withholds A's real sealed proof, lets B continue, then
requires a fresh explicit A Resume after proof arrives. These are synthetic
test executables on an actual kernel; no authenticated provider account or quota
is exercised. This invocation is not race-instrumented; the portable suites are.

## Production API/Web and Playwright

`source-environment-manifest.json` initially compared 6994 application files with
both fixed snapshots and found no difference. API health, listener PID, source
and configuration fingerprints, exact launch commands and Web build IDs are
retained in the neighboring `*.running.json` files. Subsequent test-only changes
do not alter those application artifacts; the command-history service repair
requires a refreshed API identity recorded separately.

| Environment | API | Production Web / build | Native renderer |
| --- | --- | --- | --- |
| Legacy collaboration | `localhost:18985` | `localhost:13905` / `SrpeaQzGK_I-BlKn9Vy0e` | `localhost:6079` |
| Password administration/managed fixture | `localhost:18996` | `localhost:13916` / `tP4r55FoDsuyXfi1EylGw` | `localhost:6090` |

The initial offer/history Web smoke passed **2/2**. Final mixed mention journeys
passed **Web 3/3 and actual Electron 3/3**; agent/squad/whole-issue control journeys
passed **Web 2/2 and actual Electron 2/2**. Their reports preserve every attempted
run, final source hashes, safe identities and wide/narrow/native 200% captures:

- [Mixed mentions, refusal, deduplication and offline edit notifications](mention-journeys.md)
- [Agent, squad and whole-issue controls with pending-proof explanation](control-journeys.md)
- [Independent final review](mention-control-final-review.md)

The dedicated password/admin-enabled journey was rerun from the real public
password login and native account shell. **Web 1/1 and actual Electron 1/1 passed**,
with zero retries/skips/flaky. Exact raw results are
`playwright-admin-web.json` and `playwright-admin-desktop.json`; commands used
the existing private `admin-test.py` launcher, one worker and zero retries.
They cover disabled-creator lifecycle handling, explicit double-disabled
successor selection, original receipt recovery, creator restoration refusal,
one handoff/audit and paused/no-AI reopening. Canonical DB tests own candidate
expiry, role/session/scope races, replay conflicts and atomic failure matrices.
The stale handoff's claim that S5 UI was unrun is superseded by these executions
and the earlier independently hash-checked S5 passes.

For synthetic managed enrollment, only the task-owned admin configuration's
`MULTICA_MANAGED_INSTALLATIONS_ENABLED` was changed to true after saving its
private original; the API was restarted under ownership checks. Public config
and new PID are recorded in `managed-control-environment.json` and
`gap-recovery-admin-20261021-api-after-managed.running.json`. Production creation
and execution rollout was not changed.

Both inherited static renderer helpers exited during the session. After checking
that their ports were empty, the coordinator restarted the known task-owned
helpers. `native-renderer-restart.json` proves both responses exactly matched
their built indexes and records new listeners. Final Electron tests ran only
after this check. The initial blank-window failure is environment evidence.

## Additional static and native checks

- UI exports: passed, 62 files.
- `node apps/desktop/scripts/verify-renderer-file-access.mjs`: passed all 16
  native assertions with Electron 39.8.7. Expected blocked-file diagnostics remain
  in stderr; no assertion failed.
- Scoped E2E strict TypeScript, ESLint and discovery: passed in the independent
  review. Existing Core/Views/Web/Desktop typechecks and focused suites remain
  attributed to their original current-task report rather than rerun counts.
- `pnpm knip`: exit 1, reporting 11 unused files, one dependency and one dev
  dependency. Every reported source/config path is unchanged from HEAD and
  outside this slice. No unrelated deletion or dependency edit was made.

## Limits

AC33's complete combined client journey and its discovered successful-history
projection repair are tracked in their own reports. Acceptance definitions remain
immutable; a matching test name alone never counts as execution. Full installed
Main startup, old installed clients, other-OS containment, real provider/account
billing and explicit opt-in/fault environments remain separate evidence scopes.
The task and its parent stay `in_progress`; no deployment or parent archive was
performed.
