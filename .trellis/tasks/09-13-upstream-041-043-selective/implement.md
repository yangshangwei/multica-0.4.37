# Execution and verification

- [x] Confirm main is clean and preserve its exact SHA; create independent worktree/branch.
- [x] Record approved scope and layer guidance; audit test/environment prerequisites.
- [ ] Install only frozen existing dependencies in the new worktree.
- [ ] Run narrow existing baseline tests and demonstrate key new regressions fail against old behavior where feasible.
- [ ] Port all nine requested commits with atomic source provenance and inspect the combined diff.
- [ ] Add behavior-focused missing regressions: Inbox nav at desktop/compact widths, preview identity/full response, and browser journeys.
- [ ] Run targeted TS and Go regressions, including handler tests against the isolated DB; inspect run counts to reject silently skipped suites.
- [ ] Run complete TS tests/typecheck/lint, Go race suite and vet, builds/static boundary checks; record every failure and resolution.
- [ ] Run the complete Playwright suite against this worktree's API/web and isolated application DB, then investigate all failures and feasible conditional suites.
- [ ] Perform independent change/coverage review, verify main unchanged, update impact/results/provenance records, and leave work ready on the feature branch without merging main.

Expected checks use repository scripts, bounded concurrency, fresh logs and exact exit codes. Do not hide failures by disabling tests or weakening assertions. Baseline failures must be reproduced/classified and fixed in scope when appropriate. Full E2E is mandatory, not replaced by unit tests or a small smoke subset. Re-run only checks affected by subsequent fixes unless unresolved failures require broader verification.
