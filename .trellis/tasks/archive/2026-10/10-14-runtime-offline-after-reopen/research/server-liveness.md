# Research: Server liveness, online projections, and credential recovery

- Query: Can successful daemon heartbeat ACKs coexist with offline runtime/agent displays after reopening the next day? What changes when the user's token-renewal script restored service?
- Scope: Internal server heartbeat handlers, Redis liveness, PostgreSQL state, sweeper, runtime/agent read models, authentication and PAT renewal. Daemon sender, desktop lifecycle and the user's script are owned by other research lanes.
- Date: 2026-10-14

## Findings

### Evidence boundary and updated conclusion

The logs establish successful registration and repeated acknowledged heartbeats during their recorded interval. The user subsequently clarified that they did **not** correlate that interval with the offline display. Their recovery script performs **stop old daemon → log in again/refresh token → restart daemon**, after which service recovers. This combined intervention cannot isolate credential replacement from process restart. Therefore credential handoff and daemon lifecycle take priority over a server heartbeat defect, while neither natural token expiry nor a particular component is proved solely by this recovery.

The main session checked `v0.6.5` against current HEAD and reported no difference for `server/internal/handler/daemon.go`, `heartbeat_scheduler.go`, `runtime_liveness_store.go`, and `server/cmd/server/runtime_sweeper.go`. It subsequently confirmed the inspected authentication middleware, token/session checks, PAT handler and daemon WS hub are also unchanged. The coordinator checked the 14-line `auth/password_revoke.go` difference: it adds collaboration revocation identity handling; the PAT revocation and owner-runtime offlining described here already exist in v0.6.5. Runtime/agent read-model references also describe current checked-out source. This research role did not run git operations. The user ultimately confirmed all incident components are v0.6.5, rather than this working tree's current build.

The main session additionally identified desktop commit `296c377a4` (2026-10-12), `fix(desktop): restore device connectivity after password binding`, as included in `v0.6.6`. Its reported fixes concern cached same-user PAT reuse, a same-user token change not restarting the daemon, and startup authentication failure not surfacing `auth_expired`. These are findings from the desktop/version lane, not independently re-investigated here; they provide a more targeted planning lead than changing the server liveness algorithm.

### Files found

| Path | Responsibility |
| --- | --- |
| `server/internal/handler/daemon.go` | Registration, deregistration, HTTP/WS heartbeat adapters and shared heartbeat processing. |
| `server/internal/daemonws/hub.go` | WS authentication checks, frame dispatch and ACK transport. |
| `server/internal/handler/heartbeat_scheduler.go` | Synchronous transitions and batched timestamp writes. |
| `server/internal/handler/runtime_liveness_store.go` | Redis TTL liveness and no-Redis fallback. |
| `server/pkg/db/queries/runtime.sql` | Runtime upsert, online/offline transitions and stale predicates. |
| `server/cmd/server/runtime_sweeper.go` | Every-30-second stale-runtime detection and offline notifications. |
| `server/internal/service/agent_runtime_lookup.go` | Direct runtime row lookup with metrics, without a row cache. |
| `server/internal/handler/runtime.go` | Runtime list visibility and DB-to-response serialization. |
| `server/internal/handler/agent.go`, `squad.go` | Agent state plus coarse runtime availability projection. |
| `server/cmd/server/main.go`, `router.go` | Shared DB/Redis wiring, scheduler lifecycle and authenticated routes. |
| `server/internal/middleware/daemon_auth.go` | Authentication before daemon HTTP endpoints and WS upgrade. |
| `server/internal/auth/password_token.go`, `password_session.go`, `password_revoke.go` | Password-mode token/version checks and account-wide revocation. |
| `server/internal/handler/personal_access_token.go` | PAT issuance, conditional renewal and revocation. |
| `server/pkg/db/queries/personal_access_token.sql`, `daemon_token.sql` | Persisted credential validity predicates. |
| `server/internal/auth/pat_cache.go`, `cookie.go`, `jwt.go` | Auth cache/expiry bounds, configured JWT lifetime and token formats. |
| `server/internal/daemonws/connection_diagnostics.go` | Safe WS error classifications such as `session_invalid`. |

### 1. What a heartbeat ACK proves

1. HTTP `DaemonHeartbeat` reads the runtime and then calls `processHeartbeat` (`handler/daemon.go:1235`, `:1265`). WS `HandleDaemonWSHeartbeat` performs a fresh lookup, workspace/managed-scope checks and calls the same function (`handler/daemon.go:1319`). `RuntimeLookup.Get` calls the DB directly (`service/agent_runtime_lookup.go:43`); this is not an indefinitely cached online snapshot.
2. `processHeartbeat` calls `recordHeartbeat` before constructing `status: "ok"` (`handler/daemon.go:1402`). An error returns before the normal ACK. The WS hub only sends an ACK after its callback returned without error (`daemonws/hub.go:982`). A deleted-runtime ACK is a distinct `runtime_gone` response, not ordinary health (`handler/daemon.go:1327`).
3. This is **not** an ACK of an ordinary WebSocket ping. The callback has processed the named runtime's application heartbeat. It also does not prove Claude can complete a model invocation; provider credentials and task execution are outside this heartbeat contract.
4. With production batching, a normal ACK can precede the timestamp-only DB UPDATE, but an observed offline runtime takes the synchronous status-restoration branch. Consequently, repeated normal ACKs plus a persistently offline row for the same runtime in the same DB require additional evidence; that is not the ordinary steady-state behavior.

### 2. Liveness timing and online restoration

- Redis presence uses `SET mul:runtime:hb:<runtime-id> 1` with a 90-second TTL (`runtime_liveness_store.go:70`, `:96`; `daemon.go:1167`). It is shared Redis state, not per-WS-connection memory.
- `recordHeartbeat` requires a DB write when Redis is absent, status is not online, `last_seen_at` is absent, or DB age reaches 60 seconds (`daemon.go:1358`). A Redis touch failure forces the DB path and logs `liveness touch failed; falling back to DB heartbeat` (`:1375`).
- Production batches only already-online rows with valid timestamps. Offline/never-seen rows use the synchronous fallback (`heartbeat_scheduler.go:117`). `MarkAgentRuntimeOnline` sets `status='online'`, `last_seen_at=now()` and `updated_at=now()` (`queries/runtime.sql:202`).
- The batch interval is 30 seconds. Nominal maximum DB age under healthy scheduling is `60 + 15 + 30 = 105 seconds`, below the 150-second stale window (`heartbeat_scheduler.go:96`; `service/task.go:191`). This is a healthy-timing invariant, not a guarantee under arbitrary DB stalls, process suspension or clock skew.
- A failed bulk UPDATE logs `heartbeat batch flush failed` and does not requeue failed IDs; the next heartbeat schedules them again (`heartbeat_scheduler.go:188`). Persistent flush errors can allow stale DB timestamps despite earlier ACKs. With fresh Redis liveness alone, the sweeper still skips those rows; false offlining also requires Redis absence/error/mismatched wiring or another offline writer.
- If a sweeper flips a queued row offline before flush, the bulk UPDATE intentionally leaves it offline. The next heartbeat performs another DB lookup, sees offline and synchronously restores it (`heartbeat_scheduler.go:208`; `heartbeat_scheduler_test.go:202`). This permits a short race window; it does not explain indefinite offlining across continuing healthy beats.

### 3. Sweeper and multi-instance behavior

- Every 30 seconds, the sweeper selects DB rows with `status='online'` and `last_seen_at < now()-150s` (`runtime_sweeper.go:24`, `:224`; `queries/runtime.sql:232`). It excludes Redis-alive candidates when that store is usable (`runtime_sweeper.go:370`). If Redis is unavailable or its read fails, it trusts the DB stale window.
- The final offline UPDATE repeats both online status and stale timestamp predicates, preventing a concurrent committed timestamp refresh from being overwritten (`queries/runtime.sql:241`). Confirmed-offline rows trigger Redis Forget and a workspace `daemon:register` event with `action: stale_sweep` (`runtime_sweeper.go:280`).
- Registration upserts an existing `(workspace_id, daemon_id, provider)` runtime, normally as online, and refreshes `last_seen_at` (`handler/daemon.go:569`; `queries/runtime.sql:56`). It need not allocate a new runtime ID each restart.
- The server uses one `DATABASE_URL` pool (`main.go:362`). Heartbeat processing and the sweeper both select a Redis/noop store from the same `storeRedis` client (`router.go:523`; `main.go:672`). The scheduler is injected before routing and its Run loop starts with the sweeper (`main.go:617`, `:727`, `:735`). There is no read-replica routing in these paths.
- Normal multiple API replicas sharing the same DB/Redis do not inherently make runtimes offline. Separate DBs behind one address can create a genuine write/read split; mismatched Redis/failed relay wiring can disrupt liveness hints or notifications. Those are deployment hypotheses requiring instance/configuration evidence, not facts shown by these logs.
- Runtime GC has a seven-day offline retention policy (`runtime_sweeper.go:49`), independently scheduled hourly. Ordinary overnight inactivity is not proof that the runtime row was deleted.

### 4. Runtime/agent read models and a confirmed notification gap

- `/api/runtimes` reads DB runtime rows and serializes `status` and `last_seen_at` without consulting Redis (`handler/runtime.go:738`, `:52`; `queries/runtime.sql:1`). It applies workspace and owner/visibility scope.
- `AgentResponse.Status` is the stored agent status; it is not rewritten to offline by heartbeats or the runtime sweeper (`handler/agent.go:219`, `:244`). Agent availability follows its bound runtime. For private runtimes hidden from regular members, the server supplies a coarse `runtime_availability`; owner/admin viewers normally derive from the full runtime list (`agent.go:1008`). This explains why one runtime state can make several agent displays change together without separate agent registration failures.
- Server coarse availability is online whenever runtime status is online. Otherwise, recent `last_seen_at` under five minutes produces unstable; older/absent timestamps produce offline (`handler/squad.go:825`). It does not reinterpret every 60-second DB flush interval as an offline runtime.

| State-changing path | DB change | Workspace invalidation/event |
| --- | --- | --- |
| Successful daemon registration | Upsert status + timestamps | `daemon:register` with runtimes (`daemon.go:836`). |
| Explicit daemon deregistration | Set offline | `daemon:register`, `action: deregister` (`daemon.go:1117`). |
| Stale-runtime sweep | Set offline | `daemon:register`, `action: stale_sweep` (`runtime_sweeper.go:291`). |
| Offline runtime restored by heartbeat | Synchronous `MarkAgentRuntimeOnline` | **None** in `recordHeartbeat`, `processHeartbeat` or either scheduler (`daemon.go:1358–1554`; `heartbeat_scheduler.go:52`, `:117`, `:188`). |
| Ordinary timestamp batch flush | Refresh `last_seen_at` | None, by design. |

Thus pure heartbeat restoration after suspension/partition can leave an already-mounted client with stale offline cache until another invalidation/refetch. This is a confirmed code gap, not a confirmed explanation for the incident. The supplied startup also registered both runtimes, and registration **does** broadcast. A cold restart incident needs evidence of missed registration delivery, frontend resubscription/refetch ordering or a different time interval before blaming the gap. Other agents own that client-side verification.

### 5. Credential expiry, renewal and the user's new evidence

The same personal token can authorize multiple workspaces: PAT auth resolves a user, then each daemon endpoint validates workspace access (`middleware/daemon_auth.go:212`; `handler/daemon.go:1253`). A single shared PAT becoming unusable can stop heartbeats/reconnection for both runtime IDs. A workspace-bound MDT is narrower; failure of one MDT alone is not automatically all-workspace failure.

| Credential condition | Confirmed server behavior | Diagnostic value |
| --- | --- | --- |
| PAT expired or `revoked=true` | Lookup filters it out (`queries/personal_access_token.sql:6`). | A future logged expiry does not rule out revocation; historical logs do not establish current validity. |
| PAT in password mode has old `auth_version`, disabled account, or forced password change | `CheckPasswordVersion` rejects even a future-expiry PAT (`auth/password_token.go:31`, `:86`; `password_session.go:55`). | Inspect account/version facts and any reset/disable operation, not only date. |
| Account-wide password credential revocation | Revokes all user PATs, removes task/daemon tokens, forces all user-owned runtimes offline (`auth/password_revoke.go:21`). | Concrete mechanism for simultaneous workspace runtime loss. No such operation is evidenced in the provided log. |
| Expired/revoked PAT sent to renewal | 401; the existing renewal endpoint cannot revive it (`personal_access_token.go:225`; SQL `:41`). | The user's script does re-login and restart the process, so its success cannot be attributed to this in-place renewal endpoint. |
| Valid PAT with more than seven days remaining | 200, `renewed:false`, authoritative `expires_at` (`personal_access_token.go:250`). | `auth token not yet eligible for renewal` with 2027-01-12 expiry is consistent with this no-op at that recorded time. |
| Valid expiring PAT with seven days or less remaining | Conditional in-place extension to now + 90 days; raw secret unchanged (`personal_access_token.go:268`; SQL `:29`). | No token rotation is required for ordinary renewal; concurrent renewals are idempotent. |
| PAT without expiry | 200, `renewed:false`, empty expiry (`personal_access_token.go:253`). | Not all manually created PATs have a hardcoded 90-day expiry; issuance uses optional positive `expires_in_days` (`:91`). |
| JWT/browser session | `AUTH_TOKEN_TTL`, default 30 days (`auth/cookie.go:23`, `:74`). | “Next day” alone is not evidence of a default one-day token expiry. Actual config and issuance time matter. |

Both PATs (`mul_`) and daemon tokens (`mdt_`) are 44 characters (`auth/jwt.go:60`, `:69`); `token_len=44` alone cannot identify which was loaded. Do not request or persist the raw credential to disambiguate it.

#### WS differences that matter when interpreting ACKs

- In password mode, the WS hub calls `CheckPasswordToken` at upgrade and again in its authorization closure before reads, writes and pings (`daemonws/hub.go:347`, `:405`, `:839`, `:1024`). It revalidates persisted expiry/revocation/version without the legacy token cache (`auth/password_token.go:14`). A token/session failure closes the connection and logs `daemon websocket authorization failed` with `error_class=session_invalid` (`hub.go:251`; `connection_diagnostics.go:15`). Sustained normal ACKs cannot, in that same interval, be explained by that same credential already failing these checks.
- In non-password mode, `DaemonAuth` checks the credential during WS upgrade. The per-frame closure still checks runtime scope, but does not rerun the original PAT/JWT expiry lookup (`hub.go:405`; `handler/daemon_ws.go:108`). An already-established socket may therefore continue across the credential's expiry and fail when the app reconnects the next day. This is a code-supported mechanism consistent with a reconnect-only auth problem; the actual deployment auth mode and token history remain unknown.
- PAT/MDT Redis auth caches are bounded to `min(10 minutes, remaining lifetime)` (`auth/pat_cache.go:19`, `:85`). They do not intentionally extend natural expiry. Revocation invalidates the shared PAT entry (`handler/personal_access_token.go:328`). Password-mode auth bypasses those caches.

#### Useful failure signatures, with limitations

- `/api/daemon/register`, `/heartbeat`, `/workspaces` and `/ws` all pass through `DaemonAuth` (`router.go:1491`). In password mode, invalid credentials return 401; DB/dependency failures return 503 with a generic body (`middleware/daemon_auth.go:106`).
- Non-password PAT and MDT lookups log `daemon_auth: invalid PAT` / `daemon_auth: invalid daemon token`, then return 401 for any lookup error (`daemon_auth.go:142`, `:230`). This conflates absent/expired/revoked tokens with DB errors; correlate the structured `error` and DB health before claiming natural expiry. Invalid JWT logs `daemon_auth: invalid token` (`:265`).
- A valid PAT presented to `/api/tokens/current/renew` can return 200/no-op, 200/renewed, 401/no longer valid, or 500/lookup or renewal failure; non-PAT input is 400 at the handler, while machine actors are blocked by the route (`personal_access_token.go:193`; `router.go:1961`).
- Password-mode WS `session_invalid` is distinct from `runtime_scope_invalid`, `timeout` and `unavailable`; preserve those distinctions in recovery diagnostics.

### Ranked hypotheses and falsifiers

1. **Credential/profile handoff or daemon lifecycle recovery.** Raised by the newest user evidence and the desktop fix found by the main session. The script combines re-login with process replacement, so either can explain recovery. Verify pre-repair 401/session-invalid events, expiry/revocation/version facts and which token/profile the running daemon actually consumed. Falsify a credential failure with contemporaneous successful authenticated requests using the same active profile/token and matching runtime IDs; do not interpret a successful combined script as an isolated expiry experiment.
2. **Recovered backend with stale client offline state.** Supported by the confirmed heartbeat-restoration notification gap. Requires `/api/runtimes` returning online while UI remains offline, or a captured missing registration/reconnect invalidation. Falsified as the sole cause if the relevant API row remains offline. Cold registration broadcasts already exist, so do not rank this above the user's token evidence without corroboration.
3. **Read/write deployment divergence or abnormal DB flush timing.** Requires multiple instances/config differences, stale raw DB timestamps, batch-flush errors and/or failed Redis/relay. Shared healthy DB+Redis and current timing do not imply this. Capture per-instance health/build identity and sanitized DB/Redis destination identity; do not change production data to test it.
4. **Brief sweeper/heartbeat race or a different/offline bound runtime.** Race heals on the next healthy heartbeat; a mismatched agent runtime ID does not. Compare the agent binding to the two acknowledged IDs before interpreting both displays as the same resource. Neither establishes a sustained heartbeat failure in this log.

### Minimal regression/fix directions for planning

- First reproduce the actual script-dependent credential flow. Test expired PAT at reopen, a future-expiry revoked/version-stale PAT, network/DB 503 versus auth 401, correct-profile re-login/replacement, and successful recovery of all affected workspace runtimes. Preserve auth and expiry checks; extending TTLs or allowing invalid tokens is not the fix.
- If notification gap is selected as a separately evidenced fix, publish one existing workspace runtime invalidation after a committed offline-to-online transition. Cover both HTTP and WS callers and synchronous recovery from a stale online snapshot; do not broadcast on every normal heartbeat. Let the authoritative update result distinguish the actual transition to avoid duplicate events from concurrent beats.
- For that regression, subscribe to the event bus, start from an offline row, heartbeat through the real handler, assert DB online before ACK and exactly one appropriate runtime event; a second normal heartbeat should emit no transition. Include production batch scheduler wiring, since default handler tests use the synchronous scheduler.
- Keep existing sweeper race and outage tests. Add healthy steady-state, Redis absent/error, and reconnect-order coverage only where current tests lack the selected behavior. Do not broaden this task into a heartbeat rewrite.

### Verification and existing tests

Executed using local `go version go1.27.0 darwin/arm64` (module requests Go 1.26.6):

- `go test -mod=readonly ./internal/daemonws -run '^TestHeartbeat(RoundTrip|HandlerCtxNotTimeBounded|RejectsUnauthorizedRuntime)$' -count=1 -v` — all three passed. These use a local httptest server/fake heartbeat handler; they verify ACK dispatch, callback lifetime and unauthorized-runtime rejection, **not** live DB status or incident reproduction.
- `go test -mod=readonly ./internal/auth -run '^(TestTTLForExpiry|TestParseAuthTokenTTL)$' -count=1 -v` — both passed, including all TTL parser subcases. These verify cache expiry clamping and configured JWT TTL parsing; they do not exercise production authentication or password-mode DB state.

Inspected, not executed because they use DB-backed fixtures:

- `heartbeat_test.go:120`, `:151`, `:183`, `:212`, `:245`, `:283`: no-Redis writes, Redis throttle, summary flush, offline-to-online sync write, Redis failure fallback and sweeper race recovery.
- `heartbeat_scheduler_test.go:71`, `:202`: offline synchronous fallback and recovery on the next beat after a batch race.
- `runtime_sweeper_race_test.go:20`, `:90`: final UPDATE protects concurrent committed heartbeat, genuinely stale rows go offline.
- `runtime_sweeper_filter_test.go:67`, `:87`, `:114`: noop, live Redis filtering and Redis error fallback.
- `personal_access_token_test.go:68`, `:101`, `:127`, `:140`, `:180`, `:199`, `:256`: renewal window/no-op, expired/revoked rejection, no-expiry and concurrent renewal.
- `password_ws_test.go:218`: token-version revocation and DB loss reject further WS heartbeat processing before consuming pending actions.

`internal/handler/handler_test.go:44` opens a default local DB and owns shared fixtures; on unavailable DB it exits zero before running tests. A bare green package command is not proof these handler tests ran. Run DB suites serially against an explicitly isolated test DB during implementation verification.

### Related specs and external references

- Root `CLAUDE.md`: query ownership, package boundaries, checked errors and DB fixture/test rules.
- `.trellis/spec/server/security-boundaries.md`: human-only PAT mint/renew routes; password/session failures versus dependency failures.
- `.trellis/spec/server/managed-installations.md`: fresh runtime authorization, account/binding scope, separate client and daemon liveness.
- `.trellis/spec/server/platform-observability.md`: source failures are unavailable, not mass offline; DB fixture suites run serially.
- `.trellis/spec/guides/cross-layer-thinking-guide.md`: trace source → storage → read model → display explicitly.
- No external documentation was required. `server/go.mod` declares pgx v5.9.2, go-redis v9.18.0, gorilla/websocket v1.5.3 and Go 1.26.6; conclusions above cite this project's actual implementation.

## Caveats / Not Found

- No onsite before/after API response, DB snapshot, auth-mode configuration, token metadata, exact script source or precisely matched incident timestamps were available to this lane. The script's operation sequence was supplied by the user via the main session and is incorporated above.
- The main session reported a five-second timeout from this environment to the private server's health URL. That only limits our access; it does not establish that the user's server was down.
- No production mutations, deployed changes, Windows actions, provider CLI executions, new dependencies or product-code edits were performed. Only this research file was written by this lane.
- The primary incident cause remains unconfirmed. A confirmed missing heartbeat-recovery notification is separate from proof that it caused this user's failure, especially after the token-script recovery evidence.
