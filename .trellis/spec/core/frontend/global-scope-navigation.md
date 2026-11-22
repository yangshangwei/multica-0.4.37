# Global scope navigation and restoration

## 1. Scope / Trigger

Apply when changing Web/Desktop shell navigation, account destinations, tab
activation, or collaboration scroll/focus. Collaboration is account-owned and
must remain available with zero, pending or unavailable workspaces.

## 2. Signatures

- The shared `AppSidebar` owns the accessible Collaboration and Workspace
  headings. Platform shells provide workspace-selection callbacks;
  they do not render a separate fixed scope-switch row.
- `AppSidebar({ navigationWorkspace?, onWorkspaceSelect?,
  workspaceListUnavailable?, onRetryWorkspaces?,
  topSlot?, searchSlot?, versionSlot? })` accepts an authoritative directory
  member or null as its navigation preview. Omitted `navigationWorkspace`
  uses the actual route workspace. Selection callbacks receive its slug.
- Core `useSidebarPreferences` persists only `groups.work`, `groups.ai`,
  `groups.resources` and `groups.collaboration` booleans through
  `StorageAdapter`. `useNavigationGroupDisclosure(group, active)` shares
  active-route reveal and path-local manual collapse between `NavigationGroup`
  and the collaboration directory; it returns `{ open, onOpenChange }`.
- Core `navigationScope(path)`, `resolveScopePath(state, owner, scope, workspaceSlugs?)`
  and `useScopeHistoryStore` own Web's session-lifetime last destinations.
- Desktop `resumeDesktopScope(scope, workspaceSlugs?)` restores via no-argument
  `switchAccount()` or `switchWorkspace(slug)`. `lastWorkspaceSlug` is persisted
  independently of `activeWorkspaceSlug`, which stays null in account scope.
- `NavigationAdapter.isScopeResume?` suppresses a consumed URL anchor on resume;
  `navigationKey?` distinguishes repeated explicit opens of the same URL.
- `NavigationAdapter.isHistoryTraversal?` identifies committed Back/Forward
  returns. It is transient and separate from scope resume.
- `NavigationAdapter.replace(path, { preserveView?: boolean })` allows a caller
  to normalize an alias of the already-resolved resource without replacing its
  mounted view. Only canonical issue URLs currently request this option.

## 3. Contracts

Platform restoration operations consult remembered destinations. Notifications
and task links keep explicit path/query/fragment semantics. Group disclosure
only changes its open state; it never navigates. Do not send a default issue/list
path to a global resume operation.

Collaboration appears above Workspace in the shared left directory. Its fresh
preference is collapsed, presenting a single quiet heading with a neutral Beta
marker. The whole heading is one native disclosure button: clicking its title,
Beta marker, unread count, arrow or remaining row space toggles the directory
without navigating, matching the other first-level groups. Entering
a collaboration destination reveals Current handling, Participating and
Collaboration notifications, with
the selected list remaining highlighted when its detail is open. Manual collapse
is still allowed on that full path. The workspace name resumes the displayed
workspace's destination; its adjacent selector chooses a directory member.

`CollaborationNavigation` owns one protected notification Query observer so
collapsing the directory retains authoritative unread feedback. Its collapsed
heading shows a named count only when known and positive; unavailable counts
are not zero. The notification item is an active native link to
`/account?view=notifications&filter=unread`; collapse removes the child links
while the protected count observer and main notification page remain available.
The standalone `CollaborationNotifications` link keeps its own query for other
callers. Filtering, exact targets and mounted return restoration follow
[the notification contract](./collaboration-notifications.md).

The title's hover/focus tooltip and shared workbench header explain cross-workspace
scope and Beta maturity in both supported locales. New collaboration creation
also has a neutral Beta marker. The heading and creation entry provide the same
localized `aria-description` independently of the floating content. The heading
has a localized expand/collapse name, one keyboard stop and a coarse-pointer
target; Enter and Space each toggle it once. Child links own account navigation.

Select the navigation workspace from the actual route, otherwise the remembered
available workspace, otherwise the first authoritative directory member. A
preview must never populate `WorkspaceSlugProvider`, the API workspace header
or the route singleton. Workspace hooks, pins, search, shortcuts and ordinary
creation mount only under the real workspace route. Cache-only capability
queries in the preview use `enabled: false`. Web scope history is session-only:
an account document reload can fall back to the first workspace; Desktop keeps
its existing persisted `lastWorkspaceSlug`.

New is one menu backed by the existing ordinary and collaboration forms. In
account scope ordinary creation is disabled. Collaboration creation preselects
the workspace displayed in the sidebar in both account and workspace scope;
users can switch its source in the dialog. Pass this source separately from the
real workspace-route context used by ordinary creation. Without a sidebar
workspace, retain the creation panel's existing source selection and recovery.
Pending creation/upload recovery still owns and locks its original source.
The ordinary shortcut remains workspace-scoped. The controlled collaboration
dialog returns focus to New. Shared notifications keep the protected account
Query and original exact links/read/archive flow. Workspace-side realtime owns
notification invalidation only; the account workbench owns issue subscriptions.

Work defaults open; AI Team, Resources & connections and Collaboration default
closed. Legacy persisted groups acquire the missing collaboration default through
the existing boolean merge. Persist only manual boolean preferences, not server data or automatic active-route
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
mementos use `accountRouteKey(fullPath)` to distinguish list/view/filter,
notification and selected issue, removing the fragment;
workspace mementos keep their existing pathname semantics. No server data is
copied into these client stores.

Next may keep the outgoing route mounted after accepting a push. Publish the
new focus intent only with the rendered destination. Otherwise changing a key
on the outgoing anchored detail can refocus it and overwrite its saved scroll.
Next may also commit a query before browser history updates its fragment, or
update the browser fragment while an outgoing query is still rendered. Publish
one matching complete destination; issue-less notification links cannot inherit
an outgoing comment/run/body fragment. Same-destination explicit opens must
still advance the focus key. Shared detail
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
| Fresh workspace sidebar or legacy preferences without collaboration | Show one collapsed Collaboration/Beta heading |
| Collaboration manually collapsed on its current full path | Keep it collapsed until the heading is activated or the path changes |
| Collaboration collapsed on the notification page | Remove directory child links; retain the Query, positive unread feedback and main page |
| Same-filter/same-issue notification Back/Forward | Retain list/detail/draft nodes and restore the destination's offsets |
| New query commits before the old browser hash clears | Publish its exact target fragment, including an empty one |
| Compact account query changes | Dismiss the sidebar sheet after destination commit |
| Account Help opens Feedback | Render one feedback host; keep workspace-only modals unmounted |

## 5. Good / Base / Bad Cases

Good: the whole Collaboration heading toggles its directory in place; an
explicit child link opens its named account view. From a participating detail,
the Workspace heading resumes the displayed workspace. Navigation history
retains the account detail, search, sort and both scroll roots when returning.

Base: first use enters the existing default list, then remembers later visits.

Bad: deriving workspace MRU from object insertion order; treating a failed list
read as an empty account; using window.location as Desktop route authority.

## 6. Tests Required

Core scope-history tests own valid paths, ownership and membership fallback.
Core `sidebar-preferences.test.ts` covers legacy group hydration and manual
collaboration choices. Shared `app-sidebar.test.tsx` owns collaboration's
full-row disclosure, keyboard activation, route reveal, collapsed unread feedback and page
navigation; other groups retain their canonical disclosure regressions.
The Beta browser assertion uses real Tab/Shift+Tab focus and checks the actual
`data-slot="tooltip-content"` floating element's visibility/text plus the title's
accessible description. Base UI 1.3 does not assign a default `tooltip` role;
programmatic focus after pointer input also need not be `:focus-visible`. Do not
change the shared primitive or assume a role merely to satisfy a selector.
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
`e2e/collaboration-notifications-page.spec.ts` adds complete filtered pages,
exact targets, processing and same-issue list/detail return offsets. Assert real
Base UI active-attribute presence and NumberFlow's accessible numeric label;
unit mocks do not define those primitives' rendered DOM.

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

Wrong: `nav.push(accountPath())` for the Collaboration disclosure heading and
`nav.push(paths.workspace(slug).issues())` for its Workspace counterpart.

Correct: compose the Collaboration heading with `CollapsibleTrigger`; use the
platform resume operation for the Workspace heading and explicit account paths
for child links. Reuse existing restoration adapters rather than keeping both
application trees mounted or adding a second scroll state store.

Wrong: canonical UUID-to-identifier normalization through Web `router.replace`,
which changes Next's dynamic segment identity and closes an in-use picker.
Correct: use the explicit same-resource `preserveView` option after resolution.
