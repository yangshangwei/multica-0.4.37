# Skill library audit fixes

**Goal:** Resolve the ten findings in `docs/audits/2026-09-30-skill-library-impeccable.md`, as requested by the user after reviewing the audit.

**Architecture:** Preserve the shared web/desktop skills surface and existing view store. Use real AppLink titles inside pointer-clickable rows, directly labelled Checkbox controls, existing semantic tokens and existing presentation summaries. Keep the virtualized card height contract explicit. Do not change shared navigation behavior, global checkbox styling, stored skill instructions, or unrelated agent work in progress.

**Tech stack:** React, Base UI, TanStack Virtual, Zustand, existing i18n catalogs, Vitest and Playwright. No dependencies added.

## Implementation and cleanup sequence

1. Add failing behavior regressions for title navigation and selection isolation, category multi-selection, effective toolbar settings, and search/filter recovery.
2. Toolbar/navigation lane: name compact controls, expose keyboard-accessible filter clearing, restrict column settings to list view, and make sidebar/chips reflect zero/one/multiple categories truthfully. Preserve sorting and persistent preferences.
3. Card/list lane: replace presentational-checkbox wrappers with labelled controls, use visible contrasting boundaries, add AppLink titles, expose translated full titles, and clearly label origin metadata.
4. Reallocate card space: use display-only summaries for recognized built-ins, two-line names, conditional labels, and an explicit virtualizer-compatible height. Preserve stored descriptions and the existing visual identity.
5. Render a shared page-level no-results state with a cause and working search/filter reset. Announce result changes without duplicating visible counts.
6. Run focused regressions, skills suite, locale parity, affected ESLint and typecheck, and the Impeccable detector. Check browser behavior and desktop/compact screenshots in one batch, fix discovered issues together, then confirm once.
7. Record visual verdict under `.omx/state/skill-library/ralph-progress.json`, update scoped skill presentation guidance, and append actual verification evidence to the audit. Leave unrelated changes untouched.

## Ownership

- Main agent: skill-card, skill-card-grid, skills-page, related tests, both skills locale catalogs, E2E validation, documentation and integration.
- Independent executor: skill-list-toolbar, skill-category-sidebar, skill-category-chips and their focused tests. Request locale keys from the main agent.

## Acceptance

- Tab/Enter opens skill titles exactly once; checkbox/menu interactions do not navigate.
- Selection controls remain visible, have names, and expose checked/mixed state.
- Compact filters retain names and all input methods can clear filters.
- Multiple selected categories never report All as active; list-only controls stay in list mode.
- Sources are identified as metadata, localized long titles remain obtainable, and no empty label slot wastes card space.
- Empty search/filter results explain their cause and can be reset in place.
- Focused tests and checks pass, with any unrelated workspace failures identified explicitly.

## Verification commands

- `pnpm --filter @multica/views exec vitest run skills locales/parity.test.ts`
- `pnpm --filter @multica/views exec eslint <changed TS/TSX files>`
- `pnpm --filter @multica/views typecheck`
- `pnpm exec playwright test e2e/skill-library-accessibility.spec.ts`
- `node /Users/artisan/.agents/skills/impeccable/scripts/detect.mjs --json <changed skill component files>`

## Rollback

Revert only this task's named files/hunks. Card dimensions and virtualizer estimates must be reverted together. There are no database, API, dependency or persistence migrations.

## Completion evidence

All ten audit findings have corresponding fixes. Final validation: 454 tests in 27 files, views typecheck, affected ESLint, diff whitespace check, static detector and the real Chromium E2E passed. Two bounded visual rounds covered desktop light/dark and 320/375px coarse-pointer layouts. Card lines use explicit 144/168px heights and height-aware identity to preserve virtualization. See the audit appendix for files, evidence and remaining platform coverage.
