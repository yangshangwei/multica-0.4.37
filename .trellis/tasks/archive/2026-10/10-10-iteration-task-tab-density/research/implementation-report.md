# Implementation report

## Boundary and cleanup plan

The behavior gap is excessive space before tasks, unavailable closed-filter feedback, and ambiguous phase/row metadata. The task owns the shared iteration page/list and bilingual copy. The existing lifecycle helper and query factories remain authoritative.

1. Extend canonical detail/navigation regressions for applied-condition removal/reset, phase wiring, and unknown historical rows.
2. Compact the existing summary and toolbar using shared Select/Popover/Button primitives. Remove the repeated Tasks heading and unconditional matching-count row.
3. Derive conditions from existing filter state; reuse one full reset while retaining grouping.
4. Make titles primary, move identifier/project/positive rollover into meaningful metadata, and give category/assignee stable readable positions.
5. Run focused suites, locale parity, lint and typecheck. Parent owns live browser, broader final verification and specification updates.

No core, server, global primitives, settings, dependencies, E2E fixtures or unrelated working-tree edits are owned here. The parent approved extending test ownership to `iteration-navigation.test.tsx` because it already owns page/tab/phase integration.

## Baseline

- `pnpm --filter @multica/views test iterations/iteration-details.test.tsx iterations/iteration-page.test.tsx iterations/iteration-history.test.tsx`: **27 tests passed, 3 files**, 2026-10-10.
- Existing unrelated changes recorded before editing: iteration settings, project description, next-env, and Trellis records/spec.

## Implementation and verification

### Files and behavior

- `packages/views/iterations/iteration-page.tsx`: pass the existing lifecycle phase into Tasks; use inline whole-period counters and a concise planning row; retain statistical explanation in an accessible help popover. Snapshot counters remain authoritative.
- `packages/views/iterations/iteration-issue-list.tsx`: keep a bounded search visible, including empty plans; expose grouping independently; move five field filters to the shared Popover. Derive removable conditions and count from existing state (typed assignee counts once), and reuse a single reset that clears search/fields/scope/cursors while preserving grouping. Current/original selection requires a confirmed started phase. Unknown-phase transitions restore current scope without dropping other refinements.
- Task rows prioritize the title, with identifier/project/positive rollover as metadata. Category and assignee have stable columns at wide container sizes and wrap at narrow sizes. Blocked tasks have both text and a warning icon. Missing stored identities remain unknown, null assignees read Unassigned, and review guidance remains at rollover >= 3.
- `packages/views/locales/{en,zh-Hans}/projects.json`: localized filter trigger, planning/unknown context, statistics help and reset explanation. Existing issues translations supply filter counts, removal/reset names and Unassigned.
- `iteration-details.test.tsx` and `iteration-navigation.test.tsx`: canonical regressions for controls, focus return, conditions/reset, source availability, counter independence, empty plans, tab retention, unknown metadata and rollovers. The capability-only `iteration-page.test.tsx` and history/chart suite remain unchanged.

### Commands and evidence

- Before production edits, new regressions failed as expected: **9 failed, 75 passed** across details/navigation. Failures covered the missing popup, condition/reset UI, title hierarchy, unstarted scope restriction and empty-plan search.
- `pnpm --filter @multica/views test iterations/iteration-details.test.tsx iterations/iteration-navigation.test.tsx iterations/iteration-page.test.tsx iterations/iteration-history.test.tsx locales/parity.test.ts`: **156 passed, 5 files** (final run).
- `pnpm lint`: **exit 0, 6 tasks successful**. Existing unrelated warnings remain (views: 27 warnings, 0 errors); no warning names a task-owned file.
- `pnpm typecheck`: **exit 0, 9 tasks successful**.
- `pnpm --filter @multica/views typecheck`: **exit 0**, after the final test additions.
- `pnpm --filter @multica/views exec eslint iterations/iteration-details.test.tsx iterations/iteration-navigation.test.tsx`: **exit 0**, after the final test additions.
- `/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json packages/views/iterations/iteration-page.tsx packages/views/iterations/iteration-issue-list.tsx`: **exit 0, `[]`**. This is static evidence only.
- `git diff --check`: **exit 0**.
- Tool logs are under `/tmp/iteration-task-tab-{red,focused,lint,typecheck,final-test-lint}.log` for this session.

### Remaining verification ownership

Parent owns production Web/Desktop rendering, bilingual wide/narrow/coarse-pointer captures and any bounded follow-up visual batch, full workspace tests, UI exports, E2E fixture updates, durable spec updates and task closeout. No backend changes or Go checks are required by this diff. No commits were made; unrelated edits were preserved.
