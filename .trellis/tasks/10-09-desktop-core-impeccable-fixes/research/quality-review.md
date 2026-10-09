# Independent quality review

Date: 2026-10-09. Role: dispatched Trellis check agent. Reviewed the accepted PRD, design, implementation plan, check manifest, injected specifications, research, candidate repairs and final lane corrections, including the native header-removal focus follow-up. No product files, staging state or commits were changed by this reviewer. One mechanical signature correction was made to the new code-spec.

## Findings (fixed)

No additional product fix was required from this reviewer. The new `desktop-core-accessibility.md` described `useLinkHover(editor, disabled = false)`, while the implementation accepts `containerRef: React.RefObject<HTMLElement | null>` and optional `disabled?: boolean`. Corrected only that signature and its explanatory wrapping.

The active accessibility lane resolved the intermediate direct-composer-blur ownership gap during review. A visible composer can blur directly to body without a subsequent focusin event; retaining its last focus record would incorrectly restore the opener when chat later closes. Final `use-chat-input-focus.ts` observes both focusin and focusout, clears departed ownership, and preserves it while the root is inert so native close-time blur still permits legitimate focus return. The reviewer independently ran the direct-blur regression against the earlier source: one test passed, ten skipped. The lane's final hook/window run covers the remaining transitions, including inert blur.

Native Electron then exposed a different ordering: removing the focused Minimize button emits focusout while the button is still connected and its retained parent is not yet inert (`evidence/chat-focus-native-red.json`). The subsequent lane correction retains that blur target in a one-commit token. Closing layout accepts it only if the target has become disconnected and focus is on body; a still-connected explicit composer blur is rejected. A microtask clears the candidate, preventing a header blur from an earlier interaction from claiming later body focus. React focus capture and unrelated document focusin clear the token, and the microtask compares object identity so it cannot clear a newer token. Closing clears all handoff refs. The token performs no state update or delayed focus and does not survive an unmount as an active document listener.

The exact native FAB Enter → Minimize sequence now returns focus to the launcher (`evidence/chat-focus-native-green.json`, Electron 39.8.7). The reviewer separately executed the final canonical 13-case hook matrix with Testing Library's `reactStrictMode: true`: **13/13 passed**, covering initial persisted-open behavior, readiness/reopening, outside focus, direct body blur, inert blur, header removal and an earlier header blur. A temporary copy configured the existing matrix under StrictMode and was verified/deleted after the run; no duplicate canonical suite remains. This evidence supersedes the earlier direct-blur-only check for final source.

## Findings (not fixed)

No unresolved P0/P1/P2 product defect was found in the inspected task scope.

Two existing `project-detail.tsx` exhaustive-deps warnings remain at lines 152 and 257. These predate this task's new edits and are outside the bounded repair; the scoped lint command exits 0 with no errors. The root package-manager configuration warning is also existing and was not changed.

## Behavior and boundary review

- **Query recovery:** Inbox uses the active collection's defined data rather than a nonempty array as its cache signal. Failed/undefined reads cannot trigger the shared-link fallback. Active/archive cached failures retain the list and keyed detail, including compact-detail retry. Projects keep their existing protected request wrapper: workspace denial is rendered before ordinary recovery and protected cache is removed. Detail 404 is distinct from retryable 500. Retry keeps query keys, filters, selection and editor identity. The new real QueryClient project tests verify the production list projection, workspace/AbortSignal options, delayed retry and real revocation.
- **Project identity and selection:** The title's AppLink remains a direct keyboard destination. Consumer click/auxclick isolation runs before adapter navigation without cancelling the native/adapter behavior. Selection isolation is on the parent cell, covering Base UI's sibling input click. Row pointer shortcuts, inline controls and desktop/web modifier semantics remain intact.
- **Desktop navigation and reorder:** Current navigation uses aria-current without claiming an incomplete ARIA tab contract. Menu and Alt+Shift+Arrow commands share the existing moveTab owner. Adjacent pin-state checks prevent segment crossing; the store supplies active-workspace and persistence invariants. Focus restoration uses the moved ID and only scrolls the strip, preserving the active destination.
- **Reduced motion:** useShellGeometry initializes at the final target. A live reduced-motion change calls motionValue.jump(target), which stops an existing spring even when the target is unchanged. Header padding and drag clearance share the same value. Ordinary motion retains the established spring; cleanup stops superseded animations. The added regression verifies an intermediate ordinary-motion frame, snapping after a preference change, and later restoration.
- **Gantt:** UTC-day geometry and the full timeline width are preserved. Arithmetic viewport bounds allocate only the exposed day indices plus overscan; month headers also iterate the visible range. TanStack Virtual supplies fixed-height rows with stable issue IDs and a pinned focus/context target. Pending keyboard focus is resolved immediately or after the requested row mounts; Home/End and Tab/Shift+Tab can cross virtual boundaries. Semantic category colors include custom statuses and solid neutral fills; titles inherit their dedicated theme foreground rather than fixed white.
- **Pagination:** The existing launcher clearance token owns the end padding. Flex wrapping supplies narrow layouts without arbitrary bottom padding or route changes. The enabled-Next regression preserves offset navigation.
- **Chat/editor visibility:** The same visible condition governs inert/aria-hidden, initial focus and transient controls. Editor/session/draft/upload ownership remains mounted. ContentEditor's optional isVisible defaults to true and changes only BubbleMenu/LinkHoverCard visibility, retaining its document and upload callbacks. useLinkHover immediately masks the portal and resets stored hover state; listeners/timers clean up on disable. All hooks remain above the editor-null return. Header, add/project pickers and queue menus are gated without replacing their persistent owners.
- **List/detail accessibility:** Agent compact header, rows and skeletons consistently include the selection track. Select-all now describes the listed range and the filtered-selection regression checks that excluded rows stay unselected. Issue/project/agent rows use one named primitive with keyboard focus visibility. Agent and skill detail navigation reuse the existing Tabs wrapper, manual activation, local orientation and associated panels, while preserving URL query/hash and dirty-navigation confirmation. Panels keep semantic identities; inactive content does not introduce a second accessible interaction tree.

## Verification

Checks executed by this reviewer:

| Check | Result | Evidence |
| --- | --- | --- |
| Scoped Inbox/Projects/Gantt/Triage source and test ESLint | Pass, exit 0; 0 errors, 2 existing project-detail Hook warnings | `evidence/quality-review-views-lint.log` |
| Scoped desktop shell/tab source and test ESLint | Pass, exit 0; no lint findings | `evidence/quality-review-desktop-lint.log` |
| Scoped final chat/editor/agent/skill source and test ESLint | Pass, exit 0; no lint findings | `evidence/quality-review-a11y-lint.log` |
| Direct body-blur focus regression | Pass, 1 passed / 10 skipped | `evidence/quality-review-focus-blur.log` |
| Final 13-case hook matrix under StrictMode | Pass, 13 passed | `evidence/quality-review-focus-strict.log` |
| Final header-removal hook/source test ESLint | Pass, exit 0; no lint findings | `evidence/quality-review-header-removal-lint.log` |
| Scoped tracked-file git diff --check, finished lanes and final accessibility lane | Pass, exit 0; no output | Commands executed during review |

Reviewed other lanes' verification records without duplicating their suites: recovery 38 tests passed; desktop 116 tests passed; Gantt/Triage/parity 79 tests passed; accessibility 302 tests passed with subsequent focus corrections verified separately. Final coordinator logs now record **406 views tests in 19 files** (the earlier 404 plus the two header-ordering cases), **111 desktop tests in 4 files**, and **9/9 workspace typecheck tasks**, with six cached and three executed. These counts describe different overlapping runs and must not be added together. Per dispatch, this reviewer did not repeat workspace typecheck; the coordinator owns freshness of its last incremental typecheck after the header correction.

## Acceptance and spec handoff

The reviewed native red/green records close the header-removal focus regression. Other native claims remain tied to the coordinator's batch evidence; this reviewer does not independently reclassify screenshots or assume coverage from jsdom. Keep these assertions explicit in the final evidence/report:

1. Closed chat and former portals are absent from native keyboard traversal and the accessibility tree. Open/close and popup dismissal restore focus only while chat owns it; a page action or explicit composer blur must remain undisturbed. Draft and upload state survive reopening.
2. Cold/cached HTTP 500 and successful Retry preserve URL/content/selection/editing; actual permission loss removes protected project content. Genuine 404 remains distinct.
3. Keyboard-focused selection/title/tab controls have computed visible focus treatment, including the compact agent list and narrow nested-tab orientation. Desktop menu/shortcut reorder preserves focused-tab visibility, destination and pin/workspace boundaries; pointer dragging still works.
4. Measure every rendered Gantt category in both themes, including neutral/custom category resolution and effective backgrounds. Require ordinary title contrast of at least 4.5:1; light success has a small margin. Bounded DOM is not a frame-time or all-date-range performance claim.
5. With enabled pagination, test 900×700 and native zoom 2 rectangles and real hit targeting. Verify no horizontal overflow and both retained locales.
6. Switch reduced motion during an already-running shell collapse and inspect subsequent geometry frames. Scroll the ten-year/large-row Gantt to all endpoints at every zoom and traverse virtual boundaries with the real browser's scrolling/focus behavior.

The coordinator should capture the executable contracts in the applicable specs: retained nonmodal content must gate its portalled overlays; focus ownership must distinguish direct visible blur from inert-induced close blur; compact list selection must remain available with matching header/skeleton geometry; cached read errors retain authorized editing while terminal access loss wins; reduced-motion preference changes must interrupt an active geometry spring. Preserve the original time-specific audit and retain the already documented ten-year/assistive-technology coverage limits.
