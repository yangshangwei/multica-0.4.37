# Missing-user session recovery verification

Code commit: `fda7bec73334529513bd6cf290b9447a22db57f5`. Completed 2026-09-30 in the isolated MCP worktree.

## Verified behavior

- Missing database user returns 401; lookup failure returns 500; neither response changes cookies.
- Only legacy GET /api/me + 404 + exact user-not-found response is normalized. Unrelated errors retain their status and session.
- New bearer/cookie logins survive stale identity and workspace rejection, including login while the old error body is still being read.
- Credential generation is distinct from duplicate-401 deduplication; real React StrictMode device login still completes once.

## Fresh results

- Core API/auth/initializer: 5 files, 169 tests passed. Seven new initializer regressions were observed failing before their corrections.
- Go GetMe handler: 2 tests passed against isolated database multica_multica_mcp_market_800; no skips.
- Playwright missing-user-session: 1 passed (28.6 seconds), real Web/API/database, stale identity redirects to login and fresh login reaches MCP market.
- Core typecheck, scoped ESLint, handler/server go vet and git diff --check passed.
- Read-only review after the StrictMode correction found no remaining blocker.

Commands used:

```sh
pnpm --filter @multica/core exec vitest run platform/auth-initializer.test.tsx api/missing-user-session.test.ts api/client.test.ts auth/store.test.ts auth/utils.test.ts --maxWorkers=2
pnpm --filter @multica/core typecheck
pnpm --filter @multica/core exec eslint api/client.ts api/missing-user-session.test.ts platform/auth-initializer.tsx platform/auth-initializer.test.tsx
bash scripts/dev-env.sh exec mcp-market-20260929 -- sh -c 'cd server && go test ./internal/handler -run "TestGetMe" -count=1 -v'
bash scripts/dev-env.sh exec mcp-market-20260929 -- env PLAYWRIGHT_BASE_URL=http://localhost:13800 NEXT_PUBLIC_API_URL=http://localhost:18880 MULTICA_DEV_VERIFICATION_CODE=888888 pnpm exec playwright test e2e/missing-user-session.spec.ts --workers=1
```

Browser test services used local development email delivery, no external mail service, and were stopped afterwards. No production deployment, native Electron smoke run or remote push.
