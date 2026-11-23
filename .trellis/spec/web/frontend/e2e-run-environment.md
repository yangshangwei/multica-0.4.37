# E2E Run Environment

## GitHub smoke bootstrap

Runner-owned smoke paths must be initialized in a shell step from `RUNNER_TEMP`
and exported through `GITHUB_ENV` before browser installation or allocation.
GitHub does not expose the `runner` expression context in job-level `env`; a
workflow using `${{ runner.temp }}` there is rejected before any job starts.
Step-level upload paths may still use that context. The workflow contract test
executes the initializer with a temporary path containing spaces.

Windows CI jobs enable `git config --global core.longpaths true` on their
ephemeral runner before `actions/checkout`. Historical evidence contains paths
over the default Windows limit; deleting those records or skipping the native
jobs is not a checkout fix. `agent-critical-path-workflow.test.mjs` checks the
initialization ordering for installer and native/runtime verification jobs.

`playwright.config.ts` starts no servers — "they must be running already". `baseURL`
resolves from `PLAYWRIGHT_BASE_URL`, then `FRONTEND_ORIGIN`, then
`http://localhost:3000`. Whatever listens on that port decides the suite's verdict,
so the web server's build mode is part of the test contract, not an operator preference.

## Rule

Run the Playwright suite against a production web server:

```bash
make check                         # isolated API/Go databases; builds production Web and Desktop

# For a task-owned, configured environment used by focused browser runs:
make up C=api,web ARGS="--web-mode production"
make status ARGS="--json"
make env-exec ARGS="-- pnpm exec playwright test --workers=1 --retries=0"
```

Never point the suite at `pnpm dev:web` or `next dev --webpack` (the `apps/web`
`dev` script). A dev server compiles routes lazily on first request, so the suite
measures webpack, not the product.

`make check` uses the registry allocator to create a separate `check-*` environment.
It copies the selected env file, sets legacy auth only in that task copy
(`MULTICA_AUTH_MODE=legacy`, `MULTICA_DEVICE_AUTH_ENABLED=false`), and runs static checks, bounded TypeScript
tests, isolated Go race tests, API startup, Desktop build/static preview and
production Web build/start, then Playwright in sequence. The Go database is never served by the API and is dropped without
forcing connections closed. API data and the environment record are retained
for inspection until TTL collection. Only processes started by this run are
stopped; an early failure or cleanup failure produces a nonzero exit.

"Classic" describes the suite, not a valid `MULTICA_AUTH_MODE` value. The server
accepts `legacy` and `password`. The task copy must override an inherited password
deployment and disable its managed-installation/platform-admin flags without
changing the source env; managed installation startup requires password mode.
`scripts/check.test.sh` covers all three overrides and source preservation.
Password administration requires its own production-Web run and synthetic actors.
Give independent specs distinct administrator accounts so login/reauth limits
remain active without one shared fixture exhausting the account budget.

### Native renderer origins in `make check`

1. **Scope:** the default check builds Desktop and serves its renderer on the
   registry-allocated loopback port, alongside production Web and the isolated
   API. Optional separately configured native origins remain validated inputs.
2. **Signatures:** `prepare_check_environment(source_env)` derives task-only
   `CORS_ALLOWED_ORIGINS` and `ALLOWED_ORIGINS` from the allocated Web and Desktop
   origins plus any explicit `COLLABORATION_ELECTRON_RENDERER_URL` or
   `CHANGELOG_ELECTRON_RENDERER_URL` loaded before preparation. The task copy
   then binds Collaboration API/WS and renderer/source-root to this allocation.
3. **Contract:** input renderer URLs must use HTTP(S) on `localhost`, `127.0.0.1`
   or `[::1]`, without credentials or port zero. Their paths do not enter the
   allowlist. Deduplicate origins, keep the source env byte-identical, and
   discard arbitrary production CORS/wildcards. Write the allocated
   `DESKTOP_RENDERER_PORT` into the task copy before reloading it: a stale source
   assignment must not separate the listener from the manifest and allowed
   origins. Complete these bindings before computing API provenance.
4. **Errors:** invalid, non-loopback, credentialed or unsupported-scheme input
   stops preparation before services start without echoing the rejected URL.
   WebSocket checks still require exact allowed-origin membership; Electron's
   disabled browser `webSecurity` is not a server-side exemption.
5. **Cases:** an unconfigured run gets the built native renderer automatically;
   an explicit second loopback renderer adds its origin, and duplicate origins
   add none. A remote renderer is refused even if source CORS allowed it.
6. **Verification:** `scripts/check.test.sh` covers actual preparation with fake
   allocation/services, stale source-file port/API/build paths, source
   preservation, both optional origins, IPv6 and nonsecret rejection. Its helper
   checks serve real built files, refuse missing output/busy ports and verify
   shutdown. Browser and WebSocket acceptance remains a separate gate.
7. **Wrong / correct:** do not widen production CORS or change only a later
   Playwright child's environment. The copied env, manifest, allowed origins,
   served build and owned listeners must describe the same check allocation.

### API-only native Knowledge configuration

1. **Scope:** `e2e/fixtures/project-knowledge-electron.mjs` adapts the shared
   native stub only for the Knowledge acceptance journey. It uses the existing
   explicit API/WS configuration and production renderer.
2. **Signature:** synchronous `runtime-config:get` returns
   `{ ok: true, source: "configured", config: { schemaVersion: 1, apiUrl,
   wsUrl, appUrl } }`. `/api/config.daemon_app_url` supplies the Server's Web
   address, derived from the owned `FRONTEND_ORIGIN`.
3. **Contract:** preserve the original schema/API/WS/app values and report the
   fixture's API-only configuration as `configured`. Capture the original
   callback on a plain event before setting the real IPC `returnValue` once.
   Leave the generic development stub unchanged; do not force `VITE_APP_URL`
   to bypass Server publication.
4. **Errors:** reject an unknown listener count, multiple original writes or
   changed payload shape. Refuse browser admission if live `daemon_app_url`
   does not match the allocated production Web origin.
5. **Cases:** a development stub with `appUrl` equal to the API origin can be
   interpreted as an explicit development address. API-only configured
   metadata must instead resolve the Server-published Web origin. Explicit
   development choices retain their normal product behavior.
6. **Verification:** the original fixture/production resolver combination
   reproduces the API-versus-Web mismatch. Five bounded fixture cases cover
   configured resolution, exact payload preservation, single real reply and
   invalid callbacks. Retain actual Web/native journeys, cross-origin v2
   traffic and source/artifact/cleanup bindings. Renderer acceptance does not
   establish packaged Main, installer or updater compatibility.
7. **Wrong / correct:** calling the original callback on the real event and
   then overwriting `returnValue` can send the development reply first.
   Capture its payload separately and emit one configured reply through
   `configuredKnowledgeRuntimeHandler()`.

### Password acceptance startup configuration

1. **Scope:** switching a task-owned API from legacy to password authentication,
   including managed-installation and collaboration-control acceptance.
2. **Signature:** `auth.ValidatePasswordConfig()` in
   `server/internal/auth/password_config.go` runs before the API can serve tests.
3. **Contract:** set `MULTICA_AUTH_MODE=password`, disable device authentication,
   clear `MULTICA_CLOUD_URL`, and explicitly select
   `MULTICA_PASSWORD_LIMITER_MODE=single` for one task-owned API process. Multiple
   API processes require `shared` and the same `REDIS_URL`.
4. **Errors:** a missing or unsupported limiter mode rejects startup with
   `password mode requires MULTICA_PASSWORD_LIMITER_MODE=single or shared`;
   `shared` without Redis and password mode with device/Cloud authentication are
   also rejected. A retained registry record is not a live listener.
5. **Cases:** a complete single-process fixture starts; an unchanged legacy
   fixture keeps its previous mode; changing only `MULTICA_AUTH_MODE` is an
   incomplete password fixture.
6. **Verification:** retain the startup command and exit, probe the actual owned
   listener, and check `/api/config` reports password authentication before
   running browser tests. Keep the existing `password_config_test.go` matrix
   and real browser login assertions; do not bypass startup validation.
7. **Wrong / correct:** do not treat a stale process record or health failure as
   a browser authentication defect. Complete the isolated configuration,
   restart only its owned services, and verify the live configuration. Apply
   this to the task copy, preserving the source deployment's authentication
   settings. Deployment guidance is in
   `docs/offline-upgrade.zh-CN.md`, sections 7.2 and 7.5.1.

### Password actors and managed profile prerequisites

1. **Scope / trigger:** an explicitly password-authenticated E2E actor that
   launches the production managed daemon. A running password API and a valid
   PAT alone do not establish an onboarded account or local profile owner.
2. **Signatures:** `auth.NormalizeUsername(raw)` in
   `server/internal/auth/password.go`; `TestApiClient.registerPassword()`;
   authenticated `PATCH /api/me/onboarding` and
   `POST /api/me/onboarding/complete`; `prepareManagedInstallationSession()` in
   `apps/desktop/src/main/managed-session.ts`; `managedProfileUser()` and
   `initializeManagedTransport()` in `server/internal/daemon/managed_lifecycle.go`.
   Desktop's `writeProfileUserId()` uses the same `.desktop-user-id` marker.
3. **Contracts:** usernames contain 3–32 ASCII letters, digits or underscores
   after trimming and are normalized to lowercase. Bound both the role and the
   unique suffix; a hyphenated email identifier is not a valid username. The
   password actor must submit the actual questionnaire/completion requests and
   assert the returned ID equals the registered ID and `onboarded_at` is non-null.
   The existing fixture uses `questionnaire.source: ["friends_colleagues"]`.
   Preserve the existing legacy branch for fixtures that did not opt in.

   For the managed branch, use a unique owned process `HOME` and named profile,
   the API-issued PAT and the actual enrolled/signed installation session. After
   `createHandoff()`, write the verified `managed.userId` in that same profile:

   ```ts
   const profileOwnerPath = join(profileDir, ".desktop-user-id");
   writeFileSync(profileOwnerPath, managed.userId, { mode: 0o600, flag: "wx" });
   expect(readFileSync(profileOwnerPath, "utf8")).toBe(fixture.creator.id);
   ```

   Pass the real handoff through stdin with `--managed-handoff-stdin`. Keep the
   deployment/installation/daemon/user binding and `/api/me` principal checks;
   profile contents never substitute for enrollment or daemon authorization.
4. **Validation and errors:** registration rejects hyphens or out-of-range
   usernames with HTTP 400 `invalid_username`. Missing `.desktop-user-id` rejects
   managed startup with `managed profile has no user identity`; an invalid UUID
   yields `managed profile user is invalid`. Mismatching managed daemon identity
   yields `managed profile identity mismatch`; a differing API actor, or one with
   `requires_account_setup` or `requires_password_change` set to true, yields
   `daemon credential principal differs from managed profile`.
   A null managed session is incomplete fixture admission, not permission to
   fall back to an unenrolled daemon or invented user marker.
5. **Good / base / bad cases:** good is a real registered/onboarded fixture user,
   its own enrolled session and PAT, plus an owner marker read back as that same
   user. Base is an unchanged legacy fixture with no managed launch. Bad is a
   random UUID or another user's marker, onboarding state edited around the API,
   or a borrowed default HOME/profile.
6. **Tests required:** use the canonical two journeys in
   `e2e/project-knowledge-automation.spec.ts` with setup in
   `e2e/fixtures/collaboration-detail-journey.ts` and
   `e2e/fixtures/project-knowledge-automation.ts`. Retain the original registration
   and startup failures, bind a fresh complete build after a fixture change and
   require both actual Web/Desktop journeys with zero skip/retry. Assert real
   daemon input, v2 validity/session proof, old-continuation refusal and fresh
   execution. Do not replace those with a mocked registration or new unit suite
   that duplicates the production username validator. Preserve profile/log/owner
   hashes, audit actual daemon/provider processes and the per-profile health
   port; a retained `daemon.pid` file does not prove the process is running.
7. **Wrong vs correct:** writing an arbitrary `.desktop-user-id` beside a PAT is
   wrong. Establish the actual registered/onboarded user and enrolled session,
   then persist its verified ID in the owned profile and retain all production
   principal/identity guards. Current source-bound receipts are under
   `.trellis/tasks/11-14-project-knowledge-automation/research/`; a private pass
   must still be reconciled with the final shared ROOT integration.

### Browser cache with a private acceptance home

1. **Scope:** a Playwright child whose `HOME` is a fresh task-owned directory.
   Playwright resolves its default browser cache from that home, so a cache
   present under the operator's home is not available without an explicit bind.
2. **Signature:** set `PLAYWRIGHT_BROWSERS_PATH=<absolute verified cache>` in
   the actual child environment passed to `subprocess.run(command, env=env)`.
   A variable present only in the parent shell or a recorder is insufficient.
3. **Contract:** retain the resolved Playwright package versions, registry
   revision, platform/architecture, installation-complete markers, executable
   and required resource hashes, ownership and modes. Bind this metadata to the
   selected runner and source before acceptance; preserve the isolated home.
   Cache reuse must not discover user accounts, install browsers or introduce a
   download fallback. Newly measured hashes establish current identity only.
4. **Errors:** missing binding, revision/package mismatch, redirected paths,
   changed bytes or unowned/writable installation directories reject the entry.
   A missing executable at browser launch is a prerequisite failure; preserve
   its raw result and do not count a test body that never ran as passed.
5. **Cases:** a bound, installed browser launches under a fresh home; the
   ordinary default-cache path remains available to existing nonisolated runs;
   a fresh home without a binding cannot borrow an operator cache implicitly.
6. **Verification:** use a credential-free child recorder to check unchanged
   argv and the explicit browser path, plus rejection controls for metadata
   drift. Then run the original browser test separately with one worker and
   zero retries, retaining its actual input/events and complete named terminal.
   Recorder success is not browser or deployed-product acceptance.
7. **Wrong / correct:** do not change the child's `HOME` to the operator home
   or infer historical executable hashes from a matching browser revision.
   Keep the private home, verify current cache bytes and set only the explicit
   `PLAYWRIGHT_BROWSERS_PATH` in the child environment. Preserve failed runs and
   use a distinct output label for the later original-test execution.

### Docker packaged Main prerequisites

1. **Scope:** Linux directory-packaged Main acceptance through the cached,
   explicitly selected task image.
2. **Signature:** `new InstalledMainClient({ image? })` resolves the fixture
   repository and `COLLABORATION_INSTALLED_APP_DIR` with `realpathSync()` before
   binding them; a named `COLLABORATION_INSTALLED_APP_VOLUME` is the alternative.
3. **Contract:** Docker must share both resolved bind paths. A host-readable
   directory or matching ASAR hash alone does not prove the bind is available.
4. **Errors:** a denied bind exits Docker with 125 before Main starts. Preserve
   that prerequisite failure rather than report it as an application failure.
5. **Cases:** a verified shared source/package runs; a previously verified
   package retains its pinned bytes; an unshared `/private/tmp` source fails.
6. **Verification:** probe the actual resolved paths with the selected cached
   image, retain source/ASAR/executable/CLI hashes, then verify the real Main
   flow. Recheck hashes and owned listener identity after relocation.
7. **Wrong / correct:** do not weaken assertions or change global Docker
   sharing to make a retained fixture work. Use a task-owned verified shared
   location, preserving the original worktrees and evidence. Check filesystem
   device IDs before `git worktree move`; across devices, create same-commit
   worktree copies and verify their bytes instead of forcing the move.

`psql` must be available on PATH. Database creation uses the PostgreSQL endpoint
in `DATABASE_URL`; a Docker container with the same database name is not proof
that this endpoint has the database.

### Debian installed Main acceptance

1. **Scope:** package-manager installation is a separate acceptance fact from
   running an electron-builder directory package. Reuse the full Main
   host-control journey in `e2e/collaboration-host-installed.spec.ts`.
2. **Signatures:** `InstalledMainClient` selects an explicit
   `COLLABORATION_INSTALLED_DEB` instead of `COLLABORATION_INSTALLED_APP_DIR` or
   `COLLABORATION_INSTALLED_APP_VOLUME`. Build from accepted bytes with
   `electron-builder --prepackaged <directory> --linux deb --arm64 --publish never`
   and a fresh `-c.directories.output=<directory>`; the full `package.mjs`
   wrapper deletes its old `dist` before rebuilding and is unsuitable here.
3. **Contracts:** pin `COLLABORATION_INSTALLED_DEB_SHA256`,
   `COLLABORATION_INSTALLED_EXECUTABLE_SHA256`,
   `COLLABORATION_HOST_INSTALLED_ASAR_SHA256` and
   `COLLABORATION_INSTALLED_CLI_SHA256`. The fresh UUID-owned container mounts
   only the `.deb` read-only and installs with real `dpkg`; Main runs from
   `/opt/Multica`, while the directory mode retains `/opt/multica`. Keep the
   installed Debian version and application version separately and verify the
   build tool's Debian prerelease conversion, rather than equating two reads
   of the same package metadata. The current builder maps application `-` to
   Debian `~`; the bundled CLI can retain git's leading `v`. Compare the CLI
   after the existing single-leading-`v` normalization and keep all raw versions
   in evidence (`bundle-cli.mjs`, `package.mjs`, `main/app-version.ts`).
4. **Validation / errors:** mixed package modes, missing bytes, invalid pins,
   hash drift, failed dependency/install status, wrong architecture/version,
   misplaced payload or desktop metadata must fail acceptance. A configured
   prerequisite failure is never a successful skip. Do not force dependencies
   or substitute a directory mount after installation fails.
5. **Cases:** a pinned `.deb` owns the expected executable/ASAR/CLI and desktop
   entry; the directory variant retains its original behavior; an unrelated
   standalone `multica` command survives installation unchanged. The desktop
   launcher is `multica-desktop`.
6. **Required verification:** retain installation output, package status,
   ownership, versions, all three installed hashes, desktop protocol/icon
   metadata and CLI-name separation. Run the real password-form,
   Stop/restart/explicit-Resume journey in both modes without retries. Preserve
   actual `file:` renderer paths, native IPC and fixture-only Provider scope.
   Attach boot observations before assertions and always close the owned Main
   container even if daemon evidence collection or cleanup rejects.
7. **Wrong / correct:** do not describe unpacking an archive as installation,
   rebuild accepted payloads during repackaging, or count the two modes as new
   originals in an existing inventory. Record the installer run separately
   with its source, package hash and actual execution results. Debian arm64
   container installation does not prove other OS installers, upgrades,
   signatures, vendor-account execution or release readiness.

### macOS packaged Main acceptance

1. **Scope:** macOS arm64 packaged `.app` control is a separate acceptance fact
   from directory or Linux package execution. Use the same full host-control
   journey with a fresh task-owned `MULTICA_PACKAGED_ACCEPTANCE_HOME`.
2. **Contracts:** the fixture must pass the real `.app` executable explicitly,
   map Electron `home` and `userData` into that acceptance home, and retain the
   app, executable, ASAR and bundled CLI hashes. Ad-hoc local signing is a test
   artifact and does not establish notarized release readiness.
3. **Fixture setup:** open `collaboration_execution_gate.creation_enabled`,
   `enabled` and `host_execution_enabled` only in the isolated acceptance
   database. Save the original booleans; restore them after daemon close or any
   setup failure, and keep the gate epoch monotonic. Wait for the enrolled host
   runtime to report `online` before creating Agents or issues; a heartbeat or
   claim race must not be recorded as a packaged-client result.
   Keep the repository Agent CLI guard active and explicitly pin
   `MULTICA_DSH_PATH` to a task-owned missing absolute path unless this fixture
   supplies its own fake DSH executable. Cold runtime discovery calls
   `dsh --profile multica --probe` even when daemon auto-start is disabled;
   resolving a guard sentinel on PATH is not an authorized fake provider.
   Startup registry, `ENV_FILE` and rendered API/WS origin checks follow
   [database test isolation](../../server/database-test-isolation.md#private-environment-registry-identity).
4. **Renderer setup:** fresh acceptance homes have no locale preference. Set the
   renderer locale to English before the password form is exercised. After real
   login, allow the short asynchronous daemon `starting` transition to settle
   before asserting the configured `autoStart=false` state is `stopped`.
5. **Validation:** require the real password form, native IPC, target Stop,
   paused restart, mixed-author explicit Resume, v3 lifecycle receipts, sealed
   ledger records and `host_exit_observed`. Run with zero automatic retries and
   zero skips. Keep the test-owned host fixture scope separate from authenticated
   Provider/account acceptance.
6. **Cleanup:** close the packaged Main and host daemon even when evidence
   collection rejects; verify all three gate booleans are false, the owned API/
   Web listeners are stopped, and no task-owned packaged process remains.

### macOS installed-upgrade protocol boundary

The packaged acceptance HOME redirects filesystem paths; startup still calls
app.setAsDefaultProtocolClient before its single-instance lock. When running
an installed old-to-new upgrade, capture NSWorkspace's exact default application
URL for the multica scheme and its existing same-bundle application URLs.
Use only a task-owned stable installation/profile path. After closing both
versions, unregister only newly added paths under that task prefix with
lsregister -u, then verify the original exact default URL and registration set.
Never unregister a preexisting application or launch/replace the user's install.
If exact restoration is needed, NSWorkspace's URL-based default-handler setter
can require OS consent; record the actual result instead of assuming recovery.
Acceptance HOME alone does not prove LaunchServices isolation.

Keep manual file replacement, actual Main/preload configuration reads, retained
renderer localStorage, real authenticated migration, automatic updater and
signature/notarization as distinct assertions. A synthetic startup endpoint can
prove native boot/profile plumbing but does not prove account migration or
complete network isolation. Attach request observers before launch when claiming
all startup requests; a listener added after firstWindow covers only later
renderer requests.

A synthetic password startup server must return auth_mode=password,
password_auth_available=true and device_auth_available=false. Assert the actual
password form and both fields are visible; Main/preload/localStorage checks can
pass while an incomplete synthetic response correctly renders an auth error.
Retain the original fixture failure and use a new numbered run for correction.

Production Web startup uses `pnpm --filter @multica/web build`, then
`pnpm --filter @multica/web exec next start --port <allocated-port>`. The build
receives `NEXT_PUBLIC_API_URL` / `NEXT_PUBLIC_WS_URL`, and `REMOTE_API_URL` points
to the task API. An existing listener is reusable only when it belongs to this
environment and its commit, source fingerprint, mode, build ID and configuration
match. A mismatch is reported without stopping the listener. Stop the explicitly
owned environment before rebuilding it; do not kill an unknown port owner.
Run one build/check per checkout; separate ports do not isolate its `.next`
directory. The launcher refuses a build while another registered Web from the
same checkout is still running. A checkout-local lock remains held through
production identity checks, build, startup and listener registration, so two
concurrent check runs cannot overwrite each other’s build between those phases.

## Why

The 2026-09-12 full validation ran against `next dev --webpack`. Cold compiles
measured 35.8 s (`/onboarding`), 31.0 s, 30.8 s and 30.4 s (`/[workspaceSlug]/agents/[id]`)
against these budgets:

- `playwright.config.ts` `timeout: 60000` — the whole test
- `ROUTE_CHANGE_TIMEOUT = 30000` in `e2e/navigation.spec.ts`
- ad-hoc 15 s visibility assertions in agent and MCP specs

A single 30 s compile consumes all three. Two cases failed for that reason alone —
`comments.spec.ts:25` and `navigation.spec.ts:12` — and were reported as remaining
E2E failures against the product. Both pass unmodified under `next build` +
`next start`: 82 expected, 0 unexpected, 0 skipped, 0 flaky at commit `23d771ca7`.
Neither spec file has been touched since.

## Failure signature of a dev-mode run

Suspect the server, not the spec, when:

- Timeouts cluster on the first visit to a route and the same route passes later.
- A URL assertion polls the old URL for its entire budget (`33 × unexpected value ".../issues"`)
  while the target route is still compiling.
- The web log shows `▲ Next.js <version> (webpack)`, `○ Compiling /<route> ...`, and
  `GET /<route> 200 in 30.4s`.
- Restarting the web process changes which cases fail, or a "warm-up" pass fixes them.

Do not repair specs on that evidence. Re-run against a production server first; a
locator or contract repair based on a compile timeout removes real coverage.

## Recording a run

An E2E result is only interpretable with its server mode. Record the launch command
and commit for both API and web alongside the results, as
`.omx/reports/main-upstream-merge-20260913/web.running.json` and `e2e-summary.json` do.
A pass/fail count with no server provenance cannot be compared against an earlier run.
The controlled launcher writes `api.running.json` and `web.running.json` under
`~/.multica/dev/envs/<name>/` (or `MULTICA_DEV_HOME`). They include the launch
command, PID, commit, source fingerprint, mode and Web build ID. `make check`
also saves `verification.running.json` after both services pass ownership checks.
Keep these files with test results. Production reuse verifies the current source
fingerprint, including untracked application code, so uncommitted repairs cannot
reuse an older build merely because HEAD stayed the same. The fingerprint
normalizes only Next’s generated `next-env.d.ts` route-types import between
`.next/dev/types/routes.d.ts` and `.next/types/routes.d.ts`; all other declaration
edits, file identity changes and application edits remain significant. In production Web
mode, API reuse also checks its source/configuration fingerprints; a changed
owned API is restarted, while an unknown listener is left untouched. Both
provenance records include the actual listener PID after readiness, and a
source/configuration change during API startup fails readiness.

Next rewrites the generated `apps/web/next-env.d.ts` route-types import between
`.next/dev/types/routes.d.ts` and `.next/types/routes.d.ts` during a production
build. The source fingerprint normalizes only that exact import line. All other
declaration content, file identity/mode and application source changes remain
checked; do not exclude the entire generated file or disable the build-time
source comparison. The launcher regressions exercise both an allowed generated
rewrite and a rejected concurrent application edit.

When several retained `check-*` records point to one checkout, `dev-env.sh up`
currently reuses the checkout's registered environment even if a different
`--name` was requested. Enter `dev-env.sh exec` and pass `ENV_FILE` from that
same effective environment. Before interpreting browser failures, compare the
reported API port with `apps/web/.next/routes-manifest.json` rewrite destinations.
A build that inherited another environment's `REMOTE_API_URL` can pass readiness
but proxy every application request to a stopped API. Preserve that failed run
as environment evidence, stop only the owned services, and rebuild with matching
configuration; do not weaken product assertions to accommodate it.

## Shared test services

The deployment MCP and Skill catalogs belong to the API, even when tests use
different workspaces. Serialize `mcp-desktop.spec.ts` with
`admin-resource-publishing.spec.ts` and live-directory cases in
`mcp-market.spec.ts` when they share an API/template directory, waiting for
fixture cleanup before the next suite starts. Concurrent publication can add a
seventeenth card while native acceptance expects the sixteen built-ins. Preserve
the catalog assertions and rerun sequentially; parallel execution requires
separate API environments, not just separate workspace IDs.

Redis enables the real per-IP auth budget (five send-code requests per minute by
default). A full E2E run creates many synthetic accounts from loopback. Configure
`RATE_LIMIT_AUTH` and `RATE_LIMIT_AUTH_VERIFY` for the task-only API's test
throughput; do not change production defaults or weaken login assertions. Go
Redis integration tests require `REDIS_TEST_URL`, using a separate Redis instance
because fixtures flush their assigned logical databases.

Password registration has a separate `register-ip` budget: 30 requests per five
minutes in `auth_password.go`. The email-auth variables above do not change it.
Run independent password specs in bounded batches or separate task-owned API
processes. With `MULTICA_PASSWORD_LIMITER_MODE=single`, restarting only the owned
API between independent specs gives each fixture a fresh budget while retaining
all rate-limit assertions within a spec. A 429 during fixture registration is
not evidence that the later product journey failed. Keep distinct synthetic
administrators per spec and preserve the account-level login/reauth limits.

`mcp-desktop.spec.ts` and `password-binding-desktop.spec.ts` serve
`apps/desktop/out/renderer` from their own checkout. An external
`COLLABORATION_ELECTRON_SOURCE_ROOT` does not redirect those paths. Build there,
or copy the matching renderer/preload from a verified same-source build and
compare hashes before running; otherwise the fixture proxy returns 502 and the
native root remains blank. The compiled-CLI acceptance in
`upstream-backports-cli.spec.ts` likewise requires `MULTICA_E2E_CLI_BINARY` to
resolve inside that checkout's `server/bin`, not a separate evidence directory.

Live changelog tests mutate a task-owned feed. Enter cleanup protection before
authentication, fixture setup or publication; a setup failure must leave the
original feed and remove temporary repositories. Do not run two publication
tests against the same feed concurrently. The raw source seed and published
release history are never writable test targets.

## Branch and fault-injection runs

Before switching an existing test environment between device and legacy login,
remove duplicate assignments for the overridden keys from its task-only env
file. Shell sourcing uses the last assignment. Verify the effective `/api/config`
and provider `/health` after restart; a matching first line in the file is not
evidence of the running configuration.

For a one-request browser fault, fulfill or continue the active route before
unregistering its handler. Unregistering first can consume the active request
and cause `Route is already handled`, preventing the intended failure from
reaching the application. Verify the failed response, retained state, and the
subsequent real write independently.

CORS-dependent regressions must assert that the response origin differs from
the page origin. Copy the deployment's actual CORS headers when injecting a
status, rather than inventing exposure headers. `password-branches.spec.ts`
checks that cross-origin `Retry-After` reaches the password form's cooldown.

Report exact test identities and actual executions separately from code branch
coverage. Inventory entries, conditional skips, route mocks, and HTTP-only
integration tests must not be presented as full browser or native coverage.

Password-authenticated browser fixtures must obtain both cookies from the real
`POST /auth/login` response. `multica_auth` is HttpOnly; `multica_csrf` supplies
the signed value the API client sends as `X-CSRF-Token`. Injecting only the auth
cookie permits reads but correctly produces `403 CSRF validation failed` on
collaboration control writes. Keep this fixture error separate from product
control refusals. Never attach credentials or cookie values to test evidence.

Before a retained Electron run, probe its exact renderer URL and compare the
response with the matching built `index.html`. A previous static-helper process
can exit while the API and production Web remain healthy. Restore only the
known task-owned helper after confirming the port is empty, then record its new
listener and artifact hash; no application rebuild is needed when source and
built artifacts still match.

## Responsive captures and stable task navigation

For a native portaled dialog, wait for its intended initial focus before filling
another field. In MCP setup, assert that Configuration name is focused, then fill
the secret input and assert its value before saving. Visibility alone can precede
deferred dialog autofocus: an early native fill was redirected into the name
field while the password remained empty. Keep synthetic credentials in these
fixtures and verify the focused control instead of adding a sleep.

Wait for a Base UI popup to become visible before asserting an option is absent
or sending Escape. An absent-option assertion can pass before the popup opens;
the queued opening can then outlive Escape and the next trigger click closes it.
After dismissal, wait for the listbox/menu to close (and for an exiting portal
to detach before measuring overflow at a new width). Assert the real closed
state and focus return rather than sleeping or forcing the next click.
`projects-p1-lifecycle.spec.ts`, `agent-category-grouping.spec.ts`, and
`iterations-audit-desktop.spec.ts` exercise these boundaries.

For an app with internal scroll containers, use `page.setViewportSize` to keep
Playwright's viewport state aligned with Chromium, then apply any touch
emulation. A raw CDP device-metrics override combined with a `fullPage: true`
screenshot can restore the old context viewport and pointer state: a 680px
capture may crop a desktop layout, and subsequent "narrow" steps may run at
1280px. Use a viewport screenshot (`fullPage: false`, `scale: "css"`) for this
case and assert width/height/coarse-pointer/touch-points both before and after
capture. Check the PNG dimensions too. A pre-capture no-overflow assertion alone
does not prove what the saved image shows. The dedicated scope-business fixture
owns this bounded helper; do not change the shared screenshot helper silently.

When a focused screenshot requires `toBeInViewport({ ratio: 1 })`, scroll the
target through all native scroll ancestors with
`scrollIntoView({ block: "center", inline: "nearest", behavior: "instant" })`
before measuring it. Start alignment can leave a fractional edge clipped even
after layout settles: the preparation-record fixture measured `y=-0.25` and
311.75 visible pixels of a 312px region. Preserve the full-ratio and bounding-box
assertions; changing the ratio or waiting longer does not correct that geometry.
Keep the failed capture, actual geometry and successful uninstrumented replay
as separate evidence (`collaboration-preparation-records-web.spec.ts`).

### Frozen native capture geometry

Use `captureNativeSection(app, page, focus, stage, outputPath, rootSelector)`
for tiled native account-detail evidence. After native size/zoom changes, require
two samples separated by rendering frames to agree on focus x/width/height,
root x/y/width/height/clientHeight and viewport width/height. Freeze dimensions
and tile offsets once. Rectangle comparisons retain the existing one-pixel
precision; client/viewport sizes must match exactly.

The frozen layout excludes focus y, current scrollTop and total scrollHeight:
planned tile scrolling and content growth below the target remain legal. Each
tile separately records all focus/root rectangle fields, requested and actual
scrollTop, maxScroll, viewport and achieved visible interval before capture.
After `webContents.capturePage`, require the same part and scroll position, and
persist both measurements even when an assertion fails. Retain the native
compositor, root bounds, continuous tile coverage and final
`coveredThrough >= initialFocusHeight - 1` assertions.

| Observation | Required result |
| --- | --- |
| Stable layout and a fully covered target | Save the native image and matching geometry |
| Planned scrolling between tiles, or content growth below the target | Keep frozen target dimensions; measure the actual legal scroll |
| Changed target/root layout after offsets freeze | Refuse with durable geometry; do not recalculate offsets to hide drift |
| Movement or resize during native capture | Refuse that part and retain before/after observations |
| Only the old initial height and final coverage were recorded | Keep the failure; height drift versus clipping/clamping is unproved |

Verify both legal scrolling and actual target/root/viewport drift against the
source predicates, then retain an unmodified production-Web/native-Desktop
journey with complete PNG/geometry hashes. The two Relations submenu gestures
use the configured Base UI hover trigger, await the exact Remove/Set-parent
item, then click and keep the existing API and parent-link assertions. Increasing
timeouts or lowering coverage does not establish those interaction contracts.
Root E2E fixtures are outside the current workspace ESLint base paths: an
ignored-file exit0 is a documented coverage gap, not a passing fixture lint.

Task links have two valid representations to test separately. Desktop AppLink
emits an absolute shareable URL, while Web can use a relative href; compare the
normalized full URL against the expected origin and task path. After a click,
`IssueDetailRoute` replaces the UUID segment with the API's human-readable task
identifier. Wait for that stable canonical route plus the expected task title,
including the desktop memory-router path. Accepting the transient UUID can
make a routing assertion race the normal canonicalization effect.

### Packaged-controller dependencies and current account pickers

Private packaging copies must also rebind workspace dependency symlinks into
the frozen source. A copied app whose `node_modules` still resolves
`@multica/*` to the live checkout is not isolated: a growing `.turbo` log between
ASAR stat/integrity and stream writing can shift every later member offset while
electron-builder exits zero. Exclude copied development caches, preserve runtime
dependency selection, validate every packed/unpacked member hash and archive
bounds, and compare extracted product outputs with the pinned source manifest
before functional acceptance. Preserve corrupt originals and rebuild into a new
directory. Do not substitute older product artifacts. The r2 regression found
41,855 failed member hashes after a single live log grew; the frozen rebuild
retained all 495 product output files and removed only cache logs/directories.

The plain-Node packaged Main controller and its host Playwright process must
resolve the same installed Playwright version. An absolute host `node_modules`
symlink is not available inside a container that mounts only the checkout.
Verify resolution inside the selected cached image before authentication; a
task-owned copy of the already installed `@playwright/test`, `playwright` and
`playwright-core` packages can supply that boundary without downloading or
changing versions. Keep the outer runner on that same copy to avoid two test
framework instances, and record this ignored dependency layout separately from
the product and harness fingerprints.

Invite, Transfer and request-information forms use the Recipient property
picker. Open its labelled listbox, enter the account UUID, use the exact-lookup
footer, verify the account name, select Use this account and assert the form's
selected-account status. `collaboration-offline-handoff.ts` owns the TypeScript
helper; the packaged controller's small `.mjs` counterpart follows the same
steps because plain Node cannot assume Playwright's TypeScript loader. Bind new
untracked harness files by content hash as well as the tracked patch. Keep all
receipt, revision, authorization and workflow assertions when adapting controls.

The exact lookup result can coexist with a recent/search option for the same
account. Anchor its card to the unique `Use this account` button, then assert
both the exact display name and `Account ID: <full lowercase UUID>` inside that
card before clicking. A popup-wide exact-name locator is ambiguous after a
previous transfer has made the recipient recent. Keep the TypeScript helper
and `collaboration-recipient-picker.mjs` aligned; do not resolve ambiguity with
`.first()` or remove the account-identity assertion. Replay the complete detail
and handoff journeys, preserving the original strict-locator failure.

Collaboration actions move between the primary controls and More actions as
responsibility and status change. In `uiWorkflow`, locate the exact requested
button and, if it is not visible, call the existing
`revealCollaborationDisclosure(page, "More actions")` before clicking. Apply
this to both inviting and transferring: Transfer can be primary for the creator
but collapsed for the recipient after a reload. Expand the section before
asserting that an unauthorized action is absent; keep capability/API refusals.

For an already visible toolbar action, click its unique exact button before
opening More actions. The Base UI root menu is modal: its pointer backdrop
leaves the dropdown trigger accessible but blocks neighboring toolbar buttons.
Opening a menu that lacks Submit result and then selecting the toolbar button
can therefore time out even while that button is visible, enabled and stable.

```typescript
await page.getByRole("button", { name: "Submit result", exact: true }).click();
```

When an action actually belongs in More actions, open the menu and scope its
unique menuitem to that visible menu. Dismiss an unrelated open menu through
Escape or its trigger and await closure before returning to a toolbar action.
Do not use a button/menuitem union with `.first()`, force clicks, remove the
normal backdrop or increase deadlines to conceal the wrong active surface.
Preserve the failed run and verify the real form, request identity and subsequent
business assertions in a fresh complete journey. PAY-241 R23 retained this
20-second click failure; R24's spec-only repair preserves the original 480-second
scenario deadline and all later M1/authorization/recovery assertions.

The password handoff chain submits through Submit result with explicit Result
and Basis. Bind its `delivery_confirmation.submission_id` to the immutable
`delivery.current` snapshot, then use Accept and complete with that same
`submission_id`, current observations and status-options version. A forged
non-creator completion must include a valid submission ID so its 403 proves
authorization rather than an incomplete request. Verify retained result/basis,
the accepted decision's actor/ID, notifications, current/participating lists and
idempotency alongside the UI actions. These password-only cases are skipped by
the default legacy deployment: retain their failed evidence and require a new
complete seven-case password run, not a sum of passing cases from prior runs.

For the closed-memory purge journey, explicitly open Version history before
asserting that Restore actions are absent and choosing a historical Purge action.
An absent-action assertion against collapsed content does not prove the closed
state's authority. Preserve the original failed run when adapting that visible
interaction; the observed collapsed state alone does not identify its cause.

## Collaboration drawers and visible authorization assertions

### Native startup waits for the renderer document

`e2e/fixtures/collaboration-native.ts:openDesktop().start()` must await both
the existing `desktopAccountAcceptance.start()` IPC call and
`page.waitForURL(new URL(COLLABORATION_ELECTRON_RENDERER_URL).href,
{ waitUntil: "domcontentloaded", timeout: 20_000 })`. Arm the URL waiter before
starting navigation. Production `loadRenderer()` returns void after initiating
`BrowserWindow.loadURL()`, so awaiting only `app.evaluate(start)` can leave the
page at `about:blank` and spend the entire five-second business assertion on
renderer bootstrap. Require the explicit renderer origin and propagate actual
navigation failure; leave task-heading and workflow assertion budgets unchanged.

The fresh native probe retained start returning at 6.425s, DOMContentLoaded at
13.006s and the requested task heading at 15.508s. After the fixture correction,
start returned after DOMContentLoaded and the heading appeared about one second
later. Keep both probes and replay complete native journeys; a successful IPC
call alone never proves the renderer is ready.

Resource drawers are modal. Before using background controls such as Copy link,
Notifications, Submit result or application navigation, dismiss the active
drawer with its real control or Escape and await its hidden state. Reopen the
resource through its visible toolbar when the next assertion belongs there.
Do not force clicks through the overlay.

The drawer can remain mounted and briefly visible during its closing animation.
After Escape, await the resource dialog's hidden state before calling a helper
that chooses between its inner navigation and the outer resource toolbar. A
one-shot `isVisible()` can otherwise select a closing drawer and wait on a
button that has become hidden. Keep the original failed run and use a new run ID.

An absence assertion against an inaccessible background can pass without proving
authorization. For example, close Sub-issues before checking that Edit source
fields is absent. Open an available More actions menu before checking forbidden
workflow actions, and show the Mention group before asserting revoked AI targets
are absent. Assert the containing panel/group is visible first; retain separate
API capability, refusal and unchanged-state assertions.

Pending recovery follows the current drawer contract: after reload, assert the
visible Needs attention status and activate its named resource button. Then
exercise the original recovery control and request identity. Do not substitute
the ordinary resource-discovery helper for this attention assertion. The
knowledge recovery test observes requests after reload and requires zero POSTs
while the original-operation GET resolves the retained candidate.

After editing a comment, await its exact rendered new content before opening a
version-bound deletion. A disabled confirmation showing Use latest version is
evidence of a stale observation, not a DELETE request that the server lost.
Keep scroll-restoration expectations unchanged when adjusting these controls;
record layout/scroll measurements separately if restoration still fails.

Evidence: `11-01-branch-e2e-acceptance/research/continuation-20261107/`.
The fixture repairs preserve the failed original runs and unchanged product
bytes; successful repaired journeys do not retroactively pass the originals.

### Immutable preparation and selectable completion fixtures

Complete `verifyAPIPaging()` before starting native clients whose initial deep
links read run records. Its direct synthetic SQL appends an immutable event but
publishes no WebSocket invalidation. Pass that already verified evidence into
`exercisePreparationRecords()` once; do not append twice or require the product
to refresh a frozen page after an unannounced database write. Preserve old cursor,
privacy, preparation summary and termination-proof assertions on Web and Desktop.

M1 completion evidence has a real expiry. Create fresh staging/package approvals
immediately before a complete replay and retain the original expired evidence.
Select the exact fixture `approvalID`, assert its radio is enabled and focused,
then press Space and observe the actual preview/authorization/completion requests.
A disabled radio cannot take focus; pressing Space on a previously focused Close
button can otherwise appear as a missing preview. Never extend an immutable
approval's expiry or change the product's selection guard to repair this fixture.

View-switch tests must use the exact current toolbar button and its
`menuitemradio`, then assert the target view, the expected row and unchanged
collection URL. A `text=List` locator can select a task card whose title contains
List and test navigation instead. R3 retains the actual wrong-card trace and
complete corrected file replay.

### Business-scenario fixture contracts

- Synthetic namespace control journeys (`collaboration-live-revoke-web` and
  `collaboration-terminal-reopen-web`) need `creation_enabled=true`,
  `enabled=true`, and `host_execution_enabled=false` in their owned database.
  Their signed runtime deliberately has no host capabilities and uses the v2
  namespace claim/permit protocol. Turning on the host gate makes preparation
  reject newly promoted groups before the run can be claimed. Keep them in a
  separate allocation from actual host S1/S2/S3 and compare the recorded live
  gate with the fixture protocol, not only with a launch script's intended
  settings. Restore original booleans with a monotonic epoch; never weaken the
  server's host capability check to admit this simulation.
- Empty Squad inventories intentionally show the welcome creation link instead
  of a header creation action. `localized-template-defaults.spec.ts` locates
  the exact localized link and its `/<slug>/squads/new` destination. Preserve
  localized roles, restaffing reuse and all-or-nothing collision assertions;
  do not restore a redundant product button to satisfy a header-only locator.
- Use `gapWorkflowObservations(issue)` for ordinary `/ai-policy` and `/lifecycle`
  refusal probes. `hostAuthority()` also adds `execution_view_version`, which
  those strict bodies reject with 400 before authorization. Preserve the exact
  403 assertion; malformed input does not prove an authority boundary.
- With multiple host targets, scope Resume to the row displaying the resource
  UUID, then verify request and receipt target IDs. Join click and response
  waiting with `Promise.all` so a rejected click does not leave an unhandled
  response promise that later reports only page closure.
- After accepted delivery, reopening retains the current handler. A creator
  who wants the original developer to submit again must explicitly transfer
  responsibility. Assert the developer's submit capability before/after that
  transfer, retained delivery history, and the still-paused execution lane.
- `openHostAcceptanceGate()` restores the state observed at test entry. That
  state can already be enabled from an earlier interrupted run. Final cleanup
  must compare the owned database's booleans with the original allocation,
  restore that baseline explicitly if needed, and preserve the monotonic epoch.
  Per-test `gate_restored=true` alone does not prove the deployment gate closed.

Canonical examples: `e2e/collaboration-business-scenarios.spec.ts` and its
task-local fixture. Evidence: `artifacts/qa/collaboration-business-scenarios-20261109/`.
The selected three business passes use deterministic Provider output; they do
not replace real-provider or model-behavior acceptance.

## macOS updater isolation and phase evidence

### 1. Scope / trigger

Apply when preparing a real packaged macOS updater journey. This is a
source-derived verification contract; the offline feed/parser preparation in
`11-02-branch-functional-regression/research/mac-updater-readiness-1.md` did not
execute an updater or establish native signing/restart compatibility.

### 2. Signatures

Use the original preload `window.updater.checkForUpdates()`,
`onDownloadProgress`, `onUpdateDownloaded` and `installUpdate()` paths.
`apps/desktop/src/main/updater.ts` enables autoDownload and calls
`quitAndInstall(false, true)`. Manual check is independent of the
`automaticUpdates` scheduling preference. Mac ARM64 consumes `latest-mac.yml`;
`latest-arm64` is the Windows ARM64 channel.

### 3. Contracts

Set the task-owned process `HOME` as well as `MULTICA_PACKAGED_ACCEPTANCE=1` and
`MULTICA_PACKAGED_ACCEPTANCE_HOME`. The latter redirects Electron home/userData;
electron-updater 6.8.3 uses Node `os.homedir()` for its base cache path. With the
current generated updaterCacheDirName, the Mac cache is
`<task HOME>/Library/Caches/@multicadesktop-updater`. Verify all actual paths
before checking, and bind the restarted process to the same owned profile.
Do not infer native relaunch environment propagation from initial launch alone.

Use explicit owned API/app/WS/updateUrl configuration under
`<task HOME>/.multica/desktop.json`. Preserve authentication mode. Feed SHA-512,
file sizes, version and ZIP selection must match exact distribution bytes. A
prepared template or explicit offline prerelease opt-in does not prove runtime
availability, native signature acceptance or publication.

### 4. Validation / error matrix

| Observation | Required interpretation |
| --- | --- |
| Node HOME/cache path outside the task | Reject updater entry; do not read or clean user caches |
| Only Electron home/userData were redirected | Cache isolation is unproved |
| Manual check with automaticUpdates=false | May still auto-download; attach observers before calling |
| Wrapper download event | Native Squirrel preparation/install/restart remain unproved |
| Native error or absent automatic restart | Preserve failure; no manual copy/relaunch substitute for success |
| Offline feed/parser validation | Preparation only; actual updater gate stays NOT_RUN |

### 5. Good / base / bad cases

Good: exact unchanged old/new bytes, owned feed/profile, observed Node/Electron
paths, transport, native preparation, explicit install and actual restarted
candidate. Base: retain an honest offline preparation result while conditions
are missing. Bad: use a download callback or manual app replacement as updater
installation evidence.

### 6. Required assertions

Require real check ok/available/version, ZIP transport/checksum, native
readiness/errors, same installation path, candidate executable/ASAR/CLI hashes,
actual restart and retained supported profile state. Capture the dynamic
Squirrel loopback proxy port and its owner; fixed browser ports alone are not a
complete updater cleanup. Audit task-added protocol registrations and exact
original default/set, all owned app/helper/feed/proxy processes and ports.

### 7. Wrong vs correct

Wrong: set only MULTICA_PACKAGED_ACCEPTANCE_HOME and assume updater writes are
isolated. Correct: also set process HOME, verify actual Node/cache paths and
restarted-profile identity, then collect each production updater phase without
changing its retry, download or install behavior.


## Complete local gate: Desktop and Collaboration prerequisites

`make check` owns the default native fixture environment as well as production
Web. Its copied task env binds `COLLABORATION_E2E_API_URL` and the native WS URL
to the newly allocated API, and records the current checkout as the native
source root. Never carry a prior run's API port or build directory into the new
environment; the source env remains unchanged.

Build `@multica/desktop` before Playwright so the default native fixtures receive
this checkout's `out/renderer` and `out/preload`. The check starts a loopback
static preview on its registry-allocated renderer port, with Vite config loading
disabled. It does not launch production Electron Main or the daemon. Include
that exact renderer origin in API CORS/WS allowed origins, verify the served
index matches the built file and the listener belongs to this check, and retain
renderer PID/path/hash provenance. Cleanup stops only the recorded renderer,
Web and API processes, including failure paths.

Providing these endpoints makes human Collaboration Web/native suites runnable;
keep their existing password/admin/provider/installed-agent opt-ins unchanged.
An unconditional Knowledge native case needs the explicit renderer/source root;
it must not acquire a new skip to compensate for missing preparation. The
password-session persistence test has its own synthetic password API and needs
only the same built Desktop assets, not a real password-mode server.

## Controlled Agent output allocation

`collaboration-runtime-output.ts` admits its current task before any database
gate write, actor registration, upload inspection or daemon launch. Its historical
exact database and storage guard remains available only when every new allocation
environment field is absent. A partial or invalid explicit allocation must fail;
it must never fall back to the historical paths.

For the 2026-11-20 full regression, explicitly set `COLLABORATION_OUTPUT_TASK`
to the task's actual active or archived path (see the lifecycle contract below),
`COLLABORATION_OUTPUT_STATE_ROOT`, `COLLABORATION_OUTPUT_ALLOCATION_FILE` and
`COLLABORATION_OUTPUT_ALLOCATION_SHA256`. The hash pins the complete JSON record:
task, source root, private state and storage roots, database name and connection
URL digest, API/Web/renderer origins and deployment ID. The task directory must
exist under the exact checkout. Keep the record mode0600 within an externally
allocated owner0700 state directory; storage must be an owner0700 descendant and
equal both `LOCAL_UPLOAD_DIR` and `COLLABORATION_OUTPUT_STORAGE_ROOT`.
Reject symlink path segments, noncanonical paths and mismatching ownership.

The database URL must identify the exact nondefault loopback database and contain
no query or fragment. `pg.Client` can honor `?host=` and `?port=` overrides even
when `new URL(...).hostname` is loopback. Pinning that URL does not remove the
override: reject it before connecting. The Node guard suite verifies the actual
effective PG connection parameters without opening a connection, as well as
historical admission, fresh allocation, partial/malformed records, task/source
mismatch, storage escapes, permissions and origin/deployment drift. POSIX ownership
verification explicitly skips on platforms without `getuid`; it does not establish
Windows acceptance.

Retain each allocation and source/artifact hash set with its browser report. If a
stopped container receives a new ephemeral port, record and pin its new exact
connection identity before restarting the owned services. Verify real API identity,
production Web runtime URLs, built native renderer and effective PG destination.
The canonical two output journeys exercise genuine local daemon/provider/CLI
processes with deterministic fixture output; they do not establish real-model,
packaged-Main or installer acceptance. Restore the original gate booleans, keep
their epoch monotonic, and stop only the recorded owned services.

### Task archival and explicit allocation identity

1. **Scope:** archiving the full-regression task must not require retaining an
   empty active task directory or prevent later controlled output acceptance.
   `assertRuntimeOutputAllocation(env, checkout, io)` still runs before side
   effects in the output fixture.
2. **Signatures:** `COLLABORATION_OUTPUT_TASK` and the pinned JSON `task` field
   select the same exact relative path. Allowed paths are
   `.trellis/tasks/11-20-full-e2e-regression-20261120` and
   `.trellis/tasks/archive/YYYY-MM/11-20-full-e2e-regression-20261120`, with a
   four-digit year and month 01–12. No search or implicit redirection occurs.
3. **Contract:** the selected canonical directory must exist under the recorded
   checkout. Its regular, nonsymlink `task.json` must contain both `id` and
   `name` equal to `full-e2e-regression-20261120`. Lifecycle status may change;
   it is not the identity. All existing allocation digest, database, private
   state/storage, ownership and service-origin checks still apply.
4. **Validation:** reject foreign task names, malformed archive paths,
   traversal/aliases/symlinks, missing or invalid task metadata and a record
   naming a different path. A stale active-path allocation fails after the
   directory moves. Partial explicit configuration never enters the historical
   branch.
5. **Cases:** an active task with its exact allocation passes; after a normal
   archive move, a new allocation explicitly pinning the real archive path
   passes. Reusing the old path or merely creating an empty directory fails.
6. **Tests:** retain the existing guard matrix and a real temporary-filesystem
   sequence: admit active, move to archive, refuse the stale allocation, then
   explicitly repin and admit the archive. These tests make no database or
   network connection. Actual output journeys remain a separate acceptance gate.
7. **Wrong / correct:** do not rewrite retained reports/allocation records or
   keep a symlink at the active path to manufacture continuity. Preserve old
   evidence; subsequent runs create their own private allocation using the
   archived task path and a newly computed digest. Archive only after current
   verification and owned-resource cleanup are complete.
