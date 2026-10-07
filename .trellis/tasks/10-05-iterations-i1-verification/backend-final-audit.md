# I1 final backend acceptance audit

2026-10-07. Base HEAD `115b4cd28`, with the preserved CG/client/acceptance
working tree. This is the backend evidence audit for the existing verification
task, not a separate product gate or remote release certification.

## Evidence and source boundary

- `B`: `.omx/logs/i1-verification/full-check-accepted.log`, the accepted complete
  local check. The parent verified that its 10:38 launch follows the last five
  changes relative to `pre-full-source.json`. It includes guarded full Go race
  tests, build/vet and TypeScript checks. Relevant packages report handler
  202.745s, service 174.145s, migrations 7.854s, and cmd/migrate 10.409s. Some
  unaffected packages use the Go test cache; this is not a claim of zero cache.
- `L`: parent `lg-hg-verification.md/json` and the lifecycle/history child
  verification ledgers: real LG/HG/T1 paths, not just fixtures with fabricated
  statistics. Old `/tmp` paths are provenance, not currently retained raw logs.
- `C`: parent `cg-verification.md`, closure `cg-integration-evidence.json`,
  `closure-writers-verification.md/json`, and `notifications-verification.md/json`.
  Their selections overlap; counts must not be summed as unique scenarios.
- `P`: `performance-evidence.json` and retained P95 log: the real 1,000-task
  repeated-carryover and 51-plan/1,530-member disable harnesses. Ordinary-write
  samples are separately retained in `ordinary-write-evidence.json`.
- `H`: `.omx/logs/i1-verification/acceptance-history-final.jsonl`, fresh guarded
  race run after this audit strengthened the A–J test with real external
  completion and a parent/child relation. It also runs the history family and
  the dedicated active-issue deletion regression. Test hashes and exact results
  are in `migration-recovery-evidence.json`.
- `M`: `.omx/logs/i1-verification/migration-recovery-final.jsonl`, the new
  I1-specific partial/invalid-index recovery rehearsal described below.

All paths in the table are relative to `server/internal/` unless prefixed
otherwise. File/function pairs identify the actual inspected assertion; a
composed proof is stated explicitly rather than claiming a literal scenario
that the test did not run. User-interface assertions remain with the client
audit and final Web/Electron verification.

## All 29 ITR-AC requirements

| AC | Current source/test assertion | Execution evidence |
| --- | --- | --- |
| 01 | `handler/iteration_settings_test.go`: `TestIterationSettingsDefaultAndReleaseGate` asserts off/no settings row/no operation; `TestEnableIterationSettingsValidatesCurrentRoleAndTimezone` rejects stale timezone and accepts confirmed Shanghai; `TestEnableIterationSettingsDurableReplay` saves settings once. Enable writes only settings/operation, not iterations or memberships. | B; FG settings ledger; visible timezone confirmation belongs to UG. |
| 02 | `handler/project_timezone_test.go`: `TestProjectPlanningTimezoneContractAndHumanPermissions` rejects member and machine configuration; `service/iteration_closure_test.go`: `TestIterationClosureDisableAdminLimitAndRollback` rejects member disable and retains enabled/data on failure. | B, L, C. |
| 03 | `handler/iteration_list_test.go`: `TestIterationListKeysetAndScopeBinding` creates three plans and pages them independently; `service/iteration_lifecycle_test.go`: `TestIterationLifecycleCreateReplayAndSavedTimezone` creates explicit future dates. `Create` saves planned status and has no start side effect. | B, L; composed create/list proof. |
| 04 | `service/iteration_lifecycle_test.go`: `TestIterationLifecycleCompetingStartsOnlyOneActive` uses two members behind a held workspace fence and asserts one success, one409 and one active. `migrations/iteration_rollback_test.go` also exercises the actual unique index with two connections. | B, L. |
| 05 | `service/iteration_notifications_test.go`: `TestIterationOverdueLocalDayAndFallback` crosses Shanghai midnight, gets two distinct-day reminders and compares the entire running task row; active period and in_progress issue stay unchanged. DST, date-extension, disabled and revoked cases are adjacent tests. | B, C. |
| 06 | `handler/issue_iteration_assignment_test.go`: `TestIssueIterationAssignmentTerminalAdmissionAndHistory` rejects pending/rejected/duplicate confirmed PUT; `service/iteration_lifecycle_test.go`: `TestIterationLifecycleNoopAndNewActiveAdmission` rejects the same complete-preview entries. `handler/iteration_confirmation_test.go` rejects unconfirmed old create/update/batch/Plugin fields428 without writes. `PrepareMembershipChange` fails every admission value except accepted/not_required before writing. | B, L, FG cross-entrypoint ledger. |
| 07 | `service/iteration_lifecycle_test.go`: `TestIterationLifecycleManagementPreservesRunningExecutionAndProject` compares the complete persisted task before/after join/start/leave; `service/iteration_closure_test.go`: `TestIterationClosureManagementPreservesRunningTask` additionally verifies project/status/assignee for end/cancel/handoff/disable. | B, L, C. |
| 08 | `service/iteration_membership_event_facts_test.go`: `TestIterationMembershipEventsFreezeSourceAndTarget` executes null→A→B→null→B plus rollover for serial/batch paths, checks both event sides, identity and counters. `TestIterationLifecycleBatchSwappedSources` proves complete swapped-source results. | B, later history-fields ledger. |
| 09 | `handler/issue_iteration_assignment_test.go`: `TestIssueIterationAssignmentTerminalAdmissionAndHistory` rejects historical membership changes409; `iteration/membership.go` applies the same open-status rejection loop to source and target before terminal eligibility or writes. Closed read/snapshot tests verify immutability. This is a shared-branch proof, not an independent done-to-completed-target browser scenario. | B, L; static target-branch audit. |
| 10 | `handler/iteration_history_test.go`: `TestIterationHistoryCanonicalThroughLifecycleAndIssueWriters` starts A–H then joins I/J through real service operations; Original=8, AddedUnique=2. | B, L; strengthened H. |
| 11 | The same real A–J fixture asserts Current9, Effective8, Completed5, OriginalCompleted4 and .625/.5. Pure `iteration/stats_test.go:TestStatisticsCanonicalExample` owns the mathematical boundary example. | B, L, H. |
| 12 | The strengthened real A–J fixture moves F out via the complete operation, completes it through ordinary `UpdateIssue`, proves live F is done/unassigned, keeps D5/OD4 and creates no source-period event; deletion afterward still preserves the original summary. | H, fresh direct proof; old pure comment alone was insufficient. |
| 13 | The same fixture creates B with real `parent_issue_id=A`; both original and current history contain A and B exactly once, while A–J counts remain unchanged. The UI must separately explain task count rather than workload. | H, fresh direct proof; UI explanation owned by UG. |
| 14 | `iteration/stats_test.go`: `TestStatisticsEmptyCancelledNoOpsAndReopen` asserts null denominators for empty/all-cancelled and treats cancellation separately; `TestStatisticsDeletionPreservesOriginalAndNullRatiosJSON` preserves O while removing completion contribution. | B, L. |
| 15 | `service/iteration_closure_test.go`: `TestIterationClosureRejectsMissingAndTerminalMoves` enforces the exact remaining set; `TestIterationClosureCompletionAfterPreviewAndTerminalRelease` rejects stale completion, then freezes a null destination with unchanged count; end/batch/multi-target tests verify actual moves. | B, C; set validation rather than a special hard-coded three-plus-two fixture. |
| 16 | `handler/iteration_i1_p95_test.go:TestIterationI1P95` repeatedly closes/starts the same 1,000 tasks and asserts every task's count equals sample+1 (including1→2). `TestIterationClosureLostCommitResponseRecoversOriginalOperation` and `TestIterationClosureRollbackReplayAndFrozenDigest` prove same-ID retries do not repeat count/snapshot/outbox. | P plus B/C replay evidence; composed proof. |
| 17 | `service/iteration_closure_test.go`: `TestIterationClosureTargetCancelRace` holds/cancels target while closure waits and asserts whole-operation rejection. Real handler writer races compare snapshot/settings and pointer results in both queue orders. | B, C. |
| 18 | `handler/iteration_closure_writers_test.go`: `TestIterationClosureHTTPRecoversLostCommitResponse` injects an error after successful Commit, GET retrieves the same operation and repeat keeps one snapshot. Service lost-response test additionally asserts one end event/outbox and one rollover; delivery test proves one visible inbox. | B, C. |
| 19 | `service/iteration_closure_test.go`: `TestIterationClosureManagementPreservesRunningTask` compares running task identity/fields; `TestIterationClosureDaemonCompletionBarrierDoesNotRestartExecution` uses actual `TaskService.CompleteTask` concurrently, leaving exactly one completed task and unchanged issue todo semantics. | B, C; uses test task rows, never a real agent CLI. |
| 20 | `handler/iteration_closure_writers_test.go`: `TestIterationClosureRealPostClosureWritersPreserveSnapshot` drives reopen/rename, project reassignment, actual P1 project deletion and issue deletion and compares the snapshot digest after each. `iteration_history_fields_test.go` adds actual label rename/delete and priority changes with frozen names. | B, C, H for history field tests. |
| 21 | `service/iteration_lifecycle_test.go`: `TestIterationLifecycleUsedPlanCannotDeleteAndCancelReleasesAll` rejects a now-empty used plan; `TestIterationLifecycleEditGuardsAndDeleteReplay` renames an unused plan then deletes/replays it successfully. | B, L. |
| 22 | `service/iteration_closure_test.go`: `TestIterationClosureCancelAndDisable` reaches 103 plans beyond page1; P95 disable fills51 plans/1,530 members and checks zero remaining pointers, lifecycle state and snapshots. | B, C, P. |
| 23 | `service/iteration_closure_test.go`: `TestIterationClosureDisableAdminLimitAndRollback` injects failure and tests2,001 cap; `handler/iteration_closure_writers_test.go`: `TestIterationClosureRealIssueAndProjectWriterRaces` exercises writer-first disable409 with no snapshot and enabled settings. | B, C. |
| 24 | `service/iteration_lifecycle_test.go`: `TestIterationLifecycleCreateReplayAndSavedTimezone` preserves an old UTC plan after workspace default changes; shared planning-timezone API test verifies the new effective default. `Create` validates and captures that current effective timezone; edits do not accept timezone. | B, L; composed persisted-field/API/creation-path proof. |
| 25 | `iteration/calendar_test.go`: `TestDayBoundaryCalendarTransitions`, `TestCalendarInputValidation`, `TestStartDatesRespectLocalTodayAndCalendarLength` cover NY23/25h, Apia skipped date, midnight gap/fold, invalid timezone and pure dates. Start/handoff midnight tests reject changed reference dates. Viewer display evidence belongs to UG. | B, L, C; no claim that backend tests alone render multiple viewer zones. |
| 26 | `handler/iteration_confirmation_test.go`: `TestIssueIterationOmittedFieldsPreserveExistingMembership` changes title via real PUT/batch and retains pointer/count. `packages/core/api/iteration-client.test.ts` distinguishes old404 from permission/network/protocol errors; strict schema tests reject malformed identities/counters. `apps/mobile/data/iteration-compatibility.test.ts` verifies actual client serialization omits fields and old-server unknown remains unknown. | B, FG, client/mobile ledger; mobile uses API-response fixtures, not an installed historical binary. |
| 27 | `handler/iteration_operations_test.go`: `TestGetIterationOperationConcurrentRevocation` and history authorized-read tests hold current membership; `iteration_closure_writers_test.go:TestIterationClosureRealMemberRevocationRaces` rejects protected reads after revoke. Notification revoke/crash/rejoin suites retain content-free tombstones and cannot recreate inbox. Shared attachment authorization remains the existing resource boundary. | B, C, H; full linked-page/UI cache epoch proof owned by UG. |
| 28 | `handler/iteration_closure_writers_test.go`: `TestIterationClosureRealIssueAndProjectWriterRaces` includes real ordinary done writer versus end/disable in both lock orders. Writer-first returns stale409 without closure; service completion-after-preview test independently verifies terminal release. | B, C. |
| 29 | `service/iteration_lifecycle_test.go`: `TestIterationLifecycleClockMidnightAndEmptyBaseline` actually starts an empty plan. `TestIterationLifecycleFirstActiveJoinAfterPlannedLeaveAndStartedReset` proves O0/AddedUnique1/Reentry1; pure nullable-ratio tests establish null original denominator. | B, L; composed lifecycle/math proof. |

## Migration, compatibility and rollout conclusions

The initial audit found no I1-specific real partial/invalid-index recovery run.
Existing generic runner recovery tests and cleanup-map registration tests were
valuable, but could not honestly be called that rehearsal. The new
`server/cmd/migrate/migrate_iteration_recovery_test.go` now fills it with real
550–566 SQL and `hooksForDirection`, using only owned private schemas in the
exclusive audit DB. See `migration-recovery-evidence.md/json`.

Existing `migrations/iteration_rollback_test.go` protects pointers, rollover,
settings and operation tombstones at every down step and rejects an uncommitted
writer. `iteration_project_p1_migration_test.go` checks all17 down steps preserve
used I1 data/all29 P1+I1 indexes, and empty I1 rollback retains real P1 tables,
13 indexes and shared timezone. B reran those packages. The new partial-prefix
test complements these; it does not duplicate a production database deployment.

Compatibility remains additive. No I1 migration owns or drops
`workspace.planning_timezone`; old ordinary writes omit and preserve iteration
fields. Unknown new data is read-only/protocol error at client boundaries, and
missing confirmation cannot become a write. Installed old Desktop/Mobile/CLI
binaries are not executed; the retained wire fixtures and real handler guards
are the compatibility evidence. A server predating I1 facts is not a supported
downgrade once any I1 data exists, even if the feature is later disabled.

The final `label.go` change adds fences only to attach/detach/rename/delete label
routes. The P95 harness invokes detail/preview/closure; the ordinary benchmark
invokes `UpdateIssue`, which does not call `beginIssueLabelWrite`. Existing
performance samples remain applicable to their measured paths. They do not
certify throughput under concurrent label-edit workloads.

`iterations_i1` remains false by default and is absent from generic public
frontend flags. These tests enable only isolated providers. No commit, push,
merge, deployment, external message or rollout setting was performed here.

## Changes and remaining limits

- Added one migration regression file; reused the runner, real SQL and existing
  schema-scoped pool helpers. No production migration or service change.
- Strengthened the existing canonical history test for AC12/13 instead of
  creating a second statistics implementation or duplicate A–J suite. The
  dedicated active-delete regression still runs alongside it.
- Preserved the first history fixture failure: it used nonexistent `parent_id`;
  the real database field is `parent_issue_id`. The corrected run is green; no
  product defect was hidden or fixed to make it pass.
- Go race and narrow vet/format checks cover the two changed test files. Prior
  complete TS lint/typecheck remain applicable because this lane changes no TS.
- Final visual/keyboard checks, explicit skip classification, final gate/task
  states and whole-worktree source provenance remain parent-owned. No additional
  backend product bug or mandatory backend rerun was identified by this audit.
