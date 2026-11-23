# Research: Current-source RK1 fake normal/race preflight

- Query: Identify the exact safe, reproducible RK1 fake commands and prerequisites for committed candidate `b6c74033f1913e46418ec1b119da33236dcba805`.
- Scope: internal
- Date: 2026-11-03

## Findings

Use a **new clean task-owned worktree from committed b6**, as the current v12 worktree has concurrent E2E-only repairs. Run normal and race **serially**, each against a separately allocated and migrated `multica_rk1_*` database. The selected suite executes only test-created Provider copies/simulators through the real router, database, Runner, HostExecutor and current built Multica CLI. Do not run `run_rk1.py` or `TestRK1RealProvider`; keep the paid gate false and runtime25 consumed0.

The retained historical commands are in `research/real-model-rk1/fake-proof/final639-quiet-normal-race-summary.json`, `executions[0/1].actual.command`. They used source639/source9f72, Go1.27, migration763; they are historical evidence only. Each had62 named pass events,0 fail/skip,19 FakeProof fixture cases,22 public FakeProof trials and32 FakeProof physical launches. Do not describe those32 as the entire combined suite's launch inventory: `TestRK1EvidenceFaultProof` also creates faulted runs outside `TestRK1FakeProof`.

### Pinned tools and environment

Current Go executable: `/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go`, SHA256 `a1c83801d1756c3eca78366c6b585f2c21c20694fb1c7eb92c446a0580420412`. GOROOT is its parent directory, `/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64`. This matches `server/go.mod:3`, the v11 pinned-tool record and v12 `adapter-env-preflight-v12.py:13`.

Put that Go bin directory first on the **test process PATH** as well as invoking it explicitly. `rk1BuildEnv` (`server/cmd/server/rk1_entry_test.go:265-285`) runs a nested `exec.CommandContext(..., "go", "build", ..., "./cmd/multica")`; a top-level absolute Go path alone does not pin that CLI build.

Use private0700 per-run HOME, output and cache directories; TMPDIR must be private and outside every Git checkout. Pin `GOMODCACHE=/Users/artisan/go/pkg/mod`, `GOTOOLCHAIN=local`, `GOENV=off`, `GOPROXY=off`, `GOSUMDB=off`, `GOTELEMETRY=off`, `GOMAXPROCS=2`, `GOFLAGS=-p=2`. For race, retain the available macOS C toolchain rather than disabling cgo. No cache/module download or installed Agent CLI is needed. Keep `DATABASE_MAX_CONNS`, `DATABASE_MIN_CONNS`, `MULTICA_RESOURCE_PUBLISH_DIR` out of the ordinary test environment.

Do not source the application dotenv as shell code. The coordinator should parse its private configuration as data and supply only the exact allocated DB URL, not inherited API deployment configuration. Do not copy database credentials into public research/log summaries.

### Database allocation and migration

The coordinator allocates **two new** names, for example `multica_rk1_b6_normal_20261103_01` and `multica_rk1_b6_race_20261103_01`, and verifies no existing database owns either name before creation. These names are proposed, **not allocated by this research**. `rk1RequireDatabase` (`rk1_fixture_test.go:49-62`) skips unless the URL's actual database begins `multica_rk1_`, verifies the pool is connected to that exact name, and rejects a nonempty `MULTICA_KNOWLEDGE_CONTEXT_ENABLED`.

The retained clone source was `mem_template`; it is acceptable only after the coordinator verifies that recorded template still belongs to this test infrastructure. Do not use an API/browser/failed historical database as the clone source. A clone from an older template must run current migrations. Candidate b6 contains **`764_collaboration_ai_policy`**, absent from the final639 migration763 proof; stopping at763 is insufficient. Migration764 adds `issue_collaboration.ai_policy`, which new generated readers expect.

Coordinator-side allocation shape (URLs come from private data, not literals in this file):

```sh
createdb --maintenance-db="$RK1_ADMIN_DATABASE_URL" --template=mem_template "$RK1_TASK_DATABASE_NAME"
test "$(psql "$RK1_TASK_DATABASE_URL" -Atqc 'SELECT current_database()')" = "$RK1_TASK_DATABASE_NAME"
env DATABASE_URL="$RK1_TASK_DATABASE_URL" GOTOOLCHAIN=local GOENV=off GOPROXY=off GOSUMDB=off \
  /Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go \
  -C "$RK1_CANDIDATE/server" run ./cmd/migrate up
psql "$RK1_TASK_DATABASE_URL" -Atqc 'SELECT current_database(),max(version) FROM schema_migrations'
```

Use a credential-free/public allocation receipt containing actual DB name, template name, latest migration764, allowed package `./cmd/server`, exact selector, fake-only status and Provider budget0. Keep its URL privately. If a child receives this DB permit, it must not create/migrate/drop/switch databases. Preserve both databases after any failure; do not terminate unowned connections or raise shared limits. Check connection headroom first. These are future coordinator commands, not actions taken by this research.

### Source/framework binding before execution

Run the canonical script **against the new clean worktree** and write a new private output path; do not overwrite `source-manifest.json` or historical manifests:

```sh
python3 /Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-context-handoff-runtime/research/real-model-rk1/source_manifest.py \
  "$RK1_CANDIDATE" "$RK1_PRIVATE_ROOT/source-before.json"
```

The script hashes tracked regular files under `server`, `apps`, `packages`, `scripts`, `Makefile`, `package.json`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`. Digest input is sorted `path NUL file_sha256 LF`. It records actual HEAD and dirty inventory. Require committed b6 and clean product roots **before** execution. The canonical product digest does not cover `e2e/`, so record the all-source/commit identity separately. A later Next-generated `next-env.d.ts` rewrite must keep its raw identity and the precise existing normalization contract; do not silently omit it or call a dirty snapshot clean.

Read-only current-file/index scan is retained in `source-comparison.json`:7984 product files, raw source ID `62e12d358a8d2af1a5ca7e569c696dbe4a4eae37479fbb3905280a832c26cdf4` at research time. This is **not final canonical registration**, since the current v12 worktree is concurrently active. It found128 changed product paths (24 added) versus final639. Changes include creation/recipient/source-picker behavior, delivery/exact human completion and AI-policy764, router/service permissions, generated SQL, and frontend contracts. Runtime Runner/checkpoint/recall framework files themselves remain byte-identical, but the combined source and Go toolchain differ, so historical passes cannot substitute for these runs.

All17 `server/cmd/server/rk1_*_test.go` files currently hash to `5eaea44f480f8e94a9a90d35025ca2d68d76c895c5bf70fa902da37c5bed94c9`, matching reviewed registration. Framework digest is SHA256 over sorted basenames, each as `basename NUL decimal_length NUL content`. Recompute and record it in the new worktree before/after; no registration/`runnable` edits are needed for fake tests.

### Exact test commands

After the coordinator has supplied the new clean worktree, exact normal/race database permits, private0700 homes/output roots and external temp directories, use the following template from the candidate root. `RK1_NORMAL_DATABASE_URL` and `RK1_RACE_DATABASE_URL` are private coordinator-provided values; shell task variables below do not repurpose system HOME.

```sh
RK1_GO=/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64/bin/go
RK1_GOROOT=/Users/artisan/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.6.darwin-arm64
RK1_SELECT='^(TestRK1FakeProof|TestRK1Evidence.*|TestRK1HandoffAnswersRequireTheirExactLineCounts|TestRK1ProviderWithoutMarkerCannotEnterTestMain|TestRK1TraceSnapshotWaitsForCompletedHandler)$'

env -u MULTICA_RUN_REAL_AGENT_SMOKE -u MULTICA_KNOWLEDGE_CONTEXT_ENABLED \
  -u MULTICA_RK1_EXECUTABLE -u MULTICA_RK1_SCENARIO -u MULTICA_RK1_OUT \
  -u DATABASE_MAX_CONNS -u DATABASE_MIN_CONNS -u MULTICA_RESOURCE_PUBLISH_DIR \
  -u PGOPTIONS -u ANTHROPIC_API_KEY -u OPENAI_API_KEY \
  DATABASE_URL="$RK1_NORMAL_DATABASE_URL" HOME="$RK1_NORMAL_HOME" TMPDIR="$RK1_NORMAL_TMPDIR" \
  RK1_FAKE_PROOF_OUT="$RK1_NORMAL_OUTPUT" \
  PATH="$RK1_GOROOT/bin:/opt/homebrew/opt/libpq/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin" \
  GOROOT="$RK1_GOROOT" GOCACHE="$RK1_NORMAL_GOCACHE" GOMODCACHE=/Users/artisan/go/pkg/mod GOPATH="$RK1_NORMAL_GOPATH" \
  GOTOOLCHAIN=local GOENV=off GOPROXY=off GOSUMDB=off GOTELEMETRY=off GOMAXPROCS=2 GOFLAGS=-p=2 \
  bash scripts/go-test-with-agent-cli-guard.sh -- "$RK1_GO" -C server test ./cmd/server \
    -parallel=2 -count=1 -json -timeout=40m -tags=agentintegration -run "$RK1_SELECT"
```

Wait for normal's actual exit and inspect all named results. Only then run the identical command with the race-specific DB/HOME/TMP/output/cache/GOPATH values and **add `-race`** before `-tags=agentintegration`. Preserve the actual child/guard exit, JSONL, exact environment fingerprint and output path for each run. Avoid a shell pipeline that replaces the process exit with `tee`'s exit; a structured Python subprocess recorder can stream to the private JSONL while retaining the real exit status.

This selects9 top-level tests, including all evidence-failure contract tests through `TestRK1Evidence.*`, without selecting `TestRK1RealProvider`. Build tag is `agentintegration && !windows`. No `run_rk1.py` invocation occurs. The guard creates its own CLI sentinels and fails if an installed Agent command is accidentally requested. The fake suite explicitly builds its own Multica CLI, and its `real_path_*` cases use only the test-owned `rk1-sim-claude` executable.

### Acceptance and post-run evidence

- Require actual normal/race exit0,62 named pass events each,0 fail/skip, exact9 top-level inventory,19 FakeProof cases and22 public FakeProof trial records with registered expected verdicts. Intentional `incomplete`/`blocking_review` fault cases are successful assertions, not unrecorded failed samples.
- Confirm no unexpected guard marker invocation; real Provider calls remain0 and runtime25 consumed0.
- Cross-check raw stdout/launch/trace/public hashes, original stopping/numbering, framework17 hashes and source/Go/tool identity before/after. Keep private ledgers/private stdout private.
- `TestMain` currently can exit0 if its database is unreachable (`integration_test.go:51-62`), so exit0 alone cannot pass. A positive actual DB identity and the full named inventory are mandatory.
- The first unexpected missing seal/incomplete result stops acceptance and all paid entry. Preserve it; do not extend the40m cap, sleep/retry within a sample, add an extra Runner call or weaken scoring. `recovery-preflight-20261102.md` explains the still-reachable one-second terminal-recording context mechanism; a quiet pass does not relabel old failures.

## Files found and related specs

- `server/cmd/server/rk1_entry_test.go:32-176,181-199,223-285,298-350` — selected fake matrix, marker guard, paid entry gate, nested CLI build, source/framework hashing.
- `server/cmd/server/rk1_fixture_test.go:49-62` — required DB prefix/identity and knowledge-off check.
- `server/cmd/server/rk1_run_test.go:81-128` — fresh private ledger/work directory and one Runner call.
- `server/cmd/server/integration_test.go:43-100` — explicit DB required, setup/cleanup, silent unreachable-DB exit hazard.
- `server/migrations/764_collaboration_ai_policy.up.sql` — current schema requirement.
- `scripts/go-test-with-agent-cli-guard.sh` — sentinel wrapper and real command exit preservation.
- `research/real-model-rk1/{framework.md,framework-registration.json,source_manifest.py,recovery-preflight-20261102.md}` — registered scope/hash, canonical source method and preserved failure limits.
- `.trellis/spec/server/{database-test-isolation.md,verification-environment.md}` — fresh DB allocation, exact identity, configuration separation and final audit contracts.

## Caveats / Not Found

No DB/service/test/Provider command was executed, no candidate/product/framework file was changed and no `git` process was invoked by this research. Only the research directory was written. The raw-index/file scan is an independent observation, not a substitute for the canonical manifest and clean-worktree check.

Still required before execution: choose the new clean b6 worktree; canonical source manifest and exact Go/tool hashes; allocate/migrate two new databases through764; provide per-run permits/private directories; establish a quiet serial window with the complete-check/browser owner. Do not run fake/race concurrently with the full check just because it uses a different database: prior timing-sensitive missing-seal failures require preserving the quiet-window condition. No remaining issue authorizes real Provider execution from this preflight alone.
