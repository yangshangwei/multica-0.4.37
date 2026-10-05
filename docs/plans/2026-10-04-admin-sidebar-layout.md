# Admin navigation and settings layout

## Direction

Refine the existing Multica administration surface in Operate mode. Use a quiet
240px desktop sidebar with the six existing destinations, place the current
group's child links directly below its parent, and keep return/sign-out actions
at the bottom. The content pane has a compact location header and a consistent
left reading edge. Below 768px, reuse the shared Sheet for navigation, with
selected-link focus, Escape dismissal, focus return, and closing on navigation
or a switch to the desktop viewport.

Retain the current neutral tokens, typography, routes, permissions and copy.
Consolidate observation quality/time into one supporting metadata row. Present
deployment settings as aligned definition rows, with wrapped values and readable
section spacing. Unknown values remain unknown; settings remain read-only.

## Bounded implementation

1. Update existing shell tests for navigation containment; cover the new compact
   menu, active child navigation and protected-state behavior before editing.
2. Replace the horizontal shell with shared desktop/compact navigation. Reuse
   Sheet, Button, useIsMobile, AppLink and existing localization keys; add only
   the two missing accessible menu action labels in both locales.
3. Tighten observation metadata and settings rows without changing other metric
   layouts, data fetching or any backend code.
4. Run focused admin tests, locale parity, views/web typechecks and scoped lint.
   Inspect desktop settings and another data page, compact navigation/content,
   English labels and dark mode in one batched browser pass. Make at most one
   correction batch, then confirm. Persist visual verdicts and evidence.

## Boundaries

No dependencies, route changes, API changes, invented values, or new admin
capabilities. Preserve existing unrelated password-form/auth locale edits.
Do not commit or publish unrelated work.

## Verification

- Admin and locale parity suites: 16 files, 107 tests passed.
- Views and Web typechecks passed; scoped ESLint and whitespace checks passed.
- Impeccable mechanical detector returned no findings.
- Browser checks passed at 1440px, 768px and 390px, including English and dark
  mode. No document/main horizontal overflow or page errors were observed.
- Compact navigation focuses the current link, dismisses after selection or
  Escape, restores trigger focus, and closes on desktop resize. The access-loss
  test failed before the additional dismissal condition and passes after it.
- Final visual verdict: 94/100. Screenshots and browser evidence are in
  `.omx/reports/admin-sidebar-layout/`; verdict is in
  `.omx/state/admin-sidebar-layout/ralph-progress.json`.
- Verification used the local development server; no production build, backend
  change or deployment was part of this task.
