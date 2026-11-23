# Native pointer verification — 2026-11-19

## Result
The reported physical-mouse X failure is repaired by excluding the resource popup from Electron native window dragging. The earlier hidden-lifecycle repair remains necessary and unchanged.

## Before / after with actual OS pointer input
Environment: existing Multica Canary work-management-local Electron from this checkout, localhost:5666. Captured app screenshot was 1442x768. Original issue: the retained 11-06 boundary-validation sample.

- Before this change, CUA `multica.click([1420,20])` on the visible top-right X left the populated Activity drawer and blur/backdrop present. The cursor screenshot confirmed the click position.
- After the no-drag boundary, the same coordinate click closed the Details drawer with the task page visible.
- Opened Activity with OS coordinate `[875,225]`, then clicked X at `[1420,20]`: entire drawer and backdrop disappeared; AX returned Activity opener focus and screenshot showed the unobscured task page.
- Repeated Activity open/X using the same coordinates: drawer/backdrop again disappeared, confirmed by screenshot.
- Reopened and pressed Escape; then reopened and clicked backdrop at `[1000,400]`: dismissal still works. Final screenshot was the original task page without a modal or backdrop.

All X acceptance actions in this pass used screen-coordinate clicks, not accessibility-element activation, DOM click or renderer injection. The live UI was re-observed after CUA timeouts/user navigation. No business writes/submissions were performed.

## Automated checks
- New native CSS contract assertion RED before source change: expected undefined to be no-drag.
- Auxiliary and detail suites: 21 passed.
- Views typecheck: passed (exit 0).
- ESLint for both changed TypeScript files and scoped git diff --check: passed.
- Independent Trellis check: no unresolved findings; independently repeated scoped lint/diff checks.

## Limits
OS pointer acceptance covers the current macOS Electron development client. Full E2E, packaged release, Windows/Linux and alternate zoom levels were not run. Component tests guard the CSS boundary, lifecycle and draft retention; they do not claim native hit-testing coverage.

## Source hashes
- `packages/views/collaboration/detail-auxiliary.tsx`: `c31651afa781c2700a55987471b3595e4c9bb5067d5ac553a92c7f141e98ca5a`
- `packages/views/collaboration/detail-auxiliary.test.tsx`: `07c0ead693230c3d58f5439f32730bfbf37a5343ea705810c3ae85fbb6cae3fd`
- `.trellis/spec/views/frontend/collaboration-detail.md`: `3a52a3b83ab55fc55edbb4672ddac818bb3e11ade08fca11754229c86a518ab1`
- `.trellis/tasks/archive/2026-11/11-19-collaboration-drawer-dismissal/research/verification.md`: `57306375e8511595102078bc7eda4945815617742150186cdb03c0a72051be12`
