# Verification

## Changes

- `apps/desktop/src/renderer/src/routes.tsx`: translate the daemon settings navigation label through the existing settings namespace.
- `apps/desktop/src/renderer/src/components/daemon-settings-tab.tsx`: translate page copy, conditional notices, diagnostic labels, state labels, save feedback, and displayed uptime units.
- `packages/views/locales/{en,zh-Hans,ja,ko}/settings.json`: add matching `desktop.tabs.daemon` and `desktop.daemon` resources without altering pre-existing edits.

The existing `useT` hook and uptime formatter are reused. No dependencies, new production abstractions, IPC changes, or backend changes were introduced. Existing product conventions already cover this localization; no spec change was needed.

## Automated checks

- Desktop node and renderer TypeScript checks: passed.
- Shared views TypeScript check: passed.
- ESLint for both modified desktop source files: passed.
- Settings navigation, page, preferences, and locale parity: 4 files, 206 tests passed.
- Daemon shared types, recovery, authentication probe, profiles, OS detection, reauthentication, login sync, runtime card, and update settings: 9 files, 84 tests passed.
- `git diff --check`: passed.
- TypeScript AST comparison against the original component: identical IPC calls, event handlers, checked/disabled bindings, state defaults, and status subscriptions.
- Translation interpolation placeholders match English in all three translated locales.

## UI verification

- Native desktop inspection confirmed the Chinese navigation and page, live diagnostics, auto-start enabled, and auto-stop disabled.
- An isolated preview using the real component and in-memory IPC verified both toggle payloads, successful/failed saves, external-management disabling, the unchanged installation guide URL, authentication-expired UI, CLI checking/missing states, English fallback, and Chinese hour/minute/second display.
- Initial broad selectors matched multiple elements for two preview waits. Both were rerun successfully with exact selectors; these were verification selector errors, not product errors.
- Normal, externally managed, CLI missing, and expired-authentication screenshots were visually inspected. No clipping or layout regression was found. Visual verdict: 98/100, pass.
- Preview console: no errors.

Evidence is stored under `.omx/state/daemon-settings-zh/` (screenshots, UI logs, typecheck logs, static checks, and `ralph-progress.json`).

## Limits

Real daemon start/stop and credential renewal were not triggered during verification. Their code and event wiring are unchanged, and existing regression suites passed. Raw server/IPC error details, device names, profile identifiers, URLs, and literal commands remain intact. No new functional risk was identified.

All unrelated working-tree edits were preserved. Changes are left uncommitted for the existing workspace workflow.
