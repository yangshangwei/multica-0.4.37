# Web unified sidebar implementation

- Date: 2026-11-21
- Scope: Web shell/route integration and shared DashboardLayout sidebar injection
- Ownership: Web implementation lane; shared sidebar and Desktop are owned by other lanes

## Result

`WebScopeShell` retains the existing identity-scoped history and stable routed tree, replacing its top scope-switch header and sidebar offsets with a Web navigation context. Both the workspace dashboard and account shell pass that context to the shared AppSidebar.

The navigation workspace comes from the active route's directory member, otherwise a remembered available workspace, otherwise the first authoritative directory member. A remembered slug alone never constructs a Workspace or grants workspace context. Empty, pending and unavailable workspace discovery never replace account content.

The workspace section heading resumes the existing full remembered destination through `useWebScopeResume`; selecting another directory member explicitly opens that workspace's issue list. The collaboration heading resumes the exact remembered account URL. Shared explicit list/task/notification links remain ordinary navigation. Current-scope heading actions remain no-ops. Owner checks reject obsolete callbacks, and attempt/owner checks reject late directory fetches. Retry is independent of account content and navigation.

The account route retains AccountRoute above the new WebAccountShell, so its ambient workspace mirror is cleared before the sidebar or private account children render. WebAccountShell mounts SidebarProvider, AppSidebar, SidebarInset and an external compact trigger. It does not mount workspace slug/ID providers, GlobalShortcuts, workspace search, ModalRegistry, presence or floating chat. Workspace-only resources remain in the existing dashboard. The account adapter opts into `navigationInSidebar`, removing duplicated workbench controls through the shared page's existing prop.

## Files

- `apps/web/platform/scope-shell.tsx`: context, selected navigation workspace, restored/explicit callbacks, no top header or 48px offset.
- `apps/web/platform/scope-shell.test.tsx`: platform selection/restoration/error/identity/mounting regressions.
- `apps/web/platform/account-shell.tsx`: account-safe sidebar and compact trigger.
- `apps/web/platform/account-shell.test.tsx`: actual account layout/mirror/provider wiring and usable recovery.
- `apps/web/app/account/layout.tsx`: mount the account shell inside AccountRoute.
- `apps/web/platform/account-workbench-route.tsx`: opt into sidebar-owned navigation.
- `apps/web/platform/account-workbench-route.test.tsx`: explicit participating/run targets, query-only current view and invalid-route normalization.
- `apps/web/app/[workspaceSlug]/(dashboard)/layout.tsx`: inject AppSidebar with Web navigation callbacks and the existing workspace search trigger.
- `packages/views/layout/dashboard-layout.tsx`: optional `sidebar` ReactNode slot, retaining its existing default shared sidebar and all workspace resources.

## Verification

Passed after the implementation:

```sh
pnpm --filter @multica/web exec vitest run platform/scope-shell.test.tsx platform/account-shell.test.tsx platform/account-workbench-route.test.tsx platform/account-route.test.tsx platform/navigation.test.tsx platform/scroll-restoration.test.tsx 'app/[workspaceSlug]/layout.test.tsx'
```

Result: **7 files / 51 tests passed**. The revised scope suite first failed against the incumbent implementation because its context did not exist and the old top header remained. The final run covers exact full URLs, active/remembered/available directory precedence, explicit workspace selection, current-scope no-op, empty/unknown/error membership, late fetches, obsolete owner callbacks, stable page mounting, account mirror clearing and unchanged navigation/scroll contracts.

Also passed: affected Web ESLint (zero warnings/errors), shared `layout/dashboard-layout.tsx` ESLint, Web `tsc --noEmit`, and scoped `git diff --check`.

## Follow-ups and evidence scope

The coordinating session owns the combined shared/Core/Desktop checks, production builds, Web browser journeys and visual QA. DOM tests establish wiring and ownership, not painted compact geometry, locale appearance or native behavior. No commits were created. Pre-existing Web next-env and unrelated dirty files were left untouched.
