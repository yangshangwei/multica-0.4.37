# Implementation report

## Result

Implemented the approved default-collapse and quiet Beta presentation in the shared Web/Desktop sidebar. The title still resumes the account destination, a separate keyboard-accessible arrow controls disclosure, and active account routes reveal the directory through the existing path-local preference behavior.

The collaboration heading and New collaboration issue entry carry neutral, decorative Beta markers without changing their exact accessible control names. A hover/focus tooltip and quiet workbench header paragraph share the approved bilingual maturity explanation. A labelled `output` retains positive unread feedback while collapsed; unknown and zero counts do not render unread indicators.

## Reuse and ownership

- Extended the existing core preferences with `collaboration: false`; legacy stored groups acquire this default through the existing boolean-only parser.
- Extracted `useNavigationGroupDisclosure` from the existing NavigationGroup module. The other groups keep their prior markup, default preferences and path-local reveal behavior.
- The collaboration directory owns one existing protected infinite notification query. Its count stays live while collapsed, and the presentational popover unmounts with the directory. The standalone `CollaborationNotifications` export retains a query-owning wrapper for existing callers. No count is copied into view or store state.
- No dependencies, backend contracts, platform routes, broad restyling or unrelated user changes were modified.

## Files changed

- `packages/core/navigation/sidebar-preferences.ts`
- `packages/core/navigation/sidebar-preferences.test.ts`
- `packages/views/layout/app-sidebar.tsx`
- `packages/views/layout/app-sidebar.test.tsx`
- `packages/views/layout/sidebar-navigation-group.tsx`
- `packages/views/layout/sidebar-navigation-group.test.tsx`
- `packages/views/collaboration/notifications.tsx`
- `packages/views/collaboration/account-workbench-page.tsx`
- `packages/views/collaboration/account-workbench-page.test.tsx`
- `packages/views/locales/en/collaboration.json`
- `packages/views/locales/zh-Hans/collaboration.json`

## Verification

Before product edits:

- `pnpm --filter @multica/core exec vitest run navigation/sidebar-preferences.test.ts` — 3/3 passed.
- `pnpm --filter @multica/views exec vitest run layout/sidebar-navigation-group.test.tsx layout/app-sidebar.test.tsx collaboration/account-workbench-page.test.tsx collaboration/notifications.test.tsx locales/parity.test.ts` — 123/123 passed.
- Added disclosure and preference regressions failed against the incumbent behavior: missing collapsed state/disclosure/feedback and missing collaboration preference default.
- Inspected the fresh native baseline supplied by root at `research/visual/native-baseline-workspace-zh.png` before UI edits.

After implementation:

- Core preferences suite — 4/4 passed.
- The five related views suites above — 127/127 passed, including canonical notification actions and locale parity.
- After making collapsed unread feedback a semantic labelled `output`, `pnpm --filter @multica/views exec vitest run layout/app-sidebar.test.tsx` — 44/44 passed.
- `pnpm --filter @multica/core typecheck` — passed.
- `pnpm --filter @multica/views typecheck` — passed. The initial local query-result pageParam type and a Testing Library-only option were corrected without assertions or boundary changes.
- Affected-file ESLint in core and views — passed. Sidebar lint was rerun after the semantic output change.
- `git diff --check` over the final owned files — passed.

## Remaining verification

Root owns the bounded native/Web visual inspection, platform-consumer type checks, E2E fixture adjustment, spec synchronization, commit and task completion. The native baseline backend is older; actual screenshots establish frontend presentation/interaction, not current backend collaboration execution support.
