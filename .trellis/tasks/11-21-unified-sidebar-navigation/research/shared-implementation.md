# Shared sidebar implementation

- Date: 2026-11-21
- Owner: `shared_sidebar_implement`
- Scope: shared sidebar, Core disclosure preferences, existing collaboration controls and bilingual navigation labels. No staging or commits performed.

## Delivered behavior

- `AppSidebar` is account-safe. It renders the Collaboration directory before the Workspace directory and keeps real workspace query/mutation hooks in a separately mounted `WorkspaceNavigation` child. Remembered workspaces are navigation previews only, using explicit paths; capability previews use disabled cache-only Queries. No workspace identity/provider is fabricated.
- Public sidebar props: optional `navigationWorkspace?: Workspace | null`, `onWorkspaceSelect?(slug)`, `onCollaborationResume?()`, `workspaceListUnavailable?: boolean`, `onRetryWorkspaces?()` plus existing slots. The workspace heading invokes `onWorkspaceSelect(selected.slug)` so platforms can resume it; other dropdown choices use the same adapter callback.
- The nav landmark is `Application navigation` / `应用导航`. Collaboration resume has the exact accessible name `Collaboration` / `协作空间`. Workspace resume has the exact `workspace.name`. The picker is `Switch workspace` / `切换工作空间`. Scope containers use `data-sidebar-scope="collaboration"` and `"workspace"`.
- Current handling and Participating are explicit account list links. The selected view retains its highlight while a detail is open. Account query/hash navigation dismisses the compact sheet.
- Collaboration notifications reuse the existing protected Query, popover, paging, exact links and read/archive mutations with a customizable sidebar trigger. Unknown counts do not become zero. Outside account scope, existing generic WS hooks invalidate only the notification Query; they do not install another collaboration issue binder. Account workbench remains the issue realtime owner.
- New combines the existing ordinary creation action and existing collaboration creation dialog. Ordinary creation is disabled with an explanation outside a real workspace; account collaboration leaves source selection explicit. The existing collaboration entry now supports controlled opening and returns focus to New after close. Its ordinary uncontrolled entry is preserved for source/standalone flows.
- Work defaults open; AI and Resources default closed. A Core StorageAdapter-backed store persists only validated boolean disclosure choices. Active child navigation reveals its group without persisting the automatic reveal; manual collapse works until the next route.
- `AccountWorkbenchPage` accepts `navigationInSidebar?: boolean` (default false). Full account shells pass true, giving the page its selected-view title and removing duplicate tabs/create/notification controls; standalone tests retain their existing toolbar contract.
- Existing pins, unread/chat exclusion, gates, Help/version forwarding and footer ownership remain covered. Settings now states its Workspace scope.

## Files owned

- `packages/core/navigation/sidebar-preferences.ts` and `.test.ts` (new); `navigation/index.ts` exports.
- `packages/views/layout/app-sidebar.tsx`, `.test.tsx`; `sidebar-navigation-group.tsx` and `.test.tsx` (new).
- `packages/views/collaboration/notifications.tsx`, `.test.tsx`; `creation-form.tsx`, `.test.tsx`; `account-workbench-page.tsx`, `.test.tsx`.
- `packages/views/locales/en/layout.json` and `zh-Hans/layout.json`: eight added sidebar keys each. A deep JSON comparison with the task's captured baseline proved every pre-existing field unchanged, including the unrelated dirty marketplace copy. Stage only task-owned locale additions.

`packages/views/layout/dashboard-layout.tsx` is the Web agent's separate small sidebar-slot change, not this lane's edit.

## Verification evidence

- Regression red: `pnpm --filter @multica/core exec vitest run navigation/sidebar-preferences.test.ts` initially failed because the new module did not exist.
- Core preferences: final focused suite passed **3 tests**; Core typecheck and focused navigation ESLint passed.
- Shared combined run: `pnpm --filter @multica/views exec vitest run layout/app-sidebar.test.tsx layout/sidebar-navigation-group.test.tsx collaboration/notifications.test.tsx collaboration/account-workbench-page.test.tsx collaboration/creation-form.test.tsx locales/parity.test.ts` passed **6 files, 155 tests** on the final production code.
- One additional read/archive sidebar regression was then added; `pnpm --filter @multica/views exec vitest run collaboration/notifications.test.tsx` passed **5 tests**. Together the selected suites now contain 156 tests; the six-file invocation above was not repeated after that test-only addition.
- Final Views typecheck passed after the final test addition. ESLint passed all ten owned TSX files, followed by the added notification test's focused lint. `git diff --check` passed for all owned paths.
- Initial ESLint identified the literal product brand in JSX; moving the unchanged brand into a named constant resolved it. Existing pnpm configuration warnings were warnings only; no dependencies/configuration changed.

## Handoff and remaining verification

- Production Web/Desktop builds, actual browser/native navigation, compact/200% zoom screenshots, visual verdict, obsolete top-switch E2E selectors, spec/docs sync and selective commit remain with the coordinating session.
- `HelpLauncher` previously lived only in workspace scope and always opens Feedback through the workspace ModalRegistry. The new account sidebar forwards Help/version, but account shells intentionally do not mount that registry. The leader was notified: account feedback needs an account-safe host or a scoped menu action before final acceptance; this lane did not modify HelpLauncher outside its assigned files.
- The leader's provider session was observed as errored in the final agent inventory (provider conversation-turn limit). All task-owned implementation/evidence is on disk for a resumed coordinator; this lane did not widen into final delivery or claim the whole task complete.
