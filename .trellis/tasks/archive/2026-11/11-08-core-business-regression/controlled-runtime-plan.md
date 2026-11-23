# Controlled runtime supplemental lanes

Preparation only. No model account, daemon, container workload, API, database or test body was started. Read-only Docker image inspection confirmed the cached prerequisites below. Playwright collection exited 0 with **17 unique identities in 10 specs**; Knowledge OFF/ON requires two configurations of one identity, so the full manifest has **11 lanes / 18 required executions**.

Private artifacts:

- `/Users/artisan/.multica/qa/core-20261108/controlled/run.py`
- `/Users/artisan/.multica/qa/core-20261108/controlled/manifest.json`
- `/Users/artisan/.multica/qa/core-20261108/controlled/cached-prerequisites.json`
- `/Users/artisan/.multica/qa/core-20261108/controlled/collection-inventory.json`

This is a separate static controller derived from the password controller, whose original file is unchanged. Its current hash and base-controller hash are recorded in `preparation-source.json`. `extend.py` is retained preparation history, not the execution entry point.

## Runnable groups after root releases the baseline API

| IDs | Actual specs/configurations | Required explicit binaries |
| --- | --- | --- |
| pw-11, pw-12, pw-13 | collaboration-host-runtime, collaboration-host-squad, collaboration-host-switch | Native daemon test binary, controlled Host provider, current native Multica CLI |
| pw-22 | collaboration-memory-runtime | Same daemon/CLI, distinct controlled Memory provider |
| pw-16, pw-17 | collaboration-knowledge-deployed, OFF then ON | Host binaries; separate fresh database/API startup and matching literal expectation for each flag |
| pw-07, pw-18, pw-20, pw-24 | collaboration-cross-client-execution, collaboration-lane-close-controls, collaboration-lifecycle-controls, collaboration-offline-result-notifications | Static Linux daemon test binary and controlled cross-client provider, architecture matching cached Redis image; reachable numeric Docker-to-host callback IPv4 |
| pw-14 | collaboration-installed-clients | Static Linux amd64 stdio relay and exact cached v0.6.7 CLI image |

The native fixture speaks ordinary Claude stream-json through a test-owned executable. Its actual daemon/HostRunner, capabilities, CLI calls, input files, stop/resume and proof transport are exercised; no commercial model account or real reasoning quality is claimed. Linux namespace fixtures use privileged task-owned containers and test-produced binaries. Installed CLI compatibility uses the cached old executable with `--network none` and a narrow stdio relay; this is distinct from a current CLI or installed desktop.

```sh
# Preparation-only output; no build or execution.
python3 /Users/artisan/.multica/qa/core-20261108/controlled/run.py

# Root executes sequentially after baseline/password browser work releases API.
python3 /Users/artisan/.multica/qa/core-20261108/controlled/run.py \
  --execute --run-id host-1 --ids pw-11,pw-12,pw-13,pw-22,pw-16,pw-17

python3 /Users/artisan/.multica/qa/core-20261108/controlled/run.py \
  --execute --run-id namespace-1 --ids pw-07,pw-18,pw-20,pw-24

python3 /Users/artisan/.multica/qa/core-20261108/controlled/run.py \
  --execute --run-id old-cli-1 --ids pw-14
```

Only selected groups compile their prerequisites. Compilation uses existing Go caches with `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`; `go test -c` does not execute TestMain. Commands are recorded with artifact hashes:

```sh
go -C server test -c -o <owned>/daemon-native ./internal/daemon
go -C server build -o <owned>/cli-native ./cmd/multica
go build -o <owned>/provider-host e2e/fixtures/collaboration-host-provider.go
go build -o <owned>/provider-memory e2e/fixtures/collaboration-memory-provider.go
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go -C server test -c -o <owned>/daemon-linux ./internal/daemon
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o <owned>/provider-linux e2e/fixtures/collaboration-cross-client-provider.go
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o <owned>/relay-linux-amd64 e2e/fixtures/collaboration-installed-cli-relay.go
```

## Cached prerequisites and runtime gates

| Cached reference | Observed identity | Architecture |
| --- | --- | --- |
| redis:7-alpine | `sha256:7aec734b2bb298a1d769fd8729f13b8514a41bf90fcdd1f38ec52267fbaa8ee6` | arm64 |
| installed v0.6.7 CLI | `sha256:8d5ede4df2fa4b66d499803b95428b729fcb6ba753b3800e9b931b968ee16104` | amd64 |
| multica-gap-full-main:20261022 | `sha256:db164dd5195d37ded1e6732eec4cc6b87decac66825be15fae9c7b87b2bac6c4` | arm64 |
| multica-collaboration-deb-acceptance:20261026 | `sha256:0d433ecf8ede66777494051568fd760468be288fd99d36d80bc68d022769471c` | arm64 |

Only the first two are used by this controller. Image presence/architecture is not runtime, Rosetta, privilege or containment evidence. The exact installed CLI version remains asserted by its original fixture.

The controller pins the existing local Docker Unix socket into children with private HOME. Its task-local Docker wrapper rejects pull/build commands and inserts `--pull=never` for fixture create/run commands, so missing cache cannot silently download. It checks cached image identities before fixture entry. At execution time the namespace lane uses a short owned cached-image command to resolve `host.docker.internal` to one IPv4; root may instead supply `--host-ipv4 <verified-address>`. Real callback/API connectivity, namespace support, stop/cleanup and proof behavior are still assertions of the original fixtures. No IPv4 was guessed or probed during preparation.

Every spec/configuration uses a fresh local database and private HOME/TMP/catalog. API max connections 6/min 1, password/managed mode, fixed allocated Web/API/renderer origins, and fresh deployment identity apply. Host specs open/restore their own actual gate; initial Host admission remains closed. Linux namespace and old-CLI lanes start with execution enabled and Host disabled as required. The runner rejects skips/retries, stops on the first failure, retains evidence and restores the original owned API afterward. It never touches another environment or drops retained databases.

## Packaged full Main remains outside this prepared runner

The cached container images above provide a harness only. Current-source macOS/Linux app directories or DEBs have not been prepared or hash-bound. Do not set `COLLABORATION_INSTALLED_IMAGE`, `COLLABORATION_HOST_INSTALLED_IMAGE` or a native executable merely to remove a skip.

Candidate packaging must use a separate source-matched checkout/output or wait until the shared renderer build is no longer served, because `package.mjs` rebuilds `apps/desktop/out` and stages the target CLI. Existing script commands for later root-owned preparation are:

```sh
CSC_IDENTITY_AUTO_DISCOVERY=false pnpm --filter @multica/desktop package \
  --mac dir --arm64 --publish never -c.directories.output=<owned-mac-output>
CSC_IDENTITY_AUTO_DISCOVERY=false pnpm --filter @multica/desktop package \
  --linux dir --arm64 --publish never -c.directories.output=<owned-linux-output>
```

These commands have not run; cached Electron/electron-builder platform artifacts must be checked first to obey the no-download boundary. Native macOS requires task-owned process HOME and `MULTICA_PACKAGED_ACCEPTANCE_HOME`, explicit `COLLABORATION_INSTALLED_MAIN_EXECUTABLE`, and a missing task-owned `MULTICA_DSH_PATH`. Bind source commit plus dirty overlay, executable, app.asar, bundled CLI and profile path. Linux Main requires the cached full-Main image plus exact current app directory/volume; Host-installed also takes the ASAR hash and optional DEB/executable/CLI pins. Review the offline-upgrade authentication/migration guide before preparing deliverables. Package manager lifecycle, updater restart, signature and notarization are separate acceptance facts.

## Verification of preparation

The private controller passes `python3 -m py_compile`; preparation-only CLI and Playwright collection passed. Runtime preparation/builds, local Docker DNS/capability checks and all 18 executions remain NOT_RUN until root invocation. This preparation introduces no product source changes.
