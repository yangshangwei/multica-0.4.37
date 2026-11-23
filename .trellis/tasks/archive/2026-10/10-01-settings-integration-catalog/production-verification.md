# Production implementation verification

Implemented the approved integration catalog in shared Web/Desktop settings.
The existing GitHub, self-hosted Git and deployment-enabled messaging forms are
mounted only for their selected detail. ONES, Plane, Kaneo and Fuxin remain inert
planned entries. The existing deployment-gated Composio surface and callback
handling remain available without adding an applications/tools category.

## Changes

- `packages/views/settings/components/integrations-tab.tsx`: categorized catalog,
  existing forms, workspace query status, accessible links and focus restoration.
- `settings-integration-navigation.ts` and `settings-page.tsx`: shared URL handling,
  backend callbacks, directory/detail widths and main-tab query cleanup.
- `github-tab.tsx`: existing business behavior retained; section hierarchy,
  narrow-screen wrapping/touch targets and repository shortcut corrected.
- Both settings locale files, the four shared test suites, and
  `e2e/settings-integration-catalog.spec.ts`.
- `.trellis/spec/views/frontend/component-guidelines.md`: late config loading,
  callback, status accessibility and switch testing contracts.

## Verified

- `pnpm typecheck`: all 9 tasks passed, including Web and Desktop.
- `pnpm --filter @multica/views lint`: 0 errors; 26 existing warnings.
- `pnpm --filter @multica/views exec vitest run --maxWorkers=2 --testTimeout=15000`:
  472 files and 5,866 tests passed on the final run.
- Production Web: English and Chinese flows passed at 320, 390, 768, 900, 1024
  and 1440px. Covered keyboard entry, return focus, browser Back/Forward, refresh,
  legacy GitHub callback, query/hash cleanup, master-off/on preference retention,
  disconnect cancel/confirm, cache status refresh, and planned-detail rejection.
- Existing Composio callback regression: passed.
- Native Electron: actual preload, renderer and memory-router; two tests passed.
  Covered settings entry, catalog/detail Back/Forward, keyboard focus, legacy URL,
  repository URL context and 420px overflow. No renderer exceptions or unexpected
  daemon, installer or external-link calls.
- Visual verdict 95/100, no detector findings, `git diff --check` passed.

Web uses an isolated `next build` + `next start`, build
`XU37jhEnBEZ-xNLFZFGVW`, with separate local API/database. The final application
files byte-match the source checkout. See `production/verification.json`, logs,
provenance records and screenshots.

## Verification boundaries

Real GitHub OAuth was not invoked: installation and disconnect responses were
mocked, while workspace preference updates exercised the local API. Native
settings verification used a temporary copy of an existing test to tolerate its
fixture's extra `authSessions` field; the adjustment and smoke source are saved
in `production/`. The original test was not changed.

An invalid or disabled provider shows the catalog at detail width while retaining
its URL. This avoids losing a valid link when deployment configuration arrives
late. Existing messaging heading hierarchy was left outside this GitHub-focused
change. No backend behavior changed, so Go tests were not rerun.

An initial unbounded test run was interrupted after resource contention; one
existing dialog test failed during a subsequent concurrent run and passed on
isolated retry. The final bounded full run passed all 5,866 tests.
