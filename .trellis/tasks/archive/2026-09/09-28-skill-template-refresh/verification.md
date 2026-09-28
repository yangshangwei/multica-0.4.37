# Verification

## Regression evidence

- Before the fix, six of seven live Query regressions failed against production
  QueryClient defaults, and source-switch discovery failed in the real dialog.
- After the fix, both core catalog suites passed: 11 tests.
- Shared skills suite passed: 23 files, 363 tests before the final additional
  component lifecycle regression. The final panel/creation-flow rerun passed
  all 66 tests, including that new regression.
- The component lifecycle test was mutation-checked: suppressing the polling
  opt-in at the test boundary made discovery fail; the temporary mock was removed.
- Core and views typechecks passed. Both package lints passed with 28 existing
  warnings outside edited files. Edited-file lint and `git diff --check` passed.
- Independent review found no implementation defects; its component-wiring
  coverage suggestion was implemented and verified.

## Actual Electron and filesystem check

The existing Electron acceptance fixture ran the real desktop renderer/preload
at localhost:5666, with an isolated profile, a dedicated workspace, and a
separate API at localhost:18573 configured with a temporary template directory.
The temporary API allowed the renderer origin; production configuration was
unchanged. `TestApiClient` handled workspace setup/cleanup.

- First displayed an empty deployment source, then wrote a real SKILL.md.
- The picker discovered it in **29,976 ms** without navigation or reload.
- Changing the file and switching back to the deployment tab showed the update.
- Removing the file and revisiting the deployment tab removed the entry.
- After adoption and local editing, another file change did not replace the
  edited instructions after 31 seconds.
- Electron PID remained 56137 throughout the checks. No browser page errors.
- Playwright result: **1 passed (1.1m)**.

Evidence is under `.omx/skill-template-refresh/`: `observations.json`,
`before-empty.png`, `after-discovered.png`, and the task-specific smoke script.
The temporary Electron profile, workspace, template directory, and env copy
were removed; the separate API was stopped.

## Final state

The user's original API still reports status=ok on localhost:18572 with PID
44462, and the original desktop renderer still returns HTTP 200 on port 5666.
Vite delivered the query/panel changes to the running desktop.

No backend implementation, dependencies, API shape, or database schema changed.
Existing unrelated `scripts/dev-env.sh` and `.impeccable/` edits were preserved.
The fix is verified in the local development build; packaged clients need a
release containing the updated shared frontend code.
