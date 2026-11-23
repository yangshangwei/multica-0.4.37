# Verification

Result: requested stale tests repaired. No product implementation changes.

- Before: three component files had 17 passed / 3 failed; after: 20 passed.
- Views package: 469 files, 5769 tests passed.
- Changed E2E files + canonical AI suite: 22 passed, 0 failed/skipped, one worker and zero retries, real API/PostgreSQL and production Web.
- Non-mobile lint/typecheck: 15 successful tasks. Focused E2E strict TypeScript check passed.
- Production build and scoped diff checks passed.

AI coverage ownership is documented in e2e/support/README.md. All applicable behavior from the removed suite is covered by the canonical AI suite, including the added failure-then-create persistence case. No new product contract was introduced, so no additional domain-spec update was needed.

Full evidence: .gstack/qa-reports/2026-10-01-test-maintenance/README.md.

Known device-auth navigation and squad accessible-name defects remain unchanged. Unrelated welcome/Trellis edits were preserved. No backend code changed or Go rerun required. Work is uncommitted for review; no unrelated files were staged. Task services stopped; dedicated DB/worktree retained for reproduction.
