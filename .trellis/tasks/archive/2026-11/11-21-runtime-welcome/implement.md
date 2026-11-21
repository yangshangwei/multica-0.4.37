# Execution plan

The user's latest request explicitly authorizes planning followed by development. No unresolved product decision remains: use the supplied reference and preserve existing runtime management semantics.

1. [x] Inspect page, nearest welcome, connection/help constraints, tokens and localization.
2. [x] Record PRD/design, boundary and curated manifests; review against the user brief.
3. [x] Activate task; dispatch Trellis implementer for shared component, page, locales and colocated tests only.
4. [x] Add gate/action/carousel regressions, confirm failure, implement targeted change, run focused checks.
5. [x] Main session writes browser acceptance and verifies isolated production API/Web origins.
6. [x] Batch desktop/narrow, English/Chinese, light/dark screenshots; verify real dialog/help, reduced motion and transition to runtime inventory.
7. [x] Run Impeccable detector once; persist visual verdict; fix the carousel endpoint focus issue and confirm the corrected production render.
8. [x] Run the Trellis check and independent Impeccable finish review. The reviewer confirmed both carousel arrows retain focus at their endpoints.
9. [x] Record the inventory and endpoint focus contracts, commit the verified task files, and archive the delivery record. The developer journal links to the feature commit.

See `verification.md` for the final test results and artifact paths.

## Validation

- `pnpm --filter @multica/views exec vitest run runtimes/components/runtime-welcome.test.tsx runtimes/components/runtimes-page.test.tsx`
- Existing runtime/machine/profile/docs/connection regressions.
- Affected-file ESLint and `pnpm --filter @multica/views typecheck`; consumer checks as needed.
- Production build; `pnpm exec playwright test e2e/runtime-welcome.spec.ts` against verified isolated origins.
- `/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json <changed UI paths>` once after build.

## Safeguards

Preserve unrelated dirty work. Do not stop mismatched processes or destroy existing environments. Record pre-existing/environmental check failures and use alternative verification when possible. No dependencies.
