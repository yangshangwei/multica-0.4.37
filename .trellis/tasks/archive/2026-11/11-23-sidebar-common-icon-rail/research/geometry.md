# Sidebar and Desktop geometry findings

Scope: read-only source/spec/test inspection. Only this research file was written. No product code or existing edits were changed; no tests were run for this research slice.

## Current contracts

- `packages/ui/components/ui/sidebar.tsx:29`: expanded width defaults to 256px, clamps to 200–360px and persists as `sidebar_width`. Collapsed/open state deliberately does not persist because viewport-driven collapse must not become a lasting user preference.
- `packages/ui/components/ui/sidebar.tsx:39`: 1024–1279px auto-collapses on entry, parks the previous open state, permits manual reopening within the band, and restores the parked state when leaving. Preserve this transition logic.
- `packages/ui/components/ui/sidebar.tsx:317`: below 1024px, `Sidebar` renders a full 18rem Sheet controlled by independent `openMobile`; desktop `open` may still be parked collapsed. `useSidebar().state` alone must never select the icon-only business branch.
- `packages/ui/components/ui/sidebar.tsx:349`: desktop root/container are CSS-gated by `lg:block` / `lg:flex`. `useIsCompact` initially resolves false until its effect, so these CSS gates prevent a full desktop sidebar flash at 768–1023px. Preserve `lg`, not `md`.
- `packages/ui/components/ui/sidebar.tsx:364`: inset icon gap already reserves 64px: 3rem icon width plus 16px inset padding. Container currently paints 66px (`+2px`). Use one explicit shared collapsed inset width contract and align gap/container intentionally; do not independently guess 64px in Desktop.
- `packages/ui/components/ui/sidebar.tsx:576`: Web `SidebarInset` adds 8px left margin for every collapsed state. That rule was written for zero-width offcanvas. Distinguish offcanvas from persistent icon mode if the desired canvas begins immediately beside the rail.
- `apps/desktop/src/renderer/src/components/desktop-layout.tsx:92`: native toolbar remains at window x=0, width 200px, height 48px, with 92px traffic-light clearance. Controls declare `no-drag`; its own trigger is always available. `SidebarTopSpacer` begins beyond x=200 because native drag hit testing ignores visual z-index.
- `apps/desktop/src/renderer/src/components/desktop-layout.tsx:165`: Desktop currently treats `state === "collapsed" || isCompact` as no in-flow sidebar. It gives the main header 200px left padding and the canvas 8px left margin. That assumption becomes wrong for an icon rail.
- `packages/views/layout/page-header.tsx:47`: Web fallback trigger is `xl:hidden`; Desktop suppresses it with `hasExternalTrigger`. At >=1280px, a rail must retain a visible expand button itself or collapse can strand the user.

## Exact suggested geometry

With a shared 64px inset icon rail:

| Surface | In-flow sidebar | Header padding / drag left | Canvas left margin |
| --- | ---: | ---: | ---: |
| Desktop expanded | Stored 200–360px | 0px | 2px |
| Desktop icon-collapsed, >=1024px | 64px | 136px (`200 - 64`) | 2px |
| Desktop compact Sheet, <1024px | 0px | 200px | 8px |

Header padding and its absolute drag region must continue sharing the same Motion value. Keep the live reduced-motion subscription and `jump()` cancellation; changing an animation duration does not cancel an in-flight spring.

The native top spacer should retain its 48px height and x=200 drag exclusion in expanded and compact surfaces. A 64px rail cannot use a 200px left margin as an intrinsic min-width that grows the rail; ensure that spacer has `min-w-0` or a collapsed-specific non-draggable empty spacer, while keeping native controls unobstructed.

## State / context distinction

- Existing fields `state`, `isCompact`, `openMobile`, `hasExternalTrigger` already express the needed display distinction. Derive `iconCollapsed = state === "collapsed" && !isCompact` where `AppSidebar` is guaranteed to use icon folding. Do not introduce another persisted state or rewrite the current viewport state machine.
- If geometry is exposed by `SidebarContext`, expose one shared numeric collapsed inset width (or export the shared width from the primitive) rather than a second independently maintained icon-collapse Boolean. The provider currently does not know a child's `collapsible` variant; blindly publishing `state === "collapsed"` as an icon-mode field would mislabel offcanvas consumers.
- Desktop `MainTopBar`: `isCompact ? 200 : state === "collapsed" ? 200 - collapsedInsetWidth : 0`.
- Desktop `MainCanvas`: `isCompact ? 8 : 2` once its sidebar is guaranteed to preserve icons on all desktop collapses.
- Web rail toggle remains accessible on desktop, including >=1280px; suppress duplicate content-header fallback on lg+ when the rail supplies it, while preserving the Sheet opener below 1024px. Desktop retains its one external toolbar trigger.
- Any initial/SSR class that prevents icon-mode text/layout flash must use the same >=1024px desktop gate. Do not infer mobile/full navigation solely from a post-effect open state; `Sidebar` has already chosen a Sheet versus in-flow branch.
- The 1024–1279px auto-collapse itself is currently effect-driven, so the first render can be expanded there. Keep that existing behavior unless a new CSS startup rule is justified; adding a blanket media-query collapse would fight a manual reopen in the band.

## Resize, focus and overflow risks

- `SidebarRail.onPointerDown` currently starts a resize regardless of collapsed state. A drag from a 64/66px rail clamps its starting width to 200px and can persist an unrelated expanded width. Either hide the resize rail in icon mode or guard drag startup with actual expanded noncompact state. A collapsed click may continue to reopen if desired, but must not write `sidebar_width`.
- Active drag cleanup currently handles pointercancel, lost capture, blur and unmount. If collapse or compact breakpoint changes happen mid-drag, cancel the live gap/container inline widths before the icon/Sheet layout is applied, without committing the preview.
- Existing group-label icon CSS uses negative margin and opacity=0, which does not remove a `CollapsibleTrigger` from keyboard/AX reachability. Full navigation controls hidden in icon mode must be unmounted, `hidden`, or inert rather than opacity-only.
- `SidebarContent` forces `overflow-hidden` in icon mode. With common actions plus footer, short desktop heights or coarse targets can clip navigation. Allow purposeful vertical scrolling in the appropriate rail body or prove all required controls fit; retain footer reachability.
- `SidebarMenuButton` has forced 32px icon size, and inset groups/header add further padding. Check actual 44px effective coarse targets and icon alignment; a 44px button cannot be placed in a 32/34px nested content column without adjusting compact branch padding.
- `SidebarMenuButton.tooltip` already supports icon-only right-side tooltips gated by `state` and `isCompact`. Base UI tooltip content uses `data-slot="tooltip-content"` rather than default `role="tooltip"`; keyboard visibility must be tested using Tab, not synthetic focus after pointer input.
- More/workspace/account popovers must close on display-mode transitions; focus must move to a remaining visible trigger, and a collapsed or replaced full-nav trigger must not retain an open portal.

## Smallest relevant file/test scope

1. `packages/ui/components/ui/sidebar.tsx`: shared collapsed inset geometry, optional offcanvas-versus-icon Web inset margin, collapsed drag guard/cancel lifecycle. Preserve default offcanvas for other consumers.
2. `apps/desktop/src/renderer/src/components/desktop-layout.tsx`: subtract persistent rail from native toolbar clearance, compact-only symmetric canvas margin, top spacer sizing. Preserve routing and workspace gates.
3. `apps/desktop/src/renderer/src/components/desktop-layout.test.tsx`: update its stub `AppSidebar` to `collapsible="icon"`; assert icon data mode, 136/2 desktop versus 200/8 compact geometry, one external trigger, native drag exclusion, and startup/live/interrupted reduced-motion behavior.
4. `packages/views/layout/sidebar-resize.test.tsx`: expanded width survives icon collapse/reopen; collapsed pointer gestures never persist; mid-drag collapse/cancel clears preview widths. This is the current canonical shared primitive suite.
5. `packages/views/layout/sidebar-auto-collapse.test.tsx`: retain existing crossings/manual reopen/no persistence cases; add icon-folded desktop versus full compact Sheet and preserved `lg` gates if needed.
6. `packages/views/layout/page-header.test.tsx` only if fallback trigger contract changes; keep shared behavior out of app tests.
7. Actual CSS/native proof through `e2e/global-scope-navigation.spec.ts` (existing scoped restoration journey) or a dedicated icon-rail journey preserving its original assertions. Verify width and element hit testing, rail toggle on >=1280px, native window toolbar/no-drag controls, short/compact windows, and theme variants. Existing `openSidebarNavigation` helper intentionally expands any desktop collapsed sidebar before testing full navigation; it is still correct for full-nav callers and should not be reused to test a collapsed rail.

Useful verification commands for this slice: `pnpm -C packages/views exec vitest run layout/sidebar-resize.test.tsx layout/sidebar-auto-collapse.test.tsx layout/page-header.test.tsx`; `pnpm -C apps/desktop exec vitest run src/renderer/src/components/desktop-layout.test.tsx src/renderer/src/components/desktop-layout.workspace-gate.test.tsx`. Production Web plus real Electron acceptance remains separate from JSDOM geometry assertions.
