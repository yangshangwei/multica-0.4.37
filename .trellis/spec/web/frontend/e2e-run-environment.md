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
It copies the selected env file, sets classic auth only in that task copy
(`MULTICA_DEVICE_AUTH_ENABLED=false`), and runs static checks, bounded TypeScript
tests, isolated Go race tests, API startup, Web build/start, then Playwright in
sequence. The Go database is never served by the API and is dropped without
forcing connections closed. API data and the environment record are retained
for inspection until TTL collection. Only processes started by this run are
stopped; an early failure or cleanup failure produces a nonzero exit.

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
