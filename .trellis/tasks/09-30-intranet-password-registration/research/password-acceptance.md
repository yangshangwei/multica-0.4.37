# Password-mode browser acceptance

Date: 2026-10-01 (Asia/Shanghai).

## Scope and isolation

- Local password API: `localhost:18091`, built from this checkout with `go build -o /tmp/multica-password-acceptance-server ./cmd/server`.
- Database: this worktree's isolated `.env.worktree` database; no production database, mail delivery or real agent invocation.
- Web: copied `apps/web` source under `/tmp/multica-password-web.ambZ8L/apps/web`, excluding `.next`, with dependencies/shared packages linked to this checkout. Its own `next dev --webpack --port 13091` avoids writing the `.next` used by the leader's concurrent `make check`.
- API and browser environment: `MULTICA_AUTH_MODE=password`, `MULTICA_PASSWORD_LIMITER_MODE=single`, API `http://localhost:18091`, browser `http://localhost:13091`.
- Migration window for this run: cutoff `2026-09-30T18:00:00Z`, deadline `2026-10-10T00:00:00Z`. Future reruns must choose a cutoff before the current time and a future deadline. Legacy fixture tokens are signed with the isolated deployment's existing secret; no secret values are recorded.

## Added coverage

`e2e/password-migration.spec.ts` adds two browser paths, using `TestApiClient` for API fixtures/teardown and test-scoped database updates for migration setup:

1. A legacy JWT lacking `auth_version` loads the setup form, binds credentials to the same UUID, reaches its existing workspace and survives reload. Existing server integration tests remain the canonical ownership/data-preservation matrix.
2. The real `password-recover --user <fixture UUID>` command receives a distinct temporary password over stdin. Browser login reaches the forced-change form; reload retains the restriction; changing the password returns to the same UUID/workspace and survives reload.

Both record sanitized request-order attachments containing only timestamps, methods, URL paths and status codes, never request bodies, headers, cookies or passwords. Teardown removes only each test's workspace/account and repairs its credential state if a test fails midway.

## Findings and regression evidence

The first combined browser run passed legacy binding and registration but failed recovery after a successful password change. A second recovery-only run reproduced the failure. Sanitized evidence is `/tmp/password-recovery-request-order.json`:

- `POST /api/me/password/change` returned 200.
- Two `GET /api/workspaces` requests returned 200 within 26 ms.
- Browser navigated back to `/login` and remained there; there was no 401 after password change.

The leader identified and removed the account gate's stale completion redirect: changing the auth state unmounted the form and restored the authenticated route tree, but the old async form completion subsequently replaced its navigation with `/login`.

The browser also exposed denied `/api/client-usage` reports during restricted states; the leader's other lane added the authenticated-state guard.

A run immediately after hot-reloading these changes hit a stale dev configuration state (email login instead of password login, with no browser config request). It was stopped, evidence retained at `/tmp/password-acceptance-final-results`, and the copied dev server restarted before final verification.

## Commands

After loading `.env.worktree` without printing its contents and exporting the isolated values above:

```sh
E2E_PASSWORD_AUTH=1 \
E2E_PASSWORD_SERVER_BINARY=/tmp/multica-password-acceptance-server \
pnpm exec playwright test \
  e2e/password-registration.spec.ts \
  e2e/password-migration.spec.ts \
  e2e/password-server-switch.spec.ts \
  --reporter=json --output=/tmp/password-acceptance-verified-results

pnpm exec tsc --noEmit --target ES2022 --module ESNext \
  --moduleResolution bundler --skipLibCheck --esModuleInterop --types node \
  e2e/password-migration.spec.ts e2e/password-registration.spec.ts
```

Standalone TypeScript check passed. The root has no ESLint config for direct root `eslint e2e/...`; lint was not claimed from that command. The leader owns workspace lint/typecheck and the full pipeline.

## Final result

Final unchanged combined run: **4 passed**, 32.3 seconds, zero skips, failures or flaky retries. JSON evidence: `/tmp/multica-password-acceptance-verified.json`; screenshots/attachments: `/tmp/password-acceptance-verified-results`.

| Browser path | Result |
| --- | --- |
| Legacy JWT setup → original workspace → reload | Passed, 7.7 s |
| Actual temporary recovery → forced change → original workspace → reload | Passed, 7.8 s |
| Registration → original onboarding → new workspace → second browser identity | Passed, 12.7 s |
| Real Electron same-host server switch isolation | Passed, 1.7 s |

The restarted server's first cold run had a legacy navigation timeout during a 20.8-second route compilation; the same test passed with warmed compilation without modifying assertions or adding retries. Recovery, registration and Electron also passed in that cold run. This acceptance used a development Web server; the leader's separate pipeline owns production-build verification.

Both final migration request-order attachments contain **zero 403 responses**, confirming the restricted-state telemetry guard. Final standalone TypeScript check passed again.

Own API and copied Web server were stopped; `lsof` confirmed no listeners on 18091/13091. The copied Web directory was removed. Normal teardown plus explicit cleanup of one interrupted-run fixture left **zero migration acceptance accounts** in the isolated database. Logs, sanitized evidence and the temporary API binary remain under `/tmp` for review.

