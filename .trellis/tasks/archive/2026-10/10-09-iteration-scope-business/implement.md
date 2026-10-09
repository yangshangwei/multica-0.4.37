# Implementation and verification plan

## Authorization and convergence

The user approved the preceding business review and explicitly requested task creation, requirements, design, development and verification in one instruction. The main session resolved technical choices through the two research reports, reviewed this PRD/design, and continues under that authorization. No unresolved product decision or scope expansion blocks implementation.

## Steps

- [x] Create task and capture requirements, prior screenshot and the user's full execution authorization.
- [x] Research domain/statistics contracts and shared UI/verification boundaries independently.
- [x] Establish passing baseline: core iterations 144 tests; existing events/navigation/history view suites 65 tests.
- [x] Converge PRD and design; populate real implementation/check context manifests.
- [x] Validate manifests, set task `in_progress`, then dispatch bounded Trellis implementation agents.
- [x] Core: add meaningful failing regression tests, implement phase/impact/metric selection helpers, export the agreed interface, run the core iteration suite and core lint/typecheck.
- [x] UI: integrate lifecycle metadata, planned/empty-plan presentation, inline metric/task evidence, category/search/refinements, links and audit wording. Keep main Tasks filters and authorization behavior. Update canonical component tests and both locales.
- [x] Add real-API production browser scenarios for planned/active/frozen stages, count drilldown, filtering/keyboard, empty-plan history, current navigation and responsive wrapping. Use task-owned fixture resources only.
- [x] Run independent `trellis-check` review with local safe fixes; review edge cases against the canonical Go semantics and existing contracts.
- [x] Run final frontend lint/typecheck, all iteration tests, locale parity, relevant platform regressions and static analysis appropriate to the change.
- [x] Build production Web and Desktop using existing dependencies, then run focused Web/native flows in the task-owned environment with provenance.
- [x] Inspect real screenshots in one batched wide/narrow round, save `visual-verdict` and use any findings before a bounded fix/confirmation round. Run the Impeccable detector over changed UI once it is complete.
- [x] Update executable iteration specs and acceptance evidence; commit only the 17 owned files using the repository protocol (`6d3bf906d`).
- [x] Archive the task and record session progress (artisan journal session 51).

## Checks and evidence

Initial baseline is in `evidence/baseline-core.log`, `evidence/baseline-views.log`; the supplied reference is `evidence/reference-planned.png`.

Focused iteration commands (run package commands through installed pnpm; no dependency changes):

```sh
pnpm --filter @multica/core exec vitest run iterations --maxWorkers=2
pnpm --filter @multica/views exec vitest run iterations locales/parity.test.ts --maxWorkers=2
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/core lint
pnpm --filter @multica/views lint
pnpm typecheck
pnpm lint
git diff --check
```

Use the root scripts as authority; run static analysis with a changed-file focus or compare against the existing baseline so unrelated retained work is not rewritten. Backend business rules remain read-only; their canonical tests guide the new frontend matrix. Broaden checks only when changed files or failures justify it.

Production browser plan follows `.trellis/spec/web/frontend/e2e-run-environment.md`. `make status` initially found all checkout services stopped. Prepare a task-only environment/config with appropriate test auth, then use:

```sh
make up C=api,web ARGS="--web-mode production"
make status ARGS="--json"
make env-exec ARGS="-- env MULTICA_RUN_I1_E2E=1 pnpm exec playwright test e2e/iteration-scope-business.spec.ts --project=chromium --workers=1 --retries=0"
```

Adapt only to verified launcher options; record effective env, API/Web commit/source/build identity and commands without credentials. Existing fixture `p1Session` creates synthetic accounts/workspaces; always delete the synthetic feature workspace in `finally`. One production build per checkout; no dev-server evidence. Do not stop an unknown listener or alter the user's manual iteration data.

Keep exact test identities and actual run/pass/fail/skip counts. Capture both locales, wide/narrow layouts, selected metric, zero matches, technical audit and frozen/current separation through `TestInfo` paths. Native parity requires a real Electron build/run; shared-source reuse alone is not a native verification claim.

## Acceptance matrix

| Acceptance | Canonical evidence |
| --- | --- |
| AC-01/02 phases and empty plan | Core phase tests, mounted page/panel tests, real planned and cancelled/start-empty fixtures |
| AC-03 impact/count semantics | Core table-driven stored-fact tests; integration wiring only in view tests |
| AC-04 exact details | Core selectors and duplicates/missing-facts tests; UI lazy current/original revision/count checks; real add/remove/reenter scenario |
| AC-05 history/current/access | Existing guard regressions plus selected detail eviction; frozen fixture after live rename/status update; platform links |
| AC-06 filtering/state | Mounted interaction tests and real category/search/reset flow preserving Task filters and summary |
| AC-07 copy/audit | Locale parity, closed/open technical disclosure checks and screenshots |
| AC-08 quality | Command logs, independent review, production browser/native results and visual verdict |

## Risks and rollback points

- Avoid interpreting `cancelled → done` as a reopen count, or `done → todo` as scope growth.
- Do not use event timestamps, initial scope count, live issue titles or closure release leaves to reconstruct historical metrics.
- Complete activity and detail reads have no common event revision. Label activity evidence accurately; only matching revision-bound task collections can claim current membership equality.
- Preserve the caller's `scope_revision`, main task-list state, disclosure identity and protected-query eviction while adding detail queries.
- Pre-existing modification: `apps/web/next-env.d.ts` has production route-types import. Preserve its exact incoming contents after any build.
- Before each commit, re-check worktree ownership and stage only this task's files; do not include existing ahead-of-origin history or unrelated work.
