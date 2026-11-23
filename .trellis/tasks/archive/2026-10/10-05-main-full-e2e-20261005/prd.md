# E2E branch coverage expansion

## Goal
Extend the existing 180-case baseline with explicit high-risk branch coverage and execute the resulting suites against isolated local production builds.

## Requirements
- Preserve existing changes and test assertions. No new dependencies.
- Cover authentication validation/recovery, triage invalid/stale/repeated operations, issue mutation failure/recovery and cross-workspace access.
- Distinguish browser + real API, browser fault injection, API integration, native fixture, and canonical unit coverage.
- Keep every existing and new test identifiable; skipped or missing executions never count as passes.
- Use synthetic data and TestApiClient cleanup. No external paid model execution.

## Acceptance criteria
- [x] Inventory current cases and prior unresolved results.
- [x] Write a traceable branch matrix including exclusions and missing coverage.
- [x] Implement independently named regressions for uncovered high-risk branches.
- [x] Execute added tests and relevant existing suites with zero retries; investigate failures.
- [x] Run static checks and summarize fresh evidence without claiming 100% code-branch coverage.
