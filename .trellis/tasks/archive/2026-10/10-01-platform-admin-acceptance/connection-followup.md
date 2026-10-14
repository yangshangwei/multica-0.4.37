# Connection and contention fixes — 2026-10-02

The local thousand-client rerun met the previously open paging, heartbeat and native task-start checks: no extra native disconnections, no ten-second ACK timeouts, paging P95 ≤289.3 ms, and reconnect task starts in 39.355–39.548 seconds. Production, Windows/distribution and dedicated reference-environment acceptance remain open.

## Diagnosis and changes

The previous run acquired database connections about 13.9 million times across its recorded metrics interval. The pool was fully occupied in 50/56 snapshots; 89.44% of acquires found it empty and mean acquisition wait was 33.00 ms. Idle EXPLAIN measured the installation list at about 1.23 ms and its evidence query at about 18.5 ms. This supported investigating shared authorization traffic rather than rewriting the paging query.

`AuthorizeDaemonConnection` repeated the same managed namespace/account/binding checks for each runtime, then reread each runtime. A three-runtime token-plus-connection check used 46 database queries. It now validates the managed binding once **within each invocation**, then reads every current runtime and checks owner/workspace/daemon against that binding. Observed query count is 19 (handler-only 38 →11). There is no cross-frame cache. Credential, membership, account, organization, binding and runtime changes are rechecked; legacy and transactional claim paths are unchanged.

A separate native bug was reproduced before fixing: connection cleanup detached global `d.wsRPC` instead of the passed workspace RPC. A sent pending workspace claim remained attached to a closed transport and an unrelated sender was detached. Cleanup now calls `rpc.attach(nil)`. Sent requests still have uncertain outcomes; this does not authorize unsafe immediate duplicate claims.

Both sides now emit bounded failure classifications and phase/scope correlation. Native records include RPC generation; server records distinguish read/write/ping from authorization failures. Logs omit raw endpoint errors, credentials and peer close text. Expected cancellation during teardown is not promoted to an authorization warning.

A diagnostic baseline retained the old authorization query path and new logging. It reproduced five **server read timeouts** in 216.101 seconds before intentional interruption. Authorization cancellation events followed the reader closing the connection, rather than establishing authorization rejection as the first cause. Together with the pool/query evidence and the successful reduced-query rerun, this identifies contention and keepalive/read processing as the reproduced failure mechanism. The original two historical closes lacked error logs, so their individual cause cannot be proven retrospectively.

Correction to earlier explanations: the daemon renews its read deadline on every successfully read application message **and** on Ping/Pong. The server renews on Pong. A proxy must relay control frames correctly, particularly when application ACKs are suppressed, but missing control forwarding alone cannot explain a close while application messages continue. This run used the previously corrected end-to-end control-frame and HTTP-cancellation proxy.

## Final workload and results

Window: `2026-10-02T06:37:51.524Z`–`06:48:51.730Z`, 660.207 seconds, uninterrupted with zero undrained HTTP requests. Background fixtures: 1,000 installations, 3,000 runtimes, one million historical executions and ten observers. The added real daemon had three runtimes in one workspace. Eight earlier enrollment records remained outside the active load fixture; preflight verified the exact fixture IDs instead of hiding them in a global count.

The native probe ran through approximately 602 seconds of the measured fleet window, then stopped as scheduled; it did not remain connected for all 660 seconds. Three baseline tasks preceded the measured window and nine tasks ran during it. All 12 correlate with actual synthetic provider processes and non-cancelled exits. No personal account, external model call, signed distribution or Windows client was exercised.

Same local M4/shared Docker topology and limits: API GOMAXPROCS 4, PostgreSQL 4 CPU/4 GiB, 256 MiB shared memory. API snapshot `c8f0406bfc4ed6f04e253fa97725f6b80f01ff37b495dfb871c667b2c2459e69`; the separately hashed CLI was rebuilt with the scoped cleanup and diagnostic changes. These observations are not dedicated production-hardware guarantees.

| Observation | Previous recorded run | Final rerun |
| --- | ---: | ---: |
| Three-runtime token + connection DB queries | 46 | 19 |
| Mean pool acquisition wait | 33.00 ms | 3.14 ms |
| Acquires finding an empty pool | 89.44% | 9.59% |
| Fully occupied pool snapshots | 50/56 | 4/56 |
| Installation page P95, 54 samples | 834.2 ms | 62.8 ms |
| Worst paging group P95 | 834.2 ms | 289.3 ms, source page 2, 16 samples |
| Native reconnect issue-to-running | 109.875–109.879 s | 39.355–39.548 s |
| Background ten-second ACK timeouts | 9,300 | 0 |
| Extra native connection closures | 2 | 0 |

Final observer HTTP: 2,265 requests, zero errors/429/503. All paging groups met 800 ms. There were 133,512 successful, unambiguously matched runtime ACKs in the measured phases (141,165 including ramp); the only failed heartbeat samples were nine intentional reconnect interruptions and eleven measurement-end interruptions. Those remain in raw data.

Loaded browser against the local production build: all 31 observations passed. P95 was 887.6 ms for overview (10 samples), 198.4 ms for installations (10), 174.9 ms for tasks (10), and 901.9 ms for the single login-to-overview observation. Small groups and shared-host conditions limit extrapolation.

The corrected proxy observed only the deliberate native disconnect and scheduled final closure. Per-runtime HTTP heartbeat fallback started 30.125, 35.077 and 36.355 seconds after the last proxy-observed ACK during the controlled ACK-loss window; no socket closed within that interval. This proves heartbeat fallback and task recovery, not the transport used to claim each task or nonce-correlated ACK RTT. CLI generations are RPC attachment generations (1 →3), while proxy connection ordinals are 1 →2; correlate scope and timing, not identical numbers.

## Verification and remaining scope

- Full `internal/daemon` and `internal/daemonws` race suites passed; both packages passed `go vet`.
- Selected handler race suite passed 11 top-level tests, including the 14 persisted-scope mutation cases; handler vet passed.
- Red regressions demonstrated incorrect scoped RPC cleanup before the fix. Query instrumentation verified 46 →19 calls and fail-closed revocation/scope behavior.
- Production Web build, real CLI execution and independent native phase/provider/fallback verification passed. Read-only peer review found no authorization or diagnostic behavior regression.
- No database migration, dependency, API response shape, authorization interval, duplicate-claim recovery window or admission policy changed.

The current local paging/ACK/native recovery gaps are closed for this measured fixture. Windows runtime/ACL, signed/old-client install and upgrade/reinstall, cold/long-failure/reference-hardware behavior, and production rollout remain open. Reconnect start latency is improved rather than instantaneous; the protected uncertain-response recovery path remains available and separately regression-tested.

Evidence: [summary](evidence/connection-followup-summary.json), [cleanup](evidence/connection-followup-cleanup.json). Raw logs and build/verdict files are under `.omx/reports/platform-admin/connection-followup/`; the interrupted diagnostic baseline is retained under `s07-capacity/runs/s07-20261001212929-d12b32/connection-diagnostic-baseline/`. All owned API/Web/container and native/provider processes are stopped; private data and failed baselines are retained. No push, merge or production deployment occurred.
