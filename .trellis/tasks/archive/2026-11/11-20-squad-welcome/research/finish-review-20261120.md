# Squad welcome independent finish review — 2026-11-20

Verdict: **pass for the approved shared welcome-page scope**. No material acceptance blocker was found. This pass changed no product code and did not repeat builds, tests or browser journeys.

## Reviewed evidence

- Loaded the complete saved native Trellis context, task requirements/design/plan, the first review report and task-owned source/test changes.
- Compared the pinned reference with final Chinese 1440px, English 390px, dark Web, wide Electron and Electron 200% CTA/carousel captures under `artifacts/qa/squad-welcome-20261120/results/`. The initial `results-r1` captures were excluded from the final verdict.
- Inspected `playwright-results.json`: all three final journeys passed, with zero unexpected results, skips, flaky results or report errors. Build and final browser exit receipts are zero.
- Recomputed all 12 product/test/document source hashes from `summary.json`; every hash matches the current file. The production Web build ID and Desktop renderer index hash also match. The compiled Web CSS contains both corrected radius formulas, and final geometry records show the active card's computed radius is `18px` on Web and Electron.
- Reviewed the new squad welcome and runtime-token paragraphs in `.trellis/spec/views/frontend/component-guidelines.md`; their contracts match the actual source and canonical tests.

## Acceptance review

| Requirement | Finding |
| --- | --- |
| Successful-empty only | `SquadsPage` requires Query `isSuccess` and the unfiltered list to be empty. Error feedback has priority. Canonical tests cover pending, failed refresh, populated/filtered-empty and template URLs. The inherited malformed-success fallback remains explicitly qualified below. |
| Reference composition | The final view retains the neutral canvas, left introduction and creation/docs actions, light-blue delegation diagram, selected working member, idle member, rounded panels and partial next-slide preview. Sizing accommodates the existing shell and shared typography. English narrow and dark captures remain legible. |
| Creation and docs | The welcome supplies one `AppLink` to the existing chooser and one central `DOCS_SLUGS.squads` link. Canonical tests assert exactly one adapter navigation for keyboard and pointer activation; final Web and native journeys exercise the chooser, both method entries, template catalog and embedded documentation. |
| Manual examples | Only local state drives the three scenes. Named controls expose the pressed dot, respect both ends and update the status region. Inactive scenes carry both `aria-hidden` and `inert`. Tests establish same-workspace retention and new-workspace reset. Source contains no auto-advance timer or agent execution. |
| Responsive and enlarged text | Final Web and native geometry records have page widths equal to their viewport widths. At native 200%, the CTA is within the viewport with a 44px height, and the active carousel/control remains reachable after scrolling. The final captures show the actual painted scroll positions. |
| Verification and spec sync | The first reviewer recorded 93 Views, 5 Docs and 28 Core passing tests, scoped lint and final Views typecheck. Main's final browser run and production builds pass. The spec update records empty-state priority, navigation ownership, inert/manual scenes, remount behavior and the runtime-radius gotcha without broadening the task. |

## Findings fixed in this pass

None. The radius correction and test strengthening from the first review are present in the current source and final built artifacts.

## Remaining limits

- `ApiClient.listSquads()` converts a malformed successful HTTP payload into `EMPTY_SQUAD_LIST`. The view cannot distinguish this existing fallback from valid `[]`. Design, specification and review evidence correctly state this limitation; API/schema changes are outside the approved scope.
- Native evidence uses the built Electron renderer, production routing/preload functions and the isolated fixture. It does not establish packaged production Main, installer/updater or real-model execution acceptance. Embedded docs navigation is verified against the local API, not a disconnected-client offline mode.

No further product changes or cosmetic iterations are recommended before the main session completes task-owned commit, resource cleanup and lifecycle recording.
