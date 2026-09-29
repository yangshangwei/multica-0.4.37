# Implementation plan

User-approved option 1 is the implementation authority. Apply writing-plans and verification-before-completion within this task.

## Cleanup plan and ownership
1. Add failing separation/entry regressions to `packages/views/skills/components/skill-library-catalog.test.tsx`; existing creation/session tests protect copy behavior.
2. Remove the shelf and add the workspace catalog action in `skill-library-catalog.tsx`; update shared page header, template card wording and English/Chinese skill locale files.
3. Remove shelf-only state in `packages/core/skills/stores/view-store.ts`; adapt persistence tests and affected test setup, preserving other preferences.
4. Update `e2e/skill-market.spec.ts` and `e2e/skill-template-creation.spec.ts` to the new IA.
   Resume audit: update the same retired heading count/name expectations in `e2e/localized-template-defaults.spec.ts` and `e2e/workspace-defaults.spec.ts`; verify the existing bilingual/narrow-screen entry and empty-workspace scenarios.
5. Run skill/store/locale tests, lint/typecheck and both browser scenarios. Capture/review wide/narrow screenshots and fix scoped findings.
6. Update `.trellis/spec/views/frontend/skill-market-discovery.md`, its index and user skill docs. Record evidence, commit only task-owned files with Lore trailers, and complete/archive the task.

Implementation subagent owns skill UI/core state, skill locale files and related tests. Leader owns task artifacts, spec/docs, browser environment and final verification. Preserve others' edits.

## Commands
- `pnpm --filter @multica/views exec vitest run skills locales/parity.test.ts`
- `pnpm --filter @multica/core exec vitest run skills/stores/view-store.test.ts`
- `pnpm lint`
- `pnpm typecheck`
- `pnpm exec playwright test e2e/skill-market.spec.ts e2e/skill-template-creation.spec.ts --workers=1`
- `git diff --check`

## Progress
- [x] Inspect behavior and coverage; record approved scope.
- [x] Implementation and regression coverage verified on resume; original red-first history was not recreated.
- [x] Shared checks and browser/visual verification.
- [x] Spec/docs sync, scoped implementation commit and completion evidence.
