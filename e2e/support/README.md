# AI creation end-to-end fixture

`issue-assist-flow.spec.ts` is the canonical AI refinement suite. It exercises the
real web app, Go API, streaming parser, and PostgreSQL. Only the model provider is
deterministic. It does not mock browser API routes. The suite is skipped unless
`E2E_ASSIST_PROVIDER_URL` is set.

1. Start `node e2e/support/assist-provider.mjs` (loopback port 14592).
2. Start an **isolated** backend/database with:
   - `MULTICA_LLM_BASE_URL=http://127.0.0.1:14592/v1`
   - `MULTICA_LLM_API_KEY=e2e-local-provider`
   - `MULTICA_DEVICE_AUTH_ENABLED=false` when testing email login/anonymous access.
3. Start the web app against that backend. Export the same `DATABASE_URL`,
   `NEXT_PUBLIC_API_URL`, and `PLAYWRIGHT_BASE_URL` for Playwright.
4. Run:

```sh
E2E_ASSIST_PROVIDER_URL=http://127.0.0.1:14592 \
  pnpm exec playwright test e2e/issue-assist-flow.spec.ts \
  e2e/task-lifecycle-qa.spec.ts e2e/auth-qa.spec.ts --workers=1
```

Tests create unique users/workspaces and clean their workspace data through the
application API. Run only against a disposable database. The agent case seeds a
runtime without starting a daemon, then checks the real dispatch queue. It does
not execute an installed agent CLI or validate model reasoning quality.

The old supplemental preview/adoption cases are consolidated into this suite:
refinement undo and persistence, stale-edit preservation and cancellation, saving
an unchanged draft after provider failure, and exact refined-prompt dispatch.
`main-supplemental-qa.spec.ts` retains authentication, manual lifecycle and
second-workspace onboarding coverage; it no longer needs a separate AI provider.

The provider recognizes `E2E_ASSIST_ZERO`, `E2E_ASSIST_ONE`,
`E2E_ASSIST_DELAY`, and `E2E_ASSIST_FAIL` in Markdown input. It supports JSON and
streamed OpenAI-compatible responses. `E2E_ASSIST_PROVIDER_PORT` overrides its
port. `/health` and `/requests` support fixture diagnostics using test data only.
