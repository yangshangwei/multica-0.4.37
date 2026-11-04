# E2E Run Environment

`playwright.config.ts` starts no servers — "they must be running already". `baseURL`
resolves from `PLAYWRIGHT_BASE_URL`, then `FRONTEND_ORIGIN`, then
`http://localhost:3000`. Whatever listens on that port decides the suite's verdict,
so the web server's build mode is part of the test contract, not an operator preference.

## Rule

Run the Playwright suite against a production web server:

```bash
make check                         # isolated API/Go databases; builds production Web

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
tests, isolated Go race tests, API startup, Web build/start, then Playwright in
sequence. The Go database is never served by the API and is dropped without
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

### Optional native renderer origins in `make check`

1. **Scope:** an explicitly selected, separately built Electron renderer used
   alongside the allocated production Web/API verification environment.
2. **Signatures:** `prepare_check_environment(source_env)` derives the task-only
   `CORS_ALLOWED_ORIGINS` and `ALLOWED_ORIGINS` from its allocated Web origin plus
   the origins of optional `COLLABORATION_ELECTRON_RENDERER_URL` and
   `CHANGELOG_ELECTRON_RENDERER_URL` values loaded before preparation.
3. **Contract:** renderer URLs must use HTTP(S) on `localhost`, `127.0.0.1` or
   `[::1]`, with no userinfo credentials and no port zero. Their paths do not
   enter the allowlist. Deduplicate origins, keep the source env byte-identical,
   and never carry arbitrary production CORS values or wildcard origins into
   the test copy. The default without either renderer remains Web-only. These
   values must exist before API configuration identity/provenance is computed;
   adding a renderer only to a later Playwright child cannot configure the API.
4. **Errors:** invalid, non-loopback, credentialed or unsupported-scheme input
   stops preparation before services start, without echoing the rejected URL.
   Server-side WebSocket checks still require exact allowed-origin membership;
   Electron's disabled browser `webSecurity` is not a server-side exemption.
5. **Cases:** an explicit second loopback renderer adds one origin; duplicate
   renderer/Web origins add none; an unconfigured run retains its original
   Web-only behavior. A remote renderer is refused even if source CORS allowed it.
6. **Verification:** `scripts/check.test.sh` exercises the real preparation
   function with fake allocation/services, both independent renderer variables,
   shared origins, IPv6, source preservation and nonsecret rejection. These are
   configuration contracts, not a native browser or WebSocket execution pass.
7. **Wrong / correct:** do not widen production server CORS or secretly mutate
   only a child server's environment after its identity was recorded. Set the
   explicit task-owned renderer URLs before invoking `make check`; both task
   allowlists then match the actual environment used to start its API.

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

Collaboration actions move between the primary controls and More actions as
responsibility and status change. In `uiWorkflow`, locate the exact requested
button and, if it is not visible, call the existing
`revealCollaborationDisclosure(page, "More actions")` before clicking. Apply
this to both inviting and transferring: Transfer can be primary for the creator
but collapsed for the recipient after a reload. Expand the section before
asserting that an unauthorized action is absent; keep capability/API refusals.

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
