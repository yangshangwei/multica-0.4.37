# E2E Branch Expansion Implementation Plan

**Goal:** Convert the 180-case baseline into an auditable branch matrix and close high-risk gaps with executed regressions.
**Architecture:** Reuse TestApiClient and isolated production environments. Parallel authors own separate spec files; root serializes browser runs per environment and reconciles exact test identities.
**Tech stack:** Playwright, TypeScript, Go HTTP API, PostgreSQL.

1. Save `playwright test --list --reporter=json` inventory and prior evidence.
2. Audit branch gaps against source and existing canonical unit tests.
3. Add `e2e/password-branches.spec.ts`, `e2e/triage-branches.spec.ts`, `e2e/issue-mutation-branches.spec.ts`, and `e2e/workspace-access-branches.spec.ts`, `e2e/autopilot-lifecycle-branches.spec.ts`, and `e2e/chat-recovery-branches.spec.ts` where source evidence supports distinct cases. Keep setup/cleanup protected.
4. Execute device auth in its enabled environment; run legacy and password tests with their respective config. Commands use `pnpm exec playwright test <specs> --workers=1 --retries=0 --reporter=line,json` with per-run output paths.
5. Diagnose any failure from response/trace and source before fixing. Re-run modified cases and affected existing suites.
6. Run TypeScript compilation of E2E specs, `pnpm typecheck`, `pnpm lint`, relevant unit checks and `git diff --check`. Record unsupported checks explicitly.
7. Generate tracked `docs/qa/e2e-branch-coverage-2026-10-05.md` with scenario mapping, actual outcomes and residual boundaries. Keep raw JSON/logs under `.gstack/qa-reports/2026-10-05-branch-expansion/`.
