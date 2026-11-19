# R26 candidate-read regression implementation

The initial original-product regression is ready and has passed compile-only verification. Main has started one RED run under the sealed permit; its outcome has not been delivered to this implementation agent. This report contains no new database-test PASS claim and authorizes no product repair.

- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.
- Sealed plan: `r25-m1-read-coexistence-test-plan-20261117.json`, SHA256 `0cc6121e5e237b5352f02ce479e457352a0e782564ae01fcb3cb145545406a28`.
- New source: `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r26-m1-candidates-read-r01/source/server/internal/service/collaboration_completion_candidates_read_concurrency_test.go`.
- Source SHA256: `086a559dcb420a41a88dcd5e499edae2f76325b49b4b0266bcf4704865a46ce9`; 32,315 bytes, 674 lines, mode0644.
- Original R25 base: all5047 entries have equal bytes/modes/links; R26 has exactly5048 entries, with this test as the only addition. Source-comparison manifest SHA256 `7d5fd6e96ac8d271316ee658a21dc75068995302b08d451e9c493c4e95fc71f5` is an evidence fingerprint, not a new canonical product source ID.

## Implemented test boundaries

The four top-level tests contain exactly11 literal leaves (15 total named records). Go's actual parser matched the sealed inventory. There are no Skip or Parallel calls in those tests.

| Test | Leaves |
| --- | --- |
| `TestCollaborationCompletionCandidatesR4Coexistence` | `r4_first`, `candidates_first` |
| `TestCollaborationCompletionCandidatesSiblingReads` | `same_creator`, `different_current_managers` |
| `TestCollaborationCompletionCandidatesWriterFences` | `writer_first`, `reader_first` |
| `TestCollaborationCompletionCandidatesCurrentAuthority` | `wrong_workspace_child`, `outside_participant`, `disabled_account`, `expired_credential`, `revoked_artifact_manager` |

Every fixture reuses `stageArtifactConcurrentReadFixture`, its real stage review/packager/candidate/advance setup, and `stageExitServiceM1(...,"enforce")`. An unheld real candidate baseline must return the exact root/source/project revisions and exactly one selectable approval with the actual canonical spec, execution, output and receipt metadata. A real R4 baseline must identify the same current artifact candidate and passed CI. A failure at this setup boundary remains a setup/baseline failure and cannot count as read/read RED.

Candidate calls use actual `f.pool.Begin(actorCtx)` and the same borrowed transaction, actual `f.ctx`/reviewer contexts and `completionRequestCredential(ctx)`, source/root fixture IDs, org zero, limit20 and empty cursor. The test leaves the default candidate transaction isolation intact. Each reader retains the five-second context and product250ms artifact lock timeout. R4 uses its unchanged evidence-read transaction.

R4-first reuses `holdStageArtifactR4` after its real authority fence. The candidate method, final credential check and actual commit/rollback all finish before R4 is released. Candidate-first holds a completed candidate transaction open while a second actual R4 reaches its terminal; only then does it commit. Sibling readers use the same held-return barrier and require the second terminal before the first commit. All concurrency controls require two distinct, nonzero PostgreSQL PIDs. These timings express method/transaction terminal order; they do not assert an exact kernel lock-release timestamp.

The SQL-forwarding transaction captures the first real PgError at Exec, Query, Row.Scan, Rows.Scan/Err or Commit before `deliveryCompletionError` can flatten it into a domain409. SQL, arguments and Scan destinations are forwarded unchanged. Witnesses retain the query name and SQL hash, UUID-only arguments, phase, backend PID, raw SQLSTATE/message/detail/where/table/constraint and outer error type. No SQL result, authority, readiness or row is injected. Current candidate SQL does not use SendBatch; that inherited method is not claimed as an instrumented path.

The real writer controls call unchanged `EvaluateDeliveryEvidenceInTx` with `stageProjectionWriterRequest`. Writer-first must establish an actual pass observation and make the candidate reader refuse without a partial page before writer release. Reader-first must retain the writer's original `LockLifecycleIssue` refusal and never reach the `LockArtifactUsers` marker. Fresh reads after rollback/commit must recover the exact original page.

Authority cases distinguish the actual response contracts:

| Control | Expected service response |
| --- | --- |
| Real ordinary child in another workspace | Delivery404 `delivery_completion_not_found`, zero page |
| Actual external handler/participant with no source membership | Delivery403 `delivery_completion_denied`, zero page |
| Current creator account disabled in the owned database | Collaboration404 `not_found`, zero page |
| Same verified fixture creator with expired finite JWT context | Delivery403 `delivery_completion_denied`, zero page |
| Current reviewer/admin demoted through actual `UpdateMemberRole` | Authorized root/source page with empty items, no selectable artifact/private metadata |

The expiry case is a service-context boundary, not an actual signed-token/router login journey. The manager case first requires its real positive candidate baseline. Each deliberate authority mutation is incorporated in that case's own no-write baseline.

The test compares `stageExitServiceSnapshot` plus complete rows/counts for 72 explicit relations on the fresh, exclusively admitted serial service database. It covers issue/source statuses and revisions, collaboration/session/account epochs, roles/scope/candidates/decisions/revocations, operations/events/outbox/notifications, grants/terminals, commands/runs/attempts/reservations/runtime leases, actual artifact canonical spec/receipt/request/generation/output metadata, source PR/check/trust facts, task usage and iteration effects. Successful and failed control cleanup precedes the final comparison, including rollback/cancel and the existing once-only R4 release/drain. Pool snapshots are verification outside product calls and never substitute for their borrowed transactions.

The existing packager fixture creates a real tar but does not expose its output path. This test compares complete database artifact records; it does not claim a fresh filesystem-byte audit or add another packager setup.

## Checks actually run

- `gofmt -w` and final `gofmt -l`: clean for the new file.
- Actual Go AST inventory:4 tops/11 leaves, exact sealed names, zero prohibited Skip/Parallel, no test bodies executed.
- Exact main-authorized compile-only command, working directory `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r26-m1-candidates-read-r01/source/server`:
  `/opt/homebrew/bin/go test -c -o /Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r26-m1-candidates-read-r01/evidence/service-red.test ./internal/service`.
- Environment was exclusively the parsed `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r26-m1-candidates-read-r01/database/profiles-r01/red-environment.json`, SHA256 `02d6bc6bfa34f965ff5993c61a77264c092182677b65d01edaca0327f31ab485`; no inherited environment was forwarded.
- Compile exit0 in 21.606s, empty stdout/stderr. TestMain and database fixtures were not executed. Binary SHA256 `6f57745cb014dc5fffe7ab54da08658c15bdd60fdac9da23a9052bce4177005a`, 86,030,802 bytes. No mechanical compile fix was needed.
- Read-only source comparison:5047 unchanged original entries, one new test, zero missing/changed paths.

Raw compile/AST/source records are under `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r26-m1-candidates-read-r01/evidence`. Their exact path/hash bindings are in the accompanying JSON.

## Remaining gates

Main RED is running once under `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r26-m1-candidates-read-r01/verification/red-permit-r01.json`, SHA256 `13be3d737b83a3934793ebf97f1dae2b70cf3ca9ca81e85af92de765609c0fa2`, source086a/5048inputs. This agent will neither repeat that run nor change the source while it runs. The recorded selector is `^(TestCollaborationCompletionCandidatesR4Coexistence|TestCollaborationCompletionCandidatesSiblingReads|TestCollaborationCompletionCandidatesWriterFences|TestCollaborationCompletionCandidatesCurrentAuthority)$`, with original5s reader/10m package bounds and15 expected named terminals. Main owns the database, API0/PAYsuite1 admission, outcome classification and later bounded product permission.

R24's candidate409 body/SQLSTATE/holder remain unknown. R25's retained full-four result remains1PASS/1FAIL/2NOT_RUN at its earlier QA failure, with no M1 candidate capture. A deterministic new RED can prove this read boundary independently; it cannot identify either historical holder. Preview POST, complete M1 writer/R38/Web/Desktop acceptance, A01–A12, the parent24 cases and S3–S5 remain separate gates. No product, SQL, root code, old source/evidence, configuration, Git or dependency changes were made by this implementation agent.
