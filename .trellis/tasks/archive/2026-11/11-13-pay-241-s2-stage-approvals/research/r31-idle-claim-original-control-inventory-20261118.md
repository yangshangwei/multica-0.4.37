# Research: R31 idle-claim original control inventory

- Query: Exact existing test names and source hashes for a possible batch-only necessary-condition EXISTS that skips empty Step2 reclaim while preserving all real-claim fences.
- Scope: internal; frozen-source filesystem/static research only.
- Date: 2026-11-18 (Asia/Shanghai).
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.
- Frozen source: `/Volumes/artisan/.multica-acceptance/pay241-r31-idle-claim-20261118-bz7ym_r1/source-red-r01`.
- Detailed machine inventory: `r31-idle-claim-original-control-inventory-20261118.json`; final report pair mode0444.

## Findings

**Recommend a no-external-Redis original GREEN union of 16 top-level / 27 named / 24 leaf tests in `./internal/service`, in both normal and race modes.** These are exact existing source names, not execution results. The union includes actual batch queued/reclaim/deferred/partial-prefix controls, the complete pertinent namespace-capacity top, current managed source/history controls, five nil/closed-client cache controls, and the batch-capable stopped-admission recovery top. ROOT owns any subsequent execution, source/profile/resource admission and GREEN decision.

`task_batch_claim_test.go` has **five** childless top-level tests. Two require external Redis, leaving three DB-only tests in that file; adding the two partial-prefix tests and the deferred test gives **6/6/6** DB-only batch originals. No sixth main-file test exists in this frozen source.

### Primary original union and count proof

| Group | Top | Named | Leaf | Reason |
| --- | ---: | ---: | ---: | --- |
| DB-only original batch claims | 6 | 6 | 6 | Primary for batch Step2-only gating; preserves queued work, complete returned prefix, cap, deferred side effects and original trivial short circuits. |
| Pertinent namespace capacity top | 1 | 5 | 4 | Use the complete top to retain the actual batch leaf plus agent/runtime/SQL siblings without slash-selector ambiguity. |
| Current managed source and historical claim provenance | 3 | 8 | 7 | Primary current-source guard supplement. The claim/history top and revoked-namespace claim leaf are singular controls, not direct batch coverage. |
| Batch-capable previously admitted recovery | 1 | 3 | 2 | Primary because the optimized step is reclaim: true directly exercises batch recovery under stopped admission and missing historical proof; false retains the singular sibling. |
| Nil and closed-client cache originals | 5 | 5 | 5 | Primary five childless tests; none calls newRedisTestClient. RedisFailureFallsBackToEveryRuntime uses a closed local client, not a service. |
| **Primary original union** | **16** | **27** | **24** | No external Redis calls. |

The smaller base without `TestManagedAdmissionReclaimsOnlyPreviouslyAdmittedFence` is **15/24/22**. Include that whole top in the preferred union: its `/true` leaf performs real batch reclaim while installation admission is stopped; `/false` is the unchanged singular sibling. Recovering proven admission and refusing unknown admission are part of the optimized Step2 contract.

Use complete top-level names to avoid a mixed slash selector accidentally filtering childless tests or inventing intermediate named events. Exact proposed selector (no trailing newline in its SHA256):

```text
^(TestClaimTasksForRuntimes_EmptyInputs|TestClaimTasksForRuntimes_MaxTasksCap|TestClaimTasksForRuntimes_MultiRuntimeDrain|TestClaimTasksForRuntimes_PartialSuccessOnCandidateQueryFailureAfterReclaim|TestClaimTasksForRuntimes_PartialSuccessOnSecondAgentClaimFailure|TestClaimTasksForRuntimes_PromotesDeferredAndEmitsQueuedEvent|TestEmptyClaimCache_NilSafe|TestManagedAdmissionReclaimsOnlyPreviouslyAdmittedFence|TestManagedClaimCapturesBindingAndDoesNotBackfillReclaimedHistory|TestManagedRuntimeRejectsPATForgedNamespaceAndOtherOwner|TestManagedRuntimeRevokedNamespaceCannotDowngradeToLegacy|TestNewEmptyClaimCache_NilRedisReturnsNil|TestNewReclaimCheckCache_NilRedisReturnsNil|TestOrdinaryClaimsWaitForCollaborationNamespaceProof|TestReclaimCheckCache_NilFallsBackToEveryRuntime|TestReclaimCheckCache_RedisFailureFallsBackToEveryRuntime)$
```

- Selector SHA256: `bd78f7b603a44d657180e491d2fc21786c9d46bfa644b41151fd9bbda20f220d`.
- ROOT normal proposed argv: `go test ./internal/service -run <exact selector above> -count=1 -json`.
- ROOT race proposed argv: `go test -race ./internal/service -run <exact selector above> -count=1 -json`.
- These argv are proposals for the separately admitted R31 service permit, not commands executed by this research or permission for other packages.
- Require exact expected named/leaf sets, zero FAIL/SKIP/unfinished identities, and identical normal/race sets. Legitimate build events do not enter named-test denominators.

Complete expected **27-name** primary set:

```text
TestClaimTasksForRuntimes_EmptyInputs
TestClaimTasksForRuntimes_MaxTasksCap
TestClaimTasksForRuntimes_MultiRuntimeDrain
TestClaimTasksForRuntimes_PartialSuccessOnCandidateQueryFailureAfterReclaim
TestClaimTasksForRuntimes_PartialSuccessOnSecondAgentClaimFailure
TestClaimTasksForRuntimes_PromotesDeferredAndEmitsQueuedEvent
TestEmptyClaimCache_NilSafe
TestManagedAdmissionReclaimsOnlyPreviouslyAdmittedFence
TestManagedAdmissionReclaimsOnlyPreviouslyAdmittedFence/false
TestManagedAdmissionReclaimsOnlyPreviouslyAdmittedFence/true
TestManagedClaimCapturesBindingAndDoesNotBackfillReclaimedHistory
TestManagedRuntimeRejectsPATForgedNamespaceAndOtherOwner
TestManagedRuntimeRevokedNamespaceCannotDowngradeToLegacy
TestManagedRuntimeRevokedNamespaceCannotDowngradeToLegacy/claim
TestManagedRuntimeRevokedNamespaceCannotDowngradeToLegacy/legacy_context
TestManagedRuntimeRevokedNamespaceCannotDowngradeToLegacy/owner_PAT
TestManagedRuntimeRevokedNamespaceCannotDowngradeToLegacy/registration
TestManagedRuntimeRevokedNamespaceCannotDowngradeToLegacy/unbound_daemon_credential
TestNewEmptyClaimCache_NilRedisReturnsNil
TestNewReclaimCheckCache_NilRedisReturnsNil
TestOrdinaryClaimsWaitForCollaborationNamespaceProof
TestOrdinaryClaimsWaitForCollaborationNamespaceProof/agent
TestOrdinaryClaimsWaitForCollaborationNamespaceProof/batch
TestOrdinaryClaimsWaitForCollaborationNamespaceProof/runtime
TestOrdinaryClaimsWaitForCollaborationNamespaceProof/sql
TestReclaimCheckCache_NilFallsBackToEveryRuntime
TestReclaimCheckCache_RedisFailureFallsBackToEveryRuntime
```

The **24-leaf** set is this same set excluding the three explicit subtest-parent tops: `TestOrdinaryClaimsWaitForCollaborationNamespaceProof`, `TestManagedRuntimeRevokedNamespaceCannotDowngradeToLegacy`, and `TestManagedAdmissionReclaimsOnlyPreviouslyAdmittedFence`. Childless tops are leaves. The JSON contains both complete sets.

### Exact bounded original inventory

Every table below records existing source-derived names and line numbers. Child names are appended to the stated top with `/`; whitespace becomes `_` as shown. Maps have nondeterministic execution order but the same named set. Slash-bearing names from a single `t.Run` create one leaf, with no phantom intermediate parent. Complete function-definition hashes and expanded named/leaf arrays are in the JSON.

#### DB-only original batch claims — 6/6/6

Primary. Primary for batch Step2-only gating; preserves queued work, complete returned prefix, cap, deferred side effects and original trivial short circuits.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestClaimTasksForRuntimes_MultiRuntimeDrain` | `server/internal/service/task_batch_claim_test.go:102` | Childless: top is its leaf. | No |
| `TestClaimTasksForRuntimes_MaxTasksCap` | `server/internal/service/task_batch_claim_test.go:151` | Childless: top is its leaf. | No |
| `TestClaimTasksForRuntimes_EmptyInputs` | `server/internal/service/task_batch_claim_test.go:303` | Childless: top is its leaf. | No |
| `TestClaimTasksForRuntimes_PartialSuccessOnSecondAgentClaimFailure` | `server/internal/service/task_batch_claim_partial_test.go:83` | Childless: top is its leaf. | No |
| `TestClaimTasksForRuntimes_PartialSuccessOnCandidateQueryFailureAfterReclaim` | `server/internal/service/task_batch_claim_partial_test.go:115` | Childless: top is its leaf. | No |
| `TestClaimTasksForRuntimes_PromotesDeferredAndEmitsQueuedEvent` | `server/internal/service/task_batch_claim_deferred_test.go:28` | Childless: top is its leaf. | No |

#### Redis-backed batch reclaim schedule controls — 2/2/2

Conditional/source-retained. Conditional fresh exact service permit when the repair changes due-hint/MarkChecked/backstop semantics or ROOT chooses broader cache integration proof; dedicated task-owned Redis required.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestClaimTasksForRuntimes_AnyDueRuntimeChecksTheCompleteSet` | `server/internal/service/task_batch_claim_test.go:173` | Childless: top is its leaf. | Yes |
| `TestClaimTasksForRuntimes_EmptyReclaimDefersHintForRetry` | `server/internal/service/task_batch_claim_test.go:232` | Childless: top is its leaf. | Yes |

#### Pertinent namespace capacity top — 1/5/4

Primary. Use the complete top to retain the actual batch leaf plus agent/runtime/SQL siblings without slash-selector ambiguity.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestOrdinaryClaimsWaitForCollaborationNamespaceProof` | `server/internal/service/task_collaboration_capacity_test.go:75` | `agent`, `runtime`, `batch`, `sql` | No |

#### Other capacity originals — 3/3/3

Conditional/source-retained. Source-retained for a batch-only negative precheck. Fresh if shared claim, namespace locking or reservation/capacity queries change.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestOrdinaryCapacityCountsRetainedCollaborationReservation` | `server/internal/service/task_collaboration_capacity_test.go:61` | Childless: top is its leaf. | No |
| `TestOrdinaryClaimIgnoresAnotherCollaborationNamespace` | `server/internal/service/task_collaboration_capacity_test.go:122` | Childless: top is its leaf. | No |
| `TestOrdinaryClaimRechecksReservationAfterNamespaceLock` | `server/internal/service/task_collaboration_capacity_test.go:131` | Childless: top is its leaf. | No |

#### Current managed source and historical claim provenance — 3/8/7

Primary. Primary current-source guard supplement. The claim/history top and revoked-namespace claim leaf are singular controls, not direct batch coverage.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestManagedRuntimeRejectsPATForgedNamespaceAndOtherOwner` | `server/internal/service/managed_runtime_test.go:49` | Childless: top is its leaf. | No |
| `TestManagedRuntimeRevokedNamespaceCannotDowngradeToLegacy` | `server/internal/service/managed_runtime_test.go:103` | `owner_PAT`, `unbound_daemon_credential`, `legacy_context`, `registration`, `claim` | No |
| `TestManagedClaimCapturesBindingAndDoesNotBackfillReclaimedHistory` | `server/internal/service/managed_runtime_test.go:165` | Childless: top is its leaf. | No |

#### Legacy validation and first-bind registration originals — 2/2/2

Conditional/source-retained. Source-retained unless validation/default writer/registration scope changes; then activate exact original tests.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestManagedRuntimeAllowsUnboundLegacyNamespace` | `server/internal/service/managed_runtime_test.go:86` | Childless: top is its leaf. | No |
| `TestManagedRegistrationCannotCrossFirstBindWithLegacyPAT` | `server/internal/service/managed_runtime_test.go:240` | Childless: top is its leaf. | No |

#### Batch-capable previously admitted recovery — 1/3/2

Primary. Primary because the optimized step is reclaim: true directly exercises batch recovery under stopped admission and missing historical proof; false retains the singular sibling.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestManagedAdmissionReclaimsOnlyPreviouslyAdmittedFence` | `server/internal/service/managed_admission_test.go:197` | `false`, `true` | No |

#### Other admission and stale-delivery originals — 4/8/7

Conditional/source-retained. Source-retained unless shared admission, generation, finalization, comment receipt, requeue or canonical SQL changes. No whole admission package is implied.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestManagedAdmissionStopsEveryClaimPrimitive` | `server/internal/service/managed_admission_test.go:22` | `agent`, `runtime`, `batch`, `sql` | No |
| `TestManagedAdmissionStopSerializesBeforeClaim` | `server/internal/service/managed_admission_test.go:90` | Childless: top is its leaf. | No |
| `TestManagedAdmissionOperationCASReplayAndAuditRollback` | `server/internal/service/managed_admission_test.go:146` | Childless: top is its leaf. | No |
| `TestManagedReclaimRejectsEarlierDeliveryRollback` | `server/internal/service/managed_admission_test.go:250` | Childless: top is its leaf. | No |

#### Nil and closed-client cache originals — 5/5/5

Primary. Primary five childless tests; none calls newRedisTestClient. RedisFailureFallsBackToEveryRuntime uses a closed local client, not a service.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestReclaimCheckCache_NilFallsBackToEveryRuntime` | `server/internal/service/reclaim_check_cache_test.go:13` | Childless: top is its leaf. | No |
| `TestNewReclaimCheckCache_NilRedisReturnsNil` | `server/internal/service/reclaim_check_cache_test.go:26` | Childless: top is its leaf. | No |
| `TestReclaimCheckCache_RedisFailureFallsBackToEveryRuntime` | `server/internal/service/reclaim_check_cache_test.go:32` | Childless: top is its leaf. | No |
| `TestEmptyClaimCache_NilSafe` | `server/internal/service/empty_claim_cache_test.go:43` | Childless: top is its leaf. | No |
| `TestNewEmptyClaimCache_NilRedisReturnsNil` | `server/internal/service/empty_claim_cache_test.go:57` | Childless: top is its leaf. | No |

#### External Redis cache originals — 12/12/12

Conditional/source-retained. Conditional cache helper/scheduling/version/TTL/invalidation changes; activate only on a dedicated task-owned Redis instance, DB12, with one owner.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestReclaimCheckCache_ProductionWindowHintOutlivesDeadline` | `server/internal/service/reclaim_check_cache_test.go:45` | Childless: top is its leaf. | Yes |
| `TestReclaimCheckCache_CheckedHintIsDeferredAndRetriggers` | `server/internal/service/reclaim_check_cache_test.go:79` | Childless: top is its leaf. | Yes |
| `TestReclaimCheckCache_TrackLaterAndForget` | `server/internal/service/reclaim_check_cache_test.go:132` | Childless: top is its leaf. | Yes |
| `TestReclaimCheckCache_CollectionCheckAlignsBackstops` | `server/internal/service/reclaim_check_cache_test.go:168` | Childless: top is its leaf. | Yes |
| `TestReclaimCheckCache_DueRuntimeIDsPreservesInputOrder` | `server/internal/service/reclaim_check_cache_test.go:202` | Childless: top is its leaf. | Yes |
| `TestReclaimCheckCache_MarkCheckedBatchLimitFailsOpen` | `server/internal/service/reclaim_check_cache_test.go:223` | Childless: top is its leaf. | Yes |
| `TestEmptyClaimCache_EmptyRuntimeIDIsNoOp` | `server/internal/service/empty_claim_cache_test.go:63` | Childless: top is its leaf. | Yes |
| `TestEmptyClaimCache_MarkAndIsEmptyVersionMatched` | `server/internal/service/empty_claim_cache_test.go:75` | Childless: top is its leaf. | Yes |
| `TestEmptyClaimCache_BumpInvalidatesPriorMark` | `server/internal/service/empty_claim_cache_test.go:94` | Childless: top is its leaf. | Yes |
| `TestEmptyClaimCache_StaleMarkRejected` | `server/internal/service/empty_claim_cache_test.go:115` | Childless: top is its leaf. | Yes |
| `TestEmptyClaimCache_TTL` | `server/internal/service/empty_claim_cache_test.go:134` | Childless: top is its leaf. | Yes |
| `TestEmptyClaimCache_RuntimeIsolation` | `server/internal/service/empty_claim_cache_test.go:149` | Childless: top is its leaf. | Yes |

#### No-external-Redis notification control — 1/1/1

Conditional/source-retained. Conditional notification/wakeup/tracking scope only; not needed for an untouched notification path.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestNotifyTasksFinished_CoalescesByRuntime` | `server/internal/service/task_notify_test.go:140` | Childless: top is its leaf. | No |

#### External Redis notification originals — 4/4/4

Conditional/source-retained. Conditional enqueue/finish/invalidation/wakeup changes; requires dedicated task-owned Redis DB12.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestNotifyTaskAvailable_BumpsBeforeWakeup` | `server/internal/service/task_notify_test.go:31` | Childless: top is its leaf. | Yes |
| `TestNotifyTaskAvailable_InvalidWithoutRuntimeIsNoOp` | `server/internal/service/task_notify_test.go:78` | Childless: top is its leaf. | Yes |
| `TestNotifyTaskFinished_BumpsBeforeRuntimeWakeup` | `server/internal/service/task_notify_test.go:109` | Childless: top is its leaf. | Yes |
| `TestNotifyTaskEnqueued_UnwiredServiceCannotInvalidateClaimVerdict` | `server/internal/service/task_notify_test.go:182` | Childless: top is its leaf. | Yes |

#### Direct batch handler adapter originals — 13/13/13

Conditional/source-retained. Conditional exact handler permit for HTTP signature/body/auth/wiring/finalization/receipt changes. Unchanged adapter source can retain prior binding; no fresh handler99 is implied.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestClaimTasksByRuntime_RoutesAcrossRuntimesAndMintsTokens` | `server/internal/handler/daemon_batch_claim_test.go:62` | Childless: top is its leaf. | No |
| `TestClaimTasksByRuntime_IncludesActiveSiblingRun` | `server/internal/handler/daemon_batch_claim_test.go:98` | Childless: top is its leaf. | No |
| `TestClaimTasksByRuntime_SkipsCrossWorkspaceRuntime` | `server/internal/handler/daemon_batch_claim_test.go:159` | Childless: top is its leaf. | No |
| `TestClaimTasksByRuntime_CancelsTaskWhenRuntimeOwnerMissing` | `server/internal/handler/daemon_batch_claim_test.go:224` | Childless: top is its leaf. | No |
| `TestFailClaimedTaskBeforeLaunchSettlesDispatchedTask` | `server/internal/handler/daemon_batch_claim_test.go:266` | Childless: top is its leaf. | No |
| `TestClaimTasksByRuntime_MaxTasksZeroClaimsNothing` | `server/internal/handler/daemon_batch_claim_finalize_test.go:24` | Childless: top is its leaf. | No |
| `TestClaimTasksByRuntime_MaxTasksNegativeIsBadRequest` | `server/internal/handler/daemon_batch_claim_finalize_test.go:55` | Childless: top is its leaf. | No |
| `TestClaimTasksByRuntime_SkipsInvalidRuntimeID` | `server/internal/handler/daemon_batch_claim_finalize_test.go:69` | Childless: top is its leaf. | No |
| `TestClaimTasksByRuntime_PersistsCommentDeliveryReceipt` | `server/internal/handler/daemon_batch_claim_finalize_test.go:134` | Childless: top is its leaf. | No |
| `TestClaimTasksByRuntime_StaleReclaimRecordsDeliveryReceipt` | `server/internal/handler/daemon_batch_claim_finalize_test.go:170` | Childless: top is its leaf. | No |
| `TestClaimTasksByRuntime_SkipsRuntimeOwnedByAnotherDaemon` | `server/internal/handler/daemon_batch_claim_finalize_test.go:217` | Childless: top is its leaf. | No |
| `TestClaimTasksByRuntime_RequiresDaemonID` | `server/internal/handler/daemon_batch_claim_finalize_test.go:261` | Childless: top is its leaf. | No |
| `TestClaimTasksByRuntime_RepairsStaleCommentPlan` | `server/internal/handler/daemon_batch_claim_finalize_test.go:280` | Childless: top is its leaf. | No |

#### Managed lost-response time and lease control — 1/1/1

Conditional/source-retained. Strong exact handler supplement if ROOT needs original production-path temporal eligibility proof for the new probe or changes recovery/finalization. No Redis required, but handler TestMain still needs a task-owned DB.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestManagedBatchClaimLostResponseRecovery` | `server/internal/handler/managed_claim_recovery_test.go:12` | Childless: top is its leaf. | No |

#### Password reset versus claim finalization — 1/1/1

Conditional/source-retained. Conditional exact handler permit if finalization, minting or current password-source behavior changes.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestPasswordBlockedTaskAndDaemonMintRejectsReset` | `server/internal/handler/password_claim_test.go:16` | Childless: top is its leaf. | No |

#### Managed router identity and recovery control — 1/4/3

Conditional/source-retained. Conditional router permit if route/auth/cache wiring changes; this top is not direct batch router coverage. No fresh router54 is implied.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestManagedInstallationRouterEnrollmentBindingRenewalAndRecovery` | `server/cmd/server/managed_installation_test.go:31` | `desktop`, `web`, `forged_declared_id` | No |

#### Canonical ordinary source/admission tops — 2/75/73

Conditional/source-retained. Source-retained for a conservative Step2-only probe. Fresh exact tops if ordinary source/triage/lock_issue_execution/candidate/preparation boundaries change; do not count slash-only intermediate labels as named tests.

| Existing top | Source | Exact child suffixes (or childless) | External Redis |
| --- | --- | --- | --- |
| `TestLegacyExecutionSQLBoundaryAndOrdinaryChild` | `server/internal/service/legacy_execution_boundary_test.go:139` | `collaboration/insert`, `collaboration/deferred`, `collaboration/claim`, `collaboration/runtime`, `collaboration/batch`, `collaboration/start`, `collaboration/sql_start`, `collaboration/retry`, `collaboration/promote`, `collaboration/reclaim`, `collaboration/link`, `ordinary/insert`, `ordinary/deferred`, `ordinary/claim`, `ordinary/runtime`, `ordinary/batch`, `ordinary/start`, `ordinary/sql_start`, `ordinary/retry`, `ordinary/promote`, `ordinary/reclaim`, `ordinary/link`, `ordinary_child/insert`, `ordinary_child/deferred`, `ordinary_child/claim`, `ordinary_child/runtime`, `ordinary_child/batch`, `ordinary_child/start`, `ordinary_child/sql_start`, `ordinary_child/retry`, `ordinary_child/promote`, `ordinary_child/reclaim`, `ordinary_child/link` | No |
| `TestTriageAdmissionEnqueueAndRuntimeBoundaries` | `server/internal/service/triage_admission_test.go:34` | `pending/enqueue`, `pending/claim`, `pending/runtime`, `pending/batch`, `pending/start`, `pending/retry`, `pending/promote`, `pending/rerun`, `rejected/enqueue`, `rejected/claim`, `rejected/runtime`, `rejected/batch`, `rejected/start`, `rejected/retry`, `rejected/promote`, `rejected/rerun`, `duplicate/enqueue`, `duplicate/claim`, `duplicate/runtime`, `duplicate/batch`, `duplicate/start`, `duplicate/retry`, `duplicate/promote`, `duplicate/rerun`, `not_required/enqueue`, `not_required/claim`, `not_required/runtime`, `not_required/batch`, `not_required/start`, `not_required/retry`, `not_required/promote`, `not_required/rerun`, `accepted/enqueue`, `accepted/claim`, `accepted/runtime`, `accepted/batch`, `accepted/start`, `accepted/retry`, `accepted/promote`, `accepted/rerun` | No |

The legacy top expands exactly `collaboration`, `ordinary`, `ordinary_child` × `insert`, `deferred`, `claim`, `runtime`, `batch`, `start`, `sql_start`, `retry`, `promote`, `reclaim`, `link` =33 slash-bearing leaves. The triage top expands exactly `pending`, `rejected`, `duplicate`, `not_required`, `accepted` × `enqueue`, `claim`, `runtime`, `batch`, `start`, `retry`, `promote`, `rerun` =40 slash-bearing leaves. JSON lists all73 full leaf names. Source file siblings outside these two relevant tops were not expanded or proposed as fresh controls.

### Redis isolation and honest dependency classification

`newRedisTestClient` at `server/internal/service/empty_claim_cache_test.go:17–40` reads `REDIS_TEST_URL`, forces **DB12**, Pings, Flushes the DB before use and Flushes/Closes it at cleanup. Unset/unreachable endpoints SKIP. It is an external client, not miniredis. Conditional Redis controls require a dedicated task-owned Redis instance with one test owner; never publish a global endpoint or flush foreign Redis. ROOT reports system Redis tools unavailable and no dependency installation authorized; this inventory does not need those tools.

`TestReclaimCheckCache_RedisFailureFallsBackToEveryRuntime:32–43` instead creates `127.0.0.1:0`, immediately closes it, then checks fallback. It never calls the external helper and belongs in the five-test no-external-Redis cache group. Conversely, `TestEmptyClaimCache_EmptyRuntimeIDIsNoOp:63` calls the external helper despite its name. The two cache files contain **17/17/17** tests: **5/5/5** no-external and **12/12/12** external. The two external batch schedule tests add another **2/2/2**; notification controls add **4/4/4** external plus **1/1/1** no-external.

`newTaskClaimRacePool` is at `server/internal/service/task_claim_race_test.go:18–38`. It has an implicit local DB fallback when `DATABASE_URL` is absent and SKIPs on pool/Ping failure. ROOT must use an explicit recorded owned service database and reject any SKIP. No DB or Redis endpoint was loaded or contacted here. Handler/router package `TestMain` remains side-effectful even with a name filter; service profile allocation alone cannot admit those packages.

### Code patterns and retained fences

- **Initial current runtime/source validation:** Keep ValidateManagedRuntime before any no-work branch. Reliable absence never grants binding/PAT/owner/workspace/account authority. Source: `server/internal/service/task.go:3938`, `server/internal/service/managed_runtime.go:25`, `server/internal/service/managed_runtime.go:63`.
- **Original real writer defaults and lock order:** LockManagedRuntime delegates false. Canonical positive reclaim still uses exclusive password user, namespace, current runtime/binding and installation admission before task locks; no shared-mode/public flag or singular rewrite. Source: `server/internal/service/managed_runtime.go:123`, `server/internal/service/managed_runtime.go:129`, `server/internal/service/managed_runtime.go:186`, `server/internal/auth/password_session.go:175`, `server/pkg/db/queries/password.sql:16`.
- **Necessary-condition probe only:** Only a reliable necessary-condition negative may skip Step2. True/unknown/error must follow the separately admitted canonical/error policy. Probe is read-only and non-authorizing; it must not call lock_issue_execution or return private rows. Source: `server/internal/service/task.go:3989`, `server/pkg/db/queries/agent.sql:1037`.
- **Complete set and due-hint/backstop policy:** Any due runtime still triggers complete uniqueIDs canonical work when potential rows exist. Nil/failed cache remains conservative; retain retry/backstop schedule and do not forget due hints for a skipped/locked positive candidate. Source: `server/internal/service/task.go:3986`, `server/internal/service/task.go:4009`, `server/internal/service/reclaim_check_cache.go:97`, `server/internal/service/reclaim_check_cache.go:271`.
- **Queued work and deferred side effects:** Do not return the whole method early after a negative reclaim probe. Continue original cache version sampling, queued candidates, per-agent claims, routing, cap and same-call deferred promotion events. Source: `server/internal/service/task.go:3960`, `server/internal/service/task.go:4035`, `server/internal/service/task.go:4053`, `server/internal/service/task.go:4102`, `server/internal/service/task_batch_claim_deferred_test.go:28`.
- **Temporal/lease/started fences:** Potential predicate must be a conservative superset using the DB clock and necessary dispatched/never-started/recovery/expired-or-absent lease predicates. Canonical UPDATE remains unchanged; active lease/young dispatch/started task never becomes an authorized reclaim. Source: `server/pkg/db/queries/agent.sql:1079`, `server/internal/handler/managed_claim_recovery_test.go:12`.
- **Binding/admission/generation/history:** Previously admitted stopped recovery keeps original binding/epoch/admission. Unknown history is never backfilled. Real reclaim advances generation and finalization/receipt/requeue CAS continue to reject older delivery. Source: `server/internal/service/managed_admission_test.go:197`, `server/internal/service/managed_runtime_test.go:165`, `server/internal/service/managed_admission_test.go:250`, `server/pkg/db/queries/agent.sql:1058`.
- **Committed partial prefix:** Return genuinely committed earlier claims/reclaims when a later candidate/agent claim fails; never convert that partial success to an empty failure. Source: `server/internal/service/task_batch_claim_partial_test.go:83`, `server/internal/service/task_batch_claim_partial_test.go:115`, `server/internal/service/task.go:4055`, `server/internal/service/task.go:4110`.
- **Retained capacity/source admission:** Retained same-namespace reservation still blocks ordinary queued work until proof. Preserve ordinary source/triage/lock_issue_execution admission; a pure negative precheck grants no execution. Source: `server/internal/service/task_collaboration_capacity_test.go:75`, `server/internal/service/legacy_execution_boundary_test.go:139`, `server/internal/service/triage_admission_test.go:34`, `server/pkg/db/queries/agent.sql:1892`.
- **Unchanged HTTP/router and original acceptance:** Service profile allocation alone does not admit handler/router execution. Retain unchanged adapter/source/finalization bindings and R30 2PASS/1FAIL/1NOT_RUN,retry0; do not claim new R4/native/full S2 acceptance. Source: `server/internal/handler/daemon.go:1740`, `server/cmd/server/router.go:1316`, `server/cmd/server/router.go:1320`.

The frozen `agent.sql` contains the real `ReclaimStaleDispatchedTasksForRuntimes` UPDATE at1037 and candidate SELECT at2415, but no standalone `HasPotentialDispatchedReclaimForRuntimes`. The candidate SELECT calls `lock_issue_execution`; it is not a lock-free absence probe and does not establish absent dispatched work. A future reliable negative can skip **only** the reclaim transaction after existing initial validation and deferred handling, then must continue queued/capacity/finalization behavior. Positive rows still enter the exact original locked UPDATE. Probe failure must not become empty success. Negative-at-read and a later clock/concurrent-row transition must retain the ordinary retry/backstop policy.

No named original probe-error or negative-clock-transition test exists for that new helper in this frozen scope. ROOT must assess true/false/error and temporal boundary coverage for the actual admitted implementation; this inventory does not invent new original test names.

`TestManagedBatchClaimLostResponseRecovery` is a useful **conditional exact handler** temporal supplement: it demonstrates young dispatch with expired lease, old dispatch with live lease, eligible real reclaim, generation+1 with retained binding/epoch/admission, duplicate denial and started-task denial. It requires a separate owned handler permit/database, not Redis. The direct batch handler pair supplies **13/13/13** originals. Password-reset finalization adds **1/1/1** only if relevant. The managed router top is **1/4/3** and does not supply a direct batch router assertion. Unchanged handler/router source and old execution bindings may be retained; a fresh full116/handler99/router54 is not implied.

### New regression boundary

The separate R31 authority-scope report proposes implement-owned **2/6/4** new tests: `TestOrdinaryIdleClaimAndR4Coexistence/{idle_first,r4_first}` and `TestOrdinaryReclaimPreservesAuthorityFences/{reclaim_first,r4_first}`. This inventory did not inspect or modify that peer test. If ROOT verifies those are the actual frozen names, combining them with the primary originals gives **18/33/28**. Keep proposed inventory distinct from existing names and actual normal/race terminal evidence.

### Files found and raw source hashes

Each SHA256 below is the complete raw file from the stated frozen source. The JSON also records file modes/byte counts, R30 comparison status, each selected test definition,33 protected production functions and12 canonical query spans. A bounded read manifest does not replace ROOT full frozen-source/configuration/resource binding.

| Frozen path | One-line role | Raw SHA256 |
| --- | --- | --- |
| `server/cmd/server/managed_installation_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `0222a3c70f59882ea04c5a86af8873339e594d26a6f262a438df27d21fe46d17` |
| `server/cmd/server/router.go` | Actual router construction, current-auth routes and both claim-cache factories. | `99b8add17fa3251ec830a0069e8477c67c74a83626a62a829228685142bf4b11` |
| `server/go.mod` | Frozen local Go/pgx/Redis dependency versions; no miniredis dependency. | `c09c290d68ca3a6e4a6aad430fbdeb35a18020ba2d767da156f249093d3180b5` |
| `server/internal/auth/password_session.go` | Original account/version validation and exclusive password-session fence. | `69530f6da81e79658f47e28f4db801d651a6603b9fd7bea621aaecf8e393d33c` |
| `server/internal/handler/daemon.go` | Actual HTTP batch adapter and singular adapter; conditional scope fence. | `239693e1a475a72d3706b565d9a7e219f3fb1f13ec2fd1a2a87ddefba581fa73` |
| `server/internal/handler/daemon_batch_claim_finalize_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `f66fb053c5864d6ab26ee8d5cf0c350046ac8ad7fc0069ebe5ae091201d6d381` |
| `server/internal/handler/daemon_batch_claim_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `65df76992d12e70a368978112127388c2291f7e339a598f4c0f11d0b6bc1f7cf` |
| `server/internal/handler/managed_claim_recovery_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `e85a824c8f13ef97f5dd4bb7cd65eb5b035252730d427fecc6e1a48d11c41460` |
| `server/internal/handler/password_claim_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `b373720af4d5950f3b67d937049054cce97362672d9c29ffc5332276c7431281` |
| `server/internal/service/empty_claim_cache.go` | Queued negative cache version, miss, mark and invalidation semantics. | `71d40bbf6d1b936532868c6562ad3b3147170f6552f16fdfaf5e408652841430` |
| `server/internal/service/empty_claim_cache_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `d7671491440c8e37bc9943b17656ba82858161670df6e93205a7752d9d5d43dd` |
| `server/internal/service/legacy_execution_boundary_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `60d6e09f12461df0599a49346f7e79c6ce681508deae3d58f519aef56008792b` |
| `server/internal/service/managed_admission_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `1536e327a6e8f6fb1bb8161fd78e7a8cc4b44d2d957b2ebad585e1c7cf52ca0d` |
| `server/internal/service/managed_runtime.go` | Current source validation, exclusive writer default and namespace/admission/runtime ordering. | `325251ecb8a8adeaf3c8c3a97ef53302750fce9a8c163d579087e25f12288e96` |
| `server/internal/service/managed_runtime_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `ade350bbe3d64aa3021d920b7e32d06959982269adcf1b7dbc8b63b8fdf27da0` |
| `server/internal/service/reclaim_check_cache.go` | Nil/failure due behavior and Redis schedule/backstop/MarkChecked policy. | `7fa0f3a8ec6568e4d6bf3abcdc2088ac131f79656715f19fead5ea78b0c8dfe3` |
| `server/internal/service/reclaim_check_cache_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `4fb2b4965d108d499ed2a826f498ffaeff8988bb6a5714ad65e393a50b5d5f12` |
| `server/internal/service/task.go` | Batch Step2 control flow; singular/shared claim, finalization, tracking and notification defaults. | `28c28dfe8bc3a75bb6e38d9727ed8632d7d52ac120d3d61cb06422021625fd50` |
| `server/internal/service/task_batch_claim_deferred_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `7166c5b63baf90bf157e89328d081ce5e776798599209c9989f819f1176d8a00` |
| `server/internal/service/task_batch_claim_partial_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `0c8f0f56fe85efb73866ac59efc54604a17b590ab7f1d2482658c08fd34f980e` |
| `server/internal/service/task_batch_claim_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `5e26de4c2953b80518d4de275995551af7c3278b433a20b48fe80d6d4857c4f8` |
| `server/internal/service/task_claim_race_test.go` | Original DB-pool helper: implicit local fallback and setup SKIP behavior; not executed. | `2c6f64965557e263167b17a0d182fbfa3013f9057a09c61873f05e2123fc16b2` |
| `server/internal/service/task_collaboration_capacity_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `0c918a7c36a1c269b60c66f14bd0bdb175ec066ce0ec00dd7b05e6366c0cf68d` |
| `server/internal/service/task_notify_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `8529b37553ea806f0de134036824e4c5b980e96ddef1cfbc4f0db5c4ec285154` |
| `server/internal/service/triage_admission_test.go` | Original bounded test source; complete or selected top inventory recorded below. | `d99a2bdb034a5d011631ef3dd321565c4b4d24844eb17bca68d792493ca09f67` |
| `server/pkg/db/generated/agent.sql.go` | Frozen generated agent queries and sqlc v1.31.1 header. | `234283e657fcdb39c14bea05decb8005cf20e43b5bb63b29ba7f99a3347a42a5` |
| `server/pkg/db/queries/agent.sql` | Canonical claims/reclaims/candidates/capacity/lease/generation SQL and location for any proposed new EXISTS. | `4d900ec193fb94e9b3c8f4835224ab64035872a85788bb2b42526cd0ea3999a1` |
| `server/pkg/db/queries/managed_admission.sql` | Installation admission read lock query. | `6e444d1b5a231ed1d6ffd728875cd8ecc9a807dd6b38acae6295925427d3f977` |
| `server/pkg/db/queries/managed_installation.sql` | Installation namespace lock query. | `cf5d12b86d21aaa3120823e7b37c1922b5fc3b933be9119a568cef927779a181` |
| `server/pkg/db/queries/password.sql` | Original LockPasswordUser FOR UPDATE query. | `6dee450352ed9d26f193b94d64fd6e6716722a994cde4570982697c1f0a5ba38` |
| `server/sqlc.yaml` | PostgreSQL/pgx-v5 generation configuration; no generator executed here. | `ade6bdf31030c0a8c366e6a981c97efacd3faab408a8e504d6f4530e11e057e4` |

- Bounded sorted file/hash manifest SHA256: `1f69f05391112121c35c936a56e978ded4314404fa7d2878d849d319373d8d62`.
- Comparison source: `/Volumes/artisan/.multica-acceptance/pay241-r30-qa-read-20261118-pohc3l9t/source-green-r01`; individual equal/different/not-found statuses are in JSON, without Git. All primary-original source files must retain their exact selected names/definitions in an admitted GREEN snapshot.
- Root `Makefile` is absent from this frozen snapshot. Separately read workspace `Makefile:406` uses `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`; the frozen generated header independently reports sqlc v1.31.1. This is a local command/version reference, not a generator/compile result.

### External references and versions

- Frozen `server/go.mod:3`: Go1.26.6; declared pgxv5.9.2, go-redisv9.18.0 and redismockv9.2.0. No miniredis dependency is declared.
- Frozen `server/pkg/db/generated/agent.sql.go:3`: sqlcv1.31.1; `server/sqlc.yaml` selects PostgreSQL and pgx/v5 generation.
- First-party reference entry points: [Go module reference](https://go.dev/doc/modules/gomod-ref), [sqlc documentation](https://docs.sqlc.dev/en/stable/), [go-redis source](https://github.com/redis/go-redis), [redismock source](https://github.com/go-redis/redismock), [pgx source](https://github.com/jackc/pgx). No web retrieval occurred; only local source/version facts are asserted.

### Related specs

- `.trellis/workflow.md`: Research persistence/role separation and parent-owned lifecycle.
- `CLAUDE.md`: Go conventions, narrow verification, test fixtures, migrations and package boundaries.
- `.trellis/spec/server/index.md`: Server authority and verification guide index.
- `.trellis/spec/server/managed-installations.md`: Current MDT/PAT/binding/owner authority, user-to-namespace ordering, admission and delivery history/generation/90-second lease recovery.
- `.trellis/spec/server/security-boundaries.md`: Current source account version, password reset/minting lock and unavailable-versus-invalid distinctions.
- `.trellis/spec/server/collaboration-stage-approvals.md`: Same-Q R4/current-source read authority and unchanged real writer defaults; bounded source/raw reuse.
- `.trellis/spec/server/database-test-isolation.md`: Explicit task-owned DB, TestMain side effects, source/profile/resource proof and zero skipped named acceptance.
- `.trellis/spec/server/verification-environment.md`: Process-specific env, source/configuration/ownership binding and exact mixed JSON inventory.
- `.trellis/spec/server/triage.md`: Formal source admission and canonical no-execution fences.

## Caveats / Not Found

- All counts are static source-derived inventories, not PASS results. No Go/AST/test-list/TestMain/SQL/DB/Redis/API/browser/runtime/signals/Git/dependency or agent operation was executed.
- The original batch test file has exactly five top-level tests. DB-only batch originals total six across the main/partial/deferred files. Primary original union is 16/27/24, not an estimate based on a sixth nonexistent main-file test.
- newRedisTestClient reads REDIS_TEST_URL, forces DB12, Ping/FlushDB and cleanup FlushDB/Close; unset/unreachable endpoints skip. Dedicated task-owned Redis is required for conditional external controls. Never export a global endpoint or reset foreign Redis.
- TestReclaimCheckCache_RedisFailureFallsBackToEveryRuntime closes a 127.0.0.1:0 client before use and belongs in the no-external-Redis union. TestEmptyClaimCache_EmptyRuntimeIDIsNoOp uses the external helper despite its name.
- newTaskClaimRacePool still falls back to an implicit local database and skips on parse/Ping failure. ROOT must load the recorded explicit owned DB/profile and reject SKIP; no environment or fixture edits were made here.
- Singular managed-history and revoked-claim controls protect shared source/provenance defaults; they do not establish that a batch-only optimization fixes singular idle polls.
- No existing standalone necessary-condition EXISTS or named probe-error/negative-clock regression was found in the frozen bounded source. Any new probe needs ROOT-reviewed true/false/error and clock-boundary tests appropriate to the admitted behavior; proposed names are not invented as originals.
- No direct batch router test was found in the bounded source inventory. The listed router top covers managed enrollment/provenance/recovery, while direct HTTP batch coverage lives in handler tests.
- No fresh full116/handler99/router54, R4/native diagnosis, original browser retry, deadline adjustment or successful-completion replay is justified by this static inventory alone.
- Root Makefile is absent from the frozen snapshot; sqlc v1.31.1 is established by the frozen generated header and separately read workspace Makefile:406, not by generator execution or a frozen Makefile hash.
- Only this new report pair may be created and sealed0444; source files, specs, manifests, configuration, peer files and historical reports are preserved. The bounded source hashes do not replace ROOT full source/configuration/resource binding.
