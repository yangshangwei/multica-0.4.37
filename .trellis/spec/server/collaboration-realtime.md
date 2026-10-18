# Account and issue collaboration realtime

## 1. Scope / Trigger

Apply to collaboration WebSocket authentication, scoped delivery, event outbox,
runtime invalidations and the shared account Query lifecycle. Realtime hints do
not replace server authorization or constitute an AI control action.

## 2. Signatures

`/ws?scope=account` authenticates an existing human session/PAT without a workspace.
`collaborationScopeAuthorizer` authorizes account and explicit issue scopes using
`CollaborationService.ResolveIssueAccess`, delegating ordinary scopes to
`dbScopeAuthorizer`. `CollaborationRealtime` drains retained
`collaboration_event.outbox_state` through the existing concurrent outbox index.
Detail includes optional `realtime: {event_seq, authority_epoch}` using exact
decimal strings from the same authorized snapshot.

`useCollaborationRealtime(scope, issueId)` is mounted by the shared account shell;
`WSClient` owns authentication, reference-counted subscriptions and reconnection.

## 3. Contracts

Account connections join the viewer's account scope and existing own user room.
They do not join a workspace room. Explicit issue scopes
retain independent source-membership and collaboration-participation authority;
external participants do not join a source workspace room. Preserve verified
credential class, the original password version and deny-only legacy agent/task
headers through the adapter. Machines and unclassified callers cannot subscribe.

The Hub queues scope metadata with each payload. Revalidate at final send, also
for relayed messages; subscribe-time checks alone cannot protect queued content.
A rejected issue delivery drops the payload and sends a content-free access hint
through the viewer's still-authorized account. Unknown/malformed frame text must
not become a private logging channel.

The retained event is the outbox. Failed publication leaves it pending; retries
reuse its ID and do not recreate notifications. Frames contain only scope IDs,
event sequence, revision and authority generation, or an empty account hint.
Do not forward stored event bodies, titles, runtime details or credentials.
Runtime preparation, first claim/permit, outcome, proof and group settlement
record ordered invalidation facts in their owning transaction; replay is inert.
Notification state and source-member loss also invalidate the appropriate account.

Normal changes invalidate/refetch Query data without advancing the access epoch
or clearing drafts; an in-flight comment retains its original request and result.
Actual access loss advances the fence before cancellation and private cleanup,
then revalidates any surviving source-only projection. Losing source membership
preserves independently valid participation and its drafts. Reconnect and denied
subscription recovery perform reads only. They never Resume or submit execution.

Ignore stale socket callbacks and old issue events at or before the authorized
snapshot cursor. Account/source-authority hints without an issue cursor cannot
be dismissed using an unrelated issue's sequence. Keep the existing polling,
focus and reconnect recovery for lost transient hints and older servers.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| No workspace on an account connection | Authenticate the account normally |
| Another account or unauthorized issue scope | Deny without broadening rooms |
| Source member loses source access but remains a participant | Refresh projection; keep participation |
| Last read basis revoked before send | Drop queued payload; invalidate private view |
| Normal change arrives during a write | Preserve original key and completion |
| Relay publish fails | Roll back delivery marker; retry same event |
| Reconnect / old socket callback | Revalidate snapshots / ignore stale callback |

## 5. Good / Base / Bad Cases

- Good: a participant removal clears an already open account detail even though
  its issue-room subscription is no longer authorized.
- Base: ordinary workspace/task/chat rooms keep their existing authorization.
- Bad: treat a workspace departure as departure from every collaboration, or
  expose a queued body because the subscription was authorized earlier.

## 6. Tests Required

Hub `collaboration_test.go` uses actual WebSockets for workspace-free account
connections, authorizer requirements and final-send denial.
`TestAccountWebSocketUnknownFrameDoesNotLogPrivateInput` verifies untrusted frame
types stay out of logs. Server route tests
cover legacy/password and agent-header boundaries, independent read bases and
snapshot cursor identity. Service realtime suites cover original outbox IDs,
retry, removed recipients, redaction and runtime replay. Core WS/realtime suites
cover ref-counts, reauth, late reads/writes, normal concurrent events and GET-only
recovery. Browser tests must observe a live account WS and clear private content
without reload; a poll-only success is not realtime integration evidence.

## 7. Wrong vs Correct

Wrong: broadcast a stored event payload to a workspace room and trust the last
successful subscribe check.

Correct: publish a scoped invalidation, revalidate at send, and fetch the current
authorized projection with a fenced Query lifecycle.
