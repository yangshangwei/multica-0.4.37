# Execution plan

- [x] Inspect prior failures and current code; record the user-authorized scope.
- [x] Prepare an isolated production Web and development API and reproduce all six identities before repairs.
- [x] Diagnose and minimally fix auth, compact recommendations and inbox selectors in disjoint scopes.
- [x] Rebuild production Web; run all six identities and related auth/quick-create/inbox cases with zero retries.
- [x] Inspect English/Chinese compact and desktop screenshots; persist visual verdict JSON.
- [x] Run relevant unit tests, lint/typecheck and static diff checks; obtain independent Trellis review.
- [ ] Update applicable specs, preserve evidence, commit only task files with Lore trailers, archive and journal.

## Ownership and evidence

Primary agent owns environment setup, baseline/final integration runs, task artifacts, specs, commits and cleanup. Trellis implement agents own bounded auth, recommendation and inbox slices. Preserve concurrent edits; no child may rebuild or stop shared services.

Follow `.trellis/spec/web/frontend/e2e-run-environment.md`. Save configured commands (without secrets), JSON results, screenshots, provenance and initial dirty-state snapshots under `.omx/reports/qa-three-regressions-20261017/`. Existing E2E assertions remain the acceptance floor. No broad cleanup is planned.
