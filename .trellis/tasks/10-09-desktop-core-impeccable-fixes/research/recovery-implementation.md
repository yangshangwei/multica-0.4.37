# Collection/project recovery implementation review

## Boundary before new edits

- Owned requirements: DCF-02, project selection in DCF-03, and DCF-04.
- Starting candidate code already distinguishes cold/cached errors in Inbox,
  ProjectsPage, and ProjectDetail; project rows already use named Checkbox
  controls and AppLink titles with row interaction isolation.
- Initial lane verification: InboxPage recovery, ProjectsPage, and ProjectDetail
  suites passed **35/35 tests** (3 files). No product defect was demonstrated.
- The smallest remaining gap is verification: ProjectsPage's query recovery
  tests replace `useQuery` with result branches, so they do not prove a real
  failed read/retry, cache retention, or access revocation transition.
- The behavior lives in the existing `projectListOptions` and protected request
  wrapper plus ProjectsPage rendering. Add real-query cases to its canonical
  component suite using the existing partial-real-query testing pattern from
  ProjectDetail; keep generic access classification matrices in core tests.
- Expected new files/edits: `projects-page.test.tsx` for QueryClient coverage,
  and this lane record for provenance and results. Product/locale changes are
  only warranted by a reproduced failure.
- Outside this lane: chat, issue/agent/skill lists, timeline, desktop/triage,
  global Dialog, other tasks' state/specs, commits and staging.
- No cleanup/refactor or dependency changes are proposed.

## Verification and results

New edits are limited to `projects-page.test.tsx` and this record. SHA-256
comparison against `evidence/baseline.json` confirmed the owned Inbox/Projects
product files, Inbox/ProjectDetail tests, and both Inbox/Projects locale pairs
are unchanged from the starting candidate repairs.

The canonical ProjectsPage suite now conditionally runs its production
`projectListOptions`, `protectProjectRequest`, QueryClient, and real project
access store, following the existing ProjectDetail partial-real-query pattern.
Three new regressions establish:

1. Cold `ApiError(500)` displays the localized load error and Retry, not the
   create-first-project empty state. Retry uses the real `select(data.projects)`
   projection and workspace/AbortSignal options, restores the title link, and
   does not navigate away.
2. Cached `ApiError(500)` preserves the row/search DOM identities, keyboard
   selection, search text/focus, and active status filter. A delayed Retry is
   disabled while fetching; its success clears only the refresh notice.
3. A real protected-query `ApiError(403)` hides selected cached rows, removes
   their query data, offers no retry of protected content, and does not loop
   requests. Core classification matrices remain in their existing owner.

Existing candidate regressions continue to cover active/archive Inbox recovery,
selected URL retention, mounted reply/value/focus retention, compact detail
Retry, successful empty Inbox, ProjectDetail 500 vs 403/404, mounted progress
draft retention, project title Tab/Enter singular navigation, modifier/middle
click isolation, and project mixed selection.

| Check | Actual result |
| --- | --- |
| `pnpm --filter @multica/views exec vitest run inbox/components/inbox-page-recovery.test.tsx projects/components/projects-page.test.tsx projects/components/project-detail.test.tsx` before edits | 3 files, 35 tests passed; 3.94 s |
| Same command after new real-query coverage | 3 files, 38 tests passed; 4.28 s; exit 0 |
| `pnpm --filter @multica/views exec eslint inbox/components/inbox-page.tsx inbox/components/inbox-page-recovery.test.tsx projects/components/projects-page.tsx projects/components/projects-page.test.tsx projects/components/project-detail.tsx projects/components/project-detail.test.tsx` | Exit 0; 0 errors, 2 existing `project-detail.tsx` Hook warnings at lines 152 and 257 |
| Scoped `git diff --check` across the six owned product/test files | Exit 0, no output |

The existing native-web modifier-click test emits jsdom's `Not implemented:
navigation to another Document` diagnostic. It is unchanged from the initial
passing run; the assertions intentionally preserve native anchor behavior.

## Remaining handoff

- The API method is stubbed in these component regressions. They prove real
  Query/cache/permission transitions, not HTTP parsing, production retry
  backoff, computed focus styling, or native tab behavior. Main-session isolated
  Electron fault injection owns those acceptance claims.
- Main session owns workspace typecheck, broader integration checks, locale
  parity, detector, final specs/report, task state, and commit selection.
- Retain incumbent product implementation: no additional product defect was
  demonstrated in this lane, so no redundant product repair or abstraction was
  added. No dependencies, business writes, staging, or commits were performed.
