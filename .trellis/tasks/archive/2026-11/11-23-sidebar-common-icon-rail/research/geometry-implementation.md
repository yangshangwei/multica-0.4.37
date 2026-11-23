# Geometry implementation evidence

## Owned changes

- `packages/ui/components/ui/sidebar.tsx`: publish one numeric
  `SIDEBAR_WIDTH_INSET_ICON` (64px) and the matching runtime property; use it for
  both inset gap and painted container. Keep generic offcanvas and the existing
  compact/auto-collapse state machine. Floating retains its former 2px paint
  allowance. Web inset's extra collapsed margin is now offcanvas-specific.
- `SidebarRail` rejects resize startup while collapsed/compact and cancels a
  preview in layout cleanup when folding, changing compact mode or unmounting.
  Remembered expanded width is not overwritten by a collapsed gesture.
- As requested by the shared UI lane, icon content permits vertical scrolling
  and coarse-pointer icon menu targets become 44px; fine targets remain 32px.
- `apps/desktop/src/renderer/src/components/desktop-layout.tsx`: preserve the
  fixed 200px toolbar by reserving its remaining 136px beside the persistent
  rail. Compact sheets still require 200px. Canvas margins are 2px beside either
  large-screen presentation and 8px in compact layouts. The narrow top spacer
  has no drag surface over toolbar controls; expanded/compact exclusions retain
  their existing 200px left margin.
- Tests are limited to the existing shared resize/auto-collapse suites and
  Desktop shell suite. No business navigation, locale, E2E or store edits.

## Test-first evidence

Before production edits, the added shared resize regressions failed exactly at
the missing behaviors: a gesture from a 64px collapsed rail wrote
`sidebar_width=236`, and an active 300px preview survived folding. The new
width-consistency regression lacked the shared 64px property. Desktop's changed
geometry assertions failed with `200px` instead of the required `136px`.

After implementation, focused checks passed:

| Command | Result |
| --- | --- |
| `pnpm -C packages/views exec vitest run layout/sidebar-resize.test.tsx layout/sidebar-auto-collapse.test.tsx layout/sidebar-wrapper-fill.test.tsx` | 3 files, 17 tests passed |
| `pnpm -C apps/desktop exec vitest run src/renderer/src/components/desktop-layout.test.tsx src/renderer/src/components/desktop-layout.workspace-gate.test.tsx` | 2 files, 20 tests passed |
| `pnpm -C packages/ui typecheck` | Passed |
| `pnpm -C apps/desktop typecheck` | Node and renderer passed |
| `pnpm -C packages/ui exec eslint components/ui/sidebar.tsx` | Passed |
| `pnpm -C apps/desktop exec eslint src/renderer/src/components/desktop-layout.tsx src/renderer/src/components/desktop-layout.test.tsx` | Passed |
| `pnpm -C packages/views exec eslint layout/sidebar-resize.test.tsx layout/sidebar-auto-collapse.test.tsx` | Passed |
| `git diff --check` for the five owned files | Passed |

The extra compact mid-drag regression verifies removal cancels preview widths
and prevents a later pointer-up commit. Desktop preserves startup/live reduced
motion and interrupts an in-flight spring at the new targets. The compact test
still proves full 200px/8px clearance.

## Integration limits

A full `packages/views typecheck` run encountered the UI lane's then-in-progress
`app-sidebar.test.tsx` changes (anchor/button mock props and unsupported
`exact` in Testing Library role options). These findings were sent directly to
that owner and were not fixed outside this slice. The parent must rerun after
integration. pnpm emits the checkout's existing warning about its ignored
package.json `pnpm` settings; no dependency configuration was changed.

JSDOM does not serialize `-webkit-app-region`, so the unsupported computed-style
assertion was removed; the meaningful spacer margin transition remains covered.
Actual compiled CSS/native drag and hit testing, short-height scrolling and
44px coarse target measurements require the parent's production Web/Electron
journey. No build or server was launched by this slice. No staging, commit,
push or merge was performed. Baseline hashes are in `geometry-baseline.json`.

## Account shell trigger wiring follow-up

The parent's compiled-browser integration found that `WebAccountShell` claimed
an external always-visible trigger while its content opener was `xl:hidden`.
This suppressed the shared rail trigger and stranded a collapsed account sidebar
at 1440px. The follow-up was scoped to `apps/web/platform/account-shell.tsx` and
its canonical wiring test.

The new regression first failed because the provider published
`hasExternalTrigger=true`. Removing that declaration lets the shared sidebar own
its large-screen rail toggle; changing the account content opener to `lg:hidden`
keeps that chrome for the compact Sheet only. Workspace mirror clearing,
navigation preview, providers and feedback ownership remain unchanged.

Verification after the wiring fix:

- `pnpm -C apps/web exec vitest run platform/account-shell.test.tsx platform/scope-shell.test.tsx`: 13 tests in 2 files passed.
- `pnpm -C apps/web exec eslint platform/account-shell.tsx platform/account-shell.test.tsx`: passed.
- `pnpm -C apps/web typecheck`: passed.
- `git diff --check` for both files: passed.

No server/build, staging or commit was performed by this follow-up. Compiled Web
acceptance of the now-reachable account rail trigger remains the parent's gate.

## Compiled coarse icon alignment correction

The parent's r3 Web journey confirmed a real presentation defect at 1100×650:
nine 16px menu icons were 6px left of the centers of their 44px coarse-pointer
targets. The pre-edit visual verdict was read from
`.omx/state/sidebar-common-icon-rail/ralph-progress.json` (score 86, revise).

The bounded correction adds `group-data-[collapsible=icon]:justify-center` to
the generic `SidebarMenuButton` variants. Expanded navigation alignment and
individual button implementations are unchanged; all existing icon menu callers
share the same centering behavior.

Checks after correction:

- `pnpm -C packages/views exec vitest run layout/app-sidebar.test.tsx layout/sidebar-resize.test.tsx layout/sidebar-auto-collapse.test.tsx`: 85 tests in 3 files passed.
- `pnpm -C packages/ui exec eslint components/ui/sidebar.tsx`: passed.
- `pnpm -C packages/ui typecheck`: passed.
- `git diff --check` for the primitive: passed.
- Primitive SHA-256 after correction: `93b8c5968206f03aeee8263260c80d949dc1d5792dc05cde6fcaa7a4c795e81f`.

The real E2E failure is the regression evidence; a new class-only test was not
added. Compiled Web/native alignment confirmation remains the parent's next
gate. No build, staging or commit was performed by this correction.
