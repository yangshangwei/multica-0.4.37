# Research: Actual cli02/cli03 database waits and API delays

- Query: Identify the actual slow request/SQL/wait relationships in the retained cli02 and cli03 runs, including captured blockers and whether a specific safe root fix follows.
- Scope: internal recorded evidence and source inspection only; no new run, SQL connection, Docker action, account access or source edit.
- Date: 2026-11-25

## Findings

### Concrete result

**The dominant captured state during slow requests is PostgreSQL waiting for its API client between statements (`ClientRead`), inside long transactions. Neither run captured a lock wait, a blocker PID, or a multi-second active SQL statement.** Resources remains the longest aggregate API path after parallelizing the five snapshot reads. The longest individual cli03 call is now a comment write, with the same changing-query/ClientRead pattern.

This materially narrows the previous report: an issue-row/namespace lock bottleneck is **not observed** in these actual runs. The captured `Lock…` query names frequently describe a statement already completed while PostgreSQL waits for the next command. Do not treat their transaction age as time spent acquiring that lock.

The evidence supports costly sequential database exchanges and application-held transaction lifetimes. It does **not** distinguish client computation/scheduling, transport delay, or short repeated database work missed between samples. Consequently **no specific causal production fix is yet established**. Pool/deadline increases, removing fences, permission caching and speculative indexes are unsupported. No fourth run or implementation is proposed by this sidecar.

### Direct measurements

Paths below are relative to `.omx/private/acceptance-fixes/native/`. All displayed request times are **Asia/Shanghai (UTC+8)**; `pg-waits.jsonl` timestamps are UTC and were converted for correlation. HTTP start is reconstructed as logged completion minus logged duration. Database-to-request attribution is temporal plus query-path correlation, not a request-ID/backend-ID join.

| Measured fact | cli02 | cli03 |
| --- | ---: | ---: |
| Successful observer samples | 252 | 242 |
| Additional observer failure | 0 | Final record :243, exit -15 during termination |
| Sample spacing, median / maximum | 0.918 / 2.400 s | 0.966 / 1.883 s |
| All sampled backend states | 3,013 | 2,897 |
| `idle in transaction`, `ClientRead` | 365 | 428 |
| `active`, no wait / `ClientRead` / IO | 11 / 7 / 0 | 19 / 15 / 3 |
| Captured `Lock` or `LWLock` wait | **0** | **0** |
| Nonempty `blockers` array | **0** | **0** |
| Maximum sampled active query age | **83.343 ms** | **202.988 ms** |
| Maximum query age excluding fully idle backends | 333.528 ms | 285.990 ms |
| Completed HTTP duration sum | 169.949310 s /146 calls | 208.010996 s /142 calls |
| Resources HTTP sum | 67.801294 s /23 calls | **82.690797 s /22 calls** |
| Issue-detail HTTP sum | 32.407050 s /32 calls | 32.473650 s /31 calls |
| Runtime-list HTTP duration | 5.826321 s | **16.301155 s** |
| Long comment HTTP duration | 15.752755 s | **20.239589 s** |
| Completed pool-acquisition duration upper bound | <10.209 s /4,450 acquisitions | <11.990 s /4,378 acquisitions |

HTTP sums overlap in cli03 and are not elapsed critical-path time. Sample counts are not percentages of execution time; completed SQL between samples is not measured. Pool bounds use every emitted count × `(integer avg_ms +1)` through the final pool sample, as in the earlier report, and include all API background work. Both logs report zero canceled acquisitions. These bounds cannot attribute a particular request, but do not support acquisition being the dominant aggregate cost.

Both Playwright results explicitly retain `timedOut`, 240000ms exceeded (actual test durations240010/240003ms). Both receipts retain exit1 and cleanup_status fail, with source unchanged, gate booleans restored and remaining DB clients0. Neither run passed.

### Request-to-transaction evidence

**1. cli03 final completed parallel snapshot: resources alone remains the long pole.**

| Request, `cli03/api.log` | Reconstructed start | Completion | Duration |
| --- | --- | --- | ---: |
| commands :201 | 20:17:19.033136 | 20:17:19.721 | 0.687864 s |
| participants :202 | 20:17:19.033451 | 20:17:19.801 | 0.767549 s |
| issue :203 | 20:17:19.033304 | 20:17:21.639 | 2.605696 s |
| resources :205 | 20:17:19.033993 | 20:17:33.948 | **14.914007 s** |

The four HTTP starts are within0.857ms. The fifth snapshot read is fixture dispatch-count SQL, so it is not an API access-log line. Parallelization is therefore visibly working for the HTTP reads; it cannot shorten the resources handler's internal serial projection.

Correlated resources backend **PID31824**, `cli03/pg-waits.jsonl:224–236`: snapshot setup at :224, then runtime → accounts → workspace organization → invocation targets → participant → grants → installation → agent → binding reads. Every one of these13 observations is `idle in transaction / ClientRead / blockers:[]`. Transaction age reaches13.992244s at :236; current/last query ages across those samples are only25.010–123.902ms. For example:

| Raw sample | Local time | Last query | Query age | Transaction age |
| --- | --- | --- | ---: | ---: |
| :224 | 20:17:19.327 | read-only snapshot SET | 88.626 ms | 167.937 ms |
| :227 | 20:17:22.817 | GetInstallationWorkspaceOrganization | 123.902 ms | 3.650873 s |
| :232 | 20:17:28.579 | GetManagedInstallation | 61.989 ms | 9.409067 s |
| :234 | 20:17:30.959 | GetAgent | 55.684 ms | 11.791131 s |
| :236 | 20:17:33.158 | GetInstallationBinding | 74.047 ms | 13.992244 s |

The issue-detail backend PID31818 also appears early (:225) and then finishes, while31824 continues. This is not one14-second binding or installation query.

**2. cli03 runtime-list: a long promotion transaction, no captured lock blocker.**

`cli03/api.log:148`: **20:14:42.172845–20:14:58.474**,16.301155s, HTTP200. Correlated PID31816 in `pg-waits.jsonl:73–90`:

- :74 samples `ListCollaborationRuntimeWakeups`, active/no wait, query age11.037ms; :75 samples cursor advance with xact age0.647159s.
- :76 begins the observed promotion chain (`GetCollaborationWorkflowUserIDs`, xact age0.786038s). :78 `LockCollaborationGateShared` and :79 `LockCollaborationChannels` are already **ClientRead**, query ages18.516/30.315ms—not lock waits.
- :81 and :83 are completed authority namespace/agent checks, again ClientRead. :82 samples `GetAgentRuntime`, query age208.260ms but xact age6.110188s.
- :85 is the only in-window IO sample: `RebatchCollaborationWaitingInputs`, `DataFileRead`, query age**20.446ms**, xact age8.761039s. Do not call this an8.76-second rebatch SQL.
- :87 `CreateCollaborationRuntimeGroup` and :89 `StartCollaborationBatchInputs` show progress, reaching xact age12.347941s. :90 has a new short transaction (0.247523s), proving the preceding long transaction ended before the HTTP response.
- All captured blocker arrays are empty. Of the14 promotion-chain samples :76–89,13 are idle-in-transaction ClientRead and one is the20ms IO observation.

Thus the earlier hypothesis of *possibly many serial promotions* is unnecessary to explain this observed interval: **one observed promotion transaction already occupies over12.3s**. The preceding scan transaction did not retain its cursor for that entire period.

**3. cli03 longest comment: almost the entire HTTP span is a progressing write transaction.**

`cli03/api.log:186`: **20:16:22.924411–20:16:43.164**,20.239589s, HTTP201. PID31823 has15 consecutive samples `pg-waits.jsonl:177–191`, all idle-in-transaction ClientRead, all blockers empty. It progresses from `begin` through source-member/participant locks, runtime and managed-authority reads, waiting-batch locks and `CreateCollaborationContentEvent`. At :189, `GetCollaborationParticipant` has query age**22.697ms**, transaction age**16.945096s**. At :191, content-event creation has query age34.601ms and transaction age**19.264909s**. None of these samples captures a SQL query running for19s.

cli02 independently reproduces the pattern: `api.log:183`,19:41:44.122245–19:41:59.875,15.752755s, correlated PID22498; `pg-waits.jsonl:169–182` is14 successive ClientRead observations with changing SQL. The final xact age is15.044311s, but last query age25.257ms.

**4. cli02 resources corroboration.**

`api.log:180`,19:41:25.995969–19:41:39.649,13.653031s, PID22501. `pg-waits.jsonl:156–165` records10 successive ClientRead observations ending with xact age13.287080s and last query age37.220ms. The same PID's earlier resources call (`api.log:172`,9.843070s) is sampled11 times at :134–144 with changing SQL, all ClientRead. This is not a cli03-only effect introduced by parallel reads.

### Exact captured query identities

MD5s were matched offline against generated query strings; no SQL was executed. **All named-query hashes observed in both files match current generated SQL either in full or at its first1023 bytes.** The latter explains apparent mismatches for long SQL such as `GetCollaborationAccess`; the observer hashes the activity view's truncated text, not necessarily the complete prepared statement. It cannot prove two full queries with the same1023-byte prefix are identical.

| Query / command | Captured MD5 | Relevant observed fact |
| --- | --- | --- |
| GetAgentRuntime | `ab977c490b3a57f8462325cf6a80be27` | Repeated short last-query ages in resource/promotion/claim transactions |
| GetInstallationBinding | `ab4bac90a7c2cf32f6ae076e85ba2c64` | cli03 final resources sample :236,74.047ms; not13.99s SQL |
| GetManagedInstallation | `34696913d960e23f2dad281ef946f9d0` | Repeated ClientRead observations during final resources |
| GetPasswordCredential | `fc681c73a11ce58bad258feeb9fe1956` | cli03 comment :186,60.553ms query /13.113837s transaction |
| GetInstallationWorkspaceOrganization | `76fa4da31533803047dce250e4a2578b` | cli02 resources :164,333.528ms last-query age /11.903022s transaction |
| TryLockCollaborationAuthorityNamespace | `fb5149df423d12556f90fa19e3114b91` | Captured as completed/ClientRead, no blockers |
| RebatchCollaborationWaitingInputs | `74099e973293b8681a501d9bae501393` | cli03 :85, DataFileRead,20.446ms active query age |
| CreateCollaborationContentEvent | `60910e171ad5131ba9b03ffde7da6d1d` | cli03 :191,34.601ms last-query age /19.264909s transaction |
| GetCollaborationAccess, first1023 bytes | `25347b22c4f50824ec926e71065d06b1` | Not evidence of a different API query or source mismatch |
| `begin` | `8d589afa4dfaeeed85fff5aa78e5ff6a` |101/124 idle-in-transaction ClientRead observations; includes background transactions |
| read-only snapshot SET | `cf4a18b9d023ea61a95f5c7539dbcaef` | Confirms observed resource snapshot boundary |
| `rollback` | `fff66e9b3d962fa319c8068b5c1997cd` | Frequently last statement on fully idle connections; idle age is not rollback execution time |

Exact captured IO inventory: cli02 none; cli03 only :1 `LockNextCollaborationConfigInvalidation`38.359ms and `LockNextCollaborationAccountInvalidation`46.529ms, plus :85 rebatch20.446ms. No sustained IO wait is established. Highest active query age cli02: `ListExpiredPendingSourceContextsForCleanup`, :251,83.343ms. cli03: `SetAgentRuntimeOffline`, :14,202.988ms. These maxima are sampled ages, not upper bounds on every completed SQL duration.

### Source relationship and fix decision

The observed repeated chain is explained by current source:

- `server/internal/service/collaboration_resources_view.go:95,108,132` performs configuration, target authorization and per-resource consent projection serially inside the read-only transaction opened at `collaboration_comments.go:103`. `collaboration_authority.go:388` repeats configuration inside authorization; :440 and `collaboration_resources_view.go:230` each enter identity validation.
- `server/internal/service/collaboration_authority.go:302` calls `CheckPasswordVersion` for each managed identity. `server/internal/auth/password_session.go:81,88,102` reads user/credential and checks managed binding. `server/internal/auth/managed_binding.go:40,50,60,67,77` then reads binding, installation, internal organization, workspace organization and membership. These are the exact query families retained in the long ClientRead timelines. Raw samples do not include parameters, so identical query names alone do not establish the same resource was checked twice.
- Runtime-list source `server/internal/service/collaboration_runtime.go:650,694` performs promotion under the transaction and authority fences; the observed `CreateCollaborationRuntimeGroup` and `StartCollaborationBatchInputs` match :837/:863.
- The resource projection file in retained source02/source03 is byte-identical to current. Both receipts pin API binary SHA256 `a4e3a2c12cd0ec3154796b1dc012675cdbdb15c8362e26b5dee51e706896db16`. Full binary reconstruction was not performed.

**Do not make a lock or pool “root fix.”** The remaining boundary is the elapsed time of successive pgx calls and the client/transport work between them. A safe read-only projection improvement could reduce independent SQL round trips while retaining all checks, but the captures neither quantify that improvement nor identify the initiating client/transport delay. In particular, this report does not authorize permission caching, parallel operations on one pgx transaction, or changing writer locks. The smallest missing evidence is a request-tagged pgx call timeline for one resources request, tied to backend PID; this is a handoff fact, not a request for another full run.

### Files found, specs and evidence integrity

- `cli02/` and `cli03/{api.log,pg-waits.jsonl,receipt.json,playwright.json}`: actual request, database-state, timeout and terminal evidence. `timings.jsonl` remains adapter/container phase evidence, not SQL timing.
- `.omx/private/acceptance-fixes/native/pg_monitor.py:10–18`: fresh `psql -c` observations, followed by0.5s sleep; actual cadence includes process/query overhead. It captures blocker PIDs and query identity but no SQL parameters or request IDs.
- `.trellis/spec/server/database-test-isolation.md:285`: request-scoped performance attribution and sampling limits.
- `.trellis/spec/server/collaboration-runtime-recovery.md:32`: runtime/cursor transaction boundaries; preserve authorization fences.
- `.trellis/spec/server/collaboration-host-execution.md:340`: pool pressure is not permit-failure causation; original Host status0/503 and cancellation ordering remain unresolved by these different runs.
- External references: none newly consulted. PostgreSQL17 activity/locking semantics are linked in `cli-api-delay-review.md`; repository pgx dependency isv5.9.2 (`server/go.mod:18`).

| Evidence | SHA256 |
| --- | --- |
| cli02/api.log | `eac206df668cb4ebbf3ad7fb8bfcdb948bb6998da27b1beb51583176ff2e69e8` |
| cli02/pg-waits.jsonl | `36034aeb603b0327e30754eb8817d31fc82e37eca504d348fdafeb80ace6fa7f` |
| cli02/receipt.json | `b22e4da7ce90edc87b0db9dda1e459832402cad3f9820e26fa53eb288e01a5cc` |
| cli03/api.log | `bc4c7c8bdc9ca6a05b1af60a5e0ddd76244215e75ef0a7ebd7a8d4c469d4bb70` |
| cli03/pg-waits.jsonl | `bc19815859181659ea4f4a1013e99d376bb0fe58eb364d95d96ba0d5445a165b` |
| cli03/receipt.json | `135b2daebda5f1a6cf71ee22e5f5ce30d25074e16f28d65cc73a9951bb8a27f2` |

## Caveats / Not Found

- No long active SQL or blocker is present in the samples. Subsecond waits and SQL completed between roughly1s observations remain unmeasured. Do not extrapolate sampled state counts into exact time shares.
- `ClientRead` means PostgreSQL waits for the client; it does not by itself establish Go CPU saturation, a specific network fault, TCP behavior, or scheduler delay. Host1m load ranges6.55–22.23/13.20–27.04 and observer wall times do not identify a process or establish causation.
- Transaction start reconstructed from `at - xact_age_s` has small jitter because the query calls `clock_timestamp()` repeatedly while building rows. The consecutive PID timelines above use progress and transaction-age reset, not exact microsecond equality.
- Different runs reached different stages and had different concurrency; cli03's208s overlapping HTTP sum versus cli02's170s is not a controlled performance regression measurement.
- No tests, mutation, additional run, pool change, timeout change or source fix was performed. Native Hooke retains implementation and actual-Main ownership.
