# S07 capacity follow-up — 2026-10-02

Latest result: [connection and contention fixes](connection-followup.md) close the recorded local paging, ACK-timeout and extra-native-disconnection gaps in a new full rerun. Earlier failures below remain historical evidence. Windows/distribution/reference/production gates remain open.

Latest continuation: [native recovery under fleet load](native-load-recovery.md) validates 12 tasks on a real three-runtime daemon, while retaining ~109.9-second reconnect starts, 834.2 ms paging P95 and other open timing/connection limits. It does not mark S07 released.

The local paging and loaded-browser targets passed after optimizing overview SQL and explicitly sizing PostgreSQL shared memory. S07 remains open: the ten-second heartbeat threshold still failed 1,000 times, and Windows/distribution/reference-environment/production gates remain unverified.

## Changes and controlled comparisons

`GetAdminOverviewExecutions` no longer groups every usage report by task before reducing to one row. It counts reported/unpriced tasks distinctly, sums reports directly, lets the finished CTE's consumers plan independently, and filters terminal rows before current-state joins. Same-snapshot full result rows matched across five SQL variants. Three alternating runs measured original median 1,022.773 ms versus final 608.411 ms (40.5% lower). Unknown cost, missing versus zero usage, report multiplicity, clocks, organization/workspace scope and window boundaries remain covered by regression tests.

The first optimized 660-second load retained seven overview 503s. Every failure matched PostgreSQL `could not resize shared memory segment ... No space left on device`; Docker's default `/dev/shm` was 64 MiB, while one optimized parallel hash used about 20 MiB. That failed run is retained under `followup-default-shm/`. Only the owned PostgreSQL container was recreated with its existing volume and explicit 256 MiB shared memory; the 4 CPU/4 GiB total resource quotas stayed unchanged. Both bundled Compose files now specify this limit. This is an environment change, not an identical-environment SQL-only comparison or a production capacity guarantee.

## Final measured window

2026-10-02T03:16:09.204Z, duration 660.244 seconds, uninterrupted and with no undrained HTTP requests. The original fixture was retained: 1,000 installations, 3,000 runtimes, 1,000,300 executions, 750,000 usage reports and ten observers. The production Web browser ran inside this load window. API/Web source snapshot: `000b1d04ee18066df79154d6032adc1fd3010b4b93472c920b69fffd3a3f0b9b`; base checkout `1c56f6cd6`, copied from implementation HEAD `03bba8aed` plus the corrective working tree. Later fallback test and Compose/documentation edits do not change the measured API/Web code; the actual shared-memory setting is recorded separately.

| Measurement | Samples | P95 | Errors |
| --- | ---: | ---: | ---: |
| Observer HTTP, all routes | 2,250 | 1,583.5 ms | 0 |
| 31-day overview API | 405 | 2,059.8 ms | 0 |
| Worst pagination group, source page 2 | 16 | 482.0 ms | 0 |
| Loaded browser overview, 24-hour UI default | 10 | 1,901.3 ms | 0 |
| Loaded browser tasks | 10 | 808.7 ms | 0 |
| Loaded browser installations | 10 | 644.4 ms | 0 |
| Loaded login-to-overview | 1 | 397.6 ms | 0 |

All pagination groups passed P95 ≤800 ms. All 31 browser observations passed; each full operation occurred within the load window, verified from timestamps. The browser sample groups are small: ten-sample P95 is the maximum observation. The baseline loaded overview P95 was 2,808.4 ms; final P95 is 26.7% lower, with the explicit environment and browser-load differences above. API overview still exceeds two seconds; the specified two-second target applies to Web rendering, not every 31-day API query.

## Reconnect evidence and limits

The harness now records every ACK receipt before timer matching, including installation, socket generation, wall/monotonic receive time. Expired sends retain queue entries, so a late ACK cannot clear a newer send's deadline. After a timeout, FIFO assignment is inferred and subsequent matched outcomes are labeled ambiguous rather than successful latency samples. Eleven helper/report tests protect this accounting and loaded-browser window classification.

All 1,000 sockets recovered within five seconds (connection P95 3,646.1 ms). Successful raw ACKs on newly opened generations covered 1,000 runtimes by ten seconds, 2,000 by fifteen seconds, and all 3,000 by **15,779 ms**. The retained ten-second timer metric still contains **1,000 timeouts**. Another 28 pending sends were interrupted by the deliberate reconnect and 17 by teardown. There were 22,965 correlation-ambiguous outcomes; these are not server refusals or established packet loss. Only 2,000 runtime identities have an unambiguous post-reconnect timer-success observation. Do not replace raw receipt and timer-success curves with a single “recovered” claim.

Native ACK freshness is twice the heartbeat interval, normally 30 seconds. HTTP resumes on the next 15-second per-runtime tick after expiry (roughly 30–45 seconds after the last ACK, plus scheduling/request latency), or the next tick after disconnect clears freshness. ACKs carry no request nonce/server timestamp. Therefore raw receipts do not establish per-send RTT, native fallback under fleet load, or a guaranteed 30-second recovery bound.

A new native `httptest` regression verifies real HTTP fallback for absent/fresh/expired/scoped-disconnect ACK states with the correct workspace credential and no Run/provider/model/claim loop. Six focused daemon tests passed with race detection; this proves policy and credential scope, not installed-client timing.

## Verification and remaining work

- Five overview tests passed before and after SQL optimization; the final five also passed with `-race`. Handler vet passed; SQL regenerated with `make sqlc`.
- Six native heartbeat/transport/isolation tests passed with `-race`; daemon vet passed.
- Eleven ACK tracking/report tests and script syntax checks passed. Both Compose configurations resolve PostgreSQL shared memory to 268,435,456 bytes.
- Production Web build/preflight and final 660-second load passed the paging/rendering gates. The earlier full S07 suite remains recorded in `verification.md`; it was not rerun for this bounded SQL/test/configuration follow-up.
- Independent read-only review found no actionable SQL, scope, Compose or deployment-documentation issue.

Still open: ten-second ACK threshold and operational margin, native fallback during fleet load, Windows runtime/ACL and signed/old-binary/reinstall acceptance, dedicated reference hardware, genuinely storage-cold behavior, prolonged failures and production maintenance/deployment/recovery. No push, merge or production deployment is part of this follow-up.

Nonsecret results and artifact hashes: [summary](evidence/capacity-followup-summary.json), [allocation failures](evidence/capacity-followup-errors.json). Raw runs, SQL comparisons, private fixture and test logs remain under `.omx/reports/platform-admin/s07-capacity/`; the run is `s07-20261001212929-d12b32`. All owned listeners/containers are stopped after verification; data/volumes are retained. See `evidence/capacity-followup-cleanup.json` for ownership-checked cleanup.
