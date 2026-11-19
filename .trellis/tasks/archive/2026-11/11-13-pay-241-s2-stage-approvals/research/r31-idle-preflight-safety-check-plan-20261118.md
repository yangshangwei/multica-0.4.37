# Research: R31 idle preflight safety check plan

- Query: Minimal remaining verification for the conservative batch-only preflight before GREEN acceptance, with one useful new failure control and existing temporal/cache evidence.
- Scope: internal; frozen-source filesystem/static research only; exactly this NEW report pair.
- Date: 2026-11-18 (Asia/Shanghai).
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.
- Frozen GREEN: `/Volumes/artisan/.multica-acceptance/pay241-r31-idle-claim-20261118-bz7ym_r1/source-green-r01`.
- Machine plan: `r31-idle-preflight-safety-check-plan-20261118.json`; both reports sealed0444.

## Findings

**Add exactly one childless error control: `TestOrdinaryIdlePreflightFailureDoesNotAuthorizeClaim`.** The intended new file is `server/internal/service/task_idle_preflight_safety_test.go`; this research does not create code. Adding1/1/1 to the already defined18/33/28 main union gives **19 top /34 named /29 leaf** in both normal and race modes. Run the two existing Redis batch controls separately at **2/2/2** per mode and the existing temporal handler control at **1/1/1** with its own handler database/permit. No new predicate/time matrix, false-to-eligible race, dependency or broad suite is needed for the accepted next-due-poll semantics.

### Frozen source and generation state

| File | SHA256 read | State |
| --- | --- | --- |
| `server/internal/service/task.go` | `bcfb7399e75040f8eeb809cada849e11095ba1a105b9553d322d450397fdd7bc` | Frozen GREEN after ROOT notice; byte hash independently read. Generation/test extension graph remains ROOT-owned. |
| `server/pkg/db/queries/agent.sql` | `4a661a0c1374525177d4cd103b3d47aa39ea67c1a0a32c3eb4bf4067aa6494be` | Frozen GREEN after ROOT notice; byte hash independently read. Generation/test extension graph remains ROOT-owned. |
| `server/internal/service/task_idle_claim_read_concurrency_test.go` | `b07397c51e304ef51e95d3c3757143600a8a37528b96005e5641df9f765f300b` | Frozen GREEN after ROOT notice; byte hash independently read. Generation/test extension graph remains ROOT-owned. |
| `server/pkg/db/generated/agent.sql.go` | `2f6233d3d901d96c03f3964b69319932cf8246f65402c2123c1d9a5c2eb7c794` | Current generated artifact hash read after ROOT canonical generation; previous pre-generation GREEN read observed the unchanged old artifact 234283e657fcdb39c14bea05decb8005cf20e43b5bb63b29ba7f99a3347a42a5. This report did not execute the generator. |

At the first frozen GREEN read, generated `agent.sql.go` still had old SHA`234283e657fcdb39c14bea05decb8005cf20e43b5bb63b29ba7f99a3347a42a5` and lacked the new method; that state could not compile the new service call. ROOT subsequently ran actual cached sqlc1.31.1 generation and installed only `agent.sql.go` SHA`2f6233d3d901d96c03f3964b69319932cf8246f65402c2123c1d9a5c2eb7c794`, with142 other generated files reported identical. This report read the new hash and static Params/method signature, but did not run Go or re-execute ROOT generation. ROOT retains generator/build/proof receipts and final execution-graph binding. The new error test must also enter that final graph before main19/34/29 admission.

Accepted genuine RED receipt: `r31-idle-claim-red-independent-check-20261118.json` SHA`49f7686df12cb6463b7181eafd26fd52b4302ff80ed2483155f994f7fbdc6ae0`; two actual idle leaves FAIL, two real-reclaim leaves PASS, zero SKIP. Exact original control inventory remains in `r31-idle-claim-original-control-inventory-20261118.json` SHA`aa2dd4e25ab19fddd9ed65718b1da73bc6479cf2eec91270498764570580584f`; it is referenced without repeating its full inventory.

### Static source checks already resolved

- `task.go:3995` calls the new boolean query with **complete uniqueIDs** and the original DB recovery window. Its `%w` error return precedes the writer branch and `MarkChecked`.
- The initial runtime/current-source validation and promotion prefix is byte-identical. Reliable false skips only the reclaim transaction and continues the queued/cache/candidate/per-agent path.
- The original `runInTx → LockManagedRuntimes → ReclaimStaleDispatchedTasksForRuntimes → ValidateManagedTaskBinding` callback is byte-identical after removing the one added indentation level. Singular helpers, exclusive writer defaults and canonical positive SQL remain intact.
- The `MarkChecked` call and all subsequent queued behavior are byte-identical. Its implementation is unchanged; retry is **30 seconds**, backstop **90 seconds**, hint safety margin **5 seconds**.
- `agent.sql:1037` adds exactly one12-line `SELECT EXISTS` before the original batch UPDATE. It uses runtimeANY, dispatched status, startedNULL, strict DB-now recovery age and absent/expired lease; it returns only a boolean and has no authority join, `lock_issue_execution`, row lock or write.
- The current generated method has UUID-array and float64-window Params, returns `(bool,error)`, passes those arguments to QueryRow and scans the boolean. Actual compiled generated graph and ROOT142-sibling/source proof remain required.

### The one new error control

Reuse the real `r31Fixture`/`newStageCapabilitiesReadFixture` and `r31Snapshot`, preserving valid current QA/password/managed source, owner/binding/epoch, online runtime, positive capacity and explicitly nil caches. Establish the real R4 fixture first, prove all selected-runtime ordinary task rows absent, then take the no-effects baseline. This true-idle shape ensures initial promotion/retry cleanup has no owned rows to write.

Forward actual PostgreSQL DBTX methods. Only the exact new `HasPotentialDispatchedReclaimForRuntimes` QueryRow should return a `pgx.Row` whose Scan yields a unique sentinel error. The existing `agentClaimFailureRow` at `task_batch_claim_partial_test.go:54–56` is a reusable fault shape. Record the query marker/digest and full runtime/window arguments. Do not fabricate a false boolean, actor, authority result, other SQL outcome or cancellation. Invoke actual `TaskService.ClaimTasksForRuntimes` with the real daemon context.

Required assertions:

1. The exact preflight query is reached once after valid prerequisites; `errors.Is(err,sentinel)` proves the intended fault rather than an auth/setup error.
2. The returned task slice is empty and the error remains non-nil. A failed query cannot be accepted as successful empty polling.
3. No Step2 transaction/account/reclaim or downstream candidate/per-agent claim enters after the fault. Source ordering independently proves `MarkChecked` is not reached.
4. The post-fixture canonical snapshot remains unchanged, including task/issue/user/credential/binding/installation/token/generation/lease and original evidence relations; no claim, mint or event effect.
5. The normal/race named set is exactly this one childless top. Setup failures, maxTasks0, invalid runtime, revoked auth or a skipped DB setup do not count as this control.

This test protects the service error/authority/effect boundary at a new I/O failure point. It would catch an error swallowed as false, true or empty success, and any continuation into writer/claim behavior. It does not mirror the new SQL predicate matrix.

Zero effects apply to the **true-idle post-fixture baseline**. Deferred promotion and superseded-retry cleanup precede the probe in the original algorithm; do not invent rollback/atomicity for legitimate earlier committed writes in a different fixture. With nil cache, the test also does not directly inspect Redis mutation: the frozen source error-before-MarkChecked check supplies that ordering, and the two separate original cache controls supply scheduling integration.

Implementation handoff owns **only the new safety test file**, after ROOT dispatch. It must preserve the b073 regression file and other owners’ task/SQL/generated/source edits, reuse existing helpers/patterns, and add no dependency, config or spec change.

### Existing controls fill the remaining behavior

| Control | Count | What it establishes | Limit |
| --- | --- | --- | --- |
| Main original16 plus unchanged idle/reclaim2 |18/33/28 before the one error top | Actual queued/deferred/partial-prefix/capacity/source/admission behavior, true-idle coexistence and actual exclusive reclaim generation/lease/history | Require actual GREEN normal/race PASS; prior RED is preserved. |
| `TestClaimTasksForRuntimes_AnyDueRuntimeChecksTheCompleteSet` (`task_batch_claim_test.go:173`) |1/1/1 | rt1 is due while a stale dispatched task sits on non-due rt2; actual batch call must recover it across the complete set | Requires dedicated Redis; do not substitute dueKeys for uniqueIDs. |
| `TestClaimTasksForRuntimes_EmptyReclaimDefersHintForRetry` (`task_batch_claim_test.go:232`) |1/1/1 | Potential-positive stale task plus stale heartbeat gives canonical0; retained hint defers to the30-second retry and becomes due again | It checks scheduling, not a subsequent actual reclaim or the new false+concurrent-eligibility branch. |
| `TestManagedBatchClaimLostResponseRecovery` (`managed_claim_recovery_test.go:12`) |1/1/1 | Real managed HTTP first claim; young dispatch/expired lease refuses; stale dispatch/live lease refuses; stale expired lease recovers with generation+1 and retained binding/epoch/admission; immediate duplicates/started task refuse | Uses fixture DB timestamps and repeated actual polls; separate owned handler package DB needed. |

No separate false-to-eligible race is proposed. A reliable negative is a fact at that read snapshot; a later row/clock change can wait for the next normal due poll under the accepted contract. The existing handler already changes a previously negative dispatch/lease condition and later recovers the same task through the real production path. Nil cache remains due every poll. Redis source keeps all-runtime MarkChecked, future-hint retention, bounded due retries, TTL/backstop and failure-open behavior; the two original batch cache tests check complete-set and empty-canonical retry behavior. Canonical priority/order/LIMIT remains unchanged, so this repair claims no new fairness or elapsed-time SLA.

### ROOT run lanes and exact counts

| Lane | Package | Expected top/named/leaf per mode | Isolation |
| --- | --- | --- | --- |
| Main including the one new error control | `./internal/service` | **19/34/29**, normal and race | Same ROOT-authorized fresh main service source/profile, with separately recorded mode DB/permit and final generated/new-test graph. |
| Two original cache controls | `./internal/service` | **2/2/2**, normal and race | Separate fresh service package DB/profile; only this test process receives owned Redis configuration. Serialize Redis owner. |
| Existing temporal HTTP control | `./internal/handler` | **1/1/1**, normal | Own fresh handler DB/permit; TestMain still initializes fixtures under the exact selector. Additional race only if ROOT existing gate requires it. |

Proposed main selector (contains no slash):

```text
^(TestClaimTasksForRuntimes_EmptyInputs|TestClaimTasksForRuntimes_MaxTasksCap|TestClaimTasksForRuntimes_MultiRuntimeDrain|TestClaimTasksForRuntimes_PartialSuccessOnCandidateQueryFailureAfterReclaim|TestClaimTasksForRuntimes_PartialSuccessOnSecondAgentClaimFailure|TestClaimTasksForRuntimes_PromotesDeferredAndEmitsQueuedEvent|TestEmptyClaimCache_NilSafe|TestManagedAdmissionReclaimsOnlyPreviouslyAdmittedFence|TestManagedClaimCapturesBindingAndDoesNotBackfillReclaimedHistory|TestManagedRuntimeRejectsPATForgedNamespaceAndOtherOwner|TestManagedRuntimeRevokedNamespaceCannotDowngradeToLegacy|TestNewEmptyClaimCache_NilRedisReturnsNil|TestNewReclaimCheckCache_NilRedisReturnsNil|TestOrdinaryClaimsWaitForCollaborationNamespaceProof|TestOrdinaryIdleClaimAndR4Coexistence|TestOrdinaryIdlePreflightFailureDoesNotAuthorizeClaim|TestOrdinaryReclaimPreservesAuthorityFences|TestReclaimCheckCache_NilFallsBackToEveryRuntime|TestReclaimCheckCache_RedisFailureFallsBackToEveryRuntime)$
```

Proposed cache selector:

```text
^(TestClaimTasksForRuntimes_AnyDueRuntimeChecksTheCompleteSet|TestClaimTasksForRuntimes_EmptyReclaimDefersHintForRetry)$
```

Proposed temporal handler selector:

```text
^TestManagedBatchClaimLostResponseRecovery$
```

The JSON contains the complete34-name and29-leaf main sets. Counts are planned until ROOT authors/reviews/freezes the one childless test and records actual terminals. Require every expected name PASS, zero FAIL/SKIP/missing/unfinished names, and matching service normal/race sets. Source/profile/resource and final generation bindings remain independent acceptance prerequisites. No full116/handler99/router54, new router execution, native/browser replay or broad suite is proposed.

ROOT reports dedicated existing-cache Redis7-alpine container `pay241-r31-claim-cache-bz7ym-r1` at `127.0.0.1:51802`, logicalDB12, owned labels/PING proof and allocation receipt prefix2d9dea13. This is ROOT attestation; research made no Redis connection. The helper forcesDB12 and FlushDB at setup/cleanup. Use an exact cache-lane process env; do not export a global endpoint or flush ambient Redis.

### Generation checks without test-only SQL evidence

ROOT must retain actual sqlc1.31.1 generator/build/generation receipts, verify the installed generated bool method’s UUID-array/window positional mapping and Scan/error propagation, preserve the canonical query/default/function hashes and142 unchanged generated siblings, then compile/static-check the admitted graph with pinned tools. Freeze/bind the final graph including the new error file before updating main permits. A copied SQL string, fake boolean, successful test filter or old pre-generation source receipt is insufficient. Research performed no Go/AST/test-list/TestMain/SQL/DB execution.

### Files found, patterns and references

- `server/internal/service/task.go`: Frozen GREEN after ROOT notice; byte hash independently read. Generation/test extension graph remains ROOT-owned.
- `server/pkg/db/queries/agent.sql`: Frozen GREEN after ROOT notice; byte hash independently read. Generation/test extension graph remains ROOT-owned.
- `server/internal/service/task_idle_claim_read_concurrency_test.go`: Frozen GREEN after ROOT notice; byte hash independently read. Generation/test extension graph remains ROOT-owned.
- `server/pkg/db/generated/agent.sql.go`: Current generated artifact hash read after ROOT canonical generation; previous pre-generation GREEN read observed the unchanged old artifact 234283e657fcdb39c14bea05decb8005cf20e43b5bb63b29ba7f99a3347a42a5. This report did not execute the generator.
- `server/internal/service/task_idle_claim_read_concurrency_test.go:163` (`r31Fixture`): existing real current-Q fixture, no-effects snapshot or canonical reclaim witness to reuse; definition SHA`f8c965aeaf6a89117c40269997e3de7338cd6633d550fa1d079047df299f80f5`.
- `server/internal/service/task_idle_claim_read_concurrency_test.go:178` (`r31Snapshot`): existing real current-Q fixture, no-effects snapshot or canonical reclaim witness to reuse; definition SHA`05d217268af868fba515c583d07770c8174818640de9e00b052561e95f0c4cfb`.
- `server/internal/service/task_idle_claim_read_concurrency_test.go:189` (`r31NoWrites`): existing real current-Q fixture, no-effects snapshot or canonical reclaim witness to reuse; definition SHA`ad2a7db93736b43e97a4969d9a53a2e35c1f970b804e727ff9765568542fc879`.
- `server/internal/service/task_idle_claim_read_concurrency_test.go:331` (`r31WantReclaimed`): existing real current-Q fixture, no-effects snapshot or canonical reclaim witness to reuse; definition SHA`750d770f93dc3597cb168938240e9919b65f9cbe61d5868db4c26a7ea33c33ec`.
- `server/internal/service/collaboration_runtime_capabilities_read_concurrency_test.go:27` (`newStageCapabilitiesReadFixture`): existing real current-Q fixture, no-effects snapshot or canonical reclaim witness to reuse; definition SHA`bf4492d643ff3be179f7eab3fb2d3e1f7a587be53fa25b65a444203eaf766b76`.
- `server/internal/service/task_batch_claim_partial_test.go:54`: pgx.Row Scan-error forwarding shape; existing partial-prefix faults retain real committed behavior.
- `server/internal/service/reclaim_check_cache.go:271`: original unchanged MarkChecked; all-runtime scheduled hint deferral/backstop.
- `server/internal/service/task_batch_claim_test.go:173,232` and `server/internal/handler/managed_claim_recovery_test.go:12`: existing full control hashes remain in the original inventory and this JSON’s selected source-shape references.
- First-party documentation entry points: [sqlc](https://docs.sqlc.dev/en/stable/), [pgx](https://github.com/jackc/pgx), [go-redis](https://github.com/redis/go-redis). Locally observed versions: sqlc1.31.1, pgxv5.9.2 and go-redisv9.18.0. No external fetch or new dependency occurred.

### Related specs

- `.trellis/workflow.md`: Persist research; role write isolation and parent-owned execution.
- `.trellis/spec/server/managed-installations.md`: Current source/owner/binding/admission,90-second recovery/lease and immutable delivery generation.
- `.trellis/spec/server/security-boundaries.md`: Original source account version, password/mint fence and unavailable-versus-invalid outcomes.
- `.trellis/spec/server/collaboration-stage-approvals.md`: Same-Q R4 read authority and real writer defaults; original raw/binding reuse.
- `.trellis/spec/server/database-test-isolation.md`: Fresh task-owned package DB, TestMain side effects, no SKIP acceptance and source/profile ownership.
- `.trellis/spec/server/verification-environment.md`: Final generated/new-test source graph, exact package/test permits, process env and JSON named inventory.

## Caveats / Not Found

- One proposed error-control name/file is not an existing test until ROOT author/review/freeze. Counts19/34/29 are the exact planned denominator, conditional on that childless implementation.
- Initial GREEN inspection observed old generated source; ROOT later generated agent.sql.go2f6233... and its hash was read at sealing. Final execution graph/142 sibling generation proof remains separate ROOT evidence.
- Source-correct error handling is present, but generated Scan failure behavior is not yet executed by this research. One meaningful DBTX fault control supplies that new failure boundary.
- Zero effects apply to the true-idle post-fixture baseline. Earlier legitimate deferred-promotion/cancel writes in other calls are not rolled back by the preflight error policy.
- The nil-cache error test cannot directly assert Redis keys unchanged; source return-before-MarkChecked is independently checked. No extra Redis error test or predicate/time/race matrix is proposed.
- Exact temporal handler execution needs its own fresh handler DB/permit because TestMain runs; source/service allocation alone grants no handler/router or full116 test execution.
- All source reads/checks were static. No product/test/spec/config edits, Go/DB/Redis/runtime/signals/Git/dependency or recursive-agent operation occurred.
- R30 original four remains2PASS/1FAIL/1NOT_RUN,retry0; the historical native holder remains unknown. This plan does not accept GREEN/full S2 or redo diagnosis, browser/completion replay or deadlines.
