# Password reset implementation evidence

## Owned changes

- `packages/views/admin/users/user-action-form.tsx`: target temporary-password confirmation, accessible show/hide control, distinct administrator-password label, reset-specific impact/result/retry text, disabled-account guidance, and recoverable username/password validation feedback. Password values remain uncontrolled and are cleared after requests, including the confirmation; no password is copied into local state or receipts.
- `packages/views/admin/users/user-action-form.test.tsx`: preserves prior role/reason/password regression coverage and adds reset confirmation, accessibility, visibility, uncertain retry, receipt lookup, username collision, disabled-account and Chinese coverage.
- `packages/views/admin/users/user-detail-page.tsx`: keeps reset visible but disabled with role/self/unknown/deployment explanations; self-reset remains blocked even if advertised by a response.
- `packages/views/admin/users/user-detail-page.test.tsx`: preserves return-path/timezone tests and covers reset availability/entry wiring.
- `.omx/reports/admin-resource-publishing/password-locales.json`: bilingual locale fragment, merged by the parent agent into its owned locale files.

The existing endpoint/action identifiers, forced password change, credential revocation, authorization and audit behavior are unchanged. Initial username and reason stay frozen after an uncertain outcome; all three password inputs remain available for re-entry. No dependency, API, global style, generation/copy/delivery or permanent-password mode was added.

## Tests-first evidence

1. `pnpm -C packages/views exec vitest run admin/users/user-action-form.test.tsx admin/users/user-detail-page.test.tsx` before implementation: 12 new expected failures, 6 passes.
2. Same command after implementation: 18/18 passed (2 files).
3. `pnpm -C packages/views exec eslint admin/users/user-action-form.tsx admin/users/user-action-form.test.tsx admin/users/user-detail-page.tsx admin/users/user-detail-page.test.tsx`: passed, exit 0.
4. `git diff --check --` on the four owned source/test files: passed.
5. `pnpm -C packages/views exec vitest run admin/users`: 27/27 passed (3 files), including the existing account-list regressions.

pnpm reports an existing workspace warning that package.json `pnpm.onlyBuiltDependencies` and `pnpm.overrides` are ignored. No dependency configuration was changed.

## Parent integration handoff

- Adapt `e2e/platform-admin-accounts.spec.ts` reset selectors and confirmation fill, reset-specific success, and observer disabled-entry expectation. The detail opener and form submit both say Reset password, so scope submit to its form/section.
- Run integrated typecheck/build after parallel resource changes settle; parent requested avoiding a broad typecheck during concurrent implementation.
- Live browser verification and visual verdict remain for parent integration: both locales/themes, narrow layout, show/hide and forced-change flow. No screenshot comparison was claimed in this implementation slice.
- The server does not expose a detailed disallow-reason field. UI identifies role/self/unknown locally and otherwise describes deployment account restrictions without inventing a specific cause.

## Spec assessment

No new API or architecture contract was introduced. Confirmation is client-only and excluded from mutation bodies; the existing secret-clearing/idempotency rules remain authoritative. No additional shared spec change is required for this bounded slice.
