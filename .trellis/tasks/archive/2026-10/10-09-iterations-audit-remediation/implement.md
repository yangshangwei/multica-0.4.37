# Execution plan

## Approval and boundary

The user has approved task creation and execution of the concrete 19-finding plan. Planning artifacts have been reviewed against current code and baseline suites. No additional product decision is pending.

## Ordered work

- [x] Inspect repository conventions, current source and frozen-operation contracts.
- [x] Run baseline iteration suites: views 12 files/133 tests; core 11 files/120 tests.
- [x] Persist PRD, technical design, source-boundary plan and real context manifests.
- [x] Start this task after manifest validation.
- [x] Obtain current renderer evidence and record the initial visual verdict.
- [x] Lane A: shared semantic text tokens/tab focus, overview/detail primary actions, landmarks, skeletons and touch targets. Own tabs.tsx, tokens.css, iteration-page.tsx, iteration-overview.tsx, their canonical page/navigation tests. Do not change locales.
- [x] Lane B: forms, operation/assignment controls and summaries, required/disabled feedback and recovery identity labels. Own iteration-form.tsx, iteration-operation.tsx, iteration-assignment.tsx, iteration-recovery.tsx and their tests. Do not change locales.
- [x] Lane C: complete filter metadata in server/schema/core, accessible task filters and previous-page navigation. Own iteration_history.go, focused handler tests, iteration-schemas.ts, relevant API/core tests, iterations/index.ts, iteration-issue-list.tsx and its canonical details/list tests. Do not change locales.
- [x] Main: locales, historical comparisons, project embed/paging, settings/loading/disclosures, scope headings/stored-evidence presentation, browser evidence and integrated review. Do not edit lane-owned files while the lane is active.
- [x] Run a Trellis check agent after implementation lanes finish; fix local failures only inside this scope.
- [x] Verify actual interactions, rendered contrast, focus, narrow/coarse layout and request counts; batch any findings and one confirmation pass.
- [x] Publish a fresh impeccable audit with per-finding evidence and explicit verification limits.
- [x] Update scoped specs, commit the verified implementation, archive this task and record session 50.

## Validation

pnpm --filter @multica/core exec vitest run iterations api/iteration-client.test.ts
pnpm --filter @multica/views exec vitest run iterations locales/parity.test.ts
pnpm typecheck
pnpm --filter @multica/core lint
pnpm --filter @multica/ui lint
pnpm --filter @multica/views lint
pnpm check:ui-exports
Scoped Go tests for the changed history response, plus go vet/static checks when applicable.
Fresh production-Web or actual Desktop-renderer browser checks in an isolated profile; record the mode, commit, source fingerprint and request/DOM evidence. Never treat a dev-mode Web run as production E2E.

git diff --check and a task-delta ownership review before final reporting. Do not commit pre-existing working-tree work. If pre-existing untracked source cannot be isolated into an independent commit, keep the task changes reviewable and report the commit limitation.

## Closeout

The user authorized browser verification followed by commit and archive. The selected 66-file implementation was verified in an isolated checkout and committed as `e93a758fe3ec7bd9acdbf7ec891fb940fa9255a0`. Necessary iteration page/progress/creation-context dependencies are included; unrelated desktop-core, Gantt, rollout and release work remains outside the commit.

Four production Electron E2E tests, 457 unit/component tests, nine workspace typechecks, scoped Go tests/vet, package lint, UI exports and source fingerprints passed. Nineteen screenshots include all four disable-preview theme/viewport cases; the independent visual verdict is 95/100. The task is archived and session 50 records the evidence and coverage limits. No task-specific work remains.
