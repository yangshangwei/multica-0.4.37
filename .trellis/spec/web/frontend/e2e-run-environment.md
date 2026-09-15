# E2E Run Environment

`playwright.config.ts` starts no servers — "they must be running already". `baseURL`
resolves from `PLAYWRIGHT_BASE_URL`, then `FRONTEND_ORIGIN`, then
`http://localhost:3000`. Whatever listens on that port decides the suite's verdict,
so the web server's build mode is part of the test contract, not an operator preference.

## Rule

Run the Playwright suite against a production web server:

```bash
pnpm --filter @multica/web build     # next build --webpack
node node_modules/next/dist/bin/next start --port <port>   # apps/web "start"
pnpm exec playwright test
```

Never point the suite at `pnpm dev:web` or `next dev --webpack` (the `apps/web`
`dev` script). A dev server compiles routes lazily on first request, so the suite
measures webpack, not the product.

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
