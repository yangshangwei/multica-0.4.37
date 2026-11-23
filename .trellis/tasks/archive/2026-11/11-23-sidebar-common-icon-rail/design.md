# Sidebar common icon rail design

## Reviewed direction

The user selected A after reviewing the interactive alternatives and explicitly
requested a task and implementation. Preserve that reviewed composition: a
roughly 64px rail of common actions on large Web/Desktop surfaces, with secondary
destinations in More. Keep the work uncommitted.

## Behavior boundary

The missing behavior lives in shared `AppSidebar` presentation and the shared
`Sidebar` primitive's collapse geometry. Desktop currently assumes every
collapsed sidebar occupies zero width, so its tab/header and canvas offsets
must distinguish an icon rail from a compact sheet.

Expected files are shared layout components/tests/locales, the sidebar
primitive, Desktop layout/tests, targeted E2E coverage and the existing global
scope navigation spec. No server, store, schema or dependency changes are
required. Preserve all pre-existing changes in the checkout.

## Shared navigation

- Use the existing `Sidebar` icon collapse mode for large screens. Derive the
  presentation from both collapsed state and `isCompact`; a compact Sheet
  always renders the full navigation even when the parked large-screen state
  is collapsed.
- Reuse existing action handlers, native `AppLink` destinations, route icons,
  capability gates and scope rules. Common workspace destinations must render
  independently of the expanded navigation groups' manual preferences.
- Keep one authoritative notification observer and the actual-workspace chat
  observer. Do not fetch workspace data or install workspace identity for
  account previews. Preserve existing active-workspace query ownership.
- More uses existing Base UI dropdown primitives, with localized group labels
  and native links. Preserve accessible names and current-destination feedback;
  use short group/submenus where the directory is long. Positive unread counts
  remain visible in icon mode; unknown counts are never presented as zero.
- Keep complete expanded directories and pinned items intact. Icon mode is a
  presentation choice, not a new persisted preference or directory data model.
- Collaboration creation uses its existing dialog and source selection. If its
  action moves into More, return focus to the visible More trigger after close.
  Ordinary creation retains its existing target-bound account handoff.
- Brand text, search captions and full footer text are hidden only in icon
  mode. The web rail retains a reachable expand/collapse trigger. Desktop uses
  its existing external toolbar trigger. Help/account remain reachable.

## Geometry and focus

The inset primitive already uses a 48px icon width plus 16px shell padding.
Align painted and reserved collapsed widths if necessary; do not overwrite the
remembered expanded width while the resize rail is collapsed. Preserve the
existing 1024–1279 automatic-collapse band and SSR compact width gate.

Desktop retains a 200px top-toolbar region and 92px traffic-light clearance.
With a 64px rail, its content header needs the remaining 136px clearance;
compact layouts still reserve the full 200px. Canvas spacing must follow the
real rail instead of zero-width offcanvas assumptions. Keep interactive
controls no-drag and the top sidebar spacer draggable.

Menus close on route/context/presentation changes, cannot leave focus inside
hidden content, and Escape returns to the visible trigger. Use Base UI's native
menu lifecycle rather than a custom popup controller. Respect reduced motion,
semantic surface/type tokens and 44px coarse-pointer controls.

## Trade-offs and compatibility

Common entries remain one click away; secondary entries may require a menu
selection. The expanded layout, routes, scope histories and capability switches
remain the authority. This is shared Web/Desktop UI and needs no backend or
installed-client response migration. B/C and a third fully hidden state are
excluded. Reverting the scoped diff restores the original offcanvas behavior.

## Verification

Shared tests own common-entry rendering, exact links/actions, truthful unread,
capability gates and compact presentation. Primitive tests own widths/resize;
Desktop tests own toolbar/tab/canvas geometry. Production Web and actual
Electron renderer journeys prove the large-screen rail, overflow menus, native
focus and compact sheet behavior; retain the original restoration journey.
Capture light/dark and narrow/large screenshots and compare with the reviewed A
preview and current product surfaces. Native harness acceptance does not claim
installer or packaged Main compatibility.

## Authorized delivery

The initial no-commit instruction governed implementation and acceptance. After
reviewing the completed result, the user explicitly requested local commits.
Deliver the verified work, archive this task and record the journal; do not push
or include unrelated changes.
