# Research: Desktop tab reordering, triage launcher clearance, and reduced motion

- Query: Locate the implementation boundaries, reusable patterns, and meaningful regression coverage for audit items P1-06, P2-07, and P2-09.
- Scope: internal, with standards and library reference links
- Date: 2026-10-09

## Findings

### Current checkout differs from the audited snapshot

All three assigned items already have corresponding source changes and regression tests in the checkout read for this research. The audit describes an earlier snapshot; do not blindly reimplement its proposed fixes. This researcher did not run git operations or modify product code, so the ownership/origin of those existing changes is deliberately not inferred. The main session must review its own baseline and decide which existing changes belong to this task.

| Audit item | Current implementation | Existing test coverage | Remaining proof |
| --- | --- | --- | --- |
| P1-06: pointer-only tab reordering | Move-left/right context-menu items, `Alt+Shift+ArrowLeft/Right`, pinned-segment guards, focus restoration | `tab-bar.accessibility.test.tsx` covers menu/keyboard moves, boundaries, current destination, focus, persistence, and Chinese copy | Real Electron keyboard/context-menu interaction, overflow visibility, preserved pointer dragging |
| P2-07: triage pagination overlaps chat at 900px | Footer uses `pe-chat-launcher`, logical start padding, and flex wrapping | `triage-page.test.tsx:226` enables Next with `total=101`, checks the shared class, and verifies `offset=50` navigation | Browser-computed rectangles and an actual enabled Next click at 900×700 and enlarged scale |
| P2-09: sidebar geometry interpolates despite reduced motion | Shell subscribes to `matchMedia` and uses zero-duration transitions for header, drag region, and canvas | `desktop-layout.test.tsx:151` checks multiple animation frames, both initial and runtime preference changes | Real Electron media emulation and frame sampling, plus ordinary-motion retention |

### P1-06: reorder through the existing tab owner

- `apps/desktop/src/renderer/src/components/tab-bar.tsx:181` owns each sortable tab's navigation trigger and context menu. Reordering remains desktop shell behavior, not a shared business-view component.
- `tab-bar.tsx:305` handles only `Alt+Shift+ArrowLeft/Right` and delegates to the same `onMove` callback used by the menu. It prevents default and propagation for the recognized chord; normal tab activation remains a native button behavior.
- `tab-bar.tsx:319` exposes the title, `aria-current="page"`, and `aria-keyshortcuts`. The desktop shell currently uses navigation semantics, so adding a separate `role="tab"` contract is unnecessary for this scope.
- `tab-bar.tsx:449` renders localized Move left/Move right menu items and disables impossible moves. The UI derives each boundary from the immediately adjacent tab's `pinned` state at `tab-bar.tsx:729`.
- `tab-bar.tsx:688` resolves the current indices by tab ID, rejects a nonexistent neighbor or a pinned/unpinned crossing, sets a pending focus ID, then calls the existing store's `moveTab`.
- `tab-bar.tsx:634` restores focus to the moved tab after the new order commits and calls `keepTabVisible`. `tab-bar.tsx:160` intentionally scrolls only the tab strip; native `scrollIntoView` can displace desktop shell ancestors.
- `tab-bar.tsx:597` retains PointerSensor with its 5px activation constraint. A KeyboardSensor alone would not satisfy the audit's non-drag pointer alternative; the click commands address that requirement without replacing pointer drag behavior.
- `apps/desktop/src/renderer/src/stores/tab-store.ts:252` documents that `moveTab` operates only on the active workspace and clamps pinned/unpinned crossings. It already preserves the rest of the group, including the active destination; do not introduce another reorder store or persistence format.
- `packages/views/locales/en/desktop.json:3` and `packages/views/locales/zh-Hans/desktop.json:3` contain both commands and a reorder hint. `tab-bar.tsx:702` passes that hint to DnD accessibility instructions.

Canonical regression locations:

- `apps/desktop/src/renderer/src/stores/tab-store.test.ts:981` owns the pinned-boundary clamp matrix. Avoid duplicating that entire pure-state matrix in a DOM test.
- `apps/desktop/src/renderer/src/components/tab-bar.accessibility.test.tsx:48` owns actual shell wiring: exactly one current destination; inactive tab reorder without changing destination; keyboard/menu boundaries; focus; persisted order; Chinese command labels.
- `apps/desktop/src/renderer/src/components/tab-bar.test.tsx` covers established shell behavior including title presentation and tab-strip layout/visibility behavior. Keep those checks when validating the new commands.

Additional useful acceptance checks are one real keyboard move and one secondary-click menu move with many overflowing tabs, checking that the moved tab retains focus and remains visible, then exercising pointer drag and confirming the active content/history stayed unchanged. The existing low-level tests use `fireEvent`; they do not establish OS shortcut routing or assistive-technology announcement behavior.

### P2-07: reuse the launcher geometry contract

- `packages/views/triage/triage-page.tsx:875` is the single pagination component used by the shared triage surface. The page already stores pagination intent in route parameters; a layout fix should not change queue selection, filtering, or offset behavior.
- `triage-page.tsx:886` now has `flex-wrap ... ps-3 pe-chat-launcher`. This is the smallest fix: reserve the overlay corner using the existing utility and permit narrow content to wrap.
- `packages/ui/styles/base.css:21` defines `pe-chat-launcher` as `padding-inline-end: var(--chat-launcher-clearance)`. The same file documents why a full-width footer uses this variant instead of padding-bottom or `above-chat-launcher`.
- `packages/ui/styles/tokens.css:149` owns launcher geometry: size `2.5rem`, inset `0.5rem`, clearance `size + inset * 2`. At a default 16px root size the reserved distance is 56px. Do not hardcode the audit's 40px button size or introduce another local clearance constant.
- `packages/views/chat/components/chat-fab.tsx:74` derives its actual size and bottom/right inset from those same tokens.
- `apps/desktop/src/renderer/src/components/desktop-layout.tsx:324` places both page content and FloatingChat inside the same relative MainCanvas, which is the correct geometry boundary for the overlap assertion.
- The audit's `.impeccable/audit/2026-10-09-desktop-core/triage-fab-overlap.json` records old Next `(816,652,64,32)` and launcher `(844,644,40,40)` bounds. The old specimen's Next was disabled; an enabled specimen is required for closure.

Canonical regression and acceptance shape:

- `packages/views/triage/triage-page.test.tsx:226` now sets `total=101`, asserts Next is enabled, asserts the footer's shared clearance utility, clicks Next, and verifies `/acme/triage?issue=a&offset=50`. This proves route wiring; jsdom does not compute the actual overlap.
- Browser acceptance should read both `getBoundingClientRect()` values at 900×700 with Next enabled and assert no intersection. Click the actual center of Next (or use a normal Playwright click, which checks hit targeting), then verify offset 50 and subsequent Previous behavior. Repeat with both retained locales and native zoom 2.0; check page-level horizontal overflow and wrapping.
- Prefer fixtures/mocked read responses in an isolated Electron window for an enabled page, or seed an isolated test workspace with TestApiClient. Do not mutate the user's current queue merely to obtain a second page.
- `e2e/triage-desktop.spec.ts:9` already exercises real renderer/preload, an isolated profile, shared real backend, navigation, review, and compact history. It currently seeds one item and never exercises enabled pagination or the 900px launcher overlap; it is a useful harness, not proof of this fix.

### P2-09: instant final shell geometry with live preference changes

- `apps/desktop/src/renderer/src/components/desktop-layout.tsx:43` names the standard media query; `desktop-layout.tsx:53` subscribes/unsubscribes to its change event, and `desktop-layout.tsx:245` reads it through `useSyncExternalStore`. This covers an OS preference change while the window remains open.
- `desktop-layout.tsx:140` animates the header's `paddingLeft`, `desktop-layout.tsx:146` animates the drag region's `left`, and `desktop-layout.tsx:172` animates the canvas's `marginLeft`. All three now choose `{ duration: 0 }` under reduced motion while retaining the existing spring otherwise.
- Final values remain the existing geometry: collapsed/compact header and drag clearance 184px versus expanded 0px; collapsed/compact canvas margin 8px versus expanded 2px. All three must snap together, otherwise tabs or drag surfaces can still slide or temporarily cover fixed toolbar controls.
- `packages/ui/components/ui/sidebar.tsx:360` and `:372` already have `motion-reduce:transition-none` for their CSS width/position transitions. That CSS does not govern the shell's Motion-generated inline interpolation, which is why the explicit shell branch is necessary.
- `desktop-layout.test.tsx:151` uses a live `matchMedia` mock and the real Motion components. It samples five requestAnimationFrame callbacks after each toggle and requires the final header/drag/canvas inline values on every sampled frame. It runs for reduced motion present at launch and toggled while running.
- `.impeccable/audit/2026-10-09-desktop-core/reduced-motion.json` preserves the earlier interpolating values; use equivalent frame sampling for after-fix evidence.

Scope boundary: `packages/views/layout/animated-right-sidebar.tsx:134` is a separate detail-panel Motion wrapper. Its CSS flex-size transition is disabled in `packages/ui/styles/base.css:293`, but its inner opacity/x Motion transition does not read reduced-motion preference. The audit's P2-09 evidence specifically targets the desktop left-sidebar shell's padding/margin. Do not silently broaden this lane to every animation; the main task may record or separately assign this adjacent finding if needed.

## Files Found

| Path | Description |
| --- | --- |
| `docs/audits/2026-10-09-desktop-core-impeccable.md` | Source audit, severity, evidence boundaries, recommended ordering |
| `apps/desktop/src/renderer/src/components/tab-bar.tsx` | Sortable tab triggers, menu commands, focus and scroll restoration |
| `apps/desktop/src/renderer/src/components/tab-bar.accessibility.test.tsx` | Existing current-destination and reorder interaction regressions |
| `apps/desktop/src/renderer/src/components/tab-bar.test.tsx` | Established tab appearance/presentation/visibility behavior |
| `apps/desktop/src/renderer/src/stores/tab-store.ts` | Existing persisted tab state, pinned boundary, reorder owner |
| `apps/desktop/src/renderer/src/stores/tab-store.test.ts` | Canonical pure reorder boundary tests |
| `apps/desktop/src/renderer/src/components/desktop-layout.tsx` | Left-sidebar shell geometry and live reduced-motion subscription |
| `apps/desktop/src/renderer/src/components/desktop-layout.test.tsx` | Shell wiring and real Motion geometry-frame regressions |
| `packages/views/triage/triage-page.tsx` | Shared queue pagination and corner reservation |
| `packages/views/triage/triage-page.test.tsx` | Enabled Next navigation and clearance utility regression |
| `packages/views/chat/components/chat-fab.tsx` | Launcher actual size and inset |
| `packages/ui/styles/tokens.css` | Shared launcher geometry source |
| `packages/ui/styles/base.css` | Shared corner utilities and existing reduced-motion CSS rules |
| `packages/ui/components/ui/context-menu.tsx` | Existing Base UI menu primitive, focus, disabled-item behavior |
| `packages/ui/components/ui/sidebar.tsx` | Existing CSS motion preference handling in the sidebar |
| `packages/views/layout/animated-right-sidebar.tsx` | Separate detail-sidebar state/animation boundary |
| `packages/views/layout/animated-right-sidebar.test.tsx` | Detail-sidebar restored geometry, explicit toggle window, shortcut tests |
| `packages/views/locales/{en,zh-Hans}/desktop.json` | Localized reorder commands and keyboard hint |
| `e2e/triage-desktop.spec.ts` | Real Electron triage acceptance harness, currently one-page specimen |
| `e2e/fixtures/changelog-electron.cjs` | Native services isolated while using the real preload/renderer |
| `.impeccable/audit/2026-10-09-desktop-core/scripts/harness.cjs` | Original audit-only isolated read fixture |

## Related Specs

- `CLAUDE.md`: authoritative package boundaries, platform-owned tab state, semantic tokens, localized copy conventions, and canonical test ownership. Shared page behavior belongs in views tests; desktop shell behavior belongs in desktop tests. No new dependencies or stores are needed here.
- `.trellis/spec/desktop/frontend/index.md` and `quality-guidelines.md`: desktop scope and isolated Electron acceptance prerequisites. Served renderer/API origins must agree; preserve native preload and isolate unrelated daemon services.
- `.trellis/spec/views/frontend/index.md`, `component-guidelines.md`, and `quality-guidelines.md`: preserve direct navigation semantics, visible focus, Base UI primitives, and tests under the owning shared package.
- `.trellis/spec/ui/frontend/index.md`: shared primitive/token scope; most linked guidelines are placeholders, so do not invent additional requirements from them.
- `.trellis/spec/guides/code-reuse-thinking-guide.md`: reuse existing store operations and token geometry; do not introduce parallel command or clearance layers.

## External References

- Existing direct package versions: `@dnd-kit/core ^6.3.1`, `@dnd-kit/sortable ^10.0.0`, Motion `^12.38.0`, Base UI `^1.3.0` (shared views/ui dependencies), Electron `^39.2.6` declared by desktop. The audit reports actual Electron 39.8.7. No upgrades are necessary for these fixes.
- WCAG 2.1.1 keyboard: https://www.w3.org/WAI/WCAG22/Understanding/keyboard.html
- WCAG 2.5.7 dragging movements: https://www.w3.org/WAI/WCAG22/Understanding/dragging-movements.html — provide a non-drag single-pointer operation in addition to keyboard behavior.
- WCAG 2.3.3 animation from interactions: https://www.w3.org/WAI/WCAG22/Understanding/animation-from-interactions.html — AAA criterion; retain the audit's distinction from an AA compliance claim.
- Motion reduced-motion reference: https://motion.dev/docs/react-use-reduced-motion
- Base UI context-menu reference: https://base-ui.com/react/components/context-menu
- These documentation links identify the relevant contracts; external pages were not fetched in this internal research pass.

## Verification Handoff

Commands for the implement/check owner, not claims that this researcher ran them:

```sh
pnpm --filter @multica/desktop exec vitest run src/renderer/src/components/tab-bar.accessibility.test.tsx src/renderer/src/components/tab-bar.test.tsx src/renderer/src/components/desktop-layout.test.tsx src/renderer/src/stores/tab-store.test.ts
pnpm --filter @multica/views exec vitest run triage/triage-page.test.tsx locales/parity.test.ts
pnpm --filter @multica/desktop exec eslint src/renderer/src/components/tab-bar.tsx src/renderer/src/components/desktop-layout.tsx src/renderer/src/components/tab-bar.accessibility.test.tsx src/renderer/src/components/desktop-layout.test.tsx
pnpm --filter @multica/views exec eslint triage/triage-page.tsx triage/triage-page.test.tsx
pnpm --filter @multica/desktop typecheck
pnpm --filter @multica/views typecheck
```

Record fresh Electron screenshots, enabled-pagination hit testing, keyboard/menu focus state, and reduced-motion frame samples separately from the original audit baseline. Avoid modifying its 32 original screenshots or implying those pre-fix files validate the current checkout.

## Caveats / Not Found

- No product-code, spec, task metadata, git state, or other task files were changed by this research role.
- Existing tests were inspected, not executed. Current corresponding source/tests establish implementation presence, not passing validation or task completion.
- The relevant general desktop/core/UI spec files still contain placeholders. The active desktop acceptance contract and root CLAUDE.md provide the concrete rules.
- Browser/Electron rectangles and reduced-motion behavior require fresh acceptance evidence. The existing jsdom triage test cannot establish non-overlap.
- The audit's `run.mjs` attaches to fixed local CDP and renderer ports. Inspect actual environment/profile ownership before reusing it; do not attach to the user's active window.
- The original audit fixture deliberately blocks business writes and only allows its two read-only POST table endpoints. Use an appropriate isolated fixture when an enabled pagination specimen requires more data.
- The separate inner right-detail-sidebar Motion effect remains an adjacent finding, outside the recorded P2-09 shell evidence.
