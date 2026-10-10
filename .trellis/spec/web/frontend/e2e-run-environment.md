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

`psql` must be available on PATH. Database creation uses the PostgreSQL endpoint
in `DATABASE_URL`; a Docker container with the same database name is not proof
that this endpoint has the database.

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

Redis enables the real per-IP auth budget (five send-code requests per minute by
default). A full E2E run creates many synthetic accounts from loopback. Configure
`RATE_LIMIT_AUTH` and `RATE_LIMIT_AUTH_VERIFY` for the task-only API's test
throughput; do not change production defaults or weaken login assertions. Go
Redis integration tests require `REDIS_TEST_URL`, using a separate Redis instance
because fixtures flush their assigned logical databases.

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

## Responsive captures and stable task navigation

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

Task links have two valid representations to test separately. Desktop AppLink
emits an absolute shareable URL, while Web can use a relative href; compare the
normalized full URL against the expected origin and task path. After a click,
`IssueDetailRoute` replaces the UUID segment with the API's human-readable task
identifier. Wait for that stable canonical route plus the expected task title,
including the desktop memory-router path. Accepting the transient UUID can
make a routing assertion race the normal canonicalization effect.
