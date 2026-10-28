# Global scope navigation and restoration

## 1. Scope / Trigger

Apply when changing Web/Desktop shell navigation, account destinations, tab
activation, or collaboration scroll/focus. Collaboration is account-owned and
must remain available with zero, pending or unavailable workspaces.

## 2. Signatures

- Views `GlobalScopeNavigation({ activeScope, onWorkspace, onCollaboration, children? })`
  owns the accessible two-button switch; platforms own its fixed shell row.
- Core `navigationScope(path)`, `resolveScopePath(state, owner, scope, workspaceSlugs?)`
  and `useScopeHistoryStore` own Web's session-lifetime last destinations.
- Desktop `resumeDesktopScope(scope, workspaceSlugs?)` restores via no-argument
  `switchAccount()` or `switchWorkspace(slug)`. `lastWorkspaceSlug` is persisted
  independently of `activeWorkspaceSlug`, which stays null in account scope.
- `NavigationAdapter.isScopeResume?` suppresses a consumed URL anchor on resume;
  `navigationKey?` distinguishes repeated explicit opens of the same URL.

## 3. Contracts

Only global switching consults remembered destinations. Notifications and task
links keep explicit path/query/fragment semantics. Activating the current scope
is a no-op. Do not send a default issue/list path to a global resume operation.

Web's route history and scroll/view-state maps are Server/viewer/session-owned.
Changing ownership clears the former identity, including if that identity later
returns. An obsolete callback cannot write into the new owner's state. Account
mementos use `accountRouteKey(fullPath)` to distinguish list/view/selected issue;
workspace mementos keep their existing pathname semantics. No server data is
copied into these client stores.

Next may keep the outgoing route mounted after accepting a push. Publish the
new focus intent only with the rendered destination. Otherwise changing a key
on the outgoing anchored detail can refocus it and overwrite its saved scroll.
Same-destination explicit opens must still advance the focus key. Shared detail
restores after data readiness on resume, including URLs with run/comment or
section anchors; explicit opens retain anchor focus. Shared-context version URLs
retain their exact meaning; a global resume must not reopen a dismissed dialog.

The global row is 48px high and outside the content canvas and sidebar collapse.
Viewport-fixed sidebar containers start below it. Desktop reserves 80px for
native traffic lights in that row, with no-drag interactive controls; history
controls move to the next row with 120px tab clearance when the sidebar is hidden.
The switch remains at the same coordinates in both scopes. Do not reintroduce a
footer switch or a content-level workspace selector.

## 4. Validation / Error Matrix

| Condition | Behavior |
| --- | --- |
| Last workspace is in authoritative directory | Restore its exact page/tab |
| Directory is unknown | Keep remembered path; unknown is not membership loss |
| Last workspace was removed | Use an available workspace, preserving account state |
| Confirmed empty directory | Use existing create-workspace flow |
| Directory load fails | Keep account content and retry available |
| Late directory fetch after navigation/identity change | Ignore it |
| Auth/admin/one-shot/external or malformed path | Do not record as a resumable workspace |
| Explicit account target while already in account | Open target; never replace it with history |
| Resume with old anchor after user scrolls | Restore latest offset without refocusing old anchor |
| Explicit same-anchor link repeated | Refocus through a new navigation key |

## 5. Good / Base / Bad Cases

Good: non-first workspace project → participating collaboration detail → one
Workspace click returns to that project; one Collaboration click restores the
detail, search, sort and both scroll roots.

Base: first use enters the existing default list, then remembers later visits.

Bad: deriving workspace MRU from object insertion order; treating a failed list
read as an empty account; using window.location as Desktop route authority.

## 6. Tests Required

Core scope-history tests own valid paths, ownership and membership fallback.
Desktop account-tab tests own pointer migration/persistence/pruning and exact
tab/history/memento roundtrips. Platform tests cover workspace mirror clearing,
resume/direct opens, current-scope no-op and delayed Next route commits. Shared
tests own switch keyboard/current state and anchor/scroll behavior. Production
Web and actual Electron navigation journeys live in
`e2e/global-scope-navigation.spec.ts`; record build identity and exact native
harness scope. Class assertions or JSDOM offsets are not visual/native proof.

## 7. Wrong vs Correct

Wrong: `nav.push(accountPath())` for the global Collaboration button and
`nav.push(paths.workspace(slug).issues())` for its Workspace counterpart.

Correct: use the platform resume operations; keep explicit `push(path)` for
notification/task links. Reuse existing restoration adapters rather than keeping
both application trees mounted or adding a second scroll state store.
