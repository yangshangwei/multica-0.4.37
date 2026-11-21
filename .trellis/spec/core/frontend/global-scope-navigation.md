# Global scope navigation and restoration

## 1. Scope / Trigger

Apply when changing Web/Desktop shell navigation, account destinations, tab
activation, or collaboration scroll/focus. Collaboration is account-owned and
must remain available with zero, pending or unavailable workspaces.

## 2. Signatures

- The shared `AppSidebar` owns the accessible Collaboration and Workspace
  headings. Platform shells provide resume and workspace-selection callbacks;
  they do not render a separate fixed scope-switch row.
- `AppSidebar({ navigationWorkspace?, onWorkspaceSelect?,
  onCollaborationResume?, workspaceListUnavailable?, onRetryWorkspaces?,
  topSlot?, searchSlot?, versionSlot? })` accepts an authoritative directory
  member or null as its navigation preview. Omitted `navigationWorkspace`
  uses the actual route workspace. Selection callbacks receive its slug.
- Core `useSidebarPreferences` persists only `groups.work`, `groups.ai` and
  `groups.resources` booleans through `StorageAdapter`; `NavigationGroup`
  owns active-route reveal and manual collapse for the current full path.
- Core `navigationScope(path)`, `resolveScopePath(state, owner, scope, workspaceSlugs?)`
  and `useScopeHistoryStore` own Web's session-lifetime last destinations.
- Desktop `resumeDesktopScope(scope, workspaceSlugs?)` restores via no-argument
  `switchAccount()` or `switchWorkspace(slug)`. `lastWorkspaceSlug` is persisted
  independently of `activeWorkspaceSlug`, which stays null in account scope.
- `NavigationAdapter.isScopeResume?` suppresses a consumed URL anchor on resume;
  `navigationKey?` distinguishes repeated explicit opens of the same URL.
- `NavigationAdapter.replace(path, { preserveView?: boolean })` allows a caller
  to normalize an alias of the already-resolved resource without replacing its
  mounted view. Only canonical issue URLs currently request this option.

## 3. Contracts

Only global switching consults remembered destinations. Notifications and task
links keep explicit path/query/fragment semantics. Activating the current scope
is a no-op. Do not send a default issue/list path to a global resume operation.

Collaboration appears above Workspace in the shared left directory. Current
handling and Participating are explicit list links, with the selected list
remaining highlighted when its detail is open. The Collaboration heading
resumes the exact account destination. The workspace name resumes the displayed
workspace's destination; its adjacent selector chooses a directory member.

Select the navigation workspace from the actual route, otherwise the remembered
available workspace, otherwise the first authoritative directory member. A
preview must never populate `WorkspaceSlugProvider`, the API workspace header
or the route singleton. Workspace hooks, pins, search, shortcuts and ordinary
creation mount only under the real workspace route. Cache-only capability
queries in the preview use `enabled: false`. Web scope history is session-only:
an account document reload can fall back to the first workspace; Desktop keeps
its existing persisted `lastWorkspaceSlug`.

New is one menu backed by the existing ordinary and collaboration forms. In
account scope ordinary creation is disabled and collaboration creation requires
an explicit source. Workspace-origin collaboration preselects the actual source.
The ordinary shortcut remains workspace-scoped. The controlled collaboration
dialog returns focus to New. Shared notifications keep the protected account
Query and original exact links/read/archive flow. Workspace-side realtime owns
notification invalidation only; the account workbench owns issue subscriptions.

Work defaults open; AI Team and Resources & connections default closed. Persist
only manual boolean preferences, not server data or automatic active-route
reveal. Entering a different child route reveals its group; manual collapse on
the current route remains possible. Clear a path-local manual override when the
path changes: `/agents` collapse → `/squads` → Back to `/agents` must reveal the
active group again, rather than resurrecting the old collapse. Compact sheets close on full-path changes,
including account query-only navigation. Account Help uses `FeedbackModalHost`
from the existing modal registry module; workspace shells use `ModalRegistry`
exclusively, so one feedback dialog is mounted and no workspace modal opens on
behalf of a preview.

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

The account detail container owns its numeric scroll memento, so opt that
container out of browser scroll anchoring with `overflow-anchor: none`. A cached
read-error banner can remain during restoration and disappear after a successful
background refetch; native anchoring otherwise moves the restored offset by the
banner's height and gap, even when the scroll range still permits the saved
position. Keep this policy local to the account detail root, not the generic
restoration hook or every application scroller. The tradeoff is that later
layout changes in this container no longer receive automatic anchor adjustment;
explicit comment/run/description navigation still owns its deliberate scroll.
Do not compensate with a timer or repeated writes of the saved position.

An issue UUID and its human-readable identifier name the same resource. After
resolving that identity and participation, canonicalization preserves the query
and fragment and calls `replace(canonicalHref, { preserveView: true })`. The Web
adapter uses Next's supported `history.replaceState` integration for same-origin
aliases, keeping its route tree, open picker and pending editor mounted. The
shared provider forwards the option; native SPA adapters keep their existing
replacement behavior. Never request `preserveView` to open a different resource
or workspace. Ordinary `replace` calls still use the platform router.

The shared sidebar remains outside the content canvas and owns both scope
headings. Desktop's toolbar sits in the top 48px with 92px traffic-light
clearance, no-drag sidebar/history controls and 200px tab clearance when the
sidebar is hidden. Its sidebar top spacer remains draggable. Do not reintroduce
a fixed scope-switch row, footer switch, or content-level workspace selector.

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
| Resolved UUID becomes its canonical issue identifier | Preserve mounted controls, query and fragment; replace history entry |
| Navigation to another resource | Use ordinary routing; never retain the outgoing view with `preserveView` |
| Account previews a remembered workspace | Render explicit workspace links without installing its identity or fetching workspace resources |
| Account page reload on Web | Use the first authoritative workspace if session history no longer exists |
| Group entered through a child route | Reveal it without persisting an automatic preference |
| Compact account query changes | Dismiss the sidebar sheet after destination commit |
| Account Help opens Feedback | Render one feedback host; keep workspace-only modals unmounted |

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
tests own sidebar keyboard/current state, group preferences and anchor/scroll
behavior. `layout/app-sidebar.test.tsx` covers preview query gating, pins and
unread counts; `modals/feedback-host.test.tsx` and platform shell tests cover
account modal ownership. Production
Web and actual Electron navigation journeys live in
`e2e/global-scope-navigation.spec.ts`; record build identity and exact native
harness scope. Class assertions or JSDOM offsets are not visual/native proof.

The browser journey targets the workspace-name heading, never the removed
Workspaces scope button. In compact layouts open the actual sidebar trigger
before selecting a destination and assert that query-only view changes close
the sheet. Before compact captures, await the actual responsive branch, the
sheet's final in-viewport geometry and its finite opening animations. An
attached or visible navigation landmark alone can still be the outgoing desktop
branch or an offscreen sheet in transition; it is not painted layout evidence.
Preserve the same-session exact workspace restoration checks; after
a document reload distinguish Web's directory fallback from Desktop's persisted
workspace pointer. `E2E_PASSWORD_AUTH=1` uses synthetic password registration
and the public onboarding APIs, with isolated-account cleanup; the legacy
email-code branch remains available.

The scope-navigation browser regression injects a failed issue read, resumes the
cached detail at 620, drains earlier handlers, then activates the current error
banner's Refresh control with Enter. Focus it with `preventScroll` so an offscreen
pointer click does not move the offset. Hold that GET, assert 620, and release it
with `route.continue()`. Match the browser response by Request identity and require
200 and completed transport before error removal, painted frames and the same
offset on Web and actual Electron. In the native harness, synthesized
`route.fulfill` responses can surface status 0 even when `route.fetch()` returned
200; that upstream result alone does not prove recovery. A check that finishes
before the banner disappears can pass while the later position is wrong. Retain
the complete detail journeys and their exact restoration assertions.

Web document reloads process the URL's explicit comment anchor; wait for its
actual focus before choosing a screenshot position. Desktop rehydration restores
the persisted account tab with `isScopeResume: true`, so it must not be required
to refocus the old anchor. Keep explicit native deep-link focus assertions and
all screenshot geometry checks separate from reload/memento restoration.

`e2e/canonical-issue-interaction.spec.ts` holds the former canonical route
request, opens the status picker before normalization completes, and verifies
that the original trigger stays mounted and one write reaches the correct issue.
Check Back/Forward and navigation to a second issue before reloading; reloading
first replaces the retained UUID tree and misses that history boundary. Verify
reload persistence separately. Do not hide this race with `networkidle` or a
fixed delay before clicking a visible control.

## 7. Wrong vs Correct

Wrong: `nav.push(accountPath())` for the global Collaboration button and
`nav.push(paths.workspace(slug).issues())` for its Workspace counterpart.

Correct: use the platform resume operations; keep explicit `push(path)` for
notification/task links. Reuse existing restoration adapters rather than keeping
both application trees mounted or adding a second scroll state store.

Wrong: canonical UUID-to-identifier normalization through Web `router.replace`,
which changes Next's dynamic segment identity and closes an in-use picker.
Correct: use the explicit same-resource `preserveView` option after resolution.
