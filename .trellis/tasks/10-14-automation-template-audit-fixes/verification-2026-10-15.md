# Final native acceptance

A01–A07 are accepted. Independent visual review scored the final screenshots
**96/100, pass**. The final production Electron run passes all 16 layout cases,
8 return paths, 8 focus-owner checks and 4 touch-entry cases. No remaining
defect was found within this task's scope.

## Additional repair found during acceptance

The first fresh native run exposed an A02 timing gap after implementation commit
`e5fe7b909`: Desktop published the destination URL before replacing the outgoing
configuration page. Its temporary picker consumed the destination's focus
record, then React deleted the focused card. An immediate focus capture missed
the loss; the settled destination had focus on BODY.

The final patch changes only the template page and its canonical test file.
A nine-line render guard leaves destination state alone when the page's route
is departing. It preserves accepted trailing-slash/case aliases and keeps all
hooks unconditional. The test separates navigation publication from route
mounting, verifies the pending token through the intermediate render, then
checks the final card, category and 142px scroll. This regression failed before
the guard and passed afterward.

The existing gallery, restoration adapter and navigation contracts remain the
implementation boundary. No new store, delayed focus callback, dependency or
global navigation change was needed. The discovery spec now documents the
staged-route invariant and requires settled, connected-node focus evidence.

## Acceptance evidence

| Requirement | Final evidence |
| --- | --- |
| A01 — visible focus | Minimum card/canvas ring contrast is **19.23:1 light**, **16.97:1 dark**. Intro text measures **5.71:1 / 7.19:1**. Actual keyboard focus is visible in both themes. |
| A02 — return continuity | Both origins × Chinese/English × in-flow/native keyboard Back pass: **8/8**. Category, exact scroll and original card focus persist after the real route commits; the same node remains connected and focused. |
| A02 — newer focus owners | **4/4** native toolbar cases retain deliberate toolbar focus; **4/4** refetch cases retain the newer owner. Pending gallery focus is consumed once. Creation success/failure lifetime remains covered by canonical tests. |
| A03 — recommendation | The label sits with the title. In the Chinese wide layout, the first row is 196px and later rows 194px; the former dedicated recommendation footer row is gone. English copy wraps without clipping. |
| A04 / A06 — guidance | Executor, timezone, default schedule and editability information appears before filters/cards in both languages. The duplicate footer explanation is removed. |
| A05 — truthful output | The rendered labels distinguish conditional issue creation/update from creating an issue each run. Existing conservative unknown-template behavior is retained. |
| A07 — blank creation | Both origins at 1771px and 390px with coarse input have labelled **44px** blank/filter targets. Synthetic touch filters to four cards, opens the real blank dialog and returns focus on dismissal. Dialog screenshots were captured after opening animations completed. |

The final layout matrix is Chinese/English × light/dark × 1771×1080,
886×720, 390×844, and native 200% zoom at 1771×1080: **16/16 pass**.
The grid retains three/two/one columns and has no horizontal or card overflow.
`webContents.setZoomFactor(2)` supplies the zoom evidence; it is not a renamed
small viewport.

## Quality checks

- Final scoped views run: **6 files, 123 tests passed**. The focused page/catalog
  run also passed **41 tests**; these counts overlap and are not additive.
- Unchanged Web restoration, Desktop coordinator and core schema suites passed
  **6, 6 and 8 tests** respectively during the full-scope review.
- Scoped ESLint and typechecks passed for views, Web and Desktop. After the
  additional page repair, its ESLint and views typecheck passed again.
- The post-fix impeccable detector returned **[]** for the three affected
  automation views. `git diff --check` and the UI wildcard-export check passed.
- Independent review approved the original implementation, the additional
  repair, its staged regression and the final native evidence.

The bounded post-change audit rates accessibility **3/4**, performance **3/4**,
theming **4/4**, responsive behavior **4/4**, and implementation integrity
**4/4**: **18/20**. There are no remaining P0–P3 findings in the approved scope.
The accessibility/performance scores retain the limits below rather than
asserting full screen-reader or production-performance certification.

## Build and runtime provenance

The final renderer and real preload were built from base
`d8228f82509a75af0bcb7fe9c389c9e6ab58a36a` plus the recorded source patch.
Its SHA-256 is
`7e9bfc7d0874908cf316052f7c097ebf94937710824c11c38b83c50877c37040`.
All **14 recorded source hashes** and **15 served artifact hashes** match the
reviewed source/build. The manifest covers all 495 generated artifacts.

The fixture used Electron **39.8.7**, Chromium **142.0.7444.265**, a temporary
profile and built assets served at `http://localhost:13492`. Sandbox, context
isolation and web security were enabled; the API explicitly allowed that origin.
The existing read-only local API at `http://localhost:18572` reported commit
`bb2d9f481`; this acceptance does not claim a rebuilt backend.

Final counters: page errors **0**, failed requests **0**, HTTP error responses
**0**, daemon starts **0**, installs **0**, external links **0**, blocked writes
**0**. No creation, enablement or agent execution was performed against the API.
The temporary window, servers, profile and both temporary builds were removed.
The existing human-owned API/desktop listeners retain their original PIDs.

## Evidence index and limits

- [Final matrix and measured geometry](evidence/final-native/matrix.json)
- [Return, focus-owner and touch results](evidence/final-native/interactions.json)
- [Independent visual verdict](research/visual-verdict-final-2026-10-15.json)
- [Full-scope code review and exact commands](research/final-check-2026-10-15.md)
- [Pre-fix route-order diagnosis](research/native-focus-regression-2026-10-15.md)
- [Build provenance](evidence/final-native/build-provenance.json)
- [Final runtime](evidence/final-native/runtime-final.json)
- [Cleanup](evidence/final-native/cleanup.json)

Verification covers the shared UI in native macOS Electron with synthesized
Chromium touch. Physical touch devices, native Windows/Linux, independent Expo
UI, spoken screen-reader output, production performance, real automation
execution and production deployment were outside this acceptance scope.
