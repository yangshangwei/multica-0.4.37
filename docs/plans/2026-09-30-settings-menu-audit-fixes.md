# Settings menu audit fixes

**Goal:** Resolve the six approved settings-menu audit findings while preserving settings URLs, feature gates, and the shared web/desktop visual system.

**Architecture:** Keep the declarative settings navigation as the single source of truth. Desktop settings form their own final group; workspace-wide labels belong to workspace administration. Use one vertical, named, grouped tab list, displayed in the sidebar on desktop and a dismissible settings-directory sheet below 768px. The existing Base UI components own keyboard navigation and focus restoration.

**Scope:** Shared Tabs orientation; settings navigation, responsive shell and tests; desktop injection, labels and icons; English/Chinese locale keys; relevant convention notes. No new dependencies.

## Implementation and verification

1. Add failing regressions for vertical arrow navigation and named groups in the real settings shell, plus opening/selecting/dismissing the compact directory. Retain URL and feature-gate coverage.
2. Update the pure navigation order tests before moving labels to workspace and injected tabs to a desktop-only group. Rename the internal injection prop to `extraDesktopTabs` in all callers. Keep every `?tab=` value unchanged.
3. Forward `orientation` into Base UI; render labelled groups inside one vertical tablist. Use existing Sheet/Button primitives for the compact directory; restore focus on dismissal and close on selection. Expand touch targets to 44px, including on coarse-pointer desktop devices; allow long labels to wrap.
4. Distinguish daemon, server connection, MCP and desktop behavior icons. Update server-connection copy and directory/group strings in both locales.
5. Run focused Vitest suites and locale parity, affected-package typechecks/lint, static design detector and diff checks. Verify keyboard flow, selected-hover styling, compact navigation, dark mode and enlarged text in a browser in two bounded inspection rounds. Record a visual verdict under `.omx/state/settings-menu-audit/`.
6. Document the settings navigation and primitive forwarding contracts. Report changed areas, evidence and any limits.

## Ownership

- Main agent: shared Tabs, settings shell and DOM tests, browser verification, integration and spec updates.
- Navigation agent: settings-nav.ts/tests, desktop route/tests, settings locale files only.

## Decisions

- A grouped settings sheet keeps all destinations discoverable on compact screens while reusing the existing focus-management primitive.
- Keep the 32px dense desktop row minimum for fine pointers; compact/coarse-pointer targets have a 44px minimum and can grow for wrapped text.
- Labels move to workspace because the page configures both task and skill labels. Desktop application settings come last because they are outside workspace configuration.
- Existing unrelated working-tree changes are outside this task.

## Completed verification

- Settings and locale suite: 25 files, 367 tests passed. Desktop route suite: 6 tests passed.
- UI, Views, Web and Desktop typechecks passed. Changed UI/Views/Desktop source and tests passed ESLint; `git diff --check` passed.
- Impeccable detector: no findings. Focused independent review: no remaining findings after fixing panel names and breakpoint dismissal.
- Browser fixture: real SettingsPage, DesktopSettingsRoute, Base UI and application CSS; setting content/network isolated because the local API is stopped. Verified cross-group arrows, Enter, selection hover, directory entry focus, selection/Escape dismissal and return focus, panel naming, resize reset, long-label wrapping, coarse-pointer 44px targets, English/Chinese, light/dark, 320px width and CSS zoom 200%.
- Screenshot and structured verification evidence: `.omx/state/settings-menu-audit/`. Final visual verdict: pass, 95/100. Full authenticated settings-content flows were not exercised.

## Changed files

- `packages/ui/components/ui/tabs.tsx`
- `packages/views/settings/components/settings-page.tsx`
- `packages/views/settings/components/settings-page.test.tsx`
- `packages/views/settings/components/settings-nav.ts`
- `packages/views/settings/components/settings-nav.test.ts`
- `apps/desktop/src/renderer/src/components/desktop-settings-route.tsx`
- `apps/desktop/src/renderer/src/components/desktop-settings-route.test.tsx`
- `packages/views/locales/en/settings.json`
- `packages/views/locales/zh-Hans/settings.json`
- `.trellis/spec/views/frontend/component-guidelines.md`
- `docs/plans/2026-09-30-settings-menu-audit-fixes.md`
