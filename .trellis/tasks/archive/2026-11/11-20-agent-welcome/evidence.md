# Agent welcome acceptance

Completed the approved shared empty-state scope on 2026-11-21. The page presents
the reference introduction, creation/docs links, runtime checklist and three
manual workflow examples. Successful raw empty data selects the welcome;
pending/error and populated/filtered/archived-only states retain their own paths.

## Acceptance mapping

| Criterion | Evidence |
| --- | --- |
| AC1: successful-empty query, independent auxiliary reads | `agents-page.test.tsx`: pending/paused reads, cached empty/populated refresh failures and existing list-readiness regressions |
| AC2: workspace routes and factual runtime readiness | `agent-welcome.test.tsx` and real browser creation/docs/runtime navigation; successful-online/offline/unknown unit matrix and runtime observation in E2E |
| AC3: manual accessible examples | Keyboard Enter, position controls, endpoint disabling, inert/hidden scenes, selected buttons and live status in component/browser tests |
| AC4: data-driven arrival and archive semantics | Unit empty→populated→empty and workspace-remount regressions; real API creation/WebSocket arrival and archive retaining the directory |
| AC5: themes, locales, responsive layout and regressions | Both locales × both themes × 1440×900 / 1024×768 / 390×844 / 320×568; 134 focused tests, existing directory browser journey, three package typechecks and scoped lint |
| AC6: actual screenshot/reference inspection | `.omx/state/agent-welcome/user-reference.png`, baseline and corrected production screenshots, structured 84→94 verdict and computed radius measurements |

## Checks

- Trellis independent full-scope check: 6 Vitest files, 134 tests passed. The
  selected files are `agents-page.test.tsx`, `agent-welcome.test.tsx`,
  `agents-page-filter-predicate.test.ts`,
  `agents-page-bulk-access-dialog-state.test.ts`,
  `agents-page-access-cell.test.tsx` and `locales/parity.test.ts`.
- Scoped ESLint passed for the page, welcome and colocated DOM tests.
- Final `@multica/views`, `@multica/web` and `@multica/desktop` typechecks passed.
- Isolated `pnpm exec next build --webpack` passed after the single CSS fix.
- Existing `e2e/agents-discovery.spec.ts` passed on that corrected production
  build. The welcome replay passed in 8.4 seconds with one worker and zero retries.
- The mechanical Impeccable detector returned `[]` for the welcome TSX/CSS.
- Scoped `git diff --check` and task context validation passed.

## Source and browser evidence

All QA files are under `artifacts/qa/agent-welcome-20261120/`:

- `environment.json` preserves the original allocation. Its labelled synthetic
  PostgreSQL container, task-owned API health PID/commit and private production
  Web candidate were verified before reuse.
- `baseline-environment.json` and `baseline-build.log` bind a fresh build to the
  resumed source hashes. The prior build's welcome hash had drifted, so the
  prior production bundle was not used for current-source acceptance.
- `baseline-playwright/` and `baseline-playwright.log` retain the failing radius
  regression: cards/checklist computed to `0px` in all 16 layout combinations.
- `final-environment.json`, `final-build.log` and `final-playwright/` retain the
  corrected build, initial functional replay and all final visual captures.
  Computed radii are now `18px` for cards and `14px` for the checklist, with no
  document or welcome-container horizontal overflow. Primary creation links
  are visible, and narrow carousel controls are reachable after scrolling.
- The initial functional replay reached the real new-agent list, then failed
  because an accessible-name locator matched both its avatar profile link and
  agent-name navigation link. The corrected test selects the exact agent href;
  it does not change production UI behavior.
- `confirmation-environment.json` pins the corrected test and unchanged product
  source hashes to the same production build. `confirmation-playwright.log`
  and `confirmation-playwright/` retain the full passing welcome replay.
- `cleanup.json` records stopping only the owned API/Web process groups,
  verifying both listeners closed and removing only the labelled test database
  container. QA logs, builds and screenshots remain available.

The reference and verdict history are in `.omx/state/agent-welcome/`. The initial
visual score was 84; the corrected production render scored 94, passing the
90-point threshold. No further visual edits followed confirmation.

## Scope and limits

The generic empty branch and its old localized copy were replaced; the hero
owns one creation action, so the header does not duplicate it. Routing uses
existing shared adapters and primitives. No dependencies, stores, persisted
onboarding flags, backend endpoints or native mobile surfaces were added.

`ApiClient.listAgents()` already normalizes a malformed successful response to
`[]`; the view cannot distinguish that inherited fallback from a valid empty
list. This limit is documented in the agent-discovery spec and was not changed.
Actual empty restoration is covered in the shared page suite; the browser uses
the supported archive endpoint, which correctly keeps a directory.

Browser acceptance covers production Web. Desktop consumption passed typecheck;
an actual Electron window, installers, real-model execution and the full
repository `make check` pipeline were not exercised by this focused UI task.
