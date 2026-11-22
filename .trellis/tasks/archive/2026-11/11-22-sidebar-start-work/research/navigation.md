# Sidebar creation across account and workspace scope

## Existing owners

- `packages/views/layout/app-sidebar.tsx`: `AppSidebar` distinguishes the actual route workspace from its authoritative navigation preview. `SidebarCreation` currently disables ordinary creation in account scope, but gives the preview UUID to `CollaborationCreationEntry` separately.
- `packages/views/navigation/types.ts`: `NavigationAdapter.push(path)` returns `void`. It schedules navigation and provides no promise proving a destination committed.
- `packages/views/navigation/context.tsx`: `NavigationProvider` wraps pushes in React transitions. Desktop's tab-store write can settle before its actual route subtree mounts, so `useIsNavigating()` alone is not a workspace-ready signal.
- `apps/web/platform/scope-shell.tsx`: `WebScopeShell` chooses route workspace, remembered available workspace, then first authoritative member. It never grants preview workspace authority. Its selection callback resumes remembered destinations; a content action must use an explicit adapter push to its destination instead.
- `apps/web/platform/account-shell.tsx`: account chrome mounts `FeedbackModalHost`, never ordinary workspace modals. The workspace dashboard mounts `ModalRegistry` through shared `DashboardLayout`. These route layouts can remount the sidebar, so a local sidebar ref/state cannot carry the action reliably.
- `apps/desktop/src/renderer/src/platform/navigation.tsx`: `tryRouteToOtherWorkspace` already delegates an adapter push to `switchWorkspace(slug, path)`. Do not introduce platform routing imports into Views.
- `apps/desktop/src/renderer/src/components/desktop-layout.tsx`: actual `WorkspaceRouteLayout` establishes the platform singleton; only then does desktop shell chrome receive a real `WorkspaceSlugProvider` and mount `ModalRegistry`. It keeps account previews outside that authority.
- `packages/core/modals/store.ts`: existing nonpersisted Zustand store; currently no queued create intent.
- `packages/core/issues/stores/create-mode-store.ts`: `openCreateIssueWithPreference` owns the last-mode choice and opens the existing modal. Its store is persisted; a transient intent added here would require explicit partialization, so the nonpersisted modal store is a cleaner existing owner.
- `packages/core/api/client.ts`: `getSessionScope()` returns endpoint/credential epochs. Capture it with the API base URL and viewer UUID to reject stale actions after server/session replacement, even if the same viewer later returns.

## Recommended small approach

1. With an actual matching workspace, retain the existing immediate `openCreateIssueWithPreference()` call.
2. From account scope, capture the selected authoritative workspace UUID/slug, full source location and API/viewer/session ownership into a transient pending descriptor in the existing nonpersisted Core modal store, then call `navigation.push(paths.workspace(slug).issues())`.
3. Consume that descriptor in a workspace-only host such as `ModalRegistry`, after its actual route/provider UUID and current pathname match the selected target. Verify current API/viewer/session ownership and the live platform workspace identity as well. Clear/consume the slot atomically before calling the existing preference-backed open function so StrictMode cannot open twice.
4. A different explicit action, another modal open/close, mismatched committed destination, or identity change must retire the old descriptor. While the route still displays the original account location, leave the descriptor pending; do not use timers, `await navigation.push`, or a provider constructed from the preview.
5. Keep the request transient and do not add a route, localStorage, a second store, or dependencies. Chat requires only an explicit link to `paths.workspace(slug).chat()`; existing platform adapters already make that cross-scope transition.

Opening the global modal store before navigation could be invisible in account scope, but it prematurely publishes an ordinary modal and lacks target/identity ownership. A bare `?create=issue` query is also possible but adds a public replayable route intent and does not inherently fence the initiating viewer/session. A target-bound transient descriptor preserves the explicit user action across Web remounts with less routing change.

## Required regressions

- Account preview alone never opens ordinary forms or starts workspace-owned queries.
- A delayed account-to-workspace navigation leaves creation pending until the actual target provider mounts.
- Web sidebar remount still opens exactly once in the intended workspace and preserves last creation mode.
- A different workspace, a cancelled/superseded navigation, viewer/server/session replacement, or another modal action cannot consume stale creation.
- A missing workspace keeps account collaboration usable and explains why ordinary creation/chat need a workspace.
- Collaboration still preselects the displayed workspace, permits explicit source changes and returns focus to its visible direct creation button.
- Chat has one visible sidebar link, retains current-workspace unread semantics and cannot use a preview as API authority.

## Existing E2E edits

- `e2e/fixtures/sidebar-navigation.ts:9`: `sidebarNewButton` matches only `New|新建`. Its existing name is imported by visibility-only checks in auth, issues, helpers, preferences and workspace-access tests; it can keep its exported helper name while targeting the direct ordinary creation button, avoiding unrelated caller churn.
- `e2e/fixtures/sidebar-navigation.ts:44`: `openNewIssue` currently clicks `New`, then a `menuitem`. Replace the two-step interaction with the direct button.
- `e2e/fixtures/sidebar-navigation.ts:50`: `openNewCollaborationIssue` likewise needs the direct visible collaboration action and the new localized label (`Start collaboration|发起协作`, if selected by the main plan).
- `e2e/global-scope-navigation.spec.ts:180`: workspace state currently opens the New menu and asserts two enabled menuitems plus Beta. Replace with visibility/enabled/description assertions on direct actions; preserve the original geometry, disclosure and history journeys.
- `e2e/global-scope-navigation.spec.ts:203`: account state currently expects ordinary creation disabled. New contract should assert direct action is enabled for the displayed authoritative workspace and test account-to-workspace creation separately before returning to the original collaboration view.
- `e2e/global-scope-navigation.spec.ts:216`: collaboration dismissal expects focus on `New`. Target the direct collaboration button.
- This E2E file has no direct Chat selector today. Add focused header/context and single-link checks instead of duplicating the full existing chat suite.

All findings are read-only; no product or test files were changed by this research agent.
