# Verification: create project polish

## Changed files
- packages/views/modals/create-project.tsx: content-driven height, viewport cap, compact resource row, fewer repeated optional markers.
- packages/views/projects/components/project-squads-picker.tsx: one muted configuration surface; default badge next to a single selection; default explanation moved into the menu; secondary runtime deferral.
- packages/views/agents/components/inspector/runtime-picker.tsx: opt-in compact field with bounded device subtitle and explicit online/offline text, preserving canonical identity/tooltip.
- packages/views/projects/components/project-squad-picker.test.tsx: verify the initial default is visible.
- packages/views/agents/components/inspector/runtime-picker.test.tsx: online/offline compact field identity coverage.
- packages/views/locales/{en,zh-Hans}/modals.json: only resources_hint changed by this task. Other existing changes belong to separate work.
- packages/views/locales/{en,zh-Hans}/projects.json: shorter purpose and preparation copy.

## Checks
- Baseline: 4 targeted suites, 38 tests passed before editing.
- After implementation: 5 suites including locale parity, 105 tests passed.
- After adding compact-status/default assertions: the 2 affected suites passed, 22 tests. Total relevant coverage now 107 tests.
- Views TypeScript check: passed, including final test changes.
- Views full lint: exit 0, 0 errors; 26 pre-existing warnings outside changed files. Scoped source and test lint clean.
- Impeccable detector on 3 edited UI components: 0 findings.
- git diff --check: passed.
- Independent read-only review: no introduced issues found.

## Rendered checks
Playwright loaded the actual current Next.js source at localhost:13494 with mocked API responses. Existing localhost:13493 serves a stale production build and was not modified. No backend or agent process behavior is claimed by this visual check.

Verified Chinese desktop 1280×900, narrow 390×844, short 390×568, expanded view, dark theme and English narrow view. Checked empty-title disabled state, filled-title enabled state, ordered multiple selections, runtime deferral retaining squads, and footer visibility. Browser page errors: 0. Full-runtime identity remains available through the existing accessible label and tooltip.

Artifacts: screenshots/desktop.png, narrow.png, narrow-multiple.png, deferred.png, short-window.png, expanded.png, dark.png, english-narrow.png. visual-check.cjs is a task-local reproducible browser fixture. Visual verdict: .omx/state/project-modal-polish/ralph-progress.json (93/pass).

## Limits and remaining risks
This task changes shared web/desktop presentation only. Actual backend creation remains covered by existing unit/component contracts; no new live-database E2E or packaged Electron build was run. Unrelated workspace changes and existing lint warnings remain outside this task.

## Spec decision
No new domain rule was introduced. Existing runtime-display and ordered project-squad contracts remain authoritative; the compact field presentation is opt-in. No shared spec update required.

Implementation committed as `5f8095bf9405cea41d2b0a8752b8f9a37a002025`. The final combined main snapshot passed the 5657-test views suite, full typecheck/lint and production Web build before push. Shared locale changes were staged by feature so unrelated pending work was not mixed into this commit.
