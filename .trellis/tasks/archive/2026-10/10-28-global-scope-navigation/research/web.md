# Research: Web global scope navigation and position restoration

- Query: Identify the smallest Web implementation for a fixed Workspace / Collaboration switch that resumes each scope, while preserving explicit deep links, identity isolation, and zero-workspace account access.
- Scope: Internal source, tests, and project specifications; no product changes.
- Date: 2026-10-28

## Findings

### Current route and shell boundaries

| File | Current responsibility |
| --- | --- |
| `apps/web/components/web-providers.tsx` | Persistent provider tree: `CoreProvider` -> `WebNavigationProvider` -> `WebScrollRestorationProvider` -> account setup/password gate -> route children. |
| `apps/web/platform/navigation.tsx` | Only platform adapter for Next router, pathname/search/hash, browser history and explicit internal-link events. |
| `apps/web/platform/account-route.tsx` | Authentication-only account guard; clears workspace singleton synchronously before account children render. |
| `apps/web/platform/account-workbench-route.tsx` | Parses `/account`, sets title, sends invalid account locations to default account route. |
| `apps/web/app/account/layout.tsx` | Mounts `AccountRoute`; currently has no application shell or global return control. |
| `apps/web/app/[workspaceSlug]/layout.tsx` | Authentication/onboarding and workspace resolution; protects singleton writes against outgoing Next layouts. |
| `apps/web/app/[workspaceSlug]/(dashboard)/layout.tsx` | Web-specific search, notifications and chat slots for shared dashboard layout. |
| `packages/views/layout/dashboard-layout.tsx` | Shared workspace sidebar plus main canvas inside `SidebarProvider`. |
| `packages/views/layout/use-dashboard-guard.ts` | Dashboard validation and legacy pathname-only navigation-store writes. |

The global switch must live outside the collapsible workspace sidebar and outside the account list/detail headers. A small authenticated shell wrapper can be mounted around the relevant Web route content using the existing persistent provider boundary; it must not display on login, onboarding, invitations, admin, or unrelated public routes without an explicit design decision. Alternatively, route-specific shell mounts can share one presentational component, provided the coordinate and reserved height match in both scopes. Do not reorganize the Next route tree merely to gain shared state: the provider boundary already exists (`web-providers.tsx:113–116`).

If a top shell band is introduced above `DashboardLayout`, replace its viewport-sized root and loading fallback with parent-fill sizing. It currently uses `h-svh` (`dashboard-layout.tsx:39,45`); leaving that intact adds a viewport below the band and clips scrollable content. Sidebar fixed positioning/height must be considered with the shared `SidebarProvider` implementation.

### Existing location memory cannot serve the new switch

`packages/core/navigation/store.ts:25–49` stores one `lastPath` in workspace-aware persistence. The only current production writer is `use-dashboard-guard.ts:64–66`, which records `pathname` and drops both query and fragment. No production reader of `lastPath` was found. It cannot identify the last workspace after entering account context because `AccountRoute` clears the active workspace mirror, and `createWorkspaceAwareStorage` deliberately reads null/drops writes when no workspace is active (`workspace-storage.ts:94–115`).

Minimal recommended contract:

1. Keep ordinary `push`, `replace`, notification links, editor `multica:navigate`, and `AppLink` behavior unchanged. They describe explicit destinations.
2. Add dedicated scope-resume behavior to platform navigation/shell wiring. The shared switch receives active scope and actions/links; it does not reach into Next or Electron stores.
3. Web remembers the complete committed `currentPath(navigation)` independently for account and workspace, with the last workspace slug. Capture query and fragment, not just pathname. Store only navigation/client data in `packages/core/navigation/`; no workspace objects or server projections.
4. Do not key global return memory by the active-workspace singleton. Scope it by Server + viewer, optionally browser session; reset when identity ends/changes. Web platform owns any browser-only storage adapter.
5. A global Workspace click resumes the remembered path when its slug is still in an authoritative workspace list. Otherwise choose a valid workspace's normal default route. Never treat failed/pending workspace discovery as authoritative empty membership.
6. With a successfully empty workspace list, account content stays available; Workspace can lead to the existing workspace creation/onboarding affordance as agreed in the design. No global redirect should force account users to create a workspace.
7. A global Collaboration click resumes its last valid account destination, falling back to `accountPath()` only on first visit or invalid memory. Clicking the already active scope is a no-op.

`currentPath()` already preserves pathname + query + fragment (`packages/views/navigation/current-path.ts:12`). `parseAccountPath` strictly validates destinations and `accountRouteKey` identifies view + selected issue + administration without navigation anchors (`packages/core/paths/account.ts:20,64`). Workspace parsing should reject reserved/global paths and malformed or external URLs rather than remember arbitrary strings.

### Scroll and workbench state mostly already exist

`packages/views/collaboration/account-workbench-page.tsx` already owns the relevant restoration protocol:

- `WorkbenchBody` is remounted by Server/viewer plus `accountRouteKey` (`:60`).
- Search and sort are initialized from `account-search` / `account-sort` view-state entries and written when changed (`:69–74,93–96`).
- `account-list` and `account-detail` scroll containers are marked for capture (`:75–82,102,120`).
- List restoration waits for list data (`:79–82`).
- Detail restoration additionally waits for issue and comments (`collaboration-detail.tsx:40–46`).

The Web adapter currently keys all entries by **pathname only** (`apps/web/platform/scroll-restoration.tsx:52–53`). Thus `/account?view=current&issue=A` and `/account?view=participating&issue=B` collide for search, sort, list scroll, and detail scroll. Desktop's existing account-route-key semantics should be reused: account mementos use `accountRouteKey(fullPath)`; workspace mementos keep their current pathname-based contract to avoid unrelated list-filter behavior changes.

Capture and serve must use the same canonical account key. Capture currently reads `window.location.pathname` while serving reads Next's rendered pathname (`scroll-restoration.tsx:70,101–126`). Preserve the existing distinction: Next can render incoming children before browser history commits. Add search params to both sides as appropriate instead of replacing every read with `window.location`.

The adapter's module maps (`savedOffsets`, `savedViewState`, `suppressedUntil`) have no identity boundary (`:31,40,49`). Adding scope restoration must close that gap: a viewer/server change must not serve old search text, selected-route offsets, or pending view state. Ensure synchronous read isolation, not merely eventual clearing in an effect. Do not call `useCollaborationScope` solely to identify a global shell: it also calls `retainCollaborationScope` and prunes query/mutation/draft state (`packages/core/collaboration/queries.ts:13–22`). The pure `collaborationScope(userId)` helper exposes a session-sensitive `apiScope`, but stable navigation persistence should distinguish durable Server/viewer ownership from its transient credential epoch.

The existing 1-second restore-write shield prevents browser-clamped restoration from destroying stored offsets (`scroll-restoration.tsx:49,72–75,101–107`); preserve it. Its tests already establish this behavior.

### Explicit anchor versus resume is a real edge case

`CollaborationDetail` skips normal restore whenever comment/run/description/memory anchor is present, then applies that anchor on remount; `consumedAnchor` is only component-local (`collaboration-detail.tsx:39–57`). After a user opens a notification, scrolls elsewhere, leaves, and resumes Collaboration, the anchor can win over their saved position.

The design should explicitly distinguish fresh explicit navigation from global resume, or persist consumed-anchor state with a way to re-apply a newly requested explicit link. Do not indiscriminately strip the fragment from remembered account paths: memory-version and run anchors carry meaningful selected-content semantics. Existing issue-detail restoration hooks provide a local reference for consumed deep-link state, but account behavior needs its own targeted wiring test.

### Account access and failure contracts

- `AccountRoute` must remain independent of workspace discovery. Its test deliberately makes `useWorkspaceList` throw if account guarding calls it (`apps/web/platform/account-route.test.tsx:11`). A global navigation component may observe the list for Workspace affordances, but must never gate account children on it.
- Restricted account-setup/password sessions must not render private account content (`account-route.tsx:21`; `web-providers.tsx:51–58`).
- `useWorkspaceList` uses `data !== undefined` for readiness, not `isFetched`; an initial request error is not an empty account (`packages/core/workspace/hooks.ts:35–60`).
- The workspace layout already guards stale outgoing-layout singleton writes by the live pathname (`apps/web/app/[workspaceSlug]/layout.tsx:54–78`). Preserve this during cross-scope transitions.
- `clearClientSessionData` clears workspace storage and desktop tab persistence but does not clear the Web scroll maps or any future global navigation key (`packages/core/platform/session-cleanup.ts:31–77`). New persistence must have explicit cleanup or owner isolation.

## Suggested implementation ownership and validation

Shared/Core lane: global scope switch UI and translations; minimal account/workspace location memory and pure route selection helpers; tests for identity, reserved routes, complete query/hash, remembered workspace removal, and no-op active-scope behavior.

Web lane: shell placement, adapter tracking/resume, account wrapper sizing, account route-key and owner isolation in scroll provider. Keep Next imports in `apps/web/platform/` for new logic. Existing app layout violations do not authorize adding more.

Canonical tests to extend:

- `packages/core/navigation/store.test.ts` or a sibling focused scope-memory suite: pure transition and ownership matrix; Node environment for non-DOM tests.
- `apps/web/platform/navigation.test.tsx`: full path tracking and explicit destination behavior.
- `apps/web/platform/scroll-restoration.test.tsx`: independent current/participating/issue entries; query-aware capture/serve; identity reset; keep rendered-route versus browser-history and clamped-scroll protections.
- `apps/web/platform/account-route.test.tsx`: zero-workspace/pending/failure must not block account; setup/password/auth handling remains unchanged.
- `packages/views/collaboration/account-workbench-page.test.tsx`: one restore wiring scenario for selected detail + search/sort; do not duplicate the platform matrix here.
- New shared switch test: exactly two accessible scope controls, active-state semantics, stable actions; sidebar footer no longer owns another global collaboration entry.
- Browser acceptance: actual task/project detail -> global Collaboration -> selected collaboration detail -> global Workspace restores exact task/project -> global Collaboration restores selection/search/sort/list+detail scroll. Add explicit notification anchor and zero-workspace journeys. Test normal and collapsed sidebar/narrow layout.

Use targeted package Vitest tests during implementation, then relevant typecheck/lint. E2E must use production `next build` + `next start` as required by `.trellis/spec/web/frontend/e2e-run-environment.md`; `next dev` cold-route compilation is not acceptance evidence.

## Related Specs and References

- `CLAUDE.md`: client/server state separation, package boundaries, common Web/Desktop UI, route-driven workspace identity, tests follow their canonical layer.
- `.trellis/spec/core/frontend/collaboration-workbench.md`: scoped workbench and protected account reads; preserve denial/late-response fences.
- `.trellis/spec/web/frontend/state-management.md`: restricted sessions resume existing route guards; forms must not perform late competing navigation.
- `.trellis/spec/views/frontend/component-guidelines.md`: shared UI conventions.
- `.trellis/spec/web/frontend/e2e-run-environment.md`: production Web evidence.
- External references: none needed; all proposed mechanisms use established Next.js, Zustand, and repository restoration APIs.

## Caveats / Not Found

- No UI was launched and no tests were executed in this read-only research slice.
- Existing Web scroll/view-state maps intentionally reset on full page reload. The current request clearly requires in-session round trips; cross-reload scroll restoration would be additional scope. Persisting last destinations and preserving in-session mementos can remain distinct decisions.
- Existing workbench search/sort are per account route key on Desktop, not a single global filter across every selected issue. Preserve parity unless the design intentionally changes that behavior.
- Account administration is embedded under `/account?administration=collaboration` on Desktop but linked to `/admin/collaboration` on Web. Do not accidentally classify all admin routes as Collaboration when remembering global destinations.
- Unrelated uncommitted changes already exist; this research changed only the task research file.
