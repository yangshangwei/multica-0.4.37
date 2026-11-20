# S5 r02 rehearsal: independent evidence review

Review date: 2026-11-14. Scope: the complete retained r02 evidence at `/Users/artisan/.multica/acceptance/pay241-s5-20261114-r02`. **Successful tool rehearsal, with the limits below retained. All 79 independent checks pass; no blocking sampling or identity finding remains.** Final database/process/gate facts have also been cross-checked. This is a tool rehearsal on pre-S2 source, not S5 or TC-24 acceptance; TC-24 remains `NOT_RUN` for the final S2–S4 candidate.

The reviewer used independent Python standard-library calculations, without importing either harness statistics module or calling any API/database. Exact Cartesian-pair counters check the full population; nearest-rank is `rank = (N × percentile + 99) // 100`. Missing or unexecuted planned pairs would enter the denominator as +∞. No successful subset was substituted for the registered denominator. Only this report and its JSON companion were written.

## Recomputed results

| Metric | Complete formal denominator | Missing / NOT_RUN | Independent p95 | Maximum | Budget |
| --- | --- | --- | --- | --- | --- |
| Propagation, pooled | 10,000 observer/event pairs | 0 / 0 | 702.579 ms | 794.577 ms | 2,000 ms |
| Propagation, slowest observer per event | 100 events | 0 / 0 | 740.635 ms | 794.577 ms | 2,000 ms |
| Business-result visibility | 100 distinct runs | 0 / 0 | 130.694 ms | 3,108.177 ms | 5,000 ms |
| Finalization visibility | 100 distinct runs | 0 / 0 | 724.774 ms | 1,793.891 ms | 5,000 ms |

Every independently computed quantile equals the retained summary value before rounding. Propagation contains exactly **10,500 raw rows**: 500 warmup rows plus the complete 100 × 100 formal grid. Writeback contains exactly **202 rows**: one warmup run with two metrics, plus 100 formal runs with two metrics. No duplicate pairs, omitted planned pairs, late reclassification or changed warmup assignment was found. All 100 per-observer p95 values are below two seconds. The separate 9,900-pair distribution excluding writers also matches the summary.

The prior r01 failure remains retained: 71 successful formal writeback runs, a missing run at index 71, and 28 unexecuted runs. r02 has a new run ID and database. Its complete population is reported; it does not replace r01's failed result or supply final-candidate acceptance.

## Identity and timing

Preparation records 100 distinct user IDs and usernames, 100 public registrations returning 201, and 99 distinct successful public invitations. Registrations use four fixed five-minute windows with populations **30 / 30 / 30 / 10**. The first-to-last registration interval is **661.701 seconds**; the design's approximate fifteen-minute estimate is not substituted for the observed duration.

The API log records 201 successful logins: one preparation-owner login and 100 fresh logins for each measurement phase. Each phase contains exactly one server-observed connection and one normal close frame for every prepared user, plus successful `/api/me` and issue GETs for all 100 distinct user IDs. Propagation independently retains 100 distinct socket IDs, each consistently bound to one observer. The frozen `openObserverSessions` also asserts distinct tokens, `/api/me` identity, authentication/subscription ACKs, current readable authority and the exact active participant set before measuring. Raw tokens and complete authentication replies were intentionally not persisted, so token uniqueness is an executed source-bound assertion, not a later recount of a token file.

Each formal propagation event is written by its corresponding actor. All 105 request IDs, event IDs and expected sequences are unique; all 100 observers agree on each event's write identity. Every sample has a matching `S+1` issue frame before its own read, an authoritative target status/version, coherent monotonic timestamps and the exact reported latency difference. All round boundaries show 100 open observers, stable cursors, empty unsatisfied-observer lists and no reconnects. Jitter remains bounded in `[0,500)` and is retained per round.

Writeback has 101 distinct run IDs, result request IDs, trigger request IDs and command IDs including warmup. Formal trigger k is actor k. Readers and secondary hint readers belong to the same prepared population and differ from the trigger and resource owner. Each run retains the 404 prechecks, content digest, exact comment ID and run attribution result. Both metrics preserve their distinct origins, read intervals and complete denominators. All 101 boundaries and completed records report 100 connected sockets, zero disconnected indices and no reconnects. The controlled finalization record requires settled state, host exit proof, one launch, success and the matching result; final database facts are checked separately from these harness assertions.

## Heartbeat and fixture lifetime

The raw heartbeat journal and API HTTP log agree on **18 actual successful owner heartbeats**, all status 200. The managed runtime ID and owner are constant. The first start is `2026-11-14T03:38:16.617Z`; the last completion is `03:42:32.390Z`, spanning **255.774 seconds**, longer than the unchanged 150-second freshness threshold.

End-to-next-start intervals range from **15,003.326 to 15,009.746 ms**. Requests take **9.696–135.716 ms**, without overlap or request timeout. All 101 pre-trigger owner reads show the exact runtime online; the maximum persisted heartbeat age is **30,653 ms**. These reads precede the measurement origins. The first heartbeat completes before daemon start; the last sample completes **258,090.790 ms** after the daemon-start request, inside the unchanged 300,000 ms fixture lifetime. The heartbeat is stopped and joined after daemon cleanup. No freshness, timeout, pool or sampling budget was increased.

## Source, configuration and logs

The manifest binds commit `fb9f4c07059445cea41ed19e956b3eb1861817cc` plus the four reviewed S5 files. All observed phase hashes, current frozen files, final-freeze record and source overlay agree. The checked performance harness is `75d25dc3f531fa80ca807d7f3da95a0bcc746082702bcf966f225f09b9d6471a`. The earlier review report's prose-finalization timing is explicitly recorded; the source bytes did not change after readiness or during preparation.

The independent source audit checked 28,817 tracked Git blobs. The only raw byte difference is the Windows uninstall batch file's declared CRLF checkout under the repository's existing `.gitattributes`; replacing only CRLF with LF exactly reproduces its Git blob. No measured product source differs. All five selected binaries match their provenance hashes and the retained r01 originals; the provider source, daemon and CLI also match manifest/configuration paths and hashes.

All three phase receipts record exactly one passed test, one worker, zero retries and exit 0; their actual log hashes match. The manifest/raw sample/round/run hash bindings all match. The private configuration hash matches the API process and startup receipt. Recorded `/health` and phase observations identify the same process, commit, deployment and isolated database. Password mode, disabled device auth, empty cloud URL, no Redis and one replica are retained. The API startup log confirms the default **25 maximum / 5 minimum** pool, with no pool override in the selected configuration.

## Findings and limits retained

- One actual `stop-intent` GET returned 503 at local `11:41:02.391`, for formal run index 67 (`415cf06a-0fe4-40a2-9584-f42722439a89`). Both of that run's samples converged. No later 200 for that exact GET path appears in the retained log; this is not described as a proven successful retry. The final database snapshot independently confirms its success, settlement and host exit proof.
- The API startup log records pprof's port-6060 bind conflict. The application health and measurement endpoints remained available. No profiling service success is claimed.
- The optional hint path is present for 58 formal runs and null for **42**. Its finite-only informational distribution does not establish complete hint delivery and is not used for either writeback gate.
- The API process's descriptor limit is null in the manifest/resources. The harness records its own soft limit as 1,048,575. The API limit remains an environment metadata gap; no inherited limit is invented.
- This is same-host loopback on Apple M4 arm64, 10 CPUs and 16 GiB RAM, with PostgreSQL 17.11 x86_64 and shared cluster `max_connections=100`. Recorded total cluster connections range 24–43, host one-minute load 3.559–15.904, and writeback event-loop delay reaches 95.683 ms. These costs stay inside the measured latencies. The result does not establish native-arm PostgreSQL, production capacity, cross-region delivery, 100 rendered pages or real-model performance.

## Final database and cleanup cross-check

The reviewer inspected the read-only auditor's SQL and independently compared the returned rows from `database-after-writeback-final.json` with the raw samples, rather than relying on its `checks` booleans. The snapshot contains exactly the 100 active prepared participants and all 101 measured run IDs. Every run has the matching issue, Agent and runtime, `host_process_supervision_v1`, `host_exit_observed`, settled success, authorized/sealed attempt sequence 1, and complete recording. Each has one authorized launch and one terminal observation. All 101 result comment IDs, Agent authors and content hashes match their raw samples; 101 durable comment events bind each comment to its exact run.

Two failed post-run audit attempts remain preserved. Attempt 1 queried a nonexistent issue column and failed with SQLSTATE 42703. Attempt 2 incorrectly expected the namespace proof `exited_proven`; its retained rows actually had the correct host proof. Attempt 3 corrected the auditor and added stronger launch/comment bindings, then exited 0. The independent comparison confirms that the earlier and final snapshots retain the same 101 successful host-run facts. No product or measurement evidence was changed to obtain that auditor result.

The phase artifacts show zero active WebSockets after every phase, daemon exit 0, zero fixture errors, and 101 `proof_acknowledged` ledger records. Gate identities are distinct: the **allocation original** was `(enabled=false, creation=true, host=false)` at epoch 2; setup enabled the rehearsal at epoch 3; the host fixture returned to that phase-entry state at epoch 5; the coordinator finally restored the allocation's original flags at epoch 6, confirmed by a separate read. The manifest's `gate_original` records the phase-entry state, not the migration default.

`api-stop.json` binds the exact API binary, process, healthy commit and unchanged private configuration before SIGTERM, then records both process and port-18996 listener absent. The database and evidence are retained. Its later runtime row is offline after heartbeats and the API stopped; that is not a measurement-time offline result. The subsequent read-only `final-resources.json` snapshot at `2026-11-14T04:09:47.537Z` confirms **zero connections to the allocated r02 database** and **zero processes using the owned binary paths** after API shutdown and audit-client closure. Its database/process scope is specific to this rehearsal; it does not claim the shared PostgreSQL cluster has no connections. The snapshot SHA-256 is `f96d40f5a773a2ecfbc1613b87dd4cd003c5ae3a130ad0f2b0bd5964e4fc2386`. This addendum resolves the previously missing cleanup-count evidence without rerunning measurement or tests.

## Verification evidence

The independent audit records **79 passing checks**, with calculations, source/raw evidence hashes and final cleanup bindings in [rehearsal-r02-review.json](./rehearsal-r02-review.json). Its first log parser expected an explicit “disconnected” message; the API instead logs normal close-1005 frames as read errors. That parser correction is retained in the audit history, with no source or sample edits.

Lint/type-check/31 node regressions belong to the prior exact-source harness review and remain hash-bound; they were not rerun for this read-only evidence review. The three live Playwright phase results were verified from their complete retained logs. No real models, funds, production deployment or real business-person sign-off are claimed.
