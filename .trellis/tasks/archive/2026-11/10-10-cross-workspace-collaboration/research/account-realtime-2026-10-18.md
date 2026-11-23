# Account and issue realtime delivery

Implemented by the `account_realtime` lane on 2026-10-18. The parent task remains
in progress. No commits, new dependencies, or new migrations were made by this
lane. The existing migration 600 outbox index is reused; the main session ran
sqlc for the new queries and the extended access snapshot.

## Behavior and boundaries

- `/ws?scope=account` authenticates a JWT/PAT without requiring or joining a
  workspace. Account subscriptions are limited to the authenticated viewer.
  Explicit issue subscriptions use the same current `ResolveIssueAccess` as
  HTTP, including the independent source-member and participant read bases.
- The verified credential kind, original password version, and deny-only legacy
  `X-Agent-ID`/`X-Task-ID` context survive the WebSocket adapter. Both password
  and legacy accounts are covered. Machine or unclassified principals cannot
  enter collaboration scopes.
- The Hub retains scope metadata in its send queue. Account/issue authorization
  is rechecked at final send, including Redis deliveries. A rejected issue
  payload is dropped and replaced with a content-free access invalidation;
  unrelated account access remains available.
- `CollaborationRealtime` drains the existing retained event outbox. A failed
  relay publish leaves the row pending; retries keep the event ID and do not
  recreate notification rows. Wire data is limited to issue ID, event sequence,
  collaboration revision and authority epoch, or an empty account invalidation.
  Stored payloads, titles, content, resource details and credentials are not
  copied into these frames.
- All existing collaboration-event writers now reach the account/issue delivery
  path. Runtime prepare, first claim, first permit, outcome, proof and group
  settlement also record small ordered invalidation facts in their transaction.
  Replays do not add duplicate facts. New notification read/archive hints and
  source membership-loss hints are account-scoped; their durable HTTP rows and
  the retained 15-second polling/focus/reconnect checks provide recovery.
- Detail responses expose `realtime: { event_seq, authority_epoch }` from the
  same authorized snapshot. `resource_forms` owns the matching optional strict
  core schema addition and malformed-version tests.
- `WSProvider` uses the existing connection for account destinations. WSClient
  reference-counts issue subscriptions, sends them only after authentication,
  replays them on reconnect, rejects old-socket callbacks and redacts protected
  malformed-frame logging.
- The shared account shell mounts `useCollaborationRealtime` once. Ordinary
  changes invalidate Query data without advancing the access epoch or deleting
  drafts, so an in-flight comment POST can finish with its original key.
  Access loss advances the fence before cancellation/cleanup and then reloads
  the surviving source-only projection where appropriate. Source membership
  loss clears old projections but retains independently valid collaboration
  drafts. Reconnect and resubscription perform GET revalidation only; they never
  Resume AI. A denied desired issue subscription can be revalidated when the
  account changes again.

## Owned files

- Server transport: `server/internal/realtime/{hub.go,collaboration.go,broadcaster.go,redis_relay.go}`
  and the corresponding Hub/relay tests, including the new collaboration suite.
- Server delivery/access: `server/internal/service/collaboration_realtime.go`,
  `collaboration.go`, `collaboration_runtime.go`,
  `collaboration_runtime_capabilities.go`, `collaboration_squad.go`,
  `collaboration_groups.go`, and the new realtime service/runtime tests.
- Server wiring: `server/cmd/server/collaboration_scope_authorizer.go`,
  `collaboration_realtime_test.go`, the authorizer/worker startup in `main.go`,
  account listener blocks in `listeners.go`, detail metadata in
  `server/internal/handler/collaboration.go`, post-commit notification invalidation
  in `collaboration_notifications.go`, and the password-WS regression baseline.
- SQL: `server/pkg/db/queries/collaboration_realtime.sql` and the access SELECT
  in `collaboration.sql`; generated output is integrated by the main session.
- Core: `packages/core/api/ws-client.ts` and its account/regression tests,
  `packages/core/realtime/{provider.tsx,index.ts,collaboration.ts,collaboration.test.ts}`,
  `packages/core/collaboration/access.ts`, and `packages/core/types/events.ts`.
- Views: only the realtime import/mount in
  `packages/views/collaboration/account-workbench-page.tsx`. Ownership of that
  shell was released to the creation/attachment lane after this mount landed.

Other agents' changes in these shared files were retained. No changes were made
to the shared API client, collaboration barrel, resource/detail forms or locale
JSON by this lane.

## Validation evidence

Evidence directory:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-product-20261018-nebz79rl`.
Database: task-owned `cwc_product_realtime_nebz79rl` on loopback port 51974.
Go runs use `bash ../scripts/go-test-with-agent-cli-guard.sh go ...` from `server`.

- `realtime-hub-race.log`: `go test -race ./internal/realtime -count=1`, passed.
  New tests: `TestAccountWebSocketDoesNotJoinWorkspaceRooms`,
  `TestCollaborationWebSocketRevocationBeforeSend`,
  `TestAccountWebSocketRequiresAnAuthorizer`. These first failed against the
  workspace-only endpoint and then passed.
- `realtime-service-race.log`: `go test -race ./internal/service -run
  '^TestCollaborationRealtime' -count=1 -v`, all four tests passed: dual read
  bases/machine denial, durable retry/no-private-payload, removed-account
  invalidation, and idempotent runtime transitions.
- `realtime-routes-race.log`: real HTTP and WebSocket DB tests passed under both
  legacy and password modes, including legacy-agent-header rejection, source-only
  access, last-basis loss, reconnect denial and exact snapshot cursor strings.
  Account routing for source loss and notification state plus the five existing
  scope-authorizer regressions passed.
- `realtime-password-race.log`: `TestPasswordRealtimeRevocationBeforeSendAndSubscribe`
  passed all four outgoing/incoming × password-revoked/account-disabled cases.
- `realtime-runtime-regressions.log`: managed batch/proof replay, capacity-one
  squad leader/member/continuation, final leader ordering and separate
  activity/proof regressions passed with `-race`.
- `realtime-core-tests.log`: four files / 32 tests passed. Coverage includes
  account URL/auth, ref-count/reconnect, old socket callbacks, private logging,
  normal-event POST completion, revoked late POST/GET rejection, source-only and
  independent-participant recovery, stale/malformed event rejection, GET-only
  reconnect and resubscription after a denial.
- `packages/views/collaboration/account-workbench-page.test.tsx`: four tests
  passed after the realtime mount. Core typecheck and changed core/views lint
  passed. Core results are saved in `realtime-core-typecheck.log` and
  `realtime-core-lint.log`.
- `realtime-go-vet.log`: `go vet ./internal/realtime ./internal/service
  ./cmd/server` passed. `git diff --check` passed for owned edits.

## Remaining acceptance

The lane does not claim new native Web/Electron acceptance, a live distributed
Redis outage exercise, complete activity projections, or every R52 notification
kind. Runtime events here are invalidation facts; richer execution history and
AI-result notification semantics remain part of the parent product work.
HTTP polling is intentionally retained as recovery for disconnected clients,
older servers and dropped transient membership/notification hints. The parent
owns full TS/Go verification, native UI checks and independent review.
