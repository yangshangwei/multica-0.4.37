# Implementation
1. Frontend executor: update assist tests first, then component and en/zh copy; update create and quick-create wiring tests and real editor tests. Run targeted Vitest, ESLint and typecheck.
2. Backend executor: refine prompt for concise executable tasks, max 2 critical questions and no invented requirements; add focused tests, run Go targeted tests and vet.
3. Root: integrate, review race/undo guards, update scoped spec, verify and record results. Preserve unrelated dirty files. No publish.

## Verification
- 100 tests passed across assist, real editor, manual create and agent create suites.
- Scoped ESLint and views TypeScript check passed.
- Go OptimizeIssueDescription handler tests and go vet ./internal/handler passed.
- Isolated Chromium real-editor harness: auto-fill, inline/delegated answers, merge, undo preserving answers, re-merge without duplication; desktop/390px, no horizontal overflow or page errors.
- git diff --check passed.
- Existing local app environment reported ownership mismatches, so browser verification used an isolated component harness with mocked AI output, not a full deployed application. No real-model quality evaluation or deployment performed.
- Existing unrelated onboarding/style changes preserved. Work left uncommitted.
