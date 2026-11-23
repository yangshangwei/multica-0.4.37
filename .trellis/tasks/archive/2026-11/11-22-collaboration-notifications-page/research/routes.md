# Research: Account notification page routing and restoration

- Query: Find the smallest shared Web/Electron route boundary for the accepted standalone collaboration notifications page, while retaining exact source targets and navigation restoration.
- Scope: Internal repository code, tests and Trellis specs; current worktree is authoritative.
- Date: 2026-11-22

## Findings

### 1. Keep the existing `/account` pathname

The route is already shared and independent of workspace discovery:

- `packages/core/paths/account.ts:2` defines `AccountView = "current" | "participating"` and the account destination's issue/comment/run/submission/description/memory targets.
- `packages/core/paths/account.ts:22` strictly parses only `/account`, accepts unique known query parameters, normalizes UUIDs, and refuses malformed fragments.
- `apps/web/app/account/page.tsx:1` delegates to `AccountWorkbenchRoute`; its parent layout mounts the account guard and shared shell.
- `apps/web/platform/account-workbench-route.tsx:10` obtains the exact path from the navigation adapter, parses it, redirects invalid destinations, and renders the shared `AccountWorkbenchPage`.
- `apps/desktop/src/renderer/src/routes.tsx:121` has one `account` route. `platform/account-workbench-route.tsx:17` parses the MemoryRouter location and mounts the same shared page after clearing the workspace mirror.
- `apps/desktop/src/renderer/src/stores/tab-store.ts:362` canonicalizes an exact `/account` path through the shared parser/builder. Account history persistence also reparses every URL (`:1202`, `:1213`).
- `packages/core/navigation/scope-history.ts:9` classifies a valid account URL before workspace-slug handling; `/account/notifications` is currently invalid everywhere above.

**Recommendation:** add a notifications destination under the existing pathname, for example:

```text
/account?view=notifications&filter=unread
/account?view=notifications&filter=all&notification=<notification UUID>&issue=<issue UUID>#comment-<comment UUID>
/account?view=notifications&filter=archived&notification=<notification UUID>&issue=<issue UUID>#run-<run UUID>
```

No new Next.js route directory, Desktop router branch, root reserved slug, native context category, or tab type is necessary. Notifications remain a page in the one existing account tab.

### 2. Destination types should preserve workbench query types

`AccountView` is a workbench API contract, not just a route label:

- `packages/core/collaboration/queries.ts:32` passes it to `collaborationWorkbenchOptions`.
- `packages/core/collaboration/access.ts:22` uses it in the workbench query key.
- `packages/views/collaboration/account-workbench-page.tsx:65` assumes `destination.view` is a workbench view.

Keep `AccountView = "current" | "participating"` unchanged. Recommended type decomposition:

```ts
export type CollaborationNotificationView = "unread" | "all" | "archived";

// Existing issue, comment, run, submission, description and memory fields.
interface AccountTarget { /* existing target fields */ }

export interface AccountWorkbenchDestination extends AccountTarget {
  view: AccountView;
  administration?: true;
}

export interface AccountNotificationsDestination extends AccountTarget {
  view: "notifications";
  notificationView: CollaborationNotificationView;
  notificationId: string | null;
  administration?: never;
}

export type AccountDestination =
  | AccountWorkbenchDestination
  | AccountNotificationsDestination;
```

`AccountWorkbenchPage` dispatches the notifications arm first, then administration/workbench. The workbench arm naturally narrows to the original two query views. `CollaborationDetail` can retain the full destination union because all exact-target fields remain shared.

Parser/builder rules:

- A missing filter on a notifications URL resolves to `unread`; a missing selection resolves to `notificationId: null`.
- `filter` and `notification` are valid only for `view=notifications`; reject duplicate parameters, unknown filters, malformed UUIDs, inappropriate administration combinations and every existing invalid fragment case.
- Preserve every legacy parser result shape and canonical `accountPath` string, including administration and memory/delivery targets.
- Serialize notification-specific parameters only while `view === "notifications"`. Existing calls such as `accountPath({ ...destination, view: "participating" })` deliberately leave the notification page and must not carry stray filter/notification query parameters.
- Do not add `notificationId?: never` to the workbench arm unless all existing destination-spread call sites are intentionally updated; excess fields originating in spreads can otherwise make that transition unnecessarily awkward.
- Export the new types through `packages/core/paths/index.ts:7`.

### 3. Encode selection separately from its content target

Two notifications can reference the same issue or different anchors within the same issue. Selection must use the notification UUID, not only `issueId`.

Construct a selected notification destination with:

1. `view: "notifications"`, the current notification filter, and `notificationId: item.id`.
2. `issueId: item.issue_id` only when the notification is readable/openable.
3. `runId: item.run_id` when present; otherwise a comment target for `source_kind === "comment"`, or `anchor: "description"` for a body source.
4. Never set more than one fragment target. The existing builder enforces exclusivity at `account.ts:57`.

The current notification link builder is in `packages/views/collaboration/notifications.tsx:69`. It already chooses the participating workbench for run notifications, retains comment/body fragments, and distinguishes removed/unavailable entries. Preserve those facts when moving to the page.

On a filter switch or compact “Back to notifications” action, construct a list-only notifications URL with the same/new filter and no notification, issue, or fragment target. An unavailable notification may remain a selected content-free notification without mounting `CollaborationDetail` or making a private issue read.

### 4. Reuse the detail and restoration seams

`packages/views/collaboration/account-workbench-page.tsx` is the nearest production example:

- `:38` obtains account scope; `:39` owns account/selected-issue realtime subscriptions.
- `:66` reads restored view state and `:71` writes it through the platform adapter.
- `:72` restores the list after data arrives; `:87` uses a responsive two-column list/detail layout.
- `:128` names its detail scroll root `account-detail`, disables browser scroll anchoring, and gives compact layouts an explicit list return link.

`CollaborationDetail` can be embedded directly, with the same `account-detail` scroll-root name:

- `packages/views/collaboration/collaboration-detail.tsx:24` accepts scope and exact account destination.
- `:34` separately fetches a linked comment missing from loaded comment pages.
- `:42` and `:46` restore the nearest `account-detail` container after data readiness.
- `:50` consumes run/comment/body anchors using the navigation adapter; `navigationKey` allows repeated explicit opens and `isScopeResume` suppresses old anchors during restoration.
- `:76` protects access-loss cases and preserves retry/recovery.
- `:104` copies a canonical participating link; it should continue producing a content link rather than a notification-page link.
- `packages/views/collaboration/run-records.tsx:32` reparses the current account URL and clears only its own run fragment; notification filter and selection should survive this replace.

The platform already captures all containers with `data-tab-scroll-root`:

- `packages/views/platform/scroll-restoration.tsx:146`: `useRestoredScrollRef`.
- `:166`: `useRestoredViewState`.
- `:176`: `useViewStateWriter`, optionally targeting another route.
- `apps/web/platform/scroll-restoration.tsx:69`: route keys from `accountRouteKey`.
- `apps/desktop/src/renderer/src/stores/tab-store.ts:1510`: the same `accountRouteKey` for Desktop restoration.

Update `accountRouteKey` (`packages/core/paths/account.ts:70`) to retain notifications view/filter/notification selection and issue identity, while dropping content fragments as it already does for workbench destinations. Different filters or notification IDs must not borrow each other's detail mementos.

**Keep the notifications list component mounted while only the selected notification/anchor changes.** Key its main list body by Server/viewer/filter, not the full selected route key; otherwise every row click remounts the list and can reset its scroll. Key the detail by issue identity and allow anchor/navigation-key changes to rerender it normally.

### 5. Sidebar, document title and tab presentation

`packages/views/layout/app-sidebar.tsx:616` owns the collaboration directory:

- `:621` parses the full current path; `:622` auto-reveals collaboration for an account destination.
- `:624` owns one protected infinite notification query even when the directory is collapsed.
- `:636` shows a count only when authoritative and positive; unavailable is not zero.
- `:644` selects Current/Participating by their route view.
- `:650` currently renders the notification popover. Replace it with an `AppLink`-backed `SidebarMenuButton`, use the existing Bell and count, and set its active/current state for the notifications destination.
- `:606` invalidates notifications only while the workspace sidebar is active; the account page should keep its existing account realtime owner, avoiding duplicate issue subscriptions.

Retain collapse/disclosure separation, Beta hints, unknown-count behavior, and compact sheet dismissal on query-only navigation. No background notification popup should remain in the sidebar.

For presentation:

- Web route title is always Collaboration at `apps/web/platform/account-workbench-route.tsx:16`; choose the localized notifications title for the new view.
- Desktop route title is likewise generic at `apps/desktop/src/renderer/src/platform/account-workbench-route.tsx:14`.
- `apps/desktop/src/renderer/src/components/tab-bar.tsx:228` special-cases account paths and overrides the normal tab presenter with Collaboration/ListTodo. Adjust that existing account branch for notifications; no new workspace TabSubject is required.
- If a Bell descriptor is used in the shared visual renderer, add Bell to `RouteIconName` and its component map. An existing Inbox icon avoids that extra type change but must match the selected product direction.

### 6. Minimal implementation boundary

Required core changes:

- `packages/core/paths/account.ts`, `paths/index.ts`, `paths/account.test.ts`: new destination parse/build/restoration contract.
- Notification API/query/filter changes are owned by the parallel API research slice.

Required shared view changes:

- `packages/views/collaboration/account-workbench-page.tsx`: dispatch the new destination; retain old workbench layout and query assumptions.
- New `packages/views/collaboration/notifications-page.tsx` and its canonical shared component test: list/filter/action/detail behavior and stable list mounting.
- `packages/views/collaboration/notifications.tsx`: replace obsolete popup exports with page navigation or extract only the factual shared notification label/target helper that the new page actually needs. Search all imports before removing it.
- `packages/views/layout/app-sidebar.tsx` and its test: destination link, selected state, unread query owner.
- Both locale files for new page/filter/empty/back labels; read the authoritative conventions before naming or editing Chinese product copy.

Platform presentation/wiring checks:

- Web and Desktop `platform/account-workbench-route.tsx` plus tests.
- Desktop `components/tab-bar.tsx` plus a focused account notification title/icon regression if its behavior changes.
- Core scope-history, Web restoration and Desktop account-tab suites should gain notification roundtrip/memento cases; they already use shared route helpers.
- No app route tree change should be necessary if using `/account?view=notifications`.

Related contract updates after implementation:

- `.trellis/spec/core/frontend/global-scope-navigation.md`: replace its explicit popover ownership wording with a navigation/page contract while retaining the protected sidebar count observer.
- `.trellis/spec/core/frontend/collaboration-workbench.md` and `.trellis/spec/desktop/frontend/account-navigation.md` as needed for notification destination/restoration semantics.

### 7. Exact routing/platform implementation handoff

Confirmed by main: adopt the discriminated destination union, `/account?view=notifications&filter=...`, default `unread`, the existing `account-detail` root, and a notification page list keyed by Server/viewer/filter only. Restoration acceptance covers Back/Forward and existing Collaboration scope resume; the explicit sidebar Notifications entry may consistently open the default unread list.

Recommended routing/platform lane ownership:

1. `packages/core/paths/account.ts`
2. `packages/core/paths/account.test.ts`
3. `packages/core/paths/index.ts`
4. `packages/core/navigation/scope-history.test.ts` (notification classification/resume regressions; production scope history should need no change)
5. `packages/views/layout/app-sidebar.tsx`
6. `packages/views/layout/app-sidebar.test.tsx`
7. `apps/web/platform/account-workbench-route.tsx`
8. `apps/web/platform/account-workbench-route.test.tsx`
9. `apps/desktop/src/renderer/src/platform/account-workbench-route.tsx`
10. `apps/desktop/src/renderer/src/platform/account-workbench-route.test.tsx`
11. `apps/desktop/src/renderer/src/components/tab-bar.tsx`
12. `apps/desktop/src/renderer/src/components/tab-bar.test.tsx` (only the new account title/icon regression)
13. `apps/desktop/src/renderer/src/stores/account-tab.test.ts` (canonical route/history/memento roundtrip)

Coordinate with the shared notification-page lane before editing `packages/views/collaboration/account-workbench-page.tsx`; that lane should own its new dispatch branch and workbench destination narrowing. Coordinate new locale keys with that same lane. If Bell is chosen as a tab visual descriptor, explicitly widen the routing lane to `packages/core/paths/route-icons.ts` and `packages/views/layout/route-icon-components.tsx` plus their existing tests; avoid two writers in the shared map. No production platform restoration file change or router-tree change is required by the adopted contract.

## Verification recommendations

Pure route/navigation matrix:

```sh
pnpm --filter @multica/core exec vitest run paths/account.test.ts navigation/scope-history.test.ts
```

Shared behavior and platform roundtrips:

```sh
pnpm --filter @multica/views exec vitest run collaboration/notifications-page.test.tsx collaboration/account-workbench-page.test.tsx layout/app-sidebar.test.tsx collaboration/run-navigation.test.tsx platform/scroll-restoration.test.tsx
pnpm --filter @multica/web exec vitest run platform/account-workbench-route.test.tsx platform/scroll-restoration.test.tsx
pnpm --filter @multica/desktop exec vitest run src/renderer/src/platform/account-workbench-route.test.tsx src/renderer/src/stores/account-tab.test.ts src/renderer/src/platform/navigation.test.tsx src/shared/issue-window.test.ts
```

Use a production Web build and actual Electron harness for focused browser/native acceptance. Existing harnesses are `e2e/global-scope-navigation.spec.ts`, `e2e/collaboration-account-desktop.spec.ts`, and `e2e/collaboration-offline-handoff.spec.ts`. JSDOM offsets or CSS class assertions do not prove painted list/detail behavior. Capture wide and compact layouts, long titles, exact comment/run/body focus, post-read/archive state, and Back/scope-resume restoration.

## Caveats / Not Found

- Main resolved the restoration distinction: exact browser/native Back/Forward and workspace↔Collaboration scope resume are in scope; explicit Notifications sidebar navigation opens the default unread list. No new per-section resume store is needed.
- The native issue-window parser (`apps/desktop/src/shared/issue-window.ts:66`) accepts any valid account destination with an issue ID. A new notifications destination with a selection may therefore become popout-eligible. Preserve the contract that account lists live in the main tab; explicit open/copy-content actions can target the canonical participating issue link.
- All current target anchors and privacy/access protections must remain operational even when the selected notification leaves the visible filtered list after marking it read or archiving it.
- No external docs were needed: the required mechanisms already exist in the current code and specs.
- Line anchors were collected before implementation; other agents are working in the same checkout, so offsets may shift.
