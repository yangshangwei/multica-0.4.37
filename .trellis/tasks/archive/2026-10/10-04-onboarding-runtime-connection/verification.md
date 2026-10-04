# Verification

## Root cause

The running Desktop process inherited a quoted, stale `MULTICA_SERVER_URL` from
the Makefile's environment include. CLI URL resolution prioritizes that variable
over the Desktop-owned profile, so daemon startup failed before CLI discovery.
The fix removes only this override from the child environment and shares that
environment between start, stop and local probes. PATH and explicit agent paths
are preserved.

## Automated checks

- Six regressions for malformed/stale server overrides across start, stop and
  probe failed before the fix and passed afterward.
- Desktop daemon suites: 6 files, 96 tests passed.
- Onboarding and locale parity: 17 files, 172 tests passed.
- Views typecheck and Desktop main/renderer typechecks passed.
- Focused Desktop ESLint passed. Views ESLint has no errors and one pre-existing
  missing `t` hook dependency warning in `onboarding-flow.tsx`.
- Independent read-only review found no actionable defects.
- `git diff --check` passed.

## Browser and local runtime checks

- Rendered the real shared desktop and web components with an empty runtime
  fixture. Cloud choices are gone; the remaining layouts have no horizontal
  overflow. Clicking the desktop Skip card invoked its callback.
- Screenshots: `/tmp/multica-runtime-connect-after.png` and
  `/tmp/multica-runtime-web-after.png`. Visual verdict: 96/pass, recorded in
  `.omx/state/onboarding-runtime-connection/ralph-progress.json`.
- Rebuilt/relaunched the existing development Desktop using its original
  renderer port and app suffix. Current `/api/me` returns 200.
- Local daemon health reports running against the selected server, with six
  registered runtimes: Claude, Codex, Cursor, Hermes, OpenClaw and OpenCode.
  Current logs show runtime registration and acknowledged WebSocket heartbeats.

## Limits

- Qwen is independently excluded: installed 0.2.3 is below the supported 0.20.0
  minimum. No global tool upgrade was performed.
- No real-agent task was submitted or quota-consuming execution tested.
- No packaged application release was produced.
