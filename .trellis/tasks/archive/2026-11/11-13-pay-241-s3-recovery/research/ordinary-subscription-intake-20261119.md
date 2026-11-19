# Research: ordinary subscriber delivery and literal event replay

- Query: Which ordinary subscriber/inbox path closes the subscription portion of TC-22, and what is the smallest persisted, concurrent-safe seam for replaying the same event without a second notification?
- Scope: internal; source inspection only
- Date: 2026-11-19
- Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`

## Findings

### Actual product path and authority

Ordinary subscriptions use `issue_subscriber`; collaboration subscriptions use current collaboration participants and `collaboration_notification`. A WebSocket `subscribe_ack` establishes transport membership, not either persisted delivery. Extras-A R05 exercised the latter collaboration path and explicitly retained `issue_subscriber=0` and `inbox_item=0`; it does not prove ordinary delivery.

| Operation | Exact ordinary HTTP contract | Source |
| --- | --- | --- |
| List active subscribers | `GET /api/issues/{resolved issue}/subscribers` | `server/internal/handler/subscriber.go:36`; `server/pkg/db/queries/subscriber.sql:232` |
| Explicitly subscribe | `POST /api/issues/{issue}/subscribe`, JSON `{}` for self or `{ "user_id": "<UUID>", "user_type": "member" }` for a workspace member; 200 `{ "subscribed": true }` | `subscriber.go:59`; `packages/core/api/client.ts:3846` |
| Unsubscribe this issue | `POST /api/issues/{issue}/unsubscribe`, same optional target; 200 `{ "subscribed": false }` | `subscriber.go:115`; `client.ts:3856` |
| Unsubscribe subtree | Separate `POST /api/issues/{issue}/unsubscribe/subtree` | `subscriber.go:121`; `client.ts:3879` |
| Read own inbox | `GET /api/inbox`, current workspace header/context, authenticated member recipient | `server/internal/handler/inbox.go:156`; `client.ts:5012` |
| Read/archive exact notice | `POST /api/inbox/{inbox UUID}/read`, `/archive` | `inbox.go:223`; `server/cmd/server/router.go:2749` |

The handler resolves the issue through `loadIssueForUser`; the subscriber target must be a current entity of that source workspace (`subscriber.go:61`, `:86`, `:138`, `:164`). Caller defaults use `resolveActor`, not an arbitrary passed account. Password Web requests use actual cookies and CSRF; Desktop requests use the original authenticated token. `X-Workspace-ID` must bind the source workspace for ordinary routes.

Explicit subscription clears `unsubscribed_at` and `opt_out_scope` and records `reason='manual'` (`subscriber.sql:148–156`). This-issue unsubscribe tombstones the row with `opt_out_scope='issue'` (`:158–165`). Auto-subscription never resurrects a tombstone (`:1–25`). Active-list reads exclude tombstones (`:232–241`). Subtree/revoke/delegated races already use the common `(workspace,user)` subscriber fence (`subscriber.sql:27`); it is not a notification event deduplication fence.

Ordinary subscribe/unsubscribe writes to a collaboration source are rejected with 409 `unsupported_collaboration_action` (`server/internal/handler/collaboration_boundary.go:61–72`). Use a real ordinary issue or ordinary child for this acceptance. Do not manufacture `issue_subscriber` rows for the collaboration root or interpret an unmentioned participant as an ordinary watcher.

### Recipients and event identity

`server/cmd/server/main.go:603–608` registers subscriber listeners before notification listeners. Ordinary creator/assignee/commenter rules write subscriber state before notification fanout (`server/cmd/server/subscriber_listeners.go:30`, `:107`).

For `comment:created`, the handler publishes the actual committed `CommentResponse.ID` (`server/internal/handler/comment.go:1928–1936`). The notification listener puts that ID in `details.comment_id`, queries active subscribers, and calls the generic inbox insert (`server/cmd/server/notification_listeners.go:850–900`, `:370–445`). The recipient matrix for a plain, nonmentioned comment is:

- Current subscribed members other than the commenter receive `new_comment`.
- The commenter is excluded even though the commenter rule can subscribe them.
- An unsubscribed or never-subscribed member receives no `new_comment`.
- Agent subscribers receive no member inbox notice.
- A member with the `comments` preference muted receives no `new_comment`.
- A parent-only watcher does not receive child comment traffic; only `status_changed` bubbles to the immediate parent's watchers (`notification_listeners.go:71–77`, `:293–333`).

`new_comment` and `mentioned` are distinct ordinary notice types. A directly mentioned subscriber can legitimately have both ordinary rows; choose plain text for the initial subscription/replay test so one-row expectations do not accidentally suppress an existing mention behavior. Ordinary direct mention parsing is a separate path (`notification_listeners.go:902–907`, `:513–623`).

`server/internal/events/bus.go:9–22` has no durable generic `Event.ID`. `Bus.Publish` invokes every matching callback each time (`:61–86`). Scope hints are routing hints. `handler.publish` merely constructs that event (`server/internal/handler/handler.go:858–865`). There is no bus-level retained notification decision. A transport/realtime frame ID is not an input to this notification decision.

The strongest available identity for the narrowly scoped comment path is therefore the original committed comment UUID, with workspace, issue, recipient, and notification type defining its delivery identity. An ordinary `issue:updated` event contains a revision and field-change flags; this intake does not propose a bus-wide identity redesign or a guarantee for every generic event type.

### Existing defect boundary: event replay is different from request replay

The current `notifyIssueSubscribers` unconditionally supplies `dbid.NewV7()` to `CreateInboxItem` on each callback (`notification_listeners.go:416–429`). The existing SQL insert has no event/source conflict rule (`server/pkg/db/queries/inbox.sql:92–98`). Therefore source inspection predicts that publishing the same `comment:created` event twice will create two fresh inbox rows per eligible recipient. This is a concrete code gap, **not yet an executed RED or measured failure**.

The public ordinary comment API also has an opt-in original-request guarantee: a canonical nonzero `Idempotency-Key` routes through `comment_delegation.go:196`. Its repeated key/body returns the stored original comment response before `comment:created` publication (`:240–261`). Changed body/issue conflicts with 409 (`:246–248`). The stored request is accessible at `GET /api/issues/{issue}/delegation-receipts/{key}` (`:337`). That can prove original-request recovery, but **cannot substitute for literal repeated `Bus.Publish`**.

The ordinary shared UI presently sends no comment idempotency key (`packages/core/api/client.ts:3718–3735`; `packages/core/issues/mutations.ts:777`). An E2E fixture may deliberately call the existing opt-in API and must label that as API original-request recovery. It must not claim that the ordinary composer automatically acquires that guarantee.

### Recommended minimal persisted seam

Prefer a narrow new comment inbox insertion query over a bus-wide event field, new table, new migration, new dependency, or changes to generic notifications. Existing precedent is `CreateIterationInbox`, which uses the inbox primary key plus `ON CONFLICT(id) DO NOTHING RETURNING *` (`server/pkg/db/queries/iteration_notifications.sql:24–27`). Its service treats `pgx.ErrNoRows` as successful deduplication and only publishes a new notice when an insert actually returned a row (`server/internal/service/iteration_notifications.go:157–168`).

For a valid original comment UUID:

1. Derive a stable, domain-separated UUID from the original comment ID and the exact workspace/issue/member recipient/type tuple. `github.com/google/uuid v1.6.0` is already a direct Go dependency. Existing stable UUID derivation is demonstrated by `collaboration_knowledge_invalidation.go:32–33`; no dependency installation is needed.
2. Add a narrowly named inbox query accepting that ID and the ordinary notification columns. Keep `CreateInboxItem` unchanged for all other producers. `ON CONFLICT(id) DO NOTHING` gives concurrent fixed writers one persisted winner even when their initial snapshots both see no inbox row.
3. Before inserting, suppress when **any retained** ordinary `new_comment` row already matches workspace, issue, recipient type/ID and `details.comment_id`. Include read and archived rows. This retains historical random v4/v7 IDs, read/archive state, original title/body/details, and timestamps; no backfill, delete, or `DO UPDATE` is warranted.
4. In the `new_comment` producer, treat no returned row as deduplicated and skip `inbox:new`. Continue normal actor exclusion, active subscriber lookup, delegated tiers, and mute checks. Do not route distinct comments or notice types through the same key.

Required qualifications:

- The stable-ID path adds a deliberate exception to the ordinary inbox v7 insertion convention. `server/pkg/dbid/dbid.go` documents v7 identity clustering, and `writes_test.go:46` includes the generic inbox writer. Retain the generic v7 path and capture the source-derived stable identity decision in the eventual spec.
- The legacy retained-row lookup protects rows already present when the new writer runs. It does not provide a cross-version uniqueness constraint against an older server concurrently publishing the same source with a random ID. If mixed-server rolling deployment is in scope, a separately reviewed source uniqueness/index or transaction-fenced legacy adoption may be necessary. Do not silently claim that larger deployment guarantee.
- Existing duplicate historical rows must remain visible as retained history; this slice prevents further duplication and must not delete old rows to make counts pass.
- Derive from UUID values, so alternate case spelling does not create a second identity. Invalid, empty or zero synthetic comment identities need an explicit disposition preserving existing producer/tests; do not claim a new stable event guarantee for absent identity.
- A changed payload carrying the **same original comment ID** must not rewrite the first notice or reopen read/archived rows. A genuine different comment ID remains a separate event.

There is no authoritative ordinary-subscription contract found requiring a frozen late-subscriber set. The current ordinary fanout deliberately looks up **current** active subscribers on each delivery. The explicit “freeze effective recipients in the terminal transaction; retries do not recompute” rule applies to collaboration terminal group results (`.trellis/spec/server/collaboration-records.md:193–203`). The existing S3 research likewise restricts historical late-join freezing to collaboration results (`recovery-gaps.md:113–128`). Do not change ordinary subscription semantics by importing that separate rule. A late ordinary subscriber may receive an old re-emitted event once under the current ordinary contract; the same source/recipient must still never receive a second row.

### Exact RED/GREEN and ownership proposal

The sole implementation owner can start a real PostgreSQL RED in a **new** file `server/cmd/server/notification_comment_replay_test.go`. It should reuse `newNotificationBus`, or create `events.New()` and register only the actual notification listener to keep unrelated subscription/activity effects out of the dedup concurrency claim. `Bus.Publish` is synchronized only around its listener registry; concurrent caller goroutines are permitted and run callbacks concurrently (`events/bus.go:61–86`). Use the **same immutable typed event** containing one real fixture-created comment UUID twice, not two HTTP-created comments with equal text.

Recommended named assertions:

| Named check | Necessary observation |
| --- | --- |
| Exact sequential replay | First delivery creates one original row and one `inbox:new` per expected subscriber; second same event adds neither |
| Concurrent same event | Two independently started callers or two bus instances share one fresh DB; exactly one stable row/notice per recipient, no handler panic or swallowed DB failure |
| Fresh bus after delivery | Re-register listeners on a new bus and republish the same event; persisted identities stay unchanged, proving process-local sets are insufficient |
| Read/archive preservation | Mark/archive original inbox UUID, replay with same comment identity and changed content/title; preserve entire original row, no revived inbox/broadcast |
| Legacy random ID | Insert a legitimate old random-ID `new_comment` with the exact tuple, read/archive it, replay; keep that original UUID and values |
| Distinct source and recipient | A new actual comment UUID produces its own one notice; distinct members retain distinct notices; an opted-out member receives no new comment |
| Ordinary delivery tiers | Actor, unsubscribed, nonwatcher, Agent and mute controls keep their existing behavior; no private collaboration notice appears in the ordinary inbox |

Use `server/internal/testutil` for new DB rows/cleanup and the task-owned separate normal/race allocations. Do not transplant the old test helpers' open-coded SQL row creation into the new suite. Existing reference tests are `notification_listeners_test.go:241` (status recipient matrix), `:380` (plain comment recipient matrix), and `:70` (real subscriber + notification bus wiring). They presently use fake zero comment UUIDs and do not contain a literal replay test.

Expected product ownership after RED: `server/cmd/server/notification_listeners.go`, `server/pkg/db/queries/inbox.sql`, regenerated `server/pkg/db/generated/inbox.sql.go`, and the new focused test. No unrelated `main.go` signature, `events.Bus`, subscriber rules, collaboration notifications, frontend product source or migration change is needed for this selected slice. Parent coordinates code acceptance and final environment binding.

### Current Web/Desktop acceptance slice

After literal-event same-layer RED/GREEN and independent acceptance, add a separate ordinary subscriber E2E spec/fixture. Keep the six Extras-A R05 tops unchanged. Reuse current password accounts, public source membership/invitation, ordinary issue creation, `TestApiClient`, the native renderer harness and read-only inventory conventions.

Smallest meaningful two-platform journey:

1. Create one real ordinary child of a collaboration parent, verify `collaboration == null`, and use a distinct current source member as a noncreator/noncommenter watcher. Subscribe through the actual **Subscribe** button, observe its `POST .../subscribe`, then read its persisted manual row and the actual **Unsubscribe** button. Keep another eligible nonwatcher or opted-out source member as a negative control.
2. Send a plain comment by a different authenticated actor, get the real saved comment ID, and compare **all recipient/notification IDs** through each actor's `/api/inbox` and a private read-only SQL audit. Verify the exact row in production Web/matching Electron and its original comment anchor; selecting it intentionally changes read state through `/api/inbox/{id}/read`.
3. Reconnect/reload the subscriber's actual client and verify the same inbox ID and no additional business writes. Execute the separately labeled opt-in original-request replay if useful, while retaining the literal same-event proof from the Go listener layer. Unsubscribe through the actual control and post a distinct second comment; no second event notice for the opted-out user, previous notice unchanged. Explicit re-subscribe may admit only a separately identified new event under the current rule.

Ordinary child navigation can reuse the established account-parent disclosure sequence (`e2e/fixtures/collaboration-recovery.ts:717–723`): reveal **Sub-issues**, click the exact child identifier/title, and assert its actual title button. `withSessions` already owns password Web cookies, original Desktop token, private profile, physical sockets and cleanup (`:269–356`). That helper's `session.navigate` is account-oriented; do not pass an ordinary workspace route through `collaboration-native.ts:59`'s account deep-link rewriting. Native ordinary inbox navigation should use the actual workspace/sidebar after `selectDesktopWorkspace` (`collaboration-native.ts:65`) or another existing workspace harness that supports it.

Shared selectors and product wiring:

- Subscribe/Unsubscribe controls and subscriber picker: `packages/views/issues/components/issue-detail.tsx:3458–3542`; no extra test-only product buttons required.
- Mutation/rollback/authoritative refresh: `packages/core/issues/mutations.ts:1067–1128`; `packages/views/issues/hooks/use-issue-subscribers.ts`.
- Actual inbox row: `page.getByRole('listitem').filter({has: page.getByRole('button', {name: issue.title, exact:true})})`; tested in `e2e/core-business-journeys.spec.ts:142–158` and `e2e/upstream-selected-fixes.spec.ts:78–102`.
- Inbox detail anchor: `#comment-body-<original comment UUID>`; see `upstream-selected-fixes.spec.ts:151–168`.
- `onInboxNew` invalidates server-state queries (`packages/core/inbox/ws-updaters.ts:5–17`). UI grouping by issue can conceal duplicate **persisted** rows (`packages/core/inbox/queries.ts:108`), so one visible row is insufficient to assert event deduplication.

### Audit storage

Retain the exact SQL with result hashes. Bounded read-only queries should enumerate:

- `issue_subscriber`: `issue_id,user_type,user_id,reason,created_at,unsubscribed_at,opt_out_scope`, with the actual member/source relation.
- `inbox_item`: `id,workspace_id,recipient_type,recipient_id,type,issue_id,actor_type,actor_id,details,read,archived,created_at`; include archived/read rows and match `details->>'comment_id'` against originals. Hash body/title instead of exporting private text where appropriate.
- `comment`: exact `id,issue_id,workspace_id,author_type,author_id`, content hash and relevant revision/source fields.
- `comment_delegation_receipt` only when the opt-in keyed API was actually used: namespace `workspace_id,actor_type,actor_id,originator_id,request_key`, `request_digest,issue_id,comment_id,state,targets,response` hash (`comment_delegation.sql:1–16`).
- Ordinary `agent_task_queue`, collaboration command/group/run/event/notification paths separately; keep all nonexecuted effects at zero and retain unrelated root preparation effects without mixing them into ordinary event counts.

The schema of `inbox_item` includes existing historical foreign keys (`server/migrations/001_init.up.sql:110–123`); this intake proposes none. The repository's hard no-new-FK/no-cascade and concurrent single-statement-index rules apply if scope later expands.

## Related specs and references

- `.trellis/spec/web/frontend/collaboration-recovery-qa.md:133–137`: WS subscription is not inbox delivery; original recipient IDs and runs are separate denominators.
- `.trellis/spec/server/durable-delegation-receipts.md:1–18`, `:73`: existing opt-in ordinary original-request recovery and current-authority bounds.
- `.trellis/spec/server/collaboration-records.md:193–203`: frozen **collaboration terminal result** recipients.
- `.trellis/spec/server/database-test-isolation.md` and `verification-environment.md`: fresh owned DB, exact source/config identity, named run inventory and cleanup.
- `.trellis/tasks/11-13-pay-241-s3-recovery/research/extras-a-r05-independent-check-20261118.md`: participant notification success with ordinary subscriber/inbox **not run**.
- `server/pkg/db/queries/iteration_notifications.sql:24` and `server/internal/service/iteration_notifications.go:157`: persisted inbox-ID conflict precedent.
- External docs were not fetched: all findings concern installed source and existing dependency `github.com/google/uuid v1.6.0`.

## Caveats / Not Found

- No DB, API, browser, test body, build, Provider, configuration mutation or Git operation was performed. No original result was replayed by this research.
- The literal generic comment event replay defect is source-derived and needs a real owned PostgreSQL RED before a product acceptance claim.
- No ordinary source-event uniqueness, notification source ID, or retained original recipient snapshot was found. The recommended stable inbox ID slice prevents duplicates for a source/recipient under fixed writers; it does not add eventual recovery for a crash before any subscriber decision, guarantee mixed old/new server writers, or freeze ordinary late subscribers.
- No existing ordinary shared composer idempotency behavior was found. Opt-in HTTP keyed comment replay and literal listener event replay must stay separately labeled.
- This file does not change parent TC-22 or full S3 status. Proven runtime result notices remain the separately owned Extras-B work.
