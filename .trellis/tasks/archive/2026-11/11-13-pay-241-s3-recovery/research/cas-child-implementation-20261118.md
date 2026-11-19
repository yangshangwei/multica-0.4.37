# S3 ordinary-child CAS implementation — 2026-11-18

Status: **implemented; final CAS and authorized TestMain repair formatting/compile-only/vet passed; corrected runtime acceptance pending**. The coordinator's original R03 handler normal run passed all three functional names but is NOT_ACCEPTED because of a separately reproduced shared-fixture residue. This is the assigned test-only S3 R04 / TC-16 increment. It does not close TC-16, S3 or the parent acceptance matrix.

## Source and ownership

- Accepted source admission: [s3-source-binding-20261118.md](s3-source-binding-20261118.md) and the parent [CAS intake](../../11-13-pay-241-collaboration-acceptance/research/s3-cas-intake-20261118.md).
- Isolated source: `/Volumes/artisan/.multica-acceptance/pay241-s3-cas-20261118-hxjmd77_/source-r01`.
- Final coordinator verification source: `source-verification-r04` under that evidence root, manifest SHA256 `1275d9709b94d55586d155cfec42886d14663ed9090d0da34cb7388d49e62dd3`. Registered current execution-plan SHA256: `60ebfd2c216985537886f1654575357e635740457da41e79a735d3411fe9f5a4`. The coordinator verified all 5,055 other accepted base files unchanged; the only changed base file is the authorized handler fixture below. These are source/input bindings, not database acceptance results.
- The only code addition is `server/internal/handler/collaboration_child_recovery_cas_test.go`, SHA256 `0e6e67e6c45358aaf758de0409fe062193fbdbb3f35551b5ae458c212de0e046`, mode `0644`, 252 lines. The initial 258-line source/hash `d1baf025462818be7c84113367c3f7580163f16d44278675e1cc4c1a187e7b4d` remains in the coordinator's preserved pre-execution snapshots and the original static receipts.
- After the coordinator reproduced the shared-fixture failure, write scope was explicitly widened to private `server/internal/handler/handler_test.go`. Its only delta is the five-line checked dependent-target cleanup below. SHA256 `e925f073ec818d34c65c3706005223c2114e02ed633fe6a1350590cd386daa52` → `abf93cbad6eca6c1e93d75ea8d02e90a4a2306b41669ee08b60246617a8b7fc3`; original mode `0755` preserved. Every other byte and the finalized child CAS file remain unchanged.
- This implementer did not modify ROOT server code, existing test bodies/setup, production/SQL/DDL, dependencies, specs, task plans or Git. The coordinator owns integration and the combined source audit. Static receipts and logs are confined to the explicitly granted `handler-static-r01`, `handler-static-r02` and `handler-testmain-static-r01` directories.

## Implemented assertions

`TestCollaborationOrdinaryChildPriorityAssigneeCASCommitOrders` has the two fixed subtests `priority_first` and `assignee_first`.

Each subtest creates a real collaboration parent through `stageHTTPFixture`, keeps stage policy off, and builds one ordinary linked child and a distinct valid source-member assignee through `testutil.Fixture`. It sends a priority-only PUT and an assignee-ID-only PUT with the same explicit **numeric** `expected_revision`. Requests reuse `stageExitHTTPRequest`, retaining its verified principal, actor/workspace headers and chi resource identity when adding cancellation.

Only the leading handler is cloned, with `stageExitHTTPBarrierStarter{beforeCommit:true}` as its transaction starter. The follower uses the unchanged handler. `stageExitHTTPWaiter` observes the real holder through `pg_blocking_pids` before release; no query, response, commit or access result is substituted. Both responses are collected before status assertions. Every failure path releases the barrier, cancels started requests, waits for both goroutines and drains any uncollected buffered responses before fixture cleanup.

The leading request must return 200; the follower must return 409 with exact `revision_conflict`, `resource_type:"issue"`, child identity, original expected revision and the winner's actual revision. Typed `int64` decoding also rejects string revisions. Persisted child comparison checks every unrequested column, allowing only the ordinary writer's edit/activity clocks to advance. The winning field changes and the child revision advances once. A deliberate new request adopting the winner's numeric revision applies the remaining field and advances the revision once more.

The test compares retained parent issue/collaboration/session/participant rows, event business content, handoffs, operations, notification business identity, commands, targets, groups, grants, runs, run events, termination rows and parent/child ordinary task rows. It also reuses `stagePolicyBusinessSnapshot` for stage facts. Existing outbox delivery markers are excluded from the parent business comparison. The fixture begins with a paused parent and no execution; this proves preserved idle state and no new execution, not preservation of a genuinely executing process.

The existing bus is observed without replacing services. No ordinary update or child hint may appear while the leading commit is held. The race must publish exactly one ordinary update with the winner's actor, revision and field flags; the deliberate continuation adds exactly one more. Parent child-reference hints must be **exactly zero at all three checkpoints**, because neither field change alters that projection. No durable notification/reconnect deduplication result is claimed.

The initial oracle allowed parent hints up to the committed update count, which could mask one loser-only hint. Before any database test ran, the independent checker verified the actual `issue.go:4154–4159` publisher guard: only changed parent, title or admission status triggers `PublishCollaborationChildrenChanged`. The current parent child projection at `handler/collaboration.go:164–173` contains id/identifier/title/can_open. The coordinator then authorized exact zero and removal of the six-line hint-shape loop, which becomes unreachable after the fatal zero-count check. The existing canonical child-hint suite remains the structural owner. All CAS, fields, revisions, parent state, ordinary update events, real barriers and test names were preserved.

The shared creation fixture closes creation unconditionally during teardown. This new test first records the original `creation_enabled` and registers its final restoration before calling the fixture. Restoration changes neither execution flags nor epoch.

## Authorized shared TestMain repair

The coordinator's R03 handler-normal stream has 1 top / 3 named / 2 leaf PASS, exit 0, unchanged source/configuration/gate and connections 0, but `agent_invocation_target` remains at count 1 while the other 29 declared relations are empty. That run is **NOT_ACCEPTED**. An independent fresh TestMain-only RED diagnostic intentionally runs the compiled binary with `-test.run '^$'`; zero test bodies match, yet the same target residue remains. The failure therefore belongs to shared fixture teardown, not the child CAS bodies or product priority writer. Both original databases, raw streams and source snapshots are preserved by the coordinator.

Before this edit the coordinator wrote [testmain-cleanup-diagnosis-20261118.md](testmain-cleanup-diagnosis-20261118.md) with the root cause, cleanup plan, ownership and fresh verification requirements. `setupHandlerTestFixture` creates a shared seeded agent invocation target. The original cleanup deletes workspace/user rows but the target relation has no foreign key/cascade, so it survives that cleanup.

The authorized repair adds only:

```go
if _, err := pool.Exec(ctx, `DELETE FROM agent_invocation_target WHERE agent_id IN (
    SELECT a.id FROM agent a JOIN workspace w ON w.id = a.workspace_id WHERE w.slug = $1
)`, handlerTestWorkspaceSlug); err != nil {
    return err
}
```

This runs in `cleanupHandlerTestFixture` before workspace deletion, selects only agents belonging to the actual handler fixture workspace, and propagates errors through the existing checked-Exec pattern. Shared seed authority remains present throughout `m.Run`; no child test removes it. Setup, other helpers, pools/timeouts, product code and SQL/migrations remain untouched. The precise before source, byte-restoration assertion and five-line diff are retained in `handler-testmain-static-r01/{handler_test.before.go,before.json,file-ready.json,delta.diff}`.

The independent checker confirmed this exact delta and the static/input bindings in `testmain-red-review-r01.json`: the selected agents belong to the actual handler fixture workspace, removal precedes parent teardown, errors propagate, global seed availability and original file mode are preserved, and child/product bytes are unchanged. Its final runtime review remains separate and will retain the original NOT_ACCEPTED residue and TestMain-only RED.

## Verification evidence

Static runner: `/Volumes/artisan/.multica-acceptance/pay241-s3-cas-20261118-hxjmd77_/helpers/run-nodb.mjs`, SHA256 `ab064d765f018d3b9174b8b6fb49c5df608629e7632df2cdd18d4e50a10accd1`.

Private no-database config: `configs/nodb.json` in the same evidence root, SHA256 `d1eecd914509737b828dfc1a9607b6c410d8e85d9bdd8c26cab7d8b23c3096c2`. It is consumed as JSON, never sourced as shell code. The permitted runner strips inherited database/PG/Multica/Redis settings before selecting this config. The implementer displayed no credential values.

| Check | Result | Receipt |
| --- | --- | --- |
| Initial `gofmt`, compile-only and vet | all exit 0 at original `d1baf0…` source | `handler-static-r01/{file-ready,compile,vet}.json`; original stdout/stderr empty and preserved |
| Final `gofmt -w` on the new file only | exit 0 | `handler-static-r02/file-ready.json`; stdout/stderr empty |
| Final `go test -c -o …/handler-static-r02/handler.test ./internal/handler` through the permitted no-database runner | exit 0; final file hash unchanged | `handler-static-r02/compile.json`; stdout/stderr empty |
| Final `go vet ./internal/handler` through the same runner | exit 0; final file hash unchanged | `handler-static-r02/vet.json`; stdout/stderr empty |
| TestMain fixture `gofmt` and exact five-line/mode audit | exit 0; other bytes identical; child CAS hash unchanged | `handler-testmain-static-r01/file-ready.json` and `delta.diff` |
| Corrected fixture `go test -c -o …/handler-testmain-static-r01/handler.test ./internal/handler` | exit 0; fixture/child hashes unchanged | `handler-testmain-static-r01/compile.json`; stdout/stderr empty |
| Corrected fixture `go vet ./internal/handler` | exit 0; fixture/child hashes unchanged | `handler-testmain-static-r01/vet.json`; stdout/stderr empty |
| Coordinator R03 handler normal suite | functional 3 PASS; NOT_ACCEPTED on target residue 1 | `verification/handler-normal/result.json`; original evidence preserved |
| Fresh corrected TestMain-only GREEN | pending coordinator execution | Teardown proof; zero business test bodies |
| Fresh corrected handler normal/race suites | pending coordinator execution | Each expected 1 top / 3 named / 2 leaf; all 30 final relation counts 0 required |

All commands executed by this implementer remain compile-only/static and did **not** execute handler `TestMain`, a test body or database SQL. The coordinator preserved the earlier combined r01/r02 candidates as **NOT_RUN**, following the service fixture cleanup correction and reviewed handler oracle refinement; the later R03 functional run and separate TestMain RED remain NOT_ACCEPTED. The coordinator has now frozen the declared R04 source/input above and owns the fresh TestMain-GREEN, handler-normal-r04 and handler-race-r04 runs. Previously accepted service R03 normal/race evidence may be reused only after proving this new delta is confined to a handler `_test.go` file not consumed by the service package. This is the completed static implementation handoff; runtime acceptance remains coordinator-owned and pending in this report. It can be updated from the retained final receipts on a follow-up. The implementation source is held unchanged for that verification.

## Remaining scope

The reproduced defect is shared test infrastructure; no product CAS defect or production fix is included. Ordinary `expected_revision` remains optional for existing clients; this test explicitly supplies it. Current API/production Web/Desktop combined evidence, other S3 recovery requirements, independent Trellis checking and parent-matrix updates remain with their assigned owners. Real Provider/account execution, payment, deployment, full S3 and S4/S5 are outside this increment.
