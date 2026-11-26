# Research: CLI API delay and Host prelaunch relationships

**Follow-up with actual cli02/cli03 wait evidence:** [cli-api-delay-cli02-cli03.md](cli-api-delay-cli02-cli03.md). It finds no captured lock/blocker or multi-second active SQL; long request transactions repeatedly show ClientRead with changing queries. Use that note for the latest attribution; the hypotheses below describe cli01's earlier evidence limits.

- Query: Explain the retained cli01 API delays; rank concrete mechanisms and identify the smallest query/lock experiment without attributing causation to pool size or host load without evidence.
- Scope: mixed; recorded evidence and current repository source, with PostgreSQL primary documentation for lock/observer semantics. Read-only diagnosis; this research file is the only write.
- Date: 2026-11-25
- Workspace: `/Users/artisan/.codex/worktrees/main-acceptance-fixes/multica-0.4.37`
- Owner boundary: Native Hooke owns implementation and full runs. No test, API request, SQL connection, Docker operation, old runner, account access, or recursive agent was executed by this sidecar.

## Findings

### Actionable conclusion

**Investigate query execution/round-trip amplification first, then specific writer lock waits. The retained data do not establish a dominant pool-acquisition delay.** Resources and issue-detail responses hold one connection through a repeatable-read, read-only snapshot containing many sequential queries. Runtime-list performs scheduling mutations, including serial promotions, before responding; it is not a cheap SELECT endpoint. The existing native `pg_monitor.py` can discriminate actual PostgreSQL wait classes without another observer or duplicate full run. Its remaining gap is request-to-backend attribution and completed query/acquire timing.

**The initiating slowdown is still UNKNOWN.** Repeated reads and long transaction lifetimes are proven code mechanisms/amplifiers, not proof of a particular slow SQL plan, host CPU bottleneck, deadlock, or leaked connection. Do not change pool size, deadlines, runtime scan limits, or authorization fences based on this review.

### Retained measurements

Evidence prefix: `.omx/private/acceptance-fixes/native/cli01/`. API totals were recomputed from all completed `http request` records, normalizing UUID path components only.

| Endpoint | Calls | Sum | Maximum | Evidence |
| --- | ---: | ---: | ---: | --- |
| GET collaboration issue resources | 18 | 42.013 s | 11.619 s | `api.log:177`; earlier 4.818/6.798/9.216 s at :134/:141/:149 |
| GET collaboration issue detail | 25 | 29.885 s | 6.696 s | :173 |
| GET collaboration runtime runs | 1 | 22.374 s | 22.374 s | :156 |
| POST collaboration issue control | 2 | 17.292 s | 11.293 s | :152/:164 |
| POST runtime claim / permit | 1 each | 4.131 / 4.428 s | same | :158/:159 |
| All HTTP requests | — | **146.190095 s** | — | Entire access log |

The HTTP sums are request durations, not a disjoint wall-clock breakdown. `timings.jsonl` contains 149 adapter phase records, not query timings. Docker phases sum to 83.346 s, including seven starts (one version probe plus six relay starts), with six `cp` spans totaling 13.210 s (1.595–3.362 s each). Start spans can include relay HTTP work, so adding 83.346 s to 146.190 s double-counts overlapping work. This evidence does not identify container staging as the dominant API cause.

`receipt.json` reports `exit:1`, `cleanup_status:"fail"`, `source_unchanged:true`, `listener_closed:true`, `remaining_db_clients:0`, and restored gate booleans. Its top-level `status:"completed"` means the launcher finished, not that acceptance passed. Resource cleanup separately reports pass. The original 240-second failure remains a failure; six relay starts do not establish all downstream assertions ran.

Pool evidence is materially more restrictive than the warning label suggests:

- `api.log:5` records max12/min1. Some samples show acquired12/idle0; others warn with idle connections still available.
- `server/cmd/server/dbstats.go:156` computes interval deltas; :165 divides cumulative successful acquisition milliseconds by acquisition count with integer truncation; :182 warns when **any** empty or canceled acquisition occurred, regardless of current idle count. The installed dependency source `/Users/artisan/go/pkg/mod/github.com/jackc/pgx/v5@v5.9.2/pgxpool/stat.go` confirms `AcquireCount` and `AcquireDuration` cover successful acquisitions.
- Across every emitted pool sample through 19:23:05, there are 4,271 successful acquisitions and zero canceled acquisitions. Summing `count * avg_ms` gives 8.844 s; allowing for the integer truncation bounds their aggregate completed acquisition duration below **13.115 s**. This includes background work, not only HTTP.
- This bound is not a per-request percentile and omits acquisitions still unfinished at sampling boundaries. It nonetheless makes a claim that pool acquisition accounts for most of the observed 146.19 s implausible. The final slow HTTP completed before the final stats sample.
- For the 22.374 s runtime-list request (approximately 19:21:25.281–19:21:47.655), the enclosing stats at :155/:157 show 251/245 acquisitions, both avg0 ms, both canceled0. These windows bound their completed aggregate acquisition time below 0.496 s, assuming the logged pool serves this path as the source wiring indicates.

### Ranked mechanisms and exact code relationships

#### 1. Sequential query amplification inside long read snapshots — strongest concrete lead

`handler/collaboration_resources.go:26` → `service/collaboration_resources_view.go:70` → `service/collaboration_comments.go:103`.

The reader calls `Begin`, executes `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY`, resolves access, executes the entire projection, then commits (:104–120). All resource helpers inspected receive the transaction's `q`; `RuntimeLookup.Get` calls that same query object (`service/agent_runtime_lookup.go:28,43`). No second pool checkout hidden in those inspected resource helpers was found.

For each target, `Resources` performs:

1. `collaborationConfigurationSnapshot` at `collaboration_resources_view.go:95`.
2. `collaborationScopeUsesHost` at :103; this re-reads all session runs once per target (`collaboration_host_compatibility.go:127`) and projects runtime mode.
3. `authorizeCollaborationTargetInput` at :108, which calls **the same configuration snapshot again** (`collaboration_authority.go:388`) and then checks grants/participants/accounts/managed identity.
4. Another `ListCollaborationTargetGrants` at :118, resource summaries at :123, and another consent-current chain at :132.

Each full configuration snapshot performs at least `GetAgent`, `GetAgentRuntime`, `GetMemberByUserAndWorkspace`, and `ListAgentInvocationTargets` **per roster resource** (`collaboration_authority.go:157,164,175,182`); squads additionally read squad and membership (:114/:121). A current target therefore repeats these four queries twice before summary/consent work. Full authorization and consent-current both call participant, `CollaborationGrantOwnerCurrent`, recipient eligibility and `collaborationIdentityCurrent` (`collaboration_authority.go:419–440`; `collaboration_resources_view.go:212–230`). Identity checking re-reads runtime/binding/account credentials (`collaboration_authority.go:254,287,302`; `auth/password_session.go:81,88,102`).

SQL names to classify first: `GetCollaborationAccess` (`pkg/db/queries/collaboration.sql:1`), `GetAgentRuntime` (`runtime.sql:15`), `ListCollaborationTargetGrants` (`collaboration_authority.sql:3`), `CollaborationGrantOwnerCurrent` (`collaboration_targets.sql:56`), `FindCollaborationRecipients` (`collaboration_recipients.sql:9`), plus the plain account/configuration reads above. The recipient query has parameterized OR branches; no generic-plan regression is established without the observed query and plan.

Issue detail has the same kind of amplification, at smaller fanout. `handler/collaboration.go:249` resolves access before `collaborationIssueResponse`; the latter opens a read-only snapshot (:117–128), resolves access again, reads workspace and account summaries, then delivery/fields/execution. `service/collaboration_execution_view.go:157–198` executes session, gate, channels, targets, commands, groups, runs, group-input reads and one dependency query per group. Version6 adds per-run metadata/memory reads (:215). This can grow with retained history even without queued work.

**Implication:** repeated small query/transport/client scheduling costs can accumulate to seconds while acquisition remains cheap. One long query instead would show a very different profile. Neither has yet been measured per request. Authorization snapshot consistency must remain intact in any later optimization; this note does not propose bypassing checks or caching authority across requests.

Resources/detail are ordinary SELECT snapshots. PostgreSQL row-write locks do not block ordinary SELECT reads; a conflicting table `ACCESS EXCLUSIVE` lock can. Therefore a claim that Stop's issue-row lock directly explains the 11.619 s resources read needs different evidence. Source: [PostgreSQL 17 explicit locking](https://www.postgresql.org/docs/17/explicit-locking.html).

#### 2. Runtime-list serial promotion work and writer lock contention — concrete mechanism, actual blockers unobserved

`handler/collaboration_runtime.go:146` → `service/collaboration_runtime.go:626` performs daemon authorization, wake scan and wake draining, waiting-target scan, **one promotion transaction per returned target**, then candidate priority/scan. Waiting and wake pages each allow 16 items (`pkg/db/queries/collaboration_runtime.sql:88,98`); the final candidate page intentionally permits only one (:53). A single 22.374 s HTTP span may contain multiple completed transactions and substantial SQL work, rather than one lock wait.

The runtime/cursor scan transaction is correctly committed before promotion. `collaborationRuntimeScanPage` (:493) takes `LockAgentRuntime` (:502), re-authorizes, ensures/locks cursor (:515/:518), queries/advances and commits through `collaborationRuntimeMutation` (:450). Do not incorrectly attribute promotion-duration lock retention to the already released scan cursor.

Promotion then calls `lockCollaborationRuntimeSource` (:694), whose acquisition order is explicit (`collaboration_runtime.go:119`): workflow user rows → managed namespace/admission → shared execution gate/status-catalog fence → workspace → issue/state/session → channels/targets/grants/commands/batches. SQL relationships:

| Source operation | SQL object / mode | Potential competing operation |
| --- | --- | --- |
| `LockCollaborationUsers` | `"user"` rows, ordered `FOR UPDATE`; `collaboration.sql:22` | Queue control, another same-user collaboration, account/binding writes |
| `LockManagedRuntime` / `LockInstallationNamespace` | Namespace transaction advisory lock; `managed_runtime.go:129`, `managed_installation.sql:16` | Same namespace registration/admission/other managed runtime work |
| `LockManagedInstallationForAdmissionRead` | `managed_installation FOR SHARE`; `managed_admission.sql:1` | Exclusive admission change; other shared readers do not conflict |
| `LockCollaborationGateShared` | Singleton gate `FOR SHARE`; `collaboration.sql:80` | Exclusive gate update, not another shared gate reader |
| `LockIssueStatusCatalogShared` | Workspace advisory shared lock; `issue_status.sql:124` | Exclusive status-catalog mutation |
| `LockWorkspaceForChatSessionCreate` | Workspace `FOR KEY SHARE`; `workspace.sql:82` | Workspace deletion/exclusive lock, not ordinary fellow creators |
| `LockCollaborationIssue` | `issue FOR NO KEY UPDATE`; `collaboration.sql:162` | Same-issue control/content/lifecycle writers |
| `LockIssueCollaboration` / `LockCollaborationSession` | State/session `FOR UPDATE`; `collaboration.sql:56,59` | Same-issue control/runtime/content/reconciliation |
| `LockCollaborationChannels/Targets/...` | Ordered session-scoped row locks; `collaboration_batches.sql:3,48`, `collaboration_targets.sql:19`, `collaboration_authority.sql:51` | Same-session stop, promotions, invalidation |
| `LockAgentRuntime` / scan cursor | Runtime/cursor `FOR UPDATE`; `runtime.sql:28`, `collaboration_runtime.sql:112` | Runtime update/delete and another scan on the same runtime |

Promotion also performs preparation validation, namespace occupancy and pending-derived-run queries before creating group/run/reservation and recording events (`collaboration_runtime.go:792–869`). These happen with the source locks held. Control enters the same writer family via `mutateCollaborationQueue` (`collaboration_control.go:159,202`; `collaboration_queue.go:209,315–369`), holds a connection/locks until commit, and rechecks authorization and host-mode scope before mutation. Both original control durations are therefore candidates for long *transaction work* or *actual lock waits*, not proof of either.

There are background competing transactions: API startup starts reconciliation and realtime workers (`cmd/server/main.go:700–703`); account/config reconciliation polls every 2 s (`collaboration_account_reconciliation.go:71`; `collaboration_config_invalidation.go:60`), and realtime polls every 500 ms (`collaboration_realtime.go:64`). Configuration reconciliation can reach the same issue/session via `lockCollaborationResourceSource` (:88). These are a concrete reason to classify non-HTTP DB activity. Their being enabled does not prove they blocked cli01. Normal background transactions alone also mean acquired connections need not equal concurrent user requests.

#### 3. Pool exhaustion / transaction leak — secondary hypothesis, contradicted as the principal measured cost

Connections are retained intentionally across multi-query transactions, so occupancy is real. In inspected paths there is commit plus deferred rollback or rollback-on-error; query helpers retain the caller's transaction. No leaked transaction or nested pool self-deadlock was found in these readers. A future trace showing a request waiting to acquire while its own backend is idle in transaction would justify checking a missed nested checkout. The present successful-acquisition bound and zero cancellations do not support increasing max12.

#### 4. Host/cluster contention, planner or I/O slowdown — unresolved underlying cause, not a diagnosis

Several endpoint families slow together late in cli01, including plain SELECT readers. That is compatible with increased per-query database/transport/client scheduling cost. It does not distinguish CPU pressure, I/O, planner/JIT work, a particular SQL plan, or application runtime pauses. Broad-suite concurrency is mentioned in the native progress note, but cli01 has no retained host CPU, per-query or blocker samples establishing causation. Other test databases cannot directly row-lock this task's ordinary issue rows; shared cluster resources are a separate mechanism requiring measurements.

### Relationship to the original Host prelaunch failures

The prior review `.trellis/tasks/11-24-main-post-merge-e2e-acceptance/research/host-visit-failure-review.md` records successful claims followed by daemon permit status0, API503 and never-launched proofs. It expressly does not know whether API delay or cancellation happened first. Keep that classification.

There is an exact shared source boundary: legacy permit calls `lockCollaborationRuntimeRun(..., true, true)` at `collaboration_runtime_capabilities.go:344`; Host permit does the same at `collaboration_host_runtime.go:96`. Both use `collaborationRuntimeMutation`, and the common source lock chain above can consume the prelaunch budget. `handler/collaboration.go:93` still collapses non-domain errors to generic503. In cli01, by contrast, runtime-list, claim and permit all completed200, so this is evidence of a shared slow path, **not a reproduction or explanation of the earlier Host503**. Provider execution cannot explain a delay that occurs before permit.

### Smallest discriminating next experiment

Use the **already planned Native Hooke actual-wait run and observer**. Do not start another full test or competing sampler. Keep original240s, pool12/1, API binary/source binding and containment unchanged.

1. First classify its existing `pg_monitor.py` output. At `:10–18` it already samples task-DB PID/state/wait type/wait event/xact age/query age/query-name/hash/blocking PIDs, aggregate cluster clients and host load. Each `psql -c` is a fresh autocommit observation; its effective interval is subprocess/query duration plus0.5s, not exactly0.5s. Retain the actual sampling gaps and errors. `idle in transaction` query text identifies the **last** query, not a currently running query.
2. For runtime-list, split the names into scan (`LockAgentRuntime`, cursor ensure/lock, wake/wait/candidate SQL), promotion (`LockCollaborationUsers`, namespace/source/queue locks, `CreateCollaborationRuntimeRun`), and ordinary auth reads. Match timestamps to the 200-response completion minus its duration. A sustained `Lock` sample plus nonempty blockers identifies a real wait; inspect the blocking task-DB PID's state, transaction age and safe query name. Resolve chains, not only first-level waiter counts.
3. Only if that observer leaves ambiguity, the smallest implementation-side addition is request-scoped pgx tracing for **one resources read and one runtime-list call already in the authorized run**, or one retained synthetic-state diagnostic pair if the run is over. Put it before auth middleware to include boundary work. Record a generated request tag, monotonic begin/end, backend PID, operation phase, fixed sqlc query name (unknown SQL as a hash only), query count/elapsed, acquisition elapsed, transaction duration and bounded SQLSTATE/context outcome. Never record SQL arguments, raw SQL/error text, auth headers or private payloads. Record `QueryRow` through Scan and rows through close/exhaustion; a timer around `QueryRow` construction alone misses work. The existing `iterationCapacityTracer` at `handler/iteration_capacity_benchmark_test.go:119` only counts statements and is not itself adequate timing instrumentation.
4. If an actual blocking sample appears, one bounded read-only lock inspection is sufficient; no settings changes or induced lock are required:

```sql
SELECT a.pid, a.backend_start, a.state, a.wait_event_type, a.wait_event,
       extract(epoch FROM clock_timestamp()-a.xact_start) AS xact_age_s,
       extract(epoch FROM clock_timestamp()-a.query_start) AS query_age_s,
       substring(a.query FROM '-- name: ([A-Za-z0-9_]+)') AS query_name,
       pg_blocking_pids(a.pid) AS blockers,
       l.locktype, l.mode, l.granted, l.relation, l.transactionid,
       l.classid, l.objid, l.objsubid
FROM pg_stat_activity a
LEFT JOIN pg_locks l ON l.pid=a.pid AND NOT l.granted
WHERE a.datname=current_database() AND a.pid<>pg_backend_pid()
  AND (a.wait_event_type='Lock' OR a.state='idle in transaction');
```

Run through the owner-approved diagnostic connection only, in fresh autocommit samples. Do not persist raw activity SQL. Only the requested task DB's rows are selected; unrelated database details need not be collected. PID plus backend_start avoids process-reuse confusion. For a blocker chain, include blocker PIDs from the same task DB even when those blockers are active rather than idle.

| Observation | Discriminates / next step |
| --- | --- |
| Many completed short queries, low acquire time, no sustained wait | Quantify duplicate snapshot/consent reads and transaction-held wall time; investigate safe same-snapshot reuse after measurement |
| One named query dominates, active with no wait | Inspect that exact read-only query's plan and row estimates on owned synthetic state; no speculative index, ANALYZE, JIT or pool change |
| `Lock` + blocker chain on namespace/users/source | Attribute wait to the exact writer and transaction lifetime; preserve admission/authority lock contracts |
| `Lock` on ordinary read + relation lock | Look for task-DB table `ACCESS EXCLUSIVE`; ordinary issue-row contention is not the explanation |
| `IO` / `LWLock` | Attribute the observed wait class; host load alone is insufficient |
| Long transaction + `ClientRead` / idle-in-transaction gaps | Client-side computation, scheduling or delay between statements; not proof that PostgreSQL is executing slowly |
| Long acquire with all pool slots held, backend activity otherwise short | Pool queueing becomes a measured contributor; inspect holders/nested checkout before tuning capacity |
| All traced DB/acquire work short but HTTP remains long | Inspect auth, serialization, middleware, blocking metrics/logging and Go scheduling; do not label residual time CPU without evidence |

The existing observer does not see client-side pool queues, completed short queries between samples, or an HTTP request tag. Host load average is context, not attribution. PostgreSQL documents activity state/wait separately and retains activity snapshots within a transaction: [PostgreSQL 17 activity/statistics](https://www.postgresql.org/docs/17/monitoring-stats.html).

### Files found and related specifications

All source paths above are relative to `server/internal/` unless prefixed by `server/`, `pkg/`, `cmd/`, `.trellis/`, `.omx/` or `e2e/`. SQL paths prefixed `pkg/` resolve under `server/`; `cmd/` likewise.

| File | Purpose |
| --- | --- |
| `.omx/private/acceptance-fixes/native/cli01/api.log` | Completed request times and periodic pool counters |
| `.omx/private/acceptance-fixes/native/cli01/timings.jsonl` | Container/relay adapter phase durations, not SQL timings |
| `.omx/private/acceptance-fixes/native/cli01/receipt.json` | Failed exit and cleanup/source-restoration facts |
| `.omx/private/acceptance-fixes/native/pg_monitor.py` | Existing owner-side read-only wait observer; inspected, not run |
| `e2e/collaboration-installed-clients.spec.ts:32` | Original240s; :40–47 reads issue/commands/participants/resources serially for snapshots, repeated for refusal comparisons at :73/:83 |
| `server/internal/service/collaboration_resources_view.go` | Repeated target/configuration/consent projection |
| `server/internal/service/collaboration_comments.go` | Borrowed repeatable-read read-only content snapshot |
| `server/internal/service/collaboration_authority.go` | Configuration, consent and identity checks reused by projections/writers |
| `server/internal/handler/collaboration.go` | Detail snapshot and generic503 mapping |
| `server/internal/service/collaboration_runtime.go` | Runtime scan transactions, serial promotions and common source locks |
| `server/internal/service/collaboration_queue.go` | Control transaction lifetime and lock order |
| `server/internal/service/managed_runtime.go` | Account/namespace/admission fences |
| `.trellis/spec/server/collaboration-runtime-recovery.md` | Runtime→cursor order, commit before promotion,16 waiting/1 candidate bounds |
| `.trellis/spec/server/collaboration-host-execution.md:340` | Status0 vs503, sanitized transport/context/clock evidence, no causal pool claim |
| `.trellis/spec/server/database-test-isolation.md:285` | Preserve failed gates; request-scoped tracing; no CPU claim from residual time; sampling limitations |
| `.trellis/spec/server/platform-observability.md` | Capacity and operational evidence contract |
| `.trellis/tasks/11-24-main-post-merge-e2e-acceptance/research/host-visit-failure-review.md` | Original Host prelaunch evidence and remaining unknown cancellation ordering |

External references are PostgreSQL17 official documentation, matching the repository's documented CI major; the actual cli01 server version was not queried. `server/go.mod:3,18` declares Go1.26.11 and pgx/v5 v5.9.2. These are source dependencies, not a claim about the binary's compiler/runtime.

## Caveats / Not Found

- cli01 contains no per-query timings, PostgreSQL blocker/wait samples or request-to-backend mapping. A specific root SQL/blocker is not proved. The report ranks mechanisms to measure, not confirmed root causes.
- Current source was inspected. Receipt `source_unchanged:true` binds the run's own manifest; this sidecar did not independently reconstruct the frozen API binary's complete source manifest. Native Hooke must compare its accepted source binding before applying these current file/line mappings to the binary.
- No permanent pool leak, nested checkout in the inspected resource-reader helpers, deadlock, provider-side cause or staging-dominant cause was found. This is bounded inspection, not proof of absence across the entire API/middleware stack.
- The pprof listener failed to bind at `api.log:26`; port6060 must not be used to profile this API on assumption alone, since it belongs to another listener.
- No broad suites, account APIs, SQL mutations, EXPLAIN ANALYZE, ANALYZE, cluster settings, timeouts or pool sizes were touched. SQL above is a proposed read-only observation, not an executed query.
- No lint/test rerun was needed for this research-only file. Original failure and cleanup status remain unchanged.

Observed evidence SHA-256:

```text
api.log       86eacf259b2b6e45bed0511cba9fedef9755906814d481195d1da556836784d8
timings.jsonl 66ba43fb6312bdcf361855247bd258dd77fdc839a5d0036b408385be0d2e2ccb
receipt.json  bb87726f4bbf890d9c4ca14174964a1e68dbe59c36a1d38a81a5bc133083c5a8
```
