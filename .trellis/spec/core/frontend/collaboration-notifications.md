# Account collaboration notifications

## 1. Scope / Trigger

Apply to recipient notification reads, the shared Web/Desktop notification page,
account links, unread feedback and notification history restoration. This is an
account destination available without a workspace. Workspace inbox data and
notification generation keep their existing owners.

## 2. Signatures

- `notification-view.ts` owns `COLLABORATION_NOTIFICATION_VIEWS`,
  `CollaborationNotificationView` and its guard: `unread | all | archived`.
- `AccountDestination` discriminates the existing workbench destinations from
  `AccountNotificationsDestination { view: "notifications", notificationView,
  notificationId, issueId, commentId, runId?, submissionId?, anchor?, memoryVersion? }`.
  `AccountView` remains `current | participating` for workbench queries.
- `accountPath({view: "notifications", notificationView: "unread"})` produces
  `/account?view=notifications&filter=unread`. `notification=<UUID>` selects a
  notification independently of `issue=<UUID>` and the existing exact fragment.
- `GET /api/me/collaboration-notifications?view=unread|all|archived&cursor&limit`
  returns `{items, unread_count, next_cursor, view}`. Legacy `include_archived`
  remains supported when `view` is absent.
- `ApiClient.getCollaborationNotifications({cursor?, view?, includeArchived?},
  {signal?})` fixes the client page size at 50 and validates the response.
- `collaborationNotificationsOptions(client, scope, view = "all")` keys each
  projection below `collaborationKeys.notifications(scope)` with `{view}`.
  `useUpdateCollaborationNotification(scope)` accepts `{id, action: "read" | "archive"}`.
- `CollaborationNotificationsPage({destination})` owns protected list/detail
  reads and realtime. Scroll roots are `account-notifications-list` and
  `account-detail`. `CollaborationDetail` accepts optional
  `returnRestorationKey?: string | null` for this page's retained history returns.

## 3. Contracts

### Complete reads and compatibility

Filter in SQL before `(created_at DESC, id DESC)` cursor pagination:

| View | Membership |
| --- | --- |
| Unread | `archived_at IS NULL AND read_at IS NULL` |
| All | `archived_at IS NULL` |
| Archived | `archived_at IS NOT NULL` |

`unread_count` always counts unarchived unread rows for the authenticated
recipient. The service reads rows, count and privacy projection in one read-only
repeatable-read transaction, fetching `limit + 1` to determine continuation.
Legacy `include_archived=true` retains mixed active/archived history.

An explicit view receives an additive matching response marker. Core requires
the exact marker for Unread and Archived, including empty pages: old servers may
ignore the query. Default/All accepts a legacy envelope without the marker but
still validates unarchived membership. Invalid markers or membership produce a
null boundary result and the protected Query's unavailable state.

### Selection, privacy and processing

The sidebar item navigates to default Unread and stays active for all notification
views and selections. Its protected All observer remains mounted while the
Collaboration directory is collapsed. Only known positive counts render;
unavailable counts do not become zero. The standalone workbench entry is a link
with its own protected observer. No notification popover remains.

Rows are identified by notification UUID. Readable comment, run and body targets
retain their exact existing anchors. Removed/inaccessible rows have no private
title, source ID or run, and cannot mount or subscribe to their private detail.
Unknown event kinds use the generic label. A URL for a known unavailable row
contains its notification UUID without an issue or content fragment.

Read and archive are explicit, awaited operations. A successful receipt
invalidates the entire notification prefix; the page joins its current list's
refresh. Archive preserves `read_at`. Mutation failures retain their own retry;
query refresh never repeats a failed write. An authorized preview can remain
open after its notification leaves the filter. Recover focus to a surviving
control, list, compact back link or detail when the former control disappears.

Choose the recovery target from the validated mutation receipt before refreshing.
If that receipt leaves the current filter, skip the row and use the stable
list/back/detail fallback even when the row is still mounted or refetch fails.
Query promise completion does not guarantee that its observer's DOM projection
has committed; restoring a doomed row link can lose focus again when it is
deleted. A still-connected unrelated control retains its focus. Reads inside
All keep their row-local recovery.

### Mounted lists and committed navigation

Key the list by Server/viewer/filter. Notification/issue/fragment changes keep
the mounted list; same-issue selections also keep the detail and draft. Compact
list/detail cells retain layout geometry with visibility and inert/ARIA state,
so hiding the list does not destroy its numeric scroll range.

`accountRouteKey` retains view, filter, notification and issue, removing the
fragment. Existing platform mementos own the offsets; no section-resume store or
persisted server data is added. Explicit sidebar entry opens Unread. Global
Collaboration resume retains the remembered destination.

`NavigationAdapter.isHistoryTraversal` is transient Back/Forward intent;
`isScopeResume` retains its separate meaning. Both use existing
`navigationKey` to identify a committed return. The page's return key is
undefined for fresh navigation, null while the adapter target differs from the
rendered destination, and a string for a matching return. Consume saved list
and detail offsets after data readiness, including zero/missing offsets. Close
auxiliary content before applying detail restoration; fence deferred focus
callbacks against a newer return. Fresh explicit links continue focusing their
source anchor and preserve the list's existing position.

Web must publish a query and fragment from the same committed destination.
Next can render new query parameters before browser history updates, or update
the browser URL while the outgoing query is still rendered. Preserve the
outgoing complete location while pending and give the incoming query its exact
target fragment, including an empty fragment. A stale comment/run/body hash
must never attach to an issue-less notification or a different issue. Desktop
derives the complete location from its tab URL, through `NavigationAdapter`.

The non-suspending outer Web navigation provider owns the existing transient
intent and installs popstate in its layout effect, before Next's passive router
listener. At Window `AT_TARGET`, native listeners follow registration order;
the capture flag alone does not establish precedence. Commit the existing intent
state synchronously with `flushSync` inside that native event callback so a
higher-priority route/hash render cannot precede a batched update. This narrow
platform boundary does not add an intent store or repeated scroll writes. Valid
notification pushes and replacements pass `{scroll: false}` to Next: the
shared notification page owns inner scrolling and exact focus, while Next's
default `ScrollAndFocusHandler` can otherwise scroll/focus the outer main.
Generic workspace router calls keep their existing scroll behavior.

While a notification destination is mounted, Web temporarily uses
`history.scrollRestoration = "manual"` and restores the prior browser setting
when leaving/unmounting. Native browser auto restoration can move an inner
overflow container to its old fragment after a popstate handler has already
restored zero; `overflow-anchor: none` does not disable that history behavior.
The existing platform mementos remain the sole scroll source for notification
returns. Other routes retain their prior browser policy.

Before account push/replace/scope resume or native history publication, the Web
scroll provider snapshots marked `account-*` roots into the same offset map. Its
registered callback is bound to the committed outgoing route and Server/viewer;
an explicit caller path must match that route, and stale cleanup cannot clear a
newer registration. Keep retained hidden compact roots. Settled explicit departure
records the actual position, including zero, even without a scroll event; pending
returns preserve the existing restoration shield. A preserved list position on
A→B does not emit scroll, so continuous event capture alone cannot restore B
after it unmounts. No additional memento store is introduced.

## 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Duplicate/unknown account params, invalid view/filter/UUID/fragment | Parser returns null; builder rejects the destination |
| Notification params on a workbench destination or notification administration | Reject the combination |
| Explicit HTTP `view` combined with any legacy selector | 400 `invalid_notification_query` |
| Invalid/duplicate HTTP view or legacy boolean | 400; no filtered read |
| Missing/mismatched filtered response marker, even with no rows | Unavailable Query; no false empty-filter claim |
| Row state contradicts the requested view | Reject the response at the Core boundary |
| Known removed, denied or issue-mismatched selection | Content-free detail state; no private issue read/subscription |
| Notification leaves Unread/All after confirmed processing | Retain authorized selection; converge list/count and recover focus |
| Initial, refresh or next-page failure | Visible retry for that read; no mutation replay |
| Back/Forward or scope return to a retained same-issue detail | Restore its saved offsets; preserve draft/list nodes |
| Explicit repeated source link | New focus intent; exact source anchor, preserved list offset |
| Notification selection passed to native issue-window admission | Reject popout admission; use the main account tab |

## 5. Good / Base / Bad Cases

Good: All → notification A/body for issue X → notification B/issue X, with
distinct saved offsets; Back/Forward restores each tuple while the draft remains.

Base: legacy Default/All reads return unarchived rows without a view marker;
the independent sidebar observer still supplies the global count.

Bad: filtering loaded rows after pagination, substituting issue UUID for
notification UUID, replaying a write from Refresh, or pairing a newly rendered
query with the browser's outgoing fragment.

## 6. Tests Required

Core route tests own valid/invalid destination and memento-key matrices.
Notification schema/API tests own marker and observable membership validation,
legacy compatibility, malformed payloads and exact receipt IDs. Query/mutation
tests own protected scope keys and prefix invalidation.

Go handler tests own pre-cursor filtering across tied timestamps, another
recipient's sentinels, all global counts, legacy mixed history and filtered
privacy. Service tests own early validation and archive/read independence.
Run DB packages on distinct, recorded migrated databases.

Shared page tests own loading/empty/error/retry, exact links, read/archive
feedback, focus and stable list identity. `run-navigation.test.tsx` composes real
detail with same-issue draft/offset restoration. Platform suites own committed
query/fragment races, history intent, titles and native window admission.
`e2e/collaboration-notifications-page.spec.ts` proves complete second pages,
privacy, exact targets, same-issue offsets, compact back navigation and confirmed
processing in production Web and actual Electron. Retained/synthetic records
remain distinct from proof of runtime ingestion.

Real DOM assertions must follow the real primitives: ordinary issue titles are
editable buttons; Base UI's `data-active` is a present boolean attribute, not a
literal `"true"`. NumberFlow's text contains the entire digit roller: inspect
its named numeric element (`aria-label`) and painted capture instead of treating
`textContent` as the value. Confirm the destination path and active filter before
capturing a list handle; otherwise an outgoing filter's node can falsely fail
retained-list proof. Keep the identity assertions and exact target/count checks.

## 7. Wrong vs Correct

Wrong: fetch default rows and filter the loaded pages in JSX.
Correct: request an explicit view, apply its SQL predicate before pagination,
and validate the echoed marker and row membership.

Wrong: key the notification list by notification/issue or restore only at its
first mount.
Correct: retain its scope/filter identity and consume matching committed-return
mementos through the existing adapter.

Wrong: combine incoming Next query parameters with an outgoing window hash.
Correct: publish one complete committed destination, preserving pending-source
intent and the incoming target's exact or empty fragment.
