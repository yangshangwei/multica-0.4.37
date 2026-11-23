# Research: Desktop global scope navigation and restoration

- Query: Find the minimum change for persistent top-left Workspace/Collaboration switching, preserving each scope’s last position.
- Scope: internal
- Date: 2026-10-28

## Findings

### Existing behavior and responsible files

| File | Responsibility |
| --- | --- |
| `apps/desktop/src/renderer/src/components/desktop-layout.tsx` | Persistent authenticated shell, native window toolbar, workspace sidebar gate, tabs, content canvas. |
| `apps/desktop/src/renderer/src/components/account-navigation.tsx` | Current always-visible content-level collaboration link, workspace selector and workspace recovery/create controls. |
| `apps/desktop/src/renderer/src/stores/tab-store.ts` | Persisted workspace tab groups and one account tab, per-tab URL/history/memento, Server/viewer isolation. |
| `apps/desktop/src/renderer/src/platform/navigation.tsx` | NavigationAdapter, account/workspace destination dispatch, overlay interception, ambient-workspace clearing. |
| `apps/desktop/src/renderer/src/platform/tab-coordinator.ts` | Single router projection and synchronous outgoing scroll capture before host unmount. |
| `apps/desktop/src/renderer/src/components/tab-content.tsx` | One active tab host, restoration provider, remount on tab changes. |
| `apps/desktop/src/renderer/src/platform/account-bootstrap.ts` | Keeps account accessible independently of workspace-list readiness/failure. |
| `apps/desktop/src/renderer/src/platform/account-workbench-route.tsx` | Account route validation, auth gates, workspace-mirror clearing, shared workbench mounting. |
| `packages/views/collaboration/account-workbench-page.tsx` | Existing account view, search, sort, list/detail scroll restoration. |
| `packages/core/paths/account.ts` | Canonical account URL and accountRouteKey; no new account route required. |

### Restoration contracts that already work

- `tab-store.ts:528`: `switchAccount()` with no path selects the existing account tab’s full URL, or the canonical default on first use. An explicit path navigates the account tab to that target. It preserves account tab identity, history and memento.
- `tab-store.ts:539`: `switchWorkspace(slug)` with no openPath restores that group’s active tab. Supplying a path deduplicates/opens that resource; the current selector supplies `/slug/issues`, explaining the return-to-home behavior.
- `tab-store.ts:131`: activeScope is `account | workspace | none`; `switchAccount` deliberately sets activeWorkspaceSlug to null. There is **no remembered last workspace slug**. Object insertion order of byWorkspace is not MRU and must not select the return destination.
- `tab-store.ts:1019`: persistence stores activeScope, activeWorkspaceSlug, accountGroup, sessionOwner and byWorkspace. `mergePersistedTabs` at approximately line 1149 reconstructs workspace state, but an account-active session again returns activeWorkspaceSlug=null. A new last-workspace marker must survive serialization and account-active restoration.
- `tab-store.ts:516`: identity changes wipe tab groups. Any added marker must reset here and in reset, and be validated/pruned with validateWorkspaceSlugs (line 949). Existing profiles can derive a first fallback from a valid persisted activeWorkspaceSlug, then valid saved groups; existing account-active profiles cannot recover actual historical MRU because it was never recorded.
- `tab-content.tsx:13`: inactive tabs are pure saved state; only one RouterProvider host is mounted. Do not keep both page trees alive or introduce a second router.
- `tab-coordinator.ts:240`: host/URL transitions synchronously capture outgoing scroll to the previous tab before React unmount. Its adapter already accepts account tabs.
- `account-workbench-page.tsx:62`: search and sort read/write `account-search` and `account-sort` through the existing view-state adapter. List and detail roots already have `data-tab-scroll-root` and restoration hooks. `accountRouteKey` keeps separate state per account view/issue while excluding deep-link fragments. No additional desktop filter/scroll store is needed.
- `collaboration-detail.tsx:41`: detail restoration retries after data/comments load; explicit run/comment/section anchors retain priority.

### Recommended minimal platform boundary

1. Shared global scope UI belongs in packages/views and receives scope state/handlers or a narrow platform capability. Desktop wiring belongs in platform. Do not add new stores in packages/views; do not move/rebuild the existing desktop tab store as part of this feature.
2. Desktop global Collaboration action uses no-argument account restoration. Clear `setCurrentWorkspace(null, null)` **before** `switchAccount()` so account reads cannot inherit workspace headers. Existing explicit `push(accountPath(...))`, native deep links, notification links and issue popouts keep their precise target behavior.
3. Remember the last workspace selection as an identity-scoped persisted pointer, then invoke `switchWorkspace(lastValidSlug)` without a URL for the global Workspace action. Keep marker updated for every actual workspace activation, including explicit cross-workspace links. Do not repurpose activeWorkspaceSlug: null in account is a correctness contract used throughout shell/bootstrap.
4. For no remembered valid workspace, use a valid available workspace’s normal default; with zero workspaces route to the existing workspace creation overlay. A pending/failed workspace list must not disable or unmount Collaboration. Workspace retry/create affordances can live in the workspace entry’s secondary UI or empty workspace area, but removing the current navigation bar must not remove their only reachable surface.
5. Put the switch above the conditional workspace content, outside MainCanvas and outside AppSidebar’s collapse lifecycle. A topSlot inside the existing AppSidebar alone fails because the sidebar disappears in account scope and compact/collapsed layouts. Remove both current duplicated entry points only once the persistent switch is mounted in every authenticated shell state.

### Layout constraints

- `desktop-layout.tsx:83`: native toolbar is fixed x=0, y=0, width 184px, height 48px. It reserves traffic lights (70px), sidebar toggle and history controls. Buttons are no-drag.
- `desktop-layout.tsx:155`: MainTopBar animates 184px clearance only when sidebar is hidden/compact. Account has no workspace sidebar, so that offset prevents tabs from colliding with native controls.
- `desktop-layout.tsx:298`: workspace chrome is gated on both activeScope and a current slug present in the authoritative workspace list. Keep this gate; stale singleton values previously crashed SearchCommand and blanked the renderer.
- `desktop-layout.tsx:350`: old account navigation lives inside content canvas. Remove it there after lifting global switching into the shell.
- The fixed global switch must keep the same screen coordinates across both scopes and sidebar collapse, and coexist with traffic lights, history controls, TabBar and reduced-motion geometry. A separate persistent shell row is lower risk than inserting additional controls into the already-full 184px native toolbar; whichever design is chosen, include Sidebar’s fixed positioning in the geometry calculation.
- Compact/narrow/200% zoom must retain both text labels or an accessible overflow design, with no horizontal document overflow. Keep selected state identifiable on hover and keyboard focus visible.

### Canonical tests and suggested additions

| Test | Existing coverage / needed extension |
| --- | --- |
| `apps/desktop/src/renderer/src/stores/account-tab.test.ts` | Account history, single-tab reuse, owner changes, migration, workspace eviction. Add last-workspace persistence/reset/prune matrix and no-argument roundtrip. |
| `apps/desktop/src/renderer/src/stores/tab-store.test.ts` | Workspace restore without openPath, pins/MRU, memento. Reuse instead of duplicating its full matrix. |
| `apps/desktop/src/renderer/src/platform/navigation.test.tsx` | Add resume operation vs explicit destination distinction, workspace header clearing before account publication, no duplicate history from clicking current scope. |
| `apps/desktop/src/renderer/src/platform/tab-coordinator.test.ts` | Existing account route and scroll capture; extend only if new switching path changes its contract. |
| `apps/desktop/src/renderer/src/components/desktop-layout.test.tsx` | Native toolbar geometry, single sidebar toggle, reduced motion. Update geometry expectations and verify global switch outside canvas/collapse. |
| `apps/desktop/src/renderer/src/components/desktop-layout.workspace-gate.test.tsx` | Stale/zero/pending workspace gate. Verify persistent global switch survives these states, without mounting workspace-only providers. |
| `apps/desktop/src/renderer/src/platform/account-bootstrap.test.ts` | Pending/error/zero workspace, auth guards. Preserve behavior. |
| `e2e/collaboration-account-desktop.spec.ts` | Real Electron account history/scroll/restart, no account workspace headers, zoom. Existing selectors expect old Collaboration nav and Workspaces combobox and must be updated when replaced. |

Acceptance journey: workspace detail (in a non-first workspace, ideally non-default tab) → Collaboration → choose participating/detail, filter/sort and scroll → Workspace (one click restores exact tab/detail) → Collaboration (one click restores selected issue/view/filter/sort/scroll). Add explicit notification link to a different issue to prove deep-link priority. Include restart while account-active and owner switch.

Suggested focused verification: `pnpm --filter @multica/desktop test -- <focused test paths>` (check script argument behavior), `pnpm --filter @multica/desktop typecheck`, `pnpm --filter @multica/desktop lint`, followed by the existing Electron E2E with its isolated renderer/API setup.

## Related Specs

- `.trellis/spec/desktop/frontend/account-navigation.md` is the authoritative account route/persistence/header/native contract.
- `.trellis/spec/desktop/frontend/quality-guidelines.md` contains isolated Electron acceptance prerequisites.
- `CLAUDE.md` supplies package boundaries, state ownership, no new dependencies and desktop overlays/navigation rules.

## External References

None needed: this design extends existing repository adapters/stores without unfamiliar SDKs.

## Caveats / Not Found

- This was source research only; no product edits or tests run.
- Existing last-workspace MRU is not persisted, so old account-active profiles need a deterministic first-run fallback.
- Many generic desktop spec files remain placeholders; rely on CLAUDE.md and account-navigation.md for operative rules.
- Several existing E2E suites use the removed nav/combobox selectors. Search all E2E callers before deletion; changing only the principal test leaves others stale.
