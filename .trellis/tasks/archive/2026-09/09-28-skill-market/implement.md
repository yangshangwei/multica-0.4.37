# Skill Market Implementation Plan

**Goal:** Make deployment templates visible and usable directly from the skill library.
**Architecture:** Shared views catalog wrapper + existing core workspace preferences + unchanged template create/session/query contracts.
**Tech Stack:** React, TypeScript, Zustand, TanStack Query, Base UI, Vitest, Playwright.

## Execution
1. [x] Core lane: extend `packages/core/skills/stores/view-store.ts` and exports; write preference/reset tests first, run failing test, implement and verify.
2. [x] Catalog lane: implement `packages/views/skills/components/skill-library-catalog.tsx` and reusable template card with EN/ZH locales and focused RTL tests. Reuse current discovery helpers, icons and queries. Test empty/error distinctions, filter composition, copy links and accessibility.
3. [x] Integration lane: update `skills-page.tsx`, remove obsolete compact `skill-template-entry.tsx`, adapt real-page template-session tests and page tests. Keep root dialog lifetime; test market creation and retained draft/focus.
4. [x] Run focused suites, changed-package lint/typecheck, and Web/Desktop typecheck. Run static diff/boundary checks and Impeccable detector once on finished UI.
5. [x] Inspect browser at wide and narrow widths using local app or a request-mocked local app route; record screenshots and visual-verdict. One correction batch, one confirmation pass.
6. [x] Independent Trellis check agent reviews complete task against manifests, fixes bounded findings; rerun relevant checks.
7. [x] Update skill discovery spec, verification report and task checklist. Commit only owned files/hunks using Lore protocol; preserve unrelated dirty work. Archive task and record session when verified complete.

## Commands
- `pnpm --filter @multica/core exec vitest run skills/stores/view-store.test.ts`
- `pnpm --filter @multica/views exec vitest run skills`
- `pnpm --filter @multica/core typecheck` and `pnpm --filter @multica/views typecheck`
- `pnpm --filter @multica/web typecheck` and `pnpm --filter @multica/desktop typecheck`
- `pnpm --filter @multica/core exec eslint skills/stores` and changed views/locale files
- `git diff --check`; detector and browser evidence recorded in verification.md.

The user explicitly approved proceeding with the prior design; planning is recorded here before implementation. This work stays in the shared checkout to retain the previous task's uncommitted template-refresh prerequisite. No unrelated change is staged or reverted.
