# S07 local capacity verification

The 660.85-second local baseline completed with 1,000 installations, 3,000 runtimes, one million historical executions and ten observers. Every measured pagination group met P95 ≤800 ms, and the separate browser phase met ≤2 seconds. Concentrated reconnect exceeded the harness’s ten-second heartbeat-ACK threshold 1,000 times; this limitation remains visible and is not an unconditional fleet-capacity pass.

## Source, fixture and environment

- Base commit: `1c56f6cd6c16f11327aee2f3f000e6b19e401d0b` plus the frozen corrected API/Web working snapshot.
- Snapshot SHA-256: `5935c327fc3c84b096ffe565f93ffc111228849ed585fb37c4e25c73efbc4252`; 68 changed/new files were copied and individually verified. Later desktop-only private-config and verification-script changes are outside this API/Web measurement.
- Measured window: 2026-10-02T00:31:26.981Z through 2026-10-02T00:42:27.830Z; 660.85 seconds, uninterrupted, zero undrained HTTP requests. Ramp and first-use probes precede the window.
- Fixture: 1,000 managed installations, 3,000 bound runtimes, 100 workspaces, 1,000,000 historical tasks over 90 days, and 300 current/deferred fixtures. Historical outcomes include retries, cancellations and failures. There are 750,000 usage rows, leaving 250,000 historical tasks without usage; 150,000 usage rows have unknown cost. Unknown usage/cost was not replaced with zero.
- Apple M4 host: ten CPU cores, 16 GiB RAM. Native API/Web/load generator share this machine; PostgreSQL and Redis run in a Docker VM. API GOMAXPROCS=4, pool maximum 25/minimum 5. PostgreSQL quota 4 CPUs, memory 4 GiB, shared_buffers 512 MiB; Redis memory 128 MiB. This differs from the proposed larger reference cloud topology.
- Runtime heartbeats repeat every 15 seconds; signed installation reports use 48–72 second jitter. Ten observers each poll overview/list every 15 seconds and detail every 5 seconds, plus occasional second pages. Actual observer throughput was 3.39HTTP requests/second.
- Observer password login, Ed25519 reports and bound MDT WebSockets pass through normal middleware. Fixtures were provisioned directly; no agent executable, real user account, provider session, task claim or model request ran.

## Measured latency and errors

All values below are milliseconds, computed directly from retained request samples using nearest-rank percentiles. Timeouts remain in the samples. Small second-page groups have only 11–17samples, so their P95 is effectively their largest observation.

| Request group | Samples | P50 ms | P95 ms | P99 ms | Errors |
| --- | ---: | ---: | ---: | ---: | ---: |
| All observer HTTP | 2242 | 18.1 | 2076.0 | 2713.7 | 0 |
| installations | 54 | 45.8 | 287.1 | 363.7 | 0 |
| tasks_current | 54 | 20.1 | 265.8 | 311.5 | 0 |
| tasks_current_page2 | 11 | 21.4 | 293.0 | 293.0 | 0 |
| tasks_default | 55 | 18.2 | 302.1 | 418.5 | 0 |
| tasks_default_page2 | 17 | 21.9 | 767.8 | 767.8 | 0 |
| tasks_empty | 54 | 11.7 | 224.3 | 358.6 | 0 |
| tasks_failed | 56 | 18.9 | 245.7 | 764.0 | 0 |
| tasks_failed_page2 | 17 | 20.7 | 513.7 | 513.7 | 0 |
| tasks_finished | 56 | 19.5 | 206.5 | 311.8 | 0 |
| tasks_finished_page2 | 11 | 22.0 | 268.3 | 268.3 | 0 |
| tasks_source | 55 | 34.5 | 297.3 | 311.5 | 0 |
| tasks_source_page2 | 16 | 37.9 | 295.2 | 295.2 | 0 |
| tasks_workspace | 56 | 14.5 | 201.2 | 287.0 | 0 |
| tasks_workspace_page2 | 11 | 15.1 | 279.2 | 279.2 | 0 |
| overview | 394 | 1839.4 | 2808.4 | 3140.2 | 0 |
| task_detail | 655 | 11.4 | 200.7 | 474.3 | 0 |
| installation_detail | 654 | 14.4 | 315.0 | 696.3 | 0 |
| installation_heartbeat | 11001 | 17.6 | 259.3 | 497.8 | 0 |
| runtime_heartbeat | 132927 | 85.8 | 2810.6 | 9002.2 | 1032 |

Observer and signed-heartbeat HTTP had zero errors, 429s or 503s. The overview’s loaded P95 was 2,808.4 ms; it must not be hidden by the faster pagination groups. All measured pagination P95 values were ≤800 ms; the narrowest margin was default second-page 767.8 ms across 17 samples.

Runtime heartbeat error samples total 1,032 (0.776%): **1,000 ACK timers expired at 10 seconds**, **7 pending heartbeats were deliberately interrupted by the scheduled reconnect**, and **25 pending heartbeats were flushed when the measurement ended**. The 25 are measurement teardown, not additional unexplained post-reconnect failures. All remain in raw data and aggregate latency/error calculations.

## Concentrated reconnect

All 1,000 first reconnect attempts were started together. Retry policy was full-jitter exponential backoff capped at 60 seconds; all connections authenticated without a failed upgrade. Connection recovery P50/P95/P99 was 3,124.6/3,164.1/3,168.8 ms; the final connection opened at 3,173.1 ms.

| Elapsed after reconnect | Authenticated sockets | Runtime identities with a measured successful ACK |
| --- | ---: | ---: |
| 1s | 0 / 1,000 | 0 / 3,000 |
| 2s | 0 / 1,000 | 0 / 3,000 |
| 5s | 1000 / 1,000 | 0 / 3,000 |
| 10s | 1000 / 1,000 | 1000 / 3,000 |
| 15s | 1000 / 1,000 | 2000 / 3,000 |
| 20s | 1000 / 1,000 | 2252 / 3,000 |
| 30s | 1000 / 1,000 | 3000 / 3,000 |
| 60s | 1000 / 1,000 | 3000 / 3,000 |
| 120s | 1000 / 1,000 | 3000 / 3,000 |

All 3,000 runtime identities had a measured successful ACK by 28.234 seconds. This is distinct from transport reconnection.996 of the 1,000 ten-second timeouts affected the third runtime in a connection’s initial three-frame burst; four affected the second.

The server processes heartbeat frames serially per socket and reauthorizes inbound and outbound work. The real daemon also sends its runtimes together, but uses a 30-second freshness/fallback budget. This harness does not perform native HTTP fallback, drops ACKs received with no pending timer, and the wire protocol correlates ACKs by runtime ID rather than request ID. A very late ACK can therefore be attributed to a later periodic send. The baseline proves ten-second threshold exceedances; it does **not** prove frame loss or a 30-second native-liveness failure. A subsequent run should record every receive timestamp and socket generation before timer matching while retaining the same ten-second threshold results.

## Production Web browser phase

The browser ran after the baseline with the same seeded database, outside the 1,000-connection/ten-observer load. These are full navigation-to-data/heading/first-row checks, not just navigation response times. All 31samples passed with no page errors.

| Browser flow | Samples | P50 ms | P95 ms | P99 ms | Errors |
| --- | ---: | ---: | ---: | ---: | ---: |
| login_to_overview | 1 | 896.9 | 896.9 | 896.9 | 0 |
| overview | 10 | 580.5 | 638.8 | 638.8 | 0 |
| tasks | 10 | 102.0 | 109.3 | 109.3 | 0 |
| installations | 10 | 123.2 | 150.8 | 150.8 | 0 |

The browser overview uses the UI’s default 24-hour range; load API requests omit the range and use the server’s 31-day default. Browser P95 ≤2 seconds is established only for this separate phase. Loaded browser rendering was not measured. The first login-to-overview sample was 896.9 ms. First-use is not storage-cold: seeding, ANALYZE, preflight and normal server work warmed PostgreSQL and OS caches; no caches were dropped.

## Resource observations

- API pool total and acquired connections both reached 25, as measured by exported pool gauges. PostgreSQL activity snapshots peaked at 30 entries, 12 active and zero lock waiters; activity includes parallel workers and is not the API pool size.
- Across 56 Docker resource samples, PostgreSQL CPU averaged 256.3% and peaked 437.6% (coarse interval accounting may cross the 4 CPU quota briefly); memory peaked 53.09% of 4 GiB. Redis averaged 4.35% CPU, peaked 7.77%, and used at most 16.57% of 128 MiB.
- Thirty-four host snapshots recorded background activity. The Docker VM, which also hosted unrelated Plane containers, averaged 405.4% CPU and peaked 546.3%. WindowServer averaged 20.3%/peaked 29.2%; Warp averaged 11.2%/peaked 31.1%. Percentages use 100% per logical CPU. Background services were neither stopped nor reconfigured.
- The root full verification, native builds/tests and password E2E finished before the baseline. No repository build/test overlapped the window. These local observations are not dedicated-host or production-hardware capacity guarantees.

## Separate authorization and database recovery probes

- Five synthetic observers had their role removed and restored through the normal reauthenticated, audited role API. Twenty-five concurrent requests from the existing session version returned 403`admin_forbidden`; denial P50/P95/P99 was 9.3/10.1/10.2 ms. Fresh password sessions without the role also returned 403.
- Restoring each role advanced its authorization version: fresh sessions regained 200 access, while the prior-version sessions returned 401. All five roles were restored; ten operations and ten linked audit events remain in the private database.
- Early harness attempts are retained: concurrent password hashing returned 503 before any role mutation; a later harness assertion incorrectly expected 401 for role removal although the documented contract is 403 without an elevation/version bump. The final probe used serial login prerequisites and the actual role contract; no limiter, authorization or audit policy was changed.
- Only the run-owned PostgreSQL container was paused for 8.057 seconds, without restarting or changing its port. Five admin requests reached their 8-second client deadline and none returned stale authorization success. After unpause, the first admin request returned 200 in 5.956 ms (5.974 ms measured recovery). This proves bounded client-observed unavailability and recovery; it does not establish server-side 503 timing, prolonged database failure, source-unknown UI behavior, native reconnect or production failover.
- Local backup/restore evidence is recorded separately in `backup-restore.md`; it preserves non-default credential/binding/admission state and pending operation/audit data.

## Read-only overview plan comparison

After the workload and recovery probes, original SQL was compared with a single additional `current_states` predicate: `t.status NOT IN ('completed','failed','cancelled')`. Both ran against the same 31-day repeatable-read snapshot and returned identical full result rows. Three alternating EXPLAIN ANALYZE runs measured median 1,032.824 ms (original) versus 943.689 ms (filtered), an 8.6% improvement. No product SQL was changed.

The filtered branch still parallel-sequentially scans all 1,000,300task rows; it removes 1 million before joining. The larger finished/usage work remains:342,436 finished rows, an external sort using 23,768 KiB disk, 16,255 temporary blocks written and 16,547 read. These costs were unchanged by the predicate. A WHERE-only change is a modest improvement, not evidence that the loaded overview or reconnect issue is solved. Detailed plans are in `overview-explain.json`.

## Evidence and remaining limits

Nonsecret structured summaries are checked in under `evidence/capacity-*.json`; `evidence/backup-restore-summary.json` records the separate restoration result. Credentials, private fixtures, environment secrets and the archive are excluded.

All raw artifacts are under `.omx/reports/platform-admin/s07-capacity/runs/s07-20261001212929-d12b32`: `measurement-complete.json`, `samples-1790901051342.jsonl`, `summary.json`, `database-statements.json`, `source-snapshot.json`, `browser-samples-1790901858130.json`, `host-background-1790901095372.jsonl`, `recovery-probes.json`, retained failed-probe attempts, and `overview-explain.json`. Scripts are under `.omx/reports/platform-admin/s07-capacity/`.

Unmeasured: storage-cold behavior, 30-observer/5,000-runtime stress, enrollment throughput, long database outage/failover, simultaneous browser rendering under the fleet load, native HTTP fallback during this reconnect, Windows or signed/old-binary compatibility, and production rollout. The ten-second reconnect threshold finding and loaded overview latency remain follow-up items. Local paging/browser results do not close those limits.

Owned cleanup completed at 2026-10-02T01:14:01.847Z. Both API/Web listeners and all three manifest-owned containers were verified stopped; labeled volumes, rehearsal databases, private fixtures, archive and detached checkout remain. No unrelated service was stopped. Evidence: `evidence/capacity-cleanup.json`.
