# Unified workspace and collaboration sidebar

## Goal and authorization

Make daily workspace work and cross-workspace collaboration discoverable in one left navigation on Web and Desktop, without changing task ownership or access. The user approved the preceding concrete navigation proposal and explicitly requested task creation, planning, design and implementation on 2026-11-21. That approval covers this implementation; no further product decision is blocking.

## Background

- The existing sidebar is workspace-owned; account collaboration currently removes it. `packages/views/layout/app-sidebar.tsx`, `apps/web/platform/scope-shell.tsx`, and `apps/desktop/src/renderer/src/components/desktop-layout.tsx` own these paths.
- Collaboration is account-owned across workspaces and remains available with zero, pending or unavailable workspace membership. Current and participating are the two supported list views. Notifications are currently a workbench popover.
- The current scope switch restores exact workspace/account locations and scroll state. Explicit task, notification and view links carry their own destinations.
- Layout locale files already contain unrelated uncommitted market changes; preserve these and all other pre-existing work.

## Requirements

R1. A single left sidebar exposes Collaboration above Workspace in both scopes. Remove the top two-button scope switch while preserving native Desktop window controls and drag regions.

R2. Collaboration exposes Current handling, Participating and Collaboration notifications. Its heading can resume the last collaboration location. Each list link opens that explicit view. Notifications retain existing read/archive, paging, denial and exact task/comment/run-link behavior, with a truthful unread badge available from workspace pages.

R3. The workspace selector belongs to the Workspace section. The workspace name can resume its remembered location; links resolve against the selected workspace. Account navigation must never set a workspace mirror or launch workspace-scoped requests on behalf of the previewed workspace. Known-empty and unavailable directories provide create/retry affordances while collaboration stays usable.

R4. Workspace personal links remain visible. Work defaults open; AI Team and Resources & connections default closed, are keyboard accessible and remember manual preferences. Entering a child route reveals its group. Keep two navigation levels and preserve permissions/capability gating, pin semantics and unread counts.

R5. Consolidate ordinary and collaboration creation in a New menu, reusing the existing forms, draft recovery and keyboard shortcut. Workspace-origin collaboration preselects its source; account-origin collaboration keeps source selection explicit. Scope search to the workspace; do not mislabel loaded collaboration search as global search.

R6. Preserve existing product tokens, active/hover distinction, resizing, compact sidebar sheet, touch targets and locale parity. Mobile-sheet navigation dismisses on account query-only changes as well as pathname changes. Settings identifies its workspace scope.

## Acceptance criteria

- AC1 (R1-R3): Web and Desktop both retain a reachable sidebar on workspace and account pages; the old top switch is absent. Account remains accessible with an empty, loading or failed workspace list.
- AC2 (R2): Current and Participating select the correct URL and highlight, including selected details; Collaboration heading resumes the prior detail/filters/scroll. Notifications open from either scope, update unread/read/archive and retain exact links.
- AC3 (R3): Workspace heading restores the prior selected workspace location; workspace links open the displayed workspace. No account-content workspace mirror or implicit workspace reads are introduced.
- AC4 (R4): Work/AI/resources groups toggle with keyboard, persist only client preferences, and reveal active destinations after navigation. Existing gated items, pins and badge behavior remain covered.
- AC5 (R5-R6): New menu opens both existing creation flows, preserves ordinary shortcut, and keeps source selection explicit in account scope. Both locales, compact sheet dismissal, narrow/no-overflow layouts and Desktop native controls are verified.
- AC6: Focused regression tests, affected lint/type checks, static diff checks and built-browser visual evidence pass; any unavailable native/environment verification is recorded accurately.

## Out of scope

New list APIs or saved views (Created by me / Awaiting acceptance / Ended), merged inbox data, global search, task-detail reorganization, global file/AI authorization/knowledge directories, backend or schema changes, mobile app changes and a replacement visual identity.
