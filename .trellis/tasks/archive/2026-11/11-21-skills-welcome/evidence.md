# Skills Welcome Acceptance

Implemented and accepted the user-approved shared welcome state on 2026-11-21.
The feature is committed as `86b910dfa0fd7e5593739fc5defee2bf99831c35`.
On resumption, the final check reran all 474 Skills and locale tests, scoped
ESLint, and Views, Web and Desktop typechecks. All passed without source edits.
See [the resumed check](research/resume-check.md) for the commands and limits.

## Delivered Behavior

The empty Skills directory now presents the reference's quiet introduction,
one primary creation action, embedded documentation link and three manually
selected illustrative scenes. The scenes cover all five real creation methods,
explicit assignment and subsequent-run reuse. An adjacent faded card previews
the next scene on sufficiently wide containers.

Only successful defined raw empty Query data selects the welcome. Loading,
paused/unknown reads, failed reads, populated lists and filtered recovery retain
their own paths. Existing creation, local picking, template recovery and
virtualized list ownership remain in place. No backend, dependencies, stores,
persisted onboarding flag or mobile-native UI was added.

The old centered `EmptyState` was removed, the repeated header creation action
is hidden while the hero owns it, and the existing market entry uses native
`AppLink` semantics with the same shared button styling.

## Acceptance Mapping

| Criterion | Evidence |
| --- | --- |
| AC1: confirmed empty versus loading/error | Page regression suite covers successful empty, paused/idle unknown reads, initial/refresh failure and auxiliary-query failures |
| AC2: saved instances and data transitions | Page suite covers built-in-origin instances and filtered recovery; complete Web journey verifies live empty→populated→empty transitions |
| AC3: real actions and focus | Web and native journeys open/cancel the existing chooser and verify returned focus; Web follows workspace docs/market links and creates a real skill |
| AC4: truthful examples | Colocated bilingual tests, source review and visual review verify five methods, explicit assignment, future runs and snapshot limits; carousel/chooser cancellation produces no skill/agent writes |
| AC5: accessible manual carousel | Component/Web tests verify keyboard selection, all positions, disabled endpoints, hidden/inert scenes, announcements and workspace remount |
| AC6: themes and container sizes | Both locales/themes at 1440px and English 390px / Chinese 320px; no document or welcome overflow; native 720×700 content pane reaches controls through actual scrolling |
| AC7: checks and evidence | 474 tests, scoped lint, three package typechecks, isolated builds, complete browser/native journeys and independent code/visual review |

## Executed Checks

- `pnpm --filter @multica/views exec vitest run skills/ locales/parity.test.ts --reporter=dot`: **27 files, 474 tests passed**, independently repeated by Trellis check.
- Final focused welcome/page/create-dialog/locale run: **4 files, 116 tests passed**.
- Scoped ESLint and `git diff --check`: passed. Root E2E files are outside the
  existing workspace ESLint base paths; the lint claim applies to the scoped
  views components/tests.
- `@multica/views`, `@multica/web` and `@multica/desktop` typechecks passed,
  including Desktop node and renderer checks.
- Isolated `next build --webpack`: passed, including its TypeScript and static
  generation stages. Use `web-build-2-result.json` for final Web acceptance.
- Isolated `electron-vite build --outDir <task QA>/desktop-out`: passed for
  Main/preload/renderer compilation; this is not installer acceptance.
- Complete Web welcome journey: **1 passed in 8.5s**, one worker, zero retries.
- Existing skill-library accessibility journey: passed on the same unchanged
  production candidate, including keyboard selection/filter recovery.
- Complete built-renderer Electron welcome journey: **1 passed in 5.5s**, one
  worker, zero retries.
- Impeccable mechanical detector: **`[]`**, run once over the changed UI.
- Context manifests and final planning/acceptance structure validated.

## Independent Review

`research/check.md` records the full-scope Trellis review. It found no remaining
product, test-fixture or spec findings and independently checked retained command
results and tested source hashes.

`research/impeccable-finish-review.md` records the fresh shipped-role visual
review: **disposition `ship`, 94/100**, exceeding the 90-point threshold, with no
material fixes. The reviewer opened all 16 required captures and verified exact
dimensions/hashes. Approved deviations are the fifth actual creation method,
the existing shell, responsive stacking, dark themes and practical controls.

Visual state is retained at `.omx/state/skills-welcome/ralph-progress.json`;
required screenshots are in `.impeccable/review/skills-welcome/`. The primary
Chinese screenshot is `desktop.png`; `mobile.png` and `mobile-examples.png` show
the initial and scrolled 320px view. Native wide and narrow compositor captures
are retained separately.

## Runtime and Source Provenance

QA files live under `artifacts/qa/skills-welcome-20261121/`:

- `environment.json` binds the task-owned private registry, labelled synthetic
  PostgreSQL container, isolated Web candidate/build ID, actual API health and
  process identities. The tested API is the explicitly pinned owned process
  PID 17645 at commit `9113cda91`. The recorded `apiState: mismatch` is not a
  passing current-full-checkout registry reuse check; other concurrent work
  changed the checkout independently.
- `prebuild-source-sha256.json` and `accepted-source-sha256.json` prove the
  task's product target bytes did not change across build and acceptance.
- `web-build-2.log` and result JSON retain the passing isolated production Web
  build. `desktop-build.log` and the Desktop result in `build-result.json`
  retain the passing native compilation.
- `web-browser-confirmation*`, `desktop-browser-confirmation*` and
  `review-screenshots.json` retain final complete journeys and capture bindings.
- `check-*.log` contains independently executed static/unit checks.
- `cleanup.json` proves the owned Next process group, API and labelled test
  database container were stopped/removed and both allocated HTTP ports closed.
  The manual Next listener was verified by exact PID, cwd and port before stop.

Original failures are retained. The first environment attempt lacked `psql` on
PATH; the actual installed binary was found and a private task registry/env was
used. Candidate Web bootstrap needed the same workspace hoist configuration and
the app working directory. No dependency or product config change was needed.
Native test repair normalized the deliberately absolute Desktop docs URL and
settled transitions before compositor capture. The first Web welcome replay
reached live create/filter recovery before a fixture tried to JSON-decode a
successful empty DELETE response; the corrected complete replay asserts 204
and then executes all remaining welcome-return and real-creation assertions.

## Changed Files and Commit Boundary

The exact 12-path work/spec grouping is in `commit-plan.md`. New files are the
welcome component, CSS/declaration/test, two focused browser/native tests and
the executable Skills welcome spec. Modified files are `skills-page.tsx`, its
regression suite, the English/Chinese `skills.json` files and only the Skills
Welcome row in the shared spec index.

This archive retains the task plans, reference and review reports. Generated
QA/runtime evidence remains local. `commit-scope.json` and `resume-scope.json`
record the dirty paths before closure. The feature commit contains only the
Skills Welcome index row; the unrelated Triage Welcome row was already committed
when this task resumed.

## Limits

- The inherited API parser can normalize a malformed successful skill response
  to `[]`; this scoped UI cannot distinguish that from valid empty data.
- Native acceptance covers a real Electron window consuming the built renderer
  and preload through the existing fixture. Packaged Main, installers, updater,
  provider execution and native mobile UI were not exercised by this task.
- Existing broader Skills tests emit jsdom navigation/React act notices; pnpm
  reports pre-existing config deprecation and the Web CSS optimizer reports its
  existing custom-highlight notices. These checks all exited successfully.
