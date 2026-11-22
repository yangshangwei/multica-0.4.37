# Collaboration notifications page design

## Accepted behavior and minimum gap

The approved page replaces the sidebar popover, adds genuinely complete state-filtered lists and embeds the existing collaboration detail. This is an Operate surface: scanability, familiar app chrome, exact target recovery and restrained semantic states govern its design. Current account workbench/inbox components and shared tokens are the visual authority. No visual-world replacement is intended.

## Routing and ownership

- Keep `/account`; `?view=notifications&filter=unread|all|archived` identifies the new page. `notification=<UUID>` selects a notification; `issue=<UUID>` and the existing fragment identify its exact authorized content.
- Keep `AccountView = current | participating` for workbench queries. Define shared target fields and a discriminated `AccountWorkbenchDestination | AccountNotificationsDestination` union. Notifications carry `notificationView` and nullable `notificationId`; they cannot be an administration destination.
- Preserve every existing account URL/parser result. Reject duplicate/unknown params, unsupported filters, malformed UUIDs and invalid target combinations. Notification-specific params serialize only for `view=notifications`, so existing participating content links deliberately leave the notification page.
- `accountRouteKey` retains notification filter/selection/issue and removes the fragment. The existing account tab and scope-history mechanisms remain the only navigation/restoration owners.
- Web/Desktop page titles and the account tab's existing presenter distinguish notifications. Main account lists remain in the primary account tab; an explicit open-content action may use a canonical participating issue URL. Do not accidentally make selected notification lists eligible for issue-window popout.

## Data flow and API compatibility

`authenticated recipient → SQL state predicate + stable cursor → repeatable-read service projection/count → handler envelope → validated Core API → protected infinite Query → shared page`.

- Extend the existing GET endpoint with `view=all|unread|archived`. Explicit `view` and legacy `include_archived` are mutually exclusive at the HTTP boundary. Calls without `view` retain the existing include-archived behavior.
- Filter before `(created_at DESC, id DESC)` pagination. All excludes archived; Unread additionally requires `read_at IS NULL`; Archived requires `archived_at IS NOT NULL`. Keep limit+1 and recipient-global unread count in the same read-only repeatable-read transaction.
- Echo an additive `view` marker for explicit requests. The Zod envelope validates marker values. Requested Unread/Archived data requires the exact marker, including empty pages, so an older backend that ignores a query cannot appear to support filtering. Default/All retains legacy-envelope acceptance. Validate observable state membership for filtered payloads at the API boundary, not in JSX.
- Add one pure `notification-view.ts` leaf owning the view values/type/guard, reused by route parsing, schema, queries and UI. No new dependency or schema migration.
- Extend the notification query helper to accept a typed view (default All), with distinct list keys below the existing notification prefix. Existing read/archive mutation invalidation covers every view; account realtime retains its current ownership.
- Existing privacy projection and access/session-epoch fences remain unchanged. Title/source/run are unavailable after lost access. No actor or source-workspace inference is added.

## Shared page and interaction

- `AccountWorkbenchPage` dispatches notifications before its existing administration/workbench branches. The new page owns `useCollaborationScope` and account/selected-issue realtime, queries notification data and uses the existing `CollaborationDetail` for readable selections.
- Use shared page header/chrome with Bell, localized Collaboration notifications title, known positive unread count and concise cross-workspace explanation. Render three route links in a named filter navigation region with persistent active styling and coarse-pointer targets.
- Wide screens show a restrained list column and a flexible detail pane, each independently scrollable. Compact layouts retain the list DOM but hide it while detail is selected; a visible back action clears selection/content while preserving the filter. Keep the `account-detail` root name and disable browser scroll anchoring for exact restoration.
- Key the list subtree only by Server/viewer/filter; notification/issue/fragment changes rerender without remounting the list. Key detail by Server/viewer/issue, so same-issue notifications preserve draft state while consuming a new exact anchor.
- A native link in each row selects the notification, sibling action controls handle Mark read/Archive. Keep selection and unread styling legible while hovered. Event type and timestamp support scanning; titles wrap deliberately. A named Notification actions menu may reduce repetitive visible buttons, with keyboard/touch access.
- Unavailable/removed rows stay actionable only for their own notification state; no private title, content URL or detail request is fabricated. Loading, empty, failed-page and failed-mutation states offer the correct recovery action.
- Read/archive is explicit and non-optimistic. Confirm the write, then refetch/invalidate. Selection remains route-owned if the row disappears; the authorized detail can stay open with a factual filtered-out message. Recover focus to a surviving notification control/list/preview as appropriate. Retry of a read query must not retry a failed write.
- Browser/native Back/Forward and existing Collaboration scope resume use route/memento state. The sidebar's explicit Notifications link opens Unread. No new persisted server/selection store.

## Same-issue history restoration follow-up

Independent review found that a mounted scope/filter list reads its offset only once, while native Back/Forward intentionally keeps the account host and same-issue detail mounted. The coordinator saves distinct notification-route mementos correctly, but the mounted page does not consume the destination memento. Both platforms currently classify history returns as fresh opens, so an old source fragment can also override the saved detail offset.

- Add an optional, transient `NavigationAdapter.isHistoryTraversal` signal. Web marks the committed popstate target; native marks Back/Forward in its existing navigation state and clears the flag on explicit navigation/scope transitions. Do not persist this flag or change `isScopeResume` semantics.
- The notification page consumes saved list/detail offsets on a new return intent, including a zero/default offset, without remounting the list or same-issue draft. Preserve current scrolling for ordinary row clicks and query refreshes.
- The existing collaboration detail may accept a narrowly scoped optional return-restoration input for the notification page so saved detail offsets win over historical fragments. Its other callers retain their existing behavior. Apply restoration after relevant data is ready and only when the navigation target matches the rendered destination; do not consume an outgoing page's pending return intent.
- Regression proof must distinguish A/B notification IDs for the same issue, retain the same list/detail/draft nodes, and restore deliberately different list/detail offsets on Back and Forward. Explicit same-link opens must still focus their source anchor.

## Boundaries and rejected alternatives

- Rejected `/account/notifications`: native validators/router history already share a strict `/account` contract; adding a path branch is unnecessary.
- Rejected filtering only fetched rows: it yields false empty views and skips matching records outside the loaded pages.
- Rejected defaulting an absent view marker to the requested filter: older servers ignore unknown query parameters, including when they return an empty list.
- Rejected reimplementing collaboration detail or duplicating the workspace inbox's data model: account ownership, source privacy and anchors already have canonical implementations.
- Remove obsolete notification popover code/imports/tests. Keep one useful standalone notification navigation link for account workbench callers without sidebar navigation.
- Preserve unrelated working-copy changes. Route, API and UI implementers have separate file ownership; only the coordinator updates shared specs and records final verification/commit.

## Verification and rollback

- Canonical layers: route/schema/API tests own invalid-input/filter matrices; Go handler tests own server filter/cursor/recipient/privacy behavior; shared component tests own visible state/actions/focus/list identity; platform tests own title/history/window wiring. Live production Web and Electron acceptance prove painted layout/scroll/anchors.
- Reverse this atomic task diff to restore the popover; no migration or retained data conversion is involved. Legacy API behavior is unchanged for installed clients.
