# Desktop unified sidebar implementation

- Date: 2026-11-21
- Scope: Desktop shell and account-route wiring only

## Changes

- `apps/desktop/src/renderer/src/components/desktop-layout.tsx` always mounts the shared sidebar and its single external toggle. The navigation workspace is selected from the actual active workspace, then the remembered available workspace, then the first authoritative directory item. It is never written into account workspace context or the API mirror.
- Account, pending, failed, zero-workspace and stale-slug states keep the sidebar, tab content and navigation feedback available. Workspace search, shortcuts, modals, floating chat and update integration remain gated by the actual active workspace.
- Workspace heading selection calls the existing `switchWorkspace(slug)` without an explicit path, retaining each workspace's tab session. Collaboration heading uses `resumeDesktopScope("account")`, retaining its workspace-header clearing and exact account resume behavior.
- Removed `components/account-navigation.tsx` and the separate 48px scope-switch row. A single top-left 200px drag strip reserves the native traffic-light area and places sidebar/history controls after 92px. Hidden-sidebar tabs reserve the same 200px; expanded-sidebar tabs keep zero extra padding. The original reduced-motion geometry logic remains in use.
- `platform/account-workbench-route.tsx` passes `navigationInSidebar` to suppress duplicate workbench navigation while retaining account setup/password/auth guards and `setCurrentWorkspace(null, null)`.

## Regression evidence

- Added shell tests for account/sidebar presence, null account workspace context with a remembered navigation workspace, removed-workspace fallback, pending/error directory access, independent retry, resume callback arguments, one directory and no old scope row.
- Updated reduced-motion tests for the 200px native-control clearance while retaining interruption and restoration checks.
- Added an account-route regression for sidebar ownership plus mirror clearing and each existing account restriction.
- The new shell assertions first produced 10 intended failures against the old shell (sidebar absent in account/stale states, old row present, old geometry and missing selected navigation workspace/retry). They passed after implementation.

## Verification

- PASS: `pnpm -C apps/desktop exec vitest run src/renderer/src/components/desktop-layout.test.tsx src/renderer/src/components/desktop-layout.workspace-gate.test.tsx src/renderer/src/platform/account-workbench-route.test.tsx src/renderer/src/platform/navigation.test.tsx src/renderer/src/stores/account-tab.test.ts` — 5 files, 60 tests.
- PASS: `pnpm -C apps/desktop typecheck` — node and renderer checks completed with exit 0.
- PASS: ESLint for changed Desktop TS/TSX files and the new route test.
- PASS: `git diff --check` for Desktop-owned changes.

## Remaining verification boundary

No build, browser/native session or screenshot was run by this lane. Main owns the integrated production Web/renderer build and actual Electron journey. Native 200% zoom, traffic-light hit regions and painted compact/sidebar geometry need that integration evidence; JSDOM assertions do not prove native rendering. No backend, navigation adapter, tab-state migration or dependency changes were made.
