# Research: Desktop unified navigation shell

- Query: Replace the global Workspace/Collaboration switch with one persistent left sidebar without changing account/workspace ownership or native navigation.
- Scope: internal
- Date: 2026-11-21

## Findings

### Current ownership and geometry

- `apps/desktop/src/renderer/src/components/desktop-layout.tsx:260` owns `DesktopShell`. `DesktopNavigationProvider` wraps a nullable `WorkspaceSlugProvider`, then the shell and route tree. `TabContent` is deliberately never workspace-gated because the route layout must mount to resolve a workspace.
- `desktop-layout.tsx:296` resolves the platform singleton slug against the current workspace directory and excludes account scope. Keep this **active** workspace gate for SearchCommand, GlobalShortcuts, ModalRegistry, FloatingChat and workspace OS unread integration.
- `desktop-layout.tsx:317` currently renders `DesktopAccountNavigation` above `SidebarProvider`; this extra 48px row is why `WindowToolbar` and fixed sidebar containers start at `top-12`.
- `desktop-layout.tsx:83` owns native history/back/forward and the single external SidebarTrigger. `MainTopBar` (`:155`) reserves 120px when the sidebar is hidden; `MainCanvas` (`:183`) matches the sidebar's inset margin. These continue to work if the unified sidebar is always mounted and `hasSidebar` is always true in the authenticated shell.
- `components/account-navigation.tsx:17` reserves 80px for native traffic lights and marks controls `no-drag`. Removing the scope switch must preserve this native clear area and a `WebkitAppRegion: drag` top strip. The lightest geometry change is to keep the existing 48px strip but remove the switch content; the second toolbar row and `top-12` sidebar offset can then stay intact. A single-row redesign is possible but changes toolbar clearance, sidebar start and native geometry together, adding avoidable regression scope.

### Remembered workspace is not active workspace

- `stores/tab-store.ts:134` already separates `activeScope`, `activeWorkspaceSlug` and persisted `lastWorkspaceSlug`; no new store is needed for the shell's remembered workspace pointer.
- `tab-store.ts:542` `switchAccount(path?)` clears `activeWorkspaceSlug`, remembers the outgoing workspace, and uses `path === undefined` to mark a true scope resume.
- `tab-store.ts:561` `switchWorkspace(slug, openPath?)` resumes that workspace's exact active tab when `openPath` is omitted. Supplying a path opens/deduplicates an explicit destination.
- `platform/navigation.tsx:92` `resumeDesktopScope()` already implements these distinctions, including unknown-directory versus confirmed-empty handling and the create-workspace overlay.
- `platform/navigation.tsx:227` explicit account links clear `setCurrentWorkspace(null, null)` **before** `switchAccount(path)`; `components/workspace-route-layout.tsx:106` prevents outgoing workspace layouts from restoring that mirror while account scope is active.
- `platform/account-workbench-route.tsx:16` also clears the mirror during account rendering. Keep this contract unchanged even when workspace menu links remain visible.

### Critical shared-sidebar boundary

`packages/core/paths/hooks.tsx:67` `useWorkspacePaths()` throws outside a workspace route, while `useCurrentWorkspace()` returns null. The existing `AppSidebar` additionally runs implicit-header requests such as `api.listInbox()` and mounts workspace-owned creation controls. It cannot safely be mounted under `lastWorkspaceSlug` while the API singleton still intentionally has null workspace identity for `/account`.

Recommended API shape for the shared sidebar:

- Always mount one `AppSidebar` shell with account navigation and workspace directory/selection.
- Accept an optional **navigation workspace** or remembered slug for display and `paths.workspace(slug)` link construction. Do not turn this into the route WorkspaceSlugProvider or call `setCurrentWorkspace` from it.
- Accept platform callbacks `onResumeWorkspace(slug)` and optionally `onResumeCollaboration()` so group-level resume preserves tab history. Ordinary leaf links must still use `AppLink`/NavigationAdapter with explicit paths.
- Keep workspace data hooks and actions in a child rendered only when an actual active workspace is available. In account scope, render the remembered workspace's pure navigation links without inbox/chat/pins queries or workspace creation actions, unless those data reads are changed to capture explicit scope in the owning API/query layer.
- Scope-independent collaboration list/notifications/create-collaboration controls remain outside that child and work with zero/pending/failed workspaces.
- Always show the sidebar trigger, including account and zero-workspace states. Compact sidebar behavior stays owned by the existing SidebarProvider/Sheet.

### Minimal Desktop file set

1. `components/desktop-layout.tsx`: always mount unified sidebar; pass `lastWorkspaceSlug` separately from actual `slug`; preserve active-workspace gating of workspace services; keep top native strip/toolbar geometry.
2. `components/account-navigation.tsx`: remove obsolete global scope switch content (either replace with native strip or delete and inline its trivial geometry in the shell). Do not retain a second hidden navigation authority.
3. `platform/navigation.tsx`: only if the shared sidebar callback needs a public workspace-resume function; existing store APIs are sufficient, so avoid broad adapter changes.
4. `components/desktop-layout.workspace-gate.test.tsx`: invert assertions that account/empty directory remove AppSidebar; retain assertions that SearchCommand and workspace actions stay unmounted. Assert exactly one reachable toggle and independent account navigation for zero/pending/failure/stale slug.
5. `components/desktop-layout.test.tsx`: update mocks/API and geometry expectation only if top strip dimensions change; retain reduced-motion interruption and single-toggle regressions.
6. `platform/navigation.test.tsx`, `stores/account-tab.test.ts`: preserve existing resume/direct-link/history/ownership matrices. Add only any new callback wiring regression, rather than duplicating store tests.
7. `e2e/global-scope-navigation.spec.ts`: replace fixed top switch selectors with unified sidebar resume affordances; preserve exact search/sort/both-scroll roots and failed-read recovery assertions.
8. `e2e/fixtures/collaboration-native.ts:73`: `selectDesktopWorkspace()` currently clicks the old Workspaces global button before finding the header menu. Update this shared helper to use the new persistent workspace switcher so all native source journeys continue to work.
9. Other `e2e/*` consumers of `Application navigation` + `Workspaces`/`Collaboration` require bounded selector migration. `e2e/collaboration-account-desktop.spec.ts:704` is one direct consumer outside the helper.

### Verification surfaces

- `pnpm -C apps/desktop exec vitest run src/renderer/src/components/desktop-layout.test.tsx src/renderer/src/components/desktop-layout.workspace-gate.test.tsx src/renderer/src/platform/navigation.test.tsx src/renderer/src/stores/account-tab.test.ts`
- `pnpm -C apps/desktop typecheck`
- `pnpm -C apps/desktop exec eslint src/renderer/src/components/desktop-layout.tsx src/renderer/src/components/account-navigation.tsx` (adjust if the latter is removed).
- `pnpm -C apps/desktop exec electron-vite build` builds native Main/preload/renderer without the unrelated CLI-bundling stage; `pnpm -C apps/desktop build` is the full package script when CLI artifacts are needed.
- Serve the resulting `apps/desktop/out/renderer` through a task-owned static HTTP server and set `COLLABORATION_ELECTRON_RENDERER_URL` to its reachable URL. Existing native harness reads this at `e2e/fixtures/collaboration-native.ts:56` and launches the checkout's actual Electron executable (`:18`).
- With task-owned actual API/database and production Web renderer: `COLLABORATION_E2E_API_URL=<loopback-api> DATABASE_URL=<task-db> COLLABORATION_ELECTRON_RENDERER_URL=<static-renderer> PLAYWRIGHT_BASE_URL=<production-web> pnpm exec playwright test e2e/global-scope-navigation.spec.ts --workers=1`.
- `openDesktop` isolates userData through a test-created profile, strips `ELECTRON_RUN_AS_NODE`, collects page errors and account request headers. `nativeScreenshot()` captures via `webContents.capturePage`, suitable for Retina/native zoom proof; do not substitute browser screenshot claims for actual Electron evidence.

## Related specs

- `.trellis/spec/core/frontend/global-scope-navigation.md`
- `.trellis/spec/desktop/frontend/account-navigation.md`
- `.trellis/spec/views/frontend/desktop-core-accessibility.md`
- `.trellis/spec/views/frontend/core-ui-state-contracts.md`

## Caveats / Not Found

- Current specs explicitly mandate the old fixed two-button row and no sidebar in account scope. The user-approved redesign supersedes that presentation contract; update those paragraphs while preserving the underlying restoration/security contracts.
- Account/notification deep links, native Cmd+W, popout scope and tab coordinator need no redesign. Avoid changing these just to move navigation.
- No native/test environment was started and no verification command was executed for this research. The commands above are recommended verification, not pass evidence.
- I made no product-code changes. All work is confined to this research file.
