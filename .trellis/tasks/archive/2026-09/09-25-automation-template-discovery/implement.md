# Implementation

1. Update page/gallery behavior tests and run the expected failing cases before implementation.
2. Build the shared responsive gallery, scenario presentation and output hints; wire both entry points and preserve non-default filter context.
3. Update all four locales and final submission wording.
4. Run focused tests, package typecheck/lint and static checks; inspect the actual UI in a browser and record the visual verdict.
5. Review the complete task diff, document verification and preserve unrelated edits.

No new dependencies. The accepted proposal and prior mockup under `.omx/artifacts/` provide the visual reference.


## Resume verification, 2026-09-25

- Recovered the prior implementation and replaced the PRD/context placeholders.
- Added gallery view-state restoration and origin-aware Back navigation. Native
  and in-flow Back retain the latest filter and offset, including Desktop where
  plain scroll entries are replaced by the configuration step.
- Corrected English/Japanese/Korean run-only descriptions and removed an E2E
  catalog-loading race.
- Relevant Vitest suite: 18 files, 666 tests passed with two workers.
- Full views typecheck passed; full views lint passed with 26 pre-existing
  warnings outside this task. Shared routing boundary and diff checks passed.
- Browser component harness passed 1440/768/360 layouts, English/Chinese,
  light/dark, keyboard selection, filters and restored scrolling; no page errors.
- Production application E2E: all 9 passed in 52.9 seconds on build
  `MARNMZBvNnkAoughBFaR6`; the task-owned web process was stopped afterwards.
- The browser regression exposed render-time Web restoration using the mutable
  browser URL. The platform now keys its adapter by Next's rendered pathname;
  all 5 platform restoration tests pass. Total relevant unit tests: 671.
- Full production Web build, including TypeScript, passed. The task-owned copy
  lives at `.omx/artifacts/automation-ui-qa/production/` and proxies browser API
  requests through Web. Discarded development-server/CORS runs are retained as
  diagnostics; the final production run is authoritative.
- Waited for the separate squads commit to finish, then committed exactly the
  19 automation/restoration files as `3083d6e09`, preserving task boundaries.
