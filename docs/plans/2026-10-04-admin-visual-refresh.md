# Admin visual refresh implementation plan

**Goal:** Apply the supplied dashboard's calm surfaces, strong metrics and grouped modules to platform administration while preserving the desktop product and all previously verified administration behavior.

**Architecture:** Keep presentation under `packages/views/admin`. Use a CSS module imported only by administration components; no global selectors, root theme overrides, shared UI primitive changes or new dependencies. Keep data access, authorization, routes, filters, time boundaries and actions unchanged.

**Tech stack:** React, TypeScript, existing Tailwind tokens and primitives, CSS Modules, Vitest and Playwright.

## Scope and decision

The user requested both design and implementation after accepting the reference direction. Proceed in this session. This is a bounded presentation change, not a repository migration. Existing uncommitted UI audit fixes and unrelated MCP work must be preserved; record a session baseline rather than rolling them back or committing them.

Options considered:
1. Change shared theme tokens: rejected because web and Electron both consume them.
2. Duplicate the administration UI into the web app: rejected because it violates package boundaries and creates behavioral drift.
3. Administration-only presentation: selected. Reuse semantic tokens and isolate styling through CSS Modules.

Protected paths: `apps/desktop`, `packages/ui`, `packages/core`, non-admin views including layout, settings, issues and chat. The root `.dark` preference may be inherited but must never be mutated by administration.

## Design

- White navigation/header and content panels over a very lightly tinted canvas; a muted blue-green accent derived from existing brand/success tokens.
- Navigation selection remains recognizable on hover; compact Sheet retains current focus, dismissal and access-loss behavior.
- Overview: title/description and refresh; quality/timestamp; four key metrics (open alerts, running, queued, ready); remaining attention items; compact current capacity; historical filter section; grouped outcomes, alert history, usage and timing.
- Main figures use existing display type tokens with tabular numerals. Unknown and zero remain distinct. Links retain their names, destinations and query semantics.
- Non-overview administration routes receive a single consistent content surface without rewriting their forms or tables.
- Four columns on wide screens, two on intermediate screens and one/two as content permits on phones. Panels stack in DOM order. Touch targets remain at least 44px; support Chinese, English, dark mode and enlarged text.
- No fabricated trends, progress bars or sample data in product source. No billing semantics attached to reported tokens.

## Implementation sequence

1. Record baseline administration tests, protected-path fingerprints and desktop route/style dependencies.
2. Add administration CSS module and apply it only to AdminShell and its Sheet portal. Preserve navigation logic and authorization branches.
3. Recompose overview using local metric/section helpers, keeping all original fields and links. Add refresh with existing query.refetch and label.
4. Add meaningful regressions for key metric/link semantics, null vs zero and refresh. Keep prior filter/history/access tests.
5. Verify affected tests and lint; run repository typecheck, desktop tests and renderer build. Inspect desktop/browser dependency output for administration style isolation.
6. Capture authenticated web overview plus representative account/terminal/audit pages at desktop/phone widths and both themes/locales. Check overflow, navigation, filters and keyboard behavior. Read-only browser fixtures may exercise populated states and must be labelled in the report.
7. Compare native desktop business shell before/after where the local runtime permits. Verify protected-path hashes; report any unrelated concurrent edits separately.
8. Independent review, bounded visual verdict and final evidence report. Keep local preview running for the user.

## Acceptance

- Reference-inspired hierarchy is apparent without changing business semantics.
- No global CSS/token or Electron route/runtime changes.
- Existing administration regression tests remain green; unknown data never becomes a success-colored zero.
- Compact navigation, current-link focus, validation and history drilldowns work.
- No overflow in the tested Chinese/English light/dark layouts; user actions remain reachable.
- Desktop verification distinguishes native runtime checks from renderer/typecheck evidence.

## Runtime precautions

Do not run `make up`, `make destroy`, `make gc` or broad cleanup for this task. `make up` previously triggered automatic cleanup of expired environments. Reuse localhost:13493 and the retained audit API localhost:18393. The restored original API listens on localhost:18573.

## Implementation record

- Implemented: scoped shell and Sheet portal, non-overview panel wrapper, overview metrics/sections and manual refresh.
- Source files: `admin-shell.tsx`, `overview/overview-page.tsx`, `overview/overview-page.test.tsx`, `admin-visual.module.css`, `admin-visual.module.css.d.ts` (all within `packages/views/admin`).
- Documentation: this plan and the administration surface convention in `.trellis/spec/views/frontend/component-guidelines.md`.
- Reused: existing tokens, primitives, labels, hooks, filters and drilldown builders. No dependency, endpoint, database or shared theme changes.
- Baseline: 83 administration tests passed. Current administration plus locale tests: 148 passed; core/admin: 142 passed. Desktop: 957 tests, both typechecks and isolated production build passed.
- Visual correction: use rectangular OKLab to preserve the intended cool tint when mixing accent and neutral colors; compact metrics use two columns and at least 44px link targets.
- Independent review: approved after correction; no behavioral blockers.
- Evidence directory: `.omx/reports/admin-visual-refresh-2026-10-04/`.
- Native Electron visual smoke is not claimed: the existing app has no debugging port and this session has no working CUA surface. Its process/profile were not restarted or modified.
