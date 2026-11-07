# Independent review: handler heartbeat fixture

Verdict: **PASS WITH NOTES**. No blocking code finding in the proposed test-only patch. Integration should wait for the recorded whole-handler recheck to finish successfully. This review does not turn the original failed package run into a pass or approve the previous adapter-v5/RKI6 launcher.

## Scope and identity

- Candidate: `7af9ab9905c98c77bb091223ebe687c639475bb4`.
- Reviewed worktree: `/Volumes/artisan/code/2026/mc-functional-go-7af9ab990-20261102`.
- Only dirty tracked path there: `server/internal/handler/daemon_test.go`.
- Actual patch SHA-256 matches the binding: `f78404b75d10b2527e0bcef236f86fe2504901df1625a989a23e99f77db46fbe`.
- Read task requirements/design/execution contract, database-isolation spec, the complete patch, affected tests, shared `TestMain` fixture, production claim SQL/constant, canonical stale/offline regressions, original failure events and isolated replay results.
- No test, database connection, service, Provider, process control or source mutation was performed. Only this review file was written.

## Findings

### No blocking finding

1. **The change supplies a legitimate precondition for the assertions.** `server/internal/handler/handler_test.go:135` creates the shared online runtime once with `last_seen_at=now()`. The five affected tests subsequently obtain that shared agent/runtime and assert project payload, repository fallback, run-only context or workspace-mismatch settlement. None is a stale-runtime test. A live runtime is required to reach the behavior each test names.

2. **The production fence remains intact.** `server/internal/service/task.go:196` defines the 150-second heartbeat requirement. `server/pkg/db/queries/agent.sql:892` independently requires online status and a recent `COALESCE(last_seen_at, updated_at)` before dispatch. The patch changes neither the constant, SQL, production code nor handler response rules. It refreshes only the selected test runtime's heartbeat through a parameterized `dbfx.Exec` call.

3. **Other authorization dimensions are not repaired or bypassed.** Runtime status, owner, visibility, agent binding, workspace identity, admission state, task contents and assertions remain unchanged. The mismatch test still requires HTTP 500 and a cancelled task; a fresh heartbeat makes that intended consistency check reachable rather than accepting an empty queue response as its result.

4. **Stale/offline coverage remains separate and meaningful.** `server/internal/service/runtime_health_gate_test.go:64` tests wrong scope, offline status and an explicitly stale heartbeat, checks that the task remains queued, then explicitly refreshes the heartbeat for the positive claim. The retained `evidence/go/61-internal_service/test-outcomes.json` records this canonical regression as `pass`. Handler-side `TestClaimTask_BriefingFixturesDoNotUseExpiredSuiteRuntime` also explicitly ages its suite runtime and is untouched. No global heartbeat loop or broad helper refresh was introduced that would undermine these negative cases.

5. **The diagnostics are limited and appropriate.** The first affected test records the heartbeat age before refreshing it, using the same timestamp fallback as production. It logs no credential or business content. Existing assertion placement is preserved, no imports/dependencies were added, and no timeout or retry changed. The fixture helper executes SQL and reports SQL errors; it does not assert the product outcome on the test's behalf.

## Evidence limits and required follow-through

- The original whole-handler run remains failed: 2,858 top-level pass, five fail, seven skip. Its five claim failures occurred consecutively around `2026-11-02T00:27:09Z`, roughly 4 minutes 48 seconds after the runner started. The original five tests passed together on a new database in `evidence/handler-claim-replay-1`. This supports a suite-state/freshness prerequisite explanation, but isolated success alone cannot establish the precise heartbeat age in the old failed run.
- At this review, `evidence/handler-full-freshfixture-1/run.json` still reports `state=running`; it has no final exit or heartbeat diagnostic yet. Do not report a successful whole-handler recheck until its final record and named outcomes actually exist.
- Before synchronizing the patch to the root branch, require that fresh whole-package race run to exit 0, all five affected names to pass, and no new failures appear. Preserve its actual pre-refresh age diagnostic. If it exceeds 150 seconds, it directly corroborates the stale-suite-fixture explanation for the new controlled run; it still does not reconstruct the deleted old fixture's exact timestamp.
- Keep the original failure, isolated original replay and patched whole-run evidence separate. Record the final harness SHA and identify the change as a test-fixture repair, not a product behavior fix or a successful rerun of the original unchanged harness.

Severity summary: **0 critical, 0 high, 0 medium; verification/evidence notes only**.
