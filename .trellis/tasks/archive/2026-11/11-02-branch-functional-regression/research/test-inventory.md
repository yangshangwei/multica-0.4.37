# Test inventory and execution contracts

Actual collection: `pnpm exec playwright test --list --reporter=line`, exit 0. Current inventory is 439 cases in 148 spec files, project Chromium. No business test executed during collection. Raw output and command/time/SHA are retained beside this file.

`playwright.config.ts` starts no servers, has one worker, zero retries and a 60-second default case budget. The base URL chooses PLAYWRIGHT_BASE_URL, FRONTEND_ORIGIN, then localhost:3000. Explicit verified task configuration is required; default URL availability is insufficient.

Root `pnpm typecheck/lint/test` uses Turborepo and excludes `@multica/mobile`. Core, views, web, desktop and docs have Vitest test scripts. Mobile separately defines typecheck, expo lint, and `vitest run && bash scripts/ios-run.test.sh`. Do not append Vitest flags to the chained script without inspecting forwarding.

`scripts/check.sh` orders static checks, two-worker package-serialized TS tests, script regressions, Go race/vet, isolated API, production Web, Playwright. It creates separate Go/API DBs but shares one Go DB across its package sequence. Server database-test-isolation spec requires a fresh owned database per DB-backed package for joint acceptance.

`scripts/test-go.sh --race` uses timeout=40m, regular package concurrency 1 and test parallelism 2; agent package concurrency 2 and parallelism 2. `scripts/go-test-with-agent-cli-guard.sh` catches ambient user-installed CLI execution. This guard must remain active for test commands that can discover Provider processes.

Conditional E2E gates include legacy, password, device, synthetic platform administrator, explicit loopback API/database, deployment identity, renderer URL, namespace images, installed artifacts, resource-publishing directories and live changelog feed. See `e2e/fixtures/lane-guards.ts`. Singleton gate tests cannot run concurrently against the same database. Setup-skipped tests are not covered merely because the default suite exits successfully.

Core feature/recent-change entry points and required observable behavior are mapped in `test-plan.md`; exact case identities are in `coverage.csv`. Current statuses are NOT_RUN until raw run results justify an update.
