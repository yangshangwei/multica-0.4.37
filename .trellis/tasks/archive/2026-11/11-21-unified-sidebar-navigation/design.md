# Unified sidebar design

## Confirmed direction

Preserve the current restrained product UI and semantic tokens. Replace the top scope switch with a shared left directory:

1. Compact product/account header, scoped search when available, New menu.
2. Collaboration heading (resume), cross-workspace scope caption, Current handling, Participating, Collaboration notifications.
3. Workspace heading/name (resume) with a separate workspace dropdown; personal links, pins, collapsible Work, AI Team and Resources & connections.
4. Workspace analytics/settings and Help in the fixed footer.

The prior user-visible proposal was approved with an explicit request to implement. This document materializes that direction without expanding the product scope. Notifications remain the existing popover; a dedicated notifications page is unnecessary. New remains a menu backed by the existing forms, never a new creation route.

## Boundaries and component contract

- Shared navigation is owned by `packages/views/layout/app-sidebar.tsx`. It accepts an optional explicit `navigationWorkspace: Workspace | null`, `onWorkspaceSelect(slug)`, `onCollaborationResume()` and directory-error retry props from platforms. Default workspace-route behavior may use `useCurrentWorkspace`.
- The sidebar uses explicit `paths.workspace(slug)` rather than requiring workspace route providers. Workspace requests/actions/pins/search and ordinary creation mount or enable only for the real active workspace. In account scope it renders navigation links for the remembered available workspace without making it the active workspace.
- Platform shells choose the navigation workspace from the current route, otherwise the remembered workspace slug, otherwise the first authoritative available workspace. Unknown directory state does not constitute membership loss. Only directory snapshots establish previewable workspaces; never fabricate one from a slug.
- Platform handlers use existing resume adapters for section headings, and explicit NavigationAdapter links for destinations. Keep account tabs and workspace tab groups, all original account destination fragments, ownership fencing and mementos intact.
- Web keeps workspace page/modal providers in its existing dashboard layout, adds an account sidebar layout and simplifies the outer scope shell. A common presentation shell is preferable only if it does not duplicate providers or remount content needlessly.
- Desktop mounts the shared sidebar in both scopes, keeps workspace-only effects/modals/search gated, and removes only the top scope switch. Keep native traffic-light clearance, no-drag controls, history and tab chrome consistent.

## Client preferences and notifications

- Collapsible group preferences live in Core behind StorageAdapter, with a small validated persisted shape containing only booleans. Reuse existing client-preference storage patterns; no API/server data in Zustand.
- Active-route reveal is presentation state, not a persisted override of the user's preference. A user may collapse a current group; entering a different child route reveals it again.
- Reuse `CollaborationNotifications` with a sidebar trigger variant and existing account-scoped protected Query; do not duplicate notification data or endpoint logic. Realtime ownership must avoid duplicate issue subscriptions/invalidation handlers in account content; a workspace-side notification subscription may own account invalidation only outside the workbench.
- Navigation labels distinguish workspace Inbox from Collaboration notifications, and Workspace settings from account/platform management.

## Regression and simplification plan

Before altering ownership or controls, add meaningful tests for no-workspace sidebar access, explicit account views, scope restoration, group preferences and account query-only sheet dismissal. Retain existing pin, permission, unread/chat and draft behavior tests. Remove obsolete top-switch presentation and duplicated workbench view/creation/notification controls once the left directory owns them; retain any embedded/standalone entry needed by source task flows. Do not add a second store for server state or scroll restoration.

## Compatibility, alternatives and rollback

- Retain public `/account?view=...&issue=...#...` and workspace URLs; no redirect or API migration.
- Rejected: append ungrouped collaboration items under workspace name; that misstates scope.
- Rejected: second icon rail or switcher replacing the directory; extra chrome hides destinations and conflicts with the approved unified left navigation.
- Rejected: global notifications page/new view APIs in this task; existing popover fulfills the entry requirement.
- Roll back as one navigation change, retaining existing routes and server contracts. All pre-existing dirty files/hunks remain outside the commit.

## Verification

Focused Core/shared/platform Vitest, locale parity, affected lint/type checks and diff/static checks. Build the Web and Desktop renderer; use task-owned local API and isolated browser/native fixtures for navigation, both locales, compact mode and screenshot review. Persist visual-verdict JSON with reference screenshot and scope-aware acceptance. Record concrete gaps if a required environment cannot run; do not label skipped tests as passing.
