# Desktop and Gantt implementation review

Date: 2026-10-09. Role: dispatched Trellis implementer. Scope: DCF-05/06/07/09/10 and desktop DCF-08. The coordinator owns real Electron acceptance, spec updates, task state and commit selection.

## Change boundary and baseline

The checkout already contained candidate repairs. Review these candidates against the accepted plan, preserve working behavior, and add product changes only for a reproduced gap. The assigned boundary is the desktop tab/shell geometry owner, shared triage pagination, Gantt view and Gantt-specific tokens. There are no dependency, API, database, right-detail-sidebar, global Dialog or unrelated-task changes in this lane.

Before adding a regression, the four desktop suites passed 110 tests and the three shared view suites passed 79 tests. The starting-source snapshot belongs to the coordinator's `evidence/baseline.json` and `evidence/starting-scope.patch`.

## Retained candidate repairs

| Requirement | Review result | Evidence and remaining acceptance |
| --- | --- | --- |
| DCF-05 | Retain Gantt-only foreground tokens and solid neutral category fills. All status categories have a semantic foreground/background pair, including custom statuses resolved through category. | No global status colors changed. Unit tests cannot prove rendered contrast; the coordinator must measure all categories in both Electron themes. |
| DCF-06 | Retain localized Move left/right menu commands and Alt+Shift+Arrow shortcuts through the existing `moveTab` owner. Adjacent pin-state guards prevent boundary crossing, IDs preserve focus and active destination, and the store owns active-workspace grouping/persistence. | `tab-bar.accessibility.test.tsx` covers keyboard, menu, focus, disabled boundaries and persisted order. The store suite owns the clamp matrix. Real shortcut, overflowing-strip and drag acceptance remains with the coordinator. |
| DCF-07 | Retain wrapping pagination with shared `pe-chat-launcher` geometry. | Enabled Next navigates to offset 50 in `triage-page.test.tsx`; actual non-overlap at 900×700/native zoom 2 requires Electron rectangles and hit testing. |
| DCF-08 (desktop) | Retain exactly one `aria-current="page"` navigation destination. | The real tab primitive/store interaction test verifies selection changes without adding a second ARIA tab contract. |
| DCF-10 | Retain arithmetic horizontal date windows, full-range width and existing TanStack row virtualization with stable issue IDs and focused/context-menu row pinning. | `gantt-view.test.tsx` covers all three zooms over 2020–2030, first/last dates, 1,000 rows, sorting, Home/End, Tab/Shift+Tab across window bounds and resolved pending focus. Electron geometry/context-menu confirmation remains with the coordinator. |

## Confirmed DCF-09 gap and correction

The existing candidate subscribed to live media changes and switched Motion transition duration to zero. It handled reduced motion at launch and when toggling the sidebar after the preference change, but it did not interrupt an already-running spring if the geometry target remained unchanged.

Added a failing regression in `apps/desktop/src/renderer/src/components/desktop-layout.test.tsx`: collapse under ordinary motion, observe an intermediate frame, turn on reduced motion during that animation, and require the next frame's final header/drag/canvas values. Before the fix, header padding was `12.995332838664666px` instead of `184px`, proving the previous spring continued after the preference changed.

The local correction in `desktop-layout.tsx` uses existing Motion `useMotionValue`/`animate` through `useShellGeometry`. Initial values are the final mount geometry. Ordinary preference keeps the same spring parameters; reduced preference calls `jump(target)` to stop active animation and reset velocity. Cleanup stops superseded animation. Header padding and drag-region left use the same motion value, while canvas margin retains its existing 2px/8px geometry. Components remain mounted, including tab content and chat.

Verified the library contract against the official Motion motion-value reference and installed Motion DOM 12.38.0 source: `jump()` ends active animations and resets velocity. Reference: https://motion.dev/docs/react-motion-value#jump

The new regression passes after the correction and verifies that disabling reduced motion restores the ordinary spring for the next sidebar toggle. Existing at-launch/live preference tests still require final geometry on each of five frames in both directions. Real Electron acceptance should also switch the preference during an in-flight collapse, rather than only before the toggle.

## Changes made by this lane

- `apps/desktop/src/renderer/src/components/desktop-layout.tsx`: interruptible geometry owner; reuse one toolbar offset for padding and drag clearance.
- `apps/desktop/src/renderer/src/components/desktop-layout.test.tsx`: actual mid-animation preference regression and ordinary-motion restoration.
- This implementation record.

The tab bar, triage, Gantt, token and locale candidates were reviewed and retained without further edits. Existing dirty-file hunks and other agents' edits were preserved. No stage or commit operation was performed.

## Final verification

All final commands exited 0 against the corrected source:

```sh
pnpm --filter @multica/desktop exec vitest run src/renderer/src/components/tab-bar.test.tsx src/renderer/src/components/tab-bar.accessibility.test.tsx src/renderer/src/components/desktop-layout.test.tsx src/renderer/src/components/desktop-layout.workspace-gate.test.tsx src/renderer/src/stores/tab-store.test.ts
# 5 files, 116 tests passed

pnpm --filter @multica/views exec vitest run issues/components/gantt-view.test.tsx triage/triage-page.test.tsx locales/parity.test.ts
# 3 files, 79 tests passed

pnpm --filter @multica/desktop typecheck
pnpm --filter @multica/views typecheck

pnpm --filter @multica/desktop exec eslint src/renderer/src/components/tab-bar.tsx src/renderer/src/components/tab-bar.test.tsx src/renderer/src/components/tab-bar.accessibility.test.tsx src/renderer/src/components/desktop-layout.tsx src/renderer/src/components/desktop-layout.test.tsx
pnpm --filter @multica/views exec eslint issues/components/gantt-view.tsx issues/components/gantt-view.test.tsx triage/triage-page.tsx triage/triage-page.test.tsx
# Both scoped lint commands: no errors or warnings
```

The Impeccable edit hook reported no deterministic design-quality findings on the modified shell source/test. That is not visual approval. All pnpm invocations report the existing ignored `pnpm.onlyBuiltDependencies`/`pnpm.overrides` root configuration warning; this lane does not change package-manager configuration.

## Verification limits and handoff

This lane did not run Electron, take screenshots or use assistive technology. Rendered contrast, enabled-pagination overlap/native zoom, real keyboard/menu/drag focus and real large-range layout/DOM measurements remain coordinator acceptance work. No broad frame-time/jank or all-platform accessibility compliance claim follows from the bounded jsdom DOM tests. Dates beyond the ten-year fixture and very large physical browser scroll extents are not covered here.

When updating the reduced-motion spec, require an already-running animation to stop on a live preference change; setting `transition.duration = 0` alone is insufficient if the target does not change.
