# Workspace name series implementation plan

**Goal:** Let users choose memorable, development-oriented random workspace names.

**Architecture:** Pure catalog/generator and persisted account preferences live in
core. A shared views picker wires into the existing onboarding workspace step.

**Tech stack:** TypeScript, Zustand, React, Base UI, i18next, Vitest, Playwright.

## Execution

- [x] 1. Core lane: add failing tests for catalog/generator and preference behavior
  in `packages/core/workspace/workspace-names.test.ts` and
  `workspace-name-preferences.test.ts`; run to establish the failure.
- [x] 2. Core lane: implement `workspace-names.ts` and
  `workspace-name-preferences.ts`; export subpaths from `packages/core/package.json`.
  Validate focused tests before handing off the contract.
- [x] 3. Views lane: add failing tests for protected URL/prefix and series menu
  behavior; implement `packages/views/workspace/workspace-name-picker.tsx`, wire
  `packages/views/onboarding/steps/step-workspace.tsx`, add en/zh-Hans workspace copy.
- [x] 4. Views lane: remove `celestial-workspace-names.ts` and celestial-only
  exports/tests from `slug.ts`/`slug.test.ts`; preserve other slug behavior.
- [x] 5. Integrate and run affected tests, locale parity, core/views lint and
  typecheck; check consuming web/desktop types and static diff analysis.
- [x] 6. Browser inspection: desktop and narrow layout, Chinese and English,
  real menu selection/keyboard interaction and random field behavior. Persist
  visual verdict in `.omx/state/workspace-name-series/ralph-progress.json`.
- [x] 7. Independent review; fix concrete findings, update relevant Trellis spec,
  write verification evidence, and commit only this task's changes.

## Validation commands

```sh
pnpm --filter @multica/core exec vitest run workspace/workspace-names.test.ts workspace/workspace-name-preferences.test.ts
pnpm --filter @multica/views exec vitest run workspace/slug.test.ts workspace/workspace-name-picker.test.tsx onboarding/steps/step-workspace.test.tsx locales/parity.test.ts
pnpm --filter @multica/core --filter @multica/views typecheck
pnpm --filter @multica/core --filter @multica/views lint
pnpm --filter @multica/web --filter @multica/desktop typecheck
git diff --check
```

Expected: all focused behavior tests and required static checks exit zero. Record
any unrelated baseline failures with exact evidence rather than changing their
files. Preserve unrelated task docs and `step-welcome.tsx` edits present at start.

## Rollback / review gates

Keep core and views changes bounded. If contract changes are needed, update this
task's design before integration. No deployment or remote push is required.
