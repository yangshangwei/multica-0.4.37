# Research: Web unified sidebar integration

- Query: Make the same left navigation available on workspace and account collaboration routes, remove the top scope switch, preserve navigation restoration and independent account access.
- Scope: internal
- Date: 2026-11-21

## Findings

### Current ownership and boundaries

- `apps/web/components/web-providers.tsx:116`: `WebScopeShell` surrounds the entire routed application inside `WebNavigationProvider` and `WebScrollRestorationProvider`; it is outside both workspace and account route providers.
- `apps/web/platform/scope-shell.tsx:14`: the scope shell already owns identity-scoped history recording, the last workspace/account resume resolution, and stale asynchronous workspace-refetch fencing. Its current top switch is the only presentation that needs removing; retain these navigation mechanics.
- `apps/web/app/[workspaceSlug]/layout.tsx`: the route guards login/onboarding and known workspace membership, mirrors the workspace into the API platform singleton, then provides `WorkspaceSlugProvider`. It guards its mirror write by current pathname to prevent outgoing layouts from restoring an obsolete workspace.
- `packages/views/layout/dashboard-layout.tsx:36`: workspace `DashboardLayout` owns `SidebarProvider`, `GlobalShortcuts`, `AppSidebar`, `SidebarInset`, `ModalRegistry`, and workspace presence. It is inside `DashboardGuard`; account routes must not use this guard.
- `apps/web/app/[workspaceSlug]/(dashboard)/layout.tsx:20`: the Web adapter injects `SearchTrigger` and mounts `SearchCommand`, notification bridge, and floating chat as workspace-only extras.
- `apps/web/app/account/layout.tsx`: account layout currently wraps children only in `AccountRoute`.
- `apps/web/platform/account-route.tsx:9`: account access clears `setCurrentWorkspace(null, null)` during render and depends on authentication/setup/password state, never on a workspace-list result. This behavior must remain above account reads.
- `packages/core/paths/hooks.tsx`: `useWorkspacePaths()` throws without the actual route slug; `useCurrentWorkspace()` and `useWorkspaceSlug()` are nullable. The existing `AppSidebar` calls the throwing hook and therefore cannot be mounted unchanged under `/account`.

### Smallest integration shape

Keep the routed workspace/account providers in place. Extract account-safe navigation chrome from the current `AppSidebar`, with a workspace-only child for parts that need route-owned workspace hooks. Mount that shared sidebar under both the existing workspace `DashboardLayout` and a small account sidebar layout. Route transitions may remount the sidebar, which is acceptable if durable fold preferences are owned by the Core store; preserving both entire application trees merely to keep the sidebar mounted is unnecessary and conflicts with the current restoration contract.

Recommended shared sidebar interface is either a nullable explicit workspace descriptor plus platform resume callbacks, or an account-safe root that uses nullable workspace hooks and receives resume callbacks through shared navigation plumbing. Build cross-workspace menu links with `paths.workspace(slug)`, never by temporarily installing a remembered slug around account content.

Account layout needs `SidebarProvider`, the same `AppSidebar`, `SidebarInset`, and an always reachable compact `SidebarTrigger`. It must render authenticated collaboration even if the workspace directory is pending, empty, or fails. Workspace directory loading/error/retry belongs inside the workspace section of the sidebar and cannot replace account content. `DashboardLayout` and the new account layout can share a focused shell component if that avoids repeating meaningful shell markup, but do not broaden this task to relocate all workspace resources into root providers.

Retain `WebScopeShell`'s stable element tree: its regression test proves that adding/removing wrappers after viewer discovery remounts page content and loses an active editor or deep-linked chat state. It may become a scope-navigation context provider and stable wrapper while losing the 48px header and `top-12` sidebar offsets. A shared platform-neutral callback surface should expose "resume workspace"/"resume collaboration" for sidebar section actions; explicit row links continue to call normal `push`/`AppLink`.

### Resume and explicit navigation

- `packages/core/navigation/scope-history.ts` owns existing `navigationScope`, `resolveScopePath`, and `useScopeHistoryStore`. No replacement navigation-history store is needed.
- The workspace section's resume affordance must call the existing resolver with an authoritative directory only when `list.ready` is true. Unknown directory is not membership loss. The existing async attempt/owner checks are necessary after a directory retry.
- A collaboration group heading may resume the exact remembered detail; `Current work` and `Participating` rows are explicit destinations and must not silently redirect through history.
- `apps/web/platform/navigation.tsx` provides `useWebScopeResume`, normal `push`, same-resource replacement, and deferred focus-intent publication. Preserve it unchanged unless introducing the callback interface requires a narrow extension.
- `AccountRoute` must clear the ambient workspace header before shared account queries render. A remembered workspace displayed as a directory shortcut is not active workspace authority.

### Search and new-task scoping

- `packages/views/search/search-command.tsx:319` calls `useWorkspaceId()` and makes workspace issue/project/member queries. Mount it only in a confirmed active workspace; do not mount it under `/account`, even when a remembered workspace exists.
- `packages/views/search/search-trigger.tsx:12` only opens `useSearchStore`, but a trigger without the matching mounted search command would be a dead action. Keep it in the workspace section, label its scope, or omit it in account mode.
- `packages/views/layout/global-shortcuts.tsx` calls `useWorkspacePaths()` and builds workspace destinations. Keep that component under the actual workspace provider. Account sidebar toggle keyboard handling, if needed, requires an account-safe subset rather than mounting these global workspace shortcuts.
- `packages/views/modals/registry.tsx` mounts create-issue/project dialogs, parent pickers, and issue-limit UI that depend on workspace context. Keep it in the workspace subtree. An account-side ordinary `New task` must first enter a selected workspace through a deliberate navigation flow; it cannot merely open the global modal store while remaining in account scope.
- Account collaboration creation already has its own source-workspace picker through `CollaborationCreationEntry`; preserve this existing route-independent creation behavior. Workspace-side collaboration creation uses `SourceCollaborationCreation` and preselects that workspace.

### Files and canonical tests

Likely Web changes:

1. `apps/web/platform/scope-shell.tsx` and `.test.tsx`: remove the top switch/48px offsets, retain restoration callbacks, identity reset, stale-fetch protection, and no-remount behavior.
2. `apps/web/platform/account-route.tsx` and `.test.tsx`, or a new focused account layout imported by `apps/web/app/account/layout.tsx`: install the account-safe sidebar shell after the existing auth checks.
3. `packages/views/layout/app-sidebar.tsx` plus extracted children: account-safe shared shell and collaboration rows; workspace-only server queries/creation/pins remain gated.
4. `packages/views/layout/dashboard-layout.tsx`: only if extracting shared shell markup or passing resume callbacks.
5. `packages/views/layout/index.ts`: export any new shared shell/context API.
6. Shared sidebar tests should own collaboration/current/participating selection, no-workspace rendering, folds, keyboard interactions, notifications, and scoped search/new actions. App tests own only wiring.

Existing regression locations:

- `apps/web/platform/scope-shell.test.tsx`: full remembered workspace/account URL restoration, selected-scope no-op, confirmed empty directory, removed directory membership, failed refetch with usable account content, stale async refetch, stable children during owner resolution, no chrome in auth/one-shot routes.
- `apps/web/platform/account-route.test.tsx`: mirror clearing before private children, login redirect, setup/password blocking.
- `apps/web/platform/navigation.test.tsx`: deferred focus intent, explicit/resume distinction, same-route fragment behavior.
- `packages/views/layout/page-header.tsx:45`: compact fallback toggle exists only when a `SidebarProvider` is present and no external toggle is declared. Collaboration's own header does not currently use this helper, so adding sidebar on account must supply a compact toggle explicitly.

### Browser/native acceptance harness

`e2e/global-scope-navigation.spec.ts` already exercises production Web and actual Electron, exact source/account issue restoration, search/sort preservation, two independent scroll roots, failed cached read then successful refresh without scroll movement, explicit description-anchor focus, compact layout, and Chinese navigation. It currently selects the top global controls by `Application navigation` and two exact buttons. Adapt those selectors to sidebar group resume controls while retaining all behavior and geometry assertions that remain meaningful. Do not replace the existing long restoration journey with only new menu-visibility checks.

The test requires `COLLABORATION_E2E_API_URL`, `DATABASE_URL`, and for Desktop `COLLABORATION_ELECTRON_RENDERER_URL`; Playwright does not launch servers (`playwright.config.ts`). It seeds isolated workspaces/issues through `TestApiClient`, seeds narrowly scoped collaboration rows through PostgreSQL, and removes only those rows before workspace cleanup.

`e2e/fixtures/collaboration-native.ts` provides real Electron harness startup, deep links, native compositor screenshots, and viewport/200% zoom coverage. `selectDesktopWorkspace` still assumes the old top Workspaces button and a first sidebar-header workspace trigger; update this helper once centrally if the new sidebar changes those affordances.

## Related specs

- `.trellis/spec/core/frontend/global-scope-navigation.md`
- `.trellis/spec/desktop/frontend/account-navigation.md`
- `.trellis/spec/core/frontend/collaboration-workbench.md`
- `.trellis/spec/views/frontend/desktop-core-accessibility.md`
- `.trellis/spec/web/frontend/e2e-run-environment.md`

The global navigation spec explicitly requires the old fixed 48px two-button switch. The accepted product redesign intentionally supersedes that presentation contract; update the spec to the new left-sidebar affordances while retaining ownership/restoration/anchor guarantees.

## Caveats / Not Found

- Missing product/design metadata does not imply a greenfield visual system. The current sidebar components, semantic tokens, and supplied screenshot are authority for this bounded rearrangement.
- A collaboration notification route does not yet exist. Existing notifications are a protected popover; a sidebar trigger can reuse it without introducing a fake list route or changing parser semantics.
- No cross-workspace account search exists. Do not rename workspace search to global search.
- This research ran no builds, tests, or browser sessions and made no product-code edits.
