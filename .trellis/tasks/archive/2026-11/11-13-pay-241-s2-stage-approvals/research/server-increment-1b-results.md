# S2 server increment 1b — results

- Date: 2026-11-14. Owner: `/root/s2_server`, resuming the preserved 1b brief and predecessor implementation.
- Start HEAD `fb9f4c070`; report HEAD `482cd7f683739aca72050b6b09fdfe724742a898`. The intervening creation-page commit belongs to another session. This owner made no git writes.
- Status: **increment 1b implemented and narrowly verified; S2 acceptance remains open**. `stagePolicyActivationAvailable` remains `false`; no increment-2 route or completion guard was enabled.
- Scope: descendant admission, explicit adopted-code binding, real enqueue/start/update entry-point regressions, and frozen later-stage refusal/R4 contract. No API/daemon process, real model, payment, remote CI or deployment was used.
- Machine-readable twin: `server-increment-1b-results.json` contains exact command argv, directly captured exit codes, log hashes, every named test terminal and current hashes of this resume's files.

## 1. Preserved predecessor evidence

The original logs remain unchanged under the run root's `logs/` directory:

| Log | Evidence |
| --- | --- |
| `s2-server-inc1-review-items-red.log` | Four genuine failures: machine creation below a child; machine attach below a child; descendant enqueue; no offered/confirmed PR code reference. |
| `s2-server-inc1-review-items-green-1.log` | Predecessor service stage green run. |
| `s2-server-inc1-handler-2.log` | Predecessor handler stage green, including explicit A5 code. |
| `s2-server-inc1-migrations-updown-2.log` | Predecessor migration shape/rollback test. |
| `s2-server-inc1-migrate-runner-updown-2.log` | Predecessor real migration-runner upgrade/refusal/rollback/re-upgrade test. |

These are inherited evidence, not runs repeated by this owner. Standalone shell exit records were not reconstructed from PASS/FAIL text. The original red tests were `TestStageChildCreationRequiresApprovedProposalForMachines`, `TestStageChildUpdateKeepsMappingAndRefusesMachineAttach`, `TestStageDescendantsOfManagedRootAreGatedAndConfirmable`, and `TestStageScopeConfirmationBindsVerifiedCodeVersion`.

## 2. Files and behaviour

This resume changed six Go files:

| File | Change |
| --- | --- |
| `server/internal/handler/issue.go` | Preserve `stage_proposal_required` and `stage_ancestry_unresolved` in the existing batch item refusal instead of replacing them with `delivery_completion_conflict`. |
| `server/internal/handler/daemon.go` | Wait-for-directory returns the existing collaboration error status/code, matching start and the documented 409. |
| `server/internal/service/collaboration_stage_actions.go` | A5 with non-null code takes the existing delivery observer's four-table PR/link `SHARE MODE NOWAIT` fence before observation and holds it through commit. Explicit null does not acquire that fence. |
| `server/internal/handler/collaboration_stage_admission_test.go` (new) | Move, Batch, daemon StartTask and wait-for-directory, unchanged human controls, exact error bodies/statuses. |
| `server/internal/service/collaboration_stage_enqueue_test.go` (new) | Eight actual enqueue callers; prepared enqueue success/reuse/stale-lifecycle refusal/root-lock contention; approved-child decomposition refusal and human control. |
| `server/internal/service/collaboration_stage_scope_code_test.go` | Deterministic second-connection PR-head/link mutations between observation and decision commit; remove an unused import sentinel. |

Documentation: this report and JSON twin; `research/wire-contract.md` sections 10.1 and 13. No dependency, SQL, generated file, migration, role-isolation rule or manual-child permission changed during this resume.

The audited inherited 1b implementation resides in service `collaboration_stage_admission.go`, `collaboration_stage_actions.go`, `collaboration_stage_view.go` and their tests, handler `collaboration_stage.go`/tests, `pkg/db/queries/collaboration_stage.sql`, matching generated code/models, and migration 820. These already contained the bounded ancestor/subtree walk, explicit A5 code parser, verified PR links/heads, immutable confirmation payload and R1 code state. This is a relevant-file inventory, not a reconstructed diff against an unavailable predecessor checkpoint.

The ancestry walk is bounded by `stageAncestorDepth = 64`; a cut chain or cycle returns `stage_ancestry_unresolved` on machine admission instead of treating it as outside a managed root. Human creation/attach is unchanged. Further machine decomposition below an approved child also refuses; v1 requires another human-authored proposal or a manual human child. Proposal acceptance still creates only unassigned todo children and never dispatches.

## 3. Commands and results

Logs are in `/Users/artisan/.multica/acceptance/pay241-20261113-01/logs/`, prefixed `s2-server-inc1b-resume-`. Every DB-backed run used that run root's `run.mjs`, which validates actual database identity before spawning Go. Normal runs used only `s2_impl_service.json` / `s2_impl_handler.json`; race runs used only the matching `s2_race_service.json` / `s2_race_handler.json`. Config contents were not printed.

Exact selectors and full named inventories are in the JSON report. “Named” counts include top-level tests and their reported subtests; parent failures are not additional defects.

| Command / scope | Log suffix | Exit | Result |
| --- | --- | --- | --- |
| service new enqueue/prepared tests | `service-new-1.log` | 0 | 2 top-level / 10 named pass |
| handler new descendant entry points, before fixes | `handler-new-1.log` | 1 | 3 named pass, 3 fail terminals; two actual HTTP defects |
| service stage + affected start/creation/legacy/workflow regressions | `service-normal-1.log` | 0 | 37 top-level / 125 named pass, zero skip |
| handler stage + affected Move/Batch/M1/handoff/StartTask regressions | `handler-normal-1.log` | 0 | 29 top-level / 61 named pass, zero skip |
| service A5 connector race, before fence | `code-fence-red.log` | 1 | Both head and link cases fail; 3 fail terminals including parent |
| service all `^TestStage` after fence | `service-green-2.log` | 0 | 19 top-level / 36 named pass, zero skip |
| handler same narrow selector with `-race` | `handler-race-1.log` | 0 | 29 top-level / 61 named pass, zero skip |
| service same narrow selector with `-race`, including new fence test | `service-race-1.log` | 0 | 38 top-level / 128 named pass, zero skip |
| `go -C server vet ./internal/service ./internal/handler` | `vet-1.log` | 0 | No diagnostics |
| `gofmt -l` on this resume's six Go files, failing on nonempty output | `gofmt-1.log` | 0 | No files listed |

The final service normal stage run includes the A5 fence fix; its final race run includes the complete narrow regression selector. The handler race run also uses the final A5 fence implementation. No broad package/full repository suite was run this increment.

Migration 820 remains byte-identical to the coordinator's allocations: up SHA-256 `bd7c405d8d1f8d8e517463fec0305df87f0b0b6cb93a2fcf751e21290e9cc81f`, down `5bae0fb1d84f02a55c0a1a3a35ec5d9ba7bdbb1fd1056dae9e3a1c17fd0ab3e5`. This owner ran no migration or sqlc command; no SQL changed.

## 4. Real entry points and named assertions

| Actual entry point | Test and observation |
| --- | --- |
| `IssueService.Create` → `CreateInTx` | `TestStageChildCreationRequiresApprovedProposalForMachines`: direct/deeper machine and agent-header refusal, cycle refusal, human/unrelated/disabled controls. `TestStageApprovedChildCannotAuthorizeMachineDecomposition`: accepted child cannot grant further machine decomposition; manual creation works. |
| `Handler.CreateIssue`, `Handler.UpdateIssue`, `Handler.BatchUpdateIssues` | `TestStageAdmissionThroughOrdinaryIssueHandlers`: direct/descendant refusal, no issue/mapping from rejected creation/attach, human control, semantic-version behaviour. |
| `Handler.MoveIssue`, exact batch refusal | `TestStageDescendantMoveAndBatchRefuseExactAdmissionCode`: exact stage refusal and unchanged parent; human Move succeeds without mapping. |
| Assignment, handoff, mention, thread, squad, squad handoff, deferred fallback, deferred channel enqueue | `TestStageDescendantEnqueueEntryPoints`: all eight actual TaskService methods return `stage_child_unapproved`; zero task rows and execution events. |
| `PrepareIssueTaskEnqueue` → `EnqueuePreparedIssueTaskInTx` | `TestStagePreparedEnqueueAdmitsExactlyConfirmedDescendant`: real prepared agent/runtime/attribution, held child row, committed queued task, exact runtime/agent/actor, reuse of one row; unapproved/stale lifecycle refusal; R1 shows stale queued work gated; root writer yields NOWAIT without insertion; manual descendant works. |
| `TaskService.StartTask` → `startTaskAtomically` | `TestStageChildExecutionGateAtEnqueueAndStart`, `TestStageDescendantsOfManagedRootAreGatedAndConfirmable`, `TestStageChildStartSerializesWithRootLifecycle`: both lifecycle/start orders, current confirmation, subtree depth, persistent mapping after reparenting, human/unmanaged controls and in-flight root contention. |
| `Handler.StartTask`, `Handler.MarkTaskWaitingLocalDirectory` | `TestStageDescendantDaemonStartAndWaitRefuseBeforeMutation`: exact 409 error and unchanged dispatched task; human controls reach running/waiting. Real handler functions are called in-process, not through a launched API. |
| A5 service/handler and R1 | `TestStageScopeConfirmationBindsVerifiedCodeVersion`, `TestStageHTTPScopeConfirmationRequiresExplicitVerifiedCode`: required code/null, verified GitHub/VCS linked head, wrong provider/head/link/reference-only refusal, atomic no-mapping on failure, immutable payload, changed/unavailable R1 state. All external facts are synthetic fixtures. |
| A5 observation-to-commit boundary | `TestStageScopeConfirmationFencesCodeUntilCommit/head` and `/link`: a real second transaction attempts mutation after the read but before decision insertion; with the fence it gets SQLSTATE 55P03 and the committed reference stays current. |
| Existing surrounding workflows | Selected M1 batch/authorization, lifecycle handoff, creation dependency failure, legacy execution, StartTask iteration, workflow rollback/lost-commit/current-lock/CAS tests pass. Every selected name is in the JSON. |

Prepared-enqueue and most additional entry-point controls passed on first execution; they are characterization/regression coverage, not newly claimed red-first evidence. Every new production fix has a preceding actual failure: the HTTP mappings in `handler-new-1.log`, and the connector interleavings in `code-fence-red.log`.

## 5. Contract delivery

- Sections 5.1 / 7.5 / 13 already freeze required A5 per-child `code`, explicit null, verified provider/PR/head and R1 `code`, `code_state`, `pull_requests`, subtree `depth`. Head changes affect code applicability, not ordinary execution or the scope-state enum.
- Section 10.1 explicitly rejects further machine decomposition below an approved child and includes wait-for-directory's error body. Batch and daemon mappings now match the contract.
- Section 13 records the PR/link fence and digest privacy limitation. No request or success DTO changed in this resume.
- Section 10.2 freezes submit/complete/R38 `stage_incomplete` with 1–32 ordered `stage_blockers`, and project exit/delete `stage_policy_exit_required`. Section 8.6 freezes R4 including `signable.max_expires_at`. These remain **not served** pending increment 2.

## 6. Remaining risks and handoff

1. **Digest privacy is not proven.** SHA-256 of title/description permits equality comparison and offline guesses of low-entropy private descriptions. External R1 readers currently receive it when `can_open=false`. The coordinator retained this for independent privacy review and instructed 1b not to change the frozen digest protocol. Do not describe the leak as “equality only.”
2. **Connector source integrity is separate.** The coordinator found non-monotonic GitHub PR upserts and silently omitted unnormalizable snapshot checks. Increment 2a must add synthetic counterexamples/minimal repairs before policy activation; VCS without repository identity stays unsupported. The A5 fence prevents local observation/commit races, not remote staleness or old webhook ordering.
3. **Increment 2 remains required:** signer credential identity/generation/expiry, code review, reports/QA, deployment/rollback, submission/final guard, project exit/delete, activity and public enabling. The old scope role-restoration and child-admission slice is not complete F01/F03 coverage.
4. **Pending client clarifications precede 2a code:** code-candidate discovery, R4 source-denied variant, approval-versus-return semantics; detailed T4 DTOs precede 2b. These coordinator requests arrived during verification and are not labelled served here.
5. **Unrelated failures remain untouched:** `TestBatchUpdateIssues_AutopilotRunAssign` and deletion-manifest `report_snapshot` / `report_snapshot_control`. Neither was changed or rerun in this bounded increment.
6. No new production-router/password-auth journey, Web/Desktop build or interactive acceptance, Linux runtime, real provider/CI/payment/deployment, or complete A01–A12 run occurred. Migration evidence is inherited. This report fills no real S4/S5 acceptance cell.

The 1b implementation boundary is complete. The server owner stops here for coordinator review and a separate 2a continuation prompt.
