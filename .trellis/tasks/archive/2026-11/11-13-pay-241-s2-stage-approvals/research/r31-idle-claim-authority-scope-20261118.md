# Research: R31 idle-claim authority scope

- Query: Supported nil/no-cache ordinary idle claim versus harness/production; safest bounded no-work/R4 RED and potential read-only preselection without weakening actual reclaim.
- Scope: internal / filesystem-only. Only this new report pair is writable; final mode0444. No code/test/config/spec/peer/old-report writes, Go/SQL/DB/API/browser/runtime/signals/Git or agents.
- Date: 2026-11-18 (Asia/Shanghai).
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Findings

**Best next step: authorize only a fresh same-layer true-idle concurrency RED, while product remains frozen.** The desired contract is concrete: an authenticated valid ordinary claim poll with no queued or potentially reclaimable dispatched work should not exclude a same-Q human R4 read through a global user UPDATE fence. Actual queued claim/reclaim stays a real writer, with its original account/binding/task/admission/generation locks and full SQL. A failing idle regression proves a reachable no-work contention; it does not identify the historical runtime holder or automatically approve a repair.

### Source and original failure facts

Consuming R30 source is `/Volumes/artisan/.multica-acceptance/pay241-r30-qa-read-20261118-pohc3l9t/source-green-r01` (current binding records5053 files, not the older5052 baseline). ROOT reports a fresh R31 5053-file source and three exact service profiles prepared; this research neither allocates nor executes them. UI/R4 diagnosis and the earlier eligible writer chain are accepted peer inputs, not redone here.

R30 original4 is **2PASS/1FAIL/1NOT_RUN,retry0**: release/rollback E02 passed; E03 failed before QA approve click after the unchanged original20-second wait; E04 was not run. Exact Q=`8aa26ea6-e1a9-4607-8b20-dcc4834731b8`, root=`01a1eb40-a908-7ce0-a472-b9d3d743a85b`, candidate=`01a1eb40-aba3-7995-b2e8-e1971ff79660`.

`runtime/original-e03-qa-read-capture-summary-r01.json` SHA `b2c396b674d5d94f61f6c929b7c6734f0e1eedd92582ba2ceafe083a6d8e41a5` records3193 entries/no4096cap. Peer source frames prove current-Q singleton `LockArtifactUsers` SHARE NOWAIT at the first observer/current-Q fence: request7057cde6/trace1055/20:44:20.339–.340/PID96015 and requestb0ee8d1e/trace1061/20:44:35.351888/PID96066 each55P03→sameHTTP503`collaboration_unavailable`. They precede producer/retained-issuer checks. Prior successful trace1051 hasQA allowed=true/blockers=[]; root8be08d6c200 is a plain different projection. **The failed read/query/cause class is now correlated; the physical holder is still unknown.** A later witness of idle connections/0locks cannot reconstruct those old holders. Do not add Refresh merely to turn the unchanged original test green, alter read deadlines/retries, or replay R28's successful completion.

### Cache initialization correction and actual idle behavior

The missing-router-initialization hypothesis is false:

| Source | Actual behavior |
| --- | --- |
| `router.go:1316,1320` | ALWAYS constructs `NewEmptyClaimCache(rdb)` and `NewReclaimCheckCache(rdb)`. This happens in `NewRouterWithOptions`, not only main. |
| `main.go:663` | Calls the same router with conditional `storeRedis`, selected from production Redis configuration. Unset/invalid Redis leaves nil and uses the supported fallback. |
| `collaboration_stage_execution_control_test.go:331,373` | Actual original stage harness passes nil Redis, so both factories return nil. It is not a missing init or a fake authority configuration. |
| `reclaim_check_cache.go:70–74,97–112,271–273` | Nil constructor→nil; every nonempty valid runtime set is due; `MarkChecked` is a no-op. Successful empty reclaim does not suppress the next poll. |
| `empty_claim_cache.go:78–82,124–126,166–168` | Nil constructor→nil; `IsEmpty=false`, version0, `MarkEmpty` no-op. It only covers queued work even when configured. |
| `task.go:340–345` | Raw NewTaskService itself starts without caches; router attaches them afterward. No arbitrary memory cache is silently present. |

For an actual authenticated positive-capacity poll with valid runtime IDs, after initial `ValidateManagedRuntime` and normal due-deferred promotion succeed, nil ReclaimCheck makes Step2 due **on every poll**. `ClaimTasksForRuntimes:3989–3999` calls `runInTx→LockManagedRuntimes` **before** `ReclaimStaleDispatchedTasksForRuntimes`, even if that query later returns0 tasks. Thus `LockManagedRuntime(false)→LockPasswordSession→LockPasswordUser FOR UPDATE` occurs in this legitimate no-cache fallback with no actual task row reclaimed. EmptyClaim is checked only later at4041; setting EmptyClaim alone cannot remove that earlier fence. MaxTasks0/invalid/unauthorized/failed prerequisite paths are different and cannot be used as a false “idle” RED.

Configured Redis is an optimization, never authority: DueRuntimeIDs returns scheduled/backstop due runtime IDs; missing/malformed/failure means DB now. MarkChecked records all runtime backstops and defers seen due hints to bounded retry; schedule TTL/backstop protects missed writes and old rows. Even healthy-cache cold start/backstop still executes the canonical reclaim path. Nil Redis is a supported production configuration, and transient Redis failure reaches it. Harness frequency/no-cache can amplify contention, but the algorithm belongs to product fallback; it cannot be dismissed as harness-only or fixed by an unauthorized Redis dependency/input. No pool constructor or pool/deadline change is required.

The TestMain pool is the explicitly selected `pay241DiagnosticPool`, which adds observation to the same pgx pool; it does not construct Redis. The actual stage harness uses that pool and the same router/service. Production creates its store Redis outside the router and passes it through. Cache factories—not a pool shortcut or missing RouterOptions—explain the difference.

### Existing queries: what can and cannot be reused as a pure probe

`server/pkg/db/queries/agent.sql:1037–1110` has the authoritative **UPDATE** reclaim query, not a standalone pure due-existence query. It checks dispatched/never-started/past recovery window/expired-or-absent lease plus runtime health/visibility, current managed binding/admission/user/password/member and `lock_issue_execution`, then updates generation/lease under task locks. Keep all of it.

`agent.sql:2415–2449` has `ListQueuedClaimCandidatesByRuntimes`, but it only sees queued rows and invokes **`lock_issue_execution(...)`**. A SELECT calling that lock helper is not an unlocked negative probe. It cannot prove absence of dispatched lost-response recovery, and moving it before account/namespace prefix may reverse the original lock order. No exact reusable pure queued+due-dispatched absence query was located in the bounded agent-query inventory. Do not call the UPDATE as a harmless preflight or treat its0 result as proof no authority lock was taken.

### Safest bounded candidate after genuine RED: skip only empty reclaim Step2

A small new **non-authorizing, lock-free EXISTS** query is acceptable scope if RED demonstrates the desired contract. Use necessary eligibility predicates only, without current-authority joins, `lock_issue_execution`, row/table locks or business writes. Suggested shape (not created/executed here):

```sql
SELECT EXISTS (
    SELECT 1
    FROM agent_task_queue t
    WHERE t.runtime_id = ANY(sqlc.arg(runtime_ids)::uuid[])
      AND t.status = 'dispatched'
      AND t.started_at IS NULL
      AND t.dispatched_at < now() - make_interval(secs => sqlc.arg(claim_recovery_secs)::double precision)
      AND (t.prepare_lease_expires_at IS NULL OR t.prepare_lease_expires_at < now())
);
```

Proposed name: `HasPotentialDispatchedReclaimForRuntimes`. This is a **conservative superset** of actual reclaim eligibility, not authorization: actual managed admission/agent/runtime/issue conditions can exclude a present row later. It deliberately does not acquire those fences. The query returns only a boolean, no private task content. A simpler superset `status='dispatched' AND started_at IS NULL` is safe for the strict zero-task idle case but still takes unnecessary writer locks for young/leased dispatches; use the matching DB-clock recovery/lease predicates above if that wider no-work contract is admitted.

Place it **after existing initial runtime/current-auth validation and due-deferred handling, just before Step2's LockManagedRuntimes transaction when dueKeys is nonempty**:

- Reliable false means there was no row satisfying even these necessary reclaim conditions at that read. Skip **only** the reclaim transaction; then continue the original EmptyClaim/list-queued/real claim/finalize path. Do not return the whole claim method early and hide newly queued work.
- True keeps the untouched original `runInTx→LockManagedRuntimes(false)→original UPDATE` and full revalidation/locks. The boolean is never a capability, cached permission, task identity or replacement positive result.
- Error/unknown is not false. Preserve the existing error/fallback policy as explicitly admitted; never label an unavailable probe empty/success. Current credential/binding validation remains ahead of every no-work return, matching the existing cached-empty authority contract.
- A row/clock boundary can become eligible just after the probe. An absence optimization may defer it to the next normal poll, as existing negative caches already do; it must not permanently mark recovery unavailable or delay known due work beyond the established schedule/backstop contract. Positive rows always re-enter canonical checks.
- Keep due-hint/backstop semantics: `MarkChecked` may only follow a successful canonical pass or a reliable necessary-condition negative under the reviewed policy; do not remove due hints or turn skipped/locked rows into no-work. Nil cache remains nil/fallback-safe.

**Potential product write scope only after accepted RED and ROOT's bounded dispatch:** `server/internal/service/task.go`, `server/pkg/db/queries/agent.sql`, corresponding `server/pkg/db/generated/agent.sql.go`, plus the new test file. This is one query/helper, no new dependency, migration/index/schema, shared lock-mode flag or change to real reclaim/current auth. Both singular and batch paths have the same supported fallback issue; initial RED/repair can own batch only, and the singular sibling is an explicit residual applicability decision—not silently declared fixed. Do not delete or weaken `LockManagedRuntime`, replace itsfalse default with shared mode, or let a cache stand in for current authority.

Harness-only Redis alignment is an optional separate test-configuration choice **only if a concrete owned Redis setup is already authorized and declared**, with raw nil fallback still tested. It cannot establish a product guarantee for no-Redis users. The existing caches are Redis-backed; adding miniredis/new artifacts or a new public memory backend solely for this hypothesis is not part of this recommendation. The first RED requires no Redis.

### Concrete RED-only test handoff: 2 top /4 leaf /6 named

**Only NEW `server/internal/service/task_idle_claim_read_concurrency_test.go` in ROOT's fresh R31 service source/profile. Product/query/generated/spec/runtime remain frozen during RED.** ROOT owns the three prepared databases/permits and verification. Proposed selector:

```text
^(TestOrdinaryIdleClaimAndR4Coexistence|TestOrdinaryReclaimPreservesAuthorityFences)$
```

| New top | Exact leaves | Contract |
| --- | --- | --- |
| `TestOrdinaryIdleClaimAndR4Coexistence` | `idle_first`, `r4_first` | Genuine same-Q artifact-bound R4 and valid positive-capacity no-cache ordinary claim, no selected runtime queued or potentially reclaimable dispatched rows; both methods complete while held first remains, original immutable R4/empty claim and zero effects. |
| `TestOrdinaryReclaimPreservesAuthorityFences` | `reclaim_first`, `r4_first` | Real current managed stale-dispatched task and ordinary canonical reclaim; exclusive account/task fence persists, refused/blocked competing method emits no effects before original lock is released, actual successful reclaim advances generation/lease once with original binding/history. |

Complete proposed named set:

```text
TestOrdinaryIdleClaimAndR4Coexistence
TestOrdinaryIdleClaimAndR4Coexistence/idle_first
TestOrdinaryIdleClaimAndR4Coexistence/r4_first
TestOrdinaryReclaimPreservesAuthorityFences
TestOrdinaryReclaimPreservesAuthorityFences/reclaim_first
TestOrdinaryReclaimPreservesAuthorityFences/r4_first
```

Requirements for an actual—not artificial—RED:

1. Reuse the existing real same-Q R4/managed fixture family, establish baseline R4 allowed/current before holding, and genuinely valid managed Q runtime/credential. The source artifact producer may belong to another runtime; fixture proof must show **all selected Q runtime ordinary queued rows and necessary reclaim rows absent**. No queued/stale-dispatched row is planted in the idle cases. Do not use maxTasks0, invalidruntime, revokedauth, dummy response, cache-only fabricated empty result or permission/epoch reset.
2. Use the actual TaskService.ClaimTasksForRuntimes and actual R4 on real PostgreSQL/forwarding DBTX and borrowed transactions. Distinct PID witnesses and actual second-method completion before release establish idle coexistence. Idle-first can hold the real completed no-work eligibility operation/transaction before owner return/commit; read-first holds actual R4 SHARE. Do not hold only a finished method after it released all locks and call that concurrency.
3. Current fallback idle-first should expose the user FOR UPDATE unnecessarily held around zero-row reclaim; R4 fails at actual current-Q SHARE. In read-first the blocking original `LockPasswordUser FOR UPDATE` can be observed waiting; a bounded **test-only explicit cancellation after its genuine entered/blocking state** (e.g. the suggested250ms diagnostic observation, without changing product/read deadlines) records the actual non-success. Do not manufacture55P03 for this blocking writer: preserve actual cancellation/wait evidence and canonical query/PID/UUID. A no-work method is expected to return empty successfully before first release in the desired contract.
4. Actual-work controls deliberately plant the real eligible stale-dispatched ordinary task only in their own fresh fixture. Reclaim-first holds the canonical exclusive fence; current R4 refusal/zero projection is expected. R4-first preserves original writer behavior: it waits or refuses under the existing caller contract, then can reclaim after reader release; do not change that blocking query into NOWAIT or require a new invented error. Verify current binding/admission, before/after generation/lease and original provenance, not just HTTP200 or an empty count.
5. Register no-effect snapshots **after** genuine R4 fixture establishment and any intentional writer-control row setup. For idle leaves compare original task/issue/user/credential/binding/lease/revision/handoff/artifact/operation rows, plus existing current-authority/adapters. Real writer controls have a declared allowed mutation and separate no-partial-effects-before-release proof, not whole-DB-zero-change.
6. Keep initialValidateManagedRuntime/current-auth ahead of the no-work branch. Retain actual wrong/revokedbinding/changedpassword/noRedis and queued/claim/reclaim controls. Do not infer all idle requests valid from a cached boolean. No Go or test-list/TestMain executed by this research.

This exact new test inventory is **proposed**, not existing/source-derived/executed; actual nil cache fallbacks and SQL shape are source-derived. ROOT may execute one RED-only service command on its exact fresh permit with this6-name selector. A test setup error or real queued-work contention does not count as idle RED. Preserve genuine RED before any product scope choice.

### Existing controls to preserve / conditional verification

- `reclaim_check_cache_test.go`: NilFallsBackToEveryRuntime, NilRedisReturnsNil, RedisFailureFallsBackToEveryRuntime, CheckedHintIsDeferredAndRetriggers, CollectionCheckAlignsBackstops. Preserve nil/failure conservative fallback, due hints/MarkChecked/retry and complete-runtime backstop semantics.
- `empty_claim_cache_test.go`: NilSafe, NilRedisReturnsNil, BumpInvalidatesPriorMark, StaleMarkRejected. Queued negative caching must not suppress dispatched recovery. Redis-backed tests need explicit task-owned Redis isolation; their setup canPing/FlushDB, so do not execute against an inherited endpoint.
- `managed_runtime_test.go`: `TestManagedClaimCapturesBindingAndDoesNotBackfillReclaimedHistory`, PAT/forgednamespace/otherowner refusal and revokednamespace cannotdowngrade. Preserve actual credential minting/binding/generation/history and canonical authorization for real work.
- `collaboration_runtime_capabilities_read_concurrency_test.go` four canonical same-Q tests **4top/8named/7leaf** keep compatible read/read, real registration writer and current authority. No full116 rerun, new cache dependency or actual browser/Redis/run replay is required merely for this RED preparation.

## Files / patterns / related specs / caveats

JSON binds cache/router/main/harness/task/SQL source hashes, originalE03 capture, exact proposed inventory/probe shape and test/repair boundaries. Related specs: serverstage authority/current-account read versus actualwriter, security identity/minting, DB test isolation and fullscope verified source. Prior R30 writer/cancel chain is referenced without redoing it.

- Current-Q singleton55P03→target503 is proved by the captured frames; holder and exact release time remain unproved. QueryEnd/CommitEnd or later0locks cannot identify the old holder. This study does not wait for that unavailable historical proof before preparing a diagnostic RED.
- Repeated supported nil-cache idle reclaim currently does acquire the globaluserexclusive fence without returning any work. That is product fallback behavior amplified by harness cadence, not missingrouterinit. The **desired no-work coexistence contract is proposed and must pass real RED/real-writer/currentauth controls before repair is accepted**.
- R30 outcome2PASS1FAIL1NOT_RUN, original20-second QA assertion, Qscope, stopped0connections/epoch66/58settled/41complete and oldunknown/history remain ROOT's preserved records. E04notrun; R28 completion notreplayed. FullS2/A12,parent24,S3–S5 remain under existing gates.
- Only this new pair was written/sealed0444. No Go/SQL/DB/API/browser/runtime/process/signal/Git/dependency/config/test/product/spec/peer/old-report change or recursive agent execution.
