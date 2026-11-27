# Backend implementation and verification — 2026-11-27

Native Trellis implementation lane continued the existing uncommitted runtime implementation on `codex/git-engineering-material-workflow`, HEAD `ba39ec44e`. No unrelated work was reverted. Production ownership was handed to the independent checker after the runs below; subsequent checker fixes need their own final evidence.

## Implemented behavior

The existing runtime settings, hidden carrier, immutable operation/task insertion, claim challenge, daemon-only observation endpoints, plan preparation, and server-provider removal were audited and extended:

- Complete trusted `observe/publish` absence can settle `no_effect`; only a later explicit retry creates another publication attempt, retaining the original branch/key and previous tasks. Direct publication/model output or incomplete absence cannot release another write.
- Accepted observation identities replay after completion without replaying state transitions. Different bytes under the same identity conflict, and a fresh observation cannot rewrite an already settled successful/no-effect operation.
- Merged receipts use the provider's actual `merged_at` and current approval/head/path checks. Observation can arrive after permit expiry if the actual merge occurred within its issued interval. Missing, earlier/later or future merge timestamps fail closed. The issue boundary is the second containing permit issuance, matching hosting API timestamp resolution; there is no arbitrary extra grace interval.
- Runtime and profile reads during authorization take SHARE NOWAIT fences (reuse the existing runtime dispatch query, one narrow profile query). Runtime/configuration changes cannot silently transfer a frozen attempt. Database failures remain retryable errors; confirmed revoked runtime-owner membership settles the unstarted task without blocking later queue work.
- Public-runtime membership alone cannot bootstrap the private challenge. Refused host claim payloads requeue the exact unfinalized delivery so the actual host can claim it; the router test no longer repairs the queue manually.
- Runtime deletion retains carrier, operation descriptor/receipt history, terminal tasks, decisions, baseline and plan issue. The frozen original runtime remains in operation history.
- Terminal cancelled queued operations project `unknown`; terminal preflight with unmet native gates projects `blocked` with `material_native_checks_pending`, rather than falsely showing queued/running/checking forever.
- Operation identity index down migrations now refuse to remove uniqueness while operation history exists. The actual up/down regression verifies all three concurrent unique indexes, no foreign keys, unchanged receipt rows, retained indexes and unchanged ledger after refused downgrade.

## This continuation's changed files

- `server/internal/service/material_operation.go`, `material.go`: recovery, receipt/permit handling, current authorization, accurate operation projection.
- `server/internal/service/material_test.go`, `material_observation_test.go`: canonical service and pure evidence regressions.
- `server/internal/handler/daemon.go`, `server/cmd/server/material_runtime_authority_test.go`: refused host claim recovery and real-router assertion.
- `server/pkg/db/queries/git_material.sql`, regenerated `server/pkg/db/generated/git_material.sql.go`: narrow profile authorization lock.
- `server/migrations/910_git_material_operation_id.down.sql`, `911_git_material_operation_task.down.sql`, `912_git_material_operation_attempt.down.sql`: identity retention guards.
- `server/cmd/migrate/migrate_material_operation_test.go`: real concurrent-index and retained-history down test.

The protocol, provider parser and daemon observer additions for `merged_at` belong to the runtime lane. The server performs no remote reads or writes in these changes.

## Verification

Task-private evidence root: `/tmp/material-backend-resume-ed583b04`. `allocation.json` records fresh service/handler/router databases, verified with `current_database()` at allocation before migration; package invocations loaded those exact recorded environments. A separate migration-only database was recorded in `migrate.json`. Credentials stay in private files, outside the repository. No concurrent suites shared a database. Repeated focused suites used their own existing package database; these were not whole-repository acceptance runs.

| Check | Result | Evidence |
| --- | --- | --- |
| Initial runtime lookup and absent-publish regressions | 2 expected failures before fixes | `backend-red.log` |
| Revoked runtime owner regression | Expected failure before error classification fix | `runtime-revoke-red.log` |
| Final service material + ordinary claim/batch/finalization/start, `-race` | 43 top-level tests passed | `service-final-race-2.log` |
| Final material service after status projection changes, `-race` | 25 top-level tests passed | `service-final-race-3.log` |
| Handler material/claim/reclaim/start regression | 84 top-level tests passed | `handler-regression-1.log` |
| Router material + existing task-capability regression, `-race` | 6 top-level tests passed | `router-final-race-2.log` |
| Actual material operation migration upgrade/down | 1 top-level test passed | `migrate-regression-1.log` |
| `make sqlc` | Passed, generated query updated | Current generated SQL code |
| `go vet` service/handler/server/migrate | Passed | `backend-final-vet.log` |
| gofmt and `git diff --check` | Passed | Current tree |

`backend-summary.json` in the evidence root records named log hashes and pass/fail totals. The first router run retained one stale test expectation failure (material Create returns the established 200, not 201); the corrected test then exercised the real host boundary and passed. Logs retain this failure rather than replacing it.

Key final commands, with the exact package-private environment loaded:

```sh
go test -race ./internal/service -run '^(TestMaterial|TestClaimTaskConcurrentCapacityRespected|TestClaimTasksForRuntimes_|TestFinalizeTaskClaimFailureRollsBackTokenThenRequeue|TestStartTaskIteration)' -count=1 -v
go test -race ./internal/service -run '^TestMaterial' -count=1 -v
go test ./internal/handler -run 'Test.*(Material|ClaimTask|ReclaimTask|StartTask)' -count=1 -v
go test -race ./cmd/server -run 'TestMaterial|TestTaskCapability' -count=1 -v
go test ./cmd/migrate -run '^TestMaterialOperationMigrations' -count=1 -v
```

## Acceptance mapping and remaining scope

- C1: Runtime configuration/dispatch and no material server provider path exercised; actual two-provider native execution is owned by runtime composite acceptance.
- C2: Runtime switch, deletion, owner revocation, claim contention and ordinary queue regressions pass. Same-carrier plan submission after configuration change is under independent review.
- C3: Current human approval/HEAD/native gates, unexpected changes, byte/commit/mode rejection, unauthorized external merges and provider timestamp boundaries pass at service level.
- C4: Wrong binding, host-only bootstrap, task-token refusal, exact replay and conflicting receipt identities pass; independent full-scope check remains required.
- C5: Scheduler lease duplication, absent publication recovery, lost replies, delayed actual merge observation, and stable blocked gate status pass. Independent checker identified review-note absence recovery as an additional gap and owns its correction.
- C6: Auto-off/no queue, re-enable preserving manual hold, same-issue terminal restart, fresh baseline preparation and superseded baseline regressions remain passing.
- C7/C8 frontend/browser/build/full integrated final check remain main/other lane responsibilities.

No real model invocation or remote repository mutation was performed. Full default Go suites, real platform/model acceptance and packaged installers are not claimed by this lane. Service fake readback validates server admission and state transitions; it is not a replacement for the runtime executable/daemon composite chain.
