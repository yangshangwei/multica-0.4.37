# Ordinary comment notification replay

## 1. Scope / trigger

Apply to the ordinary `comment:created` listener in `cmd/server/notification_listeners.go`. Ordinary `issue_subscriber`/`inbox_item` delivery is separate from collaboration participants and terminal-group notifications. A WebSocket subscription or one grouped inbox row does not prove persisted notification delivery or deduplication.

## 2. Signatures

`notifyIssueSubscribers` uses `CreateCommentInboxItem` only for `new_comment` with a valid, nonzero original comment UUID. Its sqlc parameters are `ID`, `WorkspaceID`, `RecipientID`, `IssueID`, `ActorID`, `CommentID` (`pgtype.UUID`), `Severity`, `Title` (`string`), `Body`, `ActorType` (`pgtype.Text`) and `Details` (`[]byte`). The query fixes `recipient_type='member'` and `type='new_comment'`.

The delivery ID is UUIDv5, with the original comment UUID as namespace. The name is the exact bytes:

```text
multica:ordinary-comment-inbox:v1:member:new_comment:
    + workspace UUID bytes + issue UUID bytes + recipient UUID bytes
```

The UUID fields are fixed-width 16-byte values, not variable text. Keep this v1 domain and tuple encoding stable across processes. Changing them requires an explicit persisted-identity/adoption design.

## 3. Contracts

The dedicated query inserts only when no retained row matches workspace, issue, member recipient, `new_comment` and the canonical original comment ID. It compares `lower(details->>'comment_id')` with UUID canonical text without casting historical JSON. Read, archived and historical duplicate rows remain retained with their original IDs, title, body, details, actor, timestamp and state.

`ON CONFLICT (id) DO NOTHING RETURNING *` gives concurrent current writers one persisted winner. `pgx.ErrNoRows` means this delivery did not create a new row; skip `inbox:new`. Other database errors retain the existing error reporting. Do not use `DO UPDATE`, resurrect read/archive state or delete historical duplicates.

Ordinary recipients remain the current active subscribers after actor exclusion, delivery-tier and mute checks. An ordinary late subscriber can receive a re-emitted old comment once. This differs from collaboration terminal results, whose effective recipients are frozen in the terminal transaction.

Other notification types and absent, malformed or zero synthetic comment identities retain the existing generic UUIDv7 writer. This slice does not guarantee generic status-event deduplication, mixed old/new server writers, recovery before any delivery decision, or automatic idempotency in the ordinary comment composer. The opt-in HTTP comment receipt and literal bus-event replay are separate contracts.

## 4. Validation and error matrix

| Input / operation | Required result |
| --- | --- |
| Same original comment/recipient, sequential or concurrent replay | One retained inbox row and one `inbox:new` |
| New listener/bus replays that source | Same persisted ID, no process-local dedup assumption |
| Equivalent UUID case | Same identity; retained uppercase legacy details are adopted |
| Read/archive or old random-ID row already exists | Preserve the complete old row; no new broadcast |
| Changed payload with the original comment ID | Do not rewrite or reopen the first delivery |
| Different actual comment or member | Separate delivery identities |
| Actor, Agent, nonwatcher, opt-out or muted comments | Preserve current exclusions and subscriber rows |
| Current late subscriber | One new delivery for that recipient; previous rows unchanged |
| Invalid/empty/zero synthetic identity or other producer | Existing generic UUIDv7 behavior; no new replay guarantee |
| Dedicated insert returns `pgx.ErrNoRows` | Successful deduplication, no `inbox:new` |
| Another database error | Report the failure; it must not masquerade as deduplication |

## 5. Good / base / bad cases

Good: replay the same saved comment event through a newly registered listener and compare every retained ID, row and broadcast. Base: preserve legacy random-ID rows and their original state. Bad: resend equal text as a new HTTP comment, or inspect only the grouped UI, and claim original-event deduplication.

## 6. Required tests

`cmd/server/notification_comment_replay_test.go` is the canonical listener/storage suite. Its seven tops and 22 leaves cover sequential/concurrent replay, retained state, legacy rows, distinct tuples, recipient controls and the explicitly scoped legacy fallback. Use real `testutil` rows and forwarded SQL, not replaced query results. Both concurrent publishers must reach the real insertion boundary and be released/joined on failure. Observe database errors/listener panics as well as row and broadcast counts.

Run the selected notification/subscriber suite on separate fresh normal/race databases and compare the full name sets. Current verified scope is 35 tops/57 names per mode, including the new 7 tops/29 names. Include archived/read rows in independent audits. Preserve fixture-produced configuration invalidation history and the original failed rounds.

Web/Desktop subscription acceptance separately checks real Subscribe/Unsubscribe requests, persistent subscriber state, exact recipients and IDs, the original comment anchor, and actual reconnect/reload. It cannot replace literal listener-event replay or prove controlled-run notifications.

## 7. Wrong vs correct

Wrong: an in-memory set suppresses this process's repeat, or every repeat generates another random inbox ID. Correct: a stable source/member delivery ID and the existing primary-key conflict preserve one persistent delivery, including after a fresh bus registration.

Evidence: PAY-241 S3 `research/ordinary-comment-replay-red-20261119.md` and `research/ordinary-notification-green-independent-check-20261119.md`; final normal/race and original logger-failure records are retained in the task's private evidence root. Full S3 and broader notification guarantees require their own acceptance.
