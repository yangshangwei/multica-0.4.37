# Runtime teardown / autopilot dispatch race

## Boundary and hypotheses

CI45656329181 failed TestProjectAutopilotRunOnlyRacesRealRuntimeTeardown/with_project with `check agent readiness: no rows in result set`. That failed CI head is c920c5e5b0354258ad0a37d500655a6856d5ed5b. This repair starts from b7563d8bbeb3abe0f73151a3d17f0414cbc4d89f; their server application trees were identical before this repair. Current process Go1.27.0 darwin/arm64; final Linux CI remains separate.

Ranked hypotheses: (1) runtime teardown commits between leader resolution and the separate runtime readiness read, including a retry after NOWAIT; (2) fixture starts with a stale binding; (3) an ordinary lookup/connection failure is incorrectly classified. The deterministic regression executes the actual TeardownRuntime plus runtime DELETE transaction at the existing DBTX query boundary, after GetAgent and before GetAgentRuntime. A separate ordinary lookup-failure case must stay an error. No timeout, skip allowlist or permission fence is relaxed.

Expected contract: work never admitted to a deleted runtime is `errDispatchSkipped` / target_unavailable, as existing missing leader and locked-owner branches already specify. The transaction still owns runtime/agent/project/principal/grant validation before insertion. The shared RuntimeLookup/AgentReadiness error behavior stays intact for other callers.

Implementation scope: autopilot.go's pre-transaction readiness error classification; project_association_dispatch_test.go regression and existing real contention replay; relevant spec note if warranted. No migration, SQL, dependencies, routes or protocol changes. No commit or push by implementer.

## Isolation

Task container multica-release-088-dispatch-race, loopback28999, DB release_088_dispatch_race, owner label release-v0.8.8-dispatch-race. Random credentials are only in the private release artifact directory, mode0600. Readiness verifies actual current_database overTCP; current877 migrations executed. No other database/container used. Logs and exact commands will be retained privately with sanitized summary.

## Verification

The unchanged deterministic test failed before the production repair with the exact CI message: `check agent readiness: no rows in result set`; its ordinary-lookup-error control passed. The five-line production repair then passed both cases, including persisted skipped status / target_unavailable, preserved-unbound agent and zero enqueued tasks. Other lookup errors retain their original error and are not persisted as skipped.

The full related selection `^TestProject(DeletionStopsStaleAutopilotDispatch|Autopilot)` ran normal and race, each `-count=20 -parallel=2 -timeout=10m` under the agent-CLI guard. Each run contains six top-level tests and 22 unique named identities, each executed 20 times: 440 PASS, zero FAIL/SKIP/unfinished. This includes both original runtime-teardown cases, project association fences, dispatch-first/teardown-first commit ordering, runtime/agent/member permission revalidation, and stale project deletion. Package times: normal48.571s, race58.783s; both exit0. The before-fix regression exit1 is retained, not overwritten.

`go -C server vet ./internal/service` and final `git diff --check` passed. Commands and all executed Go source hashes are preserved in the companion receipt. Production Go and regression test bytes remained unchanged through the successful normal/race runs and match final files. Built-in skill/spec wording was synchronized separately and is not claimed as covered by those assertions. A temporary extra diagnostic-log experiment was withdrawn on the parent's clarification; its separate logs remain private and are not acceptance evidence. The exact stress-tested test bytes were restored and hash-verified.

No full service package or final Linux release CI was run by this agent. Host verification uses Go1.27.0 Darwin/arm64 with a task-owned Linux/amd64 PostgreSQL container under Docker Desktop; final same-SHA Linux/race verification remains the parent's release gate.

## Final change list

- `server/internal/service/autopilot.go`: distinguish only pre-transaction runtime ErrNoRows from ordinary database failure; preserve all owner locks/retries and final invocation checks.
- `server/internal/service/project_association_dispatch_test.go`: deterministic real teardown at the query boundary plus ordinary-error control, persisted skip and no-task checks; original contention tests unchanged.
- `server/internal/service/builtin_skills/multica-autopilots/SKILL.md`: narrow debugging statement to the pre-transaction readiness check, without claiming post-commit teardown never had a task.
- `server/internal/service/builtin_skills/multica-autopilots/references/autopilots-source-map.md`: source ownership and regression mapping.
- `.trellis/spec/server/iterations.md`: preserve the read-gap classification and final transaction fences.

## Resources and evidence

The recorded owner label and exact container ID were verified before removing the task container with its anonymous PostgreSQL data volume. Removal was independently queried; no other resource was touched. Credentials remain only in the private mode0600 fixture files, excluded from public receipts.

Companion `dispatch-race-verification.json` binds red/green/normal/race event counts, full command arrays, platform, source hashes and cleanup. Original logs and source snapshots remain in `.artifacts/release-v0.8.8/dispatch-race/`. No commit, push, main change, tag or release was performed by this agent. The parent must independently review and rebuild release artifacts from the eventual commit.
