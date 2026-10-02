# Native daemon recovery under fleet load — 2026-10-02

A source-built macOS daemon with three runtimes completed 12 synthetic tasks: three baseline tasks before the measured workload and nine during the retained thousand-installation load. The result is **recovery evidence, not full S07 acceptance**: reconnect task starts took about 109.9 seconds, exceeding the original 90-second harness cutoff; the same run's worst pagination P95 was 834.2 ms, exceeding 800 ms. Background ten-second ACK timeouts and two non-injected native connection closures remain recorded.

## Scope and provenance

- API snapshot: `1a472f8a9d23a7a8caa05a5fc33abbc0e18f8040fef7e8ee4346486cf749d993`, copied from implementation HEAD `af3d3f45f` into the retained base checkout `1c56f6cd6`. No production code changed during this continuation.
- Real CLI SHA-256: `9a5acf5f0238bdcce09ee8af2f622dd29acbda148b6f9537587ed62d9c0253c2`; compiler `go1.27.0 darwin/arm64`. This is the local compiler, not the repository's Go 1.26 CI environment.
- Background: 1,000 protocol-fixture installations, 3,000 runtimes, one million historical executions, ten observers. Added native sample: one installation, one workspace, one WebSocket namespace and three Claude-protocol runtimes (builtin plus two custom profiles).
- Native credentials/enrollment/binding, scheduler, RPC/HTTP traffic and subprocess execution were real. Each provider command was an isolated deterministic fixture; all other provider paths were disabled. No personal account or model request ran. This round did not exercise Electron, signed distributions or Windows.
- Same local M4/shared Docker topology, API GOMAXPROCS 4, PostgreSQL 4 CPU/4 GiB and explicit 256 MiB `/dev/shm`. These are not dedicated reference-host capacity guarantees.
- Final window: `2026-10-02T05:35:43.277Z`–`05:46:43.513Z`, 660.238 seconds, uninterrupted with no undrained HTTP requests. Faults were scheduled at 60/300/480/600 seconds for reconnect/ACK drop/restore/finish. This differs from earlier runs' 300/360/450/600-second schedule.

## Corrected measurement method

Earlier message-terminating proxies did not forward Ping/Pong. Node automatically answered upstream pings while the real daemon's control-frame-based read deadline expired after 60 seconds. They also failed to propagate downstream HTTP cancellation. Those runs are retained, but their results cannot establish pure ACK-expiry recovery; the first single-runtime driver's raw “passed” flag is not accepted for that claim.

The versioned proxy forwards control frames in both directions, disables automatic intermediary Pong responses, and cancels upstream HTTP when the native caller aborts. Two regressions failed against the old proxy and passed after correction. An 80-second native preparation gate required one stable generation and ACKs for all three runtimes after 60 seconds. The fixed proxy uses already-installed ws 8.20.1 because the root's ws 7.5.10 does not support the needed autoPong option; no project dependency was added.

The first corrected-proxy run still ended at the driver's 90-second task-start cutoff. Source inspection found a 90-second stale-dispatch recovery window, an approximately 95-second Redis recovery hint and subsequent polling. The final driver observes for up to 180 seconds while explicitly preserving each original 90-second miss. It records issue identity immediately and task-state transitions, captures failure snapshots before stopping the daemon, and retains a failed workspace for diagnosis instead of deleting its evidence.

Independent offline verification checks the complete phase/runtime matrix, the source fault signals, per-runtime request-start timing, socket generations, real provider execution and cleanup. This caught assertion gaps in the original drivers and a verifier attribution error that selected a later socket instead of the first reconnection. The final checker uses reconnect generation 2 and the already-open generation 3 at restoration, preserving subsequent connection events.

## Observed native behavior

| Phase | Native tasks | Result |
| --- | ---: | --- |
| Baseline before fleet window | 3 | Completed; three runtimes shared one connection generation |
| Synchronized reconnect | 3 | Completed; issue-to-observed-running 109.875–109.879 s, all three original 90 s cutoffs exceeded |
| ACK and task-notification loss | 3 | Completed; all three HTTP heartbeat fallbacks observed before tasks were submitted; starts about 15.77 s after submission |
| Transport restored | 3 | Completed; starts about 8.28 s after submission |

All 12 task IDs correlate with real fixture-provider invocation, started and non-cancelled exit events, with the native daemon as parent. All observed provider PIDs and daemon PID `94472` were verified absent afterward; the synthetic workspace was deleted.

During ACK loss, the three HTTP heartbeat requests began **31.681, 37.982 and 44.946 seconds** after the last proxy-observed ACK for their runtime. No WebSocket closed within that fault interval. These timings use HTTP request start, excluding requests already in flight before the fault. ACK and task-available notifications were suppressed while control frames and other RPC traffic continued.

These are HTTP **heartbeat** fallback observations. Task claims could still use WS RPC; do not describe task completion as proof of HTTP task claiming. ACK arrival at the proxy does not establish daemon handler processing time or nonce-correlated RTT. Issue-to-running time includes API polling observation delay.

Two additional native connection closures occurred outside the injected fault interval:

- `05:38:42.464Z` → reconnected `05:38:48.427Z` (5.963 s).
- `05:45:17.563Z` → reconnected `05:45:24.298Z` (6.735 s).

The existing managed connection callback does not log the underlying error, so their causes remain unknown. A nearby uncertain claim and HTTP deadline are observations, not proof of the disconnection cause. Do not claim uninterrupted connectivity across the entire run.

## Recovery contract and regression

New `server/internal/handler/managed_claim_recovery_test.go` uses real managed authentication, the HTTP batch-claim handler and an isolated PostgreSQL fixture. It verifies immediate retries do not duplicate an unacknowledged dispatch, neither a prematurely expired lease nor an old dispatch alone bypasses the two eligibility gates, and reclaim returns the same task with a new claim generation and unchanged execution binding/epoch/admission. A started task cannot be reclaimed.

The test ages fixture timestamps and uses a nil Redis hint cache; it does not simulate real elapsed scheduling. The native API snapshots show queued → dispatched → running for the same task IDs but omit `claim_generation`. Generation advancement is proven by the database regression, not inferred from native API snapshots.

No production recovery window, authorization, admission or audit rule was changed. Detaching cancelled-request cleanup and requeueing immediately would require a separate admission review: previously admitted dispatched work can become queued work subject to a later installation stop.

## Fleet measurements and open gates

- Observer HTTP: 2,163 samples, zero errors/429/503; aggregate P95 2,599.4 ms.
- Pagination: zero errors, but installations P95 **834.2 ms** across 54 samples failed the 800 ms target. Prior passing capacity runs remain historical evidence, not a substitute for this result.
- Background heartbeat outcomes: **9,300 ten-second timeouts**, 2,352 socket-closed pending sends, 290 deliberate-reconnect interruptions, 743 teardown interruptions and 69,036 correlation-ambiguous outcomes. Ambiguous matches are not proven server refusals or packet loss; none are hidden from the retained samples.
- The final round did not repeat browser rendering. It does not supersede or expand the earlier loaded-browser evidence.

Open work: characterize/reduce uncertain-claim recovery latency without breaking duplicate/admission fences; diagnose remaining connection closures; repeat capacity measurements on controlled reference resources and resolve the paging/ACK timing limits. Windows runtime/ACL, signed/old-binary/reinstall lifecycle, storage-cold/prolonged-failure tests and actual production rollout remain unverified.

## Verification and evidence

- Managed recovery and stopped-admission regression: two tests passed with race detection on the identity-verified isolated Go database; handler `go vet` passed.
- Proxy, observation, signal/tracker/report and independent-verifier suites: **36 tests passed**.
- Final independent evidence verifier passed the recovery matrix while explicitly reporting `original_90s_start_deadline_passed=false`.
- All owned API/Web listeners and capacity containers stopped; private data/volumes and raw failed runs retained. No push, merge or production deployment occurred.

Checked evidence: [summary](evidence/native-recovery-summary.json), [cleanup](evidence/native-recovery-cleanup.json). Raw drivers/reports are under `.omx/reports/platform-admin/native-load/`; independent checks are under `native-verification/`; fleet data remain in `s07-capacity/runs/s07-20261001212929-d12b32/`. That directory separately preserves the single-runtime, broken-proxy three-runtime and corrected-proxy 90-second-cutoff attempts. The first preparation launch's overwritten raw report is explicitly represented only by its captured session observation, not claimed as an original retained artifact.
