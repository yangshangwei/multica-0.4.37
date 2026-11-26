# Backend review and regression evidence

Reviewed on 2026-11-27 (Asia/Shanghai). Scope: `server/**` excluding `internal/materialgit/**`, which has its own provider review. No live Git repository or real agent/model was used.

## Fixed findings

- The service test provider still implemented the old merge signature. Updated it to `MergeExpectation`, matching the production worker/provider boundary.
- A second approved requirement baseline tried to create another `Plan: <title>` issue and failed the ordinary active-title duplicate guard. Reproduced by `TestMaterialNewRequirementSupersedesPlanUntilNewBaseline`. Internal plan creation now uses the existing `AllowDuplicate` option; the project lock and unique material/baseline identity continue to own deduplication. Prior plan issues and runs remain intact.
- Initial claim, singular stale-dispatch reclaim, batch reclaim, and task-token finalization did not recheck material authority. Four new regressions demonstrated that a manager demotion still allowed those paths. All now check the exact persisted task, current baseline/input, settings, human membership and current agent invocation rights. Project and agent locks use `SHARE NOWAIT` so reversed claim lock order refuses contention instead of waiting on a configuration writer.
- A permanent material denial must not leave an invalid plan at the queue head. Initial claim/reclaim now cancel only the unstarted denied task with a reason, retain its history, and omit it from returned payloads. Batch reclaim keeps healthy ordinary tasks. Transient database and `NOWAIT` errors propagate unchanged and never cancel an attempt. Regression coverage includes mixed ordinary/material queues and project/member/agent lock contention.
- Terminal planner failures had no recovery path: `retry` returned early because a task ID existed, while manual start always rejected it. An explicit human `start-plan` can now restart failed, cancelled, or completed-without-output runs on the same issue after current input/configuration/invocation checks. The old queue row stays intact; ordinary job recovery and re-enabling the switch do not restart it. Active tasks and any current plan revision remain nonrestartable; original request replay does not create a second attempt.

## Verification

Created three new owned loopback databases, proved actual database identities, and migrated each from empty through migration 908. Service, handler, and router packages ran serially against their own database. Private allocation records and raw logs are under `/private/tmp/multica-material-20261126/backend-review-e53635be/`; credentials are not copied into task files.

- `go -C server test -race ./internal/service -run '^Test(Material|RuntimeAccessGatesQueuedTaskClaims|ClaimTask|ClaimTasksForRuntimes|RuntimeHealthGatesClaims|ManagedAdmission|ManagedClaim|TriageAdmission|CollaborationManualPolicyFencesRuntimeAdmission|CollaborationRuntimeHoldBlocksClaimAndPermit)' -count=1 -v`: pass; 34 top-level tests, 167 named passes including subtests. Two existing Redis-only tests initially skipped because `REDIS_TEST_URL` was unset; both subsequently passed in the dedicated Redis run below. Raw result: `service-final-r2.log` (35.064 seconds).
- The two Redis-only claim/reclaim tests passed with `-race -count=1 -v` on a fourth fresh migrated database (`multica_gitmaterial_redis_70236f3d`) and a dedicated, password-protected `redis:7-alpine` container bound only to loopback. Raw result: `service-redis-final.log` (2.630 seconds), two passes and zero skips. Docker 29.2.1 was available; its private-directory bind mount was refused, so the private config was copied into the owned container without changing shared Docker settings. The container was removed after the run; the database and private evidence remain.
- `go -C server test -race ./internal/handler -run '^Test(PluginMaterial|TaskCapability|RequireHumanActor|PluginAction)' -count=1 -v`: pass; 27 top-level tests, 52 named passes, no skips. Raw result: `handler-final-r2.log`.
- `go -C server test -race ./cmd/server -run '^Test(MachineCredential|Plugin.*Action|PluginBridge|.*Material)' -count=1 -v`: pass; 7 top-level tests, 121 named passes, no skips. Raw result: `router-final-r2.log`.
- `go -C server test ./internal/material ./cmd/multica ./pkg/plugincontract -count=1`: all three full packages passed.
- `go -C server vet ./internal/material ./internal/service ./internal/handler ./cmd/multica ./cmd/server`: pass after final changes.
- `make sqlc`: pass using the repository-pinned version; regenerated the material query and existing model output.
- Migration 890–908 static audit: no new foreign keys; each index uses `CONCURRENTLY` in its own one-statement migration. All three fresh database migrations passed.
- `git diff --check`: pass.

The original failing service/new-baseline, four-path claim and terminal-restart logs remain in the private evidence directory, separate from successful runs. The first auto-disable test attempted `StartTask` on an unclaimed queued task; its fixture was corrected to claim first, preserving the production start-state requirement.

## Coverage limits and spec follow-up

This is an expanded targeted backend gate, not a full `internal/service` package or whole Go repository run. The initially skipped `TestClaimTasksForRuntimes_AnyDueRuntimeChecksTheCompleteSet` and `TestClaimTasksForRuntimes_EmptyReclaimDefersHintForRetry` both passed in the dedicated Redis follow-up, leaving no unresolved skips within this selected test set. Real GitHub/GitLab deployments, real model output, and installed desktop packages remain separate acceptance work.

The main session should capture the baseline identity vs title rule, current-authority claim/finalization fences, permanent-denial queue isolation, and explicit human terminal-attempt recovery in the Git materials spec. No additional unresolved backend defect was identified in this review's checked scope.
