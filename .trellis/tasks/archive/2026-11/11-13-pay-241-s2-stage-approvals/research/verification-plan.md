# Research: PAY-241 S2–S5 verification environment

- Query: identify an immediately usable isolated test/deployment setup, exact S1 helper paths, fixture gate profiles, and retained limitations.
- Scope: internal, read-only research; only this document was written.
- Date: 2026-11-13
- Source checkout: `/Volumes/artisan/code/2026/multica-0.4.37`.

## Findings

### 1. Recommended environment arrangement

Use the root checkout for implementation and focused package checks, and a separately frozen source copy for production Web/Desktop acceptance. Keep test databases, browser/API database, private process state, and reports separate. This matches the coordinator's intended arrangement and avoids replacing another task's live `.next` output.

Allocate a new external run root, for example `/Users/artisan/.multica/acceptance/pay241-s2-20261113-01`. This is a proposed new name, not an existing allocation. Recommended database labels are `multica_pay241_s2_20261113_01_service`, `_handler`, `_router` when needed, and `_api`; actual ownership must be recorded and verified before migration/tests. Do not select an existing database merely because its name resembles the proposal. Use new run IDs and databases for complete reruns after failures; retain earlier reports.

The current shell has Node, Go, pnpm, lsof, make, curl and Docker. `psql` is absent from PATH, but `/opt/homebrew/opt/libpq/bin/psql` exists. Add that installed directory to the private launcher's PATH. No installation is necessary.

Relevant constraints:

- `.trellis/spec/server/database-test-isolation.md:5`: `-run` does not suppress `TestMain` setup; no implicit/default database.
- `.trellis/spec/server/database-test-isolation.md:23`: explicit URL alone is not ownership; browser fixtures use another database, singleton gates are not isolated by workspace IDs.
- `.trellis/spec/server/database-test-isolation.md:42`: joint verification uses a fresh database for each DB-backed package. Child test permits name the exact package/database; children do not migrate, drop or choose a different database.
- `.trellis/spec/server/verification-environment.md:16`: API pool/publisher overrides must not leak into ordinary Go tests.
- `.trellis/spec/server/verification-environment.md:29`: private process HOME per package, with explicit GOCACHE/GOMODCACHE/GOPATH. Execution HOME/TMPDIR/TMP/TEMP must be outside every checkout, not under `.omx`.
- `.trellis/spec/server/verification-environment.md:31`: parse dotenv as data; canonical files passed into Make must reject decoded dollar signs and control characters before publication. Shell quoting does not make a Make include safe.

### 2. Exact S1 reusable assets

These paths exist. They are historical reference inputs; copy and bind new versions before use, never execute them against their S1 database or overwrite their records.

| Asset | Exact location | Reuse boundary |
| --- | --- | --- |
| Minimal JSON environment runner | `/Users/artisan/.multica/acceptance/s1-agent-identity-20261113-01/run.mjs` | Accepts `<config.json> <command> <args...>`; strips inherited DATABASE/PG/MULTICA/REDIS/ENV_FILE keys, merges config, verifies `current_database() == TASK_DATABASE_NAME`, then spawns. It hardcodes the root package.json for resolving pg; bind the new intended package path. |
| Namespace gate/E2E runner | `/Users/artisan/.multica/acceptance/s1-agent-identity-20261113-01/run-gated-e2e.mjs` | Accepts `<run-prefix> <spec...>`; hardcodes old source root, run root and API database. Set all three to the new run, retain before/after gate recording and `finally` restoration. |
| Historical incorrect gate runner | `/Users/artisan/.multica/acceptance/s1-agent-identity-20261113-01/run-gated-e2e-host-mismatch.mjs` | Failed-run evidence only. Do not use for namespace fixtures. |
| Built Web starter | `/Users/artisan/.multica/acceptance/s1-agent-identity-20261113-01/start-web.sh` | Loads the S1 registry manifest and starts an already built `next start`; hardcodes `s1-agent-identity`. Requires new name/registry/build identity before reuse. |
| Private configs | `/Users/artisan/.multica/acceptance/s1-agent-identity-20261113-01/{service,handler,browser}.json` and `browser.env` | Key schema/reference only; do not print or reuse credentials/URLs/ports/profile names. |
| S1 allocation and source binding | `.trellis/tasks/archive/2026-11/11-13-collaboration-agent-identity-baseline/research/{environment-allocation,validation-source}.json` | Explains the prior source overlay and databases; no current acceptance claim. |
| S1 build/fixture handoff | `.trellis/tasks/archive/2026-11/11-13-collaboration-agent-identity-baseline/research/e2e-implementation.md:35` | Exact native/cross-client binary types and environment prerequisites. |

No persistent allocation script exists among the S1 run root's scripts. `environment-allocation.json` is a result, not an executable allocator. The coordinator must allocate new databases with an owned helper using a parsed source connection configuration and preserve a new allocation record. No need to invent a missing S1 allocator or rerun a historical command blindly.

The S1 JSON runner only clears a subset of ambient state. For the new runner also make the per-test boundary explicit: remove API-only `DATABASE_MAX_CONNS`, `DATABASE_MIN_CONNS`, `MULTICA_RESOURCE_PUBLISH_DIR`; clear/replace inherited TMPDIR/TMP/TEMP with the package's external private state; do not globally set task config/workspace roots which override a test-selected HOME. Never output a raw connection error or connection string.

### 3. Necessary configuration keys

Only key names are listed. Their values must be new task-owned configuration, not copied S1 identities.

| Consumer | Required keys |
| --- | --- |
| Every DB package runner | `TASK_DATABASE_NAME`, `DATABASE_URL`, `HOME`, `TMPDIR`, `TMP`, `TEMP`, `GOCACHE`, `GOMODCACHE`, `GOPATH`, `PATH` |
| API | `POSTGRES_DB`, `PORT`, `BACKEND_PORT`, `APP_ENV`, `JWT_SECRET`, `MULTICA_AUTH_MODE`, `MULTICA_DEVICE_AUTH_ENABLED`, `MULTICA_PASSWORD_LIMITER_MODE`, `MULTICA_CLOUD_URL`, `MULTICA_DEPLOYMENT_ID`, `MULTICA_MANAGED_INSTALLATIONS_ENABLED`, `MULTICA_PLATFORM_ADMIN_ENABLED`, `DATABASE_MAX_CONNS`, `DATABASE_MIN_CONNS` |
| Storage and origins | `LOCAL_UPLOAD_DIR`, `LOCAL_UPLOAD_BASE_URL`, `FRONTEND_PORT`, `DESKTOP_RENDERER_PORT`, `FRONTEND_ORIGIN`, `CORS_ALLOWED_ORIGINS`, `ALLOWED_ORIGINS` |
| Web/browser binding | `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_WS_URL`, `REMOTE_API_URL`, `PLAYWRIGHT_BASE_URL`, `PLAYWRIGHT_BROWSERS_PATH`, `E2E_PASSWORD_AUTH`, `COLLABORATION_E2E_API_URL` |
| Desktop fixture binding | `COLLABORATION_ELECTRON_RENDERER_URL`, `COLLABORATION_ELECTRON_SOURCE_ROOT`, `COLLABORATION_ELECTRON_WS_URL`, `MULTICA_E2E_DESKTOP_OUTPUT_DIR` |
| Host controlled runs | `COLLABORATION_HOST_DAEMON`, `COLLABORATION_HOST_PROVIDER`, `COLLABORATION_HOST_CLI` |
| Optional existing namespace runs | `COLLABORATION_CROSS_CLIENT_DAEMON`, `COLLABORATION_CROSS_CLIENT_PROVIDER`, `COLLABORATION_CROSS_CLIENT_HOST_IPV4` |
| Private registry/process ownership | `ENV_FILE`, `MULTICA_DEV_HOME`, `MULTICA_DEV_TMPDIR`, `MULTICA_DEV_PROFILES_HOME`, `MULTICA_DEV_WORKSPACES_PARENT` |

Password startup must follow `.trellis/spec/web/frontend/e2e-run-environment.md:78`: password auth, device authentication off, Cloud URL cleared, and a single-process password limiter selected. More than one API requires the shared limiter/Redis contract. Host acceptance explicitly requires password opt-in and loopback/database guards in `e2e/fixtures/collaboration-host-runtime.ts:83`.

The root `.env` was inspected as data only. Its `GITHUB_APP_SLUG`, `GITHUB_WEBHOOK_SECRET`, `GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY` are empty. No payment-sandbox or release/rollback configuration key was found in root dotenv files or current process key inventory. This does not prove every external account or encrypted repository connection is absent; those were not inspected.

### 4. Execution sequence and concrete command shapes

The following are preparation commands for the coordinator after allocation, configuration and source binding. They were not executed during this research. `TASK_RUN_ROOT` and `TASK_SOURCE_ROOT` denote the newly allocated external run directory and selected checkout, respectively.

1. Parse the root configuration as data, select the actual PostgreSQL cluster, inspect only aggregate connection headroom, and allocate the new databases. Record ownership, expected names, configuration hashes and original `collaboration_execution_gate` flags. Preserve the root source env. The old S1 allocation recorded max100/current25 connections; that is historical and requires a fresh measurement.
2. Verify actual database identity in the same child environment used for migrations. Migrate each selected owned database, sequentially, before granting exact package test permits:

   ```sh
   node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/service.json" go -C "$TASK_SOURCE_ROOT/server" run ./cmd/migrate up
   node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/handler.json" go -C "$TASK_SOURCE_ROOT/server" run ./cmd/migrate up
   node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/browser.json" go -C "$TASK_SOURCE_ROOT/server" run ./cmd/migrate up
   ```

3. Run focused package tests with JSON inventory, explicit timeout/count and the appropriate config. Record matching named tests, passes, skips and failures; a compile-only or silent `TestMain` exit is not a test pass:

   ```sh
   node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/service.json" go -C "$TASK_SOURCE_ROOT/server" test -json -count=1 -timeout=180s ./internal/service -run '<reviewed-exact-selection>'
   node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/handler.json" go -C "$TASK_SOURCE_ROOT/server" test -json -count=1 -timeout=180s ./internal/handler -run '<reviewed-exact-selection>'
   go -C "$TASK_SOURCE_ROOT/server" vet ./internal/service ./internal/handler
   ```

4. Freeze the source copy and ensure workspace dependencies resolve within that copy. Bind root owned-file hashes to copied hashes. Compile current CLI, controlled Provider and daemon test harness without running `TestMain`:

   ```sh
   go -C "$TASK_SOURCE_ROOT/server" build -o "$TASK_RUN_ROOT/bin/multica" ./cmd/multica
   go -C "$TASK_SOURCE_ROOT/server" test -c -o "$TASK_RUN_ROOT/bin/host-daemon.test" ./internal/daemon
   go build -o "$TASK_RUN_ROOT/bin/closure-provider" "$TASK_SOURCE_ROOT/e2e/fixtures/collaboration-multi-account-provider.go"
   ```

   The daemon fixture invokes `__multica_collaboration_host_router_poller`. This is an explicit test harness binary, not the user's daemon. The closure Provider is different from `collaboration-host-provider.go` used by host/squad/switch fixtures; do not mix their callback protocols. Existing offline notification tests additionally need Linux ELF daemon/cross-client Provider binaries for the Docker architecture, not these native binaries.

5. Select private registry **and** `ENV_FILE` before any `make up`. For a new registry the allocator may rewrite database/ports; accept only its recorded owned allocation and reconcile all API/Web/WS/renderer configuration before fixture writes. For a pre-recorded environment, verify the manifest matches expected checkout/database/ports first. Both keys are required; importing only application variables can allocate in the default human registry.

   ```sh
   node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/browser.json" make -C "$TASK_SOURCE_ROOT" up C=api,web ARGS='--name pay241-s2 --ephemeral --web-mode production'
   node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/browser.json" make -C "$TASK_SOURCE_ROOT" status ARGS='--json'
   node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/browser.json" pnpm --dir "$TASK_SOURCE_ROOT" --filter @multica/desktop build
   ```

   `scripts/dev-env.sh:803` builds production Web and verifies source/config stability; `:813` starts `next start`. It refuses a foreign/busy listener or another environment using this checkout's Web build. A dedicated copy is therefore appropriate. `start-web.sh` can start a separately verified prebuilt Web, but must bind the new registry name and source/config hashes.

6. Start only the built renderer with `scripts/check-desktop-renderer.mjs <out/renderer> <owned-port> <provenance.json>`, as in `scripts/check.sh:136`. Capture PID, exact served-index equality, preload/renderer hashes and allowlisted origin. Do not invoke production Electron Main merely to serve a renderer; it owns daemon lifecycle. Existing tests launch their own isolated Electron renderer fixture. No claim about a full installed Main follows.
7. Preflight `/health`, `/api/config`, Web compiled API/WS addresses, `.next/BUILD_ID`, renderer served bytes, CLI/harness/Provider hashes, source files, and current database identity. Use one worker/zero retries:

   ```sh
   node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/browser.json" pnpm --dir "$TASK_SOURCE_ROOT" exec playwright test e2e/collaboration-multi-account-closure.spec.ts --workers=1 --retries=0
   ```

   Run the new S2 spec under the same bound environment when its entry exists. On failure, retain exact request, source, report and cleanup results under a new run ID before rerunning.
8. After owned daemons finish and run proofs are sealed, restore gate booleans to the **original allocation baseline**, keeping epochs monotonic. Stop only owned API/Web/renderer processes, verify both IPv4/IPv6 listeners closed, retain data/evidence, and recheck final source/build/config hashes. Do not stop human listeners or drop old S1 databases.

`make check` is useful general verification but is not a drop-in password closure runner: `scripts/check.sh:114` forces legacy auth and `:76–77` allocates one API and one shared Go database. A joint per-package acceptance needs the private package isolation adapter/runner. Reuse its production build, native renderer and ownership checks without interpreting skipped password/Provider cases as a full pass.

### 5. Host versus namespace gate profiles

- Host closure: `e2e/fixtures/collaboration-host-runtime.ts:106` locks `collaboration_execution_gate`, enables all three booleans, and returns an exact prior-state restoration callback. `collaboration-multi-account-closure.spec.ts:31` calls it. Run this profile independently.
- Namespace offline-result-notifications/live-revoke: require execution and creation enabled while host execution stays disabled. The S1 `run-gated-e2e.mjs:16` implements that task-only profile; `:21` restores flags in `finally` and records the after state.
- Host cutover intentionally rejects new namespace promotion with `409 execution_mode_unsupported`. S1 `supplemental-run1` retained this failure; `supplemental-run2` passed after correcting only the environment. Do not relax the production fence.
- Final restoration must compare with the original allocation, not only the state at test entry; an earlier interrupted run may already have left a gate open. See `.trellis/spec/web/frontend/collaboration-closure-qa.md:20` and the business-scenario gate contract in `e2e-run-environment.md`.

### 6. Bounded S3/S5 reuse map

These are source-discovered candidates, not new passing evidence:

| Concern | Existing anchor |
| --- | --- |
| Concurrent mutation/original request | `server/internal/service/collaboration_workflow_test.go:406` (`ConcurrentIdentityAndRevision`), `:633` (`LostCommitRecoveryAndRefusedKey`); `collaboration_comments_test.go:300` (`EditsSerializeAndUnknownCommitDoesNotRepeat`) |
| Cancellation provenance and claim race | `collaboration_command_history_test.go:21`, `:379`, `:407`; `collaboration_command_history_execution_test.go:86` |
| Host transient disconnect/retry/Stop | `server/internal/daemon/collaborationrun/host_runner_test.go:460`, `:500`; `server/internal/service/collaboration_host_runtime_test.go:267`, `:559` |
| Checkpoint/recording recovery | `host_recording_recovery_test.go:49`, `:121`, `:217`, `:233`, `:332`; `host_checkpoint_delivery_test.go:230` |
| Durable outbox/current authorization | `server/internal/service/collaboration_realtime_test.go:72`, `:135`, `:173`; `collaboration_realtime_runtime_test.go:11` |
| Notification replay/dedup and mentions | `collaboration_result_notifications_test.go:105`; `collaboration_notifications_test.go:14`, `:56`; `collaboration_comments_test.go:399`; Web/Electron `e2e/collaboration-offline-result-notifications.spec.ts` |
| Bounded recall/history/indirect access | `collaboration_comments_recall_test.go:97`, `:143`, `:188`; `collaboration_records_test.go:31`, `:134`; native/Web records fixtures |
| 100 independent observers | No existing PAY-241-capable harness found. `packages/core/api/ws-client.ts:87`, `:108`, `:147`, `:261` provide actual account-mode auth/ACK/scope subscription contracts. |
| Measurement recording precedent | `e2e/projects-p1.spec.ts:8` measures **two pages, one account, 30 mutations**, captures per-client times/version and partial failures. It is reusable recording guidance only, not TC24 evidence. |

S5 must pre-register independent account/session identities, exact event/version identities, start/end times, timeouts/missing observations and all samples. The existing project measurement uses `Date.now`; a new benchmark should use a consistent monotonic clock or documented calibration. Measure result submission to authoritative visibility separately from model generation/queue time. No existing result-writeback p95 harness was found by the bounded source search.

### 7. S4 readiness boundary

VCS integration code already exists: `server/internal/handler/github.go` exposes PR snapshot/CI facts, and `server/internal/integrations/vcs/gitlab.go:41`, `:76`, `:160` verify/parse GitLab signatures, PRs and pipeline status. These are reusable platform boundaries; product approval and deployment authorization remain independent. Empty GitHub App dotenv fields do not prove repository connections in encrypted storage are absent; no account/secret-store inspection or external call occurred here.

No real payment service, sandbox ledger, fulfillment observer or deployment/rollback target was identified in this bounded environment research. Keep S4 real cases NOT_RUN/BLOCKED until configured and authorized; local callback simulations and controlled Provider output cannot replace actual sandbox/payment/fulfillment evidence. No production release, money operation, real model account access or provider discovery was performed.

## External references

None needed: all recommendations derive from repository-owned contracts and preserved S1 artifacts. No external API or account was contacted.

## Caveats / Not Found

- This is research only: no environment/database creation, migration, test, build, gate mutation or process startup was performed.
- S1 helper paths and hashes describe a prior source/version and cannot be current evidence. New scripts/configs must be reviewed and recorded before running.
- Root dotenv key availability is a limited configuration check, not a full external integration inventory.
- The coordinator requested narrowing to operational environment preparation; S3 details here are a reuse map, not a complete gap analysis.
