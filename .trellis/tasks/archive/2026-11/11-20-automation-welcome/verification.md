# Automation welcome verification

## Result

The approved automation welcome is complete. An independently reviewed,
successfully loaded empty workspace shows the introduction, blank-creation and
documentation actions, six real server templates and complete-catalog access.
Saved automations, including paused instances and filtered-out results, retain
the management list.

The welcome presentation reuses the existing catalog metadata, schedule
formatter, creation route and one-shot scroll/focus handoff. No backend,
dependency, API, global-token or native-mobile change was introduced.

## Checks

| Check | Result | Evidence |
| --- | --- | --- |
| Affected Vitest and locale parity | Pass: 4 suites, 122 tests | Fresh Trellis implementation run: page, catalog, template configuration and locale parity. |
| Focused ESLint | Pass | Both Trellis implementation and independent full-scope check verified the two UI files and their component suites. |
| Views TypeScript | Pass | Both independent `pnpm --filter @multica/views typecheck` runs exited 0. |
| Production Chromium | Pass: 14 cases, 40.3 seconds | `.impeccable/review/automation-welcome/playwright-final.log`; final `.last-run.json` reports passed and no failed tests. |
| Impeccable detector | Pass: no findings | `.impeccable/review/automation-welcome/detector.json` is `[]`. |
| Independent visual review | Ship: 94/100 | `research/visual-review.md` and `research/visual-verdict.json`; all 18 required captures were inspected. |
| Diff whitespace | Pass | Scoped `git diff --check`. |

The component command was:

```sh
pnpm --filter @multica/views exec vitest run \
  autopilots/components/autopilots-page.test.tsx \
  autopilots/components/autopilot-template-catalog.test.tsx \
  autopilots/components/template-create-autopilot-page.test.tsx \
  locales/parity.test.ts
```

The final browser command loaded the existing task-owned `check.env`, validated
its database and live API identity, then ran:

```sh
pnpm exec playwright test \
  e2e/autopilot-template.spec.ts \
  e2e/autopilot-lifecycle-branches.spec.ts \
  e2e/progress-reporting.spec.ts \
  --project=chromium --workers=1 --retries=0 \
  --output=.impeccable/review/automation-welcome/final-browser-results \
  --reporter=list
```

## Browser and visual evidence

The production Web used the task-owned
`.multica/automation-welcome/web-runtime` build copy on port 13522, with the
isolated legacy-auth API on port 18602. `runtime-provenance.json` retains the
build ID, API process/deployment, allocated database name and source-file hashes;
it records only the database URL's hash, not its credential-bearing value.

Stable screenshot and metrics copies are under
`.impeccable/review/automation-welcome/`. Both locales cover 320, 390, 1024,
1440 and 2048px, a dark desktop, the bottom of the narrow template list,
genuine Tab/Shift+Tab focus, and a populated paused-automation list. Region and
document overflow measurements are false; page-error arrays are empty.

Real browser journeys verified blank creation, documentation, browsing all ten
templates, configuration without premature creation, native and in-flow Back
with restored card focus, and saved-automation lifecycle operations. Reporting
coverage uses deterministic fixture delivery rather than a commercial model.

## Continuation corrections

The first replay omitted the isolated database environment and failed at login
fixture preparation. Those results are preserved in `playwright.log` and are
not product acceptance.

After environment binding, the first two lifecycle cases exposed a selector
race: global New autopilot could select the loading management header's full
picker action. `openCreate()` now waits for the welcome region and clicks its
blank-creation action. The failed replay is retained in
`playwright-before-selector-fix.log` and `browser-before-selector-fix/`. All
original assertions, timeouts and retry settings remain intact; the complete
fresh replay then passed all 14 cases.

The preceding session corrected pointer-mode keyboard screenshots and a shared
`.next` overwrite through an isolated build and genuine keyboard recaptures.
No application styling change was needed for either evidence correction.

## Cleanup and limits

`cleanup.json` records shutdown of only the task-owned API and isolated Web
processes, with both ports verified closed. The allocated database and registry
record remain available under their existing TTL; unrelated services and
working-tree edits were preserved.

This acceptance covers shared component contracts and production Chromium.
Packaged Electron, native mobile, physical touch and spoken screen-reader
output were not independently exercised. No new product defect remains in the
approved scope.
