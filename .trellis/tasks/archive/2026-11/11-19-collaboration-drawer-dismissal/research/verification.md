# Verification — 2026-11-19

## Correction after native-pointer feedback
The close-button checks below used CUA accessibility element activation, not an
OS coordinate click. They verified the first hidden-lifecycle repair but did
not prove native mouse delivery. The user subsequently reported X still inert
while Escape/backdrop worked. A real screen-coordinate click reproduced a
second cause: the resource popup overlaps Electron's drag region without a
no-drag boundary. Follow-up task: `11-19-collaboration-drawer-native-close`.
The original claim of complete native mouse dismissal was too broad.

## Result
The retained collaboration drawer and its backdrop now disappear after close. Base UI owns native hidden lifecycle while scope visibility still suppresses hidden surfaces. Editors stay mounted.

## Red / green evidence
- Baseline auxiliary suite: 4 failed / 1 passed, closed dialog unexpectedly queryable.
- New regression before repair: `pnpm --filter @multica/views exec vitest run collaboration/detail-auxiliary.test.tsx -t 'hides the complete drawer'` failed at `expect(drawer).not.toBeVisible()` with a visible data-closed dialog.
- After repair: `pnpm --filter @multica/views exec vitest run collaboration/detail-auxiliary.test.tsx collaboration/collaboration-detail.test.tsx layout/animated-right-sidebar.test.tsx` — 44 passed (6 auxiliary, 15 detail, 23 sidebar).
- Independent reviewer: create-skill-dialog suite — 14 passed; focused iteration edit close/reopen draft test — 1 passed (61 unrelated cases skipped).
- Total relevant executed tests: 59 passed. Existing auxiliary suite act/key warnings are unchanged.
- UI and Views package typechecks passed.
- ESLint on both changed TypeScript files and scoped git diff --check passed.
- Independent Trellis check found no blocking findings and changed no files.

## Native Electron acceptance
Actual running Multica Canary work-management-local from this checkout, renderer http://localhost:5666/, operated with native CUA.
1. Opened original reported issue's Activity drawer; actual history displayed.
2. Clicked top-right Close resources. Drawer and blur/backdrop disappeared; screenshot showed unobscured task page. AX focus returned to Activity opener.
3. On another existing task: opened Activity, pressed Escape, reopened Activity, clicked top-right Close resources. History reappeared on reopen; each close returned focus and removed the full shell/backdrop.
4. Finished with the task page visible and developer tools closed. No task writes/submissions were made.

The temporary blank window during verification recovered after raising the native window and re-reading its UI; no additional product code changes were needed. A CUA timeout required reconnecting once; subsequent close/reopen validation completed.

## Scope / limits
Only shared Dialog prop forwarding, the auxiliary regression, and the UI contract were changed. No new dependencies or backend changes. This does not claim the full E2E suite, packaged-release, Windows/Linux, or standalone Web browser validation. Other popup families were inspected but unchanged.

## Source SHA-256
- `packages/ui/components/ui/dialog.tsx`: `d323c7d47d11ba05cb879472389dc5ca1e24f18d54875b8de7d6dfe8befedf3e`
- `packages/views/collaboration/detail-auxiliary.test.tsx`: `bbb78c1b68a9a3cd71c71c2b41cd1cb0c7b5e5f3a9e63b61333358ed2430732a`
- `.trellis/spec/ui/frontend/component-guidelines.md`: `24484f419538ac9371e42a36701a4280a2307cd52a2fb2dfd28779e5962bf984`
