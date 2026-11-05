# Implementation and verification plan

## Sequence
- [x] Load project and Impeccable guidance; create approved task and inventory scope.
- [x] Complete independent design and technical assessments; record reviewed findings and page matrix.
- [x] Capture baseline behavior and visuals before product edits.
- [x] Activate task and implement scoped repairs with meaningful behavioral regression coverage.
- [x] Repair workbench discovery/hierarchy, action feedback, auxiliary recovery, and verified cross-surface accessibility defects in separate owned lanes.
- [x] Run affected collaboration/admin suites, locale parity, Views lint/typecheck and platform type checks; inspect all results.
- [x] Run one batched wide/narrow, light/dark, English/Chinese browser inspection; fix all observed defects together, then confirm.
- [x] Independent final source/spec review, static detector on final changed UI, evidence-backed completion report and spec notes.

## Commands
- pnpm --filter @multica/views exec vitest run collaboration admin/collaboration locales/parity.test.ts --maxWorkers=2
- pnpm --filter @multica/views lint
- pnpm --filter @multica/views typecheck
- pnpm --filter @multica/web typecheck
- pnpm --filter @multica/desktop typecheck
- git diff --check
- Impeccable final detect --json on changed UI targets.

## Validation boundary
Browser evidence uses real current-checkout UI and test-owned API accounts/data where available. Targeted fault injection is explicitly marked. No real AI provider invocation is needed for visual/interaction repairs. Do not count shared browser checks as installed Electron Main, physical-device or spoken screen-reader acceptance.

## Ownership and risks
Expected product edits are confined to packages/views/collaboration, packages/views/admin/collaboration, their tests and en/zh-Hans collaboration/admin locales. Existing source/knowledge entry components are reviewed and changed only for concrete defects. Parent owns shared locale edits and final integration unless explicitly delegated. Preserve recovery/action authority while changing presentation.
