# CG notification slice verification

Implementation ownership: `service/iteration_notifications.go`, corresponding service tests, `queries/iteration_notifications.sql`, `handler/iteration_notifications.go` and corresponding handler test. The CG owner integrated lifecycle enqueue, generated SQL and suppression tombstones; root integrated the background runner with the handler callback. The existing member-revocation handler test now asserts retained suppression ledger rather than deleted outbox rows. Release remains off.

## Evidence (2026-10-06)

Exclusive migrated database: `multica_i1_notify_20261006_2310` on local PostgreSQL. All Go commands ran through the agent CLI guard.

RED: initial tests failed for absent enqueue/delivery/reminder APIs before implementation. The realtime callback contract test subsequently failed for the absent callback argument before that API change.

Final focused command:

```sh
DATABASE_URL='<exclusive notification test database>' bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -json -race -p 1 ./internal/service ./internal/handler -run '^TestIteration(Notification|Overdue|InboxEvent|MemberRevocation)' -count=1
```

Result: exit 0; **15 tests passed**, no failures or skips. Service package 5.750s; handler package 63.293s (concurrent development load; not a latency benchmark). Raw log `/tmp/i1-cg-notifications-final.jsonl`, SHA-256 `0b2f153ea7ecf9c809f1cef6c441569217f286f9a88d594120b2665d369c22d7`. Per-test names and package completion events are in `notifications-verification.json`. `git diff --check` passed.

Coverage:

- Workspace disable emits one deduplicated workspace summary with nullable iteration identity and no unrelated period title.
- Duplicate recipients/delivery produce one stable inbox ID; callback queries confirm delivered state is already committed. Replay does not republish.
- An injected database trigger fails after inbox INSERT and before marking delivered. Inbox insertion rolls back, no callback fires, persisted retry advances once. Two concurrent recovery workers create exactly one inbox item.
- A second transaction holds the recipient fence; `pg_stat_activity` proves the delivery worker waits. Removing membership and committing forces fresh authorization, producing no inbox.
- Asia/Shanghai midnight and America/New_York DST fall-back repeated hour deduplicate by saved timezone local date.
- Null coordinator falls back to started_by; present but revoked coordinator does not fall back. Rejoining after suppression does not recreate the same day's protected inbox.
- Overdue enqueue leaves the iteration active, issue status in_progress, and the entire running task row unchanged.
- Date extension suppresses a queued overdue reminder, without callback. Disabled workspace settings prevent new reminders. Release-off worker startup does not access storage or publish.
- Twelve retries preserve exponential delay (5s through 900s cap), then enter dead_letter. Dead letters are not automatically delivered. Queue counters report pending/dead_letter accurately.
- 101 active workspaces produce at most 100 discovery rows; successive passes drain the remaining eligible workspace without losing it.
- Handler callback publishes the existing inbox:new envelope with correct workspace, recipient and stable item identity.
- Real handler membership revoke removes only the revoked recipient's protected inbox, retains suppression tombstones, preserves other recipients/workspaces and durable operation audit; injected final-member-delete failure rolls all cleanup back.

## Runtime and scalability

Delivery resolves current source identity, membership and overdue relevance under ordered workspace/member/iteration/outbox locks. Inbox insertion and delivered marking share one transaction; realtime callback runs only after successful commit and only for a newly inserted inbox item. Revocation retains content-free suppression rows to preserve operation/day deduplication.

Delivery checks persisted due work every 5 seconds, at most 100 candidates per pass. Reminder discovery and queue-health sampling run once per minute. Discovery joins current membership, uses the saved timezone, excludes existing local-day ledger entries and returns at most 100 rows. The active-iteration scan remains O(active workspaces), using the existing partial active index; no claim of constant-cost global discovery or fleet-scale P95 is made. Filtering and bounded batches remove the previous all-active lock/recheck loop every 5 seconds. The 101-workspace test proves batch draining, not a performance threshold.

`IterationNotificationCounts` exposes content-free pending/dead_letter counts; the worker emits them in structured `iteration notification queue` logs once per minute. No titles, descriptions or credentials are logged. Count sampling may scan retained pending/dead-letter rows; full operational performance belongs to VG.

The loop sends local inbox records only; no execution or external messaging. Supported lifecycle kinds: `start`, `end`, `cancel`, `dates_changed`, `disable`. Disable uses only `details.kind=disable` and routes to the workspace iteration list. Inbox type is `iteration`, with `details.iteration_id` and `details.kind` for UG routing.

## Remaining integration verification

Lifecycle enqueue paths and full CG/VG acceptance are owned by their existing task lanes. This focused evidence is not a full CG/VG pass claim. UG must render the iteration inbox type. Realtime fanout retains the existing inbox event authorization/routing contract; this slice does not redesign global realtime revocation.

Additional integration repair: sqlc DBTX now includes SendBatch. Twelve unrelated test fake types in eleven files explicitly reject unexpected batch invocation; embedded onboarding fake inherits the method. Guarded `go -C server test ./... -run '^$' -p 2` passed after these compatibility fixes (`/tmp/i1-notify-compile.log`). This is compilation evidence, not test-behavior coverage for those unrelated modules.
