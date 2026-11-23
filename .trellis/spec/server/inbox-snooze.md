# Personal Inbox snooze

## 1. Scope / trigger

Apply to ordinary member Inbox storage, delivery, read/archive, expiry and the
shared Web/Desktop reader. Collaboration notifications remain separate.
Snooze is a viewer's notification choice; it never changes issue state/dates,
subscriptions or agent execution.

## 2. Signatures

- `GET /api/inbox/snoozed` returns a raw Inbox item array.
- `POST /api/inbox/{id}/snooze`: RFC3339 `snoozed_until`, required nonnegative
  `expected_revision`.
- `POST /api/inbox/{id}/unsnooze`: required `expected_revision`.
- `POST /api/inbox/{id}/snooze/undo`: server-issued `undo_token`.
- Receipt: `workspace_id`, `group_kind`, `group_id`, `revision`, nullable
  `snoozed_until`/`reminded_at`, optional `reminder_reason`/`undo_token`.
- `internal/inboxstate`: `Group`, `State`, `LockScope`, `Lock`, `Snooze`,
  `Unsnooze`, `Undo`, `Expire`, `Deliver`, `DeliverTx`, `Payload`.
- `Deliver(ctx, TxStarter, db.CreateInboxItemParams)` owns its transaction;
  `DeliverTx(ctx, pgx.Tx, params)` borrows one. Both return
  `Delivery{Item, Created, Suppressed, State}` for postcommit publication.
- Migration 886 introduces `inbox_snooze`/`inbox_critical_source`; 887–889
  build the unique/due indexes concurrently in separate single-statement files.
  No foreign keys or implicit table-created indexes.

## 3. Contracts

### Group transitions and effective reads

Identity is `(workspace_id, recipient_type, recipient_id, group_kind, group_id)`.
Issue-associated notifications share one group; issue-less notifications use
their own ID. Resolve an owned real anchor with `loadInboxItemForUser`, then
recheck authority inside the transaction.

`LockScope` follows workspace/member/subscriber fence ordering; `Lock` takes
the transaction advisory group fence before establishing its state row. Every
producer participates, including the first notification/snooze. Insert, source
claim, wake/suppression and revision commit together; publish only afterward.
Borrowed transactions must use `DeliverTx`, never an independent nested write.

Future `snoozed_until > now()` hides the group. Expired unmaterialized plans
project unread and acquire reminder ordering. Individual/bulk acknowledgment
consumes that marker atomically; raw `read=true` must not skip overdue groups.
Archive-all-read preserves their newly unread reminder. Main-list bulk actions
exclude future snoozes. Representatives/read counts follow the newest original
notification, not raw unread-row totals. Sort with
`greatest(created_at, reminded_at)`; preserve original ID/time/comment anchor.
Snoozed groups sort by deadline.

`Snooze` canonicalizes the requested deadline to UTC microsecond precision
before final-clock validation, persistence and receipt construction. pgx stores
PostgreSQL timestamps at that precision; the immediate receipt, later list and
undo/replay must report the same instant. Tests inject a nonzero nanosecond
remainder explicitly so a microsecond-only host clock cannot hide the boundary.

Undo consumes only the last server snapshot/token at the current group version.
Read/archive, another schedule/reminder, expiry and conflicting new critical
delivery invalidate it. Never restore client Query snapshots over newer state.
Cancelled state rows retain monotonic versions. Explicit workspace/issue/user/
member cleanup removes plans and receipts. Expiry rechecks authority and active
rows, never unarchives old failures, and leaves failed processing retryable.
Refuse active-plan rollback before any 886–889 index can be removed.

### Source identity and producer semantics

Ordinary snoozed delivery persists but emits no banner-triggering `inbox:new`;
content-minimal `inbox:updated` recovers its caches. Critical wake is only a
genuinely new eligible direct `mentioned` (`mention_scope=member`),
`issue_assigned` or `task_failed`. `@all`/`@squad`, arbitrary severity and
`autopilot_paused` do not acquire direct-wake authority.

Consume exact source/recipient/group identity even while unsnoozed: original
comment UUID; issue UUID plus original committed revision for description
mention/assignment; failed task UUID; saved delegated-exhaustion comment UUID.
Do not read a later current revision/task to rename a replayed event. Preserve
receipts through read/archive/subsequent snooze. Legacy baselines consume
relevant committed-visible source IDs during Snooze, plus committed issue
revision watermark. PostgreSQL transaction-start `now()` or write-time
`clock_timestamp()` is not a commit-visibility witness: a source opened before
Snooze but committed afterward remains new.

Validate known source strings while preserving raw unknown/numeric JSON
details. Preserve each actual writer inside the fence: ordinary comment UUIDv5
v1 query, triage title/details upsert, project/iteration stable-ID `DO NOTHING`.
Generic insertion must not replace retries or resurrect read/archive history.
All ordinary listener/task/quick-create/autopilot/triage/project/iteration
sources participate. Solved-failure autoarchive takes the group fence and clears
plans only when no active rows remain. A postcommit sink without a Bus must
carry typed `Delivery`, not discard suppression and re-query state later.

### Shared reader and interaction

Optional projections are `snoozed_until`, `snooze_revision`, `reminded_at`,
`reminder_reason`; old omissions remain supported. Essential new receipts are
strict: malformed JSON cannot report success or enable undo. Snoozed-list
failure is a visible error, not a fabricated empty view.

Receipts also confirm the operation: positive advancing revision, the exact
requested absolute deadline for snooze (including normalized fractional
precision), and a cleared deadline plus valid reminder instant for unsnooze.
The notification anchor is not an issue group UUID; the handler derives its
owned group, so do not compare an anchor directly with `group_id`.

Queries/mutations pass captured workspace ID with `workspaceRequestInit`.
`inbox:updated`/`inbox:unread` require both personal-event and recipient-routing
registration. Refresh the source workspace plus account summary, independent
of active workspace; legacy unscoped events/reconnect recover all cached
workspaces. Query owns state; focus/reconnect/30s refresh repairs missed events.

Use one shared clock/menu/detail picker. Day presets use local-calendar 09:00;
hours use durations. Custom time must be future and reject nonexistent local
times. Unchanged edits retain the exact original instant, seconds and repeated
DST offset. Await success before selection changes, preserve failed drafts,
use localized safe errors and prevent stale detail from instantly reading a
revived reminder. Snoozed tab keys/title queries are distinct. Inbox coarse
controls are at least 44px; generic menu props preserve other defaults.

## 4. Validation / error matrix

| Condition | Outcome |
| --- | --- |
| Missing/invalid/past time or required revision | 400, no committed change |
| Foreign/unavailable anchor or lost membership | Resource denial, no reminder |
| Stale revision/undo or consumed token | 409, preserve/refresh newer state |
| Original important source replay after another snooze | No new row/wake/banner |
| New committed source after visible baseline | One eligible retained delivery/wake |
| Ordinary future-snoozed notification | Persist/group, state invalidation, no banner |
| Expired raw-read group marked all read | Consume marker, refresh stays read |
| Expired raw-read group archived all read | Newly unread reminder stays active |
| Valid empty Snoozed array | Empty view with return control |
| Malformed response/network failure | Visible error, no fabricated success/undo |

## 5. Good / base / bad cases

Good: one transaction fence, exact source receipt, authoritative version and
source-workspace recovery without changing original notification identity.
Base: old projections omit optional fields; unknown details/types remain
ordinary without invented critical authority. Bad: browser timer owns expiry,
random UUID proves new source, timestamp proves commit, raw reads decide overdue
bulk actions, or a generic writer silently changes another producer's retries.

## 6. Required tests / assertions

Canonical state/handler/listener suites must prove real first-fence orders,
source visibility, source claims/re-snooze replay, legacy adoption, numeric
details, stable scheduled writes, undo/critical races, overdue acknowledgment,
archive non-resurrection, cleanup and unchanged IDs. Run normal/race on
coordinator-recorded owned databases separate from API/browser fixtures.

Fixtures that directly delete Inbox rows or their owners explicitly remove
matching `inbox_snooze`/`inbox_critical_source` rows first. Keep this cleanup in
the owning fixture, with checked errors; generic `testutil` row deletion does
not infer dependent relationships. Final audits count both new tables and
connections, rather than treating zero Inbox rows as a complete cleanup proof.

Core API/query/mutation/time/realtime/tab suites own parsing, calendar/DST,
captured headers, counts, failure retention and cache scopes. Views owns
accessible wiring, exact unchanged edit, safe errors, focus/selection and read
guards. Production Web/native Electron proves integrated user/device recovery;
prototype images do not. Keep migration rollback/cleanup registration tests and
docs-bundle source parity. Execution evidence lives in the Inbox snooze task;
this spec is a maintenance contract, not runtime acceptance by itself.

## 7. Wrong vs correct

Wrong: insert UUIDv7, publish `inbox:new`, then separately query snooze.
Correct: preserve the real writer in `Deliver`/`DeliverTx`, commit its claim and
state decision together, then publish only the committed result.

Wrong: undo restores old Query cache or source age compares with `now()`.
Correct: server version/token protects undo; committed-visible IDs and original
issue revision identify previously known sources.
