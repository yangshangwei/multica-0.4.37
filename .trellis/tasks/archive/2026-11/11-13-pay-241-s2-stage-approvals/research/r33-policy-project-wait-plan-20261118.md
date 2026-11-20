# Research: R33 bounded M1 policy-project wait

- Query: Plan a genuine handler regression and the smallest M1-only repair for R32's now-confirmed policy-project contention.
- Scope: internal, frozen source and original R32 evidence; no implementation or runtime execution.
- Date: 2026-11-18

## Findings

Recommend a new explicit **M1-only** `LockDeliveryPolicyProjectWait` query and one delivery-policy PUT call-site switch, after fresh genuine RED. Preserve the original `LockDeliveryPolicyProject` query and its stage-policy caller unchanged. Both variants retain the same project/workspace predicates and exclusive `FOR UPDATE`; the new variant uses the existing250ms lock-wait budget.

### Primary evidence and files found

`R32` is `/Volumes/artisan/.multica-acceptance/pay241-r32-policy-read-20261118-24y0i9dv`; source citations below are relative to its frozen `source-red-r01`. Coordinator prepared a fresh5,055-file R33 source at `/Volumes/artisan/.multica-acceptance/pay241-r33-policy-wait-20261118-hc2gedj3/source-red-r01`; its complete manifest/profile preparation was not re-audited here.

| File | Relevant pattern / responsibility |
| --- | --- |
| `R32/runtime/policy-diagnostics.jsonl` |37 actual records, no capacity marker; precise GET11/PUT12 correlation. |
| `R32/runtime/four-scenarios-r01/result.json` | Original4 outcome3PASS/1FAIL/0NOT_RUN, retry0, fixture356/spec587 failure. |
| `server/internal/service/artifact_authority.go:72` | `PrepareArtifactTransaction` installs `SET LOCAL lock_timeout='250ms'` before schema work. |
| `server/internal/handler/delivery_policy.go:118` | Current authority, owner/admin check, exclusive project fence, current revision CAS, final credential check and commit. |
| `server/pkg/db/queries/delivery_policy.sql:31` | Existing `LockDeliveryPolicyProject` uses `FOR UPDATE NOWAIT`. |
| `server/pkg/db/generated/delivery_policy.sql.go:197` | Exact generated SQL and real `QueryRow`/`Scan` adapter. |
| `server/internal/service/collaboration_stage_policy.go:99` | Second production caller: stage policy independently sets250ms at108 and uses the original query at122. |
| `server/pkg/db/queries/project.sql:92` | Actual policy GET's association fence is `FOR SHARE NOWAIT`, with the same ID/workspace predicates. |
| `server/internal/handler/delivery_policy_test.go:21` | Existing `deliveryPolicyFixture`, real handler adapters and `testutil`/`dbfx` setup. |
| `server/internal/handler/agent_artifact_test.go:836` | Existing borrowed-real-transaction commit barrier pattern; no invented SQL result. |
| `delivery_policy_schema_test.go`, `delivery_completion_activation_test.go`, `issue_delivery_enforcement_test.go` in handler | Focused schema, enforce activation and actual completion controls. |

The preceding GET, JSONL33/request_trace11, returned complete200 with current policy revision5/enforce for project `771cb66e-265b-4983-aabe-6cc7882389e6`. JSONL35/request_trace12 then records the actual PUT project query failing at `2026-11-17T23:50:01.653428Z`: PID4038, SQLSTATE55P03, query SHA256 `f56557e1843a2ebb4a8d99175df7e5edd369d42445f06127536eab0cd93eab7e`. Its original frames identify generated Scan209 → `deliveryPolicyInTx:135` → PUT. JSONL37 correlates the same path, verified actor and request ID at `23:50:01.657868Z`, HTTP409, complete36-byte body `{"error":"artifact_authority_busy"}\n`.

This proves the busy branch of **this R32 request**, before current-policy CAS. It supersedes the earlier unknown-branch diagnosis for this new run; it does not retrospectively attribute R31's separate failure. The physical holder and its reader/writer class remain unknown. They are not prerequisites for a deterministic short-reader regression. Original completed E04 writes stay preserved; R38 is still not reached.

### Chosen repair boundary and tradeoffs

After RED and independent acceptance, only these three product files are candidates:

1. `server/pkg/db/queries/delivery_policy.sql`: add `-- name: LockDeliveryPolicyProjectWait :one` with `SELECT * FROM project WHERE id=$1 AND workspace_id=$2 FOR UPDATE;`. Keep original `LockDeliveryPolicyProject` byte-for-byte unchanged.
2. `server/pkg/db/generated/delivery_policy.sql.go`: canonical sqlc output for the added query/params/method. Same project row, `ID`/`WorkspaceID` parameters, exact existing column scan and return/error behavior.
3. `server/internal/handler/delivery_policy.go:135`: PUT calls the new explicit waiting method and corresponding params. GET137 remains on its original shared NOWAIT method. No other handler/service caller changes.

Rejected: removing NOWAIT from the shared existing query in only two files. It also changes stage-policy writes at `collaboration_stage_policy.go:122`, broadening timing and lock-order exposure beyond the proven M1 request. The explicit one-query addition isolates that effect without a flag, global retry helper or changed schema. `LockProjectForUpdate` is unsuitable because its `FOR NO KEY UPDATE` is weaker than the original fence (`project.sql:12`). The existing blocking `LockProjectForExecutionSquad` has matching lock strength but describes another operation; an explicit policy wait keeps the audited call-site contract clear.

Removing NOWAIT permits waits behind **readers and writers**. A short competing writer may commit, after which the waiting request sees the new policy revision and returns stale-CAS409. A long writer still produces native55P03/409 within the existing budget. Immediate writer refusal is not preserved or claimed. Serialization, current authority, CAS and zero effects from the losing request must be preserved.

`lock_timeout` bounds **each lock acquisition**, not the complete transaction or HTTP duration. The250ms setting already exists; no timeout, pool, retry, auth, schema or predicate changes are proposed. While waiting, the transaction retains its earlier workspace/user/member/credential fences. A conflicting reverse lock order can therefore stall or form a cycle; the existing finite timeout and rollback contain it, but do not prove deadlocks impossible. Preserve error mapping and the original stage NOWAIT path; do not add lock upgrades, retries or a special reader bypass.

### Genuine regression: one new handler file,2top/4leaf/6named

New test-only file: `server/internal/handler/delivery_policy_project_wait_test.go`. No existing test/fixture file edits. Use two tops:

- `TestDeliveryPolicyProjectWaitShortReader`: `short_reader_release`.
- `TestDeliveryPolicyProjectWaitBoundaries`: `long_reader`, `competing_writer`, `member_denied`.

Freeze the actual authored inventory and identical new test source before RED/GREEN. The short-reader leaf must fail on original source with the actual409/55P03; do not force all other leaves to pass RED. The stricter post-wait CAS outcome may also fail the old immediate-refusal path. Preserve every actual named/leaf outcome.

Use existing `newDeliveryPolicyFixture`, `deliveryPolicyAttach`, `artifactFixture.request`, `testutil.Call`, and `dbfx` (`delivery_policy_test.go:29`,40,49; `agent_artifact_test.go:104`). The fixture supplies a real existing human JWT/current database authority, not PAT: `request` installs `CredentialJWT` with a one-hour expiry. These are handler fixtures, not evidence of router/login or original Web/Desktop execution. `newDeliveryPolicyFixture(t,false)` provides a legitimate virtual revision0/off policy; use an actual PUT with expected revision0 and explicit observe/off intent. Do not substitute manufactured policy/SQL/auth results.

| Leaf | Actual transaction sequence and required assertions |
| --- | --- |
| `short_reader_release` | Run actual `GetDeliveryPolicy` on one handler copy; hold its real transaction at the `GetDeliveryPolicy` query, after successful `LockProjectForAssociationNowait` Scan, or at the real pre-Commit barrier. Record reader PID and confirm GET has not returned. Start actual PUT on a separate copy/connection with the same project and expected revision0. Observe the real project-lock attempt, then release GET after20–50ms, well below250ms. Original source returns409/55P03 while held: genuine RED. Repaired source waits, GET200 and PUT200 finish after release; exactly one append/revision1, requested mode, old facts intact, general project/issue revisions unchanged. |
| `long_reader` | Hold the same actual GET until the PUT has returned. PUT must return409 `error:artifact_authority_busy`, native55P03, no successful policy append or unrelated effects. Record the real250ms setting and measured elapsed time. Use a bounded test guard (for example1s for the result, existing5s orchestration where necessary), not an exact249/250ms assertion. A guard does not increase the product setting. Release/read completion and rollback/connection cleanup remain mandatory. |
| `competing_writer` | First actual PUT reaches the real pre-Commit barrier with the exclusive project fence and one uncommitted append. Second actual PUT, same observed revision, begins before release. Release the winner well inside250ms. Winner200; loser must reach current-policy CAS and return409 `error:delivery_policy_revision_conflict`, with exactly one committed append and no stale overwrite. This specifically distinguishes serialization/CAS from accepting a generic busy409. Existing `TestDeliveryPolicyConcurrentCAS`/writer-first controls additionally cover a writer held beyond the budget. |
| `member_denied` | Create a genuine current member in the fixture workspace with `dbfx.User`/`dbfx.Member`, leaving the owner's row unchanged; use that member's actual fixture principal. GET remains200 where authorized; PUT returns403 `error:delivery_policy_management_denied` before either project-writer query is attempted. Assert no wait-query entry, no append and unchanged fixture-owned facts. This preserves current-role denial without pretending a retained member/user SHARE fence permits concurrent revocation. |

The reader barrier must hold an actual GET's acquired shared row fence, not an open-coded `SELECT ... FOR SHARE` stand-in. Use independent handler copies so the existing barrier's counter is not shared concurrently. A new test-local transaction/row observer may forward real `QueryRow`/`Scan` then record native error, SQL hash/name, PID and barrier events. It must never replace a return value. Recognize the original and new project-query names only so the same test source exercises both versions.

For repaired source, a narrowly bounded real `pg_blocking_pids`/wait-state witness can show writer PID is waiting on reader/winner before release; use existing ordinary pool capacity and real results. For RED, observe its early error immediately instead of consuming250ms waiting for a wait state that NOWAIT cannot enter. Record causal order and distinct PIDs; do not rely on a sleep alone to claim the PUT began while held. Every failure path releases channels, finishes goroutines and rolls back owned transactions. Do not silently recast timeout/cancellation as a valid reader-release witness.

Snapshot only fixture-owned facts after setup: immutable existing policy rows, append count/revision/mode, general project/issue state and relevant retained evaluation/authorization/artifact identities. Permit the expected winner append; assert zero additional effects for refused attempts. Fixture construction has its own legitimate artifact/history effects, so no whole-database zero-mutation claim is appropriate.

### Focused controls and execution handoff

Coordinator owns fresh package-specific RED/GREEN/race databases and all execution. First author/freeze the new file against unchanged R33 product, then compile/static-check and run genuine RED. Only accepted RED permits the three-file product slice. Run fresh same-source new leaves normal then race, plus these six existing tops normal/race as the focused verification set:

- `TestDeliveryPolicyDefaultAppendAndCAS`, `TestDeliveryPolicyManagementAuthority`, `TestDeliveryPolicyStrictRequests`.
- `TestDeliveryPolicyRuntimeSchemaRefusesPartialAndWrongIndexes`.
- `TestDeliveryCompletionActivationExplicitCASAndCapabilities`, `TestDeliveryCompletionActivationRejectsPartialSchemaAllowsOff`.

Original source-member/password fences cannot be assumed revocable concurrently through retained shared locks. Precommitted role/password withdrawal must still refuse; the existing final credential check also continues to reject expiry after waiting (`artifact_authority.go:101`,110,142; `delivery_policy.go:164`). Preserve these calls and predicates byte-for-byte and retain their source-bound prior controls; this bounded increment adds no new expiry/authority matrix. No new service/routing dependency is introduced under the chosen M1-only branch. Verify original stage query/caller unchanged through exact source diff; do not reexecute the entire handler99/router54 inventory without a new affected path or failure. Format, canonical generation/static SQL diff, scoped build/vet and independent review accompany this change; broader tests need concrete new evidence.

After ROOT integration and matching product builds, coordinator runs the original four once with fresh binding and unchanged assertions/deadlines/retry0. This handler regression cannot close fullS2 or claim R38, realS4 or finalS5. Keep prior failures, successful completions and stopped-runtime history intact.

### Related specs / references

- `prd.md` A08/A09 and `design.md` policy-off/CAS and lock/recovery contracts preserve explicit rollback, current authority and independent final consumers.
- `.trellis/spec/server/delivery-policy-evaluations.md` §§3–4 currently documents project NOWAIT/no retry and contention409. After accepted implementation, the spec owner should document the explicit M1-only bounded wait while retaining stage-policy/original query NOWAIT; this researcher cannot edit specs.
- `.trellis/spec/server/database-test-isolation.md` and `verification-environment.md` govern fresh handler profiles, inventories and raw normal/race evidence.
- `.trellis/workflow.md` §1.2 requires task-owned persisted research. Existing primary dependencies remain Go1.26.6, pgx/v5 v5.9.2 and Chi/v5 v5.3.0; no new dependency or external lookup is needed.

## Caveats / Not Found

- R32 proves the exact request/query/error; the physical holder remains unknown. The planned short-reader case is a new deterministic contract regression, not reconstructed evidence of that holder.
- No product edit or RED/GREEN result exists from this researcher. Only these two new research files are written and sealed0444; no Go/test/DB/API/browser/process-control/agent actions occurred.
- Research-role manifest isolation still applies: no `research.jsonl`; `implement.jsonl`/`check.jsonl` were not loaded. Previously loaded PRD/design/implement and specs remain context.
- Coordinator reports normal API/bridge stop once, zero connections, gate82,70settled/55complete and preservation of381old/424current paths. These cleanup/profile facts were not re-audited here. FullS2 remains open; parent24 remains0PASS/9PARTIAL/15NOT_RUN.
