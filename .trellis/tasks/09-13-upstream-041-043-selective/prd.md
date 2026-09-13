# Selective upstream fixes with complete regression verification

## User request and authorization

The user approved nine specific upstream commits, creation of a new worktree, comprehensive business-impact analysis, and a complete end-to-end test run. All code and bookkeeping changes must stay in this worktree and its feature branch; main must remain at its original commit with a clean working directory. This approval covers routine implementation, conflict resolution, isolated test databases/services, and regression fixes needed to finish verification. It does not request a push, a PR, a release, or a merge into main.

## Accepted changes

- 51803b3b3: distinguish transient workspace lookup failures from deleted tasks.
- 6194a8b3a: decline Codex environment reuse when home preparation fails.
- 485278407: preserve valid UTF-8 in tool output previews.
- 9d3613653: classify concurrent request limit failures without prompting reauthentication.
- 71ee15422: dispatch nested list shortcuts at the selected list level.
- 440ef8aa1: do not reject agents merely because their private runtime is not listed.
- b566b5fd4: bound issue-comment notification list previews without losing full content.
- ca47495fc: explain retired Codex compaction route configuration errors.
- a419cf20e: remove the duplicate Inbox sidebar toggle and preserve compact back navigation.

## Acceptance criteria

1. All nine source changes and their meaningful upstream tests are incorporated with source provenance; no unrelated upstream features, schema migrations, or dependency upgrades.
2. Existing local Chinese templates, approval/autonomy rules, intranet integration controls, project defaults, authentication, changelog, and workdir safety remain intact.
3. Regression evidence covers positive, negative, boundary, error, concurrency and compatibility cases relevant to the nine fixes. Additional tests target real behavior gaps, not implementation duplication.
4. Run full TS tests, typecheck, lint, Go tests with a real isolated PostgreSQL database and race detection, relevant static analysis/build checks, and the entire default Playwright E2E suite. Audit conditional skips and run feasible optional coverage separately.
5. Tests must never invoke authenticated real agent CLIs. E2E uses fixture agents and a new isolated application DB; Go tests use a different isolated DB.
6. Document exact results, remaining skips/limitations, impact scope, source-to-local commits and environment. Do not claim that passing tests mathematically guarantee no regression.
