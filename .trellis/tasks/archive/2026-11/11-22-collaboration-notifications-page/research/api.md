# Research: Collaboration notification API and state filters

- Query: How can the approved independent notifications page support Unread, All (unarchived) and Archived with complete pagination, truthful counts and existing privacy guarantees?
- Scope: internal
- Date: 2026-11-22

## Findings

### Existing transport and data contract

- `packages/core/api/client.ts:1659` exposes `getCollaborationNotifications({ cursor?, includeArchived? }, { signal? })`. It requests `/api/me/collaboration-notifications`, fixes the page size at 50, and parses the response through the collaboration Zod boundary.
- `packages/core/api/client.ts:1666` exposes read/archive POST operations and refuses a receipt whose notification ID differs from the request.
- `packages/core/collaboration/content-schema.ts:153` validates notification IDs, exact decimal event sequence, kind, mention flag, issue/comment/body source, optional result run, issue title, `can_open`, `created_at`, `read_at` and `archived_at`. Private/unavailable records must have a null title, issue-only source and null source ID; optional result runs require an authorized recognized result event.
- `packages/core/collaboration/content-schema.ts:162` validates `{ items, unread_count, next_cursor }`. There is currently no response filter/version marker.
- The current DTO does **not** contain source workspace identity, source workspace label or human actor. Timestamps are already available. System result notifications deliberately have no proxy human actor (`.trellis/spec/server/collaboration-records.md:204`).

### Existing Go behavior

- `server/internal/handler/collaboration_notifications.go:11` parses the generic content cursor and only an optional `include_archived` boolean. Unknown query parameters are ignored.
- `server/pkg/db/queries/collaboration_notifications.sql:11` filters the authenticated recipient and, by default, excludes archived records. `include_archived=true` means a **mixed** active-plus-archived history, not archived-only.
- `server/internal/service/collaboration_notifications.go:147` uses a read-only REPEATABLE READ transaction. It obtains `limit + 1` rows, computes a recipient-global unread count in the same snapshot, and returns `HasMore` without hydrating a notification's private content after access is lost.
- `server/pkg/db/queries/collaboration_notifications.sql:23` orders/paginates by `(created_at DESC, id DESC)`. `server/internal/handler/collaboration_comments.go:28` validates the timestamp/UUID cursor pair, bounds page sizes to 1–100 and cursor strings to 512 bytes; `:61` creates the next cursor from the last returned row.
- `server/pkg/db/queries/collaboration_notifications.sql:28` defines unread count as recipient rows where both `read_at` and `archived_at` are null. This count must remain global and independent of the selected list view.
- `server/pkg/db/queries/collaboration_notifications.sql:32` marks read idempotently with `COALESCE(read_at, now())`. `:37` archives idempotently without changing `read_at`; archived unread rows stop contributing to the count because the count excludes archived rows.
- `server/internal/service/collaboration_notifications.go:199` restricts mutation to the authenticated human recipient and validates account status inside the transaction. A revoked recipient may still mark their retained content-free notification; another recipient gets not-found.

### Scope, cache and realtime

- `server/internal/handler/collaboration_comments.go:21` clears ambient workspace headers and sets no-store/no-referrer for the account content request.
- `packages/core/collaboration/access.ts:19` scopes notification keys to API instance/session and user identity. `protectCollaborationRead` (`:101`) rejects malformed/null envelopes, stale identity and late access-epoch responses. Keep new list reads and mutations inside this fence.
- `packages/core/collaboration/content-queries.ts:50` adds `{ includeArchived }` to the notification prefix, provides infinite pagination and refreshes every 15 seconds plus foreground/reconnect. `:67` invalidates the entire notification prefix after a successful read/archive.
- `packages/views/layout/app-sidebar.tsx:609` invalidates the notification prefix on collaboration workbench changes while retaining an authoritative Query observer even when the collaboration directory is collapsed.
- `server/internal/handler/collaboration_notifications.go:65` publishes a content-free `collaboration:workbench_changed` hint after committed read/archive state, so other windows re-read rather than receiving private row data.

## Recommended smallest extension

1. Add an explicit `view=all|unread|archived` query parameter to the existing endpoint; do not add another endpoint or database migration.
2. Preserve legacy calls with no `view`: no `include_archived` still returns unarchived history, and `include_archived=true` still returns mixed history for installed older clients. Validate explicit view values. Specify that an explicit view owns selection and either reject a contradictory legacy flag or ignore that flag deterministically; do not leave the combination ambiguous.
3. Pass the view through `CollaborationNotificationsQuery` and add a SQL predicate **before pagination**:
   - all: `archived_at IS NULL`;
   - unread: `archived_at IS NULL AND read_at IS NULL`;
   - archived: `archived_at IS NOT NULL`.
   Preserve legacy mixed selection when view is absent. Use the existing `limit + 1`, cursor pair, order and global unread count.
4. Regenerate `server/pkg/db/generated/collaboration_notifications.sql.go` with `make sqlc`; never edit only generated SQL.
5. Extend client params with `view?: "all" | "unread" | "archived"`. Give new infinite list views distinct keys under the same notification prefix; update internal callers directly rather than adding a legacy boolean adapter to the query helper.
6. Add a response marker for explicit filtered requests, for example `view`. Make the schema field additive for legacy responses, but validate its identity against the requested filter before returning filtered data. An older backend ignores an unknown `view` and can return the wrong rows or a false empty set; a response marker is needed to detect this even when `items` is empty. Default/all calls can still accept old envelopes because their legacy semantics match.
7. Keep read/archive API receipts, recipient/global count semantics and prefix invalidation. Avoid client-only filtering of the pages already fetched: an empty unread/archived projection does not prove there are no matching records farther down the server cursor.

Recommended API-side write boundary:

- `server/internal/handler/collaboration_notifications.go`: explicit view parse/validation, response filter marker.
- `server/internal/service/collaboration_notifications.go`: query field/validation and DB args; preserve private record hydration.
- `server/pkg/db/queries/collaboration_notifications.sql` and regenerated counterpart: view predicates.
- `packages/core/api/client.ts`: view serialization and exact response-filter validation.
- `packages/core/collaboration/content-schema.ts`: additive filter marker/type and malformed filter validation.
- `packages/core/collaboration/content-queries.ts`: per-view keys/requests with common prefix invalidation.
- `packages/core/collaboration/index.ts`: export new notification view/type if the shared page needs it.

## Meaningful verification

- Extend `server/internal/handler/collaboration_notifications_test.go` with a recipient fixture containing unread, read and archived rows across multiple issues, another recipient's sentinel rows, duplicate creation timestamps and a page size of one/two. Assert each filtered page and subsequent cursor contain only the requested state, have stable order and report the same global unread count. Assert legacy mixed selection stays unchanged, invalid view is bad-request, ambient invalid workspace headers remain irrelevant, and revoked/removed private titles/run/comment anchors remain absent.
- Extend the canonical service notification suite only for service-owned validation/idempotence, avoiding a duplicated full HTTP filter matrix. Existing tests at `server/internal/service/collaboration_notifications_test.go:56` own lost-access/private serialization/idempotent read behavior; retain them.
- Add a focused core API transport suite next to the existing `packages/core/api/collaboration-content.test.ts` checks: each view query, cursor forwarding, no ambient workspace headers, accepted legacy default responses, rejected mismatched/missing explicit filter identity including empty pages, malformed payloads and matching read/archive receipt IDs.
- Put pure schema/filter identity checks in a node-environment core test, not a DOM mount. Existing `content-schema.test.ts:212` and `result-notification.test.ts:16` are the canonical private wire/result-anchor coverage.
- Shared page tests should exercise visible filter switching, complete load-more, correct empty states, unavailable-source rows and count refresh after a committed mutation. Do not duplicate the backend filter matrix through every DOM case.

Suggested commands after implementation (research did not execute them):

```sh
make sqlc
pnpm --filter @multica/core exec vitest run api/collaboration-content.test.ts collaboration/content-schema.test.ts collaboration/result-notification.test.ts
pnpm --filter @multica/core typecheck
pnpm --filter @multica/core lint
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test ./internal/handler ./internal/service -run 'TestCollaborationNotification|TestCollaborationNotifications' -count=1
go -C server vet ./internal/handler ./internal/service ./pkg/db/generated
```

`make test` (`Makefile:383`) ensures the target database exists, applies migrations and runs the guarded Go suite with race detection; use that broader pipeline when the narrow DB-backed checks are ready. `make sqlc` pins sqlc v1.31.1 (`Makefile:405`). No external API/SDK research was needed.

## Related specs

- `.trellis/spec/core/frontend/global-scope-navigation.md`: authoritative account-owned notification observer/count and realtime ownership.
- `.trellis/spec/core/frontend/collaboration-workbench.md`: React Query ownership, shared account reads, exact notification anchors and protected unavailable behavior.
- `.trellis/spec/server/collaboration-records.md`: additive result run links, revoked content removal, system-result actor semantics.
- `.trellis/spec/server/collaboration-realtime.md`: content-free invalidation hints and durable database recovery.
- `.trellis/spec/server/collaboration-source-editing.md`: canonical event/notification identity and source reaction boundaries.

## Caveats / Not Found

- **Acceptance blocker in current code:** neither unread-only nor archived-only server pagination exists. Local filtering is insufficient.
- **Compatibility blocker if only a query is added:** older servers ignore the new filter. Explicit filtered reads need an echoed filter contract or another detectable capability; unavailable UI is preferable to false filtered results.
- The approved MVP does not require actor or source workspace labels, and they are not in the current wire DTO. Adding them would need access-aware service hydration and additional schema privacy tests; do not invent labels or infer a human actor for system results.
- Archive does not imply read. Keep this distinction when rendering archived notifications and updating badges.
- The current mutation hook invalidates asynchronously. A page that immediately removes a selected unread/archived row needs deliberate focus/selection recovery and must not interpret a query refetch retry as a retry of a failed write.
- No product code, specs or git state were changed during this research.
