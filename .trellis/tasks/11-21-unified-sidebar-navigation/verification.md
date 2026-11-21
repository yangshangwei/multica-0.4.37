# Unified sidebar verification

- Final verification: 2026-11-22, Asia/Shanghai.
- Task: the existing approved `unified-sidebar-navigation` implementation; no new product scope or dependencies.
- Final source/build identity: `research/final-acceptance.json`.

## Result

The unified Collaboration and Workspace sidebar is implemented in shared views and wired into Web and Desktop. The old top scope switch and duplicated account toolbar controls are removed. Existing routes, scope restoration, source selection, notification mutations, permissions and workspace identity boundaries are retained.

The continuation fixed one additional disclosure regression: manually collapse child A, navigate to child B, then return to A. An obsolete path-local override no longer hides the active child. The regression failed before the fix and passed afterward. Compact screenshot readiness now verifies the actual responsive branch, sheet geometry, opacity, animations and hit testing; earlier R2 captures were insufficient painted-layout evidence.

## Acceptance criteria

| Criteria | Evidence |
| --- | --- |
| AC1: sidebar in both scopes; no old top switch; account independent of workspace discovery | Shared/Web/Desktop unit suites; final runtime journeys assert a single directory and absent old switch. Empty/pending/failed directories are covered at the canonical unit/platform layers. |
| AC2: explicit account views, exact resume, notifications | Sidebar/notification tests cover selection, read/archive and exact links. Both runtime journeys retain selected task, filters, sort and two scroll roots, including read-error refresh and explicit anchors. |
| AC3: workspace resume and account preview isolation | Platform tests prove mirror clearing, authoritative directory selection, stale callback fencing and no preview-owned workspace requests. Runtime journeys return to the exact source task. |
| AC4: disclosures, keyboard/persistence and existing gates | Core/store and shared group/sidebar tests, including the new history regression, pins, capability gates and unread behavior. |
| AC5: New menu, source selection, compact EN/ZH and native controls | Both runtime journeys check ordinary creation availability by scope, explicit account source, returned focus, account Feedback, query-only sheet dismissal, both locales and native 200% zoom. |
| AC6: tests, checks, builds and visual evidence | Checks below; final screenshot batch inspected after animation completion. |

## Verification executed

- **275 tests passed in 21 files**: Core 7, Views 157, Web 51, Desktop 60. Exact commands and RED/GREEN evidence: `research/continuation-quality-review.md`.
- **All four affected package typechecks passed**, including Desktop Main/preload and renderer configurations.
- **Task-owned production/unit ESLint passed** in Core, Views, Web and Desktop. Locale parity and `git diff --check` passed.
- **Web production build passed**: `pnpm --filter @multica/web build`, exit 0.
- **Desktop Main/preload/renderer build passed**: `pnpm -C apps/desktop exec electron-vite build`, exit 0.
- **Final production Web + actual Electron journeys passed: 2/2, zero retries, zero skips**, 20.9 seconds reported by Playwright. Command: `pnpm exec playwright test e2e/global-scope-navigation.spec.ts --workers=1 --retries=0 --reporter=line --output=artifacts/qa/unified-sidebar-navigation-20261121/journey-final-results`.
- Synthetic password accounts complete public onboarding and are cleaned up with their owned workspaces/protocol rows. Runtime used production Web on loopback port 13493, the existing task-owned API on 18573 and freshly built Desktop renderer on 5667. No deployment env files or production CORS were changed.
- **116 tests discovered across 27 selector-synced E2E specs**; discovery is not execution. The exact edited helper/journey files passed strict TypeScript and root-scoped ESLint. Broad E2E lint has six existing findings, identical to HEAD; it is not reported as clean. Details: `research/continuation-e2e-sync.md` and its machine-readable comparison.
- Impeccable detector over changed sidebar/shell surfaces returned `[]` with exit 0. Final batched wide/compact EN/ZH screenshots passed visual review; verdict: `.omx/state/unified-sidebar-navigation/ralph-progress.json`.
- Verified source/build hashes still matched after final execution. Runtime manifest, build logs, journey log and original compositor captures remain under `artifacts/qa/unified-sidebar-navigation-20261121/`.

## Limits and remaining risks

- This is actual macOS Electron renderer/preload acceptance using the existing named Main harness, not a full installed/packaged Main, installer or Windows/Linux OS acceptance.
- The full repository E2E/Go suite was not rerun. Backend/schema are unchanged. The selected 275 tests and two complete browser/native journeys establish this navigation increment.
- Six baseline E2E lint findings were preserved: two undeclared-root imports in the existing collaboration-pay fixture and four empty Playwright fixture patterns. No new findings were introduced.
- Build tooling emitted existing pnpm configuration and CSS `::highlight` optimizer warnings; builds exited successfully.
- Unrelated pending `next-env.d.ts`, PAY-241 notes, agent-template marketplace test additions and generated/artifact files remain outside this task's commit.
